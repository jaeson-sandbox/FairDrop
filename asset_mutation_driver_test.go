package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"testing"
)

func TestAssetMutationDriverPinsCanonicalInventory(t *testing.T) {
	cmd := exec.Command(pythonForMutationDriver(t), "scripts/verify-asset-mutations.py", "--inventory")
	raw, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("read executable inventory: %v\n%s", err, raw)
	}
	var got struct {
		Sizes      []int    `json:"sizes"`
		AssetCases []string `json:"asset_cases"`
		ExeCases   []string `json:"exe_cases"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode inventory: %v\n%s", err, raw)
	}
	if fmt.Sprint(got.Sizes) != fmt.Sprint(wantIcoSizes) {
		t.Fatalf("inventory sizes %v, want canonical declaration %v", got.Sizes, wantIcoSizes)
	}
	asset := map[string]bool{}
	for _, label := range got.AssetCases {
		asset[label] = true
	}
	for _, want := range []string{
		"master shifted down 5px while both bands remain in range",
		"64x64 ICO directory entry declares a 64x63 rectangle",
		"16x16 measured distance inflated above twice the real distance",
		"derivation script pins a digest different from the Go test",
	} {
		if !asset[want] {
			t.Errorf("canonical asset inventory is missing %q", want)
		}
	}
	for _, size := range wantIcoSizes {
		label := fmt.Sprintf("%dx%d entry a flat coloured square", size, size)
		if !asset[label] {
			t.Errorf("canonical asset inventory is missing derived size case %q", label)
		}
	}
	exe := map[string]bool{}
	for _, label := range got.ExeCases {
		exe[label] = true
	}
	for _, want := range []string{
		"one RT_ICON directory entry stripped from the built exe",
		"rebuilt with a language-neutral version string table",
		"exe absent",
	} {
		if !exe[want] {
			t.Errorf("canonical executable inventory is missing %q", want)
		}
	}
}

func TestAssetMutationDriverRejectsParserFalseGreens(t *testing.T) {
	if out, err := exec.Command(pythonForMutationDriver(t), "scripts/verify-asset-mutations.py", "--self-test").CombinedOutput(); err != nil {
		t.Fatalf("parser self-test: %v\n%s", err, out)
	}
}

func pythonForMutationDriver(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"python3", "python"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	t.Fatal("python3/python is required to inspect the canonical mutation inventory")
	return ""
}
