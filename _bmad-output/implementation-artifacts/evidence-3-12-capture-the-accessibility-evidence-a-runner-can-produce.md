# Evidence: Story 3.12: Capture the Accessibility Evidence a Runner Can Produce

Baseline: `cb4c9e9`. Native gate: run
[34796962306](https://github.com/jaeson-sandbox/FairDrop/actions/runs/34796962306) at `b8fb09a` —
`verify (windows-latest)`, `verify (macos-latest)` and `Linux adapter verification` all **success**,
read with `gh run view --json conclusion,jobs`.

## What the owner decided, and when

**2026-09-14, at Checkpoint 1.** Vitest browser mode with a Playwright provider, over a standalone
Playwright suite and over leaving these as human rows. The component states this needs are mounted
directly rather than navigated to, and the accessibility floor being proved against stylesheet text
is the gap the Epic 1 retrospective named.

**2026-09-14, mid-implementation.** The review found that `EXPERIENCE.md`'s assistive progress
throttle reads per-mode while the code has always applied both thresholds in both modes. Two
readings, one of them shipped for two epics. The owner chose to keep the code and correct the spine:
a known total that is large and slow gains bytes without gaining percentage points, and the
five-second floor already caps how often the byte threshold can speak.

## The suite, and what makes it worth its browser

`frontend/browser/accessibility.test.tsx` renders Staged and Transferring in real Chromium and
measures four things jsdom cannot evaluate at all: 320-pixel reflow, 200% text, the 44x44
activation floor, and forced colors. It is a second Vitest project reached only through
`npm run test:browser`, whose config is deliberately not named `vitest.config.ts` and not wired into
`vite.config.ts`'s `test.projects` — either would let a bare `vitest run` pull a browser into the
one command that has to keep working unchanged.

Every failure names the element and the measurement. `styles.test.ts` stays exactly as it was: it
runs everywhere in milliseconds and catches a token edit before this suite's browser launches.

**The 200% case really is 200%.** `doubleTextTokens` reads the eight `--text-*` custom properties
back out of the live document and doubles them, rather than re-asserting the token values
`styles.test.ts` already pins from `DESIGN.md`. Instrumented to confirm it moves layout rather than
only the stylesheet: the Staged `h1` renders at 24px before and 48px after.

**One of its checks examines nothing today, and says so.** With the three deliberate clips excluded
(`.fd-clamp`, `.fd-status-announcer`, `.fd-visually-hidden`), the views render no element whose
computed overflow is hidden, so the fixed-height clip loop reaches its expectation zero times. That
is the correct answer and not a passing test. It is a standing guard, it is proved armed by
mutation M4 below, and the comment says outright that its silence is not evidence.

## The forced-colors check cannot pass by not emulating

`forced-colors` is not on the cross-provider `page` surface, so the suite drives it through the
Playwright provider's raw CDP session. An emulation that silently stopped working would make every
negative assertion trivially true — which is the shape of vacuity this project has been bitten by
before. It cannot happen here, because the same test asserts the positive half: `.fd-qr-panel` and
`.fd-qr` must compute `forced-color-adjust: none`, and that declaration exists only inside the
`@media (forced-colors: active)` block. Mutation M6 proves it: a no-op emulation fails on the QR
panel reading `auto`.

## A capture is not a scan

`frontend/browser/captures/qr-panel-forced-colors.capture.png` is retained, named "capture", and
shows the substrate still painting light-on-dark with its quiet zone distinct under forced colors.
Its bitmap is a synthetic finder-pattern grid drawn with Canvas, which encodes nothing and decodes
to nothing — the panel's styling is the subject, not the payload.

`DESIGN.md` gates the exemption on *native scan evidence*, and that gate is still unmet. It is now
release-evidence row 14, optional and unverified, with what would have to be observed; and the
limitations table names it as the one entry no story owns, because it needs a person with a phone.
Nothing in the repo claims a QR code was scanned.

## D-109: two review layers, two epics late

Both missing layers ran against Story 1.10's change, and every finding was revalidated against
current code rather than against the diff — two epics of other work had moved underneath it. It
earned its cost four times over.

| What Blind Hunter found | Verified how | Fixed |
|---|---|---|
| `aria-disabled` dimmed the quiet Cancel button with `opacity: 0.7` | Recomputed both composites from the declared tokens | The dimming is gone; `styles.test.ts` refuses a fractional opacity anywhere |
| The clipboard rejection was discarded by `StagedView` | Read the handler: `, () => undefined)` | A `clipboard-failed` action carries it to the reducer, scoped to the session that issued it |
| `Copied` outlived the attempt it described | Rendered a success, then a rejection | The confirmation clears before each attempt |
| A copy resolving behind a cancel replaced "Canceling…" | `announceFromStaged` guarded only phase and session | It refuses a retiring session, as `active-cancel-failed` does from the other side |
| `EXPERIENCE.md`'s throttle reads per-mode | Compared the sentence with `isMeaningfulChange` | Owner decision: the spine is corrected and pinned to the code |

**The contrast one is the story's own subject.** `.fd-button--quiet` is muted on elevated, published
at 4.504478335:1 — and at 0.7 that composites to 2.6447:1 light and 4.0133:1 dark, on the one label
that says a cancellation is in progress. No fraction could have cleared 4.5:1: the light pair has
four thousandths of headroom at full strength. `DESIGN.md`'s palette table had already settled it in
one line — *"Muted is readable copy, never disabled text."* The contrast table's premise is "every
authored pair the views actually place together", and an opacity composites one of those pairs at
render time against whatever is behind it, so the pair the user reads is not the pair the table
publishes and no token-level assertion can see the difference. The state is carried instead by the
label change, the cursor, and the hover rule that pins the control to its resting appearance.

**Two findings needed no change, which is also a result.** The transition effect's missing
dependency array is deliberate — it observes commits rather than state, and returns early when the
state is unchanged, which is what makes it a transition observer at all. D-059's dead terminal
control was already fixed earlier in this story.

## The stylesheet comment that described a different element

`.fd-url`'s block comment called it "a readonly `<input>`" three lines above an inner comment
explaining why it is a textarea, and claimed `min-inline-size: 0` was load-bearing: without it "the
URL field forces a page-level horizontal scrollbar at the 320px reflow floor."

Measured, three ways: removing `min-inline-size: 0`, removing `.fd-direct-row`'s `minmax(0, 1fr)`,
and removing both leave all eight rendered checks green. `overflow-wrap: anywhere` gets there first
— it lets the field's min-content width fall to a single character, so the capability URL, one
unbroken token, cannot widen the grid track.

Both declarations stay. The spec's Never list forbids loosening a reflow rule because a measurement
disagrees, and they are the standard floors an edit that narrowed the wrapping would land on. What
went is the false claim, replaced by what the measurement actually shows.

## Mutation table

Rendered suite (`npm run test:browser`):

| # | Mutation | Result |
|---|---|---|
| M1 | `--spacing-target-min: 44px` to `30px` | KILLED — `button.fd-button.fd-name-toggle.fd-target "Show full name" is 34.9px tall` |
| M2 | `.fd-button` takes the exemption **inside** the forced-colors block | KILLED — `computes forced-color-adjust: none outside the QR substrate` |
| M3 | `.fd-region { min-inline-size: 400px }` | KILLED — `documentElement.scrollWidth (400px) exceeds clientWidth (320px)` |
| M4 | `.fd-hero__details` gets a fixed block-size and hidden overflow | KILLED — `clips its content at 200% text: scrollHeight (386px) exceeds its own clientHeight (320px)` |
| M5 | `.fd-notice { white-space: nowrap }` | KILLED — both the 320px and the 200% reflow checks, at 940px and 1998px |
| M6 | the CDP forced-colors emulation becomes a no-op | KILLED — `div.fd-qr-panel: expected 'auto' to be 'none'` |

Findings and pins (jsdom, Go):

| # | Mutation | Result |
|---|---|---|
| M7 | `opacity: 0.7` returns | KILLED — `fractional opacity declarations: expected [ '0.7' ] to deeply equal []` |
| M8 | the clipboard rejection is swallowed again | KILLED — three named tests across `StagedView.test.tsx` and `App.test.tsx` |
| M9 | the confirmation is not cleared before an attempt | KILLED — `clears an earlier confirmation when a later write rejects` |
| M10 | the reducer stops refusing a cancelling session | KILLED — `ignores a rejection that lands while the session is being cancelled` |
| M11 | the reducer stops checking the session | KILLED — `ignores a rejection naming a session that is no longer the staged one` |
| M12 | `announceFromStaged` drops the `cancelPending` guard | KILLED — `keeps the cancellation acknowledgement when a copy resolves behind it` |
| M13 | `EXPERIENCE.md` reverts to the per-mode wording | KILLED — `expected … to contain 'in either mode'` |
| M14 | `progressSpeechIntervalMs` to 4 000 | KILLED — `a spelling for 4 seconds: expected undefined to be truthy` |
| M15 | the browser suite moves onto the Linux adapter job | KILLED — `Linux adapter job contains forbidden desktop/frontend step "playwright"` |
| M16 | the Playwright cache key loses its pinned version | KILLED — `a stale cache would be restored across upgrades` |
| M17 | the browser suite moves above `wails build` | KILLED — the step-order pin, and `does not run after wails build finished` |

## What the subagent produced, and what it left

The browser harness was built by a Sonnet implementer. Its report was accurate and, as every
subagent report in this project has been, incomplete. Accepted after M1–M6 above, which are mine;
its own three mutations did not include the two that mattered most — the exemption moved *inside*
the media block, and the emulation itself disabled — and the clip check's zero-element count was
neither measured nor mentioned.

Three gaps closed here rather than filed: the workflow pins (M15–M17), since the spec requires the
browser to be desktop-only, pinned and cached, and never beside `wails build`, and none of those was
expressible from a green suite; the vacuity note on the clip check; and the `.fd-url` disagreement,
which the subagent correctly refused to "fix" by deleting a rule and correctly reported instead.

## Verification

Read stage by stage:

- `gofmt -l .`; `go vet ./...`; `go tool staticcheck ./...` — clean
- `go test -count=1 ./...` — 8 packages ok
- `cd frontend && npx tsc --noEmit -p tsconfig.json` — clean, the browser project included
- `npx vitest run` (jsdom) — 519 passing in 17 files
- `npm run test:browser` (Chromium) — 8 passing
- Native gate: run 34796962306 at `b8fb09a`, three jobs, all success

## Deferrals

One: **D-114**, the Copy button's accessible name after a successful copy, routed to Story 4.1
because every available fix is a design decision the owner holds — swapping the label back needs a
trigger `EXPERIENCE.md` does not sanction, keeping the accessible name while showing "Copied" breaks
label-in-name, and moving the confirmation beside the button changes a control `DESIGN.md` lays out.

D-065, D-068 and D-109 are discharged, with D-065's unscanned half moved to release-evidence row 14,
where an observation nobody owes belongs.
