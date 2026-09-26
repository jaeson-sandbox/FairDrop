import type {KeyboardEvent, ReactNode} from 'react'
import {useId, useState} from 'react'

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
 * records the removal.
 *
 * Story 9.1: this used to be native `<details>`/`<summary>`, which gave
 * keyboard operability and open/closed state for free but could not smoothly
 * expand or collapse in both engines this product ships to -- neither the
 * `<details>` display swap nor (at the time this was written) the
 * `::details-content` pseudo-element it would take to transition it is
 * available across the WKWebView range `build/darwin/Info.plist` declares.
 * It is now a controlled `<button aria-expanded>` plus a region that expands
 * and collapses via the `grid-template-rows: 0fr <-> 1fr` technique in
 * style.css, which has broader cross-engine support than an animated
 * `<details>` open would. See DESIGN.md's Disclosure row for the fuller
 * trade-off.
 *
 * `<button>` cannot contain a heading in its content model (its content model
 * is phrasing content only), so the heading now *wraps* the button instead
 * of sitting inside it -- the WAI-ARIA APG accordion pattern -- which is why
 * `headingId` lands on the `<h2>` rather than on any child of the button.
 * `screen.getByRole('heading', {name: ...})` still finds it: the accessible
 * name of a heading is computed from its full text content, which descends
 * into the button same as it did into `<summary>` before.
 *
 * A `<button>` is a native Tab stop and answers Enter and Space by itself --
 * no keydown handler is wired here for either, matching what `<summary>`
 * already gave for free and keeping this component's own input handling
 * limited to the one key it must intercept, Escape.
 */
export function Disclosure({className, headingId, summary, children, defaultOpen = false}: DisclosureProps) {
    const [open, setOpen] = useState(defaultOpen)
    const regionId = useId()

    /**
     * Escape clears focus from the disclosure trigger.
     *
     * There is no native `<details>`/`<summary>` Escape behaviour to
     * preserve here -- unlike `BrowseControl`'s menu, there is nothing open
     * to dismiss -- so this is purely "Escape removes the highlighting" (the
     * owner's words) in isolation. See the fuller trade-off comment on
     * `closeAndBlur` in `IdleView.tsx`'s `BrowseControl`, which this mirrors:
     * nothing here re-focuses anything afterward, so the next Tab restarts
     * from the top of the document rather than continuing from this trigger.
     */
    function handleSummaryKeyDown(event: KeyboardEvent<HTMLButtonElement>): void {
        if (event.key !== 'Escape') return
        event.currentTarget.blur()
    }

    return (
        <div className={`${className} fd-disclosure`} data-open={open || undefined}>
            <h2 id={headingId} className="fd-disclosure__heading">
                <button
                    type="button"
                    className="fd-disclosure__summary"
                    aria-expanded={open}
                    aria-controls={regionId}
                    onClick={() => setOpen((was) => !was)}
                    onKeyDown={handleSummaryKeyDown}
                >
                    {summary}
                    <span className="fd-disclosure__chevron" aria-hidden="true"/>
                </button>
            </h2>
            {/*
              Stays in the DOM either way (Story 9.1's motion foundation): a
              transitioned `visibility: hidden` in style.css, not `inert`
              (older macOS WebKit in this product's compatibility range lacks
              it) and not an unmount, is what keeps the collapsed content out
              of the tab order and the accessibility tree while still letting
              `grid-template-rows` animate its height in both engines.
            */}
            <div
                id={regionId}
                className="fd-disclosure__region"
                data-open={open || undefined}
            >
                <div className="fd-disclosure__region-inner">
                    <div className="fd-disclosure__body">
                        {children}
                    </div>
                </div>
            </div>
        </div>
    )
}
