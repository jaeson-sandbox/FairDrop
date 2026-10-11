package sink

import "testing"

// The raw MoveFileEx call is only as long-path-safe as this spelling, and the
// spelling is pure string logic, so it is pinned on every host with literals.
func TestExtendedLengthPathSpellsDrivesAndUNCSharesAndLeavesTheRestAlone(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"a drive path", `C:\Users\me\Downloads`, `\\?\C:\Users\me\Downloads`},
		{"a lowercase drive path", `d:\x`, `\\?\d:\x`},
		{"a volume root", `C:\`, `\\?\C:\`},
		{"a UNC share", `\\server\share\dir`, `\\?\UNC\server\share\dir`},
		{"an already extended path", `\\?\C:\x`, `\\?\C:\x`},
		{"an already extended UNC path", `\\?\UNC\server\share`, `\\?\UNC\server\share`},
		{"a device path", `\\.\pipe\x`, `\\.\pipe\x`},
		{"a relative path", `dir\file`, `dir\file`},
		{"a drive-relative path", `C:file`, `C:file`},
		{"a POSIX path", `/home/me`, `/home/me`},
		{"an empty path", ``, ``},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := extendedLengthPath(testCase.in); got != testCase.want {
				t.Fatalf("extendedLengthPath(%q) = %q, want %q", testCase.in, got, testCase.want)
			}
		})
	}
}
