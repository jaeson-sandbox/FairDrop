import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {afterEach, describe, expect, it, vi} from 'vitest'
import type {ReceiveOutcomeTransferState} from '../transfer/state'
import type {ReceiveResult} from '../transfer/types'
import {ReceiveOutcomeView} from './ReceiveOutcomeView'

afterEach(cleanup)

const sessionId = '0102030405060708090a0b0c0d0e0f10'

function outcome(
    kind: ReceiveResult,
    overrides: Partial<ReceiveOutcomeTransferState['outcome']> = {},
    destination = 'Phone Photos',
    commandError: ReceiveOutcomeTransferState['commandError'] = null,
): ReceiveOutcomeTransferState {
    return {
        phase: 'receive-outcome',
        session: {sessionId, lastSeq: 4},
        destination,
        outcome: {kind, filesSaved: 3, subfolderExists: true, markingWarning: false, ...overrides},
        commandError,
    }
}

function renderOutcome(state: ReceiveOutcomeTransferState, handlers: {show?: () => void; done?: () => void} = {}) {
    return render(<ReceiveOutcomeView state={state} onShowInFolder={handlers.show ?? vi.fn()} onDone={handlers.done ?? vi.fn()}/>)
}

// Folder names are wrapped in first-strong isolates inside a sentence; strip them to compare the words.
function text(element: HTMLElement | null): string {
    return (element?.textContent ?? '').replace(/[⁨⁩]/g, '')
}

const heading = () => screen.getByRole('heading', {level: 1})
const body = () => document.getElementById('fd-receive-outcome-body')

describe('a complete receive', () => {
    it('says how many files were saved, as the heading', () => {
        renderOutcome(outcome('complete'))
        expect(heading().textContent).toBe('3 files saved')
    })

    it('says "1 file saved" for one and never "1 files"', () => {
        renderOutcome(outcome('complete', {filesSaved: 1}))
        expect(heading().textContent).toBe('1 file saved')
    })

    it('says where, in words that claim only that they are on this computer', () => {
        renderOutcome(outcome('complete'))
        expect(text(body())).toBe('Saved to this computer, in a new folder inside Phone Photos.')
    })

    it('shows the success disc and check, in the done styling', () => {
        const {container} = renderOutcome(outcome('complete'))

        const card = container.querySelector('.fd-outcome') as HTMLElement
        expect(card.className).toContain('fd-outcome--done')
        expect(card.querySelector('.fd-outcome__check')).toBeTruthy()
        expect(card.getAttribute('data-receive-outcome')).toBe('complete')
    })

    it('says no files saved rather than "0 files saved" if the backend reports a complete with none (defensive)', () => {
        renderOutcome(outcome('complete', {filesSaved: 0, subfolderExists: false}))
        expect(heading().textContent).toBe('No files saved')
    })
})

describe('an incomplete receive is never shown as a success', () => {
    it('heads it "Upload incomplete" with the exact count and no success wording', () => {
        const {container} = renderOutcome(outcome('incomplete', {filesSaved: 2}))

        expect(heading().textContent).toBe('Upload incomplete — 2 files saved')
        expect(heading().textContent).not.toBe('2 files saved')
        const card = container.querySelector('.fd-outcome') as HTMLElement
        expect(card.className).toContain('fd-outcome--error')
        expect(card.className).not.toContain('fd-outcome--done')
        expect(card.querySelector('.fd-outcome__check')).toBeNull()
        expect(card.textContent).toContain('!')
        expect(card.getAttribute('data-receive-outcome')).toBe('incomplete')
    })

    it('says "1 file saved" for one', () => {
        renderOutcome(outcome('incomplete', {filesSaved: 1}))
        expect(heading().textContent).toBe('Upload incomplete — 1 file saved')
    })

    it('explains that finished files were kept and the one in progress was removed', () => {
        renderOutcome(outcome('incomplete', {filesSaved: 2}))
        expect(text(body())).toBe(
            'The upload stopped early. Files that finished are saved to this computer, in a new folder inside Phone Photos. The file in progress was removed.',
        )
    })

    it('says no files saved, and that nothing was added, when none finished', () => {
        renderOutcome(outcome('incomplete', {filesSaved: 0, subfolderExists: false}))

        expect(heading().textContent).toBe('Upload incomplete — no files saved')
        expect(text(body())).toBe('The upload stopped before any file finished saving. Nothing was added to Phone Photos.')
    })
})

describe('a cancelled receive', () => {
    it('heads it "Upload cancelled" with the exact count', () => {
        renderOutcome(outcome('cancelled', {filesSaved: 1}))

        expect(heading().textContent).toBe('Upload cancelled — 1 file saved')
        expect(text(body())).toBe(
            'You cancelled the upload. Files that finished are saved to this computer, in a new folder inside Phone Photos. The file in progress was removed.',
        )
        expect(document.querySelector('.fd-outcome')?.getAttribute('data-receive-outcome')).toBe('cancelled')
    })

    it('says that nothing was added when nothing was saved', () => {
        renderOutcome(outcome('cancelled', {filesSaved: 0, subfolderExists: false}))

        expect(heading().textContent).toBe('Upload cancelled — no files saved')
        expect(text(body())).toBe('You cancelled the upload before any file finished saving. Nothing was added to Phone Photos.')
    })

    it('does not render as an Error panel: it has its own heading, not an error-code one', () => {
        renderOutcome(outcome('cancelled', {filesSaved: 2}))
        expect(document.querySelector('[data-error-code]')).toBeNull()
        expect(screen.queryByText('Transfer canceled')).toBeNull()
    })
})

describe('Show in Folder', () => {
    it.each(['complete', 'incomplete', 'cancelled'] as const)('is offered for a %s outcome whose subfolder exists', (kind) => {
        renderOutcome(outcome(kind, {subfolderExists: true}))
        expect(screen.getByRole('button', {name: 'Show in Folder'})).toBeTruthy()
    })

    it.each(['complete', 'incomplete', 'cancelled'] as const)('is not rendered for a %s outcome with no subfolder', (kind) => {
        renderOutcome(outcome(kind, {filesSaved: 0, subfolderExists: false}))
        expect(screen.queryByRole('button', {name: 'Show in Folder'})).toBeNull()
    })

    it('follows the backend flag, not the count: saved files with no subfolder get no button', () => {
        renderOutcome(outcome('incomplete', {filesSaved: 2, subfolderExists: false}))
        expect(screen.queryByRole('button', {name: 'Show in Folder'})).toBeNull()
    })

    it('follows the backend flag, not the count: an existing subfolder with a zero count still gets the button', () => {
        renderOutcome(outcome('incomplete', {filesSaved: 0, subfolderExists: true}))
        expect(screen.getByRole('button', {name: 'Show in Folder'})).toBeTruthy()
    })

    it('calls onShowInFolder once per press and leaves the outcome as it was', () => {
        const show = vi.fn()
        const done = vi.fn()
        renderOutcome(outcome('complete'), {show, done})

        fireEvent.click(screen.getByRole('button', {name: 'Show in Folder'}))

        expect(show).toHaveBeenCalledTimes(1)
        expect(done).not.toHaveBeenCalled()
        expect(heading().textContent).toBe('3 files saved')
    })

    it('says so beside the outcome, as a focus target, when opening failed', () => {
        renderOutcome(outcome('complete', {}, 'Phone Photos', {code: 'path_not_found', message: 'x'}))

        const note = document.querySelector('[data-receive-show-failed]') as HTMLElement
        expect(note.textContent).toBe('FairDrop couldn’t open that folder. It may have been moved or renamed.')
        expect(note.getAttribute('data-focus-target')).toBe('command-error')
        expect(note.getAttribute('tabindex')).toBe('-1')
        // The outcome itself is untouched.
        expect(heading().textContent).toBe('3 files saved')
    })

    it('shows no failure note when nothing failed', () => {
        renderOutcome(outcome('complete'))
        expect(document.querySelector('[data-receive-show-failed]')).toBeNull()
    })
})

describe('the marking warning', () => {
    it('appears when the backend says marking failed, and says the files were still saved', () => {
        renderOutcome(outcome('complete', {markingWarning: true}))

        const warning = document.querySelector('[data-receive-marking-warning]') as HTMLElement
        expect(warning.textContent).toBe('FairDrop couldn’t add the download mark to some files on this drive. The files were saved without it.')
    })

    it('is absent otherwise', () => {
        renderOutcome(outcome('complete', {markingWarning: false}))
        expect(document.querySelector('[data-receive-marking-warning]')).toBeNull()
    })

    it('is part of the description the one focus move reads', () => {
        renderOutcome(outcome('complete', {markingWarning: true}))

        const card = document.querySelector('.fd-outcome') as HTMLElement
        expect((card.getAttribute('aria-describedby') ?? '').split(' ')).toEqual(['fd-receive-outcome-body', 'fd-receive-outcome-marking'])
    })
})

describe('leaving', () => {
    it('Done calls onDone once and nothing else', () => {
        const show = vi.fn()
        const done = vi.fn()
        renderOutcome(outcome('complete'), {show, done})

        fireEvent.click(screen.getByRole('button', {name: 'Done'}))

        expect(done).toHaveBeenCalledTimes(1)
        expect(show).not.toHaveBeenCalled()
    })

    it('is the primary action when there is no folder to show, and quiet beside Show in Folder', () => {
        const {unmount} = renderOutcome(outcome('complete'))
        expect(screen.getByRole('button', {name: 'Done'}).className).toContain('fd-button--quiet')
        expect(screen.getByRole('button', {name: 'Show in Folder'}).className).toContain('fd-button--primary')
        unmount()

        renderOutcome(outcome('incomplete', {filesSaved: 0, subfolderExists: false}))
        expect(screen.getByRole('button', {name: 'Done'}).className).toContain('fd-button--primary')
    })

    it('never offers a send affordance for a receive outcome', () => {
        renderOutcome(outcome('complete'))

        for (const label of ['Send Again', 'Send Another', 'Try Again', 'Choose Another', 'Dismiss', 'Choose File or Folder']) {
            expect(screen.queryByRole('button', {name: label}), label).toBeNull()
        }
        expect(screen.queryByRole('menu')).toBeNull()
        expect(screen.getAllByRole('button').map((button) => button.textContent)).toEqual(['Show in Folder', 'Done'])
    })
})

describe('the outcome as a focus target', () => {
    it('is the routed receive-outcome target, named by the heading and described by the body', () => {
        renderOutcome(outcome('complete'))

        const card = document.querySelector('.fd-outcome') as HTMLElement
        expect(card.getAttribute('data-focus-target')).toBe('receive-outcome')
        expect(card.getAttribute('tabindex')).toBe('-1')
        expect(card.getAttribute('aria-labelledby')).toBe(heading().id)
        expect(card.getAttribute('data-phase-view')).toBe('receive-outcome')
    })

    it('is not an alert and not a live region: focus is its only announcer', () => {
        renderOutcome(outcome('incomplete'))

        const card = document.querySelector('.fd-outcome') as HTMLElement
        expect(card.getAttribute('role')).toBeNull()
        expect(card.querySelector('[role="alert"], [aria-live]')).toBeNull()
    })
})

describe('what the outcome never contains', () => {
    it('carries no path, token, url or file name, only the destination display name', () => {
        renderOutcome(outcome('complete', {markingWarning: true}))

        const rendered = document.body.textContent ?? ''
        expect(rendered).not.toContain('/upload/')
        expect(rendered).not.toContain('http')
        expect(rendered).not.toMatch(/\/Users\//)
        expect(rendered).toContain('Phone Photos')
    })

    it('isolates a right-to-left folder name inside the sentence', () => {
        renderOutcome(outcome('complete', {}, 'مجلد الصور'))

        expect(body()?.textContent).toBe('Saved to this computer, in a new folder inside ⁨مجلد الصور⁩.')
    })
})


it('offers Receive Again as the primary action only on success and disables all actions while pending', () => {
    const again = vi.fn()
    const view = render(<ReceiveOutcomeView state={outcome('complete')} onReceiveAgain={again}
        onShowInFolder={vi.fn()} onDone={vi.fn()}/>)
    const primary = screen.getByRole('button', {name: 'Receive Again'})
    expect(primary.classList.contains('fd-button--primary')).toBe(true)
    fireEvent.click(primary)
    expect(again).toHaveBeenCalledTimes(1)
    expect(screen.getByRole('button', {name: 'Show in Folder'}).classList.contains('fd-button--secondary')).toBe(true)
    view.rerender(<ReceiveOutcomeView state={outcome('complete')} onReceiveAgain={again} busy
        onShowInFolder={vi.fn()} onDone={vi.fn()}/>)
    for (const button of screen.getAllByRole('button')) expect((button as HTMLButtonElement).disabled).toBe(true)
    view.rerender(<ReceiveOutcomeView state={outcome('incomplete')} onReceiveAgain={again}
        onShowInFolder={vi.fn()} onDone={vi.fn()}/>)
    expect(screen.queryByRole('button', {name: 'Receive Again'})).toBeNull()
})

it('renders truthful fixed retirement error separately from Show in Folder failure', () => {
    const state = {...outcome('complete', {}, 'Photos', {code: 'transfer_failed', message: 'ignored'}), commandErrorPurpose: 'retire' as const}
    renderOutcome(state)
    expect(screen.queryByText('FairDrop couldn’t confirm the previous session closed. Try again or choose Done.')).toBeTruthy()
    expect(screen.queryByText('FairDrop couldn’t open that folder. It may have been moved or renamed.')).toBeNull()
})

it('restores the Show initiating control after disabling it without relying on pointer focus', () => {
    const handlers = {onShowInFolder: vi.fn(), onDone: vi.fn()}
    const view = render(<ReceiveOutcomeView state={outcome('complete')} {...handlers}/>)
    const show = screen.getByRole('button', {name: 'Show in Folder'})
    expect(document.activeElement).not.toBe(show)
    fireEvent.click(show)
    expect(document.activeElement).toBe(show)
    view.rerender(<ReceiveOutcomeView state={outcome('complete')} busy {...handlers}/>)
    document.body.tabIndex = -1
    document.body.focus() // jsdom refuses blur() on a disabled button.
    document.body.removeAttribute('tabindex')
    expect(document.activeElement).toBe(document.body)
    view.rerender(<ReceiveOutcomeView state={outcome('complete')} {...handlers}/>)
    expect(document.activeElement).toBe(show)
    expect(show.hasAttribute('data-focus-return')).toBe(true)
})

it('does not restore Show over a new outcome, error or deliberate focus move', () => {
    for (const change of ['session', 'error', 'focus']) {
        const handlers = {onShowInFolder: vi.fn(), onDone: vi.fn()}
        const view = render(<ReceiveOutcomeView state={outcome('complete')} {...handlers}/>)
        const show = screen.getByRole('button', {name: 'Show in Folder'})
        fireEvent.click(show)
        view.rerender(<ReceiveOutcomeView state={outcome('complete')} busy {...handlers}/>)
        document.body.tabIndex = -1
        document.body.focus()
        document.body.removeAttribute('tabindex')
        const next = change === 'session' ? {...outcome('complete'), session: {sessionId: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', lastSeq: 4}} :
            change === 'error' ? outcome('complete', {}, 'Photos', {code: 'path_not_found', message: 'ignored'}) : outcome('complete')
        const done = screen.getByRole('button', {name: 'Done'})
        if (change === 'focus') { done.removeAttribute('disabled'); done.focus() }
        view.rerender(<ReceiveOutcomeView state={next} {...handlers}/>)
        expect(document.activeElement).not.toBe(show)
        view.unmount()
    }
})
