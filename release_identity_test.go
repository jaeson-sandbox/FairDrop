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
	// The matrix line itself, not the two names appearing anywhere in the job:
	// a comment mentioning either would otherwise satisfy this.
	if !strings.Contains(buildJob, "os: [windows-latest, macos-latest]") {
		t.Error("release.yml's build matrix line does not read exactly `os: [windows-latest, macos-latest]`")
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
	if !strings.Contains(buildJob, "UPX: ${{ inputs.upx }}") {
		t.Error("release.yml's build job does not carry the upx input into the step environment")
	}
	if !strings.Contains(buildJob, `if [ "$UPX" = "true" ]`) {
		t.Error("release.yml's build job does not gate -upx behind the workflow_dispatch upx input")
	}
	// The input must reach the script through the environment, never expanded
	// into it. A typed boolean cannot carry shell metacharacters today; the
	// pattern becomes an injection the moment somebody widens the input's type,
	// and it is what Actions static analysis flags.
	if strings.Contains(buildJob, `[ "${{ inputs.upx }}"`) {
		t.Error("release.yml expands the upx input directly into a shell condition rather than passing it through env")
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
	// Both upload steps, counted rather than matched once: the job has one per
	// platform, and a single match cannot tell whether the other was changed.
	if got := strings.Count(buildJob, "uses: actions/upload-artifact@v7"); got != 2 {
		t.Errorf("release.yml's build job has %d pinned upload-artifact steps, want 2 (one per platform)", got)
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
		// The workflows are shipped surfaces too: release.yml writes the
		// notes on the page a downloader reads, and names the artifacts.
		filepath.Join(".github", "workflows", "release.yml"),
		filepath.Join(".github", "workflows", "verify.yml"),
		filepath.Join("frontend", "package.json"),
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

// TestVerifyWorkflowStaysReusableByTheReleaseWorkflow pins the other half of
// the gate reuse.
//
// TestReleaseWorkflowGatesBeforeBuildingAndNeverCrossCompiles proves
// release.yml asks for verify.yml through workflow_call. Nothing proved
// verify.yml still offers it. Removing that one line from verify.yml left
// every test in this repository green while making every future release fail
// at run time, with an error about an invalid workflow reference rather than
// about the trigger somebody deleted -- found by mutation after the
// implementation reported it as a known gap.
func TestVerifyWorkflowStaysReusableByTheReleaseWorkflow(t *testing.T) {
	verify := readTextFile(t, filepath.Join(".github", "workflows", "verify.yml"))

	triggers, _, found := strings.Cut(verify, "\njobs:")
	if !found {
		t.Fatal("verify.yml has no jobs: block, so this test would pass vacuously")
	}
	if !strings.Contains(triggers, "workflow_call:") {
		t.Error("verify.yml no longer declares workflow_call: release.yml reuses it as its gate, " +
			"and without that trigger every release fails at run time complaining about an " +
			"invalid workflow reference rather than about the deleted trigger")
	}
}

// TestTheVersionIsStatedOnceAndFollowedEverywhere pins the second copy of the
// product version.
//
// wails.json's productVersion is what both platform templates resolve, so
// changing it alone cannot make those two disagree -- correctly, since bumping
// it is exactly what a maintainer does before tagging. frontend/package.json
// carries its own "version" that nothing resolves and nothing shipped reads,
// which is precisely why it drifts unnoticed: it looks authoritative and is
// not. Pinning the two together makes a version bump one coherent act rather
// than a thing to remember twice.
func TestTheVersionIsStatedOnceAndFollowedEverywhere(t *testing.T) {
	var wails struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	raw, err := os.ReadFile("wails.json")
	if err != nil {
		t.Fatalf("read wails.json: %v", err)
	}
	if err := json.Unmarshal(raw, &wails); err != nil {
		t.Fatalf("decode wails.json: %v", err)
	}
	if wails.Info.ProductVersion == "" {
		t.Fatal("wails.json declares no productVersion, so this test would pass vacuously")
	}

	var pkg struct {
		Version string `json:"version"`
	}
	pkgPath := filepath.Join("frontend", "package.json")
	raw, err = os.ReadFile(pkgPath)
	if err != nil {
		t.Fatalf("read %s: %v", pkgPath, err)
	}
	if err := json.Unmarshal(raw, &pkg); err != nil {
		t.Fatalf("decode %s: %v", pkgPath, err)
	}
	if pkg.Version != wails.Info.ProductVersion {
		t.Errorf("%s says version %q but wails.json says productVersion %q: "+
			"the version is stated twice and these two disagree, so one of them is wrong "+
			"about what this build is", pkgPath, pkg.Version, wails.Info.ProductVersion)
	}
}

// TestReleaseWorkflowHoldsTheSmallestTokenAndChecksWhatItPublishes pins two
// properties of the only job in this repository that is ever handed a
// write-capable token.
//
// The first is least privilege stated rather than inherited. The repository's
// default workflow token is currently read-only, so every job here would be
// read-only even without the declaration -- but that default is a repository
// setting a person can change in a web form, silently granting write to every
// job in every workflow. Declaring it in the file means only the job that must
// write a release can.
//
// The second is that the checksum published beside an artifact describes the
// artifact that was published, not one that was identical to it before it
// crossed the artifact store. Computing a checksum on the build runner and
// then shipping both without re-checking makes the checksum a claim about a
// file that is no longer the one anybody downloads.
func TestReleaseWorkflowHoldsTheSmallestTokenAndChecksWhatItPublishes(t *testing.T) {
	release := readTextFile(t, filepath.Join(".github", "workflows", "release.yml"))

	preamble, _, found := strings.Cut(release, "\njobs:")
	if !found {
		t.Fatal("release.yml has no jobs: block, so this test would pass vacuously")
	}
	if !strings.Contains(preamble, "permissions:") || !strings.Contains(preamble, "contents: read") {
		t.Error("release.yml declares no workflow-level `permissions: contents: read`: " +
			"every job would then inherit whatever the repository default happens to be, " +
			"which a person can change without touching this file")
	}

	releaseJob := jobBlock(t, release, "release", "")
	if !strings.Contains(releaseJob, "contents: write") {
		t.Error("the release job does not elevate to contents: write, so it cannot create a release")
	}

	verifyIdx := strings.Index(releaseJob, "sha256sum --check")
	publishIdx := strings.Index(releaseJob, "gh release create")
	if verifyIdx < 0 {
		t.Error("the release job never re-checks a downloaded checksum: the published checksum " +
			"would describe the artifact as it was before it crossed the artifact store")
	}
	if publishIdx < 0 {
		t.Fatal("the release job does not run `gh release create`, so ordering cannot be checked")
	}
	if verifyIdx >= 0 && verifyIdx > publishIdx {
		t.Error("the checksum re-check runs after `gh release create`: a corrupted artifact " +
			"would already be published by the time it failed")
	}
}

// TestBothWorkflowsPinTheSameWailsCLI pins the invariant release.yml states
// about itself and nothing enforced.
//
// The build job does not reuse verify.yml's steps -- only the gate job does,
// through workflow_call -- so it re-declares its own Go, Node and Wails CLI
// setup as independent text, including a second copy of the version literal.
// Changing that copy alone drifts silently: the release job's own assertion
// compares the installed CLI against its own literal, so the drift is
// self-consistent and no check anywhere fails. The artifact people download
// would then be built by a toolchain the gate never verified, which is the
// one thing the comment above that literal promises cannot happen.
func TestBothWorkflowsPinTheSameWailsCLI(t *testing.T) {
	pin := regexp.MustCompile(`(?m)^\s*WAILS_VERSION:\s*'([^']+)'\s*$`)

	versions := map[string]string{}
	for _, name := range []string{"verify.yml", "release.yml"} {
		workflow := readTextFile(t, filepath.Join(".github", "workflows", name))
		match := pin.FindStringSubmatch(workflow)
		if match == nil {
			t.Fatalf("%s declares no WAILS_VERSION, so this test would pass vacuously", name)
		}
		versions[name] = match[1]
	}

	if versions["verify.yml"] != versions["release.yml"] {
		t.Errorf("verify.yml pins the Wails CLI at %q and release.yml at %q: "+
			"the release build job does not reuse verify.yml's steps, so a release would be "+
			"built by a CLI the gate never verified, and each file's own assertion would still pass",
			versions["verify.yml"], versions["release.yml"])
	}
}

// TestTheDraftReleaseNotesStateWhatTheBuildIsNot pins the claims on the page a
// downloader reads before running an unsigned binary.
//
// These are the same class of claim the project treats as load-bearing
// everywhere else, and they were the one shipped surface no assertion touched:
// deleting "no notarization", or inverting it, left every test green. The
// checksum sentence matters too -- both the binary and its checksum come from
// this one pipeline, so the checksum proves the download arrived intact and
// nothing about whether the pipeline was tampered with, and the notes must not
// imply otherwise.
// TestBothDarwinPlistsShareOneBundleIdentity pins the dev bundle to the
// release bundle.
//
// build/darwin/Info.dev.plist is what `wails dev` packages, and it carried
// Wails' default com.wails.* identifier after the release plist was changed to
// com.fairdrop.*. macOS keys per-app state -- TCC permission grants among it --
// to the bundle identifier, so two identifiers means the development build and
// the shipped build are different applications to the operating system, and a
// permission granted while developing says nothing about the one users run.
func TestBothDarwinPlistsShareOneBundleIdentity(t *testing.T) {
	identifiers := map[string]string{}
	for _, name := range []string{"Info.plist", "Info.dev.plist"} {
		path := filepath.Join("build", "darwin", name)
		identifier := plistKeyValue(t, readTextFile(t, path), path, "CFBundleIdentifier")
		if identifier == "" {
			t.Fatalf("%s declares no CFBundleIdentifier, so this test would pass vacuously", path)
		}
		identifiers[name] = identifier
		if strings.Contains(strings.ToLower(identifier), "wails") {
			t.Errorf("%s's CFBundleIdentifier is %q: the platform identity names the toolkit, not the product",
				path, identifier)
		}
	}
	if identifiers["Info.plist"] != identifiers["Info.dev.plist"] {
		t.Errorf("the release bundle identifies as %q and the dev bundle as %q: macOS treats them as two "+
			"different applications, so per-app state granted to one says nothing about the other",
			identifiers["Info.plist"], identifiers["Info.dev.plist"])
	}
}

func TestTheDraftReleaseNotesStateWhatTheBuildIsNot(t *testing.T) {
	release := readTextFile(t, filepath.Join(".github", "workflows", "release.yml"))

	notes := jobBlock(t, release, "release", "")
	if !strings.Contains(notes, "release-notes.md") {
		t.Fatal("the release job builds no release-notes.md, so this test would pass vacuously")
	}

	for _, required := range []struct{ phrase, why string }{
		{"plain HTTP", "the transport is not encrypted and the notes must say so"},
		{"does not protect against an", "the capability URL reduces discovery, it does not protect the payload"},
		{"no end-to-end encryption", "the banned-claim list starts here"},
		{"ad-hoc", "the macOS binary carries an ad-hoc signature and the notes must explain what that is"},
		{"codesign --sign -", "naming the exact command is what stops \"signed\" reading as \"signed by an identity\""},
		{"no notarization", "the macOS binary is ad-hoc signed only"},
		{"no auto-update", "there is no update channel"},
		{"no Linux packaging", "there is no Linux artifact"},
		{"It is not a signature", "a checksum from the same pipeline proves delivery, not provenance"},
	} {
		if !strings.Contains(notes, required.phrase) {
			t.Errorf("the draft release notes no longer say %q: %s", required.phrase, required.why)
		}
	}

	// The inverse claims, which must never appear.
	for _, forbidden := range []string{"is notarized", "is signed with", "auto-updates", "end-to-end encrypted"} {
		if strings.Contains(notes, forbidden) {
			t.Errorf("the draft release notes claim %q, which this build does not do", forbidden)
		}
	}
}

// TestTheReleaseWorkflowSurvivesARetryAndCannotRaceItself pins two properties
// that only show up on the unhappy path.
//
// `gh release create` fails outright when a release already exists for the
// tag, so without the existence check any failure after publishing turned a
// retry into "a person must delete the draft first". And two runs for one tag
// -- a re-pushed tag, or a dispatch overlapping a tag push -- had nothing
// stopping them racing to publish. The concurrency group deliberately does not
// cancel the run in flight: a superseded verification is worth nothing, but a
// half-finished publish is worth waiting for.
func TestTheReleaseWorkflowSurvivesARetryAndCannotRaceItself(t *testing.T) {
	release := readTextFile(t, filepath.Join(".github", "workflows", "release.yml"))

	preamble, _, found := strings.Cut(release, "\njobs:")
	if !found {
		t.Fatal("release.yml has no jobs: block, so this test would pass vacuously")
	}
	if !strings.Contains(preamble, "concurrency:") {
		t.Error("release.yml declares no concurrency group: two runs for one tag could race to publish")
	}
	if !strings.Contains(preamble, "cancel-in-progress: false") {
		t.Error("release.yml cancels a release run already in flight: a half-finished publish is worth waiting for, " +
			"unlike a superseded verification")
	}

	releaseJob := jobBlock(t, release, "release", "")
	if !strings.Contains(releaseJob, `gh release view "$TAG"`) {
		t.Error("the release job does not check whether a release already exists: `gh release create` fails " +
			"outright on a second run, so any retry would need a human to delete the draft first")
	}
	if !strings.Contains(releaseJob, "TAG: ${{ github.ref_name }}") {
		t.Error("the release job does not carry the tag through the environment")
	}
	if strings.Contains(releaseJob, `create "${{ github.ref_name }}"`) {
		t.Error("the release job expands the tag directly into a shell script: a tag name is not restricted " +
			"from shell metacharacters, which is the standard Actions injection")
	}
}
