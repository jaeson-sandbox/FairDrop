---
title: 'Story 1.7 — Expose Safe Transfer Commands through Wails'
type: 'feature'
created: '2026-08-28'
status: 'done'
review_loop_iteration: 0
baseline_commit: '30227e69bf5998271f4b903bf03e204d9854e12a'
context:
  - '{project-root}/docs/fairdrop-contracts.md'
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Six adapters and a complete coordinator exist and nothing composes them. `app.go` is still the Phase 1 stub — a context field and two empty hooks — so no desktop action reaches the transfer implementation, no lifecycle event reaches the UI, and `main.go` constructs no adapter at all.

**Approach:** Make `app.go` the Wails translation layer and `main.go` the composition root. Commands delegate to the coordinator and return the binding shapes unchanged; the App implements `Observer` and turns each lifecycle event into the matching `transfer-*` runtime emission; coded failures cross the boundary through `ErrorFormatter` as validated `{code,message}` JSON, and the frontend parses them with the same fixed fallback.

## Boundaries & Constraints

**Always:** Keep `main.go` the only place a concrete adapter is named, and `app.go` free of HTTP, filesystem, discovery, and transfer-state logic. Return the coordinator's shapes unchanged rather than remapping them. `Observer.Publish` runs while the coordinator holds its operation lease, so it must return promptly and must never block, panic, or call back into the coordinator — a panic there strands the lease and wedges every later command. Store the Wails runtime context only at startup and never replace it with a transfer-scoped one. Regenerate `frontend/wailsjs/**` through the Wails CLI.

**Ask First:** Any change to the contract's `FileMetadata`, `Event`, `PublicError`, or public command signatures. Any new dependency. Any Wails option beyond `ErrorFormatter` — the existing window, drop, and lifecycle options are proven and pinned. Bounding `Shutdown` with a timeout, which would let the process exit while a listener is live.

**Never:** No transfer-state machine, session bookkeeping, or event sequencing in `app.go` — the coordinator owns all of it. No automatic staging from a native dialog. No hand-editing generated bindings. Never let an absolute path, capability token, or raw adapter text reach a command error, an emitted event, or a log. No reducer, view, or styling work — Stories 1.8 to 1.10 own the frontend.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Composition | `main.go` starts | One coordinator wired to the six real adapters; `App` receives it | N/A |
| Stage succeeds | A valid absolute path | Delegates; returns `FileMetadata` unchanged, warnings a non-null array | N/A |
| Stage refused | Busy, invalid, setup failure, or cancellation | No metadata; the coordinator's code is preserved | applicable coded error |
| Cancel | Any state | Delegates to the quiescent Cancel; returns nil once IDLE | a cleanup diagnostic never becomes a command failure |
| Cancel while closing | After `Shutdown` | Refused with no state change | `shutting_down` |
| Dialog returns a path | `SelectFile` / `SelectDirectory` | Returns that path; nothing is staged | N/A |
| Dialog cancelled | The user dismisses it | Empty selection, no transfer error, no lifecycle event | N/A |
| Event published | Any coordinator `Event` | One `transfer-*` emission carrying `sessionId`, `seq`, and the event's own payload | internal `Kind` never serialized |
| Publish before startup | `a.ctx` still nil | Dropped without emitting; no panic reaches the coordinator | N/A |
| Publish panics | The runtime emit panics | Recovered inside the adapter; the operation lease is not stranded | recorded, not propagated |
| Coded command failure | A `DomainError` reaches `ErrorFormatter` | A JSON string of the validated `{code,message}` | N/A |
| Unknown command failure | An arbitrary error | The fixed `transfer_failed` code and copy | no adapter text, no path |
| Startup | Wails calls `OnStartup` | Stores the application-lifetime context and nothing else | N/A |
| Shutdown | Wails calls `OnShutdown` | Delegates to the idempotent `Shutdown`; returns only when quiescent | diagnostics stay internal |
| Options | `appOptions` | File drop, standard chrome, normal start state, both hooks, and `ErrorFormatter` all present | N/A |
| Frontend parse | A rejection carrying the JSON string | `{code,message}` validated and surfaced | malformed or unknown falls back to `transfer_failed` |
| Disclosure | Every command error and emitted event | No absolute path, capability token, or adapter text | N/A |

</frozen-after-approval>

## Code Map

- `docs/fairdrop-contracts.md:247-258` — **read-only, binding.** The public Wails API, the `ErrorFormatter` rule, and `parseCommandError`'s fallback.
- `docs/fairdrop-contracts.md:305-330` — **read-only, binding.** Event grammar and the per-event payload table the emission must satisfy.
- `app.go:1-25` — the Phase 1 stub this story replaces. `startup` already stores `a.ctx`; keep that and add nothing else to it.
- `main.go:22-54` — `appOptions` is the seam that exists so options are testable; add `ErrorFormatter` here, change nothing else.
- `main_test.go:14-48` — the proven option assertions. Extend, never weaken: `EnableFileDrop` false still passes every other check in the repo.
- `internal/transfer/ports.go:113-140` — `EventKind`, `Event`, and `Observer`. `Kind` is `json:"-"`, which is what keeps it off the wire.
- `internal/transfer/errors.go:117-146` — `PublicErrorOf` already maps coded and unknown errors; the formatter must call it rather than re-deriving.
- `internal/transfer/lifecycle.go:23`, `:61` — `Cancel` and `Shutdown`. Both run to quiescence and take no context on purpose.
- `internal/transfer/coordinator.go:167-184` — `Dependencies`. `Entropy`, `Now`, and `AfterFunc` stay defaulted in production.
- `internal/source/source.go:30`, `internal/network/network.go:57`, `internal/qr/qr.go:41`, `internal/stream/archiver.go:86`, `internal/server/lifecycle.go:70` — the five constructors `main.go` composes. `server.New` takes the payload port that `stream.New` provides.
- `frontend/src/App.tsx:1-16` — the drop integration to preserve verbatim; Story 1.9 rewrites this view.
- `frontend/src/App.test.tsx:1-20` — the mocking pattern frontend contract tests follow.
- Wails v2.15.0: `options.ErrorFormatter` is `func(error) any`; `runtime.OpenFileDialog` and `OpenDirectoryDialog` return `(string, error)`; `runtime.EventsEmit(ctx, name, ...any)`.

## Tasks & Acceptance

**Execution:**
- [x] `app.go` — the four commands, the `Observer` implementation, and the two lifecycle hooks; nothing else.
- [x] `main.go` — compose the six adapters and the coordinator, resolve the App/coordinator cycle in one place, and register `ErrorFormatter`.
- [x] `app_test.go` — **new.** Drive every matrix row against a fake coordinator boundary and fake Wails seams.
- [x] `main_test.go` — pin `ErrorFormatter` beside the existing option assertions.
- [x] `frontend/src/transfer/errors.ts` and its test — **new.** `parseCommandError` with the fixed fallback.
- [x] `frontend/wailsjs/**` — regenerate through the Wails CLI; do not hand-edit.

**Acceptance Criteria:**
- Given the composed application, when `app.go` is searched for `net/http`, `os`, `mdns`, and transfer-state vocabulary, then none appears — translation only.
- Given each command failure the coordinator can produce, when it crosses `ErrorFormatter` and `parseCommandError` in turn, then the frontend receives the same stable code the coordinator chose, and an unrecognized error becomes `transfer_failed` on both sides.
- Given an `Observer.Publish` whose emission panics or arrives before startup, when the coordinator publishes under its operation lease, then the command that owns that lease still returns.
- Given every emitted event, when its JSON is inspected, then it carries `sessionId` and `seq`, satisfies the contract's payload table, and contains no `Kind` field.
- Given the finished boundary, when Go tests, frontend tests, and `wails build` run, then all pass and the Phase 1 drop, chrome, and lifecycle options are still asserted.

## Spec Change Log

- **Review round 1 (2026-08-28, patches only — no loopback):** All three layers ran and converged on the same top finding, which was in code added during pre-review verification rather than by the implementer. **Two severe verification gaps, both now killed by mutation.** `appObserver.Publish` — the sole link between the coordinator's `Observer` port and the Wails emitter — was never executed by a test: every publish test drove `App.publish` directly, so emptying the adapter left the entire suite green while no lifecycle event reached the window at all. And `main()` could lose its `compose(app)` call with `go vet` and the whole suite still passing, shipping a binary whose every command answers "FairDrop is not ready"; composition is now behind a `newBoundApp` seam for the same reason `appOptions` is one. **`Bind` was unpinned** — emptying it ships zero callable commands past build, vet, test and `wails build`, which is exactly the argument `main_test.go` already made for `EnableFileDrop`, and it became load-bearing only when this story gave the App its first exported method. **Four more production fixes:** every emission is now asserted to carry the stored application-lifetime context, because the real `EventsEmit` answers a foreign one with `log.Fatalf` and takes the process down rather than returning; an event kind this build cannot name is refused and counted instead of emitted under a name no listener subscribes to; `formatCommandError(nil)` returns the fixed fallback rather than an empty code; and a whitespace-only message falls back rather than rendering a code beside a blank line. **The error-code registry is now pinned as a set**, so a thirteenth code fails in `internal/transfer` with a message naming all four places it has to be added — previously both the Go and TypeScript lists were hand-written twelves and an *addition* failed nothing on either side, arriving at the UI as unrecognized. **One claim I made was corrected rather than defended:** the commit removing the bound `Publish` said it stopped the webview forging lifecycle events. It does not. Wails' frontend `EventsEmit` calls `notifyListeners(payload)` before it forwards anything to Go, so any script in the window can already deliver a `transfer-complete` to every `EventsOn` subscriber without this process being involved. Removing the binding narrows the *command surface* to the contract's four; the defence against forged events is the frontend rule the contract already fixes, and Story 1.8's reducer owns it. The comment and the commit message now say that. **KEEP:** the disclosure test now drives `StageTransfer` and asserts the token *is* present in the metadata while the path is absent — its predecessor built payloads out of two integers and a fixed string, so nothing it searched could ever have contained either.

## Design Notes

**The construction cycle.** The App is the coordinator's `Observer`, and the coordinator is the App's delegate. `main.go` breaks that in the obvious place: build the App, build the coordinator with the App as its observer, then hand the coordinator back to the App once. That single setter is why the cycle does not need a channel, a registry, or a lazily-initialized global.

**Why `Publish` is defensive.** Story 1.6 made publication lease-owned, which is what orders events causally — and means a panicking or blocking observer strands the lease and wedges every later `Cancel` and `Shutdown`. The adapter therefore owns that risk rather than exporting it: guard the nil context, recover around the emit, and return.

**`Shutdown` is allowed to take its time.** It returns only when the listener, beacon, drainer, and session context are gone, so the Wails hook blocks. That is the intended trade: a process that exits while its listener is live is the failure this whole epic has been avoiding.

## Verification

**Commands:**
- `go build ./... && go vet ./... && go test -count=1 ./...` — module and deterministic tests pass.
- `CGO_ENABLED=1 go test -race -count=1 ./...` — no race across the new boundary. Needs MinGW `bin` on PATH (see AGENTS.md).
- `cd frontend && npm test && npm run build` — frontend contract tests and the production bundle.
- `wails build` — the generated bindings compile against the real App.
- `rg -n 'net/http|os\.|mdns|stateStaged|sessionState' app.go` — no output: translation only.
- `rg -n 'wails' internal/ --glob '!**/*_test.go'` — no output: the core stays framework-independent.
- `go mod tidy && go mod verify`, `gofmt -l .`, `git diff --check` — no drift, formatting, or whitespace defects.

## Suggested Review Order

**The composition root**

- Start here: the only place a concrete adapter is named, and the cycle it closes.
  [`main.go:40`](../../main.go#L40)

- Composition behind a seam, because main itself cannot be called from a test.
  [`main.go:142`](../../main.go#L142)

- The App's whole view of the transfer implementation; app.go names nothing concrete.
  [`app.go:20`](../../app.go#L20)

**Why Publish is not on the App**

- An exported Publish would be bound as a fifth command; this narrows the surface.
  [`app.go:95`](../../app.go#L95)

- The riskiest code in the file: it runs while the coordinator holds its lease.
  [`app.go:216`](../../app.go#L216)

- A closed set, so a sixth kind has to be added deliberately rather than lost.
  [`app.go:30`](../../app.go#L30)

**The error path, both halves**

- PublicErrorOf decides the code; this only serializes it, and never adapter text.
  [`main.go:70`](../../main.go#L70)

- Registered here, or every rejection carries raw text with no stable code.
  [`main.go:125`](../../main.go#L125)

- The consumer, with the same fixed fallback the backend applies.
  [`errors.ts:64`](../../frontend/src/transfer/errors.ts#L64)

**Commands and lifecycle**

- Metadata crosses unchanged; remapping it would be a second definition that drifts.
  [`app.go:133`](../../app.go#L133)

- Cancel takes no context because it runs to quiescence.
  [`app.go:155`](../../app.go#L155)

- A dialog returns a path and stages nothing; a dismissal is not an error.
  [`app.go:169`](../../app.go#L169)

- The hook blocks until the listener, beacon and drainer are gone. That is the trade.
  [`app.go:265`](../../app.go#L265)

- The one context stored, and the one never stored over it.
  [`app.go:251`](../../app.go#L251)

**Evidence the boundary is real**

- The port the coordinator actually calls; emptying it once left the suite green.
  [`app_test.go:947`](../../app_test.go#L947)

- Composition asserted on an App built the way main builds it.
  [`main_test.go:174`](../../main_test.go#L174)

- Bind pinned: emptying it ships zero callable commands past every other check.
  [`main_test.go:159`](../../main_test.go#L159)

- The exported method set is the command surface, so it is pinned as a set.
  [`app_test.go:302`](../../app_test.go#L302)

- A foreign context is log.Fatalf in production, so every emission is checked.
  [`app_test.go:984`](../../app_test.go#L984)

- The token is asserted present and the path absent, through a real command.
  [`app_test.go:1060`](../../app_test.go#L1060)

- The two goroutines a.mu exists for, finally driven at once.
  [`app_test.go:1028`](../../app_test.go#L1028)

- The cross-language pin, reading inside the exported array rather than the file.
  [`main_test.go:105`](../../main_test.go#L105)

- A thirteenth code now fails here, naming every place it has to be added.
  [`errors_test.go:217`](../../internal/transfer/errors_test.go#L217)
