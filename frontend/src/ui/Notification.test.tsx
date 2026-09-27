import {act} from 'react'
import {cleanup, fireEvent, render} from '@testing-library/react'
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {NOTIFICATION_TEXT_ID, Notification} from './Notification'

function show(onDismiss: () => void, visibleMs = 1_000, exitMs = 100) {
    return render(
        <Notification
            icon={<span aria-hidden="true">&times;</span>}
            title="Transfer canceled"
            body="Ready for another file or folder."
            textId={NOTIFICATION_TEXT_ID}
            onDismiss={onDismiss}
            visibleMs={visibleMs}
            exitMs={exitMs}
        />,
    )
}

function notification(): HTMLElement {
    return document.querySelector('.fd-notification') as HTMLElement
}

/** The state flip a timer callback causes needs a flush before an assertion reads the DOM. */
function advance(ms: number): void {
    act(() => {
        vi.advanceTimersByTime(ms)
    })
}

beforeEach(() => {
    vi.useFakeTimers()
})

afterEach(() => {
    cleanup()
    vi.useRealTimers()
})

describe('lifetime: shows, stays, exits, then unmounts', () => {
    it('stays visible for the full duration and only then begins to exit', () => {
        const onDismiss = vi.fn()
        show(onDismiss)

        expect(notification().dataset.phase).toBe('visible')

        advance(999)
        expect(notification().dataset.phase).toBe('visible')
        expect(onDismiss).not.toHaveBeenCalled()

        advance(1)
        expect(notification().dataset.phase).toBe('leaving')
        expect(onDismiss).not.toHaveBeenCalled()
    })

    /*
      Story 10.2's named mutation: rewire the unmount step through
      `transitionend` instead of a plain timeout. jsdom never dispatches a
      real `transitionend` -- there is no layout engine to finish a
      transition -- so a `transitionend`-driven implementation would leave
      `onDismiss` uncalled forever here, exactly the failure the story asks
      this test to catch. Advancing fake timers past `exitMs` with no
      transition event ever fired is what proves the unmount is timer-driven.
    */
    it('unmounts (calls onDismiss) once the exit duration elapses, without any transitionend event', () => {
        const onDismiss = vi.fn()
        show(onDismiss)

        advance(1_000)
        expect(onDismiss).not.toHaveBeenCalled()

        advance(99)
        expect(onDismiss).not.toHaveBeenCalled()

        advance(1)
        expect(onDismiss).toHaveBeenCalledTimes(1)
    })

    it('never calls onDismiss before the full visible-plus-exit duration has elapsed', () => {
        const onDismiss = vi.fn()
        show(onDismiss)

        advance(1_099)
        expect(onDismiss).not.toHaveBeenCalled()
    })
})

describe('hover pauses the dismissal timer', () => {
    it('pauses while the pointer is over it and resumes with the remaining time when it leaves', () => {
        const onDismiss = vi.fn()
        show(onDismiss, 1_000, 100)

        advance(500)
        fireEvent.pointerEnter(notification())

        // Paused: however long real time (fake-timer time) passes while the
        // pointer stays over it, nothing moves toward leaving.
        advance(10_000)
        expect(notification().dataset.phase).toBe('visible')
        expect(onDismiss).not.toHaveBeenCalled()

        fireEvent.pointerLeave(notification())

        // Resumed with the ~500ms that was left, not a fresh 1000ms.
        advance(499)
        expect(notification().dataset.phase).toBe('visible')

        advance(1)
        expect(notification().dataset.phase).toBe('leaving')
    })

    it('does nothing on a hover that starts after the notification has already begun leaving', () => {
        const onDismiss = vi.fn()
        show(onDismiss, 1_000, 100)

        advance(1_000)
        expect(notification().dataset.phase).toBe('leaving')

        fireEvent.pointerEnter(notification())
        advance(100)

        // The exit is not pausable -- only the visible countdown is.
        expect(onDismiss).toHaveBeenCalledTimes(1)
    })
})

describe('the timer is presentation-only', () => {
    it('calls onDismiss and nothing else -- no dispatch, no bound command, no focus move', () => {
        // onDismiss is the *only* function this component is given. Proving
        // it is the only thing called, and that it is called exactly once,
        // is what "the timer may only unmount the notification" reduces to
        // for a component that owns no reducer, no Go binding and no ref to
        // move focus with.
        const onDismiss = vi.fn()
        const activeBefore = document.activeElement
        show(onDismiss)

        // Two steps, not one: the second (exit) timer is only scheduled once
        // React flushes the effect that reacts to `phase` becoming
        // 'leaving', so a single large jump would advance the fake clock
        // past that timer's own deadline before it was ever registered.
        advance(1_000)
        advance(100)

        expect(onDismiss).toHaveBeenCalledTimes(1)
        expect(onDismiss).toHaveBeenCalledWith()
        expect(document.activeElement).toBe(activeBefore)
    })

    it('renders no interactive control and is not a live region or an alert', () => {
        show(vi.fn())
        const el = notification()

        expect(el.querySelector('button')).toBeNull()
        expect(el.getAttribute('role')).not.toBe('alert')
        expect(el.closest('[aria-live]')).toBeNull()
        expect(el.hasAttribute('tabindex')).toBe(false)
    })
})

describe('accessible text', () => {
    it('exposes the title and body as one combined sentence at the well-known id', () => {
        show(vi.fn())

        expect(document.getElementById(NOTIFICATION_TEXT_ID)?.textContent)
            .toBe('Transfer canceled. Ready for another file or folder.')
    })

    it('hides the visible title/body paragraphs from assistive technology, so the sentence is not read twice', () => {
        show(vi.fn())
        const el = notification()

        expect(el.querySelector('.fd-notification__title')?.getAttribute('aria-hidden')).toBe('true')
        expect(el.querySelector('.fd-notification__body')?.getAttribute('aria-hidden')).toBe('true')
        expect(el.querySelector('.fd-notification__title')?.textContent).toBe('Transfer canceled')
        expect(el.querySelector('.fd-notification__body')?.textContent).toBe('Ready for another file or folder.')
    })
})
