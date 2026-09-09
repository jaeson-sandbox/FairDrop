---
title: 'Story 3.1: Enforce One Running FairDrop Instance'
type: 'feature'
created: '2026-09-08'
status: 'in-review'
review_loop_iteration: 0
baseline_commit: '31205904871e7430d6572e7d6b42f26ee6698805'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-3-context.md'
  - '{project-root}/docs/fairdrop-contracts.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Two FairDrop processes can run at once, each with its own coordinator, listener and beacon, so a second launch — a double-click, "Open with", a stale shortcut — competes with a live transfer instead of returning the user to it. AD-3 requires single-instance locking, pinned in `main_test.go`.

**Approach:** Configure Wails' `SingleInstanceLock` with one fixed UUID and an `App` method that restores the existing window through runtime seams — unminimise, then show — guarded for the pre-startup case and touching no transfer state.

## Boundaries & Constraints

**Always:** One literal UUID constant in `main.go`, pinned by a literal written out in `main_test.go`. The callback reaches the window only through `App` seams defaulting to `wailsruntime.WindowUnminimise` and `wailsruntime.WindowShow`, pinned beside `emit` and the dialogs. With no runtime context yet it calls nothing, counts `undelivered`, and logs one `fairdrop:` line. It never touches coordinator state, emits no event, and preserves the window's session, retained outcome and focus. `Args` and `WorkingDirectory` from the second launch are ignored. Every existing option pin stays green.

**Ask First:** Any change to the existing options contract — drop, frame, start state, dimensions, hooks, formatter, bindings. Staging a path carried by the second launch's arguments.

**Never:** Build a second coordinator, listener or beacon. Call a runtime function with a nil context — the real one answers `log.Fatalf`. Claim the native second-launch smoke check; that is human evidence in `release-evidence.md`, owned by Story 3.9.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| First launch | No lock held | Window opens; lock held for the process lifetime | N/A |
| Second launch, first started | Lock held; runtime context present | Second process hands off and exits `0`; first unminimises then shows; session, status and focus untouched; no event | N/A |
| Second launch before first finished starting | Nil runtime context | No runtime call; `undelivered` counted; one log line | N/A |
| Second launch with arguments | `Args` non-empty | Ignored; window restored only | N/A |
| Second launch mid-transfer | Session `TRANSFERRING` | Window restored; transfer continues | N/A |
| UUID or callback edited | Options changed | Literal pin in `main_test.go` fails | Test |

</frozen-after-approval>

## Code Map

- `main.go:97` `appOptions` — add `SingleInstanceLock{UniqueId, OnSecondInstanceLaunch: app.restoreWindow}`; constant `d1766c78-45cf-4e6d-9f04-c3700ab32024` beside it.
- `main_test.go:20-112` option pins to extend; `app_test.go:244` `TestNewAppWiresTheRealWailsRuntime` compares seams by function pointer.
- `app.go:77-93` seams; `:130` `NewApp`; `:311` `publish` — nil-context refusal, `undelivered`, `logEvent`: the shape to mirror; `:383` `startup` stores `a.ctx`; `:423` `runtimeContext` is nil before it.
- `app_test.go:118-200` — harness seam recording for `emit`, dialogs, `logf`.
- Wails v2.15.0 — `options.go:190` `SingleInstanceLock`/`SecondInstanceData`; `windows/single_instance.go:37` named mutex, `WM_COPYDATA` JSON, second process `os.Exit(0)`; `windows/frontend.go:1046` and `darwin/frontend.go:157` invoke the callback from a Wails-owned goroutine that is not synchronised with `OnStartup`; `darwin/single_instance.go:25` lock file plus notification; `pkg/runtime/window.go:61,137` `WindowShow`/`WindowUnminimise(ctx)`.
- `docs/fairdrop-architecture.md:199` item 10; `EXPERIENCE.md:43,193,266` — restore, preserve session and logical focus, no duplicated speech.

## Tasks & Acceptance

**Execution:**
- [x] `app.go` — `unminimise`/`show` seams with real defaults in `NewApp`; `restoreWindow(options.SecondInstanceData)` with the nil-context guard, `undelivered` and a log line, else unminimise then show.
- [x] `main.go` — the UUID constant and `SingleInstanceLock` in `appOptions`.
- [x] `main_test.go` — pin the UUID literal and the callback's presence; existing pins untouched.
- [x] `app_test.go` — harness seams; pre-startup no-op-with-log, post-startup order and exactly-once with the application-lifetime context, coordinator untouched, arguments ignored; extend the seam-pin test; `TestRestoreWindowRacingStartupNeverCallsTheRuntimeWithANilContext` drives `startup` and `restoreWindow` concurrently, 200 rounds under `-race`, asserting only the zero-or-two invariant.
- [x] `release-evidence.md` — the pending second-instance smoke row, for a person to fill.
- [x] `docs/fairdrop-architecture.md:199` — item 10 recorded as delivered here.
- [x] `evidence-3-1-enforce-one-running-fairdrop-instance.md` — mutation table and gate transcript.

**Acceptance Criteria:**
- Given `appOptions`, when built, then `SingleInstanceLock` carries the literal UUID and a callback, and `main_test.go` fails if either is removed or the UUID changes.
- Given a started `App`, when the callback fires, then `WindowUnminimise` then `WindowShow` are each called exactly once with the application-lifetime context, and no coordinator call or lifecycle event results.
- Given an `App` before startup, when the callback fires, then no runtime function is called, `undelivered` increments, and one `fairdrop:` line is logged.
- Given each guard is deliberately broken, when its mutation runs, then a named behavioral test fails rather than compilation.

## Evidence

Mutation tables, gate transcripts and review triage live in
[evidence-3-1-enforce-one-running-fairdrop-instance.md](evidence-3-1-enforce-one-running-fairdrop-instance.md), created with the implementation.

## Spec Change Log

- 2026-09-08: Review found two verification gaps and one wording gap, all closed in this pass without touching the frozen Intent, Boundaries, or Matrix. (1) `main_test.go`'s `SingleInstanceLock` pin checked `OnSecondInstanceLaunch` only for non-nilness, which a no-op stub `func(options.SecondInstanceData) {}` wired into `appOptions` also satisfies; two new tests now drive the callback through the built `options.App` value itself, so only the real `restoreWindow` can pass. (2) The started path's silence was unpinned: nothing asserted that `undelivered` stays `0` and nothing is logged on a successful restoration, so lifting the drop-path's `undelivered.Add(1)`/log call above the nil-context guard passed every existing test. The relevant tests now assert silence directly, and a disclosure check confirms no log line ever carries the second launch's `Args` or `WorkingDirectory`. (3) Comments describing what a second launch does claimed it hands off "instead of starting a second coordinator, listener and beacon", which reads as though composition itself does not happen. It does: `main()` composes before `wails.Run` reaches the lock check inside `Frontend.Run`. That composition is provably inert (`NewCoordinator` only assigns fields; `network.NewManager`/`qr.New` only close over dependencies; `server.New` stores an uncalled `listen` closure) and is discarded by the second process's `os.Exit(0)` before anything could start, but it does happen. Comments in `app.go` and `main.go`, and the evidence file's Notes, now say this precisely rather than implying nothing is constructed.

## Design Notes

Seams for the reason `emit` has one: the real runtime functions `log.Fatalf` on a context that never came from a window, and the callback arrives on a goroutine Wails does not synchronise with `startup`. Unminimise precedes show because a minimised window that is only shown can stay minimised; the runtime marshals to the UI thread itself. The callback holds only `App`'s read lock and never waits on the coordinator, whose lease a live transfer may hold.

## Verification

**Commands:**
- `gofmt -l .`; `go vet ./...`; `go test -count=1 ./...`; `go test -count=1 -race ./...` — clean
- `cd frontend && npx vitest run` — 490 unchanged; then, alone, `wails build` — exit 0
- Mutations, recorded in the evidence file: remove the nil-context guard; swap the call order; call show twice; change the UUID; drop the callback; make the callback touch the coordinator; read `a.ctx` directly instead of through the read lock (fails only under `-race`); swap `app.restoreWindow` for a stub wired into `appOptions`; lift the `undelivered` increment and the log call above the nil-context guard; log the second launch's `Args` in the drop line — each fails a named test.

**Manual checks:** launching FairDrop twice on Windows and macOS is Story 3.9's evidence row, not this story's claim.
