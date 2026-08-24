---
title: 'Story 1.2 — Select and Advertise a Reachable LAN Endpoint'
type: 'feature'
created: '2026-08-23'
status: 'done'
review_loop_iteration: 0
baseline_commit: '89f4dc3536221fd7793a521afec47efb93aceef5'
context:
  - '{project-root}/docs/fairdrop-contracts.md'
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** FairDrop has only a Phase 1 network placeholder. It cannot deterministically choose a reachable IPv4 address or reliably publish/remove the non-sensitive `_fairdrop._tcp` record staging needs.

**Approach:** Replace the placeholder with the consumer-owned `NetworkPort`, a deterministic interface selector, and a synchronized `hashicorp/mdns` v1.0.7 adapter whose startup is transactional and whose shutdown is idempotent.

## Boundaries & Constraints

**Always:** Use the binding port/value shapes and existing coded errors. Require up+broadcast and reject loopback, point-to-point, or names containing `docker`, `veth`, or `tun` case-insensitively. Rank private, global-unicast, then link-local IPv4; tie-break by interface index, folded/original name, then numeric address. Cache the exact interface/address. Advertise only the fixed service, selected IPv4, process-unique non-persistent identity, and TXT `version=1`. Check context around synchronous calls. Failed Start cleans partial ownership; every Stop return owns no responder/socket.

**Ask First:** Any contract-shape change, dependency other than `github.com/hashicorp/mdns` v1.0.7, persistent identity, custom/forked mDNS responder, IPv6, or interface-selection UI.

**Never:** No hostname-based address lookup, loopback fallback, arbitrary TXT passthrough, SessionID/token/URL/filename/path disclosure, compatibility `NetworkManager`, coordinator/HTTP/QR/UI work, persistence, telemetry, or default-test reliance on multicast.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Ranked selection | Eligible candidates in any order | Same highest-ranked candidate/interface every time | N/A |
| Excluded candidates | Bad flags/name, IPv6, unspecified, multicast, loopback, broadcast | Ignore; never return them | `network_unavailable` if none remain |
| Partial enumeration failure | One interface errors; another works | Select usable candidate | Join causes only if none work |
| Cancelled selection | Nil/done/during enumeration context | No cached candidate | safe typed failure (`transfer_failed` for nil; otherwise `cancelled`) |
| Beacon success | Cached candidate; valid request | Exact interface/IP, safe unique names; active before return | N/A |
| Invalid or duplicate Start | No selection, bad request, or active beacon | No factory call or replacement/leak | `beacon_warning` |
| Failed/cancelled Start | Factory error, partial handle, or cancellation after creation | Shutdown partial handle; retain no active state | wrapped `beacon_warning` |
| Stop lifecycle | Before Start, after failure, repeated, concurrent, or cleanup error | Idempotent; handle forgotten and responder/socket closed on every return | cleanup diagnostic is `beacon_warning` |

</frozen-after-approval>

## Code Map

- `internal/transfer/ports.go` — preserve `SourcePort`; add `BeaconRequest`, `NetworkPort`, and single-source protocol constants for `_fairdrop._tcp` and `version=1`.
- `internal/network/network.go` — delete `NetworkManager`; `Manager` owns selection, process identity, beacon, seams, and mutex.
- `internal/network/address.go` — pure snapshot filtering/ranking over `net.Interfaces`/`Interface.Addrs`; never trust OS ordering.
- `internal/network/beacon.go` — validate, build explicit-IP/FQDN config with discard logger, and transactionally own `Shutdown`.
- `internal/network/*_test.go` — immutable OS/factory/hostname/entropy seams; no sleeps or default multicast.
- `go.mod`, `go.sum` — add only direct `github.com/hashicorp/mdns v1.0.7` through Go tooling; do not downgrade the existing `x/*` graph.
- `github.com/hashicorp/mdns@v1.0.7/server.go` — read-only evidence for readiness and shutdown limitations.
- `internal/source`, `internal/{server,stream}`, `app.go`, `main.go`, frontend — read-only/out of scope.

## Tasks & Acceptance

**Execution:**
- [x] `internal/transfer/ports.go` — add the exact consumer-owned network contract and protocol constants.
- [x] `internal/network/{network,address,beacon}.go` — implement selection, validation, explicit mDNS config, and race-clean lifecycle; remove the old interface.
- [x] `internal/network/address_test.go` — cover eligibility, ranking, cancellation, mapped IPv4, and reordered fixtures.
- [x] `internal/network/beacon_test.go` — capture disclosure/config and force failure, cancellation, duplicate Start, and teardown paths.
- [x] `go.mod`, `go.sum` — add/tidy/verify the pinned direct dependency without unrelated version drift.

**Acceptance Criteria:**
- Given any permutation of identical interface data, when `GetLocalIP` runs, then it returns the same highest-ranked canonical `netip.Addr` and no excluded address.
- Given successful selection and a valid request, when `StartBeacon` returns nil, then the registrar was configured with that exact interface/IPv4, `_fairdrop._tcp`, safe unique names, port, and only `version=1`.
- Given any failed Start or any Stop outcome, when the call returns, then the manager retains no beacon handle and repeated Stop is safe.
- Given the finished module, when build, vet, ordinary tests, race tests, formatting, dependency verification, and interface-surface checks run, then all pass with no `NetworkManager` remaining.

## Spec Change Log

- **Pre-review verification hardening (2026-08-23):** Matrix audit made the excluded-address typed failure, global-over-link-local ordering, link-local fallback, already-cancelled error code, and removal of a previously cached selection directly observable. These tests strengthen proof of the approved behavior without changing the frozen intent or contract surface.
- **Accepted review hardening (2026-08-23):** A context-aware lifecycle/selection gate now prevents Start from observing an in-progress reselection and prevents an active beacon's cached endpoint from being cleared or replaced. Review also removed address calls for excluded interfaces, fixed IPv4 network-address filtering while preserving `/31` and `/32` host semantics, normalized typed-nil responder handles, and added deterministic identity-failure and concurrent-start evidence.

## Design Notes

`BeaconRequest.Instance` is a validated base label; the adapter appends safe hostname and an injected 128-bit process suffix. `SessionID` is correlation-only and never advertised. Selection must precede Start so mDNS cannot diverge from the direct URL.

A capacity-one gate serializes selection with beacon Start/Stop without wrapping synchronous OS or upstream calls in goroutines. `GetLocalIP` and `StartBeacon` can abandon a gate wait through their contexts with their contract-specific error codes. Once a beacon is active, it owns the cached endpoint: a later `GetLocalIP` returns that exact address without enumeration until Stop completes. Responder ownership treats nil and typed-nil handles identically, including partial-start cleanup.

v1.0.7 `Shutdown` closes sockets but exposes no receive-goroutine join or TTL=0 goodbye; caches may retain records until TTL expiry. Unit tests prove FairDrop ownership/closure; fresh-query and eventual behavior require opt-in native evidence. Never claim immediate cache eviction.

## Review Disposition

- **Accepted:** active-endpoint immutability, cancellation-aware lifecycle gating, excluded-interface short-circuiting, IPv4 network-address filtering with RFC 3021 preservation, typed-nil responder defense, and missing identity/concurrency/tie-break tests. Each was an in-scope patch that made an approved invariant executable.
- **Rejected as contract conflicts:** requiring `FlagMulticast`, publishing/retaining `SessionID`, expanding the named virtual-interface blacklist, or removing the sanitized hostname. The approved selection/disclosure rules are explicit; mDNS failure remains a non-terminal warning while the selected HTTP endpoint can still be valid.
- **Rejected as already bounded:** v1.0.7 cannot prove goodbye-cache eviction or joined internal receive goroutines. This artifact explicitly limits its claim to FairDrop's responder/socket ownership and does not claim native multicast interoperability.

## Verification

**Commands:**
- `go mod tidy && go mod verify` — dependency graph is valid and pinned.
- `go build ./... && go vet ./... && go test -count=1 ./...` — module and deterministic tests pass.
- `CGO_ENABLED=1 go test -race -count=1 ./...` — lifecycle seams are race-clean using the installed MinGW toolchain.
- `gofmt -l .` and `git diff --check` — no formatting/whitespace defects. (An earlier note here claimed `gofmt -l .` does not recurse; that is incorrect — verified 2026-08-23 on go1.26.7 by planting a malformed file two directories down and seeing it reported.)
- `rg -n 'type NetworkManager interface' internal` — no output.
- `rg -n 'type NetworkPort interface|func \([^)]*\) (GetLocalIP|StartBeacon|StopBeacon)\(' internal --glob '*.go' --glob '!**/*_test.go'` — one port and one concrete implementation set.

**Verified 2026-08-23 on go1.26.7/windows/amd64:** Winget package `BrechtSanders.WinLibs.POSIX.UCRT` supplied GCC 16.1.0. With Go and its `mingw64/bin` on `PATH` plus `CGO_ENABLED=1`, dependency verification, build, vet, uncached ordinary tests, uncached race tests, formatting, whitespace, dependency pin, and interface-surface checks all passed. The ordinary suite reported 101 passing test events: 44 network, 32 source, 22 transfer-contract, and 3 application events. A separate `go test -race -count=20 ./internal/network` stress run also passed. All network evidence is deterministic and factory-injected; no native multicast integration result is claimed.

**Matrix Test Audit:**

| Row | Direct evidence | Result |
| --- | --- | --- |
| Ranked selection | rank tiers, isolated folded/original/numeric tie-breaks, reordered interfaces and addresses | Pass |
| Excluded candidates | interface flags/name filters skip `Addrs`; IPv6, unspecified, loopback, multicast, subnet network, and broadcast addresses are rejected; both `/31` hosts and `/32` remain valid | Pass |
| Partial enumeration failure | one interface fails while another wins; all failure causes remain inspectable only when none work | Pass |
| Cancelled selection | nil, already-done, during enumeration, and while waiting behind an in-flight selection; inactive stale state is cleared without disturbing active/in-flight ownership | Pass |
| Beacon success | captured config proves exact interface/IP and safe fixed records; Start waits for committed selection, and active-beacon reselection returns the immutable cached endpoint without OS calls | Pass |
| Invalid or duplicate Start | missing selection, bad fields, nil/done/wait-cancelled context, active responder, and two concurrent starts yield one factory call and no replacement | Pass |
| Failed/cancelled Start | partial and typed-nil handles, post-creation cancellation, hostname failure, and short entropy retain no responder; applicable causes and cleanup are preserved | Pass |
| Stop lifecycle | before Start, after failed Start, repeated, concurrent, and cleanup-error paths retain no handle and shutdown once | Pass |

## Suggested Review Order

**Endpoint and lifecycle ownership**

- Begin with cached-endpoint immutability and cancellation-aware selection coordination.
  [`network.go:82`](../../internal/network/network.go#L82)

- Startup serializes with selection and retains ownership only after readiness.
  [`beacon.go:25`](../../internal/network/beacon.go#L25)

- Teardown forgets ownership once while synchronously closing the responder.
  [`beacon.go:101`](../../internal/network/beacon.go#L101)

- Typed-nil normalization prevents partial-start cleanup panics.
  [`beacon.go:243`](../../internal/network/beacon.go#L243)

**Reachable IPv4 policy**

- Pure ranking makes interface and address enumeration order irrelevant.
  [`address.go:23`](../../internal/network/address.go#L23)

- Host filtering rejects subnet endpoints while preserving valid `/31` semantics.
  [`address.go:119`](../../internal/network/address.go#L119)

- Enumeration avoids OS calls for adapters already excluded by policy.
  [`network.go:157`](../../internal/network/network.go#L157)

**Consumer-owned contract and dependency**

- Transfer owns the fixed beacon request and network port surface.
  [`ports.go:23`](../../internal/transfer/ports.go#L23)

- The sole new direct dependency remains pinned to reviewed v1.0.7 behavior.
  [`go.mod:6`](../../go.mod#L6)

**Defense evidence**

- Active discovery cannot silently diverge from a later direct URL.
  [`beacon_test.go:79`](../../internal/network/beacon_test.go#L79)

- Beacon startup waits for the exact in-flight selection to commit.
  [`beacon_test.go:121`](../../internal/network/beacon_test.go#L121)

- Cancelled callers abandon gate waits without blocking behind OS work.
  [`beacon_test.go:229`](../../internal/network/beacon_test.go#L229)

- Identity failures and concurrent starts retain no unsafe partial state.
  [`beacon_test.go:449`](../../internal/network/beacon_test.go#L449)

- Subnet and excluded-interface tests pin the reviewed reachability boundary.
  [`address_test.go:197`](../../internal/network/address_test.go#L197)
