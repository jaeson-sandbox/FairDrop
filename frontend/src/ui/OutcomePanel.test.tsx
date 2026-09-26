import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {afterEach, describe, expect, it, vi} from 'vitest'
import type {CompletionReceipt, PublicError} from '../transfer/types'
import {OutcomePanel} from './OutcomePanel'

afterEach(cleanup)

function panel(): HTMLElement {
    const found = document.querySelector('.fd-outcome')
    expect(found, 'an outcome panel').toBeTruthy()
    return found as HTMLElement
}

const doneReceipt: CompletionReceipt = {
    name: 'report.pdf',
    isDir: false,
    bytesSent: 100,
}

function doneOutcome(retained: boolean) {
    return {kind: 'done', retained, receipt: doneReceipt} as const
}

function browseHandlers() {
    return {onSelectFile: vi.fn(), onSelectDirectory: vi.fn()}
}

describe('the Done card', () => {
    it('says only "Sent", with no body paragraph', () => {
        render(<OutcomePanel outcome={doneOutcome(false)}/>)

        expect(screen.getByRole('heading').textContent).toBe('Sent')
        expect(panel().getAttribute('data-outcome')).toBe('done')
        // copy.done.body is removed (Story 9.6): the receipt now carries what
        // completion means, so there is no separate paragraph beside it.
        expect(screen.queryByText('FairDrop finished sending the item.')).toBeNull()
        expect(screen.queryByText('Transfer finished')).toBeNull()
    })

    it('claims nothing about the receiver, its storage, or the download', () => {
        render(<OutcomePanel outcome={doneOutcome(false)}/>)

        const text = panel().textContent ?? ''
        for (const claim of ['saved', 'received', 'downloaded', 'stored', 'Files']) {
            expect(text, claim).not.toContain(claim)
        }
    })

    // Story 9.6 AC1: Send Another is unconditional -- live or retained -- so
    // the way forward never depends on whether the backend's reset already
    // landed (D-059's original concern, now structural rather than a
    // control the caller might omit).
    it('always offers Send Another, live or retained', () => {
        render(<OutcomePanel outcome={doneOutcome(false)}/>)
        expect(screen.getByRole('button', {name: 'Send Another'})).toBeTruthy()

        cleanup()
        render(<OutcomePanel outcome={doneOutcome(true)} onDismiss={vi.fn()}/>)
        expect(screen.getByRole('button', {name: 'Send Another'})).toBeTruthy()
    })

    it('wires Send Another to the browse menu the caller supplies', () => {
        const {onSelectFile, onSelectDirectory} = browseHandlers()
        render(
            <OutcomePanel
                outcome={doneOutcome(false)}
                browse={{label: 'Send Another', onSelectFile, onSelectDirectory}}
            />,
        )

        fireEvent.click(screen.getByRole('button', {name: 'Send Another'}))
        fireEvent.click(screen.getByRole('menuitem', {name: 'File'}))

        expect(onSelectFile).toHaveBeenCalledTimes(1)
        expect(onSelectDirectory).not.toHaveBeenCalled()
    })

    it('labels its own Dismiss "Done", quiet rather than primary -- Send Another already carries the weight', () => {
        const onDismiss = vi.fn()
        render(<OutcomePanel outcome={doneOutcome(false)} onDismiss={onDismiss}/>)

        const done = screen.getByRole('button', {name: 'Done'})
        expect(done.className).toContain('fd-button--quiet')
        expect(done.className).not.toContain('fd-button--primary')

        fireEvent.click(done)
        expect(onDismiss).toHaveBeenCalledTimes(1)
    })

    it('offers no Dismiss/Done when the caller supplies no handler, but still offers Send Another', () => {
        render(<OutcomePanel outcome={doneOutcome(false)}/>)

        expect(screen.queryByRole('button', {name: 'Done'})).toBeNull()
        expect(screen.getByRole('button', {name: 'Send Another'})).toBeTruthy()
    })

    it('marks itself retained so a reader can tell status from session', () => {
        render(<OutcomePanel outcome={doneOutcome(true)} onDismiss={vi.fn()}/>)

        expect(panel().getAttribute('data-retained')).toBe('true')
    })

    it('carries the inherited Wails drop-target style when the caller supplies one', () => {
        render(
            <OutcomePanel
                outcome={doneOutcome(true)}
                onDismiss={vi.fn()}
                dropTargetStyle={{'--wails-drop-target': 'drop'} as never}
            />,
        )

        expect(panel().style.getPropertyValue('--wails-drop-target')).toBe('drop')
    })
})

describe('the Error card', () => {
    it.each([
        ['invalid_selection', 'Choose one item', 'Choose exactly one file or folder.'],
        [
            'busy',
            'FairDrop is still busy',
            'FairDrop is still finishing the last item. If it doesn’t finish, close FairDrop and reopen it.',
        ],
        ['path_not_found', 'Item not found', 'That file or folder is no longer available. Choose it again.'],
        ['setup_failed', 'Couldn’t prepare that item', 'FairDrop couldn’t prepare that item. Nothing was sent. Choose it again.'],
        ['source_changed', 'Item changed', 'The item changed after it was prepared. Cancel and create a fresh link.'],
        [
            'transfer_failed',
            'Transfer stopped',
            'The transfer stopped before FairDrop finished sending. Check the local network and create a fresh link.',
        ],
        ['shutting_down', 'FairDrop is closing', 'FairDrop is closing. Reopen it to start a transfer.'],
    ])('renders the fixed heading and message for %s', (code, heading, message) => {
        const error = {code, message} as PublicError
        render(<OutcomePanel outcome={{kind: 'error', retained: false, error}}/>)

        expect(screen.getByRole('heading').textContent).toBe(heading)
        expect(screen.getByText(message)).toBeTruthy()
        expect(panel().getAttribute('data-error-code')).toBe(code)
    })

    it('ignores the message carried by the error and uses the fixed table', () => {
        const doctored = {
            code: 'path_not_found',
            message: String.raw`open C:\Users\jaeson\secrets\report.pdf: no such file`,
        } as PublicError

        render(<OutcomePanel outcome={{kind: 'error', retained: false, error: doctored}}/>)

        expect(panel().textContent).toContain('That file or folder is no longer available. Choose it again.')
        expect(panel().textContent).not.toContain('jaeson')
        expect(panel().textContent).not.toContain('no such file')
    })

    /*
      Story 9.6 AC3's three named mutations, in one describe block: this
      component renders exactly the primary its caller wires, never invents
      one and never repurposes Try Again into a chooser. The caller (App.tsx)
      is what actually calls `selectEffectiveErrorAction`; this proves the
      component side of the contract -- given each shape of wiring, exactly
      that control appears and does exactly what it was given.
    */
    describe('renders exactly the primary action it is given (Story 9.6 AC3)', () => {
        it('renders Try Again, with a refresh glyph, calling onRetry -- never a browse chooser', () => {
            const onRetry = vi.fn()
            const {onSelectFile, onSelectDirectory} = browseHandlers()
            const error: PublicError = {code: 'transfer_failed', message: 'x'}
            render(
                <OutcomePanel
                    outcome={{kind: 'error', retained: false, error}}
                    onRetry={onRetry}
                    onDismiss={vi.fn()}
                />,
            )

            const tryAgain = screen.getByRole('button', {name: 'Try Again'})
            expect(tryAgain.className).toContain('fd-button--primary')
            expect(tryAgain.querySelector('svg')).toBeTruthy()
            expect(screen.queryByRole('menu')).toBeNull()

            fireEvent.click(tryAgain)
            expect(onRetry).toHaveBeenCalledTimes(1)
            expect(onSelectFile).not.toHaveBeenCalled()
            expect(onSelectDirectory).not.toHaveBeenCalled()
            // *Mutation named in the AC:* "Try Again opening the chooser" ->
            // this assertion is what would fail: no menu ever opens from it.
            expect(screen.queryByRole('button', {name: 'Choose Another'})).toBeNull()
        })

        it('renders Choose Another as the browse menu when the caller wires `browse`, not Try Again', () => {
            const {onSelectFile, onSelectDirectory} = browseHandlers()
            const error: PublicError = {code: 'path_not_found', message: 'x'}
            render(
                <OutcomePanel
                    outcome={{kind: 'error', retained: false, error}}
                    browse={{label: 'Choose Another', onSelectFile, onSelectDirectory}}
                    onDismiss={vi.fn()}
                />,
            )

            expect(screen.queryByRole('button', {name: 'Try Again'})).toBeNull()
            const chooseAnother = screen.getByRole('button', {name: 'Choose Another'})
            expect(chooseAnother.className).toContain('fd-button--primary')

            fireEvent.click(chooseAnother)
            fireEvent.click(screen.getByRole('menuitem', {name: 'Folder'}))
            expect(onSelectDirectory).toHaveBeenCalledTimes(1)
            expect(onSelectFile).not.toHaveBeenCalled()
        })

        // *Mutation named in the AC:* "offer a primary for not_ready" ->
        // failing to omit both `onRetry` and `browse` here is exactly that
        // regression; `not_ready`'s row is `dismiss` (no primary at all).
        it('offers no primary at all when the caller wires neither -- not_ready\'s own shape', () => {
            const error: PublicError = {code: 'not_ready', message: 'x'}
            render(<OutcomePanel outcome={{kind: 'error', retained: false, error}} onDismiss={vi.fn()}/>)

            expect(screen.queryByRole('button', {name: 'Try Again'})).toBeNull()
            expect(screen.queryByRole('button', {name: 'Choose Another'})).toBeNull()
            expect(screen.getAllByRole('button')).toHaveLength(1)
        })
    })

    it('makes Dismiss primary-weighted only when it is the card\'s one control', () => {
        const error: PublicError = {code: 'not_ready', message: 'x'}
        render(<OutcomePanel outcome={{kind: 'error', retained: false, error}} onDismiss={vi.fn()}/>)

        const dismiss = screen.getByRole('button', {name: 'Dismiss'})
        expect(dismiss.className).toContain('fd-button--primary')
        expect(dismiss.className).not.toContain('fd-button--quiet')
    })

    it('makes Dismiss quiet once a primary (Try Again) exists beside it', () => {
        const error: PublicError = {code: 'transfer_failed', message: 'x'}
        render(
            <OutcomePanel
                outcome={{kind: 'error', retained: false, error}}
                onRetry={vi.fn()}
                onDismiss={vi.fn()}
            />,
        )

        const dismiss = screen.getByRole('button', {name: 'Dismiss'})
        expect(dismiss.className).toContain('fd-button--quiet')
        expect(dismiss.className).not.toContain('fd-button--primary')
    })

    it('offers no control when the caller supplies no handler and no action at all', () => {
        const error: PublicError = {code: 'not_ready', message: 'x'}
        render(<OutcomePanel outcome={{kind: 'error', retained: false, error}}/>)

        expect(screen.queryByRole('button')).toBeNull()
    })
})

describe('the busy state (Story 9.6)', () => {
    it('marks the primary and Dismiss aria-disabled, never the native disabled attribute, while busy', () => {
        const onRetry = vi.fn()
        const onDismiss = vi.fn()
        const error: PublicError = {code: 'transfer_failed', message: 'x'}
        render(
            <OutcomePanel
                outcome={{kind: 'error', retained: false, error}}
                onRetry={onRetry}
                onDismiss={onDismiss}
                busy
            />,
        )

        const tryAgain = screen.getByRole('button', {name: 'Try Again'})
        const dismiss = screen.getByRole('button', {name: 'Dismiss'})
        expect(tryAgain.getAttribute('aria-disabled')).toBe('true')
        expect(tryAgain.hasAttribute('disabled')).toBe(false)
        expect(dismiss.getAttribute('aria-disabled')).toBe('true')
        expect(dismiss.hasAttribute('disabled')).toBe(false)

        // A control that keeps taking focus while busy, not one Tab skips.
        tryAgain.focus()
        expect(document.activeElement).toBe(tryAgain)
        dismiss.focus()
        expect(document.activeElement).toBe(dismiss)
    })

    it('ignores a click on the primary or Dismiss while busy', () => {
        const onRetry = vi.fn()
        const onDismiss = vi.fn()
        const error: PublicError = {code: 'transfer_failed', message: 'x'}
        render(
            <OutcomePanel
                outcome={{kind: 'error', retained: false, error}}
                onRetry={onRetry}
                onDismiss={onDismiss}
                busy
            />,
        )

        fireEvent.click(screen.getByRole('button', {name: 'Try Again'}))
        fireEvent.click(screen.getByRole('button', {name: 'Dismiss'}))

        expect(onRetry).not.toHaveBeenCalled()
        expect(onDismiss).not.toHaveBeenCalled()
    })

    it('disables the Send Another browse trigger while busy, on a Done card', () => {
        const {onSelectFile, onSelectDirectory} = browseHandlers()
        render(
            <OutcomePanel
                outcome={doneOutcome(false)}
                browse={{label: 'Send Another', onSelectFile, onSelectDirectory}}
                busy
            />,
        )

        const trigger = screen.getByRole('button', {name: 'Send Another'})
        expect(trigger.getAttribute('aria-disabled')).toBe('true')
        fireEvent.click(trigger)
        expect(screen.queryByRole('menu')).toBeNull()
    })

    it('never changes Try Again\'s label to anything, let alone an ellipsis, while busy', () => {
        const error: PublicError = {code: 'transfer_failed', message: 'x'}
        render(
            <OutcomePanel
                outcome={{kind: 'error', retained: false, error}}
                onRetry={vi.fn()}
                onDismiss={vi.fn()}
                busy
            />,
        )

        expect(screen.getByRole('button', {name: 'Try Again'})).toBeTruthy()
        expect(screen.queryByText(/…|\.\.\./)).toBeNull()
    })

    it('re-enables every control once busy clears', () => {
        const onRetry = vi.fn()
        const onDismiss = vi.fn()
        const error: PublicError = {code: 'transfer_failed', message: 'x'}
        const {rerender} = render(
            <OutcomePanel
                outcome={{kind: 'error', retained: false, error}}
                onRetry={onRetry}
                onDismiss={onDismiss}
                busy
            />,
        )
        expect(screen.getByRole('button', {name: 'Try Again'}).getAttribute('aria-disabled')).toBe('true')

        rerender(
            <OutcomePanel
                outcome={{kind: 'error', retained: false, error}}
                onRetry={onRetry}
                onDismiss={onDismiss}
                busy={false}
            />,
        )

        const tryAgain = screen.getByRole('button', {name: 'Try Again'})
        expect(tryAgain.hasAttribute('aria-disabled')).toBe(false)
        fireEvent.click(tryAgain)
        expect(onRetry).toHaveBeenCalledTimes(1)
    })
})

describe('heading rank and phase ownership', () => {
    it('owns the document heading when it is the whole phase view', () => {
        render(<OutcomePanel outcome={doneOutcome(false)} level={1} phaseView/>)

        expect(screen.getByRole('heading', {level: 1}).textContent).toBe('Sent')
        expect(panel().getAttribute('data-phase-view')).toBe('outcome')
    })

    it('keeps the heading rank but gives up the phase once it is retained status', () => {
        // Rank and phase are separate props because reset changes only one of
        // them: the user must be looking at the same node, at the same weight,
        // while Idle becomes the phase view underneath it.
        render(<OutcomePanel outcome={doneOutcome(true)} level={1} onDismiss={vi.fn()}/>)

        expect(screen.getByRole('heading', {level: 1}).textContent).toBe('Sent')
        expect(panel().hasAttribute('data-phase-view')).toBe(false)
    })

    it('defers to the surrounding view when it is a nested command failure', () => {
        const error: PublicError = {code: 'busy', message: 'ignored'}
        render(<OutcomePanel outcome={{kind: 'error', retained: false, error}} focusTarget="command-error"/>)

        expect(screen.getByRole('heading', {level: 2}).textContent).toBe('FairDrop is still busy')
        expect(panel().hasAttribute('data-phase-view')).toBe(false)
        expect(panel().getAttribute('data-focus-target')).toBe('command-error')
    })
})

describe('the focused container has a name', () => {
    /*
      Focus lands on this section, and a container is not named by a heading
      inside it unless it says so. Without this the terminal outcome -- the row
      the whole routing table exists to deliver -- is announced as a nameless
      region, which is the same "browsers disagree" problem that took the
      role="textbox" div out of StagedView.
    */
    it.each([
        ['done', doneOutcome(false), 'Sent'],
        ['error', {kind: 'error', retained: false, error: {code: 'transfer_failed', message: 'x'} as PublicError} as const,
            'Transfer stopped'],
    ])('names the %s panel with its own heading', (_name, outcome, heading) => {
        render(<OutcomePanel outcome={outcome} focusTarget="outcome"/>)
        const panel = document.querySelector('[data-focus-target="outcome"]')!
        const labelledBy = panel.getAttribute('aria-labelledby')

        expect(labelledBy).toBeTruthy()
        expect(document.getElementById(labelledBy!)?.textContent).toBe(heading)
    })
})

describe('focus surface', () => {
    it('is reachable by a programmatic focus move without joining the Tab order', () => {
        render(<OutcomePanel outcome={doneOutcome(false)}/>)

        // Story 1.10 routes focus; this story only guarantees the target exists.
        expect(panel().getAttribute('tabindex')).toBe('-1')
    })

    it('never paints its state with color alone', () => {
        render(<OutcomePanel outcome={doneOutcome(false)}/>)
        const icon = document.querySelector('.fd-outcome__icon')

        expect(icon?.getAttribute('aria-hidden')).toBe('true')
        expect(icon?.querySelector('svg.fd-outcome__check')).toBeTruthy()
        // The heading text is the real cue; the glyph only reinforces it.
        expect(screen.getByRole('heading').textContent).toBe('Sent')
    })

    it('marks the error state with the literal "!" glyph, not color alone', () => {
        const error: PublicError = {code: 'transfer_failed', message: 'x'}
        render(<OutcomePanel outcome={{kind: 'error', retained: false, error}}/>)
        const icon = document.querySelector('.fd-outcome__icon')

        expect(icon?.textContent).toBe('!')
        expect(icon?.getAttribute('aria-hidden')).toBe('true')
        expect(icon?.querySelector('svg')).toBeNull()
    })

    it('draws the check stroke once after mount, unconditionally', async () => {
        render(<OutcomePanel outcome={doneOutcome(false)}/>)
        const check = document.querySelector('svg.fd-outcome__check')

        // React Testing Library flushes mount effects inside act() before
        // render() returns, so the drawn modifier is already applied here --
        // this is what "unconditional" is: nothing gates it on a media query
        // or a later interaction the reduced-motion mutation could hide behind.
        expect(check?.classList.contains('fd-outcome__check--drawn')).toBe(true)
    })

    // Every path that shows this panel is a focus-owned row of the routing
    // table, and the spine allows an alert only on a path that does not also
    // move focus -- so role="alert" is refused in every shape this component
    // can take, not merely the default one.
    it.each([
        ['a live Done', doneOutcome(false)],
        ['a retained Done', doneOutcome(true)],
        ['a live Error', {kind: 'error', retained: false, error: {code: 'transfer_failed', message: 'x'} as PublicError} as const],
        ['a retained Error', {kind: 'error', retained: true, error: {code: 'transfer_failed', message: 'x'} as PublicError} as const],
    ])('never carries role="alert", for %s', (_name, outcome) => {
        render(<OutcomePanel outcome={outcome} onDismiss={vi.fn()}/>)

        expect(document.querySelector('[role="alert"]')).toBeNull()
        expect(panel().getAttribute('role')).toBeNull()
    })
})

/*
  Story 9.6 replaces the two-cell grid receipt with one line: a kind glyph,
  the name, and the wire bytes actually sent (Done); the name alone (Error,
  when Story 9.2 retained one). The values still come from the same
  `CompletionReceipt`/`itemName` Story 7.4/9.2 built -- this only confirms the
  panel renders them in the new shape.
*/
describe('the outcome receipt (Story 9.6)', () => {
    it('renders a kind glyph, the item name, and the wire bytes sent for Done', () => {
        render(<OutcomePanel outcome={doneOutcome(false)}/>)

        const receipt = document.querySelector('.fd-outcome__receipt')
        expect(receipt).toBeTruthy()
        expect(receipt?.querySelector('svg.fd-outcome__receipt-glyph')).toBeTruthy()
        expect(document.querySelector('.fd-outcome__receipt-name')?.textContent).toBe('report.pdf')
        // The meta span holds the separator and the figure together (one
        // element, so the crossfade/layout has one thing to measure), so this
        // checks its combined text rather than an exact getByText match.
        expect(document.querySelector('.fd-outcome__receipt-meta')?.textContent).toContain('100 bytes')
    })

    it('carries the same receipt once the outcome is retained in Idle', () => {
        render(<OutcomePanel outcome={doneOutcome(true)} onDismiss={vi.fn()}/>)

        expect(document.querySelector('.fd-outcome__receipt-name')?.textContent).toBe('report.pdf')
        expect(document.querySelector('.fd-outcome__receipt-meta')?.textContent).toContain('100 bytes')
    })

    it('formats a directory receipt\'s byte count the same as a file\'s', () => {
        const dirReceipt: CompletionReceipt = {name: 'papers', isDir: true, bytesSent: 4_096}
        render(<OutcomePanel outcome={{kind: 'done', retained: false, receipt: dirReceipt}}/>)

        expect(document.querySelector('.fd-outcome__receipt-name')?.textContent).toBe('papers')
        expect(document.querySelector('.fd-outcome__receipt-meta')?.textContent).toContain('4.1 KB')
    })

    // The mutation this guards: inventing a duration is the worst outcome
    // this story could produce. One receipt line only, never a duration figure.
    it('never renders an elapsed-time figure', () => {
        render(<OutcomePanel outcome={doneOutcome(false)}/>)

        for (const forbidden of ['duration', 'elapsed', 'Elapsed', 'Duration']) {
            expect(panel().textContent, forbidden).not.toContain(forbidden)
        }
    })

    it('renders the item name alone for an Error outcome that retained one -- no glyph, no byte count', () => {
        const error: PublicError = {code: 'transfer_failed', message: 'x'}
        render(<OutcomePanel outcome={{kind: 'error', retained: false, error, itemName: 'report.pdf'}}/>)

        const receipt = document.querySelector('.fd-outcome__receipt')
        expect(receipt).toBeTruthy()
        expect(document.querySelector('.fd-outcome__receipt-name')?.textContent).toBe('report.pdf')
        expect(receipt?.querySelector('svg')).toBeNull()
        expect(receipt?.textContent?.includes('bytes')).toBe(false)
    })

    // A Stage-time command failure never retained a name (Story 9.2), so no
    // receipt line renders for one at all.
    it('renders no receipt for an Error outcome with no retained item name', () => {
        const error: PublicError = {code: 'transfer_failed', message: 'x'}
        render(<OutcomePanel outcome={{kind: 'error', retained: false, error}}/>)

        expect(document.querySelector('.fd-outcome__receipt')).toBeNull()
    })
})
