/**
 * The announcement-ownership routing table, in one place.
 *
 * EXPERIENCE.md gives every transition exactly one owner: either focus moves to
 * a state heading or panel, or the single pre-mounted atomic polite announcer
 * replaces its text. Never both. The failure this table exists to prevent is a
 * screen reader hearing one transition twice -- once from the focused heading
 * and once from a live region -- which is what happens when the decision is
 * spread across five views instead of written down once.
 *
 * `routeTransition` is a pure function of the previous and next reducer states,
 * so every row is testable without a DOM, and the App only has to obey the
 * answer: focus the named target if it exists, or write the named text.
 *
 * Two rows are deliberately absent from this function because they are not
 * reducer transitions: `copy-success` belongs to StagedView's own local state,
 * and `throttled-progress` belongs to `progressSpeech.ts`, which is throttled
 * on a clock rather than on a state change. Both still name their row here so
 * the table stays the one enumeration of owners.
 */

import type {IdleTransferState, TransferState} from '../transfer/state'
import type {PublicError} from '../transfer/types'
import {copy} from './copy'

/**
 * Every element the app is ever allowed to move focus to.
 *
 * A target is addressed by `data-focus-target` rather than by a class or a tag,
 * so the stylesheet, the views and this table cannot drift: the attribute is
 * the contract, and a view that stops carrying one makes the focus move a
 * proven no-op instead of a silent focus of the wrong node.
 */
export const focusTargets = [
    'idle-instruction',
    'cancel-summary',
    'command-error',
    'pending-heading',
    'staged-heading',
    'transferring-heading',
    'outcome',
] as const

export type FocusTarget = (typeof focusTargets)[number]

/** One row of the EXPERIENCE.md "Announcement ownership" table. */
export type TransitionRow =
    | 'stage-pending'
    | 'stage-success'
    | 'command-failure'
    | 'beacon-warning'
    | 'copy-success'
    | 'transfer-started'
    | 'throttled-progress'
    | 'cancel-requested'
    | 'cancel-won'
    | 'terminal-outcome'
    | 'dismiss-retained'

export interface FocusAnnouncement {
    readonly row: TransitionRow
    readonly owner: 'focus'
    readonly target: FocusTarget
}

export interface SpokenAnnouncement {
    readonly row: TransitionRow
    readonly owner: 'announcer'
    readonly text: string
}

export type Announcement = FocusAnnouncement | SpokenAnnouncement

/** The selector for a focus target, so the attribute name is written once. */
export function focusSelector(target: FocusTarget): string {
    return `[data-focus-target="${target}"]`
}

/**
 * The owner of one reducer transition, or `null` when the table assigns none.
 *
 * `null` is a real answer, not a gap: "Native dialog cancel" and "Reset after
 * terminal" are both rows whose owner is None, and a progress snapshot is
 * deliberately left to the throttle.
 */
export function routeTransition(previous: TransferState, next: TransferState): Announcement | null {
    if (previous === next) return null

    switch (next.phase) {
        case 'pending':
            return toPending(previous, next.cancelPending)

        case 'staged':
            return toStaged(previous, next.cancelPending, next.commandError, next.metadata.warnings.length)

        case 'transferring':
            return toTransferring(previous, next.cancelPending, next.commandError)

        case 'done':
            return focusRow('terminal-outcome', 'outcome')

        case 'error':
            // "Return to Idle; never render as Error" -- so a cancellation that
            // somehow arrives as a terminal error is the cancel-winning summary,
            // which is what App renders for it, and not an outcome panel that
            // will not be on the screen to receive focus.
            return next.outcome.error.code === 'cancelled'
                ? focusRow('cancel-won', 'cancel-summary')
                : focusRow('terminal-outcome', 'outcome')

        case 'idle':
            return toIdle(previous, next)

        default: {
            // The assignment is the real guard: `next` narrows to `never` only
            // while every phase above is routed, so adding one to the reducer
            // fails this build rather than reaching the return. The return is
            // the runtime half, for a state this build cannot name.
            const unrouted: never = next
            void unrouted
            return null
        }
    }
}

function toPending(previous: TransferState, cancelPending: boolean): Announcement | null {
    // Idle is the only phase the reducer will accept a Stage request from, so
    // anything else arriving here is a change within Pending itself.
    if (previous.phase !== 'pending') return focusRow('stage-pending', 'pending-heading')
    if (!previous.cancelPending && cancelPending) {
        return spokenRow('cancel-requested', copy.cancel.preparationPending)
    }
    return null
}

function toStaged(
    previous: TransferState,
    cancelPending: boolean,
    commandError: PublicError | null,
    warningCount: number,
): Announcement | null {
    if (previous.phase !== 'staged') return focusRow('stage-success', 'staged-heading')
    if (isNewCommandError(previous.commandError, commandError)) return focusRow('command-failure', 'command-error')
    if (!previous.cancelPending && cancelPending) return spokenRow('cancel-requested', copy.cancel.pending)

    // The warning is announced only when it appears at a session that is
    // already Staged, because "Keep focus in Staged" is the rule attached to
    // it. Arriving together with the metadata makes the transition Stage
    // success, whose owner is the focused heading, and one transition never
    // gets two owners.
    if (previous.metadata.warnings.length === 0 && warningCount > 0) {
        return spokenRow('beacon-warning', copy.discovery.warning)
    }
    return null
}

function toTransferring(
    previous: TransferState,
    cancelPending: boolean,
    commandError: PublicError | null,
): Announcement | null {
    if (previous.phase !== 'transferring') return focusRow('transfer-started', 'transferring-heading')
    if (isNewCommandError(previous.commandError, commandError)) return focusRow('command-failure', 'command-error')
    if (!previous.cancelPending && cancelPending) return spokenRow('cancel-requested', copy.cancel.pending)

    // A progress snapshot repaints the meter every time it arrives and speaks
    // on its own throttle. Owning it here would make every accepted snapshot an
    // announcement, which is the event log the spine forbids.
    return null
}

function toIdle(previous: TransferState, next: IdleTransferState): Announcement | null {
    // Reset after a terminal outcome moves no focus and says nothing: the same
    // visible node stays mounted, and focus already inside it stays there.
    if (previous.phase === 'done' || previous.phase === 'error') return null

    const previousError = previous.phase === 'idle' ? previous.commandError : null
    if (isNewCommandError(previousError, next.commandError)) return focusRow('command-failure', 'command-error')

    if (previous.phase === 'idle') {
        return previous.retainedOutcome !== null && next.retainedOutcome === null
            ? focusRow('dismiss-retained', 'idle-instruction')
            : null
    }

    // Pending, Staged or Transferring reached plain Idle without a terminal
    // event: the cancellation won the race, so the Idle summary owns it.
    return next.retainedOutcome === null ? focusRow('cancel-won', 'cancel-summary') : null
}

/**
 * A command failure is new when a panel that was not on screen now is.
 *
 * Identity, not equality: the reducer replaces the whole state object for every
 * accepted action, so a second failure carrying the same code is a different
 * value and is announced again. `cancelled` never counts, because
 * `selectCommandError` refuses to render it and focusing an absent panel is not
 * a transition anyone can hear.
 */
function isNewCommandError(previous: PublicError | null, next: PublicError | null): boolean {
    return next !== null && next !== previous && next.code !== 'cancelled'
}

function focusRow(row: TransitionRow, target: FocusTarget): FocusAnnouncement {
    return {row, owner: 'focus', target}
}

function spokenRow(row: TransitionRow, text: string): SpokenAnnouncement {
    return {row, owner: 'announcer', text}
}
