import {describe, expect, it} from 'vitest'
import {fixedErrorMessages} from '../transfer/errors'
import {copy, errorHeadings, errorMessages, qrAltFor} from './copy'

/*
  Every expectation below is a literal written out at the assertion site, not a
  reference back to the registry. Comparing copy.ts to itself would let a
  wording change sail through green while the screen said something the
  experience spine never approved.
*/

describe('approved product copy', () => {
    it('holds the exact experience-spine string for every stable key', () => {
        expect(copy.external.promise, 'copy.external.promise').toBe(
            'Send from FairDrop on Windows or Mac to one browser on the same local network—no account or receiver app.',
        )
        expect(copy.idle.instruction, 'copy.idle.instruction').toBe(
            'Drop one file or folder.',
        )
        expect(copy.firewall.preflight, 'copy.firewall.preflight').toBe(
            'Your first transfer may ask to allow FairDrop on this local network.',
        )
        expect(copy.firewall.windows, 'copy.firewall.windows').toBe(
            'Allow FairDrop on Private networks only. Leave Public networks off.',
        )
        expect(copy.firewall.macos, 'copy.firewall.macos').toBe(
            'Allow incoming connections for FairDrop.',
        )
        expect(copy.firewall.windowsRecovery, 'copy.firewall.windows_recovery').toBe(
            'Open Windows Firewall settings and allow FairDrop on Private networks only, then prepare the item again.',
        )
        expect(copy.firewall.macosRecovery, 'copy.firewall.macos_recovery').toBe(
            'Open System Settings → Network → Firewall → Options, allow incoming connections for FairDrop, then prepare the item again.',
        )
        expect(copy.stage.pending.file, 'copy.stage.pending.file').toBe(
            'Preparing your file…',
        )
        expect(copy.stage.pending.folder, 'copy.stage.pending.folder').toBe(
            'Preparing your folder…',
        )
        expect(copy.stage.heading, 'copy.stage.heading').toBe(
            'Ready to pass along',
        )
        expect(copy.qr.instruction, 'copy.qr.instruction').toBe(
            'Scan this code on the receiving device to start the download.',
        )
        expect(copy.qr.alt, 'copy.qr.alt').toBe(
            'Download QR code for [item name]',
        )
        expect(copy.folder.note, 'copy.folder.note').toBe(
            'This folder downloads as a ZIP.',
        )
        expect(copy.directLink.action, 'copy.direct_link.action').toBe(
            'Copy download link',
        )
        expect(copy.directLink.helper, 'copy.direct_link.helper').toBe(
            'Open this link directly in the receiving device’s browser.',
        )
        expect(copy.firstOpener.warning, 'copy.first_opener.warning').toBe(
            'One device only—the first device or software to open this link starts the download. Link previews may use this V1 link before the intended browser.',
        )
        expect(copy.network.disclosure, 'copy.network.disclosure').toBe(
            'Use FairDrop only on a network you trust. The transfer is not encrypted, so someone monitoring this network may be able to observe it.',
        )
        expect(copy.localCopy.disclosure, 'copy.local_copy.disclosure').toBe(
            'Sent directly over your local network. FairDrop does not upload or store an extra copy. The receiving device keeps the downloaded file.',
        )
        expect(copy.copy.confirmation, 'copy.copy.confirmation').toBe(
            'Copied',
        )
        expect(copy.discovery.warning, 'copy.discovery.warning').toBe(
            'Device discovery isn’t available. The QR code and download link still work.',
        )
        expect(copy.progress.unknown, 'copy.progress.unknown').toBe(
            'Sending — total size unknown',
        )
        expect(copy.progress.knownEmpty, 'copy.progress.known_empty').toBe(
            'Empty file — 0 bytes to transfer',
        )
        expect(copy.done.heading, 'copy.done.heading').toBe(
            'Transfer finished',
        )
        expect(copy.done.body, 'copy.done.body').toBe(
            'FairDrop finished sending the item.',
        )
        expect(copy.cancel.preparation, 'copy.cancel.preparation').toBe(
            'Cancel preparation',
        )
        expect(copy.cancel.preparationPending, 'copy.cancel.preparation_pending').toBe(
            'Canceling preparation…',
        )
        expect(copy.cancel.action, 'copy.cancel.action').toBe(
            'Cancel',
        )
        expect(copy.cancel.pending, 'copy.cancel.pending').toBe(
            'Canceling…',
        )
        expect(copy.cancel.won, 'copy.cancel.won').toBe(
            'Transfer canceled. Ready for another file or folder.',
        )
        expect(copy.outcome.dismiss, 'copy.outcome.dismiss').toBe(
            'Dismiss',
        )
        expect(copy.name.showFull, 'copy.name.show_full').toBe(
            'Show full name',
        )
        expect(copy.help.differentLan, 'copy.help.different_lan').toBe(
            'Not downloading? Make sure both devices use the same local Wi-Fi. Guest or isolated networks may block device-to-device traffic. Then cancel and prepare the item again for a fresh link.',
        )
        expect(copy.help.receiverHttp, 'copy.help.receiver_http').toBe(
            'Browser says Not Found: the link may be wrong or expired. Locked: another opener claimed it. Gone: the selected item changed. Cancel and prepare the item again for a fresh link.',
        )
    })

    /*
      A count is blind to a rename: dropping one key and adding another keeps
      the total identical. Derive nothing from the object under test -- the
      expected paths are written out here, so a moved, renamed or silently
      added string fails with the path that changed.
    */
    it('exposes exactly the key paths the spine registers, and no extra prose', () => {
        const paths: string[] = []
        const walk = (value: object, prefix: string): void => {
            for (const [key, nested] of Object.entries(value)) {
                const path = prefix === '' ? key : `${prefix}.${key}`
                if (typeof nested === 'string') paths.push(path)
                else walk(nested as object, path)
            }
        }
        walk(copy, '')

        expect(paths.sort()).toEqual([
            'cancel.action',
            'cancel.pending',
            'cancel.preparation',
            'cancel.preparationPending',
            'cancel.won',
            'copy.confirmation',
            'directLink.action',
            'directLink.helper',
            'discovery.warning',
            'done.body',
            'done.heading',
            'external.promise',
            'firewall.macos',
            'firewall.macosRecovery',
            'firewall.preflight',
            'firewall.windows',
            'firewall.windowsRecovery',
            'firstOpener.warning',
            'folder.note',
            'help.differentLan',
            'help.receiverHttp',
            'idle.instruction',
            'label.directLinkHeading',
            'label.file',
            'label.firewallHeading',
            'label.folder',
            'label.logicalSize',
            'label.macos',
            'label.metaSeparator',
            'label.of',
            'label.selectDirectory',
            'label.selectFile',
            'label.sending',
            'label.sent',
            'label.throughput',
            'label.windows',
            'label.wireBytes',
            'localCopy.disclosure',
            'name.showFull',
            'network.disclosure',
            'outcome.dismiss',
            'progress.knownEmpty',
            'progress.unknown',
            'qr.alt',
            'qr.instruction',
            'stage.heading',
            'stage.pending.file',
            'stage.pending.folder',
            'unit.byte',
            'unit.bytes',
            'unit.gigabytes',
            'unit.kilobytes',
            'unit.megabytes',
            'unit.perSecond',
            'unit.terabytes',
        ])
    })
})

describe('functional labels the spine names in prose', () => {
    it('holds the control, firewall, item and metric words used by the views', () => {
        expect(copy.label.selectFile).toBe('Select File')
        expect(copy.label.selectDirectory).toBe('Select Directory')
        expect(copy.label.firewallHeading).toBe('Local network access')
        expect(copy.label.windows).toBe('Windows')
        expect(copy.label.macos).toBe('macOS')
        expect(copy.label.file).toBe('File')
        expect(copy.label.folder).toBe('Folder')
        expect(copy.label.logicalSize).toBe('logical size')
        expect(copy.label.directLinkHeading).toBe('Direct download link')
        expect(copy.label.sending).toBe('Sending')
        expect(copy.label.wireBytes).toBe('Wire bytes')
        expect(copy.label.throughput).toBe('Throughput')
        expect(copy.label.sent).toBe('sent')
        expect(copy.label.of).toBe('of')
        expect(copy.label.metaSeparator).toBe(' \u00b7 ')
    })

    it('holds decimal byte units and the rate suffix', () => {
        expect(copy.unit.byte).toBe('byte')
        expect(copy.unit.bytes).toBe('bytes')
        expect(copy.unit.kilobytes).toBe('KB')
        expect(copy.unit.megabytes).toBe('MB')
        expect(copy.unit.gigabytes).toBe('GB')
        expect(copy.unit.terabytes).toBe('TB')
        expect(copy.unit.perSecond).toBe('/s')
    })
})

describe('fixed error surface', () => {
    it('pairs every stable code with its exact visible heading', () => {
        expect(errorHeadings).toEqual({
            invalid_selection: 'Choose one item',
            busy: 'Transfer already active',
            cancelled: 'Transfer canceled',
            path_not_found: 'Item not found',
            path_unsupported: 'Can’t use that item',
            source_changed: 'Item changed',
            network_unavailable: 'Local network unavailable',
            server_start_failed: 'Couldn’t open a connection',
            qr_failed: 'Couldn’t create the QR code',
            beacon_warning: 'Discovery unavailable',
            transfer_failed: 'Transfer stopped',
            shutting_down: 'FairDrop is closing',
        })
    })

    it('re-exports the validated message table rather than restating it', () => {
        expect(errorMessages).toBe(fixedErrorMessages)
        expect(errorMessages.invalid_selection).toBe('Choose exactly one file or folder.')
        expect(errorMessages.transfer_failed).toBe(
            'The transfer stopped before FairDrop finished sending. Check the local network and create a fresh link.',
        )
    })

    it('gives the beacon warning the same message as the discovery copy key', () => {
        expect(errorMessages.beacon_warning).toBe(copy.discovery.warning)
    })
})

describe('the QR accessible-name template', () => {
    it('substitutes the item name for the one placeholder', () => {
        expect(qrAltFor('Dad\u2019s PDFs')).toBe('Download QR code for Dad\u2019s PDFs')
    })

    it('leaves no placeholder behind for an empty name', () => {
        expect(qrAltFor('')).toBe('Download QR code for ')
        expect(qrAltFor('')).not.toContain('[item name]')
    })
})

describe('the QR accessible-name template under $-bearing names', () => {
    /*
      `String.replace` with a string replacement gives `$&`, `` $` ``, `$'` and
      `$$` meaning inside the REPLACEMENT. A file may legally be named any of
      them, and each used to rewrite its own accessible name: `` $` `` copied
      the template back in and `$'` deleted the rest of it.
    */
    it.each([
        ['$&', 'Download QR code for $&'],
        ['$`', 'Download QR code for $`'],
        ["$'", "Download QR code for $'"],
        ['$$', 'Download QR code for $$'],
        ['a$&b.pdf', 'Download QR code for a$&b.pdf'],
        ['quarterly $`24 report.pdf', 'Download QR code for quarterly $`24 report.pdf'],
        ['report.pdf', 'Download QR code for report.pdf'],
    ])('inserts %j exactly as given', (name, expected) => {
        expect(qrAltFor(name)).toBe(expected)
    })
})

describe('banned vocabulary', () => {
    /*
      Each entry carries a sample that must match, so a pattern that stops
      matching -- a mangled escape, a stray anchor -- fails here instead of
      quietly passing every not.toMatch below it.

      The two Windows firewall strings name the operating system's own "Private
      networks" profile, which the spine approves verbatim, and the network
      disclosure has to say "not encrypted" to be honest. Both are exempted by
      identity rather than by loosening a pattern.
    */
    const banned = [
        {term: 'secure', pattern: /\bsecure\b/i, sample: 'a secure transfer'},
        {term: 'private', pattern: /\bprivate\b/i, sample: 'a private link'},
        {term: 'pair', pattern: /\bpair\b/i, sample: 'pair the two devices'},
        {term: 'sync', pattern: /\bsync\b/i, sample: 'sync your folder'},
        {term: 'airdrop', pattern: /\bairdrop\b/i, sample: 'AirDrop for any device'},
        {term: 'encrypted', pattern: /\bencrypted\b/i, sample: 'the transfer is encrypted'},
        {term: 'universal', pattern: /works with every device/i, sample: 'works with every device'},
        {term: 'universal compatibility', pattern: /universal compat/i, sample: 'universal compatibility'},
    ]

    it('uses patterns that catch the term each one names', () => {
        for (const {term, pattern, sample} of banned) {
            expect(sample, term).toMatch(pattern)
        }
    })

    it('does not mistake the product name for the benchmark name', () => {
        expect('FairDrop is closing').not.toMatch(/\bairdrop\b/i)
        expect('AirDrop').toMatch(/\bairdrop\b/i)
    })

    it('keeps every banned term out of every registered string', () => {
        const profileStrings: readonly string[] = [copy.firewall.windows, copy.firewall.windowsRecovery]

        for (const value of collectStrings(copy)) {
            for (const {term, pattern} of banned) {
                if (term === 'private' && profileStrings.includes(value)) continue
                // Exempt the two approved strings by identity. `includes` would
                // have let any future sentence carry the term through by
                // quoting the disclosure.
                if (term === 'encrypted' && (value === copy.network.disclosure ||
                    value === errorMessages.beacon_warning)) continue
                expect(value, `${term} in ${JSON.stringify(value)}`).not.toMatch(pattern)
            }
        }
    })

    it('keeps every banned term out of every fixed error heading and message', () => {
        for (const value of [...Object.values(errorHeadings), ...Object.values(errorMessages)]) {
            for (const {term, pattern} of banned) {
                expect(value, `${term} in ${JSON.stringify(value)}`).not.toMatch(pattern)
            }
        }
    })
})

describe('registry immutability', () => {
    it('refuses a write to a nested string at runtime', () => {
        const mutable = copy.idle as {instruction: string}

        expect(() => {
            mutable.instruction = 'Drop anything you like.'
        }).toThrow()
        expect(copy.idle.instruction).toBe('Drop one file or folder.')
    })

    it('refuses a write to a heading', () => {
        const mutable = errorHeadings as Record<string, string>

        expect(() => {
            mutable.busy = 'Busy'
        }).toThrow()
        expect(errorHeadings.busy).toBe('Transfer already active')
    })
})

function collectStrings(value: object): string[] {
    const found: string[] = []
    for (const nested of Object.values(value)) {
        if (typeof nested === 'string') found.push(nested)
        else if (typeof nested === 'object' && nested !== null) found.push(...collectStrings(nested))
    }
    return found
}
