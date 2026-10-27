#!/usr/bin/env python3
"""Canonical Receive Again inventory; shares the tested desktop mutation harness.

The harness uses resolved repository-local workspaces, explicit UTF-8 decoding,
ANSI-tolerant named assertion verdicts, and unique complete per-case logs.
"""
import importlib.util
from pathlib import Path

CORE = Path(__file__).with_name('verify-receive-text-frontend-mutations.py')
SPEC = importlib.util.spec_from_file_location('receive_again_mutation_core', CORE)
assert SPEC is not None and SPEC.loader is not None
core = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(core)

TEST = 'src/transfer/useTransfer.receive.test.tsx'
SOURCE = 'src/transfer/useTransfer.ts'
CASES = (
    ('admission', SOURCE, TEST,
     'parent === null || receiveAgainPendingRef.current || showFolderPendingRef.current || activeCancelRef.current !== null',
     'parent === null || showFolderPendingRef.current',
     'waits for actual reset and admits one repeat with the original parent and clean metadata'),
    ('actual-reset', SOURCE, TEST,
     'if (!await waitForReceiveAgainIdle()) {', 'if (false) {',
     'waits for actual reset and admits one repeat with the original parent and clean metadata'),
    ('reset-bound', SOURCE, TEST,
     '            }, 15_000)', '            }, 30_000)',
     'bounds a missing repeat reset, releases its waiter and keeps the held success retryable'),
    ('original-parent', SOURCE, TEST,
     'await startReceiveAt(parent)', 'await startReceiveAt(current.destination)',
     'waits for actual reset and admits one repeat with the original parent and clean metadata'),
    ('waiting-state-guard', SOURCE, TEST,
     "current.phase !== 'receive-outcome' || current.outcome.kind !== 'complete' ||\n            parent === null || receiveAgainPendingRef.current",
     'parent === null || receiveAgainPendingRef.current',
     'refuses repeat outside successful file outcomes'),
    ('incomplete-eligibility', SOURCE, TEST,
     "canReceiveAgain: receiveParentRef.current !== null && state.phase === 'receive-outcome' && state.outcome.kind === 'complete'",
     "canReceiveAgain: state.phase === 'receive-outcome'",
     'never enables Receive Again on a held incomplete outcome'),
    ('cancelled-eligibility', SOURCE, TEST,
     "canReceiveAgain: receiveParentRef.current !== null && state.phase === 'receive-outcome' && state.outcome.kind === 'complete'",
     "canReceiveAgain: state.phase === 'receive-outcome'",
     'never enables Receive Again on a held cancelled outcome'),
    ('text-eligibility', SOURCE, TEST,
     "canReceiveAgain: receiveParentRef.current !== null && state.phase === 'receive-outcome' && state.outcome.kind === 'complete'",
     'canReceiveAgain: true',
     'never enables Receive Again on a received text outcome'),
    ('send-eligibility', SOURCE, TEST,
     "canReceiveAgain: receiveParentRef.current !== null && state.phase === 'receive-outcome' && state.outcome.kind === 'complete'",
     'canReceiveAgain: true',
     'never enables Receive Again on a successful send outcome'),
    ('user-abandonment-clears', SOURCE, TEST,
     'if (!releaseForOutcome) {\n            receiveParentRef.current = null',
     'if (!releaseForOutcome) {\n            // injected: retain abandoned parent',
     'forgets Receive Again after failed user Done even while the successful outcome stays held'),
    ('stale-work-fence', SOURCE, TEST,
     "if (!mountedRef.current || selectionEpochRef.current !== epoch ||\n                receiveParentRef.current !== parent || stateRef.current.phase !== 'idle') return",
     "if (!mountedRef.current || stateRef.current.phase !== 'idle') return",
     'forgets the parent on Done and fences an already waiting repeat'),
    ('replacement-owner-finalizer', SOURCE, TEST,
     "if (!admitted && stateRef.current.phase === 'idle' &&\n                selectionEpochRef.current === epoch && receiveParentRef.current === parent) receiveParentRef.current = null",
     "if (!admitted && stateRef.current.phase === 'idle') receiveParentRef.current = null",
     'keeps a newly admitted parent when an obsolete repeat finalizes before its pending render'),
    ('show-done-lock', SOURCE, TEST,
     "if (current.phase === 'receive-outcome' && showFolderPendingRef.current) return false",
     '// injected: Done overlaps Show',
     'ignores Done while Show owns the outcome without forgetting its repeat parent'),
    ('app-show-lock', 'src/App.tsx', 'src/App.receiveAgain.test.tsx',
     'onShowInFolder={() => void receive.runOutcomeAction(transfer.showReceivedFolder)}',
     'onShowInFolder={() => void transfer.showReceivedFolder()}',
     'locks all outcome controls during Show and restores its initiating focus after success'),
    ('app-repeat-entry', 'src/App.tsx', 'src/App.receiveAgain.test.tsx',
     'onReceiveAgain={transfer.canReceiveAgain ? () => void receive.runOutcomeAction(transfer.receiveAgain) : undefined}',
     'onReceiveAgain={transfer.canReceiveAgain ? () => undefined : undefined}',
     'clicks Receive Again through the real controller, blocks overlapping actions and focuses the fresh code without duplicate speech'),
    ('app-outcome-busy', 'src/App.tsx', 'src/App.receiveAgain.test.tsx',
     'busy={receive.busy}', 'busy={false}',
     'clicks Receive Again through the real controller, blocks overlapping actions and focuses the fresh code without duplicate speech'),
    ('show-focus-capture', 'src/ui/ReceiveOutcomeView.tsx', 'src/ui/ReceiveOutcomeView.test.tsx',
     'event.currentTarget.focus()\n                            showFocusRef.current',
     '// injected: pointer does not focus on WebKit\n                            showFocusRef.current',
     'restores the Show initiating control after disabling it without relying on pointer focus'),
    ('show-focus-restore', 'src/ui/ReceiveOutcomeView.tsx', 'src/ui/ReceiveOutcomeView.test.tsx',
     "pending.button.setAttribute('data-focus-return', '')\n        pending.button.focus()",
     "pending.button.setAttribute('data-focus-return', '')",
     'restores the Show initiating control after disabling it without relying on pointer focus'),
    ('retirement-error-purpose', 'src/ui/ReceiveOutcomeView.tsx', 'src/ui/ReceiveOutcomeView.test.tsx',
     "state.commandErrorPurpose === 'retire' ? copy.receive.retireFailed : copy.receive.showFailed",
     'copy.receive.showFailed',
     'renders truthful fixed retirement error separately from Show in Folder failure'),
)
core.CASES = CASES

if __name__ == '__main__':
    core.main()
