# Evidence: Story 10.2: Announce a Cancellation with a Sliding Notification

## Summary

The static Idle cancellation summary (`.fd-cancel-summary`, `copy.cancel.won`)
is retired. A cancel-winning reset now moves focus to the Idle heading
(`idle-instruction`) exactly as a retained-outcome Dismiss already does, and
a new, generic `Notification` component (`frontend/src/ui/Notification.tsx`)
mounts alongside it: a translucent, top-centre overlay that slides down into
place, holds for ~4 seconds (paused while the pointer is over it, restarted
by a fresh cancellation while one is showing), then slides up, fades out and
unmounts. The Idle heading's `aria-describedby` points at the notification's
own hidden text while it is mounted, so the one focus move still announces
"Drop one file or folder" plus "Transfer canceled. Ready for another file or
folder." as a single owner -- the same pattern Staged already uses for
warnings, per the story's routing amendment.

The notification is not interactive (no controls, no focus, not
`role="alert"`, not a live region) and its own dismissal timer is
presentation-only: it may only unmount the notification and does nothing
else -- never a reducer dispatch, a bound Go command, or a focus move. This
is the one narrow exception `EXPERIENCE.md`'s "no frontend lifecycle/reset
timers" ban now carves out, recorded in the same commit as the code per the
story's own instruction.

## Spine amendments (landed in the same commit as the code)

All three amendments the story named, in `EXPERIENCE.md`
(`_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/EXPERIENCE.md`):

1. **Interaction Primitives' banned list** ("frontend lifecycle/reset
   timers") amended to permit exactly one kind of frontend timer: a
   presentation-only dismissal timer for a transient notification, scoped to
   "may remove the notification and do nothing else." Every other frontend
   timer this line has ever banned stays banned; the backend's three-second
   terminal lease is explicitly unaffected.
2. **Copy Feedback row** note added: "no toast" still holds for copy
   feedback specifically, and the cancellation notification is the one
   sanctioned transient notification in the product -- not an exception to
   that row, a separate thing entirely.
3. **Announcement ownership table's Cancel-winning reset row** changed from
   "Focused Idle cancellation summary" to "Focused Idle heading... described
   by the sliding notification's text via `aria-describedby` while it is
   mounted." The `cancelled`-code table row, the Cancel race section, Flow 3,
   and a new **Notification** row in the Component Patterns table were
   updated to match.

`DESIGN.md` gained a **Notification** row in the Components table (material,
geometry, the `@supports`-gated translucent/opaque split) and a Motion
section bullet naming this as the one deliberate exception to "views animate
in and never out" (the notification is not a view -- it owns no phase and
nothing waits to inherit its slot). The Colors section gained a fourth
"unmeasured text background" finding (after the hover fill, `primary-tint`
and the button sheen): the notification's translucent material, with its
four composited contrast figures published verbatim from `styles.test.ts`.

`copy.ts`'s `copy.cancel.won` (a single combined sentence) is replaced by
`copy.cancel.wonTitle` ("Transfer canceled") and `copy.cancel.wonBody`
("Ready for another file or folder.") -- the same two strings, split so the
notification can bold the title and mute the body as DESIGN.md's row
describes. `copy.test.ts`'s spine cross-check (which parses
`EXPERIENCE.md`'s Voice and Tone table directly) required the matching
`EXPERIENCE.md` table edit in the same commit.

## What changed

- **`frontend/src/ui/Notification.tsx`** (new) -- the generic component
  described above: `visible`/`leaving` phase state, an injectable
  `visibleMs`/`exitMs` pair (defaults 4000/260, per `EXPERIENCE.md`'s exact
  numbers) so tests use fake timers rather than real waits, a
  pause/resume pair driven by `onPointerEnter`/`onPointerLeave` that tracks
  the remaining time via a deadline ref, and a visually-hidden `<span>`
  carrying the exact combined sentence `aria-describedby` needs (the visible
  title/body paragraphs are themselves `aria-hidden`, so the sentence is not
  read twice). Exports `NOTIFICATION_TEXT_ID`, the one stable id both
  `App.tsx` and `IdleView.tsx` read.
- **`frontend/src/ui/Notification.test.tsx`** (new) -- 9 cases: full
  lifetime (stays, exits, unmounts), the named mutation (unmount must be
  timer-driven, never `transitionend`), hover pause/resume with correct
  remaining-time math, a hover starting after the exit has already begun
  (not pausable), the "calls onDismiss and nothing else" presentation-only
  proof, no interactive control/live-region/alert, and the combined
  accessible text.
- **`frontend/src/App.tsx`** -- new `cancelNotification` state (`{nonce:
  number} | null`) replaces the old plain `cancelWon` boolean as the source
  of truth (a derived `cancelWon = cancelNotification !== null` keeps
  `IdleView`'s existing prop). The transition effect that already computes
  `routeTransition` on every commit now also decides the notification's own
  lifetime: a `cancel-won` row increments the nonce (mounting a fresh
  `Notification` via `key={nonce}`, which is what "a new cancellation
  replaces it and restarts the timer" reduces to for a component whose own
  effect starts the countdown on mount); leaving Idle (`previous.phase ===
  'idle' && next.phase !== 'idle'`) clears it immediately, covering both
  "starting a Stage" and any other departure from Idle. The `Notification`
  itself renders above the outcome-panel slot, `onDismiss` simply clearing
  the state -- proving the "unmount = clear this state" contract App owns.
- **`frontend/src/ui/IdleView.tsx`** -- the static `.fd-cancel-summary`
  render block is gone. `cancelWon` now only decides whether the Idle
  heading (`data-focus-target="idle-instruction"`) carries
  `aria-describedby={NOTIFICATION_TEXT_ID}`.
- **`frontend/src/ui/announce.ts`** -- the `cancel-won` row's target changed
  from `'cancel-summary'` to `'idle-instruction'` in both places it can be
  produced (`toIdle`'s plain reset case, and the terminal-error-carrying-
  `cancelled` case). `'cancel-summary'` removed from `focusTargets` entirely.
- **`frontend/src/ui/copy.ts`** -- `cancel.won` replaced by `cancel.wonTitle`
  / `cancel.wonBody`.
- **`frontend/src/style.css`** -- `.fd-cancel-summary`/`__icon`/`__text` are
  gone, replaced by `.fd-notification` and its children (see Rendered
  measurements and Contrast figures below for the material itself), plus a
  `--color-notification-surface-translucent` token (`:root` + the existing
  dark-mode `@media` block) and a new `@supports (backdrop-filter: blur(1px))
  or (-webkit-backdrop-filter: blur(1px))` block that layers the translucent
  material over an unconditional opaque `--color-elevated` fallback. A
  forced-colors override restates the material as an opaque system surface
  with a real `outline`.
- Tests updated in place: `App.test.tsx`, `App.focus.test.tsx` (comment
  only -- it stubs `routeTransition` directly and asserts no specific
  target string), `IdleView.test.tsx`, `announce.test.ts`, `copy.test.ts`,
  `styles.test.ts` (new describe blocks plus the pinned `atRules`/
  `componentRules` lists updated for the new `@supports` block),
  `frontend/browser/accessibility.test.tsx` (three new rendered checks).

## Routing change summary

| | Before | After (Story 10.2) |
|---|---|---|
| Cancel-winning reset target | `cancel-summary` (a dedicated, always-focusable static node) | `idle-instruction` (the Idle heading itself) |
| Cancellation text | Rendered permanently in the DOM whenever `cancelWon` was true | Rendered only while the notification is mounted (~4.26s), reached via the heading's `aria-describedby` |
| Terminal error carrying `cancelled` | Same target as above | Same target as above (unchanged relationship, new value) |
| `focusTargets` | Included `'cancel-summary'` | Does not; the row now shares `'idle-instruction'` with the pre-existing `dismiss-retained` row |

Every other row in `announce.ts`'s table is unchanged (verified: `git diff`
against the story branch point touches only the two `cancel-won`
occurrences, the `focusTargets` array, and comments).

## Mutation table

| # | Mutation | Expected failure | Verified |
|---|---|---|---|
| M1 | Rewire the unmount step through a `transitionend` listener instead of a `setTimeout` (`Notification.tsx`) | `Notification.test.tsx`: 3 cases fail (`onDismiss` never called, since jsdom never dispatches a real `transitionend`) | **Pass** -- reproduced live: `unmounts (calls onDismiss)...`, `does nothing on a hover that starts after leaving`, and `calls onDismiss and nothing else` all failed naming `expected 1 times, but got 0 times`; reverted, all 9 green again |
| M2 | Remove the hover-pause branch (`pause()` becomes a no-op) | `Notification.test.tsx`: `pauses while the pointer is over it...` fails | **Pass** -- reproduced live: phase reached `'leaving'` at t=10.5s instead of staying `'visible'`; reverted, all 9 green again |
| M3 | Change `announce.ts`'s `cancel-won` target back to any other value | `announce.test.ts`: 3 cases fail, each naming the wrong target in the diff | **Pass** -- reproduced live with target forced to `'command-error'`; reverted, all 33 green again |
| M4 | Change `.fd-notification`'s `position` from `fixed` to `relative` (`style.css`) | `accessibility.test.tsx`: "leaves the drop zone's own top position unchanged" fails at both 1024x768 and 640x480 | **Pass** -- reproduced live: drop zone moved from 24.0px to 117.0px (displaced by the notification's own height) at both sizes; reverted, all 72 green again |
| M5 | (Implicit, exercised by the existing global bans) Any `@keyframes`/`animation:` on `.fd-notification`, or a second gradient | `styles.test.ts`'s existing whole-file `@keyframes`/`animation:` bans and the exactly-2-gradients count | Not separately reproduced -- these are the same global, already-mutation-proven assertions every other component in the sheet relies on; `.fd-notification` adds no new gradient and no keyframe, so it inherits the existing proof rather than needing a new one |

Every mutation above was applied to the real file, run against the real
suite, confirmed to fail naming the defect, then reverted and confirmed
`diff`-identical to the pre-mutation file before continuing.

## Rendered measurements (Chromium, `frontend/browser/accessibility.test.tsx`)

New describe blocks: "the cancellation notification overlays Idle rather
than displacing it (Story 10.2)".

- **Overlay, not displacement** (1024x768 and 640x480): the drop zone's own
  `getBoundingClientRect().top` is identical (within `toBeCloseTo(..., 0)`,
  i.e. rounds to the same integer pixel) whether the notification is mounted
  above it or not -- 24.0px in both cases at both sizes in the actual run.
  Mutation-verified (M4 above).
- **Horizontally centred** (1024x768 and 640x480): left/right gap to the
  window edge differs by at most 2px.
- **Fits at 640x480**: left edge ≥ 0, right edge ≤ 640, and
  `assertNoHorizontalOverflow` (the suite's existing page-level
  no-horizontal-scroll helper) passes.

All three run against `renderIdleWithNotification`, a new helper that mounts
`<Notification>` beside `<IdleView>` inside the same `.fd-app` stand-in
`renderIdleInAppShell` already uses, in the same document order `App.tsx`
itself renders (notification first, then the phase body) -- so the
measurement reflects the real sibling relationship, not a synthetic
approximation of it.

## Contrast figures (copied verbatim from `styles.test.ts`'s own output)

Computed by the new describe block "the notification's title and body clear
4.5:1 against the material it actually resolves to (Story 10.2)", which
composites the translucent overlay token (`rgb(255 255 255 / 0.75)` light,
`rgb(39 39 43 / 0.75)` dark) over each of the two worst-case backdrops
(`--color-canvas`, `--color-surface`) in both authored modes, using the same
luminance/contrast formula "the unrounded contrast proof" describe block
already uses elsewhere in the file:

| Pair | Composited over | Ratio |
|---|---|---:|
| `text` on the light notification | `canvas` | 16.938172843 |
| `text` on the light notification | `surface` | 17.377657264 |
| `muted` on the light notification | `canvas` | 5.643294609 |
| `muted` on the light notification | `surface` | 5.789717726 |
| `text` on the dark notification | `canvas` | 14.018293623 |
| `text` on the dark notification | `surface` | 13.657025243 |
| `muted` on the dark notification | `canvas` | 5.750604244 |
| `muted` on the dark notification | `surface` | 5.602404218 |

All eight clear 4.5:1 with wide margin in both authored modes. The opaque
`@supports`-unsupported fallback reuses `--color-elevated` directly (already
proven in the pre-existing "unrounded contrast proof" table:
`text`/`elevated` and `muted`/`elevated`, both authored modes, both >4.5),
so no new derivation was needed for that path.

## Gate counts

Run in order, from the repository root unless noted, on this native macOS
darwin/arm64 host:

1. `wails build` -- succeeded (`Built '.../fairdrop.app/Contents/MacOS/fairdrop'`); `git checkout -- frontend/wailsjs` afterward reverted the regenerated bindings (no exported-command surface changed this story, so the diff was empty).
2. `gofmt -l .` -- no output (nothing unformatted).
3. `go vet ./...` -- no output (clean).
4. `go tool staticcheck ./...` -- no output (clean).
5. `go test -count=1 ./...` -- `ok` for all 9 packages (`fairdrop`, `internal/network`, `internal/qr`, `internal/server`, `internal/source`, `internal/stream`, `internal/transfer`, `scripts`, `scripts/mutationverdict`).
6. `CGO_ENABLED=1 go test -count=1 -race ./...` -- confirmed `go env CGO_ENABLED` prints `1` first; `ok` for all 9 packages, none silently skipped.
7. `cd frontend && npm test` -- **20 files, 824 tests passed** (baseline before this story was 20 files / 806 tests; +9 from `Notification.test.tsx` and the rest from expanded cases in `App.test.tsx`/`styles.test.ts`).
8. `npm run test:browser` -- **2 files, 78 tests passed** (baseline 72 in `accessibility.test.tsx`; +6 new rendered cases for this story, `staged-url-field.test.tsx` unchanged).
9. `git checkout -- frontend/browser/captures/` -- not needed: `git status --porcelain frontend/browser/captures/` reported no changes after either browser run in this session.

No Go source changed this story (frontend-only), so the darwin/arm64 native
build and both Go gates above are a control, not new coverage; they are
included because the gate runs unconditionally. As a light pre-flight (not
a substitute for the macOS CI job), `GOOS=linux GOARCH=amd64 go build ./...`
was also run and produced no output.

## Deviations / uncertain

- **Native check: pending -- orchestrator.** No manual phone/browser or
  screen-reader observation was performed in this session, per the story's
  own instruction; automated verification above is what this session
  produced.
- The four rendered contrast composites (`#fcfcfc`, `#ffffff`, `#232326`,
  `#252529`) are close to, but not identical to, `--color-canvas`/
  `--color-surface`/`--color-canvas-dark`/`--color-surface-dark` themselves
  -- expected, since a 75%-opacity near-white/near-`elevated-dark` overlay
  composited over a light/dark canvas necessarily resolves close to white
  (light) or close to the dark overlay's own value (dark), not identical to
  either token. Recorded here in case a reviewer expects the resolved figure
  to exactly match an existing published row; it does not, by construction.
- `Notification.tsx`'s two timers are plain `setTimeout`/`clearTimeout`
  rather than a dedicated injected clock abstraction (unlike, say, the
  backend's `boundTimer` seam) -- "injectable timing" here means the
  *durations* are constructor arguments a test can shrink, combined with
  Vitest's fake-timer support for the underlying global. This matches how
  the story's acceptance criteria phrase the requirement ("implemented with
  injectable timing so tests use fake timers, not real waits") but is a
  narrower mechanism than some other seams in this codebase; flagged in case
  a reviewer expects the stronger pattern.
- The icon glyph is a literal `&times;` character reused from the retired
  `.fd-cancel-summary__icon`, not a new SVG -- kept for continuity and
  because the acceptance criteria did not specify a particular glyph shape,
  only "a small glyph in a tinted disc."
