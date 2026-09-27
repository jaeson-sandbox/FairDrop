# Evidence: Defect fix — URL field selection invisible against its field in dark mode

## Summary

Observed by the orchestrator on the built macOS binary, dark appearance:
Story 10.1 added `.fd-url::selection { background: var(--color-primary-tint);
color: var(--color-text); }` to give the Direct URL Row's select-all-on-focus
highlight a Quartz colour instead of the engine default. The field's text
*is* selected on focus — Cmd+C copies the URL correctly — but in dark mode
the selection is **invisible**: `--color-primary-tint`'s dark value
(`#372E2B`) is almost identical to `.fd-url`'s own background,
`--color-fill` (`#313135`). Story 10.1's own contrast check only ever
measured the selection's *text* against the tint (`text`/`primary-tint`,
already published and load-bearing for the drop zone's drag-active fill) —
nothing measured the tint itself against the field's own background that the
selection paints over. Before Story 10.1 the engine's own default selection
colour (blue) was at least visible against both fills, so this was a
regression a ship-time review that reused an already-proven pair, without
asking what the pair was actually being used for this time, did not catch.

This is the AGENTS.md "Git workflow" defect-fix carve-out: the defect was
already observed, a failing test was written before the fix (below), and the
fix is mutation-verified (below). No story exists for this change; the
failing test replaces the spec.

## The failing-first test, and its output before any fix

Added to `frontend/src/ui/styles.test.ts`, inside "the unrounded contrast
proof" describe block (where `contrast`/`lightTokens`/`darkTokens` are
already in scope from the existing infrastructure): a new `it.each` test
resolves whichever `var(--color-*)` `.fd-url::selection`'s `background`
currently reads, and whichever `var(--color-*)` `.fd-url`'s own `background`
currently reads, then asserts their contrast clears 1.5:1 — a *visibility*
floor ("can the selection be told apart from the row it sits on"), distinct
from the 4.5:1 *text* floor the existing pair already covered.

Run against the code exactly as observed (`.fd-url::selection` still reading
`var(--color-primary-tint)`, no `--color-selection` token declared yet):

```
 ❯ src/ui/styles.test.ts (192 tests | 2 failed | 190 skipped) 9ms
     × the URL field selection is visible against its own field: clears 1.5:1 in light mode 7ms
     × the URL field selection is visible against its own field: clears 1.5:1 in dark mode 1ms

⎯⎯⎯⎯⎯⎯⎯ Failed Tests 2 ⎯⎯⎯⎯⎯⎯⎯

 FAIL  src/ui/styles.test.ts > the unrounded contrast proof > the URL field selection is visible against its own field: clears 1.5:1 in light mode
AssertionError: primary-tint vs fill (light): expected 1.0159727871452413 to be greater than 1.5
 ❯ src/ui/styles.test.ts:2072:11

 FAIL  src/ui/styles.test.ts > the unrounded contrast proof > the URL field selection is visible against its own field: clears 1.5:1 in dark mode
AssertionError: primary-tint vs fill (dark): expected 1.0209053336174227 to be greater than 1.5
 ❯ src/ui/styles.test.ts:2072:11

 Test Files  1 failed (1)
      Tests  2 failed | 190 skipped (192)
```

Both modes fail, and light is in fact marginally worse than dark
(1.016:1 vs 1.021:1) — the defect was observed and reported specifically in
dark mode, but the underlying measurement shows light mode was equally
broken; it simply was not what the owner happened to look at.

At that point in the change, adding the `['text', 'selection', 4.5, true]`
row to the `placed` table and renaming the existing selection-pair pin to
expect `--color-selection` also failed (both against the not-yet-declared
token), for a combined failing-first total of **5 failed / 187 passed** on
the full `styles.test.ts` run.

## The fix

### `--color-selection`, a dedicated, authored token

`frontend/src/style.css`, `@theme` (light) and the dark override block: a
new token, **not derived from `--color-primary-tint` by any formula** —
authored against both floors this selection specifically needs:

| Mode | Value |
|---|---|
| Light | `#D9A679` |
| Dark | `#7A5240` |

Derived figures (recomputed by `styles.test.ts`'s own `contrast()`, copied
verbatim from its output, never hand-computed):

| Pair | Light | Dark | Floor |
|---|---:|---:|---|
| `text` on `selection` | 8.011038123 | 6.042413035 | ≥4.5:1 (text) |
| `selection` on `fill` (the URL field's own background) | 1.921761995 | 1.917235399 | ≥1.5:1 (visibility) |

Both comfortably clear their floors in both modes — there is no
razor's-edge value here, unlike `--color-primary-tint`'s dark fraction
(12%, chosen specifically because 16% put `muted` at 4.49:1, just under
its own floor).

### `.fd-url::selection` points at the new token

```css
.fd-url::selection {
    background: var(--color-selection);
    color: var(--color-text);
}
```

### Forced colors

`--color-selection: Highlight;` is added to the forced-colors `:root`
override, consistent with how `primary`/`primary-hover` map there. But
`.fd-url::selection`'s `color` reads `--color-text`, which resolves to
`CanvasText` everywhere else in forced colors (it feeds body copy, headings,
labels) — remapping that shared token to `HighlightText` just for this rule
would wrongly recolor everything else that reads it. Instead,
`.fd-url::selection` itself is restated directly in the forced-colors block,
the same pattern `.fd-button--primary` already uses there:

```css
.fd-url::selection {
    background: Highlight;
    color: HighlightText;
}
```

This delivers the real system selection pair (`Highlight`/`HighlightText`)
rather than `Highlight` on `CanvasText`.

### A collateral fix while writing the defect-fix comment

Adding the defect narrative directly above `.fd-url::selection` in
`style.css` (outside the `@theme`/dark/forced-colors blocks that
`styles.test.ts` subtracts before checking for literal colours) initially
included the two hex values being compared (`#372E2B`, `#313135`) in prose.
`styles.test.ts`'s "reaches every component value through a token rather
than a literal" test caught this immediately — a CSS comment in the
component region is not exempt from that scan — and the comment was
rewritten to describe the same fact by reference to the tokens' own
declarations instead of restating their hex values. No functional change;
noted because it is exactly the kind of test-catches-the-fix's-own-evidence
case AGENTS.md's testing standards describe.

## Mutation: point the selection back at `--color-primary-tint`

```
$ sed -i '/^\.fd-url::selection {$/,/^}$/ s/background: var(--color-selection);/background: var(--color-primary-tint);/' src/style.css
$ npx vitest run src/ui/styles.test.ts
     × the URL field selection is visible against its own field: clears 1.5:1 in light mode
     × the URL field selection is visible against its own field: clears 1.5:1 in dark mode
     × paints the URL field selection with the published text on selection pair

 FAIL  src/ui/styles.test.ts > the unrounded contrast proof > the URL field selection is visible against its own field: clears 1.5:1 in dark mode
AssertionError: primary-tint vs fill (dark): expected 1.0209053336174227 to be greater than 1.5
 ❯ src/ui/styles.test.ts:2097:71

 Test Files  1 failed (1)
      Tests  3 failed | 190 passed (193)
```

The dark-scheme case fails and names exactly the mutated pair
(`primary-tint vs fill (dark)`), as required. (The light-scheme case fails
too, for the same underlying reason — both directions of this mutation are
caught, not only the one the defect report happened to observe in.) The
mutation was reverted immediately after this check and the suite reconfirmed
green (193/193) before proceeding.

## DESIGN.md

- Front matter `colors:` block: added `selection: '#D9A679'` and
  `selection-dark: '#7A5240'`.
- Colors table: new **Selection** row explaining the defect, the two floors,
  and the forced-colors treatment.
- Text pair table: new `text` on `selection` row (8.011038123 /
  6.042413035), required and checked automatically by `styles.test.ts`'s
  "publishes every figure it proves, unrounded, in DESIGN.md" test now that
  `['text', 'selection', 4.5, true]` is in the `placed` table.
- New narrative paragraph plus a small "Visibility pair" table publishing
  `selection` on `fill` (1.921761995 / 1.917235399) — required by a
  dedicated assertion in the new visibility test (`designSpine` must
  contain each unrounded ratio).
- Direct URL Row component row (Components section) updated to name the
  defect and point at the new token instead of describing the reuse of
  `primary-tint` as settled.

## Full gate, run in the order given, from the worktree root

Worktree: `/Users/jaesonmartin/Projects/FairDrop/.claude/worktrees/agent-a6c2f80916a109e18`
(branch `fix-url-selection-visibility`, forked from
`origin/epic-10-polish-after-1-3-0`).

1. `wails build` — succeeded: `Built '.../fairdrop.app/Contents/MacOS/fairdrop' in 14.282s.`
   Regenerated `frontend/wailsjs/`.
2. `git checkout -- frontend/wailsjs` — reverted the regenerated bindings to
   the committed copy (no content diff expected or found).
3. `gofmt -l .` — no output.
4. `go vet ./...` — no output.
5. `go tool staticcheck ./...` — no output.
6. `go test -count=1 ./...` — all packages `ok`: `fairdrop`,
   `fairdrop/internal/network`, `fairdrop/internal/qr`,
   `fairdrop/internal/server`, `fairdrop/internal/source`,
   `fairdrop/internal/stream`, `fairdrop/internal/transfer`,
   `fairdrop/scripts`, `fairdrop/scripts/mutationverdict`.
7. `cd frontend && npm test` — **829/829 pass**, 20 test files.
8. `npm run test:browser` — **81/81 pass**, 2 test files.
9. `git checkout -- frontend/browser/captures/` — the capture PNG was not
   regenerated with a diff on this run (`git status` showed nothing under
   `frontend/browser/captures/` before this step); ran anyway per the gate.

`git status` after the full gate shows exactly three files changed:
`_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/DESIGN.md`,
`frontend/src/style.css`, `frontend/src/ui/styles.test.ts`.

## Native re-check

Pending — orchestrator. This fix is proved by the recomputed contrast proof
(`styles.test.ts`) against the exact tokens `style.css` declares, mirroring
how Story 10.1's own (incomplete) proof was structured, but the original
defect was reported from the built binary and this fix has not yet been
re-observed there.

## Files changed

- `frontend/src/style.css` — the fix: `--color-selection` (light + dark),
  `.fd-url::selection` pointed at it, forced-colors token mapping plus the
  dedicated `.fd-url::selection` Highlight/HighlightText override.
- `frontend/src/ui/styles.test.ts` — the failing-test-first evidence: the
  visibility test, the `placed` table addition, the renamed selection-pair
  pin, the light/dark token pins, and the forced-colors pair test.
- `_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/DESIGN.md` —
  publishes both new figures and documents the defect and the fix.
