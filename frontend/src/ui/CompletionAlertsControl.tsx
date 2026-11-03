import {copy} from './copy'

export function CompletionAlertsControl({enabled, pending, hydrated, error, onChange}: {
    enabled: boolean
    pending: boolean
    hydrated: boolean
    error: boolean
    onChange: () => void
}) {
    return <div className="fd-completion-alerts">
        <label className="fd-completion-alerts__label">
            <input type="checkbox" checked={enabled} aria-busy={pending}
                aria-disabled={pending}
                aria-describedby={pending && !hydrated ? 'completion-alert-loading' :
                    error ? 'completion-alert-error' : undefined}
                onChange={onChange}/>
            <span>{copy.completionAlerts.toggle}</span>
        </label>
        {pending && !hydrated ? <p id="completion-alert-loading">{copy.completionAlerts.loading}</p> : null}
        {error ? <p id="completion-alert-error" role="alert">{copy.completionAlerts.failed}</p> : null}
    </div>
}
