package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

/*
TestAcquireInstanceLockRefusesASecondHolderAndReleasesOnClose drives the
function main calls, not the primitive underneath it.

main_test.go already proves lockFileExclusive is exclusive, and
TestAnUnheldInstanceLockLeavesTheAppUncomposed already proves what an unheld
lock does to composition -- but it is handed `held` as a boolean, so nothing
executed the code that decides it. The Epic 3 retrospective replaced this
entire function body with `return func() {}, true` and the whole repository
stayed green (B1): D-088's backstop could have stopped working with no test
noticing.

Two acquisitions against one config directory is the whole guarantee: the
second process must be told it lost, and the lock must come back when the
first lets go.
*/
func TestAcquireInstanceLockRefusesASecondHolderAndReleasesOnClose(t *testing.T) {
	configDir := fixedConfigDir(t.TempDir())

	release, held := acquireInstanceLockIn(configDir, discardLog)
	if !held {
		t.Fatal("the first acquisition did not take an uncontended lock")
	}

	if _, second := acquireInstanceLockIn(configDir, discardLog); second {
		t.Error("a second acquisition took the lock while the first still held it: " +
			"a competing coordinator, listener and beacon would start")
	}

	release()

	third, held := acquireInstanceLockIn(configDir, discardLog)
	if !held {
		t.Error("the lock did not come back after its holder released it, so every later launch is refused")
	}
	third()
}

/*
TestAcquireInstanceLockCreatesItsOwnDirectory pins the path the lock lives at,
which is the reason it survives a temp cleaner (see instanceLockName's comment).
Asserting the file exists also distinguishes a real acquisition from a
short-circuit that reports held without taking anything.
*/
func TestAcquireInstanceLockCreatesItsOwnDirectory(t *testing.T) {
	root := t.TempDir()

	release, held := acquireInstanceLockIn(fixedConfigDir(root), discardLog)
	if !held {
		t.Fatal("the lock was not taken")
	}
	defer release()

	if _, err := os.Stat(filepath.Join(root, "FairDrop", instanceLockName)); err != nil {
		t.Errorf("stat the lock file: %v -- a reported hold that created no file holds nothing", err)
	}
}

/*
TestAcquireInstanceLockLaunchesWithoutABackstopAndSaysSo covers both halves of
the setup-failure contract, on each of the two reachable failures.

Launching is the decision D-088 made and this keeps it: a missing configuration
directory is the behaviour that shipped before the backstop existed, and
refusing to launch over it would trade a rare double instance for a certain
dead application.

Saying so is the half that was missing. Every other protection in this build
that disables itself leaves a line; this one left nothing, so the exact failure
mode the file exists to catch could recur with no trail (Epic 3 retrospective,
B13).
*/
func TestAcquireInstanceLockLaunchesWithoutABackstopAndSaysSo(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("a file where a directory must go"), 0o600); err != nil {
		t.Fatalf("write the blocking file: %v", err)
	}

	cases := map[string]func() (string, error){
		"no configuration directory": func() (string, error) { return "", errors.New("no config dir") },
		"a file where the directory must be created": func() (string, error) {
			return filepath.Join(blocker, "under-a-file"), nil
		},
	}

	for name, configDir := range cases {
		t.Run(name, func(t *testing.T) {
			var logged []string
			release, held := acquireInstanceLockIn(configDir, func(format string, args ...any) {
				// Formatted, never the raw format string: what AD-9 cares
				// about is the line that reaches the log, and a path reaches
				// it through the arguments.
				logged = append(logged, fmt.Sprintf(format, args...))
			})
			defer release()

			if !held {
				t.Error("a failed lock setup reported the lock as taken by someone else, which would " +
					"leave this launch uncomposed for no reason")
			}
			if len(logged) != 1 {
				t.Fatalf("logged %d lines, want exactly one: a protection that disables itself "+
					"silently is the D-088 failure mode with no trail", len(logged))
			}
			if logged[0] != lockUnavailable {
				t.Errorf("logged %q, want %q", logged[0], lockUnavailable)
			}
		})
	}
}

/*
TestTheInstanceLockDiagnosticNamesNoPath is AD-9 at this boundary.

All three setup failures are filesystem errors, and a filesystem error's text
is mostly the path it was about. The temporary directory below is the one this
test knows the lock would have used, so finding any part of it in the line is
the disclosure this assertion exists to refuse.
*/
func TestTheInstanceLockDiagnosticNamesNoPath(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(blocker, []byte("blocked"), 0o600); err != nil {
		t.Fatalf("write the blocking file: %v", err)
	}

	var line string
	release, _ := acquireInstanceLockIn(
		func() (string, error) { return filepath.Join(blocker, "under-a-file"), nil },
		func(format string, args ...any) { line = fmt.Sprintf(format, args...) },
	)
	defer release()

	for _, secret := range []string{root, blocker, "not-a-directory", "under-a-file"} {
		if strings.Contains(line, secret) {
			t.Errorf("the diagnostic carries %q: a lock path is a filesystem path (AD-9)", secret)
		}
	}
	if !strings.Contains(line, "instance lock") {
		t.Errorf("the diagnostic %q does not say what became unavailable", line)
	}
}

func fixedConfigDir(path string) func() (string, error) {
	return func() (string, error) { return path, nil }
}

func discardLog(string, ...any) {}
