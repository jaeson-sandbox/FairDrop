---
title: 'Story 5.1: Split internal/transfer/coordinator.go'
type: 'refactor'
created: '2026-09-17'
status: 'in-progress'
baseline_commit: '96284e9b43cacd6e54f27114d53d21dac031fdd6'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-5-context.md'
  - '{project-root}/AGENTS.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `internal/transfer/coordinator.go` is 1009 lines, the largest production file in the
repository and 29% larger than the next. Four of the concerns inside it -- a diagnostics ring
buffer, the session state machine, identity/URL construction, and two warning constructors -- have
nothing to do with coordinating a transfer, so every lifecycle change begins by scrolling past them.
Two retrospectives named the split and deferred it; a third of it then happened on an unspecced
branch, which is the pattern AGENTS.md's 2026-09-17 rule now forbids.

**Approach:** Move those four regions into four new files in the same package, carrying each
region's comments and its own constants with it. Change nothing else -- no renames, no signatures,
no behaviour, no restructuring of the coordination core.

## Boundaries & Constraints

**Always:** Every moved declaration stays in package `transfer`, byte-identical apart from its new
file. Comments move with the code they explain. A constant whose only consumers move with it moves
too (`maxDiagnostics`, `identityBytes`, `downloadPathPrefix`). The concurrency discipline the moved
code documents -- fields guarded by `Coordinator.mu`, the operation lease's happens-before edge,
`stop()` not being called under the state mutex -- is preserved verbatim in comment and in fact.

**Ask First:** Any move that would require editing a `_test.go` file. Any rename, signature change,
or new exported identifier. Moving anything beyond the four named regions -- in particular the
operation lease or the publish family, which stay put by design.

**Never:** Touch `Stage`, `AuthorizeClaim`, or the unwind family (`failStage`, `unwind`,
`releaseAcquired`, `afterStep`). Move `beaconInstanceBase`, which `Stage` uses at line 534. Create a
new package or a new import edge. Alter any pinned timing constant. Re-point the line-numbered
citations of this file in the retrospectives, the Story 1.5/3.5 artifacts, or discharged ledger
entries D-099 and D-103 -- those are closed historical records, and re-numbering them would falsify
the record rather than maintain it.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| The move is faithful | Split applied, suite run | `go test ./...` passes with **zero `_test.go` files changed** | A required test edit means it was a redesign, not a move -- HALT and ask |
| The surface is untouched | `go doc` before vs after | Byte-identical output for the package | Any diff means an exported identifier moved or changed -- revert that part |
| The file actually shrank | `wc -l coordinator.go` | Under 830 lines; none of the four regions remains | Materially above 830 means something unscoped moved; materially below means scope was widened |
| A constant outlives its region | `maxDiagnostics` used only by the sink | Moves to `diagnostics.go` with it | A constant still used by the core (`beaconInstanceBase`) stays |
| Race discipline survives | `go test -race ./...` | Passes, unchanged | A new race means a guard was separated from what it guards -- revert |

</frozen-after-approval>

## Code Map

- `internal/transfer/coordinator.go` -- 1008 lines, the only file edited by removal. Measured
  regions to move (non-overlapping line union = **188 lines**, plus ~15 lines of constants):
  - *diagnostics* -- `type diagnostic`, `type diagnosticSink`, `var diagnosticOverflow`,
    `record`, `snapshot` (~108-166). Depends only on `sync`, `ErrorCode`, `ErrTransferFailed`,
    `maxDiagnostics`. No production caller outside this file.
  - *session* -- `type sessionState` + its const block (81-100), `type resource` + its const block
    (98-110), `type session` (155-196), `hold`, `stop`, `release` (196-224).
  - *identity* -- `newIdentity` (950-964), `randomHex` (966-981), `capabilityURL` (981-989), plus
    `identityBytes` (:20, used only at :971) and `downloadPathPrefix` (:26, used only at :984).
  - *warnings* -- `unportableNamesWarning`, `beaconWarning` (987-1008).
- `internal/transfer/bounded.go` -- the precedent. A prior same-package extraction of the
  bounded-call subsystem; follow its file shape rather than inventing one.
- `internal/transfer/coordinator_outcomes_test.go:967` -- references `maxDiagnostics`; same package,
  so the move needs no edit here. This is the check that the "zero test edits" bar is real.
- `internal/transfer/coordinator_stage_test.go:578` -- references `beaconWarning` in a comment,
  asserting against a literal deliberately. Leave it.
- `internal/transfer` has **no `init()` and no build tags** (verified) -- so a file split carries no
  initialization-order or constraint risk.
- `_bmad-output/specs/spec-fairdrop/SPEC.md` -- read-only. "The coordinator is the sole lifecycle
  owner" constrains ownership, not file layout, and a move-only refactor is not the
  evidence-driven divergence that would require updating it.

## Tasks & Acceptance

**Execution:**
- [x] `internal/transfer/diagnostics.go` -- create; move the five diagnostics declarations and
  `maxDiagnostics` -- the sink's only writer and reader now live beside it.
- [x] `internal/transfer/session.go` -- create; move `sessionState`, `resource`, their const blocks,
  `session` and its three methods -- the state machine is one unit.
- [x] `internal/transfer/identity.go` -- create; move `newIdentity`, `randomHex`, `capabilityURL`,
  `identityBytes`, `downloadPathPrefix` -- identity and the URL it builds.
- [x] `internal/transfer/warnings.go` -- create; move the two `Warning` constructors.
- [x] `internal/transfer/coordinator.go` -- remove exactly those declarations; change nothing else.
- [x] Verify no test file changed: `git diff --name-only` must list no `_test.go` path.

**Acceptance Criteria:**
- Given the split is applied, when `git diff --stat` is read, then no `_test.go` file appears in it.
- Given the package before and after, when `go doc -all ./internal/transfer` is diffed, then the
  output is byte-identical.
- Given `coordinator.go` after the move, when measured, then it is under 830 lines and contains none
  of `diagnosticSink`, `session`, `newIdentity`, or `beaconWarning`.
- Given each new file, when opened, then it declares one concern and carries the comments that
  explained that concern in the original.

## Evidence

[evidence-5-1-split-internal-transfer-coordinator-go.md](evidence-5-1-split-internal-transfer-coordinator-go.md)
-- the faithfulness proof (pure deletion, 194/194 lines verbatim), the `go doc` byte-comparison, the
six-mutation table and the matrix audit.

## Spec Change Log

## Design Notes

**Why 830 and not 750.** The epic first said 750; that was an estimate and wrong by ~56 lines. The
measured union of the four regions is 188 lines, plus ~15 of constants, taking 1008 to roughly 805.
750 was reachable only by also moving the operation lease and the publish family, which this story
keeps with the coordination core on purpose. The count follows the scope.

**The one honest cost.** `session`'s doc comment describes its fields as guarded by `Coordinator.mu`
and by the operation lease. After the move a reader needs both files to see the whole locking
discipline. This is accepted rather than hidden: the alternative is leaving an 86-line state machine
inside the file the split exists to shrink. The comment stays verbatim so the pointer to
`Coordinator.mu` remains explicit.

**Why the historical citations stay stale.** Of 119 ledger entries, two mention this file (D-099,
D-103) and both are `discharged`; zero open entries depend on it. Their evidence describes what was
true when written, as do the line-numbered citations in the Epic 1 and 3 retrospectives. Updating
them would rewrite a closed record.

## Verification

**Commands:**
- `gofmt -l .` -- expected: clean
- `go vet ./...`; `go tool staticcheck ./...` -- expected: clean
- `go test -count=1 ./...` -- expected: 8 packages ok
- `go test -count=1 -race ./...` -- expected: 8 packages ok
- `git diff --name-only` -- expected: no path ending `_test.go`
- `go doc -all ./internal/transfer` before and after -- expected: identical
- `wc -l internal/transfer/coordinator.go` -- expected: under 830
