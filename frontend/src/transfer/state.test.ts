import {describe, expect, it} from 'vitest'
import {publicError} from './errors'
import {createInitialTransferState, transferReducer, type TransferState} from './state'
import type {PublicError} from './types'

const sessionId = '0123456789abcdef0123456789abcdef'
const qrPNG = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII='

function metadata(overrides: Record<string, unknown> = {}): Record<string, unknown> {
    return {
        sessionId,
        name: 'report.pdf',
        size: 100,
        isDir: false,
        url: 'http://192.0.2.1:34123/download/fedcba9876543210fedcba9876543210',
        qrBase64: qrPNG,
        warnings: [],
        ...overrides,
    }
}

function progress(bytesSent: number, totalBytes = 100): Record<string, unknown> {
    return {
        bytesSent,
        totalBytes,
        totalKnown: true,
        percent: totalBytes === 0 ? 0 : 100 * bytesSent / totalBytes,
        speedBytesPerSec: 25,
    }
}

function staged(metadataOverrides: Record<string, unknown> = {}): TransferState {
    let state: TransferState = createInitialTransferState()
    state = transferReducer(state, {type: 'stage-requested', generation: 1, itemKind: 'unknown'})
    return transferReducer(state, {type: 'stage-succeeded', generation: 1, metadata: metadata(metadataOverrides)})
}

function event(state: TransferState, eventName: 'transfer-started' | 'transfer-progress' | 'transfer-complete' | 'transfer-error' | 'transfer-reset', payload: unknown): TransferState {
    return transferReducer(state, {type: 'lifecycle', eventName, args: [payload]})
}

describe('Stage acknowledgement and local command state', () => {
    it('keeps pending local and initializes the session only from current valid metadata', () => {
        const idle = createInitialTransferState()
        const pending = transferReducer(idle, {type: 'stage-requested', generation: 7, itemKind: 'unknown'})

        expect(pending).toEqual({phase: 'pending', generation: 7, itemKind: 'unknown', cancelPending: false})
        expect(JSON.stringify(pending)).not.toContain('sessionId')

        const installed = transferReducer(pending, {type: 'stage-succeeded', generation: 7, metadata: metadata()})
        expect(installed).toMatchObject({
            phase: 'staged',
            session: {sessionId, lastSeq: 0},
            metadata: {name: 'report.pdf'},
        })
    })

    it('rejects obsolete or malformed acknowledgements with state identity unchanged', () => {
        const pending = transferReducer(createInitialTransferState(), {
            type: 'stage-requested', generation: 7, itemKind: 'file',
        })

        expect(transferReducer(pending, {type: 'stage-succeeded', generation: 6, metadata: metadata()})).toBe(pending)
        expect(transferReducer(pending, {type: 'stage-succeeded', generation: 7, metadata: metadata({sessionId: ''})})).toBe(pending)

        const cancelling = transferReducer(pending, {type: 'cancel-requested'})
        expect(transferReducer(cancelling, {type: 'stage-succeeded', generation: 7, metadata: metadata()})).toBe(cancelling)
    })

    it('leaves no active session after command failure and never renders cancelled as Error', () => {
        const pending = transferReducer(createInitialTransferState(), {
            type: 'stage-requested', generation: 1, itemKind: 'file',
        })
        const failed = transferReducer(pending, {
            type: 'stage-failed', generation: 1, error: publicError('path_not_found'),
        })
        expect(failed).toEqual({
            phase: 'idle', retainedOutcome: null,
            commandError: {
                code: 'path_not_found',
                message: 'That file or folder is no longer available. Choose it again.',
            },
        })
        expect(JSON.stringify(failed)).not.toContain('sessionId')

        const pendingAgain = transferReducer(failed, {
            type: 'stage-requested', generation: 2, itemKind: 'file',
        })
        const cancelled = transferReducer(pendingAgain, {
            type: 'stage-failed', generation: 2, error: publicError('cancelled'),
        })
        expect(cancelled).toEqual(createInitialTransferState())
    })
})

describe('authoritative lifecycle grammar', () => {
    it('accepts the complete success grammar and advances sequence exactly once per event', () => {
        let state = staged()
        state = event(state, 'transfer-started', {sessionId, seq: 1})
        expect(state).toMatchObject({phase: 'transferring', session: {lastSeq: 1}, progress: null})

        state = event(state, 'transfer-progress', {sessionId, seq: 2, progress: progress(50)})
        expect(state).toMatchObject({phase: 'transferring', session: {lastSeq: 2}, progress: {bytesSent: 50}})

        state = event(state, 'transfer-complete', {sessionId, seq: 3, progress: progress(100)})
        expect(state).toEqual({
            phase: 'done',
            session: {sessionId, lastSeq: 3},
            outcome: {kind: 'done'},
        })

        state = event(state, 'transfer-reset', {sessionId, seq: 4})
        expect(state).toEqual({phase: 'idle', retainedOutcome: {kind: 'done'}, commandError: null})
    })

    it('accepts failure with or without final progress and uses fixed terminal copy', () => {
        let withProgress = event(staged(), 'transfer-started', {sessionId, seq: 1})
        withProgress = event(withProgress, 'transfer-progress', {
            sessionId, seq: 2, progress: progress(25),
        })
        withProgress = event(withProgress, 'transfer-error', {
            sessionId,
            seq: 3,
            progress: progress(25),
            error: {code: 'source_changed', message: String.raw`C:\secret\report.pdf`},
        })
        expect(withProgress).toEqual({
            phase: 'error',
            session: {sessionId, lastSeq: 3},
            outcome: {
                kind: 'error',
                error: {
                    code: 'source_changed',
                    message: 'The item changed after it was prepared. Cancel and create a fresh link.',
                },
            },
        })

        let withoutProgress = event(staged(), 'transfer-started', {sessionId, seq: 1})
        withoutProgress = event(withoutProgress, 'transfer-error', {
            sessionId, seq: 2, error: null,
        })
        expect(withoutProgress).toMatchObject({
            phase: 'error',
            outcome: {error: {code: 'transfer_failed'}},
        })
    })

    it('accepts preterminal reset but retains only Done and Error outcomes', () => {
        const resetFromStaged = event(staged(), 'transfer-reset', {sessionId, seq: 1})
        expect(resetFromStaged).toEqual(createInitialTransferState())

        const transferring = event(staged(), 'transfer-started', {sessionId, seq: 1})
        const resetFromTransferring = event(transferring, 'transfer-reset', {sessionId, seq: 2})
        expect(resetFromTransferring).toEqual(createInitialTransferState())
    })

    it('rejects foreign, stale, duplicate, illegal, malformed, and variadic input without consuming sequence', () => {
        const state = staged()
        const rejectedActions = [
            {type: 'lifecycle', eventName: 'transfer-started', args: [{sessionId: 'foreign', seq: 1}]},
            {type: 'lifecycle', eventName: 'transfer-progress', args: [{sessionId, seq: 1, progress: progress(1)}]},
            {type: 'lifecycle', eventName: 'transfer-started', args: [{sessionId, seq: 0}]},
            {type: 'lifecycle', eventName: 'transfer-started', args: [{sessionId, seq: 1, progress: progress(1)}]},
            {type: 'lifecycle', eventName: 'transfer-started', args: [{sessionId, seq: 1}, 'extra']},
        ] as const

        for (const action of rejectedActions) expect(transferReducer(state, action)).toBe(state)

        const accepted = event(state, 'transfer-started', {sessionId, seq: 1})
        expect(accepted).toMatchObject({phase: 'transferring', session: {lastSeq: 1}})
        expect(event(accepted, 'transfer-progress', {sessionId, seq: 1, progress: progress(1)})).toBe(accepted)
        expect(event(accepted, 'transfer-started', {sessionId, seq: 2})).toBe(accepted)
    })

    it('ignores every lifecycle event while no session exists', () => {
        const fresh = createInitialTransferState()
        const pending = transferReducer(fresh, {type: 'stage-requested', generation: 1, itemKind: 'file'})
        let retained = event(staged(), 'transfer-started', {sessionId, seq: 1})
        retained = event(retained, 'transfer-complete', {sessionId, seq: 2, progress: progress(100)})
        retained = event(retained, 'transfer-reset', {sessionId, seq: 3})
        expect(retained).toMatchObject({phase: 'idle', retainedOutcome: {kind: 'done'}})

        // A forged event reaches every window listener without the backend
        // being involved, so a session-less state is as exposed as a live one
        // and has no cursor of its own to reject the event with.
        for (const state of [fresh, pending, retained]) {
            for (const name of ['transfer-started', 'transfer-progress', 'transfer-complete',
                'transfer-error', 'transfer-reset'] as const) {
                expect(event(state, name, {sessionId, seq: 1})).toBe(state)
                expect(event(state, name, {sessionId, seq: 9, progress: progress(100)})).toBe(state)
                expect(event(state, name, {
                    sessionId, seq: 9, error: {code: 'transfer_failed', message: 'forged'},
                })).toBe(state)
            }
        }
    })

    it('accepts a legal same-session event when its sequence skips ahead', () => {
        const state = event(staged(), 'transfer-started', {sessionId, seq: 7})

        expect(state).toMatchObject({
            phase: 'transferring',
            session: {sessionId, lastSeq: 7},
        })
    })

    it('rejects regressive progress without consuming its sequence', () => {
        let state = event(staged(), 'transfer-started', {sessionId, seq: 1})
        state = event(state, 'transfer-progress', {sessionId, seq: 2, progress: progress(50)})

        const rejected = event(state, 'transfer-progress', {sessionId, seq: 3, progress: progress(40)})
        expect(rejected).toBe(state)

        const accepted = event(rejected, 'transfer-progress', {sessionId, seq: 3, progress: progress(75)})
        expect(accepted).toMatchObject({session: {lastSeq: 3}, progress: {bytesSent: 75}})
    })

    it('rejects file snapshots that disagree with staged size without consuming sequence', () => {
        let state = event(staged(), 'transfer-started', {sessionId, seq: 1})

        const wrongTotal = event(state, 'transfer-progress', {
            sessionId, seq: 2, progress: progress(25, 50),
        })
        expect(wrongTotal).toBe(state)
        const unknownTotal = event(state, 'transfer-progress', {
            sessionId, seq: 2,
            progress: {bytesSent: 25, totalBytes: 0, totalKnown: false, percent: 0, speedBytesPerSec: 25},
        })
        expect(unknownTotal).toBe(state)

        state = event(state, 'transfer-progress', {sessionId, seq: 2, progress: progress(25)})
        expect(state).toMatchObject({session: {lastSeq: 2}, progress: {totalBytes: 100}})

        const wrongComplete = event(state, 'transfer-complete', {
            sessionId, seq: 3, progress: progress(50, 50),
        })
        expect(wrongComplete).toBe(state)
        const wrongError = event(state, 'transfer-error', {
            sessionId, seq: 3, progress: progress(50, 50),
            error: {code: 'transfer_failed', message: 'forged'},
        })
        expect(wrongError).toBe(state)

        expect(event(state, 'transfer-complete', {sessionId, seq: 3, progress: progress(100)})).toMatchObject({
            phase: 'done', session: {lastSeq: 3},
        })
    })

    it('accepts unknown-total snapshots for staged directories', () => {
        let state = event(staged({name: 'papers', size: 0, isDir: true}), 'transfer-started', {sessionId, seq: 1})
        const unknown = {bytesSent: 25, totalBytes: 0, totalKnown: false, percent: 0, speedBytesPerSec: 25}

        expect(event(state, 'transfer-progress', {sessionId, seq: 2, progress: progress(25)})).toBe(state)
        state = event(state, 'transfer-progress', {sessionId, seq: 2, progress: unknown})
        expect(state).toMatchObject({phase: 'transferring', progress: {totalKnown: false, bytesSent: 25}})
        expect(event(state, 'transfer-complete', {sessionId, seq: 3, progress: unknown})).toMatchObject({
            phase: 'done', session: {lastSeq: 3},
        })
    })

    it('suppresses all late progress and terminal input after terminal acceptance', () => {
        let state = event(staged(), 'transfer-started', {sessionId, seq: 1})
        state = event(state, 'transfer-complete', {sessionId, seq: 2, progress: progress(100)})

        expect(event(state, 'transfer-progress', {sessionId, seq: 3, progress: progress(100)})).toBe(state)
        expect(event(state, 'transfer-error', {
            sessionId, seq: 3, error: {code: 'transfer_failed', message: 'x'},
        })).toBe(state)
        expect(event(state, 'transfer-complete', {sessionId, seq: 3, progress: progress(100)})).toBe(state)
    })
})

describe('terminal scrubbing and retained outcome', () => {
    it('discards metadata at terminal and all correlation data on reset', () => {
        let state = event(staged(), 'transfer-started', {sessionId, seq: 1})
        state = event(state, 'transfer-complete', {sessionId, seq: 2, progress: progress(100)})
        expect(JSON.stringify(state)).not.toContain('fedcba9876543210fedcba9876543210')
        expect(JSON.stringify(state)).not.toContain(
            'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=',
        )
        expect(JSON.stringify(state)).not.toContain('report.pdf')

        state = event(state, 'transfer-reset', {sessionId, seq: 3})
        const serialized = JSON.stringify(state)
        expect(serialized).toBe('{"phase":"idle","retainedOutcome":{"kind":"done"},"commandError":null}')
        expect(serialized).not.toContain('session')
        expect(serialized).not.toContain('seq')
        expect(serialized).not.toContain('url')
        expect(serialized).not.toContain('qr')
        expect(serialized).not.toContain('timer')
    })

    it('keeps a scrubbed Error through reset until dismiss or the next Stage attempt', () => {
        let state = event(staged(), 'transfer-started', {sessionId, seq: 1})
        state = event(state, 'transfer-error', {
            sessionId, seq: 2,
            error: {code: 'path_not_found', message: 'secret'},
        })
        state = event(state, 'transfer-reset', {sessionId, seq: 3})
        expect(state).toEqual({
            phase: 'idle',
            retainedOutcome: {
                kind: 'error',
                error: {
                    code: 'path_not_found',
                    message: 'That file or folder is no longer available. Choose it again.',
                },
            },
            commandError: null,
        })

        expect(transferReducer(state, {type: 'dismiss-retained'})).toEqual(createInitialTransferState())
        expect(transferReducer(state, {
            type: 'stage-requested', generation: 2, itemKind: 'directory',
        })).toEqual({phase: 'pending', generation: 2, itemKind: 'directory', cancelPending: false})
    })

    it('rewrites caller-supplied error copy rather than storing what it was handed', () => {
        const forged: PublicError = {code: 'busy', message: 'C:\\private\\report.pdf?token=fedcba98'}
        const registryCopy = 'Finish or cancel the current transfer before choosing another item.'

        const pending = transferReducer(createInitialTransferState(), {
            type: 'stage-requested', generation: 1, itemKind: 'file',
        })
        const failed = transferReducer(pending, {type: 'stage-failed', generation: 1, error: forged})
        expect(failed).toEqual({
            phase: 'idle', retainedOutcome: null,
            commandError: {code: 'busy', message: registryCopy},
        })

        const cancelling = transferReducer(staged(), {type: 'cancel-requested'})
        const reported = transferReducer(cancelling, {type: 'active-cancel-failed', sessionId, error: forged})
        expect(reported).toMatchObject({
            phase: 'staged', cancelPending: false,
            commandError: {code: 'busy', message: registryCopy},
        })
        expect(JSON.stringify(reported)).not.toContain('token=')
    })

    it('keeps retained terminal outcome when invalid selection supplies the visible command error', () => {
        const retained = {
            phase: 'idle',
            retainedOutcome: {kind: 'done'},
            commandError: null,
        } as const

        expect(transferReducer(retained, {type: 'invalid-selection'})).toEqual({
            phase: 'idle',
            retainedOutcome: {kind: 'done'},
            commandError: {
                code: 'invalid_selection',
                message: 'Choose exactly one file or folder.',
            },
        })
    })
})
