# Epics Reconciliation Review

**Reviewed:** 2026-08-22  
**Inputs:** `_bmad-output/planning-artifacts/epics.md` and `ARCHITECTURE-SPINE.md`  
**Verdict:** **Changes required before either artifact is used as the implementation plan.** The requirements inventory is a strong extraction of the product spec and deferred findings, and the spine gives most of its unresolved cross-cutting questions sound dispositions. However, `epics.md` still describes a pre-architecture state, contains unresolved items that the spine has now decided, and has no coverage map or epic list. The spine also claims complete FR/NFR coverage while omitting two requirements that remain material: stopping mDNS at the first claim and the accessibility path into staging.

## Scope and standard

This review treats the requirements inventory in `epics.md` as the planning baseline and asks two questions:

1. Did every requirement or validated deferred finding receive a compatible architectural disposition?
2. What architecture-derived obligations must be added when the placeholder coverage map and epic list are completed?

The review does not require the terse spine to repeat implementation-detail requirements that are already unambiguous in a binding source. It does flag a requirement when independently built units could follow the spine and still make incompatible choices.

## Findings

### RE-1 — Critical: `epics.md` has not been resumed after architecture and is not an executable breakdown

**Evidence**

- Frontmatter says `stepsCompleted: [1]`.
- The introduction says no separate Architecture document exists and that `docs/fairdrop-spec.md` is the sole architecture authority.
- `inputDocuments` omits both `ARCHITECTURE-SPINE.md` and its design companion.
- `{{requirements_coverage_map}}` and `{{epics_list}}` are still literal placeholders.

**Impact**

There is no trace from any FR, NFR, UX requirement, or architecture decision to an implementable epic/story. An agent starting Phase 2 from this artifact would miss the coordinator, contract migration, capability-token protocol, session-scoped events, and release-verification obligations.

**Required reconciliation**

- Add the finalized spine and `docs/fairdrop-architecture.md` as architecture inputs and revise the stale input note.
- Preserve all requirement IDs, then add architecture-requirement IDs for the obligations listed under **Architecture requirements to add** below.
- Replace both placeholders with a complete coverage map and epic/story list; every FR1-FR18, NFR1-NFR11, UX-DR1-UX-DR5, and architecture requirement must map to at least one story and acceptance criterion.

### RE-2 — Critical: the Phase 1 interface inventory conflicts with the spine's contract ownership and must become an explicit migration story

**Evidence**

- `epics.md` says the three `internal/` contracts exist and records their Phase 1 signatures as implementation inputs.
- The working tree places `NetworkManager`, `Streamer`, and `TransferServer` in their provider packages.
- AD-1 instead requires consumer-owned lifecycle ports in `internal/transfer`, a server-consumed streaming port, and concrete adapters behind those ports.
- The current `TransferServer.Start(ctx, filePath, onProgress)` and `TransferStats` cannot carry the session identity, capability claim, complete/error lifecycle, or `totalKnown` semantics required by AD-2, AD-5, AD-7, and AD-8.

**Impact**

If the later epic stories simply say “implement the existing interfaces,” they will build the wrong seam and immediately require a second redesign. Calling AD-1 `[ADOPTED]` also overstates the brownfield reality: the ports-and-adapters paradigm is newly selected, while the compile-only Phase 1 contracts use provider-owned interfaces.

**Required reconciliation**

- Add a story before the first affected implementation that treats the Phase 1 interfaces as transitional scaffolding and migrates them to the architecture's consumer-owned port model.
- Pin who generates the independent session ID and download token, how the server reports claim/completion/failure, and how wire-level progress includes `sessionId` and `totalKnown` without importing Wails into core/server packages.
- Do not keep duplicate public interfaces solely to preserve the Phase 1 file locations; there are no implementations yet, so this is the lowest-cost migration point.
- In the spine, remove or qualify the claim that AD-1's ownership rule is already adopted from Phase 1.

### RE-3 — High: the epic requirements inventory must convert resolved deferred questions into binding acceptance criteria

The spine makes deliberate decisions for several entries that `epics.md` still labels `[deferred]` or presents as open Additional Requirements:

| Existing epic entry | Spine disposition | Required planning change |
| --- | --- | --- |
| FR18 multi-file behavior | AD-3: v1 accepts exactly one regular file or one directory; zero/multiple paths are typed errors; multi-item staging remains deferred with a revisit contract | Remove the unresolved wording. Add acceptance cases for zero, one file, one directory, and multiple paths; never silently pick the first path. Keep only future multi-item support deferred. |
| Unknown directory percentage | AD-7: add `totalKnown`; directories use `false/0/0`, known empty files use `true/0/0`, UI renders indeterminately | Promote to Phase 4/server and Phase 6/UI acceptance criteria. |
| Failure after headers | AD-6: report ERROR and abort the connection via `http.ErrAbortHandler` | Promote to streaming/server failure tests; receiver must observe an interrupted download rather than a normal truncated response. |
| `TransferServer.Stop` idempotency | AD-4: safe before Start and after repeated calls; one teardown path | Promote to lifecycle interface contract and unit tests. |
| Progress versus zero-copy | AD-7: honest wire-level progress wins; count successful response writes at at most 4 Hz plus terminal snapshot | Record as decided, not as a Phase 4 design question. |
| No CI | AD-10: race-enabled Go tests, frontend test/build, Wails build, native Windows/macOS release runners | Add a release/verification story; do not describe CI as merely deferred. |

This reclassification matters because a coverage generator typically treats `[deferred]` items as non-blocking. These six decisions are now part of the build contract.

### RE-4 — High: capability security and session isolation are new architecture requirements absent from the inventory

**Evidence**

The product-derived inventory requires random-port HTTP and a 423 response during an active transfer, but the spine additionally requires:

- an independent capability token with at least 128 random bits;
- an exact `/download/:token` GET that atomically claims the session;
- 404 for invalid routes and 423 only for later valid-token requests;
- session IDs on callbacks, timers, and all UI event payloads, with stale callbacks/events ignored;
- a fresh session-owned cancellation context and one idempotent teardown;
- safe filename handling, `nosniff`, bounded header/read-idle limits, and no whole-transfer write deadline;
- no token, filename, or absolute path in mDNS TXT data.

**Impact**

These are cross-package protocol invariants, not optional implementation details. Without explicit architecture requirements in the epics artifact, individual network, server, coordinator, and frontend stories can each pass their product FRs while remaining mutually incompatible or weakening the intended security envelope.

**Required reconciliation**

Add architecture-derived requirements and trace them across the coordinator, server, network, Wails adapter, and frontend stories. Acceptance coverage must include token entropy/encoding, invalid-token behavior, concurrent valid claim behavior, stale-session callbacks/events, timeout configuration, safe content disposition, and mDNS privacy.

### RE-5 — High: FR11 is bound by the spine but not actually enforced by a rule

**Evidence**

- FR11 requires stopping the mDNS beacon on entry to `TRANSFERRING` so a second receiver cannot discover the offer.
- AD-4 says the session owns the beacon and defines terminal/cancel/shutdown teardown.
- AD-5 defines atomic HTTP claim and 423 behavior.
- Neither rule says the first valid claim stops the beacon before payload delivery begins.

**Impact**

An implementation can comply with AD-4 and AD-5 yet leave the offer advertised for the entire transfer. The HTTP token still prevents a second successful claim, but discovery behavior would violate FR11 and disclose a stale offer.

**Required reconciliation**

Make “first valid claim atomically transitions to `TRANSFERRING` and stops mDNS before streaming” a binding coordinator effect. In the epic coverage map, trace FR11 to both the coordinator/server claim story and the network beacon lifecycle tests.

### RE-6 — High: the spine's declared binding scope excludes the UX requirements, and UX-DR5 has no disposition

**Evidence**

- `epics.md` contains UX-DR1 through UX-DR5. UX-DR5 requires a keyboard-reachable Browse fallback and an `aria-live` announcement.
- Spine frontmatter binds only `FR1-FR18` and `NFR1-NFR11`.
- The structural seed names the three views and AD-8 governs UI event authority, but no rule or deferral addresses keyboard staging, semantic controls, focus, or live announcements.

**Impact**

The UI epic could be declared fully architecture-compliant while shipping drag-and-drop as the only input path. This is not a new product expansion; it is an accessible route into the existing Stage action.

**Required reconciliation**

- Preserve UX-DR1-UX-DR5 as first-class coverage rows even if UX presentation details live outside the architecture spine.
- Add an accessibility invariant or explicitly bind to an authoritative UX companion. It should require a keyboard/pointer-free staging action and state announcements; exact visual treatment can remain a UI-story decision.
- The Browse implementation must still produce the same exactly-one-path coordinator request and typed errors as native drop.

### RE-7 — Medium: NFR11 remains only partially decided and needs an explicit behavior/test matrix

**Evidence**

- NFR11 calls out spaces, non-ASCII, paths beyond 260 characters, UNC shares, and symlinks.
- AD-3 covers zero/multiple paths and AD-6 rejects symlinks/non-regular entries and prevents ZIP traversal.
- The spine does not state whether spaces, Unicode, long Windows paths, or UNC paths are supported or rejected with stable typed errors.

**Impact**

The capability map's package assignment does not prevent Phase 3 and Phase 5 stories from choosing incompatible path behavior. Deferring all cases would also contradict the spine's claim that it binds NFR11.

**Required reconciliation**

Before generating story acceptance criteria, decide per case: supported end to end, or deliberately rejected with a typed safe error. Add platform-focused tests for metadata inspection and revalidation during streaming. Keep symlink rejection and relative ZIP-name normalization as already decided by AD-6.

### RE-8 — Medium: dependency and toolchain requirements are stale or contradictory

**Evidence**

- `epics.md` says to add `github.com/skip2/go-qrcode`; AD-10 and the Stack select `github.com/boombuler/barcode` v1.1.0.
- `epics.md` summarizes React as 19.1 without exact current frontend versions; the spine records the verified current tree as React 19.2.8, TypeScript 5.9.3, Vite 7.3.6, Tailwind 4.3.3, Framer Motion 13.1.1, and Vitest 4.1.11.
- The deferred inventory still contains the unresolved TypeScript `moduleResolution: "Node"` and `npm ci --omit=dev` problems; the spine's generic dependency-locking rule does not select a concrete disposition for either.

**Impact**

An epic generated from the current Additional Requirements could reintroduce the superseded QR library or leave reproducible frontend builds underspecified.

**Required reconciliation**

- Replace the QR dependency entry with the architecture selection and label it an explicit supersession of the source spec.
- Derive stack versions from the lockfile/spine at planning time rather than the old Phase 1 prose.
- Add a frontend-foundation maintenance story or acceptance criteria selecting `moduleResolution: "Bundler"` and a supported reproducible install/build contract that includes dependencies needed by `tsc`, tests, Vite, and Wails.

## Architecture requirements to add to `epics.md`

The exact IDs may follow the epics workflow's convention, but these obligations need stable traceable entries rather than being buried in story prose:

| Proposed area | Binding obligation | Primary architecture source |
| --- | --- | --- |
| AR-DEP | `main.go` composes concrete adapters; Wails remains an outer adapter; coordinator and consumer-owned ports remain framework-independent | AD-1 |
| AR-STATE | Only the coordinator mutates lifecycle state; it never calls ports while locked; session-tagged callbacks/timers from old sessions are ignored | AD-2 |
| AR-SESSION | Exactly one root per session; fresh independent session identity; zero/multiple selections are typed errors; single-instance behavior is implemented and option-tested | AD-3 |
| AR-TEARDOWN | Per-session child context and owned resources; reverse setup unwind; one idempotent teardown; Stop safe before/repeated Start | AD-4 |
| AR-HTTP | At least 128-bit capability token, exact single-use GET claim, 404/423 semantics, safe headers, resource limits, and no whole-transfer deadline | AD-5 |
| AR-STREAM | Context-aware bounded copying; `io.Pipe` close ordering; path normalization/revalidation; symlink/special-file rejection; abort on post-header failure | AD-6 |
| AR-PROGRESS | Count successful wire writes, at most 4 Hz plus terminal update; `totalKnown`; indeterminate directory UI; rolling wire-byte speed | AD-7 |
| AR-EVENTS | Typed observer notifications; Wails alone maps to `transfer-*`; every payload has `sessionId`; React reducer ignores mismatches; explicit reset event | AD-8 |
| AR-PRIVACY | Trusted-LAN/plain-HTTP contract; non-sensitive mDNS metadata; no runtime persistent state, telemetry, or cloud | AD-9 |
| AR-VERIFY | Locked dependencies, race-enabled backend tests, frontend tests/build, Wails build, native Windows/macOS runners, UPX opt-in | AD-10 |

## Coverage-map guidance

When the placeholder map is generated, the following cross-epic traces are necessary:

- FR1/FR18/UX-DR1/UX-DR5 must cover the native `[]string` drop boundary, Browse fallback, exactly-one validation, and coordinator Stage command. The Phase 1 path-echo behavior is proof scaffolding, not the final multi-item product behavior.
- FR2/FR5/FR11 span network discovery and coordinator lifecycle; the stop-on-claim effect cannot be owned by the network adapter alone.
- FR3/FR8-FR14 span coordinator and server. The first valid HTTP request claims through the coordinator; the server must not own global transfer state.
- FR6-FR7/NFR1/NFR8-NFR9/NFR11 span server and stream adapters, including cancellation and post-header failure tests.
- FR9/FR17/NFR7 span response-layer byte accounting, coordinator observer notifications, and indeterminate/known-total frontend rendering.
- FR14-FR17 span coordinator terminal/reset timers, Wails event mapping, and the reducer's stale-session filtering.
- NFR10 and AD-10 require their own CI/release coverage rather than being left as a final manual command list.

## Items already aligned

- Ports and adapters with one lifecycle coordinator is compatible with the product's strict state machine and improves on concentrating orchestration in `app.go`.
- The spine preserves bounded-memory streaming, no staged ZIP, no runtime persistence, cancellation, 4 Hz progress, three-second terminal display, random-port LAN binding, file-versus-directory response length behavior at the design level, and Windows/macOS release intent.
- AD-3, AD-4, AD-6, and AD-7 provide sound dispositions for the most dangerous deferred concurrency/streaming ambiguities.
- Capability-token URLs, session-scoped events, trusted-LAN documentation, and explicit privacy constraints are compatible hardening rather than product-scope reversals.

## Gate recommendation

Do not proceed directly from the current `epics.md` to Phase 2 implementation. First finalize the spine after addressing RE-2, RE-5, RE-6, and RE-7; then resume the BMAD epics workflow using the finalized architecture as an input. Completion requires a real requirements coverage map and epic/story list that incorporates the architecture requirements above and reclassifies all resolved deferred items.
