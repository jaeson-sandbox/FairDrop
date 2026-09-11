# Evidence: Story 3.4: Bound Every Lifecycle Wait and Prove Quiescence

## Implementation Evidence

Transcript below is from the working tree at baseline commit `68db281f82840a98f48c5b29c0009780c9287e90`
plus this story's changes (uncommitted at the time of this write-up, per the working convention of
leaving evidence beside the diff it describes).

```
$ gofmt -l .
$ go vet ./...
$ go tool staticcheck ./...
$ go test -count=1 ./...
ok  	fairdrop	0.137s
ok  	fairdrop/internal/network	0.218s
ok  	fairdrop/internal/qr	0.199s
ok  	fairdrop/internal/server	3.388s
ok  	fairdrop/internal/source	0.282s
ok  	fairdrop/internal/stream	0.714s
ok  	fairdrop/internal/transfer	0.487s
$ CGO_ENABLED=1 go test -count=1 -race ./...
ok  	fairdrop	1.751s
ok  	fairdrop/internal/network	1.156s
ok  	fairdrop/internal/qr	1.420s
ok  	fairdrop/internal/server	4.481s
ok  	fairdrop/internal/source	1.292s
ok  	fairdrop/internal/stream	5.030s
ok  	fairdrop/internal/transfer	1.478s
$ GOOS=darwin GOARCH=arm64 go build ./...
$ GOOS=darwin GOARCH=arm64 go vet ./...
$ GOOS=darwin GOARCH=arm64 staticcheck ./...
$ GOOS=linux GOARCH=amd64 go build ./...
$ GOOS=linux GOARCH=amd64 go vet ./...
$ cd frontend && npx vitest run
 Test Files  17 passed (17)
      Tests  490 passed (490)
$ cd .. && wails build
Built 'C:\Users\jaeso\Coding\FairDrop\build\bin\fairdrop.exe' in 6.382s.
$ git status --porcelain   # after wails build: no bindings drift, app.go's public API is unchanged
$ go test -count=1 -timeout 120s ./internal/transfer/ ./internal/server/
ok  	fairdrop/internal/transfer	0.481s
ok  	fairdrop/internal/server	1.576s
```

`go test -count=1 -timeout 120s` was used throughout implementation for every run touching
`internal/transfer` or `internal/server`, per this story's own caution: a deadlock introduced while
writing the bounds would otherwise surface as an apparently-slow pass rather than a named failure.
It never fired against the finished code; it did fire, correctly, against every mutation below.

## What changed

- `internal/server/lifecycle.go`: `Stop()` now releases `s.mu` immediately after taking ownership of
  `s.active`, never holding it across the teardown wait. `run.teardown()`'s three waits (`<-r.serveDone`,
  `r.handlers.Wait()`, `r.awaitConnections()`) are bounded by a new `awaitQuiescence`, which converts
  the two non-channel waits to `doneChannel`-wrapped goroutines and selects across all three plus a
  `time.Timer` seeded from a new `serverTimeouts.teardown` seam (production default `teardownBound =
  10s`, declared beside the existing three net/http deadlines). A bound hit returns a coded
  `transfer_failed` naming exactly which of the accept loop / a request handler / a connection is
  still outstanding; the event lane is still closed either way, because `eventLane.close()` is always
  safe against a racing producer.
- `internal/transfer/lifecycle.go`: `Cancel` and `Shutdown` now take a `context.Context`. `awaitLease`
  became `awaitLeaseBounded`, which fast-paths a lease that is already free (no timer armed, so a
  healthy Cancel/Shutdown's call log is unchanged) and otherwise selects on the lease, the caller's
  `ctx.Done()`, and a new bound timer. `retire` returns the first bound failure `unwind` reports, so
  Cancel/Shutdown can surface it.
- `internal/transfer/coordinator.go`: a new `boundTimer` seam on `Dependencies`/`Coordinator`, distinct
  from `afterFunc` (the reset scheduler), drives every quiescence-wait bound. `callBounded` runs one
  adapter call on its own goroutine and races it against the bound (arming the timer *before* launching
  the goroutine, so its position in a test's call log is deterministic rather than a scheduling race).
  `stopServerBounded`/`stopBeaconBounded` wrap `ServerPort.Stop`/`NetworkPort.StopBeacon`;
  `awaitBounded`/`joinDrainerBounded` wrap the drainer join. `releaseAcquired`/`unwind` now return the
  first bound failure instead of nothing. `AuthorizeClaim`'s synchronous `StopBeacon` call is bounded
  (D-024). A test-only `afterArm` hook runs inside `armReset` between arming the reset timer and
  re-checking the session, forcing D-037's window deterministically.
- `internal/transfer/ports.go`: `NetworkPort`'s doc states the `StartBeacon` precondition; `ServerPort`'s
  doc states the bounded-Stop and restart contract.
- `app.go`: `CancelTransfer` hands `Cancel` the stored application-lifetime context; `shutdown` hands
  `Shutdown` its own Wails-supplied context (rather than discarding it) and logs `fairdrop: shutdown
  begin` on entry (D-087).
- `docs/fairdrop-contracts.md`, `docs/fairdrop-architecture.md`, `AGENTS.md`: amended postconditions and
  the decision behind them (see each file's diff for exact text).
- `_bmad-output/implementation-artifacts/deferred-work.md`: all eleven ids this story's `epics.md`
  Closes line names (D-017, D-019, D-022, D-024, D-027, D-030, D-032, D-036, D-037, D-087, D-090) are
  now `owner: discharged`, with a banner explaining what closed each.
- No new public error code: every new failure is expressed through the existing `transfer_failed`
  carrier, per the frozen-twelve Ask First constraint.

New tests, every one executed and passing (plus 20 consecutive `-race` iterations for the five
`internal/transfer` ones, and inclusion in the whole-package `-race` run above for the three
`internal/server` ones):

- `internal/server/lifecycle_test.go`:
  - `TestStopReturnsACodedFailureWhenAHandlerNeverReturns` (D-017) -- a payload whose `WriteTo` blocks
    forever, ignoring both its context and the destination (so a forced connection close cannot reach
    it); `Stop()` returns a coded failure naming "a request handler" within the shrunk bound, and a
    subsequent `Start()` succeeds.
  - `TestStopReleasesItsMutexBeforeWaitingSoAConcurrentStartIsNeverBlocked` (D-017, the mutex half) --
    races a `Start()` against a `Stop()` that is still genuinely in flight (a 3-second teardown bound),
    proving `s.mu` is released before the wait rather than merely by the time `Stop()` eventually
    returns, which every implementation satisfies trivially and therefore proves nothing on its own.
  - `TestStopBoundsAHandlerStuckInAuthorizeClaim` (D-019) -- the coordinator's `AuthorizeClaim` never
    returns; from `internal/server`'s point of view this is indistinguishable from any other stuck
    handler, and the same bound covers it.
  - `TestStartAfterStopBuildsAFreshRun` (D-022) -- `Start` -> `Stop` -> `Start` serves a second transfer
    successfully against the same fixed token, proving the restart contract rather than merely
    compiling.
- `internal/network/beacon_test.go`: `TestStartBeaconRequiresSelectionAndLiveContext` already pinned the
  real adapter's refusal before this story (D-030's "internal/network enforces it" half); unchanged.
- `internal/transfer/coordinator_lifecycle_test.go` (all forced through the `bounds`/`afterArm` seams,
  never by sleeping):
  - `TestCancelReportsACodedFailureWhenTheDrainerNeverEnds` (D-027, D-032, D-090) -- `ServerPort.Stop`
    returns but leaves the event lane open; the drainer join's own bound fires and Cancel reports a
    coded failure while still reaching `IDLE` with the lease freed.
  - `TestAuthorizeClaimCommitsWhenStopBeaconNeverReturns` (D-024) -- `NetworkPort.StopBeacon` never
    returns during the claim handshake; authorization still commits on the bound, with a diagnostic.
  - `TestCancelHonoursAnAbandonedCallerContext` (D-036) -- Cancel genuinely waiting for a lease someone
    else holds returns promptly, with a coded failure, the instant the caller's own context is
    cancelled, rather than waiting out the full lease bound.
  - `TestShutdownReportsACodedFailureWhenTheLeaseNeverFrees` (D-032, D-036) -- the same lease wait,
    forced through its fixed bound instead of a context, via Shutdown; also proves Shutdown never
    "releases" a lease it did not acquire.
  - `TestCancelInsideArmResetsWindowLeavesExactlyOneTimerStoppedNotLeaked` (D-037) -- the `afterArm`
    hook parks the drainer goroutine inside `armReset`'s window while a concurrent Cancel marks the
    session cancelled; exactly one timer is armed and stopped.
- `app_test.go`: `TestCancelTransferDelegatesAndReturnsQuietly` and
  `TestShutdownDelegatesAndBlocksUntilQuiescent` extended to assert the exact context each hands the
  coordinator (the stored application-lifetime one for Cancel, the hook's own for Shutdown).

All existing coordinator, server, and app tests stayed green with only the following adjustments, each
because a healthy bounded wait now legitimately touches a seam these assertions did not previously
know about:

- `TestAuthorizeClaimCommitsAndPublishesStarted`, `TestDirectoryStageFailureKeepsReverseUnwind`: exact
  call-sequence assertions updated for the deterministic `timer.AfterFunc` entry `callBounded` now logs
  before the adapter call it bounds (armed before the call is launched, specifically so this ordering
  is deterministic rather than a scheduling race -- see Mutation Table row 8's discussion in the code
  comment on `callBounded`), and, for the directory test, a new `withoutBoundTimerCalls` filter for the
  one entry (the drainer-join bound) whose presence in the log *is* a genuine scheduling accident.
- `TestCancelFromIdleWaitsForAFinishingTeardown`: its "no adapter at all" assertion now excludes the
  bounds seam's own arm-then-stop entry (a legitimate part of waiting for an already-held lease, not an
  adapter reached), the same way it already excluded `entropy.Read`.

## Mutation Table

Each deliberate break below was made against the real production file, run against the named test(s)
with `go test -count=1 -timeout <short> -run <name> -v`, observed to fail (or, for row 1, observed to
hang until `-timeout` killed it with a named goroutine dump), then reverted from a pre-edit backup copy
and reconfirmed green (`go build ./...` and the package's full `go test`).

| # | Deliberate break | Named failing test(s) | Observed failure |
| --- | --- | --- | --- |
| 1 | Delete the `case <-bound.C` branch from `run.awaitQuiescence` (`internal/server/lifecycle.go`), leaving the three waits unbounded | `TestStopReturnsACodedFailureWhenAHandlerNeverReturns` | No named assertion failure -- the test itself hung. `go test -timeout 15s` killed the process and printed a full goroutine dump naming the test and the exact stuck goroutines (`sync.WaitGroup.Wait` in `doneChannel`, `sync.Cond.Wait` in `awaitConnections`, the handler parked in the fixture's `WriteTo`), proving the acceptance criterion "a named test fails rather than the suite hanging" the hard way: by removing the guard and confirming a silent hang is exactly what disappears without it. |
| 2 | Revert `Server.Stop` to hold `s.mu` across `active.teardown()` (`defer s.mu.Unlock()` before the wait, instead of unlocking after clearing `s.active`) | `TestStopReleasesItsMutexBeforeWaitingSoAConcurrentStartIsNeverBlocked` | `Start() did not proceed while a prior Stop was still in flight -- s.mu is being held across the bounded wait instead of being released before it` |
| 3 | Revert `joinDrainerBounded` to the old unconditional `<-live.drainerDone` (`internal/transfer/coordinator.go`) | `TestCancelReportsACodedFailureWhenTheDrainerNeverEnds` | `no bound was ever left pending` (the test's own `awaitBoundsPending` helper timed out first, itself bounded, so the whole run still finished in ~2s rather than hanging -- the harness's unconditional `h.server.closeEvents()` in cleanup, which exists independently of the code under test, is what let the leaked goroutine finish and the process exit) |
| 4 | Remove the `ctx.Done()` case from `awaitLeaseBounded`'s select (`internal/transfer/lifecycle.go`) | `TestCancelHonoursAnAbandonedCallerContext` | `Cancel did not honour the cancelled context; it waited for the full bound instead` |
| 5 | Revert `AuthorizeClaim`'s beacon stop to the unbounded `c.network.StopBeacon()` call (`internal/transfer/coordinator.go`) | `TestAuthorizeClaimCommitsWhenStopBeaconNeverReturns` | `no bound was ever left pending` (same shape as row 3: the test's own bounded wait for a pending timer is what fails by name, not a hang) |
| 6 | Replace `armReset`'s second `resetIsDueLocked` re-check with an unconditional `live.stopReset = stop` (`internal/transfer/outcomes.go`) -- the exact mutation D-037's own evidence text names | `TestCancelInsideArmResetsWindowLeavesExactlyOneTimerStoppedNotLeaked` | `armReset never stopped the timer it armed for an already-cancelled session` |
| 7 | Call `NetworkPort.StartBeacon` directly on a fresh harness's fake, with no prior `GetLocalIP` (scratch test, not committed) | n/a (throwaway) | `fakeNetwork.StartBeacon` correctly flags it: `StartBeacon was called before a successful GetLocalIP selected an address` -- confirms the D-030 regression guard added to `helpers_test.go` actually fires; reordering Stage's own two production steps to reproduce this end-to-end was judged too invasive to revert safely for the value it would add on top of this direct check plus `internal/network`'s own `TestStartBeaconRequiresSelectionAndLiveContext`, and was not attempted. |
| 8 | (Reasoning check, not executed as a code mutation) Would arming `callBounded`'s timer *after* launching the adapter-call goroutine, instead of before, make a test's call-log order nondeterministic? | `TestAuthorizeClaimCommitsAndPublishesStarted` | Confirmed empirically during implementation: the after-launch ordering produced `[timer.AfterFunc network.StopBeacon ...]` on one run and would race to `[network.StopBeacon timer.AfterFunc ...]` on another, since the two run on different goroutines with no ordering guarantee between them. Reordered to arm-before-launch (see the comment on `callBounded`), which fixed the test deterministically rather than merely by chance, and is recorded here as a mutation-shaped finding even though it surfaced as an implementation bug rather than a deliberate revert-and-confirm break. |

No survivors: every mutation attempted against the finished code produced either a named test failure
or (row 1) a named, killed hang -- never a silent pass and never an unbounded hang that outlived
`-timeout`.

## Bound-value reasoning

Every bound is a seam with a real default (`internal/server/lifecycle.go`'s `serverTimeouts.teardown`,
defaulting to `teardownBound`; `internal/transfer/coordinator.go`'s `adapterCallBound`,
`drainerJoinBound`, `leaseBound`), never a literal sprinkled at each call site, and every test above
forces its bound through the injected seam (`server.timeouts = serverTimeouts{...}` with a shrunk
`teardown`, or `h.bounds.fire()`/`h.timer` for the coordinator) rather than sleeping out a real
duration.

- **`internal/server` `teardownBound = 10s`.** This bounds the *teardown* wait only -- the accept loop
  returning, a handler returning, a connection closing -- which begins after `Stop` cancels the
  data-plane context and force-closes the destination. `TestStopUnblocksAStalledPayload` (pre-existing)
  already proves that a payload ignoring cancellation and writing forever is unblocked by the forced
  close within, in practice, well under a second; its own 60-second assertion ceiling is generous
  slack for a slow CI host, not the expected duration. Ten seconds is headroom above that millisecond
  reality for the one case a forced destination close cannot reach at all (a stuck *source* read), not
  a ceiling any real transfer's own bytes could approach -- the transfer's data plane is already over
  or being abandoned by the time teardown even starts.
- **`internal/transfer` `adapterCallBound = 10s`.** Bounds one external call the operation lease holds
  across: `ServerPort.Stop` (which is itself now internally bounded to 10s, so this is an outer,
  independent backstop against an adapter that is not the one shipped here) and
  `NetworkPort.StopBeacon` (a single mDNS unregister, expected to complete in milliseconds on a healthy
  host). Matches the server-side bound so the two layers reason about "a stuck adapter call" the same
  way.
- **`internal/transfer` `drainerJoinBound = 10s`.** Bounds the wait for the drainer goroutine to notice
  the event channel has closed and exit -- a channel receive plus a function return, expected in
  microseconds once `ServerPort.Stop` has actually closed the lane. Ten seconds gives a slow host the
  same headroom as every other bound rather than inventing a different number with no comparative
  reasoning behind it.
- **`internal/transfer` `leaseBound = 45s`.** Bounds how long `Cancel`/`Shutdown` wait to *join* a
  teardown some other operation already owns -- the worst case being that operation running
  `stopBeaconBounded` (up to `adapterCallBound`), then `stopServerBounded` (up to `adapterCallBound`
  again, itself possibly absorbing the full `internal/server` `teardownBound` inside a hung real
  `Server.Stop`), then `joinDrainerBounded` (up to `drainerJoinBound`), sequentially. `10 + 10 + 10 =
  30s` is the sum of those three top-level bounds; `45s` leaves comfortable margin above that sum for a
  slow host without approaching a duration a real transfer's setup or teardown could ever need, since
  none of this waits on the transfer's own bytes -- only on adapters returning from calls that are
  supposed to be near-instant.

None of these bounds can be hit by a real Wi-Fi transfer, because none of them wait on the transfer's
data plane: they all begin only once `Stop`, `Cancel`, or `Shutdown` has already been called, at which
point the transfer is already complete, cancelled, or failed.

## Matrix Audit

Every I/O & Edge-Case Matrix row maps to an executed test:

| Row | Test |
| --- | --- |
| Healthy cancel | Every existing `TestCancelFromEveryState` case, unchanged; `TestCancelSucceedsThroughACleanupDiagnostic` |
| Drainer never ends | `TestCancelReportsACodedFailureWhenTheDrainerNeverEnds` |
| `StopBeacon` hangs | `TestAuthorizeClaimCommitsWhenStopBeaconNeverReturns` (claim path, D-024); `internal/server`'s bound covers the teardown-path `StopBeacon` call the same way it covers `ServerPort.Stop` itself, since both run through the same `stopBeaconBounded`/`stopServerBounded` machinery inside `releaseAcquired` |
| Handler stuck in `WriteTo` | `TestStopReturnsACodedFailureWhenAHandlerNeverReturns`, `TestStopReleasesItsMutexBeforeWaitingSoAConcurrentStartIsNeverBlocked` |
| Cancel abandoned | `TestCancelHonoursAnAbandonedCallerContext` |
| Restart | `TestStartAfterStopBuildsAFreshRun` |
| Beacon before address | Regression-guarded by `fakeNetwork.StartBeacon`'s ordering assertion (every existing Stage test now exercises it); real refusal pinned pre-existing by `internal/network`'s `TestStartBeaconRequiresSelectionAndLiveContext` |
| Cancel inside `armReset` | `TestCancelInsideArmResetsWindowLeavesExactlyOneTimerStoppedNotLeaked` |

## Notes

- **The lease-holding-a-bound-error's exact worst case is not independently tested end-to-end.**
  `leaseBound`'s reasoning (30s worst case, 45s bound) is derived arithmetically from the three
  documented sub-bounds rather than proven by a single test that forces all three to their own bounds
  simultaneously and measures the total. Doing so deterministically would require composing three
  separate `bounds.fire()` calls in sequence inside one still-in-flight operation, which the current
  `fakeTimer` (fires only the most-recently-armed call) supports in principle but was judged not worth
  the added test complexity given each sub-bound is independently proven and the arithmetic composing
  them is straightforward addition with margin.
- **D-090's "either the port's postcondition is enough and the story records that reasoning, or the
  waits need the documented bound" is resolved in favor of the bound**, not the postcondition alone:
  `stopServerBounded` and `joinDrainerBounded` do not merely trust that `ServerPort.Stop` closes its
  event channel -- they independently bound the wait for that closure, so a `ServerPort.Stop` that
  returns without closing the lane (exactly the scenario `TestCancelReportsACodedFailureWhenTheDrainerNeverEnds`
  drives) is survived by the coordinator regardless of which adapter implements the port.
- Sprint status and the spec's frontmatter are updated to `done` only after this evidence file, the
  full verification transcript above, and the deferred-work closures were all in place.

## Orchestrator mutation pass and the verification-gap layer

Ten mutations run against the implementation before accepting it. Six died immediately; four survived
and every one was a real gap, three of them named first by the verification-gap review layer.

| # | Mutation | Result |
|---|---|---|
| M59 | `awaitBounded` claims success after timing out | killed |
| M60 | `callBounded` claims the adapter returned cleanly after timing out | killed |
| M61 | `joinDrainerBounded` swallows its own timeout | killed |
| M63 | `Server.Stop` holds the mutex across the whole teardown again | **killed by hang** -- caught by the explicit `-timeout`, which for this story is the only honest way to catch it |
| M65 | `armReset` stops re-checking that the session survived | killed |
| M58 | Stage advertises the beacon before an address is selected | killed -- three tests, with the exact precondition message |
| M67 | `stopServerBounded` swallows its own timeout | **survived**, then fixed |
| M68 | `leaseBound` cut from 45s to 45ms | **survived**, then fixed |
| M69 | `teardownBound` cut from 10s to 500ms | **survived**, then fixed |
| M70 | the shutdown-begin log line deleted | **survived**, then fixed |

**M67 is the one worth dwelling on**, because closing it took three attempts and the first two were
wrong in instructive ways. The sibling bound on `StopBeacon` was tested; the bound on `ServerPort.Stop`
was not, so replacing that whole branch with `return nil` left the package green -- the exact case
D-017 named. The first test fired the bound once and failed, because a stuck Stop *cascades*: the call
bound elapses, and then the drainer join has its own bound, since the drainer only ends when Stop
closes the lane. The second version drove every bound in the cascade and passed -- and still did not
kill the mutation, because Cancel was reporting the *drainer's* failure, not the server's. Only when
the fake closes the lane first and then hangs does the test isolate this branch, and the assertion now
names the failure rather than accepting any coded error, since both waits carry the same public code.

**M68 and M69 are the same vacuity in the configuration.** Every bounded-wait test drives its timeout
through an injected seam that never sleeps, which is the right way to test the mechanism and is
completely blind to the duration. Cutting `leaseBound` from 45 seconds to 45 milliseconds -- long
enough to make a healthy teardown on a loaded machine report failure -- left every one of those tests
green. The real values are now pinned, along with the arithmetic `leaseBound`'s comment claims: that it
outlasts the sequence of sub-bounds it covers.

**One assertion the orchestrator wrote and then withdrew.** A first version also required
`teardownBound` to outlast `readTimeout`. It failed immediately, and the rule was what was wrong:
`readTimeout` bounds reading a request, while teardown cancels the data-plane context and force-closes
the destination *before* it waits at all, so a request still being read is already broken by the time
that bound starts counting. The two govern different phases. The reason is recorded in the test file
so the next person does not re-derive the same wrong rule.

**Left open, recorded rather than fixed.** `teardownTimeoutError` names three independently outstanding
waits -- the accept loop, a request handler, a tracked connection -- and only the handler branch is
driven by a test. `assertQuiescent` checks the other two on the healthy path, so they are not
unverified, but no test isolates a timeout in which only the accept loop or only a connection is still
outstanding.

## The adversarial layer, and the two rule violations it found

The Blind Hunter layer returned twelve findings. Two of them were this story's own governing rule --
"a bound that elapses never reports the resource gone" -- broken in places the rule's author did not
look, and both are fixed here. The other ten are recorded as `D-096` through `D-102`.

**A repeated `Stop` upgraded an unresolved teardown to success.** `Stop` is documented idempotent, and
this story made it detach the run *before* waiting so a bounded teardown could not deadlock a later
`Start`. Those two correct facts combined into a wrong one: the second call found nothing attached and
answered a bare `nil` for a teardown whose first call had just reported that quiescence was unproven.
Any caller repeating `Stop` to re-confirm got a false all-clear. The server now remembers an
unresolved run and repeats its answer.

**The claim booked the beacon as released when its bound had elapsed.** `AuthorizeClaim` must commit
even when `StopBeacon` hangs -- that is D-024, and blocking the claim is the worse failure -- but it
marked the resource released on the way past. Teardown walks the acquired list, so a beacon marked
released is one no later cleanup revisits: the advertisement could outlive the session with a
diagnostic as the only trace. The release is now conditional on the adapter actually confirming, and
a test proves teardown makes a second attempt.

Three mutations against those two fixes, all killed: answering `nil` again from the idempotence
clause, forgetting the unresolved run, and booking the beacon released unconditionally.

**What was deferred rather than fixed, and why.** Four of the ten are about the same root cause the
layer found one level below this story: `network.Manager.StopBeacon` holds the shared selection gate
across its own blocking `Shutdown`, so a hung mDNS shutdown degrades every later transfer in the
process even though the coordinator itself now recovers (`D-096`). `internal/server` got exactly that
treatment here -- detach, release, then wait -- and `internal/network` was outside this story's Code
Map. Two more are the honesty mechanism being unreachable: the context production hands `Cancel` and
`Shutdown` can never be cancelled, because Wails builds it from `Background` plus `WithValue` and
never wraps it (`D-097`), and every bound-timeout diagnostic goes to a sink `app.go` never reads
(`D-098`). Both mean a feature this story delivered is real in tests and inert in the shipped binary,
which is worth stating plainly rather than filing quietly.

The remaining three are smaller: the coordinator's bound on `Stop` equals the server's own, so the
outer wait can preempt the inner one's more precise message (`D-099`, which the orchestrator had
flagged independently); `unwind` reports only the first of two simultaneous bound failures (`D-100`);
and `callBounded` abandons one goroutine per timed-out call with no cap (`D-101`). Plus `D-102`, from
the verification-gap layer: only one of the teardown report's three named waits is driven by a test.

