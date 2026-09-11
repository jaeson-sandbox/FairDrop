package main

import "testing"

func TestVerdictRequiresNamedBaselineAndSpecificFailure(t *testing.T) {
	pass := "--- PASS: TestGuard (0.00s)\nPASS\nok example\n"
	fail := "--- FAIL: TestGuard (0.00s)\n    guard_test.go:3: expected guard missing\nFAIL\nFAIL\texample\t0.1s\n"
	for _, tc := range []struct {
		name, mode, log string
		status          int
		want            bool
	}{
		{"baseline", "baseline", pass, 0, true},
		{"killed", "mutation", fail, 1, true},
		{"wrong-test", "mutation", "--- FAIL: TestOther (0.00s)\nexpected guard missing", 1, false},
		{"wrong-assertion", "mutation", "--- FAIL: TestGuard (0.00s)\nfixture failed", 1, false},
		{"timed-out-after-failure", "mutation", fail + "panic: test timed out", 1, false},
		{"build-after-failure", "mutation", fail + "FAIL another [build failed]", 1, false},
		{"setup-after-failure", "mutation", fail + "FAIL another [setup failed]", 1, false},
		{"panic-after-failure", "mutation", fail + "panic: runtime error", 1, false},
		{"killed-process", "mutation", fail, 137, false},
		{"false-status", "mutation", fail, 0, false},
		{"no-baseline", "baseline", "ok example [no tests to run]", 0, false},
		{"skipped-baseline", "baseline", pass + "--- SKIP: TestGuard/permission (0.00s)", 0, false},
		{"failed-baseline", "baseline", fail, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := verdict(tc.mode, "TestGuard", "expected guard missing", tc.status, tc.log)
			if (err == nil) != tc.want {
				t.Fatalf("verdict accepted=%t, want %t", err == nil, tc.want)
			}
		})
	}
}
