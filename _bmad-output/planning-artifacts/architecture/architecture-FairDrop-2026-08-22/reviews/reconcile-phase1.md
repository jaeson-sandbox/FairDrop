# Phase 1 Reconciliation Review

**Reviewed:** 2026-08-22  
**Inputs:** `spec-phase-1-wails-scaffold.md`, the implemented Phase 1 contracts and Wails/React boundary, and `ARCHITECTURE-SPINE.md`  
**Verdict:** **Changes required before finalization.** The coordinator direction is compatible with the product and is a better long-term shape, but the spine currently presents a new contract model as already adopted, omits several proven Wails boundary constraints, and does not name the migrations required to make the Phase 1 interfaces satisfy AD-2, AD-5, AD-7, and AD-8.

## Findings

### R1 — Critical: AD-1 is not an adopted Phase 1 contract; it contradicts all three declared interface locations

**Evidence**

- The spine says lifecycle ports are consumer-owned in `internal/transfer`, the server consumes a streaming port, and interfaces live with their consumers (`ARCHITECTURE-SPINE.md:44-48`, `:123`).
- Phase 1 deliberately declared `NetworkManager` in `internal/network`, `Streamer` in `internal/stream`, and `TransferServer` in `internal/server` (`spec-phase-1-wails-scaffold.md:17`, `:63`, `:71-73`, `:85`). The working tree matches those locations.
- `internal/stream.Streamer` also accepts `http.ResponseWriter`, so it is explicitly coupled to the HTTP adapter rather than expressing a transport-neutral core port (`internal/stream/archiver.go:14-25`).

**Impact**

An agent following the Phase 1 acceptance contract will implement provider-owned interfaces in place. An agent following AD-1 will replace or relocate them before implementation. Both choices cannot be described as the same adopted invariant, and postponing the choice creates avoidable churn and likely import-direction drift in Phases 2-4.

**Required reconciliation**

- Keep the ports-and-adapters direction, but remove the implication that this ownership rule was adopted in Phase 1.
- Explicitly classify the three Phase 1 interfaces as compile-only transitional scaffolding superseded by the architecture before implementations begin.
- Put the network and server lifecycle ports beside their coordinator consumer, and the streaming port beside its server consumer. Concrete constructors should remain in `internal/network`, `internal/server`, and `internal/stream` and return concrete adapter types.
- Make this migration part of the first affected phase, while there are no implementations or downstream callers. Do not retain duplicate public interfaces merely to preserve the Phase 1 file layout.

### R2 — Critical: the Phase 1 server/progress contracts cannot carry the spine's session protocol

**Evidence**

- `TransferServer.Start(ctx, filePath, onProgress) (port, error)` supplies neither a capability token nor an endpoint/result containing one, and exposes only a progress callback (`internal/server/server.go:20-29`).
- AD-5 requires a cryptographically generated exact-token route and an atomic single-use claim. AD-2 requires every callback to carry a session ID. AD-8 requires started, complete, error, and reset notifications. The current server contract exposes none of the claim/completion/error lifecycle transitions (`ARCHITECTURE-SPINE.md:47-48`, `:83-87`, `:101-105`).
- Phase 1 `TransferStats` has `bytesSent`, `totalBytes`, `percent`, and `speedBytesPerSec`, but AD-7 adds `totalKnown`; AD-8 requires `sessionId` on every UI payload (`internal/server/server.go:10-15`; `ARCHITECTURE-SPINE.md:95-105`).

**Impact**

Implementing the existing interface first would force the HTTP adapter to invent session/token ownership or leak those concerns into globals and closures. It also makes directory progress ambiguous (`0/0/0` cannot distinguish unknown total from a known empty file) and leaves no explicit path for the coordinator to own state transitions.

**Required reconciliation**

- Define the coordinator-facing server port before Phase 4 implementation. Its start input/result and callback surface must account for session identity, capability-token ownership, bound endpoint, atomic claim, terminal completion, and safe failure reporting.
- Decide and state whether the coordinator generates the capability token and passes it inward, or the server generates and returns it. The current spine says a session owns random identity but does not distinguish session ID from download token; they should not become accidentally interchangeable.
- Replace or adapt `TransferStats` so unknown totals are explicit. Keep wire-byte counting in the HTTP response layer as AD-7 requires.
- Keep UI event names and Wails payload mapping out of `internal/server`; the server reports domain lifecycle signals to the coordinator, and only the Wails observer maps them to `transfer-*` events.

### R3 — High: the raw Wails drop boundary was proven in Phase 1 but disappeared from the spine

**Evidence**

- Phase 1 established four non-obvious runtime constraints: nested `DragAndDrop.EnableFileDrop`, `OnFileDrop(..., true)`, cleanup through `OnFileDropOff()`, and the inherited `--wails-drop-target: drop` CSS property (`spec-phase-1-wails-scaffold.md:25`, `:42-46`, `:61-62`, `:75`, `:92`, `:114`). The working React code and tests enforce them (`frontend/src/App.tsx:3-15`; `frontend/src/App.test.tsx:108-162`).
- Phase 1 also proves that the native callback delivers a list and deliberately renders all paths for a multi-file drop (`spec-phase-1-wails-scaffold.md:44`). AD-3 instead says Stage accepts exactly one path and zero/multiple are typed errors (`ARCHITECTURE-SPINE.md:71-75`).
- The spine's UI map and seed name components and a reducer, but state none of these Wails-specific boundary rules (`ARCHITECTURE-SPINE.md:149-166`, `:178`).

**Impact**

A Phase 6 agent can comply with the spine while accidentally replacing native Wails drop with a DOM handler, removing the CSS gate, leaking listeners, or assuming the callback itself is scalar. AD-3 is not inherently wrong, but without a boundary rule it appears to contradict the verified multi-path native input.

**Required reconciliation**

- Preserve the four native-drop constraints as an inherited platform binding in the spine or its authoritative design companion.
- State the translation rule: the Wails adapter receives `[]string`; only one path is forwarded to `Coordinator.Stage`; zero or multiple paths become a typed safe error and never silently select the first item.
- Revise the Phase 1 echo-only multi-file UI test when real staging replaces the proof UI, while retaining tests for array receipt, drop-target gating, and listener cleanup.

### R4 — High: application lifetime and transfer-session cancellation contexts are not distinguished

**Evidence**

- Phase 1 retains `App.ctx` from Wails startup because Wails event emission requires that exact runtime context, and it installed both startup and shutdown hooks (`spec-phase-1-wails-scaffold.md:55`, `:70`, `:96`; `app.go:8-25`; `main.go:48-49`).
- The corrected Phase 1 interfaces added leading `context.Context` parameters specifically to make cancellation possible (`spec-phase-1-wails-scaffold.md:85`, `:96`).
- AD-4 says each session owns a child context/cancel and one idempotent teardown, while AD-1 says `app.go` is only an adapter (`ARCHITECTURE-SPINE.md:44-48`, `:77-81`).

**Impact**

Without an explicit bridge, a future implementation may pass the long-lived Wails runtime context directly into the server as the transfer cancellation context, making Cancel ineffective, or may replace `App.ctx` with a session context and break `runtime.EventsEmit`. Shutdown ownership can also remain split between the current `App.shutdown` comment and the new coordinator.

**Required reconciliation**

- Preserve `App.ctx` solely as Wails adapter/runtime state.
- Have the coordinator derive and own a fresh cancellable child context per Stage from an application-lifetime context; pass that session context to server and streamer adapters.
- Make `App.shutdown` delegate to the coordinator's idempotent shutdown. The coordinator, not `app.go`, performs listener/beacon/session teardown.
- Update the lifecycle-hook test when constructor injection is introduced; the hooks remain a settled Phase 1 option contract.

### R5 — Medium: newly added process/install invariants are not yet reflected in the actual Phase 1 configuration

**Evidence**

- AD-3 requires Wails single-instance locking, but `appOptions` contains no single-instance option and `main_test.go` does not assert one (`ARCHITECTURE-SPINE.md:71-75`; `main.go:20-54`; `main_test.go:12-48`).
- AD-10 says Go/npm dependencies are locked and installs reproducible, while `wails.json` still invokes `npm install`, not `npm ci` (`ARCHITECTURE-SPINE.md:113-117`; `wails.json:5`). A lockfile exists, but the command does not enforce a frozen install.
- The settled standard OS frame and native drop options are pinned in `main_test.go`; project instructions require those assertions to change whenever options change.

**Impact**

These are reasonable new architecture decisions, but they are not inherited reality. Treating them as already established makes the spine overstate brownfield conformance and allows the option contract to drift untested.

**Required reconciliation**

- Mark single-instance behavior as a new decision to implement, verify the Wails v2.15 API, and add exact `main_test.go` assertions when it lands.
- Clarify whether “lock npm dependencies” means merely committing `package-lock.json` or enforcing frozen installs. If reproducibility is the intent, migrate the Wails install command to `npm ci` and verify clean-tree `wails build`.
- Keep `Frameless: false`, normal window start state, nested native-drop enablement, and lifecycle hooks in the pinned Wails options contract.

### R6 — Medium: one settled network identifier is omitted

**Evidence**

- Phase 1 corrected and reserved the DNS-SD service identifier as `_fairdrop._tcp`, and the actual `NetworkManager` comment preserves it (`spec-phase-1-wails-scaffold.md:21`, `:63`, `:93`; `internal/network/network.go:9-17`).
- AD-9 constrains mDNS metadata but never fixes the service type. The stack fixes the mDNS library, which does not determine the advertised service name (`ARCHITECTURE-SPINE.md:107-117`).

**Impact**

The sender advertisement and any receiver/discovery implementation can independently choose incompatible service types while each still follows the spine.

**Required reconciliation**

Add `_fairdrop._tcp` as an adopted protocol constant, distinct from the human-readable per-host instance name. Preserve the rule that TXT data excludes token, filename, and absolute path.

## Constraints that are already aligned

- The current `app.go` remains thin and has no Phase 2-6 behavior, so introducing the coordinator does not require untangling a monolith.
- Phase 1's leading `context.Context` additions support the cancellation direction in AD-4 and AD-6; the remaining issue is ownership and lifetime, not whether contexts exist.
- Startup/shutdown hooks, native file drop, standard window chrome, generated Wails bindings, Tailwind v4 integration, and the `.gitkeep` build seam are implemented and tested foundations. They should be preserved rather than redesigned.
- None of the three compile-only interfaces has an implementation, making this the lowest-cost point to perform the AD-1/R2 contract migration.

## Gate recommendation

Do not finalize the spine until R1-R4 are resolved in the spine/memlog and the human-facing design document describes the migration from Phase 1 scaffolding. R5-R6 should also be recorded before Phase 2 work begins so the first implementation agent receives a truthful contract rather than a mixture of current and desired state.
