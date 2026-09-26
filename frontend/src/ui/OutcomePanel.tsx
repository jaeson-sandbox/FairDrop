import type {CSSProperties} from 'react'
import {useEffect, useState} from 'react'
import type {OutcomePresentation} from '../transfer/selectors'
import type {CompletionReceipt as CompletionReceiptData} from '../transfer/types'
import type {FocusTarget} from './announce'
import {BrowseControl} from './BrowseControl'
import {copy, errorHeadings, errorMessages} from './copy'
import {formatBytes} from './format'

/**
 * The browse-menu wiring for a card's primary "pick something else" action --
 * "Send Another" on a Done card, "Choose Another" on an Error card whose
 * action is `choose` (Story 9.6). The label varies by caller; the menu itself
 * (File/Folder) never does, which is why `BrowseControl` still owns it.
 */
export interface OutcomeBrowseAction {
    readonly label: string
    readonly onSelectFile: () => void
    readonly onSelectDirectory: () => void
}

/**
 * Everything a caller supplies beyond the outcome itself, shared verbatim
 * between App's top-level slot (a live or retained Done/Error) and IdleView's
 * command-failure card -- both are the same "one card" Story 9.6 describes,
 * so both are built from the same prop shape.
 */
export interface OutcomeCardProps {
    /**
     * Present only when this card should itself be a native Wails drop
     * target: App's outcome slot and Idle's command-failure slot pass this;
     * Staged's inline `clipboard_failed` panel does not, because that one is
     * not an outcome card (it keeps its existing inline form).
     */
    readonly dropTargetStyle?: CSSProperties
    /** "Done" (a Done outcome) or "Dismiss" (an Error outcome/command failure). */
    readonly onDismiss?: () => void
    /** "Send Another" / "Choose Another". Omitted -> no browse-menu action renders. */
    readonly browse?: OutcomeBrowseAction
    /** "Try Again". Omitted -> not rendered. */
    readonly onRetry?: () => void
    /**
     * True while `stageFromOutcome`/`retry()` is releasing a live lease
     * before staging (Story 9.6). Every action this card owns -- the primary
     * (retry or browse), and Dismiss -- becomes non-activatable via
     * `aria-disabled`, never the native `disabled` attribute, so focus stays
     * put and a screen reader still finds the control. Dismiss is disabled
     * too, not only the action that started the wait: a Dismiss that took
     * effect immediately while a queued `stage()` was still going to fire
     * once the wait resolved would look instant but stage anyway a moment
     * later, which is the exact "Dismiss during the wait must not be
     * followed by a stage firing later" defect this guards against.
     */
    readonly busy?: boolean
}

interface OutcomePanelProps extends OutcomeCardProps {
    readonly outcome: OutcomePresentation
    /** Heading rank. The lifecycle outcome owns a document heading; nested panels do not. */
    readonly level?: 1 | 2
    /**
     * Whether this panel is the phase's own view.
     *
     * Separate from `level` because the retained node keeps the terminal
     * panel's heading rank -- reset must not change what the user is looking at
     * -- while the phase view moves to Idle underneath it.
     */
    readonly phaseView?: boolean
    /**
     * The routing-table target this panel answers to, when it is one.
     *
     * The lifecycle panel and a command-failure panel are different rows of the
     * table and carry different targets, which is what lets Idle show a
     * retained outcome and a fresh failure at once without the failure's focus
     * landing on the wrong one. The lifecycle panel keeps its target across a
     * reset even though no row targets it in Idle: focus is still sitting on
     * it, and removing the attribute would take its focus ring with it.
     */
    readonly focusTarget?: FocusTarget
}

/**
 * The one Done/Error surface -- one centred card, never wider than the
 * Staged card's own column, for a live or retained outcome and for a
 * Stage-time command failure in Idle (Story 9.6).
 *
 * The error text is read from the fixed registry by code, not from the error
 * value handed in. The reducer already replaces every incoming message with
 * registry copy; taking the code as the only input makes that structural, so a
 * message that somehow arrived from an adapter still cannot reach the screen.
 *
 * There is no `role="alert"` here, in any form. Every path that shows this
 * panel is a focus-owned row of the routing table, and the spine allows an
 * alert only on a path that does not also move focus.
 *
 * Every outcome carries a control, not only a retained one: a live Done or
 * Error still offers "Send Another"/Try Again/Choose Another and Done/Dismiss,
 * so a lost backend reset (D-059) never leaves the window with nothing to
 * press. Done's primary is always the browse menu ("Send Another"); an
 * Error's primary is whichever of Try Again/Choose Another the caller
 * supplies, chosen by `selectEffectiveErrorAction` one layer up -- this
 * component only renders what it is given.
 */
export function OutcomePanel({
    outcome,
    level = 2,
    phaseView = false,
    focusTarget,
    dropTargetStyle,
    onDismiss,
    browse,
    onRetry,
    busy = false,
}: OutcomePanelProps) {
    const done = outcome.kind === 'done'
    const Heading = level === 1 ? 'h1' : 'h2'
    // Focus lands on this section, so it needs a name of its own: a container
    // with a heading inside it is not named by that heading unless it says so.
    const headingId = `fd-outcome-heading-${outcome.kind}${outcome.retained ? '-retained' : ''}`

    // A retry/browse action is this card's "the primary next action" (DESIGN.md's
    // Outcome Panel rows); Dismiss/Done is quiet whenever one exists beside it,
    // and only takes primary weight itself when it is the only control on the
    // card -- exactly the D-059 concern the single-control shape used to carry
    // wholesale, now scoped to "no other action was offered" rather than to
    // "live vs retained".
    const hasOtherPrimary = done || onRetry !== undefined || browse !== undefined
    const dismissClassName = hasOtherPrimary
        ? 'fd-button fd-button--quiet fd-button--pill fd-target'
        : 'fd-button fd-button--primary fd-button--pill fd-target'

    // Narrowed once, explicitly, rather than leaned on through `done` in a
    // non-discriminant expression (`a || b`'s right side): `itemName` only
    // exists on the error variant, and this keeps that fact local to one line
    // instead of asking every later read to re-derive it from `outcome.kind`.
    const itemName = outcome.kind === 'error' ? outcome.itemName : undefined

    let step = 0
    const headingStep = step++
    const receiptStep = done || itemName !== undefined ? step++ : null
    const messageStep = done ? null : step++
    const actionsStep = step++

    return (
        <section
            className={`fd-outcome ${done ? 'fd-outcome--done' : 'fd-outcome--error'}`}
            style={dropTargetStyle}
            data-phase-view={phaseView ? 'outcome' : undefined}
            data-outcome={outcome.kind}
            data-retained={String(outcome.retained)}
            data-error-code={done ? undefined : outcome.error.code}
            data-focus-target={focusTarget}
            aria-labelledby={headingId}
            tabIndex={-1}
        >
            <OutcomeIcon done={done}/>
            <Heading className="fd-state-heading fd-rise" id={headingId} style={rise(headingStep)}>
                {done ? copy.done.heading : errorHeadings[outcome.error.code]}
            </Heading>
            {receiptStep === null ? null : done
                ? <DoneReceipt receipt={outcome.receipt} rise={rise(receiptStep)}/>
                : <ErrorReceipt name={itemName ?? ''} rise={rise(receiptStep)}/>}
            {messageStep === null || done ? null : (
                <p className="fd-outcome__body fd-rise" style={rise(messageStep)}>
                    {errorMessages[outcome.error.code]}
                </p>
            )}
            <div className="fd-outcome__actions fd-rise" style={rise(actionsStep)}>
                {done ? (
                    <BrowseControl
                        label={copy.done.sendAnother}
                        onSelectFile={browse?.onSelectFile ?? noop}
                        onSelectDirectory={browse?.onSelectDirectory ?? noop}
                        disabled={busy}
                    />
                ) : browse !== undefined ? (
                    <BrowseControl
                        label={browse.label}
                        onSelectFile={browse.onSelectFile}
                        onSelectDirectory={browse.onSelectDirectory}
                        disabled={busy}
                    />
                ) : onRetry !== undefined ? (
                    <button
                        type="button"
                        className="fd-button fd-button--primary fd-button--pill fd-target"
                        aria-disabled={busy || undefined}
                        onClick={() => { if (!busy) onRetry() }}
                    >
                        <RefreshGlyph/>
                        {copy.outcome.tryAgain}
                    </button>
                ) : null}
                {onDismiss === undefined ? null : (
                    <button
                        type="button"
                        className={dismissClassName}
                        aria-disabled={busy || undefined}
                        onClick={() => { if (!busy) onDismiss() }}
                    >
                        {done ? copy.done.dismiss : copy.outcome.dismiss}
                    </button>
                )}
            </div>
        </section>
    )
}

function noop(): void {
    // BrowseControl always renders for a Done outcome (Send Another is
    // unconditional per the AC); a caller that has not wired real handlers
    // yet (a bare render in a test, for instance) gets an inert menu rather
    // than a crash.
}

/** Story 9.1's per-child stagger, typed so `--fd-stagger` needs no cast at each call site. */
function rise(step: number): CSSProperties {
    return {'--fd-stagger': step} as CSSProperties
}

/**
 * The Done/Error glyph, painted inside a ~96px tinted disc.
 *
 * The check is a stroke-drawn path, not a text glyph: `pathLength={32}`
 * normalizes the path's own length to exactly 32 units regardless of its
 * geometry, so `stroke-dasharray: 32` / `stroke-dashoffset: 32` in style.css
 * always describes "fully hidden" and `stroke-dashoffset: 0` always describes
 * "fully drawn", independent of the coordinates chosen here.
 *
 * The draw is a CSS *transition*, never `@keyframes`/`animation` --
 * `styles.test.ts` refuses either anywhere in the sheet -- triggered by adding
 * a class once after mount. That happens unconditionally, on every mount,
 * regardless of `prefers-reduced-motion`: gating it on a reduced-motion check
 * would leave the class never added and the check permanently undrawn, which
 * is exactly the meaning-carrying state `prefers-reduced-motion` is not
 * allowed to remove. What reduced motion changes instead is speed, through
 * the sheet's existing universal `transition-duration: 1ms !important` rule --
 * the same seam every other transition in the product already relies on.
 *
 * Story 9.6: the disc itself now scales in from ~0.7 (`.fd-outcome__icon`'s
 * own `@starting-style`, unstaggered -- like the QR tile in 9.4, it is the
 * view's focal object, not a queued `.fd-rise` child), which is what reads as
 * "the check draws after a short delay": the check's own transition keeps its
 * existing 500ms duration and no added delay (the literal `styles.test.ts`
 * pins for it are unchanged per this story's own instruction), but the disc's
 * ~520ms scale-in means the check is not legible until partway through that
 * motion regardless.
 */
function OutcomeIcon({done}: {readonly done: boolean}) {
    const [drawn, setDrawn] = useState(false)

    useEffect(() => {
        if (!done) return
        setDrawn(true)
    }, [done])

    return (
        <span
            className={`fd-outcome__icon ${done ? 'fd-outcome__icon--done' : 'fd-outcome__icon--error'}`}
            aria-hidden="true"
        >
            {done ? (
                <svg
                    className={`fd-outcome__check${drawn ? ' fd-outcome__check--drawn' : ''}`}
                    viewBox="0 0 24 24"
                    aria-hidden="true"
                    focusable="false"
                >
                    <path className="fd-outcome__check-path" d="M5 12.5l4.3 4.3L19 7.5" pathLength={32}/>
                </svg>
            ) : '!'}
        </span>
    )
}

/**
 * The Done receipt: one line, not two cells (Story 9.6 replaces Story 7.4/7.5's
 * grid) -- a kind glyph, the item name, and the wire bytes actually sent,
 * never `metadata.size`. `receipt.bytesSent` is the wire count the terminal
 * `ProgressSnapshot` reported (Story 7.4's retained state); no elapsed-time
 * figure exists here or anywhere else, because no clock is tracked anywhere
 * in the frontend and EXPERIENCE.md forbids a frontend lifecycle timer.
 */
function DoneReceipt({receipt, rise}: {readonly receipt: CompletionReceiptData; readonly rise: CSSProperties}) {
    return (
        // The stagger class/style live directly on this element rather than
        // on a wrapping div: a wrapping block, being itself a non-stretched
        // flex item of .fd-outcome, sizes to its own content and so never
        // actually constrains this pill's max-width: 100% to anything --
        // found rendered at 640px, where a long name pushed the pill 2.9px
        // past the card's own right edge. Making the pill the direct flex
        // item is what lets its max-width resolve against .fd-outcome's own
        // definite width instead of an intermediate box sized to match it.
        <p className="fd-outcome__receipt fd-rise" style={rise}>
            <span className="fd-outcome__receipt-icon" aria-hidden="true">
                {receipt.isDir ? <FolderGlyph/> : <FileGlyph/>}
            </span>
            <bdi className="fd-outcome__receipt-name" dir="auto">{receipt.name}</bdi>
            <span className="fd-outcome__receipt-meta">
                {copy.label.metaSeparator}{formatBytes(receipt.bytesSent)}
            </span>
        </p>
    )
}

/**
 * The Error receipt: the failed item's display name alone, when Story 9.2
 * retained one -- never a kind glyph or a byte count, because
 * `RetainedErrorOutcome` carries neither (a terminal error retains the name
 * only; a Stage-time command failure retains nothing and never reaches this
 * component with a name at all, since the caller only renders this element
 * when `outcome.itemName` is present).
 */
function ErrorReceipt({name, rise}: {readonly name: string; readonly rise: CSSProperties}) {
    return (
        <p className="fd-outcome__receipt fd-rise" style={rise}>
            <bdi className="fd-outcome__receipt-name" dir="auto">{name}</bdi>
        </p>
    )
}

/** Decorative file-kind glyph beside the Done receipt's name (Story 9.6). */
function FileGlyph() {
    return (
        <svg
            className="fd-outcome__receipt-glyph"
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

/** Decorative folder-kind glyph beside the Done receipt's name (Story 9.6). */
function FolderGlyph() {
    return (
        <svg
            className="fd-outcome__receipt-glyph"
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

/** Decorative refresh glyph on Try Again (Story 9.6). */
function RefreshGlyph() {
    return (
        <svg
            className="fd-button__glyph"
            viewBox="0 0 16 16"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.8"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden="true"
            focusable="false"
        >
            <path d="M13 8a5 5 0 1 1-1.5-3.6"/>
            <path d="M13 2.5v2.5h-2.5"/>
        </svg>
    )
}
