---
id: SPEC-connection-recovery
companions:
  - recovery-contract.md
  - ../spec-multiple-selected-items/selection-contract.md
  - ../spec-phone-to-desktop-receiving/receive-contract.md
  - ../../../docs/fairdrop-contracts.md
  - ../../../docs/fairdrop-architecture.md
sources: []
---

# Connection recovery

## Why

A desktop with several eligible networks can publish an address the nearby device cannot use. Changing networks also leaves a waiting session with an old code. Users need to recover without reselecting their files or receive folder, while active transfers remain protected.

## Capabilities

- **CAP-1**
  - **intent:** A user can inspect eligible local network endpoints for a waiting send or receive session and choose one.
  - **success:** A fresh list shows interface labels and IPv4 addresses without changing the live session, exposing paths or claiming remote reachability.
- **CAP-2**
  - **intent:** A user can refresh a waiting connection on the automatic or explicitly chosen network.
  - **success:** Recovery retires the old capability before publishing a fresh session, URL and QR; sources or the approved receive destination remain selected. A claim that wins the race proceeds without cancellation. Failed replacement never presents the retired code as usable.

## Constraints

- Only a matching waiting session may refresh. Claiming, transferring, terminal, stale and empty session identifiers cannot authorize retirement. Coordinator ownership decides races, not frontend visibility or the HTTP reservation alone.
- Existing bounded cleanup must prove quiescence before replacement. No overlapping listener or beacon, and no success when cleanup remains unconfirmed.
- Automatic selection preserves existing ranking and eligibility. Explicit choices are matched against fresh eligible interface/address pairs; no arbitrary endpoint or silent fallback.
- Keep network choice in process memory only. Preserve one peer, plain HTTP disclosure, token secrecy and metadata-only GET behavior.
- Preserve the 10,000-root lazy collection contract and every receive write-safety guarantee. Receive recovery transfers the existing destination capability after old HTTP work ends; it does not retain a new raw destination path or create output before upload.
- Follow recovery-contract.md for APIs, ownership, errors and accessible presentation.

## Non-goals

- Background network monitoring, connectivity probes, active-transfer migration, resume, IPv6, firewall changes, persistent preferences, text/link sharing, version changes or publication.

## Success signal

A waiting sender or receiver can switch eligible IPv4 endpoints or refresh automatic selection, scan a new code and transfer successfully; the old code cannot claim. Deterministic race and cleanup tests prove recovery cannot cancel a winning claim or conceal an unquiescent resource.

## Assumptions

- Recovery is explicit under Trouble connecting in waiting views. A new independently selected session starts with automatic network selection.
