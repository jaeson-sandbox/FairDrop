//go:build windows

package source

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"fairdrop/internal/transfer"
	"golang.org/x/sys/windows"
)

type nativeHandleFactory struct{}

type windowsLocator struct {
	parent   *windowsNode
	absolute string
	name     string
}

type windowsNode struct {
	file    *os.File
	locator windowsLocator
	list    bool
}

func (nativeHandleFactory) Parse(path string) (pathPlan, error) {
	if path == "" || strings.IndexByte(path, 0) >= 0 {
		return pathPlan{}, transfer.NewError(transfer.ErrInvalidSelection, "selection must be one absolute path")
	}
	return parseWindowsPath(path)
}

func parseWindowsPath(path string) (pathPlan, error) {
	lower := strings.ToLower(path)
	if strings.HasPrefix(lower, "\\\\.\\") || strings.HasPrefix(lower, "\\??\\") ||
		strings.HasPrefix(lower, "\\\\?\\globalroot") || strings.HasPrefix(lower, "\\\\?\\device\\") {
		return pathPlan{}, transfer.NewError(transfer.ErrPathUnsupported, "selection uses an unsupported Windows namespace")
	}

	var anchor string
	var label string
	var rest string
	switch {
	case strings.HasPrefix(lower, "\\\\?\\unc\\"):
		server, share, tail, ok := splitUNC(path[len("\\\\?\\UNC\\"):])
		if !ok || !validWindowsComponent(server) || !validWindowsComponent(share) {
			return pathPlan{}, transfer.NewError(transfer.ErrPathUnsupported, "selection UNC root is unsupported")
		}
		anchor = "\\??\\UNC\\" + server + "\\" + share + "\\"
		label, rest = share, tail
	case strings.HasPrefix(lower, "\\\\?\\"):
		tail := path[len("\\\\?\\"):]
		if len(tail) < 3 || !isDriveLetter(tail[0]) || tail[1] != ':' || !isWindowsSeparator(tail[2]) {
			return pathPlan{}, transfer.NewError(transfer.ErrPathUnsupported, "selection extended path is unsupported")
		}
		anchor = "\\??\\" + tail[:2] + "\\"
		label, rest = tail[:1], tail[3:]
	case strings.HasPrefix(path, "\\\\") || strings.HasPrefix(path, "//"):
		server, share, tail, ok := splitUNC(path[2:])
		if !ok || !validWindowsComponent(server) || !validWindowsComponent(share) {
			return pathPlan{}, transfer.NewError(transfer.ErrPathUnsupported, "selection UNC root is unsupported")
		}
		anchor = "\\??\\UNC\\" + server + "\\" + share + "\\"
		label, rest = share, tail
	default:
		if len(path) < 3 || !isDriveLetter(path[0]) || path[1] != ':' || !isWindowsSeparator(path[2]) {
			return pathPlan{}, transfer.NewError(transfer.ErrInvalidSelection, "selection must be one absolute path")
		}
		anchor = "\\??\\" + path[:2] + "\\"
		label, rest = path[:1], path[3:]
	}

	components, trailing, ok := splitWindowsComponents(rest)
	if !ok {
		return pathPlan{}, transfer.NewError(transfer.ErrPathUnsupported, "selection contains an unsupported Windows component")
	}
	return pathPlan{anchor: anchor, components: components, rootLabel: label, hadTrailingSep: trailing}, nil
}

func splitUNC(path string) (server, share, rest string, ok bool) {
	first := indexWindowsSeparator(path)
	if first <= 0 {
		return "", "", "", false
	}
	server = path[:first]
	path = strings.TrimLeftFunc(path[first:], func(r rune) bool { return r == '\\' || r == '/' })
	second := indexWindowsSeparator(path)
	if second < 0 {
		if path == "" {
			return "", "", "", false
		}
		return server, path, "", true
	}
	if second == 0 {
		return "", "", "", false
	}
	share = path[:second]
	rest = strings.TrimLeftFunc(path[second:], func(r rune) bool { return r == '\\' || r == '/' })
	return server, share, rest, true
}

func splitWindowsComponents(rest string) ([]string, bool, bool) {
	trailing := len(rest) > 0 && isWindowsSeparator(rest[len(rest)-1])
	components := make([]string, 0, 4)
	start := 0
	for index := 0; index <= len(rest); index++ {
		if index < len(rest) && !isWindowsSeparator(rest[index]) {
			continue
		}
		if index > start {
			component := rest[start:index]
			if component != "." && component != ".." && !validWindowsComponent(component) {
				return nil, false, false
			}
			components = append(components, component)
		}
		start = index + 1
	}
	return components, trailing, true
}

func validWindowsComponent(component string) bool {
	return component != "" && !strings.Contains(component, ":") && filepath.IsLocal(component)
}

func indexWindowsSeparator(path string) int { return strings.IndexAny(path, "\\/") }
func isWindowsSeparator(value byte) bool    { return value == '\\' || value == '/' }
func isDriveLetter(value byte) bool {
	return value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func (nativeHandleFactory) OpenAnchor(plan pathPlan) (metadataHandle, error) {
	return openWindowsNode(windowsLocator{absolute: plan.anchor}, windowsSearchAccess(), windowsDirectoryOptions(), false)
}

func (n *windowsNode) Stat() (fs.FileInfo, error) {
	if n == nil || n.file == nil {
		return nil, fs.ErrClosed
	}
	return n.file.Stat()
}

func (n *windowsNode) OpenChildMetadata(name string) (metadataHandle, error) {
	return openWindowsNode(windowsLocator{parent: n, name: name}, windowsMetadataAccess(), windowsAnyOptions(), false)
}

func (n *windowsNode) OpenSearch() (metadataHandle, error) {
	return openWindowsNode(n.locator, windowsSearchAccess(), windowsDirectoryOptions(), false)
}

func (n *windowsNode) OpenEnumeration() (directoryHandle, error) {
	return openWindowsNode(n.locator, windowsEnumerationAccess(), windowsDirectoryOptions(), true)
}

// OpenChildContent is the one read-capable open in the package. It stays
// relative to this already-open directory and keeps FILE_OPEN_REPARSE_POINT so
// a name swapped for a junction or symlink is opened as the link itself rather
// than followed; FILE_NON_DIRECTORY_FILE makes the kernel refuse a directory
// outright, so read access is never granted to one.
func (n *windowsNode) OpenChildContent(name string) (contentHandle, error) {
	file, err := openWindowsFile(windowsLocator{parent: n, name: name}, windowsContentAccess(), windowsContentOptions())
	if err != nil {
		return nil, err
	}
	return &windowsContent{file: file}, nil
}

// windowsContent is a read-only view of one opened regular file. It is a
// separate type from windowsNode precisely so that no metadata, search, or
// enumeration handle carries a Read at all.
type windowsContent struct{ file *os.File }

func (c *windowsContent) Read(p []byte) (int, error) {
	if c == nil || c.file == nil {
		return 0, fs.ErrClosed
	}
	return c.file.Read(p)
}

func (c *windowsContent) Stat() (fs.FileInfo, error) {
	if c == nil || c.file == nil {
		return nil, fs.ErrClosed
	}
	return c.file.Stat()
}

func (c *windowsContent) Close() error {
	if c == nil || c.file == nil {
		return nil
	}
	err := c.file.Close()
	c.file = nil
	return err
}

func (n *windowsNode) ReadDir(count int) ([]fs.DirEntry, error) {
	if n == nil || n.file == nil || !n.list {
		return nil, fs.ErrPermission
	}
	return n.file.ReadDir(count)
}

func (n *windowsNode) Close() error {
	if n == nil || n.file == nil {
		return nil
	}
	err := n.file.Close()
	n.file = nil
	return err
}

func openWindowsNode(locator windowsLocator, access, options uint32, list bool) (*windowsNode, error) {
	file, err := openWindowsFile(locator, access, options)
	if err != nil {
		return nil, err
	}
	return &windowsNode{file: file, locator: locator, list: list}, nil
}

func openWindowsFile(locator windowsLocator, access, options uint32) (*os.File, error) {
	var objectName *windows.NTUnicodeString
	var err error
	oa := &windows.OBJECT_ATTRIBUTES{Attributes: windows.OBJ_CASE_INSENSITIVE}
	if locator.parent != nil {
		if locator.parent.file == nil {
			return nil, fs.ErrClosed
		}
		oa.RootDirectory = windows.Handle(locator.parent.file.Fd())
		objectName, err = windows.NewNTUnicodeString(locator.name)
	} else {
		objectName, err = windows.NewNTUnicodeString(locator.absolute)
	}
	if err != nil {
		return nil, err
	}
	oa.ObjectName = objectName
	oa.Length = uint32(unsafe.Sizeof(*oa))
	var handle windows.Handle
	var status windows.IO_STATUS_BLOCK
	err = windows.NtCreateFile(
		&handle,
		access|windows.SYNCHRONIZE,
		oa,
		&status,
		nil,
		0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		windows.FILE_OPEN,
		options,
		0,
		0,
	)
	if err != nil {
		return nil, windowsOperationError(err)
	}
	displayName := locator.name
	if displayName == "" {
		displayName = "root"
	}
	file := os.NewFile(uintptr(handle), displayName)
	if file == nil {
		_ = windows.CloseHandle(handle)
		return nil, errors.New("Windows handle could not be wrapped")
	}
	return file, nil
}

func windowsMetadataAccess() uint32 {
	return windows.FILE_READ_ATTRIBUTES
}

func windowsSearchAccess() uint32 {
	return windows.FILE_READ_ATTRIBUTES | windows.FILE_TRAVERSE
}

func windowsEnumerationAccess() uint32 {
	return windows.FILE_READ_ATTRIBUTES | windows.FILE_LIST_DIRECTORY
}

// windowsContentAccess is the only right in this package that can pull bytes
// out of a file, and it is requested only for an entry already proved regular
// through its parent-relative metadata handle.
func windowsContentAccess() uint32 {
	return windows.FILE_READ_ATTRIBUTES | windows.FILE_READ_DATA
}

func windowsAnyOptions() uint32 {
	return windows.FILE_OPEN_REPARSE_POINT | windows.FILE_SYNCHRONOUS_IO_NONALERT
}

func windowsDirectoryOptions() uint32 {
	return windowsAnyOptions() | windows.FILE_DIRECTORY_FILE
}

func windowsContentOptions() uint32 {
	return windowsAnyOptions() | windows.FILE_NON_DIRECTORY_FILE
}

func windowsOperationError(err error) error {
	var status windows.NTStatus
	if errors.As(err, &status) {
		return status.Errno()
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno
	}
	return err
}
