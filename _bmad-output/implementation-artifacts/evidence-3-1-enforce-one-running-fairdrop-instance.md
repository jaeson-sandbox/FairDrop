# Evidence: Story 3.1: Enforce One Running FairDrop Instance

## Implementation Evidence

Gates on this worktree: `gofmt -l .` clean; `go vet ./...` 0; `go test -count=1 ./...` 0
across every package; `go test -count=1 -race ./...` 0 (native runner, `CGO_ENABLED=1`, `CC=gcc`,
so the race build is real rather than silently skipped); frontend Vitest 17 files / 490 tests
unchanged; `wails build` alone afterward, exit 0, producing `build/bin/fairdrop.exe`.

```
$ gofmt -l .
$ go vet ./...
$ go test -count=1 ./...
ok  	fairdrop	0.135s
ok  	fairdrop/internal/network	0.272s
ok  	fairdrop/internal/qr	0.238s
ok  	fairdrop/internal/server	1.654s
ok  	fairdrop/internal/source	0.301s
ok  	fairdrop/internal/stream	0.755s
ok  	fairdrop/internal/transfer	0.495s
$ go test -count=1 -race ./...
ok  	fairdrop	1.846s
ok  	fairdrop/internal/network	1.203s
ok  	fairdrop/internal/qr	1.487s
ok  	fairdrop/internal/server	2.532s
ok  	fairdrop/internal/source	1.365s
ok  	fairdrop/internal/stream	6.086s
ok  	fairdrop/internal/transfer	1.488s
$ cd frontend && npx vitest run
 Test Files  17 passed (17)
      Tests  490 passed (490)
$ cd .. && wails build
Built 'C:\Users\jaeso\Coding\FairDrop\build\bin\fairdrop.exe' in 4.571s.
```

New/extended tests, every one executed and passing:

- `TestAppOptionsEnforcesSingleInstance` (`main_test.go`) -- `SingleInstanceLock` carries the
  literal UUID and a non-nil `OnSecondInstanceLaunch`.
- `TestNewAppWiresTheRealWailsRuntime` (`app_test.go`) -- extended with `unminimise` →
  `wailsruntime.WindowUnminimise` and `show` → `wailsruntime.WindowShow`.
- `TestRestoreWindowBeforeStartupCountsUndeliveredAndLogsWithoutCallingTheRuntime` -- the
  pre-startup race: no runtime call, `undelivered` incremented, exactly one `fairdrop: ` line,
  coordinator untouched.
- `TestRestoreWindowUnminimisesThenShowsExactlyOnceWithTheApplicationLifetimeContext` -- the
  started case: unminimise then show, each exactly once, both carrying the stored
  application-lifetime context; no coordinator call; no emitted event.
- `TestRestoreWindowIgnoresTheSecondLaunchsArguments` -- `SecondInstanceData.Args`/
  `WorkingDirectory` populated with a path and a flag; behavior is identical to the empty case
  and nothing is staged.

All existing option and harness pins (window, drop, background colour, error formatter, bind,
lifecycle hooks, disclosure, publish ordering) stayed green with no edits beyond the seam-list
extension named above.

## Mutation Table

Each deliberate break below was made against the real `app.go`/`main.go`, run against the named
test with `go test -run <name> -v`, observed to fail, then reverted and reconfirmed green
(`gofmt -l .`, `go vet ./...`, `go test -count=1 ./...`).

| Deliberate break | Named failing test | Observed failure |
| --- | --- | --- |
| Remove the nil-context guard in `restoreWindow` | `TestRestoreWindowBeforeStartupCountsUndeliveredAndLogsWithoutCallingTheRuntime` | `restoreWindow called the runtime before startup: [{unminimise <nil>} {show <nil>}]`; `undelivered = 0, want 1`; `logged 0 lines, want exactly 1` |
| Swap the call order (`show` before `unminimise`) | `TestRestoreWindowUnminimisesThenShowsExactlyOnceWithTheApplicationLifetimeContext` | `the calls were show then unminimise, want unminimise then show` |
| Call `show` twice instead of `unminimise` then `show` | `TestRestoreWindowUnminimisesThenShowsExactlyOnceWithTheApplicationLifetimeContext` | `the calls were show then show, want unminimise then show` |
| Change the UUID literal in `main.go` | `TestAppOptionsEnforcesSingleInstance` | `SingleInstanceLock.UniqueId = "...025", want "...024"` |
| Drop `OnSecondInstanceLaunch` from the `SingleInstanceLock` literal | `TestAppOptionsEnforcesSingleInstance` | `OnSecondInstanceLaunch is nil: a second launch would have nothing to hand the window to` |
| Make `restoreWindow` call the coordinator (`_ = a.CancelTransfer()`) after restoring | `TestRestoreWindowUnminimisesThenShowsExactlyOnceWithTheApplicationLifetimeContext` | `restoreWindow reached the coordinator: [Cancel]` |

No survivors: every mutation in the spec's Verification section was caught by exactly the test
named there, and no additional test needed adjustment to catch it.

## Notes

- The native second-launch smoke check (double-click/"Open with" while FairDrop is already
  running, on real Windows and macOS) is not claimed here. It is Story 3.9's evidence row; a
  pending placeholder was added to `release-evidence.md` for a person to fill in.
- `docs/fairdrop-architecture.md` item 10 is updated in place to record this story as the
  delivery, per the epic's contract-change rule.
