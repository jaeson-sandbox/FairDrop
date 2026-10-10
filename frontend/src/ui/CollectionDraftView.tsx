import {useEffect, useRef} from 'react'
import type {CSSProperties} from 'react'
import {copy} from './copy'
import type {PublicError} from '../transfer/types'

interface Props {
    readonly names: readonly string[]
    readonly error: PublicError | null
    readonly dropTargetStyle: CSSProperties
    readonly onAddFiles: () => void
    readonly onAddFolder: () => void
    readonly onRemove: (index: number) => void
    readonly onSend: () => void
    readonly onCancel: () => void
}

export function CollectionDraftView({names, error, dropTargetStyle, onAddFiles, onAddFolder, onRemove, onSend, onCancel}: Props) {
    const headingRef = useRef<HTMLHeadingElement>(null)
    const addFilesRef = useRef<HTMLButtonElement>(null)
    const removeRefs = useRef<Array<HTMLButtonElement | null>>([])
    useEffect(() => { headingRef.current?.focus() }, [])

    function remove(index: number): void {
        onRemove(index)
        queueMicrotask(() => {
            const adjacent = removeRefs.current[Math.min(index, names.length - 2)]
            ;(adjacent ?? addFilesRef.current)?.focus()
        })
    }

    return (
        <div className="fd-region" data-phase-view="draft" style={dropTargetStyle}>
            <section className="fd-draft fd-packet" aria-labelledby="fd-draft-heading">
                <h1 className="fd-state-heading" id="fd-draft-heading" ref={headingRef} tabIndex={-1}
                    aria-describedby={error ? 'fd-draft-error' : undefined}>{copy.selection.heading}</h1>
                <p className="fd-meta" aria-live="polite">{copy.selection.count(names.length)}</p>
                {error ? <p className="fd-draft__error" id="fd-draft-error" role="status">{error.message}</p> : null}
                <ol className="fd-draft__list">
                    {names.map((name, index) => (
                        <li className="fd-draft__row" key={`${index}-${name}`}>
                            <span className="fd-draft__ordinal" aria-hidden="true">{index + 1}.</span>
                            <span className="fd-draft__name"><bdi dir="auto">{name}</bdi></span>
                            <button
                                type="button" className="fd-button fd-button--quiet fd-target"
                                ref={element => { removeRefs.current[index] = element }}
                                aria-label={`${copy.selection.remove} ${index + 1}. ${name}`}
                                onClick={() => remove(index)}
                            >{copy.selection.remove}</button>
                        </li>
                    ))}
                </ol>
                <div className="fd-draft__actions">
                    <button type="button" ref={addFilesRef} className="fd-button fd-button--secondary fd-target" onClick={onAddFiles}>{copy.selection.addFiles}</button>
                    <button type="button" className="fd-button fd-button--secondary fd-target" onClick={onAddFolder}>{copy.selection.addFolder}</button>
                    <button type="button" className="fd-button fd-button--primary fd-target" disabled={names.length === 0} onClick={onSend}>{copy.selection.send}</button>
                    <button type="button" className="fd-button fd-button--quiet fd-target" onClick={onCancel}>{copy.selection.cancel}</button>
                </div>
            </section>
        </div>
    )
}
