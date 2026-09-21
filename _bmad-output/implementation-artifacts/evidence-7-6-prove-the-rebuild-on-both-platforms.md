# Evidence — Story 7.6: Prove the Rebuild on Both Platforms

**Status:** complete except for one named item, recorded below as **not observed**.
**Tip proven:** `dec76ae` (`fix(epic-7): make focus visible where the app moves it`).
**Policy:** `docs/release-policy.md`. Automated verification is mandatory; manual
observations are optional for this personal project, but an unobserved check is
recorded as unobserved and never implied to have passed.

## Method

Verification was performed by the orchestrating session against the **built macOS
binary** (`wails build`, `build/bin/fairdrop.app`), not against a dev server and not
against the test suites alone. Windows evidence comes from the canonical CI gate
running natively on `windows-latest`. Two real transfers were driven end to end by
fetching the capability URL with `curl --limit-rate`, so the progress and completion
states show real numbers rather than fixtures.

## Automated gate on the tip

GitHub Actions run **35600709363**, head `dec76ae`:

| Job | Conclusion |
|---|---|
| `verify (windows-latest)` | success |
| `verify (macos-latest)` | success |
| `Linux adapter verification (not release proof)` | success |

This run's `headSha` **is** the tip. That matters: `verify.yml` sets
`cancel-in-progress: true` on a per-ref concurrency group, so during this epic several
earlier runs completed with `verify (windows-latest)` **cancelled** rather than passed
— including the Story 7.9 run, which finished macOS and Linux green with Windows cut
off. A cancelled job is an absence, not a pass. The pitfall is recorded in `AGENTS.md`.

Local gate at the same commit: `wails build`, `gofmt -l .`, `go vet ./...`,
`go tool staticcheck ./...`, `go test -count=1 ./...`,
`CGO_ENABLED=1 go test -count=1 -race ./...`, `npm test -- --run` (637),
`npm run test:browser` (23), and `GOOS=windows/linux/darwin go build ./...` — all green.

## Observed on macOS, on the built binary

Both colour schemes were observed by switching the OS appearance and relaunching; the
system was returned to the user's original Dark setting afterwards.

| Claim | Observed |
|---|---|
| Idle, dark | yes — drop zone fills the window, no dead space (Story 7.7) |
| Idle, light | yes — neutral canvas, mocha primary, white label |
| Browse menu opens by keyboard | yes — Tab to the control, ArrowDown opens it |
| Browse control is the first tab stop | yes — **one** Tab reaches it, confirming Story 7.8's reorder |
| Staged with a real QR | yes — QR, item name, size, capability URL |
| Direct-URL field unclipped | yes — a URL wrapping to three lines sits fully inside its box, at two window widths |
| Transferring | yes — 34% and 40% captured mid-transfer, wire bytes first, throughput second |
| Done, with the completion receipt | yes — item name and **14.0 MB wire bytes** from retained state (Story 7.4) |
| Retained outcome composes above Idle | yes |
| Transfer correctness | yes — SHA-256 of the received bytes matched the source on **both** runs |
| No first-paint flash | yes, in both schemes (`BackgroundColour` tracks `--color-canvas`) |
| Violet focus ring distinct from the mocha action colour | yes — a real Tab paints it clearly on the trigger |
| Window resize | yes — adapts across widths with no clipping and no overlap |

## Proven mechanically rather than by eye

The rendered Chromium suite (`npm run test:browser`, 23 tests) covers what a screenshot
would assert less reliably, and re-runs on every build:

- The field stays unclipped **when the whole window narrows to the 640x480 minimum**
  after mount, and at the 320 CSS pixel reflow floor.
- A width sweep from 1200px to 320px in steps of 40 asserts no clipping, no
  page-level horizontal scrollbar and no content overlap at every step (Story 7.9).
- Forced-colors, 200% text zoom, WCAG 1.4.12 text-spacing overrides, and the 44px
  activation-target floor.

The 640x480 minimum was **not** additionally dragged to by hand. The mechanical proof
is stronger than a one-off screenshot would have been, because it runs on every CI build.

## Defects found by this verification pass

Verification found two defects that every suite had been green through. Both are
recorded here because finding them is the story's actual value.

1. **Direct-URL field clipped its own content.** Reported by the owner from a Staged
   screenshot: a `rows={2}` textarea holding a URL that wrapped to three lines. Fixed
   as a defect-fix carve-out (failing test first, in the rendered suite because jsdom
   reports `scrollHeight` as 0 and cannot observe it), then a second round added a
   `ResizeObserver` for live resizes, then Story 7.9 replaced the whole JavaScript
   mechanism with CSS. See `evidence-url-field-clipping-fix.md` and
   `evidence-7-9-make-resizing-seamless.md`.
2. **Focus was invisible inside the browse menu on macOS.** Found during this pass.
   WebKit does not match `:focus-visible` for a script-focused element, and the menu's
   items carry `tabIndex={-1}` by design, so every focus rule keyed to it was dead on
   macOS while Chromium — and therefore the rendered suite and Windows — looked
   correct. A WCAG 2.4.7 failure on an operable component. Fixed by Story 7.10. See
   `evidence-7-10-make-focus-visible.md`.

Both are instances of `AGENTS.md`'s first testing lesson: a green test can pin a
behaviour that is dead on a platform you never ran. Both were invisible to a
Chromium-only rendered proof, on a product that ships WKWebView as well.

## NOT observed

- **The Story 7.10 focus fix has not been re-checked by hand on the rebuilt binary.**
  The fix is sound by construction — `:focus` matches a focused element unconditionally,
  with no engine heuristic — and is pinned by five new tests plus a mechanism assertion
  that fails if the rule is returned to `:focus-visible`. But the interactive
  confirmation that the focused menu item now paints its fill, halo and ring was not
  performed: the machine's screen locked partway through this pass, and macOS blocks
  window-level Accessibility actions while locked. **This is recorded as unobserved,
  not as passed.** It requires roughly a minute of unlocked screen to close.
- **No screenshots are retained as files.** The states above were observed live and
  reviewed in-session; the capture tool returned them inline but did not persist them
  to disk, and `screencapture` is blocked by screen-recording permission for the shell.
  No evidence file in this repo cites an image that cannot be opened.
- **Windows was not driven interactively.** No Windows host was available. The
  `verify (windows-latest)` job proves the gate — build, vet, staticcheck, unit tests,
  race tests, frontend suites, line endings — but nobody clicked through the app on
  Windows this pass. Per `docs/release-policy.md` that is permitted and is stated here
  rather than implied.
