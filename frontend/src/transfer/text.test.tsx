import {act, renderHook} from '@testing-library/react'
import {StrictMode} from 'react'
import {beforeEach, describe, expect, it, vi} from 'vitest'
import {parseConnectionRefresh, parseTextMetadata, validateText} from './validation'
import {useTransfer} from './useTransfer'
import {routeTransition} from '../ui/announce'
import {publicError} from './errors'

const mocks = vi.hoisted(() => ({stageText: vi.fn(), cancel: vi.fn(), refresh: vi.fn(), events: vi.fn()}))
vi.mock('../../wailsjs/go/main/App', () => ({
    StageText: mocks.stageText, CancelTransfer: mocks.cancel, RefreshConnection: mocks.refresh,
    StageTransfer: vi.fn(), StageTransfers: vi.fn(), SelectFile: vi.fn(), SelectDirectory: vi.fn(),
    SelectFiles: vi.fn(), SelectReceiveFolder: vi.fn(), StartReceive: vi.fn(), ShowReceivedFolder: vi.fn(),
}))
vi.mock('../../wailsjs/runtime/runtime', () => ({EventsOn: mocks.events}))

const first = '11111111111111111111111111111111'
const second = '22222222222222222222222222222222'
const png = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII='
const metadata = (sessionId = first, byteLength = 4) => ({sessionId, byteLength,
    url: `http://192.168.1.5:45678/text/${sessionId}`, qrBase64: png, warnings: []})
const listeners = new Map<string, (...args: unknown[]) => void>()
function emit(name: string, payload: unknown) { act(() => listeners.get(name)?.(payload)) }
function progress(bytesSent: number) { return {bytesSent, totalBytes: 0, totalKnown: false, percent: 0, speedBytesPerSec: 0} }

beforeEach(() => {
    listeners.clear()
    mocks.stageText.mockReset().mockResolvedValue(metadata())
    mocks.cancel.mockReset().mockResolvedValue(undefined)
    mocks.refresh.mockReset()
    mocks.events.mockReset().mockImplementation((name: string, callback: (...args: unknown[]) => void) => {
        listeners.set(name, callback)
        return () => listeners.delete(name)
    })
})

describe('outbound text input', () => {
    it('normalizes only line endings and preserves spaces, tabs and Unicode', () => {
        expect(validateText('  café\r\n\t<a>\r  ')).toEqual({ok: true, text: '  café\n\t<a>\n  ', byteLength: 15})
        expect(validateText('x'.repeat(65536)).ok).toBe(true)
        expect(validateText('x'.repeat(65537))).toEqual({ok: false, reason: 'too-large'})
        expect(validateText('a' + '\r\n'.repeat(65535))).toEqual({ok: true,
            text: 'a' + '\n'.repeat(65535), byteLength: 65536})
        expect(validateText('\r\n'.repeat(65536))).toEqual({ok: false, reason: 'blank'})
        expect(validateText('https://example.test/?a=1&b=2')).toEqual({ok: true,
            text: 'https://example.test/?a=1&b=2', byteLength: 29})
    })
    it('refuses blank text, controls, unpaired surrogate and oversized raw input', () => {
        expect(validateText('\u2003\u00a0\t')).toEqual({ok: false, reason: 'blank'})
        expect(validateText('a\u0000b')).toEqual({ok: false, reason: 'unsupported'})
        expect(validateText('a\u007fb')).toEqual({ok: false, reason: 'unsupported'})
        expect(validateText('\ud800')).toEqual({ok: false, reason: 'unsupported'})
        expect(validateText('a'.repeat(131073))).toEqual({ok: false, reason: 'too-large'})
        expect(validateText('a'.repeat(65536) + '\u0000')).toEqual({ok: false, reason: 'too-large'})
        expect(validateText('a'.repeat(65536) + '\ud800')).toEqual({ok: false, reason: 'too-large'})
    })
    it('accepts only exact text-route metadata and exclusive recovery modes', () => {
        expect(parseTextMetadata(metadata())?.byteLength).toBe(4)
        expect(parseTextMetadata({...metadata(), url: `http://192.168.1.5:45678/download/${first}`})).toBeNull()
        expect(parseTextMetadata({...metadata(), payload: 'secret'})).toBeNull()
        expect(parseConnectionRefresh({mode: 'text', text: metadata(second)})).toEqual({mode: 'text', text: metadata(second)})
        expect(parseConnectionRefresh({mode: 'text', text: metadata(second), send: metadata()})).toBeNull()
    })
})

describe('outbound text controller', () => {
    it('stages normalized text without file commands and excludes file selection while composing', async () => {
        mocks.stageText.mockResolvedValue(metadata(first, 3))
        const hook = renderHook(() => useTransfer())
        act(() => hook.result.current.openText())
        act(() => hook.result.current.editText('A\r\nB'))
        await act(async () => hook.result.current.stage('/private/file.txt'))
        expect(hook.result.current.state.phase).toBe('idle')
        await act(async () => hook.result.current.sendText())
        expect(mocks.stageText).toHaveBeenCalledExactlyOnceWith('A\nB')
        expect(hook.result.current.state.phase).toBe('text-staged')
        expect(hook.result.current.composer).toBeNull()
        expect(JSON.stringify(hook.result.current.state)).not.toContain('A\nB')
    })
    it('does not serialize an unpaired surrogate and retains a correctable draft', async () => {
        const hook = renderHook(() => useTransfer())
        act(() => hook.result.current.openText())
        act(() => hook.result.current.editText('\ud800'))
        await act(async () => hook.result.current.sendText())
        expect(mocks.stageText).not.toHaveBeenCalled()
        expect(hook.result.current.composer?.error).toBe('Text contains an unsupported character.')
        act(() => hook.result.current.cancelText())
        expect(hook.result.current.composer).toBeNull()
    })
    it('keeps the last bounded draft when an edit is oversized even with unsupported characters', () => {
        const hook = renderHook(() => useTransfer())
        act(() => hook.result.current.openText())
        act(() => hook.result.current.editText('note'))
        act(() => hook.result.current.editText('a'.repeat(65536) + '\u0000'))
        expect(hook.result.current.composer).toEqual({text: 'note', error: 'Text is too long (65,536 UTF-8 bytes maximum).'})
        act(() => hook.result.current.editText('a'.repeat(65536) + '\ud800'))
        expect(hook.result.current.composer).toEqual({text: 'note', error: 'Text is too long (65,536 UTF-8 bytes maximum).'})
    })
    it('scrubs terminal receipt and sends again through a fresh StageText', async () => {
        const hook = renderHook(() => useTransfer())
        act(() => hook.result.current.openText())
        act(() => hook.result.current.editText('note'))
        await act(async () => hook.result.current.sendText())
        emit('transfer-started', {sessionId: first, seq: 1})
        emit('transfer-complete', {sessionId: first, seq: 2, progress: progress(600)})
        expect(hook.result.current.state.phase).toBe('done')
        expect(JSON.stringify(hook.result.current.state)).not.toContain('/text/')
        mocks.stageText.mockResolvedValue(metadata(second))
        emit('transfer-reset', {sessionId: first, seq: 3})
        await act(async () => hook.result.current.sendAgain())
        expect(mocks.stageText).toHaveBeenCalledTimes(2)
        expect(mocks.stageText).toHaveBeenLastCalledWith('note')
        expect(hook.result.current.state.phase).toBe('text-staged')
        if (hook.result.current.state.phase === 'text-staged') expect(hook.result.current.state.session.sessionId).toBe(second)
        emit('transfer-started', {sessionId: first, seq: 4})
        expect(hook.result.current.state.phase).toBe('text-staged')
    })
    it('keeps text across the internal live-outcome cancel used by Send Again', async () => {
        const hook = renderHook(() => useTransfer())
        act(() => hook.result.current.openText())
        act(() => hook.result.current.editText('note'))
        await act(async () => hook.result.current.sendText())
        emit('transfer-started', {sessionId: first, seq: 1})
        emit('transfer-complete', {sessionId: first, seq: 2, progress: progress(600)})
        mocks.stageText.mockResolvedValue(metadata(second))
        let again!: Promise<void>
        act(() => { again = hook.result.current.sendAgain() })
        await act(async () => { await Promise.resolve(); await Promise.resolve() })
        emit('transfer-reset', {sessionId: first, seq: 3})
        await act(async () => again)
        expect(mocks.cancel).toHaveBeenCalledTimes(1)
        expect(mocks.stageText).toHaveBeenCalledTimes(2)
        expect(mocks.stageText).toHaveBeenLastCalledWith('note')
        expect(hook.result.current.state.phase).toBe('text-staged')
    })
    it('forgets remembered text on explicit Cancel', async () => {
        const hook = renderHook(() => useTransfer())
        act(() => hook.result.current.openText())
        act(() => hook.result.current.editText('note'))
        await act(async () => hook.result.current.sendText())
        await act(async () => hook.result.current.cancel())
        expect(hook.result.current.canRetry).toBe(false)
        emit('transfer-reset', {sessionId: first, seq: 1})
        await act(async () => hook.result.current.sendAgain())
        expect(mocks.stageText).toHaveBeenCalledTimes(1)
    })
    it('refreshes waiting text with an exclusive new text acknowledgement', async () => {
        const hook = renderHook(() => useTransfer())
        act(() => hook.result.current.openText())
        act(() => hook.result.current.editText('note'))
        await act(async () => hook.result.current.sendText())
        mocks.refresh.mockResolvedValue({mode: 'text', text: metadata(second)})
        await act(async () => hook.result.current.refreshConnection!(first, ''))
        expect(mocks.refresh).toHaveBeenCalledExactlyOnceWith(first, '')
        expect(hook.result.current.state.phase).toBe('text-staged')
        if (hook.result.current.state.phase === 'text-staged') expect(hook.result.current.state.metadata.sessionId).toBe(second)
    })
    it('ignores a late text recovery acknowledgement after Cancel', async () => {
        let resolve!: (value: unknown) => void
        mocks.refresh.mockReturnValue(new Promise(yes => { resolve = yes }))
        const hook = renderHook(() => useTransfer())
        act(() => hook.result.current.openText())
        act(() => hook.result.current.editText('note'))
        await act(async () => hook.result.current.sendText())
        let recovery!: Promise<void>
        act(() => { recovery = hook.result.current.refreshConnection!(first, '') })
        await act(async () => hook.result.current.cancel())
        await act(async () => { resolve({mode: 'text', text: metadata(second)}); await recovery })
        expect(hook.result.current.state.phase).toBe('idle')
        expect(hook.result.current.canRetry).toBe(false)
    })

    it('cancels an in-flight text stage and ignores its late acknowledgement', async () => {
        let resolve!: (value: unknown) => void
        mocks.stageText.mockReturnValue(new Promise(yes => { resolve = yes }))
        const hook = renderHook(() => useTransfer())
        act(() => hook.result.current.openText())
        act(() => hook.result.current.editText('note'))
        let stage!: Promise<void>
        act(() => { stage = hook.result.current.sendText() })
        let cancellation!: Promise<void>
        act(() => { cancellation = hook.result.current.cancel() })
        await act(async () => { resolve(metadata()); await Promise.all([stage, cancellation]) })
        expect(hook.result.current.state.phase).toBe('idle')
        expect(hook.result.current.composer).toBeNull()
        expect(hook.result.current.canRetry).toBe(false)
        expect(mocks.cancel).toHaveBeenCalledTimes(1)
    })

    it('quiesces a malformed text acknowledgement before retaining the correctable draft', async () => {
        mocks.stageText.mockResolvedValue({...metadata(), url: `http://192.168.1.5:45678/download/${first}`})
        const hook = renderHook(() => useTransfer())
        act(() => hook.result.current.openText())
        act(() => hook.result.current.editText('note'))
        await act(async () => hook.result.current.sendText())
        expect(mocks.cancel).toHaveBeenCalledTimes(1)
        expect(hook.result.current.state.phase).toBe('idle')
        expect(hook.result.current.composer?.text).toBe('note')
        expect(hook.result.current.composer?.error).toBe('FairDrop couldn’t prepare that item. Nothing was sent. Choose it again.')
    })
    it('reports cleanup uncertainty when malformed StageText metadata cannot be quiesced', async () => {
        mocks.stageText.mockResolvedValue({...metadata(), payload: 'secret'})
        mocks.cancel.mockRejectedValue(new Error('private adapter text'))
        const hook = renderHook(() => useTransfer())
        act(() => hook.result.current.openText())
        act(() => hook.result.current.editText('note'))
        await act(async () => hook.result.current.sendText())
        expect(hook.result.current.state.phase).toBe('idle')
        expect(hook.result.current.composer?.error).toBe('FairDrop couldn’t confirm it released the connection. Nothing was sent. Close FairDrop and reopen it before sending again.')
        expect(hook.result.current.composer?.error).not.toContain('private')
    })
    it('clears a stale Stage failure on edit so a new invalid submission owns the alert', async () => {
        mocks.stageText.mockRejectedValue(new Error(JSON.stringify({code: 'setup_failed', message: 'ignored'})))
        const hook = renderHook(() => useTransfer())
        act(() => hook.result.current.openText())
        act(() => hook.result.current.editText('note'))
        await act(async () => hook.result.current.sendText())
        expect(hook.result.current.state.phase).toBe('idle')
        act(() => hook.result.current.editText(''))
        expect(hook.result.current.state.phase === 'idle' && hook.result.current.state.commandError).toBeNull()
        await act(async () => hook.result.current.sendText())
        expect(hook.result.current.composer?.error).toBe('Enter some text to send.')
    })
    it('clears stale Stage errors for oversized edits and all selection conflicts', async () => {
        mocks.stageText.mockRejectedValue(new Error(JSON.stringify({code: 'setup_failed', message: 'ignored'})))
        const hook = renderHook(() => useTransfer())
        act(() => hook.result.current.openText())
        act(() => hook.result.current.editText('note'))
        async function failSetup() {
            await act(async () => hook.result.current.sendText())
            expect(hook.result.current.state.phase === 'idle' && hook.result.current.state.commandError?.code).toBe('setup_failed')
        }
        function expectLocal(message: string) {
            expect(hook.result.current.state.phase === 'idle' && hook.result.current.state.commandError).toBeNull()
            expect(hook.result.current.composer?.error).toBe(message)
        }
        await failSetup()
        act(() => hook.result.current.editText('a'.repeat(65537)))
        expectLocal('Text is too long (65,536 UTF-8 bytes maximum).')
        expect(hook.result.current.composer?.text).toBe('note')
        await failSetup()
        await act(async () => hook.result.current.stage('/private/item'))
        expectLocal('Finish or cancel this text before selecting files.')
        await failSetup()
        await act(async () => hook.result.current.selectFile())
        expectLocal('Finish or cancel this text before selecting files.')
        await failSetup()
        act(() => hook.result.current.rejectSelection())
        expectLocal('Finish or cancel this text before selecting files.')
    })
    it('cancels and forgets a live text session on unmount', async () => {
        const hook = renderHook(() => useTransfer())
        act(() => hook.result.current.openText())
        act(() => hook.result.current.editText('note'))
        await act(async () => hook.result.current.sendText())
        hook.unmount()
        await act(async () => { await Promise.resolve(); await Promise.resolve() })
        expect(mocks.cancel).toHaveBeenCalledTimes(1)
    })
    it('cancels a transferring text session on unmount', async () => {
        const hook = renderHook(() => useTransfer())
        act(() => hook.result.current.openText())
        act(() => hook.result.current.editText('note'))
        await act(async () => hook.result.current.sendText())
        emit('transfer-started', {sessionId: first, seq: 1})
        expect(hook.result.current.state.phase).toBe('text-transferring')
        hook.unmount()
        await act(async () => { await Promise.resolve(); await Promise.resolve() })
        expect(mocks.cancel).toHaveBeenCalledTimes(1)
    })
    it('does not cancel a retained live text session during StrictMode effect replay', async () => {
        const hook = renderHook(() => useTransfer(), {wrapper: ({children}) => <StrictMode>{children}</StrictMode>})
        act(() => hook.result.current.openText())
        act(() => hook.result.current.editText('note'))
        await act(async () => hook.result.current.sendText())
        await act(async () => { await Promise.resolve(); await Promise.resolve() })
        expect(mocks.cancel).not.toHaveBeenCalled()
    })
    it('routes text Cancel failure and recovery failure to the rendered command error', () => {
        const base = {phase: 'text-staged' as const, session: {sessionId: first, lastSeq: 0},
            metadata: metadata(), cancelPending: false, commandError: null}
        expect(routeTransition({...base, cancelPending: true}, {...base, commandError: publicError('transfer_failed')}))
            .toEqual({row: 'command-failure', owner: 'focus', target: 'command-error'})
        const sending = {phase: 'text-transferring' as const, session: base.session, metadata: base.metadata,
            cancelPending: true, commandError: null}
        expect(routeTransition(sending, {...sending, cancelPending: false, commandError: publicError('transfer_failed')}))
            .toEqual({row: 'command-failure', owner: 'focus', target: 'command-error'})
        const recovering = {phase: 'recovering' as const, generation: 1, mode: 'text' as const,
            session: base.session, metadata: base.metadata, cancelPending: false, retired: false, refusals: 0}
        expect(routeTransition(recovering, {...base, commandError: publicError('network_unavailable')}))
            .toEqual({row: 'command-failure', owner: 'focus', target: 'command-error'})
    })
    it('cancels a pending text stage on unmount and ignores its later acknowledgement', async () => {
        let resolve!: (value: unknown) => void
        mocks.stageText.mockReturnValue(new Promise(yes => { resolve = yes }))
        const hook = renderHook(() => useTransfer())
        act(() => hook.result.current.openText())
        act(() => hook.result.current.editText('note'))
        act(() => { void hook.result.current.sendText() })
        hook.unmount()
        await act(async () => { await Promise.resolve(); await Promise.resolve() })
        expect(mocks.cancel).toHaveBeenCalledTimes(1)
        resolve(metadata())
        await act(async () => { await Promise.resolve(); await Promise.resolve() })
        expect(mocks.stageText).toHaveBeenCalledTimes(1)
    })
})
