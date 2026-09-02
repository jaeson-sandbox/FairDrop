# Epic 1 Context: Share One File with a Nearby Device

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

A sender picks exactly one local regular file by native drop or keyboard-reachable browse controls, FairDrop stages it behind a random-port LAN listener with a QR code and direct capability URL, and exactly one nearby browser downloads it through a clear lifecycle with honest progress and cancellation. This epic builds nearly the whole spine — validation, LAN identity and discovery, bounded streaming, the one-shot HTTP protocol, the transactional coordinator, the Wails boundary, the frontend reducer, the view layer, and the accessibility contract — so Epic 2 only extends the source and payload adapters for directories and Epic 3 only packages and ships. There is no PRD or product brief: the canonical spec plus its binding architecture, contracts, and UX companions are the entire contract, and the contracts document governs domain values, error codes, port signatures, event payloads, claim ordering, HTTP outcomes, and teardown postconditions.

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

- One process, one session, one selected root, one receiver; only IDLE accepts Stage, and zero or multiple dropped paths fail safely rather than silently taking the first.
- Invalid, missing, link-like, reparse, and special paths fail with stable typed codes before any network resource is acquired. Spaces, Unicode, and native-supported long Windows/UNC paths are supported wherever Go filesystem APIs allow; never shell out or rewrite a path.
- Staging is transactional: commit only when required resources are live, unwind in reverse on failure, and emit no lifecycle event before acknowledgement. A discovery failure alone is a non-fatal warning while HTTP and QR remain usable.
- Session ID and capability token are independent, each 128+ bits from an injectable CSPRNG, never persisted. Tokens, source paths, and raw adapter text never reach discovery records, HTTP errors, events, logs, or the UI.
- Memory stays O(buffer) at any payload size, and nothing persists: no database, settings, telemetry, logs, or payload copy.
- Progress is wire-accurate: count only accepted bytes, cap events at four per second plus required terminal snapshots, keep values finite and clamped, and use an explicit known/unknown discriminator.
- Every listener, connection, beacon, descriptor, goroutine, and timer has one owner and is quiescent before Stop returns; cancellation is never reported as a transfer error.
- Plain HTTP on a trusted LAN; copy must never imply confidentiality, receiver identity, pairing, or sync. WCAG 2.2 AA is a release gate, and race checks need a cgo-capable native runner.
- Verification spans boundary behaviour, source mutation, claim and teardown races, bounded memory, protocol responses, frontend event correlation, accessibility, native Wails configuration, and one real nearby-browser download.

## Technical Decisions

- Ports and adapters around one coordinator that owns lifecycle state; network, HTTP server, streaming, QR, and Wails are adapters. `main.go` composes, `app.go` only translates Wails commands, dialogs, hooks, and events, and the coordinator imports no Wails API or concrete adapter.
- Delete each transitional Phase 1 provider-owned interface as its consumer-owned replacement lands — no duplicate or conversion-only shadow types.
- A mutex guards state and session identity and is never held across an external call; after each unlocked adapter call, revalidate session, state, cancellation generation, and closing flag. The mutex-protected STAGED and TRANSFERRING commits are the linearization points, and one per-session operation lease performs all adapter Start/Stop/unwind work, held through synchronous publication of the started event.
- Authorization is a synchronous handshake — reserve, stop the beacon, commit, publish — with no payload opened and no header or event emitted before it succeeds.
- One synchronous FIFO event lane starts at `seq=1` and increments by one; coalesced snapshots get no sequence, no progress is accepted after a terminal outcome, and a drainer keeps delivery from blocking teardown.
- Errors wrap with `%w` behind stable codes via an `errors.As`-compatible contract; unknown errors map to a fixed safe fallback, and the Wails error formatter serializes `{code,message}` JSON the frontend parses with that same fallback.
- Conventions: lowerCamelCase JSON, `transfer-*` event names, `context.Context` first, injected clocks and entropy in coordinator tests, and no frontend lifecycle timers — the three-second terminal reset is a backend lease.
- Stack: Wails v2.15.0 with the existing locked frontend stack, `hashicorp/mdns` v1.0.7, `boombuler/barcode` v1.1.0 (superseding the inactive QR dependency), Node 24 LTS pinned, `npm ci` with dev dependencies, both TypeScript projects moved together to `moduleResolution: "Bundler"`, Tailwind v4 via the Vite plugin with no v3/PostCSS config.
- Preserve the proven Wails input boundary and application-lifetime runtime context, regenerate Wails bindings rather than editing them, and keep option assertions pinned in `main_test.go`.
- The webview is a secure context on Windows (`http://wails.localhost`) and not on macOS (the custom `wails://` scheme), so `navigator.clipboard`, `crypto.subtle`, geolocation and media capture are undefined on one supported platform with no error. Anything gated that way goes through a bound Go command; the clipboard already does. The native window's `BackgroundColour` must likewise track `--color-canvas`, since it paints before the webview.

## UX & Interaction Patterns

- Paper Relay / Terracotta Linen tokens drive all visual decisions through Tailwind v4 CSS variables. Light/dark follow the OS preference with no theme control and no opposite-theme flash; forced colors supersede the palette except the tested QR substrate; color is never the sole cue.
- Every literal string comes from the experience-spine copy registry by stable key, including the fixed error-message table rendered verbatim; banned vocabulary (secure, private, pair, sync, AirDrop naming, universal-compatibility claims) never appears.
- Idle leads with firewall preflight guidance ahead of the selection controls; a local pending surface covers an outstanding Stage without claiming backend STAGED; Staged makes the QR primary beside a readonly URL, disclosures, and Cancel; Transferring shows wire bytes with visually-only throughput; terminal views show fixed safe text and never render cancellation as an error.
- After a terminal outcome, reset clears the backend session while the same visible node stays in Idle as dismissible sessionless status until Dismiss or the next Stage; no frontend timer removes it, and reset moves no focus and makes no announcement.
- Three progress presentations with matching ARIA — determinate for known positive totals, a static non-directional pattern for directory/unknown totals, and literal text with no percentage-bearing progressbar for known-empty files.
- Exactly one announcement owner per transition (a focus move to a state heading or the single pre-mounted atomic polite announcer, never both), with assistive progress speech throttled separately from visual refresh and throughput never spoken.
- Accessibility floor: 320 CSS-pixel one-dimensional reflow, 200% text with text-spacing overrides, targets at or above 44px, reduced motion preserving text and state, bidi-isolated names with full-value access, and every focus target proven to exist before focusing.
- Receivers see only generic browser failures, so sender-side help explains wrong or expired links, a competing opener, a changed source, and guest or client isolation, directing the user to Cancel and make a fresh link. FairDrop never predicts, restyles, or duplicates the OS firewall prompt.

## Cross-Story Dependencies

- Stories 1.1–1.4 build the adapters that 1.5–1.6 compose, and 1.4 consumes the payload port defined in 1.3. Story 1.5 establishes the session, lease, and revalidation machinery that 1.6 extends for progress, terminal outcomes, cancel, reset, and shutdown.
- Story 1.7 fixes the DTO shapes 1.8's reducer mirrors, 1.9 renders what 1.8 exposes, and 1.10 spans 1.8 and 1.9; Story 1.8 also carries the shared frontend toolchain migration 1.9 and 1.10 depend on.
- Correlation is a security boundary, not tidiness: Wails' frontend `EventsEmit` notifies same-window listeners before it forwards anything to Go, so any script in the webview can deliver a lifecycle event to every subscriber. The `(sessionId, lastSeq)` rule in Story 1.8's reducer is the only defence, and no backend change can substitute for it.
- Epic 2 extends the same source and payload ports for directories without changing the contracts fixed here, so keep the file-only adapters open to that extension. Epic 3 owns single-instance behavior, packaging, and release smoke tests, but Epic 1 still closes with its own native check that one file can be selected and downloaded exactly once by a nearby browser.
- Epic 1 keeps generating two kinds of debt it is not allowed to fix in place, and Epic 3 now owns both: Story 3.4 takes every unbounded lifecycle wait, the deadlock and wedge risks, and the inert dropped-event counter; Story 3.5 takes the coded failures whose fixed copy describes a state they do not fit. Record such a finding in `deferred-work.md` with an `owner:` rather than widening the story in hand -- a test fails if an entry names no live story.
