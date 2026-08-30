import {useCallback, useEffect, useReducer, useRef} from 'react'
import {CancelTransfer, SelectDirectory, SelectFile, StageTransfer} from '../../wailsjs/go/main/App'
import {EventsOn} from '../../wailsjs/runtime/runtime'
import {parseCommandError, publicError} from './errors'
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
    readonly dismissRetained: () => void
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
    stateRef.current = state

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
        }
    }, [])

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
                try {
                    await CancelTransfer()
                } catch {
                    // Best effort: no rejection text from cleanup is trusted.
                }
                if (mountedRef.current && stageOperationRef.current === operation) {
                    dispatch({type: 'stage-failed', generation, error: publicError('transfer_failed')})
                }
                if (stageOperationRef.current === operation) stageOperationRef.current = null
                return
            }

            dispatch({type: 'stage-succeeded', generation, metadata})
        } catch (rejection) {
            if (!stageMayCommit(operation)) return
            dispatch({type: 'stage-failed', generation, error: parseCommandError(rejection)})
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

            // The chooser failed, so no Stage was ever attempted -- yet the
            // reducer is the only owner of a visible command error, and its one
            // route into Idle runs through Pending. Both halves are therefore
            // dispatched together: React applies them in a single batch, so the
            // Pending state is reduced but never rendered and no view can claim
            // a preparation that never started.
            dispatch({type: 'stage-requested', generation, itemKind})
            dispatch({type: 'stage-failed', generation, error: parseCommandError(rejection)})
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
    const dismissRetained = useCallback(() => dispatch({type: 'dismiss-retained'}), [])

    return {state, stage, selectFile, selectDirectory, cancel, rejectSelection, dismissRetained}

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
