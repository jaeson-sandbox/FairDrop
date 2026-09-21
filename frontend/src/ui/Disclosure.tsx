import type {ReactNode} from 'react'

interface DisclosureProps {
    /**
     * The class the caller's selector already keys off (`fd-preflight`,
     * `fd-help`). It goes first in the class list so a test that reads
     * `element.className.split(' ')[0]` still finds it -- see
     * `IdleView.test.tsx`'s document-order assertion.
     */
    readonly className: string
    readonly headingId: string
    readonly summary: string
    readonly children: ReactNode
    /**
     * DESIGN.md's stated fallback for the FR23 amendment: "If acceptance
     * decides FR23 means *visible* rather than *present and preceding*, the
     * disclosure ships `open` by default and the rest of the treatment is
     * unaffected." One flag, flipped at the call site -- nothing else here
     * has to change to restore the fully-expanded reading.
     */
    readonly defaultOpen?: boolean
}

/**
 * `{rounded.xl}` surface at `{elevation.sh-1}` with a chevron that rotates
 * when open and a hover fill -- DESIGN.md's Disclosure row. No leading icon:
 * Story 7.7 removed the tinted dot that used to sit before the heading after
 * finding it resolved, at its rendered size, to a featureless coloured circle
 * that read as a bullet rather than a glyph -- DESIGN.md's Components table
 * records the removal. Built on native `<details>`/`<summary>` rather than a
 * button-plus-div
 * pair: the browser already gives that element pair keyboard operability
 * (Enter and Space toggle a focused summary, and it is a native Tab stop) and
 * an implicit accessible name/state, so there is no ARIA to hand-roll and no
 * open/closed state to keep in sync with the DOM the way `BrowseControl`
 * must.
 *
 * The heading lives inside `<summary>` as its first child, which HTML's
 * content model for `summary` permits and which is what lets
 * `screen.getByRole('heading', {name: ...})` keep finding it: collapsing a
 * `<details>` is a rendering-only effect with no default UA stylesheet in
 * jsdom, so the heading and every string in `children` stay in the
 * accessibility tree for a test exactly as before, even while collapsed in a
 * real browser.
 */
export function Disclosure({className, headingId, summary, children, defaultOpen = false}: DisclosureProps) {
    return (
        <details className={`${className} fd-disclosure`} open={defaultOpen || undefined}>
            <summary className="fd-disclosure__summary">
                <h2 id={headingId} className="fd-disclosure__heading">{summary}</h2>
                <span className="fd-disclosure__chevron" aria-hidden="true"/>
            </summary>
            <div className="fd-disclosure__body">
                {children}
            </div>
        </details>
    )
}
