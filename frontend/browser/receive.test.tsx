import type {} from '@vitest/browser-playwright' // pulls in the CDPSession#send() augmentation for the playwright provider
import type {CSSProperties} from 'react'
import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {cdp, page} from 'vitest/browser'
import {afterEach, describe, expect, it, vi} from 'vitest'
import type {
    IdleTransferState,
    PendingTransferState,
    ReceiveOutcomeTransferState,
    ReceiveReceivingTransferState,
    ReceiveWaitingTransferState,
} from '../src/transfer/state'
import type {ProgressSnapshot, ReceiveMetadata} from '../src/transfer/types'
import {IdleView} from '../src/ui/IdleView'
import {ReceiveOutcomeView} from '../src/ui/ReceiveOutcomeView'
import {ReceiveWaitingView} from '../src/ui/ReceiveWaitingView'
import {ReceivingView} from '../src/ui/ReceivingView'
import {StagePendingCard} from '../src/ui/StagePendingCard'
import '../src/style.css'

/*
  Rendered evidence for the receive views (spec-phone-to-desktop-receiving,
  Story B). The jsdom suites prove what each view says and does; this file
  proves what only a real layout engine can: that the receive views reflow to
  320 CSS pixels with only vertical scroll, that every control keeps the 44px
  floor, that 200% text and the WCAG text-spacing overrides do not clip them,
  that forced colors restore a real outline on the Receive control, and that
  the receive QR image cannot start a drag (AGENTS.md, WebKit fact 6).

  It does not prove WKWebView behaviour. Chromium focuses a clicked button and
  matches :focus-visible after a scripted focus; WebKit does neither. What this
  file can pin about focus is the *mechanism* (the marker attribute and the
  outline rule that keys off it), not the platform behaviour, which the owner
  confirms on the built binary.
*/

vi.mock('../wailsjs/go/main/App', () => ({CopyToClipboard: vi.fn().mockResolvedValue(undefined), ListConnectionOptions: vi.fn()}))

const sessionId = '0123456789abcdef0123456789abcdef'
const uploadURL = `http://192.0.2.1:34123/upload/${'fedcba9876543210fedcba9876543210'}`

/** A synthetic finder-pattern bitmap; it encodes nothing, as in accessibility.test.tsx. */
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
    const dataURL = canvas.toDataURL('image/png')
    return dataURL.slice(dataURL.indexOf(',') + 1)
}

const qrBase64 = syntheticQrBase64()

function receiveMetadata(overrides: Partial<ReceiveMetadata> = {}): ReceiveMetadata {
    return {sessionId, destination: 'Phone Photos', url: uploadURL, qrBase64, warnings: [], ...overrides}
}

function waiting(overrides: Partial<ReceiveWaitingTransferState> = {}): ReceiveWaitingTransferState {
    return {
        phase: 'receive-waiting',
        session: {sessionId, lastSeq: 0},
        metadata: receiveMetadata(),
        cancelPending: false,
        commandError: null,
        refusals: 0,
        ...overrides,
    }
}

function receiving(overrides: Partial<ReceiveReceivingTransferState> = {}): ReceiveReceivingTransferState {
    const progress: ProgressSnapshot = {
        bytesSent: 41_000_000,
        totalBytes: 120_000_000,
        totalKnown: true,
        percent: 34,
        speedBytesPerSec: 5_100_000,
    }
    return {
        phase: 'receive-receiving',
        session: {sessionId, lastSeq: 1},
        metadata: receiveMetadata(),
        progress,
        filesSaved: 3,
        cancelPending: false,
        commandError: null,
        ...overrides,
    }
}

function outcome(
    kind: 'complete' | 'incomplete' | 'cancelled',
    overrides: Partial<ReceiveOutcomeTransferState['outcome']> = {},
): ReceiveOutcomeTransferState {
    return {
        phase: 'receive-outcome',
        session: {sessionId, lastSeq: 4},
        destination: 'Phone Photos',
        outcome: {kind, filesSaved: 3, subfolderExists: true, markingWarning: false, ...overrides},
        commandError: null,
    }
}

const dropTargetStyle = {'--wails-drop-target': 'drop'} as CSSProperties

function idle(): IdleTransferState {
    return {phase: 'idle', retainedOutcome: null, commandError: null}
}

async function waitForEntranceToSettle(element: Element): Promise<void> {
    await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()))
    await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()))
    await Promise.all(element.getAnimations({subtree: true}).map((animation) => animation.finished))
}

/** Each view inside the `.fd-app` stand-in the real shell provides, so centring has a definite height to resolve against. */
async function mount(view: React.ReactElement): Promise<HTMLElement> {
    const {container} = render(<div className="fd-app" style={{height: '100vh'}}>{view}</div>)
    await waitForEntranceToSettle(container)
    return container
}

const scenes: ReadonlyArray<readonly [string, () => React.ReactElement]> = [
    ['Idle with the Receive entry', () => (
        <IdleView
            state={idle()}
            dropTargetStyle={dropTargetStyle}
            cancelWon={false}
            onSelectFile={() => undefined}
            onSelectDirectory={() => undefined}
            onReceive={() => Promise.resolve('proceeded')}
            commandErrorPanelProps={{}}
        />
    )],
    ['receive start pending', () => (
        <StagePendingCard
            state={{phase: 'pending', generation: 1, itemKind: 'receive', cancelPending: false} satisfies PendingTransferState}
            onCancel={() => undefined}
        />
    )],
    ['receive waiting', () => <ReceiveWaitingView state={waiting()} onCancel={() => undefined}/>],
    ['receive waiting after a refused upload', () => (
        <ReceiveWaitingView state={waiting({refusals: 1})} onCancel={() => undefined}/>
    )],
    ['receiving', () => <ReceivingView state={receiving()} onCancel={() => undefined}/>],
    ['complete outcome', () => (
        <ReceiveOutcomeView state={outcome('complete')} onShowInFolder={() => undefined} onDone={() => undefined}/>
    )],
    ['incomplete outcome with a marking warning', () => (
        <ReceiveOutcomeView
            state={outcome('incomplete', {filesSaved: 2, markingWarning: true})}
            onShowInFolder={() => undefined}
            onDone={() => undefined}
        />
    )],
    ['cancelled outcome with nothing saved', () => (
        <ReceiveOutcomeView
            state={outcome('cancelled', {filesSaved: 0, subfolderExists: false})}
            onShowInFolder={() => undefined}
            onDone={() => undefined}
        />
    )],
]

const textSpacingStyleId = 'wcag-1-4-12-overrides-receive'

afterEach(async () => {
    cleanup()
    document.documentElement.style.cssText = ''
    document.getElementById(textSpacingStyleId)?.remove()
    await page.viewport(1024, 800)
    await setForcedColorsActive(false)
})

async function setForcedColorsActive(active: boolean): Promise<void> {
    await cdp().send('Emulation.setEmulatedMedia', {
        features: active ? [{name: 'forced-colors', value: 'active'}] : [],
    })
}

function describeElement(element: Element): string {
    const tag = element.tagName.toLowerCase()
    const classes = element.classList.length > 0 ? `.${[...element.classList].join('.')}` : ''
    const label = element.getAttribute('aria-label') ?? element.getAttribute('alt') ??
        (element.textContent ?? '').trim().slice(0, 40)
    return label === '' ? `${tag}${classes}` : `${tag}${classes} "${label}"`
}

function assertNoHorizontalOverflow(container: HTMLElement, sceneName: string): void {
    const root = document.documentElement
    expect(
        root.scrollWidth,
        `${sceneName}: documentElement.scrollWidth (${root.scrollWidth}px) exceeds clientWidth (${root.clientWidth}px)`,
    ).toBeLessThanOrEqual(root.clientWidth)

    for (const element of container.querySelectorAll<HTMLElement>('*')) {
        const rect = element.getBoundingClientRect()
        if (rect.width === 0 && rect.height === 0) continue
        expect(
            rect.right,
            `${sceneName}: ${describeElement(element)} extends to ${rect.right.toFixed(1)}px, past the ${root.clientWidth}px viewport`,
        ).toBeLessThanOrEqual(root.clientWidth + 0.5)
    }
}

function assertNoFixedHeightClipsGrownText(container: HTMLElement, sceneName: string): void {
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
            `${sceneName}: ${describeElement(element)} clips its content: scrollHeight (${element.scrollHeight}px) ` +
                `exceeds clientHeight (${element.clientHeight}px)`,
        ).toBeLessThanOrEqual(element.clientHeight + 0.5)
    }
}

function assertEveryTargetMeetsTheFloor(container: HTMLElement, sceneName: string): void {
    const targets = container.querySelectorAll<HTMLElement>('.fd-target')
    expect(targets.length, `${sceneName}: no .fd-target controls were found to measure`).toBeGreaterThan(0)
    for (const target of targets) {
        const rect = target.getBoundingClientRect()
        // A collapsed link field legitimately measures 0x0; it is not an activation target until revealed.
        if (target.closest('.fd-url-reveal:not([data-open])') !== null) continue
        expect(rect.width, `${sceneName}: ${describeElement(target)} is ${rect.width.toFixed(1)}px wide`).toBeGreaterThanOrEqual(44)
        expect(rect.height, `${sceneName}: ${describeElement(target)} is ${rect.height.toFixed(1)}px tall`).toBeGreaterThanOrEqual(44)
    }
}

describe('the receive views at the desktop window size', () => {
    it.each(scenes)('%s fits 1024x768 with no horizontal scroll and every target at 44px', async (name, view) => {
        await page.viewport(1024, 768)
        const container = await mount(view())
        assertNoHorizontalOverflow(container, name)
        assertEveryTargetMeetsTheFloor(container, name)
    })

    it.each(scenes)('%s fits the 640x480 minimum window without clipping', async (name, view) => {
        await page.viewport(640, 480)
        const container = await mount(view())
        assertNoHorizontalOverflow(container, name)
        assertNoFixedHeightClipsGrownText(container, name)
    })
})

describe('the receive views at 320 CSS pixels', () => {
    it.each(scenes)('%s scrolls only vertically with nothing clipped', async (name, view) => {
        await page.viewport(320, 900)
        const container = await mount(view())
        assertNoHorizontalOverflow(container, name)
        assertNoFixedHeightClipsGrownText(container, name)
        assertEveryTargetMeetsTheFloor(container, name)
    })

    it('keeps a long folder name inside the card instead of widening it', async () => {
        await page.viewport(320, 900)
        const long = 'A very long destination folder name that has no business being this long and keeps going'
        const container = await mount(
            <ReceiveWaitingView state={waiting({metadata: receiveMetadata({destination: long})})} onCancel={() => undefined}/>,
        )
        assertNoHorizontalOverflow(container, 'receive waiting with a long folder name')
    })
})

describe('the receive views under 200% text and the WCAG text-spacing overrides', () => {
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

    function applyTextSpacingOverrides(): void {
        const sheet = document.createElement('style')
        sheet.id = textSpacingStyleId
        sheet.textContent = `
            * { line-height: 1.5 !important; letter-spacing: 0.12em !important; word-spacing: 0.16em !important; }
            p { margin-block-end: 2em !important; }
        `
        document.head.append(sheet)
    }

    it.each(scenes)('%s grows without horizontal scroll or a clipped container', async (name, view) => {
        await page.viewport(1024, 900)
        doubleTextTokens()
        applyTextSpacingOverrides()
        const container = await mount(view())
        assertNoHorizontalOverflow(container, name)
        assertNoFixedHeightClipsGrownText(container, name)
    })
})

describe('forced colors on the receive views', () => {
    it('keeps the forced-color-adjust exemption on the receive QR substrate and nowhere else', async () => {
        await page.viewport(1024, 900)
        await setForcedColorsActive(true)
        const container = await mount(<ReceiveWaitingView state={waiting()} onCancel={() => undefined}/>)

        for (const element of container.querySelectorAll<HTMLElement>('*')) {
            const adjust = getComputedStyle(element).getPropertyValue('forced-color-adjust')
            const exempt = element.classList.contains('fd-qr-panel') || element.classList.contains('fd-qr')
            if (exempt) expect(adjust, describeElement(element)).toBe('none')
            else expect(adjust, `${describeElement(element)} computes forced-color-adjust: none outside the QR substrate`).not.toBe('none')
        }
    })

    it('draws a real outline, not a box-shadow, on the Receive control after a scripted focus return', async () => {
        await page.viewport(1024, 900)
        await setForcedColorsActive(true)
        await mount(
            <IdleView
                state={idle()}
                dropTargetStyle={dropTargetStyle}
                cancelWon={false}
                onSelectFile={() => undefined}
                onSelectDirectory={() => undefined}
                onReceive={() => Promise.resolve('dismissed')}
                commandErrorPanelProps={{}}
            />,
        )
        const receive = screen.getByRole('button', {name: 'Receive'})

        // A keyboard activation (detail 0) whose chooser is dismissed: focus is returned by
        // script, and the control carries the marker that stands in for :focus-visible, which
        // WebKit never matches after a scripted focus.
        fireEvent.click(receive, {detail: 0})
        await vi.waitFor(() => expect(receive.hasAttribute('data-focus-return')).toBe(true))

        // Chromium DOES match :focus-visible after that scripted focus, so the live control cannot
        // isolate the marker rule. An unfocused probe carrying only the marker can: nothing but
        // `.fd-button[data-focus-return]` can draw its outline.
        const probe = document.createElement('button')
        probe.className = 'fd-button fd-button--secondary fd-target'
        probe.setAttribute('data-focus-return', '')
        probe.textContent = 'probe'
        document.querySelector('.fd-receive-entry')!.append(probe)
        expect(document.activeElement, 'the probe must not be focused').not.toBe(probe)

        for (const [label, element] of [['the Receive control', receive], ['a marker-only probe', probe]] as const) {
            const style = getComputedStyle(element)
            expect(style.outlineStyle, `${label} draws no outline under forced colors`).toBe('solid')
            expect(Number.parseFloat(style.outlineWidth), `${label}'s outline has no width`).toBeGreaterThan(0)
            expect(style.boxShadow, `${label}: forced colors strips decorative box-shadow, so the ring cannot depend on it`).toBe('none')
        }
    })
})

describe('the receive QR image cannot start a drag (AGENTS.md, WebKit fact 6)', () => {
    it('marks the receive QR non-draggable both ways', async () => {
        const container = await mount(<ReceiveWaitingView state={waiting()} onCancel={() => undefined}/>)
        const qr = container.querySelector<HTMLImageElement>('.fd-qr')
        if (qr === null) throw new Error('.fd-qr did not render')

        expect(qr.draggable, 'the receive QR <img> is draggable -- the Wails drop handler crashes on a non-file NSURL').toBe(false)
        expect(
            getComputedStyle(qr).getPropertyValue('-webkit-user-drag').trim(),
            'the receive QR <img> computes -webkit-user-drag other than none',
        ).toBe('none')
    })
})

describe('the held outcome does not jump when it appears', () => {
    it('centres the outcome card in the window rather than pinning it to the top', async () => {
        await page.viewport(1024, 768)
        const container = await mount(
            <ReceiveOutcomeView state={outcome('complete')} onShowInFolder={() => undefined} onDone={() => undefined}/>,
        )
        const card = container.querySelector<HTMLElement>('.fd-outcome')
        if (card === null) throw new Error('.fd-outcome did not render')
        const rect = card.getBoundingClientRect()
        const above = rect.top
        const below = window.innerHeight - rect.bottom
        expect(Math.abs(above - below), `the card sits ${above.toFixed(0)}px from the top and ${below.toFixed(0)}px from the bottom`)
            .toBeLessThan(40)
    })
})
