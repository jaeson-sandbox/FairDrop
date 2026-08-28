---
title: 'Story 1.6 — Complete, Cancel, and Reset the Transfer Lifecycle'
type: 'feature'
created: '2026-08-27'
status: 'done'
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

- **Review round 1 (2026-08-27, patches only — no loopback):** The three step-04 review layers all terminated on an API rate limit before producing findings, so their exact child prompts were written to `review-layer-prompts-1-6.md` for an out-of-session run, and the review was carried by twenty-four independent mutations instead. Eighteen were caught, naming the test that failed. The six survivors split three ways. **Three were real verification gaps and are now closed:** the production `AfterFunc` default that `NewCoordinator` installs was never executed once — every test injected the seam, so a default that scheduled nothing would have parked every terminal session in DONE forever with the suite green; `Shutdown` with nothing staged never proved the operation lease was free, which matters because `retire` clears the session several lines before it hands the lease back, so a Shutdown in that window would report everything gone while a reset was still being published; and `Cancel` was never shown to cancel the data-plane *context* rather than only the generation marker, because every fake returns immediately and the two are indistinguishable without a step that will not finish until its context is cancelled. **Two were redundant guards** — `session.terminal`, already self-reported in `deferred-work.md`, and `retire`'s STAGING check, whose comment claimed to be the mechanism keeping Cancel-from-STAGING silent when the live mechanism is the `c.session != live` return above it; the comment was corrected rather than the guard removed. **One was a weak mutation** that added a redundant write without breaking a guarantee, and proves nothing either way. **KEEP:** the two non-blocking lease acquisitions in the drainer and the `releaseAcquired`/`joinDrainer` split — mutating any of the three deadlocks the suite outright, which is the evidence that the central design constraint is enforced rather than described; `publish` panicking without the lease; `assertEventGrammar`, which pins seq-from-one, no-gap, one-terminal and the contract's per-event payload table on every collected stream; and the 3-second reset asserted against a literal at the assertion site rather than against `resetDelay`. **One flaky test was found and fixed:** `TestASecondTerminalEventIsDiscarded` published both terminal events up front under a comment asserting "both are queued before the drainer can take either", which nothing synchronised -- roughly one suite run in forty, the drainer took the first, tore down, and closed the lane before the second was offered, failing with "the second terminal event was not queued". The second outcome is now queued from inside the first one's teardown, the one deterministic moment when an outcome has been accepted and the lane is still open, and `fakeServer.Stop` closes the lane last to match the real port's teardown order. Sixty consecutive suite runs are clean.

- **Review round 2 (2026-08-28, patches only — no loopback):** The three review layers ran on a retry and returned findings; nothing required renegotiating the frozen block. **Five demonstrated verification gaps, each now killed by mutation.** The reset timer's contract row names DONE *and* ERROR, but every timer-firing test reached DONE, so narrowing `fireReset` to `stateDone` alone stayed green while a failed transfer parked in ERROR forever and every later Stage was refused as `busy`. `Stage` during the three-second terminal lease was never exercised, and that lease deliberately leaves the operation lease free, so the busy guard is the only thing holding it — widening it installed a replacement session whose predecessor's reset then failed its id check and never reached the UI. Quiesce-before-announce was asserted only by `teardownCalls`, which filters the log down to release calls and so can show *that* Stop ran but never that it ran first; moving the release after the publications was invisible. Terminal snapshots were never fed an out-of-range value, so `terminalSnapshot`'s clamp could be dropped entirely — and a terminal event is not coalescable, so a snapshot that fails JSON marshalling costs the UI the outcome permanently. `harness.close` returned early whenever the session was nil, which is the state after every reset and every Cancel, so a drainer belonging to a cleared session was never joined at teardown. **Six production fixes.** `terminalPublicError` became an allow-list — it rewrote only `nil` and `cancelled`, so a `ServerFailed` carrying `beacon_warning` would have published "the QR code and download link still work" as a transfer's terminal error, and `busy`, `shutting_down`, `invalid_selection` and `qr_failed` were equally reachable and equally wrong. `acceptTerminal` and `fireReset` now re-check `closing` after taking the lease, because Shutdown raises that flag and then waits for the lease the outcome already holds — probe-verified publishing `transfer-progress` and `transfer-complete` into a UI Shutdown had declared gone. `sanitizeProgress` now enforces the whole progress contract rather than only its floats. `fireReset` cancels the session context *before* handing the lease back, as `retire` already did, so a Cancel released by that hand-back cannot return "everything is gone" while the context is still live. `Cancel` from IDLE now waits for the lease for the same reason Shutdown does. An unrecognized `ServerEventKind` and a `ServerProgress` with no snapshot are still discarded, but now leave a diagnostic instead of vanishing — the second would have panicked the drainer, where nothing recovers. `Cancel` and `Shutdown` no longer panic on a nil receiver, and a scheduler seam that withholds its stop function is recorded rather than called. **Three pieces of dead or dishonest code removed:** `retire`'s unreachable STAGING condition (the `c.session != live` return above it is the live mechanism), `cancelSession`, which had no production caller and now lives in the test helpers, and `publish`'s comment, which claimed to prove the caller holds the lease when `len(c.lease) == 0` only proves somebody does. **KEEP:** `assertEventGrammar` now also pins position — started first, reset last, nothing after a terminal but reset — and its one self-referential call site, which took the expected session id from the stream under test, now uses the independent `testSessionID`.

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

## Suggested Review Order

**The deadlock the design exists to avoid**

- Start here: the two rules that make terminal handling on the drainer safe.
  [`outcomes.go:25`](../../internal/transfer/outcomes.go#L25)

- Non-blocking, always. A held lease means a teardown already owns this outcome.
  [`outcomes.go:109`](../../internal/transfer/outcomes.go#L109)

- Release without joining: this code *is* the drainer it would wait for.
  [`coordinator.go:535`](../../internal/transfer/coordinator.go#L535)

- The join every other path owes, and why it stays unbounded.
  [`coordinator.go:559`](../../internal/transfer/coordinator.go#L559)

**Emission order by construction**

- Only the lease holder publishes, and the comment says what it really checks.
  [`coordinator.go:712`](../../internal/transfer/coordinator.go#L712)

- A dropped snapshot consumes no sequence number, so a gap cannot occur.
  [`outcomes.go:66`](../../internal/transfer/outcomes.go#L66)

- Sequence assignment sits next to the publication it belongs to.
  [`outcomes.go:210`](../../internal/transfer/outcomes.go#L210)

**Quiesce, then announce**

- Resources go first; Shutdown is re-checked after, because it waits on this lease.
  [`outcomes.go:109`](../../internal/transfer/outcomes.go#L109)

- An allow-list: only codes describing a transfer that began and then failed.
  [`outcomes.go:300`](../../internal/transfer/outcomes.go#L300)

- The boundary that cannot afford to trust its adapter, now for every field.
  [`outcomes.go:329`](../../internal/transfer/outcomes.go#L329)

**Cancel joins; it never cleans up twice**

- Waiting on the lease *is* the join, from IDLE as much as anywhere else.
  [`lifecycle.go:23`](../../internal/transfer/lifecycle.go#L23)

- One retire drives every marked session to IDLE; only Shutdown stays silent.
  [`lifecycle.go:95`](../../internal/transfer/lifecycle.go#L95)

- A timer cannot be un-fired, only outrun, so it revalidates on the far side.
  [`lifecycle.go:144`](../../internal/transfer/lifecycle.go#L144)

- The lease being free is Shutdown's proof that nothing is still running.
  [`lifecycle.go:61`](../../internal/transfer/lifecycle.go#L61)

**Contract surface**

- The three-second lease, scheduled through a seam so it is deterministic.
  [`lifecycle.go:10`](../../internal/transfer/lifecycle.go#L10)

- The timer seam, and the one property the coordinator depends on.
  [`coordinator.go:183`](../../internal/transfer/coordinator.go#L183)

- Naming no state refuses everything; a zero-value skip is how an id became a wildcard.
  [`coordinator.go:605`](../../internal/transfer/coordinator.go#L605)

**Evidence the rules are executable**

- Position as well as payload: started first, reset last, nothing after a terminal.
  [`coordinator_outcomes_test.go:502`](../../internal/transfer/coordinator_outcomes_test.go#L502)

- Ordering, not presence: Stop must precede the publication that announces it.
  [`coordinator_outcomes_test.go:143`](../../internal/transfer/coordinator_outcomes_test.go#L143)

- A held lease discards the outcome rather than waiting -- the deadlock, forced.
  [`coordinator_outcomes_test.go:572`](../../internal/transfer/coordinator_outcomes_test.go#L572)

- Both terminal states fire their reset; only DONE used to be driven.
  [`coordinator_lifecycle_test.go:878`](../../internal/transfer/coordinator_lifecycle_test.go#L878)

- Shutdown suppresses an outcome that already owns the lease it waits for.
  [`coordinator_lifecycle_test.go:947`](../../internal/transfer/coordinator_lifecycle_test.go#L947)

- The busy guard is all that holds the terminal lease, since the lease is free.
  [`coordinator_stage_test.go:854`](../../internal/transfer/coordinator_stage_test.go#L854)

- The production default nothing else executes, which Story 1.7 will wire.
  [`coordinator_lifecycle_test.go:730`](../../internal/transfer/coordinator_lifecycle_test.go#L730)

- Cancel walks all seven rows of the command table.
  [`coordinator_lifecycle_test.go:16`](../../internal/transfer/coordinator_lifecycle_test.go#L16)

- Every adapter passes this gate; it is what keeps the mutex rule from regressing.
  [`helpers_test.go:188`](../../internal/transfer/helpers_test.go#L188)
