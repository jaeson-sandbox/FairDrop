# Deferred Work

Real findings surfaced during review that are not the current story's problem.
Append-only. Each entry names the spec that surfaced it.

> **Note (Story 1.3):** entries below that name `Streamer`, `StreamFile`, or `StreamZip`
> describe a contract that no longer exists. It was replaced by the server-owned
> `PayloadPort`/`PreparedPayload`. The findings still stand; read them against the new
> contract in `docs/fairdrop-contracts.md`.

- source_spec: `spec-phase-1-wails-scaffold.md`
  summary: Backend contracts assume a single staged path while the frontend accepts multi-file drops.
  evidence: `TransferServer.Start(filePath string, ...)` and `Streamer.StreamFile`/`StreamZip` each take one path, but the I/O matrix requires a three-file drop to be accepted and `App.tsx` stores `string[]`. Phase 5 (`StageTransfer`) must decide: zip a multi-selection, stage only the first, or reject.

- source_spec: `spec-phase-1-wails-scaffold.md`
  summary: `TransferStats.Percent` has no defined behavior when `TotalBytes` is 0 (unknown-size zip stream).
  evidence: Spec §6 Module B says Content-Length is omitted when streaming a directory, so TotalBytes is genuinely unknown for zips. Dividing by it yields NaN/Inf, which `encoding/json` refuses to marshal — the progress event would fail to emit. Phase 4 must define the unknown-total case.

- source_spec: `spec-phase-1-wails-scaffold.md`
  summary: `Streamer` has no defined way to signal a mid-stream failure after headers are written.
  evidence: Once bytes are on the wire a returned error cannot change the status code, so the receiver saves a truncated file that looks complete. Phase 3 should specify `panic(http.ErrAbortHandler)` (or equivalent) to break the connection so the client sees a failed download.

- source_spec: `spec-phase-1-wails-scaffold.md`
  summary: `TransferServer.Stop` has no documented idempotency contract.
  evidence: Spec §3 resets to IDLE after DONE/ERROR and §7 allows user cancellation, so Stop can plausibly be reached twice or before Start. Undefined semantics invite a double-close panic or a leaked listener in Phase 4.

- source_spec: `spec-phase-1-wails-scaffold.md`
  summary: No CI runs the verification commands, so the verified state decays from the next commit.
  evidence: No `.github/`, Makefile, or task runner exists. The spec's six verification commands are hand-run only. A clean-clone CI job would have caught the `go:embed` gap on the very first push.

- source_spec: `spec-phase-1-wails-scaffold.md`
  summary: Drag-and-drop is the only input path; there is no keyboard or pointer-free way to stage a file.
  evidence: The zone is a bare `div` with no role, no `aria-label`, no "Browse…" fallback, and the results list has no `aria-live` region so new paths are never announced. Phase 6 builds the real DropZone and should add a file-picker fallback.

- source_spec: `spec-phase-1-wails-scaffold.md`
  summary: Path edge cases are untested — spaces, non-ASCII, >260 chars, UNC shares, symlinks, and zero-path drops.
  evidence: Every path in the matrix and tests is a simple `C:\x\...`. Windows MAX_PATH and UNC handling are real hazards for a file-transfer tool, and they become testable in Phases 2-4 where the Go side actually opens the paths.

- source_spec: `spec-phase-1-wails-scaffold.md`
  summary: `frontend/tsconfig.json` uses legacy `moduleResolution: "Node"` against a toolchain that publishes subpath exports.
  evidence: Vite 7, Vitest 4, and `@tailwindcss/vite` expose entry points such as `vitest/config` through `exports` maps that node10 resolution cannot read. It type-checks today, but `"bundler"` is the resolution these tools expect and the current setting is a latent trap.

- source_spec: `spec-phase-1-wails-scaffold.md`
  summary: `npm ci --omit=dev` would break `tsc` because test files are inside the build's type-check scope.
  evidence: `tsconfig.json` includes all of `src`, which now contains `App.test.tsx` importing `vitest` and `@testing-library/react` (both devDependencies). Not currently reachable — `wails.json` runs plain `npm install` — but a production-flavored CI install would fail the build.

- source_spec: `spec-1-1-validate-and-describe-one-file-selection.md`
  summary: Claim-time source revalidation and payload opening should pin filesystem identities so an ancestor replacement cannot exploit the metadata snapshot's TOCTOU window.
  evidence: Story 1.1 now `Lstat`s every syntactic ancestor, rejects native Windows reparse attributes, and rechecks cancellation, but separate path-based metadata calls cannot atomically prevent a local rename between checks. The binding contract already assigns later defenses to claim-time re-`Lstat` and descriptor-first payload opening; their stories must preserve and verify that layering.

- source_spec: `spec-1-3-prepare-and-stream-a-regular-file-safely.md`
  summary: A post-header stream failure must break the receiver's connection, and only the HTTP handler can do that.
  evidence: `PreparedPayload.WriteTo` reports a mid-stream read, cancellation, or destination failure as a coded `transfer_failed`/`cancelled` error, but `Content-Length` is already on the wire by then, so a plain handler return leaves the receiver holding a truncated file that looks complete. The payload port owns no connection, so Story 1.4 must turn a non-nil `WriteTo` error into a killed connection (`panic(http.ErrAbortHandler)` or equivalent). This supersedes the Phase 1 entry that assigned the same finding to Phase 3.

- source_spec: `spec-1-3-prepare-and-stream-a-regular-file-safely.md`
  summary: `transfer_failed`'s fixed public copy misdescribes a deadline that expires during Prepare, before any byte is sent.
  evidence: Story 1.3 deliberately separates a deadline from a user cancel, but both Prepare-time and stream-time deadlines map to `transfer_failed`, whose registry string is "The transfer stopped before FairDrop finished sending." Prepare runs before headers, so nothing was being sent. The copy registry is fixed by the UX contract, so changing this is a UX decision (EXPERIENCE.md), not an adapter one.

- source_spec: `spec-1-3-prepare-and-stream-a-regular-file-safely.md`
  summary: The claim-time re-`Lstat` does not re-apply Story 1.1's reparse-point check, so a junction created inside the validate-to-open window surfaces as `source_changed` rather than `path_unsupported`.
  evidence: `Payloads.lstatPath` is a bare `os.Lstat`. A reparse point created between `Inspect` and that `Lstat` is caught only incidentally, by `os.SameFile` failing. The transfer is still refused and no wrong bytes stream, so this is a code-accuracy issue rather than a safety hole, but the frozen matrix's link-like row promises `path_unsupported`.

- source_spec: `spec-1-3-prepare-and-stream-a-regular-file-safely.md`
  summary: `WriteTo`'s once-only CAS is never exercised concurrently, so the race suite never visits the one place two callers can collide.
  evidence: `TestCloseIsSafeWhenCalledConcurrently` fires eight goroutines at `Close`, but `streamed.CompareAndSwap` is only driven sequentially by `TestWriteToRefusesASecondCall`. The contract says the server never calls `WriteTo` twice, so this is defense-in-depth coverage rather than a live defect.

- source_spec: `spec-1-3-prepare-and-stream-a-regular-file-safely.md`
  summary: `Prepare` returns a `SourcePort` error verbatim, so the "every Prepare failure is coded" postcondition rests on the adapter rather than being enforced at the boundary.
  evidence: `internal/source` complies today, but `SourcePort` is an interface and nothing checks. An uncoded error would only be flattened to `transfer_failed` at the UI boundary, losing the specific code. Enforcing it means wrapping unrecognized errors at the port call, which touches the error-code mapping the spec puts behind Ask First.

- source_spec: `spec-1-3-prepare-and-stream-a-regular-file-safely.md`
  summary: `internal/stream/archiver.go` no longer archives anything, and no linter backs the `//nolint:staticcheck` directives in its tests.
  evidence: `StreamZip` and the zip logic are gone; the file now holds the single-file payload adapter, and Epic 2 will reintroduce directories behind the same port. Renaming to `payload.go` is a Code Map decision for whoever opens Epic 2. Separately, Verification runs build, vet, test, race, gofmt and greps but no staticcheck, so those directives are unenforced decoration.
