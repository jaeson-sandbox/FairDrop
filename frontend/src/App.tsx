import {useEffect, useRef, useState} from 'react'
import type {CSSProperties, ReactElement} from 'react'
import {OnFileDrop, OnFileDropOff} from '../wailsjs/runtime/runtime'
import {selectOutcome, selectProgressSnapshot} from './transfer/selectors'
import {createInitialTransferState, type IdleTransferState, type TransferState} from './transfer/state'
import {useTransfer, type TransferController} from './transfer/useTransfer'
import {focusSelector, routeTransition, type FocusTarget} from './ui/announce'
import {nextProgressSpeech, type ProgressSpeechMemory} from './ui/progressSpeech'
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
    const rootRef = useRef<HTMLElement | null>(null)
    const previousStateRef = useRef<TransferState>(transfer.state)
    const speechRef = useRef<ProgressSpeechMemory | null>(null)

    // The announcer's whole content. Replaced, never appended: an atomic polite
    // region that accumulated text would be the event log the spine forbids.
    const [announcement, setAnnouncement] = useState('')

    // A cancel-winning reset lands on plain Idle, which is also the state the
    // app starts in, so the summary cannot be derived from the reducer. The
    // transition is the evidence, and App is where transitions are observed.
    const [cancelWon, setCancelWon] = useState(false)

    // Requested here, performed by the effect below -- after the render that
    // this transition's own view (the cancellation summary, for one) is part of.
    const [focusRequest, setFocusRequest] = useState<{readonly target: FocusTarget} | null>(null)

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

    /*
      One transition in, one owner out.

      This runs after every commit and compares the state it last saw with the
      state now rendered. `routeTransition` is the whole routing table, so the
      only decisions here are mechanical: write the announcer, or ask for the
      one focus move, or -- for a progress snapshot, the single transition the
      table hands to a throttle -- consult the throttle.
    */
    useEffect(() => {
        const previous = previousStateRef.current
        const next = transfer.state
        if (previous === next) return
        previousStateRef.current = next

        // Leaving Transferring cancels progress speech: a terminal outcome must
        // not be followed by a queued update about the transfer that ended.
        if (next.phase !== 'transferring') speechRef.current = null

        const routed = routeTransition(previous, next)
        setCancelWon(routed !== null && routed.row === 'cancel-won')

        if (routed === null) {
            speakProgress(previous, next)
            return
        }

        if (routed.owner === 'announcer') {
            setAnnouncement(routed.text)
            return
        }

        // A focus-owned transition is announced by the focus move alone, so the
        // announcer is emptied rather than left holding an older message.
        setAnnouncement('')
        setFocusRequest({target: routed.target})
    })

    useEffect(() => {
        if (focusRequest === null) return
        setFocusRequest(null)

        // Proven, not assumed: a cancellation rendered as Idle has no outcome
        // panel, and a view may legitimately not carry the target the table
        // names. The transition still completes; only the focus call is skipped.
        const target = rootRef.current?.querySelector<HTMLElement>(focusSelector(focusRequest.target)) ?? null
        if (target !== null) target.focus()
    }, [focusRequest])

    function speakProgress(previous: TransferState, next: TransferState): void {
        if (next.phase !== 'transferring' || next.progress === null) return
        if (previous.phase === 'transferring' && previous.progress === next.progress) return

        const speech = nextProgressSpeech(
            speechRef.current,
            selectProgressSnapshot(next.progress),
            Date.now(),
        )
        if (speech === null) return

        speechRef.current = speech.memory
        setAnnouncement(speech.text)
    }

    /*
      The lifecycle outcome is rendered here, above the phase body, and stays in
      that one slot across the reset that retains it.

      That is what makes "the retained node stays mounted" true of the DOM and
      not just of the copy: React keeps the same <section> because it is the
      same element type in the same position, so focus that was moved to the
      terminal panel is still on it afterwards. Rendering it from inside
      IdleView instead would unmount and rebuild it, dropping focus to the
      document body silently -- a second focus move by omission, on the one row
      whose owner is None.
    */
    const outcome = selectOutcome(transfer.state)
    const terminal = transfer.state.phase === 'done' || transfer.state.phase === 'error'

    return (
        <main className="fd-app" data-transfer-phase={transfer.state.phase} ref={rootRef}>
            {outcome === null ? null : (
                <OutcomePanel
                    outcome={outcome}
                    level={1}
                    phaseView={terminal}
                    focusTarget="outcome"
                    onDismiss={outcome.retained ? transfer.dismissRetained : undefined}
                />
            )}
            {phaseBody(transfer, cancelWon, setAnnouncement)}
            <div className="fd-status-announcer" role="status" aria-live="polite" aria-atomic="true">
                {announcement}
            </div>
        </main>
    )
}

/**
 * Exactly one view per phase, chosen by the reducer's own discriminator.
 *
 * No branch consults a path, a promise, elapsed time, or an animation: the
 * phase field is the only input, so the screen cannot disagree with the state
 * machine that Story 1.8 defended.
 *
 * A terminal phase has no body of its own: the outcome panel above is its whole
 * view, which is exactly why the panel survives the reset that puts Idle here.
 */
function phaseBody(
    transfer: TransferController,
    cancelWon: boolean,
    announce: (text: string) => void,
): ReactElement | null {
    const {state} = transfer

    switch (state.phase) {
        case 'idle':
            return idleView(transfer, state, cancelWon)

        case 'pending':
            return <StagePendingCard state={state} onCancel={() => void transfer.cancel()}/>

        case 'staged':
            return <StagedView state={state} onCancel={() => void transfer.cancel()} onAnnounce={announce}/>

        case 'transferring':
            return <TransferringView state={state} onCancel={() => void transfer.cancel()}/>

        case 'done':
        case 'error':
            // The selector, not this switch, decides an outcome is renderable:
            // it is what refuses to dress a cancellation up as an Error. When it
            // refuses, the spine's rule is "return to Idle; never render as
            // Error" -- rendering nothing would be neither, leaving a window
            // with no heading, no drop target and no control. The routing table
            // sends that transition to the Idle cancellation summary, so Idle is
            // rendered here with the summary showing.
            return selectOutcome(state) === null
                ? idleView(transfer, createInitialTransferState(), cancelWon)
                : null
    }
}

function idleView(
    transfer: TransferController,
    state: IdleTransferState,
    cancelWon: boolean,
): ReactElement {
    return (
        <IdleView
            state={state}
            dropTargetStyle={dropTargetStyle}
            cancelWon={cancelWon}
            onSelectFile={() => void transfer.selectFile()}
            onSelectDirectory={() => void transfer.selectDirectory()}
        />
    )
}

export default App
