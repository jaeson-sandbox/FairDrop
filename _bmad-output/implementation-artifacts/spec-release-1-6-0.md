---
title: 'Release FairDrop 1.6.0'
type: 'chore'
created: '2026-10-11'
status: 'done'
baseline_commit: '4ca2de2'
context:
  - '{project-root}/AGENTS.md'
  - '{project-root}/docs/release-policy.md'
  - '{project-root}/_bmad-output/implementation-artifacts/spec-release-1-5-1.md'
---

<frozen-after-approval reason="owner authorized 1.6.0 on 2026-10-11 after a successful hands-on phone upload">

## Intent

Ship phone-to-desktop receiving (roadmap slice 4, PR #13, merged with 1.5.1 in PR #14), plus the Idle secondary-button spacing defect fix the owner reported on the built app.

## Boundaries & Constraints

**Always:** Bump only product versions. Notes and README describe receiving exactly as the receive contract states, including the folder-per-session choice, the new subfolder, no overwrite, the 3 GB reserve, keep-completed-on-failure, best-effort download marking, the OneDrive/link and macOS FAT/exFAT limits, and the narrowed persistence wording. Keep the trusted-LAN and signing text.

**Never:** Edit historical notes. Publish without exact-head CI, the tagged release run, checksums and a bundle-version check.

## Verification

Owner hands-on: one real phone upload on a build of `main` succeeded (2026-10-11). Then the local ordered gate, exact-head CI, merge to main, tag, release run, downloaded SHA-256 checks, CFBundleShortVersionString 1.6.0, release body equal to `docs/release-notes-1.6.0.md`.
