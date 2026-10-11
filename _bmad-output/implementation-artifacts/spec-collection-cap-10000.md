---
title: 'Raise the collection cap to 10,000 with lazy member preparation'
type: 'feature'
created: '2026-10-11'
status: 'in-progress'
baseline_commit: 'e28cc304c979be64428688997e9815e7e57a8eef'
review_loop_iteration: 0
context:
  - '{project-root}/AGENTS.md'
  - '{project-root}/_bmad-output/specs/spec-multiple-selected-items/SPEC.md'
  - '{project-root}/_bmad-output/specs/spec-multiple-selected-items/selection-contract.md'
  - '{project-root}/_bmad-output/implementation-artifacts/spec-multiple-selected-items.md'
  - '{project-root}/docs/fairdrop-contracts.md'
  - '{project-root}/docs/fairdrop-architecture.md'
---

<frozen-after-approval reason="owner direction 2026-10-11: replace the 16-root cap with 10,000 and ship as 1.5.1 from v1.5.0">

## Intent

**Problem:** A collection is capped at 16 top-level items. An agent chose that number because every member was pinned before response headers: one open file descriptor per loose file, and one directory pin per folder inside the shared 64-handle budget. Sending 50 photos is impossible.

**Approach:** Prepare collection members lazily, one at a time in archive order. Prepare the first member before headers and each later member when the stream reaches it, closing each before the next. Resource use then no longer depends on the item count. Raise the cap to a 10,000-root sanity cap, widen ordinals to the count's width, and make the selection list usable at large counts. Built on the v1.5.0 tag; receiving is not part of this branch.

## Boundaries & Constraints

**Always:**
- Follow the re-derived selection-contract.md.
- Stage still validates every root before networking: existence, kind, no-follow, canonical duplicate and ancestor overlap, and a checked JavaScript-safe aggregate size. Count refusal (0 or more than 10,000) happens before any filesystem work.
- At most one top-level file descriptor and one prepared directory pin are retained at any moment; the count never consumes the 64 + 3 handle budget.
- Per-member identity checks still apply at that member's preparation: a file's opened descriptor must match its Lstat; a folder's walk refuses replacement.
- A later member's failure aborts the ZIP without central-directory finalization.
- Singleton file and folder flows are byte-for-byte unchanged.
- Sonnet implements; the main session reviews and integrates.

**Ask First:** Weakening any no-follow, identity or finalization guarantee. New dependencies.

**Never:**
- Prepare all members before headers.
- Hold more than one member's resources at once.
- Keep a per-descendant index.
- Change receiving code (it is not on this branch).
- Commit, push or bump versions from implementation.
- Hand-edit bindings.
- Relax tests to fit.

## I/O & Edge-Case Matrix

| Input/state | Expected behavior | Failure handling |
|---|---|---|
| 2–10,000 roots | One ZIP; ordinals padded to max(2, digits(count)) | — |
| 0 or 10,001 roots | `invalid_selection` before filesystem work | Never trimmed |
| 100 loose files | Streams completely; never more than one source file descriptor held | — |
| 100 folders, unchanged | Streams completely within the single-folder handle budget | — |
| First member changed after Stage | Pre-stream refusal (existing 410 path) | Nothing streamed |
| Later member changed, replaced or removed after Stage | ZIP aborted mid-stream; never finalized; error reported | Earlier members' resources already closed |
| Cancel/Close mid-collection | Current member's resources released; nothing else held | Bounded teardown |
| Draft list with 10,000 rows | Scrolls; focus visible; count "N of 10,000"; 10,001st append refused with fixed error | Prior rows kept |
| Metadata DTO itemCount 2..10,000 | Accepted; 10,001 refused | Strict validation |

</frozen-after-approval>

## Code Map

- `internal/transfer/coordinator.go`: the collection count limit; future-pin reservations at Stage (`InspectWithRetained`/`otherPins`) drop to the singleton reservation.
- `internal/source/*`: `CollectionSourcePort` accounting may simplify; keep the singleton budget tests intact.
- `internal/stream/payload.go`, `archive.go`: replace all-member preparation with a lazy per-member iterator that prepares the first member before headers; ordinal width; ownership, Close and cancellation across the iterator.
- `app.go`, `selection_source.go`: confirm nothing else assumes 16.
- `frontend/src/transfer/{validation,useTransfer,types}.ts`, `frontend/src/ui/CollectionDraftView.tsx`, `copy.ts`, `style.css`: 10,000 limit, scrolling list, count copy.
- Tests and mutation inventories that encode 16 or pin accumulation: `internal/stream/collection_test.go`, `internal/source/collection_budget_test.go`, `internal/transfer/coordinator_collection_test.go`, `collection_native_test.go`, `_bmad-output/implementation-artifacts/mutate-multiple-selected-items.py`, `scripts/verify-collection-frontend-mutations.py`, `scripts/verify-native-mutations.sh` (check its anchors still match).
- Docs: `docs/fairdrop-contracts.md`, `docs/fairdrop-architecture.md`, README, UX EXPERIENCE/DESIGN where 16 appears.

## Tasks & Acceptance

**Execution:**
- [ ] Backend: lazy member preparation, the 10,000 cap and ordinal width, with tests for every matrix row (failing first where behaviour changes).
- [ ] Frontend: limit, scrolling draft list, copy and validation.
- [ ] Update every test and mutation inventory that encoded 16 or pin accumulation, so each guarantee is still named by a mutation; replace obsolete accumulation guarantees with one-at-a-time guarantees.
- [ ] Docs updated; evidence file `evidence-collection-cap-10000.md` with matrix coverage and mutation totals.

**Acceptance Criteria:**
- Given 100 loose files and 100 folders, when downloaded, then an independently read ZIP contains every byte under `001-…` to `200-…` ordinals, and an instrumented source shows at most one member's resources held at once.
- Given a later member removed after Stage, when streaming reaches it, then the ZIP is aborted unfinalized and all resources are released.
- Given singleton transfers, when the full gate runs, then every existing test passes unchanged.

## Evidence

See `evidence-collection-cap-10000.md`.

## Verification

Canonical ordered gate (Wails before frontend checks, never concurrently), platform preflight, all three mutation scripts. Native Windows and macOS CI is the platform proof.
