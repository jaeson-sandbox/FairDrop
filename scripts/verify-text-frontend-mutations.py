#!/usr/bin/env python3
"""Scoped text frontend mutations in isolated copies with named assertion verdicts."""

import argparse
from pathlib import Path
import re
import shutil
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]
TEST = 'src/transfer/text.test.tsx'
CASES = (
    ('send-again-retention', (
        ('src/transfer/useTransfer.ts', 'if (!releaseForOutcome) {', 'if (true) {'),
    ), 'keeps text across the internal live-outcome cancel used by Send Again'),
    ('explicit-cancel-forget', (
        ('src/transfer/useTransfer.ts',
         'if (!releaseForOutcome) {\n            rememberedPathsRef.current = null\n            rememberedTextRef.current = null',
         'if (!releaseForOutcome) {\n            rememberedPathsRef.current = null\n            // injected: remembered text survives explicit Cancel'),
    ), 'forgets remembered text on explicit Cancel'),
    ('unmount-cancel', (
        ('src/transfer/useTransfer.ts',
         'stageOperation.cancelRequested = true\n                void Promise.resolve().then(() => { if (!mountedRef.current) return CancelTransfer() }).catch(() => undefined)',
         'stageOperation.cancelRequested = true\n                // injected: pending text stage is not cancelled'),
    ), 'cancels a pending text stage on unmount and ignores its later acknowledgement'),
    ('stale-stage-ack', (
        ('src/transfer/useTransfer.ts',
         'return mountedRef.current && stageOperationRef.current === operation && !operation.cancelRequested',
         'return mountedRef.current && stageOperationRef.current === operation'),
        ('src/transfer/state.ts',
         "if (state.phase !== 'pending' || state.generation !== action.generation || state.cancelPending ||\n                state.itemKind !== 'text') return state",
         "if (state.phase !== 'pending' || state.generation !== action.generation || false ||\n                state.itemKind !== 'text') return state"),
    ), 'cancels an in-flight text stage and ignores its late acknowledgement'),
    ('stale-recovery-ack', (
        ('src/transfer/useTransfer.ts',
         "if (mountedRef.current) dispatch({type: 'recovery-failed', generation: current.generation,",
         "if (false) dispatch({type: 'recovery-failed', generation: current.generation,"),
        ('src/transfer/useTransfer.ts',
         'const raw = await RefreshConnection(sessionId, networkId)\n            if (!mountedRef.current || recoveryRef.current !== operation) return',
         'const raw = await RefreshConnection(sessionId, networkId)\n            if (false) return'),
        ('src/transfer/state.ts',
         'state.session.sessionId !== action.sessionId || state.cancelPending || state.mode !== action.result.mode) return state',
         'state.session.sessionId !== action.sessionId || false || state.mode !== action.result.mode) return state'),
    ), 'ignores a late text recovery acknowledgement after Cancel'),
    ('oversize-before-unsupported', (
        ('src/transfer/validation.ts',
         "if (byteLength > 65536) return {ok: false, reason: 'too-large'}",
         "if (false) return {ok: false, reason: 'too-large'}"),
    ), 'keeps the last bounded draft when an edit is oversized even with unsupported characters'),
    ('cleanup-status', (
        ('src/transfer/useTransfer.ts',
         "if (mountedRef.current && stageOperationRef.current === operation) {\n                    const error = publicError(quiesced ? 'setup_failed' : 'cleanup_unconfirmed')\n                    dispatchStageFailed(generation, error)\n                    setComposer",
         "if (mountedRef.current && stageOperationRef.current === operation) {\n                    const error = publicError('setup_failed')\n                    dispatchStageFailed(generation, error)\n                    setComposer"),
    ), 'reports cleanup uncertainty when malformed StageText metadata cannot be quiesced'),
    ('live-unmount-cancel', (
        ('src/transfer/useTransfer.ts',
         "if ((live.phase === 'text-staged' || live.phase === 'text-transferring') && !live.cancelPending) {",
         "if (false && (live.phase === 'text-staged' || live.phase === 'text-transferring') && !live.cancelPending) {"),
    ), 'cancels and forgets a live text session on unmount'),
    ('clear-stale-error-on-edit', (
        ('src/transfer/useTransfer.ts',
         "setComposer({text: value, error: null})\n        dispatch({type: 'clear-idle-command-error'})",
         "setComposer({text: value, error: null})\n        // injected: retain stale command error"),
    ), 'clears a stale Stage failure on edit so a new invalid submission owns the alert'),
    ('oversize-local-alert', (
        ('src/transfer/useTransfer.ts',
         "if (!checked.ok && checked.reason === 'too-large') {\n            dispatch({type: 'clear-idle-command-error'})",
         "if (!checked.ok && checked.reason === 'too-large') {\n            // injected: stale command error suppresses alert"),
    ), 'clears stale Stage errors for oversized edits and all selection conflicts'),
    ('stage-conflict-local-alert', (
        ('src/transfer/useTransfer.ts',
         "if (!mountedRef.current || stateRef.current.phase !== 'idle') return\n        if (composerRef.current !== null) {\n            dispatch({type: 'clear-idle-command-error'})",
         "if (!mountedRef.current || stateRef.current.phase !== 'idle') return\n        if (composerRef.current !== null) {\n            // injected: stale command error suppresses alert"),
    ), 'clears stale Stage errors for oversized edits and all selection conflicts'),
    ('browse-conflict-local-alert', (
        ('src/transfer/useTransfer.ts',
         "if (!mountedRef.current || stateRef.current.phase !== 'idle') return 'ignored'\n        if (composerRef.current !== null) {\n            dispatch({type: 'clear-idle-command-error'})",
         "if (!mountedRef.current || stateRef.current.phase !== 'idle') return 'ignored'\n        if (composerRef.current !== null) {\n            // injected: stale command error suppresses alert"),
    ), 'clears stale Stage errors for oversized edits and all selection conflicts'),
    ('drop-conflict-local-alert', (
        ('src/transfer/useTransfer.ts',
         "const rejectSelection = useCallback(() => {\n        if (composerRef.current !== null) {\n            dispatch({type: 'clear-idle-command-error'})",
         "const rejectSelection = useCallback(() => {\n        if (composerRef.current !== null) {\n            // injected: stale command error suppresses alert"),
    ), 'clears stale Stage errors for oversized edits and all selection conflicts'),
)


def run(frontend: Path, name: str, log: Path, mutation: bool) -> None:
    node = shutil.which('node')
    if node is None:
        raise RuntimeError('node is not on PATH')
    command = [node, str(frontend / 'node_modules/vitest/vitest.mjs'), 'run', TEST, '-t', name]
    completed = subprocess.run(command, cwd=frontend, universal_newlines=True, stdout=subprocess.PIPE,
                               stderr=subprocess.STDOUT, timeout=120, check=False)
    log.write_text(completed.stdout, encoding='utf-8')
    if mutation:
        named = re.search(r'FAIL\s+.*>\s+' + re.escape(name), completed.stdout)
        if completed.returncode != 1 or named is None or 'AssertionError' not in completed.stdout:
            raise RuntimeError(f'{log.name}: mutation did not fail its named assertion')
        if re.search(r'Failed to import|SyntaxError|TS\d{4}|Error: Transform failed', completed.stdout):
            raise RuntimeError(f'{log.name}: compiler/import failure is not a kill')
    elif completed.returncode != 0 or 'Test Files  1 passed' not in completed.stdout or '1 passed' not in completed.stdout:
        raise RuntimeError(f'{log.name}: named baseline did not pass')


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument('--log-dir', type=Path, required=True)
    args = parser.parse_args()
    args.log_dir.mkdir(parents=True, exist_ok=True)
    for label, patches, name in CASES:
        with tempfile.TemporaryDirectory(prefix=f'fairdrop-text-frontend-{label}-') as directory:
            frontend = Path(directory) / 'frontend'
            shutil.copytree(ROOT / 'frontend', frontend, ignore=shutil.ignore_patterns('node_modules', 'dist', 'captures'))
            frontend.joinpath('node_modules').symlink_to(ROOT / 'frontend/node_modules', target_is_directory=True)
            run(frontend, name, args.log_dir / f'{label}-baseline.log', False)
            for filename, before, after in patches:
                target = frontend / filename
                source = target.read_text(encoding='utf-8').replace('\r\n', '\n')
                if source.count(before) != 1:
                    raise RuntimeError(f'{label}: source anchor count={source.count(before)} in {filename}')
                target.write_text(source.replace(before, after, 1), encoding='utf-8')
            run(frontend, name, args.log_dir / f'{label}-mutation.log', True)
            print(f'KILLED: {label} by {name}', flush=True)


if __name__ == '__main__':
    main()
