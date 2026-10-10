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
        expect(screen.getByText('2 of 16 items')).toBeTruthy()
        expect(screen.getByRole('button', {name: 'Remove 1. report.txt'})).toBeTruthy()
        const ordinal = document.querySelector<HTMLElement>('.fd-draft__ordinal')
        expect(ordinal?.textContent).toBe('1.')
        expect(ordinal?.getBoundingClientRect().width).toBeGreaterThan(0)
        await userEvent.tab()
        expect(document.activeElement?.textContent).toContain('Remove')
        await userEvent.keyboard('{Enter}')
        expect(onRemove).toHaveBeenCalledWith(0)
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
