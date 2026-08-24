---
title: 'Story 1.3 — Prepare and Stream a Regular File Safely'
type: 'feature'
created: '2026-08-23'
status: 'in-progress'
review_loop_iteration: 1
baseline_commit: '1e55e0c24e5fc18b54e4f25872f286756efbec9e'
context:
  - '{project-root}/docs/fairdrop-contracts.md'
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** FairDrop validates a selection and advertises an endpoint but cannot produce payload bytes. The only streaming contract is a Phase 1 placeholder taking a path string, so nothing re-checks the source at claim time, derives a wire length from a real descriptor, or bounds memory.

**Approach:** Replace `Streamer` with the server-owned `PayloadPort`/`PreparedPayload` contracts and a file-only adapter that re-validates the selected root, opens it, and streams from that same descriptor through one reusable buffer.

## Boundaries & Constraints

**Always:** Layer the defense — re-validate the root, `Lstat` it, open it, then `Stat` the *opened descriptor*, confirm it is the same filesystem object via `os.SameFile`, and derive the wire length from it, never from staging metadata. Bound the stream at that advertised length: never write more, and fail if fewer bytes arrive. Sanitize the download name to a bare basename carrying no separator, `..`, or control character. Reuse Story 1.1's link/reparse/ancestor policy instead of reimplementing it. Keep memory O(buffer) with one reusable buffer sized independently of the payload. Return existing coded errors wrapping causes with `%w` and disclosing no source path. Check context around blocking work. Give the payload exactly one effective `Close`, safe when repeated. Pass paths to native Go filesystem APIs unchanged as values.

**Ask First:** Any change to the contract's `PayloadPort`/`PreparedPayload` *or* `SourcePort` shapes, any new dependency, any buffer strategy allocating proportionally to payload size, any change to which code a scenario maps to, or implementing directory payloads (Epic 2 owns those).

**Never:** No `io.ReadAll`, `os.ReadFile`, `mmap`, payload-sized allocation, temporary copy, or persistence. Never shell out or rewrite a path. Never disclose a path, token, or raw adapter text publicly. No retry, no body appended after bytes are on the wire, no surviving worker goroutine. No compatibility `Streamer` or duplicate streaming interface, and no HTTP server, coordinator, QR, or UI work.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Prepare a regular file | Staged item matching disk | Descriptor opened and stat'd; known `Size`; `DownloadName` is the selected basename | N/A |
| Prepare an empty file | Zero-byte regular file | Known zero-byte payload, not unknown | N/A |
| Source missing | Root deleted after staging | No descriptor retained | `path_not_found` before headers |
| Source changed | Kind, size, or modtime differs from staged | No descriptor retained | `source_changed` before headers |
| Source became link-like | Now a symlink, junction, reparse point, or special file | No descriptor retained | `path_unsupported` |
| Directory item | `StagedItem.Kind` is directory | Refused; structure stays open to Epic 2 | `path_unsupported` |
| Unreadable source | Open fails otherwise (permissions) | No descriptor retained | `transfer_failed` wrapping the cause |
| Stream exact bytes | Prepared file, live context | Destination receives byte-identical content via the reused buffer | N/A |
| Cancellation mid-stream | Context cancelled during `WriteTo` | Returns promptly; no retry; no goroutine survives | `cancelled` |
| Destination failure | Destination write errors | Returns promptly; nothing further written | `transfer_failed` |
| Close lifecycle | After `WriteTo`, repeated, or without `WriteTo` | Descriptor released exactly once; later calls safe | cause reported on first call only |
| Path classes | Spaces, Unicode, long Windows, UNC | Path reaches Go APIs unchanged | typed error for host-unsupported cases |
| Identity swap | Replacement keeps kind, size, and modtime | Refused; `os.SameFile` separates the objects | `source_changed` before headers |
| Source truncated mid-stream | Fewer bytes readable than the advertised length | Fails rather than reporting a short body as success | `transfer_failed` |
| Source grown mid-stream | More bytes readable than the advertised length | Writes exactly the advertised length and stops | N/A |
| Adversarial staged name | `Name` holds separators, `..`, or CR/LF | Reduced to a safe bare basename | N/A |
| Repeat stream | `WriteTo` called a second time | Refused; never reports a no-op as success | `transfer_failed` |
| Deadline expiry | Context deadline exceeded, not cancelled | Distinguished from a user cancel | `transfer_failed` |

</frozen-after-approval>

## Code Map

- `docs/fairdrop-contracts.md:220-239` — **read-only, binding.** Fixes the exact method sets, the `Prepare`-before-headers rule, and the server's single-`Close` ownership. Copy the shapes.
- `docs/fairdrop-contracts.md:335-342` — **read-only, binding.** Source-mutation and link policy: re-`Lstat` at claim, `source_changed` before headers, open before deriving length, preserve spaces/Unicode/long paths.
- `internal/server/server.go` — add `PayloadPort` and `PreparedPayload` here; the contract makes them consumer-owned by `server`. Leave `TransferServer`/`TransferStats` untouched — Story 1.4 replaces those.
- `internal/stream/archiver.go` — **delete `Streamer`** and implement the file-only adapter in its place. `StreamZip` goes with it; Epic 2 reintroduces directories through `PayloadPort`.
- `internal/transfer/ports.go:16-19` — `SourcePort`. Inject it into the adapter to reuse claim-time validation.
- `internal/source/source.go:33` — `Inspect` already walks syntactic ancestors, rejects reparse points via build-tagged `platformReparsePoint`, and gates on `Mode().IsRegular()`. Reuse through the port; do not duplicate `internal/source/platform_windows.go`.
- `docs/fairdrop-contracts.md:330` — **read-only, binding.** The disclosure matrix requires a *sanitized* `Content-Disposition`; the download name is where that is enforced.
- `docs/fairdrop-spec.md:12-13,155-159,179` — still publishes the deleted `Streamer` contract. The contracts "Update rule" requires spec, contract, and tests to move together, so update this text to `PayloadPort`/`PreparedPayload`.
- `internal/transfer/errors.go:78-85` — `NewError`/`WrapError` are the only constructors. All needed codes exist; add none. Prefer `WrapError` wherever a cause exists.
- `internal/source/source_test.go` — the immutable-seam test pattern to follow.
- `_bmad-output/implementation-artifacts/deferred-work.md` — its final entry assigns this descriptor-first layering to this story.
- `app.go`, `main.go`, `internal/network`, frontend — read-only, out of scope.

## Tasks & Acceptance

**Execution:**
- [ ] `internal/server/server.go` — add the two interfaces verbatim from the contract, documenting the `Close` ownership rule.
- [ ] `internal/stream/archiver.go` — delete `Streamer`; implement `Prepare` (re-validate, open, stat the descriptor, compare against staged metadata) and `DownloadName`/`Size`/`WriteTo`/`Close`.
- [ ] `internal/stream/*_test.go` — cover every matrix row with injected seams; prove exact bytes, prompt cancellation, single Close, and goroutine exit.
- [ ] `internal/stream/*_test.go` — bounded-memory evidence that payload memory does not grow with file size across two sizes an order of magnitude apart. The small arm must sit well under the asserted bound so it can actually discriminate, and the buffer-size benchmark must time only `WriteTo`.
- [ ] `internal/stream/*_test.go` — assert the seam arguments: the same byte-identical path must reach both the source port and the open call, and a mid-stream read failure must be driven with bytes still pending.
- [ ] `docs/fairdrop-spec.md` — replace the deleted `Streamer` contract text with `PayloadPort`/`PreparedPayload`.

**Acceptance Criteria:**
- Given a successful `Prepare`, when the wire length is read, then it derives from the opened descriptor's `Stat` rather than `StagedItem.LogicalSize`, proven by a test where the two would diverge.
- Given any `Prepare` failure, when it returns, then no descriptor remains open and the error carries a matrix code whose public message contains no path.
- Given a `WriteTo` that ends for any reason, when `Close` runs after it, then the descriptor is released exactly once and no goroutine this package created is still running.
- Given a source mutated after `Prepare`, when `WriteTo` finishes, then the destination received exactly the advertised length or the call returned a coded error — never a silent short or long body.
- Given a `StagedItem.Name` containing separators, `..`, or CR/LF, when `DownloadName` is read, then it yields a bare basename safe to place in a header.
- Given the finished module, when build, vet, ordinary tests, race tests, formatting, and interface-surface checks run, then all pass with no `Streamer` remaining anywhere in the repository, documentation included.

## Spec Change Log

- **Review loop 1 (2026-08-23):** Three parallel review layers converged, with two independent probe demonstrations, on `WriteTo` streaming to EOF without accounting against the `Size` it advertised — a 6-byte file appended to streamed 106 bytes under a 6-byte `Content-Length` and returned `nil`. Review also showed that kind/size/modtime alone accept an `os.Chtimes`-forged replacement, and that `DownloadName` passed `StagedItem.Name` through verbatim despite the binding contract requiring a sanitized `Content-Disposition`. The human renegotiated the frozen block: the stream is now bounded at the advertised length and fails on mismatch, identity is pinned with `os.SameFile` inside `internal/stream` (explicitly *not* by changing `SourcePort`), and the download name is reduced to a safe bare basename. Non-frozen sections gained the repository-wide `Streamer` grep that an `internal/`-only scope let slip, the `docs/fairdrop-spec.md` drift, and the seam-argument and benchmark-timing test obligations. This avoids the known-bad state where FairDrop reports a completed transfer for a body that never matched its header. **KEEP:** the `Prepare` order (validate through `SourcePort`, open, stat the descriptor, compare) and its descriptor-release-on-every-failure discipline; the single per-stream reusable buffer and its `defaultBufferSize` rationale; `sync.Once`-guarded `Close`; the always-running seam coverage that keeps the link-like matrix row honest on an unprivileged host; and the existing passing tests, which are additive to — not contradicted by — these amendments.

## Design Notes

`Prepare` is the security boundary and its order is load-bearing: validate through `SourcePort`, then open, then `Stat` the descriptor and compare kind, size, and modtime. The path-based check cannot close the TOCTOU window alone — the descriptor stat is what the wire length comes from, so a swap between the two checks fails `source_changed` rather than streaming unexpected bytes.

Kind, size, and modtime are forgeable together, so they cannot be the whole claim-time check: `os.SameFile` compares the volume serial and file index and is what actually separates a replacement from the staged object. Compare the pre-open `Lstat` info against the opened descriptor's `Stat`, so the identity verified is the identity streamed.

`Size()` is the promise `Content-Length` is built from, which makes it a bound and not a hint. Cap reads at it and verify the total on the way out; a short read returning `nil` would report success for a truncated body and slip past the connection-abort defense Story 1.4 owns, because that defense keys on a non-nil error.

Directories are refused, not stubbed. Epic 2 adds that branch behind the same port, so leave room for a second payload kind without introducing one.

Buffer size is a benchmark choice per the epic's Phase 3 note; record the size and rationale. What is fixed: the buffer is reused across the stream and its size is independent of payload size.

## Verification

**Commands:**
- `go build ./... && go vet ./... && go test -count=1 ./...` — module and deterministic tests pass.
- `CGO_ENABLED=1 go test -race -count=1 ./...` — streaming and Close seams are race-clean. Needs MinGW `bin` on PATH (see AGENTS.md); without it the detector errors instead of running.
- `gofmt -l .` and `git diff --check` — no formatting or whitespace defects.
- `rg -n 'Streamer|StreamFile|StreamZip' --glob '!_bmad-output/**'` — no output. Scope this to the whole repository, not `internal/`: documentation drift is invisible to an `internal/`-only grep.
- `rg -n 'io.ReadAll|os.ReadFile|mmap' internal/stream` — no output.
- `rg -n 'type (PayloadPort|PreparedPayload) interface' internal --glob '!**/*_test.go'` — exactly one declaration each.
