package main

import (
	"encoding/json"
	"testing"
)

func testEvent(action, test, output string) string {
	encoded, _ := json.Marshal(map[string]string{"Action": action, "Package": "example", "Test": test, "Output": output})
	return string(encoded) + "\n"
}

func TestVerdictRequiresNamedBaselineAndSpecificFailure(t *testing.T) {
	pass := testEvent("pass", "TestGuard", "") + testEvent("pass", "", "")
	fail := testEvent("output", "TestGuard", "guard_test.go:3: expected guard missing\n") + testEvent("fail", "TestGuard", "") + testEvent("fail", "", "")
	sibling := testEvent("output", "TestGuard/passing", "expected guard missing\n") + testEvent("pass", "TestGuard/passing", "") + testEvent("output", "TestGuard/failing", "fixture failed\n") + testEvent("fail", "TestGuard/failing", "") + testEvent("fail", "TestGuard", "") + testEvent("fail", "", "")
	subtest := testEvent("output", "TestGuard/guard", "expected guard missing\n") + testEvent("fail", "TestGuard/guard", "") + testEvent("fail", "TestGuard", "") + testEvent("fail", "", "")
	for _, tc := range []struct {
		name, mode, log string
		status          int
		want            bool
	}{
		{"baseline", "baseline", pass, 0, true},
		{"killed", "mutation", fail, 1, true},
		{"killed-subtest", "mutation", subtest, 1, true},
		{"passing-sibling-is-not-evidence", "mutation", sibling, 1, false},
		{"package-output-is-not-evidence", "mutation", testEvent("output", "", "expected guard missing") + testEvent("fail", "TestGuard", "") + testEvent("fail", "", ""), 1, false},
		{"wrong-test", "mutation", testEvent("output", "TestOther", "expected guard missing") + testEvent("fail", "TestOther", "") + testEvent("fail", "", ""), 1, false},
		{"wrong-assertion", "mutation", testEvent("output", "TestGuard", "fixture failed") + testEvent("fail", "TestGuard", "") + testEvent("fail", "", ""), 1, false},
		{"timed-out-after-failure", "mutation", fail + testEvent("output", "TestGuard", "panic: test timed out"), 1, false},
		{"build-after-failure", "mutation", fail + testEvent("output", "", "FAIL another [build failed]"), 1, false},
		{"setup-after-failure", "mutation", fail + testEvent("output", "", "FAIL another [setup failed]"), 1, false},
		{"panic-after-failure", "mutation", fail + testEvent("output", "TestGuard", "panic: runtime error"), 1, false},
		{"killed-process", "mutation", fail, 137, false},
		{"false-status", "mutation", fail, 0, false},
		{"no-baseline", "baseline", testEvent("output", "", "ok example [no tests to run]"), 0, false},
		{"skipped-baseline", "baseline", pass + testEvent("skip", "TestGuard/permission", ""), 0, false},
		{"failed-baseline", "baseline", fail, 1, false},
		{"plain-text-is-not-structured-proof", "mutation", "--- FAIL: TestGuard (0.00s)\nexpected guard missing\n", 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := verdict(tc.mode, "TestGuard", "expected guard missing", tc.status, tc.log)
			if (err == nil) != tc.want {
				t.Fatalf("verdict accepted=%t, want %t", err == nil, tc.want)
			}
		})
	}
}
