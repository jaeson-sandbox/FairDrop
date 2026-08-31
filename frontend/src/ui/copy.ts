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
 * Error text is not restated here at all: `errorMessages` re-exports the frozen
 * table the reducer already validates against, so the two cannot drift apart.
 */

import {fixedErrorMessages, type TransferErrorCode} from '../transfer/errors'

export const copy = {
    external: {
        promise: 'Send from FairDrop on Windows or Mac to one browser on the same local network—no account or receiver app.',
    },
    idle: {
        instruction: 'Drop one file or folder.',
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
        },
        heading: 'Ready to pass along',
    },
    qr: {
        instruction: 'Scan this code on the receiving device to start the download.',
        alt: 'Download QR code for [item name]',
    },
    folder: {
        note: 'This folder downloads as a ZIP.',
    },
    directLink: {
        action: 'Copy download link',
        helper: 'Open this link directly in the receiving device’s browser.',
    },
    firstOpener: {
        warning: 'One device only—the first device or software to open this link starts the download. Link previews may use this V1 link before the intended browser.',
    },
    network: {
        disclosure: 'Use FairDrop only on a network you trust. The transfer is not encrypted, so someone monitoring this network may be able to observe it.',
    },
    localCopy: {
        disclosure: 'Sent directly over your local network. FairDrop does not upload or store an extra copy. The receiving device keeps the downloaded file.',
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
        preparationPending: 'Canceling preparation…',
        action: 'Cancel',
        pending: 'Canceling…',
        won: 'Transfer canceled. Ready for another file or folder.',
    },
    outcome: {
        dismiss: 'Dismiss',
    },
    name: {
        showFull: 'Show full name',
    },
    help: {
        differentLan: 'Not downloading? Make sure both devices use the same local Wi-Fi. Guest or isolated networks may block device-to-device traffic. Then cancel and prepare the item again for a fresh link.',
        receiverHttp: 'Browser says Not Found: the link may be wrong or expired. Locked: another opener claimed it. Gone: the selected item changed. Cancel and prepare the item again for a fresh link.',
    },

    /** Functional words the spine names in prose rather than in the copy table. */
    label: {
        /** Information Architecture: "Native drop target, Select File, Select Directory". */
        selectFile: 'Select File',
        selectDirectory: 'Select Directory',
        /** "Firewall Preflight and Recovery" bullet labels, in document order. */
        firewallHeading: 'Local network access',
        windows: 'Windows',
        macos: 'macOS',
        /** The approved item vocabulary: file, folder. */
        file: 'File',
        folder: 'Folder',
        /** Item Summary: "sanitized bidi-isolated full name and logical size". */
        logicalSize: 'logical size',
        /** Direct URL Row heading, from the staged production reference. */
        directLinkHeading: 'Direct download link',
        /** Transferring state heading, from the transferring production reference. */
        sending: 'Sending',
        /** Transfer Metrics: wire bytes first, throughput second. */
        wireBytes: 'Wire bytes',
        throughput: 'Throughput',
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
