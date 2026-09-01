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
        const field = screen.getByRole('textbox') as HTMLInputElement
        expect(field.tagName).toBe('INPUT')
        expect(field.value).toBe(capabilityURL)
        expect(field.readOnly).toBe(true)
        expect(field.className).toContain('fd-target')
        expect(field.getAttribute('aria-labelledby')).toBe('fd-direct-link-heading')
        expect(container.querySelectorAll('a')).toHaveLength(0)
    })

    it('exposes the capability token once, and never as prose or a link target', () => {
        const {container} = render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        const serialized = container.innerHTML
        const occurrences = serialized.split(token).length - 1
        expect(occurrences).toBe(1)
        expect((screen.getByRole('textbox') as HTMLInputElement).value).toContain(token)
        // The QR carries the same URL as an image, not as readable text.
        expect(screen.getByRole('img').getAttribute('src')).not.toContain(token)
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
            fireEvent.click(screen.getByRole('button', {name: 'Copy download link'}))
        })

        expect(writeText).toHaveBeenCalledWith(capabilityURL)
        expect(screen.getByRole('button', {name: 'Copied'})).toBeTruthy()
        expect(screen.queryByRole('button', {name: 'Copy download link'})).toBeNull()
    })

    it('leaves the action label alone when the clipboard write rejects', async () => {
        writeText.mockRejectedValue(new Error('denied'))
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        await act(async () => {
            fireEvent.click(screen.getByRole('button', {name: 'Copy download link'}))
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
            fireEvent.click(screen.getByRole('button', {name: 'Copy download link'}))
        })

        expect(writeText).toHaveBeenCalledTimes(1)
        expect(browserWrite).not.toHaveBeenCalled()
    })

    it('changes nothing but the label: no toast, no lifecycle move, no cleared clipboard', async () => {
        render(<StagedView state={staged()} onCancel={vi.fn()}/>)

        await act(async () => {
            fireEvent.click(screen.getByRole('button', {name: 'Copy download link'}))
        })

        expect(writeText).toHaveBeenCalledTimes(1)
        expect(screen.getByRole('heading', {level: 1}).textContent).toBe('Ready to pass along')
        expect((screen.getByRole('textbox') as HTMLInputElement).value).toBe(capabilityURL)
        expect(document.querySelector('[role="alert"]')).toBeNull()
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
            fireEvent.click(screen.getByRole('button', {name: 'Copy download link'}))
        })

        expect(onAnnounce).toHaveBeenCalledTimes(1)
        expect(onAnnounce).toHaveBeenCalledWith('Copied')
    })

    it('says nothing when the write rejects', async () => {
        const onAnnounce = vi.fn()
        writeText.mockRejectedValue(new Error('denied'))
        render(<StagedView state={staged()} onCancel={vi.fn()} onAnnounce={onAnnounce}/>)

        await act(async () => {
            fireEvent.click(screen.getByRole('button', {name: 'Copy download link'}))
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
