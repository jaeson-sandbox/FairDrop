---
title: 'Release FairDrop 1.5.1'
type: 'chore'
created: '2026-10-11'
status: 'in-progress'
baseline_commit: 'e28cc304c979be64428688997e9815e7e57a8eef'
context:
  - '{project-root}/AGENTS.md'
  - '{project-root}/docs/release-policy.md'
  - '{project-root}/_bmad-output/implementation-artifacts/spec-release-1-5-0.md'
---

<frozen-after-approval reason="owner authorized 1.5.1 on 2026-10-11: the collection cap change only, built from v1.5.0">

## Intent

**Problem:** 1.5.0 limits a multi-item send to 16 items, an agent-chosen number the owner rejected.

**Approach:** A patch release from the v1.5.0 tag containing only the 10,000-item cap with lazy member preparation (spec-collection-cap-10000.md), plus metadata and notes. Phone-to-desktop receiving, which is on `main`, is excluded and must not be mentioned as shipped. After publication, merge the branch forward into `main`.

## Boundaries & Constraints

**Always:** Bump only product versions (`wails.json`, `frontend/package.json`, the root fields of `frontend/package-lock.json`). Notes state the new limit, that the selection is still checked in full before sharing, and the trade-off: an item changed or removed after Send now fails partway through the download, and the ZIP is never finalized. Keep the trusted-LAN and signing limitation text from 1.5.0.

**Never:** Ship receiving. Edit historical notes. Publish without exact-head CI, a tagged release run, checksums and a bundle-version check.

## Verification

Exact-head CI on the hotfix branch; tagged release run with all jobs; downloaded SHA-256 checks; CFBundleShortVersionString 1.5.1; body equals `docs/release-notes-1.5.1.md`; then merge forward into `main` with CI green.
