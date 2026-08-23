# FairDrop Architecture and Design

Status: Final  
Updated: 2026-08-22  
Binding companions: `_bmad-output/planning-artifacts/architecture/architecture-FairDrop-2026-08-22/ARCHITECTURE-SPINE.md` and `docs/fairdrop-contracts.md`

This is the durable technical handoff for humans and implementation agents. The architecture spine is the terse source of binding invariants; this document explains how those invariants fit together and why. Product behavior comes from `docs/fairdrop-spec.md`, with its Phase 1 Corrections applied first. Where this document explicitly supersedes implementation guidance in that older spec, this document wins.

## Goals

- Transfer one local file or directory to one receiver over the LAN.
- Keep payload memory constant in payload size and never stage a ZIP on disk.
- Own every listener, beacon, file handle, pipe, goroutine, callback, and timer for deterministic cancellation and shutdown.
- Keep Wails and React at the edge so transfer behavior is testable without a desktop window.
- Leave enough decision context that another agent can continue without reconstructing the design.

FairDrop is not a cloud service, sync tool, transfer-history database, resumable downloader, or secure channel for hostile networks.

## System shape

FairDrop uses ports and adapters around a single `internal/transfer.Coordinator`.

| Component | Responsibility | Must not own |
| --- | --- | --- |
| `main.go` | Construct concrete adapters, configure Wails, enforce one process | Transfer state or business rules |
| `app.go` | Translate Wails commands and coordinator notifications | HTTP handlers, filesystem traversal, lifecycle truth |
| `internal/transfer` | Validate Stage intent, own state/session, coordinate setup and teardown | Wails runtime calls or concrete network/HTTP code |
| `internal/network` | Select a LAN IPv4 address and own `_fairdrop._tcp` registration | Transfer state or UI events |
| `internal/server` | Own listener, one-shot HTTP claim, headers, progress, and queued terminal events | Wails events or mDNS |
| `internal/stream` | Copy a file or stream a directory ZIP with cancellation | Listener lifecycle or frontend state |
| `internal/qrcode` | Encode the capability URL to an in-memory PNG | Filesystem output |
| React transfer reducer | Render backend-authoritative snapshots/events | Server or transfer lifecycle decisions |

Interfaces belong to the package that consumes them. The three Phase 1 provider-owned interfaces are compile-only transitional scaffolding, not settled locations: before its first implementation, move the network/server lifecycle ports to `internal/transfer` and the streaming port to `internal/server`, then remove rather than duplicate the old public interface. Concrete constructors remain in `internal/network`, `internal/server`, and `internal/stream`. Context-aware Start/Stream behavior remains mandatory; Stop is idempotent and quiescent on every return; the server reports progress and terminal outcomes through the binding event stream.

## Transfer lifecycle

Only the coordinator mutates lifecycle state. It uses a mutex rather than a channel-based actor because commands are infrequent and synchronous entry points are useful, but it follows two strict concurrency rules:

1. Never call an external adapter while holding the state mutex.
2. Attach the immutable current `sessionId` to callbacks and timers; ignore work from an older session.

A single per-session operation lease serializes all adapter Start/Stop/unwind work. Cancel and Shutdown mark cancellation, cancel the data-plane context, and join the existing cleanup result; they never launch cleanup concurrently with Stage, claim authorization, or terminal handling. After every external call made without the state mutex, Stage and claim authorization reacquire it and revalidate the session, expected state, cancellation marker, and shutdown flag before proceeding. The operation lease stays held from a successful TRANSFERRING commit through synchronous publication of `transfer-started`, so a Cancel that linearizes later cannot publish reset first. A dedicated event drainer remains active while Stop runs so terminal delivery cannot deadlock quiescence.

Externally visible states remain `IDLE`, `STAGED`, `TRANSFERRING`, `DONE`, and `ERROR`. `STAGING` and `CLAIMING` are internal so setup, claim authorization, Cancel, and Shutdown cannot interleave partial work. The complete command/state table and canonical port shapes are binding in `docs/fairdrop-contracts.md`.

`App.ctx` is the application-lifetime Wails context and remains stored solely so the adapter can call `runtime.EventsEmit` and other Wails runtime APIs. The coordinator receives that lifetime context and creates a fresh cancellable child for every Stage. It never substitutes a transfer context for `App.ctx`. `App.shutdown` delegates to the coordinator's idempotent shutdown. Terminal live resources are quiesced before DONE/ERROR; the three-second reset timer is a generation-checked application-lifetime UI lease, not a child of the cancelled transfer context.

### Stage transaction

1. Accept exactly one path while `IDLE`; reject zero, multiple, busy, symlink, missing, or special-file inputs with typed errors.
2. Create an independent session ID and capability token through an injectable entropy source backed by `crypto/rand`, plus a child context/cancel function. Neither random value is derived from the other.
3. Preflight the path. For a directory, walk in constant memory to validate entries and calculate the displayed logical size; retain no file index.
4. Resolve the LAN IPv4 address.
5. Start the HTTP listener on `0.0.0.0:0`, passing the capability token and synchronous claim authorizer in the coordinator-owned server request; begin the dedicated server-event drainer.
6. Construct the URL and encode its QR PNG in memory.
7. Attempt to publish a non-sensitive mDNS record. If publication fails after HTTP/QR are usable, retain a safe warning rather than discarding the working direct transfer.
8. Commit `STAGED` and return `FileMetadata`, including `sessionId` and warnings, to the Wails caller.

If any step fails before Stage acknowledgement, cancel and unwind every acquired resource in reverse order, return the command error, emit no lifecycle event, and return to `IDLE`. `ERROR` is a visible terminal lease only for a session the frontend already learned from a successful Stage result.

### Download transaction

1. The server registers only the methodless Go `http.ServeMux` pattern `/download/{token}` and explicitly accepts exactly `GET` in the handler. A method-qualified pattern would make `ServeMux` return `405` and route `HEAD` to `GET`, contrary to the required disguise. Wrong methods, malformed routes, and token mismatches look nonexistent (`404`).
2. The first exact-token request atomically reserves the transfer before authorization; another valid request receives `423 Locked` while the listener remains live.
3. The reserved handler synchronously requests authorization. The coordinator enters `CLAIMING`, generation-checks, stops mDNS, commits `TRANSFERRING`, and publishes `transfer-started` before authorization returns. Stop guarantees the advertisement is gone even if it reports a cleanup diagnostic.
4. Only after authorization succeeds does the server prepare the payload, write safe response headers, and stream through the appropriate adapter.
5. Successful completion supplies authoritative terminal progress, quiesces resources, emits final progress then `transfer-complete`, enters `DONE`, and emits `transfer-reset` after three seconds.
6. A disconnect or stream failure quiesces resources, emits authoritative final progress when bytes were written, then `transfer-error`, enters `ERROR`, and resets after three seconds.
7. Cancel after Stage acknowledgement cancels and joins the single teardown, suppresses queued server outcomes, emits `transfer-reset` before returning, and enters `IDLE`. Cancel during pre-acknowledgement STAGING returns only the command error and emits nothing.

Application shutdown calls the same idempotent teardown and does not wait for the three-second UI reset.

## Data and event contracts

The public Wails commands remain `StageTransfer(absolutePath string) (*FileMetadata, error)` and `CancelTransfer() error`, with additive `SelectFile()` and `SelectDirectory()` dialog commands. `FileMetadata` uses `sessionId`, `name`, `size`, `isDir`, `url`, `qrBase64`, and `warnings`. Absolute paths stay inside the sender process and must never appear in HTTP responses, mDNS records, or user-safe remote errors.

Every runtime event carries `sessionId`. The required event family is:

| Event | Payload purpose |
| --- | --- |
| `transfer-started` | Identify the claimed session and switch to the transferring view |
| `transfer-progress` | `bytesSent`, `totalBytes`, `totalKnown`, `percent`, `speedBytesPerSec` |
| `transfer-complete` | Identify successful terminal state and final statistics |
| `transfer-error` | Stable error code plus safe user message |
| `transfer-reset` | Authoritatively return the UI to idle |

For a regular file, `totalKnown=true`; zero bytes is a valid known total. For a known positive total, `percent` is `100 * bytesSent / totalBytes`, clamped to the finite range `[0,100]`. Known-empty and unknown totals use `percent=0`. For a streamed ZIP, the wire length is not known before completion, so `totalKnown=false`, `totalBytes=0`, and `percent=0`. The UI shows an indeterminate bar and the actual wire-byte count/speed. JSON must never contain NaN or infinity.

Progress counts bytes only after `http.ResponseWriter.Write` reports success. Emit no more than once per 250 ms during transfer, then emit an unthrottled terminal snapshot.

Every event also carries a contiguous sequence: the first published event is `seq=1`, and each later published event increments by one. One synchronous coordinator emission lane guarantees Complete uses `started → progress* → final progress → complete → reset`; Failed publishes final progress only when bytes were written, so a preparation failure may use `started → error → reset`; Cancel after claim ends any already-published `started, progress*` prefix with reset and no terminal event. Post-Stage pre-claim Cancel uses reset only; pre-acknowledgement failure uses no lifecycle event. No progress follows terminal acceptance. The React reducer initializes its session only from a successful Stage result and ignores another session ID or a non-increasing sequence. Stage can show a pending state locally, but it does not claim `STAGED` until Go succeeds and it ignores an obsolete promise after local request cancellation/unmount.

`options.App.ErrorFormatter` serializes a stable `{code,message}` `PublicError` as a JSON string. The generated Wails runtime exposes that string through rejected `Error.message`; the frontend parser validates it and falls back safely. `qrBase64` is padded standard base64 of PNG bytes without a data-URI prefix.

Internally, `internal/transfer` owns `ErrorCode`, `DomainError`, and an `errors.As`-compatible `CodedError` contract. Adapters preserve coded causes through `%w`; they never compare error strings. Unknown adapter errors map to `transfer_failed` only at the coordinator/UI boundary, and safe public messages never copy arbitrary adapter text. A completed Cancel returns nil once teardown is quiescent; cleanup errors are non-sensitive diagnostics rather than contradictory command failures.

## Wails input and accessibility boundary

Phase 1 proved integration facts that later UI work must retain:

- Configure native drops through `DragAndDrop: &options.DragAndDrop{EnableFileDrop: true}`.
- Register `OnFileDrop(callback, true)` and always clean up through `OnFileDropOff()`.
- Mark the drop zone through inherited CSS `--wails-drop-target: drop`; do not replace this with a DOM drop handler or class-only gate.
- Treat the callback's `string[]` as untrusted shape: exactly one path may call `StageTransfer`; zero or multiple paths show a safe validation error and never select the first silently.

Drag-and-drop is not the only input. The idle view provides semantic keyboard-reachable `SelectFile` and `SelectDirectory` actions backed by Wails native runtime dialogs, visible focus, and an `aria-live` region for staging and lifecycle changes. A cancelled dialog is not a transfer error. Phase 6 may revise the Phase 1 echo-only multi-file test, but it must retain coverage for array receipt, CSS targeting, and listener cleanup.

## HTTP protocol and security envelope

The capability path contains at least 128 random bits generated by `crypto/rand` and encoded with raw URL-safe base64. The token is ephemeral and never written to disk. It appears in the local Stage URL/QR and receiver request path, but not in mDNS or diagnostics. The selected basename/archive name is additionally disclosed in the authorized response's sanitized `Content-Disposition`; source paths remain local.

Response rules:

- `Content-Disposition: attachment` with a sanitized ASCII fallback and RFC 5987 `filename*` for Unicode.
- `Content-Length` only for a regular file.
- `Cache-Control: no-store`, `Access-Control-Allow-Origin: *`, and `X-Content-Type-Options: nosniff`.
- Bounded request-header and idle timeouts, bounded maximum headers, no request body, and no whole-transfer write deadline.
- No range/resume behavior in v1.

FairDrop v1 is plain HTTP because an ad-hoc sender cannot present a certificate trusted by arbitrary phone browsers. The capability URL reduces blind discovery but does not protect against a LAN observer. UI copy and release documentation must call this a trusted-LAN transfer, not end-to-end secure sharing.

mDNS advertises `_fairdrop._tcp` with a unique, non-sensitive instance name and protocol-version TXT field. It does not advertise filename, absolute path, token, or full URL. Discovery failure is recoverable when the QR/direct URL remains usable.

## Filesystem and streaming rules

Single files are opened at transfer time, re-statted, and copied through a context-aware bounded buffer. `Content-Length` comes from the open file descriptor, not only stale Stage metadata.

Directory staging and streaming reject symbolic links and non-regular special files. ZIP entry names are computed relative to the selected root, converted with `filepath.ToSlash`, and rejected if absolute or traversal-bearing. The archive contains one top-level directory. Streaming uses `io.Pipe`; all exit paths close both ends, and `zip.Writer.Close()` precedes pipe-writer closure so the central directory is emitted.

Spaces, Unicode, Windows paths longer than 260 characters, and UNC paths are supported wherever the host filesystem and Go `os` APIs support them. Paths are passed as values—never interpolated into a shell command or destructively rewritten. A native platform that cannot open a path returns a stable typed path error. Native-runner tests cover each supported class; symlinks remain an explicit rejection.

After claim authorization, a payload-preparation failure returns a generic `410 Gone` without source details while preserving a specific local `source_changed` or `path_not_found` error. Once headers or bytes have been written, the server notifies the coordinator and aborts with `http.ErrAbortHandler`; it must not append an error body to a partial payload or let the client interpret truncation as success.

After `Prepare` succeeds, the server owns exactly one payload `Close`. It cancels the data-plane context and closes the HTTP destination, waits for `WriteTo` and workers, then calls `Close`; `Close` never races `WriteTo`.

Buffer size and per-entry ZIP compression are Phase 3 benchmark choices. They may change without architecture review if payload memory remains O(buffer), cancellation remains prompt, and archive compatibility tests remain green.

## Network selection

Phase 2 owns the exact scoring algorithm. It must be deterministic and testable from injected interface data:

- require up, broadcast-capable, non-loopback, non-point-to-point IPv4;
- exclude interface names containing `docker`, `veth`, or `tun`;
- prefer private LAN addresses over link-local fallbacks;
- use stable interface/address ordering when candidates tie;
- return a typed error when no suitable address exists.

IPv6 and an interface-selection UI are deferred until real multi-homed failures justify the product complexity.

The DNS-SD service type is the fixed protocol constant `_fairdrop._tcp`. The human-readable instance name includes hostname plus a process-unique suffix so separate hosts with the same name do not collide, without writing a persistent device identifier.

## Dependencies and platform decisions

- Keep Wails v2.15.0. The working scaffold and file-drop contract are verified against it, v2 is the stable upstream line, and v3 remains pre-GA.
- Keep the Go module floor at 1.25.0 and verify with the installed Go 1.26.7 toolchain.
- Use `github.com/hashicorp/mdns` v1.0.7. Its current maintenance release addresses vulnerable transitive dependencies.
- Use `github.com/boombuler/barcode` v1.1.0 for QR generation. It is tagged and maintained; this supersedes the inactive, unversioned `github.com/skip2/go-qrcode` guidance in the source spec.
- Pin Node 24 LTS (24.19.0 at authoring) in local development and CI; it satisfies the resolved Vite/Vitest engine ranges.
- npm versions are owned by `package-lock.json`; use `npm ci`, including dev dependencies required by TypeScript, Vite, tests, and Wails builds. `npm ci --omit=dev` is not a supported build mode because those tools are not shipped in the desktop artifact. Migrate both TypeScript projects to `moduleResolution: "Bundler"` together with compatible ES target/lib settings and locked Node types, then prove both config and application type-checks.
- Do not migrate Wails majors or add a router/state library without a requirement-level reason and architecture update.

Upstream evidence checked during this architecture run:

- [Wails repository status](https://github.com/wailsapp/wails)
- [Wails v2 build CLI](https://wails.io/docs/reference/cli/)
- [Go release history](https://go.dev/doc/devel/release)
- [Node.js release schedule](https://nodejs.org/en/about/previous-releases)
- [hashicorp/mdns v1.0.7](https://github.com/hashicorp/mdns/releases/tag/v1.0.7)
- [boombuler/barcode v1.1.0](https://github.com/boombuler/barcode/releases/tag/v1.1.0)

## Verification and release

Tests are organized around boundaries:

- Coordinator: fake network/server/QR/observer/clock ports; full transition table, stale callback rejection, teardown order, repeated Stop, and cancellation races.
- Network: injected interface fixtures covering VPNs, loopback, multiple candidates, no candidate, and mDNS start/stop idempotency.
- Stream: file and directory success, empty payloads, Unicode/long paths where supported, symlink rejection, permission failures, cancellation, ZIP integrity, goroutine exit, and bounded-memory evidence.
- Server: exact route/method, random token shape, first-claim race, 423 behavior, headers, progress cadence, disconnects, and forced abort after headers.
- Wails/React: command errors, event-to-reducer mapping, stale session events, listener cleanup, transitions, keyboard/pointer file selection, and accessibility announcements.

Required pre-merge checks grow to include Go tests/vet, frontend tests/build, and `wails build`. Run `go test -race ./...` on a native CI runner provisioned with the C toolchain required by the race detector; it is not assumed to work in every local Windows shell. Release artifacts are built and smoke-tested on native Windows and macOS runners. UPX is optional because upstream documents Apple Silicon issues and Windows antivirus false positives.

## Explicit supersessions of the source spec

1. Multi-path drops are rejected in v1; the first path is never selected silently.
2. Progress adds `totalKnown`; directory wire progress is indeterminate instead of dividing by zero or pretending uncompressed size equals response size.
3. `FileMetadata` adds `sessionId` and warnings; lifecycle events add session ID, sequence, and `transfer-reset` so the backend remains authoritative.
4. The Phase 1 provider-owned interfaces are replaced before first implementation by the consumer-owned ports in `docs/fairdrop-contracts.md`; context-aware behavior and adapter package responsibilities remain.
5. `Stop` is idempotent and waits for owned work to finish.
6. Mid-stream errors force an aborted response after notifying the coordinator.
7. Capability URLs, trusted-LAN limits, and non-sensitive mDNS metadata define the previously missing security envelope.
8. `boombuler/barcode` replaces `skip2/go-qrcode`.
9. Release builds run on native OS runners; `-upx` is not the default recommendation.
10. Wails single-instance locking is a new architecture requirement and must be added to `appOptions()` and pinned in `main_test.go` alongside the already-settled frame, drop, and lifecycle-hook options.

## Maintenance rule

Any change that alters an architecture decision must update, in the same branch:

1. the architecture memlog with the new evidence and decision;
2. the stable AD in `ARCHITECTURE-SPINE.md` without renumbering unrelated ADs;
3. this document where operational guidance or rationale changed;
4. the relevant phase spec, tests, and `AGENTS.md` managed context through `bmad-project-context` when agent instructions changed.

Ordinary implementation details belong in code and phase specs, not here. The design remains lean by deferring multi-item transfer, authenticated discovery/TLS, IPv6/interface UI, ZIP tuning, Linux packaging, signing/notarization, auto-update, transfer history, resume/range requests, and multi-receiver support until a requirement demands them.
