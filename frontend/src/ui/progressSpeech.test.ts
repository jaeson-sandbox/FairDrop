import {readFileSync, readdirSync} from 'node:fs'
import {resolve} from 'node:path'
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

// The one design folder, located the way styles.test.ts locates DESIGN.md:
// by discovery rather than by a hardcoded dated path, so a re-dated folder
// fails loudly here instead of silently skipping the assertions below.
function experienceSpinePath(): string {
    // Vitest roots at frontend/, the same anchor styles.test.ts uses to find DESIGN.md.
    const designs = resolve(process.cwd(), '..', '_bmad-output', 'planning-artifacts', 'ux-designs')
    const folders = readdirSync(designs).filter((entry) => entry.startsWith('ux-'))

    expect(folders, 'a ux-* design folder').toHaveLength(1)
    return resolve(designs, folders[0], 'EXPERIENCE.md')
}


describe('the assistive progress throttle', () => {
    /*
      The case where the two readings of the threshold differ.

      EXPERIENCE.md used to read per-mode -- "10 percentage points for known
      totals or 10 MiB of new wire bytes for unknown totals" -- while the
      implementation applied both thresholds in both modes. Nothing pinned
      either reading, so the UX contract and the shipped behaviour disagreed
      for two epics and a rewrite to one threshold per mode would have passed
      the whole suite. Found by the Blind Hunter layer re-run that Story 1.10
      never got (D-109).

      Settled on 2026-09-14 by keeping the code and correcting the spine: a
      known total that is large and slow gains bytes without gaining percentage
      points, so the per-mode reading meant minutes of silence, and the
      five-second floor already caps how often the extra threshold can speak.
      The test below is the behaviour; the test after it is the spine.
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

/*
  The UX contract has to keep saying what the throttle does.

  Nothing connected EXPERIENCE.md's sentence to progressSpeech.ts, which is how
  they came to disagree unnoticed. This reads the spine the same way
  styles.test.ts reads DESIGN.md's contrast figures: the document is the
  artefact under test, and a silent edit to either side fails here.
*/
describe('the spine describes the throttle it documents', () => {
    it('states the thresholds as cross-mode, matching isMeaningfulChange', () => {
        const spine = readFileSync(experienceSpinePath(), 'utf8')

        const sentence = spine.split(/\r?\n/).find((line) => line.includes('Assistive progress speech is separate'))
        expect(sentence, 'the assistive-speech sentence').toBeTruthy()
        expect(sentence).toContain('in either mode')
        expect(sentence).not.toContain('for known totals or 10 MiB of new wire bytes for unknown totals')
    })

    it('names the same two numbers the code enforces', () => {
        const spine = readFileSync(experienceSpinePath(), 'utf8')

        expect(spine).toContain(`${progressSpeechPercentPoints} percentage points`)
        expect(spine).toContain(`${progressSpeechBytes / mib} MiB`)
        // The spine spells the interval and writes the other two as numerals.
        // Keyed lookup rather than a numeral: an interval with no spelling here
        // fails on the missing key instead of quietly matching nothing.
        const spelled: Record<number, string> = {5: 'five'}
        const seconds = progressSpeechIntervalMs / 1000
        expect(spelled[seconds], `a spelling for ${seconds} seconds`).toBeTruthy()
        expect(spine).toContain(`every ${spelled[seconds]} seconds`)
    })
})
