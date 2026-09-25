import {describe, expect, it} from 'vitest'
import {publicError, transferErrorCodes} from './errors'
import {
    selectCommandError,
    selectEffectiveErrorAction,
    selectErrorAction,
    selectMetadata,
    selectOutcome,
    selectPendingItemKind,
    selectProgress,
    selectProgressSnapshot,
    selectWarnings,
} from './selectors'
import type {TransferState} from './state'
import type {ErrorAction, TransferErrorCode} from './types'
import {parseProgressSnapshot} from './validation'

const metadata = {
    sessionId: '0123456789abcdef0123456789abcdef',
    name: 'report.pdf',
    size: 100,
    isDir: false,
    url: 'http://192.0.2.1:34123/download/fedcba9876543210fedcba9876543210',
    qrBase64: 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=',
    warnings: [],
} as const

const finalProgress = {
    bytesSent: 100, totalBytes: 100, totalKnown: true, percent: 100, speedBytesPerSec: 0,
} as const

const doneReceipt = {name: metadata.name, isDir: metadata.isDir, bytesSent: finalProgress.bytesSent} as const

describe('progress presentation modes', () => {
    it('derives the determinate percentage from the authoritative byte pair', () => {
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

        // The wire's own percent does not fill the bar. A sender that rounded
        // it for display would otherwise put a figure on screen that disagrees
        // with the byte counts printed beside it (Epic 1 retrospective item 7).
        expect(selectProgressSnapshot({
            bytesSent: 58,
            totalBytes: 84,
            totalKnown: true,
            percent: 68,
            speedBytesPerSec: 20,
        }).value).toBeCloseTo(100 * 58 / 84, 12)
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

/*
      One layer decides coherence, and it is not this one.

      The selector used to re-clamp these three totals into known-empty. That
      branch could never run: parseProgressSnapshot is the only producer of a
      ProgressSnapshot in the app, and it refuses all three before any selector
      sees them -- a rejecting layer followed by a repairing layer, two
      strategies for one rule (Epic 1 retrospective item 7). The guarantee the
      repair stood for is asserted here against the layer that actually owns it,
      so deleting the dead branch did not delete the promise.
    */
    it.each([Number.NaN, Number.POSITIVE_INFINITY, -1])(
        'never lets a runtime-invalid known total %s reach a selector at all',
        (totalBytes) => {
            expect(parseProgressSnapshot({
                bytesSent: 25,
                totalBytes,
                totalKnown: true,
                percent: 50,
                speedBytesPerSec: 10,
            })).toBeNull()
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
            phase: 'done',
            session: {sessionId: metadata.sessionId, lastSeq: 3},
            outcome: {kind: 'done', receipt: doneReceipt},
        }

        expect(selectProgress(transferring)).toMatchObject({mode: 'known-positive', value: 25})
        expect(selectMetadata(transferring)).toBe(metadata)
        expect(selectProgress(done)).toBeNull()
        expect(selectMetadata(done)).toBeNull()
    })

    // The two selectors this used to exercise -- selectVisibleError and
    // selectRetainedOutcome -- were removed by Story 1.10. They were dead and
    // they contradicted these replacements: selectVisibleError folded a
    // retained terminal error into "the visible error" and did not refuse
    // `cancelled`, which are the two behaviours selectOutcome and
    // selectCommandError exist to prevent.
    it('keeps a retained outcome and a current command failure separate', () => {
        const retainedError = publicError('path_not_found')
        const commandError = publicError('invalid_selection')
        const idle: TransferState = {
            phase: 'idle',
            retainedOutcome: {kind: 'error', error: retainedError},
            commandError,
        }

        expect(selectOutcome(idle)).toEqual({kind: 'error', retained: true, error: retainedError})
        expect(selectCommandError(idle)).toEqual({
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

        expect(selectCommandError(staged)).toEqual({
            code: 'busy',
            message: 'FairDrop is still finishing the last item. If it doesn’t finish, close FairDrop and reopen it.',
        })
        expect(selectOutcome(terminal)).toEqual({
            kind: 'error',
            retained: false,
            error: {
                code: 'source_changed',
                message: 'The item changed after it was prepared. Cancel and create a fresh link.',
            },
        })
        // A terminal Error is a phase, not a retained node: nothing in Idle yet.
        expect(selectCommandError(terminal)).toBeNull()
    })
})

describe('pending item kind', () => {
    it('reports the kind a browse command supplied', () => {
        expect(selectPendingItemKind({phase: 'pending', generation: 1, itemKind: 'file', cancelPending: false}))
            .toBe('file')
        expect(selectPendingItemKind({phase: 'pending', generation: 1, itemKind: 'directory', cancelPending: false}))
            .toBe('directory')
    })

    it('passes a native drop through as unknown rather than guessing', () => {
        expect(selectPendingItemKind({phase: 'pending', generation: 1, itemKind: 'unknown', cancelPending: false}))
            .toBe('unknown')
    })

    it('reports nothing outside the pending phase', () => {
        expect(selectPendingItemKind({phase: 'idle', retainedOutcome: null, commandError: null})).toBeNull()
    })
})

describe('staged warnings', () => {
    const warning = {
        code: 'beacon_warning',
        message: 'Device discovery isn’t available. The QR code and download link still work.',
    } as const

    it('exposes the warnings carried by the live session metadata', () => {
        const staged: TransferState = {
            phase: 'staged',
            session: {sessionId: metadata.sessionId, lastSeq: 0},
            metadata: {...metadata, warnings: [warning]},
            cancelPending: false,
            commandError: null,
        }

        expect(selectWarnings(staged)).toEqual([warning])
    })

    it('keeps carrying them once the transfer starts', () => {
        const transferring: TransferState = {
            phase: 'transferring',
            session: {sessionId: metadata.sessionId, lastSeq: 1},
            metadata: {...metadata, warnings: [warning]},
            progress: null,
            cancelPending: false,
            commandError: null,
        }

        expect(selectWarnings(transferring)).toEqual([warning])
    })

    it('returns an empty list where no session exists', () => {
        expect(selectWarnings({phase: 'idle', retainedOutcome: null, commandError: null})).toEqual([])
        expect(selectWarnings({phase: 'pending', generation: 1, itemKind: 'file', cancelPending: false})).toEqual([])
    })
})

describe('command errors', () => {
    // A Cancel that fails mid-transfer is the one way a command error reaches
    // Transferring, and nothing exercised it: dropping 'transferring' from the
    // guard left all 310 tests green.
    it('selects the command error while a transfer is running', () => {
        expect(selectCommandError({
            phase: 'transferring',
            session: {sessionId: '0123456789abcdef0123456789abcdef', lastSeq: 2},
            metadata: {
                sessionId: '0123456789abcdef0123456789abcdef',
                name: 'report.pdf',
                size: 100,
                isDir: false,
                url: 'http://192.0.2.1:34123/download/fedcba9876543210fedcba9876543210',
                qrBase64: 'iVBORw0KGgo=',
                warnings: [],
            },
            progress: null,
            cancelPending: false,
            commandError: publicError('shutting_down'),
        })).toEqual({
            code: 'shutting_down',
            message: 'FairDrop is closing. Reopen it to start a transfer.',
        })
    })

    it('selects the command error of whichever phase owns one', () => {
        expect(selectCommandError({
            phase: 'idle',
            retainedOutcome: null,
            commandError: publicError('invalid_selection'),
        })).toEqual({code: 'invalid_selection', message: 'Choose exactly one file or folder.'})

        expect(selectCommandError({
            phase: 'staged',
            session: {sessionId: metadata.sessionId, lastSeq: 0},
            metadata,
            cancelPending: false,
            commandError: publicError('busy'),
        })?.code).toBe('busy')
    })

    it('refuses to surface a cancellation as a command error', () => {
        expect(selectCommandError({
            phase: 'idle',
            retainedOutcome: null,
            commandError: publicError('cancelled'),
        })).toBeNull()
    })

    it('ignores a retained terminal error, which the outcome selector owns', () => {
        expect(selectCommandError({
            phase: 'idle',
            retainedOutcome: {kind: 'error', error: publicError('transfer_failed')},
            commandError: null,
        })).toBeNull()
    })
})

describe('terminal and retained outcomes', () => {
    it('presents a live terminal outcome as not retained', () => {
        expect(selectOutcome({
            phase: 'done',
            session: {sessionId: metadata.sessionId, lastSeq: 4},
            outcome: {kind: 'done', receipt: doneReceipt},
        })).toEqual({kind: 'done', retained: false, receipt: doneReceipt})

        expect(selectOutcome({
            phase: 'error',
            session: {sessionId: metadata.sessionId, lastSeq: 4},
            outcome: {kind: 'error', error: publicError('transfer_failed')},
        })).toEqual({
            kind: 'error',
            retained: false,
            error: {
                code: 'transfer_failed',
                message: 'The transfer stopped before FairDrop finished sending. ' +
                    'Check the local network and create a fresh link.',
            },
        })
    })

    it('presents the same outcome as retained once reset has cleared the session', () => {
        expect(selectOutcome({
            phase: 'idle',
            retainedOutcome: {kind: 'done', receipt: doneReceipt},
            commandError: null,
        })).toEqual({kind: 'done', retained: true, receipt: doneReceipt})
        expect(selectOutcome({
            phase: 'idle',
            retainedOutcome: {kind: 'error', error: publicError('source_changed')},
            commandError: null,
        })?.retained).toBe(true)
    })

    /*
      Given a retained Done outcome in Idle, it carries the same retained
      receipt a live Done carried -- so a reset does not empty the panel the
      sender is still looking at.
    */
    it('carries the same retained receipt a live Done outcome carried', () => {
        const live = selectOutcome({
            phase: 'done',
            session: {sessionId: metadata.sessionId, lastSeq: 4},
            outcome: {kind: 'done', receipt: doneReceipt},
        })
        const retained = selectOutcome({
            phase: 'idle',
            retainedOutcome: {kind: 'done', receipt: doneReceipt},
            commandError: null,
        })

        expect(live?.kind).toBe('done')
        expect(retained?.kind).toBe('done')
        expect(live?.kind === 'done' ? live.receipt : null)
            .toEqual(retained?.kind === 'done' ? retained.receipt : null)
    })

    /*
      `selectOutcome` only forwards `CompletionReceipt`; it computes nothing.
      The wire-bytes-vs-logical-size guarantee (Story 7.4 AC3) is proven once,
      where the receipt is actually built, in state.test.ts -- there is no
      `metadata.size` in scope at this layer for a selector to substitute.
      This instead proves the pass-through is exact, for a directory outcome
      Story 7.5 will need to phrase differently (`receipt.isDir`).
    */
    it('forwards the receipt unchanged, including for a directory outcome', () => {
        const dirReceipt = {name: 'papers', isDir: true, bytesSent: 4_096} as const

        const outcome = selectOutcome({
            phase: 'done',
            session: {sessionId: metadata.sessionId, lastSeq: 4},
            outcome: {kind: 'done', receipt: dirReceipt},
        })

        expect(outcome).toEqual({kind: 'done', retained: false, receipt: dirReceipt})
    })

    it('refuses to present a cancellation as an outcome panel', () => {
        expect(selectOutcome({
            phase: 'error',
            session: {sessionId: metadata.sessionId, lastSeq: 4},
            outcome: {kind: 'error', error: publicError('cancelled')},
        })).toBeNull()
        expect(selectOutcome({
            phase: 'idle',
            retainedOutcome: {kind: 'error', error: publicError('cancelled')},
            commandError: null,
        })).toBeNull()
    })

    it('presents nothing while a session is live or preparing', () => {
        expect(selectOutcome({phase: 'idle', retainedOutcome: null, commandError: null})).toBeNull()
        expect(selectOutcome({phase: 'pending', generation: 1, itemKind: 'file', cancelPending: false})).toBeNull()
        expect(selectOutcome({
            phase: 'staged',
            session: {sessionId: metadata.sessionId, lastSeq: 0},
            metadata,
            cancelPending: false,
            commandError: null,
        })).toBeNull()
    })

    /*
      Story 9.2 AC5: the retained item name is a pass-through, exactly like
      the receipt above -- the selector computes nothing, it only forwards
      whatever the reducer already retained.
    */
    it('forwards the retained item name for both a live and a retained Error outcome', () => {
        const live = selectOutcome({
            phase: 'error',
            session: {sessionId: metadata.sessionId, lastSeq: 4},
            outcome: {kind: 'error', error: publicError('transfer_failed'), itemName: 'secret-report.pdf'},
        })
        expect(live).toEqual({
            kind: 'error', retained: false, error: publicError('transfer_failed'), itemName: 'secret-report.pdf',
        })

        const retained = selectOutcome({
            phase: 'idle',
            retainedOutcome: {kind: 'error', error: publicError('source_changed'), itemName: 'secret-report.pdf'},
            commandError: null,
        })
        expect(retained).toEqual({
            kind: 'error', retained: true, error: publicError('source_changed'), itemName: 'secret-report.pdf',
        })
    })

    it('carries no item name for a Stage-time command failure, which never retained one', () => {
        const outcome = selectOutcome({
            phase: 'error',
            session: {sessionId: metadata.sessionId, lastSeq: 4},
            outcome: {kind: 'error', error: publicError('transfer_failed')},
        })
        expect(outcome?.kind).toBe('error')
        expect(outcome && 'itemName' in outcome ? outcome.itemName : undefined).toBeUndefined()
    })
})

/*
  Story 9.2 AC2: the owner-approved code -> action table.

  Written out as literals at the assertion site, per AGENTS.md's testing
  standards -- asserting against the implementation's own table would let the
  table drift with nothing to notice. Each code is checked individually
  against its own literal so a mutation that moves one code to another row
  fails naming that exact code, not just "the table changed somewhere".
*/
describe('selectErrorAction (Story 9.2 AC2)', () => {
    const expected: Readonly<Record<TransferErrorCode, ErrorAction | null>> = {
        busy: 'retry',
        source_changed: 'retry',
        network_unavailable: 'retry',
        server_start_failed: 'retry',
        qr_failed: 'retry',
        setup_failed: 'retry',
        transfer_failed: 'retry',
        name_unsupported: 'retry',
        invalid_selection: 'choose',
        path_not_found: 'choose',
        path_unsupported: 'choose',
        chooser_failed: 'choose',
        cleanup_unconfirmed: 'dismiss',
        not_ready: 'dismiss',
        shutting_down: 'dismiss',
        cancelled: null,
        clipboard_failed: null,
        beacon_warning: null,
        name_warning: null,
    }

    it('returns retry for busy, source_changed, network_unavailable, server_start_failed, qr_failed, setup_failed, transfer_failed, name_unsupported', () => {
        expect(selectErrorAction('busy'), 'busy').toBe('retry')
        expect(selectErrorAction('source_changed'), 'source_changed').toBe('retry')
        expect(selectErrorAction('network_unavailable'), 'network_unavailable').toBe('retry')
        expect(selectErrorAction('server_start_failed'), 'server_start_failed').toBe('retry')
        expect(selectErrorAction('qr_failed'), 'qr_failed').toBe('retry')
        expect(selectErrorAction('setup_failed'), 'setup_failed').toBe('retry')
        expect(selectErrorAction('transfer_failed'), 'transfer_failed').toBe('retry')
        expect(selectErrorAction('name_unsupported'), 'name_unsupported').toBe('retry')
    })

    it('returns choose for invalid_selection, path_not_found, path_unsupported, chooser_failed', () => {
        expect(selectErrorAction('invalid_selection'), 'invalid_selection').toBe('choose')
        expect(selectErrorAction('path_not_found'), 'path_not_found').toBe('choose')
        expect(selectErrorAction('path_unsupported'), 'path_unsupported').toBe('choose')
        expect(selectErrorAction('chooser_failed'), 'chooser_failed').toBe('choose')
    })

    it('returns dismiss for cleanup_unconfirmed, not_ready, shutting_down', () => {
        expect(selectErrorAction('cleanup_unconfirmed'), 'cleanup_unconfirmed').toBe('dismiss')
        expect(selectErrorAction('not_ready'), 'not_ready').toBe('dismiss')
        expect(selectErrorAction('shutting_down'), 'shutting_down').toBe('dismiss')
    })

    it('returns no action for cancelled, clipboard_failed, beacon_warning, name_warning', () => {
        expect(selectErrorAction('cancelled'), 'cancelled').toBeNull()
        expect(selectErrorAction('clipboard_failed'), 'clipboard_failed').toBeNull()
        expect(selectErrorAction('beacon_warning'), 'beacon_warning').toBeNull()
        expect(selectErrorAction('name_warning'), 'name_warning').toBeNull()
    })

    // *Mutation:* move any one code to another row -> the per-code assertion
    // above for that exact code fails and names it.
    it('matches the fixture table above for every code the fixture knows about', () => {
        for (const code of transferErrorCodes) {
            expect(selectErrorAction(code), `selectErrorAction(${JSON.stringify(code)})`).toBe(expected[code])
        }
    })

    /*
      A new `TransferErrorCode` added to the registry without a row in
      `errorActionByCode` fails *this* test, naming the code, independently of
      the fixture `expected` map above (which a careless addition could leave
      untouched) and independently of a type-check pass: the production
      `Record<TransferErrorCode, ...>` in selectors.ts would also be a
      compile-time error for the same omission, but this walks the live
      canonical list (`transferErrorCodes`) at runtime, so the gap cannot hide
      behind a build step nobody ran.

      *Mutation:* add a code to `transferErrorCodes` (errors.ts) without a
      matching row in `errorActionByCode` -> `selectErrorAction` returns
      `undefined` for it and this test fails, naming that exact code.
    */
    it('never returns undefined for any code currently in the canonical registry', () => {
        for (const code of transferErrorCodes) {
            expect(
                selectErrorAction(code),
                `selectErrorAction(${JSON.stringify(code)}) is undefined -- add a row for this code`,
            ).not.toBe(undefined)
        }
    })
})

describe('selectEffectiveErrorAction (Story 9.2 AC3)', () => {
    it('downgrades a retry row to choose when no path is remembered to retry with', () => {
        expect(selectEffectiveErrorAction('transfer_failed', false)).toBe('choose')
        expect(selectEffectiveErrorAction('busy', false)).toBe('choose')
    })

    it('keeps retry when a path is remembered', () => {
        expect(selectEffectiveErrorAction('transfer_failed', true)).toBe('retry')
    })

    it('leaves choose, dismiss, and no-action rows unaffected by canRetry either way', () => {
        for (const canRetry of [true, false]) {
            expect(selectEffectiveErrorAction('path_not_found', canRetry)).toBe('choose')
            expect(selectEffectiveErrorAction('shutting_down', canRetry)).toBe('dismiss')
            expect(selectEffectiveErrorAction('cancelled', canRetry)).toBeNull()
        }
    })
})
