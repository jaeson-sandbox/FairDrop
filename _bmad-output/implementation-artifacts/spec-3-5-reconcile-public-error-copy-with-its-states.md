---
title: 'Story 3.5: Reconcile Public Error Copy with the States It Describes'
type: 'feature'
created: '2026-09-11'
status: 'in-progress'
review_loop_iteration: 0
baseline_commit: '96a5a3c669f4a4d02cab1586542ffb261d1a2bbd'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-3-context.md'
  - '{project-root}/_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-2026-08-23/EXPERIENCE.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Five separate states tell the user "The transfer stopped before FairDrop finished sending" when no transfer ever began — a pre-startup refusal, an entropy failure while staging, a deadline during Prepare, a malformed stage acknowledgement, and any uncoded source error. A sixth tells them to "Finish or cancel the current transfer" while the coordinator is holding a *finished* transfer's outcome on screen. Each was recorded separately as a copy problem someone else would decide (D-012, D-015, D-025, D-029, D-044, D-047, D-053); together they are one missing code and one untrue sentence. Nothing pins the messages across the four files that restate them, so they can drift silently — Epic 1 retrospective item 6.

**Approach:** Audit every producer of a coded failure by the phase it runs in, add one code for failures that happen before any byte is sent, make `busy` true in both states it describes, and extend the cross-language pin from codes to the exact messages so the registry, the contract, the Go table and the TypeScript mirror cannot disagree.

## Boundaries & Constraints

**Always:** The audit is written down and groups each producer by whether a transfer had begun when it fires. Any message a user can reach describes the state it appears in and offers a recovery that applies there. New codes enter `EXPERIENCE.md`'s registry by stable key before any code emits them, and the registry, `docs/fairdrop-contracts.md`, the Go table and the TypeScript mirror change in the same commit. The cross-language pin covers every code *and* its exact message, and fails when any of the four drifts. `cancelled` still never renders as Error.

**Ask First:** The exact wording of any new or revised message — this spec proposes strings, and the reviewer owns them. Removing or renaming an existing code. Any change to what `cancelled`, `beacon_warning`, or `shutting_down` mean.

**Never:** Widen a message into vagueness to make it true everywhere; a sentence that fits every state describes none. Leave a state mapped to a code whose copy contradicts it. Change which surface owns an announcement — that is the accessibility contract, not this story. Touch lost or malformed *events* (Story 3.6) or the HTTP header matrix (Story 3.10).

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Entropy fails while staging | CSPRNG error in `newIdentity` | The new pre-transfer code; copy says nothing was sent | Coded (D-025) |
| Deadline during Prepare | context expires before headers | Same code; not described as an interrupted transfer | Coded (D-012) |
| Malformed stage acknowledgement | frontend cannot parse the reply | Same code; the user learns the selection was refused, not lost | Coded (D-053) |
| Uncoded source error | `SourcePort` returns a bare error | Wrapped at the port call, so the boundary enforces the postcondition rather than trusting it | Coded (D-015) |
| Pre-startup refusal | `errNotComposed`, dialog before startup | Same code; unreachable in a composed binary and still honest if reached | Coded (D-047) |
| Stage during the terminal lease | an outcome is on screen | `busy`, with copy true of a finished transfer as well as a running one | Coded (D-044) |
| Missing port at claim | `ready()` finds a nil port | Cannot happen: construction refuses it. Any residual path reports the pre-transfer code, not `transfer_failed` | Coded (D-029) |
| A message edited in one file | Go table only, or the registry only | The cross-language pin fails naming both sides | Test |
| A real transfer fails mid-stream | bytes were sent, then the stream broke | Still `transfer_failed`, unchanged | Coded |

</frozen-after-approval>

## Code Map

- `internal/transfer/errors.go:117` `publicMessages` — the Go table, twelve entries. `:12-23` the `ErrorCode` constants. `PublicErrorOf` maps an unknown code to `ErrTransferFailed`, which is how four of the five states reach the wrong sentence.
- `frontend/src/transfer/errors.ts:15` `transferErrorCodes` and `:34` `fixedErrorMessages` — the TypeScript mirror; both must gain the new code.
- `_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-2026-08-23/EXPERIENCE.md:116` — the registry table: code, visible heading, exact message, sole announcement owner, recovery. A new code needs all five columns. This table is the source the other three follow.
- `docs/fairdrop-contracts.md:46` — the binding code list. It carries codes only, no messages, so the pin covers codes here and messages in the other three.
- `main_test.go:201` `TestEveryStableCodeIsRecognizedByTheFrontend` — the existing pin, and the thing to extend. It checks that each code appears inside the exported array and survives `formatCommandError`; it never compares a message, which is exactly how the four copies can drift. Its "spelled out, not ranged over" comment explains why the list is literal — keep that property for messages too.
- `internal/transfer/errors_test.go:28` — the Go-side message table test, already literal; the new pin must not simply duplicate it, it must compare *across* files.
- The audit surface: 74 production sites emit `ErrTransferFailed`. Most are honest — a stream that broke, a server that failed. The question that sorts them is whether a transfer had begun, so group by phase rather than by file. The named states are `coordinator.go`'s `newIdentity` (D-025) and `ready()` (D-029), `internal/stream`'s Prepare path (D-012), `app.go`'s `errNotComposed` and the pre-startup dialog refusal (D-047), `frontend/src/transfer/useTransfer.ts`'s stage fallback (D-053), and `internal/stream`'s verbatim `SourcePort` passthrough (D-015).
- `internal/transfer/coordinator.go` `Stage`'s busy refusal and `TestStageIsRefusedDuringTheTerminalLease` in `coordinator_stage_test.go:969` — the refusal is already pinned; only the copy is wrong (D-044).

## Tasks & Acceptance

**Execution:**
- [ ] `evidence-3-5-reconcile-public-error-copy-with-its-states.md` — the audit first, grouped by phase, naming each of the seven states and whether its current message is true of it.
- [ ] `EXPERIENCE.md` — the new code's registry row and the revised `busy` row, both with all five columns.
- [ ] `docs/fairdrop-contracts.md` — the new code in the binding list.
- [ ] `internal/transfer/errors.go` — the constant and the table entry; `PublicErrorOf`'s fallback unchanged.
- [ ] `frontend/src/transfer/errors.ts` — the code and the message in both exports.
- [ ] The producers — each named state emits the new code instead of borrowing one that misdescribes it; `SourcePort` errors are wrapped at the port call; construction refuses a nil port so `ready()` cannot report one.
- [ ] `main_test.go` — the pin extended to compare every message across the registry, the Go table and the mirror, and every code across all four.
- [ ] `deferred-work.md` — the seven ids closed or re-owned with `epics.md` updated to match.

**Acceptance Criteria:**
- Given each of the seven named states, when it is reached, then the message shown describes that state and no state is called an interrupted transfer unless a transfer began.
- Given a message changed in exactly one of the registry, the Go table, or the mirror, when the suite runs, then the pin fails naming the files that disagree.
- Given a new code added to the Go table alone, when the suite runs, then the pin fails.
- Given a transfer that genuinely broke mid-stream, when it fails, then it still reports `transfer_failed` with its existing copy.

## Evidence

The audit, mutation tables and gate transcripts live in
[evidence-3-5-reconcile-public-error-copy-with-its-states.md](evidence-3-5-reconcile-public-error-copy-with-its-states.md), created with the implementation.

## Spec Change Log

## Design Notes

Strings confirmed by the reviewer at Checkpoint 1, not proposals.

A new code `setup_failed`, heading "Couldn't prepare that item", message: *"FairDrop couldn't prepare that item. Nothing was sent. Choose it again."* The second sentence is the load-bearing one — it is what five states currently get wrong.

`busy` revised from *"Finish or cancel the current transfer before choosing another item."* to *"FairDrop is still finishing the last transfer. Wait a moment, or cancel it, then choose another item."* True while a transfer runs and while an outcome is held on screen, and the recovery applies in both: cancelling works during the terminal lease, and waiting clears it.

One code rather than three. A pre-startup refusal, an entropy failure and a parse failure differ in cause and not in what the user can do about them, and a registry the user never reads twice is not improved by precision they cannot act on.

## Verification

**Commands:**
- `gofmt -l .`; `go vet ./...`; `go tool staticcheck ./...`; `go test -count=1 -timeout 240s ./...`; `go test -count=1 -race -timeout 420s ./...` — clean
- `GOOS=darwin GOARCH=arm64 go vet ./...` and the linux equivalent, per AGENTS.md's pre-flight
- `cd frontend && npx vitest run`; then, alone, `wails build`
- Mutations: edit one message in each of the three files in turn; add a code to the Go table alone; revert a producer to its old code — each must fail a named test
