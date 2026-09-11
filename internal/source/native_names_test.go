package source

import (
	"fairdrop/internal/transfer"
	"testing"
)

func TestNativeChildNameRefusesReceiverVolumePrefixes(t *testing.T) {
	for _, name := range []string{"C:evil.txt", "c:evil.txt", "Z:", "/absolute", `\\server\share`, `C:\absolute`, "nested/name", ".."} {
		if got, err := childRelativeName("parent", name); got != "" || transfer.ErrorCodeOf(err) != transfer.ErrPathUnsupported {
			t.Errorf("unsafe fixture name accepted: result=%q code=%q", got, transfer.ErrorCodeOf(err))
		}
	}
	for _, name := range []string{"ordinary.txt", "résumé with spaces.txt", "1:ordinary"} {
		if _, err := childRelativeName("parent", name); err != nil {
			t.Fatalf("safe fixture refused: %v", err)
		}
	}
}
