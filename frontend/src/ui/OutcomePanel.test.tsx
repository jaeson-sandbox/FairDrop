import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {afterEach, describe, expect, it, vi} from 'vitest'
import type {PublicError} from '../transfer/types'
import {OutcomePanel} from './OutcomePanel'

afterEach(cleanup)

function panel(): HTMLElement {
    const found = document.querySelector('.fd-outcome')
    expect(found, 'an outcome panel').toBeTruthy()
    return found as HTMLElement
}

describe('the Done panel', () => {
    it('says only that FairDrop finished sending', () => {
        render(<OutcomePanel outcome={{kind: 'done', retained: false}}/>)

        expect(screen.getByRole('heading').textContent).toBe('Transfer finished')
        expect(screen.getByText('FairDrop finished sending the item.')).toBeTruthy()
        expect(panel().getAttribute('data-outcome')).toBe('done')
    })

    it('claims nothing about the receiver, its storage, or the download', () => {
        render(<OutcomePanel outcome={{kind: 'done', retained: false}}/>)

        const text = panel().textContent ?? ''
        for (const claim of ['saved', 'received', 'downloaded', 'stored', 'Files']) {
            expect(text, claim).not.toContain(claim)
        }
    })

    it('carries no Dismiss control while the session is still the current phase', () => {
        render(<OutcomePanel outcome={{kind: 'done', retained: false}} onDismiss={vi.fn()}/>)

        expect(screen.queryByRole('button')).toBeNull()
    })
})

describe('the retained outcome node', () => {
    it('keeps the same visible content and adds Dismiss', () => {
        const onDismiss = vi.fn()
        render(<OutcomePanel outcome={{kind: 'done', retained: true}} onDismiss={onDismiss}/>)

        expect(screen.getByRole('heading').textContent).toBe('Transfer finished')
        const dismiss = screen.getByRole('button', {name: 'Dismiss'})
        expect(dismiss.className).toContain('fd-target')

        fireEvent.click(dismiss)
        expect(onDismiss).toHaveBeenCalledTimes(1)
    })

    it('marks itself retained so a reader can tell status from session', () => {
        render(<OutcomePanel outcome={{kind: 'done', retained: true}} onDismiss={vi.fn()}/>)

        expect(panel().getAttribute('data-retained')).toBe('true')
    })
})

describe('the Error panel', () => {
    it.each([
        ['invalid_selection', 'Choose one item', 'Choose exactly one file or folder.'],
        ['busy', 'Transfer already active', 'Finish or cancel the current transfer before choosing another item.'],
        ['path_not_found', 'Item not found', 'That file or folder is no longer available. Choose it again.'],
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
        render(<OutcomePanel outcome={{kind: 'done', retained: false}} level={1} phaseView/>)

        expect(screen.getByRole('heading', {level: 1}).textContent).toBe('Transfer finished')
        expect(panel().getAttribute('data-phase-view')).toBe('outcome')
    })

    it('keeps the heading rank but gives up the phase once it is retained status', () => {
        // Rank and phase are separate props because reset changes only one of
        // them: the user must be looking at the same node, at the same weight,
        // while Idle becomes the phase view underneath it.
        render(<OutcomePanel outcome={{kind: 'done', retained: true}} level={1} onDismiss={vi.fn()}/>)

        expect(screen.getByRole('heading', {level: 1}).textContent).toBe('Transfer finished')
        expect(panel().hasAttribute('data-phase-view')).toBe(false)
    })

    it('defers to the surrounding view when it is a nested command failure', () => {
        const error: PublicError = {code: 'busy', message: 'ignored'}
        render(<OutcomePanel outcome={{kind: 'error', retained: false, error}} focusTarget="command-error"/>)

        expect(screen.getByRole('heading', {level: 2}).textContent).toBe('Transfer already active')
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
        ['done', {kind: 'done', retained: false} as const, 'Transfer finished'],
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
        render(<OutcomePanel outcome={{kind: 'done', retained: false}}/>)

        // Story 1.10 routes focus; this story only guarantees the target exists.
        expect(panel().getAttribute('tabindex')).toBe('-1')
    })

    it('never paints its state with color alone', () => {
        render(<OutcomePanel outcome={{kind: 'done', retained: false}}/>)
        const icon = document.querySelector('.fd-outcome__icon')

        expect(icon?.textContent).toBe('✓')
        expect(icon?.getAttribute('aria-hidden')).toBe('true')
        // The heading text is the real cue; the glyph only reinforces it.
        expect(screen.getByRole('heading').textContent).toBe('Transfer finished')
    })
})
