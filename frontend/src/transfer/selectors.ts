import type {
    FileMetadata,
    PendingItemKind,
    ProgressSnapshot,
    PublicError,
    RetainedOutcome,
    Warning,
} from './types'
import type {TransferState} from './state'

export type ProgressSelection =
    | {
        readonly mode: 'known-positive'
        readonly determinate: true
        readonly value: number
        readonly bytesSent: number
        readonly totalBytes: number
        readonly speedBytesPerSec: number
    }
    | {
        readonly mode: 'known-empty'
        readonly determinate: false
        readonly value: 0
        readonly bytesSent: 0
        readonly totalBytes: 0
        readonly speedBytesPerSec: 0
    }
    | {
        readonly mode: 'unknown'
        readonly determinate: false
        readonly value: 0
        readonly bytesSent: number
        readonly totalBytes: 0
        readonly speedBytesPerSec: number
    }

export function selectProgress(state: TransferState): ProgressSelection | null {
    return state.phase === 'transferring' && state.progress !== null
        ? selectProgressSnapshot(state.progress)
        : null
}

export function selectProgressSnapshot(progress: ProgressSnapshot): ProgressSelection {
    if (!progress.totalKnown) {
        return {
            mode: 'unknown',
            determinate: false,
            value: 0,
            bytesSent: finiteNonNegative(progress.bytesSent),
            totalBytes: 0,
            speedBytesPerSec: finiteNonNegative(progress.speedBytesPerSec),
        }
    }

    const totalBytes = finiteNonNegative(progress.totalBytes)
    if (totalBytes === 0) {
        return {
            mode: 'known-empty',
            determinate: false,
            value: 0,
            bytesSent: 0,
            totalBytes: 0,
            speedBytesPerSec: 0,
        }
    }

    return {
        mode: 'known-positive',
        determinate: true,
        value: clampPercent(progress.percent),
        bytesSent: finiteNonNegative(progress.bytesSent),
        totalBytes,
        speedBytesPerSec: finiteNonNegative(progress.speedBytesPerSec),
    }
}

export function selectMetadata(state: TransferState): FileMetadata | null {
    return state.phase === 'staged' || state.phase === 'transferring' ? state.metadata : null
}

export function selectRetainedOutcome(state: TransferState): RetainedOutcome | null {
    return state.phase === 'idle' ? state.retainedOutcome : null
}

export function selectVisibleError(state: TransferState): PublicError | null {
    if (state.phase === 'error') return state.outcome.error
    if (state.phase === 'staged' || state.phase === 'transferring') return state.commandError
    if (state.phase !== 'idle') return null
    if (state.commandError !== null) return state.commandError
    return state.retainedOutcome?.kind === 'error' ? state.retainedOutcome.error : null
}

function clampPercent(value: number): number {
    if (!Number.isFinite(value)) return 0
    return Math.min(100, Math.max(0, value))
}

function finiteNonNegative(value: number): number {
    return Number.isFinite(value) ? Math.max(0, value) : 0
}

/** The one terminal outcome a view may render, terminal or retained. */
export type OutcomePresentation =
    | {readonly kind: 'done'; readonly retained: boolean}
    | {readonly kind: 'error'; readonly retained: boolean; readonly error: PublicError}

/**
 * The kind of item a pending Stage is preparing, or `null` outside Pending.
 *
 * `'unknown'` reaches a view unchanged. A native drop hands over a path and
 * nothing else, so the frontend cannot name the kind until metadata arrives;
 * resolving it to `'file'` here would put a claim in a selector where no view
 * could see it was invented.
 */
export function selectPendingItemKind(state: TransferState): PendingItemKind | null {
    return state.phase === 'pending' ? state.itemKind : null
}

/**
 * The non-terminal warnings carried by the current session's metadata.
 *
 * `beacon_warning` arrives with successful metadata and never changes phase,
 * so it lives here rather than anywhere near the error selectors.
 */
export function selectWarnings(state: TransferState): readonly Warning[] {
    const metadata = selectMetadata(state)
    return metadata === null ? emptyWarnings : metadata.warnings
}

/**
 * The command failure to show beside the current view, never a cancellation.
 *
 * The reducer already drops `cancelled` on every path that can set a command
 * error, so this guard is a second, local refusal: "never render `cancelled`
 * as an Error" holds even if a later reducer change forgets it.
 */
export function selectCommandError(state: TransferState): PublicError | null {
    if (state.phase !== 'idle' && state.phase !== 'staged' && state.phase !== 'transferring') return null
    const error = state.commandError
    if (error === null || error.code === 'cancelled') return null
    return error
}

/**
 * The Done or Error panel to render, whether it is the live terminal phase or
 * the same node retained in Idle after reset. `retained` is the only
 * difference the panel needs: it is what adds Dismiss.
 */
export function selectOutcome(state: TransferState): OutcomePresentation | null {
    switch (state.phase) {
        case 'done':
            return {kind: 'done', retained: false}
        case 'error':
            return outcomeError(state.outcome.error, false)
        case 'idle': {
            const retained = state.retainedOutcome
            if (retained === null) return null
            return retained.kind === 'done' ? {kind: 'done', retained: true} : outcomeError(retained.error, true)
        }
        default:
            return null
    }
}

function outcomeError(error: PublicError, retained: boolean): OutcomePresentation | null {
    return error.code === 'cancelled' ? null : {kind: 'error', retained, error}
}

const emptyWarnings: readonly Warning[] = Object.freeze([])
