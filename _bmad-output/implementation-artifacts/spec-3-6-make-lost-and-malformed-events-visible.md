---
title: 'Story 3.6: Make Lost and Malformed Events Visible'
type: 'feature'
created: '2026-09-11'
status: 'done'
review_loop_iteration: 0
baseline_commit: '3d979dd97dc3d4575848271a2ad0b21299938074'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-3-context.md'
  - '{project-root}/AGENTS.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** FairDrop's honesty mechanism does not exist in a shipped build. `docs/fairdrop-contracts.md` leans on "recorded as a diagnostic" wherever a failure is swallowed, and every one of those records goes to a sink that only tests read — the coordinator writes to it, `app.go` never reads it, and the counter for undelivered events is incremented and never surfaced. Around that hole sit five more ways a signal disappears: an observer that panics wedges the coordinator because no lease release is deferred, one that blocks stalls every command, a lane closing while staged synthesises nothing, a repeated `Stop` discards the first call's diagnostic, and the server's error log discards genuine handler panics along with the request text it exists to silence.

**Approach:** Give every diagnostic a way out of the process — the same stderr lifecycle log `app.go` already writes — through an injected seam, so a test observes it and a user can be asked for it. Then close the five paths where a signal is lost before it ever reaches that seam.

## Boundaries & Constraints

**Always:** Every diagnostic the coordinator or server records reaches a surface outside the process in a composed binary, through a seam a test can observe. AD-9 holds without exception: no capability token, no selected name, no source path, no adapter text that could contain one. The operation lease is released on every path out of a lifecycle command, including a panicking observer. A lane that closes mid-session produces a terminal outcome from any state a session can be in, not only TRANSFERRING. A terminal outcome always carries a control, so a lost reset cannot strand the window.

**Ask First:** Any new public error code or user-visible string — Story 3.5 established the four-file procedure and Story 3.11 owns the remaining copy. Changing which surface owns an announcement. Making the diagnostic log anything a user could mistake for a supported interface.

**Never:** Let a diagnostic carry a path, a token, or a selected name. Recover from a panic in a way that hides it: the point is that it becomes visible, not that it becomes silent. Decide D-035, D-039, D-048, D-097, D-103, D-104 or D-106 — those are Story 3.11's.

**Deferral rule for this story:** a review finding is fixed here unless it needs a human decision, lives in a package this Code Map does not name, or is a genuinely different goal. Each deferred entry states which.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Any recorded diagnostic | cleanup error, bound timeout, refused event | One line on the app's stderr log, code and message only | Observable (D-098) |
| Transfer failure | adapter error rewritten to public copy | The original cause is recorded before the rewrite | Observable (D-092) |
| Two resources unaccounted | teardown hits two bounds | Both reported, not only the first | Coded (D-100) |
| Observer panics | `Publish` panics mid-command | Lease released, panic surfaced as a diagnostic, coordinator still usable | Recovered and reported (D-043) |
| Observer blocks | `Publish` never returns | The command does not stall behind it | Bounded (D-034) |
| Lane closes while STAGED or CLAIMING | server dies before a claim | A terminal outcome is synthesised; the UI stops waiting | Synthesised (D-042, D-091) |
| Repeated `Stop` | second call after a diagnostic | The first call's diagnostic survives | Observable (D-020) |
| Handler panics | net/http recovers it | The panic is reported without the request text | Observable (D-021) |
| Sink full | more than `maxDiagnostics` | An overflow marker, not silence | Observable (D-031) |
| Undelivered event | emit fails or precedes startup | Counted *and* logged | Observable (D-049) |
| Terminal outcome | done or error on screen | A control is always present | UI (D-059) |

</frozen-after-approval>

## Code Map

- `internal/transfer/coordinator.go:100` `diagnosticSink`, `:105` `record` (silently returns once `maxDiagnostics` is reached), `:114` `snapshot` — the only reader is tests. `:918` `recordDiagnostic` is the single write path, which is what makes one seam sufficient.
- `internal/transfer/coordinator.go:908` `publish` — panics if the lease is not held, returns early on a nil observer, then calls `c.observer.Publish(event)` with nothing deferred. A panic there unwinds past every `releaseLease` (D-043); a block there holds the lease forever (D-034). `Dependencies` at `:241` is where a diagnostics seam joins `BoundTimer` and `AfterFunc`.
- `internal/transfer/outcomes.go` `drain`'s post-loop synthesis and `drainerMayActLocked` — the guard requires `stateTransferring`, which is why a lane closing at STAGED or CLAIMING synthesises nothing (D-042, D-091). `TestStagedServerEventsAreDrainedWhileStaged` in `coordinator_stage_test.go` closes the lane at STAGED today and asserts only that the drainer exits.
- `internal/transfer/lifecycle.go` `unwind` — returns the first bound failure only (D-100).
- `internal/server/lifecycle.go:280` `ErrorLog: log.New(io.Discard, "", 0)` — the comment is right that the request text is unsafe, and wrong that discarding everything is the only option: net/http writes recovered handler panics to the same logger (D-021). A filtering writer that forwards the panic and drops the rest is the shape.
- `internal/server/lifecycle.go` `Stop`/`teardown` — `teardownOnce` means the second call returns the stored error, and the first call's cleanup diagnostic is not kept beside it (D-020).
- `app.go:387` `logEvent` and `:398` `a.logf` — the existing surface and its AD-9 discipline: kind, seq, session, bytes, code, never progress or a path. `:481` shows the `undelivered` log line that exists for the second-instance case; the lifecycle `undelivered` counter has no equivalent (D-049). `app_test.go`'s harness captures `logf` into `h.logged()`.
- `frontend/src/ui/OutcomePanel.tsx` and `App.tsx:162` — the terminal panel; `onDismiss` is passed only when `outcome.retained`, so a non-retained terminal outcome has no control at all (D-059).
- Epic 1 retrospective items 2, 3, 4 and 7 are in `epic-1-retro-2026-09-01.md`: a refused terminal event rendering as the cancel-won summary, `Warning.Code` unconstrained at the boundary, the discovery warning unreachable by a screen reader, and progress validated by two strategies with Stage metadata parsed twice.

## Tasks & Acceptance

**Execution:**
- [x] `internal/transfer/coordinator.go` — a diagnostics seam on `Dependencies`, called by `recordDiagnostic` beside the sink; overflow marked rather than silent; the original failure cause recorded before `terminalPublicError` rewrites it.
- [x] `internal/transfer/coordinator.go` — `publish` recovers a panicking observer, reports it, and releases the lease on every path; a blocking observer cannot hold a command.
- [x] `internal/transfer/outcomes.go` — synthesise a terminal outcome from STAGED and CLAIMING, not only TRANSFERRING.
- [x] `internal/transfer/lifecycle.go` — `unwind` reports every unaccounted resource.
- [x] `internal/server/lifecycle.go` — a filtering `ErrorLog` that forwards panics and drops request text; the first `Stop`'s diagnostic survives a second call.
- [x] `app.go` — wire the seam to `logf`; log the lifecycle `undelivered` drop.
- [x] `frontend/src/ui/OutcomePanel.tsx` — a control on every terminal outcome.
- [x] Epic 1 retrospective items 2, 3, 4 and 7.
- [x] Tests for each, plus a disclosure test proving no diagnostic line can carry a path or token.
- [x] `evidence-3-6-make-lost-and-malformed-events-visible.md`, and the twelve ids closed with `epics.md` kept in step.

**Acceptance Criteria:**
- Given any diagnostic recorded in a composed binary, when it is written, then a test observes one log line carrying its code and message and nothing else.
- Given an observer that panics and one that blocks, when a lifecycle command runs, then the coordinator is still usable afterwards and the lease is free.
- Given a lane that closes in each of STAGED, CLAIMING and TRANSFERRING, when the drainer sees it, then a terminal outcome reaches the UI in all three.
- Given every diagnostic path, when a source path, token or selected name is passed through it, then a disclosure test fails.

## Evidence

Audit, mutation tables and gate transcripts live in
[evidence-3-6-make-lost-and-malformed-events-visible.md](evidence-3-6-make-lost-and-malformed-events-visible.md), created with the implementation.

## Spec Change Log

**2026-09-11 (implementation).** Three notes, none of which change the frozen Intent.

Item 3 of the Epic 1 retrospective was closed before the rest, by `transfer.WarningCode` and
`TestEveryWarningCodeIsAcceptedByTheFrontendParser`; the Tasks list names all four items together
and this records that they did not land together.

Item 7's "one strategy" was settled as **reject at the boundary, derive for display**: the validator
keeps the refusal (it is a security boundary -- Wails notifies same-window listeners before Go ever
sees an event), the selector derives the displayed percentage from the two authoritative integers,
and the selector's repair layer is deleted. The alternative -- keep trusting the wire `percent` and
delete only the repairs -- was rejected because it leaves the exact-float coupling the retrospective
flagged, and a rounded percentage would then disagree on screen with the byte counts printed beside
it. `EXPERIENCE.md`'s NFR8 is satisfied either way: it requires a finite clamped 0-100 percentage
for known positive totals, which the derived value is by construction.

A mutation sweep over all twelve ids found four implemented and defended by no test. Five tests were
added for them rather than deferring the gaps, and the story's own Verification section's mutation
list is extended accordingly in the evidence file.

## Design Notes

One seam, not a logger. The coordinator must not know what stderr is, and `internal/transfer` has no business importing a logging package; `app.go` already owns the one place FairDrop writes lines a human reads, and it already knows the AD-9 rules. So `recordDiagnostic` keeps writing to the sink that tests read and additionally calls an injected function, defaulting to a no-op, which `compose` wires to `logf`.

The panicking observer is recovered deliberately rather than left to crash. A panic in a UI callback is a frontend defect, not a reason to lose a transfer, and the lease it strands is what makes the app unusable afterwards. Recovering it and recording it is what turns an invisible wedge into a visible line.

## Verification

**Commands:**
- `gofmt -l .`; `go vet ./...`; `go tool staticcheck ./...`; `go test -count=1 -timeout 240s ./...`; `go test -count=1 -race -timeout 420s ./...` — clean
- `GOOS=darwin GOARCH=arm64 go vet ./...` and the linux equivalent, per AGENTS.md's pre-flight
- `cd frontend && npx vitest run`; then, alone, `wails build`
- Mutations: drop the seam call from `recordDiagnostic`; remove the panic recovery; revert the synthesis guard to `stateTransferring` only; discard the first `Stop`'s diagnostic; pass a path through a diagnostic — each must fail a named test
