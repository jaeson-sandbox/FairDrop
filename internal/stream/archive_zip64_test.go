package stream

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"testing"
	"time"

	"fairdrop/internal/transfer"
)

func TestArchiveZIP64EntryCountReadsBackEveryEntry(t *testing.T) {
	const entries = 65536
	walker := &scriptedSource{walk: func(_ context.Context, _ string, visit transfer.SourceVisitor) error {
		for index := range entries {
			body := fmt.Sprintf("payload-%06d", index)
			if err := visit(transfer.SourceEntry{RelativePath: fmt.Sprintf("f%06d.txt", index), Kind: transfer.ItemFile, Size: int64(len(body))}, bytes.NewBufferString(body)); err != nil {
				return err
			}
		}
		return nil
	}}
	prepared := newTestArchive(t, walker, "root")
	var destination bytes.Buffer
	if err := prepared.WriteTo(context.Background(), &destination); err != nil {
		t.Fatal(err)
	}
	reader := openArchive(t, destination.Bytes())
	if len(reader.File) != 65537 {
		t.Fatalf("ZIP entry count=%d, want 65537 including root", len(reader.File))
	}
	if !bytes.Contains(destination.Bytes(), []byte{'P', 'K', 6, 6}) {
		t.Fatal("past-threshold archive omitted ZIP64 end record")
	}
	for index, entry := range reader.File {
		if index == 0 {
			if entry.Name != "root/" {
				t.Fatal("missing root entry")
			}
			continue
		}
		if entry.Name != fmt.Sprintf("root/f%06d.txt", index-1) {
			t.Fatalf("entry %d has wrong name/order", index)
		}
		content, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(content)
		content.Close()
		if err != nil || string(body) != fmt.Sprintf("payload-%06d", index-1) {
			t.Fatalf("entry %d content or CRC failed", index)
		}
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

type validateZeroWriter struct{}

func (validateZeroWriter) Write(p []byte) (int, error) {
	if bytes.Count(p, []byte{0}) != len(p) {
		return 0, fmt.Errorf("large entry contains nonzero bytes")
	}
	return len(p), nil
}

func TestArchiveZIP64FourGiBEntryAndTotalReadBack(t *testing.T) {
	// A real >4 GiB stream traverses the production Deflate writer. Zeros
	// compress to a few MiB, so neither the fixture nor the result needs a
	// multi-gigabyte allocation or file. Readback reaches EOF to verify CRC.
	const largeSize int64 = (1 << 32) + 1
	walker := &scriptedSource{walk: func(_ context.Context, _ string, visit transfer.SourceVisitor) error {
		if err := visit(transfer.SourceEntry{RelativePath: "large.bin", Kind: transfer.ItemFile, Size: largeSize}, io.LimitReader(zeroReader{}, largeSize)); err != nil {
			return err
		}
		return visit(transfer.SourceEntry{RelativePath: "tail.bin", Kind: transfer.ItemFile, Size: 17}, io.LimitReader(zeroReader{}, 17))
	}}
	prepared := newTestArchive(t, walker, "root")
	var destination bytes.Buffer
	if err := prepared.WriteTo(context.Background(), &destination); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(destination.Bytes()), int64(destination.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.File) != 3 {
		t.Fatalf("entry count=%d, want root and two files", len(reader.File))
	}
	var total int64
	for index, entry := range reader.File {
		if index == 0 {
			if entry.Name != "root/" {
				t.Fatal("missing root")
			}
			continue
		}
		want := int64(17)
		if index == 1 {
			want = 4294967297
			if entry.Name != "root/large.bin" || entry.ReaderVersion < 45 {
				t.Fatal("large entry lacks ZIP64 metadata")
			}
		} else if entry.Name != "root/tail.bin" {
			t.Fatal("large-entry archive tail missing")
		}
		if entry.UncompressedSize64 != uint64(want) {
			t.Fatalf("entry %d size=%d, want %d", index, entry.UncompressedSize64, want)
		}
		content, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		read, err := io.Copy(validateZeroWriter{}, content)
		content.Close()
		if err != nil || read != want {
			t.Fatalf("entry %d readback=%d, want %d: %v", index, read, want, err)
		}
		total += read
	}
	if total != 4294967314 {
		t.Fatalf("archive logical total=%d, want 4294967314", total)
	}
}

// virtualPrefix supplies a sparse zero prefix without allocating or writing it.
// This exercises ZIP64 offsets, not a multi-GiB compressed payload stream.
type virtualPrefix struct {
	prefix int64
	tail   []byte
}

func (v virtualPrefix) ReadAt(p []byte, offset int64) (int, error) {
	if offset < 0 {
		return 0, fmt.Errorf("negative offset")
	}
	total := 0
	if offset < v.prefix {
		count := min(int64(len(p)), v.prefix-offset)
		clear(p[:count])
		p, offset, total = p[count:], offset+count, int(count)
	}
	if len(p) == 0 {
		return total, nil
	}
	offset -= v.prefix
	if offset >= int64(len(v.tail)) {
		return total, io.EOF
	}
	n := copy(p, v.tail[offset:])
	if n != len(p) {
		return total + n, io.EOF
	}
	return total + n, nil
}

func TestArchiveZIP64OffsetsBeyondFourGiB(t *testing.T) {
	const prefix int64 = (1 << 32) + 4096
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	writer.SetOffset(prefix)
	for _, name := range []string{"first.txt", "tail.txt"} {
		if err := writeArchiveFile(context.Background(), writer, name, time.Time{}, bytes.NewBufferString("offset payload"), make([]byte, 32)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	data := output.Bytes()
	end := bytes.LastIndex(data, []byte{'P', 'K', 6, 6})
	if end < 0 || end+56 > len(data) {
		t.Fatal("offset threshold omitted ZIP64 end record")
	}
	directoryOffset := binary.LittleEndian.Uint64(data[end+48 : end+56])
	if directoryOffset <= 4294967295 {
		t.Fatal("central-directory offset lost its high bits")
	}
	relative := directoryOffset - uint64(prefix)
	if relative+4 > uint64(len(data)) || !bytes.Equal(data[relative:relative+4], []byte{'P', 'K', 1, 2}) {
		t.Fatal("ZIP64 central-directory offset points at the wrong bytes")
	}
	reader, err := zip.NewReader(virtualPrefix{prefix: prefix, tail: data}, prefix+int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.File) != 2 {
		t.Fatal("offset archive lost subsequent entry")
	}
	lastOffset := prefix
	for index, entry := range reader.File {
		if entry.Name != []string{"first.txt", "tail.txt"}[index] {
			t.Fatal("offset archive entry order changed")
		}
		offset, err := entry.DataOffset()
		if err != nil || offset <= lastOffset || offset <= 4294967295 {
			t.Fatal("entry offset lost high bits or ordering")
		}
		lastOffset = offset
		content, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(content)
		_ = content.Close()
		if err != nil || string(body) != "offset payload" {
			t.Fatal("offset entry readback or CRC failed")
		}
	}
}
