# Evidence: Story 3.10: Settle the Release-Blocking Platform Decisions

Baseline: `572fc9aea4b96785ea27034bce1426290f4618d7`.

## The three decisions

All three Ask First items were answered by the owner on 2026-09-12, before implementation.

**D-018 — all three headers.** CORS on the error path, `Access-Control-Expose-Headers:
Content-Disposition` on the authorized response, and `Accept-Ranges: none`.

**D-088 — a per-user backstop lock**, rather than recording the residual case or adding a message
that would need Story 3.11's copy.

**D-107 — a native message box**, chosen over a written crash line. This was not the recommended
option: a dialog is new platform code on a path production cannot reach, and it shows rather than
records. It is implemented behind a seam a test drives, its message is a fixed literal, and the
panic continues afterwards so a developer still gets the runtime's stack trace.

## What each id required, and how it is defended

| id | Change | The test that fails without it |
|---|---|---|
| D-018 | `Access-Control-Allow-Origin` on `writeStatus`; `Access-Control-Expose-Headers` and `Accept-Ranges: none` on the authorized response | every response assertion, through the widened `renderResponse` |
| D-055 | the OS theme read before the options are built; `canvasFor` returns the matching token | `TestAppOptionsBackgroundTracksTheCanvasToken` (both subtests), `TestTheNativeThemeProbeIsWiredToTheOptions` |
| D-064 | `EXPERIENCE.md` platform limit and Responsive & Platform row | documentation; `AGENTS.md` already carried the implementer half |
| D-088 | per-user advisory lock under `os.UserConfigDir()`, checked before `wails.Run` | `TestTheInstanceLockIsExclusiveWithinThisUser`, `TestAnUnheldInstanceLockLeavesTheAppUncomposed` |
| D-107 | `reportWiringPanic` with a build-tagged dialog seam | `TestAWiringPanicReachesTheUserBeforeItKillsTheProcess`, `TestNoWiringPanicShowsNothing` |

## A red `main`, caused by Story 3.8 and found during this story

The first thing this story ran into was not its own. `main` went red on windows-latest after the
Story 3.9 merge: `TestAuthorizeClaimCommitsAndPublishesStarted` reported the claim handshake as
`[network.StopBeacon timer.AfterFunc …]` against a `[timer.AfterFunc network.StopBeacon …]` it has
asserted since Story 3.4.

`callBounded` arms its timer before launching the call and carries a comment saying why: *"arming
after would race the spawned goroutine for which one logs first, making the adapter-call order
nondeterministic for no reason."* Story 3.8's `callAdapterBounded`, three functions below it in the
same file, launched inside the lock and armed afterwards — reintroducing exactly that race.

Thirty local iterations of that test pass with either ordering on this Windows host, so a
behavioural regression test would only fail on the scheduler's bad days, which is how this reached
`main` in the first place. The guard is structural instead:
`TestEveryBoundedCallArmsItsBoundBeforeLaunching` parses `coordinator.go` and requires both
functions to call `boundTimer` before their `go` statement. Reversing either order fails it with
the file and line of both positions.

## Mutation table

| # | Mutation | Result |
|---|---|---|
| M1 | `callAdapterBounded` launches before arming | KILLED — `callAdapterBounded launches its call at … before arming its bound at …` |
| M2 | `callBounded` launches before arming | KILLED — same test, naming `callBounded` |
| M3 | the dark canvas branch returns the light token | KILLED — `BackgroundColour = {R:247 …}, want {R:28 …} (the dark --color-canvas)` |
| M4 | the production path passes a constant instead of the native probe | KILLED — `appOptionsWithLockProbe does not pass nativeOSPrefersDarkTheme` |
| M5 | the instance lock stops being exclusive | KILLED — `a second holder took the lock while the first still held it` |
| M6 | an unheld lock composes anyway | KILLED — `a process that lost the instance lock still composed a coordinator` |
| M7 | the wiring panic is swallowed after the dialog | KILLED — `the panic was swallowed: a half-composed FairDrop would keep running` |
| M8 | the dialog shows what the panic carried | KILLED — both the fixed-message assertion and the AD-9 disclosure assertion |

## Decisions inside the implementation worth recording

**The backstop cannot simply exit.** Wails' single-instance handoff is what restores the existing
window, and it runs inside `wails.Run` — after this lock is taken. A process that exited on losing
the lock would take the window restoration with it on the *ordinary* path, where Wails' own
mechanism works perfectly. So a process that loses the lock still calls `wails.Run`; it just does
not compose. On the ordinary path Wails exits it before any window appears. Only in the two
fallthroughs D-088 describes does that window reach a user, and then every command answers that
FairDrop is not ready — which is the point: a second listener and a second beacon are what must not
happen.

**Advisory, not a lock file's existence.** A file left behind by a process that crashed would block
every later launch forever. An advisory lock on an open descriptor is released by the operating
system when the holder dies, which the test asserts by closing the first descriptor and taking the
lock again.

**macOS reads the theme through `defaults`, not the plist.** `cfprefsd` caches
`.GlobalPreferences.plist`, so a direct read can return a stale value. `defaults` exits non-zero in
light mode because the key is absent — that is the expected light answer, not an error.

**The dialog does not swallow the panic.** A recovered wiring defect would leave a half-composed
process running, and the stack trace the runtime prints is what a developer needs. The dialog is
for the person who double-clicked, not instead of the diagnosis.

## Verification

Read stage by stage:

- `gofmt -l .`; `go vet ./...`; `go tool staticcheck ./...` — clean
- `go test -count=1 -timeout 300s ./...` — 8 packages ok
- `go test -count=1 -race` over the changed packages — ok
- `GOOS=darwin GOARCH=arm64 go vet ./...` and the linux equivalent — clean
- `cd frontend && npx vitest run` — 498 passing
- Native CI read with `gh run view --json conclusion,jobs`

**Not observed, and not claimed.** No Windows registry read, no `defaults` call, no `MessageBoxW`
and no `osascript` dialog has been seen on the machine it targets. The native runners compile and
link all of it; what they do not do is launch a window, read a theme, or show a dialog. The theme
seam, the lock and the dialog are each proved through a seam a test drives, and the platform half
of each is reasoned from the documented API rather than observed. `release-evidence.md` carries
that distinction.

## Deferrals

None.
