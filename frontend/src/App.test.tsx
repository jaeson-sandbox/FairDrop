import {StrictMode, act} from 'react'
import type {ReactElement} from 'react'
import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import App from './App'
import type {TransferState} from './transfer/state'
import {progressSpeechIntervalMs} from './ui/progressSpeech'

const mocks = vi.hoisted(() => ({
    onFileDrop: vi.fn(),
    onFileDropOff: vi.fn(),
    copyToClipboard: vi.fn(),
    useTransfer: vi.fn(),
    stage: vi.fn(),
    selectFile: vi.fn(),
    selectDirectory: vi.fn(),
    rejectSelection: vi.fn(),
    cancel: vi.fn(),
    dismissRetained: vi.fn(),
}))

vi.mock('../wailsjs/runtime/runtime', () => ({
    OnFileDrop: mocks.onFileDrop,
    OnFileDropOff: mocks.onFileDropOff,
}))

vi.mock('./transfer/useTransfer', () => ({
    useTransfer: mocks.useTransfer,
}))

vi.mock('../wailsjs/go/main/App', () => ({
    CopyToClipboard: mocks.copyToClipboard,
}))

function zone(): HTMLElement | null {
    return screen.getByRole('heading', {name: 'Drop one file or folder.'}).closest<HTMLElement>('.fd-drop-zone')
}

function isDropTarget(element: Element | null): boolean {
    for (let node = element as HTMLElement | null; node; node = node.parentElement) {
        const value = node.style?.getPropertyValue('--wails-drop-target').trim()
        if (value) return value === 'drop'
    }
    return false
}

function dropOn(target: Element | null, paths: unknown) {
    expect(mocks.onFileDrop).toHaveBeenCalled()
    const callback = mocks.onFileDrop.mock.calls.at(-1)![0]
    if (!isDropTarget(target)) return
    act(() => callback(0, 0, paths))
}

function drop(paths: unknown) {
    dropOn(zone(), paths)
}

beforeEach(() => {
    for (const mock of Object.values(mocks)) mock.mockReset()
    mocks.stage.mockResolvedValue(undefined)
    mocks.cancel.mockResolvedValue(undefined)
    mocks.selectFile.mockResolvedValue(undefined)
    mocks.selectDirectory.mockResolvedValue(undefined)
    mocks.copyToClipboard.mockResolvedValue(undefined)
    mocks.useTransfer.mockReturnValue({
        state: {phase: 'idle', retainedOutcome: null, commandError: null},
        stage: mocks.stage,
        selectFile: mocks.selectFile,
        selectDirectory: mocks.selectDirectory,
        cancel: mocks.cancel,
        rejectSelection: mocks.rejectSelection,
        dismissRetained: mocks.dismissRetained,
    })
})
afterEach(cleanup)

describe('production transfer controller integration', () => {
    it('mounts the production controller once and renders only the Idle view for the idle phase', () => {
        render(<App/>)

        expect(mocks.useTransfer).toHaveBeenCalledTimes(1)
        expect(screen.getByRole('main').getAttribute('data-transfer-phase')).toBe('idle')
        expect(screen.queryByRole('progressbar')).toBeNull()
        expect(screen.queryByRole('img')).toBeNull()
    })

    it('routes exactly one native path through Stage as unknown kind', () => {
        render(<App/>)
        const originalPath = String.raw` C:\private\report.pdf `

        drop([originalPath])

        expect(mocks.stage).toHaveBeenCalledTimes(1)
        expect(mocks.stage).toHaveBeenCalledWith(String.raw` C:\private\report.pdf `, 'unknown')
        expect(mocks.rejectSelection).not.toHaveBeenCalled()
    })

    it.each([
        ['zero paths', []],
        ['multiple paths', [String.raw`C:\one.txt`, String.raw`C:\two.txt`]],
        ['undefined input', undefined],
        ['null input', null],
        ['an object', {path: String.raw`C:\one.txt`}],
        ['a non-string singleton', [null]],
        ['an empty singleton', ['']],
        ['a whitespace-only singleton', ['   ']],
    ])('rejects %s and never stages one silently', (_name, paths) => {
        render(<App/>)

        drop(paths)

        expect(mocks.rejectSelection).toHaveBeenCalledTimes(1)
        expect(mocks.stage).not.toHaveBeenCalled()
    })
})

describe('native drop gate lifecycle', () => {
    it('opts into the Wails drop-target gate and marks the inherited zone property', () => {
        render(<App/>)

        expect(mocks.onFileDrop).toHaveBeenCalledTimes(1)
        expect(mocks.onFileDrop.mock.calls[0][1]).toBe(true)
        expect(zone()?.style.getPropertyValue('--wails-drop-target')).toBe('drop')
        expect(isDropTarget(zone()?.firstElementChild ?? null)).toBe(true)
    })

    it('ignores a drop outside the gated zone', () => {
        render(<App/>)
        const outside = screen.getByRole('heading', {name: 'Local network access'})

        expect(isDropTarget(outside)).toBe(false)
        dropOn(outside, [String.raw`C:\ignored.txt`])

        expect(mocks.stage).not.toHaveBeenCalled()
        expect(mocks.rejectSelection).not.toHaveBeenCalled()
    })

    it('deregisters the native listener on unmount and re-registers on remount', () => {
        const first = render(<App/>)
        expect(mocks.onFileDrop).toHaveBeenCalledTimes(1)

        first.unmount()
        expect(mocks.onFileDropOff).toHaveBeenCalledTimes(1)

        render(<App/>)
        expect(mocks.onFileDrop).toHaveBeenCalledTimes(2)
    })

    it('keeps exactly one live native listener under StrictMode effect replay', () => {
        render(<StrictMode><App/></StrictMode>)

        expect(mocks.onFileDropOff).toHaveBeenCalledTimes(1)
        expect(mocks.onFileDrop).toHaveBeenCalledTimes(2)

        drop([String.raw`C:\report.pdf`])
        expect(mocks.stage).toHaveBeenCalledTimes(1)
    })
})

const sessionId = '0123456789abcdef0123456789abcdef'
const capabilityURL = 'http://192.0.2.1:34123/download/fedcba9876543210fedcba9876543210'
const qrPNG = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII='

const metadata = {
    sessionId,
    name: 'report.pdf',
    size: 100,
    isDir: false,
    url: capabilityURL,
    qrBase64: qrPNG,
    warnings: [],
}

function mountWith(state: TransferState) {
    mocks.useTransfer.mockReturnValue({
        state,
        stage: mocks.stage,
        selectFile: mocks.selectFile,
        selectDirectory: mocks.selectDirectory,
        cancel: mocks.cancel,
        rejectSelection: mocks.rejectSelection,
        dismissRetained: mocks.dismissRetained,
    })
    return render(<App/>)
}

function phaseViews(): string[] {
    return [...document.querySelectorAll('[data-phase-view]')]
        .map((element) => element.getAttribute('data-phase-view') ?? '')
}

// Typed rather than Record<string, unknown>: this is the suite that claims
// "exactly one view per phase", so a reducer field it stopped matching should
// break it rather than leave it green against a stale shape. Declared here, not
// inline, so the annotation actually reaches the object literals.
const phaseCases: Array<[string, TransferState, string]> = [
        ['idle', {phase: 'idle', retainedOutcome: null, commandError: null}, 'idle'],
        ['pending', {phase: 'pending', generation: 1, itemKind: 'file', cancelPending: false}, 'pending'],
        ['staged', {
            phase: 'staged',
            session: {sessionId, lastSeq: 0},
            metadata,
            cancelPending: false,
            commandError: null,
        }, 'staged'],
        ['transferring', {
            phase: 'transferring',
            session: {sessionId, lastSeq: 1},
            metadata,
            progress: null,
            cancelPending: false,
            commandError: null,
        }, 'transferring'],
        ['done', {phase: 'done', session: {sessionId, lastSeq: 4}, outcome: {kind: 'done'}}, 'outcome'],
        ['error', {
            phase: 'error',
            session: {sessionId, lastSeq: 4},
            outcome: {kind: 'error', error: {code: 'transfer_failed', message: 'ignored'}},
        }, 'outcome'],
]

describe('one view per phase', () => {
    it.each(phaseCases)('renders exactly the %s view and marks the phase on the shell', (phase, state, view) => {
        mountWith(state)

        expect(phaseViews()).toEqual([view])
        expect(screen.getByRole('main').getAttribute('data-transfer-phase')).toBe(phase)
    })

    it('shows the cancel-winning summary when a terminal error carries cancelled', () => {
        const view = mountWith({...stagedState, cancelPending: true} as TransferState)

        // Reached by transition, not by mounting: the summary exists only when
        // the routing table has named this transition's owner, and the terminal
        // branch has to forward that on. Rendering Idle without it leaves the
        // one transition in the app with no owner at all.
        transitionTo(view, {
            phase: 'error',
            session: {sessionId, lastSeq: 4},
            outcome: {kind: 'error', error: {code: 'cancelled', message: 'Transfer canceled.'}},
        } as TransferState)

        const summary = document.querySelector('[data-focus-target="cancel-summary"]')
        expect(summary).toBeTruthy()
        expect(document.activeElement).toBe(summary)
        expect(document.querySelector('[data-outcome]')).toBeNull()
    })

    it('gives the terminal outcome the document heading', () => {
        mountWith({phase: 'done', session: {sessionId, lastSeq: 4}, outcome: {kind: 'done'}})

        expect(screen.getByRole('heading', {level: 1}).textContent).toBe('Transfer finished')
    })

    // EXPERIENCE.md gives `cancelled` one rule -- "Return to Idle; never render
    // as Error" -- and rendering nothing satisfies neither half of it. The
    // predecessor of this test asserted an empty window, which locked in a
    // screen with no heading, no drop target and no way forward.
    it('returns to Idle rather than showing a cancellation as an Error', () => {
        mountWith({
            phase: 'error',
            session: {sessionId, lastSeq: 4},
            outcome: {kind: 'error', error: {code: 'cancelled', message: 'Transfer canceled.'}},
        })

        expect(phaseViews()).toEqual(['idle'])
        expect(screen.queryByText('Transfer canceled.')).toBeNull()
        expect(screen.getByRole('heading', {level: 1}).textContent).toBe('Drop one file or folder.')
        expect(document.querySelector('[data-outcome]')).toBeNull()
    })

    it('pre-mounts one atomic polite status region in every phase', () => {
        mountWith({phase: 'idle', retainedOutcome: null, commandError: null})

        const announcers = [...document.querySelectorAll('[role="status"]')]
        expect(announcers).toHaveLength(1)
        expect(announcers[0].getAttribute('aria-live')).toBe('polite')
        expect(announcers[0].getAttribute('aria-atomic')).toBe('true')
        expect(announcers[0].textContent).toBe('')
    })
})

const cancelCases: Array<[string, TransferState]> = [

        ['Cancel preparation', {phase: 'pending', generation: 1, itemKind: 'file', cancelPending: false}],
        ['Cancel', {
            phase: 'staged',
            session: {sessionId, lastSeq: 0},
            metadata,
            cancelPending: false,
            commandError: null,
        }],
        ['Cancel', {
            phase: 'transferring',
            session: {sessionId, lastSeq: 1},
            metadata,
            progress: null,
            cancelPending: false,
            commandError: null,
        }],
]

describe('controller wiring', () => {
    it('routes both browse controls to their own command', () => {
        mountWith({phase: 'idle', retainedOutcome: null, commandError: null})

        fireEvent.click(screen.getByRole('button', {name: 'Select File'}))
        expect(mocks.selectFile).toHaveBeenCalledTimes(1)
        expect(mocks.selectDirectory).not.toHaveBeenCalled()

        fireEvent.click(screen.getByRole('button', {name: 'Select Directory'}))
        expect(mocks.selectDirectory).toHaveBeenCalledTimes(1)
        expect(mocks.stage).not.toHaveBeenCalled()
    })

    it('routes Dismiss to the retained-outcome command', () => {
        mountWith({phase: 'idle', retainedOutcome: {kind: 'done'}, commandError: null})

        fireEvent.click(screen.getByRole('button', {name: 'Dismiss'}))
        expect(mocks.dismissRetained).toHaveBeenCalledTimes(1)
    })

    it.each(cancelCases)('routes %s to the one cancel command', (name, state) => {
        mountWith(state)

        fireEvent.click(screen.getByRole('button', {name}))
        expect(mocks.cancel).toHaveBeenCalledTimes(1)
    })
})

/*
  The routing table, driven through the real views.

  `announce.test.ts` proves which owner each transition gets; these prove the
  App obeys the answer -- that the focus move lands on a node that is really
  there, and that the other mechanism is provably silent on the same transition.
*/

function controllerFor(state: TransferState) {
    return {
        state,
        stage: mocks.stage,
        selectFile: mocks.selectFile,
        selectDirectory: mocks.selectDirectory,
        cancel: mocks.cancel,
        rejectSelection: mocks.rejectSelection,
        dismissRetained: mocks.dismissRetained,
    }
}

function transitionTo(view: {rerender: (ui: ReactElement) => void}, state: TransferState) {
    mocks.useTransfer.mockReturnValue(controllerFor(state))
    act(() => view.rerender(<App/>))
}

function announcer(): HTMLElement {
    return document.querySelector('[role="status"]') as HTMLElement
}

function focused(): string | null {
    return (document.activeElement as HTMLElement | null)?.getAttribute('data-focus-target') ?? null
}

const idleState: TransferState = {phase: 'idle', retainedOutcome: null, commandError: null}
const pendingState: TransferState = {phase: 'pending', generation: 1, itemKind: 'file', cancelPending: false}
const stagedState: TransferState = {
    phase: 'staged',
    session: {sessionId, lastSeq: 0},
    metadata,
    cancelPending: false,
    commandError: null,
}
const transferringState: TransferState = {
    phase: 'transferring',
    session: {sessionId, lastSeq: 1},
    metadata,
    progress: null,
    cancelPending: false,
    commandError: null,
}
const doneState: TransferState = {phase: 'done', session: {sessionId, lastSeq: 4}, outcome: {kind: 'done'}}

const discoveryWarning = 'Device discovery isn’t available. The QR code and download link still work.'

function warned(): TransferState {
    return {
        ...stagedState,
        metadata: {...metadata, warnings: [{code: 'beacon_warning', message: discoveryWarning}]},
    } as TransferState
}

function progressAt(bytesSent: number, percent: number): TransferState {
    return {
        ...transferringState,
        progress: {bytesSent, totalBytes: 100_000_000, totalKnown: true, percent, speedBytesPerSec: 12_400_000},
    } as TransferState
}

describe('focus-owned transitions', () => {
    const focusRows: Array<[string, TransferState, TransferState, string]> = [
        ['Stage pending', idleState, pendingState, 'pending-heading'],
        ['Stage success', pendingState, stagedState, 'staged-heading'],
        ['transfer-started', stagedState, transferringState, 'transferring-heading'],
        ['Complete', transferringState, doneState, 'outcome'],
        [
            'Terminal Error',
            transferringState,
            {
                phase: 'error',
                session: {sessionId, lastSeq: 4},
                outcome: {kind: 'error', error: {code: 'transfer_failed', message: 'ignored'}},
            },
            'outcome',
        ],
        [
            'Validation failure',
            idleState,
            {
                phase: 'idle',
                retainedOutcome: null,
                commandError: {code: 'invalid_selection', message: 'Choose exactly one file or folder.'},
            },
            'command-error',
        ],
        [
            'Cancel-winning reset',
            {...stagedState, cancelPending: true} as TransferState,
            idleState,
            'cancel-summary',
        ],
        [
            'Dismiss retained outcome',
            {phase: 'idle', retainedOutcome: {kind: 'done'}, commandError: null},
            idleState,
            'idle-instruction',
        ],
    ]

    it.each(focusRows)('moves focus once for %s and leaves the announcer silent', (_name, from, to, target) => {
        const view = mountWith(from)

        transitionTo(view, to)

        expect(focused()).toBe(target)
        expect(announcer().textContent).toBe('')
    })

    it('shows the cancellation summary it focuses, and never as an Error', () => {
        const view = mountWith({...stagedState, cancelPending: true} as TransferState)

        transitionTo(view, idleState)

        expect(document.activeElement?.textContent).toBe('Transfer canceled. Ready for another file or folder.')
        expect(document.querySelector('.fd-outcome')).toBeNull()
        expect(document.querySelector('[role="alert"]')).toBeNull()
    })
})

function snapshotAt(percent: number) {
    return {
        bytesSent: Math.round(100_000_000 * percent / 100),
        totalBytes: 100_000_000,
        totalKnown: true,
        percent,
        speedBytesPerSec: 12_400_000,
    }
}

describe('announcer-owned transitions', () => {
    const spokenRows: Array<[string, TransferState, TransferState, string]> = [
        [
            'Cancel requested during preparation',
            pendingState,
            {...pendingState, cancelPending: true} as TransferState,
            'Canceling preparation…',
        ],
        [
            'Cancel requested from Staged',
            stagedState,
            {...stagedState, cancelPending: true} as TransferState,
            'Canceling…',
        ],
        ['beacon_warning', stagedState, warned(), discoveryWarning],
    ]

    it.each(spokenRows)('replaces the announcer for %s and moves no focus', (_name, from, to, text) => {
        const view = mountWith(from)
        const before = document.activeElement

        transitionTo(view, to)

        expect(announcer().textContent).toBe(text)
        expect(document.activeElement).toBe(before)
    })

    it('keeps the pending cancel control focused while its announcement is made', () => {
        const view = mountWith(stagedState)
        const cancel = screen.getByRole('button', {name: 'Cancel'})
        cancel.focus()

        transitionTo(view, {...stagedState, cancelPending: true} as TransferState)

        expect(document.activeElement).toBe(screen.getByRole('button', {name: 'Canceling…'}))
        expect(announcer().textContent).toBe('Canceling…')
    })

    it('replaces the announcer rather than appending to it', () => {
        const view = mountWith(stagedState)

        transitionTo(view, {...stagedState, cancelPending: true} as TransferState)
        expect(announcer().textContent).toBe('Canceling…')

        transitionTo(view, {...warned(), cancelPending: true} as TransferState)

        // Replaced, not accumulated. The region holds one keyed node whose
        // identity changes per announcement -- that node swap is what makes a
        // repeated message audible a second time, since a polite region is only
        // re-read when its content actually changes. What the contract forbids
        // is an event log, so the count must stay at one and the text must be
        // the newest message alone.
        expect(announcer().childElementCount).toBe(1)
        expect(announcer().textContent).toBe(discoveryWarning)
    })

    it('does not announce a copy result once its own view is gone', async () => {
        const view = mountWith(stagedState)
        let resolveCopy!: () => void
        mocks.copyToClipboard.mockReturnValue(new Promise<void>((resolve) => { resolveCopy = resolve }))

        fireEvent.click(screen.getByRole('button', {name: 'Copy download link'}))
        // The transfer starts while the clipboard command is still in flight.
        // That transition is focus-owned; announcing the copy as well would
        // give one moment two owners, which is the rule this story enforces.
        transitionTo(view, transferringState)
        await act(async () => { resolveCopy() })

        expect(announcer().textContent).toBe('')
    })

    it('measures the speech interval on a monotonic clock, not the wall clock', () => {
        /*
          The two clocks are driven to disagree. The monotonic one advances past
          the interval; the wall clock stands still, as it would after an NTP
          correction stepped it backwards and left the remembered timestamp in
          the future. Reading the wall clock mutes assistive progress for the
          rest of the transfer, so the second snapshot must still be spoken.
        */
        let tick = 0
        const monotonic = vi.spyOn(performance, 'now').mockImplementation(() => {
            tick += progressSpeechIntervalMs * 2
            return tick
        })
        const wall = vi.spyOn(Date, 'now').mockReturnValue(1_000)
        const view = mountWith(stagedState)

        try {
            transitionTo(view, transferringState)
            transitionTo(view, {...transferringState, progress: snapshotAt(10)} as TransferState)
            transitionTo(view, {...transferringState, progress: snapshotAt(40)} as TransferState)

            expect(announcer().textContent).toContain('40%')
        } finally {
            monotonic.mockRestore()
            wall.mockRestore()
        }
    })

    it('refuses a copy result that belongs to a session the view has left', async () => {
        const view = mountWith(stagedState)
        let resolveCopy!: () => void
        mocks.copyToClipboard.mockReturnValue(new Promise<void>((resolve) => { resolveCopy = resolve }))

        fireEvent.click(screen.getByRole('button', {name: 'Copy download link'}))
        // Staged again, but a different session: the in-flight command belongs
        // to the one that started it, and the phase alone cannot tell them apart.
        transitionTo(view, {
            ...stagedState,
            session: {sessionId: '11111111111111111111111111111111', lastSeq: 0},
        } as TransferState)
        await act(async () => { resolveCopy() })

        expect(announcer().textContent).toBe('')
    })

    it('re-announces a repeated message instead of going silent', async () => {
        mountWith(stagedState)
        const copy = screen.getByRole('button', {name: 'Copy download link'})

        await act(async () => { fireEvent.click(copy) })
        const first = announcer().firstElementChild
        expect(announcer().textContent).toBe('Copied')

        await act(async () => { fireEvent.click(copy) })

        // Same words both times: without a node swap the live region observes
        // no change and the second copy is announced to nobody.
        expect(announcer().textContent).toBe('Copied')
        expect(announcer().firstElementChild).not.toBe(first)
    })

    it('reports a copy success without moving focus or changing the lifecycle', async () => {
        mountWith(stagedState)
        const copy = screen.getByRole('button', {name: 'Copy download link'})
        copy.focus()

        await act(async () => {
            fireEvent.click(copy)
        })

        expect(announcer().textContent).toBe('Copied')
        expect(document.activeElement).toBe(screen.getByRole('button', {name: 'Copied'}))
        expect(screen.getByRole('main').getAttribute('data-transfer-phase')).toBe('staged')
    })

    it('empties the announcer again on the next focus-owned transition', () => {
        const view = mountWith(stagedState)
        transitionTo(view, {...stagedState, cancelPending: true} as TransferState)
        expect(announcer().textContent).toBe('Canceling…')

        transitionTo(view, {...transferringState, cancelPending: true} as TransferState)

        expect(announcer().textContent).toBe('')
        expect(focused()).toBe('transferring-heading')
    })
})

describe('reset after a terminal outcome', () => {
    it('keeps the same node, keeps focus inside it, and says nothing', () => {
        const view = mountWith(transferringState)
        transitionTo(view, doneState)

        const panel = document.activeElement
        expect(panel).toBe(document.querySelector('[data-outcome="done"]'))

        transitionTo(view, {phase: 'idle', retainedOutcome: {kind: 'done'}, commandError: null})

        // The identical DOM node, still focused: reset is the one row whose
        // owner is None, so neither mechanism may fire and focus may not move.
        expect(document.querySelector('[data-outcome="done"]')).toBe(panel)
        expect(document.activeElement).toBe(panel)
        expect(announcer().textContent).toBe('')
        // Idle is now the phase view underneath the retained node.
        expect(phaseViews()).toEqual(['idle'])
        expect(screen.getByRole('button', {name: 'Dismiss'})).toBeTruthy()
    })

    it('keeps the retained failure on the fixed table with a Dismiss control', () => {
        mountWith({
            phase: 'idle',
            retainedOutcome: {kind: 'error', error: {code: 'transfer_failed', message: 'ignored'}},
            commandError: null,
        })

        expect(screen.getByRole('heading', {name: 'Transfer stopped'})).toBeTruthy()
        expect(screen.getByText('The transfer stopped before FairDrop finished sending. ' +
            'Check the local network and create a fresh link.')).toBeTruthy()
        fireEvent.click(screen.getByRole('button', {name: 'Dismiss'}))
        expect(mocks.dismissRetained).toHaveBeenCalledTimes(1)
    })

    it('shows the retained node above a newer command failure, and both at once', () => {
        mountWith({
            phase: 'idle',
            retainedOutcome: {kind: 'done'},
            commandError: {code: 'invalid_selection', message: 'Choose exactly one file or folder.'},
        })

        const panels = [...document.querySelectorAll('.fd-outcome')]
        expect(panels).toHaveLength(2)
        expect(panels[0].getAttribute('data-retained')).toBe('true')
        expect(panels[1].getAttribute('data-error-code')).toBe('invalid_selection')
    })
})

describe('assistive progress speech', () => {
    it('speaks once at the first accepted snapshot without moving focus', () => {
        const view = mountWith(transferringState)
        const before = document.activeElement

        transitionTo(view, progressAt(1_000_000, 1))

        expect(announcer().textContent).toBe('1.0 MB of 100.0 MB · 1%')
        expect(document.activeElement).toBe(before)
    })

    it('repaints the meter on every snapshot while staying silent between updates', () => {
        vi.useFakeTimers()
        try {
            const view = mountWith(transferringState)
            transitionTo(view, progressAt(1_000_000, 1))

            vi.advanceTimersByTime(1_000)
            transitionTo(view, progressAt(40_000_000, 40))

            // The visual meter followed the snapshot; the announcer did not.
            expect(screen.getByRole('progressbar').getAttribute('aria-valuenow')).toBe('40')
            expect(announcer().textContent).toBe('1.0 MB of 100.0 MB · 1%')

            vi.advanceTimersByTime(5_000)
            transitionTo(view, progressAt(60_000_000, 60))

            expect(announcer().textContent).toBe('60.0 MB of 100.0 MB · 60%')
        } finally {
            vi.useRealTimers()
        }
    })

    it('is cancelled by a terminal outcome, which owns the transition itself', () => {
        vi.useFakeTimers()
        try {
            const view = mountWith(transferringState)
            transitionTo(view, progressAt(1_000_000, 1))
            vi.advanceTimersByTime(60_000)

            transitionTo(view, doneState)

            expect(announcer().textContent).toBe('')
            expect(focused()).toBe('outcome')
        } finally {
            vi.useRealTimers()
        }
    })

    it('never speaks throughput, however many snapshots arrive', () => {
        vi.useFakeTimers()
        try {
            const view = mountWith(transferringState)
            for (let step = 1; step <= 9; step += 1) {
                vi.advanceTimersByTime(6_000)
                transitionTo(view, progressAt(step * 11_000_000, step * 11))
            }

            expect(announcer().textContent).not.toContain('/s')
            expect(announcer().textContent).not.toContain('12.4 MB')
        } finally {
            vi.useRealTimers()
        }
    })
})

describe('progress speech across two transfers', () => {
    it('starts again at the first snapshot of the next transfer, whatever the last one left behind', () => {
        // The throttle's memory is dropped the moment the phase stops being
        // Transferring. Without that, a second transfer beginning within five
        // seconds of the first one's last update inherits its thresholds and
        // says nothing at all at its start -- the one update that is never
        // throttled.
        vi.useFakeTimers()
        try {
            const view = mountWith(transferringState)
            transitionTo(view, progressAt(1_000_000, 1))
            expect(announcer().textContent).toBe('1.0 MB of 100.0 MB · 1%')

            transitionTo(view, doneState)
            transitionTo(view, {phase: 'idle', retainedOutcome: {kind: 'done'}, commandError: null})
            transitionTo(view, stagedState)
            transitionTo(view, transferringState)

            // One second later, one percentage point on, one megabyte on: below
            // both thresholds, and still the first update of a new transfer.
            vi.advanceTimersByTime(1_000)
            transitionTo(view, progressAt(2_000_000, 2))

            expect(announcer().textContent).toBe('2.0 MB of 100.0 MB · 2%')
        } finally {
            vi.useRealTimers()
        }
    })
})

describe('the routing table under StrictMode effect replay', () => {
    // main.tsx wraps App in StrictMode, so every effect runs mount, cleanup,
    // mount in development. The routing effect has no dependency array and no
    // cleanup: what stops it re-announcing is the previous-state ref it writes
    // before it does anything else.
    function strictly(state: TransferState) {
        mocks.useTransfer.mockReturnValue(controllerFor(state))
        return render(<StrictMode><App/></StrictMode>)
    }

    function strictTransitionTo(view: {rerender: (ui: ReactElement) => void}, state: TransferState) {
        mocks.useTransfer.mockReturnValue(controllerFor(state))
        act(() => view.rerender(<StrictMode><App/></StrictMode>))
    }

    it('announces nothing and moves no focus on the first mount', () => {
        strictly(idleState)

        expect(announcer().textContent).toBe('')
        expect(focused()).toBeNull()
    })

    it('still makes exactly one focus move for a focus-owned transition', () => {
        const view = strictly(idleState)

        strictTransitionTo(view, pendingState)

        expect(focused()).toBe('pending-heading')
        expect(announcer().textContent).toBe('')
    })

    it('still leaves focus alone for an announcer-owned transition', () => {
        const view = strictly(stagedState)
        const cancel = screen.getByRole('button', {name: 'Cancel'})
        cancel.focus()

        strictTransitionTo(view, {...stagedState, cancelPending: true} as TransferState)

        expect(announcer().textContent).toBe('Canceling…')
        expect(document.activeElement).toBe(screen.getByRole('button', {name: 'Canceling…'}))
    })
})
