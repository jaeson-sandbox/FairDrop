# FairDrop Binding Integration Contracts

Status: Final  
Updated: 2026-09-01
Architecture: `docs/fairdrop-architecture.md`  
Spine: `_bmad-output/planning-artifacts/architecture/architecture-FairDrop-2026-08-22/ARCHITECTURE-SPINE.md`

This document fixes the cross-package shapes and ordering rules that separate phase agents must share. The Go below is contract-level pseudocode: implementation may split files or add private fields, but exported meanings, ownership, ordering, and postconditions may not drift without an architecture update.

## Ownership and dependency direction

| Contract | Owner | Implementer |
| --- | --- | --- |
| Coordinator public API, `NetworkPort`, `ServerPort`, `SourcePort`, `QRPort`, `Observer`, domain values/errors/events | `internal/transfer` | coordinator plus adapters |
| `PayloadPort` and `PreparedPayload` | `internal/server` | `internal/stream` |
| Wails command DTOs | `app.go` adapter, derived from transfer values | `App` |
| React event types | generated/hand-mirrored from Wails DTOs | `frontend/src/transfer` |

The provider-owned Phase 1 interfaces have been deleted. Do not recreate duplicate interfaces or conversion-only shadow types alongside the consumer-owned contracts above.

## Canonical domain values

```go
package transfer

type SessionID string       // internal/UI correlation; >=128 random bits
type CapabilityToken string // HTTP capability; separate >=128 random bits

type ItemKind string
const (
    ItemFile      ItemKind = "file"
    ItemDirectory ItemKind = "directory"
)

type StagedItem struct {
    Path        string   // sender-private; never serialized remotely
    Name        string
    Kind        ItemKind
    LogicalSize int64
    ModTime     time.Time
}

type ErrorCode string

const (
    ErrInvalidSelection   ErrorCode = "invalid_selection"
    ErrBusy               ErrorCode = "busy"
    ErrCancelled          ErrorCode = "cancelled"
    ErrPathNotFound       ErrorCode = "path_not_found"
    ErrPathUnsupported    ErrorCode = "path_unsupported"
    ErrSourceChanged      ErrorCode = "source_changed"
    ErrNetworkUnavailable ErrorCode = "network_unavailable"
    ErrServerStartFailed  ErrorCode = "server_start_failed"
    ErrQRFailed           ErrorCode = "qr_failed"
    ErrBeaconWarning      ErrorCode = "beacon_warning"
    ErrTransferFailed     ErrorCode = "transfer_failed"
    ErrShuttingDown       ErrorCode = "shutting_down"
)

type Warning struct {
    Code    ErrorCode `json:"code"`
    Message string `json:"message"`
}

type FileMetadata struct {
    SessionID SessionID `json:"sessionId"`
    Name      string    `json:"name"`
    Size      int64     `json:"size"`
    IsDir     bool      `json:"isDir"`
    URL       string    `json:"url"`
    QR        string    `json:"qrBase64"`
    Warnings  []Warning `json:"warnings"`
}

type ProgressSnapshot struct {
    BytesSent        int64   `json:"bytesSent"`
    TotalBytes       int64   `json:"totalBytes"`
    TotalKnown       bool    `json:"totalKnown"`
    Percent          float64 `json:"percent"`
    SpeedBytesPerSec float64 `json:"speedBytesPerSec"`
}

type PublicError struct {
    Code    ErrorCode `json:"code"`
    Message string `json:"message"`
}

type CodedError interface {
    error
    Code() ErrorCode
}

// DomainError stores a stable code, safe local message, and optional wrapped cause.
// Its concrete fields may remain private; errors.As/errors.Is work through Unwrap.
type DomainError struct { /* code, safe message, cause */ }

func NewError(code ErrorCode, safeMessage string) error
func WrapError(code ErrorCode, safeMessage string, cause error) error
func ErrorCodeOf(err error) ErrorCode
func PublicErrorOf(err error) PublicError
```

`Warnings` serializes as an empty array, not `null`. Directory wire totals use `TotalKnown=false`, `TotalBytes=0`, and `Percent=0`. A known empty file uses `TotalKnown=true`, `TotalBytes=0`, and `Percent=0`. NaN and infinity are forbidden.

Stable domain error codes are:

| Code | Meaning |
| --- | --- |
| `invalid_selection` | zero/multiple paths or empty path at an input boundary |
| `busy` | Stage requested outside IDLE |
| `cancelled` | Stage/claim/transfer lost to Cancel or Shutdown |
| `path_not_found` | selected root no longer exists |
| `path_unsupported` | link, reparse point, special file, or host-unsupported path |
| `source_changed` | staged regular-file type/size/modtime changed before claim |
| `network_unavailable` | no eligible LAN IPv4 |
| `server_start_failed` | listener could not become ready |
| `qr_failed` | capability QR could not be encoded |
| `beacon_warning` | HTTP/QR are ready but mDNS publication failed; non-terminal |
| `transfer_failed` | read, ZIP, connection, or post-header stream failure |
| `shutting_down` | command rejected after application shutdown begins |

Errors wrap internal causes but expose only the stable code and safe message to React. Absolute paths and capability tokens are never included in HTTP or mDNS errors.

`ErrorCodeOf` uses `errors.As` to find `CodedError` through `%w` wrappers and maps every unknown non-nil error to `transfer_failed`. `PublicErrorOf` uses the recognized code and a fixed safe message; it never copies arbitrary adapter text. `SourcePort` may return `path_not_found`, `path_unsupported`, or `source_changed`; network selection returns `network_unavailable`; beacon start returns `beacon_warning`; server start returns `server_start_failed`; QR encoding returns `qr_failed`; claim authorization returns `cancelled` or `shutting_down`; payload preparation/streaming returns the applicable path/source code or `transfer_failed`. Adapters create or preserve this `internal/transfer` carrier and never compare error strings. `ServerFailed.Err` preserves the wrapped coded error unchanged; the coordinator maps unknowns only at its UI boundary.

## Coordinator-facing ports

```go
package transfer

type SourcePort interface {
    Inspect(ctx context.Context, absolutePath string) (StagedItem, error)
}

type BeaconRequest struct {
    SessionID SessionID
    Service   string // always _fairdrop._tcp
    Instance  string
    Port      int
    TXT       []string // protocol version and non-sensitive identity only
}

type NetworkPort interface {
    GetLocalIP(ctx context.Context) (netip.Addr, error)
    StartBeacon(ctx context.Context, request BeaconRequest) error
    StopBeacon() error
}

type QRPort interface {
    EncodePNG(ctx context.Context, content string) ([]byte, error)
}

type ServerStartRequest struct {
    SessionID SessionID
    Token     CapabilityToken
    Item      StagedItem
}

type ClaimAuthorizer interface {
    AuthorizeClaim(ctx context.Context, sessionID SessionID) error
}

type ServerEventKind string
const (
    ServerProgress ServerEventKind = "progress"
    ServerComplete ServerEventKind = "complete"
    ServerFailed   ServerEventKind = "failed"
)

type ServerEvent struct {
    SessionID SessionID
    Kind      ServerEventKind
    Progress  *ProgressSnapshot // authoritative terminal snapshot on Complete; optional on Failed
    Err       error
}

type ServerHandle struct {
    Port   int
    Events <-chan ServerEvent
}

type ServerPort interface {
    Start(ctx context.Context, request ServerStartRequest, authorizer ClaimAuthorizer) (ServerHandle, error)
    Stop() error
}

type EventKind string
const (
    TransferStarted  EventKind = "transfer-started"
    TransferProgress EventKind = "transfer-progress"
    TransferComplete EventKind = "transfer-complete"
    TransferError    EventKind = "transfer-error"
    TransferReset    EventKind = "transfer-reset"
)

type Event struct {
    SessionID SessionID        `json:"sessionId"`
    Seq       uint64           `json:"seq"`
    Kind      EventKind        `json:"-"`
    Progress  *ProgressSnapshot `json:"progress,omitempty"`
    Error     *PublicError     `json:"error,omitempty"`
}

type Observer interface {
    Publish(event Event) // synchronous FIFO handoff; implementation must not reorder
}
```

The coordinator also consumes injectable entropy and clock/timer ports so session/token generation and the three-second reset are deterministic in tests. Those test seams may use idiomatic signatures chosen in the coordinator package; they must preserve the ownership and timing rules below.

### Port postconditions

- Successful `ServerPort.Start` means the listener is bound and its accept loop is ready before return. Failure cleans all partial server resources.
- `ServerPort.Stop` is idempotent and force-closing. On every return—even with an error—the listener, active connection, handlers, payload workers, and server event producers have ended; its event channel is closed and will never produce another event. Returned errors describe cleanup but never transfer ownership of live resources back to the coordinator.
- Server progress may be coalesced or dropped to satisfy the 4 Hz cap. Natural Complete/Failed outcomes deliver exactly one terminal event; Cancel/Shutdown may close silently because the coordinator owns those outcomes.
- `ServerHandle.Events` cannot block teardown: terminal capacity is reserved/non-blocking, and a dedicated coordinator drainer consumes until channel close while teardown runs on a separate operation lane. The drainer forwards events to the state lane; the server never invokes coordinator teardown inline from a handler callback stack.
- `NetworkPort.StartBeacon` returns only after registration is active; on failure it cleans every partial registration before returning. `StopBeacon` is idempotent and guarantees no advertisement remains on every return, even if it reports a cleanup diagnostic.
- Adapter Stop methods are safe before Start, after failed Start, and when repeated.

## Server-facing payload port

```go
package server

type PreparedPayload interface {
    DownloadName() string
    Size() (bytes int64, known bool)
    WriteTo(ctx context.Context, dst io.Writer) error
    Close() error
}

type PayloadPort interface {
    Prepare(ctx context.Context, item transfer.StagedItem) (PreparedPayload, error)
}
```

`Prepare` runs before response headers. For a file, it opens and stats the same descriptor, validates the staged root, and returns a known length. For a directory it returns an unknown wire length and begins streaming only from `WriteTo`. `Close` is idempotent.

`Prepare` pins filesystem identity: it `Lstat`s the selected root immediately before opening it and compares that against the opened descriptor with `os.SameFile`. Kind, size, and modification time are forgeable together, so they are not sufficient on their own; a mismatch is `source_changed` before headers.

`Size` is a bound, not a hint. `WriteTo` never writes more than the advertised length, and fails `transfer_failed` if the source delivers fewer bytes, because a short body reported as success would match no `Content-Length` already on the wire and would pass silently through any abort-on-error defense. `WriteTo` is once-only; a second call fails `transfer_failed` rather than reporting a no-op as success. A context deadline that expires is `transfer_failed`, not `cancelled` -- only a real cancellation is `cancelled`.

`DownloadName` is sanitized by the payload, not by the server. It is a bare basename with no separator, no `..`, no control or Unicode format character, and none of the delimiters that terminate or extend the `filename` parameter. The server places the value in the header as given.

After successful `Prepare`, the server owns exactly one `Close`. It never calls `Close` concurrently with `WriteTo`. Cancellation order is: cancel the data-plane context, force-close the HTTP connection/destination so writes unblock, wait for `WriteTo` and its workers to return, then call `Close`. The same ownership covers normal completion, receiver disconnect, header failure, Cancel, and Stop-before-Write.

## Public Wails API

```go
func (a *App) StageTransfer(absolutePath string) (*transfer.FileMetadata, error)
func (a *App) CancelTransfer() error
func (a *App) SelectFile() (string, error)
func (a *App) SelectDirectory() (string, error)
func (a *App) CopyToClipboard(text string) error
```

`SelectFile` and `SelectDirectory` use Wails native runtime dialogs with the application-lifetime `App.ctx`; they do not stage automatically. A cancelled native dialog returns an empty selection without emitting a transfer error. The frontend validates that native drop arrays contain exactly one path before calling `StageTransfer`.

`CopyToClipboard` writes through the Wails Go runtime. The frontend never relies on `navigator.clipboard`, because the macOS Wails webview is not a secure context.

Wails command failures use `options.App.ErrorFormatter` to serialize `PublicError` as a JSON string. The generated runtime rejects with `Error.message` containing that JSON; frontend `parseCommandError` parses and validates `{code,message}`, falling back to `transfer_failed` for malformed/unknown errors. `main_test.go` pins the formatter option.

## Command and state table

`STAGING` and `CLAIMING` are internal states. `closing` is an application-lifetime flag, not a UI state.

| Input | Allowed state(s) | Result |
| --- | --- | --- |
| Stage | IDLE | Enter STAGING; commit STAGED only after required setup; return metadata with `sessionId`. |
| Stage | Any other state | Return `busy`; no state/resource change. |
| Cancel | IDLE | Return success; no event. |
| Cancel | STAGING | Mark generation cancelled, cancel context, wait for reverse unwind, make Stage return `cancelled`, emit no lifecycle event, enter IDLE. |
| Cancel | STAGED or CLAIMING | Deny/abort claim, cancel and quiesce resources, publish one reset before returning, enter IDLE. |
| Cancel | TRANSFERRING | Cancel connection/stream, quiesce resources, suppress cancellation-as-error, publish one reset before returning, enter IDLE. |
| Cancel | DONE or ERROR | Cancel reset timer, publish reset before returning, clear session, enter IDLE. |
| AuthorizeClaim | matching STAGED | Enter CLAIMING, synchronously stop beacon without the mutex, then reacquire and revalidate before committing TRANSFERRING; hold the operation lease through started publication, then return success. Stop diagnostics are recorded safely but do not imply a live beacon. |
| AuthorizeClaim | stale/non-STAGED/cancelled | Return `cancelled`; server writes no payload. |
| Progress | matching TRANSFERRING | Assign next sequence and publish, subject to throttle. |
| Complete/failed | matching TRANSFERRING | Accept exactly once, quiesce live resources, use the server's authoritative terminal progress, publish final progress when present then terminal event, retain terminal UI lease, schedule reset. |
| Reset timer | matching DONE/ERROR | Publish reset, clear session, enter IDLE. |
| Shutdown | Any | Set closing, reject new commands, cancel reset/live contexts, quiesce resources, suppress further UI events, return only when closed. |

Cancel and Shutdown are allowed to race any setup step. A Stage call may return only after it either commits STAGED or observes cancellation and finishes unwind; it cannot return successful metadata after Cancel/Shutdown wins.

Exactly one per-session operation lease may call adapter Start/Stop/unwind methods. Stage setup, claim authorization, terminal handling, and teardown serialize through that lease. Cancel/Shutdown mark the generation cancelled, cancel the data-plane context, and wait on the existing teardown completion; they never launch concurrent cleanup. The operation owner records one teardown result for all joiners.

Every Stage or claim step that calls an external port does so without the state mutex. After the call returns, the operation owner reacquires the mutex and revalidates the session ID, expected state, `closing`, and the generation's cancellation marker before it uses the result or begins the next step. The mutex-protected STAGED commit and TRANSFERRING commit are the linearization points. Claim authorization holds the operation lease through synchronous publication of `transfer-started` after the TRANSFERRING commit. If Cancel marks cancellation before that commit, authorization returns `cancelled` and emits no started event; if the commit occurs first, started is published before Cancel can acquire the operation lease and publish reset. Coordinator tests must force both race outcomes. Stage tests must force cancellation after each external setup step and prove it never commits stale results.

Setup failure or cancellation before a successful Stage acknowledgement unwinds to IDLE, returns the command error, emits no lifecycle event, and creates no terminal UI lease. After Stage succeeds, pre-transfer user Cancel emits reset only. A post-claim failure follows started, optional final progress, error, reset.

## Claim and HTTP ordering

1. Reject malformed/oversized paths, wrong methods, wrong routes, and token mismatches as `404` without reserving or claiming.
2. The first exact-token GET atomically reserves the server. A second exact-token GET receives `423` only while that reserved/claimed listener remains live.
3. The reserved handler calls `AuthorizeClaim` synchronously. It opens no payload and writes no header first.
4. Authorization generation-checks the session, enters CLAIMING, stops the beacon, commits TRANSFERRING, and synchronously publishes `transfer-started`. `StopBeacon` diagnostics are safe because the port guarantees the advertisement is gone before return.
5. Only after authorization succeeds may the handler prepare the payload and write headers/body. If Cancel/Shutdown wins, authorization returns `cancelled`; the handler returns `404` if it can still respond, otherwise closes.
6. Terminal teardown closes the listener immediately. No replay HTTP status is promised after the listener closes.

The server registers the Go 1.22+ methodless `http.ServeMux` pattern `/download/{token}`, obtains the token only through `request.PathValue("token")`, and explicitly checks `request.Method == http.MethodGet` before any claim logic. A method-qualified `GET /download/{token}` pattern is forbidden because `ServeMux` would answer other methods with `405 Method Not Allowed` and route `HEAD` to the GET handler; FairDrop requires both to look nonexistent (`404`). No third-party router syntax or manual path splitting defines this boundary.

After authorization, a `PayloadPort.Prepare` failure returns a generic `410 Gone` response with no path/token details, emits `ServerFailed` preserving a recognized local code such as `source_changed` or `path_not_found`, and closes the listener. Its UI grammar is started, optional final progress, error, reset.

Beacon Start failure is non-fatal after HTTP and QR are ready: StartBeacon has already cleaned partial state, the session records no active beacon, and Stage succeeds with a `beacon_warning`. StopBeacon guarantees advertisement removal on every return; a returned diagnostic does not block authorization.

Because every Stop return is quiescent, cleanup errors are safe diagnostics and do not retain ownership or prevent the intended IDLE/DONE/ERROR transition. Cancel returns nil after reaching the requested quiescent state; cleanup diagnostics are recorded through a non-sensitive internal diagnostic sink and never turn a completed cancellation into a command rejection. Shutdown records diagnostics only after all resources are gone. No cleanup error permits a new Stage while resources remain live.

## Event ordering

After Stage acknowledgement, the coordinator owns one synchronous emission lane. The first published event for a session has `seq=1`; each later published event increments by exactly one. Coalesced/dropped progress snapshots are not assigned sequence numbers. Valid grammars are:

- Successful claimed transfer: `started`, progress*, authoritative final progress, complete, reset.
- Failed claimed transfer: `started`, progress*, optional authoritative final progress only when bytes were written, error, reset. A Prepare failure may therefore be `started`, error, reset.
- User Cancel from STAGED/CLAIMING: reset only if Cancel linearizes before the TRANSFERRING commit. User Cancel after that commit terminates any already-published `started`, progress* prefix with reset and no complete/error. Queued server events are discarded.
- Setup failure or Cancel before Stage acknowledgement: no lifecycle event; the command error is authoritative.
- Server channel closure before a natural terminal event while actively TRANSFERRING and not tearing down: synthesize `transfer_failed`, then reset. Closure after coordinator-requested Cancel/Shutdown is normal and silent.

No progress is accepted or emitted after terminal acceptance. Natural Complete carries the authoritative final snapshot; for a known file it matches the prepared length. Failed carries an authoritative snapshot when bytes were written and `nil` otherwise. React initializes `(sessionId, lastSeq=0)` only from a successful Stage result, ignores an obsolete Stage promise after local request cancellation/unmount, and ignores events with another session ID or `seq <= lastSeq`.

`ProgressSnapshot.Percent` is always finite and clamped to `[0,100]`. When `TotalKnown && TotalBytes > 0`, it equals `100 * BytesSent / TotalBytes`; a successful known non-empty completion is exactly `100`. Unknown totals and known empty totals use zero. A failed snapshot applies the same formula to its final written-byte count.

Event payload validity at the Wails boundary:

| Event | `progress` | `error` |
| --- | --- | --- |
| started | absent | absent |
| progress | required | absent |
| complete | required | absent |
| error | optional | required `PublicError` |
| reset | absent | absent |

The Wails adapter emits the event-specific payload without the internal `Kind` field. `qrBase64` is standard padded base64 of `image/png` bytes with no data-URI prefix; React prepends `data:image/png;base64,` when rendering.

## Disclosure matrix

| Data | Allowed disclosure |
| --- | --- |
| Capability token | Local Stage URL/QR and the receiver's authorized HTTP request path only |
| Selected basename/archive name | Local Stage metadata and sanitized `Content-Disposition` on the authorized response |
| Absolute/relative source path | Sender process only |
| mDNS TXT | Protocol version and non-sensitive instance identity only |
| Logs and unrelated HTTP errors | No token, filename, or source path |

## Source mutation and link policy

- Inspect and stream with filesystem APIs, never shell commands.
- Reject a selected symlink, Windows junction/reparse point, nested link/reparse traversal, and non-regular special file.
- Re-`Lstat` the selected root at claim. A file must retain regular-file kind, size, and modification time; otherwise fail `source_changed` before headers.
- Open a regular file before calculating `Content-Length`, and derive the header from that descriptor.
- A directory is an unsnapshotted v1 stream. Additions/removals or in-place mutations during traversal may be observed; any entry that becomes missing, link-like, special, or outside the root fails the transfer rather than being followed.
- Preserve spaces and Unicode. Support long Windows and UNC paths wherever native Go APIs permit; failures are typed and covered on capable native runners.

## Update rule

If implementation evidence requires changing any type, event order, state result, HTTP outcome, or postcondition here, update the architecture memlog, this contract, the matching AD, design guidance, phase spec, and tests together. Never patch one adapter with a private compatibility rule that forks this protocol.
