---
title: 'Story 3.4: Bound Every Lifecycle Wait and Prove Quiescence'
type: 'feature'
created: '2026-09-10'
status: 'done'
review_loop_iteration: 0
baseline_commit: '68db281f82840a98f48c5b29c0009780c9287e90'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-3-context.md'
  - '{project-root}/docs/fairdrop-contracts.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Every quiescence wait FairDrop performs is unbounded, and each rests on a port postcondition nothing enforces. One adapter that never returns — a source read ignoring its context inside `handlers.Wait()`, an mDNS `StopBeacon` that hangs, a `Stop` that returns without closing its lane — wedges Cancel and Shutdown forever, holding the operation lease, and leaves the window unusable with no way out. Eleven deferred entries describe this from different angles (D-017, D-019, D-022, D-024, D-027, D-030, D-032, D-036, D-037, D-087, D-090).

**Approach:** Every wait gets a documented bound. Reaching that bound is never reported as success: the caller returns a coded failure naming which adapter did not return, and records a diagnostic. The contract is amended to say so, because today it promises quiescence on every return, and that promise is exactly what an unbounded wait was protecting.

## Boundaries & Constraints

**Always:** Every wait that today blocks forever ends on a named, documented bound: the operation lease, the drainer join, `ServerPort.Stop`, `NetworkPort.StopBeacon`, and the server's own handler and connection waits. A wait that hits its bound reports a coded failure and a diagnostic naming the port, and never claims the resource is gone. Bounds are injected seams with real defaults, so a test forces every timeout deterministically rather than sleeping. `Cancel` and `Shutdown` take a context and honour it. `Start` after `Stop` has a stated contract and a test. Every existing lifecycle guarantee that does not concern an unresponsive adapter survives unchanged: one reset per session, the event grammar, no adapter call under the state mutex, no second cleanup.

**Ask First:** Adding or renaming a public error code — the twelve are frozen and Story 3.5 owns their copy, so this story must express every new failure through a code that already exists. Changing what `Cancel` returns on the *healthy* path. Any bound short enough that a real Wi-Fi transfer could hit it.

**Never:** Report success from a wait that timed out. Leave a bound undocumented or unnamed in the contract. Bound a wait by sleeping in a test. Make the drainer wait on the operation lease. Decide anything about lost or malformed *events* — that is Story 3.6.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Healthy cancel | every adapter returns | Unchanged: `Cancel` returns nil once quiescent, one reset, cleanup errors stay diagnostics | N/A |
| Drainer never ends | `Stop` returns but leaves the lane open | Join ends on its bound; coded failure names the lane; coordinator reaches IDLE rather than permanent `busy` | Coded |
| `StopBeacon` hangs | mDNS shutdown never returns | Claim and teardown end on the bound; diagnostic names the beacon; no claim of a stopped advertisement | Coded |
| Handler stuck in `WriteTo` | source read ignores its context | Server teardown ends on its bound; `Stop` returns a coded failure; `s.mu` is released so a later `Start` is not deadlocked | Coded |
| Cancel abandoned | caller's context cancelled mid-wait | Returns promptly with a coded failure; no second cleanup is launched | Coded |
| Restart | `Start`, `Stop`, `Start` | Behaves as the contract now states, proven by test | Per contract |
| Beacon before address | `StartBeacon` with no prior `GetLocalIP` | Refused; the port documents the precondition and the coordinator's fake asserts it | Coded |
| Cancel inside `armReset` | Cancel lands between creating the timer and re-checking | Exactly one timer armed and stopped; forced deterministically, not scheduled | Test |

</frozen-after-approval>

## Code Map

- `internal/transfer/lifecycle.go:23` `Cancel`, `:61` `Shutdown`, `:95` `retire`, `:144` `fireReset`, `:196` `awaitLease` — `awaitLease` is `<-c.lease`, already a channel, so a bounded variant is a `select` with the seam's timer. Both commands gain a context parameter here (D-036); `App.CancelTransfer` and `App.shutdown` in `app.go` are the only callers.
- `internal/transfer/coordinator.go:569` `joinDrainer` — `<-live.drainerDone`, and its comment states the reasoning a bound must now replace rather than contradict: a watchdog "would let Cancel report success while a drainer, and therefore a publication, was still in flight". The resolution the acceptance criteria fix is that a timed-out wait reports failure, so it never reports success (D-027, D-032, D-090).
- `internal/transfer/coordinator.go` `releaseAcquired`/`stopServer` — the two adapter calls the lease holds across; `stopServer` currently records a diagnostic and continues either way.
- `internal/transfer/coordinator.go:448` `AuthorizeClaim` — calls `StopBeacon` synchronously while holding the lease, which is the one unlocked call inside the handshake (D-024).
- `internal/transfer/coordinator.go:204` the `afterFunc` field, `:234` its real default and `internal/transfer/helpers_test.go:715` `fakeTimer` — the established pattern for injecting time; the bounds seam should follow it so `fakeTimer.fire()` drives every timeout with no sleeping.
- `internal/server/lifecycle.go` `Stop` (holds `s.mu` for the whole teardown, which is what also deadlocks a later `Start`, D-017), `run.teardown` (`<-r.serveDone`, `r.handlers.Wait()`, `r.awaitConnections()`), `run.awaitConnections` (a `sync.Cond` loop). `WaitGroup.Wait` and `Cond.Wait` cannot be selected on: convert each to a goroutine closing a channel, then select against the bound. `serverTimeouts`/`defaultTimeouts` in the same file is where a teardown bound belongs beside the existing deadlines.
- `internal/server/lifecycle_test.go` `TestATransferLongerThanEveryTimeoutStillCompletes` in `longevity_test.go` — the test that proves a bound must not cap a real transfer; whatever bound is chosen must keep it green.
- `internal/transfer/ports.go:72` `StopBeacon`, and the `NetworkPort` doc that must state `StartBeacon` requires a prior successful `GetLocalIP` (D-030). `internal/network` already enforces it; the coordinator's fake accepts it at any time, so the fake must assert the ordering or swapping the coordinator's two steps stays green.
- `docs/fairdrop-contracts.md:231` and `:323` — the binding text to amend. Today: "On every return—even with an error—the listener … have ended" and "Because every Stop return is quiescent, cleanup errors are safe diagnostics". Both stay true of an adapter that honours its contract; both need the sentence that says what the coordinator does when one does not. `docs/fairdrop-architecture.md` carries the matching decision.
- `internal/transfer/errors.go:12-23` — the twelve frozen codes. `ErrTransferFailed` and `ErrShuttingDown` already exist and are the honest carriers; no new code (Ask First).
- `main_test.go` `TestEveryOpenDeferredEntryIsCitedByItsOwningStory` — all eleven ids must move to `discharged` or be re-owned with the Closes line updated, or this fails.

## Tasks & Acceptance

**Execution:**
- [x] `internal/server/lifecycle.go` — a teardown bound beside the existing timeouts; the handler and connection waits become selectable; `Stop` releases `s.mu` so a bounded teardown cannot deadlock a later `Start`; the restart contract stated (D-017, D-019, D-022).
- [x] `internal/transfer/coordinator.go`, `lifecycle.go` — bounded lease, drainer join and adapter calls, each with a coded failure and a diagnostic naming the port; `Cancel`/`Shutdown` take a context (D-024, D-027, D-032, D-036, D-087, D-090).
- [x] `internal/transfer/ports.go` — the `StartBeacon` precondition (D-030).
- [x] `app.go` — pass a context from `CancelTransfer` and `shutdown`; a shutdown-entry log line so a relaunch swallowed during shutdown is diagnosable (D-087).
- [x] Tests — one deliberately unresponsive adapter per wait, driven through the timer seam; the `armReset` window forced deterministically (D-037); the beacon-ordering refusal; restart after Stop.
- [x] `docs/fairdrop-contracts.md`, `docs/fairdrop-architecture.md`, `AGENTS.md` — the amended postcondition and the decision behind it.
- [x] `deferred-work.md` — all eleven ids closed or re-owned with `epics.md` updated to match.
- [x] `evidence-3-4-bound-every-lifecycle-wait-and-prove-quiescence.md` — mutation table, gate transcript, and the reasoning for each bound's value.

**Acceptance Criteria:**
- Given an adapter that never returns, for each bounded wait, when the bound elapses, then the caller returns a coded failure naming the port and the coordinator reaches a usable state rather than holding the lease forever.
- Given every bound, when its default is read, then it is documented with why that value cannot be hit by a real transfer, and `TestATransferLongerThanEveryTimeoutStillCompletes` still passes.
- Given a bounded wait that timed out, when its result is inspected, then nothing anywhere reports the resource as quiescent.
- Given each timeout path, when its guard is deliberately removed, then a named test fails rather than the suite hanging.

## Evidence

Mutation tables, gate transcripts and the bound-value reasoning live in
[evidence-3-4-bound-every-lifecycle-wait-and-prove-quiescence.md](evidence-3-4-bound-every-lifecycle-wait-and-prove-quiescence.md), created with the implementation.

## Spec Change Log

- The Code Map's "the bounds seam should follow [`afterFunc`'s] pattern" is implemented as a **second**
  seam (`Dependencies.BoundTimer` / `Coordinator.boundTimer`, driven in tests by a second `*fakeTimer`
  instance, `h.bounds`, distinct from the existing `h.timer`), not a shared use of `afterFunc` itself.
  Reusing `afterFunc` directly would have made every quiescence-wait bound (the lease, the drainer
  join, and the bounded `ServerPort.Stop`/`NetworkPort.StopBeacon` calls) arm and fire through the same
  seam as the three-second terminal reset, which the existing suite asserts exact counts and orderings
  against (`h.timer.armed()`, `h.timer.stops()`, `TestCancelStopsTheArmedReset`, and others). A shared
  seam would have made those assertions either wrong (an unrelated lease wait incrementing "resets
  armed") or unable to distinguish a genuine reset-timer regression from a bounds-seam call. The two
  seams share the same `func(delay, run) StopTimer` shape and the same fake type, so `fakeTimer.fire()`
  still drives every timeout deterministically with no sleeping — just on whichever of the two harness
  fields (`h.timer` for the reset, `h.bounds` for everything this story adds) owns the wait being
  forced. `internal/server`'s bound uses a plain `time.Duration` seam (`serverTimeouts.teardown`)
  instead, following that file's existing pattern (`readHeader`/`read`/`idle`) rather than adopting the
  coordinator's callback-based seam, since `internal/server` has no equivalent to `afterFunc` already
  and a fourth bare duration fits its established shape more directly than introducing a new callback
  seam would.

## Design Notes

The reason these waits were left unbounded is written in the code and is a good reason: a watchdog that lets Cancel return early would let it report success while a listener was still accepting. The acceptance criteria resolve that by separating the two things the old design conflated. Waiting forever and lying are not the only options — a bounded wait that reports failure is honest about exactly what it knows: the adapter did not come back in time, so quiescence is unproven. The contract keeps its promise for adapters that honour theirs, and gains a stated answer for those that do not.

Bounds must be seams for the same reason `afterFunc` is one: a test that proves a timeout by sleeping is slow, flaky, and proves only that the machine was busy.

## Verification

**Commands:**
- `gofmt -l .`; `go vet ./...`; `go tool staticcheck ./...`; `go test -count=1 ./...`; `go test -count=1 -race ./...` — clean
- `GOOS=darwin GOARCH=arm64 go vet ./...` and the linux equivalent, per AGENTS.md's pre-flight
- `cd frontend && npx vitest run`; then, alone, `wails build`
- `go test -count=1 -timeout 120s ./internal/transfer/ ./internal/server/` — a hang is a failure, not a slow pass
