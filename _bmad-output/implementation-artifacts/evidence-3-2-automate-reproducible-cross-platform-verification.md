# Evidence: Story 3.2 -- Automate Reproducible Cross-Platform Verification

Linked from [spec-3-2-automate-reproducible-cross-platform-verification.md](spec-3-2-automate-reproducible-cross-platform-verification.md).

**Scope of this file.** This story was implemented across two sessions. The first
(commit `9b0e10e`, "feat(verify): enforce staticcheck and close the recorded
test-quality debts") did the `.gitattributes`-independent staticcheck work and the
D-045 and D-072 fixes; its mutation evidence is recorded in that commit's message,
not here. This file covers only the remaining work from the spec's Tasks &
Acceptance list: the `.gitattributes` line-ending normalisation,
`.github/workflows/verify.yml`, `verify_workflow_test.go`, the `AGENTS.md`/
`README.md`/`deferred-work.md` updates, and the gate transcript for all of it
together. The D-038 review-layer triage is owned by the orchestrator and is
appended to this file separately -- it is not in this section.

## 1. `.gitattributes` and the one-time worktree rewrite (D-063)

`.gitattributes` gained a catch-all `* text=auto eol=lf` (first in the file) and
explicit `binary` declarations for `*.png *.ico *.icns *.woff2 *.pyc`, on top of
the existing `*.go`/`*.css`/`*.ts`/`*.tsx`/`*.js`/`go.mod`/`go.sum` pins.

The worktree was then rewritten once: `git ls-files --eol -z` was parsed for every
entry whose working-tree column read `w/crlf` or `w/mixed` (594 files), and each
was rewritten in place with `sed -i 's/\r$//'`, stripping the trailing `\r` with no
other content change. `git add --renormalize .` was then run to clear the index's
stale per-entry normalisation flags -- without it, `git status` reports all 594
paths as modified even though `git hash-object` on each produces the exact blob
hash already in the index (verified byte-for-byte on a sample file: `cmp` against
`git cat-file -p <blob>` was identical, 5015 bytes both sides). `.gitattributes`
itself was then restaged out again (`git restore --staged .gitattributes`) so only
its real content edit remains an unstaged, uncommitted change, matching every
other file this session touched.

Verification:
- `git ls-files --eol | grep -E '(i|w)/(crlf|mixed)'` -- empty (exit 1, no matches).
- `gofmt -l .` -- clean, before and after the rewrite.
- `cd frontend && npx vitest run` -- 17 files / 490 tests passed, before and after.
- `go build ./...` -- clean.
- `git status --short` after the rewrite and renormalize shows no file the
  rewrite touched; only genuinely-edited files (`.gitattributes` itself, and
  later `AGENTS.md`/`README.md`/`deferred-work.md`/the new workflow/test files)
  appear.

## 2. `.github/workflows/verify.yml`

One job (`verify`), matrixed over `os: [windows-latest, macos-latest]`, `shell:
bash` throughout, steps in the fixed order the spec's Always clause states:
`wails build` -> bindings-drift/`.gitkeep` check -> `gofmt -l` -> `go vet` ->
`go tool staticcheck` -> `go test` -> an explicit cgo check -> `go test -race` ->
frontend suite -> line-ending check.

Design notes:
- Triggers are `pull_request` and `push` to `main`/`epic-*` only, matching the
  established fact that the repo is public and `main` is unprotected --
  branches are named explicitly rather than relying on branch protection.
- `concurrency` groups on `${{ github.event.pull_request.number || github.ref
  }}` with `cancel-in-progress: true`, so a superseded run is cancelled.
- `actions/checkout@v7`, `actions/setup-go@v7` (`go-version: '1.26.7'`),
  `actions/setup-node@v7` (`node-version-file: .nvmrc`, `cache: npm`,
  `cache-dependency-path: frontend/package-lock.json`), `actions/cache@v6` on
  `~/go/bin/wails*` keyed by `WAILS_VERSION`/`runner.os`/`runner.arch`.
- `$(go env GOPATH)/bin` is added to `$GITHUB_PATH` explicitly in its own step,
  rather than assumed from `setup-go`.
- The Wails CLI version lives in one place (`env.WAILS_VERSION: 'v2.15.0'`) and
  is asserted by actually running `wails version` and comparing its captured
  output, before `wails build` runs -- an unpinned or stale-cached CLI fails
  the job before any build happens.
- The cgo check (ahead of `-race`) does two things, not one: it reads
  `CGO_ENABLED` *and* writes and `go run`s a two-line `import "C"` program.
  `CGO_ENABLED=1` alone does not prove a working C toolchain (AGENTS.md's own
  pitfall note), so the check only trusts an actual compile-and-run.
- The comment directly above the `go test -race` step explains why the step is
  native and C-toolchain-dependent, per the spec's Always clause.
- `npm test` (not `--omit=dev`) runs after `wails build`'s own full `npm ci`,
  never concurrently with it -- the whole job is one step at a time.

Local validation (no YAML linter is installed, so validation used a
throwaway, repo-external Go module against `gopkg.in/yaml.v3` -- never added
as a dependency of the FairDrop module itself):
- The file parses as valid YAML; `jobs.verify.steps` lists all 17 steps in the
  order above.
- Every `run: |` block was exercised locally, standalone, including the cgo
  probe (confirmed it builds and runs against this machine's real MSYS2 gcc)
  and the wails-version assertion (confirmed `wails version | head -n1 | tr -d
  '\r'` reads `v2.15.0` against the locally installed CLI).
- `wails build`, the bindings-drift/`.gitkeep` check, and the line-ending check
  were each run for real against this tree (see Section 4) and pass.

**Not yet done:** the workflow has not been pushed, so no GitHub Actions run
exists yet and no run URL can be recorded. That is explicitly out of this
session's scope (no commit, no push) -- see "Left incomplete" below.

## 3. `verify_workflow_test.go` -- mutation table

17 tests, each pinning one clause of the spec's Always/Never list, reading
`.github/workflows/verify.yml` and `wails.json` as plain text (no YAML
library, matching `TestEveryDeferredEntryHasALiveOwner`'s precedent). Every
row below was produced by actually editing the workflow (or `wails.json`) on
disk, running `go test -run TestVerifyWorkflow`, confirming the expected test
failed and named the break, then restoring the file from a saved-clean copy
and confirming the full suite passed again before moving to the next
mutation. Two rounds of hardening are included in the table: the first
version of three checks matched their own target words inside comments or
failure-message strings rather than functional YAML, so those mutations
initially survived; the checks were tightened to match the actual
command/comparison text, and the same mutations were re-run and re-caught.

| # | Mutation | Test(s) that caught it | Result |
|---|----------|------------------------|--------|
| 1 | Remove `windows-latest` from the matrix | `TestVerifyWorkflowRunsOnlyOnNativeWindowsAndMacRunners` | FAIL, named |
| 2 | Remove `macos-latest` from the matrix | same | FAIL, named |
| 3 | Add `ubuntu-latest` to the matrix | same | FAIL, named |
| 4 | Loosen `go-version` `'1.26.7'` -> `'1.25.0'` | `TestVerifyWorkflowPinsTheGoToolchain` | FAIL, named |
| 5 | Delete the `pull_request:` trigger | `TestVerifyWorkflowTriggersOnPullRequestAndTheRightPushBranches` | FAIL, named |
| 6 | Remove `epic-*` from the push branches | same | FAIL, named |
| 7 | `cancel-in-progress: true` -> `false` | `TestVerifyWorkflowCancelsSupersededRuns` | FAIL, named |
| 8 | Downgrade `actions/checkout@v7` -> `@v6` | `TestVerifyWorkflowPinsActionMajors` | FAIL, named |
| 9 | `WAILS_VERSION` `'v2.15.0'` -> `'v2.14.0'` | `TestVerifyWorkflowPinsAndAssertsTheWailsCLIVersion` | FAIL, named |
| 10 | Hardcode `got="v2.15.0"` instead of calling `wails version` | same *(initially survived -- the bare "wails version" substring check matched the step's own failure-message text; tightened to require `got="$(wails version` )* | FAIL, named after fix |
| 11 | Compare `$got` against a literal instead of `$WAILS_VERSION` | same *(initially survived for the same reason as #10; tightened to require `!= "$WAILS_VERSION"`)* | FAIL, named after fix |
| 12 | Smuggle `NPM_CONFIG_OMIT=dev` into the `wails build` run line (env-var equivalent of `--omit=dev`) | `TestVerifyWorkflowNeverInstallsProductionOnly`, `TestVerifyWorkflowNeverUsesAForbiddenFlag` *(initially survived -- the checks only matched the literal flag `--omit=dev`; broadened to a case-insensitive `omit=dev` substring)* | FAIL, named after fix |
| 13 | `wails.json`'s `"frontend:install"` gains `--omit=dev` | `TestVerifyWorkflowNeverInstallsProductionOnly` | FAIL, named |
| 14 | Swap `go tool staticcheck ./...` for `golangci-lint run ./...` | `TestVerifyWorkflowRunsTheFixedGoCommands` | FAIL, named |
| 15 | Swap the order of the `gofmt -l` and `go vet` step headers | `TestVerifyWorkflowRunsEveryStepInTheRequiredOrder` | FAIL, named |
| 16 | Remove the word "native" from the race step's explanatory comment | `TestVerifyWorkflowRaceStepIsExplainedAndGuardedByACgoCheck` | FAIL, named |
| 17 | Remove `import "C"` from the cgo probe | `TestVerifyWorkflowCgoCheckActuallyBuildsACgoProgram` | FAIL, named |
| 18 | Loosen the line-ending grep to `grep crlf` | `TestVerifyWorkflowChecksLineEndings` | FAIL, named |
| 19 | Remove `working-directory: frontend` from the frontend-suite step | `TestVerifyWorkflowRunsTheFrontendSuite` | FAIL, named |
| 20 | Remove `shell: bash` | `TestVerifyWorkflowUsesBashAndAddsGoPathBinExplicitly` | FAIL, named |
| 21 | Replace the `$(go env GOPATH)/bin` -> `$GITHUB_PATH` command with a no-op | same *(initially survived -- the words "go env GOPATH" and "GITHUB_PATH" also appear in the step's own comment; tightened to require the exact command line)* | FAIL, named after fix |
| 22 | Remove the `frontend/dist/.gitkeep` existence check (leave the `wailsjs` drift check) | `TestVerifyWorkflowChecksBindingsDriftAndGitkeep` | FAIL, named |
| 23 | Downgrade `actions/setup-node@v7` -> `@v5` | `TestVerifyWorkflowPinsNodeFromNvmrc`, `TestVerifyWorkflowPinsActionMajors` | FAIL, named |
| 24 | Downgrade `actions/cache@v6` -> `@v4` | `TestVerifyWorkflowPinsActionMajors` | FAIL, named |
| 25 | Hardcode `node-version: 24` instead of `node-version-file: .nvmrc` | `TestVerifyWorkflowPinsNodeFromNvmrc` | FAIL, named |

A genuine false-positive was also found and fixed before this table was
produced: the first draft of the `wails build` step's own comment explained
the constraint using the literal words "never `--omit=dev`", which made
`TestVerifyWorkflowNeverInstallsProductionOnly` and
`TestVerifyWorkflowNeverUsesAForbiddenFlag` fail against the *clean* file --
a check matching its own explanatory prose rather than functional YAML. The
comment was reworded to describe the constraint without spelling out the
forbidden string, which is the same class of bug mutations 10, 11, 12 and 21
above surfaced deliberately.

After every mutation was re-verified caught, the file was restored from the
saved-clean copy and the full suite (`go test -count=1 -run TestVerifyWorkflow
.`, 17/17 tests) was confirmed green one final time, alongside `go vet ./...`
and `TestEveryDeferredEntryHasALiveOwner` (unaffected by this story's changes,
confirmed still passing after the `deferred-work.md` owner edits below).

## 4. `AGENTS.md`, `README.md`, `deferred-work.md`

- **`AGENTS.md`** ("Running and verifying"): names
  `.github/workflows/verify.yml` as the canonical gate and states its full
  step order; adds `go tool staticcheck ./...` as the fixed lint command, run
  from the `go.mod` tool directive rather than a separately installed binary;
  and records that `//lint:ignore SA1012 <reason>` (on the line above the
  finding) is staticcheck's real suppression syntax, while
  `//nolint:staticcheck` is golangci-lint syntax that staticcheck does not
  read -- the exact confusion the fifteen fixed findings included five
  instances of.
- **`README.md`** ("Build, run, verify"): adds a paragraph naming the workflow
  as canonical and stating that `verify_workflow_test.go` pins it; adds `go
  tool staticcheck ./...` to the local command block between `go vet` and `go
  test`.
- **`deferred-work.md`**: eight of the nine ids the spec's intent names are
  closed (D-005, D-016, D-045, D-050, D-054, D-063, D-072 -> `discharged`;
  D-009 -> `accepted`, since `npm run build` genuinely cannot run against a
  dev-omitting install and the finding stays true by construction rather than
  being fixed away). D-038's row is untouched, per this session's explicit
  scope boundary -- its owner and evidence are the orchestrator's to write. A
  new banner, `**Discharged (Story 3.2):**`, was inserted after the general
  "every entry now carries a stable id" note (ahead of the existing Story 2.2
  banner), summarising the mechanism that closed each of the eight. The two
  id-less entries under `spec-1-10-...md` that also name this story (the
  unreproduced `internal/transfer` flake, and Story 1.10's thin review
  coverage) were left alone: neither carries an `id:`, neither is among the
  nine ids the spec's intent enumerates, and neither is in this story's
  Tasks & Acceptance list.
- `TestEveryDeferredEntryHasALiveOwner` passes after all of the above (every
  owner is still either `discharged`, `accepted`, or a live `sprint-status.yaml`
  key).

## 5. Gate transcript (this session's final verification pass)

Run one at a time, per AGENTS.md: the Go gate, then the frontend suite, then
`wails build` alone -- never Vitest concurrently with `wails build`.

```
$ go env CGO_ENABLED
1

$ gofmt -l .
(empty -- clean)

$ go vet ./...
(clean, exit 0)

$ go tool staticcheck ./...
(clean, exit 0)

$ go test -count=1 ./...
ok  	fairdrop	0.134s
ok  	fairdrop/internal/network	0.236s
ok  	fairdrop/internal/qr	0.220s
ok  	fairdrop/internal/server	1.626s
ok  	fairdrop/internal/source	0.308s
ok  	fairdrop/internal/stream	0.764s
ok  	fairdrop/internal/transfer	0.510s

$ go test -count=1 -race ./...
ok  	fairdrop	1.736s
ok  	fairdrop/internal/network	1.162s
ok  	fairdrop/internal/qr	1.410s
ok  	fairdrop/internal/server	2.505s
ok  	fairdrop/internal/source	1.288s
ok  	fairdrop/internal/stream	4.973s
ok  	fairdrop/internal/transfer	1.457s

$ cd frontend && npx vitest run
 Test Files  17 passed (17)
      Tests  490 passed (490)

$ wails build          # run alone, after the frontend suite finished
Built 'C:\Users\jaeso\Coding\FairDrop\build\bin\fairdrop.exe' in 10.51s.

$ test -f frontend/dist/.gitkeep && echo YES
YES

$ git diff --quiet -- frontend/wailsjs && echo "NO DRIFT"
NO DRIFT

$ git ls-files --eol | grep -E '(i|w)/(crlf|mixed)'
(empty -- clean)

$ gofmt -l .            # re-checked after wails build
(empty -- clean)
```

`go test -count=1 -run TestVerifyWorkflow .` and `go test -count=1 -run
TestEveryDeferredEntryHasALiveOwner .`: both pass (17/17 and 1/1
respectively; transcripts in Section 3).

## 6. D-038 -- the three Story 1.6 review layers, run and triaged

D-038 recorded that Story 1.6's three step-04 review layers were launched together and all three died
on a rate limit before returning findings, leaving the most safety-critical code in the repository
with mutation evidence and self-review only. Their exact prompts were preserved in
`review-layer-prompts-1-6.md`. All three were run here, from those prompts, over the same diff the
entry names -- `git diff 2720bfbf30de9cb018713e2107bd0033bf9e3901..dc883b1 -- internal/`, regenerated
and handed to each layer as a file -- and on a different model than the one that wrote the code,
which is what the entry asked for. Each layer ran context-free and read-only.

The layers read a diff that is several stories old. A finding they could not see closed is not a
finding today, so every one was re-verified against HEAD by the orchestrator before disposition, and
the disposition column below says which. Nothing was accepted on a layer's say-so.

### Layer 2 — Edge Case Hunter (ran 2026-09-08, default model; returned 20 findings)

The layer read the historical Story 1.6 diff only, as its prompt requires. Every finding was
re-verified by the orchestrator against current HEAD (`4058115`) before disposition: the diff is
several stories old, so a finding it could not see closed is not a finding today.

| # | Finding (abbrev.) | HEAD evidence | Disposition |
|---|---|---|---|
| 1 | `joinDrainer` waits unboundedly if `ServerPort.Stop` errors and leaves the lane open | `coordinator.go:569` comment states the wait is deliberately unbounded because `Stop` is quiescent on every return; `ports.go:142` makes that a port postcondition, and a cleanup diagnostic is "never a transfer of ownership back to the caller" | **re-homed → 3-4**: the bound is the story's charter ("every wait ends on a documented bound with a coded failure"), and an adapter that violates its postcondition is exactly the case 3.4 must decide |
| 2 | No `default:` in the `releaseAcquired` resource switch | `coordinator.go` switch is exhaustive over the two `resource` constants | **rejected**: a third constant added without a case is a compile-time-visible omission in one 12-line function, not a runtime path |
| 3 | A non-owner could publish while another goroutine holds the lease | No such caller exists; every publisher acquires or is refused | **rejected**: no reachable path was named, and the layer had no access to HEAD to find one |
| 4 | A re-entrant injected `AfterFunc` would deadlock `joinDrainer` | Only the test seam can do this; the production default is a real `time.AfterFunc` (`TestTheDefaultResetSchedulerIsARealTimer`) | **rejected**: test-seam-only |
| 5 | `Cancel` from IDLE returns while a reset is still in flight | `lifecycle.go:41` is precisely the proposed guard: `awaitLease` then `releaseLease` before returning | **already closed** since the diff |
| 6 | No budget on `awaitLease` when the lease holder is stuck in an adapter | Unbounded by design; same charter as #1 | **re-homed → 3-4** |
| 7 | A `Cancel` racing `Shutdown` publishes a reset after `closing` was raised | `TestShutdownContendsWithEveryOtherActor` (`coordinator_lifecycle_test.go:648`) permits it: it asserts at most one reset, not zero | **rejected with reason**: `Cancel` returns `ErrShuttingDown` when it observes `closing` first, so a `Cancel` that published linearized *before* `Shutdown`, and a won `Cancel` owes the UI its reset. The test's invariant is the right one |
| 8 | A `Stage` arriving between `fireReset`'s publish and its `releaseLease` is refused `busy` | Real window; `coordinator.go` Stage refuses with "the previous transfer is still being released" | **accepted**: the refusal is coded, documented in Stage, and recoverable by retrying. Recorded rather than fixed |
| 9 | `fireReset` needs a re-check after `joinDrainer` | `lifecycle.go:146` re-checks `c.closing`; nothing else can change under a held lease | **already closed** |
| 10 | IDLE announced before `live.stop()` | `lifecycle.go` calls `live.stop()` before `c.publish(reset)` in both `retire` and `fireReset` | **already closed** |
| 11 | No handling for an unrecognized `ServerEvent.Kind` | `outcomes.go:33` has the `default:` arm with a diagnostic | **already closed** |
| 12 | A `Complete` arriving while `AuthorizeClaim` holds the lease is lost forever | `server/handler.go:104` calls `AuthorizeClaim` *before* streaming a byte, so no outcome can exist while the claim holds the lease | **rejected**: not reachable |
| 13 | The lane closing while the session is STAGED synthesises nothing | `drainerMayActLocked` requires `stateTransferring`, so the post-loop synthesis is refused. `TestStagedServerEventsAreDrainedWhileStaged` (`coordinator_stage_test.go:612`) closes the lane at STAGED and asserts only that the drainer exits | **re-homed → 3-6**: a server that dies while staged leaves the QR on screen with no terminal event, which is 3.6's requirement verbatim |
| 14 | `ServerComplete` with a nil `Progress` publishes a zero snapshot | `terminalSnapshot` returns `(zero, false)` and the Complete arm documents the unknown-total zero rather than downgrading a success | **already closed** |
| 15 | Any failure code can reach the UI as the transfer error | `terminalFailureCodes` is exactly the proposed allow-list, pinned by `TestATerminalFailureOnlyPublishesCodesThatDescribeIt` | **already closed** |
| 16 | Negative `BytesSent`/`TotalBytes` reach the UI | `sanitizeProgress` clamps both to zero | **already closed** |
| 17 | A nil `StopTimer` from the seam panics the drainer | `armReset` has the `stop == nil` guard with a diagnostic | **already closed** |
| 18 | `fakeTimer.fireIfArmed` fires only the most recently armed call | True by construction and documented at `helpers_test.go:765`; a duplicate arming is caught instead by `armed()` counts in the reset tests | **accepted**: recorded, no change |
| 19 | The contention tests discard `h.server.publish`'s boolean | Confirmed at `coordinator_lifecycle_test.go:587,588,665,666`; `:961` does check it | **fixed in this story** as part of the D-045 work: an iteration that delivered no outcome must not pass silently |
| 20 | (deletion) A DONE session whose reset never fires is never joined | `retire` → `unwind` joins it on any later `Cancel` or `Shutdown`, and `fireReset` joins it on the timer path | **rejected**: no path leaves it unjoined |

**Net:** 9 already closed by later stories, 5 rejected with reasons, 3 re-homed to the Epic 3 stories
that own them (2 → 3.4, 1 → 3.6), 2 accepted and recorded, 1 fixed here.

### Layer 3 — Verification Gap Reviewer (ran 2026-09-08, Sonnet after the first attempt hit the limit)

Result: **no verification gaps found**, with the trace recorded. The layer screened the diff's
introduced behaviour — the two terminal states, the three-second reset lease, `Cancel`, `Shutdown`,
`retire`, `fireReset`, the variadic `revalidateLocked`, the drainer split into `forwardProgress` and
`acceptTerminal`, the lease-held publish, and progress sanitisation — and traced each to the tests
that exist at HEAD rather than to the tests inside the diff.

Orchestrator verification of the report, because a clean report is a claim like any other: all six
tests it named exist and assert behaviour, not shape. `TestUnusableServerEventsAreDiscardedWithADiagnostic`
(`coordinator_outcomes_test.go:758`) drives both an unrecognized kind and a snapshot-less progress
event, then proves the drainer survived by forwarding a well-formed one and counts the diagnostic.
`TestASchedulerThatWithholdsItsStopFunctionDoesNotKillTheDrainer` (`coordinator_lifecycle_test.go:1073`)
covers the nil-stop seam. The layer's clean result and the Edge Case layer's finding 13 do not
conflict: 13 is a missing requirement (a lane that closes while STAGED synthesises nothing), and a
verification-gap pass looks for changed behaviour that regresses unseen, not for requirements no one
wrote down. That difference is the reason D-038 wanted all three layers rather than mutation alone.

### Layer 1 — Blind Hunter (ran 2026-09-08, Sonnet after two attempts hit the limit; 12 findings)

| # | Finding (abbrev.) | HEAD evidence | Disposition |
|---|---|---|---|
| 1 | `cancelSession` duplicates `Cancel`'s marking sequence and is dead in production | It is declared in `helpers_test.go:828`, a method on the production type written in the test file, so it ships in no binary. Its seven callers are all tests that need to mark a session cancelled *without* taking the operation lease | **rejected**: not production code, and the bypass is the point |
| 2 | No test issues a command during the DONE/ERROR terminal window | `TestStageIsRefusedDuringTheTerminalLease` (`coordinator_stage_test.go:968`) covers both states, and its comment records the exact regression feared: widening the busy guard left the suite green while a replacement session displaced the one the UI was displaying | **already closed** |
| 3 | `acceptTerminal` discards the original failure cause | Confirmed: `outcomes.go`'s three `recordDiagnostic` calls cover an unrecognized kind, a snapshot-less progress event and a nil stop function. A `ServerFailed` event's `Err` is rewritten to fixed public copy by `terminalPublicError` and never recorded, so a real transfer failure leaves no internal trail | **re-homed → 3-6**: a production change, outside this story's Ask First boundary, and squarely 3.6's "visible" charter |
| 4 | The cancellation-copy assertion is weak; it should assert the exact message | The same assertion is what D-045 asked to remove, because `PublicErrorOf` derives the message from the code, making it unable to fail independently of the code check beside it | **rejected as superseded**: `TestATerminalFailureOnlyPublishesCodesThatDescribeIt` pins the whole code-to-copy table, which is where an exact-message assertion belongs |
| 5 | The `+Inf` speed clamp is never asserted by value | Confirmed live: the test emitted `math.Inf(1)` and checked only that event's `Percent` | **fixed**: the clamp is now asserted against `math.MaxFloat64`. Mutation M6 (speed left unclamped) fails it |
| 6 | `publish`'s lease-panic guard reads `leaseHeld()` without the mutex | True as described | **accepted**: it is an assertion against a programming error, not a synchronisation primitive, and taking the mutex inside `publish` would invert the lock order the design forbids |
| 7 | `session.stop`'s "caller must not hold `c.mu`" is comment-only | True | **accepted**: recorded beside #6 as the same class |
| 8 | `TestLaneClosureDuringATeardownIsSilent` skips `assertEventGrammar` | Was true; the D-045 work in this story added it | **fixed in this story** |
| 9 | No test drives `ServerComplete` with a nil `Progress` | Confirmed: `completeEvent` always builds a snapshot, so the documented "a port defect must not downgrade a success" arm was never executed | **fixed**: `TestACompleteCarryingNoSnapshotStillSucceeds`. Mutation M7 (report the missing snapshot as authoritative) fails it |
| 10 | The contention tests never confirm both race outcomes occur | Confirmed, and worse: both discarded `publish`'s boolean, so an iteration where the outcome never reached the lane was indistinguishable from one where a teardown beat it. This is the Edge Case layer's finding 19 from the other direction | **fixed**: both tests now count deliveries and log the split, and fail if no iteration ever landed the outcome. Observed 26/30 and 27/30 deliveries, with Cancel linearizing first in 18 of 30. Mutation M8 (the fake never accepts an event) fails both |
| 11 | `retire`'s `stateStaging` guard looks unreachable and untested | The guard is gone at HEAD, and the comment in its place says an unreachable guard reads to the next person like a live one — the same conclusion, reached and acted on already | **already closed** |
| 12 | The `AfterFunc` ordering contract is unverified | Seam-only, as with the Edge Case layer's finding 4 | **accepted** |

**Net:** 3 already closed, 4 fixed (3 here, 1 by this story's D-045 work), 4 rejected or accepted with
reasons, 1 re-homed to 3.6.

### What D-038 bought

Thirty-two findings across three layers. Nineteen were already closed by stories written after the
diff -- which is itself most of the answer to whether running a stale review pays: usually no, and
the exceptions are what justify it. Nine were rejected or accepted with reasons. Three were re-homed
as D-090, D-091 and D-092. Four were live gaps: three fixed in this story's commits, and one already
closed by its own D-045 work.

The finding mutation could not have produced is the one the entry predicted. D-091 -- an event lane
closing while the session is staged synthesises nothing -- is a requirement nobody wrote down, and it
sits under a passing test that closes the lane and asks only whether the drainer exits. No mutation
of existing code surfaces it, because the code that would fail does not exist. That is the Blind
Hunter layer's whole purpose, and the argument for not letting a rate limit quietly retire a review
layer again.

### Orchestrator mutation table -- the D-045 and D-072 work

Run by the orchestrator against the implementer's changes before accepting them, per the project's
standing rule that a subagent's report is a claim until a mutation kills the test it names. Baseline
`4058115`; gate green before and after each mutation was reverted.

| # | Mutation | Expected | Result |
|---|---|---|---|
| M1 | Carry `seq` across sessions: seed `live.seq` from a package-level counter at session creation and write it back after the started event, simulating a coordinator-wide sequence | `TestASecondSessionsSequenceRestartsAtOneUnderItsOwnId` fails | **killed** — "the second session's started event has seq 2, want 1 -- seq is scoped per session", plus two `assertEventGrammar` gap errors |
| M2 | `drainerMayActLocked` returns `true`, bypassing `revalidateLocked` entirely | The strengthened pre-claim assertion fails | **killed** — `TestProgressIsRefusedOutsideAMatchingTransfer/before_the_transfer_is_claimed` now reports *both* leaked snapshots at the new assertion (line 103) before the pre-existing shared check reports only the second (line 145). This is exactly the strengthening D-045 asked for: the old assertion could only ever see the second snapshot |
| M3 | `unwind` calls `stopServer()` a second time | `TestLaneClosureDuringATeardownIsSilent`'s new count assertion fails | **killed** — "server.Stop ran 2 times, want exactly one" |
| M4 | `acceptTerminal` spins waiting for the operation lease instead of yielding it | Test fails | **survived** — the lane-closure test still passes, because by the time the drainer wins the lease the session is cleared and `revalidateLocked` refuses it |
| M2+M4 | Both bypasses together | — | **killed by hang**: the run deadlocks and dies on the test timeout. This is the deadlock `outcomes.go`'s header comment describes — a teardown waiting on the drainer while the drainer waits on the lease — and it is the honest answer to what pins that test: not one guard but two mutually redundant ones, the lease and the revalidate. Neither alone is load-bearing for this scenario, which is worth knowing before someone "simplifies" either |
| M5 | Add a required `retryLast` member to `TransferController` and return it from `useTransfer` | `tsc --noEmit` fails where the controller stub is built | **killed** — `TS2741: Property 'retryLast' is missing ... but required in type 'ControllerCommands'` at `App.test.tsx:183` and `App.focus.test.tsx:54`. Before the shared harness both stubs were object literals handed to a `vi.fn()`'s `mockReturnValue`, which is untyped: the same change produced no error at all, which is the D-072 drift |

One wording correction the orchestrator made rather than leaving overstated: `App.harness.tsx`'s
header said a new controller member becomes "a type error in this one place". It is a type error in
both suites' `commands` objects. The real improvement is silent-to-compile-error, not
two-places-to-one, and the comment now says that.

## 7. The first CI run, and what it found

Run 1 -- https://github.com/jaeson-sandbox/FairDrop/actions/runs/34310588517 -- commit `1eeeec4`.

| Job | Conclusion | Link |
|---|---|---|
| verify (windows-latest) | success | https://github.com/jaeson-sandbox/FairDrop/actions/runs/34310588517/job/102336256196 |
| verify (macos-latest) | **failure**, at `wails build` | https://github.com/jaeson-sandbox/FairDrop/actions/runs/34310588517/job/102336255937 |

```
internal/source/handle_posix.go:129:30: cannot use descriptor (variable of type int) as uintptr value in argument to unix.FcntlInt
internal/source/handle_posix.go:136:25: cannot use descriptor (variable of type int) as uintptr value in argument to unix.FcntlInt
```

`handle_posix.go` is `//go:build linux || darwin`. It has never been compiled by anything in this
project: every gate to date ran on this one Windows machine, where the file is excluded before the
type-checker ever sees it. `unix.FcntlInt` takes a `uintptr` file descriptor; `clearPosixNonBlocking`
was handing it the `int` that every neighbouring call in the file uses. The code has been in the tree
since Epic 1, was reviewed, and shipped inside two "green" epics.

This is the epic's own premise, demonstrated on the first run rather than argued: *"it cross-compiles"
never stands in for "it was verified"* -- and here it did not even cross-compile, because nobody had
ever asked it to. D-005 said a clean-clone CI job would have caught the `go:embed` gap on the very
first push; the same is now true of this.

Fixed by converting at the two call sites, leaving the parameter an `int` to match `Open`, `Fstat` and
`Close` beside it, exactly as `os.NewFile(uintptr(descriptor), name)` already does four lines above.
Verified locally with `GOOS=darwin GOARCH=arm64`, `GOOS=darwin GOARCH=amd64` and
`GOOS=linux GOARCH=amd64` builds plus a darwin `go vet`, all clean, and then on the real runner.

Run 2 -- https://github.com/jaeson-sandbox/FairDrop/actions/runs/34311027850 -- commit `a76d4ce`.
Windows green; macOS got past the build and failed one step later, on the bindings-drift check:

```
wails build changed frontend/wailsjs -- the committed bindings are stale:
 frontend/wailsjs/go/main/App.d.ts  old mode 100644  new mode 100755
 frontend/wailsjs/go/main/App.js    old mode 100644  new mode 100755
 frontend/wailsjs/go/models.ts      old mode 100644  new mode 100755
```

Not drift: the darwin binding generator writes the files executable, and they are committed `0644`
from Windows, where git does not track the bit at all. The check now runs under
`git -c core.fileMode=false`, because what it exists to catch is stale generated *content*, not a
permission bit that differs by the platform that ran the generator. The pin moved with it, and two
mutations confirm it: deleting the guard while leaving the reporting `git diff` in place, and dropping
`core.fileMode=false` to reintroduce the macOS failure, both fail the test.

Also pre-flighted before this push, to spend one round trip instead of three:
`GOOS=darwin GOARCH=arm64 staticcheck ./...` and the linux equivalent, both clean. Note the bare
binary -- `go tool staticcheck` under a foreign `GOOS` tries to build the tool itself for that OS and
dies before analysing anything, which looks like a broken toolchain rather than a misuse.

Two process notes, both recorded in `AGENTS.md`:

- **`gh run watch --exit-status` exited 0 on this failed run**, after printing the compile errors it
  had just watched fail. The exit code is not the verdict; `gh run view <id> --json conclusion,jobs`
  is. Had the exit code been trusted, this story would have been closed with a red macOS job and the
  defect still in the tree -- a green signal over a failed run, which is the exact failure shape this
  project keeps finding in its own tests.
- **A local cross-platform type-check is now a pre-flight step.** It is not release proof and no
  workflow step does it; the macOS job remains the proof. It costs seconds and would have caught this
  before the push.

## 8. Left incomplete / risks

- **No CI run yet.** This session did not commit or push, per its explicit
  instructions, so `.github/workflows/verify.yml` has never executed on
  GitHub Actions. The spec's acceptance criterion "each is green on its
  native runner ... and the evidence file links both runs" is not yet
  satisfiable from this session; the run URLs must be added here after the
  branch is pushed and both matrix jobs finish. Everything the workflow does
  has been exercised locally (Section 2/5), including the exact cgo probe and
  wails-version assertion commands, but a local Windows run cannot stand in
  for the macOS job.
- **D-038** is untouched by design (orchestrator's task); its triage is
  appended to this file separately, not written here.
- **`verify_workflow_test.go`'s text-matching approach** is inherently
  order-and-substring based rather than a real YAML/step-graph model. Section
  3 shows four mutations that initially slipped past a naive substring check
  because the same words appeared in a comment or failure-message string
  elsewhere in the file; all four were hardened and re-verified, but the
  general risk -- a future edit adding descriptive prose that happens to
  contain a pinned substring -- remains a class of false positive to watch
  for, not a class of false negative (a spurious failure is loud and wrong,
  not silent).
- **The one-time worktree rewrite** was a byte-level `\r`-strip over 594
  files, verified on a sample rather than exhaustively hashed against the
  index; `git status`/`git diff` being empty for all of them after
  `--renormalize` is the exhaustive confirmation (a hash mismatch on any file
  would have left it listed as modified), but no per-file hash comparison
  loop was run and logged.
