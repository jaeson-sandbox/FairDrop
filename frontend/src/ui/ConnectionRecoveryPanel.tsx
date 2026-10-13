import {useEffect, useId, useRef, useState} from 'react'
import {ListConnectionOptions} from '../../wailsjs/go/main/App'
import {parseConnectionOptions} from '../transfer/validation'
import type {ConnectionOptions} from '../transfer/types'
import {copy} from './copy'

interface Props {
    readonly active: boolean
    readonly sessionId: string
    readonly currentAddress: string
    readonly onRefresh: (sessionId: string, networkId: string) => Promise<void>
}

/** The choice is local to one waiting code. Listing never mutates it. */
export function ConnectionRecoveryPanel({active, sessionId, currentAddress, onRefresh}: Props) {
    const selectId = useId()
    const requestRef = useRef(0)
    const [options, setOptions] = useState<ConnectionOptions | null>(null)
    const [loading, setLoading] = useState(false)
    const [error, setError] = useState(false)
    const [selection, setSelection] = useState('')
    const [unavailable, setUnavailable] = useState(false)
    const busyRef = useRef(false)

    useEffect(() => {
        if (!active) return
        void reload()
        return () => { requestRef.current++ }
    }, [active, sessionId])

    async function reload() {
        const request = ++requestRef.current
        setLoading(true)
        setError(false)
        try {
            const raw = await ListConnectionOptions(sessionId)
            const parsed = parseConnectionOptions(raw)
            if (request !== requestRef.current) return
            if (parsed === null || parsed.sessionId !== sessionId) { setError(true); return }
            setOptions(parsed)
            setUnavailable(selection !== '' && !parsed.choices.some(choice => choice.id === selection))
        } catch {
            if (request === requestRef.current) setError(true)
        } finally {
            if (request === requestRef.current) setLoading(false)
        }
    }

    async function refresh() {
        if (busyRef.current || loading || options === null || error || unavailable) return
        busyRef.current = true
        try { await onRefresh(sessionId, selection) } finally { busyRef.current = false }
    }

    return <div className="fd-connection-recovery">
        <p className="fd-body">{copy.recovery.guidance}</p>
        <p className="fd-meta">{copy.recovery.currentAddress} <span>{options?.currentAddress ?? currentAddress}</span></p>
        <label className="fd-body" htmlFor={selectId}>{copy.recovery.networkLabel}</label>
        <select id={selectId} className="fd-recovery-select fd-target" value={selection}
            disabled={loading || busyRef.current}
            onChange={event => { setSelection(event.target.value); setUnavailable(false) }}>
            <option value="">{copy.recovery.automatic}</option>
            {options?.choices.map(choice => <option key={choice.id} value={choice.id}>{choice.name} — {choice.address}</option>)}
            {unavailable && selection !== '' ? <option value={selection}>{copy.recovery.unavailableChoice}</option> : null}
        </select>
        <div role="status" aria-live="polite" aria-atomic="true" className="fd-meta">
            {loading ? copy.recovery.loading : error ? copy.recovery.loadError : unavailable ? copy.recovery.unavailable :
                options?.choices.length === 0 ? copy.recovery.empty : ''}
        </div>
        <div className="fd-recovery-actions">
            <button type="button" className="fd-button fd-button--secondary fd-target" disabled={loading || busyRef.current}
                onClick={() => void reload()}>{copy.recovery.reload}</button>
            <button type="button" className="fd-button fd-button--primary fd-target"
                disabled={loading || busyRef.current || options === null || error || unavailable || options.choices.length === 0}
                onClick={() => void refresh()}>{copy.recovery.refresh}</button>
        </div>
    </div>
}
