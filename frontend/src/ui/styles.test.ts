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
            // Owner review, Story 7.7 follow-up: the dedicated hover-fill
            // token. Darkens on hover in light mode. `primary-hi` is gone
            // (black-flash defect fix): it fed only the retired colour-stop
            // gradient, replaced by a translucent-white sheen independent
            // of this token.
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
            // Owner review, Story 7.7 follow-up: the dedicated hover-fill
            // token. Dark mode lightens on hover. `primary-hi` is gone
            // (black-flash defect fix) -- see the light block above.
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

        // The primary button's 1px inset highlight, and its sheen's two
        // translucent-white stops (black-flash defect fix), are the authored
        // literals this spine permits: DESIGN.md scopes them to the single
        // gradient the product ships, and none of the three is a color role
        // a token could carry -- each is a fixed white at a fixed opacity,
        // unrelated to any theme color. The sheen appears twice
        // (byte-identical, rest and :hover -- see "the primary button hover
        // has no black flash" describe block), so both occurrences are
        // stripped.
        const withoutHighlight = componentRules
            .replaceAll('rgb(255 255 255 / 0.4)', '')
            .replaceAll('rgb(255 255 255 / 0.08)', '')
            .replaceAll('rgb(255 255 255 / 0)', '')

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

    it('removes translate and scale from every entrance outright, not only by collapsing their duration (Story 9.1)', () => {
        // Mutation named in Story 9.1's acceptance criteria: delete these two
        // lines, leaving only the duration/delay collapse above -> must fail.
        // A 1ms transition from `translate: 0 10px` to `none` still, for that
        // 1ms, translates; "no element translates or scales on entrance" is a
        // rule about the resting value reached, which only an explicit
        // override -- not a faster trip there -- can guarantee.
        expect(reducedMotion).toContain('translate: none !important;')
        expect(reducedMotion).toContain('scale: none !important;')
    })

    it("does not touch the button press accent, which is a different CSS property (Story 9.1)", () => {
        // `.fd-button:active` presses via `transform: scale(0.975)`, not the
        // standalone `scale` property reset above -- the two compose
        // independently in the Transforms spec, so neutralising `scale`
        // cannot also silently flatten the press feedback DESIGN.md's Motion
        // section still asks for.
        expect(reducedMotion).not.toMatch(/transform:\s*none/)
        const press = block('.fd-button:active {')
        expect(press).toContain('transform: scale(0.975);')
    })
})

describe('Story 9.1: the motion foundation', () => {
    it('enters every phase view with a fade and a 10px rise via @starting-style, with no keyframe and no JavaScript', () => {
        // Mutation 1 (acceptance criteria): delete the @starting-style block
        // -> must fail naming it -- proved by asserting its exact content
        // below, not merely that the string "@starting-style" occurs
        // somewhere in the file.
        // Mutation 2: implement it with a keyframe instead -> must fail --
        // proved by the sheet-wide ban already enforced in "progress
        // presentation" above and re-asserted here for locality.
        const rule = block('[data-phase-view] {')
        expect(rule).toContain('opacity: 1;')
        expect(rule).toContain('translate: none;')
        expect(rule).toMatch(/transition:\s*\n?\s*opacity 340ms var\(--ease-decelerate\),\s*\n?\s*translate 420ms var\(--ease-decelerate\);/)

        const starting = stylesheet.match(
            /@starting-style \{\s*\[data-phase-view\] \{\s*opacity: 0;\s*translate: 0 10px;\s*\}\s*\}/,
        )
        expect(starting, 'the @starting-style block for [data-phase-view]').toBeTruthy()

        expect(stylesheet).not.toContain('@keyframes')
        expect(stylesheet).not.toContain('animation:')
    })

    it('names all five views by data-phase-view, so the entrance rule reaches every one of them', () => {
        // Not a CSS assertion -- a cross-check that the selector above
        // actually has five readers, so a view that quietly stopped setting
        // the attribute would not just silently lose its entrance unnoticed.
        const readers = [
            ['IdleView.tsx', "data-phase-view=\"idle\""],
            ['StagePendingCard.tsx', "data-phase-view=\"pending\""],
            ['StagedView.tsx', "data-phase-view=\"staged\""],
            ['TransferringView.tsx', "data-phase-view=\"transferring\""],
            ['OutcomePanel.tsx', "phaseView ? 'outcome' : undefined"],
        ] as const
        for (const [file, needle] of readers) {
            const source = readFileSync(resolve(projectRoot, 'src/ui', file), 'utf8')
            expect(source, `${file} carries data-phase-view`).toContain(needle)
        }
    })

    it('never gives an exit transition to anything -- views animate in and never out', () => {
        // The spine rule: there is no exit-selector counterpart to
        // [data-phase-view] or .fd-rise anywhere in the sheet -- no
        // .fd-leaving, .fd-exit or [data-phase-view-leaving] class or
        // attribute for a later edit to have quietly wired up. An outgoing
        // view is replaced, not animated off -- keeping one mounted long
        // enough to animate out would give one moment two DOM nodes both
        // claiming to be the current view, breaking the retained-node
        // identity rule App.focus.test.tsx pins.
        for (const exitSelector of ['.fd-leaving', '.fd-exit', '-leaving]', '-exit]']) {
            expect(stylesheet, exitSelector).not.toContain(exitSelector)
        }
    })

    it('provides a capped, reduced-motion-neutral per-child stagger helper, declared exactly once', () => {
        // Two occurrences by design, not one: the live rule and its
        // @starting-style companion -- the same shape every other entrance
        // in this sheet takes ([data-phase-view], .fd-browse-menu). "Exists
        // once in the sheet" (the acceptance criterion) means one *helper*,
        // not a duplicated live rule -- a third occurrence would be that.
        const occurrences = [...stylesheet.matchAll(/\.fd-rise\s*\{/g)]
        expect(occurrences).toHaveLength(2)

        const rise = block('.fd-rise {')
        expect(rise).toMatch(/transition-delay:\s*calc\(min\(var\(--fd-stagger,\s*0\),\s*5\)\s*\*\s*55ms\);/)

        const starting = stylesheet.match(
            /@starting-style \{\s*\.fd-rise \{\s*opacity: 0;\s*translate: 0 8px;\s*\}\s*\}/,
        )
        expect(starting, 'the @starting-style block for .fd-rise').toBeTruthy()

        // Neutralised the same way every other delay in this sheet is: the
        // universal transition-delay: 0ms !important rule under reduced
        // motion overrides this calc() outright. Proved directly rather than
        // by re-deriving the cascade: reduced motion's own describe block
        // above already pins that override exists and applies to every
        // element via the universal *, *::before, *::after selector.
        expect(reducedMotion).toContain('transition-delay: 0ms !important;')
    })

    it('scales and fades the browse menu in from ~0.96 at its own corner, via @starting-style, in <=200ms', () => {
        const menu = block('.fd-browse-menu {')
        expect(menu).toContain('scale: 1;')
        expect(menu).toContain('transform-origin: top left;')
        expect(menu).toMatch(/transition:\s*\n?\s*opacity 160ms var\(--ease-decelerate\),\s*\n?\s*scale 200ms var\(--ease-decelerate\);/)

        const starting = stylesheet.match(
            /@starting-style \{\s*\.fd-browse-menu \{\s*opacity: 0;\s*scale: 0\.96;\s*\}\s*\}/,
        )
        expect(starting, 'the @starting-style block for .fd-browse-menu').toBeTruthy()
    })

    /*
      Story 9.4: DESIGN.md's Motion section names "cards and discs in the
      stories that follow" the browse menu's own fade-plus-scale pattern --
      this is the first of them. Unlike `.fd-rise`'s children, the QR tile is
      not staggered behind the view: it enters at the same time as the view
      itself, which is why this is its own rule rather than another `.fd-rise`
      caller (mutation: giving `.fd-qr-panel` the `fd-rise` class instead
      would still fade it in, but with an 8px translate rather than a scale,
      and staggered a step behind the item row -- exactly the regression this
      test's literal `scale`/`opacity` pair, not merely "it animates in",
      would catch).
    */
    it('scales and fades the Staged QR tile in from ~0.94, via @starting-style, unstaggered (Story 9.4)', () => {
        const panel = block('.fd-qr-panel {')
        expect(panel).toContain('scale: 1;')
        expect(panel).toMatch(/transition:\s*\n?\s*opacity 300ms var\(--ease-decelerate\),\s*\n?\s*scale 420ms var\(--ease-decelerate\);/)

        const starting = stylesheet.match(
            /@starting-style \{\s*\.fd-qr-panel \{\s*opacity: 0;\s*scale: 0\.94;\s*\}\s*\}/,
        )
        expect(starting, 'the @starting-style block for .fd-qr-panel').toBeTruthy()
    })
})

describe('the focus indicator', () => {
    it('draws a two-tone ring -- a surface gap, then a primary ring -- for the two Tab-reachable controls (Story 7.11)', () => {
        expect(stylesheet).toMatch(
            /\.fd-button:focus-visible,\s*\.fd-url:focus-visible,\s*\.fd-button\[data-focus-return\] \{\s*/,
        )
        const ring = block('.fd-button:focus-visible,')
        // box-shadow, never a real outline: a separate outline and
        // box-shadow "fighting each other" (the owner's words) is what
        // Story 7.11 replaced with two shadows stacked in one declaration.
        // `outline: none;` is allowed and expected here -- it is not a
        // second ring, it is what suppresses WebKit's own default one; see
        // the dedicated regression test below.
        expect(ring).not.toMatch(/outline:(?!\s*none\b)/)
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
        // See the comment above the equivalent assertion for the shared
        // ring rule: `outline: none;` is expected, a real outline value is not.
        expect(summaryRing).not.toMatch(/outline:(?!\s*none\b)/)
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

    it('suppresses the UA default outline on every box-shadow ring, so WebKit cannot paint its own blue ring underneath the product ring (regression fix)', () => {
        /*
          Owner-observed regression: pressing Escape closes the browse menu
          correctly, but whatever receives focus afterwards shows a blue
          macOS system focus ring around the product's own mocha two-tone
          ring. Story 7.11 replaced `outline` with `box-shadow` for the ring,
          but `box-shadow` does not replace the user agent's own default
          focus outline the way `outline` used to -- so WebKit keeps drawing
          its blue `outline` underneath the shadow ring, and two rings paint
          at once. The fix is an explicit `outline: none;` alongside the
          box-shadow on every normal-mode ring rule (never in the
          forced-colors block, where a real `outline` is load-bearing).
        */
        for (const selector of ['.fd-button:focus-visible,', '.fd-disclosure__summary:focus-visible {']) {
            const ring = block(selector)
            expect(ring, selector).toContain('outline: none;')
        }
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
        expect(theme).toContain('--color-primary-hover: #7F4428;')
        expect(theme).toContain('--color-primary-ink: #FFFFFF;')
        expect(theme).not.toContain('#6B4E9E')

        expect(dark).toContain('--color-primary: #E39B70;')
        expect(dark).toContain('--color-primary-hover: #F0B694;')
        expect(dark).toContain('--color-primary-ink: #2B1206;')
        expect(dark).not.toContain('#B79BE0')
    })

    it('declares no --color-primary-hi any more -- the black-flash defect fix retired it', () => {
        // primary-hi fed only the primary button's colour-stop gradient. The
        // defect fix replaced that gradient with a translucent-white sheen
        // independent of the fill colour beneath it, so nothing reads this
        // token any more; DESIGN.md's Colors and Elevation & Depth sections
        // were amended to match. A reappearance here would mean either the
        // gradient came back or a declared token has no reader again.
        expect(theme).not.toContain('--color-primary-hi')
        expect(dark).not.toContain('--color-primary-hi')
        expect(forcedColors).not.toContain('--color-primary-hi')
        // componentRules still narrates the removal in prose (the CSS
        // comments above .fd-button--primary), so this checks only that
        // nothing there still *reads* the token through var().
        expect(componentRules).not.toContain('var(--color-primary-hi)')
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
        // Story 9.4: 216px, not 224 -- the QR tile's own acceptance criterion
        // ("~216px", matching the owner-approved prototype's 216x216 `.qr`).
        //
        // Defect fix (orchestrator's rendered 1024x768 review after Story
        // 9.4 merged): the fixed track has to come *first*, matching
        // `StagedView.tsx`'s DOM order (`.fd-qr-panel` renders before
        // `.fd-hero__details`) -- grid assigns tracks to children in DOM
        // order, so a template with the fixed track second handed it to
        // whichever element is second in the DOM, not to "the QR" by name.
        // `browser/accessibility.test.tsx`'s rendered geometry test is what
        // actually proves which element ends up in which track; this only
        // pins the literal template text.
        expect(stylesheet).toMatch(/\.fd-hero \{[^}]*grid-template-columns: 216px minmax\(0, 1fr\);/)
        expect(stylesheet).toContain('@media (max-width: 759px)')
    })

    it('stacks the QR above the item details below 760px', () => {
        const narrow = block('@media (max-width: 759px) {')
        expect(narrow).toContain('.fd-hero')
        expect(narrow).toContain('grid-template-columns: minmax(0, 1fr);')
        expect(narrow).toContain('.fd-qr-panel')
        expect(narrow).toContain('order: -1;')
        // Story 9.4: `.fd-direct-row` no longer holds the field beside the
        // copy action -- it is a wrapping flex row of two buttons now (Copy
        // Link, Show/Hide Link), which needs no width-specific override of
        // its own, so this breakpoint no longer touches it. The dropped
        // `.fd-direct-row` expectation this replaces is exactly that: a
        // premise the new layout no longer has, not a coverage loss -- the
        // field's own reflow is proven separately in
        // `browser/staged-url-field.test.tsx`'s continuous sweep.
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

        // Story 9.1: the summary is a <button> now, not a <summary> --
        // `list-style: none` and the `::-webkit-details-marker` suppression
        // it used to sit beside were <summary>-only UA resets with no reader
        // on a button, and both are gone rather than shipped as dead CSS.
        // `cursor: pointer` is what survives from that assertion.
        //
        // Story 9.4 gives `list-style: none` a new, unrelated reader --
        // `.fd-caveats`, a genuine `<ul>` -- so the blanket whole-stylesheet
        // check this used to be would now fail for a reason that has nothing
        // to do with `<summary>`. Scoped to the summary rule itself instead,
        // which is what the mutation this guards against actually touches.
        const summary = block('.fd-disclosure__summary {')
        expect(summary).toContain('cursor: pointer;')
        expect(summary).not.toContain('list-style: none;')
        expect(stylesheet).not.toContain('::-webkit-details-marker')

        expect(stylesheet).toContain('.fd-disclosure__summary:hover {')

        // Story 9.1: keyed to `[data-open]` on the controlled wrapper, not
        // native `<details>`'s `[open]` attribute -- see Disclosure.tsx and
        // the DOM-structure comment above `.fd-disclosure__region` in
        // style.css for why the heading now sits between `.fd-disclosure`
        // and `.fd-disclosure__summary` in the selector chain.
        expect(stylesheet).toMatch(
            /\.fd-disclosure\[data-open\] > \.fd-disclosure__heading \.fd-disclosure__summary \.fd-disclosure__chevron \{\s*transform: rotate\(45deg\);\s*\}/,
        )
    })

    it("expands and collapses the disclosure region with grid-template-rows, not a fixed height (Story 9.1)", () => {
        // The mutation named in Story 9.1's acceptance criteria: the region
        // has to animate between a collapsed and an expanded state in both
        // Chromium and WebKit with no fixed height to measure and no
        // JavaScript reading one. grid-template-rows interpolating between
        // 0fr and 1fr is what does that; a fixed max-height or a JS-measured
        // scrollHeight would both be a regression to a mechanism this story
        // deliberately avoided.
        const region = block('.fd-disclosure__region {')
        expect(region).toContain('display: grid;')
        expect(region).toContain('grid-template-rows: 0fr;')
        expect(region).toMatch(/transition:\s*\n?\s*grid-template-rows 320ms var\(--ease-decelerate\)/)
        expect(region).not.toMatch(/max-height|max-block-size/)

        const open = block('.fd-disclosure__region[data-open] {')
        expect(open).toContain('grid-template-rows: 1fr;')

        const inner = block('.fd-disclosure__region-inner {')
        expect(inner).toContain('overflow: hidden;')
    })

    it('keeps collapsed disclosure content out of the tab order and the accessibility tree via a transitioned visibility, not inert (Story 9.1)', () => {
        // `inert` is unsupported on the older macOS WebKit this product's
        // compatibility range includes (AGENTS.md), which is why the
        // acceptance criteria name a transitioned `visibility: hidden`
        // specifically. Closed: visible until the collapse transition
        // finishes (a 320ms-delayed hide), so nothing vanishes mid-shrink.
        // Open: visible immediately (a 0-delay show), so content is never
        // hidden while it grows in.
        const closed = block('.fd-disclosure__region {')
        expect(closed).toContain('visibility: hidden;')
        expect(closed).toMatch(/visibility 0s linear 320ms/)

        const open = block('.fd-disclosure__region[data-open] {')
        expect(open).toContain('visibility: visible;')
        expect(open).toMatch(/visibility 0s linear 0s/)
    })

    /*
      Story 9.4: Staged's direct-link field is hidden until Show Link is
      activated, using the identical grid-template-rows/transitioned-
      visibility mechanism proven above for `.fd-disclosure__region` -- see
      `.fd-url-reveal` in style.css and `StagedView.tsx`'s own comment on it.
      It is a separate rule rather than a reuse of the Disclosure component,
      because the trigger is a plain button beside Copy Link, not a
      heading-wrapped disclosure summary -- but the mechanism, and the
      mutation it guards against (a fixed max-height, or an unmount that
      would make Story 7.9's CSS-grid mirror sizing meaningless), is exactly
      the same.
    */
    it('expands and collapses the Staged link-reveal region the same way, with no fixed height (Story 9.4)', () => {
        const region = block('.fd-url-reveal {')
        expect(region).toContain('display: grid;')
        expect(region).toContain('grid-template-rows: 0fr;')
        expect(region).toMatch(/transition:\s*\n?\s*grid-template-rows 320ms var\(--ease-decelerate\)/)
        expect(region).not.toMatch(/max-height|max-block-size/)

        const open = block('.fd-url-reveal[data-open] {')
        expect(open).toContain('grid-template-rows: 1fr;')

        const inner = block('.fd-url-reveal__inner {')
        expect(inner).toContain('overflow: hidden;')
    })

    it('keeps the collapsed link field out of the tab order and the accessibility tree via a transitioned visibility (Story 9.4)', () => {
        const closed = block('.fd-url-reveal {')
        expect(closed).toContain('visibility: hidden;')
        expect(closed).toMatch(/visibility 0s linear 320ms/)

        const open = block('.fd-url-reveal[data-open] {')
        expect(open).toContain('visibility: visible;')
        expect(open).toMatch(/visibility 0s linear 0s/)
    })

    it("gives the open disclosure's body enough top padding to clear the focus ring, expressed as a token, not a magic number", () => {
        // Owner: "these two tabs at the bottom when they have the
        // highlighting it sort of covers the text, so maybe we need to
        // offset them down a bit more as well." Cause: `.fd-disclosure__body`
        // had NO top padding at all, so the open body's first line sat flush
        // against the summary's bottom edge -- exactly where the summary's
        // own two-tone focus ring (`--focus-ring-offset` + `--focus-ring-width`)
        // extends beyond its box.
        //
        // This derives the ring's total reach from the same tokens the ring
        // itself is built from (never a hand-copied "4px"), then requires
        // whatever spacing token the body's top padding uses to exceed it --
        // "exceed", not merely equal, per the owner's screenshot showing the
        // body copy sitting too tight even ignoring the ring.
        // The ring tokens live on :root, not in @theme (see the comment
        // above their declaration in style.css), so they are read from the
        // whole stylesheet rather than the `theme` block like the spacing
        // tokens below.
        const ringOffset = Number(stylesheet.match(/--focus-ring-offset:\s*(\d+)px;/)?.[1])
        const ringWidth = Number(stylesheet.match(/--focus-ring-width:\s*(\d+)px;/)?.[1])
        expect(ringOffset).toBeGreaterThan(0)
        expect(ringWidth).toBeGreaterThan(0)
        const ringExtent = ringOffset + ringWidth

        const body = block('.fd-disclosure__body {')
        const topPaddingToken = body.match(/padding:\s*var\(--spacing-(\d+)\)\s+var\(--spacing-4\)\s+var\(--spacing-4\);/)
        expect(topPaddingToken, 'a spacing token for the body\'s top padding, not a bare pixel value').toBeTruthy()

        const spacingValue = Number(theme.match(new RegExp(`--spacing-${topPaddingToken![1]}:\\s*(\\d+)px;`))?.[1])
        expect(spacingValue, 'the chosen spacing token must resolve to a real value in @theme').toBeGreaterThan(0)
        expect(spacingValue).toBeGreaterThan(ringExtent)
    })

    it('gives the browse trigger chevron the same 12x12 border-chevron mechanism as the disclosure, not a text glyph (defect fix)', () => {
        /*
          Owner-observed defect: the browse trigger's chevron looked tiny and
          thin next to the disclosure chevrons. Cause: the disclosure marker
          is a CSS border chevron (12x12 box, 2px border-right/border-bottom,
          rotated), while the trigger rendered a text glyph (U+2304) styled
          only with `margin-inline-start: auto` -- a text glyph at the
          control's font size renders small and hairline-thin. The fix gives
          the trigger the same border-chevron box, coloured for the mocha
          fill it sits on (--color-primary-ink, not --color-muted, since the
          trigger is a filled primary control, unlike the disclosure summary).
        */
        const chevron = block('.fd-browse-trigger__chevron {')
        expect(chevron).toContain('width: 12px;')
        expect(chevron).toContain('height: 12px;')
        expect(chevron).toContain('border-right: 2px solid var(--color-primary-ink);')
        expect(chevron).toContain('border-bottom: 2px solid var(--color-primary-ink);')
        // Points DOWN, not right. The disclosure's chevron rests at
        // `rotate(-45deg)` (pointing right) and rotates to 45deg when the
        // details expands in place. This control does not expand in place --
        // it opens a menu *below* itself -- so its indicator rests pointing
        // down, the way the text glyph it replaced (U+2304) did and the way
        // every platform popup button does. Owner-observed regression: giving
        // it the disclosure's resting angle made the one control in Idle that
        // *acts* visually indistinguishable from the two informational rows
        // beneath it, which is the distinction Story 7.8 exists to draw.
        expect(chevron).toContain('transform: rotate(45deg);')
        expect(chevron).not.toContain('transform: rotate(-45deg);')
    })

    /*
      Story 9.4 removed Staged's always-open `RecoveryHelp` form -- its
      "Trouble connecting?" content now lives behind the same controlled
      `Disclosure` Idle's "Troubleshooting" row uses, so both consumers of
      the `fd-help` class are `.fd-disclosure` now and the box-styling
      scope this test used to require (`:not(.fd-disclosure)`, to keep an
      always-open card from painting a second surface on the same element)
      no longer has a second form to be scoped away from. This replaces the
      old test with its mirror: the escape hatch is gone, not merely renamed.
    */
    it('no longer needs an escape hatch for an always-open recovery form (Story 9.4 removed it)', () => {
        expect(stylesheet).not.toContain('.fd-help:not(.fd-disclosure)')
    })
})

describe('Story 9.3: declutter Idle and drop the button ellipses', () => {
    it('gives the browse control a pill shape -- {rounded.full}, not the standard control radius', () => {
        const pill = block('.fd-button--pill {')
        expect(pill).toContain('border-radius: var(--radius-full);')
    })

    it('no longer stretches the browse control to the full width of its row (Story 9.3 reversal)', () => {
        // Story 7.3 made this full-width, with a comment naming that as a
        // reversal of Paper Relay's "quieter than the drop zone" rule. Story
        // 9.3 reverses it a second time: the control now sits inside the
        // drop zone as a centred, intrinsic-width pill, so `.fd-selection`
        // no longer stretches its child to 100%. *Mutation:* restore
        // `.fd-selection > .fd-button { width: 100%; }` -> this must fail.
        const selection = block('.fd-selection {')
        expect(selection).not.toContain('width: 100%')
        expect(stylesheet).not.toContain('.fd-selection > .fd-button')
    })

    it('groups both Idle disclosures into one {rounded.xl} surface with a separator between rows', () => {
        const group = block('.fd-idle-disclosures {')
        expect(group).toContain('border-radius: var(--radius-xl);')
        expect(group).toContain('box-shadow: var(--shadow-sh-1);')

        // Each nested disclosure gives up its own card and shadow to the
        // group wrapper -- otherwise Idle would paint a card inside a card.
        const nested = block('.fd-idle-disclosures > .fd-disclosure {')
        expect(nested).toContain('border-radius: 0;')
        expect(nested).toContain('box-shadow: none;')

        // The decorative separator, not the functional control-border token:
        // this rule divides two rows of the same surface, it does not
        // identify anything operable.
        const separator = block('.fd-idle-disclosures > .fd-disclosure + .fd-disclosure {')
        expect(separator).toContain('border-top: 1px solid var(--color-separator);')
    })

    /*
      Defect fix, found by the orchestrator driving the built macOS binary of
      the epic branch: Story 9.1 replaced native <summary> (whose containing
      <h2> carried `flex: 1`, pushing the chevron to the row's trailing edge)
      with a <button> that never got an equivalent rule, so the chevron drifted
      to sit immediately after the label text instead of at the edge, as the
      owner-approved prototype's `.row` shows.

      *Mutation:* remove `justify-content: space-between;` from
      `.fd-disclosure__summary` -> this must fail. The rendered-Chromium proof
      that the chevron's right edge actually lands at the row's trailing edge
      (not merely that this declaration exists in the stylesheet text) lives
      in accessibility.test.tsx, "the disclosure chevron sits at the row's
      trailing edge".
    */
    it("pushes the disclosure chevron to the row's trailing edge with justify-content: space-between", () => {
        const summary = block('.fd-disclosure__summary {')
        expect(summary).toContain('display: flex;')
        expect(summary).toContain('justify-content: space-between;')
    })

    it('gives each browse menu item room for a leading glyph', () => {
        const item = block('.fd-browse-menu .fd-button {')
        expect(item).toContain('justify-content: flex-start;')
        expect(item).toMatch(/gap:\s*var\(--spacing-\d\);/)

        const icon = block('.fd-browse-menu-item__icon {')
        expect(icon).toContain('width: 16px;')
        expect(icon).toContain('height: 16px;')
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

    it('retires primary-hi rather than continuing to narrow it, once the black-flash defect fix gave it no reader', () => {
        // This test used to pin the dark primary-hi delta (Story 7.7:
        // #6FB0FF -> #5EA6FF; Story 7.8: superseded again by the mocha
        // palette's #EBAA82). The black-flash defect fix removed the token
        // outright -- it fed only the primary button's retired colour-stop
        // gradient -- so there is nothing left to narrow. DESIGN.md's
        // frontmatter and Colors/Elevation & Depth sections were amended to
        // drop primary-hi and primary-hi-dark rather than leave a published
        // value for a token the stylesheet no longer declares.
        expect(dark).not.toContain('--color-primary-hi')

        // The frontmatter token list is the maintained source of the
        // declared palette -- prose elsewhere in the document is free to
        // keep narrating primary-hi's retirement (and does, in the Colors
        // and Elevation & Depth sections), but the frontmatter itself must
        // not still list a token the stylesheet no longer declares.
        const designSpine = readFileSync(designSpinePath(), 'utf8')
        const frontmatter = designSpine.slice(0, designSpine.indexOf('\n---\n'))
        expect(frontmatter).not.toContain('primary-hi')
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

    it('replaces the single not-encrypted marker with per-line caveat glyphs (Story 9.4)', () => {
        // The old `.fd-trust p:first-child::before` "!" marked one line via a
        // CSS generated marker, applied to every paragraph until a fix scoped
        // it to the first child only. Story 9.4 replaced that whole mechanism
        // with inline SVG glyphs in StagedView.tsx (an info glyph on the
        // first-opener caveat, a lock glyph on the network one) -- distinct
        // icons a CSS `::before` selector could not express -- so what the
        // stylesheet owns now is sizing and alignment (`.fd-caveats__glyph`),
        // not the glyph choice itself. The old class must be fully gone, not
        // merely superseded, or a future edit could resurrect a mismatched
        // CSS marker alongside the new SVG icons.
        expect(stylesheet).toContain('.fd-caveats {')
        expect(stylesheet).toContain('.fd-caveats__glyph {')
        expect(stylesheet).not.toContain('.fd-trust')
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

        // Black-flash defect fix: the sheen's `background-image` now has to
        // be declared identically at rest AND on :hover (see "the primary
        // button hover has no black flash" describe block below -- if hover
        // painted a different gradient, or none, `background-image` itself
        // would jump instantly between two different images, which is the
        // same class of defect the flash was). Two occurrences, one distinct
        // value: the single-gradient rule is about how many different
        // gradients the product has, not how many times the one of them is
        // written.
        const gradients = [...componentRules.matchAll(/(?<!repeating-)linear-gradient\(/g)]
        expect(gradients).toHaveLength(2)

        const sheen = 'linear-gradient(rgb(255 255 255 / 0.08), rgb(255 255 255 / 0))'
        const primaryButton = block('.fd-button--primary {')
        const primaryHover = block('.fd-button--primary:hover {')
        expect(primaryButton).toContain(`background-image: ${sheen};`)
        expect(primaryHover).toContain(`background-image: ${sheen};`)

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

describe('the QR image cannot be dragged (macOS drag-and-drop crash, defect fix)', () => {
    /*
      Owner report: dragging the QR after staging a file crashed and closed
      the app. Diagnosis from the vendored Wails source (WailsWebView.m,
      performDragOperation:), not from an instrumented reproduction -- the
      crash is a macOS/Cocoa fatality neither suite in this repo can trigger.
      That source reads every NSURL off the drag pasteboard and calls
      `fileSystemRepresentation` on it unconditionally; the QR `<img>`'s
      `data:image/png;base64,...` src is a non-file URL, and WebKit will put
      it on the pasteboard as an NSURL if the image is draggable.

      Two independent guards close this: `draggable={false}` on the element
      (pinned as a rendered assertion in accessibility.test.tsx, where a real
      DOM and computed style exist) and `-webkit-user-drag: none` here.

      This property is text-pinned rather than checked via computed style
      because Chromium (the engine `test:browser` runs) derives
      `-webkit-user-drag` from the `draggable` attribute itself when no CSS
      rule sets it -- confirmed by rendering a plain `<img draggable={false}>`
      with no stylesheet at all and reading `getComputedStyle(...).
      getPropertyValue('-webkit-user-drag')`, which came back `"none"` with
      no `.fd-qr` rule in scope. That makes computed style blind to this
      declaration's removal in the one browser this repo's rendered suite
      uses. A real WKWebView is the platform the diagnosis is about, and
      "belt and braces" is deliberate: the safety this rule buys is real even
      though the rendered suite cannot observe it, which is exactly why the
      literal has to be pinned as text instead.
    */
    it('declares -webkit-user-drag: none on .fd-qr, so WebKit never starts a drag pasteboard for the QR image', () => {
        const qrRule = block('.fd-qr {')
        expect(
            qrRule,
            'expected .fd-qr to declare `-webkit-user-drag: none;` -- without it, dragging the QR ' +
                'image can crash the app (fileSystemRepresentation on a non-file NSURL in ' +
                'WailsWebView.m performDragOperation:)',
        ).toContain('-webkit-user-drag: none;')
    })
})

describe('the primary button hover has no black flash (defect fix)', () => {
    /*
      Observed defect: hovering "Choose a file or folder" (the primary
      button) produced a black flash on the built binary. Mechanism: `.fd-button`
      transitions `background-color` over 150ms. `.fd-button--primary` painted
      its fill with the `background` SHORTHAND
      (`background: linear-gradient(...)`), which -- as a side effect only the
      shorthand has -- resets `background-color` to its initial value,
      `transparent`. `.fd-button--primary:hover` then read
      `background: var(--color-primary-hover)`, the shorthand again, which
      resets `background-image` to `none`. `background-image` cannot
      interpolate (gradients are not transitionable), so it jumps instantly,
      while `background-color` -- reset to `transparent` a moment earlier --
      spends the full 150ms transitioning from `transparent` to the hover
      colour. For that window the button is partly see-through and the dark
      canvas shows through it.

      Fix: read and write `background-color` and `background-image` as
      longhands on this rule pair, so the shorthand can never again silently
      reset the half it does not name, and keep the sheen `background-image`
      byte-identical between rest and hover so only the opaque
      `background-color` animates.
    */
    it('never lets .fd-button--primary or its :hover use the background shorthand', () => {
        const rest = block('.fd-button--primary {')
        const hover = block('.fd-button--primary:hover {')

        // The shorthand form -- a bare `background:` -- is what silently
        // resets the paired longhand property. `background-color:` and
        // `background-image:` are fine; `background:` is not.
        for (const [name, rule] of [['.fd-button--primary', rest], ['.fd-button--primary:hover', hover]] as const) {
            expect(rule, name).not.toMatch(/\bbackground:\s/)
        }
    })

    it('keeps both background-color values opaque and the sheen background-image identical across :hover', () => {
        const rest = block('.fd-button--primary {')
        const hover = block('.fd-button--primary:hover {')

        const restColor = rest.match(/background-color:\s*([^;]+);/)?.[1]
        const hoverColor = hover.match(/background-color:\s*([^;]+);/)?.[1]
        expect(restColor, 'rest background-color').toBeTruthy()
        expect(hoverColor, 'hover background-color').toBeTruthy()

        // Opaque: neither reads `transparent`, and neither is the fully
        // transparent end of the sheen's own alpha ramp -- the actual defect
        // was a `background-color` transitioning FROM `transparent`, so an
        // opaque value in both states is the fix, not a detail of it.
        expect(restColor).not.toMatch(/transparent|\/\s*0\)/)
        expect(hoverColor).not.toMatch(/transparent|\/\s*0\)/)
        expect(restColor).toBe('var(--color-primary)')
        expect(hoverColor).toBe('var(--color-primary-hover)')

        // The sheen has to be the same declaration in both states -- if hover
        // painted its own (or no) background-image, `background-image` itself
        // would still jump instantly between two different images, which is
        // the same class of defect the flash was, just moved to the other
        // longhand.
        const restImage = rest.match(/background-image:\s*([^;]+);/)?.[1]
        expect(restImage, 'rest background-image').toBeTruthy()
        expect(hover).toContain(`background-image: ${restImage};`)
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
        // gap, then ring, both offset past the halo's own 4px. `outline:
        // none;` is expected here too, suppressing WebKit's default ring;
        // see the dedicated regression test below.
        expect(focused).not.toMatch(/outline:(?!\s*none\b)/)
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

    it('suppresses the UA default outline on the focused menu item too, for the same reason as the shared ring rule (regression fix)', () => {
        const focused = block('.fd-browse-menu .fd-button:focus {')
        expect(focused).toContain('outline: none;')
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

describe('the progress ring (Story 9.5, retiring the Story 7.5 progress card and meter)', () => {
    it('keeps the pending card at its own radius, unaffected by the ring rebuild', () => {
        // Unaffected carry-forward from Story 7.5: the stage-pending card
        // was already split from the (now-removed) `.fd-transfer-view` onto
        // its own rule, and this story does not own it either.
        const pending = block('.fd-pending-card {')
        expect(pending).toContain('border-radius: var(--radius-lg);')
        expect(pending).not.toContain('box-shadow')
    })

    it('sizes the ring panel to Staged’s own ~216px slot and reflows it the same way below 759px', () => {
        const panel = block('.fd-ring-panel {')
        expect(panel).toContain('width: min(216px, 100%);')

        const narrow = block('@media (max-width: 759px) {')
        expect(narrow).toContain('.fd-qr-panel,')
        expect(narrow).toContain('.fd-ring-panel {')
    })

    it('enters with the same fade-and-scale-from-0.94 as the QR panel it replaces, unstaggered', () => {
        const panel = block('.fd-ring-panel {')
        expect(panel).toContain('opacity 300ms var(--ease-decelerate)')
        expect(panel).toContain('scale 420ms var(--ease-decelerate)')

        const starting = stylesheet.match(
            /@starting-style \{\s*\.fd-ring-panel \{\s*opacity: 0;\s*scale: 0\.94;\s*\}\s*\}/,
        )
        expect(starting, 'the @starting-style block for .fd-ring-panel').toBeTruthy()
    })

    it('draws its own functional-boundary edge, distinct from the decorative track fill', () => {
        const track = block('.fd-ring__track {')
        expect(track).toContain('stroke: var(--color-track);')

        const edge = block('.fd-ring__edge {')
        expect(edge).toContain('stroke: var(--color-control-border);')
        expect(edge).not.toContain('var(--color-separator)')
    })

    it('fills the ring solid -- the single-gradient rule is absolute and the button already spends it', () => {
        const fill = block('.fd-ring__fill {')
        expect(fill).toContain('stroke: var(--color-primary);')
        expect(fill).not.toContain('gradient')
    })

    it('transitions the determinate fill’s stroke-dashoffset over 400ms, never a keyframe', () => {
        const fill = block('.fd-ring__fill {')
        expect(fill).toContain('transition: stroke-dashoffset 400ms var(--ease-decelerate);')
        expect(fill).not.toContain('@keyframes')
    })

    it('renders the percentage in {typography.numeric} with tabular numerals, centred over the ring', () => {
        const percent = block('.fd-ring__pct {')
        expect(percent).toContain('position: absolute;')
        expect(percent).toContain('inset: 0;')
        expect(percent).toContain('font-size: var(--text-numeric);')
        expect(percent).toContain('font-variant-numeric: tabular-nums;')
    })

    it('gives the ring a fixed, static drawing orientation rather than an animated rotation', () => {
        const ring = block('.fd-ring {')
        expect(ring).toContain('rotate: -90deg;')
        expect(ring).not.toContain('transition:')
        expect(ring).not.toContain('animation:')
    })
})

describe('progress presentation', () => {
    it('keeps the unknown ring static: a dashed stroke, no sweep, shimmer, blink, or rotation of its own', () => {
        expect(stylesheet).toMatch(/\.fd-ring__fill--unknown \{[^}]*stroke-dasharray:/)
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

    it('gives the done and error discs their own tint, at the ~96px Story 9.6 size', () => {
        const icon = block('.fd-outcome__icon {')
        expect(icon).toContain('width: 96px;')
        expect(icon).toContain('height: 96px;')
        expect(icon).toContain('border-radius: var(--radius-full);')

        const done = block('.fd-outcome__icon--done {')
        expect(done).toContain('background: var(--color-success-tint);')
        expect(done).toContain('color: var(--color-success);')

        const error = block('.fd-outcome__icon--error {')
        expect(error).toContain('background: var(--color-error-tint);')
        expect(error).toContain('color: var(--color-error);')
    })

    // Story 9.6: the disc scales in from ~0.7, via @starting-style -- the
    // same progressive-enhancement mechanism every phase view and the QR
    // tile already use. *Mutation:* drop the @starting-style block or the
    // scale figure -> must fail.
    it('scales the disc in from ~0.7, via @starting-style', () => {
        const icon = block('.fd-outcome__icon {')
        expect(icon).toMatch(/transition:\s*opacity 300ms var\(--ease-decelerate\), scale 520ms var\(--ease-decelerate\);/)

        // The @starting-style block immediately following .fd-outcome__icon's
        // own rule -- not the first @starting-style in the file, which
        // belongs to [data-phase-view] further up.
        const iconStart = stylesheet.indexOf('.fd-outcome__icon {')
        const startingStyleOpen = stylesheet.indexOf('@starting-style {\n    .fd-outcome__icon {', iconStart)
        expect(startingStyleOpen, 'a @starting-style block for .fd-outcome__icon').toBeGreaterThan(-1)
        const startingStyleClose = stylesheet.indexOf('\n}\n', startingStyleOpen)
        const startingStyleBlock = stylesheet.slice(startingStyleOpen, startingStyleClose)
        expect(startingStyleBlock).toContain('opacity: 0;')
        expect(startingStyleBlock).toContain('scale: 0.7;')
    })

    it('never wraps the card wider than the Staged card\'s own column (720px)', () => {
        // Story 9.6: one centred card, column width, for every shape it takes
        // -- a live outcome, a retained outcome, or an Idle command failure --
        // so the rule lives on the unqualified selector rather than only the
        // live-phase-view-scoped one. *Mutation:* drop max-width -> must fail.
        const outcome = block('.fd-outcome {')
        expect(outcome).toContain('max-width: 720px;')
        expect(outcome).toContain('margin-inline: auto;')

        // And not duplicated onto the phase-view-scoped selector any more --
        // the base rule alone covers every shape now.
        expect(stylesheet).not.toMatch(/\.fd-app > \.fd-outcome\[data-phase-view='outcome'\]\s*\{[^}]*max-width/)
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

describe('the outcome receipt (Story 9.6 replaces Story 7.4/7.5\'s two-cell grid)', () => {
    it('is one pill-shaped line on {colors.fill}, not a two-cell grid', () => {
        const receipt = block('.fd-outcome__receipt {')
        expect(receipt).toContain('background: var(--color-fill);')
        expect(receipt).toContain('border-radius: var(--radius-full);')
        expect(receipt).toContain('display: inline-flex;')
        // *Mutation:* reintroduce the two-cell grid on the receipt itself ->
        // must fail, naming it. (`.fd-metrics`, Transfer Metrics, is a
        // different component that legitimately keeps its own 2-column grid
        // -- this checks the receipt's own rule, not the whole sheet.)
        expect(receipt).not.toContain('grid-template-columns')
        expect(stylesheet).not.toMatch(/\.fd-receipt\b/)
    })

    it('carries no duration cell and no third column', () => {
        // The mutation this guards against is the worst possible outcome of
        // this story: inventing a displayed duration. Nothing in the sheet
        // may name a duration/elapsed rule anywhere near the receipt.
        expect(stylesheet).not.toMatch(/\.fd-outcome__receipt.*--(duration|elapsed|time)/)
        expect(stylesheet).not.toContain('grid-template-columns: 1fr 1fr 1fr')
    })

    it('truncates a long name with an ellipsis rather than wrapping or overflowing the pill', () => {
        // *Mutation:* drop text-overflow/white-space -> must fail.
        const name = block('.fd-outcome__receipt-name {')
        expect(name).toContain('overflow: hidden;')
        expect(name).toContain('text-overflow: ellipsis;')
        expect(name).toContain('white-space: nowrap;')
    })

    it('renders the wire-bytes figure with tabular numerals, like the percentage', () => {
        const meta = block('.fd-outcome__receipt-meta {')
        expect(meta).toContain('font-variant-numeric: tabular-nums;')
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
      because it drops the surface fill every other control keeps. (Story 9.4
      removed the one bare `warning`-on-`elevated` placement, the old
      `.fd-trust` marker, along with its row in the table below -- see the
      comment beside that removal.)
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
        // Story 9.4 removed the last bare `warning`-on-`elevated` placement
        // (the old `.fd-trust p:first-child::before` marker, which had no
        // background of its own and sat directly on `.fd-packet`'s
        // elevated fill): both remaining warning surfaces --
        // `.fd-warning-banner` and `.fd-cancel-summary` -- paint their own
        // `--color-surface` fill first, which is the `warning`-on-`surface`
        // row above. Removed rather than left in place unplaced ("Placed
        // together" is this table's own stated rule, further up this file).
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
        //
        // Reads `background-color:` specifically, not `background:` -- the
        // black-flash defect fix moved this rule to the longhand on purpose
        // (a bare `background:` shorthand here would reset the sheen
        // `background-image` to `none`, which is the defect this fix
        // closed), and "the primary button hover has no black flash"
        // describe block below is what pins the shorthand's absence.
        const hover = block('.fd-button--primary:hover {')
        const hoverVar = hover.match(/background-color:\s*var\((--color-[a-z-]+)\);/)?.[1]
        expect(hoverVar, 'the var() the hover background reads').toBeTruthy()
        const hoverRole = hoverVar!.replace('--color-', '')

        expect(hover).toContain(`border-color: var(--color-${hoverRole});`)
        expect(lightTokens[hoverRole], hoverRole).toBeTruthy()
        expect(darkTokens[hoverRole], hoverRole).toBeTruthy()

        // Mutation 1 (owner review, the current-at-time-of-review bug):
        // point the hover back at the gradient's top stop -> before the
        // black-flash defect fix retired that token, hoverRole would
        // resolve to 'primary-hi' and its light-mode ratio (3.654...) would
        // fail this floor by name. Now that the token is gone entirely,
        // lightTokens[hoverRole] resolves to undefined and the `toBeTruthy`
        // check two lines up fails first -- a strictly earlier catch of the
        // same mutation.
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

        // The gradient no longer has a colour-stop top value to check here:
        // the black-flash defect fix replaced the primary-hi -> primary
        // colour-stop gradient with a translucent-white sheen independent of
        // the fill colour, so `primary-hi` was retired (see "declares no
        // --color-primary-hi any more" above) rather than re-checked.
    })

    it('measures the sheen as a text background -- the label sits at the top of the button, where it is strongest', () => {
        // Third time this file has found a surface text sits on that was
        // never measured: the hover fill (Story 7.7), --color-primary-tint
        // (Story 7.8), and now the primary button's sheen. The sheen is a
        // translucent white overlay independent of the fill colour beneath
        // it, painted over --color-primary-ink text, so composited it is a
        // text background like any other and belongs in this proof.
        //
        // Re-derived from the stylesheet, not hand-copied: the alpha comes
        // from the actual `.fd-button--primary` declaration, so a future
        // edit to the sheen is measured here rather than assumed.
        const primaryButton = block('.fd-button--primary {')
        const sheenAlpha = Number(
            primaryButton.match(/background-image:\s*linear-gradient\(rgb\(255 255 255 \/ ([\d.]+)\)/)?.[1],
        )
        expect(sheenAlpha, 'sheen top-stop alpha').toBeGreaterThan(0)
        // Not a taste call -- DESIGN.md derives this exact figure and the
        // rule not to raise it from the ratios this test proves below.
        expect(sheenAlpha).toBe(0.08)

        // Composites a translucent white top stop over an opaque background,
        // rounding each channel the way a real compositor renders pixels --
        // matching, not merely approximating, what the browser paints.
        function composite(alpha: number, backgroundHex: string): string {
            const bg = [0, 2, 4].map((i) => Number.parseInt(backgroundHex.slice(1).slice(i, i + 2), 16))
            const blended = bg.map((channelValue) => Math.round(alpha * 255 + (1 - alpha) * channelValue))
            return `#${blended.map((c) => c.toString(16).padStart(2, '0')).join('')}`
        }

        // The label sits at the top of the button in every state the sheen
        // paints, so all four combinations -- rest/hover x light/dark -- are
        // real text-on-background pairs. Light rest is the worst case: the
        // darkest of the four fills, so its composite sits closest to the
        // 4.5:1 floor.
        const cases: Array<[string, string, string]> = [
            ['light rest', lightTokens['primary-ink'], lightTokens['primary']],
            ['light hover', lightTokens['primary-ink'], lightTokens['primary-hover']],
            ['dark rest', darkTokens['primary-ink'], darkTokens['primary']],
            ['dark hover', darkTokens['primary-ink'], darkTokens['primary-hover']],
        ]

        const ratios = cases.map(([name, ink, fill]) => {
            const ratio = contrast(ink, composite(sheenAlpha, fill))
            expect(ratio, name).toBeGreaterThan(4.5)
            return [name, ratio] as const
        })

        const worst = ratios.reduce((min, entry) => (entry[1] < min[1] ? entry : min))
        expect(worst[0], 'the worst case is light rest, as DESIGN.md documents').toBe('light rest')

        // Published, unrounded, as DESIGN.md requires of every figure this
        // file proves.
        expect(designSpine, `sheen top edge on primary (light rest) = ${worst[1].toFixed(9)}`)
            .toContain(worst[1].toFixed(9))

        // Mutation the task's own derivation table names: raising the alpha
        // towards 0.12 fails this floor -- which is why 0.08 is chosen and
        // pinned above, not a value someone could quietly nudge upward.
        const raised = contrast(lightTokens['primary-ink'], composite(0.12, lightTokens['primary']))
        expect(raised, 'sheen top edge on primary (light rest) at alpha 0.12').toBeLessThan(4.5)
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
        // Story 9.5 retired `.fd-meter` (the linear progress track) for
        // `.fd-ring__edge` -- the ring's own functional-boundary stroke, the
        // same role this list already checked on the old element.
        '.fd-ring__edge',
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

    /*
      Story 9.4 removed the two-line clamp this test used to pin
      (`.fd-clamp`, `-webkit-line-clamp: 2`) along with the "Show full name"
      toggle it required: the item name always wraps now, so there is no
      clamp rule left for a component to reference by name. Replaced with an
      equivalent case from the same story: `.fd-caveats__glyph` sizes the
      info/lock glyphs `StagedView.tsx` references only by class, which jsdom
      cannot verify any other way.
    */
    it("sizes the caveat glyph via its own class, since jsdom applies no stylesheet (Story 9.4)", () => {
        const glyph = block('.fd-caveats__glyph {')
        expect(glyph).toContain('width: 14px;')
        expect(glyph).toContain('height: 14px;')
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

        // Story 9.1: `0` joins the allowed set alongside `1`, and only those
        // two -- an entrance's `opacity: 0` in @starting-style (and its
        // resting `opacity: 1` counterpart) is binary presence/absence, not
        // the fractional composite this test exists to forbid. Nothing ever
        // *renders* at `opacity: 0`; it is the pre-paint state a view
        // transitions away from before a user reads anything, never a
        // dimmed pair sitting on screen the way `aria-disabled`'s old
        // `opacity: 0.7` did. Any value strictly between 0 and 1 remains
        // exactly as forbidden as before.
        const fractional = opacities.filter((value) => value !== '1' && value !== '0')
        expect(fractional, 'fractional opacity declarations').toEqual([])
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
