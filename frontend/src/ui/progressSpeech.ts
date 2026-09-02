/**
 * Assistive progress speech, throttled separately from the visible meter.
 *
 * EXPERIENCE.md: "Visual progress may refresh on every accepted snapshot.
 * Assistive progress speech is separate: start once; then no more often than
 * every five seconds **and** only after meaningful change." Both conditions are
 * required, which is why a snapshot can repaint the meter and stay silent.
 *
 * Throughput is never spoken. It is a visual metric only, and a rate read out
 * every few seconds is the event log the spine forbids rather than information
 * about the transfer's state.
 *
 * Nothing here holds a timer. The caller passes the clock reading it already
 * has, so the throttle is a pure function of (memory, snapshot, now) and a test
 * can walk it across five seconds without waiting five seconds.
 */

import type {ProgressSelection} from '../transfer/selectors'
import {copy} from './copy'
import {formatBytes} from './format'

/** No two updates closer together than this, however much changed. */
export const progressSpeechIntervalMs = 5_000

/** Meaningful change for a known total. */
export const progressSpeechPercentPoints = 10

/** Meaningful change measured in new wire bytes: 10 MiB. */
export const progressSpeechBytes = 10 * 1024 * 1024

/** What the last spoken update said, and when. */
export interface ProgressSpeechMemory {
    readonly spokenAtMs: number
    readonly percent: number
    readonly bytesSent: number
}

export interface ProgressSpeech {
    readonly text: string
    readonly memory: ProgressSpeechMemory
}

/**
 * The update to speak for this snapshot, or `null` to stay silent.
 *
 * `memory` is `null` before the first update of a transfer, which is the "start
 * once" case: the first accepted snapshot always speaks. Reset it to `null`
 * whenever the transfer leaves the Transferring phase -- a terminal outcome
 * cancels queued progress speech, and the outcome's own focused heading is the
 * only thing that should be heard next.
 */
export function nextProgressSpeech(
    memory: ProgressSpeechMemory | null,
    progress: ProgressSelection,
    nowMs: number,
): ProgressSpeech | null {
    const current: ProgressSpeechMemory = {
        spokenAtMs: nowMs,
        percent: progress.determinate ? Math.round(progress.value) : 0,
        bytesSent: progress.bytesSent,
    }

    if (memory === null) return {text: progressSpeechText(progress), memory: current}

    // Written as "not far enough apart" so a clock that produced NaN, or went
    // backwards, stays silent rather than speaking on every snapshot.
    if (!(nowMs - memory.spokenAtMs >= progressSpeechIntervalMs)) return null
    if (!isMeaningfulChange(memory, current, progress)) return null

    return {text: progressSpeechText(progress), memory: current}
}

function isMeaningfulChange(
    memory: ProgressSpeechMemory,
    current: ProgressSpeechMemory,
    progress: ProgressSelection,
): boolean {
    if (progress.determinate &&
        Math.abs(current.percent - memory.percent) >= progressSpeechPercentPoints) return true
    return current.bytesSent - memory.bytesSent >= progressSpeechBytes
}

/**
 * The spoken form of one snapshot, composed from the same registry strings the
 * meter renders. No sentence is invented here, and no rate appears in any of
 * the three modes.
 */
export function progressSpeechText(progress: ProgressSelection): string {
    if (progress.mode === 'known-empty') return copy.progress.knownEmpty

    if (progress.mode === 'unknown') {
        return copy.progress.unknown + copy.label.metaSeparator +
            `${formatBytes(progress.bytesSent)} ${copy.label.sent}`
    }

    return `${formatBytes(progress.bytesSent)} ${copy.label.of} ${formatBytes(progress.totalBytes)}` +
        copy.label.metaSeparator + `${Math.round(progress.value)}%`
}
