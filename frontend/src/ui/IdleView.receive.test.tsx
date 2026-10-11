import type {CSSProperties} from 'react'
import {act, cleanup, fireEvent, render, screen} from '@testing-library/react'
import {afterEach, describe, expect, it, vi} from 'vitest'
import type {IdleTransferState} from '../transfer/state'
import type {ReceiveActivation} from '../transfer/useTransfer'
import {IdleView} from './IdleView'
import {OutcomePanel} from './OutcomePanel'
import {StagePendingCard} from './StagePendingCard'

afterEach(cleanup)

const dropTargetStyle = {'--wails-drop-target': 'drop'} as CSSProperties

function idle(overrides: Partial<IdleTransferState> = {}): IdleTransferState {
    return {phase: 'idle', retainedOutcome: null, commandError: null, ...overrides}
}

function renderIdle(props: {onReceive?: () => Promise<ReceiveActivation>; state?: IdleTransferState} = {}) {
    return render(
        <IdleView
            state={props.state ?? idle()}
            dropTargetStyle={dropTargetStyle}
            cancelWon={false}
            onSelectFile={vi.fn()}
            onSelectDirectory={vi.fn()}
            onOpenMultiple={vi.fn()}
            onReceive={props.onReceive}
            commandErrorPanelProps={{}}
        />,
    )
}

function isDropTarget(element: Element | null): boolean {
    for (let node = element as HTMLElement | null; node; node = node.parentElement) {
        const value = node.style?.getPropertyValue('--wails-drop-target').trim()
        if (value) return value === 'drop'
    }
    return false
}

describe('the Receive entry in Idle', () => {
    it('is a button named exactly Receive, with a hint that describes it', () => {
        renderIdle({onReceive: () => Promise.resolve('proceeded')})

        const receive = screen.getByRole('button', {name: 'Receive'})
        expect(receive.textContent).toBe('Receive')
        expect(screen.getByText('Get files from a phone on the same local network.')).toBeTruthy()
        const hintId = receive.getAttribute('aria-describedby') as string
        expect(document.getElementById(hintId)?.textContent).toBe('Get files from a phone on the same local network.')
    })

    it('is a 44px target and a secondary control, so the browse pill stays Idle\'s one primary action', () => {
        renderIdle({onReceive: () => Promise.resolve('proceeded')})

        const receive = screen.getByRole('button', {name: 'Receive'})
        expect(receive.className).toContain('fd-target')
        expect(receive.className).toContain('fd-button--secondary')
        expect(receive.className).not.toContain('fd-button--primary')
    })

    it('sits outside the drop zone, so it is never a drop target for sending', () => {
        renderIdle({onReceive: () => Promise.resolve('proceeded')})

        const receive = screen.getByRole('button', {name: 'Receive'})
        expect(receive.closest('.fd-drop-zone')).toBeNull()
        expect(isDropTarget(receive)).toBe(false)
        expect(isDropTarget(screen.getByRole('button', {name: 'Choose File or Folder'}))).toBe(true)
    })

    it('is absent when no handler is wired, and the rest of Idle is unchanged either way', () => {
        const {unmount} = renderIdle()
        expect(screen.queryByRole('button', {name: 'Receive'})).toBeNull()
        expect(screen.getByRole('heading', {level: 1, name: 'Drop one file or folder'})).toBeTruthy()
        expect(screen.getByRole('button', {name: 'Choose File or Folder'})).toBeTruthy()
        expect(screen.getByRole('button', {name: 'Send multiple items'})).toBeTruthy()
        unmount()

        renderIdle({onReceive: () => Promise.resolve('proceeded')})
        expect(screen.getByRole('heading', {level: 1, name: 'Drop one file or folder'})).toBeTruthy()
        expect(screen.getByRole('button', {name: 'Choose File or Folder'})).toBeTruthy()
        expect(screen.getByRole('button', {name: 'Send multiple items'})).toBeTruthy()
        expect(screen.getByRole('button', {name: 'Local network access'})).toBeTruthy()
        expect(screen.getByRole('button', {name: 'Troubleshooting'})).toBeTruthy()
    })

    it('calls the handler once per activation and does not open the browse menu', () => {
        const onReceive = vi.fn().mockResolvedValue('proceeded')
        renderIdle({onReceive})

        fireEvent.click(screen.getByRole('button', {name: 'Receive'}))

        expect(onReceive).toHaveBeenCalledTimes(1)
        expect(screen.queryByRole('menu')).toBeNull()
    })
})

describe('focus after the chooser', () => {
    function deferredActivation() {
        let resolve!: (value: ReceiveActivation) => void
        const promise = new Promise<ReceiveActivation>((resolvePromise) => { resolve = resolvePromise })
        return {promise, resolve}
    }

    it('puts focus on Receive on activation, because WebKit does not focus a clicked button', () => {
        const pending = deferredActivation()
        renderIdle({onReceive: () => pending.promise})
        const receive = screen.getByRole('button', {name: 'Receive'})

        expect(document.activeElement).not.toBe(receive)
        fireEvent.click(receive, {detail: 1})
        expect(document.activeElement).toBe(receive)
    })

    it('returns focus to Receive when the chooser is dismissed, and marks a keyboard activation for the ring', async () => {
        const pending = deferredActivation()
        renderIdle({onReceive: () => pending.promise})
        const receive = screen.getByRole('button', {name: 'Receive'})

        fireEvent.click(receive, {detail: 0})
        // Something else takes focus while the OS dialog is up.
        act(() => (document.body as HTMLElement).focus())
        receive.blur()
        await act(async () => { pending.resolve('dismissed'); await Promise.resolve() })

        expect(document.activeElement).toBe(receive)
        expect(receive.hasAttribute('data-focus-return')).toBe(true)
    })

    it('returns focus after a pointer activation without marking it, so the ring does not paint after a click', async () => {
        const pending = deferredActivation()
        renderIdle({onReceive: () => pending.promise})
        const receive = screen.getByRole('button', {name: 'Receive'})

        fireEvent.click(receive, {detail: 1})
        receive.blur()
        await act(async () => { pending.resolve('dismissed'); await Promise.resolve() })

        expect(document.activeElement).toBe(receive)
        expect(receive.hasAttribute('data-focus-return')).toBe(false)
    })

    it('clears the marker when focus leaves the control', async () => {
        const pending = deferredActivation()
        renderIdle({onReceive: () => pending.promise})
        const receive = screen.getByRole('button', {name: 'Receive'})

        fireEvent.click(receive, {detail: 0})
        await act(async () => { pending.resolve('dismissed'); await Promise.resolve() })
        expect(receive.hasAttribute('data-focus-return')).toBe(true)

        act(() => receive.blur())
        expect(receive.hasAttribute('data-focus-return')).toBe(false)
    })

    it.each(['proceeded', 'ignored'] as const)('does not touch focus or the marker when the activation ends %s', async (result) => {
        const pending = deferredActivation()
        renderIdle({onReceive: () => pending.promise})
        const receive = screen.getByRole('button', {name: 'Receive'})

        fireEvent.click(receive, {detail: 0})
        receive.blur()
        await act(async () => { pending.resolve(result); await Promise.resolve() })

        expect(document.activeElement).not.toBe(receive)
        expect(receive.hasAttribute('data-focus-return')).toBe(false)
    })

    it('does not focus a control that has since left the document', async () => {
        const pending = deferredActivation()
        const {unmount} = renderIdle({onReceive: () => pending.promise})
        const receive = screen.getByRole('button', {name: 'Receive'})

        fireEvent.click(receive, {detail: 0})
        unmount()
        await act(async () => { pending.resolve('dismissed'); await Promise.resolve() })

        expect(document.activeElement).toBe(document.body)
    })
})

describe('the pending receive card', () => {
    it('says the receive is being prepared, names no item kind, and offers Cancel preparation', () => {
        render(<StagePendingCard
            state={{phase: 'pending', generation: 1, itemKind: 'receive', cancelPending: false}}
            onCancel={vi.fn()}
        />)

        expect(screen.getByRole('heading', {level: 1, name: 'Preparing to receive…'}).getAttribute('data-focus-target')).toBe('pending-heading')
        expect(document.querySelector('.fd-packet-tab')).toBeNull()
        expect(screen.getByRole('button', {name: 'Cancel preparation'})).toBeTruthy()
        expect(document.querySelector('[data-phase-view="pending"]')?.getAttribute('data-item-kind')).toBe('receive')
    })

    it('keeps the control and says Canceling preparation while the cancel is pending', () => {
        const onCancel = vi.fn()
        render(<StagePendingCard
            state={{phase: 'pending', generation: 1, itemKind: 'receive', cancelPending: true}}
            onCancel={onCancel}
        />)

        const cancel = screen.getByRole('button', {name: 'Canceling preparation'})
        expect(cancel.getAttribute('aria-disabled')).toBe('true')
        fireEvent.click(cancel)
        expect(onCancel).not.toHaveBeenCalled()
    })
})

describe('a failed Receive card', () => {
    const failure = (code: 'path_not_found' | 'path_unsupported' | 'setup_failed' | 'chooser_failed' | 'busy') => ({
        code,
        message: 'raw adapter text that must never be shown: C:\\Users\\someone',
    })

    it('uses receive wording for the folder it could not use', () => {
        render(<OutcomePanel outcome={{kind: 'error', retained: false, error: failure('path_not_found')}} receive onDismiss={vi.fn()}/>)

        expect(screen.getByRole('heading', {name: 'Folder not found'})).toBeTruthy()
        expect(screen.getByText('That folder is no longer available. Choose it again.')).toBeTruthy()
        expect(document.body.textContent).not.toContain('someone')
    })

    it('says what is wrong with a link, read-only or placeholder folder', () => {
        render(<OutcomePanel outcome={{kind: 'error', retained: false, error: failure('path_unsupported')}} receive onDismiss={vi.fn()}/>)

        expect(screen.getByRole('heading', {name: 'Can’t save there'})).toBeTruthy()
        expect(screen.getByText(/Links, read-only locations and OneDrive-style placeholder folders aren’t supported\./)).toBeTruthy()
    })

    it('falls back to the shared wording for a code whose send wording is already accurate', () => {
        render(<OutcomePanel outcome={{kind: 'error', retained: false, error: failure('busy')}} receive onDismiss={vi.fn()}/>)

        expect(screen.getByRole('heading', {name: 'FairDrop is still busy'})).toBeTruthy()
    })

    it('keeps send wording for a send failure of the same code', () => {
        render(<OutcomePanel outcome={{kind: 'error', retained: false, error: failure('path_not_found')}} onDismiss={vi.fn()}/>)

        expect(screen.getByRole('heading', {name: 'Item not found'})).toBeTruthy()
        expect(screen.getByText('That file or folder is no longer available. Choose it again.')).toBeTruthy()
    })

    it('labels its retry action as given, and calls it', () => {
        const onRetry = vi.fn()
        render(<OutcomePanel
            outcome={{kind: 'error', retained: false, error: failure('chooser_failed')}}
            receive
            onRetry={onRetry}
            retryLabel="Choose Another Folder"
            onDismiss={vi.fn()}
        />)

        fireEvent.click(screen.getByRole('button', {name: 'Choose Another Folder'}))
        expect(onRetry).toHaveBeenCalledTimes(1)
        expect(screen.queryByRole('button', {name: 'Try Again'})).toBeNull()
    })

    it('still says Try Again when no label was given', () => {
        render(<OutcomePanel
            outcome={{kind: 'error', retained: false, error: failure('setup_failed')}}
            receive
            onRetry={vi.fn()}
            onDismiss={vi.fn()}
        />)
        expect(screen.getByRole('button', {name: 'Try Again'})).toBeTruthy()
    })
})
