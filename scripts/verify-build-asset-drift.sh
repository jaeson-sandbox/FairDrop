#!/usr/bin/env bash
set -euo pipefail

if [ "${1:-}" = "--prove-missing-ico" ]; then
  # This invokes Wails after deliberately removing a tracked input. CI's checkout
  # is disposable; local callers must make the same isolation explicit.
  if [ "${GITHUB_ACTIONS:-}" != "true" ] && [ "${FAIRDROP_MUTATION_DISPOSABLE:-}" != "1" ]; then
    echo "missing-ICO proof requires a disposable checkout; set FAIRDROP_MUTATION_DISPOSABLE=1 only inside one" >&2
    exit 1
  fi
  icon=build/windows/icon.ico
  snapshot="$(mktemp "${TMPDIR:-/tmp}/fairdrop-icon.XXXXXX.ico")"
  index_snapshot="$(mktemp "${TMPDIR:-/tmp}/fairdrop-index.XXXXXX")"
  cp "$icon" "$snapshot"
  cp "$(git rev-parse --git-path index)" "$index_snapshot"
  export GIT_INDEX_FILE="$index_snapshot"
  restore() {
    cp "$snapshot" "$icon"
    rm -f "$snapshot" "$index_snapshot"
  }
  trap restore EXIT INT TERM
  git update-index --force-remove "$icon"
  rm "$icon"
  wails build
  if [ ! -f "$icon" ]; then
    echo "wails build did not regenerate $icon; drift rejection was not exercised" >&2
    exit 1
  fi
  set +e
  output="$(bash "$0" 2>&1)"
  verdict=$?
  set -e
  printf '%s\n' "$output"
  if [ "$verdict" -eq 0 ]; then
    echo "missing committed icon mutation passed: drift check accepted the manufactured asset" >&2
    exit 1
  fi
  if ! printf '%s\n' "$output" | grep -q 'build/windows/icon.ico'; then
    echo "drift check failed without naming build/windows/icon.ico" >&2
    exit 1
  fi
  echo "missing committed icon mutation was rejected by the drift check"
  exit 0
fi

# Wails manufactures a missing committed build asset. Include untracked files
# so a deleted icon rebuilt as the scaffold placeholder cannot satisfy CI.
# build/bin is the expected ignored product output.
drifted="$(git status --porcelain --untracked-files=all -- build/ | grep -v ' build/bin/' || true)"
if [ -n "$drifted" ]; then
  echo "wails build changed or manufactured a committed build asset:" >&2
  echo "$drifted" >&2
  exit 1
fi
