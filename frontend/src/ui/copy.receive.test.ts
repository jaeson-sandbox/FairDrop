import {describe, expect, it} from 'vitest'
import {transferErrorCodes} from '../transfer/errors'
import {copy, errorHeadingFor, errorHeadings, errorMessageFor, errorMessages, receiveErrorCopy, receiveQrAltFor} from './copy'

/*
  Receive copy, every string a literal written at the assertion site
  (spec-phone-to-desktop-receiving, Story B). The EXPERIENCE.md table is checked
  against these strings by copy.test.ts; this file pins the values themselves
  and the composed functions the table cannot hold.
*/

describe('the receive strings', () => {
    it('holds the exact approved wording', () => {
        expect(copy.receive.open).toBe('Receive')
        expect(copy.receive.chooseAnotherFolder).toBe('Choose Another Folder')
        expect(copy.receive.hint).toBe('Get files from a phone on the same local network.')
        expect(copy.receive.pending).toBe('Preparing to receive…')
        expect(copy.receive.heading).toBe('Ready to receive')
        expect(copy.receive.instruction).toBe('Scan the code with the phone’s camera, then choose the files to upload.')
        expect(copy.receive.qrAlt).toBe('Upload QR code for [folder name]')
        expect(copy.receive.destinationNote).toBe('Each upload is saved in a new folder here.')
        expect(copy.receive.once).toBe('Works once: this link accepts one upload.')
        expect(copy.receive.discoveryWarning).toBe('Device discovery isn’t available. The QR code and upload link still work.')
        expect(copy.receive.tooLargeTitle).toBe('Upload too large')
        expect(copy.receive.tooLarge).toBe(
            'A phone tried to upload more than this computer has room for. FairDrop is still waiting. Choose fewer files on the phone, or cancel and pick a folder with more free space.',
        )
        expect(copy.receive.helpDifferentLan).toBe(
            'Phone can’t open the page? Make sure both devices use the same local Wi-Fi. Guest or isolated networks may block device-to-device traffic. Then cancel and choose Receive again for a fresh link.',
        )
        expect(copy.receive.receiving).toBe('Receiving')
        expect(copy.receive.receivedCaption).toBe('Received')
        expect(copy.receive.savedCaption).toBe('Saved')
        expect(copy.receive.savedNote).toBe('A file counts as saved once it has finished.')
        expect(copy.receive.markingWarning).toBe(
            'FairDrop couldn’t add the download mark to some files on this drive. The files were saved without it.',
        )
        expect(copy.receive.showInFolder).toBe('Show in Folder')
        expect(copy.receive.showFailed).toBe('FairDrop couldn’t open that folder. It may have been moved or renamed.')
        expect(copy.label.uploadLinkHeading).toBe('Upload link')
    })

    it('puts no ellipsis on a control label', () => {
        for (const label of [copy.receive.open, copy.receive.chooseAnotherFolder, copy.receive.showInFolder]) {
            expect(label.endsWith('…') || label.endsWith('...'), label).toBe(false)
        }
    })

    it('never claims more than "saved to this computer"', () => {
        const everything = JSON.stringify([
            Object.values(copy.receive).filter((value) => typeof value === 'string'),
            copy.receive.savedTo('X'), copy.receive.stoppedKept('X'), copy.receive.stoppedNone('X'),
            copy.receive.cancelledKept('X'), copy.receive.cancelledNone('X'),
        ])
        expect(everything).not.toMatch(/\bbackup|permanent|safe(ly)? stored|guarantee/i)
    })
})

describe('the composed count copy', () => {
    it.each([
        [0, 'no files saved'],
        [1, '1 file saved'],
        [2, '2 files saved'],
        [3, '3 files saved'],
        [10, '10 files saved'],
        [1234, '1234 files saved'],
    ])('says %i as "%s"', (count, text) => {
        expect(copy.receive.filesSaved(count)).toBe(text)
    })

    it.each([
        [0, 'No files saved'],
        [1, '1 file saved'],
        [3, '3 files saved'],
    ])('heads a complete outcome of %i as "%s"', (count, text) => {
        expect(copy.receive.completeHeading(count)).toBe(text)
    })

    it.each([
        [0, 'Upload incomplete — no files saved'],
        [1, 'Upload incomplete — 1 file saved'],
        [2, 'Upload incomplete — 2 files saved'],
    ])('heads an incomplete outcome of %i as "%s"', (count, text) => {
        expect(copy.receive.incompleteHeading(count)).toBe(text)
    })

    it.each([
        [0, 'Upload cancelled — no files saved'],
        [1, 'Upload cancelled — 1 file saved'],
        [2, 'Upload cancelled — 2 files saved'],
    ])('heads a cancelled outcome of %i as "%s"', (count, text) => {
        expect(copy.receive.cancelledHeading(count)).toBe(text)
    })

    it('never says "1 files" for any wording', () => {
        for (const text of [
            copy.receive.filesSaved(1), copy.receive.completeHeading(1),
            copy.receive.incompleteHeading(1), copy.receive.cancelledHeading(1),
        ]) {
            expect(text).not.toMatch(/\b1 files\b/)
        }
    })

    it('keeps the incomplete and cancelled headings distinct from the success heading', () => {
        for (const count of [0, 1, 2, 5]) {
            expect(copy.receive.incompleteHeading(count)).not.toBe(copy.receive.completeHeading(count))
            expect(copy.receive.cancelledHeading(count)).not.toBe(copy.receive.completeHeading(count))
            expect(copy.receive.incompleteHeading(count)).toContain('Upload incomplete')
            expect(copy.receive.cancelledHeading(count)).toContain('Upload cancelled')
        }
    })
})

describe('the composed outcome sentences', () => {
    const open = '⁨'
    const close = '⁩'

    it('isolates the folder name in every sentence that carries one', () => {
        expect(copy.receive.savedTo('Phone Photos')).toBe(`Saved to this computer, in a new folder inside ${open}Phone Photos${close}.`)
        expect(copy.receive.stoppedKept('Phone Photos')).toBe(
            `The upload stopped early. Files that finished are saved to this computer, in a new folder inside ${open}Phone Photos${close}. The file in progress was removed.`,
        )
        expect(copy.receive.stoppedNone('Phone Photos')).toBe(
            `The upload stopped before any file finished saving. Nothing was added to ${open}Phone Photos${close}.`,
        )
        expect(copy.receive.cancelledKept('Phone Photos')).toBe(
            `You cancelled the upload. Files that finished are saved to this computer, in a new folder inside ${open}Phone Photos${close}. The file in progress was removed.`,
        )
        expect(copy.receive.cancelledNone('Phone Photos')).toBe(
            `You cancelled the upload before any file finished saving. Nothing was added to ${open}Phone Photos${close}.`,
        )
    })

    it('inserts a $-bearing name exactly as given', () => {
        expect(copy.receive.savedTo("a$&b $` $' $$")).toBe(`Saved to this computer, in a new folder inside ${open}a$&b $\` $' $$${close}.`)
    })
})

describe('the receive QR name', () => {
    it('fills the one placeholder', () => {
        expect(receiveQrAltFor('Phone Photos')).toBe('Upload QR code for Phone Photos')
        expect(receiveQrAltFor('')).toBe('Upload QR code for ')
    })

    it.each([['$&'], ['$`'], ["$'"], ['$$'], ['a$&b']])('inserts %j exactly as given', (name) => {
        expect(receiveQrAltFor(name)).toBe(`Upload QR code for ${name}`)
    })
})

describe('receive failure wording', () => {
    it('holds the exact heading and message for every code a receive start can produce', () => {
        expect(receiveErrorCopy).toEqual({
            invalid_selection: {heading: 'Choose a folder', message: 'Choose a folder to save the received files in.'},
            path_not_found: {heading: 'Folder not found', message: 'That folder is no longer available. Choose it again.'},
            path_unsupported: {
                heading: 'Can’t save there',
                message: 'FairDrop can only save to a regular folder it can write to. Links, read-only locations and OneDrive-style placeholder folders aren’t supported. Choose another folder.',
            },
            setup_failed: {
                heading: 'Couldn’t start receiving',
                message: 'FairDrop couldn’t start receiving into that folder. Nothing was saved. Choose it again.',
            },
            transfer_failed: {
                heading: 'Couldn’t start receiving',
                message: 'FairDrop couldn’t finish getting ready to receive. Nothing was saved. Try again.',
            },
            qr_failed: {
                heading: 'Couldn’t create the QR code',
                message: 'FairDrop couldn’t create the QR code. Try receiving again.',
            },
            cleanup_unconfirmed: {
                heading: 'Couldn’t confirm it stopped',
                message: 'FairDrop couldn’t confirm it released the connection. Nothing was saved. Close FairDrop and reopen it before receiving again.',
            },
            not_ready: {
                heading: 'FairDrop isn’t ready',
                message: 'FairDrop isn’t ready to receive. Another copy may already be running. Close this window and use that one.',
            },
            shutting_down: {heading: 'FairDrop is closing', message: 'FairDrop is closing. Reopen it to start receiving.'},
            chooser_failed: {heading: 'Couldn’t open the chooser', message: 'FairDrop couldn’t open the folder chooser. Try again.'},
        })
    })

    it('only ever overrides a real stable code', () => {
        for (const code of Object.keys(receiveErrorCopy)) {
            expect(transferErrorCodes as readonly string[]).toContain(code)
        }
    })

    it('reads receive wording for a receive failure and send wording otherwise', () => {
        expect(errorHeadingFor('path_not_found', true)).toBe('Folder not found')
        expect(errorHeadingFor('path_not_found', false)).toBe('Item not found')
        expect(errorMessageFor('path_not_found', true)).toBe('That folder is no longer available. Choose it again.')
        expect(errorMessageFor('path_not_found', false)).toBe('That file or folder is no longer available. Choose it again.')
    })

    it('falls back to the shared wording for a code with no receive override', () => {
        for (const code of ['busy', 'network_unavailable', 'server_start_failed', 'clipboard_failed'] as const) {
            expect(errorHeadingFor(code, true), code).toBe(errorHeadings[code])
            expect(errorMessageFor(code, true), code).toBe(errorMessages[code])
        }
    })

    it('never says send, item or prepare in receive failure wording', () => {
        for (const {heading, message} of Object.values(receiveErrorCopy)) {
            expect(`${heading} ${message}`).not.toMatch(/\b(send|sent|sending|prepare|item)\b/i)
        }
    })

    it('contains no banned vocabulary', () => {
        const banned = [/\bsecure\b/i, /\bprivate\b/i, /\bpair\b/i, /\bsync\b/i, /\bairdrop\b/i, /\bencrypted\b/i, /\bcloud\b/i]
        const everything = [
            ...Object.values(receiveErrorCopy).flatMap(({heading, message}) => [heading, message]),
            ...(Object.values(copy.receive) as unknown[]).filter((value): value is string => typeof value === 'string'),
            copy.receive.savedTo('X'), copy.receive.stoppedKept('X'), copy.receive.stoppedNone('X'),
            copy.receive.cancelledKept('X'), copy.receive.cancelledNone('X'),
            copy.receive.incompleteHeading(1), copy.receive.cancelledHeading(1), copy.receive.completeHeading(1),
        ]
        for (const text of everything) {
            for (const pattern of banned) expect(text, `${pattern} in ${JSON.stringify(text)}`).not.toMatch(pattern)
        }
    })

    it('is frozen like the rest of the registry', () => {
        expect(() => { (receiveErrorCopy.path_not_found as {heading: string}).heading = 'x' }).toThrow()
        expect(() => { (copy.receive as {open: string}).open = 'x' }).toThrow()
    })
})
