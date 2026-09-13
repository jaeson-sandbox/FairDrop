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

Against `c3a80064640db97c94188dce25e39a9211dbccf2` (`main`, 2026-09-12).

| Check | Where it ran | Identity | Result |
| --- | --- | --- | --- |
| Full gate, native Windows | `verify (windows-latest)` | [Verify 34726508196](https://github.com/jaeson-sandbox/FairDrop/actions/runs/34726508196) at `c3a8006` | **success** |
| Full gate, native macOS | `verify (macos-latest)` | the same run | **success** |
| Go adapter suite, Linux | `Linux adapter verification (not release proof)` | the same run | **success** — executes the `O_PATH` branch of `handle_linux.go`, which neither desktop runner compiles, let alone runs; explicitly not release proof |
| Go tests | every runner in the same run | 502 test functions across 8 packages | **pass** |
| Go tests under `-race` with cgo | every runner in the same run | the same, with a compiled cgo probe first so a cgo-less runner fails loudly instead of reporting a clean race run | **pass** |
| Frontend suite | every runner in the same run | 498 tests in 17 files | **pass** |
| `wails build` | both desktop runners in the same run | the real production build, plus a bindings-drift check | **pass** |
| gofmt, `go vet`, pinned staticcheck, line endings | every runner in the same run | — | **pass** |
| Mutation proof | the desktop runners in the same run | 68 mutations, each required to fail a **named** assertion: 57 on every runner, 3 on Linux and macOS, 8 macOS-only (`scripts/verify-native-mutations.sh`) | **pass** |

What this does not say: that the product works. It says the code compiles, builds, and behaves as
its tests describe on both supported operating systems, and that breaking any guarded behaviour
makes a named test fail. Everything a machine cannot observe is below.

## Artifact identity

| Artifact | Size | SHA-256 |
| --- | ---: | --- |
| `fairdrop.exe` | 13,617,152 | `9785404b3b6373b9763756366c04edb71679bff94f7e58ad3b8c7d199c143fa5` |
| `fairdrop-macos.zip` | 4,548,773 | `9f1039bfd75083cda3d26619d507d6efd2da442e73c39494195b7a678a897689` |

Release `v0.1.0`, published 2026-09-10, built from `cc9ce580aa63281b5bd8cc4c6de685b0ac79068f`.

**The published artifact is 36 commits behind `main`** and predates Stories 3.4 through 3.8
entirely. It therefore does not contain bounded lifecycle waits, the visible-failure work, the
reconciled error copy, the native platform matrix, or the directory-stream hardening. Any
observation recorded against `v0.1.0` describes that build and not the current tree. A release
cut from `main` needs its own row here.

## Recorded rather than verified

These are known and open. None is a defect discovered here; each is work an identified story
owns, listed so that reading this file is enough to know what a release would ship with.

| Limitation | Owner | What is unsettled |
| --- | --- | --- |
| HTTP header set for the download response (D-018) | Story 3.10 | CORS, `Accept-Ranges` and `Access-Control-Expose-Headers` are undecided; an error response reachable cross-origin may read as opaque to a receiver page rather than as a coded failure |
| Window theme flash at first paint (D-055) | Story 3.10 | The native window background is not yet read from the OS in Go ahead of the webview's first paint, so a one-frame flash of the opposite theme is possible |
| macOS non-secure-context limit (D-064) | Story 3.10 | WKWebView serves from `wails://`, so browser APIs requiring a secure context are absent; the clipboard already routes through Go, but the limit is not written into the UX contract |
| Windows single-instance fallthrough (D-088) | Story 3.10 | An elevated owner, a tight double launch, or a second logged-in user can leave a first instance undetected |
| Residual error copy (D-035, D-039, D-048, D-097, D-103, D-104, D-106, D-111, D-112) | Story 3.11 | Nine states whose wording does not yet describe what happened — including an ordinary macOS or Linux filename containing `:` failing a whole folder transfer under copy that says FairDrop can use regular files and folders only |
| Rendered accessibility capture (D-065, D-068, D-109) | Story 3.12 | 320 CSS pixel reflow, 200% text, the 44px target floor and forced-colors QR rendering are proved against stylesheet text and the DOM, never against a rendered layout |

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
who does run one can fill it in. `v0.1.0` is the only artifact that exists; see the warning above
about what it predates.

| # | Scenario | Sender | Receiver | Artifact | Date | Reviewer | Result | Evidence |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | One valid folder ZIP download | Windows 11 Home 10.0.26200, `192.168.1.169` (Ethernet; selector verified to rank the private address above a VPN adapter) | Phone on the same Wi-Fi — OS, browser, and whether the ZIP opened were never confirmed | `build/bin/fairdrop.exe` built 2026-09-08 19:28 from the tree committed as `f2ea414` | 2026-09-08 | Jaeson | **pass, sender-side; receiver-side unverified** | Sender log: `transfer-started seq=1` 20:46:51 → `transfer-complete seq=3 bytes=665` 20:46:51 → `transfer-reset seq=4` 20:46:54. Grammar matches the contract exactly; reset fired on the three-second lease. |
| 2 | Single-instance restoration, second launch mid-transfer | — | — | — | — | — | optional / unverified | Launch FairDrop, start a transfer, then launch it again (double-click, "Open with", or a stale shortcut). Observe: the second process exits without opening a window and starts no second coordinator, listener or beacon; the first window unminimises with its in-progress transfer and its keyboard focus unchanged; a screen reader re-announces nothing; and closing the one remaining app still shuts the coordinator down exactly once. On Windows, a flashing taskbar button rather than a window jumping forward is a pass — see D-086 above. Story 3.1 proved all of this against fake runtime seams; only a native launch exercises the real ones. |
| 3 | First launch and the firewall prompt | — | — | — | — | — | optional / unverified | Observe the OS firewall prompt's accessible name, its buttons, and where focus returns after dismissing it. FairDrop never predicts, restyles or duplicates that prompt. |
| 4 | Native input paths | — | — | — | — | — | optional / unverified | Each of: drag-and-drop onto the window, the file chooser, the folder chooser. Observe that the chosen item stages and that the drop target's wording matches what its click actually opens. |
| 5 | One exact file download | — | — | — | — | — | optional / unverified | Scan the QR with a phone on the same Wi-Fi; observe that the received file is byte-identical to the sent one and that the sender reaches Done. |
| 6 | A folder ZIP that opens on the receiver | — | — | — | — | — | optional / unverified | The receiver-side half row 1 never got: that the downloaded ZIP opens on the receiving device with the expected entries. |
| 7 | Progress, cancel, terminal reset, retained outcome | — | — | — | — | — | optional / unverified | Observe that progress moves, that Cancel returns to Idle with the cancellation summary, that a terminal outcome clears on the three-second reset, and that the retained outcome's Dismiss works. |
| 8 | Clean shutdown | — | — | — | — | — | optional / unverified | Close the window mid-transfer; observe the process exits and leaves no listener or beacon behind. |
| 9 | Browser combinations | — | — | — | — | — | optional / unverified | The four sender-to-receiver combinations `EXPERIENCE.md` names. Until each is observed, "supported modern browser" is not a claim this project has evidence for. |
| 10 | Windows assistive technology | — | — | — | — | — | optional / unverified | One keyboard-only transfer with NVDA running: each transition announced exactly once, no duplicate or lost announcement. The routing table and throttle are unit-proved; what a screen reader actually says is not. |
| 11 | macOS assistive technology | — | — | — | — | — | optional / unverified | The same, with VoiceOver. |
| 12 | Rendered layout limits | — | — | — | — | — | optional / unverified | Staged at 320 CSS pixels, at 200% text, and with forced colors on. Story 3.12 intends to capture what a headless browser can; a real window at those settings is the rest. |

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
