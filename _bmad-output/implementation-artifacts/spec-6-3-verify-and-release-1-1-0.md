---
title: 'Story 6.3: Verify and Release 1.1.0'
type: 'chore'
created: '2026-09-20'
status: 'done'
baseline_commit: '023faaad6fdafe60a44556190a6d95d4c78a4c2d'
review_loop_iteration: 0
context: ['{project-root}/AGENTS.md']
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Epic 6 builds and its Mac logo displays, but its retrospective rejected
acceptance because required mutation proofs were missing. The release still identifies
as 1.0.0. GitHub authentication on this Mac also blocks pushing.

**Approach:** Close the outstanding verification and documentation work, set coherent
1.1.0 metadata, pass native CI, merge with a merge commit, and publish the resulting
native release with verified checksums. Preserve the prior rejected verdict as history.

**Closes:** D-133, D-134, D-135; retrospective action items 32–34.

## Boundaries & Constraints

**Always:** Use the canonical mutation inventory and retain complete failure logs.
Mutate isolated checkouts or temporary copies, restore inputs, and prove restoration.
Require native Windows/macOS and Linux adapter CI conclusions for the exact merge
candidate. Keep release builds native, checksums verified after artifact transit,
and existing unsigned/ad-hoc/notarization limitations explicit.

**Ask First:** Artwork changes, expanded product behavior, weakening acceptance, or
replacing an existing published version/tag.

**Never:** Change transfer behavior, weaken tests to pass, waive functional failures,
force-push shared history, publish before the gate, or claim unobserved manual passes.
D-131 and D-132 remain explicitly accepted limitations; no new icon artwork,
macOS regression gate, signing, notarization, or installer is required here.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
| --- | --- | --- | --- |
| Asset proofs | Four D-133 guards and required size inventory | Each isolated mutation fails its intended assertion | Missing case or wrong failure rejects proof |
| Windows resource proofs | Missing embedded icon, neutral version key, stale payload/version | Real built-resource tests fail by name | Compile errors and skips are not kills |
| Missing committed ICO | Isolated Windows checkout with ICO untracked after rebuilding | Canonical drift step rejects manufactured asset | Retain failing output; restore checkout |
| No executable | Local source-only run versus post-build CI | Local test skips; CI refuses skip | Never count absence as verification |
| Version update | Wails and npm metadata set to 1.1.0 | Native artifact versions and v1.1.0 agree | Existing tag mismatch gate blocks release |
| GitHub unavailable | Missing/expired credentials | Local work retained; publish waits | Report blocker, never claim push/merge |

</frozen-after-approval>

## Code Map

- `scripts/verify-asset-mutations.py`: `CASES`, `ASSETS`, restoration, and `run_exe_table`; currently 27 asset cases and three executable probes.
- `appicon_test.go`: `wantIcoSizes`, `measuredEntryDistance255`, geometry symmetry, non-square freshness refusal, source digest agreement. Existing assertions are the mutation targets.
- `exe_resources_windows_test.go`: built PE parser and two artifact tests; skip only on missing binary. Reuse this consumer for Windows negative proofs.
- `.github/workflows/verify.yml`, `verify_workflow_test.go`: native post-build checks, exact drift clause, skip refusal, and step/order pins.
- `release_identity_test.go`, `.github/workflows/release.yml`: version coherence, native gate reuse, tag check, draft artifact creation and checksum recheck.
- `wails.json`, `frontend/package.json`, `frontend/package-lock.json`: product and root npm versions; dependency versions stay unchanged.
- `build/README.md`, `AGENTS.md`, Story 6.2 spec, `epics.md`: D-135 drift. Native Mac packaging never regenerates the Windows ICO.

## Tasks & Acceptance

**Execution:**
- [x] `scripts/verify-asset-mutations.py` — derive sizes, add the four isolated cases, restore every mutated source, and retain unique complete logs.
- [x] `exe_resources_windows_test.go`, `scripts/verify-exe-mutations.go` — execute the missing built-resource and language-key probes using standard-library tooling; preserve local skip behavior.
- [x] `.github/workflows/verify.yml`, `verify_workflow_test.go`, `scripts/verify-build-asset-drift.sh` — execute native Windows proof and the exact drift rejection in isolation; keep pins synchronized and avoid Pillow/image derivation in CI.
- [x] `AGENTS.md`, `build/README.md`, Story 6.2 spec, `epics.md` — reconcile actual gate order, absence-based gating, 0409/FileVersion behavior, calibration command, and Windows-specific regeneration. Record the supersession of obsolete frozen wording under this approved story.
- [x] `wails.json`, both frontend package files, `README.md` — set 1.1.0 identity and accurate release description.
- [x] `epics.md`, `sprint-status.yaml`, `deferred-work.md`, retrospective/evidence files — register Story 6.3, record three review layers, discharge only proved items, and reassess acceptance while preserving history.
- [x] `docs/release-notes-1.1.0.md`, `release-evidence.md` — prepare notes, then record exact merge/tag SHA, run/job conclusions, artifact sizes and checksums after release.

**Acceptance Criteria:**
- Given the final mutation inventory, when it executes, then every declared case fails the named assertion, absence skips only locally, and inputs are restored with complete evidence retained.
- Given D-133–D-135, when closure is recorded, then each has concrete proof or corrected documentation and the retrospective no longer has unmet mandatory acceptance criteria.
- Given the reviewed 1.1.0 candidate, when the full native gate runs, then every required job succeeds on that exact head before a non-fast-forward merge.
- Given the merged tag, when Release completes, then native Windows/macOS artifacts identify as 1.1.0, their downloaded checksums verify, and the published release accurately states changes and limits.
- Given authentication remains unavailable, when publication is attempted, then no remote success or completed release is recorded.

## Evidence

See `evidence-6-3-verify-and-release-1-1-0.md` for review, mutation, gate and release results.

## Spec Change Log

- 2026-09-20: Recorded the successful final-candidate PR and push gates, merge commit,
  annotated tag, successful Release workflow, published artifact identities, and
  post-transit checksum verification. Story accepted and closed.

## Execution ownership

Approved by Jaeson on 2026-09-20, including implementation, verification, merge and
publication without repeated approval. Implementation is delegated to Sol; the
parent session orchestrates independent review, remote CI, merging, tagging and
publication. The implementer prepares local changes and evidence, leaves remote
operations to the parent, and reports externally gated acceptance as incomplete.
Authentication has been repaired with `gh auth login`; the earlier intent records
the initial problem. Do not infer permission to bypass CI or reuse an existing tag.

## Verification

Run Wails first, then the ordered canonical local checks and cross-platform preflight;
run asset mutations in an isolated worktree and Windows artifact/drift proofs natively.
Inspect GitHub run and job conclusions directly. Reopen the built Mac app after the
version change and inspect packaged version/icon; nearby-device checks remain optional.

## Suggested Review Order

1. [Story evidence](evidence-6-3-verify-and-release-1-1-0.md)
2. [Canonical mutation driver](../../scripts/verify-asset-mutations.py)
3. [Executable mutation helper](../../scripts/verify-exe-mutations.go)
4. [Native verification workflow](../../.github/workflows/verify.yml)
5. [Release notes](../../docs/release-notes-1.1.0.md)
