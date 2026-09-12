---
title: 'Story 3.8: Harden the Directory Stream'
type: 'bugfix'
created: '2026-09-12'
status: 'in-progress'
baseline_commit: '6a366121fa9fbcd218935aa2119970e3b4e6913a'
review_loop_iteration: 0
context:
  - '{project-root}/AGENTS.md'
  - '{project-root}/_bmad-output/implementation-artifacts/epic-3-context.md'
  - '{project-root}/docs/fairdrop-contracts.md'
  - '{project-root}/docs/fairdrop-architecture.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Folder traversal and cleanup have unbounded resources, unsafe readers, and ambiguous identity/naming behavior.

**Approach:** Fail safely through preparation, streaming, and cleanup without poisoning retries. Close D-077, D-079, D-080, D-081, D-082, D-096, D-099, D-101, D-102, D-105 and the archive-drain retrospective action.

## Boundaries & Constraints

**Always:** Preserve Story 3.7 no-follow metadata, cancellation, HTTP finalization, and native proof. Keep bounded-memory, unsnapshotted contents and lazy directory preparation. Update binding documents with changed contracts.

**Ask First:** Dependencies, new public codes/copy, or policy changes beyond this matrix and the prepared-directory capability below.

**Never:** Rename nested entries silently, buffer whole trees, claim a timed-out resource stopped, or weaken tests. No UI, release-platform decisions, or manual certification.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|---|---|---|---|
| Healthy folder | Empty/nested/Unicode names | Valid ZIP; directories 0755, files 0644 | Existing coded errors |
| Excessive depth | Retained directory handles exceed 64 | Refuse before acquiring another; count lexical ancestors, enumeration frames, prepared root; at most three transient handles | `path_unsupported` |
| Root replacement | After Prepare, before WriteTo | Compare retained identity with freshly validated traversal root; never stream replacement bytes | `source_changed`; links remain `path_unsupported` |
| Borrowed reader misuse | Concurrent read/visitor return | No race; reads after revocation return `fs.ErrClosed`; owned close remains safe | Preserve cancellation/close precedence |
| Unsafe archive segment | Control/format characters, Windows device names, reserved punctuation, trailing dot/space, traversal syntax | Reject in inspection and ZIP defense; preserve ordinary spaces/Unicode | `path_unsupported` |
| Empty reads | Consecutive `(0,nil)` | Fail after 101 empty reads; progress resets count | `transfer_failed`, wraps `io.ErrNoProgress` |
| Source arithmetic/batch fault | Invalid size or oversized enumeration batch | Phase-correct failure | `setup_failed` during inspection; `transfer_failed` during streaming; preserve other codes |
| Wedged cleanup | Stop/Shutdown never returns; repeated retries | One outstanding call per adapter; no new session exposed to late cleanup; no accumulating workers | Existing coded failure/busy |
| Nested timeout | Server wait exceeds 10s | Coordinator allows 15s; lease exceeds cleanup budgets | Propagate inner timeout failure, not diagnostic-only success |
| Isolated server wait | Accept loop, handler, or connection alone stuck | Each named timeout independently proven | No false quiescence |

</frozen-after-approval>

## Code Map

- `internal/source/source.go`: `withSelection`, `walkDirectory`, `emitFile`, `borrowedContent`; reuse native handles and `verifyOpened`, not path-based stat checks. `source_test.go`/`walk_test.go` provide deterministic handle fixtures.
- `internal/transfer/ports.go`: extend SourcePort with a source-owned prepared-directory capability (walk/close); migrate implementations/fakes and `selection_source.go` delegation. `internal/source/handle*.go` retain platform safety.
- `internal/stream/payload.go`/`archive.go`: replace directory Lstat-only preparation and path-only archive ownership; reuse pipe/join/failure handling. Existing archive, concurrent, ZIP64 and stall tests remain.
- `internal/network/{network,beacon}.go`: StopBeacon currently holds both locks across Shutdown. Detach under lock, wait outside, retain outstanding-stop identity/result; GetLocalIP stays responsive and StartBeacon refuses overlapping responders.
- `internal/transfer/coordinator.go`: `callBounded`, stop wrappers and Stage admission need per-adapter in-flight ownership; preserve lease, diagnostics and late-result fencing. `internal/server/lifecycle.go` owns inner timeout; root-package tests can compare exported bounds without an import cycle.

## Tasks & Acceptance

**Execution:**
Checkpoints: complete the first two tasks with matching documentation, run the full gate, commit/push, and report before starting cleanup. Then complete remaining tasks and repeat verification. Keep one story; no findings waived.

- [ ] `internal/source`, `internal/transfer/ports.go`, `selection_source.go` — bounded traversal, prepared root ownership, synchronized reader revocation and source error correction; migrate all port consumers/fakes.
- [ ] `internal/stream`, shared archive-name predicate in `internal/transfer` — portable segment rejection at both boundaries, explicit modes and drain guard; test every matrix row.
- [ ] `internal/network`, `internal/transfer/coordinator.go`, `internal/server/lifecycle_test.go`, root integration tests — coalesced cleanup, retry admission, related timeout budgets and isolated timeout branches.
- [ ] Canonical SPEC, contracts, architecture/spine/memlog, epic context, `deferred-work.md`, `epics.md`, `sprint-status.yaml` — synchronize decisions and close all ten IDs plus retrospective action with evidence.

**Acceptance Criteria:**
- Given native adapters, when a folder is transferred, then the matrix holds and file/HTTP-finalization regressions stay green.
- Given blocked cleanup, when repeated claims/cancels/stages occur and the old call eventually returns, then worker count stays bounded, no newer session is stopped, and recovery is proven deterministically.
- Given each load-bearing guarantee, when deliberately broken, then a named behavioral assertion fails; compilation, skips and timeouts are not mutation proof.

## Evidence

[Implementation evidence](evidence-3-8-harden-the-directory-stream.md) is created in step 03.

## Spec Change Log

## Design Notes

A source-owned search handle pins the root from Prepare, not original Stage. Walk revalidates ancestors and compares against that pin before traversing the same validated handle. Close owns release even without streaming. Test concurrent Close/Walk and read-in-flight/visitor-return/closure, not merely flags.

Reject, never silently rename, unsafe segments: `<>:"/\\|?*`, case-insensitive Windows device stems including extensions/superscript digits. Keep apostrophes/semicolons: ZIP names are not HTTP parameters. Validate the sanitized root too. No cross-entry case/normalization collision guarantee or permission-preserving backup claim.

The handle budget limits our consumption, not ambient exhaustion. OS reads remain non-interruptible. Outstanding cleanup stays owned after timeout; test inner-timeout propagation independently of diagnostic-only cleanup errors.

## Verification

- Run AGENTS.md's complete sequential local gate, including native cgo/race, frontend, lint, bindings and line endings; normal Go timeout 240s, race 1200s.
- Run foreign-platform preflight, then Windows/macOS native CI and Linux adapter verification. Preserve complete failures; read actual job conclusions.
- Mutate depth, root identity/wiring, reader lifetime, name/mode, stall/error codes, cleanup admission/count, timeout relationship and each named wait; use existing assertion-associated mutation verdict tooling.
