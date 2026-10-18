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

/**
 * Wraps a folder name in Unicode first-strong isolates (U+2068 .. U+2069) so a
 * right-to-left or bidi-control-bearing name cannot reorder the sentence it sits
 * in. The name arrives already free of control characters (`parseReceiveMetadata`
 * refuses them), but bidi *format* characters are not controls, and plain
 * strings have no `<bdi>` to lean on.
 */
function isolate(name: string): string {
    return `\u2068${name}\u2069`
}

/** "1 file saved", "3 files saved", "no files saved": the one place singular and plural are decided. */
function savedCount(count: number): string {
    return count === 0 ? 'no files saved' : `${count} ${count === 1 ? 'file' : 'files'} saved`
}

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
    text: {
        open: 'Send text',
        intro: 'Paste or type a short note or link. The receiver chooses when to read it.',
        label: 'Text to send',
        limit: 'Up to 65,536 UTF-8 bytes.',
        blank: 'Enter some text to send.',
        tooLong: 'Text is too long (65,536 UTF-8 bytes maximum).',
        unsupported: 'Text contains an unsupported character.',
        selectionBlocked: 'Finish or cancel this text before selecting files.',
        stagedHeading: 'Text ready to send',
        stagedInstruction: 'Scan the code, then choose Read text in the receiving browser.',
        qrAlt: 'QR code for text',
        kind: 'Text',
        byteLength: (count: number) => `${count.toLocaleString('en-US')} UTF-8 bytes`,
        once: 'Works once. The first device to choose Read text receives it.',
        linkLabel: 'Text link',
        sendingHeading: 'Sending text',
        sendingBody: 'FairDrop is sending the text to the receiving browser.',
        sentHeading: 'Text sent',
        sentBody: 'The text was sent to the receiving browser.',
    },
    receiveText: {
        open: 'Receive text',
        pending: 'Preparing to receive text…',
        hint: 'Invite one nearby browser to send a note or link.',
        waitingHeading: 'Waiting for text',
        waitingBody: 'Scan the code on your phone, then send text from its browser.',
        linkLabel: 'Text receive link',
        once: 'Works once: this link accepts one text submission.',
        receivingHeading: 'Receiving text',
        receivingBody: 'FairDrop is receiving text from the browser.',
        receivedHeading: 'Text received',
        receivedBody: 'Inspect the text, then copy it when you are ready.',
        fieldLabel: 'Received text',
        copyAction: 'Copy text',
        copied: 'Copied',
        copyFailed: 'The text could not be copied. Try again.',
        loading: 'Loading received text…',
        fetchFailed: 'The received text could not be shown. Try again.',
        retry: 'Try again',
        failedHeading: 'Text not received',
        failedBody: 'The text did not finish arriving. Ask the sender to try again with a new code.',
    },
    firewall: {
        preflight: 'Your first transfer may ask to allow FairDrop on this local network.',
        windows: 'Allow FairDrop on Private networks only. Leave Public networks off.',
        macos: 'Allow incoming connections for FairDrop.',
        windowsRecovery: 'Open Windows Firewall settings and allow FairDrop on Private networks only, then prepare the item again.',
        macosRecovery: 'Open System Settings → Network → Firewall → Options, allow incoming connections for FairDrop, then prepare the item again.',
        windowsRecoveryWaiting: 'Open Windows Firewall settings and allow FairDrop on Private networks only, then use Refresh connection below for a new code.',
        macosRecoveryWaiting: 'Open System Settings → Network → Firewall → Options, allow incoming connections for FairDrop, then use Refresh connection below for a new code.',
    },
    stage: {
        pending: {
            file: 'Preparing your file…',
            folder: 'Preparing your folder…',
        item: 'Preparing your item…',
            collection: 'Preparing your items…',
            text: 'Preparing your text…',
        },
        heading: 'Ready to send',
    },
    qr: {
        instruction: 'Scan the code, then tap Download on the receiving device.',
        alt: 'Download QR code for [item name]',
    },
    folder: {
        note: 'This folder downloads as a ZIP.',
    },
    collection: {
        note: 'Downloads as ZIP',
    },
    selection: {
        open: 'Send multiple items',
        heading: 'Selected items',
        addFiles: 'Add Files',
        addFolder: 'Add Folder',
        send: 'Send',
        cancel: 'Cancel',
        remove: 'Remove',
        count: (count: number) => `${count.toLocaleString('en-US')} of 10,000 items`,
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
        warning: 'Works once: the first device to tap Download gets the item.',
        /**
         * Receiver handoff: explain the inspection step inside "Trouble
         * connecting?", beside the rest of the connection guidance.
         */
        inspect: 'Opening the link shows the item before download. Tap Download to receive it.',
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
        /**
         * Story 9.6: the Done card's own heading. `copy.done.body` is
         * removed -- the one-line receipt (kind glyph, name, wire bytes) now
         * carries what completion means, and the sentence that used to spell
         * it out is redundant beside it.
         */
        heading: 'Sent',
        sendAgain: 'Send Again',
        /** Story 9.6: the Done card's primary action -- the extracted BrowseControl, reused verbatim. */
        sendAnother: 'Send Another',
        /** Story 9.6: the Done card's own Dismiss, distinct wording from an Error card's. */
        dismiss: 'Done',
    },
    cancel: {
        preparation: 'Cancel preparation',
        preparationPending: 'Canceling preparation',
        action: 'Cancel',
        pending: 'Canceling',
        /**
         * Story 10.2: the cancel-winning notification's title and body,
         * replacing the single combined `copy.cancel.won` the static Idle
         * summary used to render. Split so the notification can bold the
         * title and mute the body as two lines (DESIGN.md's Notification
         * row); the combined sentence still exists, assembled from these two
         * strings, as the accessible description the Idle heading points at
         * while the notification is mounted (`Notification.tsx`).
         */
        wonTitle: 'Transfer canceled',
        wonBody: 'Ready for another file or folder.',
    },
    outcome: {
        dismiss: 'Dismiss',
        /** Story 9.6: the Error card's primary action when `selectEffectiveErrorAction` returns `retry`. */
        tryAgain: 'Try Again',
        /** Story 9.6: the Error card's primary action when `selectEffectiveErrorAction` returns `choose`. */
        chooseAnother: 'Choose Another',
    },
    /**
     * Receiving from a phone (spec-phone-to-desktop-receiving, Story B).
     *
     * Strings are tabulated in EXPERIENCE.md's Voice and Tone table like every
     * other approved string. The entries that are functions (a count, a folder
     * name) compose approved fragments rather than stand for one sentence, so
     * they are documented in the table's "Receive composed copy" subsection and
     * pinned literal-for-literal in `copy.test.ts` instead.
     *
     * "Saved" is said only for files whose rename has completed: the backend's
     * count is the only source for any number here, and no sentence claims more
     * than "saved to this computer".
     */
    receive: {
        open: 'Receive',
        chooseAnotherFolder: 'Choose Another Folder',
        hint: 'Get files from a phone on the same local network.',
        pending: 'Preparing to receive…',
        heading: 'Ready to receive',
        instruction: 'Scan the code with the phone’s camera, then choose the files to upload.',
        qrAlt: 'Upload QR code for [folder name]',
        destinationNote: 'Each upload is saved in a new folder here.',
        once: 'Works once: this link accepts one upload.',
        discoveryWarning: 'Device discovery isn’t available. The QR code and upload link still work.',
        tooLargeTitle: 'Upload too large',
        tooLarge: 'A phone tried to upload more than this computer has room for. FairDrop is still waiting. Choose fewer files on the phone, or cancel and pick a folder with more free space.',
        helpDifferentLan: 'Phone can’t open the page? Make sure both devices use the same local Wi-Fi. Guest or isolated networks may block device-to-device traffic. Use Refresh connection below for a new code.',
        receiving: 'Receiving',
        receivedCaption: 'Received',
        savedCaption: 'Saved',
        savedNote: 'A file counts as saved once it has finished.',
        markingWarning: 'FairDrop couldn’t add the download mark to some files on this drive. The files were saved without it.',
        showInFolder: 'Show in Folder',
        showFailed: 'FairDrop couldn’t open that folder. It may have been moved or renamed.',
        /** "1 file saved", "3 files saved", "no files saved". The one place singular and plural are decided. */
        filesSaved: savedCount,
        completeHeading: (count: number) => count === 0 ? 'No files saved' : savedCount(count),
        incompleteHeading: (count: number) => `Upload incomplete — ${savedCount(count)}`,
        cancelledHeading: (count: number) => `Upload cancelled — ${savedCount(count)}`,
        savedTo: (destination: string) => `Saved to this computer, in a new folder inside ${isolate(destination)}.`,
        stoppedKept: (destination: string) => `The upload stopped early. Files that finished are saved to this computer, in a new folder inside ${isolate(destination)}. The file in progress was removed.`,
        stoppedNone: (destination: string) => `The upload stopped before any file finished saving. Nothing was added to ${isolate(destination)}.`,
        cancelledKept: (destination: string) => `You cancelled the upload. Files that finished are saved to this computer, in a new folder inside ${isolate(destination)}. The file in progress was removed.`,
        cancelledNone: (destination: string) => `You cancelled the upload before any file finished saving. Nothing was added to ${isolate(destination)}.`,
    },
    help: {
        /**
         * Story 9.4: the "Trouble connecting?" disclosure's own summary --
         * the second thing Staged's foot row offers, beside Cancel.
         */
        heading: 'Trouble connecting?',
        differentLan: 'Not downloading? Make sure both devices use the same local Wi-Fi. Guest or isolated networks may block device-to-device traffic. Then cancel and prepare the item again for a fresh link.',
        differentLanWaiting: 'Not downloading? Make sure both devices use the same local Wi-Fi. Guest or isolated networks may block device-to-device traffic. Use Refresh connection below for a new code.',
        receiverHttp: 'Browser says Not Found: the link may be wrong or expired. Locked: another downloader claimed it. Gone: the selected item changed. Cancel and prepare the item again for a fresh link.',
        receiverHttpWaiting: 'Browser says Not Found: the link may be wrong or expired. Locked: another downloader claimed it. Gone: the selected item changed. If this code is still waiting, use Refresh connection below for a new code.',
    },
    recovery: {
        pendingHeading: 'Refreshing connection',
        pendingBody: 'The old code is being retired. Wait for a new code before sharing it.',
        guidance: 'Choose a local network and refresh the code. Both devices need the same local network. Listed networks have not been tested for firewall or guest-network access.',
        currentAddress: 'Current address:',
        networkLabel: 'Local network',
        automatic: 'Automatic',
        unavailableChoice: 'Selected network unavailable',
        loading: 'Loading networks…',
        loadError: 'Couldn’t load local networks. Try again.',
        empty: 'No eligible local networks found.',
        unavailable: 'That network is no longer available. Choose Automatic or another network.',
        reload: 'Reload networks',
        refresh: 'Refresh connection',
        success: 'New code ready. The old code no longer works.',
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
        items: 'Items',
        /** Item Summary: "sanitized bidi-isolated full name and logical size". */
        logicalSize: 'logical size',
        /** Direct URL Row heading, from the staged production reference. */
        directLinkHeading: 'Direct download link',
        /** The receive waiting view's link field name: the phone uploads here, it does not download. */
        uploadLinkHeading: 'Upload link',
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
    invalid_selection: 'Choose 1 to 10,000 items',
    invalid_text: 'Check your text',
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
 * Receive-specific wording for the stable failure codes a receive attempt can
 * produce, keyed by the same codes (no new code exists). The send copy for
 * these says "item", "send" and "prepare", which is wrong beside a folder
 * chooser, so a failure that came from Receive reads from here first and falls
 * back to the shared table only for the codes whose send wording is already
 * accurate (`busy`, `network_unavailable`, `server_start_failed`,
 * `clipboard_failed`). Messages are fixed registry strings like their siblings;
 * no value is ever interpolated, so a path or adapter text cannot reach one.
 */
export const receiveErrorCopy: Readonly<Partial<Record<TransferErrorCode, {
    readonly heading: string
    readonly message: string
}>>> = {
    invalid_selection: {
        heading: 'Choose a folder',
        message: 'Choose a folder to save the received files in.',
    },
    path_not_found: {
        heading: 'Folder not found',
        message: 'That folder is no longer available. Choose it again.',
    },
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
    shutting_down: {
        heading: 'FairDrop is closing',
        message: 'FairDrop is closing. Reopen it to start receiving.',
    },
    chooser_failed: {
        heading: 'Couldn’t open the chooser',
        message: 'FairDrop couldn’t open the folder chooser. Try again.',
    },
}

/** The heading a failure shows: receive wording when the failure came from Receive and has some. */
export function errorHeadingFor(code: TransferErrorCode, receive: boolean): string {
    return (receive ? receiveErrorCopy[code]?.heading : undefined) ?? errorHeadings[code]
}

/** The message a failure shows, under the same rule as `errorHeadingFor`. */
export function errorMessageFor(code: TransferErrorCode, receive: boolean): string {
    return (receive ? receiveErrorCopy[code]?.message : undefined) ?? errorMessages[code]
}

/** Fills the one placeholder in the receive QR accessible-name template; see `qrAltFor` for why the replacement is a function. */
export function receiveQrAltFor(folderName: string): string {
    return copy.receive.qrAlt.replace('[folder name]', () => folderName)
}

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
freezeDeep(receiveErrorCopy)
