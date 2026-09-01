import {useState} from 'react'
import {CopyToClipboard} from '../../wailsjs/go/main/App'
import {selectCommandError, selectWarnings} from '../transfer/selectors'
import type {StagedTransferState} from '../transfer/state'
import {OutcomePanel} from './OutcomePanel'
import {RecoveryHelp} from './RecoveryHelp'
import {copy, errorHeadings, qrAltFor} from './copy'
import {formatBytes} from './format'

interface StagedViewProps {
    readonly state: StagedTransferState
    readonly onCancel: () => void
    /**
     * Writes the app's one status announcer.
     *
     * Copy success is an announcer-owned row of the routing table and it is not
     * a reducer transition, so this view is the only thing that can report it.
     * Optional so the view still renders standalone.
     */
    readonly onAnnounce?: (text: string) => void
}

/**
 * Staged: the QR is the handoff, the direct URL is the fallback.
 *
 * The capability token reaches the DOM exactly twice and both places are
 * inert: encoded inside the QR bitmap, and as the readonly value of the URL
 * field. There is no anchor, because a sender-side activation link would let
 * this window consume its own one-shot download.
 *
 * `data:image/png;base64,` is prepended here, at render, and nowhere else. The
 * reducer holds the bare base64 payload the backend produced.
 */
export function StagedView({state, onCancel, onAnnounce}: StagedViewProps) {
    const {metadata} = state
    const warnings = selectWarnings(state)
    const commandError = selectCommandError(state)
    const [copied, setCopied] = useState(false)
    const [showFullName, setShowFullName] = useState(false)

    const size = metadata.isDir
        ? `${formatBytes(metadata.size)} ${copy.label.logicalSize}`
        : formatBytes(metadata.size)

    /**
     * The copy goes through the bound Go command, not `navigator.clipboard`.
     *
     * The browser API is unavailable on one of the two supported platforms:
     * WKWebView serves this frontend from the custom `wails://` scheme, which
     * is not a secure context, so `navigator.clipboard` is undefined on macOS
     * and a browser-side copy would silently do nothing there. WebView2 serves
     * `http://wails.localhost`, which is trustworthy, so it would have worked
     * on Windows only. The Wails runtime clipboard works on both.
     *
     * A write that never happened is still never reported as one: the label
     * changes, and the announcer speaks, only after the command resolves.
     */
    const handleCopy = () => {
        void Promise.resolve()
            .then(() => CopyToClipboard(metadata.url))
            .then(() => {
                setCopied(true)
                onAnnounce?.(copy.copy.confirmation)
            }, () => undefined)
    }

    return (
        <div className="fd-region" data-phase-view="staged">
            <h1 className="fd-state-heading" tabIndex={-1} data-focus-target="staged-heading">
                {copy.stage.heading}
            </h1>
            <p className="fd-meta">{copy.qr.instruction}</p>

            <div>
                <span className="fd-packet-tab">{metadata.isDir ? copy.label.folder : copy.label.file}</span>
                <section className="fd-packet">
                    {warnings.map((warning, index) => (
                        <aside
                            key={`${warning.code}-${index}`}
                            className="fd-warning-banner"
                            data-warning-code={warning.code}
                        >
                            <strong className="fd-subheading">{errorHeadings[warning.code]}</strong>
                            <span>{warning.message}</span>
                        </aside>
                    ))}

                    <div className="fd-hero">
                        <div className="fd-hero__details">
                            <h2 className="fd-headline" id="fd-item-name">
                                <bdi dir="auto" className={showFullName ? undefined : 'fd-clamp'}>
                                    {metadata.name}
                                </bdi>
                            </h2>
                            {/*
                              The visual clamp above is allowed only beside a
                              persistent keyboard-operable control that reaches
                              the whole value, and an assistive description that
                              carries it in full. The name is never cut by
                              JavaScript; only its box is.
                            */}
                            <button
                                type="button"
                                className="fd-button fd-name-toggle fd-target"
                                aria-expanded={showFullName}
                                aria-controls="fd-item-name"
                                aria-describedby="fd-item-name-full"
                                onClick={() => setShowFullName((shown) => !shown)}
                            >
                                {copy.name.showFull}
                            </button>
                            <span id="fd-item-name-full" className="fd-visually-hidden">
                                <bdi dir="auto">{metadata.name}</bdi>
                            </span>
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
                                    {/*
                                      A readonly form control, not a div wearing
                                      a textbox role: assistive technology reads
                                      the value of one reliably and disagrees
                                      about the other. It is still not a link.
                                    */}
                                    <input
                                        className="fd-url fd-target"
                                        type="text"
                                        readOnly
                                        value={metadata.url}
                                        aria-labelledby="fd-direct-link-heading"
                                        onFocus={(event) => event.currentTarget.select()}
                                    />
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

                    <RecoveryHelp/>

                    <button
                        type="button"
                        className="fd-button fd-button--quiet fd-target"
                        aria-disabled={state.cancelPending || undefined}
                        onClick={() => {
                            if (!state.cancelPending) onCancel()
                        }}
                    >
                        {state.cancelPending ? copy.cancel.pending : copy.cancel.action}
                    </button>
                </section>
            </div>

            {commandError === null ? null : (
                <OutcomePanel
                    outcome={{kind: 'error', retained: false, error: commandError}}
                    focusTarget="command-error"
                />
            )}
        </div>
    )
}
