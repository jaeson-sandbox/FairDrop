import {publicError} from './errors'
import {parseFileMetadata, parseLifecycleEvent} from './validation'
import type {
    FileMetadata,
    LifecycleEvent,
    LifecycleEventName,
    PendingItemKind,
    ProgressSnapshot,
    PublicError,
    RetainedOutcome,
} from './types'

export interface IdleTransferState {
    readonly phase: 'idle'
    readonly retainedOutcome: RetainedOutcome | null
    readonly commandError: PublicError | null
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
    readonly outcome: {readonly kind: 'done'}
}

export interface ErrorTransferState {
    readonly phase: 'error'
    readonly session: SessionCursor
    readonly outcome: {readonly kind: 'error'; readonly error: PublicError}
}

export type TransferState =
    | IdleTransferState
    | PendingTransferState
    | StagedTransferState
    | TransferringTransferState
    | DoneTransferState
    | ErrorTransferState

export type TransferAction =
    | {readonly type: 'stage-requested'; readonly generation: number; readonly itemKind: PendingItemKind}
    | {readonly type: 'stage-succeeded'; readonly generation: number; readonly metadata: unknown}
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
    | {readonly type: 'dismiss-retained'}

export function createInitialTransferState(): IdleTransferState {
    return {phase: 'idle', retainedOutcome: null, commandError: null}
}

/** Pure state machine for local command state plus the authoritative event grammar. */
export function transferReducer(state: TransferState, action: TransferAction): TransferState {
    switch (action.type) {
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
            if (state.phase !== 'pending' || state.generation !== action.generation || state.cancelPending) return state
            const metadata = parseFileMetadata(action.metadata)
            if (metadata === null) return state
            return {
                phase: 'staged',
                session: {sessionId: metadata.sessionId, lastSeq: 0},
                metadata,
                cancelPending: false,
                commandError: null,
            }
        }

        case 'stage-failed':
            if (state.phase !== 'pending' || state.generation !== action.generation || state.cancelPending) return state
            return idleWithError(action.error.code === 'cancelled' ? null : action.error)

        case 'invalid-selection':
            if (state.phase !== 'idle') return state
            return {...state, commandError: publicError('invalid_selection')}

        case 'cancel-requested':
            if (state.phase === 'pending') {
                return state.cancelPending ? state : {...state, cancelPending: true}
            }
            if (state.phase === 'staged' || state.phase === 'transferring') {
                return state.cancelPending ? state : {...state, cancelPending: true, commandError: null}
            }
            return state

        case 'pending-cancel-settled':
            if (state.phase !== 'pending' || state.generation !== action.generation || !state.cancelPending) return state
            return idleWithError(action.error === null || action.error.code === 'cancelled' ? null : action.error)

        case 'active-cancel-failed':
            if ((state.phase !== 'staged' && state.phase !== 'transferring') ||
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

        case 'dismiss-retained':
            if (state.phase !== 'idle' || state.retainedOutcome === null) return state
            return createInitialTransferState()
    }
}

function reduceLifecycle(state: TransferState, event: LifecycleEvent): TransferState {
    if (!hasSession(state)) return state
    if (event.sessionId !== state.session.sessionId || event.seq <= state.session.lastSeq) return state

    const session = {sessionId: state.session.sessionId, lastSeq: event.seq}

    switch (state.phase) {
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
                if (!progressMatchesMetadata(state.metadata, event.progress) ||
                    !progressCanFollow(state.progress, event.progress)) return state
                return {phase: 'done', session, outcome: {kind: 'done'}}
            }
            if (event.kind === 'transfer-error') {
                if (event.progress !== null && (!progressMatchesMetadata(state.metadata, event.progress) ||
                    !progressCanFollow(state.progress, event.progress))) return state
                return {
                    phase: 'error',
                    session,
                    outcome: {kind: 'error', error: fixedCopy(event.error)},
                }
            }
            if (event.kind === 'transfer-reset') return createInitialTransferState()
            return state

        case 'done':
            if (event.kind !== 'transfer-reset') return state
            return {
                phase: 'idle',
                retainedOutcome: {kind: 'done'},
                commandError: null,
            }

        case 'error':
            if (event.kind !== 'transfer-reset') return state
            return {
                phase: 'idle',
                retainedOutcome: {kind: 'error', error: fixedCopy(state.outcome.error)},
                commandError: null,
            }
    }
}

function hasSession(state: TransferState): state is Exclude<TransferState, IdleTransferState | PendingTransferState> {
    return state.phase === 'staged' || state.phase === 'transferring' || state.phase === 'done' || state.phase === 'error'
}

function progressCanFollow(previous: ProgressSnapshot | null, next: ProgressSnapshot): boolean {
    if (previous === null) return true
    return next.bytesSent >= previous.bytesSent &&
        next.totalKnown === previous.totalKnown &&
        next.totalBytes === previous.totalBytes
}

function progressMatchesMetadata(metadata: FileMetadata, progress: ProgressSnapshot): boolean {
    return metadata.isDir
        ? !progress.totalKnown
        : progress.totalKnown && progress.totalBytes === metadata.size
}

function idleWithError(error: PublicError | null): IdleTransferState {
    return {
        phase: 'idle',
        retainedOutcome: null,
        commandError: error === null ? null : fixedCopy(error),
    }
}

function fixedCopy(error: PublicError): PublicError {
    return publicError(error.code)
}
