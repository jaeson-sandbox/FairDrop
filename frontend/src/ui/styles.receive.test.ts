import {readFileSync} from 'node:fs'
import {resolve} from 'node:path'
import {describe, expect, it} from 'vitest'

/*
  Stylesheet pins for the receive views (spec-phone-to-desktop-receiving,
  Story B). jsdom lays nothing out, so the rendered proof is in
  browser/receive.test.tsx; what is pinned here is the *text* that makes each
  rendered guarantee possible, so a deleted rule fails in milliseconds and by
  name before the browser suite even launches. Every selector and value is a
  literal written at the assertion site.
*/

const stylesheet = readFileSync(resolve(process.cwd(), 'src/style.css'), 'utf8').replace(/\r\n/g, '\n')

/** The declarations of the first rule whose selector list is exactly `selector`. */
function declarationsOf(selector: string): string {
    const start = stylesheet.indexOf(`\n${selector} {`)
    expect(start, `a rule for ${selector}`).toBeGreaterThan(-1)
    const end = stylesheet.indexOf('\n}', start)
    return stylesheet.slice(start, end)
}

describe('the Receive entry row', () => {
    const rule = () => declarationsOf('.fd-receive-entry')

    it('wraps, so at 320px the hint sits above the button instead of scrolling sideways', () => {
        expect(rule()).toContain('flex-wrap: wrap;')
    })

    it('declares no fixed width or height of its own', () => {
        expect(rule()).not.toMatch(/(^|\s)(width|height|min-width|min-height|max-width|block-size|inline-size):/)
    })

    it('centres its contents, matching the rest of Idle', () => {
        expect(rule()).toContain('justify-content: center;')
        expect(rule()).toContain('text-align: center;')
    })
})

describe('the receive views join the phase views that centre vertically', () => {
    it('lists the waiting and receiving regions beside staged, pending and transferring', () => {
        const start = stylesheet.indexOf(".fd-region[data-phase-view='pending'],")
        expect(start).toBeGreaterThan(-1)
        const end = stylesheet.indexOf('\n}', start)
        const selectors = stylesheet.slice(start, end)

        expect(selectors).toContain(".fd-region[data-phase-view='receive-waiting']")
        expect(selectors).toContain(".fd-region[data-phase-view='receive-receiving']")
        expect(selectors).toContain('justify-content: center;')
    })

    it('centres the outcome card by the same auto margin every outcome card uses', () => {
        // The receive outcome is a direct child of .fd-app, so the existing rule is what centres it.
        const start = stylesheet.indexOf('.fd-app > .fd-outcome,')
        expect(start).toBeGreaterThan(-1)
        expect(stylesheet.slice(start, stylesheet.indexOf('\n}', start))).toContain('margin-block: auto;')
    })
})

describe('the focus ring the Receive control returns to', () => {
    it('keys the ring to the marker as well as :focus-visible, because WebKit never matches :focus-visible after a scripted focus', () => {
        expect(stylesheet).toMatch(/\.fd-button:focus-visible,\s*\.fd-url:focus-visible,\s*\.fd-button\[data-focus-return\] \{/)
    })

    it('restores a real outline for the marker under forced colors, which strip box-shadow', () => {
        const forced = stylesheet.slice(stylesheet.indexOf('@media (forced-colors: active) {'))
        const index = forced.indexOf('.fd-button[data-focus-return]')
        expect(index).toBeGreaterThan(-1)
        const body = forced.slice(index, forced.indexOf('}', index))
        expect(body).toContain('outline: var(--focus-ring-width) solid Highlight;')
        expect(body).toContain('box-shadow: none;')
    })
})

describe('the receive QR cannot be dragged', () => {
    it('keeps -webkit-user-drag: none on the shared QR image rule the receive view reuses', () => {
        expect(declarationsOf('.fd-qr')).toContain('-webkit-user-drag: none;')
    })
})
