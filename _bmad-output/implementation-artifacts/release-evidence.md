# Release evidence

> **Owner policy change, 2026-09-11:** manual observations below are optional for
> personal development/releases, not blocking sign-off. Automated native gates
> remain mandatory. See `docs/release-policy.md`. Earlier gate wording below is
> historical and superseded; pending rows have not been converted into passes.

Human-observed results on real devices, one row per scenario. Story 3.9 owns the full template
and the rules for it; this file is started early so the first observation is not lost.

The rules that already apply: a row records platform and OS version, artifact, date, reviewer,
and pass/fail; an agent may create and maintain rows and record results a person supplies, and
never fills in a pass itself; a missing, ambiguous, stale, or failed row blocks a release rather
than being summarised as an unverified manual check. Sender-side facts below come from the
`fairdrop:` lifecycle log the app writes to stderr when launched from a shell; receiver-side facts
come only from the person holding the receiving device.

## Rows

| # | Scenario | Sender | Receiver | Artifact | Date | Reviewer | Result | Evidence |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | One valid folder ZIP download | Windows 11 Home 10.0.26200, `192.168.1.169` (Ethernet; selector verified to rank the private address above a VPN adapter) | Phone on the same Wi-Fi — **OS, browser, and whether the ZIP opened: to be confirmed by the reviewer** | `build/bin/fairdrop.exe` built 2026-09-08 19:28 from the tree committed as `f2ea414` | 2026-09-08 | Jaeson | **pass (sender-side verified; receiver-side pending confirmation)** | Sender log: `transfer-started seq=1` 20:46:51 → `transfer-complete seq=3 bytes=665` 20:46:51 → `transfer-reset seq=4` 20:46:54. Grammar matches the contract exactly; reset fired on the three-second lease. |
| 2 | Single-instance restoration: launch FairDrop, launch it again (double-click, "Open with", or a stale shortcut) while the first is still running | — | — | **pending** | **pending** | **pending** | **pending** | Story 3.1 implements and unit-tests `SingleInstanceLock`/`restoreWindow` against fake runtime seams; this row is the native-OS smoke check the fakes cannot stand in for. Run it with a transfer already in progress (the matrix's "second launch mid-transfer" row), not from Idle, since no automated test puts a real session in flight when the second launch arrives. Confirm on both Windows and macOS. Pass requires all of: (a) the second process exits without ever opening a window, and starts no second coordinator, listener, or beacon; (b) the first window unminimises and comes to the front with its in-progress transfer, or retained Done/Error outcome, unchanged; (c) focus, per EXPERIENCE.md's "logical focus" and "does not reset or duplicate speech" (lines 193, 266) -- not merely "some control has focus", but the specific control that held keyboard focus before the second launch still holds it afterward, and a screen reader does not re-announce anything; (d) afterward, closing the one remaining app still performs coordinator shutdown exactly once, per the epic's acceptance criterion. Windows foreground-lock rules may make (b) present as the taskbar button flashing rather than the window jumping to the front, because the second process has already exited by the time the first process's window would ask for focus, and a background process generally cannot steal it from a same-process focus change; record what was actually observed rather than assuming a flash is a failure -- whether a flash-only restoration counts as a pass is Story 3.9's decision, not this row's. Owned by Story 3.9. |

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
