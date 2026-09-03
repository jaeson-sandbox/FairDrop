import {selectPendingItemKind} from '../transfer/selectors'
import type {PendingTransferState} from '../transfer/state'
import {copy} from './copy'

interface StagePendingCardProps {
    readonly state: PendingTransferState
    readonly onCancel: () => void
}

/**
 * The local preparation surface for an outstanding `StageTransfer`.
 *
 * It says only that FairDrop is preparing the item on this device. There is no
 * QR, no link, no session control, and no badge claiming the backend reached
 * STAGED -- the command has not been acknowledged yet, and this card is the
 * only thing on screen that is allowed to be true before it is.
 *
 * A native drop supplies a path and nothing else, so its kind is `'unknown'`
 * until metadata arrives. An unknown kind therefore shows no kind tab at all
 * and falls back to the file wording, which is the one this epic's file-only
 * source adapter can actually accept; a folder dropped here fails validation a
 * moment later with its own fixed copy.
 */
export function StagePendingCard({state, onCancel}: StagePendingCardProps) {
    const itemKind = selectPendingItemKind(state)
    const folder = itemKind === 'directory'

    return (
        <div className="fd-region" data-phase-view="pending" data-item-kind={itemKind ?? 'none'}>
            <div>
                {itemKind === 'unknown' ? null : (
                    <span className="fd-packet-tab">{folder ? copy.label.folder : copy.label.file}</span>
                )}
                <section className="fd-pending-card">
                    <h1 className="fd-state-heading" tabIndex={-1} data-focus-target="pending-heading">
                        {folder ? copy.stage.pending.folder : copy.stage.pending.file}
                    </h1>
                    {/*
                      A pending cancellation keeps this control focused and
                      answers a second activation with nothing. `aria-disabled`
                      rather than `disabled`: a disabled button loses focus, and
                      the spine requires the control to keep it while the
                      command is outstanding.
                    */}
                    <button
                        type="button"
                        className="fd-button fd-button--quiet fd-target"
                        aria-disabled={state.cancelPending || undefined}
                        onClick={() => {
                            if (!state.cancelPending) onCancel()
                        }}
                    >
                        {state.cancelPending ? copy.cancel.preparationPending : copy.cancel.preparation}
                    </button>
                </section>
            </div>
        </div>
    )
}
