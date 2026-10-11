import type {CSSProperties, MouseEvent} from 'react'
import {useId, useRef, useState} from 'react'
import {CopyToClipboard} from '../../wailsjs/go/main/App'
import {selectCommandError, selectWarnings} from '../transfer/selectors'
import type {ReceiveWaitingTransferState} from '../transfer/state'
import {Disclosure} from './Disclosure'
import {OutcomePanel} from './OutcomePanel'
import {copy, errorHeadings, receiveQrAltFor} from './copy'

/** The per-child stagger, written once per view like its siblings (see `StagedView.tsx`). */
function rise(step: number): CSSProperties {
    return {'--fd-stagger': step} as CSSProperties
}

interface ReceiveWaitingViewProps {
    readonly state: ReceiveWaitingTransferState
    readonly onCancel: () => void
    /** Writes the app's one status announcer; copy success is a spoken row, not a reducer transition. */
    readonly onAnnounce?: (sessionId: string, text: string) => void
    /** Reports a clipboard write that failed, which only this view can observe. */
    readonly onCopyFailed?: (sessionId: string) => void
}

/**
 * Receive, waiting for a phone: the QR is the point of the screen and the link
 * is there only when asked for, exactly as Staged is for a send.
 *
 * It reuses Staged's card geometry and its Copy Link / Show Link behaviour on
 * purpose -- one way to hand a code to a phone, not two -- but it is its own
 * view because almost nothing it says is the same: the folder is named by its
 * basename only (the path never reaches the frontend), the instruction tells
 * the phone's user to *upload*, and the link is described as the upload link.
 *
 * The capability token reaches the DOM exactly where it does in Staged, and
 * both places are inert: inside the QR bitmap and as the readonly value of the
 * revealed field. There is no anchor, because a desktop-side activation link
 * would let this window spend its own one-time upload.
 *
 * A refused upload (`transfer-notice`) does not end the session or move focus.
 * It leaves a visible note beside the code and is spoken once by the
 * announcer, which is the routing table's rule for a non-terminal fact.
 */
export function ReceiveWaitingView({state, onCancel, onAnnounce, onCopyFailed}: ReceiveWaitingViewProps) {
    const {metadata} = state
    const warnings = selectWarnings(state)
    const commandError = selectCommandError(state)
    const [copied, setCopied] = useState(false)
    // Whether the copy control holds focus right now; see `StagedView` for why
    // this is a ref and why activation focuses the control explicitly.
    const focusedRef = useRef(false)
    const [revealed, setRevealed] = useState(false)
    const revealId = useId()

    const noteIds = [
        ...warnings.map((_, index) => `fd-receive-warning-${index}`),
        ...(state.refusals > 0 ? ['fd-receive-refused'] : []),
    ]

    /**
     * Copies the upload link through the bound Go command, never
     * `navigator.clipboard` (unavailable on macOS's non-secure webview scheme).
     * Focusing the control on every activation is what makes "the control holds
     * focus after activation" true on WebKit, which does not focus a clicked
     * `<button>`; it is the same target this handler already owns, so it needs
     * no `data-focus-return` marker.
     */
    const handleCopy = (event: MouseEvent<HTMLButtonElement>) => {
        event.currentTarget.focus()
        setCopied(false)
        void Promise.resolve()
            .then(() => CopyToClipboard(metadata.url))
            .then(() => {
                if (focusedRef.current) setCopied(true)
                onAnnounce?.(state.session.sessionId, copy.copy.confirmation)
            }, () => onCopyFailed?.(state.session.sessionId))
    }

    const handleCopyBlur = () => {
        focusedRef.current = false
        setCopied(false)
    }

    return (
        <div className="fd-region" data-phase-view="receive-waiting">
            <div className="fd-staged-head">
                <h1
                    className="fd-state-heading"
                    tabIndex={-1}
                    data-focus-target="receive-waiting-heading"
                    aria-describedby={noteIds.length === 0 ? undefined : noteIds.join(' ')}
                >
                    {copy.receive.heading}
                </h1>
                <p className="fd-meta">{copy.receive.instruction}</p>
            </div>

            <section className="fd-packet">
                {warnings.map((warning, index) => (
                    <aside
                        key={`${warning.code}-${index}`}
                        id={`fd-receive-warning-${index}`}
                        className="fd-warning-banner"
                        data-warning-code={warning.code}
                    >
                        <strong className="fd-subheading">{errorHeadings[warning.code]}</strong>
                        <span>{warning.code === 'beacon_warning' ? copy.receive.discoveryWarning : warning.message}</span>
                    </aside>
                ))}

                {state.refusals > 0 ? (
                    <aside id="fd-receive-refused" className="fd-warning-banner" data-receive-notice="receive_too_large">
                        <strong className="fd-subheading">{copy.receive.tooLargeTitle}</strong>
                        <span>{copy.receive.tooLarge}</span>
                    </aside>
                ) : null}

                <div className="fd-hero">
                    <div className="fd-qr-panel">
                        {/*
                          Not draggable, for the reason recorded in StagedView and
                          AGENTS.md (WebKit puts the data: URL on the drag
                          pasteboard and the vendored Wails drop handler crashes on
                          a non-file NSURL). `draggable={false}` here and
                          `-webkit-user-drag: none` in style.css are both needed.
                        */}
                        <img
                            className="fd-qr"
                            src={`data:image/png;base64,${metadata.qrBase64}`}
                            alt={receiveQrAltFor(metadata.destination)}
                            draggable={false}
                        />
                    </div>

                    <div className="fd-hero__details">
                        <div className="fd-item fd-rise" style={rise(0)}>
                            <span className="fd-item__icon" aria-hidden="true">
                                <FolderKindGlyph/>
                            </span>
                            <div className="fd-item__text">
                                <h2 className="fd-headline" id="fd-item-name">
                                    <bdi dir="auto">{metadata.destination}</bdi>
                                </h2>
                                <p className="fd-meta">{copy.receive.destinationNote}</p>
                            </div>
                        </div>

                        <div className="fd-direct-row fd-rise" style={rise(1)}>
                            <button
                                type="button"
                                className={`fd-button fd-target ${copied ? 'fd-button--copied' : 'fd-button--primary'}`}
                                onClick={handleCopy}
                                onFocus={() => { focusedRef.current = true }}
                                onBlur={handleCopyBlur}
                            >
                                <span className="fd-button__swap">
                                    <span className="fd-button__swap-face" aria-hidden={copied || undefined}>
                                        <LinkGlyph/>
                                        {copy.directLink.action}
                                    </span>
                                    <span className="fd-button__swap-face" aria-hidden={copied ? undefined : true}>
                                        <CheckGlyph/>
                                        {copy.copy.confirmation}
                                    </span>
                                </span>
                            </button>
                            <button
                                type="button"
                                className="fd-button fd-button--secondary fd-target"
                                aria-expanded={revealed}
                                aria-controls={revealId}
                                onClick={() => setRevealed((was) => !was)}
                            >
                                {revealed ? copy.directLink.hide : copy.directLink.show}
                            </button>
                        </div>

                        <div id={revealId} className="fd-url-reveal" data-open={revealed || undefined}>
                            <div className="fd-url-reveal__inner">
                                <div className="fd-url-reveal__body">
                                    <div className="fd-url-wrap">
                                        <div className="fd-url-mirror" aria-hidden="true">
                                            {metadata.url + ' '}
                                        </div>
                                        <textarea
                                            className="fd-url fd-target"
                                            readOnly
                                            rows={1}
                                            value={metadata.url}
                                            aria-label={copy.label.uploadLinkHeading}
                                            onFocus={(event) => event.currentTarget.select()}
                                            onKeyDown={(event) => {
                                                if (event.key === 'Escape') event.currentTarget.blur()
                                            }}
                                            onMouseDown={(event) => {
                                                if (document.activeElement !== event.currentTarget) {
                                                    event.preventDefault()
                                                    event.currentTarget.focus()
                                                }
                                            }}
                                        />
                                    </div>
                                </div>
                            </div>
                        </div>

                        <ul className="fd-caveats fd-rise" style={rise(2)}>
                            <li>
                                <InfoGlyph/>
                                {copy.receive.once}
                            </li>
                            <li>
                                <LockGlyph/>
                                {copy.network.disclosure}
                            </li>
                        </ul>
                    </div>
                </div>
            </section>

            <div className="fd-staged-foot fd-rise" style={rise(3)}>
                <Disclosure
                    className="fd-help"
                    headingId="fd-receive-help-heading"
                    summary={copy.help.heading}
                >
                    <p className="fd-body">{copy.receive.helpDifferentLan}</p>
                </Disclosure>
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

function LinkGlyph() {
    return (
        <svg className="fd-button__glyph" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.7"
            strokeLinecap="round" aria-hidden="true" focusable="false">
            <path d="M6.5 9.5a3 3 0 0 0 4.2 0l2.1-2.1a3 3 0 0 0-4.2-4.2l-.7.7"/>
            <path d="M9.5 6.5a3 3 0 0 0-4.2 0L3.2 8.6a3 3 0 0 0 4.2 4.2l.7-.7"/>
        </svg>
    )
}

function CheckGlyph() {
    return (
        <svg className="fd-button__glyph" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2"
            strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false">
            <path d="m3.5 8.5 3 3 6-7"/>
        </svg>
    )
}

function InfoGlyph() {
    return (
        <svg className="fd-caveats__glyph" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5"
            aria-hidden="true" focusable="false">
            <circle cx="8" cy="8" r="6.2"/>
            <path d="M8 7.2v3.8M8 5v.1" strokeLinecap="round"/>
        </svg>
    )
}

function LockGlyph() {
    return (
        <svg className="fd-caveats__glyph" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5"
            strokeLinejoin="round" aria-hidden="true" focusable="false">
            <rect x="3" y="7" width="10" height="7" rx="1.5"/>
            <path d="M5.5 7V5a2.5 2.5 0 0 1 4.8-1"/>
        </svg>
    )
}

/** Decorative folder glyph beside the destination name. */
export function FolderKindGlyph() {
    return (
        <svg className="fd-item__glyph" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.6"
            strokeLinejoin="round" aria-hidden="true" focusable="false">
            <path d="M2.5 5a1.2 1.2 0 0 1 1.2-1.2h4.2l1.8 1.8h7.1a1.2 1.2 0 0 1 1.2 1.2v8.2a1.2 1.2 0 0 1-1.2 1.2h-13a1.2 1.2 0 0 1-1.2-1.2z"/>
        </svg>
    )
}
