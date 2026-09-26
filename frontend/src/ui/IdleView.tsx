import type {CSSProperties} from 'react'
import {selectCommandError} from '../transfer/selectors'
import type {IdleTransferState} from '../transfer/state'
import {BrowseControl} from './BrowseControl'
import {Disclosure} from './Disclosure'
import {OutcomePanel, type OutcomeCardProps} from './OutcomePanel'
import {RecoveryHelpContent} from './RecoveryHelp'
import {copy} from './copy'

/**
 * Story 9.1's per-child stagger, expressed as a typed style rather than a
 * bare object literal at each call site: `--fd-stagger` is a custom
 * property, which `CSSProperties` does not itself declare, so every caller
 * would otherwise need its own `as CSSProperties` cast. `.fd-rise`'s own
 * CSS reads this from style.css.
 */
function rise(step: number): CSSProperties {
    return {'--fd-stagger': step} as CSSProperties
}

/**
 * DESIGN.md's stated fallback for the FR23 amendments ("Rebuild Idle", Story
 * 7.3; reordered again by Story 7.8): the firewall preflight is collapsed by
 * default inside a keyboard-operable disclosure rather than fully expanded.
 * If acceptance later decides FR23 means the preflight must be *visible*,
 * not merely *present*, this is the one flag that restores that: flip it to
 * `true` and nothing else about the treatment changes.
 */
const FIREWALL_DISCLOSURE_DEFAULT_OPEN = false

interface IdleViewProps {
    readonly state: IdleTransferState
    /** The inherited Wails drop gate, owned by App so the boundary stays in one place. */
    readonly dropTargetStyle: CSSProperties
    /**
     * Whether this Idle was reached by a cancellation winning its race.
     *
     * It cannot be read from the state: a cancel-winning reset lands on plain
     * Idle, which is also how the app starts. App owns the transition, so App
     * owns this flag, and the reducer keeps its two retained-outcome kinds.
     */
    readonly cancelWon: boolean
    readonly onSelectFile: () => void
    readonly onSelectDirectory: () => void
    /**
     * Story 9.6: the Stage-time command-failure card's own action wiring
     * (dropTargetStyle, onDismiss, browse/onRetry, busy) -- built by App using
     * the same logic it applies to its own top-level outcome slot, since a
     * command failure is one of this component's three "one card" shapes.
     * Computed unconditionally by the caller; only read here when
     * `selectCommandError(state)` is non-null.
     */
    readonly commandErrorPanelProps: OutcomeCardProps
}

/**
 * Idle: the cancellation summary, the drop target (glyph, heading, promise
 * line and the one browse control, in that order -- Story 9.3), a command
 * failure, and the grouped disclosure list (firewall preflight then
 * troubleshooting), in that document order.
 *
 * The drop instruction leads because it is this region's `h1`. Story 7.8 put
 * the browse control -- the one control that does something -- ahead of both
 * informational disclosures: FR23 no longer binds the preflight to precede
 * the selection control (see DESIGN.md's "FR23 and the disclosures", second
 * amendment), so the ordering rule here is "the control that acts leads",
 * not "firewall guidance leads". Story 9.3 moves the control a second time,
 * from a full-width row below the drop zone to a centred pill *inside* it --
 * see DESIGN.md's Browse Control row and the same section's Story 9.3
 * addendum for why that does not reopen the FR23 question.
 *
 * A retained terminal outcome is not rendered here. App owns it, above this
 * region, so that reset keeps the identical DOM node rather than rebuilding one
 * that merely says the same thing.
 *
 * Story 9.6: a Stage-time command failure now **replaces** this whole
 * composition rather than rendering inside it -- the drop zone, the browse
 * pill and the grouped disclosure list are not rendered while it shows, and
 * the card itself carries `--wails-drop-target: drop` (via
 * `commandErrorPanelProps.dropTargetStyle`) so a native drop on it stages
 * that item exactly as dropping on the zone does. This mirrors what App does
 * for a retained outcome one level up: both are "one card replaces Idle",
 * just at two different points the same composition can be interrupted.
 */
export function IdleView({
    state,
    dropTargetStyle,
    cancelWon,
    onSelectFile,
    onSelectDirectory,
    commandErrorPanelProps,
}: IdleViewProps) {
    const commandError = selectCommandError(state)

    if (commandError !== null) {
        return (
            <div className="fd-region" data-phase-view="idle">
                <OutcomePanel
                    outcome={{kind: 'error', retained: false, error: commandError}}
                    focusTarget="command-error"
                    {...commandErrorPanelProps}
                />
            </div>
        )
    }

    return (
        <div className="fd-region" data-phase-view="idle">
            <section className="fd-idle">
                {/*
                  The cancel-winning summary, first in the region.

                  It leads because it is the answer to what just happened, and
                  because a retained Done or Error already renders above this
                  view from the shell -- an outcome that appeared under the
                  browse control was the odd one out.

                  Warning, not error. The spine's rule for `cancelled` is
                  "return to Idle; never render as Error", and `--color-error`
                  is the error language here: it appears on nothing but the
                  Error Panel. Amber says "this stopped" without calling a
                  deliberate action a failure. It is a focus target and nothing
                  else -- no live region, because focus owns this transition.
                */}
                {cancelWon ? (
                    <div
                        className="fd-cancel-summary"
                        tabIndex={-1}
                        data-focus-target="cancel-summary"
                    >
                        <span className="fd-cancel-summary__icon" aria-hidden="true">&times;</span>
                        <p className="fd-cancel-summary__text">{copy.cancel.won}</p>
                    </div>
                ) : null}

                {/*
                  A drop target that also frames the one browse control. The
                  outer card itself still carries no click handler and no tab
                  stop of its own -- the browse control nested inside it (Story
                  9.3) is the whole pointer and keyboard path to both choosers.

                  It used to open the file chooser on click, added when only
                  files could be sent. Once folders worked that shortcut
                  contradicted the instruction it sat under -- "file or folder"
                  -- by opening a picker that can only choose a file, and a live
                  run went straight into it. A native chooser is one kind or the
                  other, so the honest click target is the one labelled control,
                  which opens a menu rather than assuming a kind itself. Story
                  7.8 put that control ahead of both disclosures below (the
                  useful control leads, not the firewall preflight); Story 9.3
                  moves it a second time, from a full-width row after this
                  card to a centred pill inside it.
                */}
                <div
                    className="fd-drop-zone"
                    style={dropTargetStyle}
                >
                    {/*
                      The concentric inner rule (DESIGN.md, Shapes): a 24px
                      card ({rounded.xxl}) padded 7px in, so the inner dashed
                      boundary resolves to an 18px radius ({rounded.xl}) --
                      the parent's radius minus the inset between them, not an
                      independently chosen value.

                      Story 9.3: the browse control now renders *inside* this
                      inner wrapper, after the glyph, the heading and one
                      line of promise copy -- glyph, heading, promise,
                      control, in that order, matching the owner-approved
                      prototype's `t-idle` template. Each child carries
                      `.fd-rise` with its own `--fd-stagger` step (Story
                      9.1's per-child entrance), so the four arrive as a
                      staggered group rather than all at once.
                    */}
                    <div className="fd-drop-zone__inner">
                        <div className="fd-drop-symbol fd-rise" style={rise(0)} aria-hidden="true">↓</div>
                        <h1
                            className="fd-state-heading fd-rise"
                            style={rise(1)}
                            tabIndex={-1}
                            data-focus-target="idle-instruction"
                        >
                            {copy.idle.instruction}
                        </h1>
                        {/*
                          Story 9.3: the Idle-only short promise line
                          (`copy.idle.promise`), not `copy.external.promise`
                          -- that key keeps its longer wording for external
                          use (README, store copy) and is no longer rendered
                          here.
                        */}
                        <p className="fd-meta fd-rise" style={rise(2)}>{copy.idle.promise}</p>
                        <div className="fd-rise" style={rise(3)}>
                            <BrowseControl
                                label={copy.label.chooseFileOrFolder}
                                onSelectFile={onSelectFile}
                                onSelectDirectory={onSelectDirectory}
                            />
                        </div>
                    </div>
                </div>

                {/*
                  Story 9.3: the two Idle disclosures are now one grouped
                  list -- a single {rounded.xl} surface, the two rows
                  divided by a separator -- rather than two separately
                  carded disclosures. `Disclosure.tsx` itself is unchanged
                  (its keyboard, focus and content guarantees are Story
                  9.1's, proven there); this wrapper only restyles the
                  surface and rule between the two rows it now contains. See
                  `.fd-idle-disclosures` in style.css.
                */}
                <div className="fd-idle-disclosures fd-rise" style={rise(4)}>
                    {/*
                      FR23 amendment (Story 7.3, reordered by Story 7.8):
                      still present on first paint, but collapsed by default
                      inside a keyboard-operable disclosure whose summary
                      names the topic, and now rendered after the browse
                      control rather than before it. See
                      FIREWALL_DISCLOSURE_DEFAULT_OPEN above for the
                      visibility fallback.
                    */}
                    <Disclosure
                        className="fd-preflight"
                        headingId="fd-firewall-heading"
                        summary={copy.label.firewallHeading}
                        defaultOpen={FIREWALL_DISCLOSURE_DEFAULT_OPEN}
                    >
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
                    </Disclosure>

                    {/*
                      The second disclosure (Story 7.3), relabelled
                      "Troubleshooting" in Story 9.3. Every string
                      RecoveryHelpContent rendered when this block was
                      always open is still rendered here -- none dropped,
                      only collapsed behind a keyboard-operable summary.
                    */}
                    <Disclosure
                        className="fd-help"
                        headingId="fd-recovery-heading"
                        summary={copy.label.recoveryHeading}
                    >
                        <RecoveryHelpContent/>
                    </Disclosure>
                </div>
            </section>
        </div>
    )
}
