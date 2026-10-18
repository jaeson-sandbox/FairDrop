import {act} from 'react'
import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {afterEach, beforeEach, expect, it, vi} from 'vitest'
import App from './App'

const mocks = vi.hoisted(() => ({start: vi.fn(), cancel: vi.fn(), events: vi.fn(), fileDrop: vi.fn(), fileDropOff: vi.fn()}))
vi.mock('../wailsjs/go/main/App', () => ({
    StartReceiveText: mocks.start, CancelTransfer: mocks.cancel,
    GetReceivedText: vi.fn(), CopyReceivedText: vi.fn(), CopyToClipboard: vi.fn(),
    ListConnectionOptions: vi.fn().mockResolvedValue([]), RefreshConnection: vi.fn(),
    StageTransfer: vi.fn(), StageTransfers: vi.fn(), StageText: vi.fn(), StartReceive: vi.fn(),
    SelectFile: vi.fn(), SelectFiles: vi.fn(), SelectDirectory: vi.fn(), SelectReceiveFolder: vi.fn(), ShowReceivedFolder: vi.fn(),
}))
vi.mock('../wailsjs/runtime/runtime', () => ({
    EventsOn: mocks.events, OnFileDrop: mocks.fileDrop, OnFileDropOff: mocks.fileDropOff,
}))

const id = '11111111111111111111111111111111'
const url = `http://192.168.1.5:45678/receive-text/${id}`
const qrBase64 = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII='

beforeEach(() => {
    for (const mock of Object.values(mocks)) mock.mockReset()
    mocks.start.mockResolvedValue({sessionId: id, url, qrBase64, warnings: []})
    mocks.cancel.mockResolvedValue(undefined)
    mocks.events.mockReturnValue(() => undefined)
})
afterEach(cleanup)

it('opens the real Receive text controller from Idle and shows its waiting code', async () => {
    render(<App/>)
    await act(async () => { fireEvent.click(screen.getByRole('button', {name: 'Receive text'})); await Promise.resolve() })
    expect(mocks.start).toHaveBeenCalledOnce()
    expect(screen.getByRole('heading', {name: 'Waiting for text'})).toBeTruthy()
    expect(screen.getByRole('button', {name: 'Show Link'})).toBeTruthy()
    fireEvent.click(screen.getByRole('button', {name: 'Show Link'}))
    expect(screen.getByLabelText('Text receive link')).toHaveProperty('value', url)
})
