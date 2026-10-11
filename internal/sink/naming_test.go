package sink

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// Every expectation is a literal written out at the assertion site. Deriving one
// from SanitizeName itself would let the rules change with the table.
func TestSanitizeNameTable(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("a", 300)
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"plain name is unchanged", "photo.jpg", "photo.jpg"},
		{"posix traversal keeps only the last component", "../../etc/passwd", "passwd"},
		{"windows drive path keeps only the last component", `C:\Users\x\a.jpg`, "a.jpg"},
		{"mixed separators keep only the last component", `dir/sub\file.txt`, "file.txt"},
		{"a trailing separator leaves nothing", "a/b/", "file"},
		{"an empty name falls back", "", "file"},
		{"only dots falls back", "...", "file"},
		{"only dots and spaces falls back", " . ", "file"},
		{"dot-dot falls back", "..", "file"},
		{"NUL is removed", "name\x00.txt", "name.txt"},
		{"control characters are removed", "ctl\x07\x1b[31m.txt", "ctl[31m.txt"},
		{"DEL is removed", "a\x7fb.txt", "ab.txt"},
		{"a right-to-left override is removed", "report\u202egpj.exe", "reportgpj.exe"},
		{"a zero width joiner is removed", "a\u200db.txt", "ab.txt"},
		{"a byte order mark is removed", "\ufeffnote.txt", "note.txt"},
		{"line and paragraph separators are removed", "a\u2028b\u2029c.txt", "abc.txt"},
		{"invalid utf-8 is removed", "a\xffb.txt", "ab.txt"},
		{"a literal replacement character is removed", "a\ufffdb.txt", "ab.txt"},
		{"characters Windows cannot store are replaced", `a:b*c?.txt`, "a_b_c_.txt"},
		{"quotes pipes and angle brackets are replaced", `q"r|s<t>.txt`, "q_r_s_t_.txt"},
		{"a bare device name is prefixed", "CON", "_CON"},
		{"a device name with an extension is prefixed", "con.txt", "_con.txt"},
		{"a device name with a double extension is prefixed", "NUL.tar.gz", "_NUL.tar.gz"},
		{"a numbered device name is prefixed", "COM1", "_COM1"},
		{"a numbered printer device name is prefixed", "LPT9.txt", "_LPT9.txt"},
		{"a name that merely starts like a device is unchanged", "CONSOLE.txt", "CONSOLE.txt"},
		{"COM0 is not a device name", "COM0", "COM0"},
		{"trailing dots and spaces are trimmed", "name. . ", "name"},
		{"leading spaces are trimmed", "  leading.txt", "leading.txt"},
		{"a leading dot is kept", ".hidden", ".hidden"},
		{"a long ascii name keeps its extension", long + ".jpg", strings.Repeat("a", 251) + ".jpg"},
		{"an implausibly long extension is part of the stem", "a." + strings.Repeat("b", 100), "a." + strings.Repeat("b", 100)},
		{"unicode is kept", "café 写真.jpg", "café 写真.jpg"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if got := SanitizeName(testCase.raw); got != testCase.want {
				t.Fatalf("SanitizeName(%q) = %q, want %q", testCase.raw, got, testCase.want)
			}
		})
	}
}

// A path-bearing name is the case the contract exists for: the host's own
// filepath rules must not decide, so the same name sanitizes identically on a
// Windows and a macOS desktop.
func TestSanitizeNameStripsBothSeparatorsOnEveryHost(t *testing.T) {
	t.Parallel()

	for raw, want := range map[string]string{
		`..\..\Windows\System32\evil.dll`: "evil.dll",
		`/etc/cron.d/job`:                 "job",
		`C:\a.txt`:                        "a.txt",
		`\\server\share\b.txt`:            "b.txt",
		`x/y\z/w\v.txt`:                   "v.txt",
	} {
		if got := SanitizeName(raw); got != want {
			t.Errorf("SanitizeName(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestSanitizeNameTruncatesOnARuneBoundaryWithinTwoHundredFiftyFiveBytes(t *testing.T) {
	t.Parallel()

	got := SanitizeName(strings.Repeat("é", 200) + ".txt")
	if !utf8.ValidString(got) {
		t.Fatalf("the cut name is not valid UTF-8: %q", got)
	}
	if len(got) > 255 {
		t.Fatalf("the cut name is %d bytes, want at most 255", len(got))
	}
	if !strings.HasSuffix(got, ".txt") {
		t.Fatalf("the cut name lost its extension: %q", got)
	}
	if want := strings.Repeat("é", 125) + ".txt"; got != want {
		t.Fatalf("SanitizeName cut to %d bytes, want exactly 125 whole runes plus the extension", len(got))
	}
}

func TestSanitizedNamesAreAlwaysPortableSingleSegments(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"", ".", "..", "con", "a:b", "x/", `\`, "a\x00", strings.Repeat("é", 400), "  ", "trail. ", "NUL:"} {
		got := SanitizeName(raw)
		if got == "" || got == "." || got == ".." {
			t.Errorf("SanitizeName(%q) = %q, want a usable segment", raw, got)
		}
		if strings.ContainsAny(got, `/\:*?"<>|`) || strings.HasSuffix(got, ".") || strings.HasSuffix(got, " ") {
			t.Errorf("SanitizeName(%q) = %q, want a name Windows and macOS can both store", raw, got)
		}
		if len(got) > 255 || !utf8.ValidString(got) {
			t.Errorf("SanitizeName(%q) = %q (%d bytes), want valid UTF-8 within 255 bytes", raw, got, len(got))
		}
	}
}

func TestDuplicateNamesAreNumberedBeforeTheExtensionCaseInsensitively(t *testing.T) {
	t.Parallel()

	set := newNameSet()
	for _, step := range []struct{ in, want string }{
		{"x.jpg", "x.jpg"},
		{"x.jpg", "x (1).jpg"},
		{"X.JPG", "X (2).JPG"},
		{"x (1).jpg", "x (1) (1).jpg"},
		{"y", "y"},
		{"Y", "Y (1)"},
		{".dotfile", ".dotfile"},
		{".dotfile", ".dotfile (1)"},
	} {
		if got := set.claim(step.in); got != step.want {
			t.Fatalf("claim(%q) = %q, want %q", step.in, got, step.want)
		}
	}
}

func TestADuplicateOfALongNameStillFitsTwoHundredFiftyFiveBytes(t *testing.T) {
	t.Parallel()

	name := SanitizeName(strings.Repeat("a", 400) + ".jpeg")
	set := newNameSet()
	first := set.claim(name)
	second := set.claim(name)
	if len(first) != 255 || len(second) > 255 {
		t.Fatalf("lengths %d and %d, want 255 and at most 255", len(first), len(second))
	}
	if first == second || !strings.HasSuffix(second, " (1).jpeg") {
		t.Fatalf("the duplicate was %q, want a distinct name ending \" (1).jpeg\"", second)
	}
}
