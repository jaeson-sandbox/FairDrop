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
    ErrSetupFailed        ErrorCode = "setup_failed"
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
| `path_not_found` | selected root or nested entry disappears during inspection or preparation |
| `path_unsupported` | link, reparse point, special file, or host-unsupported path |
| `source_changed` | directory identity mismatched before enumeration, or staged regular-file type/size/modtime changed before claim |
| `network_unavailable` | no eligible LAN IPv4 |
| `server_start_failed` | listener could not become ready |
| `qr_failed` | capability QR could not be encoded |
| `setup_failed` | a coded failure before any byte was sent: entropy exhaustion, a Prepare-time deadline, an uncoded `SourcePort` error, a malformed Stage acknowledgement, a pre-startup/pre-composition refusal, or `ready()` finding a missing port |
| `beacon_warning` | HTTP/QR are ready but mDNS publication failed; non-terminal |
| `transfer_failed` | invalid preflight size arithmetic, handle-close, read, ZIP, connection, or post-header stream failure |
| `shutting_down` | command rejected after application shutdown begins |

Errors wrap internal causes but expose only the stable code and safe message to React. Absolute paths and capability tokens are never included in HTTP or mDNS errors.

`ErrorCodeOf` uses `errors.As` to find `CodedError` through `%w` wrappers and maps every unknown non-nil error to `transfer_failed`. `PublicErrorOf` uses the recognized code and a fixed safe message; it never copies arbitrary adapter text. `SourcePort` may return `cancelled`, `path_not_found`, `path_unsupported`, `source_changed`, or `transfer_failed` for invalid size arithmetic; network selection returns `network_unavailable`; beacon start returns `beacon_warning`; server start returns `server_start_failed`; QR encoding returns `qr_failed`; claim authorization returns `cancelled` or `shutting_down` (or, only on the residual path where a required port is missing -- unreachable through `NewCoordinator`, which refuses to build a coordinator missing one -- `setup_failed`); payload preparation returns the applicable path/source code, `setup_failed` for a pre-header deadline or an uncoded `SourcePort` error, or `transfer_failed`; streaming after headers are written returns the applicable code or `transfer_failed`. Adapters create or preserve this `internal/transfer` carrier and never compare error strings. `ServerFailed.Err` preserves the wrapped coded error unchanged; the coordinator maps unknowns only at its UI boundary.

## Coordinator-facing ports

```go
package transfer

type SourceEntry struct {
    RelativePath string // slash-separated, beneath the root, never empty/absolute/dot-dot
    Kind         ItemKind
    Size         int64 // meaningful only for ItemFile
    ModTime      time.Time
}

// content is nil for a directory; for a file it is borrowed for the call only.
type SourceVisitor func(entry SourceEntry, content io.Reader) error

type SourcePort interface {
    Inspect(ctx context.Context, absolutePath string) (StagedItem, error)
    Walk(ctx context.Context, absolutePath string, visit SourceVisitor) error
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

The raw source inspector's `SourcePort.Inspect` preserves the `absolutePath` it receives byte-for-byte in `StagedItem.Path`. The coordinator-facing selection decorator resolves ancestors first, behind lifecycle admission, so the path committed to the session and supplied to streaming is canonical at that boundary. It parses only a POSIX root, Windows drive root, or UNC share (including their supported extended spellings), rejects device namespaces, alternate streams, and non-local Windows components, and then evaluates `.` and `..` against a validated handle stack without cleaning or reconstructing the path. Windows metadata opens request only attribute rights, Linux uses no-read `O_PATH`, and Darwin uses parent-relative `fstatat(AT_SYMLINK_NOFOLLOW)` snapshots without pinning the leaf; lexical directory handles request search/traverse rights, and list/read rights are acquired only for a directory that will actually be enumerated. All component and nested lookups are native no-follow operations relative to the already-open parent; Windows uses `NtCreateFile` with `FILE_OPEN_REPARSE_POINT`, while POSIX uses no-follow `openat` for search/enumeration/content and Linux metadata; Darwin queries metadata with no-follow `fstatat`. A Darwin snapshot's borrowed parent remains traversal-owned when the snapshot closes. Darwin compares device/inode, generation and birth timestamp in both `unix.Stat_t` snapshots and `syscall.Stat_t` opened-descriptor metadata before use, since `os.SameFile` rejects custom FileInfo values. Generation/birth fields distinguish recycled device/inode pairs, but identical or zero fields on a filesystem retain a residual fingerprint collision risk; a snapshot is not an inode lease. No private entitlement or content-read access is needed for metadata inspection. Before enumeration, the inspected and opened directory identities must match, the opened object must still be non-link-like, and every child directory identity is compared with active ancestors to refuse cycles without a global visited index. Enumeration is exactly `ReadDir(1)`; traversal sums only non-negative regular-file sizes with checked `int64` addition, attempts every owned close, and retains only active-depth handles plus one entry. Cancellation is checked immediately after every native operation and wins over operation or cleanup failures. An entry name that cannot be placed safely in an archive -- empty, `.`, `..`, or containing a separator or NUL byte -- is `path_unsupported` at `Inspect` as well as at `Walk`. This refuses a folder at selection rather than staging it and failing the download, and it means a name that is legal on POSIX but not archive-shaped, such as one containing a backslash, makes its folder unshareable. A link-like, cyclic, or special entry is `path_unsupported`; a mismatched opened identity is `source_changed`; disappearance remains `path_not_found`; arithmetic faults are `transfer_failed`.

`SourcePort.Walk` re-validates `absolutePath` under exactly the rules `Inspect` applies -- preflight is not a snapshot, so link, reparse, special-file, identity and cycle checks are repeated here rather than trusted -- and then calls `visit` once per entry, a parent before its children. `RelativePath` is slash-separated and locates the entry beneath the root: never empty, absolute, volume-qualified, or dot-dot bearing, and never the root itself, so a consumer places entries under a single top-level name without re-deriving a path. `content` is nil for a directory; for a regular file it is a reader borrowed for exactly that call, because the source owns the descriptor and closes it as the visitor returns. A retained reader is a use-after-close. A visitor error stops the walk and is returned unchanged unless cancellation or a close failure takes precedence. Walk keeps no per-entry index: one enumeration handle per active depth plus the entry being visited, every handle closed in reverse order on every exit. A selection that is not a directory is `path_unsupported`.

After inspection and cancellation revalidation, the coordinator rejects every file or directory `LogicalSize` outside `0..9007199254740991` as `transfer_failed` before calling the network, server, QR, or beacon ports. This is the exact-integer boundary of the JavaScript metadata contract.

The coordinator also consumes injectable entropy and clock/timer ports so session/token generation and the three-second reset are deterministic in tests. Those test seams may use idiomatic signatures chosen in the coordinator package; they must preserve the ownership and timing rules below.

### Port postconditions

- Successful `ServerPort.Start` means the listener is bound and its accept loop is ready before return. Failure cleans all partial server resources. Start after a completed Stop is a specified contract, not merely a possibility: it builds a fresh run with no state carried over from the one before it.
- `ServerPort.Stop` is idempotent and force-closing, and now bounded (Story 3.4): it returns once the listener, active connection, handlers, payload workers, and server event producers have ended and its event channel is closed for good — or once its own documented teardown bound elapses, whichever comes first. A returned error means one of two different things, and a caller must not conflate them. A cleanup diagnostic means teardown finished and something merely went wrong along the way; returned errors describe cleanup but never transfer ownership of live resources back to the coordinator. A bound-elapsed failure means teardown did **not** finish and quiescence is unproven for whatever adapter call or wait is still outstanding — Stop never reports that resource as gone when it cannot prove it, and it names what did not return. Either kind of return leaves Stop safe to call again, and an implementation must never hold a lock across the bounded wait: a Stop that hits its bound must still let a later Start proceed.
- Server progress may be coalesced or dropped to satisfy the 4 Hz cap. Natural Complete/Failed outcomes deliver exactly one terminal event; Cancel/Shutdown may close silently because the coordinator owns those outcomes.
- Natural `ServerComplete` follows successful HTTP final body/framing writes, including chunk termination. `WriteTo` returning or handler-level Flush is insufficient. The connection wrapper observes errors and short writes and preserves TCP CloseWrite for unread-body/oversized-header responses; with keep-alives disabled, `StateClosed` publishes while still tracked for quiescence. A final-write failure reports `transfer_failed`, never Complete. Preparation failure finalizes its empty 410 before publishing its original coded cause. Post-header streaming failures abort; Cancel/Stop retain bounded force-close behavior during a blocked final write. This proves sender-observed transport success only, not receiver saving/opening.
- `ServerHandle.Events` cannot block teardown: terminal capacity is reserved/non-blocking, and a dedicated coordinator drainer consumes until channel close while teardown runs on a separate operation lane. The drainer forwards events to the state lane; the server never invokes coordinator teardown inline from a handler callback stack. The coordinator's own join of that drainer is bounded too (Story 3.4): a `Stop` that returns without closing the lane — the one postcondition violation above that a coordinator cannot prevent on its own — no longer wedges the coordinator forever. A bound that elapses is reported as a coded failure naming the drainer, recorded as a diagnostic, and the coordinator still reaches IDLE rather than holding the operation lease.
- `NetworkPort.StartBeacon` requires a prior successful `GetLocalIP`: it advertises the address that call selected, and there is no address to advertise without one. A `StartBeacon` reached without that precondition is refused with `beacon_warning`. `StartBeacon` returns only after registration is active; on failure it cleans every partial registration before returning. `StopBeacon` is idempotent and guarantees no advertisement remains on every return, even if it reports a cleanup diagnostic — but `StopBeacon` takes no context, so a caller that must bound its own wait for that return (Story 3.4) does so on its own seam, not through the port.
- Adapter Stop methods are safe before Start, after failed Start, and when repeated.
- Every wait the coordinator performs while holding the operation lease or the state mutex — the lease itself, the drainer join, and its own bounded calls into `ServerPort.Stop` and `NetworkPort.StopBeacon` — is bounded (Story 3.4). A bound that elapses is reported as a coded failure naming the port or wait that did not return in time, recorded as a diagnostic, and never reported as success: the coordinator still reaches IDLE and frees the lease, but it makes no claim that the resource itself is quiescent. `Cancel` and `Shutdown` each take a `context.Context` and honour it while waiting to join a teardown already in flight; a caller that abandons that context gets a prompt coded failure rather than the full bound. None of this changes the healthy path: `Cancel` still returns `nil` once every step actually completes, exactly as before.

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

`Prepare` pins filesystem identity. For a file it `Lstat`s the selected root immediately before opening it and compares that against the opened descriptor with `os.SameFile`; kind, size, and modification time are forgeable together, so they are not sufficient on their own, and a mismatch is `source_changed` before headers. A directory has no opened descriptor at `Prepare` and therefore no `SameFile` comparison: preparation is lazy, so the claim-time check is an `Lstat` that must still find a directory and must not find a link-like entry, and every deeper guarantee is re-established per entry during `Walk`. A root that stopped being a directory is `source_changed`; a root that became link-like is `path_unsupported`. Because the walk begins after the response has started, a failure it finds cannot choose an HTTP status and terminates the connection instead.

`Size` is a bound, not a hint, whenever it is known. A directory reports `(0, false)` and writes an archive whose length is unknowable until the last entry is compressed, so no length bounds it and the server performs no delivered-length recheck; the payload alone is responsible for reporting truncation, and it does so by returning a non-nil error from `WriteTo`, which is the only signal available once headers are on the wire. For a known length `WriteTo` never writes more than the advertised length, and fails `transfer_failed` if the source delivers fewer bytes, because a short body reported as success would match no `Content-Length` already on the wire and would pass silently through any abort-on-error defense. `WriteTo` is once-only; a second call fails `transfer_failed` rather than reporting a no-op as success. A context deadline that expires is `transfer_failed`, not `cancelled` -- only a real cancellation is `cancelled`.

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

Beacon Start failure is non-fatal after HTTP and QR are ready: StartBeacon has already cleaned partial state, the session records no active beacon, and Stage succeeds with a `beacon_warning`. StopBeacon guarantees advertisement removal on every return; a returned diagnostic does not block authorization. Authorization's own call into StopBeacon is bounded (Story 3.4): an mDNS shutdown that never returns does not hang the handshake, the commit still proceeds on the bound, and the timeout is recorded as a diagnostic rather than claiming the advertisement is gone.

When every adapter honours its own postcondition, cleanup errors are safe diagnostics and do not retain ownership or prevent the intended IDLE/DONE/ERROR transition: Cancel returns nil after reaching the requested quiescent state, cleanup diagnostics are recorded through a non-sensitive internal diagnostic sink and never turn a completed cancellation into a command rejection, Shutdown records diagnostics only after all resources are gone, and no cleanup error permits a new Stage while resources remain live. That was the whole story before Story 3.4, and every sentence above is still true of an adapter that returns.

What Story 3.4 adds is the other case, because the prior paragraph did not say what happens when an adapter does *not* return. Every wait the coordinator performs while holding the operation lease — the lease itself, the drainer join, and its own bounded calls into `ServerPort.Stop` and `NetworkPort.StopBeacon` — now ends on a documented bound. A wait that hits its bound is never reported as success: the caller (`Cancel` or `Shutdown`) returns a coded failure naming the wait or port that did not return, the coordinator still reaches IDLE and frees the lease so the next command is not blocked forever, and the timeout is recorded as a diagnostic — but nothing anywhere claims the resource itself is quiescent, because that is exactly the one thing an elapsed bound cannot prove. `Cancel` and `Shutdown` each take a `context.Context`: it bounds how long either will wait to join a teardown some other operation already owns, and an abandoned context is honoured with a prompt coded failure rather than the full bound. `ServerPort.Stop` gained the matching bound on its own side (`internal/server`): a teardown that cannot confirm its listener, handlers, and connections have ended within its bound returns a coded failure instead of blocking, and never holds its own mutex across that wait, so a later `Start` is never deadlocked by it.

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
- `App.StageTransfer` delegates unchanged. After lifecycle admission a coordinator-facing SourcePort decorator resolves selection ancestors before the raw inspector. It checks cancellation before/after resolution and while waiting, permits at most one unresolved filesystem call per decorator, and refuses retries busy until that call returns. No mutex spans filesystem I/O and cancellation does not claim the OS call itself stopped. The stream adapter keeps the raw inspector and receives the canonical staged path. The final selected component is never resolved; a trailing separator or explicit final dot traversal retains its original meaning. Resolution failures and Windows device/extended namespace spellings remain with the source's existing grammar and coded refusals. The source's no-follow traversal is unchanged; it receives and preserves the boundary-resolved path.
- Reject a selected symlink, Windows junction/reparse point, nested link/reparse traversal, and non-regular special file.
- Re-`Lstat` the selected root at claim. A file must retain regular-file kind, size, and modification time; otherwise fail `source_changed` before headers.
- Open a regular file before calculating `Content-Length`, and derive the header from that descriptor.
- A directory is an unsnapshotted v1 stream. Additions/removals or in-place mutations during traversal may be observed; any entry that becomes missing, link-like, special, or outside the root fails the transfer rather than being followed.
- Preserve spaces and Unicode. Support long Windows and UNC paths wherever native Go APIs permit; failures are typed and covered on capable native runners.

## Native single-instance lock availability

On macOS, composition checks the same `NSTemporaryDirectory` location, fixed UUID lock file, open flags and nonblocking `flock` that Wails v2.15.0 uses. Contention still enables the Wails handoff. Any other preflight failure disables that option so Wails cannot mistake an unusable lock for a running first instance and exit silently; stderr reports a fixed degraded-protection diagnostic with no filesystem cause or path. Windows locking is unchanged. The probe releases its descriptor before Wails acquires its own; a filesystem change between those opens remains a race, and the probe is not a replacement lock owner.

## Update rule

If implementation evidence requires changing any type, event order, state result, HTTP outcome, or postcondition here, update the architecture memlog, this contract, the matching AD, design guidance, phase spec, and tests together. Never patch one adapter with a private compatibility rule that forks this protocol.
