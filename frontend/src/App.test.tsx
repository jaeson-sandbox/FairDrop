import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import {StrictMode, act} from 'react'
import {cleanup, render, screen} from '@testing-library/react'
import App from './App'

// Covers the I/O & Edge-Case Matrix in
// _bmad-output/implementation-artifacts/spec-phase-1-wails-scaffold.md.
//
// The native half of the drop (OS -> webview -> absolute paths) belongs to the
// Wails runtime and cannot run under jsdom. These tests stand in for it by
// mocking the runtime module, then driving the exact callback the app
// registered -- so what is asserted is our contract with Wails and what the app
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

/** The drop zone element -- the one carrying the --wails-drop-target property. */
function zone() {
    return screen.getByText('Drop a file or folder here').parentElement
}

/**
 * Resolves --wails-drop-target the way a browser would. The property inherits,
 * so the value is whatever the nearest ancestor that sets it says. jsdom does
 * not compute custom-property inheritance, so it is resolved here from inline
 * styles. Mirrors checkStyleDropTarget() in Wails' draganddrop.js.
 */
function isDropTarget(element: Element | null): boolean {
    for (let node = element as HTMLElement | null; node; node = node.parentElement) {
        const value = node.style?.getPropertyValue('--wails-drop-target').trim()
        if (value) return value === 'drop'
    }
    return false
}

/**
 * Replays a native drop landing on `target`, applying the same gate Wails
 * applies before it invokes our callback: it looks up the element under the
 * drop point and requires --wails-drop-target: drop. A drop that fails the gate
 * never reaches the app, which is exactly the "drop outside the zone" row of
 * the matrix.
 */
function dropOn(target: Element | null, paths: string[]) {
    expect(onFileDrop).toHaveBeenCalled()
    // Wails holds the most recent registration.
    const callback = onFileDrop.mock.calls.at(-1)![0]
    if (!isDropTarget(target)) return
    act(() => callback(0, 0, paths))
}

/** Replays a native drop landing inside the drop zone. */
function drop(paths: string[]) {
    dropOn(zone(), paths)
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

    it('renders a repeated path once per occurrence without a key collision', () => {
        render(<App/>)

        drop(['C:\\x\\a.txt', 'C:\\x\\a.txt'])

        expect(screen.getAllByText('C:\\x\\a.txt')).toHaveLength(2)
    })

    it('opts into the drop-target gate and marks the zone with the property', () => {
        render(<App/>)

        expect(onFileDrop).toHaveBeenCalledTimes(1)
        expect(onFileDrop.mock.calls[0][1]).toBe(true)
        expect(zone()?.style.getPropertyValue('--wails-drop-target')).toBe('drop')
    })

    // Matrix row: a drop landing outside the zone must not fire the callback and
    // must leave the UI untouched. Asserted from a non-empty state so it cannot
    // pass trivially the way an initial-render assertion would.
    it('ignores a drop that lands outside the zone, leaving the UI unchanged', () => {
        render(<App/>)
        drop(['C:\\x\\a.txt'])
        expect(screen.getAllByRole('listitem')).toHaveLength(1)

        // The heading sits outside the zone and inherits no drop-target property.
        const outside = screen.getByRole('heading', {name: 'FairDrop'})
        expect(isDropTarget(outside)).toBe(false)
        dropOn(outside, ['C:\\x\\ignored.txt'])

        expect(screen.queryByText('C:\\x\\ignored.txt')).toBeNull()
        expect(screen.getAllByRole('listitem')).toHaveLength(1)
        expect(screen.getByText('C:\\x\\a.txt')).toBeTruthy()
    })

    it('renders no list before anything has been dropped', () => {
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

    // main.tsx renders under StrictMode, which double-invokes effects
    // (effect -> cleanup -> effect). Wails guards re-registration with
    // `if (flags.registered) return` and OnFileDropOff clears that flag, so the
    // cleanup is what lets the second registration take effect. Asserted rather
    // than assumed, since a missing cleanup would silently leave the app deaf.
    it('keeps exactly one live listener under StrictMode double-invoked effects', () => {
        render(<StrictMode><App/></StrictMode>)

        expect(onFileDropOff).toHaveBeenCalledTimes(1)
        expect(onFileDrop).toHaveBeenCalledTimes(2)

        drop(['C:\\x\\a.txt'])

        expect(screen.getAllByRole('listitem')).toHaveLength(1)
    })
})
