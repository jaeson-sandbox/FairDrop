import {describe, expect, it} from 'vitest'
import {
    createInitialTransferState,
    transferReducer,
    type ReceiveReceivingTransferState,
    type ReceiveWaitingTransferState,
    type TransferAction,
    type TransferState,
} from './state'
import type {FileMetadata, ReceiveMetadata} from './types'

/*
  Receive reducer rules (spec-phone-to-desktop-receiving, Story B).

  Expectations are literals: phase names, counts and codes are written out at
  the assertion site rather than read back from the reducer's own types.
*/

const sessionId = '0102030405060708090a0b0c0d0e0f10'
const qrPNG = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII='

const receiveMetadata: ReceiveMetadata = {
    sessionId,
    destination: 'Phone Photos',
    url: 'http://192.168.1.50:45678/upload/1112131415161718191a1b1c1d1e1f20',
    qrBase64: qrPNG,
    warnings: [],
}

const sendMetadata: FileMetadata = {
    sessionId,
    name: 'report.pdf',
    size: 400,
    isDir: false,
    url: 'http://192.168.1.50:45678/download/1112131415161718191a1b1c1d1e1f20',
    qrBase64: qrPNG,
    warnings: [],
}

function run(state: TransferState, ...actions: TransferAction[]): TransferState {
    return actions.reduce(transferReducer, state)
}

function event(eventName: string, body: Record<string, unknown>): TransferAction {
    return {type: 'lifecycle', eventName: eventName as never, args: [{sessionId, ...body}]}
}

function progress(bytesSent: number, totalBytes = 400): Record<string, unknown> {
    return {bytesSent, totalBytes, totalKnown: true, percent: (100 * bytesSent) / totalBytes, speedBytesPerSec: 0}
}

const savedStatus = (filesSaved: number) => ({filesSaved, subfolderExists: false, markingWarning: false})

function waitingState(): ReceiveWaitingTransferState {
    const pendingState = run(createInitialTransferState(),
        {type: 'stage-requested', generation: 1, itemKind: 'receive'},
        {type: 'receive-started', generation: 1, metadata: receiveMetadata})
    if (pendingState.phase !== 'receive-waiting') throw new Error(`expected receive-waiting, got ${pendingState.phase}`)
    return pendingState
}

function receivingState(): ReceiveReceivingTransferState {
    const state = run(waitingState(), event('transfer-started', {seq: 1}))
    if (state.phase !== 'receive-receiving') throw new Error(`expected receive-receiving, got ${state.phase}`)
    return state
}

describe('starting a receive session', () => {
    it('moves a pending receive to receive-waiting with the session cursor at zero', () => {
        expect(waitingState()).toEqual({
            phase: 'receive-waiting',
            session: {sessionId, lastSeq: 0},
            metadata: receiveMetadata,
            cancelPending: false,
            commandError: null,
            refusals: 0,
        })
    })

    it('refuses to commit a receive acknowledgement over a pending send', () => {
        const pending = run(createInitialTransferState(), {type: 'stage-requested', generation: 1, itemKind: 'file'})
        expect(run(pending, {type: 'receive-started', generation: 1, metadata: receiveMetadata})).toBe(pending)
    })

    it('refuses a stale generation and a cancelled pending receive', () => {
        const pending = run(createInitialTransferState(), {type: 'stage-requested', generation: 2, itemKind: 'receive'})
        expect(run(pending, {type: 'receive-started', generation: 1, metadata: receiveMetadata})).toBe(pending)

        const cancelling = run(pending, {type: 'cancel-requested'})
        expect(run(cancelling, {type: 'receive-started', generation: 2, metadata: receiveMetadata})).toBe(cancelling)
    })

    it('refuses to commit a send acknowledgement over a pending receive', () => {
        const pending = run(createInitialTransferState(), {type: 'stage-requested', generation: 1, itemKind: 'receive'})
        expect(run(pending, {type: 'stage-succeeded', generation: 1, metadata: sendMetadata})).toBe(pending)
    })

    it('marks a failed receive start so the failure card uses receive wording', () => {
        const pending = run(createInitialTransferState(), {type: 'stage-requested', generation: 1, itemKind: 'receive'})
        const failed = run(pending, {type: 'stage-failed', generation: 1, error: {code: 'path_not_found', message: 'x'}})

        expect(failed).toEqual({
            phase: 'idle',
            retainedOutcome: null,
            commandError: {
                code: 'path_not_found',
                message: 'That file or folder is no longer available. Choose it again.',
            },
            commandContext: 'receive',
        })
    })

    it('does not mark a failed send start as a receive failure', () => {
        const pending = run(createInitialTransferState(), {type: 'stage-requested', generation: 1, itemKind: 'file'})
        const failed = run(pending, {type: 'stage-failed', generation: 1, error: {code: 'path_not_found', message: 'x'}})

        expect(failed).not.toHaveProperty('commandContext')
    })

    it('treats a cancelled receive start as a quiet return to plain Idle', () => {
        const pending = run(createInitialTransferState(),
            {type: 'stage-requested', generation: 1, itemKind: 'receive'},
            {type: 'cancel-requested'})
        expect(run(pending, {type: 'pending-cancel-settled', generation: 1, error: null})).toEqual({
            phase: 'idle', retainedOutcome: null, commandError: null,
        })
    })

    it('carries the receive context through a pending-cancel failure too', () => {
        const pending = run(createInitialTransferState(),
            {type: 'stage-requested', generation: 1, itemKind: 'receive'},
            {type: 'cancel-requested'})
        expect(run(pending, {type: 'pending-cancel-settled', generation: 1, error: {code: 'cleanup_unconfirmed', message: 'x'}}))
            .toMatchObject({phase: 'idle', commandError: {code: 'cleanup_unconfirmed'}, commandContext: 'receive'})
    })

    it('drops the receive context when the failure is dismissed', () => {
        const pending = run(createInitialTransferState(), {type: 'stage-requested', generation: 1, itemKind: 'receive'})
        const failed = run(pending, {type: 'stage-failed', generation: 1, error: {code: 'path_not_found', message: 'x'}})
        expect(run(failed, {type: 'dismiss-retained'})).toEqual({phase: 'idle', retainedOutcome: null, commandError: null})
    })

    it('replaces a receive failure with a send one when a bad drop follows it', () => {
        const pending = run(createInitialTransferState(), {type: 'stage-requested', generation: 1, itemKind: 'receive'})
        const failed = run(pending, {type: 'stage-failed', generation: 1, error: {code: 'path_not_found', message: 'x'}})
        const refused = run(failed, {type: 'invalid-selection'})

        expect(refused).toEqual({
            phase: 'idle',
            retainedOutcome: null,
            commandError: {code: 'invalid_selection', message: 'Choose 1 to 10,000 separate files or folders.'},
        })
    })
})

describe('waiting for a phone', () => {
    it('counts each refused upload as a new state and keeps waiting', () => {
        const once = run(waitingState(), event('transfer-notice', {seq: 1, notice: 'receive_too_large'}))
        const twice = run(once, event('transfer-notice', {seq: 2, notice: 'receive_too_large'}))

        expect(once).toMatchObject({phase: 'receive-waiting', refusals: 1, session: {sessionId, lastSeq: 1}})
        expect(twice).toMatchObject({phase: 'receive-waiting', refusals: 2, session: {sessionId, lastSeq: 2}})
        expect(twice).not.toBe(once)
    })

    it('ignores a duplicate or older notice sequence', () => {
        const once = run(waitingState(), event('transfer-notice', {seq: 3, notice: 'receive_too_large'}))
        expect(run(once, event('transfer-notice', {seq: 3, notice: 'receive_too_large'}))).toBe(once)
        expect(run(once, event('transfer-notice', {seq: 2, notice: 'receive_too_large'}))).toBe(once)
    })

    it('ignores a notice for another session', () => {
        const state = waitingState()
        expect(run(state, {
            type: 'lifecycle', eventName: 'transfer-notice',
            args: [{sessionId: 'f'.repeat(32), seq: 1, notice: 'receive_too_large'}],
        })).toBe(state)
    })

    it('moves to receive-receiving on started, carrying a pending cancel through', () => {
        const cancelling = run(waitingState(), {type: 'cancel-requested'})
        expect(run(cancelling, event('transfer-started', {seq: 1}))).toEqual({
            phase: 'receive-receiving',
            session: {sessionId, lastSeq: 1},
            metadata: receiveMetadata,
            progress: null,
            filesSaved: 0,
            cancelPending: true,
            commandError: null,
        })
    })

    it('does not apply progress or complete before started', () => {
        const state = waitingState()
        expect(run(state, event('transfer-progress', {seq: 1, progress: progress(10), receive: savedStatus(0)}))).toBe(state)
        expect(run(state, event('transfer-complete', {
            seq: 1, progress: progress(400), receive: {filesSaved: 1, result: 'complete', subfolderExists: true, markingWarning: false},
        }))).toBe(state)
    })

    // The backend publishes a terminal incomplete outcome when the server lane
    // closes before any phone claimed the session (Go
    // TestALaneThatClosesBeforeAnyClaimFailsAWaitingReceive). Dropping it left a
    // QR for a listener that no longer existed, with nothing to tell the user.
    it('accepts a terminal incomplete outcome with nothing saved while still waiting', () => {
        const cancelling = run(waitingState(), {type: 'cancel-requested'})
        for (const state of [waitingState(), cancelling]) {
            expect(run(state, event('transfer-error', {
                seq: 1, error: {code: 'transfer_failed', message: 'x'},
                receive: {filesSaved: 0, result: 'incomplete', subfolderExists: false, markingWarning: false},
            }))).toEqual({
                phase: 'receive-outcome',
                session: {sessionId, lastSeq: 1},
                destination: 'Phone Photos',
                outcome: {kind: 'incomplete', filesSaved: 0, subfolderExists: false, markingWarning: false},
                commandError: null,
            })
        }
    })

    it('still drops a waiting error whose receive payload is missing, unresolved or claims saved files', () => {
        const state = waitingState()
        const error = {code: 'transfer_failed', message: 'x'}
        // No receive payload at all: not a receive terminal event.
        expect(run(state, event('transfer-error', {seq: 1, error}))).toBe(state)
        // No result: not terminal.
        expect(run(state, event('transfer-error', {
            seq: 1, error, receive: {filesSaved: 0, result: null, subfolderExists: false, markingWarning: false},
        }))).toBe(state)
        // Files saved, or a folder that exists, cannot be true of a session no phone has claimed.
        expect(run(state, event('transfer-error', {
            seq: 1, error, receive: {filesSaved: 1, result: 'incomplete', subfolderExists: true, markingWarning: false},
        }))).toBe(state)
        expect(run(state, event('transfer-error', {
            seq: 1, error, receive: {filesSaved: 0, result: 'incomplete', subfolderExists: true, markingWarning: false},
        }))).toBe(state)
        // Only an incomplete result can end a session nobody claimed.
        // (A cancelled result must arrive with the cancelled code, or the parser refuses it
        // before the reducer sees it; the matching code isolates the reducer's own rule.)
        expect(run(state, event('transfer-error', {
            seq: 1, error: {code: 'cancelled', message: 'x'},
            receive: {filesSaved: 0, result: 'cancelled', subfolderExists: false, markingWarning: false},
        }))).toBe(state)
    })

    it('returns to plain Idle on reset without a retained outcome', () => {
        expect(run(waitingState(), event('transfer-reset', {seq: 1}))).toEqual({
            phase: 'idle', retainedOutcome: null, commandError: null,
        })
    })

    it('records a pending cancel and reports a failed one as a command error', () => {
        const cancelling = run(waitingState(), {type: 'cancel-requested'})
        expect(cancelling).toMatchObject({cancelPending: true})
        expect(run(cancelling, {type: 'active-cancel-failed', sessionId, error: {code: 'busy', message: 'x'}})).toMatchObject({
            phase: 'receive-waiting', cancelPending: false, commandError: {code: 'busy'},
        })
    })

    it('reports a failed clipboard write beside the waiting view, and not while cancelling', () => {
        expect(run(waitingState(), {type: 'clipboard-failed', sessionId})).toMatchObject({
            phase: 'receive-waiting', commandError: {code: 'clipboard_failed'},
        })
        const cancelling = run(waitingState(), {type: 'cancel-requested'})
        expect(run(cancelling, {type: 'clipboard-failed', sessionId})).toBe(cancelling)
    })
})

describe('receiving', () => {
    it('accepts increasing progress with its saved count', () => {
        const state = run(receivingState(),
            event('transfer-progress', {seq: 2, progress: progress(100), receive: savedStatus(1)}),
            event('transfer-progress', {seq: 3, progress: progress(250), receive: savedStatus(2)}))

        expect(state).toMatchObject({
            phase: 'receive-receiving',
            session: {sessionId, lastSeq: 3},
            filesSaved: 2,
            progress: {bytesSent: 250, totalBytes: 400, totalKnown: true},
        })
    })

    it('refuses progress that goes backwards, changes the declared total, or lowers the saved count', () => {
        const base = run(receivingState(), event('transfer-progress', {seq: 2, progress: progress(200), receive: savedStatus(2)}))
        expect(run(base, event('transfer-progress', {seq: 3, progress: progress(100), receive: savedStatus(2)}))).toBe(base)
        expect(run(base, event('transfer-progress', {seq: 3, progress: progress(250, 500), receive: savedStatus(2)}))).toBe(base)
        expect(run(base, event('transfer-progress', {seq: 3, progress: progress(250), receive: savedStatus(1)}))).toBe(base)
    })

    it('refuses progress with no receive payload and progress with an unknown total', () => {
        const base = receivingState()
        expect(run(base, event('transfer-progress', {seq: 2, progress: progress(100)}))).toBe(base)
        expect(run(base, event('transfer-progress', {
            seq: 2, receive: savedStatus(0),
            progress: {bytesSent: 100, totalBytes: 0, totalKnown: false, percent: 0, speedBytesPerSec: 0},
        }))).toBe(base)
    })

    it('ends complete with the count and the flags the event carried', () => {
        const state = run(receivingState(),
            event('transfer-progress', {seq: 2, progress: progress(400), receive: savedStatus(3)}),
            event('transfer-complete', {
                seq: 3, progress: progress(400),
                receive: {filesSaved: 3, result: 'complete', subfolderExists: true, markingWarning: false},
            }))

        expect(state).toEqual({
            phase: 'receive-outcome',
            session: {sessionId, lastSeq: 3},
            destination: 'Phone Photos',
            outcome: {kind: 'complete', filesSaved: 3, subfolderExists: true, markingWarning: false},
            commandError: null,
        })
    })

    it('demotes a success whose snapshot does not follow the accepted progress to incomplete, never the reverse', () => {
        const state = run(receivingState(),
            event('transfer-progress', {seq: 2, progress: progress(300), receive: savedStatus(2)}),
            event('transfer-complete', {
                // A terminal snapshot whose declared total differs from the one already accepted.
                seq: 3, progress: {bytesSent: 500, totalBytes: 500, totalKnown: true, percent: 100, speedBytesPerSec: 0},
                receive: {filesSaved: 3, result: 'complete', subfolderExists: true, markingWarning: false},
            }))

        expect(state).toMatchObject({phase: 'receive-outcome', outcome: {kind: 'incomplete', filesSaved: 3}})
    })

    it('demotes a success with an unknown total to incomplete', () => {
        const state = run(receivingState(), event('transfer-complete', {
            seq: 2,
            progress: {bytesSent: 0, totalBytes: 0, totalKnown: false, percent: 0, speedBytesPerSec: 0},
            receive: {filesSaved: 3, result: 'complete', subfolderExists: true, markingWarning: false},
        }))
        expect(state).toMatchObject({phase: 'receive-outcome', outcome: {kind: 'incomplete', filesSaved: 3}})
    })

    it('shows an incomplete upload as incomplete with the exact count', () => {
        const state = run(receivingState(), event('transfer-error', {
            seq: 2, progress: progress(100),
            error: {code: 'transfer_failed', message: 'x'},
            receive: {filesSaved: 2, result: 'incomplete', subfolderExists: true, markingWarning: true},
        }))

        expect(state).toEqual({
            phase: 'receive-outcome',
            session: {sessionId, lastSeq: 2},
            destination: 'Phone Photos',
            outcome: {kind: 'incomplete', filesSaved: 2, subfolderExists: true, markingWarning: true},
            commandError: null,
        })
    })

    it('shows a cancelled upload as cancelled with the exact count', () => {
        const state = run(receivingState(), event('transfer-error', {
            seq: 2,
            error: {code: 'cancelled', message: 'Transfer canceled.'},
            receive: {filesSaved: 1, result: 'cancelled', subfolderExists: true, markingWarning: false},
        }))

        expect(state).toMatchObject({phase: 'receive-outcome', outcome: {kind: 'cancelled', filesSaved: 1, subfolderExists: true}})
    })

    it('keeps an outcome with nothing saved and no subfolder honest', () => {
        const state = run(receivingState(), event('transfer-error', {
            seq: 2, error: {code: 'transfer_failed', message: 'x'},
            receive: {filesSaved: 0, result: 'incomplete', subfolderExists: false, markingWarning: false},
        }))
        expect(state).toMatchObject({outcome: {kind: 'incomplete', filesSaved: 0, subfolderExists: false}})
    })

    it('never retains the capability url or qr in the outcome', () => {
        const state = run(receivingState(), event('transfer-complete', {
            seq: 2, progress: progress(400),
            receive: {filesSaved: 1, result: 'complete', subfolderExists: true, markingWarning: false},
        }))
        const serialized = JSON.stringify(state)
        expect(serialized).not.toContain('/upload/')
        expect(serialized).not.toContain(qrPNG)
    })

    it('refuses a terminal event with no receive payload', () => {
        const base = receivingState()
        expect(run(base, event('transfer-complete', {seq: 2, progress: progress(400)}))).toBe(base)
        expect(run(base, event('transfer-error', {seq: 2, error: {code: 'transfer_failed', message: 'x'}}))).toBe(base)
    })

    it('records a pending cancel while receiving', () => {
        expect(run(receivingState(), {type: 'cancel-requested'})).toMatchObject({phase: 'receive-receiving', cancelPending: true})
    })

    it('returns to plain Idle on a reset with no terminal event', () => {
        expect(run(receivingState(), event('transfer-reset', {seq: 2}))).toEqual({
            phase: 'idle', retainedOutcome: null, commandError: null,
        })
    })
})

describe('the held outcome', () => {
    function outcomeState(): TransferState {
        return run(receivingState(), event('transfer-complete', {
            seq: 2, progress: progress(400),
            receive: {filesSaved: 3, result: 'complete', subfolderExists: true, markingWarning: false},
        }))
    }

    it('is held: nothing but a reset moves it', () => {
        const held = outcomeState()
        expect(run(held, event('transfer-progress', {seq: 3, progress: progress(400), receive: savedStatus(3)}))).toBe(held)
        expect(run(held, event('transfer-started', {seq: 3}))).toBe(held)
        expect(run(held, event('transfer-notice', {seq: 3, notice: 'receive_too_large'}))).toBe(held)
        expect(run(held, {type: 'dismiss-retained'})).toBe(held)
    })

    it('leaves to plain Idle on reset, retaining nothing a send outcome card could offer', () => {
        expect(run(outcomeState(), event('transfer-reset', {seq: 3}))).toEqual({
            phase: 'idle', retainedOutcome: null, commandError: null,
        })
    })

    it('records a failed Show in Folder beside the outcome and clears it on the next request', () => {
        const failed = run(outcomeState(), {type: 'receive-show-failed', sessionId})
        expect(failed).toMatchObject({
            phase: 'receive-outcome',
            commandError: {code: 'path_not_found'},
            outcome: {kind: 'complete', filesSaved: 3},
        })
        expect(run(failed, {type: 'receive-show-requested', sessionId})).toMatchObject({commandError: null})
    })

    it('ignores a show failure for another session or outside the outcome', () => {
        const held = outcomeState()
        expect(run(held, {type: 'receive-show-failed', sessionId: 'f'.repeat(32)})).toBe(held)
        const waiting = waitingState()
        expect(run(waiting, {type: 'receive-show-failed', sessionId})).toBe(waiting)
    })
})

describe('send sessions never take receive events', () => {
    function stagedSend(): TransferState {
        return run(createInitialTransferState(),
            {type: 'stage-requested', generation: 1, itemKind: 'file'},
            {type: 'stage-succeeded', generation: 1, metadata: sendMetadata})
    }

    function transferringSend(): TransferState {
        return run(stagedSend(), event('transfer-started', {seq: 1}))
    }

    it('ignores a notice while staged or transferring', () => {
        const staged = stagedSend()
        expect(run(staged, event('transfer-notice', {seq: 1, notice: 'receive_too_large'}))).toBe(staged)
        const sending = transferringSend()
        expect(run(sending, event('transfer-notice', {seq: 2, notice: 'receive_too_large'}))).toBe(sending)
    })

    it('ignores receive progress, complete and error on a transferring send session', () => {
        const sending = transferringSend()
        expect(run(sending, event('transfer-progress', {seq: 2, progress: progress(100), receive: savedStatus(1)}))).toBe(sending)
        expect(run(sending, event('transfer-complete', {
            seq: 2, progress: progress(400),
            receive: {filesSaved: 1, result: 'complete', subfolderExists: true, markingWarning: false},
        }))).toBe(sending)
        expect(run(sending, event('transfer-error', {
            seq: 2, error: {code: 'transfer_failed', message: 'x'},
            receive: {filesSaved: 1, result: 'incomplete', subfolderExists: true, markingWarning: false},
        }))).toBe(sending)
    })

    it('still completes a send session from an event with no receive payload', () => {
        const done = run(transferringSend(), event('transfer-complete', {seq: 2, progress: progress(400)}))
        expect(done).toMatchObject({phase: 'done', outcome: {kind: 'done', receipt: {name: 'report.pdf', bytesSent: 400}}})
    })
})
