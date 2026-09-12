//go:build !darwin

package source

import (
	"io/fs"
	"os"
)

func nativeSameFile(first, second fs.FileInfo) bool { return os.SameFile(first, second) }
