# Deferred Work

> **Resumption audit (2026-09-11):** two id-less legacy records are now D-108
> (lost log evidence, discharged by the existing direct CI test output) and D-109
> (missing Story 1.10 review layers, now owned and cited by 3.12). Their original
> observations remain below. The citation check now rejects missing/duplicate ids
> instead of skipping them. D-110 is a newly reproduced HTTP completion-ordering
> failure, routed to 3.8 and blocking 3.7 acceptance; see Story 3.7 evidence.

Real findings surfaced during review that are not the current story's problem.
Append-only. Each entry names the spec that surfaced it and the `owner:` that will
resolve it. An owner is a story key from `sprint-status.yaml`, or one of two
closed states: `discharged` (a later story resolved it -- the banners above say
which) or `accepted` (reviewed, and deliberately left as it is, with the reason
in the evidence). An entry closed either way keeps its original evidence text,
so read the owner before reading the evidence as open work.
`TestEveryDeferredEntryHasALiveOwner` in `main_test.go` fails if an entry has no
owner or names a story that does not exist, so a finding cannot quietly stop
being anyone's problem.

Naming a live story is necessary and was not sufficient. A build session reads
its story's acceptance criteria in `epics.md`, so an open entry whose id appears
in no `**Closes:**` line is invisible to the only session that would ever
resolve it -- five entries were in exactly that state when the check was
written, added here and never added there.
`TestEveryOpenDeferredEntryIsCitedByItsOwningStory` now fails on that, and on an
entry left open under a story already marked `done`. Adding an entry therefore
means two edits: the entry here, and its id on the owning story's `**Closes:**`
line.

> **Every entry now carries a stable `id:` (D-001 … D-085, file order).** Story acceptance criteria in
> `epics.md` cite these ids, so a build session knows exactly which entries its story must close.
> Epic 3 was re-planned on 2026-09-08 into nine stories; owners below were re-homed accordingly.
> D-083 (drive the server handler with a directory payload) is discharged by
> `TestATransferLongerThanEveryTimeoutStillCompletes`, which streams an unknown-length payload through
> a real listener and asserts the omitted `Content-Length` and the unknown-total terminal snapshot.

> **Discharged (Story 3.5):** all seven ids this story's Closes line names are closed --
> `D-012`, `D-015`, `D-025`, `D-029`, `D-044`, `D-047`, `D-053`. A new code, `setup_failed`
> ("Couldn't prepare that item" / "FairDrop couldn't prepare that item. Nothing was sent. Choose it
> again."), now covers every state where a coded failure fires before any byte is sent: a CSPRNG
> exhaustion during Stage (D-025, `internal/transfer/coordinator.go`'s `randomHex`), a deadline that
> expires anywhere inside `Prepare`/`prepareArchive` before headers are written (D-012,
> `internal/stream/payload.go`'s `prepareContextError`, split from the mid-stream `contextError`
> so the three post-header call sites inside `WriteTo` are unaffected), an uncoded `SourcePort`
> error now wrapped at the `Inspect` port call rather than trusted (D-015,
> `wrapUncodedSourceError`), a malformed Stage acknowledgement (D-053,
> `frontend/src/transfer/useTransfer.ts`), and both pre-startup/pre-composition refusals (D-047,
> `app.go`'s `errNotComposed` and `chooseWith`'s nil-context guard). D-029's `ready()` finding a
> nil port is now unreachable through normal construction -- `NewCoordinator` panics if any port or
> the observer is nil, since the one real caller (`main.go`'s `compose`) always supplies every one
> -- and the residual defensive path, reachable only by assembling a `*Coordinator` some other way,
> reports `setup_failed` rather than `transfer_failed` if it is ever hit. `busy` (D-044) keeps its
> code; its message is revised to "FairDrop is still finishing the last transfer. Wait a moment, or
> cancel it, then choose another item.", true both while a transfer is running and while its
> outcome is held on screen for the three-second terminal lease. `EXPERIENCE.md`,
> `docs/fairdrop-contracts.md`, `internal/transfer/errors.go` and
> `frontend/src/transfer/errors.ts` all carry the new code and both revised messages, and
> `main_test.go`'s `TestTheCrossLanguageErrorRegistryPinsEveryCodeAndMessage` now pins every code
> across all four places and every message across the three that carry one (Epic 1 retrospective
> item 6). One new finding fell out of the audit and is **not** fixed here, because fixing it would
> mean writing message text the reviewer has not confirmed: see `D-103` below.
> Mutation tables and gate transcripts live in
> `evidence-3-5-reconcile-public-error-copy-with-its-states.md`.

> **Discharged (Story 3.6):** all twelve ids this story's Closes line names are closed --
> `D-020`, `D-021`, `D-031`, `D-034`, `D-042`, `D-043`, `D-049`, `D-059`, `D-091`, `D-092`,
> `D-098`, `D-100`. Every diagnostic the coordinator or server records now leaves the process
> through an injected seam `compose` wires to the stderr line `app.go` already writes (D-098), so
> the "recorded as a diagnostic" the contract leans on is something a person can actually be asked
> for. Around it: a terminal failure's original cause is recorded before `terminalPublicError`
> rewrites it (D-092), a teardown that loses two resources reports both (D-100), a panicking
> observer is recovered and reported rather than unwinding past every lease release (D-043), a
> blocking one is bounded like any adapter call (D-034), a lane that closes at STAGED or CLAIMING
> synthesises a terminal outcome instead of leaving a QR code up for a listener that is gone
> (D-042, D-091), a repeated `Stop` replays the first call's diagnostic (D-020), `ErrorLog`
> forwards a fixed line for a genuine handler panic while still dropping every byte net/http
> wrote (D-021), the diagnostic sink marks its overflow rather than dropping silently (D-031),
> every `undelivered` drop is logged and not only counted (D-049), and a terminal outcome always
> carries a control so a lost `transfer-reset` cannot strand the window (D-059). A mutation sweep
> over all twelve found that four -- `D-031`, `D-042`/`D-091`, `D-092`, `D-100` -- were correctly
> implemented and defended by no test at all; each has one now. Epic 1 retrospective items 2, 3, 4
> and 7 are closed with them. Mutation tables and gate transcripts live in
> `evidence-3-6-make-lost-and-malformed-events-visible.md`.

> **Discharged (Story 3.4):** all eleven ids this story's Closes line names are closed --
> `D-017`, `D-019`, `D-022`, `D-024`, `D-027`, `D-030`, `D-032`, `D-036`, `D-037`, `D-087`, `D-090`.
> Every quiescence wait the coordinator or server performs while holding a lock or lease now ends
> on a documented, injected bound: the operation lease and the drainer join in
> `internal/transfer/coordinator.go` (a new `boundTimer` seam, distinct from `afterFunc`'s reset
> scheduler, drives both deterministically), the coordinator's own bounded calls into
> `ServerPort.Stop` and `NetworkPort.StopBeacon` (closing D-024, D-027, D-032, D-090), and
> `ServerPort.Stop`'s own wait for its listener, handlers and connections inside
> `internal/server/lifecycle.go` (a `serverTimeouts.teardown` seam, closing D-017 and, since a
> coordinator that never returns from `AuthorizeClaim` is just another stuck handler from the
> server's point of view, D-019 as well). A bound that elapses is reported as a coded
> `transfer_failed` naming what did not return, recorded as a diagnostic, and never reported as
> success; the coordinator still reaches `IDLE` and frees the lease regardless, because a stuck
> adapter must cost only its own resource's proven quiescence, never the coordinator's ability to
> serve the next command. `Server.Stop` also stopped holding `s.mu` across its wait, so a bounded
> teardown can never deadlock a later `Start`, and `Start` after `Stop` is now a specified,
> tested restart contract (D-022). `Coordinator.Cancel` and `Coordinator.Shutdown` take a
> `context.Context` and honour it while waiting to join a teardown some other operation already
> owns (D-036); `app.go`'s `CancelTransfer` hands it the stored application-lifetime context and
> `shutdown` hands it the Wails-supplied shutdown context, with a `shutdown begin` log line so a
> second launch swallowed while shutdown is blocked on a live transfer is diagnosable (D-087).
> `armReset`'s window between arming its timer and re-checking the session survived is forced
> deterministically through a test-only `afterArm` hook rather than left to scheduling (D-037).
> `NetworkPort`'s doc now states that `StartBeacon` requires a prior successful `GetLocalIP`, and
> the coordinator's `fakeNetwork` asserts the ordering rather than accepting the call at any time
> (D-030); `internal/network`'s own `TestStartBeaconRequiresSelectionAndLiveContext` already pinned
> the real adapter's refusal. `docs/fairdrop-contracts.md`, `docs/fairdrop-architecture.md`, and
> `AGENTS.md` carry the amended postconditions and the decision behind them. No new public error
> code was added: every new failure this story reports uses the existing `transfer_failed` carrier.
> Mutation tables and the bound-value reasoning live in
> `evidence-3-4-bound-every-lifecycle-wait-and-prove-quiescence.md`.

> **Discharged (Story 3.2, macOS):** `D-093` is closed. The POSIX adapter now
> builds, tests and race-tests green on `macos-latest`
> (https://github.com/jaeson-sandbox/FairDrop/actions/runs/34428026493). Three
> distinct causes hid behind one errno. `O_EVTONLY` is not darwin's `O_PATH` --
> it cannot open a directory that carries execute permission but not read
> permission, so `O_SEARCH` (which `x/sys/unix` does not export) is now the
> search flag, with an `EACCES` fallback on the metadata open. macOS puts the
> per-user temp tree under `/var`, a symlink, and this package refuses a
> link-like component by design, so fifty fixtures now resolve their path
> first. And `archiveEntryName` asked `filepath.VolumeName` whether an entry
> was volume qualified, which is a no-op off Windows, so the same unsafe name
> was refused from a Windows sender and accepted from a macOS one while the
> risk is receiver-side. Two new entries, `D-094` and `D-095`, record what this
> deliberately did **not** change.

> **Discharged (Story 3.2):** eight of the nine ids Epic 3's re-plan named for
> this story are closed here. `.github/workflows/verify.yml` runs the whole
> gate natively on `windows-latest` and `macos-latest` for every pull request
> and every push to `main`/`epic-*`, closing D-005; its "Frontend suite" step
> runs `npm test` after `wails build`'s full `npm ci`, closing D-050 and
> D-054. `go.mod`'s `tool honnef.co/go/tools/cmd/staticcheck` directive backs
> the fifteen findings the linter used to be decoration for -- five of them
> were `//nolint:staticcheck` comments, which is golangci-lint syntax that
> staticcheck never read -- closing D-016. `.gitattributes` now normalises
> every text file to LF (`* text=auto eol=lf`, explicit `binary` for image,
> font and `.pyc` extensions) and the worktree was rewritten once to match
> the already-LF index, closing D-063. The four D-045 test-quality findings
> and the three D-072 frontend-harness findings were fixed directly in their
> own files. D-009 is `accepted` rather than discharged: `npm run build`
> genuinely cannot run against a dev-omitting install, since `tsc` and Vite
> are both devDependencies, so the finding stays true by construction --
> `verify_workflow_test.go` and `wails.json`'s `"frontend:install": "npm ci"`
> pin the full install rather than "fixing" a tension that cannot be fixed
> away. D-038 is not among the eight above: its three review layers and
> their triage are recorded separately in this story's evidence file.

> **Discharged (Story 2.2):** three entries below that named this story are
> closed. `SourcePort.Walk` re-validates every entry from the descriptor it is
> about to read rather than from a path, closing the TOCTOU layering entry, and
> `pinIdentity` now carries the reparse refusal into the claim-time `Lstat`, so a
> junction opened in the validate-to-open window is `path_unsupported` rather
> than `source_changed`. `TestInspectRefusesAPoppedAncestorThatIsNoLongerADirectory`
> pins the not-a-directory fallback that no mutation could previously reach.
> Two entries were re-owned rather than closed: `archiver.go` is renamed to
> `payload.go`, but nothing yet backs its `//nolint:staticcheck` directives, so
> the linter half moves to the story that owns verification tooling; and the CORS
> and `Accept-Ranges` gaps stay open under the story that smoke-tests the
> receiver, because the frozen header matrix makes them an Ask First change that
> Story 2.2 deliberately did not make.
>
> **Discharged (Story 1.10):** the five entries below that named this story are
> closed. `selectVisibleError` and `selectRetainedOutcome` are deleted with their
> tests. The muted/elevated pair is published in `DESIGN.md`, along with every
> other authored pair the views place together, and a test recomputes each ratio
> from the stylesheet's own tokens and fails if the published figure drifts. The
> direct URL is a readonly `<input>` taking its 44px floor from the shared
> `.fd-target` rule. Idle's outline opens on its `h1`, and `copy.help.*`, both
> firewall recoveries, `copy.cancel.won`, `copy.name.show_full` and
> `copy.external.promise` are all rendered. `framer-motion` and the bundled Nunito
> face are gone, and a test fails if either returns.
>
> **Discharged (Story 1.8):** the Story 1.7 entry below requiring the frontend
> reducer to treat forged lifecycle events as a security property is closed.
> `reduceLifecycle` refuses any event whose `sessionId` differs from the active
> session or whose `seq` does not exceed `lastSeq`, refuses every event at a
> state that owns no session at all, and never consumes a sequence for a
> rejected event. Mutation confirms each of those three is load-bearing and
> named by a failing test. The webview can still deliver forged events to its
> own listeners -- no backend change can prevent that -- but they can no longer
> move the visible transfer.

> **Discharged (Story 1.4):** three entries below are now closed and are kept only
> for the trail. `TransferStats.Percent` with a zero total is resolved by
> `ProgressSnapshot.TotalKnown` plus the clamped `percentOf`. `TransferServer.Stop`
> idempotency is resolved by `ServerPort.Stop`, which is force-closing, repeatable,
> and quiescent on every return. The Story 1.3 entry requiring a non-nil `WriteTo`
> error to break the receiver's connection is resolved by `panic(http.ErrAbortHandler)`
> in the download handler, and the server additionally re-checks a `nil` return
> against the advertised length so a short body cannot reach that path as success.

> **Discharged (Story 1.6):** the four Story 1.5 entries that named this story are
> settled. A committed session now has a production-reachable teardown -- `Cancel` and
> `Shutdown` mark the generation, cancel the data-plane context, join the teardown already
> in flight through the operation lease, and return only once the listener, beacon, drainer
> and session context are gone. The CLAIMING wedge is closed by Cancel-from-CLAIMING and
> pinned by `TestCancelFromEveryState/CLAIMING`, which forces the lost revalidation
> deliberately. The unbounded `unwind` wait is *restated* rather than closed, below: it is
> now a deliberate design commitment rather than an oversight, and this story added two
> more waits of the same kind. `diagnosticSink` is restated too -- Story 1.6 does not
> expose it after all, so nothing outside the package reads it yet.

> **Note (Story 1.3):** entries below that name `Streamer`, `StreamFile`, or `StreamZip`
> describe a contract that no longer exists. It was replaced by the server-owned
> `PayloadPort`/`PreparedPayload`. The findings still stand; read them against the new
> contract in `docs/fairdrop-contracts.md`.

- source_spec: `spec-phase-1-wails-scaffold.md`
  id: D-001
  summary: Backend contracts assume a single staged path while the frontend accepts multi-file drops.
  owner: discharged
  evidence: `TransferServer.Start(filePath string, ...)` and `Streamer.StreamFile`/`StreamZip` each take one path, but the I/O matrix requires a three-file drop to be accepted and `App.tsx` stores `string[]`. Phase 5 (`StageTransfer`) must decide: zip a multi-selection, stage only the first, or reject.

- source_spec: `spec-phase-1-wails-scaffold.md`
  id: D-002
  summary: `TransferStats.Percent` has no defined behavior when `TotalBytes` is 0 (unknown-size zip stream).
  owner: discharged
  evidence: Spec §6 Module B says Content-Length is omitted when streaming a directory, so TotalBytes is genuinely unknown for zips. Dividing by it yields NaN/Inf, which `encoding/json` refuses to marshal — the progress event would fail to emit. Phase 4 must define the unknown-total case.

- source_spec: `spec-phase-1-wails-scaffold.md`
  id: D-003
  summary: `Streamer` has no defined way to signal a mid-stream failure after headers are written.
  owner: discharged
  evidence: Once bytes are on the wire a returned error cannot change the status code, so the receiver saves a truncated file that looks complete. Phase 3 should specify `panic(http.ErrAbortHandler)` (or equivalent) to break the connection so the client sees a failed download.

- source_spec: `spec-phase-1-wails-scaffold.md`
  id: D-004
  summary: `TransferServer.Stop` has no documented idempotency contract.
  owner: discharged
  evidence: Spec §3 resets to IDLE after DONE/ERROR and §7 allows user cancellation, so Stop can plausibly be reached twice or before Start. Undefined semantics invite a double-close panic or a leaked listener in Phase 4.

- source_spec: `spec-phase-1-wails-scaffold.md`
  id: D-005
  summary: No CI runs the verification commands, so the verified state decays from the next commit.
  owner: discharged
  evidence: No `.github/`, Makefile, or task runner exists. The spec's six verification commands are hand-run only. A clean-clone CI job would have caught the `go:embed` gap on the very first push.

- source_spec: `spec-phase-1-wails-scaffold.md`
  id: D-006
  summary: Drag-and-drop is the only input path; there is no keyboard or pointer-free way to stage a file.
  owner: discharged
  evidence: The zone is a bare `div` with no role, no `aria-label`, no "Browse…" fallback, and the results list has no `aria-live` region so new paths are never announced. Phase 6 builds the real DropZone and should add a file-picker fallback.

- source_spec: `spec-phase-1-wails-scaffold.md`
  id: D-007
  summary: Path edge cases are untested — spaces, non-ASCII, >260 chars, UNC shares, symlinks, and zero-path drops.
  owner: discharged
  resolution: Story 3.7 native matrix and mutation audit; see D-007 in evidence-3-7-execute-the-native-platform-test-matrix.md.
  evidence: Every path in the matrix and tests is a simple `C:\x\...`. Windows MAX_PATH and UNC handling are real hazards for a file-transfer tool, and they become testable in Phases 2-4 where the Go side actually opens the paths.

- source_spec: `spec-phase-1-wails-scaffold.md`
  id: D-008
  summary: `frontend/tsconfig.json` uses legacy `moduleResolution: "Node"` against a toolchain that publishes subpath exports.
  owner: discharged
  evidence: Vite 7, Vitest 4, and `@tailwindcss/vite` expose entry points such as `vitest/config` through `exports` maps that node10 resolution cannot read. It type-checks today, but `"bundler"` is the resolution these tools expect and the current setting is a latent trap.

- source_spec: `spec-phase-1-wails-scaffold.md`
  id: D-009
  summary: `npm ci --omit=dev` would break `tsc` because test files are inside the build's type-check scope.
  owner: accepted
  evidence: `tsconfig.json` includes all of `src`, which now contains `App.test.tsx` importing `vitest` and `@testing-library/react` (both devDependencies). Not currently reachable — `wails.json` runs plain `npm install` — but a production-flavored CI install would fail the build.

- source_spec: `spec-1-1-validate-and-describe-one-file-selection.md`
  id: D-010
  summary: Claim-time source revalidation and payload opening should pin filesystem identities so an ancestor replacement cannot exploit the metadata snapshot's TOCTOU window.
  owner: discharged
  evidence: Story 1.1 now `Lstat`s every syntactic ancestor, rejects native Windows reparse attributes, and rechecks cancellation, but separate path-based metadata calls cannot atomically prevent a local rename between checks. The binding contract already assigns later defenses to claim-time re-`Lstat` and descriptor-first payload opening; their stories must preserve and verify that layering.

- source_spec: `spec-1-3-prepare-and-stream-a-regular-file-safely.md`
  id: D-011
  summary: A post-header stream failure must break the receiver's connection, and only the HTTP handler can do that.
  owner: discharged
  evidence: `PreparedPayload.WriteTo` reports a mid-stream read, cancellation, or destination failure as a coded `transfer_failed`/`cancelled` error, but `Content-Length` is already on the wire by then, so a plain handler return leaves the receiver holding a truncated file that looks complete. The payload port owns no connection, so Story 1.4 must turn a non-nil `WriteTo` error into a killed connection (`panic(http.ErrAbortHandler)` or equivalent). This supersedes the Phase 1 entry that assigned the same finding to Phase 3.

- source_spec: `spec-1-3-prepare-and-stream-a-regular-file-safely.md`
  id: D-012
  summary: `transfer_failed`'s fixed public copy misdescribes a deadline that expires during Prepare, before any byte is sent.
  owner: discharged
  evidence: Story 1.3 deliberately separates a deadline from a user cancel, but both Prepare-time and stream-time deadlines map to `transfer_failed`, whose registry string is "The transfer stopped before FairDrop finished sending." Prepare runs before headers, so nothing was being sent. The copy registry is fixed by the UX contract, so changing this is a UX decision (EXPERIENCE.md), not an adapter one.

- source_spec: `spec-1-3-prepare-and-stream-a-regular-file-safely.md`
  id: D-013
  summary: The claim-time re-`Lstat` does not re-apply Story 1.1's reparse-point check, so a junction created inside the validate-to-open window surfaces as `source_changed` rather than `path_unsupported`.
  owner: discharged
  evidence: `Payloads.lstatPath` is a bare `os.Lstat`. A reparse point created between `Inspect` and that `Lstat` is caught only incidentally, by `os.SameFile` failing. The transfer is still refused and no wrong bytes stream, so this is a code-accuracy issue rather than a safety hole, but the frozen matrix's link-like row promises `path_unsupported`.

- source_spec: `spec-1-3-prepare-and-stream-a-regular-file-safely.md`
  id: D-014
  summary: `WriteTo`'s once-only CAS is never exercised concurrently, so the race suite never visits the one place two callers can collide.
  owner: discharged
  resolution: Story 3.7 native matrix and mutation audit; see D-014 in evidence-3-7-execute-the-native-platform-test-matrix.md.
  evidence: `TestCloseIsSafeWhenCalledConcurrently` fires eight goroutines at `Close`, but `streamed.CompareAndSwap` is only driven sequentially by `TestWriteToRefusesASecondCall`. The contract says the server never calls `WriteTo` twice, so this is defense-in-depth coverage rather than a live defect.

- source_spec: `spec-1-3-prepare-and-stream-a-regular-file-safely.md`
  id: D-015
  summary: `Prepare` returns a `SourcePort` error verbatim, so the "every Prepare failure is coded" postcondition rests on the adapter rather than being enforced at the boundary.
  owner: discharged
  evidence: `internal/source` complies today, but `SourcePort` is an interface and nothing checks. An uncoded error would only be flattened to `transfer_failed` at the UI boundary, losing the specific code. Enforcing it means wrapping unrecognized errors at the port call, which touches the error-code mapping the spec puts behind Ask First.

- source_spec: `spec-1-3-prepare-and-stream-a-regular-file-safely.md`
  id: D-016
  summary: `internal/stream/archiver.go` no longer archives anything, and no linter backs the `//nolint:staticcheck` directives in its tests.
  owner: discharged
  evidence: `StreamZip` and the zip logic are gone; the file now holds the single-file payload adapter, and Epic 2 will reintroduce directories behind the same port. Renaming to `payload.go` is a Code Map decision for whoever opens Epic 2. Separately, Verification runs build, vet, test, race, gofmt and greps but no staticcheck, so those directives are unenforced decoration.

- source_spec: `spec-1-4-serve-a-one-shot-capability-download.md`
  id: D-017
  summary: `Stop` can block indefinitely while holding the server mutex, which also deadlocks a later `Start`.
  owner: discharged
  evidence: `<-r.serveDone`, `r.handlers.Wait()`, and `r.awaitConnections()` have no deadline. A payload parked on the destination is covered, because `http.Server.Close` breaks it, but a `WriteTo` blocked on a slow source read that ignores its context hangs `Stop` forever, and `s.mu` is held throughout. A watchdog changes the force-closing semantics the contract states, so this needs a design decision rather than a patch.

- source_spec: `spec-1-4-serve-a-one-shot-capability-download.md`
  id: D-018
  summary: CORS is configured for the 200 only, and the receiver page cannot read the filename it was encoded to carry.
  owner: 3-10-settle-the-release-blocking-platform-decisions
  evidence: `Access-Control-Allow-Origin: *` is set in `writeDownloadHeaders` but not by `writeStatus`, so a cross-origin receiver sees an opaque failure instead of 404/410/423. There is no `Access-Control-Expose-Headers: Content-Disposition`, so that page cannot read the name, and no `Accept-Ranges: none`, so a download manager may attempt a range retry against a consumed capability. The frozen matrix fixes the exact header set, so adding any of these is an Ask First change.

- source_spec: `spec-1-4-serve-a-one-shot-capability-download.md`
  id: D-019
  summary: `AuthorizeClaim` is trusted to return, so a coordinator that blocks in it hangs `Stop` and loses quiescence.
  owner: discharged
  evidence: The handler calls it synchronously with `r.ctx` and waits. The contract makes it the coordinator's own synchronous handshake, so bounding it here would duplicate a timeout the coordinator should own -- but nothing on the server side currently survives a coordinator that never returns. Worth settling when Story 1.5 implements the authorizer.

- source_spec: `spec-1-4-serve-a-one-shot-capability-download.md`
  id: D-020
  summary: Repeated `Stop` discards the first call's cleanup diagnostic, and `teardownOnce`/`teardownDone` guard a path with one structurally unreachable entrant.
  owner: discharged
  evidence: `teardown()` is called only after `Stop` takes `s.mu` and clears `s.active`, so a second entrant cannot occur; the `sync.Once` plus channel join therefore protects nothing, while causing later `Stop` calls to return `nil` rather than replaying the diagnostic the contract says they may report.

- source_spec: `spec-1-4-serve-a-one-shot-capability-download.md`
  id: D-021
  summary: `ErrorLog` discards genuine handler panics along with the request diagnostics it is there to silence.
  owner: discharged
  evidence: `log.New(io.Discard, "", 0)` blinds every `net/http` report from this server, including a real panic that is not `http.ErrAbortHandler`. A redacting writer that strips the request line would keep the disclosure property without making a production fault in this package invisible.

- source_spec: `spec-1-4-serve-a-one-shot-capability-download.md`
  id: D-022
  summary: Restart after `Stop` is possible but unspecified and untested.
  owner: discharged
  evidence: `Stop` clears `s.active`, so `Start` -> `Stop` -> `Start` succeeds and builds a fresh run. The type comment says "one listener, one capability token, one authorized download, then nothing", which reads as forbidding it. Whether a server instance is reusable belongs in the contract, since the coordinator will decide whether to construct one per session.

- source_spec: `spec-1-5-stage-and-authorize-a-transfer-transactionally.md`
  id: D-023
  summary: Story 1.5 was split so the QR adapter lands first as Story 1.5a; nothing is deferred, only sequenced.
  owner: discharged
  evidence: The epic puts QR encoding inside the Stage sequence, but `internal/qr` is independently mergeable and unrelated to the coordinator's concurrency work. Building it first lets the coordinator be specified and tested against a real `QRPort` rather than a stub. Both specs belong to sprint key `1-5-stage-and-authorize-a-transfer-transactionally`, which is done only when both are.

- source_spec: `spec-1-5-stage-and-authorize-a-transfer-transactionally.md`
  id: D-024
  summary: `AuthorizeClaim` is now deadlock-free by construction, but it is still not time-bounded, and the one unlocked call inside it is `StopBeacon`.
  owner: discharged
  evidence: This closes half of the Story 1.4 entry above. The handshake never blocks on the operation lease (it takes it with a non-blocking try and answers `cancelled` when a teardown owns it) and never holds the state mutex across a call, so `ServerPort.Stop` can always make progress against a handler parked in it. What remains is the `NetworkPort` side: the claim calls `StopBeacon` synchronously while holding the lease, so an mDNS shutdown that never returns hangs the serving handler and therefore `Stop`. Bounding it needs the same design decision as the unbounded `Stop` entry above, not a patch here.

- source_spec: `spec-1-5-stage-and-authorize-a-transfer-transactionally.md`
  id: D-025
  summary: A CSPRNG failure during Stage has no stable code of its own, so it borrows `transfer_failed`, whose fixed copy describes an interrupted transfer that never began.
  owner: discharged
  evidence: `Coordinator.newIdentity` maps an exhausted entropy source to `transfer_failed` because the code table has no entry for it and the contract sends everything unrecognized to that fallback. The registry string is "The transfer stopped before FairDrop finished sending," but nothing was staged, advertised, or sent. Same shape as the Story 1.3 entry about a Prepare-time deadline: the copy registry is fixed by the UX contract, so a new code or new copy is a UX decision (EXPERIENCE.md), not a coordinator one. The failure is unreachable in practice on a healthy host.

- source_spec: `spec-1-5-stage-and-authorize-a-transfer-transactionally.md`
  id: D-026
  summary: A committed session has no production-reachable teardown until Story 1.6 lands.
  owner: discharged
  evidence: After a successful Stage, the only code that cancels the session context, stops the server and beacon, or joins the drainer is `failStage`. `cancelSession` and `beginClosing` are unexported and called only from tests, so a staged listener, mDNS registration, session context, and drainer goroutine currently outlive the coordinator with nothing able to release them. Story 1.6's Cancel and Shutdown are the fix; recording it because the code alone reads as an omission rather than a sequencing decision.

- source_spec: `spec-1-5-stage-and-authorize-a-transfer-transactionally.md`
  id: D-027
  summary: `unwind` waits on the drainer unbounded while holding the operation lease.
  owner: discharged
  evidence: `<-live.drainerDone` depends entirely on `ServerPort.Stop` closing the event lane. A Stop that returns without closing it wedges the coordinator in permanent `busy` with no timeout and no diagnostic. This is the second unbounded wait in the file -- the deferred entry about `StopBeacon` inside `AuthorizeClaim` names the first -- and no test drives a Stop that leaves the lane open, because the fake always closes it.

- source_spec: `spec-1-5-stage-and-authorize-a-transfer-transactionally.md`
  id: D-028
  summary: A claim that loses its post-`StopBeacon` revalidation leaves the state at CLAIMING with no public exit.
  owner: discharged
  evidence: The contract gives Cancel-from-CLAIMING to Story 1.6, so this is the intended division of labour rather than a defect. Until 1.6 lands, though, a lost revalidation wedges the coordinator: every later Stage answers `busy` and every claim answers `cancelled`. Worth a deliberate test in 1.6 rather than being discovered there.

- source_spec: `spec-1-5-stage-and-authorize-a-transfer-transactionally.md`
  id: D-029
  summary: `AuthorizeClaim` can return `transfer_failed`, which the contract's claim-authorization row does not list.
  owner: discharged
  evidence: `ready()` yields `ErrTransferFailed` when a port is missing, but `docs/fairdrop-contracts.md` says claim authorization returns `cancelled` or `shutting_down` only. A missing port is a wiring defect rather than a runtime outcome, so the honest fix may be to make it unrepresentable at construction instead of widening the contract. Related: `Stage(nil ctx)` is `transfer_failed` while `AuthorizeClaim(nil ctx)` is `cancelled`, for one class of programmer error.

- source_spec: `spec-1-5-stage-and-authorize-a-transfer-transactionally.md`
  id: D-030
  summary: `NetworkPort` does not document that `StartBeacon` requires a prior successful `GetLocalIP`.
  owner: discharged
  evidence: `internal/network` enforces that ordering and answers `beacon_warning` without it, but the port doc states no such precondition and the coordinator's fake accepts `StartBeacon` at any time. Swapping the coordinator's address and beacon steps would keep every coordinator test green and fail in production. The ordering belongs on the port, and the fake should assert it.

- source_spec: `spec-1-5-stage-and-authorize-a-transfer-transactionally.md`
  id: D-031
  summary: `diagnosticSink` silently drops entries past 32 with no marker.
  owner: discharged
  evidence: A truncated sink is indistinguishable from a complete one in a structure whose stated purpose is to be inspected, and the policy drops newest rather than oldest. Nothing outside the package reads it until Story 1.6 exposes it, which is the moment to settle both.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  id: D-032
  summary: Every quiescence wait in the coordinator is unbounded, and this story added two more.
  owner: discharged
  evidence: Restates the Story 1.5 entry about `unwind`. `Cancel` and `Shutdown` wait on the operation lease, then on `<-live.drainerDone`, then inside `releaseAcquired` on `ServerPort.Stop` and `NetworkPort.StopBeacon`. Each rests on a port postcondition -- Stop is quiescent on every return, StopBeacon guarantees no advertisement remains -- so an adapter that violates one wedges the command with no timeout and no diagnostic. The spec puts a watchdog behind Ask First deliberately: bounding these would let Cancel report success while a listener was still live. The decision belongs with the `internal/server` entry about `Stop` blocking on a source read that ignores its context, which is the one concrete way this can happen today.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  id: D-033
  summary: `session.terminal` is redundant with the state check that follows it, and no mutation can distinguish them.
  owner: accepted
  evidence: `drainerMayActLocked` refuses an event when `live.terminal` is set *and* when the state is no longer TRANSFERRING. Acceptance sets the flag and the settled state on the same goroutine with no other entry point in between -- `acceptTerminal` and `forwardProgress` are both called only from `drain` -- so deleting the flag leaves every test green (verified by mutation). It is kept because the spec names it and because it makes exactly-once acceptance independent of where the state transition lands, but it is defense in depth, not a tested guarantee. Either delete it or move the settled-state transition into the acceptance critical section and let the flag be the only record.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  id: D-034
  summary: A blocking `Observer.Publish` now stalls every lifecycle command, not just the event lane.
  owner: discharged
  evidence: Publication is lease-owned by design, so the coordinator calls `Publish` while holding the operation lease. A Wails observer that blocks -- an emit into a frontend that is not draining, say -- therefore blocks the next `Cancel` or `Shutdown` for as long as it blocks, because those wait for the lease. The contract already calls `Publish` a synchronous FIFO handoff, so this is a constraint on the Story 1.7 adapter rather than a coordinator defect: whatever implements `Observer` must return promptly and must never call back into the coordinator.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  id: D-035
  summary: A `ServerComplete` carrying no snapshot publishes a zero-value one, and no contract row covers that shape.
  owner: 3-11-close-the-residual-contract-and-copy-gaps
  evidence: The payload table makes `progress` required on `transfer-complete`, and the port says Complete always carries the authoritative snapshot, so a nil there is a port defect with no defined outcome. `acceptTerminal` reports the unknown-total zero snapshot rather than downgrading a success the receiver actually got, and publishes no separate final-progress event. The alternative -- treating it as `transfer_failed` -- would lie about a transfer that completed. Worth a contract sentence either way, since the current behaviour is a coordinator choice rather than a stated rule.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  id: D-036
  summary: `Cancel` and `Shutdown` take no context, so a Wails command cannot abandon one.
  owner: discharged
  evidence: Both return only when the resources they name are quiescent, which is the contract's requirement, so a caller-supplied context would be a parameter they must ignore. Story 1.7 wires `App.CancelTransfer` and the Wails shutdown hook to them: the hook must be prepared to block for as long as the adapters take, and `App.shutdown` is the only place that may call `Shutdown`.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  id: D-037
  summary: `armReset`'s window between creating the reset timer and re-checking that the session survived is guarded but unproven, and can leave one timer armed and unstopped.
  owner: discharged
  evidence: Found by mutation: replacing the second `resetIsDueLocked` check with an unconditional `live.stopReset = stop` leaves every test green. The window is real but narrow. `retire` reads `stopReset` before it calls `joinDrainer`, and the drainer is the goroutine running `armReset`, so a Cancel that arrives just after the terminal outcome reads a nil `stopReset`, then blocks until `armReset` finishes -- by which point `armReset` has seen the session still installed and armed a timer nobody will ever stop. That timer fires three seconds later, revalidates, finds the session cleared and publishes nothing, so the consequence is one leaked pending callback rather than a second reset. `fakeTimer` already exposes `armed()` and `stops()`, so the assertion exists; what is missing is a deterministic way to hold the drainer inside `armReset` while a Cancel runs.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  id: D-038
  summary: The three step-04 review layers never ran for this story, so its only adversarial coverage is self-review plus mutation.
  owner: discharged
  evidence: Blind Hunter, Edge Case Hunter and Verification Gap were all launched together and all three terminated on an Anthropic session rate limit (HTTP 429) before returning findings. Their exact child prompts are preserved in `review-layer-prompts-1-6.md` so they can be run in a separate session, ideally on a different model. Twenty-four mutations stood in for them and found three real verification gaps, which are fixed, but mutation only tests guarantees somebody already thought to encode -- it cannot find a missing requirement, which is precisely what the Blind Hunter layer is for.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  id: D-039
  summary: `sanitizeProgress` distrusts NaN but trusts the known/unknown total invariant it sits next to.
  owner: 3-11-close-the-residual-contract-and-copy-gaps
  evidence: The function's own comment says this is "the boundary that cannot afford to trust" the producing adapter, and it clamps `Percent` and `SpeedBytesPerSec` accordingly. It does not enforce the rest of the contract's progress rules: a snapshot with `TotalKnown=false` and a non-zero `Percent`, or a negative `BytesSent`, passes through unchanged, though the contract fixes unknown totals at `TotalBytes=0, Percent=0`. `internal/server`'s meter complies today, so this is defense-in-depth rather than a live defect, but the asymmetry means the comment overstates what the function does.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  id: D-040
  summary: The two guards that refuse a drainer event on a settled session mask each other, so neither can be verified on its own.
  owner: accepted
  evidence: Completes the entry above about `session.terminal`. Mutation runs both ways: deleting the `live.terminal` check leaves the suite green because the state check refuses the event, and widening the state check to accept DONE and ERROR also leaves it green because `live.terminal` refuses it. A third defence covers the same window -- the drainer's non-blocking lease acquisition fails while the terminal path still owns the lease. The behaviour is correct and triply defended; what cannot be done today is prove any single one of them is doing the work, which is why `TestASecondTerminalEventIsDiscarded` should not be read as pinning the state check.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  id: D-041
  summary: Two hardening steps inside `fireReset` are correct but cannot be made to fail deterministically without a seam that does not exist.
  owner: accepted
  evidence: Mutation survivors after review round 2. Moving `live.stop()` back after `releaseLease`, and deleting the `closing` re-check that follows `joinDrainer`, both leave the suite green. Each needs a competitor parked at one instant -- a Cancel blocked in `awaitLease` that inspects the session context the moment the lease is handed back, and a Shutdown that raises `closing` while the reset is inside `joinDrainer`. The fakes can park a goroutine inside an adapter call but not between two statements of the coordinator's own, so forcing either window needs a test-only hook in `fireReset`. Both changes are kept because they close real windows the sibling paths already close; neither is a tested guarantee.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  id: D-042
  summary: An event lane that closes while the session is STAGED or CLAIMING leaves the coordinator holding a dead session with no synthesized outcome.
  owner: discharged
  evidence: Raised by the edge-case layer, which proposed widening `acceptTerminal` to accept those states. That fix would break the event grammar: publishing `transfer-error` from STAGED means an error with no preceding `started`, and the contract's grammars all begin with `started` for anything after Stage acknowledgement. It is also unreachable today -- only `ServerPort.Stop` closes the lane, and every caller of Stop is a teardown that drives to IDLE -- and the user can still recover with Cancel. What is missing is a defined grammar for a server that dies under a staged-but-unclaimed session, which is a contract question rather than a coordinator one.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  id: D-043
  summary: A panicking `Observer.Publish` permanently wedges the coordinator, because no lease release is deferred.
  owner: discharged
  evidence: Raised by the edge-case layer. `AuthorizeClaim` runs on the serving goroutine, where `net/http` recovers a handler panic; the `c.publish(event)` before `c.releaseLease()` is not deferred, so a panicking observer leaves the lease held forever and every later `Cancel` or `Shutdown` blocks in `awaitLease` with no timeout. The drainer sites have the same shape but no recovery at all, so there the process dies instead. The root shape predates this story (Story 1.5 wrote the AuthorizeClaim path), and converting five lease sites to deferred release during triage risks a double release, which panics. Worth a focused pass together with the `Observer` constraint already recorded above.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  id: D-044
  summary: `Stage` during the three-second terminal lease is refused with `busy`, whose fixed copy tells the user to finish or cancel a transfer that already ended.
  owner: discharged
  evidence: Now covered by `TestStageIsRefusedDuringTheTerminalLease`, so the refusal itself is pinned; what is unresolved is the copy. `busy` renders as "Finish or cancel the current transfer before choosing another item." Nothing is in progress and there is nothing to finish -- the transfer completed and the coordinator is holding its outcome on screen for three seconds. The copy registry is fixed by the UX contract, so a new code or new copy is an EXPERIENCE.md decision, the same shape as the Story 1.3 and 1.5 entries about `transfer_failed` describing a transfer that never began.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  id: D-045
  summary: Four smaller test-quality findings from the review layers are recorded but not yet fixed.
  owner: discharged
  evidence: `TestServerFailureCodedCancelledIsPublishedAsATransferFailure` asserts the published code is `transfer_failed` and then asserts the message is not the cancellation copy, but `PublicErrorOf` sources the message from the code, so the second assertion cannot fail independently -- it is now redundant with `TestATerminalFailureOnlyPublishesCodesThatDescribeIt`, which pins the whole table. `TestLaneClosureDuringATeardownIsSilent` asserts `got == stateError` rather than pinning the state to TRANSFERRING, never checks the `server.Stop` count, and ends with a dangling `_ = metadata`. `TestProgressIsRefusedOutsideAMatchingTransfer`'s STAGED case emits two snapshots but the shared assertion only looks for the second, so forwarding the first pre-claim snapshot would pass. And no test emits events on a second session, so `seq` restarting at 1 with a new session id -- load-bearing for the frontend's discard rule -- is unproven; `assertEventGrammar` would need to filter per session to check it.

- source_spec: `spec-1-7-expose-safe-transfer-commands-through-wails.md`
  id: D-046
  summary: The webview can deliver forged lifecycle events to its own listeners, and only the Story 1.8 reducer can defend against it.
  owner: discharged
  evidence: Wails' desktop runtime calls `notifyListeners(payload)` inside `EventsEmit` before `window.WailsInvoke('EE'...)`, so a script in the window reaches every `EventsOn` subscriber without Go being involved. Removing the bound `Publish` command closed the route through this process; it cannot close that one, and no backend change can. The contract already specifies the defence -- initialize `(sessionId, lastSeq)` only from a successful Stage result and ignore events carrying another session or a seq at or below the last one -- so Story 1.8 must implement it as a security property rather than as tidiness, and Story 1.10's recovery contract should say what a rejected event does.

- source_spec: `spec-1-7-expose-safe-transfer-commands-through-wails.md`
  id: D-047
  summary: `errNotComposed` and the pre-startup dialog refusal render as "The transfer stopped before FairDrop finished sending", which describes something that never happened.
  owner: discharged
  evidence: Both use `ErrTransferFailed`, so `PublicErrorOf` discards their safe messages and selects that fixed copy. Neither state involves a transfer that began. There is no not-ready code in the `ErrorCode` set, and the copy registry is fixed by the UX contract, so this is an EXPERIENCE.md decision -- the same shape as the Story 1.3, 1.5 and 1.6 entries about `transfer_failed` and `busy` describing states they do not fit. Both states are unreachable in a composed binary, since Wails runs `OnStartup` before the webview can call a command.

- source_spec: `spec-1-7-expose-safe-transfer-commands-through-wails.md`
  id: D-048
  summary: `StageTransfer` and `CancelTransfer` fabricate a background context before startup while the dialogs refuse, so the two halves of the boundary disagree about "no window yet".
  owner: 3-11-close-the-residual-contract-and-copy-gaps
  evidence: `delegate()` defaults a nil `a.ctx` to `context.Background()`, so a pre-startup Stage would bind a listener and start a beacon whose every lifecycle event `publish` then drops -- handing the UI a session it can never hear from. `chooseWith` refuses instead, because the real dialog would `log.Fatalf`. Unreachable today for the same reason as the entry above. Settling it means choosing whether a command may run before the window exists at all, which is a contract question rather than an adapter one.

- source_spec: `spec-1-7-expose-safe-transfer-commands-through-wails.md`
  id: D-049
  summary: `undelivered` counts dropped lifecycle events and nothing in production reads it.
  owner: discharged
  evidence: Incremented on a nil context, a recovered emit panic, and an unknown event kind; read only by tests. A dropped terminal event is precisely the failure no other party can observe -- the UI simply waits forever -- so the count is the only trace, and it is inert. The spec forbids logs, so surfacing it means a UI-visible degraded state, which belongs to Story 1.10's recovery contract. The comment now says it is inert rather than calling itself "the record".

- source_spec: `spec-1-7-expose-safe-transfer-commands-through-wails.md`
  id: D-050
  summary: Nothing runs the frontend test suite automatically, and there is no CI at all.
  owner: discharged
  evidence: `wails.json` wires `frontend:build` to `npm run build` (`tsc && vite build`) and never `npm test`, there is no `.github/workflows` directory, and `frontend/tsconfig.json` includes only `src`, so `frontend/wailsjs` is not type-checked on its own. `errors.test.ts` runs only when someone types `npm test`. Story 3.2 owns reproducible cross-platform verification; this is the concrete list of what it has to pick up.

- source_spec: `spec-1-7-expose-safe-transfer-commands-through-wails.md`
  id: D-051
  summary: Two hardening changes at the Wails boundary are correct but undistinguishable by any test.
  owner: accepted
  evidence: Narrowing the cross-language pin to search inside the `transferErrorCodes` array rather than the whole file closes a real hole -- a code surviving only in a comment would have satisfied the old check -- but every one of the twelve codes genuinely sits inside that array, so both forms pass and mutation cannot tell them apart (verified: the earlier "caught" result was a compile error from an unused variable, which proves nothing either way). The same applies to `appObserver`'s nil-app guard, which no production path can reach. Both are kept; neither is a tested guarantee.

- source_spec: `spec-1-8-manage-session-scoped-frontend-state-and-events.md`
  id: D-052
  summary: Regenerating `epic-1-context.md` silently discards refinements, and the next `compile-epic-context` run will do it again.
  owner: accepted
  evidence: The 2026-08-29 run replaced a 1293-word compile with a 1064-word one, dropping the copy-registry-by-stable-key rule, the lowerCamelCase/`transfer-*`/`context.Context` conventions, the transitional-interface deletion rule, "every focus target proven to exist before focusing", Epic 1's own native download exit check, and the whole receiver-help paragraph that Stories 1.9 and 1.10 render from. The file is the compiled orientation artifact every later story reads, its own header invites free editing, and nothing reconciles an edit against a later regeneration -- so the loss was invisible until a diff was read. Restored by hand for Epic 1. The durable fix is a workflow question rather than a code one: either treat the compiled context as generated-only and move refinements into the planning artifacts it compiles from, or stop regenerating it once an epic is underway.

- source_spec: `spec-1-8-manage-session-scoped-frontend-state-and-events.md`
  id: D-053
  summary: A malformed Stage acknowledgement is cancelled and reported, but nothing tells the user their selection was refused rather than lost.
  owner: discharged
  evidence: `stage()` falls back to `publicError('transfer_failed')`, whose fixed copy says the transfer "stopped before FairDrop finished sending" -- the same mismatch the Story 1.3, 1.5, 1.6 and 1.7 entries record for states where no transfer began. Correct within this story, which may not change copy; it is the fifth instance of one missing code, and the accumulated case belongs to an EXPERIENCE.md decision before Story 1.10 fixes recovery text against it.

- source_spec: `spec-1-8-manage-session-scoped-frontend-state-and-events.md`
  id: D-054
  summary: The frontend suite still runs only when someone types `npm test`, one story after the same finding.
  owner: discharged
  evidence: `wails build` regenerates bindings and compiles the frontend without running a single Vitest file, and there is still no `.github/workflows`. Story 1.8 raised the frontend suite from 35 tests to 164 and made it the only executable evidence for the forged-event defence, which raises the cost of the gap rather than changing it. Restated here so Story 3.2 sees that it grew.

- source_spec: `spec-1-9-render-the-paper-relay-transfer-views.md`
  id: D-055
  summary: The native window paints a single background colour, so one of the two themes still gets a one-frame flash.
  owner: 3-10-settle-the-release-blocking-platform-decisions
  evidence: `main.go` takes one `options.RGBA` and Wails offers no per-theme value, so the constant now tracks the light `--color-canvas` and a dark-mode OS gets one light frame before the webview paints. Before this story it tracked a Tailwind class the story deleted, so light mode flashed slate-900 on every launch and nothing failed -- `main_test.go` now pins the constant to the token and names the coupling. Closing the residual means reading the OS theme in Go before building the options: on Windows that is one `golang.org/x/sys/windows/registry` read of `AppsUseLightTheme` (already an indirect dependency), with a build-tag sibling for macOS. That is platform code Story 1.9 was not scoped for, and Story 3.3's release evidence is where a first-paint check belongs.

- source_spec: `spec-1-9-render-the-paper-relay-transfer-views.md`
  id: D-056
  summary: `selectVisibleError` and `selectRetainedOutcome` are now dead, and one of them answers the same question differently from its replacement.
  owner: discharged
  evidence: No view imports either; `selectCommandError` and `selectOutcome` replaced them. `selectVisibleError` folds a retained terminal error into the "visible error" and does not refuse `cancelled` -- precisely the two behaviours the replacements were written to prevent -- so the module exports contradictory answers and a later view can pick the wrong one and still compile. Not removed here because the spec's Code Map says `selectors.ts` may be extended "only additively", which is the right default for a module Story 1.8 defended; deleting an exported symbol and its tests is a separate, deliberate edit.

- source_spec: `spec-1-9-render-the-paper-relay-transfer-views.md`
  id: D-057
  summary: Muted text now sits on the elevated surface, a pairing DESIGN.md's contrast table does not publish.
  owner: discharged
  evidence: Every muted string in the new views (`.fd-meta`, `.fd-trust`, `.fd-packet-tab`) renders inside `.fd-packet`/`.fd-transfer-view`, which are `--color-elevated`. DESIGN.md proves muted/canvas at 5.070533:1 and instructs "Re-run unrounded automated checks if ... adjacent surfaces change". Computed here, muted/elevated is 4.504478:1 light and 6.569759:1 dark -- it passes AA, with about 0.1% headroom in light mode, and `.fd-packet-tab` renders it at 12px. Story 1.10 owns the unrounded contrast proof and should add this pair to the published table rather than leave it derived.

- source_spec: `spec-1-9-render-the-paper-relay-transfer-views.md`
  id: D-058
  summary: The direct URL is exposed as a `div` with `role="textbox"`, which assistive technology handles inconsistently.
  owner: discharged
  evidence: A textbox role with no editable host is not a pattern browsers agree on; a readonly `<input>` or a labelled `<output>` carries the value reliably and keeps it selectable. It also joins the Tab order and takes its 44px floor from a duplicated `min-block-size` rather than the shared `.fd-target` rule that a test pins. Story 1.10 owns the accessibility contract and should settle the element, not just its name.

- source_spec: `spec-1-9-render-the-paper-relay-transfer-views.md`
  id: D-059
  summary: A terminal outcome offers no control at all, so a lost `transfer-reset` strands the window.
  owner: discharged
  evidence: Done and Error render heading, message and nothing else; the way out is the backend's three-second reset, which produces the retained Idle node that carries Dismiss. The Story 1.7 entry above records that `publish` silently drops an event it cannot deliver and counts it in an inert `undelivered`, so a dropped reset is both possible and invisible. The frontend is forbidden a lifecycle timer, which makes this a recovery-contract question for Story 1.10 rather than something a view can fix.

- source_spec: `spec-1-9-render-the-paper-relay-transfer-views.md`
  id: D-060
  summary: Idle's document outline opens on an `h2`, and the registered-but-unrendered Story 1.10 copy is tracked only in spec prose.
  owner: discharged
  evidence: The firewall guidance is an `<h2>` and any outcome panel above it is another, while the page's only `<h1>` is the drop instruction below both -- a consequence of the acceptance criterion that puts preflight first in document order, and no test pins the resolution. Separately, `copy.help.*`, both firewall recovery strings, `copy.cancel.won`, `copy.name.showFull` and `copy.external.promise` are registered and rendered nowhere, and nothing outside this spec's Design Notes records that debt. Story 1.10 owns both; a test asserting these strings stay unrendered until it does would make the boundary executable.

- source_spec: `spec-1-9-render-the-paper-relay-transfer-views.md`
  id: D-061
  summary: Two declared assets are now referenced by nothing: the bundled Nunito face and `framer-motion`.
  owner: discharged
  evidence: `style.css` no longer declares `@font-face`, so `frontend/src/assets/fonts/nunito-v16-latin-regular.woff2` and its `OFL.txt` are unreferenced (Vite will stop emitting the file, but it stays in the tree). `framer-motion` is a locked runtime dependency that nothing imports, and this story's Never list bans every animation it would serve. Neither is a defect; both are weight a later story should either use or drop deliberately.

- source_spec: `spec-1-9-render-the-paper-relay-transfer-views.md`
  id: D-062
  summary: The phase re-check added to the chooser's failure path is correct but adds nothing the reducer was not already doing.
  owner: accepted
  evidence: Applied during review so a chooser that failed while a drop was being staged could not report against another session. Mutation shows it is undistinguishable: `browse` reserves its own generation, so its `stage-requested` is refused for not being Idle and its `stage-failed` is refused on generation mismatch -- the reducer rejects both halves without the guard. Kept as a documented second line, in the same spirit as `selectCommandError`'s redundant `cancelled` refusal, but it is not a tested guarantee and should not be described as a race fix.

- source_spec: `spec-1-9-render-the-paper-relay-transfer-views.md`
  id: D-063
  summary: The working tree holds mixed line endings, so text-matching tests and diffs behave differently per file.
  owner: discharged
  evidence: `TransferringView.tsx` and `StagedView.tsx` are CRLF on disk while `useTransfer.ts` and `style.css` are LF, because only `*.go` was pinned before this story. Two review mutations silently matched nothing for that reason and had to be re-run against normalized text -- a harness that reports "anchor matched 0" rather than a failure is exactly how a mutation pass overstates its own coverage. `.gitattributes` now pins `*.css`, `*.ts` and `*.tsx` to LF, so git renormalizes them on the next write, but nothing has rewritten the existing files and no check fails while they disagree.

- source_spec: `spec-1-9-render-the-paper-relay-transfer-views.md`
  id: D-064
  summary: Wails' custom `wails://` scheme is not a secure context, so every browser API gated on one is unavailable on macOS.
  owner: 3-10-settle-the-release-blocking-platform-decisions
  evidence: WKWebView loads `wails://wails/` through `setURLSchemeHandler:`, and Wails 2.15.0 registers no secure scheme anywhere in its darwin frontend; WebView2 loads `http://wails.localhost/`, which Chromium treats as trustworthy. The clipboard hit this first and is fixed by routing through `runtime.ClipboardSetText`, but the asymmetry is general: `crypto.subtle`, `navigator.geolocation`, media capture and service workers are gated the same way, and a frontend feature that works in `wails dev` on Windows can be inert on macOS with no error. Worth a line in the project's agent instructions before another story reaches for a browser API, and worth confirming on a real Mac during Story 3.3's release evidence.

- source_spec: `spec-1-10-meet-the-accessibility-and-recovery-contract.md`
  id: D-065
  summary: The QR substrate now opts out of forced colors, which `DESIGN.md` gates on native scan evidence that does not exist.
  owner: 3-12-capture-the-accessibility-evidence-a-runner-can-produce
  evidence: `DESIGN.md` allows `forced-color-adjust: none` on the production QR bitmap and its quiet-zone substrate "only after native scan evidence confirms it remains readable", and the Compatibility and Evidence Gates record no such run. The alternative is worse -- without the opt-out the user agent repaints the quiet zone in the system palette and a scanner loses the code -- so the exemption is applied and scoped to `.fd-qr-panel, .fd-qr`, with a test that fails if a second selector ever takes it. What is missing is the evidence, not the rule: a real high-contrast Windows session and a phone camera, recorded like every other gate.

- source_spec: `spec-1-10-meet-the-accessibility-and-recovery-contract.md`
  id: D-066
  summary: The `beacon_warning` row of the announcement table cannot fire from any transition the reducer can produce.
  owner: accepted
  evidence: A discovery warning reaches the frontend inside the successful `FileMetadata`, so it lands on the same transition as Stage success -- and that transition's owner is the focused Staged heading. Giving the warning the announcer as well would be the double speech the whole table exists to prevent, so `routeTransition` announces it only when warnings appear at a session that is already Staged, which today no event does. The warning is on the screen either way, in its own banner. Closing it properly means a lifecycle event that adds a warning after STAGED, which is a contracts change rather than a frontend one.

- source_spec: `spec-1-10-meet-the-accessibility-and-recovery-contract.md`
  id: D-067
  summary: Idle with a retained outcome has two `h1`s.
  owner: accepted
  evidence: The retained node keeps the terminal panel's heading rank because reset "does not change what the user is looking at", and the drop instruction is Idle's own state heading. Two top-level headings is valid HTML and not a WCAG failure, and the alternative -- demoting the panel to `h2` at reset -- changes the visible node on a transition whose announcement owner is None. Recorded because it is a deliberate outline choice a later reviewer will otherwise read as an oversight.

- source_spec: `spec-1-10-meet-the-accessibility-and-recovery-contract.md`
  id: D-068
  summary: The accessibility floor is proved against the stylesheet and the DOM, never against a rendered layout or a real screen reader.
  owner: 3-12-capture-the-accessibility-evidence-a-runner-can-produce
  evidence: jsdom performs no layout and evaluates no media query, so 320-pixel reflow, the 44px target floor, 200% text, forced colors and reduced motion are all asserted as stylesheet text; and no automated check can hear what a screen reader says. The routing table, the throttle and every focus target are unit-proved, but "each transition is announced exactly once" is ultimately an observation about NVDA or VoiceOver. The spec's own manual checks -- one keyboard-only transfer with a screen reader running, and Staged at 320 CSS pixels with 200% text and forced colors on -- are still owed, and belong with the release evidence rather than in a story that cannot run them.

- source_spec: `spec-1-10-meet-the-accessibility-and-recovery-contract.md`
  id: D-108
  owner: discharged
  summary: A single unreproduced failure in `internal/transfer`, whose evidence the gate discarded.
  evidence: `go test ./...` failed once in `internal/transfer` on 2026-09-01 during the Story 1.10 gate, then did not reproduce in 40 in-process iterations plus 12 separate processes. The failure detail was lost because the command piped through `tail -3`, which discarded everything above the summary -- a gate that hides the evidence it exists to surface. Story 1.6 fixed a 1-in-40 flake in this same package, so a second one is plausible rather than hypothetical. Verification should run the suite without swallowing output and should keep failing runs.

- source_spec: `spec-1-10-meet-the-accessibility-and-recovery-contract.md`
  id: D-109
  owner: 3-12-capture-the-accessibility-evidence-a-runner-can-produce
  summary: Story 1.10 was reviewed by one adversarial layer instead of three, because a rate limit killed the other two.
  evidence: Only the edge-case layer completed for Story 1.10; Blind Hunter and the verification-gap layer both terminated on a session rate limit. Blind Hunter is the layer that looks for what is missing rather than what is wrong, and it is the one that found the unexecuted `appObserver` in 1.7 and the blank-window test in 1.9. Story 1.10 closes Epic 1 and its own acceptance criteria are the epic's accessibility gate, so the thinnest review in the epic sits on its most cross-cutting story. Re-running the two layers against `f7338af..HEAD` costs nothing but time.

- source_spec: `spec-1-10-meet-the-accessibility-and-recovery-contract.md`
  id: D-069
  summary: The recovery panel carries no heading, which matches the spine, and shows both platforms because the spine lists both.
  owner: accepted
  evidence: `EXPERIENCE.md`'s "Firewall Preflight and Recovery" section gives the preflight block a label ("Local network access") and gives the recovery block none -- it is a bulleted list under a section heading, with items labelled "Windows recovery" and "macOS recovery" and no platform detection implied. So both halves of this finding describe the contract rather than a deviation from it, and a heading would be invented copy. What was a real defect is fixed: `RecoveryHelp` rendered `copy.label.windows`/`macos`, the preflight block's labels, so the two blocks displayed identically and the second read as a duplicate of the first. It now uses the spine's own recovery labels. A future story wanting the panel in heading navigation needs a registered string first, which is an EXPERIENCE.md change.

- source_spec: `spec-1-10-meet-the-accessibility-and-recovery-contract.md`
  id: D-070
  summary: Story 1.9's spec stated Idle's document order more strictly than the spine it was derived from, and Story 1.10 followed the spine.
  owner: discharged
  evidence: The line is `EXPERIENCE.md`, "Firewall Preflight and Recovery": "The following guidance is always available in Idle **before selection controls**, in document order." Before the selection controls, not first in the document -- so drop target, then preflight, then controls satisfies it, and so would preflight first. Story 1.9's acceptance criterion wrote the stricter reading, and the UX contract wins on presentation where the two disagree. Story 1.10's Spec Change Log made the right call and cited nothing; the citation is here. The related two-`h1` case (a retained outcome above Idle's own heading) remains accepted, and no test renders that configuration.

- source_spec: `spec-1-10-meet-the-accessibility-and-recovery-contract.md`
  id: D-071
  summary: The full-name affordance renders for every name, including names that were never clamped, which is what the design requires.
  owner: accepted
  evidence: `DESIGN.md`: "A two-line visual clamp may be used only with a persistent keyboard-operable control labeled by ... `copy.name.show_full` and an assistive description containing the complete value; a tooltip alone is insufficient." Persistent is the requirement, and the clamp is applied by CSS to every name, so a control that appeared only when the text overflowed would be the deviation. The verbosity cost for a short name is real and unmeasured -- deciding whether it outweighs the rule needs layout, which jsdom does not provide, so it belongs with the native accessibility pass rather than here.

- source_spec: `spec-1-10-meet-the-accessibility-and-recovery-contract.md`
  id: D-072
  summary: Three test-quality items from the review that are real but did not change behaviour.
  owner: discharged
  evidence: `App.focus.test.tsx` redefines `mountWith`, `transitionTo` and the controller stub that `App.test.tsx` already has, with a different signature, so the two drift as the controller gains methods. The StrictMode describe is named for effect replay but only its first test exercises it -- React double-invokes effects on mount only, so the rest are ordinary updates. And `data-transfer-phase` can disagree with `data-phase-view` (a terminal `error` carrying `cancelled` renders the Idle body while the shell still advertises `error`); both are test and styling surfaces and nothing says which is authoritative.

- source_spec: `spec-1-10-meet-the-accessibility-and-recovery-contract.md`
  id: D-073
  summary: `DESIGN.md` gained nine contrast pairs as prose beside the table that would have held them, and does not say a test now owns the figures.
  owner: 3-9-record-human-release-evidence
  evidence: Eighteen figures were added as one run-on sentence under a formatted contrast table. Separately, the retained instruction "Re-run unrounded automated checks if opacity, blending, color-mix, or adjacent surfaces change" predates `styles.test.ts` recomputing and pinning these ratios, so an editor following the spine's own instruction would hand-edit values a test derives -- and the test would then fail against the document it is meant to serve.

- source_spec: `spec-2-1-validate-and-stage-one-directory.md`
  id: D-074
  summary: Execute Story 2.1's production no-follow, search-only-ancestor, and non-reading special-file tests on native Linux and macOS runners.
  owner: discharged
  resolution: Story 3.7 native matrix and mutation audit; see D-074 in evidence-3-7-execute-the-native-platform-test-matrix.md.
  evidence: The Linux and Darwin test binaries cross-compile and include direct tests for post-metadata symlink substitution, search-only ancestors, Linux `O_PATH`, and FIFO refusal, but this Windows host cannot execute those platform implementations. Windows production behavior, deterministic cross-platform seams, and cross-compilation are green; only native POSIX execution remains unproved.

- source_spec: `spec-2-1-validate-and-stage-one-directory.md`
  id: D-075
  summary: Pin the not-a-directory fallback after the component walk, which no mutation can currently reach.
  owner: discharged
  evidence: `source.go`'s `if !currentInfo.IsDir()` after the lexical walk survives deletion with the suite green. It is genuinely reachable -- a `..` pop re-`Stat`s the popped ancestor without re-running `rejectUnsupportedInfo`, so an ancestor swapped for a special file between descent and pop lands there -- but the fake handle harness cannot yet vary a non-opened handle's reported mode between two stats, which is what the case needs. Story 2.2 owns claim-time revalidation and the same TOCTOU window.

- source_spec: `spec-2-2-stream-a-safe-directory-zip.md`
  id: D-076
  summary: Execute the POSIX content-open guards -- O_NONBLOCK plus fstat-then-reject -- on native Linux and macOS.
  owner: discharged
  resolution: Story 3.7 native matrix and mutation audit; see D-076 in evidence-3-7-execute-the-native-platform-test-matrix.md.
  evidence: `posixNode.OpenChildContent` is the only read-granting open in the source package and the architecture text now claims a FIFO cannot block the response, but nothing asserts it. Every fixture entry is an ordinary regular file, so dropping `O_NONBLOCK` or the `S_IFREG` branch leaves the suite green. The failure it guards -- an entry swapped for a FIFO inside the metadata-to-content window -- parks the serving goroutine inside `openat` forever, after the response has started. The Windows twin is pinned by literal constants; POSIX has no equivalent.

- source_spec: `spec-2-2-stream-a-safe-directory-zip.md`
  id: D-077
  summary: Bound traversal depth so descriptor exhaustion cannot land mid-response.
  owner: 3-8-harden-the-directory-stream
  evidence: `walkDirectory` holds one enumeration handle per active level with no cap, and `classifyMetadataError` renders `EMFILE` as `path_unsupported`. Under Story 2.1 that failed during preflight, where it could still choose an HTTP status; under 2.2 the same exhaustion breaks a live download after headers, with a misleading code. The deepest fixture is twelve levels.

- source_spec: `spec-2-2-stream-a-safe-directory-zip.md`
  id: D-078
  summary: Validate a large-entry-count archive by reading it back, and cover the ZIP64 thresholds.
  owner: discharged
  resolution: Story 3.7 native matrix and mutation audit; see D-078 in evidence-3-7-execute-the-native-platform-test-matrix.md.
  evidence: The fifty-thousand-entry archive is streamed to `io.Discard` and never opened, so nothing proves a large archive is still readable. No test approaches 65,535 entries, a 4 GiB entry, or a 4 GiB total, which are the points where `archive/zip` switches to ZIP64 and where a receiver's extractor is most likely to disagree.

- source_spec: `spec-2-2-stream-a-safe-directory-zip.md`
  id: D-079
  summary: Reconcile ZIP entry-name hardening with the download-name sanitizer.
  owner: 3-8-harden-the-directory-stream
  evidence: `sanitizeDownloadName` strips control and format characters, quotes, semicolons, colons, and trailing dots and spaces. `archiveEntryName` refuses only empty, separator, NUL, volume-qualified and dot segments. Both land on a receiver's filesystem, but an entry name may still carry a Windows reserved device name, a trailing dot or space, or a bidi override. The asymmetry is unexplained rather than deliberate.

- source_spec: `spec-2-2-stream-a-safe-directory-zip.md`
  id: D-080
  summary: Make the borrowed content reader safe to touch from another goroutine, or prove it cannot be.
  owner: 3-8-harden-the-directory-stream
  evidence: `borrowedContent.returned` is a plain bool written after `visit` returns, and the platform `Close` implementations nil their file without synchronisation. The port comment promises a retained reader 'reads nothing', but a visitor that hands the reader to another goroutine gets a data race instead of a clean `fs.ErrClosed`. The existing test retains readers only sequentially, so `-race` never observes it.

- source_spec: `spec-2-2-stream-a-safe-directory-zip.md`
  id: D-081
  summary: Decide whether archive entries should carry a file mode.
  owner: 3-8-harden-the-directory-stream
  evidence: `writeArchiveDirectory` sets `fs.ModeDir | 0o755`; `writeArchiveFile` sets no mode at all, so extracted files take whatever default the extractor picks and an executable bit is dropped. No test asserts an extracted mode on either kind.

- source_spec: `spec-2-2-stream-a-safe-directory-zip.md`
  id: D-082
  summary: Pin the selected root's identity across the Prepare-to-WriteTo window, or record why it is not pinned.
  owner: 3-8-harden-the-directory-stream
  evidence: `prepareArchive` pins identity with an `Lstat` but the archive keeps only the path, so `Walk` re-resolves by name. A root replaced between claim and streaming is streamed under the approved download name and root entry. The unsnapshotted policy covers contents changing; it does not obviously cover the root becoming a different object. `os.Lstat` also follows ancestors, so an ancestor swapped for a symlink passes Prepare and is refused only after headers.

- source_spec: `spec-2-2-stream-a-safe-directory-zip.md`
  id: D-083
  summary: Exercise the server handler with a directory payload so the omitted Content-Length is proved end to end.
  owner: discharged
  evidence: The acceptance criterion names response headers and unknown-total progress, but the story verifies only `Size() == (0, false)` at the payload level. No file under `internal/server` changed, so `writeDownloadHeaders` and `newMeter` are never driven by a directory payload and nothing proves the handler actually omits the header for one.

- source_spec: `spec-2-2-stream-a-safe-directory-zip.md`
  id: D-084
  summary: Portable archive names are validated with host-dependent predicates.
  owner: discharged
  resolution: Story 3.7 native matrix and mutation audit; see D-084 in evidence-3-7-execute-the-native-platform-test-matrix.md.
  evidence: `childRelativeName` and `archiveEntryName` both use `filepath.IsAbs` and `filepath.VolumeName`, which are compiled for the sender's platform. On a Linux sender `C:evil.txt` is neither absolute nor volume-qualified, so those branches are dead exactly where the threat -- a Windows receiver extracting the archive -- lives.

- source_spec: `spec-2-2-stream-a-safe-directory-zip.md`
  id: D-085
  summary: The post-open regular-file recheck in emitFile is unreachable.
  owner: accepted
  evidence: `verifyOpened` already runs `rejectUnsupportedInfo`, which admits only regular files and directories, and compares identity, so a file cannot become a directory under the same identity. The follow-up `IsRegular` check therefore cannot fire and no mutation can reach it. Kept as defence in depth rather than deleted, on the same reasoning as the other layered guards in this package; recorded so a later reviewer does not re-derive it as a gap.

- source_spec: `spec-3-1-enforce-one-running-fairdrop-instance.md`
  id: D-086
  summary: Windows may refuse to bring the restored window to the foreground, leaving it behind with a flashing taskbar button.
  owner: 3-9-record-human-release-evidence
  evidence: `WindowShow` calls `SetForegroundWindow`, which Windows refuses from a process that is not the foreground process -- and by the time the first instance handles the second launch, the second process has already exited, so the first is not in the foreground. The window would unminimise and stay behind, taskbar flashing. Only a native second launch can show whether this happens; the `AlwaysOnTop` toggle workaround waits on that observation and on a decision about whether a flash counts as restored.

- source_spec: `spec-3-1-enforce-one-running-fairdrop-instance.md`
  id: D-087
  summary: A second launch arriving while `Shutdown` is blocked on a live transfer's lease is swallowed: the lock and listener stay alive but no window returns.
  owner: discharged
  evidence: `OnShutdown` blocks in `coordinator.Shutdown` (unbounded, see D-032) after the window has closed. The mutex is still held, so a second launch hands off and exits `0`, and `restoreWindow` runs against a window that no longer exists. Bounding the shutdown wait is this story's work; a log line at shutdown entry would at least make the swallowed relaunch diagnosable.

- source_spec: `spec-3-1-enforce-one-running-fairdrop-instance.md`
  id: D-088
  summary: Wails' Windows lock falls through to a full second instance when the mutex exists but cannot be used, and the mutex is session-local.
  owner: 3-10-settle-the-release-blocking-platform-decisions
  evidence: `SetupSingleInstance` treats any `CreateMutex` error other than `ERROR_ALREADY_EXISTS` as "no other instance" (an elevated first instance is the common case), and returns without exiting when `FindWindowW` finds no event window (a tight double-launch), so two coordinators, listeners and beacons can run. Separately the mutex lives in the logon session, so two logged-in Windows users each get an instance. An app-owned backstop lock under `os.UserConfigDir` is the candidate fix; it is a release-platform decision, not this story's.

- source_spec: `spec-3-1-enforce-one-running-fairdrop-instance.md`
  id: D-089
  summary: On macOS a lock file that cannot be opened for any reason but contention makes the launching process exit silently, so FairDrop never opens.
  owner: discharged
  resolution: Story 3.7 native matrix and mutation audit; see D-089 in evidence-3-7-execute-the-native-platform-test-matrix.md.
  evidence: `darwin/single_instance.go` treats every `createLockFile` failure as "another instance holds it", sends the second-instance data, and `os.Exit(0)`s. A read-only or full temp directory therefore makes FairDrop refuse to launch with no message. Needs the native macOS runner to confirm and to decide between a pre-flight check and a documented limit.

- source_spec: `spec-3-2-automate-reproducible-cross-platform-verification.md`
  id: D-090
  summary: Two coordinator waits are unbounded by design, and nothing decides what happens when the port postcondition they rest on is violated.
  owner: discharged
  evidence: Raised by the Edge Case Hunter layer while discharging D-038, and verified against HEAD. `joinDrainer` waits forever on `live.drainerDone`, and `awaitLease` waits forever on the lease; both are deliberate, and the comments say why -- a watchdog would let Cancel report success while a publication was still in flight. The unexamined case is the one the layer named: `ServerPort.Stop` is documented as quiescent on every return, so an adapter that returns an error *and* leaves the event lane open makes Cancel and Shutdown hang and the app unclosable. Either the port's postcondition is enough and the story records that reasoning, or the waits need the documented bound with a coded failure that this story's own charter asks for. `stopServer` currently records a diagnostic and continues either way.

- source_spec: `spec-3-2-automate-reproducible-cross-platform-verification.md`
  id: D-091
  summary: An event lane that closes while the session is STAGED synthesises no terminal outcome, so the sender keeps looking at a QR code for a server that is gone.
  owner: discharged
  evidence: Raised by the Edge Case Hunter layer while discharging D-038, and verified against HEAD. `drain`'s post-loop synthesis calls `acceptTerminal`, which is refused by `drainerMayActLocked` for anything but `stateTransferring`. `TestStagedServerEventsAreDrainedWhileStaged` (`coordinator_stage_test.go:612`) closes the lane at STAGED and asserts only that the drainer exits, so the gap is covered by a passing test that never asks the question. Epic 3's requirement for this story says an event lane closing mid-session synthesises a terminal outcome; STAGED is mid-session.

- source_spec: `spec-3-2-automate-reproducible-cross-platform-verification.md`
  id: D-092
  summary: A transfer failure's original cause is rewritten to fixed public copy and never recorded, so a real failure leaves no internal trail.
  owner: discharged
  evidence: Raised by the Blind Hunter layer while discharging D-038, and verified against HEAD. `acceptTerminal` passes `event.Err` to `terminalPublicError`, which maps it onto the twelve-message registry; the adapter's own error text is then discarded. `outcomes.go`'s three `recordDiagnostic` calls cover an unrecognized event kind, a snapshot-less progress event and a nil stop function -- not the failure itself. Adjacent code does record adapter errors as diagnostics (`stopServer`, `StopBeacon`), and `TestStagedSessionNeverDisclosesTheTokenOrThePath` already pins that diagnostics disclose neither the token nor the path, so the disclosure rule is not the obstacle. Deferred rather than fixed in 3.2 because it is a production change outside that story's stated boundary.

- source_spec: `spec-3-2-automate-reproducible-cross-platform-verification.md`
  id: D-093
  summary: FairDrop's POSIX source adapter has never run, and macOS cannot stage a file or a folder at all.
  owner: discharged
  evidence: Found by this story's first CI runs. `handle_posix.go` (`//go:build linux || darwin`) did not compile until commit `a76d4ce`, so no gate in the project's history had ever type-checked it, let alone run it. With it compiling, run 3 (https://github.com/jaeson-sandbox/FairDrop/actions/runs/34311314866) fails 40-odd tests across `internal/source` and `internal/stream` on `macos-latest` from one root cause: every `Inspect` returns `path_unsupported: selection metadata could not be read`, and every `internal/stream` test that stages a fixture fails behind it. `TestPOSIXInspectUsesSearchOnlyAncestorRights` and `TestPOSIXReopenRefusesPostMetadataSymlinkSubstitution` fail directly, the latter with a raw `ELOOP`. The likely root cause, unconfirmed without a Mac to iterate on: darwin's `nativeMetadataFlags`/`nativeSearchFlags` use `O_EVTONLY`, which is not Linux's `O_PATH` -- an `O_EVTONLY` descriptor is a notification handle and does not grant the relative-lookup rights `openat` needs from a base directory, so the whole parent-descriptor traversal collapses on the first hop. Fixing it means choosing darwin's replacement for the no-read-access metadata handle, which is an architecture decision (AD-level: the metadata open must not grant read access), not a flag swap. Windows is unaffected and green.

- source_spec: `spec-3-2-automate-reproducible-cross-platform-verification.md`
  id: D-094
  summary: On macOS a selection anywhere under /tmp, /var or /etc is refused, because each is a symlink and the traversal refuses link-like components.
  owner: discharged
  resolution: Story 3.7 native matrix and mutation audit; see D-094 in evidence-3-7-execute-the-native-platform-test-matrix.md.
  evidence: Established while discharging D-093. `rejectUnsupportedInfo` refuses a link-like component anywhere in a selection, which `TestInspectRejectsLinksSpecialsAndStopsBeforeLaterEntries` pins as intended behaviour, and macOS ships `/var`, `/tmp` and `/etc` as symlinks to `/private/*`. So a user who picks a file under any of them gets `path_unsupported` rather than a transfer, and inspecting `/` itself always fails because the root's own entries include those symlinks. Normal selections under `/Users/...` are unaffected, which is why the test fixtures were the only thing this broke. Left as-is deliberately: whether the picker should resolve the path before handing it over, or the copy should explain the refusal, is a product decision for the story that owns the native platform matrix, not a change to make while chasing a green build. Note Linux has the same shape wherever `/bin` or `/home` is a symlink.

- source_spec: `spec-3-2-automate-reproducible-cross-platform-verification.md`
  id: D-095
  summary: The POSIX adapter is verified on macOS only; the Linux half of the same file is still compile-checked and never executed.
  owner: discharged
  resolution: Story 3.7 native matrix and mutation audit; see D-095 in evidence-3-7-execute-the-native-platform-test-matrix.md.
  evidence: `handle_posix.go` is `//go:build linux || darwin` and `handle_linux.go` supplies the `O_PATH` flag sets. Story 3.2's workflow runs Windows and macOS only, and the frozen boundary makes a Linux job Ask First -- correctly, since the epic's requirement is that a Linux job never stand in for release proof of a supported platform. But that leaves the Linux branch of a shared file in the same position darwin was in before this story: type-checked by `GOOS=linux go vet` and never run. The `O_PATH` path is the better-established of the two and the fallback seam declines on Linux, so the risk is lower than darwin's was -- and darwin's was assumed low too, right up until the first run failed forty tests. Deciding whether FairDrop wants a Linux job for adapter verification only, clearly not release proof, belongs to the platform-matrix story.

- source_spec: `spec-3-4-bound-every-lifecycle-wait-and-prove-quiescence.md`
  id: D-096
  summary: `network.Manager.StopBeacon` holds the selection gate across its own blocking Shutdown, so one hung mDNS shutdown degrades every later transfer in the process.
  owner: 3-8-harden-the-directory-stream
  evidence: Raised by the Blind Hunter layer reviewing Story 3.4 and verified against HEAD. `internal/network/beacon.go` takes `m.selectionGate` and `m.mu` and releases them by `defer`, after `handle.Shutdown()` returns. Story 3.4 bounded the coordinator's *wait* for `StopBeacon`, which stops the coordinator wedging -- but the adapter itself is unchanged, so a genuinely hung `Shutdown` leaves the gate held forever and every later `GetLocalIP` on the shared `Manager` blocks on it. `acquireSelectionGate` does honour its context, so an explicit Cancel can still unstick a later Stage, but nothing does that automatically. `internal/server.Server.Stop` received exactly this treatment in 3.4 -- detach, release the lock, then wait -- and the network adapter did not. The same fix shape applies. Not done in 3.4 because `internal/network` was outside that story's Code Map and the coordinator-side bound already removed the wedge the story was scoped to remove.

- source_spec: `spec-3-4-bound-every-lifecycle-wait-and-prove-quiescence.md`
  id: D-097
  summary: Cancel and Shutdown honour a caller's context, but the context production hands them can never be cancelled, so the feature is unreachable in the shipped app.
  owner: 3-11-close-the-residual-contract-and-copy-gaps
  evidence: Raised by the Blind Hunter layer reviewing Story 3.4 and verified against the vendored Wails v2.15.0. `app.go`'s `CancelTransfer` and `shutdown` pass their Wails context straight through, which is what D-036 asked for and what the new tests exercise. But Wails builds that context once from `context.Background()` plus `WithValue` and never wraps it in `WithCancel` or `WithTimeout`, so `ctx.Done()` never fires in a running FairDrop: both commands are bounded in production only by the internal `leaseBound`. The same dead context is Stage's only escape from a setup-phase adapter that ignores cancellation -- `Inspect`, `GetLocalIP`, `Server.Start`, `EncodePNG`, `StartBeacon` got no `callBounded` wrap -- so a hung setup call leaves `StageTransfer` unable to return at all. Closing it means the app owning a cancellable context of its own rather than borrowing the runtime's, which is an `app.go` design change rather than a coordinator one.

- source_spec: `spec-3-4-bound-every-lifecycle-wait-and-prove-quiescence.md`
  id: D-098
  summary: Every bound-timeout diagnostic is written to a sink nothing in the running binary ever reads.
  owner: discharged
  evidence: Raised by the Blind Hunter layer reviewing Story 3.4 and verified against HEAD. `recordDiagnostic` writes to the coordinator's `diagnosticSink`, and `docs/fairdrop-contracts.md` leans on "recorded as a diagnostic" as the honesty mechanism for a bound that elapsed. But `app.go` never reads `coordinator.diagnostics`; the `logf` seam is wired only to lifecycle events. So in a shipped build the caller's coded error is the entire trace, and the diagnostic record the contract cites is reachable only from tests through `h.coordinator.diagnostics.snapshot()`. This story's charter is that a lost or refused signal always reaches a visible surface, which is exactly what a diagnostic nobody reads is not.

- source_spec: `spec-3-4-bound-every-lifecycle-wait-and-prove-quiescence.md`
  id: D-099
  summary: The coordinator's bound on ServerPort.Stop equals the server's own teardown bound, so the outer wait can give up on an inner one that was about to succeed.
  owner: 3-8-harden-the-directory-stream
  evidence: Raised independently by the orchestrator and the Blind Hunter layer while reviewing Story 3.4. `adapterCallBound` is 10s in `internal/transfer/coordinator.go` and `teardownBound` is 10s in `internal/server/lifecycle.go`. The outer bound therefore races the inner one rather than outlasting it: the coordinator can report "the transfer server did not confirm it stopped" for a `Stop` that was about to return its own, more specific coded failure naming which wait was outstanding. Both values are now pinned by tests, but to their own literals -- nothing ties them to each other, because `internal/server` imports `internal/transfer` and the reverse import would be a cycle, and both constants are unexported. Fixing it means either exporting them for a root-package pin or giving the coordinator a margin above whatever the server documents. Harmless today in that both answers are honest failures; it costs the more precise message.

- source_spec: `spec-3-4-bound-every-lifecycle-wait-and-prove-quiescence.md`
  id: D-100
  summary: `unwind` returns only the first bound failure, so a second simultaneously-unaccounted resource is invisible to the caller.
  owner: discharged
  evidence: Raised by the Blind Hunter layer reviewing Story 3.4. `releaseAcquired` and `joinDrainerBounded` can each hit their own bound in one `unwind`, and only the first error reaches Cancel or Shutdown's caller; the second exists only in the diagnostic sink, which D-098 records is unread in production. A caller told "the server did not confirm it stopped" has no way to learn the drainer is also unaccounted for. Joining the failures, or reporting a count, is the fix; it pairs naturally with D-098 since both are about what a teardown failure actually tells anyone.

- source_spec: `spec-3-4-bound-every-lifecycle-wait-and-prove-quiescence.md`
  id: D-101
  summary: `callBounded` abandons one goroutine per timed-out adapter call with no cap, so repeated attempts against a wedged device accumulate them.
  owner: 3-8-harden-the-directory-stream
  evidence: Raised by the Blind Hunter layer reviewing Story 3.4. `callBounded` spawns a goroutine per bounded adapter call and abandons it when the bound elapses -- Go offers no way to make a function return, which the code documents honestly. The consequence the code does not address is accumulation: `NetworkPort.StopBeacon` takes no context and, per D-096, can hang forever, so a user retrying Cancel or Stage against the same broken device leaks one goroutine each time with no cap, backoff, or circuit breaker. Bounded in practice by how many times a person retries, and each goroutine is idle rather than spinning, which is why this is recorded rather than fixed in 3.4.

- source_spec: `spec-3-4-bound-every-lifecycle-wait-and-prove-quiescence.md`
  id: D-102
  summary: Only one of the server teardown report's three named waits is ever driven by a test.
  owner: 3-8-harden-the-directory-stream
  evidence: Raised by the verification-gap layer reviewing Story 3.4. `teardownTimeoutError` names the accept loop, a request handler, and a tracked connection independently, and only the handler branch is exercised (`TestStopReturnsACodedFailureWhenAHandlerNeverReturns`, `TestStopBoundsAHandlerStuckInAuthorizeClaim`). `assertQuiescent` verifies the other two on the healthy path, so they are not unverified, but no test isolates a timeout where only the accept loop or only a connection is still outstanding -- so the wording of those two branches is unproven.

- source_spec: `spec-3-5-reconcile-public-error-copy-with-its-states.md`
  id: D-103
  summary: A `Cancel` against a STAGED-but-never-claimed session can render `transfer_failed`'s "the transfer stopped" copy if Story 3.4's bounded teardown wait elapses, even though no transfer ever began.
  owner: 3-11-close-the-residual-contract-and-copy-gaps
  evidence: Found while auditing every phase-before-a-transfer-began producer for Story 3.5. `internal/transfer/lifecycle.go`'s `retire` (called by both `Cancel` and `Shutdown`) returns `unwind`'s first bound failure directly as its own error (`return unwindErr`). `unwind`'s bound failures are coded `ErrTransferFailed` (`coordinator.go:656,727,743`, Story 3.4). A `Cancel` issued against a session that reached STAGED but never had `AuthorizeClaim` commit it -- no `transfer-started` was ever published, no byte was ever sent -- that then hits one of those *rare, adapter-misbehavior-only* bounds while stopping the server/beacon or joining the drainer would show the user "The transfer stopped before FairDrop finished sending", the same false-interruption shape Story 3.5's seven named states fixed. Not fixed by that story because it is not one of its seven named states and introducing a message for it is an Ask First change the reviewer has not seen; recorded here instead. `busy`'s revised copy (Story 3.5) does not cover it either -- this is a `Cancel` failing outright, not a `Stage` refusal. See `evidence-3-5-reconcile-public-error-copy-with-its-states.md`'s audit for the full trace.

- source_spec: `spec-3-5-reconcile-public-error-copy-with-its-states.md`
  id: D-104
  summary: `Cancel` on a nil coordinator, and `CopyToClipboard` failures, still report copy that describes a transfer that stopped midway.
  owner: 3-11-close-the-residual-contract-and-copy-gaps
  evidence: Raised by the adversarial layer reviewing Story 3.5 and verified against HEAD. `lifecycle.go`'s `Cancel` answers a nil receiver with `ErrTransferFailed` and a comment that is now stale -- it says Stage and AuthorizeClaim answer a missing coordinator the same way, which stopped being true when 3.5 routed them through `ready()` and the construction guard. `app.go`'s three `CopyToClipboard` sites do the same for a clipboard write failure, which is reachable only from a staged session so it is not a "no transfer began" miscode, but "the transfer stopped before FairDrop finished sending" still misdescribes a clipboard that would not accept text. Neither was fixed in 3.5: `setup_failed`'s copy ("couldn't prepare that item... Choose it again") does not fit a cancel or a clipboard write either, so both want wording the reviewer has not confirmed, and inventing it unreviewed is what the story's Ask First forbids.

- source_spec: `spec-3-5-reconcile-public-error-copy-with-its-states.md`
  id: D-105
  summary: A mis-coded error from the source port passes the Prepare boundary untouched, because the wrap only catches uncoded ones.
  owner: 3-8-harden-the-directory-stream
  evidence: Raised by the adversarial layer reviewing Story 3.5. `wrapUncodedSourceError` enforces the port's postcondition by wrapping errors that carry no code, and deliberately passes coded ones through. But `internal/source`'s own `walkDirectory` returns a fully coded `ErrTransferFailed` for "selection enumeration exceeded its fixed batch" and "selection logical size is invalid", and both are reachable from `Prepare`'s re-`Inspect` before any header is written. They are coded, so the wrap does nothing, and the user sees the pre-transfer copy this story exists to eliminate. The fix is not a broader wrap -- remapping every coded error at a boundary would destroy the specific codes the contract promises -- but for `internal/source` to stop using `transfer_failed` for two conditions that are neither transfers nor failures of one.

- source_spec: `spec-3-5-reconcile-public-error-copy-with-its-states.md`
  id: D-106
  summary: A malformed Stage acknowledgement tells the user nothing was sent while the backend may still hold a live staged session.
  owner: 3-11-close-the-residual-contract-and-copy-gaps
  evidence: Raised by the adversarial layer reviewing Story 3.5. When `StageTransfer` resolves but `parseFileMetadata` fails, the backend has already committed a STAGED session with a listener and a capability URL. `useTransfer.ts` makes a best-effort `CancelTransfer()` and swallows its own failure in a bare `catch {}`, then reports `setup_failed` -- "Nothing was sent. Choose it again." If that cleanup call failed, nothing was sent but plenty was *started*, and the user's next Stage is refused `busy` for a session they were told did not exist. The copy is right about bytes and wrong about state; making it right means either surfacing the cleanup failure or guaranteeing the cleanup, which is this story's charter rather than 3.5's.

- source_spec: `spec-3-5-reconcile-public-error-copy-with-its-states.md`
  id: D-107
  summary: A wiring regression in compose would crash FairDrop before any window exists, invisibly in a release build.
  owner: 3-10-settle-the-release-blocking-platform-decisions
  evidence: Raised by the adversarial layer reviewing Story 3.5. `NewCoordinator` now panics when a port is nil, which is what makes `ready()`'s nil-port branch unreachable and was the right trade for D-029. The only production caller supplies all five, so it cannot fire today. What is unexamined is the failure shape if it ever does: `compose` runs in `main()` before `wails.Run`, no `recover` covers that path, and a release Wails build has no console -- so the process would vanish with no window, no dialog, and no visible message. A panic is the right answer for a wiring defect; whether it should be preceded by something a user can see belongs with the release-platform decisions.

- source_spec: `spec-3-7-execute-the-native-platform-test-matrix.md`
  id: D-110
  summary: Natural completion can close the socket before net/http finishes the response, producing HTTP 200 with unexpected EOF for files and folders.
  owner: discharged
  resolution: Story 3.7 native matrix and mutation audit; see D-110 in evidence-3-7-execute-the-native-platform-test-matrix.md.
  resolution_plan: Owner approved bringing this fix into Story 3.7 on 2026-09-11. The original routing/evidence below is historical; no test failure is waived.
  evidence: Story 3.7's App/coordinator/real-server HTTP matrix failed in the full Windows race run, then reproduced 54 incomplete downloads in 240 attempts (30 of 40 iterations; 48 folders and 6 files), all with unexpected_eof=true. handler.go publishes ServerComplete before ServeHTTP returns; coordinator terminal teardown invokes Server.Stop and http.Server.Close before net/http's finishRequest has necessarily flushed buffered bytes and final chunk framing. Existing server tests finish reading before Stop, excluding the race. Full failure logs and exact code-path reasoning are in evidence-3-7-execute-the-native-platform-test-matrix.md. Routed to 3.8 because it owns server/stream lifecycle hardening outside 3.7's approved Code Map; this remains a blocker for 3.7's matrix, not an accepted failure. Fix requires deterministic response-finalization coverage while retaining force-close cancellation/failure semantics. No connection to the historical phone failure is proven.

- source_spec: `spec-3-7-execute-the-native-platform-test-matrix.md`
  id: D-111
  summary: Generic busy recovery copy suggests cancellation and retry even when an uninterruptible filesystem lookup requires waiting or restarting FairDrop.
  owner: 3-11-close-the-residual-contract-and-copy-gaps
  evidence: Second independent review reproduced this through the bounded selection resolver: Cancel abandons the wait, but cannot stop the OS call; retries correctly refuse busy until it returns. The generic copy predates Story 3.7, as does the underlying limit: baseline Coordinator.Stage calls SourcePort.Inspect synchronously, and Cancel can exhaust its lease wait without an uninterruptible Inspect returning. The new tests make the recovery-copy limitation explicit. New public codes/strings remain Story 3.11 under 3.7's Ask First boundary; no authority to bring that wording forward was received. docs/release-policy.md supplies wait/restart guidance meanwhile. Story 3.11 must add applicable recovery wording through the UX registry, contract, Go and TypeScript mirrors together, preserving bounded outstanding work and testing the visible state.
