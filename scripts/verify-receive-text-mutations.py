#!/usr/bin/env python3
"""Canonical scoped receive-text Go mutation inventory; run in an isolated copy."""

import argparse
import os
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parent.parent
CASES = (
    ('bounded-reader', 'internal/server/receive_text.go', './internal/server',
     'TestReceiveTextEncodedReaderStopsAtLiteralCapPlusOne', 'encoded body read=',
     'io.ReadAll(io.LimitReader(request.Body, maxReceiveTextFormBytes+1))',
     'io.ReadAll(request.Body)'),
    ('encoded-cap', 'internal/server/receive_text.go', './internal/server',
     'TestReceiveTextRejectsMalformedBodiesWithoutSpendingCode', 'chunked encoded oversize status=400',
     'if len(body) > maxReceiveTextFormBytes {', 'if false && len(body) > maxReceiveTextFormBytes {'),
    ('one-reader', 'internal/server/receive_text.go', './internal/server',
     'TestReceiveTextOneBodyReaderAndInterruptedBodyDoNotClaim', 'competing POST = 400',
     'if !r.processing.CompareAndSwap(false, true) {',
     'if false && !r.processing.CompareAndSwap(false, true) {'),
    ('post-authorization-cancel', 'internal/server/receive_text.go', './internal/server',
     'TestReceiveTextCancelledDuringAuthorizationCannotCommit', 'cancelled post = 200',
     'if request.Context().Err() != nil {\n\t\twriteStatus(w, http.StatusNotFound)\n\t\tif r.ctx.Err() == nil {',
     'if false && request.Context().Err() != nil {\n\t\twriteStatus(w, http.StatusNotFound)\n\t\tif r.ctx.Err() == nil {'),
    ('request-parented-claim-context', 'internal/server/receive_text.go', './internal/server',
     'TestReceiveTextCancelledDuringAuthorizationCannotCommit', 'claim context did not inherit the HTTP request',
     'context.WithCancel(request.Context())', 'context.WithCancel(r.ctx)'),
    ('stale-body', 'internal/server/receive_text.go', './internal/server',
     'TestReceiveTextRetiredWhileBodyIsBlockedCannotCommit', 'retired request status=200',
     'if err != nil || request.Context().Err() != nil || r.ctx.Err() != nil ||',
     'if err != nil || request.Context().Err() != nil || false ||'),
    ('no-payload-echo', 'internal/server/receive_text.go', './internal/server',
     'TestReceiveTextRealFormEncodingCommitsExactNormalizedValueOnce', 'receipt status=',
     'writeReceiveTextPage(w, http.StatusOK, receiveTextSuccess)',
     'writeReceiveTextPage(w, http.StatusOK, receiveTextSuccess+value)'),
    ('no-empty-success', 'internal/transfer/outcomes.go', './internal/transfer',
     'TestReceiveTextServerCompleteWithoutCommitBecomesFailure', 'empty completion was accepted',
     'event.Kind == ServerComplete && live.receiveText.value == nil {',
     'false && event.Kind == ServerComplete && live.receiveText.value == nil {'),
    ('held-receipt', 'internal/transfer/outcomes.go', './internal/transfer',
     'TestReceiveTextCommitReadHeldOutcomeAndDoneClear', 'outcome not held',
     'if live.receive == nil && live.receiveText == nil {',
     'if live.receive == nil {'),
    ('claim-caller-cancel', 'internal/transfer/coordinator.go', './internal/transfer',
     'TestReceiveTextCallerCancellationAtClaimCommitLeavesSessionReusable', 'cancelled claim left state',
     'if ctx.Err() != nil && c.session == live && c.state == stateClaiming && !live.cancelled && !c.closing && live.ctx.Err() == nil {',
     'if false && ctx.Err() != nil && c.session == live && c.state == stateClaiming && !live.cancelled && !c.closing && live.ctx.Err() == nil {'),
    ('getter-current-identity', 'internal/transfer/coordinator_receive_text.go', '.',
     'TestNativeReceiveTextFormReachesHeldDesktopValue', 'stale read',
     'c.session.id != id || ', ''),
)
EXTRA_PATCHES = {
    'stale-body': (
        'if request.Context().Err() != nil || r.ctx.Err() != nil {\n\t\twriteStatus(w, http.StatusNotFound)\n\t\treturn\n\t}\n\tif !r.claimed.CompareAndSwap',
        'if request.Context().Err() != nil {\n\t\twriteStatus(w, http.StatusNotFound)\n\t\treturn\n\t}\n\tif !r.claimed.CompareAndSwap',
    ),
}


def verify(package: str, name: str, log: Path, mode: str, assertion: str = '') -> None:
    command = ['go', 'test', '-count=1', '-json', '-timeout', '60s', '-run', f'^{name}$', package]
    with log.open('w', encoding='utf-8') as stream:
        result = subprocess.run(command, cwd=ROOT, stdout=stream, stderr=subprocess.STDOUT, check=False)
    verdict = ['go', 'run', './scripts/mutationverdict', '-mode', mode, '-test', name,
               '-status', str(result.returncode), '-log', str(log)]
    if assertion:
        verdict.extend(['-assert', assertion])
    subprocess.run(verdict, cwd=ROOT, check=True)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument('--log-dir', type=Path, required=True)
    args = parser.parse_args()
    if os.environ.get('FAIRDROP_MUTATION_ISOLATED') != '1':
        parser.error('set FAIRDROP_MUTATION_ISOLATED=1 in an isolated source copy')
    args.log_dir.mkdir(parents=True, exist_ok=True)
    for label, filename, package, name, assertion, before, after in CASES:
        source = ROOT / filename
        original = source.read_text(encoding='utf-8').replace('\r\n', '\n')
        if original.count(before) != 1:
            raise RuntimeError(f'{label}: source anchor count {original.count(before)}')
        verify(package, name, args.log_dir / f'{label}-baseline.json', 'baseline')
        mutated = original.replace(before, after, 1)
        if label in EXTRA_PATCHES:
            extra_before, extra_after = EXTRA_PATCHES[label]
            if mutated.count(extra_before) != 1:
                raise RuntimeError(f'{label}: extra source anchor count {mutated.count(extra_before)}')
            mutated = mutated.replace(extra_before, extra_after, 1)
        try:
            source.write_text(mutated, encoding='utf-8')
            verify(package, name, args.log_dir / f'{label}-mutation.json', 'mutation', assertion)
        finally:
            source.write_text(original, encoding='utf-8')
        print(f'KILLED: {label} by {name}', flush=True)


if __name__ == '__main__':
    main()
