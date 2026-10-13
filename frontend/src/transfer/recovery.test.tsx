import {act, renderHook} from '@testing-library/react'
import {beforeEach, expect, it, vi} from 'vitest'

const mocks = vi.hoisted(() => ({stage: vi.fn(), startReceive: vi.fn(), selectReceiveFolder: vi.fn(), refresh: vi.fn(), cancel: vi.fn(), events: vi.fn()}))
vi.mock('../../wailsjs/go/main/App', () => ({
    StageTransfer: mocks.stage, RefreshConnection: mocks.refresh, CancelTransfer: mocks.cancel,
    StageTransfers: vi.fn(), SelectDirectory: vi.fn(), SelectFile: vi.fn(), SelectFiles: vi.fn(),
    SelectReceiveFolder: mocks.selectReceiveFolder, StartReceive: mocks.startReceive, ShowReceivedFolder: vi.fn(),
}))
vi.mock('../../wailsjs/runtime/runtime', () => ({EventsOn: mocks.events}))
import {useTransfer} from './useTransfer'
import {parseConnectionOptions, parseConnectionRefresh} from './validation'
import {transferReducer, type TransferState} from './state'
import {publicError} from './errors'
import {routeTransition} from '../ui/announce'

const oldID = '0123456789abcdef0123456789abcdef'
const newID = '11111111111111111111111111111111'
const thirdID = '22222222222222222222222222222222'
const png = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII='
const stage = (sessionId = oldID) => ({sessionId, name: 'report.pdf', size: 100, isDir: false,
    url: `http://192.168.1.5:45678/download/${sessionId}`, qrBase64: png, warnings: []})
const receive = (sessionId = oldID) => ({sessionId, destination: 'Photos',
    url: `http://192.168.1.5:45678/upload/${sessionId}`, qrBase64: png, warnings: []})

let listeners: Map<string, (...args: unknown[]) => void>
function deferred() {
    let resolve!: (value: unknown) => void
    let reject!: (reason: unknown) => void
    const promise = new Promise<unknown>((yes, no) => { resolve = yes; reject = no })
    return {promise, resolve, reject}
}
function emit(name: string, value: unknown) { act(() => listeners.get(name)?.(value)) }
async function stagedHook() {
    const hook = renderHook(() => useTransfer())
    await act(async () => { await hook.result.current.stage('/tmp/report.pdf') })
    expect(hook.result.current.state.phase).toBe('staged')
    return hook
}

beforeEach(() => {
    listeners = new Map()
    mocks.events.mockReset().mockImplementation((name: string, callback: (...args: unknown[]) => void) => {
        listeners.set(name, callback); return () => listeners.delete(name)
    })
    mocks.stage.mockReset().mockResolvedValue(stage())
    mocks.startReceive.mockReset().mockResolvedValue(receive())
    mocks.selectReceiveFolder.mockReset().mockResolvedValue('/tmp/Photos')
    mocks.refresh.mockReset()
    mocks.cancel.mockReset().mockResolvedValue(undefined)
})

it('keeps the old code hidden through reset and applies early events to the new acknowledgment', async () => {
    const pending = deferred(); mocks.refresh.mockReturnValue(pending.promise)
    const hook = await stagedHook()
    let command!: Promise<void>
    act(() => { command = hook.result.current.refreshConnection!(oldID, '') })
    expect(hook.result.current.state.phase).toBe('recovering')
    emit('transfer-reset', {sessionId: oldID, seq: 1})
    expect(hook.result.current.state.phase).toBe('recovering')
    emit('transfer-started', {sessionId: newID, seq: 1})
    await act(async () => { pending.resolve({mode: 'send', send: stage(newID)}); await command })
    expect(hook.result.current.state.phase).toBe('transferring')
    if (hook.result.current.state.phase === 'transferring') expect(hook.result.current.state.session.sessionId).toBe(newID)
})

it('preserves a waiting code on preflight refusal and ignores duplicate refresh', async () => {
    const pending = deferred(); mocks.refresh.mockReturnValue(pending.promise)
    const hook = await stagedHook()
    let first!: Promise<void>
    act(() => { first = hook.result.current.refreshConnection!(oldID, 'missing') })
    await act(async () => { await hook.result.current.refreshConnection!(oldID, '') })
    expect(mocks.refresh).toHaveBeenCalledTimes(1)
    await act(async () => { pending.reject('{"code":"network_unavailable","message":"safe"}'); await first })
    expect(hook.result.current.state.phase).toBe('staged')
    if (hook.result.current.state.phase === 'staged') expect(hook.result.current.state.metadata.sessionId).toBe(oldID)
})

it.each([
    '{"code":"setup_failed","message":"safe","retired":true}',
    new Error('{"code":"setup_failed","message":"safe","retired":true}'),
])('removes the retired code when rejection is serialized as %s', async rejection => {
    mocks.refresh.mockRejectedValue(rejection)
    const hook = await stagedHook()
    await act(async () => { await hook.result.current.refreshConnection!(oldID, '') })
    expect(hook.result.current.state.phase).toBe('idle')
    if (hook.result.current.state.phase === 'idle') expect(hook.result.current.state.commandError?.code).toBe('setup_failed')
})

it('preserves a claim that starts while refresh is in flight', async () => {
    const pending = deferred(); mocks.refresh.mockReturnValue(pending.promise)
    const hook = await stagedHook()
    let command!: Promise<void>
    act(() => { command = hook.result.current.refreshConnection!(oldID, '') })
    emit('transfer-started', {sessionId: oldID, seq: 1})
    await act(async () => { pending.reject('{"code":"cancelled","message":"safe"}'); await command })
    expect(hook.result.current.state.phase).toBe('transferring')
})

it('invalidates an in-flight acknowledgment when Cancel wins', async () => {
    const pending = deferred(); mocks.refresh.mockReturnValue(pending.promise)
    const hook = await stagedHook()
    let command!: Promise<void>
    act(() => { command = hook.result.current.refreshConnection!(oldID, '') })
    await act(async () => { await hook.result.current.cancel() })
    expect(mocks.cancel).toHaveBeenCalledTimes(1)
    await act(async () => { pending.resolve({mode: 'send', send: stage(newID)}); await command })
    expect(hook.result.current.state.phase).toBe('idle')
})

it('cancels an undisplayed replacement when the controller unmounts', async () => {
    const pending = deferred(); mocks.refresh.mockReturnValue(pending.promise)
    const hook = await stagedHook()
    act(() => { void hook.result.current.refreshConnection!(oldID, '') })
    act(() => hook.unmount())
    await act(async () => { await Promise.resolve(); await Promise.resolve() })
    expect(mocks.cancel).toHaveBeenCalledTimes(1)
    pending.resolve({mode: 'send', send: stage(newID)})
})

it('coalesces same-tick recovery cancellation and clears remembered send selection', async () => {
    const pending = deferred(); mocks.refresh.mockReturnValue(pending.promise)
    const cancelPending = deferred(); mocks.cancel.mockReturnValue(cancelPending.promise)
    const hook = await stagedHook()
    act(() => { void hook.result.current.refreshConnection!(oldID, '') })
    let first!: Promise<void>; let second!: Promise<void>
    act(() => { first = hook.result.current.cancel(); second = hook.result.current.cancel() })
    await act(async () => { await Promise.resolve() })
    expect(mocks.cancel).toHaveBeenCalledTimes(1)
    await act(async () => { cancelPending.resolve(undefined); await Promise.all([first, second]) })
    expect(hook.result.current.canRetry).toBe(false)
    pending.resolve({mode: 'send', send: stage(newID)})
})

it('an old cancelled command cannot erase a newer recovery event buffer', async () => {
    const oldPending = deferred(); const nextPending = deferred()
    mocks.refresh.mockReturnValueOnce(oldPending.promise).mockReturnValueOnce(nextPending.promise)
    const hook = await stagedHook()
    let oldCommand!: Promise<void>
    act(() => { oldCommand = hook.result.current.refreshConnection!(oldID, '') })
    await act(async () => { await hook.result.current.cancel() })
    mocks.stage.mockResolvedValueOnce(stage(newID))
    await act(async () => { await hook.result.current.stage('/tmp/report.pdf') })
    expect(hook.result.current.state.phase).toBe('staged')
    let nextCommand!: Promise<void>
    act(() => { nextCommand = hook.result.current.refreshConnection!(newID, '') })
    emit('transfer-started', {sessionId: thirdID, seq: 1})
    await act(async () => { oldPending.resolve({mode: 'send', send: stage(newID)}); await oldCommand })
    await act(async () => { nextPending.resolve({mode: 'send', send: stage(thirdID)}); await nextCommand })
    expect(hook.result.current.state.phase).toBe('transferring')
    if (hook.result.current.state.phase === 'transferring') expect(hook.result.current.state.session.sessionId).toBe(thirdID)
})

it('fences a malformed replacement acknowledgment with Cancel', async () => {
    mocks.refresh.mockResolvedValue({mode: 'send', send: stage(newID), receive: {sessionId: newID}})
    const hook = await stagedHook()
    await act(async () => { await hook.result.current.refreshConnection!(oldID, '') })
    expect(mocks.cancel).toHaveBeenCalledTimes(1)
    expect(hook.result.current.state.phase).toBe('idle')
    if (hook.result.current.state.phase === 'idle') expect(hook.result.current.state.commandError?.code).toBe('setup_failed')
})

it('reports unconfirmed cleanup for malformed acknowledgment', async () => {
    mocks.refresh.mockResolvedValue({mode: 'receive', receive: {sessionId: newID}})
    mocks.cancel.mockRejectedValue(new Error('adapter detail'))
    const hook = await stagedHook()
    await act(async () => { await hook.result.current.refreshConnection!(oldID, '') })
    expect(hook.result.current.state.phase).toBe('idle')
    if (hook.result.current.state.phase === 'idle') expect(hook.result.current.state.commandError?.code).toBe('cleanup_unconfirmed')
})

it('bounds early replacement events and cancels rather than accepting an incomplete event history', async () => {
    const pending = deferred(); mocks.refresh.mockReturnValue(pending.promise)
    const hook = await stagedHook()
    let command!: Promise<void>
    act(() => { command = hook.result.current.refreshConnection!(oldID, '') })
    for (let seq = 1; seq <= 129; seq++) emit('transfer-started', {sessionId: newID, seq})
    await act(async () => { pending.resolve({mode: 'send', send: stage(newID)}); await command })
    expect(mocks.cancel).toHaveBeenCalledTimes(1)
    expect(hook.result.current.state.phase).toBe('idle')
})

it('accepts opaque network IDs and refuses malformed result unions', () => {
    expect(parseConnectionOptions({sessionId: oldID, currentAddress: '192.168.1.5', choices: [
        {id: 'opaque-network-choice', name: 'Wi-Fi', address: '192.168.1.5'},
    ]})?.choices[0].id).toBe('opaque-network-choice')
    expect(parseConnectionRefresh({mode: 'send', send: stage(newID), receive: {sessionId: newID}})).toBeNull()
})

it('refreshes a waiting receive without reopening the destination chooser or exposing its old code', async () => {
    const pending = deferred(); mocks.refresh.mockReturnValue(pending.promise)
    const hook = renderHook(() => useTransfer())
    await act(async () => { await hook.result.current.receive() })
    expect(hook.result.current.state.phase).toBe('receive-waiting')
    let command!: Promise<void>
    act(() => { command = hook.result.current.refreshConnection!(oldID, '') })
    expect(hook.result.current.state.phase).toBe('recovering')
    emit('transfer-reset', {sessionId: oldID, seq: 1})
    await act(async () => { pending.resolve({mode: 'receive', receive: receive(newID)}); await command })
    expect(hook.result.current.state.phase).toBe('receive-waiting')
    if (hook.result.current.state.phase === 'receive-waiting') {
        expect(hook.result.current.state.metadata.sessionId).toBe(newID)
        expect(hook.result.current.state.recovered).toBe(true)
    }
    expect(mocks.selectReceiveFolder).toHaveBeenCalledTimes(1)
    expect(mocks.startReceive).toHaveBeenCalledTimes(1)
})

it('preserves receive refusals and focuses preflight errors instead of announcing a false success', () => {
    const waiting: TransferState = {phase: 'receive-waiting', session: {sessionId: oldID, lastSeq: 2},
        metadata: receive(), cancelPending: false, commandError: null, refusals: 3}
    const recovering = transferReducer(waiting, {type: 'recovery-requested', generation: 1, sessionId: oldID})
    const refused = transferReducer(recovering, {type: 'recovery-failed', generation: 1, sessionId: oldID,
        retired: false, error: publicError('network_unavailable')})
    expect(refused.phase).toBe('receive-waiting')
    if (refused.phase === 'receive-waiting') expect(refused.refusals).toBe(3)
    expect(routeTransition(recovering, refused)).toEqual({row: 'command-failure', owner: 'focus', target: 'command-error'})
})
