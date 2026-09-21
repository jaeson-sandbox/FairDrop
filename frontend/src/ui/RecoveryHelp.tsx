import {copy} from './copy'

/**
 * The recovery surface EXPERIENCE.md keeps available from both Idle and Staged.
 *
 * Two halves, both verbatim from the copy registry:
 *
 *  - Platform firewall recovery, for the case where the OS prompt was denied or
 *    never appeared. FairDrop never predicts, restyles or duplicates that
 *    prompt; this is the after-the-fact instruction for reopening it.
 *  - Receiver help, because a receiver only ever sees a generic browser 404,
 *    423 or 410. The sender cannot diagnose those, so the two registry strings
 *    cover every shape between them: a wrong or expired link, a competing
 *    opener, a changed source, and guest or client isolation on the network.
 *
 * Split into content and a plain wrapper so `IdleView` can sink the same
 * content into a `Disclosure` (Story 7.3) without producing two elements that
 * both carry the `fd-help` class -- the wrapper here keeps that class for
 * `StagedView`, which still renders this always-open, exactly as before.
 */
export function RecoveryHelpContent() {
    return (
        <>
            <dl className="fd-help__platforms">
                <div>
                    <dt>{copy.label.windowsRecovery}</dt>
                    <dd>{copy.firewall.windowsRecovery}</dd>
                </div>
                <div>
                    <dt>{copy.label.macosRecovery}</dt>
                    <dd>{copy.firewall.macosRecovery}</dd>
                </div>
            </dl>
            <p className="fd-body">{copy.help.differentLan}</p>
            <p className="fd-body">{copy.help.receiverHttp}</p>
        </>
    )
}

/**
 * The always-open form `StagedView` renders unchanged.
 *
 * It carries no heading itself. Every heading in the app is a registered
 * string, and the spine registers none for this block; two self-describing
 * paragraphs and a platform list read correctly without one, and inventing a
 * heading here would be inventing product copy. `IdleView`'s disclosure form
 * names its own summary instead -- see `copy.label.recoveryHeading`.
 */
export function RecoveryHelp() {
    return (
        <div className="fd-help" data-recovery-help="true">
            <RecoveryHelpContent/>
        </div>
    )
}
