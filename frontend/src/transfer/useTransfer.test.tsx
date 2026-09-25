import {StrictMode} from 'react'
import {act, renderHook, waitFor} from '@testing-library/react'
import {beforeEach, describe, expect, it, vi} from 'vitest'
import {useTransfer} from './useTransfer'

const mocks = vi.hoisted(() => ({
    stageTransfer: vi.fn(),
    cancelTransfer: vi.fn(),
    selectFile: vi.fn(),
    selectDirectory: vi.fn(),
    eventsOn: vi.fn(),
}))

vi.mock('../../wailsjs/go/main/App', () => ({
    StageTransfer: mocks.stageTransfer,
    CancelTransfer: mocks.cancelTransfer,
    SelectFile: mocks.selectFile,
    SelectDirectory: mocks.selectDirectory,
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
    mocks.selectFile.mockReset()
    mocks.selectDirectory.mockReset()
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

    /*
      Epic 1 retrospective item 7: the acknowledgement was parsed twice.

      The controller parsed it, dispatched the parsed record, and the reducer
      parsed it again -- a second base64 decode and a second full PNG chunk walk
      of up to 2 MB on the main thread, for a value the first parse had already
      accepted. Worse than the cost was the disagreement: the reducer's answer
      to a parse it refused was to return the same state, leaving the window in
      Pending with no error and no announcement, while the controller's answer
      was to quiesce the session and report setup_failed.

      Counting `atob` is what makes the claim observable: it is called once per
      parse, by the PNG check, and by nothing else in this path.
    */
    it('parses the acknowledgement once, decoding the QR payload a single time', async () => {
        const decoded = vi.spyOn(globalThis, 'atob')
        mocks.stageTransfer.mockResolvedValue(metadata())
        const hook = renderHook(() => useTransfer())

        await act(async () => { await hook.result.current.stage('C:\report.pdf') })

        expect(hook.result.current.state.phase).toBe('staged')
        expect(decoded).toHaveBeenCalledTimes(1)
        decoded.mockRestore()
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
                code: 'setup_failed',
                message: 'FairDrop couldn’t prepare that item. Nothing was sent. Choose it again.',
            },
        })
    })

    /*
      D-106: the same malformed acknowledgement, with the cleanup failing.

      "Nothing was sent. Choose it again." stays true about the bytes and
      becomes misleading about everything else: the backend had already
      committed a staged session with a listener and a capability URL, the
      cleanup that would have released it failed, and the next Stage is refused
      busy for a session the user was just told did not exist.

      The rejection's own text is still discarded -- it is adapter text -- but
      the fact of it is not, because it changes which sentence is true.
    */
    it('reports an unconfirmed cleanup when quiescing a malformed acknowledgement fails', async () => {
        mocks.stageTransfer.mockResolvedValue(metadata({sessionId: ''}))
        mocks.cancelTransfer.mockRejectedValue(new Error(String.raw`C:\privateeport.pdf?token=secret`))
        const hook = renderHook(() => useTransfer())

        await act(async () => { await hook.result.current.stage('C:\report.pdf') })

        expect(mocks.cancelTransfer).toHaveBeenCalledTimes(1)
        expect(hook.result.current.state).toEqual({
            phase: 'idle',
            retainedOutcome: null,
            commandError: {
                code: 'cleanup_unconfirmed',
                message: 'FairDrop couldn’t confirm it released the connection. Nothing was sent. ' +
                    'Close FairDrop and reopen it before sending again.',
            },
        })
        expect(JSON.stringify(hook.result.current.state)).not.toContain('token=secret')
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

    it('issues exactly one Cancel when two reach the same live session in one tick', async () => {
        const cancelDeferred = deferred<void>()
        mocks.stageTransfer.mockResolvedValue(metadata())
        mocks.cancelTransfer.mockReturnValue(cancelDeferred.promise)
        const hook = renderHook(() => useTransfer())

        await act(async () => { await hook.result.current.stage('C:\\report.pdf') })
        act(() => emit('transfer-started', {sessionId, seq: 1}))

        // Both activations read the same pre-dispatch state, so cancelPending
        // has not been set for the second one yet and cannot deduplicate it.
        let settled!: Promise<unknown>
        act(() => {
            settled = Promise.all([hook.result.current.cancel(), hook.result.current.cancel()])
        })
        expect(mocks.cancelTransfer).toHaveBeenCalledTimes(1)

        await act(async () => {
            cancelDeferred.resolve()
            await settled
        })
        expect(mocks.cancelTransfer).toHaveBeenCalledTimes(1)
        expect(hook.result.current.state).toMatchObject({phase: 'transferring', cancelPending: true})
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

describe('native browse commands', () => {
    it.each([
        ['selectFile'],
        ['selectDirectory'],
    ])('stages a non-empty %s result immediately', async (command) => {
        const chooser = command === 'selectFile' ? mocks.selectFile : mocks.selectDirectory
        const other = command === 'selectFile' ? mocks.selectDirectory : mocks.selectFile
        chooser.mockResolvedValue('C:\\chosen\\item')
        mocks.stageTransfer.mockResolvedValue(metadata())
        const hook = renderHook(() => useTransfer())

        await act(async () => {
            await (command === 'selectFile'
                ? hook.result.current.selectFile()
                : hook.result.current.selectDirectory())
        })

        expect(chooser).toHaveBeenCalledTimes(1)
        expect(other).not.toHaveBeenCalled()
        expect(mocks.stageTransfer).toHaveBeenCalledWith('C:\\chosen\\item')
        expect(hook.result.current.state).toMatchObject({phase: 'staged', metadata: {name: 'report.pdf'}})
    })

    it('names the pending item kind the chooser established', async () => {
        mocks.selectDirectory.mockResolvedValue('C:\\chosen\\folder')
        const staging = deferred<unknown>()
        mocks.stageTransfer.mockReturnValue(staging.promise)
        const hook = renderHook(() => useTransfer())

        let pending!: Promise<void>
        await act(async () => {
            pending = hook.result.current.selectDirectory()
            await Promise.resolve()
        })

        expect(hook.result.current.state).toMatchObject({phase: 'pending', itemKind: 'directory'})

        await act(async () => {
            staging.resolve(metadata())
            await pending
        })
        expect(hook.result.current.state.phase).toBe('staged')
    })

    it.each([
        ['a dismissed chooser', ''],
        ['a whitespace-only path', '   '],
    ])('stays silently in Idle for %s', async (_name, selection) => {
        mocks.selectFile.mockResolvedValue(selection)
        const hook = renderHook(() => useTransfer())
        const before = hook.result.current.state

        await act(async () => { await hook.result.current.selectFile() })

        expect(mocks.stageTransfer).not.toHaveBeenCalled()
        expect(mocks.cancelTransfer).not.toHaveBeenCalled()
        expect(hook.result.current.state).toBe(before)
    })

    it('reports a chooser rejection as a fixed command error without ever showing preparation', async () => {
        mocks.selectFile.mockRejectedValue(new Error(JSON.stringify({
            code: 'shutting_down',
            message: 'anything the adapter felt like saying',
        })))
        const phases: string[] = []
        const hook = renderHook(() => {
            const controller = useTransfer()
            phases.push(controller.state.phase)
            return controller
        })

        await act(async () => { await hook.result.current.selectFile() })

        expect(mocks.stageTransfer).not.toHaveBeenCalled()
        expect(hook.result.current.state).toEqual({
            phase: 'idle',
            retainedOutcome: null,
            commandError: {code: 'shutting_down', message: 'FairDrop is closing. Reopen it to start a transfer.'},
        })
        // The Idle command error is reached through Pending, but in one batch:
        // no render ever observes the preparation surface.
        expect(phases).not.toContain('pending')
    })

    it('falls back to the safe unknown failure when the rejection is not a public error', async () => {
        mocks.selectDirectory.mockRejectedValue(new Error('C:\\Users\\jaeson\\Pictures could not be opened'))
        const hook = renderHook(() => useTransfer())

        await act(async () => { await hook.result.current.selectDirectory() })

        expect(hook.result.current.state).toMatchObject({
            phase: 'idle',
            commandError: {code: 'transfer_failed'},
        })
        expect(JSON.stringify(hook.result.current.state)).not.toContain('jaeson')
    })

    it('never reports a cancellation as a command error', async () => {
        mocks.selectFile.mockRejectedValue(new Error(JSON.stringify({
            code: 'cancelled',
            message: 'Transfer canceled.',
        })))
        const hook = renderHook(() => useTransfer())

        await act(async () => { await hook.result.current.selectFile() })

        expect(hook.result.current.state).toEqual({phase: 'idle', retainedOutcome: null, commandError: null})
    })

    it('runs one chooser at a time and ignores a drop while one is open', async () => {
        const chooser = deferred<string>()
        mocks.selectFile.mockReturnValue(chooser.promise)
        mocks.stageTransfer.mockResolvedValue(metadata())
        const hook = renderHook(() => useTransfer())

        let first!: Promise<void>
        await act(async () => {
            first = hook.result.current.selectFile()
            await Promise.resolve()
        })

        await act(async () => {
            await hook.result.current.selectDirectory()
            await hook.result.current.stage('C:\\dropped.pdf', 'unknown')
        })
        expect(mocks.selectDirectory).not.toHaveBeenCalled()
        expect(mocks.stageTransfer).not.toHaveBeenCalled()

        await act(async () => {
            chooser.resolve('C:\\chosen.pdf')
            await first
        })
        expect(mocks.stageTransfer).toHaveBeenCalledTimes(1)
        expect(mocks.stageTransfer).toHaveBeenCalledWith('C:\\chosen.pdf')
    })

    it('refuses to open a chooser outside Idle', async () => {
        mocks.stageTransfer.mockResolvedValue(metadata())
        mocks.selectFile.mockResolvedValue('C:\\second.pdf')
        const hook = renderHook(() => useTransfer())
        await act(async () => { await hook.result.current.stage('C:\\first.pdf', 'unknown') })

        await act(async () => { await hook.result.current.selectFile() })

        expect(mocks.selectFile).not.toHaveBeenCalled()
        expect(hook.result.current.state.phase).toBe('staged')
    })

    it('drops a chooser result that resolves after unmount', async () => {
        const chooser = deferred<string>()
        mocks.selectFile.mockReturnValue(chooser.promise)
        const hook = renderHook(() => useTransfer())

        let pending!: Promise<void>
        await act(async () => {
            pending = hook.result.current.selectFile()
            await Promise.resolve()
        })
        hook.unmount()

        await act(async () => {
            chooser.resolve('C:\\chosen.pdf')
            await pending
        })
        expect(mocks.stageTransfer).not.toHaveBeenCalled()
    })

    it('owes no CancelTransfer for a chooser that was never a session', async () => {
        const chooser = deferred<string>()
        mocks.selectFile.mockReturnValue(chooser.promise)
        const hook = renderHook(() => useTransfer())

        await act(async () => {
            void hook.result.current.selectFile()
            await Promise.resolve()
        })
        hook.unmount()
        await act(async () => {
            chooser.reject(new Error('dismissed'))
            await Promise.resolve()
        })

        expect(mocks.cancelTransfer).not.toHaveBeenCalled()
    })
})

/*
  D-059's control, driven through the real hook.

  Story 3.6 made a live Done or Error always carry a control, because a lost
  transfer-reset would otherwise strand the window with no way out. App wires
  that control to cancel(). What nothing checked was whether cancel() does
  anything from a terminal phase -- and until 2026-09-14 it did not: the guard
  for staged/transferring sent it straight back, so the control rendered and
  was inert, which is the same stranded window with a button on it.

  The test that was supposed to cover this mocks useTransfer entirely and
  asserts the mock's cancel was called. That proves App's wiring and nothing
  about the hook, which is exactly how this survived being written and
  reviewed. This drives the real reducer into `done` with real lifecycle events
  and requires the backend command to actually be invoked.
*/
describe('a live terminal outcome is cancellable', () => {
    async function driveToDone(hook: {result: {current: ReturnType<typeof useTransfer>}}) {
        mocks.stageTransfer.mockResolvedValue(metadata())
        await act(async () => { await hook.result.current.stage('C:\report.pdf') })
        act(() => {
            emit('transfer-started', {sessionId, seq: 1})
            emit('transfer-complete', {sessionId, seq: 2, progress: progress(100)})
        })
    }

    it('invokes the backend cancel from a live done outcome', async () => {
        const hook = renderHook(() => useTransfer())
        await driveToDone(hook)
        expect(hook.result.current.state.phase).toBe('done')

        mocks.cancelTransfer.mockResolvedValue(undefined)
        await act(async () => { await hook.result.current.cancel() })

        expect(mocks.cancelTransfer).toHaveBeenCalledTimes(1)
    })

    it('does not issue a second cancel for the same terminal session', async () => {
        const hook = renderHook(() => useTransfer())
        await driveToDone(hook)

        const pending = deferred<void>()
        mocks.cancelTransfer.mockReturnValue(pending.promise)
        let first!: Promise<void>
        let second!: Promise<void>
        act(() => {
            first = hook.result.current.cancel()
            second = hook.result.current.cancel()
        })
        await act(async () => {
            pending.resolve()
            await Promise.all([first, second])
        })

        expect(mocks.cancelTransfer).toHaveBeenCalledTimes(1)
    })

    it('leaves the outcome on screen when the backend refuses', async () => {
        const hook = renderHook(() => useTransfer())
        await driveToDone(hook)

        mocks.cancelTransfer.mockRejectedValue(new Error(String.raw`C:PRIVATE?token=secret`))
        await act(async () => { await hook.result.current.cancel() })

        expect(hook.result.current.state.phase).toBe('done')
        expect(JSON.stringify(hook.result.current.state)).not.toContain('token=secret')
    })
})

/*
  The one command a view issues on its own behalf, wired end to end.

  The staged view proves it calls this, and the reducer proves what the action
  does; without a test here the middle was a seam nobody drove -- a
  reportCopyFailure that dispatched nothing passed all 519 other tests.
*/
describe('a failed clipboard write reaches the reducer', () => {
    async function driveToStaged(hook: {result: {current: ReturnType<typeof useTransfer>}}) {
        mocks.stageTransfer.mockResolvedValue(metadata())
        await act(async () => { await hook.result.current.stage('C:\\report.pdf') })
    }

    it('shows the registry message for the staged session', async () => {
        const hook = renderHook(() => useTransfer())
        await driveToStaged(hook)

        act(() => { hook.result.current.reportCopyFailure(sessionId) })

        const state = hook.result.current.state
        expect(state.phase).toBe('staged')
        expect(state.phase === 'staged' ? state.commandError?.code : null).toBe('clipboard_failed')
    })

    it('ignores a rejection carrying a session that is not the staged one', async () => {
        const hook = renderHook(() => useTransfer())
        await driveToStaged(hook)
        const before = hook.result.current.state

        act(() => { hook.result.current.reportCopyFailure('ffffffffffffffffffffffffffffffff') })

        expect(hook.result.current.state).toBe(before)
    })
})

/*
  Story 7.4, driven through the real hook rather than the reducer directly.

  state.test.ts proves the reducer's transition retains metadata and the
  final snapshot; this proves the retained values actually reach a real
  useTransfer consumer -- through the real Wails event listeners, a real
  stage() round trip, and a real transfer-reset -- rather than only living in
  an isolated reducer call.
*/
describe('the completion receipt reaches a live consumer (Story 7.4)', () => {
    it('carries the retained receipt (name, isDir, wire bytes sent) on a live Done outcome', async () => {
        mocks.stageTransfer.mockResolvedValue(metadata({name: 'report.pdf'}))
        const hook = renderHook(() => useTransfer())

        await act(async () => { await hook.result.current.stage('C:\\report.pdf') })
        act(() => {
            emit('transfer-started', {sessionId, seq: 1})
            emit('transfer-complete', {sessionId, seq: 2, progress: progress(100)})
        })

        const state = hook.result.current.state
        expect(state.phase).toBe('done')
        expect(state.phase === 'done' ? state.outcome.receipt : null).toEqual({
            name: 'report.pdf', isDir: false, bytesSent: 100,
        })
    })

    it('keeps the same receipt once transfer-reset retains the outcome in Idle', async () => {
        mocks.stageTransfer.mockResolvedValue(metadata({name: 'report.pdf'}))
        const hook = renderHook(() => useTransfer())

        await act(async () => { await hook.result.current.stage('C:\\report.pdf') })
        act(() => {
            emit('transfer-started', {sessionId, seq: 1})
            emit('transfer-complete', {sessionId, seq: 2, progress: progress(100)})
        })
        const live = hook.result.current.state
        expect(live.phase).toBe('done')
        const liveReceipt = live.phase === 'done' ? live.outcome.receipt : null

        act(() => emit('transfer-reset', {sessionId, seq: 3}))

        const retained = hook.result.current.state
        expect(retained.phase).toBe('idle')
        const retainedReceipt = retained.phase === 'idle' && retained.retainedOutcome?.kind === 'done'
            ? retained.retainedOutcome.receipt
            : null

        // Given a retained Done outcome in Idle, it carries the same receipt
        // -- reset must not empty the panel the sender is looking at.
        expect(retainedReceipt).toEqual(liveReceipt)
        expect(retainedReceipt).not.toBeNull()
    })

    /*
      The product's ephemerality contract, proven through the real hook: the
      capability URL and its QR code must not survive into the retained Idle
      outcome, even though the full FileMetadata the hook received from
      StageTransfer (via the mocked wire) carried both.
    */
    it('never carries the capability URL or QR code into the retained outcome', async () => {
        mocks.stageTransfer.mockResolvedValue(metadata({
            name: 'report.pdf',
            url: 'http://192.0.2.1:34123/download/fedcba9876543210fedcba9876543210',
            qrBase64: qrPNG,
        }))
        const hook = renderHook(() => useTransfer())

        await act(async () => { await hook.result.current.stage('C:\\report.pdf') })
        act(() => {
            emit('transfer-started', {sessionId, seq: 1})
            emit('transfer-complete', {sessionId, seq: 2, progress: progress(100)})
            emit('transfer-reset', {sessionId, seq: 3})
        })

        const serialized = JSON.stringify(hook.result.current.state)
        expect(serialized, 'the capability URL must not outlive the session').not.toContain(
            'fedcba9876543210fedcba9876543210',
        )
        expect(serialized, 'the QR code must not outlive the session').not.toContain(qrPNG)
    })
})

/*
  Story 9.2: the controller remembers the absolute path it last passed to
  StageTransfer -- in JS memory only -- so a failed transfer can retry the
  same item without reopening the chooser.
*/
describe('remembering the staged item for retry (Story 9.2)', () => {
    const rememberedPath = String.raw`C:\Users\jaeson\Shared Folder\secret-report.pdf`

    async function driveToLiveError(
        hook: {result: {current: ReturnType<typeof useTransfer>}},
        path = rememberedPath,
    ) {
        mocks.stageTransfer.mockResolvedValueOnce(metadata())
        await act(async () => { await hook.result.current.stage(path) })
        act(() => {
            emit('transfer-started', {sessionId, seq: 1})
            emit('transfer-error', {sessionId, seq: 2, error: {code: 'transfer_failed', message: 'x'}})
        })
    }

    it('reports canRetry once a Stage succeeds, before any terminal outcome exists', async () => {
        mocks.stageTransfer.mockResolvedValueOnce(metadata())
        const hook = renderHook(() => useTransfer())

        expect(hook.result.current.canRetry, 'nothing has been staged yet').toBe(false)
        await act(async () => { await hook.result.current.stage(rememberedPath) })
        expect(hook.result.current.canRetry).toBe(true)
    })

    it('does nothing when retry() is called with nothing remembered', async () => {
        const hook = renderHook(() => useTransfer())

        await act(async () => { await hook.result.current.retry() })

        expect(mocks.stageTransfer).not.toHaveBeenCalled()
        expect(mocks.cancelTransfer).not.toHaveBeenCalled()
    })

    /*
      *Mutation:* persist the remembered path anywhere outside JS memory
      (localStorage, a log line) or render it -> the assertions below must
      fail. `localStorage` is spied directly; "never rendered" is proven the
      only way it can be from this layer -- the path never appears anywhere
      in the serialized `TransferState`, even while retry() can still use it.
    */
    it('never persists or renders the remembered path outside JS memory', async () => {
        const setItem = vi.spyOn(Storage.prototype, 'setItem')
        const hook = renderHook(() => useTransfer())
        await driveToLiveError(hook)

        const serialized = JSON.stringify(hook.result.current.state)
        // JSON.stringify escapes backslashes (`\` -> `\\`), so a Windows path
        // like `rememberedPath` never appears in serialized text verbatim --
        // comparing against its own JSON-escaped form is what makes this
        // assertion actually exercise the claim instead of trivially passing.
        const escapedPath = JSON.stringify(rememberedPath).slice(1, -1)
        expect(serialized, 'the remembered path must never appear in TransferState').not.toContain(escapedPath)
        expect(setItem, 'the remembered path must never reach localStorage').not.toHaveBeenCalled()

        // Still usable by retry() despite never being visible above -- proving
        // it really was remembered, just never exposed.
        mocks.cancelTransfer.mockResolvedValue(undefined)
        mocks.stageTransfer.mockResolvedValueOnce(metadata())
        let retryPromise!: Promise<void>
        act(() => { retryPromise = hook.result.current.retry() })
        act(() => emit('transfer-reset', {sessionId, seq: 3}))
        await act(async () => { await retryPromise })

        expect(mocks.stageTransfer).toHaveBeenLastCalledWith(rememberedPath)
        setItem.mockRestore()
    })

    it('replaces the remembered path with the next Stage', async () => {
        const hook = renderHook(() => useTransfer())
        await driveToLiveError(hook, String.raw`C:\first.pdf`)
        expect(hook.result.current.canRetry).toBe(true)

        // A fresh Stage from Idle (as "Choose Another" would issue) replaces
        // the remembered target, even though the previous one is still
        // showing as a retained/live outcome's error at this point.
        mocks.cancelTransfer.mockResolvedValue(undefined)
        mocks.stageTransfer.mockResolvedValueOnce(metadata())
        let stagePromise!: Promise<void>
        act(() => { stagePromise = hook.result.current.stageFromOutcome(String.raw`C:\second.pdf`) })
        act(() => emit('transfer-reset', {sessionId, seq: 3}))
        await act(async () => { await stagePromise })
        expect(hook.result.current.state.phase).toBe('staged')

        act(() => {
            emit('transfer-started', {sessionId, seq: 1})
            emit('transfer-error', {sessionId, seq: 2, error: {code: 'transfer_failed', message: 'x'}})
        })
        mocks.stageTransfer.mockClear()
        mocks.stageTransfer.mockResolvedValueOnce(metadata())
        mocks.cancelTransfer.mockClear()
        mocks.cancelTransfer.mockResolvedValue(undefined)
        let retryPromise!: Promise<void>
        act(() => { retryPromise = hook.result.current.retry() })
        act(() => emit('transfer-reset', {sessionId, seq: 3}))
        await act(async () => { await retryPromise })

        expect(mocks.stageTransfer).toHaveBeenCalledWith(String.raw`C:\second.pdf`)
        expect(mocks.stageTransfer).not.toHaveBeenCalledWith(String.raw`C:\first.pdf`)
    })

    it('clears the remembered path on Dismiss', async () => {
        const hook = renderHook(() => useTransfer())
        await driveToLiveError(hook)
        mocks.cancelTransfer.mockResolvedValue(undefined)
        let cancelPromise!: Promise<void>
        act(() => { cancelPromise = hook.result.current.cancel() })
        act(() => emit('transfer-reset', {sessionId, seq: 3}))
        await act(async () => { await cancelPromise })
        expect(hook.result.current.state.phase).toBe('idle')
        expect(hook.result.current.canRetry, 'still remembered while retained').toBe(true)

        act(() => { hook.result.current.dismissRetained() })

        expect(hook.result.current.canRetry).toBe(false)
        mocks.stageTransfer.mockClear()
        await act(async () => { await hook.result.current.retry() })
        expect(mocks.stageTransfer).not.toHaveBeenCalled()
    })

    it('clears the remembered path on a completed Done', async () => {
        mocks.stageTransfer.mockResolvedValueOnce(metadata())
        const hook = renderHook(() => useTransfer())
        await act(async () => { await hook.result.current.stage(rememberedPath) })
        expect(hook.result.current.canRetry).toBe(true)

        act(() => {
            emit('transfer-started', {sessionId, seq: 1})
            emit('transfer-complete', {sessionId, seq: 2, progress: progress(100)})
        })

        expect(hook.result.current.state.phase).toBe('done')
        expect(hook.result.current.canRetry, 'a completed Done needs no retry target').toBe(false)
    })

    it('clears the remembered path when a Stage fails with a non-retry code, keeps it for a retry code', async () => {
        const hook = renderHook(() => useTransfer())

        mocks.stageTransfer.mockRejectedValueOnce(new Error(JSON.stringify({
            code: 'path_not_found', message: 'gone',
        })))
        await act(async () => { await hook.result.current.stage(String.raw`C:\choose-again.pdf`) })
        expect(hook.result.current.canRetry, 'path_not_found is a choose-action code').toBe(false)

        mocks.stageTransfer.mockRejectedValueOnce(new Error(JSON.stringify({
            code: 'network_unavailable', message: 'no network',
        })))
        await act(async () => { await hook.result.current.stage(String.raw`C:\retry-me.pdf`) })
        expect(hook.result.current.canRetry, 'network_unavailable is a retry-action code').toBe(true)
    })

    /*
      Story 9.2 AC3: retry() consults the current error's action -- a
      remembered path is not enough on its own. `transfer-error`'s own code
      travels with the event, not through `dispatchStageFailed`'s clearing
      rule (that rule is specific to a *Stage command* failing), so this
      constructs the one case where a path is still remembered but the live
      error's code maps to `choose`, to prove retry() checks the action and
      not just "is something remembered".
    */
    it('does nothing when the current error\'s action is not retry, even with a path remembered', async () => {
        const hook = renderHook(() => useTransfer())
        await driveToLiveError(hook)
        expect(hook.result.current.canRetry, 'transfer_failed is a retry-action code').toBe(true)

        // Re-drive the same live session to a different terminal error whose
        // code maps to `choose` -- transfer-error's own code is not run
        // through the Stage-failure clearing rule, so the remembered path is
        // untouched by this transition.
        mocks.stageTransfer.mockResolvedValueOnce(metadata())
        mocks.cancelTransfer.mockResolvedValue(undefined)
        let firstRetry!: Promise<void>
        act(() => { firstRetry = hook.result.current.retry() })
        act(() => emit('transfer-reset', {sessionId, seq: 3}))
        await act(async () => { await firstRetry })
        expect(hook.result.current.state.phase).toBe('staged')
        act(() => emit('transfer-started', {sessionId, seq: 1}))
        act(() => emit('transfer-error', {sessionId, seq: 2, error: {code: 'path_not_found', message: 'gone'}}))

        expect(hook.result.current.state.phase).toBe('error')
        expect(hook.result.current.canRetry, 'the remembered path itself is untouched by this transition').toBe(true)

        mocks.stageTransfer.mockClear()
        mocks.cancelTransfer.mockClear()
        await act(async () => { await hook.result.current.retry() })

        expect(mocks.stageTransfer, 'path_not_found maps to choose, not retry').not.toHaveBeenCalled()
        expect(mocks.cancelTransfer).not.toHaveBeenCalled()
    })

    /*
      Story 9.2 AC4, driven through the real hook: retry() from a *live*
      terminal Error must release the backend's ~3s lease first (D-059) --
      exactly the `cancel()` Dismiss already uses -- and wait for the actual
      transfer-reset transition to Idle before staging, so the attempt never
      surfaces `busy` for a session the sender already considers finished.
    */
    it('releases the live lease before retrying, and stages exactly once with the remembered path', async () => {
        const hook = renderHook(() => useTransfer())
        await driveToLiveError(hook)
        expect(hook.result.current.state.phase).toBe('error')

        const cancelDeferred = deferred<void>()
        mocks.cancelTransfer.mockReturnValueOnce(cancelDeferred.promise)
        mocks.stageTransfer.mockClear()
        mocks.stageTransfer.mockResolvedValueOnce(metadata())

        let retryPromise!: Promise<void>
        act(() => { retryPromise = hook.result.current.retry() })
        await waitFor(() => expect(mocks.cancelTransfer).toHaveBeenCalledTimes(1))

        // Cancel has been issued but not yet settled, and no reset has
        // arrived: staging now would risk `busy`, so nothing must have been
        // staged yet.
        expect(mocks.stageTransfer).not.toHaveBeenCalled()
        expect(hook.result.current.state.phase).toBe('error')

        await act(async () => { cancelDeferred.resolve() })
        // The Cancel command settling is not enough on its own -- the reset
        // event is a separate, later message, and retry() must still be
        // waiting for it.
        expect(mocks.stageTransfer).not.toHaveBeenCalled()

        act(() => emit('transfer-reset', {sessionId, seq: 3}))
        await act(async () => { await retryPromise })

        expect(mocks.stageTransfer).toHaveBeenCalledTimes(1)
        expect(mocks.stageTransfer).toHaveBeenCalledWith(rememberedPath)
        expect(hook.result.current.state.phase).toBe('staged')
    })

    it('retries directly from a retained (already Idle) Error with no Cancel call at all', async () => {
        const hook = renderHook(() => useTransfer())
        await driveToLiveError(hook)
        mocks.cancelTransfer.mockResolvedValue(undefined)
        let cancelPromise!: Promise<void>
        act(() => { cancelPromise = hook.result.current.cancel() })
        act(() => emit('transfer-reset', {sessionId, seq: 3}))
        await act(async () => { await cancelPromise })
        expect(hook.result.current.state.phase).toBe('idle')
        mocks.cancelTransfer.mockClear()
        mocks.stageTransfer.mockClear()
        mocks.stageTransfer.mockResolvedValueOnce(metadata())

        await act(async () => { await hook.result.current.retry() })

        expect(mocks.cancelTransfer, 'nothing live to release from Idle').not.toHaveBeenCalled()
        expect(mocks.stageTransfer).toHaveBeenCalledTimes(1)
        expect(mocks.stageTransfer).toHaveBeenCalledWith(rememberedPath)
        expect(hook.result.current.state.phase).toBe('staged')
    })

    it('retries a Stage-time command failure with a retry-action code directly from Idle', async () => {
        const hook = renderHook(() => useTransfer())
        mocks.stageTransfer.mockRejectedValueOnce(new Error(JSON.stringify({
            code: 'busy', message: 'still finishing',
        })))
        await act(async () => { await hook.result.current.stage(rememberedPath) })
        expect(hook.result.current.state).toMatchObject({phase: 'idle', commandError: {code: 'busy'}})
        expect(hook.result.current.canRetry).toBe(true)

        mocks.stageTransfer.mockResolvedValueOnce(metadata())
        await act(async () => { await hook.result.current.retry() })

        expect(mocks.cancelTransfer, 'Idle never holds a lease to release').not.toHaveBeenCalled()
        expect(mocks.stageTransfer).toHaveBeenCalledWith(rememberedPath)
        expect(hook.result.current.state.phase).toBe('staged')
    })
})

/*
  Story 9.2: `stageFromOutcome` is the primitive Story 9.6 will call for
  "Send Another"/"Choose Another" and for a drop on a live outcome card --
  the same lease-release-then-stage behaviour retry() uses, for an
  arbitrary new path rather than the remembered one.
*/
describe('stageFromOutcome releases a live lease before staging a new item (Story 9.2)', () => {
    it('stages immediately when no terminal outcome is live', async () => {
        mocks.stageTransfer.mockResolvedValueOnce(metadata())
        const hook = renderHook(() => useTransfer())

        await act(async () => { await hook.result.current.stageFromOutcome('C:\\fresh.pdf') })

        expect(mocks.cancelTransfer).not.toHaveBeenCalled()
        expect(mocks.stageTransfer).toHaveBeenCalledWith('C:\\fresh.pdf')
        expect(hook.result.current.state.phase).toBe('staged')
    })

    it('releases a live Done outcome\'s lease before staging a new item, waiting for the reset', async () => {
        mocks.stageTransfer.mockResolvedValueOnce(metadata())
        const hook = renderHook(() => useTransfer())
        await act(async () => { await hook.result.current.stage('C:\\first.pdf') })
        act(() => {
            emit('transfer-started', {sessionId, seq: 1})
            emit('transfer-complete', {sessionId, seq: 2, progress: progress(100)})
        })
        expect(hook.result.current.state.phase).toBe('done')

        mocks.cancelTransfer.mockResolvedValue(undefined)
        mocks.stageTransfer.mockResolvedValueOnce(metadata())
        let stagePromise!: Promise<void>
        act(() => { stagePromise = hook.result.current.stageFromOutcome('C:\\second.pdf') })
        await waitFor(() => expect(mocks.cancelTransfer).toHaveBeenCalledTimes(1))
        expect(mocks.stageTransfer).toHaveBeenCalledTimes(1) // only the first stage so far

        act(() => emit('transfer-reset', {sessionId, seq: 3}))
        await act(async () => { await stagePromise })

        expect(mocks.stageTransfer).toHaveBeenCalledTimes(2)
        expect(mocks.stageTransfer).toHaveBeenLastCalledWith('C:\\second.pdf')
        expect(hook.result.current.state.phase).toBe('staged')
    })
})
