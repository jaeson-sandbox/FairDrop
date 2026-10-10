---
title: 'Send multiple selected items as one streamed ZIP'
type: 'feature'
created: '2026-10-07'
status: 'done'
baseline_commit: 'c56a39eee5494b8d8706e1b801e5fc077245223b'
review_loop_iteration: 0
context:
  - '{project-root}/AGENTS.md'
  - '{project-root}/_bmad-output/specs/spec-multiple-selected-items/SPEC.md'
  - '{project-root}/_bmad-output/specs/spec-multiple-selected-items/selection-contract.md'
  - '{project-root}/docs/fairdrop-contracts.md'
  - '{project-root}/docs/fairdrop-architecture.md'
  - '{project-root}/_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/DESIGN.md'
  - '{project-root}/_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/EXPERIENCE.md'
---

<frozen-after-approval reason="owner authorized next roadmap intent with specification before Sol implementation">

## Intent

**Problem:** Sending several files or folders requires repeated sessions or manually creating a containing directory.

**Approach:** Add a bounded, accessible in-memory selection list and stream two through 16 selected roots as one ZIP through the existing single-session lifecycle. Preserve ordinary one-item sending. The sibling product contract and its selection-contract.md define the complete behavior.

## Boundaries & Constraints

**Always:** Follow selection-contract.md, including atomic validation, numbered archive names, immutable selection ownership, aggregate size checks, global handle accounting, all-root preparation, source identity, cleanup and disclosure rules. Reuse existing ports, native drop, outcome recovery and ZIP machinery. Sol implements; main reviews and integrates.

**Ask First:** Any weakened security/resource guarantee, new dependency, persistence, or feature outside the bounded collection handoff.

**Never:** Publish or bump versions; commit/push from implementation; edit generated bindings by hand; silently discard selected items; create a synthetic filesystem folder or staged ZIP; relax tests to fit broken behavior. Preserve unrelated build/.DS_Store.

## I/O & Edge-Case Matrix

| Input/state | Expected behavior | Failure handling |
|---|---|---|
| One file/folder | Existing byte or ZIP flow unchanged | Existing typed errors |
| 2–16 mixed roots, matching basenames | Atomic session, N items preview, single ZIP with distinct numbered members | No skipped members |
| Zero/17 paths, duplicate or overlap | Reject before networking; count refusal before filesystem work | invalid_selection, safe copy |
| Invalid member, size overflow, cancellation | Refuse whole collection, no listener/partial Stage | Existing safe codes; overflow path_unsupported |
| Multiple pins and deep tree | At most 64 retained directories + 3 transient; admission accounts for future pins | Safe refusal, no leaks |
| Later source replaced after Prepare | Directory refuses replacement; file reads pinned original descriptor | Never substitute new content or finalize failed ZIP |
| Partial Prepare/Close-before-write/disconnect | Close every acquired resource, bounded teardown | Never false quiescence/success |
| List Add/Remove/Cancel, stale chooser | Accessible count/list, correct focus, no stale append or early Stage | Dismissal no-op; invalid input preserves usable list |
| Collection retry/Send Again | Copy/revalidate all paths with fresh token after actual reset | Preserve lease-release recovery; forget on cancel/dismiss/unmount |

</frozen-after-approval>

## Code Map

- `internal/transfer/{types,ports,coordinator}.go`: immutable aggregate, Stage admission and metadata; external work stays outside the mutex.
- `selection_source.go`: admitted ancestor-resolution decorator; sequential canonicalization preserves generation-scoped busy and bounded timeout recovery.
- `internal/source/{source,prepared}.go`: no-follow metadata, prepared root identity, directory budget; independent one-pin capabilities cannot prove collection-wide accounting.
- `internal/stream/{payload,archive}.go`: verified top-level file descriptors, prepared directories, streaming ZIP, worker/Close ownership and aborted finalization.
- `internal/server/{server,lifecycle,landing}.go`: accept real aggregate metadata and render N items; preserve GET/POST authorization and framing.
- `app.go`: add StageTransfers/SelectFiles, keep singleton wrapper and safe chooser errors. Wails runtime OpenMultipleFilesDialog supports files only.
- `frontend/src/transfer/{useTransfer,types,validation,state,selectors}.ts`, `frontend/src/App.tsx`: collection command generation, mutable-input defense, native multi-drop, Send Again/retry lifecycle.
- `frontend/src/ui/`: add Quartz selection list; update copy, summaries, receipt, progress and announcements to distinguish a collection.
- Source hardening, prepared/payload/archive, coordinator and selection-source tests contain reusable identity, arithmetic, cancellation and ownership fixtures. Frontend and rendered-browser tests cover focus and double admission.

## Tasks & Acceptance

**Execution:**
- [x] Extend domain/ports and coordinator for immutable atomic multi-root admission and checked aggregate metadata.
- [x] Extend source budgeting and payload/archive preparation for bounded all-root ownership and a single correctly named ZIP.
- [x] Extend app/server and regenerate bindings with Wails before frontend checks.
- [x] Implement accessible collection controls and end-to-end batch lifecycle, including repeat/retry and metadata validation.
- [x] Update active contracts, architecture decision log, Quartz UX and README development section; preserve 1.4.0 published behavior distinction. Main owns canonical SPEC/memlog re-derivation; report required amendments, do not hand-edit canonical SPEC.
- [x] Cover every matrix row with executed assertions; derive portable scoped mutations from one canonical inventory, record baseline and assertion failures with unique full logs in sibling evidence.

**Acceptance Criteria:**
- Given mixed roots with duplicate basenames, when one receiver downloads, then an independently read ZIP contains every expected byte and empty directory under unique numbered roots, without nested ZIPs or staged storage.
- Given cancellation or a replaced later root, when preparation/streaming exits, then every resource is released or honestly reported unquiescent and no completed archive/success is claimed.
- Given keyboard-only interaction, when items are added, removed and sent again, then focus, announcements, single-flight admission and memory-only retention remain correct.
- Given existing single-item features, when full native and frontend gates run, then existing single-file, folder, receiver-preview and outcome recovery guarantees still pass.

## Evidence

See [implementation evidence](evidence-multiple-selected-items.md).

## Spec Change Log

- Pre-implementation design review clarified existing per-kind preparation and resolver recovery. Files retain verified descriptors across pathname replacement; directories refuse replacement at Walk without adding snapshot comparisons. Resolver timeout releases admission while blocked OS work may persist. KEEP all no-follow, resource and generation checks; do not strengthen this feature into snapshots or indefinite busy.

## Verification

Run Wails before frontend checks, then canonical ordered drift/format/vet/staticcheck/Go/cgo/race/frontend/browser/line-ending gate and platform preflight. Never run Wails concurrently with frontend tests. Add focused red/green and mutation proof for count bounds, aggregate overflow, immutability, collision names, collection-wide handle budget, later-root replacement, cleanup and stale/repeated UI admission. Native CI remains mandatory; do not claim synthetic Escape or browser-unit checks prove WKWebView interaction.

## Suggested Review Order

**Contract and admission**

- Bounded selection and ownership rules guide the entire handoff.
  [selection-contract.md:1](../specs/spec-multiple-selected-items/selection-contract.md#L1)

- Validate every root before acquiring network resources.
  [coordinator.go:226](../../internal/transfer/coordinator.go#L226)

**Preparation and streaming**

- Prepare all members transactionally with one shared directory budget.
  [payload.go:263](../../internal/stream/payload.go#L263)

- Resolve once while preserving cancellation and bounded recovery.
  [selection_source.go:142](../../selection_source.go#L142)

**Sender interaction**

- Keep paths private and guard draft admission and outcome recovery.
  [useTransfer.ts:524](../../frontend/src/transfer/useTransfer.ts#L524)

- Present ordered rows, accessible errors and native selection controls.
  [CollectionDraftView.tsx:17](../../frontend/src/ui/CollectionDraftView.tsx#L17)

**Verification**

- Exercise real mixed-collection delivery through the native adapters and HTTP.
  [collection_native_test.go:20](../../collection_native_test.go#L20)

- Review independent findings, corrections and executed mutation evidence.
  [evidence-multiple-selected-items.md:1](evidence-multiple-selected-items.md#L1)
