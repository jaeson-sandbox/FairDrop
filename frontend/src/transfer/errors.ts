/**
 * Command failures cross the Wails boundary as JSON, not as prose.
 *
 * A bound Go command that returns an error is rejected by the generated
 * binding with an `Error` whose `message` is whatever `options.ErrorFormatter`
 * produced -- for FairDrop, the `{code, message}` of the backend's public
 * error. Nothing else about that rejection is trustworthy: it is a string that
 * arrived from another process, so it is parsed and validated here rather than
 * read field by field at the call site.
 */

import type {PublicError} from './types'

/** The literal cross-language registry pinned by main_test.go. */
export const transferErrorCodes = [
    'invalid_selection',
    'busy',
    'cancelled',
    'path_not_found',
    'path_unsupported',
    'source_changed',
    'network_unavailable',
    'server_start_failed',
    'qr_failed',
    'beacon_warning',
    'transfer_failed',
    'shutting_down',
] as const

export type TransferErrorCode = (typeof transferErrorCodes)[number]
export type {PublicError}

/** The only public copy allowed to enter frontend state for each stable code. */
export const fixedErrorMessages: Readonly<Record<TransferErrorCode, string>> = {
    invalid_selection: 'Choose exactly one file or folder.',
    busy: 'Finish or cancel the current transfer before choosing another item.',
    cancelled: 'Transfer canceled.',
    path_not_found: 'That file or folder is no longer available. Choose it again.',
    path_unsupported: 'FairDrop can use regular files and folders only. Choose another item.',
    source_changed: 'The item changed after it was prepared. Cancel and create a fresh link.',
    network_unavailable: 'FairDrop couldn’t find a usable local network. Connect to local Wi-Fi, then try again.',
    server_start_failed: 'FairDrop couldn’t open a local transfer connection. Check firewall access, then try again.',
    qr_failed: 'FairDrop couldn’t create the QR code. Prepare the item again.',
    beacon_warning: 'Device discovery isn’t available. The QR code and download link still work.',
    transfer_failed:
        'The transfer stopped before FairDrop finished sending. Check the local network and create a fresh link.',
    shutting_down: 'FairDrop is closing. Reopen it to start a transfer.',
}

/** Reports whether a value is one of the stable backend failure codes. */
export function isTransferErrorCode(value: unknown): value is TransferErrorCode {
    return typeof value === 'string' && (transferErrorCodes as readonly string[]).includes(value)
}

/**
 * Converts a rejected command into a validated `{code, message}`.
 *
 * Anything that is not a well-formed public error -- a timeout the runtime
 * raised itself, a truncated payload, a code this build does not know --
 * becomes `transfer_failed` with the fixed copy, which is the same fallback
 * the backend applies to an unrecognized error. The two ends therefore agree
 * on what an unknown failure is called, from either direction.
 */
export function parseCommandError(rejection: unknown): PublicError {
    let serialized: string | null
    try {
        serialized = serializedPayloadOf(rejection)
    } catch {
        return unknownCommandError()
    }
    if (serialized === null) return unknownCommandError()

    let decoded: unknown
    try {
        decoded = JSON.parse(serialized)
    } catch {
        return unknownCommandError()
    }

    return parsePublicError(decoded) ?? unknownPublicError()
}

/**
 * Validates a public-error-shaped value and replaces its message with fixed
 * registry copy. The incoming message is shape evidence only; it is never UI
 * text, even when the code is recognized.
 */
export function parsePublicError(value: unknown): PublicError | null {
    try {
        if (typeof value !== 'object' || value === null || Array.isArray(value)) return null

        const {code, message} = value as {code?: unknown; message?: unknown}
        if (!isTransferErrorCode(code)) return null
        if (typeof message !== 'string' || message.trim() === '') return null

        return publicError(code)
    } catch {
        return null
    }
}

/** Returns a fresh fixed-copy error for a known code. */
export function publicError(code: TransferErrorCode): PublicError {
    return {code, message: fixedErrorMessages[code]}
}

/** Returns a fresh safe fallback for malformed or unknown input. */
export function unknownPublicError(): PublicError {
    return publicError('transfer_failed')
}

/** The rejection's payload, or null when it cannot carry one. */
function serializedPayloadOf(rejection: unknown): string | null {
    if (rejection instanceof Error) return rejection.message
    if (typeof rejection === 'string') return rejection
    return null
}

/** A fresh fallback, so a caller that mutates its result cannot poison the next one. */
function unknownCommandError(): PublicError {
    return unknownPublicError()
}
