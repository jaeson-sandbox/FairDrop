---
title: 'Story 6.2: Pin What Ships, Not the Template'
type: 'feature'
created: '2026-09-19'
status: 'done'
baseline_commit: '611cad441173b94dcba2f11ade94c2d121f870db'
review_loop_iteration: 0
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Every automated assertion Epic 6 added reads the *input* assets under `build/`, never
the *product* under `build/bin/`. If the resource-embedding path regressed — `compileResources`'
`.syso` step, or an `.ico` the resource compiler cannot parse — the exe would fall back to the
shell's default icon and all eight tests would still pass. The same layer is already unverified
for the sibling version-string resources: `release_identity_test.go` pins ProductName,
ProductVersion and CompanyName **as they appear in `info.json`'s template** and passes, while
nothing opens a built binary. Story 6.1's manual pass is the only thing that ever looked, and it
reached a wrong conclusion once (D-128) by reading the exe with a tool that cannot see a
language-neutral string table.

**Approach:** Walk the built exe's PE resource directory with the standard library and assert what
actually ships: `RT_ICON` carries exactly the committed size set with payloads matching
`build/windows/icon.ico`, and `RT_VERSION`'s string table carries the release identity. Run it on
the Windows runner after `wails build`, and extend the post-build drift check to `build/` so an
`.ico` missing from git cannot be manufactured by the runner and pass unnoticed.

**Closes:** D-128, D-129, D-130.

## Boundaries & Constraints

**Always:** Read the built artifact, not the template. Standard library only — the resource walk is
plain struct parsing over `debug/pe`'s section table. Env-gate the test exactly as
`TestDarwinBuiltAppSurvivesUnusableLock` does, so a local `go test ./...` with no build present
skips rather than fails. Reuse `appicon_test.go`'s existing size set and pixel helpers.

**Ask First:** Any change to `build/windows/info.json` beyond adding the missing `FileVersion`
string and moving the string table off the language-neutral key.

**Never:** Re-verify what Story 6.1 already pins about the committed assets — this story pins the
exe. Touch the artwork, the derivation, or `scripts/build-appicon.py`. Add an image pipeline to
`verify.yml`; a test step and a drift-check line are not that.

**Deliberately out of scope, stated rather than silently absent:** the macOS product icon. Wails
derives `iconfile.icns` from `appicon.png` on every build, and nothing is committed under
`build/darwin/` except the two plists, so the `.app`'s icon is unverified at the product layer
exactly as the exe's was. This story covers Windows because that is where the stale-`.ico` defect
actually lives; the macOS half is a separate deferred entry, not an unstated omission.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Windows CI after `wails build` | exe present, resources intact | `RT_ICON` = {256,128,64,48,32,16}, payloads match `icon.ico` | — |
| Resource embedding regresses | `.syso` step fails or `.ico` unparseable | exe ships the shell default icon | new test fails, naming the exe |
| `icon.ico` missing from git | runner regenerates it during `wails build` | the path shows as **untracked** | the drift clause uses `git status --porcelain`, not `git diff`, because diff never reports untracked paths |
| Local run, no build | `build/bin/fairdrop.exe` absent | test skips | never a false failure |
| Version strings read by .NET | language-neutral table | PowerShell reads blank (D-128) | test reads the resource block directly and pins the real values |

</frozen-after-approval>

## Superseded by Story 6.3

The approved Story 6.3 reconciles two historical statements above without rewriting
the frozen record. The built-resource tests intentionally gate on artifact absence:
they skip on a source-only local run, while the Windows CI step refuses any skip after
`wails build`. The shipped version string table is now filed under language key `0409`,
so its embedded key is `040904b0`; PowerShell and .NET read ProductVersion and the added
FileVersion from that table. Story 6.3 supplies the missing neutral-key rebuild proof.

## Code Map

**The resource walk is verified, not assumed** — prototyped against the current
`build/bin/fairdrop.exe` before this spec was written:

- PE signature offset at `0x3C`; section table follows the optional header
  (`peOffset + 24 + SizeOfOptionalHeader`). `.rsrc` sits at vaddr `0x02d68000`, raw `0x00d0a600`.
- The resource directory is three levels — type, then name/id, then language. At each level the
  16-byte header's counts are at `+12` (named) and `+14` (id); entries are 8 bytes, high bit of the
  second word meaning "child is a directory".
- A leaf's 8-byte data entry holds an **RVA**, not a section offset: subtract the section's vaddr.
- Measured on the current exe: `RT_ICON` (type 3) holds **six PNG payloads** decoding to 256, 128,
  64, 48, 32 and 16 RGBA — the same set `icon.ico` carries. `RT_GROUP_ICON` (14) holds one,
  `RT_VERSION` (16) holds one. All at language id 0.
- `debug/pe` supplies the section table without hand-parsing the headers; the directory walk is
  still manual, as no stdlib package exposes it.

Sites the change touches:

- `single_instance_darwin_test.go:155` -- the env-gating precedent: `FAIRDROP_NATIVE_APP_SMOKE=1`
  **and** `GITHUB_ACTIONS=true`, else `t.Skip`.
- `.github/workflows/verify.yml:124` (`wails build`), `:126` (the macOS smoke step this one sits
  beside, gated `if: runner.os == 'macOS'`), `:137` (the bindings/`.gitkeep` drift check to extend).
- `verify_workflow_test.go` pins the workflow in **three** places that all need the new step:
  `:248`'s native-proof table (job, step name, run line, `if:` guard), `:299-305`'s drift-check
  content pin, and `:529`'s required step-order list. A step added without updating all three
  fails the gate, which is the point of those pins.
- `build/windows/info.json` -- the `"0000"` language block and the absent `FileVersion` string.
  D-128 records what is and is not known: the values themselves are correct and Win32 reads them
  fine, so this is interoperability, not a defect.
- `appicon_test.go` -- `wantIcoSizes`, `decodeICOEntryImage`, `assertCentralSaturationExceeds` and
  `meanChannelDistance255` are reused. Do not duplicate them.

## Tasks & Acceptance

**Execution:**
- [x] `exe_resources_windows_test.go` -- new, `//go:build windows`: walk the exe's resource
  directory and pin `RT_ICON` and `RT_VERSION` against the committed assets.
- [x] `.github/workflows/verify.yml` -- add the Windows-gated step after `wails build`, and extend
  the post-build drift check from `frontend/` to also cover `build/windows/icon.ico`.
- [x] `verify_workflow_test.go` -- update all three pins named in the Code Map.
- [x] `build/windows/info.json` -- add the `FileVersion` string and move the table off the
  language-neutral key, so .NET-based tooling can read what Win32 already reads.
- [x] `_bmad-output/planning-artifacts/epics.md` -- add Story 6.2 under Epic 6 with a
  `**Closes:** D-128, D-129, D-130` line, which `TestEveryOpenDeferredEntryIsCitedByItsOwningStory`
  requires.
- [x] `_bmad-output/implementation-artifacts/sprint-status.yaml` -- register the story.
- [x] `_bmad-output/implementation-artifacts/deferred-work.md` -- set D-128, D-129 and D-130 to
  `discharged`.
- [x] `_bmad-output/implementation-artifacts/evidence-6-2-pin-what-ships-not-the-template.md` --
  the mutation table.

**Acceptance Criteria:**

Each criterion names the mutation that must fail it, measured against a real build.

- **The exe's icon is the committed one.** Given a built exe, when its `RT_ICON` payloads are
  decoded, then their sizes equal {256,128,64,48,32,16} and each matches the same-sized
  `icon.ico` entry within the freshness tolerance Story 6.1 calibrated. *Mutations:* build with a
  stale `.ico`; strip an `RT_ICON` entry **-> both must fail, naming the exe.**
- **The exe's identity is the committed one.** Given a built exe, when `RT_VERSION`'s string table
  is read, then ProductName, CompanyName, ProductVersion and FileVersion match `wails.json`.
  *Mutation:* change `wails.json`'s productVersion without rebuilding **-> must fail.** This is the
  layer `release_identity_test.go` pins only in template form.
- **Absent build skips, never passes.** Given no `build/bin/fairdrop.exe`, when the suite runs,
  then the test skips. *Mutation:* make it `t.Fatal` on a missing exe and confirm a bare local
  `go test ./...` then fails -- the skip must be a skip, not a silent pass.
- **A missing committed `.ico` cannot be manufactured by CI.** Given `icon.ico` deleted from git,
  when the Windows job runs, then the drift check fails naming `build/`. *Mutation:* delete it and
  confirm the job fails at the drift step rather than passing on a runner-generated file.
- Given the full gate on all three platforms, when it runs, then it passes.

## Spec Change Log

## Verification

**Commands:**
- `wails build` then `FAIRDROP_NATIVE_APP_SMOKE=1 GITHUB_ACTIONS=true go test -count=1 -run TestExeResources -v .`
- `go test -count=1 -run TestVerifyWorkflow -v .` -- the three pins must accept the new step.
- `gofmt -l . && go vet ./... && go tool staticcheck ./...`
- `go test -count=1 ./... && go test -count=1 -race ./...`
- `CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go vet ./...`, and the same for `linux/amd64` -- **vet,
  not build**: `go build` never compiles `_test.go` files on any GOOS, so a cross `go build` would
  prove nothing about a `_test.go` file that is the whole of this story.
