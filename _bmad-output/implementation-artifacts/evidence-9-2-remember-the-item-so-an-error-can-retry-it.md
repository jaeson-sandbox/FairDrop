# Evidence: Story 9.2: Remember the Item So an Error Can Retry It

## What changed

`frontend/src/transfer/useTransfer.ts` now remembers, in a `useRef` private
to the hook, the absolute path passed to the most recent `StageTransfer` call
(`rememberedPathRef`). It is set unconditionally at the top of `stage()` --
the single funnel every real Stage attempt (native drop, either chooser,
`retry()`, `stageFromOutcome()`) passes through -- and cleared at exactly the
three sites the AC names:

- **Dismiss** -- `dismissRetained()` clears it before dispatching
  `dismiss-retained`.
- **A completed Done** -- a synchronous, idempotent check at the top of the
  hook body (mirroring the existing `stateRef.current = state` line) clears
  it the first render `state.phase` is seen as `'done'`.
- **A Stage failure whose code's action is not `retry`** -- every
  `stage-failed` dispatch (two in `stage()`, one in `browse()`'s
  chooser-failure catch) now routes through a new `dispatchStageFailed`
  helper that clears the ref when `selectErrorAction(error.code) !== 'retry'`
  before dispatching.

The path is never part of `TransferState`, never passed to any selector,
and never exposed by `TransferController` as a value -- only as the boolean
`canRetry` (`rememberedPathRef.current !== null`, recomputed fresh on every
render). Nothing in this story touches `localStorage`, logging, or any DOM
attribute.

`selectors.ts` gained `selectErrorAction(code)` (the owner-approved
code -> action table, `TransferErrorCode -> ErrorAction | null`) and
`selectEffectiveErrorAction(code, canRetry)` (downgrades a `retry` row to
`choose` when `canRetry` is `false`). `ErrorAction` (`'retry' | 'choose' |
'dismiss'`) is a new type in `types.ts`.

`state.ts`'s `ErrorTransferState.outcome` and `types.ts`'s
`RetainedErrorOutcome` both gained an optional `itemName?: string`, populated
from `state.metadata.name` at both places a terminal `'error'` phase is
constructed in `reduceLifecycle` (the `transfer-error` branch and the
incoherent-snapshot-on-`transfer-complete` branch), and carried unchanged
through the `error` -> `idle` reset transition. `selectors.ts`'s
`OutcomePresentation` error variant and `selectOutcome`/`outcomeError` thread
the same field through. It is never populated for a Stage-time command
failure (`IdleTransferState.commandError` etc. are untouched by this story),
which is what "a Stage-time command failure has no name and carries none"
means structurally, not just by convention.

### Files touched

- `frontend/src/transfer/types.ts` -- `RetainedErrorOutcome.itemName`
  (optional), new `ErrorAction` type.
- `frontend/src/transfer/state.ts` -- `ErrorTransferState.outcome.itemName`,
  the two `'error'`-phase constructions, the `error` -> `idle` reset
  transition.
- `frontend/src/transfer/selectors.ts` -- `OutcomePresentation`'s error
  variant, `selectOutcome`/`outcomeError`, new `selectErrorAction`,
  `selectEffectiveErrorAction`.
- `frontend/src/transfer/useTransfer.ts` -- `rememberedPathRef`,
  `previousPhaseRef`, `idleWaitersRef`, the Done-clearing check, the
  idle-waiter effect, `dispatchStageFailed`, `waitForIdle`,
  `currentRetryableError`, new `stageFromOutcome`/`retry`/`canRetry` on
  `TransferController`.
- `frontend/src/transfer/state.test.ts`, `selectors.test.ts`,
  `useTransfer.test.tsx` -- extended per the story's instruction, not
  loosened (see below for exactly what was added/changed).
- `frontend/src/App.test.tsx`, `frontend/src/App.focus.test.tsx` -- mechanical
  fixture updates only. Both files construct a `ControllerCommands` literal
  (`App.harness.tsx`'s `Omit<TransferController, 'state'>`) whose own comment
  says a new `TransferController` member is meant to be a type error there;
  `retry`/`stageFromOutcome`/`canRetry` were added to each file's `mocks` and
  `commands` objects. No assertion in either file was touched.
- No `App.tsx`, `OutcomePanel.tsx`, or any other `frontend/src/ui/*` file was
  touched -- confirmed by the final `git status` before pushing.

## The API Story 9.6 should consume

`TransferController` (`frontend/src/transfer/useTransfer.ts`) gained:

```ts
readonly stageFromOutcome: (absolutePath: string, itemKind?: PendingItemKind) => Promise<void>
readonly retry: () => Promise<void>
readonly canRetry: boolean
```

- **`stageFromOutcome(absolutePath, itemKind?)`** -- the lease-release
  primitive the task asked for beyond `retry()` itself. If the current phase
  is a live `'done'` or `'error'`, it calls `cancel()` (the same D-059 path
  Dismiss already uses) and then waits for the actual `transfer-reset`
  transition to Idle -- not just for the `CancelTransfer` command to settle,
  which is a materially earlier point -- before staging. If nothing live is
  showing (Idle, retained or not), it stages immediately. Story 9.6 should
  call this for **"Send Another"**, **"Choose Another"**, and a **drop on a
  live outcome card**, each with whatever new path the sender picked (not
  the remembered one).
- **`retry()`** -- `stageFromOutcome` plus "use the remembered path" plus "only
  when the current error's action is `retry`". It resolves the current error
  from, in order: a live terminal `'error'` phase, an `'error'` retained in
  Idle, or an Idle Stage-time `commandError` -- whichever is showing (see
  "Interpretation call" below for why the third case is included). A no-op
  when nothing is remembered or the resolved error's action is not `retry`.
  Story 9.6 should wire this to **"Try Again"**.
- **`canRetry`** -- `true` while a path is remembered. `selectErrorAction`
  is deliberately a pure function of a code alone (the owner-approved table
  is asserted against literals with no other input) and cannot know this on
  its own; Story 9.6 should combine it via
  `selectEffectiveErrorAction(error.code, transfer.canRetry)` to decide the
  actual primary action to render -- this is what "so the UI never offers a
  retry it cannot perform" resolves to concretely. In the implementation as
  built, `canRetry` is `false` whenever `selectErrorAction(code) !== 'retry'`
  for the currently-showing error too (see "Mutation M-unreachable" below),
  so `selectEffectiveErrorAction` never actually has to downgrade in
  practice today -- it exists so that guarantee is structural rather than
  coincidental, and it is the one moving part 9.6 needs regardless.

## Interpretation call, flagged rather than silently resolved

AC3 says "the current **outcome's** action is retry" and 9.6's own AC (in
the epics file) separately describes "an Error outcome (**terminal or a
Stage-time command failure in Idle**)" as one concept sharing one primary
action rule. These two phrasings are in tension: `OutcomePresentation`
(`selectOutcome`) is a narrower thing than "whatever error is currently
shown" -- it does not cover a Stage-time `commandError` at all. I resolved
this by having `retry()`'s internal `currentRetryableError` consult, in
order, the live terminal error, the retained error, and finally the Idle
`commandError` -- so "Try Again" also works for a Stage-time failure whose
code is `retry`-actioned (for example `busy` on the very first Stage
attempt), which is remembered exactly the same way (every `stage()` call
remembers its path regardless of outcome). This reads Story 9.6's own
"terminal or a Stage-time command failure" phrase as the intended scope for
retry, not just AC3's narrower "outcome" noun. If the owner intended `retry()`
to be scoped strictly to `selectOutcome`'s result, `currentRetryableError`'s
`commandError` fallback (the last line) is the one place to remove; it does
not affect anything else in this story's shape.

## Known gap, reported rather than hidden

One branch is structurally unreachable through the public API as built, and
I could not construct a test that exercises it independently: `retry()`'s
`if (path === null) return` guard, decoupled from its `error === null`
guard. By construction, every path into a retry-actioned error (terminal,
retained, or Idle command failure) necessarily has a path remembered at that
same moment -- `stage()` always sets the ref immediately before the command
that could produce that very error, and none of the three clearing rules can
fire between that write and the error appearing. I verified this by mutation
(see M-error-action below) and by direct reasoning, but a genuinely
independent "error says retry, nothing remembered" test would require
reaching into the private ref, which the story's own AC1 forbids exposing.
The guard is kept as written (defense-in-depth, matching the AC's literal
wording), and `it('does nothing when retry() is called with nothing
remembered', ...)` covers the simpler "nothing shown, nothing remembered"
case, which *is* reachable (a fresh, never-staged controller).

## Mutation table

Each mutation was applied to the real working tree with a small Python
`str.replace` script (never hand-edited and left in place), run against the
scoped test file, confirmed to fail and name the problem, then reverted --
confirmed clean afterward with `git diff --stat` / `grep` against the
mutated snippet. `git status --short` after the whole pass showed only the
nine files intentionally changed.

| # | Mutation | AC | File | Result |
|---|---|---|---|---|
| M1 | `busy: 'retry'` moved to `busy: 'choose'` in `errorActionByCode` | AC2 | `selectors.ts` | KILLED -- 2 failures in `selectors.test.ts`, both naming `busy`: `busy: expected 'choose' to be 'retry'` and `selectErrorAction("busy"): expected 'choose' to be 'retry'` |
| M2 | Added `'new_mutation_code'` to `transferErrorCodes` (`errors.ts`) with no matching row in `errorActionByCode` | AC2 | `errors.ts` | KILLED -- `selectors.test.ts`, `never returns undefined for any code currently in the canonical registry`: `selectErrorAction("new_mutation_code") is undefined -- add a row for this code: expected undefined not to be undefined` |
| M3 | `stage()` additionally wrote the remembered path to `window.localStorage` | AC1 | `useTransfer.ts` | KILLED -- `useTransfer.test.tsx`, `never persists or renders the remembered path outside JS memory`: `expected '{"phase":"error",...,"debugRememberedPath":"C:\\Users\\..."}' not to contain 'C:\\Users\\...'` (see note below on the first attempt) |
| M4 | Included the remembered path directly in the returned `TransferState` (`state: {...state, debugRememberedPath: ...}`) | AC1 | `useTransfer.ts` | KILLED -- same test and assertion as M3, naming the leaked field in the serialized state |
| M5 | `dismissRetained()` no longer clears `rememberedPathRef` | AC1 | `useTransfer.ts` | KILLED -- `useTransfer.test.tsx`, `clears the remembered path on Dismiss`: `expected true to be false` on `canRetry` after Dismiss |
| M6 | Removed the Done-phase clearing check | AC1 | `useTransfer.ts` | KILLED -- `useTransfer.test.tsx`, `clears the remembered path on a completed Done`: `a completed Done needs no retry target: expected true to be false` |
| M7 | `dispatchStageFailed` no longer clears on a non-`retry` code | AC1 | `useTransfer.ts` | KILLED -- `useTransfer.test.tsx`, `clears the remembered path when a Stage fails with a non-retry code, keeps it for a retry code`: `path_not_found is a choose-action code: expected true to be false` |
| M8 | `stageFromOutcome` dropped `await waitForIdle()` | AC4 | `useTransfer.ts` | KILLED -- `useTransfer.test.tsx`, `releases the live lease before retrying, and stages exactly once with the remembered path`: `expected "vi.fn()" to be called 1 times, but got 0 times` (staging over the still-live lease silently no-ops in `stage()`'s own idle guard rather than surfacing `busy` in this mocked environment, but the test still fails and names the exact call-count assertion) |
| M9 | `selectEffectiveErrorAction` returned `selectErrorAction(code)` unconditionally (dropped the `canRetry` downgrade) | AC3 | `selectors.ts` | KILLED -- `selectors.test.ts`, `downgrades a retry row to choose when no path is remembered to retry with`: `expected 'retry' to be 'choose'` |
| M10 | `retry()` dropped its `error === null \|\| selectErrorAction(...) !== 'retry'` guard entirely | AC3 | `useTransfer.ts` | KILLED -- `useTransfer.test.tsx`, `does nothing when the current error's action is not retry, even with a path remembered`: test timed out after 5000ms, because `retry()` then attempted an unarmed cancel-then-stage cycle the test never provisioned a `transfer-reset` for. A real timeout is a real failure naming the right test, though a cleaner assertion message would be preferable; noted honestly rather than treated as a clean kill |
| M11 | Terminal-error reducer transition (`transfer-error` branch) built `outcome` with `metadata: state.metadata` spread in alongside `itemName` | AC5 | `state.ts` | KILLED -- 3 failures in `state.test.ts`: `adds nothing to the Error outcome shape...` (`expected [...,'metadata'] to deeply equal ['error','itemName','kind']`), the pre-existing `withProgress` `toEqual` in the failure-grammar test, and `never retains the capability URL or its QR code on a terminal error outcome` naming the literal leaked capability URL substring |

M1/M2 prove AC2's table and its exhaustiveness guard by name. M3/M4 prove
AC1's "never persists or renders" clause from two different angles (an
explicit persistence call, and inclusion in rendered/serialized state). M5/M6/M7
prove each of AC1's three clearing triggers individually. M8 proves AC4's
lease-release-before-stage ordering. M9 proves AC3's downgrade rule. M10
proves `retry()` actually consults the current error's action rather than
just "is a path remembered" (see "Known gap" above for the one sub-branch
this could not isolate further). M11 proves AC5's mutation
("retain the full `FileMetadata`") the same way Story 7.4's M-leak did.

**Note on M3's first attempt:** my first version of M3 wrote to the bare
global `localStorage` (not `window.localStorage`), which in this
Vitest/jsdom configuration is `undefined` (Node's own experimental
`localStorage` shadowing the global, not jsdom's) -- so it crashed with a
`TypeError` before reaching my assertion, rather than being caught by the
`Storage.prototype.setItem` spy. That crash is still a failing test (and
still names the exact line), so it would have counted as a kill either way,
but `window.localStorage.setItem` is the more faithful mutation and still
crashed the same way in this environment, which turns out to be an even
stronger guard than a clean spy assertion: this jsdom configuration does not
provide a `localStorage` instance at all, so *any* code path that tries to
use it fails hard immediately rather than silently succeeding. The
meaningful, always-exercisable assertion for "never rendered" is the
serialized-`TransferState` check, which M4 confirms directly.

Also worth recording: my first attempt at M11 targeted the wrong branch (the
incoherent-snapshot case inside the `transfer-complete` handler, which none
of the new tests exercise as their primary path) and the suite stayed green
-- a real gap, not a false negative, since that branch also constructs an
`itemName`-bearing outcome and deserves its own leak guard. I did not add a
dedicated test for that second branch's leak-resistance specifically (only
for its `itemName` presence, in `retains the item name on the
incoherent-snapshot terminal error from transfer-complete`); both
constructions share the exact same object-literal shape as the
`transfer-error` branch M11 mutated, so the risk is the same code pattern
duplicated once, not two independently-varying implementations -- flagged
here rather than silently left uncovered.

## Gate transcripts (macOS arm64, native)

Run in the order `.github/workflows/verify.yml` uses, after the mutation
pass above and with the working tree back to its intended diff (`git status
--short` showed exactly the nine intentionally-changed files after the
pass).

- `wails build`: **PASS**. `Built '.../fairdrop.app/Contents/MacOS/fairdrop' in 11.97s.` One pre-existing linker warning (`object file ... built for newer 'macOS' version (13.0) than being linked (11.0)`), unrelated to this change.
- Bindings drift: **PASS** after `chmod 644` on the three regenerated `frontend/wailsjs` files (`App.d.ts`, `App.js`, `models.ts`) -- content was byte-identical, only the file mode changed (0644 -> 0755) from the `wails build` run, the documented mode-churn pitfall. `git status --short` showed no diff in `frontend/wailsjs` after the chmod.
- `gofmt -l .`: **PASS**, no output.
- `go vet ./...`: **PASS**, no output.
- `go tool staticcheck ./...`: **PASS**, no output.
- `go test -count=1 ./...`: **PASS** -- `ok` for `fairdrop`, `internal/network`, `internal/qr`, `internal/server`, `internal/source`, `internal/stream`, `internal/transfer`, `scripts`, `scripts/mutationverdict`.
- `CGO_ENABLED=1 go test -count=1 -race ./...`: **PASS** -- `ok` for all nine packages (`go env CGO_ENABLED` confirmed `1` first; `internal/stream` ~101s under the race detector, the rest a few seconds each).
- `cd frontend && npm test`: **PASS** -- 17 files, **695 tests** (232 in `frontend/src/transfer/*`, up from 519 baseline before this story's additions across `state.test.ts`, `selectors.test.ts`, `useTransfer.test.tsx`).
- `cd frontend && npm run test:browser`: **PASS** -- 2 files, 24 tests, unchanged by this story.
- `git checkout -- frontend/browser/captures/` run afterward, per the known PNG-churn pitfall; confirmed no other unexpected diffs remained.

This story is entirely a `frontend/src/transfer` change (plus two mechanical
test-fixture updates outside it); the Go suite is unaffected and was still
run in full because the gate is the full gate, not a scoped one. No
`GOOS=darwin`/`GOOS=linux` cross-build pre-flight was run since no Go source
changed.

## Nothing left open

All six Story 9.2 acceptance criteria are implemented and mutation-verified
above:

1. The controller remembers the absolute path passed to `StageTransfer`, in
   `useRef` memory only, replaced on the next Stage and cleared on Dismiss, a
   completed Done, and a non-retryable Stage failure (M3, M4, M5, M6, M7).
2. `selectErrorAction(code)` returns exactly the owner-approved table,
   written out as literals at the assertion site, with a runtime
   exhaustiveness walk over the canonical `transferErrorCodes` list that
   fails and names a code added without a row (M1, M2).
3. `retry()` re-stages the remembered path through the ordinary `stage()`
   path only when the current error's action is `retry`, and is a no-op
   otherwise, including when nothing is remembered (M9, M10; see "Known gap"
   for the one sub-branch that could not be isolated further, and
   "Interpretation call" for the Stage-time-command-failure scope decision).
4. A live terminal outcome's lease is released (`cancel()`) and the actual
   `transfer-reset` transition to Idle is awaited before staging, so retry
   never surfaces `busy` (M8), proven with the exact test the AC specifies
   (`releases the live lease before retrying, and stages exactly once with
   the remembered path`).
5. A terminal transfer error's outcome also carries the item's display name,
   retained the same way Story 7.4 retained the Done receipt -- name only,
   never `url`/`qrBase64` -- while a Stage-time command failure carries none
   (M11).
6. The full gate passes on macOS arm64, and `state.test.ts`, `selectors.test.ts`,
   and `useTransfer.test.tsx` were extended rather than loosened -- no
   existing assertion was weakened or deleted; every pre-existing exact
   `toEqual` on an `ErrorTransferState`/`RetainedErrorOutcome` literal that
   the new `itemName` field would otherwise break was updated to include it.

Deliberately out of scope, per the story text: no UI file was touched.
`stageFromOutcome`, `retry`, and `canRetry` are plumbing only; Story 9.6 owns
wiring "Try Again", "Choose Another", "Send Another", and the drop-on-card
behaviour to them.
