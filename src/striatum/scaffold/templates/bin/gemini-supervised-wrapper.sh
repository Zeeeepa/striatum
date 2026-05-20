#!/usr/bin/env bash
# RFC 0009 / RFC 0010 V2 supervised lane wrapper for Gemini CLI.
#
# Reads newline-terminated work packets from stdin (the supervisor's
# named pipe at .striatum/scratch/<supervisor_id>/stdin.pipe) and
# spawns a fresh `gemini --prompt -` invocation per packet. The agent
# inside the Gemini CLI advances workflow state via `striatum` CLI
# commands the packet tells it to invoke.
#
# Stdin: line-delimited UTF-8 JSON packets.
# Stdout/stderr: per-packet output captured to
#                "$STRIATUM_SCRATCH_DIR/gemini-logs/packet-NNNN.log".
#                The wrapper's own stdout/stderr remains quiet (the
#                supervisor already DEVNULLs us).
# Exit: 0 on writer-EOF or SIGTERM. Inner `gemini` failures are
#       logged but do not crash the supervisor — each packet is
#       independent (own lease), so per-packet failures surface via
#       lease expiry, not by killing the long-lived consumer.
set -euo pipefail

log_dir="${STRIATUM_SCRATCH_DIR:-.striatum/scratch}/gemini-logs"
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
    printf '## --- gemini stdout/stderr ---\n'
  } >"$log_file"
  # --approval-mode yolo auto-approves all tools including run_shell_command.
  # auto_edit (the previous setting) approved edits but not shell calls,
  # so gemini wrote artifacts but could not invoke the striatum CLI verbs
  # the packet required, and the lane stalled with the artifact on disk
  # but no publish/verdict/complete recorded. Filesystem boundaries are
  # enforced by the packet's write_scope.
  printf '%s\n' "$packet" \
    | gemini --prompt - --output-format stream-json --approval-mode yolo >>"$log_file" 2>&1 \
    &
  inner_pid=$!
  rc=0
  wait "$inner_pid" || rc=$?
  inner_pid=""
  printf '## exit=%d\n' "$rc" >>"$log_file"
done
