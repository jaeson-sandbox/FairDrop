---
title: 'Release FairDrop 1.7.0'
type: 'chore'
created: '2026-10-15'
status: 'in-review'
baseline_commit: '954d91813b0d6b190be8bc19267eb46739c92d5f'
review_loop_iteration: 0
context:
  - '{project-root}/AGENTS.md'
  - '{project-root}/docs/release-policy.md'
  - '{project-root}/_bmad-output/implementation-artifacts/evidence-release-1-6-0.md'
  - '{project-root}/_bmad-output/implementation-artifacts/evidence-text-link-sharing.md'
---

<frozen-after-approval reason="owner explicitly requested release, with standing approval for specification and Sol implementation">

## Intent

Publish FairDrop 1.7.0 with the completed connection recovery and text/link sharing additions since 1.6.0, including the receive-notice claim race fix. Produce downloadable Windows and macOS artifacts whose version, tag, notes and checksums agree. Release only; photo improvements are on hold until a separate owner discussion.

## Boundaries & Constraints

**Always:** Keep the existing release policy and automation gates. Describe actual behavior and limitations truthfully. Preserve the trusted-LAN/plain-HTTP, signing, notarization, checksum and platform limitations. Preserve build/.DS_Store. Main orchestrates review, gate, integration, tag, asset verification and publication; Sol implements version/docs changes and focused checks. The owner has authorized publication, so no repeat approval is necessary.

**Ask First:** New feature scope, altered signing/trust model, new dependency, or inability to satisfy an automated gate.

**Never:** Change unrelated dependency versions or historical release notes, weaken tests/workflows, claim manual iPhone/Photos verification, add photo-picker behavior in this release story, overwrite an existing published tag/release, or publish unverified binaries. Implementation subagent must not commit, push, tag or publish.

## I/O & Edge-Case Matrix

| Scenario | Input/state | Expected result | Failure handling |
|---|---|---|---|
| Product version | Existing 1.6.0 metadata | Wails and frontend root versions 1.7.0; package dependencies unchanged | Refuse inconsistent product identity |
| Release contents | Main at baseline | Notes cover connection recovery, bounded text and receive-claim fix | Verify claims against active contracts |
| Tag/artifacts | v1.7.0 with native builds | Both native assets, SHA-256 sidecars and macOS bundle version agree | Do not publish on mismatch |
| CI | Exact branch/main/tag heads | Every required native/adapter job passes | Preserve full failure output; fix rather than waive |
| Publication | Verified draft, matching notes | Public non-prerelease v1.7.0 with four expected assets | Verify release body/assets/latest after publishing |

</frozen-after-approval>

## Code Map

- `wails.json` info.productVersion is the native version source. `build/darwin/Info.plist` and `build/windows/info.json` template it; do not replace placeholders.
- `frontend/package.json` version and `frontend/package-lock.json` top-level/root-package versions are product fields. A dependency currently contains ^1.6.0; it is unrelated and must not change.
- `README.md` identifies current release and describes usage. New text paragraph precedes file-oriented Send Again retention wording; clarify that files retain paths and text retains the bounded value in memory. Keep historical “new in 1.6.0” receive claim accurate.
- `docs/release-notes-1.6.0.md` is read-only style/trust context. Add `docs/release-notes-1.7.0.md` with bounded text, explicit reveal/native copy, fresh-code recovery, unchanged receiving safety, and truthful completion semantics.
- `_bmad-output/specs/spec-receiver-handoff/roadmap.md` records the current published baseline; update the release reference as part of this release, without inventing another completed feature.
- `release_identity_test.go` checks version/template identity and release workflow safeguards. `.github/workflows/release.yml` reuses the native verify gate, builds native artifacts, checksums them and creates a draft on v* tags; main later verifies and publishes that draft.
- `_bmad-output/implementation-artifacts/evidence-release-1-7-0.md` records focused verification, three review layers, full local gate, exact heads, release run, downloaded checksums/version and final URL.

## Tasks & Acceptance

- [x] Bump only product metadata to 1.7.0 and update release-facing README/roadmap references.
- [x] Add accurate release notes for all changes since v1.6.0, retaining trust/platform limitations and explaining text retention without overstating delivery or photo support.
- [x] Run focused release identity tests and inspect version-only package-lock diff. Record results and matrix coverage in evidence; leave full ordered gate and remote work to main.
- [ ] Main: complete three review layers and full local gate, commit/push verified milestone, open PR, read exact-head CI conclusions and merge with no-ff.
- [ ] Main: verify merged head CI, tag v1.7.0, await all release jobs, download assets and validate hashes, macOS plist versions/ad-hoc signature, Windows PE architecture and embedded FileVersion/ProductVersion, and notes equality; explicitly replace the workflow-generated draft body with docs/release-notes-1.7.0.md before checking equality, then publish and confirm public release/latest identity.

Given consistent 1.7.0 metadata and passing automated gates, when the owner-authorized release is published, then both downloadable artifacts identify the intended release and have matching SHA-256 sidecars. Given any failed or cancelled required job or mismatched artifact, publication remains blocked until corrected. Given a browser-side photo selection question, notes do not claim unobserved native behavior or original-photo conversion guarantees.

## Evidence

See evidence-release-1-7-0.md beside this spec. Main fills remote evidence after publication; pending stages must be clearly labeled pending.

## Spec Change Log

## Verification

Focused implementation checks: `go test -count=1 -run 'TestRelease|TestBoth|TestTheDraft|TestTheRelease' ./` plus structured JSON inspection of product versions and unchanged dependency records. Main runs the ordered AGENTS gate in an isolated snapshot, including Wails before frontend tests, platform preflight and canonical mutations. Existing mutation inventory covers release identity/workflow defenses; do not create tests that compare a version literal to itself. Use exact run headSha and each job conclusion, never a watch exit code. Manual native/phone checks are optional under release policy and remain unverified unless actually performed.

Release preflight must confirm v1.7.0 tag and release do not exist before creating the tag. Never rerun the upload workflow against a published release: its existing retry path clobbers assets. Inspect macOS architecture with `lipo -archs`, both plist version keys with PlistBuddy, and ad-hoc identity with `codesign -dv --verbose=4`; require `codesign --verify --deep --strict` success. Inspect downloaded Windows VERSIONINFO as well as PE architecture. Use `gh release edit v1.7.0 --notes-file docs/release-notes-1.7.0.md` while still a draft, compare body bytes, then publish only the verified draft.

## Suggested Review Order

- Start with the shipped behavior and accurately bounded release claims.
  [release-notes-1.7.0.md:1](../../docs/release-notes-1.7.0.md#L1)

- Native product version is the source for both platform templates.
  [wails.json:16](../../wails.json#L16)

- Frontend product metadata follows the same version without dependency changes.
  [package.json:4](../../frontend/package.json#L4)

- Current download reference and retention wording match the release.
  [README.md:11](../../README.md#L11)

- Trace checks, review outcomes and publication proof.
  [evidence-release-1-7-0.md:1](evidence-release-1-7-0.md#L1)
