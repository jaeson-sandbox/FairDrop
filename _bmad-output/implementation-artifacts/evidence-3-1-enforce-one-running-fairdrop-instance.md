# Evidence: Story 3.1: Enforce One Running FairDrop Instance

## Implementation Evidence

Transcript below is from the working tree at commit `d6b1baf` plus the patches this review round
applied on top (uncommitted, per the reviewer's instruction to leave changes in the working tree).
Gates on this worktree: `gofmt -l .` clean; `go vet ./...` 0; `go test -count=1 ./...` 0 across
every package; `go test -count=1 -race ./...` 0 (native runner, `CGO_ENABLED=1`, `CC=gcc`, so the
race build is real rather than silently skipped); frontend Vitest 17 files / 490 tests unchanged;
`wails build` alone afterward (never concurrently with `vitest`, since it regenerates bindings),
exit 0, producing `build/bin/fairdrop.exe`.

```
$ gofmt -l .
$ go vet ./...
$ go test -count=1 ./...
ok  	fairdrop	0.192s
ok  	fairdrop/internal/network	0.429s
ok  	fairdrop/internal/qr	0.383s
ok  	fairdrop/internal/server	1.955s
ok  	fairdrop/internal/source	0.677s
ok  	fairdrop/internal/stream	1.503s
ok  	fairdrop/internal/transfer	0.680s
$ go test -count=1 -race ./...
ok  	fairdrop	2.631s
ok  	fairdrop/internal/network	1.387s
ok  	fairdrop/internal/qr	1.877s
ok  	fairdrop/internal/server	2.959s
ok  	fairdrop/internal/source	1.712s
ok  	fairdrop/internal/stream	8.017s
ok  	fairdrop/internal/transfer	1.631s
$ cd frontend && npx vitest run
 Test Files  17 passed (17)
      Tests  490 passed (490)
$ cd .. && wails build
Built 'C:\Users\jaeso\Coding\FairDrop\build\bin\fairdrop.exe' in 11.843s.
```

New/extended tests, every one executed and passing:

- `TestAppOptionsEnforcesSingleInstance` (`main_test.go`) -- `SingleInstanceLock` carries the
  literal UUID and a non-nil `OnSecondInstanceLaunch`.
- `TestAppOptionsSecondInstanceCallbackRestoresTheWindow` (`main_test.go`) -- drives the callback
  *through the options value* (`appOptions(h.app).SingleInstanceLock.OnSecondInstanceLaunch`, not
  `h.app.restoreWindow` directly), so a stub wired into `appOptions` instead of the real method
  cannot pass. Asserts exactly `[unminimise, show]` with the stored context, coordinator untouched,
  nothing emitted.
- `TestAppOptionsSecondInstanceCallbackBeforeStartupIsSafe` (`main_test.go`) -- the same
  options-value proof for the pre-startup path: `undelivered == 1`, one `fairdrop:` line.
- `TestNewAppWiresTheRealWailsRuntime` (`app_test.go`) -- extended with `unminimise` →
  `wailsruntime.WindowUnminimise` and `show` → `wailsruntime.WindowShow`.
- `TestRestoreWindowBeforeStartupCountsUndeliveredAndLogsWithoutCallingTheRuntime` -- the
  pre-startup race: no runtime call, `undelivered` incremented, exactly one `fairdrop: ` line that
  discloses neither the second launch's `Args` nor its `WorkingDirectory`, coordinator untouched.
- `TestRestoreWindowUnminimisesThenShowsExactlyOnceWithTheApplicationLifetimeContext` -- the
  started case: unminimise then show, each exactly once, both carrying the stored
  application-lifetime context; no coordinator call; no emitted event; `undelivered` stays `0` and
  nothing is logged on a successful restoration.
- `TestRestoreWindowRacingStartupNeverCallsTheRuntimeWithANilContext` -- drives `startup` and
  `restoreWindow` concurrently, 200 rounds under `-race`, asserting only the zero-or-two invariant
  (never which side wins) plus, on every round, coordinator untouched, nothing emitted, and the
  undelivered/log state matching whichever outcome occurred.
- `TestRestoreWindowIgnoresTheSecondLaunchsArguments` -- `SecondInstanceData.Args`/
  `WorkingDirectory` populated with a path and a flag; behavior is identical to the empty case,
  nothing is staged or emitted, `undelivered` stays `0`, and no log line discloses either field.

All existing option and harness pins (window, drop, background colour, error formatter, bind,
lifecycle hooks, disclosure, publish ordering) stayed green with no edits beyond the seam-list
extension named above.

## Mutation Table

Each deliberate break below was made against the real `app.go`/`main.go`, run against the named
test(s) with `go test -run <name> -v` (the racing-startup row additionally with `-race`), observed
to fail, then reverted and reconfirmed green (`gofmt -l .`, `go vet ./...`, `go build ./...`).
Rows 1-6 are the implementer's original mutations; rows 7-11 are an independent verification round
that found one survivor (row 11) the implementer's set could not see and added a test to close it;
rows 12-14 are this review round's mutations, each demonstrating a gap in what rows 1-11 asserted.

| # | Deliberate break | Named failing test(s) | Observed failure |
| --- | --- | --- | --- |
| 1 | Remove the nil-context guard in `restoreWindow` | `TestRestoreWindowBeforeStartupCountsUndeliveredAndLogsWithoutCallingTheRuntime` | `restoreWindow called the runtime before startup: [{unminimise <nil>} {show <nil>}]`; `undelivered = 0, want 1`; `logged 0 lines, want exactly 1` |
| 2 | Swap the call order (`show` before `unminimise`) | `TestRestoreWindowUnminimisesThenShowsExactlyOnceWithTheApplicationLifetimeContext` | `the calls were show then unminimise, want unminimise then show` |
| 3 | Call `show` twice instead of `unminimise` then `show` | `TestRestoreWindowUnminimisesThenShowsExactlyOnceWithTheApplicationLifetimeContext` | `the calls were show then show, want unminimise then show` |
| 4 | Change the UUID literal in `main.go` | `TestAppOptionsEnforcesSingleInstance` | `SingleInstanceLock.UniqueId = "...025", want "...024"` |
| 5 | Drop `OnSecondInstanceLaunch` from the `SingleInstanceLock` literal | `TestAppOptionsEnforcesSingleInstance` | `OnSecondInstanceLaunch is nil: a second launch would have nothing to hand the window to` |
| 6 | Make `restoreWindow` call the coordinator (`_ = a.CancelTransfer()`) after restoring | `TestRestoreWindowUnminimisesThenShowsExactlyOnceWithTheApplicationLifetimeContext` | `restoreWindow reached the coordinator: [Cancel]` |
| 7 | Remove the `undelivered` increment on the pre-startup path | `TestRestoreWindowBeforeStartupCountsUndeliveredAndLogsWithoutCallingTheRuntime` | `undelivered = 0, want 1` |
| 8 | Drop the `show` call rather than swapping it | `TestRestoreWindowUnminimisesThenShowsExactlyOnceWithTheApplicationLifetimeContext` | runtime called once instead of twice |
| 9 | Call the runtime with `context.Background()` instead of the stored context | `TestRestoreWindowUnminimisesThenShowsExactlyOnceWithTheApplicationLifetimeContext` | context equality assertion fails |
| 10 | Swap the two seam defaults in `NewApp` (`unminimise`/`show`) | `TestNewAppWiresTheRealWailsRuntime` | seam does not point at the real Wails runtime function |
| 11 | Read `a.ctx` directly instead of through `runtimeContext()` (unlocked read) | `TestRestoreWindowRacingStartupNeverCallsTheRuntimeWithANilContext` (must run with `-race`) | `WARNING: DATA RACE` -- write at `App.startup` (`app.go:402`), previous read at `App.restoreWindow` (`app.go:460`); re-verified against the current test code in this round |
| 12 | Replace `app.restoreWindow` with a stub `func(options.SecondInstanceData) {}` in `appOptions` | `TestAppOptionsSecondInstanceCallbackRestoresTheWindow`, `TestAppOptionsSecondInstanceCallbackBeforeStartupIsSafe` | `the callback produced [], want exactly [unminimise, show]`; `undelivered = 0, want 1`; `logged 0 lines, want exactly 1: []` -- note `TestAppOptionsEnforcesSingleInstance` (non-nil check only) stayed green throughout, which is exactly the gap these two close |
| 13 | Lift `a.undelivered.Add(1)` and the log call above the nil-context guard | `TestRestoreWindowUnminimisesThenShowsExactlyOnceWithTheApplicationLifetimeContext`, `TestRestoreWindowIgnoresTheSecondLaunchsArguments`, `TestRestoreWindowRacingStartupNeverCallsTheRuntimeWithANilContext` | `undelivered = 1, want 0: a successful restoration is not a drop`; `restoreWindow logged ["fairdrop: undelivered (second instance before startup)"] on the success path, want silence`; racing test failed at round 14 (`a successful restoration counted undelivered = 1, want 0`) -- confirms the sequential tests alone are sufficient here, the racing test is not required to catch it but does |
| 14 | Log `data.Args` in the pre-startup drop line | `TestRestoreWindowBeforeStartupCountsUndeliveredAndLogsWithoutCallingTheRuntime` | `a log line disclosed the second launch's Args or WorkingDirectory: "fairdrop: undelivered (second instance before startup) args=[C:\Users\sender\Documents\quarterly report.pdf]"` |

No survivors as of this round: all fourteen mutations, including the one (row 11) that survived
the implementer's original six and every existing test under `-race`, now fail through a named
test. Row 11 is the one mutation any future change to this file should rerun with `-race`
specifically -- it is invisible without it, by construction.

## Matrix Audit

Every I/O & Edge-Case Matrix row maps to an executed test, with two qualifications:

- **"First launch -- lock held"** is Wails' own guarantee once `SingleInstanceLock` is configured,
  not something this story's Go tests can exercise (there is no second process to hold the lock
  against in a unit test). `TestAppOptionsEnforcesSingleInstance` and the two options-driven
  callback tests pin the configuration; the lock's actual behavior is Story 3.9's native evidence
  row.
- **"Second launch mid-transfer"** is covered only partially. Every `restoreWindow` test proves the
  coordinator is never called, in any `App` state the test harness can construct -- and that is
  what keeps a live transfer untouched, since nothing about restoration depends on transfer state.
  But no test actually drives `fakeCoordinator` into a transferring state first: the harness has no
  notion of transfer state at all, only a call log. The claim this story's tests support is
  "`restoreWindow` never calls the coordinator, unconditionally", not "a mid-transfer session was
  observed unaffected" -- the two happen to imply the same outcome here, but only the first is
  actually tested. `release-evidence.md` row 2 now asks the native smoke check to be run with a
  transfer in progress specifically so the untested half gets human coverage.

## Notes

- The native second-launch smoke check (double-click/"Open with" while FairDrop is already
  running, on real Windows and macOS) is not claimed here. It is Story 3.9's evidence row;
  `release-evidence.md` row 2 carries pass criteria (coordinator-shutdown-once after closing, the
  EXPERIENCE.md meaning of preserved focus, the Windows foreground-lock caveat, and running the
  check mid-transfer) for a person to fill in.
- `docs/fairdrop-architecture.md` item 10 is updated in place to record this story as the
  delivery, per the epic's contract-change rule.
- **Composition on a second launch is inert, not absent.** `main()` calls `newBoundApp` --
  building a real coordinator, network manager, server and QR encoder -- *before* `wails.Run` is
  called at all, and Wails only checks `SingleInstanceLock` once it is running, inside
  `Frontend.Run`. So a second launch does construct all four of those before it is turned away.
  None of that construction starts anything: `transfer.NewCoordinator` only assigns fields,
  `network.NewManager` and `qr.New` only close over dependencies, and `server.New` stores a
  `listen` closure without calling it -- no listener is bound and no beacon is started at
  construction. The second process's `os.Exit(0)` (Windows: after `SendMessage` to the first
  process's hidden window; macOS: after writing its lock file) discards that inert construction
  before anything the coordinator owns could ever compete with the first process. The comments on
  `restoreWindow` (`app.go`) and `singleInstanceLockUniqueID` (`main.go`) now say this precisely,
  rather than the earlier, stronger-than-true "hands off instead of starting a second coordinator,
  listener and beacon" phrasing, which read as though composition itself did not happen.

## Review Triage (step 04, 2026-09-08)

Three context-free layers over `3120590..d6b1baf`, findings deduplicated by claim and action.

- **Patched** (all test or documentation; no behaviour change): the callback pinned only as non-nil
  (Verification Gap, Edge Case Hunter, Blind Hunter -- one finding, three names); the started path's
  silence unpinned (all three); no disclosure assertion on the new log line (Blind Hunter); the
  racing test's thinner assertions and misplaced comment (Blind Hunter, Edge Case Hunter); the
  macOS-only ordering rationale written as universal, the "no second coordinator" overclaim, the
  session-local mutex, the garbled option comment and three stale doc comments (Blind Hunter);
  the second-instance evidence row lacking pass criteria (Blind Hunter); the evidence file and spec
  not reconciled after the racing commit (Blind Hunter); the matrix audit overclaiming the
  mid-transfer row (Blind Hunter).
- **Deferred with owners:** Windows may refuse to foreground the restored window (D-086, 3.9); a
  second launch during a blocked Shutdown is swallowed (D-087, 3.4); the Windows mutex is
  session-local and falls through on any error but "already exists" (D-088, 3.3); macOS exits
  silently on any lock-file failure but contention (D-089, 3.7).
- **Already owned:** `-race` not in a committed gate is D-005 under Story 3.2.
- **Rejected:** sprint status reading `in-progress` while the spec read `in-review` -- the build
  workflow moves sprint status at close-out, which this section is part of.
- **Not a finding:** a foreground-activation workaround (`AlwaysOnTop` toggle) proposed before any
  native observation exists; it waits on D-086's evidence.
