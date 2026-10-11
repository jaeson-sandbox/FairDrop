import {useState} from 'react'
import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {afterEach, describe, expect, it, vi} from 'vitest'
import {CollectionDraftView} from './CollectionDraftView'

afterEach(cleanup)

function Harness({initial}: {initial: string[]}) {
    const [names, setNames] = useState(initial)
    return <CollectionDraftView names={names} error={null} dropTargetStyle={{}}
        onAddFiles={vi.fn()} onAddFolder={vi.fn()}
        onRemove={index => setNames(current => current.filter((_, i) => i !== index))}
        onSend={vi.fn()} onCancel={vi.fn()}/>
}

describe('collection draft focus', () => {
    it('shows explicit one-based order for identical basenames', () => {
        render(<Harness initial={['same.txt', 'same.txt']}/> )
        const rows = document.querySelectorAll('.fd-draft__row')
        expect(rows[0]?.textContent).toContain('1.')
        expect(rows[1]?.textContent).toContain('2.')
        expect(screen.getByRole('button', {name: 'Remove 1. same.txt'})).toBeTruthy()
        expect(screen.getByRole('button', {name: 'Remove 2. same.txt'})).toBeTruthy()
    })

    it('shows the count out of 10,000, grouped for large lists', () => {
        const {unmount} = render(<Harness initial={['one.txt', 'two.txt']}/> )
        expect(screen.getByText('2 of 10,000 items')).toBeTruthy()
        unmount()
        render(<Harness initial={Array.from({length: 1234}, (_, i) => `file-${i}.txt`)}/> )
        expect(screen.getByText('1,234 of 10,000 items')).toBeTruthy()
        expect(screen.getByRole('button', {name: 'Remove 1234. file-1233.txt'})).toBeTruthy()
    })

    it('describes a newly focused draft heading with its restored refusal', () => {
        render(<CollectionDraftView names={['one.txt']} error={{code: 'invalid_selection', message: 'Choose 1 to 10,000 separate files or folders.'}}
            dropTargetStyle={{}} onAddFiles={vi.fn()} onAddFolder={vi.fn()} onRemove={vi.fn()}
            onSend={vi.fn()} onCancel={vi.fn()}/> )
        const heading = screen.getByRole('heading', {name: 'Selected items'})
        const error = screen.getByText('Choose 1 to 10,000 separate files or folders.')
        expect(document.activeElement).toBe(heading)
        expect(heading.getAttribute('aria-describedby')).toBe(error.id)
    })
    it('moves from a removed row to the adjacent remaining Remove control', async () => {
        render(<Harness initial={['one.txt', 'two.txt', 'three.txt']}/> )
        const remove = screen.getByRole('button', {name: 'Remove 2. two.txt'})
        remove.focus()
        fireEvent.click(remove)
        await vi.waitFor(() => expect(document.activeElement).toBe(screen.getByRole('button', {name: 'Remove 2. three.txt'})))
    })

    it('moves to Add Files when the last row is removed', async () => {
        render(<Harness initial={['one.txt']}/> )
        fireEvent.click(screen.getByRole('button', {name: 'Remove 1. one.txt'}))
        await vi.waitFor(() => expect(document.activeElement).toBe(screen.getByRole('button', {name: 'Add Files'})))
        expect(screen.getByRole('button', {name: 'Send'}).hasAttribute('disabled')).toBe(true)
    })
})
