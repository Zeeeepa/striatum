#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$ROOT/scripts/smoke_common.sh"
PYTHON_FOR_BUILD="${PYTHON_FOR_BUILD:-python3}"
if [[ "$PYTHON_FOR_BUILD" == */* ]]; then
  PYTHON_FOR_BUILD="$(cd "$(dirname "$PYTHON_FOR_BUILD")" && pwd)/$(basename "$PYTHON_FOR_BUILD")"
else
  PYTHON_FOR_BUILD="$(command -v "$PYTHON_FOR_BUILD")"
fi
TMP="$(mktemp -d)"
DIST="$TMP/dist"
PACKAGE_VENV="$TMP/package-venv"
SOURCE="$TMP/source"
TARGET="$TMP/target-repo"
WORKFLOW="$TARGET/docs/workflows/smoke/workflow.json"
DAEMON_PID=""
PG_ADMIN_URL=""
PG_DATABASE_NAME=""
SOURCE_GIT_SHA="$(git -C "$ROOT" rev-parse HEAD 2>/dev/null || echo unknown)"
SOURCE_GIT_DIRTY="$(
  if git -C "$ROOT" diff --quiet --ignore-submodules HEAD -- 2>/dev/null; then
    echo clean
  else
    echo dirty
  fi
)"

if [[ "$SOURCE_GIT_SHA" == "unknown" ]]; then
  echo "package smoke requires source git SHA for Go daemon provenance" >&2
  exit 1
fi

cleanup() {
  if [[ -n "${DAEMON_PID:-}" ]] && kill -0 "$DAEMON_PID" 2>/dev/null; then
    kill "$DAEMON_PID" 2>/dev/null || true
    wait "$DAEMON_PID" 2>/dev/null || true
  fi
  if [[ -n "${PG_ADMIN_URL:-}" && -n "${PG_DATABASE_NAME:-}" && -x "$PACKAGE_VENV/bin/python" ]]; then
    smoke_drop_pg_db "$PACKAGE_VENV/bin/python" "$PG_ADMIN_URL" "$PG_DATABASE_NAME"
  fi
  rm -rf "$TMP"
}
trap cleanup EXIT

mkdir -p "$SOURCE"
tar \
  --exclude .git \
  --exclude .striatum \
  --exclude .venv \
  --exclude .pytest_cache \
  --exclude .mypy_cache \
  --exclude .ruff_cache \
  --exclude __pycache__ \
  --exclude build \
  --exclude dist \
  -C "$ROOT" \
  -cf - \
  . | tar -C "$SOURCE" -xf -

cd "$SOURCE"
GIT_SHA="$SOURCE_GIT_SHA" GIT_DIRTY="$SOURCE_GIT_DIRTY" make daemon-go-install >/dev/null
"$PYTHON_FOR_BUILD" -m build --sdist --wheel --outdir "$DIST" >/dev/null

wheel_count="$(find "$DIST" -maxdepth 1 -name 'striatum_orchestrator-*.whl' | wc -l | tr -d ' ')"
sdist_count="$(find "$DIST" -maxdepth 1 -name 'striatum_orchestrator-*.tar.gz' | wc -l | tr -d ' ')"
if [[ "$wheel_count" != "1" ]]; then
  echo "expected exactly one wheel in $DIST, found $wheel_count" >&2
  exit 1
fi
if [[ "$sdist_count" != "1" ]]; then
  echo "expected exactly one sdist in $DIST, found $sdist_count" >&2
  exit 1
fi

wheel="$(find "$DIST" -maxdepth 1 -name 'striatum_orchestrator-*.whl' -print -quit)"
"$PYTHON_FOR_BUILD" -m venv "$PACKAGE_VENV"
"$PACKAGE_VENV/bin/python" -m pip install --quiet --upgrade pip
"$PACKAGE_VENV/bin/python" -m pip install --quiet "$wheel[daemon-pg]"

RUNNER="$PACKAGE_VENV/bin/striatum"
"$RUNNER" --help >/dev/null
GO_BIN="$("$PACKAGE_VENV/bin/python" - <<'PY'
from striatum._daemongo import find_binary

print(find_binary())
PY
)"
GO_DESCRIBE="$("$GO_BIN" --describe)"
PACKAGE_VERSION="$("$PACKAGE_VENV/bin/python" - <<'PY'
import striatum

print(striatum.__version__)
PY
)"
if [[ "$GO_DESCRIBE" != *"daemon_version=$PACKAGE_VERSION"* ]]; then
  echo "packaged Go daemon version mismatch: $GO_DESCRIBE" >&2
  exit 1
fi
if [[ "$GO_DESCRIBE" != *"git_sha=$SOURCE_GIT_SHA"* ]]; then
  echo "packaged Go daemon git SHA mismatch: $GO_DESCRIBE" >&2
  exit 1
fi

mkdir -p "$TARGET"
git -C "$TARGET" init --quiet
mkdir -p "$(dirname "$WORKFLOW")"
cp "$SOURCE/examples/rfc-ledger-cleanup/workflow.json" "$WORKFLOW"
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
  run_id="$("$PACKAGE_VENV/bin/python" -c 'import json,sys; print(json.load(sys.stdin)["data"]["run_id"])' <<< "$prepare_json")"

  "$RUNNER" --repo "$TARGET" branch confirm \
    --run-id "$run_id" \
    --branch "$branch" \
    --json >/dev/null
  "$RUNNER" --repo "$TARGET" run start --run-id "$run_id" --json >/dev/null
  "$RUNNER" --repo "$TARGET" status --run-id "$run_id" --json >/dev/null
}

PG_ENV="$TMP/pg.env"
if smoke_create_pg_db "$PACKAGE_VENV/bin/python" "$PG_ENV"; then
  # shellcheck disable=SC1090
  source "$PG_ENV"
  export STRIATUM_DAEMON_DB_URL="$PG_DATABASE_URL"
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
  run_workflow_smoke "striatum/package-smoke"
  test -d "$TARGET/.striatum/scratch"
  test ! -e "$TARGET/.striatum/state.sqlite3"
else
  echo "package smoke: skipping; PostgreSQL is required for daemon-owned Striatum state" >&2
  exit 0
fi
