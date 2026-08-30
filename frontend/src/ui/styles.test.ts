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

const stylesheet = readFileSync(resolve(projectRoot, 'src/style.css'), 'utf8')

function block(opening: string): string {
    const start = stylesheet.indexOf(opening)
    expect(start, opening).toBeGreaterThan(-1)
    const end = stylesheet.indexOf('\n}\n', start)
    expect(end, `${opening} close`).toBeGreaterThan(start)
    return stylesheet.slice(start, end)
}

const theme = block('@theme {')
const dark = block('@media (prefers-color-scheme: dark) {')
const componentRules = stylesheet.replace(theme, '').replace(dark, '')

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
        const literals = componentRules.match(/#[0-9A-Fa-f]{3,8}\b/g) ?? []
        expect(literals).toEqual([])
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

    it('never disables forced-color adjustment', () => {
        expect(stylesheet).not.toContain('forced-color-adjust')
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
        const declarations = [...stylesheet.matchAll(/(?:^|[;{\s])((?:min-)?(?:width|inline-size))\s*:\s*([^;}]+)/g)]
        expect(declarations.length).toBeGreaterThan(0)

        for (const [, property, value] of declarations) {
            for (const [, pixels] of value.matchAll(/(\d+)px/g)) {
                expect(Number(pixels), `${property}: ${value.trim()}`).toBeLessThanOrEqual(320)
            }
        }
    })

    it('lets long names and URLs wrap instead of overflowing', () => {
        expect(stylesheet).toMatch(/\.fd-url \{[^}]*overflow-wrap: anywhere;/)
        expect(stylesheet).toMatch(/\.fd-headline \{[^}]*overflow-wrap: anywhere;/)
        expect(stylesheet).toMatch(/\.fd-url \{[^}]*min-inline-size: 0;/)
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
