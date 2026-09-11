import {describe, expect, it} from 'vitest'
import {selectProgressSnapshot, type ProgressSelection} from '../transfer/selectors'
import {
    nextProgressSpeech,
    progressSpeechBytes,
    progressSpeechIntervalMs,
    progressSpeechPercentPoints,
    progressSpeechText,
    type ProgressSpeechMemory,
} from './progressSpeech'

/*
  The wire's `percent` is deliberately 0 in every case below.

  The selector derives the figure that is spoken from the byte pair, so a
  snapshot whose percent field disagrees changes nothing anyone hears (Epic 1
  retrospective item 7). Passing a plausible-looking figure here would hide
  that: every expectation would still read as though the wire had been
  believed.
*/
function known(bytesSent: number, totalBytes = 100_000_000): ProgressSelection {
    return selectProgressSnapshot({
        bytesSent, totalBytes, totalKnown: true, percent: 0, speedBytesPerSec: 12_400_000,
    })
}

function unknown(bytesSent: number): ProgressSelection {
    return selectProgressSnapshot({
        bytesSent, totalBytes: 0, totalKnown: false, percent: 0, speedBytesPerSec: 12_400_000,
    })
}

const empty: ProgressSelection = selectProgressSnapshot({
    bytesSent: 0, totalBytes: 0, totalKnown: true, percent: 0, speedBytesPerSec: 0,
})

const mib = 1024 * 1024

describe('the assistive progress throttle', () => {
    /*
      EXPERIENCE.md reads per-mode -- "10 percentage points for known totals or
      10 MiB of new wire bytes for unknown totals" -- while the spec's frozen
      Always clause reads cross-mode, and the implementation follows the spec.
      Neither reading was pinned, so rewriting this to one threshold per mode
      passed the whole suite. This case is the difference between them.
    */
    it('speaks a known total that gained bytes without gaining percentage points', () => {
        const memory = {spokenAtMs: 0, percent: 0, bytesSent: 0}
        // A total large enough that 10 MiB is worth fewer than ten percentage
        // points, which is the whole distinction this case exists to draw.
        const snapshot = known(progressSpeechBytes, 200_000_000)

        expect(nextProgressSpeech(memory, snapshot, progressSpeechIntervalMs)).not.toBeNull()
    })

    it('speaks once at the start, before any interval has passed', () => {
        expect(nextProgressSpeech(null, known(1_000), 0)).toEqual({
            text: '1.0 KB of 100.0 MB · 0%',
            memory: {spokenAtMs: 0, percent: 0, bytesSent: 1_000},
        })
    })

    it('stays silent inside the five-second floor however much changed', () => {
        const start = nextProgressSpeech(null, known(0), 0)!

        // 90 percentage points and 90 MB of new bytes, one millisecond early.
        const early = nextProgressSpeech(start.memory, known(90_000_000), progressSpeechIntervalMs - 1)

        expect(early).toBeNull()
    })

    it('stays silent past the floor when nothing meaningful changed', () => {
        const start = nextProgressSpeech(null, known(0), 0)!

        // Nine percentage points, and well under 10 MiB of new wire bytes.
        const small = nextProgressSpeech(start.memory, known(9_000_000), 60_000)

        expect(small).toBeNull()
    })

    it('speaks at exactly the interval and exactly the percentage threshold', () => {
        const memory: ProgressSpeechMemory = {spokenAtMs: 0, percent: 10, bytesSent: 0}

        const spoken = nextProgressSpeech(
            memory,
            known(20_000_000),
            progressSpeechIntervalMs,
        )

        expect(spoken?.text).toBe('20.0 MB of 100.0 MB · 20%')
        expect(spoken?.memory).toEqual({
            spokenAtMs: progressSpeechIntervalMs,
            percent: 10 + progressSpeechPercentPoints,
            bytesSent: 20_000_000,
        })
    })

    it('speaks on 10 MiB of new wire bytes when the total is unknown', () => {
        const memory: ProgressSpeechMemory = {spokenAtMs: 0, percent: 0, bytesSent: 0}

        const justUnder = nextProgressSpeech(memory, unknown(progressSpeechBytes - 1), 10_000)
        const atThreshold = nextProgressSpeech(memory, unknown(progressSpeechBytes), 10_000)

        expect(justUnder).toBeNull()
        expect(atThreshold?.text).toBe('Sending — total size unknown · 10.5 MB sent')
    })

    it('counts a backwards percentage as change, since it is still new information', () => {
        const memory: ProgressSpeechMemory = {spokenAtMs: 0, percent: 60, bytesSent: 50_000_000}

        // The throttle compares magnitudes, not direction. Nothing in it
        // depends on the reducer's monotonicity holding, which is the point:
        // it is a pure function over a remembered figure, and a figure that
        // moved backwards is still information the listener does not have.
        expect(nextProgressSpeech(memory, known(50_000_000), 20_000)?.text).toContain('50%')
    })

    it('stays silent on a clock that went backwards or produced nothing usable', () => {
        const memory: ProgressSpeechMemory = {spokenAtMs: 60_000, percent: 0, bytesSent: 0}

        expect(nextProgressSpeech(memory, known(90_000_000), 0)).toBeNull()
        expect(nextProgressSpeech(memory, known(90_000_000), Number.NaN)).toBeNull()
    })

    it('is cancelled by clearing the memory, which is what a terminal outcome does', () => {
        // The App drops the memory the moment the phase stops being
        // Transferring, so nothing is left queued to speak after Done or Error.
        const restarted = nextProgressSpeech(null, known(90_000_000), 1_000)

        expect(restarted).not.toBeNull()
    })
})

describe('what a progress update says', () => {
    it('reads a known positive total as sent-of-total and the derived percentage', () => {
        expect(progressSpeechText(known(5_800_000, 8_400_000))).toBe('5.8 MB of 8.4 MB · 69%')
    })

    it('reads an unknown total as its literal status and the wire bytes', () => {
        expect(progressSpeechText(unknown(48_200_000))).toBe('Sending — total size unknown · 48.2 MB sent')
    })

    it('reads a known-empty payload as its literal status alone', () => {
        expect(progressSpeechText(empty)).toBe('Empty file — 0 bytes to transfer')
    })

    it('never speaks throughput in any mode', () => {
        for (const progress of [known(5_800_000, 8_400_000), unknown(48_200_000), empty]) {
            const spoken = progressSpeechText(progress)
            expect(spoken, spoken).not.toContain('/s')
            expect(spoken, spoken).not.toContain('12.4 MB')
        }
    })

    it('holds the thresholds the spine names', () => {
        expect(progressSpeechIntervalMs).toBe(5_000)
        expect(progressSpeechPercentPoints).toBe(10)
        expect(progressSpeechBytes).toBe(10 * mib)
    })
})
