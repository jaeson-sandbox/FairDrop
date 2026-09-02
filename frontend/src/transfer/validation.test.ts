import {describe, expect, it} from 'vitest'
import {parseFileMetadata, parseLifecycleEvent, parseProgressSnapshot, parseWarning} from './validation'

const sessionId = '0123456789abcdef0123456789abcdef'
const capabilityURL = 'http://192.0.2.1:34123/download/fedcba9876543210fedcba9876543210'
const qrPNG = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII='

function mutatePNGByte(index: number, value: number): string {
    const bytes = Uint8Array.from(atob(qrPNG), (character) => character.charCodeAt(0))
    bytes[index] = value
    return btoa(String.fromCharCode(...bytes))
}

function appendPNGByte(value: number): string {
    return btoa(`${atob(qrPNG)}${String.fromCharCode(value)}`)
}

function metadata(overrides: Record<string, unknown> = {}): Record<string, unknown> {
    return {
        sessionId,
        name: 'report.pdf',
        size: 4,
        isDir: false,
        url: capabilityURL,
        qrBase64: qrPNG,
        warnings: [],
        ...overrides,
    }
}

function progress(overrides: Record<string, unknown> = {}): Record<string, unknown> {
    return {
        bytesSent: 25,
        totalBytes: 100,
        totalKnown: true,
        percent: 25,
        speedBytesPerSec: 50,
        ...overrides,
    }
}

describe('Stage metadata validation', () => {
    it('copies only allow-listed fields into fresh metadata and warning records', () => {
        const rawWarning = {code: 'beacon_warning', message: String.raw`C:\secret\x`, token: 'secret'}
        const raw = metadata({warnings: [rawWarning], path: String.raw`C:\secret\x`, token: 'secret'})

        const parsed = parseFileMetadata(raw)

        expect(parsed).toEqual({
            sessionId,
            name: 'report.pdf',
            size: 4,
            isDir: false,
            url: capabilityURL,
            qrBase64: qrPNG,
            warnings: [{
                code: 'beacon_warning',
                message: 'Device discovery isn’t available. The QR code and download link still work.',
            }],
        })
        expect(parsed).not.toBe(raw)
        expect(parsed?.warnings).not.toBe(raw.warnings)
        expect(parsed?.warnings[0]).not.toBe(rawWarning)
        expect(JSON.stringify(parsed)).not.toContain('secret')
    })

    it.each([
        ['null', null],
        ['an array', []],
        ['an empty session', metadata({sessionId: ''})],
        ['an uppercase session', metadata({sessionId: '0123456789ABCDEF0123456789ABCDEF'})],
        ['a short session', metadata({sessionId: '0123456789abcdef'})],
        ['a 31-character session', metadata({sessionId: '0123456789abcdef0123456789abcde'})],
        ['a 33-character session', metadata({sessionId: '0123456789abcdef0123456789abcdef0'})],
        ['an unsafe size', metadata({size: Number.MAX_SAFE_INTEGER + 1})],
        ['negative size', metadata({size: -1})],
        ['null warnings', metadata({warnings: null})],
        ['non-PNG QR data', metadata({qrBase64: 'c2VjcmV0'})],
        ['a signature-only PNG', metadata({qrBase64: 'iVBORw0KGgo='})],
        ['a truncated PNG chunk', metadata({qrBase64: qrPNG.slice(0, -8)})],
        ['a PNG whose first chunk is not IHDR', metadata({qrBase64: mutatePNGByte(12, 'J'.charCodeAt(0))})],
        ['a PNG without an IDAT chunk', metadata({qrBase64: mutatePNGByte(37, 'X'.charCodeAt(0))})],
        ['a PNG carrying bytes after IEND', metadata({qrBase64: appendPNGByte(0)})],
        ['an unknown warning', metadata({warnings: [{code: 'busy', message: 'x'}]})],
        ['a missing field', (() => { const value = metadata(); delete value.url; return value })()],
    ])('rejects %s without throwing', (_name, raw) => {
        expect(() => parseFileMetadata(raw)).not.toThrow()
        expect(parseFileMetadata(raw)).toBeNull()
    })

    it('rejects hostile getters without throwing', () => {
        const raw = metadata()
        Object.defineProperty(raw, 'sessionId', {get: () => { throw new Error('boom') }})

        expect(() => parseFileMetadata(raw)).not.toThrow()
        expect(parseFileMetadata(raw)).toBeNull()
    })

    it('validates warning shape independently', () => {
        expect(parseWarning({code: 'beacon_warning', message: 'forged'})).toEqual({
            code: 'beacon_warning',
            message: 'Device discovery isn’t available. The QR code and download link still work.',
        })
        expect(parseWarning({code: 'beacon_warning'})).toBeNull()
    })

    it.each([
        ['https', capabilityURL.replace('http:', 'https:')],
        ['credentials', 'http://user:pass@192.0.2.1:34123/download/fedcba9876543210fedcba9876543210'],
        ['DNS host', 'http://fairdrop.local:34123/download/fedcba9876543210fedcba9876543210'],
        ['out-of-range IPv4 octet', 'http://256.0.2.1:34123/download/fedcba9876543210fedcba9876543210'],
        ['zero-padded IPv4 octet', 'http://192.000.2.1:34123/download/fedcba9876543210fedcba9876543210'],
        ['missing port', 'http://192.0.2.1/download/fedcba9876543210fedcba9876543210'],
        ['out-of-range port', 'http://192.0.2.1:65536/download/fedcba9876543210fedcba9876543210'],
        ['zero-padded port', 'http://192.0.2.1:034123/download/fedcba9876543210fedcba9876543210'],
        ['query', `${capabilityURL}?source=C%3A%5Csecret.pdf`],
        ['fragment', `${capabilityURL}#secret`],
        ['uppercase token', 'http://192.0.2.1:34123/download/FEDCBA9876543210FEDCBA9876543210'],
        ['short token', 'http://192.0.2.1:34123/download/fedcba9876543210fedcba987654321'],
        ['long token', 'http://192.0.2.1:34123/download/fedcba9876543210fedcba98765432100'],
        ['encoded path', 'http://192.0.2.1:34123/download/%66edcba9876543210fedcba9876543210'],
        ['source path', String.raw`C:\secret\report.pdf`],
    ])('rejects a noncanonical capability URL with %s', (_name, url) => {
        expect(parseFileMetadata(metadata({url}))).toBeNull()
    })

    it('accepts an explicitly written valid default-numbered port', () => {
        expect(parseFileMetadata(metadata({
            url: 'http://192.0.2.1:80/download/fedcba9876543210fedcba9876543210',
        }))).not.toBeNull()
    })
})

describe('progress validation', () => {
    it.each([
        ['known positive', progress()],
        ['known empty', progress({bytesSent: 0, totalBytes: 0, percent: 0, speedBytesPerSec: 0})],
        ['unknown', progress({totalKnown: false, totalBytes: 0, percent: 0})],
    ])('accepts coherent %s progress', (_name, raw) => {
        expect(parseProgressSnapshot(raw)).toEqual(raw)
        expect(parseProgressSnapshot(raw)).not.toBe(raw)
    })

    it.each([
        ['NaN percent', progress({percent: Number.NaN})],
        ['infinite speed', progress({speedBytesPerSec: Number.POSITIVE_INFINITY})],
        ['negative bytes', progress({bytesSent: -1, percent: -1})],
        ['unsafe bytes', progress({bytesSent: Number.MAX_SAFE_INTEGER + 1})],
        ['bytes beyond total', progress({bytesSent: 101, percent: 100})],
        ['incoherent percentage', progress({percent: 75})],
        ['invented unknown total', progress({totalKnown: false, totalBytes: 100, percent: 0})],
        ['percentage for unknown total', progress({totalKnown: false, totalBytes: 0, percent: 1})],
        ['bytes for known empty', progress({bytesSent: 1, totalBytes: 0, percent: 0})],
    ])('rejects %s', (_name, raw) => {
        expect(parseProgressSnapshot(raw)).toBeNull()
    })
})

describe('lifecycle event validation', () => {
    it('parses all five payloads and makes optional progress explicit', () => {
        expect(parseLifecycleEvent('transfer-started', [{sessionId: 'session-1', seq: 1}])).toEqual({
            kind: 'transfer-started', sessionId: 'session-1', seq: 1,
        })
        expect(parseLifecycleEvent('transfer-progress', [{sessionId: 'session-1', seq: 2, progress: progress()}])).toEqual({
            kind: 'transfer-progress', sessionId: 'session-1', seq: 2, progress: progress(),
        })
        expect(parseLifecycleEvent('transfer-complete', [{sessionId: 'session-1', seq: 3, progress: progress({bytesSent: 100, percent: 100})}])).toEqual({
            kind: 'transfer-complete', sessionId: 'session-1', seq: 3, progress: progress({bytesSent: 100, percent: 100}),
        })
        expect(parseLifecycleEvent('transfer-error', [{
            sessionId: 'session-1', seq: 3, error: {code: 'source_changed', message: 'forged'},
        }])).toEqual({
            kind: 'transfer-error', sessionId: 'session-1', seq: 3, progress: null,
            error: {
                code: 'source_changed',
                message: 'The item changed after it was prepared. Cancel and create a fresh link.',
            },
        })
        expect(parseLifecycleEvent('transfer-reset', [{sessionId: 'session-1', seq: 4}])).toEqual({
            kind: 'transfer-reset', sessionId: 'session-1', seq: 4,
        })
    })

    it.each([
        ['no payload', 'transfer-started', []],
        ['variadic payloads', 'transfer-started', [{sessionId: 'session-1', seq: 1}, 'extra']],
        ['unknown name', 'transfer-paused', [{sessionId: 'session-1', seq: 1}]],
        ['zero sequence', 'transfer-started', [{sessionId: 'session-1', seq: 0}]],
        ['fractional sequence', 'transfer-started', [{sessionId: 'session-1', seq: 1.5}]],
        ['unsafe sequence', 'transfer-started', [{sessionId: 'session-1', seq: Number.MAX_SAFE_INTEGER + 1}]],
        ['empty session', 'transfer-started', [{sessionId: '', seq: 1}]],
        ['started with progress', 'transfer-started', [{sessionId: 'session-1', seq: 1, progress: progress()}]],
        ['progress without progress', 'transfer-progress', [{sessionId: 'session-1', seq: 2}]],
        ['progress with error', 'transfer-progress', [{sessionId: 'session-1', seq: 2, progress: progress(), error: {}}]],
        ['complete with malformed progress', 'transfer-complete', [{sessionId: 'session-1', seq: 3, progress: progress({percent: 70})}]],
        ['error with malformed progress', 'transfer-error', [{sessionId: 'session-1', seq: 3, progress: null, error: {code: 'transfer_failed', message: 'x'}}]],
        ['reset with error', 'transfer-reset', [{sessionId: 'session-1', seq: 4, error: {}}]],
    ] as const)('rejects %s without throwing', (_name, eventName, args) => {
        expect(() => parseLifecycleEvent(eventName, args)).not.toThrow()
        expect(parseLifecycleEvent(eventName, args)).toBeNull()
    })

    it('maps missing, malformed, unknown, and disallowed terminal errors to fixed transfer_failed', () => {
        const bodies = [
            {},
            {error: null},
            {error: {code: 'quota_exceeded', message: 'secret'}},
            {error: {code: 'busy', message: 'secret'}},
            {error: {code: 'path_not_found', message: ''}},
        ]

        for (const body of bodies) {
            const parsed = parseLifecycleEvent('transfer-error', [{sessionId: 'session-1', seq: 3, ...body}])
            expect(parsed).toMatchObject({
                kind: 'transfer-error',
                error: {
                    code: 'transfer_failed',
                    message: 'The transfer stopped before FairDrop finished sending. Check the local network and create a fresh link.',
                },
            })
        }
    })

    it('accepts transfer-error with a throwing error getter as fixed transfer_failed', () => {
        const payload = {sessionId: 'session-1', seq: 3}
        Object.defineProperty(payload, 'error', {get: () => { throw new Error('boom') }})

        expect(parseLifecycleEvent('transfer-error', [payload])).toEqual({
            kind: 'transfer-error',
            sessionId: 'session-1',
            seq: 3,
            progress: null,
            error: {
                code: 'transfer_failed',
                message: 'The transfer stopped before FairDrop finished sending. Check the local network and create a fresh link.',
            },
        })
    })

    it('rejects incomplete known-total completion but accepts known-empty and unknown completion', () => {
        expect(parseLifecycleEvent('transfer-complete', [{
            sessionId: 'session-1', seq: 3, progress: progress(),
        }])).toBeNull()
        expect(parseLifecycleEvent('transfer-complete', [{
            sessionId: 'session-1', seq: 3,
            progress: progress({bytesSent: 0, totalBytes: 0, percent: 0}),
        }])).not.toBeNull()
        expect(parseLifecycleEvent('transfer-complete', [{
            sessionId: 'session-1', seq: 3,
            progress: progress({totalKnown: false, totalBytes: 0, percent: 0}),
        }])).not.toBeNull()
    })

    it('does not throw when an event getter throws', () => {
        const hostile = Object.defineProperty({sessionId: 'session-1', seq: 1}, 'progress', {
            get: () => { throw new Error('boom') },
        })

        expect(() => parseLifecycleEvent('transfer-progress', [hostile])).not.toThrow()
        expect(parseLifecycleEvent('transfer-progress', [hostile])).toBeNull()
    })
})
