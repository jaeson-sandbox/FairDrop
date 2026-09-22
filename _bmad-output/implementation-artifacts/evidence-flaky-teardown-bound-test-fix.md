# Evidence: fixing the flaky `TestATeardownThatLosesTwoResourcesReportsBoth`

## The CI failure

CI run 35749628271 on `main` at `028fb3e` failed on `verify (windows-latest)` under
`go test -race`:

```
--- FAIL: TestATeardownThatLosesTwoResourcesReportsBoth (10.00s)
    coordinator_lifecycle_test.go:1840: Cancel never returned
```

The failing commit touched zero Go files (docs, frontend, and version strings
only); `internal/transfer` was untouched. 60 consecutive local runs of the test
under `-race` passed, and it had not failed in the previous 12 `main` runs. In
the failing run, `internal/stream` took 241 seconds (normally a few seconds),
indicating the CI runner was severely starved for scheduling time -- this is a
scheduling-timing flake in the test harness, not a product regression.

## Diagnosis: confirmed, and refined

The task's working diagnosis was that `awaitBoundsPending()` returns as soon as
*any* bound is pending (`armed() > stops()`), and `fireIfArmed()` fires the
most-recently-armed bound -- so a test that fires twice could hit the wrong
bound under starvation. Investigation confirmed this and found the precise
mechanism, which is slightly sharper than "any bound pending":

1. `fakeTimer.fire()` marks a bound `fired` but can **never** mark it
   `stopped` -- production resolves a bound by winning the `select` between
   `pending.done` and `timedOut`, and a forced fire deliberately makes the
   `timedOut` branch win instead, so the real `stop()` closure is never called
   for that entry. This means the very first forced `fire()` in a test
   permanently and irrecoverably widens the gap between `h.bounds.armed()` and
   `h.bounds.stops()` for the rest of that test. Every `awaitBoundsPending()`
   call *after* the first forced fire could return `true` immediately,
   regardless of what is actually pending.
2. `callAdapterBounded` (`internal/transfer/bounded.go`) arms a call's bound
   *before* launching the goroutine that makes the call, and the fake port
   (`fakeServer.Stop`, `helpers_test.go`) logs the call the instant it is
   *entered*, not once it returns. So `awaitCalls("server.Stop")` proves the
   bound is armed, but not that it has resolved -- there is a real window
   where `server.Stop`'s own bound is genuinely still pending at the moment
   the test tries to move on to the next (drainer-join) bound in the cascade.

Combined, `TestATeardownThatLosesTwoResourcesReportsBoth`'s second
`awaitBoundsPending(); fire()` pair could land on `ServerPort.Stop`'s own
(interposed, self-resolving) bound instead of the drainer join's. Firing the
server bound instead just produces a timeout diagnostic for a call that would
have succeeded anyway; the drainer join's bound then gets armed afterward but
is never fired, because the test had already spent both of its forced fires --
so `Cancel` hangs until the test's own 10s safety net, producing exactly
"Cancel never returned."

## Deterministic reproduction

Rather than rely on real scheduling starvation (unreproducible locally),
the interleaving was forced directly: a temporary copy of the test hung the
fake `server.stop` hook on a channel so `ServerPort.Stop`'s own bound was
provably still armed-and-pending when the test's second
`awaitCalls("server.Stop"); awaitBoundsPending(); fire()` sequence ran. This
reproduced "Cancel never returned" deterministically, 5/5 runs:

```
=== RUN   TestReproTeardownWrongBoundFired
    coordinator_lifecycle_test.go:1894: Cancel never returned: reproduced the wrong-bound-fired flake deterministically
--- FAIL: TestReproTeardownWrongBoundFired (2.00s)
(x5)
```

This confirmed the mechanism before any fix was written. The scratch
reproduction test was then removed and replaced with a permanent regression
test (see below) that pins the same interleaving using the real fix.

## The fix

`awaitBoundsPending()` (a single boolean check comparing two *global* counts)
was replaced with two baseline-anchored, per-call helpers in
`internal/transfer/helpers_test.go`:

- `awaitBoundPending(baseline int)` -- waits until a bound armed *after*
  `baseline` exists and is itself still pending (neither fired nor stopped).
  Safe whenever the very next bound to arm is the one the test wants to force
  (no interposed, self-resolving bound ahead of it).
- `awaitBoundResolvedAt(index int)` -- waits until the bound at a specific,
  already-armed index has resolved on its own (or been forced), without ever
  forcing it. Used to let an interposed, healthy call's own bound (typically
  `ServerPort.Stop`'s) finish naturally *before* the test starts looking for
  the bound after it, removing the ambiguity a single "is anything pending"
  snapshot cannot resolve.

This is closest to the task's first suggested option ("capture an expected
armed-count baseline, then wait for armed() to exceed it and for a pending
bound"), refined to use each call's own `fired`/`stopped` state instead of
the corrupted global `stops()` counter, and extended with an explicit
"wait for this one to resolve, don't force it" step wherever a cascade has
an interposed bound ahead of the target. The failure mode on a genuine
mismatch is now a named `t.Fatalf` ("no bound armed after the first N became
pending" / "the bound at index N never resolved on its own"), not a 10-second
hang -- addressing the task's second requirement, that a wrong-bound situation
should produce a named assertion failure rather than telling the caller
nothing.

Production code (`internal/transfer/*.go`, non-test) was not touched.

### Why not "make fire() target a specific armed call" or "add call identity"?

Both were considered. Targeting fire() by index would still need the caller
to know which index is safe to fire, which is exactly what the baseline/
resolved-at helpers already provide without changing the seam itself; adding
identity to the fake timer's calls would work but is a larger, riskier change
to a widely-shared seam (`fakeTimer` backs both `h.bounds` and `h.timer`) for
no behavioural benefit over anchoring in the test helpers. The chosen fix is
the smallest change that removes the ambiguity and keeps `fireIfArmed`'s
existing "most recently armed" semantics, which several other tests
(`TestCancelReportsACodedFailureWhenServerStopNeverReturns`,
`TestTimedOutServerStopFencesNewSessionsUntilTheProductionCallCompletes`)
already rely on safely via a poll-and-retry pattern.

## Sibling audit

Every use of `awaitBoundsPending` in `internal/transfer/coordinator_lifecycle_test.go`
was audited (8 call sites across 7 tests):

| Test | Cascade shape | Risk | Fix |
|---|---|---|---|
| `TestCancelReportsACodedFailureWhenTheDrainerNeverEnds` | beacon (already resolved during claim) -> server (interposed) -> drainer (target) | Real: single fire, but could land on server's still-pending bound | `awaitBoundResolvedAt` + `awaitBoundPending` with indices |
| `TestAuthorizeClaimCommitsWhenStopBeaconNeverReturns` | single bound (beacon) | None (no interposed bound) | Converted to `awaitBoundPending(baseline)` for consistency |
| `TestCancelHonoursAnAbandonedCallerContext` | single bound (lease wait) | None | Converted for consistency |
| `TestShutdownReportsACodedFailureWhenTheLeaseNeverFrees` | single bound (lease wait) | None | Converted for consistency |
| `TestAClaimWhoseStopBeaconTimedOutLeavesItForTeardown` (1st fire) | single bound (claim-time beacon) | None | Converted for consistency |
| `TestAClaimWhoseStopBeaconTimedOutLeavesItForTeardown` (2nd fire) | single new bound (coalesced beacon cleanup; beacon releases before server) | None (already used a correct baseline-based `Gosched` loop) | Replaced the ad-hoc loop with `awaitBoundPending` (same safety, less duplication) |
| `TestATeardownThatLosesTwoResourcesReportsBoth` | beacon (target #1) -> server (interposed) -> drainer (target #2) | **Real, confirmed root cause** | `awaitBoundResolvedAt` + `awaitBoundPending` with indices |
| `TestAnObserverThatNeverReturnsCannotHoldACommand` | single bound (observer publish) | None | Converted for consistency |

Two other multi-bound tests use a different, already-safe pattern
(`fireIfArmed()` in a poll-and-retry loop with no `awaitBoundsPending` call at
all -- `TestCancelReportsACodedFailureWhenServerStopNeverReturns` and
`TestTimedOutServerStopFencesNewSessionsUntilTheProductionCallCompletes`'s
first phase). These were reviewed and left unchanged: `fireIfArmed` only
checks a call's own `fired` flag, so a stray fire on an already-resolved
bound is a harmless no-op (closes an abandoned `timedOut` channel nobody is
listening on), and the loop keeps retrying until the awaited command
actually returns -- it never depends on the corrupted global `armed()`/
`stops()` comparison the buggy helper used.

**Total: 2 tests carried the real flaw** (confirmed above); the fix was
applied to all 7 affected `awaitBoundsPending` call sites for consistency,
even where risk was absent, so no copy-pasted use of the removed, unsafe
helper remains anywhere in the package.

### A bug introduced and caught during the fix itself

An earlier version of this fix captured the baseline for the second phase of
`TestATeardownThatLosesTwoResourcesReportsBoth` *after* `awaitCalls("server.Stop")`
returned, reasoning that a call's bound is armed strictly before it is logged
(true) and therefore already counted. That reasoning was incomplete: because
`ServerPort.Stop`'s hook has no artificial delay in that test, by the time the
polling `awaitCalls` loop (200us sleep granularity) noticed the log entry, the
coordinator's own goroutine could already have resolved that bound **and**
armed the drainer join's -- so the captured baseline sometimes already
included the drainer bound too, and the helper then waited forever for a
fourth bound that would never arm. This was caught immediately by running the
test at `-count=20`, which failed deterministically with "no bound armed after
the first 3 became pending." The fix was to wait for the interposed bound to
*resolve* by its own known index (`awaitBoundResolvedAt`) rather than to infer
its resolution from when the baseline was captured, which removes the
timing-dependence entirely.

## Permanent regression test

`TestATeardownThatLosesTwoResourcesStillFiresTheRightBoundWhenServerStopIsSlow`
was added next to the original test. It reuses the same D-100 scenario but
deliberately holds `ServerPort.Stop`'s hook open until the test has explicitly
confirmed (via a direct assertion on the fake timer's call state) that its
bound is still armed and unresolved, then releases it and proves the fixed
helpers still land on the drainer join's bound rather than the interposed
one. This pins the exact interleaving that caused the CI flake, deterministically,
rather than relying on scheduling luck to ever exercise it again.

## Verification

### Targeted hard run

```
$ CGO_ENABLED=1 go test -count=200 -race -run 'TestATeardown|Bound' ./internal/transfer
ok  	fairdrop/internal/transfer	2.521s
```

200/200, no failures, under `-race`.

### Full gate, in `verify.yml` order

```
$ wails build
... Built '.../fairdrop.app/Contents/MacOS/fairdrop' in 6.799s.
```
(flipped the three `frontend/wailsjs/go/...` files to executable with no
content change; restored to 644 per AGENTS.md, and reverted the regenerated
`frontend/browser/captures/qr-panel-forced-colors.capture.png` diff with
`git checkout --`.)

```
$ gofmt -l .
(no output -- clean)

$ go vet ./...
(no output -- clean)

$ go tool staticcheck ./...
(no output -- clean)

$ go test -count=1 ./...
ok  	fairdrop	1.580s
ok  	fairdrop/internal/network	0.219s
ok  	fairdrop/internal/qr	0.947s
ok  	fairdrop/internal/server	5.396s
ok  	fairdrop/internal/source	0.796s
ok  	fairdrop/internal/stream	3.434s
ok  	fairdrop/internal/transfer	1.925s
ok  	fairdrop/scripts	1.080s
ok  	fairdrop/scripts/mutationverdict	1.426s

$ CGO_ENABLED=1 go test -count=1 -race ./...
ok  	fairdrop	8.222s
ok  	fairdrop/internal/network	2.211s
ok  	fairdrop/internal/qr	1.700s
ok  	fairdrop/internal/server	5.825s
ok  	fairdrop/internal/source	1.979s
ok  	fairdrop/internal/stream	102.370s
ok  	fairdrop/internal/transfer	2.882s
ok  	fairdrop/scripts	2.729s
ok  	fairdrop/scripts/mutationverdict	2.381s

$ npm test   (frontend)
 Test Files  17 passed (17)
      Tests  666 passed (666)

$ npm run test:browser   (frontend)
 Test Files  2 passed (2)
      Tests  24 passed (24)

$ GOOS=windows GOARCH=amd64 go build ./...
(no output -- clean)
```

This machine is macOS (darwin/arm64), so the darwin build/vet/staticcheck IS
the native run above (not a cross-build pre-flight); `GOOS=linux GOARCH=amd64
go build ./...` was also run clean as an extra check, matching the AGENTS.md
pre-flight guidance for platforms this machine does not natively compile as
its primary target.

## Scope

Test-only change: `internal/transfer/coordinator_lifecycle_test.go` and
`internal/transfer/helpers_test.go`. No production code
(`internal/transfer/*.go` non-test files) was modified. No assertion was
weakened -- every test still asserts the same outcomes it did before; the
fix only changes how the test seam decides *when* to force a bound, and adds
one new regression test.
