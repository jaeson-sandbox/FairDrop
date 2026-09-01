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
 * It carries no heading. Every heading in the app is a registered string, and
 * the spine registers none for this block; two self-describing paragraphs and a
 * platform list read correctly without one, and inventing a heading here would
 * be inventing product copy.
 */
export function RecoveryHelp() {
    return (
        <div className="fd-help" data-recovery-help="true">
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
        </div>
    )
}
