package transfer

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// SafeArchiveSegment reports whether one path segment is safe to write into an
// archive somebody else will extract.
//
// This is the security half of archive naming and it refuses, because every
// rule in it describes a name that can do damage on the receiving side rather
// than merely fail to be saved:
//
//   - "", "." and ".." are path traversal, as is any segment carrying a
//     separator. An entry named "../../.bashrc" is the zip-slip primitive.
//   - A control character is a truncation primitive: a C-based extractor stops
//     at the NUL and writes a different file from the one the entry names.
//   - A Unicode format character is a spoofing primitive. U+202E
//     RIGHT-TO-LEFT OVERRIDE is the classic one, turning "report<RLO>txt.exe"
//     into something that displays as "report.exe" while executing as one.
//   - Invalid UTF-8 leaves the name to each extractor's guess.
//
// What it deliberately no longer refuses is a name that is merely awkward on
// Windows. Those are PortableArchiveSegment's, and they warn (2026-09-13
// owner decision): "Q1 report:final.txt" is an ordinary filename on macOS and
// Linux and on the phone that is this product's flagship receiver, and
// refusing a whole folder because one entry might inconvenience one possible
// receiver was the most restrictive answer available. Every other archiver
// stores such a name unchanged and lets the extractor decide.
func SafeArchiveSegment(name string) bool {
	if name == "" || name == "." || name == ".." || !utf8.ValidString(name) ||
		strings.ContainsAny(name, `/\`) || volumeQualified(name) {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return false
		}
	}
	return true
}

// PortableArchiveSegment reports whether one segment can also be saved by a
// Windows receiver.
//
// A false answer is never a refusal. It raises the Staged warning that tells a
// sender some names may not extract on Windows, so the choice of whether to
// rename stays theirs and the transfer happens either way.
//
// A colon is here rather than in the security half on purpose, with one
// exception carved back out. The colon that is genuinely dangerous is a drive
// prefix -- "C:evil.txt", which a receiver joining it onto a destination turns
// into a drive-relative path -- and volumeQualified above refuses exactly that
// shape, on every sender platform rather than only where filepath agrees. A
// colon anywhere else in a name reaches an NTFS alternate data stream at
// worst, which every mainstream extractor refuses or substitutes rather than
// writing.
//
// That distinction was nearly lost moving the colon here: Story 3.8 had folded
// the old volumeQualified check into the blanket colon ban, so relaxing the
// ban deleted a traversal guard with it. It is spelled out again below.
func PortableArchiveSegment(name string) bool {
	if strings.ContainsAny(name, `<>:"|?*`) ||
		strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return false
	}
	stem, _, _ := strings.Cut(strings.ToUpper(name), ".")
	stem = strings.TrimRight(stem, " ")
	switch stem {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return false
	}
	for _, prefix := range []string{"COM", "LPT"} {
		if strings.HasPrefix(stem, prefix) {
			suffix := strings.TrimPrefix(stem, prefix)
			if len([]rune(suffix)) == 1 && strings.ContainsAny(suffix, "123456789¹²³") {
				return false
			}
		}
	}
	return true
}

// volumeQualified reports whether a segment begins with a Windows volume
// prefix such as "C:".
//
// filepath.VolumeName cannot answer this and used to be asked: it is a no-op
// on POSIX, so the identical entry name was refused when the sender ran
// Windows and accepted when it ran macOS or Linux. The risk is entirely
// receiver-side -- whoever extracts the archive may well be on Windows -- so
// the sender's platform must not decide it. A directory named "C:" is
// perfectly legal on macOS and Linux, which is what makes this reachable
// rather than theoretical.
func volumeQualified(segment string) bool {
	if len(segment) < 2 || segment[1] != ':' {
		return false
	}
	letter := segment[0]
	return (letter >= 'A' && letter <= 'Z') || (letter >= 'a' && letter <= 'z')
}
