# Adversarial Divergence Review — Second Pass

> Historical gate pass. Its findings were resolved; the authoritative final verdict is `review-adversarial.md` (PASS).

**Artifacts reviewed:** `ARCHITECTURE-SPINE.md`, binding `docs/fairdrop-contracts.md`, and explanatory `docs/fairdrop-architecture.md`  
**Lens:** construct independently compliant phase units that still fail when composed  
**Verdict:** **CHANGES REQUIRED, with substantial improvement since pass one.** The revision closes the original session handshake, synchronous claim gate, canonical port, event sequence number, state-table, discovery-failure, terminal-timer, HTTP matrix, and source-mutation gaps. The remaining blockers are narrower but still load-bearing: pre-transfer state/event contradictions, event-channel teardown deadlock, concurrent cleanup ownership, and teardown context/postconditions.

## Closed since pass one

The following earlier findings are now materially resolved and should be preserved:

- Successful Stage returns `sessionId`, and React initializes correlation only from that acknowledgement.
- `AuthorizeClaim` is a synchronous pre-header gate with a local server reservation.
- `docs/fairdrop-contracts.md` owns the cross-boundary types and port locations.
- UI events carry increasing sequence numbers through one FIFO coordinator lane.
- The command/state table covers STAGING and CLAIMING races.
- mDNS publication failure is explicitly non-fatal after HTTP/QR readiness.
- DONE/ERROR retain a separate application-lifetime UI lease and reset timer.
- The HTTP method/token/replay matrix and filesystem mutation/link policies are binding.

## 1. Critical — pre-success and pre-transfer failures contradict the state and event contracts

**Relevant rules:** AD-2, AD-4, AD-8, AD-9; contract lines 214–255.

Three binding statements cannot all hold:

1. The spine diagram sends setup failure from `STAGING` to `ERROR`.
2. Cancel during `STAGING` publishes reset, although Stage must return `cancelled` and never returns the session ID.
3. The sole event grammar begins with `transfer-started`, but beacon-stop failure denies authorization, enters ERROR, and must not publish started.

Two independently compliant units then fail together:

- A coordinator follows the state table and emits reset for a cancelled STAGING session, or error/reset when beacon Stop fails in CLAIMING.
- A reducer follows AD-8 and initializes `(sessionId, lastSeq)` only from successful Stage. It must discard the STAGING reset because that ID was never acknowledged. For beacon-stop failure it either rejects `transfer-error` because no `transfer-started` preceded it, or violates the declared event order to render the error.

The diagram also has no `CLAIMING -> ERROR` edge even though AD-9 and the contract require that outcome. No stable error code names beacon-stop failure.

**Hole to close:** Define separate event grammars and align the diagram/table:

- Setup failure or Cancel before Stage acknowledgement: unwind to IDLE, return the command error, emit no lifecycle event, and create no terminal lease.
- Failure after successful Stage but before transfer authorization: publish `transfer-error` then reset without a started/progress event; add `CLAIMING -> ERROR` and a stable safe code.
- Successful claim: started, progress*, final progress, complete/error, reset.
- User Cancel after successful Stage: reset only.

ERROR should be a visible lease only for a session the UI already learned through successful Stage. Remove `STAGING -> ERROR` unless Stage also returns an acknowledged session/result, which the current API intentionally forbids.

## 2. Critical — an allowed server event channel can deadlock quiescent Stop

**Relevant rules:** AD-4, AD-12; contract lines 139–147 and 173–180.

`ServerHandle.Events` has no buffering or draining guarantee. Terminal events “may not be dropped,” while Stop waits for every handler/event producer and closes the channel.

An incompatible but fully natural pair is:

- The server uses an unbuffered `Events` channel. When cancellation closes the connection, its handler must send one `ServerFailed` terminal event before returning.
- The coordinator uses one control loop to consume events and mutate state. A user Cancel enters that loop and calls quiescent `ServerPort.Stop`; while it is inside Stop it is no longer receiving from `Events`.

The handler blocks sending the required terminal event, Stop blocks waiting for the handler, and the coordinator blocks in Stop. Each unit follows its ownership contract.

**Hole to close:** Bind a delivery topology that cannot require the Stop caller to receive while Stop is waiting. For example, require a dedicated coordinator drainer to remain active until channel close and forward signals to the state lane, while Stop runs independently; alternatively reserve sufficient non-blocking terminal capacity and specify cancellation behavior. Narrow “terminal exactly once” to natural complete/fail outcomes—Cancel/Shutdown may close without a terminal signal because their UI error is suppressed. State that no server producer may block teardown on event delivery.

## 3. Critical — Cancel can race claim/setup by launching a second teardown over the same ports

**Relevant rules:** AD-2, AD-4; command table lines 216–232.

“One idempotent teardown” does not identify which goroutine owns unwind when an external operation is in flight. Idempotence does not imply concurrent safety.

One conforming coordinator can implement `AuthorizeClaim` by entering CLAIMING, unlocking, and calling `StopBeacon`. A concurrent Cancel sees CLAIMING and starts reverse teardown, calling `StopBeacon` and `ServerPort.Stop`. The server Stop waits for the request handler; the handler is waiting for AuthorizeClaim; AuthorizeClaim may be contending with Cancel's second beacon Stop. A network adapter may safely support repeated sequential Stop calls, as required, but not concurrent calls. Nothing currently forbids this composition.

The same issue exists when Cancel/Shutdown races a long Stage setup step: the Stage transaction and Cancel can both believe they are responsible for reverse unwind.

**Hole to close:** Specify single-flight cleanup ownership, not just idempotence. A robust contract is: the goroutine holding the current session operation lease is the only one that performs port Start/Stop/unwind; Cancel/Shutdown marks the generation cancelled, cancels its live context, and waits on a teardown completion channel. If another path wins teardown, all later callers join the same completion rather than invoking adapters again. If concurrent Stop is intentionally required instead, state it on every port and test it explicitly.

## 4. Critical — cancelled control contexts can make “quiescent Stop” return without quiescence

**Relevant rules:** AD-4, AD-12; port postconditions lines 173–180; Shutdown row line 230.

Server Start and streaming use the session context, which Cancel deliberately cancels. `ServerPort.Stop(ctx)` and `StopBeacon(ctx)` also accept contexts, but the contract does not say which context the coordinator passes. Passing the just-cancelled session context is idiomatic and can cause Stop to return immediately with `context.Canceled` while listeners/workers remain. Wails' shutdown context can likewise already be cancelled or short-lived.

A second divergence follows from the postcondition being conditional on success: one adapter may return deadline exceeded with work still alive, while the coordinator follows the state table, publishes reset, clears the session, and enters IDLE. Another adapter may treat the deadline as advisory and refuse to return until quiescent. Both fit “bounded by its context” plus “after success ... quiescent,” but only one satisfies AD-4's unconditional resource guarantee.

**Hole to close:** Separate data-plane cancellation from cleanup control. Require a fresh bounded teardown context derived from application lifetime (not the cancelled session or request context), and state the postcondition on every return. Prefer Stop methods that force-close and return only once quiescent, with any returned error describing the cleanup rather than permitting live ownership to escape. If a timeout can leave resources live, the coordinator must remain CLOSING/non-IDLE and report a defined fatal cleanup outcome; it may not clear the generation or return successful Cancel/Shutdown.

## 5. High — `PreparedPayload` ownership and concurrent Close semantics are undefined

**Relevant rules:** AD-4, AD-6, AD-12; payload contract lines 182–199.

The contract requires `Close` to be idempotent but does not state who owns it after Prepare succeeds, whether it is called exactly once, or whether it may race `WriteTo`.

Two valid implementations can deadlock or corrupt each other:

- A server calls `PreparedPayload.Close` concurrently with `WriteTo` during Cancel to force an open file/pipe to unblock.
- A stream adapter assumes Close follows WriteTo and closes pipe/file state without synchronization, relying on the context and connection closure to interrupt the writer.

Both satisfy idempotent Close in sequential tests. Their composition races and can panic, truncate without a classified error, or leave a ZIP goroutine blocked.

**Hole to close:** Bind the lifecycle. On successful Prepare, the server owns exactly one Close. Recommended cancellation order is: cancel payload context, force-close the HTTP connection/destination to unblock Write, wait for `WriteTo` to return, then call Close; Close does not run concurrently with WriteTo. If concurrent Close is the chosen interrupt mechanism, make that a required concurrency-safe postcondition instead. Cover Prepare-success/header-failure, normal completion, receiver disconnect, Cancel, and Stop-before-Write paths.

## 6. High — the event and public-error shapes are still incomplete at the Wails seam

**Relevant rules:** AD-8, AD-12; canonical values lines 21–86 and Event lines 149–168.

`Event.Error` references `PublicError`, but that type is never defined. The tagged Event shape also has a value-typed `Progress` with `omitempty`; common Go JSON encoders still serialize a zero-valued struct, so started/reset/error payloads may unexpectedly contain an all-zero progress object. Valid field combinations per event kind are not stated.

Command failures have a second gap: the public Wails methods still return Go `error`, while the contract promises a stable code plus safe message “to React” without defining how the code survives the Wails rejection channel. One adapter can expose only `error.Error()`, while the frontend reasonably expects `{code,message}`. They agree on domain errors but do not interoperate.

The QR seam has the same smaller ambiguity: `QRPort` returns PNG bytes while `FileMetadata.qrBase64` does not say standard padded base64 versus a `data:image/png;base64,` URI. Either choice is common and requires a different React `img src` implementation.

**Hole to close:** Define `PublicError`, the exact Wails command-error representation, and per-event DTOs or a tagged-union validity table with JSON examples. Use pointer/optional payload fields if absence matters. Define `qrBase64` exactly (recommended: standard padded base64 with no data-URI prefix, and have React add the prefix) and require the PNG media type.

## 7. High — final progress truth and abnormal event-channel closure have no producer contract

**Relevant rules:** AD-7, AD-8, AD-12; server events lines 125–142 and postconditions lines 173–180.

Progress may be coalesced or dropped, so the coordinator cannot derive an accurate final snapshot from the last progress event. `ServerEvent` contains a Progress field on every kind, but nothing says Complete/Failed must carry the authoritative unthrottled totals. Two servers can comply: one populates terminal Progress, another leaves it zero and expects the coordinator to remember the last sampled value. The latter produces a stale “final” snapshot after coalescing.

The channel may also close without a terminal event because of an accept-loop or internal dispatcher failure. The coordinator has no binding transition for that condition and can remain TRANSFERRING forever or treat normal Stop closure as an error, depending on implementation.

**Hole to close:** Require natural Complete/Failed events to contain the authoritative final `ProgressSnapshot`; Complete for a known file must match the prepared length. Define whether a failed event with zero bytes carries a zero snapshot or optional absence. Channel closure without a terminal event while the session is active and teardown was not requested must synthesize `transfer_failed`; closure after coordinator-requested Cancel/Shutdown is normal and silent.

## 8. High — the privacy disclosure rule contradicts the HTTP attachment contract

**Relevant rules:** AD-5, AD-9; design HTTP section.

AD-9 says “The token and filename appear only in the Stage result/QR.” AD-5 and the design require the filename in `Content-Disposition`. The QR contains the capability URL, not the filename. A server that suppresses the filename obeys AD-9 but violates AD-5; one that sends it does the reverse.

**Hole to close:** Replace the sentence with an explicit disclosure matrix: capability token appears in the local Stage URL/QR and receiver HTTP request path only; filename appears in local Stage metadata and the authorized download response's sanitized `Content-Disposition`; neither appears in mDNS, logs, unrelated HTTP errors, or absolute-path-bearing output. State that only the selected basename/archive name—not an absolute or relative source path—is disclosed.

## 9. Medium — cleanup-error outcomes are absent from the command/state table

**Relevant rules:** AD-4, AD-9; command/state table and port postconditions.

`ServerPort.Stop` and `StopBeacon` return errors, but Cancel, terminal teardown, and Shutdown rows describe only success. Beacon Stop failure during claim is singled out as fatal, while the same failure during Cancel or completion has no state, event, retry, or return rule.

One coordinator can ignore cleanup errors and enter IDLE; another can return the error and retain ERROR/CLOSING. Both use the declared signatures. This affects stale mDNS publication, leaked listener ownership, whether Stage is safe to accept again, and whether Shutdown may return.

**Hole to close:** Define adapter error postconditions first, then add cleanup-failure rows. If Stop errors are guaranteed quiescent, record/report safely and continue the intended transition. If an error can mean still live, never enter IDLE or accept Stage; retry/escalate under a single teardown owner and make Cancel/Shutdown return a stable cleanup error. The special claim-time beacon failure should use the same model.

## 10. Medium — pre-header payload failure has no canonical HTTP or UI error mapping

**Relevant rules:** AD-5, AD-6; claim ordering lines 234–243; design streaming rules.

Authorization publishes `transfer-started` before `PayloadPort.Prepare`. Prepare can then detect `source_changed`, path disappearance, or permission failure before headers. The design merely says a pre-header error “can return a normal HTTP error,” without fixing the status or mapping the source-domain code into the server/coordinator event.

One server can return 404/410 and emit ServerFailed wrapping `source_changed`; another can return 500 and collapse it to `transfer_failed`. Both follow the current prose but produce different UI guidance and protocol tests.

**Hole to close:** Define the HTTP status and domain-code preservation for Prepare failures after a valid claim. The remote response must remain safe and reveal no path/token; the local UI should retain a specific `source_changed`/`path_not_found` code when available. Specify whether the listener is torn down immediately and confirm that the event sequence is started, optional zero-byte final snapshot, error, reset.

## Required closure set

The second-pass gate can pass once the binding documents make these narrow corrections:

1. Split event grammars for pre-ack failure, pre-transfer failure, claimed transfer, and Cancel; align all state transitions.
2. Define server event draining so quiescent Stop cannot wait on its own undelivered terminal event.
3. Establish single-flight teardown ownership across Stage, AuthorizeClaim, Cancel, terminal handling, and Shutdown.
4. Separate teardown control contexts from cancelled data-plane contexts and make Stop outcomes unambiguous.
5. Bind PreparedPayload ownership/call ordering and authoritative terminal progress/channel-close semantics.
6. Complete `PublicError`, command error transport, event union, QR encoding, privacy disclosure, and cleanup-error mappings.

Until then, phase agents can compile against the same nominal interfaces yet still deadlock on Cancel, publish events the reducer is required to ignore, clear a session while resources remain live, or disagree on public error/security behavior.
