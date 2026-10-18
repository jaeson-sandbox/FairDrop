---
title: 'Release FairDrop 1.8.0'
type: 'chore'
created: '2026-10-18'
status: 'in-review'
baseline_commit: 'af4199273318602da22fbdcf7ec1c72a8c2ed2c9'
review_loop_iteration: 0
context:
  - '{project-root}/AGENTS.md'
  - '{project-root}/docs/release-policy.md'
  - '{project-root}/_bmad-output/specs/spec-phone-to-desktop-text/receive-text-contract.md'
---

<frozen-after-approval reason="owner requested publication, with standing authorization for spec-first Sol implementation and integration">

## Intent

Publish FairDrop 1.8.0 so users can download the completed phone-to-desktop text/link feature. Update product metadata and release documentation, then build, verify and publish matching native artifacts through the existing release pipeline.

## Boundaries & Constraints

**Always:** Preserve automated gates and existing trust/platform limits. Main owns specification, review, verification, commits, PR, integration, tagging and publication; Sol implements metadata/docs and focused proof. Preserve unrelated build/.DS_Store. Treat absent manual checks honestly. User authorization includes publishing the verified draft without another approval.

**Ask First:** New features, dependency changes, changed trust/signing model, or a blocker that cannot be resolved within this release scope.

**Never:** Change runtime behavior, photo selection, historical release notes, dependencies, or workflow guarantees; claim unperformed manual passes; replace an existing tag or published asset; publish failed, cancelled or mismatched artifacts. Implementation subagent performs no remote operations.

## I/O & Edge-Case Matrix

| Scenario | Input/state | Expected outcome | Refusal |
| --- | --- | --- | --- |
| Identity | Current product 1.7.0 | Native/frontend/root lock metadata agree on 1.8.0 | Reject mixed versions or dependency churn |
| Contents | Feature merged at baseline | Notes describe explicit Receive text, bounded ephemeral receipt and copy | No automatic clipboard or delivery claims |
| Gate | Exact PR and tag heads | Windows/macOS native and Linux adapter jobs succeed | Preserve failures; no waived or cancelled jobs |
| Assets | Tag v1.8.0, native builds | `fairdrop.exe`, `fairdrop.exe.sha256`, `fairdrop-macos.zip`, `fairdrop-macos.zip.sha256`; matching hashes and Windows x86_64/macOS arm64 identity | No publication on mismatch |
| Publication | Verified draft | Correct notes, public stable latest v1.8.0 | Never overwrite existing publication |

</frozen-after-approval>

## Code Map

- `wails.json` info.productVersion drives `build/darwin/Info.plist` and `build/windows/info.json`; retain their templates.
- `frontend/package.json` and top-level/root package versions in `frontend/package-lock.json` are product fields; all dependency records stay identical.
- `README.md` current release link and `_bmad-output/specs/spec-receiver-handoff/roadmap.md` published baseline need 1.8.0. Existing receiving/text descriptions already describe the implementation.
- `docs/release-notes-1.7.0.md` supplies style and limitations; add `docs/release-notes-1.8.0.md` rather than changing history. Feature contract and implementation evidence define exact text bounds, normalization, explicit native copy, clearing and acknowledgement semantics.
- `release_identity_test.go` verifies coherent identity, native templates, gate reuse, checksum checks and draft safeguards. Existing mutation inventory exercises these guarantees.
- `.github/workflows/release.yml` reuses the full verification gate, builds native Windows/macOS assets and creates a draft on tag push. Draft creation is tested; refusing a rerun that could clobber published assets is Main's preflight. Main replaces its generic body with the reviewed notes, verifies downloads, then publishes. Never rerun its clobber-capable upload path against a published release.

## Tasks & Acceptance

- [x] Sol: bump only product versions to 1.8.0 in `wails.json`, frontend package and lock root fields; update README/roadmap release references.
- [x] Sol: add `docs/release-notes-1.8.0.md` covering phone-to-desktop text/link receipt, normalization/limits, one-shot semantics, explicit copy, memory clearing and existing trusted-LAN/signing/checksum limitations. Use version-pinned absolute documentation links.
- [x] Sol: run focused identity tests and structured dependency-diff checks; create companion evidence with matrix coverage and pending remote stages.
- [ ] Main: complete three independent review layers and ordered local gate, commit/push, open PR, verify exact-head CI, merge without squashing.
- [ ] Main: confirm absent tag/release, tag merged head, verify all exact-tag release jobs, download and inspect artifacts, replace draft notes, publish and confirm latest/public identity. Record publication proof in `_bmad-output/implementation-artifacts/release-evidence.md` on a follow-up documentation branch.

Given passing identity and automated gates, when v1.8.0 is published, then both native downloads identify the release and match their checksums. Given any failed required check or inconsistent asset, publication remains pending until corrected. Given the new receive-text flow, when release notes describe it, then they distinguish receipt from clipboard success and preserve the limits of local in-memory clearing.

## Evidence

See evidence-release-1-8-0.md beside this spec for matrix coverage, reviews, logs, exact heads and publication results.

## Spec Change Log

## Verification

Run `go test -count=1 -run 'TestRelease|TestBoth|TestTheDraft|TestTheRelease|TestTheVersionIsStatedOnceAndFollowedEverywhere' .`. Parse product metadata and compare baseline lock data after removing only the two product version fields. Main runs AGENTS' ordered gate and canonical mutations in an isolated snapshot; Wails precedes frontend checks. Canonical entrypoints are `scripts/verify-native-mutations.sh` (including `scripts/verify-receive-text-mutations.py` for receive Go), `scripts/verify-text-mutations.py`, `scripts/verify-text-frontend-mutations.py`, and `scripts/verify-receive-text-frontend-mutations.py`, plus the runner unit suite `python3 -m unittest discover -s scripts -p 'verify_receive_text_frontend_mutations_test.py'`. Read each CI job conclusion and headSha directly. The merged tree must equal the verified PR tree; the release gate independently checks that merged tag head.

Before tagging, verify both remote v1.8.0 and release absence. Download the draft's `fairdrop.exe`, `fairdrop.exe.sha256`, `fairdrop-macos.zip`, and `fairdrop-macos.zip.sha256`; check SHA-256 sidecars, macOS arm64 architecture, both plist version keys, deep/strict codesign integrity and ad-hoc identity. Inspect Windows x86_64 PE architecture and VERSIONINFO strings/FileVersion. Existing Windows numeric ProductVersion is unset (0.0.0.0); preserve and record it rather than inventing a new requirement. Replace draft body with the checked-in notes, verify equality, then publish and confirm the latest-release API shows stable public v1.8.0 and matching assets. Manual phone/native interaction remains optional and unverified unless performed.

## Suggested Review Order

- Start with shipped behavior and the release’s limits.
  [release-notes-1.8.0.md:1](../../docs/release-notes-1.8.0.md#L1)

- Confirm the download reference and inbound-text description.
  [README.md:3](../../README.md#L3)

- Native metadata supplies both platform templates.
  [wails.json:16](../../wails.json#L16)

- Frontend identity follows the native product version.
  [package.json:4](../../frontend/package.json#L4)

- Trace reviews, gates, native artifacts, and publication proof.
  [evidence-release-1-8-0.md:1](evidence-release-1-8-0.md#L1)
