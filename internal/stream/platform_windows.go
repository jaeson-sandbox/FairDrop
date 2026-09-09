//go:build windows

package stream

import (
	"errors"
	"fmt"
	"io/fs"
	"syscall"
)

// reparsePoint reports whether the claim-time Lstat landed on a junction,
// symlink, or other reparse point. Go's FileMode only marks the subset it
// models as ModeSymlink, so a Windows junction reads as an ordinary directory
// unless the native attribute is consulted directly.
//
// An unrecognized metadata shape is an error rather than a false: a check that
// silently passes when it cannot run is indistinguishable from no check.
func reparsePoint(info fs.FileInfo) (bool, error) {
	if info == nil {
		return false, errors.New("Windows file metadata is nil")
	}
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if ok && data != nil {
		return data.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0, nil
	}
	if metadata, ok := info.(interface{ FairDropReparse() bool }); ok {
		return metadata.FairDropReparse(), nil
	}
	return false, fmt.Errorf("unexpected Windows file metadata type %T", info.Sys())
}
