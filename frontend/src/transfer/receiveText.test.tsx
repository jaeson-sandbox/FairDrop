import {act, renderHook} from '@testing-library/react'
import {beforeEach, describe, expect, it, vi} from 'vitest'
import {parseConnectionRefresh, parseReceiveTextMetadata} from './validation'
import {useTransfer} from './useTransfer'

const mocks = vi.hoisted(() => ({start: vi.fn(), get: vi.fn(), copy: vi.fn(), cancel: vi.fn(), refresh: vi.fn(), events: vi.fn()}))
vi.mock('../../wailsjs/go/main/App', () => ({
    StartReceiveText: mocks.start, GetReceivedText: mocks.get, CopyReceivedText: mocks.copy,
    CancelTransfer: mocks.cancel, RefreshConnection: mocks.refresh,
    StageTransfer: vi.fn(), StageTransfers: vi.fn(), StageText: vi.fn(), StartReceive: vi.fn(),
    SelectFile: vi.fn(), SelectFiles: vi.fn(), SelectDirectory: vi.fn(), SelectReceiveFolder: vi.fn(), ShowReceivedFolder: vi.fn(),
}))
vi.mock('../../wailsjs/runtime/runtime', () => ({EventsOn: mocks.events}))

const first = '11111111111111111111111111111111'
const second = '22222222222222222222222222222222'
const png = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII='
const metadata = (id = first) => ({sessionId: id,
    url: `http://192.168.1.5:45678/receive-text/${id}`, qrBase64: png, warnings: []})
const listeners = new Map<string, (...args: unknown[]) => void>()
function emit(name: string, id: string, seq: number, extra: Record<string, unknown> = {}) {
    act(() => listeners.get(name)?.({sessionId: id, seq, ...extra}))
}
function progress() { return {bytesSent: 0, totalBytes: 0, totalKnown: false, percent: 0, speedBytesPerSec: 0} }
function deferred<T>() {
    let resolve!: (value: T) => void
    let reject!: (reason?: unknown) => void
    const promise = new Promise<T>((a, b) => { resolve = a; reject = b })
    return {promise, resolve, reject}
}

beforeEach(() => {
    listeners.clear()
    for (const mock of Object.values(mocks)) mock.mockReset()
    mocks.start.mockResolvedValue(metadata())
    mocks.cancel.mockResolvedValue(undefined)
    mocks.copy.mockResolvedValue(undefined)
    mocks.get.mockResolvedValue('exact <value>')
    mocks.events.mockImplementation((name: string, callback: (...args: unknown[]) => void) => {
        listeners.set(name, callback)
        return () => listeners.delete(name)
    })
})

async function waiting() {
    const hook = renderHook(() => useTransfer())
    await act(async () => { await hook.result.current.receiveText() })
    expect(hook.result.current.state.phase).toBe('receive-text-waiting')
    return hook
}

async function received(value = 'exact <value>') {
    mocks.get.mockResolvedValue(value)
    const hook = await waiting()
    emit('transfer-started', first, 1)
    await act(async () => { emit('transfer-complete', first, 2, {progress: progress()}); await Promise.resolve() })
    expect(hook.result.current.state.phase).toBe('receive-text-outcome')
    return hook
}

describe('inbound text metadata and controller', () => {
    it('accepts only the exact receive-text route and an exclusive recovery result', () => {
        expect(parseReceiveTextMetadata(metadata())).toEqual(metadata())
        expect(parseReceiveTextMetadata({...metadata(), value: 'secret'})).toBeNull()
        expect(parseReceiveTextMetadata({...metadata(), url: `http://192.168.1.5:45678/text/${first}`})).toBeNull()
        expect(parseConnectionRefresh({mode: 'receive-text', receiveText: metadata(second)}))
            .toEqual({mode: 'receive-text', receiveText: metadata(second)})
        expect(parseConnectionRefresh({mode: 'receive-text', receiveText: metadata(second), receive: {}})).toBeNull()
    })

    it('starts without file chooser, source or clipboard commands and recovers with a fresh empty identity', async () => {
        const hook = await waiting()
        expect(mocks.start).toHaveBeenCalledTimes(1)
        expect(mocks.get).not.toHaveBeenCalled()
        expect(mocks.copy).not.toHaveBeenCalled()
        mocks.refresh.mockResolvedValue({mode: 'receive-text', receiveText: metadata(second)})
        await act(async () => { await hook.result.current.refreshConnection?.(first, '') })
        expect(hook.result.current.state.phase).toBe('receive-text-waiting')
        if (hook.result.current.state.phase === 'receive-text-waiting') {
            expect(hook.result.current.state.session.sessionId).toBe(second)
            expect(hook.result.current.state.metadata.url).toBe(`http://192.168.1.5:45678/receive-text/${second}`)
        }
        expect(mocks.get).not.toHaveBeenCalled()
    })

    it('fetches only after complete and keeps exact normalized literal markup until Done', async () => {
        const hook = await received('\n<&/textarea><script>bad()</script>\n雪')
        expect(mocks.get).toHaveBeenCalledExactlyOnceWith(first)
        if (hook.result.current.state.phase !== 'receive-text-outcome') throw new Error('missing outcome')
        expect(hook.result.current.state.value).toBe('\n<&/textarea><script>bad()</script>\n雪')
        await act(async () => { await hook.result.current.cancel() })
        emit('transfer-reset', first, 3)
        expect(hook.result.current.state.phase).toBe('idle')
        expect(JSON.stringify(hook.result.current.state)).not.toContain('bad()')
    })

    it('keeps a fixed recoverable error for an unexpected fetch refusal and retries', async () => {
        mocks.get.mockRejectedValueOnce(new Error('secret adapter detail')).mockResolvedValueOnce('retry value')
        const hook = await waiting()
        emit('transfer-started', first, 1)
        await act(async () => { emit('transfer-complete', first, 2, {progress: progress()}); await Promise.resolve() })
        expect(hook.result.current.state.phase).toBe('receive-text-outcome')
        if (hook.result.current.state.phase !== 'receive-text-outcome') return
        expect(hook.result.current.state.fetchError).toBe(true)
        expect(hook.result.current.state.value).toBeNull()
        await act(async () => { await hook.result.current.fetchReceivedText(first) })
        if (hook.result.current.state.phase !== 'receive-text-outcome') return
        expect(hook.result.current.state.value).toBe('retry value')
        expect(hook.result.current.state.fetchError).toBe(false)
    })

    it('announces copy success only after native success and allows a failure retry', async () => {
        const hook = await received()
        mocks.copy.mockRejectedValueOnce(new Error('clipboard denied')).mockResolvedValueOnce(undefined)
        let copied = false
        await act(async () => { copied = await hook.result.current.copyReceivedText(first) })
        expect(copied).toBe(false)
        if (hook.result.current.state.phase !== 'receive-text-outcome') return
        expect(hook.result.current.state.copyError).toBe(true)
        await act(async () => { copied = await hook.result.current.copyReceivedText(first) })
        expect(copied).toBe(true)
        expect(mocks.copy).toHaveBeenCalledTimes(2)
        expect(mocks.copy).toHaveBeenNthCalledWith(2, first)
    })

    it('fences old copy across Done and a new receipt while allowing the new copy', async () => {
        const hook = await received()
        const old = deferred<void>()
        const newer = deferred<void>()
        mocks.copy.mockReturnValueOnce(old.promise).mockReturnValueOnce(newer.promise)
        let oldResult = true
        let newResult = false
        act(() => { void hook.result.current.copyReceivedText(first).then(value => { oldResult = value }) })
        await act(async () => { await hook.result.current.cancel() })
        emit('transfer-reset', first, 3)
        mocks.start.mockResolvedValue(metadata(second))
        await act(async () => { await hook.result.current.receiveText() })
        emit('transfer-started', second, 1)
        await act(async () => { emit('transfer-complete', second, 2, {progress: progress()}); await Promise.resolve() })
        let state = hook.result.current.state
        expect(state.phase).toBe('receive-text-outcome')
        act(() => { void hook.result.current.copyReceivedText(second).then(value => { newResult = value }) })
        await act(async () => { old.resolve() })
        expect(oldResult).toBe(false)
        state = hook.result.current.state
        if (state.phase === 'receive-text-outcome') expect(state.copying).toBe(true)
        await act(async () => { newer.resolve() })
        expect(newResult).toBe(true)
        expect(mocks.copy).toHaveBeenNthCalledWith(2, second)
    })

    it('ignores a late value after Done and unmount', async () => {
        const pending = deferred<string>()
        mocks.get.mockReturnValue(pending.promise)
        const hook = await waiting()
        emit('transfer-started', first, 1)
        act(() => emit('transfer-complete', first, 2, {progress: progress()}))
        expect(mocks.get).toHaveBeenCalledWith(first)
        await act(async () => { await hook.result.current.cancel() })
        emit('transfer-reset', first, 3)
        hook.unmount()
        await act(async () => { pending.resolve('late secret') })
        expect(hook.result.current.state.phase).toBe('idle')
    })

    it('ignores an obsolete fetch while a newer read of the same receipt is pending', async () => {
        const old = deferred<string>()
        const fresh = deferred<string>()
        mocks.get.mockReturnValueOnce(old.promise).mockReturnValueOnce(fresh.promise)
        mocks.cancel.mockRejectedValueOnce(new Error('teardown failed'))
        const hook = await waiting()
        emit('transfer-started', first, 1)
        act(() => emit('transfer-complete', first, 2, {progress: progress()}))
        await act(async () => { await hook.result.current.cancel() })
        if (hook.result.current.state.phase !== 'receive-text-outcome') throw new Error('missing receipt after failed Done')
        expect(hook.result.current.state.fetchError).toBe(true)
        act(() => { void hook.result.current.fetchReceivedText(first) })
        await act(async () => { old.resolve('obsolete value') })
        if (hook.result.current.state.phase !== 'receive-text-outcome') throw new Error('missing receipt')
        expect(hook.result.current.state.value).toBeNull()
        await act(async () => { fresh.resolve('fresh value') })
        if (hook.result.current.state.phase !== 'receive-text-outcome') throw new Error('missing receipt')
        expect(hook.result.current.state.value).toBe('fresh value')
    })

    it('keeps a failed Done on a pending fetch recoverable and refuses the obsolete value', async () => {
        const old = deferred<string>()
        mocks.get.mockReturnValueOnce(old.promise).mockResolvedValueOnce('recovered text')
        mocks.cancel.mockRejectedValueOnce(new Error('private teardown detail')).mockResolvedValueOnce(undefined)
        const hook = await waiting()
        emit('transfer-started', first, 1)
        act(() => emit('transfer-complete', first, 2, {progress: progress()}))
        await act(async () => { await hook.result.current.cancel() })
        if (hook.result.current.state.phase !== 'receive-text-outcome') throw new Error('missing held receipt')
        expect(hook.result.current.state.fetchError).toBe(true)
        expect(hook.result.current.state.commandError?.message).not.toContain('private teardown detail')
        await act(async () => { old.resolve('obsolete secret') })
        if (hook.result.current.state.phase !== 'receive-text-outcome') throw new Error('missing held receipt')
        expect(hook.result.current.state.value).toBeNull()
        await act(async () => { await hook.result.current.fetchReceivedText(first) })
        if (hook.result.current.state.phase !== 'receive-text-outcome') throw new Error('missing held receipt')
        expect(hook.result.current.state.value).toBe('recovered text')
        await act(async () => { await hook.result.current.cancel() })
        expect(mocks.cancel).toHaveBeenCalledTimes(2)
    })

    it('does not admit a fetch retry while Done is unresolved', async () => {
        mocks.get.mockRejectedValueOnce(new Error('initial read failed'))
        const hook = await waiting()
        emit('transfer-started', first, 1)
        await act(async () => { emit('transfer-complete', first, 2, {progress: progress()}); await Promise.resolve() })
        const pending = deferred<void>()
        mocks.cancel.mockReturnValueOnce(pending.promise)
        act(() => { void hook.result.current.cancel() })
        if (hook.result.current.state.phase !== 'receive-text-outcome') throw new Error('missing held receipt')
        expect(hook.result.current.state.donePending).toBe(true)
        await act(async () => { await hook.result.current.fetchReceivedText(first) })
        expect(mocks.get).toHaveBeenCalledTimes(1)
        await act(async () => { pending.resolve() })
    })

    it('keeps a failed Done on a pending copy usable and fences its late completion', async () => {
        const old = deferred<void>()
        mocks.copy.mockReturnValueOnce(old.promise).mockResolvedValueOnce(undefined)
        mocks.cancel.mockRejectedValueOnce(new Error('private teardown detail')).mockResolvedValueOnce(undefined)
        const hook = await received()
        let oldResult = true
        act(() => { void hook.result.current.copyReceivedText(first).then(value => { oldResult = value }) })
        await act(async () => { await hook.result.current.cancel() })
        if (hook.result.current.state.phase !== 'receive-text-outcome') throw new Error('missing held receipt')
        expect(hook.result.current.state.copying).toBe(false)
        expect(hook.result.current.state.value).toBe('exact <value>')
        expect(hook.result.current.state.commandError?.message).not.toContain('private teardown detail')
        await act(async () => { old.resolve() })
        expect(oldResult).toBe(false)
        let retried = false
        await act(async () => { retried = await hook.result.current.copyReceivedText(first) })
        expect(retried).toBe(true)
        expect(mocks.copy).toHaveBeenCalledTimes(2)
        await act(async () => { await hook.result.current.cancel() })
        expect(mocks.cancel).toHaveBeenCalledTimes(2)
    })

    it('keeps Receive text context for initial and retired recovery failures', async () => {
        mocks.start.mockRejectedValueOnce(new Error('setup private'))
        const hook = renderHook(() => useTransfer())
        await act(async () => { await hook.result.current.receiveText() })
        expect(hook.result.current.state).toMatchObject({phase: 'idle', commandContext: 'receive-text'})
        await act(async () => { await hook.result.current.receiveText() })
        mocks.refresh.mockRejectedValueOnce(new Error(JSON.stringify({retired: true, code: 'setup_failed', message: 'recovery private'})))
        await act(async () => { await hook.result.current.refreshConnection?.(first, '') })
        expect(hook.result.current.state).toMatchObject({phase: 'idle', commandContext: 'receive-text'})
    })

    it.each(['waiting', 'receiving', 'outcome'] as const)('cancels a live %s session on direct unmount and ignores late getter work', async phase => {
        const pending = deferred<string>()
        mocks.get.mockReturnValue(pending.promise)
        const hook = await waiting()
        if (phase !== 'waiting') emit('transfer-started', first, 1)
        if (phase === 'outcome') act(() => emit('transfer-complete', first, 2, {progress: progress()}))
        hook.unmount()
        await act(async () => { await Promise.resolve() })
        expect(mocks.cancel).toHaveBeenCalledTimes(1)
        await act(async () => { pending.resolve('late private value') })
        expect(JSON.stringify(hook.result.current.state)).not.toContain('late private value')
    })
})
