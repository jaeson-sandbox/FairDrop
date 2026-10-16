import {fixedErrorMessages, parsePublicError, publicError, unknownPublicError} from './errors'
import {
    lifecycleEventNames,
    MAX_COLLECTION_ITEMS,
    type FileMetadata,
    type LifecycleEvent,
    type LifecycleEventName,
    type ProgressSnapshot,
    type PublicError,
    type ReceiveMetadata,
    type TextMetadata,
    type ConnectionOptions,
    type ConnectionRefresh,
    type ReceiveResult,
    type ReceiveStatus,
    type Warning,
} from './types'

type UnknownRecord = Record<string, unknown>

const ipv4 = (value: unknown): value is string => typeof value === 'string' &&
    /^(?:\d{1,3}\.){3}\d{1,3}$/.test(value) && value.split('.').every(part => Number(part) <= 255)

export function parseConnectionOptions(value: unknown): ConnectionOptions | null {
    const record = asRecord(value)
    if (record === null || !isSessionId(record.sessionId) || !ipv4(record.currentAddress) || !Array.isArray(record.choices)) return null
    const choices: ConnectionOptions['choices'][number][] = []
    for (const raw of record.choices) {
        const choice = asRecord(raw)
        if (choice === null || typeof choice.id !== 'string' || choice.id.length < 1 || choice.id.length > 128 ||
            /[\u0000-\u001f\u007f]/.test(choice.id) ||
            !isDestinationName(choice.name) || !ipv4(choice.address)) return null
        choices.push({id: choice.id, name: choice.name, address: choice.address})
    }
    return {sessionId: record.sessionId, currentAddress: record.currentAddress, choices}
}

export function parseConnectionRefresh(value: unknown): ConnectionRefresh | null {
    const record = asRecord(value)
    if (record === null) return null
    if (record.mode === 'send' && !('receive' in record) && !('text' in record)) {
        const send = parseFileMetadata(record.send)
        return send === null ? null : {mode: 'send', send}
    }
    if (record.mode === 'receive' && !('send' in record) && !('text' in record)) {
        const receive = parseReceiveMetadata(record.receive)
        return receive === null ? null : {mode: 'receive', receive}
    }
    if (record.mode === 'text' && !('send' in record) && !('receive' in record)) {
        const text = parseTextMetadata(record.text)
        return text === null ? null : {mode: 'text', text}
    }
    return null
}

export type TextValidation =
    | {readonly ok: true; readonly text: string; readonly byteLength: number}
    | {readonly ok: false; readonly reason: 'blank' | 'unsupported' | 'too-large'}

/** Mirror the Go boundary without replacing malformed UTF-16 during serialization. */
export function validateText(value: string): TextValidation {
    if (value.length > 131072) return {ok: false, reason: 'too-large'}
    let rawBytes = 0
    let malformed = false
    for (let index = 0; index < value.length; index++) {
        const unit = value.charCodeAt(index)
        if (unit >= 0xd800 && unit <= 0xdbff) {
            const low = value.charCodeAt(index + 1)
            if (low >= 0xdc00 && low <= 0xdfff) { index++; rawBytes += 4 }
            else { malformed = true; rawBytes += 3 }
        } else if (unit >= 0xdc00 && unit <= 0xdfff) { malformed = true; rawBytes += 3 }
        else rawBytes += unit < 0x80 ? 1 : unit < 0x800 ? 2 : 3
        if (rawBytes > 131072) return {ok: false, reason: 'too-large'}
    }
    const text = value.replace(/\r\n?/g, '\n')
    const byteLength = new TextEncoder().encode(text).length
    if (byteLength > 65536) return {ok: false, reason: 'too-large'}
    if (malformed || /[\u0000-\u0008\u000b-\u001f\u007f]/.test(text)) return {ok: false, reason: 'unsupported'}
    if (/^[\u0009-\u000d\u0020\u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]*$/u.test(text)) {
        return {ok: false, reason: 'blank'}
    }
    return {ok: true, text, byteLength}
}

export function parseTextMetadata(value: unknown): TextMetadata | null {
    try {
        const record = asRecord(value)
        if (record === null || !ownsExactly(record, ['sessionId', 'byteLength', 'url', 'qrBase64', 'warnings'])) return null
        if (!isSessionId(record.sessionId) || !isNonNegativeSafeInteger(record.byteLength) || record.byteLength < 1 || record.byteLength > 65536 ||
            !isTextURL(record.url) || !isCompletePNGBase64(record.qrBase64) || !Array.isArray(record.warnings)) return null
        const warnings: Warning[] = []
        for (const warning of record.warnings) {
            const parsed = parseWarning(warning)
            if (parsed === null || parsed.code !== 'beacon_warning') return null
            warnings.push(parsed)
        }
        return {sessionId: record.sessionId, byteLength: record.byteLength, url: record.url, qrBase64: record.qrBase64, warnings}
    } catch { return null }
}

const terminalErrorCodes = new Set([
    'path_not_found',
    'path_unsupported',
    'source_changed',
    'transfer_failed',
])

export function isLifecycleEventName(value: unknown): value is LifecycleEventName {
    return typeof value === 'string' && (lifecycleEventNames as readonly string[]).includes(value)
}

/** Parses Stage metadata into a fresh record containing only contract fields. */
export function parseFileMetadata(value: unknown): FileMetadata | null {
    try {
        const record = asRecord(value)
        if (record === null || !ownsEvery(record, metadataFields)) return null

        const {sessionId, name, size, isDir, url, qrBase64, warnings} = record
        if (!isSessionId(sessionId) || !isNonEmptyString(name)) return null
        if (!isNonNegativeSafeInteger(size) || typeof isDir !== 'boolean') return null
        const hasCollection = hasOwn(record, 'isCollection')
        const hasCount = hasOwn(record, 'itemCount')
        if (hasCollection !== hasCount) return null
        const isCollection = hasCollection ? record.isCollection : false
        const itemCount = hasCount ? record.itemCount : 1
        if (typeof isCollection !== 'boolean' || !Number.isSafeInteger(itemCount)) return null
        if (isCollection) {
            if (isDir || (itemCount as number) < 2 || (itemCount as number) > MAX_COLLECTION_ITEMS || name !== `${itemCount} items`) return null
        } else if (itemCount !== 1) return null
        if (!isCapabilityURL(url) || !isCompletePNGBase64(qrBase64) || !Array.isArray(warnings)) return null

        const parsedWarnings: Warning[] = []
        for (const warning of warnings) {
            const parsed = parseWarning(warning)
            if (parsed === null) return null
            parsedWarnings.push(parsed)
        }

        return {
            sessionId,
            name,
            size,
            isDir,
            ...(hasCollection ? {isCollection, itemCount: itemCount as number} : {}),
            url,
            qrBase64,
            warnings: parsedWarnings,
        }
    } catch {
        return null
    }
}

/**
 * Parses the `StartReceive` acknowledgement into a fresh record holding only
 * contract fields.
 *
 * Stricter than it looks in two places that matter for a feature that writes
 * to the user's disk. The URL must be the receive route (`/upload/` plus the
 * 128-bit token), so a send URL cannot be mistaken for a receive session, and
 * `destination` must be a plain display name: non-empty, bounded, and free of
 * control characters, so a hostile or corrupted value cannot reach the view as
 * anything but text. A volume root legitimately carries separators (it has no
 * basename), so separators are not refused.
 */
export function parseReceiveMetadata(value: unknown): ReceiveMetadata | null {
    try {
        const record = asRecord(value)
        if (record === null || !ownsExactly(record, receiveMetadataFields)) return null

        const {sessionId, destination, url, qrBase64, warnings} = record
        if (!isSessionId(sessionId) || !isDestinationName(destination)) return null
        if (!isUploadURL(url) || !isCompletePNGBase64(qrBase64) || !Array.isArray(warnings)) return null

        const parsedWarnings: Warning[] = []
        for (const warning of warnings) {
            const parsed = parseWarning(warning)
            if (parsed === null) return null
            parsedWarnings.push(parsed)
        }

        return {sessionId, destination, url, qrBase64, warnings: parsedWarnings}
    } catch {
        return null
    }
}

/**
 * Parses the `receive` payload of one event.
 *
 * `kind` is the event it arrived on, because the contract fixes what each may
 * say: a progress event carries only a saved count (no result, both flags
 * false); a complete event says `complete`; an error event says `incomplete` or
 * `cancelled`. Anything else is malformed, including an extra field -- the
 * payload is pinned to exactly these keys so nothing that names a path or a
 * file can ride along.
 */
export function parseReceiveStatus(value: unknown, kind: 'progress' | 'complete' | 'error'): ReceiveStatus | null {
    try {
        const record = asRecord(value)
        if (record === null) return null
        const allowed = ['filesSaved', 'result', 'subfolderExists', 'markingWarning']
        if (Object.keys(record).some((key) => !allowed.includes(key))) return null
        if (!hasOwn(record, 'filesSaved') || !hasOwn(record, 'subfolderExists') || !hasOwn(record, 'markingWarning')) {
            return null
        }

        const {filesSaved, subfolderExists, markingWarning} = record
        if (!isNonNegativeSafeInteger(filesSaved)) return null
        if (typeof subfolderExists !== 'boolean' || typeof markingWarning !== 'boolean') return null

        if (kind === 'progress') {
            if (hasOwn(record, 'result') || subfolderExists || markingWarning) return null
            return {filesSaved, result: null, subfolderExists: false, markingWarning: false}
        }

        const result = record.result
        if (!isReceiveResult(result)) return null
        if (kind === 'complete' ? result !== 'complete' : result === 'complete') return null
        return {filesSaved, result, subfolderExists, markingWarning}
    } catch {
        return null
    }
}

/** Parses the only non-terminal warning currently permitted by the contract. */
export function parseWarning(value: unknown): Warning | null {
    try {
        const record = asRecord(value)
        if (record === null || !ownsEvery(record, ['code', 'message'])) return null
        // The codes a warning may carry, which is deliberately not every error
        // code: a Warning is non-terminal, and a code that describes a failure
        // has no business arriving as one. Go's WarningCode type is the other
        // half of this, and main_test.go fails if the two lists disagree.
        if (record.code !== 'beacon_warning' && record.code !== 'name_warning') return null
        if (!isNonEmptyString(record.message)) return null

        // The registry's copy, never the sender's: an incoming message is
        // discarded exactly as a PublicError's is.
        return {code: record.code, message: fixedErrorMessages[record.code]}
    } catch {
        return null
    }
}

/**
 * Parses a finite, internally coherent wire-progress snapshot.
 *
 * This is the one layer that decides whether a snapshot is coherent. It
 * refuses, and nothing downstream repairs: a rejecting layer followed by a
 * repairing layer is two strategies for one rule, and the repairs could never
 * run because the refusal ran first.
 */
export function parseProgressSnapshot(value: unknown): ProgressSnapshot | null {
    try {
        const record = asRecord(value)
        if (record === null || !ownsEvery(record, progressFields)) return null

        const {bytesSent, totalBytes, totalKnown, percent, speedBytesPerSec} = record
        if (!isNonNegativeSafeInteger(bytesSent) || !isNonNegativeSafeInteger(totalBytes)) return null
        if (typeof totalKnown !== 'boolean') return null
        if (!isFiniteRange(percent, 0, 100) || !isFiniteRange(speedBytesPerSec, 0, Number.MAX_VALUE)) {
            return null
        }

        if (!totalKnown) {
            if (totalBytes !== 0 || percent !== 0) return null
        } else if (totalBytes === 0) {
            if (bytesSent !== 0 || percent !== 0) return null
        } else if (bytesSent > totalBytes) {
            return null
        }
        // `percent` is checked for range above and never re-derived here.
        // Requiring it to equal 100 * bytesSent / totalBytes within 8 ULP made
        // the frontend depend on an exact float expression evaluated in another
        // language: the day the Go side rounded a percentage for display, every
        // progress event would have been refused, silently. The two integers
        // are the authoritative pair, and selectProgressSnapshot derives what
        // is displayed from them (Epic 1 retrospective item 7).

        return {bytesSent, totalBytes, totalKnown, percent, speedBytesPerSec}
    } catch {
        return null
    }
}

/**
 * Parses one variadic Wails callback invocation. Exactly one payload is legal;
 * rejected input yields null and never exposes a reference supplied by Wails.
 */
export function parseLifecycleEvent(eventName: unknown, args: readonly unknown[]): LifecycleEvent | null {
    try {
        if (!isLifecycleEventName(eventName) || args.length !== 1) return null

        const record = asRecord(args[0])
        if (record === null || !ownsEvery(record, ['sessionId', 'seq'])) return null
        if (!isNonEmptyString(record.sessionId) || !isPositiveSafeInteger(record.seq)) return null
        // A notice field belongs to the notice event alone, and nothing else may
        // borrow it: an unexpected one is a malformed event, not an ignored extra.
        if (eventName !== 'transfer-notice' && hasOwn(record, 'notice')) return null

        const cursor = {sessionId: record.sessionId, seq: record.seq}
        switch (eventName) {
            case 'transfer-started':
                if (ownsEither(record, 'progress', 'error') || hasOwn(record, 'receive')) return null
                return {...cursor, kind: eventName}

            case 'transfer-progress': {
                if (!hasOwn(record, 'progress') || hasOwn(record, 'error')) return null
                const progress = parseProgressSnapshot(record.progress)
                if (progress === null) return null
                const receive = parseOptionalReceive(record, 'progress')
                return receive === undefined ? null : {...cursor, kind: eventName, progress, ...withReceive(receive)}
            }

            case 'transfer-complete': {
                if (!hasOwn(record, 'progress') || hasOwn(record, 'error')) return null
                const progress = parseProgressSnapshot(record.progress)
                if (progress === null) return null
                if (progress.totalKnown && progress.bytesSent !== progress.totalBytes) return null
                const receive = parseOptionalReceive(record, 'complete')
                return receive === undefined ? null : {...cursor, kind: eventName, progress, ...withReceive(receive)}
            }

            case 'transfer-error': {
                if (hasOwn(record, 'progress') && record.progress === undefined) return null
                const progress = hasOwn(record, 'progress') ? parseProgressSnapshot(record.progress) : null
                if (hasOwn(record, 'progress') && progress === null) return null
                const receive = parseOptionalReceive(record, 'error')
                if (receive === undefined) return null

                if (receive === null) return {...cursor, kind: eventName, progress, error: parseTerminalError(record)}

                // The two results that end in an error event must agree with the
                // code beside them. Only a cancellation may say `cancelled`, and an
                // incomplete upload must never arrive dressed as one: either
                // mismatch is a contradiction, and the event is refused rather
                // than guessed at.
                const cancelledCode = receiveCancelled(record)
                if ((receive.result === 'cancelled') !== cancelledCode) return null
                const error = cancelledCode ? publicError('cancelled') : parseTerminalError(record)
                return {...cursor, kind: eventName, progress, error, receive}
            }

            case 'transfer-reset':
                if (ownsEither(record, 'progress', 'error') || hasOwn(record, 'receive')) return null
                return {...cursor, kind: eventName}

            case 'transfer-notice':
                if (ownsEither(record, 'progress', 'error') || hasOwn(record, 'receive')) return null
                if (record.notice !== 'receive_too_large') return null
                return {...cursor, kind: eventName, notice: record.notice}
        }
    } catch {
        return null
    }
}

/** The `receive` field of a parsed event: present only for a receive session, so a send event keeps its exact shape. */
function withReceive(receive: ReceiveStatus | null): {readonly receive?: ReceiveStatus} {
    return receive === null ? {} : {receive}
}

/**
 * The event's `receive` payload: `null` when the field is absent (a send
 * event), the parsed status when it is present and well formed, and
 * `undefined` when it is present and malformed -- the caller refuses the whole
 * event in that case.
 */
function parseOptionalReceive(
    record: UnknownRecord,
    kind: 'progress' | 'complete' | 'error',
): ReceiveStatus | null | undefined {
    if (!hasOwn(record, 'receive')) return null
    return parseReceiveStatus(record.receive, kind) ?? undefined
}

/** Whether a terminal error event's own code is the cancellation code. */
function receiveCancelled(record: UnknownRecord): boolean {
    if (!hasOwn(record, 'error')) return false
    const parsed = parsePublicError(record.error)
    return parsed !== null && parsed.code === 'cancelled'
}

const metadataFields = ['sessionId', 'name', 'size', 'isDir', 'url', 'qrBase64', 'warnings'] as const
const receiveMetadataFields = ['sessionId', 'destination', 'url', 'qrBase64', 'warnings'] as const
const progressFields = ['bytesSent', 'totalBytes', 'totalKnown', 'percent', 'speedBytesPerSec'] as const

function parseTerminalError(record: UnknownRecord): PublicError {
    try {
        if (!hasOwn(record, 'error')) return unknownPublicError()
        const parsed = parsePublicError(record.error)
        if (parsed === null || !terminalErrorCodes.has(parsed.code)) return unknownPublicError()
        return publicError(parsed.code)
    } catch {
        return unknownPublicError()
    }
}

function asRecord(value: unknown): UnknownRecord | null {
    return typeof value === 'object' && value !== null && !Array.isArray(value)
        ? value as UnknownRecord
        : null
}

function hasOwn(record: UnknownRecord, key: string): boolean {
    return Object.prototype.hasOwnProperty.call(record, key)
}

function ownsEvery(record: UnknownRecord, keys: readonly string[]): boolean {
    return keys.every((key) => hasOwn(record, key))
}

function ownsExactly(record: UnknownRecord, keys: readonly string[]): boolean {
    return Object.keys(record).length === keys.length && ownsEvery(record, keys)
}

function ownsEither(record: UnknownRecord, first: string, second: string): boolean {
    return hasOwn(record, first) || hasOwn(record, second)
}

function isNonEmptyString(value: unknown): value is string {
    return typeof value === 'string' && value.length > 0
}

const sessionIdPattern = /^[0-9a-f]{32}$/
const capabilityURLPattern = /^http:\/\/((?:0|[1-9]\d{0,2})(?:\.(?:0|[1-9]\d{0,2})){3}):([1-9]\d{0,4})(\/download\/[0-9a-f]{32})$/
const uploadURLPattern = /^http:\/\/((?:0|[1-9]\d{0,2})(?:\.(?:0|[1-9]\d{0,2})){3}):([1-9]\d{0,4})(\/upload\/[0-9a-f]{32})$/
const textURLPattern = /^http:\/\/((?:0|[1-9]\d{0,2})(?:\.(?:0|[1-9]\d{0,2})){3}):([1-9]\d{0,4})(\/text\/[0-9a-f]{32})$/
const maximumDestinationLength = 1024
const maximumQRBase64Length = 2 * 1024 * 1024
const pngSignature = [137, 80, 78, 71, 13, 10, 26, 10] as const

function isSessionId(value: unknown): value is string {
    return typeof value === 'string' && sessionIdPattern.test(value)
}

function isCapabilityURL(value: unknown): value is string {
    if (typeof value !== 'string') return false
    const match = capabilityURLPattern.exec(value)
    if (match === null) return false
    const octets = match[1].split('.').map(Number)
    const port = Number(match[2])
    if (octets.some((octet) => octet > 255) || !Number.isInteger(port) || port < 1 || port > 65535) return false
    return true
}

function isUploadURL(value: unknown): value is string {
    if (typeof value !== 'string') return false
    const match = uploadURLPattern.exec(value)
    if (match === null) return false
    const octets = match[1].split('.').map(Number)
    const port = Number(match[2])
    return !octets.some((octet) => octet > 255) && Number.isInteger(port) && port >= 1 && port <= 65535
}

function isTextURL(value: unknown): value is string {
    if (typeof value !== 'string') return false
    const match = textURLPattern.exec(value)
    if (match === null) return false
    const octets = match[1].split('.').map(Number)
    const port = Number(match[2])
    return !octets.some(octet => octet > 255) && Number.isInteger(port) && port >= 1 && port <= 65535
}

/** A name to display: present, bounded, and free of control characters. */
function isDestinationName(value: unknown): value is string {
    return typeof value === 'string' && value.trim() !== '' && value.length <= maximumDestinationLength &&
        !/\p{Cc}/u.test(value)
}

function isReceiveResult(value: unknown): value is ReceiveResult {
    return value === 'complete' || value === 'incomplete' || value === 'cancelled'
}

function isCompletePNGBase64(value: unknown): value is string {
    if (typeof value !== 'string' || value.length === 0 || value.length > maximumQRBase64Length) return false
    if (value.length % 4 !== 0 || !/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(value)) {
        return false
    }

    let bytes: Uint8Array
    try {
        const decoded = atob(value)
        bytes = Uint8Array.from(decoded, (character) => character.charCodeAt(0))
    } catch {
        return false
    }

    if (bytes.length < 8 || pngSignature.some((byte, index) => bytes[index] !== byte)) return false

    let offset = 8
    let chunkIndex = 0
    let sawIDAT = false
    while (offset < bytes.length) {
        if (bytes.length - offset < 12) return false
        const length = ((bytes[offset] * 0x1000000) + (bytes[offset + 1] << 16) +
            (bytes[offset + 2] << 8) + bytes[offset + 3]) >>> 0
        const chunkEnd = offset + 12 + length
        if (chunkEnd > bytes.length) return false

        const type = String.fromCharCode(bytes[offset + 4], bytes[offset + 5], bytes[offset + 6], bytes[offset + 7])
        if (chunkIndex === 0 && (type !== 'IHDR' || length !== 13)) return false
        if (type === 'IDAT') sawIDAT = true
        if (type === 'IEND') return length === 0 && sawIDAT && chunkEnd === bytes.length

        offset = chunkEnd
        chunkIndex += 1
    }
    return false
}

function isNonNegativeSafeInteger(value: unknown): value is number {
    return Number.isSafeInteger(value) && (value as number) >= 0
}

function isPositiveSafeInteger(value: unknown): value is number {
    return Number.isSafeInteger(value) && (value as number) > 0
}

function isFiniteRange(value: unknown, minimum: number, maximum: number): value is number {
    return typeof value === 'number' && Number.isFinite(value) && value >= minimum && value <= maximum
}
