import {describe, expect, it} from 'vitest'
import {publicError} from './errors'
import {createInitialTransferState, transferReducer, type TransferState} from './state'
import type {FileMetadata, PublicError} from './types'

const sessionId = '0123456789abcdef0123456789abcdef'
const qrPNG = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII='

function metadata(overrides: Partial<FileMetadata> = {}): FileMetadata {
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

function staged(metadataOverrides: Partial<FileMetadata> = {}): TransferState {
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
        // Malformed metadata is refused one layer out, by the controller that
        // received it -- which quiesces the backend session and reports
        // setup_failed rather than leaving the window in Pending. Pinned by
        // useTransfer.test.tsx's "attempts Cancel exactly once for malformed
        // successful metadata"; the reducer's parameter type is what makes an
        // unparsed acknowledgement unable to reach this case at all.

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
            outcome: {kind: 'done', metadata: metadata(), progress: progress(100)},
        })

        state = event(state, 'transfer-reset', {sessionId, seq: 4})
        expect(state).toEqual({
            phase: 'idle',
            retainedOutcome: {kind: 'done', metadata: metadata(), progress: progress(100)},
            commandError: null,
        })
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

    it('drops disagreeing progress without consuming sequence, but never drops a terminal event', () => {
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

        /*
          A terminal event with a disagreeing snapshot ends the session.

          Dropping it left the view in Transferring until the backend's reset
          landed three seconds later, and transferring -> plain idle is the
          cancel-won row: a transfer that had actually completed was announced
          as "Transfer canceled" (Epic 1 retrospective item 2). What the
          refusal costs is the claim of success, not the end of the session.
        */
        expect(event(state, 'transfer-complete', {
            sessionId, seq: 3, progress: progress(50, 50),
        })).toMatchObject({
            phase: 'error',
            session: {lastSeq: 3},
            outcome: {kind: 'error', error: {code: 'transfer_failed'}},
        })
        expect(event(state, 'transfer-error', {
            sessionId, seq: 3, progress: progress(50, 50),
            error: {code: 'transfer_failed', message: 'forged'},
        })).toMatchObject({
            phase: 'error',
            session: {lastSeq: 3},
            outcome: {kind: 'error', error: {code: 'transfer_failed'}},
        })

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

describe('terminal receipt retention (Story 7.4)', () => {
    /*
      Reversed from the pre-7.4 behaviour this test used to pin: the reducer
      used to discard `metadata` and the `transfer-complete` snapshot at this
      exact transition, which is why the finished-transfer screen had nothing
      left to draw (Epic 7's diagnosis). Both values are already in hand here
      and are now retained on `DoneTransferState.outcome`, which is what lets
      Story 7.5 render a receipt instead of an empty window.
    */
    it('retains the session metadata and the final progress snapshot at transfer-complete', () => {
        let state = event(staged(), 'transfer-started', {sessionId, seq: 1})
        state = event(state, 'transfer-complete', {sessionId, seq: 2, progress: progress(100)})

        expect(state).toMatchObject({
            phase: 'done',
            outcome: {kind: 'done', metadata: {name: 'report.pdf'}, progress: {bytesSent: 100}},
        })
        // *Mutation:* drop `metadata` or `progress` from the outcome -> must fail here.
        const outcome = state.phase === 'done' ? state.outcome : null
        expect(outcome && 'metadata' in outcome, 'DoneTransferState.outcome must carry metadata').toBe(true)
        expect(outcome && 'progress' in outcome, 'DoneTransferState.outcome must carry progress').toBe(true)
    })

    /*
      Wire bytes, not logical size. A file's `progress.totalBytes` is forced
      equal to `metadata.size` by `progressMatchesMetadata`, and a known-total
      `transfer-complete` is refused unless `bytesSent === totalBytes` --
      so for a plain file the two numbers coincide and cannot tell a correct
      receipt from a broken one. An unknown-total directory transfer has no
      such constraint: the wire bytes actually sent are unrelated to the
      directory's `size` placeholder, which is what this test exploits.
    */
    it('shows the wire bytes actually sent, never metadata.size', () => {
        let state = event(staged({name: 'papers', size: 0, isDir: true}), 'transfer-started', {sessionId, seq: 1})
        const unknownFinal = {bytesSent: 4_096, totalBytes: 0, totalKnown: false, percent: 0, speedBytesPerSec: 512}
        state = event(state, 'transfer-complete', {sessionId, seq: 2, progress: unknownFinal})

        const receiptBytes = state.phase === 'done' ? state.outcome.progress.bytesSent : null
        const logicalSize = state.phase === 'done' ? state.outcome.metadata.size : null
        // *Mutation:* substitute `metadata.size` for `progress.bytesSent` -> must fail:
        // logicalSize is 0 here while the receipt figure must be 4096.
        expect(receiptBytes, 'the receipt must read progress.bytesSent').toBe(4_096)
        expect(logicalSize).toBe(0)
    })

    it('carries the same retained metadata and progress into Idle after reset, so reset does not empty the panel', () => {
        let state = event(staged(), 'transfer-started', {sessionId, seq: 1})
        state = event(state, 'transfer-complete', {sessionId, seq: 2, progress: progress(100)})
        const doneOutcome = state.phase === 'done' ? state.outcome : null
        expect(doneOutcome).not.toBeNull()

        state = event(state, 'transfer-reset', {sessionId, seq: 3})

        expect(state).toEqual({
            phase: 'idle',
            retainedOutcome: {kind: 'done', metadata: doneOutcome!.metadata, progress: doneOutcome!.progress},
            commandError: null,
        })

        // Session correlation (the sequence cursor) is still scrubbed on reset;
        // only the receipt's two retained values survive it.
        const serialized = JSON.stringify(state)
        expect(serialized).not.toContain('lastSeq')
        expect(serialized).not.toContain('"session"')
    })

    /*
      *Mutation:* add an elapsed-time field anywhere on the Done outcome ->
      this must fail and name the extra key. No clock is tracked anywhere in
      this product and EXPERIENCE.md forbids frontend lifecycle timers, so a
      duration shown on the receipt could only be invented.
    */
    it('never gains a duration, elapsed, or timer field on Done -- no clock exists to have measured one', () => {
        let state = event(staged(), 'transfer-started', {sessionId, seq: 1})
        state = event(state, 'transfer-complete', {sessionId, seq: 2, progress: progress(100)})

        const outcomeKeys = state.phase === 'done' ? Object.keys(state.outcome).sort() : []
        expect(
            outcomeKeys,
            'DoneTransferState.outcome must carry exactly kind/metadata/progress -- any other key ' +
            'is presumed to be an invented duration/elapsed/timer field, which EXPERIENCE.md bans',
        ).toEqual(['kind', 'metadata', 'progress'])

        state = event(state, 'transfer-reset', {sessionId, seq: 3})
        const retainedKeys = state.phase === 'idle' && state.retainedOutcome !== null
            ? Object.keys(state.retainedOutcome).sort()
            : []
        expect(
            retainedKeys,
            'RetainedDoneOutcome must carry exactly kind/metadata/progress -- no duration field is permitted',
        ).toEqual(['kind', 'metadata', 'progress'])
    })

    it('adds nothing to the Error outcome shape: this story only retains a receipt for Done', () => {
        let state = event(staged(), 'transfer-started', {sessionId, seq: 1})
        state = event(state, 'transfer-error', {
            sessionId, seq: 2, error: {code: 'transfer_failed', message: 'x'},
        })

        const outcomeKeys = state.phase === 'error' ? Object.keys(state.outcome).sort() : []
        expect(outcomeKeys).toEqual(['error', 'kind'])
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
        const registryCopy = 'FairDrop is still finishing the last item. If it doesn’t finish, close FairDrop and reopen it.'

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
            retainedOutcome: {kind: 'done', metadata: metadata(), progress: progress(100)},
            commandError: null,
        } as unknown as TransferState

        expect(transferReducer(retained, {type: 'invalid-selection'})).toEqual({
            phase: 'idle',
            retainedOutcome: {kind: 'done', metadata: metadata(), progress: progress(100)},
            commandError: {
                code: 'invalid_selection',
                message: 'Choose exactly one file or folder.',
            },
        })
    })
})

/*
  The clipboard command's rejection, which used to have nowhere to go.

  `clipboard_failed` has carried a registry message since Story 3.11 and could
  not reach a user: the staged view discarded the rejection. This is the action
  that carries it, and it is scoped to a session rather than to a phase, because
  the command outlives the render that issued it.
*/
describe('clipboard-failed', () => {
    const failed = {type: 'clipboard-failed', sessionId} as const

    it('shows the registry message for the session that issued the command', () => {
        const next = transferReducer(staged(), failed)

        expect(next.phase).toBe('staged')
        expect(next.phase === 'staged' ? next.commandError : null).toEqual({
            code: 'clipboard_failed',
            message: 'FairDrop couldn’t copy the link. Select the link and copy it yourself.',
        })
    })

    it('ignores a rejection naming a session that is no longer the staged one', () => {
        const before = staged()
        const other = 'ffffffffffffffffffffffffffffffff'

        expect(transferReducer(before, {type: 'clipboard-failed', sessionId: other})).toBe(before)
    })

    /*
      A session being retired has already said so. `cancel-requested` is a
      spoken row, and a failed copy is not a reason to replace the one answer
      the user is waiting on -- the same rule `active-cancel-failed` follows
      from the other direction.
    */
    it('ignores a rejection that lands while the session is being cancelled', () => {
        const cancelling = transferReducer(staged(), {type: 'cancel-requested'})

        expect(transferReducer(cancelling, failed)).toBe(cancelling)
    })

    it('ignores a rejection in a phase that has no copy control', () => {
        const idle = createInitialTransferState()
        const transferring = event(staged(), 'transfer-started', {sessionId, seq: 1})

        expect(transferReducer(idle, failed)).toBe(idle)
        expect(transferReducer(transferring, failed)).toBe(transferring)
    })
})
