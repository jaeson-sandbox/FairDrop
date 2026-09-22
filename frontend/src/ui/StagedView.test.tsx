import {act, cleanup, fireEvent, render, screen} from '@testing-library/react'
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'

const mocks = vi.hoisted(() => ({copyToClipboard: vi.fn()}))

vi.mock('../../wailsjs/go/main/App', () => ({
    CopyToClipboard: mocks.copyToClipboard,
}))
import type {StagedTransferState} from '../transfer/state'
import type {FileMetadata, PublicError} from '../transfer/types'
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

        expect(screen.getByRole('heading', {level: 1, name: 'Ready to pass along'})).toBeTruthy()
        expect(screen.getByText('Scan this code on the receiving device to start the download.')).toBeTruthy()
    })

    it('renders the QR from the bare base64, prefixing the data URL only at render', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        const image = screen.getByRole('img') as HTMLImageElement
        expect(image.getAttribute('src')).toBe(`data:image/png;base64,${qrPNG}`)
        expect(image.getAttribute('alt')).toBe('Download QR code for Travel Notes.pdf')
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

    it('says a folder downloads as a ZIP and labels its size as logical', () => {
        const state = staged({metadata: metadata({name: "Dad's PDFs", isDir: true, size: 36_800_000})})
        render(<StagedView state={state} onCancel={vi.fn()}/>)

        expect(screen.getByText('This folder downloads as a ZIP.')).toBeTruthy()
        expect(screen.getByText('Folder · 36.8 MB logical size')).toBeTruthy()
    })

    it('omits the folder note for a file', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        expect(screen.queryByText('This folder downloads as a ZIP.')).toBeNull()
    })
})

describe('the direct URL row', () => {
    it('is a readonly form control rather than a sender-side activation link', () => {
        const {container} = render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        // A real <input readonly>, not a div wearing role="textbox": assistive
        // technology reads the value of one reliably and disagrees about the
        // other. It is named by the row's own heading and it is still not a link.
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
        expect(field.getAttribute('aria-labelledby')).toBe('fd-direct-link-heading')
        expect(container.querySelectorAll('a')).toHaveLength(0)
    })

    it('exposes the capability token as readable content exactly once, and never as prose or a link target', () => {
        const {container} = render(<StagedView state={staged()} onCancel={vi.fn()}/>)

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

    it('selects the whole capability URL on focus, as the manual fallback', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)
        const field = screen.getByRole('textbox') as HTMLInputElement
        const select = vi.spyOn(field, 'select')

        fireEvent.focus(field)

        // The <div> this replaced carried `user-select: all`, so one click took
        // the whole URL. An input has no such behaviour, and this is the path a
        // user needs when the clipboard command fails.
        expect(select).toHaveBeenCalledTimes(1)
    })

    it('keeps the click that focuses the URL from collapsing its selection', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)
        const field = screen.getByRole('textbox') as HTMLTextAreaElement

        const prevented = !fireEvent.mouseDown(field)

        // select-on-focus alone is defeated by the mouseup that follows: the
        // selection collapses to a caret and the manual fallback is a call
        // that happened and a selection nobody got.
        expect(prevented).toBe(true)
        expect(document.activeElement).toBe(field)
    })

    it('carries the direct-link helper beside the action', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        expect(screen.getByRole('button', {name: 'Copy download link'})).toBeTruthy()
        // The copy action was one of two controls with no floor assertion:
        // stripping fd-target from it passed the whole suite.
        expect(screen.getByRole('button', {name: 'Copy download link'}).className).toContain('fd-target')
        expect(screen.getByText('Open this link directly in the receiving device’s browser.')).toBeTruthy()
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
        expect(screen.queryByRole('button', {name: 'Copy download link'})).toBeNull()
    })

    it('leaves the action label alone when the clipboard write rejects', async () => {
        writeText.mockRejectedValue(new Error('denied'))
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        await act(async () => {
            pressCopy()
        })

        expect(screen.getByRole('button', {name: 'Copy download link'})).toBeTruthy()
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
        expect(screen.getByRole('heading', {level: 1}).textContent).toBe('Ready to pass along')
        expect((screen.getByRole('textbox') as HTMLInputElement).value).toBe(capabilityURL)
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

        expect(screen.getByRole('button', {name: 'Copy download link'})).toBeTruthy()
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

        expect(screen.getByRole('button', {name: 'Copy download link'})).toBeTruthy()
        expect(screen.queryByRole('button', {name: 'Copied'})).toBeNull()
    })

    it('still names the action when the sender returns to the control later', async () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        const button = () => screen.getByRole('button', {name: /^(Copy download link|Copied)$/})
        await act(async () => {
            // Focus first: a real press focuses before it clicks, and the
            // confirmation is only claimed while the control holds focus.
            button().focus()
            fireEvent.click(button())
        })
        expect(button().textContent).toBe('Copied')

        // The sender moves on -- Tab reaches Cancel, say -- and later returns.
        fireEvent.blur(button(), {relatedTarget: screen.getByRole('button', {name: 'Cancel'})})
        fireEvent.focus(button())

        expect(button().textContent).toBe('Copy download link')
        expect(screen.getByRole('button', {name: 'Copy download link'})).toBe(button())
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
    it('states the first-opener limit, the unencrypted network, and the no-extra-copy fact', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        expect(screen.getByText('One device only—the first device or software to open this link starts the ' +
            'download. Link previews may use this V1 link before the intended browser.')).toBeTruthy()
        expect(screen.getByText('Use FairDrop only on a network you trust. The transfer is not encrypted, so ' +
            'someone monitoring this network may be able to observe it.')).toBeTruthy()
        expect(screen.getByText('Sent directly over your local network. FairDrop does not upload or store an ' +
            'extra copy. The receiving device keeps the downloaded file.')).toBeTruthy()
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
        expect((screen.getByRole('textbox') as HTMLInputElement).value).toBe(capabilityURL)
        // A warning is not an Error, so no outcome panel appears.
        expect(document.querySelector('.fd-outcome')).toBeNull()
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

        expect(screen.getByRole('button', {name: 'Canceling…'})).toBeTruthy()
        expect(screen.getByRole('heading', {level: 2}).textContent).toBe('Travel Notes.pdf')
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

describe('full-value access to a long or bidi name', () => {
    const unbroken = 'Q1-report-' + 'x'.repeat(180) + '.pdf'

    it('clamps the name visually while keeping the complete value in the DOM', () => {
        render(<StagedView state={staged({metadata: metadata({name: unbroken})})} onCancel={vi.fn()}/>)

        const isolate = document.querySelector('bdi') as HTMLElement
        expect(isolate.className).toContain('fd-clamp')
        // Clamped by its box, never by JavaScript: no code unit is dropped.
        expect(isolate.textContent).toBe(unbroken)
    })

    it('offers a persistent keyboard control that expands the clamp', () => {
        render(<StagedView state={staged({metadata: metadata({name: unbroken})})} onCancel={vi.fn()}/>)

        const control = screen.getByRole('button', {name: 'Show full name'})
        expect(control.className).toContain('fd-target')
        expect(control.getAttribute('aria-expanded')).toBe('false')

        fireEvent.click(control)

        expect(screen.getByRole('button', {name: 'Show full name'}).getAttribute('aria-expanded')).toBe('true')
        expect((document.querySelector('bdi') as HTMLElement).className).not.toContain('fd-clamp')
    })

    it('describes the control with the complete value, so a tooltip is not the only route', () => {
        render(<StagedView state={staged({metadata: metadata({name: unbroken})})} onCancel={vi.fn()}/>)

        const describedBy = screen.getByRole('button', {name: 'Show full name'}).getAttribute('aria-describedby')
        const description = document.getElementById(describedBy ?? '')
        expect(description?.textContent).toBe(unbroken)
        expect(description?.className).toContain('fd-visually-hidden')
    })

    it('isolates both copies of a mixed-direction name', () => {
        const mixed = 'تقرير ٢٠٢٦ ‮report‬.pdf'
        render(<StagedView state={staged({metadata: metadata({name: mixed})})} onCancel={vi.fn()}/>)

        const isolates = [...document.querySelectorAll('bdi')]
        expect(isolates).toHaveLength(2)
        for (const isolate of isolates) {
            expect(isolate.getAttribute('dir')).toBe('auto')
            expect(isolate.textContent).toBe(mixed)
        }
        // The accessible name of the QR matches the visible name exactly.
        expect(screen.getByRole('img').getAttribute('alt')).toBe(`Download QR code for ${mixed}`)
    })
})

describe('recovery help beside the handoff', () => {
    it('covers platform firewall recovery and every generic receiver failure', () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        expect(document.querySelector('.fd-packet .fd-help')).toBeTruthy()
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

        const pending = screen.getByRole('button', {name: 'Canceling…'})
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
        expect(heading.textContent).toBe('Ready to pass along')
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
    const button = screen.getByRole('button', {name: 'Copy download link'})
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

    const button = screen.getByRole('button', {name: 'Copy download link'})
    button.focus()
    expect(document.activeElement).toBe(button)

    fireEvent.click(button)
    // The sender moves on before the command comes back.
    fireEvent.blur(button, {relatedTarget: document.body})
    await act(async () => {
        pending.resolve()
        await pending.promise
    })

    expect(screen.getByRole('button', {name: 'Copy download link'})).toBeTruthy()
    expect(screen.queryByRole('button', {name: 'Copied'})).toBeNull()
})
