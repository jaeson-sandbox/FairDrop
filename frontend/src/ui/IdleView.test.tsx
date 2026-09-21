import type {CSSProperties} from 'react'
import {cleanup, createEvent, fireEvent, render, screen} from '@testing-library/react'
import {afterEach, describe, expect, it, vi} from 'vitest'
import type {IdleTransferState} from '../transfer/state'
import type {PublicError} from '../transfer/types'
import {IdleView} from './IdleView'

afterEach(cleanup)

const dropTargetStyle = {'--wails-drop-target': 'drop'} as CSSProperties

function idle(overrides: Partial<IdleTransferState> = {}): IdleTransferState {
    return {phase: 'idle', retainedOutcome: null, commandError: null, ...overrides}
}

const doneReceipt = {name: 'report.pdf', isDir: false, bytesSent: 100}

function show(
    state: IdleTransferState = idle(),
    handlers: Record<string, () => void> = {},
    cancelWon = false,
) {
    const onSelectFile = handlers.onSelectFile ?? vi.fn()
    const onSelectDirectory = handlers.onSelectDirectory ?? vi.fn()
    const view = render(
        <IdleView
            state={state}
            dropTargetStyle={dropTargetStyle}
            cancelWon={cancelWon}
            onSelectFile={onSelectFile}
            onSelectDirectory={onSelectDirectory}
        />,
    )
    return {view, onSelectFile, onSelectDirectory}
}

describe('where the cancellation summary sits', () => {
    // Reported from the running app: it read as nothing in particular, under
    // the selection area. It now leads the region, which is also where a
    // retained Done or Error already appears from the shell.
    it('leads the Idle region rather than following the controls', () => {
        show(idle(), {}, true)
        const region = document.querySelector('.fd-idle')!

        expect(region.firstElementChild?.classList.contains('fd-cancel-summary')).toBe(true)
    })

    it('carries a visible glyph beside the text, hidden from assistive technology', () => {
        show(idle(), {}, true)
        const icon = document.querySelector('.fd-cancel-summary__icon')!

        // The colour is not the only cue, and the glyph is not read twice: the
        // sentence already says the transfer was cancelled.
        expect(icon.textContent?.trim()).not.toBe('')
        expect(icon.getAttribute('aria-hidden')).toBe('true')
    })

    it('is absent entirely when no cancellation won', () => {
        show()

        expect(document.querySelector('.fd-cancel-summary')).toBeNull()
    })
})

describe('the drop target is only a drop target', () => {
    // Reported from a live run: the zone said "file or folder" and clicking it
    // opened the file chooser, so a folder sender was handed a single-file
    // picker. The labelled control below is the only honest click target --
    // it offers both kinds and opens whichever the sender picks -- so the zone
    // opens nothing.
    it('opens no chooser when the drop target is clicked', () => {
        const {onSelectFile, onSelectDirectory} = show()

        fireEvent.click(document.querySelector('.fd-drop-zone')!)

        expect(onSelectFile).not.toHaveBeenCalled()
        expect(onSelectDirectory).not.toHaveBeenCalled()
    })

    it('keeps the zone out of the tab order, because the browse control is the keyboard path', () => {
        show()
        const zone = document.querySelector('.fd-drop-zone')!

        expect(zone.getAttribute('tabindex')).toBeNull()
        expect(zone.tagName).toBe('DIV')
    })
})

describe('Idle at rest', () => {
    it('leads with the drop target, keeps the preflight ahead of the browse controls, and closes with recovery', () => {
        const {view} = show()

        const regions = [...view.container.querySelectorAll(
            '.fd-preflight, .fd-drop-zone, .fd-selection, .fd-help',
        )]
        expect(regions.map((element) => element.className.split(' ')[0]))
            .toEqual(['fd-drop-zone', 'fd-preflight', 'fd-selection', 'fd-help'])
    })

    it('opens the outline on the h1, not on the preflight or a command failure', () => {
        show(idle({commandError: {code: 'invalid_selection', message: 'Choose exactly one file or folder.'}}))

        const headings = [...document.querySelectorAll('h1, h2, h3')]
        expect(headings[0].tagName).toBe('H1')
        expect(headings[0].textContent).toBe('Drop one file or folder.')
    })

    it('states the approved external promise', () => {
        show()

        expect(screen.getByText('Send from FairDrop on Windows or Mac to one browser on the same local ' +
            'network—no account or receiver app.')).toBeTruthy()
    })

    it('offers platform firewall recovery and receiver help from Idle', () => {
        show()

        expect(screen.getByText('Open Windows Firewall settings and allow FairDrop on Private networks only, ' +
            'then prepare the item again.')).toBeTruthy()
        expect(screen.getByText('Open System Settings → Network → Firewall → Options, allow incoming ' +
            'connections for FairDrop, then prepare the item again.')).toBeTruthy()
        expect(screen.getByText('Not downloading? Make sure both devices use the same local Wi-Fi. Guest or ' +
            'isolated networks may block device-to-device traffic. Then cancel and prepare the item again for ' +
            'a fresh link.')).toBeTruthy()
        expect(screen.getByText('Browser says Not Found: the link may be wrong or expired. Locked: another ' +
            'opener claimed it. Gone: the selected item changed. Cancel and prepare the item again for a ' +
            'fresh link.')).toBeTruthy()
    })

    it('states the preflight and both platform guidances in document order', () => {
        show()

        expect(screen.getByRole('heading', {name: 'Local network access'})).toBeTruthy()
        expect(screen.getByText('Your first transfer may ask to allow FairDrop on this local network.')).toBeTruthy()

        const terms = [...document.querySelectorAll('.fd-preflight dt')].map((node) => node.textContent)
        expect(terms).toEqual(['Windows', 'macOS'])
        expect(screen.getByText('Allow FairDrop on Private networks only. Leave Public networks off.')).toBeTruthy()
        expect(screen.getByText('Allow incoming connections for FairDrop.')).toBeTruthy()
    })

    it('instructs the drop inside the gated zone and marks the zone with the inherited property', () => {
        const {view} = show()

        const heading = screen.getByRole('heading', {name: 'Drop one file or folder.'})
        const zone = heading.closest('.fd-drop-zone') as HTMLElement
        expect(zone).toBeTruthy()
        expect(zone.style.getPropertyValue('--wails-drop-target')).toBe('drop')
        expect(view.container.querySelector('[style*="--wails-drop-target"]')).toBe(zone)
    })

    it('offers one control, labelled for both kinds, that reaches the full activation target', () => {
        const {view} = show()

        const control = screen.getByRole('button', {name: 'Choose a file or folder'})
        expect(control.className).toContain('fd-target')
        // Inverted for Story 7.3, not deleted: this used to assert the
        // opposite, encoding Paper Relay's rule that the selection control
        // stays "quieter than the drop zone". Quartz deliberately reverses
        // that -- the drop zone carries no click handler and no tab stop, so
        // it is no longer a control at all, and the browse control is now
        // the one action in Idle. DESIGN.md's Components table specifies it
        // as the full-width primary button.
        expect(control.className).toContain('fd-button--primary')
        expect(view.container.querySelectorAll('.fd-selection button')).toHaveLength(1)
    })

    it('shows no session surface, no history and no QR while idle', () => {
        show()

        expect(screen.queryByRole('img')).toBeNull()
        expect(screen.queryByRole('progressbar')).toBeNull()
        expect(screen.queryByRole('textbox')).toBeNull()
        expect(screen.queryByRole('button', {name: 'Cancel'})).toBeNull()
        expect(document.querySelector('.fd-outcome')).toBeNull()
    })
})

describe('the drop zone carries the concentric inner rule', () => {
    it('wraps the instruction in an inner element, distinct from the outer card', () => {
        show()

        const zone = document.querySelector('.fd-drop-zone')!
        const inner = zone.querySelector('.fd-drop-zone__inner')
        expect(inner).toBeTruthy()
        expect(inner?.contains(screen.getByRole('heading', {name: 'Drop one file or folder.'}))).toBe(true)
        // Still no click handler and no tab stop on the outer card -- the
        // inner wrapper does not reintroduce either.
        expect(zone.getAttribute('tabindex')).toBeNull()
    })
})

describe('the firewall preflight is a collapsed disclosure (Story 7.3, FR23 amendment)', () => {
    it('renders as a <details> that is present but not open on first paint', () => {
        show()

        const preflight = document.querySelector('.fd-preflight')!
        expect(preflight.tagName).toBe('DETAILS')
        expect(preflight.hasAttribute('open')).toBe(false)
        // Present on first paint, per FR23 amended to "present and preceding"
        // rather than "expanded and preceding": the guidance text exists in
        // the document even while the disclosure reads as closed.
        expect(screen.getByText('Your first transfer may ask to allow FairDrop on this local network.')).toBeTruthy()
    })

    it('names the topic in a keyboard-operable summary', () => {
        show()

        const summary = document.querySelector('.fd-preflight > summary')!
        expect(summary.textContent).toContain('Local network access')
        // Native <summary> is a Tab stop and answers Enter/Space by itself --
        // no keydown handler is wired here, which is the point.
        expect(summary.tagName).toBe('SUMMARY')
    })

    it('precedes the browse control, same as the always-open preflight did', () => {
        const {view} = show()

        const order = [...view.container.querySelectorAll('.fd-preflight, .fd-selection')]
        expect(order.map((el) => el.className.split(' ')[0])).toEqual(['fd-preflight', 'fd-selection'])
    })
})

describe('recovery guidance is a second collapsed disclosure', () => {
    it('renders as a <details>, closed by default, distinct from the preflight disclosure', () => {
        show()

        const help = document.querySelector('.fd-help')!
        expect(help.tagName).toBe('DETAILS')
        expect(help.hasAttribute('open')).toBe(false)
        expect(document.querySelector('.fd-help > summary')?.textContent).toContain('Recovery help')
    })

    /*
      Every string RecoveryHelpContent renders must still be rendered here --
      an acceptance criterion names this explicitly. Each assertion below
      fails, naming its own string, if that one line is dropped -- a single
      combined assertion would not say which string went missing.
    */
    it.each([
        ['the Windows recovery instruction', 'Open Windows Firewall settings and allow FairDrop on Private ' +
            'networks only, then prepare the item again.'],
        ['the macOS recovery instruction', 'Open System Settings → Network → Firewall → Options, allow ' +
            'incoming connections for FairDrop, then prepare the item again.'],
        ['the different-network guidance', 'Not downloading? Make sure both devices use the same local Wi-Fi. ' +
            'Guest or isolated networks may block device-to-device traffic. Then cancel and prepare the item ' +
            'again for a fresh link.'],
        ['the receiver-error guidance', 'Browser says Not Found: the link may be wrong or expired. Locked: ' +
            'another opener claimed it. Gone: the selected item changed. Cancel and prepare the item again for ' +
            'a fresh link.'],
    ])('still renders %s', (_label, text) => {
        show()

        expect(screen.getByText(text)).toBeTruthy()
    })

    it('labels the Windows and macOS recovery terms, in document order', () => {
        show()

        const terms = [...document.querySelectorAll('.fd-help dt')].map((node) => node.textContent)
        expect(terms).toEqual(['Windows recovery', 'macOS recovery'])
    })
})

describe('the browse menu', () => {
    it('stays closed until the control is activated', () => {
        show()

        expect(screen.queryByRole('menu')).toBeNull()
        const control = screen.getByRole('button', {name: 'Choose a file or folder'})
        expect(control.getAttribute('aria-expanded')).toBe('false')
        expect(control.getAttribute('aria-haspopup')).toBe('menu')
    })

    it('opens on activation, offers both kinds, and lands focus in it', () => {
        show()

        fireEvent.click(screen.getByRole('button', {name: 'Choose a file or folder'}))

        const menu = screen.getByRole('menu')
        expect(menu).toBeTruthy()
        const items = screen.getAllByRole('menuitem')
        expect(items.map((item) => item.textContent)).toEqual(['File', 'Folder'])
        expect(document.activeElement).toBe(items[0])
        expect(screen.getByRole('button', {name: 'Choose a file or folder'}).getAttribute('aria-expanded'))
            .toBe('true')
    })

    it('reaches the full activation target on every item', () => {
        show()
        fireEvent.click(screen.getByRole('button', {name: 'Choose a file or folder'}))

        for (const item of screen.getAllByRole('menuitem')) {
            expect(item.className).toContain('fd-target')
        }
    })

    it('runs the matching command and closes when a kind is chosen', () => {
        const {onSelectFile, onSelectDirectory} = show()

        fireEvent.click(screen.getByRole('button', {name: 'Choose a file or folder'}))
        fireEvent.click(screen.getByRole('menuitem', {name: 'File'}))

        expect(onSelectFile).toHaveBeenCalledTimes(1)
        expect(onSelectDirectory).not.toHaveBeenCalled()
        expect(screen.queryByRole('menu')).toBeNull()

        fireEvent.click(screen.getByRole('button', {name: 'Choose a file or folder'}))
        fireEvent.click(screen.getByRole('menuitem', {name: 'Folder'}))

        expect(onSelectDirectory).toHaveBeenCalledTimes(1)
        expect(screen.queryByRole('menu')).toBeNull()
    })

    it('returns focus to the control once a kind is chosen', () => {
        show()
        fireEvent.click(screen.getByRole('button', {name: 'Choose a file or folder'}))

        fireEvent.click(screen.getByRole('menuitem', {name: 'File'}))

        expect(document.activeElement).toBe(screen.getByRole('button', {name: 'Choose a file or folder'}))
    })

    it('closes on Escape, returns focus to the control, and announces nothing', () => {
        show()
        fireEvent.click(screen.getByRole('button', {name: 'Choose a file or folder'}))

        fireEvent.keyDown(screen.getByRole('menu'), {key: 'Escape'})

        expect(screen.queryByRole('menu')).toBeNull()
        expect(document.activeElement).toBe(screen.getByRole('button', {name: 'Choose a file or folder'}))
        // Not asserted here: that nothing was announced. IdleView renders no
        // live region in any state, so querying for one passes whatever the
        // menu does -- the announcer belongs to App, and the one-owner rule is
        // pinned there against the routing table. What this can honestly say
        // is that the menu raised no error surface of its own.
        expect(document.querySelector('[role="alert"]')).toBeNull()
    })

    it('closes quietly when focus leaves the menu on its own, without recapturing it', () => {
        const {view} = show()
        fireEvent.click(screen.getByRole('button', {name: 'Choose a file or folder'}))
        const menu = screen.getByRole('menu')
        const outside = document.createElement('button')
        view.container.append(outside)

        fireEvent.blur(screen.getByRole('menuitem', {name: 'File'}), {relatedTarget: outside})

        expect(screen.queryByRole('menu')).toBeNull()
        // No focus trap outside an OS dialog: focus is left where the sender
        // sent it, not stolen back to the control.
        expect(document.activeElement).not.toBe(screen.getByRole('button', {name: 'Choose a file or folder'}))
        expect(menu.isConnected).toBe(false)
    })

    it('moves focus between items with the arrow keys', () => {
        show()
        fireEvent.click(screen.getByRole('button', {name: 'Choose a file or folder'}))
        const [file, folder] = screen.getAllByRole('menuitem')

        fireEvent.keyDown(screen.getByRole('menu'), {key: 'ArrowDown'})
        expect(document.activeElement).toBe(folder)

        fireEvent.keyDown(screen.getByRole('menu'), {key: 'ArrowDown'})
        expect(document.activeElement).toBe(file)

        fireEvent.keyDown(screen.getByRole('menu'), {key: 'ArrowUp'})
        expect(document.activeElement).toBe(folder)
    })
})

describe('Idle with a command failure', () => {
    it('renders the fixed invalid-selection panel and stages nothing', () => {
        const error: PublicError = {code: 'invalid_selection', message: 'Choose exactly one file or folder.'}
        show(idle({commandError: error}))

        expect(screen.getByRole('heading', {name: 'Choose one item'})).toBeTruthy()
        expect(screen.getByText('Choose exactly one file or folder.')).toBeTruthy()
        // The failure sits beside Idle, which stays fully usable.
        expect(screen.getByRole('button', {name: 'Choose a file or folder'})).toBeTruthy()
    })

    it('never dresses a cancellation up as an Error', () => {
        const cancelled: PublicError = {code: 'cancelled', message: 'Transfer canceled.'}
        show(idle({commandError: cancelled}))

        expect(document.querySelector('.fd-outcome')).toBeNull()
        expect(screen.queryByText('Transfer canceled.')).toBeNull()
    })

    it('renders the outcome panel between the drop zone and the preflight disclosure', () => {
        const error: PublicError = {code: 'invalid_selection', message: 'Choose exactly one file or folder.'}
        const {view} = show(idle({commandError: error}))

        const order = [...view.container.querySelectorAll('.fd-drop-zone, .fd-outcome, .fd-preflight')]
        expect(order.map((el) => el.className.split(' ')[0])).toEqual(['fd-drop-zone', 'fd-outcome', 'fd-preflight'])
    })

    it('keeps the command-error focus target unchanged', () => {
        const error: PublicError = {code: 'invalid_selection', message: 'Choose exactly one file or folder.'}
        show(idle({commandError: error}))

        const target = document.querySelector('[data-focus-target="command-error"]')
        expect(target).toBeTruthy()
        expect(target?.getAttribute('tabindex')).toBe('-1')
    })
})

describe('what Idle no longer owns', () => {
    // App renders the retained terminal outcome above this region, in a slot it
    // keeps across the reset. Rebuilding it here would drop the focus that is
    // sitting on it -- see App.test.tsx, "reset after a terminal outcome".
    it('renders no outcome panel for a retained outcome', () => {
        show(idle({retainedOutcome: {kind: 'done', receipt: doneReceipt}}))

        expect(document.querySelector('.fd-outcome')).toBeNull()
        expect(screen.queryByRole('button', {name: 'Dismiss'})).toBeNull()
    })

    it('renders exactly one Idle phase view whatever it carries', () => {
        show(idle({retainedOutcome: {kind: 'done', receipt: doneReceipt}}))

        expect(document.querySelectorAll('[data-phase-view]')).toHaveLength(1)
        expect(document.querySelector('[data-phase-view]')?.getAttribute('data-phase-view')).toBe('idle')
    })
})

describe('Idle after a cancellation won its race', () => {
    it('carries the cancellation summary as a focus target, not as an Error', () => {
        show(idle(), {}, true)

        const summary = document.querySelector('[data-focus-target="cancel-summary"]') as HTMLElement
        // The decorative glyph shares the focused container, so assert the
        // text node rather than the container's raw textContent: the glyph
        // is aria-hidden and is not part of what is announced.
        expect(summary.querySelector('.fd-cancel-summary__text')?.textContent)
            .toBe('Transfer canceled. Ready for another file or folder.')
        expect(summary.getAttribute('tabindex')).toBe('-1')
        // Never an Error, and never a live region: focus is this row's one owner.
        expect(document.querySelector('.fd-outcome')).toBeNull()
        expect(document.querySelector('[role="alert"]')).toBeNull()
        expect(summary.closest('[aria-live]')).toBeNull()
    })

    it('shows no summary on the Idle the app simply starts in', () => {
        show()

        expect(document.querySelector('[data-focus-target="cancel-summary"]')).toBeNull()
    })

    it('marks the drop instruction as the target Dismiss focuses', () => {
        show()

        const instruction = document.querySelector('[data-focus-target="idle-instruction"]') as HTMLElement
        expect(instruction.tagName).toBe('H1')
        expect(instruction.textContent).toBe('Drop one file or folder.')
        expect(instruction.getAttribute('tabindex')).toBe('-1')
    })

    it('gives a command failure its own focus target, distinct from the instruction', () => {
        show(idle({commandError: {code: 'invalid_selection', message: 'Choose exactly one file or folder.'}}))

        const targets = [...document.querySelectorAll('[data-focus-target]')]
            .map((element) => element.getAttribute('data-focus-target'))
        expect(targets).toEqual(['idle-instruction', 'command-error'])
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
        const control = screen.getByRole('button', {name: 'Choose a file or folder'})

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
        return screen.getByRole('button', {name: 'Choose a file or folder'})
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

    it('closes on Escape while focus is still on the control', () => {
        show()
        fireEvent.click(control())
        // What a pointer press leaves behind: the menu open, focus on the
        // trigger rather than inside the menu.
        fireEvent.blur(screen.getByRole('menu'), {relatedTarget: control()})

        fireEvent.keyDown(control(), {key: 'Escape'})

        expect(screen.queryByRole('menu')).toBeNull()
        expect(control().getAttribute('aria-expanded')).toBe('false')
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

      The trigger is the only tabbable element in Idle -- every other focusable
      node here carries tabIndex={-1}, and RecoveryHelp has none -- so Tab out
      of a menu item finds nothing after it, wraps around the document, and
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
