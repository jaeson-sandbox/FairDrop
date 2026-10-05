import {act, cleanup, fireEvent, render, screen, waitFor} from '@testing-library/react'
import {afterEach, beforeEach, expect, it, vi} from 'vitest'
import App from './App'

const mocks = vi.hoisted(() => ({
    stage: vi.fn(),
    cancel: vi.fn(),
    selectFile: vi.fn(),
    selectDirectory: vi.fn(),
    copy: vi.fn(),
    onFileDrop: vi.fn(),
    onFileDropOff: vi.fn(),
    eventsOn: vi.fn(),
}))
vi.mock('../wailsjs/go/main/App', () => ({
    StageTransfer: mocks.stage, CancelTransfer: mocks.cancel,
    SelectFile: mocks.selectFile, SelectDirectory: mocks.selectDirectory,
    CopyToClipboard: mocks.copy,
}))
vi.mock('../wailsjs/runtime/runtime', () => ({
    OnFileDrop: mocks.onFileDrop, OnFileDropOff: mocks.onFileDropOff,
    EventsOn: mocks.eventsOn,
}))

const firstSession = '0123456789abcdef0123456789abcdef'
const nextSession = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
const firstURL = 'http://192.0.2.1:34123/download/fedcba9876543210fedcba9876543210'
const nextURL = 'http://192.0.2.1:34124/download/11111111111111111111111111111111'
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
    mocks.stage.mockResolvedValueOnce({sessionId: firstSession, name: 'report.pdf', size: 100,
        isDir: false, url: firstURL, qrBase64: qr, warnings: []})
    mocks.stage.mockResolvedValueOnce({sessionId: nextSession, name: 'report.pdf', size: 100,
        isDir: false, url: nextURL, qrBase64: nextQR, warnings: []})
})
afterEach(cleanup)

function emit(name: string, payload: unknown) {
    const callback = events.get(name)
    expect(callback).toBeTruthy()
    callback!(payload)
}

it('clicks Send Again through the real controller and presents the newly staged code without duplicate speech', async () => {
    render(<App/>)
    const path = '/Users/example/report.pdf'
    await act(async () => { mocks.onFileDrop.mock.calls.at(-1)![0](0, 0, [path]) })
    await waitFor(() => expect(mocks.stage).toHaveBeenCalledWith(path))
    expect(screen.getByRole('heading', {name: 'Ready to send'})).toBeTruthy()
    act(() => {
        emit('transfer-started', {sessionId: firstSession, seq: 1})
        emit('transfer-complete', {sessionId: firstSession, seq: 2, progress: {
            bytesSent: 100, totalBytes: 100, totalKnown: true, percent: 100, speedBytesPerSec: 10,
        }})
    })
    const doneCard = document.querySelector<HTMLElement>('.fd-outcome')!
    expect(document.activeElement).toBe(doneCard)
    const status = screen.getByRole('status')
    expect(status.textContent).toBe('')
    fireEvent.click(screen.getByRole('button', {name: 'Send Again'}))
    await waitFor(() => expect(mocks.cancel).toHaveBeenCalledTimes(1))
    expect(mocks.stage).toHaveBeenCalledTimes(1)
    act(() => emit('transfer-reset', {sessionId: firstSession, seq: 3}))
    expect(document.querySelector('.fd-outcome')).toBe(doneCard)
    expect(document.activeElement).toBe(doneCard)
    expect(status.textContent).toBe('')
    await waitFor(() => expect(mocks.stage).toHaveBeenCalledTimes(2))
    expect(mocks.stage).toHaveBeenLastCalledWith(path)
    expect(screen.getByRole('heading', {name: 'Ready to send'})).toBeTruthy()
    await waitFor(() => expect(document.activeElement).toBe(document.querySelector('[data-focus-target="staged-heading"]')))
    expect(screen.getByRole('img', {name: 'Download QR code for report.pdf'}).getAttribute('src')).toBe(`data:image/png;base64,${nextQR}`)
    fireEvent.click(screen.getByRole('button', {name: 'Show Link'}))
    const link = document.querySelector<HTMLInputElement>('.fd-url')
    expect(link?.value).toBe(nextURL)
    expect(link?.value).not.toBe(firstURL)
    expect(status.textContent).toBe('')
})

it('removes Send Again after user Done even when terminal Cancel rejects without reset', async () => {
    mocks.cancel.mockRejectedValueOnce(new Error('release failed'))
    render(<App/>)
    await act(async () => { mocks.onFileDrop.mock.calls.at(-1)![0](0, 0, ['/Users/example/report.pdf']) })
    act(() => {
        emit('transfer-started', {sessionId: firstSession, seq: 1})
        emit('transfer-complete', {sessionId: firstSession, seq: 2, progress: {
            bytesSent: 100, totalBytes: 100, totalKnown: true, percent: 100, speedBytesPerSec: 10,
        }})
    })
    expect(screen.getByRole('button', {name: 'Send Again'})).toBeTruthy()
    fireEvent.click(screen.getByRole('button', {name: 'Done'}))
    await waitFor(() => expect(mocks.cancel).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(screen.queryByRole('button', {name: 'Send Again'})).toBeNull())
    expect(screen.getByRole('button', {name: 'Send Another'})).toBeTruthy()
    expect(mocks.stage).toHaveBeenCalledTimes(1)
})
