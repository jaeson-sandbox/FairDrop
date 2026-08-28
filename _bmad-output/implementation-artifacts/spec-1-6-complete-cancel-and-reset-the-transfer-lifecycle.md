---
title: 'Story 1.6 — Complete, Cancel, and Reset the Transfer Lifecycle'
type: 'feature'
created: '2026-08-27'
status: 'review'
review_loop_iteration: 0
baseline_commit: '2720bfbf30de9cb018713e2107bd0033bf9e3901'
context:
  - '{project-root}/docs/fairdrop-contracts.md'
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The coordinator can start a transfer but cannot finish one. `drain` reads every server event and throws it away, and there is no Cancel, Shutdown, or reset — so a committed session's listener, mDNS registration, session context and drainer goroutine outlive the coordinator with nothing able to release them, and a sender whose link was claimed would see `transfer-started` followed by silence forever.

**Approach:** Give server events lifecycle meaning and add the three public lifecycle commands. Progress and terminal outcomes flow from the drainer onto the coordinator's emission lane; Cancel wins from any state by joining the teardown already in flight instead of starting a second one; an injected three-second timer publishes reset and returns to IDLE; Shutdown closes the application lifetime.

## Boundaries & Constraints

**Always:** Never hold the state mutex across an adapter or `Observer` call. Only the holder of the operation lease publishes, so emission order follows causality rather than scheduling. The drainer never *blocks* acquiring the lease — a held lease means a teardown already owns that outcome — and progress that cannot take it is dropped, which the contract permits for progress alone. Assign a sequence number under the mutex only to an event that is actually published, so a dropped snapshot leaves no gap. Accept exactly one terminal outcome per session, and no progress after it. Cancel and Shutdown mark the generation cancelled, cancel the data-plane context, and wait on the existing teardown. Return from Cancel and Shutdown only once the resources they name are quiescent.

**Ask First:** Bounding any quiescence wait with a watchdog, which would trade a hang for returning while a resource is still live. Any change to the contract's port, `Event`, `Observer`, or `ProgressSnapshot` shapes, or a `NewCoordinator` signature change to make a missing port unrepresentable. Any new dependency. Any background goroutine beyond the per-session drainer and the reset timer.

**Never:** No Wails, HTTP-handler, or frontend work, and no frontend-visible state beyond the events themselves. No second cleanup path competing with the lease, no progress after a terminal outcome, and no cancellation rendered as a transfer error. Never let a capability token, source path, or raw adapter text reach an event, a warning, or a diagnostic. No persistence, telemetry, or logs.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Progress accepted | Matching session, TRANSFERRING, no terminal yet | Next contiguous sequence assigned; `transfer-progress` published | N/A |
| Progress while the lease is held | Teardown or started publication in flight | Snapshot dropped; no sequence consumed, no gap | N/A |
| Progress refused | Wrong session, wrong state, or after terminal acceptance | Discarded silently; nothing published, no state change | N/A |
| Natural success | `ServerComplete` while TRANSFERRING | Lease taken; server and beacon quiesced; authoritative final progress then `transfer-complete`; DONE; reset armed | N/A |
| Natural failure with bytes | `ServerFailed` carrying a snapshot | Quiesce, final progress, then `transfer-error`; ERROR; reset armed | Recognized code preserved |
| Natural failure before any byte | `ServerFailed` with nil progress | Quiesce, then `transfer-error` only — no zero-byte progress event | Unknown or nil cause maps to `transfer_failed` |
| Failure coded `cancelled` | Server reports it with no coordinator teardown pending | `transfer-error` published as `transfer_failed` | Cancellation copy never reaches an error event |
| Second terminal event | One already accepted | Discarded; state unchanged | N/A |
| Lane closes unexpectedly | TRANSFERRING, no terminal accepted, no teardown requested | One synthesized outcome, then the normal ERROR path | `transfer_failed` |
| Lane closes during teardown | Cancel or Shutdown requested the Stop | Silent; no event, no synthesis | N/A |
| Cancel while IDLE | No session | Returns nil | No adapter call, no event |
| Cancel while STAGING | Stage in flight | Generation marked, context cancelled, joins Stage's own unwind; Stage returns `cancelled`; IDLE | No lifecycle event |
| Cancel while STAGED | Nothing claimed | Resources quiesced, one `transfer-reset`, IDLE before Cancel returns | N/A |
| Cancel while CLAIMING | Before the TRANSFERRING commit | Authorization denied, quiesce, one reset, IDLE | Claim returns `cancelled` |
| Cancel after the commit | `transfer-started` already published | Data plane cancelled, server and payload quiesced, queued outcomes discarded, one reset | No complete and no error |
| Cancel while DONE or ERROR | Reset timer armed | Timer stopped, one reset, session cleared, IDLE | N/A |
| Reset timer fires | DONE or ERROR, generation matches | One `transfer-reset`, session cleared, IDLE | N/A |
| Stale reset timer | Session replaced or already cleared | Publishes nothing and mutates no state | N/A |
| Timer races Cancel | Both reach the terminal transition | Exactly one reset published | N/A |
| Shutdown | Any state | Closing set, contexts cancelled, timer stopped, teardown joined, further events suppressed; returns only when quiescent | Diagnostics recorded internally |
| Command after Shutdown | Stage or Cancel | Refused with no state or resource change | `shutting_down` |
| Repeated Shutdown | Already closed | Idempotent; no second teardown, no second event | N/A |
| Disclosure | Every event, warning, and diagnostic produced above | Contains no capability token, source path, or adapter text | N/A |

</frozen-after-approval>

## Code Map

- `docs/fairdrop-contracts.md:260-286` — **read-only, binding.** Command/state table, the operation-lease rule, and the two linearization points.
- `docs/fairdrop-contracts.md:305-330` — **read-only, binding.** Event grammar, `seq` rules, per-event payload validity, and the `Percent` rules.
- `docs/fairdrop-contracts.md:211-218` — **read-only, binding.** Port postconditions. `ServerPort.Stop` being force-closing and quiescent on every return is what every wait in this story rests on.
- `internal/transfer/coordinator.go:510` — `drain` currently discards every event; this story is what it was left open for.
- `internal/transfer/coordinator.go:480` — `unwind` joins the drainer. Terminal handling runs *inside* the drainer, so it needs a variant that does not join itself.
- `internal/transfer/coordinator.go:394-452` — `AuthorizeClaim`: the precedent for non-blocking lease acquisition, and the publication-under-lease ordering to preserve.
- `internal/transfer/coordinator.go:545` — `revalidateLocked`: extend its state check to the new terminal states rather than adding a parallel validator.
- `internal/transfer/coordinator.go:579-601` — `cancelSession` and `beginClosing`: today unexported and test-only. `Cancel` and `Shutdown` are built from them.
- `internal/transfer/coordinator.go:141-149` — `Dependencies`: add the timer seam beside `Entropy` and `Now`.
- `internal/server/events.go:70-101` — **read-only.** The lane is non-blocking and terminal-once; the coordinator never has to keep up to stay safe.
- `internal/server/handler.go:202` — **read-only.** `finish(nil)` is how the server stays silent about an outcome the coordinator owns, and `finish` closes the *listener*, never the lane.
- `internal/server/lifecycle.go:268` — **read-only.** Only `teardown` closes the lane, so an unexpected lane close means `Stop` ran.
- `internal/transfer/errors.go:117-146` — `publicMessages` and `PublicErrorOf` already do the recognized/unknown mapping; do not write a second one.
- `internal/transfer/helpers_test.go:167` — the `enter()` gate every fake passes through. New fakes and new call sites keep using it.
- `internal/transfer/helpers_test.go:125` — `fakeServer.events` is unbuffered today; terminal-event tests need to drive it.
- `app.go`, `main.go`, frontend — read-only. Story 1.7 wires `Cancel` and `Shutdown` up.

## Tasks & Acceptance

**Execution:**
- [x] `internal/transfer/coordinator.go` — add `stateDone`/`stateError`, the session's terminal and reset-timer fields, and the injectable `AfterFunc` timer seam; make publication lease-owned.
- [x] `internal/transfer/outcomes.go` — **new.** Drainer event handling: progress forwarding, terminal acceptance, unexpected-close synthesis, and the unwind variant that does not join the drainer.
- [x] `internal/transfer/lifecycle.go` — **new.** Public `Cancel` and `Shutdown`, and the generation-checked reset timer.
- [x] `internal/transfer/*_test.go` — a fake timer and a controllable server lane; drive every matrix row, force the timer/Cancel race both ways, and force Cancel at each state boundary.
- [x] `_bmad-output/implementation-artifacts/deferred-work.md` — close or restate the four Story 1.5 entries this story inherits.

**Acceptance Criteria:**
- Given a claimed transfer driven to each terminal outcome under `-race` with repeats, when the drainer, a Cancel, a Shutdown and the reset timer contend, then no deadlock or race occurs and the lease is unheld once the coordinator reaches IDLE.
- Given any single session, when every published event is collected, then `seq` starts at 1, increases by exactly one, carries exactly one terminal event, and matches the contract's per-event payload table.
- Given Cancel issued from IDLE, STAGING, STAGED, CLAIMING, TRANSFERRING, DONE and ERROR in turn, when it returns, then the coordinator is IDLE, every acquired resource was released exactly once, and the published grammar is the one the contract names for that state.
- Given a terminal outcome followed by a session that is replaced or cleared, when the stale reset timer fires, then it publishes nothing and mutates no state.
- Given every event, warning, and diagnostic this story can produce, when each is searched for the staged path and the capability token, then neither appears.

## Spec Change Log

## Design Notes

**The deadlock this story exists to avoid.** Terminal handling needs the lease (it stops adapters) but runs on the drainer goroutine, and `unwind` waits for the drainer. If the drainer ever blocked on the lease, a Cancel holding it would wait for a drainer waiting for Cancel. Two rules make that unrepresentable: the drainer takes the lease with a non-blocking try and discards the event when it fails — a held lease during TRANSFERRING can only be a teardown, which already owns the outcome — and the terminal path unwinds without joining the drainer, letting the loop end on its own when `Stop` closes the lane.

**Why the lease also guards publication.** `transfer-started` publishes under the lease at the TRANSFERRING commit. If progress published freely, a snapshot could reach the UI before the started event for the transfer it belongs to — the same defect class as reset-before-started. Making the lease the emission right fixes that order by construction, and the only cost is dropped progress, which the contract already declares droppable.

**Cancel joins; it never cleans up twice.** Waiting on the lease *is* the join: whoever holds it is the one cleanup in flight. That is also what closes Story 1.5's CLAIMING wedge — a claim that loses its post-beacon revalidation leaves the state at CLAIMING and releases the lease, and the Cancel that caused the loss then drives it to IDLE.

**Every wait is deliberate.** `<-drainerDone`, `StopBeacon`, and `ServerPort.Stop` are unbounded on purpose: their ports guarantee quiescence on every return, and a watchdog would let Cancel report success while a listener is still live. That reliance is load-bearing, so it is stated rather than assumed.

## Verification

**Commands:**
- `go build ./... && go vet ./... && go test -count=1 ./...` — module and deterministic tests pass.
- `CGO_ENABLED=1 go test -race -count=1 ./...` — no state, sequence, or ownership race. Needs MinGW `bin` on PATH (see AGENTS.md).
- `go test -race -count=20 ./internal/transfer` — the drainer, timer, and Cancel races are concurrency tests; repeat them.
- `go test -run 'Cancel|Shutdown|Terminal|Reset|Progress' -timeout 60s ./internal/transfer` — a lifecycle deadlock surfaces as a timeout, so this must never need the default ten minutes.
- `go mod tidy && go mod verify` — this story adds no dependency.
- `gofmt -l .` and `git diff --check` — no formatting or whitespace defects.
- `rg -n 'wails|net/http' internal/transfer --glob '!**/*_test.go'` — no output: the coordinator stays framework-independent.
