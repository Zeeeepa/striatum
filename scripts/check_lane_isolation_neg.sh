#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat >&2 <<'USAGE'
usage: STRIATUM_LANE_OS_USER=<user> \
       STRIATUM_LANE_ISOLATION_UNIX_URL=<postgres-url> \
       STRIATUM_LANE_ISOLATION_TCP_URL=<postgres-url> \
       scripts/check_lane_isolation_neg.sh

Fails if the configured lane OS identity can connect to PostgreSQL over either
the protected UNIX-socket URL or the loopback TCP URL. This is the RFC 0110
T-LANE-ISOLATION-NEG gate for #87.
USAGE
}

require_env() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    echo "$name is required" >&2
    usage
    exit 2
  fi
}

require_env STRIATUM_LANE_OS_USER
require_env STRIATUM_LANE_ISOLATION_UNIX_URL
require_env STRIATUM_LANE_ISOLATION_TCP_URL

if ! command -v psql >/dev/null 2>&1; then
  echo "psql is required for lane isolation probes" >&2
  exit 2
fi

if ! id "$STRIATUM_LANE_OS_USER" >/dev/null 2>&1; then
  echo "lane OS user does not exist: $STRIATUM_LANE_OS_USER" >&2
  exit 2
fi

run_as_lane() {
  local url="$1"
  local current_user
  current_user="$(id -un)"
  if [[ "$current_user" == "$STRIATUM_LANE_OS_USER" ]]; then
    env -i PATH="${PATH:-/usr/bin:/bin}" psql "$url" -v ON_ERROR_STOP=1 -c 'select 1'
    return
  fi
  if ! command -v sudo >/dev/null 2>&1; then
    echo "sudo is required unless running as $STRIATUM_LANE_OS_USER" >&2
    return 125
  fi
  sudo -n -u "$STRIATUM_LANE_OS_USER" env -i PATH="${PATH:-/usr/bin:/bin}" \
    psql "$url" -v ON_ERROR_STOP=1 -c 'select 1'
}

expect_denied() {
  local label="$1"
  local url="$2"
  local output status
  set +e
  output="$(run_as_lane "$url" 2>&1)"
  status=$?
  set -e
  if [[ "$status" -eq 0 ]]; then
    echo "FAIL: lane identity connected over $label; #87 isolation is not closed" >&2
    echo "$output" >&2
    exit 1
  fi
  if [[ "$status" -eq 125 ]]; then
    echo "$output" >&2
    exit 2
  fi
  echo "ok: lane identity denied over $label"
}

expect_denied "protected UNIX socket" "$STRIATUM_LANE_ISOLATION_UNIX_URL"
expect_denied "loopback TCP" "$STRIATUM_LANE_ISOLATION_TCP_URL"

echo "T-LANE-ISOLATION-NEG: ok"
