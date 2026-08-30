import type {CSSProperties} from 'react'
import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {afterEach, describe, expect, it, vi} from 'vitest'
import type {IdleTransferState} from '../transfer/state'
import type {PublicError, RetainedOutcome} from '../transfer/types'
import {IdleView} from './IdleView'

afterEach(cleanup)

const dropTargetStyle = {'--wails-drop-target': 'drop'} as CSSProperties

function idle(overrides: Partial<IdleTransferState> = {}): IdleTransferState {
    return {phase: 'idle', retainedOutcome: null, commandError: null, ...overrides}
}

function show(state: IdleTransferState = idle(), handlers: Record<string, () => void> = {}) {
    const onSelectFile = handlers.onSelectFile ?? vi.fn()
    const onSelectDirectory = handlers.onSelectDirectory ?? vi.fn()
    const onDismissRetained = handlers.onDismissRetained ?? vi.fn()
    const view = render(
        <IdleView
            state={state}
            dropTargetStyle={dropTargetStyle}
            onSelectFile={onSelectFile}
            onSelectDirectory={onSelectDirectory}
            onDismissRetained={onDismissRetained}
        />,
    )
    return {view, onSelectFile, onSelectDirectory, onDismissRetained}
}

describe('Idle at rest', () => {
    it('leads with the firewall preflight, then the drop target, then the browse controls', () => {
        const {view} = show()

        const regions = [...view.container.querySelectorAll('.fd-preflight, .fd-drop-zone, .fd-selection')]
        expect(regions.map((element) => element.className.split(' ')[0]))
            .toEqual(['fd-preflight', 'fd-drop-zone', 'fd-selection'])
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

describe('Idle with a retained outcome', () => {
    it('keeps the finished outcome visible with a Dismiss control', () => {
        const retained: RetainedOutcome = {kind: 'done'}
        const {onDismissRetained} = show(idle({retainedOutcome: retained}))

        expect(screen.getByRole('heading', {name: 'Transfer finished'})).toBeTruthy()
        expect(screen.getByText('FairDrop finished sending the item.')).toBeTruthy()

        fireEvent.click(screen.getByRole('button', {name: 'Dismiss'}))
        expect(onDismissRetained).toHaveBeenCalledTimes(1)
    })

    it('keeps a retained failure on the fixed table', () => {
        const retained: RetainedOutcome = {
            kind: 'error',
            error: {code: 'transfer_failed', message: 'The transfer stopped before FairDrop finished sending. ' +
                'Check the local network and create a fresh link.'},
        }
        show(idle({retainedOutcome: retained}))

        expect(screen.getByRole('heading', {name: 'Transfer stopped'})).toBeTruthy()
        expect(screen.getByRole('button', {name: 'Dismiss'})).toBeTruthy()
    })

    it('shows the retained node above a newer command failure, and both at once', () => {
        show(idle({
            retainedOutcome: {kind: 'done'},
            commandError: {code: 'invalid_selection', message: 'Choose exactly one file or folder.'},
        }))

        const panels = [...document.querySelectorAll('.fd-outcome')]
        expect(panels).toHaveLength(2)
        expect(panels[0].getAttribute('data-retained')).toBe('true')
        expect(panels[1].getAttribute('data-error-code')).toBe('invalid_selection')
    })

    it('renders exactly one Idle phase view whatever it carries', () => {
        show(idle({retainedOutcome: {kind: 'done'}}))

        expect(document.querySelectorAll('[data-phase-view]')).toHaveLength(1)
        expect(document.querySelector('[data-phase-view]')?.getAttribute('data-phase-view')).toBe('idle')
    })
})
