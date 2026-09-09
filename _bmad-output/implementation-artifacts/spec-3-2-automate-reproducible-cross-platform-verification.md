---
title: 'Story 3.2: Automate Reproducible Cross-Platform Verification'
type: 'feature'
created: '2026-09-08'
status: 'in-progress'
review_loop_iteration: 0
baseline_commit: '405811574b26a6cd34988f789354859a06d8fac4'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-3-context.md'
  - '{project-root}/AGENTS.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Nothing runs the gate but a person at this one Windows machine, so the verified state decays from the next commit, the frontend suite runs only when someone types `npm test`, five `//nolint:staticcheck` directives are decoration no linter reads, 597 tracked files sit CRLF on disk, and three Epic 1 review layers never ran (D-005, D-009, D-016, D-038, D-045, D-050, D-054, D-063, D-072).

**Approach:** One GitHub Actions workflow runs the whole gate serially on native Windows and macOS runners from pinned toolchains, a Go test pins the workflow's load-bearing lines so it cannot drift, `.gitattributes` normalises every text file to LF with a CI check, staticcheck runs from a `go.mod` tool directive, and the recorded test-quality debts are fixed or accepted in writing.

## Boundaries & Constraints

**Always:** Runners are `windows-latest` and `macos-latest` only; triggers are `pull_request` plus `push` to `main` and `epic-*`; superseded runs are cancelled. Go `1.26.7` (≥ the `go.mod` floor), Node from `.nvmrc`, Wails CLI installed at exactly `v2.15.0` and asserted by `wails version`, `npm ci` with dev dependencies. Every job runs, one step at a time and in this order: `wails build` (which runs `npm ci` and regenerates bindings), bindings-drift and `.gitkeep` checks, `gofmt -l`, `go vet`, `go tool staticcheck`, `go test`, an explicit cgo check, `go test -race`, the frontend suite, the line-ending check. The race step's comment says why it is native and C-toolchain-dependent. `verify_workflow_test.go` fails when any of these pins is removed. Existing deferred entries keep their evidence text; only `owner` changes.

**Ask First:** A Linux job for any purpose. Raising the `go.mod` floor. golangci-lint instead of staticcheck. Excluding tests from `tsconfig.json`. Changing an error string a test or the public registry pins. Any production-code change beyond the staticcheck sites named in the Code Map.

**Never:** `-upx`, `--omit=dev`, `GOOS`/`GOARCH` cross-builds, or Vitest running while `wails build` runs. Hand-edit `frontend/wailsjs`. Claim a macOS result without the run URL. Publish release artifacts (Story 3.3) or execute the platform matrix (Story 3.7).

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Verified push | PR, or push to `main`/`epic-*` | Both native jobs green; an older run on the same ref is cancelled | N/A |
| Other branch push | push to `scratch/x` | No run | N/A |
| cgo missing | runner without a C compiler | cgo step fails naming the cause; race never reports clean | Step exit 1 |
| Unpinned CLI | `wails version` ≠ `v2.15.0` | Toolchain step fails before any build | Step exit 1 |
| Line endings | a committed or checked-out CRLF/mixed file | Check fails listing the paths | Step exit 1 |
| Stale bindings / lost `.gitkeep` | `wails build` changes `frontend/wailsjs` or drops `frontend/dist/.gitkeep` | Check fails | Step exit 1 |
| Platform-only file | a `_windows.go` or `_darwin.go` finding | Fails on the job whose OS compiles it | Step exit 1 |
| Workflow tampered | ubuntu runner, `-upx`, `--omit=dev`, unpinned CLI, reordered steps | `verify_workflow_test.go` fails locally and in CI | Test |

</frozen-after-approval>

## Code Map

- `.github/workflows/` -- absent; create `verify.yml`. `actions/checkout@v7`, `actions/setup-go@v7` (`go-version: '1.26.7'`), `actions/setup-node@v7` (`node-version-file: .nvmrc`, `cache: npm`, `cache-dependency-path: frontend/package-lock.json`), `actions/cache@v6` on `~/go/bin/wails*` keyed by CLI version, OS and arch. All steps `shell: bash`; add `$(go env GOPATH)/bin` to `$GITHUB_PATH` explicitly. Repo is public, `main` unprotected.
- `main_test.go:302` `TestEveryDeferredEntryHasALiveOwner` -- precedent for a root-package test that reads a repo file line by line; mirror it in `verify_workflow_test.go` (no YAML library; `gopkg.in/yaml.v3` is only an indirect dependency).
- `wails.json:5` `"frontend:install": "npm ci"` -- pin it in the same test; `npm run build` needs Vite and tsc, so a production install can never build (D-009 is accepted on that ground, and enforced).
- `.gitattributes` -- pins `*.go *.css *.ts *.tsx *.js go.mod go.sum` only. Index is entirely `i/lf`; 596 files plus `frontend/vite.config.ts` are `w/crlf`. Add `* text=auto eol=lf` first and explicit `binary` for `*.png *.ico *.icns *.woff2 *.pyc`, then rewrite the worktree once (`git ls-files --eol -z`, convert `w/crlf` entries to LF). Check: `git ls-files --eol | grep -E '(i|w)/(crlf|mixed)'` must be empty.
- `go.mod` -- `go get -tool honnef.co/go/tools/cmd/staticcheck@v0.8.1` adds a `tool` directive (staticcheck 2026.2.1, verified to run here); command is `go tool staticcheck ./...`. Fifteen findings today: SA1012 nil context ×7 (`qr_test.go:141`, `walk_test.go:186`, `archive_test.go:386`, `payload_test.go:481,1149` carry `//nolint:staticcheck`, which staticcheck does not honour -- its directive is `//lint:ignore SA1012 <reason>` on the preceding line; `address_test.go:304`, `beacon_test.go:340` are bare); ST1005 capitalised error strings at `source/handle_windows.go:269`, `source/platform_windows.go:14`, `stream/platform_windows.go:21`; ST1018 at `payload_test.go:605-606` (write `‮`, `​`); S1016 at `transfer/coordinator.go:785` (`Warning(public)`); SA4023 at `server/lifecycle_test.go:550` -- `port == nil` can never be true through the interface, so the test proves nothing: compare the concrete pointer and add `var _ transfer.ServerPort = (*Server)(nil)`.
- `internal/transfer/coordinator_outcomes_test.go` (D-045) -- `:302` message assertion is redundant with `:688` `TestATerminalFailureOnlyPublishesCodesThatDescribeIt`: delete it, say why. `:396` `TestLaneClosureDuringATeardownIsSilent`: pin the exact post-cancel state the cancel tests pin (`coordinator_lifecycle_test.go:1006,1096,1128` use `stateIdle`), the single `Stop` call, and drop `_ = metadata`. `:85` STAGED case emits two snapshots and only the second is checked: assert no progress event at all. Add a second-session test: complete, `h.timer.fire()`, stage and transfer again, then `assertEventGrammar` (`:502`) filtered per session so `seq` provably restarts at 1 under a new id.
- `frontend/src/App.test.tsx:174-186,339-356` and `App.focus.test.tsx:47-75` (D-072) -- two controller stubs with different signatures. One shared `frontend/src/App.harness.tsx` (not matched by the test glob) typed as `ReturnType<typeof useTransfer>` so a new controller method is a type error in one place; mocks stay in each test file and are passed in. `App.test.tsx:774` describe claims effect replay but only `:149`-style first-mount tests exercise it: keep the replay claim on the tests that replay, rename or relocate the rest. `App.tsx:162` `data-transfer-phase` mirrors the reducer phase; `data-phase-view` (`IdleView.tsx:47`, `OutcomePanel.tsx:64`, `StagedView.tsx:68`, `StagePendingCard.tsx:30`, `TransferringView.tsx:30`) names the rendered body; `style.css` uses neither. Document that in `App.tsx` and pin the one legitimate disagreement (terminal `error` carrying `cancelled` renders the Idle body).
- `_bmad-output/implementation-artifacts/review-layer-prompts-1-6.md` (D-038) -- three verbatim prompts over `git diff 2720bfbf30de9cb018713e2107bd0033bf9e3901..dc883b1 -- internal/`. Run each as a context-free subagent; re-verify every finding against HEAD before it counts.
- `AGENTS.md:32-40` "Running and verifying" and `README.md:32-50` -- name the workflow as the canonical sequence, add the staticcheck command, and record that `//nolint` is golangci-lint syntax.
- `_bmad-output/implementation-artifacts/deferred-work.md` -- nine owners to `discharged` or `accepted`, one banner.

## Tasks & Acceptance

**Execution:**
- [x] `.gitattributes` + worktree rewrite -- normalise, then confirm `gofmt -l .`, Vitest and `git status` are unchanged.
- [x] `go.mod`/`go.sum` -- staticcheck tool directive; fix the fifteen findings as mapped; five directives become `//lint:ignore`.
- [x] `internal/transfer/coordinator_outcomes_test.go` -- the four D-045 items.
- [x] `frontend/src/App.harness.tsx`, `App.test.tsx`, `App.focus.test.tsx`, `App.tsx` -- the three D-072 items.
- [x] `.github/workflows/verify.yml` -- the workflow, as bounded above.
- [x] `verify_workflow_test.go` -- pins for every Always clause and `wails.json`.
- [x] D-038 -- the orchestrator runs the three layers in parallel with this implementation and writes their triage into the evidence file itself; the implementer leaves this task, the prompts file, and that evidence section untouched.
- [x] `AGENTS.md`, `README.md`, `deferred-work.md` -- as mapped; `evidence-3-2-automate-reproducible-cross-platform-verification.md` with the mutation table, the D-038 triage, and the run URLs.

**Acceptance Criteria:**
- Given the branch pushed, when both jobs finish, then each is green on its native runner with every step present in order, and the evidence file links both runs.
- Given a runner without cgo, when the cgo step runs, then the job fails there and the race step never runs.
- Given each pinned workflow line deliberately broken, when `go test ./...` runs, then `verify_workflow_test.go` names the broken pin.
- Given the story closes, when `deferred-work.md` is read, then all nine ids have a closed owner and the evidence file cites each with its disposition.

## Evidence

Mutation tables, gate transcripts, the D-038 review triage and the CI run URLs live in
[evidence-3-2-automate-reproducible-cross-platform-verification.md](evidence-3-2-automate-reproducible-cross-platform-verification.md), created with the implementation.

## Spec Change Log

- 2026-09-09: The workflow's first run failed on macOS, and what it found is the story's own
  justification: `internal/source/handle_posix.go` has never compiled. `unix.FcntlInt` takes a
  `uintptr` and was handed an `int` at two call sites, in a file built only on darwin and linux,
  so no Windows build since Epic 1 could see it. Fixed here as a two-call-site conversion. This is
  a production change outside the Code Map, which Boundaries makes Ask First, and it was made
  without asking for one reason: the story's first acceptance criterion is that both jobs finish
  green, so the story cannot close around it. The frozen Never clause forbidding `GOOS`/`GOARCH`
  cross-builds is read as governing the workflow, whose whole point is that a cross-compiled result
  is not release proof; a local pre-flight type-check that makes no release claim is not the same
  act, and `AGENTS.md` now asks for one before pushing, labelled as not being proof. No workflow
  step cross-builds, and `verify_workflow_test.go` still fails if one appears.

## Design Notes

The workflow is the single source of the gate; the Go test is what makes it load-bearing, in the same way `main_test.go` pins the Wails options. Staticcheck as a `go.mod` tool is reproducible from `go.sum` rather than from whatever `@latest` resolves to on a given day. Windows-only and darwin-only files are analysed only on the job whose OS compiles them, which is one concrete reason the jobs are native. The line-ending fix rewrites disk copies only: the index is already LF, so the commit carries `.gitattributes` and nothing else.

## Verification

**Commands:**
- `gofmt -l .`; `go vet ./...`; `go tool staticcheck ./...`; `go test -count=1 ./...`; `go test -count=1 -race ./...` -- clean
- `cd frontend && npx vitest run`; then, alone, `wails build` -- green
- `git ls-files --eol | grep -E '(i|w)/(crlf|mixed)'` -- empty
- `gh run watch` on the push; both jobs green -- URLs into the evidence file
