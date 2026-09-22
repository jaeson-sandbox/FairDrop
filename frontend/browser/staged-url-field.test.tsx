import {cleanup, render} from '@testing-library/react'
import {page} from 'vitest/browser'
import {afterEach, describe, expect, it, vi} from 'vitest'
import type {StagedTransferState} from '../src/transfer/state'
import type {FileMetadata} from '../src/transfer/types'
import {StagedView} from '../src/ui/StagedView'
import '../src/style.css'

/*
  Rendered evidence for the Staged URL field clipping defect (observed on the
  built binary: the bottom line of the capability URL was clipped by the
  field's border).

  jsdom cannot see this at all -- it reports every element's scrollHeight as 0
  and performs no layout, so a unit test under `src/` would pass against both
  the broken code and the fix, proving nothing (AGENTS.md "Testing standards,
  learned the hard way": a test that agrees with the bug). This lives beside
  accessibility.test.tsx in the rendered Chromium suite instead, which exists
  precisely to catch what jsdom cannot evaluate.

  The field's required height is variable -- host (IPv4 or IPv6), port, and a
  32-hex token -- so this checks both a realistic capability URL and a
  deliberately longer one (an IPv6 host) that wraps to more lines still. A fix
  that merely raises `rows` to the exact line count of one observed URL would
  still fail the second case.
*/

const sessionId = '0123456789abcdef0123456789abcdef'
const token = '94adac272a8fee62a4436c58a4d4bac6'

vi.mock('../wailsjs/go/main/App', () => ({CopyToClipboard: vi.fn().mockResolvedValue(undefined)}))

function metadata(url: string): FileMetadata {
    return {
        sessionId,
        name: 'Travel Notes.pdf',
        size: 8_400_000,
        isDir: false,
        url,
        qrBase64: '',
        warnings: [],
    }
}

function staged(url: string): StagedTransferState {
    return {
        phase: 'staged',
        session: {sessionId, lastSeq: 0},
        metadata: metadata(url),
        cancelPending: false,
        commandError: null,
    }
}

function renderStagedWithURL(url: string): HTMLElement {
    return render(<StagedView state={staged(url)} onCancel={() => undefined}/>).container
}

afterEach(() => {
    cleanup()
})

/**
 * Fails naming the field and both measurements when the textarea's rendered
 * box is shorter than the content it holds -- the exact shape of the observed
 * defect, where the bottom line of the URL sat past the field's own border.
 */
function assertURLFieldFitsItsContent(container: HTMLElement): void {
    const field = container.querySelector<HTMLTextAreaElement>('.fd-url')
    if (field === null) throw new Error('.fd-url did not render')

    expect(
        field.scrollHeight,
        `.fd-url clips its value: scrollHeight (${field.scrollHeight}px) exceeds its own ` +
            `clientHeight (${field.clientHeight}px) for a ${field.value.length}-character URL`,
    ).toBeLessThanOrEqual(field.clientHeight + 0.5)
}

describe('Staged direct URL field sizes to its content (observed clipping defect)', () => {
    it('fits the observed three-line capability URL with nothing clipped', async () => {
        await page.viewport(1024, 900)
        const url = `http://192.168.1.168:63367/download/${token}`
        const container = renderStagedWithURL(url)
        assertURLFieldFitsItsContent(container)
    })

    it('fits a longer IPv6-host URL that wraps to more lines still', async () => {
        await page.viewport(1024, 900)
        const url = `http://[2001:0db8:85a3:0000:0000:8a2e:0370:7334]:63367/download/${token}`
        const container = renderStagedWithURL(url)
        assertURLFieldFitsItsContent(container)
    })

    it('fits the observed URL at the 320px reflow floor, where wrapping is worst', async () => {
        await page.viewport(320, 900)
        const url = `http://192.168.1.168:63367/download/${token}`
        const container = renderStagedWithURL(url)
        assertURLFieldFitsItsContent(container)
    })
})

/**
 * Waits past both hops the sizing effect's `ResizeObserver` path takes after
 * a live resize: the observer's own notification (delivered before a paint,
 * itself after the layout the resize caused) and then the `resize()` call
 * that notification schedules via `requestAnimationFrame` -- deferred a
 * frame on purpose, to keep the observer's own callback from writing
 * `style.height` synchronously (see the comment in `StagedView.tsx` on why
 * that alone still trips Chromium's loop-detector). Four nested frames, plus
 * a macrotask tick so a `setTimeout`-scheduled continuation inside any of
 * that also has a turn, is comfortably past both hops without pinning an
 * exact count. `page.viewport` and the direct container-width write below
 * both change layout without ever calling React's render -- FairDrop's
 * window is user-resizable down to 640x480 (main.go), and React does not
 * re-render on a resize, so nothing except a live observer on the field
 * itself can react.
 */
async function waitForResizeObserverToSettle(): Promise<void> {
    for (let frame = 0; frame < 4; frame++) {
        await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()))
    }
    await new Promise((resolve) => setTimeout(resolve, 0))
}

/**
 * Chromium reports a `ResizeObserver` callback that re-triggers its own
 * observer as a genuine `window` error event -- "ResizeObserver loop
 * completed with undelivered notifications." -- separate from whatever the
 * callback's own logic concludes. This is how the width-change guard in
 * `StagedView`'s effect is proven load-bearing rather than decorative
 * (review follow-up): removing it was confirmed, by hand, to trip exactly
 * this error even in a run whose final layout still happened to end up
 * correctly sized -- so a content-only assertion would not have caught it.
 * Tracking this error is what makes that mutation fail here reliably.
 */
function trackWindowErrors(): {errors: string[]; stop: () => void} {
    const errors: string[] = []
    const handler = (event: ErrorEvent) => { errors.push(event.message) }
    window.addEventListener('error', handler)
    return {errors, stop: () => window.removeEventListener('error', handler)}
}

/**
 * Counts `style` attribute mutations on the field as a proxy for how many
 * times the sizing effect's internal `resize()` actually ran (each run
 * writes `style.height` twice: reset to `auto`, then to the measured
 * value). This is the "redundant-write" half of the review's requested
 * mutation proof, alongside `trackWindowErrors` above: with the deferred
 * (`requestAnimationFrame`) write in place, removing the width-change guard
 * no longer reproduces Chromium's loop error or a clipped field by itself
 * (the write being deferred a frame already keeps Chromium's loop-detector
 * quiet, and `resize()` is idempotent once the field is correctly sized) --
 * but it does keep running `resize()` a second, unnecessary time in
 * response to the notification its *own* first write causes, which this
 * catches directly rather than relying on a side effect that a different
 * browser build or timing could mask.
 */
function countStyleMutations(field: HTMLElement): {count: () => number; stop: () => void} {
    let mutations = 0
    const observer = new MutationObserver((records) => {
        for (const record of records) if (record.attributeName === 'style') mutations++
    })
    observer.observe(field, {attributes: true, attributeFilter: ['style']})
    return {count: () => mutations, stop: () => observer.disconnect()}
}

describe('Staged direct URL field re-sizes on a live window resize, not just at mount', () => {
    /*
      Review follow-up: the mount-time `useLayoutEffect` measures once, keyed
      to `metadata.url`. The URL never changes while Staged is on screen, but
      the field's *width* does -- `.fd-hero` is a two-column grid that shares
      width with the QR panel, and the window itself is draggable from 1024
      wide down to the 640x480 minimum. Narrowing the window re-wraps the URL
      to more lines without ever re-rendering `StagedView`, so a fix keyed
      only to the URL value clips again the moment the sender resizes -- the
      same defect, reachable by a gesture the product explicitly supports.

      The container is narrowed directly (not via `page.viewport`, though that
      is exercised too) to isolate "the field's own box got smaller" from any
      viewport media-query change, proving the general case rather than one
      breakpoint.
    */
    it('keeps the field unclipped when its container narrows after mount, with no re-render', async () => {
        await page.viewport(1024, 900)
        const url = `http://192.168.1.168:63367/download/${token}`
        const windowErrors = trackWindowErrors()

        const wrapper = document.createElement('div')
        wrapper.style.width = '900px'
        document.body.append(wrapper)

        try {
            const {container} = render(
                <StagedView state={staged(url)} onCancel={() => undefined}/>,
                {container: wrapper},
            )
            assertURLFieldFitsItsContent(container) // sanity: fits at the initial width

            const field = container.querySelector<HTMLTextAreaElement>('.fd-url')
            if (field === null) throw new Error('.fd-url did not render')
            const styleWrites = countStyleMutations(field)

            // Narrows the field's own box, the way dragging the window's edge
            // does -- no React state changes, no re-render.
            wrapper.style.width = '420px'
            await waitForResizeObserverToSettle()

            assertURLFieldFitsItsContent(container)
            styleWrites.stop()

            // One genuine width change should cost exactly one `resize()`
            // run (two `style.height` writes: `auto`, then the measured
            // value) -- not a second, redundant run triggered by that run's
            // own height write notifying the observer again. Four would be
            // two runs; the guard is what keeps it at two.
            expect(
                styleWrites.count(),
                'the field was resized more times than one width change should cost -- the ' +
                    'width-change guard should have discarded the notification caused by the ' +
                    "resize's own height write",
            ).toBeLessThanOrEqual(2)
        } finally {
            wrapper.remove()
            windowErrors.stop()
        }

        expect(
            windowErrors.errors,
            'a live resize triggered a window error (e.g. a ResizeObserver loop) -- the ' +
                'width-change guard in the sizing effect should have prevented this',
        ).toEqual([])
    })

    it('keeps the field unclipped when the whole window narrows to the 640x480 minimum after mount', async () => {
        await page.viewport(1024, 900)
        const url = `http://192.168.1.168:63367/download/${token}`
        const windowErrors = trackWindowErrors()
        const container = renderStagedWithURL(url)
        assertURLFieldFitsItsContent(container) // sanity: fits at the initial viewport

        await page.viewport(640, 480)
        await waitForResizeObserverToSettle()

        assertURLFieldFitsItsContent(container)
        windowErrors.stop()
        expect(
            windowErrors.errors,
            'a live resize triggered a window error (e.g. a ResizeObserver loop) -- the ' +
                'width-change guard in the sizing effect should have prevented this',
        ).toEqual([])
    })
})

/*
  Story 7.9: the seam it removes -- a one-frame lag between the URL field's
  own width and its height during a live resize -- cannot be seen by any test
  above. Those all settle after a resize and then assert; a lag that is gone
  by the time the assertion runs is invisible to them. What is measurable
  instead is "at every width the field is sized correctly, right now, with no
  transient extra arrangement anywhere in between" -- a continuous sweep
  replacing "looks smooth" with a per-step assertion, exactly as the story
  asks for. jsdom cannot evaluate any of this; it performs no layout.
*/

/** Bounding rects that intersect indicate two elements are drawn on top of each other. */
function rectsOverlap(a: DOMRect, b: DOMRect): boolean {
    return a.left < b.right && b.left < a.right && a.top < b.bottom && b.top < a.bottom
}

/**
 * The two reflow pairs `.fd-hero` and `.fd-direct-row` fold at 759px
 * (style.css, "Reflow"): side-by-side above it, stacked at or below it. Their
 * track *count* -- not the fluid pixel sizes of a `minmax(0, 1fr)` track,
 * which legitimately differ at every width -- is what identifies which of
 * the two arrangements the sweep is currently in.
 */
function currentArrangement(container: HTMLElement): string {
    const hero = container.querySelector('.fd-hero')
    const row = container.querySelector('.fd-direct-row')
    if (hero === null || row === null) throw new Error('.fd-hero or .fd-direct-row did not render')

    const heroTracks = getComputedStyle(hero).gridTemplateColumns.split(' ').length
    const rowTracks = getComputedStyle(row).gridTemplateColumns.split(' ').length
    return `hero:${heroTracks}/row:${rowTracks}`
}

describe('Staged view resizes seamlessly across a continuous width sweep (Story 7.9)', () => {
    it('never clips the URL field, never opens a page scrollbar, and never overlaps content from 1200px down to 320px', async () => {
        // The deliberately longer IPv6-host URL: the case most likely to clip
        // if the CSS box-model tokens the mirror and the field share ever
        // drift apart (see assertURLFieldFitsItsContent's caller comment
        // above and the "mismatch" mutation this proves).
        const url = `http://[2001:0db8:85a3:0000:0000:8a2e:0370:7334]:63367/download/${token}`
        const container = renderStagedWithURL(url)

        const heroDetails = container.querySelector('.fd-hero__details')
        const qrPanel = container.querySelector('.fd-qr-panel')
        const urlWrap = container.querySelector('.fd-url-wrap')
        const copyButton = container.querySelector('.fd-direct-row > .fd-button')
        if (heroDetails === null || qrPanel === null || urlWrap === null || copyButton === null) {
            throw new Error('.fd-hero__details, .fd-qr-panel, .fd-url-wrap or its copy button did not render')
        }

        const arrangements: string[] = []

        // 1200 down to 320 in steps of 40 (at most the step the acceptance
        // criterion allows): 23 widths, covering the 759px reflow breakpoint
        // and the 320px reflow floor from both directions.
        for (let width = 1200; width >= 320; width -= 40) {
            await page.viewport(width, 900)

            // Re-thrown with the sweep's own current width in the message: the
            // acceptance criterion asks the sweep to "fail and name the first
            // width at which it clips", and assertURLFieldFitsItsContent's own
            // message (shared with the non-sweep cases above, which each know
            // their one fixed width already) does not carry that context.
            try {
                assertURLFieldFitsItsContent(container)
            } catch (error) {
                const reason = error instanceof Error ? error.message : String(error)
                throw new Error(`at ${width}px, ${reason}`)
            }

            expect(
                document.documentElement.scrollWidth,
                `a page-level horizontal scrollbar appeared at ${width}px: ` +
                    `scrollWidth (${document.documentElement.scrollWidth}px) exceeds ` +
                    `clientWidth (${document.documentElement.clientWidth}px)`,
            ).toBeLessThanOrEqual(document.documentElement.clientWidth + 0.5)

            expect(
                rectsOverlap(heroDetails.getBoundingClientRect(), qrPanel.getBoundingClientRect()),
                `.fd-hero__details overlaps .fd-qr-panel at ${width}px`,
            ).toBe(false)

            expect(
                rectsOverlap(urlWrap.getBoundingClientRect(), copyButton.getBoundingClientRect()),
                `the URL field overlaps its copy button at ${width}px`,
            ).toBe(false)

            arrangements.push(currentArrangement(container))
        }

        // Exactly the arrangements the CSS defines (one 759px breakpoint ->
        // two arrangements: side-by-side above it, stacked at or below it),
        // and no more -- a transient third arrangement at some width in
        // between would mean something briefly mis-lays-out mid-sweep.
        expect(new Set(arrangements).size).toBe(2)

        // No flapping: once the sweep crosses into the stacked arrangement it
        // stays there for the rest of the (narrowing) sweep -- exactly one
        // transition, at the 759px boundary, never a width that reverts to
        // the wider arrangement or bounces between the two.
        let transitions = 0
        for (let i = 1; i < arrangements.length; i++) {
            if (arrangements[i] !== arrangements[i - 1]) transitions++
        }
        expect(
            transitions,
            `expected exactly one arrangement transition across the sweep, saw ${transitions}: ` +
                arrangements.join(' -> '),
        ).toBe(1)
    })
})
