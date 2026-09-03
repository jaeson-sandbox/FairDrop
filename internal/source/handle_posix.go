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

func openPosixNode(locator posixLocator, flags int, list bool) (*posixNode, error) {
	var fd int
	var err error
	if locator.parent != nil {
		if locator.parent.file == nil {
			return nil, fs.ErrClosed
		}
		fd, err = unix.Openat(int(locator.parent.file.Fd()), locator.name, flags, 0)
	} else {
		fd, err = unix.Open(locator.absolute, flags, 0)
	}
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
