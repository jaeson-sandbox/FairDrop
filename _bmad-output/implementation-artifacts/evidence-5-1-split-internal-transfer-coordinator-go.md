# Evidence: Story 5.1: Split internal/transfer/coordinator.go

Baseline: `96284e9b43cacd6e54f27114d53d21dac031fdd6`. Measurements taken at `6ff98d5` and re-taken
after the review patches; where a number moved, the later one is given.

## What a move-only refactor lets you prove

Because nothing about the package's behaviour is supposed to change, faithfulness is checkable
directly rather than inferred from a passing suite.

**1. The diff into `coordinator.go` is pure deletion.** `git diff 96284e9 -- internal/transfer/coordinator.go`
contains **0** lines beginning `+`. Independently reproduced by two review layers.

**2. Every removed line reappears verbatim.** Of the **194** non-blank lines deleted, **194** appear
character-for-character in the union of the four new files.

*What this check cannot see, stated because it was first presented without the caveat.* It is set
membership over distinct lines -- the 194 collapse to 165 unique -- so it is blind to grouping,
order and count. A line that exists anywhere in the union satisfies it. The `const` regrouping in
section 3 is the proof: a genuine structural change that this check reports as "0 novel lines" for
`identity.go`, because `const (` and `)` both existed in the original.

**3. The reverse direction, and the one thing it found.** Comparing each new file back against the
original, the only lines *not* present in the pre-split `coordinator.go` were `import "sync"` and
two file headers. Those headers are addressed in "What the review changed" below.

## The `go doc` check was vacuous, and it was mine

The evidence originally presented `go doc -all ./internal/transfer` as the second of three
faithfulness checks and called the set "stronger than 'the tests still pass'". That was wrong, and
the Blind Hunter layer caught it.

`go doc -all` prints only **exported** symbols. Every one of the 194 moved lines is unexported.
Grepping the captured 594-line output for `diagnosticSink`, `sessionState`, `randomHex`,
`capabilityURL`, `beaconWarning`, `maxDiagnostics`, `identityBytes` and `downloadPathPrefix` returns
**zero hits for all eight**. The check is accurate as literally worded -- no exported identifier
changed -- but it is insensitive to everything this story touched. It could not have failed.

This is the same shape as the vacuity Story 4.1 found in its own 640x480 browser case, recorded in
this project's notes hours earlier: a measurement that cannot move when the thing it claims to
protect moves. Finding it again, in a check written to guard against exactly that, is the finding
worth keeping from this story.

**The covering command, run since:** `go doc -all -u` (which includes unexported symbols), compared
across a throwaway worktree at `96284e9` and the current tree, differs by **36 lines**.
`identityBytes`, `downloadPathPrefix` and `maxDiagnostics` now render as three separate `const ( … )`
groups rather than as members of the coordinator's single block. No doc text changed and nothing is
lost, but the package's documented shape did change -- which is precisely what the "byte-identical"
bar existed to surface, and would have surfaced on day one had the covering command been used.

## Mutation table

Fifteen mutations, each applied alone and reverted before the next. The first six were run before
review; the rest were added because the Blind Hunter layer observed that "six mutations, one per
concern" was not what the table showed -- four concerns, six mutations, with several declarations
never exercised.

| # | Mutation | File | Result |
|---|---|---|---|
| R1 | `maxDiagnostics` 32 -> 8 | `diagnostics.go` | **SURVIVED** -- D-120 |
| R2 | `record` returns before recording | `diagnostics.go` | KILLED -- `TestAuthorizeClaimTreatsABeaconStopDiagnosticAsSafe`, `TestCancelSucceedsThroughACleanupDiagnostic`, +1 |
| R3 | `release` forgets nothing | `session.go` | KILLED -- `TestCancelFromEveryState` (incl. `/TRANSFERRING`) |
| R4 | `downloadPathPrefix` -> `/dl/` | `identity.go` | KILLED -- `TestStageCommitsOnlyWhenEveryResourceIsLive`, +2 |
| R5 | `identityBytes` 16 -> 4 | `identity.go` | KILLED -- `TestCancelFromEveryState` (`/STAGED`, `/CLAIMING`) |
| R6 | `beaconWarning` takes `ErrNameWarning` | `warnings.go` | KILLED -- `TestStageCommitsWithAWarningWhenOnlyTheBeaconFails` |
| W1 | `unportableNamesWarning` takes the beacon code | `warnings.go` | **SURVIVED** -- D-121 |
| W2 | `unportableNamesWarning` takes the wrong registry entry | `warnings.go` | **SURVIVED** -- D-121 |
| X1 | `session.hold` forgets the resource | `session.go` | KILLED **by timeout, not by assertion** -- see below |
| X2 | `session.stop` never cancels | `session.go` | KILLED -- `TestResetTimerClearsTheSessionAndReturnsToIdle`, `TestShutdownQuiescesEverythingAndPublishesNothing`, +4 |
| X3 | `snapshot` returns nothing | `diagnostics.go` | KILLED -- `TestEveryRecordedDiagnosticAlsoReachesTheSeam`, `TestTheDiagnosticSinkSaysWhenItStoppedRecording`, +6 |
| X4 | a `sessionState` string value changes | `session.go` | **SURVIVED**, and correctly -- see below |
| X5 | `capabilityURL` drops the `http://` scheme | `identity.go` | KILLED -- `TestNativePathClassesStageAndDownload`, +5 |
| X6 | `randomHex` loses its nil-entropy fallback | `identity.go` | INVALID -- build failure, not a mutant |
| -- | `unportableNamesWarning` body replaced wholesale | `warnings.go` | INVALID -- left `public` unused, build failure |

**X1 is a caveat, not a pass.** Gutting `session.hold` does fail CI, but the failure arrives as a
60-second test timeout rather than a named assertion. Recorded because this project's practice is to
name the test that kills each mutation, and here there is none to name.

**X4 survived and should have.** The `sessionState` string values are opaque internal labels: every
comparison goes through the constant, so changing `"STAGING"` to `"STAGINGX"` moves both sides at
once. The only literal occurrences anywhere else are subtest *names* in
`coordinator_lifecycle_test.go`, not assertions. Nothing observable depends on the value. Not every
surviving mutation is a finding, and calling this one a gap would be noise.

**Two invalid mutants are listed rather than dropped.** Both produced build failures, which are not
killed mutants -- a compiler error proves nothing about test coverage. Listing them keeps the
denominator honest: fifteen attempts, thirteen valid, ten killed, three survived.

### R1: a vacuity the refactor exposed but did not cause (D-120)

Shrinking the sink's cap from 32 to 8 left the suite green.
`coordinator_outcomes_test.go` drives `maxDiagnostics + 8` entries (`:967`) and asserts
`len(entries) != maxDiagnostics` (`:975`), so both sides move with the constant.

Two corrections to how this was first written. The claim "`maxDiagnostics` could be any value and
the test would pass identically" is **false at the boundary**: at 0 the test fails with "the sink
holds 1 entries, want exactly its 0 cap". The floor is pinned; only the magnitude is not. And the
account of what the test *does* pin omitted its third assertion (`:988`), which requires the
`Diagnose` seam to have seen all `maxDiagnostics + 8` diagnostics -- "the sink's cap bounds what is
kept, never what a running FairDrop is told". That assertion is correctly derived on both sides and
must stay derived. The fix touches `:975` and its message at `:976` only.

### W1/W2: `warnings.go`'s other half is unobserved (D-121)

Found by the Verification Gap layer and reproduced here. `unportableNamesWarning` can be given the
wrong warning code *and* the wrong registry entry with all 8 packages green. No test in the
repository stages an item with `UnportableNames > 0`: the transfer harness leaves it at zero, and
the only two `inspect` overrides return errors, so the branch never executes.

This **falsifies** the claim this evidence made in its first version -- "every new file has at least
one mutation killed in it, so none of the extracted code is decorative". `warnings.go` holds two
functions and only `beaconWarning` was pinned. The corrected claim is in the table above.

The consequence is not cosmetic: `frontend/src/transfer/validation.ts:66` rejects any warning code
outside `beacon_warning`/`name_warning` and returns `null` for the entire Staged metadata record, so
a wrong code here would show the sender nothing staged rather than a mislabelled warning.

## Acceptance criteria audit

The first version of this file audited the five I/O-matrix rows and never walked the four Acceptance
Criteria. Both are audited now.

| Criterion | Result |
|---|---|
| No `_test.go` file in `git diff --stat` | Met. The story's own diff touches no test file. |
| `go doc -all` byte-identical | Met **as worded**, and vacuous as evidence -- see above. `go doc -all -u` differs by 36 lines (const regrouping). |
| Under 830 lines, no declaration of the four regions | Met, after the criterion was corrected. As originally worded it could not pass: `session` appears 47 times in `coordinator.go` as a type reference, and `diagnosticSink`, `newIdentity` and `beaconWarning` each appear once as a field or call. The intent was "no declaration of", verified by grep. |
| Each file declares one concern and carries its comments | Met, after the review. All four now carry a file header matching `bounded.go`, the precedent the Code Map names. |

### I/O matrix rows

| Row | Result |
|---|---|
| The move is faithful | No `_test.go` in the diff; 194/194 verbatim (with the caveat above) |
| The surface is untouched | Vacuous as run; the covering command shows a benign 36-line shape change |
| The file actually shrank | 1008 -> **790**, under 830 |
| A constant outlives its region | `maxDiagnostics` -> `diagnostics.go`; `identityBytes`, `downloadPathPrefix` -> `identity.go`; **`beaconInstanceBase` stayed** |
| Race discipline survives | `go test -race ./...` 8 packages ok |

**Reconciling "materially below 830", which the matrix asked and the first audit did not answer.**
The spec predicted ~805 and the result is 790. The prediction counted only the 188 non-blank
declaration lines plus ~15 of constants; in fact **218** lines left `coordinator.go` -- 194 non-blank
and **24 blank separators** that went with them. Nothing unscoped moved. The gap is blank lines.

## Line arithmetic

The first version stated this wrongly ("four import blocks"; figures that did not reconcile).
Measured:

| | Non-blank | Blank | Total |
|---|---|---|---|
| Removed from `coordinator.go` | 194 | 24 | 218 |
| Added across four new files | 220 | 29 | 249 |

The +26 non-blank is 4 package clauses + 11 import lines across **three** import blocks
(`warnings.go` has none) + the file headers + 2 extra `const` block delimiters. The +5 blank is
separator lines inside the new files.

## What the review changed

- **A false comment, fixed.** `diagnostics.go`'s header said the sink is what "a test (or the
  injected `Diagnose` seam) reads from". `coordinator.go:80-88` and `recordDiagnostic` make
  `Diagnose` a *parallel write destination*, not a reader -- it keeps receiving calls after the sink's
  cap stops keeping them. The header now says that. This evidence had certified the headers accurate
  on the strength of confirming the seam *existed*, without checking the relationship the sentence
  asserted.
- **The header diagnosis, reversed.** The first version called the headers "not specified" and the
  two-with/two-without split "a coherence question, not a defect". The Code Map names `bounded.go` as
  the precedent and says to follow its file shape; `bounded.go` carries a file header. Headers are
  specified by reference, so the *deviation* was `identity.go` and `warnings.go` lacking one. Both
  now have one.
- **Acceptance Criterion 3, reworded** to say "no declaration of", which is what it meant.

## Verification

Re-run independently of the implementer's report, and again after the review patches:

- `gofmt -l .`, `go vet ./...`, `go tool staticcheck ./...` -- clean
- `go test -count=1 ./...` -- 8 packages ok
- `go test -count=1 -race ./...` -- 8 packages ok (`CGO_ENABLED=1` confirmed, so the detector was
  present rather than silently skipped)
- `go doc -all -u` before vs after -- 36-line diff, const regrouping only

Not run, and out of scope: `wails build`, the frontend suites, the release workflow. This is a
backend-only, same-package file move with no exported-API change. `scripts/verify-native-mutations.sh`
was also not run; it carries a hard-coded backup list naming `internal/transfer/coordinator.go` and
`bounded.go`, and the four new files are absent from it (D-124).

## A process hazard worth recording

A review subagent mutated the shared working tree to test a guard and did not restore it:
`session.go`'s `hold` was found gutted to `_ = held` mid-review. It was caught by another layer,
reverted, and never reached a commit -- `6ff98d5` predates it, and origin and CI both hold correct
code. But three agents reviewing against one working tree is the same collision that produced a red
commit earlier in this project. Future parallel review should use worktrees; one of the three layers
did exactly that unprompted and left the tree clean.

## Deferrals

- **D-120** -- the diagnostic sink's cap is asserted against itself. Fixed on this branch as a
  test-only commit, which `AGENTS.md` permits without a story.
- **D-121** -- `unportableNamesWarning` is executed by no test. Fixed on this branch as a test-only
  commit.
- **D-122** -- `releaseAcquired`'s switch over `resource` has no `default`, and the `resource`
  constants now live in a different file, so a third resource would compile and silently never be
  released. Not fixed here: `releaseAcquired` is named in this spec's **Never** clause.
- **D-123** -- `capabilityURL` narrows `port` to `uint16` unchecked; its only range guard is at
  `coordinator.go:336`, now a file away.
- **D-124** -- `scripts/verify-native-mutations.sh` carries a hard-coded file list that does not
  include the four new files.
- **D-125** -- `randomHex`'s `if source == nil` fallback is unreachable: `NewCoordinator` already
  defaults `entropy`, and it is the only construction site.
