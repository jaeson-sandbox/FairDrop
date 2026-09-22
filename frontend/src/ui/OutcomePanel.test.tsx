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

describe('the Done panel', () => {
    it('says only that FairDrop finished sending', () => {
        render(<OutcomePanel outcome={doneOutcome(false)}/>)

        expect(screen.getByRole('heading').textContent).toBe('Transfer finished')
        expect(screen.getByText('FairDrop finished sending the item.')).toBeTruthy()
        expect(panel().getAttribute('data-outcome')).toBe('done')
    })

    it('claims nothing about the receiver, its storage, or the download', () => {
        render(<OutcomePanel outcome={doneOutcome(false)}/>)

        const text = panel().textContent ?? ''
        for (const claim of ['saved', 'received', 'downloaded', 'stored', 'Files']) {
            expect(text, claim).not.toContain(claim)
        }
    })

    // Reversed by Story 3.6. A live Done or Error used to render no control
    // even when a handler was supplied, because the way out was the backend's
    // three-second reset. The coordinator drops an event it cannot deliver, so
    // a reset that never arrives left the window with nothing to press
    // (D-059). The expectation moved because the contract did.
    it('carries the control a live terminal outcome is given, so a lost reset cannot strand it', () => {
        const dismiss = vi.fn()
        render(<OutcomePanel outcome={doneOutcome(false)} onDismiss={dismiss}/>)

        const control = screen.getByRole('button')
        fireEvent.click(control)
        expect(dismiss).toHaveBeenCalledTimes(1)
    })

    // A live terminal outcome has nothing else on screen to be a next action --
    // its one control has to carry that weight itself, so it takes the
    // primary style rather than the quiet one the retained node uses below.
    it('gives the live control primary weight, not the quiet retained styling', () => {
        render(<OutcomePanel outcome={doneOutcome(false)} onDismiss={vi.fn()}/>)

        const control = screen.getByRole('button')
        expect(control.className).toContain('fd-button--primary')
        expect(control.className).not.toContain('fd-button--quiet')
    })

    it('offers no control when the caller supplies no handler', () => {
        render(<OutcomePanel outcome={doneOutcome(false)}/>)

        expect(screen.queryByRole('button')).toBeNull()
    })
})

describe('the retained outcome node', () => {
    it('keeps the same visible content and adds Dismiss', () => {
        const onDismiss = vi.fn()
        render(<OutcomePanel outcome={doneOutcome(true)} onDismiss={onDismiss}/>)

        expect(screen.getByRole('heading').textContent).toBe('Transfer finished')
        const dismiss = screen.getByRole('button', {name: 'Dismiss'})
        expect(dismiss.className).toContain('fd-target')
        // Idle's own browse control is the next action once retained; this
        // one only needs to be the quiet Dismiss beside it.
        expect(dismiss.className).toContain('fd-button--quiet')
        expect(dismiss.className).not.toContain('fd-button--primary')

        fireEvent.click(dismiss)
        expect(onDismiss).toHaveBeenCalledTimes(1)
    })

    it('marks itself retained so a reader can tell status from session', () => {
        render(<OutcomePanel outcome={doneOutcome(true)} onDismiss={vi.fn()}/>)

        expect(panel().getAttribute('data-retained')).toBe('true')
    })
})

describe('the Error panel', () => {
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
})

describe('heading rank and phase ownership', () => {
    it('owns the document heading when it is the whole phase view', () => {
        render(<OutcomePanel outcome={doneOutcome(false)} level={1} phaseView/>)

        expect(screen.getByRole('heading', {level: 1}).textContent).toBe('Transfer finished')
        expect(panel().getAttribute('data-phase-view')).toBe('outcome')
    })

    it('keeps the heading rank but gives up the phase once it is retained status', () => {
        // Rank and phase are separate props because reset changes only one of
        // them: the user must be looking at the same node, at the same weight,
        // while Idle becomes the phase view underneath it.
        render(<OutcomePanel outcome={doneOutcome(true)} level={1} onDismiss={vi.fn()}/>)

        expect(screen.getByRole('heading', {level: 1}).textContent).toBe('Transfer finished')
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
        ['done', doneOutcome(false), 'Transfer finished'],
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
        expect(screen.getByRole('heading').textContent).toBe('Transfer finished')
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
  Story 7.4 wired the retained receipt values through as non-visible
  attributes; Story 7.5 replaces that proof with the real receipt UI these
  tests check instead. The values still come from the same `CompletionReceipt`
  Story 7.4 built (state.test.ts and selectors.test.ts already prove
  `bytesSent` is the wire count, never `metadata.size`) -- this only confirms
  the panel renders it.
*/
describe('the completion receipt (Story 7.5)', () => {
    it('renders exactly two cells: the item name and the wire bytes sent', () => {
        render(<OutcomePanel outcome={doneOutcome(false)}/>)

        const cells = document.querySelectorAll('.fd-receipt__cell')
        expect(cells).toHaveLength(2)
        expect(document.querySelector('.fd-receipt bdi')?.textContent).toBe('report.pdf')
        expect(screen.getByText('100 bytes')).toBeTruthy()
        expect(screen.getByText('Wire bytes')).toBeTruthy()
    })

    it('carries the same receipt once the outcome is retained in Idle', () => {
        render(<OutcomePanel outcome={doneOutcome(true)} onDismiss={vi.fn()}/>)

        expect(document.querySelector('.fd-receipt bdi')?.textContent).toBe('report.pdf')
        expect(screen.getByText('100 bytes')).toBeTruthy()
    })

    it('names a directory receipt as a folder and formats its byte count', () => {
        const dirReceipt: CompletionReceipt = {name: 'papers', isDir: true, bytesSent: 4_096}
        render(<OutcomePanel outcome={{kind: 'done', retained: false, receipt: dirReceipt}}/>)

        expect(document.querySelector('.fd-receipt bdi')?.textContent).toBe('papers')
        expect(screen.getByText('Folder')).toBeTruthy()
        expect(screen.getByText('4.1 KB')).toBeTruthy()
    })

    // The mutation this guards: inventing a duration is the worst outcome
    // this story could produce. Two cells only, never a third.
    it('never renders a third cell or any elapsed-time figure', () => {
        render(<OutcomePanel outcome={doneOutcome(false)}/>)

        expect(document.querySelectorAll('.fd-receipt__cell')).toHaveLength(2)
        for (const forbidden of ['duration', 'elapsed', 'Elapsed', 'Duration']) {
            expect(panel().textContent, forbidden).not.toContain(forbidden)
        }
    })

    it('does not render a receipt for an Error outcome', () => {
        const error: PublicError = {code: 'transfer_failed', message: 'x'}
        render(<OutcomePanel outcome={{kind: 'error', retained: false, error}}/>)

        expect(document.querySelector('.fd-receipt')).toBeNull()
    })
})
