#!/usr/bin/env bash
# revert.sh — undo instrument.sh, restoring byte-identical baseline behavior.
#
# Owner side of instrument.sh was READ-ONLY (Hook 1 reads, Hook 2 separate-conn
# sampler), so revert only has to: stop the sampler, remove its CSV/SQL scratch,
# print the superuser RESETs (if you enabled auto_explain), and ASSERT the daemon
# role is at its expected baseline {statement_timeout=600s} (no leaked lock_timeout
# / idle_in_transaction_session_timeout — see REPORT.md §0).
set -euo pipefail

OWNER_URL="${STRIATUM_OWNER_URL:-postgres://halbritt@/striatum_daemon?host=/var/run/postgresql}"
OUTDIR="${STRIATUM_INSTRUMENT_DIR:-/tmp/striatum-instrument}"
SAMPLER_PID_FILE="${OUTDIR}/sampler.pid"
SAMPLE_CSV="${OUTDIR}/pg_locks_sample.csv"
SAMPLER_SQL="${OUTDIR}/sampler.sql"

echo "== Stopping Hook 2 sampler =="
if [[ -f "${SAMPLER_PID_FILE}" ]]; then
  PID="$(cat "${SAMPLER_PID_FILE}")"
  if kill -0 "${PID}" 2>/dev/null; then
    kill "${PID}" 2>/dev/null && echo "killed sampler pid ${PID}"
  else
    echo "sampler pid ${PID} not running"
  fi
  rm -f "${SAMPLER_PID_FILE}"
else
  echo "no sampler pid file; nothing to stop"
fi

echo "== Removing sampler scratch (operator-local /tmp diagnostics) =="
rm -f "${SAMPLE_CSV}" "${SAMPLER_SQL}"
rmdir "${OUTDIR}" 2>/dev/null || true

echo "== Hook 1 owner side: read-only, nothing to revert =="

cat <<'SUPER'
== Hook 1 (SUPERUSER-ONLY) — run ONLY IF you enabled auto_explain in instrument.sh:
   sudo -u postgres psql -c "ALTER SYSTEM RESET session_preload_libraries;
                             ALTER SYSTEM RESET auto_explain.log_min_duration;
                             ALTER SYSTEM RESET auto_explain.log_analyze;
                             ALTER SYSTEM RESET auto_explain.log_nested_statements;
                             SELECT pg_reload_conf();"
   # pg_stat_statements was preloaded BEFORE instrumentation: do NOT DROP it.
   # If you ran pg_stat_statements_reset(), prior cumulative stats are gone
   # (acceptable — they re-accumulate); the extension itself stays installed.
SUPER

echo
echo "== Asserting daemon role baseline (expect: {statement_timeout=600s}) =="
ROLCONFIG="$(psql "${OWNER_URL}" -tAc "SELECT rolconfig FROM pg_roles WHERE rolname='striatumd_rw';")"
echo "striatumd_rw rolconfig = ${ROLCONFIG}"
case "${ROLCONFIG}" in
  *lock_timeout*|*idle_in_transaction_session_timeout*)
    echo "WARN: a lock_timeout / idle_in_transaction_session_timeout is set on the"
    echo "      daemon role. If you did NOT intend a permanent change, reset it:"
    echo "      psql \"${OWNER_URL}\" -c 'ALTER ROLE striatumd_rw RESET lock_timeout;'"
    echo "      psql \"${OWNER_URL}\" -c 'ALTER ROLE striatumd_rw RESET idle_in_transaction_session_timeout;'"
    ;;
  *)
    echo "OK: no leaked timeout GUCs on the daemon role."
    ;;
esac
echo "Baseline restored (owner side)."
