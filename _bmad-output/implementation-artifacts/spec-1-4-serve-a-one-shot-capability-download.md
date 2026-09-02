---
title: 'Story 1.4 — Serve a One-Shot Capability Download'
type: 'feature'
created: '2026-08-24'
status: 'done'
review_loop_iteration: 0
baseline_commit: '5f7017134a9403fb431d11fa33c1b9c85f8008d0'
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
- [x] `internal/transfer/{ports,types}.go` — add the server contract and `ProgressSnapshot` verbatim from the contract.
- [x] `internal/server/server.go` — delete the Phase 1 interfaces; implement `Start`/`Stop` with transactional startup and force-closing, idempotent teardown.
- [x] `internal/server/*.go` — the router, the constant-time token check, atomic reservation, the synchronous authorization handshake, and response headers.
- [x] `internal/server/*.go` — the progress meter and the non-blocking event lane with reserved terminal capacity.
- [x] `internal/server/*_test.go` — cover every matrix row with `httptest` and injected seams; drive claim races concurrently, not sequentially.
- [x] `internal/server/*_test.go` — prove payload `Close` happens exactly once, after `WriteTo` returns, and never concurrently with it.

**Acceptance Criteria:**
- Given a competing pair of exact-token GETs issued concurrently, when both are served, then `AuthorizeClaim` ran exactly once and the loser received 423 — proven under `-race`, not by sequential calls.
- Given any request that fails routing, method, or token checks, when it is answered, then no reservation, authorization, payload open, or event occurred, and the response carries no token, filename, or path.
- Given a stream that fails after headers, when the handler returns, then the connection is aborted rather than completed, no error body was appended, and exactly one `ServerFailed` carries final progress only if bytes were written.
- Given `Stop` at any point in the lifecycle, when it returns, then no listener, connection, handler, payload worker, or event producer is live, and the channel is closed and stays closed.
- Given the finished module, when build, vet, ordinary tests, race tests, formatting, and interface-surface checks run, then all pass with no `TransferServer` or `TransferStats` remaining anywhere in the repository.

## Spec Change Log

- **Review round 1 (2026-08-24, patches only — no loopback):** Three parallel layers found three code defects and two verification gaps, none of which required renegotiating the frozen block; the matrix already demanded what the code was missing. The defects: `WriteTo` returning `nil` was trusted as fact even though `PayloadPort` is an interface and `Content-Length` was already on the wire, so a short body could publish as `ServerComplete`; `publishTerminal` set `terminated` before either send attempt, so a lane that failed to deliver a terminal event would refuse every later one; and the terminal event escaped before the payload descriptor was released, which a coordinator acting on `Complete` can observe as a still-open source handle. The gaps, each demonstrated by mutation: binding `127.0.0.1` instead of `0.0.0.0` passed the whole suite, because every test replaces the listen seam with a loopback binder that discards its argument; and the teardown gate in `enter()` had no execution anywhere, with a concurrent test unable to reach it because the window between `beginStop` and the listener closing is too narrow. Non-frozen sections were not amended. **KEEP:** the six-step claim order and its measured routing rationale; the canonicality guard, which is the only thing preventing a 307 that echoes the capability token; the header flush before `WriteTo`, which keeps an unknown length unknown and lets the abort path be honest; the reserved-capacity event lane; and the mutation-tested evidence added for each fix.

## Design Notes

The routing rule is measured, not stylistic. On go1.26.7 the forbidden method-qualified pattern `GET /download/{token}` answers POST with **405 and `Allow: GET, HEAD`**, and routes **HEAD into the GET handler** — both of which tell an unauthorized caller the resource exists. The methodless pattern hands every method to the handler, so the handler's own `Method == http.MethodGet` check is what makes HEAD and everything else indistinguishable from a nonexistent path. Verified with `httptest`: `/download/a/b` is already 404 from `ServeMux`, because a `{token}` wildcard does not match across a `/`.

A second measured finding: `ServeMux` answers a non-canonical path with **307 and a `Location` header echoing the full capability token** -- confirmed for `/download/../download/{token}`, `//download/{token}`, `/download//{token}`, and `/./download/{token}`. A pattern guard does not catch it, because `mux.Handler(request)` reports the *cleaned* path's pattern (`/download/{token}`) for every one of those. Only an explicit canonicality check refuses them, and refusing is required rather than rewriting: the redirect is unauthenticated, so the token would reach logs and proxies before any claim is made.

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

## Suggested Review Order

**The claim sequence, in the order it must happen**

- The six steps in one function; their order is the security property.
  [`handler.go:76`](../../internal/server/handler.go#L76)

- The reservation is the linearization point of the claim race.
  [`handler.go:96`](../../internal/server/handler.go#L96)

- Constant-time comparison, so a wrong token leaks no timing signal.
  [`handler.go:214`](../../internal/server/handler.go#L214)

**Refusing what ServeMux would otherwise answer**

- Two of the mux's answers are wrong for a capability URL and are replaced.
  [`handler.go:34`](../../internal/server/handler.go#L34)

- Non-canonical paths are refused, never rewritten: rewriting is what leaks the token.
  [`handler.go:60`](../../internal/server/handler.go#L60)

**Size as a promise, and the honest failure**

- The server re-checks the payload's nil against the length it advertised.
  [`handler.go:164`](../../internal/server/handler.go#L164)

- Once bytes are on the wire, breaking the connection is the only signal left.
  [`handler.go:195`](../../internal/server/handler.go#L195)

- Only bytes the destination accepted are counted.
  [`progress.go:133`](../../internal/server/progress.go#L133)

- Percent stays finite and clamped whatever the totals are.
  [`progress.go:94`](../../internal/server/progress.go#L94)

**Delivery that cannot block teardown**

- The terminal event evicts a stale snapshot to make room for itself.
  [`events.go:70`](../../internal/server/events.go#L70)

- Progress is droppable by contract, so no publish ever blocks.
  [`events.go:49`](../../internal/server/events.go#L49)

**Lifecycle and ownership**

- Startup is transactional: readiness before return, nothing retained on failure.
  [`lifecycle.go:124`](../../internal/server/lifecycle.go#L124)

- The gate that keeps a late request out of the WaitGroup Stop is awaiting.
  [`lifecycle.go:302`](../../internal/server/lifecycle.go#L302)

- Teardown drives every owned resource to quiescent before returning.
  [`lifecycle.go:268`](../../internal/server/lifecycle.go#L268)

- The consumer-owned port this package implements.
  [`ports.go:98`](../../internal/transfer/ports.go#L98)

**Evidence the defenses are not vacuous**

- The claim race driven concurrently; ordering is not asserted, outcome is.
  [`handler_test.go:187`](../../internal/server/handler_test.go#L187)

- Binding loopback instead of every interface fails here and nowhere else.
  [`lifecycle_test.go:430`](../../internal/server/lifecycle_test.go#L430)

- The teardown gate, pinned deterministically where a concurrent test could not reach it.
  [`lifecycle_test.go:470`](../../internal/server/lifecycle_test.go#L470)

- A payload that under-delivers must not be published as Complete.
  [`handler_test.go:475`](../../internal/server/handler_test.go#L475)

- Close blocks, so the ordering against the terminal event is deterministic.
  [`handler_test.go:509`](../../internal/server/handler_test.go#L509)

- A failed terminal delivery must leave the outcome retryable.
  [`events_test.go:138`](../../internal/server/events_test.go#L138)

- Every rejection shape, including the traversal only the pattern guard stops.
  [`handler_test.go:24`](../../internal/server/handler_test.go#L24)

- Separators and dot-dot must not reach the legacy filename form.
  [`handler_test.go:853`](../../internal/server/handler_test.go#L853)
