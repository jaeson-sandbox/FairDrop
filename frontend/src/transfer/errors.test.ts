import {describe, expect, it} from 'vitest'
import {isTransferErrorCode, parseCommandError, parsePublicError, transferErrorCodes} from './errors'

// Covers the "Frontend parse" row of the I/O & Edge-Case Matrix in
// _bmad-output/implementation-artifacts/spec-1-8-manage-session-scoped-frontend-state-and-events.md.
//
// The rejection shape is what the Wails runtime really produces: calls.js does
// `new Error(message.error)`, and message.error is the string the Go
// ErrorFormatter returned. So every case here starts from an Error whose
// message is that JSON.

/** Builds the rejection a bound command produces for a given formatter output. */
function rejectionCarrying(formatted: string): Error {
    return new Error(formatted)
}

// Spelled out rather than imported from the module under test: this list is
// half of a cross-language contract, and comparing the module to itself would
// let both ends drift together.
const backendCodes = [
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
]

// Written independently of fixedErrorMessages so every literal is pinned at
// the assertion site. The object-shaped cases also put the changed code in a
// failing test's name instead of hiding it inside one loop.
const expectedFixedCopies = [
    {code: 'invalid_selection', message: 'Choose exactly one file or folder.'},
    {code: 'busy', message: 'Finish or cancel the current transfer before choosing another item.'},
    {code: 'cancelled', message: 'Transfer canceled.'},
    {code: 'path_not_found', message: 'That file or folder is no longer available. Choose it again.'},
    {code: 'path_unsupported', message: 'FairDrop can use regular files and folders only. Choose another item.'},
    {code: 'source_changed', message: 'The item changed after it was prepared. Cancel and create a fresh link.'},
    {code: 'network_unavailable', message: 'FairDrop couldn’t find a usable local network. Connect to local Wi-Fi, then try again.'},
    {code: 'server_start_failed', message: 'FairDrop couldn’t open a local transfer connection. Check firewall access, then try again.'},
    {code: 'qr_failed', message: 'FairDrop couldn’t create the QR code. Prepare the item again.'},
    {code: 'beacon_warning', message: 'Device discovery isn’t available. The QR code and download link still work.'},
    {code: 'transfer_failed', message: 'The transfer stopped before FairDrop finished sending. Check the local network and create a fresh link.'},
    {code: 'shutting_down', message: 'FairDrop is closing. Reopen it to start a transfer.'},
] as const

const fixedTransferFailedCopy =
    'The transfer stopped before FairDrop finished sending. Check the local network and create a fresh link.'

describe('parseCommandError', () => {
    it.each(expectedFixedCopies)('uses the exact literal fixed copy for $code', ({code, message}) => {
        const formatted = JSON.stringify({code, message: String.raw`C:\private\file-${code}?token=secret`})

        expect(parseCommandError(rejectionCarrying(formatted))).toEqual({code, message})
    })

    it('exposes exactly the backend code list, in one place', () => {
        expect([...transferErrorCodes]).toEqual(backendCodes)
    })

    it('uses exact fixed copy instead of a known code carrying forged prose', () => {
        const formatted = JSON.stringify({
            code: 'busy',
            message: String.raw`C:\private\document.txt?token=secret`,
        })

        expect(parseCommandError(rejectionCarrying(formatted)).message).toBe(
            'Finish or cancel the current transfer before choosing another item.',
        )
    })

    it('ignores fields the backend did not promise', () => {
        const formatted = JSON.stringify({code: 'qr_failed', message: 'nope', path: String.raw`C:\x\a.txt`})

        expect(parseCommandError(rejectionCarrying(formatted))).toEqual({
            code: 'qr_failed',
            message: 'FairDrop couldn’t create the QR code. Prepare the item again.',
        })
    })

    it('accepts a bare string payload as well as an Error', () => {
        const formatted = JSON.stringify({code: 'cancelled', message: 'forged'})

        expect(parseCommandError(formatted)).toEqual({code: 'cancelled', message: 'Transfer canceled.'})
    })

    // Every way the payload can fail validation lands on the same fixed
    // fallback, which is the code the backend itself uses for an unrecognized
    // error -- so the two ends name an unknown failure the same thing.
    it.each([
        ['a truncated payload', rejectionCarrying('{"code":"busy"')],
        ['prose instead of JSON', rejectionCarrying('runtime: call to CancelTransfer timed out')],
        ['an empty message', rejectionCarrying('')],
        ['JSON null', rejectionCarrying('null')],
        ['a JSON array', rejectionCarrying('[{"code":"busy","message":"x"}]')],
        ['a JSON number', rejectionCarrying('42')],
        ['a JSON string', rejectionCarrying('"busy"')],
        ['a code this build does not know', rejectionCarrying('{"code":"quota_exceeded","message":"x"}')],
        ['an absent code', rejectionCarrying('{"message":"x"}')],
        ['a non-string code', rejectionCarrying('{"code":7,"message":"x"}')],
        ['an absent message', rejectionCarrying('{"code":"busy"}')],
        ['a non-string message', rejectionCarrying('{"code":"busy","message":7}')],
        ['an empty message field', rejectionCarrying('{"code":"busy","message":""}')],
        // A message of spaces is not a message: it renders as a code beside a
        // blank line, which reads to the user as a broken UI rather than a
        // failure they can act on.
        ['a whitespace-only message', rejectionCarrying('{"code":"busy","message":"   "}')],
        ['undefined', undefined],
        ['null', null],
        ['a plain object', {code: 'busy', message: 'x'}],
        ['a number', 500],
        // Stringifies to exactly the payload a real rejection would carry, so
        // this is what proves the rejection is trusted only when it is an
        // Error or a string rather than coerced into one.
        ['a value that merely stringifies to a public error', [JSON.stringify({code: 'busy', message: 'x'})]],
    ])('falls back to transfer_failed for %s', (_name, rejection) => {
        expect(parseCommandError(rejection)).toEqual({
            code: 'transfer_failed',
            message: fixedTransferFailedCopy,
        })
    })

    it('returns a fresh fallback each time, so one caller cannot poison the next', () => {
        const first = parseCommandError(undefined)
        ;(first as {message: string}).message = 'mutated'

        expect(parseCommandError(undefined).message).toBe(fixedTransferFailedCopy)
    })

    it('falls back without throwing when an Error message getter throws', () => {
        const hostile = new Error('placeholder')
        Object.defineProperty(hostile, 'message', {get: () => { throw new Error('boom') }})

        expect(() => parseCommandError(hostile)).not.toThrow()
        expect(parseCommandError(hostile)).toEqual({
            code: 'transfer_failed',
            message: fixedTransferFailedCopy,
        })
    })
})

describe('parsePublicError', () => {
    it('is total for an object whose getters throw', () => {
        const hostile = Object.defineProperty({}, 'code', {get: () => { throw new Error('boom') }})

        expect(() => parsePublicError(hostile)).not.toThrow()
        expect(parsePublicError(hostile)).toBeNull()
    })
})

describe('isTransferErrorCode', () => {
    it('accepts the stable codes and rejects everything else', () => {
        expect(isTransferErrorCode('beacon_warning')).toBe(true)
        expect(isTransferErrorCode('quota_exceeded')).toBe(false)
        expect(isTransferErrorCode(undefined)).toBe(false)
        expect(isTransferErrorCode(7)).toBe(false)
    })
})
