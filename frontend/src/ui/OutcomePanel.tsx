import type {OutcomePresentation} from '../transfer/selectors'
import type {FocusTarget} from './announce'
import {copy, errorHeadings, errorMessages} from './copy'

interface OutcomePanelProps {
    readonly outcome: OutcomePresentation
    /** Heading rank. The lifecycle outcome owns a document heading; nested panels do not. */
    readonly level?: 1 | 2
    /**
     * Whether this panel is the phase's own view.
     *
     * Separate from `level` because the retained node keeps the terminal
     * panel's heading rank -- reset must not change what the user is looking at
     * -- while the phase view moves to Idle underneath it.
     */
    readonly phaseView?: boolean
    /**
     * The routing-table target this panel answers to, when it is one.
     *
     * The lifecycle panel and a command-failure panel are different rows of the
     * table and carry different targets, which is what lets Idle show a
     * retained outcome and a fresh failure at once without the failure's focus
     * landing on the wrong one. The lifecycle panel keeps its target across a
     * reset even though no row targets it in Idle: focus is still sitting on
     * it, and removing the attribute would take its focus ring with it.
     */
    readonly focusTarget?: FocusTarget
    readonly onDismiss?: () => void
}

/**
 * The one Done/Error surface.
 *
 * The same component renders the live terminal phase, the identical node after
 * a matching reset has retained it in Idle, and a command or validation
 * failure. Reset is not allowed to change what the user is looking at, so the
 * only thing `retained` adds is Dismiss.
 *
 * The error text is read from the fixed registry by code, not from the error
 * value handed in. The reducer already replaces every incoming message with
 * registry copy; taking the code as the only input makes that structural, so a
 * message that somehow arrived from an adapter still cannot reach the screen.
 *
 * There is no `role="alert"` here, in any form. Every path that shows this
 * panel is a focus-owned row of the routing table, and the spine allows an
 * alert only on a path that does not also move focus.
 */
export function OutcomePanel({
    outcome,
    level = 2,
    phaseView = false,
    focusTarget,
    onDismiss,
}: OutcomePanelProps) {
    const done = outcome.kind === 'done'
    const Heading = level === 1 ? 'h1' : 'h2'
    // Focus lands on this section, so it needs a name of its own: a container
    // with a heading inside it is not named by that heading unless it says so.
    const headingId = `fd-outcome-heading-${outcome.kind}${outcome.retained ? '-retained' : ''}`

    return (
        <section
            className={`fd-outcome ${done ? 'fd-outcome--done' : 'fd-outcome--error'}`}
            data-phase-view={phaseView ? 'outcome' : undefined}
            data-outcome={outcome.kind}
            data-retained={String(outcome.retained)}
            data-error-code={done ? undefined : outcome.error.code}
            data-focus-target={focusTarget}
            aria-labelledby={headingId}
            tabIndex={-1}
        >
            <span className="fd-outcome__icon" aria-hidden="true">{done ? '✓' : '!'}</span>
            <Heading className="fd-state-heading" id={headingId}>
                {done ? copy.done.heading : errorHeadings[outcome.error.code]}
            </Heading>
            <p className="fd-outcome__body">
                {done ? copy.done.body : errorMessages[outcome.error.code]}
            </p>
            {outcome.retained && onDismiss !== undefined ? (
                <button type="button" className="fd-button fd-target" onClick={onDismiss}>
                    {copy.outcome.dismiss}
                </button>
            ) : null}
        </section>
    )
}
