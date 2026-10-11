import {describe, expect, it} from 'vitest'
import {parseLifecycleEvent, parseReceiveMetadata, parseReceiveStatus} from './validation'

/*
  Receive wire validation (spec-phone-to-desktop-receiving, Story B).

  Every accepted and rejected shape below is written out as a literal, in the
  field names and values the Go side pins in app_receive_test.go
  (TestTheReceiveWireShapesAreExactlyWhatTheFrontendReads), never built from the
  parser's own constants. A receive feature writes to the user's disk, so the
  rule here is the strict one: a malformed payload is refused, not repaired.
*/

const sessionId = '0102030405060708090a0b0c0d0e0f10'
const qrPNG = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII='

function metadata(overrides: Record<string, unknown> = {}): Record<string, unknown> {
    return {
        sessionId,
        destination: 'Phone Photos',
        url: 'http://192.168.1.50:45678/upload/1112131415161718191a1b1c1d1e1f20',
        qrBase64: qrPNG,
        warnings: [],
        ...overrides,
    }
}

function progress(bytesSent: number, totalBytes = 400): Record<string, unknown> {
    return {bytesSent, totalBytes, totalKnown: true, percent: (100 * bytesSent) / totalBytes, speedBytesPerSec: 0}
}

describe('parseReceiveMetadata', () => {
    it('parses exactly the pinned StartReceive acknowledgement into a fresh record', () => {
        const parsed = parseReceiveMetadata(metadata())

        expect(parsed).toEqual({
            sessionId: '0102030405060708090a0b0c0d0e0f10',
            destination: 'Phone Photos',
            url: 'http://192.168.1.50:45678/upload/1112131415161718191a1b1c1d1e1f20',
            qrBase64: qrPNG,
            warnings: [],
        })
        expect(parsed).not.toBe(metadata())
    })

    it('replaces a warning message with the registry copy and refuses a code a warning may not carry', () => {
        const parsed = parseReceiveMetadata(metadata({warnings: [{code: 'beacon_warning', message: 'forged'}]}))
        expect(parsed?.warnings).toEqual([
            {code: 'beacon_warning', message: 'Device discovery isn’t available. The QR code and download link still work.'},
        ])
        expect(parseReceiveMetadata(metadata({warnings: [{code: 'busy', message: 'x'}]}))).toBeNull()
    })

    it.each([
        ['null', null],
        ['an array', []],
        ['a primitive', 'metadata'],
        ['a missing destination', (() => { const {destination: _omitted, ...rest} = metadata(); return rest })()],
        ['an extra field', metadata({path: '/Users/someone/Phone Photos'})],
        ['a non-hex session id', metadata({sessionId: 'not-a-session'})],
        ['a short session id', metadata({sessionId: sessionId.slice(1)})],
        ['an empty destination', metadata({destination: ''})],
        ['a whitespace-only destination', metadata({destination: '   '})],
        ['a destination with a control character', metadata({destination: 'Photos\u0007'})],
        ['a destination with a newline', metadata({destination: 'Photos\nand more'})],
        ['an over-long destination', metadata({destination: 'x'.repeat(1025)})],
        ['a non-string destination', metadata({destination: 7})],
        ['a send (download) url', metadata({url: 'http://192.168.1.50:45678/download/1112131415161718191a1b1c1d1e1f20'})],
        ['an https url', metadata({url: 'https://192.168.1.50:45678/upload/1112131415161718191a1b1c1d1e1f20'})],
        ['a hostname url', metadata({url: 'http://example.com:45678/upload/1112131415161718191a1b1c1d1e1f20'})],
        ['a short token', metadata({url: 'http://192.168.1.50:45678/upload/1112131415161718'})],
        ['an out-of-range octet', metadata({url: 'http://192.168.1.300:45678/upload/1112131415161718191a1b1c1d1e1f20'})],
        ['port zero', metadata({url: 'http://192.168.1.50:0/upload/1112131415161718191a1b1c1d1e1f20'})],
        ['a trailing path', metadata({url: 'http://192.168.1.50:45678/upload/1112131415161718191a1b1c1d1e1f20/extra'})],
        ['a query string', metadata({url: 'http://192.168.1.50:45678/upload/1112131415161718191a1b1c1d1e1f20?x=1'})],
        ['an empty qr', metadata({qrBase64: ''})],
        ['a qr that is not a PNG', metadata({qrBase64: 'aGVsbG8gd29ybGQh'})],
        ['non-array warnings', metadata({warnings: 'none'})],
        ['a malformed warning', metadata({warnings: [{code: 'beacon_warning'}]})],
    ])('refuses %s', (_name, value) => {
        expect(parseReceiveMetadata(value)).toBeNull()
    })

    it('accepts a volume-root destination that carries separators, because a root has no basename', () => {
        expect(parseReceiveMetadata(metadata({destination: 'D:\\'}))?.destination).toBe('D:\\')
        expect(parseReceiveMetadata(metadata({destination: '/'}))?.destination).toBe('/')
    })

    it('never throws on a hostile object', () => {
        const hostile = {}
        Object.defineProperty(hostile, 'sessionId', {get: () => { throw new Error('boom') }, enumerable: true})
        expect(() => parseReceiveMetadata(hostile)).not.toThrow()
        expect(parseReceiveMetadata(hostile)).toBeNull()
    })
})

describe('parseReceiveStatus', () => {
    it('reads a progress payload as a count only', () => {
        expect(parseReceiveStatus({filesSaved: 2, subfolderExists: false, markingWarning: false}, 'progress')).toEqual({
            filesSaved: 2, result: null, subfolderExists: false, markingWarning: false,
        })
    })

    it('reads a complete payload', () => {
        expect(parseReceiveStatus(
            {filesSaved: 3, result: 'complete', subfolderExists: true, markingWarning: false}, 'complete',
        )).toEqual({filesSaved: 3, result: 'complete', subfolderExists: true, markingWarning: false})
    })

    it.each(['incomplete', 'cancelled'] as const)('reads an error payload whose result is %s', (result) => {
        expect(parseReceiveStatus(
            {filesSaved: 1, result, subfolderExists: true, markingWarning: true}, 'error',
        )).toEqual({filesSaved: 1, result, subfolderExists: true, markingWarning: true})
    })

    it.each([
        ['progress carrying a result', {filesSaved: 1, result: 'complete', subfolderExists: false, markingWarning: false}, 'progress'],
        ['progress with subfolderExists true', {filesSaved: 1, subfolderExists: true, markingWarning: false}, 'progress'],
        ['progress with markingWarning true', {filesSaved: 1, subfolderExists: false, markingWarning: true}, 'progress'],
        ['complete without a result', {filesSaved: 1, subfolderExists: true, markingWarning: false}, 'complete'],
        ['complete saying incomplete', {filesSaved: 1, result: 'incomplete', subfolderExists: true, markingWarning: false}, 'complete'],
        ['complete saying cancelled', {filesSaved: 1, result: 'cancelled', subfolderExists: true, markingWarning: false}, 'complete'],
        ['error saying complete', {filesSaved: 1, result: 'complete', subfolderExists: true, markingWarning: false}, 'error'],
        ['error without a result', {filesSaved: 1, subfolderExists: true, markingWarning: false}, 'error'],
        ['an unknown result', {filesSaved: 1, result: 'partial', subfolderExists: true, markingWarning: false}, 'error'],
        ['a negative count', {filesSaved: -1, subfolderExists: false, markingWarning: false}, 'progress'],
        ['a fractional count', {filesSaved: 1.5, subfolderExists: false, markingWarning: false}, 'progress'],
        ['an unsafe count', {filesSaved: Number.MAX_SAFE_INTEGER + 1, subfolderExists: false, markingWarning: false}, 'progress'],
        ['a string count', {filesSaved: '2', subfolderExists: false, markingWarning: false}, 'progress'],
        ['a missing count', {subfolderExists: false, markingWarning: false}, 'progress'],
        ['a non-boolean flag', {filesSaved: 1, subfolderExists: 'yes', markingWarning: false}, 'progress'],
        ['an extra field naming a path', {filesSaved: 1, subfolderExists: false, markingWarning: false, folder: '/Users/x'}, 'progress'],
        ['an extra field naming a file', {filesSaved: 1, subfolderExists: false, markingWarning: false, names: ['a.jpg']}, 'progress'],
        ['null', null, 'progress'],
        ['an array', [], 'progress'],
    ] as const)('refuses %s', (_name, value, kind) => {
        expect(parseReceiveStatus(value, kind)).toBeNull()
    })
})

describe('receive lifecycle events', () => {
    it('keeps a send event exactly as it was: no receive field appears', () => {
        const parsed = parseLifecycleEvent('transfer-progress', [{sessionId, seq: 2, progress: progress(100)}])
        expect(parsed).toEqual({kind: 'transfer-progress', sessionId, seq: 2, progress: progress(100)})
        expect(parsed).not.toHaveProperty('receive')
    })

    it('parses the pinned receive progress event', () => {
        expect(parseLifecycleEvent('transfer-progress', [{
            sessionId, seq: 3, progress: progress(100),
            receive: {filesSaved: 2, subfolderExists: false, markingWarning: false},
        }])).toEqual({
            kind: 'transfer-progress', sessionId, seq: 3, progress: progress(100),
            receive: {filesSaved: 2, result: null, subfolderExists: false, markingWarning: false},
        })
    })

    it('parses the pinned receive complete event', () => {
        expect(parseLifecycleEvent('transfer-complete', [{
            sessionId, seq: 4, progress: progress(400),
            receive: {filesSaved: 3, result: 'complete', subfolderExists: true, markingWarning: false},
        }])).toMatchObject({
            kind: 'transfer-complete',
            receive: {filesSaved: 3, result: 'complete', subfolderExists: true, markingWarning: false},
        })
    })

    it('parses the pinned receive cancelled event and keeps the cancelled code only here', () => {
        expect(parseLifecycleEvent('transfer-error', [{
            sessionId, seq: 5,
            error: {code: 'cancelled', message: 'Transfer canceled.'},
            receive: {filesSaved: 1, result: 'cancelled', subfolderExists: true, markingWarning: true},
        }])).toEqual({
            kind: 'transfer-error', sessionId, seq: 5, progress: null,
            error: {code: 'cancelled', message: 'Transfer canceled.'},
            receive: {filesSaved: 1, result: 'cancelled', subfolderExists: true, markingWarning: true},
        })
    })

    it('parses an incomplete receive error and rewrites its message to the registry copy', () => {
        expect(parseLifecycleEvent('transfer-error', [{
            sessionId, seq: 5, progress: progress(100),
            error: {code: 'transfer_failed', message: 'forged'},
            receive: {filesSaved: 2, result: 'incomplete', subfolderExists: true, markingWarning: false},
        }])).toMatchObject({
            error: {
                code: 'transfer_failed',
                message: 'The transfer stopped before FairDrop finished sending. Check the local network and create a fresh link.',
            },
            receive: {filesSaved: 2, result: 'incomplete'},
        })
    })

    it('maps an incomplete receive error with an unknown code to transfer_failed rather than refusing it', () => {
        expect(parseLifecycleEvent('transfer-error', [{
            sessionId, seq: 5,
            error: {code: 'quota_exceeded', message: 'x'},
            receive: {filesSaved: 2, result: 'incomplete', subfolderExists: true, markingWarning: false},
        }])).toMatchObject({error: {code: 'transfer_failed'}, receive: {result: 'incomplete'}})
    })

    it.each([
        ['an incomplete result beside the cancelled code', 'incomplete', 'cancelled'],
        ['a cancelled result beside transfer_failed', 'cancelled', 'transfer_failed'],
        ['a cancelled result beside an unknown code', 'cancelled', 'quota_exceeded'],
    ] as const)('refuses %s: a contradiction is never guessed at', (_name, result, code) => {
        expect(parseLifecycleEvent('transfer-error', [{
            sessionId, seq: 5,
            error: {code, message: 'x'},
            receive: {filesSaved: 1, result, subfolderExists: true, markingWarning: false},
        }])).toBeNull()
    })

    it('refuses a cancelled result with no error at all', () => {
        expect(parseLifecycleEvent('transfer-error', [{
            sessionId, seq: 5,
            receive: {filesSaved: 1, result: 'cancelled', subfolderExists: true, markingWarning: false},
        }])).toBeNull()
    })

    it.each([
        ['progress with a malformed receive', 'transfer-progress', {progress: progress(1), receive: {filesSaved: -1, subfolderExists: false, markingWarning: false}}],
        ['progress with a receive that already has a result', 'transfer-progress', {progress: progress(1), receive: {filesSaved: 1, result: 'complete', subfolderExists: false, markingWarning: false}}],
        ['complete with a receive that says incomplete', 'transfer-complete', {progress: progress(400), receive: {filesSaved: 1, result: 'incomplete', subfolderExists: true, markingWarning: false}}],
        ['complete with a receive that is not an object', 'transfer-complete', {progress: progress(400), receive: 'complete'}],
        ['started carrying a receive', 'transfer-started', {receive: {filesSaved: 0, subfolderExists: false, markingWarning: false}}],
        ['reset carrying a receive', 'transfer-reset', {receive: {filesSaved: 0, subfolderExists: false, markingWarning: false}}],
        ['progress carrying a stray notice', 'transfer-progress', {progress: progress(1), notice: 'receive_too_large'}],
        ['started carrying a stray notice', 'transfer-started', {notice: 'receive_too_large'}],
    ] as const)('refuses %s', (_name, kind, body) => {
        expect(parseLifecycleEvent(kind, [{sessionId, seq: 2, ...body}])).toBeNull()
    })

    it('still refuses a receive complete whose bytes do not reach the declared total', () => {
        expect(parseLifecycleEvent('transfer-complete', [{
            sessionId, seq: 4, progress: progress(399),
            receive: {filesSaved: 3, result: 'complete', subfolderExists: true, markingWarning: false},
        }])).toBeNull()
    })
})

describe('the transfer-notice event', () => {
    it('parses the one notice a waiting receive session can publish', () => {
        expect(parseLifecycleEvent('transfer-notice', [{sessionId, seq: 1, notice: 'receive_too_large'}])).toEqual({
            kind: 'transfer-notice', sessionId, seq: 1, notice: 'receive_too_large',
        })
    })

    it.each([
        ['an unknown notice', {notice: 'disk_on_fire'}],
        ['a missing notice', {}],
        ['an empty notice', {notice: ''}],
        ['a non-string notice', {notice: 1}],
        ['a notice carrying progress', {notice: 'receive_too_large', progress: progress(1)}],
        ['a notice carrying an error', {notice: 'receive_too_large', error: {code: 'busy', message: 'x'}}],
        ['a notice carrying a receive payload', {notice: 'receive_too_large', receive: {filesSaved: 0, subfolderExists: false, markingWarning: false}}],
    ] as const)('refuses %s', (_name, body) => {
        expect(parseLifecycleEvent('transfer-notice', [{sessionId, seq: 1, ...body}])).toBeNull()
    })

    it('needs the same session id and a positive sequence as every other event', () => {
        expect(parseLifecycleEvent('transfer-notice', [{sessionId: '', seq: 1, notice: 'receive_too_large'}])).toBeNull()
        expect(parseLifecycleEvent('transfer-notice', [{sessionId, seq: 0, notice: 'receive_too_large'}])).toBeNull()
    })
})
