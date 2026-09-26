import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {afterEach, describe, expect, it, vi} from 'vitest'
import type {TransferringTransferState} from '../transfer/state'
import type {FileMetadata, ProgressSnapshot} from '../transfer/types'
import {TransferringView} from './TransferringView'

afterEach(cleanup)

const sessionId = '0123456789abcdef0123456789abcdef'

// Matches TransferringView.tsx's own `ringRadius`/`ringCircumference` -- kept
// as a second, independent computation rather than an import, so a broken
// formula in the view cannot also make its own test agree with it.
const ringCircumference = 2 * Math.PI * 96

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

/** The ring panel: the mode-bearing element, same role `.fd-meter` used to carry. */
function meter(): HTMLElement | null {
    return document.querySelector('[data-progress-mode]')
}

describe('known positive totals', () => {
    const snapshot: ProgressSnapshot = {
        bytesSent: 5_800_000,
        totalBytes: 8_400_000,
        totalKnown: true,
        // Deliberately not 5.8/8.4. The ring reads the authoritative byte
        // pair, so this figure reaches no surface: what shows is 69%, and a
        // sender that rounds its own percentage for display cannot make the
        // ring disagree with the counts printed beside it (Epic 1 retrospective
        // item 7).
        percent: 68,
        speedBytesPerSec: 4_700_000,
    }

    it('renders a determinate progress ring carrying the derived percentage', () => {
        render(<TransferringView state={transferring(snapshot)} onCancel={vi.fn()}/>)

        const bar = screen.getByRole('progressbar')
        expect(bar.getAttribute('aria-valuenow')).toBe('69')
        expect(bar.getAttribute('aria-valuemin')).toBe('0')
        expect(bar.getAttribute('aria-valuemax')).toBe('100')
        expect(meter()?.getAttribute('data-progress-mode')).toBe('known-positive')
    })

    it('shows the derived percentage, in tabular numerals, centred in the ring', () => {
        render(<TransferringView state={transferring(snapshot)} onCancel={vi.fn()}/>)

        const pct = document.querySelector('.fd-ring__pct')
        expect(pct?.textContent).toBe('69%')
        expect(screen.queryByText('68%')).toBeNull()
    })

    it('reports wire bytes first and throughput second, captioned Sent and Speed', () => {
        render(<TransferringView state={transferring(snapshot)} onCancel={vi.fn()}/>)

        const metrics = [...document.querySelectorAll('.fd-metric')].map((node) => node.textContent)
        expect(metrics).toEqual(['5.8 MB sentSent', '4.7 MB/sSpeed'])
    })

    it('draws the ring from the byte pair, not from the wire-reported percent', () => {
        render(<TransferringView state={transferring(snapshot)} onCancel={vi.fn()}/>)

        const fill = document.querySelector('.fd-ring__fill') as unknown as SVGElement & {style: CSSStyleDeclaration}
        const derivedValue = 100 * 5_800_000 / 8_400_000
        const derivedOffset = ringCircumference * (1 - derivedValue / 100)
        const wireOffset = ringCircumference * (1 - 68 / 100)

        // The two would visibly disagree, so this assertion has teeth.
        expect(Math.abs(derivedOffset - wireOffset)).toBeGreaterThan(1)
        expect(Number(fill.style.strokeDashoffset)).toBeCloseTo(derivedOffset, 6)
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

    it('states the unknown total and exposes no value on the progress ring', () => {
        const state = transferring(snapshot, {metadata: metadata({name: "Dad's PDFs", isDir: true, size: 36_800_000})})
        render(<TransferringView state={state} onCancel={vi.fn()}/>)

        expect(screen.getByText('Sending — total size unknown')).toBeTruthy()
        const bar = screen.getByRole('progressbar')
        expect(bar.hasAttribute('aria-valuenow')).toBe(false)
        expect(meter()?.getAttribute('data-progress-mode')).toBe('unknown')
    })

    it('uses a static non-directional dashed ring and never a percentage', () => {
        render(<TransferringView state={transferring(snapshot)} onCancel={vi.fn()}/>)

        expect(document.querySelector('.fd-ring__fill--unknown')).toBeTruthy()
        expect(document.querySelector('.fd-ring__pct')).toBeNull()
        expect(document.body.textContent).not.toMatch(/\d+%/)
    })

    it('still reports the actual wire bytes and throughput', () => {
        render(<TransferringView state={transferring(snapshot)} onCancel={vi.fn()}/>)

        expect(screen.getByText('48.2 MB sent')).toBeTruthy()
        expect(screen.getByText('12.4 MB/s')).toBeTruthy()
    })

    it('keeps the folder identity, its logical size distinct from the wire total, and its ZIP note visible', () => {
        const state = transferring(snapshot, {metadata: metadata({name: "Dad's PDFs", isDir: true, size: 36_800_000})})
        render(<TransferringView state={state} onCancel={vi.fn()}/>)

        expect(document.querySelector('bdi')?.textContent).toBe("Dad's PDFs")
        expect(screen.getByText('This folder downloads as a ZIP.')).toBeTruthy()
        // The packet tab is gone (Story 9.5): the item row's own meta line
        // carries the kind and the logical size, explicitly labelled so it
        // is never mistaken for the wire total the ZIP stream reports.
        expect(document.querySelector('.fd-meta')?.textContent).toBe('Folder · 36.8 MB logical size')
        expect(screen.getByText('48.2 MB sent')).toBeTruthy()
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

    it('states the literal empty status with no percentage-bearing progress ring', () => {
        const state = transferring(snapshot, {metadata: metadata({name: 'Empty Notes.txt', size: 0})})
        render(<TransferringView state={state} onCancel={vi.fn()}/>)

        expect(screen.getByText('Empty file — 0 bytes to transfer')).toBeTruthy()
        expect(screen.queryByRole('progressbar')).toBeNull()
        expect(meter()?.getAttribute('data-progress-mode')).toBe('known-empty')
    })

    it('keeps a decorative ring that claims nothing', () => {
        render(<TransferringView state={transferring(snapshot)} onCancel={vi.fn()}/>)

        const panel = document.querySelector('.fd-ring-panel')
        expect(panel?.getAttribute('aria-hidden')).toBe('true')
        expect(panel?.hasAttribute('role')).toBe(false)
    })

    it('shows zero wire bytes and omits the meaningless speed', () => {
        render(<TransferringView state={transferring(snapshot)} onCancel={vi.fn()}/>)

        const metrics = [...document.querySelectorAll('.fd-metric')].map((node) => node.textContent)
        expect(metrics).toEqual(['0 bytes sentSent'])
        expect(screen.queryByText('Speed')).toBeNull()
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

    it('still occupies the ring slot with a plain track, so the card never changes shape', () => {
        render(<TransferringView state={transferring(null)} onCancel={vi.fn()}/>)

        expect(document.querySelector('.fd-ring-panel')).toBeTruthy()
        expect(document.querySelector('.fd-ring__fill')).toBeNull()
        expect(document.querySelector('.fd-ring__fill--unknown')).toBeNull()
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
        expect(screen.getByRole('button', {name: 'Canceling'})).toBeTruthy()
    })

    it('is exactly one phase view', () => {
        render(<TransferringView state={transferring(null)} onCancel={vi.fn()}/>)

        const views = [...document.querySelectorAll('[data-phase-view]')]
        expect(views).toHaveLength(1)
        expect(views[0].getAttribute('data-phase-view')).toBe('transferring')
    })
})

describe('the pending cancellation contract', () => {
    it('keeps the control focused, marks it aria-disabled, and refuses a second activation', () => {
        const onCancel = vi.fn()
        const {rerender} = render(<TransferringView state={transferring(null)} onCancel={onCancel}/>)

        const cancel = screen.getByRole('button', {name: 'Cancel'})
        cancel.focus()
        fireEvent.click(cancel)
        expect(onCancel).toHaveBeenCalledTimes(1)

        rerender(<TransferringView state={transferring(null, {cancelPending: true})} onCancel={onCancel}/>)

        const outstanding = screen.getByRole('button', {name: 'Canceling'})
        expect(outstanding).toBe(cancel)
        expect(document.activeElement).toBe(outstanding)
        expect(outstanding.getAttribute('aria-disabled')).toBe('true')
        expect(outstanding.hasAttribute('disabled')).toBe(false)

        fireEvent.click(outstanding)
        expect(onCancel).toHaveBeenCalledTimes(1)
    })

    it('keeps the metrics readable while the cancellation is outstanding', () => {
        const snapshot: ProgressSnapshot = {
            bytesSent: 5_800_000, totalBytes: 8_400_000, totalKnown: true, percent: 68, speedBytesPerSec: 4_700_000,
        }
        render(<TransferringView state={transferring(snapshot, {cancelPending: true})} onCancel={vi.fn()}/>)

        expect(screen.getByText('5.8 MB sent')).toBeTruthy()
        expect(screen.getByRole('progressbar').getAttribute('aria-valuenow')).toBe('69')
    })

    it('marks the sending heading as the target transfer-started focuses', () => {
        render(<TransferringView state={transferring(null)} onCancel={vi.fn()}/>)

        const heading = document.querySelector('[data-focus-target="transferring-heading"]') as HTMLElement
        expect(heading.tagName).toBe('H1')
        expect(heading.getAttribute('tabindex')).toBe('-1')
        expect(heading.textContent).toBe('Sending')
    })
})

describe('the progress ARIA the three modes already carry', () => {
    // Verified rather than rebuilt: Story 1.10 owns the speech throttle and the
    // announcement owner, not the roles. These pin what must not drift.
    it('gives a known positive total a finite value between an explicit min and max', () => {
        const snapshot: ProgressSnapshot = {
            bytesSent: 4_200_000, totalBytes: 8_400_000, totalKnown: true, percent: 50, speedBytesPerSec: 1,
        }
        render(<TransferringView state={transferring(snapshot)} onCancel={vi.fn()}/>)

        const bar = screen.getByRole('progressbar')
        expect(Number(bar.getAttribute('aria-valuenow'))).toBe(50)
        expect(Number.isFinite(Number(bar.getAttribute('aria-valuenow')))).toBe(true)
        expect(bar.getAttribute('aria-valuemin')).toBe('0')
        expect(bar.getAttribute('aria-valuemax')).toBe('100')
    })

    it('omits aria-valuenow entirely for an unknown total', () => {
        const snapshot: ProgressSnapshot = {
            bytesSent: 48_200_000, totalBytes: 0, totalKnown: false, percent: 0, speedBytesPerSec: 1,
        }
        render(<TransferringView state={transferring(snapshot)} onCancel={vi.fn()}/>)

        const bar = screen.getByRole('progressbar')
        expect(bar.hasAttribute('aria-valuenow')).toBe(false)
        expect(bar.hasAttribute('aria-valuemin')).toBe(false)
        expect(bar.hasAttribute('aria-valuemax')).toBe(false)
    })

    it('exposes a known-empty transfer as literal text with no progress ring role at all', () => {
        const snapshot: ProgressSnapshot = {
            bytesSent: 0, totalBytes: 0, totalKnown: true, percent: 0, speedBytesPerSec: 0,
        }
        render(<TransferringView state={transferring(snapshot)} onCancel={vi.fn()}/>)

        expect(screen.queryByRole('progressbar')).toBeNull()
        expect(screen.getByText('Empty file — 0 bytes to transfer')).toBeTruthy()
    })
})
