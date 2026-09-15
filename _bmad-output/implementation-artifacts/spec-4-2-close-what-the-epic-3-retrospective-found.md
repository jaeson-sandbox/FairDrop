---
title: 'Story 4.2: Close What the Epic 3 Retrospective Found'
type: 'chore'
created: '2026-09-14'
status: 'ready-for-dev'
baseline_commit: 'fa6ee8a'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-3-retro-2026-09-14.md'
  - '{project-root}/_bmad-output/implementation-artifacts/epic-3-context.md'
  - '{project-root}/AGENTS.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The Epic 3 retrospective found ten things that are cheap to close and were each verified
at source before being written down. Three are gaps in what the suite proves: a declared epic
requirement that no test reaches (WCAG 1.4.12 text-spacing), a settled decision whose three header
lines can each be deleted with the whole repository green (D-018), and D-088's backstop function,
whose body can be replaced with `return func() {}, true` against a green repo. Two more guarantees
are mutable the same way. Two are behaviours with no guard at all: a macOS subprocess with no
deadline in front of `wails.Run`, and a protection that disables itself with no log line. And four
comments now state things the code stopped doing — one of them badly enough that `go doc` prints it
under the wrong function.

**Approach:** Close each one the way the project already closes things: a named test that fails when
the guarantee is removed, a bound where a call can hang, a fixed diagnostic where a protection
switches itself off, and a corrected sentence where the code outgrew its comment. Nothing here
changes what the product does except the two places where the current behaviour is unbounded or
silent. The retrospective is the specification — every item below carries its own source and its
own verified failure mode.

## Boundaries & Constraints

**Always:** Every fix is proved by the mutation that motivated it — the same mutation the
retrospective ran to confirm the gap must now fail a test by name. A comment correction states what
is true rather than deleting the claim.

**Ask First:** Any change to what a user sees or is told. Nothing in this story should reach public
copy; if a fix seems to need it, that is a signal the item was misread.

**Never:** Loosen a guarantee to make a test easier to write. Weaken the D-018 headers, or the
threat-model reasoning behind them, on the strength of a review finding — the retrospective already
judged the behaviour sound and only the sentence wrong. Convert a deferred id into a fix here: the
six ids the retrospective deferred need the owner's judgment and stay deferred.

**Deferral rule for this story:** a review finding is fixed here unless it needs a human decision,
lives in a package this Code Map does not name, or is a genuinely different goal. Each deferred
entry states which.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| WCAG text-spacing overrides applied | rendered, 1.5x line height, 2x paragraph, 0.12em letter, 0.16em word | No horizontal page scroll, nothing clipped | Fails naming the element and the measurement |
| Any D-018 header deleted | the three response headers | A named test fails on the literal value | — |
| `acquireInstanceLock` always reports held | mutated body | A named test fails | — |
| Lock setup cannot open its file | unwritable config dir | Launch continues without the backstop, one fixed path-free line logged | Never the path (AD-9) |
| Server dies while CLAIMING | lane closes mid-claim | The drainer still synthesizes a failure, proved through `drain` | Fails if the production call site narrows |
| `StartBeacon` with no selection | selection cleared | Returns the warning **and** the gate is re-acquirable | Fails if the release is dropped |
| Ancestor eval fails | `EvalSymlinks` errors | The original selection is preserved unchanged | Fails if a path is built from a zero parent |
| `defaults read` never returns | wedged `cfprefsd` | Light theme after a short bound; `wails.Run` still reached | The bound is the answer, not a hang |
| A comment claims a count | a later story adds a second | The claim is written so it cannot go stale, or a test reads the count | — |

</frozen-after-approval>

## Code Map

- `frontend/browser/accessibility.test.tsx` — the rendered suite Story 3.12 built. `doubleTextTokens`
  is the model for the text-spacing case: override on the live document, measure, restore in
  `afterEach`. `frontend/src/style.css` sets `letter-spacing` on three selectors, which is exactly
  what a 1.4.12 user stylesheet overrides.
- `internal/server/handler.go` — `writeStatus` (the rejection headers, and the D-018 rationale whose
  last clause is false) and `writeDownloadHeaders` (the two success headers).
  `internal/server/handler_test.go` — `TestRejectionsAreIndistinguishable` compares rejections only
  to each other, which is why all three headers are unpinned; the literal assertions belong beside
  `TestSuccessfulDownloadServesHeadersBodyAndOneCompleteEvent`.
- `instance_lock.go` — `acquireInstanceLock`'s three setup-failure branches and the missing
  diagnostic. `main_test.go` tests `lockFileExclusive` and takes `held` as a parameter, so the
  function itself is untested; `main.go:223` is its only caller.
- `internal/transfer/outcomes.go` — `drain`'s synthesis call site and its state list.
  `internal/transfer/coordinator_stage_test.go` holds the test that hands the guard its own literal.
- `internal/network/beacon.go` — `StartBeacon`'s unselected branch; `beacon_test.go:325` shows the
  sibling pattern that proves a gate came back.
- `selection_source.go` — `resolveAncestorsWith`'s eval-failure branch, reachable only through a
  stub.
- `theme_darwin.go` — the unbounded `defaults read`; `theme_windows.go` is the sibling that cannot
  hang, and `main.go` calls both before `wails.Run`.
- `internal/transfer/types.go`, `internal/transfer/coordinator.go`, `app.go`, `main.go`,
  `internal/server/lifecycle.go` — the five comments the code outgrew.

## Tasks & Acceptance

**Execution:**
- [ ] Retro item 12 — the WCAG 1.4.12 text-spacing case in the rendered suite, failing on a measured value.
- [ ] Retro item 13 — literal-value assertions for all three D-018 response headers.
- [ ] Retro item 14 — a test that drives `acquireInstanceLock` itself.
- [ ] Retro item 15 — CLAIMING driven through `drain`, so narrowing the production call site fails by name.
- [ ] Retro item 16 — the selection gate proved re-acquirable after the unselected `StartBeacon` branch.
- [ ] Retro item 17 — `resolveAncestorsWith` driven with a failing eval stub.
- [ ] Retro item 18 — a deadline on `theme_darwin.go`'s `defaults read`.
- [ ] Retro item 19 — one fixed, path-free line when the instance lock's setup fails.
- [ ] Retro item 20 — the four falsified comments corrected and `beaconWarning`'s doc comment reattached.
- [ ] Retro item 21 — `writeStatus`'s D-018 rationale and `Stop()`'s docstring corrected.
- [ ] `evidence-4-2-close-what-the-epic-3-retrospective-found.md`, the ten items marked `done`, `epics.md` kept in step.

**Acceptance Criteria:**
- Given each mutation the retrospective ran to confirm a gap, when it is re-applied, then a named test fails.
- Given the built frontend under WCAG text-spacing overrides, when the rendered suite runs, then a clipped control or a horizontal page scroll fails by name rather than by substring.
- Given a `defaults read` that never returns, when FairDrop launches, then the window still opens on the light canvas rather than never opening.
- Given a comment that states a count or a uniqueness claim, when a later story adds a second, then either the claim is still true or a test fails.

## Evidence

Audit, mutation tables and gate transcripts live in
[evidence-4-2-close-what-the-epic-3-retrospective-found.md](evidence-4-2-close-what-the-epic-3-retrospective-found.md),
created with the implementation.

## Spec Change Log

**2026-09-14 (creation).** Scoped to the ten retrospective items that carry a verified source and a
named fix. Retro items 22 (whether the three stream copy loops stay independent) and 23 (splitting
`internal/transfer/coordinator.go`) are deliberately excluded: the first needs the owner's decision
and the second is a refactor whose cheapest moment is a story that is not also fixing ten unrelated
things. Both remain open action items.

The six ids the retrospective deferred stay deferred and are not in scope: the Complete event
dropped when a stop races finalization, the wedged ancestor resolution that makes `busy` permanent,
the per-snapshot publish goroutine, the inert second window on macOS, the beacon stop in front of
the receiver's first byte, and `StopBeacon` answering with "did not start".

## Design Notes

**The retrospective is the specification.** Every item was verified at source before it was written
down, and most were verified by mutation — break the guarantee, run the suite, watch nothing fail.
That is why this story has no investigation phase: the failure modes are already established, and
the work is to make each one fail a named test instead.

**Two behaviour changes, eight proofs.** Only the `defaults read` deadline and the instance-lock
diagnostic change what a running FairDrop does. Everything else either adds a test to code that is
already correct, or corrects a sentence. That ratio is itself the retrospective's finding: the
epic's code was in better shape than its prose.

**A comment correction is not a comment deletion.** The four falsified claims are load-bearing —
they explain why a guard exists, which is how the next session learns not to remove it. Each is
rewritten to state what is now true, not trimmed to avoid being wrong.

## Verification

**Commands:**
- `gofmt -l .`; `go vet ./...`; `go tool staticcheck ./...`; `go test -count=1 ./...` — clean
- `cd frontend && npx tsc --noEmit -p tsconfig.json`; `npx vitest run`; `npm run test:browser`
- `go doc -all -u ./internal/transfer` — `beaconWarning` documents itself again
- The native run read with `gh run view --json conclusion,jobs`
- Mutations: every one the retrospective ran to confirm a gap, re-applied, each now failing a named test
