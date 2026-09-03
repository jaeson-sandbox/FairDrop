//go:build windows

package source

import (
	"errors"
	"fmt"
	"io/fs"
	"syscall"
)

func platformReparsePoint(info fs.FileInfo) (bool, error) {
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

func platformUnreachableNetworkError(err error) bool {
	// The syscall package does not name these Win32 network errors, so keep
	// their stable system error numbers local to the Windows implementation.
	const (
		errorBadNetPath         syscall.Errno = 53
		errorBadNetName         syscall.Errno = 67
		errorNoNetOrBadPath     syscall.Errno = 1203
		errorNetworkUnreachable syscall.Errno = 1231
		errorHostUnreachable    syscall.Errno = 1232
	)
	return errors.Is(err, errorBadNetPath) ||
		errors.Is(err, errorBadNetName) ||
		errors.Is(err, errorNoNetOrBadPath) ||
		errors.Is(err, errorNetworkUnreachable) ||
		errors.Is(err, errorHostUnreachable)
}
