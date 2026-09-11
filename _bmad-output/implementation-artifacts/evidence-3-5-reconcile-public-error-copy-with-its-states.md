# Evidence: Story 3.5: Reconcile Public Error Copy with the States It Describes

## Audit (written before any code changed)

Baseline: `8d3c499dd03b14dfb7074c24b62f8f31ddfa4e95` (the spec-approval commit on top of `96a5a3c`,
the frozen `baseline_commit`). Working tree clean at the start of this audit.

### Method

`grep -rn "ErrTransferFailed" --include="*.go" . | grep -v _test.go | grep -v /errors.go:` finds every
production Go site that can emit the `transfer_failed` code, independent of the spec's own "74
production sites" estimate. The actual count, by file:

| File | Sites |
|---|---|
| `app.go` | 6 |
| `internal/network/network.go` | 1 |
| `internal/qr/qr.go` | 1 |
| `internal/server/handler.go` | 2 |
| `internal/server/lifecycle.go` | 2 |
| `internal/source/source.go` | 6 |
| `internal/stream/archive.go` | 15 |
| `internal/stream/payload.go` | 18 |
| `internal/transfer/coordinator.go` | 8 |
| `internal/transfer/lifecycle.go` | 3 |
| `internal/transfer/outcomes.go` | 6 |
| **Total** | **68** |

Plus one TypeScript producer that reaches the same fixed copy without going through
`ErrorCodeOf`'s fallback at all: `frontend/src/transfer/useTransfer.ts`'s `stage()` hand-writes
`publicError('transfer_failed')` when a successful Stage acknowledgement fails to parse.

Each site was read in place (not just counted) and classified by **phase**: had a transfer begun
-- meaning the backend had committed to sending something, whether or not a byte had reached the
wire -- when this site can fire? Two phases separate every site found:

- **Before a transfer began.** Stage has not yet committed STAGED (setup: source inspection,
  network selection, identity generation), or a transfer command is refused before any of that
  runs at all (pre-startup, pre-composition, an already-terminal session refusing a new Stage).
  Nothing has been sent, and in most cases nothing has even been acquired yet.
- **After a transfer began.** Bytes have started flowing, or the backend has already told the
  receiver it is committed to a claim (`transfer-started` published, or headers already on the
  wire), or genuine mid-stream/mid-close I/O failed against a live resource.

The 68 Go sites plus the one TypeScript site sort overwhelmingly into "after a transfer began" or
into a third bucket -- Story 3.4's bounded-wait diagnostics during teardown, which are reported
only as internal diagnostics or, when they do reach a caller, reach it after resources a real
session held were already live (see the D-103 finding below for the one case that is not quite
that clean). Representative honest sites, sampled rather than exhaustively re-derived here since
their correctness is not this story's finding:

- `internal/stream/payload.go:390,407,414,426` -- `WriteTo`'s mid-copy read/write/progress
  failures. Headers are already on the wire (a directory or a file's `Content-Length`); a stream
  that breaks here is exactly "the transfer stopped before FairDrop finished sending."
- `internal/stream/archive.go` (bulk of its 15 sites) -- all inside `WriteTo`/`produce`/`drain`/
  `writeEntries`, i.e. after the response has started for a directory download. Per
  `docs/fairdrop-contracts.md`, "Because the walk begins after the response has started, a failure
  it finds cannot choose an HTTP status and terminates the connection instead" -- these sites are
  the honest terminations that sentence describes.
  `internal/stream/payload.go:107` and `payload.go:229` are the one Prepare-phase call classified
  differently below (D-012).
- `internal/server/handler.go:116,166` -- a claimed, authorized request whose payload preparation
  or streaming failed after the claim committed.
- `internal/source/source.go` (6 sites) -- selection-walk arithmetic/visitor/context guards that
  fire while a real Inspect/Walk is already underway for a staged or claimed item.

### The seven named states

Grouped by phase. All seven are real, and six of the seven currently render as
"The transfer stopped before FairDrop finished sending. Check the local network and create a
fresh link." (`transfer_failed`'s fixed copy) -- false in every one of them, because none involves
a transfer that began. The seventh (`busy`) is a real code correctly chosen, with a message false
in one of the two states it actually covers.

**Phase: before a transfer began**

1. **D-025 -- entropy fails while staging.** `internal/transfer/coordinator.go:920-943`,
   `newIdentity`/`randomHex`. Called from `Stage` (line 321) *before* `c.mu.Lock()` -- no state has
   changed, no resource has been acquired, IDLE was never left. Current code: `ErrTransferFailed`
   via `WrapError(ErrTransferFailed, "FairDrop could not create a transfer session", err)`
   (`coordinator.go:941`). Current message: "The transfer stopped before FairDrop finished
   sending..." -- false; nothing was ever staged, let alone sent.

2. **D-012 -- a deadline expires during Prepare.** `internal/stream/payload.go`, `contextError`
   (`payload.go:564-584`), reached from `Prepare` at line 111 (top of the function, before the
   Kind switch or any resource is touched), line 138 (after `Inspect`), line 154 (after
   `pinIdentity`), and line 175 (after `Stat`); and from `prepareArchive` at line 229 (after its
   own `pinIdentity`). All five run before `Prepare` returns a payload, and per
   `docs/fairdrop-contracts.md`, "`Prepare` runs before response headers" -- no byte has reached
   the wire at any of them, file or directory. Current code for a `DeadlineExceeded`:
   `ErrTransferFailed` (`payload.go:573-577`). Current message: "The transfer stopped..." -- false;
   Prepare's own three sibling `contextError` call sites inside `WriteTo` (`payload.go:360,378,401`)
   *are* mid-stream and correctly stay `transfer_failed` -- this is the one place in the audit
   where the same helper function serves both a wrong-copy site and several honest ones, which is
   why the fix must split the function by phase rather than change the code globally.

3. **D-053 -- a malformed Stage acknowledgement.** `frontend/src/transfer/useTransfer.ts:104-137`,
   the `stage()` callback. When `parseFileMetadata(rawMetadata)` returns `null` after a Stage
   command that itself *resolved* successfully, the code cancels the phantom session and dispatches
   `publicError('transfer_failed')` (line 123). No lifecycle event was ever received (Stage merely
   resolved with unparseable JSON), so nothing was sent; the user's selection was refused, not
   interrupted. Current message: "The transfer stopped..." -- false.

4. **D-015 -- an uncoded `SourcePort` error.** `internal/stream/payload.go:134-137`, `Prepare`'s
   call to `p.source.Inspect(ctx, item.Path)`, returned verbatim: `if err != nil { return nil, err
   }`. The real `internal/source` adapter always returns a `CodedError` today, so this path is not
   observed to misfire in production -- but nothing enforces that at the port call, and an
   uncoded error reaching `PublicErrorOf` three layers away would fall through
   `ErrorCodeOf`'s generic fallback to `transfer_failed`'s "The transfer stopped..." copy, which is
   false for the same reason as D-012: `Inspect` here runs before headers, inside `Prepare`.
   `docs/fairdrop-contracts.md`'s `SourcePort.Inspect` postcondition already promises only
   `cancelled`, `path_not_found`, `path_unsupported`, `source_changed`, or `transfer_failed` (for
   invalid size arithmetic) -- the boundary should enforce that promise rather than trust it.

5. **D-047 -- pre-startup refusals.** `app.go`, two sites. `errNotComposed()` (line 513-518),
   returned by `StageTransfer` (line 177) and `CancelTransfer` (line 203) when `main.go` has not
   yet handed the App its coordinator -- unreachable in a composed binary, since `compose` always
   runs before `wails.Run`. `chooseWith`'s nil-context guard (line 271-280), reached by
   `SelectFile`/`SelectDirectory` if a dialog is invoked before `OnStartup` installs `a.ctx` --
   also unreachable in a composed binary, since Wails runs `OnStartup` before the webview can call
   a command, but `TestAppOptionsSecondInstanceCallbackBeforeStartupIsSafe`-style races already
   prove ordering isn't free to assume. Both use `transfer.ErrTransferFailed`. Current message:
   "The transfer stopped..." -- false; neither state involves a transfer that began, and both are
   pinned reachable-by-test today (`TestCommandsRefuseBeforeCompositionRatherThanPanicking`,
   `TestDialogBeforeStartupIsRefusedRatherThanFatal`) even though production composition makes
   them unreachable, exactly D-047's own description: "still honest if reached."

6. **D-029 -- `ready()` finds a nil port.** `internal/transfer/coordinator.go:908-914`, called
   from both `Stage` (line 314) and `AuthorizeClaim` (line 504), before either touches state.
   Current code: `ErrTransferFailed` (line 911). Current message: "The transfer stopped..." --
   false for `Stage`'s call (nothing began); also contradicts
   `docs/fairdrop-contracts.md`'s claim-authorization row, which documents only `cancelled` or
   `shutting_down` as `AuthorizeClaim`'s return codes. In the one real caller, `main.go`'s
   `compose`, every port is supplied, so this is unreachable in the shipped binary -- but nothing
   today makes that true by construction; `NewCoordinator(Dependencies{})` (used directly by two
   existing tests, `TestStageRefusesWithoutItsPorts` and `TestTheDefaultResetSchedulerIsARealTimer`)
   builds a coordinator with every port nil and returns it without complaint.

**Phase: state, not send**

7. **D-044 -- `Stage` during the terminal lease.** `internal/transfer/coordinator.go:331-334`, the
   `c.state != stateIdle` busy guard inside `Stage`, already correctly coded as `ErrBusy` and
   already pinned reachable by `TestStageIsRefusedDuringTheTerminalLease`
   (`coordinator_stage_test.go:971`) for both the DONE and ERROR terminal states. The code is
   right; only the fixed message is wrong for half of what it describes: "Finish or cancel the
   current transfer before choosing another item" is true while STAGING, STAGED, CLAIMING, or
   TRANSFERRING (there genuinely is an active transfer to finish or cancel), but false while DONE
   or ERROR -- the transfer already finished, and the coordinator is holding its outcome on screen
   for the fixed three-second terminal lease (`resetDelay`, `lifecycle.go:13`). There is nothing
   left to "finish."

### A new finding, out of this story's scope: D-103

While tracing every phase-before-a-transfer-began call site, `internal/transfer/lifecycle.go`'s
`retire` (line 125-167) surfaced one more shape of the same defect that is **not** one of the
seven named states and is **not fixed by this story**, because fixing it would mean writing new
message text the reviewer has not confirmed (Ask First).

`Cancel` called against a **STAGED** session (metadata already returned to the UI, but
`AuthorizeClaim` never ran -- no `transfer-started` was ever published, no byte was ever sent) ends
in `retire`, whose `unwindErr` -- the *first bound failure* `unwind` hits while stopping the
server/beacon or joining the drainer (Story 3.4's bounded-wait work) -- is returned directly as
`Cancel`'s own error (`lifecycle.go:166`: `return unwindErr`). Every one of those bound failures is
coded `ErrTransferFailed` (`coordinator.go:656,727,743`), so a `Cancel` that hits one of these
*rare, adapter-misbehavior-only* bounds while cancelling a STAGED-but-never-claimed session would
show the user "The transfer stopped before FairDrop finished sending" for a transfer that, again,
never sent anything. This is the same shape as the seven states above, introduced by Story 3.4's
bounded-wait work rather than by anything this story's Code Map named, and not covered by the
`busy`/`setup_failed` fix either (busy's revised copy talks about a transfer still running or
just finished; this is a `Cancel` failing outright). Recorded as `D-103`, owned by
`3-6-make-lost-and-malformed-events-visible` (the next story touching the fixed public error
surface, per `epic-3-context.md`'s cross-story note that "3.5 and 3.6 both touch the fixed public
error surface, so the cross-language pin must cover any code either introduces") -- see
`deferred-work.md` and `epics.md`'s Story 3.6 `Closes:` line.

### Disposition

| State | Current code | Current message true? | Fix |
|---|---|---|---|
| D-025 entropy fails while staging | `transfer_failed` | No | New code `setup_failed` |
| D-012 deadline during Prepare | `transfer_failed` | No | New code `setup_failed` (only the four/five Prepare-phase `contextError` call sites; `WriteTo`'s three stay `transfer_failed`) |
| D-053 malformed Stage acknowledgement | `transfer_failed` | No | New code `setup_failed` |
| D-015 uncoded `SourcePort` error | falls through to `transfer_failed` | No (when it would misfire) | Wrap at the port call with `setup_failed` when the error is not already a `CodedError` |
| D-047 pre-startup refusals (×2) | `transfer_failed` | No | New code `setup_failed` |
| D-029 `ready()` finds a nil port | `transfer_failed` | No | Construction refuses a nil port (`NewCoordinator` panics); the residual defensive path in `ready()` reports `setup_failed`, not `transfer_failed` |
| D-044 `Stage` during the terminal lease | `busy` | Half true | Same code, revised message true in both the running and terminal-lease states |

Six states get the new code `setup_failed` (heading "Couldn't prepare that item", message
"FairDrop couldn't prepare that item. Nothing was sent. Choose it again." -- confirmed by the
reviewer at Checkpoint 1, used verbatim including its typographic apostrophes). One state
(`busy`) keeps its code and gets a revised message, also confirmed verbatim. No other producer
changes.

---

## What changed

**New code.** `internal/transfer/errors.go`: `ErrSetupFailed ErrorCode = "setup_failed"`, and its
entry in `publicMessages`: *"FairDrop couldn't prepare that item. Nothing was sent. Choose it
again."* `busy`'s entry revised to *"FairDrop is still finishing the last transfer. Wait a moment,
or cancel it, then choose another item."* Both strings used verbatim from the spec's Design Notes,
including their typographic apostrophes.

**Registry, contract, mirror.** `EXPERIENCE.md`'s stable-error table gains the `setup_failed` row
(heading "Couldn't prepare that item", the confirmed message, "Focused Error Panel", recovery
"Choose the item again.") and its `busy` row's message and recovery are revised.
`docs/fairdrop-contracts.md` gains `ErrSetupFailed` in its Go pseudocode block and a `setup_failed`
row in its stable-codes table, and its two prose sentences describing which codes claim
authorization and payload preparation/streaming can return now name it.
`frontend/src/transfer/errors.ts` gains `setup_failed` in `transferErrorCodes` and
`fixedErrorMessages`, and revises `busy`'s message. `frontend/src/ui/copy.ts`'s
`errorHeadings` (a `Record<TransferErrorCode, string>`, so the compiler itself requires every code)
gains `setup_failed: 'Couldn't prepare that item'`.

**The six producers, each now emitting `setup_failed`:**

- `internal/transfer/coordinator.go`'s `randomHex` (D-025).
- `internal/stream/payload.go`'s `Prepare`/`prepareArchive`: `contextError` split into
  `classifyContextError(ctx, deadlineCode)` with two thin wrappers -- `contextError` (unchanged,
  `ErrTransferFailed`) for the three call sites inside `WriteTo` (after headers), and the new
  `prepareContextError` (`ErrSetupFailed`) for the five call sites inside `Prepare`/`prepareArchive`
  (before headers) (D-012).
- `internal/stream/payload.go`'s `Prepare`, at the `p.source.Inspect` call: a new
  `wrapUncodedSourceError` helper checks `errors.As(err, &transfer.CodedError)` and wraps only when
  the adapter's error is not already coded, so a compliant `SourcePort` (the only kind that exists
  today) is unaffected (D-015).
- `frontend/src/transfer/useTransfer.ts`'s `stage()`, the malformed-acknowledgement fallback
  (D-053).
- `app.go`'s `errNotComposed()` and `chooseWith`'s nil-context guard (D-047).
- `internal/transfer/coordinator.go`'s `ready()` (D-029) -- now a residual, defensive path only:
  `NewCoordinator` panics (`"transfer: NewCoordinator requires every port and the observer"`) if
  `Source`, `Network`, `Server`, `QR`, or `Observer` is nil, so a coordinator built by the one real
  caller (`main.go`'s `compose`, which always supplies every port) can never reach `ready()`'s nil
  branch at all.

**`busy`'s copy only** (`internal/transfer/coordinator.go`'s `Stage` busy guard, D-044): no code
change; the guard was already correct for both states it covers.

**The extended pin.** `main_test.go`: `TestEveryStableCodeIsRecognizedByTheFrontend` replaced by
`TestTheCrossLanguageErrorRegistryPinsEveryCodeAndMessage`, driven by a spelled-out
`registryEntries` literal (13 `{code, message}` pairs). For each entry it checks the Go table
through the real `formatCommandError` (not by parsing `errors.go`'s source), the TypeScript
mirror's code list and message (`fixedErrorMessages`, resilient to its longest value wrapping onto
its own line), `docs/fairdrop-contracts.md`'s code list, and `EXPERIENCE.md`'s row -- including
resolving `beacon_warning`'s cell, which names the stable Voice-and-Tone key
`copy.discovery.warning` rather than spelling the message out inline, against that second table
rather than skipping it. `internal/transfer/errors_test.go`'s
`TestTheCodeRegistryIsExactlyTheseTwelveCodes` renamed to `...Thirteen...` and gained
`ErrSetupFailed`; `TestPublicErrorOfExactRegistryCopy` and `TestIndependentCodedErrorSurvivesWrapping`
carry the revised `busy` message.

**Tests updated for the code/message changes:** `main_test.go` (`TestAppOptionsRegistersTheErrorFormatter`),
`app_test.go` (`TestDialogBeforeStartupIsRefusedRatherThanFatal`,
`TestCommandsRefuseBeforeCompositionRatherThanPanicking`), `internal/transfer/coordinator_stage_test.go`
(`TestStageFailsClosedWhenEntropyFails`; `TestStageRefusesWithoutItsPorts` replaced by
`TestNewCoordinatorRefusesAMissingPortOrObserver`, which drives six sub-cases -- no ports at all and
each single missing port/observer -- through `recover()`, and `TestReadyReportsSetupFailedRatherThanTransferFailed`,
which constructs a zero-value `Coordinator{}` directly to exercise the now-unreachable-via-constructor
residual path), `internal/transfer/coordinator_lifecycle_test.go` (`TestTheDefaultResetSchedulerIsARealTimer`
now supplies inert fakes rather than `Dependencies{}`), `internal/stream/payload_test.go`
(`TestPrepareDistinguishesDeadlineExpiryFromCancellation`, plus four new tests isolating each of the
other four Prepare-phase `contextError` call sites -- during `Inspect`, during `pinIdentity`, during
`open`, and `prepareArchive`'s own -- and two new tests for D-015:
`TestPrepareCodesAnUncodedSourceError` and `TestPrepareLeavesAnAlreadyCodedSourceErrorUnchanged`),
frontend `errors.test.ts`, `selectors.test.ts`, `state.test.ts`, `useTransfer.test.tsx`,
`OutcomePanel.test.tsx` (gained a `setup_failed` case), `copy.test.ts`.

**Deferred work.** `deferred-work.md`: D-012, D-015, D-025, D-029, D-044, D-047, D-053 all set to
`owner: discharged`, with a new "Discharged (Story 3.5)" banner explaining what closed each and
pointing at this file; a new entry, `D-103`, records the one finding this story's audit surfaced but
did not fix (see above), owned by `3-6-make-lost-and-malformed-events-visible`. `epics.md`'s Story
3.6 `Closes:` line gained `D-103`; Story 3.5's own `Closes:` line was already correct and untouched.

## Verification

Run from the repo root, in the order AGENTS.md and this spec's Verification section specify. Every
command below was run to completion against the finished tree; the full transcript is not pasted
verbatim (it is long), but the outcome of each is:

```
$ gofmt -l .                                              # clean, no output
$ go vet ./...                                             # clean
$ go tool staticcheck ./...                                # clean
$ go test -count=1 -timeout 240s ./...
ok  	fairdrop                    0.137s
ok  	fairdrop/internal/network   0.222s
ok  	fairdrop/internal/qr        0.204s
ok  	fairdrop/internal/server    3.395s
ok  	fairdrop/internal/source    0.283s
ok  	fairdrop/internal/stream    0.711s
ok  	fairdrop/internal/transfer  0.501s
$ CGO_ENABLED=1 go test -count=1 -race -timeout 420s ./...
ok  	fairdrop                    1.761s
ok  	fairdrop/internal/network   1.157s
ok  	fairdrop/internal/qr        1.429s
ok  	fairdrop/internal/server    4.279s
ok  	fairdrop/internal/source    1.287s
ok  	fairdrop/internal/stream    5.341s
ok  	fairdrop/internal/transfer  1.476s
$ GOOS=darwin GOARCH=arm64 go build ./... && GOOS=darwin GOARCH=arm64 go vet ./...      # clean
$ GOOS=darwin GOARCH=arm64 staticcheck ./...                                            # clean (bare binary, per AGENTS.md)
$ GOOS=linux GOARCH=amd64 go build ./... && GOOS=linux GOARCH=amd64 go vet ./...        # clean
$ cd frontend && npx vitest run
 Test Files  17 passed (17)
      Tests  492 passed (492)
$ cd .. && wails build
Built 'C:\Users\jaeso\Coding\FairDrop\build\bin\fairdrop.exe' in 3.915s.
$ git status --porcelain frontend/wailsjs      # no drift: the App's bound command surface is unchanged
```

`go env CGO_ENABLED` printed `1` before the race run, with the WinLibs UCRT `mingw64/bin` on
`PATH`, per AGENTS.md's cgo caution. `go test -timeout` was set explicitly on every run touching
Go, per this story's own caution.

## Mutation table

Every mutation below was applied by hand, run, confirmed to fail a named test (or, for the two
"add/remove a whole-file guarantee" rows, confirmed against the relevant package), and reverted
before moving to the next. The tree was green (`go build ./...` plus the affected package's tests)
before and after every entry.

| Mutation | Result |
|---|---|
| Edit `busy`'s message in `internal/transfer/errors.go`'s Go table alone | `TestAppOptionsRegistersTheErrorFormatter`, `TestTheCrossLanguageErrorRegistryPinsEveryCodeAndMessage/busy`, `TestIndependentCodedErrorSurvivesWrapping`, `TestPublicErrorOfExactRegistryCopy/busy` all fail, naming the Go side |
| Edit `busy`'s message in `frontend/src/transfer/errors.ts` alone | `TestTheCrossLanguageErrorRegistryPinsEveryCodeAndMessage/busy` (Go) and `state.test.ts`'s "rewrites caller-supplied error copy..." (Vitest) both fail, naming the TS side |
| Edit `busy`'s message in `EXPERIENCE.md`'s registry table alone | `TestTheCrossLanguageErrorRegistryPinsEveryCodeAndMessage/busy` fails, naming the registry |
| Add a code to `internal/transfer/errors.go`'s `publicMessages` map alone (`mutated_extra_code`) | `TestTheCodeRegistryIsExactlyTheseThirteenCodes` fails, naming the stray code |
| Revert D-025 (`coordinator.go`'s `randomHex`) to `ErrTransferFailed` | `TestStageFailsClosedWhenEntropyFails` (both sub-cases) fails |
| Revert D-029 (`coordinator.go`'s `ready()`) to `ErrTransferFailed` | `TestReadyReportsSetupFailedRatherThanTransferFailed` fails on both its `Stage` and `AuthorizeClaim` assertions |
| Revert D-012's five `payload.go` call sites to `contextError`, one at a time | Prepare's entry check (line ~111) -- `TestPrepareDistinguishesDeadlineExpiryFromCancellation` fails. After `Inspect` -- `TestPrepareDistinguishesDeadlineExpiryDuringInspectFromCancellation` fails. After `pinIdentity` (file path) -- `TestPrepareDistinguishesDeadlineExpiryDuringPinIdentityFromCancellation` fails. After `Stat` -- `TestPrepareDistinguishesDeadlineExpiryDuringOpenFromCancellation` fails. `prepareArchive`'s own -- `TestPrepareArchiveDistinguishesDeadlineExpiryFromCancellation` fails. Each of the five was mutated and reverted independently; none is caught by another's test |
| Revert D-015 (`payload.go`'s `Inspect` call) to return `err` verbatim | `TestPrepareCodesAnUncodedSourceError` fails |
| Revert D-053 (`useTransfer.ts`'s malformed-ack fallback) to `publicError('transfer_failed')` | `useTransfer.test.tsx`'s "attempts Cancel exactly once for malformed successful metadata" fails |
| Revert D-047's `chooseWith` nil-context guard to `ErrTransferFailed` | `TestDialogBeforeStartupIsRefusedRatherThanFatal` fails |
| Revert D-047's `errNotComposed()` to `ErrTransferFailed` | `TestCommandsRefuseBeforeCompositionRatherThanPanicking` fails |
| Remove `NewCoordinator`'s missing-port/observer panic | `TestNewCoordinatorRefusesAMissingPortOrObserver` fails on all six sub-cases (no ports at all, and each of Source/Network/Server/QR/Observer missing alone) |

Every mutation named at least one failing test; no mutation left the suite silently green.

## Orchestrator mutation pass

Ten mutations run against the implementation before accepting it. Nine died at once; one survived and
was a real gap.

| # | Mutation | Result |
|---|---|---|
| M76 | The new message edited in the Go table alone | killed |
| M77 | The new message edited in the TypeScript mirror alone | killed |
| M78 | The new message edited in the EXPERIENCE registry alone | killed |
| M79 | The revised `busy` message reverted in the Go table alone | killed, by two tests |
| M80 | The new code removed from the TypeScript code list alone | killed |
| M81 | The new code removed from the binding contract alone | **survived**, then fixed |
| M82 | The new code removed from the registry table alone | killed |
| M83 | Entropy exhaustion reverted to `transfer_failed` | killed |
| M84 | The pre-composition refusal reverted to `transfer_failed` | killed |
| M85 | `NewCoordinator` accepts a missing port again | killed |

The first four are the point of the story, and they are the ones the old pin could not make: before
this, a message could be edited in one of three files and nothing would notice.

**M81 is a document disagreeing with itself.** `docs/fairdrop-contracts.md` states the code set twice
-- once as a prose table and once as a block of Go source restating the `ErrorCode` constants. The new
pin read the table and not the block, so deleting a code from the constant block left the suite green
while the contract contradicted itself inside one file. Both are now pinned, and removing a code from
either fails.

That is the same shape as the story itself. Four files restating one fact drift because nothing
compares them; a single file restating one fact twice drifts for exactly the same reason, and is
easier to miss because it looks like one source rather than two.

