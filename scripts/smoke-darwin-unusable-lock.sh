#!/usr/bin/env bash
# Touch the real Foundation lock only on the isolated hosted runner, never a
# developer's machine. The Go harness locks it and restores its exact mode.
set -euo pipefail
cd "$(dirname "$0")/.."
if [[ "${GITHUB_ACTIONS:-}" != true ]]; then
  echo "Native lock smoke requires an isolated GitHub Actions runner" >&2
  exit 1
fi
if ! result="$(FAIRDROP_NATIVE_APP_SMOKE=1 go test -count=1 -timeout 45s -v -run '^TestDarwinBuiltAppSurvivesUnusableLock$' . 2>&1)"; then
  printf '%s\n' "$result"
  exit 1
fi
printf '%s\n' "$result"
# A skipped, renamed or unregistered test must not make this step green.
if ! grep -F 'built native process survives unusable lock; window visibility is unobserved' <<< "$result" >/dev/null; then
  echo "Native launch smoke did not execute its survival assertion" >&2
  exit 1
fi
