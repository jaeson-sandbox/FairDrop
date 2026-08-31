import {describe, expect, it} from 'vitest'
import {publicError} from '../transfer/errors'
import {createInitialTransferState} from '../transfer/state'
import type {TransferState} from '../transfer/state'
import type {FileMetadata, Warning} from '../transfer/types'
import {focusSelector, focusTargets, routeTransition, type Announcement} from './announce'

const sessionId = '0123456789abcdef0123456789abcdef'

function metadata(overrides: Partial<FileMetadata> = {}): FileMetadata {
    return {
        sessionId,
        name: 'report.pdf',
        size: 8_400_000,
        isDir: false,
        url: `http://192.0.2.1:34123/download/${'fedcba9876543210fedcba9876543210'}`,
        qrBase64: 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=',
        warnings: [],
        ...overrides,
    }
}

const discovery: Warning = {code: 'beacon_warning', message: 'Device discovery isn’t available. ' +
    'The QR code and download link still work.'}

function idle(overrides: Partial<Extract<TransferState, {phase: 'idle'}>> = {}): TransferState {
    return {phase: 'idle', retainedOutcome: null, commandError: null, ...overrides}
}

function pending(cancelPending = false): TransferState {
    return {phase: 'pending', generation: 1, itemKind: 'file', cancelPending}
}

function staged(overrides: Partial<Extract<TransferState, {phase: 'staged'}>> = {}): TransferState {
    return {
        phase: 'staged',
        session: {sessionId, lastSeq: 0},
        metadata: metadata(),
        cancelPending: false,
        commandError: null,
        ...overrides,
    }
}

function transferring(overrides: Partial<Extract<TransferState, {phase: 'transferring'}>> = {}): TransferState {
    return {
        phase: 'transferring',
        session: {sessionId, lastSeq: 2},
        metadata: metadata(),
        progress: null,
        cancelPending: false,
        commandError: null,
        ...overrides,
    }
}

const done: TransferState = {phase: 'done', session: {sessionId, lastSeq: 5}, outcome: {kind: 'done'}}

const terminalError: TransferState = {
    phase: 'error',
    session: {sessionId, lastSeq: 5},
    outcome: {kind: 'error', error: publicError('transfer_failed')},
}

/*
  One case per row of the EXPERIENCE.md "Announcement ownership" table. The
  table's whole point is that no row ever produces both a focus move and a
  spoken update, which is what the shared invariant below checks for every case
  at once rather than case by case.
*/
const rows: Array<[string, TransferState, TransferState, Announcement | null]> = [
    [
        'Native dialog cancel',
        // A dismissed chooser dispatches nothing, so the reducer returns the
        // very same state object. No transition, no owner, no speech.
        staged(),
        staged(),
        null,
    ],
    ['Stage pending', idle(), pending(), {row: 'stage-pending', owner: 'focus', target: 'pending-heading'}],
    ['Stage success', pending(), staged(), {row: 'stage-success', owner: 'focus', target: 'staged-heading'}],
    [
        'Validation failure',
        idle(),
        idle({commandError: publicError('invalid_selection')}),
        {row: 'command-failure', owner: 'focus', target: 'command-error'},
    ],
    [
        'Command failure from a pending Stage',
        pending(),
        idle({commandError: publicError('path_not_found')}),
        {row: 'command-failure', owner: 'focus', target: 'command-error'},
    ],
    [
        'Command failure beside an active session',
        staged(),
        staged({commandError: publicError('shutting_down')}),
        {row: 'command-failure', owner: 'focus', target: 'command-error'},
    ],
    [
        'beacon_warning',
        staged(),
        staged({metadata: metadata({warnings: [discovery]})}),
        {
            row: 'beacon-warning',
            owner: 'announcer',
            text: 'Device discovery isn’t available. The QR code and download link still work.',
        },
    ],
    [
        'transfer-started',
        staged(),
        transferring(),
        {row: 'transfer-started', owner: 'focus', target: 'transferring-heading'},
    ],
    [
        'Throttled progress',
        // Owned by progressSpeech.ts, on a clock rather than on this table: a
        // snapshot that repainted the meter is not by itself an announcement.
        transferring(),
        transferring({progress: {
            bytesSent: 1_000, totalBytes: 8_400_000, totalKnown: true, percent: 1, speedBytesPerSec: 1_000,
        }}),
        null,
    ],
    [
        'Cancel requested during preparation',
        pending(),
        pending(true),
        {row: 'cancel-requested', owner: 'announcer', text: 'Canceling preparation…'},
    ],
    [
        'Cancel requested from Staged',
        staged(),
        staged({cancelPending: true}),
        {row: 'cancel-requested', owner: 'announcer', text: 'Canceling…'},
    ],
    [
        'Cancel requested from Transferring',
        transferring(),
        transferring({cancelPending: true}),
        {row: 'cancel-requested', owner: 'announcer', text: 'Canceling…'},
    ],
    [
        'Cancel-winning reset',
        transferring({cancelPending: true}),
        createInitialTransferState(),
        {row: 'cancel-won', owner: 'focus', target: 'cancel-summary'},
    ],
    ['Complete', transferring(), done, {row: 'terminal-outcome', owner: 'focus', target: 'outcome'}],
    ['Terminal Error', transferring(), terminalError, {row: 'terminal-outcome', owner: 'focus', target: 'outcome'}],
    [
        'Reset after terminal Done',
        done,
        idle({retainedOutcome: {kind: 'done'}}),
        null,
    ],
    [
        'Reset after terminal Error',
        terminalError,
        idle({retainedOutcome: {kind: 'error', error: publicError('transfer_failed')}}),
        null,
    ],
    [
        'Dismiss retained outcome',
        idle({retainedOutcome: {kind: 'done'}}),
        createInitialTransferState(),
        {row: 'dismiss-retained', owner: 'focus', target: 'idle-instruction'},
    ],
]

describe('the announcement-ownership routing table', () => {
    it.each(rows)('routes %s to its one owner', (_name, previous, next, expected) => {
        expect(routeTransition(previous, next)).toEqual(expected)
    })

    it('never gives one transition both a focus move and a spoken update', () => {
        for (const [name, previous, next] of rows) {
            const routed = routeTransition(previous, next)
            if (routed === null) continue

            const owners = [
                'target' in routed ? 'focus' : null,
                'text' in routed ? 'announcer' : null,
            ].filter((owner) => owner !== null)
            expect(owners, name).toHaveLength(1)
            expect(owners[0], name).toBe(routed.owner)
        }
    })

    it('names only targets a view can actually carry', () => {
        for (const [name, previous, next] of rows) {
            const routed = routeTransition(previous, next)
            if (routed === null || routed.owner !== 'focus') continue
            expect(focusTargets, name).toContain(routed.target)
        }
    })
})

describe('rows the table names but a reducer transition cannot produce', () => {
    it('leaves Stage success owning a session that arrives already warned', () => {
        // beacon_warning reaches the frontend inside the successful metadata, so
        // it lands on the same transition as Stage success. That transition has
        // one owner, and the table gives it to the focused heading: announcing
        // the warning as well would be the double speech this table exists to
        // stop. The warning is on the screen either way.
        const warned = staged({metadata: metadata({warnings: [discovery]})})

        expect(routeTransition(pending(), warned))
            .toEqual({row: 'stage-success', owner: 'focus', target: 'staged-heading'})
    })

    it('sends a terminal error carrying `cancelled` to the Idle summary, never to an outcome panel', () => {
        const cancelled: TransferState = {
            phase: 'error',
            session: {sessionId, lastSeq: 5},
            outcome: {kind: 'error', error: publicError('cancelled')},
        }

        expect(routeTransition(transferring(), cancelled))
            .toEqual({row: 'cancel-won', owner: 'focus', target: 'cancel-summary'})
    })

    it('stays silent after a terminal outcome even when no node was retained', () => {
        // The previous phase decides this row, not the shape of the Idle it
        // landed on. Reading only the destination would make a terminal that
        // retained nothing look exactly like a cancellation winning its race,
        // and announce a cancellation that never happened.
        expect(routeTransition(done, createInitialTransferState())).toBeNull()
        expect(routeTransition(terminalError, createInitialTransferState())).toBeNull()
    })

    it('says nothing when a reset lands on Idle with a retained outcome still attached', () => {
        expect(routeTransition(staged(), idle({retainedOutcome: {kind: 'done'}}))).toBeNull()
    })

    it('refuses to treat a `cancelled` command error as a failure worth focusing', () => {
        // selectCommandError will not render it, so there would be no panel to
        // focus. The transition still completes; it simply owns no owner.
        expect(routeTransition(staged(), staged({commandError: publicError('cancelled')}))).toBeNull()
    })

    it('announces a second command failure that repeats the first one’s code', () => {
        const first = staged({commandError: publicError('busy')})
        const second = staged({commandError: publicError('busy')})

        expect(routeTransition(first, second))
            .toEqual({row: 'command-failure', owner: 'focus', target: 'command-error'})
    })
})

describe('the focus selector', () => {
    it('addresses a target by the attribute the views carry', () => {
        expect(focusSelector('outcome')).toBe('[data-focus-target="outcome"]')
    })

    it('covers every target the table can name', () => {
        expect([...focusTargets]).toEqual([
            'idle-instruction',
            'cancel-summary',
            'command-error',
            'pending-heading',
            'staged-heading',
            'transferring-heading',
            'outcome',
        ])
    })
})
