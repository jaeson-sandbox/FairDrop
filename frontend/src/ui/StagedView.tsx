import type {CSSProperties} from 'react'
import {useId, useRef, useState} from 'react'
import {CopyToClipboard} from '../../wailsjs/go/main/App'
import {selectCommandError, selectWarnings} from '../transfer/selectors'
import type {StagedTransferState} from '../transfer/state'
import {Disclosure} from './Disclosure'
import {OutcomePanel} from './OutcomePanel'
import {StagedHelpContent} from './RecoveryHelp'
import {copy, errorHeadings, qrAltFor} from './copy'
import {formatBytes} from './format'

/**
 * Story 9.1's per-child stagger, duplicated from `IdleView.tsx` rather than
 * imported: the helper is trivial (one custom property) and nothing has
 * extracted a shared motion module yet -- see that file's own comment on the
 * same function. `--fd-stagger` is a custom property, which `CSSProperties`
 * does not itself declare, so every caller needs its own cast somewhere.
 */
function rise(step: number): CSSProperties {
    return {'--fd-stagger': step} as CSSProperties
}

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
 * Staged: the QR is the whole point of the screen; the link is there only
 * when the sender asks for it.
 *
 * Story 9.4 rebuilt this view around a single card (`.fd-packet`): the QR
 * tile on one side, the item, its two link actions, the revealed link and
 * the caveat lines on the other. Below the card, one row offers "Trouble
 * connecting?" and Cancel. The `fd-packet-tab` kind label, the always-visible
 * direct-link heading, the always-open `RecoveryHelp` block and the "Show
 * full name" toggle are all gone -- see `_bmad-output/planning-artifacts/
 * epics.md`'s Story 9.4 acceptance criteria.
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
    const [revealed, setRevealed] = useState(false)
    const revealId = useId()

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
     *
     * Story 9.4: this copies the link without revealing it -- `revealed` is
     * untouched here, matching the acceptance criterion that Copy Link and
     * Show Link are two independent actions.
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
            <div className="fd-staged-head">
                <h1
                    className="fd-state-heading"
                    tabIndex={-1}
                    data-focus-target="staged-heading"
                    aria-describedby={warningIds.length === 0 ? undefined : warningIds.join(' ')}
                >
                    {copy.stage.heading}
                </h1>
                <p className="fd-meta">{copy.qr.instruction}</p>
            </div>

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
                    {/*
                      Not `.fd-rise`: the QR is the view's own focal object,
                      not a staggered child behind it -- DESIGN.md's Motion
                      section ("cards and discs in the stories that follow")
                      and the owner-approved prototype both enter it with its
                      own fade-plus-scale-from-~0.94 at the same time as the
                      view itself, unstaggered, while the details column's
                      children stagger in behind it via `.fd-rise` below.
                    */}
                    <div className="fd-qr-panel">
                        {/*
                          Not draggable. The QR is a rendered bitmap of a
                          one-shot capability URL, not a file and not a
                          link -- there is no reason to drag it, and doing
                          so is a crash vector: WebKit puts the `data:`
                          image URL on the OS drag pasteboard as an NSURL,
                          and the vendored Wails native drop handler
                          (WailsWebView.m, performDragOperation:) calls
                          fileSystemRepresentation on every NSURL on the
                          pasteboard unconditionally, with no guard for a
                          non-file URL. Both `draggable={false}` (the
                          attribute WebKit's own drag-start check reads)
                          and `-webkit-user-drag: none` in style.css (the
                          CSS property WebKit actually honours for `<img>`)
                          are needed -- see the pinning test beside
                          "QR drag source is disabled" in
                          accessibility.test.tsx.
                        */}
                        <img
                            className="fd-qr"
                            src={`data:image/png;base64,${metadata.qrBase64}`}
                            alt={qrAltFor(metadata.name)}
                            draggable={false}
                        />
                    </div>

                    <div className="fd-hero__details">
                        <div className="fd-item fd-rise" style={rise(0)}>
                            <span className="fd-item__icon" aria-hidden="true">
                                {metadata.isDir ? <FolderKindGlyph/> : <FileKindGlyph/>}
                            </span>
                            <div className="fd-item__text">
                                <h2 className="fd-headline" id="fd-item-name">
                                    <bdi dir="auto">{metadata.name}</bdi>
                                </h2>
                                <p className="fd-meta">
                                    {(metadata.isDir ? copy.label.folder : copy.label.file) +
                                        copy.label.metaSeparator + size}
                                </p>
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
                                {/*
                                  Story 9.4: a non-reflowing crossfade, not a
                                  jump cut -- both faces are always mounted in
                                  the same grid cell (`.fd-button__swap` in
                                  style.css) so the transition can run, and the
                                  inactive one carries `aria-hidden="true"` so
                                  the button's accessible name is always
                                  exactly one of "Copy Link" or "Copied", never
                                  both concatenated.
                                */}
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

                        {/*
                          Story 9.4: the link is not rendered until requested.
                          This region stays mounted either way -- the same
                          transitioned grid-rows/visibility technique
                          `Disclosure.tsx` uses (see `.fd-url-reveal` in
                          style.css) rather than an unmount -- which is what
                          lets it animate closed as well as open, and what
                          keeps Story 7.9's CSS-grid mirror sizing untouched:
                          the field beneath is exactly the one Story 7.9 built,
                          just conditionally out of the accessibility tree and
                          the tab order while collapsed.
                        */}
                        <div id={revealId} className="fd-url-reveal" data-open={revealed || undefined}>
                            <div className="fd-url-reveal__inner">
                                <div className="fd-url-reveal__body">
                                    {/*
                                      A readonly form control, not a div wearing
                                      a textbox role: assistive technology reads
                                      the value of one reliably and disagrees
                                      about the other. It is still not a link.

                                      The field's height comes entirely from CSS: this
                                      wrapper is `display: grid`, and it and the hidden
                                      `.fd-url-mirror` below carry the same URL in the same
                                      grid cell, `1 / 1`. The mirror is a plain block that
                                      wraps like any other text, so it -- not the textarea's
                                      own `rows` -- is what the grid cell's height comes
                                      from; the textarea then stretches to fill that cell,
                                      which is the grid default for a block-axis-auto item.
                                      A width change re-wraps the mirror and resizes the
                                      cell in the same layout pass that re-wraps everything
                                      else on the page -- no observer, no
                                      `requestAnimationFrame`, no JavaScript at all.

                                      The mirror is a real element carrying the text as its
                                      own content, not a `::after` reading it back from a
                                      `data-*` attribute with `attr()`. See
                                      `_bmad-output/implementation-artifacts/evidence-7-9-make-resizing-seamless.md`
                                      for the failing-test evidence this was found with.
                                    */}
                                    <div className="fd-url-wrap">
                                        <div className="fd-url-mirror" aria-hidden="true">
                                            {metadata.url + ' '}
                                        </div>
                                        <textarea
                                            className="fd-url fd-target"
                                            readOnly
                                            rows={1}
                                            value={metadata.url}
                                            aria-label={copy.label.directLinkHeading}
                                            onFocus={(event) => event.currentTarget.select()}
                                            onKeyDown={(event) => {
                                                // Escape clears focus rather than leaving the field
                                                // ringed -- the same "get me out of this" reading as
                                                // BrowseControl's `closeAndBlur` and the disclosure
                                                // summary's own Escape handler (see their comments for
                                                // the fuller reasoning). Nothing here is open to
                                                // dismiss, so this is only the blur.
                                                if (event.key === 'Escape') event.currentTarget.blur()
                                            }}
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
                                    </div>
                                </div>
                            </div>
                        </div>

                        {/*
                          Story 9.4: the caveat lines. First-opener and network
                          stay visible on the card -- an info glyph and a lock
                          glyph respectively, per the acceptance criteria --
                          while the local-copy line and the link-preview
                          caveat move into "Trouble connecting?" below. The
                          folder note (when present) is the third line, with
                          no glyph of its own.
                        */}
                        <ul className="fd-caveats fd-rise" style={rise(2)}>
                            <li>
                                <InfoGlyph/>
                                {copy.firstOpener.warning}
                            </li>
                            <li>
                                <LockGlyph/>
                                {copy.network.disclosure}
                            </li>
                            {metadata.isDir ? <li className="fd-caveats__plain">{copy.folder.note}</li> : null}
                        </ul>
                    </div>
                </div>
            </section>

            <div className="fd-staged-foot fd-rise" style={rise(3)}>
                <Disclosure
                    className="fd-help"
                    headingId="fd-staged-help-heading"
                    summary={copy.help.heading}
                >
                    <StagedHelpContent/>
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

/** Decorative link-chain glyph on the resting Copy Link face (Story 9.4). */
function LinkGlyph() {
    return (
        <svg
            className="fd-button__glyph"
            viewBox="0 0 16 16"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.7"
            strokeLinecap="round"
            aria-hidden="true"
            focusable="false"
        >
            <path d="M6.5 9.5a3 3 0 0 0 4.2 0l2.1-2.1a3 3 0 0 0-4.2-4.2l-.7.7"/>
            <path d="M9.5 6.5a3 3 0 0 0-4.2 0L3.2 8.6a3 3 0 0 0 4.2 4.2l.7-.7"/>
        </svg>
    )
}

/** Decorative check glyph on the Copied face (Story 9.4). */
function CheckGlyph() {
    return (
        <svg
            className="fd-button__glyph"
            viewBox="0 0 16 16"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden="true"
            focusable="false"
        >
            <path d="m3.5 8.5 3 3 6-7"/>
        </svg>
    )
}

/** Decorative info glyph beside the first-opener caveat (Story 9.4). */
function InfoGlyph() {
    return (
        <svg
            className="fd-caveats__glyph"
            viewBox="0 0 16 16"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            aria-hidden="true"
            focusable="false"
        >
            <circle cx="8" cy="8" r="6.2"/>
            <path d="M8 7.2v3.8M8 5v.1" strokeLinecap="round"/>
        </svg>
    )
}

/** Decorative lock glyph beside the network caveat (Story 9.4). */
function LockGlyph() {
    return (
        <svg
            className="fd-caveats__glyph"
            viewBox="0 0 16 16"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinejoin="round"
            aria-hidden="true"
            focusable="false"
        >
            <rect x="3" y="7" width="10" height="7" rx="1.5"/>
            <path d="M5.5 7V5a2.5 2.5 0 0 1 4.8-1"/>
        </svg>
    )
}

/** Decorative file-kind glyph beside the item name (Story 9.4). */
function FileKindGlyph() {
    return (
        <svg
            className="fd-item__glyph"
            viewBox="0 0 20 20"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.6"
            strokeLinejoin="round"
            aria-hidden="true"
            focusable="false"
        >
            <path d="M5 2.5h6.5L15.5 6.5v11H5z"/>
            <path d="M11.5 2.5v4h4"/>
        </svg>
    )
}

/** Decorative folder-kind glyph beside the item name (Story 9.4). */
function FolderKindGlyph() {
    return (
        <svg
            className="fd-item__glyph"
            viewBox="0 0 20 20"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.6"
            strokeLinejoin="round"
            aria-hidden="true"
            focusable="false"
        >
            <path d="M2.5 5a1.2 1.2 0 0 1 1.2-1.2h4.2l1.8 1.8h7.1a1.2 1.2 0 0 1 1.2 1.2v8.2a1.2 1.2 0 0 1-1.2 1.2h-13a1.2 1.2 0 0 1-1.2-1.2z"/>
        </svg>
    )
}
