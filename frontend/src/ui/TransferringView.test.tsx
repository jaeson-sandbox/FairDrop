import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {afterEach, describe, expect, it, vi} from 'vitest'
import type {TransferringTransferState} from '../transfer/state'
import type {FileMetadata, ProgressSnapshot} from '../transfer/types'
import {TransferringView} from './TransferringView'

afterEach(cleanup)

const sessionId = '0123456789abcdef0123456789abcdef'

function metadata(overrides: Partial<FileMetadata> = {}): FileMetadata {
    return {
        sessionId,
        name: 'Travel Notes.pdf',
        size: 8_400_000,
        isDir: false,
        url: `http://192.0.2.1:34123/download/${'fedcba9876543210fedcba9876543210'}`,
        qrBase64: 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=',
        warnings: [],
        ...overrides,
    }
}

function transferring(
    progress: ProgressSnapshot | null,
    overrides: Partial<TransferringTransferState> = {},
): TransferringTransferState {
    return {
        phase: 'transferring',
        session: {sessionId, lastSeq: 1},
        metadata: metadata(),
        progress,
        cancelPending: false,
        commandError: null,
        ...overrides,
    }
}

function meter(): HTMLElement | null {
    return document.querySelector('[data-progress-mode]')
}

describe('known positive totals', () => {
    const snapshot: ProgressSnapshot = {
        bytesSent: 5_800_000,
        totalBytes: 8_400_000,
        totalKnown: true,
        // Deliberately not 5.8/8.4: the view must show the wire percentage, not
        // one it computed for itself, which would read 69%.
        percent: 68,
        speedBytesPerSec: 4_700_000,
    }

    it('renders a determinate progressbar carrying the wire percentage', () => {
        render(<TransferringView state={transferring(snapshot)} onCancel={vi.fn()}/>)

        const bar = screen.getByRole('progressbar')
        expect(bar.getAttribute('aria-valuenow')).toBe('68')
        expect(bar.getAttribute('aria-valuemin')).toBe('0')
        expect(bar.getAttribute('aria-valuemax')).toBe('100')
        expect(meter()?.getAttribute('data-progress-mode')).toBe('known-positive')
    })

    it('labels the meter with sent-of-total and the same percentage', () => {
        render(<TransferringView state={transferring(snapshot)} onCancel={vi.fn()}/>)

        expect(screen.getByText('5.8 MB of 8.4 MB')).toBeTruthy()
        expect(screen.getByText('68%')).toBeTruthy()
        expect(screen.queryByText('69%')).toBeNull()
    })

    it('reports wire bytes first and throughput second', () => {
        render(<TransferringView state={transferring(snapshot)} onCancel={vi.fn()}/>)

        const metrics = [...document.querySelectorAll('.fd-metric')].map((node) => node.textContent)
        expect(metrics).toEqual(['5.8 MB sentWire bytes', '4.7 MB/sThroughput'])
    })

    it('fills the track from the wire percentage', () => {
        render(<TransferringView state={transferring(snapshot)} onCancel={vi.fn()}/>)

        const fill = document.querySelector('.fd-meter__fill') as HTMLElement
        expect(fill.style.width).toBe('68%')
    })
})

describe('unknown totals', () => {
    const snapshot: ProgressSnapshot = {
        bytesSent: 48_200_000,
        totalBytes: 0,
        totalKnown: false,
        percent: 0,
        speedBytesPerSec: 12_400_000,
    }

    it('states the unknown total and exposes no value on the progressbar', () => {
        const state = transferring(snapshot, {metadata: metadata({name: "Dad's PDFs", isDir: true, size: 36_800_000})})
        render(<TransferringView state={state} onCancel={vi.fn()}/>)

        expect(screen.getByText('Sending — total size unknown')).toBeTruthy()
        const bar = screen.getByRole('progressbar')
        expect(bar.hasAttribute('aria-valuenow')).toBe(false)
        expect(meter()?.getAttribute('data-progress-mode')).toBe('unknown')
    })

    it('uses a static non-directional pattern and never a percentage', () => {
        render(<TransferringView state={transferring(snapshot)} onCancel={vi.fn()}/>)

        const bar = screen.getByRole('progressbar')
        expect(bar.className).toContain('fd-meter--unknown')
        expect(document.querySelector('.fd-meter__fill')).toBeNull()
        expect(document.body.textContent).not.toMatch(/\d+%/)
    })

    it('still reports the actual wire bytes and throughput', () => {
        render(<TransferringView state={transferring(snapshot)} onCancel={vi.fn()}/>)

        expect(screen.getByText('48.2 MB sent')).toBeTruthy()
        expect(screen.getByText('12.4 MB/s')).toBeTruthy()
    })

    it('keeps the folder identity and its ZIP note visible', () => {
        const state = transferring(snapshot, {metadata: metadata({name: "Dad's PDFs", isDir: true, size: 36_800_000})})
        render(<TransferringView state={state} onCancel={vi.fn()}/>)

        expect(document.querySelector('bdi')?.textContent).toBe("Dad's PDFs")
        expect(screen.getByText('This folder downloads as a ZIP.')).toBeTruthy()
        expect(document.querySelector('.fd-packet-tab')?.textContent).toBe('Folder')
        // The logical size never becomes a wire total for a ZIP stream.
        expect(document.body.textContent).not.toContain('36.8 MB')
    })
})

describe('known empty files', () => {
    const snapshot: ProgressSnapshot = {
        bytesSent: 0,
        totalBytes: 0,
        totalKnown: true,
        percent: 0,
        speedBytesPerSec: 0,
    }

    it('states the literal empty status with no percentage-bearing progressbar', () => {
        const state = transferring(snapshot, {metadata: metadata({name: 'Empty Notes.txt', size: 0})})
        render(<TransferringView state={state} onCancel={vi.fn()}/>)

        expect(screen.getByText('Empty file — 0 bytes to transfer')).toBeTruthy()
        expect(screen.queryByRole('progressbar')).toBeNull()
        expect(meter()?.getAttribute('data-progress-mode')).toBe('known-empty')
    })

    it('keeps a decorative track that claims nothing', () => {
        render(<TransferringView state={transferring(snapshot)} onCancel={vi.fn()}/>)

        const track = document.querySelector('.fd-meter')
        expect(track?.getAttribute('aria-hidden')).toBe('true')
        expect(track?.hasAttribute('role')).toBe(false)
    })

    it('shows zero wire bytes and omits the meaningless speed', () => {
        render(<TransferringView state={transferring(snapshot)} onCancel={vi.fn()}/>)

        const metrics = [...document.querySelectorAll('.fd-metric')].map((node) => node.textContent)
        expect(metrics).toEqual(['0 bytes sentWire bytes'])
        expect(screen.queryByText('Throughput')).toBeNull()
    })
})

describe('before the first accepted snapshot', () => {
    it('shows the packet without inventing a progress mode', () => {
        render(<TransferringView state={transferring(null)} onCancel={vi.fn()}/>)

        expect(screen.getByRole('heading', {level: 1, name: 'Sending'})).toBeTruthy()
        expect(document.querySelector('bdi')?.textContent).toBe('Travel Notes.pdf')
        expect(meter()).toBeNull()
        expect(screen.queryByRole('progressbar')).toBeNull()
        expect(document.querySelectorAll('.fd-metric')).toHaveLength(0)
    })
})

describe('the transfer surface', () => {
    it('has yielded the QR and the link to progress', () => {
        const snapshot: ProgressSnapshot = {
            bytesSent: 1, totalBytes: 8_400_000, totalKnown: true, percent: 0, speedBytesPerSec: 1,
        }
        const {container} = render(<TransferringView state={transferring(snapshot)} onCancel={vi.fn()}/>)

        expect(screen.queryByRole('img')).toBeNull()
        expect(screen.queryByRole('textbox')).toBeNull()
        expect(container.textContent).not.toContain('http://')
        expect(container.textContent).not.toContain('fedcba')
    })

    // Deleting the panel from TransferringView, or narrowing the selector that
    // feeds it, both passed the whole suite before this existed.
    it('shows a failed cancellation on the fixed table without leaving the transfer', () => {
        render(
            <TransferringView
                state={transferring(null, {commandError: {
                    code: 'shutting_down',
                    message: 'FairDrop is closing. Reopen it to start a transfer.',
                }})}
                onCancel={() => undefined}
            />,
        )

        expect(screen.getByText('FairDrop is closing')).toBeTruthy()
        expect(screen.getByText('FairDrop is closing. Reopen it to start a transfer.')).toBeTruthy()
        expect(document.querySelector('[data-phase-view="transferring"]')).toBeTruthy()
        expect(screen.getByRole('button', {name: 'Cancel'})).toBeTruthy()
    })

    it('keeps Cancel on the shared 44px activation floor', () => {
        render(<TransferringView state={transferring(null)} onCancel={() => undefined}/>)

        expect(screen.getByRole('button', {name: 'Cancel'}).className).toContain('fd-target')
    })

    it('offers Cancel and changes its label while a cancellation is outstanding', () => {
        const onCancel = vi.fn()
        const {rerender} = render(<TransferringView state={transferring(null)} onCancel={onCancel}/>)

        fireEvent.click(screen.getByRole('button', {name: 'Cancel'}))
        expect(onCancel).toHaveBeenCalledTimes(1)

        rerender(<TransferringView state={transferring(null, {cancelPending: true})} onCancel={onCancel}/>)
        expect(screen.getByRole('button', {name: 'Canceling…'})).toBeTruthy()
    })

    it('is exactly one phase view', () => {
        render(<TransferringView state={transferring(null)} onCancel={vi.fn()}/>)

        const views = [...document.querySelectorAll('[data-phase-view]')]
        expect(views).toHaveLength(1)
        expect(views[0].getAttribute('data-phase-view')).toBe('transferring')
    })
})
