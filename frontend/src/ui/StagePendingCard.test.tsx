import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {afterEach, describe, expect, it, vi} from 'vitest'
import type {PendingTransferState} from '../transfer/state'
import type {PendingItemKind} from '../transfer/types'
import {StagePendingCard} from './StagePendingCard'

afterEach(cleanup)

function pending(itemKind: PendingItemKind, cancelPending = false): PendingTransferState {
    return {phase: 'pending', generation: 1, itemKind, cancelPending}
}

describe('local preparation copy', () => {
    it('names a file selection', () => {
        render(<StagePendingCard state={pending('file')} onCancel={vi.fn()}/>)

        expect(screen.getByRole('heading', {name: 'Preparing your file…'})).toBeTruthy()
        expect(document.querySelector('.fd-packet-tab')?.textContent).toBe('File')
    })

    it('names a folder selection', () => {
        render(<StagePendingCard state={pending('directory')} onCancel={vi.fn()}/>)

        expect(screen.getByRole('heading', {name: 'Preparing your folder…'})).toBeTruthy()
        expect(document.querySelector('.fd-packet-tab')?.textContent).toBe('Folder')
    })

    it('claims no kind at all for a native drop, which supplies only a path', () => {
        render(<StagePendingCard state={pending('unknown')} onCancel={vi.fn()}/>)

        expect(document.querySelector('.fd-packet-tab')).toBeNull()
        expect(screen.getByRole('heading', {name: 'Preparing your file…'})).toBeTruthy()
        expect(screen.queryByText('Folder')).toBeNull()
    })
})

describe('preparation cancellation', () => {
    it('offers the preparation cancel action and reports its activation once', () => {
        const onCancel = vi.fn()
        render(<StagePendingCard state={pending('file')} onCancel={onCancel}/>)

        const cancel = screen.getByRole('button', {name: 'Cancel preparation'})
        expect(cancel.className).toContain('fd-target')

        fireEvent.click(cancel)
        expect(onCancel).toHaveBeenCalledTimes(1)
    })

    it('changes the visible label while the cancellation is outstanding', () => {
        render(<StagePendingCard state={pending('file', true)} onCancel={vi.fn()}/>)

        expect(screen.getByRole('button', {name: 'Canceling preparation…'})).toBeTruthy()
        expect(screen.queryByRole('button', {name: 'Cancel preparation'})).toBeNull()
    })

    it('uses the preparation wording, not the transfer wording', () => {
        render(<StagePendingCard state={pending('file')} onCancel={vi.fn()}/>)

        expect(screen.queryByRole('button', {name: 'Cancel'})).toBeNull()
        expect(screen.queryByRole('button', {name: 'Canceling…'})).toBeNull()
    })
})

describe('what preparation may not claim', () => {
    it('shows no QR, no link, no progress and no staged wording', () => {
        const {container} = render(<StagePendingCard state={pending('file')} onCancel={vi.fn()}/>)

        expect(screen.queryByRole('img')).toBeNull()
        expect(screen.queryByRole('textbox')).toBeNull()
        expect(screen.queryByRole('progressbar')).toBeNull()
        expect(container.querySelector('a')).toBeNull()
        expect(container.textContent).not.toContain('Ready to pass along')
        expect(container.textContent).not.toContain('http://')
    })

    it('is exactly one phase view', () => {
        render(<StagePendingCard state={pending('directory')} onCancel={vi.fn()}/>)

        const views = [...document.querySelectorAll('[data-phase-view]')]
        expect(views).toHaveLength(1)
        expect(views[0].getAttribute('data-phase-view')).toBe('pending')
    })
})

describe('the pending cancellation contract', () => {
    it('keeps the control focused, marks it aria-disabled, and refuses a second activation', () => {
        const onCancel = vi.fn()
        const {rerender} = render(<StagePendingCard state={pending('file')} onCancel={onCancel}/>)

        const cancel = screen.getByRole('button', {name: 'Cancel preparation'})
        cancel.focus()
        fireEvent.click(cancel)
        expect(onCancel).toHaveBeenCalledTimes(1)

        rerender(<StagePendingCard state={pending('file', true)} onCancel={onCancel}/>)

        const outstanding = screen.getByRole('button', {name: 'Canceling preparation…'})
        expect(outstanding).toBe(cancel)
        expect(document.activeElement).toBe(outstanding)
        expect(outstanding.getAttribute('aria-disabled')).toBe('true')
        // `disabled` would move focus off the control the spine says keeps it.
        expect(outstanding.hasAttribute('disabled')).toBe(false)

        fireEvent.click(outstanding)
        expect(onCancel).toHaveBeenCalledTimes(1)
    })

    it('marks the preparation heading as the target Stage pending focuses', () => {
        render(<StagePendingCard state={pending('file')} onCancel={vi.fn()}/>)

        const heading = document.querySelector('[data-focus-target="pending-heading"]') as HTMLElement
        expect(heading.tagName).toBe('H1')
        expect(heading.getAttribute('tabindex')).toBe('-1')
        expect(heading.textContent).toBe('Preparing your file…')
    })
})
