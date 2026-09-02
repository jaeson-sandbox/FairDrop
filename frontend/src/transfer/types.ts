/** Runtime-facing transfer types. Every field name mirrors the Wails JSON contract. */

import type {TransferErrorCode} from './errors'
export type {TransferErrorCode} from './errors'

export interface PublicError {
    readonly code: TransferErrorCode
    readonly message: string
}

export interface Warning {
    readonly code: 'beacon_warning'
    readonly message: string
}

export interface FileMetadata {
    readonly sessionId: string
    readonly name: string
    readonly size: number
    readonly isDir: boolean
    readonly url: string
    readonly qrBase64: string
    readonly warnings: readonly Warning[]
}

export interface ProgressSnapshot {
    readonly bytesSent: number
    readonly totalBytes: number
    readonly totalKnown: boolean
    readonly percent: number
    readonly speedBytesPerSec: number
}

export const lifecycleEventNames = [
    'transfer-started',
    'transfer-progress',
    'transfer-complete',
    'transfer-error',
    'transfer-reset',
] as const

export type LifecycleEventName = (typeof lifecycleEventNames)[number]

interface EventCursor {
    readonly sessionId: string
    readonly seq: number
}

export interface TransferStartedEvent extends EventCursor {
    readonly kind: 'transfer-started'
}

export interface TransferProgressEvent extends EventCursor {
    readonly kind: 'transfer-progress'
    readonly progress: ProgressSnapshot
}

export interface TransferCompleteEvent extends EventCursor {
    readonly kind: 'transfer-complete'
    readonly progress: ProgressSnapshot
}

export interface TransferErrorEvent extends EventCursor {
    readonly kind: 'transfer-error'
    /** `null` means the optional wire field was absent. */
    readonly progress: ProgressSnapshot | null
    readonly error: PublicError
}

export interface TransferResetEvent extends EventCursor {
    readonly kind: 'transfer-reset'
}

export type LifecycleEvent =
    | TransferStartedEvent
    | TransferProgressEvent
    | TransferCompleteEvent
    | TransferErrorEvent
    | TransferResetEvent

export type PendingItemKind = 'file' | 'directory' | 'unknown'

export interface RetainedDoneOutcome {
    readonly kind: 'done'
}

export interface RetainedErrorOutcome {
    readonly kind: 'error'
    readonly error: PublicError
}

/** A terminal result after every session/capability field has been scrubbed. */
export type RetainedOutcome = RetainedDoneOutcome | RetainedErrorOutcome
