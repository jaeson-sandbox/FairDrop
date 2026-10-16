import {describe, expect, it} from 'vitest'
import {transferErrorCodes} from './errors'
import {
    selectCommandContext,
    selectCommandError,
    selectOutcome,
    selectReceiveErrorAction,
    selectReceiveOutcome,
    selectReceiveProgress,
    selectWarnings,
} from './selectors'
import type {
    ReceiveOutcomeTransferState,
    ReceiveReceivingTransferState,
    ReceiveWaitingTransferState,
} from './state'

const sessionId = '0102030405060708090a0b0c0d0e0f10'
const metadata = {
    sessionId,
    destination: 'Phone Photos',
    url: 'http://192.168.1.50:45678/upload/1112131415161718191a1b1c1d1e1f20',
    qrBase64: 'iVBORw0KGgo=',
    warnings: [{code: 'beacon_warning' as const, message: 'Device discovery isn’t available. The QR code and download link still work.'}],
}

const waiting: ReceiveWaitingTransferState = {
    phase: 'receive-waiting', session: {sessionId, lastSeq: 0}, metadata, cancelPending: false, commandError: null, refusals: 0,
}

const receiving: ReceiveReceivingTransferState = {
    phase: 'receive-receiving',
    session: {sessionId, lastSeq: 1},
    metadata,
    progress: {bytesSent: 100, totalBytes: 400, totalKnown: true, percent: 25, speedBytesPerSec: 50},
    filesSaved: 1,
    cancelPending: false,
    commandError: null,
}

const outcome: ReceiveOutcomeTransferState = {
    phase: 'receive-outcome',
    session: {sessionId, lastSeq: 3},
    destination: 'Phone Photos',
    outcome: {kind: 'complete', filesSaved: 2, subfolderExists: true, markingWarning: false},
    commandError: null,
}

describe('receive progress', () => {
    it('is null until the first accepted snapshot and outside receiving', () => {
        expect(selectReceiveProgress({...receiving, progress: null})).toBeNull()
        expect(selectReceiveProgress(waiting)).toBeNull()
        expect(selectReceiveProgress(outcome)).toBeNull()
    })

    it('derives the percentage from the two authoritative integers, not the wire percent', () => {
        const selection = selectReceiveProgress({
            ...receiving,
            progress: {bytesSent: 100, totalBytes: 400, totalKnown: true, percent: 99, speedBytesPerSec: 50},
        })
        expect(selection).toEqual({
            mode: 'known-positive', determinate: true, value: 25, bytesSent: 100, totalBytes: 400, speedBytesPerSec: 50,
        })
    })
})

describe('receive outcome and warnings', () => {
    it('exposes the held outcome only in the outcome phase', () => {
        expect(selectReceiveOutcome(outcome)).toBe(outcome)
        expect(selectReceiveOutcome(receiving)).toBeNull()
        expect(selectReceiveOutcome(waiting)).toBeNull()
    })

    it('never offers a receive outcome as a send outcome card', () => {
        expect(selectOutcome(outcome)).toBeNull()
        expect(selectOutcome(receiving)).toBeNull()
        expect(selectOutcome(waiting)).toBeNull()
    })

    it('reads the waiting session\'s warnings, and no other receive phase has any', () => {
        expect(selectWarnings(waiting)).toEqual(metadata.warnings)
        expect(selectWarnings(receiving)).toEqual([])
        expect(selectWarnings(outcome)).toEqual([])
    })
})

describe('command errors and their context', () => {
    it('shows a command error beside every receive phase and never a cancellation', () => {
        const error = {code: 'clipboard_failed' as const, message: 'x'}
        expect(selectCommandError({...waiting, commandError: error})).toBe(error)
        expect(selectCommandError({...receiving, commandError: error})).toBe(error)
        expect(selectCommandError({...outcome, commandError: error})).toBe(error)
        expect(selectCommandError({...waiting, commandError: {code: 'cancelled', message: 'x'}})).toBeNull()
    })

    it('reports the receive context only for an Idle failure that came from Receive', () => {
        const failure = {code: 'path_not_found' as const, message: 'x'}
        expect(selectCommandContext({phase: 'idle', retainedOutcome: null, commandError: failure, commandContext: 'receive'})).toBe('receive')
        expect(selectCommandContext({phase: 'idle', retainedOutcome: null, commandError: failure})).toBeNull()
        expect(selectCommandContext(waiting)).toBeNull()
    })
})

describe('the receive failure card action table', () => {
    // Written out per code: the receive actions are derived from the owner-approved send table, and
    // this is the literal that proves the derivation lands where intended.
    const expected: Record<string, 'again' | 'choose' | 'dismiss' | null> = {
        invalid_selection: 'choose',
        invalid_text: 'again',
        busy: 'again',
        cancelled: null,
        path_not_found: 'choose',
        path_unsupported: 'choose',
        source_changed: 'again',
        network_unavailable: 'again',
        server_start_failed: 'again',
        qr_failed: 'again',
        setup_failed: 'again',
        beacon_warning: null,
        transfer_failed: 'again',
        cleanup_unconfirmed: 'dismiss',
        not_ready: 'dismiss',
        clipboard_failed: null,
        name_unsupported: 'again',
        name_warning: null,
        shutting_down: 'dismiss',
        chooser_failed: 'choose',
    }

    it.each(Object.entries(expected))('maps %s to %s', (code, action) => {
        expect(selectReceiveErrorAction(code as never)).toBe(action)
    })

    it('covers every stable code, so a new code cannot arrive without a decision', () => {
        expect(Object.keys(expected).sort()).toEqual([...transferErrorCodes].sort())
    })
})
