import type {ReceiveOutcomeTransferState} from '../transfer/state'
import {OutcomeIcon, rise} from './OutcomePanel'
import {copy} from './copy'

interface ReceiveOutcomeViewProps {
    readonly state: ReceiveOutcomeTransferState
    /** Opens the subfolder in the OS file manager. Only offered when the subfolder exists. */
    readonly onShowInFolder: () => void
    /** Leaves the outcome: the existing Cancel command, which resets to Idle. */
    readonly onDone: () => void
}

/**
 * How a receive session ended, held until the user leaves it.
 *
 * Four outcomes share one card because they answer one question -- what is on
 * this computer now? -- and differ in how they answer it:
 *
 *  - complete: "N files saved".
 *  - incomplete: "Upload incomplete -- N files saved", and "no files saved"
 *    when nothing finished.
 *  - cancelled: "Upload cancelled -- N files saved".
 *
 * The number is the backend's own, read from the destination after it closed,
 * so it cannot disagree with the phone's result page. Nothing here derives it
 * and nothing rounds it up: an incomplete or cancelled upload is never worded
 * as a success, and the success heading exists only for the `complete` result.
 *
 * **Show in Folder** is rendered only when the subfolder exists. That is the
 * backend's report of the filesystem, not an inference from the count, so an
 * upload that saved nothing (its subfolder was removed) is never offered a
 * button that could only fail. The path itself is not here and never was: the
 * command opens it from Go.
 *
 * The card is a focus target for the whole outcome (`receive-outcome`), with
 * the heading as its name and the body as its description, so the single focus
 * move announces the result and what it means in one reading.
 */
export function ReceiveOutcomeView({state, onShowInFolder, onDone}: ReceiveOutcomeViewProps) {
    const {outcome, destination, commandError} = state
    const complete = outcome.kind === 'complete'
    const saved = outcome.filesSaved > 0

    const heading = complete
        ? copy.receive.completeHeading(outcome.filesSaved)
        : outcome.kind === 'incomplete'
            ? copy.receive.incompleteHeading(outcome.filesSaved)
            : copy.receive.cancelledHeading(outcome.filesSaved)

    const body = complete
        ? copy.receive.savedTo(destination)
        : outcome.kind === 'incomplete'
            ? (saved ? copy.receive.stoppedKept(destination) : copy.receive.stoppedNone(destination))
            : (saved ? copy.receive.cancelledKept(destination) : copy.receive.cancelledNone(destination))

    const headingId = 'fd-receive-outcome-heading'
    const bodyId = 'fd-receive-outcome-body'
    const markingId = 'fd-receive-outcome-marking'
    const describedBy = [bodyId, ...(outcome.markingWarning ? [markingId] : [])].join(' ')

    let step = 0
    const headingStep = step++
    const bodyStep = step++
    const markingStep = outcome.markingWarning ? step++ : null
    const actionsStep = step++

    return (
        <section
            className={`fd-outcome ${complete ? 'fd-outcome--done' : 'fd-outcome--error'}`}
            data-phase-view="receive-outcome"
            data-receive-outcome={outcome.kind}
            data-focus-target="receive-outcome"
            aria-labelledby={headingId}
            aria-describedby={describedBy}
            tabIndex={-1}
        >
            <OutcomeIcon done={complete}/>
            <h1 className="fd-state-heading fd-rise" id={headingId} style={rise(headingStep)}>{heading}</h1>
            <p className="fd-outcome__body fd-rise" id={bodyId} style={rise(bodyStep)}>{body}</p>
            {markingStep === null ? null : (
                <p
                    className="fd-outcome__body fd-rise"
                    id={markingId}
                    data-receive-marking-warning
                    style={rise(markingStep)}
                >
                    {copy.receive.markingWarning}
                </p>
            )}
            {commandError === null ? null : (
                // A failed Show in Folder is a command failure, which the routing
                // table focuses. The outcome above is still true, so this sits
                // beside it rather than replacing it.
                <p
                    className="fd-outcome__body"
                    tabIndex={-1}
                    data-focus-target="command-error"
                    data-receive-show-failed
                >
                    {copy.receive.showFailed}
                </p>
            )}
            <div className="fd-outcome__actions fd-rise" style={rise(actionsStep)}>
                {outcome.subfolderExists ? (
                    <button
                        type="button"
                        className="fd-button fd-button--primary fd-button--pill fd-target"
                        data-receive-show
                        onClick={onShowInFolder}
                    >
                        {copy.receive.showInFolder}
                    </button>
                ) : null}
                <button
                    type="button"
                    className={`fd-button fd-button--pill fd-target ${outcome.subfolderExists ? 'fd-button--quiet' : 'fd-button--primary'}`}
                    data-receive-done
                    onClick={onDone}
                >
                    {copy.done.dismiss}
                </button>
            </div>
        </section>
    )
}
