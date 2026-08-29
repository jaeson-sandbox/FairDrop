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

/**
 * Every stable failure code the backend can send. The list is spelled out
 * rather than derived from the generated bindings: it is one half of a
 * cross-language contract, and a code that quietly disappeared from a
 * generated file would otherwise become an unrecognized error at runtime with
 * nothing failing at build time.
 */
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

/** The validated failure a rejected command carries. */
export interface CommandError {
    code: TransferErrorCode
    message: string
}

/**
 * The copy the backend sends with `transfer_failed`, repeated here because a
 * malformed rejection carries no message to show. It is the one string this
 * module owns.
 */
const transferFailedMessage =
    'The transfer stopped before FairDrop finished sending. Check the local network and create a fresh link.'

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
export function parseCommandError(rejection: unknown): CommandError {
    const serialized = serializedPayloadOf(rejection)
    if (serialized === null) return unknownCommandError()

    let decoded: unknown
    try {
        decoded = JSON.parse(serialized)
    } catch {
        return unknownCommandError()
    }

    // `null` has type 'object', so it needs its own refusal. Arrays do not:
    // a JSON array has no `code` property, so it fails the same validation a
    // JSON object with a bad code fails, and a guard here would be a branch no
    // input can reach.
    if (typeof decoded !== 'object' || decoded === null) return unknownCommandError()

    const {code, message} = decoded as {code?: unknown; message?: unknown}
    if (!isTransferErrorCode(code)) return unknownCommandError()
    // Trimmed, not just empty-checked: a message of spaces renders as a code
    // beside a blank line, which reads as a UI bug rather than a failure.
    if (typeof message !== 'string' || message.trim() === '') return unknownCommandError()

    return {code, message}
}

/** The rejection's payload, or null when it cannot carry one. */
function serializedPayloadOf(rejection: unknown): string | null {
    if (rejection instanceof Error) return rejection.message
    if (typeof rejection === 'string') return rejection
    return null
}

/** A fresh fallback, so a caller that mutates its result cannot poison the next one. */
function unknownCommandError(): CommandError {
    return {code: 'transfer_failed', message: transferFailedMessage}
}
