import {useEffect, useId, useRef, useState, type MouseEvent} from 'react'
import {CopyToClipboard} from '../../wailsjs/go/main/App'
import type {ReceiveTextOutcomeTransferState, ReceiveTextReceivingTransferState, ReceiveTextWaitingTransferState} from '../transfer/state'
import {ConnectionRecoveryPanel} from './ConnectionRecoveryPanel'
import {copy} from './copy'

export function ReceiveTextWaitingView({state, onCancel, onRefresh, onAnnounce, onCopyFailed}: {
    readonly state: ReceiveTextWaitingTransferState
    readonly onCancel: () => void
    readonly onRefresh: (sessionId: string, networkId: string) => Promise<void>
    readonly onAnnounce: (sessionId: string, text: string) => void
    readonly onCopyFailed: (sessionId: string) => void
}) {
    const [helpOpen, setHelpOpen] = useState(false)
    const [revealed, setRevealed] = useState(false)
    const labelId = useId()
    const revealId = useId()
    const address = new URL(state.metadata.url).hostname
    const noteIds = [
        ...state.metadata.warnings.map((_, index) => `${labelId}-warning-${index}`),
        ...(state.recovered ? [`${labelId}-recovered`] : []),
    ]
    async function copyLink(event: MouseEvent<HTMLButtonElement>) {
        event.currentTarget.focus()
        try { await CopyToClipboard(state.metadata.url); onAnnounce(state.session.sessionId, copy.copy.confirmation) }
        catch { onCopyFailed(state.session.sessionId) }
    }
    return <section className="fd-region" data-phase-view="receive-text-waiting">
        <div className="fd-staged-head">
            <h1 className="fd-state-heading" tabIndex={-1} data-focus-target="receive-text-waiting-heading"
                aria-describedby={noteIds.length > 0 ? noteIds.join(' ') : undefined}>{copy.receiveText.waitingHeading}</h1>
            <p className="fd-meta">{copy.receiveText.waitingBody}</p>
            {state.recovered ? <span id={`${labelId}-recovered`} className="sr-only">{copy.recovery.success}</span> : null}
        </div>
        <div className="fd-packet fd-text-packet">
            <div className="fd-qr-panel"><img className="fd-qr" src={`data:image/png;base64,${state.metadata.qrBase64}`}
                alt="QR code for receiving text" draggable={false}/></div>
            <div className="fd-hero__details">
                <strong className="fd-subheading">{copy.text.kind}</strong>
                <div className="fd-direct-row">
                    <button type="button" className="fd-button fd-button--primary fd-target"
                        aria-disabled={state.cancelPending || undefined}
                        onClick={event => { if (!state.cancelPending) void copyLink(event) }}>{copy.directLink.action}</button>
                    <button type="button" className="fd-button fd-button--secondary fd-target"
                        aria-expanded={revealed} aria-controls={revealId}
                        onClick={() => setRevealed(was => !was)}>{revealed ? copy.directLink.hide : copy.directLink.show}</button>
                </div>
                <div id={revealId} className="fd-url-reveal" data-open={revealed || undefined}>
                    <div className="fd-url-reveal__inner"><div className="fd-url-reveal__body">
                        <label className="fd-subheading" htmlFor={labelId}>{copy.receiveText.linkLabel}</label>
                        <textarea id={labelId} className="fd-text-field fd-url" readOnly value={state.metadata.url} rows={1}
                            onFocus={event => event.currentTarget.select()}/>
                    </div></div>
                </div>
                {state.commandError ? <p className="fd-text-error" tabIndex={-1} data-focus-target="command-error">{state.commandError.message}</p> : null}
                {state.metadata.warnings.map((warning, index) => <p className="fd-warning-banner"
                    id={`${labelId}-warning-${index}`} key={`${warning.code}-${index}`}>{warning.message}</p>)}
                <p className="fd-meta">{copy.receiveText.once}</p>
                <p className="fd-meta">{copy.network.disclosure}</p>
            </div>
        </div>
        <div className="fd-text-actions">
            <button type="button" className="fd-button fd-button--secondary fd-target" aria-expanded={helpOpen}
                onClick={() => setHelpOpen(open => !open)}>{copy.help.heading}</button>
            <button type="button" className="fd-button fd-button--quiet fd-target"
                aria-disabled={state.cancelPending || undefined}
                onClick={() => { if (!state.cancelPending) onCancel() }}>{state.cancelPending ? copy.cancel.pending : copy.cancel.action}</button>
        </div>
        {helpOpen ? <ConnectionRecoveryPanel active={helpOpen} sessionId={state.session.sessionId}
            currentAddress={address} onRefresh={onRefresh}/> : null}
    </section>
}

export function ReceiveTextReceivingView({state, onCancel}: {
    readonly state: ReceiveTextReceivingTransferState
    readonly onCancel: () => void
}) {
    return <section className="fd-region fd-packet" data-phase-view="receive-text-receiving">
        <h1 className="fd-state-heading" tabIndex={-1} data-focus-target="receive-text-receiving-heading">{copy.receiveText.receivingHeading}</h1>
        <p className="fd-body">{copy.receiveText.receivingBody}</p>
        <button type="button" className="fd-button fd-button--quiet fd-target"
            aria-disabled={state.cancelPending || undefined}
            onClick={() => { if (!state.cancelPending) onCancel() }}>{state.cancelPending ? copy.cancel.pending : copy.cancel.action}</button>
        {state.commandError ? <p className="fd-text-error" tabIndex={-1} data-focus-target="command-error">{state.commandError.message}</p> : null}
    </section>
}

export function ReceiveTextOutcomeView({state, onDone, onRetryFetch, onCopy, onAnnounce}: {
    readonly state: ReceiveTextOutcomeTransferState
    readonly onDone: () => void
    readonly onRetryFetch: (sessionId: string) => Promise<void>
    readonly onCopy: (sessionId: string) => Promise<boolean>
    readonly onAnnounce: (sessionId: string, text: string) => void
}) {
    const fieldId = useId()
    const headingId = useId()
    const outcomeRef = useRef<HTMLElement>(null)
    const retryFocusRef = useRef(false)
    const complete = state.kind === 'complete'
    useEffect(() => {
        if (retryFocusRef.current && state.value !== null) {
            retryFocusRef.current = false
            outcomeRef.current?.focus()
        }
    }, [state.value])
    async function copyValue(event: MouseEvent<HTMLButtonElement>) {
        if (state.copying || state.donePending) return
        event.currentTarget.focus()
        if (await onCopy(state.session.sessionId)) onAnnounce(state.session.sessionId, copy.receiveText.copied)
    }
    return <section className={`fd-outcome ${complete ? 'fd-outcome--done' : 'fd-outcome--error'}`}
        data-phase-view="receive-text-outcome" data-focus-target="receive-text-outcome" tabIndex={-1}
        aria-labelledby={headingId} ref={outcomeRef}>
        <h1 id={headingId} className="fd-state-heading">{complete ? copy.receiveText.receivedHeading : copy.receiveText.failedHeading}</h1>
        <p className="fd-outcome__body">{complete ? copy.receiveText.receivedBody : copy.receiveText.failedBody}</p>
        {complete && state.value !== null ? <>
            <label className="fd-subheading" htmlFor={fieldId}>{copy.receiveText.fieldLabel}</label>
            <textarea id={fieldId} className="fd-text-field fd-received-text" readOnly value={state.value} rows={8}/>
        </> : null}
        {complete && state.value === null && !state.fetchError ? <p className="fd-meta">{copy.receiveText.loading}</p> : null}
        {complete && (state.fetchError || state.fetchPending) ? <div>
            {state.fetchError && !state.commandError ? <p className="fd-text-error" data-focus-target="command-error" tabIndex={-1}>{copy.receiveText.fetchFailed}</p> : null}
            <button type="button" className="fd-button fd-button--secondary fd-target"
                aria-disabled={state.fetchPending || state.donePending || undefined}
                onClick={event => { if (!state.fetchPending && !state.donePending) {
                    event.currentTarget.focus()
                    retryFocusRef.current = true
                    void onRetryFetch(state.session.sessionId)
                } }}>
                {state.fetchPending ? copy.receiveText.loading : copy.receiveText.retry}</button>
        </div> : null}
        {complete && state.copyError && !state.commandError ? <p className="fd-text-error" data-focus-target="command-error" tabIndex={-1}>{copy.receiveText.copyFailed}</p> : null}
        {state.commandError ? <p className="fd-text-error" data-focus-target="command-error" tabIndex={-1}>{state.commandError.message}</p> : null}
        <div className="fd-outcome__actions">
            {complete && state.value !== null ? <button type="button" className="fd-button fd-button--primary fd-target"
                aria-disabled={state.copying || state.donePending || undefined}
                onClick={event => void copyValue(event)}>{copy.receiveText.copyAction}</button> : null}
            <button type="button" className="fd-button fd-button--quiet fd-target"
                aria-disabled={state.donePending || undefined}
                onClick={() => { if (!state.donePending) onDone() }}>{copy.done.dismiss}</button>
        </div>
    </section>
}
