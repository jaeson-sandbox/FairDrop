//go:build windows

package source

import (
	"context"
	"encoding/binary"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"unicode/utf16"
	"unsafe"

	"fairdrop/internal/transfer"
	"golang.org/x/sys/windows"
)

func TestParseWindowsPathAcceptsSupportedVolumesAndRejectsNamespaces(t *testing.T) {
	t.Parallel()
	accepted := []struct {
		path       string
		anchor     string
		components []string
		label      string
		trailing   bool
	}{
		{path: `C:\folder\item`, anchor: `\??\C:\`, components: []string{"folder", "item"}, label: "C"},
		{path: `C:/folder/item`, anchor: `\??\C:\`, components: []string{"folder", "item"}, label: "C"},
		{path: `C:\folder\`, anchor: `\??\C:\`, components: []string{"folder"}, label: "C", trailing: true},
		{path: `\\server\share\folder`, anchor: `\??\UNC\server\share\`, components: []string{"folder"}, label: "share"},
		{path: `\\?\C:\folder`, anchor: `\??\C:\`, components: []string{"folder"}, label: "C"},
		{path: `\\?\UNC\server\share\folder`, anchor: `\??\UNC\server\share\`, components: []string{"folder"}, label: "share"},
	}
	for _, test := range accepted {
		plan, err := parseWindowsPath(test.path)
		if err != nil {
			t.Errorf("parseWindowsPath(%q) error = %v", test.path, err)
			continue
		}
		if plan.anchor != test.anchor || !slices.Equal(plan.components, test.components) ||
			plan.rootLabel != test.label || plan.hadTrailingSep != test.trailing {
			t.Errorf("parseWindowsPath(%q) = %+v, want anchor=%q components=%v label=%q trailing=%v",
				test.path, plan, test.anchor, test.components, test.label, test.trailing)
		}
	}

	for _, path := range []string{
		`C:relative`,
		`\rooted`,
		`\\.\C:\folder`,
		`\??\C:\folder`,
		`\\?\GLOBALROOT\Device\HarddiskVolume1`,
		`\\?\Volume{00000000-0000-0000-0000-000000000000}\folder`,
		`C:\folder\file:stream`,
		`C:\NUL`,
		`C:\CON`,
		`C:\PRN`,
		`C:\COM1`,
		`C:\LPT9`,
		`C:\CONIN$`,
		`C:\CONOUT$`,
		`C:\COM¹`,
		`C:\COM²`,
		`C:\COM³`,
		`C:\LPT¹`,
		`C:\LPT²`,
		`C:\LPT³`,
	} {
		if _, err := parseWindowsPath(path); err == nil {
			t.Errorf("parseWindowsPath(%q) succeeded, want rejection", path)
		}
	}
}

func TestWindowsHandleRightsSeparateMetadataSearchAndEnumeration(t *testing.T) {
	t.Parallel()
	if got := windowsMetadataAccess(); got != 0x00000080 {
		t.Fatalf("metadata access = %#x, want literal FILE_READ_ATTRIBUTES only", got)
	}
	if got := windowsSearchAccess(); got != 0x000000a0 {
		t.Fatalf("search access = %#x, want literal attributes plus traverse", got)
	}
	if got := windowsEnumerationAccess(); got != 0x00000081 {
		t.Fatalf("enumeration access = %#x, want literal attributes plus list", got)
	}
	if got := windowsContentAccess(); got != 0x00000081 {
		t.Fatalf("content access = %#x, want literal attributes plus read-data only", got)
	}
	if got := windowsAnyOptions(); got&0x00200000 == 0 {
		t.Fatalf("metadata open options = %#x, want literal FILE_OPEN_REPARSE_POINT", got)
	}
	if got := windowsDirectoryOptions(); got&0x00000001 == 0 || got&0x00200000 == 0 {
		t.Fatalf("directory open options = %#x, want literal FILE_DIRECTORY_FILE with FILE_OPEN_REPARSE_POINT", got)
	}
	// FILE_NON_DIRECTORY_FILE is what makes the kernel, rather than a later
	// check, refuse to hand back a readable directory handle.
	if got := windowsContentOptions(); got&0x00000040 == 0 || got&0x00000001 != 0 || got&0x00200000 == 0 {
		t.Fatalf("content open options = %#x, want literal FILE_NON_DIRECTORY_FILE with FILE_OPEN_REPARSE_POINT and no FILE_DIRECTORY_FILE", got)
	}
}

func TestPlatformReparsePointReadsWindowsAttributes(t *testing.T) {
	t.Parallel()

	info := fakeFileInfo{
		name:    "regular-mode-reparse",
		mode:    0,
		reparse: true,
		sys: &syscall.Win32FileAttributeData{
			FileAttributes: syscall.FILE_ATTRIBUTE_REPARSE_POINT,
		},
	}
	if !info.Mode().IsRegular() {
		t.Fatal("fixture must have regular Go mode")
	}
	reparse, err := platformReparsePoint(info)
	if err != nil {
		t.Fatalf("platformReparsePoint() error = %v", err)
	}
	if !reparse {
		t.Fatal("Windows reparse attribute was not detected")
	}
}

func TestPlatformUnreachableNetworkErrorsOverrideNotExist(t *testing.T) {
	t.Parallel()

	for _, errno := range []syscall.Errno{53, 67, 1203, 1231, 1232} {
		err := &os.PathError{Op: "Lstat", Path: `\\server\share\file`, Err: errno}
		if !platformUnreachableNetworkError(err) {
			t.Errorf("Win32 error %d was not classified as an unreachable network path", errno)
		}
	}
	if platformUnreachableNetworkError(fs.ErrNotExist) {
		t.Fatal("plain not-exist was incorrectly classified as an unreachable network path")
	}
}

func TestInspectRejectsNativeJunctionAncestor(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	target := filepath.Join(base, "target")
	junction := filepath.Join(base, "junction")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "child.txt"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := createJunction(junction, target); err != nil {
		requireWindowsJunctionCapability(t, err)
	}
	t.Cleanup(func() {
		// Remove the reparse point before TempDir cleanup reaches its target.
		_ = os.Remove(junction)
	})

	_, err := New().Inspect(context.Background(), filepath.Join(junction, "child.txt"))
	assertCode(t, err, transfer.ErrPathUnsupported)
}

func TestInspectRejectsNativeJunctionRootWithTrailingSeparator(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	target := filepath.Join(base, "target")
	junction := filepath.Join(base, "junction")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := createJunction(junction, target); err != nil {
		requireWindowsJunctionCapability(t, err)
	}
	t.Cleanup(func() { _ = os.Remove(junction) })

	_, err := New().Inspect(context.Background(), junction+string(os.PathSeparator))
	assertCode(t, err, transfer.ErrPathUnsupported)
}

func TestInspectRejectsNativeNestedJunction(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	root := filepath.Join(base, "selected")
	target := filepath.Join(base, "target")
	junction := filepath.Join(root, "nested-junction")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := createJunction(junction, target); err != nil {
		requireWindowsJunctionCapability(t, err)
	}
	t.Cleanup(func() { _ = os.Remove(junction) })

	_, err := New().Inspect(context.Background(), root)
	assertCode(t, err, transfer.ErrPathUnsupported)
}

func TestInspectRejectsNativeSelectedSymlink(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	target := filepath.Join(base, "target.txt")
	link := filepath.Join(base, "selected-link.txt")
	if err := os.WriteFile(target, []byte("target"), 0o600); err != nil {
		t.Fatal(err)
	}
	createNativeSymlinkOrSkip(t, target, link)

	_, err := New().Inspect(context.Background(), link)
	assertCode(t, err, transfer.ErrPathUnsupported)
}

func TestInspectRejectsNativeDanglingSymlink(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	link := filepath.Join(base, "dangling-link.txt")
	createNativeSymlinkOrSkip(t, filepath.Join(base, "missing.txt"), link)

	_, err := New().Inspect(context.Background(), link)
	assertCode(t, err, transfer.ErrPathUnsupported)
}

func TestInspectRejectsNativeAncestorSymlink(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	target := filepath.Join(base, "target")
	link := filepath.Join(base, "ancestor-link")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "child.txt"), []byte("child"), 0o600); err != nil {
		t.Fatal(err)
	}
	createNativeSymlinkOrSkip(t, target, link)

	_, err := New().Inspect(context.Background(), filepath.Join(link, "child.txt"))
	assertCode(t, err, transfer.ErrPathUnsupported)
}

func TestInspectRejectsNativeNULDevice(t *testing.T) {
	t.Parallel()

	volume := filepath.VolumeName(t.TempDir())
	path := volume + string(os.PathSeparator) + "NUL"
	_, err := New().Inspect(context.Background(), path)
	assertCode(t, err, transfer.ErrPathUnsupported)
}

func TestInspectPreservesLongWindowsPath(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	for len(directory) < 280 {
		directory = filepath.Join(directory, "long-path-segment")
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		requireWindowsLongPathCapability(t, "create long directory", err)
	}
	path := filepath.Join(directory, "file.txt")
	if err := os.WriteFile(path, []byte("long"), 0o600); err != nil {
		requireWindowsLongPathCapability(t, "write long file", err)
	}

	item, err := New().Inspect(context.Background(), path)
	if err != nil {
		t.Fatalf("Inspect(long path) error = %v", err)
	}
	if item.Path != path {
		t.Fatalf("Path = %q, want byte-identical %q", item.Path, path)
	}

	directoryItem, err := New().Inspect(context.Background(), directory)
	if err != nil {
		t.Fatalf("Inspect(long directory path) error = %v", err)
	}
	if directoryItem.Path != directory || directoryItem.Kind != transfer.ItemDirectory {
		t.Fatalf("directory metadata = %+v, want byte-identical directory path", directoryItem)
	}
}

func TestInspectPreservesExtendedLengthWindowsPath(t *testing.T) {
	t.Parallel()

	ordinary := filepath.Join(t.TempDir(), "extended.txt")
	if err := os.WriteFile(ordinary, []byte("extended"), 0o600); err != nil {
		t.Fatal(err)
	}
	extended := extendedLengthPath(ordinary)
	if _, err := os.Lstat(extended); err != nil {
		requireWindowsLongPathCapability(t, "inspect extended file", err)
	}

	item, err := New().Inspect(context.Background(), extended)
	if err != nil {
		t.Fatalf("Inspect(extended path) error = %v", err)
	}
	if item.Path != extended {
		t.Fatalf("Path = %q, want byte-identical %q", item.Path, extended)
	}

	ordinaryDirectory := filepath.Join(t.TempDir(), "extended-directory")
	if err := os.Mkdir(ordinaryDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	extendedDirectory := extendedLengthPath(ordinaryDirectory)
	if _, err := os.Lstat(extendedDirectory); err != nil {
		requireWindowsLongPathCapability(t, "inspect extended directory", err)
	}
	for _, selected := range []string{extendedDirectory, extendedDirectory + string(os.PathSeparator)} {
		directoryItem, err := New().Inspect(context.Background(), selected)
		if err != nil {
			t.Fatalf("Inspect(extended directory %q) error = %v", selected, err)
		}
		if directoryItem.Path != selected || directoryItem.Kind != transfer.ItemDirectory {
			t.Fatalf("directory metadata = %+v, want byte-identical extended directory %q", directoryItem, selected)
		}
	}
}

func TestInspectPreservesReachableUNCPath(t *testing.T) {
	path := os.Getenv("FAIRDROP_TEST_UNC_FILE")
	if path == "" {
		t.Skip("FAIRDROP_TEST_UNC_FILE does not name a reachable UNC fixture")
	}
	if !strings.HasPrefix(path, `\\`) {
		t.Fatalf("FAIRDROP_TEST_UNC_FILE = %q, want a UNC path", path)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Skipf("configured UNC fixture is not reachable: %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("configured UNC fixture mode = %v, want a regular file", info.Mode())
	}

	item, err := New().Inspect(context.Background(), path)
	if err != nil {
		t.Fatalf("Inspect(UNC path) error = %v", err)
	}
	if item.Path != path {
		t.Fatalf("Path = %q, want byte-identical %q", item.Path, path)
	}
}

func TestInspectPreservesReachableUNCDirectory(t *testing.T) {
	path := os.Getenv("FAIRDROP_TEST_UNC_DIRECTORY")
	if path == "" {
		t.Skip("FAIRDROP_TEST_UNC_DIRECTORY does not name a reachable UNC fixture")
	}
	if !strings.HasPrefix(path, `\\`) {
		t.Fatalf("FAIRDROP_TEST_UNC_DIRECTORY = %q, want a UNC path", path)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Skipf("configured UNC fixture is not reachable: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("configured UNC fixture mode = %v, want a directory", info.Mode())
	}

	item, err := New().Inspect(context.Background(), path)
	if err != nil {
		t.Fatalf("Inspect(UNC directory) error = %v", err)
	}
	if item.Path != path || item.Kind != transfer.ItemDirectory {
		t.Fatalf("directory metadata = %+v, want byte-identical UNC directory", item)
	}
}

func TestInspectLoopbackAdministrativeShareDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "unc-selected")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.bin"), []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "b.bin"), []byte("12345"), 0o600); err != nil {
		t.Fatal(err)
	}
	volume := filepath.VolumeName(root)
	relative, err := filepath.Rel(volume+string(os.PathSeparator), root)
	if err != nil {
		t.Fatal(err)
	}
	unc := `\\localhost\` + strings.TrimSuffix(volume, ":") + "$\\" + relative
	if _, err := os.Stat(unc); err != nil {
		t.Skipf("loopback administrative share unavailable: %v", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	item, err := New().Inspect(context.Background(), unc)
	if err != nil {
		t.Fatalf("Inspect(reachable loopback administrative share) error = %v", err)
	}
	if item.Path != unc || item.Name != "unc-selected" || item.Kind != transfer.ItemDirectory ||
		item.LogicalSize != 8 || !item.ModTime.Equal(info.ModTime()) {
		t.Fatalf("UNC item = %+v, want complete directory metadata", item)
	}
}

func TestNativeChildLookupStaysRelativeToOpenedParentAfterRename(t *testing.T) {
	base := t.TempDir()
	selected := filepath.Join(base, "selected")
	if err := os.Mkdir(selected, 0o700); err != nil {
		t.Fatal(err)
	}
	kept := filepath.Join(selected, "kept")
	if err := os.Mkdir(kept, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(kept, "payload.txt"), []byte("kept"), 0o600); err != nil {
		t.Fatal(err)
	}

	plan, err := parseWindowsPath(selected)
	if err != nil {
		t.Fatal(err)
	}
	factory := nativeHandleFactory{}
	anchor, err := factory.OpenAnchor(plan)
	if err != nil {
		t.Fatal(err)
	}
	handles := []metadataHandle{anchor}
	defer func() { _ = closeMetadataHandles(context.Background(), handles, nil) }()
	current := anchor
	for _, component := range plan.components {
		metadata, err := current.OpenChildMetadata(component)
		if err != nil {
			t.Fatal(err)
		}
		search, err := metadata.OpenSearch()
		_ = metadata.Close()
		if err != nil {
			t.Fatal(err)
		}
		handles = append(handles, search)
		current = search
	}

	renamed := filepath.Join(base, "renamed")
	if err := os.Rename(selected, renamed); err != nil {
		t.Skipf("host could not rename an opened directory: %v", err)
	}
	child, err := current.OpenChildMetadata("kept")
	if err != nil {
		t.Fatalf("parent-relative child open after rename failed: %v", err)
	}
	defer child.Close()
	info, err := child.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if info.Name() != "kept" || !info.IsDir() {
		t.Fatalf("child metadata = %q/%v, want kept directory", info.Name(), info.Mode())
	}
	directory, err := child.OpenEnumeration()
	if err != nil {
		t.Fatalf("parent-relative directory open after rename failed: %v", err)
	}
	defer directory.Close()
	entries, err := directory.ReadDir(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "payload.txt" {
		t.Fatalf("directory entries = %+v, want payload.txt", entries)
	}
}

func TestInspectOrdinaryFileThroughTraverseOnlyAncestor(t *testing.T) {
	account, err := user.Current()
	if err != nil || account.Username == "" {
		t.Skipf("current Windows account is unavailable: %v", err)
	}
	ancestor := filepath.Join(t.TempDir(), "traverse-only")
	if err := os.Mkdir(ancestor, 0o700); err != nil {
		t.Fatal(err)
	}
	selected := filepath.Join(ancestor, "ordinary.txt")
	if err := os.WriteFile(selected, []byte("ordinary"), 0o600); err != nil {
		t.Fatal(err)
	}

	// The direct ACL fixture tool expresses traverse/read-attributes without
	// directory-list rights. Production inspection never changes ACLs. Restore
	// full control before TempDir cleanup.
	fileGrant := account.Username + ":(RA,S)"
	if output, err := exec.Command("icacls.exe", selected, "/inheritance:r", "/grant:r", fileGrant).CombinedOutput(); err != nil {
		t.Skipf("metadata-only file ACL fixture unavailable: %v (%s)", err, strings.TrimSpace(string(output)))
	}
	grant := account.Username + ":(X,RA,S)"
	output, err := exec.Command("icacls.exe", ancestor, "/inheritance:r", "/grant:r", grant).CombinedOutput()
	if err != nil {
		t.Skipf("traverse-only ACL fixture unavailable: %v (%s)", err, strings.TrimSpace(string(output)))
	}
	t.Cleanup(func() {
		_, _ = exec.Command("icacls.exe", ancestor, "/grant:r", account.Username+":(F)", "/inheritance:e").CombinedOutput()
	})
	if _, err := os.ReadDir(ancestor); err == nil {
		t.Skip("host privileges bypass the fixture's directory-list restriction")
	}
	item, err := New().Inspect(context.Background(), selected)
	if err != nil {
		t.Fatalf("Inspect(file through traverse-only ancestor) error = %v", err)
	}
	if item.Kind != transfer.ItemFile || item.Name != "ordinary.txt" || item.LogicalSize != 8 {
		t.Fatalf("item = %+v, want ordinary file metadata", item)
	}
}

func extendedLengthPath(path string) string {
	if strings.HasPrefix(path, `\\`) {
		return `\\?\UNC\` + strings.TrimPrefix(path, `\\`)
	}
	return `\\?\` + path
}

func createNativeSymlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		if isWindowsCapabilityError(err, syscall.Errno(5), syscall.Errno(50), syscall.Errno(1314)) {
			t.Skipf("native symlink capability unavailable: %v", err)
		}
		t.Fatalf("create native symlink: %v", err)
	}
}

func requireWindowsJunctionCapability(t *testing.T, err error) {
	t.Helper()
	if isWindowsCapabilityError(err, syscall.Errno(1), syscall.Errno(5), syscall.Errno(50), syscall.Errno(1314)) {
		t.Skipf("native junction capability unavailable: %v", err)
	}
	t.Fatalf("create native junction fixture: %v", err)
}

func requireWindowsLongPathCapability(t *testing.T, operation string, err error) {
	t.Helper()
	if isWindowsCapabilityError(err, syscall.Errno(50), syscall.Errno(123), syscall.Errno(206)) {
		t.Skipf("native long-path capability unavailable during %s: %v", operation, err)
	}
	t.Fatalf("%s fixture: %v", operation, err)
}

func isWindowsCapabilityError(err error, allowed ...syscall.Errno) bool {
	for _, code := range allowed {
		if errors.Is(err, code) {
			return true
		}
	}
	return false
}

func createJunction(junction, target string) error {
	if err := os.Mkdir(junction, 0o700); err != nil {
		return err
	}
	path, err := windows.UTF16PtrFromString(junction)
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(
		path,
		windows.GENERIC_WRITE,
		0,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)

	substitute := utf16.Encode([]rune(`\??\` + target))
	printName := utf16.Encode([]rune(target))
	pathWords := append(append(append([]uint16{}, substitute...), 0), printName...)
	pathWords = append(pathWords, 0)
	pathBytes := make([]byte, len(pathWords)*2)
	for index, word := range pathWords {
		binary.LittleEndian.PutUint16(pathBytes[index*2:], word)
	}

	dataLength := 8 + len(pathBytes)
	buffer := make([]byte, 8+dataLength)
	binary.LittleEndian.PutUint32(buffer[0:], windows.IO_REPARSE_TAG_MOUNT_POINT)
	binary.LittleEndian.PutUint16(buffer[4:], uint16(dataLength))
	binary.LittleEndian.PutUint16(buffer[8:], 0)
	binary.LittleEndian.PutUint16(buffer[10:], uint16(len(substitute)*2))
	binary.LittleEndian.PutUint16(buffer[12:], uint16((len(substitute)+1)*2))
	binary.LittleEndian.PutUint16(buffer[14:], uint16(len(printName)*2))
	copy(buffer[16:], pathBytes)

	return windows.DeviceIoControl(
		handle,
		windows.FSCTL_SET_REPARSE_POINT,
		(*byte)(unsafe.Pointer(&buffer[0])),
		uint32(len(buffer)),
		nil,
		0,
		nil,
		nil,
	)
}
