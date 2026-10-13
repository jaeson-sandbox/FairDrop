---
title: 'Recover a waiting connection on another local network'
type: 'feature'
created: '2026-10-12'
status: 'in-review'
baseline_commit: '578a2910dc985bb89963d7719000e2a46808437d'
review_loop_iteration: 0
context:
  - '{project-root}/AGENTS.md'
  - '{project-root}/_bmad-output/specs/spec-connection-recovery/SPEC.md'
  - '{project-root}/_bmad-output/specs/spec-connection-recovery/recovery-contract.md'
  - '{project-root}/docs/fairdrop-contracts.md'
  - '{project-root}/docs/fairdrop-architecture.md'
  - '{project-root}/_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/DESIGN.md'
  - '{project-root}/_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/EXPERIENCE.md'
---

<frozen-after-approval reason="owner authorized roadmap work sequentially, spec first and Sol implementation without repeated approvals">

## Intent

**Problem:** Automatic network selection can advertise an unusable address; changing networks leaves an old waiting code. Users currently must cancel and select their sources or destination again.

**Approach:** Offer eligible network choices and explicit connection refresh under Trouble connecting in both waiting views. Replace the waiting session safely with fresh identity while preserving its subject. The recovery-contract companion defines all command, lifecycle and UI behavior.

## Boundaries & Constraints

**Always:** Follow recovery-contract.md. Preserve every existing eligibility filter, deterministic automatic choice, operation lease, generation fence, bounded cleanup and disclosure rule. Claim/recovery race winners are backend-authoritative. Preserve lazy 10,000-root collections and receive sink safety. Main orchestrates; Sol implements.

**Ask First:** A weakened lifecycle/security invariant, new dependency, persistent state, or additional independently shippable feature.

**Never:** Cancel a winning claim to recover; start a new listener before confirmed retirement; accept arbitrary IPs; treat local candidates as proven reachable; retain receive paths in frontend; introduce background polling/probes; implement text/link sharing, release or version changes. Do not commit/push from implementation. Preserve unrelated build/.DS_Store. Main owns canonical SPEC/memlog/roadmap edits; report amendments rather than changing them.

## I/O & Edge-Case Matrix

| Input/state | Expected behavior | Failure handling |
|---|---|---|
| Waiting send/receive, list options | Fresh sorted eligible pairs and current address, no live mutation | Fixed safe load error; retry |
| Automatic/explicit refresh | Retire old capability, revalidate/send or retain receive capability, new ID/token/URL/QR | No silent explicit fallback |
| Empty/stale ID, active or terminal session | No resource/state mutation | Safe refusal |
| Claim races refresh | Exactly one wins coordinator ownership | Losing recovery never cancels claim |
| Choice disappears or enumeration fails before retirement | Old session preserved | network_unavailable or existing safe code |
| Server/beacon/drainer cannot quiesce | No replacement listener | Coded failure, honest unconfirmed cleanup |
| Setup fails after retirement | Old code removed; no leaked listener/destination | Idle with actionable fixed error |
| Cancel/shutdown or duplicate refresh | Single owner, bounded cleanup, no late resurrection | Ordinary cancellation semantics |
| Receive refresh | No saved file/subfolder; existing destination owned exactly once | Close retained capability on final failure |
| Late list/refresh response or early events | Only current generation applied | No stale code/focus/selection overwrite |

</frozen-after-approval>

## Code Map

- `internal/network/{address,network,beacon}.go`: eligibleInterface, canonicalIPv4, candidateLess and enumeration are reusable. GetLocalIP mutates selected state; listing must be pure. Extend through consumer-owned optional network capabilities.
- `internal/transfer/{coordinator,lifecycle,session,receive,ports,types}.go`: activate shares send/receive setup; it commits STAGED and releases its lease, so refactor carefully for recovery ownership. AuthorizeClaim reserves CLAIMING under the same lock; use that boundary, never a frontend Cancel sequence. Session identity is immutable.
- `internal/server/{lifecycle,upload}.go`: Stop quiesces HTTP but does not close ReceiveDestination. Upload authorizes before SaveFile; only the coordinator transfers/closes its destination ledger. Preserve existing sink identity enforcement.
- `app.go`, `main.go`: additive bound commands and composition; return structured safe DTOs. Regenerate frontend/wailsjs with Wails, never manually.
- `frontend/src/transfer/{useTransfer,state,types,validation}.ts`: generation guards, early-event buffering, typed validation and reset handling. Receive currently forgets folder paths; preserve that policy.
- `frontend/src/{App.tsx,ui/StagedView.tsx,ui/ReceiveWaitingView.tsx,ui/copy.ts,style.css}`: place a shared accessible recovery panel inside connection-help disclosures; route focus/announcements through existing mechanisms.
- Existing coordinator claim/lifecycle/receive tests, network dependency seams and rendered browser tests provide deterministic fixtures. Keep canonical native mutation anchors working after helper changes.

## Tasks & Acceptance

**Execution:**
- [x] Extend network discovery/explicit selection and safe public DTOs; test pure enumeration and fresh validation.
- [x] Implement waiting-only recovery ownership and retirement/rebuild for send and receive; preserve shared helpers and ledger cleanup.
- [x] Add bound commands and regenerate Wails bindings; keep old APIs compatible.
- [x] Implement controller/parser/panel, stale-response guards, pending-code removal, keyboard focus and announcements.
- [x] Update active API contracts, architecture decision log, Quartz copy/design and README development notes. Main updates canonical specs.
- [x] Cover every matrix row with executed tests and portable named-assertion mutations; retain full logs and review-ready evidence.

**Acceptance Criteria:**
- Given a waiting send, when a different eligible endpoint is chosen, then a fresh code serves exact bytes and the retired token cannot claim.
- Given a waiting receive, when refreshed, then the approved destination still receives safely without another chooser and closes exactly once on final exit.
- Given deterministic claim/recovery races and stuck cleanup, when recovery runs, then active payloads remain untouched and no replacement can bypass quiescence.
- Given keyboard and narrow-layout use, when recovery succeeds or fails, then focus, announcements and displayed codes match backend state.

## Evidence

Write executed matrix coverage, mutation inventory and full log references to [implementation evidence](evidence-connection-recovery.md).

## Spec Change Log

## Verification

Run Wails first, then ordered bindings/assets/gofmt/vet/staticcheck/Go/cgo/race/frontend/browser/LF gate and platform preflight from AGENTS.md. Never overlap Wails and frontend tests. Run the complete existing native mutation inventory and new canonical scoped mutations in isolated copies; compilation failures are not kills. Native Windows/macOS and Linux CI precede integration. Manual native UI remains optional and must be labeled unobserved unless actually exercised.

## Suggested Review Order

- Reserve waiting ownership, retire the old capability, and transfer receive destination ownership safely.
  [recovery.go:55](../../internal/transfer/recovery.go#L55)

- List candidates without changing the live endpoint; revalidate explicit choices at activation.
  [network.go:64](../../internal/network/network.go#L64)

- Expose the additive refresh command through the Wails boundary.
  [app.go:260](../../app.go#L260)

- Fence stale acknowledgments and carry early replacement events into the new session.
  [useTransfer.ts:614](../../frontend/src/transfer/useTransfer.ts#L614)

- Hide retired codes and preserve waiting state on preflight refusal.
  [state.ts:211](../../frontend/src/transfer/state.ts#L211)

- Offer on-demand network selection inside the existing help disclosure.
  [ConnectionRecoveryPanel.tsx:15](../../frontend/src/ui/ConnectionRecoveryPanel.tsx#L15)

- Prove retired tokens fail and replacement send and receive sessions transfer exact bytes.
  [connection_recovery_integration_test.go:40](../../connection_recovery_integration_test.go#L40)

- Break lifecycle guarantees and require their named tests to fail.
  [verify-connection-recovery-mutations.py:1](../../scripts/verify-connection-recovery-mutations.py#L1)

- Prove stale-response and retired-code guards with isolated frontend mutations.
  [verify-connection-recovery-frontend-mutations.py:1](../../scripts/verify-connection-recovery-frontend-mutations.py#L1)

