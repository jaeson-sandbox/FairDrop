---
id: SPEC-fairdrop
companions:
  - ../../planning-artifacts/architecture/architecture-FairDrop-2026-08-22/ARCHITECTURE-SPINE.md
  - ../../../docs/fairdrop-contracts.md
  - ../../../docs/fairdrop-architecture.md
sources:
  - ../../../docs/fairdrop-spec.md
  - ../../implementation-artifacts/spec-phase-1-wails-scaffold.md
  - ../../implementation-artifacts/deferred-work.md
---

> **Canonical contract.** This SPEC and the files in `companions:` are the complete, preservation-validated contract for what to build, test, and validate. Source documents listed in frontmatter are for traceability—consult them only for narrative rationale or prose color this contract intentionally omits.

# FairDrop

## Why

People need a quick way to move a local file or directory from a Windows or macOS desktop to a nearby browser without accounts, cloud storage, installation on the receiver, or retained product data. FairDrop realizes that trusted-LAN handoff as a small, ephemeral desktop application whose behavior remains understandable and recoverable when different implementation agents continue the work.

## Capabilities

- **CAP-1**
  - **intent:** A sender can select exactly one local regular file or directory through native drag-and-drop or accessible browse controls.
  - **success:** One valid absolute path can stage; zero, multiple, missing, link-like, reparse, or special-file selections fail safely without silently choosing an item or disclosing its path.

- **CAP-2**
  - **intent:** A sender can create a receiver-reachable staged transfer on the local network.
  - **success:** Stage returns session metadata, a direct URL, and a QR code only after a random-port listener is ready; eligible IPv4 selection is deterministic, and mDNS failure is a safe warning when direct transfer remains usable.

- **CAP-3**
  - **intent:** One receiver can download the selected regular file through its capability URL.
  - **success:** The first exact-token GET receives the exact bytes and safe filename headers; a wrong method, route, or token receives 404, while a competing valid request receives 423 only while the listener remains live.

- **CAP-4**
  - **intent:** One receiver can download the selected directory as a browser-compatible ZIP without a staged archive.
  - **success:** The response is a valid archive with one top-level root, bounded memory, normalized safe entry names, a complete central directory, and explicit failure when a source becomes unsafe or invalid.

- **CAP-5**
  - **intent:** A sender can observe and control one transfer lifecycle.
  - **success:** Backend-authoritative session events report claim, honest wire progress, completion, or safe failure; Cancel and shutdown quiesce every owned resource, and terminal UI returns to idle after three seconds.

- **CAP-6**
  - **intent:** A sender can operate FairDrop through a clear, accessible desktop interface.
  - **success:** Distinct Idle, Staged, Transferring, Done, and Error presentations expose the QR code and URL, item name and size, determinate or indeterminate progress, throughput, visible focus, keyboard equivalents, and live announcements.

- **CAP-7**
  - **intent:** Maintainers can reproducibly verify and ship FairDrop on supported desktop platforms.
  - **success:** Locked Go and npm builds, unit and integration tests, race checks on capable native CI, Wails builds, and Windows and macOS smoke tests pass without relying on cross-compiled release claims.

## Constraints

- V1 permits one process, one live session, one selected root, and one receiver; only IDLE accepts Stage, and multi-selection is rejected.
- Runtime product state is ephemeral: no database, settings, telemetry, persistent logs, cloud service, or payload archive is written.
- Payload memory remains O(buffer) regardless of payload size; files and ZIPs stream with prompt context cancellation and no whole-payload reads.
- V1 is trusted-LAN plain HTTP with a separate cryptographically random capability token of at least 128 bits. The receiver uses a modern browser on the same LAN, and the sender permits the operating-system firewall access required for inbound HTTP.
- Capability tokens and source paths never enter mDNS, diagnostics, unrelated HTTP errors, or persistent storage; receiver-visible names are sanitized as specified by the binding contracts.
- The coordinator is the sole lifecycle owner. External calls occur outside its mutex, state commits are generation-checked, and every Stop is idempotent, force-closing, and quiescent on every return.
- Backend events and typed errors are authoritative and session-scoped. Ordering, sequence, progress, public DTO, HTTP, and disclosure semantics are exactly those in the binding contracts companion.
- Preserve the proven Wails v2 native drop boundary, standard OS chrome, application-lifetime Wails context, single-instance restoration, and accessibility rules defined by the architecture companions.
- Preserve spaces, Unicode, long Windows paths, and UNC paths wherever native Go APIs permit. Never shell-interpolate paths; reject symlinks, reparse traversal, and non-regular files with typed errors.
- Implementation follows the final architecture spine and contracts. Evidence-driven divergence updates this spec, the architecture decision log and documents, the relevant phase artifact, tests, and managed agent context together.

## Non-goals

- Multi-item staging, collision rules, multi-root archives, and multiple receivers.
- TLS, authenticated discovery, hostile-network security, cloud relay, or a native receiver application.
- IPv6, an interface-selection UI, transfer resume or ranges, transfer history, or persistent preferences.
- Linux packaging, installers, signing, notarization, auto-update, or default UPX compression.

## Success signal

On native Windows and macOS, a sender can choose one file and one directory in separate sessions, and a nearby browser downloads the exact file bytes and a valid ZIP. Progress and cancellation remain honest, every session returns to a leak-free IDLE state, and FairDrop retains no product data.
