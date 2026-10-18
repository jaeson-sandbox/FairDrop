#!/usr/bin/env python3
"""Canonical scoped receive-text UI mutation inventory with per-case logs."""

import argparse
from pathlib import Path
import re
import shutil
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]
CASES = (
    ('copy-owned-by-session', 'src/transfer/useTransfer.ts', 'src/transfer/receiveText.test.tsx',
     "receiveTextEpochRef.current += 1\n                receiveTextCopyPendingRef.current = null\n                receiveTextFetchPendingRef.current = null\n                dispatch({type: 'receive-text-done-requested'",
     "receiveTextEpochRef.current += 1\n                // injected: old copy retains the global slot\n                receiveTextFetchPendingRef.current = null\n                dispatch({type: 'receive-text-done-requested'",
     'fences old copy across Done and a new receipt while allowing the new copy'),
    ('obsolete-fetch', 'src/transfer/useTransfer.ts', 'src/transfer/receiveText.test.tsx',
     'if (!mountedRef.current || receiveTextEpochRef.current !== epoch) return\n            const latest = stateRef.current',
     'if (!mountedRef.current) return\n            const latest = stateRef.current',
     'ignores an obsolete fetch while a newer read of the same receipt is pending'),
    ('failed-done-copy', 'src/transfer/useTransfer.ts', 'src/transfer/receiveText.test.tsx',
     "dispatch({type: 'receive-text-done-failed', sessionId: terminal.sessionId})",
     'void 0 // injected: failed Done leaves copying locked',
     'keeps a failed Done on a pending copy usable and fences its late completion'),
    ('failed-done-fetch', 'src/transfer/useTransfer.ts', 'src/transfer/receiveText.test.tsx',
     "dispatch({type: 'receive-text-done-failed', sessionId: terminal.sessionId})",
     'void 0 // injected: failed Done leaves loading locked',
     'keeps a failed Done on a pending fetch recoverable and refuses the obsolete value'),
    ('app-entry', 'src/App.tsx', 'src/App.receiveText.integration.test.tsx',
     'onReceiveText={() => void transfer.receiveText()}', 'onReceiveText={() => undefined}',
     'opens the real Receive text controller from Idle and shows its waiting code'),
    ('late-link-copy', 'src/App.tsx', 'src/App.receive.test.tsx',
     'const current = previousStateRef.current\n        // The session, not just the phase:',
     "const current = previousStateRef.current\n        if (current.phase === 'receive-text-outcome' && current.session.sessionId === sessionId) { announce(text); return }\n        // The session, not just the phase:",
     'does not announce a late Copy Link success after the same session receives text'),
    ('live-unmount', 'src/transfer/useTransfer.ts', 'src/transfer/receiveText.test.tsx',
     "live.phase === 'receive-text-waiting' || live.phase === 'receive-text-receiving' ||",
     "false && live.phase === 'receive-text-waiting' || false && live.phase === 'receive-text-receiving' ||",
     'cancels a live waiting session on direct unmount and ignores late getter work'),
    ('done-blocks-fetch-retry', 'src/transfer/useTransfer.ts', 'src/transfer/receiveText.test.tsx',
     'current.session.sessionId !== sessionId || current.value !== null || current.donePending ||',
     'current.session.sessionId !== sessionId || current.value !== null ||',
     'does not admit a fetch retry while Done is unresolved'),
    ('retry-click-focus', 'src/ui/ReceiveTextViews.tsx', 'src/ui/ReceiveTextViews.test.tsx',
     'event.currentTarget.focus()\n                    retryFocusRef.current = true',
     '// injected: WebKit click leaves focus on body\n                    retryFocusRef.current = true',
     'keeps retry focus during a pending read and moves it to the named outcome on success'),
)


def run(frontend: Path, test: str, name: str, log: Path, mutation: bool) -> None:
    command = ['node', str(frontend / 'node_modules/vitest/vitest.mjs'), 'run', test, '-t', name]
    completed = subprocess.run(command, cwd=frontend, text=True, stdout=subprocess.PIPE,
                               stderr=subprocess.STDOUT, timeout=120, check=False)
    log.write_text(completed.stdout, encoding='utf-8')
    if mutation:
        if completed.returncode != 1 or not re.search(r'FAIL\s+.*>\s+' + re.escape(name), completed.stdout) or 'AssertionError' not in completed.stdout:
            raise RuntimeError(f'{log.name}: mutation did not fail its named assertion')
        if re.search(r'Failed to import|SyntaxError|TS\d{4}|Error: Transform failed', completed.stdout):
            raise RuntimeError(f'{log.name}: compiler/import failure is not a kill')
    elif completed.returncode != 0 or '1 passed' not in completed.stdout:
        raise RuntimeError(f'{log.name}: named baseline did not pass')


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument('--log-dir', type=Path, required=True)
    args = parser.parse_args()
    args.log_dir.mkdir(parents=True, exist_ok=True)
    for label, filename, test, before, after, name in CASES:
        with tempfile.TemporaryDirectory(prefix=f'fairdrop-receive-text-{label}-') as directory:
            frontend = Path(directory) / 'frontend'
            shutil.copytree(ROOT / 'frontend', frontend, ignore=shutil.ignore_patterns('node_modules', 'dist', 'captures'))
            frontend.joinpath('node_modules').symlink_to(ROOT / 'frontend/node_modules', target_is_directory=True)
            run(frontend, test, name, args.log_dir / f'{label}-baseline.log', False)
            target = frontend / filename
            source = target.read_text(encoding='utf-8').replace('\r\n', '\n')
            if source.count(before) != 1:
                raise RuntimeError(f'{label}: source anchor count {source.count(before)}')
            target.write_text(source.replace(before, after, 1), encoding='utf-8')
            run(frontend, test, name, args.log_dir / f'{label}-mutation.log', True)
            print(f'KILLED: {label} by {name}', flush=True)


if __name__ == '__main__':
    main()
