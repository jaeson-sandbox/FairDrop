import {publicError} from './errors'
import {parseLifecycleEvent} from './validation'
import type {
    CompletionReceipt,
    FileMetadata,
    LifecycleEvent,
    LifecycleEventName,
    PendingItemKind,
    ProgressSnapshot,
    PublicError,
    ReceiveMetadata,
    ReceiveResult,
    ReceiveStatus,
    RetainedOutcome,
} from './types'

export interface IdleTransferState {
    readonly phase: 'idle'
    readonly retainedOutcome: RetainedOutcome | null
    readonly commandError: PublicError | null
    /**
     * Present (and `'receive'`) only when `commandError` came from starting a
     * receive session, so the failure card can use receive wording ("Choose
     * Another Folder", not "Choose Another") instead of send wording. Absent
     * everywhere else, including every send state.
     */
    readonly commandContext?: 'receive'
}

export interface PendingTransferState {
    readonly phase: 'pending'
    readonly generation: number
    readonly itemKind: PendingItemKind
    readonly cancelPending: boolean
}

interface SessionCursor {
    readonly sessionId: string
    readonly lastSeq: number
}

export interface StagedTransferState {
    readonly phase: 'staged'
    readonly session: SessionCursor
    readonly metadata: FileMetadata
    readonly cancelPending: boolean
    readonly commandError: PublicError | null
    readonly recovered?: boolean
}

export interface TransferringTransferState {
    readonly phase: 'transferring'
    readonly session: SessionCursor
    readonly metadata: FileMetadata
    readonly progress: ProgressSnapshot | null
    readonly cancelPending: boolean
    readonly commandError: PublicError | null
}

export interface DoneTransferState {
    readonly phase: 'done'
    readonly session: SessionCursor
    readonly outcome: {
        readonly kind: 'done'
        /**
         * What was sent and how much -- never the full `FileMetadata`. That
         * type also carries the one-shot capability URL and its QR code,
         * which must not outlive the session (see `CompletionReceipt`).
         */
        readonly receipt: CompletionReceipt
    }
}

export interface ErrorTransferState {
    readonly phase: 'error'
    readonly session: SessionCursor
    readonly outcome: {
        readonly kind: 'error'
        readonly error: PublicError
        /**
         * The failed item's display name -- never the full `FileMetadata`,
         * which also carries the one-shot capability URL and its QR code (see
         * `RetainedErrorOutcome`, Story 9.2). Always populated at the two
         * transitions below, since both originate from a live session that
         * already has `metadata` in scope.
         */
        readonly itemName?: string
        readonly isCollection?: boolean
        readonly itemCount?: number
    }
}

/**
 * A receive session that is open and waiting for a phone.
 *
 * Receive has its own phases rather than a flag on the send ones. A send
 * `StagedTransferState` carries a `FileMetadata` and offers Send Again, retry
 * and the collection draft; a receive session has none of those, and the
 * cheapest way to be sure no send affordance can ever attach to it is for it to
 * be a different type that the send code never names.
 */
export interface ReceiveWaitingTransferState {
    readonly phase: 'receive-waiting'
    readonly session: SessionCursor
    readonly metadata: ReceiveMetadata
    readonly cancelPending: boolean
    readonly commandError: PublicError | null
    readonly recovered?: boolean
    /**
     * How many times a phone's upload was refused for space while waiting. The
     * session keeps waiting; this only exists so each refusal is a new state
     * and is therefore announced, rather than the second one going unheard.
     */
    readonly refusals: number
}

export interface RecoveringTransferState {
    readonly phase: 'recovering'
    readonly generation: number
    readonly session: SessionCursor
    readonly mode: 'send' | 'receive'
    readonly metadata: FileMetadata | ReceiveMetadata
    readonly retired: boolean
    readonly cancelPending: boolean
    readonly refusals: number
}

/** A receive session whose phone has started uploading. */
export interface ReceiveReceivingTransferState {
    readonly phase: 'receive-receiving'
    readonly session: SessionCursor
    readonly metadata: ReceiveMetadata
    /** Bytes read from the phone against the declared upload size; `null` until the first accepted snapshot. */
    readonly progress: ProgressSnapshot | null
    /** Files whose rename has succeeded so far -- the same count the outcome will state. */
    readonly filesSaved: number
    readonly cancelPending: boolean
    readonly commandError: PublicError | null
}

/**
 * What a receive session came to.
 *
 * Deliberately not the `ReceiveMetadata`: the capability URL and its QR code
 * are scrubbed the moment the session ends, exactly as a send receipt is
 * scrubbed of its own (`CompletionReceipt`). The destination's display name
 * stays so the outcome can say where the files went.
 */
export interface ReceiveOutcome {
    readonly kind: ReceiveResult
    readonly filesSaved: number
    readonly subfolderExists: boolean
    readonly markingWarning: boolean
}

/**
 * A finished receive session, held until the user leaves it.
 *
 * The backend does not reset a receive outcome on a timer -- Show in Folder
 * needs it to still exist -- so this phase has no automatic way out. The only
 * exit is the user's Done, which is the existing `CancelTransfer`.
 */
export interface ReceiveOutcomeTransferState {
    readonly phase: 'receive-outcome'
    readonly session: SessionCursor
    readonly destination: string
    readonly outcome: ReceiveOutcome
    /** A failed Show in Folder; the outcome itself is unaffected. */
    readonly commandError: PublicError | null
}

export type TransferState =
    | IdleTransferState
    | PendingTransferState
    | StagedTransferState
    | TransferringTransferState
    | DoneTransferState
    | ErrorTransferState
    | ReceiveWaitingTransferState
    | ReceiveReceivingTransferState
    | ReceiveOutcomeTransferState
    | RecoveringTransferState

export type TransferAction =
    | {readonly type: 'recovery-requested'; readonly generation: number; readonly sessionId: string}
    | {readonly type: 'recovery-succeeded'; readonly generation: number; readonly sessionId: string; readonly result: import('./types').ConnectionRefresh}
    | {readonly type: 'recovery-failed'; readonly generation: number; readonly sessionId: string; readonly retired: boolean; readonly error: PublicError}
    | {readonly type: 'stage-requested'; readonly generation: number; readonly itemKind: PendingItemKind}
    | {readonly type: 'stage-succeeded'; readonly generation: number; readonly metadata: FileMetadata}
    | {readonly type: 'receive-started'; readonly generation: number; readonly metadata: ReceiveMetadata}
    | {readonly type: 'stage-failed'; readonly generation: number; readonly error: PublicError}
    | {readonly type: 'invalid-selection'}
    | {readonly type: 'cancel-requested'}
    | {readonly type: 'pending-cancel-settled'; readonly generation: number; readonly error: PublicError | null}
    | {readonly type: 'active-cancel-failed'; readonly sessionId: string; readonly error: PublicError | null}
    | {
        readonly type: 'lifecycle'
        readonly eventName: LifecycleEventName
        readonly args: readonly unknown[]
    }
    | {readonly type: 'clipboard-failed'; readonly sessionId: string}
    | {readonly type: 'receive-show-requested'; readonly sessionId: string}
    | {readonly type: 'receive-show-failed'; readonly sessionId: string}
    | {readonly type: 'dismiss-retained'}

export function createInitialTransferState(): IdleTransferState {
    return {phase: 'idle', retainedOutcome: null, commandError: null}
}

/** Pure state machine for local command state plus the authoritative event grammar. */
export function transferReducer(state: TransferState, action: TransferAction): TransferState {
    switch (action.type) {
        case 'recovery-requested':
            if ((state.phase !== 'staged' && state.phase !== 'receive-waiting') ||
                state.cancelPending || state.session.sessionId !== action.sessionId) return state
            return {phase: 'recovering', generation: action.generation, session: state.session,
                mode: state.phase === 'staged' ? 'send' : 'receive', metadata: state.metadata,
                retired: false, cancelPending: false, refusals: state.phase === 'receive-waiting' ? state.refusals : 0}

        case 'recovery-succeeded':
            if (state.phase !== 'recovering' || state.generation !== action.generation ||
                state.session.sessionId !== action.sessionId || state.cancelPending || state.mode !== action.result.mode) return state
            if (action.result.mode === 'send') {
                const metadata = action.result.send
                if (metadata.sessionId === action.sessionId) return state
                return {phase: 'staged', session: {sessionId: metadata.sessionId, lastSeq: 0}, metadata,
                    cancelPending: false, commandError: null, recovered: true}
            }
            if (action.result.receive.sessionId === action.sessionId) return state
            return {phase: 'receive-waiting', session: {sessionId: action.result.receive.sessionId, lastSeq: 0},
                metadata: action.result.receive, cancelPending: false, commandError: null, refusals: 0, recovered: true}

        case 'recovery-failed':
            if (state.phase !== 'recovering' || state.generation !== action.generation ||
                state.session.sessionId !== action.sessionId) return state
            if (state.retired || action.retired) return idleWithError(action.error.code === 'cancelled' ? null : action.error, state.mode === 'receive')
            return state.mode === 'send'
                ? {phase: 'staged', session: state.session, metadata: state.metadata as FileMetadata,
                    cancelPending: false, commandError: action.error}
                : {phase: 'receive-waiting', session: state.session, metadata: state.metadata as ReceiveMetadata,
                    cancelPending: false, commandError: action.error, refusals: state.refusals}
        case 'stage-requested':
            if (state.phase !== 'idle' || !Number.isSafeInteger(action.generation) || action.generation <= 0) {
                return state
            }
            return {
                phase: 'pending',
                generation: action.generation,
                itemKind: action.itemKind,
                cancelPending: false,
            }

        case 'stage-succeeded': {
            // A pending receive is committed by `receive-started` and nothing
            // else: a send acknowledgement can never turn it into a Staged send.
            if (state.phase !== 'pending' || state.generation !== action.generation || state.cancelPending ||
                state.itemKind === 'receive') return state
            // Already parsed. The controller is the boundary the acknowledgement
            // crosses, and it refuses a malformed one by quiescing the backend
            // session and dispatching stage-failed -- so a second parse here
            // could only fail on a value the first accepted, and its answer
            // ("return state") was to leave the window in Pending with no
            // error and no announcement. It also repeated a base64 decode and
            // a full PNG chunk walk of up to 2 MB (Epic 1 retrospective item
            // 7). The type is the proof: nothing unparsed can reach this case.
            const {metadata} = action
            return {
                phase: 'staged',
                session: {sessionId: metadata.sessionId, lastSeq: 0},
                metadata,
                cancelPending: false,
                commandError: null,
            }
        }

        case 'receive-started': {
            // The receive twin of stage-succeeded: same generation and
            // cancel guards, and additionally only a pending *receive* may
            // become a waiting session, so a send acknowledgement can never
            // be committed as one by mistake.
            if (state.phase !== 'pending' || state.generation !== action.generation || state.cancelPending ||
                state.itemKind !== 'receive') return state
            const {metadata} = action
            return {
                phase: 'receive-waiting',
                session: {sessionId: metadata.sessionId, lastSeq: 0},
                metadata,
                cancelPending: false,
                commandError: null,
                refusals: 0,
            }
        }

        case 'stage-failed':
            if (state.phase !== 'pending' || state.generation !== action.generation || state.cancelPending) return state
            return idleWithError(action.error.code === 'cancelled' ? null : action.error, state.itemKind === 'receive')

        case 'invalid-selection':
            if (state.phase !== 'idle') return state
            return {
                phase: 'idle',
                retainedOutcome: state.retainedOutcome,
                commandError: publicError('invalid_selection'),
            }

        case 'cancel-requested':
            if (state.phase === 'recovering') return {...state, cancelPending: true}
            if (state.phase === 'pending') {
                return state.cancelPending ? state : {...state, cancelPending: true}
            }
            if (state.phase === 'staged' || state.phase === 'transferring' ||
                state.phase === 'receive-waiting' || state.phase === 'receive-receiving') {
                return state.cancelPending ? state : {...state, cancelPending: true, commandError: null}
            }
            return state

        case 'pending-cancel-settled':
            if (state.phase !== 'pending' || state.generation !== action.generation || !state.cancelPending) return state
            return idleWithError(
                action.error === null || action.error.code === 'cancelled' ? null : action.error,
                state.itemKind === 'receive',
            )

        case 'active-cancel-failed':
            if ((state.phase !== 'staged' && state.phase !== 'transferring' &&
                state.phase !== 'receive-waiting' && state.phase !== 'receive-receiving') ||
                state.session.sessionId !== action.sessionId || !state.cancelPending) {
                return state
            }
            return {
                ...state,
                cancelPending: false,
                commandError: action.error === null || action.error.code === 'cancelled'
                    ? null
                    : fixedCopy(action.error),
            }

        case 'lifecycle': {
            const event = parseLifecycleEvent(action.eventName, action.args)
            return event === null ? state : reduceLifecycle(state, event)
        }

        /*
          The one command a view issues on its own behalf.

          Story 3.11 gave the clipboard write a registry code and a heading and
          nothing that could reach them: the view discarded the rejection, so a
          copy that never happened looked exactly like one that did. This is the
          `invalid-selection` shape, scoped to a session rather than to a phase:
          the copy control exists only in Staged, and a rejection that arrives
          after its own session has been replaced belongs to nobody.

          `cancelPending` refuses it for the same reason `active-cancel-failed`
          requires it. A session being retired has already told the user what is
          happening, and a failed copy is not a reason to talk over that.
        */
        case 'clipboard-failed':
            if ((state.phase !== 'staged' && state.phase !== 'receive-waiting') ||
                state.session.sessionId !== action.sessionId || state.cancelPending) {
                return state
            }
            return {...state, commandError: publicError('clipboard_failed')}

        /*
          Show in Folder is the receive outcome's own command, and like the
          clipboard write it has nowhere else to put a rejection: the outcome
          itself is still true (files were saved), so a failed open is a note
          beside it, never a replacement for it. Requesting clears the previous
          note so a retry that works does not sit beside a stale failure.
        */
        case 'receive-show-requested':
            if (state.phase !== 'receive-outcome' || state.session.sessionId !== action.sessionId ||
                state.commandError === null) {
                return state
            }
            return {...state, commandError: null}

        case 'receive-show-failed':
            if (state.phase !== 'receive-outcome' || state.session.sessionId !== action.sessionId) return state
            return {...state, commandError: publicError('path_not_found')}

        /*
          Story 9.6: this action now clears whichever Idle-level outcome is
          currently showing -- a retained Done/Error, a Stage-time command
          failure, or (the rare dual case `invalid-selection` can produce,
          see "keeps retained terminal outcome when invalid selection
          supplies the visible command error" above) both at once -- rather
          than only a retained outcome. The one-card rebuild replaces the
          whole Idle composition with whichever of the two is showing, and
          every such card carries a working Dismiss (the AC's own words:
          "every error card also has Dismiss"), so this is the one action
          both "Done" (a retained/live Done outcome) and "Dismiss" (an Error
          outcome or a command failure) call. The action name is kept
          rather than renamed: it is still "dismiss whatever is retained at
          the Idle level," just no longer scoped to `retainedOutcome` alone.
        */
        case 'dismiss-retained':
            if (state.phase !== 'idle' || (state.retainedOutcome === null && state.commandError === null)) {
                return state
            }
            return createInitialTransferState()
    }
}

function reduceLifecycle(state: TransferState, event: LifecycleEvent): TransferState {
    if (!hasSession(state)) return state
    if (event.sessionId !== state.session.sessionId || event.seq <= state.session.lastSeq) return state

    const session = {sessionId: state.session.sessionId, lastSeq: event.seq}

    // Session kind is enforced here, once, for every send phase: an event that
    // carries a receive payload, or a receive-only notice, belongs to a receive
    // session and is never applied to a send one. Without this a confused or
    // hostile event could drive a send session into a state its views describe
    // as something else entirely.
    if (state.phase === 'staged' || state.phase === 'transferring' || state.phase === 'done' || state.phase === 'error') {
        if (event.kind === 'transfer-notice') return state
        if ((event.kind === 'transfer-progress' || event.kind === 'transfer-complete' || event.kind === 'transfer-error') &&
            event.receive !== undefined) return state
    }

    switch (state.phase) {
        case 'recovering':
            if (event.kind === 'transfer-reset') return {...state, session, retired: true}
            if (event.kind === 'transfer-started' && !state.retired) {
                return state.mode === 'send'
                    ? {phase: 'transferring', session, metadata: state.metadata as FileMetadata,
                        progress: null, cancelPending: false, commandError: null}
                    : {phase: 'receive-receiving', session, metadata: state.metadata as ReceiveMetadata,
                        progress: null, filesSaved: 0, cancelPending: false, commandError: null}
            }
            return state
        case 'receive-waiting':
            return reduceReceiveWaiting(state, event, session)

        case 'receive-receiving':
            return reduceReceiveReceiving(state, event, session)

        case 'receive-outcome':
            // Held until the user leaves it. Only the reset their Done causes
            // moves it, and it lands on plain Idle: a receive outcome is not
            // retained, so nothing downstream can offer Send Again for it.
            return event.kind === 'transfer-reset' ? createInitialTransferState() : state

        case 'staged':
            if (event.kind === 'transfer-started') {
                return {
                    phase: 'transferring',
                    session,
                    metadata: state.metadata,
                    progress: null,
                    cancelPending: state.cancelPending,
                    commandError: null,
                }
            }
            if (event.kind === 'transfer-reset') return createInitialTransferState()
            return state

        case 'transferring':
            if (event.kind === 'transfer-progress') {
                if (!progressMatchesMetadata(state.metadata, event.progress) ||
                    !progressCanFollow(state.progress, event.progress)) return state
                return {...state, session, progress: event.progress}
            }
            if (event.kind === 'transfer-complete') {
                // A terminal event ends the session even when its snapshot is
                // refused. Returning `state` here left the view in Transferring
                // until the backend's reset landed, and transferring -> plain
                // idle is the cancel-won row -- so a completed transfer was
                // announced as "Transfer canceled", strictly worse than the
                // downgrade the Go side refuses to make (Epic 1 retrospective
                // item 2). What the refusal costs is the claim of success, not
                // the end of the session: FairDrop cannot vouch for bytes it
                // could not reconcile, so it says the transfer did not finish.
                if (!progressMatchesMetadata(state.metadata, event.progress) ||
                    !progressCanFollow(state.progress, event.progress)) {
                    return {
                        phase: 'error',
                        session,
                        outcome: {kind: 'error', error: publicError('transfer_failed'), itemName: state.metadata.name, ...(state.metadata.isCollection ? {isCollection: true, itemCount: state.metadata.itemCount} : {})},
                    }
                }
                return {
                    phase: 'done',
                    session,
                    outcome: {
                        kind: 'done',
                        receipt: {
                            name: state.metadata.name,
                            isDir: state.metadata.isDir,
                            ...(state.metadata.isCollection ? {isCollection: true, itemCount: state.metadata.itemCount} : {}),
                            bytesSent: event.progress.bytesSent,
                        },
                    },
                }
            }
            if (event.kind === 'transfer-error') {
                // Same rule, easier answer: this event is already a failure, so
                // an incoherent snapshot beside it changes no claim. Dropping
                // the event instead would hide a failure behind a cancellation.
                return {
                    phase: 'error',
                    session,
                    outcome: {kind: 'error', error: fixedCopy(event.error), itemName: state.metadata.name, ...(state.metadata.isCollection ? {isCollection: true, itemCount: state.metadata.itemCount} : {})},
                }
            }
            if (event.kind === 'transfer-reset') return createInitialTransferState()
            return state

        case 'done':
            if (event.kind !== 'transfer-reset') return state
            return {
                phase: 'idle',
                retainedOutcome: {
                    kind: 'done',
                    receipt: state.outcome.receipt,
                },
                commandError: null,
            }

        case 'error':
            if (event.kind !== 'transfer-reset') return state
            return {
                phase: 'idle',
                retainedOutcome: {
                    kind: 'error',
                    error: fixedCopy(state.outcome.error),
                    itemName: state.outcome.itemName,
                    ...(state.outcome.isCollection ? {isCollection: true, itemCount: state.outcome.itemCount} : {}),
                },
                commandError: null,
            }
    }
}

function reduceReceiveWaiting(
    state: ReceiveWaitingTransferState,
    event: LifecycleEvent,
    session: SessionCursor,
): TransferState {
    switch (event.kind) {
        case 'transfer-started':
            return {
                phase: 'receive-receiving',
                session,
                metadata: state.metadata,
                progress: null,
                filesSaved: 0,
                cancelPending: state.cancelPending,
                commandError: null,
            }
        case 'transfer-notice':
            // The upload that was refused wrote nothing and the session is
            // still open, so this changes nothing but a count.
            return {...state, session, refusals: state.refusals + 1}
        case 'transfer-error': {
            // The one terminal event that CAN precede `started`: the backend ends
            // a receive nobody claimed when its server lane closes first (Go
            // TestALaneThatClosesBeforeAnyClaimFailsAWaitingReceive). Dropping it
            // left a QR for a listener that no longer exists. Only a payload that
            // is coherent with a session no phone claimed is applied -- an
            // incomplete result with nothing saved and no folder; anything else
            // is refused rather than guessed at.
            const receive = event.receive
            if (receive === undefined || receive.result !== 'incomplete' ||
                receive.filesSaved !== 0 || receive.subfolderExists) return state
            return receiveOutcomeState(state, session, receive, 'incomplete')
        }
        case 'transfer-reset':
            return createInitialTransferState()
        default:
            // A progress or complete event cannot precede `started` in a valid
            // grammar. Applying one would show a receive that never began.
            return state
    }
}

function reduceReceiveReceiving(
    state: ReceiveReceivingTransferState,
    event: LifecycleEvent,
    session: SessionCursor,
): TransferState {
    switch (event.kind) {
        case 'transfer-progress': {
            // A receive progress event always names its saved count, and a
            // receive total is always known (the declared upload size). One
            // that does not is not a receive progress event.
            if (event.receive === undefined || !event.progress.totalKnown) return state
            if (!progressCanFollow(state.progress, event.progress) || event.receive.filesSaved < state.filesSaved) return state
            return {...state, session, progress: event.progress, filesSaved: event.receive.filesSaved}
        }

        case 'transfer-complete': {
            if (event.receive === undefined) return state
            // FairDrop cannot vouch for bytes it could not reconcile, so a
            // success whose snapshot does not follow the progress already
            // accepted is demoted to incomplete -- never the other way round.
            const coherent = event.progress.totalKnown && progressCanFollow(state.progress, event.progress)
            return receiveOutcomeState(state, session, event.receive, coherent ? 'complete' : 'incomplete')
        }

        case 'transfer-error': {
            if (event.receive === undefined || event.receive.result === null) return state
            return receiveOutcomeState(state, session, event.receive, event.receive.result)
        }

        case 'transfer-reset':
            return createInitialTransferState()

        default:
            return state
    }
}

function receiveOutcomeState(
    state: ReceiveWaitingTransferState | ReceiveReceivingTransferState,
    session: SessionCursor,
    status: ReceiveStatus,
    kind: ReceiveResult,
): ReceiveOutcomeTransferState {
    return {
        phase: 'receive-outcome',
        session,
        destination: state.metadata.destination,
        outcome: {
            kind,
            filesSaved: status.filesSaved,
            subfolderExists: status.subfolderExists,
            markingWarning: status.markingWarning,
        },
        commandError: null,
    }
}

function hasSession(state: TransferState): state is Exclude<TransferState, IdleTransferState | PendingTransferState> {
    return state.phase === 'staged' || state.phase === 'transferring' || state.phase === 'done' || state.phase === 'error' ||
        state.phase === 'receive-waiting' || state.phase === 'receive-receiving' || state.phase === 'receive-outcome' || state.phase === 'recovering'
}

function progressCanFollow(previous: ProgressSnapshot | null, next: ProgressSnapshot): boolean {
    if (previous === null) return true
    return next.bytesSent >= previous.bytesSent &&
        next.totalKnown === previous.totalKnown &&
        next.totalBytes === previous.totalBytes
}

function progressMatchesMetadata(metadata: FileMetadata, progress: ProgressSnapshot): boolean {
    return metadata.isDir || metadata.isCollection
        ? !progress.totalKnown
        : progress.totalKnown && progress.totalBytes === metadata.size
}

function idleWithError(error: PublicError | null, receive = false): IdleTransferState {
    return {
        phase: 'idle',
        retainedOutcome: null,
        commandError: error === null ? null : fixedCopy(error),
        // Only an error that actually shows needs the context, and only a
        // receive attempt sets it, so every send state stays byte-identical to
        // what it was before receive existed.
        ...(receive && error !== null ? {commandContext: 'receive' as const} : {}),
    }
}

function fixedCopy(error: PublicError): PublicError {
    return publicError(error.code)
}
