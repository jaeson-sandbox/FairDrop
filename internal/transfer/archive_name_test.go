package transfer

import "testing"

func TestPortableArchiveSegmentsRejectAmbiguousReceiverNames(t *testing.T) {
	unsafe := []string{"", ".", "..", "a/b", `a\b`, "a\x00b", "a\nb", "a\u202eb", "a\u200db", "a<b", "a>b", "a:b", `a"b`, "a|b", "a?b", "a*b", "tail.", "tail ", "CON", "con.txt", "NUL .txt", "prn", "AUX.log", "COM1", "com9.txt", "LPT1", "lpt9.data", "COM¹.txt", "LPT²", "com³", "CONIN$", "CONOUT$.txt"}
	for _, name := range unsafe {
		if SafeArchiveSegment(name) {
			t.Errorf("unsafe archive segment accepted: %q", name)
		}
	}
	for _, name := range []string{"ordinary.txt", "résumé with spaces.txt", "照片", "a'b;c", ".hidden", "a..b", "COM10", "com0", "lpt10", " leading", "report.zip"} {
		if !SafeArchiveSegment(name) {
			t.Errorf("ordinary archive segment refused: %q", name)
		}
	}
}
