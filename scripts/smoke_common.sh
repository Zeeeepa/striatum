#!/usr/bin/env bash

smoke_create_pg_db() {
  local python_bin="$1"
  local env_file="$2"
  "$python_bin" - "$env_file" <<'PY'
import os
import re
import shlex
import sys
import uuid
from pathlib import Path
from urllib.parse import parse_qsl, urlencode, urlsplit, urlunsplit

try:
    import psycopg
except Exception as exc:
    print(f"postgres smoke unavailable: psycopg import failed: {exc}", file=sys.stderr)
    raise SystemExit(42)


def database_url(url: str, database: str) -> str:
    if not re.fullmatch(r"[A-Za-z0-9_]+", database):
        raise ValueError(f"unsafe database name {database!r}")
    parsed = urlsplit(url)
    query = urlencode(parse_qsl(parsed.query, keep_blank_values=True))
    path = "/" + database
    if parsed.netloc == "" and parsed.scheme in {"postgresql", "postgres"}:
        suffix = ""
        if query:
            suffix += f"?{query}"
        if parsed.fragment:
            suffix += f"#{parsed.fragment}"
        return f"{parsed.scheme}:///{database}{suffix}"
    return urlunsplit((parsed.scheme, parsed.netloc, path, query, parsed.fragment))


env_file = Path(sys.argv[1])
base = (
    os.environ.get("STRIATUM_TEST_POSTGRES_URL")
    or os.environ.get("STRIATUM_DAEMON_DB_URL")
    or "postgresql:///postgres"
)
if not base:
    raise SystemExit(42)
admin_url = database_url(base, "postgres")
db_name = f"striatum_smoke_{uuid.uuid4().hex}"
try:
    conn = psycopg.connect(admin_url)
    conn.autocommit = True
    with conn.cursor() as cur:
        cur.execute(f'CREATE DATABASE "{db_name}"')
finally:
    try:
        conn.close()
    except Exception:
        pass
database = database_url(base, db_name)
env_file.write_text(
    "\n".join(
        [
            f"PG_ADMIN_URL={shlex.quote(admin_url)}",
            f"PG_DATABASE_NAME={shlex.quote(db_name)}",
            f"PG_DATABASE_URL={shlex.quote(database)}",
            "",
        ]
    ),
    encoding="utf-8",
)
PY
}


smoke_drop_pg_db() {
  local python_bin="$1"
  local admin_url="$2"
  local db_name="$3"
  "$python_bin" - "$admin_url" "$db_name" <<'PY' || true
import re
import sys

try:
    import psycopg
except Exception:
    raise SystemExit(0)

admin_url = sys.argv[1]
db_name = sys.argv[2]
if not re.fullmatch(r"[A-Za-z0-9_]+", db_name):
    raise SystemExit(0)
conn = psycopg.connect(admin_url)
conn.autocommit = True
try:
    with conn.cursor() as cur:
        cur.execute(
            """
            SELECT pg_terminate_backend(pid)
            FROM pg_stat_activity
            WHERE datname = %s AND pid <> pg_backend_pid()
            """,
            (db_name,),
        )
        cur.execute(f'DROP DATABASE IF EXISTS "{db_name}"')
finally:
    conn.close()
PY
}


smoke_wait_for_socket() {
  local socket_path="$1"
  local daemon_pid="$2"
  local daemon_log="$3"
  for _ in $(seq 1 100); do
    if [[ -S "$socket_path" ]]; then
      return 0
    fi
    if ! kill -0 "$daemon_pid" 2>/dev/null; then
      cat "$daemon_log" >&2 || true
      return 1
    fi
    sleep 0.1
  done
  cat "$daemon_log" >&2 || true
  return 1
}
