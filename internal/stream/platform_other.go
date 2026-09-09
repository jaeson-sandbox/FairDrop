//go:build !windows

package stream

import "io/fs"

// reparsePoint has no meaning outside Windows: a POSIX link is already carried
// by FileMode's ModeSymlink bit, which the caller checks alongside this. The
// test seam stays so fabricated metadata behaves the same on every platform.
func reparsePoint(info fs.FileInfo) (bool, error) {
	if metadata, ok := info.(interface{ FairDropReparse() bool }); ok {
		return metadata.FairDropReparse(), nil
	}
	return false, nil
}
