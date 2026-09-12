package stream

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fairdrop/internal/source"
	"fairdrop/internal/transfer"
)

func TestPreparedArchiveNativeRootReplacementIsRefused(t *testing.T) {
	base := fixtureDir(t)
	root := filepath.Join(base, "folder")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	staged, err := source.New().Inspect(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	p, err := New(source.New()).Prepare(context.Background(), staged)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if err := os.Rename(root, filepath.Join(base, "original")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "secret"), []byte("replacement"), 0o644); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	err = p.WriteTo(context.Background(), &body)
	assertCode(t, err, transfer.ErrSourceChanged)
	if _, err := zip.NewReader(bytes.NewReader(body.Bytes()), int64(body.Len())); err == nil {
		t.Fatal("replacement yielded a readable ZIP")
	}
}

func TestArchivePortableNamesAreRejectedAtBothBoundaries(t *testing.T) {
	for _, name := range []string{"con.txt", "NUL .txt", "COM¹", "lpt².log", "bad\u202ename", "bad\u0001name", "name?", "name*", "name<", "name>", "name|", `name"`, "name:", "tail.", "tail "} {
		if got, err := archiveEntryName("root", "folder/"+name); got != "" || transfer.ErrorCodeOf(err) != transfer.ErrPathUnsupported {
			t.Errorf("unsafe nested ZIP name accepted: %q %v", got, err)
		}
		if _, err := archiveEntryName(name, "file"); transfer.ErrorCodeOf(err) != transfer.ErrPathUnsupported {
			t.Errorf("unsafe ZIP root accepted: %q", name)
		}
	}
	for _, name := range []string{"résumé with spaces.txt", "a'b;c", "照片"} {
		if got, err := archiveEntryName("root", "folder/"+name); err != nil || got != "root/folder/"+name {
			t.Fatalf("ordinary ZIP segment changed: %q %v", got, err)
		}
	}
}

func TestPrepareValidatesSanitizedArchiveRootAndReleasesPin(t *testing.T) {
	closed := 0
	s := &scriptedSource{prepare: func(context.Context, string) (transfer.PreparedDirectory, error) {
		return testPreparedDirectory{close: func() error { closed++; return nil }}, nil
	}}
	p, err := New(s).Prepare(context.Background(), transfer.StagedItem{Kind: transfer.ItemDirectory, Name: "CON.txt"})
	assertNoPayload(t, p, err, transfer.ErrPathUnsupported)
	if closed != 1 {
		t.Fatalf("unsafe archive root leaked its prepared pin: closes=%d", closed)
	}
}

func TestArchiveEmitsExplicitPortableModes(t *testing.T) {
	p := newTestArchive(t, &scriptedSource{walk: func(ctx context.Context, path string, visit transfer.SourceVisitor) error {
		if err := visit(transfer.SourceEntry{RelativePath: "nested", Kind: transfer.ItemDirectory}, nil); err != nil {
			return err
		}
		return visit(transfer.SourceEntry{RelativePath: "nested/file", Kind: transfer.ItemFile}, bytes.NewBufferString("data"))
	}}, "root")
	defer p.Close()
	var body bytes.Buffer
	if err := p.WriteTo(context.Background(), &body); err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(body.Bytes()), int64(body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(z.File) != 3 {
		t.Fatalf("ZIP entry count=%d, want 3", len(z.File))
	}
	for _, f := range z.File {
		want := os.FileMode(0o644)
		if f.FileInfo().IsDir() {
			want = os.ModeDir | 0o755
		}
		if f.Mode() != want {
			t.Errorf("ZIP mode for %q = %v, want %v", f.Name, f.Mode(), want)
		}
	}
}

// The sentinel at read 102 means removing a guard fails an assertion instead
// of hanging or relying on a test timeout as mutation evidence.
type finiteEmptyReads struct {
	reads      int
	progressAt int
}

func (r *finiteEmptyReads) Read(p []byte) (int, error) {
	r.reads++
	if r.reads == r.progressAt || (r.progressAt != 0 && r.reads == 202) {
		p[0] = 'x'
		return 1, nil
	}
	limit := 102
	if r.progressAt != 0 {
		limit = 203
	}
	if r.reads == limit {
		return 0, io.EOF
	}
	return 0, nil
}

func TestEmptyReadGuardsFailOnRead101AndResetOnProgress(t *testing.T) {
	for _, lane := range []string{"drain", "entry", "file"} {
		for _, progress := range []bool{false, true} {
			t.Run(lane+"/"+map[bool]string{false: "stalled", true: "progress"}[progress], func(t *testing.T) {
				r := &finiteEmptyReads{}
				if progress {
					r.progressAt = 101
				}
				var err error
				switch lane {
				case "drain":
					a := &archive{bufferSize: 32}
					err, _ = a.drain(context.Background(), io.Discard, r)
				case "entry":
					z := zip.NewWriter(io.Discard)
					err = writeArchiveFile(context.Background(), z, "file", time.Time{}, r, make([]byte, 32))
					_ = z.Close()
				case "file":
					p := &payload{file: readerPayloadFile{Reader: r}, size: 2, bufferSize: 32}
					err = p.WriteTo(context.Background(), io.Discard)
				}
				if !progress {
					if transfer.ErrorCodeOf(err) != transfer.ErrTransferFailed || !errors.Is(err, io.ErrNoProgress) || r.reads != 101 {
						t.Fatalf("%s empty-read guard returned %v after %d reads; want transfer_failed wrapping ErrNoProgress at 101", lane, err, r.reads)
					}
				} else if err != nil {
					t.Fatalf("%s did not reset empty-read count after progress: %v", lane, err)
				}
			})
		}
	}
}

type readerPayloadFile struct{ io.Reader }

func (readerPayloadFile) Stat() (os.FileInfo, error) { return nil, nil }
func (readerPayloadFile) Close() error               { return nil }
