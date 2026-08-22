import {useEffect, useState} from 'react'
import type {CSSProperties} from 'react'
import {OnFileDrop, OnFileDropOff} from '../wailsjs/runtime/runtime'

// Wails gates native file drops on a CSS custom property rather than a class.
// The property inherits, so every descendant of the zone is a valid drop point.
const dropTargetStyle = {'--wails-drop-target': 'drop'} as CSSProperties

function App() {
    const [paths, setPaths] = useState<string[]>([])

    useEffect(() => {
        // useDropTarget=true: only fire when the drop lands inside the zone.
        OnFileDrop((_x, _y, dropped) => setPaths(dropped), true)
        return () => OnFileDropOff()
    }, [])

    return (
        <main className="flex h-full flex-col gap-6 bg-slate-900 p-8 font-[Nunito,system-ui,sans-serif] text-slate-100">
            <h1 className="text-2xl font-semibold tracking-tight">FairDrop</h1>

            <div
                style={dropTargetStyle}
                className="flex flex-1 flex-col items-center justify-center gap-4 rounded-2xl border-2 border-dashed border-slate-600 p-8 text-center transition-colors [&.wails-drop-target-active]:border-sky-400 [&.wails-drop-target-active]:bg-slate-800"
            >
                <p className="text-slate-400">Drop a file or folder here</p>

                {paths.length > 0 && (
                    <ul className="w-full max-w-2xl space-y-1 text-left">
                        {paths.map((path) => (
                            <li
                                key={path}
                                className="truncate rounded-md bg-slate-800 px-3 py-2 font-mono text-sm text-sky-300"
                                title={path}
                            >
                                {path}
                            </li>
                        ))}
                    </ul>
                )}
            </div>
        </main>
    )
}

export default App
