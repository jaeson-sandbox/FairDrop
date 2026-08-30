import type {OutcomePresentation} from '../transfer/selectors'
import {copy, errorHeadings, errorMessages} from './copy'

interface OutcomePanelProps {
    readonly outcome: OutcomePresentation
    /** Heading rank. Terminal phases own the document heading; nested panels do not. */
    readonly level?: 1 | 2
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
 */
export function OutcomePanel({outcome, level = 2, onDismiss}: OutcomePanelProps) {
    const done = outcome.kind === 'done'
    const Heading = level === 1 ? 'h1' : 'h2'

    return (
        <section
            className={`fd-outcome ${done ? 'fd-outcome--done' : 'fd-outcome--error'}`}
            data-phase-view={level === 1 ? 'outcome' : undefined}
            data-outcome={outcome.kind}
            data-retained={String(outcome.retained)}
            data-error-code={done ? undefined : outcome.error.code}
            tabIndex={-1}
        >
            <span className="fd-outcome__icon" aria-hidden="true">{done ? '✓' : '!'}</span>
            <Heading className="fd-state-heading">
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
