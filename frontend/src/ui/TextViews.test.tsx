import {fireEvent, render, screen, waitFor} from '@testing-library/react'
import {expect, it, vi} from 'vitest'
import {TextComposer, TextStagedView, TextTransferringView} from './TextViews'
import {StagePendingCard} from './StagePendingCard'
import type {TextStagedTransferState, TextTransferringTransferState} from '../transfer/state'
import {publicError} from '../transfer/errors'

const mocks = vi.hoisted(() => ({copy: vi.fn()}))
vi.mock('../../wailsjs/go/main/App', () => ({CopyToClipboard: mocks.copy, ListConnectionOptions: vi.fn()}))

const id = '11111111111111111111111111111111'
const url = `http://192.168.1.5:45678/text/${id}`
const metadata = {sessionId: id, byteLength: 7, url,
    qrBase64: 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=',
    warnings: []}
const staged: TextStagedTransferState = {phase: 'text-staged', session: {sessionId: id, lastSeq: 0},
    metadata, cancelPending: false, commandError: null}

it('gives the text composer a labelled multiline input and explicit actions', () => {
    const edit = vi.fn(); const send = vi.fn(); const cancel = vi.fn()
    render(<TextComposer text="hello" error={null} onEdit={edit} onSend={send} onCancel={cancel}/>)
    expect((screen.getByRole('textbox', {name: 'Text to send'}) as HTMLTextAreaElement).value).toBe('hello')
    fireEvent.change(screen.getByRole('textbox', {name: 'Text to send'}), {target: {value: 'hello\r\nworld'}})
    expect(edit).toHaveBeenCalledWith('hello\nworld')
    fireEvent.click(screen.getByRole('button', {name: 'Send'}))
    fireEvent.click(screen.getByRole('button', {name: 'Cancel'}))
    expect(send).toHaveBeenCalledTimes(1)
    expect(cancel).toHaveBeenCalledTimes(1)
})

it('shows metadata only while waiting, keeps the QR inert, and copies the URL through Go', async () => {
    const announce = vi.fn(); const failed = vi.fn()
    mocks.copy.mockReset().mockResolvedValue(undefined)
    const view = render(<TextStagedView state={staged} onCancel={vi.fn()} onAnnounce={announce}
        onCopyFailed={failed} onRefreshConnection={vi.fn()}/>)
    expect(screen.getByRole('heading', {name: 'Text ready to send'})).toBeTruthy()
    expect((screen.getByRole('textbox', {name: 'Text link'}) as HTMLInputElement).value).toBe(url)
    expect(view.container.querySelector('img')?.getAttribute('draggable')).toBe('false')
    expect(view.container.querySelector('a')).toBeNull()
    expect(view.container.textContent).not.toContain('secret payload')
    fireEvent.click(screen.getByRole('button', {name: 'Copy Link'}))
    await waitFor(() => expect(mocks.copy).toHaveBeenCalledExactlyOnceWith(url))
    expect(announce).toHaveBeenCalledWith(id, 'Copied')
    expect(failed).not.toHaveBeenCalled()
})

it('presents sending as indeterminate text, without payload byte claims', () => {
    const state: TextTransferringTransferState = {phase: 'text-transferring', session: {sessionId: id, lastSeq: 1},
        metadata, cancelPending: false, commandError: null}
    render(<TextTransferringView state={state} onCancel={vi.fn()}/>)
    expect(screen.getByRole('heading', {name: 'Sending text'})).toBeTruthy()
    expect(screen.queryByRole('progressbar')).toBeNull()
})

it('presents a failed text Cancel beside Sending with a focus target', () => {
    const state: TextTransferringTransferState = {phase: 'text-transferring', session: {sessionId: id, lastSeq: 1},
        metadata, cancelPending: false, commandError: publicError('transfer_failed')}
    const view = render(<TextTransferringView state={state} onCancel={vi.fn()}/> )
    expect(view.container.querySelector('[data-focus-target="command-error"]')?.textContent).toBe(state.commandError?.message)
})

it('names text preparation without describing a file', () => {
    render(<StagePendingCard state={{phase: 'pending', generation: 1, itemKind: 'text', cancelPending: false}}
        onCancel={vi.fn()}/>)
    expect(screen.getByRole('heading', {name: 'Preparing your text…'})).toBeTruthy()
    expect(screen.queryByText('File')).toBeNull()
})
