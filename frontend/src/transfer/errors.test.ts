import {describe, expect, it} from 'vitest'
import {isTransferErrorCode, parseCommandError, transferErrorCodes} from './errors'

// Covers the "Frontend parse" row of the I/O & Edge-Case Matrix in
// _bmad-output/implementation-artifacts/spec-1-7-expose-safe-transfer-commands-through-wails.md.
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

const fixedTransferFailedCopy =
    'The transfer stopped before FairDrop finished sending. Check the local network and create a fresh link.'

describe('parseCommandError', () => {
    it('recognizes every stable code the backend can send', () => {
        for (const code of backendCodes) {
            const formatted = JSON.stringify({code, message: `copy for ${code}`})

            expect(parseCommandError(rejectionCarrying(formatted))).toEqual({
                code,
                message: `copy for ${code}`,
            })
        }
    })

    it('exposes exactly the backend code list, in one place', () => {
        expect([...transferErrorCodes]).toEqual(backendCodes)
    })

    it('surfaces the backend message rather than re-deriving one from the code', () => {
        const formatted = JSON.stringify({
            code: 'busy',
            message: 'Finish or cancel the current transfer before choosing another item.',
        })

        expect(parseCommandError(rejectionCarrying(formatted)).message).toBe(
            'Finish or cancel the current transfer before choosing another item.',
        )
    })

    it('ignores fields the backend did not promise', () => {
        const formatted = JSON.stringify({code: 'qr_failed', message: 'nope', path: String.raw`C:\x\a.txt`})

        expect(parseCommandError(rejectionCarrying(formatted))).toEqual({
            code: 'qr_failed',
            message: 'nope',
        })
    })

    it('accepts a bare string payload as well as an Error', () => {
        const formatted = JSON.stringify({code: 'cancelled', message: 'Transfer canceled.'})

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
        first.message = 'mutated'

        expect(parseCommandError(undefined).message).toBe(fixedTransferFailedCopy)
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
