import {readFileSync} from 'node:fs'
import {resolve} from 'node:path'
import {describe, expect, it} from 'vitest'

/*
  jsdom performs no layout, so the large-list guarantees are proved against the
  stylesheet text. This file reads only src/style.css (not the design spine) so
  the isolated collection mutation runner can execute it from a frontend-only copy.
  Every value is a literal written at the assertion site.
*/
const stylesheet = readFileSync(resolve(process.cwd(), 'src/style.css'), 'utf8').replace(/\r\n/g, '\n')

describe('collection draft list at large counts', () => {
    // The list can hold up to 10,000 rows, so it must scroll inside the window
    // rather than push Send and Cancel out of reach, and its focus ring must
    // never be clipped by the scrollport.
    const rule = /\.fd-draft__list \{([^}]*)\}/.exec(stylesheet)?.[1] ?? ''

    it('scrolls within the window instead of stretching the page', () => {
        expect(rule).toContain('max-height: min(24rem, 50vh)')
        expect(rule).toContain('overflow-y: auto')
        expect(rule).toContain('overscroll-behavior: contain')
    })

    it('leaves room inside the scrollport for the 2px outline and its 3px offset', () => {
        expect(rule).toContain('padding-inline-end: .75rem')
        expect(rule).toContain('padding-block: .25rem')
        expect(stylesheet).toContain('.fd-draft .fd-button:focus, .fd-multiple-outcome:focus { outline: 2px solid var(--color-primary); outline-offset: 3px; }')
    })

    it('sizes the ordinal column to the widest ordinal, not a fixed three characters', () => {
        const ordinal = /\.fd-draft__ordinal \{([^}]*)\}/.exec(stylesheet)?.[1] ?? ''
        expect(ordinal).toContain('flex: 0 0 calc((var(--fd-ordinal-digits, 2) + 1) * 1ch)')
        expect(ordinal).not.toContain('3ch')
        expect(ordinal).toContain('font-variant-numeric: tabular-nums')
    })

    it('keeps a system-colour border and focus outline in forced colors', () => {
        expect(stylesheet).toContain('.fd-draft__row, .fd-draft__list { border-color: CanvasText; }')
        expect(stylesheet).toContain('.fd-draft .fd-button:focus, .fd-multiple-outcome:focus { outline: 2px solid Highlight; }')
    })
})
