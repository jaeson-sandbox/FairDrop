import {memo, useCallback, useEffect, useRef} from 'react'
import type {CSSProperties} from 'react'
import {copy} from './copy'
import type {PublicError} from '../transfer/types'

interface Props {
    readonly names: readonly string[]
    /**
     * One stable id per row, assigned when the row is appended and never reused.
     * Rows are keyed by it, so removing one row leaves every other row mounted.
     * Without it a row falls back to its position, which remounts every later row.
     */
    readonly ids?: readonly number[]
    readonly error: PublicError | null
    readonly dropTargetStyle: CSSProperties
    readonly onAddFiles: () => void
    readonly onAddFolder: () => void
    readonly onRemove: (index: number) => void
    readonly onSend: () => void
    readonly onCancel: () => void
}

interface RowProps {
    readonly rowKey: number
    readonly index: number
    readonly name: string
    readonly onRemove: (index: number) => void
    readonly register: (key: number, element: HTMLButtonElement | null) => void
}

// A row re-renders only when its own position or name changes, so appending to
// or removing from a 10,000-row list does not re-render the rows it leaves alone.
const DraftRow = memo(function DraftRow({rowKey, index, name, onRemove, register}: RowProps) {
    return (
        <li className="fd-draft__row">
            <span className="fd-draft__ordinal" aria-hidden="true">{index + 1}.</span>
            <span className="fd-draft__name"><bdi dir="auto">{name}</bdi></span>
            <button
                type="button" className="fd-button fd-button--quiet fd-target"
                ref={element => register(rowKey, element)}
                aria-label={`${copy.selection.remove} ${index + 1}. ${name}`}
                onClick={() => onRemove(index)}
            >{copy.selection.remove}</button>
        </li>
    )
})

export function CollectionDraftView({names, ids, error, dropTargetStyle, onAddFiles, onAddFolder, onRemove, onSend, onCancel}: Props) {
    const headingRef = useRef<HTMLHeadingElement>(null)
    const addFilesRef = useRef<HTMLButtonElement>(null)
    const removeButtons = useRef<Map<number, HTMLButtonElement>>(new Map())
    const keys = ids !== undefined && ids.length === names.length ? ids : names.map((_, index) => index)
    const latest = useRef({keys, onRemove})
    latest.current = {keys, onRemove}
    useEffect(() => { headingRef.current?.focus() }, [])

    const register = useCallback((key: number, element: HTMLButtonElement | null) => {
        if (element === null) removeButtons.current.delete(key)
        else removeButtons.current.set(key, element)
    }, [])

    const remove = useCallback((index: number) => {
        const {keys: current, onRemove: removeAt} = latest.current
        // Focus moves to the row that takes this one's place, else the one before it.
        const successor = current[index + 1] ?? current[index - 1]
        removeAt(index)
        queueMicrotask(() => {
            const adjacent = successor === undefined ? undefined : removeButtons.current.get(successor)
            ;(adjacent ?? addFilesRef.current)?.focus()
        })
    }, [])

    // Every ordinal is sized to the widest one ("10000." at 10,000 rows) so a
    // long number can never run into the name beside it.
    const listStyle = {'--fd-ordinal-digits': Math.max(2, String(names.length).length)} as CSSProperties

    return (
        <div className="fd-region" data-phase-view="draft" style={dropTargetStyle}>
            <section className="fd-draft fd-packet" aria-labelledby="fd-draft-heading">
                <h1 className="fd-state-heading" id="fd-draft-heading" ref={headingRef} tabIndex={-1}
                    aria-describedby={error ? 'fd-draft-error' : undefined}>{copy.selection.heading}</h1>
                <p className="fd-meta" aria-live="polite">{copy.selection.count(names.length)}</p>
                {error ? <p className="fd-draft__error" id="fd-draft-error" role="status">{error.message}</p> : null}
                <ol className="fd-draft__list" style={listStyle}>
                    {names.map((name, index) => (
                        <DraftRow key={keys[index]} rowKey={keys[index]} index={index} name={name} onRemove={remove} register={register}/>
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
