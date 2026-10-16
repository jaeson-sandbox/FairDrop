package transfer

import (
	"strings"
	"unicode/utf8"
)

const (
	MaxTextBytes    = 65536
	MaxRawTextBytes = 131072
)

// NormalizeText enforces the input contract before a session retains any text.
// Only line endings change; accepted spaces and all other runes remain exact.
func NormalizeText(raw string) (string, error) {
	if len(raw) > MaxRawTextBytes || !utf8.ValidString(raw) {
		return "", NewError(ErrInvalidText, "text exceeds the input limit or is malformed")
	}
	var b strings.Builder
	b.Grow(len(raw))
	blank := true
	previousCR := false
	for _, r := range raw {
		if r == '\r' {
			b.WriteByte('\n')
			previousCR = true
			if b.Len() > MaxTextBytes {
				return "", NewError(ErrInvalidText, "text exceeds 65,536 bytes")
			}
			continue
		}
		if r == '\n' && previousCR {
			previousCR = false
			continue
		}
		previousCR = false
		if r < 0x20 && r != '\t' && r != '\n' || r == 0x7f {
			return "", NewError(ErrInvalidText, "text contains unsupported characters")
		}
		if !textWhiteSpace(r) {
			blank = false
		}
		b.WriteRune(r)
		if b.Len() > MaxTextBytes {
			return "", NewError(ErrInvalidText, "text exceeds 65,536 bytes")
		}
	}
	if blank {
		return "", NewError(ErrInvalidText, "text is blank")
	}
	return b.String(), nil
}

// textWhiteSpace spells out the shared White_Space set without language-runtime
// trimming rules that vary between Go and JavaScript.
func textWhiteSpace(r rune) bool {
	return r >= 0x09 && r <= 0x0d || r == 0x20 || r == 0x85 || r == 0xa0 || r == 0x1680 || r >= 0x2000 && r <= 0x200a || r == 0x2028 || r == 0x2029 || r == 0x202f || r == 0x205f || r == 0x3000
}
