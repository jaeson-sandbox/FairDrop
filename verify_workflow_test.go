package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Story 3.2 makes `.github/workflows/verify.yml` the one gate every change
// runs through -- native Windows and macOS jobs, pinned toolchains, every
// check in a fixed order. A workflow file is not itself executable Go, so
// nothing else in this repo notices when a step is reordered, a version pin
// is loosened, or a forbidden flag creeps back in. These tests read the
// workflow (and wails.json, which it depends on) as plain text, the same way
// TestEveryDeferredEntryHasALiveOwner reads deferred-work.md: no YAML
// library, because gopkg.in/yaml.v3 is only an indirect dependency here and
// adding it as a direct one is out of scope for a verification test. Each
// test below pins one clause of the spec's "Always" list and names it in the
// failure message, so breaking a pin fails here rather than in CI three
// minutes later with no explanation.

const verifyWorkflowPath = "verify.yml"

func readVerifyWorkflow(t *testing.T) string {
	t.Helper()
	path := filepath.Join(".github", "workflows", verifyWorkflowPath)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

func readWailsJSONForVerify(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("wails.json")
	if err != nil {
		t.Fatalf("read wails.json: %v", err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

// The repo is public and `main` is unprotected, so the workflow itself is
// the only thing standing between an unwanted branch and a burned Actions
// minute -- the trigger must name branches explicitly.
func TestVerifyWorkflowTriggersOnPullRequestAndTheRightPushBranches(t *testing.T) {
	wf := readVerifyWorkflow(t)

	onIdx := strings.Index(wf, "\non:")
	jobsIdx := strings.Index(wf, "\njobs:")
	if onIdx < 0 || jobsIdx < 0 || jobsIdx < onIdx {
		t.Fatal("could not locate the `on:` trigger block ahead of `jobs:`")
	}
	triggerBlock := wf[onIdx:jobsIdx]

	if !strings.Contains(triggerBlock, "pull_request:") {
		t.Error("the `on:` block does not trigger on pull_request")
	}
	if !strings.Contains(triggerBlock, "push:") {
		t.Error("the `on:` block does not trigger on push")
	}
	if !strings.Contains(triggerBlock, "- main") {
		t.Error("the push trigger does not name the `main` branch explicitly")
	}
	if !strings.Contains(triggerBlock, "epic-*") {
		t.Error("the push trigger does not name the `epic-*` branches explicitly")
	}
}

// A superseded run must be cancelled, or a fast-follow push waits behind a
// run whose result nobody will read.
func TestVerifyWorkflowCancelsSupersededRuns(t *testing.T) {
	wf := readVerifyWorkflow(t)

	if !strings.Contains(wf, "concurrency:") {
		t.Fatal("the workflow declares no concurrency group at all")
	}
	if !strings.Contains(wf, "cancel-in-progress: true") {
		t.Error("the concurrency group does not cancel-in-progress: a superseded run is not cancelled")
	}
}

// Windows and macOS only: a Linux job would prove nothing about the
// platforms FairDrop ships to, and a cross-compiled or emulated result is
// never release proof.
func TestVerifyWorkflowRunsOnlyOnNativeWindowsAndMacRunners(t *testing.T) {
	wf := readVerifyWorkflow(t)

	if !strings.Contains(wf, "windows-latest") {
		t.Error("the workflow does not target windows-latest")
	}
	if !strings.Contains(wf, "macos-latest") {
		t.Error("the workflow does not target macos-latest")
	}
	lower := strings.ToLower(wf)
	if strings.Contains(lower, "ubuntu") {
		t.Error("the workflow references a ubuntu runner: a Linux job is Ask First and none was approved")
	}
	if strings.Contains(lower, "runs-on: linux") {
		t.Error("the workflow runs a job on Linux directly")
	}
}

// The Go floor is set by go.mod; the workflow's pin must be at or above it,
// and raising the floor itself is Ask First, so the workflow should not be
// the place that quietly does it.
func TestVerifyWorkflowPinsTheGoToolchain(t *testing.T) {
	wf := readVerifyWorkflow(t)

	if !strings.Contains(wf, "actions/setup-go@v7") {
		t.Error("the workflow does not use actions/setup-go@v7")
	}
	if !strings.Contains(wf, "go-version: '1.26.7'") {
		t.Error("the workflow does not pin go-version to '1.26.7'")
	}

	mod, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	if !strings.Contains(string(mod), "go 1.26") {
		t.Errorf("go.mod's floor no longer starts with go 1.26.x, so the workflow's 1.26.7 pin can no longer be assumed >= the floor: go.mod says %q", firstLine(string(mod), "go "))
	}
}

func firstLine(s, prefix string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			return strings.TrimSpace(line)
		}
	}
	return "(not found)"
}

// Node comes from .nvmrc, not a hardcoded version, so the two cannot drift
// apart; and the frontend cache is keyed to the lockfile that npm ci reads.
func TestVerifyWorkflowPinsNodeFromNvmrc(t *testing.T) {
	wf := readVerifyWorkflow(t)

	if !strings.Contains(wf, "actions/setup-node@v7") {
		t.Error("the workflow does not use actions/setup-node@v7")
	}
	if !strings.Contains(wf, "node-version-file: .nvmrc") {
		t.Error("the workflow does not read the Node version from .nvmrc")
	}
	if !strings.Contains(wf, "cache: npm") {
		t.Error("the workflow does not enable npm caching")
	}
	if !strings.Contains(wf, "cache-dependency-path: frontend/package-lock.json") {
		t.Error("the workflow's npm cache is not keyed to frontend/package-lock.json")
	}

	if _, err := os.Stat(".nvmrc"); err != nil {
		t.Errorf(".nvmrc is missing, so node-version-file has nothing to read: %v", err)
	}
}

// actions/checkout and actions/cache are the other two pinned action
// majors; a bump to any of the four is a deliberate decision, not drift.
func TestVerifyWorkflowPinsActionMajors(t *testing.T) {
	wf := readVerifyWorkflow(t)

	for _, action := range []string{
		"actions/checkout@v7",
		"actions/setup-go@v7",
		"actions/setup-node@v7",
		"actions/cache@v6",
	} {
		if !strings.Contains(wf, action) {
			t.Errorf("the workflow does not pin %s", action)
		}
	}
}

// The Wails CLI is not resolved from go.mod, so nothing else pins its
// version: a build from whatever `wails` happens to be cached or resolved
// is not the verified build. The version must be asserted, not just
// requested, so a stale cache cannot silently serve the wrong CLI.
func TestVerifyWorkflowPinsAndAssertsTheWailsCLIVersion(t *testing.T) {
	wf := readVerifyWorkflow(t)

	if !strings.Contains(wf, "WAILS_VERSION: 'v2.15.0'") {
		t.Error("the workflow does not pin WAILS_VERSION to 'v2.15.0'")
	}
	// The install must request the pinned version itself. An `@latest`
	// install is caught only by the assertion below, and only after the wrong
	// CLI is already on PATH -- which is a slower, less obvious failure than
	// never installing it.
	if !strings.Contains(wf, "go install github.com/wailsapp/wails/v2/cmd/wails@${{ env.WAILS_VERSION }}") {
		t.Error("the workflow does not install the wails CLI at the pinned ${{ env.WAILS_VERSION }}")
	}
	// Match the actual command substitution and comparison, not just the
	// words "wails version" or "$WAILS_VERSION" appearing anywhere -- both
	// also occur inside the step's own failure-message text, which must not
	// be enough to satisfy this pin.
	if !strings.Contains(wf, `got="$(wails version`) {
		t.Error("the workflow never captures the real output of `wails version` into a variable to assert against")
	}
	if !strings.Contains(wf, `!= "$WAILS_VERSION"`) {
		t.Error("the workflow's wails-version assertion does not compare the captured output against $WAILS_VERSION")
	}

	assertIdx := strings.Index(wf, "- name: Assert the Wails CLI is pinned")
	buildIdx := strings.Index(wf, "\n      - name: wails build\n")
	if assertIdx < 0 || buildIdx < 0 || assertIdx > buildIdx {
		t.Error("the wails version assertion does not run before `wails build`: an unpinned CLI could still build")
	}
}

// D-009: `npm run build` needs the devDependency-only Vite and tsc, so a
// production-flavored install can never build, and `--omit=dev` is
// therefore a build failure by construction, not a policy this test alone
// enforces. Pinning wails.json's frontend:install here (rather than a
// separate test) matches the Code Map, which puts both pins in this file.
func TestVerifyWorkflowNeverInstallsProductionOnly(t *testing.T) {
	wf := readVerifyWorkflow(t)
	wailsJSON := readWailsJSONForVerify(t)

	// Case-insensitive and without requiring the leading "--": npm honours
	// an OMIT=dev / NPM_CONFIG_OMIT=dev environment variable exactly like
	// the flag, so a mutation that swaps one for the other must still be
	// caught.
	if strings.Contains(strings.ToLower(wf), "omit=dev") {
		t.Error("the workflow omits dev dependencies from an npm install (flag or env var): " +
			"`npm run build` needs Vite and tsc, both devDependencies, and would fail")
	}
	if !strings.Contains(wailsJSON, `"frontend:install": "npm ci"`) {
		t.Error(`wails.json's "frontend:install" is no longer plain "npm ci": D-009 depends on this staying a full, dev-dependency install`)
	}
}

// Never: -upx (Apple Silicon and Windows antivirus risk, opt-in only),
// GOOS/GOARCH cross-builds (never release proof), a Linux job (covered by
// the runner test above, restated here for completeness against the exact
// forbidden strings).
func TestVerifyWorkflowNeverUsesAForbiddenFlag(t *testing.T) {
	wf := readVerifyWorkflow(t)

	for _, forbidden := range []string{"-upx", "GOOS=", "GOARCH="} {
		if strings.Contains(wf, forbidden) {
			t.Errorf("the workflow contains %q, which the spec's Never list forbids", forbidden)
		}
	}
	if strings.Contains(strings.ToLower(wf), "omit=dev") {
		t.Error(`the workflow omits dev dependencies from an npm install (flag or env var), which the spec's Never list forbids`)
	}
}

// Every step runs one at a time, in the fixed order the spec's Always
// clause states. This is the test that catches a reordered or deleted
// step: it locates each step's unique `- name:` heading and asserts the
// headings appear with strictly increasing offsets.
func TestVerifyWorkflowRunsEveryStepInTheRequiredOrder(t *testing.T) {
	wf := readVerifyWorkflow(t)

	steps := []string{
		"- name: wails build",
		"- name: Check for bindings drift and a restored .gitkeep",
		"- name: gofmt -l",
		"- name: go vet",
		"- name: staticcheck",
		"- name: go test",
		"- name: Confirm cgo is enabled",
		"- name: go test -race",
		"- name: Frontend suite",
		"- name: Line-ending check",
	}

	last := -1
	lastName := "(start of file)"
	for _, step := range steps {
		idx := strings.Index(wf, step)
		if idx < 0 {
			t.Errorf("step %q is missing entirely", step)
			continue
		}
		if idx <= last {
			t.Errorf("step %q appears at or before %q: the required order is wails build, "+
				"bindings-drift/.gitkeep, gofmt, go vet, staticcheck, go test, cgo check, "+
				"go test -race, frontend suite, line-ending check", step, lastName)
		}
		last = idx
		lastName = step
	}
}

// The race step's comment must say why it is native and C-toolchain
// dependent -- the Always clause requires this in words, not just in step
// order, and the cgo check must run before it so a runner that silently
// lost its C toolchain fails there instead of the race step "passing"
// having detected nothing.
func TestVerifyWorkflowRaceStepIsExplainedAndGuardedByACgoCheck(t *testing.T) {
	wf := readVerifyWorkflow(t)

	cgoIdx := strings.Index(wf, "- name: Confirm cgo is enabled")
	raceIdx := strings.Index(wf, "- name: go test -race")
	if cgoIdx < 0 || raceIdx < 0 {
		t.Fatal("the cgo check or the race step is missing")
	}
	if cgoIdx >= raceIdx {
		t.Fatal("the cgo check does not run before go test -race")
	}

	between := wf[cgoIdx:raceIdx]
	if !strings.Contains(strings.ToLower(between), "cgo_enabled") {
		t.Error("the cgo check does not inspect CGO_ENABLED")
	}

	comment := wf[cgoIdx:raceIdx]
	lower := strings.ToLower(comment)
	if !strings.Contains(lower, "native") {
		t.Error("the race step's surrounding comment does not say it depends on running natively")
	}
	if !strings.Contains(lower, "c toolchain") && !strings.Contains(lower, "cgo") {
		t.Error("the race step's surrounding comment does not say it depends on a C toolchain / cgo")
	}
}

// go env CGO_ENABLED alone can read 1 while a compiler is actually broken
// or absent, which is exactly the "looks clean, ran nothing" failure mode
// AGENTS.md warns about -- so the check must also try to build and run a
// trivial cgo program, not just read the environment variable.
func TestVerifyWorkflowCgoCheckActuallyBuildsACgoProgram(t *testing.T) {
	wf := readVerifyWorkflow(t)

	cgoIdx := strings.Index(wf, "- name: Confirm cgo is enabled")
	raceIdx := strings.Index(wf, "- name: go test -race")
	if cgoIdx < 0 || raceIdx < 0 || cgoIdx >= raceIdx {
		t.Fatal("the cgo check or the race step is missing or out of order")
	}
	step := wf[cgoIdx:raceIdx]

	if !strings.Contains(step, `import "C"`) {
		t.Error(`the cgo check never compiles a program with import "C": CGO_ENABLED=1 alone does not prove the C toolchain works`)
	}
	if !strings.Contains(step, "go run") {
		t.Error("the cgo check never actually runs the trivial cgo program it writes")
	}
}

// The Always clause fixes both the linter (staticcheck, not golangci-lint)
// and how it is invoked (the go.mod tool directive, not a separately
// installed binary), and fixes -count=1 on both go test invocations so a
// cached pass can never stand in for a run.
func TestVerifyWorkflowRunsTheFixedGoCommands(t *testing.T) {
	wf := readVerifyWorkflow(t)

	for _, want := range []string{
		"gofmt -l .",
		"go vet ./...",
		"go tool staticcheck ./...",
		"go test -count=1 ./...",
		"go test -count=1 -race ./...",
	} {
		if !strings.Contains(wf, want) {
			t.Errorf("the workflow does not run %q", want)
		}
	}
	if strings.Contains(wf, "golangci-lint") {
		t.Error("the workflow runs golangci-lint: staticcheck is the fixed linter and swapping it is Ask First")
	}
}

// The frontend suite must run with dev dependencies installed and must not
// run concurrently with a `wails build`/`npm ci` in the same job -- the
// workflow already runs every step serially, so this test only pins that
// the step exists and runs vitest through `npm test`.
func TestVerifyWorkflowRunsTheFrontendSuite(t *testing.T) {
	wf := readVerifyWorkflow(t)

	idx := strings.Index(wf, "- name: Frontend suite")
	if idx < 0 {
		t.Fatal("the frontend suite step is missing")
	}
	block := wf[idx:]
	if end := strings.Index(block[1:], "- name:"); end >= 0 {
		block = block[:end+1]
	}
	if !strings.Contains(block, "working-directory: frontend") {
		t.Error("the frontend suite step does not run inside frontend/")
	}
	if !strings.Contains(block, "npm test") {
		t.Error("the frontend suite step does not run `npm test`")
	}
}

// The exact check the spec's Verification section names: the index is
// already all-LF, so after the one-time worktree rewrite this must find
// nothing, on every future commit.
func TestVerifyWorkflowChecksLineEndings(t *testing.T) {
	wf := readVerifyWorkflow(t)

	want := `git ls-files --eol | grep -E '(i|w)/(crlf|mixed)'`
	if !strings.Contains(wf, want) {
		t.Errorf("the workflow's line-ending check is not %q", want)
	}
}

// wails build regenerates frontend/wailsjs and can drop frontend/dist/
// .gitkeep; both must be checked immediately after the build, before any
// other step runs against a possibly-stale tree.
func TestVerifyWorkflowChecksBindingsDriftAndGitkeep(t *testing.T) {
	wf := readVerifyWorkflow(t)

	idx := strings.Index(wf, "- name: Check for bindings drift and a restored .gitkeep")
	if idx < 0 {
		t.Fatal("the bindings-drift / .gitkeep step is missing")
	}
	block := wf[idx:]
	if end := strings.Index(block[1:], "- name:"); end >= 0 {
		block = block[:end+1]
	}
	if !strings.Contains(block, "frontend/dist/.gitkeep") {
		t.Error("the bindings-drift step does not check frontend/dist/.gitkeep")
	}
	if !strings.Contains(block, "frontend/wailsjs") {
		t.Error("the bindings-drift step does not check frontend/wailsjs for drift")
	}
	// The functional command, not merely the substring "git diff": the step
	// also runs `git diff -- frontend/wailsjs` inside its failure branch to
	// print what drifted, so a check for the substring alone stays green when
	// the guarding condition itself is deleted.
	if !strings.Contains(block, "git diff --quiet -- frontend/wailsjs") {
		t.Error("the bindings-drift step does not run `git diff --quiet -- frontend/wailsjs` as its condition")
	}
}

// Every step is bash, and $(go env GOPATH)/bin is added to $GITHUB_PATH
// explicitly rather than assumed, per the Code Map.
func TestVerifyWorkflowUsesBashAndAddsGoPathBinExplicitly(t *testing.T) {
	wf := readVerifyWorkflow(t)

	if !strings.Contains(wf, "shell: bash") {
		t.Error("the workflow does not set shell: bash")
	}
	// The actual functional command, not just the words "go env GOPATH" and
	// "GITHUB_PATH" appearing anywhere -- both also occur in this step's own
	// explanatory comment, which must not be enough to satisfy this pin.
	if !strings.Contains(wf, `echo "$(go env GOPATH)/bin" >> "$GITHUB_PATH"`) {
		t.Error("the workflow does not explicitly run the command adding $(go env GOPATH)/bin to $GITHUB_PATH")
	}
}
