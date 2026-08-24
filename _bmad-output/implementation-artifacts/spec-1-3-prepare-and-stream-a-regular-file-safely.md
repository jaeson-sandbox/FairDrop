---
title: 'Story 1.3 — Prepare and Stream a Regular File Safely'
type: 'feature'
created: '2026-08-23'
status: 'in-progress'
review_loop_iteration: 0
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

**Always:** Layer the defense — re-validate the root, open it, then `Stat` the *opened descriptor* and derive the wire length from it, never from staging metadata. Reuse Story 1.1's link/reparse/ancestor policy instead of reimplementing it. Keep memory O(buffer) with one reusable buffer sized independently of the payload. Return existing coded errors wrapping causes with `%w` and disclosing no source path. Check context around blocking work. Give the payload exactly one effective `Close`, safe when repeated. Pass paths to native Go filesystem APIs unchanged as values.

**Ask First:** Any change to the contract's `PayloadPort`/`PreparedPayload` shapes, any new dependency, any buffer strategy allocating proportionally to payload size, any change to which code a scenario maps to, or implementing directory payloads (Epic 2 owns those).

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

</frozen-after-approval>

## Code Map

- `docs/fairdrop-contracts.md:220-239` — **read-only, binding.** Fixes the exact method sets, the `Prepare`-before-headers rule, and the server's single-`Close` ownership. Copy the shapes.
- `docs/fairdrop-contracts.md:335-342` — **read-only, binding.** Source-mutation and link policy: re-`Lstat` at claim, `source_changed` before headers, open before deriving length, preserve spaces/Unicode/long paths.
- `internal/server/server.go` — add `PayloadPort` and `PreparedPayload` here; the contract makes them consumer-owned by `server`. Leave `TransferServer`/`TransferStats` untouched — Story 1.4 replaces those.
- `internal/stream/archiver.go` — **delete `Streamer`** and implement the file-only adapter in its place. `StreamZip` goes with it; Epic 2 reintroduces directories through `PayloadPort`.
- `internal/transfer/ports.go:16-19` — `SourcePort`. Inject it into the adapter to reuse claim-time validation.
- `internal/source/source.go:33` — `Inspect` already walks syntactic ancestors, rejects reparse points via build-tagged `platformReparsePoint`, and gates on `Mode().IsRegular()`. Reuse through the port; do not duplicate `internal/source/platform_windows.go`.
- `internal/transfer/errors.go:78-85` — `NewError`/`WrapError` are the only constructors. All needed codes exist; add none.
- `internal/source/source_test.go` — the immutable-seam test pattern to follow.
- `_bmad-output/implementation-artifacts/deferred-work.md` — its final entry assigns this descriptor-first layering to this story.
- `app.go`, `main.go`, `internal/network`, frontend — read-only, out of scope.

## Tasks & Acceptance

**Execution:**
- [x] `internal/server/server.go` — add the two interfaces verbatim from the contract, documenting the `Close` ownership rule.
- [x] `internal/stream/archiver.go` — delete `Streamer`; implement `Prepare` (re-validate, open, stat the descriptor, compare against staged metadata) and `DownloadName`/`Size`/`WriteTo`/`Close`.
- [x] `internal/stream/*_test.go` — cover every matrix row with injected seams; prove exact bytes, prompt cancellation, single Close, and goroutine exit.
- [x] `internal/stream/*_test.go` — bounded-memory evidence that payload memory does not grow with file size across two sizes an order of magnitude apart.

**Acceptance Criteria:**
- Given a successful `Prepare`, when the wire length is read, then it derives from the opened descriptor's `Stat` rather than `StagedItem.LogicalSize`, proven by a test where the two would diverge.
- Given any `Prepare` failure, when it returns, then no descriptor remains open and the error carries a matrix code whose public message contains no path.
- Given a `WriteTo` that ends for any reason, when `Close` runs after it, then the descriptor is released exactly once and no goroutine this package created is still running.
- Given the finished module, when build, vet, ordinary tests, race tests, formatting, and interface-surface checks run, then all pass with no `Streamer` remaining.

## Spec Change Log

## Design Notes

`Prepare` is the security boundary and its order is load-bearing: validate through `SourcePort`, then open, then `Stat` the descriptor and compare kind, size, and modtime. The path-based check cannot close the TOCTOU window alone — the descriptor stat is what the wire length comes from, so a swap between the two checks fails `source_changed` rather than streaming unexpected bytes.

Directories are refused, not stubbed. Epic 2 adds that branch behind the same port, so leave room for a second payload kind without introducing one.

Buffer size is a benchmark choice per the epic's Phase 3 note; record the size and rationale. What is fixed: the buffer is reused across the stream and its size is independent of payload size.

## Verification

**Commands:**
- `go build ./... && go vet ./... && go test -count=1 ./...` — module and deterministic tests pass.
- `CGO_ENABLED=1 go test -race -count=1 ./...` — streaming and Close seams are race-clean. Needs MinGW `bin` on PATH (see AGENTS.md); without it the detector errors instead of running.
- `gofmt -l .` and `git diff --check` — no formatting or whitespace defects.
- `rg -n 'type Streamer interface' internal` and `rg -n 'io.ReadAll|os.ReadFile' internal/stream` — no output.
- `rg -n 'type (PayloadPort|PreparedPayload) interface' internal --glob '!**/*_test.go'` — exactly one declaration each.
