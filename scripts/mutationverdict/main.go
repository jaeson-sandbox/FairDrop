// mutationverdict rejects build/setup failures and timeouts as mutation proof.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func verdict(mode, name, evidence string, status int, output string) error {
	type key struct{ pkg, test string }
	states, transcripts := map[key]string{}, map[key]string{}
	decoder := json.NewDecoder(strings.NewReader(output))
	for {
		var event struct{ Action, Package, Test, Output string }
		err := decoder.Decode(&event)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("invalid go test JSON transcript: %w", err)
		}
		for _, invalid := range []string{"panic:", "[build failed]", "[setup failed]", "build constraints exclude all Go files", "no Go files", "[no tests to run]"} {
			if strings.Contains(event.Output, invalid) {
				return fmt.Errorf("invalid test execution: %s", invalid)
			}
		}
		if event.Action == "skip" || event.Action == "build-fail" {
			return fmt.Errorf("invalid test execution: %s", event.Action)
		}
		k := key{event.Package, event.Test}
		if event.Action == "output" {
			transcripts[k] += event.Output
		}
		if event.Action == "pass" || event.Action == "fail" {
			states[k] = event.Action
		}
	}
	rootPassed, rootFailed, assertionFailed := false, false, false
	for k, state := range states {
		if mode == "baseline" && state == "fail" {
			return fmt.Errorf("baseline contains a failing test or package")
		}
		if k.test == name {
			rootPassed = rootPassed || state == "pass" && states[key{k.pkg, ""}] == "pass"
			rootFailed = rootFailed || state == "fail" && states[key{k.pkg, ""}] == "fail"
		}
		// Evidence is local to the failing test/subtest. Output from a passing
		// sibling, another package, or a package-level log is never proof.
		if (k.test == name || strings.HasPrefix(k.test, name+"/")) && state == "fail" && states[key{k.pkg, name}] == "fail" && states[key{k.pkg, ""}] == "fail" && evidence != "" && strings.Contains(transcripts[k], evidence) {
			assertionFailed = true
		}
	}
	switch mode {
	case "baseline":
		if status != 0 || !rootPassed {
			return fmt.Errorf("named baseline did not pass")
		}
	case "mutation":
		if status != 1 || !rootFailed || !assertionFailed {
			return fmt.Errorf("mutation lacks its named assertion failure")
		}
	default:
		return fmt.Errorf("unknown verdict mode")
	}
	return nil
}

func main() {
	mode := flag.String("mode", "", "baseline or mutation")
	name := flag.String("test", "", "exact top-level test name")
	evidence := flag.String("assert", "", "mutation-specific assertion text")
	status := flag.Int("status", -1, "go test exit status")
	path := flag.String("log", "", "complete test transcript")
	flag.Parse()
	output, err := os.ReadFile(*path)
	if err == nil {
		err = verdict(*mode, *name, *evidence, *status, string(output))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
