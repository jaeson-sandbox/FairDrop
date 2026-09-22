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
            // Story 7.8: the accent returns to the logo's mocha, sampled
            // from build/appicon.png rather than invented.
            primary: '#9C5636',
            'primary-hi': '#B06A45',
            // Owner review, Story 7.7 follow-up: the dedicated hover-fill
            // token. Darkens on hover in light mode -- the opposite
            // direction from primary-hi's lighter gradient top stop.
            'primary-hover': '#7F4428',
            'primary-ink': '#FFFFFF',
            // Story 7.8 review follow-up: mocha at 10% on --color-surface,
            // replacing the leftover blue wash. Carries text (the drop
            // zone's heading and meta line, drag-active) -- see the
            // 'primary-tint carries text' describe block below.
            'primary-tint': '#F5EEEB',
            track: '#E9E9EB',
            // Story 7.8 introduced a dedicated violet `focus` role here;
            // Story 7.11 retired it in favour of the ring reading `primary`
            // directly -- see "the focus ring is primary, kept visible by a
            // structural gap, not a separate hue" describe block below.
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
            // Story 7.8: dark half of the mocha accent -- see the light
            // block above.
            primary: '#E39B70',
            'primary-hi': '#EBAA82',
            // Owner review, Story 7.7 follow-up: the dedicated hover-fill
            // token, distinct from primary-hi above. Dark mode lightens on
            // hover, the same direction primary-hi already moves in. Story
            // 7.8 gives it its own mocha value rather than reusing primary-hi.
            'primary-hover': '#F0B694',
            'primary-ink': '#2B1206',
            // Story 7.8 review follow-up: mocha at 12%, not 16%, on
            // --color-surface-dark -- 16% put muted under the 4.5:1 floor
            // on this fill. See the CSS comment beside the real
            // declaration.
            'primary-tint': '#372E2B',
            track: '#3A3A3E',
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
            'primary-hover': 'Highlight',
            'primary-ink': 'HighlightText',
            'primary-tint': 'Canvas',
            track: 'Canvas',
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

    it('restates every box-shadow focus ring as an outline, because Windows High Contrast Mode strips decorative box-shadow (Story 7.11)', () => {
        // Story 7.11 moved every ring from `outline` to `box-shadow`
        // everywhere else in this file. `box-shadow` has no guaranteed
        // survival in forced colors the way `outline` does, so it is
        // restated here in system colors rather than left to inherit the
        // (stripped) authored shadow.
        const ring = block('.fd-button:focus-visible,\n    .fd-url:focus-visible,\n    .fd-button[data-focus-return],\n    .fd-disclosure__summary:focus-visible,\n    .fd-browse-menu .fd-button:focus {')
        expect(ring).toContain('outline: var(--focus-ring-width) solid Highlight;')
        expect(ring).toContain('box-shadow: none;')
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
    it('draws a two-tone ring -- a surface gap, then a primary ring -- for the two Tab-reachable controls (Story 7.11)', () => {
        expect(stylesheet).toMatch(
            /\.fd-button:focus-visible,\s*\.fd-url:focus-visible,\s*\.fd-button\[data-focus-return\] \{\s*/,
        )
        const ring = block('.fd-button:focus-visible,')
        // box-shadow, never outline: a separate outline and box-shadow
        // "fighting each other" (the owner's words) is what Story 7.11
        // replaced with two shadows stacked in one declaration.
        expect(ring).not.toMatch(/outline:/)
        expect(ring).toContain('0 0 0 var(--focus-ring-offset) var(--color-surface)')
        expect(ring).toContain('0 0 0 calc(var(--focus-ring-offset) + var(--focus-ring-width)) var(--color-primary)')
        expect(stylesheet).toContain('--focus-ring-width: 2px;')
        expect(stylesheet).toContain('--focus-ring-offset: 2px;')
    })

    it('rings the browse trigger on its scripted-return marker (Story 7.10), alongside :focus-visible rather than replacing it', () => {
        // `BrowseControl` sets `[data-focus-return]` only on the two returns
        // it makes itself (Escape, or an item chosen by keyboard) and clears
        // it on blur -- see `frontend/src/ui/IdleView.tsx`. This CANNOT
        // become a bare `.fd-button:focus` rule: the trigger sits in the
        // ordinary tab order and a mouse click focuses it too, so a plain
        // `:focus` rule would repaint the ring after every click on it --
        // the stale-ring regression `:focus-visible` exists to prevent.
        const ring = block('.fd-button:focus-visible,')
        expect(ring).toContain('[data-focus-return]')
        expect(ring).not.toMatch(/\.fd-button:focus\b(?!-visible)/)
    })

    it('applies the identical two-tone ring to the disclosure summary, the third Tab-reachable ring in the product (Story 7.11)', () => {
        // "Apply the same treatment everywhere the ring is used... so there
        // is one focus appearance in the product, not two" -- the owner's
        // words. The trigger/.fd-url rule above is one ring; this is the
        // second of the three named explicitly.
        const summaryRing = block('.fd-disclosure__summary:focus-visible {')
        expect(summaryRing).not.toMatch(/outline:/)
        expect(summaryRing).toContain('0 0 0 var(--focus-ring-offset) var(--color-surface)')
        expect(summaryRing).toContain(
            '0 0 0 calc(var(--focus-ring-offset) + var(--focus-ring-width)) var(--color-primary)',
        )
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

describe('the focus ring is primary, kept visible by a structural gap, not a separate hue (Story 7.11)', () => {
    /*
      Story 7.8 protected the ring's visibility with a guard that the two
      colours must never be equal -- a hue-difference mechanism, because the
      ring was an `outline` painted directly against whatever fill sat under
      it, and a same-hue outline on a same-hue fill disappears. The owner
      found the resulting violet "poorly polished," a second hue with no
      relationship to the rest of the product, and asked for the ring to be
      the product's one accent colour instead.

      Story 7.11 removes `--color-focus` entirely -- the ring now reads
      `var(--color-primary)` directly, so there is no second token left to
      collapse onto the first by accident. What still needs protecting is
      the thing the old guard was actually protecting, visibility, not the
      hue-difference mechanism it happened to use. The two-tone ring keeps
      it visible structurally instead: a `--color-surface` gap sits between
      the ring and whatever it surrounds, so a same-hue ring never touches a
      same-hue fill directly. This is a reframing of the Story 7.8 guard,
      not a deletion of it -- the assertion below is what replaces "must not
      equal primary" with "must have a gap namely surface-coloured, strictly
      inside the ring".
    */
    it('has no --color-focus token left in either mode -- the ring is primary itself', () => {
        expect(theme).not.toMatch(/--color-focus:/)
        expect(dark).not.toMatch(/--color-focus:/)
    })

    it('keeps a surface-coloured gap strictly inside the primary ring on every ringed control', () => {
        // Mutation named by the owner: remove the gap (collapse to a single
        // primary-only shadow) -> this must fail, naming the missing gap --
        // the reframed form of Story 7.8's "must not collapse onto primary"
        // guard. A ring with no gap sits flush against a primary fill (the
        // trigger's own background once chosen, the browse menu item's
        // fill) and disappears exactly as the outline-era violet was
        // introduced to prevent.
        for (const selector of ['.fd-button:focus-visible,', '.fd-disclosure__summary:focus-visible {']) {
            const rule = block(selector)
            const shadows = rule.match(/box-shadow:\s*([^;]+);/)?.[1]
            expect(shadows, `${selector} box-shadow`).toBeTruthy()

            const gapOffset = shadows!.match(/0 0 0 (var\(--focus-ring-offset\)) var\(--color-surface\)/)?.[1]
            const ringOffset = shadows!.match(
                /0 0 0 calc\((var\(--focus-ring-offset\)) \+ (var\(--focus-ring-width\))\) var\(--color-primary\)/,
            )
            expect(gapOffset, `${selector} surface gap stop`).toBeTruthy()
            expect(ringOffset, `${selector} primary ring stop, offset by the gap plus its own width`).toBeTruthy()
        }

        // The browse menu item stacks a third shadow (the tint halo) ahead
        // of the same gap/ring pair -- same guarantee, applied after an
        // existing 4px layer rather than from zero.
        const menuItemRing = block('.fd-browse-menu .fd-button:focus {')
        expect(menuItemRing).toContain('0 0 0 calc(4px + var(--focus-ring-offset)) var(--color-surface)')
        expect(menuItemRing).toContain(
            '0 0 0 calc(4px + var(--focus-ring-offset) + var(--focus-ring-width)) var(--color-primary)',
        )
    })

    it('declares the published Story 7.8 mocha values exactly, with no violet left to declare', () => {
        expect(theme).toContain('--color-primary: #9C5636;')
        expect(theme).toContain('--color-primary-hi: #B06A45;')
        expect(theme).toContain('--color-primary-hover: #7F4428;')
        expect(theme).toContain('--color-primary-ink: #FFFFFF;')
        expect(theme).not.toContain('#6B4E9E')

        expect(dark).toContain('--color-primary: #E39B70;')
        expect(dark).toContain('--color-primary-hi: #EBAA82;')
        expect(dark).toContain('--color-primary-hover: #F0B694;')
        expect(dark).toContain('--color-primary-ink: #2B1206;')
        expect(dark).not.toContain('#B79BE0')
    })
})

describe('primary-tint carries text (Story 7.8 review follow-up)', () => {
    it('is mocha, not the leftover blue wash, in both modes', () => {
        // Mutation named in the review follow-up: revert primary-tint to a
        // blue value -> must fail, naming the pair. Checked here directly
        // against the published tokens, and again below via the recomputed
        // contrast pairs that would actually catch a bad fraction, not only
        // a wrong hue.
        expect(theme).toContain('--color-primary-tint: #F5EEEB;')
        expect(theme).not.toContain('--color-primary-tint: #E6EFFB;')

        expect(dark).toContain('--color-primary-tint: #372E2B;')
        expect(dark).not.toContain('--color-primary-tint: #23303F;')
    })

    it('keeps muted readable on the drag-active fill, which is why the dark fraction is 12%, not 16%', () => {
        // Mutation named in the review follow-up: a mocha fraction that
        // puts muted under 4.5:1 -> must fail, naming the pair. #2E2621 is
        // the 16% fraction the review rejected for exactly this reason
        // (muted measured 4.49:1 there); resolved by luminance formula
        // here, not by re-typing the rejected hex, so this test would catch
        // any future fraction that repeats the same mistake, not only this
        // one hex.
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

        const darkMuted = dark.match(/--color-muted:\s*(#[0-9A-Fa-f]{6});/)?.[1]
        const darkTint = dark.match(/--color-primary-tint:\s*(#[0-9A-Fa-f]{6});/)?.[1]
        expect(darkMuted, '--color-muted (dark)').toBeTruthy()
        expect(darkTint, '--color-primary-tint (dark)').toBeTruthy()

        expect(contrast(darkMuted!, darkTint!), 'muted on primary-tint (dark)').toBeGreaterThan(4.5)
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

describe('Story 7.3: rebuilding Idle', () => {
    it('gives the drop zone the xxl radius and sh-2 elevation, with no boundary of its own', () => {
        const zone = block('.fd-drop-zone {')
        expect(zone).toContain('border-radius: var(--radius-xxl);')
        expect(zone).toContain('box-shadow: var(--shadow-sh-2);')
        expect(zone).not.toMatch(/\bborder(-color|-style)?\s*:/)
    })

    it('insets the dashed inner rule 7px so its radius is concentric with the card', () => {
        // DESIGN.md, Shapes: "A 24px card with 7px inset padding takes a
        // 17-18px inner radius." --radius-xxl is 24px and --radius-xl is
        // 18px (both pinned above), so the inner rule has to read the xl
        // token, not a value chosen on its own -- a hand-picked radius here
        // would drift silently the next time --radius-xxl or --radius-xl
        // moved.
        const zone = block('.fd-drop-zone {')
        expect(zone).toContain('padding: 7px;')

        const inner = block('.fd-drop-zone__inner {')
        expect(inner).toContain('border-radius: var(--radius-xl);')
        // The functional token, not the decorative one -- this dashed rule is
        // the only thing that identifies the drop target. See the controls
        // list below, which now includes .fd-drop-zone__inner.
        expect(inner).toContain('border: 2px dashed var(--color-control-border);')
    })

    it('goes solid primary with a tinted fill on drag-active, and lifts the glyph -- never fill alone', () => {
        const active = block('.fd-drop-zone.wails-drop-target-active .fd-drop-zone__inner {')
        expect(active).toContain('border-style: solid;')
        expect(active).toContain('border-color: var(--color-primary);')
        expect(active).toContain('background: var(--color-primary-tint);')

        const lift = block('.fd-drop-zone.wails-drop-target-active .fd-drop-symbol {')
        expect(lift).toContain('transform: translateY(-3px);')
    })

    it('gives the disclosure family the xl radius, sh-1 elevation, a hover fill and a rotating chevron', () => {
        const disclosure = block('.fd-disclosure {')
        expect(disclosure).toContain('border-radius: var(--radius-xl);')
        expect(disclosure).toContain('box-shadow: var(--shadow-sh-1);')

        const summary = block('.fd-disclosure__summary {')
        expect(summary).toContain('list-style: none;')

        expect(stylesheet).toContain('.fd-disclosure__summary:hover {')

        expect(stylesheet).toMatch(
            /\.fd-disclosure\[open\] > \.fd-disclosure__summary \.fd-disclosure__chevron \{\s*transform: rotate\(45deg\);\s*\}/,
        )
    })

    it('keeps the always-open recovery block styled separately from the Idle disclosure form', () => {
        // Both share the fd-help class name -- StagedView's plain <div> and
        // IdleView's <details> -- so the box styling has to be scoped away
        // from the disclosure form, or Idle would paint both a card and a
        // disclosure surface on the same element.
        expect(stylesheet).toContain('.fd-help:not(.fd-disclosure) {')
    })
})

describe('Story 7.7: compose the lifecycle region vertically', () => {
    it('grows the region to fill the available height rather than hugging its content', () => {
        // The mutation named in the acceptance criteria: remove the growth
        // and restore the hugging column -> this must fail.
        const region = block('.fd-region {')
        expect(region).toMatch(/flex:\s*1 1 auto;/)
    })

    it('centres Pending, Transferring and the terminal outcome-as-phase-view, never Idle or Staged', () => {
        // Mutation: top-align any of the three named states -> must fail.
        const centered = block(".fd-region[data-phase-view='pending'],")
        expect(centered).toContain("data-phase-view='transferring'")
        expect(centered).toContain(".fd-app > .fd-outcome[data-phase-view='outcome']")
        expect(centered).toContain('justify-content: center;')

        // Idle and Staged are not named by the centering selector at all --
        // top alignment is the flex default, so their absence here is what
        // keeps them top-aligned. DESIGN.md states the Staged exception
        // explicitly rather than leaving it as CSS silence.
        expect(centered).not.toContain("data-phase-view='idle'")
        expect(centered).not.toContain("data-phase-view='staged'")

        const designSpine = readFileSync(designSpinePath(), 'utf8')
        expect(designSpine).toMatch(/Staged is the one exception, and stays top-aligned/)
    })

    it('excludes a retained outcome from growth or centering, so it keeps its natural height', () => {
        // OutcomePanel.tsx only sets data-phase-view when `phaseView` is true;
        // a retained outcome above Idle renders without it, and a
        // command-failure panel (also OutcomePanel, also phaseView=false)
        // renders *inside* .fd-idle, not as .fd-app's direct child at all --
        // so the growth/centering rule has to key off the base .fd-outcome
        // class doing nothing on its own. Growth belongs only on the two
        // qualified selectors this describe block already pins:
        // .fd-app > .fd-outcome[data-phase-view='outcome'] for the terminal
        // phase view, and nothing for a retained or command-failure panel.
        const base = block('.fd-outcome {')
        expect(base).not.toMatch(/flex:\s*1/)
        expect(base).not.toContain('justify-content: center;')
        expect(stylesheet).not.toMatch(/\.fd-app > \.fd-outcome\s*\{[^}]*flex:\s*1/)
    })

    it('grows the drop zone into Idle\'s slack with a bounded flexible height, never a fixed one', () => {
        // Mutation: remove the growth and restore the hugging column, or give
        // the drop zone a fixed height instead of a bounded flexible one ->
        // both must fail.
        const idle = block('.fd-idle {')
        expect(idle).toMatch(/flex:\s*1 1 auto;/)

        const zone = block('.fd-drop-zone {')
        expect(zone).toMatch(/flex:\s*1 1 auto;/)
        expect(zone).toMatch(/min-block-size:\s*200px;/)
        expect(zone).toMatch(/max-block-size:\s*\d+px;/)
        // A plain, unqualified height/block-size would be the fixed height
        // the acceptance criteria forbid. Only the min-/max- bounded forms
        // may appear.
        expect(zone).not.toMatch(/(?<!min-|max-)\bblock-size:/)
        expect(zone).not.toMatch(/(?<!min-)\bheight:/)

        const inner = block('.fd-drop-zone__inner {')
        expect(inner).toMatch(/flex:\s*1 1 auto;/)
    })

    it('states the vertical-composition rule in DESIGN.md before the stylesheet implements it', () => {
        const designSpine = readFileSync(designSpinePath(), 'utf8')
        expect(designSpine).toMatch(/### Vertical composition \(Story 7\.7\)/)
        expect(designSpine).toMatch(
            /The lifecycle region fills the available window height rather than hugging/,
        )
        expect(designSpine).toMatch(/bounded flexible/)
    })

    it('removes the disclosure summary icon rather than shipping a featureless dot', () => {
        // Either resolution is acceptable per the acceptance criteria; this
        // repo took removal. Mutation: reintroduce the icon markup or its
        // rule -> must fail.
        expect(stylesheet).not.toContain('.fd-disclosure__icon')
    })

    it('narrows the dark primary-hi delta without disturbing any published contrast figure', () => {
        // The original blue-palette narrowing this test pinned (Story 7.7:
        // #6FB0FF -> #5EA6FF) was superseded by Story 7.8's mocha palette;
        // primary-hi-dark is now #EBAA82 per that story's token table. The
        // property this test guards -- that primary-hi never silently drifts
        // out of sync with the published DESIGN.md value -- still holds.
        expect(dark).toContain('--color-primary-hi: #EBAA82;')
        expect(dark).not.toContain('--color-primary-hi: #6FB0FF;')
        expect(dark).not.toContain('--color-primary-hi: #5EA6FF;')

        const designSpine = readFileSync(designSpinePath(), 'utf8')
        expect(designSpine).toContain("primary-hi-dark: '#EBAA82'")
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

    it('distinguishes the focused item by primary fill, a tint halo, and its own two-tone ring -- keyed to :focus (Story 7.10, ring redrawn Story 7.11)', () => {
        const focused = block('.fd-browse-menu .fd-button:focus {')
        expect(focused).toContain('background: var(--color-primary);')
        expect(focused).toMatch(/box-shadow:\s*0 0 0 4px var\(--color-primary-tint\),/)
        // The ring is drawn here too, not only inherited from the shared
        // `.fd-button:focus-visible` rule: that rule never matches these
        // items on WebKit (see the mechanism test below), so it has to live
        // in this rule for macOS to paint one at all. Story 7.11: stacked
        // into the same box-shadow as the halo, not a separate `outline` --
        // gap, then ring, both offset past the halo's own 4px.
        expect(focused).not.toMatch(/outline:/)
        expect(focused).toContain('0 0 0 calc(4px + var(--focus-ring-offset)) var(--color-surface)')
        expect(focused).toContain(
            '0 0 0 calc(4px + var(--focus-ring-offset) + var(--focus-ring-width)) var(--color-primary)',
        )
    })

    it('keys the menu item focus rule to :focus, never :focus-visible, because WebKit never matches :focus-visible on a script-focused element', () => {
        /*
          Mechanism pin for Story 7.10. `BrowseControl`'s menu items carry
          `tabIndex={-1}` (roving tabindex), so they are only ever focused by
          `element.focus()` -- the open effect, and the arrow-key handler --
          never by a real Tab keypress. Confirmed directly against a real
          engine: a `tabindex="-1"` button given `.focus()` from a keydown
          handler matches `:focus-visible` in Chromium but never in WebKit
          (Playwright's bundled build; see
          `_bmad-output/implementation-artifacts/evidence-7-10-make-focus-visible.md`
          for the probe). A `:focus-visible`-keyed rule here is therefore dead
          on macOS specifically: the browse menu opens and Arrow keys move
          between items with **no visible focus indication at all** on the
          shipped app, even though every suite proving Chromium stays green.
          If this assertion starts failing, whoever changed the rule back to
          `:focus-visible` has just reintroduced that macOS defect.
        */
        expect(stylesheet).toContain('.fd-browse-menu .fd-button:focus {')
        expect(stylesheet).not.toContain('.fd-browse-menu .fd-button:focus-visible {')
    })

    it('leaves the shared ring rule scoped to the ordinary controls, not folded in with the menu items', () => {
        // The shared rule still exists for the trigger, the URL field, and
        // every other plain .fd-button -- it is simply no longer what paints
        // the browse menu items' ring (the test above pins that split).
        expect(stylesheet).toMatch(
            /\.fd-button:focus-visible,\s*\.fd-url:focus-visible,\s*\.fd-button\[data-focus-return\] \{/,
        )
    })

    it('carries no separate :hover appearance for menu items (Story 7.11)', () => {
        // Before this story, `:hover` painted a faint, separate
        // `background: var(--color-fill)` while `:focus` painted the full
        // treatment -- two competing ideas of "the item about to be chosen."
        // `BrowseControl` now moves focus to the hovered item itself
        // (`onMouseEnter` in IdleView.tsx), so hover and keyboard focus
        // share the single `:focus` rule above and there is nothing left
        // for a `:hover` rule to paint. Its reappearance would restore the
        // two-active-items defect this story closed.
        expect(stylesheet).not.toContain('.fd-browse-menu .fd-button:hover')
    })

    it('gives the focused menu item the only marked appearance -- the identical rule serves hover and keyboard alike', () => {
        // The behavioural half of "at most one item is ever marked" is
        // proved in IdleView.test.tsx (focus is a single DOM property, and
        // hover moves it rather than adding a second marker). This is the
        // stylesheet half: there is exactly one selector, keyed to :focus,
        // that marks a menu item at all, so whichever item holds focus --
        // for any reason -- gets the identical treatment.
        const focused = block('.fd-browse-menu .fd-button:focus {')
        expect(focused).toContain('background: var(--color-primary);')
        expect(stylesheet).not.toMatch(/\.fd-browse-menu \.fd-button:hover\s*\{/)
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
        // Owner review, Story 7.7 follow-up: the hover fill is its own text
        // background, not a stand-in covered by the resting primary-ink/
        // primary row above. Its absence here is exactly what let
        // .fd-button--primary:hover read --color-primary-hi (the gradient's
        // lighter top stop, wrong direction for light mode) and land at
        // 3.654:1 in light mode, unmeasured, under the 4.5:1 floor.
        ['primary-ink', 'primary-hover', 4.5, true],
        // Story 7.8 review follow-up: primary-tint is a text background too
        // -- the drop zone's drag-active fill, which the heading and meta
        // line render on while a drag is over it. Its absence here is
        // exactly the kind of gap that hid the hover-fill bug in Story 7.7:
        // primary-tint moved from a leftover blue wash to mocha alongside
        // the rest of the accent, unmeasured, until this review pass.
        ['text', 'primary-tint', 4.5, true],
        ['muted', 'primary-tint', 4.5, true],
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
        // Story 7.11: the focus ring is `--color-primary` itself now (no
        // separate `--color-focus` token), so its visibility against every
        // surface it can sit on is exactly the `primary` rows around it --
        // `canvas` and `fill` are the two this table did not already need
        // for another reason (surface/elevated/track/primary-tint above
        // were already load-bearing before the ring moved onto `primary`).
        ['primary', 'canvas', 3, true],
        ['primary', 'fill', 3, true],
        // Story 7.8 review follow-up: the drag-active rule and the
        // solid-primary border sit directly on the primary-tint fill.
        ['primary', 'primary-tint', 3, true],
        // Status rules against the stronger surfaces are published as the
        // weakest-adjacent claim rather than one row each.
        ['warning', 'canvas', 3, false],
        ['success', 'canvas', 3, false],
        ['error', 'canvas', 3, false],
    ]

    it.each(placed)('%s on %s clears its AA ratio in both authored modes', (foreground, background, minimum) => {
        expect(lightTokens[foreground], foreground).toBeTruthy()
        expect(lightTokens[background], background).toBeTruthy()

        expect(contrast(lightTokens[foreground], lightTokens[background])).toBeGreaterThan(minimum)
        expect(contrast(darkTokens[foreground], darkTokens[background])).toBeGreaterThan(minimum)
    })

    it('gives the hover fill its own direction-correct token, distinct from the gradient top stop', () => {
        // Owner review, Story 7.7 follow-up. Resolves whichever --color-*
        // var() the hover rule actually reads, then recomputes its own
        // contrast against primary-ink -- rather than only string-matching
        // the token name -- so a regression is reported as a failed ratio
        // for that pair, not just a text mismatch.
        const hover = block('.fd-button--primary:hover {')
        const hoverVar = hover.match(/background:\s*var\((--color-[a-z-]+)\);/)?.[1]
        expect(hoverVar, 'the var() the hover background reads').toBeTruthy()
        const hoverRole = hoverVar!.replace('--color-', '')

        expect(hover).toContain(`border-color: var(--color-${hoverRole});`)
        expect(lightTokens[hoverRole], hoverRole).toBeTruthy()
        expect(darkTokens[hoverRole], hoverRole).toBeTruthy()

        // Mutation 1 (owner review, the current-at-time-of-review bug):
        // point the hover at --color-primary-hi again -> hoverRole resolves
        // to 'primary-hi' and its light-mode ratio (3.654...) fails this
        // floor by name, not merely by a token-string mismatch.
        expect(
            contrast(lightTokens['primary-ink'], lightTokens[hoverRole]),
            `primary-ink on ${hoverRole} (light)`,
        ).toBeGreaterThan(4.5)
        expect(
            contrast(darkTokens['primary-ink'], darkTokens[hoverRole]),
            `primary-ink on ${hoverRole} (dark)`,
        ).toBeGreaterThan(4.5)

        // The resolved token must actually be the dedicated one, not merely
        // one that happens to clear the floor.
        expect(hoverRole).toBe('primary-hover')

        // Mutation 2: swap the light hover for a value lighter than
        // --color-primary -> caught on luminance direction, not a hex
        // compare, so any lighter replacement is caught, not only the one
        // hex this repo happened to pick. Dark mode is checked the opposite
        // direction: it is supposed to lighten on hover.
        expect(luminance(lightTokens['primary-hover'])).toBeLessThan(luminance(lightTokens['primary']))
        expect(luminance(darkTokens['primary-hover'])).toBeGreaterThan(luminance(darkTokens['primary']))

        // The gradient's own top stop keeps lightening in both modes --
        // unaffected by the hover fix, still the value the single-gradient
        // assertion elsewhere in this file pins.
        expect(luminance(lightTokens['primary-hi'])).toBeGreaterThan(luminance(lightTokens['primary']))
        expect(luminance(darkTokens['primary-hi'])).toBeGreaterThan(luminance(darkTokens['primary']))
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
        // Story 7.11: the ring reads `--color-primary` directly, so the
        // token this claim is about is `primary`, not a separate `focus`
        // role -- there is no longer one to look up.
        const weakest = Math.min(
            ...['canvas', 'surface', 'elevated'].map((surface) => contrast(lightTokens['primary'], lightTokens[surface])),
        )

        // The ring's weakest pairing is against canvas -- the lowest-contrast
        // of the three surfaces it sits on -- the same relationship the
        // violet token had before Story 7.11 retired it, because canvas was
        // already the weakest of the three regardless of which hue sits in
        // the ring.
        expect(weakest).toBe(contrast(lightTokens['primary'], lightTokens['canvas']))
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
    // .fd-drop-zone is deliberately absent (Story 7.3): its outer surface
    // carries no boundary of its own any more, only the sh-2 shadow, and its
    // dashed inner rule (.fd-drop-zone__inner) is checked separately below --
    // .fd-drop-zone__inner is in this list, not exempt from it. The zone
    // carries no click handler and no tab stop, but its dashed rule is the
    // only thing that identifies the drop target: the card is
    // --color-surface on --color-canvas at 1.09:1 in light mode, and the
    // spine forbids shadow from carrying a boundary. DESIGN.md's Colors
    // table requires the functional token on "the rest-state drop target"
    // in as many words. An earlier revision exempted it here and authored
    // the rule with --color-separator at 1.45:1, which left the product's
    // primary affordance with no perceivable edge.
    const controls = [
        '.fd-button',
        '.fd-qr-panel',
        '.fd-url',
        '.fd-meter',
        '.fd-browse-menu',
        '.fd-drop-zone__inner',
    ]

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

    /*
      Story 7.9: the URL field's height comes from a CSS grid + hidden-mirror
      technique (`.fd-url-wrap`/`.fd-url-mirror`), not a JS ResizeObserver.
      `.fd-url-mirror` replicates the URL as text so its wrapped height can
      size the grid cell -- and it must never be exposed as a second,
      duplicate reading of the capability URL. Only `visibility: hidden`
      removes generated/replicated content from the accessibility tree;
      `opacity` and off-screen positioning both leave it readable.
    */
    it('hides the URL field sizing mirror from assistive technology with visibility, not opacity or position', () => {
        const mirror = block('.fd-url-mirror {')
        expect(mirror).toContain('visibility: hidden;')
        expect(mirror).not.toMatch(/opacity:\s*0/)
        expect(mirror).not.toContain('position: absolute')
        expect(mirror).not.toMatch(/left:\s*-\d/)
    })

    /*
      The mirror and the field must share font, padding, border and wrapping
      rules exactly, or the mirror silently mis-sizes the box (Story 7.9
      acceptance criteria). Both are driven from the same design tokens
      declared once in `@theme`, so this checks token names rather than
      resolved literals -- the guarantee the tokens exist to provide.
    */
    it.each([
        'padding: var(--spacing-3);',
        'border: 1px solid var(--color-control-border);',
        'border-radius: var(--radius-md);',
        'font-family: var(--font-code);',
        'font-size: var(--text-code);',
        'font-weight: var(--font-weight-code);',
        'line-height: var(--leading-code);',
        'overflow-wrap: anywhere;',
        'grid-area: 1 / 1;',
    ])('shares %s between the URL field and its sizing mirror', (declaration) => {
        const field = block('.fd-url {')
        const mirror = block('.fd-url-mirror {')
        expect(field, '.fd-url').toContain(declaration)
        expect(mirror, '.fd-url-mirror').toContain(declaration)
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
