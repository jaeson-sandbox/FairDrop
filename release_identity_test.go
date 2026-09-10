package main

// release_identity_test.go pins the five identity facts a release artifact
// must agree on -- name, output filename, product name, version, and
// platform identifier -- across wails.json (the single source every other
// file follows) and the files that restate or template them:
// build/darwin/Info.plist, build/windows/info.json, and main.go's window
// title. It also pins the tag-to-version rule release.yml enforces before it
// builds anything, that the QR dependency is wired rather than inactive, and
// that no shipped surface still says the stale "DeadDrop" name.
//
// Like TestEveryDeferredEntryHasALiveOwner in main_test.go, these read repo
// files as text -- wails.json is small enough to also decode as JSON, which
// catches a renamed field a substring match would miss -- rather than
// executing Wails' own template engine. The two platform templates are read
// here exactly as a release build reads them: verbatim source with
// unresolved {{ }} placeholders. A placeholder silently replaced by a
// hardcoded literal is exactly the drift this file exists to catch, and a
// literal changed in one file without its counterpart is what makes a fact
// "disagree" rather than merely differ from what this test expected.

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type wailsInfo struct {
	CompanyName    string `json:"companyName"`
	ProductName    string `json:"productName"`
	ProductVersion string `json:"productVersion"`
	Copyright      string `json:"copyright"`
	Comments       string `json:"comments"`
}

type wailsProject struct {
	Name           string    `json:"name"`
	OutputFilename string    `json:"outputfilename"`
	Info           wailsInfo `json:"info"`
}

func readWailsProject(t *testing.T) wailsProject {
	t.Helper()
	data, err := os.ReadFile("wails.json")
	if err != nil {
		t.Fatalf("read wails.json: %v", err)
	}
	var project wailsProject
	if err := json.Unmarshal(data, &project); err != nil {
		t.Fatalf("parse wails.json: %v", err)
	}
	return project
}

// readTextFile normalises CRLF so this file's assertions are independent of
// the Windows worktree's core.autocrlf, the same reasoning main_test.go's
// splitLines documents.
func readTextFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

var (
	windowTitlePattern = regexp.MustCompile(`Title:\s*"([^"]*)"`)
	semverPattern      = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
)

// plistKeyValue extracts the <string> value immediately following a <key>
// entry in a Wails Info.plist template.
func plistKeyValue(t *testing.T, plist, path, key string) string {
	t.Helper()
	pattern := regexp.MustCompile(`(?s)<key>` + regexp.QuoteMeta(key) + `</key>\s*<string>(.*?)</string>`)
	m := pattern.FindStringSubmatch(plist)
	if m == nil {
		t.Fatalf("%s: could not find <key>%s</key> followed by a <string> value", path, key)
	}
	return m[1]
}

func assertPlistKeyValue(t *testing.T, plist, path, key, want string) {
	t.Helper()
	got := plistKeyValue(t, plist, path, key)
	if got != want {
		t.Errorf("%s's %s = %q, want %q", path, key, got, want)
	}
}

// TestReleaseIdentityAgreesAcrossWailsJSONTemplatesAndMainGo pins wails.json
// as the single source of truth for FairDrop's identity, and pins every
// other file that names or templates it against that source rather than
// against a second hardcoded literal -- so a value changed in exactly one
// file, alone, fails here naming the file and the value that disagreed.
func TestReleaseIdentityAgreesAcrossWailsJSONTemplatesAndMainGo(t *testing.T) {
	project := readWailsProject(t)

	// wails.json itself: the five identity facts, pinned as literals. This is
	// the source every other file below is checked against, so a rename here
	// alone is exactly what the rest of this test is built to catch -- none
	// of it would fail without this anchor.
	if project.Name != "fairdrop" {
		t.Errorf("wails.json's name = %q, want %q: it drives build/bin/<name>.app on macOS and half of the bundle identifier", project.Name, "fairdrop")
	}
	if project.OutputFilename != "fairdrop" {
		t.Errorf("wails.json's outputfilename = %q, want %q: it drives build/bin/<outputfilename>.exe on Windows and CFBundleExecutable on macOS", project.OutputFilename, "fairdrop")
	}
	if project.Info.ProductName != "FairDrop" {
		t.Errorf("wails.json's info.productName = %q, want %q", project.Info.ProductName, "FairDrop")
	}
	if !semverPattern.MatchString(project.Info.ProductVersion) {
		t.Errorf("wails.json's info.productVersion = %q, not a plain x.y.z version the release tag rule can compare against", project.Info.ProductVersion)
	}

	// main.go: the window title is the one place outside wails.json that
	// restates productName as a second literal rather than a template, so it
	// is asserted equal to wails.json's value -- not to a second hardcoded
	// "FairDrop" -- or a rename in wails.json alone would leave this test
	// agreeing with itself instead of with the source.
	mainGo := readTextFile(t, "main.go")
	titleMatch := windowTitlePattern.FindStringSubmatch(mainGo)
	if titleMatch == nil {
		t.Fatal(`main.go: could not find Title: "..." inside appOptions`)
	}
	if titleMatch[1] != project.Info.ProductName {
		t.Errorf("main.go's window Title = %q, wails.json's info.productName = %q: they disagree", titleMatch[1], project.Info.ProductName)
	}

	// build/darwin/Info.plist: every identity field must still be the Wails
	// template placeholder tying it back to wails.json. A hardcoded literal
	// here -- even a correct-looking one -- is exactly the drift this test
	// exists to catch, because it would silently stop following wails.json on
	// the next version bump.
	plistPath := filepath.Join("build", "darwin", "Info.plist")
	plist := readTextFile(t, plistPath)
	assertPlistKeyValue(t, plist, plistPath, "CFBundleName", "{{.Info.ProductName}}")
	assertPlistKeyValue(t, plist, plistPath, "CFBundleExecutable", "{{.OutputFilename}}")
	assertPlistKeyValue(t, plist, plistPath, "CFBundleVersion", "{{.Info.ProductVersion}}")
	assertPlistKeyValue(t, plist, plistPath, "CFBundleShortVersionString", "{{.Info.ProductVersion}}")

	// The platform identity: com.wails.* named Wails, not FairDrop, before
	// this story. It must now name FairDrop and must still derive the
	// project-specific suffix from wails.json's name via safeBundleID, not a
	// second hardcoded string.
	identifier := plistKeyValue(t, plist, plistPath, "CFBundleIdentifier")
	if strings.Contains(strings.ToLower(identifier), "wails") {
		t.Errorf("%s's CFBundleIdentifier is %q: the platform identity names Wails, not FairDrop", plistPath, identifier)
	}
	if !strings.Contains(strings.ToLower(identifier), "fairdrop") {
		t.Errorf("%s's CFBundleIdentifier is %q: it does not name FairDrop at all", plistPath, identifier)
	}
	if !strings.Contains(identifier, "{{safeBundleID .Name}}") {
		t.Errorf("%s's CFBundleIdentifier is %q: it no longer derives from wails.json's name via {{safeBundleID .Name}}", plistPath, identifier)
	}

	// build/windows/info.json: every field templates from wails.json's info
	// block with no edit needed -- confirmed here, not changed, per the Code
	// Map.
	winInfoPath := filepath.Join("build", "windows", "info.json")
	winInfo := readTextFile(t, winInfoPath)
	for _, want := range []string{
		`"file_version": "{{.Info.ProductVersion}}"`,
		`"ProductVersion": "{{.Info.ProductVersion}}"`,
		`"CompanyName": "{{.Info.CompanyName}}"`,
		`"FileDescription": "{{.Info.ProductName}}"`,
		`"ProductName": "{{.Info.ProductName}}"`,
	} {
		if !strings.Contains(winInfo, want) {
			t.Errorf("%s does not contain %s: Windows metadata would stop following wails.json", winInfoPath, want)
		}
	}
}

const releaseWorkflowPath = "release.yml"

func readReleaseWorkflow(t *testing.T) string {
	t.Helper()
	return readTextFile(t, filepath.Join(".github", "workflows", releaseWorkflowPath))
}

// jobBlock isolates one top-level job's YAML text, from its `  <job>:` line
// up to (but excluding) the next top-level job named by nextJob, or to the
// end of the file when nextJob is "". Jobs in release.yml are declared in
// the fixed order gate, build, release.
func jobBlock(t *testing.T, wf, job, nextJob string) string {
	t.Helper()
	marker := "\n  " + job + ":\n"
	start := strings.Index(wf, marker)
	if start < 0 {
		t.Fatalf("release.yml has no top-level job %q", job)
	}
	start++ // keep the leading "  job:\n", drop only the newline before it
	if nextJob == "" {
		return wf[start:]
	}
	endMarker := "\n  " + nextJob + ":\n"
	end := strings.Index(wf[start:], endMarker)
	if end < 0 {
		t.Fatalf("release.yml's job %q never ends before job %q", job, nextJob)
	}
	return wf[start : start+end]
}

// TestReleaseWorkflowAssertsTheTagMatchesProductVersionBeforeBuilding pins
// the tag-to-version rule: a tag whose version disagrees with wails.json's
// productVersion must fail before any artifact is built, naming both values,
// and the check must not run on a workflow_dispatch, which carries no tag to
// compare.
func TestReleaseWorkflowAssertsTheTagMatchesProductVersionBeforeBuilding(t *testing.T) {
	wf := readReleaseWorkflow(t)

	stepIdx := strings.Index(wf, "- name: Assert the tag matches wails.json's productVersion")
	buildIdx := strings.Index(wf, "\n      - name: wails build\n")
	if stepIdx < 0 {
		t.Fatal("release.yml has no step asserting the tag matches wails.json's productVersion")
	}
	if buildIdx < 0 {
		t.Fatal("release.yml has no `wails build` step")
	}
	if stepIdx > buildIdx {
		t.Fatal("the tag/productVersion assertion does not run before `wails build`: a mismatched tag could still build")
	}

	step := wf[stepIdx:buildIdx]
	if !strings.Contains(step, "if: github.event_name == 'push'") {
		t.Error("the tag/productVersion assertion has no `if: github.event_name == 'push'` guard: it would also run -- and fail for lack of a tag -- on a workflow_dispatch")
	}
	if !strings.Contains(step, `tag="${GITHUB_REF_NAME#v}"`) {
		t.Error("the assertion does not strip the leading 'v' from GITHUB_REF_NAME before comparing")
	}
	if !strings.Contains(step, `jq -r '.info.productVersion' wails.json`) {
		t.Error("the assertion does not read productVersion out of wails.json with jq")
	}
	if !strings.Contains(step, `"$tag" != "$productVersion"`) {
		t.Error("the assertion does not compare the tag's version against wails.json's productVersion")
	}
	if !strings.Contains(step, "exit 1") {
		t.Error("the assertion does not fail the job on a mismatch")
	}
	if !strings.Contains(step, "$GITHUB_REF_NAME") || !strings.Contains(step, "$productVersion") {
		t.Error("the assertion's failure message does not name both the tag and wails.json's productVersion")
	}
}

// TestReleaseWorkflowGatesBeforeBuildingAndNeverCrossCompiles pins the
// spec's Always clause: the full Story 3.2 gate runs to completion, on the
// same native-runner matrix, before any artifact is built, and no step ever
// cross-compiles or defaults to UPX compression.
func TestReleaseWorkflowGatesBeforeBuildingAndNeverCrossCompiles(t *testing.T) {
	wf := readReleaseWorkflow(t)

	gateJob := jobBlock(t, wf, "gate", "build")
	if !strings.Contains(gateJob, "uses: ./.github/workflows/verify.yml") {
		t.Error("release.yml's gate job does not reuse verify.yml through workflow_call")
	}

	buildJob := jobBlock(t, wf, "build", "release")
	if !strings.Contains(buildJob, "needs: gate") {
		t.Error("release.yml's build job does not `needs: gate`: an artifact could build before the gate passes")
	}
	if !strings.Contains(buildJob, "windows-latest") || !strings.Contains(buildJob, "macos-latest") {
		t.Error("release.yml's build matrix does not target both windows-latest and macos-latest")
	}
	if strings.Contains(strings.ToLower(buildJob), "ubuntu") {
		t.Error("release.yml's build job references a Linux runner: no artifact may be built on one")
	}
	for _, forbidden := range []string{"GOOS=", "GOARCH="} {
		if strings.Contains(buildJob, forbidden) {
			t.Errorf("release.yml's build job contains %q: no step may cross-compile", forbidden)
		}
	}

	// UPX must be reachable only through the dispatch input, and off by
	// default: an unconditional `-upx` anywhere in the build job would
	// compress every release regardless of the input.
	if !strings.Contains(buildJob, `if [ "${{ inputs.upx }}" = "true" ]`) {
		t.Error("release.yml's build job does not gate -upx behind the workflow_dispatch upx input")
	}
	if !strings.Contains(buildJob, "wails build -upx") {
		t.Error("release.yml's build job never passes -upx, so the opt-in input would do nothing")
	}
	unconditional := regexp.MustCompile(`(?m)^\s*wails build\s*$`)
	if !unconditional.MatchString(buildJob) {
		t.Error("release.yml's build job has no plain `wails build` (no -upx) branch: the default run would always compress")
	}

	if !strings.Contains(buildJob, "ditto -c -k --sequesterRsrc --keepParent fairdrop.app fairdrop-macos.zip") {
		t.Error("release.yml does not zip the macOS bundle with ditto's resource-fork-preserving flags")
	}
	if !strings.Contains(buildJob, "shasum -a 256 fairdrop-macos.zip") {
		t.Error("release.yml does not SHA-256 checksum the macOS artifact")
	}
	if !strings.Contains(buildJob, "sha256sum fairdrop.exe") {
		t.Error("release.yml does not SHA-256 checksum the Windows artifact")
	}
	if !strings.Contains(buildJob, "actions/upload-artifact@v7") {
		t.Error("release.yml does not upload artifacts with actions/upload-artifact@v7")
	}
}

// TestReleaseWorkflowOnlyDraftsOnATagAndTrustsNoThirdPartyAction pins two
// Always/Never clauses at once: publishing anything beyond a draft is
// Ask First, so the release job must run only for a tag push and must create
// the release itself with the first-party `gh` CLI rather than a third-party
// action that would also hold the repository token.
func TestReleaseWorkflowOnlyDraftsOnATagAndTrustsNoThirdPartyAction(t *testing.T) {
	wf := readReleaseWorkflow(t)
	releaseJob := jobBlock(t, wf, "release", "")

	if !strings.Contains(releaseJob, "needs: build") {
		t.Error("release.yml's release job does not `needs: build`: it could run before artifacts exist")
	}
	if !strings.Contains(releaseJob, "if: github.event_name == 'push'") {
		t.Error("release.yml's release job has no `if: github.event_name == 'push'` guard: a workflow_dispatch run would also create a release")
	}
	if !strings.Contains(releaseJob, "gh release create") {
		t.Fatal("release.yml's release job does not run `gh release create`")
	}
	if !strings.Contains(releaseJob, "--draft") {
		t.Error("release.yml's release job does not pass --draft: publishing a release is Ask First on this public repository")
	}

	// Nothing outside the pinned first-party actions/* set may hold the
	// token this job carries (permissions: contents: write).
	usesPattern := regexp.MustCompile(`(?m)^\s*uses:\s*(\S+)`)
	found := false
	for _, m := range usesPattern.FindAllStringSubmatch(releaseJob, -1) {
		found = true
		if !strings.HasPrefix(m[1], "actions/") {
			t.Errorf("release.yml's release job uses %q: only first-party actions/* actions may run in a job holding the release token", m[1])
		}
	}
	if !found {
		t.Fatal("release.yml's release job uses no actions at all, so this test would pass vacuously")
	}
}

// shippedSurfaceFiles lists the paths a release ships or documents, plus the
// two source trees that compile into it. docs/fairdrop-spec.md is
// deliberately absent here: it is the one exempted historical document,
// asserted separately below.
func shippedSurfaceFiles(t *testing.T) []string {
	t.Helper()

	files := []string{
		"README.md",
		"wails.json",
		"main.go",
		"app.go",
		filepath.Join("build", "darwin", "Info.plist"),
		filepath.Join("build", "darwin", "Info.dev.plist"),
		filepath.Join("build", "windows", "info.json"),
		filepath.Join("docs", "fairdrop-architecture.md"),
		filepath.Join("docs", "fairdrop-contracts.md"),
		filepath.Join("_bmad-output", "specs", "spec-fairdrop", "SPEC.md"),
		filepath.Join("_bmad-output", "planning-artifacts", "ux-designs", "ux-FairDrop-2026-08-23", "EXPERIENCE.md"),
		filepath.Join("_bmad-output", "planning-artifacts", "ux-designs", "ux-FairDrop-2026-08-23", "DESIGN.md"),
	}

	for _, root := range []string{
		"internal",
		filepath.Join("frontend", "src"),
		filepath.Join("frontend", "wailsjs"),
	} {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			name := d.Name()
			// Test files are not shipped, and this file's own literal
			// "DeadDrop" strings (in the exemption logic below) must never
			// be scanned as if they were product content.
			if strings.HasSuffix(name, "_test.go") || strings.Contains(name, ".test.") {
				return nil
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}

	return files
}

// TestNoShippedSurfaceNamesTheStaleProduct pins the other half of the
// Story 3.3 naming acceptance criterion: no stale "DeadDrop" name survives
// in shipped metadata, source, or documentation. docs/fairdrop-spec.md is
// the sole, explicit exception -- the original working spec, kept for
// traceability, whose own banner already records the name as stale.
func TestNoShippedSurfaceNamesTheStaleProduct(t *testing.T) {
	files := shippedSurfaceFiles(t)
	if len(files) < 10 {
		t.Fatalf("only %d shipped-surface files were collected, so this test would pass close to vacuously", len(files))
	}

	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.Contains(string(data), "DeadDrop") {
			t.Errorf("%s still says DeadDrop: the product is FairDrop", path)
		}
	}

	// Assert the exemption is still needed, not merely still declared: if
	// docs/fairdrop-spec.md is ever cleaned up so it no longer says
	// DeadDrop, this fails and says so, rather than leaving a permanent,
	// silently unused carve-out in the test above.
	historicalPath := filepath.Join("docs", "fairdrop-spec.md")
	historical, err := os.ReadFile(historicalPath)
	if err != nil {
		t.Fatalf("read %s: %v", historicalPath, err)
	}
	if !strings.Contains(string(historical), "DeadDrop") {
		t.Fatalf("%s no longer says DeadDrop: remove its exemption from this test instead of leaving it unused", historicalPath)
	}
}

// TestQRDependencyIsLiveNotInactive pins the other clause of the same
// acceptance criterion: internal/qr is wired into the composed application,
// not dead code nobody imports, and no inactive QR package duplicates it in
// the frontend.
func TestQRDependencyIsLiveNotInactive(t *testing.T) {
	mainGo := readTextFile(t, "main.go")
	if !strings.Contains(mainGo, `"fairdrop/internal/qr"`) {
		t.Error("main.go does not import fairdrop/internal/qr: the QR dependency would be dead code")
	}
	if !strings.Contains(mainGo, "qr.New()") {
		t.Error("main.go never calls qr.New(): the QR dependency would be imported but inactive")
	}

	pkgPath := filepath.Join("frontend", "package.json")
	pkg, err := os.ReadFile(pkgPath)
	if err != nil {
		t.Fatalf("read %s: %v", pkgPath, err)
	}
	if strings.Contains(strings.ToLower(string(pkg)), "qr") {
		t.Errorf("%s mentions \"qr\": QR encoding is Go-only (internal/qr), so a frontend QR package would be an inactive duplicate", pkgPath)
	}
}
