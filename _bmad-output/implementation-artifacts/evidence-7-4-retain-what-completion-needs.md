# Evidence: Story 7.4: Retain What Completion Needs

## What changed

`DoneTransferState.outcome` (`frontend/src/transfer/state.ts`) grew one
field: `receipt: CompletionReceipt`, alongside the existing `kind: 'done'`.
`RetainedDoneOutcome` (`frontend/src/transfer/types.ts`) grew the same field.
Both were `{kind: 'done'}` and nothing else before this story -- the exact
shape Epic 7's diagnosis named as the reason the finished-transfer screen
renders "a small bit of text and a bunch of empty space."

`CompletionReceipt` (`frontend/src/transfer/types.ts`) is new:

```ts
export interface CompletionReceipt {
    readonly name: string
    readonly isDir: boolean
    readonly bytesSent: number
}
```

It is a purpose-built projection, not `FileMetadata`. See "Narrowed to
`CompletionReceipt`" below for why.

The reducer's `transferring` -> `done` transition (`reduceLifecycle`'s
`transfer-complete` branch) builds the receipt from values already in hand
at that exact line: `name`/`isDir` from `state.metadata` (the
`TransferringTransferState`'s own metadata, already validated at Stage) and
`bytesSent` from `event.progress.bytesSent` (the `TransferCompleteEvent`'s
final snapshot, already validated by
`parseLifecycleEvent`/`parseProgressSnapshot`). Neither source value is
refetched, recomputed, or newly validated -- `name`/`isDir`/`bytesSent` were
already in hand and being discarded at this line, not missing.

The `done` -> `idle` transition (on a matching `transfer-reset`) carries
`state.outcome.receipt` straight into the new `retainedOutcome`, unchanged.
The session cursor (`sessionId`/`lastSeq`) is still scrubbed at this
transition exactly as before -- only the receipt survives reset, not the
correlation data, and the receipt itself carries no correlation data either
(see below).

`selectors.ts`'s `OutcomePresentation` (`kind: 'done'` branch) grew the same
field, and `selectOutcome` forwards `state.outcome.receipt` (live Done) or
`retained.receipt` (retained Done in Idle) into the presentation unchanged --
it computes nothing, it only routes the one value that exists to one place a
view can read regardless of whether the outcome is live or retained.

`OutcomePanel.tsx` gained two non-visible attributes,
`data-receipt-name`/`data-receipt-bytes-sent`, set from
`outcome.receipt.name`/`outcome.receipt.bytesSent` on the Done branch only.
This is the "minimum rendering change" the story explicitly allows to prove
the retained values reach the view -- it is deliberately not the receipt's
markup, layout, or styling, which is Story 7.5's job and out of this story's
scope. The diff to this file is two lines, on purpose: Story 7.2 is
concurrently changing `OutcomePanel.tsx` on `epic-7-quartz`, and keeping this
touch minimal keeps that merge conflict small.

`ErrorTransferState` and `RetainedErrorOutcome` are byte-for-byte unchanged;
this story does not touch the error path at all (verified by mutation M6
below).

### Files touched

- `frontend/src/transfer/types.ts` -- `CompletionReceipt` (new),
  `RetainedDoneOutcome`.
- `frontend/src/transfer/state.ts` -- `DoneTransferState`, the two
  transitions above.
- `frontend/src/transfer/selectors.ts` -- `OutcomePresentation`,
  `selectOutcome`.
- `frontend/src/ui/OutcomePanel.tsx` -- two proof-only `data-*` attributes.
- `frontend/src/transfer/state.test.ts`, `frontend/src/transfer/selectors.test.ts`,
  `frontend/src/transfer/useTransfer.test.tsx` -- extended per the story's
  explicit instruction, not loosened.
- `frontend/src/ui/OutcomePanel.test.tsx`, `frontend/src/App.test.tsx`,
  `frontend/src/App.focus.test.tsx`, `frontend/src/ui/IdleView.test.tsx`,
  `frontend/src/ui/announce.test.ts` -- mechanical fixture updates. Every one
  of these files constructs a `TransferState`/`OutcomePresentation` `'done'`
  literal directly (rather than through the reducer), so the new required
  field is a compile error there until supplied. No test's assertions were
  weakened; each got a fixture `doneReceipt`/`doneOutcome()` helper added to
  its existing literal.

## Narrowed to `CompletionReceipt` -- a follow-up from review

The first pass of this story retained the full `FileMetadata` object (the
acceptance criteria's literal wording: "retains the session's `FileMetadata`
and the event's final `ProgressSnapshot`"), and flagged that as a possible
tidiness concern for the story owner. Review correctly identified that this
was understating it: it is the product's own premise, not a preference.

`FileMetadata` carries `url` (the one-shot capability download link) and
`qrBase64` (a scannable PNG of that same link). The retained value this story
adds lives in `RetainedDoneOutcome`, which sits in **Idle** *after* the
session has been reset and the server has stopped serving that URL.
FairDrop's contract is that it is ephemeral and persists nothing; a
sender-side surface that could re-present a dead capability link -- or hold a
scannable QR of one indefinitely, for no reason -- is exactly what that
contract exists to prevent. It is also a base64 PNG string retained
indefinitely with no purpose once the Done receipt no longer needs it.

The fix: `DoneTransferState.outcome` and `RetainedDoneOutcome` now carry
`receipt: CompletionReceipt` (`name`, `isDir`, `bytesSent`) instead of
`metadata: FileMetadata` plus `progress: ProgressSnapshot`. `isDir` is kept
because Story 7.5 may need it to phrase the receipt differently for a folder.
`url`, `qrBase64`, `size`, `sessionId`, and `warnings` are structurally
absent -- not merely unused, but impossible to reach through this type, which
guards the reducer's construction site against a future edit that widens the
receipt back toward the full metadata. `bytesSent` still comes from
`event.progress.bytesSent`, never `metadata.size` (unchanged from the first
pass; see M-size below).

Two consequences of the narrowing, both intentional: `selectOutcome` is now a
pure pass-through with no computation of its own (it used to build the
presentation's `metadata`/`progress` pair; now it forwards one already-built
`receipt`), and the wire-bytes-vs-logical-size guarantee has exactly one
place it can be broken -- the reducer's receipt construction -- rather than
two (the reducer and the selector each used to independently carry both raw
values). This is a smaller attack surface for the same guarantee, which is
why the mutation table below is shorter than the first pass's, not weaker.

## Mutation table

Each mutation was applied to the real working tree, run against the real
suite, confirmed to fail and name the problem, then reverted before the next
one (`diff` against a pre-mutation backup confirmed a clean revert after the
whole pass, for every one of `state.ts`, `selectors.ts`, and
`OutcomePanel.tsx`).

| # | Mutation | AC | Result |
|---|---|---|---|
| M-drop | Dropped `receipt` from the `transfer-complete` reducer transition (`outcome: {kind: 'done'} as never`) | AC1 | KILLED -- 5 failures in `state.test.ts`: the wire-bytes test (`Cannot read properties of undefined (reading 'bytesSent')`), the no-URL/no-QR test (`Cannot convert undefined or null to object`), the duration-guard test (`expected ['kind'] to deeply equal ['kind','receipt']`), the primary retention `toEqual`, and the reset-carries-the-same-receipt test |
| M-size | `receipt.bytesSent` built from `state.metadata.size` instead of `event.progress.bytesSent` | AC3 | KILLED -- `state.test.ts`, `shows the wire bytes actually sent, never metadata.size`: `expected +0 to be 4096` (the directory fixture's `metadata.size` is 0, distinguishing it from the wire bytes the mutation should have used) |
| M-duration | Added `elapsedMs: 0` to the receipt at construction | AC4 | KILLED -- 4 failures in `state.test.ts`, including the duration-guard test naming the exact field: `expected ['bytesSent','elapsedMs','isDir','name'] to deeply equal ['bytesSent','isDir','name']` |
| M-leak | **The mutation review asked for.** `receipt` built as `{...state.metadata, bytesSent: event.progress.bytesSent}` -- the full `FileMetadata` spread back in, exactly the regression the narrowing exists to prevent | AC (ephemerality) | KILLED -- 6 failures across two files: `state.test.ts`'s key-shape guards name the leak directly (`expected {8 keys} to deeply equal {name, isDir, bytesSent}`), and `useTransfer.test.tsx`'s dedicated capability-leak test fails with the literal URL substring in its message: `the capability URL must not outlive the session: expected '...' not to contain 'fedcba9876543210fedcba9876543210'` |
| M-error | Added `receipt: state.metadata` to the `transferring` -> `error` transition's outcome | AC5 | KILLED -- `state.test.ts`, `adds nothing to the Error outcome shape: this story only retains a receipt for Done`: `expected ['error','kind','receipt'] to deeply equal ['error','kind']`, plus an existing `toEqual` in the failure-grammar test |
| M-panel | `OutcomePanel.tsx`'s `data-receipt-bytes-sent` set from `outcome.retained` instead of `outcome.receipt.bytesSent` | wiring proof | KILLED -- `OutcomePanel.test.tsx`, two tests: `expected 'true' to be '100'` and `expected 'false' to be '4096'` |

M-drop proves the primary claim (AC1: the receipt is retained, not
discarded). M-size proves AC3 (wire bytes, not logical size), using an
unknown-total directory transfer where nothing forces `metadata.size` and
`progress.bytesSent` to the same value the way a plain file's validation does
(`progressMatchesMetadata` pins `progress.totalBytes === metadata.size` for a
file, and `parseLifecycleEvent`'s `transfer-complete` branch pins
`bytesSent === totalBytes` whenever `totalKnown`, so for an ordinary file the
two figures are mathematically forced equal and a size-for-bytes substitution
would be invisible). M-duration proves AC4 (the duration ban) by name.
M-leak is the mutation review specifically asked for, proving the
ephemerality guarantee: putting the full metadata back on the retained
outcome fails loudly, in two independent places, and the failure message
names the actual leaked capability URL. M-error proves AC5 (the error path is
untouched). M-panel proves the view-wiring attributes actually read the
receipt rather than some other field that happens to be present.

## Gate transcripts (macOS arm64, native)

Run in the order `.github/workflows/verify.yml` uses, after the mutation
pass above and with the working tree back to its intended diff (confirmed
identical to the pre-mutation-pass backups by `diff`, for every mutated
file).

- `wails build`: **PASS**. `Built '.../fairdrop.app/Contents/MacOS/fairdrop' in 9.315s.` One pre-existing linker warning (`object file ... built for newer 'macOS' version (13.0) than being linked (11.0)`), unrelated to this change.
- Bindings drift / `.gitkeep` check: **PASS**. `frontend/dist/.gitkeep` present; `git -c core.fileMode=false diff --quiet -- frontend/wailsjs` exit 0 (no drift after `chmod 644` on the three regenerated bindings files, per the documented mode-churn pitfall); `scripts/verify-build-asset-drift.sh` exit 0, no output.
- `gofmt -l .`: **PASS**, no output.
- `go vet ./...`: **PASS**, no output.
- `go tool staticcheck ./...`: **PASS**, no output.
- `go test -count=1 ./...`: **PASS** -- `ok` for `fairdrop`, `internal/network`, `internal/qr`, `internal/server`, `internal/source`, `internal/stream`, `internal/transfer`, `scripts`, `scripts/mutationverdict`.
- `CGO_ENABLED=1 go test -count=1 -race ./...`: **PASS** -- `ok` for all nine packages (`internal/stream` ~105s under the race detector, the rest a few seconds each). `go env CGO_ENABLED` confirmed `1` before trusting the result.
- `GOOS=windows GOARCH=amd64 go build ./...`: **PASS**.
- `GOOS=linux GOARCH=amd64 go build ./...`: **PASS** (pre-flight, per AGENTS.md; not part of the story's minimum gate but run anyway).
- `cd frontend && npm test -- --run` (not concurrent with `wails build`): **PASS** -- 17 files, **562 tests** (549 baseline before this story + 13 net new across both review passes: the narrowing pass replaced a few metadata/progress-shaped tests with equivalent receipt-shaped ones and added the two dedicated capability-leak tests).
- `cd frontend && npm run test:browser`: **PASS** -- 1 file, 17 tests, unchanged.
- `git diff --check`: **PASS**, no output (no whitespace errors).
- Line-ending check (`git ls-files --eol | grep -E '(i|w)/(crlf|mixed)'`): **PASS**, no matches.

The Go suite is unaffected by this story (it is entirely a `frontend/src/transfer`
and `frontend/src/ui` change); it was run in full anyway because the gate is
"the full gate," not a scoped one.

## Nothing left open

All six Story 7.4 acceptance criteria are implemented and mutation-verified
above:

1. `DoneTransferState` retains a completion receipt (name, isDir, wire bytes
   sent) at the `transfer-complete` transition (M-drop).
2. A retained Done outcome in Idle carries the same receipt
   (`carries the same retained receipt into Idle after reset` in
   `state.test.ts`, and the live-vs-retained equality test in
   `selectors.test.ts`).
3. The receipt's bytes are the wire bytes actually sent, never the logical
   size (M-size).
4. No duration/elapsed-time field exists anywhere on the retained shapes
   (M-duration).
5. The error path's state shape is unchanged (M-error).
6. The full gate passes, and `state.test.ts`, `selectors.test.ts`, and
   `useTransfer.test.tsx` were extended, not loosened -- no existing
   assertion was weakened or deleted; the one test whose premise the story
   explicitly reverses (the old "discards metadata at terminal" pin) was
   replaced with tests pinning the new, opposite, spec-mandated behavior,
   under a renamed `describe` block that says why.

Additionally, following review: the retained Done outcome structurally
cannot carry the one-shot capability URL or its QR code (M-leak), which is
FairDrop's own ephemerality contract rather than an incidental tidiness
choice -- both `state.test.ts` and `useTransfer.test.tsx` pin this with tests
named so a failure states why, not just what.

Deliberately out of scope, per the story text: `OutcomePanel.tsx`'s markup,
layout, and visible styling were not touched beyond the two non-visible
`data-*` proof attributes, kept to a two-line diff so Story 7.2's concurrent
change to the same file (on `epic-7-quartz`) stays a small conflict to
resolve at integration. Story 7.5 owns rebuilding the panel to actually
display the two-cell receipt (`DESIGN.md`'s "Completion Receipt" row) from
the state this story now retains.
