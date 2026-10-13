import {act, cleanup, fireEvent, render, screen} from '@testing-library/react'
import {afterEach, expect, it, vi} from 'vitest'

const mocks = vi.hoisted(() => ({list: vi.fn()}))
vi.mock('../../wailsjs/go/main/App', () => ({ListConnectionOptions: mocks.list}))
import {ConnectionRecoveryPanel} from './ConnectionRecoveryPanel'

afterEach(() => { cleanup(); mocks.list.mockReset() })
const oldID = '0123456789abcdef0123456789abcdef'
const choiceID = '11111111111111111111111111111111'
const options = (sessionId = oldID, choices = [{id: choiceID, name: 'Wi-Fi', address: '192.168.1.7'}]) =>
    ({sessionId, currentAddress: '192.168.1.5', choices})

it('loads on opening, preserves a user choice, and refuses a vanished choice after reload', async () => {
    mocks.list.mockResolvedValueOnce(options()).mockResolvedValueOnce(options(oldID, []))
    const refresh = vi.fn().mockResolvedValue(undefined)
    const {rerender} = render(<ConnectionRecoveryPanel active={false} sessionId={oldID} currentAddress="192.168.1.5" onRefresh={refresh}/> )
    expect(mocks.list).not.toHaveBeenCalled()
    rerender(<ConnectionRecoveryPanel active sessionId={oldID} currentAddress="192.168.1.5" onRefresh={refresh}/> )
    await screen.findByText('Current address:')
    fireEvent.change(screen.getByLabelText('Local network'), {target: {value: choiceID}})
    fireEvent.click(screen.getByRole('button', {name: 'Reload networks'}))
    await screen.findByText('That network is no longer available. Choose Automatic or another network.')
    expect((screen.getByLabelText('Local network') as HTMLSelectElement).value).toBe(choiceID)
    expect((screen.getByRole('button', {name: 'Refresh connection'}) as HTMLButtonElement).disabled).toBe(true)
    fireEvent.change(screen.getByLabelText('Local network'), {target: {value: ''}})
    expect((screen.getByRole('button', {name: 'Refresh connection'}) as HTMLButtonElement).disabled).toBe(true)
    expect(refresh).not.toHaveBeenCalled()
})

it('ignores a late list result after the session changes', async () => {
    let resolve!: (value: unknown) => void
    mocks.list.mockReturnValueOnce(new Promise(value => { resolve = value })).mockResolvedValueOnce({
        ...options('22222222222222222222222222222222'), currentAddress: '10.0.0.5',
    })
    const refresh = vi.fn()
    const {rerender} = render(<ConnectionRecoveryPanel active sessionId={oldID} currentAddress="192.168.1.5" onRefresh={refresh}/> )
    const nextID = '22222222222222222222222222222222'
    rerender(<ConnectionRecoveryPanel active sessionId={nextID} currentAddress="192.168.1.5" onRefresh={refresh}/> )
    await screen.findByText('Current address:')
    await act(async () => resolve(options()))
    expect((screen.getByLabelText('Local network') as HTMLSelectElement).value).toBe('')
    expect(screen.getByText('10.0.0.5')).toBeTruthy()
})

it('reenables a selected endpoint when it returns and keeps the advertised address through list failure', async () => {
    mocks.list.mockResolvedValueOnce(options()).mockResolvedValueOnce(options(oldID, []))
        .mockResolvedValueOnce(options()).mockRejectedValueOnce(new Error('hidden adapter detail'))
    const refresh = vi.fn().mockResolvedValue(undefined)
    render(<ConnectionRecoveryPanel active sessionId={oldID} currentAddress="192.168.1.5" onRefresh={refresh}/> )
    await screen.findByRole('option', {name: /Wi-Fi/})
    fireEvent.change(screen.getByLabelText('Local network'), {target: {value: choiceID}})
    fireEvent.click(screen.getByRole('button', {name: 'Reload networks'}))
    await screen.findByText('That network is no longer available. Choose Automatic or another network.')
    fireEvent.click(screen.getByRole('button', {name: 'Reload networks'}))
    await screen.findByRole('option', {name: /Wi-Fi/})
    expect((screen.getByRole('button', {name: 'Refresh connection'}) as HTMLButtonElement).disabled).toBe(false)
    fireEvent.click(screen.getByRole('button', {name: 'Reload networks'}))
    await screen.findByText('Couldn’t load local networks. Try again.')
    expect(screen.getByText('192.168.1.5')).toBeTruthy()
})
