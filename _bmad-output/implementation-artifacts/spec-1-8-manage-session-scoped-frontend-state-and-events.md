---
title: 'Story 1.8 — Manage Session-Scoped Frontend State and Events'
type: 'feature'
created: '2026-08-28'
status: 'done'
review_loop_iteration: 0
baseline_commit: 'aa356ce3edc9628204413e0bba460d2c8979740f'
context:
  - '{project-root}/docs/fairdrop-contracts.md'
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
  - '{project-root}/_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-2026-08-23/EXPERIENCE.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** The backend emits an authoritative ordered lifecycle, but the frontend has no session state, event validation, or command-race handling. Stale promises and forged or malformed Wails events can corrupt the visible transfer.

**Approach:** Add a runtime-validated transfer module: a pure reducer and selectors enforce session grammar, while one React controller owns Stage generations and the five Wails subscriptions. Correlation hardens stale/blind injection; it does not authenticate events against code already running in the webview.

## Boundaries & Constraints

**Always:** Parse Wails results/events from `unknown` into fresh allow-listed records without throwing. Only current valid Stage metadata creates `(sessionId,lastSeq=0)`; malformed success triggers one best-effort Cancel. Accept events only for the active session, positive safe-integer `seq > lastSeq`, valid payload, and legal transition; rejection never consumes sequence. Use exact fixed error copy; terminal events allow only `path_not_found`, `path_unsupported`, `source_changed`, or `transfer_failed`. Terminal reset discards all session/capability data before retaining minimal Done/Error status. Use each `EventsOn` disposer.

**Ask First:** Any binding/Go change, runtime dependency, changed copy, visual work beyond controller integration, persistence/timer, or different reset/correlation rule.

**Never:** Never initialize from pending/events, infer kind from paths, advance invalid/foreign/stale input, retain session metadata after reset, use `EventsOff`, edit generated bindings, add Tailwind v3/PostCSS, or claim event authenticity.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Stage acknowledgement | Current pending generation + valid metadata | Staged with authoritative metadata and `lastSeq=0` | Rejection leaves no active session; `cancelled` is not Error |
| Obsolete/bad acknowledgement | Cancelled/unmounted generation or malformed success | No session install; bad success attempts Cancel once | Fixed fallback; no raw input retained |
| Valid lifecycle | Matching session, increasing seq, valid grammar | Staged → Transferring → Done/Error → retained Idle | Each accepted event advances `lastSeq` once |
| Hostile lifecycle | Foreign/stale/illegal/malformed/variadic event | Same state object and sequence | Ignore without throwing |
| Terminal/reset | Late progress/terminal; preterminal or terminal reset | Suppress late input; reset yields Idle or minimal retained outcome | Old events cannot correlate |
| Progress | Known-positive, known-empty, or unknown | Explicit finite mode without division | Ignore invalid/regressive data |
| Mount lifecycle | StrictMode/remount/unmount | One live listener/name; each disposed once | Stale callbacks cannot dispatch |

</frozen-after-approval>

## Code Map

- `_bmad-output/planning-artifacts/epics.md:641` — binding Story 1.8 acceptance.
- `docs/fairdrop-contracts.md:305` — read-only event grammar and payload table.
- `internal/transfer/types.go:34`, `ports.go:115` — read-only wire DTOs/events.
- `app.go:85`, `app.go:205` — forged-event limit and production emitter.
- `frontend/src/transfer/errors.ts:19` — reuse codes; replace arbitrary-message trust with fixed-copy validation.
- `frontend/src/App.tsx:10`, `App.test.tsx:134` — production mount, native drop, StrictMode evidence.
- `frontend/wailsjs/runtime/runtime.d.ts:40` — generated per-listener disposer contract.
- `frontend/{tsconfig.json,tsconfig.node.json,package.json,package-lock.json}` — Bundler resolution and Node 24/types pins.

## Tasks & Acceptance

**Execution:**
- [x] `frontend/src/transfer/{types,validation}.ts` — mirror DTOs and total validators for metadata, progress, warnings, and five event payloads.
- [x] `frontend/src/transfer/{state,selectors}.ts` — immutable state/transition table, retained-outcome scrub, actions, and progress modes.
- [x] `frontend/src/transfer/errors.ts` — share literal fixed-copy validation across command and event errors.
- [x] `frontend/src/transfer/useTransfer.ts` — own Stage/Cancel generations, subscriptions, stale-callback guard, and cleanup; expose Story 1.9's typed API.
- [x] `frontend/src/App.tsx` — mount the production controller and route exactly-one native drops through Stage while preserving drop gating; no Paper Relay views.
- [x] `frontend/src/transfer/*.test.ts*`, `App.test.tsx` — cover the matrix and mutation-pin production mount, literal listeners, cleanup, correlation, grammar, suppression, and scrubbing.
- [x] `frontend/{tsconfig.json,tsconfig.node.json,package.json,package-lock.json}` and `.nvmrc` — migrate both projects to Bundler, direct locked Node types, and Node 24 LTS; preserve Tailwind v4.

**Acceptance Criteria:**
- Given rejected input, when reduction finishes, then state identity, sequence, and data are unchanged.
- Given command/event race orderings, when all settle, then backend event order selects the outcome and obsolete promises cannot overwrite it.
- Given a known code carrying a path/token message, when parsed, then only fixed registry copy enters state.
- Given terminal reset, when retained state is serialized, then no session, capability, sequence, timer, or persisted data exists.
- Given StrictMode and two subscribers, when one unmounts, then its five disposers run once and the other stays live.

## Spec Change Log

## Design Notes

Native drop supplies no pre-acknowledgement kind. Pending permits `unknown` and never guesses; Story 1.9 must reconcile its file/folder copy.

A malformed error body becomes fixed `transfer_failed`; other payload defects reject the event without consuming sequence.

Stage metadata validation pins the semantics of the current Go producer, not merely its JSON shape: each identity is 16 random bytes rendered as exactly 32 lowercase hex characters; the direct link is the canonical explicit-port `http://<IPv4>:<port>/download/<token>` capability URL with no credentials, query, fragment, or normalization; and QR data must be bounded padded base64 containing a structurally complete PNG (`IHDR`, at least one `IDAT`, terminal `IEND`). React does not attempt to decode the QR payload or prove scannability — Story 1.5a owns the encoder and Story 3.3 owns native scan evidence — but it does refuse truncated/non-image acknowledgements that would install an unusable session.

For a staged regular file, every accepted progress or final snapshot must keep `totalKnown=true` and `totalBytes=metadata.size`; a staged directory must remain unknown-total. This correlation is reducer-owned because the event parser has no staged metadata. A rejected mismatch does not consume its sequence, so a corrected event with the same `seq` remains admissible.

An outstanding Stage is best-effort cancelled on controller unmount before its promise is made obsolete. This closes the otherwise-live backend session that can exist when Go commits STAGED just before JavaScript unmounts. Listener acquisition and release are likewise failure-isolated: partial registration unwinds every acquired disposer, and one throwing disposer cannot prevent later cleanup.

The cancel-winning Idle summary is intentionally not added to this story's state. Story 1.8's frozen retention rule covers Done/Error only; Story 1.10 explicitly owns the `copy.cancel.won` presentation and focus route and may add the minimal presentation marker when it implements that contract.

## Review Disposition

- **Accepted — boundary and copy integrity:** restored the four typographic-apostrophe messages to the binding/UX registry; made command and terminal-error parsing total for throwing getters; rejected malformed singleton/native-drop values; and strengthened Stage metadata from non-empty strings to canonical session, capability-URL, and bounded structural-PNG validation. These are in-scope patches to the approved rule that Wails input is `unknown` and only fixed allow-listed data enters state.
- **Accepted — lifecycle integrity:** known-total completion must be complete; file/directory progress must agree with staged metadata; invalid selection no longer erases retained Done/Error; unmount cancels an outstanding Stage once; an unresolved Cancel from an old reset session cannot block the current session; repeated Stage is suppressed; and partial/throwing listener cleanup cannot leak the rest. Each closes a concrete stuck, orphaned, contradictory, or stale-session outcome without changing the frozen correlation/reset rules.
- **Accepted — verification gaps:** the real controller now pins exact path forwarding, same-controller single-Stage ownership, state-aware selector behavior and error precedence, coherent selector fallback for runtime-invalid totals, semantic metadata rejection, and every new race/cleanup branch. The Code Map and Matrix Test Audit were refreshed after the final line layout.
- **Rejected — cancellation presentation in Story 1.8:** `copy.cancel.won` is a Story 1.10 focus/announcement obligation, while this story explicitly retains only terminal Done/Error. Adding a third retained outcome here would widen frozen intent and mix presentation ownership into the controller prematurely; the downstream story already names the exact missing state transition and copy.
- **Rejected — generation wrap:** reaching `Number.MAX_SAFE_INTEGER` requires more than nine quadrillion completed Stage operations in one mounted controller. Wrapping would create a stale-generation collision risk for no process-lifetime benefit, so the existing monotonic counter remains.
- **Rejected — mid-workflow status drift:** `status: in-review` with sprint `in-progress` is the deliberate Step-04 state, not product drift. Sprint status is synchronized only after the review verdict. No frontend QR-content decoder or CRC implementation was added: the frontend now rejects truncated/non-PNG structure, while the backend encoder and native scan gate remain the authoritative semantic evidence.

## Verification

**Commands:**
- `cd frontend && npm.cmd ci` — clean dev-dependency install.
- `wails build` — regenerate/compile bindings before standalone frontend build.
- `cd frontend && npm.cmd test && npm.cmd run build` — state/integration tests and TypeScript pass.
- `go test -count=1 ./... && go vet ./...` — backend regression passes.
- `CGO_ENABLED=1 go test -race -count=1 ./...` — native race check passes with documented PATH.
- `git diff --check` — clean diff; no Tailwind/PostCSS v3 files.

**Evidence (2026-08-29):** clean `npm ci` (165 packages, zero reported vulnerabilities); Wails v2.15.0 Windows/amd64 production build; 6 Vitest files and 161/161 tests; TypeScript/Vite production build; all 7 Go packages under deterministic tests, vet, and the CGO race detector; clean diff check; no `EventsOff`, frontend timer/persistence API, or Tailwind v3/PostCSS configuration. Sixteen targeted mutations were killed and reverted. The original seven covered duplicate-sequence acceptance, arbitrary fixed-copy drift, missing disposer cleanup, exact-adjacency sequence rejection, stale StrictMode dispatch, repeated pending Cancel, and premature cancel settlement. The review round additionally broke typographic copy, completion finality, metadata/progress correlation, exact path forwarding, old-session Cancel isolation, retained-outcome preservation, unmount cleanup, same-controller Stage ownership, and partial-listener unwind; each failed a test named for that guarantee before restoration.

## Matrix Test Audit

| Matrix row | Passing coverage |
|---|---|
| Stage acknowledgement | `state.test.ts:42`, `state.test.ts:69`, `useTransfer.test.tsx:198` |
| Obsolete/bad acknowledgement | `state.test.ts:57`, `useTransfer.test.tsx:224`, `useTransfer.test.tsx:252`, `useTransfer.test.tsx:286`, `useTransfer.test.tsx:323` |
| Valid lifecycle | `state.test.ts:96`, `state.test.ts:115`, `state.test.ts:148`, `state.test.ts:175` |
| Hostile lifecycle | `validation.test.ts:184`, `validation.test.ts:203`, `validation.test.ts:224`, `validation.test.ts:254`, `state.test.ts:157`, `useTransfer.test.tsx:375` |
| Terminal/reset | `state.test.ts:148`, `state.test.ts:238`, `state.test.ts:251`, `state.test.ts:270`, `useTransfer.test.tsx:353`, `useTransfer.test.tsx:387` |
| Progress | `validation.test.ts:134`, `validation.test.ts:240`, `selectors.test.ts:23`, `selectors.test.ts:55`, `selectors.test.ts:72`, `state.test.ts:184`, `state.test.ts:195`, `state.test.ts:226` |
| Mount lifecycle | `useTransfer.test.tsx:90`, `useTransfer.test.tsx:105`, `useTransfer.test.tsx:125`, `useTransfer.test.tsx:142`, `useTransfer.test.tsx:160`, `useTransfer.test.tsx:170`, `useTransfer.test.tsx:188`, `useTransfer.test.tsx:323`, `App.test.tsx:63`, `App.test.tsx:123`, `App.test.tsx:134` |

## Suggested Review Order

**Controller ownership and transitions**

- Start here: one controller owns command generations, subscriptions, cancellation, and cleanup.
  [`useTransfer.ts:29`](../../frontend/src/transfer/useTransfer.ts#L29)

- The pure reducer enforces correlation, grammar, scrubbing, and metadata-progress consistency.
  [`state.ts:88`](../../frontend/src/transfer/state.ts#L88)

- Native drop remains a thin, total adapter that forwards one path unchanged.
  [`App.tsx:10`](../../frontend/src/App.tsx#L10)

**Boundary validation and safe copy**

- Stage acknowledgements and lifecycle payloads become fresh canonical records or are refused.
  [`validation.ts:25`](../../frontend/src/transfer/validation.ts#L25)

- Command and event errors can enter state only through fixed registry copy.
  [`errors.ts:64`](../../frontend/src/transfer/errors.ts#L64)

**Downstream presentation contract**

- Selectors expose explicit progress modes and scrubbed state-owned values for Story 1.9.
  [`selectors.ts:30`](../../frontend/src/transfer/selectors.ts#L30)

- Mirrored wire and controller types keep optionality and outcome retention explicit.
  [`types.ts:1`](../../frontend/src/transfer/types.ts#L1)

**Adversarial and race evidence**

- Listener tests prove partial unwind, throwing-disposer isolation, StrictMode, and subscriber independence.
  [`useTransfer.test.tsx:90`](../../frontend/src/transfer/useTransfer.test.tsx#L90)

- Command tests pin exact path forwarding, single Stage ownership, and obsolete acknowledgements.
  [`useTransfer.test.tsx:198`](../../frontend/src/transfer/useTransfer.test.tsx#L198)

- Cleanup and two-session race tests prove no orphaned Stage or stale Cancel blockage.
  [`useTransfer.test.tsx:323`](../../frontend/src/transfer/useTransfer.test.tsx#L323)

- Reducer tests pin first-snapshot totals, sequence preservation, terminal scrubbing, and retention.
  [`state.test.ts:195`](../../frontend/src/transfer/state.test.ts#L195)

- Validator tests exercise canonical metadata, hostile getters, and completion finality.
  [`validation.test.ts:42`](../../frontend/src/transfer/validation.test.ts#L42)

- Literal error tests pin canonical copy values instead of comparing symbols to themselves.
  [`errors.test.ts:56`](../../frontend/src/transfer/errors.test.ts#L56)

- Drop-boundary tests cover malformed collections while retaining the inherited Wails gate.
  [`App.test.tsx:72`](../../frontend/src/App.test.tsx#L72)

- State-aware selector tests pin visible-error precedence and ownership boundaries.
  [`selectors.test.ts:113`](../../frontend/src/transfer/selectors.test.ts#L113)

**Toolchain and durable handoff**

- Node 24 and direct Node types make clean frontend installation reproducible.
  [`package.json:6`](../../frontend/package.json#L6)

- The lockfile records the exact clean-install development dependency graph.
  [`package-lock.json:1`](../../frontend/package-lock.json#L1)

- Both TypeScript projects move together to ES2023 Bundler resolution.
  [`tsconfig.json:3`](../../frontend/tsconfig.json#L3)

- The Node-side Vite project receives the matching resolution contract.
  [`tsconfig.node.json:4`](../../frontend/tsconfig.node.json#L4)

- The repository-level runtime pin makes the intended Node major unambiguous.
  [`.nvmrc:1`](../../.nvmrc#L1)

- Compiled epic context preserves Story 1.8's constraints for future agents.
  [`epic-1-context.md:18`](epic-1-context.md#L18)

- Sprint tracking records implementation as ready for independent review.
  [`sprint-status.yaml:46`](sprint-status.yaml#L46)
