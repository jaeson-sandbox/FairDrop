---
title: 'Release FairDrop 1.5.0'
type: 'chore'
created: '2026-10-10'
status: 'done'
baseline_commit: '8712760'
review_loop_iteration: 0
context:
  - '{project-root}/AGENTS.md'
  - '{project-root}/docs/release-policy.md'
  - '{project-root}/_bmad-output/implementation-artifacts/spec-release-1-4-0.md'
---

<frozen-after-approval reason="owner explicitly authorized release on 2026-10-10">

## Intent

**Problem:** Sending multiple selected items as one streamed ZIP (roadmap slice 3, PR #11) is merged and verified, but published binaries still identify as 1.4.0 and lack it.

**Approach:** Prepare minor release 1.5.0 with consistent metadata and accurate user-facing notes, mirroring the 1.4.0 release. The orchestrator verifies, merges, tags, inspects native artifacts and publishes under the owner's explicit authorization.

## Boundaries & Constraints

**Always:** Use existing version templates and release pipeline. Describe the collection feature exactly as specified in `_bmad-output/specs/spec-multiple-selected-items/` (2–16 files/folders, native multi-drop or Add Files/Add Folder list, one receiver, preview shows item count and total size, one `FairDrop.zip` with numbered `FairDrop/NN-<name>` members, whole selection validated before sending, any failing member fails the whole transfer, Send Again/Try Again revalidate every item). Keep one receiver, ephemeral in-memory state, trusted-LAN plain HTTP and existing signing limitations accurately described. Preserve unrelated files, including untracked build/.DS_Store. Implementation belongs to a Sonnet subagent; release coordination belongs to the main session. Record evidence honestly.

**Ask First:** A new feature, dependency upgrade, changed transport/security contract, or destructive replacement of an existing published release.

**Never:** Change transfer code, workflows, dependencies, icon assets or historical release notes merely to cut this release. Claim publication or artifact verification before it happens. Present planned roadmap items (phone-to-desktop receiving, connection recovery, text sharing) as shipped. Commit, push, merge, tag or publish from the implementation subagent.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|---|---|---|---|
| Version identity | Release configuration and npm root metadata | All product version surfaces identify 1.5.0; platform templates still derive from Wails | Reject disagreement |
| Accidental dependency bump | Lockfile contains dependency versions | Only package root metadata changes; dependency graph identical | Restore unrelated edits |
| Collection description | Reader of notes/README | 2–16 items, one ZIP, numbered names, atomic validation, count/size preview, no per-item paths disclosed | Do not imply >16 items, multiple receivers, or a staged/snapshotted copy |
| Single item | Existing user | Notes state one file/folder behaves as in 1.4.0 | No compatibility change claimed for single items |
| Publication | Tag workflow produces native assets | Publish only after exact revision checks pass and downloaded checksums match | Hold release on failed or missing proof |

</frozen-after-approval>

## Code Map

- `wails.json` info.productVersion: native release identity source.
- `frontend/package.json` and the two root version fields of `frontend/package-lock.json`: product version repeats; dependency entries unrelated.
- `release_identity_test.go`: version agreement and workflow constraints; reuse, do not add constant-comparison tests.
- `README.md`: "Current release" line and any 1.4.0-as-current statements, plus any development-only caveat on collections.
- `docs/release-notes-1.4.0.md`: template for concise notes; create `docs/release-notes-1.5.0.md`, never edit historical notes.

## Tasks & Acceptance

**Execution:**
- [x] Bump only product versions to 1.5.0 in `wails.json`, `frontend/package.json`, `frontend/package-lock.json`.
- [x] `README.md`: present collections as a 1.5.0 capability; current release v1.5.0; no development-only caveat.
- [x] `docs/release-notes-1.5.0.md`: feature description, unchanged single-item behavior, trusted-LAN scope, unsigned Windows / ad-hoc macOS, no notarization/auto-update/Linux packaging, checksums prove integrity not authenticity. Becomes the published release body.
- [x] `evidence-release-1-5-0.md` beside this spec: diff audit and focused verification; remote release proof left pending for the orchestrator.

**Acceptance Criteria:**
- Given the merged feature baseline, when release changes are inspected, then only product metadata, current-release documentation and this story's artifacts change.
- Given the prepared tree, when existing gates run, then they pass with consistent native identity and no dependency or binding drift.
- Given native CI artifacts for tag v1.5.0, when downloaded checksums and bundle metadata check out, then the orchestrator publishes 1.5.0 as latest and records revision and URL.

## Evidence

See [release evidence](evidence-release-1-5-0.md).

## Verification

- Implementer: `go test -count=1 -run 'Release|Identity|Version' .`, structured JSON comparison of package-lock against baseline; full failing output retained if any.
- Orchestrator: canonical sequential gate; exact-head CI; tag-triggered release workflow; four assets, SHA-256 sidecars, CFBundleShortVersionString 1.5.0, release state after publication.
