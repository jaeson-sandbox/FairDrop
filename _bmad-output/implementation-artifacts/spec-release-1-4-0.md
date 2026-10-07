---
title: 'Release FairDrop 1.4.0'
type: 'chore'
created: '2026-10-07'
status: 'in-review'
baseline_commit: '7fdb98ede732d15a1f1129168e9ae9d805e37900'
review_loop_iteration: 0
context:
  - '{project-root}/AGENTS.md'
  - '{project-root}/docs/release-policy.md'
---

<frozen-after-approval reason="owner explicitly authorized preparation and publication">

## Intent

**Problem:** The receiver landing page and Send Again are merged and verified, but published binaries still identify as 1.3.1 and lack these features.

**Approach:** Prepare the next minor release, 1.4.0, with consistent metadata and accurate user-facing notes. The orchestrator will verify, merge, tag, inspect native artifacts and publish the release under the owner's explicit authorization.

## Boundaries & Constraints

**Always:** Use existing version templates and release pipeline. Keep one file or folder, one receiver, ephemeral in-memory state, trusted-LAN plain HTTP, and existing signing limitations accurately described. Preserve unrelated files, including untracked build/.DS_Store. Implementation belongs to a Sol subagent; release coordination belongs to the main session. Record evidence honestly, distinguishing already verified feature behavior from newly verified release artifacts.

**Ask First:** A new feature, dependency upgrade, changed transport/security contract, or destructive replacement of an existing published release.

**Never:** Change transfer code, workflows, dependencies, icon assets or historical release notes merely to cut this release. Claim publication or artifact verification before it happens. Include planned multi-item selection or receiving on the desktop as shipped features. Commit, push, merge, tag or publish from the implementation subagent.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|---|---|---|---|
| Version identity | Release configuration and npm root metadata | All product version surfaces identify 1.4.0; platform templates still derive from Wails | Reject disagreement |
| Accidental dependency bump | Lockfile contains dependency version 1.3.1 | Only package root metadata changes; dependency graph stays identical | Restore unrelated edits |
| Receiver compatibility | User opens capability URL | Notes explain GET previews metadata and explicit Download POST claims transfer | Do not imply GET still downloads or shell clients remain unchanged |
| Repeated send | Completed item chosen again | Notes describe revalidation and a fresh link/QR; no persistent history or resume | No promise of old-link reuse |
| Publication | Tag workflow produces native assets | Publish only after exact revision checks pass and both downloaded checksums match | Hold release on failed or missing proof |

</frozen-after-approval>

## Code Map

- `wails.json` info.productVersion is the native release identity source.
- `frontend/package.json` and the two root version fields in `frontend/package-lock.json` repeat the product version; dependency entries are unrelated.
- `build/darwin/Info.plist`, `build/darwin/Info.dev.plist`, and `build/windows/info.json` already template identity correctly; read-only.
- `release_identity_test.go` verifies version agreement, native template wiring, gate ordering, checksum checks and workflow constraints; reuse existing tests.
- `README.md` currently calls 1.3.1 current and labels the receiver workflow as development-only; update those statements for 1.4.0.
- `docs/release-notes-1.3.1.md` illustrates concise historical notes; create a new file instead of editing it.
- `_bmad-output/specs/spec-receiver-handoff/SPEC.md` and `receiver-protocol.md` define the shipped features and GET-to-POST compatibility change.
- `.github/workflows/release.yml` builds Windows exe and macOS zip, their SHA-256 sidecars and a draft after the full native gate; publication stays an orchestrator action.

## Tasks & Acceptance

**Execution:**
- [x] `wails.json`, `frontend/package.json`, `frontend/package-lock.json` — bump only product versions to 1.4.0.
- [x] `README.md` — present the new receiver and Send Again behavior as 1.4.0 capabilities with no development-only caveat.
- [x] `docs/release-notes-1.4.0.md` — describe both features, direct HTTP client migration from GET to POST, ephemeral path retention, trusted-LAN scope, unsigned Windows and ad-hoc-only macOS, no notarization/auto-update/Linux packaging. State checksums prove integrity, not authenticity. These notes become the published release body.
- [x] `evidence-release-1-4-0.md` beside this spec — record implementation diff audit and focused verification. Leave remote release proof explicitly pending for the orchestrator.
- [ ] Verify matrix rows through existing automated identity tests, structured lockfile comparison, documentation review and, for publication, orchestrator-owned artifact checks. Do not add trivial constant-comparison tests.

**Acceptance Criteria:**
- Given the merged feature baseline, when the release changes are inspected, then only product metadata, current release documentation and this story's artifacts change.
- Given the prepared release tree, when the existing build and verification gates run, then they pass with consistent native product identity and no dependency or generated-binding drift.
- Given native CI artifacts for the release tag, when their downloaded checksums and metadata are checked, then the orchestrator may publish 1.4.0 as the latest stable release and record its exact revision and URL.

## Evidence

See [release evidence](evidence-release-1-4-0.md).

## Spec Change Log

## Verification

- Implementer: focused existing release identity/workflow tests and JSON comparison against baseline; retain full failing output if any.
- Orchestrator: canonical sequential Wails build, drift, formatting, vet, staticcheck, Go tests, cgo/race, frontend and browser tests, line endings and platform preflight. Preserve full logs outside the spec.
- Orchestrator: exact-head Windows/macOS/Linux CI, then tag-triggered release workflow; inspect four assets, verify downloaded SHA-256 sidecars, native bundle 1.4.0 metadata and release state after publication. Windows execution proof comes from native CI, not cross compilation.

## Suggested Review Order

- Read the feature summary and client migration note.
  [release-notes-1.4.0.md:1](../../docs/release-notes-1.4.0.md#L1)
- Check the native product identity source.
  [wails.json:16](../../wails.json#L16)
- Inspect verification and publication evidence.
  [evidence-release-1-4-0.md:1](evidence-release-1-4-0.md#L1)
