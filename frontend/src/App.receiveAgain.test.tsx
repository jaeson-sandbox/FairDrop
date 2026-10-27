import {act, cleanup, fireEvent, render, screen, waitFor} from '@testing-library/react'
import {afterEach, beforeEach, expect, it, vi} from 'vitest'
import App from './App'

const mocks = vi.hoisted(() => ({
    start: vi.fn(),
    receiveFolder: vi.fn(),
    cancel: vi.fn(),
    show: vi.fn(),
    selectFile: vi.fn(),
    selectDirectory: vi.fn(),
    copy: vi.fn(),
    onFileDrop: vi.fn(),
    onFileDropOff: vi.fn(),
    eventsOn: vi.fn(),
}))
vi.mock('../wailsjs/go/main/App', () => ({
    StartReceive: mocks.start, SelectReceiveFolder: mocks.receiveFolder, CancelTransfer: mocks.cancel,
    SelectFile: mocks.selectFile, SelectDirectory: mocks.selectDirectory,
    CopyToClipboard: mocks.copy, ShowReceivedFolder: mocks.show,
}))
vi.mock('../wailsjs/runtime/runtime', () => ({
    OnFileDrop: mocks.onFileDrop, OnFileDropOff: mocks.onFileDropOff,
    EventsOn: mocks.eventsOn,
}))

const firstSession = '0123456789abcdef0123456789abcdef'
const nextSession = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
const firstURL = 'http://192.0.2.1:34123/upload/fedcba9876543210fedcba9876543210'
const nextURL = 'http://192.0.2.1:34124/upload/11111111111111111111111111111111'
const qr = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII='
const nextQR = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGP4z8AAAAMBAQDJ/pLvAAAAAElFTkSuQmCC'
let events: Map<string, (...args: unknown[]) => void>

beforeEach(() => {
    events = new Map()
    for (const mock of Object.values(mocks)) mock.mockReset()
    mocks.eventsOn.mockImplementation((name: string, callback: (...args: unknown[]) => void) => {
        events.set(name, callback)
        return () => { events.delete(name) }
    })
    mocks.cancel.mockResolvedValue(undefined)
    mocks.show.mockResolvedValue(undefined)
    mocks.receiveFolder.mockResolvedValue('/Users/example/Photos')
    mocks.start.mockResolvedValueOnce({sessionId: firstSession, destination: 'Photos', url: firstURL, qrBase64: qr, warnings: []})
    mocks.start.mockResolvedValueOnce({sessionId: nextSession, destination: 'Photos', url: nextURL, qrBase64: nextQR, warnings: []})
})
afterEach(cleanup)

function emit(name: string, payload: unknown) {
    const callback = events.get(name)
    expect(callback).toBeTruthy()
    callback!(payload)
}

it('clicks Receive Again through the real controller, blocks overlapping actions and focuses the fresh code without duplicate speech', async () => {
    render(<App/>)
    fireEvent.click(screen.getByRole('button', {name: 'Receive'}))
    await waitFor(() => expect(mocks.start).toHaveBeenCalledWith('/Users/example/Photos'))
    await waitFor(() => expect(document.querySelector('[data-phase-view="receive-waiting"]')).toBeTruthy())
    const progress = {bytesSent: 100, totalBytes: 100, totalKnown: true, percent: 100, speedBytesPerSec: 10}
    act(() => {
        emit('transfer-started', {sessionId: firstSession, seq: 1})
        emit('transfer-complete', {sessionId: firstSession, seq: 2, progress,
            receive: {result: 'complete', filesSaved: 1, subfolderExists: true, markingWarning: false}})
    })
    await waitFor(() => expect(document.activeElement).toBe(document.querySelector('[data-focus-target="receive-outcome"]')))
    const status = screen.getByRole('status')
    expect(status.textContent).toBe('')
    fireEvent.click(screen.getByRole('button', {name: 'Receive Again'}))
    await waitFor(() => expect(mocks.cancel).toHaveBeenCalledTimes(1))
    for (const label of ['Receive Again', 'Show in Folder', 'Done']) {
        expect((screen.getByRole('button', {name: label}) as HTMLButtonElement).disabled).toBe(true)
    }
    expect(mocks.start).toHaveBeenCalledTimes(1)
    act(() => emit('transfer-reset', {sessionId: firstSession, seq: 3}))
    await waitFor(() => expect(mocks.start).toHaveBeenCalledTimes(2))
    expect(mocks.start).toHaveBeenLastCalledWith('/Users/example/Photos')
    expect(mocks.receiveFolder).toHaveBeenCalledTimes(1)
    await waitFor(() => expect(document.activeElement).toBe(document.querySelector('[data-focus-target="receive-waiting-heading"]')))
    expect(screen.getByRole('img').getAttribute('src')).toBe(`data:image/png;base64,${nextQR}`)
    fireEvent.click(screen.getByRole('button', {name: 'Show Link'}))
    expect(document.querySelector<HTMLInputElement>('.fd-url')?.value).toBe(nextURL)
    expect(status.textContent).toBe('')
})

it('focuses a truthful retirement failure and keeps Receive Again and Show in Folder usable', async () => {
    mocks.cancel.mockRejectedValueOnce(new Error('/private/adapter-detail'))
    render(<App/>)
    fireEvent.click(screen.getByRole('button', {name: 'Receive'}))
    await waitFor(() => expect(document.querySelector('[data-phase-view="receive-waiting"]')).toBeTruthy())
    act(() => emit('transfer-started', {sessionId: firstSession, seq: 1}))
    act(() => emit('transfer-complete', {sessionId: firstSession, seq: 2,
        progress: {bytesSent: 100, totalBytes: 100, totalKnown: true, percent: 100, speedBytesPerSec: 10},
        receive: {result: 'complete', filesSaved: 1, subfolderExists: true, markingWarning: false}}))
    fireEvent.click(screen.getByRole('button', {name: 'Receive Again'}))
    const error = await screen.findByText('FairDrop couldn’t confirm the previous session closed. Try again or choose Done.')
    await waitFor(() => expect(document.activeElement).toBe(error))
    expect((screen.getByRole('button', {name: 'Receive Again'}) as HTMLButtonElement).disabled).toBe(false)
    expect((screen.getByRole('button', {name: 'Show in Folder'}) as HTMLButtonElement).disabled).toBe(false)
    expect(screen.queryByText('FairDrop couldn’t open that folder. It may have been moved or renamed.')).toBeNull()
    expect(mocks.start).toHaveBeenCalledTimes(1)
    expect(document.body.textContent).not.toContain('/private/')
    fireEvent.click(screen.getByRole('button', {name: 'Show in Folder'}))
    await waitFor(() => expect(mocks.show).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(document.activeElement).toBe(screen.getByRole('button', {name: 'Show in Folder'})))
    expect(screen.queryByText('FairDrop couldn’t confirm the previous session closed. Try again or choose Done.')).toBeNull()
})


it('locks all outcome controls during Show and restores its initiating focus after success', async () => {
    let finish!: () => void
    mocks.show.mockReturnValueOnce(new Promise<void>((resolve) => { finish = resolve }))
    render(<App/>)
    fireEvent.click(screen.getByRole('button', {name: 'Receive'}))
    await waitFor(() => expect(document.querySelector('[data-phase-view="receive-waiting"]')).toBeTruthy())
    act(() => emit('transfer-started', {sessionId: firstSession, seq: 1}))
    act(() => emit('transfer-complete', {sessionId: firstSession, seq: 2,
        progress: {bytesSent: 100, totalBytes: 100, totalKnown: true, percent: 100, speedBytesPerSec: 10},
        receive: {result: 'complete', filesSaved: 1, subfolderExists: true, markingWarning: false}}))
    const show = screen.getByRole('button', {name: 'Show in Folder'})
    expect(document.activeElement).not.toBe(show)
    fireEvent.click(show)
    expect(mocks.show).toHaveBeenCalledTimes(1)
    for (const name of ['Receive Again', 'Show in Folder', 'Done']) {
        expect((screen.getByRole('button', {name}) as HTMLButtonElement).disabled).toBe(true)
        fireEvent.click(screen.getByRole('button', {name}))
    }
    expect(mocks.cancel).not.toHaveBeenCalled()
    expect(mocks.start).toHaveBeenCalledTimes(1)
    document.body.tabIndex = -1
    document.body.focus()
    document.body.removeAttribute('tabindex')
    await act(async () => { finish() })
    expect(document.activeElement).toBe(show)
    expect(show.hasAttribute('data-focus-return')).toBe(true)
    for (const name of ['Receive Again', 'Show in Folder', 'Done']) expect((screen.getByRole('button', {name}) as HTMLButtonElement).disabled).toBe(false)
    fireEvent.click(show)
    await waitFor(() => expect(mocks.show).toHaveBeenCalledTimes(2))
})


it('releases the App action lock on a missing reset without rearming on a late reset', async () => {
    render(<App/>)
    fireEvent.click(screen.getByRole('button', {name: 'Receive'}))
    await waitFor(() => expect(document.querySelector('[data-phase-view="receive-waiting"]')).toBeTruthy())
    act(() => emit('transfer-started', {sessionId: firstSession, seq: 1}))
    act(() => emit('transfer-complete', {sessionId: firstSession, seq: 2,
        progress: {bytesSent: 100, totalBytes: 100, totalKnown: true, percent: 100, speedBytesPerSec: 10},
        receive: {result: 'complete', filesSaved: 1, subfolderExists: true, markingWarning: false}}))
    vi.useFakeTimers()
    try {
        fireEvent.click(screen.getByRole('button', {name: 'Receive Again'}))
        await act(async () => { await Promise.resolve() })
        expect((screen.getByRole('button', {name: 'Receive Again'}) as HTMLButtonElement).disabled).toBe(true)
        await act(async () => { await vi.advanceTimersByTimeAsync(15_000) })
        const error = screen.queryByText('FairDrop couldn’t confirm the previous session closed. Try again or choose Done.')
        expect(error).toBeTruthy()
        expect(document.activeElement).toBe(error)
        for (const name of ['Receive Again', 'Show in Folder', 'Done']) expect((screen.getByRole('button', {name}) as HTMLButtonElement).disabled).toBe(false)
        expect(mocks.start).toHaveBeenCalledTimes(1)
        act(() => emit('transfer-reset', {sessionId: firstSession, seq: 3}))
        await act(async () => { await Promise.resolve() })
        expect(mocks.start).toHaveBeenCalledTimes(1)
    } finally {
        vi.useRealTimers()
    }
})
