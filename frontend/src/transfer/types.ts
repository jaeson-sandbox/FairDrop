/** Runtime-facing transfer types. Every field name mirrors the Wails JSON contract. */

import type {TransferErrorCode} from './errors'
export type {TransferErrorCode} from './errors'

export interface PublicError {
    readonly code: TransferErrorCode
    readonly message: string
}

export interface Warning {
    readonly code: 'beacon_warning' | 'name_warning'
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

/**
 * What the completion receipt needs, and nothing else `FileMetadata` carries.
 *
 * Deliberately not `FileMetadata`: that type also carries `url` (the one-shot
 * capability download link) and `qrBase64` (a scannable PNG of that same
 * link). This receipt lives on in `RetainedDoneOutcome`, in Idle, after the
 * session has been reset and the server has stopped -- FairDrop's contract is
 * that it persists nothing, and a sender-side surface that could re-present a
 * dead capability link (or hold its QR code indefinitely) is exactly what
 * that contract forbids. `bytesSent` comes from the terminal
 * `ProgressSnapshot`, never from `FileMetadata.size` (Story 7.4 AC3).
 */
export interface CompletionReceipt {
    readonly name: string
    readonly isDir: boolean
    readonly bytesSent: number
}

export interface RetainedDoneOutcome {
    readonly kind: 'done'
    /** What was sent and how much, scrubbed of the capability link and its QR. */
    readonly receipt: CompletionReceipt
}

export interface RetainedErrorOutcome {
    readonly kind: 'error'
    readonly error: PublicError
    /**
     * The failed item's display name, retained the same way Story 7.4 retained
     * the Done receipt -- the name only, **never** `url` or `qrBase64` (Story
     * 9.2). Present only for a terminal transfer error reached from a live
     * session (Staged or Transferring failed); a Stage-time command failure
     * never reached a session and carries none.
     */
    readonly itemName?: string
}

/** A terminal result after every session/capability field has been scrubbed. */
export type RetainedOutcome = RetainedDoneOutcome | RetainedErrorOutcome

/**
 * The next action a sender can take from an error, keyed to the owner-approved
 * table in `selectErrorAction` (Story 9.2). `null` (no row) means the error
 * never reaches an outcome card at all.
 */
export type ErrorAction = 'retry' | 'choose' | 'dismiss'
