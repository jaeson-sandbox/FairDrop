import type {
    FileMetadata,
    PendingItemKind,
    ProgressSnapshot,
    PublicError,
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

/**
 * The one place a percentage is derived, and the only progress repair layer.
 *
 * A `ProgressSnapshot` reaches this function from `parseProgressSnapshot` and
 * nowhere else, so its fields are already finite, non-negative and internally
 * coherent -- clamping them again here was a second strategy for a rule the
 * validator had already settled, and one whose branches could never run.
 *
 * The displayed percentage comes from the two authoritative integers rather
 * than from the wire's `percent`, so a sender that rounds a percentage for
 * display moves the bar by a rounding error instead of having every snapshot
 * refused (Epic 1 retrospective item 7). `bytesSent <= totalBytes` is the
 * validator's, which is what keeps the result inside [0,100]. Only
 * `known-positive` gets a value at all: an unknown total and an empty payload
 * have no percentage, and inventing one is what this shape exists to prevent.
 */
export function selectProgressSnapshot(progress: ProgressSnapshot): ProgressSelection {
    if (!progress.totalKnown) {
        return {
            mode: 'unknown',
            determinate: false,
            value: 0,
            bytesSent: progress.bytesSent,
            totalBytes: 0,
            speedBytesPerSec: progress.speedBytesPerSec,
        }
    }

    if (progress.totalBytes === 0) {
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
        value: 100 * progress.bytesSent / progress.totalBytes,
        bytesSent: progress.bytesSent,
        totalBytes: progress.totalBytes,
        speedBytesPerSec: progress.speedBytesPerSec,
    }
}

export function selectMetadata(state: TransferState): FileMetadata | null {
    return state.phase === 'staged' || state.phase === 'transferring' ? state.metadata : null
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
