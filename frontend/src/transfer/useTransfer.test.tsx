import {StrictMode} from 'react'
import {act, renderHook, waitFor} from '@testing-library/react'
import {beforeEach, describe, expect, it, vi} from 'vitest'
import {useTransfer} from './useTransfer'

const mocks = vi.hoisted(() => ({
    stageTransfer: vi.fn(),
    cancelTransfer: vi.fn(),
    eventsOn: vi.fn(),
}))

vi.mock('../../wailsjs/go/main/App', () => ({
    StageTransfer: mocks.stageTransfer,
    CancelTransfer: mocks.cancelTransfer,
}))

vi.mock('../../wailsjs/runtime/runtime', () => ({
    EventsOn: mocks.eventsOn,
}))

interface Subscription {
    readonly name: string
    readonly callback: (...args: unknown[]) => void
    readonly dispose: ReturnType<typeof vi.fn>
    active: boolean
}

let subscriptions: Subscription[]
const sessionId = '0123456789abcdef0123456789abcdef'
const capabilityURL = 'http://192.0.2.1:34123/download/fedcba9876543210fedcba9876543210'
const qrPNG = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII='

function metadata(overrides: Record<string, unknown> = {}): Record<string, unknown> {
    return {
        sessionId,
        name: 'report.pdf',
        size: 100,
        isDir: false,
        url: capabilityURL,
        qrBase64: qrPNG,
        warnings: [],
        ...overrides,
    }
}

function progress(bytesSent: number): Record<string, unknown> {
    return {
        bytesSent,
        totalBytes: 100,
        totalKnown: true,
        percent: bytesSent,
        speedBytesPerSec: 10,
    }
}

function deferred<T>() {
    let resolve!: (value: T | PromiseLike<T>) => void
    let reject!: (reason?: unknown) => void
    const promise = new Promise<T>((resolvePromise, rejectPromise) => {
        resolve = resolvePromise
        reject = rejectPromise
    })
    return {promise, resolve, reject}
}

function emit(name: string, ...args: unknown[]): void {
    const subscription = subscriptions.findLast((candidate) => candidate.name === name && candidate.active)
    expect(subscription, `live ${name} subscription`).toBeTruthy()
    subscription!.callback(...args)
}

beforeEach(() => {
    subscriptions = []
    mocks.stageTransfer.mockReset()
    mocks.cancelTransfer.mockReset()
    mocks.eventsOn.mockReset()
    mocks.cancelTransfer.mockResolvedValue(undefined)
    mocks.eventsOn.mockImplementation((name: string, callback: (...args: unknown[]) => void) => {
        const subscription: Subscription = {
            name,
            callback,
            active: true,
            dispose: vi.fn(() => { subscription.active = false }),
        }
        subscriptions.push(subscription)
        return subscription.dispose
    })
})
describe('Wails lifecycle subscriptions', () => {
    it('registers the five literal names once and invokes each disposer exactly once', () => {
        const hook = renderHook(() => useTransfer())

        expect(mocks.eventsOn.mock.calls.map(([name]) => name)).toEqual([
            'transfer-started',
            'transfer-progress',
            'transfer-complete',
            'transfer-error',
            'transfer-reset',
        ])

        hook.unmount()
        for (const subscription of subscriptions) expect(subscription.dispose).toHaveBeenCalledTimes(1)
    })

    it('leaves a second subscriber live when the first unmounts', async () => {
        mocks.stageTransfer.mockResolvedValue(metadata())
        const first = renderHook(() => useTransfer())
        const second = renderHook(() => useTransfer())

        await act(async () => {
            await Promise.all([
                first.result.current.stage('C:\\one.pdf'),
                second.result.current.stage('C:\\two.pdf'),
            ])
        })
        first.unmount()

        expect(subscriptions.slice(0, 5).every(({dispose}) => dispose.mock.calls.length === 1)).toBe(true)
        expect(subscriptions.slice(5).every(({dispose}) => dispose.mock.calls.length === 0)).toBe(true)

        act(() => emit('transfer-started', {sessionId, seq: 1}))
        expect(second.result.current.state.phase).toBe('transferring')
    })

    it('keeps one live listener per name through StrictMode cleanup and remount', () => {
        const hook = renderHook(() => useTransfer(), {wrapper: StrictMode})

        expect(mocks.eventsOn).toHaveBeenCalledTimes(10)
        expect(subscriptions.slice(0, 5).every(({dispose}) => dispose.mock.calls.length === 1)).toBe(true)
        expect(subscriptions.filter(({active}) => active).map(({name}) => name)).toEqual([
            'transfer-started',
            'transfer-progress',
            'transfer-complete',
            'transfer-error',
            'transfer-reset',
        ])

        hook.unmount()
        expect(subscriptions.every(({dispose}) => dispose.mock.calls.length === 1)).toBe(true)
    })

    it('ignores a disposed StrictMode callback while the remounted subscription is live', async () => {
        mocks.stageTransfer.mockResolvedValue(metadata())
        const hook = renderHook(() => useTransfer(), {wrapper: StrictMode})
        await act(async () => { await hook.result.current.stage('C:\\one.pdf') })
        const stagedState = hook.result.current.state
        const stale = subscriptions.slice(0, 5).find(({name}) => name === 'transfer-started')!.callback
        const live = subscriptions.slice(5).find(({name}) => name === 'transfer-started')!.callback

        act(() => stale({sessionId, seq: 1}))
        expect(hook.result.current.state).toBe(stagedState)

        act(() => live({sessionId, seq: 1}))
        expect(hook.result.current.state).toMatchObject({
            phase: 'transferring',
            session: {sessionId, lastSeq: 1},
        })
    })

    it('ignores a callback retained after its subscription cleanup', async () => {
        mocks.stageTransfer.mockResolvedValue(metadata())
        const hook = renderHook(() => useTransfer())
        await act(async () => { await hook.result.current.stage('C:\\one.pdf') })
        const stale = subscriptions.find(({name}) => name === 'transfer-started')!.callback

        hook.unmount()
        expect(() => stale({sessionId, seq: 1})).not.toThrow()
    })

    it('disposes every listener acquired before registration throws', () => {
        mocks.eventsOn.mockImplementation((name: string, callback: (...args: unknown[]) => void) => {
            if (name === 'transfer-complete') throw new Error('registration failed')
            const subscription: Subscription = {
                name,
                callback,
                active: true,
                dispose: vi.fn(() => { subscription.active = false }),
            }
            subscriptions.push(subscription)
            return subscription.dispose
        })

        expect(() => renderHook(() => useTransfer())).toThrow('registration failed')
        expect(subscriptions).toHaveLength(2)
        expect(subscriptions.every(({dispose}) => dispose.mock.calls.length === 1)).toBe(true)
    })

    it('attempts every listener disposer when one throws', () => {
        const hook = renderHook(() => useTransfer())
        subscriptions[1].dispose.mockImplementation(() => { throw new Error('dispose failed') })

        expect(() => hook.unmount()).not.toThrow()
        expect(subscriptions.map(({dispose}) => dispose.mock.calls.length)).toEqual([1, 1, 1, 1, 1])
    })
})

describe('Stage generations and malformed acknowledgements', () => {
    it('forwards the exact selected path and suppresses a repeated Stage while the first is pending', async () => {
        const stageDeferred = deferred<unknown>()
        mocks.stageTransfer.mockReturnValue(stageDeferred.promise)
        const hook = renderHook(() => useTransfer())
        const selectedPath = String.raw`C:\Shared Folder\ report.pdf `

        let first!: Promise<void>
        let repeated!: Promise<void>
        act(() => {
            first = hook.result.current.stage(selectedPath)
            repeated = hook.result.current.stage(String.raw`C:\must-not-stage.pdf`)
        })
        await act(async () => { await Promise.resolve() })

        expect(mocks.stageTransfer).toHaveBeenCalledTimes(1)
        expect(mocks.stageTransfer).toHaveBeenCalledWith(String.raw`C:\Shared Folder\ report.pdf `)

        await act(async () => {
            stageDeferred.resolve(metadata())
            await Promise.all([first, repeated])
        })
        expect(hook.result.current.state).toMatchObject({
            phase: 'staged', session: {sessionId, lastSeq: 0},
        })
    })

    it('does not install an acknowledgement made obsolete by local cancellation', async () => {
        const stageDeferred = deferred<unknown>()
        const cancelDeferred = deferred<void>()
        mocks.stageTransfer.mockReturnValue(stageDeferred.promise)
        mocks.cancelTransfer.mockReturnValue(cancelDeferred.promise)
        const hook = renderHook(() => useTransfer())

        let stagePromise!: Promise<void>
        act(() => { stagePromise = hook.result.current.stage('C:\\report.pdf') })
        await waitFor(() => expect(hook.result.current.state).toMatchObject({phase: 'pending', cancelPending: false}))

        let cancelPromise!: Promise<void>
        act(() => { cancelPromise = hook.result.current.cancel() })
        await waitFor(() => expect(hook.result.current.state).toMatchObject({phase: 'pending', cancelPending: true}))

        await act(async () => {
            stageDeferred.resolve(metadata())
            await stagePromise
        })
        expect(hook.result.current.state.phase).toBe('pending')

        await act(async () => {
            cancelDeferred.resolve()
            await cancelPromise
        })
        expect(hook.result.current.state).toEqual({phase: 'idle', retainedOutcome: null, commandError: null})
    })

    it('waits for a later cancelled Stage after Cancel settles first and suppresses repeated Cancel', async () => {
        const stageDeferred = deferred<unknown>()
        const cancelDeferred = deferred<void>()
        mocks.stageTransfer.mockReturnValue(stageDeferred.promise)
        mocks.cancelTransfer.mockReturnValue(cancelDeferred.promise)
        const hook = renderHook(() => useTransfer())

        let stagePromise!: Promise<void>
        act(() => { stagePromise = hook.result.current.stage('C:\\report.pdf') })
        await waitFor(() => expect(hook.result.current.state).toMatchObject({phase: 'pending', cancelPending: false}))

        let cancelPromise!: Promise<void>
        let repeatedCancelPromise!: Promise<void>
        act(() => {
            cancelPromise = hook.result.current.cancel()
            repeatedCancelPromise = hook.result.current.cancel()
        })
        await repeatedCancelPromise
        await waitFor(() => expect(mocks.cancelTransfer).toHaveBeenCalledTimes(1))

        await act(async () => {
            cancelDeferred.resolve()
            await Promise.resolve()
        })
        expect(hook.result.current.state).toMatchObject({phase: 'pending', cancelPending: true})

        await act(async () => {
            stageDeferred.reject(new Error(JSON.stringify({code: 'cancelled', message: 'forged'})))
            await Promise.all([stagePromise, cancelPromise])
        })
        expect(mocks.cancelTransfer).toHaveBeenCalledTimes(1)
        expect(hook.result.current.state).toEqual({phase: 'idle', retainedOutcome: null, commandError: null})
    })

    it('attempts Cancel exactly once for malformed successful metadata', async () => {
        mocks.stageTransfer.mockResolvedValue(metadata({sessionId: ''}))
        const hook = renderHook(() => useTransfer())

        await act(async () => { await hook.result.current.stage('C:\\report.pdf') })

        expect(mocks.cancelTransfer).toHaveBeenCalledTimes(1)
        expect(hook.result.current.state).toEqual({
            phase: 'idle',
            retainedOutcome: null,
            commandError: {
                code: 'transfer_failed',
                message: 'The transfer stopped before FairDrop finished sending. Check the local network and create a fresh link.',
            },
        })
    })

    it('uses fixed command copy and keeps cancelled out of Error state', async () => {
        const hook = renderHook(() => useTransfer())
        mocks.stageTransfer.mockRejectedValueOnce(new Error(JSON.stringify({
            code: 'path_not_found', message: String.raw`C:\\private\\report.pdf?token=secret`,
        })))

        await act(async () => { await hook.result.current.stage('C:\\report.pdf') })
        expect(hook.result.current.state).toMatchObject({
            phase: 'idle',
            commandError: {
                code: 'path_not_found',
                message: 'That file or folder is no longer available. Choose it again.',
            },
        })

        mocks.stageTransfer.mockRejectedValueOnce(new Error(JSON.stringify({code: 'cancelled', message: 'forged'})))
        await act(async () => { await hook.result.current.stage('C:\\report.pdf') })
        expect(hook.result.current.state).toEqual({phase: 'idle', retainedOutcome: null, commandError: null})
    })

    it('cancels an outstanding Stage exactly once on unmount and ignores its late acknowledgement', async () => {
        const stageDeferred = deferred<unknown>()
        mocks.stageTransfer.mockReturnValue(stageDeferred.promise)
        const hook = renderHook(() => useTransfer())

        let promise!: Promise<void>
        act(() => { promise = hook.result.current.stage('C:\\report.pdf') })
        hook.unmount()
        stageDeferred.resolve(metadata())

        await expect(promise).resolves.toBeUndefined()
        await waitFor(() => expect(mocks.cancelTransfer).toHaveBeenCalledTimes(1))
    })

    it('swallows the best-effort unmount cancellation rejection', async () => {
        const stageDeferred = deferred<unknown>()
        mocks.stageTransfer.mockReturnValue(stageDeferred.promise)
        mocks.cancelTransfer.mockRejectedValue(new Error('cleanup failed'))
        const hook = renderHook(() => useTransfer())

        const promise = hook.result.current.stage('C:\\report.pdf')
        hook.unmount()
        stageDeferred.resolve(metadata())

        await expect(promise).resolves.toBeUndefined()
        await waitFor(() => expect(mocks.cancelTransfer).toHaveBeenCalledTimes(1))
    })
})

describe('command and event races', () => {
    it('lets a terminal event win over an obsolete active Cancel rejection', async () => {
        const cancelDeferred = deferred<void>()
        mocks.stageTransfer.mockResolvedValue(metadata())
        mocks.cancelTransfer.mockReturnValue(cancelDeferred.promise)
        const hook = renderHook(() => useTransfer())

        await act(async () => { await hook.result.current.stage('C:\\report.pdf') })
        act(() => emit('transfer-started', {sessionId, seq: 1}))

        let cancelPromise!: Promise<void>
        act(() => { cancelPromise = hook.result.current.cancel() })
        act(() => emit('transfer-complete', {
            sessionId, seq: 2, progress: progress(100),
        }))

        await act(async () => {
            cancelDeferred.reject(new Error(JSON.stringify({code: 'transfer_failed', message: 'forged'})))
            await cancelPromise
        })
        expect(hook.result.current.state).toMatchObject({phase: 'done', outcome: {kind: 'done'}})
    })

    it('routes hostile callback input through the reducer without throwing or advancing', async () => {
        mocks.stageTransfer.mockResolvedValue(metadata())
        const hook = renderHook(() => useTransfer())
        await act(async () => { await hook.result.current.stage('C:\\report.pdf') })

        act(() => emit('transfer-started', {sessionId, seq: 1}, 'forged-extra'))
        expect(hook.result.current.state).toMatchObject({phase: 'staged', session: {lastSeq: 0}})

        act(() => emit('transfer-started', {sessionId, seq: 1}))
        expect(hook.result.current.state).toMatchObject({phase: 'transferring', session: {lastSeq: 1}})
    })

    it('does not let a stale unresolved Cancel block cancellation of a new session', async () => {
        const firstCancel = deferred<void>()
        const secondCancel = deferred<void>()
        const secondSessionId = '11111111111111111111111111111111'
        mocks.stageTransfer
            .mockResolvedValueOnce(metadata())
            .mockResolvedValueOnce(metadata({
                sessionId: secondSessionId,
                url: 'http://192.0.2.2:34124/download/22222222222222222222222222222222',
            }))
        mocks.cancelTransfer
            .mockReturnValueOnce(firstCancel.promise)
            .mockReturnValueOnce(secondCancel.promise)
        const hook = renderHook(() => useTransfer())

        await act(async () => { await hook.result.current.stage('C:\\first.pdf') })
        act(() => emit('transfer-started', {sessionId, seq: 1}))
        let staleCancelPromise!: Promise<void>
        act(() => { staleCancelPromise = hook.result.current.cancel() })
        act(() => emit('transfer-complete', {sessionId, seq: 2, progress: progress(100)}))
        act(() => emit('transfer-reset', {sessionId, seq: 3}))

        await act(async () => { await hook.result.current.stage('C:\\second.pdf') })
        act(() => emit('transfer-started', {sessionId: secondSessionId, seq: 1}))
        let currentCancelPromise!: Promise<void>
        act(() => { currentCancelPromise = hook.result.current.cancel() })

        expect(mocks.cancelTransfer).toHaveBeenCalledTimes(2)
        expect(hook.result.current.state).toMatchObject({
            phase: 'transferring', session: {sessionId: secondSessionId}, cancelPending: true,
        })

        await act(async () => {
            firstCancel.reject(new Error(JSON.stringify({code: 'transfer_failed', message: 'stale'})))
            secondCancel.resolve()
            await Promise.all([staleCancelPromise, currentCancelPromise])
        })
        expect(hook.result.current.state).toMatchObject({
            phase: 'transferring', session: {sessionId: secondSessionId}, cancelPending: true,
        })
        expect(JSON.stringify(hook.result.current.state)).not.toContain('stale')
    })
})
