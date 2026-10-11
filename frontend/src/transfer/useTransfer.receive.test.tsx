import {act, renderHook} from '@testing-library/react'
import {beforeEach, describe, expect, it, vi} from 'vitest'
import {useTransfer} from './useTransfer'

/*
  The receive controller path (spec-phone-to-desktop-receiving, Story B): the
  folder chooser, the single-flight rules, the cancel and unmount invalidation,
  the held outcome, Show in Folder, and the guarantee that no send affordance
  attaches to a receive session.

  Kept beside useTransfer.test.tsx rather than inside it so the send controller's
  own suite stays byte-identical -- the send flows' proof is that suite passing
  unchanged, not a copy of it here.
*/

const mocks = vi.hoisted(() => ({
    stageTransfer: vi.fn(),
    stageTransfers: vi.fn(),
    cancelTransfer: vi.fn(),
    selectFile: vi.fn(),
    selectDirectory: vi.fn(),
    selectFiles: vi.fn(),
    selectReceiveFolder: vi.fn(),
    startReceive: vi.fn(),
    showReceivedFolder: vi.fn(),
    eventsOn: vi.fn(),
}))

vi.mock('../../wailsjs/go/main/App', () => ({
    StageTransfer: mocks.stageTransfer,
    StageTransfers: mocks.stageTransfers,
    CancelTransfer: mocks.cancelTransfer,
    SelectFile: mocks.selectFile,
    SelectDirectory: mocks.selectDirectory,
    SelectFiles: mocks.selectFiles,
    SelectReceiveFolder: mocks.selectReceiveFolder,
    StartReceive: mocks.startReceive,
    ShowReceivedFolder: mocks.showReceivedFolder,
}))

vi.mock('../../wailsjs/runtime/runtime', () => ({EventsOn: mocks.eventsOn}))

interface Subscription {
    readonly name: string
    readonly callback: (...args: unknown[]) => void
    active: boolean
}

let subscriptions: Subscription[]
const sessionId = '0102030405060708090a0b0c0d0e0f10'
const qrPNG = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII='

function receiveMetadata(overrides: Record<string, unknown> = {}): Record<string, unknown> {
    return {
        sessionId,
        destination: 'Phone Photos',
        url: 'http://192.168.1.50:45678/upload/1112131415161718191a1b1c1d1e1f20',
        qrBase64: qrPNG,
        warnings: [],
        ...overrides,
    }
}

function progress(bytesSent: number): Record<string, unknown> {
    return {bytesSent, totalBytes: 400, totalKnown: true, percent: bytesSent / 4, speedBytesPerSec: 10}
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

function emit(name: string, body: Record<string, unknown>): void {
    const subscription = subscriptions.findLast((candidate) => candidate.name === name && candidate.active)
    expect(subscription, `live ${name} subscription`).toBeTruthy()
    subscription!.callback({sessionId, ...body})
}

beforeEach(() => {
    subscriptions = []
    for (const mock of Object.values(mocks)) mock.mockReset()
    mocks.cancelTransfer.mockResolvedValue(undefined)
    mocks.showReceivedFolder.mockResolvedValue(undefined)
    mocks.eventsOn.mockImplementation((name: string, callback: (...args: unknown[]) => void) => {
        const subscription: Subscription = {name, callback, active: true}
        subscriptions.push(subscription)
        return () => { subscription.active = false }
    })
})

/** Mounts the controller and walks one receive session to the waiting view. */
async function waitingController() {
    mocks.selectReceiveFolder.mockResolvedValue('/Users/someone/Phone Photos')
    mocks.startReceive.mockResolvedValue(receiveMetadata())
    const hook = renderHook(() => useTransfer())
    await act(async () => { await hook.result.current.receive() })
    expect(hook.result.current.state.phase).toBe('receive-waiting')
    return hook
}

async function outcomeController(result: 'complete' | 'incomplete' | 'cancelled', status: {filesSaved: number; subfolderExists: boolean; markingWarning?: boolean}) {
    const hook = await waitingController()
    const receive = {filesSaved: status.filesSaved, result, subfolderExists: status.subfolderExists, markingWarning: status.markingWarning ?? false}
    act(() => {
        emit('transfer-started', {seq: 1})
        emit('transfer-progress', {seq: 2, progress: progress(400), receive: {filesSaved: status.filesSaved, subfolderExists: false, markingWarning: false}})
    })
    act(() => {
        if (result === 'complete') emit('transfer-complete', {seq: 3, progress: progress(400), receive})
        else emit('transfer-error', {seq: 3, error: {code: result === 'cancelled' ? 'cancelled' : 'transfer_failed', message: 'x'}, receive})
    })
    expect(hook.result.current.state.phase).toBe('receive-outcome')
    return hook
}

describe('activating Receive', () => {
    it('does nothing when the chooser is dismissed: no session, no command, no error', async () => {
        mocks.selectReceiveFolder.mockResolvedValue('')
        const hook = renderHook(() => useTransfer())
        const before = hook.result.current.state

        let result: unknown
        await act(async () => { result = await hook.result.current.receive() })

        expect(result).toBe('dismissed')
        expect(mocks.selectReceiveFolder).toHaveBeenCalledTimes(1)
        expect(mocks.startReceive).not.toHaveBeenCalled()
        expect(mocks.cancelTransfer).not.toHaveBeenCalled()
        expect(hook.result.current.state).toBe(before)
    })

    it('treats a whitespace-only result like a dismissal', async () => {
        mocks.selectReceiveFolder.mockResolvedValue('   ')
        const hook = renderHook(() => useTransfer())
        let result: unknown
        await act(async () => { result = await hook.result.current.receive() })
        expect(result).toBe('dismissed')
        expect(mocks.startReceive).not.toHaveBeenCalled()
    })

    it('can be activated again after a dismissal', async () => {
        mocks.selectReceiveFolder.mockResolvedValueOnce('')
        const hook = renderHook(() => useTransfer())
        await act(async () => { await hook.result.current.receive() })

        mocks.selectReceiveFolder.mockResolvedValueOnce('/Users/someone/Phone Photos')
        mocks.startReceive.mockResolvedValue(receiveMetadata())
        await act(async () => { await hook.result.current.receive() })
        expect(hook.result.current.state.phase).toBe('receive-waiting')
    })

    it('starts one session into exactly the chosen folder, passed through untouched', async () => {
        const folder = String.raw`C:\Users\someone\Phone Photos `
        mocks.selectReceiveFolder.mockResolvedValue(folder)
        mocks.startReceive.mockResolvedValue(receiveMetadata())
        const hook = renderHook(() => useTransfer())

        let result: unknown
        await act(async () => { result = await hook.result.current.receive() })

        expect(result).toBe('proceeded')
        expect(mocks.startReceive).toHaveBeenCalledTimes(1)
        expect(mocks.startReceive).toHaveBeenCalledWith(folder)
        expect(hook.result.current.state).toMatchObject({
            phase: 'receive-waiting',
            session: {sessionId, lastSeq: 0},
            metadata: {destination: 'Phone Photos'},
        })
    })

    it('shows the pending view while the session starts, and not before the chooser returns', async () => {
        const chooser = deferred<string>()
        const starting = deferred<Record<string, unknown>>()
        mocks.selectReceiveFolder.mockReturnValue(chooser.promise)
        mocks.startReceive.mockReturnValue(starting.promise)
        const hook = renderHook(() => useTransfer())

        let activation!: Promise<unknown>
        act(() => { activation = hook.result.current.receive() })
        expect(hook.result.current.state.phase).toBe('idle')

        await act(async () => { chooser.resolve('/Users/someone/Phone Photos'); await Promise.resolve() })
        expect(hook.result.current.state).toMatchObject({phase: 'pending', itemKind: 'receive'})

        await act(async () => { starting.resolve(receiveMetadata()); await activation })
        expect(hook.result.current.state.phase).toBe('receive-waiting')
    })
})

describe('single flight', () => {
    it('refuses a second activation while the chooser is open', async () => {
        const chooser = deferred<string>()
        mocks.selectReceiveFolder.mockReturnValue(chooser.promise)
        mocks.startReceive.mockResolvedValue(receiveMetadata())
        const hook = renderHook(() => useTransfer())

        let first!: Promise<unknown>
        let second!: Promise<unknown>
        act(() => {
            first = hook.result.current.receive()
            second = hook.result.current.receive()
        })

        await expect(second).resolves.toBe('ignored')
        expect(mocks.selectReceiveFolder).toHaveBeenCalledTimes(1)

        await act(async () => { chooser.resolve('/Users/someone/Phone Photos'); await first })
        expect(mocks.startReceive).toHaveBeenCalledTimes(1)
    })

    it('refuses a second activation while the session is starting', async () => {
        const starting = deferred<Record<string, unknown>>()
        mocks.selectReceiveFolder.mockResolvedValue('/Users/someone/Phone Photos')
        mocks.startReceive.mockReturnValue(starting.promise)
        const hook = renderHook(() => useTransfer())

        let first!: Promise<unknown>
        act(() => { first = hook.result.current.receive() })
        await act(async () => { await Promise.resolve(); await Promise.resolve() })
        expect(hook.result.current.state.phase).toBe('pending')

        let second: unknown
        await act(async () => { second = await hook.result.current.receive() })
        expect(second).toBe('ignored')
        expect(mocks.selectReceiveFolder).toHaveBeenCalledTimes(1)

        await act(async () => { starting.resolve(receiveMetadata()); await first })
        expect(mocks.startReceive).toHaveBeenCalledTimes(1)
    })

    it('refuses Receive outside Idle', async () => {
        const hook = await waitingController()
        mocks.selectReceiveFolder.mockClear()

        let result: unknown
        await act(async () => { result = await hook.result.current.receive() })

        expect(result).toBe('ignored')
        expect(mocks.selectReceiveFolder).not.toHaveBeenCalled()
        expect(hook.result.current.state.phase).toBe('receive-waiting')
    })

    it('refuses Receive while a send chooser is open, and a send chooser while Receive is', async () => {
        const sendChooser = deferred<string>()
        mocks.selectFile.mockReturnValue(sendChooser.promise)
        const hook = renderHook(() => useTransfer())

        let sending!: Promise<void>
        act(() => { sending = hook.result.current.selectFile() })
        let result: unknown
        await act(async () => { result = await hook.result.current.receive() })
        expect(result).toBe('ignored')
        expect(mocks.selectReceiveFolder).not.toHaveBeenCalled()
        await act(async () => { sendChooser.resolve(''); await sending })

        const receiveChooser = deferred<string>()
        mocks.selectReceiveFolder.mockReturnValue(receiveChooser.promise)
        let receiving!: Promise<unknown>
        act(() => { receiving = hook.result.current.receive() })
        await act(async () => { await hook.result.current.selectFile() })
        expect(mocks.selectFile).toHaveBeenCalledTimes(1)
        await act(async () => { receiveChooser.resolve(''); await receiving })
    })
})

describe('invalidated chooser completions', () => {
    it('ignores a folder chosen after unmount', async () => {
        const chooser = deferred<string>()
        mocks.selectReceiveFolder.mockReturnValue(chooser.promise)
        const hook = renderHook(() => useTransfer())

        let activation!: Promise<unknown>
        act(() => { activation = hook.result.current.receive() })
        hook.unmount()
        await act(async () => { chooser.resolve('/Users/someone/late'); await activation })

        expect(mocks.startReceive).not.toHaveBeenCalled()
    })

    it('ignores a folder chosen after Cancel was activated', async () => {
        const chooser = deferred<string>()
        mocks.selectReceiveFolder.mockReturnValue(chooser.promise)
        const hook = renderHook(() => useTransfer())

        let activation!: Promise<unknown>
        act(() => { activation = hook.result.current.receive() })
        await act(async () => { await hook.result.current.cancel() })
        await act(async () => { chooser.resolve('/Users/someone/late'); await activation })

        expect(mocks.startReceive).not.toHaveBeenCalled()
        expect(hook.result.current.state.phase).toBe('idle')
    })

    it('ignores a chooser failure that arrives after unmount', async () => {
        const chooser = deferred<string>()
        mocks.selectReceiveFolder.mockReturnValue(chooser.promise)
        const hook = renderHook(() => useTransfer())

        let activation!: Promise<unknown>
        act(() => { activation = hook.result.current.receive() })
        hook.unmount()
        await act(async () => { chooser.reject(JSON.stringify({code: 'chooser_failed', message: 'x'})); await activation })
        expect(mocks.startReceive).not.toHaveBeenCalled()
    })

    it('does not commit a session whose start finished after unmount, and quiesces it', async () => {
        const starting = deferred<Record<string, unknown>>()
        mocks.selectReceiveFolder.mockResolvedValue('/Users/someone/Phone Photos')
        mocks.startReceive.mockReturnValue(starting.promise)
        const hook = renderHook(() => useTransfer())

        let activation!: Promise<unknown>
        act(() => { activation = hook.result.current.receive() })
        await act(async () => { await Promise.resolve(); await Promise.resolve() })
        hook.unmount()
        await act(async () => { starting.resolve(receiveMetadata()); await activation })

        expect(mocks.cancelTransfer).toHaveBeenCalledTimes(1)
    })
})

describe('refusals', () => {
    it('shows a chooser failure as a receive-context command error with fixed copy', async () => {
        mocks.selectReceiveFolder.mockRejectedValue(JSON.stringify({code: 'chooser_failed', message: 'C:\\secret\\path'}))
        const hook = renderHook(() => useTransfer())

        await act(async () => { await hook.result.current.receive() })

        expect(hook.result.current.state).toEqual({
            phase: 'idle',
            retainedOutcome: null,
            commandError: {
                code: 'chooser_failed',
                message: 'FairDrop couldn’t open the chooser. Try again, or drop the item on the zone above.',
            },
            commandContext: 'receive',
        })
        expect(JSON.stringify(hook.result.current.state)).not.toContain('secret')
    })

    it.each([
        ['path_not_found'],
        ['path_unsupported'],
        ['busy'],
        ['network_unavailable'],
        ['server_start_failed'],
        ['setup_failed'],
    ])('shows a StartReceive rejection of %s as a receive-context command error', async (code) => {
        mocks.selectReceiveFolder.mockResolvedValue('/Users/someone/Phone Photos')
        mocks.startReceive.mockRejectedValue(JSON.stringify({code, message: '/Users/someone/Phone Photos is not writable'}))
        const hook = renderHook(() => useTransfer())

        await act(async () => { await hook.result.current.receive() })

        expect(hook.result.current.state).toMatchObject({phase: 'idle', commandError: {code}, commandContext: 'receive'})
        expect(JSON.stringify(hook.result.current.state)).not.toContain('/Users/someone')
        expect(hook.result.current.canRetry).toBe(false)
        expect(hook.result.current.canSendAgain).toBe(false)
    })

    it('maps an unrecognised rejection to transfer_failed rather than showing its text', async () => {
        mocks.selectReceiveFolder.mockResolvedValue('/Users/someone/Phone Photos')
        mocks.startReceive.mockRejectedValue(new Error('timeout from the runtime'))
        const hook = renderHook(() => useTransfer())

        await act(async () => { await hook.result.current.receive() })
        expect(hook.result.current.state).toMatchObject({commandError: {code: 'transfer_failed'}, commandContext: 'receive'})
    })

    it('refuses a malformed acknowledgement, quiesces the backend once, and says nothing started', async () => {
        mocks.selectReceiveFolder.mockResolvedValue('/Users/someone/Phone Photos')
        mocks.startReceive.mockResolvedValue(receiveMetadata({url: 'http://192.168.1.50:45678/download/1112131415161718191a1b1c1d1e1f20'}))
        const hook = renderHook(() => useTransfer())

        await act(async () => { await hook.result.current.receive() })

        expect(mocks.cancelTransfer).toHaveBeenCalledTimes(1)
        expect(hook.result.current.state).toMatchObject({
            phase: 'idle', commandError: {code: 'setup_failed'}, commandContext: 'receive',
        })
    })

    it('says the connection could not be confirmed released when the quiescing cancel also fails', async () => {
        mocks.selectReceiveFolder.mockResolvedValue('/Users/someone/Phone Photos')
        mocks.startReceive.mockResolvedValue({sessionId: 'nope'})
        mocks.cancelTransfer.mockRejectedValue(new Error('adapter text'))
        const hook = renderHook(() => useTransfer())

        await act(async () => { await hook.result.current.receive() })

        expect(hook.result.current.state).toMatchObject({commandError: {code: 'cleanup_unconfirmed'}, commandContext: 'receive'})
    })

    it('treats a cancelled start as a quiet return to Idle', async () => {
        const starting = deferred<Record<string, unknown>>()
        mocks.selectReceiveFolder.mockResolvedValue('/Users/someone/Phone Photos')
        mocks.startReceive.mockReturnValue(starting.promise)
        const hook = renderHook(() => useTransfer())

        let activation!: Promise<unknown>
        act(() => { activation = hook.result.current.receive() })
        await act(async () => { await Promise.resolve(); await Promise.resolve() })
        expect(hook.result.current.state.phase).toBe('pending')

        await act(async () => {
            const cancelling = hook.result.current.cancel()
            starting.reject(JSON.stringify({code: 'cancelled', message: 'x'}))
            await Promise.all([cancelling, activation])
        })

        expect(mocks.cancelTransfer).toHaveBeenCalledTimes(1)
        expect(hook.result.current.state).toEqual({phase: 'idle', retainedOutcome: null, commandError: null})
    })

    it('ignores a start that succeeds after the user cancelled it', async () => {
        const starting = deferred<Record<string, unknown>>()
        mocks.selectReceiveFolder.mockResolvedValue('/Users/someone/Phone Photos')
        mocks.startReceive.mockReturnValue(starting.promise)
        const hook = renderHook(() => useTransfer())

        let activation!: Promise<unknown>
        act(() => { activation = hook.result.current.receive() })
        await act(async () => { await Promise.resolve(); await Promise.resolve() })

        await act(async () => {
            const cancelling = hook.result.current.cancel()
            starting.resolve(receiveMetadata())
            await Promise.all([cancelling, activation])
        })

        expect(hook.result.current.state.phase).toBe('idle')
    })
})

describe('the session lifecycle', () => {
    it('counts refused uploads and keeps waiting', async () => {
        const hook = await waitingController()

        act(() => emit('transfer-notice', {seq: 1, notice: 'receive_too_large'}))
        act(() => emit('transfer-notice', {seq: 2, notice: 'receive_too_large'}))

        expect(hook.result.current.state).toMatchObject({phase: 'receive-waiting', refusals: 2})
    })

    it('walks waiting, receiving and a complete outcome with the backend count', async () => {
        const hook = await waitingController()

        act(() => emit('transfer-started', {seq: 1}))
        expect(hook.result.current.state).toMatchObject({phase: 'receive-receiving', filesSaved: 0})

        act(() => emit('transfer-progress', {seq: 2, progress: progress(200), receive: {filesSaved: 1, subfolderExists: false, markingWarning: false}}))
        expect(hook.result.current.state).toMatchObject({phase: 'receive-receiving', filesSaved: 1, progress: {bytesSent: 200}})

        act(() => emit('transfer-complete', {
            seq: 3, progress: progress(400),
            receive: {filesSaved: 3, result: 'complete', subfolderExists: true, markingWarning: false},
        }))
        expect(hook.result.current.state).toMatchObject({
            phase: 'receive-outcome',
            outcome: {kind: 'complete', filesSaved: 3, subfolderExists: true},
        })
    })

    it('ends a waiting session with an incomplete outcome when the backend closes it before any phone claimed it', async () => {
        const hook = await waitingController()

        // The wire shape Go publishes when the server lane closes first
        // (TestALaneThatClosesBeforeAnyClaimFailsAWaitingReceive), through the real parser.
        act(() => emit('transfer-error', {
            seq: 1, error: {code: 'transfer_failed', message: 'x'},
            receive: {filesSaved: 0, result: 'incomplete', subfolderExists: false, markingWarning: false},
        }))

        expect(hook.result.current.state).toMatchObject({
            phase: 'receive-outcome',
            destination: 'Phone Photos',
            outcome: {kind: 'incomplete', filesSaved: 0, subfolderExists: false},
        })
        await act(async () => { await hook.result.current.showReceivedFolder() })
        expect(mocks.showReceivedFolder).not.toHaveBeenCalled()
    })

    it('holds the outcome: nothing resets it until the user leaves', async () => {
        const hook = await outcomeController('complete', {filesSaved: 3, subfolderExists: true})
        const held = hook.result.current.state

        // No timer, no event: the outcome simply stays.
        await act(async () => { await new Promise((resolve) => setTimeout(resolve, 20)) })
        expect(hook.result.current.state).toBe(held)
    })

    it('leaves the outcome through the existing Cancel command, and only the reset returns to Idle', async () => {
        const hook = await outcomeController('complete', {filesSaved: 3, subfolderExists: true})

        await act(async () => { await hook.result.current.cancel() })
        expect(mocks.cancelTransfer).toHaveBeenCalledTimes(1)
        expect(hook.result.current.state.phase).toBe('receive-outcome')

        act(() => emit('transfer-reset', {seq: 4}))
        expect(hook.result.current.state).toEqual({phase: 'idle', retainedOutcome: null, commandError: null})
    })

    it('does not issue a second Cancel while the first is outstanding', async () => {
        const hook = await outcomeController('complete', {filesSaved: 3, subfolderExists: true})
        const release = deferred<void>()
        mocks.cancelTransfer.mockReturnValue(release.promise)

        let first!: Promise<void>
        let second!: Promise<void>
        act(() => {
            first = hook.result.current.cancel()
            second = hook.result.current.cancel()
        })
        await act(async () => { release.resolve(); await Promise.all([first, second]) })

        expect(mocks.cancelTransfer).toHaveBeenCalledTimes(1)
    })

    it('keeps an outcome visible if leaving fails, so Done can be pressed again', async () => {
        const hook = await outcomeController('incomplete', {filesSaved: 1, subfolderExists: true})
        mocks.cancelTransfer.mockRejectedValueOnce(new Error('adapter text'))

        await act(async () => { await hook.result.current.cancel() })
        expect(hook.result.current.state.phase).toBe('receive-outcome')

        await act(async () => { await hook.result.current.cancel() })
        expect(mocks.cancelTransfer).toHaveBeenCalledTimes(2)
    })

    it('cancels a waiting session through CancelTransfer and returns to Idle on its reset', async () => {
        const hook = await waitingController()

        await act(async () => { await hook.result.current.cancel() })
        expect(mocks.cancelTransfer).toHaveBeenCalledTimes(1)
        expect(hook.result.current.state).toMatchObject({phase: 'receive-waiting', cancelPending: true})

        act(() => emit('transfer-reset', {seq: 1}))
        expect(hook.result.current.state).toEqual({phase: 'idle', retainedOutcome: null, commandError: null})
    })

    it('reports a failed cancel of a receiving session and lets it be tried again', async () => {
        const hook = await waitingController()
        act(() => emit('transfer-started', {seq: 1}))
        mocks.cancelTransfer.mockRejectedValueOnce(JSON.stringify({code: 'busy', message: 'x'}))

        await act(async () => { await hook.result.current.cancel() })
        expect(hook.result.current.state).toMatchObject({phase: 'receive-receiving', cancelPending: false, commandError: {code: 'busy'}})
    })
})

describe('Show in Folder', () => {
    it('opens the folder once for a held outcome whose subfolder exists', async () => {
        const hook = await outcomeController('complete', {filesSaved: 3, subfolderExists: true})

        await act(async () => { await hook.result.current.showReceivedFolder() })

        expect(mocks.showReceivedFolder).toHaveBeenCalledTimes(1)
        expect(mocks.showReceivedFolder).toHaveBeenCalledWith()
        expect(hook.result.current.state).toMatchObject({phase: 'receive-outcome', commandError: null})
    })

    it('never calls the command when the subfolder does not exist', async () => {
        const hook = await outcomeController('incomplete', {filesSaved: 0, subfolderExists: false})

        await act(async () => { await hook.result.current.showReceivedFolder() })

        expect(mocks.showReceivedFolder).not.toHaveBeenCalled()
    })

    it('never calls the command outside a held outcome', async () => {
        const hook = await waitingController()
        await act(async () => { await hook.result.current.showReceivedFolder() })
        expect(mocks.showReceivedFolder).not.toHaveBeenCalled()
    })

    it('launches once for a double activation', async () => {
        const hook = await outcomeController('complete', {filesSaved: 3, subfolderExists: true})
        const launching = deferred<void>()
        mocks.showReceivedFolder.mockReturnValue(launching.promise)

        let first!: Promise<void>
        let second!: Promise<void>
        act(() => {
            first = hook.result.current.showReceivedFolder()
            second = hook.result.current.showReceivedFolder()
        })
        await act(async () => { launching.resolve(); await Promise.all([first, second]) })

        expect(mocks.showReceivedFolder).toHaveBeenCalledTimes(1)
    })

    it('reports a failed open as a fixed note beside the outcome, never the adapter text', async () => {
        const hook = await outcomeController('complete', {filesSaved: 3, subfolderExists: true})
        mocks.showReceivedFolder.mockRejectedValue(new Error('open /Users/someone/Phone Photos/FairDrop 2026-10-11: no such file'))

        await act(async () => { await hook.result.current.showReceivedFolder() })

        expect(hook.result.current.state).toMatchObject({
            phase: 'receive-outcome',
            commandError: {code: 'path_not_found'},
            outcome: {kind: 'complete', filesSaved: 3},
        })
        expect(JSON.stringify(hook.result.current.state)).not.toContain('/Users/someone')
    })

    it('clears the note when the next attempt succeeds', async () => {
        const hook = await outcomeController('complete', {filesSaved: 3, subfolderExists: true})
        mocks.showReceivedFolder.mockRejectedValueOnce(new Error('x'))
        await act(async () => { await hook.result.current.showReceivedFolder() })
        expect(hook.result.current.state).toMatchObject({commandError: {code: 'path_not_found'}})

        mocks.showReceivedFolder.mockResolvedValue(undefined)
        await act(async () => { await hook.result.current.showReceivedFolder() })
        expect(hook.result.current.state).toMatchObject({commandError: null})
    })
})

describe('send flows stay out of a receive session', () => {
    it.each([
        ['complete', 3],
        ['incomplete', 2],
        ['cancelled', 1],
    ] as const)('offers neither Send Again nor Try Again for a %s receive outcome', async (result, filesSaved) => {
        const hook = await outcomeController(result, {filesSaved, subfolderExists: true})

        expect(hook.result.current.canSendAgain).toBe(false)
        expect(hook.result.current.canRetry).toBe(false)

        await act(async () => { await hook.result.current.sendAgain() })
        await act(async () => { await hook.result.current.retry() })
        expect(mocks.stageTransfer).not.toHaveBeenCalled()
        expect(mocks.stageTransfers).not.toHaveBeenCalled()
    })

    it('forgets a remembered send path when a receive session starts', async () => {
        mocks.stageTransfer.mockRejectedValue(JSON.stringify({code: 'network_unavailable', message: 'x'}))
        mocks.selectReceiveFolder.mockResolvedValue('/Users/someone/Phone Photos')
        mocks.startReceive.mockResolvedValue(receiveMetadata())
        const hook = renderHook(() => useTransfer())

        await act(async () => { await hook.result.current.stage('/Users/someone/report.pdf') })
        expect(hook.result.current.canRetry).toBe(true)

        await act(async () => { await hook.result.current.receive() })
        expect(hook.result.current.state.phase).toBe('receive-waiting')
        expect(hook.result.current.canRetry).toBe(false)

        await act(async () => { await hook.result.current.retry() })
        expect(mocks.stageTransfer).toHaveBeenCalledTimes(1)
    })

    it('keeps a send retry available when the Receive chooser is only dismissed', async () => {
        mocks.stageTransfer.mockRejectedValue(JSON.stringify({code: 'network_unavailable', message: 'x'}))
        mocks.selectReceiveFolder.mockResolvedValue('')
        const hook = renderHook(() => useTransfer())

        await act(async () => { await hook.result.current.stage('/Users/someone/report.pdf') })
        await act(async () => { await hook.result.current.receive() })

        expect(hook.result.current.canRetry).toBe(true)
    })

    it('does not let a drop stage over a receive session', async () => {
        const hook = await waitingController()

        await act(async () => { await hook.result.current.stageFromOutcome('/Users/someone/report.pdf', 'unknown') })

        expect(mocks.stageTransfer).not.toHaveBeenCalled()
        expect(hook.result.current.state.phase).toBe('receive-waiting')
    })

    it('refuses Receive while the collection draft is open, without opening a chooser', async () => {
        const hook = renderHook(() => useTransfer())
        await act(async () => { await hook.result.current.openDraft(['/p/a', '/p/b']) })

        let result: unknown
        await act(async () => { result = await hook.result.current.receive() })

        expect(result).toBe('ignored')
        expect(mocks.selectReceiveFolder).not.toHaveBeenCalled()
        expect(hook.result.current.draft?.names).toEqual(['a', 'b'])
    })

    it('does not open the collection draft over a receive session', async () => {
        const hook = await waitingController()
        await act(async () => { await hook.result.current.openDraft(['/p/a', '/p/b']) })
        expect(hook.result.current.draft).toBeNull()
    })

    it('stages a send normally once the receive session has been left', async () => {
        const hook = await outcomeController('complete', {filesSaved: 1, subfolderExists: true})
        await act(async () => { await hook.result.current.cancel() })
        act(() => emit('transfer-reset', {seq: 4}))

        mocks.stageTransfer.mockResolvedValue({
            sessionId: 'abcdefabcdefabcdefabcdefabcdefab', name: 'report.pdf', size: 100, isDir: false,
            url: 'http://192.0.2.1:34123/download/fedcba9876543210fedcba9876543210', qrBase64: qrPNG, warnings: [],
        })
        await act(async () => { await hook.result.current.stage('/Users/someone/report.pdf') })
        expect(hook.result.current.state.phase).toBe('staged')
    })
})
