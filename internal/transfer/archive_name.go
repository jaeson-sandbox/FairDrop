package transfer

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// SafeArchiveSegment applies receiver-side naming rules on every sender OS.
// It rejects ambiguous or unsafe names; it never renames source entries.
func SafeArchiveSegment(name string) bool {
	if name == "" || name == "." || name == ".." || !utf8.ValidString(name) ||
		strings.ContainsAny(name, `<>:"/\|?*`) || strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return false
		}
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
