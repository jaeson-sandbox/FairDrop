import {existsSync, readFileSync} from 'node:fs'
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

describe('the Terracotta Linen token layer', () => {
    it('declares every authored light value as a Tailwind v4 theme variable', () => {
        const light: Record<string, string> = {
            canvas: '#F7F0E7',
            surface: '#FFFAF4',
            elevated: '#EFE2D4',
            text: '#2C2723',
            muted: '#70645B',
            border: '#CDBDAE',
            'control-border': '#8B7462',
            primary: '#A94724',
            'primary-ink': '#FFFFFF',
            hover: '#873719',
            drop: '#F1D0BD',
            progress: '#B64B23',
            focus: '#7E4B92',
            success: '#2F7658',
            warning: '#946000',
            error: '#AB3932',
            'qr-surface': '#FFFFFF',
            'qr-ink': '#221F1C',
        }

        for (const [role, value] of Object.entries(light)) {
            expect(theme, role).toContain(`--color-${role}: ${value};`)
        }
    })

    it('declares the authored dark half as exact values rather than an inversion', () => {
        const darkPair: Record<string, string> = {
            canvas: '#1C1916',
            surface: '#25211D',
            elevated: '#312A24',
            text: '#F7EFE5',
            muted: '#BCAF9F',
            border: '#55493F',
            'control-border': '#89796A',
            primary: '#FF986D',
            'primary-ink': '#2B1309',
            hover: '#FFB18F',
            drop: '#4A2D23',
            progress: '#FF8858',
            focus: '#C5A1D3',
            success: '#79D5AA',
            warning: '#F2BD62',
            error: '#FF8B83',
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
            '--text-display: 24px;',
            '--text-headline: 20px;',
            '--text-body: 14px;',
            '--text-label: 12px;',
            '--text-code: 12px;',
            '--text-control: 13px;',
            '--radius-xs: 3px;',
            '--radius-sm: 5px;',
            '--radius-md: 8px;',
            '--radius-lg: 10px;',
            '--radius-full: 9999px;',
            '--spacing-window-gutter: 20px;',
            '--spacing-target-min: 44px;',
            '--shadow-paper: 3px 3px 0 #CDBDAE;',
        ]) {
            expect(theme, declaration).toContain(declaration)
        }
    })

    it('loads no web font, because the spine allows system-safe stacks only', () => {
        expect(stylesheet).not.toContain('@font-face')
        expect(stylesheet).not.toContain('Nunito')
        expect(theme).toContain('--font-body: system-ui,')
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

        expect(componentRules.match(colorLiteral) ?? []).toEqual([])
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

    it('supersedes Terracotta Linen with system colors on every authored token', () => {
        const systemColors: Record<string, string> = {
            canvas: 'Canvas',
            surface: 'Canvas',
            elevated: 'Canvas',
            text: 'CanvasText',
            muted: 'CanvasText',
            border: 'CanvasText',
            'control-border': 'CanvasText',
            primary: 'Highlight',
            'primary-ink': 'HighlightText',
            hover: 'Highlight',
            drop: 'Canvas',
            progress: 'Highlight',
            focus: 'Highlight',
            success: 'CanvasText',
            warning: 'CanvasText',
            error: 'CanvasText',
        }

        for (const [role, value] of Object.entries(systemColors)) {
            expect(forcedColors, role).toContain(`--color-${role}: ${value};`)
        }

        // Every authored role is covered: a token added to @theme without a
        // forced-colors answer keeps its Terracotta value in the system palette.
        const authored = [...theme.matchAll(/--color-([a-z-]+):/g)].map(([, role]) => role)
        const uncovered = authored.filter((role) => !(role in systemColors) && !role.startsWith('qr-'))
        expect(uncovered).toEqual([])
    })

    it('keeps the progress fill distinguishable from its own track', () => {
        // Highlight on Canvas. Left to the user agent both would become Canvas
        // and a determinate meter would read as empty at every percentage.
        expect(forcedColors).toContain('--color-progress: Highlight;')
        expect(forcedColors).toContain('--color-drop: Canvas;')
    })

    it('drops the decorative paper offset, which has no system color', () => {
        expect(forcedColors).toContain('--shadow-paper: none;')
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
    it('draws one ring from the focus token for controls and routed targets alike', () => {
        expect(stylesheet).toMatch(
            /\.fd-button:focus-visible,\s*\.fd-url:focus-visible,\s*\[data-focus-target\]:focus \{\s*/,
        )
        expect(stylesheet).toContain('outline: var(--focus-ring-width) solid var(--color-focus);')
        expect(stylesheet).toContain('outline-offset: var(--focus-ring-offset);')
        expect(stylesheet).toContain('--focus-ring-width: 3px;')
    })

    it('uses plain :focus for the nodes only the routing table focuses', () => {
        // A programmatic focus move is not reliably :focus-visible across
        // engines, and a focus move with no visible ring is one the user cannot
        // follow.
        expect(stylesheet).toContain('[data-focus-target]:focus {')
        expect(stylesheet).not.toContain('[data-focus-target]:focus-visible')
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

    it('collapses the remaining pairs into one column below 640px', () => {
        const narrowest = block('@media (max-width: 639px) {')
        expect(narrowest).toContain('.fd-selection')
        expect(narrowest).toContain('.fd-metrics')
        expect(narrowest).toContain('grid-template-columns: minmax(0, 1fr);')
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

    it('spends the one sanctioned paper offset on the packet and nowhere else', () => {
        // DESIGN.md allows one decorative offset edge, names 3px 3px 0, scopes
        // it to StagedView, and says every other surface stays flat.
        const shadows = [...componentRules.matchAll(/box-shadow:\s*([^;]+);/g)].map(([, value]) => value.trim())

        expect(shadows).toEqual(['var(--shadow-paper)'])
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

describe('progress presentation', () => {
    it('keeps the unknown pattern static: no sweep, shimmer, or blink', () => {
        expect(stylesheet).toMatch(/\.fd-meter--unknown \{[^}]*repeating-linear-gradient\(/)
        expect(stylesheet).not.toContain('@keyframes')
        expect(stylesheet).not.toContain('animation:')
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
        resolve(repositoryRoot, '_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-2026-08-23/DESIGN.md'),
        'utf8',
    )

    /** [foreground, background, minimum ratio, published as an exact figure]. */
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
        ['progress', 'drop', 3, true],
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
        expect(designSpine).toContain('exceed 5.14:1 light and 7.05:1 dark')

        for (const status of ['warning', 'success', 'error']) {
            expect(contrast(lightTokens[status], lightTokens['surface']), status).toBeGreaterThan(5.14)
            expect(contrast(darkTokens[status], darkTokens['surface']), status).toBeGreaterThan(7.05)
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

        expect(weakest).toBe(contrast(lightTokens['focus'], lightTokens['elevated']))
        expect(designSpine).toContain(weakest.toFixed(9))
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

    it('bundles no font file, because the spine allows system-safe stacks only', () => {
        expect(existsSync(resolve(projectRoot, 'src/assets/fonts'))).toBe(false)
    })
})
