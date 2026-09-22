# Evidence: Defect fix — QR drag crash

## The defect

Owner report: "when I drag on the QR code after attaching a file, it sort of just
breaks and the application closes."

## Diagnosis, and where it came from

**This diagnosis is from reading the vendored Wails source, not from an instrumented
reproduction.** The crash is a macOS/Cocoa fatality in a cgo-hosted dependency; neither
`npm test` (jsdom, no drag pasteboard model at all) nor `npm run test:browser` (real
layout, but Chromium, whose default drag-source behaviour differs from WebKit's) can
trigger it, and no automated harness in this repo drives real Cocoa drag-and-drop. Per
`AGENTS.md`'s recorded case of a confident measurement being wrong (macOS WebKit item
5), this is stated as the leading hypothesis with its evidence shown, not as a settled
fact from live reproduction.

The staged QR is rendered as:

```tsx
<img
    className="fd-qr"
    src={`data:image/png;base64,${metadata.qrBase64}`}
    alt={qrAltFor(metadata.name)}
/>
```

(`frontend/src/ui/StagedView.tsx`, before this fix). WebKit treats any `<img>` as a drag
source by default; dragging one puts its `src` on the OS drag pasteboard as an `NSURL`.
For the QR, that URL is `data:image/png;base64,...` — not a file URL.

The vendored Wails native drop handler
(`~/go/pkg/mod/github.com/wailsapp/wails/v2@v2.15.0/internal/frontend/desktop/darwin/WailsWebView.m`,
`performDragOperation:`) reads every `NSURL` off the drag pasteboard and calls
`fileSystemRepresentation` on each one **unconditionally**:

```objc
NSArray<NSURL*> *files = [pboard readObjectsForClasses:@[[NSURL class]] options:@{}];
NSMutableArray *files_strs = [[NSMutableArray alloc] init];
for (NSURL *url in files) {
    const char *fs_path = [url fileSystemRepresentation];
    NSString *fs_path_str = [[NSString alloc] initWithCString:fs_path encoding:NSUTF8StringEncoding];
    [files_strs addObject:fs_path_str];
}
```

`fileSystemRepresentation` on a non-file URL does not return a usable path — it can
raise, or yield `NULL`, which then feeds `initWithCString:`. An Objective-C exception
inside a cgo process is fatal, consistent with the app vanishing with no Go panic and no
crash report. `disableWebViewDragAndDrop` sits *after* this loop, so no Wails option
guards it, and `EnableFileDrop` cannot be disabled — it is the product's core input
path (`AGENTS.md`, "Native drop targets" convention), so this loop cannot be avoided by
configuration.

Nothing found during this fix contradicts the diagnosis; no alternative cause turned up
in `frontend/src/ui/StagedView.tsx`, `frontend/src/style.css`, or the rest of the
frontend that would better explain a full-process crash on a QR drag.

## The fix

`frontend/src/ui/StagedView.tsx`: `draggable={false}` added to the QR `<img>`.

`frontend/src/style.css` (`.fd-qr`): `-webkit-user-drag: none;` added.

Both are needed ("belt and braces"): `draggable={false}` is the standard HTML attribute
WebKit's own drag-start check consults; `-webkit-user-drag: none` is the CSS property
WebKit actually honours for images specifically. Neither one on its own is redundant on
real WebKit, even though the rendered suite here (Chromium) cannot tell them apart once
both are present — see "Why two mechanisms, and why one test can't see both" below.

## Audit: other drag hazards in the app

Searched the whole `frontend/src` tree (`.tsx`/`.ts`) for anything that could put a URL
on a drag pasteboard the way the QR image did:

- **`<img>` elements:** exactly one in the entire frontend — the QR image in
  `StagedView.tsx`, now fixed. No other `<img>` exists anywhere in the app.
- **Anchors (`<a href>`):** none exist in the app. `StagedView.tsx`'s own doc comment
  states this is deliberate — "There is no anchor, because a sender-side activation
  link would let this window consume its own one-shot download" — so this hazard class
  has no instances to fix.
- **Inline `<svg>`:** one, the outcome checkmark icon in `OutcomePanel.tsx`. Not a
  hazard: only `<img>` and `<a href>` are draggable by default per the HTML spec; an
  inline `<svg>` with no `draggable` attribute is not, and it carries no backing
  resource URL to put on a pasteboard even if it were made draggable.
- **The readonly URL `<textarea>`** (`.fd-url`, `StagedView.tsx`): deliberately left
  alone. A `<textarea>` is not a default drag source (form controls aren't in the
  browser's default-draggable list), and the hazard here is specifically an `NSURL`
  landing on the pasteboard from a resource-backed element (`src`/`href`). Selecting
  and dragging *text* out of the field puts string/text pasteboard types there, not a
  URL type, so it does not exercise the vulnerable `NSURL` loop in
  `performDragOperation:`. The sender may legitimately want to drag-select or copy the
  capability URL from this field (`onMouseDown`/`onFocus` in the component already
  support select-on-focus for exactly this), so it is left interactive on purpose.
- **`data:`/`http:` strings elsewhere:** `grep` for `src=`, `href=`, `data:`, `http://`,
  `https://` across `frontend/src` turned up only the QR `src` and the URL textarea's
  `value`, both covered above.

Conclusion: the QR image was the only drag hazard in the app. No other element needed a
guard, and none was blanket-disabled.

## Test-first: failing output captured before the fix

Two assertions were written, each in the suite that can actually evaluate it:

1. **Rendered (Chromium), `frontend/browser/accessibility.test.tsx`**, new describe
   block `"QR drag source is disabled (macOS drag-and-drop crash)"`: renders
   `StagedView`, asserts `qr.draggable === false` and that the QR's computed
   `-webkit-user-drag` is `none`.
2. **jsdom (stylesheet text), `frontend/src/ui/styles.test.ts`**, new describe block
   `"the QR image cannot be dragged (macOS drag-and-drop crash, defect fix)"`: asserts
   the `.fd-qr` rule in `style.css` contains the literal `-webkit-user-drag: none;`.

### Why two mechanisms, and why one test can't see both

While writing the mutation table it was found that **Chromium derives
`-webkit-user-drag` from the `draggable` attribute itself** when no CSS rule sets it —
confirmed by rendering a bare `<img draggable={false}>` with *no stylesheet at all* and
reading `getComputedStyle(...).getPropertyValue('-webkit-user-drag')`, which came back
`"none"`. That means once `draggable={false}` is on the element, the rendered suite's
computed-style check for `-webkit-user-drag` can no longer distinguish "the CSS rule is
present" from "the CSS rule is absent, but Chromium filled it in anyway from the
attribute." This is exactly the kind of engine-specific quirk `AGENTS.md`'s macOS
section is about (Chromium behaves correctly/differently from WebKit and hides a gap):
here it hides the CSS declaration's own mutation from the one browser the rendered
suite runs. The CSS declaration is therefore pinned as stylesheet **text**
(`styles.test.ts`) instead, following this repo's existing pattern (e.g. the
forced-colors QR exemption, also pinned as text) for exactly this reason.

### Failing-test output, captured on the pre-fix tree

Rendered suite, before the fix (`draggable={false}` and `-webkit-user-drag: none`
absent):

```
$ npm run test:browser -- accessibility.test.tsx -t "QR drag source"
 ❯ |chromium| browser/accessibility.test.tsx (18 tests | 1 failed | 17 skipped)
     × marks the QR image non-draggable both ways, so WebKit never starts a drag pasteboard

AssertionError: the QR <img> is draggable -- dragging it can crash the app
(fileSystemRepresentation on a non-file NSURL in WailsWebView.m performDragOperation:);
set draggable={false}: expected true to be false

 Test Files  1 failed (1)
      Tests  1 failed | 17 skipped (18)
```

(The jsdom stylesheet-text test was added after the CSS mutation was found to be
invisible to the rendered suite; see the mutation table below for its failing output on
the pre-fix stylesheet.)

## Mutation table

All three combinations run against **both** tests, restoring the fix in between each.

| Mutation | Rendered (`accessibility.test.tsx`, "QR drag source…") | jsdom stylesheet text (`styles.test.ts`, "the QR image cannot be dragged…") |
|---|---|---|
| Baseline (fix in place) | **pass** | **pass** |
| Remove `draggable={false}` only | **fail**, names the crash (`the QR <img> is draggable -- dragging it can crash the app (fileSystemRepresentation on a non-file NSURL in WailsWebView.m performDragOperation:)`) | pass (CSS rule untouched) |
| Remove `-webkit-user-drag: none;` only | pass (Chromium fills in `none` from the `draggable` attribute regardless — the exact quirk above) | **fail**, names the crash (`expected .fd-qr to declare -webkit-user-drag: none; -- without it, dragging the QR image can crash the app (fileSystemRepresentation on a non-file NSURL in WailsWebView.m performDragOperation:)`) |
| Remove both | **fail**, names the crash | **fail**, names the crash |

Every mutation is caught by at least one of the two suites, and every failure names the
crash it prevents. Together the two tests pin both halves of the "belt and braces" fix;
neither test alone would.

## Full gate, run in `verify.yml` order

All commands below were run from the repository root (or `frontend/` where noted) on
the fix branch, after the mutation table above and with the fix restored.

```
$ wails build
...
Built '/Users/jaesonmartin/Projects/FairDrop/build/bin/fairdrop.app/Contents/MacOS/fairdrop' in 7.901s.

$ gofmt -l .
(no output)

$ go vet ./...
(no output)

$ go tool staticcheck ./...
(no output)

$ go test -count=1 ./...
ok  	fairdrop	1.465s
ok  	fairdrop/internal/network	0.448s
ok  	fairdrop/internal/qr	0.815s
ok  	fairdrop/internal/server	4.756s
ok  	fairdrop/internal/source	1.186s
ok  	fairdrop/internal/stream	3.778s
ok  	fairdrop/internal/transfer	1.933s
ok  	fairdrop/scripts	1.318s
ok  	fairdrop/scripts/mutationverdict	1.476s

$ go env CGO_ENABLED
1
$ CGO_ENABLED=1 go test -count=1 -race ./...
ok  	fairdrop	8.021s
ok  	fairdrop/internal/network	1.704s
ok  	fairdrop/internal/qr	2.870s
ok  	fairdrop/internal/server	5.664s
ok  	fairdrop/internal/source	1.999s
ok  	fairdrop/internal/stream	102.506s
ok  	fairdrop/internal/transfer	2.382s
ok  	fairdrop/scripts	2.227s
ok  	fairdrop/scripts/mutationverdict	2.753s

$ cd frontend && npm test
 Test Files  17 passed (17)
      Tests  666 passed (666)

$ npm run test:browser
 Test Files  2 passed (2)
      Tests  24 passed (24)

$ cd .. && GOOS=windows GOARCH=amd64 go build ./...
(no output, exit 0)
```

Counts match the expected baseline plus the one new test in each suite: jsdom 665 → 666,
Chromium 23 → 24.

Pre-flight cross-builds (not part of `verify.yml`, but recommended in `AGENTS.md`
before pushing):

```
$ GOOS=darwin GOARCH=arm64 go build ./...
(no output, exit 0)
$ GOOS=linux GOARCH=amd64 go build ./...
(no output, exit 0)
```

## Housekeeping

- `wails build` did not flip the permission bits on `frontend/wailsjs/go/main/App.d.ts`
  or `frontend/wailsjs/go/main/App.js` from their prior committed mode in a way that
  left a content diff; `chmod 644` was applied to those two plus
  `frontend/wailsjs/go/models.ts` as a precaution, and `git diff` over all three showed
  no changes to commit.
- `npm run test:browser` regenerated
  `frontend/browser/captures/qr-panel-forced-colors.capture.png` as expected (PNG
  encoding is not byte-deterministic); reverted with
  `git checkout -- frontend/browser/captures/` per `AGENTS.md`.
- No test-only instrumentation (debug assertions, console logging) was left in the tree
  before the gate ran; the ad hoc Chromium probes used to find the
  `-webkit-user-drag`/`draggable` interaction were written to a throwaway file
  (`browser/debug.test.tsx`) and deleted immediately after each check, never committed.

## Files changed

- `frontend/src/ui/StagedView.tsx` — `draggable={false}` on the QR `<img>`, plus a
  comment pointing at this evidence file and the pinning tests.
- `frontend/src/style.css` — `-webkit-user-drag: none;` on `.fd-qr`, plus a matching
  comment.
- `frontend/browser/accessibility.test.tsx` — new rendered test pinning the `draggable`
  attribute and computed `-webkit-user-drag`.
- `frontend/src/ui/styles.test.ts` — new stylesheet-text test pinning the CSS
  declaration, needed because the rendered suite's Chromium engine cannot distinguish
  its removal once the `draggable` attribute is also present.
- `AGENTS.md` — recorded as platform fact 6 in the macOS WebKit differences section,
  with the upstream `WailsWebView.m` excerpt, so a future Wails upgrade can be checked
  against this exact loop.
