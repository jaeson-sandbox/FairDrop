import {describe, expect, it} from 'vitest'
import {publicError} from './errors'
import {
    selectMetadata,
    selectProgress,
    selectProgressSnapshot,
    selectRetainedOutcome,
    selectVisibleError,
} from './selectors'
import type {TransferState} from './state'

const metadata = {
    sessionId: '0123456789abcdef0123456789abcdef',
    name: 'report.pdf',
    size: 100,
    isDir: false,
    url: 'http://192.0.2.1:34123/download/fedcba9876543210fedcba9876543210',
    qrBase64: 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=',
    warnings: [],
} as const

describe('progress presentation modes', () => {
    it('returns finite clamped determinate values for known positive totals', () => {
        expect(selectProgressSnapshot({
            bytesSent: 75,
            totalBytes: 100,
            totalKnown: true,
            percent: 75,
            speedBytesPerSec: 20,
        })).toEqual({
            mode: 'known-positive',
            determinate: true,
            value: 75,
            bytesSent: 75,
            totalBytes: 100,
            speedBytesPerSec: 20,
        })

        expect(selectProgressSnapshot({
            bytesSent: 100,
            totalBytes: 100,
            totalKnown: true,
            percent: 150,
            speedBytesPerSec: 20,
        }).value).toBe(100)
        expect(selectProgressSnapshot({
            bytesSent: 0,
            totalBytes: 100,
            totalKnown: true,
            percent: Number.POSITIVE_INFINITY,
            speedBytesPerSec: 20,
        }).value).toBe(0)
    })

    it('represents known empty totals explicitly without a determinate percentage', () => {
        expect(selectProgressSnapshot({
            bytesSent: 0,
            totalBytes: 0,
            totalKnown: true,
            percent: 0,
            speedBytesPerSec: 999,
        })).toEqual({
            mode: 'known-empty',
            determinate: false,
            value: 0,
            bytesSent: 0,
            totalBytes: 0,
            speedBytesPerSec: 0,
        })
    })

    it('represents unknown totals explicitly without division', () => {
        const selected = selectProgressSnapshot({
            bytesSent: 4096,
            totalBytes: 0,
            totalKnown: false,
            percent: 0,
            speedBytesPerSec: 512,
        })

        expect(selected).toEqual({
            mode: 'unknown',
            determinate: false,
            value: 0,
            bytesSent: 4096,
            totalBytes: 0,
            speedBytesPerSec: 512,
        })
        expect(Number.isFinite(selected.value)).toBe(true)
    })

    it.each([Number.NaN, Number.POSITIVE_INFINITY, -1])(
        'does not classify runtime-invalid known total %s as known-positive',
        (totalBytes) => {
            expect(selectProgressSnapshot({
                bytesSent: 25,
                totalBytes,
                totalKnown: true,
                percent: 50,
                speedBytesPerSec: 10,
            })).toEqual({
                mode: 'known-empty',
                determinate: false,
                value: 0,
                bytesSent: 0,
                totalBytes: 0,
                speedBytesPerSec: 0,
            })
        },
    )
})

describe('state-aware selectors', () => {
    it('selects progress and metadata only while their owning state retains them', () => {
        const progress = {bytesSent: 25, totalBytes: 100, totalKnown: true, percent: 25, speedBytesPerSec: 10}
        const transferring: TransferState = {
            phase: 'transferring',
            session: {sessionId: metadata.sessionId, lastSeq: 2},
            metadata,
            progress,
            cancelPending: false,
            commandError: null,
        }
        const done: TransferState = {
            phase: 'done', session: {sessionId: metadata.sessionId, lastSeq: 3}, outcome: {kind: 'done'},
        }

        expect(selectProgress(transferring)).toMatchObject({mode: 'known-positive', value: 25})
        expect(selectMetadata(transferring)).toBe(metadata)
        expect(selectProgress(done)).toBeNull()
        expect(selectMetadata(done)).toBeNull()
    })

    it('selects retained outcome and gives current command error precedence', () => {
        const retainedError = publicError('path_not_found')
        const commandError = publicError('invalid_selection')
        const idle: TransferState = {
            phase: 'idle',
            retainedOutcome: {kind: 'error', error: retainedError},
            commandError,
        }

        expect(selectRetainedOutcome(idle)).toEqual({kind: 'error', error: retainedError})
        expect(selectVisibleError(idle)).toEqual({
            code: 'invalid_selection', message: 'Choose exactly one file or folder.',
        })
    })

    it('selects active command and terminal errors from their owning states', () => {
        const staged: TransferState = {
            phase: 'staged',
            session: {sessionId: metadata.sessionId, lastSeq: 0},
            metadata,
            cancelPending: false,
            commandError: publicError('busy'),
        }
        const terminal: TransferState = {
            phase: 'error',
            session: {sessionId: metadata.sessionId, lastSeq: 3},
            outcome: {kind: 'error', error: publicError('source_changed')},
        }

        expect(selectVisibleError(staged)).toEqual({
            code: 'busy', message: 'Finish or cancel the current transfer before choosing another item.',
        })
        expect(selectVisibleError(terminal)).toEqual({
            code: 'source_changed',
            message: 'The item changed after it was prepared. Cancel and create a fresh link.',
        })
        expect(selectRetainedOutcome(terminal)).toBeNull()
    })
})
