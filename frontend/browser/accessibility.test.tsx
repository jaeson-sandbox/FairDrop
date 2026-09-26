import type {} from '@vitest/browser-playwright' // pulls in the CDPSession#send() augmentation for the playwright provider
import type {CSSProperties} from 'react'
import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {cdp, page} from 'vitest/browser'
import {afterEach, describe, expect, it, vi} from 'vitest'
import type {IdleTransferState, StagedTransferState, TransferringTransferState} from '../src/transfer/state'
import type {FileMetadata, ProgressSnapshot} from '../src/transfer/types'
import {IdleView} from '../src/ui/IdleView'
import {OutcomePanel} from '../src/ui/OutcomePanel'
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

/**
 * Staged wrapped in a stand-in for `.fd-app` (App.tsx's real shell), the
 * same reason `renderIdleInAppShell` above exists: `.fd-region`'s vertical
 * centering (Story 9.6 review follow-up: `justify-content: center` on
 * `.fd-region[data-phase-view='staged']`) resolves against `.fd-app`'s own
 * height, which needs a real, definite ancestor height to grow into and
 * centre within -- `renderStaged` mounts `StagedView` as the render root
 * with no such ancestor, so there is no free space for the centering rule
 * to distribute at all, and a measurement taken against it reads as
 * top-pinned regardless of whether the rule is present.
 */
async function renderStagedInAppShell(): Promise<HTMLElement> {
    const {container} = render(
        <div className="fd-app" style={{height: '100vh'}}>
            <StagedView state={staged()} onCancel={() => undefined}/>
        </div>,
    )
    await waitForEntranceToSettle(container)
    return container
}

/**
 * Story 9.4 defect fix: the orchestrator's rendered 1024x768 review used
 * "dev-environment-guide.html" (the owner-approved prototype's own item
 * name) to observe the item name wrapping mid-word once the details column
 * was crushed into the QR's 216px track. Kept as its own helper rather than
 * a `staged()` override at each call site, so the name a defect was
 * actually observed with is named once, here.
 */
async function renderStagedNamed(name: string): Promise<HTMLElement> {
    const state = staged({metadata: metadata({name})})
    const {container} = render(<StagedView state={state} onCancel={() => undefined}/>)
    await waitForEntranceToSettle(container)
    return container
}

/**
 * True when every character of `element`'s text content renders on a single
 * visual line -- a `Range` spanning its contents reports one client rect per
 * line it wraps across, so more than one rect is a direct measurement of
 * wrapping rather than an inference from height (which a taller font or
 * line-height could satisfy by accident either way).
 */
function isSingleLine(element: Element): boolean {
    const range = document.createRange()
    range.selectNodeContents(element)
    return range.getClientRects().length <= 1
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

/** Plain Idle, menu closed -- for measurements that do not involve the menu. */
async function renderIdle(): Promise<HTMLElement> {
    const {container} = render(
        <IdleView
            state={idle()}
            dropTargetStyle={dropTargetStyle}
            cancelWon={false}
            onSelectFile={() => undefined}
            onSelectDirectory={() => undefined}
            commandErrorPanelProps={{}}
        />,
    )
    await waitForEntranceToSettle(container)
    return container
}

/**
 * Idle wrapped in a stand-in for `.fd-app` (`App.tsx`'s real shell, not
 * reproduced by `renderIdle` above) with an explicit `100vh` height.
 *
 * `.fd-region` and `.fd-idle` both rely on `flex: 1 1 auto` to grow into
 * whatever height `.fd-app` provides (`min-height: 100%`, which itself only
 * resolves against a definite ancestor height) -- that is the vertical slack
 * the drop-zone spacing defect below is actually about. `renderIdle` mounts
 * `IdleView` as the render root with no such ancestor, so `.fd-app`'s
 * `min-height: 100%` computes against an auto-height container and resolves
 * to nothing: every flex-grow box up the chain then sizes to its own
 * content, no free space exists to distribute, and the spacing bug this
 * measures cannot reproduce at all. The explicit inline `height: 100vh` here
 * is what gives that chain a real, viewport-relative height to grow into,
 * the way the actual app shell does.
 */
async function renderIdleInAppShell(): Promise<HTMLElement> {
    const {container} = render(
        <div className="fd-app" style={{height: '100vh'}}>
            <IdleView
                state={idle()}
                dropTargetStyle={dropTargetStyle}
                cancelWon={false}
                onSelectFile={() => undefined}
                onSelectDirectory={() => undefined}
                commandErrorPanelProps={{}}
            />
        </div>,
    )
    await waitForEntranceToSettle(container)
    return container
}

/**
 * A retained Done outcome, wrapped in the same `.fd-app` stand-in
 * `renderIdleInAppShell` uses -- Story 9.6's card replaces Idle's own
 * composition, so this is what actually renders in Idle once one is showing.
 */
async function renderRetainedDoneOutcome(name = 'report.pdf'): Promise<HTMLElement> {
    const {container} = render(
        <div className="fd-app" style={{height: '100vh'}}>
            <OutcomePanel
                outcome={{kind: 'done', retained: true, receipt: {name, isDir: false, bytesSent: 8_400_000}}}
                dropTargetStyle={dropTargetStyle}
                onDismiss={() => undefined}
                browse={{label: 'Send Another', onSelectFile: () => undefined, onSelectDirectory: () => undefined}}
            />
        </div>,
    )
    await waitForEntranceToSettle(container)
    return container
}

/** A live terminal Error outcome (the App-level phase view, `phaseView`/`level=1`), Try Again plus Dismiss. */
async function renderLiveErrorOutcome(): Promise<HTMLElement> {
    const {container} = render(
        <div className="fd-app" style={{height: '100vh'}}>
            <OutcomePanel
                outcome={{
                    kind: 'error',
                    retained: false,
                    error: {code: 'transfer_failed', message: 'x'},
                    itemName: 'report.pdf',
                }}
                level={1}
                phaseView
                dropTargetStyle={dropTargetStyle}
                onDismiss={() => undefined}
                onRetry={() => undefined}
            />
        </div>,
    )
    await waitForEntranceToSettle(container)
    return container
}

/** An Idle Stage-time command failure -- the card that replaces Idle's whole composition (Story 9.6). */
async function renderIdleCommandFailure(): Promise<HTMLElement> {
    const {container} = render(
        <div className="fd-app" style={{height: '100vh'}}>
            <IdleView
                state={{
                    phase: 'idle',
                    retainedOutcome: null,
                    commandError: {code: 'invalid_selection', message: 'Choose exactly one file or folder.'},
                }}
                dropTargetStyle={dropTargetStyle}
                cancelWon={false}
                onSelectFile={() => undefined}
                onSelectDirectory={() => undefined}
                commandErrorPanelProps={{
                    dropTargetStyle,
                    onDismiss: () => undefined,
                    browse: {label: 'Choose Another', onSelectFile: () => undefined, onSelectDirectory: () => undefined},
                }}
            />
        </div>,
    )
    await waitForEntranceToSettle(container)
    return container
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
            commandErrorPanelProps={{}}
        />,
    )
    fireEvent.click(screen.getByRole('button', {name: 'Choose File or Folder'}))
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
 * Two selectors are excluded on purpose, not by omission: `.fd-status-announcer`
 * and `.fd-visually-hidden` are pinned to 1x1px on purpose so their content
 * never becomes visible to a sighted user at any text size; "clipped" is
 * their entire job, not a bug 200% text could cause. (Story 9.4 removed a
 * third case, `.fd-clamp` -- the two-line name clamp behind a persistent
 * "Show full name" toggle -- along with the toggle itself: the item name
 * now always wraps, so nothing needs an exemption for it any more.)
 *
 * A third case, added Story 9.1: a *collapsed* `.fd-disclosure__region`
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
 * A fourth case, added Story 9.4: a *collapsed* `.fd-url-reveal` (Staged's
 * hidden-until-requested direct-link field) is excluded for exactly the same
 * reason and in exactly the same shape -- it is the identical
 * grid-template-rows/visibility mechanism, just not wrapped in the
 * `Disclosure` component itself (the reveal trigger is a plain button beside
 * Copy Link, not a heading-wrapped summary). Revealed (`[data-open]`), it is
 * not excluded and still has to grow -- Story 7.9's guarantee applies to the
 * revealed field exactly as it always did.
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
            element.classList.contains('fd-status-announcer') ||
            element.classList.contains('fd-visually-hidden') ||
            element.closest('.fd-disclosure__region:not([data-open])') !== null ||
            element.closest('.fd-url-reveal:not([data-open])') !== null
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
        const trigger = screen.getByRole('button', {name: 'Choose File or Folder'})
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
        const container = await renderStaged()
        // Story 9.4: the direct-link field is collapsed (`.fd-url-reveal`,
        // no `data-open`) until Show Link is activated, and a collapsed
        // `.fd-target` legitimately measures 0x0 -- that is the collapse
        // working, not an activation floor regression. Reveal it first,
        // and wait out its own expansion transition the same way the
        // initial render's entrance is awaited above, so this measures the
        // field's settled size rather than a mid-transition one.
        fireEvent.click(screen.getByRole('button', {name: 'Show Link'}))
        await waitForEntranceToSettle(container)
        assertEveryTargetMeetsTheFloor(container)
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

/*
  Defect found by the orchestrator driving the built macOS binary of the
  epic branch, after Story 9.1 merged: the disclosure chevron sat
  immediately after the label text ("Local network access >") instead of at
  the row's right edge, as it did in 1.2.1 and as the owner-approved
  prototype's `.row` shows. Cause: Story 9.1 replaced <summary> (whose
  containing <h2> carried `flex: 1`, pushing the chevron to the far end)
  with a <button> that never got an equivalent rule.

  styles.test.ts already pins the fix's *text* (`justify-content:
  space-between` on `.fd-disclosure__summary`); this is the rendered proof
  that the fix actually lands the chevron at the edge, in real Chromium
  layout, not merely that the declaration exists in the sheet.
*/
describe("the disclosure chevron sits at the row's trailing edge (Story 9.3 defect fix)", () => {
    it('keeps the chevron within the row\'s own right padding of the summary\'s right edge', async () => {
        await page.viewport(1024, 900)
        const container = await renderIdle()

        const summary = container.querySelector<HTMLElement>('.fd-preflight .fd-disclosure__summary')
        if (summary === null) throw new Error('.fd-preflight .fd-disclosure__summary did not render')
        const chevron = summary.querySelector<HTMLElement>('.fd-disclosure__chevron')
        if (chevron === null) throw new Error('.fd-disclosure__chevron did not render')

        const paddingRight = Number.parseFloat(getComputedStyle(summary).paddingRight)
        expect(paddingRight, 'a resolved right padding to measure the gap against').toBeGreaterThan(0)

        const gap = summary.getBoundingClientRect().right - chevron.getBoundingClientRect().right
        const rowWidth = summary.getBoundingClientRect().width

        /*
          Not a tight match against the padding token: the chevron is a 12px
          box rotated 45deg, and a rotated box's own bounding rect is its
          diagonal (~17px), not its unrotated side -- so the measured gap
          runs a few pixels inside the padding value itself, consistently,
          which a tight tolerance would flag as noise. What actually
          distinguishes "at the row's trailing edge" from the defect is
          scale: fixed, the gap is on the order of the row's own padding
          (well under half the row's width); broken (chevron packed right
          after the label with `justify-content` back at its flex-start
          default), the gap is most of the row's remaining width instead.
          Mutation: remove `justify-content: space-between` from
          `.fd-disclosure__summary` -> the gap grows past this bound and the
          assertion fails, naming the measured value.
        */
        expect(
            gap,
            `the chevron's right edge sits ${gap.toFixed(1)}px inside the ${rowWidth.toFixed(1)}px-wide ` +
                `summary's own right edge (right padding: ${paddingRight.toFixed(1)}px) -- too far from the ` +
                'edge to read as "trailing", which is what a chevron packed right after the label instead would look like',
        ).toBeLessThanOrEqual(paddingRight + 6)
        expect(gap, 'the chevron must not sit flush against or past the summary\'s own edge').toBeGreaterThanOrEqual(0)
    })
})

/*
  Defect found by the orchestrator driving the built macOS binary after Story
  9.3 merged: the centred Idle pill's trailing chevron (`.fd-browse-trigger__chevron`,
  a 12x12 box rotated 45deg) rendered touching or overlapping the final "r" of
  "Choose File or Folder". Cause: the chevron's only spacing rule was
  `margin-inline-start: auto`, which produces a gap only when its flex
  container (the button) has spare inline space to distribute to that auto
  margin -- and the pill is `display: inline-flex` sized to its own content,
  so there never is any spare space to distribute. A rotated 12px square's own
  bounding box is its diagonal, ~17px, not its unrotated 12px side, so even the
  near-zero gap `auto` degrades to reads as an overlap rather than a narrow
  miss.

  Measured against a Range over the label's own text node, not the button's
  bounding rect, because the button's content box already includes the
  chevron: measuring button-right-edge-to-chevron would always read as
  whatever padding the button has, regardless of whether the chevron actually
  clears the label.

  Mutation: put `.fd-browse-trigger__chevron`'s `margin-inline-start` back to
  `auto` -> the gap collapses to (near) 0px and this fails, naming the
  measured value.
*/
describe('the browse pill keeps its chevron clear of its label (defect fix)', () => {
    it.each([
        ['the default viewport', 1024, 900],
        ['the 640x480 minimum main.go sets', 640, 480],
    ])('at %s, the chevron sits clear of the label and inside the button', async (_name, width, height) => {
        await page.viewport(width, height)
        await renderIdle()

        const trigger = screen.getByRole('button', {name: 'Choose File or Folder'})
        const chevron = trigger.querySelector<HTMLElement>('.fd-browse-trigger__chevron')
        if (chevron === null) throw new Error('.fd-browse-trigger__chevron did not render')

        const textNode = [...trigger.childNodes].find((node) => node.nodeType === Node.TEXT_NODE)
        if (textNode === undefined) {
            throw new Error('expected the label\'s own text node as a direct child of the trigger button')
        }
        const range = document.createRange()
        range.selectNodeContents(textNode)
        const textRect = range.getBoundingClientRect()
        const chevronRect = chevron.getBoundingClientRect()
        const triggerRect = trigger.getBoundingClientRect()

        const gap = chevronRect.left - textRect.right
        expect(
            gap,
            `the chevron's left edge sits ${gap.toFixed(1)}px past the label text's own right edge ` +
                `(text right: ${textRect.right.toFixed(1)}px, chevron left: ${chevronRect.left.toFixed(1)}px): ` +
                'a rotated 12px box has a ~17px diagonal, so anything under 6px reads as touching or ' +
                'overlapping the label',
        ).toBeGreaterThanOrEqual(6)

        expect(
            chevronRect.left,
            'the chevron must not start left of the button it belongs to',
        ).toBeGreaterThanOrEqual(triggerRect.left - 0.5)
        expect(
            chevronRect.right,
            'the chevron must not spill past the button\'s right edge',
        ).toBeLessThanOrEqual(triggerRect.right + 0.5)
        expect(
            chevronRect.top,
            'the chevron must not spill above the button',
        ).toBeGreaterThanOrEqual(triggerRect.top - 0.5)
        expect(
            chevronRect.bottom,
            'the chevron must not spill below the button',
        ).toBeLessThanOrEqual(triggerRect.bottom + 0.5)
    })
})

/*
  Defect found by the orchestrator driving the built macOS binary after Story
  9.3 merged: Idle's drop zone contents (glyph, heading, promise line, browse
  pill) read as spread out, with 50-70px gaps between the heading, the promise
  line and the pill. Cause: `.fd-drop-zone__inner` is `display: grid` with no
  `align-content` declared, so it computes to Grid's initial `normal`, which
  behaves as `stretch` for auto-sized row tracks -- the container's free block
  space (it grows to fill `.fd-drop-zone`, itself a flex item with `flex: 1 1
  auto`) was distributed equally across the four implicit rows instead of
  packing them together as a group.

  Mutation: remove `align-content: center` from `.fd-drop-zone__inner` (or
  widen the per-element margins pinned below back toward the original grid
  default) -> the gaps this measures grow past their bound and this fails,
  naming the measured value.
*/
describe('the drop zone packs its contents together instead of spreading them out (defect fix)', () => {
    it.each([
        ['the default viewport', 1024, 900],
        ['the 640x480 minimum main.go sets', 640, 480],
    ])('at %s, keeps the heading, promise line and pill close together', async (_name, width, height) => {
        await page.viewport(width, height)
        const container = await renderIdleInAppShell()

        const heading = container.querySelector<HTMLElement>('.fd-state-heading')
        if (heading === null) throw new Error('.fd-state-heading did not render')
        const promise = container.querySelector<HTMLElement>('.fd-drop-zone__inner .fd-meta')
        if (promise === null) throw new Error('.fd-drop-zone__inner .fd-meta did not render')
        const pill = screen.getByRole('button', {name: 'Choose File or Folder'})

        const headingToPromise = promise.getBoundingClientRect().top - heading.getBoundingClientRect().bottom
        const promiseToPill = pill.getBoundingClientRect().top - promise.getBoundingClientRect().bottom

        expect(
            headingToPromise,
            `the heading's bottom edge sits ${headingToPromise.toFixed(1)}px above the promise line's top edge`,
        ).toBeLessThanOrEqual(12)
        expect(
            promiseToPill,
            `the promise line's bottom edge sits ${promiseToPill.toFixed(1)}px above the pill's top edge`,
        ).toBeLessThanOrEqual(32)
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
                commandErrorPanelProps={{}}
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

/*
  Defect fix, orchestrator's rendered 1024x768 review after Story 9.4
  merged (99df048): `.fd-hero`'s fixed track came *second*
  (`minmax(0, 1fr) 216px`) while `StagedView.tsx`'s DOM order puts
  `.fd-qr-panel` *first* -- grid assigns tracks to children in DOM order,
  so the QR actually landed in the large `1fr` track (centred in the
  leftover space) and the item details were crushed into 216px: the name
  wrapped mid-word, Copy Link and Show Link stacked vertically, and the
  caveats wrapped into narrow lines. None of the existing suites caught
  this because `styles.test.ts` only proves the CSS text contains a
  two-track template (it does, just in the wrong order) and no rendered
  test measured which track either element actually occupied.
*/
describe('the hero card assigns its fixed track to the element the DOM actually puts there (Story 9.4 defect fix)', () => {
    it.each([[1024, 768], [640, 480]])(
        'gives the details column more width than the QR tile at %ix%i',
        async (width, height) => {
            await page.viewport(width, height)
            const container = await renderStaged()

            const qr = container.querySelector('.fd-qr-panel')
            const details = container.querySelector('.fd-hero__details')
            if (qr === null || details === null) throw new Error('.fd-qr-panel or .fd-hero__details did not render')

            const qrWidth = qr.getBoundingClientRect().width
            const detailsWidth = details.getBoundingClientRect().width
            expect(
                detailsWidth,
                `the details column (${detailsWidth.toFixed(1)}px) is not wider than the QR tile ` +
                    `(${qrWidth.toFixed(1)}px) at ${width}x${height} -- the fixed 216px track landed on the ` +
                    'wrong element',
            ).toBeGreaterThan(qrWidth)
        },
    )

    it.each([[1024, 768], [640, 480]])(
        'keeps Copy Link and Show Link on the same row at %ix%i',
        async (width, height) => {
            await page.viewport(width, height)
            await renderStaged()

            const copyTop = screen.getByRole('button', {name: 'Copy Link'}).getBoundingClientRect().top
            const showTop = screen.getByRole('button', {name: 'Show Link'}).getBoundingClientRect().top
            expect(
                Math.abs(copyTop - showTop),
                `Copy Link (top ${copyTop.toFixed(1)}) and Show Link (top ${showTop.toFixed(1)}) are not on ` +
                    `the same row at ${width}x${height}`,
            ).toBeLessThanOrEqual(2)
        },
    )

    it.each([[1024, 768], [640, 480]])(
        'keeps a realistic item name on one line at %ix%i',
        async (width, height) => {
            await page.viewport(width, height)
            const container = await renderStagedNamed('dev-environment-guide.html')

            const isolate = container.querySelector('#fd-item-name bdi')
            if (isolate === null) throw new Error('#fd-item-name bdi did not render')

            expect(
                isSingleLine(isolate),
                `"dev-environment-guide.html" wraps across more than one line at ${width}x${height}`,
            ).toBe(true)
        },
    )
})

/*
  Defect fix, same rendered review: the AC's "one row" for the foot
  (the "Trouble connecting?" disclosure on the left, Cancel on the right)
  was never actually one row. `.fd-disclosure__summary` carries its own
  `width: 100%`, and a flex item whose only child demands that resolves, in
  the engine this suite runs, by filling the whole row rather than hugging
  the label text -- so the disclosure took the full row width and Cancel
  wrapped onto its own line below it.
*/
describe('the Staged foot row keeps "Trouble connecting?" and Cancel on one row (Story 9.4 defect fix)', () => {
    it.each([[1024, 768], [640, 480]])(
        'aligns the collapsed disclosure trigger and Cancel at the same top at %ix%i',
        async (width, height) => {
            await page.viewport(width, height)
            await renderStaged()

            const trigger = screen.getByRole('button', {name: 'Trouble connecting?'})
            const cancel = screen.getByRole('button', {name: 'Cancel'})
            const diff = Math.abs(trigger.getBoundingClientRect().top - cancel.getBoundingClientRect().top)
            expect(
                diff,
                `the disclosure trigger and Cancel are not on the same row at ${width}x${height} ` +
                    `(top difference ${diff.toFixed(1)}px)`,
            ).toBeLessThanOrEqual(2)
        },
    )

    it('keeps Cancel’s top unchanged when "Trouble connecting?" expands, with every help string visible', async () => {
        await page.viewport(1024, 768)
        const container = await renderStaged()

        const cancel = screen.getByRole('button', {name: 'Cancel'})
        const topBefore = cancel.getBoundingClientRect().top

        fireEvent.click(screen.getByRole('button', {name: 'Trouble connecting?'}))
        await waitForEntranceToSettle(container)

        const topAfter = cancel.getBoundingClientRect().top
        expect(
            Math.abs(topAfter - topBefore),
            `Cancel moved from top ${topBefore.toFixed(1)} to ${topAfter.toFixed(1)} when the disclosure opened`,
        ).toBeLessThanOrEqual(2)

        // The region itself, not per-string getByText: every string below
        // nests inside a <p>/<dd> whose own ancestors (the region, its
        // inner wrapper, the body) also match a substring search on
        // textContent, so a per-string element lookup finds more than one
        // node. Checking the region's own rendered size once, then its
        // aggregate text for each string, is unambiguous and exercises the
        // same "did it actually become visible" question.
        const region = document.querySelector('.fd-help .fd-disclosure__region') as HTMLElement
        const regionRect = region.getBoundingClientRect()
        expect(regionRect.width, 'the opened help region has zero rendered width').toBeGreaterThan(0)
        expect(regionRect.height, 'the opened help region has zero rendered height').toBeGreaterThan(0)

        for (const text of [
            'Open Windows Firewall settings',
            'Open System Settings',
            'Not downloading?',
            'Browser says Not Found',
            'FairDrop keeps no copy.',
            'Link previews in chat apps',
        ]) {
            expect(region.textContent, `"${text}" missing from the opened help region`).toContain(text)
        }
    })

    it('keeps the collapsed disclosure trigger at or above the 44px activation floor despite its quieter padding', async () => {
        await page.viewport(1024, 768)
        await renderStaged()

        const trigger = screen.getByRole('button', {name: 'Trouble connecting?'})
        const rect = trigger.getBoundingClientRect()
        expect(rect.height, `the disclosure trigger is ${rect.height.toFixed(1)}px tall`).toBeGreaterThanOrEqual(44)
    })
})

/**
 * Story 9.5: Sending reuses Staged's own card geometry (`.fd-hero`'s fixed
 * 216px/`minmax(0, 1fr)` grid), with a progress ring in the QR's slot instead
 * of the QR bitmap. `styles.test.ts` already proves the CSS text declares that
 * template -- what it cannot prove is which element the browser actually put
 * in the fixed track, which is exactly the class of defect the Story 9.4
 * review above this one found (a swapped column order, and a foot row that
 * wrapped) with every text-only suite green. Rendered here in the real
 * `.fd-app` shell, at both sizes the Story 9.4 defect-fix tests already use.
 */
async function renderTransferringInAppShell(overrides: Partial<TransferringTransferState> = {}): Promise<HTMLElement> {
    const {container} = render(
        <div className="fd-app" style={{height: '100vh'}}>
            <TransferringView state={transferring(overrides)} onCancel={() => undefined}/>
        </div>,
    )
    await waitForEntranceToSettle(container)
    return container
}

describe('the sending card keeps the ring in Staged’s fixed column (Story 9.5)', () => {
    it.each([[1024, 768], [640, 480]])(
        'gives the ring a ~216px column narrower than the details beside it at %ix%i',
        async (width, height) => {
            await page.viewport(width, height)
            const container = await renderTransferringInAppShell()

            const ring = container.querySelector('.fd-ring-panel')
            const details = container.querySelector('.fd-hero__details')
            if (ring === null || details === null) throw new Error('.fd-ring-panel or .fd-hero__details did not render')

            const ringWidth = ring.getBoundingClientRect().width
            const detailsWidth = details.getBoundingClientRect().width

            expect(
                ringWidth,
                `the ring column is ${ringWidth.toFixed(1)}px, not the ~216px Staged's QR slot uses at ${width}x${height}`,
            ).toBeGreaterThan(190)
            expect(ringWidth).toBeLessThan(230)
            expect(
                detailsWidth,
                `the details column (${detailsWidth.toFixed(1)}px) is not wider than the ring ` +
                    `(${ringWidth.toFixed(1)}px) at ${width}x${height} -- the fixed 216px track landed on the ` +
                    'wrong element',
            ).toBeGreaterThan(ringWidth)
        },
    )

    it.each([[1024, 768], [640, 480]])(
        'keeps a realistic item name on one line at %ix%i',
        async (width, height) => {
            await page.viewport(width, height)
            const container = await renderTransferringInAppShell({metadata: metadata({name: 'dev-environment-guide.html'})})

            const isolate = container.querySelector('#fd-item-name bdi')
            if (isolate === null) throw new Error('#fd-item-name bdi did not render')

            expect(
                isSingleLine(isolate),
                `"dev-environment-guide.html" wraps across more than one line at ${width}x${height}`,
            ).toBe(true)
        },
    )

    it.each([[1024, 768], [640, 480]])(
        'centres the determinate percentage inside the ring at %ix%i',
        async (width, height) => {
            await page.viewport(width, height)
            const container = await renderTransferringInAppShell()

            const frame = container.querySelector('.fd-ring-frame')
            const pct = container.querySelector('.fd-ring__pct')
            if (frame === null || pct === null) throw new Error('.fd-ring-frame or .fd-ring__pct did not render')

            const frameRect = frame.getBoundingClientRect()
            const pctRect = pct.getBoundingClientRect()
            const frameCenterX = frameRect.left + frameRect.width / 2
            const frameCenterY = frameRect.top + frameRect.height / 2
            const pctCenterX = pctRect.left + pctRect.width / 2
            const pctCenterY = pctRect.top + pctRect.height / 2

            expect(
                Math.abs(pctCenterX - frameCenterX),
                `the percentage is not horizontally centred in the ring at ${width}x${height}`,
            ).toBeLessThanOrEqual(4)
            expect(
                Math.abs(pctCenterY - frameCenterY),
                `the percentage is not vertically centred in the ring at ${width}x${height}`,
            ).toBeLessThanOrEqual(4)
        },
    )

    it.each([[1024, 768], [640, 480]])(
        'keeps the whole card within the viewport with nothing overflowing at %ix%i',
        async (width, height) => {
            await page.viewport(width, height)
            const container = await renderTransferringInAppShell()

            const card = container.querySelector('.fd-packet')
            if (card === null) throw new Error('.fd-packet did not render')

            const rect = card.getBoundingClientRect()
            expect(rect.left, `the card overflows the left edge at ${width}x${height} (left ${rect.left.toFixed(1)})`)
                .toBeGreaterThanOrEqual(-1)
            expect(
                rect.right,
                `the card overflows the right edge at ${width}x${height} (right ${rect.right.toFixed(1)}, viewport ${width})`,
            ).toBeLessThanOrEqual(width + 1)
        },
    )
})

/**
 * Defect fix (orchestrator's rendered 1024x768 review against the
 * owner-approved prototype, after the Story 9.5 merge, branch
 * fix-9-5-sending-details): four further visual defects the text-only
 * suites and the rendered describe block above both passed. See the
 * "Review follow-up" section of
 * evidence-9-5-rebuild-sending-around-a-ring.md for the full mutation
 * table.
 */
describe('the Sending figures and Cancel read as plain text, not boxed controls (Story 9.5 defect fix)', () => {
    it.each([[1024, 768], [640, 480]])(
        'keeps the percentage and its "%" sign on one baseline, centred together in the ring at %ix%i',
        async (width, height) => {
            await page.viewport(width, height)
            const container = await renderTransferringInAppShell()

            const frame = container.querySelector('.fd-ring-frame')
            const value = container.querySelector('.fd-ring__pct-value')
            const sign = container.querySelector('.fd-ring__pct small')
            if (frame === null || value === null || sign === null) {
                throw new Error('.fd-ring-frame, .fd-ring__pct-value or .fd-ring__pct small did not render')
            }

            const frameRect = frame.getBoundingClientRect()
            const valueRect = value.getBoundingClientRect()
            const signRect = sign.getBoundingClientRect()

            /*
              Same line: the two boxes share a text baseline (`align-items:
              baseline`), which for digits and "%" -- neither has a
              descender -- sits at each box's own *bottom* edge, not its
              centre: a 30px number and a 12.5px "%" aligned on one baseline
              still have centres several pixels apart purely from the font-
              size difference (measured here at ~6.9px, comfortably past a
              naive centre-only tolerance), so bottom is the check that
              actually distinguishes "same line" from "detached onto its own
              row" without being fooled by that. The old "centres the
              determinate percentage inside the ring" test above measured
              `.fd-ring__pct` itself, which is `position: absolute; inset: 0`
              and therefore always exactly the frame's own box regardless of
              how its children are laid out inside it -- it could not have
              caught the number and "%" landing on two separate rows, which
              is exactly what happened.
            */
            expect(
                Math.abs(valueRect.bottom - signRect.bottom),
                `the number (bottom ${valueRect.bottom.toFixed(1)}) and "%" (bottom ${signRect.bottom.toFixed(1)}) ` +
                    `are not on the same baseline at ${width}x${height}`,
            ).toBeLessThanOrEqual(5)

            // Horizontally adjacent: the "%" starts at or soon after the
            // number's own right edge, not centred independently somewhere
            // else in the frame.
            const gap = signRect.left - valueRect.right
            expect(
                gap,
                `the "%" sign (left ${signRect.left.toFixed(1)}) is not adjacent to the number's right edge ` +
                    `(${valueRect.right.toFixed(1)}) at ${width}x${height} (gap ${gap.toFixed(1)}px)`,
            ).toBeGreaterThanOrEqual(-1)
            expect(gap).toBeLessThanOrEqual(6)

            // The combined "43%" reads as centred in the ring as one unit.
            const left = Math.min(valueRect.left, signRect.left)
            const right = Math.max(valueRect.right, signRect.right)
            const top = Math.min(valueRect.top, signRect.top)
            const bottom = Math.max(valueRect.bottom, signRect.bottom)
            const combinedCenterX = (left + right) / 2
            const combinedCenterY = (top + bottom) / 2
            const frameCenterX = frameRect.left + frameRect.width / 2
            const frameCenterY = frameRect.top + frameRect.height / 2

            expect(
                Math.abs(combinedCenterX - frameCenterX),
                `the combined "43%" is not horizontally centred in the ring at ${width}x${height}`,
            ).toBeLessThanOrEqual(4)
            expect(
                Math.abs(combinedCenterY - frameCenterY),
                `the combined "43%" is not vertically centred in the ring at ${width}x${height}`,
            ).toBeLessThanOrEqual(4)
        },
    )

    it.each([[1024, 768], [640, 480]])(
        'gives each figure a plain, unboxed presentation whose value does not repeat its own caption at %ix%i',
        async (width, height) => {
            await page.viewport(width, height)
            const container = await renderTransferringInAppShell()

            const metrics = [...container.querySelectorAll('.fd-metric')]
            expect(metrics.length, `no .fd-metric rendered at ${width}x${height}`).toBeGreaterThan(0)

            for (const metric of metrics) {
                const style = getComputedStyle(metric)
                // Tailwind's preflight resets every element to
                // `border-style: solid; border-width: 0`, so `borderStyle`
                // itself always reads "solid" regardless of whether this
                // rule declares a border -- `borderWidth` is the property
                // that actually says whether one is visible.
                expect(
                    style.borderWidth,
                    `a .fd-metric has a border (${style.borderWidth} ${style.borderStyle}) at ${width}x${height}`,
                ).toBe('0px')
                expect(
                    style.backgroundColor,
                    `a .fd-metric has a non-transparent background (${style.backgroundColor}) at ${width}x${height}`,
                ).toMatch(/^rgba\(0, 0, 0, 0\)$|^transparent$/)

                const value = metric.querySelector('.fd-metric__value')
                const caption = metric.querySelector('.fd-metric__caption')
                if (value === null || caption === null) {
                    throw new Error('.fd-metric__value or .fd-metric__caption did not render')
                }
                const captionText = (caption.textContent ?? '').toLowerCase()
                const valueText = (value.textContent ?? '').toLowerCase()
                expect(
                    valueText.endsWith(captionText) && captionText.length > 0,
                    `the figure "${value.textContent}" repeats its own caption "${caption.textContent}" at ${width}x${height}`,
                ).toBe(false)
            }
        },
    )

    it.each([[1024, 768], [640, 480]])(
        'gives the known-empty status plain text with no border or fill at %ix%i',
        async (width, height) => {
            await page.viewport(width, height)
            const knownEmpty: ProgressSnapshot = {
                bytesSent: 0, totalBytes: 0, totalKnown: true, percent: 0, speedBytesPerSec: 0,
            }
            const container = await renderTransferringInAppShell({progress: knownEmpty})

            const status = container.querySelector('.fd-empty-status')
            if (status === null) throw new Error('.fd-empty-status did not render')

            const style = getComputedStyle(status)
            // See the matching comment on the .fd-metric case above:
            // Tailwind's preflight makes `borderStyle` always read "solid",
            // so `borderWidth` is the property that says whether a border
            // is actually visible.
            expect(
                style.borderWidth,
                `.fd-empty-status has a border (${style.borderWidth} ${style.borderStyle}) at ${width}x${height}`,
            ).toBe('0px')
            expect(
                style.backgroundColor,
                `.fd-empty-status has a non-transparent background (${style.backgroundColor}) at ${width}x${height}`,
            ).toMatch(/^rgba\(0, 0, 0, 0\)$|^transparent$/)
        },
    )

    it.each([[1024, 768], [640, 480]])(
        'gives Cancel an intrinsic width well under half the details column at %ix%i',
        async (width, height) => {
            await page.viewport(width, height)
            const container = await renderTransferringInAppShell()

            const details = container.querySelector('.fd-hero__details')
            const cancel = screen.getByRole('button', {name: 'Cancel'})
            if (details === null) throw new Error('.fd-hero__details did not render')

            const detailsWidth = details.getBoundingClientRect().width
            const cancelWidth = cancel.getBoundingClientRect().width

            expect(
                cancelWidth,
                `Cancel is ${cancelWidth.toFixed(1)}px wide, not less than half the details column ` +
                    `(${detailsWidth.toFixed(1)}px) at ${width}x${height}`,
            ).toBeLessThan(detailsWidth / 2)
        },
    )
})

/*
  Story 9.6's own rendered-layout requirement: the outcome card (live,
  retained, or an Idle command failure) is one centred card, never wider than
  the Staged card's own column, with its contents centred and its two action
  buttons on one row -- and a long receipt name never overflows it. jsdom
  performs no layout, so `styles.test.ts` can only prove the stylesheet text
  contains a 720px cap; it cannot prove which element actually receives it,
  the same class of gap Story 9.4's own defect-fix section documents.
*/
describe('the outcome card is one centred, column-width card (Story 9.6)', () => {
    it.each([[1024, 768], [640, 480]])(
        'never renders wider than the Staged card\'s own column at %ix%i',
        async (width, height) => {
            await page.viewport(width, height)
            const stagedContainer = await renderStaged()
            const stagedRegion = stagedContainer.querySelector('.fd-region')
            if (stagedRegion === null) throw new Error('.fd-region did not render for Staged')
            const stagedWidth = stagedRegion.getBoundingClientRect().width
            cleanup()

            for (const renderCard of [renderRetainedDoneOutcome, renderLiveErrorOutcome, renderIdleCommandFailure]) {
                const container = await renderCard()
                const card = container.querySelector('.fd-outcome')
                if (card === null) throw new Error(`.fd-outcome did not render for ${renderCard.name}`)
                const cardWidth = card.getBoundingClientRect().width
                expect(
                    cardWidth,
                    `${renderCard.name}'s card (${cardWidth.toFixed(1)}px) is wider than Staged's own column ` +
                        `(${stagedWidth.toFixed(1)}px) at ${width}x${height}`,
                ).toBeLessThanOrEqual(stagedWidth + 0.5)
                cleanup()
            }
        },
    )

    it.each([[1024, 768], [640, 480]])(
        'centres the card horizontally in the window at %ix%i',
        async (width, height) => {
            await page.viewport(width, height)

            for (const renderCard of [renderRetainedDoneOutcome, renderLiveErrorOutcome, renderIdleCommandFailure]) {
                const container = await renderCard()
                const card = container.querySelector('.fd-outcome')
                if (card === null) throw new Error(`.fd-outcome did not render for ${renderCard.name}`)
                const rect = card.getBoundingClientRect()
                const leftGap = rect.left
                const rightGap = document.documentElement.clientWidth - rect.right
                expect(
                    Math.abs(leftGap - rightGap),
                    `${renderCard.name}'s card sits ${leftGap.toFixed(1)}px from the left and ` +
                        `${rightGap.toFixed(1)}px from the right at ${width}x${height} -- not centred`,
                ).toBeLessThanOrEqual(2)
                cleanup()
            }
        },
    )

    it.each([[1024, 768], [640, 480]])(
        'keeps the two pill buttons on one row at %ix%i',
        async (width, height) => {
            await page.viewport(width, height)

            await renderRetainedDoneOutcome()
            const sendAnother = screen.getByRole('button', {name: 'Send Another'})
            const done = screen.getByRole('button', {name: 'Done'})
            const doneRowDiff = Math.abs(
                sendAnother.getBoundingClientRect().top - done.getBoundingClientRect().top,
            )
            expect(
                doneRowDiff,
                `Send Another and Done are not on the same row at ${width}x${height} ` +
                    `(top difference ${doneRowDiff.toFixed(1)}px)`,
            ).toBeLessThanOrEqual(2)
            cleanup()

            await renderLiveErrorOutcome()
            const tryAgain = screen.getByRole('button', {name: 'Try Again'})
            const dismiss = screen.getByRole('button', {name: 'Dismiss'})
            const errorRowDiff = Math.abs(
                tryAgain.getBoundingClientRect().top - dismiss.getBoundingClientRect().top,
            )
            expect(
                errorRowDiff,
                `Try Again and Dismiss are not on the same row at ${width}x${height} ` +
                    `(top difference ${errorRowDiff.toFixed(1)}px)`,
            ).toBeLessThanOrEqual(2)
            cleanup()

            await renderIdleCommandFailure()
            const chooseAnother = screen.getByRole('button', {name: 'Choose Another'})
            const commandDismiss = screen.getByRole('button', {name: 'Dismiss'})
            const commandRowDiff = Math.abs(
                chooseAnother.getBoundingClientRect().top - commandDismiss.getBoundingClientRect().top,
            )
            expect(
                commandRowDiff,
                `Choose Another and Dismiss are not on the same row at ${width}x${height} ` +
                    `(top difference ${commandRowDiff.toFixed(1)}px)`,
            ).toBeLessThanOrEqual(2)
        },
    )

    /*
      Defect found by the orchestrator driving the built binary: the refresh
      glyph on "Try Again" renders flush against the "T" of the label -- no
      gap at all -- unlike "Send Another"/"Choose Another" (BrowseControl's
      own trailing chevron carries `margin-inline-start`, see style.css) and
      unlike the Copy Link glyph in StagedView, which sits inside
      `.fd-button__swap-face` and inherits that element's own `gap`.
      RefreshGlyph in OutcomePanel.tsx is a direct child of the button, not
      wrapped the same way, so it never picked up any spacing.

      Measured with a Range over the label's own text node, the same
      technique the browse-pill chevron test above uses, rather than the
      button's or glyph's bounding rect against the button -- both already
      include the button's padding and would not distinguish "spaced from the
      label" from "spaced from the button edge".

      Mutation: delete `.fd-button > .fd-button__glyph`'s `margin-inline-end`
      in style.css -> this fails, naming the near-zero gap.
    */
    it('keeps the refresh glyph clear of the "Try Again" label at 1024x768', async () => {
        await page.viewport(1024, 768)
        await renderLiveErrorOutcome()

        const tryAgain = screen.getByRole('button', {name: 'Try Again'})
        const glyph = tryAgain.querySelector<SVGElement>('.fd-button__glyph')
        if (glyph === null) throw new Error('.fd-button__glyph did not render inside Try Again')

        const textNode = [...tryAgain.childNodes].find((node) => node.nodeType === Node.TEXT_NODE)
        if (textNode === undefined) {
            throw new Error('expected the label\'s own text node as a direct child of Try Again')
        }
        const range = document.createRange()
        range.selectNodeContents(textNode)
        const textRect = range.getBoundingClientRect()
        const glyphRect = glyph.getBoundingClientRect()

        const gap = textRect.left - glyphRect.right
        expect(
            gap,
            `the glyph's right edge sits ${gap.toFixed(1)}px from the label text's own left edge ` +
                `(glyph right: ${glyphRect.right.toFixed(1)}px, text left: ${textRect.left.toFixed(1)}px) -- ` +
                'anything under 5px reads as touching the label',
        ).toBeGreaterThanOrEqual(5)
    })

    it.each([[1024, 768], [640, 480]])(
        'never lets a long receipt name push the pill past the card\'s own width at %ix%i',
        async (width, height) => {
            await page.viewport(width, height)
            const longName = 'a-very-long-descriptive-file-name-that-would-otherwise-overflow-the-pill.pdf'
            const container = await renderRetainedDoneOutcome(longName)

            const card = container.querySelector('.fd-outcome') as HTMLElement
            const receipt = container.querySelector('.fd-outcome__receipt') as HTMLElement
            if (receipt === null) throw new Error('.fd-outcome__receipt did not render')

            const cardRect = card.getBoundingClientRect()
            const receiptRect = receipt.getBoundingClientRect()
            expect(
                receiptRect.right,
                `the receipt (${receiptRect.right.toFixed(1)}px) extends past the card's own right edge ` +
                    `(${cardRect.right.toFixed(1)}px) at ${width}x${height}`,
            ).toBeLessThanOrEqual(cardRect.right + 0.5)
            expect(
                receiptRect.left,
                `the receipt (${receiptRect.left.toFixed(1)}px) extends past the card's own left edge ` +
                    `(${cardRect.left.toFixed(1)}px) at ${width}x${height}`,
            ).toBeGreaterThanOrEqual(cardRect.left - 0.5)
            assertNoHorizontalOverflow(container)
        },
    )
})

/*
  Story 9.6 review follow-up: the orchestrator's rendered check at 1024x768
  found the live terminal card growing to fill the whole window (a ~560px
  tall card with its content floating inside it, via the old
  `.fd-app > .fd-outcome[data-phase-view='outcome'] { flex: 1 1 auto;
  justify-content: center }` rule) and then, once `transfer-reset` made it
  retained, snapping to its natural ~250px height pinned to the top of the
  window -- the same node visibly collapsing and jumping the instant reset
  landed. The fix (style.css's `.fd-app > .fd-outcome, .fd-region >
  .fd-outcome:only-child { margin-block: auto }`) keeps the card at its own
  natural height in every form and centres it by consuming the *container's*
  free space instead of its own -- proved here by measuring across a real
  reset (test (c) below), not only by rendering each form in isolation.
*/
describe('the outcome card does not jump size or position at reset (Story 9.6 review follow-up)', () => {
    it.each([[1024, 768], [640, 480]])(
        'keeps the card at its own natural height rather than stretching to the window, at %ix%i',
        async (width, height) => {
            await page.viewport(width, height)

            for (const renderCard of [renderRetainedDoneOutcome, renderLiveErrorOutcome, renderIdleCommandFailure]) {
                await renderCard()
                const shortHeight = document.querySelector('.fd-outcome')!.getBoundingClientRect().height
                cleanup()

                // The same content, rendered again with a much taller window
                // available: a card that grows to fill its container would
                // measure hundreds of pixels taller here; a naturally-sized
                // one measures the same either way. This is the direct,
                // mutation-sensitive form of "the card's height matches its
                // content, not the window" -- the old flex: 1 1 auto rule
                // this replaces made the live form's height track the
                // window almost 1:1, which this comparison catches directly
                // rather than needing a second, separate "content height"
                // measurement to compare against.
                await page.viewport(width, 1400)
                await renderCard()
                const tallHeight = document.querySelector('.fd-outcome')!.getBoundingClientRect().height
                cleanup()
                await page.viewport(width, height)

                expect(
                    Math.abs(tallHeight - shortHeight),
                    `${renderCard.name}'s card measured ${shortHeight.toFixed(1)}px tall in a ${height}px ` +
                        `window and ${tallHeight.toFixed(1)}px tall in a 1400px window -- it is stretching ` +
                        'to fill the window rather than sizing to its own content',
                ).toBeLessThanOrEqual(40)
            }
        },
    )

    it.each([[1024, 768], [640, 480]])(
        'centres the card vertically -- top and bottom gaps to the window differ by <=2px, at %ix%i',
        async (width, height) => {
            await page.viewport(width, height)

            for (const renderCard of [renderRetainedDoneOutcome, renderLiveErrorOutcome, renderIdleCommandFailure]) {
                const container = await renderCard()
                const card = container.querySelector('.fd-outcome')
                if (card === null) throw new Error(`.fd-outcome did not render for ${renderCard.name}`)
                const rect = card.getBoundingClientRect()
                const topGap = rect.top
                const bottomGap = document.documentElement.clientHeight - rect.bottom
                expect(
                    Math.abs(topGap - bottomGap),
                    `${renderCard.name}'s card sits ${topGap.toFixed(1)}px from the top and ` +
                        `${bottomGap.toFixed(1)}px from the bottom at ${width}x${height} -- not centred`,
                ).toBeLessThanOrEqual(2)
                cleanup()
            }
        },
    )

    it('keeps the same node at the same rect (top, height) when a live outcome becomes retained', async () => {
        await page.viewport(1024, 768)
        const error = {code: 'transfer_failed', message: 'x'} as const

        const {container, rerender} = render(
            <div className="fd-app" style={{height: '100vh'}}>
                <OutcomePanel
                    outcome={{kind: 'error', retained: false, error, itemName: 'report.pdf'}}
                    level={1}
                    phaseView
                    dropTargetStyle={dropTargetStyle}
                    onDismiss={() => undefined}
                    onRetry={() => undefined}
                />
            </div>,
        )
        await waitForEntranceToSettle(container)
        const liveCard = container.querySelector('.fd-outcome')
        if (liveCard === null) throw new Error('.fd-outcome did not render (live)')
        const liveRect = liveCard.getBoundingClientRect()

        rerender(
            <div className="fd-app" style={{height: '100vh'}}>
                <OutcomePanel
                    outcome={{kind: 'error', retained: true, error, itemName: 'report.pdf'}}
                    dropTargetStyle={dropTargetStyle}
                    onDismiss={() => undefined}
                    onRetry={() => undefined}
                />
            </div>,
        )
        await waitForEntranceToSettle(container)
        const retainedCard = container.querySelector('.fd-outcome')

        // The identical DOM node -- App.focus.test.tsx's own retained-node
        // guarantee, re-proven here at the geometry level: reset must not
        // only preserve identity, it must preserve what the sender sees.
        expect(retainedCard, 'the same node, not a rebuilt one').toBe(liveCard)
        const retainedRect = retainedCard!.getBoundingClientRect()

        // *Mutation:* restore the old `.fd-app > .fd-outcome
        // [data-phase-view='outcome'] { flex: 1 1 auto; justify-content:
        // center }` rule -> this must fail, since the live form would again
        // grow to a materially different height/position than the retained
        // form measures at rest.
        expect(
            Math.abs(retainedRect.top - liveRect.top),
            `top moved from ${liveRect.top.toFixed(1)}px (live) to ${retainedRect.top.toFixed(1)}px (retained)`,
        ).toBeLessThanOrEqual(1)
        expect(
            Math.abs(retainedRect.height - liveRect.height),
            `height changed from ${liveRect.height.toFixed(1)}px (live) to ${retainedRect.height.toFixed(1)}px (retained)`,
        ).toBeLessThanOrEqual(1)
    })

    it('centres Staged vertically when it fits in the window', async () => {
        await page.viewport(1024, 900)
        const container = await renderStagedInAppShell()

        const region = container.querySelector('.fd-region')
        if (region === null) throw new Error('.fd-region did not render')
        const rect = region.getBoundingClientRect()
        const topGap = rect.top
        const bottomGap = document.documentElement.clientHeight - rect.bottom
        expect(
            Math.abs(topGap - bottomGap),
            `Staged sits ${topGap.toFixed(1)}px from the top and ${bottomGap.toFixed(1)}px from the ` +
                'bottom -- not centred',
        ).toBeLessThanOrEqual(2)
    })

    it('scrolls Staged from the top rather than clipping it when its content exceeds a 640x480 window', async () => {
        await page.viewport(640, 480)
        const container = await renderStagedInAppShell()

        const heading = screen.getByRole('heading', {name: 'Ready to send'})
        expect(
            heading.getBoundingClientRect().top,
            'the heading is clipped above the top of the viewport',
        ).toBeGreaterThanOrEqual(0)
        assertNoHorizontalOverflow(container)
    })
})
