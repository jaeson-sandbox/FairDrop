import type {CSSProperties, FocusEvent, KeyboardEvent, MouseEvent} from 'react'
import {useEffect, useId, useRef, useState} from 'react'
import {selectCommandError} from '../transfer/selectors'
import type {IdleTransferState} from '../transfer/state'
import {Disclosure} from './Disclosure'
import {OutcomePanel} from './OutcomePanel'
import {RecoveryHelpContent} from './RecoveryHelp'
import {copy} from './copy'

/**
 * DESIGN.md's stated fallback for the FR23 amendments ("Rebuild Idle", Story
 * 7.3; reordered again by Story 7.8): the firewall preflight is collapsed by
 * default inside a keyboard-operable disclosure rather than fully expanded.
 * If acceptance later decides FR23 means the preflight must be *visible*,
 * not merely *present*, this is the one flag that restores that: flip it to
 * `true` and nothing else about the treatment changes.
 */
const FIREWALL_DISCLOSURE_DEFAULT_OPEN = false

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
 * Idle: the drop target, the cancellation summary, a command failure, the one
 * browse control, the firewall preflight, and recovery help, in that document
 * order.
 *
 * The drop instruction leads because it is this region's `h1`. Story 7.8 put
 * the browse control -- the one control that does something -- ahead of both
 * informational disclosures: FR23 no longer binds the preflight to precede
 * the selection control (see DESIGN.md's "FR23 and the disclosures", second
 * amendment), so the ordering rule here is "the control that acts leads",
 * not "firewall guidance leads".
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
                  which opens a menu rather than assuming a kind itself. It is
                  not inside this zone; it renders below it, ahead of both
                  disclosures (Story 7.8) -- the useful control leads, not the
                  firewall preflight.
                */}
                <div
                    className="fd-drop-zone"
                    style={dropTargetStyle}
                >
                    {/*
                      The concentric inner rule (DESIGN.md, Shapes): a 24px
                      card ({rounded.xxl}) padded 7px in, so the inner dashed
                      boundary resolves to an 18px radius ({rounded.xl}) --
                      the parent's radius minus the inset between them, not an
                      independently chosen value.
                    */}
                    <div className="fd-drop-zone__inner">
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

                {/*
                  Story 7.8: the browse control -- the one control in Idle
                  that does something -- is now the first control in document
                  (and tab) order, ahead of both informational disclosures
                  below. Previously the firewall preflight preceded it per
                  FR23; that requirement is amended a second time in
                  DESIGN.md's "FR23 and the disclosures" section.
                */}
                <BrowseControl onSelectFile={onSelectFile} onSelectDirectory={onSelectDirectory}/>

                {/*
                  FR23 amendment (Story 7.3, reordered by Story 7.8): still
                  present on first paint, but collapsed by default inside a
                  keyboard-operable disclosure whose summary names the topic,
                  and now rendered after the browse control rather than
                  before it. See FIREWALL_DISCLOSURE_DEFAULT_OPEN above for
                  the visibility fallback.
                */}
                <Disclosure
                    className="fd-preflight"
                    headingId="fd-firewall-heading"
                    summary={copy.label.firewallHeading}
                    defaultOpen={FIREWALL_DISCLOSURE_DEFAULT_OPEN}
                >
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
                </Disclosure>

                {/*
                  The second disclosure (Story 7.3). Every string
                  RecoveryHelpContent rendered when this block was always
                  open is still rendered here -- none dropped, only collapsed
                  behind a keyboard-operable summary.
                */}
                <Disclosure
                    className="fd-help"
                    headingId="fd-recovery-heading"
                    summary={copy.label.recoveryHeading}
                >
                    <RecoveryHelpContent/>
                </Disclosure>
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
    useEffect(() => {
        if (open && openedByRef.current === 'keyboard') firstItemRef.current?.focus()
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
        // The scripted return this component makes itself, distinct from the
        // trigger's plain mouse-click toggle -- see the flag's declaration
        // above and the CSS `[data-focus-return]` rule it feeds.
        setTriggerFocusReturned(true)
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
     *
     * Story 7.11 closes a second, related gap here: with the menu already
     * open and focus still on the trigger (the pointer-open state, since a
     * pointer open no longer moves focus into the menu), ArrowDown/ArrowUp
     * used to call `setOpen(true)` on a menu that was already open -- no
     * state change, so the open effect never re-ran, and the key did
     * nothing. It now focuses the first item directly in that case, which
     * both satisfies the key and hands off to the menu's own `onKeyDown` for
     * every keypress after this one.
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
        if (open) {
            // Already open with focus still on the trigger (pointer-opened):
            // move focus into the menu rather than no-op `setOpen(true)`.
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
                // Quartz reverses Paper Relay's rule that the selection
                // control stays quieter than the drop zone: the drop zone is
                // no longer a control at all (it carries no click handler and
                // no tab stop, above), so the browse control is the one
                // action in Idle and DESIGN.md's Components table specifies
                // it as the full-width primary button. See
                // `IdleView.test.tsx`'s inverted assertion, which names this
                // reversal explicitly rather than merely deleting the old one.
                className="fd-button fd-button--primary fd-target"
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
                {copy.label.chooseFileOrFolder}
                {/* Decorative only: aria-haspopup already tells assistive
                    technology this opens a menu. */}
                <span className="fd-browse-trigger__chevron" aria-hidden="true">⌄</span>
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
                        onMouseEnter={(event) => event.currentTarget.focus()}
                    >
                        {copy.label.file}
                    </button>
                    <button
                        type="button"
                        role="menuitem"
                        tabIndex={-1}
                        className="fd-button fd-target"
                        onClick={() => choose(onSelectDirectory)}
                        onMouseEnter={(event) => event.currentTarget.focus()}
                    >
                        {copy.label.folder}
                    </button>
                </div>
            ) : null}
        </div>
    )
}
