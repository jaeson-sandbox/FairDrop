import type {CSSProperties} from 'react'
import {selectCommandError} from '../transfer/selectors'
import type {IdleTransferState} from '../transfer/state'
import {OutcomePanel} from './OutcomePanel'
import {RecoveryHelp} from './RecoveryHelp'
import {copy} from './copy'

interface IdleViewProps {
    readonly state: IdleTransferState
    /** The inherited Wails drop gate, owned by App so the boundary stays in one place. */
    readonly dropTargetStyle: CSSProperties
    /**
     * Whether this Idle was reached by a cancellation winning its race.
     *
     * It cannot be read from the state: a cancel-winning reset lands on plain
     * Idle, which is also how the app starts. App owns the transition, so App
     * owns this flag, and the reducer keeps its two retained-outcome kinds.
     */
    readonly cancelWon: boolean
    readonly onSelectFile: () => void
    readonly onSelectDirectory: () => void
}

/**
 * Idle: the drop target, the cancellation summary, a command failure, the
 * firewall preflight, the two browse controls, and recovery help, in that
 * document order.
 *
 * The drop instruction leads because it is this region's `h1`. The spine's one
 * binding ordering rule is that firewall guidance precedes the selection
 * controls, which it does.
 *
 * A retained terminal outcome is not rendered here. App owns it, above this
 * region, so that reset keeps the identical DOM node rather than rebuilding one
 * that merely says the same thing.
 */
export function IdleView({
    state,
    dropTargetStyle,
    cancelWon,
    onSelectFile,
    onSelectDirectory,
}: IdleViewProps) {
    const commandError = selectCommandError(state)

    return (
        <div className="fd-region" data-phase-view="idle">
            <section className="fd-idle">
                {/*
                  The cancel-winning summary, first in the region.

                  It leads because it is the answer to what just happened, and
                  because a retained Done or Error already renders above this
                  view from the shell -- an outcome that appeared under the
                  selection controls was the odd one out.

                  Warning, not error. The spine's rule for `cancelled` is
                  "return to Idle; never render as Error", and `--color-error`
                  is the error language here: it appears on nothing but the
                  Error Panel. Amber says "this stopped" without calling a
                  deliberate action a failure. It is a focus target and nothing
                  else -- no live region, because focus owns this transition.
                */}
                {cancelWon ? (
                    <div
                        className="fd-cancel-summary"
                        tabIndex={-1}
                        data-focus-target="cancel-summary"
                    >
                        <span className="fd-cancel-summary__icon" aria-hidden="true">&times;</span>
                        <p className="fd-cancel-summary__text">{copy.cancel.won}</p>
                    </div>
                ) : null}

                {/*
                  Clicking the target opens the file chooser -- the same command
                  the Select File control below runs. It is a pointer shortcut,
                  not a control: the zone stays out of the tab order because the
                  two browse buttons are already the keyboard path to both
                  choosers, and a third tab stop reaching only one of them would
                  be worse than none. The native drop gate is untouched.
                */}
                <div
                    className="fd-drop-zone"
                    style={dropTargetStyle}
                    onClick={onSelectFile}
                >
                    <div>
                        <div className="fd-drop-symbol" aria-hidden="true">↓</div>
                        <h1
                            className="fd-state-heading"
                            tabIndex={-1}
                            data-focus-target="idle-instruction"
                        >
                            {copy.idle.instruction}
                        </h1>
                        <p className="fd-meta">{copy.external.promise}</p>
                    </div>
                </div>

                {commandError === null ? null : (
                    <OutcomePanel
                        outcome={{kind: 'error', retained: false, error: commandError}}
                        focusTarget="command-error"
                    />
                )}

                <aside className="fd-preflight" aria-labelledby="fd-firewall-heading">
                    <h2 id="fd-firewall-heading" className="fd-preflight__heading">
                        {copy.label.firewallHeading}
                    </h2>
                    <p className="fd-body">{copy.firewall.preflight}</p>
                    <dl>
                        <div>
                            <dt>{copy.label.windows}</dt>
                            <dd>{copy.firewall.windows}</dd>
                        </div>
                        <div>
                            <dt>{copy.label.macos}</dt>
                            <dd>{copy.firewall.macos}</dd>
                        </div>
                    </dl>
                </aside>

                <div className="fd-selection">
                    <button type="button" className="fd-button fd-target" onClick={onSelectFile}>
                        {copy.label.selectFile}
                    </button>
                    <button type="button" className="fd-button fd-target" onClick={onSelectDirectory}>
                        {copy.label.selectDirectory}
                    </button>
                </div>

                <RecoveryHelp/>
            </section>
        </div>
    )
}
