import {act, cleanup, fireEvent, render, screen} from '@testing-library/react'
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'

const mocks = vi.hoisted(() => ({copyToClipboard: vi.fn()}))

vi.mock('../../wailsjs/go/main/App', () => ({
    CopyToClipboard: mocks.copyToClipboard,
}))
import type {StagedTransferState} from '../transfer/state'
import type {FileMetadata, PublicError} from '../transfer/types'
import {copy} from './copy'
import {StagedView} from './StagedView'

afterEach(cleanup)

const sessionId = '0123456789abcdef0123456789abcdef'
const token = 'fedcba9876543210fedcba9876543210'
const capabilityURL = `http://192.0.2.1:34123/download/${token}`
const qrPNG = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII='

function metadata(overrides: Partial<FileMetadata> = {}): FileMetadata {
    return {
        sessionId,
        name: 'Travel Notes.pdf',
        size: 8_400_000,
        isDir: false,
        url: capabilityURL,
        qrBase64: qrPNG,
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

// The bound Go command, not navigator.clipboard: the browser API is undefined
// on macOS, where this frontend runs from a non-secure custom scheme.
const writeText = mocks.copyToClipboard

beforeEach(() => {
    writeText.mockReset()
    writeText.mockResolvedValue(undefined)
})

describe('the staged handoff', () => {
    it('leads with the staged heading and the QR instruction', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        expect(screen.getByRole('heading', {level: 1, name: 'Ready to send'})).toBeTruthy()
        expect(screen.getByText('Scan the code with the receiving device’s camera.')).toBeTruthy()
    })

    it('renders the QR from the bare base64, prefixing the data URL only at render', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        const image = screen.getByRole('img') as HTMLImageElement
        expect(image.getAttribute('src')).toBe(`data:image/png;base64,${qrPNG}`)
        expect(image.getAttribute('alt')).toBe('Download QR code for Travel Notes.pdf')
    })

    /*
      Story 9.4: the QR tile gets its own fade-plus-scale entrance
      (`.fd-qr-panel` in style.css), not the generic staggered `.fd-rise` the
      item row/actions/caveats use -- it is the view's focal object, entering
      with the view itself rather than queued a step behind it.
    */
    it('does not stagger the QR tile behind the view -- it carries no fd-rise class', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        expect(document.querySelector('.fd-qr-panel')?.className).not.toContain('fd-rise')
    })

    it('isolates the full sanitized name and states its logical size', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        const isolate = document.querySelector('bdi')
        expect(isolate?.getAttribute('dir')).toBe('auto')
        expect(isolate?.textContent).toBe('Travel Notes.pdf')
        expect(screen.getByText('File · 8.4 MB')).toBeTruthy()
    })

    it('renders a right-to-left name inside its own isolate without truncating it', () => {
        const name = 'تقرير ٢٠٢٦.pdf'
        render(<StagedView state={staged({metadata: metadata({name})})} onCancel={vi.fn()}/>)

        expect(document.querySelector('bdi')?.textContent).toBe(name)
        expect(screen.getByRole('img').getAttribute('alt')).toBe(`Download QR code for ${name}`)
    })

    it('says a folder downloads as a ZIP, as the third caveat line, and labels its size as logical', () => {
        const state = staged({metadata: metadata({name: "Dad's PDFs", isDir: true, size: 36_800_000})})
        render(<StagedView state={state} onCancel={vi.fn()}/>)

        const note = screen.getByText('This folder downloads as a ZIP.')
        expect(note).toBeTruthy()
        expect(note.closest('.fd-caveats')).toBeTruthy()
        expect(screen.getByText('Folder · 36.8 MB logical size')).toBeTruthy()
    })

    it('omits the folder note for a file', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        expect(screen.queryByText('This folder downloads as a ZIP.')).toBeNull()
    })

    it('gives the item row a kind glyph, decorative and unlabelled', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        const icon = document.querySelector('.fd-item__icon')
        expect(icon?.getAttribute('aria-hidden')).toBe('true')
        expect(icon?.querySelector('svg')).toBeTruthy()
    })
})

/*
  Story 9.4 removed the two-line name clamp and its persistent "Show full
  name" toggle: the name always wraps instead
  (`.fd-headline`'s `overflow-wrap: anywhere`, unaffected by this story).
  These replace the old "full-value access to a long or bidi name" describe
  block, whose clamp/toggle tests no longer have a feature to exercise.
*/
describe('long or bidi names always wrap, never clip behind a clamp', () => {
    const unbroken = 'Q1-report-' + 'x'.repeat(180) + '.pdf'

    it('never clamps a long name, and offers no toggle to reveal one', () => {
        render(<StagedView state={staged({metadata: metadata({name: unbroken})})} onCancel={vi.fn()}/>)

        const isolate = document.querySelector('bdi') as HTMLElement
        // Mutation: reintroducing a clamp on the headline without a toggle
        // beside it must fail here, naming the stray class.
        expect(isolate.closest('.fd-headline')?.className).not.toContain('fd-clamp')
        expect(isolate.textContent).toBe(unbroken)
        expect(screen.queryByRole('button', {name: 'Show full name'})).toBeNull()
    })

    it('isolates a mixed-direction name exactly once -- no second, assistive-only copy', () => {
        const mixed = 'تقرير ٢٠٢٦ ‮report‬.pdf'
        render(<StagedView state={staged({metadata: metadata({name: mixed})})} onCancel={vi.fn()}/>)

        // The old clamp shipped a second, visually hidden <bdi> carrying the
        // full value for the toggle's aria-describedby. With no clamp there
        // is nothing left to describe, so exactly one isolate remains.
        const isolates = [...document.querySelectorAll('bdi')]
        expect(isolates).toHaveLength(1)
        expect(isolates[0].getAttribute('dir')).toBe('auto')
        expect(isolates[0].textContent).toBe(mixed)
        expect(screen.getByRole('img').getAttribute('alt')).toBe(`Download QR code for ${mixed}`)
    })
})

describe('the direct URL row', () => {
    it('does not render the link until Show Link is activated', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        const trigger = screen.getByRole('button', {name: 'Show Link'})
        expect(trigger.getAttribute('aria-expanded')).toBe('false')
        const region = document.querySelector('.fd-url-reveal')
        expect(region?.hasAttribute('data-open')).toBe(false)
        expect(screen.queryByRole('button', {name: 'Hide Link'})).toBeNull()
    })

    it('reveals the field on Show Link, and toggles the trigger to Hide Link', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        fireEvent.click(screen.getByRole('button', {name: 'Show Link'}))

        const trigger = screen.getByRole('button', {name: 'Hide Link'})
        expect(trigger.getAttribute('aria-expanded')).toBe('true')
        expect(document.querySelector('.fd-url-reveal')?.getAttribute('data-open')).toBe('true')
        expect(trigger.getAttribute('aria-controls')).toBe(document.querySelector('.fd-url-reveal')?.id)

        fireEvent.click(screen.getByRole('button', {name: 'Hide Link'}))
        expect(screen.getByRole('button', {name: 'Show Link'}).getAttribute('aria-expanded')).toBe('false')
        expect(document.querySelector('.fd-url-reveal')?.hasAttribute('data-open')).toBe(false)
    })

    it('is a readonly form control rather than a sender-side activation link, once revealed', () => {
        const {container} = render(<StagedView state={staged()} onCancel={vi.fn()}/>)
        fireEvent.click(screen.getByRole('button', {name: 'Show Link'}))

        // A real <input readonly>, not a div wearing role="textbox": assistive
        // technology reads the value of one reliably and disagrees about the
        // other. It is still not a link.
        const field = screen.getByRole('textbox') as HTMLTextAreaElement
        // A textarea rather than an input: the value has to wrap, because at
        // 320px under 200% text a single-line field shows a fraction of the
        // capability URL and that URL is the fallback when the QR cannot be
        // scanned. What matters to this assertion is that it is a readonly
        // form control and not an anchor.
        expect(field.tagName).toBe('TEXTAREA')
        expect(field.value).toBe(capabilityURL)
        expect(field.readOnly).toBe(true)
        expect(field.className).toContain('fd-target')
        expect(field.getAttribute('aria-label')).toBe(copy.label.directLinkHeading)
        expect(container.querySelectorAll('a')).toHaveLength(0)
    })

    it('exposes the capability token as readable content exactly once, and never as prose or a link target', () => {
        const {container} = render(<StagedView state={staged()} onCancel={vi.fn()}/>)
        fireEvent.click(screen.getByRole('button', {name: 'Show Link'}))

        expect((screen.getByRole('textbox') as HTMLInputElement).value).toContain(token)
        // The QR carries the same URL as an image, not as readable text.
        expect(screen.getByRole('img').getAttribute('src')).not.toContain(token)
        expect(container.querySelectorAll('a')).toHaveLength(0)

        /*
          Story 7.9: the field's height comes from a CSS-only grid + hidden-
          mirror technique (`.fd-url-wrap`/`.fd-url-mirror` in style.css),
          replacing the old JavaScript ResizeObserver. The mirror is a real
          sibling element carrying the same URL as its own text content --
          not a `::after` reading it back from a `data-*` attribute, because a
          reader's WCAG 1.4.12 text-spacing override (a bare `* { ... }` rule)
          does not reach generated pseudo-element content and would desync the
          mirror from the textarea (see the comment beside `.fd-url-mirror` in
          style.css). So the token now appears in `innerHTML` a second time, as
          `.fd-url-mirror`'s text content -- but not as a second *readable*
          occurrence: that element is `visibility: hidden` (removed from the
          accessibility tree, unlike `opacity: 0` or off-screen positioning, so
          it is never read aloud) and carries `aria-hidden="true"` besides,
          never a link, and never presented as prose. It exists purely to size
          the grid cell the real, single readable field sits in.
        */
        const serialized = container.innerHTML
        const occurrences = serialized.split(token).length - 1
        expect(occurrences).toBe(2)
        const mirror = container.querySelector('.fd-url-mirror')
        expect(mirror).not.toBeNull()
        expect(mirror?.getAttribute('aria-hidden')).toBe('true')
        expect(mirror?.textContent).toBe(`${capabilityURL} `)
    })

    it('renders every warning even when two arrive under the same code', () => {
        const warning = {
            code: 'beacon_warning' as const,
            message: 'Device discovery isn’t available. The QR code and download link still work.',
        }
        // React renders both nodes even with a duplicate key, so a count proves
        // nothing; the key collision surfaces as a console error instead.
        const reported: unknown[] = []
        const consoleError = vi.spyOn(console, 'error').mockImplementation((...args) => {
            reported.push(args[0])
        })

        try {
            render(<StagedView state={staged({metadata: metadata({warnings: [warning, warning]})})} onCancel={vi.fn()}/>)
        } finally {
            consoleError.mockRestore()
        }

        expect(document.querySelectorAll('[data-warning-code="beacon_warning"]').length).toBe(2)
        expect(reported.filter((entry) => String(entry).includes('same key'))).toEqual([])
    })

    it('selects the whole capability URL on focus, as the manual fallback, once revealed', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)
        fireEvent.click(screen.getByRole('button', {name: 'Show Link'}))
        const field = screen.getByRole('textbox') as HTMLInputElement
        const select = vi.spyOn(field, 'select')

        fireEvent.focus(field)

        // The <div> this replaced carried `user-select: all`, so one click took
        // the whole URL. An input has no such behaviour, and this is the path a
        // user needs when the clipboard command fails.
        expect(select).toHaveBeenCalledTimes(1)
    })

    it('keeps the click that focuses the URL from collapsing its selection, once revealed', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)
        fireEvent.click(screen.getByRole('button', {name: 'Show Link'}))
        const field = screen.getByRole('textbox') as HTMLTextAreaElement

        const prevented = !fireEvent.mouseDown(field)

        // select-on-focus alone is defeated by the mouseup that follows: the
        // selection collapses to a caret and the manual fallback is a call
        // that happened and a selection nobody got.
        expect(prevented).toBe(true)
        expect(document.activeElement).toBe(field)
    })

    it('blurs the revealed field on Escape rather than leaving it ringed', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)
        fireEvent.click(screen.getByRole('button', {name: 'Show Link'}))
        const field = screen.getByRole('textbox') as HTMLTextAreaElement
        field.focus()
        expect(document.activeElement).toBe(field)

        fireEvent.keyDown(field, {key: 'Escape'})

        expect(document.activeElement).not.toBe(field)
    })
})

describe('Copy Link copies without revealing the link', () => {
    it('leaves the link collapsed and the trigger labelled Show Link after a successful copy', async () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        await act(async () => {
            pressCopy()
        })

        expect(writeText).toHaveBeenCalledWith(capabilityURL)
        expect(screen.getByRole('button', {name: 'Show Link'})).toBeTruthy()
        expect(document.querySelector('.fd-url-reveal')?.hasAttribute('data-open')).toBe(false)
    })

    it('reveals independently of Copy Link -- toggling one never toggles the other', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        fireEvent.click(screen.getByRole('button', {name: 'Show Link'}))
        expect(screen.getByRole('button', {name: 'Hide Link'})).toBeTruthy()

        fireEvent.click(screen.getByRole('button', {name: 'Copy Link'}))
        // Still revealed: Copy Link does not close it either.
        expect(screen.getByRole('button', {name: 'Hide Link'})).toBeTruthy()
    })
})

describe('copy feedback', () => {
    it('confirms only after the clipboard write resolves', async () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        await act(async () => {
            pressCopy()
        })

        expect(writeText).toHaveBeenCalledWith(capabilityURL)
        expect(screen.getByRole('button', {name: 'Copied'})).toBeTruthy()
        expect(screen.queryByRole('button', {name: 'Copy Link'})).toBeNull()
    })

    /*
      Story 9.4: the swap is a crossfade, not a jump cut -- both faces are
      always mounted in the same grid cell (`.fd-button__swap`), and exactly
      one carries `aria-hidden="true"` at a time. This is what a screen
      reader's accessible-name computation relies on: without it, both
      "Copy Link" and "Copied" would read as one concatenated name.
    */
    it('keeps both label faces mounted for the crossfade, with exactly one aria-hidden at a time', async () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)
        const swap = () => document.querySelector('.fd-button__swap') as HTMLElement

        const facesBefore = swap().querySelectorAll('.fd-button__swap-face')
        expect(facesBefore).toHaveLength(2)
        expect([...facesBefore].filter((face) => face.getAttribute('aria-hidden') === 'true')).toHaveLength(1)

        await act(async () => {
            pressCopy()
        })

        const facesAfter = swap().querySelectorAll('.fd-button__swap-face')
        expect(facesAfter).toHaveLength(2)
        expect([...facesAfter].filter((face) => face.getAttribute('aria-hidden') === 'true')).toHaveLength(1)
        expect(screen.getByRole('button', {name: 'Copied'})).toBeTruthy()
    })

    it('leaves the action label alone when the clipboard write rejects', async () => {
        writeText.mockRejectedValue(new Error('denied'))
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        await act(async () => {
            pressCopy()
        })

        expect(screen.getByRole('button', {name: 'Copy Link'})).toBeTruthy()
        expect(screen.queryByRole('button', {name: 'Copied'})).toBeNull()
    })

    /*
      The copy must not go through navigator.clipboard. WKWebView serves this
      frontend from the custom wails:// scheme, which is not a secure context,
      so that API is undefined on macOS and the action would silently do
      nothing there -- on one of the two platforms V1 supports.
    */
    it('copies through the bound command and never through the browser API', async () => {
        const browserWrite = vi.fn()
        Object.defineProperty(navigator, 'clipboard', {
            value: {writeText: browserWrite}, configurable: true, writable: true,
        })
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        await act(async () => {
            pressCopy()
        })

        expect(writeText).toHaveBeenCalledTimes(1)
        expect(browserWrite).not.toHaveBeenCalled()
    })

    it('changes nothing but the label: no toast, no lifecycle move, no cleared clipboard', async () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        await act(async () => {
            pressCopy()
        })

        expect(writeText).toHaveBeenCalledTimes(1)
        expect(screen.getByRole('heading', {level: 1}).textContent).toBe('Ready to send')
        expect(document.querySelector('[role="alert"]')).toBeNull()
    })

    /*
      A rejection the user can act on, rather than one only the console sees.

      Story 3.11 gave the clipboard write its own registry code, message and
      heading. Nothing consumed them: this view discarded the rejection, so an
      OS that refused the clipboard produced exactly the same screen as one
      that accepted it -- the shipped path for `clipboard_failed` ended in a
      `() => undefined`. Found by the Blind Hunter layer re-run (D-109).
    */
    it('reports a rejected clipboard write with the session that issued it', async () => {
        const onCopyFailed = vi.fn()
        writeText.mockRejectedValue(new Error('denied'))
        render(<StagedView state={staged()} onCancel={vi.fn()} onCopyFailed={onCopyFailed}/>)

        await act(async () => {
            pressCopy()
        })

        expect(onCopyFailed).toHaveBeenCalledTimes(1)
        expect(onCopyFailed).toHaveBeenCalledWith(sessionId)
    })

    it('does not report a clipboard write that resolved', async () => {
        const onCopyFailed = vi.fn()
        render(<StagedView state={staged()} onCancel={vi.fn()} onCopyFailed={onCopyFailed}/>)

        await act(async () => {
            pressCopy()
        })

        expect(onCopyFailed).not.toHaveBeenCalled()
    })

    /*
      The confirmation describes the last attempt, not the best one.

      `Copied` was set on success and never cleared, so a second copy that the
      OS refused left a success label standing over a failure notice -- the one
      thing this control's own contract ("a write that never happened is still
      never reported as one") promises it will not do.
    */
    it('clears an earlier confirmation when a later write rejects', async () => {
        const onCopyFailed = vi.fn()
        render(<StagedView state={staged()} onCancel={vi.fn()} onCopyFailed={onCopyFailed}/>)

        await act(async () => {
            pressCopy()
        })
        expect(screen.getByRole('button', {name: 'Copied'})).toBeTruthy()

        writeText.mockRejectedValue(new Error('denied'))
        await act(async () => {
            fireEvent.click(screen.getByRole('button', {name: 'Copied'}))
        })

        expect(screen.getByRole('button', {name: 'Copy Link'})).toBeTruthy()
        expect(screen.queryByRole('button', {name: 'Copied'})).toBeNull()
        expect(onCopyFailed).toHaveBeenCalledWith(sessionId)
    })

    /*
      D-114: a successful copy used to rename this control to "Copied" for the
      rest of the session, with no way back -- the one control that reaches
      the capability URL lost the name that says what it does. Blur is the
      sanctioned trigger: it fires only from the sender's own focus move, never
      a timer EXPERIENCE.md forbids, so by the time the sender "returns to the
      control" (the acceptance criterion's words) it already names the action
      again.
    */
    it('reverts to the action label once focus leaves the control (D-114)', async () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        await act(async () => {
            pressCopy()
        })
        const button = screen.getByRole('button', {name: 'Copied'})

        fireEvent.blur(button)

        expect(screen.getByRole('button', {name: 'Copy Link'})).toBeTruthy()
        expect(screen.queryByRole('button', {name: 'Copied'})).toBeNull()
    })

    it('still names the action when the sender returns to the control later', async () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        const button = () => screen.getByRole('button', {name: /^(Copy Link|Copied)$/})
        await act(async () => {
            // Focus first: a real press focuses before it clicks, and the
            // confirmation is only claimed while the control holds focus.
            button().focus()
            fireEvent.click(button())
        })
        expect(button().textContent).toContain('Copied')

        // The sender moves on -- Tab reaches Cancel, say -- and later returns.
        fireEvent.blur(button(), {relatedTarget: screen.getByRole('button', {name: 'Cancel'})})
        fireEvent.focus(button())

        expect(button().textContent).toContain('Copy Link')
        expect(screen.getByRole('button', {name: 'Copy Link'})).toBe(button())
    })

    it('does not revert while the same activation is still retrying (a click on "Copied")', async () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        await act(async () => {
            pressCopy()
        })
        expect(screen.getByRole('button', {name: 'Copied'})).toBeTruthy()

        // A second click while still focused re-copies rather than losing the
        // confirmation to a spurious revert -- no blur happened.
        await act(async () => {
            fireEvent.click(screen.getByRole('button', {name: 'Copied'}))
        })

        expect(screen.getByRole('button', {name: 'Copied'})).toBeTruthy()
    })
})

describe('trust disclosures', () => {
    it('states the first-opener limit and the unencrypted network, always visible, each with its own glyph', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        const caveats = document.querySelector('.fd-caveats') as HTMLElement
        expect(caveats.textContent).toContain('Works once: the first device to open it gets the file.')
        expect(caveats.textContent).toContain('Not encrypted. Use it only on a network you trust.')

        const glyphs = caveats.querySelectorAll('.fd-caveats__glyph')
        expect(glyphs).toHaveLength(2)
        for (const glyph of glyphs) expect(glyph.getAttribute('aria-hidden')).toBe('true')
    })

    it('states the no-extra-copy fact and the link-preview caveat inside "Trouble connecting?", not on the always-visible card', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        // The disclosure's own content stays mounted in the DOM regardless of
        // open state (Story 9.1's collapsed-content guarantee), so this is a
        // structural containment check -- inside the disclosure, not inside
        // the always-visible caveat list -- rather than a visibility one;
        // jsdom applies no CSS, so a presence check alone would pass whether
        // the string were on the card or behind the disclosure.
        const caveats = document.querySelector('.fd-caveats') as HTMLElement
        const help = document.querySelector('.fd-help .fd-disclosure__region') as HTMLElement
        expect(caveats.textContent).not.toContain('FairDrop keeps no copy.')
        expect(caveats.textContent).not.toContain('Link previews in chat apps')
        expect(help.textContent).toContain('FairDrop keeps no copy. The receiving device keeps what it downloads.')
        expect(help.textContent).toContain(
            'Link previews in chat apps can count as that first device, so paste the link straight into a browser.',
        )
    })

    /*
      Mutation the acceptance criteria name explicitly: moving the
      not-encrypted line inside "Trouble connecting?" must fail this test. It
      is the one security disclosure and stays visible on the card regardless
      of whether the disclosure is open.
    */
    it('never renders the not-encrypted disclosure inside "Trouble connecting?"', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        fireEvent.click(screen.getByRole('button', {name: 'Trouble connecting?'}))

        const region = document.querySelector('.fd-help .fd-disclosure__region') as HTMLElement
        expect(region.textContent).not.toContain('Not encrypted. Use it only on a network you trust.')
        expect(document.querySelector('.fd-caveats')?.textContent)
            .toContain('Not encrypted. Use it only on a network you trust.')
    })
})

describe('the non-terminal discovery warning', () => {
    it('renders as a warning banner while the QR and link stay usable', () => {
        const warned = staged({
            metadata: metadata({
                warnings: [{
                    code: 'beacon_warning',
                    message: 'Device discovery isn’t available. The QR code and download link still work.',
                }],
            }),
        })
        render(<StagedView state={warned} onCancel={vi.fn()}/>)

        const banner = document.querySelector('.fd-warning-banner')
        expect(banner?.textContent).toBe('Discovery unavailable' +
            'Device discovery isn’t available. The QR code and download link still work.')
        expect(screen.getByRole('img')).toBeTruthy()
        // A warning is not an Error, so no outcome panel appears.
        expect(document.querySelector('.fd-outcome')).toBeNull()
    })

    it('renders the warning banner inside the card, above the item row', () => {
        const warned = staged({
            metadata: metadata({
                warnings: [{code: 'beacon_warning', message: 'x'}],
            }),
        })
        const {container} = render(<StagedView state={warned} onCancel={vi.fn()}/>)

        const packet = container.querySelector('.fd-packet') as HTMLElement
        const banner = packet.querySelector('.fd-warning-banner')
        const item = packet.querySelector('.fd-item')
        expect(banner).toBeTruthy()
        expect(item).toBeTruthy()

        // DOCUMENT_POSITION_FOLLOWING means banner precedes item.
        const bannerPrecedesItem =
            (banner!.compareDocumentPosition(item!) & Node.DOCUMENT_POSITION_FOLLOWING) !== 0
        expect(bannerPrecedesItem).toBe(true)
    })

    it('shows no banner when metadata carries no warning', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        expect(document.querySelector('.fd-warning-banner')).toBeNull()
        expect(screen.getByRole('heading', {level: 1}).getAttribute('aria-describedby')).toBeNull()
    })

    /*
      Epic 1 retrospective item 4: on the one machine state this warning exists
      for, a screen-reader user was never told discovery was down.

      The warning arrives with the metadata and never after it, so the only
      transition it belongs to is Stage success -- which the routing table gives
      to this heading's focus move. The banner sat inside the packet, outside
      both the focused node and any live region, and the announcer row that
      would have spoken it fires only when a warnings array grows at an
      already-staged session, which no reducer path produces.

      Describing the heading with the banner is what that focus-owned row can
      carry without becoming a second owner: the move that announces Stage
      success reads the warning as part of the same announcement.
    */
    it('describes the focused heading with the warning, so the focus move speaks it', () => {
        const warning = {
            code: 'beacon_warning' as const,
            message: 'Device discovery isn’t available. The QR code and download link still work.',
        }
        render(<StagedView state={staged({metadata: metadata({warnings: [warning, warning]})})} onCancel={vi.fn()}/>)

        const heading = screen.getByRole('heading', {level: 1})
        const described = (heading.getAttribute('aria-describedby') ?? '').split(' ').filter(Boolean)
        expect(described.length).toBe(2)

        for (const id of described) {
            const banner = document.getElementById(id)
            expect(banner?.classList.contains('fd-warning-banner')).toBe(true)
            expect(banner?.textContent).toContain('Device discovery isn’t available.')
        }
    })
})

describe('cancellation and command failure', () => {
    it('offers Cancel and reports its activation', () => {
        const onCancel = vi.fn()
        render(<StagedView state={staged()} onCancel={onCancel}/>)

        const cancel = screen.getByRole('button', {name: 'Cancel'})
        expect(cancel.className).toContain('fd-target')

        fireEvent.click(cancel)
        expect(onCancel).toHaveBeenCalledTimes(1)
    })

    it('changes the label while a cancellation is outstanding and keeps the item readable', () => {
        render(<StagedView state={staged({cancelPending: true})} onCancel={vi.fn()}/>)

        expect(screen.getByRole('button', {name: 'Canceling'})).toBeTruthy()
        // level: 2 alone now also matches the "Trouble connecting?" disclosure
        // heading (Disclosure wraps its summary in an <h2>), so this names
        // the item name specifically.
        expect(screen.getByRole('heading', {level: 2, name: 'Travel Notes.pdf'}).textContent).toBe('Travel Notes.pdf')
    })

    it('shows a command failure on the fixed table without leaving Staged', () => {
        const error: PublicError = {
            code: 'source_changed',
            message: 'The item changed after it was prepared. Cancel and create a fresh link.',
        }
        render(<StagedView state={staged({commandError: error})} onCancel={vi.fn()}/>)

        expect(screen.getByRole('heading', {name: 'Item changed'})).toBeTruthy()
        expect(screen.getByRole('img')).toBeTruthy()
        expect(document.querySelectorAll('[data-phase-view]')).toHaveLength(1)
    })
})

describe('recovery help behind "Trouble connecting?"', () => {
    it('is collapsed by default, distinct from Idle\'s own disclosure', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        const trigger = screen.getByRole('button', {name: 'Trouble connecting?'})
        expect(trigger.getAttribute('aria-expanded')).toBe('false')
    })

    it('covers platform firewall recovery and every generic receiver failure once opened', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        fireEvent.click(screen.getByRole('button', {name: 'Trouble connecting?'}))

        expect(screen.getByText('Open Windows Firewall settings and allow FairDrop on Private networks only, ' +
            'then prepare the item again.')).toBeTruthy()
        expect(screen.getByText('Open System Settings → Network → Firewall → Options, allow incoming ' +
            'connections for FairDrop, then prepare the item again.')).toBeTruthy()
        expect(screen.getByText('Not downloading? Make sure both devices use the same local Wi-Fi. Guest or ' +
            'isolated networks may block device-to-device traffic. Then cancel and prepare the item again for ' +
            'a fresh link.')).toBeTruthy()
        expect(screen.getByText('Browser says Not Found: the link may be wrong or expired. Locked: another ' +
            'opener claimed it. Gone: the selected item changed. Cancel and prepare the item again for a ' +
            'fresh link.')).toBeTruthy()
    })

    it('keeps every one of these strings in the DOM regardless of open state (mutation: drop one -> fails, naming it)', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        const region = document.querySelector('.fd-help .fd-disclosure__region') as HTMLElement
        for (const text of [
            'Open Windows Firewall settings',
            'Open System Settings → Network → Firewall → Options',
            'Not downloading?',
            'Browser says Not Found',
            'FairDrop keeps no copy.',
            'Link previews in chat apps',
        ]) {
            expect(region.textContent, text).toContain(text)
        }
    })
})

describe('a pending cancellation', () => {
    it('keeps the control focused, marks it aria-disabled, and issues no second command', () => {
        const onCancel = vi.fn()
        const {rerender} = render(<StagedView state={staged()} onCancel={onCancel}/>)

        const cancel = screen.getByRole('button', {name: 'Cancel'})
        cancel.focus()
        fireEvent.click(cancel)
        expect(onCancel).toHaveBeenCalledTimes(1)

        rerender(<StagedView state={staged({cancelPending: true})} onCancel={onCancel}/>)

        const pending = screen.getByRole('button', {name: 'Canceling'})
        // The same element, so focus never left it -- which `disabled` would
        // have done, and `aria-disabled` does not.
        expect(pending).toBe(cancel)
        expect(document.activeElement).toBe(pending)
        expect(pending.getAttribute('aria-disabled')).toBe('true')
        expect(pending.hasAttribute('disabled')).toBe(false)

        fireEvent.click(pending)
        expect(onCancel).toHaveBeenCalledTimes(1)
    })

    it('carries no aria-disabled attribute at all while cancellation is available', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        expect(screen.getByRole('button', {name: 'Cancel'}).hasAttribute('aria-disabled')).toBe(false)
    })
})

describe('the announcer rows this view owns', () => {
    it('reports copy success to the announcer only after the write resolves', async () => {
        const onAnnounce = vi.fn()
        render(<StagedView state={staged()} onCancel={vi.fn()} onAnnounce={onAnnounce}/>)

        await act(async () => {
            pressCopy()
        })

        expect(onAnnounce).toHaveBeenCalledTimes(1)
        expect(onAnnounce).toHaveBeenCalledWith(sessionId, 'Copied')
    })

    it('says nothing when the write rejects', async () => {
        const onAnnounce = vi.fn()
        writeText.mockRejectedValue(new Error('denied'))
        render(<StagedView state={staged()} onCancel={vi.fn()} onAnnounce={onAnnounce}/>)

        await act(async () => {
            pressCopy()
        })

        expect(onAnnounce).not.toHaveBeenCalled()
    })

    it('marks the staged heading as the target Stage success focuses', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        const heading = document.querySelector('[data-focus-target="staged-heading"]') as HTMLElement
        expect(heading.tagName).toBe('H1')
        expect(heading.textContent).toBe('Ready to send')
        expect(heading.getAttribute('tabindex')).toBe('-1')
    })
})

/*
 * What a real pointer press does, which fireEvent.click alone does not.
 *
 * jsdom's click moves no focus, so a test that only clicks leaves the control
 * unfocused -- and the copy confirmation is deliberately only claimed while
 * the control holds focus, since otherwise no blur is ever coming to revert it
 * (D-114, reached by ordering). Clicking through this helper states the
 * precondition the behaviour depends on instead of quietly not having it.
 */
function pressCopy(): HTMLElement {
    const button = screen.getByRole('button', {name: 'Copy Link'})
    button.focus()
    fireEvent.click(button)
    return button
}

// A promise this test resolves on demand, so the clipboard command can be made
// to come back after focus has already moved on.
function deferred<T>() {
    let resolve!: (value: T | PromiseLike<T>) => void
    const promise = new Promise<T>((settle) => { resolve = settle })
    return {promise, resolve}
}

/*
  D-114 returning by the back door.

  The clipboard command is asynchronous. A sender who clicks Copy and tabs
  straight on can have it resolve after focus has already left -- and then no
  blur is ever coming to revert the label, so the control keeps the name
  "Copied" for the rest of the session. That is the original defect, reached by
  ordering rather than by the missing revert, and the review reproduced it in
  Chromium.

  The three tests above cannot see it: each dispatches focusout directly on a
  button that never held focus, so they pin the handler body rather than the
  event meant to deliver it. This one states the precondition instead --
  focus, then move it away before the command resolves.
*/
it('does not strand the confirmation when the copy resolves after focus has left', async () => {
    const pending = deferred<void>()
    writeText.mockReturnValue(pending.promise)
    render(<StagedView state={staged()} onCancel={vi.fn()}/>)

    const button = screen.getByRole('button', {name: 'Copy Link'})
    button.focus()
    expect(document.activeElement).toBe(button)

    fireEvent.click(button)
    // The sender moves on before the command comes back.
    fireEvent.blur(button, {relatedTarget: document.body})
    await act(async () => {
        pending.resolve()
        await pending.promise
    })

    expect(screen.getByRole('button', {name: 'Copy Link'})).toBeTruthy()
    expect(screen.queryByRole('button', {name: 'Copied'})).toBeNull()
})
