import {cleanup, fireEvent, render, screen, within} from '@testing-library/react'
import {afterEach, describe, expect, it, vi} from 'vitest'
import type {ReceiveReceivingTransferState} from '../transfer/state'
import type {ProgressSnapshot, ReceiveMetadata} from '../transfer/types'
import {ReceivingView} from './ReceivingView'

afterEach(cleanup)

const sessionId = '0102030405060708090a0b0c0d0e0f10'
const metadata: ReceiveMetadata = {
    sessionId,
    destination: 'Phone Photos',
    url: 'http://192.168.1.50:45678/upload/1112131415161718191a1b1c1d1e1f20',
    qrBase64: 'iVBORw0KGgo=',
    warnings: [],
}

function snapshot(bytesSent: number, totalBytes: number): ProgressSnapshot {
    return {bytesSent, totalBytes, totalKnown: true, percent: (100 * bytesSent) / totalBytes, speedBytesPerSec: 5_100_000}
}

function receiving(overrides: Partial<ReceiveReceivingTransferState> = {}): ReceiveReceivingTransferState {
    return {
        phase: 'receive-receiving',
        session: {sessionId, lastSeq: 1},
        metadata,
        progress: snapshot(41_000_000, 120_000_000),
        filesSaved: 3,
        cancelPending: false,
        commandError: null,
        ...overrides,
    }
}

function figure(name: 'received' | 'saved' | 'speed'): HTMLElement {
    return document.querySelector(`[data-metric="${name}"]`) as HTMLElement
}

describe('the receiving view', () => {
    it('heads the view as Receiving, a routed focus target', () => {
        render(<ReceivingView state={receiving()} onCancel={vi.fn()}/>)

        const heading = screen.getByRole('heading', {level: 1, name: 'Receiving'})
        expect(heading.getAttribute('data-focus-target')).toBe('receive-receiving-heading')
        expect(heading.getAttribute('tabindex')).toBe('-1')
    })

    it('shows bytes received against the declared size, with a percentage because the total is known', () => {
        render(<ReceivingView state={receiving()} onCancel={vi.fn()}/>)

        const ring = screen.getByRole('progressbar', {name: 'Receiving'})
        expect(ring.getAttribute('aria-valuenow')).toBe('34')
        expect(ring.getAttribute('aria-valuemin')).toBe('0')
        expect(ring.getAttribute('aria-valuemax')).toBe('100')
        expect(within(figure('received')).getByText('41.0 MB of 120.0 MB')).toBeTruthy()
        expect(within(figure('received')).getByText('Received')).toBeTruthy()
    })

    it('derives the percentage from the two integers, not the wire percent', () => {
        const progress = {...snapshot(30, 120), percent: 99}
        render(<ReceivingView state={receiving({progress})} onCancel={vi.fn()}/>)

        expect(screen.getByRole('progressbar').getAttribute('aria-valuenow')).toBe('25')
    })

    it.each([
        [0, 'no files saved'],
        [1, '1 file saved'],
        [2, '2 files saved'],
        [37, '37 files saved'],
    ])('says %i saved as "%s"', (filesSaved, text) => {
        render(<ReceivingView state={receiving({filesSaved})} onCancel={vi.fn()}/>)

        expect(within(figure('saved')).getByText(text)).toBeTruthy()
        expect(within(figure('saved')).getByText('Saved')).toBeTruthy()
    })

    it('says what saved means', () => {
        render(<ReceivingView state={receiving()} onCancel={vi.fn()}/>)
        expect(screen.getByText('A file counts as saved once it has finished.')).toBeTruthy()
    })

    it('shows the speed figure as plain text, and omits it before the first snapshot', () => {
        const {unmount} = render(<ReceivingView state={receiving()} onCancel={vi.fn()}/>)
        expect(within(figure('speed')).getByText('5.1 MB/s')).toBeTruthy()
        unmount()

        render(<ReceivingView state={receiving({progress: null})} onCancel={vi.fn()}/>)
        expect(document.querySelector('[data-metric="speed"]')).toBeNull()
    })

    it('before any snapshot shows a plain track, zero bytes and the count so far', () => {
        render(<ReceivingView state={receiving({progress: null, filesSaved: 0})} onCancel={vi.fn()}/>)

        expect(screen.queryByRole('progressbar')).toBeNull()
        expect(within(figure('received')).getByText('0 bytes')).toBeTruthy()
        expect(within(figure('saved')).getByText('no files saved')).toBeTruthy()
    })

    it('names the destination by its basename only', () => {
        render(<ReceivingView state={receiving()} onCancel={vi.fn()}/>)

        expect(document.querySelector('bdi')?.textContent).toBe('Phone Photos')
        expect(document.body.textContent).not.toContain('/upload/')
    })

    it('offers Cancel, and nothing that sends or shows a folder', () => {
        const onCancel = vi.fn()
        render(<ReceivingView state={receiving()} onCancel={onCancel}/>)

        fireEvent.click(screen.getByRole('button', {name: 'Cancel'}))
        expect(onCancel).toHaveBeenCalledTimes(1)
        expect(screen.getAllByRole('button')).toHaveLength(1)
    })

    it('keeps the control and answers a second Cancel with nothing while one is pending', () => {
        const onCancel = vi.fn()
        render(<ReceivingView state={receiving({cancelPending: true})} onCancel={onCancel}/>)

        const cancel = screen.getByRole('button', {name: 'Canceling'})
        expect(cancel.getAttribute('aria-disabled')).toBe('true')
        fireEvent.click(cancel)
        expect(onCancel).not.toHaveBeenCalled()
    })

    it('shows a failed cancel in the command-error slot', () => {
        render(<ReceivingView state={receiving({commandError: {code: 'busy', message: 'x'}})} onCancel={vi.fn()}/>)

        const panel = document.querySelector('[data-focus-target="command-error"]')
        expect(panel?.getAttribute('data-error-code')).toBe('busy')
    })
})
