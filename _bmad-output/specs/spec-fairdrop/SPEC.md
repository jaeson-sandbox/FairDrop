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

> **Owner policy, 2026-09-11:** `docs/release-policy.md` supersedes earlier
> mandatory-human-release wording in this document. Automated verification remains
> required; manual device/browser, screen-reader, firewall and visual observations
> are optional for personal releases. Unobserved behavior is not a verified pass,
> and known functional failures are not waived.

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
- Payload memory remains O(buffer) in payload bytes regardless of payload size; files and ZIPs stream with prompt context cancellation and no whole-payload reads. A streamed ZIP additionally retains the one central-directory record per entry that the format requires (measured at ~250 bytes each) and never a second per-entry index of its own.
- V1 is trusted-LAN plain HTTP with a separate cryptographically random capability token of at least 128 bits. The receiver uses a modern browser on the same LAN, and the sender permits the operating-system firewall access required for inbound HTTP.
- Capability tokens and source paths never enter mDNS, diagnostics, unrelated HTTP errors, or persistent storage; receiver-visible names are sanitized as specified by the binding contracts.
- The coordinator is the sole lifecycle owner. External calls occur outside its mutex, state commits are generation-checked, and every Stop is idempotent and force-closing. Teardown waits are bounded: successful teardown proves quiescence; an elapsed bound reports a coded failure and never claims the underlying resource has stopped (Story 3.4).
- Natural transfer completion follows successful HTTP final body/framing writes, including chunk termination; a final-write error or short write is failure. Cancel/Stop force-close pending finalization within documented teardown bounds. Connection wrapping preserves TCP half-close for responses rejecting unread bodies or oversized headers.
- Metadata inspection grants no content-read access: Windows uses attribute handles, Linux `O_PATH`, and Darwin parent-relative no-follow stat snapshots. Darwin snapshots do not pin a leaf; later search/enumeration/content descriptors must match their device/inode, generation and birth timestamp before use. Filesystems exposing identical or zero generation/birth fields retain a residual snapshot fingerprint collision risk. Leaf and traversal no-follow guards remain mandatory.
- Selection ancestors resolve only after coordinator admission, before raw inspection; the final leaf and existing syntax refusals are preserved. Cancellation releases the waiter without claiming the OS call stopped. At most one unresolved call per selection decorator remains outstanding; retries refuse busy until it returns, and streaming receives the canonical staged path through the raw inspector.
- Backend events and typed errors are authoritative and session-scoped. Ordering, sequence, progress, public DTO, HTTP, and disclosure semantics are exactly those in the binding contracts companion.
- Preserve the proven Wails v2 native drop boundary, standard OS chrome, application-lifetime Wails context, single-instance restoration, and accessibility rules defined by the architecture companions.
- Preserve ordinary spaces/Unicode, long Windows paths, and UNC paths wherever native Go APIs permit, subject to the portable archive-segment refusals in the contracts. Never shell-interpolate paths; reject symlinks, reparse traversal, and non-regular files with typed errors.
- Directory preparation pins one search-only root identity until Close, revalidated before walking; this protects Prepare-to-WriteTo replacement, not original Stage or unsnapshotted contents. Traversal retains at most 64 directory handles including ancestors and the pin, plus three transient handles. Borrowed reader revocation joins admitted native reads before owned close. Unsafe archive segments are refused at source and ZIP boundaries, with explicit 0755 directory/0644 file modes; no cross-entry collision or permission-preserving backup promise is made. Consecutive empty reads fail at 101 and reset on progress; native blocking I/O remains non-interruptible.
- Implementation follows the final architecture spine and contracts. Evidence-driven divergence updates this spec, the architecture decision log and documents, the relevant phase artifact, tests, and managed agent context together.

## Non-goals

- Multi-item staging, collision rules, multi-root archives, and multiple receivers.
- TLS, authenticated discovery, hostile-network security, cloud relay, or a native receiver application.
- IPv6, an interface-selection UI, transfer resume or ranges, transfer history, or persistent preferences.
- Linux packaging, installers, signing, notarization, auto-update, or default UPX compression.

## Success signal

On native Windows and macOS, a sender can choose one file and one directory in separate sessions, and a nearby browser downloads the exact file bytes and a valid ZIP. Progress and cancellation remain honest, every session returns to a leak-free IDLE state, and FairDrop retains no product data.
