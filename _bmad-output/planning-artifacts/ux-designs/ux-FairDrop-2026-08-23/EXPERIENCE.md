---
name: FairDrop
description: Interaction contract for one ephemeral desktop-sender to same-LAN browser-receiver transfer.
status: final
created: '2026-08-23'
updated: '2026-08-23'
sources:
  - '{project-root}/_bmad-output/specs/spec-fairdrop/SPEC.md'
  - '{planning_artifacts}/architecture/architecture-FairDrop-2026-08-22/ARCHITECTURE-SPINE.md'
  - '{project-root}/docs/fairdrop-contracts.md'
  - '{planning_artifacts}/epics.md'
  - '{project-root}/docs/fairdrop-spec.md'
---

# FairDrop — Experience Spine

> Exploration references: [source extract](.working/source-extract.md), [Paper Relay direction](.working/design-directions.html), and [Terracotta Linen themes](.working/color-themes-paper-relay.html). Production references are linked where they illustrate the contract.

**Source precedence:** the canonical `SPEC.md` and its binding architecture/contracts companions control. `epics.md` supplies the approved requirement and story decomposition. The corrected `docs/fairdrop-spec.md` supplies narrative only where non-conflicting. No Phase 1 or deferred-work path is a direct spine source.

## Foundation

V1 is a compact Wails v2 sender for Windows amd64 and macOS that serves one selected regular file or directory to one supported same-LAN browser receiver. React 19, TypeScript, and Tailwind CSS v4 implement the UI; Tailwind supplies no component behavior. Standard OS chrome is mandatory, and `DESIGN.md` owns visual identity.

Windows and macOS are native senders. Windows, macOS, and iPhone are browser receivers only when their combinations pass **Compatibility and Evidence Gates**; iPhone sending is roadmap-only. Thus “Mac → Windows” and “Windows → Mac” mean a native FairDrop sender transferring to the other platform’s browser.

One process owns one live session and receiver. V1 has no receiver app or branded receiver page, account, cloud service, history, settings, telemetry, persistent log, payload archive, resume, or multiple-receiver path. Detailed behavior and boundaries live in the component, interaction, platform, trust, and anti-pattern sections. The approved external promise is `copy.external.promise`; AirDrop is an internal benchmark only and never product or release copy.

## Information Architecture

| Surface | Reached from | Purpose and boundary |
|---|---|---|
| **FairDrop desktop window — Idle** | App open; accepted `transfer-reset` | Native drop target, Select File, Select Directory, firewall preflight, optional retained Done/Error, and staged troubleshooting. |
| **Native file/directory dialogs** | Selection Controls | OS-owned selection; empty result is a quiet cancel. Dialog does not stage itself. |
| **Local staging-pending presentation** | One valid drop or non-empty dialog result | Shows local preparation and **Cancel preparation** while `StageTransfer` is pending; never claims backend STAGED. |
| **FairDrop desktop window — Staged** | Successful `FileMetadata` | Item, QR-primary handoff, readonly URL, exact trust/link guidance, warnings, troubleshooting, and Cancel. |
| **FairDrop desktop window — Transferring** | Accepted `transfer-started` | Honest wire progress/bytes, throughput, and Cancel. |
| **FairDrop desktop window — Done / Error** | Accepted `transfer-complete` / `transfer-error` | Sender-observed terminal outcome. Backend normally clears the session after about three seconds. |
| **Idle with retained outcome** | Matching `transfer-reset` after Done/Error | Same visible Done/Error content becomes a dismissible, non-session current status until the next Stage attempt or Dismiss. It is not history and is never persisted. |
| **Receiver browser/download UI** | First exact-token `GET` | Browser/OS-owned attachment download; no FairDrop receiver page. |
| **Product firewall guidance** | Always in Idle before first Stage | Explains the possible OS prompt and platform-specific allow/deny recovery in document order. |
| **OS firewall prompt** | First inbound-LAN use, platform-controlled | OS-owned permission UI; FairDrop neither restyles nor duplicates it. |
| **Second-instance restoration** | Another FairDrop launch | Shows/unminimizes the existing window and preserves its active session and current focus context. |

No navigation is present. Modal depth is limited to OS dialogs. Warnings, errors, firewall recovery, and troubleshooting are inline.

Surface references: [Idle and local preparation](mockups/key-idle-preparing.html) illustrates Idle and local Stage Pending; [Staged folder](mockups/key-staged-folder.html) illustrates Staged plus the non-terminal discovery warning; [Transferring](mockups/key-transferring.html) illustrates the three transfer-progress modes; [terminal outcomes](mockups/key-outcomes.html) illustrates Done, Error, and the same outcomes retained after reset.

## Backend-Authoritative Lifecycle

Backend lifecycle and protocol remain unchanged.

| Input | Public presentation rule |
|---|---|
| `StageTransfer` pending | Local Stage Pending only; internal STAGING is not public. |
| Successful `FileMetadata` | Initialize `(sessionId, lastSeq=0)` and show Staged. |
| `transfer-started` | Show Transferring only for matching session/increasing `seq`; internal CLAIMING is not public. |
| `transfer-progress` | Accept matching increasing sequence only; ignore duplicates, stale sessions, and post-terminal progress. |
| `transfer-complete` | Accept authoritative final snapshot; show sender-scoped Done. |
| `transfer-error` | Show fixed safe Error. |
| `transfer-reset` | Clear the matching backend session and expose Idle controls. After Done/Error, preserve that outcome visibly as dismissible non-session Idle status; reset itself does not move focus or announce. |

Valid event grammars remain exact: success `started, progress*, final progress, complete, reset`; natural failure `started, progress*, optional final progress, error, reset`; pre-claim cancel after Stage `reset`; post-commit cancel `started, progress*, reset`; pre-ack setup failure/cancel emits none.

## Voice and Tone

Calm, concise, warm, and literal. Brand posture lives in `DESIGN.md`; this section is the single registry for literal product strings. References elsewhere use the stable key without restating the value.

| Stable key | Context | Approved copy |
|---|---|---|
| `copy.external.promise` | Approved external promise | “Send from FairDrop on Windows or Mac to one browser on the same local network—no account or receiver app.” |
| `copy.idle.instruction` | Idle instruction | “Drop one file or folder.” |
| `copy.firewall.preflight` | Firewall preflight | “Your first transfer may ask to allow FairDrop on this local network.” |
| `copy.firewall.windows` | Windows prompt guidance | “Allow FairDrop on Private networks only. Leave Public networks off.” |
| `copy.firewall.macos` | macOS prompt guidance | “Allow incoming connections for FairDrop.” |
| `copy.firewall.windows_recovery` | Windows recovery | “Open Windows Firewall settings and allow FairDrop on Private networks only, then prepare the item again.” |
| `copy.firewall.macos_recovery` | macOS recovery | “Open System Settings → Network → Firewall → Options, allow incoming connections for FairDrop, then prepare the item again.” |
| `copy.stage.pending.file` | File Stage pending | “Preparing your file…” |
| `copy.stage.pending.folder` | Folder Stage pending | “Preparing your folder…” |
| `copy.stage.pending.item` | Native-drop Stage pending before kind is known | “Preparing your item…” |
| `copy.stage.heading` | Staged heading | “Ready to pass along” |
| `copy.qr.instruction` | QR instruction | “Scan this code on the receiving device to start the download.” |
| `copy.qr.alt` | QR accessible-name template | “Download QR code for [item name]” |
| `copy.folder.note` | Folder note | “This folder downloads as a ZIP.” |
| `copy.direct_link.action` | Direct-link action | “Copy download link” |
| `copy.direct_link.helper` | Direct-link helper | “Open this link directly in the receiving device’s browser.” |
| `copy.first_opener.warning` | First-opener warning | “One device only—the first device or software to open this link starts the download. Link previews may use this V1 link before the intended browser.” |
| `copy.network.disclosure` | Network disclosure | “Use FairDrop only on a network you trust. The transfer is not encrypted, so someone monitoring this network may be able to observe it.” |
| `copy.local_copy.disclosure` | Local/no-extra-copy disclosure | “Sent directly over your local network. FairDrop does not upload or store an extra copy. The receiving device keeps the downloaded file.” |
| `copy.copy.confirmation` | Copy confirmation | “Copied” |
| `copy.discovery.warning` | Discovery warning | “Device discovery isn’t available. The QR code and download link still work.” |
| `copy.progress.unknown` | Unknown-total transfer | “Sending — total size unknown” |
| `copy.progress.known_empty` | Known-empty transfer | “Empty file — 0 bytes to transfer” |
| `copy.done.heading` | Done heading | “Transfer finished” |
| `copy.done.body` | Done body | “FairDrop finished sending the item.” |
| `copy.cancel.preparation` | Cancel preparation action | “Cancel preparation” |
| `copy.cancel.preparation_pending` | Pending preparation cancellation | “Canceling preparation…” |
| `copy.cancel.action` | Transfer cancellation action | “Cancel” |
| `copy.cancel.pending` | Pending cancellation | “Canceling…” |
| `copy.cancel.won` | Cancel-winning reset | “Transfer canceled. Ready for another file or folder.” |
| `copy.outcome.dismiss` | Retained-outcome action | “Dismiss” |
| `copy.name.show_full` | Full-name disclosure action | “Show full name” |
| `copy.help.different_lan` | Different-LAN help | “Not downloading? Make sure both devices use the same local Wi-Fi. Guest or isolated networks may block device-to-device traffic. Then cancel and prepare the item again for a fresh link.” |
| `copy.help.receiver_http` | Generic receiver-error help | “Browser says Not Found: the link may be wrong or expired. Locked: another opener claimed it. Gone: the selected item changed. Cancel and prepare the item again for a fresh link.” |

`copy.done.heading` means sender-observed response-stream completion only. It never asserts where the browser saved the item, that iOS Files contains it, or that the receiver opened it.

Use **file**, **folder**, **download**, **ZIP**, **same local network**, **plain HTTP**, and **not encrypted**. **Cloud** and **upload** are allowed only in the truthful negated disclosure above. Do not use **secure**, **private**, **pair**, **sync**, “AirDrop for any device,” “works with every device,” or any claim of receiver identity, storage completion, or universal compatibility.

### Stable public error and warning copy

The codes come from the binding contract. `PublicErrorOf` and the malformed/unknown-error fallback must emit the exact `PublicError.message` below; UI headings and actions are also fixed. Arbitrary adapter text, source paths, and capability tokens never substitute for these strings.

| Code | Visible heading | Exact `PublicError.message` | Sole announcement owner | Recovery |
|---|---|---|---|---|
| `invalid_selection` | Choose one item | “Choose exactly one file or folder.” | Focused inline Error Panel | Use the drop target or one browse action and choose one item. |
| `busy` | Transfer already active | “Finish or cancel the current transfer before choosing another item.” | Focused current-state heading with the message as description | Finish or Cancel; do not replace the active session. |
| `cancelled` | Transfer canceled | “Transfer canceled.” | Polite status during pending; focused Idle cancellation summary when reset wins | Return to Idle; never render as Error. |
| `path_not_found` | Item not found | “That file or folder is no longer available. Choose it again.” | Focused Error Panel | Choose the item again. |
| `path_unsupported` | Can’t use that item | “FairDrop can use regular files and folders only. Choose another item.” | Focused Error Panel | Choose a non-link regular file or folder. |
| `source_changed` | Item changed | “The item changed after it was prepared. Cancel and create a fresh link.” | Focused terminal Error Panel | Return to Idle and prepare it again. |
| `network_unavailable` | Local network unavailable | “FairDrop couldn’t find a usable local network. Connect to local Wi-Fi, then try again.” | Focused Error Panel | Check same Wi-Fi and firewall help, then retry. |
| `server_start_failed` | Couldn’t open a connection | “FairDrop couldn’t open a local transfer connection. Check firewall access, then try again.” | Focused Error Panel | Follow platform firewall recovery, then retry. |
| `qr_failed` | Couldn’t create the QR code | “FairDrop couldn’t create the QR code. Prepare the item again.” | Focused Error Panel | Retry from Idle. |
| `beacon_warning` | Discovery unavailable | `copy.discovery.warning` | One polite status update; focus stays in Staged | Continue with QR/link; this is Warning, not Error. |
| `transfer_failed` | Transfer stopped | “The transfer stopped before FairDrop finished sending. Check the local network and create a fresh link.” | Focused terminal Error Panel | Check network; prepare again after reset. |
| `shutting_down` | FairDrop is closing | “FairDrop is closing. Reopen it to start a transfer.” | Focused inline message if the window remains | Reopen FairDrop. |

## Component Patterns

Behavioral contract; visual specs live under the same names in `DESIGN.md.Components`.

| Component | Use | Behavioral rules |
|---|---|---|
| **App Shell** | Every desktop state | Renders one authoritative lifecycle presentation plus an optional retained terminal outcome in Idle; subscribes once to `transfer-*`; a second launch preserves the session and retained status. |
| **DropZone** | Idle | Uses `OnFileDrop(callback, true)` and inherited `--wails-drop-target: drop`; no DOM drop handler. Rejects zero/multiple paths and stages exactly one. |
| **Selection Controls** | Idle | Semantic `SelectFile()` / `SelectDirectory()`; non-empty result stages immediately; empty result stays quiet. Starting Stage dismisses a retained outcome. |
| **Stage Pending Card** | Local command pending | Identifies item kind and preparation; includes semantic `copy.cancel.preparation`. No QR/session controls or authoritative-state badge. Obsolete promises cannot commit state. |
| **StagedView** | Staged | Built only from successful metadata. QR is primary; URL is fallback. Exposes exact link/trust guidance, warning, recovery help, and Cancel without implying receiver identity or claim. |
| **Item Summary** | Staged, Transferring | Shows sanitized bidi-isolated full name and logical size; folders distinguish logical size from unknown ZIP wire total. Persistent full-name access follows `DESIGN.md`. |
| **QR Panel** | Staged | Prepends `data:image/png;base64,` only at render. The noninteractive image uses `copy.qr.alt`; it never exposes or spells the token. |
| **Direct URL Row** | Staged | Readonly selectable text, not a sender-side activation link. The button uses `copy.direct_link.action`; the helper uses `copy.direct_link.helper` and `copy.first_opener.warning`. |
| **Copy Feedback** | Staged | Label becomes `copy.copy.confirmation`; one polite update, no toast, focus move, lifecycle change, or clipboard clearing. |
| **Trusted-LAN Note** | Staged | Displays the approved not-encrypted and no-extra-copy/receiver-retains-download disclosures beside handoff controls. |
| **Warning Banner** | Staged or Idle recovery | Renders safe `Warning` or firewall/recovery copy. `beacon_warning` remains non-terminal. |
| **TransferView** | Transferring | Appears only after accepted `transfer-started`; never from scan animation, browser navigation, or frontend inference. |
| **Progress Meter** | Transferring | Three modes: known positive determinate; directory/unknown static-pattern unknown; known-empty text-only status with no percentage-bearing progressbar. |
| **Transfer Metrics** | Transferring | Shows actual wire `bytesSent`; throughput is visual only. Known-empty shows 0 bytes and omits meaningless speed/percentage. |
| **Cancel Action** | Stage Pending, Staged, Transferring | While cancellation is pending, the control retains focus, suppresses repeat activation with `aria-disabled="true"`, and changes its visible and accessible label to `copy.cancel.pending`; lifecycle and command authority determine the outcome. |
| **Done Panel** | Done and retained Idle status | Says only that FairDrop finished sending. After reset the same visible node becomes non-session status with Dismiss and remains until dismissal or next Stage. |
| **Error Panel** | Error, retained Idle status, command/validation failures | Uses the fixed table. Terminal Error persists after reset as dismissible non-session status. Focused errors are not simultaneously alerts. |
| **Status Announcer** | Non-focus status only | One pre-mounted `role="status" aria-live="polite" aria-atomic="true"`; never repeats content announced by focus and never becomes an event log. |

### Stage-preparation cancellation

On `copy.cancel.preparation`, keep focus on the control, set its label to `copy.cancel.preparation_pending`, suppress repeat activation, and call `CancelTransfer`. Pre-ack cancellation emits no lifecycle event, so the matching local command pair is authoritative. Remain pending until `CancelTransfer` resolves and the guarded `StageTransfer` promise rejects with `cancelled`; either result may arrive first. Then show Idle. Any later successful or failed Stage result from that obsolete request generation is ignored. A non-`cancelled` command failure uses the fixed error table.

### Cancel race

While Staged/Transferring Cancel is pending, keep metrics readable and focus on the control labeled `copy.cancel.pending`. If `transfer-complete` or `transfer-error` linearizes first, remove cancel-pending feedback and announce only that authoritative terminal outcome. If `transfer-reset` arrives without a terminal event, focus the visible Idle summary `copy.cancel.won` and do not render Error. Command resolution is never announced separately from the winning outcome.

## State Patterns

State composition references: [Idle and local preparation](mockups/key-idle-preparing.html) covers Idle/rest and staging pending; [Staged folder](mockups/key-staged-folder.html) covers ready and `beacon_warning`; [Transferring](mockups/key-transferring.html) covers known-positive, unknown-total, and known-empty transfer states; [terminal outcomes](mockups/key-outcomes.html) covers terminal and retained-outcome states.

| Surface/state | Treatment |
|---|---|
| Idle/rest | DropZone, Selection Controls, firewall preflight, and optional retained terminal status. No transfer history. |
| Idle/retained Done or Error | Visible, dismissible, sessionless current status. Next Stage attempt or Dismiss removes it. No timer and no persistence. |
| Idle/native drag active | Highlight Wails target with solid boundary plus fill; dropping outside does nothing. |
| Idle/invalid drop | Focused safe Error Panel; zero/multiple paths never stage the first. |
| Dialog/open or cancel | OS owns focus; empty selection returns to invoking control with no message. |
| Staging pending | Stage Pending Card plus Cancel preparation. Local command results govern pre-ack completion/cancellation. |
| Staging/cancel pending | Use `copy.cancel.preparation_pending`; retain focus, suppress duplicates, and make no lifecycle-state claim. |
| Stage command failure | Idle with focused safe Error Panel; no QR, URL, terminal lease, or lifecycle event. |
| Staged/ready | Focused Staged heading; QR, link guidance, item, disclosures, recovery help, and Cancel visible. |
| Staged/`beacon_warning` | Warning added and announced once; QR/link remain usable. |
| Staged/copy success | Copy Feedback only; focus/state remain. |
| Transferring/known positive file | Determinate progressbar plus wire bytes and visual throughput. |
| Transferring/directory or unknown total | No `aria-valuenow`; static non-directional pattern, `copy.progress.unknown`, wire bytes, and visual throughput. |
| Transferring/known-empty file | Show `copy.progress.known_empty`; no percentage progressbar. A decorative track may preserve layout. Wait for authoritative Done. |
| Staged or Transferring/cancel pending | `copy.cancel.pending` remains focused and readable; the winning backend event grammar governs. |
| Done | Focused Done heading/panel; sender-observed transport copy only. |
| Error | Focused safe Error heading/panel; fixed code/message only. |
| Reset after Done/Error | Clear matching session and expose Idle controls while preserving the same visible outcome node. No announcement and no focus move; if focus is in the outcome, it remains there. |
| Dismiss retained outcome | Remove sessionless status, focus Idle instruction, and use focus as the sole announcement owner. |
| Receiver/valid first claim | First device or software issuing exact-token GET starts attachment response; sender moves only on accepted backend start. |
| Receiver/wrong method, route, or token | Generic browser 404; sender cannot diagnose it. Sender help covers wrong/expired link and creating a fresh one. |
| Receiver/competing valid claim | Generic browser 423 while listener lives; first opener continues. Sender help explains another opener may have claimed it. |
| Receiver/source changed before headers | Generic browser 410; sender gets fixed `source_changed` Error and recovery. |
| Firewall/allowed | OS prompt returns; focus goes to current Stage Pending or next authoritative heading. |
| Firewall/denied or blocked | If observable through command failure, show/focus applicable network/server Error and platform recovery. If not observable, Staged troubleshooting remains available. |
| Second instance | Restore/show/unminimize existing window; preserve active session, retained status, and logical focus. |
| Offline/different LAN | No cloud fallback. Sender-side same-Wi-Fi/guest-isolation help instructs Cancel and a fresh link. |

Generic 404/423/410 receiver pages are an accepted V1 limitation. No friendly receiver page or protocol change is authorized. Sender help uses `copy.help.receiver_http`.

## Interaction Primitives

- Pointer: native drop or semantic controls. Keyboard: Tab/Shift+Tab and Enter/Space on every action, plus native-dialog behavior. No shortcut-only command.
- Focused state headings use `tabindex="-1"` and are not in ordinary Tab order. No focus trap exists outside OS dialogs.
- State transitions use short opacity/position changes. `prefers-reduced-motion: reduce` removes spatial motion, repeating transforms, sweeps, shimmer, and blink; state swaps are immediate or near-immediate.
- Unknown progress remains understandable without motion through a static non-directional pattern, `copy.progress.unknown`, and live wire bytes.
- Visual progress may refresh on every accepted snapshot. Assistive progress speech is separate: start once; then no more often than every five seconds **and** only after meaningful change (at least 10 percentage points for known totals or 10 MiB of new wire bytes for unknown totals); terminal/error cancels queued progress speech. Throughput is never spoken in progress updates.
- Banned: DOM file drop, drag-only input, hover-only actions, modal warnings, frontend lifecycle/reset timers, automatic clipboard clearing, celebratory animation, tray/always-on-top behavior, and navigation chrome.

### Announcement ownership

Every transition has exactly one owner. Focused content is excluded from live/alert regions.

| Transition | Sole owner | Focus rule |
|---|---|---|
| Native dialog cancel | None | Return to invoking control; no new speech. |
| Stage pending | Focused pending heading | Move after dialog/drop returns; no status duplicate. |
| Stage success | Focused Staged heading | Move once; no live duplicate. |
| Validation/command failure | Focused Error Panel | Normal heading/description; no simultaneous `role="alert"`. |
| `beacon_warning` | Atomic polite Status Announcer | Keep focus in Staged. |
| Copy success | Atomic polite Status Announcer | Keep focus on Copy. |
| `transfer-started` | Focused Transferring heading | Move once; no live duplicate. |
| Throttled progress | Atomic polite Status Announcer | No focus move. |
| Cancel requested | Atomic polite Status Announcer | Keep focus on the `copy.cancel.pending` control. |
| Cancel-winning reset | Focused Idle cancellation summary | One combined message; no live reset message. |
| Complete or terminal Error | Focused outcome heading/panel | Move once; no polite/alert duplicate. |
| Reset after terminal | None | No second focus move; retained node remains mounted. |
| Dismiss retained outcome | Focused Idle instruction | Focus is the only owner. |

An actionable error may use `role="alert"` only if an exceptional implementation path does not move focus to it; the same transition can never use both mechanisms.

## Firewall Preflight and Recovery

The following guidance is always available in Idle before selection controls, in document order:

- **Local network access:** `copy.firewall.preflight`
- **Windows:** `copy.firewall.windows`
- **macOS:** `copy.firewall.macos`

FairDrop does not predict, restyle, or duplicate the OS prompt. When the prompt closes, focus moves to Stage Pending or the next state heading if access is allowed. If the command reports denial or setup failure, focus moves to the applicable Error Panel. Native accessibility verification must record the prompt’s accessible name, buttons, and return order.

Recovery remains available from Idle and Staged:

- **Windows recovery:** `copy.firewall.windows_recovery`
- **macOS recovery:** `copy.firewall.macos_recovery`
- If the app cannot observe denial directly, the `copy.help.different_lan` guidance tells users to check the same local Wi-Fi, guest or client isolation, and firewall permission, then Cancel and create a fresh link.

## Trusted-LAN & Privacy

- V1 uses plain HTTP on a trusted LAN. The capability URL reduces blind discovery but does not provide confidentiality, receiver identity, or protection from a LAN observer.
- QR is primary. `copy.direct_link.action` is for direct opening in the receiving browser, not a promise of cross-device clipboard transfer. Once copied into another app, FairDrop cannot control storage, forwarding, or previews.
- The first device or software to issue the exact-token GET claims V1. Link-preview consumption is an accepted limitation; no protocol hardening is added here.
- The capability token appears only in the local URL/QR and receiver request path. Source paths and arbitrary adapter text never appear in UI, browser errors, mDNS, or persistent storage.
- FairDrop sends directly over the local network and does not upload or store an extra copy of the payload. The original remains on the sender; the receiving browser/device retains its downloaded copy.
- Done is limited to sender-observed transport completion. Browser saving, filename prompts, Files integration, ZIP opening, and subsequent storage are receiver-owned actions.
- Generic receiver failures remain canonical. Sender-side troubleshooting and new-link guidance are the V1 recovery experience.
- No cloud fallback or queue exists. After correcting network/firewall/preview issues, Cancel and prepare the item again to create a fresh capability link.

## Accessibility Floor

- Target WCAG 2.2 AA. Visual contrast/focus tokens and their measured ratios are in `DESIGN.md`; color is never the sole state or action cue.
- Every pointer action has a semantic keyboard equivalent. Focus uses `{colors.focus}` / `{colors.focus-dark}` in authored modes and system focus colors in forced colors.
- The one-owner routing table above binds. The Status Announcer is pre-mounted, atomic, and replaces text rather than appending a log.
- Known-positive progress uses `role="progressbar"`, min 0, max 100, and finite `aria-valuenow`. Unknown total omits `aria-valuenow`. Known-empty has no percentage-bearing progressbar and exposes its literal text status.
- At an effective content width of 320 CSS pixels, use only one-dimensional vertical reflow. At 200% text and with WCAG text-spacing overrides (1.5× line height, 2× paragraph spacing, 0.12em letter spacing, 0.16em word spacing), all information and actions remain visible/reachable without page-level horizontal scroll, overlap, or clipping. Fixed-height content grows.
- Sanitized names use bidi isolation and full-value access described in `DESIGN.md`. Tests cover LTR/RTL mixing, emoji/combining graphemes, bidi controls, long segments, extension visibility, and accessible-name parity.
- Forced colors supersede Terracotta Linen. Use system colors/patterns; only the tested production QR substrate may opt out.
- Reduced motion removes all continuous unknown-progress movement and spatial lifecycle motion while preserving text, pattern, bytes, and state.
- Native dialogs/firewall prompts return focus according to the state and announcement tables. Second-instance restoration does not reset or duplicate speech.

## Responsive & Platform

| Context | Contract |
|---|---|
| Windows/macOS native sender | Same IA/lifecycle; standard chrome and native dialogs/firewall. No OS-choice transfer screen. |
| ≥760px content width | Staged may place details beside QR; Transferring uses one wide progress region. |
| 640–759px | Stack QR above URL/actions; retain Cancel, disclosures, outcome, and help. |
| 640×480 native minimum | Vertical scroll allowed; QR remains scan-ready; actions stay ≥44px. |
| 320 CSS-pixel effective width | One column, vertical scroll only, no clipped information/actions or page-level horizontal scroll. |
| Receiver browser | Browser/OS attachment UI only; no FairDrop receiver page. |
| Theme | Follow OS light/dark; no preference persistence or opposite-theme flash. Forced colors take precedence. |
| V1 roles | Windows/Mac native sender → one supported same-LAN browser receiver. iPhone is receiver-only. |
| Roadmap | Native iPhone sender → Windows receiver is later scope and is not represented as a V1 surface. |

## Compatibility and Evidence Gates

“Supported modern browser” is a gated claim, not completed evidence. Every release record names the sender OS and version, receiver OS and version, browser and version, artifact version or checksum, date, reviewer, and pass/fail result. Missing, stale, ambiguous, or failed evidence blocks that combination from the support promise.

| Priority combination | Required acceptance scenarios | Evidence state |
|---|---|---|
| **Windows FairDrop sender → current iPhone Safari receiver** | QR scan; exact one-file bytes/name; folder ZIP download, valid archive, and user can open it through browser/Files; first-opener and preview-claim behavior; browser prompt; 404/423/410 observations; sender Done wording | Required before primary journey/support claim; not yet verified. |
| **Mac FairDrop sender → current Windows Microsoft Edge receiver** | QR and direct-browser link; exact file bytes/name; valid folder ZIP; first opener; browser prompt; generic failures; sender progress/Done | Required next-phase cross-device gate; not yet verified. |
| Windows FairDrop sender → current Mac Safari receiver | Same file/folder/claim/failure checks | Required before claiming this combination; not yet verified. |
| Mac FairDrop sender → current iPhone Safari receiver | Same mobile file/folder/QR/Files checks | Required before claiming this combination; not yet verified. |

Link-preview evidence intentionally confirms the accepted limitation: a preview agent that issues GET may consume the V1 link. The UI must disclose that result; a test must not reinterpret it as a protected link.

Native accessibility evidence is also a release gate:

| Platform | Assistive setup | Required recorded scenarios |
|---|---|---|
| Windows amd64 | Current supported Windows/WebView2; keyboard; Narrator and NVDA | Both browse actions; dialog cancel; invalid drop; Stage/Start/progress/Cancel/Done/Error/reset order; retained outcome; 5-second speech throttle; firewall allow/deny; forced colors; 320px/200%/text spacing; second-instance restoration; no duplicate listeners. |
| macOS | Supported macOS/Wails WebKit; Full Keyboard Access; VoiceOver | Same lifecycle and retained outcome; native file/folder dialogs; firewall prompt/return; Reduce Motion; Increase Contrast/forced-equivalent behavior; 320px/200%/text spacing; second-instance focus preservation. |

Automated acceptance evidence must additionally prove: token contrast using unrounded calculations; a pre-mounted atomic status region; one owner per transition; focus destination existence before `.focus()`; no duplicate subscriptions after remount; stale/post-terminal suppression; all three progress modes; Cancel preparation local-result guards; cancel race outcomes; and QR scan success in light, dark, and tested forced-colors at 640×480 and 200% text. These are obligations, not claims of completed verification.

## Inspiration & Anti-patterns

- **Internal friction benchmark only:** learn from AirDrop’s low-friction intent, but never use the name or imply native discovery, identity, encryption, confirmation, or receiver completion in external copy.
- **Reject setup rituals:** no account, pairing code, receiver install, device list, or OS compatibility choice.
- **Reject security theater:** do not use lock or shield icons or describe plain HTTP as secure, private, or encrypted.
- **Reject transfer fiction:** no folder percentage from logical size, empty-file 0% fiction, optimistic completion, or animation-driven reset.
- **Keep backend authority visible:** preserve terminal outcomes across reset, and never let animation timing drive lifecycle, cancellation, completion, or reset.
- **Reject utility sprawl:** no history, settings, notifications, queue, tray, cloud fallback, custom receiver page, or compatibility claims without evidence.

## Key Flows

[Primary transfer wireframe](wireframes/flow-primary-transfer-2026-08-23.excalidraw) illustrates the V1 Windows-sender-to-iPhone-browser surface sequence, backend-authoritative branches, accessibility checkpoints, and roadmap boundary.

### Flow 1 — Windows sender to iPhone receiver (Jaeson, leaving home with PDFs from his dad)

1. Jaeson opens FairDrop on Windows, reads the firewall preflight, and drops one folder; Stage Pending takes focus before successful metadata opens Staged with the name, logical size, and `copy.folder.note`.
2. The QR remains primary; the link, first-opener and preview warning, unencrypted-network disclosure, no-extra-copy disclosure, troubleshooting, and Cancel are available.
3. He scans the QR in current iPhone Safari on the same local Wi-Fi; the first exact-token GET starts browser-owned attachment handling and the sender accepts `transfer-started`.
4. **Climax:** the download begins with no account, receiver app, pairing code, or OS-choice step.
5. FairDrop shows static unknown-total ZIP treatment, actual wire bytes, and visual throughput.
6. After the browser completes the download, Jaeson chooses where to keep or open the ZIP in Safari or Files; FairDrop does not claim to observe that action.
7. Sender-observed Done reports only sending completion; reset exposes Idle while the outcome remains until Dismiss or the next Stage.

Failure: discovery warning leaves QR/link usable. Different network, guest isolation, firewall denial, source change, or a preview claim uses the exact sender-side recovery and fresh-link guidance. This flow is a support claim only after its matrix row passes.

### Flow 2 — Mac sender to Windows browser receiver (Jaeson, handing off one document)

1. Jaeson opens the Mac sender, reads firewall guidance, and uses the keyboard to select one file; the OS dialog returns one path, which stages immediately with preparation cancellable.
2. Staged shows the isolated full name, known size, QR-primary handoff, direct-link fallback, disclosures, help, and Cancel.
3. Jaeson opens the link in current Windows Microsoft Edge.
4. **Climax:** accepted `transfer-started` replaces waiting with determinate wire progress.
5. Authoritative final progress precedes complete; Done remains visible through reset until Jaeson dismisses it or starts another item.

Failure: quiet dialog cancel remains quiet. Invalid selection uses fixed focused copy. A preview/other opener may claim V1; sender help instructs a fresh link. This combination is supported only after its matrix row passes.

### Flow 3 — Cancel, fail, and recover (Jaeson, stopping the PDF handoff)

1. During Stage Pending, Jaeson activates `copy.cancel.preparation`; the focused control changes to `copy.cancel.preparation_pending` until the local Stage/Cancel pair settles with no lifecycle event and returns Idle.
2. In a later Staged or Transferring session, he activates Cancel while current status remains readable.
3. **Climax:** if reset wins without terminal, focused Idle presents `copy.cancel.won`.
4. If complete or error wins, only that authoritative outcome appears; reset preserves it without a second focus move, after which Jaeson may Dismiss or stage another item.

Failure: command errors use the exact code table. Cancellation is never Error, and the frontend never arbitrates the backend linearization race.

### Flow 4 — Verify a native release (Morgan, release operator)

1. Morgan launches Windows and macOS artifacts, checks preflight before the native firewall prompt, and records allow/deny behavior, focus return, recovery copy, and assistive output.
2. Morgan completes the compatibility and accessibility matrices, including Windows sender → current iPhone Safari and Mac sender → current Windows Edge.
3. During an active session, Morgan launches FairDrop again.
4. **Climax:** the existing window is restored with the session and focus context intact; no competing listener or process starts, and no speech is duplicated.
5. Morgan verifies cancellation, retained terminal outcome after reset, Dismiss/next-Stage clearing, and clean shutdown.

Failure: missing or failed native/browser/accessibility evidence blocks the affected support claim or release; it is never described as verified.

### Requirement-to-flow coverage

| Source requirement names | Covered by |
|---|---|
| `CAP-1`–`CAP-6`, `FR1`–`FR11`, `FR13`–`FR21`, `UX-DR1`–`UX-DR8`; Epic 1 | Flows 1–3 plus component/state/lifecycle/privacy/accessibility contracts. |
| `FR12`; Epic 2 | Flow 1 plus unknown-total ZIP rules. |
| `CAP-7`, `FR22`, `NFR12`–`NFR14`; Epic 3 | Flow 4 plus compatibility/accessibility evidence gates. |
| `NFR1`–`NFR11` | Cross-cutting lifecycle, privacy, progress, input, accessibility, and platform rules. |
