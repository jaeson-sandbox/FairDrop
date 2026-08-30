import {StrictMode, act} from 'react'
import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import App from './App'

const mocks = vi.hoisted(() => ({
    onFileDrop: vi.fn(),
    onFileDropOff: vi.fn(),
    useTransfer: vi.fn(),
    stage: vi.fn(),
    selectFile: vi.fn(),
    selectDirectory: vi.fn(),
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

function zone(): HTMLElement | null {
    return screen.getByRole('heading', {name: 'Drop one file or folder.'}).closest<HTMLElement>('.fd-drop-zone')
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
    mocks.selectFile.mockResolvedValue(undefined)
    mocks.selectDirectory.mockResolvedValue(undefined)
    mocks.useTransfer.mockReturnValue({
        state: {phase: 'idle', retainedOutcome: null, commandError: null},
        stage: mocks.stage,
        selectFile: mocks.selectFile,
        selectDirectory: mocks.selectDirectory,
        cancel: mocks.cancel,
        rejectSelection: mocks.rejectSelection,
        dismissRetained: mocks.dismissRetained,
    })
})
afterEach(cleanup)

describe('production transfer controller integration', () => {
    it('mounts the production controller once and renders only the Idle view for the idle phase', () => {
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
        const outside = screen.getByRole('heading', {name: 'Local network access'})

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

const sessionId = '0123456789abcdef0123456789abcdef'
const capabilityURL = 'http://192.0.2.1:34123/download/fedcba9876543210fedcba9876543210'
const qrPNG = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII='

const metadata = {
    sessionId,
    name: 'report.pdf',
    size: 100,
    isDir: false,
    url: capabilityURL,
    qrBase64: qrPNG,
    warnings: [],
}

function mountWith(state: Record<string, unknown>) {
    mocks.useTransfer.mockReturnValue({
        state,
        stage: mocks.stage,
        selectFile: mocks.selectFile,
        selectDirectory: mocks.selectDirectory,
        cancel: mocks.cancel,
        rejectSelection: mocks.rejectSelection,
        dismissRetained: mocks.dismissRetained,
    })
    return render(<App/>)
}

function phaseViews(): string[] {
    return [...document.querySelectorAll('[data-phase-view]')]
        .map((element) => element.getAttribute('data-phase-view') ?? '')
}

describe('one view per phase', () => {
    it.each([
        ['idle', {phase: 'idle', retainedOutcome: null, commandError: null}, 'idle'],
        ['pending', {phase: 'pending', generation: 1, itemKind: 'file', cancelPending: false}, 'pending'],
        ['staged', {
            phase: 'staged',
            session: {sessionId, lastSeq: 0},
            metadata,
            cancelPending: false,
            commandError: null,
        }, 'staged'],
        ['transferring', {
            phase: 'transferring',
            session: {sessionId, lastSeq: 1},
            metadata,
            progress: null,
            cancelPending: false,
            commandError: null,
        }, 'transferring'],
        ['done', {phase: 'done', session: {sessionId, lastSeq: 4}, outcome: {kind: 'done'}}, 'outcome'],
        ['error', {
            phase: 'error',
            session: {sessionId, lastSeq: 4},
            outcome: {kind: 'error', error: {code: 'transfer_failed', message: 'ignored'}},
        }, 'outcome'],
    ])('renders exactly the %s view and marks the phase on the shell', (phase, state, view) => {
        mountWith(state)

        expect(phaseViews()).toEqual([view])
        expect(screen.getByRole('main').getAttribute('data-transfer-phase')).toBe(phase)
    })

    it('gives the terminal outcome the document heading', () => {
        mountWith({phase: 'done', session: {sessionId, lastSeq: 4}, outcome: {kind: 'done'}})

        expect(screen.getByRole('heading', {level: 1}).textContent).toBe('Transfer finished')
    })

    it('renders no phase view at all rather than showing a cancellation as an Error', () => {
        mountWith({
            phase: 'error',
            session: {sessionId, lastSeq: 4},
            outcome: {kind: 'error', error: {code: 'cancelled', message: 'Transfer canceled.'}},
        })

        expect(phaseViews()).toEqual([])
        expect(screen.queryByText('Transfer canceled.')).toBeNull()
    })

    it('pre-mounts one atomic polite status region in every phase', () => {
        mountWith({phase: 'idle', retainedOutcome: null, commandError: null})

        const announcers = [...document.querySelectorAll('[role="status"]')]
        expect(announcers).toHaveLength(1)
        expect(announcers[0].getAttribute('aria-live')).toBe('polite')
        expect(announcers[0].getAttribute('aria-atomic')).toBe('true')
        expect(announcers[0].textContent).toBe('')
    })
})

describe('controller wiring', () => {
    it('routes both browse controls to their own command', () => {
        mountWith({phase: 'idle', retainedOutcome: null, commandError: null})

        fireEvent.click(screen.getByRole('button', {name: 'Select File'}))
        expect(mocks.selectFile).toHaveBeenCalledTimes(1)
        expect(mocks.selectDirectory).not.toHaveBeenCalled()

        fireEvent.click(screen.getByRole('button', {name: 'Select Directory'}))
        expect(mocks.selectDirectory).toHaveBeenCalledTimes(1)
        expect(mocks.stage).not.toHaveBeenCalled()
    })

    it('routes Dismiss to the retained-outcome command', () => {
        mountWith({phase: 'idle', retainedOutcome: {kind: 'done'}, commandError: null})

        fireEvent.click(screen.getByRole('button', {name: 'Dismiss'}))
        expect(mocks.dismissRetained).toHaveBeenCalledTimes(1)
    })

    it.each([
        ['Cancel preparation', {phase: 'pending', generation: 1, itemKind: 'file', cancelPending: false}],
        ['Cancel', {
            phase: 'staged',
            session: {sessionId, lastSeq: 0},
            metadata,
            cancelPending: false,
            commandError: null,
        }],
        ['Cancel', {
            phase: 'transferring',
            session: {sessionId, lastSeq: 1},
            metadata,
            progress: null,
            cancelPending: false,
            commandError: null,
        }],
    ])('routes %s to the one cancel command', (name, state) => {
        mountWith(state)

        fireEvent.click(screen.getByRole('button', {name}))
        expect(mocks.cancel).toHaveBeenCalledTimes(1)
    })
})
