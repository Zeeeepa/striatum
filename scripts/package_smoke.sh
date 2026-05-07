#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PYTHON_FOR_BUILD="${PYTHON_FOR_BUILD:-python3}"
if [[ "$PYTHON_FOR_BUILD" == */* ]]; then
  PYTHON_FOR_BUILD="$(cd "$(dirname "$PYTHON_FOR_BUILD")" && pwd)/$(basename "$PYTHON_FOR_BUILD")"
else
  PYTHON_FOR_BUILD="$(command -v "$PYTHON_FOR_BUILD")"
fi
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

DIST="$TMP/dist"
PACKAGE_VENV="$TMP/package-venv"
SOURCE="$TMP/source"
TARGET="$TMP/target-repo"

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
"$PYTHON_FOR_BUILD" -m build --sdist --wheel --outdir "$DIST" >/dev/null

wheel_count="$(find "$DIST" -maxdepth 1 -name 'striatum-*.whl' | wc -l | tr -d ' ')"
sdist_count="$(find "$DIST" -maxdepth 1 -name 'striatum-*.tar.gz' | wc -l | tr -d ' ')"
if [[ "$wheel_count" != "1" ]]; then
  echo "expected exactly one wheel in $DIST, found $wheel_count" >&2
  exit 1
fi
if [[ "$sdist_count" != "1" ]]; then
  echo "expected exactly one sdist in $DIST, found $sdist_count" >&2
  exit 1
fi

wheel="$(find "$DIST" -maxdepth 1 -name 'striatum-*.whl' -print -quit)"
"$PYTHON_FOR_BUILD" -m venv "$PACKAGE_VENV"
"$PACKAGE_VENV/bin/python" -m pip install --quiet --upgrade pip
"$PACKAGE_VENV/bin/python" -m pip install --quiet "$wheel"

RUNNER="$PACKAGE_VENV/bin/striatum"
"$RUNNER" --help >/dev/null

mkdir -p "$TARGET"
git -C "$TARGET" init --quiet

"$RUNNER" --repo "$TARGET" init --json >/dev/null
"$RUNNER" --repo "$TARGET" workflow validate \
  "$SOURCE/examples/rfc-ledger-cleanup/workflow.json" \
  --json >/dev/null

prepare_json="$("$RUNNER" --repo "$TARGET" run prepare \
  --workflow "$SOURCE/examples/rfc-ledger-cleanup/workflow.json" \
  --json)"
run_id="$("$PACKAGE_VENV/bin/python" -c 'import json,sys; print(json.load(sys.stdin)["data"]["run_id"])' <<< "$prepare_json")"

"$RUNNER" --repo "$TARGET" branch confirm \
  --run-id "$run_id" \
  --branch striatum/package-smoke \
  --json >/dev/null
"$RUNNER" --repo "$TARGET" run start --run-id "$run_id" --json >/dev/null
"$RUNNER" --repo "$TARGET" status --run-id "$run_id" --json >/dev/null
