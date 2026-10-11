import {act, cleanup, fireEvent, render, screen, within} from '@testing-library/react'
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'

const mocks = vi.hoisted(() => ({copyToClipboard: vi.fn()}))

vi.mock('../../wailsjs/go/main/App', () => ({CopyToClipboard: mocks.copyToClipboard}))

import type {ReceiveWaitingTransferState} from '../transfer/state'
import type {ReceiveMetadata} from '../transfer/types'
import {ReceiveWaitingView} from './ReceiveWaitingView'

afterEach(cleanup)

const sessionId = '0102030405060708090a0b0c0d0e0f10'
const token = '1112131415161718191a1b1c1d1e1f20'
const uploadURL = `http://192.168.1.50:45678/upload/${token}`
const qrPNG = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII='

function metadata(overrides: Partial<ReceiveMetadata> = {}): ReceiveMetadata {
    return {sessionId, destination: 'Phone Photos', url: uploadURL, qrBase64: qrPNG, warnings: [], ...overrides}
}

function waiting(overrides: Partial<ReceiveWaitingTransferState> = {}): ReceiveWaitingTransferState {
    return {
        phase: 'receive-waiting',
        session: {sessionId, lastSeq: 0},
        metadata: metadata(),
        cancelPending: false,
        commandError: null,
        refusals: 0,
        ...overrides,
    }
}

beforeEach(() => {
    mocks.copyToClipboard.mockReset()
    mocks.copyToClipboard.mockResolvedValue(undefined)
})

describe('the receive waiting view', () => {
    it('leads with the receive heading and the phone instruction', () => {
        render(<ReceiveWaitingView state={waiting()} onCancel={vi.fn()}/>)

        const heading = screen.getByRole('heading', {level: 1, name: 'Ready to receive'})
        expect(heading.getAttribute('data-focus-target')).toBe('receive-waiting-heading')
        expect(heading.getAttribute('tabindex')).toBe('-1')
        expect(screen.getByText('Scan the code with the phone’s camera, then choose the files to upload.')).toBeTruthy()
    })

    it('renders the QR from the bare base64 with an accessible name that says upload and names the folder', () => {
        render(<ReceiveWaitingView state={waiting()} onCancel={vi.fn()}/>)

        const image = screen.getByRole('img') as HTMLImageElement
        expect(image.getAttribute('src')).toBe(`data:image/png;base64,${qrPNG}`)
        expect(image.getAttribute('alt')).toBe('Upload QR code for Phone Photos')
    })

    it('marks the QR image non-draggable (AGENTS.md, WebKit fact 6)', () => {
        render(<ReceiveWaitingView state={waiting()} onCancel={vi.fn()}/>)

        const image = screen.getByRole('img') as HTMLImageElement
        expect(image.getAttribute('draggable')).toBe('false')
        expect(image.draggable).toBe(false)
    })

    it('inserts a $-bearing folder name into the QR name exactly as given', () => {
        render(<ReceiveWaitingView state={waiting({metadata: metadata({destination: "Dad's $& $` photos"})})} onCancel={vi.fn()}/>)

        expect(screen.getByRole('img').getAttribute('alt')).toBe("Upload QR code for Dad's $& $` photos")
    })

    it('names the destination by its basename in a bidi isolate, with the new-folder note', () => {
        render(<ReceiveWaitingView state={waiting({metadata: metadata({destination: 'مجلد الصور'})})} onCancel={vi.fn()}/>)

        const isolate = document.querySelector('bdi')
        expect(isolate?.getAttribute('dir')).toBe('auto')
        expect(isolate?.textContent).toBe('مجلد الصور')
        expect(screen.getByText('Each upload is saved in a new folder here.')).toBeTruthy()
    })

    it('shows the one-upload caveat and the not-encrypted disclosure, and nothing about downloading', () => {
        render(<ReceiveWaitingView state={waiting()} onCancel={vi.fn()}/>)

        const caveats = document.querySelector('.fd-caveats')
        expect(within(caveats as HTMLElement).getByText('Works once: this link accepts one upload.')).toBeTruthy()
        expect(within(caveats as HTMLElement).getByText('Not encrypted. Use it only on a network you trust.')).toBeTruthy()
        expect(screen.queryByText(/Tap Download/)).toBeNull()
        expect(screen.queryByText(/first device to tap Download/)).toBeNull()
    })

    it('has no anchor, so this window cannot spend its own one-time upload', () => {
        render(<ReceiveWaitingView state={waiting()} onCancel={vi.fn()}/>)
        expect(document.querySelector('a')).toBeNull()
    })
})

describe('the upload link', () => {
    it('is not rendered, focusable or exposed until Show Link is activated', () => {
        render(<ReceiveWaitingView state={waiting()} onCancel={vi.fn()}/>)

        const show = screen.getByRole('button', {name: 'Show Link'})
        expect(show.getAttribute('aria-expanded')).toBe('false')
        const region = document.getElementById(show.getAttribute('aria-controls') as string)
        expect(region?.hasAttribute('data-open')).toBe(false)
    })

    it('reveals a readonly field named Upload link holding the exact url, and hides it again', () => {
        render(<ReceiveWaitingView state={waiting()} onCancel={vi.fn()}/>)

        fireEvent.click(screen.getByRole('button', {name: 'Show Link'}))

        const field = screen.getByRole('textbox', {name: 'Upload link', hidden: true}) as HTMLTextAreaElement
        expect(field.value).toBe(uploadURL)
        expect(field.readOnly).toBe(true)
        const hide = screen.getByRole('button', {name: 'Hide Link'})
        expect(hide.getAttribute('aria-expanded')).toBe('true')

        fireEvent.click(hide)
        expect(screen.getByRole('button', {name: 'Show Link'}).getAttribute('aria-expanded')).toBe('false')
    })

    it('copies the exact upload link through the bound command and confirms it, without revealing it', async () => {
        const onAnnounce = vi.fn()
        render(<ReceiveWaitingView state={waiting()} onCancel={vi.fn()} onAnnounce={onAnnounce}/>)

        const copyButton = screen.getByRole('button', {name: 'Copy Link'})
        await act(async () => { fireEvent.click(copyButton) })

        expect(mocks.copyToClipboard).toHaveBeenCalledTimes(1)
        expect(mocks.copyToClipboard).toHaveBeenCalledWith(uploadURL)
        expect(onAnnounce).toHaveBeenCalledWith(sessionId, 'Copied')
        expect(screen.getByRole('button', {name: 'Show Link'}).getAttribute('aria-expanded')).toBe('false')
    })

    it('puts focus on the copy control on activation, because WebKit does not focus a clicked button', async () => {
        render(<ReceiveWaitingView state={waiting()} onCancel={vi.fn()}/>)

        const copyButton = screen.getByRole('button', {name: 'Copy Link'})
        expect(document.activeElement).not.toBe(copyButton)
        await act(async () => { fireEvent.click(copyButton) })
        expect(document.activeElement).toBe(copyButton)
    })

    it('reports a clipboard rejection to its owner and never claims Copied', async () => {
        mocks.copyToClipboard.mockRejectedValue(new Error('denied'))
        const onAnnounce = vi.fn()
        const onCopyFailed = vi.fn()
        render(<ReceiveWaitingView state={waiting()} onCancel={vi.fn()} onAnnounce={onAnnounce} onCopyFailed={onCopyFailed}/>)

        await act(async () => { fireEvent.click(screen.getByRole('button', {name: 'Copy Link'})) })

        expect(onCopyFailed).toHaveBeenCalledWith(sessionId)
        expect(onAnnounce).not.toHaveBeenCalled()
        expect(screen.getByRole('button', {name: 'Copy Link'})).toBeTruthy()
    })

    it('shows the clipboard failure panel in the command-error slot', () => {
        render(<ReceiveWaitingView
            state={waiting({commandError: {code: 'clipboard_failed', message: 'x'}})}
            onCancel={vi.fn()}
        />)

        const panel = document.querySelector('[data-focus-target="command-error"]')
        expect(panel?.getAttribute('data-error-code')).toBe('clipboard_failed')
        expect(within(panel as HTMLElement).getByText('Couldn’t copy the link')).toBeTruthy()
    })
})

describe('a refused upload', () => {
    it('shows nothing before any upload was refused', () => {
        render(<ReceiveWaitingView state={waiting()} onCancel={vi.fn()}/>)

        expect(document.querySelector('[data-receive-notice]')).toBeNull()
        expect(screen.queryByText('Upload too large')).toBeNull()
    })

    it('keeps waiting and says what happened and what to do, as plain text beside the code', () => {
        render(<ReceiveWaitingView state={waiting({refusals: 1})} onCancel={vi.fn()}/>)

        const note = document.querySelector('[data-receive-notice="receive_too_large"]') as HTMLElement
        expect(within(note).getByText('Upload too large')).toBeTruthy()
        expect(within(note).getByText(/A phone tried to upload more than this computer has room for\. FairDrop is still waiting\./)).toBeTruthy()
        // Still the waiting view: the code and Cancel are there.
        expect(screen.getByRole('img')).toBeTruthy()
        expect(screen.getByRole('button', {name: 'Cancel'})).toBeTruthy()
        // The announcer owns the speech; the note is not a live region of its own.
        expect(note.getAttribute('role')).toBeNull()
        expect(note.getAttribute('aria-live')).toBeNull()
    })

    it('shows one note however many uploads were refused', () => {
        render(<ReceiveWaitingView state={waiting({refusals: 3})} onCancel={vi.fn()}/>)
        expect(document.querySelectorAll('[data-receive-notice]')).toHaveLength(1)
    })

    it('describes the heading by the note so the one focus move reads it too', () => {
        render(<ReceiveWaitingView state={waiting({refusals: 1})} onCancel={vi.fn()}/>)

        const heading = screen.getByRole('heading', {level: 1})
        const ids = (heading.getAttribute('aria-describedby') ?? '').split(' ')
        expect(ids).toContain('fd-receive-refused')
        expect(document.getElementById('fd-receive-refused')).toBeTruthy()
    })

    it('does not describe the heading at all when there is nothing to add', () => {
        render(<ReceiveWaitingView state={waiting()} onCancel={vi.fn()}/>)
        expect(screen.getByRole('heading', {level: 1}).hasAttribute('aria-describedby')).toBe(false)
    })
})

describe('the discovery warning', () => {
    it('uses receive wording, not the download wording the shared registry carries', () => {
        render(<ReceiveWaitingView
            state={waiting({metadata: metadata({warnings: [{code: 'beacon_warning', message: 'Device discovery isn’t available. The QR code and download link still work.'}]})})}
            onCancel={vi.fn()}
        />)

        const banner = document.querySelector('[data-warning-code="beacon_warning"]') as HTMLElement
        expect(within(banner).getByText('Discovery unavailable')).toBeTruthy()
        expect(within(banner).getByText('Device discovery isn’t available. The QR code and upload link still work.')).toBeTruthy()
        expect(banner.textContent).not.toContain('download link')
    })
})

describe('cancelling', () => {
    it('calls onCancel once, and says Cancel', () => {
        const onCancel = vi.fn()
        render(<ReceiveWaitingView state={waiting()} onCancel={onCancel}/>)

        fireEvent.click(screen.getByRole('button', {name: 'Cancel'}))
        expect(onCancel).toHaveBeenCalledTimes(1)
    })

    it('keeps the control, says Canceling, and answers a second activation with nothing', () => {
        const onCancel = vi.fn()
        render(<ReceiveWaitingView state={waiting({cancelPending: true})} onCancel={onCancel}/>)

        const cancel = screen.getByRole('button', {name: 'Canceling'})
        expect(cancel.getAttribute('aria-disabled')).toBe('true')
        expect(cancel.hasAttribute('disabled')).toBe(false)
        fireEvent.click(cancel)
        expect(onCancel).not.toHaveBeenCalled()
    })

    it('offers receive-specific trouble guidance behind a disclosure', () => {
        render(<ReceiveWaitingView state={waiting()} onCancel={vi.fn()}/>)

        const toggle = screen.getByRole('button', {name: 'Trouble connecting?'})
        fireEvent.click(toggle)
        expect(screen.getByText(/Phone can’t open the page\? Make sure both devices use the same local Wi-Fi\./)).toBeTruthy()
        expect(screen.getByText(/choose Receive again for a fresh link\./)).toBeTruthy()
    })
})
