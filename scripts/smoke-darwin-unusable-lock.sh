#!/usr/bin/env bash
# Observe an actual native binary remaining alive with degraded lock protection.
# This proves process launch and its diagnostic, not visual window presentation.
set -euo pipefail
cd "$(dirname "$0")/.."
scratch="$(mktemp -d)"
clang -framework Foundation scripts/native-temp-dir.m -o "$scratch/native-temp-dir"
native_temp="$(TMPDIR="$scratch/" "$scratch/native-temp-dir")"
if [[ ! -d "$native_temp" ]] || [[ "$(cd "$native_temp" && pwd -P)" != "$(cd "$scratch" && pwd -P)" ]]; then
  echo "Foundation ignored the isolated TMPDIR; no native lock fixture was modified" >&2
  exit 1
fi
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
# This is our blank CI application, with no selection or transfer. Retain its
# full startup output and a bounded sample instead of losing the failure cause.
echo "Controlled blank-app startup output:" >&2
cat "$scratch/launch.log" >&2
sample "$child" 1 -file "$scratch/sample.txt" >/dev/null 2>&1 || true
if [[ -f "$scratch/sample.txt" ]]; then cat "$scratch/sample.txt" >&2; fi
exit 1
