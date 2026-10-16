import {useId, useState, type MouseEvent} from 'react'
import {CopyToClipboard} from '../../wailsjs/go/main/App'
import type {TextStagedTransferState, TextTransferringTransferState} from '../transfer/state'
import {ConnectionRecoveryPanel} from './ConnectionRecoveryPanel'
import {copy} from './copy'

export function TextComposer({text, error, alertError = true, onEdit, onSend, onCancel}: {
    readonly text: string
    readonly error: string | null
    readonly alertError?: boolean
    readonly onEdit: (value: string) => void
    readonly onSend: () => void
    readonly onCancel: () => void
}) {
    const fieldId = useId()
    const errorId = useId()
    return <section className="fd-region fd-text-composer" data-phase-view="text-composer">
        <h1 className="fd-state-heading" tabIndex={-1} data-focus-target="text-composer-heading">{copy.text.open}</h1>
        <p className="fd-body">{copy.text.intro}</p>
        <label htmlFor={fieldId} className="fd-subheading">{copy.text.label}</label>
        <textarea id={fieldId} className="fd-text-field" value={text} rows={8}
            aria-describedby={`${fieldId}-limit${error === null ? '' : ` ${errorId}`}`}
            aria-invalid={error === null ? undefined : true}
            onChange={event => onEdit(event.target.value)}/>
        <p id={`${fieldId}-limit`} className="fd-meta">{copy.text.limit}</p>
        {error === null ? null : <p id={errorId} className="fd-text-error" data-focus-target="command-error"
            role={alertError ? 'alert' : undefined} tabIndex={-1}>{error}</p>}
        <div className="fd-text-actions">
            <button type="button" className="fd-button fd-button--primary fd-target" onClick={onSend}>{copy.selection.send}</button>
            <button type="button" className="fd-button fd-button--quiet fd-target" onClick={onCancel}>{copy.cancel.action}</button>
        </div>
    </section>
}

export function TextStagedView({state, onCancel, onAnnounce, onCopyFailed, onRefreshConnection}: {
    readonly state: TextStagedTransferState
    readonly onCancel: () => void
    readonly onAnnounce: (sessionId: string, text: string) => void
    readonly onCopyFailed: (sessionId: string) => void
    readonly onRefreshConnection: (sessionId: string, networkId: string) => Promise<void>
}) {
    const [helpOpen, setHelpOpen] = useState(false)
    const metadata = state.metadata
    const annotationId = useId()
    const descriptions = [
        ...metadata.warnings.map((_, index) => `${annotationId}-warning-${index}`),
        ...(state.recovered ? [`${annotationId}-recovered`] : []),
    ]
    const address = new URL(metadata.url).hostname
    async function copyLink(event: MouseEvent<HTMLButtonElement>) {
        event.currentTarget.focus()
        try {
            await CopyToClipboard(metadata.url)
            onAnnounce(state.session.sessionId, copy.copy.confirmation)
        } catch { onCopyFailed(state.session.sessionId) }
    }
    return <section className="fd-region" data-phase-view="text-staged">
        <div className="fd-staged-head">
            <h1 className="fd-state-heading" tabIndex={-1} data-focus-target="staged-heading"
                aria-describedby={descriptions.length === 0 ? undefined : descriptions.join(' ')}>{copy.text.stagedHeading}</h1>
            <p className="fd-meta">{copy.text.stagedInstruction}</p>
            {state.recovered ? <span id={`${annotationId}-recovered`} className="sr-only">{copy.recovery.success}</span> : null}
        </div>
        <div className="fd-packet fd-text-packet">
            <div className="fd-qr-panel"><img className="fd-qr" src={`data:image/png;base64,${metadata.qrBase64}`}
                alt={copy.text.qrAlt} draggable={false}/></div>
            <div className="fd-hero__details">
                <strong className="fd-subheading">{copy.text.kind}</strong>
                <p className="fd-body">{copy.text.byteLength(metadata.byteLength)}</p>
                <p className="fd-body">{copy.text.once}</p>
                <label className="fd-subheading" htmlFor="fd-text-link">{copy.text.linkLabel}</label>
                <input id="fd-text-link" className="fd-text-field" readOnly value={metadata.url}/>
                <button type="button" className="fd-button fd-button--secondary fd-target"
                    onClick={event => void copyLink(event)} disabled={state.cancelPending}>{copy.directLink.action}</button>
                {state.commandError === null ? null : <p className="fd-text-error" data-focus-target="command-error" tabIndex={-1}>{state.commandError.message}</p>}
                {metadata.warnings.map((warning, index) => <p key={index} id={`${annotationId}-warning-${index}`}
                    className="fd-warning-banner">{warning.message}</p>)}
                <p className="fd-meta">{copy.network.disclosure}</p>
            </div>
        </div>
        <div className="fd-text-actions">
            <button type="button" className="fd-button fd-button--secondary fd-target" aria-expanded={helpOpen}
                onClick={() => setHelpOpen(open => !open)}>{copy.help.heading}</button>
            <button type="button" className="fd-button fd-button--quiet fd-target" onClick={onCancel}
                disabled={state.cancelPending}>{state.cancelPending ? copy.cancel.pending : copy.cancel.action}</button>
        </div>
        {helpOpen ? <ConnectionRecoveryPanel active={helpOpen} sessionId={state.session.sessionId}
            currentAddress={address} onRefresh={onRefreshConnection}/> : null}
    </section>
}

export function TextTransferringView({state, onCancel}: {
    readonly state: TextTransferringTransferState
    readonly onCancel: () => void
}) {
    return <section className="fd-region fd-packet" data-phase-view="text-transferring">
        <h1 className="fd-state-heading" tabIndex={-1} data-focus-target="transferring-heading">{copy.text.sendingHeading}</h1>
        <p className="fd-body">{copy.text.sendingBody}</p>
        <button type="button" className="fd-button fd-button--quiet fd-target" onClick={onCancel}
            disabled={state.cancelPending}>{state.cancelPending ? copy.cancel.pending : copy.cancel.action}</button>
        {state.commandError === null ? null : <p className="fd-text-error" data-focus-target="command-error" tabIndex={-1}>{state.commandError.message}</p>}
    </section>
}
