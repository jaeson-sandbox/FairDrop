# Epic 2 Context: Share One Folder with a Nearby Device

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Extend the proven one-item transfer journey from regular files to directories. A sender stages one safe folder through the established workflow and a nearby browser receives a valid ZIP, while FairDrop preserves its defining constraints: no temporary payload archive, no payload-sized memory or retained file index, no unsafe filesystem traversal, and honest unknown-total progress. The completed one-file path must not regress.

## Stories

- Story 2.1: Validate and Stage One Directory
- Story 2.2: Stream a Safe Directory ZIP

## Requirements & Constraints

- Accept exactly one existing, absolute, non-link directory root. Recursive preflight calculates logical size from regular files only, preserves an empty directory as valid, and retains no per-entry index after inspection.
- Reject symbolic links, reparse points, junctions, nested link-like entries, and non-regular special files without following them. Missing entries, permission failures, cancellation, and size overflow must return stable safe errors without exposing source paths.
- Preserve spaces, Unicode, native-supported long Windows paths, and UNC roots through native filesystem APIs. Do not invoke a shell or destructively normalize source paths.
- Directory staging returns the same complete immutable transfer metadata as file staging, with the directory flag, root name, logical display size, session identity, capability URL, QR image, and non-fatal warnings. Existing file behavior must remain unchanged.
- The receiver must obtain a browser-compatible ZIP containing exactly one top-level root. Empty directories and empty files remain representable; archive entry names must never be absolute, volume-qualified, empty, dot-dot, or traversal-bearing.
- Runtime product data remains ephemeral. Do not create a temporary archive, build a payload-sized index, persist metadata, or read the payload into memory. Preflight uses only traversal overhead and streaming remains O(buffer) in payload size.
- Filesystem contents are not snapshotted. Revalidate entries during streaming, allow ordinary additions, removals, or in-place changes only under the defined unsnapshotted policy, and fail safely when a change creates an invalid, link-like, special, or out-of-root source.
- Directory progress reports actual ZIP bytes written to the HTTP response with an unknown total: `totalKnown=false`, `totalBytes=0`, and `percent=0`. Directory responses omit `Content-Length` while keeping the shared safe download, cache, CORS, and nosniff headers.
- Verification must cover nested and empty trees, unsafe entries, mutation, permissions, cancellation, receiver disconnects, Unicode and platform-capable long/UNC paths, bounded memory, ZIP central-directory integrity, close ordering, goroutine exit, and a native nearby-browser ZIP smoke test proving no temporary payload file appears on disk.

## Technical Decisions

- Extend the existing consumer-owned source contract and server-owned payload contract; do not introduce replacement or shadow provider interfaces. The existing coordinator remains the sole lifecycle and transactional setup authority.
- Directory payload preparation is lazy: revalidate the root, provide a sanitized `.zip` download name and unknown size, but perform no full traversal until writing begins.
- Stream ZIP output through `io.Pipe`. Every entry is reached through safe relative traversal, re-`Lstat`ed before opening, and emitted below one normalized slash-separated root name.
- Close each entry promptly and close the ZIP writer before the pipe writer so the central directory reaches the receiver. Cancellation, read/traversal failure, destination failure, and disconnect converge on one idempotent close path for the current entry, ZIP writer, both pipe ends, worker, and prepared payload.
- A failure after response headers or payload bytes begin must terminate the connection without appending an error body and surface through the existing server failure lifecycle. Buffer size and per-entry compression remain benchmark-driven tuning choices; neither may weaken bounded memory, cancellation, or archive compatibility.

## UX & Interaction Patterns

- Folder selection reuses the existing surfaces: the native drop target and the semantic Select Directory action, both keyboard-reachable, with a cancelled dialog staying quiet.
- Local preparation uses the existing Stage Pending surface and its cancel-preparation control. The pending copy names the item kind by stable key: folder wording for the directory browse action, file wording for the file action, and kind-neutral "item" wording for a native drop, where the kind is not yet known when the pending surface appears. Pending state never claims backend STAGED, and an obsolete promise from a superseded request generation cannot commit state.
- Staged keeps the QR-first handoff: the code is the primary path and the direct link is the fallback. It shows the sanitized, bidi-isolated folder name and its logical size, plus the fixed note that the folder downloads as a ZIP. The item summary must keep logical size visibly distinct from the unknown ZIP wire total.
- Transferring uses the unknown-total treatment: no `aria-valuenow`, a static non-directional pattern, the unknown-total status string, live wire bytes, and visual-only throughput. Never infer a percentage from logical source size.
- Reduced motion must leave the unknown-total state understandable without animation; text, pattern, and live byte counts carry it. Assistive progress speech stays throttled and meaningful-change gated, and throughput is never spoken.
- Completion copy describes only sender-observed sending. Where the browser saved the ZIP, whether it opened, and Files integration are receiver-owned and must not be claimed.
- All literal strings come from the copy registry by stable key, including the fixed public error messages; banned vocabulary stays out. Reuse the existing visual token set and components rather than inventing folder-specific styling; the paper-voice display face is bundled locally at one weight and is never fetched over the network.
- Generic receiver 404/423/410 pages remain an accepted V1 limitation covered by the existing sender-side help copy; folder transfers introduce no receiver page.

## Cross-Story Dependencies

Story 2.1 establishes safe directory identity, logical metadata, and staging behavior that Story 2.2 consumes. Story 2.2 must repeat safety checks at stream time because preflight is not a snapshot. Both stories depend on the existing single-use capability server, cancellation/teardown ownership, session-scoped events, and unknown-total frontend state; neither may regress the completed one-file path. The cross-device folder-ZIP checks in the native verification matrix remain open and are gated by Epic 3 release and smoke-test work.
