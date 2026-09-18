import type {CSSProperties, FocusEvent, KeyboardEvent} from 'react'
import {useEffect, useId, useRef, useState} from 'react'
import {selectCommandError} from '../transfer/selectors'
import type {IdleTransferState} from '../transfer/state'
import {OutcomePanel} from './OutcomePanel'
import {RecoveryHelp} from './RecoveryHelp'
import {copy} from './copy'

interface IdleViewProps {
    readonly state: IdleTransferState
    /** The inherited Wails drop gate, owned by App so the boundary stays in one place. */
    readonly dropTargetStyle: CSSProperties
    /**
     * Whether this Idle was reached by a cancellation winning its race.
     *
     * It cannot be read from the state: a cancel-winning reset lands on plain
     * Idle, which is also how the app starts. App owns the transition, so App
     * owns this flag, and the reducer keeps its two retained-outcome kinds.
     */
    readonly cancelWon: boolean
    readonly onSelectFile: () => void
    readonly onSelectDirectory: () => void
}

/**
 * Idle: the drop target, the cancellation summary, a command failure, the
 * firewall preflight, the one browse control, and recovery help, in that
 * document order.
 *
 * The drop instruction leads because it is this region's `h1`. The spine's one
 * binding ordering rule is that firewall guidance precedes the selection
 * controls, which it does.
 *
 * A retained terminal outcome is not rendered here. App owns it, above this
 * region, so that reset keeps the identical DOM node rather than rebuilding one
 * that merely says the same thing.
 */
export function IdleView({
    state,
    dropTargetStyle,
    cancelWon,
    onSelectFile,
    onSelectDirectory,
}: IdleViewProps) {
    const commandError = selectCommandError(state)

    return (
        <div className="fd-region" data-phase-view="idle">
            <section className="fd-idle">
                {/*
                  The cancel-winning summary, first in the region.

                  It leads because it is the answer to what just happened, and
                  because a retained Done or Error already renders above this
                  view from the shell -- an outcome that appeared under the
                  browse control was the odd one out.

                  Warning, not error. The spine's rule for `cancelled` is
                  "return to Idle; never render as Error", and `--color-error`
                  is the error language here: it appears on nothing but the
                  Error Panel. Amber says "this stopped" without calling a
                  deliberate action a failure. It is a focus target and nothing
                  else -- no live region, because focus owns this transition.
                */}
                {cancelWon ? (
                    <div
                        className="fd-cancel-summary"
                        tabIndex={-1}
                        data-focus-target="cancel-summary"
                    >
                        <span className="fd-cancel-summary__icon" aria-hidden="true">&times;</span>
                        <p className="fd-cancel-summary__text">{copy.cancel.won}</p>
                    </div>
                ) : null}

                {/*
                  A drop target and nothing else. It carries no click handler
                  and no tab stop: the browse control below is the pointer and
                  keyboard path to both choosers.

                  It used to open the file chooser on click, added when only
                  files could be sent. Once folders worked that shortcut
                  contradicted the instruction it sat under -- "file or folder"
                  -- by opening a picker that can only choose a file, and a live
                  run went straight into it. A native chooser is one kind or the
                  other, so the honest click target is the one labelled control,
                  which opens a menu rather than assuming a kind itself. It
                  stays below the firewall preflight, not inside this zone,
                  because FR23 requires the preflight ahead of the selection
                  control.
                */}
                <div
                    className="fd-drop-zone"
                    style={dropTargetStyle}
                >
                    <div>
                        <div className="fd-drop-symbol" aria-hidden="true">↓</div>
                        <h1
                            className="fd-state-heading"
                            tabIndex={-1}
                            data-focus-target="idle-instruction"
                        >
                            {copy.idle.instruction}
                        </h1>
                        <p className="fd-meta">{copy.external.promise}</p>
                    </div>
                </div>

                {commandError === null ? null : (
                    <OutcomePanel
                        outcome={{kind: 'error', retained: false, error: commandError}}
                        focusTarget="command-error"
                    />
                )}

                <aside className="fd-preflight" aria-labelledby="fd-firewall-heading">
                    <h2 id="fd-firewall-heading" className="fd-preflight__heading">
                        {copy.label.firewallHeading}
                    </h2>
                    <p className="fd-body">{copy.firewall.preflight}</p>
                    <dl>
                        <div>
                            <dt>{copy.label.windows}</dt>
                            <dd>{copy.firewall.windows}</dd>
                        </div>
                        <div>
                            <dt>{copy.label.macos}</dt>
                            <dd>{copy.firewall.macos}</dd>
                        </div>
                    </dl>
                </aside>

                <BrowseControl onSelectFile={onSelectFile} onSelectDirectory={onSelectDirectory}/>

                <RecoveryHelp/>
            </section>
        </div>
    )
}

interface BrowseControlProps {
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
function BrowseControl({onSelectFile, onSelectDirectory}: BrowseControlProps) {
    const [open, setOpen] = useState(false)
    const triggerRef = useRef<HTMLButtonElement | null>(null)
    const menuRef = useRef<HTMLDivElement | null>(null)
    const firstItemRef = useRef<HTMLButtonElement | null>(null)
    const triggerId = useId()
    const menuId = useId()

    // Opened, not merely rendered: the item the sender reaches with the very
    // next keystroke is the first one, per "the menu opens, focus lands in
    // it" (I/O matrix). Running this only on the open transition, rather than
    // on every render, is what keeps a later re-render from stealing focus
    // back off whichever item the sender has since moved to with the keyboard.
    useEffect(() => {
        if (open) firstItemRef.current?.focus()
    }, [open])

    /**
     * Closes the menu and returns focus to the control that opened it.
     *
     * Used for Escape and for an item being chosen -- never for focus simply
     * leaving the menu on its own, which is `handleMenuBlur` below and must
     * not fight the sender's own focus move.
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
    }

    function choose(action: () => void): void {
        closeAndReturnFocus()
        action()
    }

    function handleMenuKeyDown(event: KeyboardEvent<HTMLDivElement>): void {
        if (event.key === 'Escape') {
            event.preventDefault()
            closeAndReturnFocus()
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
     * The trigger's own keys, which the menu's handler cannot see.
     *
     * ArrowDown and ArrowUp open a menu button -- the convention every native
     * menu follows, and the gesture a keyboard sender reaches for before
     * finding out that Enter also works. Escape matters here for a different
     * reason: a pointer press leaves focus on the trigger while the menu is
     * open, so without this the one gesture that means "put this away" would
     * do nothing in exactly the state a mouse user is most likely to be in.
     */
    function handleTriggerKeyDown(event: KeyboardEvent<HTMLButtonElement>): void {
        if (event.key === 'Escape') {
            if (!open) return
            event.preventDefault()
            setOpen(false)
            return
        }
        if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
        event.preventDefault()
        setOpen(true)
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
                className="fd-button fd-target"
                aria-haspopup="menu"
                aria-expanded={open}
                // Only while the menu exists: aria-controls names an element by
                // id, and pointing at one that is not rendered is a dangling
                // reference for anything that resolves it.
                aria-controls={open ? menuId : undefined}
                onClick={() => setOpen((was) => !was)}
                onKeyDown={handleTriggerKeyDown}
            >
                {copy.label.chooseFileOrFolder}
            </button>
            {open ? (
                <div
                    id={menuId}
                    ref={menuRef}
                    role="menu"
                    aria-labelledby={triggerId}
                    className="fd-browse-menu"
                    onKeyDown={handleMenuKeyDown}
                    onBlur={handleMenuBlur}
                >
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
                    >
                        {copy.label.file}
                    </button>
                    <button
                        type="button"
                        role="menuitem"
                        tabIndex={-1}
                        className="fd-button fd-target"
                        onClick={() => choose(onSelectDirectory)}
                    >
                        {copy.label.folder}
                    </button>
                </div>
            ) : null}
        </div>
    )
}
