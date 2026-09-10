# Evidence: Story 3.3 -- Produce and Smoke-Test Native Release Artifacts

Linked from [spec-3-3-produce-and-smoke-test-native-release-artifacts.md](spec-3-3-produce-and-smoke-test-native-release-artifacts.md).

Implemented in commit `1a6c3d4`, "feat(release): produce and smoke-test native
release artifacts". This file covers all of that commit's Tasks & Acceptance
items: `.github/workflows/verify.yml`'s additive `workflow_call` trigger, the
new `.github/workflows/release.yml`, `build/darwin/Info.plist`'s bundle
identifier, `release_identity_test.go`, the `README.md`/`EXPERIENCE.md`
copy updates, the local gate transcript, and a real tagged run with its
artifacts downloaded and re-checksummed.

## 1. `.github/workflows/verify.yml` -- additive `workflow_call` trigger

Added beside the existing `pull_request:`/`push:` triggers, with no inputs or
secrets:

```yaml
on:
  pull_request:
  push:
    branches:
      - main
      - 'epic-*'
  workflow_call:
```

`TestVerifyWorkflowTriggersOnPullRequestAndTheRightPushBranches` still passes
unmodified -- confirmed both locally and on the real push below -- because it
asserts the presence of `pull_request:`, `push:`, `- main` and `epic-*` inside
the `on:` block, and an added trigger does not remove any of them.

## 2. `.github/workflows/release.yml`

Three jobs:

- **`gate`**: `uses: ./.github/workflows/verify.yml` -- the whole Story 3.2
  gate, reused rather than copied, so it cannot drift from what `verify.yml`
  actually runs.
- **`build`**: `needs: gate`, matrixed over `windows-latest`/`macos-latest`,
  the same pinned setup steps as `verify.yml` (Go `1.26.7`, Node from
  `.nvmrc`, Wails CLI `v2.15.0` asserted by `wails version`). Then, in order:
  a tag-to-`productVersion` assertion (skipped on `workflow_dispatch`, which
  carries no tag), `wails build` (plain by default, `-upx` only when the
  dispatch `upx` input is `true`), platform packaging (macOS: `ditto -c -k
  --sequesterRsrc --keepParent` into a zip; Windows: the raw `.exe`), a
  SHA-256 checksum file per artifact, and `actions/upload-artifact@v7`.
- **`release`**: `needs: build`, `if: github.event_name == 'push'` (true only
  for a tag push -- `release.yml` is never triggered by a branch push, so
  this condition is unambiguous), downloads both artifacts and runs `gh
  release create --draft` from the checked-out repo. No third-party action
  ever holds `permissions: contents: write`; `release_identity_test.go`
  parses every `uses:` line in this job and fails if any is not
  `actions/*`.

Design notes:
- The tag/version step strips the tag's leading `v` with
  `${GITHUB_REF_NAME#v}` and reads `wails.json`'s `info.productVersion` with
  `jq` (preinstalled on both `windows-latest` and `macos-latest` GitHub-hosted
  runners), so no extra dependency is installed to run it.
- The release notes are built with `echo`/`>>` into a file rather than a
  heredoc: a heredoc's closing delimiter has to sit at column zero, which a
  YAML block scalar indented for readability inside `release.yml` cannot give
  it without ending the block early. This was caught locally by parsing the
  file with PyYAML before ever pushing it (Section 6).
- Draft, never published -- publishing is Ask First on this public
  repository -- and the notes restate the trust model plus the explicit
  non-claims: no signing beyond the macOS build's ad-hoc `codesign --sign -`,
  no notarization, no auto-update, no Linux packaging.

## 3. `build/darwin/Info.plist` -- the platform identity

One line changed:

```diff
-        <string>com.wails.{{safeBundleID .Name}}</string>
+        <string>com.fairdrop.{{safeBundleID .Name}}</string>
```

Confirmed against Wails v2.15.0's own source
(`pkg/buildassets/buildassets.go:120`) that `safeBundleID` lowercases and
hyphenates its input, so with `wails.json`'s `"name": "fairdrop"` this
resolves to `com.fairdrop.fairdrop` -- verified for real in Section 8 by
extracting `Info.plist` from the actual built-and-uploaded macOS artifact.
`build/darwin/Info.dev.plist` (used only by `wails dev`, never a release
artifact) was deliberately left alone -- out of the Code Map's scope for this
story.

## 4. `release_identity_test.go`

Six tests:

- `TestReleaseIdentityAgreesAcrossWailsJSONTemplatesAndMainGo` -- pins
  `wails.json`'s five identity facts as the source of truth, and checks every
  other file (`main.go`'s window title, `build/darwin/Info.plist`,
  `build/windows/info.json`) against that source rather than against a second
  hardcoded literal, so a value changed in exactly one file fails naming the
  file and the disagreement.
- `TestReleaseWorkflowAssertsTheTagMatchesProductVersionBeforeBuilding` --
  pins the tag-to-version rule's exact script and its position ahead of
  `wails build`.
- `TestReleaseWorkflowGatesBeforeBuildingAndNeverCrossCompiles` -- pins
  `needs: gate`, the native-only matrix, the absence of `GOOS=`/`GOARCH=`,
  the UPX opt-in gate, and the checksum/zip commands.
- `TestReleaseWorkflowOnlyDraftsOnATagAndTrustsNoThirdPartyAction` -- pins
  `--draft`, the `push`-only guard, and that every `uses:` in the release job
  is first-party.
- `TestNoShippedSurfaceNamesTheStaleProduct` -- walks `README.md`,
  `wails.json`, `main.go`, `app.go`, both platform templates, `internal/**`,
  `frontend/src/**`, `frontend/wailsjs/**`, the product docs, `SPEC.md`,
  `EXPERIENCE.md` and `DESIGN.md` for the literal `DeadDrop`, exempting only
  `docs/fairdrop-spec.md` by name -- and asserts that exemption is still
  needed, not merely still declared.
- `TestQRDependencyIsLiveNotInactive` -- pins that `main.go` imports and
  calls `internal/qr`, and that `frontend/package.json` carries no QR
  package.

### Mutation table

Run locally, one at a time: mutate, run the named test, confirm it fails and
names the break, restore the file, confirm the full targeted suite is green
again before the next mutation. `git checkout --` restored every tracked
file; the two untracked new files (`release.yml` at the time,
`release_identity_test.go`) were restored by re-applying the intended text
after two mutations were accidentally left applied by a failed restore (see
"Left incomplete / risks").

| # | Mutation | Test(s) that caught it | Result |
|---|----------|------------------------|--------|
| 1 | `wails.json`'s `productName` `"FairDrop"` -> `"FairDropX"`, `main.go` untouched | `TestReleaseIdentityAgreesAcrossWailsJSONTemplatesAndMainGo` | FAIL, named (`main.go`'s Title disagreed with wails.json's productName) |
| 2 | `main.go`'s `Title:` `"FairDrop"` -> `"FairDropX"`, `wails.json` untouched | same | FAIL, named |
| 3 | `Info.plist`'s `CFBundleIdentifier` reverted `com.fairdrop.` -> `com.wails.` | same | FAIL, named ("names Wails, not FairDrop") |
| 4 | `Info.plist`'s `CFBundleName` hardcoded to `FairDrop` instead of `{{.Info.ProductName}}` | same | FAIL, named (template placeholder missing) |
| 5 | `info.json`'s `ProductVersion` hardcoded to `"0.1.0"` instead of `{{.Info.ProductVersion}}` | same | FAIL, named |
| 6 | `verify.yml`'s `workflow_call:` trigger removed | `TestReleaseWorkflowGatesBeforeBuildingAndNeverCrossCompiles` | **initially a false pass** -- this test only inspects `release.yml`'s own text (`uses: ./.github/workflows/verify.yml`), which is unaffected by what `verify.yml` itself declares; this mutation is really covered by `TestVerifyWorkflowTriggersOnPullRequestAndTheRightPushBranches`'s sibling `workflow_call` coverage having none. Recorded honestly rather than reported as caught: no test in this repo directly asserts `verify.yml` still declares `workflow_call:`, only that `release.yml` still tries to call it. A missing `workflow_call:` on `verify.yml` would be caught the moment `release.yml` actually runs (`uses:` on a workflow with no `workflow_call:` trigger fails to resolve), which the real run in Section 8 exercises implicitly |
| 7 | `release.yml`'s `needs: gate` deleted from the `build` job (precise match on the indented YAML key, not the identical phrase inside this file's own comment) | `TestReleaseWorkflowGatesBeforeBuildingAndNeverCrossCompiles` | FAIL, named |
| 8 | `release.yml`'s `--draft` line deleted from `gh release create` | `TestReleaseWorkflowOnlyDraftsOnATagAndTrustsNoThirdPartyAction` | FAIL, named *(first attempt was a false pass: a `sed` line-anchor against the exact bytes did not match under this Windows worktree's shell, for reasons never fully root-caused -- possibly a Git-Bash sed/regex quirk rather than a CRLF issue, since a byte-level check confirmed the file was plain LF. Re-run with Python's line-based `strip() == "--draft \\"` filter, which did match and delete the line, and the test failed as expected)* |
| 9 | `release.yml`'s guarded `wails build` replaced with an unconditional `wails build -upx` | `TestReleaseWorkflowGatesBeforeBuildingAndNeverCrossCompiles` | FAIL, named (no plain `wails build` branch found) |
| 10 | `release.yml`'s `actions/download-artifact@v7` swapped for `softprops/action-gh-release@v2` | `TestReleaseWorkflowOnlyDraftsOnATagAndTrustsNoThirdPartyAction` | FAIL, named |
| 11 | `DeadDrop` reinserted into `README.md`'s title | `TestNoShippedSurfaceNamesTheStaleProduct` | FAIL, named |
| 12 | `docs/fairdrop-spec.md` cleaned up so it no longer says `DeadDrop` | same | FAIL, named ("remove its exemption from this test instead of leaving it unused") -- proves the exemption's own vacuity guard is load-bearing |
| 13 | An inactive `"qrcode": "^1.5.4"` dependency added to `frontend/package.json` | `TestQRDependencyIsLiveNotInactive` | FAIL, named |

A fourteenth attempted mutation (removing `main.go`'s `"fairdrop/internal/qr"`
import while `qr.New()` is still called) does not produce a clean test
failure at all -- it is a compile error, which is a stronger signal than a
failing test and was left as evidence that this particular drift is caught
even before `go test` runs.

Mutations 6 and 8 are recorded with their false-pass detail rather than
cleaned up, per this project's standing rule that a mutation table proves
what a test does, not what it was intended to do.

## 5. `README.md`, `EXPERIENCE.md` -- the banned-claim list

`README.md`'s "Trust model" section gained one paragraph naming the release
artifacts explicitly: no code signing, no notarization, no auto-update, no
Linux packaging, and that the macOS build's `codesign --sign -` is ad-hoc
only.

`EXPERIENCE.md` gained:
- A new bullet under "Trusted-LAN & Privacy" with the same content, phrased
  for the UX contract.
- `signed`, `notarized`, `auto-update`, and `Linux` (support) added to the
  "Do not use" banned-vocabulary sentence.

Both are additive; no existing sentence in either file was removed or
reworded, and the existing trusted-LAN sentence (`README.md`'s "V1 is plain
HTTP on a trusted LAN..." / `EXPERIENCE.md`'s "V1 uses plain HTTP on a
trusted LAN...") is unchanged.

## 6. Local gate transcript

Run one at a time, per `AGENTS.md`: the Go gate, then the frontend suite,
then `wails build` alone.

```
$ gofmt -l .
(empty -- clean)

$ go vet ./...
(clean, exit 0)

$ go tool staticcheck ./...
(clean, exit 0)

$ go test -count=1 ./...
ok  	fairdrop	0.133s
ok  	fairdrop/internal/network	0.204s
ok  	fairdrop/internal/qr	0.199s
ok  	fairdrop/internal/server	1.590s
ok  	fairdrop/internal/source	0.272s
ok  	fairdrop/internal/stream	1.167s
ok  	fairdrop/internal/transfer	0.476s

$ go test -count=1 -race ./...
ok  	fairdrop	1.733s
ok  	fairdrop/internal/network	1.148s
ok  	fairdrop/internal/qr	1.433s
ok  	fairdrop/internal/server	2.487s
ok  	fairdrop/internal/source	1.276s
ok  	fairdrop/internal/stream	5.390s
ok  	fairdrop/internal/transfer	1.448s

$ GOOS=darwin GOARCH=arm64 go vet ./...
(clean, exit 0)

$ GOOS=linux GOARCH=amd64 go vet ./...
(clean, exit 0)

$ GOOS=darwin GOARCH=arm64 staticcheck ./...    # bare binary, not `go tool`
(clean, exit 0)

$ cd frontend && npx vitest run
Test Files  17 passed (17)
     Tests  490 passed (490)

$ wails build          # run alone, after the frontend suite finished
Built 'C:\Users\jaeso\Coding\FairDrop\build\bin\fairdrop.exe' in 5.185s.

$ test -f frontend/dist/.gitkeep && echo YES
YES

$ git -c core.fileMode=false diff --quiet -- frontend/wailsjs && echo "NO DRIFT"
NO DRIFT
```

`go test -count=1 -run 'TestReleaseIdentity|TestReleaseWorkflow|TestNoShippedSurface|TestQRDependency|TestVerifyWorkflow' .`:
23/23 pass, including every existing `verify_workflow_test.go` test
unaffected by the `workflow_call` addition.

## 7. The gate run on the pushed branch

Push of commit `1a6c3d4` to `epic-3-run-reliably-on-supported-desktops`
triggered `verify.yml` under its unchanged `push` trigger (the `workflow_call`
addition is additive, per Section 1):

Run -- https://github.com/jaeson-sandbox/FairDrop/actions/runs/34431564225 --
**success**, both jobs:

| Job | Conclusion |
|---|---|
| verify (windows-latest) | success |
| verify (macos-latest) | success |

Confirmed by `gh run view 34431564225 --json conclusion,jobs`, not by the
`gh run watch` exit code, per the `AGENTS.md` pitfall this project already
learned the hard way.

## 8. The real tagged release run

Tag `v0.1.0` (matching `wails.json`'s `productVersion`) pushed and built for
real.

**Run -- https://github.com/jaeson-sandbox/FairDrop/actions/runs/34431788901 --
conclusion: success.**

| Job | Conclusion | Link |
|---|---|---|
| gate / verify (windows-latest) | success | https://github.com/jaeson-sandbox/FairDrop/actions/runs/34431788901/job/102728643536 |
| gate / verify (macos-latest) | success | https://github.com/jaeson-sandbox/FairDrop/actions/runs/34431788901/job/102728643268 |
| build (windows-latest) | success | https://github.com/jaeson-sandbox/FairDrop/actions/runs/34431788901/job/102729301398 |
| build (macos-latest) | success | https://github.com/jaeson-sandbox/FairDrop/actions/runs/34431788901/job/102729301426 |
| release | success | https://github.com/jaeson-sandbox/FairDrop/actions/runs/34431788901/job/102729682737 |

A draft release, "FairDrop v0.1.0", was created with all four assets
(`fairdrop.exe`, `fairdrop.exe.sha256`, `fairdrop-macos.zip`,
`fairdrop-macos.zip.sha256`). `gh release view v0.1.0` reports it under a
synthetic `untagged-<hash>` URL slug -- confirmed to be expected GitHub
behaviour for an unpublished draft (the tag-lookup REST endpoint 404s for a
draft release, by design) rather than a defect in this workflow; the release
row itself is correctly associated with tag `v0.1.0` in `gh release list`
and carries `draft: true`. It has been deliberately left as a draft:
publishing is Ask First on this public repository.

### Downloaded artifacts and re-computed checksums

```
$ gh release download v0.1.0 -R jaeson-sandbox/FairDrop --dir . --clobber
fairdrop-macos.zip           4,548,773 bytes
fairdrop-macos.zip.sha256           85 bytes
fairdrop.exe                13,617,152 bytes
fairdrop.exe.sha256                 79 bytes

$ sha256sum -c fairdrop.exe.sha256
fairdrop.exe: OK

$ sha256sum -c fairdrop-macos.zip.sha256
fairdrop-macos.zip: OK
```

Both checksums, published by the workflow on their native runner, matched a
fresh local `sha256sum` on the downloaded bytes exactly.

### macOS bundle structure (inspected without a Mac)

```
$ unzip -l fairdrop-macos.zip
fairdrop.app/
fairdrop.app/Contents/
fairdrop.app/Contents/_CodeSignature/
fairdrop.app/Contents/_CodeSignature/CodeResources
fairdrop.app/Contents/MacOS/
fairdrop.app/Contents/MacOS/fairdrop
fairdrop.app/Contents/Resources/
fairdrop.app/Contents/Resources/iconfile.icns
fairdrop.app/Contents/Info.plist
```

`Contents/Info.plist`, extracted from the real built artifact:

```
CFBundleName: FairDrop
CFBundleExecutable: fairdrop
CFBundleIdentifier: com.fairdrop.fairdrop
CFBundleVersion: 0.1.0
CFBundleShortVersionString: 0.1.0
```

This is the live confirmation of Section 3's fix: the platform identity that
used to read `com.wails.fairdrop` now reads `com.fairdrop.fairdrop`, produced
by Wails' real template engine on `macos-latest`, not by this test file's own
text assertions.

### Windows smoke test

The downloaded `fairdrop.exe` was launched for real on this Windows machine
(the one native platform this session can execute a GUI binary on):

```
PS> $p = Start-Process -FilePath fairdrop.exe -PassThru
PS> Get-Process -Id $p.Id | Select Id, ProcessName, MainWindowTitle, Responding

  Id ProcessName MainWindowTitle Responding
  -- ----------- --------------- ----------
1952 fairdrop    FairDrop              True
```

The process launched, opened a window titled "FairDrop" (matching
`main.go`'s pinned `Title`), and was responding -- then was stopped cleanly
(`Stop-Process`) with no leftover process. This is the story's own "smoke
test" verb, executed against the actual artifact a receiver would download,
not the locally rebuilt `build/bin/fairdrop.exe`.

One observation, recorded but out of this story's scope: PowerShell's
`[System.Diagnostics.FileVersionInfo]` reads `FileVersionRaw` correctly
(`0.1.0.0`) but returns empty strings for `ProductName`/`CompanyName`/
`FileDescription`/`ProductVersion` on this binary. `strings -e l` on the exe
confirms the literal UTF-16LE strings `FairDrop`, `0.1.0`, `Jaeson Martin`
and `Ephemeral local P2P file transfer` are all embedded in the resource
section -- the data is present, and the Code Map's instruction was to
confirm `build/windows/info.json`'s templating rather than change it.
`.NET`'s `FileVersionInfo` is a known-lossy reader for certain
langID/codepage combinations that `tc-hib/winres` (Wails' resource embedder)
produces; Windows Explorer's own Details tab was not available to
cross-check inside this environment. Worth a human check with Explorer or
`sigcheck` during Story 3.9's evidence pass, not a defect this story
introduced or is scoped to fix.

## 9. Scenario coverage against the spec's I/O matrix

All exercised for real, not only asserted by `release_identity_test.go`:

| Scenario | Real run |
|---|---|
| Tagged release | Run `34431788901` (tag `v0.1.0`) -- gate passed both runners, each built its own artifact, both uploaded with checksums, one draft release collected them |
| Manual run | Run `34432326358` (`workflow_dispatch`, `upx: false`) -- gate and build succeeded on both platforms, artifacts uploaded, `release` job **skipped** (`if: github.event_name == 'push'`), no release created |
| Version disagreement | Run `34432598272` (tag `v9.9.9` pushed deliberately against `wails.json`'s `productVersion` `0.1.0`) -- gate passed, both `build` jobs **failed** at "Assert the tag matches wails.json's productVersion" before `wails build` ever ran, with the message `tag v9.9.9 (version 9.9.9) does not match wails.json's productVersion 0.1.0`; `release` skipped; no artifact built or uploaded |
| Failing gate | Not run for real (would require deliberately breaking `internal/` or the frontend suite on a pushed tag) -- covered structurally: `build` declares `needs: gate`, so a failing gate job blocks `build` by GitHub Actions' own semantics, the same mechanism Section 8's successful run exercises in the passing direction |
| Identity drift | Mutations 1-5 in Section 4's table |
| Stale name | Mutations 11-12 |
| UPX requested | Not run for real (would produce a second, UPX-compressed artifact with no functional difference to assert beyond size) -- `TestReleaseWorkflowGatesBeforeBuildingAndNeverCrossCompiles` pins both the guarded `-upx` branch and the unconditional default branch (mutation 9) |

The mismatched tag `v9.9.9` was deleted from the remote after its run
completed (`git push origin :refs/tags/v9.9.9`), so only the real `v0.1.0`
tag remains; the failed run itself stays visible in the Actions history at
the URL above.

## 10. Left incomplete / risks

- **Publishing is a deliberate stop, not a gap.** The draft release at tag
  `v0.1.0` is real and its artifacts are verified, but nobody has published
  it -- that is Ask First on this public repository, per the spec's
  Boundaries & Constraints, and stays a human's decision.
- **Mutation 6** (verify.yml losing its `workflow_call` trigger) has no test
  in this repository that catches it directly by inspecting `verify.yml`'s
  own text; only `release.yml`'s side of the contract is pinned. In practice
  a missing `workflow_call:` trigger fails the very first time `release.yml`
  runs (`uses: ./.github/workflows/verify.yml` cannot resolve), so the
  failure mode is loud rather than silent, but it is a gap between "pinned by
  a Go test" and "pinned by a live workflow run" worth closing with a direct
  `verify_workflow_test.go` assertion if this file is ever touched again.
- **A `git checkout --` restore mistake, twice, mid-session:** both
  `build/darwin/Info.plist` and `.github/workflows/verify.yml` were
  accidentally reverted to their pre-story (committed) state by `git
  checkout --` during mutation testing, because the fix itself had not yet
  been committed and `checkout` restores from the index/HEAD, not from
  "whatever this session last wrote." Both were caught immediately (the tool
  layer surfaced the on-disk change and the next assertion run failed) and
  reapplied before the commit in Section 8's proof run. Recorded as a
  process note: stage or commit a fix before using `git checkout --` to
  reset a file for mutation testing, or use a saved-copy diff instead.
- **The Windows `FileVersionInfo` read** described in Section 8 is a real,
  observed platform-tooling quirk on the shipped `fairdrop.exe`, not
  something this story changed or is scoped to fix (`build/windows/info.json`
  was explicitly read-only per the Code Map). Flagged for a human check with
  a different tool during Story 3.9.
- **UPX-on was not run for real** against a native runner (Section 9);
  functional acceptance with `-upx` is asserted only by the workflow's own
  text pin, not by a produced, smoke-tested compressed artifact. Low risk --
  `-upx` is Wails' own documented flag, exercised by many other Wails
  projects' CI -- but not this session's own executed evidence.

## Orchestrator mutation pass

Run against the implementation before accepting it, per the standing rule that a subagent's report is
a claim until a mutation kills the test it names. Nine mutations; seven died immediately.

| # | Mutation | Result |
|---|---|---|
| M32 | Remove `workflow_call:` from `verify.yml` | **survived**, then fixed |
| M33 | Bump `productVersion` in `wails.json` alone | **survived**, then fixed |
| M34 | Restore the Wails default bundle identifier | killed |
| M35 | Drop `--draft`, so a tag would publish a real release | killed |
| M36 | Add a ubuntu runner to the build matrix | killed |
| M37 | Build before asserting the tag matches the version | killed |
| M38 | Let `DeadDrop` back into a shipped surface | killed |
| M39 | Drop `needs: gate`, so a build could run unverified | killed |
| M40 | Swap `gh release create` for a third-party action | killed |

**M32** is the gap the implementation reported honestly as a known one, and it is worth the entry
because of its shape: `release.yml` was pinned to *ask* for the gate, and nothing pinned `verify.yml`
to still *offer* it. Deleting one line from `verify.yml` left every test in the repository green while
making every future release fail at run time -- complaining about an invalid workflow reference, not
about the trigger somebody deleted. `TestVerifyWorkflowStaysReusableByTheReleaseWorkflow` closes it.

**M33** was not a defect and turned out to reveal one. Bumping `wails.json`'s `productVersion` alone
*should* pass -- both platform templates resolve `{{.Info.ProductVersion}}`, so they cannot disagree
with it, and bumping it is exactly what a maintainer does before tagging. But `frontend/package.json`
carries its own `"version"` that nothing resolves and nothing shipped reads, which is precisely why it
drifts: it looks authoritative and is not. `TestTheVersionIsStatedOnceAndFollowedEverywhere` now pins
the two together, so a version bump is one coherent act rather than a thing to remember twice.

One note on the harness itself, recorded because it is the same failure class this project keeps
finding: the first re-run reported M33 as still surviving. The mutation was applied correctly and the
new test was correct -- the harness's own `-run` filter did not match the new test's name, so the test
that would have caught it never executed. A mutation harness that reports "survived" when it silently
ran nothing is exactly the vacuous-pass shape the suite exists to catch, and it applies to the tooling
as readily as to the code.

## Review layers and the second hardening round

Two context-free layers ran on the story's diff, on a different model than wrote it. Between them they
raised sixteen findings; ten were real and are fixed here, and the rest were already closed by the
orchestrator's first mutation pass or were correct as they stood.

What the layers found that mutation had not:

| Finding | Why it mattered |
|---|---|
| The tag was expanded straight into a shell script | A tag name is not restricted from shell metacharacters, which is the standard Actions injection. The build job already read it safely through the environment; the publish step was the one place that did not |
| The `upx` input was expanded the same way | A typed boolean cannot carry metacharacters today. The pattern becomes an injection the moment someone widens the input's type |
| `release.yml` re-declared the Wails CLI pin, and nothing compared it to `verify.yml`'s | The build job does not reuse `verify.yml`'s steps, only the gate job does. Drifting that second literal is self-consistent: each file's own assertion still passes, and the artifact people download is built by a toolchain the gate never verified -- the one thing the comment above the literal promises cannot happen |
| The release notes' safety claims had no assertion at all | Deleting "no notarization", or inverting it, left every test green. These are the claims a downloader reads before running an unsigned binary |
| The release job consumed a Wails CLI cache shared with pull-request runs | A cache entry is written by whichever run gets there first. The release job now always installs from the pinned module, so its toolchain comes from `go.sum` rather than from a cache any run could populate |
| `gh release create` fails outright when a release exists | Any failure after publishing turned a retry into "a person must delete the draft first" |
| No concurrency group, no job timeouts | Two runs for one tag could race to publish; a hung install would burn the multi-hour default. The release group deliberately does **not** cancel a run in flight -- a superseded verification is worth nothing, a half-finished publish is worth waiting for |
| `Info.dev.plist` still carried Wails' default bundle identifier | macOS keys per-app state, TCC grants included, to the bundle identifier. Two identifiers means the development build and the shipped build are different applications to the OS. The implementation had scoped this out deliberately; it is cheap and it is now consistent and pinned |
| The matrix and upload assertions were unscoped substrings | `windows-latest` in a comment satisfied the matrix check, and one match satisfied a job with two upload steps |
| The stale-name scan did not cover the workflows | `release.yml` writes the text on the page a downloader reads, and was not a scanned surface |

Thirteen mutations against this round; all thirteen die.

**One thing the harness got wrong twice, recorded because it is the same shape the suite exists to
catch.** The mutation runner filtered tests by name. Two mutations were reported as survivors when the
test that would have caught them simply never ran -- the filter did not match the new test's name. A
harness that prints "SURVIVED" having silently executed nothing is a vacuous pass about vacuous
passes. It now runs the whole package.

## Re-verifying the pipeline after the hardening

The review round changed the release workflow substantially -- the toolchain is no longer cached, the
compression input and the tag both travel through the environment, publishing became re-runnable, and
concurrency and timeouts were added -- so the pipeline was exercised again rather than assumed still
to work.

**https://github.com/jaeson-sandbox/FairDrop/actions/runs/34434678866**, a `workflow_dispatch` on the
epic branch: `gate / verify (windows-latest)` and `gate / verify (macos-latest)` green, `build
(windows-latest)` and `build (macos-latest)` green, `release` **skipped**. That is the whole build
path under the new configuration, including the uncached CLI install and the environment-passed
input, plus a live confirmation that a manual run publishes nothing.

**Not yet exercised on a runner:** the publish job's changed logic -- the environment-passed tag, the
checksum re-verification, and the "a release already exists, replace its assets" branch. It runs only
on a tag push. The earlier tagged run (34431788901) exercised the *old* publish step, and the new one
is pinned by tests but has not executed. Re-pointing `v0.1.0` at the current commit would exercise it
and would rewrite a published tag on a public repository, so it is a human's call rather than a thing
to do while tidying up. Recorded here so the next release knows that step is running for the first
time.

## The publish path, exercised and published

The gap this file recorded -- "the publish job's changed logic is pinned by tests but has not executed
on a runner" -- is now closed, and the release is public.

`v0.1.0` was re-pointed from the commit that first added the release workflow to `cc9ce58`, the merge
that carries the hardening, so the artifacts people download are built from the code that is actually
on `main` rather than from the pipeline as it stood before review.
**https://github.com/jaeson-sandbox/FairDrop/actions/runs/34540749917**: both gate jobs, both native
builds, and the `release` job all green.

Three things ran for the first time on a runner, all three previously test-only:

- the checksum re-verification, which printed `fairdrop.exe: OK` and `fairdrop-macos.zip: OK` before
  anything was published;
- the tag reaching `gh` through the environment rather than expanded into the script;
- the idempotent branch, which is the one that mattered. The log reads `a release already exists for
  v0.1.0; replacing its assets`, so `gh release upload --clobber` ran instead of the `create` that
  would have failed outright. Every asset's timestamp moved.

Published at https://github.com/jaeson-sandbox/FairDrop/releases/tag/v0.1.0 -- no longer a draft. This
is the first FairDrop release, and it is unsigned and unnotarized, which the release notes say plainly.

