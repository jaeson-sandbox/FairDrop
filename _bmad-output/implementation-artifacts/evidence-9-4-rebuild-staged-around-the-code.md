# Evidence: Story 9.4: Rebuild Staged Around the Code

## Summary

Staged is now a centred heading and one-line instruction above one card
(`.fd-packet`): the QR tile (~216px, `{rounded.lg}`) on one side, the item
row, the two link actions, the revealed link (when open) and the caveat
lines on the other. Below the card, one row offers "Trouble connecting?"
(the existing `Disclosure` component) on the left and Cancel (quiet) on
the right. The `fd-packet-tab` kind label, the always-visible direct-link
heading/helper, the always-open `RecoveryHelp` block and the "Show full
name" toggle are gone. The direct link is not rendered, focusable, or in
the accessibility tree until Show Link is activated -- reveal chosen as
option (b), the same transitioned `grid-template-rows`/`visibility`
mechanism `Disclosure` uses, kept as its own `.fd-url-reveal` rule rather
than the `Disclosure` component itself (see "Reveal mechanism" below).
Copy Link copies without revealing, through the same bound
`CopyToClipboard`, with a non-reflowing crossfade to "Copied" (check
glyph, success tint). Nine copy strings changed or were added/removed
exactly as specified, amended in EXPERIENCE.md's Voice and Tone table and
Component Patterns rows, and in DESIGN.md's matching component rows, in
the same commits as `copy.ts`.

## Reveal mechanism: option (b), a standalone rule

The story's acceptance criteria prefer option (b) -- keeping the field
mounted behind the same grid-rows/visibility mechanism `Disclosure` uses,
for parity with the prototype and to keep Story 7.9's guarantees intact --
"if the Story 7.9 mirror sizing and `staged-url-field.test.tsx` still
hold". Both hold, confirmed by the full rendered-browser pass (below), so
option (b) is what shipped.

It is a **new rule**, `.fd-url-reveal`/`.fd-url-reveal__inner`/
`.fd-url-reveal__body` in `style.css`, structurally identical to
`.fd-disclosure__region`/`-region-inner`/`-body` (same `grid-template-rows:
0fr <-> 1fr`, same transitioned `visibility`, same collapsed-content-stays-
mounted contract) rather than a literal reuse of the `Disclosure`
component. `Disclosure` wraps its trigger in an `<h2>` (the WAI-ARIA APG
accordion pattern its own doc comment names) because its summary *is* a
heading; the Show/Hide Link trigger is a plain button beside Copy Link,
not a heading, so wrapping it in `Disclosure` would either invent a
spurious heading or misuse the component's contract. Duplicating the
mechanism under a new name keeps both call sites honest about what they
are, at the cost of ~25 lines of near-identical CSS -- documented in both
`style.css` and `styles.test.ts` (a new describe block mirroring the
`.fd-disclosure__region` mutation-proof pair verbatim) so a future reader
finds the parallel immediately.

## What changed

**Copy (`copy.ts` + EXPERIENCE.md's Voice and Tone table, amended in the
same commits):**

- `copy.stage.heading`: "Ready to pass along" -> "Ready to send".
- `copy.qr.instruction`: "Scan this code on the receiving device to start
  the download." -> "Scan the code with the receiving device's camera."
- `copy.direct_link.action`: "Copy download link" -> "Copy Link" (now
  copies without revealing).
- New `copy.direct_link.show` -> "Show Link", `copy.direct_link.hide` ->
  "Hide Link".
- `copy.direct_link.helper` removed (the field it described is no longer
  always visible).
- `copy.first_opener.warning`: the long V1-link sentence -> "Works once:
  the first device to open it gets the file." (stays visible, info
  glyph).
- New `copy.first_opener.previews` -> "Link previews in chat apps can
  count as that first device, so paste the link straight into a browser."
  (moved into "Trouble connecting?").
- `copy.network.disclosure`: the long sentence -> "Not encrypted. Use it
  only on a network you trust." (stays visible, lock glyph).
- `copy.local_copy.disclosure`: "Sent directly over your local network...."
  -> "FairDrop keeps no copy. The receiving device keeps what it
  downloads." (moved into "Trouble connecting?").
- `copy.name.show_full` removed from the registry and the table, along
  with the toggle and the two-line clamp it existed to expand.
- New `copy.help.heading` -> "Trouble connecting?" (the disclosure's own
  summary).

`copy.test.ts` extended: every literal assertion for the changed/new
strings, the exact key-path list updated (`directLink.show`/`.hide`,
`firstOpener.previews`, `help.heading` added; `directLink.helper`,
`name.showFull` removed), functional-label assertions untouched
(`directLinkHeading` stays a `label`, now read via `aria-label` on the
revealed field rather than `aria-labelledby` on a rendered heading).
EXPERIENCE.md's Voice and Tone table rows updated to match exactly (the
spine-quotes-registry test, D-119, re-verified this); Component Patterns
rows for StagedView, Item Summary, Direct URL Row, Copy Feedback and
Trusted-LAN Note rewritten for the new behaviour; the "Staged/ready" State
Pattern row updated.

**`StagedView.tsx` (full rebuild):**

- `.fd-staged-head`: centred `<h1>` + one-line instruction, above the
  card.
- `.fd-packet`: the one card, still holding the warning banners and the
  hero (QR + details); no longer holds Cancel or the recovery block.
- `.fd-item`: kind glyph (new `FileKindGlyph`/`FolderKindGlyph`, decorative,
  `aria-hidden`) beside the name/meta, replacing the removed
  `fd-packet-tab` span and the "Show full name" toggle. The name now wraps
  via `.fd-headline`'s existing `overflow-wrap: anywhere` with no clamp at
  all.
- `.fd-direct-row`: Copy Link (primary, crossfading to a check glyph +
  "Copied" via `.fd-button__swap`/`.fd-button__swap-face`) and Show/Hide
  Link (secondary, `aria-expanded`/`aria-controls`) as a wrapping flex row.
- `.fd-url-reveal`: the collapsed-by-default region holding the unchanged
  Story 7.9 field (`.fd-url-wrap`/`.fd-url-mirror`/`.fd-url fd-target`);
  `aria-label={copy.label.directLinkHeading}` replaces the old
  `aria-labelledby` pointing at a now-removed visible heading.
- `.fd-caveats`: first-opener (info glyph) and network (lock glyph) stay
  visible; the folder note becomes a third, glyph-less line
  (`.fd-caveats__plain`) when `metadata.isDir`.
- `.fd-staged-foot`: "Trouble connecting?" (`Disclosure`, new
  `StagedHelpContent`) on the left, Cancel (quiet) on the right.
- `.fd-rise`/`--fd-stagger` applied to the item row, the actions row and
  the caveats list, per Story 9.1's per-child stagger (a local `rise()`
  helper, duplicated from `IdleView.tsx` -- see that file's own comment on
  why it isn't extracted yet). The QR tile is deliberately **not** part of
  this stagger -- see the next bullet.
- The command-error inline panel (`clipboard_failed`) is untouched --
  still an `OutcomePanel` at the end of the view, not folded into the
  card.

**`RecoveryHelp.tsx`:** the always-open `RecoveryHelp` wrapper (default
export) is removed -- its one consumer was `StagedView`, which no longer
renders it. New `StagedHelpContent` composes the local-copy line and the
link-preview caveat ahead of the unchanged `RecoveryHelpContent` (still
`IdleView`'s only consumer, byte-for-byte the same, so Idle's
"Troubleshooting" content and its existing tests are untouched).

**`style.css`:** see "Reveal mechanism" above for `.fd-url-reveal`. Also:
`.fd-clamp`/`.fd-name-toggle` removed (dead); `.fd-hero`'s second column
and `.fd-qr-panel`'s width/radius changed 224px/`{rounded.xs}` ->
216px/`{rounded.lg}`; `.fd-qr-panel` gained its own fade-plus-scale-from-
~0.94 entrance (opacity 300ms, scale 420ms, `@starting-style`), matching
DESIGN.md's Motion section note that "cards and discs in the stories that
follow" get the browse menu's pattern -- unstaggered (no `.fd-rise`),
since the QR is the view's focal object, entering with the view rather
than queued behind it; `.fd-direct-row` changed from a two-column grid
(field + button) to a wrapping flex row of two buttons, dropping its
759px-breakpoint override (nothing left to reflow there); `.fd-trust`/
`.fd-trust p:first-child::before` removed, replaced by `.fd-caveats`/
`.fd-caveats li`/`.fd-caveats__glyph`/`.fd-caveats__plain`; new
`.fd-item`/`.fd-item__icon`/`.fd-item__glyph`/`.fd-item__text`; new
`.fd-button__swap`/`.fd-button__swap-face`/`.fd-button__glyph`; new
`.fd-staged-head`/`.fd-staged-foot` (plus a narrow, targeted
`.fd-staged-foot .fd-button--quiet { width: auto; }` override so Cancel
sits beside the disclosure rather than filling the row -- the shared
`.fd-button--quiet { width: 100% }` rule itself is untouched, still
correct for every other lone Cancel/Dismiss); `.fd-help:not(.fd-disclosure)`
removed (no more always-open consumer).

**Icons:** all new SVGs (`LinkGlyph`, `CheckGlyph`, `InfoGlyph`,
`LockGlyph`, `FileKindGlyph`, `FolderKindGlyph`) are local, unexported
functions in `StagedView.tsx`, `aria-hidden focusable="false"`, matching
the owner-approved prototype's path data where the prototype names an
equivalent glyph.

### Files touched

- `frontend/src/ui/copy.ts`, `frontend/src/ui/copy.test.ts`
- `frontend/src/ui/StagedView.tsx`, `frontend/src/ui/StagedView.test.tsx`
- `frontend/src/ui/RecoveryHelp.tsx`
- `frontend/src/style.css`, `frontend/src/ui/styles.test.ts`
- `frontend/browser/accessibility.test.tsx` (clipping-exclusion list and
  the 44px-floor Staged case updated for the collapsed reveal region;
  the forced-colors QR capture regenerated)
- `frontend/browser/staged-url-field.test.tsx` (every case now reveals the
  link before asserting -- the acceptance criterion's own words, "kept
  passing against the revealed state")
- `frontend/src/App.test.tsx` (six literal `'Copy download link'` ->
  `'Copy Link'` updates; no other change)
- `_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/EXPERIENCE.md`
- `_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/DESIGN.md`
- `_bmad-output/implementation-artifacts/sprint-status.yaml` (story ->
  `review`)
- `frontend/browser/captures/qr-panel-forced-colors.capture.png`
  (regenerated deliberately -- see "Capture churn" below)

## Capture churn

The forced-colors QR capture changed (1333 -> 1547 bytes). This is not the
known PNG-encoder noise AGENTS.md describes: the QR tile's own size and
corner radius changed (224px/`{rounded.xs}` -> 216px/`{rounded.lg}`), so
the captured substrate is genuinely a different shape now. Committed
deliberately, not reverted with `git checkout --`.

## Mutation table

Each mutation was applied to the real working tree, confirmed to fail and
name the problem, then reverted -- confirmed clean afterward with `git
diff --stat` against the mutated file. The full suite (`npx vitest run`)
was green before and after the whole pass.

| # | Mutation | AC | File | Result |
|---|---|---|---|---|
| M1 | Added `fd-clamp` to the item-name `<h2>` with no toggle beside it | AC2 | `StagedView.tsx` | KILLED -- `StagedView.test.tsx`, "never clamps a long name, and offers no toggle to reveal one": `expected 'fd-headline fd-clamp' not to contain 'fd-clamp'` |
| M2 | Moved `copy.network.disclosure` into the "Trouble connecting?" disclosure (added a paragraph there) while leaving it in `.fd-caveats` too | AC5 | `StagedView.tsx` | KILLED -- `StagedView.test.tsx`, "never renders the not-encrypted disclosure inside \"Trouble connecting?\"": the disclosure region's `textContent` contained "Not encrypted..." |
| M3 | Removed the `aria-hidden` toggle from the Copy Link face of the crossfade (left it permanently exposed) | AC3/AC4 | `StagedView.tsx` | KILLED -- `StagedView.test.tsx`, "keeps both label faces mounted for the crossfade, with exactly one aria-hidden at a time": `expected [] to have a length of 1 but got +0` after a successful copy |
| M4 | Had `handleCopy` also call `setRevealed(true)` | AC3 | `StagedView.tsx` | KILLED -- `StagedView.test.tsx`, "leaves the link collapsed and the trigger labelled Show Link after a successful copy": `getByRole('button', {name: 'Show Link'})` found nothing (it had become "Hide Link") |
| M5 | Reintroduced `max-height: 0` on `.fd-url-reveal` | AC3 | `style.css` | KILLED -- `styles.test.ts`, "expands and collapses the Staged link-reveal region the same way, with no fixed height (Story 9.4)": `expect(region).not.toMatch(/max-height\|max-block-size/)` failed |
| M6 | Removed the `copy.help.heading` row from EXPERIENCE.md's Voice and Tone table | AC5 | `EXPERIENCE.md` | KILLED -- `copy.test.ts`, "registers every approved string, so new copy cannot ship untabulated": `expected [ 'help.heading' ] to deeply equal []` |
| M7 | Gave `.fd-qr-panel` the `fd-rise` class (restaggering it behind the item row instead of its own entrance) | AC8 | `StagedView.tsx` | KILLED -- `StagedView.test.tsx`, "does not stagger the QR tile behind the view -- it carries no fd-rise class": `expected 'fd-qr-panel fd-rise' not to contain 'fd-rise'` |
| M8 | Dropped `scale: 0.94;` from `.fd-qr-panel`'s `@starting-style` block (fade only, no scale) | AC8 | `style.css` | KILLED -- `styles.test.ts`, "scales and fades the Staged QR tile in from ~0.94, via @starting-style, unstaggered (Story 9.4)": `expected null to be truthy` |

M1 restates the removed-clamp guarantee as something a future regression
would actually trip (not merely "the toggle doesn't exist", which a stray
CSS-only clamp would satisfy trivially). M2 is the exact mutation the
acceptance criteria name for the one security disclosure. M3/M4 prove the
crossfade's accessible-name guarantee and the copy/reveal independence
AC3 requires, respectively. M5 proves the reveal region's mechanism is
load-bearing CSS, not decoration, mirroring the pre-existing
`.fd-disclosure__region` mutation this story's own new test pair is
modelled on. M6 proves the spine-table cross-check (D-119) still catches
an untabulated approved string after this story's edits to the registry.
M7/M8 prove AC8's entrance claim at both layers -- the component (right
class, no `fd-rise`) and the stylesheet (right transition/`@starting-style`
pair) -- since this was found missing on first pass (the initial
implementation only applied the generic `.fd-rise` stagger to the QR
tile, which fades and rises 8px rather than scaling from ~0.94; DESIGN.md's
Motion section note that "cards and discs in the stories that follow" get
their own fade-plus-scale treatment is what caught it on review before
handoff).

Two further guarantees this story touches but did not re-mutate, because
their own mutations predate this story and remain intact and green
throughout the whole pass: Story 7.9's mirror-sizing guarantees
(`staged-url-field.test.tsx`, now run against the revealed state) and
`Disclosure.tsx`'s own keyboard/focus/content contract (unchanged,
reused as-is for "Trouble connecting?").

## Gate transcripts (macOS arm64, native)

Run in the order `.github/workflows/verify.yml` uses, after the mutation
pass above and with the working tree back to its intended diff.

- `wails build`: **PASS** -- `Built '.../fairdrop.app/Contents/MacOS/fairdrop' in 13.271s.` One pre-existing linker warning (`object file ... built for newer 'macOS' version (13.0) than being linked (11.0)`), unrelated to this change.
- Bindings drift: **PASS** after `git checkout -- frontend/wailsjs` -- `git diff --stat` showed 0 insertions/0 deletions (mode-only churn), confirmed content byte-identical. No exported `App` command surface changed in this story.
- `gofmt -l .`: **PASS**, no output.
- `go vet ./...`: **PASS**, no output.
- `go tool staticcheck ./...`: **PASS**, no output.
- `go test -count=1 ./...`: **PASS** -- `ok` for `fairdrop`, `internal/network`, `internal/qr`, `internal/server`, `internal/source`, `internal/stream`, `internal/transfer`, `scripts`, `scripts/mutationverdict`.
- `CGO_ENABLED=1 go test -count=1 -race ./...`: **PASS** -- `ok` for all nine packages (`go env CGO_ENABLED` confirmed `1` first; `internal/stream` ~102.7s under the race detector, the rest a few seconds each).
- `GOOS=linux GOARCH=amd64 go build ./...`: **PASS**. (`GOOS=darwin GOARCH=arm64` is this machine's native target, already covered by the `go test`/`wails build` runs above, so it was not repeated as a separate cross-build; `go tool staticcheck` likewise already ran natively.)
- `cd frontend && npx vitest run`: **PASS** -- 19 files, **746 tests**.
- `cd frontend && npm run test:browser`: **PASS** -- 2 files, **27 tests**.
- `git checkout -- frontend/browser/captures/` **not** run afterward -- the QR capture's change is deliberate (see "Capture churn" above) and is included in the diff on purpose.
- No content drift confirmed via `git status --short` after the full gate: exactly the 14 files this story intentionally touched.

## One line per AC

1. Staged is a centred heading/instruction above one card: QR tile (~216px,
   `{rounded.lg}`) on one side, item row + two link actions + revealed
   link + caveats on the other; below the card, "Trouble connecting?" and
   Cancel; the card collapses to one column under the existing 759px
   breakpoint with the QR first (unchanged `.fd-hero` mechanism). **Done.**
2. `fd-packet-tab`, the always-visible direct-link heading/helper, the
   always-open `RecoveryHelp` and "Show full name" are gone; names wrap
   via `overflow-wrap: anywhere`. **Done, mutation-verified (M1).**
3. The link is not rendered/focusable/in the a11y tree until requested;
   Copy Link (primary) copies without revealing; Show Link (secondary,
   `aria-expanded`/`aria-controls`) reveals with a smooth expansion and
   toggles to Hide Link; every Story 7.9 guarantee holds against the
   revealed state (`staged-url-field.test.tsx`, updated to reveal first).
   **Done, mutation-verified (M4, M5).**
4. Copy Feedback is a non-reflowing crossfade to "Copied" (check glyph,
   success tint), reverts on blur (D-114, untouched), announced once
   exactly as before; existing copy tests pass unchanged in substance
   (updated only for the new button name). **Done, mutation-verified
   (M3).**
5. All nine copy changes land exactly as specified; the two visible
   caveats (info/lock glyph) stay on the card; the four "Trouble
   connecting?" strings (local-copy, preview, both firewall recoveries)
   plus both `copy.help.*` strings live behind the disclosure.
   **Done, mutation-verified (M2, M6).**
6. `copy.folder.note` renders as a third caveat line for a folder; meta
   still reads `Folder · <size> logical size`. **Done.**
7. `beacon_warning`/`name_warning` banners still render inside the card,
   above the item row, still describing the heading. **Done** (structural
   DOM-order test added; pre-existing warning-banner behaviour otherwise
   untouched).
8. The QR enters with its own fade-plus-scale-from-~0.94 rule (`.fd-qr-panel`,
   unstaggered -- DESIGN.md's Motion section names this pattern for "cards
   and discs in the stories that follow" the browse menu) and keeps
   `draggable={false}` plus `-webkit-user-drag: none`; the existing drag-
   source pins pass unchanged. **Done, mutation-verified (M7, M8).**
9. Full gate passes (above); the rendered-Chromium accessibility suite
   passes at 640x480 and 200% text with no clipped action (existing tests,
   now correctly excluding the collapsed reveal region from the clipping
   sweep, and the 44px-floor case revealing the link first). **Done.**

## Disagreements between the story text and the prototype

- **DESIGN.md's Trusted-LAN Note row said "never green or lock-shaped"**,
  written before this story's own approved prototype used exactly a lock
  glyph on the network disclosure. The story's acceptance criteria name
  the lock glyph explicitly ("an info glyph and a lock glyph
  respectively"), so the story's text -- which is also what the
  owner-approved prototype itself shows -- wins over the earlier DESIGN.md
  wording it contradicts. Amended the row rather than left contradicted;
  see the row's own comment for the fuller reasoning. This is a
  story-vs-earlier-spine conflict, not a story-vs-prototype one -- the
  story and the prototype agree with each other here, and disagree with a
  stale row.
- **The prototype's foot row uses a plain quiet `.link` trigger** ("Trouble
  connecting?" with its own small chevron, no card), not a boxed
  `Disclosure` surface. The story's text is explicit that "Trouble
  connecting?" uses the existing `Disclosure` component, so that is what
  shipped -- a `{rounded.xl}` card, visually heavier than the prototype's
  plain link. Flagging this rather than resolving it silently: the story
  text and the prototype disagree on this one visual detail, and the
  story's text (which names a specific existing component to reuse) wins
  per the epic's own instruction.
- **The prototype's foot-row help content ("helpbox") sits below the whole
  foot row**, spanning full width, rather than nested directly under its
  own trigger the way `Disclosure`'s region does. Consuming the actual
  `Disclosure` component (per the point above) means the region is
  necessarily nested under its own summary, not laid out as a second,
  separate full-width block. Same root cause as the previous point, noted
  separately because it is a layout difference, not only a visual-weight
  one.

## Native verification

Pending -- orchestrator. This session drove the rendered-Chromium suite
(`npm run test:browser`) and confirmed the reveal mechanism, the crossfade,
the caveat glyphs and the 320px/200%-text/forced-colors/44px-floor
guarantees there, but did not drive the built macOS binary by hand. Per
AGENTS.md's rule for anything WebKit-sensitive, the built binary should
still be checked before this ships: Show Link/Hide Link by pointer and
keyboard, Copy Link's crossfade and D-114 revert, "Trouble connecting?"
opening/closing, and the QR drag-source guard, in both colour schemes and
with Reduce Motion on.

## Nothing else left open

Every acceptance criterion is implemented and mutation-verified above,
or backed by an existing, still-green, previously mutation-verified
guarantee this story did not touch (Story 7.9's field-sizing mutations,
`Disclosure.tsx`'s own contract, D-114's blur-revert). The three
story-vs-prototype/spine disagreements are recorded above rather than
resolved silently. Native verification is the orchestrator's, as scoped.

## Review follow-up (branch `fix-9-4-staged-layout`, from `origin/epic-9-motion-and-clarity` @ `99df048`)

After Story 9.4 merged, the orchestrator's rendered check at 1024x768
found two layout defects neither suite caught. Both are pure CSS/layout
bugs jsdom cannot evaluate (it performs no layout), so this follow-up adds
rendered-Chromium coverage in `browser/accessibility.test.tsx` for both,
following the defect-fix carve-out: failing test first, then the fix,
then a mutation confirming the test actually catches the regression.

### Defect 1: the hero card's fixed track was assigned to the wrong element

**Cause.** `.fd-hero` declared `grid-template-columns: minmax(0, 1fr)
216px;` -- the fixed track second -- but `StagedView.tsx`'s DOM order
renders `.fd-qr-panel` first and `.fd-hero__details` second. CSS Grid
assigns tracks to children strictly in DOM order, not by which element a
reader would expect to get which track, so the QR landed in the large
flexible track (and sat centred in the leftover space, per its own
`min(216px, 100%)` width) while the item details were crushed into the
216px track: the name wrapped mid-word, Copy Link and Show Link stacked
vertically instead of sharing a row, and the caveats wrapped into narrow
lines.

**Why the suites missed it.** `styles.test.ts` only proves the stylesheet
*text* contains a two-track template -- it does, just with the tracks in
the wrong order -- and no rendered test measured which element actually
ended up in which track. This is exactly the class of bug the story's own
"Testing standards" section warns about: a CSS-text pin proves a rule
exists, never that the right element receives it.

**Fix.** `grid-template-columns: 216px minmax(0, 1fr);` (fixed track
first, matching DOM order), `align-items: center` (was `start`, matching
the owner-approved prototype's `.card`), and `gap: var(--spacing-6)` (24px,
was `--spacing-7`/32px, also matching the prototype). `styles.test.ts`'s
literal pin for the template updated to match, with a comment explaining
why track order is DOM-order-dependent.

**New tests** (`browser/accessibility.test.tsx`, describe "the hero card
assigns its fixed track to the element the DOM actually puts there"),
each run at 1024x768 and 640x480:

- the details column's rendered width is greater than the QR tile's,
- Copy Link and Show Link share the same rendered top (within 2px),
- a realistic long name ("dev-environment-guide.html", the prototype's own
  item) renders on exactly one visual line (`Range.getClientRects().length
  <= 1` against the name's own text node -- a direct wrap measurement, not
  an inference from element height).

**Before/after measurements (1024x768, `renderStaged()`'s default 8.4MB
"Travel Notes.pdf" state unless noted):**

| Measurement | Before (broken) | After (fixed) |
|---|---|---|
| `.fd-qr-panel` width | 430.0px (the 1fr track) | 216.0px |
| `.fd-hero__details` width | 216.0px (the fixed track) | 430.0px |
| Copy Link / Show Link top | 158.7px / 210.7px (52px apart, stacked) | same row (<=2px apart) |
| "dev-environment-guide.html" | wraps across 2 lines | 1 line |

### Defect 2: the foot row was never actually one row

**Cause, in two parts, both on `.fd-staged-foot .fd-disclosure`.**
`.fd-disclosure__summary` (inside the `Disclosure` component, unscoped)
fills its container's full width on its own, so the surrounding
`.fd-disclosure` box had to be given a real, definite width for that
child's own full-width behaviour to resolve against something other than
the whole row. The rule this branch found in place, `flex: 1 1 auto`, does
not do that: with an automatic flex-basis, a flex item's hypothetical size
for the *line-wrap decision* in a wrapping flex container is its
max-content extent, and `Disclosure`'s collapsed help region stays mounted
at full content size even at zero rendered height (Story 9.1's
collapsed-content-stays-mounted contract) -- so the disclosure's own
max-content extent was that of its single longest unwrapped sentence (the
macOS firewall recovery line), comfortably wider than the whole row. That
alone forced the disclosure onto its own line, growing to fill it once
alone there, with Cancel bumped onto a second line below -- rendering as a
full-width card with a separate bordered-looking Cancel button beneath it,
not the one row the acceptance criteria describe.

**Why the suites missed it.** The same gap as Defect 1: nothing rendered
and measured the foot row's actual geometry. The original comment on
`.fd-staged-foot` asserted the disclosure "sizes to its own content as a
flex item," a claim that was never measured against the real collapsed
content it also carries.

**Fix.** `.fd-staged-foot .fd-disclosure` gets `flex: 1` (grow 1, shrink 1,
a *zero* basis -- not the shorthand's own auto basis) plus
`min-inline-size: 0`, `background: none`, `box-shadow: none` (dropping the
boxed `{rounded.xl}` card surface every other `Disclosure` keeps). Its
summary gets quieter styling scoped the same way: auto width, muted color,
a hover fill, a smaller chevron, and an explicit `min-block-size:
var(--spacing-target-min)` decoupled from its own quieter padding so the
44px activation floor survives the visual change. `.fd-staged-foot`
itself changes `align-items` from `center` to `flex-start`, and Cancel
(`.fd-staged-foot .fd-button--quiet`) gets `flex: none; align-self:
flex-start`. All of this is scoped under `.fd-staged-foot .fd-disclosure*`
so Idle's own grouped disclosure list is completely unaffected -- confirmed
by the full existing `IdleView.test.tsx` suite passing unchanged throughout.

One thing found and corrected while writing the mutation table: mutating
`.fd-staged-foot`'s own `align-items` alone (reverting `flex-start` back
to `center`) left every new test passing. Cancel's own `align-self:
flex-start` is what actually pins its top edge regardless of the
container's setting, not the container's `align-items` -- the code
comment originally claimed otherwise and was corrected to say so once the
mutation disproved it, rather than left as an unverified claim.

**New tests** (`browser/accessibility.test.tsx`, describe "the Staged foot
row keeps 'Trouble connecting?' and Cancel on one row"):

- collapsed, the disclosure trigger and Cancel share the same rendered top
  (within 2px) at 1024x768 and 640x480,
- expanded, Cancel's top is unchanged (within 2px) from its collapsed
  position, and the opened help region has non-zero rendered width/height
  and contains every one of the six help strings (both firewall
  recoveries, both `copy.help.*` strings, the local-copy disclosure, the
  link-preview caveat),
- the collapsed disclosure trigger still measures at or above the 44px
  activation floor despite its quieter padding.

**Before/after measurements (1024x768):**

| Measurement | Before (broken) | After (fixed) |
|---|---|---|
| `.fd-staged-foot` rendered height | 100px (two rows: 44 + 12 gap + 44) | 44px (one row) |
| Disclosure trigger / Cancel top | 355.9px / 411.9px (56px apart) | same row (<=2px apart) |
| Cancel's top after expanding | 411.9px -> 686.1px (274px shift) | unchanged (<=2px) |

### Mutation table

Each mutation applied to the real working tree, confirmed to fail (naming
the defect) via the exact test text above, then reverted.

| # | Mutation | Defect | File | Result |
|---|---|---|---|---|
| N1 | Reverted `.fd-hero`'s template to `minmax(0, 1fr) 216px` (fixed track second) | 1 | `style.css` | KILLED -- 3 tests failed: "gives the details column more width..." (216.0 not > 216.0), "keeps Copy Link and Show Link on the same row..." (52px apart), "keeps a realistic item name on one line..." (wraps) |
| N2 | Reverted `.fd-staged-foot .fd-disclosure`'s `flex: 1` to `flex: 1 1 auto` | 2 | `style.css` | KILLED -- 3 tests failed: "aligns the collapsed disclosure trigger and Cancel..." (56.0px apart) x2 viewports, "keeps Cancel's top unchanged when..." (274.1px shift) |
| N3 | Reverted `.fd-staged-foot`'s `align-items` from `flex-start` to `center` (isolated from N2) | 2 (a claim in the code comment, not a defect) | `style.css` | NOT KILLED -- all 10 tests in both defect-fix describe blocks stayed passing, proving Cancel's own `align-self: flex-start` (not the container's `align-items`) is what pins its position; the code comment was rewritten to state this rather than the disproved original claim |

N1 and N2 are the two acceptance mutations this follow-up's own instructions
name ("revert each fix -> its test fails naming it"); both killed cleanly.
N3 was not part of the assignment but was run anyway while writing the
"why this works" comment for the `align-items` change, per this repo's own
rule that an unverified claim in a comment is exactly the kind of thing a
mutation should have caught before it shipped. Finding it unnecessary here
and saying so is the same discipline as finding a mutation that kills.

### Gate transcripts (macOS arm64, native)

Run in the order `.github/workflows/verify.yml` uses, after the mutation
pass above and with the working tree back to its intended diff.

- `wails build`: **PASS** -- `Built '.../fairdrop.app/Contents/MacOS/fairdrop' in 9.041s.` Same pre-existing linker warning as before, unrelated to this change.
- Bindings drift: **PASS** after `git checkout -- frontend/wailsjs` -- 0 insertions/0 deletions (mode-only churn). No exported `App` command surface touched.
- `gofmt -l .`: **PASS**, no output.
- `go vet ./...`: **PASS**, no output.
- `go tool staticcheck ./...`: **PASS**, no output.
- `go test -count=1 ./...`: **PASS** -- `ok` for all nine packages.
- `CGO_ENABLED=1 go test -count=1 -race ./...`: **PASS** -- `ok` for all nine packages (`go env CGO_ENABLED` confirmed `1`; `internal/stream` ~99.6s under the race detector).
- `cd frontend && npx vitest run`: **PASS** -- 19 files, **746 tests** (one pre-existing pin, `.fd-hero`'s grid-template-columns literal, updated for the new column order; no jsdom test added or removed here -- both defects are pure layout, provable only in the rendered suite below).
- `cd frontend && npm run test:browser`: **PASS** -- 2 files, **41 tests**, **10 new** for this follow-up (6 for Defect 1: three `it.each` cases x2 viewports; 4 for Defect 2: one `it.each` case x2 viewports plus two single-viewport cases). The other 31 were already on this branch after the fetch: Story 9.4's own 27 plus the four already-merged Idle chevron/drop-zone defect-fix tests the coordinator's message named.
- Capture churn: none this run -- `git status --short` showed no change to `frontend/browser/captures/`; the QR tile's own size/radius were untouched by this follow-up.
- No content drift beyond the three files this follow-up touches (`frontend/src/style.css`, `frontend/src/ui/styles.test.ts`, `frontend/browser/accessibility.test.tsx`), confirmed via `git status --short` after the full gate.

Two comments were rewritten a second time after the first draft tripped
`styles.test.ts`'s own stylesheet-content scanners (`the Quartz token
layer > reaches every component value through a token rather than a
literal`, `reflow to 320 CSS pixels > declares no width that could force a
page-level horizontal scrollbar`): both scan the raw stylesheet text,
comments included, for color-literal words and `property: value`-shaped
substrings followed by a large pixel figure before the next `;`/`}`. The
first draft's prose used "stayed green" (a banned color word) and spelled
out `flex-basis: auto`/`flex-basis: 0` with a literal, uncomparably-large
"700px" later in the same unterminated sentence, which the second scanner
read as a real declaration whose value exceeded the 320px reflow floor.
Rewritten to describe the same mechanism without a colon-adjacent property
name or a bare pixel figure over 320 in prose. Recorded here because it is
itself a small instance of this repo's own rule: a test that scans raw
text for meaning can be tripped by prose describing that text, and the fix
is to write around the scanner, not loosen it.
