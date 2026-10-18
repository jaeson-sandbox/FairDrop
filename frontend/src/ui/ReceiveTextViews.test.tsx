import {act, cleanup, fireEvent, render, screen} from '@testing-library/react'
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'

const mocks = vi.hoisted(() => ({copyLink: vi.fn()}))
vi.mock('../../wailsjs/go/main/App', () => ({CopyToClipboard: mocks.copyLink}))

import type {ReceiveTextOutcomeTransferState, ReceiveTextWaitingTransferState} from '../transfer/state'
import {ReceiveTextOutcomeView, ReceiveTextWaitingView} from './ReceiveTextViews'

afterEach(cleanup)
beforeEach(() => { mocks.copyLink.mockReset().mockResolvedValue(undefined) })

const sessionId = '11111111111111111111111111111111'
const url = 'http://192.168.1.5:45678/receive-text/22222222222222222222222222222222'
const png = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII='
const waiting: ReceiveTextWaitingTransferState = {phase: 'receive-text-waiting', session: {sessionId, lastSeq: 0},
    metadata: {sessionId, url, qrBase64: png, warnings: []}, cancelPending: false, commandError: null}
const outcome = (value: string | null): ReceiveTextOutcomeTransferState => ({phase: 'receive-text-outcome',
    session: {sessionId, lastSeq: 2}, kind: 'complete', value, fetchError: false, copyError: false, copying: false})

describe('Receive text views', () => {
    it('keeps the QR non-draggable, the link inert, and recovery described by the focused heading', () => {
        render(<ReceiveTextWaitingView state={{...waiting, recovered: true}} onCancel={vi.fn()} onRefresh={vi.fn()}
            onAnnounce={vi.fn()} onCopyFailed={vi.fn()}/> )
        const image = screen.getByRole('img') as HTMLImageElement
        expect(image.getAttribute('draggable')).toBe('false')
        expect(image.getAttribute('src')).toBe(`data:image/png;base64,${png}`)
        expect(screen.getByLabelText('Text receive link')).toHaveProperty('value', url)
        expect(screen.getByText('Works once: this link accepts one text submission.')).toBeTruthy()
        expect(screen.getByRole('button', {name: 'Show Link'}).getAttribute('aria-expanded')).toBe('false')
        fireEvent.click(screen.getByRole('button', {name: 'Show Link'}))
        expect(screen.getByRole('button', {name: 'Hide Link'}).getAttribute('aria-expanded')).toBe('true')
        expect(document.querySelector('a')).toBeNull()
        const heading = screen.getByRole('heading', {name: 'Waiting for text'})
        expect(document.getElementById(heading.getAttribute('aria-describedby') as string)?.textContent)
            .toBe('New code ready. The old code no longer works.')
    })

    it('shows literal normalized text in a readonly selectable field and only copies on activation', async () => {
        const copy = vi.fn().mockResolvedValue(true)
        const announce = vi.fn()
        const value = '\n<&/textarea><script>bad()</script>\nhttps://example.test/?a=1&b=2'
        render(<ReceiveTextOutcomeView state={outcome(value)} onDone={vi.fn()} onRetryFetch={vi.fn()}
            onCopy={copy} onAnnounce={announce}/> )
        const field = screen.getByLabelText('Received text') as HTMLTextAreaElement
        expect(field.readOnly).toBe(true)
        expect(field.value).toBe(value)
        expect(document.querySelector('script')).toBeNull()
        expect(document.querySelector('a')).toBeNull()
        expect(copy).not.toHaveBeenCalled()
        await act(async () => { fireEvent.click(screen.getByRole('button', {name: 'Copy text'})); await Promise.resolve() })
        expect(copy).toHaveBeenCalledExactlyOnceWith(sessionId)
        expect(announce).toHaveBeenCalledExactlyOnceWith(sessionId, 'Copied')
    })

    it('hides a partial value and Copy action on fetch failure and failed receive', () => {
        const {rerender} = render(<ReceiveTextOutcomeView state={{...outcome(null), fetchError: true}} onDone={vi.fn()}
            onRetryFetch={vi.fn()} onCopy={vi.fn()} onAnnounce={vi.fn()}/> )
        expect(screen.queryByRole('button', {name: 'Copy text'})).toBeNull()
        expect(screen.queryByLabelText('Received text')).toBeNull()
        expect(screen.getByRole('button', {name: 'Try again'})).toBeTruthy()
        rerender(<ReceiveTextOutcomeView state={{...outcome(null), kind: 'incomplete'}} onDone={vi.fn()}
            onRetryFetch={vi.fn()} onCopy={vi.fn()} onAnnounce={vi.fn()}/> )
        expect(screen.getByRole('heading', {name: 'Text not received'})).toBeTruthy()
        expect(screen.queryByRole('button', {name: 'Copy text'})).toBeNull()
    })

    it('keeps retry focus during a pending read and moves it to the named outcome on success', () => {
        const retry = vi.fn()
        const props = {onDone: vi.fn(), onRetryFetch: retry, onCopy: vi.fn(), onAnnounce: vi.fn()}
        const {rerender} = render(<ReceiveTextOutcomeView state={{...outcome(null), fetchError: true}} {...props}/> )
        const button = screen.getByRole('button', {name: 'Try again'})
        screen.getByText('The received text could not be shown. Try again.').focus()
        fireEvent.click(button)
        expect(document.activeElement).toBe(button)
        expect(retry).toHaveBeenCalledOnce()
        rerender(<ReceiveTextOutcomeView state={{...outcome(null), fetchPending: true}} {...props}/> )
        const pending = screen.getByRole('button', {name: 'Loading received text…'})
        expect(document.activeElement).toBe(pending)
        expect(pending.getAttribute('aria-disabled')).toBe('true')
        expect(screen.queryByText('The received text could not be shown. Try again.')).toBeNull()
        fireEvent.click(pending)
        expect(retry).toHaveBeenCalledOnce()
        rerender(<ReceiveTextOutcomeView state={outcome('recovered value')} {...props}/> )
        const namedOutcome = document.querySelector<HTMLElement>('[data-focus-target="receive-text-outcome"]')!
        expect(document.activeElement).toBe(namedOutcome)
        expect(namedOutcome.getAttribute('aria-labelledby')).toBe(screen.getByRole('heading', {name: 'Text received'}).id)
    })

    it('keeps Copy and Done focused while pending and blocks duplicate activation', async () => {
        const copy = vi.fn().mockResolvedValue(true)
        const done = vi.fn()
        const props = {onDone: done, onRetryFetch: vi.fn(), onCopy: copy, onAnnounce: vi.fn()}
        const {rerender} = render(<ReceiveTextOutcomeView state={outcome('value')} {...props}/> )
        const copyButton = screen.getByRole('button', {name: 'Copy text'})
        copyButton.focus()
        rerender(<ReceiveTextOutcomeView state={{...outcome('value'), copying: true}} {...props}/> )
        expect(document.activeElement).toBe(copyButton)
        expect(copyButton.getAttribute('aria-disabled')).toBe('true')
        fireEvent.click(copyButton)
        expect(copy).not.toHaveBeenCalled()
        const doneButton = screen.getByRole('button', {name: 'Done'})
        doneButton.focus()
        rerender(<ReceiveTextOutcomeView state={{...outcome('value'), donePending: true}} {...props}/> )
        expect(document.activeElement).toBe(doneButton)
        expect(doneButton.getAttribute('aria-disabled')).toBe('true')
        fireEvent.click(doneButton)
        expect(done).not.toHaveBeenCalled()
    })

    it('describes waiting beacon warnings and recovery on its heading', () => {
        render(<ReceiveTextWaitingView state={{...waiting, recovered: true,
            metadata: {...waiting.metadata, warnings: [{code: 'beacon_warning', message: 'Discovery unavailable'}]}}}
            onCancel={vi.fn()} onRefresh={vi.fn()} onAnnounce={vi.fn()} onCopyFailed={vi.fn()}/> )
        const heading = screen.getByRole('heading', {name: 'Waiting for text'})
        const descriptions = heading.getAttribute('aria-describedby')!.split(' ').map(id => document.getElementById(id)?.textContent)
        expect(descriptions).toEqual(['Discovery unavailable', 'New code ready. The old code no longer works.'])
    })

    it('makes a failed Done the one focusable command error while keeping retry available', () => {
        const error = {code: 'cleanup_unconfirmed' as const, message: 'Couldn’t confirm it stopped'}
        const {rerender} = render(<ReceiveTextOutcomeView state={{...outcome(null), fetchError: true,
            commandError: error}} onDone={vi.fn()} onRetryFetch={vi.fn()} onCopy={vi.fn()} onAnnounce={vi.fn()}/> )
        expect(document.querySelectorAll('[data-focus-target="command-error"]')).toHaveLength(1)
        expect(document.querySelector('[data-focus-target="command-error"]')?.textContent).toBe(error.message)
        expect(screen.getByRole('button', {name: 'Try again'})).toBeTruthy()
        rerender(<ReceiveTextOutcomeView state={{...outcome('value'), copyError: true, commandError: error}}
            onDone={vi.fn()} onRetryFetch={vi.fn()} onCopy={vi.fn()} onAnnounce={vi.fn()}/> )
        expect(document.querySelectorAll('[data-focus-target="command-error"]')).toHaveLength(1)
        expect(document.querySelector('[data-focus-target="command-error"]')?.textContent).toBe(error.message)
    })
})
