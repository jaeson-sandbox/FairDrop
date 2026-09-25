import type {} from '@vitest/browser-playwright' // pulls in the CDPSession#send() augmentation for the playwright provider
import type {CSSProperties} from 'react'
import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {cdp, page} from 'vitest/browser'
import {afterEach, describe, expect, it, vi} from 'vitest'
import type {IdleTransferState, StagedTransferState, TransferringTransferState} from '../src/transfer/state'
import type {FileMetadata, ProgressSnapshot} from '../src/transfer/types'
import {IdleView} from '../src/ui/IdleView'
import {StagedView} from '../src/ui/StagedView'
import {TransferringView} from '../src/ui/TransferringView'
import '../src/style.css'

/*
  Rendered evidence for D-065 and D-068: styles.test.ts already proves these
  rules exist in the stylesheet's *text*, and that suite stays exactly as it
  is -- it runs everywhere in milliseconds and catches a token edit before
  this file's browser even launches. What jsdom cannot do is lay out a page or
  evaluate a media query, so 320px reflow, 200% text, the 44px target floor,
  and forced-colors are proved here by rendering the real components in real
  Chromium and measuring them. A rule that disagrees between the two suites is
  a finding to report, never a reason to loosen either one (spec Never list).

  Staged and Transferring are the two views the spec's Intent and I/O matrix
  named for every rendered check below. Idle's browse menu joined them for
  spec-4-1 (Story 4.1): it is the product's first floating surface, so the
  same rendered proof -- targets, reflow, forced colors -- applies to it
  measured open, not merely to its text in the stylesheet.
*/

vi.mock('../wailsjs/go/main/App', () => ({CopyToClipboard: vi.fn().mockResolvedValue(undefined)}))

const sessionId = '0123456789abcdef0123456789abcdef'
const capabilityURL = `http://192.0.2.1:34123/download/${'fedcba9876543210fedcba9876543210'}`

/*
  A synthetic finder-pattern bitmap, drawn with Canvas inside the real browser
  this test already has, so the forced-colors capture below shows an actual
  light/dark module grid rather than a blank tile. It encodes nothing and
  decodes to nothing -- see "the capture is not a scan" on the screenshot test
  below, and D-065's own boundary: no camera reading is attempted here or
  anywhere in this repo.
*/
function syntheticQrBase64(): string {
    const modules = 29
    const scale = 8
    const canvas = document.createElement('canvas')
    canvas.width = modules * scale
    canvas.height = modules * scale
    const ctx = canvas.getContext('2d')
    if (ctx === null) throw new Error('2D canvas context unavailable in this browser')

    ctx.fillStyle = '#FFFFFF'
    ctx.fillRect(0, 0, canvas.width, canvas.height)
    ctx.fillStyle = '#000000'
    for (let y = 0; y < modules; y++) {
        for (let x = 0; x < modules; x++) {
            if ((x * 31 + y * 17) % 5 === 0) ctx.fillRect(x * scale, y * scale, scale, scale)
        }
    }
    // Three finder squares, the one visually distinctive feature every real
    // QR code carries, so the capture reads as "QR-shaped" rather than noise.
    for (const [fx, fy] of [[0, 0], [modules - 7, 0], [0, modules - 7]]) {
        ctx.fillStyle = '#000000'
        ctx.fillRect(fx * scale, fy * scale, 7 * scale, 7 * scale)
        ctx.fillStyle = '#FFFFFF'
        ctx.fillRect((fx + 1) * scale, (fy + 1) * scale, 5 * scale, 5 * scale)
        ctx.fillStyle = '#000000'
        ctx.fillRect((fx + 2) * scale, (fy + 2) * scale, 3 * scale, 3 * scale)
    }

    const dataURL = canvas.toDataURL('image/png')
    return dataURL.slice(dataURL.indexOf(',') + 1)
}

const qrBase64 = syntheticQrBase64()

function metadata(overrides: Partial<FileMetadata> = {}): FileMetadata {
    return {
        sessionId,
        name: 'Travel Notes.pdf',
        size: 8_400_000,
        isDir: false,
        url: capabilityURL,
        qrBase64,
        warnings: [],
        ...overrides,
    }
}

function staged(overrides: Partial<StagedTransferState> = {}): StagedTransferState {
    return {
        phase: 'staged',
        session: {sessionId, lastSeq: 0},
        metadata: metadata(),
        cancelPending: false,
        commandError: null,
        ...overrides,
    }
}

function transferring(overrides: Partial<TransferringTransferState> = {}): TransferringTransferState {
    const progress: ProgressSnapshot = {
        bytesSent: 5_800_000,
        totalBytes: 8_400_000,
        totalKnown: true,
        percent: 68,
        speedBytesPerSec: 4_700_000,
    }
    return {
        phase: 'transferring',
        session: {sessionId, lastSeq: 1},
        metadata: metadata(),
        progress,
        cancelPending: false,
        commandError: null,
        ...overrides,
    }
}

/*
  Story 9.1: every phase view now fades and rises in on mount
  ([data-phase-view]'s @starting-style entrance in style.css, plus the browse
  menu's own scale-and-fade when it is open), so a rendered-geometry
  measurement taken the instant after `render()` can read a mid-transition
  size rather than the settled one -- reported directly on the browse
  trigger's height (43.999984... vs the required 44, a sub-pixel remnant of
  measuring while an ancestor's `translate` was still interpolating) before
  this helper existed and every render below started waiting for it. It is
  flaky, not reliably wrong, precisely because it is a race: whether a given
  synchronous measurement lands before or after the browser's next paint is
  not deterministic from here.
*/
async function waitForEntranceToSettle(element: Element): Promise<void> {
    // getAnimations() only returns a running Web Animation once the browser
    // has actually started one, which needs at least one paint after the
    // element's entrance state is first committed -- two rAFs is the same
    // "definitely past the next paint" margin `staged-url-field.test.tsx`
    // already uses elsewhere in this suite for a comparable reason.
    await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()))
    await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()))
    // {subtree: true} from the render root, not from any one descendant, so
    // this waits out both the phase view's own entrance and a nested one
    // (the browse menu) in a single call -- waiting on the menu alone once
    // missed the ancestor region's still-running translate entirely, which
    // is exactly how the flake above reached a shipped assertion.
    await Promise.all(element.getAnimations({subtree: true}).map((animation) => animation.finished))
}

async function renderStaged(): Promise<HTMLElement> {
    const {container} = render(<StagedView state={staged()} onCancel={() => undefined}/>)
    await waitForEntranceToSettle(container)
    return container
}

async function renderTransferring(): Promise<HTMLElement> {
    const {container} = render(<TransferringView state={transferring()} onCancel={() => undefined}/>)
    await waitForEntranceToSettle(container)
    return container
}

const dropTargetStyle = {'--wails-drop-target': 'drop'} as CSSProperties

function idle(): IdleTransferState {
    return {phase: 'idle', retainedOutcome: null, commandError: null}
}

/** Idle with the browse menu already open -- the surface these checks measure. */
async function renderIdleMenuOpen(): Promise<HTMLElement> {
    const {container} = render(
        <IdleView
            state={idle()}
            dropTargetStyle={dropTargetStyle}
            cancelWon={false}
            onSelectFile={() => undefined}
            onSelectDirectory={() => undefined}
        />,
    )
    fireEvent.click(screen.getByRole('button', {name: 'Choose a file or folder'}))
    await waitForEntranceToSettle(container)
    return container
}

// Every rendered token or media-feature override below is applied to the
// live document, not to a copy, so leaving any of it in place would bleed
// into the next test regardless of which file declares it next.
afterEach(async () => {
    cleanup()
    document.documentElement.style.cssText = ''
    document.getElementById(textSpacingStyleId)?.remove()
    await page.viewport(1024, 800)
    await setForcedColorsActive(false)
})

function describeElement(element: Element): string {
    const tag = element.tagName.toLowerCase()
    const classes = element.classList.length > 0 ? `.${[...element.classList].join('.')}` : ''
    const label = element.getAttribute('aria-label') ?? element.getAttribute('alt') ??
        (element.textContent ?? '').trim().slice(0, 40)
    return label === '' ? `${tag}${classes}` : `${tag}${classes} "${label}"`
}

/**
 * Fails naming the overflowing element and its measured edge (D-068). Checks
 * both the page as a whole (`documentElement.scrollWidth` vs `clientWidth`,
 * the exact comparison the spec names) and every individual element, because
 * a box that pokes past the viewport's right edge does not always widen the
 * document -- an absolutely positioned node, for one.
 */
function assertNoHorizontalOverflow(container: HTMLElement): void {
    const root = document.documentElement
    expect(
        root.scrollWidth,
        `documentElement.scrollWidth (${root.scrollWidth}px) exceeds clientWidth ` +
            `(${root.clientWidth}px): the page scrolls horizontally`,
    ).toBeLessThanOrEqual(root.clientWidth)

    for (const element of container.querySelectorAll<HTMLElement>('*')) {
        const rect = element.getBoundingClientRect()
        if (rect.width === 0 && rect.height === 0) continue // not laid out / not painted: nothing to clip
        expect(
            rect.right,
            `${describeElement(element)} extends to ${rect.right.toFixed(1)}px, past the ` +
                `${root.clientWidth}px viewport`,
        ).toBeLessThanOrEqual(root.clientWidth + 0.5)
    }
}

// Doubled at render, not baked into style.css: this proves whatever text
// size is actually in effect can grow to 200% without a fixed-height
// container clipping it, rather than re-asserting the token values
// styles.test.ts already pins from DESIGN.md literals.
const textTokens = [
    '--text-display', '--text-headline', '--text-body', '--text-body-strong',
    '--text-label', '--text-meta', '--text-code', '--text-control',
]

function doubleTextTokens(): void {
    const root = document.documentElement
    const computed = getComputedStyle(root)
    for (const token of textTokens) {
        const px = Number.parseFloat(computed.getPropertyValue(token))
        if (Number.isNaN(px)) throw new Error(`${token} did not resolve to a px value to double`)
        root.style.setProperty(token, `${px * 2}px`)
    }
}

const textSpacingStyleId = 'wcag-1-4-12-overrides'

/*
  WCAG 1.4.12 Text Spacing, applied the way the success criterion means it.

  `DESIGN.md` and `EXPERIENCE.md` both require the layout to survive these four
  overrides, and EXPERIENCE.md names the values: 1.5x line height, 2x paragraph
  spacing, 0.12em letter spacing, 0.16em word spacing. The epic required it and
  nothing checked it -- found by the Epic 3 retrospective (A6).

  This is not the type ramp, and it is deliberately not done the way
  doubleTextTokens does its job. 1.4.12 is about a *reader's* stylesheet
  defeating the author's spacing, so the overrides are injected as a stylesheet
  with `!important`, exactly as the WCAG bookmarklet does. They have to be:
  style.css declares `letter-spacing` on three selectors, and an author-level
  rule of equal specificity would simply lose to it, which would leave this
  suite measuring the shipped spacing and calling it a pass.
*/
function applyTextSpacingOverrides(): void {
    const sheet = document.createElement('style')
    sheet.id = textSpacingStyleId
    sheet.textContent = `
        * {
            line-height: 1.5 !important;
            letter-spacing: 0.12em !important;
            word-spacing: 0.16em !important;
        }
        p {
            margin-block-end: 2em !important;
        }
    `
    document.head.append(sheet)
}

/**
 * Proves the overrides are in effect before anything concludes from them.
 *
 * A stylesheet that failed to apply -- a typo in a property, a rule the engine
 * dropped, an id the cleanup removed too early -- would leave every assertion
 * below measuring the shipped layout and reporting a pass. So the spacing is
 * read back off a real rendered element and checked against its own font size,
 * which is what `em` resolves against.
 */
function assertTextSpacingIsInEffect(container: HTMLElement): void {
    const sample = container.querySelector<HTMLElement>('.fd-meta') ?? container
    const style = getComputedStyle(sample)
    const fontSize = Number.parseFloat(style.fontSize)

    expect(fontSize, 'a resolved font size to measure the em overrides against').toBeGreaterThan(0)
    expect(
        Number.parseFloat(style.letterSpacing) / fontSize,
        `letter-spacing on ${describeElement(sample)} (${style.letterSpacing} at ${style.fontSize})`,
    ).toBeCloseTo(0.12, 2)
    expect(
        Number.parseFloat(style.wordSpacing) / fontSize,
        `word-spacing on ${describeElement(sample)} (${style.wordSpacing} at ${style.fontSize})`,
    ).toBeCloseTo(0.16, 2)
    expect(
        Number.parseFloat(style.lineHeight) / fontSize,
        `line-height on ${describeElement(sample)} (${style.lineHeight} at ${style.fontSize})`,
    ).toBeCloseTo(1.5, 2)
}

/**
 * DESIGN.md: "Content containers grow under 200% text ... no fixed height
 * may clip content." An element whose own computed overflow is hidden and
 * whose scrollHeight has grown past its clientHeight is exactly that failure,
 * named with both measurements.
 *
 * Three selectors are excluded on purpose, not by omission: `.fd-clamp` is
 * the one deliberate two-line clamp DESIGN.md permits, shipped with a
 * persistent keyboard control that reaches the full value (proven in
 * StagedView.test.tsx) -- flagging it here would fail a feature, not a
 * regression. `.fd-status-announcer` and `.fd-visually-hidden` are pinned to
 * 1x1px on purpose so their content never becomes visible to a sighted user
 * at any text size; "clipped" is their entire job, not a bug 200% text could
 * cause.
 *
 * A fourth case, added Story 9.1: a *collapsed* `.fd-disclosure__region`
 * (one with no `data-open`) is excluded the same way, and only while
 * collapsed -- an open one is not, and still has to grow. Before this story
 * a closed disclosure's content sat behind native `<details>`'s own
 * `display: none`, which reports a matching zero scrollHeight and
 * clientHeight and so never tripped this loop at all; the controlled
 * region that replaced it keeps the same content mounted and measurable so
 * `grid-template-rows` has something to animate, which means it legitimately
 * clips to zero height while closed -- that is the collapse working, not a
 * fixed-height bug. `.closest()` catches the region and everything nested
 * inside it (`.fd-disclosure__region-inner`, `.fd-disclosure__body`, every
 * string underneath), because the whole subtree is equally and deliberately
 * clipped for the same reason.
 *
 * Counted rather than assumed: with those four excluded, the views render
 * **no** element whose computed overflow is hidden today, so this loop reaches
 * its expect zero times. That is the correct answer and not a passing test --
 * it is a standing guard, and it is proved armed by a mutation: giving
 * `.fd-hero__details` a fixed block-size and hidden overflow fails here,
 * naming the element and both measurements. Nothing downstream should read
 * its silence as evidence that 200% text was checked for clipping; the
 * overflow assertion above is what measures this suite's 200% case.
 */
function assertNoFixedHeightClipsGrownText(container: HTMLElement): void {
    for (const element of container.querySelectorAll<HTMLElement>('*')) {
        if (
            element.classList.contains('fd-clamp') ||
            element.classList.contains('fd-status-announcer') ||
            element.classList.contains('fd-visually-hidden') ||
            element.closest('.fd-disclosure__region:not([data-open])') !== null
        ) continue

        const style = getComputedStyle(element)
        if (style.overflow !== 'hidden' && style.overflowY !== 'hidden') continue

        expect(
            element.scrollHeight,
            `${describeElement(element)} clips its content at 200% text: scrollHeight ` +
                `(${element.scrollHeight}px) exceeds its own clientHeight (${element.clientHeight}px)`,
        ).toBeLessThanOrEqual(element.clientHeight + 0.5)
    }
}

/** Fails naming the control and its measured size (D-068's own wording). */
function assertEveryTargetMeetsTheFloor(container: HTMLElement): void {
    const targets = container.querySelectorAll<HTMLElement>('.fd-target')
    expect(targets.length, 'no .fd-target controls were found to measure').toBeGreaterThan(0)

    for (const target of targets) {
        const rect = target.getBoundingClientRect()
        expect(
            rect.width,
            `${describeElement(target)} is ${rect.width.toFixed(1)}px wide, under the 44px activation floor`,
        ).toBeGreaterThanOrEqual(44)
        expect(
            rect.height,
            `${describeElement(target)} is ${rect.height.toFixed(1)}px tall, under the 44px activation floor`,
        ).toBeGreaterThanOrEqual(44)
    }
}

/*
  forced-colors is a media feature, not something the cross-provider `page`
  object exposes (it is not part of the common surface shared with
  webdriverio/preview) -- so it goes through the playwright provider's raw
  CDP session instead. This ties the check to the playwright provider, which
  is the one the spec approved.
*/
async function setForcedColorsActive(active: boolean): Promise<void> {
    const session = cdp()
    await session.send('Emulation.setEmulatedMedia', {
        features: active ? [{name: 'forced-colors', value: 'active'}] : [],
    })
}

describe('reflow at 320 CSS pixels (D-068)', () => {
    it('keeps Staged scrolling only vertically, with nothing clipped', async () => {
        await page.viewport(320, 900)
        assertNoHorizontalOverflow(await renderStaged())
    })

    it('keeps Transferring scrolling only vertically, with nothing clipped', async () => {
        await page.viewport(320, 900)
        assertNoHorizontalOverflow(await renderTransferring())
    })

    it('keeps the open browse menu scrolling only vertically, with nothing clipped', async () => {
        await page.viewport(320, 900)
        const container = await renderIdleMenuOpen()
        assertNoHorizontalOverflow(container)
        // The name promised "nothing clipped" and only the overflow half was
        // measured. Both halves now are.
        assertNoFixedHeightClipsGrownText(container)
    })
})

describe('200% text (D-068)', () => {
    it('grows Staged without a horizontal scrollbar or a clipped container', async () => {
        await page.viewport(1024, 900)
        doubleTextTokens()
        const container = await renderStaged()
        assertNoHorizontalOverflow(container)
        assertNoFixedHeightClipsGrownText(container)
    })

    it('grows Transferring without a horizontal scrollbar or a clipped container', async () => {
        await page.viewport(1024, 900)
        doubleTextTokens()
        const container = await renderTransferring()
        assertNoHorizontalOverflow(container)
        assertNoFixedHeightClipsGrownText(container)
    })
})

/*
  The half of the epic's declared requirement that shipped unchecked.

  epic-3-context.md requires "200% text with text-spacing overrides"; DESIGN.md
  says content containers grow under both; EXPERIENCE.md spells the four values
  out. Story 3.12 delivered the 200% half because its own I/O matrix named 320
  pixels, 200% text, targets and forced colors and never named text spacing --
  so no story was wrong and the requirement stayed half met until the Epic 3
  retrospective counted it (A6).

  320 pixels is included on purpose rather than for symmetry: wider letters,
  words and lines are hardest to fit in the narrowest column, and DESIGN.md's
  own sentence pairs the reflow floor with the spacing overrides.
*/
describe('WCAG 1.4.12 text-spacing overrides (D-068)', () => {
    it('keeps Staged unclipped under all four overrides', async () => {
        await page.viewport(1024, 900)
        applyTextSpacingOverrides()
        const container = await renderStaged()

        assertTextSpacingIsInEffect(container)
        assertNoHorizontalOverflow(container)
        assertNoFixedHeightClipsGrownText(container)
    })

    it('keeps Transferring unclipped under all four overrides', async () => {
        await page.viewport(1024, 900)
        applyTextSpacingOverrides()
        const container = await renderTransferring()

        assertTextSpacingIsInEffect(container)
        assertNoHorizontalOverflow(container)
        assertNoFixedHeightClipsGrownText(container)
    })

    it('keeps Staged unclipped with the overrides at the 320-pixel reflow floor', async () => {
        await page.viewport(320, 900)
        applyTextSpacingOverrides()
        const container = await renderStaged()

        assertTextSpacingIsInEffect(container)
        assertNoHorizontalOverflow(container)
        assertNoFixedHeightClipsGrownText(container)
    })

    it('keeps the overrides and 200% text survivable together', async () => {
        await page.viewport(1024, 900)
        doubleTextTokens()
        applyTextSpacingOverrides()
        const container = await renderStaged()

        assertTextSpacingIsInEffect(container)
        assertNoHorizontalOverflow(container)
        assertNoFixedHeightClipsGrownText(container)
    })
})

/*
  The first floating surface, at the sizes the product actually runs at.

  The menu was measured at 320x900 and 1024x900 and nowhere else, which left
  two gaps the review named. It is a fixed-width panel of full-width rows, so
  doubling every text token is exactly what should overflow it if anything
  does. And it opens downward only -- `inset-block-start: 100%`, no flip, no
  max-height, no scroll -- while sitting low in Idle, below the drop zone, the
  firewall preflight and any retained outcome. Idle is already taller than the
  640x480 minimum main.go sets, so at that size the window scrolls and "past
  the bottom edge" is not the failure to look for; a menu that detaches from
  its control, or that grows taller than the window it opens in, is.
*/
describe('the browse menu at the sizes the app really runs at (D-068)', () => {
    it('survives 200% text without overflowing or clipping', async () => {
        await page.viewport(1024, 900)
        doubleTextTokens()
        const container = await renderIdleMenuOpen()

        assertNoHorizontalOverflow(container)
        assertNoFixedHeightClipsGrownText(container)
    })

    /*
      Both measurements here are differences, and that is deliberate.

      Opening the menu moves focus into its first item, and Chromium scrolls a
      newly focused element into view. `getBoundingClientRect()` is
      viewport-relative, so after that scroll the menu's `bottom` sits flush
      against the viewport edge wherever it was actually laid out -- this case
      first asserted `bottom <= window.innerHeight` and a mutation pushing the
      menu 600px down the page still passed, at `scrollY = 723`. Any assertion
      comparing one viewport-relative coordinate against the viewport is
      measuring the browser's scroll-into-view, not this menu's layout.

      A gap between two rects taken after the same scroll, and a height, are
      both scroll-invariant, and between them they are the claim: the menu
      hangs directly off the control that opened it, and the whole of it is
      small enough to be read at once in the smallest window main.go allows.
    */
    it('hangs off its control and fits the 640x480 minimum main.go sets', async () => {
        await page.viewport(640, 480)
        const container = await renderIdleMenuOpen()

        assertNoHorizontalOverflow(container)

        const menu = container.querySelector<HTMLElement>('.fd-browse-menu')
        if (menu === null) throw new Error('.fd-browse-menu did not render')
        const trigger = screen.getByRole('button', {name: 'Choose a file or folder'})
        const gap = menu.getBoundingClientRect().top - trigger.getBoundingClientRect().bottom

        expect(
            gap,
            `the open menu starts ${gap.toFixed(1)}px below the control it hangs from: it opens ` +
                'downward only, with no flip, so a menu detached from its control is one the sender ' +
                'has to go looking for',
        ).toBeGreaterThanOrEqual(-0.5)
        expect(gap, 'the same, in the other direction').toBeLessThanOrEqual(24)

        const height = menu.getBoundingClientRect().height
        expect(
            height,
            `the open menu is ${height.toFixed(1)}px tall in a ${window.innerHeight}px window: it ` +
                'carries no max-height and no scroll of its own, so a menu taller than the window ' +
                'could not be read in one piece',
        ).toBeLessThanOrEqual(window.innerHeight)
    })
})

describe('the 44px activation floor (D-068)', () => {
    it('measures every Staged control at or above 44x44 CSS pixels', async () => {
        await page.viewport(1024, 900)
        assertEveryTargetMeetsTheFloor(await renderStaged())
    })

    it('measures every Transferring control at or above 44x44 CSS pixels', async () => {
        await page.viewport(1024, 900)
        assertEveryTargetMeetsTheFloor(await renderTransferring())
    })

    it('measures the browse control and every open menu item at or above 44x44 CSS pixels', async () => {
        await page.viewport(1024, 900)
        assertEveryTargetMeetsTheFloor(await renderIdleMenuOpen())
    })
})

describe('forced colors (D-065)', () => {
    it('keeps the forced-color-adjust exemption on the QR substrate and nowhere else', async () => {
        await page.viewport(1024, 900)
        await setForcedColorsActive(true)
        const container = await renderStaged()

        for (const element of container.querySelectorAll<HTMLElement>('*')) {
            const adjust = getComputedStyle(element).getPropertyValue('forced-color-adjust')
            const exempt = element.classList.contains('fd-qr-panel') || element.classList.contains('fd-qr')

            if (exempt) {
                expect(adjust, describeElement(element)).toBe('none')
            } else {
                expect(
                    adjust,
                    `${describeElement(element)} computes forced-color-adjust: none outside the QR substrate`,
                ).not.toBe('none')
            }
        }
    })

    it('keeps the open browse menu inside the system palette, with no exemption of its own', async () => {
        await page.viewport(1024, 900)
        await setForcedColorsActive(true)
        const container = await renderIdleMenuOpen()

        for (const element of container.querySelectorAll<HTMLElement>('*')) {
            const adjust = getComputedStyle(element).getPropertyValue('forced-color-adjust')
            expect(
                adjust,
                `${describeElement(element)} computes forced-color-adjust: none -- the menu carries no exemption`,
            ).not.toBe('none')
        }
        assertNoHorizontalOverflow(container)
    })

    it('captures the QR panel under forced colors -- a rendered capture, not a scan', async () => {
        await page.viewport(1024, 900)
        await setForcedColorsActive(true)
        const container = await renderStaged()
        const panel = container.querySelector('.fd-qr-panel')
        if (panel === null) throw new Error('.fd-qr-panel did not render')

        /*
          This proves the substrate still paints light-on-dark, bitmap and
          quiet zone visually distinct, under forced colors. It does not
          prove a camera decodes it -- that claim is made nowhere in this
          repo. DESIGN.md gates the forced-color-adjust exemption on native
          scan evidence that still does not exist; this screenshot is the
          other half the spec's Design Notes describe, named "capture" rather
          than "scan" so nothing downstream can mistake one for the other.
        */
        await page.screenshot({
            element: panel,
            path: 'captures/qr-panel-forced-colors.capture.png',
        })
    })
})

describe('QR drag source is disabled (macOS drag-and-drop crash)', () => {
    /*
      Owner report: dragging the QR image after staging a file crashes and
      closes the whole app. Diagnosis from the vendored Wails source
      (WailsWebView.m, performDragOperation:) rather than an instrumented
      reproduction -- the crash itself is a macOS/Cocoa fatality this repo's
      suites cannot trigger, jsdom performs no drag pasteboard at all and
      Chromium is a different engine with different drag-source behaviour.

      The QR `<img>` is a `data:image/png;base64,...` URL. WebKit lets any
      `<img>` be dragged by default, which puts that URL on the OS drag
      pasteboard as an NSURL. Wails' native drop handler then calls
      `fileSystemRepresentation` on every NSURL on the pasteboard
      unconditionally, with no guard for a non-file URL -- an Objective-C
      exception in that call is fatal inside the cgo process hosting WebKit,
      which matches the reported crash with no Go panic and no crash report.

      Nothing here can execute that native code path, so what is pinned is
      the *mechanism* that prevents WebKit from ever starting the drag:
      `draggable={false}` (the standard HTML attribute WebKit's own default
      drag-start check consults) and `-webkit-user-drag: none` (the CSS
      property WebKit actually honours for `<img>` regardless of the
      attribute). Losing either one re-opens the crash.
    */
    it('marks the QR image non-draggable both ways, so WebKit never starts a drag pasteboard', async () => {
        const container = await renderStaged()
        const qr = container.querySelector<HTMLImageElement>('.fd-qr')
        if (qr === null) throw new Error('.fd-qr did not render')

        expect(
            qr.draggable,
            'the QR <img> is draggable -- dragging it can crash the app (fileSystemRepresentation ' +
                'on a non-file NSURL in WailsWebView.m performDragOperation:); set draggable={false}',
        ).toBe(false)

        const userDrag = getComputedStyle(qr).getPropertyValue('-webkit-user-drag').trim()
        expect(
            userDrag,
            'the QR <img> computes -webkit-user-drag other than none -- WebKit can still start a drag ' +
                'that crashes the app (fileSystemRepresentation on a non-file NSURL in WailsWebView.m ' +
                'performDragOperation:); add -webkit-user-drag: none to .fd-qr',
        ).toBe('none')
    })
})

describe('a phase view settles to a fully opaque, untranslated resting state (Story 9.1)', () => {
    /*
      styles.test.ts proves the entrance rule's text -- the resting `opacity:
      1`/`translate: none` declaration and its @starting-style counterpart --
      but jsdom evaluates no transition and settles no @starting-style state
      at all, so it cannot prove a view actually *arrives* rather than
      getting stranded partway. This is that proof, in real Chromium layout:
      mount a phase view, wait for its entrance transition to run to
      completion, and read what the browser actually computed.

      Mutation (acceptance criteria): make the resting rule `opacity: 0` --
      i.e. change the *end* state a transition settles to, not merely the
      @starting-style it begins from -- and this must fail: a view that never
      finishes arriving is exactly the defect an entrance rule must not be
      able to produce. `waitForEntranceToSettle` is the same helper
      `renderIdleMenuOpen` above uses to wait out the browse menu's own
      entrance, reused rather than redefined.
    */
    it('leaves the Idle phase view fully opaque and untranslated once its entrance settles', async () => {
        const {container} = render(
            <IdleView
                state={idle()}
                dropTargetStyle={dropTargetStyle}
                cancelWon={false}
                onSelectFile={() => undefined}
                onSelectDirectory={() => undefined}
            />,
        )
        const view = container.querySelector('[data-phase-view="idle"]')
        if (view === null) throw new Error('[data-phase-view="idle"] did not render')

        await waitForEntranceToSettle(view)

        const style = getComputedStyle(view)
        expect(style.opacity).toBe('1')
        expect(style.translate).toBe('none')
    })

    it('leaves the Staged phase view fully opaque and untranslated once its entrance settles', async () => {
        const container = await renderStaged()
        const view = container.querySelector('[data-phase-view="staged"]')
        if (view === null) throw new Error('[data-phase-view="staged"] did not render')

        await waitForEntranceToSettle(view)

        const style = getComputedStyle(view)
        expect(style.opacity).toBe('1')
        expect(style.translate).toBe('none')
    })
})
