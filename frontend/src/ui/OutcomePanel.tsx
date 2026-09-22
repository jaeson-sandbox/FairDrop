import {useEffect, useState} from 'react'
import type {OutcomePresentation} from '../transfer/selectors'
import type {CompletionReceipt as CompletionReceiptData} from '../transfer/types'
import type {FocusTarget} from './announce'
import {copy, errorHeadings, errorMessages} from './copy'
import {formatBytes} from './format'

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
 *
 * Every outcome carries the control, not only a retained one. A live Done or
 * Error used to render a heading, a message and nothing else, because the way
 * out was the backend's three-second reset -- which meant a reset that never
 * arrived left the window with no way forward at all, and the coordinator
 * drops an event it cannot deliver (D-059). The caller decides what the
 * control does; this only guarantees one is offered whenever it is given.
 *
 * DESIGN.md's Done/Error rows both describe "the primary next action and a
 * quiet Dismiss", but only one handler ever reaches this component
 * (`onDismiss`): a live terminal outcome has nothing else on screen to be a
 * next action, and a retained one already sits above Idle's own browse
 * control, which *is* the next action. So the single control this panel owns
 * plays both parts -- primary weight, same label, when it is the only way
 * forward; quiet weight, "Dismiss", once Idle's control exists beneath it.
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
            <OutcomeIcon done={done}/>
            <Heading className="fd-state-heading" id={headingId}>
                {done ? copy.done.heading : errorHeadings[outcome.error.code]}
            </Heading>
            <p className="fd-outcome__body">
                {done ? copy.done.body : errorMessages[outcome.error.code]}
            </p>
            {done ? <CompletionReceipt receipt={outcome.receipt}/> : null}
            {onDismiss === undefined ? null : (
                <button
                    type="button"
                    className={
                        outcome.retained
                            ? 'fd-button fd-button--quiet fd-target'
                            : 'fd-button fd-button--primary fd-target'
                    }
                    onClick={onDismiss}
                >
                    {copy.outcome.dismiss}
                </button>
            )}
        </section>
    )
}

/**
 * The Done/Error glyph, painted inside a 74px tinted disc.
 *
 * The check is a stroke-drawn path, not a text glyph: `pathLength={32}`
 * normalizes the path's own length to exactly 32 units regardless of its
 * geometry, so `stroke-dasharray: 32` / `stroke-dashoffset: 32` in style.css
 * always describes "fully hidden" and `stroke-dashoffset: 0` always describes
 * "fully drawn", independent of the coordinates chosen here.
 *
 * The draw is a CSS *transition*, never `@keyframes`/`animation` --
 * `styles.test.ts` refuses either anywhere in the sheet -- triggered by adding
 * a class once after mount. That happens unconditionally, on every mount,
 * regardless of `prefers-reduced-motion`: gating it on a reduced-motion check
 * would leave the class never added and the check permanently undrawn, which
 * is exactly the meaning-carrying state `prefers-reduced-motion` is not
 * allowed to remove. What reduced motion changes instead is speed, through
 * the sheet's existing universal `transition-duration: 1ms !important` rule --
 * the same seam every other transition in the product already relies on.
 */
function OutcomeIcon({done}: {readonly done: boolean}) {
    const [drawn, setDrawn] = useState(false)

    useEffect(() => {
        if (!done) return
        setDrawn(true)
    }, [done])

    return (
        <span
            className={`fd-outcome__icon ${done ? 'fd-outcome__icon--done' : 'fd-outcome__icon--error'}`}
            aria-hidden="true"
        >
            {done ? (
                <svg
                    className={`fd-outcome__check${drawn ? ' fd-outcome__check--drawn' : ''}`}
                    viewBox="0 0 24 24"
                    aria-hidden="true"
                    focusable="false"
                >
                    <path className="fd-outcome__check-path" d="M5 12.5l4.3 4.3L19 7.5" pathLength={32}/>
                </svg>
            ) : '!'}
        </span>
    )
}

/**
 * Two cells, not three. DESIGN.md's Completion Receipt row is explicit: the
 * item name and the wire bytes actually sent, and nothing else -- there is no
 * elapsed-time cell because no clock is tracked anywhere in the frontend and
 * EXPERIENCE.md forbids a frontend lifecycle timer. Both figures come from
 * `CompletionReceipt`, Story 7.4's retained state, never from `FileMetadata`:
 * `receipt.bytesSent` is the wire count the terminal `ProgressSnapshot`
 * reported, and it must never be read as `metadata.size` here or anywhere
 * else that could re-derive it.
 */
function CompletionReceipt({receipt}: {readonly receipt: CompletionReceiptData}) {
    return (
        <dl className="fd-receipt">
            <div className="fd-receipt__cell">
                <dt className="fd-receipt__caption">{receipt.isDir ? copy.label.folder : copy.label.file}</dt>
                <dd className="fd-receipt__value"><bdi dir="auto">{receipt.name}</bdi></dd>
            </div>
            <div className="fd-receipt__cell">
                <dt className="fd-receipt__caption">{copy.label.wireBytes}</dt>
                <dd className="fd-receipt__value">{formatBytes(receipt.bytesSent)}</dd>
            </div>
        </dl>
    )
}
