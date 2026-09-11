// mutationverdict rejects build/setup failures and timeouts as mutation proof.
package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
)

func verdict(mode, name, evidence string, status int, output string) error {
	output = strings.ReplaceAll(output, "\r\n", "\n")
	for _, invalid := range []string{"panic:", "[build failed]", "[setup failed]", "build constraints exclude all Go files", "no Go files", "[no tests to run]", "--- SKIP:", "FAIL\t"} {
		// FAIL plus a package is expected only for a killed mutation.
		if invalid == "FAIL\t" && mode == "mutation" {
			continue
		}
		if strings.Contains(output, invalid) {
			return fmt.Errorf("invalid test execution: %s", invalid)
		}
	}
	switch mode {
	case "baseline":
		passed := regexp.MustCompile("(?m)^--- PASS: " + regexp.QuoteMeta(name) + " \\(")
		if status != 0 || !passed.MatchString(output) || strings.Contains(output, "--- FAIL:") {
			return fmt.Errorf("named baseline did not pass")
		}
	case "mutation":
		failed := regexp.MustCompile("(?m)^--- FAIL: " + regexp.QuoteMeta(name) + " \\(")
		if status != 1 || !failed.MatchString(output) || evidence == "" || !strings.Contains(output, evidence) {
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
