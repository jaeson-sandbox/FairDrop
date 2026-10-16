---
title: 'Send short text and links to a nearby browser'
type: 'feature'
created: '2026-10-14'
status: 'done'
baseline_commit: '9c6813ab0c6401f2f592b62d218e708d8c5f85cd'
review_loop_iteration: 0
context:
  - '{project-root}/AGENTS.md'
  - '{project-root}/_bmad-output/specs/spec-text-link-sharing/SPEC.md'
  - '{project-root}/_bmad-output/specs/spec-text-link-sharing/text-contract.md'
  - '{project-root}/_bmad-output/specs/spec-connection-recovery/recovery-contract.md'
  - '{project-root}/docs/fairdrop-contracts.md'
  - '{project-root}/docs/fairdrop-architecture.md'
  - '{project-root}/_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/DESIGN.md'
  - '{project-root}/_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/EXPERIENCE.md'
---

<frozen-after-approval reason="standing owner authorization to implement roadmap in priority order with Sol after specification">

## Intent

Let a desktop user send short text or a URL to one nearby browser without creating a file. One bounded text mode, explicit receiver reveal, ordinary selection/copy, and existing single-session lifecycle.

## Boundaries & Constraints

**Always:** Follow the canonical text contract and all companions. Preserve file, collection, receive and recovery guarantees. Enforce raw and normalized UTF-8 limits on their own boundaries; preserve normalized text exactly. Main orchestrates; Sol implements. URL content is inert text. Complete only after verified full local response write and finalization.

**Ask First:** New dependency, persistence, weakened security/lifecycle invariant, or independently shippable extra feature.

**Never:** Automatically read clipboard, classify/fetch/open a pasted URL, use a fake file or filesystem storage for text, disclose payload on GET or in events/metadata/logs, promise remote delivery/copy/secure erasure, introduce bidirectional clipboard synchronization, change release versions or publish. Do not commit/push from implementation. Preserve build/.DS_Store. Main owns canonical SPEC/memlog/roadmap; report needed amendments.

## I/O & Edge-Case Matrix

| Input/state | Expected behavior | Failure handling |
|---|---|---|
| Valid text/URL, Unicode, spaces, TAB, CR/CRLF | Normalize only newline sequences; retain exact resulting bytes | No trimming, URL handling or file I/O |
| Raw >131072 bytes, normalized >65536, blank, malformed Unicode, forbidden controls | Reject before staging or retained unbounded copies | Fixed field error; no listener/clipboard reads |
| Frontend unpaired surrogate / Go invalid UTF-8 | Refuse at boundary that observes it | No reliance on JSON preserving malformed Unicode |
| Exact GET text route | Metadata and Read text form only | No claim or disclosure, no-store headers |
| Valid POST / concurrent POST | One authorized HTML reveal after claim, escaped exact textarea value | Losing claim safe refusal; no disclosure before authorization |
| Wrong token/path/method, HTML-looking payload | Ordinary refusal grammar; literal inert content | No script execution or external resource/navigation |
| Short write/disconnect/cancel/finalization stall | No false completion, bounded existing cleanup | Truthful error, no body after refused authorization |
| Send Again/retry/waiting recovery | Same bounded payload, fresh session/token/QR | Internal lease release retains text; stale replies cannot resurrect it |
| Composing/staging/active plus drop/chooser/receive | Single mutually exclusive operation | Fixed refusal; no hidden mode replacement |
| User Cancel/Done/dismiss/replace/unmount | Forget draft/remembered text and capability | Late acknowledgments/events ignored/retired |
| Browser without scripting/Clipboard API | Readonly textarea permits native selection and copy | No nonfunctional custom button or false Copied label |
| Keyboard/320px/forced colors | Labelled usable composer, readable reveal, visible focus and announcements | Preserve Quartz/native WebKit mechanisms |

</frozen-after-approval>

## Code Map

- `internal/transfer/{types,ports,session,coordinator,identity,recovery}.go`: add explicit text payload/metadata and stage admission; reuse activate and one-shot claim/teardown. Recovery currently reinspects paths: immutable text needs its own branch with no SourcePort call. Preserve notice-specific bounded claim fix.
- `internal/server/{handler,landing,lifecycle}.go`: reuse exact-token GET/POST and strict receiver headers. Dedicated text reveal must not call PayloadPort.Prepare or set attachment headers. Set Content-Length, verify full write, and use existing HTTP finalization. Render plain strings through html/template and preserve leading LF despite textarea parsing rules.
- `app.go`: additive StageText; existing CopyToClipboard URL limit is unrelated. Regenerate bindings with Wails before frontend checks.
- `frontend/src/transfer/{types,validation,state,useTransfer}.ts`: explicit text metadata/mode; literal /text route validation; remembered text separate from paths and safe receipts. Recovery currently discriminates send/receive: extend it explicitly. Internal cancelImpl(true) must preserve Send Again text while explicit cancellation clears it.
- `frontend/src/{App.tsx,ui,style.css}`: route Send text/composer, focus, announcements and all input admission through existing controller. Programmatic focus uses :focus or explicit marker; buttons clicked on WebKit need explicit focus where keyboard handling requires it.
- Existing transfer/server/app/frontend/browser tests and canonical mutation scripts provide seams. Test assertions must compare independent literals; mutation scripts run only in isolated copies.

## Tasks & Acceptance

Work in the repository containing this spec: resolve the checkout root by walking up from the absolute spec path, even if the inherited shell starts in another checkout. Main owns canonical spec/memlog/roadmap edits, final full gate, integration and release coordination. Run focused implementation checks and scoped mutations; report full-gate work for the main orchestrator rather than duplicating it concurrently.

- [x] Add pure shared text validation semantics and bounded in-memory stage/metadata model with no filesystem dependency.
- [x] Implement metadata-only landing and authorized script-free reveal with exact escaped textarea content and truthful HTTP completion.
- [x] Add bound StageText, generated bindings, explicit frontend text states/composer/waiting/outcomes and safe lifecycle cleanup.
- [x] Extend Send Again/retry and waiting recovery with fresh capability identity and immutable text retention.
- [x] Update active contracts/architecture, Quartz copy/experience and README. Main updates canonical specs/roadmap.
- [x] Exercise every matrix row and canonical named-assertion mutations, preserving complete evidence and test failures.

Given an accepted text or URL, when the receiver previews and explicitly reveals it, then the textarea value exactly matches normalized input and one claim succeeds without any source/sink access. Given cancellation, short writes, stale responses or concurrent input, no payload leaks or false completion occurs. Given Send Again or refresh, a fresh code offers the same text and the retired code cannot claim.

## Evidence

Write matrix coverage, red/green checks, canonical mutation inventory and logs, review triage and gate results to evidence-text-link-sharing.md beside this file.

## Spec Change Log

## Verification

Run Wails before frontend, then complete ordered AGENTS.md gate and platform preflight. Use isolated snapshots for build drift, browser captures and all mutations. Native exact-tip Windows/macOS and Linux CI must pass before integration. Manual nearby-device/native UI observations are optional and must remain unverified unless performed. No release/version change in this feature slice.

## Suggested Review Order

**Entry and validation**

- Stage bounded text through the existing lease and activation lifecycle.
  [coordinator_text.go:7](../../internal/transfer/coordinator_text.go#L7)

- Enforce independent raw and normalized limits while preserving accepted text.
  [text.go:15](../../internal/transfer/text.go#L15)

**Receiver boundary**

- Separate preview from authorized, escaped reveal and finalized completion.
  [text.go:17](../../internal/server/text.go#L17)

**Fresh capabilities**

- Retire waiting capabilities before offering the same immutable text again.
  [recovery.go:55](../../internal/transfer/recovery.go#L55)

**Desktop lifecycle**

- Coordinate composer admission, cancellation, retries and stale acknowledgments.
  [useTransfer.ts:374](../../frontend/src/transfer/useTransfer.ts#L374)

- Present bounded text, fixed copy and accessible command failures.
  [TextViews.tsx:7](../../frontend/src/ui/TextViews.tsx#L7)

**Verification**

- Prove old HTTP capabilities stop serving after refresh.
  [text_integration_test.go:48](../../internal/server/text_integration_test.go#L48)

- Exercise text admission, retention, cancellation and refusal behavior.
  [text.test.tsx:37](../../frontend/src/transfer/text.test.tsx#L37)

- Enumerate named backend mutation defenses with preserved verdict logs.
  [verify-text-mutations.py:10](../../scripts/verify-text-mutations.py#L10)
