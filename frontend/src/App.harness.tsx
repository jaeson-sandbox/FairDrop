// Shared App-mounting mechanics for App.test.tsx and App.focus.test.tsx.
//
// Not matched by the vitest test glob (src/star-star/star.test.{ts,tsx}), so
// this file contributes no tests of its own -- only helpers both suites call.
//
// Before this file, each suite redefined its own controller stub with its own
// field list (App.test.tsx's mountWith/controllerFor, App.focus.test.tsx's
// mountWith/transitionTo), so the two drifted as the controller gained
// methods -- D-072. Each stub was an object literal handed to a vi.fn()'s
// mockReturnValue, which is untyped, so adding a controller member produced no
// error anywhere. Typing the shape here as ReturnType<typeof useTransfer>
// turns that silence into a compile error naming the missing member -- verified
// by adding a required `retryLast` to TransferController, which then failed
// tsc in both suites' `commands` objects and in neither before.
//
// The mock functions themselves stay in each test file and are passed in:
// this module only shapes the controller object and drives the render.
import {act, render} from '@testing-library/react'
import type {ReactElement} from 'react'
import App from './App'
import type {TransferState} from './transfer/state'
import type {useTransfer} from './transfer/useTransfer'

type Controller = ReturnType<typeof useTransfer>

/** Every command the controller carries besides the state it renders. */
export type ControllerCommands = Omit<Controller, 'state'>

/** The one method both helpers below need from a `vi.fn()`-mocked useTransfer. */
export interface UseTransferMock {
    mockReturnValue(value: Controller): unknown
}

export function controllerFor(state: TransferState, commands: ControllerCommands): Controller {
    return {state, ...commands}
}

export function mountWith(
    useTransferMock: UseTransferMock,
    state: TransferState,
    commands: ControllerCommands,
): ReturnType<typeof render> {
    useTransferMock.mockReturnValue(controllerFor(state, commands))
    return render(<App/>)
}

export function transitionTo(
    useTransferMock: UseTransferMock,
    view: {rerender: (ui: ReactElement) => void},
    state: TransferState,
    commands: ControllerCommands,
): void {
    useTransferMock.mockReturnValue(controllerFor(state, commands))
    act(() => view.rerender(<App/>))
}
