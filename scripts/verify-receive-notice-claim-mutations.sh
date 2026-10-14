#!/usr/bin/env bash
# Canonical scoped inventory for receive notice/claim admission. The full
# native inventory invokes this same script, so a scoped run cannot omit cases.
set -euo pipefail
cd "$(dirname "$0")/.."
scratch="$(mktemp -d)"
target=internal/transfer/coordinator.go
cp "$target" "$scratch/coordinator.go"
restore() { cp "$scratch/coordinator.go" "$target"; }
trap restore EXIT
echo "Receive notice/claim mutation logs: $scratch"

baseline() {
  local name="$1" status=0 log="$scratch/baseline-$1.log"
  go test -count=1 -json -timeout 20s -run "^$name$" ./internal/transfer > "$log" 2>&1 || status=$?
  go run ./scripts/mutationverdict -mode baseline -test "$name" -status "$status" -log "$log"
  echo "BASELINE PASSED: $name ($log)"
}

kill_mutation() {
  local label="$1" name="$2" evidence="$3" status=0
  local log="$scratch/mutation-$label.log"
  go test -count=1 -json -timeout 20s -run "^$name$" ./internal/transfer > "$log" 2>&1 || status=$?
  go run ./scripts/mutationverdict -mode mutation -test "$name" -assert "$evidence" -status "$status" -log "$log"
  echo "KILLED: $label by $name ($log)"
  restore
}

baseline TestClaimWaitsForRefusalNoticePublication
baseline TestCompetingClaimsWaitingForOneNoticeStartOnce
baseline TestSuccessiveNoticePublicationsShareOneClaimDeadline
baseline TestClaimWaitingForNoticeHonorsCallerCancellation
baseline TestPostNoticeRevalidationAbandonsACancelledCallerClaim
baseline TestCallerAbortAtNoticeCompletionStillSettlesSpentLink
baseline TestClaimWaitingForNoticeCannotClaimReplacementAfterCancel
baseline TestClaimWaitingForNoticeHasOneBound

# Restore the original nonblocking admission path while keeping the rest of
# the implementation compilable. The overlap test must name its missing wait.
perl -0pi -e 's/noticeDone := live\.noticeDone/var noticeDone chan struct{}/ or die "pre-fix admission mutation did not match\n"' "$target"
kill_mutation pre-fix-admission TestClaimWaitsForRefusalNoticePublication 'no bound armed'

perl -0pi -e 's/claimNoticeBound = 15 \* time\.Second/claimNoticeBound = time.Nanosecond/ or die "claim bound mutation did not match\n"' "$target"
kill_mutation shrink-claim-bound TestClaimWaitingForNoticeHasOneBound 'claim wait bound ='

perl -0pi -e 's/if timedOut == nil \{\n\t\t\ttimedOut = make\(chan struct\{\}\)/if true {\n\t\t\ttimedOut = make(chan struct{})/ or die "deadline reset mutation did not match\n"' "$target"
kill_mutation reset-deadline-per-notice TestSuccessiveNoticePublicationsShareOneClaimDeadline 'claim reset deadline after second notice'

perl -0pi -e 's/return WrapError\(ErrCancelled, "the transfer was cancelled", ctx\.Err\(\)\)/return NewError(ErrTransferFailed, "the transfer was cancelled")/ or die "caller cancellation mutation did not match\n"' "$target"
kill_mutation miscode-caller-cancellation TestClaimWaitingForNoticeHonorsCallerCancellation 'want cancelled'

perl -0pi -e 's/if timedOut != nil \&\& ctx\.Err\(\) != nil \{\n\t\t\t\tc\.abandonNoticeClaim\(sessionID, generation\)/if timedOut != nil \&\& false \&\& ctx.Err() != nil {\n\t\t\t\tc.abandonNoticeClaim(sessionID, generation)/ or die "postwait cleanup mutation did not match\n"' "$target"
kill_mutation omit-postwait-abandonment TestPostNoticeRevalidationAbandonsACancelledCallerClaim 'spent waiting link'

perl -0pi -e 's/(?:live|c\.session)\.noticeClaimFailure = failure/live.noticeClaimFailure = nil/g or die "terminal outcome mutation did not match\n"' "$target"
kill_mutation omit-timed-out-outcome TestClaimWaitingForNoticeHasOneBound 'want at least 2'

perl -0pi -e 's/if live\.noticeDone != nil \{\n\t\tc\.mu\.Unlock\(\)\n\t\treturn/if true {\n\t\tc.mu.Unlock()\n\t\treturn/ or die "caller-abort settlement mutation did not match\n"' "$target"
kill_mutation omit-caller-abort-outcome TestCallerAbortAtNoticeCompletionStillSettlesSpentLink 'want at least 2'
