# Epic 1 Context: Share One File with a Nearby Device

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Enable a sender to choose exactly one regular file through native drop or keyboard-accessible browse controls, prepare a QR code and direct capability URL, and let exactly one nearby browser download the file over the local network. The experience must remain simple and honest while the system enforces safe validation, transactional setup, bounded streaming, deterministic lifecycle behavior, cancellation, and complete resource cleanup.

## Stories

- Story 1.1: Validate and Describe One File Selection
- Story 1.2: Select and Advertise a Reachable LAN Endpoint
- Story 1.3: Prepare and Stream a Regular File Safely
- Story 1.4: Serve a One-Shot Capability Download
- Story 1.5: Stage and Authorize a Transfer Transactionally
- Story 1.6: Complete, Cancel, and Reset the Transfer Lifecycle
- Story 1.7: Expose Safe Transfer Commands through Wails
- Story 1.8: Manage Session-Scoped Frontend State and Events
- Story 1.9: Render the Paper Relay Transfer Views
- Story 1.10: Meet the Accessibility and Recovery Contract

## Requirements & Constraints

- Accept one absolute regular-file path. Reject empty, multiple, missing, directory, link-like, reparse, and special-file selections with stable safe errors before acquiring network resources. Preserve spaces, Unicode, and native-supported long or UNC paths without shell invocation or destructive rewriting.
- Generate independent session and capability identifiers with at least 128 cryptographically random bits. Never persist them; disclose the capability only in the local URL, QR code, and receiver request path.
- Choose an eligible LAN IPv4 address deterministically and advertise only non-sensitive protocol metadata. Discovery failure is a warning when HTTP and QR remain usable.
- Bind a ready listener on all interfaces at a random port. Only the first exact-token GET may claim the transfer; disguise wrong methods, routes, and tokens as 404, and expose no payload before synchronous authorization succeeds.
- Revalidate the selected file from an opened descriptor before headers, derive its length from that descriptor, and stream exact bytes with O(buffer) memory. Create no payload copy, database, history, settings, telemetry, or persistent log.
- Count only bytes accepted by the HTTP response. Progress is finite, clamped, wire-accurate, and emitted at most four times per second plus required terminal progress.
- Cancellation and shutdown must leave every listener, connection, beacon, descriptor, worker, callback, and timer quiescent. Cancellation is not a transfer error.
- V1 is trusted-LAN plain HTTP, not a confidential channel. Product copy must not imply encryption, receiver identity, pairing, synchronization, universal compatibility, or receiver-side save completion.
- Verification must cover boundary behavior, source mutation, claim and teardown races, bounded memory, protocol responses, frontend event correlation, accessibility, native Wails configuration, and one real nearby-browser file download.

## Technical Decisions

- Use ports and adapters around one lifecycle coordinator. The composition root constructs concrete adapters; the Wails layer translates commands, dialogs, lifecycle hooks, and notifications only. Consumer-owned ports replace transitional provider-owned interfaces rather than coexisting with shadow contracts.
- Only the coordinator mutates lifecycle state. A mutex protects state and immutable session identity but is never held across an adapter call. After each unlocked setup or claim step, revalidate session, expected state, cancellation generation, and shutdown before using the result.
- A per-session operation lease serializes setup, authorization, cancellation, teardown, and reverse-order unwind. The protected STAGED and TRANSFERRING commits are linearization points; claim authorization stops discovery before committing transfer or allowing headers and bytes.
- Stage is transactional and emits no lifecycle event before successful acknowledgement. One synchronous FIFO event lane begins at sequence 1. Events and timers are session-scoped; stale identities, non-increasing sequences, invalid payloads, and post-terminal progress are ignored.
- Public failures use stable coded errors and fixed safe messages. Unknown or malformed failures become the safe transfer fallback; arbitrary adapter text, source paths, and capability tokens never cross the Wails, HTTP, discovery, or UI boundaries.
- Backend lifecycle is authoritative. The frontend may show local preparation while Stage is pending, but successful metadata initializes the session. The backend owns the three-second terminal reset; the frontend has no lifecycle timer.
- Preserve the proven native Wails drop boundary and standard window chrome. Native file and directory dialogs return paths without staging automatically, and a cancelled dialog is quiet. Generated Wails bindings are regenerated, not hand-edited.
- Keep the locked Wails v2, React, TypeScript, Vite, and Tailwind v4 stack. Use the approved mDNS and in-memory QR libraries, install frontend development dependencies with the lockfile, and use Bundler module resolution across both TypeScript projects.

## UX & Interaction Patterns

- Use the Paper Relay / Terracotta Linen system: a compact utility with one current item, one next action, warm restrained surfaces, exact semantic tokens, and no dashboard styling. Follow the OS light/dark preference without a theme control; forced colors supersede the palette except for a tested QR substrate.
- Idle presents firewall preflight before the native drop target and equal File/Directory browse actions. Pending preparation never claims STAGED. Staged makes the QR primary, keeps the direct URL readonly, and shows item details, trusted-LAN disclosure, recovery help, warnings, and Cancel.
- Transferring shows actual wire bytes and visual-only throughput. Known positive totals are determinate; unknown totals use a static non-directional pattern; known-empty files use literal text without a percentage-bearing progress bar.
- Done and Error use fixed safe copy. After backend reset, the same outcome remains visible in Idle as dismissible sessionless status until Dismiss or the next Stage attempt. Reset itself neither moves focus nor announces.
- Exactly one mechanism announces each transition: either focus on an existing state heading or one pre-mounted atomic polite status region, never both. Progress speech is throttled independently of visual updates, and throughput is never spoken.
- Meet WCAG 2.2 AA with semantic keyboard equivalents, visible focus, targets at least 44px, one-column 320 CSS-pixel reflow, 200% text plus text-spacing support, reduced-motion behavior, forced-color support, and bidi-isolated full filenames.

## Cross-Story Dependencies

Stories 1.1-1.4 establish the source, network, payload, and server capabilities consumed transactionally by Stories 1.5-1.6. Story 1.7 exposes that backend contract to Wails; Story 1.8 must correlate and defensively reduce those session-scoped results before Story 1.9 renders them. Story 1.10 applies across the frontend state and view layers. Epic 2 later extends the same source and payload boundaries for directories without changing the file-transfer contracts established here; native packaging and release evidence remain Epic 3 work.
