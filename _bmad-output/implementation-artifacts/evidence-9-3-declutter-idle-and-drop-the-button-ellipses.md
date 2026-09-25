# Evidence: Story 9.3: Declutter Idle and Drop the Button Ellipses

## Summary

Idle's composition changed from "drop target, then a full-width browse
control below it, then two separately carded disclosures" to "drop target
containing the browse control as a centred intrinsic-width pill, then one
grouped disclosure list" -- matching the owner-approved prototype's
`t-idle` template. `BrowseControl` was extracted from `IdleView.tsx` into
its own module with a `label` prop (sprint-status action item
`epic-4-retro-item-28`, now closed). Six copy strings changed exactly as
the acceptance criteria specify, including dropping the ellipsis from the
two cancel-pending labels, and a new global test proves no `<button>` or
`role="menuitem"` anywhere in the product ends in `…`/`...`. A defect the
orchestrator found on the built macOS binary -- the disclosure chevron
drifting to sit right after the label text instead of at the row's
trailing edge -- is fixed in the same pass, with a rendered-Chromium
geometry test.

## What changed

**Copy (`copy.ts` + `EXPERIENCE.md`'s Voice and Tone table, amended in the
same commit per the epic's rule):**

- `copy.idle.instruction`: `'Drop one file or folder.'` -> `'Drop one file
  or folder'` (dropped the full stop).
- New `copy.idle.promise`: `'Sends to one browser on the same local
  network. No account or receiver app.'`, rendered inside the drop zone in
  place of `copy.external.promise`. `copy.external.promise` keeps its
  longer wording, unchanged, for external use (README/store copy); it is
  simply no longer rendered anywhere in the UI after this story.
- `copy.label.chooseFileOrFolder`: `'Choose a file or folder'` -> `'Choose
  File or Folder'`.
- `copy.label.recoveryHeading`: `'Recovery help'` -> `'Troubleshooting'`
  (a `label` entry, not tabulated in EXPERIENCE.md by the registry's own
  convention -- same as `firewallHeading`).
- `copy.cancel.pending`: `'Canceling…'` -> `'Canceling'`.
- `copy.cancel.preparationPending`: `'Canceling preparation…'` ->
  `'Canceling preparation'`.

`copy.test.ts` extended: literal assertions for the six changed/new
strings, `idle.promise` added to the exact key-path list, `recoveryHeading`
added to the functional-labels assertions (previously untested by name).
`EXPERIENCE.md` rows for `copy.idle.instruction`, a new row for
`copy.idle.promise`, `copy.label.choose_file_or_folder`,
`copy.cancel.preparation_pending` and `copy.cancel.pending` all updated to
match; the spine-quotes-registry test (D-119) re-verified these agree.

**`BrowseControl` extraction (`frontend/src/ui/BrowseControl.tsx`, new):**

Every handler, ref and doc comment moved verbatim from `IdleView.tsx`.
The only functional change is a new `label: string` prop replacing the
hardcoded `copy.label.chooseFileOrFolder` read, so the same component can
carry a different trigger label later (Story 9.6's "Send Another"/"Choose
Another"). The two menu items now also carry a small decorative,
`aria-hidden` glyph ahead of "File"/"Folder" (new `FileGlyph`/`FolderGlyph`
functions), matching the prototype; their accessible names are unchanged
since the icons contribute no text node.

**`IdleView.tsx`:** the drop zone's inner wrapper now renders, in order,
the glyph, the `<h1>` instruction, the promise paragraph, and a wrapper
around `<BrowseControl>` -- each carrying `.fd-rise` with its own
`--fd-stagger` step (0-3), Story 9.1's per-child entrance. The two
`Disclosure`s are now wrapped in a `.fd-idle-disclosures` container
(`--fd-stagger` step 4) instead of rendering as two independent siblings.
`Disclosure.tsx` itself is untouched. Two doc comments describing the old
"browse control renders below the drop zone, ahead of both disclosures"
composition were rewritten to describe the new one.

**`frontend/src/style.css`:**

- New `.fd-button--pill` (`border-radius: var(--radius-full)`, wider
  inline padding) applied to the browse trigger.
- `.fd-selection > .fd-button { width: 100%; }` removed. The control now
  sits inside `.fd-drop-zone__inner`'s `place-items: center` grid, so it
  sizes to its own intrinsic content and centres itself with no new rule
  needed for that half.
- New `.fd-idle-disclosures` group wrapper (`{rounded.xl}` surface,
  `{elevation.sh-1}`, `overflow: hidden` so each row's rectangular hover
  fill clips to the group's rounded corners) with `.fd-idle-disclosures >
  .fd-disclosure` giving up its own radius/background/shadow to the
  wrapper, and a `{colors.separator}` `border-top` between the two rows.
- New `.fd-browse-menu-item__icon` (16x16, `flex: none`) and
  `justify-content: flex-start` plus a `gap` on `.fd-browse-menu .fd-button`
  for the new menu-item glyphs.
- **Defect fix, reported by the orchestrator from the built macOS
  binary of the epic branch:** `.fd-disclosure__summary` gained
  `justify-content: space-between`. Story 9.1 replaced `<summary>` (whose
  containing `<h2>` carried `flex: 1`, pushing the chevron to the row's
  trailing edge) with a `<button>` that never got an equivalent rule, so
  the chevron drifted to sit immediately after the label text. This one
  declaration is the fix, matching the prototype's `.row` rule exactly.

**`DESIGN.md`:** Shapes section amended -- `{rounded.full}` is no longer
"never a button", it now also covers a pill-shaped primary button, with
the amendment recorded rather than left contradicted. Browse Control,
Browse Menu and Disclosure rows in the Components table rewritten for the
new placement, the menu-item glyph, the grouped disclosure list, and the
chevron defect fix; a new paragraph in "FR23 and the disclosures" records
that Story 9.3 moves the control again without reopening the FR23
trade-off.

**Global no-button-ellipsis test:** `frontend/src/ui/noButtonEllipsis.test.tsx`
(new). Renders Idle (at rest, with the menu open, and with a command
failure), Stage Pending (both cancellation states), Staged (both
cancellation states), Transferring (both cancellation states), and the
Outcome Panel (live/retained Done and Error), and asserts no `<button>` or
`[role="menuitem"]`'s accessible name/text ends in `…` or `...`. Also
proves the assertion helper itself fails and names the offending control,
against a synthetic fixture, without needing to leave the real registry in
a broken state.

### Files touched

- `frontend/src/ui/copy.ts`, `frontend/src/ui/copy.test.ts`
- `_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/EXPERIENCE.md`
- `_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/DESIGN.md`
- `frontend/src/ui/BrowseControl.tsx` (new), `frontend/src/ui/BrowseControl.test.tsx` (new)
- `frontend/src/ui/IdleView.tsx`, `frontend/src/ui/IdleView.test.tsx`
- `frontend/src/ui/noButtonEllipsis.test.tsx` (new)
- `frontend/src/style.css`, `frontend/src/ui/styles.test.ts`
- `frontend/browser/accessibility.test.tsx` (new chevron-geometry test; two
  literal button-name updates)
- `frontend/src/App.tsx` (one comment, "Canceling…" -> "Canceling")
- `frontend/src/App.test.tsx`, `frontend/src/App.focus.test.tsx` (literal
  copy-string updates -- see "Deviation" below)
- `frontend/src/ui/StagePendingCard.test.tsx`, `frontend/src/ui/StagedView.test.tsx`,
  `frontend/src/ui/TransferringView.test.tsx`, `frontend/src/ui/announce.test.ts`
  (literal `'Canceling…'`/`'Canceling preparation…'` -> without the
  ellipsis, mechanical only)
- `_bmad-output/implementation-artifacts/sprint-status.yaml` (story ->
  `review`; `epic-4-retro-item-28` action item -> `done`)

## Deviation: `App.focus.test.tsx` is not byte-for-byte unchanged

The story text says both "`copy.label.chooseFileOrFolder` -> 'Choose File
or Folder'" (AC2) and "`App.focus.test.tsx` passes unchanged" (final AC).
These two requirements are in direct tension: `App.focus.test.tsx` mounts
the real `<App/>` (via `App.harness.tsx`) and locates elements by exactly
the literal strings that changed --
`screen.getByRole('button', {name: 'Choose a file or folder'})` and
`toBe('Drop one file or folder.')`. Once those strings are the copy that
changed, a test that finds an element *by* that copy cannot both keep
querying the old string and still find anything.

I treated the copy AC as binding (it is character-for-character specified
and quoted at the assertion site elsewhere in this suite) and updated the
two literal strings in `App.focus.test.tsx` to `'Choose File or Folder'`
and `'Drop one file or folder'`, with a comment at the first occurrence
explaining why and pointing back to this note. Nothing else in the file
changed -- no assertion was added, removed, loosened, or restructured; the
routing-table stub, the mock wiring, and every other assertion are
identical to before this story. I did not find a way to satisfy both
sentences literally and am flagging the disagreement rather than resolving
it silently, per the epic's own instruction for a story-vs-prototype
conflict; this is a story-vs-itself conflict but the same principle
applies.

## Mutation table

Each mutation was applied to the real working tree, confirmed to fail and
name the problem, then reverted -- confirmed clean afterward with `git
diff --stat` against the mutated file and a targeted `grep`. `git status
--porcelain` after the whole pass showed only the files intentionally
changed (18 total: 15 modified, 3 new).

| # | Mutation | AC | File | Result |
|---|---|---|---|---|
| M1 | Moved `<BrowseControl>` out of `.fd-drop-zone__inner`, rendering it as a sibling of `.fd-drop-zone` again | AC1 | `IdleView.tsx` | KILLED -- `IdleView.test.tsx`, "nests the browse control inside the drop zone rather than beside it (Story 9.3 reversal)": `expected false to be true` on `zone.contains(control)` |
| M2 | Replaced the `.fd-idle-disclosures` wrapper `<div>` with a React fragment (`<>...</>`) | AC3 | `IdleView.tsx` | KILLED -- 3 failures in `IdleView.test.tsx`: "wraps both disclosures in one .fd-idle-disclosures surface" (`expected null to be truthy`), "keeps the firewall row before the troubleshooting row inside the group" (`Cannot read properties of null`), and "renders the outcome panel between the drop zone and the disclosure group" (order array missing `fd-idle-disclosures`) |
| M3 | Restored `.fd-selection > .fd-button { width: 100%; }` in `style.css` | AC1 | `style.css` | KILLED -- `styles.test.ts`, "no longer stretches the browse control to the full width of its row (Story 9.3 reversal)": `expected [stylesheet] not to contain '.fd-selection > .fd-button'` |
| M4 | Removed `<FileGlyph/>`/`<FolderGlyph/>` from both menu items | AC5 | `BrowseControl.tsx` | KILLED -- `BrowseControl.test.tsx`, "gives each item a decorative, aria-hidden icon ahead of its word": `expected null to be truthy` |
| M5 | Restored `copy.cancel.pending` to `'Canceling…'` | AC6 | `copy.ts` | KILLED -- 2 failures in `noButtonEllipsis.test.tsx` ("Staged, cancellation requested", "Transferring, cancellation requested"), each naming the exact string: `Staged, cancelling: "Canceling…" ends in an ellipsis: expected true to be false` |
| M6 | Removed `justify-content: space-between;` from `.fd-disclosure__summary` | orchestrator-reported defect | `style.css` | KILLED -- `accessibility.test.tsx` (real Chromium), "keeps the chevron within the row's own right padding of the summary's right edge": measured gap `537.4px` on a 720px-wide row, naming both figures, against the fixed rule's `13.5px` |

M1 and M2 prove the two structural reversals AC1/AC3 name (the control
moving inside the zone; the two disclosures becoming one group) are
load-bearing, not merely present by accident of how the JSX happens to
nest. M3 proves the CSS half of AC1's reversal -- that the intrinsic-width
behaviour is not an artifact of a rule nobody removed. M4 proves the
menu-item glyph AC. M5 re-confirms the no-ellipsis rule against the *real*
registry (not only the synthetic fixture proof already inside
`noButtonEllipsis.test.tsx` itself), which is the assertion two of the
six exact-copy changes in AC2 depend on. M6 is the orchestrator-reported
defect's own proof, in real layout, not merely that the stylesheet text
contains the declaration.

Two further pre-existing guarantees this story did not weaken were
re-confirmed rather than freshly mutated, since their own mutations
predate this story and remain intact: "keeps every string RecoveryHelpContent
and the firewall preflight carry in the DOM regardless of open state" (the
recovery/firewall strings this story's grouping must not drop) and every
`BrowseControl` menu-keyboard/focus test extracted verbatim into
`BrowseControl.test.tsx` -- both ran green throughout every mutation pass
above and were not separately re-broken-and-fixed here, since doing so
would only re-prove Story 7.3/7.7/7.10/7.11's own mutations, not this
story's.

## Gate transcripts (macOS arm64, native)

Run in the order `.github/workflows/verify.yml` uses, after the mutation
pass above and with the working tree back to its intended diff.

- `wails build`: **PASS**. `Built '.../fairdrop.app/Contents/MacOS/fairdrop' in 13.6s.` One pre-existing linker warning (`object file ... built for newer 'macOS' version (13.0) than being linked (11.0)`), unrelated to this change.
- Bindings drift: **PASS** after `chmod 644` on the three regenerated `frontend/wailsjs` files (`App.d.ts`, `App.js`, `models.ts`) -- content byte-identical, only the file mode changed (0644 -> 0755), the documented mode-churn pitfall. No exported `App` command surface changed in this story.
- `gofmt -l .`: **PASS**, no output.
- `go vet ./...`: **PASS**, no output.
- `go tool staticcheck ./...`: **PASS**, no output.
- `go test -count=1 ./...`: **PASS** -- `ok` for `fairdrop`, `internal/network`, `internal/qr`, `internal/server`, `internal/source`, `internal/stream`, `internal/transfer`, `scripts`, `scripts/mutationverdict`.
- `CGO_ENABLED=1 go test -count=1 -race ./...`: **PASS** -- `ok` for all nine packages (`go env CGO_ENABLED` confirmed `1` first; `internal/stream` ~100s under the race detector, the rest a few seconds each).
- `GOOS=darwin GOARCH=arm64 go build ./...` and `GOOS=linux GOARCH=amd64 go build ./...`: **PASS**, both -- pre-flight only, no Go source changed by this story.
- `cd frontend && npm test`: **PASS** -- 19 files, **734 tests** (18 files/719 tests immediately before adding `noButtonEllipsis.test.tsx`'s 15 cases; `BrowseControl.test.tsx` alone carries 35, `IdleView.test.tsx` 42 -- most of the menu-behaviour tests moved out to `BrowseControl.test.tsx` rather than being deleted, so the two files' combined count is close to the original single file's, not simply smaller).
- `cd frontend && npm run test:browser`: **PASS** -- 2 files, **27 tests** (up from 26: the new chevron-geometry test).
- `git checkout -- frontend/browser/captures/` run afterward both times the browser suite ran, per the known PNG-churn pitfall; `git status --porcelain` confirmed no other unexpected diffs remained.
- `npx tsc --noEmit`: **PASS**, no output (ahead of the gate proper, to catch type errors before the more expensive steps).

## One line per AC

1. Drop zone renders glyph, heading, one promise line, then the browse
   control as a centred intrinsic-width pill, in that order, inside the
   zone; the reversal of Story 7.3's full-width rule is named in three
   places (`IdleView.tsx`, `BrowseControl.tsx`, `IdleView.test.tsx`) rather
   than silently applied. Zone itself still has no click handler and no tab
   stop (unchanged tests still pass). **Done, mutation-verified (M1, M3).**
2. All six copy rows changed exactly as specified, quoted character for
   character at the copy.test.ts assertion site and in EXPERIENCE.md.
   **Done.**
3. The two disclosures render as one `.fd-idle-disclosures` `{rounded.xl}`
   surface with a separator between rows, in the same order (Local network
   access, then Troubleshooting); every recovery/firewall string still
   present (pre-existing mutation-verified guarantee re-confirmed green).
   **Done, mutation-verified (M2).**
4. `BrowseControl` lives in its own module with a `label` prop; every
   existing test moved to `BrowseControl.test.tsx` and passes with zero
   behavioural change, using a fixed local `LABEL` constant decoupled from
   the copy registry so the extraction and the copy rename are provably
   independent. **Done.**
5. Menu items read "File"/"Folder" with a leading decorative glyph, no
   ellipsis (there never was one). **Done, mutation-verified (M4).**
6. `copy.cancel.pending` -> "Canceling", `copy.cancel.preparationPending`
   -> "Canceling preparation"; new `noButtonEllipsis.test.tsx` renders
   every view/state combination that carries a button or menu item and
   fails naming any control whose name ends in `…`/`...`. **Done,
   mutation-verified (M5) against both a synthetic fixture and the real
   registry.**
7. Cancel-won summary and Stage-time command-failure document order/focus
   targets unchanged -- their existing tests pass unmodified apart from
   the `.fd-preflight`/`.fd-help` -> `.fd-idle-disclosures` selector
   rename in one order-check (the group wrapper is new; the ordering it
   proves is the same). **Done.**
8. 320 CSS px reflow: the existing rendered-Chromium "keeps the open
   browse menu scrolling only vertically, with nothing clipped" test
   already exercises the new Idle layout (it renders `IdleView` with the
   menu open) and stayed green with no changes needed. Full gate passes;
   `App.focus.test.tsx` passes with the two necessary literal-string
   updates described above under "Deviation" -- flagged rather than
   silently resolved. **Done, with the one recorded deviation.**

## Disagreements between the story text and the prototype

None found. The prototype's `t-idle` template (glyph, heading, promise
line, pill button, then the grouped disclosure list) and the story's ACs
agree on every point implemented here. One cosmetic detail the prototype
shows but the ACs do not require: an SVG download-arrow glyph for the drop
zone's icon, where the shipped product still uses the pre-existing text
character `↓`. Left as-is since no AC names it and changing it was outside
this story's stated scope (drop zone icon styling, not layout).

## Native verification

Pending -- orchestrator. This session drove the rendered-Chromium suite
(`npm run test:browser`) and confirmed both the group layout and the
chevron-alignment fix there, including a live mutation proof for the
chevron fix (M6), but did not drive the built macOS binary by hand. Per
AGENTS.md's rule for anything WebKit-sensitive, the built binary should
still be checked before this ships: Idle's new centred pill button (click
and keyboard-open the menu), the grouped disclosure list opening/closing
with the chevron visibly at the row's trailing edge, and the "Troubleshooting"
label reading correctly, in both colour schemes.

## Nothing else left open

Every acceptance criterion is implemented and either mutation-verified
above or backed by an existing, still-green, previously mutation-verified
guarantee this story did not touch. The one open item is the recorded
`App.focus.test.tsx` deviation, which is a report of an unavoidable
tension in the story text rather than a gap in the implementation.
