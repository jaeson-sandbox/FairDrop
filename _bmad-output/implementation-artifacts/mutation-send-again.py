"""Six scoped Send Again mutations. Run from the repository root with python3.

Each case first requires one named real-hook baseline pass, then changes only
useTransfer.ts and requires that same case to fail with an assertion. Complete
Vitest JSON reports go to the OS temporary directory under
fairdrop-send-again-mutation-<case>-{baseline,mutant}.json. The exact original
source bytes are restored in a finally block. The inventory below
is the complete set of six cases.
"""
from pathlib import Path
import json
import subprocess
import tempfile

source = Path('frontend/src/transfer/useTransfer.ts')
original_bytes = source.read_bytes()
decoded = original_bytes.decode('utf-8')
line_ending = '\r\n' if '\r\n' in decoded else '\n'
original = decoded.replace('\r\n', '\n')
log_dir = Path(tempfile.gettempdir())
cases = (
    ('retention', 'retains the remembered path on a completed Done',
     "    stateRef.current = state\n",
     "    stateRef.current = state\n    if (state.phase === 'done') rememberedPathRef.current = null\n"),
    ('state-guard', 'refuses Send Again while a selection',
     'if (!mountedRef.current || !isDone || path === null || sendAgainPendingRef.current) return',
     'if (!mountedRef.current || path === null || sendAgainPendingRef.current) return'),
    ('admission', 'waits for actual reset after releasing a live Done lease',
     'if (!mountedRef.current || !isDone || path === null || sendAgainPendingRef.current) return',
     'if (!mountedRef.current || !isDone || path === null) return'),
    ('abandonment', 'forgets on user Done, cancellation, replacement, and unmount',
     'selectionEpochRef.current !== epoch ||\n                rememberedPathRef.current !== path || ',
     ''),
    ('release-recovery', 'settles a failed terminal lease release without staging',
     'if (await cancelImpl(true) === false) return',
     'await cancelImpl(true)'),
    ('live-chooser', 'preserves the live Done target through Send Another',
     'cancelImpl(true)', 'cancelImpl(false)'),
)
def run_case(name: str, test: str, suffix: str) -> tuple[subprocess.CompletedProcess[str], dict]:
    result = subprocess.run(
        ['node', 'node_modules/vitest/vitest.mjs', 'run',
         'src/transfer/useTransfer.test.tsx', '-t', test, '--reporter=json'],
        cwd='frontend', capture_output=True, text=True, encoding='utf-8',
        timeout=30, check=False,
    )
    log = log_dir / f'fairdrop-send-again-mutation-{name}-{suffix}.json'
    log.write_text(result.stdout + result.stderr, encoding='utf-8')
    report = json.loads(result.stdout)
    return result, report


def selected(report: dict, test: str, status: str) -> list[dict]:
    return [assertion for suite in report['testResults']
            for assertion in suite['assertionResults']
            if test in assertion['fullName'] and assertion['status'] == status]


try:
    for name, test, old, new in cases:
        baseline, baseline_report = run_case(name, test, 'baseline')
        if (baseline.returncode != 0 or baseline_report['numPassedTests'] != 1 or
                baseline_report['numFailedTests'] != 0 or
                len(selected(baseline_report, test, 'passed')) != 1):
            raise RuntimeError(f'{name}: named baseline did not pass alone')
        start = 0
        end = len(original)
        if name == 'release-recovery':
            start = original.index('const sendAgain = useCallback')
            end = original.index('    const retry =', start)
        elif name == 'live-chooser':
            start = original.index('const selectFromOutcome = useCallback')
            end = original.index('    const sendAgain =', start)
        segment = original[start:end]
        if segment.count(old) != 1:
            raise RuntimeError(f'{name}: expected one mutation anchor, found {segment.count(old)}')
        mutation = original[:start] + segment.replace(old, new, 1) + original[end:]
        source.write_bytes(mutation.replace('\n', line_ending).encode('utf-8'))
        mutant, mutant_report = run_case(name, test, 'mutant')
        failures = selected(mutant_report, test, 'failed')
        killed = (mutant.returncode != 0 and mutant_report['numFailedTests'] == 1 and
                  mutant_report['numPassedTests'] == 0 and len(failures) == 1 and
                  any('AssertionError' in message for message in failures[0]['failureMessages']))
        baseline_log = log_dir / f'fairdrop-send-again-mutation-{name}-baseline.json'
        mutant_log = log_dir / f'fairdrop-send-again-mutation-{name}-mutant.json'
        print(f'{name}: baseline=pass; named_assertion_failure={killed}; '
              f'logs={baseline_log},{mutant_log}')
        if not killed:
            raise RuntimeError(f'{name}: no structured named assertion failure')
        source.write_bytes(original_bytes)
finally:
    source.write_bytes(original_bytes)
