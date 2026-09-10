---
title: 'Story 3.3: Produce and Smoke-Test Native Release Artifacts'
type: 'feature'
created: '2026-09-09'
status: 'in-review'
review_loop_iteration: 0
baseline_commit: 'c6016c8c72086d2dbc1e3b969d9c87fffe2488cf'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-3-context.md'
  - '{project-root}/AGENTS.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** There is no way to produce a release candidate. `wails build` runs by hand on one Windows machine, nothing builds a macOS artifact at all, nothing records a checksum, and nothing checks that what ships says FairDrop consistently — the macOS bundle is named from the project name (`fairdrop.app`) while the product is `FairDrop`, and its bundle identifier is Wails' default `com.wails.…`.

**Approach:** A release workflow, triggered by a version tag or by hand, that reuses the Story 3.2 gate through `workflow_call` rather than restating it, then builds each artifact on the runner whose OS it targets, checksums it, uploads it, and collects both into a **draft** release. A Go test pins artifact identity across `wails.json`, the two platform templates and `main.go` so the name, version and identifier cannot drift apart or disagree with the tag.

## Boundaries & Constraints

**Always:** The gate runs to completion before any artifact is built, on the same runner. Windows builds `fairdrop.exe`, macOS builds the `.app` bundle and zips it with `ditto` so the bundle survives. Each artifact is published with a SHA-256 checksum. The tag, `wails.json`'s `productVersion`, and the version in both platform templates agree, asserted before the build. UPX is off by default and reachable only through an explicit workflow input. Every Story 3.2 pin stays green and `verify.yml`'s existing triggers are unchanged. Release copy states the trusted-LAN limit.

**Ask First:** Publishing a release that is not a draft. Changing `wails.json`'s `name` (it renames the macOS bundle and the bundle identifier together). Signing, notarization, or an Apple Developer identity. Adding a Linux artifact. Any change to the frozen HTTP header matrix.

**Never:** `GOOS`/`GOARCH` cross-builds, or presenting one platform's artifact as evidence for another. Claim signing, notarization, auto-update, or Linux packaging in any copy. Loosen a `verify_workflow_test.go` pin to make a release job fit. Publish an artifact built from a tree that failed any check. Decide D-018, D-055, D-064 or D-088 — those are Story 3.10's.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Tagged release | push tag `v0.1.0` | Gate passes on both runners, each builds its own artifact, both upload with checksums, one draft release collects them | N/A |
| Manual run | `workflow_dispatch` | Same, artifacts uploaded, no release created | N/A |
| Version disagreement | tag `v0.2.0`, `productVersion` `0.1.0` | Fails before building, naming both values | Step exit 1 |
| Failing gate | any check red | No artifact is built or uploaded; the failing platform and check are what the run shows | Job fails |
| Identity drift | bundle name, identifier, product name or version disagree across the four files | `release_identity_test.go` fails | Test |
| Stale name | `DeadDrop` reaches a shipped surface | Test fails; the historical spec document is explicitly exempt | Test |
| UPX requested | dispatch input `upx: true` | Build passes `-upx`; default runs without it and acceptance is unchanged | N/A |

</frozen-after-approval>

## Code Map

- `.github/workflows/verify.yml` — add `workflow_call:` beside the existing triggers so the release can reuse the gate. Additive only: `TestVerifyWorkflowTriggersOnPullRequestAndTheRightPushBranches` asserts the `main`/`epic-*` push branches are named, and adding a trigger does not disturb it. Verify by running that test before and after.
- `.github/workflows/release.yml` — new. `on: push: tags: ['v*']` plus `workflow_dispatch` with a boolean `upx` input defaulting false. Job 1 `gate:` → `uses: ./.github/workflows/verify.yml`. Job 2 `build:` → `needs: gate`, same `windows-latest`/`macos-latest` matrix, same pinned setup steps as `verify.yml` (Go `1.26.7`, Node from `.nvmrc`, Wails CLI `v2.15.0` asserted by `wails version`). `actions/upload-artifact@v7` per platform (v7.0.1 is current; `actions/checkout@v7`, `actions/setup-go@v7`, `actions/setup-node@v7` and `actions/cache@v6` stay as `verify.yml` pins them); a final `release:` job `needs: build` runs only for a tag and creates a **draft** release with both archives and the checksum file, using `gh release create --draft` from the checked-out repo rather than a third-party action, so nothing outside the pinned first-party set is trusted with a token.
- Artifact paths, confirmed against Wails v2.15.0's source rather than assumed: Windows produces `build/bin/fairdrop.exe` from `outputfilename`. macOS produces `build/bin/<name>.app` — `packager.go:71` uses `ProjectData.Name`, which is `wails.json`'s `"name": "fairdrop"`, **not** `productName`, so the bundle is `fairdrop.app`. `build.go:447` ad-hoc signs it (`codesign --sign -`); that is not notarization and no copy may imply otherwise. Zip with `ditto -c -k --sequesterRsrc --keepParent` so the bundle is not flattened.
- `build/darwin/Info.plist` — `CFBundleName` is `{{.Info.ProductName}}` (FairDrop), `CFBundleExecutable` is `{{.OutputFilename}}` (fairdrop), `CFBundleIdentifier` is `com.wails.{{safeBundleID .Name}}`. The identifier carrying `wails` is the "platform identity" the acceptance criterion is about; changing it is a template edit here, but the bundle *name* follows `wails.json`'s `name` and is Ask First.
- `build/windows/info.json` — all four fields are already `{{.Info.*}}` templates, so Windows metadata follows `wails.json` with no edit. Read it to confirm rather than change it.
- `wails.json` — `name: fairdrop`, `outputfilename: fairdrop`, `productName: FairDrop`, `productVersion: 0.1.0`. This is the single source the identity test compares everything against.
- `main.go:120` `Title: "FairDrop"` — the window title, already pinned by `TestAppOptionsWindowContract` in `main_test.go:31`.
- `main_test.go:302,430` — the two artifact-integrity tests are the pattern for `release_identity_test.go`: read repo files as text, guard against vacuous passes, name what broke.
- `docs/fairdrop-spec.md` — the only place `DeadDrop` survives. It is the original working spec and its line 11 already records the name as stale. Treat it as historical and exempt it explicitly in the test rather than rewriting a superseded document.
- `internal/qr/qr.go` — the QR dependency is live, not inactive; `frontend/package.json` carries no QR package. The acceptance criterion is satisfied by pinning that, not by removing anything.
- `README.md:63` "Trust model" and `EXPERIENCE.md` — where the trusted-LAN sentence and the banned-claim list live.

## Tasks & Acceptance

**Execution:**
- [x] `.github/workflows/verify.yml` — add `workflow_call`; confirm every 3.2 pin still passes.
- [x] `.github/workflows/release.yml` — gate, per-platform build, checksums, uploads, draft release.
- [x] `build/darwin/Info.plist` — bundle identifier that names FairDrop rather than Wails.
- [x] `release_identity_test.go` — pin name, output filename, product name, version and identifier across `wails.json`, both templates and `main.go`; pin the tag-to-version rule; pin that no shipped surface says `DeadDrop`, with the historical spec exempt by name.
- [x] `README.md`, `EXPERIENCE.md` — trusted-LAN sentence and the banned-claim list, including no signing or notarization despite the ad-hoc signature.
- [x] `evidence-3-3-produce-and-smoke-test-native-release-artifacts.md` — mutation table, gate transcript, and the run URL of a real tagged build.

**Acceptance Criteria:**
- Given a version tag, when the release workflow runs, then both runners pass the full gate before building, each produces its own native artifact with a SHA-256 checksum, and no step cross-compiles.
- Given the identity test, when any of the five identity values is changed in one file alone, then it fails naming the file and the disagreement.
- Given a tag whose version does not match `wails.json`, when the workflow runs, then it fails before building and names both values.
- Given the default inputs, when artifacts are built, then no `-upx` is passed and the produced artifact passes the same checks as one built with it.

## Evidence

Mutation tables, gate transcripts and the tagged run live in
[evidence-3-3-produce-and-smoke-test-native-release-artifacts.md](evidence-3-3-produce-and-smoke-test-native-release-artifacts.md), created with the implementation.

## Spec Change Log

## Design Notes

The gate is reused through `workflow_call` rather than copied because a copied gate drifts, and Story 3.2's whole point was that an unverified build is not a release candidate. Draft rather than published: the repository is public, so a published release is an outward-facing act that should stay a human's to perform. The identity test exists because four files restate the same five facts, and nothing but a person reading all four would notice them disagreeing — which is the same shape as the deferred-citation gap found earlier today.

## Verification

**Commands:**
- `gofmt -l .`; `go vet ./...`; `go tool staticcheck ./...`; `go test -count=1 ./...`; `go test -count=1 -race ./...` — clean
- `cd frontend && npx vitest run`; then, alone, `wails build` — green
- `GOOS=darwin GOARCH=arm64 go vet ./...` and the linux equivalent — clean, per AGENTS.md's pre-flight
- A real tag pushed and its run URL recorded; both artifacts downloaded and their checksums re-computed locally
