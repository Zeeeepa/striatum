#!/usr/bin/env bash
# Block until a striatum run reaches a terminal state, printing state
# transitions. Exits 0 on completed, 1 on failed/canceled.
set -euo pipefail

RUN_ID="${1:?usage: wait-run.sh <run-id> [interval-seconds] [target-repo]}"
INTERVAL="${2:-60}"
TARGET="${3:-$PWD}"

LAST=""
while :; do
  SUMMARY="$(striatum --repo "$TARGET" run summary --run-id "$RUN_ID" --json 2>/dev/null || true)"
  STATE="$(printf '%s' "$SUMMARY" | python3 -c '
import json, sys
try:
    doc = json.load(sys.stdin)
except Exception:
    print("")
    sys.exit(0)
for probe in (doc.get("run"), doc, (doc.get("runs") or [None])[0]):
    if isinstance(probe, dict) and probe.get("state"):
        print(probe["state"])
        break
else:
    print("")
' 2>/dev/null || echo "")"

  if [ "$STATE" != "$LAST" ]; then
    echo "$(date -u +%H:%M:%SZ) run $RUN_ID: ${STATE:-unreachable}"
    LAST="$STATE"
  fi
  case "$STATE" in
    completed) exit 0 ;;
    failed|canceled) exit 1 ;;
  esac
  sleep "$INTERVAL"
done
