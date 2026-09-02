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

describe('the drop target as a pointer shortcut', () => {
    // Reported from the running app: the zone looks like the place to click
    // and did nothing. It runs the same command as the Select File control.
    it('opens the file chooser when the drop target is clicked', () => {
        const {onSelectFile} = show()

        fireEvent.click(document.querySelector('.fd-drop-zone')!)

        expect(onSelectFile).toHaveBeenCalledTimes(1)
    })

    it('keeps the zone out of the tab order, because the buttons are the keyboard path', () => {
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

    it('offers two equal browse controls that each reach the full activation target', () => {
        const {onSelectFile, onSelectDirectory} = show()

        const file = screen.getByRole('button', {name: 'Select File'})
        const directory = screen.getByRole('button', {name: 'Select Directory'})
        for (const control of [file, directory]) {
            expect(control.className).toContain('fd-target')
            expect(control.className).not.toContain('fd-button--primary')
        }

        fireEvent.click(file)
        fireEvent.click(directory)
        expect(onSelectFile).toHaveBeenCalledTimes(1)
        expect(onSelectDirectory).toHaveBeenCalledTimes(1)
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

describe('Idle with a command failure', () => {
    it('renders the fixed invalid-selection panel and stages nothing', () => {
        const error: PublicError = {code: 'invalid_selection', message: 'Choose exactly one file or folder.'}
        show(idle({commandError: error}))

        expect(screen.getByRole('heading', {name: 'Choose one item'})).toBeTruthy()
        expect(screen.getByText('Choose exactly one file or folder.')).toBeTruthy()
        // The failure sits beside Idle, which stays fully usable.
        expect(screen.getByRole('button', {name: 'Select File'})).toBeTruthy()
    })

    it('never dresses a cancellation up as an Error', () => {
        const cancelled: PublicError = {code: 'cancelled', message: 'Transfer canceled.'}
        show(idle({commandError: cancelled}))

        expect(document.querySelector('.fd-outcome')).toBeNull()
        expect(screen.queryByText('Transfer canceled.')).toBeNull()
    })
})

describe('what Idle no longer owns', () => {
    // App renders the retained terminal outcome above this region, in a slot it
    // keeps across the reset. Rebuilding it here would drop the focus that is
    // sitting on it -- see App.test.tsx, "reset after a terminal outcome".
    it('renders no outcome panel for a retained outcome', () => {
        show(idle({retainedOutcome: {kind: 'done'}}))

        expect(document.querySelector('.fd-outcome')).toBeNull()
        expect(screen.queryByRole('button', {name: 'Dismiss'})).toBeNull()
    })

    it('renders exactly one Idle phase view whatever it carries', () => {
        show(idle({retainedOutcome: {kind: 'done'}}))

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
