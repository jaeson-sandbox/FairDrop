import type {CSSProperties} from 'react'
import {selectCommandError, selectReceiveProgress} from '../transfer/selectors'
import type {ReceiveReceivingTransferState} from '../transfer/state'
import {OutcomePanel} from './OutcomePanel'
import {ProgressRing} from './TransferringView'
import {FolderKindGlyph} from './ReceiveWaitingView'
import {copy} from './copy'
import {formatBytes, formatRate} from './format'

function rise(step: number): CSSProperties {
    return {'--fd-stagger': step} as CSSProperties
}

interface ReceivingViewProps {
    readonly state: ReceiveReceivingTransferState
    readonly onCancel: () => void
}

/**
 * Receive, a phone is uploading: bytes read against the size the phone
 * declared, and how many files have been saved so far.
 *
 * It borrows Sending's ring and figure styling because the job is the same --
 * show honest wire progress in the QR's own slot -- but reads three figures
 * where Sending reads two, and says "Received" and "Saved" rather than "Sent".
 * The total is always known for a receive (it is the declared upload size), so
 * a percentage is honest here in a way it is not for a folder ZIP; the ring is
 * still driven by the two authoritative integers, never by the wire percent.
 *
 * "Saved" counts only files whose rename has completed. The line beneath the
 * figures says so, because a file that is still arriving is not yet saved and a
 * desktop that let those two blur together would overstate what survives a
 * dropped connection.
 */
export function ReceivingView({state, onCancel}: ReceivingViewProps) {
    const progress = selectReceiveProgress(state)
    const commandError = selectCommandError(state)

    const received = progress !== null && progress.mode === 'known-positive'
        ? `${formatBytes(progress.bytesSent)} ${copy.label.of} ${formatBytes(progress.totalBytes)}`
        : formatBytes(progress?.bytesSent ?? 0)

    return (
        <div className="fd-region" data-phase-view="receive-receiving">
            <div className="fd-staged-head">
                <h1 className="fd-state-heading" tabIndex={-1} data-focus-target="receive-receiving-heading">
                    {copy.receive.receiving}
                </h1>
            </div>

            <section className="fd-packet">
                <div className="fd-hero">
                    <ProgressRing progress={progress} label={copy.receive.receiving}/>

                    <div className="fd-hero__details">
                        <div className="fd-item">
                            <span className="fd-item__icon" aria-hidden="true">
                                <FolderKindGlyph/>
                            </span>
                            <div className="fd-item__text">
                                <h2 className="fd-headline" id="fd-item-name">
                                    <bdi dir="auto">{state.metadata.destination}</bdi>
                                </h2>
                                <p className="fd-meta">{copy.receive.destinationNote}</p>
                            </div>
                        </div>

                        <div className="fd-rise" style={rise(1)}>
                            <div className="fd-metrics">
                                <div className="fd-metric" data-metric="received">
                                    <strong className="fd-metric__value">{received}</strong>
                                    <span className="fd-metric__caption">{copy.receive.receivedCaption}</span>
                                </div>
                                <div className="fd-metric" data-metric="saved">
                                    <strong className="fd-metric__value">{copy.receive.filesSaved(state.filesSaved)}</strong>
                                    <span className="fd-metric__caption">{copy.receive.savedCaption}</span>
                                </div>
                                {progress === null ? null : (
                                    <div className="fd-metric" data-metric="speed">
                                        <strong className="fd-metric__value">{formatRate(progress.speedBytesPerSec)}</strong>
                                        <span className="fd-metric__caption">{copy.label.speedCaption}</span>
                                    </div>
                                )}
                            </div>
                        </div>

                        <p className="fd-meta fd-rise" style={rise(2)}>{copy.receive.savedNote}</p>

                        <button
                            type="button"
                            className="fd-button fd-button--quiet fd-target fd-rise"
                            style={rise(3)}
                            aria-disabled={state.cancelPending || undefined}
                            onClick={() => {
                                if (!state.cancelPending) onCancel()
                            }}
                        >
                            {state.cancelPending ? copy.cancel.pending : copy.cancel.action}
                        </button>
                    </div>
                </div>
            </section>

            {commandError === null ? null : (
                <OutcomePanel
                    outcome={{kind: 'error', retained: false, error: commandError}}
                    focusTarget="command-error"
                />
            )}
        </div>
    )
}
