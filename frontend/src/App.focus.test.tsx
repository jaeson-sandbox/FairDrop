import {act, cleanup, render, screen} from '@testing-library/react'
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import App from './App'
import type {TransferState} from './transfer/state'
import type {Announcement} from './ui/announce'

/*
  The "prove the target exists" half of the contract.

  Every routing row a reducer transition can actually produce lands on a target
  the matching view really carries, which is what makes this guard impossible to
  exercise from real state -- and exactly why it is worth a test. The routing
  table is stubbed here so a row can name a target that is not on the screen,
  which is the shape a future view edit would take: the row survives, the
  attribute quietly does not.
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

function mountWith(state: TransferState) {
    mocks.useTransfer.mockReturnValue({
        state,
        stage: mocks.noop,
        selectFile: mocks.noop,
        selectDirectory: mocks.noop,
        cancel: mocks.noop,
        rejectSelection: mocks.noop,
        dismissRetained: mocks.noop,
    })
    return render(<App/>)
}

function transitionTo(view: ReturnType<typeof mountWith>, state: TransferState, routed: Announcement | null) {
    mocks.route.mockReturnValue(routed)
    mocks.useTransfer.mockReturnValue({
        state,
        stage: mocks.noop,
        selectFile: mocks.noop,
        selectDirectory: mocks.noop,
        cancel: mocks.noop,
        rejectSelection: mocks.noop,
        dismissRetained: mocks.noop,
    })
    act(() => view.rerender(<App/>))
}

beforeEach(() => {
    for (const mock of Object.values(mocks)) mock.mockReset()
    mocks.route.mockReturnValue(null)
})
afterEach(cleanup)

describe('a focus target the screen does not carry', () => {
    it('never calls focus, and lets the transition complete anyway', () => {
        const view = mountWith(idle())
        const anchor = screen.getByRole('button', {name: 'Select File'})
        anchor.focus()

        // There is no outcome panel in Idle, so this target is absent.
        expect(document.querySelector('[data-focus-target="outcome"]')).toBeNull()
        transitionTo(view, idle(), {row: 'terminal-outcome', owner: 'focus', target: 'outcome'})

        // Focus did not move, and in particular did not fall to the body -- a
        // blind `.focus()` on a null would have thrown and taken the render
        // with it, and a `?.focus()` would have silently stranded the user.
        expect(document.activeElement).toBe(anchor)
        expect(screen.getByRole('heading', {level: 1}).textContent).toBe('Drop one file or folder.')
    })

    it('still empties the announcer, because the row is focus-owned either way', () => {
        const view = mountWith(idle())
        transitionTo(view, idle(), {row: 'cancel-requested', owner: 'announcer', text: 'Canceling…'})
        expect(document.querySelector('[role="status"]')?.textContent).toBe('Canceling…')

        transitionTo(view, idle(), {row: 'terminal-outcome', owner: 'focus', target: 'outcome'})

        expect(document.querySelector('[role="status"]')?.textContent).toBe('')
    })

    it('focuses the very same target once the view does carry it', () => {
        const view = mountWith(idle())

        transitionTo(
            view,
            {phase: 'done', session: {sessionId: '0'.repeat(32), lastSeq: 4}, outcome: {kind: 'done'}},
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
