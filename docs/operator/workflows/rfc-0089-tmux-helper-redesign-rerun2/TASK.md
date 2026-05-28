# TASK - RFC 0089 tmux helper redesign

Reference: `docs/rfcs/0089-tmux-backed-lane-monitoring.md`.

This workflow is for the helper redesign the operator requested:
replace attach-as-liveness with tmux session/pane liveness. The goal is not a
generic policy debate. Phase 1 must produce a concrete implementation path and
then code that makes `tmux attach-session` an observer only.

## Requested model lanes

- Codex: GPT-5.5 xhigh in agent-loop mode. The one-shot `codex exec` lane is
  intentionally avoided here because pipe FIFO EOF delivery is currently noisy
  for long packet prompts, while the installed daemon fixes Codex bootstrap
  delivery for agent-loop sessions.
- Codex builder: GPT-5.5 xhigh in agent-loop mode, because the build review
  must interrogate the preserved builder context.
- Claude: `claude-opus-4-7`.
- AGY: display model `Gemini 3.5 Flash High`.

Important AGY limitation: the installed `agy` binary does not expose a model
flag, and it rejects `--mcp-config`. Striatum can record the declared model
identity, but the operator must keep Antigravity configured to Gemini 3.5 Flash
High outside the workflow. This workflow therefore uses AGY in `--print`
one-shot mode for design/review lanes and does not mark AGY as an
interrogation target. Because `agy --print` requires the prompt as an argument
rather than stdin, and pipe supervisors currently keep a FIFO writer open,
the lane command wraps AGY with a one-line stdin-to-argument shim.

Important Codex bootstrap note: the current working tree fixes the old TUI
submit blocker by passing the Striatum bootstrap as codex's initial prompt
argument instead of typing it into the TUI. Live runs must use an installed or
restarted daemon built from this change; older daemon binaries will still strand
the codex builder with the bootstrap sitting in the input editor.

## Implementation scope

Implement RFC 0089 Phase 1:

1. `go/pkg/supervisor/pty.go` must stop treating `tmux attach-session` as the
   supervised lane identity.
2. Tmux-backed launches must record session name, window id, pane id, pane
   pid, pane pid start token when available, and attach command metadata.
3. Liveness for tmux-backed lanes must probe the tmux session/pane directly:
   `has-session`, pane existence, `pane_dead`, pane pid, and start-token
   comparison.
4. `supervise.status`, delivery reconciliation, doctor/status/dashboard
   details, and recovery sweep must use the tmux liveness probe for tmux-backed
   lanes.
5. `supervise.stop` must terminate the actual tmux lane, not just a transient
   attach client.
6. Raw pane text and `pty.log` remain private diagnostics only. Do not use tmux
   text for state, provenance, bylines, verdicts, exports, or recovery.

## Design requirements

The three design lanes should independently decide the smallest coherent
implementation slice. Each design must name:

- exact files and functions to change;
- metadata shape and database/read projections;
- liveness failure classes;
- tests, including attach-client-exit and missing-session/pane cases;
- rollback behavior and non-tmux fallback behavior;
- any command-authority/spec/decision-log doc updates.

## Review and interrogation logging

Design review and build review must interrogate the live reviewed session before
verdict. The review jobs must also publish a curated chat log artifact named
`INTERROGATION_CHAT.md` in their review directory. That log is a curated
`interrogation.show` projection: question/answer turns and IDs only. It must
not include raw provider terminal output, tmux pane captures, PTY logs, tokens,
or private diagnostics.

## Verification

Required verification for the implementation:

- targeted Go tests for the tmux liveness probe and supervisor status behavior;
- tests proving attach-client exit does not mark the lane lost;
- tests for `tmux_session_missing`, `tmux_pane_missing`, `tmux_pane_dead`, and
  pid/start-token mismatch where the platform supports it;
- `cd go && gofmt -l . && go vet ./... && go test ./...`;
- a handoff that states whether live tmux monitoring can now be enabled by
  configuration/default change alone.
