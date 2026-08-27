---
title: 'Story 1.5 — Stage and Authorize a Transfer Transactionally'
type: 'feature'
created: '2026-08-24'
status: 'in-review'
review_loop_iteration: 0
baseline_commit: 'f634a0b2ffadf5ffa99fc2e52daf4ac40a7a7532'
context:
  - '{project-root}/docs/fairdrop-contracts.md'
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Four adapters exist and nothing composes them. There is no session identity, no state machine, no transactional setup, and no `ClaimAuthorizer` for the server built in Story 1.4 to call — so a QR code could advertise a session whose listener never started.

**Approach:** Build the framework-independent coordinator that owns lifecycle state. It consumes the `QRPort` that Story 1.5a implements. Stage acquires every resource through one per-session operation lease and commits STAGED only when all of them are live; `AuthorizeClaim` is the synchronous handshake that turns a claimed capability into TRANSFERRING.

## Boundaries & Constraints

**Always:** Never hold the state mutex across an external adapter call. After every unlocked call, reacquire and revalidate session ID, expected state, cancellation generation, and the closing flag before using the result or starting the next step. Route every adapter Start/Stop/unwind through one per-session operation lease. Unwind in reverse acquisition order on any failure or cancellation. Generate `SessionID` and `CapabilityToken` independently, each ≥128 bits from an injectable CSPRNG, neither derived from the other. Publish `transfer-started` with `seq=1` synchronously while still holding the lease, after the TRANSFERRING commit. Keep the token in the URL and QR only.

**Ask First:** Any change to the contract's port, `FileMetadata`, `Warning`, or `Observer` shapes; any new dependency at all, since this story adds none; persisting any session value; or implementing terminal outcomes, the reset timer, or Shutdown, which Story 1.6 owns.

**Never:** No QR encoding implemented here -- Story 1.5a owns that adapter and this story only calls the port. No adapter call under the mutex, no concurrent cleanup path competing with the lease, no lifecycle event before Stage is acknowledged, and no successful metadata returned after Cancel wins. Never let the token, source path, or raw adapter text reach mDNS, an event, a warning, or a diagnostic. No Wails, HTTP-handler, or frontend work; no persistence, telemetry, or logs.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Stage from IDLE | One valid file path | STAGING under the mutex, lease held; commits STAGED only with every resource live | N/A |
| Concurrent Stage | Stage while not IDLE | Refused with no state or resource change | `busy` |
| Session identity | New Stage | Independent `SessionID` and `CapabilityToken`, each ≥128 bits from the injected CSPRNG | N/A |
| Entropy failure | CSPRNG returns an error | No resource acquired, no state change | coded failure, unwound to IDLE |
| Acquisition order | Live Stage | Inspect, resolve address, start server and drainer, build URL, encode QR, then beacon last | N/A |
| Setup failure | Source, network, server, or QR fails | Reverse unwind through the lease; IDLE; no lifecycle event | applicable coded error |
| Cancel during any step | Cancel after each external call | Result discarded, never committed; reverse unwind; no event | `cancelled` |
| Stale result | Session or generation changed during a call | Result discarded rather than used | `cancelled` |
| Beacon failure only | HTTP and QR ready, mDNS fails | Commits STAGED with usable metadata; records no active beacon | one `beacon_warning`, non-null array |
| Successful commit | All setup succeeded | `FileMetadata` with sessionId, name, size, `isDir=false`, URL, padded base64 PNG with no data-URI prefix, empty warnings | N/A |
| Token disclosure | Any staged session | Token appears in the URL and QR only, never in beacon, event, warning, or diagnostic | N/A |
| Claim while STAGED | Matching session | CLAIMING, lease held, beacon stopped without the mutex, revalidate, commit TRANSFERRING | N/A |
| Claim refused | Stale, non-STAGED, cancelled, or closing | No commit, no `transfer-started`, no payload opened | `cancelled` or `shutting_down` |
| Claim wins the race | Commit precedes Cancel | `transfer-started` published synchronously at `seq=1` under the lease | N/A |
| Cancel wins the race | Cancel marks before the commit | No started event; reset ordering preserved | `cancelled` |
| Beacon stop diagnostic | `StopBeacon` reports cleanup trouble | Authorization proceeds; diagnostic recorded internally only | never implies a live beacon |
| QR failure | `QRPort` refuses the capability URL | Reverse unwind; no session committed | `qr_failed` |
| Base64 boundary | QR bytes returned by the port | Metadata carries standard padded base64, no data-URI prefix | N/A |

</frozen-after-approval>

## Code Map

- `docs/fairdrop-contracts.md:126-151` — **read-only, binding.** `QRPort`, `Observer`, and the coordinator-facing port set.
- `docs/fairdrop-contracts.md:260-286` — **read-only, binding.** The command/state table, the operation-lease rule, the no-mutex-across-adapter-calls rule, and the two linearization points.
- `docs/fairdrop-contracts.md:60-84` — `FileMetadata`, `Warning`, `PublicError` shapes.
- `docs/fairdrop-contracts.md:305-330` — event grammar; this story emits only `transfer-started` at `seq=1`.
- `internal/transfer/types.go` — add `FileMetadata`, `Warning`; `ProgressSnapshot` already exists.
- `internal/transfer/ports.go` — add `QRPort` and `Observer` beside the existing ports. `ClaimAuthorizer` already exists — the coordinator is its first implementation.
- `internal/transfer/coordinator.go` — **new.** State, mutex, generation counter, operation lease, Stage, AuthorizeClaim. Framework-independent: imports no adapter and no Wails.
- `internal/qr/qr.go` — the `QRPort` implementation from Story 1.5a. **Read-only here**; this story is its first consumer, and the base64 encoding of its bytes happens at this boundary, not in that package.
- `internal/server/lifecycle.go:124` — `Start` returns only when the listener is ready; that is what makes "commit only when live" true.
- `internal/network/beacon.go:101` — `StopBeacon` is idempotent and guarantees no advertisement remains, which is why a claim-time diagnostic is safe.
- `internal/source/source.go:33`, `internal/stream/archiver.go:94` — the adapters Stage and the server consume.
- `app.go`, `main.go`, frontend — read-only. Story 1.7 wires this in.

## Tasks & Acceptance

**Execution:**
- [x] `internal/transfer/{types,ports}.go` — add `FileMetadata`, `Warning`, and `Observer` verbatim from the contract. `QRPort` already exists from Story 1.5a.
- [x] `internal/transfer/coordinator.go` — state machine, generation counter, operation lease, and injectable entropy/clock seams.
- [x] `internal/transfer/coordinator.go` — `Stage` with ordered acquisition, post-call revalidation, and reverse unwind.
- [x] `internal/transfer/coordinator.go` — `AuthorizeClaim` handshake through to the synchronous `transfer-started` publication.
- [x] `internal/transfer/*_test.go` — fake source, network, server, QR, observer, entropy, and clock; force cancellation after *each* external step and force *both* claim-race outcomes.
- [x] `internal/transfer/*_test.go` — every fake asserts on entry that the coordinator's state mutex is NOT held, so the no-lock-across-adapter-calls rule is executable rather than prose.

**Acceptance Criteria:**
- Given cancellation injected after each external setup step in turn, when Stage unwinds, then every acquired resource was released in reverse order, no lifecycle event was emitted, and no stale result was committed.
- Given the claim race driven both ways under `-race`, when the commit wins, then `transfer-started` is published at `seq=1` before reset is possible; when Cancel wins, then no started event exists at all.
- Given any staged session, when the beacon request, every event, every warning, and every returned error are inspected, then none contains the capability token or the source path.
- Given any Stage or claim path, when an adapter is called, then the coordinator's state mutex is provably unheld at that moment, because every fake checks it on entry and fails the test otherwise.
- Given the finished module, when build, vet, tests, race tests, formatting, and dependency verification run, then all pass with the QR dependency pinned to v1.1.0 and no unrelated version drift.

## Spec Change Log

## Design Notes

The mutex rule is proven by construction, not by inspection: every fake adapter tries the state mutex on entry and fails the test if it is held. A prose rule would survive a refactor that quietly moved a call inside the lock; this does not, and it costs nothing because every existing test then exercises it.

The rule is not stylistic: an adapter call under the state lock would let a slow mDNS registration or a blocked listener stall Cancel, and Cancel must be able to win at any moment. So each step is unlock, call, relock, revalidate — and the revalidation is what makes a stale result impossible to commit, not merely unlikely.

The operation lease is separate from the mutex on purpose. The mutex protects state for microseconds; the lease serializes long adapter work so Cancel and Shutdown join an existing teardown instead of racing a second one. One owner records the teardown result for every joiner.

The beacon is acquired last and released first because it is the only non-fatal resource: HTTP and QR ready with mDNS failed is a usable session with a warning, while the reverse is not a session at all.

`AuthorizeClaim` holds the lease *through* publication rather than releasing at the TRANSFERRING commit. That is what orders started before any reset Cancel could publish — releasing earlier would leave a window where reset precedes started and the frontend sees a terminal event for a transfer it never saw begin.

## Verification

**Commands:**
- `go mod tidy && go mod verify` — dependency graph unchanged; this story adds none.
- `go build ./... && go vet ./... && go test -count=1 ./...` — module and deterministic tests pass.
- `CGO_ENABLED=1 go test -race -count=1 ./...` — no state, sequence, or ownership race. Needs MinGW `bin` on PATH (see AGENTS.md).
- `go test -race -count=20 ./internal/transfer` — the claim race and cancellation races are concurrency tests; repeat them.
- `gofmt -l .` and `git diff --check` — no formatting or whitespace defects.
- `rg -n 'wails|net/http' internal/transfer --glob '!**/*_test.go'` — no output: the coordinator stays framework-independent.
