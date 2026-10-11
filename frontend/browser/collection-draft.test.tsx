import {cleanup, render, screen} from '@testing-library/react'
import type {CSSProperties} from 'react'
import {cdp, page, userEvent} from 'vitest/browser'
import {afterEach, describe, expect, it, vi} from 'vitest'
import {CollectionDraftView} from '../src/ui/CollectionDraftView'
import '../src/style.css'

afterEach(async () => {
    cleanup()
    await page.viewport(1024, 800)
    await cdp().send('Emulation.setEmulatedMedia', {features: []})
})

function show(names = ['report.txt', 'photos']) {
    const onRemove = vi.fn()
    const view = render(<CollectionDraftView names={names} error={null}
        dropTargetStyle={{'--wails-drop-target': 'drop'} as CSSProperties}
        onAddFiles={vi.fn()} onAddFolder={vi.fn()} onRemove={onRemove}
        onSend={vi.fn()} onCancel={vi.fn()}/>)
    return {view, onRemove}
}

describe('rendered collection controls', () => {
    it('exposes ordered basename rows, keyboard buttons and adjacent removal', async () => {
        const {onRemove} = show()
        expect(screen.getByRole('heading', {name: 'Selected items'})).toBeTruthy()
        expect(screen.getByText('2 of 10,000 items')).toBeTruthy()
        expect(screen.getByRole('button', {name: 'Remove 1. report.txt'})).toBeTruthy()
        const ordinal = document.querySelector<HTMLElement>('.fd-draft__ordinal')
        expect(ordinal?.textContent).toBe('1.')
        expect(ordinal?.getBoundingClientRect().width).toBeGreaterThan(0)
        await userEvent.tab()
        expect(document.activeElement?.textContent).toContain('Remove')
        await userEvent.keyboard('{Enter}')
        expect(onRemove).toHaveBeenCalledWith(0)
    })

    it('scrolls a 10,000-row list inside the window and keeps keyboard focus visible and in view', async () => {
        const names = Array.from({length: 10000}, (_, i) => `photo-${i + 1}.jpg`)
        show(names)
        expect(screen.getByText('10,000 of 10,000 items')).toBeTruthy()
        const list = document.querySelector<HTMLElement>('.fd-draft__list')!
        const root = document.documentElement
        // The list is a bounded scroller: it overflows itself, not the page.
        expect(getComputedStyle(list).overflowY).toBe('auto')
        expect(list.scrollHeight).toBeGreaterThan(list.clientHeight)
        expect(list.clientHeight).toBeLessThanOrEqual(window.innerHeight * 0.5 + 1)
        expect(root.scrollHeight).toBeLessThanOrEqual(window.innerHeight * 3)
        // The actions stay reachable without scrolling past 10,000 rows.
        const send = screen.getByRole('button', {name: 'Send'})
        expect(send.getBoundingClientRect().bottom).toBeLessThanOrEqual(window.innerHeight * 2)
        // Tabbing into the list scrolls a far row into view with a visible indicator.
        const far = screen.getByRole('button', {name: 'Remove 9000. photo-9000.jpg'})
        far.focus()
        const listBox = list.getBoundingClientRect()
        const farBox = far.getBoundingClientRect()
        expect(farBox.top).toBeGreaterThanOrEqual(listBox.top - 1)
        expect(farBox.bottom).toBeLessThanOrEqual(listBox.bottom + 1)
        expect(getComputedStyle(far).outlineStyle).toBe('solid')
        expect(parseFloat(getComputedStyle(far).outlineWidth)).toBeGreaterThanOrEqual(2)
    })

    it('never lets a wide ordinal touch the name at 10,000 rows', async () => {
        const names = Array.from({length: 10000}, (_, i) => `photo-${i + 1}.jpg`)
        show(names)
        const list = document.querySelector<HTMLElement>('.fd-draft__list')!
        const rows = list.querySelectorAll<HTMLElement>('.fd-draft__row')
        expect(rows).toHaveLength(10000)
        for (const ordinalNumber of [1, 100, 1000, 10000]) {
            const row = rows[ordinalNumber - 1]!
            row.scrollIntoView({block: 'center'})
            const ordinal = row.querySelector<HTMLElement>('.fd-draft__ordinal')!
            const name = row.querySelector<HTMLElement>('.fd-draft__name')!
            expect(ordinal.textContent).toBe(`${ordinalNumber}.`)
            // The ordinal's text itself, not just its box, must end before the name begins.
            const text = document.createRange()
            text.selectNodeContents(ordinal)
            const textRight = text.getBoundingClientRect().right
            const gap = name.getBoundingClientRect().left - textRight
            expect(gap, `row ${ordinalNumber}: ordinal text right ${textRight} vs name left ${name.getBoundingClientRect().left}`).toBeGreaterThanOrEqual(8)
            expect(ordinal.getBoundingClientRect().right).toBeLessThanOrEqual(name.getBoundingClientRect().left)
            expect(ordinal.scrollWidth).toBeLessThanOrEqual(ordinal.clientWidth)
        }
        // Every row's name starts at the same x, so the column reads as a column.
        const lefts = [1, 100, 10000].map(n => rows[n - 1]!.querySelector<HTMLElement>('.fd-draft__name')!.getBoundingClientRect().left)
        expect(new Set(lefts.map(Math.round)).size).toBe(1)
    })

    it('reflows at 320px and retains visible forced-colors focus', async () => {
        await page.viewport(320, 900)
        await cdp().send('Emulation.setEmulatedMedia', {features: [{name: 'forced-colors', value: 'active'}]})
        show(['a very long basename '.repeat(8), 'photos'])
        const root = document.documentElement
        expect(root.scrollWidth).toBeLessThanOrEqual(root.clientWidth)
        const button = screen.getByRole('button', {name: 'Add Files'})
        button.focus()
        expect(getComputedStyle(button).outlineStyle).toBe('solid')
        expect(button.getBoundingClientRect().height).toBeGreaterThanOrEqual(44)
    })
})
