---
stepsCompleted: [1]
inputDocuments:
  - "{project-root}/docs/fairdrop-spec.md"
  - "{project-root}/_bmad-output/implementation-artifacts/spec-phase-1-wails-scaffold.md"
  - "{project-root}/_bmad-output/implementation-artifacts/deferred-work.md"
---

# FairDrop - Epic Breakdown

## Overview

This document provides the complete epic and story breakdown for FairDrop, decomposing the requirements from the PRD, UX Design if it exists, and Architecture requirements into implementable stories.

> **Input note.** FairDrop has no separate PRD or Architecture document. `docs/fairdrop-spec.md` serves as both and is the authority for the requirements below. Its "Phase 1 Corrections" section supersedes the body wherever they disagree — all extraction below uses the corrected facts. `deferred-work.md` contributes requirements marked `[deferred]`, which are review findings already validated against the codebase.

## Requirements Inventory

### Functional Requirements

FR1: Capture the absolute path of a file or directory dropped onto the application window.
FR2: Resolve the host's LAN-routable IPv4 address, ignoring loopback, point-to-point, and `docker`/`veth`/`tun` interfaces, selecting an interface that is both up and broadcast-capable.
FR3: Bind the transfer HTTP server to port 0 and read back the OS-assigned port.
FR4: Generate transfer metadata for a staged path: name, size, directory flag, URL, and a base64 PNG QR code.
FR5: Broadcast a staged transfer over mDNS as `_fairdrop._tcp`, with an instance name unique to the host.
FR6: Stream a single staged file to the HTTP response in bounded chunks.
FR7: Archive a staged directory to zip on the fly through `io.Pipe` and stream it, without writing the archive to disk.
FR8: Emit `transfer-started` to the frontend when the HTTP handler receives a request.
FR9: Emit `transfer-progress` carrying bytesSent, totalBytes, percent, and speed while a transfer is active.
FR10: Emit `transfer-complete` when the response body finishes writing, and `transfer-error` with a message on failure.
FR11: Stop the mDNS beacon on entering TRANSFERRING, so a second receiver cannot connect.
FR12: Reject a transfer request with `423 Locked` while another transfer is already active.
FR13: Cancel an in-flight transfer on user request, dropping the connection and closing the listener.
FR14: Shut down the HTTP server and return to IDLE three seconds after DONE or ERROR.
FR15: Present distinct Idle, Staged, and Transferring views, animating the transitions between them.
FR16: Display the transfer QR code and URL in the Staged view so a phone can start the download.
FR17: Display transfer progress and throughput in MB/s in the Transferring view.
FR18 `[deferred]`: Define behavior for a multi-file drop — the frontend accepts `string[]` while the backend contract stages a single path.

### NonFunctional Requirements

NFR1: Hold memory under 20 MB regardless of payload size — chunked copying only, never `io.ReadAll`, `os.ReadFile`, or any whole-file buffering in the transfer path.
NFR2: Persist nothing — no database, no logs, no config files.
NFR3: Run the HTTP server and mDNS beacon only while a transfer is staged or active.
NFR4: Serve `Access-Control-Allow-Origin: *` so mobile browsers can download without preflight blocking.
NFR5: Set `Content-Disposition: attachment`, `Cache-Control: no-store`, and `Content-Length` for single files (omitted when streaming a directory).
NFR6: Bind `0.0.0.0` for LAN reachability, accepting a first-run OS firewall prompt.
NFR7: Throttle progress events to roughly 4 Hz to keep the UI responsive.
NFR8: Leak no goroutines — close pipe readers and writers explicitly on cancellation and shutdown.
NFR9: Close the zip writer before the pipe writer, so the archive's central directory is written and the file is not corrupt.
NFR10: Build for windows/amd64 and darwin/universal.
NFR11 `[deferred]`: Handle Windows path edge cases — spaces, non-ASCII, paths beyond 260 characters, UNC shares, and symlinks.

### Additional Requirements

- Starter template `wails init -t react-ts` — **satisfied in Epic 1**; the scaffold is committed.
- Toolchain in place: Go 1.26.7, Wails CLI v2.15.0, React 19.1, Vite 7, Tailwind v4.
- Backend dependencies still to add: `github.com/hashicorp/mdns`, `github.com/skip2/go-qrcode`.
- The three `internal/` contracts exist as interfaces with no implementations: `NetworkManager`, `Streamer`, `TransferServer`.
- `TransferServer.Start` and both `Streamer` methods take a leading `context.Context` — an approved deviation from spec §9, required because §7 mandates cancellation via `context.CancelFunc`.
- The Wails `context.Context` captured in `startup(ctx)` is mandatory for `runtime.EventsEmit`; it must reach the server package.
- `[deferred]` `TransferStats.Percent` needs defined behavior when `TotalBytes` is 0, the normal case for a streamed zip — naive division yields NaN, which `encoding/json` refuses to marshal.
- `[deferred]` `Streamer` needs a way to signal failure after headers are written, or receivers save truncated files that look complete.
- `[deferred]` `TransferServer.Stop` needs a documented idempotency contract; it is reachable twice or before `Start`.
- `[deferred]` No CI runs the verification commands, so verified state decays from the next commit.
- `[deferred]` `frontend/tsconfig.json` uses legacy `moduleResolution: "Node"`; `"bundler"` matches the shipped toolchain.
- `[deferred]` `npm ci --omit=dev` would break `tsc`, since test files sit inside the build's type-check scope.

### UX Design Requirements

> No UX design contract exists. These are extracted from spec §6 Module D and from accessibility findings in `deferred-work.md`.

UX-DR1: `DropZone.tsx` — idle empty state prompting the user to drag a file, with a visible active state while a drag hovers the zone.
UX-DR2: `StagedView.tsx` — filename, human-readable size, and the QR code.
UX-DR3: `TransferView.tsx` — large progress bar with a MB/s throughput readout.
UX-DR4: Animate IDLE → STAGED → TRANSFERRING transitions using `framer-motion` (already installed, unused).
UX-DR5 `[deferred]`: Provide a keyboard-reachable path to stage a file — a "Browse…" fallback — since drag-and-drop is currently the only input, and announce newly staged paths via an `aria-live` region.

### FR Coverage Map

{{requirements_coverage_map}}

## Epic List

{{epics_list}}
