import {useEffect} from 'react'
import type {CSSProperties, ReactElement} from 'react'
import {OnFileDrop, OnFileDropOff} from '../wailsjs/runtime/runtime'
import {selectOutcome} from './transfer/selectors'
import {createInitialTransferState, type IdleTransferState} from './transfer/state'
import {useTransfer, type TransferController} from './transfer/useTransfer'
import {IdleView} from './ui/IdleView'
import {OutcomePanel} from './ui/OutcomePanel'
import {StagePendingCard} from './ui/StagePendingCard'
import {StagedView} from './ui/StagedView'
import {TransferringView} from './ui/TransferringView'

// Wails gates native file drops on a CSS custom property rather than a class.
// The property inherits, so every descendant of the zone is a valid drop point.
const dropTargetStyle = {'--wails-drop-target': 'drop'} as CSSProperties

function App() {
    const transfer = useTransfer()

    useEffect(() => {
        // useDropTarget=true: only fire when the drop lands inside the zone.
        OnFileDrop((_x, _y, dropped) => {
            if (!Array.isArray(dropped) || dropped.length !== 1 ||
                typeof dropped[0] !== 'string' || dropped[0].trim() === '') {
                transfer.rejectSelection()
                return
            }
            void transfer.stage(dropped[0], 'unknown')
        }, true)
        return () => OnFileDropOff()
    }, [transfer.rejectSelection, transfer.stage])

    return (
        <main className="fd-app" data-transfer-phase={transfer.state.phase}>
            {phaseView(transfer)}
            <div className="fd-status-announcer" role="status" aria-live="polite" aria-atomic="true"/>
        </main>
    )
}

/**
 * Exactly one view per phase, chosen by the reducer's own discriminator.
 *
 * No branch consults a path, a promise, elapsed time, or an animation: the
 * phase field is the only input, so the screen cannot disagree with the state
 * machine that Story 1.8 defended.
 */
function phaseView(transfer: TransferController): ReactElement {
    const {state} = transfer

    switch (state.phase) {
        case 'idle':
            return idleView(transfer, state)

        case 'pending':
            return <StagePendingCard state={state} onCancel={() => void transfer.cancel()}/>

        case 'staged':
            return <StagedView state={state} onCancel={() => void transfer.cancel()}/>

        case 'transferring':
            return <TransferringView state={state} onCancel={() => void transfer.cancel()}/>

        case 'done':
        case 'error': {
            // The selector, not this switch, decides an outcome is renderable:
            // it is what refuses to dress a cancellation up as an Error.
            const outcome = selectOutcome(state)
            if (outcome !== null) return <OutcomePanel outcome={outcome} level={1}/>

            // Only a cancellation is refused, and the spine's rule for one is
            // "return to Idle; never render as Error". Rendering nothing would
            // be neither: it leaves a window with no heading, no drop target
            // and no control, which is the worst reachable screen in the app.
            return idleView(transfer, createInitialTransferState())
        }
    }
}

function idleView(transfer: TransferController, state: IdleTransferState): ReactElement {
    return (
        <IdleView
            state={state}
            dropTargetStyle={dropTargetStyle}
            onSelectFile={() => void transfer.selectFile()}
            onSelectDirectory={() => void transfer.selectDirectory()}
            onDismissRetained={transfer.dismissRetained}
        />
    )
}

export default App
