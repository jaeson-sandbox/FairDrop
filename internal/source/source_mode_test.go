package source

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fairdrop/internal/transfer"
)

// This file covers the classification decision itself, driven through the real
// Inspect via the lstat seam.
//
// TestInspectRejectsSymlink next door is the honest integration test, but
// creating a symlink needs elevation or Developer Mode on Windows, so it skips
// on an ordinary runner -- and a matrix row whose only test skips is a row with
// no coverage. These tests always run: they synthesize the fs.FileInfo that a
// real Lstat would have returned and assert the same rejection.

// stubFileInfo is a synthetic fs.FileInfo. Only Mode and Size steer Inspect;
// the rest exist to satisfy the interface.
type stubFileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
}

func (s stubFileInfo) Name() string       { return s.name }
func (s stubFileInfo) Size() int64        { return s.size }
func (s stubFileInfo) Mode() fs.FileMode  { return s.mode }
func (s stubFileInfo) ModTime() time.Time { return s.modTime }
func (s stubFileInfo) IsDir() bool        { return s.mode.IsDir() }
func (s stubFileInfo) Sys() any           { return nil }

// inspectorReturning builds an Inspector whose Lstat always reports info for
// the one path it is asked about.
func inspectorReturning(t *testing.T, info fs.FileInfo) *Inspector {
	t.Helper()
	return &Inspector{
		lstat: func(string) (fs.FileInfo, error) { return info, nil },
	}
}

// TestInspectRejectsLinkAndSpecialModes is the always-running counterpart to
// TestInspectRejectsSymlink.
//
// The paths handed to Inspect are absolute but do not exist on disk. That is
// deliberate: if the seam were not wired into Inspect, os.Lstat would run and
// every case would fail with path_not_found instead of the expected code, so a
// passing run proves the fake actually drove the decision.
func TestInspectRejectsLinkAndSpecialModes(t *testing.T) {
	cases := []struct {
		name string
		mode fs.FileMode
	}{
		// The rows TestInspectRejectsSymlink cannot reach unprivileged. Lstat
		// describes the link itself, so a symlink to a perfectly good regular
		// file still has ModeSymlink -- and a dangling one is indistinguishable
		// at this layer, which is exactly why neither may be followed.
		{"symlink-to-file", os.ModeSymlink | 0o644},
		{"symlink-to-directory", os.ModeSymlink | os.ModeDir | 0o755},
		{"symlink-dangling", os.ModeSymlink | 0o777},

		// A Windows junction: measured as ModeIrregular with ModeSymlink
		// clear, which is why the adapter tests IsRegular instead of the
		// symlink bit.
		{"junction-irregular", os.ModeIrregular},

		{"directory", os.ModeDir | 0o755},
		{"device", os.ModeDevice | 0o666},
		{"char-device", os.ModeDevice | os.ModeCharDevice | 0o666},
		{"named-pipe", os.ModeNamedPipe | 0o666},
		{"socket", os.ModeSocket | 0o666},
		{"setuid-non-regular", os.ModeSymlink | os.ModeSetuid | 0o755},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "never-created-on-disk.txt")
			info := stubFileInfo{
				name:    "never-created-on-disk.txt",
				size:    1234,
				mode:    tc.mode,
				modTime: time.Now(),
			}

			item, err := inspectorReturning(t, info).Inspect(context.Background(), path)

			if transfer.ErrorCodeOf(err) == transfer.ErrPathNotFound {
				t.Fatalf("got path_not_found: the lstat seam was bypassed, so this test proves nothing (err: %v)", err)
			}
			requireCode(t, item, err, transfer.ErrPathUnsupported)
			requireNoDisclosure(t, err, path, "never-created-on-disk.txt")
		})
	}
}

// TestInspectAcceptsRegularModeThroughSeam is the control. Without it the table
// above would still pass if Inspect rejected everything unconditionally.
func TestInspectAcceptsRegularModeThroughSeam(t *testing.T) {
	path := filepath.Join(t.TempDir(), "never-created-on-disk.txt")
	modTime := time.Now().Add(-time.Hour).Truncate(time.Second)
	info := stubFileInfo{name: "never-created-on-disk.txt", size: 4096, mode: 0o644, modTime: modTime}

	item, err := inspectorReturning(t, info).Inspect(context.Background(), path)
	if err != nil {
		t.Fatalf("Inspect on a regular mode: %v", err)
	}

	if item.Path != path {
		t.Errorf("Path = %q, want %q", item.Path, path)
	}
	if item.Name != "never-created-on-disk.txt" {
		t.Errorf("Name = %q, want %q", item.Name, "never-created-on-disk.txt")
	}
	if item.Kind != transfer.ItemFile {
		t.Errorf("Kind = %q, want %q", item.Kind, transfer.ItemFile)
	}
	if item.LogicalSize != 4096 {
		t.Errorf("LogicalSize = %d, want 4096", item.LogicalSize)
	}
	if !item.ModTime.Equal(modTime) {
		t.Errorf("ModTime = %v, want %v", item.ModTime, modTime)
	}
}

// TestInspectClassifiesLstatFailures pins the two error branches
// deterministically, without depending on a host that can produce each failure.
func TestInspectClassifiesLstatFailures(t *testing.T) {
	cases := []struct {
		name  string
		cause error
		want  transfer.ErrorCode
	}{
		{"not-exist", fs.ErrNotExist, transfer.ErrPathNotFound},
		{"wrapped-not-exist", errors.Join(errors.New("stat failed"), fs.ErrNotExist), transfer.ErrPathNotFound},
		{"permission", fs.ErrPermission, transfer.ErrPathUnsupported},
		{"invalid-name", errors.New("The filename, directory name, or volume label syntax is incorrect."), transfer.ErrPathUnsupported},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "never-created-on-disk.txt")
			inspector := &Inspector{
				lstat: func(string) (fs.FileInfo, error) { return nil, tc.cause },
			}

			item, err := inspector.Inspect(context.Background(), path)

			requireCode(t, item, err, tc.want)
			requireNoDisclosure(t, err, path, "never-created-on-disk.txt")
			if !errors.Is(err, tc.cause) {
				t.Error("the Lstat cause was not wrapped: local diagnosis would lose it")
			}
		})
	}
}

// TestSeamDefaultsToOsLstat: a nil seam must mean the real filesystem, or
// production would silently inspect nothing.
func TestSeamDefaultsToOsLstat(t *testing.T) {
	path := writeFile(t, t.TempDir(), "real.txt", []byte("on disk"))

	for name, inspector := range map[string]*Inspector{"New": New(), "zero-value": {}} {
		t.Run(name, func(t *testing.T) {
			if inspector.lstat != nil {
				t.Fatal("lstat is non-nil: production must default to os.Lstat")
			}
			item, err := inspector.Inspect(context.Background(), path)
			if err != nil {
				t.Fatalf("Inspect: %v", err)
			}
			if item.LogicalSize != int64(len("on disk")) {
				t.Errorf("LogicalSize = %d, want %d: the real file was not read", item.LogicalSize, len("on disk"))
			}
		})
	}
}

// TestSeamIsNotConsultedBeforeGuards: the cheap guards still come first, so a
// cancelled or malformed request never reaches the filesystem at all.
func TestSeamIsNotConsultedBeforeGuards(t *testing.T) {
	calls := 0
	inspector := &Inspector{
		lstat: func(string) (fs.FileInfo, error) {
			calls++
			return stubFileInfo{mode: 0o644}, nil
		},
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	cases := []struct {
		name string
		ctx  context.Context
		path string
		want transfer.ErrorCode
	}{
		{"cancelled", cancelled, filepath.Join(t.TempDir(), "x.txt"), transfer.ErrCancelled},
		{"empty", context.Background(), "", transfer.ErrInvalidSelection},
		{"relative", context.Background(), "source.go", transfer.ErrInvalidSelection},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item, err := inspector.Inspect(tc.ctx, tc.path)
			requireCode(t, item, err, tc.want)
		})
	}

	if calls != 0 {
		t.Errorf("lstat was called %d times, want 0: a guard let a request through to the filesystem", calls)
	}
}
