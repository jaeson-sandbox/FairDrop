import {useEffect} from 'react'
import type {CSSProperties} from 'react'
import {OnFileDrop, OnFileDropOff} from '../wailsjs/runtime/runtime'
import {useTransfer} from './transfer/useTransfer'

// Wails gates native file drops on a CSS custom property rather than a class.
// The property inherits, so every descendant of the zone is a valid drop point.
const dropTargetStyle = {'--wails-drop-target': 'drop'} as CSSProperties

function App() {
    const transfer = useTransfer()

    useEffect(() => {
        // useDropTarget=true: only fire when the drop lands inside the zone.
        OnFileDrop((_x, _y, dropped) => {
            if (!Array.isArray(dropped) || dropped.length !== 1 ||
                typeof dropped[0] !== 'string' || dropped[0].trim() === '') {
                transfer.rejectSelection()
                return
            }
            void transfer.stage(dropped[0], 'unknown')
        }, true)
        return () => OnFileDropOff()
    }, [transfer.rejectSelection, transfer.stage])

    return (
        <main
            data-transfer-phase={transfer.state.phase}
            className="flex h-full flex-col gap-6 bg-slate-900 p-8 font-[Nunito,system-ui,sans-serif] text-slate-100"
        >
            <h1 className="text-2xl font-semibold tracking-tight">FairDrop</h1>

            <div
                style={dropTargetStyle}
                className="flex flex-1 flex-col items-center justify-center gap-4 rounded-2xl border-2 border-dashed border-slate-600 p-8 text-center transition-colors [&.wails-drop-target-active]:border-sky-400 [&.wails-drop-target-active]:bg-slate-800"
            >
                <p className="text-slate-400">Drop a file or folder here</p>
            </div>
        </main>
    )
}

export default App
