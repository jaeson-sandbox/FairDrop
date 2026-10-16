import {cleanup, screen} from '@testing-library/react'
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {
    mountWith as harnessMountWith,
    transitionTo as harnessTransitionTo,
    type ControllerCommands,
} from './App.harness'
import type {TransferState} from './transfer/state'
import {publicError} from './transfer/errors'
import type {Announcement} from './ui/announce'

/*
  The "prove the target exists" half of the contract.

  Every routing row a reducer transition can actually produce lands on a target
  the matching view really carries, which is what makes this guard impossible to
  exercise from real state -- and exactly why it is worth a test. The routing
  table is stubbed here so a row can name a target that is not on the screen,
  which is the shape a future view edit would take: the row survives, the
  attribute quietly does not.

  Story 10.2, owner-approved: the routing table's `cancel-won` row used to
  target a dedicated `cancel-summary` focus node; that node (and the static
  notification it belonged to) is gone, and the row now targets
  `idle-instruction` instead, described by the sliding notification's own
  text via `aria-describedby` while it is mounted. This file stubs
  `routeTransition` directly and never asserts a specific target string, so
  the row rename needs no edit here -- it is exercised (and pinned) by
  `announce.test.ts` and `App.test.tsx` instead.
*/

const mocks = vi.hoisted(() => ({
    onFileDrop: vi.fn(),
    onFileDropOff: vi.fn(),
    useTransfer: vi.fn(),
    route: vi.fn(),
    noop: vi.fn(),
}))

vi.mock('../wailsjs/runtime/runtime', () => ({
    OnFileDrop: mocks.onFileDrop,
    OnFileDropOff: mocks.onFileDropOff,
}))

vi.mock('./transfer/useTransfer', () => ({useTransfer: mocks.useTransfer}))

vi.mock('../wailsjs/go/main/App', () => ({CopyToClipboard: vi.fn()}))

vi.mock('./ui/announce', async (importOriginal) => {
    const actual = await importOriginal<typeof import('./ui/announce')>()
    return {
        ...actual,
        routeTransition: (previous: TransferState, next: TransferState) => mocks.route(previous, next),
    }
})

function idle(): TransferState {
    return {phase: 'idle', retainedOutcome: null, commandError: null}
}

const doneReceipt = {name: 'report.pdf', isDir: false, bytesSent: 100}

// The mock functions live here; App.harness.tsx only shapes and mounts
// whatever controller it is handed, typed so a new TransferController member
// is a type error in that one shared place rather than a silent gap here.
const commands: ControllerCommands = {
    composer: null,
    openText: mocks.noop,
    editText: mocks.noop,
    cancelText: mocks.noop,
    sendText: mocks.noop,
    stage: mocks.noop,
    selectFile: mocks.noop,
    selectDirectory: mocks.noop,
    receive: mocks.noop,
    showReceivedFolder: mocks.noop,
    cancel: mocks.noop,
    rejectSelection: mocks.noop,
    reportCopyFailure: mocks.noop,
    dismissRetained: mocks.noop,
    retry: mocks.noop,
    sendAgain: mocks.noop,
    stageFromOutcome: mocks.noop,
    selectFromOutcome: mocks.noop,
    canRetry: false,
    canSendAgain: false,
    draft: null,
    draftIds: [],
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

function transitionTo(view: ReturnType<typeof mountWith>, state: TransferState, routed: Announcement | null) {
    mocks.route.mockReturnValue(routed)
    harnessTransitionTo(mocks.useTransfer, view, state, commands)
}

beforeEach(() => {
    for (const mock of Object.values(mocks)) mock.mockReset()
    mocks.route.mockReturnValue(null)
})
afterEach(cleanup)

describe('a focus target the screen does not carry', () => {
    it('gives a fresh local composer refusal one alert after a setup failure', () => {
        const failed = {...idle(), commandError: publicError('setup_failed')} as TransferState
        const view = harnessMountWith(mocks.useTransfer, failed,
            {...commands, composer: {text: 'note', error: publicError('setup_failed').message}})
        expect(screen.queryByRole('alert')).toBeNull()
        harnessTransitionTo(mocks.useTransfer, view, idle(),
            {...commands, composer: {text: 'note', error: 'Text is too long (65,536 UTF-8 bytes maximum).'}})
        expect(screen.getAllByRole('alert')).toHaveLength(1)
        expect(screen.getByRole('alert').textContent).toBe('Text is too long (65,536 UTF-8 bytes maximum).')
        expect(document.querySelector('[role="status"]')?.textContent).toBe('')
    })
    it('focuses the text composer heading and returns focus to Idle when Cancel closes it', () => {
        const view = mountWith(idle())
        harnessTransitionTo(mocks.useTransfer, view, idle(), {...commands, composer: {text: '', error: null}})
        expect(document.activeElement).toBe(screen.getByRole('heading', {name: 'Send text'}))
        const field = screen.getByRole('textbox', {name: 'Text to send'})
        expect(field.getAttribute('aria-describedby')).toBeTruthy()
        expect(document.getElementById(field.getAttribute('aria-describedby')!)?.textContent).toBe('Up to 65,536 UTF-8 bytes.')
        harnessTransitionTo(mocks.useTransfer, view, idle(), commands)
        expect(document.activeElement).toBe(screen.getByRole('heading', {name: 'Drop one file or folder'}))
    })

    it('never calls focus, and lets the transition complete anyway', () => {
        const view = mountWith(idle())
        // Story 9.3 renamed this control's label ('Choose a file or folder' ->
        // 'Choose File or Folder') and its heading's copy (dropped the full
        // stop) -- see copy.ts. Both literal strings below are updated to
        // match; nothing else about this test's behaviour or structure
        // changed. Flagged in evidence-9-3 as a deviation from this file's
        // "passes unchanged" acceptance criterion, which is unavoidable once
        // the query text a test uses to *find* an element is itself the copy
        // that changed.
        const anchor = screen.getByRole('button', {name: 'Choose File or Folder'})
        anchor.focus()

        // There is no outcome panel in Idle, so this target is absent.
        expect(document.querySelector('[data-focus-target="outcome"]')).toBeNull()
        transitionTo(view, idle(), {row: 'terminal-outcome', owner: 'focus', target: 'outcome'})

        // Focus did not move, and in particular did not fall to the body -- a
        // blind `.focus()` on a null would have thrown and taken the render
        // with it, and a `?.focus()` would have silently stranded the user.
        expect(document.activeElement).toBe(anchor)
        expect(screen.getByRole('heading', {level: 1}).textContent).toBe('Drop one file or folder')
    })

    it('still empties the announcer, because the row is focus-owned either way', () => {
        const view = mountWith(idle())
        transitionTo(view, idle(), {row: 'cancel-requested', owner: 'announcer', text: 'Canceling'})
        expect(document.querySelector('[role="status"]')?.textContent).toBe('Canceling')

        transitionTo(view, idle(), {row: 'terminal-outcome', owner: 'focus', target: 'outcome'})

        expect(document.querySelector('[role="status"]')?.textContent).toBe('')
    })

    it('focuses the very same target once the view does carry it', () => {
        const view = mountWith(idle())

        transitionTo(
            view,
            {
                phase: 'done',
                session: {sessionId: '0'.repeat(32), lastSeq: 4},
                outcome: {kind: 'done', receipt: doneReceipt},
            },
            {row: 'terminal-outcome', owner: 'focus', target: 'outcome'},
        )

        expect(document.activeElement).toBe(document.querySelector('[data-focus-target="outcome"]'))
    })
})

describe('the routing table is consulted once per transition', () => {
    it('asks for an owner only when the state object actually changed', () => {
        const stable = idle()
        const view = mountWith(stable)
        expect(mocks.route).not.toHaveBeenCalled()

        // The same object back again -- a dismissed native dialog dispatches
        // nothing, so this is the shape that reaches App for it.
        transitionTo(view, stable, null)
        expect(mocks.route).not.toHaveBeenCalled()

        transitionTo(view, idle(), null)
        expect(mocks.route).toHaveBeenCalledTimes(1)
    })
})

it('focuses the recovered receive heading whose description announces the new code', () => {
    const oldID = '0123456789abcdef0123456789abcdef'
    const nextID = '11111111111111111111111111111111'
    const old = {sessionId: oldID, destination: 'Photos', url: `http://192.168.1.5:45678/upload/${oldID}`,
        qrBase64: 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=', warnings: []}
    const recovering: TransferState = {phase: 'recovering', generation: 1, session: {sessionId: oldID, lastSeq: 0},
        mode: 'receive', metadata: old, retired: true, cancelPending: false, refusals: 0}
    const view = mountWith(recovering)
    transitionTo(view, {phase: 'receive-waiting', session: {sessionId: nextID, lastSeq: 0},
        metadata: {...old, sessionId: nextID, url: `http://192.168.1.5:45678/upload/${nextID}`},
        cancelPending: false, commandError: null, refusals: 0, recovered: true},
    {row: 'receive-ready', owner: 'focus', target: 'receive-waiting-heading'})
    const heading = document.querySelector('[data-focus-target="receive-waiting-heading"]')
    expect(document.activeElement).toBe(heading)
    expect(heading?.getAttribute('aria-describedby')).toContain('fd-recovery-success')
    expect(screen.getByText('New code ready. The old code no longer works.')).toBeTruthy()
    expect(screen.getByRole('img', {name: 'Upload QR code for Photos'}).getAttribute('src')).toContain('data:image/png;base64,')
})
