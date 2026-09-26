import type {CSSProperties, ReactNode} from 'react'
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
 * Story 9.1's per-child stagger, duplicated from `StagedView.tsx` rather than
 * imported: the helper is trivial (one custom property) and nothing has
 * extracted a shared motion module yet -- see that file's own comment on the
 * same function. `--fd-stagger` is a custom property, which `CSSProperties`
 * does not itself declare, so every caller needs its own cast somewhere.
 */
function rise(step: number): CSSProperties {
    return {'--fd-stagger': step} as CSSProperties
}

/**
 * The SVG circle's radius and circumference, matching the owner-approved
 * prototype's own `.ring` geometry exactly (a 216x216 viewBox, cx/cy 108,
 * r 96): `2 * PI * 96` is the same ~603.19 the prototype hardcodes, computed
 * here rather than copied so a future radius change cannot drift the two
 * apart.
 */
const ringRadius = 96
const ringCircumference = 2 * Math.PI * ringRadius

/**
 * Transferring: the code the sender scanned becomes the progress, in the
 * same slot, so the card never changes shape between Staged and Sending
 * (Story 9.5).
 *
 * Sending reuses Staged's own card geometry verbatim -- `.fd-packet` and
 * `.fd-hero`'s fixed 216px/`minmax(0, 1fr)` grid -- rather than the
 * standalone `.fd-transfer-view` card and `fd-packet-tab` kind label Story
 * 7.5 built. The packet tab is gone; the item's kind is the same plain glyph
 * beside its name that Staged uses.
 *
 * The three presentations come from `ProgressSelection.mode`, which the reducer
 * derives from the wire snapshot's own known/unknown discriminator. Nothing
 * here divides, and nothing recomputes a percentage: a folder never gets a
 * percentage from its logical size, and an empty file never gets a 0% bar.
 *
 * Until the first accepted snapshot arrives there is no mode at all --
 * `ProgressRing` renders a plain, undecorated track rather than inventing one
 * from the metadata.
 */
export function TransferringView({state, onCancel}: TransferringViewProps) {
    const {metadata} = state
    const progress = selectProgress(state)
    const commandError = selectCommandError(state)

    const size = metadata.isDir
        ? `${formatBytes(metadata.size)} ${copy.label.logicalSize}`
        : formatBytes(metadata.size)

    return (
        <div className="fd-region" data-phase-view="transferring">
            {/*
              Reuses `.fd-staged-head`'s centring rule verbatim (Story 9.4):
              it only centres its own text, nothing Staged-specific. The
              story's own text overrides the owner-approved prototype here --
              "Sending" carries no subtitle, unlike Staged's one-line
              instruction beneath its heading.
            */}
            <div className="fd-staged-head">
                <h1 className="fd-state-heading" tabIndex={-1} data-focus-target="transferring-heading">
                    {copy.label.sending}
                </h1>
            </div>

            <section className="fd-packet">
                <div className="fd-hero">
                    <ProgressRing progress={progress}/>

                    <div className="fd-hero__details">
                        {/*
                          Not staggered, matching the owner-approved
                          prototype's `t-sending` template: the item row is
                          the one thing that does not change moving from
                          Staged to Sending, so it is not re-announced with a
                          stagger delay the way the two new figures and
                          Cancel are below.
                        */}
                        <div className="fd-item">
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
                                {metadata.isDir ? <p className="fd-subheading">{copy.folder.note}</p> : null}
                            </div>
                        </div>

                        {progress?.mode === 'known-empty' ? (
                            <p className="fd-empty-status">{copy.progress.knownEmpty}</p>
                        ) : null}

                        {progress === null ? null : (
                            <div className="fd-rise" style={rise(1)}>
                                <TransferMetrics progress={progress}/>
                            </div>
                        )}

                        {/*
                          A pending cancellation keeps this control focused and
                          refuses a second activation. `aria-disabled`, never
                          `disabled`: the latter would move focus off the control
                          the spine says must keep it.
                        */}
                        <button
                            type="button"
                            className="fd-button fd-button--quiet fd-target fd-rise"
                            style={rise(2)}
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

/**
 * The 216px progress ring, in Staged's QR slot (Story 9.5), one of three
 * presentations plus the pre-snapshot state:
 *
 *  - `null` (no accepted snapshot yet): a plain, undecorated track. No mode
 *    has been reported, so nothing here invents one -- the same restraint
 *    the old linear meter observed by rendering nothing at all, just
 *    expressed as an empty track rather than an absent element, so the
 *    card's shape never jumps once the first snapshot lands.
 *  - `known-empty`: the track only, `aria-hidden`, no `role` -- no
 *    percentage-bearing progressbar exists for a known-empty payload.
 *  - `unknown`: a static, non-directional dashed ring (`stroke-dasharray`,
 *    never a rotation or a keyframe animation) with `role="progressbar"` and
 *    no `aria-valuenow`, and `copy.progress.unknown` beneath it.
 *  - `known-positive`: a determinate ring, `role="progressbar"` with
 *    `aria-valuenow`, its solid fill's `stroke-dashoffset` transitioned over
 *    400ms (collapsed to the universal 1ms under reduced motion, same as
 *    every other transition in this sheet), and the tabular-numeral
 *    percentage centred in it.
 *
 * The track's own fill is solid `{colors.primary}`: the single-gradient rule
 * is absolute and the primary button already spends the product's one
 * gradient (`styles.test.ts` counts it).
 */
function ProgressRing({progress}: {readonly progress: ProgressSelection | null}) {
    if (progress === null) {
        return (
            <div className="fd-ring-panel">
                <div className="fd-ring-frame">
                    <RingTrack/>
                </div>
            </div>
        )
    }

    if (progress.mode === 'known-empty') {
        return (
            <div className="fd-ring-panel" data-progress-mode="known-empty" aria-hidden="true">
                <div className="fd-ring-frame">
                    <RingTrack/>
                </div>
            </div>
        )
    }

    if (progress.mode === 'unknown') {
        return (
            <div
                className="fd-ring-panel"
                data-progress-mode="unknown"
                role="progressbar"
                aria-labelledby="fd-ring-status"
            >
                <div className="fd-ring-frame">
                    <RingTrack>
                        <circle className="fd-ring__fill--unknown" cx="108" cy="108" r={ringRadius}/>
                    </RingTrack>
                </div>
                <p id="fd-ring-status" className="fd-ring__status">{copy.progress.unknown}</p>
            </div>
        )
    }

    const percent = Math.round(progress.value)
    // Fully hidden at 0 (the whole dash offset showing), fully drawn at 100
    // (no offset) -- the same formula the owner-approved prototype's own
    // `runProgress()` uses.
    const offset = ringCircumference * (1 - progress.value / 100)
    return (
        <div
            className="fd-ring-panel"
            data-progress-mode="known-positive"
            role="progressbar"
            aria-label={copy.label.sending}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={percent}
        >
            <div className="fd-ring-frame">
                <RingTrack>
                    <circle
                        className="fd-ring__fill"
                        cx="108"
                        cy="108"
                        r={ringRadius}
                        strokeDasharray={ringCircumference}
                        style={{strokeDashoffset: offset}}
                    />
                </RingTrack>
                {/*
                  Defect fix (orchestrator's rendered 1024x768 review after
                  Story 9.5 merged): `.fd-ring__pct` used to be a grid
                  container with two direct children (this span and the
                  `<small>`), and an implicit grid with no declared
                  `grid-template-columns` packs multiple children into
                  separate rows by default -- so the number centred on its
                  own row while the "%" sign centred on a second row near
                  the ring's bottom edge, each individually "centred" but
                  never together. See the matching `.fd-ring__pct` rule in
                  style.css (now flex, baseline-aligned) for the fix.
                */}
                <p className="fd-ring__pct">
                    <span className="fd-ring__pct-value">{percent}</span><small>%</small>
                </p>
            </div>
        </div>
    )
}

/**
 * The track circle every mode shares, plus its own functional-boundary edge
 * (DESIGN.md's Colors table: "the progress-track outline" -- `--color-track`
 * alone measures ~1.2:1 against the card surface, so the track needs a
 * separate 3:1 edge the same way the retired linear meter's 1px `border`
 * gave it one; see the fuller comment on `.fd-ring__edge` in style.css for
 * why that edge is a second stroke here rather than the track's own color),
 * plus whichever fill circle the caller passes as `children`. Always
 * `aria-hidden`: the wrapping `.fd-ring-panel` carries whatever role and
 * ARIA attributes the mode needs, so the SVG itself never speaks for the
 * state twice.
 *
 * The `-90deg` rotation is a fixed, static orientation (the fill starts
 * drawing from 12 o'clock rather than 3 o'clock) -- not motion, and untouched
 * by `prefers-reduced-motion` for the same reason `.fd-button:active`'s own
 * press `transform` is untouched by it (see that rule's comment in
 * style.css): it is a permanent value, never a transition.
 */
function RingTrack({children}: {readonly children?: ReactNode}) {
    return (
        <svg className="fd-ring" viewBox="0 0 216 216" aria-hidden="true" focusable="false">
            <circle className="fd-ring__track" cx="108" cy="108" r={ringRadius}/>
            <circle className="fd-ring__edge" cx="108" cy="108" r={ringRadius + 5}/>
            {children}
        </svg>
    )
}

/**
 * The two plain figure-over-caption pairs (Story 9.5): the captions are
 * `copy.label.sentCaption`/`copy.label.speedCaption` ("Sent"/"Speed"), not
 * `copy.label.wireBytes`/`copy.label.throughput` ("Wire bytes"/"Throughput")
 * -- that pair is the Completion Receipt's own wording (`OutcomePanel.tsx`,
 * Story 7.5), a different card this story does not touch. The figures
 * themselves are exactly what those captions described: actual wire bytes
 * and visual-only throughput, never a wire percentage or a logical size.
 *
 * Defect fix (orchestrator's rendered review after Story 9.5 merged): the
 * "Sent" figure used to read `${bytesSent} sent` -- e.g. "14.0 KB sent" --
 * over its own "Sent" caption, so the value repeated the caption's word
 * rather than adding a second fact. A known total now reads sent-of-total
 * (`copy.label.of`, already registered and already used by the pre-ring
 * progress head this story's own predecessor rendered), which is new
 * information the caption alone does not carry; an unknown total or an
 * empty payload has no meaningful "of X" to add (an unknown ZIP's own total
 * is not a number, and an empty file's total is a meaningless "of 0
 * bytes"), so those two read the bare wire figure instead, matching the
 * unknown-mode reading the acceptance criteria name.
 */
function TransferMetrics({progress}: {readonly progress: ProgressSelection}) {
    const sentValue = progress.mode === 'known-positive'
        ? `${formatBytes(progress.bytesSent)} ${copy.label.of} ${formatBytes(progress.totalBytes)}`
        : formatBytes(progress.bytesSent)

    return (
        <div className="fd-metrics">
            <div className="fd-metric">
                <strong className="fd-metric__value">{sentValue}</strong>
                <span className="fd-metric__caption">{copy.label.sentCaption}</span>
            </div>
            {progress.mode === 'known-empty' ? null : (
                <div className="fd-metric">
                    <strong className="fd-metric__value">{formatRate(progress.speedBytesPerSec)}</strong>
                    <span className="fd-metric__caption">{copy.label.speedCaption}</span>
                </div>
            )}
        </div>
    )
}

/** Decorative file-kind glyph beside the item name, duplicated from `StagedView.tsx` (Story 9.5). */
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

/** Decorative folder-kind glyph beside the item name, duplicated from `StagedView.tsx` (Story 9.5). */
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
