package transfer

import "testing"

// TestSafeArchiveSegmentRefusesOnlyDangerousNames covers the security half.
//
// Until 2026-09-13 this function also refused every name a Windows receiver
// dislikes, and one such entry refused an entire folder. That is now
// PortableArchiveSegment's job and it warns instead, so the list below is
// deliberately short: each entry is a primitive rather than an inconvenience.
func TestSafeArchiveSegmentRefusesOnlyDangerousNames(t *testing.T) {
	dangerous := map[string]string{
		"":           "empty",
		".":          "dot element",
		"..":         "parent traversal",
		"a/b":        "separator",
		`a\b`:        "Windows separator",
		"a\x00b":     "NUL truncates the name an extractor writes",
		"a\nb":       "control character",
		"a\u202eb":   "right-to-left override spoofs an extension",
		"a\u200db":   "zero-width joiner, a format character",
		"C:evil.txt": "drive prefix a receiver would join onto its destination",
		"c:evil.txt": "the same, lowercase",
		"Z:":         "bare drive prefix",
	}
	for name, why := range dangerous {
		if SafeArchiveSegment(name) {
			t.Errorf("accepted %q, which is %s", name, why)
		}
	}

	// Everything a Windows receiver merely cannot save now travels. Each of
	// these is an ordinary filename on the sending machine and on a phone.
	ordinary := []string{
		"ordinary.txt", "résumé with spaces.txt", "照片", "a'b;c", ".hidden", "a..b",
		"a<b", "a>b", "ab:cd", `a"b`, "a|b", "a?b", "a*b", "tail.", "tail ",
		"CON", "con.txt", "NUL .txt", "prn", "AUX.log", "COM1", "com9.txt", "LPT1",
		"lpt9.data", "COM¹.txt", "LPT²", "com³", "CONIN$", "CONOUT$.txt",
		"COM10", "com0", "lpt10", " leading", "report.zip",
	}
	for _, name := range ordinary {
		if !SafeArchiveSegment(name) {
			t.Errorf("refused %q, which endangers nobody", name)
		}
	}
}

// TestPortableArchiveSegmentNamesWhatWindowsCannotSave covers the half that
// warns. A false answer costs the sender a banner beside the QR code, never
// the transfer.
func TestPortableArchiveSegmentNamesWhatWindowsCannotSave(t *testing.T) {
	unportable := []string{
		"a<b", "a>b", "ab:cd", `a"b`, "a|b", "a?b", "a*b", "tail.", "tail ",
		"CON", "con.txt", "NUL .txt", "prn", "AUX.log", "COM1", "com9.txt", "LPT1",
		"lpt9.data", "COM¹.txt", "LPT²", "com³", "CONIN$", "CONOUT$.txt",
	}
	for _, name := range unportable {
		if PortableArchiveSegment(name) {
			t.Errorf("called %q portable; a Windows receiver cannot save it", name)
		}
	}

	// COM10 and com0 are not reserved -- only COM1 through COM9 are -- and a
	// leading space is legal where a trailing one is not. Over-reporting these
	// would put a warning on a folder that travels perfectly well.
	portable := []string{
		"ordinary.txt", "résumé with spaces.txt", "照片", "a'b;c", ".hidden", "a..b",
		"COM10", "com0", "lpt10", " leading", "report.zip", "COMET", "lptop.txt",
	}
	for _, name := range portable {
		if !PortableArchiveSegment(name) {
			t.Errorf("called %q unportable; it saves fine on Windows", name)
		}
	}
}

// The two halves answer different questions, and a name can fail one without
// failing the other. This is the property the split exists to create: before
// it, every unportable name was also refused.
func TestTheTwoNameRulesAreIndependent(t *testing.T) {
	// A colon anywhere but position one. "a:b" would be refused and should be:
	// one letter then a colon is exactly the drive-prefix shape.
	const unportableButSafe = "Q1 report:final.txt"
	if !SafeArchiveSegment(unportableButSafe) || PortableArchiveSegment(unportableButSafe) {
		t.Errorf("%q should be safe to archive and not portable: that is the whole point of the split", unportableButSafe)
	}
	if SafeArchiveSegment("C:evil") {
		t.Error(`"C:evil" is a drive prefix and must stay refused however the portability rule moves`)
	}
}
