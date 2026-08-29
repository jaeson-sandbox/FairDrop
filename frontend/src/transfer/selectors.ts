import type {FileMetadata, ProgressSnapshot, PublicError, RetainedOutcome} from './types'
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
