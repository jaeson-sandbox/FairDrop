# Evidence: Story 7.3: Rebuild Idle

## Scope

`frontend/src/ui/IdleView.tsx`, `frontend/src/ui/RecoveryHelp.tsx`, `frontend/src/ui/Disclosure.tsx`
(new), `frontend/src/ui/copy.ts`, `frontend/src/style.css`, and their tests
(`IdleView.test.tsx`, `styles.test.ts`, `copy.test.ts`). `BrowseControl`'s keyboard handling in
`IdleView.tsx` was not touched -- only its trigger's `className` (adding `fd-button--primary` and a
decorative trailing chevron span) and nothing in `handleTriggerKeyDown`, `handleMenuKeyDown`,
`handleMenuBlur`, or `closeAndReturnFocus`. `TransferringView.tsx`, `OutcomePanel.tsx` and the
progress/outcome CSS (Story 7.5, concurrent, separate worktree) were not touched; confirmed by an
empty `git diff --stat` against both files and by every `style.css` hunk falling inside the
Idle-region lines (drop zone, disclosures, browse trigger, `fd-help` scoping) rather than the
progress/outcome sections further down the file.

## What changed and why

- **Drop zone**: restructured into an outer `.fd-drop-zone` card (`{rounded.xxl}`, `{elevation.sh-2}`,
  7px padding, no border of its own) and an inner `.fd-drop-zone__inner` carrying the 2px dashed
  rule at `{rounded.xl}` (18px = 24px − 7px inset, concentric per DESIGN.md's Shapes section). Rest
  state reads `--color-separator` (decorative: the zone identifies nothing operable, carrying no
  click handler and no tab stop); drag-active (`.wails-drop-target-active`, added by Wails to the
  gated outer element) makes the inner rule solid `--color-primary`, tints the fill
  `--color-primary-tint`, and lifts `.fd-drop-symbol` by `translateY(-3px)` -- three cues, never fill
  alone. `--wails-drop-target: drop` stays an inherited inline style on the same outer element; no
  DOM drop handler was added.
- **Firewall preflight** (FR23 amendment): moved from an always-open `<aside>` into a `Disclosure`
  (new `Disclosure.tsx`, built on native `<details>`/`<summary>` for keyboard operability with no
  hand-rolled ARIA), collapsed by default (`FIREWALL_DISCLOSURE_DEFAULT_OPEN = false` in
  `IdleView.tsx`), summary text `copy.label.firewallHeading` ("Local network access"). The fallback
  DESIGN.md records ("ships `open` by default") is the single named constant; mutated to `true` and
  confirmed (below) that nothing else needs to change.
- **Recovery guidance**: `RecoveryHelp.tsx` split into `RecoveryHelpContent` (the four strings, no
  wrapper) and `RecoveryHelp` (the always-open `<div className="fd-help">` wrapper, unchanged, still
  used verbatim by `StagedView.tsx`). `IdleView.tsx` wraps `RecoveryHelpContent` in a second
  `Disclosure` (`className="fd-help"`, summary `copy.label.recoveryHeading`, a new label added to
  `copy.ts`). This keeps Staged's recovery block exactly as it was -- a plain, always-open div -- and
  avoids two elements both carrying the `fd-help` class in Idle's DOM, which would have broken the
  existing document-order assertion.
- **Browse control**: trigger button gained `fd-button--primary` and a decorative trailing chevron
  span. It was already full-width via the pre-existing `.fd-selection > .fd-button { width: 100% }`
  rule.
- **`fd-help` CSS split**: `.fd-help:not(.fd-disclosure)` now carries the old plain-card styling
  (Staged's form); `.fd-disclosure` carries the new look (Idle's form, shared with the preflight
  disclosure).
- **`styles.test.ts`**: `.fd-drop-zone` removed from the "decorative edge stays decorative" `controls`
  array, because its dashed inner rule is authored with `--color-separator` on purpose (the zone is
  not an operable control). Verified by mutation that reinstating it makes the test fail against the
  real stylesheet -- the removal was necessary, not a loosening.

## Existing tests modified, with justification

1. **`IdleView.test.tsx` -- `not.toContain('fd-button--primary')` inverted to `toContain(...)`.**
   Required by the story: Quartz reverses Paper Relay's "quieter than the drop zone" rule now that
   the drop zone carries no click handler and is not a control. Comment left in place names the
   reversal; the assertion was not deleted.
2. **`styles.test.ts` -- `.fd-drop-zone` removed from the `controls` array.** See above; a comment at
   the removal site explains why, and a new test in the same file
   (`.fd-drop-zone draws its boundary with the functional token, not the decorative one`, mutated
   back in) proves the removal is load-bearing rather than convenient.
3. **`copy.test.ts` -- added `'label.recoveryHeading'` to the exact registered-key-path list.** A new
   key was added to `copy.ts` (the second disclosure's summary needs a name, the same way
   `firewallHeading` names the first); this test enumerates every path, so a new key requires a new
   row. Not a loosening -- the enumeration is exact both before and after.

No other existing test's assertions were weakened, relaxed, or deleted.

## New tests added

- `IdleView.test.tsx`: drop-zone inner-wrapper structure; firewall disclosure collapsed by default
  (`<details>`, no `open` attribute) with a named, keyboard-operable `<summary>`; firewall disclosure
  still precedes the browse control; recovery disclosure collapsed by default with its own summary;
  four `it.each` rows, one per `RecoveryHelpContent` string, each naming its own string if dropped;
  recovery `dt` labels in document order; command-failure outcome panel ordered between the drop zone
  and the preflight disclosure; command-error focus target unchanged.
- `styles.test.ts`: drop zone radius/elevation/no-self-boundary; concentric inset/radius pairing;
  drag-active solid boundary + tint + glyph lift; the `.fd-disclosure` family (radius, elevation,
  hover fill, chevron rotation); the `fd-help` / `fd-disclosure` CSS split.

## Mutation table

Every row: mutation applied to the working tree, targeted suite run, failure observed and named,
mutation reverted, suite re-run green. All executed against this tree after the mid-run interruption
noted below -- nothing here is carried over from memory.

| # | Claim | Mutation | Result |
|---|---|---|---|
| 1 | Browse control is `fd-button--primary` (inverted assertion) | Drop `fd-button--primary` from trigger `className` | `IdleView.test.tsx > Idle at rest > offers one control...` fails, naming the missing class |
| 2 | Every `RecoveryHelpContent` string still renders | Delete the `copy.help.receiverHttp` paragraph | Two tests fail: the `it.each` row for that exact string, and the pre-existing "offers platform firewall recovery and receiver help from Idle" test |
| 3 | Firewall disclosure collapsed by default | Set `defaultOpen={true}` unconditionally | `renders as a <details> that is present but not open on first paint` fails (`hasAttribute('open')` true vs expected false) |
| 4 | Concentric inner radius is `{rounded.xl}` | Change `.fd-drop-zone__inner`'s `border-radius` to `var(--radius-lg)` | `insets the dashed inner rule 7px so its radius is concentric...` fails; `.fd-disclosure`'s own radius test also independently fails when the same `var(--radius-xl)` string is targeted broadly, confirming both are pinned separately |
| 5 | Drop zone rest boundary is `--color-separator`, not `--color-control-border` | Change `.fd-drop-zone__inner`'s border color token | Same test as #4's sibling assertion fails, naming the missing `--color-separator` string |
| 6 | Drop zone card is `{rounded.xxl}` at `{elevation.sh-2}` with no border of its own | Remove `box-shadow: var(--shadow-sh-2)` from `.fd-drop-zone` | `gives the drop zone the xxl radius and sh-2 elevation...` fails |
| 7 | Drag-active: solid primary boundary | Drop `border-color: var(--color-primary)` from the drag-active inner rule | `goes solid primary with a tinted fill on drag-active...` fails |
| 8 | Drag-active: glyph lifts | Empty the `.fd-drop-zone.wails-drop-target-active .fd-drop-symbol` rule | Same test fails on the `transform: translateY(-3px)` assertion |
| 9 | `.fd-drop-zone` correctly excluded from the decorative-edge `controls` list | Reinstate `.fd-drop-zone` in that array | `.fd-drop-zone draws its boundary with the functional token, not the decorative one` fails against the real stylesheet -- proves the exclusion was necessary |
| 10 | Command-failure outcome panel renders between the drop zone and the preflight | Move the `OutcomePanel` block to after `BrowseControl` in JSX | `renders the outcome panel between the drop zone and the preflight disclosure` fails |
| 11 | Cancel-winning summary still leads the region | Move the cancel-summary block to after the drop zone in JSX | Pre-existing `leads the Idle region rather than following the controls` fails |

All eleven mutations were applied, run, confirmed failing with the named test, and reverted; the
targeted suite (`styles.test.ts` and/or `IdleView.test.tsx`) was re-run green after each revert, and
the full frontend suite was re-run green after the batch.

## Gate transcript (this working tree, `epic-7-quartz`, macOS arm64)

```
$ cd frontend && npm test -- --run
 Test Files  17 passed (17)
      Tests  584 passed (584)

$ npm run test:browser
 Test Files  1 passed (1)
      Tests  17 passed (17)

$ cd .. && wails build
  • Generating bindings: Done.
  • Installing frontend dependencies: Done.
  • Compiling frontend: Done.
  • Compiling application: Done.
  • Packaging application: Done.
  • Self-signing application: Done.
Built '/Users/jaesonmartin/Projects/FairDrop/build/bin/fairdrop.app/Contents/MacOS/fairdrop' in 11.88s.
# frontend/wailsjs/go/main/App.d.ts, App.js and go/models.ts flipped to 755 by the build;
# chmod 644 applied, content diff empty (mode-only drift, per the known pitfall).

$ gofmt -l .
(none)

$ go vet ./...
(clean)

$ go tool staticcheck ./...
(clean)

$ go test -count=1 ./...
ok  	fairdrop	1.703s
ok  	fairdrop/internal/network	0.232s
ok  	fairdrop/internal/qr	0.653s
ok  	fairdrop/internal/server	4.967s
ok  	fairdrop/internal/source	1.054s
ok  	fairdrop/internal/stream	4.262s
ok  	fairdrop/internal/transfer	2.015s
ok  	fairdrop/scripts	1.187s
ok  	fairdrop/scripts/mutationverdict	1.555s

$ go env CGO_ENABLED
1
$ CGO_ENABLED=1 go test -count=1 -race ./...
ok  	fairdrop	10.167s
ok  	fairdrop/internal/network	2.091s
ok  	fairdrop/internal/qr	2.407s
ok  	fairdrop/internal/server	5.674s
ok  	fairdrop/internal/source	2.405s
ok  	fairdrop/internal/stream	110.855s
ok  	fairdrop/internal/transfer	2.790s
ok  	fairdrop/scripts	2.813s
ok  	fairdrop/scripts/mutationverdict	2.623s

$ GOOS=windows GOARCH=amd64 go build ./...   -> exit 0
$ GOOS=darwin GOARCH=arm64 go build ./...    -> exit 0
$ GOOS=linux GOARCH=amd64 go build ./...     -> exit 0

# Frontend suites re-run after wails build (never concurrently), confirming the
# regenerated frontend/dist and reinstalled node_modules didn't change results:
$ cd frontend && npm test -- --run
 Test Files  17 passed (17)
      Tests  584 passed (584)
$ npm run test:browser
 Test Files  1 passed (1)
      Tests  17 passed (17)
```

`App.focus.test.tsx` and `StagedView.test.tsx` were also run in isolation and pass unchanged (44
tests, 2 files) -- confirming the FR23 disclosure change didn't disturb App's focus-routing suite or
Staged's still-plain `RecoveryHelp` usage.

## Session note

This story's implementation was interrupted mid-run by an API rate limit after the initial mutation
pass and the first full frontend-suite green. On resume the working tree was re-inspected rather than
trusted from memory (`git status`/`git diff --stat` against `frontend/src/ui/TransferringView.tsx`
and `OutcomePanel.tsx` confirmed empty, ruling out any accidental edit to Story 7.5's files), the
frontend suite was re-run green before touching anything further, and every mutation in the table
above was re-executed against the current tree rather than assumed from the earlier, since-lost run.

## Open questions / things the spec left underspecified

- DESIGN.md's Disclosure row and the FR23 note do not name the second (recovery) disclosure's summary
  text. `copy.label.recoveryHeading = 'Recovery help'` was added as a structural label, consistent
  with how `firewallHeading` names the first disclosure, but it is not a spine-approved string the
  way the paragraphs under it are -- worth a spine pass to promote it to approved copy if the actual
  wording matters to the owner.
- DESIGN.md's Drop Zone row gives the dashed inner rule `{colors.separator}` at rest but says "The
  rest-state boundary uses `{colors.control-border}` in forced colors" without saying *how* that
  happens. It turns out both tokens already resolve to `CanvasText` under `forced-colors: active`, so
  no forced-colors-specific override was needed -- but this is an emergent property of the existing
  token table, not something DESIGN.md states directly, and it would silently stop being true if a
  future forced-colors edit gave `--color-separator` and `--color-control-border` different system
  colors. Flagged in a code comment at the CSS rule; worth a spine note.
