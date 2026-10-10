# Multiple selected items — frontend evidence

Baseline: `c56a39eee5494b8d8706e1b801e5fc077245223b`. Frontend implementation is uncommitted and is integrated with the parent story by the main agent.

## Implementation and acceptance

- Native single drops still enter the one-item Stage path. Native drops of two or more open the editable list; drops while it is open append. Add Files calls the native files-only multi-picker; Add Folder calls the existing directory picker. Empty chooser results change nothing. Paths live only in controller refs; views receive basename rows, and Send copies the exact ordered batch. A seventeenth append keeps the prior 16 rows and shows the safe fixed error.
- The controller rejects duplicate admissions, clears pending chooser completions on Cancel/unmount, and restores an editable list after backend batch refusal. One remaining row uses `StageTransfer`; two through 16 use `StageTransfers`. Send Again/retry copy remembered paths through guarded outcome release and Stage. Terminal release failure retains the outcome and sends nothing.
- Metadata validation accepts legacy singleton DTOs with both collection fields absent, accepts coherent `false/1` singletons and `true/2..16` collections, and refuses partial or inconsistent field pairs. Retained receipts carry collection kind/count without retaining URL or QR data. Staged and Sending show aggregate logical size and ZIP wording; collection ZIP progress remains unknown-total.
- The draft uses semantic buttons, explicit one-based ordinals, 44px targets, explicit focus styling, adjacent focus after removal, and forced-colors outlines. Display basenames remove controls and bidi formatting characters; private paths still reach backend validation unchanged. A restored refusal describes the focused heading. Chromium rendered tests cover keyboard activation, narrow width and forced-colors focus. Native macOS WebKit interaction was not observed in this frontend pass.

## Review findings and triage

Five frontend findings were reproduced as named assertion failures before the fixes; the complete red run is `evidence-multiple-selected-items-frontend-logs/review-five-red.log`.

| Finding | Resolution | Regression proof |
|---|---|---|
| Cancel on a restored draft retained retryable paths | `cancelDraft` now clears remembered paths; Send still re-records its copied batch at Stage | `forgets retryable remembered paths when a restored draft is cancelled` |
| Raw basenames could carry bidi/control text into rows and Remove labels | Draft display names now strip Cc/Cf and unsafe filename punctuation, trim trailing dots/spaces, and use a safe fallback; original private paths remain unchanged | `sanitizes draft display names but sends the untouched original paths` |
| Flex rows hid ordered-list markers | Rows now render explicit one-based ordinals and distinguish duplicate Remove names by ordinal | `shows explicit one-based order for identical basenames`; rendered browser ordinal check |
| A restored draft error could be silent on initial mount | The focused heading now points to the fixed error text with `aria-describedby` | `describes a newly focused draft heading with its restored refusal` |
| Malformed native drop on an open draft sent a reducer error hidden by the draft | `rejectSelection` now keeps the rows and places fixed `invalid_selection` on the draft | `keeps draft rows and shows a safe error after malformed native selection`; App native-drop routing test |

## Verification

| Command | Result |
|---|---|
| `cd frontend && npm run build` | Passed TypeScript and Vite production build after regenerated Wails bindings. |
| `cd frontend && npm test -- --run` | 22 files, 863 tests passed after review fixes. |
| `cd frontend && npm run test:browser` | 3 Chromium files, 86 tests passed after review fixes; live receiver GET/POST and attachment bytes passed. |
| `cd frontend && npm test -- --run src/transfer/useTransfer.test.tsx src/ui/CollectionDraftView.test.tsx` | 83 tests passed after review fixes. |
| `python3 scripts/verify-collection-frontend-mutations.py` | 10/10 canonical scoped cases killed by named assertions in isolated copies. |

The browser suite generated no tracked capture diff. Backend owns the integrated Wails/Go gate. The browser suite proves Chromium rendering, not native WKWebView focus or hardware Escape behavior.

## Scoped mutation inventory

The executable inventory is [`scripts/verify-collection-frontend-mutations.py`](../../scripts/verify-collection-frontend-mutations.py). It derives and runs all cases from its `CASES` tuple, checks a passing baseline and a unique normalized source anchor before each mutation, and rejects compile/import failures as kills. Complete separate baseline and mutant output is in `evidence-multiple-selected-items-frontend-logs/`.

| Case | Mutated guarantee | Named failing test |
|---|---|---|
| `metadata-count` | Accept 17 collection items | `accepts a coherent collection` |
| `append-limit` | Let a seventeenth item append | `rejects a seventeenth append` |
| `batch-command` | Route a remaining singleton through batch Stage | `restores the editable batch` |
| `draft-recovery` | Drop editable rows after backend refusal | `restores the editable batch` |
| `cancel-forgets` | Retain retry target after draft Cancel | `forgets retryable remembered paths` |
| `display-name` | Expose raw paths/bidi controls in draft display | `sanitizes draft display names` |
| `draft-refusal` | Leave malformed selection error outside draft | `keeps draft rows and shows a safe error` |
| `visible-order` | Hide one-based ordinals in flex rows | `shows explicit one-based order` |
| `remove-order` | Make duplicate Remove labels ambiguous | `shows explicit one-based order` |
| `error-focus` | Drop initial error description from focused heading | `describes a newly focused draft heading` |

The first metadata mutation exposed a weak fixture: the 17-item row still had a singleton name, so it failed for the wrong reason. The test now supplies `17 items`, and the count mutation fails the intended assertion.
