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

The native CI jobs built FairDrop and ran platform-specific tests on macOS and Windows. The macOS tests record the informational AppKit request at the native call boundary. The Windows tests exercise the foreground observer and record the window and flags passed to the flash operation. The rendered checkbox, real HTTP transfer and recorded native requests are separate pieces of evidence. Visual confirmation of the Dock bounce and taskbar flash on a physical desktop remains open.

## Canonical mutation proof

Run `python3 scripts/verify-completion-alerts-mutations.py --log-dir <outside-checkout-directory>`. The script copies only repository source into an isolated temporary directory, runs a separate JSON baseline and mutant for every applicable case, and uses `scripts/mutationverdict` to reject compile/setup failures, skips and timeouts while requiring a named assertion failure. The workflow runs it on both native desktop jobs and always uploads complete unique logs from `$RUNNER_TEMP/completion-alert-mutations`.

The final local macOS invocation used `--log-dir /tmp/fairdrop-alert-rereview-mutations-20261102` and passed **13 applicable named kills**. Earlier attempts and complete logs remain outside the checkout under separate `/tmp/fairdrop-alert-*` directories. No generated mutation logs are part of the source diff. The mutation copy restores original source bytes and writes LF explicitly, so one case cannot alter the next Windows baseline; source-text pins normalize CRLF.

| Case | Broken guarantee | Named test failure | Verification result |
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
| windows-invocation | Wrong window class accepted | `want one finite taskbar-only flash` | killed on native Windows CI |
| windows-stale-epoch | Native opt-out invalidation removed | `disabled epoch flashed the taskbar` | killed on native Windows CI |
| windows-focus-transition | Foreign-to-foreign switch invalidates cue | `foreign-to-foreign switch invalidated eligibility` | killed on native Windows CI |
| windows-observer-exit | Observer exits with stale cached background | `observer exit retained obsolete background eligibility` | killed on native Windows CI |

The platform-specific native behavioral cases run only on their respective OS. Source mechanism cases run on both. A native CI pass must be matched to the exact branch head before treating Windows and macOS as release proof.

## Orchestrator verification

Three independent reviewers examined the initial diff. All accepted findings were corrections within the story. A stalled foreground query on the publication path was high impact. Stale delivery, UI synchronization, activation area and missing wiring proof were medium impact. Two reviewers rechecked the corrected code. The final fixes catch synchronous binding failures during reload, clear Windows eligibility on observer exit, and pin App startup and enablement to native observation. The orchestrator checked those paths in source and verified that the mutation harness preserves source bytes across Windows line endings.

The final ordered local gate passed in a clean snapshot. Logs are under `/private/tmp/fairdrop-alerts-local-gate-02/`. This includes Wails build and asset/binding drift, gofmt, vet, pinned staticcheck, the full Go suite, native integration, cgo and the full race suite. All 1,379 frontend tests and 137 Chromium tests passed, followed by live browser download and multipart upload checks. Darwin, Linux and Windows builds and Darwin staticcheck passed. The existing native, outbound text Go, outbound text UI, receive-text UI and Receive Again inventories passed with 86, 10, 13, 9 and 19 named kills respectively.

Runtime code, tests, bindings and workflow were compared byte-for-byte against the successful snapshot. The final runner also includes a Windows-specific observer-exit case. Native desktop CI provides the platform-specific evidence for the PR. Dock and taskbar animations have not been visually confirmed.

## Hosted verification of the implementation

Commit `7dbceb62dc305683e6652383456cb3f346391283` passed [Verify run 41528801333](https://github.com/jaeson-sandbox/FairDrop/actions/runs/41528801333). The Windows, macOS and Linux jobs each finished with conclusion `success`. Both desktop jobs passed their completion-alert proof and artifact upload steps. Windows also verified its executable resources, and macOS passed the native launch smoke check.

The archived JSON transcripts matched the full applicable inventory on each desktop. Replaying the repository's structured verdict tool against those artifacts confirmed all 14 Windows and 13 macOS baseline/kill pairs. The union covers all 17 canonical cases. The reconciliation commit changes only this evidence and the story status.

## Windows focus failure after reconciliation

The docs-only head `29a25e2` failed the frontend suite in [Verify run 41541544209](https://github.com/jaeson-sandbox/FairDrop/actions/runs/41541544209), Windows job, at `App.receiveAgain.test.tsx:68`. The completed receive section was mounted, but `document.activeElement` was `body`. The full failed job transcript is preserved at `/private/tmp/fairdrop-alerts-tools/ci-reconciliation-failed.log`; this is a failed job, not a hosted verification pass for that head.

The cause is a focus request ordering race in `App.tsx`. When a receive reaches waiting, the transition effect queues its waiting-heading request. If started and complete events then cause the outcome to render before that request is consumed, the same passive-effect flush queues the newer outcome request and unconditionally clears the older request. The clear wins, and unmounting the waiting view leaves focus on `body`. Completion-alert hydration adds renders but does not own the focus decision; the interleaving is possible in ordinary transfer event delivery.

`App.focus.test.tsx` now drives that exact render sequence: it advances the mocked controller from waiting to outcome when the waiting transition queues focus. **Before the fix**, the new named test failed with the outcome mounted and focus on `body`; the complete red log is `/private/tmp/fairdrop-alerts-tools/ci-focus-remediation-20261102-red/focused.log`. `App.tsx` now clears only the request that its focus effect observed, retaining a newer queued request. The focused App tests passed, `npm run build` passed, and the full frontend suite passed **43 files, 1,380 tests**. Full logs are under `/private/tmp/fairdrop-alerts-tools/ci-focus-remediation-20261102-green/`.

The `clear-new-focus` mutant restored the unconditional clear and the named regression failed (`exit 1`, `keeps the newest focus request when a receive completes before its waiting focus commits`). Its complete log is `clear-new-focus-mutant.log` in that green evidence directory. The case was then added to the canonical Receive Again frontend inventory and its explicit inventory test. `python3 -m unittest scripts/verify_receive_again_mutations_test.py` passed, followed by the full `python3 scripts/verify-receive-again-mutations.py --log-dir /private/tmp/fairdrop-alerts-tools/receive-again-canonical-20261102-aVczlI`: **20/20 named kills**, including `clear-new-focus`. That directory retains unique complete baseline and mutant logs plus `runner.log`. This local proof covers focus request ordering; the Windows CI result on the corrected head remains to be observed.

The corrected runtime and regression tests passed the full ordered local gate. The Go suite, cgo/race checks, and all 1,380 frontend tests passed. The first Receive Again mutation pass stopped before a baseline test began. Its Vitest worker startup timed out across a 674-second Mac maintenance sleep. The full failure log is retained under `/private/tmp/fairdrop-alerts-local-gate-03/receive-again-mutations/stale-work-fence-baseline.log`. A fresh run from that stage passed all 20 named mutations, all 137 rendered tests, both live browser checks, native mutations, cross-builds, platform staticcheck and outbound text inventories. Complete outputs are in `/private/tmp/fairdrop-alerts-local-gate-03/` and `/private/tmp/fairdrop-alerts-local-gate-03-resume/`. Runtime code, tests, workflow and final canonical scripts matched the verification snapshot byte for byte. The PR records CI conclusions for the corrected head.
