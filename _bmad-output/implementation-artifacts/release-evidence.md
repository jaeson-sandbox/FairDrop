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

**v1.2.0 — PUBLISHED 2026-09-22.** Version bumped on
`epic-8-release-1-2-0` at commit `46e79c700993ff72db1dae8ed48a8e3143705d19`. Verify workflow run
[35688046826](https://github.com/jaeson-sandbox/FairDrop/actions/runs/35688046826) (a push
event whose `headSha` **is** this commit — not an ancestor cancelled mid-run) concluded
**success**, each job read with `gh run view 35688046826 --json conclusion,jobs`, never
`gh run watch`:

| Job | Conclusion |
|---|---|
| `verify (windows-latest)` | success |
| `verify (macos-latest)` | success |
| `Linux adapter verification (not release proof)` | success |

Local gate at the same commit, run in `verify.yml` order: `wails build` (darwin/arm64, self-signed),
`gofmt -l .` (clean), `go vet ./...` (clean), `go tool staticcheck ./...` (clean),
`go test -count=1 ./...` (9 packages, all `ok`), `CGO_ENABLED=1 go test -count=1 -race ./...`
(same 9 packages, all `ok`), `frontend/npm test` (665 tests across 17 files), `frontend/npm run
test:browser` (23 tests across 2 files), and `GOOS=windows GOARCH=amd64 go build ./...` — all
passed. `wails build` flipped the three `frontend/wailsjs/go/...` files to mode 755; they were
`chmod 644`'d back before committing, and the regenerated
`frontend/browser/captures/qr-panel-forced-colors.capture.png` byte-diff from
`npm run test:browser` was discarded with `git checkout --` rather than committed.

**Mutation proof for the version-agreement claim `release_identity_test.go` makes:** setting
`frontend/package.json`'s `version` to `1.2.1` while `wails.json`'s `productVersion` stayed
`1.2.0` failed `TestTheVersionIsStatedOnceAndFollowedEverywhere`, naming both values verbatim
(`frontend/package.json says version "1.2.1" but wails.json says productVersion "1.2.0"`).
Separately, setting `wails.json`'s `productVersion` to `1.2.0-beta` failed
`TestReleaseIdentityAgreesAcrossWailsJSONTemplatesAndMainGo` (`not a plain x.y.z version the
release tag rule can compare against`) and, because the two files then disagreed, also failed
`TestTheVersionIsStatedOnceAndFollowedEverywhere`, again naming both values. Both mutations were
reverted before committing; `release_identity_test.go` itself was not edited.

**Windows was not driven interactively for this release.** No Windows host was available. The
`verify (windows-latest)` job above proves the build, `go vet`, staticcheck, the unit and race
suites, and both frontend suites on a native Windows runner; nobody clicked through the built
app on Windows. `docs/release-policy.md` permits this, and it is stated here rather than implied.

**What was observed on macOS is inherited from Epic 7, not re-observed against this exact
commit.** `_bmad-output/implementation-artifacts/evidence-7-6-prove-the-rebuild-on-both-platforms.md`
recorded, against the built macOS binary at Epic 7's tip `9823abf` (three commits before this
branch's version-and-docs bump, none of them touching application code): both colour schemes
across all five lifecycle states (idle, staged, transferring, done, retained-outcome), two
complete transfers with SHA-256 of the received bytes matched against the source on both runs,
the Tab-then-ArrowDown keyboard path into the browse menu, and the owner's own 2026-09-21
confirmation of the focused browse-menu item carrying the mocha fill, the tint halo and the
focus ring together. All of it is macOS-only, and cited here as evidence about the code this
release ships, not as evidence about this release's own commit — no one has launched
`46e79c7` itself and driven it by hand.

**Not observed / optional / unverified for this candidate — named, not implied as passed:**
- No macOS or Windows binary built from commit `46e79c7` has been launched and driven by a
  person. The nearest observation is Epic 7's tip, `9823abf`, as above.
- The Story 7.11 browse-menu polish's final appearance (hover following the cursor, the
  highlight clearing on mouse-leave, the mocha two-tone ring) — open since Epic 7, still
  unobserved by anyone.
- Every optional manual-check row carried in the table further down this file (browser
  combinations, screen readers, rendered layout through a real high-contrast Windows session, a
  camera reading the QR under forced colors) remains unrun for this release, exactly as it was
  for 1.1.0.
- No tag has been pushed and `release.yml` has not run: there is no built `fairdrop.exe` or
  `fairdrop-macos.zip` for 1.2.0 to size or checksum, and no published GitHub release. Cutting
  the tag is the owner's own act, by this story's instructions — this entry describes a verified
  candidate commit, not a published artifact.

**v1.1.0 — published 2026-09-20, current release.** Final candidate
`54a5f87223f34647aed40344296d79824a33d4bd` passed all three jobs in both PR
[35499365200](https://github.com/jaeson-sandbox/FairDrop/actions/runs/35499365200) and push
[35499362706](https://github.com/jaeson-sandbox/FairDrop/actions/runs/35499362706): native
Windows, native macOS, and the Linux adapter job each concluded **success** on the exact
candidate. It was merged by merge commit `cb40879764a31c512ba835f9c638c959ffa7adfd`, and
annotated tag `v1.1.0` points to that same commit. Release
[35500351435](https://github.com/jaeson-sandbox/FairDrop/actions/runs/35500351435) completed with
all six jobs successful: Linux gate `106050918775`, macOS gate `106050918841`, Windows gate
`106050918926`, Windows build `106052726162`, macOS build `106052726659`, and release
`106053084898`. The non-draft, non-prerelease
[release](https://github.com/jaeson-sandbox/FairDrop/releases/tag/v1.1.0) was published at
2026-09-20 09:03:07 UTC and is marked latest.

| Artifact | Size | SHA-256 |
| --- | ---: | --- |
| `fairdrop.exe` | 13,819,904 | `79b12f642728e09d59600d88c8d36a615b1e1106c26e07e9f5c7d373538f0a98` |
| `fairdrop-macos.zip` | 6,256,840 | `128de947600e6c7485c7dfb1fee57f609e585e987fb097dc33041464d77abecc` |

Both published artifacts and checksum files were downloaded from the release; `shasum -c`
passed, and the files were byte-identical to the inspected CI artifacts. The downloaded Mac
bundle reports both plist versions as 1.1.0, passes strict deep codesign verification, and its
icon decodes to the exact 1024px RGBA master. The downloaded Windows executable passed both
existing PE resource tests through the portable byte parser: all six icon distances were zero
and product identity/version was 1.1.0. That is byte inspection on macOS, not Windows execution.
The locked Mac prevented an additional GUI launch of the downloaded artifact; the local 1.1.0
native launch is recorded in the Story 6.3 evidence.

**v0.3.0 — published 2026-09-17, superseded by v1.0.0.** Built by Release
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
who does run one can fill it in, and against which build. `v1.1.0` is the current published release;
`v0.1.0` is still downloadable and predates most of the later work, so a
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

## Pre-publication security audit (v1.0.0, 2026-09-18)

Run before the v1.0.0 draft was published, because this repository is public and a secret
committed once stays in the history whether or not it is later deleted. Scope was the **entire
history**, not the current tree: 269 commits and 3,099 objects across all refs.

| Checked | How | Result |
|---|---|---|
| Provider credentials | AWS `AKIA`, GitHub `ghp_`/`github_pat_`, Slack `xox*`, OpenAI `sk-`, `BEGIN PRIVATE KEY`, `aws_secret_access_key`, bearer tokens — over every historical diff | **none, ever** |
| Credential files | Every path ever added (`--diff-filter=A`), including files since deleted: `.env`, `.pem`, `.key`, `.p12`, `id_rsa`, `.netrc`, `.npmrc` | **none, ever** |
| Credential-shaped assignments | `api_key`/`password`/`secret` set to a literal of 8+ characters, excluding obvious placeholders | **none** |
| Leaked capability token (AD-9) | 32-hex-character strings in added lines | only patterned fixtures — `0123456789abcdef…`, `ffffffff…`, `11111111…` — never a random token |
| Live artifacts | `gh release`/tag assets | binaries and checksums only |

**Recorded, not remediated.** Three disclosures are real but judged not worth their fix:

- Two commits authored from a university address (D-127). Removing it rewrites every SHA the
  artifact trail cites.
- `C:/Users/jaeso/AppData/Local/Temp/...` paths cited in the Story 3.7 and 3.8 evidence files.
  They name log files no one else can read and expose only a Windows username already public in
  267 commit author lines. A credibility wart rather than a leak: evidence should not cite
  something unreadable.
- `192.168.1.169` in row 1 of the manual table — an RFC1918 address, unroutable and useless off
  that LAN.

**Enabled the same day:** GitHub secret scanning and push protection, so a future accident is
blocked at push rather than found by an audit. Dependabot security updates remain off.

### v1.2.0 publication record

Tag `v1.2.0` points at `2accf9c`, the `main` merge of Epic 8. That commit's own verify run is
**35728453883**, and its three job conclusions, read with `gh run view --json conclusion,jobs`
and never `gh run watch`: `verify (windows-latest)` success, `verify (macos-latest)` success,
`Linux adapter verification` success. The commit cited here **is** the tagged commit, not an
ancestor — `verify.yml` cancels in-flight runs on every push, so a completed green run can
belong to a commit that is no longer the tip, and several runs during Epic 7 completed with the
Windows job cancelled rather than passed. A cancelled job is an absence, not a pass.

The earlier candidate entry above cites run 35688046826 against `46e79c7`, the branch commit.
That run is real and its tree differs from the tagged commit only by this evidence file itself.
It is kept rather than deleted, but the run that proves what shipped is 35728453883.

Release workflow run **35729870204** on tag `v1.2.0`, all jobs success: the shared gate on both
native runners, `build (windows-latest)`, `build (macos-latest)`, and `release`. Each platform
built its own artifact through Wails; no cross-build was used, and none ever is. The workflow
verified both checksums with `sha256sum --check` before upload.

Published assets: `fairdrop.exe` (13,810,688 bytes) with `fairdrop.exe.sha256`, and
`fairdrop-macos.zip` (6,238,224 bytes) with `fairdrop-macos.zip.sha256`.

`release.yml` creates a **draft** deliberately — its own comment reads "publishing a release is
a human act on this public repository." The draft was published on the owner's explicit
instruction to release, given in session on 2026-09-22. Recording who decided, and that the
guardrail was crossed on instruction rather than bypassed, is the point of this line.

**Still unverified at publication, and not softened:**

- **No Windows binary has been driven interactively, for this release or any other in this
  epic.** No Windows host was available. The gate proves the build, `go vet`, staticcheck, the
  unit and race suites, and both frontend suites on `windows-latest`. Nobody clicked through
  the app on Windows.
- **No binary built from the tagged commit `2accf9c` has been launched by a person.** The macOS
  observations this epic recorded — both colour schemes, all five lifecycle states, two
  transfers with SHA-256 integrity verified against the source, the Tab-then-ArrowDown keyboard
  path, and the owner's own confirmation of the focus ring, the menu behaviour and Escape — were
  made against Epic 7 commits, the last at `9823abf`. They are real observations of the same
  code paths, and they are not observations of this artifact.
- The standing optional manual rows remain unrun: browser combinations, screen readers, a real
  Windows High Contrast session, and a camera-scanned QR under forced colors.

None of the above blocks acceptance under `docs/release-policy.md`, which makes manual
observation optional for this personal project. They are listed because the policy's other half
— never convert an unrun check into a pass — is the half that is easy to forget on a green day.

## v1.2.1 — PUBLISHED 2026-09-22

A patch release for a crash: dragging the staged QR code terminated the app on macOS. The
defect predates 1.2.0 — the QR has been a plain `<img>` since the first release — so 1.0.0
and 1.1.0 carry it too.

**Root cause, measured rather than inferred.** The Wails runtime's native drop handler
(`WailsWebView.m`, `performDragOperation:`) reads every `NSURL` off the drag pasteboard with
an empty options dictionary and calls `fileSystemRepresentation` on each unconditionally.
A standalone Objective-C program compiled against Foundation established the behaviour
directly: for a non-file URL, `fileSystemRepresentation` **returns NULL rather than raising**,
and the next line — `[[NSString alloc] initWithCString:NULL encoding:]` — segfaults. The test
program exited **139 (SIGSEGV)**. That is why the app vanished with no Go panic, no
Objective-C exception, and no crash report.

The QR's `src` is a `data:image/png;base64,...` URL, so dragging it puts exactly such a
non-file URL on the pasteboard. The `disableWebViewDragAndDrop` guard sits *after* that loop,
and `EnableFileDrop` cannot be turned off — it is the product's main input path — so no Wails
option avoids it.

**The fix is a workaround, deliberately.** FairDrop stops the drag from ever starting
(`draggable={false}` plus `-webkit-user-drag: none`). The upstream loop is unchanged and is
**still present in Wails v2.16.0**, verified by inspection. The upstream one-line fix would be
`NSDictionary *options = @{NSPasteboardURLReadingFileURLsOnlyKey: @YES};`. This is recorded in
`AGENTS.md` as macOS platform fact 6 so a future Wails upgrade is checked against it.

**Verification.** Tag `v1.2.1` points at `b8840a1`. That commit's verify run is **35754321628**,
read with `gh run view --json conclusion,jobs`: `verify (windows-latest)` success,
`verify (macos-latest)` success, `Linux adapter verification` success. Release run
**35756086035** on the tag: all six jobs success — the shared gate on both native runners,
`build (windows-latest)`, `build (macos-latest)`, and `release`. Each platform built its own
artifact; no cross-build. Published assets: `fairdrop.exe` (13,810,688 bytes) and
`fairdrop-macos.zip` (6,238,229 bytes), each with its `.sha256`.

**A failed gate blocked this release once, and was fixed rather than retried.** Run
**35749628271** against `028fb3e` failed `verify (windows-latest)`:
`TestATeardownThatLosesTwoResourcesReportsBoth` reported "Cancel never returned". That commit
touched zero Go files, 200 local race runs passed, and the test had not failed in the previous
twelve `main` runs — but `docs/release-policy.md` says a failed check is never "retried until
green", so it was diagnosed instead. `fakeTimer.fire()` marks a bound fired but can never mark
it stopped, permanently corrupting the `armed() > stops()` comparison two wait helpers relied
on. The race was reproduced deterministically (5/5) before the fix and pinned by a new
regression test that fails 3/3 if the old helper semantics return. Test-only change, no
assertion weakened. See `evidence-flaky-teardown-bound-test-fix.md`.

**Still unverified at publication, and not softened:**

- **The crash was never reproduced under instrumentation.** The diagnosis is from reading the
  Wails source plus the standalone Foundation experiment above, and the owner confirmed the
  QR can no longer be dragged. Nobody has observed the crash itself in a captured session.
- **No Windows binary has been driven interactively**, for this release or any other. No
  Windows host was available. The gate proves build, vet, staticcheck, unit, race and both
  frontend suites there; nobody clicked through the app.
- **No binary built from `b8840a1` has been launched by a person.**
- The standing optional manual rows remain unrun: browser combinations, screen readers, a real
  Windows High Contrast session, and a camera-scanned QR under forced colors.

The draft was published on the owner's standing instruction to release, given 2026-09-22.
`release.yml` creates a draft on purpose — "publishing a release is a human act on this public
repository" — and recording who decided is the point of this line.

## v1.3.0 — PUBLISHED 2026-09-26

A minor release for Epic 9, "Motion and Clarity": entrance motion throughout; Idle decluttered
with the browse control inside the drop zone; Staged rebuilt around the QR code with the link
revealed only on request; Sending as a progress ring; Sent and Error as one centred card whose
next action fits the failure, where **Try Again** re-prepares the same item. It also fixes a
macOS defect present in every earlier release: a pointer click on Copy Link copied correctly
but never showed "Copied", because WebKit does not focus a clicked button (AGENTS.md fact 3).
Transfer behavior is unchanged from 1.2.1.

**Verification.** Tag `v1.3.0` points at `dd56e96` (the release merge on `main`). That
commit's verify run is **36256415581**, read with `gh run view --json conclusion,jobs`:
`verify (windows-latest)` success, `verify (macos-latest)` success, `Linux adapter
verification` success. Release run **36257594447** on the tag: all six jobs success — the
shared gate on both native runners and Linux, `build (windows-latest)`, `build
(macos-latest)`, and `release`. Each platform built its own artifact; no cross-build.
Published assets: `fairdrop.exe` (13,824,512 bytes) and `fairdrop-macos.zip` (6,241,727
bytes), each with its `.sha256`. Both checksums were re-verified against downloaded copies
of the draft before publication, and the downloaded `fairdrop.app` reports
`CFBundleShortVersionString` 1.3.0.

**A failed gate blocked the epic once, and was fixed rather than retried.** Run
**36253748499** against the docs-only `fc94b36` failed the Linux race step:
`TestTimedOutServerStopFencesNewSessionsUntilTheProductionCallCompletes` saw two
`ServerPort.Stop` calls. The cause was the test harness, not the coordinator: the fake
server handed every `Start` the one event lane its first `Stop` had closed, so a second
session's drainer read the old teardown as its own. Reproduced 5/5 with a probe delay,
pinned by a new harness test that failed first. Test-only change. See
`evidence-fake-server-lane-reuse-fix.md`.

**Driven on a built macOS binary** (the epic tip, and again after the Copied fix): a real
2.4 MB transfer to `curl` as the receiver arrived byte-identical; Show/Copy Link, Send
Another, forced `source_changed` (Try Again → same item re-staged) and forced
`path_not_found` (Choose Another, no Try Again) all behaved as specified. Full record in
`evidence-9-7-prove-epic-9-on-both-platforms.md`.

**Still unverified at publication, and not softened:**

- **Light appearance and Reduce Motion were not observed on a built binary.** Both are system
  settings the orchestrator does not change; they were checked only in rendered Chromium
  previews and through the suites' assertions.
- **No native drag-and-drop was performed** during the walkthrough; staging used the chooser.
- **No Windows binary has been driven interactively**, for this release or any other. The gate
  proves build, vet, staticcheck, unit, race and both frontend suites there.
- **No binary built from `dd56e96` has been launched by a person**; the walkthrough used local
  builds of the epic branch, whose code is identical apart from the version bump and docs.
- The standing optional manual rows remain unrun: browser combinations, screen readers, a real
  Windows High Contrast session, and a camera-scanned QR.

The draft was published on the owner's instruction "Merge and do the release", given
2026-09-26.
