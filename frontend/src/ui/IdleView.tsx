import type {CSSProperties} from 'react'
import {selectCommandError, selectOutcome} from '../transfer/selectors'
import type {IdleTransferState} from '../transfer/state'
import {OutcomePanel} from './OutcomePanel'
import {copy} from './copy'

interface IdleViewProps {
    readonly state: IdleTransferState
    /** The inherited Wails drop gate, owned by App so the boundary stays in one place. */
    readonly dropTargetStyle: CSSProperties
    readonly onSelectFile: () => void
    readonly onSelectDirectory: () => void
    readonly onDismissRetained: () => void
}

/**
 * Idle: firewall preflight, the native drop target, and the two browse
 * controls, in that document order.
 *
 * A retained terminal outcome sits above them when one survives a reset. It is
 * current status, not history: there is exactly one, it carries Dismiss, and
 * nothing here removes it on a timer.
 */
export function IdleView({
    state,
    dropTargetStyle,
    onSelectFile,
    onSelectDirectory,
    onDismissRetained,
}: IdleViewProps) {
    const retained = selectOutcome(state)
    const commandError = selectCommandError(state)

    return (
        <div className="fd-region" data-phase-view="idle">
            {retained === null ? null : <OutcomePanel outcome={retained} onDismiss={onDismissRetained}/>}
            {commandError === null ? null : (
                <OutcomePanel outcome={{kind: 'error', retained: false, error: commandError}}/>
            )}

            <section className="fd-idle">
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

                <div className="fd-drop-zone" style={dropTargetStyle}>
                    <div>
                        <div className="fd-drop-symbol" aria-hidden="true">↓</div>
                        <h1 className="fd-state-heading" tabIndex={-1}>{copy.idle.instruction}</h1>
                    </div>
                </div>

                <div className="fd-selection">
                    <button type="button" className="fd-button fd-target" onClick={onSelectFile}>
                        {copy.label.selectFile}
                    </button>
                    <button type="button" className="fd-button fd-target" onClick={onSelectDirectory}>
                        {copy.label.selectDirectory}
                    </button>
                </div>
            </section>
        </div>
    )
}
