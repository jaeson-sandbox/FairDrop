import {readFileSync, readdirSync} from 'node:fs'
import {resolve} from 'node:path'
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
            'Drop one file or folder',
        )
        expect(copy.idle.promise, 'copy.idle.promise').toBe(
            'Sends to one browser on the same local network. No account or receiver app.',
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
        expect(copy.stage.pending.item, 'copy.stage.pending.item').toBe(
            'Preparing your item…',
        )
        expect(copy.stage.heading, 'copy.stage.heading').toBe(
            'Ready to send',
        )
        expect(copy.qr.instruction, 'copy.qr.instruction').toBe(
            'Scan the code with the receiving device’s camera.',
        )
        expect(copy.qr.alt, 'copy.qr.alt').toBe(
            'Download QR code for [item name]',
        )
        expect(copy.folder.note, 'copy.folder.note').toBe(
            'This folder downloads as a ZIP.',
        )
        expect(copy.directLink.action, 'copy.direct_link.action').toBe(
            'Copy Link',
        )
        expect(copy.directLink.show, 'copy.direct_link.show').toBe(
            'Show Link',
        )
        expect(copy.directLink.hide, 'copy.direct_link.hide').toBe(
            'Hide Link',
        )
        expect(copy.firstOpener.warning, 'copy.first_opener.warning').toBe(
            'Works once: the first device to open it gets the file.',
        )
        expect(copy.firstOpener.previews, 'copy.first_opener.previews').toBe(
            'Link previews in chat apps can count as that first device, so paste the link straight into a browser.',
        )
        expect(copy.network.disclosure, 'copy.network.disclosure').toBe(
            'Not encrypted. Use it only on a network you trust.',
        )
        expect(copy.localCopy.disclosure, 'copy.local_copy.disclosure').toBe(
            'FairDrop keeps no copy. The receiving device keeps what it downloads.',
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
            'Sent',
        )
        expect(copy.done.sendAnother, 'copy.done.send_another').toBe(
            'Send Another',
        )
        expect(copy.done.dismiss, 'copy.done.dismiss').toBe(
            'Done',
        )
        expect(copy.cancel.preparation, 'copy.cancel.preparation').toBe(
            'Cancel preparation',
        )
        expect(copy.cancel.preparationPending, 'copy.cancel.preparation_pending').toBe(
            'Canceling preparation',
        )
        expect(copy.cancel.action, 'copy.cancel.action').toBe(
            'Cancel',
        )
        expect(copy.cancel.pending, 'copy.cancel.pending').toBe(
            'Canceling',
        )
        expect(copy.cancel.won, 'copy.cancel.won').toBe(
            'Transfer canceled. Ready for another file or folder.',
        )
        expect(copy.outcome.dismiss, 'copy.outcome.dismiss').toBe(
            'Dismiss',
        )
        expect(copy.outcome.tryAgain, 'copy.outcome.try_again').toBe(
            'Try Again',
        )
        expect(copy.outcome.chooseAnother, 'copy.outcome.choose_another').toBe(
            'Choose Another',
        )
        expect(copy.help.heading, 'copy.help.heading').toBe(
            'Trouble connecting?',
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
            'directLink.hide',
            'directLink.show',
            'discovery.warning',
            'done.dismiss',
            'done.heading',
            'done.sendAnother',
            'external.promise',
            'firewall.macos',
            'firewall.macosRecovery',
            'firewall.preflight',
            'firewall.windows',
            'firewall.windowsRecovery',
            'firstOpener.previews',
            'firstOpener.warning',
            'folder.note',
            'help.differentLan',
            'help.heading',
            'help.receiverHttp',
            'idle.instruction',
            'idle.promise',
            'label.chooseFileOrFolder',
            'label.directLinkHeading',
            'label.file',
            'label.firewallHeading',
            'label.folder',
            'label.logicalSize',
            'label.macos',
            'label.macosRecovery',
            'label.metaSeparator',
            'label.of',
            'label.recoveryHeading',
            'label.sending',
            'label.sent',
            'label.sentCaption',
            'label.speedCaption',
            'label.throughput',
            'label.windows',
            'label.windowsRecovery',
            'label.wireBytes',
            'localCopy.disclosure',
            'network.disclosure',
            'outcome.chooseAnother',
            'outcome.dismiss',
            'outcome.tryAgain',
            'progress.knownEmpty',
            'progress.unknown',
            'qr.alt',
            'qr.instruction',
            'stage.heading',
            'stage.pending.file',
            'stage.pending.folder',
            'stage.pending.item',
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
        expect(copy.label.chooseFileOrFolder).toBe('Choose File or Folder')
        expect(copy.label.firewallHeading).toBe('Local network access')
        expect(copy.label.recoveryHeading).toBe('Troubleshooting')
        expect(copy.label.windows).toBe('Windows')
        expect(copy.label.macos).toBe('macOS')
        expect(copy.label.file).toBe('File')
        expect(copy.label.folder).toBe('Folder')
        expect(copy.label.logicalSize).toBe('logical size')
        expect(copy.label.directLinkHeading).toBe('Direct download link')
        expect(copy.label.sending).toBe('Sending')
        expect(copy.label.wireBytes).toBe('Wire bytes')
        expect(copy.label.throughput).toBe('Throughput')
        expect(copy.label.sentCaption).toBe('Sent')
        expect(copy.label.speedCaption).toBe('Speed')
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
            busy: 'FairDrop is still busy',
            cancelled: 'Transfer canceled',
            path_not_found: 'Item not found',
            path_unsupported: 'Can’t use that item',
            source_changed: 'Item changed',
            network_unavailable: 'Local network unavailable',
            server_start_failed: 'Couldn’t open a connection',
            qr_failed: 'Couldn’t create the QR code',
            setup_failed: 'Couldn’t prepare that item',
            beacon_warning: 'Discovery unavailable',
            transfer_failed: 'Transfer stopped',
            cleanup_unconfirmed: 'Couldn’t confirm it stopped',
            not_ready: 'FairDrop isn’t ready',
            clipboard_failed: 'Couldn’t copy the link',
            name_unsupported: 'A name can’t be sent',
            name_warning: 'Some names may not save',
            shutting_down: 'FairDrop is closing',
            chooser_failed: 'Couldn’t open the chooser',
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
        expect(copy.idle.instruction).toBe('Drop one file or folder')
    })

    it('refuses a write to a heading', () => {
        const mutable = errorHeadings as Record<string, string>

        expect(() => {
            mutable.busy = 'Busy'
        }).toThrow()
        expect(errorHeadings.busy).toBe('FairDrop is still busy')
    })
})

/*
  The third side of the triangle, and the one that was missing (D-119).

  Above, every string is pinned against a literal written at the assertion
  site, and every key path against a written-out list -- so a renamed or added
  leaf fails here. Neither reads EXPERIENCE.md. The spine's "Voice and Tone"
  table is the document that makes a string *approved*, and Story 4.1 shipped
  three new ones whose rows existed only because someone went looking: nothing
  would have failed without them.

  The gap runs one way. A row naming no leaf is dead and harmless; a leaf with
  no row is unapproved copy on the screen. Both are checked below anyway,
  because a dead row is how a rename hides.

  What stays human: whether a *new* `label` entry is approved copy that belongs
  in the table (`chooseFileOrFolder`, `file` and `folder` are, and are
  tabulated) or a structural word the spine names in prose (`windows`,
  `throughput`). No mechanical line separates those -- both groups are short
  and both contain spaces -- so this pins the half that can be pinned: every
  non-label, non-unit leaf needs a row, and every row that exists must agree.
*/
describe('the spine table and the registry that quotes it', () => {
    // Vitest roots at frontend/, the same anchor progressSpeech.test.ts uses.
    const designs = resolve(process.cwd(), '..', '_bmad-output', 'planning-artifacts', 'ux-designs')
    const folders = readdirSync(designs).filter((entry) => entry.startsWith('ux-'))
    // Thrown, not expected: this runs at collection time, where a failed
    // expectation belongs to no test and would be reported as an empty file
    // rather than as the missing spine it is.
    if (folders.length !== 1) throw new Error(`expected one ux-* design folder, found ${folders.length}`)
    const spine = readFileSync(resolve(designs, folders[0], 'EXPERIENCE.md'), 'utf8')

    // The table quotes values typographically; copy.ts stores them bare.
    const openQuote = String.fromCharCode(0x201C)
    const closeQuote = String.fromCharCode(0x201D)

    /** `copy.direct_link.action` -> `directLink.action`, dropping the registry name. */
    function pathOf(id: string): string {
        return id
            .replace('copy.', '')
            .split('.')
            .map((part) => part.replace(/_(.)/g, (_, letter: string) => letter.toUpperCase()))
            .join('.')
    }

    function valueAt(path: string): string | undefined {
        let node: unknown = copy
        for (const key of path.split('.')) {
            if (typeof node !== 'object' || node === null) return undefined
            node = (node as Record<string, unknown>)[key]
        }
        return typeof node === 'string' ? node : undefined
    }

    const rows = spine
        .split(/\r?\n/)
        .filter((line) => line.trimStart().startsWith('| `copy.'))
        .map((line) => {
            const cells = line.split('|')
            // A value carrying a raw pipe would truncate silently; fail instead.
            expect(cells, `${line.slice(0, 48)} splits into five table cells`).toHaveLength(5)
            const id = cells[1].trim().replace(/`/g, '')
            const quoted = cells[3].trim()
            expect(
                quoted.startsWith(openQuote) && quoted.endsWith(closeQuote),
                `${id} is typographically quoted`,
            ).toBe(true)
            return {id, value: quoted.slice(1, -1)}
        })

    /*
      A parser that matched nothing would make both cases below pass over an
      empty set. A count alone is not enough either -- a regex that captured
      the id and dropped the value would still count rows -- so one known pair
      is asserted whole.
    */
    it('parses the table it is about to check', () => {
        expect(rows.length, 'rows parsed from the Voice and Tone table').toBeGreaterThan(30)
        expect(rows).toContainEqual({id: 'copy.idle.instruction', value: 'Drop one file or folder'})
    })

    it('quotes, in every row, the exact string the registry holds', () => {
        for (const {id, value} of rows) {
            expect(valueAt(pathOf(id)), `${id} names a registry string`).toBeTypeOf('string')
            expect(value, `${id} in EXPERIENCE.md matches copy.ts`).toBe(valueAt(pathOf(id)))
        }
    })

    it('registers every approved string, so new copy cannot ship untabulated', () => {
        const tabulated = new Set(rows.map((row) => pathOf(row.id)))
        const approved: string[] = []
        const walk = (value: object, prefix: string): void => {
            for (const [key, nested] of Object.entries(value)) {
                const path = prefix === '' ? key : `${prefix}.${key}`
                if (typeof nested === 'string') approved.push(path)
                else walk(nested as object, path)
            }
        }
        walk(copy, '')

        // `label` and `unit` are the functional words the spine writes in prose
        // rather than tabulating; copy.ts's own header draws that line.
        const needsARow = approved.filter((path) => !path.startsWith('label.') && !path.startsWith('unit.'))
        expect(needsARow.length, 'approved strings found to check').toBeGreaterThan(20)

        expect(needsARow.filter((path) => !tabulated.has(path))).toEqual([])
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
