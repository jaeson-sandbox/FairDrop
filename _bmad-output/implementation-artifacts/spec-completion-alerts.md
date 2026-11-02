---
title: 'Notice a completed transfer while working in another app'
type: 'feature'
created: '2026-11-01'
status: 'in-review'
baseline_commit: 'f90fb0cfacff51905c6067b7a348f9b3dffe7185'
review_loop_iteration: 0
context:
  - '{project-root}/AGENTS.md'
  - '{project-root}/_bmad-output/specs/spec-fairdrop/SPEC.md'
  - '{project-root}/docs/fairdrop-contracts.md'
---

<frozen-after-approval reason="owner chose a Dock bounce on macOS and taskbar flash on Windows, 2026-11-01">

## Intent

When someone starts a transfer and switches apps, give them a quiet way to notice it finished. A user can enable background completion alerts for this run of FairDrop. One successful background transfer can request one informational Dock bounce on macOS or a brief finite taskbar flash on Windows. The foreground app keeps focus.

## Boundaries & Constraints

**Always** start off on each launch. Offer a discoverable, keyboard-accessible opt-in and opt-out in the desktop UI. All sending and receiving modes qualify for successful completion. The outcome card remains authoritative. Desktop policy controls presentation.

**Ask first** before introducing persistent preferences, app registration, new dependencies, installers, or broader notifications.

**Never** use system banners, sounds, tray icons, window activation, repeated bouncing until acknowledged, filesystem writes or registry writes for this feature. Pass no text, file or folder name, path, device address, link, token or session identifier to the system cue. Preserve the cancellation message, accessibility announcements and transfer lifecycle.

## I/O & Edge-Case Matrix

| Given | When | Then |
| --- | --- | --- |
| Enabled and background | A live send or receive of files or text completes | Request one bounded informational cue for FairDrop's own window. |
| Foreground or disabled | Transfer completes | Ordinary outcome only. Later focus loss or opt-in cannot replay a completion. |
| Failed, incomplete, cancelled or still waiting | Progress, error, reset, refusal or recovery occurs | No cue. |
| Held result or rapid lifecycle events | React rerenders, a completion resets, or another transfer starts | A successful event is eligible once even if the background webview is suspended. A fresh successful session may cue independently. |
| Delayed native delivery | User disables alerts, returns to FairDrop or exits | Fence stale pending work and check foreground at delivery. No queued replay after re-enable. |
| OS unavailable or slow | A cue cannot be delivered | Transfers, cleanup and in-app feedback continue. Bound outstanding work and drop obsolete cues. |
| Relaunch | Alerts were enabled earlier | Control and backend agree on disabled, with no remembered user preference. |

</frozen-after-approval>

## Code Map

- `app.go` publishes coordinator events from `appObserver` under an operation lease. Observe authoritative `TransferComplete` and keep slow platform work outside the publisher. The coordinator accepts an outcome only once. Avoid a second session history or reliance on committed React renders for delivery.
- `main.go`, `single_instance_darwin_cgo.go` and its non-cgo fallback show platform composition. AppKit's informational request belongs on the main thread with an inactive-app check. Windows Wails v2.15 creates the `wailsWindow` class. Match the current process and its own top-level window and check foreground ownership. Use finite `FlashWindowEx` taskbar-only flags. Its return value describes prior caption activity.
- `frontend/src/App.tsx` owns UI composition and announcements. Keep the control compact and secondary. Existing `ui/Notification.tsx` is the cancellation message. Update Quartz EXPERIENCE copy table, DESIGN, `ui/copy.ts` and `ui/copy.test.ts` together.
- Wails notification initialization writes Windows registration and a temporary icon. Use no notification-center functionality. Reuse pinned toolchain and native primitives.

## Tasks & Acceptance

- [x] Add a run-local toggle and authoritative completion dispatch. Given overlapping changes or a missing adapter, when delivery stalls, then teardown and new transfers still proceed and stale work is discarded.
- [x] Add platform adapters with supported build fallbacks. Given a background FairDrop window, when a successful completion is eligible, then only its own application receives the requested finite cue. Given foreground or an unavailable window, then return quietly.
- [x] Wire the user control through regenerated Wails bindings. Given a fresh launch, when the user enables and disables alerts, then UI and backend agree. Expose command failure honestly and support ordinary keyboard interaction.
- [ ] Test the matrix, public wiring and native defaults. Derive mutation proof from a canonical script for success gating, opt-in, stale work and platform guards. Run applicable cases on native desktop CI, preserve unique complete failure logs and assert named kills.
- [x] Update current contract, README and roadmap and record evidence separately. No story key. Main handles review, complete ordered verification, commit, push and integration. The Sol implementation subagent performs no remote operations.

## Evidence

See [verification evidence](evidence-completion-alerts.md).

## Spec Change Log

## Verification

Build Wails before standalone frontend checks. Run the repository gate in order including cgo race, platform builds, frontend/browser and relevant existing canonical mutations. Compare native verification job conclusions to the branch commit before merge. Separate automated native mechanism proof from an actual observed Dock bounce or taskbar flash.

## Suggested Review Order

**Transfer completion**

- Observe a successful transfer at the coordinator boundary, including when the webview is suspended.
  [app.go:674](../../app.go#L674)

- Capture eligibility and replace pending work without delaying another transfer.
  [completion_alerts.go:76](../../completion_alerts.go#L76)


**Platform delivery**

- Check focus and the opt-in generation on the AppKit delivery path.
  [completion_alerts_darwin.go:69](../../completion_alerts_darwin.go#L69)

- Recheck window ownership and foreground state before a finite taskbar flash.
  [completion_alerts_windows.go:213](../../completion_alerts_windows.go#L213)


**Desktop control**

- Restore the current process preference when the webview mounts.
  [App.tsx:53](../../frontend/src/App.tsx#L53)


**Verification**

- Drive the real HTTP transfer and count requests at the cue boundary.
  [completion_alerts_test.go:379](../../completion_alerts_test.go#L379)

- Exercise shared and platform-specific regressions using named assertions.
  [verify-completion-alerts-mutations.py:13](../../scripts/verify-completion-alerts-mutations.py#L13)
