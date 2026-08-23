---
name: FairDrop
description: Paper Relay visual system in the Terracotta Linen palette for an ephemeral LAN handoff utility.
status: final
created: '2026-08-23'
updated: '2026-08-23'
sources:
  - '{project-root}/_bmad-output/specs/spec-fairdrop/SPEC.md'
  - '{planning_artifacts}/architecture/architecture-FairDrop-2026-08-22/ARCHITECTURE-SPINE.md'
  - '{project-root}/docs/fairdrop-contracts.md'
  - '{planning_artifacts}/epics.md'
  - '{project-root}/docs/fairdrop-spec.md'
colors:
  canvas: '#F7F0E7'
  surface: '#FFFAF4'
  elevated: '#EFE2D4'
  text: '#2C2723'
  muted: '#70645B'
  border: '#CDBDAE'
  control-border: '#8B7462'
  primary: '#A94724'
  primary-ink: '#FFFFFF'
  hover: '#873719'
  drop: '#F1D0BD'
  progress: '#B64B23'
  focus: '#7E4B92'
  success: '#2F7658'
  warning: '#946000'
  error: '#AB3932'
  canvas-dark: '#1C1916'
  surface-dark: '#25211D'
  elevated-dark: '#312A24'
  text-dark: '#F7EFE5'
  muted-dark: '#BCAF9F'
  border-dark: '#55493F'
  control-border-dark: '#89796A'
  primary-dark: '#FF986D'
  primary-ink-dark: '#2B1309'
  hover-dark: '#FFB18F'
  drop-dark: '#4A2D23'
  progress-dark: '#FF8858'
  focus-dark: '#C5A1D3'
  success-dark: '#79D5AA'
  warning-dark: '#F2BD62'
  error-dark: '#FF8B83'
  qr-surface: '#FFFFFF'
  qr-ink: '#221F1C'
typography:
  display: { fontFamily: 'Georgia, Cambria, Times New Roman, serif', fontSize: 24px, fontWeight: '600', lineHeight: '1.2', letterSpacing: -0.015em }
  headline: { fontFamily: 'Georgia, Cambria, Times New Roman, serif', fontSize: 20px, fontWeight: '600', lineHeight: '1.25', letterSpacing: -0.01em }
  body: { fontFamily: 'system-ui, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif', fontSize: 14px, fontWeight: '400', lineHeight: '1.5' }
  body-strong: { fontFamily: 'system-ui, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif', fontSize: 14px, fontWeight: '650', lineHeight: '1.4' }
  label: { fontFamily: 'system-ui, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif', fontSize: 12px, fontWeight: '700', lineHeight: '1.3', letterSpacing: 0.06em }
  meta: { fontFamily: 'system-ui, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif', fontSize: 12px, fontWeight: '400', lineHeight: '1.45' }
  code: { fontFamily: 'ui-monospace, Cascadia Mono, Segoe UI Mono, monospace', fontSize: 12px, fontWeight: '500', lineHeight: '1.4' }
  control: { fontFamily: 'system-ui, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif', fontSize: 13px, fontWeight: '650', lineHeight: '1.2' }
rounded:
  xs: 3px
  sm: 5px
  md: 8px
  lg: 10px
  xl: 14px
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
  window-gutter: 20px
  control-height: 36px
  target-min: 44px
components:
  app-shell: { background: '{colors.canvas}', background-dark: '{colors.canvas-dark}', foreground: '{colors.text}', foreground-dark: '{colors.text-dark}', gutter: '{spacing.window-gutter}' }
  drop-zone: { background: '{colors.surface}', background-dark: '{colors.surface-dark}', active: '{colors.drop}', active-dark: '{colors.drop-dark}', border: '{colors.control-border}', border-dark: '{colors.control-border-dark}', radius: '{rounded.lg}' }
  selection-controls: { background: '{colors.surface}', background-dark: '{colors.surface-dark}', foreground: '{colors.text}', foreground-dark: '{colors.text-dark}', border: '{colors.control-border}', border-dark: '{colors.control-border-dark}', radius: '{rounded.md}', height: '{spacing.target-min}' }
  stage-pending-card: { background: '{colors.elevated}', background-dark: '{colors.elevated-dark}', foreground: '{colors.text}', foreground-dark: '{colors.text-dark}', radius: '{rounded.lg}' }
  staged-view: { background: '{colors.elevated}', background-dark: '{colors.elevated-dark}', border: '{colors.border}', border-dark: '{colors.border-dark}', radius: '{rounded.lg}', shadow: '3px 3px 0 #CDBDAE', shadow-dark: '3px 3px 0 #55493F' }
  item-summary: { foreground: '{colors.text}', foreground-dark: '{colors.text-dark}', secondary: '{colors.muted}', secondary-dark: '{colors.muted-dark}', title: '{typography.headline.fontSize}' }
  qr-panel: { background: '{colors.qr-surface}', foreground: '{colors.qr-ink}', border: '{colors.control-border}', border-dark: '{colors.control-border-dark}', radius: '{rounded.xs}' }
  direct-url-row: { background: '{colors.surface}', background-dark: '{colors.surface-dark}', foreground: '{colors.text}', foreground-dark: '{colors.text-dark}', border: '{colors.control-border}', border-dark: '{colors.control-border-dark}', radius: '{rounded.sm}' }
  copy-feedback: { foreground: '{colors.success}', foreground-dark: '{colors.success-dark}', typography: '{typography.control.fontSize}' }
  trusted-lan-note: { foreground: '{colors.muted}', foreground-dark: '{colors.muted-dark}', marker: '{colors.warning}', marker-dark: '{colors.warning-dark}' }
  warning-banner: { background: '{colors.surface}', background-dark: '{colors.surface-dark}', foreground: '{colors.warning}', foreground-dark: '{colors.warning-dark}', border: '{colors.warning}', border-dark: '{colors.warning-dark}', radius: '{rounded.md}' }
  transfer-view: { background: '{colors.elevated}', background-dark: '{colors.elevated-dark}', foreground: '{colors.text}', foreground-dark: '{colors.text-dark}', radius: '{rounded.lg}' }
  progress-meter: { track: '{colors.drop}', track-dark: '{colors.drop-dark}', fill: '{colors.progress}', fill-dark: '{colors.progress-dark}', border: '{colors.control-border}', border-dark: '{colors.control-border-dark}', radius: '{rounded.full}', height: 10px }
  transfer-metrics: { foreground: '{colors.text}', foreground-dark: '{colors.text-dark}', secondary: '{colors.muted}', secondary-dark: '{colors.muted-dark}', typography: '{typography.meta.fontSize}' }
  cancel-action: { foreground: '{colors.muted}', foreground-dark: '{colors.muted-dark}', hover: '{colors.error}', hover-dark: '{colors.error-dark}', height: '{spacing.target-min}' }
  done-panel: { background: '{colors.surface}', background-dark: '{colors.surface-dark}', foreground: '{colors.success}', foreground-dark: '{colors.success-dark}', border: '{colors.success}', border-dark: '{colors.success-dark}', radius: '{rounded.lg}' }
  error-panel: { background: '{colors.surface}', background-dark: '{colors.surface-dark}', foreground: '{colors.error}', foreground-dark: '{colors.error-dark}', border: '{colors.error}', border-dark: '{colors.error-dark}', radius: '{rounded.lg}' }
  status-announcer: { position: 'visually-hidden', foreground: '{colors.text}', foreground-dark: '{colors.text-dark}' }
---

# FairDrop — Design Spine

> Selected direction: **Paper Relay** with **Terracotta Linen**. Exploration references: [design directions](.working/design-directions.html) and [color themes](.working/color-themes-paper-relay.html). The `DESIGN.md` and `EXPERIENCE.md` spines win on every conflict with a mockup, wireframe, or import.

**Source precedence:** the canonical `SPEC.md` and its binding architecture/contracts companions control. `epics.md` supplies the approved decomposition. The corrected `docs/fairdrop-spec.md` supplies narrative only where it does not conflict. Frontmatter contains only this confirmed source set.

## Brand & Style

FairDrop is a compact handoff tool, not a dashboard. Paper Relay makes one file or folder feel like a recognizable packet passed between people: warm paper surfaces, a restrained terracotta action color, quiet editorial headings, and standard OS window chrome. It should feel familial and reassuring without implying storage, postal delivery, receiver identity, or transport security.

The hierarchy is one current item, one next action, and one honest status. Decoration never competes with the QR code, progress, network disclosure, error recovery, or Cancel action. Light and dark modes follow the operating-system preference; V1 has no theme control. In forced-colors mode, the system palette supersedes Terracotta Linen.

## Colors

Terracotta Linen is an authored light/dark semantic pair, not a tint-generation recipe. Use the exact hex values above in ordinary color modes.

| Role | Light / dark | Rule |
|---|---|---|
| Canvas and surfaces | `{colors.canvas}` / `{colors.canvas-dark}`; `{colors.surface}` / `{colors.surface-dark}`; `{colors.elevated}` / `{colors.elevated-dark}` | Canvas frames the task; elevated is the paper packet, not a general card layer. |
| Text | `{colors.text}` / `{colors.text-dark}`; `{colors.muted}` / `{colors.muted-dark}` | Muted is readable copy, never disabled text. |
| Decorative edge | `{colors.border}` / `{colors.border-dark}` | Paper offsets and nonessential dividers only; never the sole boundary for a control, drop target, QR, progress track, or status. |
| Functional boundary | `{colors.control-border}` / `{colors.control-border-dark}` | Required for controls, rest-state drop target, QR frame, URL field, and progress-track outline. |
| Action | `{colors.primary}` / `{colors.primary-dark}` with the matching ink | Copy and the single strongest action only; not status decoration. |
| Transfer | `{colors.drop}` / `{colors.drop-dark}` and `{colors.progress}` / `{colors.progress-dark}` | Track and fill; active drop fill is decorative because its solid action-colored boundary carries the state. |
| Focus and outcomes | Focus, success, warning, and error pairs | Always pair color with outline, pattern, glyph, or literal text. |
| QR | `{colors.qr-surface}` / `{colors.qr-ink}` in both modes | Fixed high-contrast substrate; never recolor, invert, texture, rotate, round modules, or overlay a logo. |

WCAG 2.2 targets are ≥4.5:1 for normal text, ≥3:1 for large text, and >3:1 without rounding for load-bearing non-text boundaries, focus indicators, and value distinctions. Automated sRGB checks for the exact tokens produce:

| Load-bearing pair | Light ratio | Dark ratio |
|---|---:|---:|
| Functional boundary / canvas | 3.891771184:1 | 4.172240802:1 |
| Functional boundary / surface | 4.239569145:1 | 3.810460077:1 |
| Functional boundary / elevated | 3.457309226:1 | 3.366433989:1 |
| Progress fill / track | 3.606554351:1 | 5.272423913:1 |
| Active/action boundary / surface | 5.598738685:1 | 7.583348709:1 |
| Focus / weakest adjacent authored surface | 4.986787078:1 | 6.328871109:1 |

Text checks remain: text/canvas 13.064952890:1 light and 15.360550226:1 dark; muted/canvas 5.070532556:1 and 8.142330404:1; primary/primary-ink 5.811100200:1 and 8.304735473:1; QR ink/surface 16.396272390:1. Status rules against their surface also exceed 5.14:1 light and 7.05:1 dark. Re-run unrounded automated checks if opacity, blending, color-mix, or adjacent surfaces change.

When `forced-colors: active`, use system colors for text, surfaces, controls, borders, status rules, progress, and focus; retain text, glyph, length, and pattern distinctions. `forced-color-adjust: none` is forbidden except on the production QR bitmap and white quiet-zone substrate, and only after native scan evidence confirms it remains readable. Give the DropZone a visible system-color boundary.

## Typography

Use only system-safe stacks. `{typography.display}` and `{typography.headline}` provide the paper voice for one state heading and the item name. Functional copy uses `{typography.body}`; actions use `{typography.control}`; status and size/speed use `{typography.meta}`; the readonly direct URL uses `{typography.code}`.

Do not load a web font, use all-caps paragraphs, or render body copy below 12px. Render the complete sanitized item name in a bidi-isolated `<bdi dir="auto">` or equivalent. Keep adjacent metadata in separate isolates, use `overflow-wrap:anywhere` and `min-inline-size:0`, and never truncate by JavaScript code unit. A two-line visual clamp may be used only with a persistent keyboard-operable control labeled by `EXPERIENCE.md` key `copy.name.show_full` and an assistive description containing the complete value; a tooltip alone is insufficient. Test mixed LTR/RTL names, combining marks, emoji sequences, bidi controls, long unbroken names, and visible extension retention.

## Layout & Spacing

Use the compact 4/8/12/16/20/24/32/40 scale. App content normally uses `{spacing.window-gutter}`, with `{spacing.6}` between lifecycle regions and `{spacing.2}`–`{spacing.4}` within components. Interactive targets are at least `{spacing.target-min}` in both dimensions even when the visible control is quieter.

At the 1024×768 default window, constrain the transfer packet to a centered readable column. At the supported 640×480 native minimum, preserve actions and status before decorative space and permit vertical scrolling rather than clipping.

At an effective content width of 320 CSS pixels, reflow into one vertical column with no page-level horizontal scrolling, information loss, overlap, or clipped actions. The QR may retain a scan-ready square but cannot force horizontal scrolling. Names and URLs may wrap or visually elide only under the full-value rules above; Copy, Cancel, Dismiss, and recovery controls remain reachable. Content containers grow under 200% text and WCAG text-spacing overrides; no fixed height may clip content.

Production reference index: [Idle and local preparation](mockups/key-idle-preparing.html) covers Idle and Stage Pending; [Staged folder](mockups/key-staged-folder.html) covers the QR-primary Staged layout and warning; [Transferring](mockups/key-transferring.html) covers all three progress modes; [terminal outcomes](mockups/key-outcomes.html) covers Done, Error, and retained Idle outcomes.

## Elevation & Depth

Depth comes from tonal layering and one paper-offset edge. **StagedView** may use the concrete `3px 3px 0 #CDBDAE` light shadow or `3px 3px 0 #55493F` dark shadow. These are decorative and intentionally separate from functional boundaries. **App Shell**, **TransferView**, **Done Panel**, and **Error Panel** remain flat. No glass blur, gradients behind text, ambient glows, or stacked card shadows. Native window chrome supplies the outermost elevation.

## Shapes

Use `{rounded.xs}` for QR framing, `{rounded.sm}` for small fields, `{rounded.md}` for controls and notices, and `{rounded.lg}` for the packet and outcomes. `{rounded.full}` is reserved for the progress track and small status markers—not buttons or containers.

## Components

Visual specs pair with behavioral rows of the same names in `EXPERIENCE.md`.

The production-reference index in **Layout & Spacing** covers every visual component family.

| Component | Light | Dark | Visual contract |
|---|---|---|---|
| **App Shell** | App-shell background/foreground | Dark pair | Standard OS chrome; one centered lifecycle region plus an optional retained outcome in Idle. |
| **DropZone** | Surface + functional boundary; active drop fill + primary boundary | Dark pair | Dashed rest boundary; solid boundary during native drag-active state. Fill is never the only state cue. |
| **Selection Controls** | Surface/text/functional boundary | Dark pair | Equal-weight File and Directory controls; no false primary choice. |
| **Stage Pending Card** | Elevated/text | Dark pair | Quiet flexible-height paper block; no authoritative-state badge. |
| **StagedView** | Elevated + decorative offset edge | Dark pair | Folder-tab silhouette once; QR and primary instruction remain dominant. |
| **Item Summary** | Text/muted | Dark pair | Serif name with robust full-name access; folder says it downloads as ZIP. |
| **QR Panel** | Fixed QR pair + functional frame | Same fixed QR pair | Square, crisp, generous quiet zone, no rotation or overlay. |
| **Direct URL Row** | Surface/text/functional frame | Dark pair | Readonly monospace URL beside the terracotta action named by `EXPERIENCE.md` key `copy.direct_link.action`; never a sender-side activation link. |
| **Copy Feedback** | Success text | Dark success text | Label changes to `EXPERIENCE.md` key `copy.copy.confirmation`; no toast, badge, or layout shift. |
| **Trusted-LAN Note** | Muted with warning marker | Dark pair | Literal plain-HTTP and local-network disclosure; never green or lock-shaped. |
| **Warning Banner** | Surface with warning rule/text | Dark pair | Inline, non-modal, no full yellow fill. |
| **TransferView** | Elevated/text | Dark pair | Packet identity remains while QR/link yield to progress. |
| **Progress Meter** | Outlined drop track / progress fill | Dark pair | Determinate value, static unknown pattern, or decorative known-empty track; no fake ZIP or empty-file percentage. |
| **Transfer Metrics** | Text/muted | Dark pair | Wire bytes first, throughput second; tabular numerals when supported. |
| **Cancel Action** | Muted, error on hover/focus | Dark pair | Quiet text action with full target; use the applicable `EXPERIENCE.md` cancellation copy key. |
| **Done Panel** | Surface with success rule/text | Dark pair | Text plus simple check glyph; retained Idle form adds a quiet Dismiss control. No confetti or green fill. |
| **Error Panel** | Surface with error rule/text | Dark pair | Safe heading/message/recovery; retained Idle form adds Dismiss. No raw diagnostics. |
| **Status Announcer** | Visually hidden | Visually hidden | Pre-mounted, atomic, and layout-free; it never duplicates focused content. |

## Do's and Don'ts

| Do | Don't |
|---|---|
| Make the selected item and next action immediately legible. | Use dashboard-like density or decorative hierarchy. |
| Preserve standard OS chrome and compact utility proportions. | Use frameless or floating-widget styling. |
| Use Terracotta Linen in ordinary modes and system colors in forced colors. | Disable forced-color adjustment across the app or generate dark mode by inversion. |
| Keep production QR rendering square and unmodified. | Rotate, tint, round modules, or overlay the FairDrop mark. |
