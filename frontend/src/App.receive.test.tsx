import {act} from 'react'
import {cleanup, fireEvent, screen} from '@testing-library/react'
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {
    mountWith as harnessMountWith,
    transitionTo as harnessTransitionTo,
    type ControllerCommands,
} from './App.harness'
import type {TransferState} from './transfer/state'
import type {ProgressSnapshot, ReceiveMetadata} from './transfer/types'

/*
  The receive flow at the App boundary (spec-phone-to-desktop-receiving,
  Story B): which view each phase renders, where focus goes on each transition,
  what the announcer says and when it stays silent, and that sending and
  receiving never leak into one another.

  Like App.test.tsx the controller is a stub (App.harness.tsx), so every state
  is reached by handing App a state, never by driving the real reducer -- the
  reducer and the controller have their own suites. The routing table is the
  real one: this file is the proof that App obeys it.
*/

const mocks = vi.hoisted(() => ({
    onFileDrop: vi.fn(),
    onFileDropOff: vi.fn(),
    copyToClipboard: vi.fn(),
    useTransfer: vi.fn(),
    noop: vi.fn(),
    receive: vi.fn(),
    showReceivedFolder: vi.fn(),
    cancel: vi.fn(),
    dismissRetained: vi.fn(),
    stageFromOutcome: vi.fn(),
    reportCopyFailure: vi.fn(),
}))

vi.mock('../wailsjs/runtime/runtime', () => ({OnFileDrop: mocks.onFileDrop, OnFileDropOff: mocks.onFileDropOff}))
vi.mock('./transfer/useTransfer', () => ({useTransfer: mocks.useTransfer}))
vi.mock('../wailsjs/go/main/App', () => ({CopyToClipboard: mocks.copyToClipboard}))

const sessionId = '0102030405060708090a0b0c0d0e0f10'
const qrPNG = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII='

const receiveMetadata: ReceiveMetadata = {
    sessionId,
    destination: 'Phone Photos',
    url: 'http://192.168.1.50:45678/upload/1112131415161718191a1b1c1d1e1f20',
    qrBase64: qrPNG,
    warnings: [],
}

const commands: ControllerCommands = {
    stage: mocks.noop,
    selectFile: mocks.noop,
    selectDirectory: mocks.noop,
    receive: mocks.receive,
    showReceivedFolder: mocks.showReceivedFolder,
    cancel: mocks.cancel,
    rejectSelection: mocks.noop,
    reportCopyFailure: mocks.reportCopyFailure,
    dismissRetained: mocks.dismissRetained,
    retry: mocks.noop,
    sendAgain: mocks.noop,
    stageFromOutcome: mocks.stageFromOutcome,
    selectFromOutcome: mocks.noop,
    canRetry: false,
    canSendAgain: false,
    draft: null,
    openDraft: mocks.noop,
    appendDraft: mocks.noop,
    selectDraftFiles: mocks.noop,
    selectDraftFolder: mocks.noop,
    removeDraftItem: mocks.noop,
    cancelDraft: mocks.noop,
    sendDraft: mocks.noop,
}

function mountWith(state: TransferState) {
    return harnessMountWith(mocks.useTransfer, state, commands)
}

function transitionTo(view: ReturnType<typeof mountWith>, state: TransferState) {
    harnessTransitionTo(mocks.useTransfer, view, state, commands)
}

beforeEach(() => {
    for (const mock of Object.values(mocks)) mock.mockReset()
    mocks.receive.mockResolvedValue('proceeded')
    mocks.showReceivedFolder.mockResolvedValue(undefined)
    mocks.cancel.mockResolvedValue(undefined)
    mocks.copyToClipboard.mockResolvedValue(undefined)
})
afterEach(cleanup)

const idle: TransferState = {phase: 'idle', retainedOutcome: null, commandError: null}

function pending(): TransferState {
    return {phase: 'pending', generation: 1, itemKind: 'receive', cancelPending: false}
}

function waiting(overrides: Record<string, unknown> = {}): TransferState {
    return {
        phase: 'receive-waiting', session: {sessionId, lastSeq: 0}, metadata: receiveMetadata,
        cancelPending: false, commandError: null, refusals: 0, ...overrides,
    } as TransferState
}

function snapshot(bytesSent: number): ProgressSnapshot {
    return {bytesSent, totalBytes: 400, totalKnown: true, percent: bytesSent / 4, speedBytesPerSec: 1000}
}

function receiving(overrides: Record<string, unknown> = {}): TransferState {
    return {
        phase: 'receive-receiving', session: {sessionId, lastSeq: 1}, metadata: receiveMetadata,
        progress: null, filesSaved: 0, cancelPending: false, commandError: null, ...overrides,
    } as TransferState
}

function outcome(kind: 'complete' | 'incomplete' | 'cancelled', overrides: Record<string, unknown> = {}): TransferState {
    return {
        phase: 'receive-outcome', session: {sessionId, lastSeq: 4}, destination: 'Phone Photos',
        outcome: {kind, filesSaved: 3, subfolderExists: true, markingWarning: false, ...overrides},
        commandError: null,
    } as TransferState
}

function announcer(): string {
    return (document.querySelector('[role="status"]') as HTMLElement).textContent ?? ''
}

function focusTarget(): string | null {
    return (document.activeElement as HTMLElement | null)?.getAttribute('data-focus-target') ?? null
}

function phaseViews(): string[] {
    return [...document.querySelectorAll('[data-phase-view]')].map((element) => element.getAttribute('data-phase-view') ?? '')
}

describe('one view per receive phase', () => {
    it.each([
        ['pending', pending(), 'pending'],
        ['receive-waiting', waiting(), 'receive-waiting'],
        ['receive-receiving', receiving(), 'receive-receiving'],
        ['receive-outcome', outcome('complete'), 'receive-outcome'],
    ] as const)('renders exactly the %s view and marks the phase on the shell', (phase, state, view) => {
        mountWith(state)

        expect(phaseViews()).toEqual([view])
        expect(screen.getByRole('main').getAttribute('data-transfer-phase')).toBe(phase)
    })

    it('renders no send outcome card and no drop zone for any receive phase', () => {
        for (const state of [waiting(), receiving(), outcome('complete'), outcome('incomplete'), outcome('cancelled')]) {
            const {unmount} = mountWith(state)
            expect(document.querySelector('.fd-drop-zone'), state.phase).toBeNull()
            expect(document.querySelector('[data-outcome]'), state.phase).toBeNull()
            unmount()
        }
    })

    it('carries no native drop target in any receive view, so a drop there reaches nothing', () => {
        mountWith(waiting())
        expect(mocks.onFileDrop).toHaveBeenCalled()
        for (const element of document.querySelectorAll<HTMLElement>('*')) {
            expect(element.style.getPropertyValue('--wails-drop-target'), element.tagName).toBe('')
        }
    })
})

describe('the Receive entry', () => {
    it('is offered in Idle and starts a receive through the controller', async () => {
        mountWith(idle)

        await act(async () => { fireEvent.click(screen.getByRole('button', {name: 'Receive'})) })

        expect(mocks.receive).toHaveBeenCalledTimes(1)
    })

    it('returns focus to Receive after a dismissed chooser', async () => {
        mocks.receive.mockResolvedValue('dismissed')
        mountWith(idle)
        const receive = screen.getByRole('button', {name: 'Receive'})

        await act(async () => { fireEvent.click(receive, {detail: 0}) })

        expect(document.activeElement).toBe(receive)
        expect(announcer()).toBe('')
    })

    it('is not offered over a retained send outcome, which replaces Idle', () => {
        mountWith({
            phase: 'idle', commandError: null,
            retainedOutcome: {kind: 'done', receipt: {name: 'report.pdf', isDir: false, bytesSent: 10}},
        })
        expect(screen.queryByRole('button', {name: 'Receive'})).toBeNull()
    })
})

describe('receive transitions: one owner each', () => {
    it('moves focus to the waiting heading when the session opens, with a silent announcer', () => {
        const view = mountWith(pending())
        // Pending is itself focus-owned; the next transition is the one under test.
        transitionTo(view, waiting())

        expect(focusTarget()).toBe('receive-waiting-heading')
        expect(screen.getByRole('heading', {level: 1, name: 'Ready to receive'})).toBe(document.activeElement)
        expect(announcer()).toBe('')
    })

    it('focuses the pending heading when a receive starts', () => {
        const view = mountWith(idle)
        transitionTo(view, pending())

        expect(focusTarget()).toBe('pending-heading')
        expect(announcer()).toBe('')
    })

    it('speaks a refused upload once, in the announcer, without moving focus', () => {
        const view = mountWith(waiting())
        const focused = document.activeElement

        transitionTo(view, waiting({refusals: 1, session: {sessionId, lastSeq: 1}}))

        expect(announcer()).toContain('A phone tried to upload more than this computer has room for.')
        expect(document.activeElement).toBe(focused)
        expect(screen.getByText('Upload too large')).toBeTruthy()
        expect(phaseViews()).toEqual(['receive-waiting'])
    })

    it('speaks a second refusal again, so it is audible twice', () => {
        const view = mountWith(waiting())
        transitionTo(view, waiting({refusals: 1, session: {sessionId, lastSeq: 1}}))
        const first = document.querySelector('[role="status"] > span')
        transitionTo(view, waiting({refusals: 2, session: {sessionId, lastSeq: 2}}))

        expect(announcer()).toContain('A phone tried to upload')
        expect(document.querySelector('[role="status"] > span')).not.toBe(first)
    })

    it('does not speak a refusal over a pending cancellation', () => {
        const view = mountWith(waiting({cancelPending: true}))
        transitionTo(view, waiting({cancelPending: true, refusals: 1, session: {sessionId, lastSeq: 1}}))

        expect(announcer()).toBe('')
    })

    it('says Canceling when a cancel is requested while waiting, and keeps focus on the control', () => {
        const view = mountWith(waiting())
        const cancel = screen.getByRole('button', {name: 'Cancel'})
        cancel.focus()

        transitionTo(view, waiting({cancelPending: true}))

        expect(announcer()).toBe('Canceling')
        expect(document.activeElement).toBe(screen.getByRole('button', {name: 'Canceling'}))
    })

    it('moves focus to the receiving heading when the phone starts uploading', () => {
        const view = mountWith(waiting())
        transitionTo(view, receiving())

        expect(focusTarget()).toBe('receive-receiving-heading')
        expect(announcer()).toBe('')
    })

    it('does not move focus on progress, and speaks it on the shared throttle', () => {
        const view = mountWith(receiving())
        const focused = document.activeElement

        transitionTo(view, receiving({progress: snapshot(100), filesSaved: 1, session: {sessionId, lastSeq: 2}}))

        expect(document.activeElement).toBe(focused)
        // The first accepted snapshot always speaks: "X of Y · N%".
        expect(announcer()).toBe('100 bytes of 400 bytes · 25%')
    })

    it('moves focus to the outcome when a receive ends, and the announcer stays silent', () => {
        const view = mountWith(receiving({progress: snapshot(400), filesSaved: 3}))
        transitionTo(view, outcome('complete'))

        expect(focusTarget()).toBe('receive-outcome')
        expect(document.activeElement).toBe(document.querySelector('.fd-outcome'))
        expect(announcer()).toBe('')
    })

    it.each(['incomplete', 'cancelled'] as const)('moves focus to a %s outcome the same way', (kind) => {
        const view = mountWith(receiving())
        transitionTo(view, outcome(kind))

        expect(focusTarget()).toBe('receive-outcome')
        expect(announcer()).toBe('')
    })

    it('replaces a waiting QR with an incomplete outcome when the session ends before any phone claimed it', () => {
        const view = mountWith(waiting({refusals: 1}))
        expect(document.querySelector('img')).not.toBeNull()

        transitionTo(view, outcome('incomplete', {filesSaved: 0, subfolderExists: false}))

        expect(phaseViews()).toEqual(['receive-outcome'])
        expect(screen.getByRole('main').getAttribute('data-transfer-phase')).toBe('receive-outcome')
        expect(screen.getByRole('heading', {level: 1}).textContent).toBe('Upload incomplete — no files saved')
        // The QR for a listener that no longer exists is gone, nothing is offered
        // to show, and the one way out is Done.
        expect(document.querySelector('img')).toBeNull()
        expect(screen.queryByRole('button', {name: 'Show in Folder'})).toBeNull()
        expect(screen.getByRole('button', {name: 'Done'})).toBeTruthy()
        expect(focusTarget()).toBe('receive-outcome')
        expect(announcer()).toBe('')
    })

    it('moves focus to the Idle instruction when the outcome is left, as its only owner', () => {
        const view = mountWith(outcome('complete'))
        transitionTo(view, idle)

        expect(focusTarget()).toBe('idle-instruction')
        expect(announcer()).toBe('')
        expect(phaseViews()).toEqual(['idle'])
        // Nothing is retained: there is no card to dismiss and no Send Again.
        expect(screen.queryByRole('button', {name: 'Send Again'})).toBeNull()
        expect(screen.queryByRole('button', {name: 'Done'})).toBeNull()
    })

    it('shows the cancel notification when a waiting receive is cancelled back to Idle', () => {
        const view = mountWith(waiting({cancelPending: true}))
        transitionTo(view, idle)

        expect(focusTarget()).toBe('idle-instruction')
        expect(document.querySelector('.fd-notification')).toBeTruthy()
    })
})

describe('the waiting view at the App boundary', () => {
    it('announces Copied once when the link is copied, while the session is the one on screen', async () => {
        mountWith(waiting())

        await act(async () => { fireEvent.click(screen.getByRole('button', {name: 'Copy Link'})) })

        expect(mocks.copyToClipboard).toHaveBeenCalledWith('http://192.168.1.50:45678/upload/1112131415161718191a1b1c1d1e1f20')
        expect(announcer()).toBe('Copied')
    })

    it('stays silent when the copy resolves after the session moved on', async () => {
        let resolve!: () => void
        mocks.copyToClipboard.mockReturnValue(new Promise<void>((resolvePromise) => { resolve = resolvePromise }))
        const view = mountWith(waiting())

        act(() => { fireEvent.click(screen.getByRole('button', {name: 'Copy Link'})) })
        transitionTo(view, receiving())
        await act(async () => { resolve(); await Promise.resolve() })

        expect(announcer()).toBe('')
    })

    it('reports a failed copy to the controller', async () => {
        mocks.copyToClipboard.mockRejectedValue(new Error('denied'))
        mountWith(waiting())

        await act(async () => { fireEvent.click(screen.getByRole('button', {name: 'Copy Link'})) })

        expect(mocks.reportCopyFailure).toHaveBeenCalledWith(sessionId)
    })

    it('cancels through the controller', () => {
        mountWith(waiting())
        fireEvent.click(screen.getByRole('button', {name: 'Cancel'}))
        expect(mocks.cancel).toHaveBeenCalledTimes(1)
    })

    it('cancels a receiving session through the controller too', () => {
        mountWith(receiving())
        fireEvent.click(screen.getByRole('button', {name: 'Cancel'}))
        expect(mocks.cancel).toHaveBeenCalledTimes(1)
    })
})

describe('the held outcome at the App boundary', () => {
    it('Done leaves through the existing Cancel command, never a new one', () => {
        mountWith(outcome('complete'))

        fireEvent.click(screen.getByRole('button', {name: 'Done'}))

        expect(mocks.cancel).toHaveBeenCalledTimes(1)
        expect(mocks.dismissRetained).not.toHaveBeenCalled()
    })

    it('Show in Folder calls the bound command through the controller', () => {
        mountWith(outcome('complete'))

        fireEvent.click(screen.getByRole('button', {name: 'Show in Folder'}))

        expect(mocks.showReceivedFolder).toHaveBeenCalledTimes(1)
    })

    it('offers no Show in Folder when the subfolder is gone', () => {
        mountWith(outcome('incomplete', {filesSaved: 0, subfolderExists: false}))
        expect(screen.queryByRole('button', {name: 'Show in Folder'})).toBeNull()
        expect(screen.getByRole('button', {name: 'Done'})).toBeTruthy()
    })

    it('offers no send affordance beside any receive outcome, even when the controller could send again', () => {
        mocks.useTransfer.mockReturnValue({state: outcome('complete'), ...commands, canSendAgain: true, canRetry: true})
        harnessMountWith(mocks.useTransfer, outcome('complete'), {...commands, canSendAgain: true, canRetry: true})

        for (const label of ['Send Again', 'Send Another', 'Try Again', 'Choose Another']) {
            expect(screen.queryByRole('button', {name: label}), label).toBeNull()
        }
        // The one secondary entry every outcome and Idle card carries is a send action and must not
        // appear beside a receive outcome either.
        expect(screen.queryByRole('button', {name: 'Send multiple items'})).toBeNull()
    })

    it('says each outcome truthfully through the whole shell', () => {
        const {unmount} = mountWith(outcome('complete', {filesSaved: 1}))
        expect(screen.getByRole('heading', {level: 1}).textContent).toBe('1 file saved')
        unmount()

        const second = mountWith(outcome('incomplete', {filesSaved: 2}))
        expect(screen.getByRole('heading', {level: 1}).textContent).toBe('Upload incomplete — 2 files saved')
        second.unmount()

        mountWith(outcome('cancelled', {filesSaved: 0, subfolderExists: false}))
        expect(screen.getByRole('heading', {level: 1}).textContent).toBe('Upload cancelled — no files saved')
    })

    it('focuses a Show in Folder failure note as a command failure', () => {
        const view = mountWith(outcome('complete'))
        transitionTo(view, {...outcome('complete'), commandError: {code: 'path_not_found', message: 'x'}} as TransferState)

        expect(focusTarget()).toBe('command-error')
        expect(announcer()).toBe('')
    })
})

describe('a failed Receive start', () => {
    const failed = (code: string): TransferState => ({
        phase: 'idle', retainedOutcome: null, commandContext: 'receive',
        commandError: {code, message: 'raw'},
    }) as TransferState

    it('shows receive wording and a folder-chooser action, never the send ones', () => {
        mountWith(failed('path_unsupported'))

        expect(screen.getByRole('heading', {name: 'Can’t save there'})).toBeTruthy()
        expect(screen.getByRole('button', {name: 'Choose Another Folder'})).toBeTruthy()
        expect(screen.queryByRole('button', {name: 'Choose Another'})).toBeNull()
        expect(screen.queryByRole('menu')).toBeNull()
        expect(screen.queryByRole('button', {name: 'Choose File or Folder'})).toBeNull()
    })

    it('says Try Again for a failure that is worth another attempt, and reopens the Receive chooser', async () => {
        mountWith(failed('setup_failed'))

        await act(async () => { fireEvent.click(screen.getByRole('button', {name: 'Try Again'})) })

        expect(mocks.receive).toHaveBeenCalledTimes(1)
        expect(mocks.noop).not.toHaveBeenCalled()
    })

    it('offers only Dismiss for a failure no retry can fix', () => {
        mountWith(failed('cleanup_unconfirmed'))

        expect(screen.getByRole('heading', {name: 'Couldn’t confirm it stopped'})).toBeTruthy()
        expect(screen.queryByRole('button', {name: 'Try Again'})).toBeNull()
        expect(screen.queryByRole('button', {name: 'Choose Another Folder'})).toBeNull()
        fireEvent.click(screen.getByRole('button', {name: 'Dismiss'}))
        expect(mocks.dismissRetained).toHaveBeenCalledTimes(1)
    })

    it('focuses the failure card when a receive start fails', () => {
        const view = mountWith(pending())
        transitionTo(view, failed('path_not_found'))

        expect(focusTarget()).toBe('command-error')
        expect(announcer()).toBe('')
    })

    it('keeps send wording for a send failure of the same code', () => {
        mountWith({phase: 'idle', retainedOutcome: null, commandError: {code: 'path_not_found', message: 'raw'}})

        expect(screen.getByRole('heading', {name: 'Item not found'})).toBeTruthy()
        expect(screen.queryByRole('button', {name: 'Choose Another Folder'})).toBeNull()
    })
})
