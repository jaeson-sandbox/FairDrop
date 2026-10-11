import {useState} from 'react'
import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {afterEach, describe, expect, it, vi} from 'vitest'
import {CollectionDraftView} from './CollectionDraftView'

afterEach(cleanup)

function Harness({initial}: {initial: string[]}) {
    const [rows, setRows] = useState(() => initial.map((name, id) => ({id, name})))
    return <CollectionDraftView names={rows.map(row => row.name)} ids={rows.map(row => row.id)} error={null} dropTargetStyle={{}}
        onAddFiles={vi.fn()} onAddFolder={vi.fn()}
        onRemove={index => setRows(current => current.filter((_, i) => i !== index))}
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

    it('keeps the remaining rows mounted when the first row is removed', () => {
        render(<Harness initial={Array.from({length: 200}, (_, i) => `photo-${i}.jpg`)}/> )
        const before = Array.from(document.querySelectorAll('.fd-draft__row'))
        expect(before).toHaveLength(200)
        fireEvent.click(screen.getByRole('button', {name: 'Remove 1. photo-0.jpg'}))
        const after = Array.from(document.querySelectorAll('.fd-draft__row'))
        expect(after).toHaveLength(199)
        // Same element identity, not equal-looking replacements: a remount of
        // every later row is what made removing from a 10,000-row list slow.
        after.forEach((row, position) => expect(row).toBe(before[position + 1]))
        expect(after[0]?.textContent).toContain('1.')
        expect(after[0]?.textContent).toContain('photo-1.jpg')
        expect(screen.getByRole('button', {name: 'Remove 199. photo-199.jpg'})).toBeTruthy()
    })

    it('re-renders only the new row when a row is appended to an unchanged list', () => {
        const names = Array.from({length: 300}, (_, i) => `photo-${i}.jpg`)
        const ids = names.map((_, i) => i)
        const noop = vi.fn()
        const view = (rows: string[], keys: number[]) => (
            <CollectionDraftView names={rows} ids={keys} error={null} dropTargetStyle={{}}
                onAddFiles={noop} onAddFolder={noop} onRemove={noop} onSend={noop} onCancel={noop}/>
        )
        const {rerender} = render(view(names, ids))
        // A row that renders again re-attaches its Remove button ref (the ref
        // callback is a fresh function each render), which registers the button.
        // Count registrations: one means only the new row rendered.
        let registrations = 0
        const originalSet = Map.prototype.set
        const spy = vi.spyOn(Map.prototype, 'set').mockImplementation(function (this: Map<unknown, unknown>, key: unknown, value: unknown) {
            if (value instanceof HTMLButtonElement) registrations += 1
            return originalSet.call(this, key, value)
        })
        try {
            rerender(view([...names, 'photo-300.jpg'], [...ids, 300]))
        } finally {
            spy.mockRestore()
        }
        expect(document.querySelectorAll('.fd-draft__row')).toHaveLength(301)
        expect(registrations).toBe(1)
    })

    it('sizes the ordinal column to the digit width of the count, never below two', () => {
        const {unmount} = render(<Harness initial={['one.txt', 'two.txt', 'three.txt']}/> )
        expect(document.querySelector<HTMLElement>('.fd-draft__list')?.style.getPropertyValue('--fd-ordinal-digits')).toBe('2')
        unmount()
        const hundred = render(<Harness initial={Array.from({length: 100}, (_, i) => `f-${i}`)}/> )
        expect(document.querySelector<HTMLElement>('.fd-draft__list')?.style.getPropertyValue('--fd-ordinal-digits')).toBe('3')
        hundred.unmount()
        render(<Harness initial={Array.from({length: 10000}, (_, i) => `f-${i}`)}/> )
        expect(document.querySelector<HTMLElement>('.fd-draft__list')?.style.getPropertyValue('--fd-ordinal-digits')).toBe('5')
    })

    it('moves to Add Files when the last row is removed', async () => {
        render(<Harness initial={['one.txt']}/> )
        fireEvent.click(screen.getByRole('button', {name: 'Remove 1. one.txt'}))
        await vi.waitFor(() => expect(document.activeElement).toBe(screen.getByRole('button', {name: 'Add Files'})))
        expect(screen.getByRole('button', {name: 'Send'}).hasAttribute('disabled')).toBe(true)
    })
})
