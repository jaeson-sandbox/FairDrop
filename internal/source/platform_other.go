//go:build !windows

package source

import "io/fs"

func platformReparsePoint(info fs.FileInfo) (bool, error) {
	if metadata, ok := info.(interface{ FairDropReparse() bool }); ok {
		return metadata.FairDropReparse(), nil
	}
	return false, nil
}

func platformUnreachableNetworkError(error) bool { return false }
