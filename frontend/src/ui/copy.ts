/**
 * The one registry of literal product strings.
 *
 * Every visible character in every view comes from here, by the stable key the
 * experience spine assigns it. A view never spells a sentence of its own, so a
 * wording change is one edit in one file that a reviewer can diff against the
 * spine, instead of a search across the component tree.
 *
 * Two groups have different provenance:
 *
 *  - Everything above `label` is quoted character for character from the "Voice
 *    and Tone" table of EXPERIENCE.md. Those strings are approved product copy;
 *    changing one is a spine change, not a code change.
 *  - `label` and `unit` are the short functional words the spine names in prose
 *    rather than tabulating -- control names, the firewall block's own headings,
 *    metric captions, and byte units. Each carries its source in a comment.
 *
 * That split is enforced, not merely described: `copy.test.ts` reads the table
 * and requires a row for every leaf outside `label` and `unit`, so new approved
 * copy cannot ship untabulated (D-119). Three `label` entries are tabulated all
 * the same -- `chooseFileOrFolder`, `file` and `folder` are approved wording the
 * owner decided, not structural words -- and any row that exists must quote its
 * string exactly. Whether a new `label` entry earns a row stays a judgement:
 * nothing mechanical separates `Choose a file or folder` from `Wire bytes`.
 *
 * Error text is not restated here at all: `errorMessages` re-exports the frozen
 * table the reducer already validates against, so the two cannot drift apart.
 */

import {fixedErrorMessages, type TransferErrorCode} from '../transfer/errors'

export const copy = {
    external: {
        promise: 'Send from FairDrop on Windows or Mac to one browser on the same local network—no account or receiver app.',
    },
    idle: {
        instruction: 'Drop one file or folder',
        /**
         * Story 9.3: the Idle-only short promise line, rendered inside the
         * drop zone in place of `copy.external.promise`. That key keeps its
         * longer wording for external use (README, store copy) -- see its
         * own comment -- so this is a separate registered string rather than
         * a second reading of the same one.
         */
        promise: 'Sends to one browser on the same local network. No account or receiver app.',
    },
    firewall: {
        preflight: 'Your first transfer may ask to allow FairDrop on this local network.',
        windows: 'Allow FairDrop on Private networks only. Leave Public networks off.',
        macos: 'Allow incoming connections for FairDrop.',
        windowsRecovery: 'Open Windows Firewall settings and allow FairDrop on Private networks only, then prepare the item again.',
        macosRecovery: 'Open System Settings → Network → Firewall → Options, allow incoming connections for FairDrop, then prepare the item again.',
    },
    stage: {
        pending: {
            file: 'Preparing your file…',
            folder: 'Preparing your folder…',
            item: 'Preparing your item…',
        },
        heading: 'Ready to send',
    },
    qr: {
        instruction: 'Scan the code with the receiving device’s camera.',
        alt: 'Download QR code for [item name]',
    },
    folder: {
        note: 'This folder downloads as a ZIP.',
    },
    directLink: {
        action: 'Copy Link',
        /**
         * Story 9.4: the link is no longer rendered until the sender asks for
         * it. These two toggle `aria-expanded` and the field's own grid-rows
         * reveal (`.fd-url-reveal` in style.css, the same mechanism
         * `Disclosure` uses); `copy.direct_link.helper` is retired alongside
         * the always-visible field it used to sit beside.
         */
        show: 'Show Link',
        hide: 'Hide Link',
    },
    firstOpener: {
        warning: 'Works once: the first device to open it gets the file.',
        /**
         * Story 9.4: moved out of the always-visible card into "Trouble
         * connecting?" -- a link preview is the one first-opener case a
         * sender can actually act on, so it belongs beside the rest of the
         * troubleshooting guidance rather than in the one-line visible caveat.
         */
        previews: 'Link previews in chat apps can count as that first device, so paste the link straight into a browser.',
    },
    network: {
        disclosure: 'Not encrypted. Use it only on a network you trust.',
    },
    localCopy: {
        disclosure: 'FairDrop keeps no copy. The receiving device keeps what it downloads.',
    },
    copy: {
        confirmation: 'Copied',
    },
    discovery: {
        warning: 'Device discovery isn’t available. The QR code and download link still work.',
    },
    progress: {
        unknown: 'Sending — total size unknown',
        knownEmpty: 'Empty file — 0 bytes to transfer',
    },
    done: {
        heading: 'Transfer finished',
        body: 'FairDrop finished sending the item.',
    },
    cancel: {
        preparation: 'Cancel preparation',
        preparationPending: 'Canceling preparation',
        action: 'Cancel',
        pending: 'Canceling',
        won: 'Transfer canceled. Ready for another file or folder.',
    },
    outcome: {
        dismiss: 'Dismiss',
    },
    help: {
        /**
         * Story 9.4: the "Trouble connecting?" disclosure's own summary --
         * the second thing Staged's foot row offers, beside Cancel.
         */
        heading: 'Trouble connecting?',
        differentLan: 'Not downloading? Make sure both devices use the same local Wi-Fi. Guest or isolated networks may block device-to-device traffic. Then cancel and prepare the item again for a fresh link.',
        receiverHttp: 'Browser says Not Found: the link may be wrong or expired. Locked: another opener claimed it. Gone: the selected item changed. Cancel and prepare the item again for a fresh link.',
    },

    /** Functional words the spine names in prose rather than in the copy table. */
    label: {
        /**
         * Information Architecture, Idle row: "Native drop target, one browse
         * control (`copy.label.chooseFileOrFolder`) opening a menu for file or
         * folder". Replaces the retired `selectFile` / `selectDirectory` pair
         * (Story 4.1): the label names both kinds itself, and the menu it
         * opens is where the Windows/macOS dialog asymmetry is absorbed.
         */
        chooseFileOrFolder: 'Choose File or Folder',
        /** "Firewall Preflight and Recovery" bullet labels, in document order. */
        firewallHeading: 'Local network access',
        windows: 'Windows',
        macos: 'macOS',
        /* The recovery block's own labels, which the spine writes out. */
        windowsRecovery: 'Windows recovery',
        macosRecovery: 'macOS recovery',
        /**
         * The second Idle disclosure's summary (Story 7.3). `RecoveryHelp`
         * itself registers no heading of its own -- the disclosure that wraps
         * it is what needs a name, the same way `firewallHeading` names the
         * first one, so this is a structural label rather than approved body
         * copy.
         */
        recoveryHeading: 'Troubleshooting',
        /** The approved item vocabulary: file, folder. */
        file: 'File',
        folder: 'Folder',
        /** Item Summary: "sanitized bidi-isolated full name and logical size". */
        logicalSize: 'logical size',
        /** Direct URL Row heading, from the staged production reference. */
        directLinkHeading: 'Direct download link',
        /** Transferring state heading, from the transferring production reference. */
        sending: 'Sending',
        /**
         * Completion Receipt (Story 7.5, `OutcomePanel.tsx`): wire bytes
         * first, throughput second. Left exactly as it was -- Story 9.5 needs
         * its own, differently-worded captions on the Sending card
         * (`sentCaption`/`speedCaption` below) rather than renaming these,
         * because the Done receipt reads this same pair and 9.5's scope is
         * Sending only.
         */
        wireBytes: 'Wire bytes',
        throughput: 'Throughput',
        /**
         * Transfer Metrics (Story 9.5, `TransferringView.tsx` only): the
         * Sending card's own two figure captions, plainer than the receipt's
         * "Wire bytes"/"Throughput" above -- the owner-approved prototype's
         * own wording. The figures underneath are unchanged (actual wire
         * bytes; visual-only throughput).
         */
        sentCaption: 'Sent',
        speedCaption: 'Speed',
        sent: 'sent',
        of: 'of',
        metaSeparator: ' · ',
    },

    /** Byte and rate units, decimal because receiver browsers report decimal. */
    unit: {
        byte: 'byte',
        bytes: 'bytes',
        kilobytes: 'KB',
        megabytes: 'MB',
        gigabytes: 'GB',
        terabytes: 'TB',
        perSecond: '/s',
    },
} as const

/** The visible heading for each stable failure code. */
export const errorHeadings: Readonly<Record<TransferErrorCode, string>> = {
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
}

/** The exact `PublicError.message` table, re-exported rather than restated. */
export const errorMessages = fixedErrorMessages

/**
 * Fills the one placeholder in the QR accessible-name template.
 *
 * The replacement is a function on purpose. A string replacement makes `$&`,
 * `` $` ``, `$'` and `$$` inside the item name mean something to `replace`, so
 * a legitimately named file rewrites its own accessible name -- `` $` `` copies
 * the template back into it and `$'` deletes the rest. A function replacement
 * has no such syntax, so the name is inserted exactly as the backend sanitized
 * it.
 */
export function qrAltFor(itemName: string): string {
    return copy.qr.alt.replace('[item name]', () => itemName)
}

/**
 * `as const` stops a compiler from rewriting the registry; freezing stops a
 * running view from doing it. A component that reached in to "adjust" one
 * string would otherwise change it for every later render in the process.
 */
function freezeDeep(value: object): void {
    Object.freeze(value)
    for (const nested of Object.values(value)) {
        if (typeof nested === 'object' && nested !== null && !Object.isFrozen(nested)) freezeDeep(nested)
    }
}

freezeDeep(copy)
freezeDeep(errorHeadings)
