#!/usr/bin/env bash
# Observe an actual native binary remaining alive with degraded lock protection.
# This proves process launch and its diagnostic, not visual window presentation.
set -euo pipefail
cd "$(dirname "$0")/.."
scratch="$(mktemp -d)"
mkdir "$scratch/d1766c78-45cf-4e6d-9f04-c3700ab32024.lock"
binary="build/bin/fairdrop.app/Contents/MacOS/fairdrop"
if [[ ! -x "$binary" ]]; then
  echo "Native FairDrop app executable is missing" >&2
  exit 1
fi
TMPDIR="$scratch/" "$binary" > "$scratch/launch.log" 2>&1 &
child=$!
stop_child() {
  kill "$child" 2>/dev/null || true
  for attempt in {1..50}; do
    if ! kill -0 "$child" 2>/dev/null; then
      wait "$child" 2>/dev/null || true
      return
    fi
    sleep 0.1
  done
  kill -KILL "$child" 2>/dev/null || true
  wait "$child" 2>/dev/null || true
}
trap stop_child EXIT
diagnostic='single-instance protection unavailable (temporary lock file unusable); launching without protection'
for attempt in {1..100}; do
  if ! kill -0 "$child" 2>/dev/null; then
    echo "Native launch exited before the unusable-lock smoke completed" >&2
    exit 1
  fi
  if grep -F "$diagnostic" "$scratch/launch.log" >/dev/null; then
    sleep 2
    kill -0 "$child"
    echo "PASS: native process remains alive after its degraded-lock diagnostic; window visibility is unobserved"
    exit 0
  fi
  sleep 0.1
done
echo "Native launch did not report the degraded-lock diagnostic" >&2
exit 1
