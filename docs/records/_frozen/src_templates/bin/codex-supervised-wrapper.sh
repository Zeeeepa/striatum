#!/usr/bin/env bash
# RFC 0009 / RFC 0010 V2 supervised lane wrapper for Codex CLI.
#
# Reads newline-terminated work packets from stdin (the supervisor's
# named pipe at .striatum/scratch/<supervisor_id>/stdin.pipe) and
# spawns a fresh `codex exec` invocation per packet. The agent
# inside the Codex CLI advances workflow state via `striatum` CLI
# commands the packet tells it to invoke.
#
# Stdin: line-delimited UTF-8 JSON packets. Null bytes inside a
#        packet would truncate the line at the bash `read` layer;
#        Striatum's packet shape never emits literal null bytes.
# Stdout/stderr: per-packet output captured to
#                "$STRIATUM_SCRATCH_DIR/codex-logs/packet-NNNN.log"
#                so the operator can debug agent failures that the
#                supervisor pipe would otherwise hide. The wrapper's
#                own stdout/stderr remains quiet (the supervisor
#                already DEVNULLs us).
# Exit: 0 on writer-EOF or SIGTERM; non-zero only on shell errors.
#       Inner `codex exec` failures are logged but do not crash the
#       supervisor — each packet is independent (own lease, job_id,
#       write_scope, callback commands) so per-packet failures
#       surface via lease expiry, not by killing the long-lived
#       consumer.
#
# Per-packet shape (RFC 0010 V2): each Striatum work packet is
# independent. A fresh `codex exec` invocation per packet matches
# that independence and avoids `codex exec ... -` blocking
# indefinitely on FIFO stdin in supervised mode (the failure mode
# observed in dogfood-031).
set -euo pipefail

log_dir="${STRIATUM_SCRATCH_DIR:-.striatum/scratch}/codex-logs"
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
    printf '## --- codex stdout/stderr ---\n'
  } >"$log_file"
  # --dangerously-bypass-approvals-and-sandbox + -c approval_policy=never
  # auto-approves shell tool use so codex can invoke the striatum CLI verbs
  # the packet supplies (ack, publish-artifact, verdict, complete) without
  # an interactive approval prompt — the lane-stall failure mode observed
  # on claude/gemini lanes before their wrappers were aligned in v1.48.1.
  # Filesystem boundaries are enforced by the packet's write_scope.
  printf '%s\n' "$packet" \
    | codex exec --json --ephemeral --skip-git-repo-check \
        --dangerously-bypass-approvals-and-sandbox \
        -c approval_policy=never --ignore-user-config - >>"$log_file" 2>&1 \
    &
  inner_pid=$!
  rc=0
  wait "$inner_pid" || rc=$?
  inner_pid=""
  printf '## exit=%d\n' "$rc" >>"$log_file"
done
