import type {FocusEvent, KeyboardEvent, MouseEvent} from 'react'
import {useEffect, useId, useRef, useState} from 'react'
import {copy} from './copy'

interface BrowseControlProps {
    /**
     * The trigger's own visible and accessible label.
     *
     * Extracted in Story 9.3 (sprint-status action item `epic-4-retro-item-28`:
     * "extract BrowseControl from IdleView.tsx when a second floating surface
     * appears" -- Story 9.6's outcome card is that surface) so the same
     * component can open the same menu under two different labels: Idle's
     * `copy.label.chooseFileOrFolder` and, later, an outcome card's
     * `copy.done.sendAnother` or `copy.outcome.chooseAnother`. The menu items
     * themselves (File/Folder) never vary by caller, so they stay read from
     * the registry here rather than becoming props too.
     */
    readonly label: string
    readonly onSelectFile: () => void
    readonly onSelectDirectory: () => void
}

/**
 * The one control that replaced the two browse buttons.
 *
 * Windows' `IFileOpenDialog` cannot offer a file and a folder chooser in one
 * dialog the way macOS's `NSOpenPanel` can, so the label alone cannot promise
 * both kinds without a click sometimes breaking that promise -- exactly
 * `IdleView`'s old scar above. The menu is where that asymmetry is absorbed:
 * the control's label never changes, and either item leads to the matching
 * native chooser.
 *
 * This is the product's first floating surface, so it earns the ARIA menu
 * button pattern rather than a plain toggle: `role="menu"`/`role="menuitem"`,
 * Escape, and focus return to the trigger. Its open/closed state is local --
 * not reducer state -- because nothing about it survives a re-render of Idle
 * or needs to be reconstructed from a lifecycle event.
 */
export function BrowseControl({label, onSelectFile, onSelectDirectory}: BrowseControlProps) {
    const [open, setOpen] = useState(false)
    // Story 7.10: WebKit does not match `:focus-visible` for an element
    // focused by script (see the CSS comment above `.fd-button:focus-visible,
    // .fd-url:focus-visible` in style.css), so `closeAndReturnFocus` handing
    // focus back to the trigger paints no ring at all on macOS. This flag is
    // the marker `[data-focus-return]` keys off of: set on the two moves this
    // component makes itself (Escape, or an item chosen by keyboard), cleared
    // the moment focus leaves the trigger for any reason. It deliberately
    // does not cover the plain mouse-click toggle below (`onClick`), which
    // never touches it -- a bare `:focus` rule on the trigger would repaint
    // the ring after that click, the regression `:focus-visible` exists to
    // prevent, so the trigger stays on `:focus-visible` for every focus path
    // except this one script-driven return.
    const [triggerFocusReturned, setTriggerFocusReturned] = useState(false)
    const triggerRef = useRef<HTMLButtonElement | null>(null)
    const menuRef = useRef<HTMLDivElement | null>(null)
    const firstItemRef = useRef<HTMLButtonElement | null>(null)
    const triggerId = useId()
    const menuId = useId()

    // Story 7.11: which input is opening the menu, read by the open effect
    // below and set just before every `setOpen(true)`. A ref, not state --
    // nothing here needs to trigger a render of its own, only to be current
    // by the time the effect runs after this open commits. Defaults to
    // 'keyboard' so a mount-time `open === true` (there is none today, but
    // nothing here should silently assume one) pre-selects rather than not.
    const openedByRef = useRef<'pointer' | 'keyboard'>('keyboard')

    // Story 7.11 regression fix: which input marked the *currently* active
    // item, if any -- `null` when no item is marked. Set to `'keyboard'`
    // wherever this component moves focus to an item on purpose (the open
    // effect's keyboard branch, and every navigation move in
    // `handleMenuKeyDown`), and to `'pointer'` by each item's `onMouseEnter`.
    // `handleMenuMouseLeave` below reads it to decide whether the pointer
    // leaving the menu should clear the mark: only when the pointer put it
    // there. A keyboard user's place in the menu must survive the mouse
    // merely passing over it and leaving -- owner report, confirmed on the
    // built binary: "when I move my mouse off of any option they don't
    // unhighlight" was the pointer half of this bug; losing a keyboard
    // selection to an incidental mouse pass would have been a worse one.
    const activeSourceRef = useRef<'pointer' | 'keyboard' | null>(null)

    // Opened, not merely rendered: only a **keyboard** open pre-selects, per
    // "the menu opens, focus lands in it" (I/O matrix) -- the rule that
    // matrix entry was written for. A native menu opened by pointer
    // pre-selects nothing, which is why this now checks `openedByRef` rather
    // than firing unconditionally: firing on every open, including a pointer
    // click, was Story 7.11's first defect -- a sender who clicked the
    // control saw `File` already marked as if a choice had been made for
    // them. Running this only on the open transition, rather than on every
    // render, is what keeps a later re-render from stealing focus back off
    // whichever item the sender has since moved to with the keyboard or the
    // pointer (see the per-item `onMouseEnter` below).
    //
    // Story 7.11 regression, found on the built macOS binary after the fix
    // above first shipped: WebKit does not focus a <button> when it is
    // clicked -- it mirrors the native platform, where a pointer click on a
    // button moves no keyboard focus at all (unlike Chromium, which focuses
    // on mousedown, and unlike jsdom's own fireEvent.click, which moves no
    // focus either -- both of which is why the first version of this fix
    // looked complete in every suite). Doing nothing on a pointer-open,
    // which is what the keyboard-only guard above reduces to for the
    // pointer branch, therefore left focus on neither the trigger nor any
    // item -- on document.body -- so `handleTriggerKeyDown` never fired
    // (Escape and the arrow keys went nowhere) and `handleMenuBlur` never
    // fired (nothing was focused inside the menu for a later blur to
    // report, so a click outside never closed it). All three were reported
    // dead on the shipped app.
    //
    // The fix focuses the menu container itself on a pointer-open --
    // `tabIndex={-1}` on `.fd-browse-menu` below is what makes it a valid
    // target -- rather than the trigger. The container already carries
    // `onKeyDown={handleMenuKeyDown}` and `onBlur={handleMenuBlur}`, so
    // every one of those paths is live the instant the menu opens, and
    // nothing is visually marked because `.fd-browse-menu .fd-button:focus`
    // in style.css only ever matches an item, never the container div. This
    // is also the "no item active" focus target `handleMenuMouseLeave`
    // returns to below -- one answer, in both places, for where focus lives
    // when nothing is marked.
    useEffect(() => {
        if (!open) return
        if (openedByRef.current === 'keyboard') {
            activeSourceRef.current = 'keyboard'
            firstItemRef.current?.focus()
            return
        }
        activeSourceRef.current = null
        menuRef.current?.focus()
    }, [open])

    /**
     * Closes the menu and returns focus to the control that opened it.
     *
     * Used only for an item being chosen by keyboard -- never for Escape
     * (that is `closeAndBlur` below, a deliberately different ending) and
     * never for focus simply leaving the menu on its own, which is
     * `handleMenuBlur` below and must not fight the sender's own focus move.
     *
     * Returning focus before the native chooser opens (rather than after)
     * matters because the menu item that was just activated is about to
     * unmount: if a dismissed dialog relies on the browser's own "return
     * focus to whatever was focused when the dialog opened" behaviour --
     * which nothing here has to reimplement -- that has to be a node that
     * still exists once the dialog closes.
     */
    function closeAndReturnFocus(): void {
        setOpen(false)
        triggerRef.current?.focus()
        // The scripted return this component makes itself, distinct from the
        // trigger's plain mouse-click toggle -- see the flag's declaration
        // above and the CSS `[data-focus-return]` rule it feeds.
        setTriggerFocusReturned(true)
    }

    /**
     * Closes the menu on Escape and leaves nothing focused.
     *
     * Owner: "I feel like escape should remove ANY highlighting of the tabs,
     * not add it in. I feel like that makes more sense." Story 7.10 taught
     * `closeAndReturnFocus` to paint a visible ring on the trigger after a
     * scripted return, because WebKit does not match `:focus-visible` for an
     * element focused by script -- the right fix for keyboard navigation
     * *continuing* (choosing File or Folder). Escape is not that: it reads
     * as "get me out of this," not "put me somewhere," so it must not use
     * that marker at all.
     *
     * Blurring whichever element currently has focus (the menu container on
     * a pointer-open, an item on a keyboard-open, or the trigger itself on
     * the defensive fallback in `handleTriggerKeyDown`) is what this does
     * instead of re-focusing the trigger. Where that focused node is about
     * to unmount anyway (the menu/its items), the browser would move focus
     * to `<body>` on its own once `setOpen(false)` commits; blurring first
     * makes the "nothing is focused" outcome immediate and explicit rather
     * than relying on that unmount side effect, and it is also what reaches
     * the trigger in the one case where nothing unmounts under it.
     *
     * Trade-off, real and worth recording rather than leaving for the next
     * reader to rediscover: with nothing focused, the next Tab restarts from
     * the top of the document instead of continuing from wherever the
     * sender was. Idle has three tab stops, so the cost is small, and no
     * WCAG 2.4.7 obligation is left unmet -- that requirement is about *a*
     * focused component's indicator, and after Escape there is not one to
     * indicate.
     */
    function closeAndBlur(): void {
        (document.activeElement as HTMLElement | null)?.blur()
        setOpen(false)
    }

    function choose(action: () => void): void {
        closeAndReturnFocus()
        action()
    }

    function handleMenuKeyDown(event: KeyboardEvent<HTMLDivElement>): void {
        if (event.key === 'Escape') {
            event.preventDefault()
            closeAndBlur()
            return
        }
        // Tab leaves the menu, so the menu closes. Handled here rather than in
        // handleMenuBlur because that handler cannot tell this apart from a
        // pointer press: the trigger is the only tabbable element in Idle --
        // every other focusable node carries tabIndex={-1} -- so Tab out of an
        // item wraps around the document and lands back on the trigger, which
        // is exactly the relatedTarget a mousedown produces. Found by hand on
        // the built binary, where it left the menu open with focus on the
        // button and ArrowDown re-opening an already open menu, reading as
        // dead.
        //
        // No preventDefault: focus should move. And Tab is not made to cycle
        // the items, which would strand a keyboard sender inside a menu whose
        // only way out is the one tab stop it just left -- the focus trap
        // EXPERIENCE.md forbids outside an OS dialog.
        if (event.key === 'Tab') {
            setOpen(false)
            return
        }
        const navigation = ['ArrowDown', 'ArrowUp', 'Home', 'End']
        if (!navigation.includes(event.key)) return
        event.preventDefault()

        const items = menuRef.current?.querySelectorAll<HTMLButtonElement>('[role="menuitem"]')
        if (items === undefined || items.length === 0) return
        const itemList = [...items]
        // Story 7.11: every navigation move below marks its target as
        // keyboard-sourced, including from the "no item active" state --
        // `document.activeElement` is then the menu container itself (or,
        // on a pointer-opened trigger reached via the defensive branch in
        // `handleTriggerKeyDown` below, the trigger), neither of which is in
        // `itemList`, so `indexOf` below returns -1 and the arithmetic
        // already lands on the first item without special-casing that
        // state. Marking it here is what tells `handleMenuMouseLeave` this
        // item must survive an incidental mouse pass rather than clearing
        // it the way a merely-hovered item would.
        activeSourceRef.current = 'keyboard'
        if (event.key === 'Home') {
            itemList[0]?.focus()
            return
        }
        if (event.key === 'End') {
            itemList[itemList.length - 1]?.focus()
            return
        }
        const currentIndex = itemList.indexOf(document.activeElement as HTMLButtonElement)
        const delta = event.key === 'ArrowDown' ? 1 : -1
        itemList[(currentIndex + delta + itemList.length) % itemList.length]?.focus()
    }

    /**
     * The pointer leaving the menu entirely -- not moving between items,
     * which never fires this (`mouseleave` does not bubble and only fires
     * when the pointer actually exits the element it is bound to).
     *
     * Story 7.11 owner regression: hovering an item moves focus to it (see
     * `onMouseEnter` below), but moving the pointer away does not itself
     * remove focus, so the item stayed marked after the mouse left --
     * "when I move my mouse off of any option they don't unhighlight."
     * Clearing the mark here, by returning focus to the neutral holder the
     * pointer-open branch of the open effect above already uses, is what a
     * native menu does. Gated on `activeSourceRef`, not unconditional: a
     * keyboard user's place in the menu must not be stolen because the
     * mouse happened to drift across the window and leave -- only a
     * pointer-marked item is cleared here.
     */
    function handleMenuMouseLeave(): void {
        if (activeSourceRef.current !== 'pointer') return
        activeSourceRef.current = null
        menuRef.current?.focus()
    }

    /**
     * The trigger's own keys, which the menu's handler cannot see while the
     * trigger itself has focus.
     *
     * ArrowDown and ArrowUp open a menu button -- the convention every
     * native menu follows, and the gesture a keyboard sender reaches for
     * before finding out that Enter also works. That is the branch that
     * matters in ordinary use: the trigger is a normal Tab stop, and
     * pressing an arrow key there while the menu is closed is how a
     * keyboard sender opens it.
     *
     * The `if (open)` branches below (Escape, and the second ArrowDown/
     * ArrowUp branch) used to be the live path for a pointer-opened menu,
     * back when this component assumed a pointer press leaves focus on the
     * trigger. It does not: confirmed on the built macOS binary, WebKit
     * does not focus a `<button>` on click at all, so a real pointer-open
     * never leaves the trigger focused, and the open effect above now
     * focuses the menu container instead precisely so that ITS OWN
     * `onKeyDown` (`handleMenuKeyDown`) is what actually handles Escape and
     * the arrows after a pointer-open, not this function. These two
     * branches are kept as a defensive fallback -- for the one path that
     * can still legitimately land real focus back on the trigger while
     * `open` is true, a Tab landing on it from outside Idle entirely, and
     * for direct-dispatch tests that exercise a handler by name rather than
     * by routing a real keypress through whatever currently has focus (see
     * `BrowseControl.test.tsx`'s "closes on Escape while focus is still on
     * the control" for exactly that precedent) -- not because either is the
     * primary route any more.
     */
    function handleTriggerKeyDown(event: KeyboardEvent<HTMLButtonElement>): void {
        if (event.key === 'Escape') {
            if (!open) return
            event.preventDefault()
            closeAndBlur()
            return
        }
        if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
        event.preventDefault()
        if (open) {
            // Defensive fallback only -- see the doc comment above. Marks
            // the target keyboard-sourced for the same reason
            // `handleMenuKeyDown`'s navigation branch does.
            activeSourceRef.current = 'keyboard'
            firstItemRef.current?.focus()
            return
        }
        openedByRef.current = 'keyboard'
        setOpen(true)
    }

    /**
     * The trigger's plain click/activation toggle.
     *
     * Story 7.11: this one handler covers both a real pointer click and a
     * keyboard activation (Enter/Space), because the browser turns both into
     * the same `click` event on a `<button>` -- `handleTriggerKeyDown` above
     * never sees Enter or Space at all. The two are told apart by
     * `event.detail`: a real pointer click carries the OS click count (1 for
     * a single click, 2+ for a double), while a `click` synthesized from a
     * key press carries `0`. This is what `openedByRef` is set from before
     * every toggle, so the open effect above knows whether to pre-select.
     * Closing (the `was` branch) ignores the reason -- it only matters on the
     * transition into `open`.
     */
    function handleTriggerClick(event: MouseEvent<HTMLButtonElement>): void {
        openedByRef.current = event.detail === 0 ? 'keyboard' : 'pointer'
        setOpen((was) => !was)
    }

    /**
     * Focus leaving the menu on its own -- Tab, or a pointer landing
     * elsewhere -- closes the menu quietly, but must never pull focus back:
     * `EXPERIENCE.md` allows no focus trap outside an OS dialog, and a menu
     * that recaptured focus on every Tab-out would be exactly one, stranding
     * a keyboard user between the trigger and the item that follows it.
     * Escape is the one gesture that explicitly asks to return to the
     * control; this handler leaves focus wherever the sender sent it.
     */
    function handleMenuBlur(event: FocusEvent<HTMLDivElement>): void {
        const next = event.relatedTarget as Node | null
        if (next !== null && menuRef.current?.contains(next)) return
        // Focus moving to the trigger is not the sender leaving the menu: it
        // is a pointer press on the control itself, and mousedown focuses the
        // trigger before the click that follows. Closing here would make that
        // click read `open === false` and reopen the menu, so a second press
        // could never dismiss it -- reproduced in Chromium, and invisible to
        // both suites because fireEvent.click moves no focus. The trigger's
        // own onClick owns that toggle; this handler stays out of its way.
        if (next === triggerRef.current) return
        setOpen(false)
    }

    return (
        <div className="fd-selection">
            <button
                type="button"
                id={triggerId}
                ref={triggerRef}
                // Story 9.3 reverses Story 7.3's full-width rule (which itself
                // reversed Paper Relay's "quieter than the drop zone" rule):
                // the control now sits *inside* the drop zone as its one
                // action, a centred pill of intrinsic width, rather than a
                // full-width row below it. See `IdleView.test.tsx`'s inverted
                // assertion, which names this second reversal explicitly
                // rather than merely deleting the first one's comment.
                className="fd-button fd-button--primary fd-button--pill fd-target"
                aria-haspopup="menu"
                aria-expanded={open}
                // Only while the menu exists: aria-controls names an element by
                // id, and pointing at one that is not rendered is a dangling
                // reference for anything that resolves it.
                aria-controls={open ? menuId : undefined}
                // Present only while a script-driven return is the reason
                // this element is focused; cleared the instant focus leaves
                // it for any reason, so a later Tab or click starts clean.
                // See the flag's declaration above and the CSS rule in
                // style.css that keys off this attribute.
                data-focus-return={triggerFocusReturned ? '' : undefined}
                onClick={handleTriggerClick}
                onKeyDown={handleTriggerKeyDown}
                onBlur={() => setTriggerFocusReturned(false)}
            >
                {label}
                {/* Decorative only: aria-haspopup already tells assistive
                    technology this opens a menu. Shares the disclosure's
                    border-chevron mechanism (style.css) rather than a text
                    glyph -- see that rule for why. */}
                <span className="fd-browse-trigger__chevron" aria-hidden="true"/>
            </button>
            {open ? (
                <div
                    id={menuId}
                    ref={menuRef}
                    role="menu"
                    aria-labelledby={triggerId}
                    className="fd-browse-menu"
                    // Story 7.11 regression fix: a valid `.focus()` target,
                    // not a Tab stop -- same roving-tabindex reasoning as
                    // the items below. This is what lets a pointer-open
                    // focus the container itself (see the open effect
                    // above) so `onKeyDown`/`onBlur` here stay live with no
                    // item marked.
                    tabIndex={-1}
                    onKeyDown={handleMenuKeyDown}
                    onBlur={handleMenuBlur}
                    onMouseLeave={handleMenuMouseLeave}
                >
                    {/*
                      Story 7.11: `onMouseEnter` on each item, not a CSS
                      `:hover` rule, is what marks the item the pointer is
                      over. Moving focus to the hovered item is what makes
                      hover and keyboard navigation share one appearance
                      (style.css's `.fd-browse-menu .fd-button:focus`) and
                      one code path, and it is what guarantees at most one
                      item is ever marked: a `:hover` rule painted alongside
                      `:focus` could mark two at once, which is the defect
                      this story closes. See DESIGN.md's Browse Menu row.
                      Each item also records itself as the pointer-sourced
                      mark (`activeSourceRef`), which is what lets
                      `handleMenuMouseLeave` clear it again on the way out
                      without also clearing a keyboard-placed one.

                      Story 9.3: each item also carries a leading glyph
                      naming its kind, decorative only -- the visible and
                      accessible name is still the text alone
                      (`copy.label.file`/`copy.label.folder`), since the
                      icon is `aria-hidden` and contributes no text node.
                    */}
                    <button
                        type="button"
                        role="menuitem"
                        ref={firstItemRef}
                        // Roving tabindex: the menu is one stop in the tab
                        // order, not one per item. Arrows move within it; Tab
                        // leaves it, which is what closes it.
                        tabIndex={-1}
                        className="fd-button fd-target"
                        onClick={() => choose(onSelectFile)}
                        onMouseEnter={(event) => {
                            activeSourceRef.current = 'pointer'
                            event.currentTarget.focus()
                        }}
                    >
                        <FileGlyph/>
                        {copy.label.file}
                    </button>
                    <button
                        type="button"
                        role="menuitem"
                        tabIndex={-1}
                        className="fd-button fd-target"
                        onClick={() => choose(onSelectDirectory)}
                        onMouseEnter={(event) => {
                            activeSourceRef.current = 'pointer'
                            event.currentTarget.focus()
                        }}
                    >
                        <FolderGlyph/>
                        {copy.label.folder}
                    </button>
                </div>
            ) : null}
        </div>
    )
}

/** Decorative file-kind glyph beside the File menu item (Story 9.3). */
function FileGlyph() {
    return (
        <svg
            className="fd-browse-menu-item__icon"
            viewBox="0 0 16 16"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinejoin="round"
            aria-hidden="true"
            focusable="false"
        >
            <path d="M4 1.5h5l3 3v10H4z"/>
            <path d="M9 1.5v3h3"/>
        </svg>
    )
}

/** Decorative folder-kind glyph beside the Folder menu item (Story 9.3). */
function FolderGlyph() {
    return (
        <svg
            className="fd-browse-menu-item__icon"
            viewBox="0 0 16 16"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinejoin="round"
            aria-hidden="true"
            focusable="false"
        >
            <path d="M1.5 4a1 1 0 0 1 1-1h3.5l1.5 1.5h6a1 1 0 0 1 1 1V12a1 1 0 0 1-1 1h-11a1 1 0 0 1-1-1z"/>
        </svg>
    )
}
