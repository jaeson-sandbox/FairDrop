import {selectCommandError, selectProgress, type ProgressSelection} from '../transfer/selectors'
import type {TransferringTransferState} from '../transfer/state'
import {OutcomePanel} from './OutcomePanel'
import {copy} from './copy'
import {formatBytes, formatRate} from './format'

interface TransferringViewProps {
    readonly state: TransferringTransferState
    readonly onCancel: () => void
}

/**
 * Transferring: the packet identity stays, the QR and link give way to honest
 * progress.
 *
 * The three presentations come from `ProgressSelection.mode`, which the reducer
 * derives from the wire snapshot's own known/unknown discriminator. Nothing
 * here divides, and nothing recomputes a percentage: a folder never gets a
 * percentage from its logical size, and an empty file never gets a 0% bar.
 *
 * Until the first accepted snapshot arrives there is no meter at all. Choosing
 * one from the metadata would be inferring a mode the backend has not reported.
 */
export function TransferringView({state, onCancel}: TransferringViewProps) {
    const {metadata} = state
    const progress = selectProgress(state)
    const commandError = selectCommandError(state)

    return (
        <div className="fd-region" data-phase-view="transferring">
            <h1 className="fd-state-heading" tabIndex={-1}>{copy.label.sending}</h1>

            <div>
                <span className="fd-packet-tab">{metadata.isDir ? copy.label.folder : copy.label.file}</span>
                <section className="fd-transfer-view">
                    <div>
                        <h2 className="fd-headline"><bdi dir="auto">{metadata.name}</bdi></h2>
                        {metadata.isDir ? <p className="fd-subheading">{copy.folder.note}</p> : null}
                    </div>

                    {progress === null ? null : (
                        <>
                            <ProgressPresentation progress={progress}/>
                            <TransferMetrics progress={progress}/>
                        </>
                    )}

                    <button type="button" className="fd-button fd-button--quiet fd-target" onClick={onCancel}>
                        {state.cancelPending ? copy.cancel.pending : copy.cancel.action}
                    </button>
                </section>
            </div>

            {commandError === null ? null : (
                <OutcomePanel outcome={{kind: 'error', retained: false, error: commandError}}/>
            )}
        </div>
    )
}

function ProgressPresentation({progress}: {readonly progress: ProgressSelection}) {
    if (progress.mode === 'known-empty') {
        // No percentage-bearing progressbar exists for a known-empty payload.
        // The literal status carries the state; the track is decoration only.
        return (
            <div data-progress-mode="known-empty">
                <p className="fd-empty-status">{copy.progress.knownEmpty}</p>
                <div className="fd-meter" aria-hidden="true"/>
            </div>
        )
    }

    if (progress.mode === 'unknown') {
        return (
            <div data-progress-mode="unknown">
                <div id="fd-meter-label" className="fd-meter-label">
                    <span>{copy.progress.unknown}</span>
                </div>
                <div
                    className="fd-meter fd-meter--unknown"
                    role="progressbar"
                    aria-labelledby="fd-meter-label"
                />
            </div>
        )
    }

    const percent = Math.round(progress.value)
    return (
        <div data-progress-mode="known-positive">
            <div id="fd-meter-label" className="fd-meter-label">
                <span>
                    {`${formatBytes(progress.bytesSent)} ${copy.label.of} ${formatBytes(progress.totalBytes)}`}
                </span>
                <span>{`${percent}%`}</span>
            </div>
            <div
                className="fd-meter"
                role="progressbar"
                aria-labelledby="fd-meter-label"
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={percent}
            >
                <span className="fd-meter__fill" style={{width: `${progress.value}%`}}/>
            </div>
        </div>
    )
}

function TransferMetrics({progress}: {readonly progress: ProgressSelection}) {
    return (
        <div className="fd-metrics">
            <div className="fd-metric">
                <strong className="fd-metric__value">
                    {`${formatBytes(progress.bytesSent)} ${copy.label.sent}`}
                </strong>
                <span className="fd-metric__caption">{copy.label.wireBytes}</span>
            </div>
            {progress.mode === 'known-empty' ? null : (
                <div className="fd-metric">
                    <strong className="fd-metric__value">{formatRate(progress.speedBytesPerSec)}</strong>
                    <span className="fd-metric__caption">{copy.label.throughput}</span>
                </div>
            )}
        </div>
    )
}
