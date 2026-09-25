import type {CSSProperties} from 'react'
import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {afterEach, describe, expect, it, vi} from 'vitest'
import type {FileMetadata, ProgressSnapshot, PublicError} from '../transfer/types'
import type {
    IdleTransferState,
    PendingTransferState,
    StagedTransferState,
    TransferringTransferState,
} from '../transfer/state'
import {IdleView} from './IdleView'
import {OutcomePanel} from './OutcomePanel'
import {StagePendingCard} from './StagePendingCard'
import {StagedView} from './StagedView'
import {TransferringView} from './TransferringView'

vi.mock('../../wailsjs/go/main/App', () => ({CopyToClipboard: vi.fn()}))

afterEach(cleanup)

/*
  Epic 9's global rule, owner-approved 2026-09-25: no visible or accessible
  name of a <button> or role="menuitem" ends in an ellipsis, whether the
  literal character (…) or three dots (...). Status text that is not a
  control -- "Preparing your file…", the "Sending" heading -- is out of this
  rule and keeps its wording (copy.stage.pending.*, which this file does not
  touch).

  This renders every one of the product's views, in every state each ships
  with a button or menu item today, and walks every matching element. A
  single sweep over one fixture would miss a state whose only ellipsis-
  bearing control only ever appears there -- the pending-cancellation label on
  StagePendingCard/StagedView/TransferringView, for one, which only renders
  once `cancelPending` is true.
*/

function assertNoTrailingEllipsis(container: HTMLElement, sceneName: string): void {
    const controls = [
        ...container.querySelectorAll<HTMLElement>('button'),
        ...container.querySelectorAll<HTMLElement>('[role="menuitem"]'),
    ]
    expect(controls.length, `${sceneName}: no buttons or menu items found to check`).toBeGreaterThan(0)

    for (const control of controls) {
        const name = (control.getAttribute('aria-label') ?? control.textContent ?? '').trim()
        expect(
            name.endsWith('…') || name.endsWith('...'),
            `${sceneName}: "${name}" ends in an ellipsis`,
        ).toBe(false)
    }
}

const dropTargetStyle = {'--wails-drop-target': 'drop'} as CSSProperties

function idle(overrides: Partial<IdleTransferState> = {}): IdleTransferState {
    return {phase: 'idle', retainedOutcome: null, commandError: null, ...overrides}
}

function pending(cancelPending: boolean): PendingTransferState {
    return {phase: 'pending', generation: 1, itemKind: 'file', cancelPending}
}

function metadata(overrides: Partial<FileMetadata> = {}): FileMetadata {
    return {
        sessionId: '0'.repeat(32),
        name: 'report.pdf',
        size: 1024,
        isDir: false,
        url: 'http://192.0.2.1:34123/download/deadbeef',
        qrBase64: '',
        warnings: [],
        ...overrides,
    }
}

function staged(overrides: Partial<StagedTransferState> = {}): StagedTransferState {
    return {
        phase: 'staged',
        session: {sessionId: '0'.repeat(32), lastSeq: 0},
        metadata: metadata(),
        cancelPending: false,
        commandError: null,
        ...overrides,
    }
}

function transferring(overrides: Partial<TransferringTransferState> = {}): TransferringTransferState {
    const progress: ProgressSnapshot = {
        bytesSent: 512,
        totalBytes: 1024,
        totalKnown: true,
        percent: 50,
        speedBytesPerSec: 1000,
    }
    return {
        phase: 'transferring',
        session: {sessionId: '0'.repeat(32), lastSeq: 1},
        metadata: metadata(),
        progress,
        cancelPending: false,
        commandError: null,
        ...overrides,
    }
}

const doneReceipt = {name: 'report.pdf', isDir: false, bytesSent: 100}
const error: PublicError = {code: 'transfer_failed', message: 'The transfer stopped.'}

describe('no button or menu item ends in an ellipsis (Epic 9, owner rule 2026-09-25)', () => {
    it('Idle at rest', () => {
        const {container} = render(
            <IdleView
                state={idle()}
                dropTargetStyle={dropTargetStyle}
                cancelWon={false}
                onSelectFile={() => undefined}
                onSelectDirectory={() => undefined}
            />,
        )
        assertNoTrailingEllipsis(container, 'Idle at rest')
    })

    it('Idle with the browse menu open', () => {
        const {container} = render(
            <IdleView
                state={idle()}
                dropTargetStyle={dropTargetStyle}
                cancelWon={false}
                onSelectFile={() => undefined}
                onSelectDirectory={() => undefined}
            />,
        )
        fireEvent.click(screen.getByRole('button', {name: 'Choose File or Folder'}))
        assertNoTrailingEllipsis(container, 'Idle with the browse menu open')
    })

    it('Idle with a Stage-time command failure', () => {
        const {container} = render(
            <IdleView
                state={idle({commandError: {code: 'invalid_selection', message: 'Choose exactly one file or folder.'}})}
                dropTargetStyle={dropTargetStyle}
                cancelWon={false}
                onSelectFile={() => undefined}
                onSelectDirectory={() => undefined}
            />,
        )
        assertNoTrailingEllipsis(container, 'Idle with a command failure')
    })

    it('Stage Pending, cancellation not yet requested', () => {
        const {container} = render(<StagePendingCard state={pending(false)} onCancel={() => undefined}/>)
        assertNoTrailingEllipsis(container, 'Stage Pending')
    })

    it('Stage Pending, cancellation requested (the "Canceling preparation" label)', () => {
        const {container} = render(<StagePendingCard state={pending(true)} onCancel={() => undefined}/>)
        assertNoTrailingEllipsis(container, 'Stage Pending, cancelling')
    })

    it('Staged, cancellation not yet requested', () => {
        const {container} = render(<StagedView state={staged()} onCancel={() => undefined}/>)
        assertNoTrailingEllipsis(container, 'Staged')
    })

    it('Staged, cancellation requested (the "Canceling" label)', () => {
        const {container} = render(<StagedView state={staged({cancelPending: true})} onCancel={() => undefined}/>)
        assertNoTrailingEllipsis(container, 'Staged, cancelling')
    })

    it('Transferring, cancellation not yet requested', () => {
        const {container} = render(<TransferringView state={transferring()} onCancel={() => undefined}/>)
        assertNoTrailingEllipsis(container, 'Transferring')
    })

    it('Transferring, cancellation requested (the "Canceling" label)', () => {
        const {container} = render(
            <TransferringView state={transferring({cancelPending: true})} onCancel={() => undefined}/>,
        )
        assertNoTrailingEllipsis(container, 'Transferring, cancelling')
    })

    it('a live Done outcome', () => {
        const {container} = render(
            <OutcomePanel outcome={{kind: 'done', retained: false, receipt: doneReceipt}} onDismiss={() => undefined}/>,
        )
        assertNoTrailingEllipsis(container, 'live Done outcome')
    })

    it('a retained Done outcome', () => {
        const {container} = render(
            <OutcomePanel outcome={{kind: 'done', retained: true, receipt: doneReceipt}} onDismiss={() => undefined}/>,
        )
        assertNoTrailingEllipsis(container, 'retained Done outcome')
    })

    it('a live Error outcome', () => {
        const {container} = render(
            <OutcomePanel outcome={{kind: 'error', retained: false, error}} onDismiss={() => undefined}/>,
        )
        assertNoTrailingEllipsis(container, 'live Error outcome')
    })

    it('a retained Error outcome', () => {
        const {container} = render(
            <OutcomePanel outcome={{kind: 'error', retained: true, error}} onDismiss={() => undefined}/>,
        )
        assertNoTrailingEllipsis(container, 'retained Error outcome')
    })
})

/*
  Mutation proof: restoring either ellipsis this story removed must fail the
  suite above, naming the offending control. Exercised here directly against
  the shared assertion helper rather than by actually reintroducing the
  regression into copy.ts, so the mutation is provable without leaving the
  registry in a broken state for any other test in the run.
*/
describe('the assertion itself catches a restored ellipsis (mutation proof)', () => {
    it('fails, naming the control, when a button ends in the ellipsis character', () => {
        const container = document.createElement('div')
        container.innerHTML = '<button>Canceling…</button>'

        expect(() => assertNoTrailingEllipsis(container, 'mutation fixture')).toThrowError(/Canceling…/)
    })

    it('fails, naming the control, when a button ends in three literal dots', () => {
        const container = document.createElement('div')
        container.innerHTML = '<button>Canceling preparation...</button>'

        expect(() => assertNoTrailingEllipsis(container, 'mutation fixture'))
            .toThrowError(/Canceling preparation\.\.\./)
    })
})
