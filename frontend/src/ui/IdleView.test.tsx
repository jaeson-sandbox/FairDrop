import type {CSSProperties} from 'react'
import {cleanup, fireEvent, render, screen} from '@testing-library/react'
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
    it('leads with the drop target, puts the grouped disclosure list after it and after any command failure', () => {
        const {view} = show()

        const regions = [...view.container.querySelectorAll(
            '.fd-preflight, .fd-drop-zone, .fd-selection, .fd-help',
        )]
        expect(regions.map((element) => element.className.split(' ')[0]))
            .toEqual(['fd-drop-zone', 'fd-selection', 'fd-preflight', 'fd-help'])
    })

    it('opens the outline on the h1, not on the preflight or a command failure', () => {
        show(idle({commandError: {code: 'invalid_selection', message: 'Choose exactly one file or folder.'}}))

        const headings = [...document.querySelectorAll('h1, h2, h3')]
        expect(headings[0].tagName).toBe('H1')
        expect(headings[0].textContent).toBe('Drop one file or folder')
    })

    // Story 9.3: the Idle-only short promise line (`copy.idle.promise`), not
    // `copy.external.promise` -- that key keeps its longer wording for
    // external use and is no longer rendered inside the drop zone.
    it('states the short Idle-only promise line', () => {
        show()

        expect(screen.getByText(
            'Sends to one browser on the same local network. No account or receiver app.',
        )).toBeTruthy()
        expect(screen.queryByText(
            'Send from FairDrop on Windows or Mac to one browser on the same local ' +
                'network—no account or receiver app.',
        )).toBeNull()
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

        const heading = screen.getByRole('heading', {name: 'Drop one file or folder'})
        const zone = heading.closest('.fd-drop-zone') as HTMLElement
        expect(zone).toBeTruthy()
        expect(zone.style.getPropertyValue('--wails-drop-target')).toBe('drop')
        expect(view.container.querySelector('[style*="--wails-drop-target"]')).toBe(zone)
    })

    it('offers one control, labelled for both kinds, that reaches the full activation target', () => {
        const {view} = show()

        const control = screen.getByRole('button', {name: 'Choose File or Folder'})
        expect(control.className).toContain('fd-target')
        // Still primary (inverted for Story 7.3, not deleted then), and now
        // also a pill (Story 9.3): see the width assertion below for the
        // second reversal this story makes.
        expect(control.className).toContain('fd-button--primary')
        expect(control.className).toContain('fd-button--pill')
        expect(view.container.querySelectorAll('.fd-selection button')).toHaveLength(1)
    })

    // Story 9.3 reverses Story 7.3's full-width primary control, which
    // itself had inverted Paper Relay's "quieter than the drop zone" rule:
    // `IdleView.test.tsx` used to assert `fd-button--primary` alone was
    // enough, with a comment naming that first reversal. This is the
    // second one, named the same way rather than silently dropped: the
    // control used to be the full-width row directly under the drop zone
    // (`.fd-selection > .fd-button { width: 100% }`); now that it sits
    // *inside* the drop zone as the zone's one action, DESIGN.md's Browse
    // Control row calls for a centred, intrinsic-width pill instead, and
    // that CSS rule is gone from style.css (see `.fd-selection`'s comment
    // there). *Mutation:* restore `width: 100%` on `.fd-selection > .fd-button`
    // -> this structural placement still holds (the control stays nested in
    // the drop zone either way), but styles.test.ts's own assertion that the
    // rule is gone is what catches a regression of the pill's own width.
    it('nests the browse control inside the drop zone rather than beside it (Story 9.3 reversal)', () => {
        const {view} = show()

        const zone = view.container.querySelector('.fd-drop-zone')!
        const control = screen.getByRole('button', {name: 'Choose File or Folder'})
        expect(zone.contains(control)).toBe(true)

        // Document order inside the zone: glyph, heading, promise, control.
        const inner = zone.querySelector('.fd-drop-zone__inner')!
        const children = [...inner.children]
        expect(children[0].classList.contains('fd-drop-symbol')).toBe(true)
        expect(children[1].tagName).toBe('H1')
        expect(children[2].tagName).toBe('P')
        expect(children[3].contains(control)).toBe(true)
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

describe('the two disclosures render as one grouped list (Story 9.3)', () => {
    it('wraps both disclosures in one .fd-idle-disclosures surface', () => {
        const {view} = show()

        const group = view.container.querySelector('.fd-idle-disclosures')
        expect(group).toBeTruthy()
        expect(group?.querySelector('.fd-preflight')).toBeTruthy()
        expect(group?.querySelector('.fd-help')).toBeTruthy()
    })

    it('keeps the firewall row before the troubleshooting row inside the group', () => {
        const {view} = show()

        const group = view.container.querySelector('.fd-idle-disclosures')!
        const rows = [...group.querySelectorAll('.fd-preflight, .fd-help')]
        expect(rows.map((row) => row.className.split(' ')[0])).toEqual(['fd-preflight', 'fd-help'])
    })
})

describe('the drop zone carries the concentric inner rule', () => {
    it('wraps the instruction in an inner element, distinct from the outer card', () => {
        show()

        const zone = document.querySelector('.fd-drop-zone')!
        const inner = zone.querySelector('.fd-drop-zone__inner')
        expect(inner).toBeTruthy()
        expect(inner?.contains(screen.getByRole('heading', {name: 'Drop one file or folder'}))).toBe(true)
        // Still no click handler and no tab stop on the outer card -- the
        // inner wrapper does not reintroduce either.
        expect(zone.getAttribute('tabindex')).toBeNull()
    })
})

describe('the firewall preflight is a collapsed disclosure (Story 7.3, FR23 amendment)', () => {
    it('renders as a controlled disclosure that is present but not open on first paint', () => {
        show()

        const preflight = document.querySelector('.fd-preflight')!
        // Story 9.1: no longer native <details> -- see Disclosure.tsx and
        // DESIGN.md's Disclosure row for why (a native <details> open cannot
        // smoothly animate across both engines this product ships to). The
        // open/closed state now lives in aria-expanded on the trigger button
        // rather than the presence of an `open` attribute on this element.
        expect(preflight.tagName).toBe('DIV')
        const trigger = preflight.querySelector('.fd-disclosure__summary')!
        expect(trigger.getAttribute('aria-expanded')).toBe('false')
        // Present on first paint, per FR23 amended to "present and preceding"
        // rather than "expanded and preceding": the guidance text exists in
        // the document even while the disclosure reads as closed.
        expect(screen.getByText('Your first transfer may ask to allow FairDrop on this local network.')).toBeTruthy()
    })

    it('names the topic in a keyboard-operable trigger', () => {
        show()

        const summary = document.querySelector('.fd-preflight .fd-disclosure__summary')!
        expect(summary.textContent).toContain('Local network access')
        // Story 9.1: a <button>, not a <summary> -- still a native Tab stop
        // that answers Enter and Space by itself, no keydown handler wired
        // here for either, which is the point.
        expect(summary.tagName).toBe('BUTTON')
        expect(summary.getAttribute('type')).toBe('button')
    })

    it('names the same trigger id in the heading that wraps it, since <button> cannot itself contain a heading', () => {
        // Story 9.1: <button>'s content model is phrasing content only, so
        // the <h2> now wraps the button (the WAI-ARIA APG accordion pattern)
        // instead of sitting inside it the way it sat inside <summary>.
        // `getByRole('heading', ...)` below still finds it either way -- a
        // heading's accessible name is computed from its full text content,
        // descending into the button same as it descended into <summary>.
        show()

        const heading = screen.getByRole('heading', {name: 'Local network access'})
        expect(heading.tagName).toBe('H2')
        expect(heading.querySelector('.fd-disclosure__summary')).toBeTruthy()
    })

    it('follows the browse control (Story 7.8), unlike the always-open preflight, which preceded it', () => {
        const {view} = show()

        const order = [...view.container.querySelectorAll('.fd-preflight, .fd-selection')]
        expect(order.map((el) => el.className.split(' ')[0])).toEqual(['fd-selection', 'fd-preflight'])
    })
})

describe('the disclosure opens and closes via its controlled region (Story 9.1)', () => {
    // The click.contains toggle is the whole state machine now that this is
    // a controlled <button>/region pair rather than native <details>: a
    // native summary answered Enter/Space itself, and a real click routes
    // through the same onClick handler jsdom's fireEvent.click exercises
    // here.
    it('flips aria-expanded and the region\'s data-open together, and back again', () => {
        show()
        const trigger = document.querySelector('.fd-preflight .fd-disclosure__summary') as HTMLElement
        const region = document.querySelector('.fd-preflight .fd-disclosure__region') as HTMLElement
        expect(trigger.getAttribute('aria-expanded')).toBe('false')
        expect(region.hasAttribute('data-open')).toBe(false)

        fireEvent.click(trigger)
        expect(trigger.getAttribute('aria-expanded')).toBe('true')
        expect(region.hasAttribute('data-open')).toBe(true)

        fireEvent.click(trigger)
        expect(trigger.getAttribute('aria-expanded')).toBe('false')
        expect(region.hasAttribute('data-open')).toBe(false)
    })

    it("names the region with aria-controls, so assistive technology can find what the trigger expands", () => {
        show()
        const trigger = document.querySelector('.fd-preflight .fd-disclosure__summary') as HTMLElement
        const region = document.querySelector('.fd-preflight .fd-disclosure__region') as HTMLElement
        expect(trigger.getAttribute('aria-controls')).toBe(region.id)
        expect(region.id).toBeTruthy()
    })

    it('keeps every string RecoveryHelpContent and the firewall preflight carry in the DOM regardless of open state (Mutation: drop a string -> the earlier per-string tests fail naming it)', () => {
        // The region stays mounted whether open or closed -- Story 9.1's
        // collapsed-content guarantee is a transitioned CSS visibility
        // (styles.test.ts), never an unmount, which is what this asserts at
        // the DOM level: the text is here before any click at all.
        show()
        expect(document.querySelector('.fd-preflight .fd-disclosure__region')?.textContent)
            .toContain('Your first transfer may ask to allow FairDrop on this local network.')
        expect(document.querySelector('.fd-help .fd-disclosure__region')?.textContent)
            .toContain('Windows recovery')
    })
})

/*
  Change 1's second Escape path: a keyboard-operable control with no menu
  open at all. The disclosure trigger has no native Escape behaviour to
  preserve -- there is nothing to dismiss -- so this is purely "Escape
  clears focus" in isolation, proving the fix is not merely an accident of
  BrowseControl's own dismissal logic.
*/
describe('Escape clears focus on a focused disclosure summary, with no menu open', () => {
    it('blurs the firewall summary on Escape', () => {
        show()
        const summary = document.querySelector('.fd-preflight .fd-disclosure__summary') as HTMLElement
        summary.focus()
        expect(document.activeElement).toBe(summary)

        fireEvent.keyDown(summary, {key: 'Escape'})

        expect(document.activeElement).not.toBe(summary)
    })

    it('never blurs a routed landing target, which this path never touches', () => {
        // Sanity check for the scoping rule: a landing target is never given
        // an Escape handler at all, so pressing Escape while one is focused
        // (the state heading, focused by script to route an announcement)
        // must leave it exactly as focused as it was.
        show()
        const heading = document.querySelector('[data-focus-target="idle-instruction"]') as HTMLElement
        heading.focus()
        expect(document.activeElement).toBe(heading)

        fireEvent.keyDown(heading, {key: 'Escape'})

        expect(document.activeElement).toBe(heading)
    })
})

describe('recovery guidance is a second collapsed disclosure', () => {
    it('renders as a controlled disclosure, closed by default, distinct from the preflight disclosure', () => {
        show()

        const help = document.querySelector('.fd-help')!
        // Story 9.1: see the equivalent preflight assertion above for why
        // this is no longer <details>/[open].
        expect(help.tagName).toBe('DIV')
        const trigger = help.querySelector('.fd-disclosure__summary')!
        expect(trigger.getAttribute('aria-expanded')).toBe('false')
        // Story 9.3: relabelled "Troubleshooting" (was "Recovery help").
        expect(document.querySelector('.fd-help .fd-disclosure__summary')?.textContent).toContain('Troubleshooting')
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

describe('Idle with a command failure', () => {
    it('renders the fixed invalid-selection panel and stages nothing', () => {
        const error: PublicError = {code: 'invalid_selection', message: 'Choose exactly one file or folder.'}
        show(idle({commandError: error}))

        expect(screen.getByRole('heading', {name: 'Choose one item'})).toBeTruthy()
        expect(screen.getByText('Choose exactly one file or folder.')).toBeTruthy()
        // The failure sits beside Idle, which stays fully usable.
        expect(screen.getByRole('button', {name: 'Choose File or Folder'})).toBeTruthy()
    })

    it('never dresses a cancellation up as an Error', () => {
        const cancelled: PublicError = {code: 'cancelled', message: 'Transfer canceled.'}
        show(idle({commandError: cancelled}))

        expect(document.querySelector('.fd-outcome')).toBeNull()
        expect(screen.queryByText('Transfer canceled.')).toBeNull()
    })

    it('renders the outcome panel between the drop zone and the disclosure group', () => {
        const error: PublicError = {code: 'invalid_selection', message: 'Choose exactly one file or folder.'}
        const {view} = show(idle({commandError: error}))

        const order = [...view.container.querySelectorAll('.fd-drop-zone, .fd-outcome, .fd-idle-disclosures')]
        expect(order.map((el) => el.className.split(' ')[0])).toEqual(['fd-drop-zone', 'fd-outcome', 'fd-idle-disclosures'])
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
        expect(instruction.textContent).toBe('Drop one file or folder')
        expect(instruction.getAttribute('tabindex')).toBe('-1')
    })

    it('gives a command failure its own focus target, distinct from the instruction', () => {
        show(idle({commandError: {code: 'invalid_selection', message: 'Choose exactly one file or folder.'}}))

        const targets = [...document.querySelectorAll('[data-focus-target]')]
            .map((element) => element.getAttribute('data-focus-target'))
        expect(targets).toEqual(['idle-instruction', 'command-error'])
    })
})
