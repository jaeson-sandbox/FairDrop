# Comprehensive Project Specification: DeadDrop

## Phase 1 Corrections (verified against Wails v2.15.0)

Phase 1 implementation proved four instructions in this document wrong, and Epic 1 has since superseded a fifth. **Follow the corrections below, not the original text.** Each is also flagged inline at the spot it affects. Everything else in this document stands as written.

1. **File drop is not a top-level option.** `options.App` has no `EnableFileDrop` field. The real form is the nested struct `DragAndDrop: &options.DragAndDrop{EnableFileDrop: true}` (evidence: `pkg/options/options.go:201-216`). *Affects §6 Module D, §10 Phase 1.*
2. **There is no `wails_file_drop` event.** The runtime event is `wails:file-drop`, and the supported API is the helper pair `OnFileDrop(callback, useDropTarget)` / `OnFileDropOff()` imported from `../wailsjs/runtime/runtime`. Drops are gated on the inherited CSS custom property `--wails-drop-target: drop` -- not a class, not a DOM handler (evidence: `internal/frontend/runtime/desktop/draganddrop.js`). *Affects §6 Module D, §10 Phase 6.*
3. **The product is FairDrop, not DeadDrop.** "DeadDrop" throughout this document is a stale working title. The mDNS service is `_fairdrop._tcp`; the Go module, `outputfilename`, and window title are all FairDrop. *Affects §9 and every naming reference.*
4. **Three §9 signatures take a leading `context.Context`** (human-approved deviation). §7 requires cancellation through a `context.CancelFunc` tied to the HTTP server, which the ctx-free §9 signatures make impossible -- the two sections contradict each other. The corrected contracts are:
   - `TransferServer.Start(ctx context.Context, filePath string, onProgress func(stats TransferStats)) (int, error)`
   - `Streamer.StreamFile(ctx context.Context, w http.ResponseWriter, filePath string) error`
   - `Streamer.StreamZip(ctx context.Context, w http.ResponseWriter, dirPath string) error`

   *Affects §9, and Phases 3-4 which implement these.* **Wholly superseded by correction 5.** Both types named above -- `TransferServer` and `Streamer` -- have since been deleted, so this ctx-signature fix is historical only. The sentence that once stood here, "`NetworkManager` is unchanged", was also overtaken: Story 1.2 replaced it with `NetworkPort`.
5. **Neither `Streamer` nor `TransferServer` exists; the payload contract is `PayloadPort`/`PreparedPayload` and the server contract is `transfer.ServerPort`** (Stories 1.3 and 1.4). A provider-owned interface taking a path string could not re-check the source at claim time or derive a wire length from a real descriptor, so `internal/server` now owns the contract and `internal/stream` implements it. `StreamZip` is gone with it -- Epic 2 reintroduces directories through the same port. `TransferServer`/`TransferStats` went the same way in Story 1.4: a path-string-plus-callback interface could not express a capability token, a claim handshake, an event channel, or teardown guarantees, so `transfer.ServerPort` replaced it and `ProgressSnapshot` replaced `TransferStats`. `NetworkManager` was likewise replaced by the consumer-owned `NetworkPort` in Story 1.2. The binding shapes live in `docs/fairdrop-contracts.md`, which supersedes §9 wherever the two disagree. *Affects §9, §10 Phases 3-4.*

## 1. Product Overview & Philosophy
DeadDrop is an ephemeral, cross-platform local P2P file transfer desktop application.
*   **Zero-State:** No database, no persistent logs, no config files.
*   **Zero-Disk (for zips):** Directories are compressed in memory and streamed directly to the socket.
*   **Ephemeral:** The HTTP server and mDNS broadcaster only exist while a file is actively queued or transferring.

## 2. Tech Stack & Dependencies
*   **Core:** Go 1.21+ 
*   **Desktop App:** Wails v2 (`wailsapp/wails/v2`)
*   **Frontend:** React 18, TypeScript, Tailwind CSS, `framer-motion` (for state transitions)
*   **Backend Dependencies:**
    *   `github.com/hashicorp/mdns` (Local network discovery)
    *   `github.com/skip2/go-qrcode` (QR code generation)

## 3. Application State Machine
The application must strictly follow these states to prevent race conditions:
1.  **IDLE:** Waiting for a file drop. HTTP server is offline.
2.  **STAGED:** File dropped. Metadata generated, QR code created, HTTP server listening, mDNS broadcasting.
3.  **TRANSFERRING:** Receiver connected. HTTP response is actively writing. mDNS broadcast stops (to prevent dual connections).
4.  **DONE / ERROR:** Transfer completed or connection dropped. HTTP server shuts down. State resets to IDLE after a 3-second delay.

## 4. API Contracts (Go Bindings for Wails)
The Go `App` struct must expose the following methods to the frontend:

```go
// Expose to frontend via Wails
type FileMetadata struct {
    Name  string `json:"name"`
    Size  int64  `json:"size"` // Total bytes (estimated if directory)
    IsDir bool   `json:"isDir"`
    URL   string `json:"url"`
    QR    string `json:"qrBase64"` // Base64 encoded PNG
}

func (a *App) StageTransfer(absolutePath string) (*FileMetadata, error)
func (a *App) CancelTransfer() error
```

> **Superseded:** this is not the shipped shape. `transfer.FileMetadata` also carries
> `sessionId` and a non-null `warnings` array, and `StageTransfer` returns
> `*transfer.FileMetadata`, which the coordinator builds. The binding shapes live in
> `docs/fairdrop-contracts.md` ("Canonical domain values" and "Public Wails API"), which
> governs wherever the two disagree. *(Story 1.5.)*

## 5. IPC Event System (Go -> React)
Use `runtime.EventsEmit(a.ctx, eventName, payload)` to drive the React UI reactively:
*   `event: transfer-started` - Triggered when the HTTP handler receives a request.
*   `event: transfer-progress` - Payload: `{ bytesSent: int64, totalBytes: int64, percent: float64, speedBytesPerSec: float64 }`. Emitted every ~250ms during active transfer.
*   `event: transfer-complete` - Triggered when the HTTP handler successfully finishes writing the response body.
*   `event: transfer-error` - Payload: `{ message: string }`.

> **Superseded:** the shipped events run on one synchronous FIFO lane. Every payload
> carries `sessionId` and a `seq` that starts at 1 and increments by one, there is a
> fifth `transfer-reset` kind, progress carries an explicit `totalKnown`, and error
> carries a `{code,message}` `PublicError` rather than free text. See "Event ordering"
> in `docs/fairdrop-contracts.md`, which governs. *(Story 1.5.)*

## 6. Detailed Implementation Modules

### Module A: Network Identity (Critical Edge Case)
*   **IP Resolution:** Do NOT rely on generic hostname resolution. Iterate through `net.Interfaces()`. Ignore `net.FlagLoopback`, `net.FlagPointToPoint`, and interface names containing `docker`, `veth`, or `tun`. Grab the first valid IPv4 address on an interface that is `net.FlagUp` and `net.FlagBroadcast`.
*   **Port Binding:** Bind the HTTP server to port `0` (`127.0.0.1:0` or `0.0.0.0:0`). The OS will assign a random available port. Extract the assigned port using `listener.Addr().(*net.TCPAddr).Port`.

### Module B: The Ephemeral HTTP Server & Progress Tracking
*   Start the server in a goroutine: `go http.Serve(listener, mux)`.
*   Implement a custom `ProgressWriter` that wraps `http.ResponseWriter`:
    ```go
    type ProgressWriter struct {
        http.ResponseWriter
        TotalBytes int64
        BytesSent  int64
        Ctx        context.Context // Wails context for event emission
    }
    func (pw *ProgressWriter) Write(p []byte) (int, error) {
        n, err := pw.ResponseWriter.Write(p)
        pw.BytesSent += int64(n)
        // Calculate percent and emit Wails event here (throttle to 4Hz to avoid UI lag)
        return n, err
    }
    ```
*   Set explicit Headers: 
    *   `Content-Disposition: attachment; filename="<name>"`
    *   `Content-Length: <size>` (If single file. Omit if streaming a directory).
    *   `Cache-Control: no-store`

### Module C: On-the-Fly Directory Archiving
*   Use `io.Pipe()`.
*   **Writer Goroutine:** Traverse the directory using `filepath.WalkDir`. For each file, create a header in `archive/zip.Writer`, open the local file, and `io.Copy` it into the zip writer. Close the zip writer and pipe writer when done.
*   **Reader Goroutine (HTTP Handler):** `io.Copy(progressWriter, pipeReader)`.
*   *Error Handling:* If `filepath.Walk` encounters a permissions error, it must send an error down the pipe to terminate the HTTP stream gracefully rather than hanging.

### Module D: Frontend (React) Specifications
*   **Drag & Drop Overlay:** Use Wails `EnableFileDrop: true` in the application options to allow capturing absolute file paths natively from the OS. In React, configure the `wails_file_drop` runtime event listener.

    > **Corrected:** both API names in this bullet are wrong. The option is `DragAndDrop: &options.DragAndDrop{EnableFileDrop: true}`, and the frontend uses the `OnFileDrop`/`OnFileDropOff` runtime helpers (underlying event `wails:file-drop`). See "Phase 1 Corrections" at the top.
*   **Components needed:**
    *   `DropZone.tsx`: An empty state prompting the user to drag a file.
    *   `StagedView.tsx`: Displays the filename, size, and the QR code.
    *   `TransferView.tsx`: Displays a large progress bar and MB/s speed metric.
*   **Animations:** Use CSS transitions or `framer-motion` to smoothly transition between IDLE -> STAGED -> TRANSFERRING.

## 7. Security & Edge Cases
*   **CORS:** The HTTP server must explicitly set CORS headers `Access-Control-Allow-Origin: *` to allow mobile browsers to initiate downloads without preflight blocking.
*   **Concurrent Requests:** The server must reject requests if a transfer is already actively in progress (return `423 Locked`).
*   **Cancellation:** If the user clicks "Cancel" in the UI, the Go backend must trigger a `context.CancelFunc` tied to the HTTP server to forcefully drop the connection and close the listener.
*   **Firewall:** Advise the user/agent that running this will trigger the macOS/Windows local firewall prompt the first time the binary runs. Bind to `0.0.0.0` to allow local LAN access.

## 8. Project Scaffolding & Directory Structure
The AI agent must initialize the project using the standard Wails CLI template to ensure the correct build hooks and configuration files are present.

**Initialization Command:**
`wails init -n deaddrop -t react-ts`

**Target Directory Structure:**
```text
deaddrop/
├── build/                # macOS/Windows icons and manifests
├── frontend/             # React SPA
│   ├── src/
│   │   ├── components/   # DropZone.tsx, ProgressBar.tsx, QRCode.tsx
│   │   ├── App.tsx       # Main state machine (IDLE -> STAGED -> TRANSFERRING)
│   │   └── style.css     # Tailwind imports
│   ├── package.json
│   └── vite.config.ts
├── internal/             # Go Backend Logic
│   ├── network/          # IP resolution, mDNS broadcasting
│   ├── server/           # Ephemeral HTTP server, ProgressWriter
│   └── stream/           # io.Pipe logic, on-the-fly zip archiving
├── app.go                # Wails lifecycle hooks and exposed frontend API
├── main.go               # Entry point, Wails configuration
└── wails.json            # Project configuration
```

## 9. Core Go Interfaces (Implementation Guide)
To prevent the agent from writing monolithic code in `app.go`, enforce these interfaces in the `internal/` packages:

> **Corrected:** the beacon service is `_fairdrop._tcp`, not `_deaddrop._tcp`. And `TransferServer.Start`, `Streamer.StreamFile`, and `Streamer.StreamZip` each take a leading `ctx context.Context` that the signatures below omit -- without it the cancellation §7 requires cannot be plumbed through. See "Phase 1 Corrections" at the top.
>
> **Superseded:** every signature in the note above is gone -- `NetworkManager` in Story 1.2, `Streamer` in Story 1.3, `TransferServer` in Story 1.4. See correction 5.

```go
// SUPERSEDED by correction 5 -- NetworkManager was replaced in Story 1.2, and
// the service is _fairdrop._tcp (correction 3), never _deaddrop._tcp.
// internal/transfer owns the contract; internal/network implements it.
type NetworkPort interface {
    GetLocalIP(ctx context.Context) (netip.Addr, error)
    StartBeacon(ctx context.Context, request BeaconRequest) error
    StopBeacon() error
}

// SUPERSEDED by correction 5 -- Streamer was replaced in Story 1.3.
// internal/server/server.go owns the contract; internal/stream implements it.
type PreparedPayload interface {
    DownloadName() string
    Size() (bytes int64, known bool)
    WriteTo(ctx context.Context, dst io.Writer) error
    Close() error
}

type PayloadPort interface {
    Prepare(ctx context.Context, item transfer.StagedItem) (PreparedPayload, error)
}

// SUPERSEDED by correction 5 -- TransferServer was replaced in Story 1.4.
// internal/transfer owns the contract; internal/server implements it.
// ServerStartRequest, ClaimAuthorizer, ServerHandle and ServerEvent are defined
// in docs/fairdrop-contracts.md, which is binding wherever it disagrees here.
type ServerPort interface {
    Start(ctx context.Context, request ServerStartRequest, authorizer ClaimAuthorizer) (ServerHandle, error)
    Stop() error
}
```

## 10. AI Agent Execution Plan (Step-by-Step)
Instruct the coding agent to complete the project strictly in these phases. Do not allow it to move to the next phase until the current one compiles successfully.

*   **Phase 1: Foundation & Wails Config.** Set up `main.go` with `wails.Options`. Enable `EnableFileDrop: true`, set `WindowStartState` to normal, and remove window frames if desired for a modern look.

    > **Corrected:** `EnableFileDrop` is not a top-level option -- use `DragAndDrop: &options.DragAndDrop{EnableFileDrop: true}`. The frame question is settled: standard OS chrome, `Frameless: false`. See "Phase 1 Corrections" at the top.
*   **Phase 2: Network Utilities.** Implement `internal/network`. Write the logic to filter out loopback/docker interfaces. This is historically tricky; ensure it explicitly looks for `net.FlagUp` and `net.FlagBroadcast`.
*   **Phase 3: The Streaming Engine.** Implement `internal/stream`. *Per correction 5, the file half of this landed in Story 1.3 as `PayloadPort`/`PreparedPayload`, not `Streamer`.* The `io.Pipe` and `archive/zip` logic belongs to Epic 2. **Crucial:** The agent must run `zipWriter.Close()` *before* `pipeWriter.Close()`, otherwise the zip file will be corrupt (missing the central directory).
*   **Phase 4: The Ephemeral Server.** Implement `internal/server`. Bind to `0.0.0.0:0`. Hook up the `PayloadPort` (*not `Streamer`* -- see correction 5) and wrap the `http.ResponseWriter` with the progress tracker.
*   **Phase 5: Wails IPC Binding.** Wire the `internal` packages into `app.go`. Implement `StageTransfer` and `CancelTransfer`. Ensure Wails `context.Context` is passed down so `runtime.EventsEmit` works.
*   **Phase 6: Frontend React.** Build the UI. Listen for the `wails_file_drop` event. Use `framer-motion` for smooth transitions between the Idle, Staged, and Transferring views.

    > **Corrected:** there is no `wails_file_drop` event -- register `OnFileDrop(callback, useDropTarget)` and clean up with `OnFileDropOff()`. See "Phase 1 Corrections" at the top.

## 11. AI Hallucination Guardrails & "Gotchas"
*   **Memory Leaks:** Do NOT use `ioutil.ReadAll` or `os.ReadFile` anywhere in the transfer pipeline. Files must be read in chunks (e.g., `io.Copy` with a standard 32KB buffer) to maintain the $<20\text{ MB}$ memory footprint regardless of file size.
*   **Wails Context:** The `context.Context` provided in the Wails `startup(ctx)` hook is *mandatory* for emitting events. The agent must save this to the `App` struct and pass it to the server package.
*   **mDNS Conflicts:** The agent must ensure the mDNS service name is unique enough (e.g., append the machine's hostname to the instance name like `DeadDrop - MacBookPro`) so multiple devices on the same network don't collide.
*   **Goroutine Leaks:** When the HTTP server shuts down or the user cancels, ensure the `io.Pipe` writers and readers are explicitly closed, or the zipping goroutine will hang forever waiting for a read.
*   **Build Target:** The final build command for the agent to suggest is `wails build -upx` (to compress the binary further) or `wails build -platform windows/amd64,darwin/universal` for cross-compilation.
