#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

CLONE="$TMP/striatum"
TARGET="$TMP/target-repo"

git clone --quiet "$ROOT" "$CLONE"
cd "$CLONE"

python3 -m venv .venv
.venv/bin/python -m pip install --quiet --upgrade pip
.venv/bin/python -m pip install --quiet -e ".[dev]"

RUNNER="$CLONE/.venv/bin/striatum"
"$RUNNER" --help >/dev/null

mkdir -p "$TARGET"
git -C "$TARGET" init --quiet

"$RUNNER" --repo "$TARGET" init --json >/dev/null
"$RUNNER" --repo "$TARGET" workflow validate \
  "$CLONE/examples/rfc-ledger-cleanup/workflow.json" \
  --json >/dev/null

prepare_json="$("$RUNNER" --repo "$TARGET" run prepare \
  --workflow "$CLONE/examples/rfc-ledger-cleanup/workflow.json" \
  --json)"
run_id="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["run_id"])' <<< "$prepare_json")"

"$RUNNER" --repo "$TARGET" branch confirm \
  --run-id "$run_id" \
  --branch striatum/smoke \
  --json >/dev/null
"$RUNNER" --repo "$TARGET" run start --run-id "$run_id" --json >/dev/null
"$RUNNER" --repo "$TARGET" evidence export \
  --run-id "$run_id" \
  --path docs/reviews/striatum/SMOKE_EVIDENCE.md \
  --json >/dev/null

test -f "$TARGET/docs/reviews/striatum/SMOKE_EVIDENCE.md"
grep -q "Striatum Evidence Export" "$TARGET/docs/reviews/striatum/SMOKE_EVIDENCE.md"
test -f "$TARGET/.striatum/state.sqlite3"
grep -q '^.striatum/$' "$TARGET/.gitignore"
