import {cleanup, createEvent, fireEvent, render, screen} from '@testing-library/react'
import {afterEach, describe, expect, it, vi} from 'vitest'
import {BrowseControl} from './BrowseControl'

afterEach(cleanup)

/*
  Story 9.3 extracted BrowseControl from IdleView.tsx into its own module
  (sprint-status action item `epic-4-retro-item-28`) and gave it a `label`
  prop instead of reading `copy.label.chooseFileOrFolder` itself. Every test
  below is unchanged from IdleView.test.tsx apart from the import path and
  this fixed LABEL constant standing in for whatever a caller passes: the
  component's menu behaviour, focus handling and keyboard/pointer contract
  do not depend on what the trigger's own label says, so the tests stay
  decoupled from the copy registry entirely and every query below reads
  exactly as it did before the extraction.
*/
const LABEL = 'Choose a file or folder'

function show(handlers: Record<string, () => void> = {}) {
    const onSelectFile = handlers.onSelectFile ?? vi.fn()
    const onSelectDirectory = handlers.onSelectDirectory ?? vi.fn()
    const view = render(
        <BrowseControl label={LABEL} onSelectFile={onSelectFile} onSelectDirectory={onSelectDirectory}/>,
    )
    return {view, onSelectFile, onSelectDirectory}
}

describe('the browse trigger chevron matches the disclosure chevron family (defect fix)', () => {
    // Owner-observed defect: the trigger's chevron looked "tiny and thin"
    // next to the disclosure chevrons. Cause: the disclosure marker is the
    // shared CSS border-chevron mechanism (`.fd-disclosure__chevron`, a
    // 12x12 box with a rotated 2px border), while the trigger rendered a
    // bare text glyph (U+2304) that inherits the control's font size and
    // renders small and hairline-thin. The fix drops the glyph and gives
    // the trigger the same border-chevron element the disclosures use.
    it('renders the chevron as the shared border-chevron element, not a text glyph', () => {
        show()

        const control = screen.getByRole('button', {name: LABEL})
        const chevron = control.querySelector('.fd-browse-trigger__chevron')!
        expect(chevron).toBeTruthy()
        // A text glyph has visible text content; the shared border-chevron
        // mechanism is an empty decorative box with no text node at all.
        expect(chevron.textContent).toBe('')
        expect(chevron.getAttribute('aria-hidden')).toBe('true')
    })
})

describe('the browse menu', () => {
    it('stays closed until the control is activated', () => {
        show()

        expect(screen.queryByRole('menu')).toBeNull()
        const control = screen.getByRole('button', {name: LABEL})
        expect(control.getAttribute('aria-expanded')).toBe('false')
        expect(control.getAttribute('aria-haspopup')).toBe('menu')
    })

    it('opens on activation, offers both kinds, and lands focus in it', () => {
        show()

        fireEvent.click(screen.getByRole('button', {name: LABEL}))

        const menu = screen.getByRole('menu')
        expect(menu).toBeTruthy()
        const items = screen.getAllByRole('menuitem')
        expect(items.map((item) => item.textContent)).toEqual(['File', 'Folder'])
        expect(document.activeElement).toBe(items[0])
        expect(screen.getByRole('button', {name: LABEL}).getAttribute('aria-expanded'))
            .toBe('true')
    })

    it('reaches the full activation target on every item', () => {
        show()
        fireEvent.click(screen.getByRole('button', {name: LABEL}))

        for (const item of screen.getAllByRole('menuitem')) {
            expect(item.className).toContain('fd-target')
        }
    })

    it('runs the matching command and closes when a kind is chosen', () => {
        const {onSelectFile, onSelectDirectory} = show()

        fireEvent.click(screen.getByRole('button', {name: LABEL}))
        fireEvent.click(screen.getByRole('menuitem', {name: 'File'}))

        expect(onSelectFile).toHaveBeenCalledTimes(1)
        expect(onSelectDirectory).not.toHaveBeenCalled()
        expect(screen.queryByRole('menu')).toBeNull()

        fireEvent.click(screen.getByRole('button', {name: LABEL}))
        fireEvent.click(screen.getByRole('menuitem', {name: 'Folder'}))

        expect(onSelectDirectory).toHaveBeenCalledTimes(1)
        expect(screen.queryByRole('menu')).toBeNull()
    })

    it('returns focus to the control once a kind is chosen', () => {
        show()
        fireEvent.click(screen.getByRole('button', {name: LABEL}))

        fireEvent.click(screen.getByRole('menuitem', {name: 'File'}))

        const trigger = screen.getByRole('button', {name: LABEL})
        expect(document.activeElement).toBe(trigger)
        // Story 7.10: this is the scripted return -- the marker style.css's
        // `[data-focus-return]` rule keys off, kept alive because
        // `:focus-visible` never matches a script-focused element on WebKit.
        // See "the trigger keeps its ring after closeAndReturnFocus" below
        // for the CSS-mutation proof of the consequence.
        expect(trigger.getAttribute('data-focus-return')).toBe('')
    })

    it('closes on Escape, clears focus rather than returning it, and announces nothing', () => {
        // Owner: "I feel like escape should remove ANY highlighting of the
        // tabs, not add it in." Escape is a distinct dismissal path from
        // choosing an item by keyboard (below): it ends with nothing focused,
        // not with the trigger re-focused and ringed. See the trade-off this
        // records in BrowseControl's `closeAndBlur` doc comment.
        show()
        fireEvent.click(screen.getByRole('button', {name: LABEL}))
        const trigger = screen.getByRole('button', {name: LABEL})

        fireEvent.keyDown(screen.getByRole('menu'), {key: 'Escape'})

        expect(screen.queryByRole('menu')).toBeNull()
        expect(document.activeElement).not.toBe(trigger)
        expect(trigger.getAttribute('data-focus-return')).toBeNull()
        // Not asserted here: that nothing was announced. BrowseControl renders
        // no live region in any state, so querying for one passes whatever the
        // menu does -- the announcer belongs to App, and the one-owner rule is
        // pinned there against the routing table. What this can honestly say
        // is that the menu raised no error surface of its own.
        expect(document.querySelector('[role="alert"]')).toBeNull()
    })

    it('never marks the trigger scripted-return for its own plain mouse-click toggle', () => {
        // The trigger's onClick just flips `open` -- it never calls
        // `closeAndReturnFocus`, so a mouse press on it must never carry the
        // marker. If it did, `[data-focus-return]` would repaint the ring
        // after every mouse click on the trigger, the exact stale-ring
        // regression `:focus-visible` was introduced to fix in Epic 1 -- the
        // scar the story text says this marker must not reintroduce.
        show()
        const trigger = screen.getByRole('button', {name: LABEL})

        fireEvent.click(trigger)

        expect(trigger.getAttribute('data-focus-return')).toBeNull()
    })

    it('clears the trigger scripted-return marker on blur', () => {
        // The marker is now set by only one path -- choosing an item by
        // keyboard (`closeAndReturnFocus`, via `choose`) -- since Escape
        // (`closeAndBlur`) never sets it at all. See "closes on Escape,
        // clears focus rather than returning it" above for that half.
        show()
        fireEvent.click(screen.getByRole('button', {name: LABEL}))
        const trigger = screen.getByRole('button', {name: LABEL})
        fireEvent.click(screen.getByRole('menuitem', {name: 'File'}))
        expect(trigger.getAttribute('data-focus-return')).toBe('')

        fireEvent.blur(trigger)

        expect(trigger.getAttribute('data-focus-return')).toBeNull()
    })

    it('closes quietly when focus leaves the menu on its own, without recapturing it', () => {
        const {view} = show()
        fireEvent.click(screen.getByRole('button', {name: LABEL}))
        const menu = screen.getByRole('menu')
        const outside = document.createElement('button')
        view.container.append(outside)

        fireEvent.blur(screen.getByRole('menuitem', {name: 'File'}), {relatedTarget: outside})

        expect(screen.queryByRole('menu')).toBeNull()
        // No focus trap outside an OS dialog: focus is left where the sender
        // sent it, not stolen back to the control.
        expect(document.activeElement).not.toBe(screen.getByRole('button', {name: LABEL}))
        expect(menu.isConnected).toBe(false)
    })

    it('moves focus between items with the arrow keys', () => {
        show()
        fireEvent.click(screen.getByRole('button', {name: LABEL}))
        const [file, folder] = screen.getAllByRole('menuitem')

        fireEvent.keyDown(screen.getByRole('menu'), {key: 'ArrowDown'})
        expect(document.activeElement).toBe(folder)

        fireEvent.keyDown(screen.getByRole('menu'), {key: 'ArrowDown'})
        expect(document.activeElement).toBe(file)

        fireEvent.keyDown(screen.getByRole('menu'), {key: 'ArrowUp'})
        expect(document.activeElement).toBe(folder)
    })
})

/*
  Story 7.11: the menu has one active item, owned by focus, driven by
  whichever input last acted. A pointer open pre-selects nothing; a keyboard
  open focuses the first item as before; hovering an item moves focus to it,
  so hover and keyboard share one appearance and one code path; and the
  dead-key gap (ArrowDown/ArrowUp on a pointer-opened trigger) now moves
  focus into the menu instead of doing nothing.

  `event.detail` is how the trigger's single onClick handler tells a real
  pointer click (detail >= 1) apart from a click synthesized from a keyboard
  activation (detail === 0, the default `fireEvent.click` already uses
  everywhere else in this file, which is why every other test above keeps
  passing unchanged: it reads exactly like a keyboard-style activation, which
  is what it always meant here).
*/
describe('the browse menu has one active item (Story 7.11)', () => {
    function control(): HTMLElement {
        return screen.getByRole('button', {name: LABEL})
    }

    it('pre-selects nothing when opened by a real pointer click', () => {
        show()

        // No manual .focus() staged beforehand: a bare fireEvent.click, like
        // a real pointer click on WebKit, moves no focus by itself. Whatever
        // BrowseControl does with focus after this has to be something the
        // component does on purpose, not something the test manufactured.
        fireEvent.click(control(), {detail: 1})

        expect(screen.getByRole('menu')).toBeTruthy()
        expect(control().getAttribute('aria-expanded')).toBe('true')
        for (const item of screen.getAllByRole('menuitem')) {
            expect(document.activeElement).not.toBe(item)
        }
    })

    it('still focuses the first item when opened by keyboard activation (Enter/Space, detail 0)', () => {
        show()

        fireEvent.click(control(), {detail: 0})

        const items = screen.getAllByRole('menuitem')
        expect(document.activeElement).toBe(items[0])
    })

    it('still focuses the first item when opened by ArrowDown or ArrowUp on the trigger', () => {
        for (const key of ['ArrowDown', 'ArrowUp']) {
            cleanup()
            show()
            fireEvent.keyDown(control(), {key})

            const items = screen.getAllByRole('menuitem')
            expect(document.activeElement).toBe(items[0])
        }
    })

    it('moves focus to an item on hover, and only that item', () => {
        show()
        fireEvent.click(control(), {detail: 1})
        const [file, folder] = screen.getAllByRole('menuitem')
        expect(document.activeElement).not.toBe(file)
        expect(document.activeElement).not.toBe(folder)

        fireEvent.mouseEnter(folder)

        expect(document.activeElement).toBe(folder)
        expect(document.activeElement).not.toBe(file)

        fireEvent.mouseEnter(file)

        expect(document.activeElement).toBe(file)
        expect(document.activeElement).not.toBe(folder)
    })

    it('hands off from keyboard focus to hover focus cleanly -- never two items marked at once', () => {
        show()
        fireEvent.click(control())
        const [file, folder] = screen.getAllByRole('menuitem')
        expect(document.activeElement).toBe(file)

        fireEvent.mouseEnter(folder)

        // document.activeElement can only ever be one node, but the point of
        // this story is the single *rule*, not merely the single DOM API --
        // assert both sides explicitly rather than trusting the API's shape.
        expect(document.activeElement).toBe(folder)
        expect(document.activeElement).not.toBe(file)
    })
})

/*
  Owner regression, found by hand on the built macOS binary after the tests
  above first shipped green: after a pointer-open, Escape did nothing, the
  arrow keys did nothing, and clicking outside the menu did not close it --
  the only way to dismiss it was a second click on the trigger. All three
  worked before Story 7.11.

  Root cause: WebKit does not focus a <button> when it is clicked (it
  mirrors the native platform, where a pointer click on a button moves no
  keyboard focus at all). The first version of the pointer-open fix removed
  the old unconditional "focus the first item on every open," which fixed
  the visual defect (File no longer pre-selected) but never put anything
  else in its place -- so a pointer-open left focus on neither the trigger
  nor any item, on document.body, and:
  - `handleTriggerKeyDown` never fires, because the trigger never has focus.
    That is what killed Escape and the arrow keys.
  - `handleMenuBlur` never fires, because focus was never inside the menu to
    begin with, so there is nothing for a later blur to report. That is
    what killed click-outside dismissal.

  jsdom cannot see this on its own -- a bare fireEvent.click moves no focus
  in jsdom either, which is exactly why it was invisible: the first version
  of these tests staged `control().focus()` by hand before the click, an
  assumption about what a pointer press leaves focused that turned out to be
  false on the one platform that matters here. The tests below do not stage
  any focus: they dispatch every key on `document.activeElement` (or
  document.body, its default), exactly where a real keypress would land,
  never on `control()` directly -- dispatching straight at a specific node
  would keep passing even with the underlying focus bug still present, which
  is how the first version of this story shipped a green suite over three
  dead interactions.

  The fix keeps every one of `BrowseControl`'s own handlers live by giving
  the menu container itself real focus on a pointer-open (`tabIndex={-1}` on
  `.fd-browse-menu`, focused from the open effect) rather than the trigger:
  the container already carries `onKeyDown={handleMenuKeyDown}` and
  `onBlur={handleMenuBlur}`, so Escape, the arrows, and click-outside are
  all live the instant the menu opens, and nothing is visually marked
  because `.fd-browse-menu .fd-button:focus` only ever matches an item, not
  the container div.
*/
describe('a pointer-open keeps Escape, the arrows and click-outside alive (owner regression, confirmed on the macOS binary)', () => {
    function control(): HTMLElement {
        return screen.getByRole('button', {name: LABEL})
    }

    function whereverFocusIs(): Element {
        return (document.activeElement ?? document.body) as Element
    }

    it('Escape still closes a pointer-opened menu', () => {
        show()
        fireEvent.click(control(), {detail: 1})
        expect(screen.getByRole('menu')).toBeTruthy()

        fireEvent.keyDown(whereverFocusIs(), {key: 'Escape'})

        expect(screen.queryByRole('menu')).toBeNull()
    })

    it('ArrowDown still moves focus into a pointer-opened menu', () => {
        show()
        fireEvent.click(control(), {detail: 1})

        fireEvent.keyDown(whereverFocusIs(), {key: 'ArrowDown'})

        const items = screen.getAllByRole('menuitem')
        expect(document.activeElement).toBe(items[0])
    })

    it('ArrowUp still moves focus into a pointer-opened menu', () => {
        show()
        fireEvent.click(control(), {detail: 1})

        fireEvent.keyDown(whereverFocusIs(), {key: 'ArrowUp'})

        const items = screen.getAllByRole('menuitem')
        expect(document.activeElement).toBe(items[0])
    })

    it('a focus move to an element outside the control closes a pointer-opened menu', () => {
        const {view} = show()
        fireEvent.click(control(), {detail: 1})
        const outside = document.createElement('button')
        view.container.append(outside)

        // What a real click outside the control does: it moves focus away
        // from wherever the pointer-open fix put it. Dispatched on
        // whichever element that turns out to be, not a hardcoded node, so
        // this does not presuppose which element the fix chose to focus.
        fireEvent.blur(whereverFocusIs(), {relatedTarget: outside})

        expect(screen.queryByRole('menu')).toBeNull()
    })
})

/*
  Second owner finding, same session, same component: "when I move my mouse
  OFF of any option they don't both unhighlight, but when I click on the
  expander again they both unhighlight." Hovering an item moves *focus* to
  it (above), and moving the pointer away does not itself remove focus, so
  the item stayed marked after the mouse left. A native menu clears the
  highlight when the pointer leaves it -- but only when the pointer is what
  put it there: a keyboard user's place in the menu must survive the mouse
  merely passing over it and leaving.
*/
describe('the pointer-marked item unmarks when the pointer leaves the menu (owner regression)', () => {
    function control(): HTMLElement {
        return screen.getByRole('button', {name: LABEL})
    }

    it('clears a hover-marked item when the pointer leaves the whole menu', () => {
        show()
        fireEvent.click(control(), {detail: 1})
        const [file, folder] = screen.getAllByRole('menuitem')
        fireEvent.mouseEnter(folder)
        expect(document.activeElement).toBe(folder)

        fireEvent.mouseLeave(screen.getByRole('menu'))

        expect(document.activeElement).not.toBe(folder)
        expect(document.activeElement).not.toBe(file)
    })

    it('keeps every interaction live after the pointer leaves and unmarks the item (same focus target as the click-outside fix)', () => {
        show()
        fireEvent.click(control(), {detail: 1})
        fireEvent.mouseEnter(screen.getAllByRole('menuitem')[0])
        fireEvent.mouseLeave(screen.getByRole('menu'))

        fireEvent.keyDown(document.activeElement ?? document.body, {key: 'Escape'})

        expect(screen.queryByRole('menu')).toBeNull()
    })

    it('leaves a keyboard-marked item alone when the pointer merely passes over the menu and leaves', () => {
        show()
        // Keyboard-open: the first item is keyboard-marked.
        fireEvent.click(control(), {detail: 0})
        const [file] = screen.getAllByRole('menuitem')
        expect(document.activeElement).toBe(file)

        // The mouse happening to be over the menu and then leaving must not
        // steal a keyboard user's place -- only a pointer-sourced mark is
        // cleared on leave.
        fireEvent.mouseLeave(screen.getByRole('menu'))

        expect(document.activeElement).toBe(file)
    })

    it('leaves a keyboard-marked item alone even after the mouse had previously marked a different one', () => {
        show()
        fireEvent.click(control(), {detail: 1})
        const [file, folder] = screen.getAllByRole('menuitem')
        fireEvent.mouseEnter(folder)
        expect(document.activeElement).toBe(folder)

        // Keyboard takes back over: the arrow key re-marks the item it
        // lands on as keyboard-sourced, superseding the hover.
        fireEvent.keyDown(screen.getByRole('menu'), {key: 'ArrowUp'})
        expect(document.activeElement).toBe(file)

        fireEvent.mouseLeave(screen.getByRole('menu'))

        expect(document.activeElement).toBe(file)
    })
})

/*
  A second press on the control closes the menu it opened.

  The review reproduced the opposite in Chromium: mousedown focuses the
  trigger, which fires focusout from the menu subtree, which closed the menu --
  and the click that followed then read `open === false` and reopened it, so
  the control could never dismiss its own menu. Neither suite could see it,
  because fireEvent.click moves no focus and even browser mode dispatches a
  synthetic event with no default focus action.

  So the focus move is staged explicitly here: blur the menu with relatedTarget
  set to the trigger, which is exactly what a real mousedown does, and only
  then click.
*/
describe('the browse control dismisses its own menu', () => {
    it('closes on a second activation, after the focus move a real press performs', () => {
        show()
        const control = screen.getByRole('button', {name: LABEL})

        fireEvent.click(control)
        expect(screen.getByRole('menu')).toBeTruthy()
        expect(control.getAttribute('aria-expanded')).toBe('true')

        // What a pointer press on the trigger does before its click lands.
        fireEvent.blur(screen.getByRole('menu'), {relatedTarget: control})
        fireEvent.click(control)

        expect(screen.queryByRole('menu')).toBeNull()
        expect(control.getAttribute('aria-expanded')).toBe('false')
    })
})

/*
  The rest of the menu-button pattern, which the first pass claimed and did not
  have.

  The review found a menu that was a menu in role only: every item its own tab
  stop rather than the pattern's single one, no Home or End, the arrow keys
  that conventionally open a menu button doing nothing on the trigger, Escape
  dead whenever focus sat on the trigger -- which is exactly where a pointer
  press leaves it -- and aria-controls naming an element that does not exist
  while the menu is closed.
*/
describe('the browse menu follows the menu-button pattern', () => {
    function control(): HTMLElement {
        return screen.getByRole('button', {name: LABEL})
    }

    it('opens on ArrowDown and on ArrowUp, not only on activation', () => {
        for (const key of ['ArrowDown', 'ArrowUp']) {
            cleanup()
            show()
            fireEvent.keyDown(control(), {key})

            expect(screen.getByRole('menu')).toBeTruthy()
            expect(control().getAttribute('aria-expanded')).toBe('true')
        }
    })

    it('closes on Escape while focus is still on the control, and clears focus rather than leaving the trigger ringed', () => {
        show()
        fireEvent.click(control())
        // What a pointer press leaves behind: the menu open, focus on the
        // trigger rather than inside the menu.
        fireEvent.blur(screen.getByRole('menu'), {relatedTarget: control()})
        // `fireEvent.blur`/`fireEvent.click` dispatch events without moving
        // real jsdom focus, so this is staged explicitly -- the scenario
        // `handleTriggerKeyDown`'s own Escape branch defends is a real Tab
        // landing on the trigger while the menu is open, and the assertions
        // below are meaningless unless the trigger is genuinely focused
        // first.
        control().focus()
        expect(document.activeElement).toBe(control())

        fireEvent.keyDown(control(), {key: 'Escape'})

        expect(screen.queryByRole('menu')).toBeNull()
        expect(control().getAttribute('aria-expanded')).toBe('false')
        // This is the defensive fallback branch (`handleTriggerKeyDown`'s own
        // Escape case, focus already on the trigger rather than routed
        // through `handleMenuKeyDown`) -- it must clear focus exactly like
        // the primary path does, not leave the trigger focused and ringed.
        expect(document.activeElement).not.toBe(control())
        expect(control().getAttribute('data-focus-return')).toBeNull()
    })

    it('is one tab stop, with the items reachable by arrow rather than by Tab', () => {
        show()
        fireEvent.click(control())

        for (const item of screen.getAllByRole('menuitem')) {
            expect(item.getAttribute('tabindex')).toBe('-1')
        }
    })

    /*
      Found by hand on the built binary, not by any suite.

      The trigger is the only tabbable element the control renders -- every
      other focusable node here carries tabIndex={-1} -- so Tab out of a
      menu item finds nothing after it, wraps around the document, and
      lands back on the trigger. handleMenuBlur cannot tell that from the
      mousedown a pointer press performs, so its trigger guard kept the menu
      open with focus on the button, where ArrowDown only re-opened an already
      open menu and read as dead.

      Tab closes the menu, which is the menu-button pattern and is also the only
      option that does not trap: cycling Tab between the two items would leave a
      keyboard sender no way out, and EXPERIENCE.md allows no focus trap outside
      an OS dialog.
    */
    it('closes on Tab, which in this view wraps focus back onto the control', () => {
        show()
        fireEvent.click(control())
        const menu = screen.getByRole('menu')

        // Captured rather than fired blind: closing the menu is only half of
        // it. Calling preventDefault here would swallow the Tab, so focus would
        // never move and the sender would be stranded on an unmounted item --
        // the same trap by another route. jsdom cannot observe real tab
        // movement, but it can observe that the key was left alone.
        const tab = createEvent.keyDown(menu, {key: 'Tab'})
        fireEvent(menu, tab)

        expect(screen.queryByRole('menu')).toBeNull()
        expect(control().getAttribute('aria-expanded')).toBe('false')
        expect(tab.defaultPrevented, 'Tab must reach the browser so focus moves').toBe(false)
    })

    it('closes on Shift+Tab as well, since that leaves the menu too', () => {
        show()
        fireEvent.click(control())

        fireEvent.keyDown(screen.getByRole('menu'), {key: 'Tab', shiftKey: true})

        expect(screen.queryByRole('menu')).toBeNull()
    })

    it('moves to the first and last item on Home and End', () => {
        show()
        fireEvent.click(control())
        const items = screen.getAllByRole('menuitem')

        fireEvent.keyDown(screen.getByRole('menu'), {key: 'End'})
        expect(document.activeElement).toBe(items[items.length - 1])

        fireEvent.keyDown(screen.getByRole('menu'), {key: 'Home'})
        expect(document.activeElement).toBe(items[0])
    })

    it('names no menu in aria-controls while there is no menu', () => {
        show()
        expect(control().getAttribute('aria-controls')).toBeNull()

        fireEvent.click(control())
        const named = control().getAttribute('aria-controls')
        expect(named).toBeTruthy()
        expect(document.getElementById(named as string)).toBe(screen.getByRole('menu'))
    })
})

/*
  Story 9.3: the trigger's label is now a prop, so BrowseControl itself
  carries no assumption about what any particular caller passes. Both of
  these are load-bearing on the extraction: a caller-supplied label must
  reach the trigger's visible and accessible name, and the item vocabulary
  (File/Folder) must stay the registry's own words regardless of what the
  trigger says, since Idle and a future outcome card share the same two
  menu items under different trigger labels.
*/
describe('the trigger label is a prop (Story 9.3 extraction)', () => {
    it('renders whatever label the caller passes as the trigger\'s visible and accessible name', () => {
        render(<BrowseControl label="Send Another" onSelectFile={vi.fn()} onSelectDirectory={vi.fn()}/>)

        expect(screen.getByRole('button', {name: 'Send Another'})).toBeTruthy()
        expect(screen.queryByRole('button', {name: LABEL})).toBeNull()
    })

    it('keeps the menu item words fixed regardless of the trigger label', () => {
        render(<BrowseControl label="Send Another" onSelectFile={vi.fn()} onSelectDirectory={vi.fn()}/>)

        fireEvent.click(screen.getByRole('button', {name: 'Send Another'}))

        expect(screen.getAllByRole('menuitem').map((item) => item.textContent)).toEqual(['File', 'Folder'])
    })
})

describe('the menu items carry a leading glyph (Story 9.3)', () => {
    it('gives each item a decorative, aria-hidden icon ahead of its word', () => {
        show()
        fireEvent.click(screen.getByRole('button', {name: LABEL}))

        for (const item of screen.getAllByRole('menuitem')) {
            const icon = item.querySelector('.fd-browse-menu-item__icon')
            expect(icon).toBeTruthy()
            expect(icon?.getAttribute('aria-hidden')).toBe('true')
        }
    })
})
