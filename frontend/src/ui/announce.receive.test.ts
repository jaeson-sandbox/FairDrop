import {describe, expect, it} from 'vitest'
import type {
    IdleTransferState,
    ReceiveOutcomeTransferState,
    ReceiveReceivingTransferState,
    ReceiveWaitingTransferState,
    TransferState,
} from '../transfer/state'
import {routeTransition} from './announce'

/*
  The receive rows of the announcement-ownership table. Every expectation names
  the owner and the target or text as a literal: the table is what keeps one
  transition from being heard twice, so it is asserted row by row rather than
  derived from the code that implements it.
*/

const sessionId = '0102030405060708090a0b0c0d0e0f10'
const metadata = {
    sessionId,
    destination: 'Phone Photos',
    url: 'http://192.168.1.50:45678/upload/1112131415161718191a1b1c1d1e1f20',
    qrBase64: 'iVBORw0KGgo=',
    warnings: [],
}

const idle: IdleTransferState = {phase: 'idle', retainedOutcome: null, commandError: null}
const pending: TransferState = {phase: 'pending', generation: 1, itemKind: 'receive', cancelPending: false}

function waiting(overrides: Partial<ReceiveWaitingTransferState> = {}): ReceiveWaitingTransferState {
    return {
        phase: 'receive-waiting', session: {sessionId, lastSeq: 0}, metadata,
        cancelPending: false, commandError: null, refusals: 0, ...overrides,
    }
}

function receiving(overrides: Partial<ReceiveReceivingTransferState> = {}): ReceiveReceivingTransferState {
    return {
        phase: 'receive-receiving', session: {sessionId, lastSeq: 1}, metadata,
        progress: null, filesSaved: 0, cancelPending: false, commandError: null, ...overrides,
    }
}

function outcome(overrides: Partial<ReceiveOutcomeTransferState> = {}): ReceiveOutcomeTransferState {
    return {
        phase: 'receive-outcome', session: {sessionId, lastSeq: 4}, destination: 'Phone Photos',
        outcome: {kind: 'complete', filesSaved: 3, subfolderExists: true, markingWarning: false},
        commandError: null, ...overrides,
    }
}

describe('opening and progressing a receive session', () => {
    it('focuses the pending heading when a receive starts', () => {
        expect(routeTransition(idle, pending)).toEqual({row: 'stage-pending', owner: 'focus', target: 'pending-heading'})
    })

    it('focuses the waiting heading when the session opens', () => {
        expect(routeTransition(pending, waiting())).toEqual({row: 'receive-ready', owner: 'focus', target: 'receive-waiting-heading'})
    })

    it('focuses the receiving heading when the phone starts uploading', () => {
        expect(routeTransition(waiting(), receiving())).toEqual({row: 'receive-started', owner: 'focus', target: 'receive-receiving-heading'})
    })

    it('leaves progress and saved-count changes to the throttle: no owner here', () => {
        const next = receiving({
            progress: {bytesSent: 10, totalBytes: 100, totalKnown: true, percent: 10, speedBytesPerSec: 1},
            filesSaved: 1,
        })
        expect(routeTransition(receiving(), next)).toBeNull()
    })
})

describe('a refused upload', () => {
    it('is spoken by the announcer, not focused', () => {
        expect(routeTransition(waiting(), waiting({refusals: 1}))).toEqual({
            row: 'receive-notice',
            owner: 'announcer',
            text: 'A phone tried to upload more than this computer has room for. FairDrop is still waiting. Choose fewer files on the phone, or cancel and pick a folder with more free space.',
        })
    })

    it('is spoken again for each further refusal', () => {
        expect(routeTransition(waiting({refusals: 1}), waiting({refusals: 2}))).toMatchObject({row: 'receive-notice', owner: 'announcer'})
    })

    it('is not spoken over a pending cancellation', () => {
        expect(routeTransition(waiting({cancelPending: true}), waiting({cancelPending: true, refusals: 1}))).toBeNull()
    })

    it('says nothing when the count did not change', () => {
        expect(routeTransition(waiting({refusals: 1}), waiting({refusals: 1, session: {sessionId, lastSeq: 5}}))).toBeNull()
    })
})

describe('cancelling and failing beside a live receive session', () => {
    it('speaks Canceling when a cancel is requested while waiting', () => {
        expect(routeTransition(waiting(), waiting({cancelPending: true}))).toEqual({
            row: 'cancel-requested', owner: 'announcer', text: 'Canceling',
        })
    })

    it('speaks Canceling when a cancel is requested while receiving', () => {
        expect(routeTransition(receiving(), receiving({cancelPending: true}))).toEqual({
            row: 'cancel-requested', owner: 'announcer', text: 'Canceling',
        })
    })

    it('focuses a new command error beside the waiting view', () => {
        const error = {code: 'clipboard_failed' as const, message: 'x'}
        expect(routeTransition(waiting(), waiting({commandError: error}))).toEqual({
            row: 'command-failure', owner: 'focus', target: 'command-error',
        })
    })

    it('focuses a new command error beside the receiving view', () => {
        const error = {code: 'busy' as const, message: 'x'}
        expect(routeTransition(receiving(), receiving({commandError: error}))).toEqual({
            row: 'command-failure', owner: 'focus', target: 'command-error',
        })
    })

    it('never focuses a cancellation as an error', () => {
        const error = {code: 'cancelled' as const, message: 'x'}
        expect(routeTransition(waiting(), waiting({commandError: error}))).toBeNull()
    })
})

describe('the outcome and leaving it', () => {
    it('focuses the outcome when a receive ends, whichever way it ended', () => {
        for (const kind of ['complete', 'incomplete', 'cancelled'] as const) {
            const next = outcome({outcome: {kind, filesSaved: 1, subfolderExists: true, markingWarning: false}})
            expect(routeTransition(receiving(), next)).toEqual({row: 'receive-outcome', owner: 'focus', target: 'receive-outcome'})
        }
    })

    it('focuses the outcome when a receive nobody claimed ends while still waiting', () => {
        // The server lane closing before any phone claimed the session ends it
        // straight from waiting; the outcome has the same one owner as any other.
        const unclaimed = outcome({outcome: {kind: 'incomplete', filesSaved: 0, subfolderExists: false, markingWarning: false}})
        for (const before of [waiting(), waiting({cancelPending: true}), waiting({refusals: 2})]) {
            expect(routeTransition(before, unclaimed)).toEqual({row: 'receive-outcome', owner: 'focus', target: 'receive-outcome'})
        }
    })

    it('focuses a failed Show in Folder as a command failure, and nothing else about the outcome', () => {
        const failed = outcome({commandError: {code: 'path_not_found', message: 'x'}})
        expect(routeTransition(outcome(), failed)).toEqual({row: 'command-failure', owner: 'focus', target: 'command-error'})
        expect(routeTransition(failed, outcome({session: {sessionId, lastSeq: 9}, commandError: failed.commandError}))).toBeNull()
    })

    it('focuses the Idle instruction when the outcome is left, because nothing stays mounted to hold focus', () => {
        expect(routeTransition(outcome(), idle)).toEqual({row: 'receive-leave', owner: 'focus', target: 'idle-instruction'})
    })

    it('treats a waiting session cancelled back to Idle as the cancel-winning row', () => {
        expect(routeTransition(waiting(), idle)).toEqual({row: 'cancel-won', owner: 'focus', target: 'idle-instruction'})
        expect(routeTransition(receiving(), idle)).toEqual({row: 'cancel-won', owner: 'focus', target: 'idle-instruction'})
    })

    it('treats a cancelled receive start as the cancel-winning row', () => {
        expect(routeTransition(pending, idle)).toEqual({row: 'cancel-won', owner: 'focus', target: 'idle-instruction'})
    })

    it('focuses the failure card when a receive start fails', () => {
        const failed: IdleTransferState = {
            phase: 'idle', retainedOutcome: null, commandContext: 'receive',
            commandError: {code: 'path_not_found', message: 'x'},
        }
        expect(routeTransition(pending, failed)).toEqual({row: 'command-failure', owner: 'focus', target: 'command-error'})
    })
})
