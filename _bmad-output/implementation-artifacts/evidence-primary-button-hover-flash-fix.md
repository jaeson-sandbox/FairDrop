# Evidence: primary button hover black-flash defect fix

Defect-fix carve-out (AGENTS.md "Git workflow"): observed defect, failing test written
first, fix mutation-verified. Branch `epic-7-quartz`, no story.

## The defect

Owner-observed on the built binary: hovering "Choose a file or folder" (the primary
button) produced a black flash.

## Diagnosis (confirmed against the stylesheet, unchanged from the brief)

`.fd-button` transitions `background-color` over 150ms. `.fd-button--primary` painted
its fill through the `background` SHORTHAND:

```css
.fd-button--primary {
    background: linear-gradient(var(--color-primary-hi), var(--color-primary));
}
.fd-button--primary:hover {
    background: var(--color-primary-hover);
}
```

The shorthand's side effect is to reset whichever longhand it does not name. The rest
rule reset `background-color` to its initial value, `transparent`; the hover rule reset
`background-image` to `none`. `background-image` cannot interpolate (gradients do not
transition), so it jumped instantly on hover, while `background-color` spent the full
150ms transitioning FROM the shorthand's `transparent` reset — a see-through button for
that window, with the dark canvas showing through it. Un-hovering reversed the same
mechanism. This was the only gradient/flat-colour pair in the sheet; every other
`background:` shorthand pairs flat colour with flat colour, where the shorthand reset is
harmless (nothing was ever `transparent`), so the fix is scoped to this one rule pair.

The diagnosis in the task brief was correct in every respect; no correction needed.

## Failing test, written first, captured before any fix

Two new tests were added to `frontend/src/ui/styles.test.ts` inside a new describe
block, `'the primary button hover has no black flash (defect fix)'`, run against the
untouched tree:

```
 RUN  v4.1.11 /Users/jaesonmartin/Projects/FairDrop/frontend

 ❯ src/ui/styles.test.ts (136 tests | 2 failed | 134 skipped) 5ms
     × never lets .fd-button--primary or its :hover use the background shorthand 4ms
     × keeps both background-color values opaque and the sheen background-image identical across :hover 0ms

⎯⎯⎯⎯⎯⎯⎯ Failed Tests 2 ⎯⎯⎯⎯⎯⎯⎯

 FAIL  src/ui/styles.test.ts > the primary button hover has no black flash (defect fix) > never lets .fd-button--primary or its :hover use the background shorthand
AssertionError: .fd-button--primary: expected '.fd-button--primary {\n    border-col…' not to match /\bbackground:\s/

- Expected:
/\bbackground:\s/

+ Received:
".fd-button--primary {
    border-color: var(--color-primary);
    background: linear-gradient(var(--color-primary-hi), var(--color-primary));
    box-shadow: var(--shadow-sh-1), inset 0 1px 0 rgb(255 255 255 / 0.4);
    color: var(--color-primary-ink);"

 ❯ src/ui/styles.test.ts:867:36
    865|         // `background-image:` are fine; `background:` is not.
    866|         for (const [name, rule] of [['.fd-button--primary', rest], ['.…
    867|             expect(rule, name).not.toMatch(/\bbackground:\s/)
       |                                    ^
    868|         }
    869|     })

⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯[1/2]⎯

 FAIL  src/ui/styles.test.ts > the primary button hover has no black flash (defect fix) > keeps both background-color values opaque and the sheen background-image identical across :hover
AssertionError: rest background-color: expected undefined to be truthy

- Expected:
true

+ Received:
undefined

 ❯ src/ui/styles.test.ts:877:52
    875|         const restColor = rest.match(/background-color:\s*([^;]+);/)?.…
    876|         const hoverColor = hover.match(/background-color:\s*([^;]+);/)…
    877|         expect(restColor, 'rest background-color').toBeTruthy()
       |                                                    ^
    878|         expect(hoverColor, 'hover background-color').toBeTruthy()
    879|

⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯⎯[2/2]⎯

 Test Files  1 failed (1)
      Tests  2 failed | 134 skipped (136)
   Start at  23:27:08
   Duration  625ms (transform 45ms, setup 0ms, import 58ms, tests 5ms, environment 468ms)
```

Both failures name the mechanism directly: the shorthand's presence, and the absence of
a `background-color:` longhand on the rule that is supposed to declare one.

## The fix

`frontend/src/style.css`, `.fd-button--primary` and `.fd-button--primary:hover`:

```css
.fd-button--primary {
    border-color: var(--color-primary);
    background-color: var(--color-primary);
    background-image: linear-gradient(rgb(255 255 255 / 0.08), rgb(255 255 255 / 0));
    box-shadow: var(--shadow-sh-1), inset 0 1px 0 rgb(255 255 255 / 0.4);
    color: var(--color-primary-ink);
}

.fd-button--primary:hover {
    background-color: var(--color-primary-hover);
    background-image: linear-gradient(rgb(255 255 255 / 0.08), rgb(255 255 255 / 0));
    border-color: var(--color-primary-hover);
}
```

- Both `background-color:` values are opaque tokens (`--color-primary`,
  `--color-primary-hover`) — neither state ever paints `transparent`, so
  `background-color` never has a see-through starting point to transition from.
- `background-image` is declared as a fixed translucent-white sheen, **byte-identical**
  in both rules, so it never itself changes on hover — only the opaque `background-color`
  beneath it animates.
- The sheen is independent of the fill colour underneath it (unlike the retired
  colour-stop gradient), which is exactly what lets it stay identical across states
  regardless of which fill token is active.
- `--color-primary-hi` fed only the retired colour-stop gradient and the (already-fixed,
  Story 7.7) former hover rule. With the gradient gone, nothing reads it, so it is
  **removed** from `@theme`, the dark override, and the forced-colors override in
  `frontend/src/style.css`, and from DESIGN.md's frontmatter and the Action row (see
  "DESIGN.md changes" below). "Do not leave a token declared that nothing reads" — the
  brief's instruction — is satisfied by removal, not by a kept-and-justified note.

Forced colors is unaffected: `.fd-button--primary { background: Highlight; box-shadow:
none; }` inside `@media (forced-colors: active)` still uses the shorthand deliberately —
that rule is never composed with a transitioning `background-color`, since forced colors
strips the whole authored palette (including the transition's own colours) to system
colours, so the shorthand-reset hazard does not apply there. Untouched, per the brief's
constraint that forced colors keep working and the focus-ring `outline` fallback stay.

### Single-gradient rule

The product still ships exactly one gradient — the sheen — declared identically in two
places (rest and `:hover`) rather than two different gradients. `styles.test.ts`'s
`'pins the three elevation tokens and the single-gradient rule'` test was updated: it
now expects 2 raw `linear-gradient(` occurrences (one visual gradient, written twice by
mechanical necessity) and pins both occurrences to the exact same text,
`linear-gradient(rgb(255 255 255 / 0.08), rgb(255 255 255 / 0))`, in both the rest and
hover rule blocks. The `repeating-linear-gradient` exemption for the unknown-total meter
is untouched.

### Contrast: the sheen is a text background

Per the brief: the sheen's alpha is not a taste call. The button label
(`--color-primary-ink`) sits at the top of the button, where the sheen is strongest, so
the sheen is a text background like any other — the third time this epic found one that
had gone unmeasured (after the hover fill, Story 7.7, and `--color-primary-tint`, Story
7.8).

Re-derived directly from the stylesheet in a new test,
`'measures the sheen as a text background...'` (`frontend/src/ui/styles.test.ts`, inside
`'the unrounded contrast proof'`): it extracts the sheen's alpha from the actual
`.fd-button--primary` `background-image` declaration (not a hand-typed constant),
composites it (rounding each channel the way a real compositor renders pixels) over
each of the four fill combinations (rest/hover × light/dark), and asserts every one
clears 4.5:1.

| sheen alpha | light rest label ratio | light hover | dark rest | dark hover | verdict |
|---|---:|---:|---:|---:|---|
| 0.06 | 4.897280271 | — | — | — | OK |
| **0.08** | **4.677257536** | 6.243877525 | 8.269767363 | — | **OK — chosen** |
| 0.10 | 4.506070489 | — | — | — | too close to the 4.5 floor |
| 0.12 | 4.341900684 | — | — | — | **FAILS** |

(Only the chosen alpha's full four-way breakdown was computed; the alpha sweep at the
other three points is the light-rest case only, which the test proves is the weakest of
the four in every case it checks.) These figures were re-derived independently from the
task brief's own table and match it exactly at every alpha listed, confirming the brief's
numbers rather than copying them — the test computes them fresh from
`--color-primary` / `--color-primary-hover` and the sheen's own declared alpha.

Light rest, **4.677257536**, is the worst case among all four rest/hover × light/dark
combinations and is the figure published. It was added to DESIGN.md's text-pair contrast
table as `primary-ink on primary button sheen (rest) | 4.677257536 | 8.269767363`
(dark rest given alongside for completeness, though it is not the worst case), with an
accompanying finding paragraph in the Colors section explaining why it belongs there —
matching the treatment of the two earlier findings (hover fill, `primary-tint`) already
in the document.

## Mutation verification

**Mutation 1 — restore the shorthand pair exactly as it was:**

```css
.fd-button--primary {
    border-color: var(--color-primary);
    background: linear-gradient(var(--color-primary-hi), var(--color-primary));
    box-shadow: var(--shadow-sh-1), inset 0 1px 0 rgb(255 255 255 / 0.4);
    color: var(--color-primary-ink);
}
.fd-button--primary:hover {
    background: var(--color-primary-hover);
    border-color: var(--color-primary-hover);
}
```

Result: 6 tests fail, including both of the new failing-first tests. The named failure:

```
FAIL  src/ui/styles.test.ts > the primary button hover has no black flash (defect fix) > never lets .fd-button--primary or its :hover use the background shorthand
AssertionError: .fd-button--primary: expected '.fd-button--primary {\n    border-col…' not to match /\bbackground:\s/
```

— names the exact rule and the exact mechanism (the shorthand's reappearance). Reverted;
full suite back to 138/138 green.

**Mutation 2 — raise the sheen alpha to 0.12 (both rest and hover, keeping them in
sync so the mutation isolates the alpha, not the shorthand bug):**

```css
background-image: linear-gradient(rgb(255 255 255 / 0.12), rgb(255 255 255 / 0));
```

Result: the new sheen-contrast test fails. With its own `alpha === 0.08` pin left in
place it fails on that pin first; with the pin temporarily removed to confirm the
*contrast* assertion is independently load-bearing (not merely the pin), it fails here
instead, naming the light-rest pair exactly as required:

```
FAIL  src/ui/styles.test.ts > the unrounded contrast proof > measures the sheen as a text background -- the label sits at the top of the button, where it is strongest
AssertionError: light rest: expected 4.341900683942201 to be greater than 4.5
```

Reverted; full suite back to 138/138 green. `git status --short` confirmed only the
three intended files (`frontend/src/style.css`, `frontend/src/ui/styles.test.ts`,
DESIGN.md) differ from HEAD after both mutations were reverted.

## `--color-primary-hi` disposition

Removed. It fed only the retired colour-stop gradient (and, briefly and incorrectly
through Story 7.7, the hover fill — already fixed by `--color-primary-hover` before this
defect). With the sheen replacing the gradient, nothing reads it. Removed from:

- `frontend/src/style.css`: `@theme` (light), the dark `@media` override, and the
  `@media (forced-colors: active)` override.
- DESIGN.md: frontmatter (`primary-hi` / `primary-hi-dark` entries), the Action row, and
  the Elevation & Depth "exactly one gradient" bullet (rewritten to describe the sheen
  and narrate the removal in place of the superseded Story 7.7 narrowing note).

A new test, `'declares no --color-primary-hi any more...'`, pins the token's absence
from `@theme`, the dark override, and the forced-colors override, and that nothing in
`componentRules` still reads it via `var()`. The former "narrows the dark primary-hi
delta" test (Story 7.7) was rewritten to assert the retirement instead of a since-moot
narrowing, and checks DESIGN.md's frontmatter specifically (not its prose, which is free
to keep narrating the token's history, and does).

## Full gate, run in `verify.yml` order

```
$ wails build
...
Built '/Users/jaesonmartin/Projects/FairDrop/build/bin/fairdrop.app/Contents/MacOS/fairdrop' in 7.786s.
```

`frontend/wailsjs/go/main/App.d.ts`, `App.js`, and `models.ts` had their mode flipped to
755 by the build (content unchanged — `git diff` showed only `old mode 100644` /
`new mode 100755`); `chmod 644` restored them per the known pitfall.

```
$ gofmt -l .
(no output)

$ go vet ./...
(no output)

$ go tool staticcheck ./...
(no output)

$ go test -count=1 ./...
ok  	fairdrop	1.394s
ok  	fairdrop/internal/network	0.956s
ok  	fairdrop/internal/qr	0.618s
ok  	fairdrop/internal/server	4.913s
ok  	fairdrop/internal/source	0.464s
ok  	fairdrop/internal/stream	4.096s
ok  	fairdrop/internal/transfer	1.741s
ok  	fairdrop/scripts	1.111s
ok  	fairdrop/scripts/mutationverdict	1.606s

$ go env CGO_ENABLED
1

$ CGO_ENABLED=1 go test -count=1 -race ./...
ok  	fairdrop	8.673s
ok  	fairdrop/internal/network	1.720s
ok  	fairdrop/internal/qr	2.884s
ok  	fairdrop/internal/server	6.378s
ok  	fairdrop/internal/source	1.401s
ok  	fairdrop/internal/stream	103.854s
ok  	fairdrop/internal/transfer	2.931s
ok  	fairdrop/scripts	2.768s
ok  	fairdrop/scripts/mutationverdict	1.494s

$ cd frontend && npm test
 Test Files  17 passed (17)
      Tests  658 passed (658)

$ npm run test:browser
 Test Files  2 passed (2)
      Tests  23 passed (23)
```

Both frontend counts are up from the stated baseline (654 → 658 jsdom; 23 unchanged
rendered/Chromium) by exactly the 4 new tests added by this fix (2 mechanism tests, 1
sheen-contrast test, 1 `--color-primary-hi` retirement test; the rewritten "narrows the
dark primary-hi delta" test replaces one existing test rather than adding one).

`frontend/browser/captures/qr-panel-forced-colors.capture.png` changed a byte after
`test:browser` (known pitfall — PNG encoding is not byte-deterministic); reverted with
`git checkout -- frontend/browser/captures/` rather than committed, since nothing in the
palette that capture proves changed.

```
$ GOOS=windows GOARCH=amd64 go build ./...
(exit 0)

$ GOOS=darwin GOARCH=arm64 go build ./...
(exit 0)

$ GOOS=linux GOARCH=amd64 go build ./...
(exit 0)
```

`git status --short` after the full gate: only `_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/DESIGN.md`,
`frontend/src/style.css`, and `frontend/src/ui/styles.test.ts` differ from HEAD.

## Files changed

- `frontend/src/style.css` — `.fd-button--primary` / `:hover` longhand fix, sheen
  gradient, `--color-primary-hi` removed (light theme, dark override, forced-colors
  override), updated comments narrating the defect and its fix.
- `frontend/src/ui/styles.test.ts` — new failing-first describe block (2 tests), new
  sheen-contrast test, `--color-primary-hi` retirement test, updated single-gradient
  pin, updated token-literal exemption, updated hover-fill-token test's regex
  (`background:` → `background-color:`) and mutation comments, rewritten
  "narrows primary-hi" test.
- `_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/DESIGN.md` —
  frontmatter, Action row, text-pair contrast table (new sheen row + finding paragraph),
  Elevation & Depth's single-gradient bullet rewritten to describe the sheen and the
  defect fix, `primary-hi` / `primary-hi-dark` removed from the token list.
