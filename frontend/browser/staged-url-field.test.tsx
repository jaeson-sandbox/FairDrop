import {cleanup, render} from '@testing-library/react'
import {page} from 'vitest/browser'
import {afterEach, describe, expect, it, vi} from 'vitest'
import type {StagedTransferState} from '../src/transfer/state'
import type {FileMetadata} from '../src/transfer/types'
import {StagedView} from '../src/ui/StagedView'
import '../src/style.css'

/*
  Rendered evidence for the Staged URL field clipping defect (observed on the
  built binary: the bottom line of the capability URL was clipped by the
  field's border).

  jsdom cannot see this at all -- it reports every element's scrollHeight as 0
  and performs no layout, so a unit test under `src/` would pass against both
  the broken code and the fix, proving nothing (AGENTS.md "Testing standards,
  learned the hard way": a test that agrees with the bug). This lives beside
  accessibility.test.tsx in the rendered Chromium suite instead, which exists
  precisely to catch what jsdom cannot evaluate.

  The field's required height is variable -- host (IPv4 or IPv6), port, and a
  32-hex token -- so this checks both a realistic capability URL and a
  deliberately longer one (an IPv6 host) that wraps to more lines still. A fix
  that merely raises `rows` to the exact line count of one observed URL would
  still fail the second case.
*/

const sessionId = '0123456789abcdef0123456789abcdef'
const token = '94adac272a8fee62a4436c58a4d4bac6'

vi.mock('../wailsjs/go/main/App', () => ({CopyToClipboard: vi.fn().mockResolvedValue(undefined)}))

function metadata(url: string): FileMetadata {
    return {
        sessionId,
        name: 'Travel Notes.pdf',
        size: 8_400_000,
        isDir: false,
        url,
        qrBase64: '',
        warnings: [],
    }
}

function staged(url: string): StagedTransferState {
    return {
        phase: 'staged',
        session: {sessionId, lastSeq: 0},
        metadata: metadata(url),
        cancelPending: false,
        commandError: null,
    }
}

function renderStagedWithURL(url: string): HTMLElement {
    return render(<StagedView state={staged(url)} onCancel={() => undefined}/>).container
}

afterEach(() => {
    cleanup()
})

/**
 * Fails naming the field and both measurements when the textarea's rendered
 * box is shorter than the content it holds -- the exact shape of the observed
 * defect, where the bottom line of the URL sat past the field's own border.
 */
function assertURLFieldFitsItsContent(container: HTMLElement): void {
    const field = container.querySelector<HTMLTextAreaElement>('.fd-url')
    if (field === null) throw new Error('.fd-url did not render')

    expect(
        field.scrollHeight,
        `.fd-url clips its value: scrollHeight (${field.scrollHeight}px) exceeds its own ` +
            `clientHeight (${field.clientHeight}px) for a ${field.value.length}-character URL`,
    ).toBeLessThanOrEqual(field.clientHeight + 0.5)
}

describe('Staged direct URL field sizes to its content (observed clipping defect)', () => {
    it('fits the observed three-line capability URL with nothing clipped', async () => {
        await page.viewport(1024, 900)
        const url = `http://192.168.1.168:63367/download/${token}`
        const container = renderStagedWithURL(url)
        assertURLFieldFitsItsContent(container)
    })

    it('fits a longer IPv6-host URL that wraps to more lines still', async () => {
        await page.viewport(1024, 900)
        const url = `http://[2001:0db8:85a3:0000:0000:8a2e:0370:7334]:63367/download/${token}`
        const container = renderStagedWithURL(url)
        assertURLFieldFitsItsContent(container)
    })

    it('fits the observed URL at the 320px reflow floor, where wrapping is worst', async () => {
        await page.viewport(320, 900)
        const url = `http://192.168.1.168:63367/download/${token}`
        const container = renderStagedWithURL(url)
        assertURLFieldFitsItsContent(container)
    })
})
