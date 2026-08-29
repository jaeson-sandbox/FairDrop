import {StrictMode, act} from 'react'
import {cleanup, render, screen} from '@testing-library/react'
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import App from './App'

const mocks = vi.hoisted(() => ({
    onFileDrop: vi.fn(),
    onFileDropOff: vi.fn(),
    useTransfer: vi.fn(),
    stage: vi.fn(),
    rejectSelection: vi.fn(),
    cancel: vi.fn(),
    dismissRetained: vi.fn(),
}))

vi.mock('../wailsjs/runtime/runtime', () => ({
    OnFileDrop: mocks.onFileDrop,
    OnFileDropOff: mocks.onFileDropOff,
}))

vi.mock('./transfer/useTransfer', () => ({
    useTransfer: mocks.useTransfer,
}))

function zone() {
    return screen.getByText('Drop a file or folder here').parentElement
}

function isDropTarget(element: Element | null): boolean {
    for (let node = element as HTMLElement | null; node; node = node.parentElement) {
        const value = node.style?.getPropertyValue('--wails-drop-target').trim()
        if (value) return value === 'drop'
    }
    return false
}

function dropOn(target: Element | null, paths: unknown) {
    expect(mocks.onFileDrop).toHaveBeenCalled()
    const callback = mocks.onFileDrop.mock.calls.at(-1)![0]
    if (!isDropTarget(target)) return
    act(() => callback(0, 0, paths))
}

function drop(paths: unknown) {
    dropOn(zone(), paths)
}

beforeEach(() => {
    for (const mock of Object.values(mocks)) mock.mockReset()
    mocks.stage.mockResolvedValue(undefined)
    mocks.cancel.mockResolvedValue(undefined)
    mocks.useTransfer.mockReturnValue({
        state: {phase: 'idle', retainedOutcome: null, commandError: null},
        stage: mocks.stage,
        cancel: mocks.cancel,
        rejectSelection: mocks.rejectSelection,
        dismissRetained: mocks.dismissRetained,
    })
})
afterEach(cleanup)

describe('production transfer controller integration', () => {
    it('mounts the production controller and exposes its current phase without rendering a transfer view', () => {
        render(<App/>)

        expect(mocks.useTransfer).toHaveBeenCalledTimes(1)
        expect(screen.getByRole('main').getAttribute('data-transfer-phase')).toBe('idle')
        expect(screen.queryByRole('progressbar')).toBeNull()
        expect(screen.queryByRole('img')).toBeNull()
    })

    it('routes exactly one native path through Stage as unknown kind', () => {
        render(<App/>)
        const originalPath = String.raw` C:\private\report.pdf `

        drop([originalPath])

        expect(mocks.stage).toHaveBeenCalledTimes(1)
        expect(mocks.stage).toHaveBeenCalledWith(String.raw` C:\private\report.pdf `, 'unknown')
        expect(mocks.rejectSelection).not.toHaveBeenCalled()
    })

    it.each([
        ['zero paths', []],
        ['multiple paths', [String.raw`C:\one.txt`, String.raw`C:\two.txt`]],
        ['undefined input', undefined],
        ['null input', null],
        ['an object', {path: String.raw`C:\one.txt`}],
        ['a non-string singleton', [null]],
        ['an empty singleton', ['']],
        ['a whitespace-only singleton', ['   ']],
    ])('rejects %s and never stages one silently', (_name, paths) => {
        render(<App/>)

        drop(paths)

        expect(mocks.rejectSelection).toHaveBeenCalledTimes(1)
        expect(mocks.stage).not.toHaveBeenCalled()
    })
})

describe('native drop gate lifecycle', () => {
    it('opts into the Wails drop-target gate and marks the inherited zone property', () => {
        render(<App/>)

        expect(mocks.onFileDrop).toHaveBeenCalledTimes(1)
        expect(mocks.onFileDrop.mock.calls[0][1]).toBe(true)
        expect(zone()?.style.getPropertyValue('--wails-drop-target')).toBe('drop')
        expect(isDropTarget(zone()?.firstElementChild ?? null)).toBe(true)
    })

    it('ignores a drop outside the gated zone', () => {
        render(<App/>)
        const outside = screen.getByRole('heading', {name: 'FairDrop'})

        expect(isDropTarget(outside)).toBe(false)
        dropOn(outside, [String.raw`C:\ignored.txt`])

        expect(mocks.stage).not.toHaveBeenCalled()
        expect(mocks.rejectSelection).not.toHaveBeenCalled()
    })

    it('deregisters the native listener on unmount and re-registers on remount', () => {
        const first = render(<App/>)
        expect(mocks.onFileDrop).toHaveBeenCalledTimes(1)

        first.unmount()
        expect(mocks.onFileDropOff).toHaveBeenCalledTimes(1)

        render(<App/>)
        expect(mocks.onFileDrop).toHaveBeenCalledTimes(2)
    })

    it('keeps exactly one live native listener under StrictMode effect replay', () => {
        render(<StrictMode><App/></StrictMode>)

        expect(mocks.onFileDropOff).toHaveBeenCalledTimes(1)
        expect(mocks.onFileDrop).toHaveBeenCalledTimes(2)

        drop([String.raw`C:\report.pdf`])
        expect(mocks.stage).toHaveBeenCalledTimes(1)
    })
})
