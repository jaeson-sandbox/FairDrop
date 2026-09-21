import {existsSync, readFileSync, readdirSync} from 'node:fs'
import {resolve} from 'node:path'
import {describe, expect, it} from 'vitest'

/*
  jsdom performs no layout and evaluates no media query, so the reflow and
  target-size guarantees are proved against the stylesheet itself. Every hex,
  breakpoint and size below is a literal written at the assertion site, taken
  from DESIGN.md -- not read back out of the file under test.
*/

// Vitest roots at frontend/, so the project is one level up from the css it serves.
const projectRoot = process.cwd()
const repositoryRoot = resolve(projectRoot, '..')

// Normalized on read: the block parser below slices on a literal newline, and
// core.autocrlf checks this file out as CRLF on Windows. Without this the
// parser throws at module scope and every assertion in the file silently stops
// existing. .gitattributes pins the checkout too; this makes the test immune
// either way.
const stylesheet = readFileSync(resolve(projectRoot, 'src/style.css'), 'utf8').replace(/\r\n/g, '\n')

/**
 * The UX spine, found rather than spelled. The directory carries the date it
 * was authored, so naming it here made re-dating or moving the folder fail this
 * whole suite as a missing file instead of as contrast drift.
 */
function designSpinePath(): string {
    const designs = resolve(repositoryRoot, '_bmad-output/planning-artifacts/ux-designs')
    const folders = readdirSync(designs).filter((entry) => entry.startsWith('ux-'))

    expect(folders, 'a ux-* design folder').toHaveLength(1)
    return resolve(designs, folders[0], 'DESIGN.md')
}

function block(opening: string): string {
    const start = stylesheet.indexOf(opening)
    expect(start, opening).toBeGreaterThan(-1)
    const end = stylesheet.indexOf('\n}\n', start)
    expect(end, `${opening} close`).toBeGreaterThan(start)
    return stylesheet.slice(start, end)
}

const theme = block('@theme {')
const dark = block('@media (prefers-color-scheme: dark) {')
const forcedColors = block('@media (forced-colors: active) {')
const reducedMotion = block('@media (prefers-reduced-motion: reduce) {')
const componentRules = stylesheet
    .replace(theme, '')
    .replace(dark, '')
    .replace(forcedColors, '')
    // Reduced motion belongs out too: its universal-selector rules are not
    // component rules, and leaving them in let them satisfy assertions about
    // what components declare.
    .replace(reducedMotion, '')

describe('the stylesheet parser sees the whole file', () => {
    /*
      componentRules is built by subtracting the at-rule blocks. Round 1 found
      reduced-motion leaking into every component assertion because it was not
      subtracted; adding it fixed that instance without making the next one
      fail. This counts the blocks instead, so a new at-rule has to be
      classified deliberately rather than silently satisfying assertions about
      what components declare.
    */
    it('subtracts every at-rule block it knows about, and knows about all of them', () => {
        const atRules = [...stylesheet.matchAll(/^@(media|supports|theme)[^{]*\{/gm)].map((m) => m[0])

        expect(atRules).toEqual([
            '@theme {',
            '@media (prefers-color-scheme: dark) {',
            '@media (max-width: 759px) {',
            '@media (max-width: 639px) {',
            '@media (forced-colors: active) {',
            '@media (prefers-reduced-motion: reduce) {',
        ])
    })
})

describe('the Quartz token layer', () => {
    it('declares every Quartz light value as a Tailwind v4 theme variable', () => {
        const light: Record<string, string> = {
            canvas: '#F2F2F4',
            surface: '#FFFFFF',
            elevated: '#FFFFFF',
            fill: '#F1F1F2',
            'fill-strong': '#E8E8EA',
            text: '#1A1A1C',
            muted: '#65656B',
            separator: '#D6D6D6',
            'control-border': '#86868B',
            primary: '#0A6CD8',
            'primary-hi': '#2B86EE',
            'primary-ink': '#FFFFFF',
            'primary-tint': '#E6EFFB',
            track: '#E9E9EB',
            focus: '#0A6CD8',
            success: '#177A48',
            'success-tint': '#E3F1EA',
            warning: '#8A5300',
            'warning-tint': '#F6EDE0',
            error: '#C0362C',
            'error-tint': '#F8E9E7',
            'qr-surface': '#FFFFFF',
            'qr-ink': '#1A1A1C',
        }

        for (const [role, value] of Object.entries(light)) {
            expect(theme, role).toContain(`--color-${role}: ${value};`)
        }
    })

    it('declares the authored dark half as exact values rather than an inversion', () => {
        const darkPair: Record<string, string> = {
            canvas: '#161618',
            surface: '#1F1F22',
            elevated: '#27272B',
            fill: '#313135',
            'fill-strong': '#3A3A3E',
            text: '#F2F2F4',
            muted: '#9C9CA4',
            separator: '#47474A',
            'control-border': '#7A7A82',
            primary: '#4C9BFF',
            'primary-hi': '#6FB0FF',
            'primary-ink': '#06203F',
            'primary-tint': '#23303F',
            track: '#3A3A3E',
            focus: '#6FB0FF',
            success: '#4ED08B',
            'success-tint': '#20342A',
            warning: '#E7A33A',
            'warning-tint': '#372E1D',
            error: '#FF7A70',
            'error-tint': '#3A2422',
        }

        for (const [role, value] of Object.entries(darkPair)) {
            expect(dark, role).toContain(`--color-${role}: ${value};`)
        }
        for (const filter of ['invert(', 'hue-rotate(', 'color-mix(']) {
            expect(dark, filter).not.toContain(filter)
        }
    })

    it('keeps the QR substrate out of the dark override', () => {
        expect(dark).not.toContain('--color-qr-surface')
        expect(dark).not.toContain('--color-qr-ink')
    })

    it('declares the type ramp, radii and spacing steps DESIGN.md publishes', () => {
        for (const declaration of [
            '--text-display: 26px;',
            '--text-headline: 20px;',
            '--text-numeric: 30px;',
            '--text-body: 13.5px;',
            '--text-label: 12px;',
            '--text-code: 12px;',
            '--text-control: 14px;',
            '--font-weight-display: 650;',
            '--font-weight-body: 400;',
            '--font-weight-body-strong: 590;',
            '--font-weight-control: 590;',
            '--radius-xs: 6px;',
            '--radius-sm: 9px;',
            '--radius-md: 11px;',
            '--radius-lg: 14px;',
            '--radius-xl: 18px;',
            '--radius-xxl: 24px;',
            '--radius-full: 9999px;',
            '--spacing-window-gutter: 24px;',
            '--spacing-target-min: 44px;',
        ]) {
            expect(theme, declaration).toContain(declaration)
        }
    })

    it('reaches every component value through a token rather than a literal', () => {
        // Hex is not the only way to write a color. A named color, an rgb()/
        // hsl()/oklch() triple or a color-mix() all reach the screen the same
        // way and all used to pass this.
        const colorLiteral = new RegExp([
            '#[0-9A-Fa-f]{3,8}\\b',
            '\\b(?:rgba?|hsla?|hwb|lab|lch|oklab|oklch|color|color-mix)\\(',
            // Hyphen-guarded on both sides: a bare `\\b` matches the `white`
            // inside `white-space`, which is a property name, not a color.
            '(?<![-\\w])(?:red|blue|green|black|white|gray|grey|orange|purple|pink|brown|yellow|cyan|magenta)(?![-\\w])',
        ].join('|'), 'g')

        // The primary button's 1px inset highlight is the one authored literal
        // this spine permits: DESIGN.md scopes it to that single gradient's top
        // edge, and it is not a color role a token could carry -- it is a fixed
        // white at a fixed opacity, unrelated to any theme color.
        const withoutHighlight = componentRules.replace('rgb(255 255 255 / 0.4)', '')

        expect(withoutHighlight.match(colorLiteral) ?? []).toEqual([])
    })
})

describe('following the operating-system scheme', () => {
    it('declares color-scheme unconditionally, so first paint cannot flash the other theme', () => {
        expect(stylesheet).toMatch(/:root\s*\{\s*color-scheme: light dark;/)
    })

    it('switches on the OS preference and offers no theme control', () => {
        expect(stylesheet).toContain('@media (prefers-color-scheme: dark)')
        expect(stylesheet).not.toContain('data-theme')
        expect(stylesheet).not.toContain('.dark ')
    })

    it('disables forced-color adjustment on the QR substrate and nowhere else', () => {
        // DESIGN.md allows exactly one exemption -- the production QR bitmap
        // and its quiet-zone substrate -- because a code forced into the system
        // palette loses the quiet zone a scanner needs. Applied anywhere else
        // it would opt the whole app out of the user's own palette.
        const declarations = stylesheet.match(/forced-color-adjust\s*:/g) ?? []

        expect(declarations).toHaveLength(1)
        expect(forcedColors).toMatch(
            /\.fd-qr-panel,\s*\.fd-qr \{\s*forced-color-adjust: none;\s*\}/,
        )
    })
})

describe('forced colors', () => {
    /*
      The whole app reads colors through var(), so overriding the tokens is what
      makes system colors reach every surface. A distinction the user agent
      would flatten -- the progress fill against its track, the action color, the
      focus ring -- is restated as a system color rather than left to it.
    */

    it('supersedes Quartz with system colors on every authored token', () => {
        const systemColors: Record<string, string> = {
            canvas: 'Canvas',
            surface: 'Canvas',
            elevated: 'Canvas',
            fill: 'Canvas',
            'fill-strong': 'Canvas',
            text: 'CanvasText',
            muted: 'CanvasText',
            separator: 'CanvasText',
            'control-border': 'CanvasText',
            primary: 'Highlight',
            'primary-hi': 'Highlight',
            'primary-ink': 'HighlightText',
            'primary-tint': 'Canvas',
            track: 'Canvas',
            focus: 'Highlight',
            success: 'CanvasText',
            'success-tint': 'Canvas',
            warning: 'CanvasText',
            'warning-tint': 'Canvas',
            error: 'CanvasText',
            'error-tint': 'Canvas',
        }

        for (const [role, value] of Object.entries(systemColors)) {
            expect(forcedColors, role).toContain(`--color-${role}: ${value};`)
        }

        // Every authored role is covered: a token added to @theme without a
        // forced-colors answer keeps its Quartz value in the system palette.
        const authored = [...theme.matchAll(/--color-([a-z-]+):/g)].map(([, role]) => role)
        const uncovered = authored.filter((role) => !(role in systemColors) && !role.startsWith('qr-'))
        expect(uncovered).toEqual([])
    })

    it('keeps the progress fill distinguishable from its own track', () => {
        // Highlight on Canvas. Left to the user agent both would become Canvas
        // and a determinate meter would read as empty at every percentage.
        expect(forcedColors).toContain('--color-primary: Highlight;')
        expect(forcedColors).toContain('--color-track: Canvas;')
    })

    it('drops the primary button gradient and its highlight, which have no system color', () => {
        expect(forcedColors).toMatch(/\.fd-button--primary \{\s*background: Highlight;\s*box-shadow: none;\s*\}/)
    })
})

describe('reduced motion', () => {
    it('neutralises any animation or transition a later edit could add', () => {
        expect(reducedMotion).toContain('animation-duration: 1ms !important;')
        expect(reducedMotion).toContain('animation-iteration-count: 1 !important;')
        expect(reducedMotion).toContain('transition-duration: 1ms !important;')
        expect(reducedMotion).toMatch(/\*,\s*\*::before,\s*\*::after/)
    })

    it('removes nothing that carries meaning', () => {
        // Text, the static unknown pattern, wire bytes and state are all
        // painted rather than moved, so none of them can be inside this block.
        for (const forbidden of ['display:', 'visibility:', 'content:', 'opacity:', 'background']) {
            expect(reducedMotion, forbidden).not.toContain(forbidden)
        }
    })
})

describe('the focus indicator', () => {
    it('draws one ring from the focus token for the two Tab-reachable controls', () => {
        expect(stylesheet).toMatch(
            /\.fd-button:focus-visible,\s*\.fd-url:focus-visible \{\s*/,
        )
        expect(stylesheet).toContain('outline: var(--focus-ring-width) solid var(--color-focus);')
        expect(stylesheet).toContain('outline-offset: var(--focus-ring-offset);')
        expect(stylesheet).toContain('--focus-ring-width: 3px;')
    })

    it('never rings a routed landing target, even when focus on it is visible', () => {
        /*
          These nodes carry tabindex="-1" so the routing table can reach them,
          which also makes them click- and script-focusable, but a keyboard
          user can never Tab to one. A shared `:focus-visible` rule painted the
          ring on every scripted focus move regardless of input modality --
          there is no "last real interaction" for `:focus-visible` to inherit
          from when a transfer completes on its own -- so the Done panel read
          as stuck "selected" long after the transfer finished, reported from
          the running app. The fix is an explicit `outline: none` rather than a
          removed rule, because an omitted rule would leave the browser's own
          default focus-visible outline in place.
        */
        expect(stylesheet).toContain('[data-focus-target]:focus-visible {')
        expect(stylesheet).toMatch(/\[data-focus-target\]:focus-visible \{\s*outline: none;\s*\}/)
        expect(stylesheet).not.toMatch(/\[data-focus-target\]:focus \{/)
        // Not folded into the controls' ring rule: a shared selector list is
        // exactly the regression this pins.
        expect(stylesheet).not.toMatch(/\.fd-button:focus-visible,[^{]*\[data-focus-target\]/)
    })
})

describe('activation targets', () => {
    it('gives every target the 44px floor in both dimensions through one rule', () => {
        expect(stylesheet).toMatch(
            /\.fd-target \{\s*min-block-size: var\(--spacing-target-min\);\s*min-inline-size: var\(--spacing-target-min\);\s*\}/,
        )
        expect(theme).toContain('--spacing-target-min: 44px;')
    })
})

describe('reflow to 320 CSS pixels', () => {
    it('keeps details beside the QR only above the 760px content width', () => {
        expect(stylesheet).toMatch(/\.fd-hero \{[^}]*grid-template-columns: minmax\(0, 1fr\) 224px;/)
        expect(stylesheet).toContain('@media (max-width: 759px)')
    })

    it('stacks the QR above the URL row and its action below 760px', () => {
        const narrow = block('@media (max-width: 759px) {')
        expect(narrow).toContain('.fd-hero')
        expect(narrow).toContain('grid-template-columns: minmax(0, 1fr);')
        expect(narrow).toContain('.fd-qr-panel')
        expect(narrow).toContain('order: -1;')
        expect(narrow).toContain('.fd-direct-row')
    })

    it('collapses the remaining pair into one column below 640px', () => {
        const narrowest = block('@media (max-width: 639px) {')
        expect(narrowest).toContain('.fd-metrics')
        expect(narrowest).toContain('grid-template-columns: minmax(0, 1fr);')
    })

    it('drops the single browse control out of the pair-collapse media query', () => {
        // fd-selection held two equal-weight buttons and needed the collapse;
        // one control with an absolutely positioned menu (spec-4-1) does not.
        const narrowest = block('@media (max-width: 639px) {')
        expect(narrowest).not.toContain('.fd-selection')
    })

    it('declares no width that could force a page-level horizontal scrollbar', () => {
        // grid-template-columns and flex-basis are the two that actually
        // overflowed in review: a 900px track passed the width-only guard.
        // `max-width` stays excluded by the leading boundary, since capping a
        // width cannot cause overflow.
        const declarations = [...stylesheet.matchAll(
            /(?:^|[;{\s])((?:min-)?(?:width|inline-size)|grid-template-columns|flex-basis)\s*:\s*([^;}]+)/g,
        )]
        expect(declarations.length).toBeGreaterThan(0)

        for (const [, property, value] of declarations) {
            for (const [, pixels] of value.matchAll(/(\d+)px/g)) {
                expect(Number(pixels), `${property}: ${value.trim()}`).toBeLessThanOrEqual(320)
            }
        }
    })

    it('lets long names wrap and keeps the URL field inside its own column', () => {
        expect(stylesheet).toMatch(/\.fd-headline \{[^}]*overflow-wrap: anywhere;/)

        /*
          `inline-size: 100%` is not enough on its own, and the predecessor of
          this test asserted the opposite. A grid item's `min-inline-size`
          defaults to `auto`, which resolves to its content-based minimum, and
          an input's is its intrinsic size -- roughly twenty characters. The
          percentage sets the preferred size; the automatic minimum is what
          stops the column shrinking, so without `min-inline-size: 0` the URL
          field forces a page-level horizontal scrollbar at 320px. The <div>
          this replaced carried the same declaration for the same reason.
        */
        expect(stylesheet).toMatch(/\.fd-url \{[^}]*inline-size: 100%;/)
        expect(stylesheet).toMatch(/\.fd-url \{[^}]*min-inline-size: 0;/)
    })
})

describe('guarantees a stylesheet edit could silently undo', () => {
    /*
      These four were each applied during review and each survived mutation
      until this block existed: the suite could not tell the fixed stylesheet
      from the broken one.
    */

    it('pins the stylesheet to LF at checkout, so the parser above cannot vanish', () => {
        // block() slices on a literal newline. core.autocrlf checks this file
        // out as CRLF on Windows, which throws at module scope and takes all
        // of these assertions with it -- reported as "no tests", not a failure
        // anyone would read as a stylesheet problem.
        const attributes = readFileSync(resolve(repositoryRoot, '.gitattributes'), 'utf8')

        expect(attributes).toMatch(/^\*\.css text eol=lf$/m)
    })

    it('marks only the not-encrypted disclosure, never the neutral one', () => {
        // DESIGN.md gives the trusted-LAN note a single warning marker. Applied
        // to every paragraph it also decorated "FairDrop does not upload or
        // store an extra copy", which is a plain statement of fact.
        expect(stylesheet).toContain('.fd-trust p:first-child::before');
        expect(stylesheet).not.toMatch(/\.fd-trust p::before/)
    })

    it('keeps the copy action a fixed width so its label swap cannot reflow the row', () => {
        expect(componentRules).toMatch(
            /\.fd-direct-row \.fd-button \{[^}]*min-inline-size: \d+px;/,
        )
    })

    it('pins the three elevation tokens and the single-gradient rule', () => {
        // The paper offset is gone; this is what replaces the assertion that
        // pinned it. Three tokens, declared once each in @theme and again as
        // exact dark values (proved above), plus exactly one gradient in the
        // whole product.
        for (const token of ['--shadow-sh-1:', '--shadow-sh-2:', '--shadow-sh-3:']) {
            expect(theme.match(new RegExp(token.replace(/[-[\]{}()*+?.,\\^$|#\s]/g, '\\$&'), 'g')) ?? [], token)
                .toHaveLength(1)
        }

        const gradients = [...componentRules.matchAll(/(?<!repeating-)linear-gradient\(/g)]
        expect(gradients).toHaveLength(1)

        const primaryButton = block('.fd-button--primary {')
        expect(primaryButton).toContain('linear-gradient(var(--color-primary-hi), var(--color-primary))')

        // Never behind text: the one gradient sits on a control's background,
        // and nothing clips a gradient to text anywhere in the sheet.
        expect(stylesheet).not.toContain('background-clip: text')
        expect(stylesheet).not.toContain('-webkit-background-clip: text')

        // No element carries more than three shadow layers. box-shadow layers
        // are comma-separated, but rgba()/rgb() commas are not layer
        // separators, so they are stripped before counting. This has to look
        // at both a literal `box-shadow:` declaration and the `--shadow-sh-*`
        // tokens it reads through var() -- a var() reference itself never has
        // a top-level comma to split on, so a layer added inside the token
        // definition would otherwise pass unseen.
        const shadowDeclarations = [
            ...stylesheet.matchAll(/box-shadow:\s*([^;]+);/g),
            ...stylesheet.matchAll(/--shadow-sh-[123]:\s*([^;]+);/g),
        ]
        expect(shadowDeclarations.length).toBeGreaterThan(0)

        for (const [, value] of shadowDeclarations) {
            const layers = value.replace(/rgba?\([^)]*\)/g, 'rgb').split(',')
            expect(layers.length, value.trim()).toBeLessThanOrEqual(3)
        }

        /*
          The primary control is the one place a `--shadow-sh-*` token and an
          extra literal layer sit in the same declaration (its 1px inset
          highlight). Counted separately, as the two loops above do, neither
          the token's own two layers nor the declaration's two comma-separated
          pieces (`var(--shadow-sh-1)` counts as one piece, `inset ...` as the
          other) can ever exceed 3 -- so a third layer added *inside* the sh-1
          token would still read as "2" from the declaration side and "3" from
          the token side, both individually under the ceiling, while the
          control would really be painting four. This resolves the var()
          reference against its own current definition and counts what the
          browser actually composites.
        */
        const sh1Definition = theme.match(/--shadow-sh-1:\s*([^;]+);/)?.[1] ?? ''
        expect(sh1Definition, 'sh-1 token value').toBeTruthy()
        const resolvedPrimaryShadow = primaryButton
            .match(/box-shadow:\s*([^;]+);/)?.[1]
            .replace('var(--shadow-sh-1)', sh1Definition) ?? ''
        expect(resolvedPrimaryShadow, 'resolved .fd-button--primary box-shadow').toBeTruthy()
        const resolvedLayers = resolvedPrimaryShadow.replace(/rgba?\([^)]*\)/g, 'rgb').split(',')
        expect(resolvedLayers.length, resolvedPrimaryShadow.trim()).toBeLessThanOrEqual(3)
    })
})

describe('the button family (Story 7.2)', () => {
    it('gives the primary control its elevation token, a 40px height token and the press scale', () => {
        const primary = block('.fd-button--primary {')
        // DESIGN.md's Elevation table has no dedicated button step; `sh-1` --
        // the lightest of the three -- is the closest match to "a resting
        // surface", and it is what keeps this control inside the three-layer
        // ceiling: two layers from the token plus the one inset highlight.
        expect(primary).toContain('box-shadow: var(--shadow-sh-1), inset 0 1px 0 rgb(255 255 255 / 0.4);')

        const base = block('.fd-button {')
        expect(base).toContain('block-size: var(--spacing-control-height);')

        // Every button, not only the primary one -- DESIGN.md's Motion section
        // makes the press scale general ("Buttons scale to 0.975 on :active.
        // Nothing else scales."), so this is one rule rather than one per
        // variant.
        expect(stylesheet).toMatch(/\.fd-button:active \{\s*transform: scale\(0\.975\);\s*\}/)
    })

    it('gives the secondary control a fill on top of the shared boundary, never instead of it', () => {
        const secondary = block('.fd-button--secondary {')
        // --color-fill-strong against --color-surface is 1.22:1 -- visible
        // enough to read, nowhere near load-bearing -- so the 1px
        // --color-control-border this rule inherits from .fd-button is what
        // actually identifies the control as operable.
        expect(secondary).toContain('background: var(--color-fill-strong);')
        // A `border`/`border-color` declaration here would substitute a
        // boundary rather than adding a fill on top of the shared one; this
        // is the mutation the acceptance criterion names ("remove the
        // boundary and keep the fill -> must fail").
        expect(secondary).not.toMatch(/\bborder(-color)?\s*:/)

        const base = block('.fd-button {')
        expect(base).toContain('var(--color-control-border)')
    })
})

describe('the browse menu surface (Story 7.2)', () => {
    it('is a rounded.lg surface at sh-3 with a functional boundary', () => {
        const menu = block('.fd-browse-menu {')
        expect(menu).toContain('border-radius: var(--radius-lg);')
        expect(menu).toContain('box-shadow: var(--shadow-sh-3);')
        expect(menu).toContain('var(--color-control-border)')
    })

    it('gives menu items the sm radius', () => {
        const items = block('.fd-browse-menu .fd-button {')
        expect(items).toContain('border-radius: var(--radius-sm);')
    })

    it('distinguishes the focused item by primary fill and a tint halo, not the ring alone', () => {
        const focused = block('.fd-browse-menu .fd-button:focus-visible {')
        expect(focused).toContain('background: var(--color-primary);')
        expect(focused).toMatch(/box-shadow:\s*0 0 0 4px var\(--color-primary-tint\);/)

        // The shared ring rule still applies to these items -- they are plain
        // .fd-button elements -- so the fill and halo above sit alongside it
        // rather than replacing it.
        expect(stylesheet).toMatch(/\.fd-button:focus-visible,\s*\.fd-url:focus-visible \{/)
    })
})

describe('the copy control takes the success tint (Story 7.2)', () => {
    it('paints the fill, not only the border and text', () => {
        const copied = block('.fd-button--copied {')
        expect(copied).toContain('background: var(--color-success-tint);')
        expect(copied).toContain('border-color: var(--color-success);')
    })
})

describe('the progress card and meter (Story 7.5)', () => {
    it('is a rounded.xxl surface at sh-3, the same step as the packet and the browse menu', () => {
        const card = block('.fd-transfer-view {')
        expect(card).toContain('border-radius: var(--radius-xxl);')
        expect(card).toContain('box-shadow: var(--shadow-sh-3);')
    })

    it('keeps the pending card at its own radius, unaffected by the progress card split', () => {
        // The two selectors shared one rule before this story; splitting them
        // is what lets the progress card take sh-3/xxl without moving the
        // stage-pending card, which Story 7.5 does not own.
        const pending = block('.fd-pending-card {')
        expect(pending).toContain('border-radius: var(--radius-lg);')
        expect(pending).not.toContain('box-shadow')
    })

    it('is an 8px rounded.full track with a functional boundary', () => {
        const meter = block('.fd-meter {')
        expect(meter).toContain('height: 8px;')
        expect(meter).toContain('border-radius: var(--radius-full);')
        expect(meter).toContain('var(--color-control-border)')
    })

    it('fills the track solid -- the single-gradient rule is absolute and the button already spends it', () => {
        const fill = block('.fd-meter__fill {')
        expect(fill).toContain('background: var(--color-primary);')
        expect(fill).not.toContain('gradient')
    })

    it('renders the percentage in {typography.numeric} with tabular numerals', () => {
        const percent = block('.fd-progress-percent {')
        expect(percent).toContain('font-size: var(--text-numeric);')
        expect(percent).toContain('font-variant-numeric: tabular-nums;')
    })
})

describe('progress presentation', () => {
    it('keeps the unknown pattern static: no sweep, shimmer, or blink', () => {
        expect(stylesheet).toMatch(/\.fd-meter--unknown \{[^}]*repeating-linear-gradient\(/)
        expect(stylesheet).not.toContain('@keyframes')
        expect(stylesheet).not.toContain('animation:')
    })
})

describe('the outcome panel (Story 7.5)', () => {
    it('is a centred rounded.xxl surface at sh-3', () => {
        const outcome = block('.fd-outcome {')
        expect(outcome).toContain('border-radius: var(--radius-xxl);')
        expect(outcome).toContain('box-shadow: var(--shadow-sh-3);')
        expect(outcome).toMatch(/align-items:\s*center;/)
    })

    it('gives the done and error discs their own tint, at the 74px DESIGN.md size', () => {
        const icon = block('.fd-outcome__icon {')
        expect(icon).toContain('width: 74px;')
        expect(icon).toContain('height: 74px;')
        expect(icon).toContain('border-radius: var(--radius-full);')

        const done = block('.fd-outcome__icon--done {')
        expect(done).toContain('background: var(--color-success-tint);')
        expect(done).toContain('color: var(--color-success);')

        const error = block('.fd-outcome__icon--error {')
        expect(error).toContain('background: var(--color-error-tint);')
        expect(error).toContain('color: var(--color-error);')
    })

    it('mutes the body copy', () => {
        const body = block('.fd-outcome__body {')
        expect(body).toContain('color: var(--color-muted);')
    })

    /*
      Mutation named in the acceptance criterion: leave the check at
      `stroke-dashoffset: 32` under reduced motion -> must fail, because the
      check is a state cue and removing it removes meaning. The resting rule
      below is what the reduced-motion universal transition-duration
      collapse resolves to -- nothing inside the reduced-motion block may
      override it back to 32.
    */
    it('draws the check via a transition, never a keyframe animation, and leaves it fully drawn under reduced motion', () => {
        const path = block('.fd-outcome__check-path {')
        expect(path).toContain('stroke-dasharray: 32;')
        expect(path).toContain('stroke-dashoffset: 32;')
        expect(path).toContain('transition: stroke-dashoffset 500ms var(--ease-decelerate);')
        expect(path).not.toContain('@keyframes')

        const drawn = block('.fd-outcome__check--drawn .fd-outcome__check-path {')
        expect(drawn).toContain('stroke-dashoffset: 0;')

        // Reduced motion must not re-hide the check by overriding its
        // resting value back to the undrawn offset -- the literal mutation
        // the acceptance criterion names.
        expect(reducedMotion).not.toContain('stroke-dashoffset: 32')
        expect(reducedMotion).not.toMatch(/\.fd-outcome__check/)
    })
})

describe('the completion receipt (Story 7.5)', () => {
    it('is two cells on {colors.fill}, divided by a separator', () => {
        const receipt = block('.fd-receipt {')
        expect(receipt).toContain('background: var(--color-fill);')
        expect(receipt).toContain('grid-template-columns: 1fr 1fr;')

        const divider = block('.fd-receipt__cell + .fd-receipt__cell {')
        expect(divider).toContain('var(--color-separator)')
    })

    it('carries no duration cell and no third column', () => {
        // The mutation this guards against is the worst possible outcome of
        // this story: inventing a displayed duration. Nothing in the sheet
        // may name a third receipt cell or a duration/elapsed rule.
        expect(stylesheet).not.toMatch(/\.fd-receipt__cell--(duration|elapsed|time)/)
        expect(stylesheet).not.toContain('grid-template-columns: 1fr 1fr 1fr')
    })

    it('renders the receipt figures with tabular numerals, like the percentage', () => {
        const value = block('.fd-receipt__value {')
        expect(value).toContain('font-variant-numeric: tabular-nums;')
    })
})

describe('forced colors beat the authored dark palette', () => {
    it('declares the forced-colors block after the dark one, which is the only reason it wins', () => {
        /*
          Both blocks redefine the same custom properties on bare `:root`, both
          match in a dark high-contrast theme, and their specificity is equal --
          so source order is the whole mechanism. Moving the dark block to the
          end of the file restores Quartz for a Windows High Contrast user with
          the whole suite green.
        */
        const darkAt = stylesheet.indexOf('@media (prefers-color-scheme: dark) {')
        const forcedAt = stylesheet.indexOf('@media (forced-colors: active) {')

        expect(darkAt).toBeGreaterThan(-1)
        expect(forcedAt).toBeGreaterThan(darkAt)
    })
})

describe('the Tailwind v4 setup', () => {
    it('imports Tailwind once, at the top', () => {
        expect(stylesheet.startsWith('@import "tailwindcss";')).toBe(true)
        expect(stylesheet.match(/@import "tailwindcss";/g)).toHaveLength(1)
    })

    it('carries no v3 or PostCSS configuration file', () => {
        for (const relative of [
            'frontend/tailwind.config.js',
            'frontend/tailwind.config.ts',
            'frontend/tailwind.config.cjs',
            'frontend/postcss.config.js',
            'frontend/postcss.config.cjs',
            'tailwind.config.js',
            'postcss.config.js',
        ]) {
            expect(existsSync(resolve(repositoryRoot, relative)), relative).toBe(false)
        }
    })
})

describe('the unrounded contrast proof', () => {
    /*
      The ratios DESIGN.md publishes are recomputed here from the tokens the
      stylesheet actually declares, so a palette edit that quietly breaks a pair
      fails at the assertion rather than at a reviewer's eye. The formula is
      written out rather than imported: a proof that shares an implementation
      with the thing it proves cannot fail.

      "Placed together" means the views really put this foreground on this
      background. `.fd-button--quiet` is what puts muted and error on elevated,
      because it drops the surface fill every other control keeps; `.fd-trust`'s
      marker is what puts warning there.
    */

    function channel(value: number): number {
        const c = value / 255
        return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
    }

    function luminance(hex: string): number {
        const digits = hex.replace('#', '')
        return 0.2126 * channel(Number.parseInt(digits.slice(0, 2), 16)) +
            0.7152 * channel(Number.parseInt(digits.slice(2, 4), 16)) +
            0.0722 * channel(Number.parseInt(digits.slice(4, 6), 16))
    }

    function contrast(foreground: string, background: string): number {
        const a = luminance(foreground)
        const b = luminance(background)
        return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05)
    }

    function declared(source: string): Record<string, string> {
        const found: Record<string, string> = {}
        for (const [, role, value] of source.matchAll(/--color-([a-z-]+):\s*(#[0-9A-Fa-f]{6});/g)) {
            found[role] = value
        }
        return found
    }

    const lightTokens = declared(theme)
    const darkTokens = {...lightTokens, ...declared(dark)}

    const designSpine = readFileSync(
        designSpinePath(),
        'utf8',
    )

    /** [foreground, background, minimum ratio, published as an exact figure]. */
    /*
      `separator` is absent on purpose. DESIGN.md calls it the decorative edge
      -- "dividers inside a surface, and nothing else... it deliberately does
      not meet 3:1 -- it is not a boundary, it is a rule between paragraphs" --
      and it would fail any load-bearing floor. The test below keeps it out of
      the places that would make it load-bearing, which is the guarantee that
      lets it stay out of this table.
    */
    const placed: Array<[string, string, number, boolean]> = [
        // Text: 4.5:1. Every one of these is body copy, a control label, or a
        // heading at a size the AA large-text allowance does not reach.
        ['text', 'canvas', 4.5, true],
        ['text', 'surface', 4.5, true],
        ['text', 'elevated', 4.5, true],
        ['muted', 'canvas', 4.5, true],
        ['muted', 'surface', 4.5, true],
        ['muted', 'elevated', 4.5, true],
        ['error', 'elevated', 4.5, true],
        ['primary-ink', 'primary', 4.5, true],
        // Status text on its own panel: published as a floor, checked below.
        ['warning', 'surface', 4.5, false],
        ['success', 'surface', 4.5, false],
        ['error', 'surface', 4.5, false],

        // Load-bearing non-text: 3:1, unrounded.
        ['control-border', 'canvas', 3, true],
        ['control-border', 'surface', 3, true],
        ['control-border', 'elevated', 3, true],
        ['primary', 'track', 3, true],
        ['primary', 'surface', 3, true],
        ['primary', 'elevated', 3, true],
        ['warning', 'elevated', 3, true],
        ['focus', 'elevated', 3, true],
        // Status rules and focus against the stronger surfaces are published as
        // the weakest-adjacent claim rather than one row each.
        ['warning', 'canvas', 3, false],
        ['success', 'canvas', 3, false],
        ['error', 'canvas', 3, false],
        ['focus', 'canvas', 3, false],
        ['focus', 'surface', 3, false],
    ]

    it.each(placed)('%s on %s clears its AA ratio in both authored modes', (foreground, background, minimum) => {
        expect(lightTokens[foreground], foreground).toBeTruthy()
        expect(lightTokens[background], background).toBeTruthy()

        expect(contrast(lightTokens[foreground], lightTokens[background])).toBeGreaterThan(minimum)
        expect(contrast(darkTokens[foreground], darkTokens[background])).toBeGreaterThan(minimum)
    })

    it('keeps the fixed QR substrate at its published ratio in both modes', () => {
        const qr = contrast(lightTokens['qr-ink'], lightTokens['qr-surface'])

        expect(designSpine).toContain(qr.toFixed(9))
        expect(darkTokens['qr-ink']).toBe(lightTokens['qr-ink'])
        expect(darkTokens['qr-surface']).toBe(lightTokens['qr-surface'])
    })

    it('publishes every figure it proves, unrounded, in DESIGN.md', () => {
        for (const [foreground, background, , published] of placed) {
            if (!published) continue
            for (const tokens of [lightTokens, darkTokens]) {
                const value = contrast(tokens[foreground], tokens[background]).toFixed(9)
                expect(designSpine, `${foreground}/${background} = ${value}`).toContain(value)
            }
        }
    })

    it('keeps the floor DESIGN.md publishes instead of a row for status on its panel', () => {
        expect(designSpine).toContain('exceed 5.36:1 light and 6.47:1 dark')

        for (const status of ['warning', 'success', 'error']) {
            expect(contrast(lightTokens[status], lightTokens['surface']), status).toBeGreaterThan(5.36)
            expect(contrast(darkTokens[status], darkTokens['surface']), status).toBeGreaterThan(6.47)
        }
    })

    it('keeps the weakest-of-three claim DESIGN.md makes for status rules on canvas', () => {
        // An outcome panel's rule and the preflight's rule both meet canvas.
        // One row for the weakest of the three is a claim about all of them,
        // so the row has to stay the weakest as the palette moves.
        for (const tokens of [lightTokens, darkTokens]) {
            const weakest = Math.min(
                ...['warning', 'success', 'error'].map((status) => contrast(tokens[status], tokens['canvas'])),
            )
            expect(designSpine).toContain(weakest.toFixed(9))
        }
    })

    it('keeps the weakest-adjacent claim DESIGN.md makes for the focus indicator', () => {
        const weakest = Math.min(
            ...['canvas', 'surface', 'elevated'].map((surface) => contrast(lightTokens['focus'], lightTokens[surface])),
        )

        // Quartz's focus token is identical to primary, so the weakest pairing
        // is against canvas -- the lowest-contrast of the three surfaces it
        // sits on -- unlike Terracotta Linen, where elevated was weakest.
        expect(weakest).toBe(contrast(lightTokens['focus'], lightTokens['canvas']))
        expect(designSpine).toContain(weakest.toFixed(9))
    })
})

describe('the decorative edge stays decorative', () => {
    // DESIGN.md requires the functional boundary token on controls, the
    // rest-state drop target, the QR frame, the URL field and the progress
    // track. `--color-separator` deliberately fails 3:1 against every surface
    // -- 1.45 light, 1.75 dark -- so using it on any of these would put an
    // invisible boundary on something that needs a visible one, and it would
    // still pass the contrast proof above, which does not look at that token
    // at all.
    const controls = ['.fd-button', '.fd-drop-zone', '.fd-qr-panel', '.fd-url', '.fd-meter', '.fd-browse-menu']

    it.each(controls)('%s draws its boundary with the functional token, not the decorative one', (selector) => {
        const rule = block(`${selector} {`)

        expect(rule).toContain('var(--color-control-border)')
        expect(rule).not.toContain('var(--color-separator)')
    })

    it('never uses the decorative edge as the sole boundary of a control anywhere in the sheet', () => {
        // A stronger, sheet-wide version of the assertion above: every
        // `border`/`border-*` declaration that names --color-separator sits
        // outside the curated control list, and none of those declarations
        // belongs to a selector this list names.
        for (const selector of controls) {
            expect(componentRules).not.toMatch(
                new RegExp(`${selector.replace('.', '\\.')} \\{[^}]*var\\(--color-separator\\)`),
            )
        }
    })
})

describe('no font ships and none is fetched', () => {
    /*
      Quartz retires Nunito. The platform's own display face carries the
      voice -- SF Pro on macOS, Segoe UI Variable on Windows, system-ui behind
      both -- so there is no local weight to protect and no faux-bold hazard
      to legislate. What is left to assert is the structural guarantee: no
      font file is bundled, and nothing in the stylesheet fetches one.
    */
    it('declares no @font-face and bundles no font file', () => {
        expect(stylesheet).not.toContain('@font-face')
        expect(stylesheet).not.toMatch(/url\(["']?assets\/fonts/)
        expect(existsSync(resolve(projectRoot, 'src/assets/fonts'))).toBe(false)
    })

    it('fetches no font or any other asset over the network', () => {
        expect(stylesheet).not.toMatch(/@import url\(|https?:\/\//)
    })

    it('names only system font stacks, never a bundled family', () => {
        expect(theme).not.toContain('Nunito')
        expect(theme).toContain('--font-display: -apple-system,')
        expect(theme).toContain('--font-body: -apple-system,')
    })
})

describe('rules the components can only reference by name', () => {
    /*
      Each of these is applied by adding a class in a component and asserted
      there only by that class name. jsdom applies no stylesheet, so the rule
      behind the name is invisible to every component test -- emptying any of
      them left all 462 green while the guarantee was gone.
    */

    it('clamps the item name to two lines and hides the overflow', () => {
        const clamp = block('.fd-clamp {')
        expect(clamp).toContain('-webkit-line-clamp: 2;')
        expect(clamp).toContain('line-clamp: 2;')
        expect(clamp).toContain('overflow: hidden;')
    })

    it('keeps the full item name off screen rather than merely invisible', () => {
        // The full value is the aria-describedby target. Dropped from the
        // off-screen rule it renders as visible duplicate text beside the name.
        expect(componentRules).toMatch(/\.fd-visually-hidden[^{]*\{|,\s*\.fd-visually-hidden/)
    })

    it('lets the URL field wrap, because a scrolling one line loses the value', () => {
        const url = block('.fd-url {')
        expect(url).toContain('overflow-wrap: anywhere;')
        expect(url).toContain('resize: none;')
    })

    it('shows an aria-disabled control as inert rather than merely saying so', () => {
        expect(componentRules).toMatch(/\.fd-button\[aria-disabled='true'\] \{[^}]*cursor: default;/)
    })

    /*
      An opacity is a contrast figure nobody published.

      DESIGN.md's text table is "every authored pair the views actually place
      together", and it is checked against the tokens this file declares. A
      fractional opacity composites one of those pairs against whatever is
      behind it at render time, so the pair the user reads is not the pair the
      table publishes and no assertion here can see the difference.

      It was not hypothetical: `.fd-button[aria-disabled='true']` carried
      `opacity: 0.7`, and the quiet Cancel button underneath it is muted on
      elevated -- a pair this file's proof publishes unrounded. No fraction
      below 1 could safely dim it either. Found by the Blind Hunter layer
      re-run (D-109); DESIGN.md had already said it in the palette table:
      "Muted is readable copy, never disabled text."

      A future design that genuinely needs to dim something has to delete this
      test and publish the composited pair, which is the point.
    */
    it('dims nothing with opacity, because a composited pair publishes no figure', () => {
        const opacities = [...stylesheet.matchAll(/^\s*opacity:\s*([^;]+);/gm)].map((match) => match[1].trim())

        expect(opacities.filter((value) => value !== '1'), 'fractional opacity declarations').toEqual([])
    })
})

describe('a cancellation is a status, and never an error', () => {
    /*
      EXPERIENCE.md's rule for `cancelled` is "return to Idle; never render as
      Error", and --color-error is what the Error Panel means in this palette --
      it is used by nothing else. The summary needs to be noticed, so it carries
      the warning token; painting it with the error token would tell the user a
      deliberate action failed.
    */
    it('paints the cancellation summary with the warning token, not the error one', () => {
        const summary = block('.fd-cancel-summary {')

        expect(summary).toContain('var(--color-warning)')
        expect(summary).not.toContain('var(--color-error)')
    })

    it('pairs that colour with a glyph, so colour is never the only cue', () => {
        expect(componentRules).toMatch(/\.fd-cancel-summary__icon \{[^}]*border: 1px solid currentColor;/)
    })
})

describe('declared weight nothing uses', () => {
    it('carries no animation library, since every animation one would serve is banned', () => {
        const manifest = JSON.parse(readFileSync(resolve(projectRoot, 'package.json'), 'utf8')) as {
            dependencies: Record<string, string>
        }

        expect(Object.keys(manifest.dependencies).sort()).toEqual(['react', 'react-dom'])
        expect(JSON.stringify(manifest)).not.toContain('framer-motion')
    })
})
