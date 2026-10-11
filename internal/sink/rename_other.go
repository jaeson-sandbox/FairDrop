//go:build !windows && !darwin && !linux

package sink

// renameNoReplaceAt is the hard-link form on a POSIX system with no exclusive
// rename of its own. FairDrop ships for Windows, macOS and Linux; this exists so
// the package still builds, and still never replaces, anywhere else.
func renameNoReplaceAt(dirfd int, from, to string) error {
	return linkThenUnlink(dirfd, from, to)
}
