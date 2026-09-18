# Evidence: Story 5.1: Split internal/transfer/coordinator.go

Baseline: `96284e9b43cacd6e54f27114d53d21dac031fdd6`.

## What a move-only refactor lets you prove

This story's acceptance bar is unusual, and deliberately so. Because nothing about the package's
behaviour or surface is supposed to change, faithfulness is checkable directly rather than inferred
from a passing suite. Three independent checks were run, and the first two are stronger than "the
tests still pass":

**1. The diff into `coordinator.go` is pure deletion.** `git diff 96284e9 -- internal/transfer/coordinator.go`
contains **0** lines beginning `+`. Nothing was rewritten on the way out; declarations were removed
and nothing took their place.

**2. Every removed line reappears verbatim.** Of the **194** non-blank lines deleted from
`coordinator.go`, **194** appear character-for-character in one of the four new files. Checked by set
membership against the union of `diagnostics.go`, `session.go`, `identity.go` and `warnings.go`, not
by reading the diff.

**3. The reverse holds too — almost.** Comparing each new file back against the original, the only
lines *not* present in the pre-split `coordinator.go` are:

| File | Novel lines | What they are |
|---|---|---|
| `diagnostics.go` | 5 | `import "sync"` (single-line form; the original used a grouped block) plus a 4-line file header |
| `session.go` | 5 | a 5-line file header |
| `identity.go` | 0 | — |
| `warnings.go` | 0 | — |

So the move is faithful, with two new file headers as the only added prose. Those headers are
**not** moved comments and were not specified; they are noted here rather than waved through. Their
content was checked rather than assumed: `diagnostics.go`'s header claims the sink is read by "a
test (or the injected `Diagnose` seam)", and that seam is real -- `Dependencies.Diagnose` at
`coordinator.go:89`, wired by `helpers_test.go:207`. The headers are accurate. They are also
inconsistent: two files have one and two do not. Raised for the review layers as a coherence
question, not a defect.

## The surface did not move

`go doc -all ./internal/transfer` was captured **before dispatching the implementer**, precisely so
this comparison could not be reconstructed after the fact from a tree that had already changed.

- Before: 594 lines, `sha256` prefix `372d1b53b589834f`
- After: 594 lines, `sha256` prefix `372d1b53b589834f`
- `diff` of the two files: no output.

Byte-identical. No exported identifier was added, removed, renamed, or re-documented.

## Mutation table

Faithfulness proves the code moved intact; it does not prove the moved code is still *live*. Six
mutations, one per concern, each applied alone and reverted before the next.

| # | Mutation | File | Result |
|---|---|---|---|
| R1 | `maxDiagnostics` 32 -> 8 | `diagnostics.go` | **SURVIVED** -- see below |
| R2 | `record` returns before recording | `diagnostics.go` | KILLED -- `TestAuthorizeClaimTreatsABeaconStopDiagnosticAsSafe`, `TestCancelSucceedsThroughACleanupDiagnostic`, `TestASchedulerThatWithholdsItsStopFunctionDoesNotKillTheDrainer` |
| R3 | `release` forgets nothing | `session.go` | KILLED -- `TestCancelFromEveryState` (incl. `/TRANSFERRING`), `TestAuthorizeClaimTreatsABeaconStopDiagnosticAsSafe` |
| R4 | `downloadPathPrefix` -> `/dl/` | `identity.go` | KILLED -- `TestStageCommitsOnlyWhenEveryResourceIsLive`, `TestStageAcquiresResourcesInContractOrder`, `TestStageCommitsWithAWarningWhenOnlyTheBeaconFails` |
| R5 | `identityBytes` 16 -> 4 | `identity.go` | KILLED -- `TestCancelFromEveryState` (`/STAGED`, `/CLAIMING`) |
| R6 | `beaconWarning` takes `ErrNameWarning` | `warnings.go` | KILLED -- `TestStageCommitsWithAWarningWhenOnlyTheBeaconFails` |

Five of six killed by named tests. Every new file has at least one mutation killed in it, so none of
the extracted code is decorative.

### R1: a vacuity the refactor exposed but did not cause (D-120)

Shrinking the sink's cap from 32 to 8 left the entire suite green.
`coordinator_outcomes_test.go`'s `TestTheDiagnosticSinkSaysWhenItStoppedRecording` writes
`maxDiagnostics + 8` entries and then asserts `len(entries) != maxDiagnostics`. Input and expectation
are both derived from the symbol under test, so they move together and the constant could hold any
value. That is vacuous-test pattern 1 in this project's notes: assert against a literal written at
the assertion site, never against the symbol under test.

The gap is narrower than it first looks. What that test exists for -- the overflow marker -- **is**
pinned: R2 was killed by it, and the marker's presence in the last slot and absence everywhere else
are both asserted against `diagnosticOverflow` directly. Only the numeric cap is unpinned, and the
constant's own comment treats 32 as a bound rather than a contract.

It was **not** fixed inside this story. The fix is one literal in a `_test.go` file, and this story's
defining acceptance criterion is that no `_test.go` file appears in its diff at all. Breaking that
bar to repair an unrelated pre-existing vacuity would have destroyed the single property that made a
move-only refactor provable in the first place. Recorded as D-120, closable immediately after this
story merges -- AGENTS.md's 2026-09-17 governance rule permits a test-only pin on a named branch
without a story.

## Matrix coverage audit

Every row of the spec's I/O matrix was exercised. For a refactor these are verification commands
rather than unit tests, which is the honest shape of the contract here.

| Row | How it was checked | Result |
|---|---|---|
| The move is faithful | `git diff --name-only`; plus the 194/194 verbatim check above | No `_test.go` path in the diff |
| The surface is untouched | `go doc -all` before vs after | Byte-identical, same sha |
| The file actually shrank | `wc -l` | 1008 -> **790**, under the 830 target |
| A constant outlives its region | `grep -l` per constant | `maxDiagnostics` -> `diagnostics.go`; `identityBytes`, `downloadPathPrefix` -> `identity.go`; **`beaconInstanceBase` stayed in `coordinator.go`** |
| Race discipline survives | `go test -race ./...` | 8 packages ok |

The fourth row is the one worth naming: moving `beaconInstanceBase` is the single tempting mistake
the spec forbids by name, because `Stage` uses it. It stayed.

## Verification

Re-run independently of the implementer's own run, not read from its report:

- `gofmt -l .` -- clean
- `go vet ./...` -- clean
- `go tool staticcheck ./...` -- clean
- `go test -count=1 ./...` -- 8 packages ok
- `go test -count=1 -race ./...` -- 8 packages ok (`CGO_ENABLED=1` confirmed before the run, so the
  detector was actually present rather than silently skipped)
- `git status --porcelain` -- four new files, one modified production file, no test file

Not run, and not in scope: `wails build`, the frontend suites, and the release workflow. This is a
backend-only, same-package file move with a byte-identical `go doc` surface and no exported-API
change, so no binding or frontend artifact can have moved. Stated rather than silently skipped.

## File sizes after the split

| File | Lines |
|---|---|
| `coordinator.go` | 790 (was 1008) |
| `session.go` | 106 |
| `diagnostics.go` | 61 |
| `identity.go` | 58 |
| `warnings.go` | 24 |

249 lines across four new files against 218 removed: the difference is four package clauses, four
import blocks and the two file headers.
