# Resolved decisions — FairDrop UX draft

All Reviewer Gate product, copy, presentation, and contract decisions are resolved in `DESIGN.md` and `EXPERIENCE.md`. The backend lifecycle and one-shot HTTP protocol remain unchanged.

## Decision record

1. **Terminal reset and review time:** `transfer-reset` still clears the backend session. The last Done/Error remains visible as a dismissible, non-session Idle status until the next Stage attempt or Dismiss. Reset does not force a second focus move.
2. **Known-empty file:** show **“Empty file — 0 bytes to transfer”** with no percentage-bearing progressbar, then wait for authoritative Done.
3. **Stage Pending cancellation:** expose **Cancel preparation**; local guarded Stage/Cancel command results control the pre-ack return to Idle.
4. **Announcement ownership:** every transition has one focus/live/alert owner. Progress speech is at most every five seconds and only after meaningful change; terminal events cancel queued speech.
5. **Cancel race:** Canceling… retains focus. Complete/Error wins with only its terminal outcome; reset-without-terminal wins with **“Transfer canceled. Ready for another file or folder.”**
6. **Firewall:** always-visible Idle preflight precedes first Stage; Windows Private-only and macOS incoming-connection guidance, focus return, denial recovery, and staged troubleshooting are fixed.
7. **Public errors:** all canonical codes now have exact safe `PublicError.message`, heading, announcement owner, and recovery. `cancelled` is non-error; `beacon_warning` is non-terminal.
8. **Trust handoff:** QR is primary. **Copy download link** is for direct receiver-browser opening. First opener claims the transfer; link previews may consume the accepted V1 link. Plain HTTP consequence, same-network/guest-isolation recovery, generic receiver-error recovery, no-extra-copy, and receiver-retains-download copy are exact.
9. **Completion boundary:** Done means sender-observed response-stream completion only; browser saving/opening is receiver-owned.
10. **Platforms and evidence:** roles are explicit; iPhone is receiver-only in V1. Windows sender → current iPhone Safari and Mac sender → current Windows Edge are required compatibility gates. Native Windows Narrator/NVDA and macOS VoiceOver evidence is also required.
11. **Accessibility visuals:** functional control-border tokens are separate from decorative borders; all listed load-bearing authored pairs exceed unrounded 3:1. Reflow at 320 CSS px, 200% text, WCAG text spacing, forced-colors precedence, static reduced-motion unknown progress, and Unicode/bidi/full-name rules are binding.
12. **Reference hygiene:** composite shadows are concrete CSS values. The HTML direction/theme boards are exploration-only; their sample copy and QR treatments are not approved and the spines win.
13. **Positioning:** AirDrop is an internal friction benchmark only. External copy names a Windows/Mac sender, one same-LAN browser receiver, and no account/receiver app. No symmetric native or universal-device claim is allowed.

## Remaining blockers

**No unresolved UX decision blocker remains.** Confirmed key-screen mocks and the primary-flow wireframe have been promoted while their `.working/` sources remain as an audit trail. The spines stay `status: draft` because final polish and the required implementation/native/browser evidence have not yet been completed. Evidence rows are explicit release acceptance obligations, not claims of verification.
