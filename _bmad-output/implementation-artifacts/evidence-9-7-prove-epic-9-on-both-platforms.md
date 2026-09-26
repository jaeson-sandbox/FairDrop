# Evidence: Story 9.7 — Prove Epic 9 on Both Platforms

## CI on the tip's own run

Read with `gh run view <id> --json headSha,conclusion,jobs`, never `gh run watch`.

| Run | headSha | Job | Conclusion |
|---|---|---|---|
| 36246204170 | `bb6cb70` (Stories 9.1–9.6 and all review follow-ups merged) | verify (windows-latest) | success |
| 36246204170 | `bb6cb70` | verify (macos-latest) | success |
| 36246204170 | `bb6cb70` | Linux adapter verification (not release proof) | success |

The defect fix found by the walkthrough below lands after `bb6cb70`, so the epic's final tip needs
its own green run before this story is `done`; that run is recorded in the section at the end.

## The built macOS binary, driven by the orchestrator (2026-09-26)

`wails build` of `bb6cb70` on macOS (arm64, Darwin 25.6), dark appearance, window at its default
size. Driven with real pointer clicks and real keystrokes through the computer-use tooling; the
receiver was `curl` on the same machine issuing a real HTTP GET against the capability URL.

| Step | Observed |
|---|---|
| Idle on launch | Drop zone with glyph, heading "Drop one file or folder", promise line and the centred "Choose File or Folder" pill packed together; grouped "Local network access" / "Troubleshooting" list below with chevrons at the right edge. |
| Browse menu, pointer | Opens below the pill with File/Folder glyphs and **nothing pre-highlighted**; hovering File gives the mocha highlight. |
| File chosen | Native Open panel; ⌘⇧G to a 2.4 MB sample; Staged appears. |
| Staged | Centred card: QR tile left, name + "File · 2.4 MB", Copy Link / Show Link on one row, the two caveat lines visible, "Trouble connecting?" and Cancel on one row below. The URL is not shown. |
| Show Link | The URL field slides open beneath the buttons; the button reads "Hide Link". The card grows and, being vertically centred, its top moves up ~27px — expected. |
| Copy Link | `pbpaste` returned the exact capability URL. **Defect:** the label did not change to "Copied" (see below). |
| Real receiver download | `curl` → HTTP 200, 2,400,000 bytes, SHA-256 identical to the source. Sending showed the ring at 38% with "917.5 KB of 2.4 MB / Sent" and "870.8 KB/s / Speed"; then Sent drew its check with the receipt "epic9-sample.bin · 2.4 MB", Send Another and Done. |
| Reset | After the backend's reset the Sent card stayed the same size in the same centred position — no jump (the Story 9.6 follow-up fix, confirmed on WebKit). |
| Send Another | Opens the File/Folder menu from the card; choosing the file again stages it with a fresh, hidden link. |
| Forced `source_changed` | Appended bytes to the staged file, then `curl` → HTTP 410. Card: "Item changed", receipt name, fixed message, **Try Again** + Dismiss. |
| Try Again | Re-staged **the same file with no chooser** and returned straight to Staged with a new link. **Cosmetic defect:** the refresh glyph touches the "T". |
| Forced `path_not_found` | Deleted the staged file, then `curl` → HTTP 410. Card: "Item not found", receipt name, **Choose Another** + Dismiss, and no Try Again — the owner's table. |
| Dismiss | Returns to Idle; a frame captured mid-entrance showed the staggered rise (pill and list still fading in after the heading). |
| Keyboard open | Tab reaches the pill (TabFocusesLinks, fact 1); Down arrow opens the menu with File focused and highlighted. |
| Click outside | Closes the menu. |

**Not a finding:** synthetic Escape did not close the keyboard-opened menu. AGENTS.md fact 5 records
that an Escape synthesised by this tooling does not reach web content while a hardware Escape does,
so a negative result from it says nothing about the product. Not recorded as a defect.

### Defects found, routed to a defect fix

1. **"Copied" never appears after a pointer click on macOS.** The copy succeeds and the announcer
   speaks, but `handleCopy` only sets the label while `focusedRef` is true, and `focusedRef` is set
   by `onFocus`, which WebKit never fires for a pointer click (AGENTS.md fact 3). jsdom and Chromium
   focus a clicked button, so every suite agreed with the bug. **Predates Epic 9** — the same code
   shipped in v1.2.1.
2. **Try Again's refresh glyph touches its label.** Cosmetic; the other button glyphs have a gap.

Both are fixed on `fix-copied-label-and-retry-glyph` (merged at `4892745`); see
`evidence-copied-label-and-retry-glyph-fix.md`.

**Re-driven on a fresh build of `4892745`:** a pointer click on Copy Link now shows "✓ Copied" in
the success tint with **no focus ring** (the fix author could not verify that on WebKit and flagged
it; it holds), and clicking elsewhere reverts the label to "Copy Link" (D-114). A forced
`source_changed` card shows Try Again with a clear gap between the refresh glyph and its label.

### What was not observed

- **Light appearance and Reduce Motion on the binary.** Both are macOS system settings, which the
  orchestrator does not change. Light mode was checked only in rendered Chromium previews of every
  view; Reduce Motion only through the suites' reduced-motion assertions. *Optional / unverified on
  WebKit* until someone switches those settings and looks.
- **A native drag-and-drop onto the drop zone or onto an outcome card.** Staging used the chooser
  throughout. The drop-on-card routing is covered by `App.test.tsx`, not by a real drop.
- **A phone scanning the QR code.** The receiver was `curl` on the same machine.

## Windows

**Not driven interactively.** No Windows host was available. The `verify (windows-latest)` job
proves the build and every suite on Windows; nobody clicked through the app there.
`docs/release-policy.md` permits the automated gate alone, and this says so rather than implying
otherwise.
