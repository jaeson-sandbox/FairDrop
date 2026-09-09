//go:build linux || darwin

package source

import (
	"io/fs"
	"os"
	"strings"

	"fairdrop/internal/transfer"
	"golang.org/x/sys/unix"
)

type nativeHandleFactory struct{}

type posixLocator struct {
	parent   *posixNode
	absolute string
	name     string
}

type posixNode struct {
	file    *os.File
	locator posixLocator
	list    bool
}

func (nativeHandleFactory) Parse(path string) (pathPlan, error) {
	if path == "" || path[0] != '/' || strings.IndexByte(path, 0) >= 0 {
		return pathPlan{}, transfer.NewError(transfer.ErrInvalidSelection, "selection must be one absolute path")
	}
	trailing := len(path) > 1 && path[len(path)-1] == '/'
	parts := strings.Split(path[1:], "/")
	components := make([]string, 0, len(parts))
	for _, component := range parts {
		if component != "" {
			components = append(components, component)
		}
	}
	return pathPlan{anchor: "/", components: components, rootLabel: "root", hadTrailingSep: trailing}, nil
}

func (nativeHandleFactory) OpenAnchor(plan pathPlan) (metadataHandle, error) {
	return openPosixNode(posixLocator{absolute: plan.anchor}, nativeSearchFlags(), false)
}

func (n *posixNode) Stat() (fs.FileInfo, error) {
	if n == nil || n.file == nil {
		return nil, fs.ErrClosed
	}
	return n.file.Stat()
}

func (n *posixNode) OpenChildMetadata(name string) (metadataHandle, error) {
	return openPosixNode(posixLocator{parent: n, name: name}, nativeMetadataFlags(), false)
}

func (n *posixNode) OpenSearch() (metadataHandle, error) {
	return openPosixNode(n.locator, nativeSearchFlags(), false)
}

func (n *posixNode) OpenEnumeration() (directoryHandle, error) {
	return openPosixNode(n.locator, nativeEnumerationFlags(), true)
}

// OpenChildContent is the one read-capable open in the package, and the only
// place a POSIX descriptor can block. O_NOFOLLOW refuses a symlink, but the
// kind of whatever else sits behind the name is knowable only after the open,
// and opening a FIFO O_RDONLY parks the caller until a writer appears. So the
// open adds O_NONBLOCK, the raw descriptor is fstat'ed before it is wrapped in
// anything that could read from it, a non-regular object is closed and
// refused, and O_NONBLOCK is cleared only once the object is known to be a
// regular file -- where the flag has no effect on reads anyway.
func (n *posixNode) OpenChildContent(name string) (contentHandle, error) {
	descriptor, err := openPosixDescriptor(posixLocator{parent: n, name: name}, nativeContentFlags())
	if err != nil {
		return nil, err
	}
	var status unix.Stat_t
	if err := unix.Fstat(descriptor, &status); err != nil {
		_ = unix.Close(descriptor)
		return nil, err
	}
	if status.Mode&unix.S_IFMT != unix.S_IFREG {
		_ = unix.Close(descriptor)
		return nil, transfer.NewError(transfer.ErrPathUnsupported, "selection entry is not a regular file")
	}
	if err := clearPosixNonBlocking(descriptor); err != nil {
		_ = unix.Close(descriptor)
		return nil, err
	}
	file := os.NewFile(uintptr(descriptor), name)
	if file == nil {
		_ = unix.Close(descriptor)
		return nil, fs.ErrInvalid
	}
	return &posixContent{file: file}, nil
}

// posixContent is a read-only view of one opened regular file. It is a
// separate type from posixNode precisely so that no metadata, search, or
// enumeration handle carries a Read at all.
type posixContent struct{ file *os.File }

func (c *posixContent) Read(p []byte) (int, error) {
	if c == nil || c.file == nil {
		return 0, fs.ErrClosed
	}
	return c.file.Read(p)
}

func (c *posixContent) Stat() (fs.FileInfo, error) {
	if c == nil || c.file == nil {
		return nil, fs.ErrClosed
	}
	return c.file.Stat()
}

func (c *posixContent) Close() error {
	if c == nil || c.file == nil {
		return nil
	}
	err := c.file.Close()
	c.file = nil
	return err
}

func clearPosixNonBlocking(descriptor int) error {
	flags, err := unix.FcntlInt(descriptor, unix.F_GETFL, 0)
	if err != nil {
		return err
	}
	if flags&unix.O_NONBLOCK == 0 {
		return nil
	}
	_, err = unix.FcntlInt(descriptor, unix.F_SETFL, flags&^unix.O_NONBLOCK)
	return err
}

func (n *posixNode) ReadDir(count int) ([]fs.DirEntry, error) {
	if n == nil || n.file == nil || !n.list {
		return nil, fs.ErrPermission
	}
	return n.file.ReadDir(count)
}

func (n *posixNode) Close() error {
	if n == nil || n.file == nil {
		return nil
	}
	err := n.file.Close()
	n.file = nil
	return err
}

func openPosixDescriptor(locator posixLocator, flags int) (int, error) {
	if locator.parent != nil {
		if locator.parent.file == nil {
			return -1, fs.ErrClosed
		}
		return unix.Openat(int(locator.parent.file.Fd()), locator.name, flags, 0)
	}
	return unix.Open(locator.absolute, flags, 0)
}

func openPosixNode(locator posixLocator, flags int, list bool) (*posixNode, error) {
	fd, err := openPosixDescriptor(locator, flags)
	if err != nil {
		return nil, err
	}
	displayName := locator.name
	if displayName == "" {
		displayName = "root"
	}
	file := os.NewFile(uintptr(fd), displayName)
	if file == nil {
		_ = unix.Close(fd)
		return nil, fs.ErrInvalid
	}
	return &posixNode{file: file, locator: locator, list: list}, nil
}
