import type {} from '@vitest/browser-playwright'
import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {cdp, page} from 'vitest/browser'
import {afterEach, expect, it, vi} from 'vitest'
import type {ReceiveTextOutcomeTransferState, ReceiveTextWaitingTransferState} from '../src/transfer/state'
import {ReceiveTextOutcomeView, ReceiveTextWaitingView} from '../src/ui/ReceiveTextViews'
import '../src/style.css'

vi.mock('../wailsjs/go/main/App', () => ({CopyToClipboard: vi.fn(), ListConnectionOptions: vi.fn()}))

const sessionId = '11111111111111111111111111111111'
const waiting: ReceiveTextWaitingTransferState = {phase: 'receive-text-waiting', session: {sessionId, lastSeq: 0},
    metadata: {sessionId, url: 'http://192.0.2.1:34567/receive-text/22222222222222222222222222222222',
        qrBase64: 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=',
        warnings: []}, cancelPending: false, commandError: null}
const outcome: ReceiveTextOutcomeTransferState = {phase: 'receive-text-outcome', session: {sessionId, lastSeq: 2},
    kind: 'complete', value: 'https://example.test/' + 'a'.repeat(100), fetchError: false, copyError: false, copying: false}

afterEach(async () => {
    cleanup()
    await cdp().send('Emulation.setEmulatedMedia', {features: []})
    await page.viewport(1024, 800)
})

it('keeps waiting and held text inside 320 CSS pixels with 44px actions', async () => {
    await page.viewport(320, 900)
    for (const view of [
        <ReceiveTextWaitingView state={waiting} onCancel={vi.fn()} onRefresh={vi.fn()} onAnnounce={vi.fn()} onCopyFailed={vi.fn()}/> ,
        <ReceiveTextOutcomeView state={outcome} onDone={vi.fn()} onRetryFetch={vi.fn()} onCopy={vi.fn()} onAnnounce={vi.fn()}/> ,
    ]) {
        const {container} = render(<div className="fd-app">{view}</div>)
        expect(document.documentElement.scrollWidth).toBeLessThanOrEqual(320)
        for (const target of container.querySelectorAll<HTMLElement>('.fd-target')) {
            expect(target.getBoundingClientRect().height).toBeGreaterThanOrEqual(44)
        }
        cleanup()
    }
})

it('shows a real forced-colors outline on text actions and a focusable readonly field', async () => {
    await cdp().send('Emulation.setEmulatedMedia', {features: [{name: 'forced-colors', value: 'active'}]})
    render(<div className="fd-app"><ReceiveTextOutcomeView state={outcome} onDone={vi.fn()} onRetryFetch={vi.fn()}
        onCopy={vi.fn()} onAnnounce={vi.fn()}/></div>)
    const copy = screen.getByRole('button', {name: 'Copy text'})
    copy.focus()
    expect(getComputedStyle(copy).outlineStyle).toBe('solid')
    const field = screen.getByLabelText('Received text') as HTMLTextAreaElement
    field.focus()
    expect(document.activeElement).toBe(field)
    expect(field.readOnly).toBe(true)
})

it('keeps browser focus on retry while reading and moves it to the named outcome on success', async () => {
    const props = {onDone: vi.fn(), onRetryFetch: vi.fn(), onCopy: vi.fn(), onAnnounce: vi.fn()}
    const {rerender} = render(<div className="fd-app"><ReceiveTextOutcomeView state={{...outcome, value: null, fetchError: true}}
        {...props}/></div>)
    const retry = screen.getByRole('button', {name: 'Try again'})
    const error = screen.getByText('The received text could not be shown. Try again.') as HTMLElement
    error.focus()
    fireEvent.click(retry)
    expect(document.activeElement).toBe(retry)
    rerender(<div className="fd-app"><ReceiveTextOutcomeView state={{...outcome, value: null, fetchPending: true}}
        {...props}/></div>)
    const pending = screen.getByRole('button', {name: 'Loading received text…'})
    expect(document.activeElement).toBe(pending)
    expect(pending.getAttribute('aria-disabled')).toBe('true')
    rerender(<div className="fd-app"><ReceiveTextOutcomeView state={{...outcome, value: 'received on retry'}}
        {...props}/></div>)
    const destination = document.querySelector<HTMLElement>('[data-focus-target="receive-text-outcome"]')!
    expect(document.activeElement).toBe(destination)
    expect(destination.getAttribute('aria-labelledby')).toBe(screen.getByRole('heading', {name: 'Text received'}).id)
})

it('keeps browser focus on Copy text and Done during pending commands', async () => {
    const props = {onDone: vi.fn(), onRetryFetch: vi.fn(), onCopy: vi.fn(), onAnnounce: vi.fn()}
    const {rerender} = render(<div className="fd-app"><ReceiveTextOutcomeView state={outcome} {...props}/></div>)
    const copyButton = screen.getByRole('button', {name: 'Copy text'})
    copyButton.focus()
    rerender(<div className="fd-app"><ReceiveTextOutcomeView state={{...outcome, copying: true}} {...props}/></div>)
    expect(document.activeElement).toBe(copyButton)
    expect(copyButton.getAttribute('aria-disabled')).toBe('true')
    fireEvent.click(copyButton)
    expect(props.onCopy).not.toHaveBeenCalled()
    const doneButton = screen.getByRole('button', {name: 'Done'})
    doneButton.focus()
    rerender(<div className="fd-app"><ReceiveTextOutcomeView state={{...outcome, donePending: true}} {...props}/></div>)
    expect(document.activeElement).toBe(doneButton)
    expect(doneButton.getAttribute('aria-disabled')).toBe('true')
    fireEvent.click(doneButton)
    expect(props.onDone).not.toHaveBeenCalled()
})
