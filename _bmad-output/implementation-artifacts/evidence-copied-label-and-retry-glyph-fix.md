# Evidence: "Copied" label after a WebKit pointer click, and Try Again's glyph spacing (defect fixes)

Defect-fix carve-out (AGENTS.md "Git workflow"): two owner-observed defects, a failing
test written first for each, both fixes mutation-verified. Branch
`fix-copied-label-and-retry-glyph`, forked from `epic-9-motion-and-clarity`, no story.

## Defect 1: "Copied" never appears after a pointer click on macOS

### The defect

Observed by the orchestrator on the built binary: on the Staged screen, clicking Copy
Link with the mouse copies the URL correctly (verified with `pbpaste`) and the announcer
speaks, but the button's label never changes to "Copied" and never gets the success
tint. This predates Epic 9 -- the same code shipped in v1.2.1.

### Diagnosis

`StagedView.tsx`'s `handleCopy` only calls `setCopied(true)` when `focusedRef.current`
is `true`, and that ref is set only by the button's own `onFocus` handler:

```tsx
const handleCopy = () => {
    setCopied(false)
    void Promise.resolve()
        .then(() => CopyToClipboard(metadata.url))
        .then(() => {
            if (focusedRef.current) setCopied(true)
            onAnnounce?.(state.session.sessionId, copy.copy.confirmation)
        }, () => onCopyFailed?.(state.session.sessionId))
}
```

AGENTS.md's macOS WebKit focus fact 3: "Clicking a `<button>` does not focus it.
WebKit mirrors native macOS here, so after a pointer click focus is on
`document.body`." Chromium and jsdom both focus a clicked button by default, so
`onFocus` fires there for free and every existing test passed -- the suite's own
`pressCopy()` helper even calls `button.focus()` itself first, stating that
precondition explicitly rather than depending on jsdom's click to supply it. On WebKit
that precondition never holds for a mouse activation, so `focusedRef.current` stays
`false`, the confirmation is never claimed, and no blur is ever coming either to explain
its absence -- the write succeeds, the announcer speaks, and the label silently never
changes.

### Failing test, written first, captured before any fix

Added to `frontend/src/ui/StagedView.test.tsx`, simulating the WebKit behaviour: a plain
`fireEvent.click` with no prior focus, `document.activeElement` left at `document.body`
(what a real WebKit pointer click leaves behind, unlike jsdom's own default click
behaviour).

```
$ npx vitest run -t "claims Copied after a pointer click"
...
 FAIL  src/ui/StagedView.test.tsx > claims Copied after a pointer click that never
       focused the button first (WebKit)
TestingLibraryElementError: Unable to find an accessible element with the role "button"
and name "Copied"
 ❯ src/ui/StagedView.test.tsx:844:19
    843|     expect(writeText).toHaveBeenCalledWith(capabilityURL)
    844|     expect(screen.getByRole('button', {name: 'Copied'})).toBeTruthy()

 Test Files  1 failed (1)
      Tests  1 failed | 50 passed (51)
```

### The fix

`handleCopy` now takes the click event and focuses the control explicitly, before
issuing the command:

```tsx
const handleCopy = (event: MouseEvent<HTMLButtonElement>) => {
    event.currentTarget.focus()
    setCopied(false)
    ...
}
```

`event.currentTarget.focus()` triggers the same `onFocus` handler that already sets
`focusedRef.current = true`, synchronously, before the async clipboard command's
`.then()` reads it -- so "the control holds focus after activation" becomes true on
WebKit exactly as it already was on Chromium/jsdom, rather than adding a second,
unfocused path to "claim Copied". D-114's revert-on-blur and the "still retrying" click-
while-already-focused case are untouched: both depend only on the same ref, set by the
same mechanism, and neither test's precondition (focus already held, then blurred, or
focus already held, then re-clicked) changes.

**Focus-ring risk, considered per the task brief's Story 7.10 pointer:** unlike
`BrowseControl`'s `closeAndReturnFocus`, which moves focus to a *different*, previously
keyboard-focused element and needs the `[data-focus-return]` marker so WebKit's separate
script-focused-`:focus-visible` gap (fact 2) doesn't hide an earned ring, this call
re-focuses the *same* element the click's own default action already focuses (or would
focus, on the engines where it does). It does not introduce a new focus target or a new
modality; it only makes WebKit perform the same self-focus Chromium and jsdom already
perform by default on a button click. No rendered check in this repo can drive real
WebKit (AGENTS.md: Playwright's WebKit project cannot reach the Cocoa
`tabFocusesLinks` preference either), so this reasoning is not automation-verified
against real WebKit; it is recorded here rather than asserted as more certain than it
is, per the "have a person press it" rule for anything a synthetic probe cannot settle.

### After the fix

```
$ npx vitest run src/ui/StagedView.test.tsx
 Test Files  1 passed (1)
      Tests  51 passed (51)
```

### Mutation

Removed `event.currentTarget.focus()` from `handleCopy`, keeping the rest of the
function (and the `MouseEvent` parameter) unchanged:

```
$ npx vitest run src/ui/StagedView.test.tsx
 FAIL  src/ui/StagedView.test.tsx > claims Copied after a pointer click that never
       focused the button first (WebKit)
TestingLibraryElementError: Unable to find an accessible element with the role "button"
and name "Copied"

 Test Files  1 failed (1)
      Tests  1 failed | 50 passed (51)
```

**Caught** -- exactly the new test fails, naming the missing "Copied" label; the other
50 tests (including D-114's revert-on-blur and the retry-while-focused case) still pass
unmodified. Fix restored and reverified green (51/51) before continuing.

## Defect 2: Try Again's refresh glyph touches its label

### The defect

On the error card, the refresh glyph on "Try Again" sits flush against the "T" of the
label -- no gap at all -- unlike "Send Another"/"Choose Another" (BrowseControl's own
trailing chevron) and unlike the Copy Link glyph in Staged, both of which have a gap.

### Diagnosis

`OutcomePanel.tsx`'s Try Again button renders `<RefreshGlyph/>` as a direct child of the
button, immediately followed by the label text:

```tsx
<button ... className="fd-button fd-button--primary fd-button--pill fd-target" ...>
    <RefreshGlyph/>
    {copy.outcome.tryAgain}
</button>
```

`.fd-button__glyph` (the glyph's own class, shared with StagedView's Copy Link glyphs)
sets only `width`/`height`/`flex: none` -- no spacing. Copy Link's glyphs are spaced for
free: they live inside `.fd-button__swap-face`, and that element's own
`gap: var(--spacing-2)` is what separates each glyph from its text. `.fd-button` itself
has no `gap`. Send Another/Choose Another's chevron (`.fd-browse-trigger__chevron`)
carries its own `margin-inline-start`. RefreshGlyph has neither a wrapping flex gap nor
its own margin, so it renders flush.

### Failing test, written first, captured before any fix

Added to `frontend/browser/accessibility.test.tsx` (rendered, real Chromium): measures
the horizontal gap between the glyph's bounding rect and a `Range` over the label's own
text node, at 1024x768.

```
$ npx vitest run --config vitest.browser.config.ts -t "keeps the refresh glyph clear"
...
 FAIL |chromium| browser/accessibility.test.tsx > the outcome card is one centred,
      column-width card (Story 9.6) > keeps the refresh glyph clear of the "Try Again"
      label at 1024x768
AssertionError: the glyph's right edge sits 0.0px from the label text's own left edge
(glyph right: 434.9px, text left: 434.9px) -- anything under 5px reads as touching the
label: expected 0 to be greater than or equal to 5

 Test Files  1 failed | 1 skipped (2)
      Tests  1 failed | 72 skipped (73)
```

### The fix

Added, in `frontend/src/style.css`, right after `.fd-button__glyph`:

```css
.fd-button > .fd-button__glyph {
    margin-inline-end: var(--spacing-2);
}
```

Scoped to a direct child of `.fd-button` (`>`), not every `.fd-button__glyph`
unconditionally: Copy Link's glyphs are children of `.fd-button__swap-face`, not of
`.fd-button` directly, so this rule does not match them and does not stack a second
margin on top of their existing flex `gap`. Only RefreshGlyph -- the one glyph that is
actually a direct child of a `.fd-button` -- picks up the new spacing, matching Copy
Link's own `var(--spacing-2)` (8px) magnitude per the task brief.

### After the fix

```
$ npx vitest run --config vitest.browser.config.ts -t "keeps the refresh glyph clear"
 Test Files  1 passed | 1 skipped (2)
      Tests  1 passed | 72 skipped (73)
```

### Mutation

Removed the new `.fd-button > .fd-button__glyph { margin-inline-end: var(--spacing-2); }`
rule from `style.css` (the whole added block, comment included):

```
$ npx vitest run --config vitest.browser.config.ts -t "keeps the refresh glyph clear"
 FAIL |chromium| ... keeps the refresh glyph clear of the "Try Again" label at 1024x768
AssertionError: ... expected 0 to be greater than or equal to 5

 Test Files  1 failed | 1 skipped (2)
      Tests  1 failed | 72 skipped (73)
```

**Caught** -- fails, naming the near-zero (0.0px) gap. Rule restored and reverified
green before continuing.

## Full verification gate (verify.yml order), repo root

All commands run in order, never `wails build` concurrent with a frontend suite.

```
$ wails build
...
Built '.../build/bin/fairdrop.app/Contents/MacOS/fairdrop' in 13.476s.

$ git checkout -- frontend/wailsjs
(git status shows only the 4 intended source files touched)

$ gofmt -l .
(no output)

$ go vet ./...
(no output)

$ go tool staticcheck ./...
(no output)

$ go test -count=1 ./...
ok  	fairdrop	1.590s
ok  	fairdrop/internal/network	0.228s
ok  	fairdrop/internal/qr	0.769s
ok  	fairdrop/internal/server	5.064s
ok  	fairdrop/internal/source	1.300s
ok  	fairdrop/internal/stream	3.874s
ok  	fairdrop/internal/transfer	1.734s
ok  	fairdrop/scripts	0.592s
ok  	fairdrop/scripts/mutationverdict	1.597s

$ go env CGO_ENABLED
1
$ CGO_ENABLED=1 go test -count=1 -race ./...
ok  	fairdrop	8.139s
ok  	fairdrop/internal/network	1.828s
ok  	fairdrop/internal/qr	2.110s
ok  	fairdrop/internal/server	6.328s
ok  	fairdrop/internal/source	2.869s
ok  	fairdrop/internal/stream	100.355s
ok  	fairdrop/internal/transfer	2.903s
ok  	fairdrop/scripts	1.261s
ok  	fairdrop/scripts/mutationverdict	1.991s

$ cd frontend && npm test
 Test Files  19 passed (19)
      Tests  794 passed (794)

$ npm run test:browser
 Test Files  2 passed (2)
      Tests  73 passed (73)

$ git checkout -- frontend/browser/captures/
```

This machine is macOS/arm64 with Homebrew Go and Xcode's `clang`/`cc` already on PATH,
so `CGO_ENABLED` was already `1` and no mingw/WinLibs path was needed (that step in
AGENTS.md is Windows-specific).

794 = the pre-existing count plus the 1 new failing-first jsdom test in
`StagedView.test.tsx`. 73 = the pre-existing rendered-Chromium count plus the 1 new
failing-first rendered test in `accessibility.test.tsx`.

## Mutation table

| # | Defect | Mutation | Expectation | Result |
|---|---|---|---|---|
| 1 | Copied label | Remove `event.currentTarget.focus()` from `handleCopy` in `StagedView.tsx` | Fails, naming the missing "Copied" label | **Caught.** `claims Copied after a pointer click that never focused the button first (WebKit)` failed: `Unable to find an accessible element with the role "button" and name "Copied"`; the other 50 tests in the file stayed green |
| 2 | Retry glyph spacing | Remove `.fd-button > .fd-button__glyph { margin-inline-end: var(--spacing-2); }` from `style.css` | Fails, naming the near-zero gap | **Caught.** `keeps the refresh glyph clear of the "Try Again" label at 1024x768` failed: `expected 0 to be greater than or equal to 5` |

Each mutation was applied individually, run against its own scoped test command, its
failure output captured above, then reverted and reverified green before the next
mutation or the full gate.

## Scope check

- Defect 1 touches only `StagedView.tsx`'s `handleCopy` (added a focus call and the
  `MouseEvent` parameter/import) and its own test file. `handleCopyBlur`, the
  crossfade markup, the announcer call, and the rejection path are all unchanged --
  confirmed by the other 50 StagedView tests staying green, including D-114's
  revert-on-blur and the retry-while-focused case.
- Defect 2's CSS rule is scoped with a direct-child combinator specifically so it
  cannot reach Copy Link's or Copied's glyphs (both children of
  `.fd-button__swap-face`, not of `.fd-button`) or the browse pill's chevron (a
  different class, `.fd-browse-trigger__chevron`, untouched). No other button in the
  app has a `.fd-button__glyph` as a direct child today, so no other control's spacing
  changed -- confirmed by the full `npm test` (794) and `npm run test:browser` (73)
  runs staying green with no new failures beyond the one new assertion.

## Anything unsure

- Defect 1's "no new focus ring on a pointer click" reasoning (see the fix section
  above) is argued from how Chromium's and WebKit's focus-visible heuristics are
  documented to behave for a script `.focus()` call on the *same* element a click just
  targeted, not verified against a real WebKit build -- this repo has no automated way
  to drive real WebKit (AGENTS.md: Playwright's WebKit project was tried and rejected
  for exactly this class of check). A person pressing Copy Link on the built macOS
  binary and confirming no stray ring appears would close this out per the "have a
  person press it" rule; that manual check was not performed as part of this session.

## Files changed

- `frontend/src/ui/StagedView.tsx` -- `handleCopy` takes the click event and calls
  `event.currentTarget.focus()` before issuing the clipboard command.
- `frontend/src/ui/StagedView.test.tsx` -- 1 new failing-first test simulating a
  WebKit pointer click (no prior focus).
- `frontend/src/style.css` -- new `.fd-button > .fd-button__glyph { margin-inline-end:
  var(--spacing-2); }` rule, scoped to a direct child so it cannot reach the
  swap-face glyphs.
- `frontend/browser/accessibility.test.tsx` -- 1 new failing-first rendered test
  measuring the gap between Try Again's glyph and its label text.
