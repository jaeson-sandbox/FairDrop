<!-- bmad:context -->
<!-- Verified 2026-09-01 against bb4ef9e. Managed by bmad-project-context; edits
     inside this block are replaced on refresh. Keep preserved guidance outside
     the markers. -->

## FairDrop

FairDrop is an ephemeral LAN peer-to-peer file-transfer desktop app: drop one file
or folder, let one receiver pull it over HTTP, and persist nothing. It uses Go,
Wails v2, React 19, TypeScript, and Tailwind v4. The canonical product contract is
`_bmad-output/specs/spec-fairdrop/SPEC.md` with `docs/fairdrop-architecture.md` and
`docs/fairdrop-contracts.md`; the UX `DESIGN.md` and `EXPERIENCE.md` control visual
presentation, public copy, focus, and announcements. Treat `docs/fairdrop-spec.md`
as historical narrative and apply all corrections and supersessions before using it.

## Where things are

- `main.go` composes adapters and Wails options; `app.go` is the bound command and
  event boundary.
- `internal/transfer` owns the domain, coordinator, and consumer-owned `SourcePort`,
  `NetworkPort`, `QRPort`, and `ServerPort`.
- `internal/server` owns the consumed `PayloadPort` and `PreparedPayload`;
  `internal/{source,network,qr,stream,server}` provide concrete adapters.
- Extend these existing ports for Epic 2. Never resurrect the retired
  `NetworkManager`, `Streamer`, `TransferServer`, or `TransferStats` contracts.
- `frontend/src/App.tsx` owns drop, focus, and announcement routing;
  `frontend/src/transfer` owns validation and session state; `frontend/src/ui`
  owns the Paper Relay views and accessibility behavior.
- Current story specs, sprint state, retrospectives, and routed findings live in
  `_bmad-output/implementation-artifacts/`.

## Running and verifying

- `.github/workflows/verify.yml` is the canonical gate, run natively on
  `windows-latest` and `macos-latest` for every pull request and every push to
  `main`/`epic-*`. It runs, in order: `wails build`, a bindings-drift and
  `.gitkeep` check, `gofmt -l .`, `go vet ./...`, `go tool staticcheck ./...`,
  `go test -count=1 ./...`, an explicit cgo check, `go test -count=1 -race
  ./...`, the frontend suite, and the line-ending check.
  `verify_workflow_test.go` pins every one of those lines and fails, naming
  the break, if a pin is loosened. Run the same commands locally, in the same
  order, before pushing.
- After changing the exported `App` command surface, run `wails build` before
  standalone frontend checks so `frontend/wailsjs/` is regenerated. Never edit
  generated bindings by hand.
- Run `frontend/npm test` separately: `wails build` compiles the frontend but does
  not run Vitest. CI now runs it too, via the workflow above.
- `go tool staticcheck ./...` is the fixed linter, run from the `go.mod` tool
  directive (`go get -tool honnef.co/go/tools/cmd/staticcheck@v0.8.1`) rather
  than a separately installed binary, so it resolves reproducibly from
  `go.sum` instead of whatever `@latest` picks on a given day. Its ignore
  directive is `//lint:ignore SA1012 <reason>` on the line **above** the
  finding -- `//nolint:staticcheck` is golangci-lint syntax and staticcheck
  does not read it, so a `//nolint:staticcheck` comment left in place is
  decoration, not a suppression.
- BMAD Python scripts need `PYTHONIOENCODING=utf-8` outside the configured Claude
  environment because the Windows default encoding is cp1252.
- Before pushing, type-check the platform files this machine never compiles:
  `GOOS=darwin GOARCH=arm64 go build ./...` and `GOOS=linux GOARCH=amd64 go build ./...`,
  plus `GOOS=darwin GOARCH=arm64 staticcheck ./...` -- note the bare binary, because
  `go tool staticcheck` under a foreign `GOOS` tries to build the *tool* for that OS
  and fails before analysing anything.
  This is a pre-flight, not release proof -- the macOS job is the proof, and the
  workflow deliberately contains no cross-build. It exists because the first CI
  run found two darwin-only compile errors in `handle_posix.go` that had sat in
  the tree since Epic 1: `unix.FcntlInt` takes a `uintptr` and was being handed
  an `int`, which no Windows build could ever see. Seconds locally, a full CI
  round trip otherwise.
- **Never use `t.TempDir()` directly in `internal/source` or `internal/stream`.** Use each
  package's `fixtureDir(t)` helper. macOS puts the per-user temp tree under `/var`, which is a
  symlink, and those packages refuse a link-like component anywhere in a selection by design --
  so a raw `t.TempDir()` hands macOS a path the code is *required* to refuse, and roughly forty
  tests fail for a reason unrelated to what they check.
- A guard that delegates to a platform API can be a silent no-op elsewhere. `filepath.VolumeName`
  does nothing on POSIX, so an unsafe archive entry name was refused from a Windows sender and
  accepted from a macOS one, while the risk was receiver-side. Prefer a check whose behaviour does
  not depend on the host when the consequence does not either.
- `gh run watch --exit-status` exited **0** on a run whose macOS job failed, after
  printing the failure. Never take a watch's exit code as the verdict: read the
  conclusion with `gh run view <id> --json conclusion,jobs`.

## Non-default conventions

- Tailwind is v4: use the Vite plugin and `@import "tailwindcss";`; do not add v3
  Tailwind or PostCSS configuration.
- Native drop targets inherit `--wails-drop-target: drop`; do not replace this
  with a DOM drop handler.
- `main_test.go` pins the `wails.Run` options contract. Keep
  `BackgroundColour` synchronized with `--color-canvas` to prevent launch flash.
- Clipboard writes go through bound `CopyToClipboard`, never
  `navigator.clipboard`.
- Every deferred-work entry needs a live story owner or
  `discharged`/`accepted`, and carries a stable `id:` (D-NNN) that story acceptance criteria
  cite. Evidence (mutation tables, triage, gate transcripts) lives in `evidence-<slug>.md`
  beside each spec, never in the spec itself; see `_bmad/custom/bmad-build.toml`.
- Adding a deferred entry is two edits, not one: the entry in `deferred-work.md`,
  and its `D-NNN` id on the owning story's `**Closes:**` line in `epics.md`. A build
  session reads the story's acceptance criteria, so an uncited entry is invisible to
  the only session that would resolve it. `TestEveryOpenDeferredEntryIsCitedByItsOwningStory`
  fails on both that and an entry left open under a story already `done`.

## Known pitfalls

- Keep `frontend/dist/.gitkeep`, its `.gitignore` exception, and the npm
  `postbuild` restoration so clean-clone embedding continues to work.
- macOS Wails uses a non-secure custom webview scheme. Browser APIs gated on a
  secure context may be absent there even when they work on Windows; route such
  capabilities through Go.
- Normalize line endings before source-text or mutation matching. This Windows
  worktree uses `core.autocrlf`, and silent non-matches can produce false-green
  verification.
- Preserve complete failing-test output. Truncated logs destroyed the only
  evidence for an unreproduced Epic 1 failure.
- Browser-unit tests do not prove native interaction. Keep a real built-app,
  nearby-device QR/download smoke test in each epic until Story 3.2 automates it.

<!-- /bmad:context -->

## Environment and verification pitfalls

<!-- Outside the bmad:context block on purpose: the 2026-09-01 context refresh silently
     dropped the first three of these from inside the managed block. Keep them here. -->

- Neither Go nor the Wails CLI is on the default PATH in this environment. Prepend
  `/c/Program Files/Go/bin` and `~/go/bin`, or bare `go`/`wails` fail with command-not-found.
- **`go test -race` needs cgo, and without it the run fails with "requires cgo" rather than
  reporting clean.** Prepend `.../WinGet/Packages/BrechtSanders.WinLibs.POSIX.UCRT_*/mingw64/bin`
  and set `CGO_ENABLED=1`. Before trusting any race result, confirm `go env CGO_ENABLED` prints
  `1` and that the run lists `ok` per package -- an environment that silently cannot run the
  detector looks, in a truncated log, exactly like one that did.
- Run `wails build` before a standalone `npm run build` in a fresh tree: it regenerates
  `frontend/wailsjs/`, and the committed copy can lack exports the frontend imports.
- Never run `wails build` concurrently with the frontend test suite. Its `frontend:install`
  step is `npm install` and it rebuilds `frontend/dist`, both underneath a running Vitest;
  the result is hundreds of import failures that look like a broken product and are not.
  Run the Go gate, the frontend suite, and `wails build` one after another, never in parallel.
- In a bash double-quoted string a Windows path such as `"C:\Users\x\$name"` turns `\$` into
  a literal `$`. Quote Windows paths with single quotes; a "does not exist" result from a path
  built in double quotes is a quoting bug until proven otherwise.
- The HTTP server sets no `WriteTimeout` on purpose: one would cap every transfer at the
  deadline, and a folder ZIP over Wi-Fi outlives any sensible value. `ReadTimeout` does *not*
  cap a transfer -- net/http clears the read deadline before its background disconnect read, a
  fact that was read the other way once and disproved by mutation.
  `TestATransferLongerThanEveryTimeoutStillCompletes` pins both; read the comment above
  `readTimeout` in `internal/server/lifecycle.go` before touching the deadlines.
- A regenerated managed block or compiled epic context drops lines it does not consider
  load-bearing. After any `bmad-project-context` refresh or `compile-epic-context` run, diff the
  file against its previous commit and restore what was lost before continuing.
- Nothing in the running app logs a transfer's lifecycle beyond the stderr lines `app.go`
  writes. `internal/stream/diagnose_test.go` (env `FAIRDROP_DIAGNOSE`) runs the production
  Inspect/Walk/Prepare/WriteTo path over a real folder and names the first stage that refuses;
  start there when a live folder download fails.

## Git workflow

<!-- Outside the bmad:context block on purpose: kept across `bmad-project-context` refreshes. -->

- Branch per phase or epic (`phase-1-wails-scaffold`, `epic-1-share-one-file`). Never commit
  directly to `main`.
- Commit at every verified-green milestone — build, tests, and lint passing — rather than
  batching to the end of a task, and **push each commit to `origin`**. Work that is only
  local is invisible to the other agents on this project.
- A finished phase or epic merges into `main` with `--no-ff`, and `main` is pushed, so the
  next branch forks from a real baseline instead of stacking on its predecessor.
- Remote is `https://github.com/jaeson-sandbox/FairDrop.git`; `gh` is authenticated.

## Testing standards, learned the hard way

<!-- Outside the bmad:context block on purpose: kept across `bmad-project-context` refreshes. -->

Every item below is a real defect this repo shipped past a green suite. They are
ordered by how much they cost.

- **A test that compares a constant to itself pins nothing.** Story 1.5 had five.
  The worst: `testURL` was built from `downloadPathPrefix`, so the capability path
  could be changed to `/dl/` with the whole suite green while every QR code pointed
  at a route the server answers with 404 — the product broken end to end, CI happy.
  Assert against a **literal** written out at the assertion site, never against the
  symbol under test. The same applied to the beacon instance and the beacon warning.

- **Seam coverage is not production coverage.** When a test injects a seam, the
  default path is untested unless it is separately asserted. This bit three stories:
  the payload read cap (1.3), the QR encoder (1.5a), and the server's bind address
  (1.4) — where binding `127.0.0.1` instead of `0.0.0.0` passed everything, because
  every test replaced the listener with a loopback binder that discarded its argument.
  If a test overrides a seam, add one assertion that drives the real default too.

- **Mutation is the acceptance bar for any load-bearing claim.** Break the thing on
  purpose and confirm a test fails and *names* it. If nothing fails, the guarantee is
  decoration. This is how all of the above were found, and how each fix was confirmed.

- **Write the test that does not exist yet.** The largest single find in Story 1.5 —
  Stage committing a live session for a command the user had abandoned — came from
  writing a caller-context test, not from reading code. Ask "what does no test
  touch?" before asking "what looks wrong?".

- **A zero-value "skip this check" convention leaks.** `revalidateLocked(id,
  generation)` let `generation == 0` mean skip, and that silently extended to `id`,
  so `AuthorizeClaim(ctx, "")` authorized whichever session was staged. Give each
  parameter its own explicit refusal; never let "not supplied" mean "matches
  anything" on an identity.

- **`context.AfterFunc` is eventually consistent.** Its callback runs on a *new*
  goroutine, so a derived context still reads clean for a moment after the parent is
  cancelled. Where the answer must be immediate, check the caller's context directly.

- **A concurrency test that accepts either outcome cannot fail.** Tally the outcomes
  and log the distribution. But do not then *assert* both: FairDrop's claim race
  resolves one way ~49 times in 50, so requiring both would fail at random. Force
  each branch deterministically in its own test and let the concurrent one check
  invariants.

- **Turn an invariant that lives in a comment into one the compiler or a test
  enforces.** The coordinator's "never hold the state mutex across an adapter call"
  became real when every fake began asserting the lock is unheld on entry; a prose
  version would survive any refactor that moved a call inside the lock, and `-race`
  does not catch it, because a lock held across a slow call is not a data race. The
  same applies to `if c.session == live` replacing a comment that claimed ownership.

- **A green test can pin a behaviour that is dead on a platform you never ran.** Story 1.9
  shipped a Copy button that did nothing on macOS, guarded by a passing test named
  *"leaves the action label alone when no clipboard exists at all"* — the suite agreed with
  the bug. The three clipboard paths were all covered; what was untested was the assumption
  about which platform provides the API, and that was answerable by reading the vendored
  Wails source. When a check is deferred to a human because *you* are unsure how the
  platform behaves, that is not a manual-test item — it is an unresolved question, and the
  answer is usually in the dependency's source. Resolve it before writing the manual step.

- **When a story deletes a contract, grep the whole repo.** Documentation drift is
  invisible to an `internal/`-only check, and `docs/fairdrop-spec.md` kept publishing
  deleted interfaces twice because of it.
