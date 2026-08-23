---
stepsCompleted: [1, 2, 3, 4]
inputDocuments:
  - "{project-root}/_bmad-output/specs/spec-fairdrop/SPEC.md"
  - "{project-root}/_bmad-output/planning-artifacts/architecture/architecture-FairDrop-2026-08-22/ARCHITECTURE-SPINE.md"
  - "{project-root}/docs/fairdrop-contracts.md"
  - "{project-root}/docs/fairdrop-architecture.md"
  - "{project-root}/docs/fairdrop-spec.md"
  - "{project-root}/_bmad-output/implementation-artifacts/spec-phase-1-wails-scaffold.md"
  - "{project-root}/_bmad-output/implementation-artifacts/deferred-work.md"
---

# FairDrop - Epic Breakdown

## Overview

This document provides the complete epic and story breakdown for FairDrop, decomposing the canonical SPEC, its binding architecture companions, the implemented Phase 1 foundation, and preserved review findings into implementable stories.

> **Authority note.** `_bmad-output/specs/spec-fairdrop/SPEC.md` and its `companions:` are the canonical contract. The original product and Phase 1 documents are traceability sources only where they do not conflict with that contract.

## Requirements Inventory

### Functional Requirements

FR1: Accept exactly one absolute path for a regular file or directory from the proven Wails native drop boundary.

FR2: Provide keyboard-reachable native Select File and Select Directory actions without staging automatically; a cancelled dialog produces no transfer error.

FR3: Reject zero or multiple dropped paths, empty or missing paths, symbolic links, Windows reparse points, and non-regular special files with a stable safe error; never select the first item silently.

FR4: Inspect the selected root and produce immutable staged metadata containing session ID, name, logical size, directory flag, direct URL, QR PNG base64, and non-fatal warnings.

FR5: Select a LAN-routable IPv4 address deterministically from injected interface data, requiring an up, broadcast-capable, non-loopback, non-point-to-point interface and excluding names containing `docker`, `veth`, or `tun`.

FR6: Start the transfer listener on `0.0.0.0:0`, report the assigned port only after the accept loop is ready, and clean every partial resource if startup fails.

FR7: Generate a session ID and a separate single-use capability token, each with at least 128 random bits from a cryptographically secure source.

FR8: Construct the capability URL and encode it as a padded base64 PNG QR code entirely in memory.

FR9: Advertise a staged transfer as `_fairdrop._tcp` with a unique non-sensitive instance and protocol-version metadata; if advertisement fails after HTTP and QR are ready, keep the direct transfer usable and return a warning.

FR10: Accept only an exact-token GET on the binding download route, disguise wrong methods/routes/tokens as 404, reserve the first valid claim atomically, and return 423 to a competing valid request only while the listener remains live.

FR11: Stream a selected regular file from an opened and revalidated descriptor to the receiver in bounded chunks, using that descriptor for Content-Length.

FR12: Stream a selected directory as a valid ZIP through `io.Pipe` with one top-level root, safe normalized entry names, and no temporary archive; close the ZIP writer before the pipe writer.

FR13: Send sanitized attachment headers, `Cache-Control: no-store`, `Access-Control-Allow-Origin: *`, and `X-Content-Type-Options: nosniff`; omit Content-Length for directory streams.

FR14: Enforce the backend lifecycle IDLE → STAGING → STAGED → CLAIMING → TRANSFERRING → DONE/ERROR → IDLE, exposing only the public states and accepting Stage only from IDLE.

FR15: Stop mDNS before committing TRANSFERRING or publishing `transfer-started`, while allowing safe cleanup diagnostics that do not imply a live beacon.

FR16: Report wire bytes, known or unknown total, finite percentage, and rolling bytes-per-second at no more than four progress events per second plus required terminal progress.

FR17: Publish session-scoped `transfer-started`, `transfer-progress`, `transfer-complete`, `transfer-error`, and `transfer-reset` events through one FIFO lane beginning at sequence 1, following the binding success, failure, and cancellation grammars.

FR18: Cancel staging or transfer on user request, make cancellation win or lose at the defined state linearization point, quiesce all resources, suppress cancellation-as-transfer-error, and return success after teardown.

FR19: On natural completion or failure, close all live transfer resources before entering the terminal UI lease, publish the prescribed terminal events, and reset to IDLE after three seconds; shutdown performs the same teardown without waiting for that timer.

FR20: Present distinct Idle, Staged, Transferring, Done, and Error experiences with item details, direct URL, QR code, honest progress and throughput, cancellation, safe warnings, and animated transitions.

FR21: Expose stable typed public errors and warnings through the Wails command and event boundary without leaking arbitrary adapter text, source paths, or capability tokens.

FR22: Enforce a single FairDrop process and restore/show the existing window when a second launch is attempted.

### NonFunctional Requirements

NFR1: Transfer memory remains O(buffer) in payload size; no whole-file read, payload-sized index, or staged ZIP is permitted.

NFR2: FairDrop persists no runtime product data: no database, settings, telemetry, persistent logs, cloud service, or payload archive.

NFR3: Cancellation and shutdown are prompt and leak-free; every listener, connection, beacon, file, pipe, goroutine, producer, callback, and timer has one owner and is quiescent before Stop returns.

NFR4: V1 is explicitly a trusted-LAN plain-HTTP product, not a confidential channel for hostile networks; user and release copy must not imply end-to-end security.

NFR5: Capability and session entropy uses `crypto/rand`-backed injectable sources; the capability token is never persisted or advertised through mDNS.

NFR6: HTTP handling bounds request headers, maximum header size, and idle/read time without imposing a whole-transfer write deadline; requests have no body and v1 has no range/resume behavior.

NFR7: Filesystem traversal cannot escape the selected root, follow link-like entries, emit absolute/traversal ZIP names, or append an error body after a partial payload.

NFR8: Progress values are finite and wire-accurate; known positive totals use a clamped 0–100 percentage, while unknown and known-empty totals use zero and render indeterminately where appropriate.

NFR9: Lifecycle races are deterministic and testable: no external port is called while holding the state mutex, Stage and claim revalidate after unlocked calls, stale generations are ignored, and one operation lease serializes setup and teardown.

NFR10: All pointer and keyboard actions have semantic equivalents, visible focus, and screen-reader announcements; native dialog cancellation is quiet and event listeners are cleaned on unmount.

NFR11: Preserve spaces and Unicode and support long Windows and UNC paths wherever native Go filesystem APIs permit; unsupported native cases return typed errors without shell invocation or destructive rewriting.

NFR12: Build and smoke-test native Windows amd64 and macOS artifacts; Linux packaging is outside the current scope.

NFR13: Dependency and toolchain installation is reproducible from locked Go and npm metadata, including development dependencies required for TypeScript, Vite, tests, and Wails builds.

NFR14: Tests cover boundary behavior, teardown and claim races, path classes, ZIP integrity, bounded-memory evidence, HTTP protocol behavior, reducer correlation, accessibility, and native Wails configuration.

### Additional Requirements

- The Phase 1 Wails v2 and React/TypeScript/Tailwind scaffold already exists; preserve its proven native file-drop, standard-frame, lifecycle-hook, and clean-clone embed contracts instead of regenerating the project.
- Use ports and adapters around `internal/transfer.Coordinator`; `main.go` composes concrete adapters and `app.go` only translates Wails commands and notifications.
- Replace the Phase 1 provider-owned `NetworkManager`, `Streamer`, and `TransferServer` interfaces before their first implementation with the consumer-owned ports in `docs/fairdrop-contracts.md`; do not retain duplicate or conversion-only shadow interfaces.
- `docs/fairdrop-contracts.md` is binding for domain values, error codes, port signatures, state results, event payloads, claim ordering, HTTP outcomes, source-mutation rules, disclosure, and teardown postconditions.
- Stage setup is transactional: acquire resources in the documented order, revalidate after each unlocked adapter call, commit STAGED only when required resources are ready, and unwind failures in reverse order without lifecycle events before acknowledgement.
- Claim authorization uses the mutex-protected TRANSFERRING commit as its linearization point and holds the operation lease through synchronous `transfer-started` publication.
- The coordinator maintains a dedicated server-event drainer while Stop runs; event delivery cannot block teardown, and server handlers never invoke coordinator teardown inline.
- After payload preparation, the server owns exactly one `PreparedPayload.Close`; cancellation closes the HTTP destination, waits for writers/workers, then closes the payload without racing `WriteTo`.
- Use `internal/transfer.ErrorCode`, `DomainError`, and the `errors.As`-compatible coded-error contract. Wails `options.App.ErrorFormatter` serializes safe `{code,message}` JSON for frontend parsing.
- Register the methodless Go `http.ServeMux` pattern `/download/{token}` and explicitly require `request.Method == http.MethodGet`; a method-qualified pattern is forbidden because its 405 and implicit HEAD behavior violate FR10.
- Retain Wails v2.15.0 and the existing locked frontend stack; use `github.com/hashicorp/mdns` v1.0.7 and `github.com/boombuler/barcode` v1.1.0, superseding the original inactive QR dependency.
- Pin Node 24 LTS in development and CI, use `npm ci` with development dependencies, and migrate both TypeScript projects together to `moduleResolution: "Bundler"` with compatible target/lib settings and locked Node types.
- Add Wails single-instance locking and pin its stable identifier and second-launch window restoration behavior in `main_test.go` alongside existing option assertions.
- Run Go tests and vet, frontend tests and build, and `wails build`; run `go test -race ./...` on a native CI runner with a C toolchain, and smoke-test releases on native Windows and macOS runners.
- Treat buffer size and per-entry ZIP compression as Phase 3 benchmark choices that may change only while bounded memory, prompt cancellation, and archive compatibility remain proven.

### UX Design Requirements

> No dedicated BMAD UX contract exists. These actionable requirements come from the canonical SPEC and architecture companions.

UX-DR1: The Idle view provides one clearly marked native drop target plus semantic, keyboard-reachable Select File and Select Directory actions with visible focus.

UX-DR2: The drop adapter accepts exactly one path; zero or multiple paths show a safe validation error and never stage the first item silently.

UX-DR3: The Staged view shows the sanitized item name, human-readable logical size, QR code, copyable direct URL, non-fatal warnings, and a Cancel action.

UX-DR4: The Transferring view shows wire bytes and throughput, a determinate progress bar only for known positive totals, an indeterminate treatment for directory and unknown totals, and a Cancel action.

UX-DR5: Done and Error presentations use backend-authoritative events, show safe outcome text, announce changes through `aria-live`, and visibly reset to Idle after the backend `transfer-reset` event.

UX-DR6: State changes use restrained CSS or Framer Motion transitions without making animation callbacks authoritative for backend lifecycle state; reduced-motion preferences remain usable.

UX-DR7: The React reducer initializes a session only from successful Stage metadata, ignores stale session IDs and non-increasing sequence values, and ignores obsolete Stage promises after unmount or local request cancellation.

UX-DR8: The Wails file-drop listener registers with `OnFileDrop(callback, true)`, cleans up with `OnFileDropOff()`, and targets the inherited `--wails-drop-target: drop` property rather than a DOM drop handler or class-only gate.

### FR Coverage Map

FR1: Epic 1 - Accept one native dropped file or directory path.
FR2: Epic 1 - Provide native Select File and Select Directory controls as part of the first usable transfer slice.
FR3: Epic 1 - Reject invalid, multiple, missing, link-like, and special selections safely.
FR4: Epic 1 - Produce complete immutable staged metadata.
FR5: Epic 1 - Select the eligible LAN IPv4 address deterministically.
FR6: Epic 1 - Start a ready random-port LAN listener transactionally.
FR7: Epic 1 - Generate independent cryptographic session and capability identities.
FR8: Epic 1 - Build the capability URL and in-memory QR PNG.
FR9: Epic 1 - Advertise non-sensitive discovery metadata with recoverable warnings.
FR10: Epic 1 - Enforce exact-token, one-receiver HTTP claim behavior.
FR11: Epic 1 - Stream a revalidated regular file in bounded chunks.
FR12: Epic 2 - Stream a directory as a safe valid ZIP without staging it.
FR13: Epic 1 - Apply safe download, cache, CORS, and content headers.
FR14: Epic 1 - Enforce the backend-authoritative transfer state machine.
FR15: Epic 1 - Remove discovery before committing the transfer claim.
FR16: Epic 1 - Report honest throttled wire progress and throughput.
FR17: Epic 1 - Publish ordered, session-scoped lifecycle events.
FR18: Epic 1 - Cancel and quiesce staging or transfer deterministically.
FR19: Epic 1 - Complete terminal teardown and timed reset behavior.
FR20: Epic 1 - Present the complete animated and accessible transfer experience.
FR21: Epic 1 - Expose stable safe command and event errors.
FR22: Epic 3 - Enforce one process and restore the existing window.

## Epic List

### Epic 1: Share One File with a Nearby Device

A sender can select one file through native drop or keyboard-accessible browse controls, receive a QR code and direct URL, and let exactly one nearby browser download it through a complete, clear lifecycle experience with honest status and cancellation.

**FRs covered:** FR1-FR11, FR13-FR21

### Epic 2: Share One Folder with a Nearby Device

A sender can share a directory through the established transfer workflow, and the receiver obtains a valid ZIP without a temporary archive or payload-sized memory.

**FRs covered:** FR12

### Epic 3: Run FairDrop Reliably on Supported Desktops

Maintainers can verify and ship a single-instance FairDrop application through reproducible native Windows and macOS builds.

**FRs covered:** FR22, plus CAP-7 and release/verification NFRs

> **Cross-cutting rule.** FR coverage identifies primary epic ownership, not the full acceptance surface. Every story also inherits the relevant NFRs, architecture requirements, binding contracts, and UX requirements. Unit, integration, race, and accessibility verification lands with the behavior it proves; Epic 3 owns native packaging, release automation, platform smoke tests, and single-instance release behavior rather than deferred testing debt.

## Epic 1: Share One File with a Nearby Device

A sender can select one file through native drop or keyboard-accessible browse controls, receive a QR code and direct URL, and let exactly one nearby browser download it through a complete, clear lifecycle experience with honest status and cancellation.

### Story 1.1: Validate and Describe One File Selection

As a sender,
I want FairDrop to validate and describe my selected file before opening network resources,
So that unsupported or unsafe input fails early with a clear, non-sensitive error.

**Acceptance Criteria:**

**Given** an absolute path to an existing regular file
**When** the source adapter inspects it with a live context
**Then** it returns an immutable `StagedItem` containing the private absolute path, basename, `ItemFile` kind, logical byte size, and modification time
**And** it performs no persistence, shell invocation, network activity, or whole-file read.

**Given** an empty path
**When** it reaches the transfer input boundary
**Then** it returns a coded `invalid_selection` error
**And** no source or network resource is acquired.

**Given** a missing path
**When** it is inspected
**Then** it returns `path_not_found` through the canonical coded-error contract
**And** its public message contains no absolute path.

**Given** a symbolic link, Windows junction/reparse point, directory, or non-regular special file
**When** it is inspected for this file-transfer slice
**Then** it returns `path_unsupported` without following or opening the target
**And** later directory support can extend the adapter without changing the consumer-owned `SourcePort`.

**Given** a supported path containing spaces or Unicode
**When** native Go filesystem APIs can open it
**Then** inspection preserves the path as a value without rewriting or shell interpolation
**And** tests cover the supported path classes on capable native runners.

**Given** a `DomainError` wrapped one or more times with `%w`
**When** `ErrorCodeOf` or `PublicErrorOf` examines it
**Then** `errors.As` preserves its stable `ErrorCode`
**And** an unknown non-nil error maps to `transfer_failed` with a fixed safe message rather than arbitrary adapter text.

**Given** the new `internal/transfer` package
**When** its public contract is compared with `docs/fairdrop-contracts.md`
**Then** `SessionID`, `CapabilityToken`, `ItemKind`, `StagedItem`, `ErrorCode`, `CodedError`, `DomainError`, `PublicError`, and `SourcePort` preserve the binding meanings
**And** no conflicting shadow type or provider-owned source interface is introduced.

**Given** the story implementation
**When** Go tests and vet run
**Then** success, cancellation, empty, missing, link-like, special-file, safe-message, wrapping, and platform-appropriate path cases pass
**And** the relevant packages remain race-clean and compile with the existing Phase 1 scaffold.

### Story 1.2: Select and Advertise a Reachable LAN Endpoint

As a sender,
I want FairDrop to choose a reachable local-network identity and advertise a staged transfer safely,
So that a nearby receiver can discover the sender without exposing transfer secrets.

**Acceptance Criteria:**

**Given** injected interface data containing multiple addresses
**When** `GetLocalIP` evaluates candidates
**Then** it requires an interface that is up, broadcast-capable, non-loopback, and non-point-to-point and an address that is usable IPv4
**And** it excludes interface names containing `docker`, `veth`, or `tun`, case-insensitively.

**Given** multiple eligible candidates
**When** they are ranked
**Then** private LAN addresses rank ahead of other global-unicast addresses, which rank ahead of link-local fallback addresses
**And** ties resolve through stable interface and address ordering so identical input always produces the same result.

**Given** no eligible IPv4 candidate or a cancelled context
**When** address selection completes
**Then** it returns the applicable coded `network_unavailable` or `cancelled` error
**And** it never falls back to loopback, hostname resolution, or an excluded interface.

**Given** a valid `BeaconRequest`
**When** `StartBeacon` succeeds
**Then** `_fairdrop._tcp` is active before the method returns using `github.com/hashicorp/mdns` v1.0.7
**And** the instance name combines a safe hostname with a process-unique, non-persistent suffix.

**Given** the mDNS registration fields
**When** the record is inspected
**Then** TXT data contains only the protocol version and approved non-sensitive identity
**And** it contains no capability token, URL, selected filename, source path, or arbitrary metadata.

**Given** registration fails or its context is cancelled during startup
**When** `StartBeacon` returns
**Then** every partial registration is removed and the error is preserved as a coded `beacon_warning`
**And** no later `StopBeacon` call is required to finish that failed cleanup.

**Given** a beacon that is active, inactive, failed during startup, or has already been stopped
**When** `StopBeacon` is called one or more times
**Then** no advertisement remains before every return, including error returns
**And** repeated or pre-start calls are safe and idempotent.

**Given** the Phase 1 provider-owned `NetworkManager` interface
**When** this story implements networking
**Then** it is replaced by the consumer-owned `transfer.NetworkPort` and binding request types
**And** no duplicate compatibility interface remains.

**Given** the completed adapter
**When** Go tests, vet, and race checks run
**Then** injected fixtures cover loopback, VPN/tunnel names, down interfaces, public/private/link-local ranking, ties, no candidate, cancellation, registration failure, and repeated Stop
**And** tests do not require a real LAN multicast environment except for an explicitly separated native integration check.

### Story 1.3: Prepare and Stream a Regular File Safely

As a receiver,
I want FairDrop to stream the selected file exactly as it exists when I claim it,
So that I receive correct bytes without the sender buffering or persisting the payload.

**Acceptance Criteria:**

**Given** staged metadata for a regular file
**When** `PayloadPort.Prepare` runs before response headers
**Then** it re-`Lstat`s the selected root, opens the file, stats the opened descriptor, and verifies regular-file kind, staged size, and modification time
**And** Content-Length can later be derived from that same open descriptor rather than stale staging metadata.

**Given** the source is missing, has changed kind/size/modification time, or has become link-like or unsupported
**When** payload preparation runs
**Then** it returns `path_not_found`, `source_changed`, or `path_unsupported` as applicable before headers are written
**And** it exposes no source path through the public error.

**Given** successful preparation
**When** the server queries the `PreparedPayload`
**Then** `DownloadName` returns the selected basename and `Size` returns the descriptor length with `known=true`
**And** an empty file is represented as a known zero-byte payload.

**Given** a prepared regular file and a live context
**When** `WriteTo` streams to a destination
**Then** the destination receives the exact file bytes through a reusable bounded buffer
**And** memory remains O(buffer) with no `io.ReadAll`, `os.ReadFile`, payload-sized allocation, temporary copy, or persistence.

**Given** cancellation or a destination write failure during streaming
**When** `WriteTo` observes it
**Then** it returns promptly with the applicable coded cancellation or transfer error
**And** it does not retry, append an error body, or leave a worker goroutine running.

**Given** a successful `Prepare`
**When** ownership passes to the server
**Then** exactly one idempotent `Close` releases the descriptor after `WriteTo` has ended
**And** the payload implementation does not close concurrently with `WriteTo`.

**Given** paths containing spaces, Unicode, or native-supported long Windows or UNC forms
**When** preparation and streaming run
**Then** native Go filesystem APIs receive the path unchanged as a value
**And** unsupported host cases return a typed error rather than invoking a shell or rewriting the path.

**Given** the Phase 1 provider-owned `Streamer` interface
**When** this story implements file streaming
**Then** it is replaced by the server-owned `PayloadPort` and `PreparedPayload` contracts implemented by `internal/stream`
**And** no duplicate public streaming interface remains.

**Given** the completed payload adapter
**When** Go tests, vet, race checks, and bounded-memory tests run
**Then** they cover exact bytes, empty files, source mutation, missing and unsupported paths, permissions, cancellation, destination failure, repeated Close, supported path classes, and goroutine exit
**And** payload memory does not grow proportionally with tested file size.

### Story 1.4: Serve a One-Shot Capability Download

As a receiver,
I want the capability URL to authorize one safe file download,
So that the intended browser receives the file while invalid or competing requests reveal nothing useful.

**Acceptance Criteria:**

**Given** a valid `ServerStartRequest` and live context
**When** `ServerPort.Start` succeeds
**Then** it binds `0.0.0.0:0`, starts its accept loop, and returns the assigned port and event channel only when ready
**And** any startup failure closes every partial listener, goroutine, and channel before returning `server_start_failed`.

**Given** the HTTP router
**When** it is configured
**Then** it registers the methodless Go `http.ServeMux` pattern `/download/{token}`, reads the token only through `request.PathValue("token")`, and explicitly requires `request.Method == http.MethodGet`
**And** method-qualified routing, manual path splitting, and a third-party router are not used.

**Given** a wrong method including HEAD, malformed route, oversized path, or wrong token
**When** the request reaches the server
**Then** it receives 404 without reserving the transfer, invoking authorization, opening the payload, or emitting a lifecycle event
**And** the response reveals no token, filename, or source detail.

**Given** two exact-token GET requests racing
**When** the first request reserves the transfer atomically
**Then** it alone invokes `AuthorizeClaim` synchronously
**And** the competing valid request receives 423 only while the reserved or claimed listener remains live.

**Given** a reserved request
**When** claim authorization is denied, cancelled, stale, or shutting down
**Then** no payload or response header is opened or written before denial
**And** the handler returns 404 if it can still respond, otherwise it closes the connection.

**Given** successful claim authorization
**When** payload preparation succeeds
**Then** the response uses a sanitized ASCII attachment fallback plus RFC 5987 `filename*`, `Cache-Control: no-store`, `Access-Control-Allow-Origin: *`, and `X-Content-Type-Options: nosniff`
**And** Content-Length is present only when `PreparedPayload.Size` reports a known length.

**Given** payload preparation fails after authorization but before headers
**When** the handler can still respond
**Then** it sends a generic 410 response without source details, queues exactly one `ServerFailed` event preserving the coded local cause, and closes the listener
**And** it does not expose the capability token or arbitrary adapter text.

**Given** payload bytes are being written
**When** progress is measured
**Then** only bytes successfully accepted by `http.ResponseWriter.Write` count toward `BytesSent`, progress is emitted at no more than four hertz, and coalescing cannot block the handler
**And** percentage and total semantics follow the binding known/unknown rules without NaN or infinity.

**Given** a successful known-length transfer
**When** the final byte is written
**Then** the server produces an authoritative terminal snapshot matching the prepared length followed by exactly one `ServerComplete` event
**And** it produces no later progress or terminal outcome.

**Given** a receiver disconnect or write failure after headers or bytes
**When** streaming fails
**Then** the server queues one `ServerFailed` event with final progress only if bytes were written and aborts through `http.ErrAbortHandler` or equivalent connection closure
**And** it never appends an error body to the partial payload.

**Given** a prepared payload on completion, failure, cancellation, or Stop
**When** the server tears down
**Then** it cancels the data-plane context, force-closes the HTTP destination, waits for `WriteTo` and workers, and calls exactly one payload `Close` afterward
**And** `Close` never races `WriteTo`.

**Given** `ServerPort.Stop` before Start, during a request, after termination, or repeatedly
**When** it returns, including with a cleanup diagnostic
**Then** the listener, active connection, handlers, payload workers, and event producers are quiescent and the event channel is closed permanently
**And** terminal delivery cannot block teardown.

**Given** the Phase 1 provider-owned `TransferServer` interface
**When** this story implements the server
**Then** it is replaced by the consumer-owned `transfer.ServerPort` and server-owned payload contracts
**And** no duplicate compatibility interface remains.

**Given** the completed server
**When** Go tests, vet, and race checks run
**Then** they cover readiness, startup unwind, exact route and methods, token mismatch, first-claim races, 423 behavior, denial, headers, empty files, progress cadence, preparation failure, disconnects, forced abort, payload ownership, channel closure, and repeated Stop
**And** request-header, maximum-header, and idle/read limits are verified without a whole-transfer write deadline.

### Story 1.5: Stage and Authorize a Transfer Transactionally

As a sender,
I want FairDrop to stage my file only when every required transfer resource is ready,
So that the QR code and URL always represent one coherent, claimable session.

**Acceptance Criteria:**

**Given** the coordinator is IDLE and receives one valid file path
**When** Stage begins
**Then** it enters internal STAGING under the state mutex and acquires the session operation lease
**And** another Stage request returns `busy` without changing state or resources.

**Given** a new Stage operation
**When** session identity is created
**Then** the coordinator generates an independent `SessionID` and `CapabilityToken`, each from at least 128 bits through an injectable `crypto/rand`-backed source
**And** neither value is derived from the other or persisted.

**Given** a live Stage operation
**When** it acquires external resources
**Then** it inspects the source, resolves the LAN address, starts the ready HTTP server and event drainer, constructs the capability URL, encodes the QR PNG with `github.com/boombuler/barcode` v1.1.0, and finally attempts mDNS publication
**And** no external adapter call occurs while the state mutex is held.

**Given** any unlocked external Stage call completes
**When** the coordinator reacquires the mutex
**Then** it revalidates session ID, STAGING state, cancellation generation, and shutdown status before using the result or starting the next step
**And** a stale or cancelled result can never commit STAGED.

**Given** source inspection, network selection, server startup, or QR encoding fails before Stage acknowledgement
**When** Stage unwinds
**Then** it cancels the session and releases every acquired resource in reverse order through the single operation lease
**And** it returns to IDLE, returns the applicable coded command error, and emits no lifecycle event.

**Given** HTTP and QR resources are ready but mDNS publication fails
**When** Stage commits
**Then** it returns usable metadata with one safe `beacon_warning`, records no active beacon, and enters STAGED
**And** the warning array is non-null and contains no arbitrary adapter text.

**Given** all required setup succeeds
**When** the mutex-protected STAGED commit linearizes
**Then** Stage returns `FileMetadata` containing `sessionId`, name, logical size, `isDir=false`, direct URL, padded QR PNG base64 without a data-URI prefix, and an empty warnings array
**And** the capability token is present only in the local URL and QR, not in mDNS or diagnostics.

**Given** a matching exact-token request reaches `AuthorizeClaim` while STAGED
**When** authorization begins
**Then** the coordinator enters CLAIMING, holds the operation lease, stops mDNS synchronously without the state mutex, and reacquires the mutex to revalidate the session and cancellation generation
**And** it opens no payload and permits no response header before this handshake succeeds.

**Given** Cancel or Shutdown marks the session before the TRANSFERRING commit
**When** claim authorization revalidates
**Then** it returns `cancelled` or `shutting_down`, commits no transfer, and emits no `transfer-started` event
**And** the server writes no payload.

**Given** claim authorization wins the race
**When** the mutex-protected TRANSFERRING commit occurs
**Then** the coordinator synchronously publishes `transfer-started` with `seq=1` while still holding the operation lease, then returns authorization success
**And** a later Cancel cannot publish reset before started.

**Given** `StopBeacon` reports a cleanup diagnostic during claim
**When** its quiescent postcondition guarantees the advertisement is already gone
**Then** authorization may proceed safely and records only a non-sensitive internal diagnostic
**And** the diagnostic cannot imply or preserve a live beacon.

**Given** cancellation during each external setup step and both claim-race outcomes
**When** coordinator tests run with fake source, network, server, QR, observer, entropy, and clock ports
**Then** they prove reverse unwind, post-call revalidation, single-owner adapter operations, correct metadata, warning behavior, and exact Stage/claim state transitions
**And** Go race checks report no state, sequence, or resource-ownership race.

### Story 1.6: Complete, Cancel, and Reset the Transfer Lifecycle

As a sender,
I want every transfer outcome and cancellation to settle predictably,
So that FairDrop never leaves a listener, connection, beacon, worker, or stale UI session behind.

**Acceptance Criteria:**

**Given** the server event drainer receives progress for the matching TRANSFERRING session
**When** the coordinator accepts the snapshot
**Then** it publishes `transfer-progress` through the single synchronous emission lane with the next contiguous sequence number
**And** it ignores stale sessions, invalid states, and progress received after terminal acceptance.

**Given** a natural successful server outcome
**When** the coordinator accepts `ServerComplete`
**Then** it serializes teardown through the operation lease, quiesces the live server and beacon resources, publishes the authoritative final progress and then `transfer-complete`, and enters DONE
**And** exactly one terminal outcome is accepted for the session.

**Given** a natural failed server outcome
**When** the coordinator accepts `ServerFailed`
**Then** it quiesces live resources, preserves a recognized coded error or maps an unknown error to `transfer_failed`, publishes final progress only when bytes were written, then publishes `transfer-error` and enters ERROR
**And** no arbitrary adapter text, source path, or capability token reaches the public event.

**Given** the server event channel closes before a natural terminal event while TRANSFERRING
**When** coordinator-requested teardown is not active
**Then** the coordinator synthesizes one safe `transfer_failed` outcome and resets normally
**And** channel closure caused by Cancel or Shutdown is silent rather than an error.

**Given** Cancel while IDLE
**When** the command runs
**Then** it returns success without publishing an event
**And** no adapter method is called.

**Given** Cancel while STAGING before successful Stage acknowledgement
**When** cancellation marks the generation and cancels its context
**Then** it joins the existing reverse unwind, makes Stage return `cancelled`, enters IDLE, and emits no lifecycle event
**And** it never starts a second cleanup operation.

**Given** Cancel while STAGED or while CLAIMING before the TRANSFERRING commit
**When** cancellation wins the linearization race
**Then** authorization is denied, resources become quiescent, one `transfer-reset` is published, and the coordinator enters IDLE before Cancel returns
**And** no started, complete, or error event is published.

**Given** Cancel after the TRANSFERRING commit
**When** it wins before a natural terminal outcome
**Then** it cancels the data plane, waits for server and payload quiescence, discards queued server outcomes, and terminates any delivered `started, progress*` prefix with one reset
**And** it returns nil after quiescence without publishing completion or cancellation-as-error.

**Given** DONE or ERROR
**When** the application-lifetime three-second timer fires for the current generation
**Then** the coordinator publishes `transfer-reset`, clears the terminal session, and enters IDLE
**And** a stale timer from an older session cannot mutate the current state.

**Given** Cancel during DONE or ERROR
**When** it runs before the reset timer
**Then** it cancels the timer, publishes reset, clears the session, and returns from IDLE
**And** the cancelled timer cannot publish a second reset.

**Given** application shutdown in any state
**When** `Shutdown` runs
**Then** it sets the application-lifetime closing flag, rejects new commands with `shutting_down`, cancels terminal and live contexts, joins all teardown, suppresses further UI events, and returns only when resources are quiescent
**And** cleanup diagnostics are recorded through a non-sensitive internal sink rather than retaining ownership.

**Given** progress, completion, failure, cancellation, reset, and shutdown races
**When** coordinator tests use fake event channels and an injected clock
**Then** every valid event grammar, sequence starting at one, stale-generation rejection, single terminal outcome, timer generation check, and Stop ordering is deterministic
**And** `go test -race` proves the drainer, state lane, operation lease, and teardown path do not race or deadlock.

### Story 1.7: Expose Safe Transfer Commands through Wails

As a sender,
I want native selection and transfer controls to reach the backend safely,
So that desktop actions and lifecycle updates use one authoritative transfer implementation.

**Acceptance Criteria:**

**Given** application construction in `main.go`
**When** concrete adapters are composed
**Then** the Wails `App` receives one coordinator and `main.go` remains the composition root
**And** `app.go` contains only Wails command, dialog, lifecycle, and event translation rather than HTTP, filesystem, discovery, or transfer-state logic.

**Given** a valid absolute path from the frontend
**When** `StageTransfer(absolutePath string)` runs
**Then** it delegates to the coordinator and returns the binding `FileMetadata` shape unchanged
**And** busy, validation, setup, and cancellation failures remain typed safe command errors.

**Given** a Cancel request
**When** `CancelTransfer()` runs
**Then** it delegates to the coordinator's quiescent Cancel operation
**And** it returns success after the requested state is IDLE rather than surfacing cleanup diagnostics as a contradictory command failure.

**Given** the Idle view invokes `SelectFile` or `SelectDirectory`
**When** the native Wails dialog returns a path
**Then** the command returns that path without staging it automatically
**And** a cancelled dialog returns an empty selection without a transfer error or lifecycle event.

**Given** a coordinator event
**When** the App's synchronous `Observer.Publish` adapter receives it
**Then** it maps the internal event kind to the matching `transfer-*` runtime event and emits only the event-specific public payload through the application-lifetime Wails context
**And** it preserves `sessionId`, `seq`, progress, and safe error fields without exposing the internal `Kind` field.

**Given** a coded command failure
**When** Wails invokes `options.App.ErrorFormatter`
**Then** it returns a JSON string containing the validated `{code,message}` `PublicError`
**And** malformed or unknown errors use the fixed `transfer_failed` fallback with no raw adapter text.

**Given** application startup and shutdown
**When** Wails invokes the lifecycle hooks
**Then** startup stores the application-lifetime runtime context solely for Wails APIs and shutdown delegates to the coordinator's idempotent `Shutdown`
**And** a transfer child context never replaces `App.ctx`.

**Given** the existing native drop integration
**When** Wails options and frontend runtime bindings are updated
**Then** `DragAndDrop.EnableFileDrop`, standard OS chrome, normal start state, `OnFileDrop(callback, true)`, `OnFileDropOff()`, and inherited `--wails-drop-target: drop` remain intact
**And** generated `frontend/wailsjs/**` files are regenerated through Wails rather than edited manually.

**Given** the Wails boundary implementation
**When** Go tests, frontend contract tests, and `wails build` run
**Then** command delegation, safe error formatting, event payload mapping, native-dialog cancellation, lifecycle hooks, and existing option assertions pass
**And** `main_test.go` pins the new `ErrorFormatter` contract alongside the proven Phase 1 options.

### Story 1.8: Manage Session-Scoped Frontend State and Events

As a sender,
I want the frontend to follow the backend's authoritative transfer session,
So that stale promises or events cannot show the wrong transfer state.

**Acceptance Criteria:**

**Given** the frontend transfer module
**When** its public types are defined
**Then** metadata, warnings, public errors, progress snapshots, and event payloads match the lower-camel-case binding contract
**And** unknown totals and optional event fields are represented explicitly rather than inferred from zero or missing values.

**Given** Stage is pending
**When** the frontend waits for its promise
**Then** it may record a local pending state but does not initialize STAGED until successful metadata returns
**And** an obsolete promise after unmount or local request cancellation cannot initialize a session.

**Given** successful Stage metadata
**When** the reducer initializes the transfer
**Then** it stores the returned `sessionId`, sets `lastSeq=0`, and derives the Staged state from backend metadata
**And** command failure leaves no active backend session in the reducer.

**Given** lifecycle events for the active session
**When** the reducer processes them
**Then** it accepts only the matching `sessionId` and a `seq` greater than `lastSeq` and applies the binding started, progress, complete, error, and reset transitions
**And** it ignores stale sessions, duplicates, regressions, invalid payload shapes, and progress after terminal acceptance.

**Given** known, empty, or unknown progress
**When** frontend selectors derive presentation data
**Then** known positive totals preserve a finite clamped 0-100 value, while known-empty and unknown totals remain zero with the correct determinate discriminator
**And** no selector performs division that can produce NaN or infinity.

**Given** command rejection or `transfer-error`
**When** the frontend parses the public error
**Then** it validates `{code,message}` from rejected `Error.message` or event payload and falls back safely to `transfer_failed` for malformed data
**And** it never renders arbitrary object text, source paths, or capability tokens.

**Given** component mount, unmount, and remount
**When** Wails lifecycle listeners are managed
**Then** each `transfer-*` listener is registered once and cleaned up exactly once and no duplicate subscription survives
**And** reducer tests prove remounting does not duplicate state transitions.

**Given** frontend toolchain configuration
**When** this story establishes the transfer module and tests
**Then** both TypeScript projects use `moduleResolution: "Bundler"` with compatible ES target/lib settings and locked Node types, and `npm ci` installs all build and test dependencies
**And** Tailwind remains v4 through the Vite plugin and CSS import without Tailwind v3 or PostCSS configuration files.

**Given** the completed state and event layer
**When** frontend tests and TypeScript build run
**Then** they cover pending and failed Stage, session initialization, every valid event grammar, malformed payloads, stale correlation, sequence ordering, progress modes, safe-error fallback, terminal suppression, reset, unmount, and listener cleanup
**And** the module has no Wails-independent visual component requirement beyond the typed state it exposes.

### Story 1.9: Present the Accessible Transfer Experience

As a sender,
I want a clear and accessible view of each transfer stage,
So that I can select, share, monitor, cancel, and understand the outcome without guessing what the backend is doing.

**Acceptance Criteria:**

**Given** the Idle view
**When** it renders
**Then** it presents one clearly marked native drop target plus semantic keyboard-reachable Select File and Select Directory controls with visible focus
**And** it keeps the inherited `--wails-drop-target: drop` property and does not add a DOM drop handler or class-only gate.

**Given** a native drop callback containing zero or multiple paths
**When** the frontend validates the array
**Then** it shows a safe validation error and never selects the first item silently or calls Stage
**And** exactly one path is passed as a value to `StageTransfer`.

**Given** successful Stage metadata
**When** the Staged view renders
**Then** it shows the sanitized item name, human-readable size, QR image, copyable direct URL, non-fatal warnings, trusted-LAN guidance, and Cancel action
**And** it prepends `data:image/png;base64,` only when rendering the padded `qrBase64` value.

**Given** the reducer enters Transferring
**When** the view renders progress
**Then** known positive file totals use a determinate bar while known-empty or unknown totals use the binding treatment, and wire bytes plus human-readable throughput remain visible
**And** Cancel remains available while the command/state contract permits it.

**Given** Done, Error, warning, cancellation, or backend reset state
**When** the corresponding presentation changes
**Then** safe outcome text is visible, changes are announced through an `aria-live` region, and only the reducer's accepted reset returns the view to Idle
**And** focus moves predictably without trapping keyboard or screen-reader users.

**Given** animated state transitions
**When** CSS or Framer Motion is used
**Then** animation callbacks never own backend lifecycle state and reduced-motion preferences remain usable
**And** standard OS window chrome and responsive minimum dimensions remain intact.

**Given** component unmount and remount
**When** the Wails native file-drop listener is managed
**Then** `OnFileDrop(callback, true)` is paired with exactly one `OnFileDropOff()` cleanup and no duplicate callback survives
**And** tests retain the Phase 1 proof for inherited CSS targeting and drops outside the target.

**Given** the complete Epic 1 experience
**When** component tests, accessibility checks, TypeScript build, Go integration tests, and `wails build` run
**Then** they cover native browse and drop shape, QR rendering, warnings, all visible states, progress modes, cancellation, safe errors, focus, live announcements, reduced motion, and cleanup
**And** a native smoke check proves one regular file can be selected and downloaded exactly once by a nearby browser.

## Epic 2: Share One Folder with a Nearby Device

A sender can share a directory through the established transfer workflow, and the receiver obtains a valid ZIP without a temporary archive or payload-sized memory.

### Story 2.1: Validate and Stage One Directory

As a sender,
I want FairDrop to validate and describe a selected directory,
So that I can stage a folder without following unsafe entries or building a payload-sized file index.

**Acceptance Criteria:**

**Given** an absolute path to an existing directory that is not link-like
**When** the source adapter inspects it
**Then** it returns `StagedItem` with `ItemDirectory`, the root basename, root modification time, and the sum of regular-file logical sizes
**And** it retains no per-entry index after inspection.

**Given** an empty directory
**When** it is inspected
**Then** it is a valid directory item with logical size zero
**And** later wire progress remains explicitly unknown rather than treating logical size as ZIP response length.

**Given** directory traversal encounters a symbolic link, Windows reparse point, junction, nested directory link, or non-regular special file
**When** preflight reaches that entry
**Then** it stops with `path_unsupported` without following the entry
**And** the public error reveals no relative or absolute source path.

**Given** traversal encounters a missing entry, permission failure, cancellation, or arithmetic overflow while totaling logical bytes
**When** inspection ends
**Then** it returns the applicable coded path, cancellation, or safe transfer error
**And** it retains no partial index or open handle.

**Given** spaces, Unicode, native-supported long Windows paths, or UNC roots
**When** the directory is inspected
**Then** traversal uses native Go filesystem APIs and preserves path values without shell invocation or destructive normalization
**And** platform-capable tests cover those path classes.

**Given** a valid directory Stage request
**When** the coordinator completes its existing transactional setup
**Then** returned metadata uses `isDir=true`, the logical display size, the directory name, capability URL, QR data, session ID, and warnings
**And** file staging behavior remains unchanged.

**Given** the extended source adapter and coordinator
**When** Go tests, vet, and race checks run
**Then** they cover empty and nested directories, logical sizing without retained indexing, unsafe entries, permission and cancellation failures, path classes, metadata, and reverse unwind
**And** memory used during preflight does not grow with the number or size of files beyond filesystem traversal overhead.

### Story 2.2: Stream a Safe Directory ZIP

As a receiver,
I want the selected folder delivered as a valid ZIP archive,
So that I can download it in a browser without the sender creating a temporary archive.

**Acceptance Criteria:**

**Given** a staged directory
**When** `PayloadPort.Prepare` succeeds
**Then** it revalidates the root as a non-link directory and returns a prepared payload with a sanitized `.zip` download name and `Size(known=false, bytes=0)`
**And** it creates no archive file and begins no full traversal before `WriteTo`.

**Given** `WriteTo` begins
**When** the archive worker traverses the directory through `io.Pipe`
**Then** every regular file is emitted beneath exactly one top-level root using relative `filepath.ToSlash` entry names
**And** absolute, volume-qualified, empty, dot-dot, or traversal-bearing ZIP names are rejected.

**Given** each entry reached during streaming
**When** it is opened
**Then** it is re-`Lstat`ed and rejected if missing, link-like, reparse, special, or outside the selected root
**And** additions, removals, or in-place changes may be observed only under the documented unsnapshotted v1 policy and fail safely when invalid.

**Given** successful archive production
**When** the writer finishes
**Then** every entry is closed promptly, `zip.Writer.Close()` completes before the pipe writer closes, and the receiver obtains a ZIP with a valid central directory
**And** empty directories and empty files remain representable in the resulting archive.

**Given** cancellation, source read failure, traversal failure, receiver disconnect, or destination write failure
**When** streaming aborts
**Then** both pipe ends, current entry, ZIP writer, worker, and prepared payload converge on one idempotent close path without deadlock
**And** post-header failure reaches the server as `ServerFailed` and forces connection abort without an appended error body.

**Given** directory progress
**When** bytes are written to the HTTP response
**Then** `bytesSent` and throughput report actual ZIP wire bytes while `totalKnown=false`, `totalBytes=0`, and `percent=0`
**And** the frontend uses its indeterminate progress treatment through completion or failure.

**Given** large directories and already-compressed files
**When** buffer size and per-entry ZIP method are chosen
**Then** the Phase 3 implementation records benchmark evidence for the choice while retaining O(buffer) payload memory, prompt cancellation, and archive compatibility
**And** no tuning choice changes the binding payload or progress contract.

**Given** the complete folder-transfer path
**When** Go tests, race checks, bounded-memory checks, ZIP compatibility tests, server integration tests, and a native smoke test run
**Then** they cover empty, nested, Unicode, long-path and UNC-capable roots, unsafe entries, mutations, permissions, cancellation, disconnects, close ordering, central-directory integrity, unknown progress, and goroutine exit
**And** a nearby browser downloads and opens a valid ZIP without any temporary payload file appearing on disk.

## Epic 3: Run FairDrop Reliably on Supported Desktops

Maintainers can verify and ship a single-instance FairDrop application through reproducible native Windows and macOS builds.

### Story 3.1: Enforce One Running FairDrop Instance

As a sender,
I want a second FairDrop launch to restore the existing window,
So that duplicate processes cannot create competing transfer sessions.

**Acceptance Criteria:**

**Given** application options are constructed
**When** single-instance locking is configured
**Then** Wails receives one fixed project UUID that remains stable across builds and launches
**And** the identifier is pinned by `main_test.go` rather than generated at runtime.

**Given** another FairDrop process is already running
**When** a second launch is attempted
**Then** the existing process handles `OnSecondInstanceLaunch`, calls `WindowUnminimise` and `WindowShow`, and retains its current transfer session
**And** the second process does not construct a competing coordinator, listener, or beacon.

**Given** the existing Wails options contract
**When** single-instance support is added
**Then** native drop, standard frame, normal start state, dimensions, lifecycle hooks, bindings, and `ErrorFormatter` remain unchanged
**And** option assertions fail if the unique ID or restoration callback is removed or changed unintentionally.

**Given** supported Windows and macOS native runners
**When** single-instance smoke checks launch FairDrop twice
**Then** only one application instance remains and the original window becomes visible and restored
**And** closing the application still performs coordinator shutdown exactly once.

### Story 3.2: Automate Reproducible Cross-Platform Verification

As a maintainer,
I want every change verified on native supported runners from locked toolchains,
So that the repository cannot silently drift away from a buildable, race-safe release.

**Acceptance Criteria:**

**Given** the GitHub repository
**When** CI is added under `.github/workflows/`
**Then** pull requests and protected-branch pushes run native Windows and macOS jobs with concurrency cancellation for superseded runs
**And** no job claims Linux or cross-compilation as release proof.

**Given** CI toolchain setup
**When** dependencies are installed
**Then** Go honors the module floor and the architecture-verified toolchain policy, Wails CLI is exactly v2.15.0, Node is pinned to 24 LTS, and npm uses the committed lockfile through `npm ci`
**And** dev dependencies remain installed because TypeScript, Vite, Vitest, and Wails frontend builds require them.

**Given** a fresh checkout whose generated Wails bindings may be stale or incomplete
**When** the verification sequence runs
**Then** a Wails build or binding-generation step occurs before the standalone frontend TypeScript build that imports those bindings
**And** `frontend/dist/.gitkeep` remains available for clean-clone Go embedding and is recreated by the existing postbuild hook.

**Given** each native CI job
**When** verification runs
**Then** it executes Go tests, vet, frontend tests, frontend build, and `wails build` using the repository scripts and documented PATH requirements
**And** failures stop the job with retained test/build diagnostics but no uploaded source paths, tokens, or payload data.

**Given** a native runner with the required C toolchain
**When** race verification runs
**Then** `go test -race ./...` covers coordinator, network, server, and streaming packages
**And** the workflow documents why the race job is native and C-toolchain-dependent rather than assuming every local Windows shell can run it.

**Given** dependency or toolchain changes
**When** lockfiles or pinned versions drift
**Then** CI uses deterministic clean installation and fails on incompatible Go, Node, TypeScript, Vite, Vitest, Wails, or generated-binding state
**And** no workflow silently substitutes `npm install`, `npm ci --omit=dev`, an unpinned Wails CLI, or default UPX compression.

### Story 3.3: Produce and Smoke-Test Native Release Artifacts

As a maintainer,
I want native FairDrop artifacts built and exercised on each supported operating system,
So that users receive a desktop application whose actual transfer journey has been verified on its target platform.

**Acceptance Criteria:**

**Given** a versioned release candidate
**When** the release workflow runs on native Windows and macOS runners
**Then** each runner builds its own FairDrop artifact through Wails with locked Go and npm dependencies
**And** the workflow does not present cross-compiled output as equivalent native verification.

**Given** produced release artifacts
**When** they are named and inspected
**Then** product name, executable/output name, window title, version metadata, and platform identity consistently use FairDrop
**And** no stale DeadDrop name or inactive QR dependency remains in shipped metadata or documentation.

**Given** release compression settings
**When** artifacts are built
**Then** UPX is opt-in rather than default because of documented Apple Silicon and Windows antivirus risks
**And** disabling UPX does not change functional acceptance.

**Given** the native smoke matrix
**When** a release candidate is exercised
**Then** it covers first launch and firewall guidance, native drop and browse, single-instance restoration, one exact file download, one valid directory ZIP download, progress, cancellation, terminal reset, and clean shutdown
**And** Windows path classes and macOS filesystem behavior are checked where the runner supports them.

**Given** each required native smoke scenario
**When** release evidence is collected
**Then** the scenario is backed either by an automated CI result or a recorded manual result naming the platform and OS version, artifact version or checksum, date, reviewer, and pass/fail outcome
**And** a missing, ambiguous, stale, or failed result blocks release rather than being summarized as an unverified manual check.

**Given** product and release copy
**When** trusted-LAN behavior is described
**Then** it states that the capability URL reduces blind discovery but plain HTTP does not protect against a LAN observer
**And** it makes no end-to-end encryption, hostile-network, cloud relay, signing, notarization, auto-update, Linux packaging, resume, or multi-receiver claim.

**Given** a release candidate that fails a native build, automated check, or required smoke scenario
**When** release readiness is evaluated
**Then** the release is blocked with the failing platform and scenario recorded
**And** architecture or contract changes discovered during release feed back into the spec, architecture decision log and documents, relevant story, tests, and managed agent context before retry.
