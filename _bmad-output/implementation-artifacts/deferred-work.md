# Deferred Work

Real findings surfaced during review that are not the current story's problem.
Append-only. Each entry names the spec that surfaced it and the `owner:` that will
resolve it -- a story key from `sprint-status.yaml`, or `discharged` (already
resolved by a later story) or `accepted` (reviewed and deliberately left alone).
`TestEveryDeferredEntryHasALiveOwner` in `main_test.go` fails if an entry has no
owner or names a story that does not exist, so a finding cannot quietly stop
being anyone's problem.

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
  summary: Backend contracts assume a single staged path while the frontend accepts multi-file drops.
  owner: discharged
  evidence: `TransferServer.Start(filePath string, ...)` and `Streamer.StreamFile`/`StreamZip` each take one path, but the I/O matrix requires a three-file drop to be accepted and `App.tsx` stores `string[]`. Phase 5 (`StageTransfer`) must decide: zip a multi-selection, stage only the first, or reject.

- source_spec: `spec-phase-1-wails-scaffold.md`
  summary: `TransferStats.Percent` has no defined behavior when `TotalBytes` is 0 (unknown-size zip stream).
  owner: discharged
  evidence: Spec §6 Module B says Content-Length is omitted when streaming a directory, so TotalBytes is genuinely unknown for zips. Dividing by it yields NaN/Inf, which `encoding/json` refuses to marshal — the progress event would fail to emit. Phase 4 must define the unknown-total case.

- source_spec: `spec-phase-1-wails-scaffold.md`
  summary: `Streamer` has no defined way to signal a mid-stream failure after headers are written.
  owner: discharged
  evidence: Once bytes are on the wire a returned error cannot change the status code, so the receiver saves a truncated file that looks complete. Phase 3 should specify `panic(http.ErrAbortHandler)` (or equivalent) to break the connection so the client sees a failed download.

- source_spec: `spec-phase-1-wails-scaffold.md`
  summary: `TransferServer.Stop` has no documented idempotency contract.
  owner: discharged
  evidence: Spec §3 resets to IDLE after DONE/ERROR and §7 allows user cancellation, so Stop can plausibly be reached twice or before Start. Undefined semantics invite a double-close panic or a leaked listener in Phase 4.

- source_spec: `spec-phase-1-wails-scaffold.md`
  summary: No CI runs the verification commands, so the verified state decays from the next commit.
  owner: 3-2-automate-reproducible-cross-platform-verification
  evidence: No `.github/`, Makefile, or task runner exists. The spec's six verification commands are hand-run only. A clean-clone CI job would have caught the `go:embed` gap on the very first push.

- source_spec: `spec-phase-1-wails-scaffold.md`
  summary: Drag-and-drop is the only input path; there is no keyboard or pointer-free way to stage a file.
  owner: discharged
  evidence: The zone is a bare `div` with no role, no `aria-label`, no "Browse…" fallback, and the results list has no `aria-live` region so new paths are never announced. Phase 6 builds the real DropZone and should add a file-picker fallback.

- source_spec: `spec-phase-1-wails-scaffold.md`
  summary: Path edge cases are untested — spaces, non-ASCII, >260 chars, UNC shares, symlinks, and zero-path drops.
  owner: 3-2-automate-reproducible-cross-platform-verification
  evidence: Every path in the matrix and tests is a simple `C:\x\...`. Windows MAX_PATH and UNC handling are real hazards for a file-transfer tool, and they become testable in Phases 2-4 where the Go side actually opens the paths.

- source_spec: `spec-phase-1-wails-scaffold.md`
  summary: `frontend/tsconfig.json` uses legacy `moduleResolution: "Node"` against a toolchain that publishes subpath exports.
  owner: discharged
  evidence: Vite 7, Vitest 4, and `@tailwindcss/vite` expose entry points such as `vitest/config` through `exports` maps that node10 resolution cannot read. It type-checks today, but `"bundler"` is the resolution these tools expect and the current setting is a latent trap.

- source_spec: `spec-phase-1-wails-scaffold.md`
  summary: `npm ci --omit=dev` would break `tsc` because test files are inside the build's type-check scope.
  owner: 3-2-automate-reproducible-cross-platform-verification
  evidence: `tsconfig.json` includes all of `src`, which now contains `App.test.tsx` importing `vitest` and `@testing-library/react` (both devDependencies). Not currently reachable — `wails.json` runs plain `npm install` — but a production-flavored CI install would fail the build.

- source_spec: `spec-1-1-validate-and-describe-one-file-selection.md`
  summary: Claim-time source revalidation and payload opening should pin filesystem identities so an ancestor replacement cannot exploit the metadata snapshot's TOCTOU window.
  owner: 2-1-validate-and-stage-one-directory
  evidence: Story 1.1 now `Lstat`s every syntactic ancestor, rejects native Windows reparse attributes, and rechecks cancellation, but separate path-based metadata calls cannot atomically prevent a local rename between checks. The binding contract already assigns later defenses to claim-time re-`Lstat` and descriptor-first payload opening; their stories must preserve and verify that layering.

- source_spec: `spec-1-3-prepare-and-stream-a-regular-file-safely.md`
  summary: A post-header stream failure must break the receiver's connection, and only the HTTP handler can do that.
  owner: discharged
  evidence: `PreparedPayload.WriteTo` reports a mid-stream read, cancellation, or destination failure as a coded `transfer_failed`/`cancelled` error, but `Content-Length` is already on the wire by then, so a plain handler return leaves the receiver holding a truncated file that looks complete. The payload port owns no connection, so Story 1.4 must turn a non-nil `WriteTo` error into a killed connection (`panic(http.ErrAbortHandler)` or equivalent). This supersedes the Phase 1 entry that assigned the same finding to Phase 3.

- source_spec: `spec-1-3-prepare-and-stream-a-regular-file-safely.md`
  summary: `transfer_failed`'s fixed public copy misdescribes a deadline that expires during Prepare, before any byte is sent.
  owner: 3-5-reconcile-public-error-copy-with-its-states
  evidence: Story 1.3 deliberately separates a deadline from a user cancel, but both Prepare-time and stream-time deadlines map to `transfer_failed`, whose registry string is "The transfer stopped before FairDrop finished sending." Prepare runs before headers, so nothing was being sent. The copy registry is fixed by the UX contract, so changing this is a UX decision (EXPERIENCE.md), not an adapter one.

- source_spec: `spec-1-3-prepare-and-stream-a-regular-file-safely.md`
  summary: The claim-time re-`Lstat` does not re-apply Story 1.1's reparse-point check, so a junction created inside the validate-to-open window surfaces as `source_changed` rather than `path_unsupported`.
  owner: 2-1-validate-and-stage-one-directory
  evidence: `Payloads.lstatPath` is a bare `os.Lstat`. A reparse point created between `Inspect` and that `Lstat` is caught only incidentally, by `os.SameFile` failing. The transfer is still refused and no wrong bytes stream, so this is a code-accuracy issue rather than a safety hole, but the frozen matrix's link-like row promises `path_unsupported`.

- source_spec: `spec-1-3-prepare-and-stream-a-regular-file-safely.md`
  summary: `WriteTo`'s once-only CAS is never exercised concurrently, so the race suite never visits the one place two callers can collide.
  owner: 3-2-automate-reproducible-cross-platform-verification
  evidence: `TestCloseIsSafeWhenCalledConcurrently` fires eight goroutines at `Close`, but `streamed.CompareAndSwap` is only driven sequentially by `TestWriteToRefusesASecondCall`. The contract says the server never calls `WriteTo` twice, so this is defense-in-depth coverage rather than a live defect.

- source_spec: `spec-1-3-prepare-and-stream-a-regular-file-safely.md`
  summary: `Prepare` returns a `SourcePort` error verbatim, so the "every Prepare failure is coded" postcondition rests on the adapter rather than being enforced at the boundary.
  owner: 3-5-reconcile-public-error-copy-with-its-states
  evidence: `internal/source` complies today, but `SourcePort` is an interface and nothing checks. An uncoded error would only be flattened to `transfer_failed` at the UI boundary, losing the specific code. Enforcing it means wrapping unrecognized errors at the port call, which touches the error-code mapping the spec puts behind Ask First.

- source_spec: `spec-1-3-prepare-and-stream-a-regular-file-safely.md`
  summary: `internal/stream/archiver.go` no longer archives anything, and no linter backs the `//nolint:staticcheck` directives in its tests.
  owner: 2-2-stream-a-safe-directory-zip
  evidence: `StreamZip` and the zip logic are gone; the file now holds the single-file payload adapter, and Epic 2 will reintroduce directories behind the same port. Renaming to `payload.go` is a Code Map decision for whoever opens Epic 2. Separately, Verification runs build, vet, test, race, gofmt and greps but no staticcheck, so those directives are unenforced decoration.

- source_spec: `spec-1-4-serve-a-one-shot-capability-download.md`
  summary: `Stop` can block indefinitely while holding the server mutex, which also deadlocks a later `Start`.
  owner: 3-4-bound-every-lifecycle-wait-and-prove-quiescence
  evidence: `<-r.serveDone`, `r.handlers.Wait()`, and `r.awaitConnections()` have no deadline. A payload parked on the destination is covered, because `http.Server.Close` breaks it, but a `WriteTo` blocked on a slow source read that ignores its context hangs `Stop` forever, and `s.mu` is held throughout. A watchdog changes the force-closing semantics the contract states, so this needs a design decision rather than a patch.

- source_spec: `spec-1-4-serve-a-one-shot-capability-download.md`
  summary: CORS is configured for the 200 only, and the receiver page cannot read the filename it was encoded to carry.
  owner: 2-2-stream-a-safe-directory-zip
  evidence: `Access-Control-Allow-Origin: *` is set in `writeDownloadHeaders` but not by `writeStatus`, so a cross-origin receiver sees an opaque failure instead of 404/410/423. There is no `Access-Control-Expose-Headers: Content-Disposition`, so that page cannot read the name, and no `Accept-Ranges: none`, so a download manager may attempt a range retry against a consumed capability. The frozen matrix fixes the exact header set, so adding any of these is an Ask First change.

- source_spec: `spec-1-4-serve-a-one-shot-capability-download.md`
  summary: `AuthorizeClaim` is trusted to return, so a coordinator that blocks in it hangs `Stop` and loses quiescence.
  owner: 3-4-bound-every-lifecycle-wait-and-prove-quiescence
  evidence: The handler calls it synchronously with `r.ctx` and waits. The contract makes it the coordinator's own synchronous handshake, so bounding it here would duplicate a timeout the coordinator should own -- but nothing on the server side currently survives a coordinator that never returns. Worth settling when Story 1.5 implements the authorizer.

- source_spec: `spec-1-4-serve-a-one-shot-capability-download.md`
  summary: Repeated `Stop` discards the first call's cleanup diagnostic, and `teardownOnce`/`teardownDone` guard a path with one structurally unreachable entrant.
  owner: 3-4-bound-every-lifecycle-wait-and-prove-quiescence
  evidence: `teardown()` is called only after `Stop` takes `s.mu` and clears `s.active`, so a second entrant cannot occur; the `sync.Once` plus channel join therefore protects nothing, while causing later `Stop` calls to return `nil` rather than replaying the diagnostic the contract says they may report.

- source_spec: `spec-1-4-serve-a-one-shot-capability-download.md`
  summary: `ErrorLog` discards genuine handler panics along with the request diagnostics it is there to silence.
  owner: 3-4-bound-every-lifecycle-wait-and-prove-quiescence
  evidence: `log.New(io.Discard, "", 0)` blinds every `net/http` report from this server, including a real panic that is not `http.ErrAbortHandler`. A redacting writer that strips the request line would keep the disclosure property without making a production fault in this package invisible.

- source_spec: `spec-1-4-serve-a-one-shot-capability-download.md`
  summary: Restart after `Stop` is possible but unspecified and untested.
  owner: 3-4-bound-every-lifecycle-wait-and-prove-quiescence
  evidence: `Stop` clears `s.active`, so `Start` -> `Stop` -> `Start` succeeds and builds a fresh run. The type comment says "one listener, one capability token, one authorized download, then nothing", which reads as forbidding it. Whether a server instance is reusable belongs in the contract, since the coordinator will decide whether to construct one per session.

- source_spec: `spec-1-5-stage-and-authorize-a-transfer-transactionally.md`
  summary: Story 1.5 was split so the QR adapter lands first as Story 1.5a; nothing is deferred, only sequenced.
  owner: discharged
  evidence: The epic puts QR encoding inside the Stage sequence, but `internal/qr` is independently mergeable and unrelated to the coordinator's concurrency work. Building it first lets the coordinator be specified and tested against a real `QRPort` rather than a stub. Both specs belong to sprint key `1-5-stage-and-authorize-a-transfer-transactionally`, which is done only when both are.

- source_spec: `spec-1-5-stage-and-authorize-a-transfer-transactionally.md`
  summary: `AuthorizeClaim` is now deadlock-free by construction, but it is still not time-bounded, and the one unlocked call inside it is `StopBeacon`.
  owner: 3-4-bound-every-lifecycle-wait-and-prove-quiescence
  evidence: This closes half of the Story 1.4 entry above. The handshake never blocks on the operation lease (it takes it with a non-blocking try and answers `cancelled` when a teardown owns it) and never holds the state mutex across a call, so `ServerPort.Stop` can always make progress against a handler parked in it. What remains is the `NetworkPort` side: the claim calls `StopBeacon` synchronously while holding the lease, so an mDNS shutdown that never returns hangs the serving handler and therefore `Stop`. Bounding it needs the same design decision as the unbounded `Stop` entry above, not a patch here.

- source_spec: `spec-1-5-stage-and-authorize-a-transfer-transactionally.md`
  summary: A CSPRNG failure during Stage has no stable code of its own, so it borrows `transfer_failed`, whose fixed copy describes an interrupted transfer that never began.
  owner: 3-5-reconcile-public-error-copy-with-its-states
  evidence: `Coordinator.newIdentity` maps an exhausted entropy source to `transfer_failed` because the code table has no entry for it and the contract sends everything unrecognized to that fallback. The registry string is "The transfer stopped before FairDrop finished sending," but nothing was staged, advertised, or sent. Same shape as the Story 1.3 entry about a Prepare-time deadline: the copy registry is fixed by the UX contract, so a new code or new copy is a UX decision (EXPERIENCE.md), not a coordinator one. The failure is unreachable in practice on a healthy host.

- source_spec: `spec-1-5-stage-and-authorize-a-transfer-transactionally.md`
  summary: A committed session has no production-reachable teardown until Story 1.6 lands.
  owner: discharged
  evidence: After a successful Stage, the only code that cancels the session context, stops the server and beacon, or joins the drainer is `failStage`. `cancelSession` and `beginClosing` are unexported and called only from tests, so a staged listener, mDNS registration, session context, and drainer goroutine currently outlive the coordinator with nothing able to release them. Story 1.6's Cancel and Shutdown are the fix; recording it because the code alone reads as an omission rather than a sequencing decision.

- source_spec: `spec-1-5-stage-and-authorize-a-transfer-transactionally.md`
  summary: `unwind` waits on the drainer unbounded while holding the operation lease.
  owner: 3-4-bound-every-lifecycle-wait-and-prove-quiescence
  evidence: `<-live.drainerDone` depends entirely on `ServerPort.Stop` closing the event lane. A Stop that returns without closing it wedges the coordinator in permanent `busy` with no timeout and no diagnostic. This is the second unbounded wait in the file -- the deferred entry about `StopBeacon` inside `AuthorizeClaim` names the first -- and no test drives a Stop that leaves the lane open, because the fake always closes it.

- source_spec: `spec-1-5-stage-and-authorize-a-transfer-transactionally.md`
  summary: A claim that loses its post-`StopBeacon` revalidation leaves the state at CLAIMING with no public exit.
  owner: discharged
  evidence: The contract gives Cancel-from-CLAIMING to Story 1.6, so this is the intended division of labour rather than a defect. Until 1.6 lands, though, a lost revalidation wedges the coordinator: every later Stage answers `busy` and every claim answers `cancelled`. Worth a deliberate test in 1.6 rather than being discovered there.

- source_spec: `spec-1-5-stage-and-authorize-a-transfer-transactionally.md`
  summary: `AuthorizeClaim` can return `transfer_failed`, which the contract's claim-authorization row does not list.
  owner: 3-5-reconcile-public-error-copy-with-its-states
  evidence: `ready()` yields `ErrTransferFailed` when a port is missing, but `docs/fairdrop-contracts.md` says claim authorization returns `cancelled` or `shutting_down` only. A missing port is a wiring defect rather than a runtime outcome, so the honest fix may be to make it unrepresentable at construction instead of widening the contract. Related: `Stage(nil ctx)` is `transfer_failed` while `AuthorizeClaim(nil ctx)` is `cancelled`, for one class of programmer error.

- source_spec: `spec-1-5-stage-and-authorize-a-transfer-transactionally.md`
  summary: `NetworkPort` does not document that `StartBeacon` requires a prior successful `GetLocalIP`.
  owner: 3-4-bound-every-lifecycle-wait-and-prove-quiescence
  evidence: `internal/network` enforces that ordering and answers `beacon_warning` without it, but the port doc states no such precondition and the coordinator's fake accepts `StartBeacon` at any time. Swapping the coordinator's address and beacon steps would keep every coordinator test green and fail in production. The ordering belongs on the port, and the fake should assert it.

- source_spec: `spec-1-5-stage-and-authorize-a-transfer-transactionally.md`
  summary: `diagnosticSink` silently drops entries past 32 with no marker.
  owner: 3-4-bound-every-lifecycle-wait-and-prove-quiescence
  evidence: A truncated sink is indistinguishable from a complete one in a structure whose stated purpose is to be inspected, and the policy drops newest rather than oldest. Nothing outside the package reads it until Story 1.6 exposes it, which is the moment to settle both.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  summary: Every quiescence wait in the coordinator is unbounded, and this story added two more.
  owner: 3-4-bound-every-lifecycle-wait-and-prove-quiescence
  evidence: Restates the Story 1.5 entry about `unwind`. `Cancel` and `Shutdown` wait on the operation lease, then on `<-live.drainerDone`, then inside `releaseAcquired` on `ServerPort.Stop` and `NetworkPort.StopBeacon`. Each rests on a port postcondition -- Stop is quiescent on every return, StopBeacon guarantees no advertisement remains -- so an adapter that violates one wedges the command with no timeout and no diagnostic. The spec puts a watchdog behind Ask First deliberately: bounding these would let Cancel report success while a listener was still live. The decision belongs with the `internal/server` entry about `Stop` blocking on a source read that ignores its context, which is the one concrete way this can happen today.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  summary: `session.terminal` is redundant with the state check that follows it, and no mutation can distinguish them.
  owner: accepted
  evidence: `drainerMayActLocked` refuses an event when `live.terminal` is set *and* when the state is no longer TRANSFERRING. Acceptance sets the flag and the settled state on the same goroutine with no other entry point in between -- `acceptTerminal` and `forwardProgress` are both called only from `drain` -- so deleting the flag leaves every test green (verified by mutation). It is kept because the spec names it and because it makes exactly-once acceptance independent of where the state transition lands, but it is defense in depth, not a tested guarantee. Either delete it or move the settled-state transition into the acceptance critical section and let the flag be the only record.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  summary: A blocking `Observer.Publish` now stalls every lifecycle command, not just the event lane.
  owner: 3-4-bound-every-lifecycle-wait-and-prove-quiescence
  evidence: Publication is lease-owned by design, so the coordinator calls `Publish` while holding the operation lease. A Wails observer that blocks -- an emit into a frontend that is not draining, say -- therefore blocks the next `Cancel` or `Shutdown` for as long as it blocks, because those wait for the lease. The contract already calls `Publish` a synchronous FIFO handoff, so this is a constraint on the Story 1.7 adapter rather than a coordinator defect: whatever implements `Observer` must return promptly and must never call back into the coordinator.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  summary: A `ServerComplete` carrying no snapshot publishes a zero-value one, and no contract row covers that shape.
  owner: 3-4-bound-every-lifecycle-wait-and-prove-quiescence
  evidence: The payload table makes `progress` required on `transfer-complete`, and the port says Complete always carries the authoritative snapshot, so a nil there is a port defect with no defined outcome. `acceptTerminal` reports the unknown-total zero snapshot rather than downgrading a success the receiver actually got, and publishes no separate final-progress event. The alternative -- treating it as `transfer_failed` -- would lie about a transfer that completed. Worth a contract sentence either way, since the current behaviour is a coordinator choice rather than a stated rule.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  summary: `Cancel` and `Shutdown` take no context, so a Wails command cannot abandon one.
  owner: 3-4-bound-every-lifecycle-wait-and-prove-quiescence
  evidence: Both return only when the resources they name are quiescent, which is the contract's requirement, so a caller-supplied context would be a parameter they must ignore. Story 1.7 wires `App.CancelTransfer` and the Wails shutdown hook to them: the hook must be prepared to block for as long as the adapters take, and `App.shutdown` is the only place that may call `Shutdown`.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  summary: `armReset`'s window between creating the reset timer and re-checking that the session survived is guarded but unproven, and can leave one timer armed and unstopped.
  owner: 3-4-bound-every-lifecycle-wait-and-prove-quiescence
  evidence: Found by mutation: replacing the second `resetIsDueLocked` check with an unconditional `live.stopReset = stop` leaves every test green. The window is real but narrow. `retire` reads `stopReset` before it calls `joinDrainer`, and the drainer is the goroutine running `armReset`, so a Cancel that arrives just after the terminal outcome reads a nil `stopReset`, then blocks until `armReset` finishes -- by which point `armReset` has seen the session still installed and armed a timer nobody will ever stop. That timer fires three seconds later, revalidates, finds the session cleared and publishes nothing, so the consequence is one leaked pending callback rather than a second reset. `fakeTimer` already exposes `armed()` and `stops()`, so the assertion exists; what is missing is a deterministic way to hold the drainer inside `armReset` while a Cancel runs.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  summary: The three step-04 review layers never ran for this story, so its only adversarial coverage is self-review plus mutation.
  owner: 3-2-automate-reproducible-cross-platform-verification
  evidence: Blind Hunter, Edge Case Hunter and Verification Gap were all launched together and all three terminated on an Anthropic session rate limit (HTTP 429) before returning findings. Their exact child prompts are preserved in `review-layer-prompts-1-6.md` so they can be run in a separate session, ideally on a different model. Twenty-four mutations stood in for them and found three real verification gaps, which are fixed, but mutation only tests guarantees somebody already thought to encode -- it cannot find a missing requirement, which is precisely what the Blind Hunter layer is for.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  summary: `sanitizeProgress` distrusts NaN but trusts the known/unknown total invariant it sits next to.
  owner: 3-4-bound-every-lifecycle-wait-and-prove-quiescence
  evidence: The function's own comment says this is "the boundary that cannot afford to trust" the producing adapter, and it clamps `Percent` and `SpeedBytesPerSec` accordingly. It does not enforce the rest of the contract's progress rules: a snapshot with `TotalKnown=false` and a non-zero `Percent`, or a negative `BytesSent`, passes through unchanged, though the contract fixes unknown totals at `TotalBytes=0, Percent=0`. `internal/server`'s meter complies today, so this is defense-in-depth rather than a live defect, but the asymmetry means the comment overstates what the function does.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  summary: The two guards that refuse a drainer event on a settled session mask each other, so neither can be verified on its own.
  owner: accepted
  evidence: Completes the entry above about `session.terminal`. Mutation runs both ways: deleting the `live.terminal` check leaves the suite green because the state check refuses the event, and widening the state check to accept DONE and ERROR also leaves it green because `live.terminal` refuses it. A third defence covers the same window -- the drainer's non-blocking lease acquisition fails while the terminal path still owns the lease. The behaviour is correct and triply defended; what cannot be done today is prove any single one of them is doing the work, which is why `TestASecondTerminalEventIsDiscarded` should not be read as pinning the state check.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  summary: Two hardening steps inside `fireReset` are correct but cannot be made to fail deterministically without a seam that does not exist.
  owner: accepted
  evidence: Mutation survivors after review round 2. Moving `live.stop()` back after `releaseLease`, and deleting the `closing` re-check that follows `joinDrainer`, both leave the suite green. Each needs a competitor parked at one instant -- a Cancel blocked in `awaitLease` that inspects the session context the moment the lease is handed back, and a Shutdown that raises `closing` while the reset is inside `joinDrainer`. The fakes can park a goroutine inside an adapter call but not between two statements of the coordinator's own, so forcing either window needs a test-only hook in `fireReset`. Both changes are kept because they close real windows the sibling paths already close; neither is a tested guarantee.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  summary: An event lane that closes while the session is STAGED or CLAIMING leaves the coordinator holding a dead session with no synthesized outcome.
  owner: 3-4-bound-every-lifecycle-wait-and-prove-quiescence
  evidence: Raised by the edge-case layer, which proposed widening `acceptTerminal` to accept those states. That fix would break the event grammar: publishing `transfer-error` from STAGED means an error with no preceding `started`, and the contract's grammars all begin with `started` for anything after Stage acknowledgement. It is also unreachable today -- only `ServerPort.Stop` closes the lane, and every caller of Stop is a teardown that drives to IDLE -- and the user can still recover with Cancel. What is missing is a defined grammar for a server that dies under a staged-but-unclaimed session, which is a contract question rather than a coordinator one.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  summary: A panicking `Observer.Publish` permanently wedges the coordinator, because no lease release is deferred.
  owner: 3-4-bound-every-lifecycle-wait-and-prove-quiescence
  evidence: Raised by the edge-case layer. `AuthorizeClaim` runs on the serving goroutine, where `net/http` recovers a handler panic; the `c.publish(event)` before `c.releaseLease()` is not deferred, so a panicking observer leaves the lease held forever and every later `Cancel` or `Shutdown` blocks in `awaitLease` with no timeout. The drainer sites have the same shape but no recovery at all, so there the process dies instead. The root shape predates this story (Story 1.5 wrote the AuthorizeClaim path), and converting five lease sites to deferred release during triage risks a double release, which panics. Worth a focused pass together with the `Observer` constraint already recorded above.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  summary: `Stage` during the three-second terminal lease is refused with `busy`, whose fixed copy tells the user to finish or cancel a transfer that already ended.
  owner: 3-5-reconcile-public-error-copy-with-its-states
  evidence: Now covered by `TestStageIsRefusedDuringTheTerminalLease`, so the refusal itself is pinned; what is unresolved is the copy. `busy` renders as "Finish or cancel the current transfer before choosing another item." Nothing is in progress and there is nothing to finish -- the transfer completed and the coordinator is holding its outcome on screen for three seconds. The copy registry is fixed by the UX contract, so a new code or new copy is an EXPERIENCE.md decision, the same shape as the Story 1.3 and 1.5 entries about `transfer_failed` describing a transfer that never began.

- source_spec: `spec-1-6-complete-cancel-and-reset-the-transfer-lifecycle.md`
  summary: Four smaller test-quality findings from the review layers are recorded but not yet fixed.
  owner: 3-2-automate-reproducible-cross-platform-verification
  evidence: `TestServerFailureCodedCancelledIsPublishedAsATransferFailure` asserts the published code is `transfer_failed` and then asserts the message is not the cancellation copy, but `PublicErrorOf` sources the message from the code, so the second assertion cannot fail independently -- it is now redundant with `TestATerminalFailureOnlyPublishesCodesThatDescribeIt`, which pins the whole table. `TestLaneClosureDuringATeardownIsSilent` asserts `got == stateError` rather than pinning the state to TRANSFERRING, never checks the `server.Stop` count, and ends with a dangling `_ = metadata`. `TestProgressIsRefusedOutsideAMatchingTransfer`'s STAGED case emits two snapshots but the shared assertion only looks for the second, so forwarding the first pre-claim snapshot would pass. And no test emits events on a second session, so `seq` restarting at 1 with a new session id -- load-bearing for the frontend's discard rule -- is unproven; `assertEventGrammar` would need to filter per session to check it.

- source_spec: `spec-1-7-expose-safe-transfer-commands-through-wails.md`
  summary: The webview can deliver forged lifecycle events to its own listeners, and only the Story 1.8 reducer can defend against it.
  owner: discharged
  evidence: Wails' desktop runtime calls `notifyListeners(payload)` inside `EventsEmit` before `window.WailsInvoke('EE'...)`, so a script in the window reaches every `EventsOn` subscriber without Go being involved. Removing the bound `Publish` command closed the route through this process; it cannot close that one, and no backend change can. The contract already specifies the defence -- initialize `(sessionId, lastSeq)` only from a successful Stage result and ignore events carrying another session or a seq at or below the last one -- so Story 1.8 must implement it as a security property rather than as tidiness, and Story 1.10's recovery contract should say what a rejected event does.

- source_spec: `spec-1-7-expose-safe-transfer-commands-through-wails.md`
  summary: `errNotComposed` and the pre-startup dialog refusal render as "The transfer stopped before FairDrop finished sending", which describes something that never happened.
  owner: 3-5-reconcile-public-error-copy-with-its-states
  evidence: Both use `ErrTransferFailed`, so `PublicErrorOf` discards their safe messages and selects that fixed copy. Neither state involves a transfer that began. There is no not-ready code in the `ErrorCode` set, and the copy registry is fixed by the UX contract, so this is an EXPERIENCE.md decision -- the same shape as the Story 1.3, 1.5 and 1.6 entries about `transfer_failed` and `busy` describing states they do not fit. Both states are unreachable in a composed binary, since Wails runs `OnStartup` before the webview can call a command.

- source_spec: `spec-1-7-expose-safe-transfer-commands-through-wails.md`
  summary: `StageTransfer` and `CancelTransfer` fabricate a background context before startup while the dialogs refuse, so the two halves of the boundary disagree about "no window yet".
  owner: 3-4-bound-every-lifecycle-wait-and-prove-quiescence
  evidence: `delegate()` defaults a nil `a.ctx` to `context.Background()`, so a pre-startup Stage would bind a listener and start a beacon whose every lifecycle event `publish` then drops -- handing the UI a session it can never hear from. `chooseWith` refuses instead, because the real dialog would `log.Fatalf`. Unreachable today for the same reason as the entry above. Settling it means choosing whether a command may run before the window exists at all, which is a contract question rather than an adapter one.

- source_spec: `spec-1-7-expose-safe-transfer-commands-through-wails.md`
  summary: `undelivered` counts dropped lifecycle events and nothing in production reads it.
  owner: 3-4-bound-every-lifecycle-wait-and-prove-quiescence
  evidence: Incremented on a nil context, a recovered emit panic, and an unknown event kind; read only by tests. A dropped terminal event is precisely the failure no other party can observe -- the UI simply waits forever -- so the count is the only trace, and it is inert. The spec forbids logs, so surfacing it means a UI-visible degraded state, which belongs to Story 1.10's recovery contract. The comment now says it is inert rather than calling itself "the record".

- source_spec: `spec-1-7-expose-safe-transfer-commands-through-wails.md`
  summary: Nothing runs the frontend test suite automatically, and there is no CI at all.
  owner: 3-2-automate-reproducible-cross-platform-verification
  evidence: `wails.json` wires `frontend:build` to `npm run build` (`tsc && vite build`) and never `npm test`, there is no `.github/workflows` directory, and `frontend/tsconfig.json` includes only `src`, so `frontend/wailsjs` is not type-checked on its own. `errors.test.ts` runs only when someone types `npm test`. Story 3.2 owns reproducible cross-platform verification; this is the concrete list of what it has to pick up.

- source_spec: `spec-1-7-expose-safe-transfer-commands-through-wails.md`
  summary: Two hardening changes at the Wails boundary are correct but undistinguishable by any test.
  owner: accepted
  evidence: Narrowing the cross-language pin to search inside the `transferErrorCodes` array rather than the whole file closes a real hole -- a code surviving only in a comment would have satisfied the old check -- but every one of the twelve codes genuinely sits inside that array, so both forms pass and mutation cannot tell them apart (verified: the earlier "caught" result was a compile error from an unused variable, which proves nothing either way). The same applies to `appObserver`'s nil-app guard, which no production path can reach. Both are kept; neither is a tested guarantee.

- source_spec: `spec-1-8-manage-session-scoped-frontend-state-and-events.md`
  summary: Regenerating `epic-1-context.md` silently discards refinements, and the next `compile-epic-context` run will do it again.
  owner: accepted
  evidence: The 2026-08-29 run replaced a 1293-word compile with a 1064-word one, dropping the copy-registry-by-stable-key rule, the lowerCamelCase/`transfer-*`/`context.Context` conventions, the transitional-interface deletion rule, "every focus target proven to exist before focusing", Epic 1's own native download exit check, and the whole receiver-help paragraph that Stories 1.9 and 1.10 render from. The file is the compiled orientation artifact every later story reads, its own header invites free editing, and nothing reconciles an edit against a later regeneration -- so the loss was invisible until a diff was read. Restored by hand for Epic 1. The durable fix is a workflow question rather than a code one: either treat the compiled context as generated-only and move refinements into the planning artifacts it compiles from, or stop regenerating it once an epic is underway.

- source_spec: `spec-1-8-manage-session-scoped-frontend-state-and-events.md`
  summary: A malformed Stage acknowledgement is cancelled and reported, but nothing tells the user their selection was refused rather than lost.
  owner: 3-5-reconcile-public-error-copy-with-its-states
  evidence: `stage()` falls back to `publicError('transfer_failed')`, whose fixed copy says the transfer "stopped before FairDrop finished sending" -- the same mismatch the Story 1.3, 1.5, 1.6 and 1.7 entries record for states where no transfer began. Correct within this story, which may not change copy; it is the fifth instance of one missing code, and the accumulated case belongs to an EXPERIENCE.md decision before Story 1.10 fixes recovery text against it.

- source_spec: `spec-1-8-manage-session-scoped-frontend-state-and-events.md`
  summary: The frontend suite still runs only when someone types `npm test`, one story after the same finding.
  owner: 3-2-automate-reproducible-cross-platform-verification
  evidence: `wails build` regenerates bindings and compiles the frontend without running a single Vitest file, and there is still no `.github/workflows`. Story 1.8 raised the frontend suite from 35 tests to 164 and made it the only executable evidence for the forged-event defence, which raises the cost of the gap rather than changing it. Restated here so Story 3.2 sees that it grew.

- source_spec: `spec-1-9-render-the-paper-relay-transfer-views.md`
  summary: The native window paints a single background colour, so one of the two themes still gets a one-frame flash.
  owner: 3-3-produce-and-smoke-test-native-release-artifacts
  evidence: `main.go` takes one `options.RGBA` and Wails offers no per-theme value, so the constant now tracks the light `--color-canvas` and a dark-mode OS gets one light frame before the webview paints. Before this story it tracked a Tailwind class the story deleted, so light mode flashed slate-900 on every launch and nothing failed -- `main_test.go` now pins the constant to the token and names the coupling. Closing the residual means reading the OS theme in Go before building the options: on Windows that is one `golang.org/x/sys/windows/registry` read of `AppsUseLightTheme` (already an indirect dependency), with a build-tag sibling for macOS. That is platform code Story 1.9 was not scoped for, and Story 3.3's release evidence is where a first-paint check belongs.

- source_spec: `spec-1-9-render-the-paper-relay-transfer-views.md`
  summary: `selectVisibleError` and `selectRetainedOutcome` are now dead, and one of them answers the same question differently from its replacement.
  owner: 1-10-meet-the-accessibility-and-recovery-contract
  evidence: No view imports either; `selectCommandError` and `selectOutcome` replaced them. `selectVisibleError` folds a retained terminal error into the "visible error" and does not refuse `cancelled` -- precisely the two behaviours the replacements were written to prevent -- so the module exports contradictory answers and a later view can pick the wrong one and still compile. Not removed here because the spec's Code Map says `selectors.ts` may be extended "only additively", which is the right default for a module Story 1.8 defended; deleting an exported symbol and its tests is a separate, deliberate edit.

- source_spec: `spec-1-9-render-the-paper-relay-transfer-views.md`
  summary: Muted text now sits on the elevated surface, a pairing DESIGN.md's contrast table does not publish.
  owner: 1-10-meet-the-accessibility-and-recovery-contract
  evidence: Every muted string in the new views (`.fd-meta`, `.fd-trust`, `.fd-packet-tab`) renders inside `.fd-packet`/`.fd-transfer-view`, which are `--color-elevated`. DESIGN.md proves muted/canvas at 5.070533:1 and instructs "Re-run unrounded automated checks if ... adjacent surfaces change". Computed here, muted/elevated is 4.504478:1 light and 6.569759:1 dark -- it passes AA, with about 0.1% headroom in light mode, and `.fd-packet-tab` renders it at 12px. Story 1.10 owns the unrounded contrast proof and should add this pair to the published table rather than leave it derived.

- source_spec: `spec-1-9-render-the-paper-relay-transfer-views.md`
  summary: The direct URL is exposed as a `div` with `role="textbox"`, which assistive technology handles inconsistently.
  owner: 1-10-meet-the-accessibility-and-recovery-contract
  evidence: A textbox role with no editable host is not a pattern browsers agree on; a readonly `<input>` or a labelled `<output>` carries the value reliably and keeps it selectable. It also joins the Tab order and takes its 44px floor from a duplicated `min-block-size` rather than the shared `.fd-target` rule that a test pins. Story 1.10 owns the accessibility contract and should settle the element, not just its name.

- source_spec: `spec-1-9-render-the-paper-relay-transfer-views.md`
  summary: A terminal outcome offers no control at all, so a lost `transfer-reset` strands the window.
  owner: 3-4-bound-every-lifecycle-wait-and-prove-quiescence
  evidence: Done and Error render heading, message and nothing else; the way out is the backend's three-second reset, which produces the retained Idle node that carries Dismiss. The Story 1.7 entry above records that `publish` silently drops an event it cannot deliver and counts it in an inert `undelivered`, so a dropped reset is both possible and invisible. The frontend is forbidden a lifecycle timer, which makes this a recovery-contract question for Story 1.10 rather than something a view can fix.

- source_spec: `spec-1-9-render-the-paper-relay-transfer-views.md`
  summary: Idle's document outline opens on an `h2`, and the registered-but-unrendered Story 1.10 copy is tracked only in spec prose.
  owner: 1-10-meet-the-accessibility-and-recovery-contract
  evidence: The firewall guidance is an `<h2>` and any outcome panel above it is another, while the page's only `<h1>` is the drop instruction below both -- a consequence of the acceptance criterion that puts preflight first in document order, and no test pins the resolution. Separately, `copy.help.*`, both firewall recovery strings, `copy.cancel.won`, `copy.name.showFull` and `copy.external.promise` are registered and rendered nowhere, and nothing outside this spec's Design Notes records that debt. Story 1.10 owns both; a test asserting these strings stay unrendered until it does would make the boundary executable.

- source_spec: `spec-1-9-render-the-paper-relay-transfer-views.md`
  summary: Two declared assets are now referenced by nothing: the bundled Nunito face and `framer-motion`.
  owner: 1-10-meet-the-accessibility-and-recovery-contract
  evidence: `style.css` no longer declares `@font-face`, so `frontend/src/assets/fonts/nunito-v16-latin-regular.woff2` and its `OFL.txt` are unreferenced (Vite will stop emitting the file, but it stays in the tree). `framer-motion` is a locked runtime dependency that nothing imports, and this story's Never list bans every animation it would serve. Neither is a defect; both are weight a later story should either use or drop deliberately.

- source_spec: `spec-1-9-render-the-paper-relay-transfer-views.md`
  summary: The phase re-check added to the chooser's failure path is correct but adds nothing the reducer was not already doing.
  owner: accepted
  evidence: Applied during review so a chooser that failed while a drop was being staged could not report against another session. Mutation shows it is undistinguishable: `browse` reserves its own generation, so its `stage-requested` is refused for not being Idle and its `stage-failed` is refused on generation mismatch -- the reducer rejects both halves without the guard. Kept as a documented second line, in the same spirit as `selectCommandError`'s redundant `cancelled` refusal, but it is not a tested guarantee and should not be described as a race fix.

- source_spec: `spec-1-9-render-the-paper-relay-transfer-views.md`
  summary: The working tree holds mixed line endings, so text-matching tests and diffs behave differently per file.
  owner: 3-2-automate-reproducible-cross-platform-verification
  evidence: `TransferringView.tsx` and `StagedView.tsx` are CRLF on disk while `useTransfer.ts` and `style.css` are LF, because only `*.go` was pinned before this story. Two review mutations silently matched nothing for that reason and had to be re-run against normalized text -- a harness that reports "anchor matched 0" rather than a failure is exactly how a mutation pass overstates its own coverage. `.gitattributes` now pins `*.css`, `*.ts` and `*.tsx` to LF, so git renormalizes them on the next write, but nothing has rewritten the existing files and no check fails while they disagree.

- source_spec: `spec-1-9-render-the-paper-relay-transfer-views.md`
  summary: Wails' custom `wails://` scheme is not a secure context, so every browser API gated on one is unavailable on macOS.
  owner: 3-3-produce-and-smoke-test-native-release-artifacts
  evidence: WKWebView loads `wails://wails/` through `setURLSchemeHandler:`, and Wails 2.15.0 registers no secure scheme anywhere in its darwin frontend; WebView2 loads `http://wails.localhost/`, which Chromium treats as trustworthy. The clipboard hit this first and is fixed by routing through `runtime.ClipboardSetText`, but the asymmetry is general: `crypto.subtle`, `navigator.geolocation`, media capture and service workers are gated the same way, and a frontend feature that works in `wails dev` on Windows can be inert on macOS with no error. Worth a line in the project's agent instructions before another story reaches for a browser API, and worth confirming on a real Mac during Story 3.3's release evidence.
