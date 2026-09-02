import {fixedErrorMessages, parsePublicError, publicError, unknownPublicError} from './errors'
import {
    lifecycleEventNames,
    type FileMetadata,
    type LifecycleEvent,
    type LifecycleEventName,
    type ProgressSnapshot,
    type PublicError,
    type Warning,
} from './types'

type UnknownRecord = Record<string, unknown>

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
            url,
            qrBase64,
            warnings: parsedWarnings,
        }
    } catch {
        return null
    }
}

/** Parses the only non-terminal warning currently permitted by the contract. */
export function parseWarning(value: unknown): Warning | null {
    try {
        const record = asRecord(value)
        if (record === null || !ownsEvery(record, ['code', 'message'])) return null
        if (record.code !== 'beacon_warning') return null
        if (!isNonEmptyString(record.message)) return null

        return {code: 'beacon_warning', message: fixedErrorMessages.beacon_warning}
    } catch {
        return null
    }
}

/** Parses a finite, internally coherent wire-progress snapshot. */
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
        } else {
            if (bytesSent > totalBytes) return null
            const expected = 100 * bytesSent / totalBytes
            if (!nearlyEqual(percent, expected)) return null
        }

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

        const cursor = {sessionId: record.sessionId, seq: record.seq}
        switch (eventName) {
            case 'transfer-started':
                if (ownsEither(record, 'progress', 'error')) return null
                return {...cursor, kind: eventName}

            case 'transfer-progress': {
                if (!hasOwn(record, 'progress') || hasOwn(record, 'error')) return null
                const progress = parseProgressSnapshot(record.progress)
                return progress === null ? null : {...cursor, kind: eventName, progress}
            }

            case 'transfer-complete': {
                if (!hasOwn(record, 'progress') || hasOwn(record, 'error')) return null
                const progress = parseProgressSnapshot(record.progress)
                if (progress === null) return null
                if (progress.totalKnown && progress.bytesSent !== progress.totalBytes) return null
                return {...cursor, kind: eventName, progress}
            }

            case 'transfer-error': {
                if (hasOwn(record, 'progress') && record.progress === undefined) return null
                const progress = hasOwn(record, 'progress') ? parseProgressSnapshot(record.progress) : null
                if (hasOwn(record, 'progress') && progress === null) return null

                return {
                    ...cursor,
                    kind: eventName,
                    progress,
                    error: parseTerminalError(record),
                }
            }

            case 'transfer-reset':
                if (ownsEither(record, 'progress', 'error')) return null
                return {...cursor, kind: eventName}
        }
    } catch {
        return null
    }
}

const metadataFields = ['sessionId', 'name', 'size', 'isDir', 'url', 'qrBase64', 'warnings'] as const
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

function ownsEither(record: UnknownRecord, first: string, second: string): boolean {
    return hasOwn(record, first) || hasOwn(record, second)
}

function isNonEmptyString(value: unknown): value is string {
    return typeof value === 'string' && value.length > 0
}

const sessionIdPattern = /^[0-9a-f]{32}$/
const capabilityURLPattern = /^http:\/\/((?:0|[1-9]\d{0,2})(?:\.(?:0|[1-9]\d{0,2})){3}):([1-9]\d{0,4})(\/download\/[0-9a-f]{32})$/
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

function nearlyEqual(actual: number, expected: number): boolean {
    const tolerance = Number.EPSILON * Math.max(1, Math.abs(expected)) * 8
    return Math.abs(actual - expected) <= tolerance
}
