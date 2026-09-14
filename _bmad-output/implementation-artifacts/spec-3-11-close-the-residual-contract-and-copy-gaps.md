---
title: 'Story 3.11: Close the Residual Contract and Copy Gaps'
type: 'bugfix'
created: '2026-09-13'
status: 'done'
baseline_commit: 'bf925a6'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-3-context.md'
  - '{project-root}/_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-2026-08-23/EXPERIENCE.md'
  - '{project-root}/docs/fairdrop-contracts.md'
  - '{project-root}/AGENTS.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Nine states where FairDrop's account of itself is wrong. Five are copy: cancelling a staged link whose teardown times out says "the transfer stopped before FairDrop finished sending" when no transfer began (D-103); a `Cancel` with no coordinator says the same, and that state is now genuinely reachable because Story 3.10's backstop leaves a second instance uncomposed on purpose (D-104); a failed clipboard write says it too (D-104); a malformed Stage acknowledgement whose cleanup also failed says "Nothing was sent", which is true, while plenty was *started* and the next attempt is refused `busy` (D-106); a folder holding one unsendable entry name is refused with "FairDrop can use regular files and folders only", which the folder plainly is (D-112); and `busy` tells a user to cancel when the thing they would cancel is an uninterruptible filesystem call (D-111). Two are contract: a `ServerComplete` carrying no snapshot has no row saying what is published for it (D-035), and `sanitizeProgress` distrusts NaN while trusting the known/unknown-total invariant beside it (D-039). Two are the contexts themselves: `StageTransfer` and `CancelTransfer` fabricate a background context before the window exists while the dialogs refuse (D-048), and the context Wails hands `Cancel` and `Shutdown` can never be cancelled, so honouring it is unreachable in a shipped build (D-097).

**Approach:** Give each state a sentence that is true of it, through the four-file procedure Story 3.5 established; write the two contract rows; and make the two context paths mean what the tests say they mean.

## Boundaries & Constraints

**Always:** A new code enters `EXPERIENCE.md` by stable key first, then the contract table, `internal/transfer/errors.go` and `frontend/src/transfer/errors.ts` move together under the cross-language pin. AD-9 holds: no message may carry a path, a token or a selected name. A state keeps an existing code when that code is already true of it.

**Ask First:** Any wording beyond the four codes and one revision the owner approved on 2026-09-13. Whether a single offending filename segment may be echoed to the user — that is a disclosure question, not a copy question, and it is unanswered.

**Never:** Invent a code for a state an existing code already describes. Weaken the identical-body rule or any refusal in order to make a message fit. Decide Story 3.12's accessibility capture.

**Deferral rule for this story:** a review finding is fixed here unless it needs a human decision, lives in a package this Code Map does not name, or is a genuinely different goal. Each deferred entry states which.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Cancel a staged link, teardown bound elapses | no transfer ever began | Nothing was sent; the release is unconfirmed | `cleanup_unconfirmed` (D-103) |
| Stage acknowledgement malformed, cleanup also fails | a live staged session remains | Same truth, same code | `cleanup_unconfirmed` (D-106) |
| Stage acknowledgement malformed, cleanup succeeds | nothing remains | Unchanged | `setup_failed` |
| A command before the window exists | uncomposed App, or pre-startup | Refused, as the dialogs already refuse | `not_ready` (D-048, D-104) |
| Clipboard write fails | staged session, link on screen | The link is still there to copy by hand | `clipboard_failed` (D-104) |
| A folder entry whose name a receiver cannot save | an ordinary folder | The refusal names the name, not the item | `name_unsupported` (D-112) |
| A stuck filesystem lookup refuses the next Stage | uninterruptible OS call outstanding | Copy offers waiting or restarting, never cancelling | revised `busy` (D-111) |
| `ServerComplete` with no snapshot | a port defect | The contract states what is published | Documented (D-035) |
| Progress with an incoherent unknown total | `TotalKnown=false`, non-zero `Percent` | Enforced at the boundary or documented as the producer's | Decided (D-039) |
| Cancel or Shutdown with a cancellable context | the caller cancels | The wait ends | Proven by test (D-097) |

</frozen-after-approval>

## Code Map

- `_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-2026-08-23/EXPERIENCE.md:116-136` — the stable copy table, thirteen rows, each with heading, exact message, sole announcement owner and recovery. Four rows join it and `busy`'s message changes.
- `docs/fairdrop-contracts.md` — the prose table **and** the Go constant block, which disagreed once before and are now both pinned; `internal/transfer/errors.go`'s `publicMessages`; `frontend/src/transfer/errors.ts`'s `transferErrorCodes` and `fixedErrorMessages`. `main_test.go`'s `TestTheCrossLanguageErrorRegistryPinsEveryCodeAndMessage` fails naming all four places when one drifts.
- `internal/transfer/lifecycle.go` `retire` — returns `unwind`'s first bound failure directly, which is how a staged-session Cancel inherits `ErrTransferFailed` (D-103). `Cancel`'s nil-receiver answer is the other half of D-104, and its comment is already stale about how Stage answers the same case.
- `app.go` `delegate()` — defaults a nil `a.ctx` to `context.Background()`, which is what lets a pre-startup Stage bind a listener whose events `publish` then drops; `chooseWith` refuses instead (D-048). `app.go`'s three `CopyToClipboard` sites are D-104's clipboard half.
- `app.go` `startup`/`shutdown` — where a cancellable derivation of the Wails context has to be made and released, since Wails builds its context from `context.Background()` plus `WithValue` and never wraps it (D-097).
- `frontend/src/transfer/useTransfer.ts` `stage()` — the bare `catch {}` around the best-effort `CancelTransfer` that turns a failed cleanup into a silent one (D-106).
- `internal/transfer/outcomes.go` `sanitizeProgress` and `acceptTerminal`'s snapshot-less `Complete` branch (D-035, D-039).
- `internal/source/source.go` `childRelativeName` and `internal/transfer/archive_name.go` `SafeArchiveSegment` — where a rejected entry name becomes `ErrPathUnsupported` today (D-112).

## Tasks & Acceptance

**Execution:**
- [x] `EXPERIENCE.md` — four new rows and the revised `busy` message, by stable key, before any code changes.
- [x] `docs/fairdrop-contracts.md`, `internal/transfer/errors.go`, `frontend/src/transfer/errors.ts` — the same four codes and the revision, moved together under the pin.
- [x] `internal/transfer/lifecycle.go` — a staged-session Cancel whose unwind hit a bound reports `cleanup_unconfirmed`; the nil receiver reports `not_ready`.
- [x] `app.go` — `delegate()` refuses before the window exists rather than fabricating a context; clipboard failures report `clipboard_failed`; a cancellable context is derived at startup and cancelled at shutdown.
- [x] `frontend/src/transfer/useTransfer.ts` — a failed cleanup after a malformed acknowledgement reports `cleanup_unconfirmed` instead of being swallowed.
- [x] `internal/source`, `internal/transfer/archive_name.go` — an unsendable entry name reports `name_unsupported`.
- [x] `internal/transfer/outcomes.go` — `sanitizeProgress` enforces the unknown-total invariant it sits beside; the contract gains the snapshot-less `Complete` row.
- [x] `evidence-3-11-close-the-residual-contract-and-copy-gaps.md`, the nine ids discharged, `epics.md` kept in step.

**Acceptance Criteria:**
- Given each of the five states, when it is reached, then the message describes that state and no other, and a test drives the state rather than asserting the string alone.
- Given the four new codes, when any of the four files drifts, then the cross-language pin fails naming all four places.
- Given `Cancel` or `Shutdown` handed a cancellable context, when the caller cancels it, then the wait ends — proved by cancelling it, not by passing it.
- Given a command before the window exists, when it is called, then it is refused rather than binding a listener whose events are dropped.

## Evidence

Audit, mutation tables and gate transcripts live in
[evidence-3-11-close-the-residual-contract-and-copy-gaps.md](evidence-3-11-close-the-residual-contract-and-copy-gaps.md), created with the implementation.

## Spec Change Log

**2026-09-13 (approval).** The owner approved four new codes and one revision, drafted in the
registry's voice and quoted here so the implementation cannot drift from what was agreed:

- `cleanup_unconfirmed` — "Couldn't confirm it stopped" — "FairDrop couldn't confirm it released the
  connection. Nothing was sent. Close FairDrop and reopen it before sending again." Covers D-103 and
  D-106's failed-cleanup case, because the true statement is identical in both.
- `not_ready` — "FairDrop isn't ready" — "FairDrop isn't ready to send. Another copy may already be
  running. Close this window and use that one."
- `clipboard_failed` — "Couldn't copy the link" — "FairDrop couldn't copy the link. Select the link
  and copy it yourself."
- `name_unsupported` — "A name can't be sent" — "One name inside that folder can't be saved on the
  receiving device. Rename anything containing : < > ? * or ending with a dot or space."
  **Amended during implementation:** the approved draft enumerated `"` and `|` as well. A pipe
  cannot sit in a markdown table cell, and escaping it in `EXPERIENCE.md` would have left the
  cross-language pin comparing an escaped string against the unescaped one the code carries — the
  exact drift that pin exists to catch. Two characters were dropped rather than the rule: the
  sentence still tells a user what to look for, and the refusal itself is unchanged.
- `busy` revised — "FairDrop is still finishing the last item. If it doesn't finish, close FairDrop
  and reopen it." The clause dropped is "or cancel it", which is false when the outstanding work is
  an uninterruptible filesystem call (D-111).

Rejected: collapsing these into two broader codes. A message vague enough to cover three causes is
the thing Story 3.5 spent a story removing.

**2026-09-13 (mid-implementation).** The owner questioned the refusal behind `name_unsupported`
rather than its wording, and that changed the design: `SafeArchiveSegment` split into a security
predicate that refuses and a portability predicate that warns, so a folder holding one
Windows-awkward entry name now transfers with a Staged warning instead of being refused. The
approved `name_unsupported` copy was rewritten for what it now covers, and `name_warning` was added
as a fifth string with the owner's approval. D-112's fix is therefore the removal of a refusal
rather than the rewording of one.

## Design Notes

**One code for two states is not the same as a vague code.** `cleanup_unconfirmed` covers a
cancelled staged session and a malformed acknowledgement whose cleanup failed because in both the
user needs the same three facts: nothing was sent, FairDrop cannot promise the connection is
released, and the way out is a restart. That is one state described once, not two states sharing a
shrug.

**`not_ready` is reachable now in a way it was not.** Story 3.10's backstop deliberately leaves a
second instance uncomposed rather than letting it start a competing listener, so "no coordinator" is
a real user-facing state for the first time, and it has a real recovery: use the window that is
already open.

**The unsendable-name message describes the rule, not the file.** AD-9 forbids a path, and whether
one offending segment may be echoed is a disclosure question nobody has answered. Naming the
character classes is useful and discloses nothing, so that is what ships; echoing the segment stays
an open question rather than an assumed yes.

## Verification

**Commands:**
- `gofmt -l .`; `go vet ./...`; `go tool staticcheck ./...`; `go test -count=1 -timeout 300s ./...`; `go test -count=1 -race -timeout 1200s ./...` — clean
- `GOOS=darwin GOARCH=arm64 go vet ./...` and the linux equivalent
- `cd frontend && npx vitest run`; then, alone, `wails build`
- Mutations: change one message in one of the four files; return `transfer_failed` from the staged-session Cancel; let `delegate()` fabricate a context again; swallow the failed cleanup; pass a non-cancellable context to Cancel — each must fail a named test
