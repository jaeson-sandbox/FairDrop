# Evidence: Defect fix — Staged direct-URL field clipped its own value

## Summary

Observed on the built binary: in the Staged view, the readonly capability URL
overflows its own box — the bottom line of the URL is clipped by the field's
border. Reproduced instance: `http://192.168.1.168:63367/download/94adac272a8fee62a4436c58a4d4bac6`,
which wraps to three lines at the field's width, inside a `<textarea rows={2}>`
sized for exactly two.

This is the AGENTS.md "Git workflow" defect-fix carve-out: the defect was
already observed, a failing test was written before the fix, and the fix is
mutation-verified below. No story exists for this change; the failing test
replaces the spec.

## Why the fix could not be a `jsdom` unit test

jsdom performs no layout: every element's `scrollHeight` reads `0`, so a test
under `frontend/src/` would pass against both the broken code and the fix,
proving nothing (AGENTS.md, "Testing standards, learned the hard way" — a
test that agrees with the bug). The test instead lives in the rendered
Chromium suite, `frontend/browser/`, which exists precisely for the class of
rule jsdom cannot evaluate — 320px reflow, 200% text, the 44px target floor,
forced colors — and now this: whether a real textarea box is tall enough for
its own value.

New file: `frontend/browser/staged-url-field.test.tsx`. It renders
`StagedView` with three capability URLs — the observed one, a deliberately
longer one (an IPv6 host, which pushes the value further than one lucky
string), and the observed one again at the 320px reflow floor — and asserts
`.fd-url`'s `scrollHeight` does not exceed its `clientHeight` (i.e. nothing is
clipped).

## Failing test output, captured before the fix

Against the code as observed (`rows={2}`, no sizing logic):

```
 RUN  v4.1.11 /Users/jaesonmartin/Projects/FairDrop/frontend

 ❯ |chromium| browser/staged-url-field.test.tsx (3 tests | 2 failed) 48ms
     × fits the observed three-line capability URL with nothing clipped 31ms
     × fits a longer IPv6-host URL that wraps to more lines still 3ms

⎯⎯⎯⎯⎯⎯⎯ Failed Tests 2 ⎯⎯⎯⎯⎯⎯⎯

 FAIL  |chromium| browser/staged-url-field.test.tsx > Staged direct URL field sizes to its content (observed clipping defect) > fits the observed three-line capability URL with nothing clipped
AssertionError: .fd-url clips its value: scrollHeight (76px) exceeds its own clientHeight (59px) for a 68-character URL: expected 76 to be less than or equal to 59.5
 ❯ assertURLFieldFitsItsContent browser/staged-url-field.test.tsx:76:6

 FAIL  |chromium| browser/staged-url-field.test.tsx > Staged direct URL field sizes to its content (observed clipping defect) > fits a longer IPv6-host URL that wraps to more lines still
AssertionError: .fd-url clips its value: scrollHeight (94px) exceeds its own clientHeight (59px) for a 96-character URL: expected 94 to be less than or equal to 59.5
 ❯ assertURLFieldFitsItsContent browser/staged-url-field.test.tsx:76:6

 Test Files  1 failed | 1 passed (2)
      Tests  2 failed | 18 passed (20)
```

(The third case, at the 320px reflow floor, happened to pass against the
unfixed code for that particular width/string combination — which is exactly
why the test suite also covers 1024px and the longer IPv6 host: a single
passing width does not mean the defect is absent, only that this one
combination did not trigger it. Two of three cases failed and named the exact
clipping, which is what "written before the fix, and it fails" requires.)

## The fix

`frontend/src/ui/StagedView.tsx`: a `useLayoutEffect` measures the textarea's
own `scrollHeight` and sets its inline `height` to match, re-running whenever
`metadata.url` changes.

```tsx
const urlFieldRef = useRef<HTMLTextAreaElement>(null)

useLayoutEffect(() => {
    const field = urlFieldRef.current
    if (field === null) return
    field.style.height = 'auto'
    const borderY = field.offsetHeight - field.clientHeight
    field.style.height = `${field.scrollHeight + borderY}px`
}, [metadata.url])
```

`rows={2}` stays on the element as the pre-effect intrinsic size (the
`useLayoutEffect` runs before paint, so there is no visible two-line frame
first); the ref was added to the `<textarea>` and nothing else about the
element — `readOnly`, `value`, `onFocus`, the `onMouseDown` select-on-focus
guard — was touched.

### Why this approach, and why it behaves identically in WKWebView and WebView2

- **`field-sizing: content` was ruled out.** It is Chromium-only; WebKit
  (WKWebView, macOS) does not implement it, so the field would auto-size on
  Windows and keep clipping on macOS — not a fix under DESIGN.md's binding
  cross-platform constraint ("No rule in this document may depend on a
  macOS-only capability" and, symmetrically, WebKit and Blink must look
  identical).
- **`scrollHeight` and inline `style.height` are plain DOM/CSSOM**, not a CSS
  layout feature gated to one engine. Both WebKit and Blink have implemented
  `scrollHeight`, `clientHeight`, and `offsetHeight` identically for as long
  as either has existed; this is the same "reset to `auto`, then set to
  `scrollHeight`" technique used for autosizing textareas since before
  `field-sizing` existed, specifically because it works everywhere. There is
  no capability query, no vendor prefix, and no CSS feature detection
  involved — it is imperative JavaScript reading and writing standard
  properties, so there is no engine-specific branch to diverge.
- **The border compensation is the one place engine differences could have
  crept in**, and it doesn't: `.fd-url` is `box-sizing: border-box`
  (Tailwind's preflight default), so `scrollHeight` (padding + content, no
  border) undershoots the border-box `height` this sets by the vertical
  border width — a small, constant ~2px error, not a whole line, but still a
  clip. `offsetHeight - clientHeight` recovers that border width from the
  box's own rendered geometry rather than a hard-coded pixel figure, so it
  self-corrects for whatever the two engines' border widths and font metrics
  actually render as, instead of assuming they match.
- **No lifecycle timer.** The effect runs once per relevant DOM
  commit/`metadata.url` change, synchronously via `useLayoutEffect` — it is
  not a `setTimeout`/`setInterval` poll, which `EXPERIENCE.md` forbids.

Constraints preserved and re-checked directly against the diff:
- Element is still a readonly `<textarea>` — not swapped for a div with
  `role="textbox"`.
- `select-on-focus` and its `onMouseDown` guard are untouched.
- `overflow-wrap: anywhere`, `.fd-url`'s `min-inline-size: 0`, and
  `.fd-direct-row`'s `minmax(0, 1fr)` in `frontend/src/style.css` were not
  touched — confirmed by the unchanged 320px reflow assertions in
  `frontend/browser/accessibility.test.tsx`, still green (see full suite run
  below), plus the new suite's own 320px case.

## Mutation table

| Mutation | Command | Result |
|---|---|---|
| None (fix in place) | `npm run test:browser` | 20/20 pass (17 pre-existing + 3 new) |
| A: revert to `rows={2}`, no sizing (the observed defect, effect returns before measuring) | `npm run test:browser -- staged-url-field` | **2/3 fail**, naming `.fd-url clips its value: scrollHeight (76px) exceeds ... clientHeight (59px)` and `(94px) exceeds ... (59px)` |
| B: hard-coded `rows={3}`, sizing effect neutered | `npm run test:browser -- staged-url-field` | **1/3 fails** — the observed three-line URL now happens to fit at `rows={3}`, but the deliberately longer IPv6-host URL still clips: `scrollHeight (94px) exceeds ... clientHeight (76px)` — exactly the "a longer host pushes it to four lines and the bug returns" case the task named, caught by the long-URL assertion rather than the observed string alone. |

Both mutations reproduce and are named by the test; the test was restored to
the fixed state afterward and reconfirmed green.

## Full gate, run locally in `verify.yml` order

All commands run from `/Users/jaesonmartin/Projects/FairDrop` (Go) or
`/Users/jaesonmartin/Projects/FairDrop/frontend` (npm), one after another,
never `wails build` concurrent with a frontend suite.

1. `wails build` — succeeded; `Built '.../fairdrop.app/Contents/MacOS/fairdrop'`.
   Regenerated `frontend/wailsjs/go/main/App.d.ts`, `App.js`, `models.ts` at
   mode 755; `chmod 644` applied to all three before committing (content
   unchanged, `git status` showed no diff for them).
2. `gofmt -l .` — no output (nothing to reformat).
3. `go vet ./...` — no output (clean).
4. `go tool staticcheck ./...` — no output (clean).
5. `go test -count=1 ./...` — all packages `ok` (`fairdrop`,
   `internal/network`, `internal/qr`, `internal/server`, `internal/source`,
   `internal/stream`, `internal/transfer`, `scripts`,
   `scripts/mutationverdict`).
6. `CGO_ENABLED=1 go test -count=1 -race ./...` — confirmed `go env
   CGO_ENABLED` prints `1` first; all packages `ok` under the race detector.
7. `npm test -- --run` (jsdom suite) — **622/622 pass**, 17 files.
8. `npm run test:browser` — **20/20 pass**, 2 files (18 pre-existing minus
   nothing removed — accessibility.test.tsx's own count is unchanged at 17;
   the new file adds 3).
9. `GOOS=windows GOARCH=amd64 go build ./...` — succeeded, no output.

Pre-flight (not part of the CI matrix, but named in AGENTS.md "Running and
verifying"):
- `GOOS=darwin GOARCH=arm64 go build ./...` — succeeded.
- `GOOS=linux GOARCH=amd64 go build ./...` — succeeded.

`frontend/browser/captures/qr-panel-forced-colors.capture.png` is regenerated
by every `test:browser` run (a live screenshot, not a fixture); it was
reverted with `git checkout --` before committing since this fix touches
nothing that capture proves.

## Files changed

- `frontend/src/ui/StagedView.tsx` — the fix (see above).
- `frontend/browser/staged-url-field.test.tsx` — new, the failing-test-first
  evidence for this defect.
