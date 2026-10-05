---
title: 'Inspect a transfer before downloading'
type: feature
created: '2026-10-04'
status: done
review_loop_iteration: 0
baseline_commit: cb92086c6168538cf80c1e88bc224e3d646be7f8
context:
  - '{project-root}/AGENTS.md'
  - '{project-root}/_bmad-output/specs/spec-receiver-handoff/SPEC.md'
  - '{project-root}/_bmad-output/specs/spec-receiver-handoff/receiver-protocol.md'
---

<frozen-after-approval reason="owner authorized sequential implementation after specification, 2026-10-04">

## Intent

**Problem:** A GET of the capability URL immediately consumes the offered file, including a link preview. The receiver cannot inspect what is offered before claiming it.

**Approach:** GET renders an accessible item page; its Download form sends POST to the same capability path, which alone enters the existing one-shot claim pipeline. This story implements CAP-1 only; Send Again follows separately.

## Boundaries & Constraints

**Always:** Preserve constant-time capability matching, one receiver, coordinator authorization before payload opening, safe streamed file/ZIP delivery, finalization-based completion, cancellation and bounded teardown. Metadata comes only from the staged display fields. Apply the companion protocol and keep wrong requests generic. Update active contract/copy/tests for this intentional method change.

**Ask First:** Changes to trust model, persistent state, parallel receivers, content previews or receiver authentication.

**Never:** Implement later roadmap slices here; add dependencies or remote assets for the page; read payloads on GET; weaken native verification or edit historical evidence to claim new behavior. Keep the existing untracked build/.DS_Store untouched. No release/version bump in this story.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
| --- | --- | --- | --- |
| Inspect | Valid GET, staged file/folder | 200 escaped HTML; file size or ZIP explanation; POST form | No payload prepare/read, claim, event or lifecycle transition |
| Revisit | Repeated/concurrent GETs | Available page; session stays STAGED | No reservation |
| Download | Valid POST, unclaimed | Existing exact file/ZIP attachment and finalization | Existing coded failure path |
| Race | Concurrent POSTs; GET racing POST | At most one authorization/payload; competing valid requests 423 while live | Page may already be stale; never second download |
| Invalid | HEAD/PUT/etc., wrong token/path, noncanonical path | Same generic 404 as baseline | No disclosure or side effect |
| Metadata | HTML metacharacters, Unicode, long name, folder | Escaped display only; responsive 320px; no source path | No injected script/resource/form |
| Cancel/change | Cancel before POST or source changes before claim | Existing refusal/closed listener or 410 | No reopened session; honest sender error |

</frozen-after-approval>

## Code Map

- `internal/server/handler.go` (`route`, `download`) — method/token checks, reservation, existing claim pipeline and headers.
- `internal/server/lifecycle.go` (`run`, route registration) — immutable staged item, listeners and teardown; keep methodless routing.
- `internal/server/{handler,finalization,lifecycle,longevity}_test.go`, `helpers_test.go` — real HTTP consumption, negative methods, concurrency, final-write proofs.
- `app*_test.go`, `scripts/verify-native-mutations.sh` — integration GET consumers and exact text mutations; inspect all active consumers, not historical prose.
- `internal/transfer/{coordinator,ports}.go` — URL and authority contracts; URL path remains unchanged.
- `frontend/src/ui/copy.ts`, `frontend/src/ui/{copy,StagedView}.test.*`, `frontend/browser/` — sender instructions and rendered proof patterns.
- `docs/fairdrop-contracts.md`, `docs/fairdrop-architecture.md`, `README.md`, `_bmad-output/specs/spec-fairdrop/`, Quartz `EXPERIENCE.md` — authoritative current HTTP/copy contracts. Canonical SPEC updates go through bmad-spec memlog/derivation; coordinate with orchestrator instead of hand-patching it.

## Tasks & Acceptance

**Execution:**
- [x] `internal/server/handler.go` and new receiver template — implement metadata GET and explicit POST without changing downstream transfer ownership.
- [x] `internal/server/*_test.go`, relevant root integration tests and mutation script — cover every matrix row, adapting actual byte consumers without weakening rejection/finalization tests.
- [x] `frontend/src/ui/copy.ts`, tests and Quartz EXPERIENCE — explain scan then Download and first downloader; remove obsolete preview caveat coherently.
- [x] `frontend/browser/` and test support — exercise the actual server-rendered form and responsive/focus behavior; do not substitute a hand-copied template. If a harness is needed, keep it test-only.
- [x] Current docs/contracts — reconcile new semantics and record canonical SPEC change needed for orchestrator.
- [x] Sibling evidence file — record row-to-test audit, meaningful mutations and full log paths; run local gate sequentially, report limitations exactly.

**Acceptance Criteria:**
- Given a staged real file or folder, when a browser visits then submits Download, then only submission starts transfer and exact bytes or a valid ZIP arrive.
- Given repeated and racing requests, when GETs and POSTs overlap, then one payload is authorized at most and cancellation remains bounded.
- Given untrusted metadata, when the real page renders, then no code executes, no source path or external resource leaks, and the form is usable by keyboard at narrow widths.
- Given existing supported platforms, when native gates execute, then streaming/finalization, frontend and mutation proofs remain intact; foreign-platform passes are never inferred from local tests.

## Evidence

[evidence-receiver-landing-page.md](evidence-receiver-landing-page.md)

## Spec Change Log

- 2026-10-04: Initial spec; owner explicitly authorized specification followed by Sol implementation without repeated approval.

## Design Notes

The URL remains a capability, not authenticated identity. A POST form prevents ordinary GET previews from consuming it; it does not prevent an intentional capability holder from downloading. After teardown the listener is gone, so no expired-page promise is made. Keep baseline byte-stream tests strong while changing their deliberate download verb.

Implementation runs in the shared named branch; no commit, push or additional agent dispatch by implementer. Main orchestrator owns review and Git milestones. Use gpt-6-sol for story implementation under the latest owner direction.

## Verification

Sequential local canonical gate from AGENTS.md: Wails build, drift checks, gofmt, vet, staticcheck, full Go, cgo check and race, frontend and rendered browser suites, LF check; Darwin/Linux build preflights. Run targeted protocol/mutation proofs first and retain complete output. Orchestrator handles native CI after a verified commit.

## Suggested Review Order

- Inspect the method boundary before reviewing the unchanged claim pipeline.
  [handler.go:55](../../internal/server/handler.go#L55)

- Read the self-contained page and receiver response policy.
  [landing.go:1](../../internal/server/landing.go#L1)

- Check the agreed request matrix and preserved invariants.
  [receiver-protocol.md:1](../specs/spec-receiver-handoff/receiver-protocol.md#L1)

- Follow real HTTP assertions for inspection, races and escaped metadata.
  [landing_test.go:1](../../internal/server/landing_test.go#L1)

- Inspect reproducible regressions for each receiver guarantee.
  [verify-receiver-mutations.py:1](../../scripts/verify-receiver-mutations.py#L1)

- Verify the actual rendered form submits and receives exact bytes.
  [receiver-live.mjs:1](../../frontend/browser/receiver-live.mjs#L1)

- Review test results, limitations and independent findings.
  [evidence-receiver-landing-page.md:1](evidence-receiver-landing-page.md#L1)

