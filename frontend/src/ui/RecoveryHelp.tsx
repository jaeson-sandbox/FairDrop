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
 * Idle and waiting views render state-appropriate guidance through this same
 * fragment: a waiting code can be refreshed in place, while Idle still
 * describes preparing an item again.
 */
export function RecoveryHelpContent({waiting = false}: {readonly waiting?: boolean}) {
    return (
        <>
            <dl className="fd-help__platforms">
                <div>
                    <dt>{copy.label.windowsRecovery}</dt>
                    <dd>{waiting ? copy.firewall.windowsRecoveryWaiting : copy.firewall.windowsRecovery}</dd>
                </div>
                <div>
                    <dt>{copy.label.macosRecovery}</dt>
                    <dd>{waiting ? copy.firewall.macosRecoveryWaiting : copy.firewall.macosRecovery}</dd>
                </div>
            </dl>
            <p className="fd-body">{waiting ? copy.help.differentLanWaiting : copy.help.differentLan}</p>
            <p className="fd-body">{waiting ? copy.help.receiverHttpWaiting : copy.help.receiverHttp}</p>
        </>
    )
}

/**
 * Staged's own "Trouble connecting?" content (Story 9.4): the local-copy
 * disclosure and the inspection guidance -- both moved out of the
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
            <p className="fd-body">{copy.firstOpener.inspect}</p>
            <RecoveryHelpContent waiting/>
        </>
    )
}
