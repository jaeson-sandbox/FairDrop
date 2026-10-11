package server

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

// The hand-built bodies elsewhere control every byte; this one proves the same
// route accepts what Go's standard multipart encoder emits, which is what most
// non-browser senders use. The encoder escapes a quote and a backslash in a
// filename, so the names exercise that escaping end to end.
func TestTheStandardMultipartEncoderIsAccepted(t *testing.T) {
	rig := startReceiveRig(t)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("caption", "holiday"); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		`say "hi" é.txt`:        "quoted",
		`back\slash.txt`:        "backslash",
		"日本語のファイル.txt":          "unicode",
		strings.Repeat("a", 10): "plain",
	} {
		part, err := writer.CreateFormFile("files", name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	response, page := rig.postAs(body.Bytes(), writer.FormDataContentType())
	if response.StatusCode != http.StatusOK || !strings.Contains(string(page), "4 files saved to this computer.") {
		t.Fatalf("status %d, page %q, want 4 files saved", response.StatusCode, page)
	}
	sub := rig.subfolder()
	for name, content := range map[string]string{
		`say _hi_ é.txt`:        "quoted",
		`slash.txt`:             "backslash", // the part after the last backslash
		"日本語のファイル.txt":          "unicode",
		strings.Repeat("a", 10): "plain",
	} {
		if got := readText(t, filepath.Join(sub, name)); got != content {
			t.Errorf("%q = %q, want %q", name, got, content)
		}
	}
}
