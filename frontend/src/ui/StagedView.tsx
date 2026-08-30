import {useState} from 'react'
import {selectCommandError, selectWarnings} from '../transfer/selectors'
import type {StagedTransferState} from '../transfer/state'
import {OutcomePanel} from './OutcomePanel'
import {copy, errorHeadings, qrAltFor} from './copy'
import {formatBytes} from './format'

interface StagedViewProps {
    readonly state: StagedTransferState
    readonly onCancel: () => void
}

/**
 * Staged: the QR is the handoff, the direct URL is the fallback.
 *
 * The capability token reaches the DOM exactly twice and both places are
 * inert: encoded inside the QR bitmap, and as the readonly text of the URL
 * row. There is no anchor, because a sender-side activation link would let this
 * window consume its own one-shot download.
 *
 * `data:image/png;base64,` is prepended here, at render, and nowhere else. The
 * reducer holds the bare base64 payload the backend produced.
 */
export function StagedView({state, onCancel}: StagedViewProps) {
    const {metadata} = state
    const warnings = selectWarnings(state)
    const commandError = selectCommandError(state)
    const [copied, setCopied] = useState(false)

    const size = metadata.isDir
        ? `${formatBytes(metadata.size)} ${copy.label.logicalSize}`
        : formatBytes(metadata.size)

    /**
     * A clipboard write that never happened is never reported as one: the label
     * changes only on a resolved promise. Wails exposes no clipboard helper, so
     * a browser that withholds the API leaves the action exactly as it was.
     */
    const handleCopy = () => {
        try {
            const clipboard = navigator.clipboard
            if (clipboard === undefined || clipboard === null) return
            void clipboard.writeText(metadata.url).then(() => setCopied(true), () => undefined)
        } catch {
            // A synchronous throw is the same non-event as a rejection.
        }
    }

    return (
        <div className="fd-region" data-phase-view="staged">
            <h1 className="fd-state-heading" tabIndex={-1}>{copy.stage.heading}</h1>
            <p className="fd-meta">{copy.qr.instruction}</p>

            <div>
                <span className="fd-packet-tab">{metadata.isDir ? copy.label.folder : copy.label.file}</span>
                <section className="fd-packet">
                    {warnings.map((warning) => (
                        <aside key={warning.code} className="fd-warning-banner" data-warning-code={warning.code}>
                            <strong className="fd-subheading">{errorHeadings[warning.code]}</strong>
                            <span>{warning.message}</span>
                        </aside>
                    ))}

                    <div className="fd-hero">
                        <div className="fd-hero__details">
                            <h2 className="fd-headline"><bdi dir="auto">{metadata.name}</bdi></h2>
                            <p className="fd-meta">
                                {(metadata.isDir ? copy.label.folder : copy.label.file) +
                                    copy.label.metaSeparator + size}
                            </p>
                            {metadata.isDir ? <p className="fd-subheading">{copy.folder.note}</p> : null}

                            <div className="fd-handoff">
                                <h3 id="fd-direct-link-heading" className="fd-subheading">
                                    {copy.label.directLinkHeading}
                                </h3>
                                <div className="fd-direct-row">
                                    <div
                                        className="fd-url"
                                        role="textbox"
                                        aria-readonly="true"
                                        aria-labelledby="fd-direct-link-heading"
                                        tabIndex={0}
                                    >
                                        {metadata.url}
                                    </div>
                                    <button
                                        type="button"
                                        className={`fd-button fd-target ${copied ? 'fd-button--copied' : 'fd-button--primary'}`}
                                        onClick={handleCopy}
                                    >
                                        {copied ? copy.copy.confirmation : copy.directLink.action}
                                    </button>
                                </div>
                                <p className="fd-meta">{copy.directLink.helper}</p>
                            </div>
                        </div>

                        <div className="fd-qr-panel">
                            <img
                                className="fd-qr"
                                src={`data:image/png;base64,${metadata.qrBase64}`}
                                alt={qrAltFor(metadata.name)}
                            />
                        </div>
                    </div>

                    <p className="fd-notice">{copy.firstOpener.warning}</p>

                    <div className="fd-trust">
                        <p>{copy.network.disclosure}</p>
                        <p>{copy.localCopy.disclosure}</p>
                    </div>

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
