# FairDrop UX Accessibility Review

Date: 2026-08-23  
Lens: WCAG 2.2 AA plus practical Windows/macOS desktop assistive behavior  
Artifacts reviewed: full `.memlog.md`, `DESIGN.md`, `EXPERIENCE.md`, `.working/source-extract.md`, the 93-element primary-flow Excalidraw file, the Paper Relay direction board, the Terracotta Linen theme board, and the confirmed canonical SPEC, architecture spine, binding contracts, epics/stories, corrected original spec, Phase 1 contract, deferred work, and finalize gaps.

## Verdict

**Not ready to claim WCAG 2.2 AA yet.** The spine has a strong base: native browse actions give drag/drop a keyboard equivalent; target size, focus colors, safe-error boundaries, session correlation, reduced-motion intent, and known-versus-unknown progress are substantially better specified than the canonical source floor. The blocking problem is that the backend's three-second terminal lease is currently treated as a three-second information lease. Error and success content can disappear, and focus can be moved again, before a disabled user can read or review it.

There are **15 findings: 1 critical, 4 high, 9 medium, and 1 low**. Thirteen are contract defects to resolve in the spines; two are implementation verification needs. The three highest-priority corrections are:

1. Decouple the backend terminal-session reset from how long the outcome remains perceivable.
2. Repair the light-mode non-text contrast tokens and the explicit 2.933531:1 progress pair.
3. Define one announcement/focus owner per transition so focused headings, live regions, and alerts do not speak the same event twice.

WCAG references used in this review: [2.2.1 Timing Adjustable](https://www.w3.org/WAI/WCAG22/Understanding/timing-adjustable.html), [1.4.10 Reflow](https://www.w3.org/WAI/WCAG22/Understanding/reflow.html), [1.4.11 Non-text Contrast](https://www.w3.org/WAI/WCAG22/Understanding/non-text-contrast.html), [2.4.3 Focus Order](https://www.w3.org/WAI/WCAG22/Understanding/focus-order.html), and [4.1.3 Status Messages](https://www.w3.org/WAI/WCAG22/Understanding/status-messages.html).

## Findings

### A11Y-01 — Critical — Contract defect: the three-second reset is an inaccessible information timeout

**Location:** `EXPERIENCE.md` → Information Architecture row **Done / Error**; State Patterns rows **Done**, **Error**, and **Reset**; Component Patterns rows **Done Panel**, **Error Panel**, and **Status Announcer**; Interaction Primitives focus destinations; Backend-Authoritative Lifecycle row `transfer-reset`; Key Flows 1–2. Canonical constraint: `docs/fairdrop-contracts.md` → Command and state table, **Reset timer**, and Event ordering.

**Trigger:** A slow reader, magnifier user, screen-reader user, user with a cognitive disability, or anyone interrupted after transfer completion/failure gets about three seconds before the backend reset removes the visible outcome and moves focus to Idle. The only promised residue is visually hidden live-region text.

**Why it fails:** The timer is controlled by FairDrop, is neither essential nor a real-time exception, and the user cannot turn it off, adjust it, or extend it. A hidden string is not an equivalent review path for sighted low-vision or cognitive users. For Error, the disappearance can remove actionable information before it can be understood. Programmatically moving focus to Done/Error and then again to Idle compounds the interruption. This conflicts with WCAG 2.2 SC 2.2.1 and undermines 2.4.3.

**Concrete fix:** Keep the binding lifecycle unchanged: matching `transfer-reset` still clears the session and makes the transfer controls Idle. Separately preserve the last terminal outcome as a visible, non-session status in Idle until the next Stage attempt or explicit dismiss. It must remain programmatically associated and reviewable, not just in a live region. Do not force a second focus move at the three-second reset while focus is on or within the terminal outcome; either keep the retained outcome node/focus target mounted or move focus only after an explicit user action. This is transient current status, not history or persistence, so it does not violate the zero-state contract.

**Consequence if shipped:** Users can miss the only success/error explanation, lose their reading position, and be unable to recover the information.

### A11Y-02 — High — Contract defect: the exact Terracotta Linen component palette does not meet its non-text contrast claim

**Location:** `DESIGN.md` → Colors, “controls/focus ≥3:1”; Components rows **DropZone**, **Selection Controls**, **Progress Meter**, **QR Panel**, and **Direct URL Row**; frontmatter `colors.border`, `colors.progress`, and `colors.drop`.

**Measured evidence:**

| Pair | Ratio | Use |
|---|---:|---|
| `border` `#CDBDAE` / light canvas `#F7F0E7` | 1.616983:1 | Drop/control boundaries against the app |
| `border` `#CDBDAE` / light surface `#FFFAF4` | 1.761489:1 | Boundaries against component interiors |
| light progress `#C65B31` / light track `#F1D0BD` | 2.933531:1 | Determinate progress distinction |

W3C explicitly treats 3:1 as a threshold that must not be passed by rounding. The rest DropZone boundary is required to identify where a native drop is accepted, and the progress fill is required to perceive the graphical value. Both are covered by SC 1.4.11. The calculated focus pairs pass comfortably; the defect is not the focus token.

**Concrete fix:** Introduce a separate `control-border`/`drop-border` token that reaches at least 3:1 against both adjacent light surfaces, and revise one side of the light progress fill/track pair to exceed 3:1 with margin. Do not globally darken `border`; decorative dividers may remain subtle. Update the “verified pairs” sentence with the actual component-adjacent pairs and an automated, unrounded contrast test.

**Consequence if shipped:** The marked drop target and progress value can be imperceptible to low-vision users despite an explicit AA claim.

### A11Y-03 — High — Contract defect: focus, polite status, and alert are competing announcement owners

**Location:** `EXPERIENCE.md` → Component Patterns rows **Status Announcer** and **Error Panel**; State Patterns rows **Staged/ready**, **Done**, **Error**, and **Reset**; Interaction Primitives focus destinations and progress announcement bullet; Accessibility Floor bullets for `aria-live` and alert treatment.

**Trigger:** Staged, Transferring, Done, Error, validation failure, or reset enters. The spine both moves focus to the new heading/message and asks the live region or alert to announce the same transition.

**Why it matters:** A focus change already exposes the destination through the accessibility API. Adding the same text to `aria-live`, especially `role="alert"`, commonly produces duplicate or interleaved speech in NVDA/JAWS/VoiceOver. W3C's status-message guidance says content that receives focus is a change of context and does not also require status-message treatment; it also warns against making applications too chatty.

**Concrete fix:** Add an announcement-routing table. For each transition, nominate exactly one primary mechanism:

- Focused lifecycle/outcome heading: announce through focus; do not duplicate the heading in the live region or alert.
- Copy feedback, cancel-pending feedback, nonfatal warning, and throttled progress: keep focus; use one pre-mounted atomic polite status.
- Actionable error that does **not** receive focus: use `role="alert"` once. If the Error Panel receives focus, give it a normal heading/description and no simultaneous alert.
- Reset/cancel: speak one combined result such as “Transfer canceled. Ready for another file or folder,” not separate reset and focus announcements.

Also require the live region to be mounted before updates and specify whether `role="status"`/`aria-live="polite"` plus `aria-atomic="true"` is used.

**Consequence if shipped:** Screen-reader users hear duplicate, interrupted, or out-of-order state information and can lose the actual error or terminal outcome.

### A11Y-04 — High — Contract defect: known-empty progress is explicitly unresolved

**Location:** `EXPERIENCE.md` → Component Patterns row **Progress Meter**; State Patterns row **Transferring/known-empty file**; Accessibility Floor progressbar bullet; `.working/finalize-gaps.md` item 1. Canonical shape: `docs/fairdrop-contracts.md` → Canonical domain values and Event ordering (`totalKnown=true`, `totalBytes=0`, `percent=0`).

**Trigger:** The receiver claims a zero-byte regular file.

**Why it matters:** Treating the state as determinate exposes a meaningless 0%; treating it as indeterminate implies an unknown amount of work even though the total is exactly known. The current accessibility contract only defines determinate and indeterminate ARIA, leaving no valid name/value/state for this canonical third case.

**Concrete fix:** Define known-empty as a third presentation: no percentage-bearing progressbar; show and announce “Empty file — 0 bytes to transfer” (or approved equivalent) while the backend completes, then the normal Done outcome. If a meter remains for layout stability, mark that visual track decorative and expose the text status, not `aria-valuenow="0"` as progress toward a nonzero total.

**Consequence if shipped:** Assistive output can say “0 percent” immediately before “complete,” or an indeterminate state can falsely imply unknown work.

### A11Y-05 — High — Contract defect: Stage Pending has no user-operable cancellation path

**Location:** `EXPERIENCE.md` → Component Patterns rows **Stage Pending Card** and **Cancel Action**; State Patterns row **Staging pending**; Key Flow 3 step 1. Canonical evidence: `docs/fairdrop-contracts.md` → Command and state table, **Cancel / STAGING**; `epics.md` → Story 1.6, Cancel while STAGING.

**Trigger:** Directory inspection, listener setup, QR generation, or another Stage step takes long enough that the user wants to stop it.

**Why it matters:** The backend explicitly supports user cancellation while STAGING, but the only specified Cancel control exists in Staged and Transferring. A user cannot invoke the canonical recovery path with pointer, keyboard, switch, or voice while the pending card is displayed.

**Concrete fix:** Add a semantic **Cancel preparation** action to Stage Pending. On activation, disable repeat activation, expose “Canceling preparation…” via the polite status channel, call `CancelTransfer`, and remain pending until the Stage promise returns `cancelled` and unwind is complete. Because pre-ack cancellation emits no lifecycle event, specify the local promise/command result as the authority for returning to Idle and guard obsolete Stage resolution exactly as Story 1.8 requires.

**Consequence if shipped:** A slow or stuck staging operation becomes an inaccessible wait with no operable escape except closing the app.

### A11Y-06 — Medium — Contract defect: “coalesced” progress has no assistive cadence or significance rule

**Location:** `EXPERIENCE.md` → Interaction Primitives, “Progress updates are polite and coalesced”; Component Patterns rows **Progress Meter**, **Transfer Metrics**, and **Status Announcer**. Canonical source: Architecture `AD-7` and contracts Event ordering, which cap backend snapshots at 4 Hz.

**Trigger:** A transfer produces snapshots at the allowed four per second and each accepted update changes meter values, byte text, throughput, or live-region text.

**Why it matters:** Four hertz is a rendering/event ceiling, not a screen-reader announcement cadence. If the live string contains changing bytes and speed, assistive speech may restart continuously and terminal speech may be delayed.

**Concrete fix:** Separate visual refresh from assistive announcements. Require progressbar values to update silently as snapshots arrive; publish a polite textual update no more often than every five seconds **and** only after meaningful change (for known totals, at least 10 percentage points; for unknown totals, a human-scale byte threshold). Always announce start once and terminal outcome once; terminal/error cancels queued progress speech. Do not put volatile throughput in the live-region string.

**Consequence if shipped:** Progress speech can monopolize the screen reader and obscure Cancel, error, or completion feedback.

### A11Y-07 — Medium — Contract defect: cancel-pending and the completion race have no user-facing announcement contract

**Location:** `EXPERIENCE.md` → Component Patterns row **Cancel Action**; State Patterns row **Staged or Transferring/cancel pending**; Key Flow 3; Backend-Authoritative Lifecycle and valid event grammars.

**Trigger:** Cancel is activated, teardown takes perceptible time, or natural completion linearizes before Cancel.

**Why it matters:** The control becomes unavailable, but no label/state/status change is required. In the race, the user may hear Done after asking to cancel without any explanation; when cancellation wins, reset can arrive without a distinct cancellation result.

**Concrete fix:** While pending, retain focus and change the button's accessible and visible label to “Canceling…” (or add adjacent `role="status"` text), set `aria-disabled="true"` only if activation is suppressed while focus remains, and keep the current metrics readable. Define race copy: if complete/error wins, announce only that authoritative outcome; if reset wins without terminal, announce “Transfer canceled. Ready for another file or folder.” Do not announce cancellation as Error.

**Consequence if shipped:** Keyboard and screen-reader users cannot tell whether their command was accepted and may interpret a race-winning completion as UI failure.

### A11Y-08 — Medium — Contract defect: the responsive claim stops short of the WCAG reflow and text-spacing floor

**Location:** `DESIGN.md` → Typography and Layout & Spacing; `EXPERIENCE.md` → Accessibility Floor and Responsive & Platform rows **640–759px** and **640×480 minimum**.

**Trigger:** 200% text resizing, an effective 320 CSS-pixel content width, enlarged default fonts, or WCAG text-spacing overrides are used in the 640×480 native window.

**Why it matters:** The spine promises reachable actions at 200% and 640×480 but does not require all information/functionality to reflow at 320 CSS pixels, nor does it cover 1.4.12 text-spacing overrides. “Stable dimensions” on Stage Pending and two-line clamping can clip copy. The fixed 20px gutters and a scan-ready QR are feasible, but not yet proven as a complete no-two-dimensional-scroll contract.

**Concrete fix:** Add an explicit reflow contract at 320 CSS px: one-dimensional vertical scrolling only, no loss/overlap/clipping, QR may retain a fixed square but must not force page-level horizontal scroll, every control remains reachable, and URLs/names may wrap or truncate without pushing Copy/Cancel offscreen. Require survival of 200% text size and WCAG text-spacing overrides; fixed-height content containers must grow. Preserve the existing 640×480 native minimum as a separate platform constraint.

**Consequence if shipped:** Content can satisfy the named window breakpoint while still failing WCAG reflow or clipping enlarged/respaced text.

### A11Y-09 — Medium — Contract defect: exact palette instructions conflict with forced-colors behavior

**Location:** `DESIGN.md` → Colors, “Implement the exact light/dark hex values”; Components; `EXPERIENCE.md` → Accessibility Floor, “Forced-colors/high-contrast mode preserves native text, borders, and focus indicators.”

**Trigger:** Windows High Contrast/`forced-colors: active` overrides authored colors, or the implementation tries to preserve exact Terracotta colors with `forced-color-adjust: none`.

**Why it matters:** The desired outcome is present, but precedence is not. An implementer can plausibly obey the exact-hex rule and disable system adjustment broadly, defeating high contrast. Outcome rules, the progress state, and the drop target also need non-color distinctions after palette substitution.

**Concrete fix:** State that forced colors explicitly supersede Terracotta Linen. Use system colors for text, surfaces, borders, focus, controls, and status rules; retain text/icons/patterns for state. Permit `forced-color-adjust: none` only on the production QR bitmap and its white quiet-zone substrate, after testing that the code remains scannable. Require a visible system-color boundary for DropZone and a pattern/length-plus-text distinction for progress.

**Consequence if shipped:** High-contrast users may receive low-contrast authored colors or lose essential state and control boundaries.

### A11Y-10 — Medium — Contract defect: reduced motion does not cover the indeterminate progress treatment

**Location:** `EXPERIENCE.md` → Interaction Primitives reduced-motion bullet; State Patterns row **Transferring/directory or unknown total**; Accessibility Floor final bullet. `DESIGN.md` → Components row **Progress Meter**.

**Trigger:** A directory/unknown transfer uses an animated stripe, sweep, shimmer, or translation while `prefers-reduced-motion: reduce` is active.

**Why it matters:** The contract removes spatial lifecycle transitions but is silent about the only likely continuous animation. Stopping the animation without a text equivalent could also remove the perceived “in progress” state.

**Concrete fix:** Require the unknown-total meter to remain understandable without animation: a static non-directional pattern plus persistent “Sending — total size unknown” text and live wire bytes. Under reduced motion, remove every repeating transform/sweep; opacity changes should be immediate or near-immediate and must not blink.

**Consequence if shipped:** Motion-sensitive users may face continuous movement, or reduced-motion mode may erase the transfer-state cue.

### A11Y-11 — Medium — Contract defect: long Unicode handling is not robust enough for visual or spoken names

**Location:** `DESIGN.md` → Typography, “Long names wrap to two lines, then truncate with an accessible full-name label”; `EXPERIENCE.md` → Component Patterns row **Item Summary** and Accessibility Floor long-Unicode bullet. Canonical source: contracts Source mutation and link policy / NFR11.

**Trigger:** A filename contains an unbroken long segment, combining characters, emoji sequences, right-to-left text, bidi controls, or a name whose meaningful suffix is outside the first two visual lines.

**Why it matters:** “Accessible full-name label” is underspecified and may replace rather than describe visible text, be applied to a noninteractive heading inconsistently, or split grapheme clusters. Unisolated bidi content can reorder adjacent size/ZIP/control text.

**Concrete fix:** Render the sanitized full name as text in a bidi-isolated container (`<bdi dir="auto">` or equivalent), use grapheme-safe CSS clamping with `overflow-wrap:anywhere` and `min-inline-size:0`, and expose the full value through persistent screen-reader text or `aria-describedby` rather than a tooltip-only title. Keep the visible substring in the accessible name (SC 2.5.3), and test LTR/RTL mixed names, emoji/combining marks, and extension preservation.

**Consequence if shipped:** Names can overlap controls, become misleadingly reordered, or be announced differently from what is visibly selected.

### A11Y-12 — Medium — Contract defect: firewall guidance is required but has no accessible timing or focus contract

**Location:** `EXPERIENCE.md` → Information Architecture row **OS firewall prompt**; Trusted-LAN & Privacy final bullet; State Patterns rows **Firewall/allowed** and **Firewall/denied**; Key Flow 4; `.working/finalize-gaps.md` item 2. Canonical source: `epics.md` Story 3.3 native smoke matrix.

**Trigger:** The first listener starts and Windows/macOS presents an OS-owned inbound-network prompt, possibly taking focus away from FairDrop; the user denies it or returns to the app.

**Why it matters:** Release guidance alone is not available to a screen-reader user in the live task. If explanation appears after the OS prompt, it cannot prepare the decision. Denial recovery and focus restoration are not specified.

**Concrete fix:** Put concise, always-available preflight guidance in Idle near the selection actions (no persisted “seen” flag is needed): “Your first transfer may ask to allow FairDrop on this local network.” Before staging starts, it is readable in document order. After the OS prompt closes, restore focus to a logical in-app target: the Stage Pending/next state heading when allowed, or the inline `network_unavailable` recovery message when denied. Do not attempt to restyle or duplicate the OS dialog; verify its native accessible name/buttons on both platforms.

**Consequence if shipped:** Users can deny an unexplained system prompt, lose focus on return, and have no keyboard/screen-reader recovery path.

### A11Y-13 — Medium — Contract defect: stable public-error codes do not yet guarantee understandable, actionable error text

**Location:** `EXPERIENCE.md` → Voice and Tone, Error Panel, State Patterns error rows, and Accessibility Floor alert rule; `.working/finalize-gaps.md` item 3. Canonical source: `docs/fairdrop-contracts.md` stable error code table and `PublicErrorOf` rule.

**Trigger:** Any command or terminal failure is presented using a safe but as-yet-unapproved `PublicError.message`.

**Why it matters:** Safety from path/token disclosure is necessary but not sufficient. An alert such as “Transfer failed” may identify no cause category or recovery. Errors must be understandable without relying on color/icon, and validation errors should identify what input is accepted.

**Concrete fix:** Approve a code-to-copy table that preserves safe disclosure while giving a literal next step where one exists. Each row should define visible heading, body, announcement string, whether it receives focus or uses alert/status, and recovery action. At minimum distinguish invalid selection, unsupported path, source changed, network unavailable, setup failure, and transfer failure. `cancelled` remains non-error feedback; `beacon_warning` explicitly preserves QR/direct URL use.

**Consequence if shipped:** Screen-reader users may receive an urgent alert that is safe but too generic to understand or recover from.

### A11Y-14 — Medium — Implementation verification need: the web contract must be proven through the native accessibility bridges

**Location:** `EXPERIENCE.md` → Accessibility Floor, Interaction Primitives, native dialog states, and second-instance restoration; `epics.md` Story 1.9 final acceptance criterion and Story 3.3 native smoke matrix.

**Gap:** React/jsdom and static ARIA inspection cannot prove actual Wails WebView2/WebKit accessibility behavior, programmatic focus, live-region interruption, OS-dialog focus return, second-instance restoration, or listener cleanup.

**Concrete verification:** Add a recorded native matrix:

| Platform | Assistive setup | Required scenarios |
|---|---|---|
| Windows amd64 | Current supported Windows + WebView2; keyboard; Narrator and NVDA | Drop fallback via both browse buttons; dialog cancel focus return; invalid drop; Stage/Start/Cancel/Done/Error/reset speech; 4 Hz visual updates without 4 Hz speech; firewall allow/deny return; forced colors; 200% text; second-instance restore without duplicate announcement |
| macOS | Supported macOS + Wails WebKit; keyboard Full Keyboard Access; VoiceOver | Same lifecycle, native file/folder dialog behavior, firewall prompt/return, Reduce Motion, Increase Contrast, 200% text/reflow, second-instance focus preservation |

Record screen reader/browser/OS versions and exact utterance order. Include automated component tests for pre-mounted live regions, one update per event, no duplicate listeners after remount, focus destination existence before `.focus()`, and ignored stale/post-terminal events.

**Consequence if skipped:** A semantically plausible React implementation can still be silent, doubled, or focus-broken in the shipped desktop accessibility tree.

### A11Y-15 — Low — Implementation verification need: the QR alternative is conceptually sound but needs exact semantics

**Location:** `EXPERIENCE.md` → Component Patterns rows **QR Panel** and **Direct URL Row**; Accessibility Floor; Key Flows 1–2. `DESIGN.md` → Components row **QR Panel**.

**Gap:** The direct URL plus Copy action is a valid non-camera alternative, but implementation details can accidentally make the QR noisy or make the URL claim the transfer locally.

**Concrete verification:** Render the QR as a noninteractive image with concise alt text such as “Download QR code for [item name]”; never include or spell the capability token in its accessible name. Render the desktop URL as readonly text, not a local activation link that could claim the one-shot transfer; provide a semantic **Copy download link** button whose accessible name includes “download link.” Verify Copy retains focus and announces “Copied” once. Test QR scan success in light/dark/forced-colors at 640×480 and 200% without sacrificing the direct URL/Copy path.

**Consequence if skipped:** Screen-reader output may expose a long token or users may accidentally consume the one-shot URL on the sender.

## Confirmed strengths

- `EXPERIENCE.md` gives every drag action a semantic native-dialog alternative, satisfying the core keyboard/drag-equivalence need; it correctly bans a DOM file-drop substitute.
- The 44px target token exceeds WCAG 2.2 AA's 24px minimum target size, subject to native rendering verification.
- Focus colors pass 3:1 against canvas, surface, and elevated backgrounds in both modes (light minimum measured 4.99:1; dark minimum 6.33:1).
- Known positive versus unknown totals, wire bytes, finite percentages, and stale/post-terminal suppression align with the authoritative backend grammar.
- Nonfatal discovery warnings remain inline and preserve the direct transfer path; cancellation is correctly excluded from Error.
- The QR retains a fixed black-on-white production substrate, and the direct URL/Copy path provides a non-camera alternative.
- Reduced-motion intent, high-contrast intent, full-name access, quiet native-dialog cancel, and listener cleanup are all present even where findings above require sharper acceptance criteria.

## Acceptance gate after correction

Accessibility validation can pass when all contract defects above are resolved and native evidence proves:

1. No terminal information becomes unavailable after three seconds.
2. Every meaningful component/state boundary reaches unrounded 3:1 contrast; all normal text reaches 4.5:1.
3. Each transition produces one coherent focus/announcement sequence.
4. Progress is visually smooth but assistively quiet, with defined known-positive, unknown, and known-empty semantics.
5. Every state, including Stage Pending and cancel-pending, remains keyboard operable.
6. 320 CSS-pixel reflow, 200% text, text-spacing overrides, forced colors, and reduced motion preserve all information and actions.
7. NVDA/Narrator on Windows and VoiceOver on macOS complete file, folder, cancel, failure, and firewall paths without lost focus or duplicate speech.
