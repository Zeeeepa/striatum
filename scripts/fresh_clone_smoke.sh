#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$ROOT/scripts/smoke_common.sh"
TMP="$(mktemp -d)"
CLONE="$TMP/striatum"
TARGET="$TMP/target-repo"
WORKFLOW="$TARGET/docs/workflows/smoke/workflow.json"
DAEMON_PID=""
PG_ADMIN_URL=""
PG_DATABASE_NAME=""

cleanup() {
  if [[ -n "${DAEMON_PID:-}" ]] && kill -0 "$DAEMON_PID" 2>/dev/null; then
    kill "$DAEMON_PID" 2>/dev/null || true
    wait "$DAEMON_PID" 2>/dev/null || true
  fi
  if [[ -n "${PG_ADMIN_URL:-}" && -n "${PG_DATABASE_NAME:-}" && -x "$CLONE/.venv/bin/python" ]]; then
    smoke_drop_pg_db "$CLONE/.venv/bin/python" "$PG_ADMIN_URL" "$PG_DATABASE_NAME"
  fi
  rm -rf "$TMP"
}
trap cleanup EXIT

git clone --quiet "$ROOT" "$CLONE"
cd "$CLONE"

python3 -m venv .venv
.venv/bin/python -m pip install --quiet --upgrade pip
.venv/bin/python -m pip install --quiet -e ".[dev,daemon-pg]"
make daemon-go-build >/dev/null

RUNNER="$CLONE/.venv/bin/striatum"
"$RUNNER" --help >/dev/null

mkdir -p "$TARGET"
git -C "$TARGET" init --quiet
mkdir -p "$(dirname "$WORKFLOW")"
cp "$CLONE/examples/rfc-ledger-cleanup/workflow.json" "$WORKFLOW"
git -C "$TARGET" add docs/workflows/smoke/workflow.json
git -C "$TARGET" \
  -c user.name="Striatum Smoke" \
  -c user.email="striatum-smoke@example.invalid" \
  commit --quiet -m "Add smoke workflow"

run_workflow_smoke() {
  local branch="$1"
  "$RUNNER" --repo "$TARGET" workflow validate \
    "$WORKFLOW" \
    --allow-same-model-pairing \
    --json >/dev/null

  prepare_json="$("$RUNNER" --repo "$TARGET" run prepare \
    --workflow "$WORKFLOW" \
    --json)"
  run_id="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["run_id"])' <<< "$prepare_json")"

  "$RUNNER" --repo "$TARGET" branch confirm \
    --run-id "$run_id" \
    --branch "$branch" \
    --json >/dev/null
  "$RUNNER" --repo "$TARGET" run start --run-id "$run_id" --json >/dev/null
  "$RUNNER" --repo "$TARGET" evidence export \
    --run-id "$run_id" \
    --path docs/reviews/striatum/SMOKE_EVIDENCE.md \
    --json >/dev/null

  test -f "$TARGET/docs/reviews/striatum/SMOKE_EVIDENCE.md"
  grep -q "Striatum Evidence Export" "$TARGET/docs/reviews/striatum/SMOKE_EVIDENCE.md"
}

PG_ENV="$TMP/pg.env"
if smoke_create_pg_db "$CLONE/.venv/bin/python" "$PG_ENV"; then
  # shellcheck disable=SC1090
  source "$PG_ENV"
  export STRIATUM_DAEMON_DB_URL="$PG_DATABASE_URL"
  export STRIATUM_DAEMON_REGISTRY="$TMP/daemon/striatumd.sqlite3"
  export STRIATUM_DAEMON_RUNTIME_DIR="$TMP/runtime"
  export STRIATUM_DAEMON_SOCKET="$TMP/runtime/striatumd.sock"
  export STRIATUM_PG_DOCTOR_TEST_HARNESS_OWNER_OK=1
  export XDG_CONFIG_HOME="$TMP/config"
  mkdir -p "$TMP/runtime"
  "$RUNNER" daemon start \
    --postgres-url "$PG_DATABASE_URL" \
    --sweep-interval-seconds 60 \
    --json >"$TMP/daemon.log" 2>&1 &
  DAEMON_PID="$!"
  smoke_wait_for_socket "$STRIATUM_DAEMON_SOCKET" "$DAEMON_PID" "$TMP/daemon.log"
  "$RUNNER" --repo "$TARGET" repo add "$TARGET" --init --json >/dev/null
  run_workflow_smoke "striatum/smoke"
  test -d "$TARGET/.striatum/scratch"
  test ! -e "$TARGET/.striatum/state.sqlite3"
else
  echo "fresh-clone smoke: skipping; PostgreSQL is required for daemon-owned Striatum state" >&2
  exit 0
fi

grep -q '^.striatum/$' "$TARGET/.gitignore"
