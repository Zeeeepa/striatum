#!/usr/bin/env bash
# instrument.sh — striatumd contention instrumentation (Hooks 1 + 2).
#
# Hook 1: read pg_stat_statements (already preloaded on this cluster) and PRINT
#         the superuser-only steps (reset + auto_explain) for copy-paste, because
#         the owner role `halbritt` is NOT a superuser (cannot ALTER SYSTEM/LOAD).
# Hook 2: a 1 Hz pg_locks ⋈ pg_stat_activity sampler -> tagged CSV, to catch the
#         #198/#355 convoy and any #325 deadlock IN FLIGHT while you provoke a
#         parallel-completion run.
#
# Idempotent. Owner side is READ-ONLY (no durable PG state changed). Loopback only.
# Fully undone by revert.sh. Does NOT modify the daemon role or run inside any txn.
set -euo pipefail

OWNER_URL="${STRIATUM_OWNER_URL:-postgres://halbritt@/striatum_daemon?host=/var/run/postgresql}"
OUTDIR="${STRIATUM_INSTRUMENT_DIR:-/tmp/striatum-instrument}"
SAMPLE_CSV="${OUTDIR}/pg_locks_sample.csv"
SAMPLER_SQL="${OUTDIR}/sampler.sql"
SAMPLER_PID_FILE="${OUTDIR}/sampler.pid"
INTERVAL="${STRIATUM_SAMPLE_INTERVAL:-1}"   # seconds; 1 Hz default

mkdir -p "${OUTDIR}"

echo "== Hook 1: pg_stat_statements — top 20 by total_exec_time (read-only) =="
psql "${OWNER_URL}" -P pager=off -c "
  SELECT calls,
         round(total_exec_time::numeric, 1)  AS total_ms,
         round(mean_exec_time::numeric, 2)   AS mean_ms,
         round(max_exec_time::numeric, 1)    AS max_ms,
         rows,
         left(regexp_replace(query, '\s+', ' ', 'g'), 90) AS query
    FROM pg_stat_statements
   ORDER BY total_exec_time DESC
   LIMIT 20;" \
  || echo "WARN: pg_stat_statements unreadable (extension missing / perms?) — Hook 1 reads skipped"

cat <<'SUPER'

== Hook 1 (SUPERUSER-ONLY — run by hand, then replay one 60s sweep + one
   claim/heartbeat cycle, then re-read the table above to see the convoy victims):
   sudo -u postgres psql -d striatum_daemon -c "SELECT pg_stat_statements_reset();"
   sudo -u postgres psql -c "ALTER SYSTEM SET session_preload_libraries='auto_explain';
                             ALTER SYSTEM SET auto_explain.log_min_duration='200ms';
                             ALTER SYSTEM SET auto_explain.log_analyze=on;
                             ALTER SYSTEM SET auto_explain.log_nested_statements=on;
                             SELECT pg_reload_conf();"
   # auto_explain output -> the PostgreSQL log (loopback-local). revert.sh prints the RESET.
SUPER

echo
echo "== Hook 2: 1 Hz pg_locks ⋈ pg_stat_activity sampler -> ${SAMPLE_CSV} =="

# Sampler query: only emits rows when a backend is actually blocked (the convoy /
# deadlock moment). clock_timestamp() is the wall-clock tag; psql --csv quotes.
cat > "${SAMPLER_SQL}" <<'SQL'
SELECT clock_timestamp()                                              AS ts,
       a.pid                                                          AS blocked_pid,
       a.usename                                                      AS blocked_user,
       a.wait_event_type,
       a.wait_event,
       left(regexp_replace(a.query, '\s+', ' ', 'g'), 120)            AS blocked_query,
       (pg_blocking_pids(a.pid))[1]                                   AS blocking_pid,
       left(regexp_replace(coalesce(b.query, ''), '\s+', ' ', 'g'), 120) AS blocking_query,
       (SELECT string_agg(l.mode, ',')
          FROM pg_locks l
         WHERE l.pid = (pg_blocking_pids(a.pid))[1] AND l.granted)    AS blocking_held_modes
  FROM pg_stat_activity a
  LEFT JOIN pg_stat_activity b ON b.pid = (pg_blocking_pids(a.pid))[1]
 WHERE a.datname = 'striatum_daemon'
   AND cardinality(pg_blocking_pids(a.pid)) > 0;
SQL

if [[ -f "${SAMPLER_PID_FILE}" ]] && kill -0 "$(cat "${SAMPLER_PID_FILE}")" 2>/dev/null; then
  echo "sampler already running (pid $(cat "${SAMPLER_PID_FILE}")); leaving it"
else
  if [[ ! -f "${SAMPLE_CSV}" ]]; then
    echo "ts,blocked_pid,blocked_user,wait_event_type,wait_event,blocked_query,blocking_pid,blocking_query,blocking_held_modes" > "${SAMPLE_CSV}"
  fi
  nohup bash -c '
    set -u
    OWNER_URL="'"${OWNER_URL}"'"; CSV="'"${SAMPLE_CSV}"'"
    SQL="'"${SAMPLER_SQL}"'"; INTERVAL="'"${INTERVAL}"'"
    while true; do
      psql "${OWNER_URL}" --csv -t -f "${SQL}" 2>/dev/null >> "${CSV}" || true
      sleep "${INTERVAL}"
    done
  ' >/dev/null 2>&1 &
  echo $! > "${SAMPLER_PID_FILE}"
  echo "sampler started (pid $(cat "${SAMPLER_PID_FILE}")) at ${INTERVAL}s interval -> ${SAMPLE_CSV}"
fi

echo
echo "NOW PROVOKE THE CONVOY: start a fan-out run and complete/publish two sibling"
echo "lanes on the same run near-simultaneously (or two artifact.publish on one job)."
echo "Blocked moments land in ${SAMPLE_CSV} (empty rows == no contention == good)."
echo "When finished:  ./revert.sh"
