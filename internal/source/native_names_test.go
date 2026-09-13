package source

import (
	"fairdrop/internal/transfer"
	"testing"
)

// TestNativeChildNameRefusesOnlyUnsafeNames covers both halves of the split
// made on 2026-09-13: what endangers a receiver is refused, and what merely
// inconveniences a Windows one is reported as unportable and sent anyway.
func TestNativeChildNameRefusesOnlyUnsafeNames(t *testing.T) {
	for _, name := range []string{"C:evil.txt", "c:evil.txt", "Z:", "/absolute", `\server\share`, `C:\absolute`, "nested/name", ".."} {
		if got, _, err := childRelativeName("parent", name); got != "" || transfer.ErrorCodeOf(err) != transfer.ErrNameUnsupported {
			t.Errorf("unsafe fixture name accepted: result=%q code=%q", got, transfer.ErrorCodeOf(err))
		}
	}
	for _, name := range []string{"ordinary.txt", "résumé with spaces.txt", "1 ordinary"} {
		got, portable, err := childRelativeName("parent", name)
		if err != nil {
			t.Fatalf("safe fixture refused: %v", err)
		}
		if !portable {
			t.Errorf("%q was reported unportable; it travels everywhere", name)
		}
		if got != "parent/"+name {
			t.Errorf("childRelativeName(%q) = %q", name, got)
		}
	}
	// Accepted, and counted. Each is an ordinary filename on the sending
	// machine; the sender is warned at Staged rather than refused.
	for _, name := range []string{"Q1 report:final.txt", "what?.txt", "con.txt", "tail.", "tail "} {
		got, portable, err := childRelativeName("parent", name)
		if err != nil {
			t.Fatalf("a Windows-awkward name was refused rather than counted: %q %v", name, err)
		}
		if portable {
			t.Errorf("%q was reported portable; a Windows receiver cannot save it", name)
		}
		if got != "parent/"+name {
			t.Errorf("childRelativeName(%q) = %q, want the name unchanged", name, got)
		}
	}
}
