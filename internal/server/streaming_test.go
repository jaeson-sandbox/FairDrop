package server

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The contract forbids ParseMultipartForm and ReadForm outright: either holds an
// upload in memory or spools it to OS temp storage. A behavioural test notices
// one that stops the upload working; this notices one that merely adds a call.
// Line endings are normalized first, because a Windows checkout may use CRLF and
// a silent non-match is a false green.
func TestTheServerNeverBuffersAMultipartBody(t *testing.T) {
	forbidden := regexp.MustCompile(`\.(ParseMultipartForm|ReadForm|FormFile|ParseForm|FormValue|PostFormValue)\(`)
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var sawStreaming bool
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for number, line := range strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n") {
			code := strings.TrimSpace(line)
			if strings.HasPrefix(code, "//") {
				continue
			}
			if forbidden.MatchString(code) {
				t.Errorf("%s:%d buffers a multipart body: %s", name, number+1, code)
			}
			if strings.Contains(code, ".MultipartReader()") {
				sawStreaming = true
			}
		}
	}
	if !sawStreaming {
		t.Error("no code in the package streams the upload with MultipartReader()")
	}
}
