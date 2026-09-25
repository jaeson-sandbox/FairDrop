import {useCallback, useEffect, useReducer, useRef} from 'react'
import {CancelTransfer, SelectDirectory, SelectFile, StageTransfer} from '../../wailsjs/go/main/App'
import {EventsOn} from '../../wailsjs/runtime/runtime'
import {parseCommandError, publicError} from './errors'
import {selectErrorAction} from './selectors'
import {createInitialTransferState, transferReducer, type TransferState} from './state'
import {parseFileMetadata} from './validation'
import type {LifecycleEventName, PendingItemKind, PublicError} from './types'

interface StageOperation {
    readonly generation: number
    readonly promise: Promise<unknown>
    cancelRequested: boolean
}

interface ActiveCancelOperation {
    readonly generation: number
    readonly sessionId: string
}

interface BrowseOperation {
    readonly generation: number
}

export interface TransferController {
    readonly state: TransferState
    readonly stage: (absolutePath: string, itemKind?: PendingItemKind) => Promise<void>
    readonly selectFile: () => Promise<void>
    readonly selectDirectory: () => Promise<void>
    readonly cancel: () => Promise<void>
    readonly rejectSelection: () => void
    /** Reported by the staged view, which is the only thing that issues the command. */
    readonly reportCopyFailure: (sessionId: string) => void
    readonly dismissRetained: () => void
    /**
     * Stages a new item, releasing a live terminal outcome's backend lease
     * first if one is still showing (Story 9.2 AC4, D-059). Story 9.6 calls
     * this for "Send Another", "Choose Another", and a drop on a live
     * Done/Error card -- anywhere a fresh Stage might land while the ~3s
     * post-terminal lease is still open. `retry()` below is this primitive
     * plus "use the item remembered from the last Stage" and "only when the
     * current error's action is retry".
     */
    readonly stageFromOutcome: (absolutePath: string, itemKind?: PendingItemKind) => Promise<void>
    /**
     * Re-stages the item remembered from the last Stage, through the ordinary
     * `stage()` path, when the current outcome's or Stage-time command
     * failure's action is `retry` (Story 9.2 AC3). A no-op otherwise --
     * including when nothing is remembered -- so wiring it to a control
     * cannot misfire; the control's own visibility should still be decided
     * with `canRetry` and `selectErrorAction`/`selectEffectiveErrorAction`.
     */
    readonly retry: () => Promise<void>
    /**
     * Whether `retry()` currently has a remembered path to act on.
     *
     * `selectErrorAction` is a pure function of an error code and cannot know
     * this -- the remembered path lives in this controller's own memory, is
     * never part of `TransferState`, and is never exposed as a value (only as
     * this boolean), so nothing downstream can render or log it. Pass this to
     * `selectEffectiveErrorAction` to get the action a control should
     * actually offer.
     */
    readonly canRetry: boolean
}

/** Owns the one local command generation and the five session event listeners. */
export function useTransfer(): TransferController {
    const [state, dispatch] = useReducer(transferReducer, undefined, createInitialTransferState)
    const stateRef = useRef<TransferState>(state)
    const mountedRef = useRef(false)
    const stageGenerationRef = useRef(0)
    const stageOperationRef = useRef<StageOperation | null>(null)
    const cancelGenerationRef = useRef(0)
    const activeCancelRef = useRef<ActiveCancelOperation | null>(null)
    const browseOperationRef = useRef<BrowseOperation | null>(null)
    const subscriptionEpochRef = useRef(0)
    // The absolute path passed to the most recent StageTransfer -- JS memory
    // only (Story 9.2 AC1). Never localStorage, never a Go call beyond the
    // ordinary StageTransfer a retry issues, never logged, never rendered:
    // it is written only in `stage()` below and read only by `retry()`: no
    // selector, view, or serialized state ever sees it, only `canRetry`'s
    // boolean. See `stage()`, `dispatchStageFailed`, and `dismissRetained`
    // for where it is replaced or cleared.
    const rememberedPathRef = useRef<string | null>(null)
    const previousPhaseRef = useRef<TransferState['phase']>(state.phase)
    const idleWaitersRef = useRef<Set<() => void>>(new Set())
    stateRef.current = state

    // A completed Done needs no retry target: the item was sent, and the next
    // thing the sender does is a fresh Stage, which remembers its own path
    // anyway. This lives here rather than in the reducer because the
    // remembered path is controller memory, not reducer state (see
    // `rememberedPathRef` above); mirrors the existing `stateRef.current =
    // state` line just above, which is likewise a synchronous, idempotent
    // ref sync safe to run twice under StrictMode's double render.
    if (state.phase === 'done' && previousPhaseRef.current !== 'done') {
        rememberedPathRef.current = null
    }
    previousPhaseRef.current = state.phase

    useEffect(() => {
        mountedRef.current = true
        return () => {
            mountedRef.current = false
            const stageOperation = stageOperationRef.current
            if (stageOperation !== null && !stageOperation.cancelRequested) {
                stageOperation.cancelRequested = true
                void Promise.resolve().then(() => CancelTransfer()).catch(() => undefined)
            }
            stageGenerationRef.current += 1
            cancelGenerationRef.current += 1
            stageOperationRef.current = null
            activeCancelRef.current = null
            browseOperationRef.current = null
            rememberedPathRef.current = null
            idleWaitersRef.current = new Set()
        }
    }, [])

    // The other half of `stageFromOutcome`'s lease release: `cancel()` only
    // awaits the CancelTransfer command settling, not the backend's later
    // transfer-reset event that actually retires the session. Anything
    // waiting for Idle (a waiter added by `waitForIdle`) is resolved once the
    // reducer actually reaches it, whichever lifecycle event gets it there.
    useEffect(() => {
        if (state.phase !== 'idle' || idleWaitersRef.current.size === 0) return
        const waiters = idleWaitersRef.current
        idleWaitersRef.current = new Set()
        for (const resolve of waiters) resolve()
    }, [state])

    useEffect(() => {
        const epoch = subscriptionEpochRef.current + 1
        subscriptionEpochRef.current = epoch

        const subscribe = (eventName: LifecycleEventName) => EventsOn(eventName, (...args: unknown[]) => {
            if (subscriptionEpochRef.current !== epoch || !mountedRef.current) return
            dispatch({type: 'lifecycle', eventName, args})
        })

        // Keep these literals visible at the production call sites. A generated
        // or shared list could drift together with a self-referential test.
        const disposers: Array<() => void> = []
        try {
            disposers.push(subscribe('transfer-started'))
            disposers.push(subscribe('transfer-progress'))
            disposers.push(subscribe('transfer-complete'))
            disposers.push(subscribe('transfer-error'))
            disposers.push(subscribe('transfer-reset'))
        } catch (error) {
            if (subscriptionEpochRef.current === epoch) subscriptionEpochRef.current = epoch + 1
            disposeAll(disposers)
            throw error
        }

        return () => {
            if (subscriptionEpochRef.current === epoch) subscriptionEpochRef.current = epoch + 1
            disposeAll(disposers)
        }
    }, [])

    const stage = useCallback(async (
        absolutePath: string,
        itemKind: PendingItemKind = 'unknown',
    ): Promise<void> => {
        if (!mountedRef.current || stateRef.current.phase !== 'idle') return
        if (stageOperationRef.current !== null || browseOperationRef.current !== null) return

        const generation = stageGenerationRef.current + 1
        stageGenerationRef.current = generation
        // Remembered unconditionally, before the command even resolves: every
        // real Stage attempt -- native drop, either chooser, or a retry --
        // funnels through this one function, so this is the single place
        // "the absolute path passed to StageTransfer" needs recording (Story
        // 9.2 AC1). Replaced by the next Stage; cleared elsewhere on Dismiss,
        // a completed Done, and a non-retryable Stage failure.
        rememberedPathRef.current = absolutePath
        const promise = Promise.resolve().then(() => StageTransfer(absolutePath) as Promise<unknown>)
        const operation: StageOperation = {generation, promise, cancelRequested: false}
        stageOperationRef.current = operation
        dispatch({type: 'stage-requested', generation, itemKind})

        try {
            const rawMetadata = await promise
            if (!stageMayCommit(operation)) return

            const metadata = parseFileMetadata(rawMetadata)
            if (metadata === null) {
                operation.cancelRequested = true
                // A malformed acknowledgement may still represent a live
                // backend session. Quiesce it once before showing the fallback.
                let quiesced = true
                try {
                    await CancelTransfer()
                } catch {
                    // Best effort on the call, but not on the report (D-106).
                    // No rejection text from cleanup is trusted -- it is adapter
                    // text -- yet whether the cleanup worked changes what is
                    // true for the user, so the outcome is kept even though the
                    // reason is discarded.
                    quiesced = false
                }
                if (mountedRef.current && stageOperationRef.current === operation) {
                    // The command itself resolved -- no lifecycle event was ever
                    // received -- so nothing was sent; the selection was refused,
                    // not interrupted (D-053).
                    //
                    // But "nothing was sent" is only half the story when the
                    // cleanup above failed: the backend had already committed a
                    // staged session with a listener and a capability URL, and
                    // the next Stage would be refused busy for a session the
                    // user was just told did not exist. That is a different
                    // sentence, and cleanup_unconfirmed is the one that says it.
                    dispatchStageFailed(generation, publicError(quiesced ? 'setup_failed' : 'cleanup_unconfirmed'))
                }
                if (stageOperationRef.current === operation) stageOperationRef.current = null
                return
            }

            dispatch({type: 'stage-succeeded', generation, metadata})
        } catch (rejection) {
            if (!stageMayCommit(operation)) return
            dispatchStageFailed(generation, parseCommandError(rejection))
        } finally {
            if (stageOperationRef.current === operation && !operation.cancelRequested) {
                stageOperationRef.current = null
            }
        }
    }, [])

    /**
     * Runs one native chooser and hands its result to the same Stage path a
     * native drop uses.
     *
     * The dialog is not a Stage, so it holds its own slot rather than the Stage
     * one: an outstanding chooser blocks a second chooser and a drop, but it
     * never leaves a `CancelTransfer` owed for a session that was never staged.
     * Three results are possible and each is spelled out below.
     */
    const browse = useCallback(async (
        open: () => Promise<string>,
        itemKind: PendingItemKind,
    ): Promise<void> => {
        if (!mountedRef.current || stateRef.current.phase !== 'idle') return
        if (stageOperationRef.current !== null || browseOperationRef.current !== null) return

        const generation = stageGenerationRef.current + 1
        stageGenerationRef.current = generation
        const operation: BrowseOperation = {generation}
        browseOperationRef.current = operation

        try {
            const selected = await Promise.resolve().then(open)
            if (!browseMayCommit(operation)) return

            // A dismissed chooser returns an empty selection, which the spine
            // makes a quiet cancel: no dispatch, no error, no announcement.
            if (typeof selected !== 'string' || selected.trim() === '') return

            browseOperationRef.current = null
            await stage(selected, itemKind)
        } catch (rejection) {
            if (!browseMayCommit(operation)) return
            // The success path is guarded by stage()'s own idle check; this one
            // has to make it itself, or a chooser that failed while a drop was
            // being staged would report against someone else's session.
            if (stateRef.current.phase !== 'idle') return

            // The chooser failed, so no Stage was ever attempted -- yet the
            // reducer is the only owner of a visible command error, and its one
            // route into Idle runs through Pending. Both halves are therefore
            // dispatched together: React applies them in a single batch, so the
            // Pending state is reduced but never rendered and no view can claim
            // a preparation that never started.
            dispatch({type: 'stage-requested', generation, itemKind})
            dispatchStageFailed(generation, parseCommandError(rejection))
        } finally {
            if (browseOperationRef.current === operation) browseOperationRef.current = null
        }
    }, [stage])

    const selectFile = useCallback(() => browse(SelectFile, 'file'), [browse])
    const selectDirectory = useCallback(() => browse(SelectDirectory, 'directory'), [browse])

    const cancel = useCallback(async (): Promise<void> => {
        const current = stateRef.current
        if (current.phase === 'pending') {
            const operation = stageOperationRef.current
            if (operation === null || operation.cancelRequested || current.cancelPending) return

            operation.cancelRequested = true
            dispatch({type: 'cancel-requested'})

            const cancelPromise = Promise.resolve().then(() => CancelTransfer())
            const [stageResult, cancelResult] = await Promise.allSettled([operation.promise, cancelPromise])
            if (!mountedRef.current || stageOperationRef.current !== operation) return

            const error = firstNonCancellationError(cancelResult, stageResult)
            stageOperationRef.current = null
            dispatch({type: 'pending-cancel-settled', generation: operation.generation, error})
            return
        }

        // A live Done or Error is cancellable too, and this is the only way out
        // of one when the backend's three-second reset never arrives (D-059).
        // App wires that outcome's control to this function precisely for that
        // case, and until 2026-09-14 the guard below sent it straight back: the
        // control rendered, did nothing, and left the window exactly as
        // stranded as having no control at all.
        //
        // No cancel-requested dispatch, because a terminal state carries no
        // cancelPending to show and the reducer ignores it there anyway. The
        // transition comes from the backend's own transfer-reset, which is what
        // retires the session and returns the UI to Idle with the outcome
        // retained.
        if (current.phase === 'done' || current.phase === 'error') {
            const outstanding = activeCancelRef.current
            if (outstanding !== null && outstanding.sessionId === current.session.sessionId) return

            const terminalGeneration = cancelGenerationRef.current + 1
            cancelGenerationRef.current = terminalGeneration
            const terminal: ActiveCancelOperation = {
                generation: terminalGeneration,
                sessionId: current.session.sessionId,
            }
            activeCancelRef.current = terminal
            try {
                await CancelTransfer()
            } catch {
                // Nowhere to put it: a terminal state has no commandError field,
                // and the reset this was trying to force is what would have
                // produced the Idle that could show one. The rejection text is
                // adapter text either way, and the outcome panel the user is
                // looking at is still correct about what happened.
            } finally {
                if (activeCancelRef.current === terminal) activeCancelRef.current = null
            }
            return
        }

        if (current.phase !== 'staged' && current.phase !== 'transferring') return
        if (current.cancelPending) return
        const outstandingCancel = activeCancelRef.current
        if (outstandingCancel !== null && outstandingCancel.sessionId === current.session.sessionId) return

        const generation = cancelGenerationRef.current + 1
        cancelGenerationRef.current = generation
        const operation: ActiveCancelOperation = {generation, sessionId: current.session.sessionId}
        activeCancelRef.current = operation
        dispatch({type: 'cancel-requested'})

        try {
            await CancelTransfer()
        } catch (rejection) {
            if (activeCancelMayReport(operation)) {
                dispatch({
                    type: 'active-cancel-failed',
                    sessionId: operation.sessionId,
                    error: parseCommandError(rejection),
                })
            }
        } finally {
            if (activeCancelRef.current === operation) activeCancelRef.current = null
        }
    }, [])

    const rejectSelection = useCallback(() => dispatch({type: 'invalid-selection'}), [])
    const dismissRetained = useCallback(() => {
        // Story 9.2 AC1: Dismiss is one of the three explicit clearing
        // triggers. Dispatched unconditionally -- if the reducer finds
        // nothing to dismiss (already idle with no retained outcome) it
        // simply returns the same state, and clearing an already-null ref a
        // second time is a no-op.
        rememberedPathRef.current = null
        dispatch({type: 'dismiss-retained'})
    }, [])
    const reportCopyFailure = useCallback((sessionId: string) => {
        dispatch({type: 'clipboard-failed', sessionId})
    }, [])

    const stageFromOutcome = useCallback(async (
        absolutePath: string,
        itemKind: PendingItemKind = 'unknown',
    ): Promise<void> => {
        if (stateRef.current.phase === 'done' || stateRef.current.phase === 'error') {
            // D-059: a live terminal outcome still holds the backend's ~3s
            // lease until its own transfer-reset arrives. Staging straight
            // over it would surface `busy` for a session the sender already
            // considers finished, so the lease is released first, exactly the
            // way Dismiss releases it today (`cancel()`) -- and this waits for
            // the actual transition to Idle, not just for the Cancel command
            // to settle: the reset event that retires the session is a
            // separate, later message from the backend.
            await cancel()
            await waitForIdle()
        }
        if (!mountedRef.current) return
        await stage(absolutePath, itemKind)
    }, [cancel, stage])

    const retry = useCallback(async (): Promise<void> => {
        const path = rememberedPathRef.current
        if (path === null) return
        const error = currentRetryableError(stateRef.current)
        if (error === null || selectErrorAction(error.code) !== 'retry') return
        await stageFromOutcome(path)
    }, [stageFromOutcome])

    return {
        state, stage, selectFile, selectDirectory, cancel, rejectSelection, reportCopyFailure, dismissRetained,
        stageFromOutcome, retry, canRetry: rememberedPathRef.current !== null,
    }

    function waitForIdle(): Promise<void> {
        if (stateRef.current.phase === 'idle') return Promise.resolve()
        return new Promise<void>((resolve) => { idleWaitersRef.current.add(resolve) })
    }

    /**
     * Dispatches a Stage failure and applies Story 9.2 AC1's third clearing
     * rule: a code whose action is not `retry` means retrying this exact path
     * would not help -- the sender needs to choose again, or the app needs to
     * be reopened -- so nothing is left remembered to retry with. Routes every
     * `stage-failed` dispatch in this file, including the chooser-failure one
     * in `browse()`, so the rule holds regardless of which command failed.
     */
    function dispatchStageFailed(generation: number, error: PublicError): void {
        if (selectErrorAction(error.code) !== 'retry') rememberedPathRef.current = null
        dispatch({type: 'stage-failed', generation, error})
    }

    function browseMayCommit(operation: BrowseOperation): boolean {
        return mountedRef.current && browseOperationRef.current === operation
    }

    function stageMayCommit(operation: StageOperation): boolean {
        return mountedRef.current && stageOperationRef.current === operation && !operation.cancelRequested
    }

    function activeCancelMayReport(operation: ActiveCancelOperation): boolean {
        if (!mountedRef.current || activeCancelRef.current !== operation ||
            cancelGenerationRef.current !== operation.generation) return false

        const latest = stateRef.current
        return (latest.phase === 'staged' || latest.phase === 'transferring') &&
            latest.session.sessionId === operation.sessionId
    }
}

function disposeAll(disposers: readonly (() => void)[]): void {
    for (const dispose of disposers) {
        try {
            dispose()
        } catch {
            // Listener cleanup is best effort, but one broken disposer must not
            // prevent the remaining Wails listeners from being removed.
        }
    }
}

function firstNonCancellationError(
    primary: PromiseSettledResult<unknown>,
    secondary: PromiseSettledResult<unknown>,
): PublicError | null {
    for (const result of [primary, secondary]) {
        if (result.status !== 'rejected') continue
        const error = parseCommandError(result.reason)
        if (error.code !== 'cancelled') return error
    }
    return null
}

/**
 * The error `retry()` should act on: a live terminal Error, an Error retained
 * in Idle, or an Idle Stage-time command failure -- whichever is currently
 * showing. A live or retained outcome takes precedence over a Stage-time
 * command failure when (rarely) both are present at once, since the outcome
 * is the older, still-unresolved failure.
 *
 * Deliberately excludes `staged`/`transferring`'s own `commandError` (for
 * example a failed Cancel, or `clipboard_failed`): those are failures of a
 * command issued *during* a live session, not a reason to abandon and
 * re-stage the item that session is still holding.
 */
function currentRetryableError(state: TransferState): PublicError | null {
    if (state.phase === 'error') return state.outcome.error
    if (state.phase !== 'idle') return null
    if (state.retainedOutcome !== null && state.retainedOutcome.kind === 'error') return state.retainedOutcome.error
    return state.commandError
}
