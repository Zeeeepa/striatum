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
# Stdout/stderr: per-packet stdout+stderr is captured to
#                "$STRIATUM_SCRATCH_DIR/claude-logs/packet-NNNN.log"
#                so the operator can debug agent failures that the
#                supervisor pipe would otherwise hide. The wrapper's
#                own stdout/stderr remains quiet (the supervisor
#                already DEVNULLs us).
# Exit: 0 on writer-EOF or SIGTERM; non-zero only on shell errors.
#       Inner `claude --print` failures are logged but do not crash
#       the supervisor — each packet is independent (own lease) so
#       a per-packet failure surfaces via lease expiry, not by
#       killing the long-lived consumer.
#
# Per-packet shape (RFC 0010 V2): each Striatum work packet is
# independent (own lease, job_id, write_scope, callback commands).
# A fresh `claude` invocation per packet matches that independence
# and avoids depending on Claude Code's stream-json multi-turn
# behaviour, which is not publicly documented as of 2026-05-08.
set -euo pipefail

log_dir="${STRIATUM_SCRATCH_DIR:-.striatum/scratch}/claude-logs"
mkdir -p "$log_dir"
counter=0
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
  counter=$((counter + 1))
  log_file=$(printf '%s/packet-%04d.log' "$log_dir" "$counter")
  {
    printf '## packet=%d ts=%s\n' "$counter" "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    printf '## stdin:\n%s\n' "$packet"
    printf '## --- claude stdout/stderr ---\n'
  } >"$log_file"
  # --permission-mode acceptEdits + --allowedTools "Bash" auto-approves
  # the striatum CLI verbs the agent must call (ack, publish-artifact,
  # verdict, complete) without an interactive prompt. Without these flags
  # `claude --print` prints "Could you confirm whether you'd like me to
  # proceed" on the first Bash tool use because stdin is closed (the
  # packet already consumed it), and the lane stalls with no artifact.
  # Filesystem boundaries are enforced by the packet's write_scope.
  printf '%s\n' "$packet" \
    | claude --print --permission-mode acceptEdits --allowedTools "Bash" >>"$log_file" 2>&1 \
    &
  inner_pid=$!
  rc=0
  wait "$inner_pid" || rc=$?
  inner_pid=""
  printf '## exit=%d\n' "$rc" >>"$log_file"
done
