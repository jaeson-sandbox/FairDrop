# Evidence: Story 3.6: Make Lost and Malformed Events Visible

## What the story found, in one sentence

FairDrop's honesty mechanism did not exist in a shipped build: `docs/fairdrop-contracts.md` leans on
"recorded as a diagnostic" wherever a failure is deliberately swallowed, and every one of those
records went to a sink that production writes and only tests read.

Baseline: `3d979dd97dc3d4575848271a2ad0b21299938074` (the spec-approval parent), on top of the
Story 3.5 merge.

## The twelve ids, and where each is now defended

| id | What it was | Where it is fixed | The test that fails without it |
|---|---|---|---|
| D-098 | Every recorded diagnostic reached nothing outside the process | `coordinator.go` `recordDiagnostic` calls an injected seam; `main.go` wires it to `app.go`'s `logf` | `TestComposeWiresTheDiagnosticSeam`, `TestEveryRecordedDiagnosticAlsoReachesTheSeam` |
| D-092 | A terminal failure's cause was destroyed by the public rewrite | `outcomes.go` records the code before `terminalPublicError` | `TestATerminalFailureRecordsItsCauseBeforeRewritingIt` |
| D-100 | `unwind` reported the first unaccounted resource only | `errors.Join` across `releaseAcquired` and the drainer join | `TestATeardownThatLosesTwoResourcesReportsBoth` |
| D-043 | A panicking observer unwound past every lease release | `publishToObserver` recovers and records | `TestEveryRecordedDiagnosticAlsoReachesTheSeam` |
| D-034 | A blocking observer held the operation lease forever | `callBounded(observerPublishBound, ...)` | `TestAnObserverThatNeverReturnsCannotHoldACommand` |
| D-042, D-091 | A lane closing at STAGED or CLAIMING synthesised nothing | `acceptTerminal(live, event, stateStaged, stateClaiming, stateTransferring)` | `TestALaneThatClosesWhileStagedStillEndsTheSession`, `TestTheDrainerMayEndASessionFromEveryStateOneCanBeIn` |
| D-020 | A repeated `Stop` discarded the first call's diagnostic | `Stop` stores `s.unresolved` and replays its `teardown()` | `TestARepeatedStopDoesNotUpgradeAnUnresolvedTeardownToSuccess`, `TestARepeatedStopReplaysTheFirstCallsCleanupDiagnostic` |
| D-021 | `ErrorLog` discarded genuine handler panics with the request text | `panicOnlyErrorLog` forwards a fixed line for the one recognized prefix and no bytes of net/http's own | `TestErrorLogForwardsOnlyARecognizedPanicLineAndDropsEverythingElse`, `TestARealHandlerPanicIsReportedThroughErrorLog` |
| D-031 | The sink dropped silently once full | A reserved last slot holding `diagnosticOverflow` | `TestTheDiagnosticSinkSaysWhenItStoppedRecording` |
| D-049 | `undelivered` was counted and never surfaced | `app.go` logs all three drop paths | `TestPublishBeforeStartupDropsTheEventWithoutEmitting`, `TestPublishRecoversAnEmitPanicSoTheLeaseIsNotStranded`, `TestRestoreWindowBeforeStartupCountsUndeliveredAndLogsWithoutCallingTheRuntime` |
| D-059 | A non-retained terminal outcome carried no control at all | `OutcomePanel` renders a control whenever `onDismiss` exists; `App` passes Cancel for a live one | "a live terminal outcome is never a dead end" (App.test.tsx) |

## Epic 1 retrospective items 2, 3, 4 and 7

**Item 2 -- a refused terminal event rendered as the cancel-won summary.** `state.ts` returned the
same state for a `transfer-complete` whose snapshot disagreed with the staged metadata. The view
stayed in Transferring, the backend's three-second reset landed on plain Idle, and
transferring -> plain idle is `announce.ts`'s `cancel-won` row: a transfer that had finished was
announced as "Transfer canceled" -- strictly worse than the downgrade the Go side deliberately
refuses to make. A terminal event is now terminal from every path. What the refusal costs is the
claim of success, not the end of the session, so a refused completion reports `transfer_failed`
and a terminal error reports its own code rather than being dropped.

Pinned end to end by `announce.test.ts`'s "announces a refused completion as the outcome, never as
the cancellation", which drives the real reducer rather than hand-building the state -- each layer
was right alone, so a routing test over a constructed state would not have caught it.

**Item 3 -- `Warning.Code` unconstrained at the boundary.** Closed earlier in the story by
`transfer.WarningCode`, a type narrower than `ErrorCode`, plus
`TestEveryWarningCodeIsAcceptedByTheFrontendParser`: a code the TypeScript `parseWarning` would
reject now fails a Go test rather than cancelling a good session at runtime.

**Item 4 -- the discovery warning never reached a screen reader.** The warning arrives with the
metadata and never after it, so the only transition it belongs to is Stage success, which the
routing table gives to the staged heading's focus move. The announcer row that would have spoken it
fires only when a warnings array grows at an already-staged session, which no reducer path produces,
and the banner sat inside the packet, outside both the focused node and any live region.

The heading is now described by the banners (`aria-describedby`). That is what a focus-owned row can
carry without becoming a second owner: the move that announces Stage success reads the warning as
part of the same announcement, and the banner keeps its place for the people who can see it. The
announcer row is left in the table for a reducer that replaces metadata, and says so.

**Item 7 -- progress coherence implemented three times, metadata parsed twice.**

*One strategy.* `validation.ts` rejected an incoherent snapshot and `selectors.ts` then repaired one.
The repairs could never run, because the rejecting layer ran first. Worse, the rejection was coupled
to an exact float expression evaluated in Go -- `percent` had to equal `100 * bytesSent / totalBytes`
within 8 ULP -- so the day a sender rounded a percentage for display, every progress event would have
been refused in silence and the meter would have frozen with no error anywhere.

The validator now checks `percent` for range only, and the selector derives the displayed figure from
the two authoritative integers. `bytesSent <= totalBytes` is the validator's rule, which is what keeps
the derived value inside [0,100]; only `known-positive` gets a value at all, so an unknown total and
an empty payload still have no percentage. The dead repair branch is deleted and the guarantee it
stood for -- that a NaN, infinite or negative total never produces a determinate bar -- is re-asserted
against `parseProgressSnapshot`, the layer that actually owns it.

Twelve existing tests changed, every one of them a test that pinned the old rule by name ("carrying
the wire percentage", "fills the track from the wire percentage"). Each now says what it pins and why
it moved.

*Parsed once.* `parseFileMetadata` ran in the controller and again in the reducer, repeating a base64
decode and a full PNG chunk walk of up to 2 MB. The second parse could only fail on a value the first
had accepted, and its answer to a failure was to return the same state -- leaving the window in
Pending with no error and no announcement, while the controller's answer to the same input was to
quiesce the backend session and report `setup_failed`. The action now carries `FileMetadata`, so the
type is the proof that nothing unparsed reaches the reducer, and `useTransfer.test.tsx` counts
`atob` to keep the claim observable.

## Mutation table

Every mutation was applied to production source, run against the full suite, and reverted.

| # | Mutation | Result |
|---|---|---|
| M99 | `app.go` drops the "no window yet" log line | KILLED -- `TestPublishBeforeStartupDropsTheEventWithoutEmitting` |
| M100 | `app.go` drops the "emit panicked" log line | KILLED -- `TestPublishRecoversAnEmitPanicSoTheLeaseIsNotStranded` |
| M101 | `state.ts` returns the same state for a refused completion | KILLED -- two tests, including the reducer-driven routing test |
| M102 | the staged heading loses its `aria-describedby` | KILLED -- "describes the focused heading with the warning" |
| M103 | the selector reads `progress.percent` again | KILLED -- nine tests across four files |
| M104 | the reducer re-parses the acknowledgement | KILLED -- "parses the acknowledgement once" |
| M105 | `compose` drops the diagnostics seam | KILLED -- `TestComposeWiresTheDiagnosticSeam` |
| M106 | `recordDiagnostic` writes only to the sink | KILLED -- `TestEveryRecordedDiagnosticAlsoReachesTheSeam` |
| M107 | a recovered observer panic is swallowed | KILLED -- same |
| M108 | the sink drops silently once full | **SURVIVED**, then killed by `TestTheDiagnosticSinkSaysWhenItStoppedRecording` |
| M109 | the failure's original cause is not recorded | **SURVIVED**, then killed by `TestATerminalFailureRecordsItsCauseBeforeRewritingIt` |
| M110 | only TRANSFERRING synthesises a terminal outcome | **SURVIVED**, then killed by `TestALaneThatClosesWhileStagedStillEndsTheSession` |
| M111 | `ErrorLog` swallows handler panics again | INVALID on its first run (unused import: a compile error proves nothing), redone behaviourally -- KILLED by two tests |
| M112 | `unwind` reports only the first unaccounted resource | **SURVIVED**, then killed by `TestATeardownThatLosesTwoResourcesReportsBoth` |
| M113 | a repeated `Stop` forgets the first call's diagnostic | KILLED -- `TestARepeatedStopDoesNotUpgradeAnUnresolvedTeardownToSuccess` |
| M114 | the observer call is unbounded again | KILLED -- `TestAuthorizeClaimCommitsAndPublishesStarted`, `TestAnObserverThatNeverReturnsCannotHoldACommand` |

**Four of twelve ids were implemented and defended by nothing.** That is the finding worth keeping
from this story's verification pass: the implementation was correct in all four cases, and the suite
would have stayed green through a revert of any of them. A fifth, D-034, was pinned only by a
call-order assertion that the bound is *armed* -- nothing drove a blocking observer to its bound and
asserted the command still returned.

**Two tests that first passed while proving less than they claimed.**
`TestATeardownThatLosesTwoResourcesReportsBoth` first cancelled from TRANSFERRING, where the claim
handshake has already stopped the beacon -- so only one resource was left to lose. It failed rather
than passing vacuously, which is the good outcome of that mistake.
`TestTheDrainerMayEndASessionFromEveryStateOneCanBeIn` passes its own state list to the guard, so it
would stay green if the production call site narrowed again; the test says so, and the behavioural
test beside it is what pins the call site.

## Verification

All read stage by stage, never chained behind a shell exit status:

- `gofmt -l .` -- clean
- `go vet ./...` -- clean
- `go tool staticcheck ./...` -- clean
- `go test -count=1 -timeout 240s ./...` -- 7 packages ok
- `go test -count=1 -race -timeout 420s ./...` -- 7 packages ok
- `GOOS=darwin GOARCH=arm64 go vet ./...` and the linux equivalent -- clean
- `cd frontend && npx tsc --noEmit` -- clean
- `cd frontend && npx vitest run` -- 17 files, 498 tests passing
- The two new timing-sensitive transfer tests, three consecutive `-race` runs -- ok each time

## Deferrals

None. Every finding this story surfaced was fixed inside it, under the deferral policy tightened on
2026-09-11: a finding is deferred only when it needs a human decision, lives in a package the Code
Map does not name, or is a genuinely different goal. The seven ids that fit the third case were moved
to Story 3.11 when the story was narrowed, before implementation began, rather than deferred out of
it afterwards.
