import {useEffect, useRef, useState} from 'react'
import type {ReactNode} from 'react'

/**
 * Story 10.2's one sanctioned transient notification.
 *
 * Built generically -- icon, title, body, caller-owned dismissal -- so a
 * later transient message can reuse it; only the cancellation notification
 * wires it today (App.tsx). It is not interactive: no controls, no focus,
 * not `role="alert"`, not a live region. The one focus move that announces
 * a cancellation targets the Idle heading, whose `aria-describedby` points
 * at this component's own text (`textId`) while it is mounted -- see
 * `IdleView.tsx` and `announce.ts`'s `cancel-won` row.
 *
 * Default durations, `EXPERIENCE.md`'s numbers exactly: ~4s fully visible,
 * ~260ms to exit. Both are constructor arguments rather than hardcoded, so
 * a test can shrink them instead of waiting out the real duration -- the
 * "injectable timing" the story's acceptance criteria ask for.
 */
const DEFAULT_VISIBLE_MS = 4_000
const DEFAULT_EXIT_MS = 260

/**
 * The one stable id this product's single notification instance uses for its
 * accessible text, shared between `App.tsx` (which passes it as `textId`
 * below) and `IdleView.tsx` (which points the Idle heading's
 * `aria-describedby` at it while a notification is mounted). Exported so
 * both call sites read the same literal rather than two copies of a string
 * that could drift.
 */
export const NOTIFICATION_TEXT_ID = 'fd-notification-text'

type NotificationPhase = 'visible' | 'leaving'

export interface NotificationProps {
    readonly icon: ReactNode
    readonly title: string
    readonly body: string
    /**
     * The id of the element carrying this notification's accessible text
     * (title plus body, as one sentence). A caller wires this into another
     * element's `aria-describedby` while the notification is mounted.
     */
    readonly textId: string
    /**
     * Called exactly once, when the notification should leave the DOM.
     *
     * This is the one frontend timer `EXPERIENCE.md`'s "no lifecycle/reset
     * timers" ban now carves out, and the carve-out is narrow on purpose:
     * this callback may remove the notification and do nothing else. It
     * never dispatches a reducer action, never calls a bound Go command,
     * and never moves focus -- the caller's own `onDismiss` is checked by
     * `Notification.test.tsx` to prove it is never anything but a state
     * setter that stops rendering this component.
     */
    readonly onDismiss: () => void
    readonly visibleMs?: number
    readonly exitMs?: number
}

/**
 * A translucent, top-centre overlay notification that slides in, holds, then
 * slides out and unmounts.
 *
 * The exit is entirely timer-driven -- `scheduleExit`'s `setTimeout` is what
 * flips `phase` to `'leaving'`, and a second `setTimeout` (not a
 * `transitionend` listener) is what calls `onDismiss` after the exit
 * transition's own duration. *Mutation:* rewire either step through
 * `transitionend` instead -> `Notification.test.tsx`'s reduced-motion-style
 * case (transitions disabled via `transition: none` and no
 * `getAnimations()` support) has to keep unmounting on schedule, and a
 * `transitionend`-driven unmount never fires there, because an engine that
 * cannot transition never dispatches that event at all.
 */
export function Notification({
    icon,
    title,
    body,
    textId,
    onDismiss,
    visibleMs = DEFAULT_VISIBLE_MS,
    exitMs = DEFAULT_EXIT_MS,
}: NotificationProps) {
    const [phase, setPhase] = useState<NotificationPhase>('visible')
    const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
    const deadlineRef = useRef<number>(Date.now() + visibleMs)
    const remainingRef = useRef<number>(visibleMs)

    function clearPendingTimer(): void {
        if (timerRef.current === null) return
        clearTimeout(timerRef.current)
        timerRef.current = null
    }

    function scheduleExit(ms: number): void {
        clearPendingTimer()
        deadlineRef.current = Date.now() + ms
        timerRef.current = setTimeout(() => setPhase('leaving'), ms)
    }

    // Mount-only: starts the one visible-duration timer. Re-running this per
    // render would restart the countdown on every unrelated re-render, which
    // is not what "stays ~4 seconds" means.
    useEffect(() => {
        scheduleExit(visibleMs)
        return clearPendingTimer
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [])

    // The unmount step: fires once phase flips to 'leaving', on a plain
    // timeout rather than the exit transition's own 'transitionend' -- see
    // the function comment above for why that distinction is load-bearing.
    useEffect(() => {
        if (phase !== 'leaving') return
        clearPendingTimer()
        timerRef.current = setTimeout(onDismiss, exitMs)
        return clearPendingTimer
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [phase])

    function pause(): void {
        if (phase !== 'visible' || timerRef.current === null) return
        clearPendingTimer()
        remainingRef.current = Math.max(0, deadlineRef.current - Date.now())
    }

    function resume(): void {
        if (phase !== 'visible' || timerRef.current !== null) return
        scheduleExit(remainingRef.current)
    }

    return (
        <div
            className="fd-notification"
            data-phase={phase}
            onPointerEnter={pause}
            onPointerLeave={resume}
        >
            <span className="fd-notification__icon" aria-hidden="true">{icon}</span>
            {/*
              The visible title/body are aria-hidden: the accessible
              description a caller wires up (below) is the one sentence
              "Transfer canceled. Ready for another file or folder." -- built
              here as a single visually-hidden node rather than left to
              flatten two separately-hidden paragraphs, which assistive
              technology is not guaranteed to join with the same punctuation
              this sentence needs.
            */}
            <div className="fd-notification__text">
                <p className="fd-notification__title" aria-hidden="true">{title}</p>
                <p className="fd-notification__body" aria-hidden="true">{body}</p>
            </div>
            <span id={textId} className="fd-visually-hidden">{title}. {body}</span>
        </div>
    )
}
