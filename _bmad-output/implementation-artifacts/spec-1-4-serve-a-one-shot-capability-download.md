---
title: 'Story 1.4 — Serve a One-Shot Capability Download'
type: 'feature'
created: '2026-08-24'
status: 'ready-for-dev'
review_loop_iteration: 0
context:
  - '{project-root}/docs/fairdrop-contracts.md'
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** FairDrop can validate a selection, advertise an endpoint, and produce payload bytes, but nothing serves them. The only server contract is a Phase 1 placeholder taking a path string and a progress callback: no capability token, no claim authorization, no event channel, and no teardown guarantees.

**Approach:** Replace `TransferServer` with the consumer-owned `transfer.ServerPort` and implement an ephemeral one-shot HTTP server that reserves atomically, authorizes synchronously, and only then prepares and streams the Story 1.3 payload with wire-accurate progress.

## Boundaries & Constraints

**Always:** Register the methodless `http.ServeMux` pattern `/download/{token}`, read the token only through `PathValue`, and check `request.Method == http.MethodGet` explicitly. Compare tokens in constant time. Reserve atomically before authorizing, authorize synchronously, and open no payload and write no header until it succeeds. Count only bytes `ResponseWriter.Write` accepted, cap progress at 4 Hz, and never let event delivery block the handler or teardown. Keep every percentage finite and clamped. Deliver exactly one terminal event per natural outcome and none after it. Tear down in the fixed order: cancel the data-plane context, force-close the destination, wait for `WriteTo` and its workers, then call `Close` exactly once. Make `Stop` idempotent and force-closing, quiescent on every return, with the event channel closed permanently.

**Ask First:** Any change to the contract's `ServerPort`, `ServerStartRequest`, `ServerEvent`, `ServerHandle`, or `ClaimAuthorizer` shapes, any change to which HTTP status a scenario returns, any new dependency, a third-party router, or a whole-transfer write deadline.

**Never:** No method-qualified route pattern, manual path splitting, or third-party router. Never write an error body after payload bytes, promise a replay status once the listener has closed, or let `Close` race `WriteTo`. Never disclose the token, source path, or raw adapter text in a body, header, event, or log. No coordinator, QR, or frontend work; no persistence.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Start succeeds | Valid request, live context | Bound to `0.0.0.0:0`, accept loop ready; returns assigned port and event channel | N/A |
| Start fails | Bind or setup failure | Every partial listener, goroutine, and channel closed before returning | `server_start_failed` |
| Wrong method | POST, PUT, or **HEAD** on the exact route | 404; nothing reserved, authorized, opened, or emitted | 404, no detail |
| Wrong route | Extra segment, oversized, or malformed path | 404; handler logic never reached | 404, no detail |
| Wrong token | Exact route, non-matching token | 404; no reservation | 404, no detail |
| First claim | Exact-token GET | Reserves atomically; `AuthorizeClaim` invoked exactly once, synchronously | N/A |
| Competing claim | Second exact-token GET while reserved/claimed | 423 while that listener is live | 423, no detail |
| Authorization refused | Denied, cancelled, stale, or shutting down | No payload opened, no header written | 404 if it can still respond, else close |
| Prepare fails | Payload preparation fails after authorization | Generic 410; listener closed | one `ServerFailed` preserving the coded cause |
| Success headers | Authorized, payload prepared | Sanitized ASCII `filename` plus RFC 5987 `filename*`, `Cache-Control: no-store`, `Access-Control-Allow-Origin: *`, `X-Content-Type-Options: nosniff` | N/A |
| Known length | `Size` reports known | `Content-Length` present and equal to it; absent when unknown | N/A |
| Known empty file | Zero-byte payload | `Content-Length: 0`, terminal snapshot with percent 0 | N/A |
| Progress cadence | Bytes flowing | Only accepted bytes counted; at most 4 Hz; coalescing never blocks the handler | N/A |
| Natural completion | Final byte written | Authoritative terminal snapshot matching the prepared length, then exactly one `ServerComplete` | N/A |
| Disconnect mid-stream | Receiver drops or write fails after headers | One `ServerFailed`, final progress only if bytes were written; connection aborted, no error body appended | `transfer_failed` |
| Teardown ownership | Any completion, failure, cancel, or Stop | Context cancelled, destination force-closed, workers awaited, then exactly one payload `Close` | cleanup diagnostic only |
| Stop lifecycle | Before Start, mid-request, after termination, or repeated | Quiescent on every return; channel closed permanently and never reopened | cleanup diagnostic only |
| Connection limits | Slow or oversized request headers | Header-size and idle/read timeouts enforced | connection closed |

</frozen-after-approval>

## Code Map

- `docs/fairdrop-contracts.md:153-185` — **read-only, binding.** The exact `ServerStartRequest`, `ClaimAuthorizer`, `ServerEvent`, `ServerHandle`, and `ServerPort` shapes. Copy them.
- `docs/fairdrop-contracts.md:211-216` — **read-only, binding.** Port postconditions: readiness before return, force-closing idempotent `Stop`, non-blocking terminal delivery.
- `docs/fairdrop-contracts.md:288-303` — **read-only, binding.** The six-step claim/HTTP order and the routing rule, including why a method-qualified pattern is forbidden.
- `docs/fairdrop-contracts.md:305-330` — **read-only, binding.** Event grammar and `ProgressSnapshot.Percent` rules.
- `docs/fairdrop-contracts.md:75-81` — `ProgressSnapshot` shape; it is a domain value, so it belongs in `internal/transfer`.
- `internal/transfer/ports.go` — add `ServerPort`, `ServerStartRequest`, `ClaimAuthorizer`, `ServerEventKind`, `ServerEvent`, `ServerHandle` beside the existing ports.
- `internal/transfer/types.go` — add `ProgressSnapshot`.
- `internal/server/server.go` — **delete `TransferServer` and `TransferStats`.** Keep `PayloadPort`/`PreparedPayload` exactly as Story 1.3 left them; this story is their first consumer.
- `internal/stream/archiver.go:94` — the `PayloadPort` implementation to drive in tests; `Prepare` runs *after* authorization.
- `internal/network/beacon.go:25` — the transactional Start/ownership pattern this server's lifecycle should mirror.
- `internal/transfer/errors.go:78-85` — `NewError`/`WrapError`; all needed codes exist, add none.
- `_bmad-output/implementation-artifacts/deferred-work.md` — its Story 1.3 entry assigns the post-header connection-abort obligation to this story.
- `app.go`, `main.go`, `internal/source`, frontend — read-only, out of scope.

## Tasks & Acceptance

**Execution:**
- [ ] `internal/transfer/{ports,types}.go` — add the server contract and `ProgressSnapshot` verbatim from the contract.
- [ ] `internal/server/server.go` — delete the Phase 1 interfaces; implement `Start`/`Stop` with transactional startup and force-closing, idempotent teardown.
- [ ] `internal/server/*.go` — the router, the constant-time token check, atomic reservation, the synchronous authorization handshake, and response headers.
- [ ] `internal/server/*.go` — the progress meter and the non-blocking event lane with reserved terminal capacity.
- [ ] `internal/server/*_test.go` — cover every matrix row with `httptest` and injected seams; drive claim races concurrently, not sequentially.
- [ ] `internal/server/*_test.go` — prove payload `Close` happens exactly once, after `WriteTo` returns, and never concurrently with it.

**Acceptance Criteria:**
- Given a competing pair of exact-token GETs issued concurrently, when both are served, then `AuthorizeClaim` ran exactly once and the loser received 423 — proven under `-race`, not by sequential calls.
- Given any request that fails routing, method, or token checks, when it is answered, then no reservation, authorization, payload open, or event occurred, and the response carries no token, filename, or path.
- Given a stream that fails after headers, when the handler returns, then the connection is aborted rather than completed, no error body was appended, and exactly one `ServerFailed` carries final progress only if bytes were written.
- Given `Stop` at any point in the lifecycle, when it returns, then no listener, connection, handler, payload worker, or event producer is live, and the channel is closed and stays closed.
- Given the finished module, when build, vet, ordinary tests, race tests, formatting, and interface-surface checks run, then all pass with no `TransferServer` or `TransferStats` remaining anywhere in the repository.

## Spec Change Log

## Design Notes

The routing rule is measured, not stylistic. On go1.26.7 the forbidden method-qualified pattern `GET /download/{token}` answers POST with **405 and `Allow: GET, HEAD`**, and routes **HEAD into the GET handler** — both of which tell an unauthorized caller the resource exists. The methodless pattern hands every method to the handler, so the handler's own `Method == http.MethodGet` check is what makes HEAD and everything else indistinguishable from a nonexistent path. Verified with `httptest`: `/download/a/b` is already 404 from `ServeMux`, because a `{token}` wildcard does not match across a `/`.

Reservation must be atomic and must precede authorization, because two receivers can race the same URL. A compare-and-swap on reservation state is the linearization point; the loser sees 423 and never reaches `AuthorizeClaim`.

Terminal events need reserved, non-blocking capacity. The coordinator's drainer does not exist yet, so a naive unbuffered send from the handler would deadlock teardown — the postcondition that `Stop` is quiescent on every return depends on the terminal event never needing a live consumer.

`Prepare` runs after authorization and before headers, which is what makes a 410 possible: once any byte is written the status is fixed and the only honest failure signal left is aborting the connection.

## Verification

**Commands:**
- `go build ./... && go vet ./... && go test -count=1 ./...` — module and deterministic tests pass.
- `CGO_ENABLED=1 go test -race -count=1 ./...` — claim races and teardown are race-clean. Needs MinGW `bin` on PATH (see AGENTS.md); without it the detector errors instead of running.
- `go test -race -count=20 ./internal/server` — the claim race is a concurrency test; run it repeatedly.
- `gofmt -l .` and `git diff --check` — no formatting or whitespace defects.
- `rg -n 'TransferServer|TransferStats' -g '*.go'` — no output.
- `rg -n 'ServerPort|ClaimAuthorizer' internal --glob '!**/*_test.go'` — the port declared once in `internal/transfer`, implemented once in `internal/server`.
