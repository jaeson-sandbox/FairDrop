package sink

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"fairdrop/internal/transfer"
)

const (
	// maxNameBytes is the longest final name written: 255 UTF-8 bytes is the
	// most any of NTFS, APFS and ext4 stores, and a UTF-8 byte count is never
	// smaller than the UTF-16 unit count Windows measures.
	maxNameBytes = 255

	// maxExtensionBytes bounds what counts as an extension worth keeping when a
	// name is cut to fit. A "." followed by a hundred characters is a name, not
	// an extension.
	maxExtensionBytes = 32

	// fallbackName stands in when nothing usable remains of a phone's name.
	fallbackName = "file"
)

// SanitizeName turns an untrusted phone-supplied file name into one safe final
// path component. It never refuses: a name that is all hazard becomes "file".
//
// The rules are those in receive-contract.md's Naming section, applied in this
// order:
//
//   - only the part after the last "/" or "\" is kept, whichever the sender's
//     platform used, so a path never reaches a join;
//   - control, format and bidirectional characters, line and paragraph
//     separators and invalid UTF-8 are removed -- the characters that truncate a
//     name in a C extractor or reorder how it displays;
//   - characters Windows or macOS cannot store (< > : " | ? *) become "_";
//   - a Windows device name (CON, NUL, COM1 ...) gets a leading "_", with or
//     without an extension;
//   - trailing dots and spaces are trimmed, which also turns "." and ".." into
//     nothing;
//   - the result is cut on a rune boundary to 255 UTF-8 bytes, keeping the
//     extension where it fits.
//
// Every check here is a pure function of the string, so a Windows desktop and a
// macOS desktop agree on every name -- the receiver-side risk must not depend on
// which host runs the check.
func SanitizeName(raw string) string {
	if cut := strings.LastIndexAny(raw, `/\`); cut >= 0 {
		raw = raw[cut+1:]
	}

	var builder strings.Builder
	for _, char := range raw {
		switch {
		case char == utf8.RuneError:
			// A decoding error, or a literal U+FFFD: either way nothing a
			// receiver can use, and the former is a name that means different
			// things to different extractors.
			continue
		case unicode.IsControl(char), unicode.Is(unicode.Cf, char),
			char == ' ', char == ' ':
			continue
		case strings.ContainsRune(`<>:"|?*`, char):
			builder.WriteByte('_')
		default:
			builder.WriteRune(char)
		}
	}

	name := strings.TrimRight(strings.TrimLeft(builder.String(), " "), ". ")
	if name == "" {
		return fallbackName
	}
	if !transfer.PortableArchiveSegment(name) {
		// What remains after the replacements above is a Windows device name.
		name = "_" + name
	}
	return fitName(name, "")
}

// fitName cuts name so that name with suffix inserted before its extension is at
// most maxNameBytes, cutting the stem rather than the extension, on a rune
// boundary, and re-trimming what the cut exposes. suffix is "" for a first
// occurrence and " (n)" for a duplicate.
func fitName(name, suffix string) string {
	stem, extension := splitExtension(name)
	room := maxNameBytes - len(suffix) - len(extension)
	if room < 1 {
		// The extension plus suffix alone leave no room for a stem. Drop the
		// extension rather than produce an over-long name.
		extension = ""
		room = maxNameBytes - len(suffix)
	}
	if len(stem) > room {
		cut := room
		for cut > 0 && !utf8.RuneStart(stem[cut]) {
			cut--
		}
		stem = strings.TrimRight(stem[:cut], ". ")
	}
	if stem == "" {
		stem = fallbackName
	}
	return stem + suffix + extension
}

// splitExtension separates the last ".ext" from a name. A leading dot alone
// (".profile") is a name, not an extension, and an implausibly long suffix is
// treated as part of the stem.
func splitExtension(name string) (stem, extension string) {
	dot := strings.LastIndexByte(name, '.')
	if dot <= 0 || len(name)-dot > maxExtensionBytes {
		return name, ""
	}
	return name[:dot], name[dot:]
}

// nameSet de-duplicates final names within one upload, case-insensitively,
// because the desktop may be on a case-insensitive volume and the phone on a
// case-sensitive one. It is not safe for concurrent use; the destination
// guards it.
type nameSet struct {
	taken map[string]struct{}
}

func newNameSet() *nameSet {
	return &nameSet{taken: make(map[string]struct{})}
}

// claim returns the first of name, "stem (1).ext", "stem (2).ext" ... that is
// not yet taken, and marks it taken.
func (s *nameSet) claim(name string) string {
	candidate := name
	for attempt := 1; ; attempt++ {
		key := strings.ToLower(candidate)
		if _, taken := s.taken[key]; !taken {
			s.taken[key] = struct{}{}
			return candidate
		}
		candidate = fitName(name, " ("+strconv.Itoa(attempt)+")")
	}
}
