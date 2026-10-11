//go:build windows

package sink

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// unwritableDir returns an existing plain directory the current user cannot
// create anything in. The read-only attribute does not do that for a directory
// on Windows (it is advisory there), so an ACL is used: a deny entry for
// Everyone (S-1-1-0) on write-data and add-subdirectory, which Windows
// evaluates before any allow entry. The entry is removed again on cleanup so
// the fixture tree can be deleted.
func unwritableDir(t *testing.T, parent string) string {
	t.Helper()
	dir := filepath.Join(parent, "readonly")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("icacls", dir, "/deny", "*S-1-1-0:(WD,AD)").CombinedOutput(); err != nil {
		t.Fatalf("icacls could not deny writes on the fixture folder: %v\n%s", err, output)
	}
	t.Cleanup(func() { _ = exec.Command("icacls", dir, "/remove:d", "*S-1-1-0").Run() })
	// The fixture must prove itself before it is used: an environment whose
	// token can still create here (an elevated runner, say) cannot produce an
	// unwritable folder, and a refusal assertion against a writable folder
	// would test the fixture, not the product. That is a named capability
	// skip with the evidence logged, never a silent pass.
	sentinel := filepath.Join(dir, "precondition")
	if err := os.Mkdir(sentinel, 0o755); err == nil {
		_ = os.Remove(sentinel)
		acl, _ := exec.Command("icacls", dir).CombinedOutput()
		privileges, _ := exec.Command("whoami", "/priv").CombinedOutput()
		t.Logf("deny ACL in place:\n%s\ntoken privileges:\n%s", acl, privileges)
		t.Skip("capability: this Windows token can create inside a write-denied folder, so no unwritable fixture exists here")
	}
	return dir
}
