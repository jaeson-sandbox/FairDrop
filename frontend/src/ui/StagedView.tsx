import {useRef, useState} from 'react'
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
    readonly onAnnounce?: (sessionId: string, text: string) => void
    /**
     * Reports a clipboard write that failed.
     *
     * The command's rejection has nowhere else to go: `commandError` is reducer
     * state, and this is the only thing that issues the command. Optional for
     * the same reason `onAnnounce` is -- the view still renders standalone.
     */
    readonly onCopyFailed?: (sessionId: string) => void
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
export function StagedView({state, onCancel, onAnnounce, onCopyFailed}: StagedViewProps) {
    const {metadata} = state
    const warnings = selectWarnings(state)
    const commandError = selectCommandError(state)
    const [copied, setCopied] = useState(false)
    // Whether the copy control holds focus right now. A ref rather than state
    // because the asynchronous clipboard callback below reads it after the
    // render that set it, and re-rendering on focus would buy nothing.
    const focusedRef = useRef(false)
    const [showFullName, setShowFullName] = useState(false)

    const size = metadata.isDir
        ? `${formatBytes(metadata.size)} ${copy.label.logicalSize}`
        : formatBytes(metadata.size)

    /*
      A warning arrives with the metadata, never after it, so the transition it
      belongs to is Stage success -- and that row is focus-owned by the heading
      below. The announcer row for a warning appearing at an already-staged
      session stays in the table for a reducer that replaces metadata, but no
      path produces one today, which is why the warning was silent on the one
      machine state it exists for (Epic 1 retrospective item 4).

      Describing the heading with the banners is what a focus-owned row can
      carry without becoming a second owner: the move that announces Stage
      success reads the warning as part of the same announcement, and the
      banner keeps its place in the packet for the people who can see it.
    */
    const warningIds = warnings.map((_, index) => `fd-warning-${index}`)

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
     * changes, and the announcer speaks, only after the command resolves -- and
     * the confirmation is cleared before each attempt, so a failure can never be
     * read beside a `Copied` left standing by an earlier success.
     *
     * The rejection is reported rather than discarded. `clipboard_failed` has
     * had a registry message and a heading since Story 3.11 and no way to reach
     * a user, which made a clipboard the OS refused indistinguishable from one
     * it accepted.
     */
    const handleCopy = () => {
        setCopied(false)
        void Promise.resolve()
            .then(() => CopyToClipboard(metadata.url))
            .then(() => {
                // Only claim the confirmation while the control still holds
                // focus. The command is asynchronous, so a sender who clicks
                // and tabs straight on can have it resolve after focus has
                // already gone -- and then no blur is ever coming to revert
                // the label, which is D-114 returning by the back door
                // (reproduced in Chromium). The announcement still happens:
                // the copy did succeed, and that is what the sender needs to
                // hear regardless of where focus went.
                if (focusedRef.current) setCopied(true)
                onAnnounce?.(state.session.sessionId, copy.copy.confirmation)
            }, () => onCopyFailed?.(state.session.sessionId))
    }

    /**
     * Reverts the label the moment the sender's own focus leaves the control
     * (D-114).
     *
     * A successful copy used to rename this button to `copy.copy.confirmation`
     * for the rest of the session with no way back: `EXPERIENCE.md` bans
     * frontend lifecycle timers, so nothing ever swapped it back, and the one
     * control that reaches the capability URL lost the name that says what it
     * does (WCAG 4.1.2) the moment it succeeded once.
     *
     * Blur is the trigger EXPERIENCE.md now sanctions for this control
     * specifically: it fires only from the sender's own action -- tabbing on,
     * clicking elsewhere -- never from a timer this product forbids, and by
     * the time focus "returns to the control" later (the acceptance
     * criterion's own words), the label has already reverted. It does not
     * fire while a fresh click on the still-focused "Copied" button retries
     * the copy (see the test beside this one): that click's own result --
     * success or failure -- is what decides the label next, exactly as
     * before.
     */
    const handleCopyBlur = () => {
        focusedRef.current = false
        setCopied(false)
    }

    return (
        <div className="fd-region" data-phase-view="staged">
            <h1
                className="fd-state-heading"
                tabIndex={-1}
                data-focus-target="staged-heading"
                aria-describedby={warningIds.length === 0 ? undefined : warningIds.join(' ')}
            >
                {copy.stage.heading}
            </h1>
            <p className="fd-meta">{copy.qr.instruction}</p>

            <div>
                <span className="fd-packet-tab">{metadata.isDir ? copy.label.folder : copy.label.file}</span>
                <section className="fd-packet">
                    {warnings.map((warning, index) => (
                        <aside
                            key={`${warning.code}-${index}`}
                            id={warningIds[index]}
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
                                className="fd-button fd-button--secondary fd-name-toggle fd-target"
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
                                    <textarea
                                        className="fd-url fd-target"
                                        readOnly
                                        rows={2}
                                        value={metadata.url}
                                        aria-labelledby="fd-direct-link-heading"
                                        onFocus={(event) => event.currentTarget.select()}
                                        onMouseDown={(event) => {
                                            // Without this the mouseup that follows collapses the
                                            // selection to a caret, and select-on-focus becomes a
                                            // call that happens and a selection nobody gets.
                                            if (document.activeElement !== event.currentTarget) {
                                                event.preventDefault()
                                                event.currentTarget.focus()
                                            }
                                        }}
                                    />
                                    <button
                                        type="button"
                                        className={`fd-button fd-target ${copied ? 'fd-button--copied' : 'fd-button--primary'}`}
                                        onClick={handleCopy}
                                        onFocus={() => { focusedRef.current = true }}
                                        onBlur={handleCopyBlur}
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
