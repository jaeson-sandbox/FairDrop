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
 * Both `IdleView`'s "Troubleshooting" disclosure and `StagedView`'s "Trouble
 * connecting?" disclosure (Story 9.4) sink this same content into a
 * `Disclosure`, so it stays a plain fragment rather than an element of its
 * own -- two callers rendering it side by side would otherwise both carry
 * whatever class this component chose.
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
 * Staged's own "Trouble connecting?" content (Story 9.4): the local-copy
 * disclosure and the link-preview caveat -- both moved out of the
 * always-visible card into this disclosure -- prepended to the same
 * firewall/receiver guidance `IdleView`'s "Troubleshooting" disclosure shows.
 * A separate component rather than added paragraphs on `RecoveryHelpContent`
 * itself, so Idle's own disclosure keeps exactly the content it already had
 * (`IdleView.test.tsx`'s existing assertions are unchanged) and this story's
 * two new lines land only where the acceptance criteria put them.
 */
export function StagedHelpContent() {
    return (
        <>
            <p className="fd-body">{copy.localCopy.disclosure}</p>
            <p className="fd-body">{copy.firstOpener.previews}</p>
            <RecoveryHelpContent/>
        </>
    )
}
