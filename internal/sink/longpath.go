package sink

import "strings"

// extendedLengthPath spells an absolute, already-cleaned Windows path in the
// extended-length form (`\\?\C:\...`, or `\\?\UNC\server\share\...` for a UNC
// path), which lifts the 260 character MAX_PATH limit for the raw Win32 calls
// this package makes. Go's own os.* functions add the prefix themselves when a
// path is long; a raw call such as MoveFileEx does not, so without this a
// completed file whose final path exceeds MAX_PATH would be discarded at the
// last step.
//
// The extended form bypasses Windows' path normalisation, so the input must
// already be absolute and cleaned (backslash separators, no "." or ".."
// elements). Anything that is not recognisably that -- a relative or
// drive-relative path -- is returned unchanged rather than guessed at, as is
// a path already in an extended or device form.
//
// It is pure string logic with no host dependence, so every platform's test run
// checks it; only the Windows handle calls it.
func extendedLengthPath(path string) string {
	switch {
	case strings.HasPrefix(path, `\\?\`), strings.HasPrefix(path, `\\.\`):
		return path
	case strings.HasPrefix(path, `\\`):
		return `\\?\UNC\` + path[2:]
	case len(path) >= 3 && path[1] == ':' && path[2] == '\\' && isDriveLetter(path[0]):
		return `\\?\` + path
	default:
		return path
	}
}

func isDriveLetter(char byte) bool {
	return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
}
