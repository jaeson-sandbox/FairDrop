//go:build !windows

package source

import "io/fs"

func platformReparsePoint(fs.FileInfo) (bool, error) { return false, nil }

func platformUnreachableNetworkError(error) bool { return false }
