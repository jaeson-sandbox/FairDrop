# Evidence: Story 7.10 — Make Focus Visible Where the App Moves It

## Summary

The browse menu's items (`frontend/src/ui/IdleView.tsx`'s `BrowseControl`)
carry `tabIndex={-1}` by design (roving tabindex: the menu is one tab stop,
arrows move within it), so they are only ever focused programmatically —
never by a real Tab keypress. WebKit does not match `:focus-visible` for a
script-focused element, so every rule keyed to `:focus-visible` was dead for
these buttons on macOS: the menu opened, and ArrowDown/ArrowUp moved focus
between File and Folder, with **no visible indication of focus at all**.
Escape returning focus to the trigger had the same problem.

Two changes in `frontend/src/style.css`, no change to `BrowseControl`'s
keyboard event handling:

1. **`.fd-browse-menu .fd-button`** — the fill, tint halo, and outline are now
   painted by `:focus`, not `:focus-visible`. Safe specifically for these
   elements: they are reachable only from an open menu, never a bare Tab
   stop, and a pointer press on one activates it and immediately closes the
   menu, so the "ring left painted after a mouse click" problem
   `:focus-visible` exists to solve cannot arise here.
2. **The trigger** (`.fd-button:focus-visible, .fd-url:focus-visible`) gained
   a third selector, `.fd-button[data-focus-return]`, alongside the existing
   `:focus-visible` rule rather than replacing it. `BrowseControl` now tracks
   a `triggerFocusReturned` boolean, set to `true` only inside
   `closeAndReturnFocus` (the function Escape and "an item chosen by
   keyboard" both call) and cleared `onBlur`. The trigger renders
   `data-focus-return` only while that flag is true. The trigger's plain
   mouse-click toggle (`onClick`) never touches the flag, so a mouse click on
   the trigger still relies on `:focus-visible` alone and paints no
   stale ring — the scar this rule must not reintroduce.

Routed landing targets (`[data-focus-target]:focus-visible { outline: none; }`,
the 2026-09-08 amendment) are untouched. The existing
`styles.test.ts` assertion for them (`'never rings a routed landing target,
even when focus on it is visible'`) was run unmodified through every mutation
pass below and stayed green throughout.

## Files changed

- `frontend/src/style.css` — the two rule changes above, each with a comment
  naming the WebKit behaviour and why the fix is safe for that element.
- `frontend/src/ui/IdleView.tsx` — `triggerFocusReturned` state,
  `data-focus-return` attribute, `onBlur` clear. No change to Escape/Tab/
  arrow/blur event handling.
- `frontend/src/ui/styles.test.ts` — mechanism assertions (below).
- `frontend/src/ui/IdleView.test.tsx` — jsdom assertions for the marker
  (below).

## Mutation table

| # | Mutation | Command | Result |
|---|---|---|---|
| 1 | `.fd-browse-menu .fd-button:focus` reverted to `.fd-browse-menu .fd-button:focus-visible` | `npx vitest run --run src/ui/styles.test.ts` | **2 tests fail**: `'keys the menu item focus rule to :focus, never :focus-visible...'` (asserts `stylesheet` contains the `:focus` selector and not the `:focus-visible` one — names the WebKit consequence in its own body) and `'distinguishes the focused item by primary fill, a tint halo, and its own ring...'` (its `block('.fd-browse-menu .fd-button:focus {')` throws `not found`, since the block no longer exists at that selector). Confirmed, then reverted. |
| 2 | `.fd-button[data-focus-return]` removed from the shared ring-rule selector list | `npx vitest run --run src/ui/styles.test.ts` | **3 tests fail**: `'draws one ring from the focus token...'` (selector regex no longer matches), the new `'rings the browse trigger on its scripted-return marker...'` test, and `'leaves the shared ring rule scoped to the ordinary controls...'`. Confirmed, then reverted. |
| 3 | `[data-focus-target]:focus-visible { outline: none; }` changed to paint a ring | `npx vitest run --run src/ui/styles.test.ts` | **1 test fails**: the pre-existing `'never rings a routed landing target, even when focus on it is visible'` — unmodified by this story, still catches the mutation. Confirmed the 2026-09-08 amendment survives this change, then reverted. |
| 4 | `setTriggerFocusReturned(true)` call removed from `closeAndReturnFocus` | `npx vitest run --run src/ui/IdleView.test.tsx` | **3 tests fail**: `'returns focus to the control once a kind is chosen'`, `'closes on Escape, returns focus to the control, and announces nothing'` (both now see `data-focus-return` as `null` instead of `''`), and `'clears the trigger scripted-return marker on blur'`. Confirmed, then reverted. |

All four mutations were applied one at a time, each confirmed to fail with a
message naming the consequence, then reverted before the next; `git diff
--stat` was checked clean against the fixed state after each revert.

## New/changed tests

`frontend/src/ui/styles.test.ts` (`describe('the focus indicator')` and
`describe('the browse menu surface (Story 7.2)')`):

- `'rings the browse trigger on its scripted-return marker (Story 7.10)...'`
  — asserts the shared ring rule's block contains `[data-focus-return]` and
  does not contain a bare `.fd-button:focus` (guards against the trigger
  quietly gaining the same blanket-`:focus` treatment the menu items use,
  which the story explicitly forbids).
- `'distinguishes the focused item by primary fill, a tint halo, and its own
  ring -- keyed to :focus (Story 7.10)'` — replaces the old
  `:focus-visible`-keyed version; now also asserts the outline declarations
  live inside this block, not only inherited from the (on WebKit, dead)
  shared rule.
- `'keys the menu item focus rule to :focus, never :focus-visible...'` — the
  mechanism pin named in the acceptance criteria. Its body states the WebKit
  fact and the observed consequence in prose, so a future revert explains
  itself to whoever reads the failure.
- `'leaves the shared ring rule scoped to the ordinary controls, not folded in
  with the menu items'` — guards against the opposite mistake (re-merging the
  two rules into one selector list).

`frontend/src/ui/IdleView.test.tsx` (`describe('the browse menu')`):

- Extended `'returns focus to the control once a kind is chosen'` and
  `'closes on Escape, returns focus to the control, and announces nothing'`
  to also assert `trigger.getAttribute('data-focus-return') === ''`.
- `'never marks the trigger scripted-return for its own plain mouse-click
  toggle'` — a bare `fireEvent.click` on the trigger must never set the
  marker.
- `'clears the trigger scripted-return marker on blur'`.

## Full gate (macOS, this machine)

Run in the `.github/workflows/verify.yml` order, from a clean `epic-7-quartz`
checkout with only this story's changes staged.

```
$ wails build
...
Built '/Users/jaesonmartin/Projects/FairDrop/build/bin/fairdrop.app/Contents/MacOS/fairdrop' in 9.316s.
```
(regenerated `frontend/wailsjs/go/main/App.d.ts`, `App.js`, `models.ts` with
mode `100755`; `chmod 644` applied to all three per the known pitfall — `git
status` then showed no diff on those three files, content-identical.)

```
$ gofmt -l .
(no output)

$ go vet ./...
(no output)

$ go tool staticcheck ./...
(no output)

$ go test -count=1 ./...
ok  	fairdrop	1.764s
ok  	fairdrop/internal/network	0.229s
ok  	fairdrop/internal/qr	0.416s
ok  	fairdrop/internal/server	5.066s
ok  	fairdrop/internal/source	0.787s
ok  	fairdrop/internal/stream	3.911s
ok  	fairdrop/internal/transfer	1.737s
ok  	fairdrop/scripts	1.277s
ok  	fairdrop/scripts/mutationverdict	1.603s

$ go env CGO_ENABLED
1

$ CGO_ENABLED=1 go test -count=1 -race ./...
ok  	fairdrop	8.924s
ok  	fairdrop/internal/network	1.406s
ok  	fairdrop/internal/qr	1.705s
ok  	fairdrop/internal/server	5.940s
ok  	fairdrop/internal/source	2.118s
ok  	fairdrop/internal/stream	101.104s
ok  	fairdrop/internal/transfer	2.939s
ok  	fairdrop/scripts	2.427s
ok  	fairdrop/scripts/mutationverdict	2.773s

$ npm test -- --run          # frontend/, jsdom
Test Files  17 passed (17)
     Tests  637 passed (637)     # 632 baseline + 5 new (styles.test.ts x4, IdleView.test.tsx x1 net new + 2 extended-in-place)

$ npm run test:browser       # frontend/, rendered Chromium
Test Files  2 passed (2)
     Tests  23 passed (23)       # unchanged from baseline

$ GOOS=windows GOARCH=amd64 go build ./...
(exit 0)
$ GOOS=darwin GOARCH=arm64 go build ./...
(exit 0)
$ GOOS=linux GOARCH=amd64 go build ./...
(exit 0)
```

`wails build` was never run concurrently with a frontend suite. After the
full run, `git status --short` showed only this story's five source/test
files plus the non-deterministic `frontend/browser/captures/
qr-panel-forced-colors.capture.png` re-encode (known pitfall); the capture
was reverted with `git checkout --` before committing, per AGENTS.md.

## Attempting a WebKit project for the rendered suite

Playwright's WebKit build downloads cleanly (`npx playwright install
webkit`, 78.1 MiB, WebKit 26.6 / playwright build v2359) and
`@vitest/browser-playwright` (already the provider here, v4.1.11) accepts
`instances: [{browser: 'chromium'}, {browser: 'webkit'}]` in
`vitest.browser.config.ts` without complaint — adding a WebKit project is
mechanically possible.

Running the existing `frontend/browser/accessibility.test.tsx` against it
does not work, for two independent reasons, one shallow and one deep:

**1. Shallow: every test in the file fails under WebKit, not just the
forced-colors ones.** The file's top-level `afterEach` unconditionally calls
`setForcedColorsActive(false)`, which calls `cdp()` — a Playwright CDP
session — to reset the forced-colors emulation after *every* test, including
the fifteen that never touch forced colors at all:

```
Error: browserContext.newCDPSession: CDP session is only available in Chromium
 ❯ PlaywrightBrowserProvider.getCDPSession node_modules/@vitest/browser-playwright/dist/index.js:1179:36
```

All 17 tests in the file fail (0 pass) under the added `webkit` instance;
the same 17 all pass under `chromium` in the same run. This is fixable in
principle (guard the `afterEach` on the active browser, or give
forced-colors its own file), but it is a change to shared rendered-suite
infrastructure well beyond this story's scope, and forced-colors is not
what this story is about.

**2. Deep, and the reason a WebKit project would not prove the right thing
even after fixing (1): Playwright's bundled WebKit does not reproduce the
same Tab-focus-on-buttons default the real, embedded WKWebView Cocoa API
Wails uses on macOS has.** DESIGN.md and `main.go` document that
`WKPreferences.tabFocusesLinks` defaults to `NO` on macOS WebKit, which is
why `main.go` sets `Mac.Preferences.TabFocusesLinks` and `main_test.go` pins
it — without it, Tab cannot reach the trigger `<button>` at all. Probed
directly against a bare HTML page in both engines via the Playwright API
(script in `frontend/probe2.mjs`, run and discarded — not committed):

```
chromium: trigger focused=trigger visible=true;  item1 focused=item1 visible=true
webkit:   trigger focused=       visible=false;  item1 focused=       visible=false
```

Chromium's Tab keypress focuses the button and matches `:focus-visible`, as
expected. WebKit's Tab keypress does not focus the button at all —
`document.activeElement` stays empty — matching the documented
`tabFocusesLinks: NO` default. Playwright's WebKit browser context exposes
no equivalent of `Mac.Preferences.TabFocusesLinks` (that is a Wails/WKWebView
Cocoa-embedding option, not a generic WebKit browser preference), so there is
no way through this provider to reproduce the "real Tab lands on the trigger
and paints a ring" half of the defect at all — only the "scripted focus
paints nothing" half could be exercised, and even that would need a
synthetic page rather than the real rendered component, since real Tab can
never reach the button to set up the comparison.

A separate, narrower probe (script focus only, no real Tab) did confirm the
core mechanism the fix relies on: a `tabindex="-1"` button given
`element.focus()` from a keydown handler matches `:focus-visible` in
Chromium and does not match it in WebKit, in two independent probe runs
against `webkit.launch()` directly (not through the vitest provider). This
is consistent with the defect as described and with `:focus` firing in both
engines regardless. It was not written up as a committed test file, for two
reasons: it duplicates what the engine-independent stylesheet-text mechanism
test (`'keys the menu item focus rule to :focus, never :focus-visible...'`
in `styles.test.ts`) already pins in a way every engine runs, and a rendered
WebKit assertion built on a synthetic page rather than the real
`BrowseControl` would not actually be testing this component.

**Conclusion: a WebKit project was not added to the tracked suite.** Making
the *existing* rendered file WebKit-clean needs an infrastructure change
(splitting the CDP-only `afterEach`) outside this story's scope, and even
with that fixed, Playwright's WebKit cannot reproduce the
`tabFocusesLinks`-gated real-Tab behaviour that is half of this specific
defect, so it would not be a faithful proxy for the shipped WKWebView either
way. This is reported plainly rather than left unstated, per the story's
acceptance criterion. **What replaces it:** the engine-independent
stylesheet-text mechanism test in `styles.test.ts`, which pins the `:focus`
vs `:focus-visible` selector choice directly and fails, naming the WebKit
consequence, the moment anyone reverts it — plus the manual observation
below.

`vitest.browser.config.ts` and `playwright.config` files are unchanged in
the final diff; the WebKit browser binary that was downloaded for this
investigation (`npx playwright install webkit`) is a local cache artifact
under `~/Library/Caches/ms-playwright/`, not part of the repository.

## Manual macOS observation

The defect this story fixes was originally found and recorded by the owner
by hand on the built `fairdrop.app` binary (see the story's "What was
observed" text in `epics.md`, dated 2026-09-21): opening the browse menu
with Tab + ArrowDown showed no focus indication on either menu item, and
Escape returned focus to the trigger with no ring either.

This session rebuilt the app (`wails build`, above) with the fix applied and
did **not** perform an independent interactive re-observation of the rebuilt
binary — doing so would require this session to drive the user's desktop
through an interactive consent flow, which was not exercised here. Per
`docs/release-policy.md` ("record which tests ran, retain failing output,
and label unobserved native UI/browser behavior as unverified... never
invent a manual pass"), this is recorded honestly as **not independently
observed by hand in this session**, rather than claimed. The automated
evidence above (mutation-proven CSS mechanism, a real-engine WebKit probe
confirming the `:focus`/`:focus-visible` split the fix relies on, and the
full green gate) is what stands in its place; a quick manual Tab +
ArrowDown check on the rebuilt `build/bin/fairdrop.app` is the natural next
step for the owner to close the loop the same way the defect was originally
found.

## Contradictions of, or additions to, the diagnosis in the story text

None found. The story's mechanism claim (WebKit does not match
`:focus-visible` for a script-focused element; Chromium does) was
independently reproduced against a real WebKit engine via Playwright, not
just taken on faith. One addition worth recording: the story's `TabFocusesLinks`
reference turned out to be directly relevant to *why* a WebKit rendered
project can't be built here, not only prior art for the pattern of a
macOS-only escape — Playwright's WebKit has no way to reproduce that
preference, which independently blocks reusing the rendered suite for this
specific defect even once the CDP-`afterEach` issue is fixed.
