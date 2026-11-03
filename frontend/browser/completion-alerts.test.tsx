import type {} from '@vitest/browser-playwright'
import {cleanup, render, screen} from '@testing-library/react'
import {page} from 'vitest/browser'
import {afterEach, expect, it, vi} from 'vitest'
import {CompletionAlertsControl} from '../src/ui/CompletionAlertsControl'
import '../src/style.css'

afterEach(cleanup)

it('keeps the named completion-alert checkbox activation target at least 44px in both window widths', async () => {
    for (const width of [1024, 320]) {
        await page.viewport(width, 768)
        const view = render(<main className="fd-app"><CompletionAlertsControl enabled={false} pending={false}
            hydrated={true} error={false} onChange={vi.fn()}/></main>)
        const checkbox = screen.getByRole('checkbox', {name: 'Background completion alerts'})
        const label = checkbox.closest('label')
        expect(label).not.toBeNull()
        const rect = label!.getBoundingClientRect()
        expect(rect.width, `${width}px window: checkbox activation width`).toBeGreaterThanOrEqual(44)
        expect(rect.height, `${width}px window: checkbox activation height`).toBeGreaterThanOrEqual(44)
        view.unmount()
    }
})
