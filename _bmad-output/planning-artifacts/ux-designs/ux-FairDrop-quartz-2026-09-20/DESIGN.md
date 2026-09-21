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
  primary: '#0A6CD8'
  primary-hi: '#2B86EE'
  primary-ink: '#FFFFFF'
  primary-tint: '#E6EFFB'
  track: '#E9E9EB'
  focus: '#0A6CD8'
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
  primary-dark: '#4C9BFF'
  primary-hi-dark: '#6FB0FF'
  primary-ink-dark: '#06203F'
  primary-tint-dark: '#23303F'
  track-dark: '#3A3A3E'
  focus-dark: '#6FB0FF'
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
near-neutral canvas, surfaces that lift rather than outline, one blue reserved for
the next action, and type that resolves to the platform's own display face.

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
| Action | `{colors.primary}` / `{colors.primary-dark}` with the matching ink, and `{colors.primary-hi}` / `{colors.primary-hi-dark}` as the gradient's top stop | The single strongest action, the (solid) progress fill, and the item-kind pill. Never status decoration. |
| Focus | `{colors.focus}` / `{colors.focus-dark}` | A 3px outline at 2px offset. Scoped to keyboard-operable controls only — see the amendment carried forward below. |
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
| `primary-ink` on `primary` | 5.060845348 | 5.784694439 |

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
| `primary` on `track` | 4.173974302 | 4.011363202 |
| `primary` on `surface` | 5.060845348 | 5.824411172 |
| `primary` on `elevated` | 5.060845348 | 5.271402992 |
| `warning` on `elevated` | 6.329195349 | 6.871224941 |
| `focus` on `elevated` | 5.060845348 | 6.610678352 |

Status rules and the focus ring against the stronger surfaces (`canvas` and
`surface`) are published as the weakest-of-the-set claim rather than one row
each, for the same reason as the status-on-surface floor above. The weakest of
`warning`/`success`/`error` on `canvas` is `success` at **4.799052371** light
and `error` at **7.116437439** dark. The weakest of `focus` against
`canvas`/`surface`/`elevated` is `focus` on `canvas` at **4.526476017** (light;
Quartz's focus token equals primary, so canvas — not elevated — is the weakest
adjacent surface).

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
- **Exactly one gradient exists in the product**: the primary button's
  `{colors.primary-hi}` → `{colors.primary}` vertical fill, plus its 1px inset top
  highlight. No other gradient is permitted, and none may sit behind text. The
  progress fill is therefore solid, not a gradient.
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
`{rounded.full}` is reserved for the progress track and the item-kind pill — never
a button or a container.

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
| **Browse Control** | Full-width primary button with a trailing chevron and `aria-haspopup="menu"`. Opens the menu; the label never changes. |
| **Browse Menu** | `{rounded.lg}` raised surface at `{elevation.sh-3}` with a functional boundary, 5px padding, `{rounded.sm}` items. The focused item takes the primary fill plus a tint halo, not only the focus ring. |
| **Disclosure** | `{rounded.xl}` surface at `{elevation.sh-1}` with a leading tinted icon, a 12px chevron that rotates 90° when open, and a hover fill. **Replaces the always-open firewall and recovery blocks** — see the FR23 note below. |
| **Packet** | `{rounded.xxl}` surface at `{elevation.sh-3}`. Holds the warning banner, hero, disclosures and cancel. Replaces the folder-tab silhouette and the paper offset. |
| **Item Kind Pill** | `{rounded.full}`, `{colors.primary-tint}` on `{colors.primary}`, `{typography.label}`, with a leading glyph. Says File or Folder. Never an authoritative-state badge. |
| **Item Summary** | `{typography.headline}` name in a bidi isolate with the full-name control beside it; kind, logical size and the ZIP note in `{typography.meta}`. |
| **QR Panel** | Fixed white substrate, 12px padding, `{rounded.lg}`, `{elevation.sh-2}` plus a functional boundary. Square, crisp, generous quiet zone, no rotation or overlay. |
| **Direct URL Row** | Readonly monospace `<textarea>` on `{colors.fill}` with a functional boundary, beside the action named by `EXPERIENCE.md` key `copy.direct_link.action`. Never a sender-side activation link. |
| **Copy Feedback** | Label swaps to `copy.copy.confirmation` and the control takes the success tint and boundary. Fixed width so the swap cannot reflow the row; reverts on blur (D-114). No toast. |
| **Warning Banner** | `{rounded.md}`, warning tint, inset warning boundary, leading glyph, heading plus message. Inline, non-modal, never a full fill. |
| **Trusted-LAN Note** | Muted copy behind a 3px `{rounded.xs}` warning-coloured bar. Literal plain-HTTP and local-network disclosure; never green or lock-shaped. |
| **Progress Card** | `{rounded.xxl}` surface at `{elevation.sh-3}`: kind pill and name, then the percentage in `{typography.numeric}` with wire bytes and throughput right-aligned beside it, then the track, then Cancel. |
| **Progress Meter** | 8px `{rounded.full}` track at `{colors.track}` with a functional boundary; fill is **solid** `{colors.primary}`, not a gradient -- the single-gradient rule is absolute and the button already spends it. Determinate value, static unknown pattern, or decorative known-empty track. No fake ZIP or empty-file percentage, and no sweep, shimmer or blink. |
| **Transfer Metrics** | Wire bytes first, throughput second, tabular numerals. |
| **Cancel Action** | Quiet text action at full target size; error-coloured on hover and focus. |
| **Outcome Panel — Done** | Centred composition at `{elevation.sh-3}`: a 74px success-tint disc with a stroke-drawn check, `{typography.display}` heading, muted body, the **completion receipt**, then the primary next action and a quiet Dismiss. This is the state that previously rendered a heading and one line into a mostly empty window. |
| **Completion Receipt** | Two cells on `{colors.fill}`, divided by a separator: the item name and the wire bytes actually sent. **Two cells, not three** — no elapsed-time cell exists, because no clock is tracked and `EXPERIENCE.md` forbids frontend lifecycle timers. Both values must come from retained state, never from a placeholder. |
| **Outcome Panel — Error** | Same composition in the error pair, with a `!` glyph, the safe heading and message from the fixed registry, and recovery guidance. No raw diagnostics. Retained form adds Dismiss. |
| **Status Announcer** | Visually hidden, pre-mounted, atomic, layout-free. Never duplicates focused content. |

### FR23 and the disclosures

`SPEC.md` FR23 requires the firewall preflight to precede the selection control.
Under Quartz the preflight is still rendered above the browse control and is still
present on first paint — but it is **collapsed by default** inside a disclosure
rather than fully expanded. Idle previously opened with roughly two hundred words
of firewall and recovery copy above and below one button, which is why the state
read as a document rather than a tool.

This is a spine amendment, not a CSS change, and it is the one place Quartz weakens
a written requirement rather than restating it. Recording the trade honestly: the
guidance is one keyboard-reachable control away instead of zero, and the summary
line still names the topic ("Local network access") so the sender can see that the
answer exists before they need it. If acceptance decides FR23 means *visible*
rather than *present and preceding*, the disclosure ships `open` by default and the
rest of the treatment is unaffected.

## Motion

Motion is new to this spine; Paper Relay had none beyond a reduced-motion guard.

- One easing curve: `cubic-bezier(.32,.72,0,1)`, the decelerate curve platform
  animations use. One accent curve is what makes a set of transitions read as one
  system.
- Durations: 120ms for a press, 150–200ms for hover and fill changes, 200ms for the
  disclosure chevron, 400ms for the progress fill, 500ms for the completion check.
- The completion check draws its stroke once via `stroke-dashoffset`. It is the
  only entrance animation in the product, and it marks the only moment worth
  marking.
- Buttons scale to 0.975 on `:active`. Nothing else scales.
- **`prefers-reduced-motion: reduce` removes every animation and transition and
  leaves the check mark fully drawn.** The existing assertion that reduced motion
  removes nothing which carries meaning stays binding: the check is a state cue, so
  it must be present, not animated.

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
