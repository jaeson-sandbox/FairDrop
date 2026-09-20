# Evidence: Story 7.1: Replace the Token Layer

## What changed

`frontend/src/style.css`'s `@theme` block and dark override now declare the
Quartz token set in full: canvas/surface/elevated, the new `fill`/`fill-strong`
pair, text/muted, the `separator`/`control-border` two-token split, the
`primary`/`primary-hi`/`primary-ink`/`primary-tint` action set, `track`,
`focus`, the three tinted outcome colors, the fixed QR pair, the full Quartz
type ramp (including the new `numeric` role), the `xs`–`xxl` radius scale, the
`window-gutter`/`control-height` spacing steps, and three `--shadow-sh-*`
elevation tokens declared once and again as exact (not derived) dark values.
The old `--shadow-paper` token, the bundled Nunito face, and the
`hover`/`drop`/`progress`/`border` Terracotta-only roles are gone.

Component rules changed only where a renamed or deleted token forced it:
`.fd-button--primary` now carries the one permitted gradient
(`--color-primary-hi` → `--color-primary`) plus a 1px inset highlight, and its
`:hover` uses `--color-primary-hi` in place of the deleted `--color-hover`.
`.fd-drop-zone.wails-drop-target-active`'s fill moved from the deleted
`--color-drop` to `--color-primary-tint`. `.fd-meter`'s track and
`.fd-meter--unknown`/`.fd-meter__fill`'s fill moved from the deleted
`--color-drop`/`--color-progress` to `--color-track`/`--color-primary`.
Every plain decorative border (`.fd-packet`, `.fd-help`, `.fd-trust`,
`.fd-metric`) moved from the renamed `--color-border` to `--color-separator`.
`.fd-packet`'s `box-shadow` moved from the deleted `--shadow-paper` to
`--shadow-sh-3`. No layout, markup, or component structure changed.

The forced-colors block was rewritten for the new token set (every authored
role mapped to a system color) and gained an explicit
`.fd-button--primary { background: Highlight; box-shadow: none; }` override so
the gradient and its highlight are actually dropped in that mode rather than
merely collapsing to two identical stops.

`frontend/src/ui/styles.test.ts` was rewritten to pin the Quartz spine with the
same rigor it pinned Terracotta Linen: every light/dark hex, the type ramp,
radii and spacing steps, the forced-colors token map, the decorative-edge
guarantee (now both a per-control positive/negative check and a sheet-wide
scan), the elevation/gradient guarantee that replaces the deleted
paper-offset assertion, and a rewritten "no font ships" block that retires the
Nunito-specific weight/faux-bold assertions in favor of two structural
checks: no `@font-face`, and no bundled font directory.

`_bmad-output/planning-artifacts/ux-designs/quartz-proposed-2026-09-20/` was
renamed to `ux-FairDrop-quartz-2026-09-20/` and
`ux-FairDrop-2026-08-23/` was deleted in the same commit, keeping the
one-`ux-*`-folder invariant green throughout. `DESIGN.md`'s frontmatter
`status` moved from `draft` to `final`, the "Tables pending" note was removed,
and both contrast tables were filled with figures copied verbatim from
`styles.test.ts`'s own computed output (never hand-computed) -- see
"Contrast figures" below. `frontend/src/assets/fonts/` was deleted along with
the `@font-face` rule.

## An undocumented dependency the story didn't name

`EXPERIENCE.md` lived inside `ux-FairDrop-2026-08-23/`, the folder Story 7.1's
scope explicitly names for deletion. But DESIGN.md's own prose says
"`EXPERIENCE.md` continues to control copy, focus routing, and announcements,
and is unchanged by this spine," and `main_test.go` /
`release_identity_test.go` hardcode its path at that exact location to prove
the cross-language error registry and the shipped-surface inventory. Deleting
the folder wholesale would have silently broken both tests and removed the
canonical behavior source out from under the product.

This was not resolved by the story text or by epics.md, so I made the
smallest change that keeps the invariant both documents state: `EXPERIENCE.md`
moved, byte-for-byte unchanged (`diff` confirmed identical), into
`ux-FairDrop-quartz-2026-09-20/` alongside the new `DESIGN.md`, and the two
hardcoded path references in `main_test.go` and `release_identity_test.go`
were updated to the new folder name. This is a path update, not a content or
behavior change. Flagging this for the story owner: the spine should either
say explicitly that `EXPERIENCE.md` travels with each new UX folder, or give
it a location independent of the dated per-spine folders so a future spine
replacement doesn't have to rediscover this.

## Contrast figures

Every figure below is `styles.test.ts`'s own `toFixed(9)` output, captured via
a temporary dump test run against the real tokens, then deleted; DESIGN.md's
two tables and prose were filled from that output, and the real
`the unrounded contrast proof` suite (12 tests) now recomputes and matches
every one of them against DESIGN.md on each run.

**Text pairs (≥4.5:1, unrounded):**

| Pair | Light | Dark |
|---|---:|---:|
| text/canvas | 15.542768731 | 16.163110010 |
| text/surface | 17.377657264 | 14.704322177 |
| text/elevated | 17.377657264 | 13.308196422 |
| muted/canvas | 5.178387528 | 6.630453856 |
| muted/surface | 5.789717726 | 6.032027848 |
| muted/elevated | 5.789717726 | 5.459307165 |
| error/elevated | 5.518575206 | 5.859450762 |
| primary-ink/primary | 5.060845348 | 5.784694439 |

**Load-bearing non-text pairs (≥3:1, unrounded):**

| Pair | Light | Dark |
|---|---:|---:|
| control-border/canvas | 3.240328251 | 4.245594789 |
| control-border/surface | 3.622862486 | 3.862412220 |
| control-border/elevated | 3.622862486 | 3.495689217 |
| primary/track | 4.173974302 | 4.011363202 |
| primary/surface | 5.060845348 | 5.824411172 |
| primary/elevated | 5.060845348 | 5.271402992 |
| warning/elevated | 6.329195349 | 6.871224941 |
| focus/elevated | 5.060845348 | 6.610678352 |

**Fixed QR substrate:** `qr-ink` on `qr-surface` is 17.377657264 in both
modes (never recolored).

**Status-on-surface floor:** the weakest of warning/success/error on
`surface` exceeds 5.36:1 light (actual 5.365600475, `success`) and 6.47:1 dark
(actual 6.474149393, `error`).

**Weakest-of-three on canvas:** warning/success/error against `canvas` is
weakest at 4.799052371 light (`success`) and 7.116437439 dark (`error`).

**Weakest-adjacent for focus:** `focus` against canvas/surface/elevated is
weakest at 4.526476017 (light), against `canvas` -- unlike Terracotta Linen,
where `elevated` was weakest, because Quartz's `focus` token equals `primary`
and canvas is the more saturated-relative surface. The test asserts this
explicitly rather than assuming the old shape held.

**Decorative edge (excluded from both tables, never load-bearing):**
`separator` against surface/elevated is 1.453401544 light / 1.775620130 (surface)
and 1.607030993 (elevated) dark; against canvas it is 1.299938406 light /
1.951776026 dark. All comfortably below 3:1 in both modes, as designed.

## Mutation table

Each mutation was applied to the real working tree, run against the real
suite, confirmed to fail and name the problem, then reverted before the next
one. Two mutations (M6, M7) additionally exercised the Go suite.

| # | Mutation | AC | Result |
|---|---|---|---|
| M1 | dark `--color-canvas` changed from `#161618` to `#E9E9E7` (an inverted-looking value) | AC1 | KILLED -- `declares the authored dark half as exact values rather than an inversion`, plus 6 contrast-proof tests on `canvas` and `publishes every figure...`; 9 tests failed total |
| M2 | DESIGN.md's `text/canvas` light figure hand-edited by one digit (`...731` -> `...732`) | AC2 | KILLED -- `publishes every figure it proves, unrounded, in DESIGN.md` |
| M3 | `.fd-button`'s `border` swapped from `var(--color-control-border)` to `var(--color-separator)` | AC3 | KILLED -- both `.fd-button draws its boundary with the functional token, not the decorative one` and `never uses the decorative edge as the sole boundary of a control anywhere in the sheet`, naming `.fd-button` |
| M4 | reintroduced an `@font-face { font-family: "Nunito"; ... }` rule at the end of the sheet | AC4 | KILLED -- `declares no @font-face and bundles no font file` |
| M5a | `.fd-meter__fill`'s background changed to a second `linear-gradient(...)` | AC5 | KILLED -- `pins the three elevation tokens and the single-gradient rule` (gradient count 2, expected 1) |
| M5b | `.fd-headline` given a `linear-gradient` background plus `background-clip: text` / `-webkit-background-clip: text` | AC5 | KILLED -- same test (gradient count 2, and the `background-clip: text` assertion) |
| M5c | `--shadow-sh-3` token definition extended to four comma-separated shadow layers | AC5 | KILLED -- same test, once the test itself was fixed to expand `--shadow-sh-*` token definitions rather than only literal `box-shadow:` declarations (see "A gap this mutation found," below) |
| M6 | `canvasFor(true)` in `main.go` changed from `{0x16,0x16,0x18}` to `{0x00,0x00,0x00}` | AC8 | KILLED -- Go test `TestAppOptionsBackgroundTracksTheCanvasToken/dark` |
| M7 | a second `ux-*` folder (`ux-FairDrop-extra-test/`) created alongside the real one | AC9 | KILLED -- `a ux-* design folder: expected [...] to have a length of 1 but got 2`, taking the whole 78-test file down as "no tests" (the documented failure shape) |

## A gap this mutation found

M5c initially survived: the "no more than three shadow layers" check only
parsed literal `box-shadow:` declarations, and `.fd-packet`'s `box-shadow:
var(--shadow-sh-3);` is a `var()` reference with no top-level comma to split
on, so a fourth layer added *inside* the `--shadow-sh-3` token definition was
invisible to the regex. Fixed by also parsing `--shadow-sh-[123]:` token
definitions with the same layer-counting logic; M5c was re-run afterward and
killed. This is the story's own warning about scoped mutation proof applied to
itself: the first version of the test proved only the literal-declaration
case, not the token-definition case a real palette edit would actually touch.

## Gate transcripts (macOS arm64, native)

Run in the order `.github/workflows/verify.yml` uses, immediately after the
mutation-testing pass above (working tree returned to clean before this run).
Go 1.27.0, Node (via `npm test`), Wails CLI 2.15.0, `CGO_ENABLED=1`.

- `wails build`: **PASS**. `Built '.../fairdrop.app/Contents/MacOS/fairdrop' in 7.445s.` One pre-existing linker warning (`object file ... built for newer 'macOS' version (13.0) than being linked (11.0)`), unrelated to this change and present before it.
- Bindings drift / `.gitkeep` check: **PASS**. `frontend/dist/.gitkeep` present; `git -c core.fileMode=false diff --quiet -- frontend/wailsjs` exit 0 (no drift); `scripts/verify-build-asset-drift.sh` exit 0, no output.
- `gofmt -l .`: **PASS**, no output.
- `go vet ./...`: **PASS**, no output.
- `go tool staticcheck ./...`: **PASS**, no output.
- `go test -count=1 ./...`: **PASS** -- `ok` for `fairdrop`, `internal/network`, `internal/qr`, `internal/server`, `internal/source`, `internal/stream`, `internal/transfer`, `scripts`, `scripts/mutationverdict`.
- `CGO_ENABLED=1 go test -count=1 -race ./...`: **PASS** -- `ok` for all nine packages (same list; `internal/stream` took ~105s under the race detector, the rest a few seconds each).
- `GOOS=windows GOARCH=amd64 go build ./...`: **PASS**.
- `GOOS=darwin GOARCH=arm64 go build ./...`: **PASS**.
- `GOOS=linux GOARCH=amd64 go build ./...`: **PASS**.
- `GOOS=darwin GOARCH=arm64 staticcheck ./...` (bare binary, per the documented pitfall): **PASS**, no output.
- `cd frontend && npm test -- --run` (not concurrent with `wails build`): **PASS** -- 17 files, 549 tests.
- `cd frontend && npm run test:browser` (rendered Chromium/Playwright accessibility suite): **PASS** -- 1 file, 17 tests, including the QR-panel-under-forced-colors capture, which regenerated `frontend/browser/captures/qr-panel-forced-colors.capture.png` because `--color-qr-ink` changed from `#221F1C` to `#1A1A1C` under Quartz. This is an expected content change, not a regression -- the QR substrate's own hex is fixed and unchanged; only the ink hex moved with the rest of the neutral palette.
- `git diff --check`: **PASS**, no output (no whitespace errors).
- Line-ending check (`git ls-files --eol | grep -E '(i|w)/(crlf|mixed)'`): **PASS**, no matches.

`TestAppOptionsBackgroundTracksTheCanvasToken` (light and dark subtests) and
`TestTheCrossLanguageErrorRegistryPinsEveryCodeAndMessage` (all 17 codes) were
additionally run individually and passed, confirming AC8 (canvas tracking)
and the `EXPERIENCE.md` path fix did not regress the registry cross-check.

## A mistake made and recovered during this work

While hand-scripting the forced-colors/dark-block reordering mutation (an
extra check beyond the ACs, not one of the nine above), a Python string-slice
script corrupted `frontend/src/style.css` -- it did not merely fail to apply
the intended swap, it dropped most of the file's `@media` blocks entirely.
Because the file had never been committed in its Quartz form, `git` had
nothing to restore from. The file was reconstructed from the complete, exact
content produced by every prior `Edit`/`Write` call in this session (all of
which are individually accounted for above), rewritten in one `Write` call,
and re-verified: `npm test -- --run src/ui/styles.test.ts` returned to 78/78,
the full frontend suite to 549/549, and the entire native gate above was
re-run from `wails build` onward afterward with everything green. The
forced-colors-ordering mutation itself was not re-attempted after the
recovery; that assertion is structurally unchanged from the pre-existing,
already-proven-correct Terracotta Linen version (only its comment text
changed, from "Terracotta Linen" to "Quartz"), so its behavior was not
considered a new claim requiring a fresh mutation proof.

## Nothing left open

All nine Story 7.1 acceptance criteria are implemented and mutation-verified
above. `separator` is asserted never to be the sole boundary of any of the six
curated control selectors, both individually (`it.each`) and via a sheet-wide
scan. The paper-offset assertion is deleted, not left passing against a
retired token. The Nunito-specific weight/faux-bold assertions are retired in
favor of the two structural no-font checks AC4 calls for. `main_test.go`
confirms `BackgroundColour` still tracks `--color-canvas` in both schemes
against the new values, verified rather than assumed, per the story's
explicit instruction. Exactly one `ux-*` folder exists, and it is the Quartz
one.

Two things deliberately deferred, both explicitly out of this story's scope
per epics.md: component visual rebuild (radii/shadow assignment beyond what a
deleted/renamed token forced, e.g. `.fd-packet` keeps its asymmetric
Paper-Relay-shaped corners) is Story 7.2 onward; `main.go`'s
`Mac.Preferences.TabFocusesLinks` setting, which DESIGN.md's Colors section
calls "new, and load-bearing on macOS" for focus-ring reachability, is not
touched here because it is Go window-configuration code, not the token layer,
and its own pinning test does not yet exist -- flagging it so a later story in
this epic does not lose the requirement.
