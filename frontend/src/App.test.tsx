import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {act} from 'react'
import {cleanup, render, screen} from '@testing-library/react'
import App from './App'

// Covers the I/O & Edge-Case Matrix in
// _bmad-output/implementation-artifacts/spec-phase-1-wails-scaffold.md.
//
// The native half of the drop (OS -> webview -> absolute paths) belongs to the
// Wails runtime and cannot run under jsdom. These tests stand in for it by
// mocking the runtime module, then driving the exact callback the app
// registered — so what is asserted is our contract with Wails and what the app
// does with the paths it is handed.

const onFileDrop = vi.fn()
const onFileDropOff = vi.fn()

vi.mock('../wailsjs/runtime/runtime', () => ({
    OnFileDrop: (
        callback: (x: number, y: number, paths: string[]) => void,
        useDropTarget: boolean,
    ) => onFileDrop(callback, useDropTarget),
    OnFileDropOff: () => onFileDropOff(),
}))

/** Replays a native drop through the callback the app registered with Wails. */
function drop(paths: string[]) {
    const callback = onFileDrop.mock.calls[0][0]
    act(() => callback(0, 0, paths))
}

beforeEach(() => {
    onFileDrop.mockClear()
    onFileDropOff.mockClear()
})

afterEach(cleanup)

describe('drop zone', () => {
    it('renders the absolute path of a single dropped file', () => {
        render(<App/>)

        drop(['C:\\x\\a.txt'])

        expect(screen.getByText('C:\\x\\a.txt')).toBeTruthy()
    })

    it('renders the absolute path of a dropped directory', () => {
        render(<App/>)

        drop(['C:\\x\\dir'])

        expect(screen.getByText('C:\\x\\dir')).toBeTruthy()
    })

    it('renders every path of a multi-file drop', () => {
        render(<App/>)

        const paths = ['C:\\x\\a.txt', 'C:\\x\\b.txt', 'C:\\x\\c.txt']
        drop(paths)

        for (const path of paths) {
            expect(screen.getByText(path)).toBeTruthy()
        }
    })

    // Matrix row: a drop landing outside the zone must not fire the callback.
    // The gate itself is Wails' code (it walks up from elementFromPoint looking
    // for the CSS custom property), so what we own — and assert here — is that
    // we opt into that gate and that the zone actually carries the property.
    it('opts into the drop-target gate and marks the zone with the property', () => {
        render(<App/>)

        expect(onFileDrop).toHaveBeenCalledTimes(1)
        expect(onFileDrop.mock.calls[0][1]).toBe(true)

        const zone = screen.getByText('Drop a file or folder here').parentElement
        expect(zone?.style.getPropertyValue('--wails-drop-target')).toBe('drop')
    })

    it('starts empty so an ignored drop leaves the UI unchanged', () => {
        render(<App/>)

        expect(screen.queryByRole('list')).toBeNull()
    })

    it('deregisters the listener on unmount and re-registers once on remount', () => {
        const {unmount} = render(<App/>)
        expect(onFileDrop).toHaveBeenCalledTimes(1)

        unmount()
        expect(onFileDropOff).toHaveBeenCalledTimes(1)

        render(<App/>)
        expect(onFileDrop).toHaveBeenCalledTimes(2)
    })
})
