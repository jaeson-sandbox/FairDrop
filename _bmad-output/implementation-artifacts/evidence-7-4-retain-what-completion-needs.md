# Evidence: Story 7.4: Retain What Completion Needs

## What changed

`DoneTransferState.outcome` (`frontend/src/transfer/state.ts`) grew two
fields: `metadata: FileMetadata` and `progress: ProgressSnapshot`, alongside
the existing `kind: 'done'`. `RetainedDoneOutcome`
(`frontend/src/transfer/types.ts`) grew the same two fields. Both were
`{kind: 'done'}` and nothing else before this story -- the exact shape
Epic 7's diagnosis named as the reason the finished-transfer screen renders
"a small bit of text and a bunch of empty space."

The reducer's `transferring` -> `done` transition
(`reduceLifecycle`'s `transfer-complete` branch) now builds the outcome from
values already in hand at that exact line: `metadata: state.metadata` (the
`TransferringTransferState`'s own metadata, already validated at Stage) and
`progress: event.progress` (the `TransferCompleteEvent`'s final snapshot,
already validated by `parseLifecycleEvent`/`parseProgressSnapshot`). Neither
value is refetched, recomputed, or newly validated -- they were already being
discarded at this line, not missing.

The `done` -> `idle` transition (on a matching `transfer-reset`) now carries
`state.outcome.metadata` and `state.outcome.progress` straight into the new
`retainedOutcome`, unchanged. The session cursor (`sessionId`/`lastSeq`) is
still scrubbed at this transition exactly as before -- only the receipt's two
values survive reset, not the correlation data.

`selectors.ts`'s `OutcomePresentation` (`kind: 'done'` branch) grew the same
two fields, and `selectOutcome` now forwards `state.outcome.metadata`/
`state.outcome.progress` (live Done) or `retained.metadata`/`retained.progress`
(retained Done in Idle) into the presentation, so a view reads both from one
place regardless of whether the outcome is live or retained.

`OutcomePanel.tsx` gained two non-visible attributes,
`data-receipt-name`/`data-receipt-bytes-sent`, set from
`outcome.metadata.name`/`outcome.progress.bytesSent` on the Done branch only.
This is the "minimum rendering change" the story explicitly allows to prove
the retained values reach the view -- it is deliberately not the receipt's
markup, layout, or styling, which is Story 7.5's job and out of this story's
scope.

`ErrorTransferState` and `RetainedErrorOutcome` are byte-for-byte unchanged;
this story does not touch the error path at all (verified by mutation M6
below).

### Files touched

- `frontend/src/transfer/state.ts` -- the two type/transition changes above.
- `frontend/src/transfer/types.ts` -- `RetainedDoneOutcome`.
- `frontend/src/transfer/selectors.ts` -- `OutcomePresentation`, `selectOutcome`.
- `frontend/src/ui/OutcomePanel.tsx` -- two proof-only `data-*` attributes.
- `frontend/src/transfer/state.test.ts`, `frontend/src/transfer/selectors.test.ts`,
  `frontend/src/transfer/useTransfer.test.tsx` -- extended per the story's
  explicit instruction, not loosened.
- `frontend/src/ui/OutcomePanel.test.tsx`, `frontend/src/App.test.tsx`,
  `frontend/src/App.focus.test.tsx`, `frontend/src/ui/IdleView.test.tsx`,
  `frontend/src/ui/announce.test.ts` -- mechanical fixture updates. Every one
  of these files constructs a `TransferState`/`OutcomePresentation` `'done'`
  literal directly (rather than through the reducer), so the new required
  fields are compile errors there until supplied. No test's assertions were
  weakened; each got a fixture `metadata`/`progress` (or a shared
  `doneOutcome()`/`doneReceipt` helper) added to its existing literal.

## A choice the story left implicit, flagged rather than guessed silently

The acceptance criteria say `DoneTransferState` "retains the session's
`FileMetadata`" -- the whole object, not a projection of it. I implemented
that literally: the full `FileMetadata` (including `url` and `qrBase64`) is
retained through Done and through reset into the retained Idle outcome, even
though Story 7.5's two-cell receipt (DESIGN.md, "Completion Receipt") only
ever reads `metadata.name` and `progress.bytesSent`. The alternative --
retaining a narrower `{name}` projection -- would have been smaller and closer
to "only what completion needs" (this story's own title), but the acceptance
criteria's literal wording ("retains... `FileMetadata`", "carries the same
two values") and the story's explicit scope ("`DoneTransferState`... and
`RetainedDoneOutcome`", not a new narrower type) both point at the full type.
Flagging this for the story owner: if a narrower projection was intended, the
acceptance criteria should name it explicitly (e.g. "retains the item's
name"), because "the session's `FileMetadata`" is the existing, already-named
type that includes the capability URL and QR payload.

## Mutation table

Each mutation was applied to the real working tree, run against the real
suite, confirmed to fail and name the problem, then reverted before the next
one (`diff` against a pre-mutation backup confirmed a clean revert after the
whole pass).

| # | Mutation | AC | Result |
|---|---|---|---|
| M1 | Dropped `metadata` from the `transfer-complete` reducer transition (`outcome: {kind: 'done', progress: event.progress}`) | AC1 | KILLED -- 4 failures in `state.test.ts`: `TypeError: Cannot read properties of undefined (reading 'size')` in `shows the wire bytes actually sent, never metadata.size`, plus the duration-guard test naming the missing key (`expected ['kind','progress'] to deeply equal ['kind','metadata','progress']`), plus the `toMatchObject` retention test and the reset-carries-the-same-values test |
| M2 | Dropped `progress` from the same transition (`outcome: {kind: 'done', metadata: state.metadata}`) | AC1 | KILLED -- same 4 tests, this time `Cannot read properties of undefined (reading 'bytesSent')` and the duration-guard test reporting `expected ['kind','metadata'] to deeply equal ['kind','metadata','progress']` |
| M3 | `selectOutcome`'s live-Done branch: `progress: {...state.outcome.progress, bytesSent: state.outcome.metadata.size}` (substitutes `metadata.size` for `progress.bytesSent`) | AC3 | KILLED -- `selectors.test.ts`, `reads bytes from the retained progress snapshot, never from metadata.size`: `expected +0 to be 4096` |
| M4 | `OutcomePanel.tsx`'s `data-receipt-bytes-sent` set from `outcome.metadata.size` instead of `outcome.progress.bytesSent` | AC3 | KILLED -- `OutcomePanel.test.tsx`, `shows the wire bytes actually sent, never the logical file size`: `expected '999999' to be '100'` |
| M5 | Added `elapsedMs` to both the live `outcome` and the retained `retainedOutcome` at the two reducer transitions | AC4 | KILLED -- 3 failures in `state.test.ts`, including the duration-guard test naming the exact field: `expected ['elapsedMs','kind','metadata','progress'] to deeply equal ['kind','metadata','progress']` |
| M6 | Added `metadata: state.metadata` to the `transferring` -> `error` transition's outcome (both the disagreeing-snapshot and the `transfer-error`-event branches) | AC5 | KILLED -- `state.test.ts`, `adds nothing to the Error outcome shape: this story only retains a receipt for Done`: `expected ['error','kind','metadata'] to deeply equal ['error','kind']`, plus an existing `toEqual` in the failure-grammar test |

M1-M2 prove the primary claim (AC1: both values are retained, not discarded).
M3-M4 prove AC3 (wire bytes, not logical size) at both the selector layer and
the view-wiring layer -- deliberately using an unknown-total directory
transfer, where nothing forces `metadata.size` and `progress.bytesSent` to
the same value the way a plain file's validation does (`progressMatchesMetadata`
pins `progress.totalBytes === metadata.size` for a file, and
`parseLifecycleEvent`'s `transfer-complete` branch pins `bytesSent ===
totalBytes` whenever `totalKnown`, so for an ordinary file the two figures are
mathematically forced equal and a size-for-bytes substitution would be
invisible). M5 proves AC4 (the duration ban) by name. M6 proves AC5 (the
error path is untouched).

## Gate transcripts (macOS arm64, native)

Run in the order `.github/workflows/verify.yml` uses, after the mutation
pass above and with the working tree back to its intended diff (confirmed
identical to the pre-mutation-pass backup by `diff`).

- `wails build`: **PASS**. `Built '.../fairdrop.app/Contents/MacOS/fairdrop' in 13.811s.` One pre-existing linker warning (`object file ... built for newer 'macOS' version (13.0) than being linked (11.0)`), unrelated to this change.
- Bindings drift / `.gitkeep` check: **PASS**. `frontend/dist/.gitkeep` present; `git -c core.fileMode=false diff --quiet -- frontend/wailsjs` exit 0 (no drift after `chmod 644` on the three regenerated bindings files, per the documented mode-churn pitfall); `scripts/verify-build-asset-drift.sh` exit 0, no output.
- `gofmt -l .`: **PASS**, no output.
- `go vet ./...`: **PASS**, no output.
- `go tool staticcheck ./...`: **PASS**, no output.
- `go test -count=1 ./...`: **PASS** -- `ok` for `fairdrop`, `internal/network`, `internal/qr`, `internal/server`, `internal/source`, `internal/stream`, `internal/transfer`, `scripts`, `scripts/mutationverdict`.
- `CGO_ENABLED=1 go test -count=1 -race ./...`: **PASS** -- `ok` for all nine packages (`internal/stream` ~106s under the race detector, the rest a few seconds each). `go env CGO_ENABLED` confirmed `1` before trusting the result.
- `GOOS=windows GOARCH=amd64 go build ./...`: **PASS**.
- `GOOS=linux GOARCH=amd64 go build ./...`: **PASS** (pre-flight, per AGENTS.md; not part of the story's minimum gate but run anyway).
- `cd frontend && npm test -- --run` (not concurrent with `wails build`): **PASS** -- 17 files, **560 tests** (549 pre-existing + 11 new: 5 in `state.test.ts`, 2 in `selectors.test.ts`, 2 in `useTransfer.test.tsx`, and the fixture updates in the other five test files added no new tests, only new required fields).
- `cd frontend && npm run test:browser`: **PASS** -- 1 file, 17 tests, unchanged.
- `git diff --check`: **PASS**, no output (no whitespace errors).
- Line-ending check (`git ls-files --eol | grep -E '(i|w)/(crlf|mixed)'`): **PASS**, no matches.

The Go suite is unaffected by this story (it is entirely a `frontend/src/transfer`
and `frontend/src/ui` change); it was run in full anyway because the gate is
"the full gate," not a scoped one.

## Nothing left open

All six Story 7.4 acceptance criteria are implemented and mutation-verified
above:

1. `DoneTransferState` retains `FileMetadata` and the final `ProgressSnapshot`
   at the `transfer-complete` transition (M1, M2).
2. A retained Done outcome in Idle carries the same two values
   (`carries the same retained metadata and progress into Idle after reset`
   in `state.test.ts`, and the live-vs-retained equality test in
   `selectors.test.ts`).
3. The receipt's bytes are the wire bytes actually sent, never the logical
   size (M3, M4).
4. No duration/elapsed-time field exists anywhere on the retained shapes (M5).
5. The error path's state shape is unchanged (M6).
6. The full gate passes, and `state.test.ts`, `selectors.test.ts`, and
   `useTransfer.test.tsx` were extended, not loosened -- no existing
   assertion was weakened or deleted; the one test whose premise the story
   explicitly reverses (the old "discards metadata at terminal" pin) was
   replaced with tests pinning the new, opposite, spec-mandated behavior,
   under a renamed `describe` block that says why.

Deliberately out of scope, per the story text: `OutcomePanel.tsx`'s markup,
layout, and visible styling were not touched beyond the two non-visible
`data-*` proof attributes. Story 7.5 owns rebuilding the panel to actually
display the two-cell receipt (`DESIGN.md`'s "Completion Receipt" row) from
the state this story now retains.
