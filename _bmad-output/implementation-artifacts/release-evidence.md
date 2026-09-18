# Release evidence

What this file is: the record of what was actually verified before a FairDrop release, and —
just as importantly — what was not. It is maintained by an agent and read by a person.

**The rules, per [`docs/release-policy.md`](../../docs/release-policy.md) (owner decision,
2026-09-11):**

- Passing automated verification is the release gate. Native Windows and macOS runs are
  mandatory; Linux and cross-compilation never substitute for them.
- A claim that a machine verified something carries the evidence to re-check it: the workflow
  run id, the commit it ran against, and each job's own conclusion — read with
  `gh run view --json conclusion,jobs`, never `gh run watch`, which has exited 0 over a failed
  run in this project.
- Manual device, browser, firewall and assistive-technology observations are **optional**. An
  unrun check reads *optional / unverified* and names what would have to be observed. It never
  reads "pending", because nobody owes it; and it is never quietly converted into a pass.
- A failed automated check or a known functional defect still blocks acceptance. It is not
  waived by this policy, retried until green, or hidden behind a weaker assertion.
- Attempts that did not pass stay recorded, as written, below.

Sender-side facts come from the `fairdrop:` lifecycle log the app writes to stderr when launched
from a shell. Receiver-side facts can only come from the person holding the receiving device.

## What machines verified

Against `0db16580` (`main`, tag `v0.3.0`, 2026-09-17), through the Release workflow's own gate --
Release [35169193793](https://github.com/jaeson-sandbox/FairDrop/actions/runs/35169193793), all six
jobs green. The Release run reuses `verify.yml` by `workflow_call` rather than restating
it, so the gate below is the same gate every pull request runs.

| Check | Where it ran | Identity | Result |
| --- | --- | --- | --- |
| Full gate, native Windows | `gate / verify (windows-latest)` | [Release 34734499181](https://github.com/jaeson-sandbox/FairDrop/actions/runs/34734499181) at `7b59c37` | **success** |
| Full gate, native macOS | `gate / verify (macos-latest)` | the same run | **success** |
| Go adapter suite, Linux | `gate / Linux adapter verification (not release proof)` | the same run | **success** — executes the `O_PATH` branch of `handle_linux.go`, which neither desktop runner compiles, let alone runs; explicitly not release proof |
| Go tests | every runner in the same run | 502 test functions across 8 packages | **pass** |
| Go tests under `-race` with cgo | every runner in the same run | the same, with a compiled cgo probe first so a cgo-less runner fails loudly instead of reporting a clean race run | **pass** |
| Frontend suite | every runner in the same run | 498 tests in 17 files | **pass** |
| `wails build` | both desktop runners in the same run | the real production build, plus a bindings-drift check | **pass** |
| gofmt, `go vet`, pinned staticcheck, line endings | every runner in the same run | — | **pass** |
| Mutation proof | the desktop runners in the same run | 68 mutations, each required to fail a **named** assertion: 57 on every runner, 3 on Linux and macOS, 8 macOS-only (`scripts/verify-native-mutations.sh`) | **pass** |
| Rendered accessibility floor | `verify (windows-latest)` and `verify (macos-latest)` | [run 34796962306](https://github.com/jaeson-sandbox/FairDrop/actions/runs/34796962306) at `b8fb09a`, on `epic-3-run-reliably-on-supported-desktops` | **pass** — 8 checks in real Chromium: 320 CSS pixel reflow and 200% text on Staged and Transferring, the 44×44 activation floor on every control, the forced-colors exemption confined to the QR substrate, and the forced-colors capture. New in Story 3.12 and not in `v0.2.0` |
| Native artifact build | `build (windows-latest)` and `build (macos-latest)` in the same run | each runner builds its own through Wails after the gate, never cross-compiled | **success** |
| Checksum re-verified after transit | the `release` job in the same run | `sha256sum --check` runs on the downloaded artifacts before `gh release create`, so the published checksum describes the published file | **success** |

The rendered row is the one line in this table produced by a branch's `verify.yml` run rather than
by the Release workflow's gate, because it postdates `v0.2.0`. It needs no separate arrangement to
become release proof: Release calls `verify.yml` by `workflow_call`, so the next release runs these
same eight checks on both desktop runners as part of its own gate, and this row is replaced by that
run's identity when it does.

What this does not say: that the product works. It says the code compiles, builds, and behaves as
its tests describe on both supported operating systems, and that breaking any guarded behaviour
makes a named test fail. Everything a machine cannot observe is below.

## Artifact identity

**v0.3.0 — published 2026-09-17, the current release.** Built by Release
[35169193793](https://github.com/jaeson-sandbox/FairDrop/actions/runs/35169193793) from
`0db16580` at tag `v0.3.0`, each artifact on its own native runner after the full gate passed on
both. The gate is `verify.yml` reused by `workflow_call`, so it is the same gate every pull request
runs, and it included the rendered accessibility suite on both desktop runners.

| Artifact | Size | SHA-256 |
| --- | ---: | --- |
| `fairdrop.exe` | 13,713,920 | `30120a41ff9c36b7ab566f28d4304c67459f7e9e79d76cdcea2d81dc47ddf4f1` |
| `fairdrop-macos.zip` | 4,585,248 | `fae558e68610b67160643630252217e9e8b440ce1fa93bd02bbc8f4c6fcd08d6` |

First build to carry Stories 3.11, 3.12 and 4.2: nine reconciled error states, the rendered
accessibility floor measured in real Chromium rather than read out of a stylesheet, the ten fixes
the Epic 3 retrospective produced, and the four this week.

The workflow built it as a draft, because making a release visible on a public repository stays a
person's act; the owner published it on 2026-09-17. The Windows artifact was then downloaded from
the published release and `sha256sum --check` passed against its published `.sha256`, so the file a
person gets is the file the runner built. What is downloadable now is this build.

**v0.2.0 — published 2026-09-13, superseded,** built by Release
[34734499181](https://github.com/jaeson-sandbox/FairDrop/actions/runs/34734499181) from
`7b59c373164c8bacdd4ae7c367ad3a231385a474`, each artifact on its own native runner after the full
gate passed on both.

| Artifact | Size | SHA-256 |
| --- | ---: | --- |
| `fairdrop.exe` | 13,704,704 | `7480b1dc6ae19a0df646a600a2d4ac8f5dc655de95195b61b190cd8d56b620f6` |
| `fairdrop-macos.zip` | 4,578,680 | `fe6fff303e77fbf2a44735818667ed18dad526ad57c74dad977f4269f03c2853` |

The workflow built it as a draft, because making a release visible on a public repository stays a
person's act; the owner published it on 2026-09-13. What is downloadable now is this build.

**v0.1.0 — published 2026-09-10, superseded,** built from
`cc9ce580aa63281b5bd8cc4c6de685b0ac79068f`. Still downloadable, and still described here because a
copy someone already has is not recalled by a newer release.

| Artifact | Size | SHA-256 |
| --- | ---: | --- |
| `fairdrop.exe` | 13,617,152 | `9785404b3b6373b9763756366c04edb71679bff94f7e58ad3b8c7d199c143fa5` |
| `fairdrop-macos.zip` | 4,548,773 | `9f1039bfd75083cda3d26619d507d6efd2da442e73c39494195b7a678a897689` |

It was **52 commits behind `main`** when v0.2.0 replaced it, and predates Stories 3.4 through 3.10
entirely: no bounded lifecycle waits, no visible-failure work, no reconciled error copy, no native
platform matrix, no directory-stream hardening, and none of the platform decisions. Any observation
recorded against `v0.1.0` describes that build and not the current tree.

The Windows artifact was downloaded from the published release on 2026-09-13 and
`sha256sum --check` passed against its published `.sha256`, so the file a person gets is the file
the runner built.

Neither checksum is a signature. Both are produced by the same pipeline as the binary, so they
prove a download arrived intact and say nothing about whether that pipeline was tampered with.
Neither artifact is signed beyond the macOS build's ad-hoc signature, which exists only so the app
will launch; there is no notarization and no auto-update.

## Recorded rather than verified

These are known and open, listed so that reading this file is enough to know what a release would
ship with. None is a defect discovered here. Most are work an identified story owns; one is owned
by nobody, because it needs a person with a phone rather than a story, and that row says so and
points at the optional check below where it belongs.

| Limitation | Owner | What is unsettled |
| --- | --- | --- |
| Residual error copy (D-035 … D-112) | Story 3.11, **closed and released in v0.3.0** | Nine states whose wording did not describe what happened. All nine ship in v0.3.0. A copy of `v0.2.0` still refuses a whole folder transfer over an ordinary macOS or Linux filename containing `:`, and still offers to cancel work that cannot be cancelled |
| A completed transfer can report cancelled (D-115) | nobody — needs a contract decision | A stop landing between the handler returning and the socket closing discards a recorded terminal event. The fix approved on 2026-09-15 rested on a premise that did not survive implementation: a recorded terminal is written when the handler stops writing, not when the bytes are flushed, and `finalizingConn` carries no signal separating the two. Narrow and rare; the behaviour is conservative |
| publish spawns a goroutine per event (D-116) | nobody — needs a harness change | An observer that stops returning accumulates one abandoned goroutine per progress snapshot. The cap that fixes it is written and proved by three mutations, and could not be verified on macOS under `-race`: `fakeTimer.fire` runs the most recently armed bound, and the cap widens the window where a publish owns one. Reaches a user only when the UI is already wedged |
| Two tests assert on wall-clock sleeps (D-117) | nobody — test harness | `TestPrepareDistinguishesDeadlineExpiryDuringOpenFromCancellation` and `TestATeardownThatLosesTwoResourcesReportsBoth` both fail intermittently on a loaded macOS runner. What is unreliable is the proof, not the behaviour it covers — stated here so a future red gate is recognised rather than re-diagnosed |
| QR forced-colors exemption, unscanned (D-065) | nobody — row 14 below | `DESIGN.md` gates the exemption on native scan evidence. Story 3.12 produced the rendered capture and the confinement proof; a camera reading that bitmap is an observation a person makes, not a check a runner owes |
| Copy button loses its name after a copy (D-114) | Story 4.1 | A successful copy renames the only control that reaches the capability URL to “Copied” for the rest of the session. Every fix is a design decision — see the id |
| Chooser failure reports the wrong state (D-113) | Story 4.1 | A native chooser that fails to open reports that a transfer stopped partway, for a transfer that never existed |

Story 3.10 closed the four platform decisions this table used to carry (D-018, D-055, D-064,
D-088) on 2026-09-12, and they are in `v0.2.0`. None of them was observed running on the machine
it targets: the runners compile and link the theme read, the instance lock and the wiring dialog,
but never launch a window, read a preference or show a dialog. Each is proved through a seam a
test drives and reasoned from the documented platform API. That distinction is the reason those
rows moved out of this table rather than becoming a claim.

**Windows restoration on a second launch (D-086), decided 2026-09-12.** Restoration means the
existing window is unminimised with its session, transfer and keyboard focus intact, and that the
second process starts no coordinator, listener or beacon and exits. Coming to the *front* is a
platform courtesy Windows may withhold: it refuses `SetForegroundWindow` from a process that is
not in the foreground, and the second instance has already exited by the time the first would ask.
A flashing taskbar button is therefore the expected Windows outcome, not a defect, and the
`AlwaysOnTop` toggle workaround is deliberately not pursued — it fights a deliberate OS protection
to win a cosmetic race. No native second launch has been observed on either platform; the
behaviour above is what Story 3.1 proved against fake runtime seams.

## Optional manual checks

Optional under the release policy. Each row says what would have to be observed, so that anyone
who does run one can fill it in, and against which build. `v0.2.0` is the current release and
carries every story through 3.10; `v0.1.0` is still downloadable and predates most of them, so a
row recorded against it says little about the code today.

Three rows are worth more now than they were: rows 2, 4 and 12 exercise the theme read, the
instance lock and the wiring dialog Story 3.10 added, and those are precisely the three things no
runner has ever executed on the machine they target.

| # | Scenario | Sender | Receiver | Artifact | Date | Reviewer | Result | Evidence |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | One valid folder ZIP download | Windows 11 Home 10.0.26200, `192.168.1.169` (Ethernet; selector verified to rank the private address above a VPN adapter) | Phone on the same Wi-Fi — OS, browser, and whether the ZIP opened were never confirmed | `build/bin/fairdrop.exe` built 2026-09-08 19:28 from the tree committed as `f2ea414` | 2026-09-08 | Jaeson | **pass, sender-side; receiver-side unverified** | Sender log: `transfer-started seq=1` 20:46:51 → `transfer-complete seq=3 bytes=665` 20:46:51 → `transfer-reset seq=4` 20:46:54. Grammar matches the contract exactly; reset fired on the three-second lease. |
| 2 | Single-instance restoration, second launch mid-transfer | — | — | — | — | — | optional / unverified | Launch FairDrop, start a transfer, then launch it again (double-click, "Open with", or a stale shortcut). Observe: the second process exits without opening a window and starts no second coordinator, listener or beacon; the first window unminimises with its in-progress transfer and its keyboard focus unchanged; a screen reader re-announces nothing; and closing the one remaining app still shuts the coordinator down exactly once. On Windows, a flashing taskbar button rather than a window jumping forward is a pass — see D-086 above. Story 3.1 proved all of this against fake runtime seams; only a native launch exercises the real ones. |
| 3 | First launch and the firewall prompt | — | — | — | — | — | optional / unverified | Observe the OS firewall prompt's accessible name, its buttons, and where focus returns after dismissing it. FairDrop never predicts, restyles or duplicates that prompt. |
| 4 | Native input paths | Windows 11 Home 10.0.26200 | — | `build/bin/fairdrop.exe` built 2026-09-18 from the tree committed as `a048aee` | 2026-09-18 | Jaeson | **partial pass — both choosers; drag-and-drop still unverified** | The browse control's menu was opened and each item taken in turn: **File** opened the native file chooser and **Folder** opened the native folder chooser, and the chosen item staged in both cases. This is the first time the menu-to-native-dialog handoff has been executed at all — Story 4.1 proved it against fakes only, and `app_test.go`'s chooser seam returns an error directly. Drag-and-drop onto the window was **not** part of this pass and remains unverified; so does the wording check, though the click that used to contradict it was removed in Story 4.1. |
| 5 | One exact file download | Windows 11 Home 10.0.26200 | A phone on the same Wi-Fi; its OS and browser were not recorded | `build/bin/fairdrop.exe` built 2026-09-18 from the tree committed as `a048aee` | 2026-09-18 | Jaeson | **pass, by inspection rather than by checksum** | A file was staged, the QR scanned from the phone, and the download completed. The received files were opened and their contents read: they matched what was sent. That is a stronger result than the transfer merely completing, and a weaker one than this row's literal wording — no hash was taken on either side, so byte-identity in the strict sense (trailing bytes, encoding, mode) is not claimed. Recorded at the strength it was actually observed, which `docs/release-policy.md` asks for in preference to rounding a real observation up to a clean pass. |
| 6 | A folder ZIP that opens on the receiver | Windows 11 Home 10.0.26200 | A phone on the same Wi-Fi; its OS and browser were not recorded | `build/bin/fairdrop.exe` built 2026-09-18 from the tree committed as `a048aee` | 2026-09-18 | Jaeson | **pass** | A folder was staged, downloaded to the phone as a ZIP, and opened there. This closes the receiver-side half that row 1 left open on 2026-09-08, and it is the second clause of the canonical SPEC's success signal — the first release for which that signal has been observed by a person rather than inferred from the sender log and the archive unit tests. |
| 7 | Progress, cancel, terminal reset, retained outcome | — | — | — | — | — | optional / unverified | Observe that progress moves, that Cancel returns to Idle with the cancellation summary, that a terminal outcome clears on the three-second reset, and that the retained outcome's Dismiss works. |
| 8 | Clean shutdown | — | — | — | — | — | optional / unverified | Close the window mid-transfer; observe the process exits and leaves no listener or beacon behind. |
| 9 | Browser combinations | — | — | — | — | — | optional / unverified | The four sender-to-receiver combinations `EXPERIENCE.md` names. Until each is observed, "supported modern browser" is not a claim this project has evidence for. |
| 10 | Windows assistive technology | — | — | — | — | — | optional / unverified | One keyboard-only transfer with NVDA running: each transition announced exactly once, no duplicate or lost announcement. The routing table and throttle are unit-proved; what a screen reader actually says is not. |
| 11 | macOS assistive technology | — | — | — | — | — | optional / unverified | The same, with VoiceOver. |
| 12 | Native window theme at launch (new in v0.2.0) | — | — | — | — | — | optional / unverified | Launch on a machine set to dark mode and watch the first frame. The window should paint the dark canvas `#1C1916` before the webview renders; a one-frame flash of cream means the OS read returned light. Repeat in light mode. This is the only way to see D-055 working: the runners compile the registry read and the `defaults` call but never launch a window. |
| 13 | Rendered layout limits, in a real window | — | — | — | — | — | optional / unverified | What a runner can measure is now measured on every run — see the rendered row in the machine table above. This row is what is left over: the same three settings driven through the OS rather than emulated, in the real WebView2 and WKWebView rather than in Chromium standalone. Set the display to 320 CSS pixels wide, then to 200% text through Windows Settings or macOS Displays, then turn on Windows high contrast; observe that nothing clips, that no control falls below a comfortable touch size, and that the window repaints in the system palette. A headless browser evaluates `forced-colors: active` because it was told to; a high-contrast Windows session also changes what the webview host paints behind the page, which no runner here has ever exercised. |
| 15 | The browse control, by keyboard and by pointer (new in v1.0.0) | Windows 11 Home 10.0.26200 | — | `build/bin/fairdrop.exe` built 2026-09-18 from the tree committed as `a048aee` | 2026-09-18 | Jaeson | **pass, after one defect found and fixed** | Keyboard: Tab reaches the control, ArrowDown opens the menu, the arrows move between items, and Tab now closes the menu and moves on. That last part **failed on the first build of the day** — Tab left the menu open with focus wrapped back onto the trigger, where ArrowDown only re-opened an already-open menu and read as dead. Cause and fix are in PR #3; the re-test on the rebuilt binary confirmed it. Pointer: a second click on the control dismisses the menu, which is the behaviour the blur guard in `IdleView.tsx` exists for and the one most at risk from that fix. Staged surface: the Copy button reverts from `Copied` to `Copy download link` once focus leaves it (D-114). Worth recording that the only defect found in this release's manual pass was in the keyboard path, which both automated suites report as fully covered. |
| 14 | A camera reading the QR under forced colors | — | — | — | — | — | optional / unverified | The half of D-065 that no runner can produce. `DESIGN.md` permits `forced-color-adjust: none` on the QR substrate “only after native scan evidence confirms it remains readable”, and that gate is still unmet: Story 3.12 retained `frontend/browser/captures/qr-panel-forced-colors.capture.png`, which shows the substrate still painting light-on-dark with its quiet zone intact under forced colors, and proves nothing at all about a camera. Scan a real staged QR with a phone in a high-contrast Windows session and record whether it decoded. Until then the exemption is applied on the strength of an argument, which is recorded here rather than implied. |

## Attempts that did not pass, kept for the record

- **2026-09-04, folder download, first attempt.** The receiving phone reported "download failed"
  and its retry could not succeed, which is expected: the capability URL is single-use, so a retry
  can only get `410`. The sender window had been closed and the app logged nothing at the time, so
  the sender-side outcome is unknown. Every server-side cause was subsequently ruled out on this
  machine with evidence (see the commit `9ca2616` message and `AGENTS.md`): the archive pipeline
  streams the same folders intact, no write deadline exists, `ReadTimeout` provably cannot cancel
  the stream, and the address selector cannot pick the VPN adapter. Not reproduced on 2026-09-08
  under logging (row 1). Treated as unexplained rather than fixed; if it recurs, the sender log now
  says in one line whether the server aborted, completed, or hung.
- **2026-09-04, folder chooser.** The drop zone read "file or folder" but its click opened the
  file-only chooser, and the folder chooser opened at an arbitrary directory. Both fixed in
  `f2ea414` and `9ca2616`; row 1 used the drop target.
