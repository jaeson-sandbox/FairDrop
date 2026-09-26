---
name: FairDrop
description: Quartz visual system — neutral materials, one signal colour, and real layered depth for an ephemeral LAN handoff utility.
status: final
created: '2026-09-20'
updated: '2026-09-20'
supersedes: 'ux-FairDrop-2026-08-23 (Paper Relay / Terracotta Linen)'
sources:
  - '{project-root}/_bmad-output/specs/spec-fairdrop/SPEC.md'
  - '{planning_artifacts}/architecture/architecture-FairDrop-2026-08-22/ARCHITECTURE-SPINE.md'
  - '{project-root}/docs/fairdrop-contracts.md'
  - '{planning_artifacts}/epics.md'
colors:
  canvas: '#F2F2F4'
  surface: '#FFFFFF'
  elevated: '#FFFFFF'
  fill: '#F1F1F2'
  fill-strong: '#E8E8EA'
  text: '#1A1A1C'
  muted: '#65656B'
  separator: '#D6D6D6'
  control-border: '#86868B'
  primary: '#9C5636'
  primary-hover: '#7F4428'
  primary-ink: '#FFFFFF'
  primary-tint: '#F5EEEB'
  track: '#E9E9EB'
  focus: '#6B4E9E'
  success: '#177A48'
  success-tint: '#E3F1EA'
  warning: '#8A5300'
  warning-tint: '#F6EDE0'
  error: '#C0362C'
  error-tint: '#F8E9E7'
  canvas-dark: '#161618'
  surface-dark: '#1F1F22'
  elevated-dark: '#27272B'
  fill-dark: '#313135'
  fill-strong-dark: '#3A3A3E'
  text-dark: '#F2F2F4'
  muted-dark: '#9C9CA4'
  separator-dark: '#47474A'
  control-border-dark: '#7A7A82'
  primary-dark: '#E39B70'
  primary-hover-dark: '#F0B694'
  primary-ink-dark: '#2B1206'
  primary-tint-dark: '#372E2B'
  track-dark: '#3A3A3E'
  focus-dark: '#B79BE0'
  success-dark: '#4ED08B'
  success-tint-dark: '#20342A'
  warning-dark: '#E7A33A'
  warning-tint-dark: '#372E1D'
  error-dark: '#FF7A70'
  error-tint-dark: '#3A2422'
  qr-surface: '#FFFFFF'
  qr-ink: '#1A1A1C'
typography:
  display: { fontFamily: '-apple-system, BlinkMacSystemFont, SF Pro Display, Segoe UI Variable Display, Segoe UI, system-ui, sans-serif', fontSize: 26px, fontWeight: '650', lineHeight: '1.2', letterSpacing: -0.022em }
  headline: { fontFamily: '-apple-system, BlinkMacSystemFont, SF Pro Display, Segoe UI Variable Display, Segoe UI, system-ui, sans-serif', fontSize: 20px, fontWeight: '650', lineHeight: '1.25', letterSpacing: -0.016em }
  numeric: { fontFamily: '-apple-system, BlinkMacSystemFont, SF Pro Display, Segoe UI Variable Display, Segoe UI, system-ui, sans-serif', fontSize: 30px, fontWeight: '650', lineHeight: '1.15', letterSpacing: -0.022em }
  body: { fontFamily: '-apple-system, BlinkMacSystemFont, SF Pro Text, Segoe UI Variable Text, Segoe UI, system-ui, sans-serif', fontSize: 13.5px, fontWeight: '400', lineHeight: '1.55' }
  body-strong: { fontFamily: '-apple-system, BlinkMacSystemFont, SF Pro Text, Segoe UI Variable Text, Segoe UI, system-ui, sans-serif', fontSize: 13.5px, fontWeight: '590', lineHeight: '1.5' }
  label: { fontFamily: '-apple-system, BlinkMacSystemFont, SF Pro Text, Segoe UI Variable Text, Segoe UI, system-ui, sans-serif', fontSize: 12px, fontWeight: '650', lineHeight: '1.3', letterSpacing: 0.045em }
  meta: { fontFamily: '-apple-system, BlinkMacSystemFont, SF Pro Text, Segoe UI Variable Text, Segoe UI, system-ui, sans-serif', fontSize: 12.5px, fontWeight: '400', lineHeight: '1.5' }
  code: { fontFamily: 'ui-monospace, SF Mono, Cascadia Mono, Segoe UI Mono, monospace', fontSize: 12px, fontWeight: '400', lineHeight: '1.45' }
  control: { fontFamily: '-apple-system, BlinkMacSystemFont, SF Pro Text, Segoe UI Variable Text, Segoe UI, system-ui, sans-serif', fontSize: 14px, fontWeight: '590', lineHeight: '1.3', letterSpacing: -0.005em }
rounded:
  xs: 6px
  sm: 9px
  md: 11px
  lg: 14px
  xl: 18px
  xxl: 24px
  full: 9999px
spacing:
  '1': 4px
  '2': 8px
  '3': 12px
  '4': 16px
  '5': 20px
  '6': 24px
  '7': 32px
  '8': 40px
  window-gutter: 24px
  control-height: 40px
  target-min: 44px
elevation:
  sh-1: '0 0.5px 1px rgba(0,0,0,.05), 0 1px 2px rgba(0,0,0,.05)'
  sh-2: '0 1px 2px rgba(0,0,0,.05), 0 4px 12px rgba(0,0,0,.07)'
  sh-3: '0 2px 6px rgba(0,0,0,.06), 0 12px 32px rgba(0,0,0,.10)'
  sh-1-dark: '0 0.5px 1px rgba(0,0,0,.4)'
  sh-2-dark: '0 1px 2px rgba(0,0,0,.4), 0 4px 14px rgba(0,0,0,.34)'
  sh-3-dark: '0 2px 8px rgba(0,0,0,.44), 0 16px 40px rgba(0,0,0,.5)'
---

# FairDrop — Design Spine (Quartz)

> **Supersedes** `ux-FairDrop-2026-08-23` (Paper Relay / Terracotta Linen) in full.
> `frontend/src/ui/styles.test.ts` asserts that exactly one `ux-*` folder exists
> under `_bmad-output/planning-artifacts/ux-designs`, so this folder **replaces**
> the previous one rather than sitting beside it. Two folders fail the entire
> frontend suite as a length assertion, not as contrast drift.

> **Owner policy, 2026-09-11:** `docs/release-policy.md` governs release evidence.
> Automated verification remains required; manual device, screen-reader, firewall
> and visual observations are optional for personal releases. Unobserved behavior
> is not a verified pass, and known functional failures are not waived.

**Source precedence:** the canonical `SPEC.md` and its binding architecture and
contracts companions control. `epics.md` supplies the approved decomposition.
`EXPERIENCE.md` continues to control copy, focus routing, and announcements, and
is unchanged by this spine except where a row below names it.

## Brand & Style

Quartz is a neutral-material system with a single signal colour. FairDrop is a
compact utility that does one thing between two devices, and the interface should
read as part of the operating system rather than as a branded destination: a
near-neutral canvas, surfaces that lift rather than outline, one accent colour
reserved for the next action, and type that resolves to the platform's own
display face. **Story 7.8** returned that accent from Quartz's first-cut system
blue to the mocha of FairDrop's own icon (`build/appicon.png`) — see the Colors
section below for the exact values.

The hierarchy is unchanged from Paper Relay: one current item, one next action, one
honest status. What changed is the material. Paper Relay carried identity in a warm
palette, a rounded display face, and a literal paper offset. Quartz carries it in
restraint, depth and motion instead, because the same treatment has to read as
native in WKWebView on macOS and WebView2 on Windows from one stylesheet.

**Cross-platform constraint, binding.** One look ships on both platforms. No rule
in this document may depend on a macOS-only capability. Window vibrancy, `-apple-`
prefixed material APIs, and any effect that samples pixels behind the window are
forbidden: WebView2 cannot reproduce them, and a treatment that degrades on
Windows is not one look. Translucency *within* the page is permitted — both engines
composite it identically — but every translucent value is additionally published
here as the opaque colour it resolves to, because the contrast proof is computed
from opaque tokens.

Light and dark follow the operating-system preference; there is no theme control.
In forced-colors mode the system palette supersedes Quartz entirely.

## Colors

Quartz is an authored light/dark pair, not a tint recipe. Use the exact values in
the frontmatter in ordinary colour modes.

| Role | Light / dark | Rule |
|---|---|---|
| Canvas and surfaces | `{colors.canvas}` / `{colors.canvas-dark}`; `{colors.surface}` / `{colors.surface-dark}`; `{colors.elevated}` / `{colors.elevated-dark}` | Canvas frames the task. In light, surface and elevated are both white and are separated by shadow. In dark they are distinct values, because shadow alone cannot separate two dark planes — dark mode lifts by tone, light mode lifts by shadow. |
| Fills | `{colors.fill}` / `{colors.fill-dark}`; `{colors.fill-strong}` / `{colors.fill-strong-dark}` | Secondary control and field backgrounds. A fill is **never** the sole cue that something is a control: see Functional boundary. |
| Text | `{colors.text}` / `{colors.text-dark}`; `{colors.muted}` / `{colors.muted-dark}` | Muted is readable secondary copy, never disabled text. |
| Decorative edge | `{colors.separator}` / `{colors.separator-dark}` | Dividers inside a surface, and nothing else. Never the sole boundary for a control, drop target, QR, progress track, or status. It deliberately does **not** meet 3:1 — it is not a boundary, it is a rule between paragraphs. |
| Functional boundary | `{colors.control-border}` / `{colors.control-border-dark}` | Required on controls, the rest-state drop target, the QR frame, the URL field, and the progress-track outline. These values were chosen as the lightest greys that still clear 3:1 against every surface they touch. |
| Action | `{colors.primary}` / `{colors.primary-dark}` with the matching ink, and `{colors.primary-hover}` / `{colors.primary-hover-dark}` as the hover fill | The single strongest action, the (solid) progress fill, and the item-kind pill. Never status decoration. **The hover fill is a separate token from the resting fill, and the two move in opposite directions: light darkens on hover, dark lightens.** A single shared lighter value once put light mode's hover label under the 4.5:1 text floor — see the hover-pair row below. The primary button's own sheen (Elevation & Depth, below) is a fixed translucent white independent of both, not a third fill token. |
| Focus | `{colors.primary}` / `{colors.primary-dark}`, with a `{colors.surface}` / `{colors.surface-dark}` gap | A two-tone ring (Story 7.11): a surface-coloured gap, then a 2px ring in the accent colour itself, stacked as `box-shadow`, never `outline`. Story 7.8's dedicated violet is retired — the owner found it "poorly polished," a second hue with no relationship to the rest of the product — and the gap, not a different hue, is what keeps a same-hue ring legible against a same-hue fill. Scoped to keyboard-operable controls only — see the amendment carried forward below. |
| Outcomes | Success, warning, error, each with a tint | Always pair colour with outline, glyph, or literal text. |
| QR | `{colors.qr-surface}` / `{colors.qr-ink}` in both modes | Fixed high-contrast substrate; never recolour, invert, texture, rotate, round modules, or overlay a logo. Unchanged from Paper Relay and non-negotiable — it is a scan-reliability constraint, not a style choice. |

WCAG 2.2 targets are unchanged: ≥4.5:1 for normal text, ≥3:1 for large text, and
>3:1 without rounding for load-bearing non-text boundaries, focus indicators, and
value distinctions.

Every figure in the two tables below is **derived, not maintained**.
`frontend/src/ui/styles.test.ts` recomputes each ratio from the tokens
`style.css` actually declares and asserts this document publishes it unrounded.
A palette edit that quietly breaks a pair fails at that assertion, and a figure
hand-edited here is rejected by the same test.

Every figure below was copied verbatim from `styles.test.ts`'s own computed
output (`frontend/src/ui/styles.test.ts`, "the unrounded contrast proof"), never
hand-computed or hand-adjusted. `qr-ink` on `qr-surface` is fixed in both modes
at **17.377657264**.

| Text pair | Light ratio | Dark ratio |
|---|---:|---:|
| `text` on `canvas` | 15.542768731 | 16.163110010 |
| `text` on `surface` | 17.377657264 | 14.704322177 |
| `text` on `elevated` | 17.377657264 | 13.308196422 |
| `muted` on `canvas` | 5.178387528 | 6.630453856 |
| `muted` on `surface` | 5.789717726 | 6.032027848 |
| `muted` on `elevated` | 5.789717726 | 5.459307165 |
| `error` on `elevated` | 5.518575206 | 5.859450762 |
| `primary-ink` on `primary` | 5.529272927 | 7.709467487 |
| `primary-ink` on `primary-hover` | 7.608987041 | 9.920419953 |
| `text` on `primary-tint` | 15.153268673 | 11.826910776 |
| `muted` on `primary-tint` | 5.048617711 | 4.851652072 |
| `primary-ink` on primary button sheen (rest) | 4.677257536 | 8.269767363 |

**The hover row above is a review finding, added after ship.** An interactive
state's own fill is a text background like any other, and belongs in this
table -- the resting `primary-ink`/`primary` pair does not stand in for it.
Before this row existed, `.fd-button--primary:hover` read `{colors.primary-hi}`
-- the gradient's top stop, correct for dark mode's lighten-on-hover direction
but wrong for light's -- and put `primary-ink` on `#2B86EE` at 3.654:1 in light
mode, under the 4.5:1 floor, unmeasured because no published pair covered it.
`{colors.primary-hover}` is the fix: a token dedicated to the hover fill,
independent of the gradient stop, so the two responsibilities cannot collide
again.

**The `primary-tint` rows above are a Story 7.8 review finding, added the same
way.** `{colors.primary-tint}` is a text background too -- the drop zone's
drag-active fill, which the heading and meta line render on while a drag is
over it -- and it went unmeasured through the whole mocha-accent change
because the story's token table named only five tokens. `primary-tint` moved
from a leftover blue wash (`#E6EFFB` / `#23303F`) to mocha at 10% light / 12%
dark on `{colors.surface}` / `{colors.surface-dark}`. The dark fraction is
deliberately 12%, not 16%: 16% was checked first and put `muted` on it at
4.49:1, under the 4.5:1 floor: `text`/`primary-tint` and `muted`/`primary-tint`
must both be re-checked before this token moves again.

**The sheen row above is a third finding of the same shape, from the
black-flash defect fix.** This is the third time in this epic a colour
turned out to be an unmeasured text background -- the hover fill, then
`{colors.primary-tint}`, and now the primary button's own sheen. The pattern
is that any surface text sits on belongs in this proof, whether or not it is
a named token. The sheen is a fixed translucent-white overlay, independent
of the fill colour beneath it (see Elevation & Depth, below, for why it
replaced the `primary-hi` -> `primary` colour-stop gradient), and it sits
directly under the `primary-ink` label at the top of the button in every
state, so composited over whichever fill is beneath it, it is a text
background like any other. Rest is the weaker of the two states in both
modes (hover's fill is already lighter in light mode and lighter still in
dark mode, both moving the composite away from the floor), so rest is what
is published; light rest, **4.677257536**, is the weakest of all four
rest/hover x light/dark combinations and the figure the sheen's alpha
(0.08) is chosen against. Re-derived, unrounded, from
`frontend/src/ui/styles.test.ts`, not hand-computed: raising the alpha
towards 0.12 fails this floor, which is why it is not raised.

Status text placed on its own panel (`.fd-button--quiet`'s muted/error on
elevated is the row above; `warning`/`success`/`error` on `surface` is what the
Warning Banner, a completed transfer's success copy, and the Error Panel place)
is published as a floor rather than one row per status, because the row has to
stay the weakest of the three as the palette moves: every one of the three is
proven, unrounded, to exceed 5.36:1 light and 6.47:1 dark.

| Load-bearing pair | Light ratio | Dark ratio |
|---|---:|---:|
| `control-border` on `canvas` | 3.240328251 | 4.245594789 |
| `control-border` on `surface` | 3.622862486 | 3.862412220 |
| `control-border` on `elevated` | 3.622862486 | 3.495689217 |
| `primary` on `track` | 4.560313844 | 4.954296293 |
| `primary` on `surface` | 5.529272927 | 7.193529288 |
| `primary` on `elevated` | 5.529272927 | 6.510527964 |
| `warning` on `elevated` | 6.329195349 | 6.871224941 |
| `primary` on `canvas` | 4.945442820 | 7.907185645 |
| `primary` on `fill` | 4.898523535 | 5.667386798 |
| `primary` on `primary-tint` | 4.821510572 | 5.785865409 |

Status rules and the focus ring against the stronger surfaces (`canvas`,
`surface`, `elevated`) are published as the weakest-of-the-set claim rather
than one row each, for the same reason as the status-on-surface floor above.
The weakest of `warning`/`success`/`error` on `canvas` is `success` at
**4.799052371** light and `error` at **7.116437439** dark. The weakest of
`primary` — the ring's own colour, since Story 7.11 — against
`canvas`/`surface`/`elevated` is `primary` on `canvas` at **4.945442820**
(light); `canvas` was already the weakest of the three before the ring moved
onto this token, and stays weakest now. Widening the set to the fourth surface
the ring can sit on, `fill`, the overall weakest is `primary` on `fill` at
**4.898523535** light and **5.667386798** dark — both published as their own
row in the table above, since `fill` is load-bearing for the ring in a way the
other three status rules never need it to be.

**Story 7.11 retires the dedicated violet Story 7.8 introduced**, and the ring
reads `{colors.primary}` / `{colors.primary-dark}` directly — the owner found
the violet "poorly polished," a second hue with no relationship to the rest of
the product. Story 7.8's reasoning still holds — a same-hue ring on a
same-hue fill is not an indicator — but the mechanism moves from a distinct
hue to a structural gap: the ring is drawn as two stacked `box-shadow` layers,
a `{colors.surface}` gap and then the `{colors.primary}` ring beyond it, so
the ring never touches a same-hue fill directly. The four rows above —
`surface`, `elevated`, `canvas`, `fill` — are exactly the ring's own
visibility proof restated using the token it now shares with the rest of the
accent colour, each well clear of the 3:1 non-text floor in both modes.

`separator` is excluded from both tables on purpose: it is the decorative edge,
never a boundary, and it deliberately fails 3:1 against every surface it can sit
on. Against `surface` (its most common adjacency) it is 1.453401544 light and
1.775620130 dark; across `canvas`/`surface`/`elevated` it ranges 1.299938406 to
1.453401544 light and 1.607030993 to 1.951776026 dark — never within reach of
3:1. `styles.test.ts` asserts it is never the sole boundary of any control.

When `forced-colors: active`, use system colours for text, surfaces, controls,
borders, status rules, progress, and focus; retain text, glyph, length, and
pattern distinctions. **Every shadow and every gradient is dropped in forced
colors** — neither has a system colour, and a gradient that survives repaints a
system-coloured control in an authored hue. `forced-color-adjust: none` remains
forbidden except on the production QR bitmap and its white quiet-zone substrate.
Give the drop zone a visible system-colour boundary.

**Carried forward from the 2026-09-08 amendment, unchanged:** the focus outline is
scoped to keyboard-Tab-reachable controls. Routed landing targets — state headings,
the cancel summary, and the outcome panel — take focus by script only, sit outside
the Tab order, and paint no outline. A shared rule made every scripted focus move
paint a ring regardless of input device, and a transfer completing on its own has
no preceding interaction for `:focus-visible` to inherit a modality from, so the
outcome panel read as stuck "selected". Each landing target carries its own glyph
and heading text instead.

**New, and load-bearing on macOS:** the focus ring is only reachable if Tab can
reach the control at all. WebKit's macOS default leaves
`WKPreferences.tabFocusesLinks` NO, which prevents Tab from focusing a `<button>`
entirely. `main.go` sets `Mac.Preferences.TabFocusesLinks` and `main_test.go` pins
it. Any future control that relies on Tab depends on that option remaining set.

## Typography

The platform's own display face carries the voice: SF Pro on macOS via
`-apple-system`, Segoe UI Variable on Windows, with `system-ui` behind both. **No
font is bundled and none is fetched.** This retires Nunito and with it the whole
class of local-font hazards the Paper Relay spine had to legislate — the single
bundled weight, the faux-bold prohibition, and the woff2 asset. What that rule
protected (no third party, no render-blocking request, no layout shift from a late
font) is now structural rather than asserted.

Weights are real, not synthesised: 400 body, 590 controls and strong body, 650
display and headline. 590 and 650 are deliberate — they map to SF Pro's true
optical weights and fall back cleanly on Segoe UI Variable.

Do not fetch a font over the network, bundle a font file, use all-caps paragraphs,
or render body copy below 12px. `{typography.label}` is the one all-caps style and
is restricted to short section labels of at most three words.

Bidi handling is unchanged and remains binding: render the complete sanitized item
name in a `<bdi dir="auto">`, keep adjacent metadata in separate isolates, use
`overflow-wrap:anywhere` and `min-inline-size:0`, and never truncate by JavaScript
code unit. A two-line visual clamp may be used only with a persistent
keyboard-operable control labelled by `EXPERIENCE.md` key `copy.name.show_full`
and an assistive description containing the complete value.

`{typography.numeric}` is new: the transfer percentage and the completion receipt
figures use it with `font-variant-numeric: tabular-nums`, so a changing value does
not reflow its own row.

## Layout & Spacing

Use the 4/8/12/16/20/24/32/40 scale. App content uses `{spacing.window-gutter}`,
`{spacing.5}` between lifecycle regions, and `{spacing.2}`–`{spacing.4}` within
components. Interactive targets remain at least `{spacing.target-min}` in both
dimensions even when the visible control is quieter than that.

At the 1024×768 default window, constrain the lifecycle region to a centred
readable column: 620px for single-column states, 800px for the staged hero. At the
supported 640×480 native minimum, preserve actions and status before decorative
space and permit vertical scrolling rather than clipping.

The reflow contract is unchanged in behaviour and must be re-proven against the new
stylesheet: details sit beside the QR only above a 760px content width; below that
the QR stacks above the URL row; below 640px the remaining pair collapses to one
column; and at an effective content width of 320 CSS pixels everything is one
vertical column with no page-level horizontal scrolling, no information loss, no
overlap, and no clipped actions. The single browse control stays out of the
pair-collapse query. Containers grow under 200% text and WCAG text-spacing
overrides; no fixed height may clip content.

### Vertical composition (Story 7.7)

Width was the only axis this section constrained until Idle's content shrank
enough (Story 7.3's disclosures) to expose the gap: nothing here said how a
lifecycle state should use the *height* the window actually gives it, so a
state could hug its own content and leave the remainder of a tall window
empty -- the owner's original complaint about the success screen, reborn in
Idle once the fix for the first one shortened its column.

**The lifecycle region fills the available window height rather than hugging
its content.** How each state spends that height depends on what kind of
state it is:

- **Idle** distributes it by growing, not by centering: the drop zone absorbs
  the slack below the column's natural height, bounded by a maximum so a
  maximised window does not produce an absurd target, while the controls
  beneath it -- the browse control, the disclosures (Story 7.8 order) -- keep their natural
  height and spacing. The drop zone is the actual target a sender drags onto,
  so a larger one is a real improvement, not only a visual one.
- **Pending, Transferring, and a terminal Done or Error rendered as the phase
  view** are short, single-purpose states with no reason to anchor to the top
  edge, so the region is centred vertically instead.
- **Staged is the one exception, and stays top-aligned.** It is content-rich --
  packet, hero, QR, direct-link row, disclosures -- and centring a tall column
  moves content upward as the window shortens; the QR is the first thing that
  leaves the viewport when it does, because it sits nearest the vertical
  centre of a hero-heavy layout. Top alignment keeps the loss monotonic: content
  is lost from the bottom, through the permitted vertical scroll, rather than
  from wherever centring happens to put the QR.
- A retained outcome rendered above Idle keeps its natural height regardless of
  the rule above; only the phase's own region grows or centres. The drop zone's
  minimum height is a floor the retained panel cannot push it under.

At the 640×480 native minimum, a 320 CSS pixel content width, 200% text zoom,
and the WCAG text-spacing overrides, the drop zone's growth is the first thing
to yield: it has a *bounded flexible* height -- a flex-grow with a maximum,
never a fixed one -- so once the window is smaller than its natural size,
vertical scrolling takes over rather than clipping, overlapping, or forcing the
zone below its floor. Every existing reflow guarantee in the section above
continues to hold unchanged; this rule only governs the block axis.

## Elevation & Depth

**This section replaces the Paper Relay elevation rule outright.** The previous
spine allowed exactly one decorative `3px 3px 0` paper offset and forbade
gradients, ambient glows and stacked shadows. Quartz depends on layered shadow and
a gradient on one control, so the prohibition is restated rather than ignored.

Three elevation steps, published in the frontmatter and declared once as tokens:

| Step | Use |
|---|---|
| `{elevation.sh-1}` | A resting surface that sits on canvas: the disclosure rows. |
| `{elevation.sh-2}` | A raised surface the user is acting on: the drop zone, the QR card. |
| `{elevation.sh-3}` | The focal object of the current lifecycle state: the packet, the progress card, the outcome panel, the open browse menu. |

Rules, all enforceable:

- **Shadow is decorative and never a boundary.** Anything a shadow separates must
  also be separable without it — by tone in dark mode, by a functional boundary
  where the element is a control. This is what keeps forced-colors honest.
- **Exactly one gradient exists in the product**: a fixed translucent-white
  sheen (`rgb(255 255 255 / 0.08)` to `rgb(255 255 255 / 0)`) over the primary
  button's own fill, plus its 1px inset top highlight. No other gradient is
  permitted, and none may sit behind text. The progress fill is therefore
  solid, not a gradient. The sheen is declared identically at rest and on
  `:hover` -- one gradient, painted twice, not two -- so only the opaque
  `background-color` beneath it changes on hover; see the black-flash defect
  fix below for why.
  **The black-flash defect fix retires `{colors.primary-hi}` and
  `{colors.primary-hi-dark}`.** Through Story 7.7 this gradient read
  `{colors.primary-hi}` → `{colors.primary}` as a colour-stop fill, and
  `primary-hi` fed only that gradient and (briefly, incorrectly) the hover
  fill -- Story 7.7 narrowed its dark value from `#6FB0FF` to `#5EA6FF` for
  exactly that reason, without disturbing any published ratio, since
  `primary-hi` was never one of the tokens the unrounded contrast proof
  publishes a figure for. The owner-observed defect: hovering the primary
  button produced a black flash, because `.fd-button--primary` set this
  gradient through the `background` shorthand, which resets
  `background-color` to `transparent` as a side effect, and `:hover` set its
  flat fill through the same shorthand, which resets `background-image` to
  `none`. `background-image` cannot interpolate, so it jumped instantly,
  while `background-color` spent the whole 150ms transition animating FROM
  the shorthand's `transparent` reset -- a see-through button for that
  window. The fix reads and writes `background-color` and `background-image`
  as longhands, and replaces the colour-stop gradient with the sheen above,
  which -- unlike a colour-stop gradient -- is independent of the fill colour
  underneath it and can therefore stay byte-identical between rest and hover.
  `primary-hi` has no remaining reader and is not declared any more. The
  sheen's alpha (0.08) is a derived value: the button label sits at the top
  of the button, where the sheen is strongest, so it is a text background --
  see the sheen row in the text-pair table above and its accompanying finding
  for the re-derived, unrounded figures and the floor the alpha must clear.
- **One `repeating-linear-gradient` is exempt** and is not a gradient in this
  rule's sense: the unknown-total meter's static diagonal pattern, which carries
  a state distinction rather than decoration and never moves.
  `styles.test.ts` encodes the exemption as a `(?<!repeating-)` lookbehind, so a
  decorative gradient cannot smuggle itself in under that name.
- **No ambient glow, no blur behind text, no more than three shadow layers on any
  element**, and no shadow on the app shell, which takes its outermost elevation
  from native window chrome.
- **Dark mode raises opacity rather than lowering it.** A shadow tuned for a light
  canvas disappears on a dark one; the `-dark` elevation tokens exist for that
  reason and are not derived from the light ones.
- The paper offset is **gone**. `frontend/src/ui/styles.test.ts` currently asserts
  it appears exactly once; Story 7.1 replaces that assertion with one that pins
  the three elevation tokens and the single-gradient rule.

## Shapes

`{rounded.xs}` for the item-kind pill and small markers, `{rounded.sm}` for menu
items, `{rounded.md}` for buttons and fields, `{rounded.lg}` for the QR card and
the browse menu, `{rounded.xl}` for notices and inner panels, `{rounded.xxl}` for
the drop zone, the packet, the progress card and the outcome panel.
`{rounded.full}` is reserved for the progress track, the item-kind pill, and a
**pill-shaped primary button** — the Browse Control (Story 9.3) and, later, the
single-action buttons Epic 9's terminal cards offer. It is still never used for
an ordinary container. This amends the previous "never a button" wording, which
predates Story 9.3's approved prototype
(`_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/mockups/epic-9-proposal.html`)
putting a pill button inside the drop zone; the row was written before that
prototype existed and is corrected here rather than left contradicted.

Nested radii are concentric: an inner element's radius equals the parent's radius
minus the padding between them. A 24px card with 7px inset padding takes a 17–18px
inner radius. This is why the drop zone's dashed inner rule is 18px inside a 24px
card, and it is the single most visible difference between a considered surface
and a careless one.

## Components

Visual specs pair with behavioral rows of the same names in `EXPERIENCE.md`.

| Component | Visual contract |
|---|---|
| **App Shell** | Canvas background, standard OS chrome, one centred lifecycle region plus an optional retained outcome in Idle. No shadow. |
| **Drop Zone** | `{rounded.xxl}` surface at `{elevation.sh-2}` with a 2px dashed inner rule at `{colors.control-border}` inset 7px (concentric, so `{rounded.xl}`). The **functional** token, not the decorative one: the zone carries no click handler and no tab stop, but that rule is the only thing identifying the drop target -- the card is `{colors.surface}` on `{colors.canvas}` at 1.09:1 in light mode, and shadow may not carry a boundary. This row previously said `{colors.separator}` and contradicted the Colors table above it, which already required the functional token on the rest-state drop target; the row was the error. Drag-active: the rule goes solid `{colors.primary}`, the fill goes `{colors.primary-tint}`, and the glyph lifts 3px. Fill is never the only state cue. The rest-state boundary uses `{colors.control-border}` in forced colors. |
| **Browse Control** | **Story 9.3 reverses Story 7.3's full-width rule** (which itself reversed Paper Relay's "quieter than the drop zone" rule): now that the drop zone carries no click handler and no tab stop of its own, and the control sits *inside* it as the zone's one action, a full-width row is no longer the shape that reads as "the acting control" -- a centred, intrinsic-width pill does, matching the owner-approved prototype. Centred, intrinsic-width, pill-shaped (`{rounded.full}`) primary button with a trailing chevron and `aria-haspopup="menu"`, placed after the glyph, heading and one line of promise copy inside the drop zone. Opens the menu; the label never changes. The same component (`BrowseControl`, extracted to its own module in Story 9.3) is reused wherever else the product needs a file-or-folder chooser, with only its label varying by caller. |
| **Browse Menu** | `{rounded.lg}` raised surface at `{elevation.sh-3}` with a functional boundary, 5px padding, `{rounded.sm}` items, each with a leading glyph naming its kind (File/Folder; Story 9.3). **One active item at a time, owned by focus** (Story 7.11): the menu has a single notion of "the item about to be chosen," and it is whichever item currently holds focus -- never a separate hover state and never two items marked at once. Hovering an item moves focus to it, so hover and keyboard navigation paint the identical treatment -- primary fill plus a tint halo and its own ring, not only the focus ring -- through the one `:focus` rule; there is no weaker `:hover`-only appearance. A menu opened by **pointer** pre-selects nothing: the trigger keeps focus and no item is marked until the sender points at or arrows onto one. A menu opened by **keyboard** (ArrowDown, ArrowUp, Enter or Space on the trigger) focuses the first item immediately, matching the platform convention that a keyboard-opened menu always lands focus inside it. |
| **Disclosure** | `{rounded.xl}` surface at `{elevation.sh-1}` with a 12px chevron that rotates 90° when open, and a hover fill. The chevron sits at the row's **trailing edge**, not immediately after the summary text -- `justify-content: space-between` on `.fd-disclosure__summary`, the same rule the owner-approved prototype's `.row` uses. (Defect fix, Story 9.3: the `<button>` that replaced `<summary>` in Story 9.1 lost the old heading's `flex: 1`, which had been what pushed the chevron to the edge, so it drifted to sit right after the label instead -- found on the built macOS binary.) **No leading icon**: Story 7.7 removed it after finding that a tinted circle small enough to sit beside a one-line summary resolves to a featureless coloured dot, which reads as a bullet rather than as an icon. A dot is not an acceptable outcome per that story's acceptance criteria, and a shape legible at this size does not fit the row without crowding the summary text, so the row ships with no glyph at all -- consistent with the platform's own disclosure rows, which frequently carry none either. **Replaces the always-open firewall and recovery blocks** — see the FR23 note below. **Idle groups both of its disclosures into one `{rounded.xl}` surface** (Story 9.3), the two rows divided by a `{colors.separator}` rule rather than each carrying its own card and shadow. **Story 9.4 retires Staged's always-open `RecoveryHelp` form**: Staged's "Trouble connecting?" is now a `Disclosure` too (its own card, not grouped with Idle's), so the sentence above that used to except Staged from this component no longer has an exception to state. **Story 9.1 implementation choice, recorded here as the acceptance criteria require:** this used to be native `<details>`/`<summary>`, which supplied keyboard operability and open/closed state for free but cannot smoothly expand or collapse in both engines this product ships to -- `build/darwin/Info.plist` declares `LSMinimumSystemVersion` 10.13, and neither the `<details>` display swap nor the newer `::details-content` pseudo-element needed to transition it is available across that whole range. It is now a controlled `<button aria-expanded aria-controls>` (wrapped in an `<h2>`, since a button's content model does not permit a heading child -- the WAI-ARIA APG accordion pattern) plus a region that expands and collapses with `grid-template-rows: 0fr ↔ 1fr`, the same technique the owner-approved prototype uses for every expand/collapse in the product and one with materially broader cross-engine support than an animated `<details>` open. Collapsed content stays in the DOM either way -- a transitioned `visibility: hidden`, not `inert` (unsupported on the older WebKit this range includes) and not an unmount, is what keeps it out of the tab order and the accessibility tree while still letting the region's height animate. Every existing keyboard, focus and content guarantee (Tab reachability, Enter/Space activation, Escape blurring the trigger, every recovery and firewall string still present regardless of open state) holds under the new markup; see `Disclosure.tsx` and `styles.test.ts` for the mutation-verified detail. |
| **Packet** | `{rounded.xxl}` surface at `{elevation.sh-3}`. Holds the warning banner and the hero (QR plus item details). Replaces the folder-tab silhouette and the paper offset. **Story 9.4:** Cancel and "Trouble connecting?" moved out of the card into a foot row below it (`.fd-staged-foot`) -- the card itself is exactly the QR-and-item handoff now, not the whole Staged surface. |
| **Item Kind Pill** | **Superseded by Story 9.4.** This described the `fd-packet-tab` label above the card (File/Folder, `{rounded.full}`, `{colors.primary-tint}` on `{colors.primary}`), which Story 9.4 removed along with the card it sat above. The item's kind is now a plain glyph beside its name in the item row (`.fd-item__icon`, `{colors.fill}` on `{colors.muted}`) -- decorative, not a badge, exactly as this row already required. |
| **Item Summary** | `{typography.headline}` name in a bidi isolate beside a kind glyph; kind, logical size and the ZIP note in `{typography.meta}`. **Story 9.4:** the name always wraps (`overflow-wrap: anywhere`); the persistent full-name control this row used to mention is removed along with the two-line clamp it existed to expand. |
| **QR Panel** | Fixed white substrate, 12px padding, `{rounded.lg}`, `{elevation.sh-2}` plus a functional boundary, ~216px square (Story 9.4; was 224px at `{rounded.xs}` before the QR became the card's whole point rather than sharing weight with an always-visible link row). Crisp, generous quiet zone, no rotation or overlay. |
| **Direct URL Row** | **Story 9.4:** not rendered until Show Link (`copy.direct_link.show`/`copy.direct_link.hide`, secondary) is activated, using the same `grid-template-rows`/transitioned-`visibility` mechanism as `Disclosure` (its own rule, `.fd-url-reveal`, since the trigger is a plain button rather than a heading-wrapped summary). Revealed: readonly monospace `<textarea>` on `{colors.fill}` with a functional boundary, unchanged from Story 7.9's CSS-grid mirror sizing. Copy Link (primary) copies without revealing it, named by `EXPERIENCE.md` key `copy.direct_link.action`. Never a sender-side activation link. |
| **Copy Feedback** | Label swaps to `copy.copy.confirmation` with a check glyph, as a non-reflowing crossfade (Story 9.4: both faces occupy the same grid cell, the inactive one `aria-hidden`) rather than a jump cut; the control takes the success tint and boundary. Fixed width so the swap cannot reflow the row; reverts on blur (D-114). No toast. |
| **Warning Banner** | `{rounded.md}`, warning tint, inset warning boundary, leading glyph, heading plus message. Inline, non-modal, never a full fill. |
| **Trusted-LAN Note** | Muted copy, each line with its own inline SVG glyph rather than a single shared warning-coloured bar (Story 9.4: an info glyph on the first-opener line, a lock glyph on the network line -- both `{colors.muted}`, matching the owner-approved prototype's `.caveats`, not warning-tinted). **This amends the previous "never green or lock-shaped" wording**, written before this story's approved prototype used exactly a lock glyph for the network disclosure; the story's own acceptance criteria name the lock glyph explicitly, and the story's text wins over an earlier row it contradicts. Literal plain-HTTP and local-network disclosure; still never green, and still never a claim of encryption. |
| **Progress Card** | **Superseded by Story 9.5.** This described a standalone `{rounded.xxl}`/`{elevation.sh-3}` surface (kind pill and name, then the percentage with wire bytes and throughput beside it, then the track, then Cancel) that Story 9.5 removed along with the card it sat above. Sending now reuses Staged's own **Packet** card and **Hero** geometry verbatim (the same `{rounded.sm}/{rounded.lg}` surface at `{elevation.sh-3}`, the fixed ~216px slot beside a `minmax(0, 1fr)` details column): the QR bitmap in that slot becomes the Progress Ring below, and the item row, figures and Cancel sit beside it exactly where Staged's link actions, revealed field and caveats do. |
| **Progress Ring** | **Replaces the Progress Meter (Story 9.5).** A ~216px ring in Staged's own QR slot, so the card never changes shape moving from Staged to Sending. Two concentric strokes on the resting track: a 10px band at `{colors.track}` (kept at that token specifically so the determinate fill's contrast against it stays the already-proven `primary`-on-`track` pairing) plus a 1px edge at `{colors.control-border}` just outside it -- the "functional boundary" this row has always required, expressed as a second stroke rather than a `border` property because a ring, unlike the retired linear track, has no separate fill rectangle to frame. The determinate fill is **solid** `{colors.primary}`, never a gradient -- the single-gradient rule is absolute and the button already spends it -- drawn with `stroke-dashoffset`, transitioned 400ms, never a `@keyframes` rule. Its own tabular-numeral percentage sits centred inside the ring. The unknown mode is a static, non-directional dashed stroke (no sweep, shimmer, or rotation of its own beyond the ring's fixed -90deg orientation) with its status caption beneath it; the known-empty mode shows the track only, `aria-hidden`, with no percentage-bearing role anywhere on the card. No fake ZIP or empty-file percentage ever appears. |
| **Transfer Metrics** | Two plain figure-over-caption pairs beside the ring, tabular numerals: wire bytes first (captioned `copy.label.sentCaption`, "Sent"), throughput second (captioned `copy.label.speedCaption`, "Speed") -- Story 9.5's own pair, distinct from the Completion Receipt's `copy.label.wireBytes`/`copy.label.throughput` ("Wire bytes"/"Throughput", unchanged), since that receipt is a different card. The figures themselves are unchanged: actual wire bytes and visual-only throughput. |
| **Cancel Action** | Quiet text action at full target size; error-coloured on hover and focus. |
| **Outcome Panel — Done** | Centred composition at `{elevation.sh-3}`: a 74px success-tint disc with a stroke-drawn check, `{typography.display}` heading, muted body, the **completion receipt**, then the primary next action and a quiet Dismiss. This is the state that previously rendered a heading and one line into a mostly empty window. |
| **Completion Receipt** | Two cells on `{colors.fill}`, divided by a separator: the item name and the wire bytes actually sent. **Two cells, not three** — no elapsed-time cell exists, because no clock is tracked and `EXPERIENCE.md` forbids frontend lifecycle timers. Both values must come from retained state, never from a placeholder. |
| **Outcome Panel — Error** | Same composition in the error pair, with a `!` glyph, the safe heading and message from the fixed registry, and recovery guidance. No raw diagnostics. Retained form adds Dismiss. |
| **Status Announcer** | Visually hidden, pre-mounted, atomic, layout-free. Never duplicates focused content. |

### FR23 and the disclosures

`SPEC.md` FR23 requires the firewall preflight to precede the selection control.
Story 7.3 first amended this: the preflight moved from "expanded and preceding the
selection control" to "present and preceding", collapsed by default inside a
disclosure rather than fully expanded. Idle previously opened with roughly two
hundred words of firewall and recovery copy above and below one button, which is
why the state read as a document rather than a tool.

**Story 7.8 amends it again, and this is the second weakening.** Idle's document
order is now drop zone, command-failure panel (if any), the browse control,
firewall preflight disclosure, recovery disclosure — the one control that acts
leads, and the two informational rows sit below it. The preflight no longer
precedes the selection control at all.

This is a spine amendment, not a CSS change, and it is the second place Quartz
weakens a written requirement rather than restating it. Recording the trade
honestly: a first-time sender can now reach the picker without having passed the
firewall guidance, which is what FR23 existed to prevent. What survives is that
the guidance is still present in Idle, still one keyboard-reachable control away,
still named by its summary line ("Local network access"), and still reachable
before the OS firewall prompt appears — and the preflight was already collapsed
by the Story 7.3 amendment, so a sender who did not expand it was never reading it
in the first place regardless of its position. If acceptance decides FR23 means
*visible* rather than merely *present*, the disclosure ships `open` by default and
the rest of the treatment is unaffected; if acceptance decides FR23 means the
preflight must again *precede* the selection control, the fix is to restore the
document order above it and nothing else in this spine depends on that ordering.

**Story 9.3 moves the control again, without touching FR23's substance.** The
browse control now renders *inside* the drop zone (glyph, heading, promise line,
control, in that order) rather than below it, so Idle's document order reads:
drop zone (containing the browse control), command-failure panel (if any), the
grouped disclosure list (firewall access, then troubleshooting). The preflight
still follows the control and is still present-but-collapsed on first paint --
this story changes where the control sits relative to the drop zone's own
content, not its relative order against the two disclosures, so the FR23
trade-off recorded above is unchanged.

## Motion

Motion is new to this spine; Paper Relay had none beyond a reduced-motion guard.
Story 9.1 gives it a foundation every later Epic 9 story builds on.

- One easing curve, unchanged: `--ease-decelerate`, `cubic-bezier(.32,.72,0,1)`,
  the decelerate curve platform animations use. One accent curve is what makes a
  set of transitions read as one system.
- Durations: 120ms for a press, 150–200ms for hover and fill changes, 200ms for the
  disclosure chevron, 400ms for the progress fill, 500ms for the completion check.
- **Entrance.** Every one of the five views -- Idle, Pending, Staged, Transferring,
  and the outcome panel when it is the phase view -- carries `data-phase-view` and
  enters with a fade (opacity 0→1, ~340ms) plus a 10px rise (translate, ~420ms),
  both on the one easing curve, via `@starting-style`. An optional per-child
  stagger (`--fd-stagger`, read into `transition-delay` at ~55ms a step) lets a
  view's own children rise in sequence behind it; it is capped at five steps so a
  long list can never queue a straggler more than ~275ms behind. **Views animate
  in and never out**: there is no exit transition anywhere in the product. An
  outgoing view is replaced, not animated off -- keeping an unmounted view's DOM
  alive long enough to animate it away would give one moment two nodes both
  claiming to be the current view, which breaks the one-retained-node identity
  reset and the outcome panel depend on.
- **Motion is progressive enhancement.** `build/darwin/Info.plist` declares
  `LSMinimumSystemVersion` 10.13, so the WKWebView this product runs in ranges from
  engines with no `@starting-style` support at all to current Safari. Every
  entrance is written so that an unsupporting engine simply shows the finished,
  resting state on first paint -- opaque, untranslated, unscaled -- with nothing
  to detect and no fallback branch to maintain. This is also why entrance motion is
  built from CSS transitions plus `@starting-style` rather than a keyframe rule:
  neither construct depends on the other, and `@starting-style`'s own graceful
  no-op is what a keyframe-based entrance would not get for free. **No `@keyframes`
  rule and no `animation` property appear anywhere in this sheet, and `styles.test.ts`
  enforces the ban directly.**
- **Disclosure.** Opens and closes by animating `grid-template-rows` between `0fr`
  and `1fr` on a controlled `<button aria-expanded>` plus region, not native
  `<details>` -- see the Disclosure row below for the choice and why. The chevron
  rotates in step, over 200ms, unchanged from before this story.
- **Browse Menu.** Enters with a fade and a scale from ~0.96, anchored at the
  corner it hangs from, over ≤200ms, via `@starting-style` -- the product's first
  floating surface gets the same progressive-enhancement treatment as a phase view.
- The completion check draws its stroke once via `stroke-dashoffset`. It is no
  longer the only entrance animation in the product, now that views, disclosures
  and the browse menu all animate in too -- but it remains the only moment marked
  by *drawing* rather than by fading or rising, which is still worth marking on its
  own terms.
- Buttons scale to 0.975 on `:active` via `transform: scale(0.975)` -- the one
  *interaction* accent that scales, and unaffected by the entrance motion above,
  which scales through the standalone `scale` property instead (the browse menu now,
  cards and discs in the stories that follow). The two compose independently and
  are neutralised independently, so pressing a button never loses its feedback to a
  rule written for an unrelated entrance.
- **`prefers-reduced-motion: reduce` collapses every duration and delay to
  effectively nothing and removes `translate`/`scale` outright, leaving the check
  mark fully drawn and every other state cue in its finished form.** The existing
  assertion that reduced motion removes nothing which carries meaning stays
  binding: the check is a state cue, so it must be present, not animated, and the
  same now holds for a disclosure's open state and the browse menu's presence --
  reduced motion changes how fast they arrive, never whether they do.

## Do's and Don'ts

| Do | Don't |
|---|---|
| Lift a surface with tone in dark and shadow in light. | Outline a card to separate it from the canvas. |
| Give every operable control a functional boundary that clears 3:1. | Let a subtle fill be the only thing that says "this is a control". |
| Keep the one gradient on the one primary button. | Add a second gradient, or put any gradient behind text. |
| Match nested radii concentrically. | Reuse one radius at every nesting level. |
| Show the item and the bytes actually sent on completion. | Invent a duration, a transfer rate, or any figure not retained in state. |
| Use Quartz in ordinary modes and system colours in forced colors. | Disable forced-color adjustment outside the QR substrate, or derive dark mode by inversion. |
| Keep production QR rendering square and unmodified. | Rotate, tint, round modules, or overlay a mark. |
