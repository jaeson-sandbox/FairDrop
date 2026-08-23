# Validation Report — FairDrop

- **DESIGN.md:** `_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-2026-08-23/DESIGN.md`
- **EXPERIENCE.md:** `_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-2026-08-23/EXPERIENCE.md`
- **Run at:** 2026-08-23T14:03:22-04:00

## Resolution update — 2026-08-23

All 33 raw reviewer findings were resolved before finalization: 31 became binding contract or document corrections, and the two findings that require real native/browser testing became explicit release evidence gates in `EXPERIENCE.md`. The final spine pair passes source, section-order, token-reference, component-parity, copy-registry, flow-schema, promoted-link, and whitespace checks. The findings below remain as the pre-correction review record; they are not open UX decisions.

## Overall verdict

The rubric review finds the pair mechanically coherent and adequate as a draft: flow, component, reference, and document-shape coverage are strong. It is not final-ready because state decisions remain open, several assumptions are presented as approved requirements, and one component token embeds unresolved references inside a CSS string.

The accessibility and trust lenses materially strengthen that warning. The current three-second terminal presentation is an inaccessible information timeout, several light-mode non-text combinations fail the stated 3:1 floor, and the copy does not yet explain bearer-link ownership, plain-HTTP consequences, firewall recovery, or the boundary of sender-side completion. Revise the contracts before claiming WCAG 2.2 AA or changing their status to `final`.

Raw reviewer findings: **1 critical, 13 high, 17 medium, and 2 low**. Repeated findings are retained because each lens establishes a different downstream consequence.

## Category verdicts

- Flow coverage — **strong**
- Token completeness — **adequate**
- Component coverage — **strong**
- State coverage — **thin**
- Visual reference coverage — **strong**
- Bloat & overspecification — **strong**
- Inheritance discipline — **thin**
- Shape fit — **strong**

## Findings by severity

### Critical (1)

**[Accessibility · A11Y-01] — Three-second reset is an inaccessible information timeout** (`EXPERIENCE.md`: Done/Error, Reset, focus, lifecycle, flows)

The backend reset removes visible success/error information and can force another focus move before a slow reader or assistive-technology user can review it.  
**Fix:** Preserve the last terminal outcome visibly in Idle until the next Stage attempt or explicit dismissal, while still letting `transfer-reset` clear the session. Do not force a second focus move while the retained outcome owns focus.

### High (13)

**[Rubric · State coverage] — Known-empty file progress is undefined** (`EXPERIENCE.md`: Progress Meter; known-empty state)  
**Fix:** Define the visible state, text, metrics, and accessibility semantics for `totalKnown=true`, `totalBytes=0`, `percent=0`.

**[Rubric · State coverage] — Firewall guidance has no committed surface or wording** (`EXPERIENCE.md`: firewall IA/state/flow)  
**Fix:** Commit pre-prompt guidance, post-denial recovery, platform-specific meaning, and exact neutral copy.

**[Rubric · State coverage] — Stable `PublicError` copy is missing** (`EXPERIENCE.md`: Voice, Error Panel, error states)  
**Fix:** Add a safe code-to-copy table with heading, body, announcement owner, and recovery action.

**[Rubric · Inheritance] — Assumptions appear as approved requirements** (`.memlog.md`; both spines)  
**Fix:** Confirm automatic theme, immediate staging, copy feedback, voice, focus/live behavior, and no-tray/no-notification posture as decisions or visibly tag them as assumptions.

**[Accessibility · A11Y-02] — Light-mode non-text contrast fails** (`DESIGN.md`: border, drop target, controls, progress)  
Measured borders are 1.62–1.76:1 and progress fill/track is 2.933531:1.  
**Fix:** Add a ≥3:1 functional boundary token and revise the light progress pair with unrounded automated contrast checks.

**[Accessibility · A11Y-03] — Focus, status, and alert duplicate announcements** (`EXPERIENCE.md`: state focus and Status Announcer)  
**Fix:** Define one announcement owner per transition; focused content is not repeated in live/alert regions.

**[Accessibility · A11Y-04] — Empty-file semantics are inaccessible** (`EXPERIENCE.md`: known-empty progress)  
**Fix:** Use a third presentation: “Empty file — 0 bytes to transfer,” no percentage-bearing progressbar, then authoritative Done.

**[Accessibility · A11Y-05] — Stage Pending has no operable Cancel** (`EXPERIENCE.md`: Stage Pending Card; Cancel Action)  
**Fix:** Add semantic Cancel preparation, pending feedback, duplicate suppression, and local command-result authority for pre-ack cancellation.

**[Trust · TCH-01] — Plain HTTP is named without explaining the consequence** (`EXPERIENCE.md`: Voice; Trusted-LAN & Privacy)  
**Fix:** Say that transfer traffic is not encrypted and may be observable by someone monitoring the network; retain neutral styling.

**[Trust · TCH-02] — “Single-use” omits first-requester ownership and preview risk** (`EXPERIENCE.md`: Direct URL and receiver claim states)  
**Fix:** Explain that the first device/software to open the bearer link starts the download; instruct direct browser opening and decide whether preview claims are an accepted V1 limitation.

**[Trust · TCH-03] — Firewall denial lacks trustworthy recovery** (`EXPERIENCE.md`: firewall surface and states)  
**Fix:** Define Windows private/public-network advice, macOS incoming-connection meaning, focus return, denial copy, and recovery help.

**[Trust · TCH-04] — Done overclaims receiver storage** (`EXPERIENCE.md`: Done and Flow 1)  
**Fix:** Limit Done to sender-observable transport completion. Treat saving/opening in iOS Files as a browser-owned expected next action.

**[Trust · TCH-05] — Receiver support promise has no validation matrix** (`EXPERIENCE.md`: Foundation; platform; Flow 1)  
**Fix:** Define supported receiver combinations and verify at least Windows sender → current iPhone Safari and Mac sender → current Windows browser before saying more than “supported modern browser.”

### Medium (17)

**[Rubric · Token completeness] — Composite shadow strings contain embedded token references** (`DESIGN.md`: `staged-view.shadow`)  
**Fix:** Use concrete shadow values or split geometry/color into standalone tokens.

**[Rubric · Inheritance] — Source precedence is implicit** (both spine frontmatters)  
**Fix:** State that canonical SPEC and binding companions win; legacy/phase documents are traceability-only where non-conflicting.

**[Accessibility · A11Y-06] — Assistive progress cadence is undefined** (`EXPERIENCE.md`: progress announcements)  
**Fix:** Separate visual updates from speech; announce at most every five seconds and only on meaningful change, with terminal events canceling queued speech.

**[Accessibility · A11Y-07] — Cancel-pending and race outcomes lack feedback** (`EXPERIENCE.md`: Cancel; recovery flow)  
**Fix:** Retain focus, show “Canceling…,” and define authoritative completion-versus-reset announcement rules.

**[Accessibility · A11Y-08] — Reflow and text-spacing floor is incomplete** (`DESIGN.md`: layout; `EXPERIENCE.md`: accessibility/platform)  
**Fix:** Require one-dimensional reflow at 320 CSS px, 200% text, and WCAG text-spacing overrides with no clipped actions/content.

**[Accessibility · A11Y-09] — Exact palette conflicts with forced colors** (both spines)  
**Fix:** State that forced colors supersede Terracotta Linen; reserve `forced-color-adjust:none` only for tested QR rendering.

**[Accessibility · A11Y-10] — Reduced motion omits continuous indeterminate progress** (`EXPERIENCE.md`: progress/reduced motion)  
**Fix:** Use a static pattern plus text under reduced motion; remove repeating transforms and sweeps.

**[Accessibility · A11Y-11] — Long Unicode names lack robust bidi/grapheme rules** (both spines: item name)  
**Fix:** Use bidi isolation, grapheme-safe wrapping/clamping, persistent full-name access, and mixed-direction test cases.

**[Accessibility · A11Y-12] — Firewall prompt lacks accessible timing/focus contract** (`EXPERIENCE.md`: firewall)  
**Fix:** Put readable guidance before first Stage and restore focus to the next state or denial recovery message after the OS prompt.

**[Accessibility · A11Y-13] — Safe errors may still be unusably generic** (`EXPERIENCE.md`: PublicError handling)  
**Fix:** Make each safe error literal and actionable without relying on color/icon; keep cancellation and warnings outside Error.

**[Accessibility · A11Y-14] — Native accessibility verification matrix is missing** (Stories 1.9 and 3.3)  
**Fix:** Record Windows Narrator/NVDA and macOS VoiceOver evidence for dialog focus, lifecycle speech, forced colors, zoom/reflow, firewall, cancellation, and listener cleanup.

**[Trust · TCH-06] — Copy URL lacks cross-device handoff instruction** (`EXPERIENCE.md`: Direct URL Row)  
**Fix:** Label it “Copy download link,” explain that it must be opened in the receiving browser, and keep QR primary.

**[Trust · TCH-07] — Different-network failure lacks recovery guidance** (`EXPERIENCE.md`: offline/different LAN)  
**Fix:** Tell users to verify the same local Wi-Fi, note guest isolation, then cancel and create a fresh link.

**[Trust · TCH-08] — No-cloud/no-extra-copy benefit is not safely visible** (`EXPERIENCE.md`: trust copy)  
**Fix:** Say FairDrop sends locally and does not upload/store an extra copy while making clear the receiver keeps the download.

**[Trust · TCH-09] — AirDrop analogy can overpromise native parity** (`EXPERIENCE.md`: Inspiration and climax copy)  
**Fix:** Keep AirDrop as an internal friction benchmark; use an external promise naming desktop sender, one browser receiver, same LAN, no account/app.

**[Trust · TCH-10] — Generic receiver HTTP errors are not a guided recovery experience** (`EXPERIENCE.md`: receiver 404/423/410)  
**Fix:** Keep canonical generic errors in V1 and add sender-side troubleshooting/new-link guidance unless architecture explicitly authorizes safe receiver pages.

**[Trust · TCH-11] — Platform arrows imply symmetric native support** (`EXPERIENCE.md`: platform roadmap)  
**Fix:** Label roles explicitly: “Mac FairDrop sender → Windows browser receiver”; iPhone is receiver-only in V1.

### Low (2)

**[Accessibility · A11Y-15] — QR alternative needs exact semantics** (`DESIGN.md`/`EXPERIENCE.md`: QR and URL)  
**Fix:** Use concise non-token alt text, readonly URL text, semantic Copy download link, one announcement, and native scan verification.

**[Trust · TCH-12] — Working boards contain unsafe exploratory copy/QR visuals** (`.working/*.html`)  
**Fix:** Add prominent exploration-only warnings or annotations; do not promote unsafe board content as production reference.

## Reviewer files

- `review-rubric.md`
- `review-accessibility.md`
- `review-trust-cross-device.md`

## Mechanical notes

- The four named flows and all 18 paired component contracts are structurally covered.
- All standalone token references resolve; the composite shadow string is the exception requiring normalization.
- All current `.working/` artifacts resolve and no promoted mockups, wireframes, or imports are orphaned.
- Both spines remain `status: draft`; no validation finding has yet been rolled into them.
