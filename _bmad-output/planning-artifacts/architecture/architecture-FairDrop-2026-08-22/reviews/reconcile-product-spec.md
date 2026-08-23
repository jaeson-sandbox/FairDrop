# Product-Spec Reconciliation Review

**Reviewed:** `docs/fairdrop-spec.md` (with Phase 1 Corrections taking precedence) against `ARCHITECTURE-SPINE.md`  
**Date:** 2026-08-22  
**Review type:** Binding requirement, constraint, and corrected-fact reconciliation  
**Verdict:** **CONDITIONAL PASS — architecturally aligned, but the spine is not yet self-sufficient as the binding handoff.**

The selected architecture preserves the product's essential shape: one ephemeral LAN transfer, bounded-memory file and directory streaming, no runtime persistence, a single lifecycle owner, random-port HTTP, single-receiver claiming, cancellation, backend-authoritative UI events, and the corrected FairDrop/Wails/context facts where they affect the core design. The departures are generally thoughtful improvements rather than regressions.

However, the spine declares that it binds all FR1–FR18 and NFR1–NFR11 while several concrete product contracts are absent from its rules. Most are explained in `docs/fairdrop-architecture.md`, but a future agent treating the spine as the terse binding source could implement incompatible behavior. The findings below should therefore be reconciled into the spine or explicitly delegated by reference to a stable contract.

## Findings

### RPS-1 — The required mDNS stop transition is not stated in any binding rule

**Severity:** High  
**Source:** Product spec §3, §5, §7 and §10 Phase 4; FR11 in the requirements inventory  
**Requirement:** mDNS exists while a transfer is staged, and the beacon stops when the first receiver begins transferring so the staged offer is no longer discoverable.

The spine assigns the beacon to the session in AD-4 and protects the HTTP claim in AD-5, but no rule says that the coordinator must stop mDNS on `STAGED -> TRANSFERRING`. The capability map says FR11 is governed by AD-4, AD-5, and AD-8, yet none contains that transition effect. This is a true traceability gap: an implementation can satisfy those ADs while leaving the beacon active throughout the transfer.

The companion design document contains the correct behavior in its Download transaction, step 3. It has not landed in the binding spine.

**Reconciliation needed:** Make stopping the beacon part of the atomic first-claim transition, while preserving teardown idempotency.

### RPS-2 — The corrected native file-drop contract did not land in the spine

**Severity:** High  
**Source:** Phase 1 Corrections 1–2; corrected §6 Module D and §10 Phases 1/6  
**Correct facts:**

- Wails uses `DragAndDrop: &options.DragAndDrop{EnableFileDrop: true}` rather than a top-level option.
- React uses `OnFileDrop(callback, useDropTarget)` and `OnFileDropOff()`, not a `wails_file_drop` listener.
- Drop targeting is gated by inherited CSS `--wails-drop-target: drop`, not a DOM handler or class.

The spine retains Wails v2 in AD-10 and maps FR1 to `app.go`/the frontend, but none of these three corrected integration facts appears in an invariant, convention, structural note, or explicit inherited-contract reference. These are exactly the facts most likely to regress when another agent implements Phase 6, because the body of the source spec still contains the obsolete instructions.

The top-level application option is already implemented and pinned by Phase 1 tests, but the frontend runtime helper and CSS gate remain future-facing implementation constraints.

**Reconciliation needed:** Bind Phase 6 to the corrected Wails helper/cleanup and inherited CSS property, or explicitly identify the Phase 1 test/config contract plus the source-spec corrections as controlling.

### RPS-3 — The public Wails command and metadata contract is only implied

**Severity:** Medium  
**Source:** Product spec §4  
**Requirement:** Expose `StageTransfer(absolutePath string) (*FileMetadata, error)` and `CancelTransfer() error`; return metadata with the JSON fields `name`, `size`, `isDir`, `url`, and `qrBase64`.

AD-1 gives `app.go` command-translation responsibility, and the capability map assigns commands/UI to it, but the spine never pins the method names, signatures, or `FileMetadata` wire shape. AD-3 additionally introduces typed errors for zero or multiple paths, while the specified public Stage method still accepts one string. That is compatible if native drop validation happens before the call, but the boundary is not explicit.

The companion design document says that `FileMetadata` remains the Stage result, but does not spell out the exact JSON tags either. A future agent could satisfy the spine with renamed commands or incompatible payload fields.

**Reconciliation needed:** Preserve the §4 Wails API as a stable outer-adapter contract, and state where the `string[]` native-drop result is reduced to the exactly-one-path coordinator request.

### RPS-4 — Network-selection and mDNS-identity constraints are not captured in the governing ADs

**Severity:** Medium  
**Source:** Product spec §6 Module A, §9 and §11; FR2 and FR5  
**Requirements:** Select a non-loopback IPv4 only from up, broadcast-capable, non-point-to-point interfaces; exclude interface names containing `docker`, `veth`, or `tun`; advertise `_fairdrop._tcp`; make the instance name unique enough to avoid collision, for example by including the hostname.

The spine correctly names `_fairdrop._tcp` in AD-9 and assigns FR2/FR5 to `internal/network`. It does not state the mandatory address filters or unique-instance rule. The Deferred section says only that Phase 2 selection must be deterministic. Determinism alone does not prevent choosing a loopback, tunnel, down, or non-broadcast interface.

The companion design document contains a stronger network-selection section and requires a unique instance name, but these facts are absent from the binding rules claimed by the spine.

**Reconciliation needed:** Add the minimum selection predicate and uniqueness requirement to the binding network rule; leave scoring/tie-breaking as Phase 2 detail.

### RPS-5 — The exact HTTP response contract is under-specified

**Severity:** Medium  
**Source:** Product spec §6 Module B and §7; NFR4–NFR6  
**Requirements:** Set `Access-Control-Allow-Origin: *`, `Content-Disposition: attachment`, `Cache-Control: no-store`, and `Content-Length` for a single file while omitting it for a streamed directory.

AD-5 requires safe `Content-Disposition`, `no-store`, CORS, and `nosniff`, but does not pin the wildcard CORS value or the conditional `Content-Length` behavior. AD-7 distinguishes known and unknown totals, but that is an event/progress contract rather than an HTTP header rule.

The companion design document includes all required header behavior. It has not fully landed in the spine.

**Reconciliation needed:** State the wildcard and conditional length rule in AD-5. The additional `nosniff`, request limits, and safe Unicode filename handling are compatible hardening.

### RPS-6 — The QR library contradicts the source stack, but appears to be an intentional architecture supersession

**Severity:** Informational / documentation integrity  
**Source:** Product spec §2 versus spine AD-10 and Stack  
**Conflict:** The source specifies `github.com/skip2/go-qrcode`; the spine selects `github.com/boombuler/barcode` v1.1.0.

This is not a product-behavior regression, and the companion design document explicitly records that the maintained, tagged `boombuler/barcode` dependency supersedes the inactive, unversioned source guidance. The spine itself does not label the choice as a supersession, so a reader reconciling only its `sources` metadata and rules sees an unexplained contradiction.

**Reconciliation needed:** Mark this dependency choice as an explicit supersession in AD-10 or the Stack section. No return to `skip2/go-qrcode` is recommended based on the recorded architecture rationale.

## Corrected facts and product constraints that did land

| Source requirement or correction | Spine coverage | Result |
| --- | --- | --- |
| Product name FairDrop and mDNS type `_fairdrop._tcp` | AD-9, Stack, Structural Seed | Landed |
| Leading `context.Context` for server/stream work | AD-4, AD-6, Cancellation convention | Landed semantically; existing Phase 1 interfaces retain the exact signatures |
| Standard Wails v2 line rather than a framework migration | AD-10 | Landed |
| Exactly one staged file or directory; no silent truncation of multi-drop | AD-3 | Landed; explicit product clarification |
| `IDLE -> STAGED -> TRANSFERRING -> DONE/ERROR -> IDLE` with three-second reset | AD-2 | Landed; internal `STAGING` is compatible |
| Server bound to `0.0.0.0:0` and OS-assigned port | AD-5 | Landed |
| Reject concurrent valid receiver with `423 Locked` | AD-5 | Landed and strengthened by capability token |
| Cancel and force teardown without lifecycle leaks | AD-4, AD-6 | Landed |
| File streaming in bounded memory; ZIP via `io.Pipe` with close ordering | AD-6 | Landed and strengthened |
| No database, config, persistent logs, telemetry, cloud, or staged ZIP | AD-6, AD-9, Persistence convention | Landed |
| Wire-level progress at no more than 4 Hz | AD-7 | Landed; `totalKnown` resolves the directory/zero-byte ambiguity |
| Required transfer event family | AD-8 | Landed; session IDs and `transfer-reset` are compatible additions |
| Trusted-LAN/plain-HTTP scope and no sensitive mDNS metadata | AD-5, AD-9 | New clarification, compatible with product behavior |
| Windows and macOS native release verification | AD-10 | Landed; UPX correctly made opt-in |

## Deliberate changes that are compatible or beneficial

The following differences should not be treated as failures during implementation:

- A capability-token path strengthens the original random-port-only download endpoint without changing the LAN/browser flow.
- `STAGING` is an internal concurrency state; the externally visible state model remains intact.
- Session-scoped events and an explicit `transfer-reset` make the three-second terminal-state rule deterministic.
- Directory progress is indeterminate because ZIP wire length is not known in advance; using logical directory size as wire total would be dishonest.
- Symlink and special-file rejection, path normalization, mid-stream abort semantics, and request limits close product-spec safety gaps.
- Wails context no longer needs to flow into the HTTP server merely to emit UI events: the Observer adapter keeps Wails at the edge while request/stream cancellation still uses session contexts. This intentionally improves the spec's implementation guidance without weakening its cancellation or event requirements.
- React 19, Go 1.25 module floor, and the verified tool versions reflect the committed Phase 1 tree rather than the source spec's older baseline.

## Conclusion

No core architectural reversal is required. The ports-and-adapters coordinator is consistent with the product and is a stronger implementation substrate than putting lifecycle concerns in `app.go`. Before treating the spine as the durable binding handoff, reconcile RPS-1 through RPS-5 and annotate RPS-6 as an explicit dependency supersession. Until then, implementers must read the product spec corrections and `docs/fairdrop-architecture.md` alongside the spine to avoid losing required behavior.
