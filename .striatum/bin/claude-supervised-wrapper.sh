#!/usr/bin/env bash
# RFC 0009 / RFC 0010 V2 supervised lane wrapper for Claude Code.
#
# Reads newline-terminated work packets from stdin (the supervisor's
# named pipe at .striatum/scratch/<supervisor_id>/stdin.pipe) and
# spawns a fresh `claude --print` invocation per packet. The agent
# inside Claude Code advances workflow state via `striatum` CLI
# commands the packet tells it to invoke.
#
# Stdin: line-delimited UTF-8 JSON packets. Null bytes inside a
#        packet would truncate the line at the bash `read` layer;
#        Striatum's packet shape never emits literal null bytes.
# Stdout/stderr: routed to /dev/null. Per RFC 0009 the supervisor
#                already DEVNULLs the wrapper's own stdout/stderr;
#                this is belt-and-braces for standalone use.
# Exit: 0 on writer-EOF or SIGTERM; non-zero only on shell errors.
#
# Per-packet shape (RFC 0010 V2): each Striatum work packet is
# independent (own lease, job_id, write_scope, callback commands).
# A fresh `claude` invocation per packet matches that independence
# and avoids depending on Claude Code's stream-json multi-turn
# behaviour, which is not publicly documented as of 2026-05-08.
set -euo pipefail

inner_pid=""

on_term() {
  if [ -n "$inner_pid" ] && kill -0 "$inner_pid" 2>/dev/null; then
    kill -TERM "$inner_pid" 2>/dev/null || true
  fi
  exit 0
}
trap on_term TERM INT

while IFS= read -r packet; do
  [ -z "$packet" ] && continue
  printf '%s\n' "$packet" \
    | claude --print >/dev/null 2>&1 \
    &
  inner_pid=$!
  wait "$inner_pid" || true
  inner_pid=""
done
