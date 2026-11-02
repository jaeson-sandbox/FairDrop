# Completion alerts implementation evidence

## Matrix audit

| Spec row | Executed proof |
| --- | --- |
| Enabled background send or receive | `TestRealFileHTTPCompletionRequestsOneBackgroundCue` runs source inspection, coordinator, HTTP POST and App observer with exact one-cue count after coordinator shutdown. `TestAppCompletionCueUsesOnlyAuthoritativeCompleteEvent` also sends a complete receive payload through `appObserver` and checks its cue epoch. Mode fields do not enter cue eligibility. |
| Foreground or disabled | `TestCompletionAlertsRequireOptInAndOneBackgroundSuccess` checks publication admission and no later replay. Native macOS and Windows decision tests check foreground suppression again at delivery. |
| Error, incomplete, cancellation, waiting | `TestAppCompletionCueUsesOnlyAuthoritativeCompleteEvent` passes each non-complete event through App and checks that none enters the dispatcher. |
| Held result and rapid lifecycle | Duplicate success is refused. `TestRapidCompletionsReplacePendingWithoutBlocking` sends 1,000 completions while the cue worker is held, proves publication finishes, and checks the newest activation generation is delivered. |
| Delayed delivery and focus return | `TestForegroundReturnFencesDelayedCueAndKeepsNewestSuccess` simulates foreground→away before an old cue is released. `TestAppOptOutAndShutdownInvalidateInFlightNativeCue` checks both App commands advance the native epoch and block in-flight obsolete delivery. The macOS native test exercises a held main-queue slot, opt-out and fresh re-enable. |
| OS slow or unavailable | One worker plus one pending slot bounds outstanding work; `TestUnavailableCueDoesNotStopLaterTransfers` proves a failed cue leaves later transfers usable. The Windows observer has one lazy event-hook thread and no default-off polling or shutdown join. |
| Relaunch and UI state | Backend starts disabled; `GetCompletionAlerts` returns effective process state. App UI tests cover pending hydration, rejected and synchronously thrown reads with retry, backend refusal of requested true, focus during pending write, and remount after opt-in. No storage is used. |

Platform-neutral and macOS native cases ran locally. The Windows test binary cross-compiled locally. The desktop workflow runs the Windows cases on its native Windows runner.

## Review triage

- **Publication lease / slow OS:** moved foreground observation to cached activation state. The coordinator publication path reads only that in-memory state. Windows uses a lazy `SetWinEventHook` observer with a message loop and posted quit; macOS uses AppKit activation notifications. Unknown state fails closed. One worker bounds cue delivery.
- **Stale and newer work:** opt-out, shutdown and activation changes fence pending delivery by generation. The pending slot always retains the newest eligible request; the macOS main-queue slot replaces its pending epoch/focus. The drain/send is structurally nonblocking because all producers share one mutex and the consumer only removes.
- **Platform proof:** macOS native C decision and informational-request seam are exercised behaviorally. Windows has an immediate Win32 seam test for own visible top-level window, foreign/unknown foreground, epoch, class, count and literal taskbar-only flag `2`, plus an observer generation test for foreign-app switches. Cross-compilation is not claimed as Windows API execution.
- **Observer lifecycle and wiring:** Windows clears the cached foreground state on every observer exit, including registration failure and unexpected `GetMessageW` error. A controlled pump test checks unhook, thread-ID cleanup and posted quit. AST callsite pins prove `App.startup` starts native observation and `SetCompletionAlerts` enables it; a default-composition test checks NewApp wires all three real native adapters. The callsite pins are wiring proof, not an observed visible cue.
- **UI:** added backend read hydration with generation fencing, a stable native checkbox, focus-preserving pending behavior and a rendered 44px activation target measured by accessible name.
- **Contracts:** both Wails command signatures are in the Public API inventory; lifecycle grammar is back under Event ordering. README, roadmap, DESIGN, EXPERIENCE and copy registry agree.

## Verification boundaries

- macOS: the final focused Go tests and their `-race` run passed; `go vet ./...` and `go tool staticcheck ./...` passed. A Wails build after the new binding command passed; final full ordered build is the main session's gate. Native C tests record the immediate AppKit request seam. No visible Dock bounce was observed or claimed.
- Windows: `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go test -c -o /tmp/fairdrop-alerts-windows.test.exe .` compiled the native tests. Actual Windows API execution and visible taskbar observation are pending native CI and optional manual review respectively.
- Frontend: before the final synchronous-throw test, `npm test` passed 43 files and 1,378 tests. The new getter-throw, pending-load and remount tests passed in a focused run (3 passed); the main session owns the final full suite. Targeted rendered Chromium test passed at 1024px and 320px. This is a rendered control measurement, not a macOS WebKit manual observation.
- `git diff --check` passed. Regenerated bindings have mode 0644. The main session owns full ordered verification, review, commit, push and CI conclusion audit.

## Canonical mutation proof

Run `python3 scripts/verify-completion-alerts-mutations.py --log-dir <outside-checkout-directory>`. The script copies only repository source into an isolated temporary directory, runs a separate JSON baseline and mutant for every applicable case, and uses `scripts/mutationverdict` to reject compile/setup failures, skips and timeouts while requiring a named assertion failure. The workflow runs it on both native desktop jobs and always uploads complete unique logs from `$RUNNER_TEMP/completion-alert-mutations`.

The final local macOS invocation used `--log-dir /tmp/fairdrop-alert-rereview-mutations-20261102` and passed **13 applicable named kills**. Earlier attempts and complete logs remain outside the checkout under separate `/tmp/fairdrop-alert-*` directories. No generated mutation logs are part of the source diff. The mutation copy restores original source bytes and writes LF explicitly, so one case cannot alter the next Windows baseline; source-text pins normalize CRLF.

| Case | Broken guarantee | Named test failure | Local status |
| --- | --- | --- | --- |
| startup-wiring | App no longer starts native observer | `App.startup no longer calls nativeCompletionStart` | killed (AST wiring) |
| enable-wiring | Opt-in no longer enables native observer | `App.SetCompletionAlerts no longer calls nativeCompletionEnable` | killed (AST wiring) |
| adapter-composition | NewApp no longer wires native cue | `NewApp did not wire the native cue adapter` | killed (default composition) |
| success-gate | Error routed instead of complete | `real HTTP completion did not reach the native cue adapter` | killed |
| opt-in | Disabled completion enters dispatcher | `disabled completion entered alert dispatcher` | killed |
| background-at-event | Foreground completion admitted | `foreground completion requested a cue` | killed |
| stale-work | Opt-out leaves queued work | `disabled alert left stale work queued` | killed |
| latest-slot | Full pending slot drops newer success | `newest pending success was swallowed` | killed |
| mac-guard | Main-queue dispatch replaced | `no longer contains "dispatch_async(dispatch_get_main_queue()` | killed (source mechanism) |
| windows-guard | Taskbar-only flag changed | `no longer contains "flashTray = 0x00000002"` | killed (source mechanism) |
| mac-invocation | Shared inactive-app decision inverted | `informational attention seam was not invoked` | killed (native seam) |
| mac-attention-kind | Critical attention substituted | `informational attention seam was not invoked` | killed (native seam) |
| mac-stale-epoch | Native opt-out epoch ignored | `opt-out failed to fence delayed native work` | killed (native seam) |
| windows-invocation | Wrong window class accepted | `want one finite taskbar-only flash` | Windows CI pending |
| windows-stale-epoch | Native opt-out invalidation removed | `disabled epoch flashed the taskbar` | Windows CI pending |
| windows-focus-transition | Foreign-to-foreign switch invalidates cue | `foreign-to-foreign switch invalidated eligibility` | Windows CI pending |
| windows-observer-exit | Observer exits with stale cached background | `observer exit retained obsolete background eligibility` | Windows CI pending |

The platform-specific native behavioral cases run only on their respective OS. Source mechanism cases run on both. A native CI pass must be matched to the exact branch head before treating Windows and macOS as release proof.

## Orchestrator verification

Three independent reviewers examined the initial diff. All accepted findings were corrections within the story. A stalled foreground query on the publication path was high impact. Stale delivery, UI synchronization, activation area and missing wiring proof were medium impact. Two reviewers rechecked the corrected code. The final fixes catch synchronous binding failures during reload, clear Windows eligibility on observer exit, and pin App startup and enablement to native observation. The orchestrator checked those paths in source and verified that the mutation harness preserves source bytes across Windows line endings.

The final ordered local gate passed in a clean snapshot. Logs are under `/private/tmp/fairdrop-alerts-local-gate-02/`. This includes Wails build and asset/binding drift, gofmt, vet, pinned staticcheck, the full Go suite, native integration, cgo and the full race suite. All 1,379 frontend tests and 137 Chromium tests passed, followed by live browser download and multipart upload checks. Darwin, Linux and Windows builds and Darwin staticcheck passed. The existing native, outbound text Go, outbound text UI, receive-text UI and Receive Again inventories passed with 86, 10, 13, 9 and 19 named kills respectively.

Runtime code, tests, bindings and workflow were compared byte-for-byte against the successful snapshot. The final runner also includes a Windows-specific observer-exit case. Native desktop CI provides the platform-specific evidence for the PR. Dock and taskbar animations have not been visually confirmed.
