---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/rfcs/0075-tmux-observable-mcp-agent-sessions.md", "docs/operator/BRIEF.md", "docs/architecture/CLI_RETIREMENT_PARITY.md"]
---

# Phase 2C Tmux Authority Guardrail
author: operator [self-declared: codex-driver]

## Result

RFC 0075 guardrail work remains bounded to local inspection metadata.
Current docs and code already state that tmux panes, pane text, and
transcripts are not workflow state.

The smallest next guardrail should be a test that scans tmux/session paths and
asserts:

- no transcript capture is introduced as an artifact payload source;
- no pane text is parsed to infer job state, verdicts, or artifact content;
- tmux attach metadata remains projected as inspection metadata only.

Suggested test location:

- `tests/architecture/test_authority_guardrails.py` or a focused
  `tests/architecture/test_tmux_authority_boundary.py`.

Suggested scan targets:

- `go/pkg/agentloop`
- `go/pkg/mutations`
- `src/striatum/supervisor.py`
- `src/striatum/daemon_pg`
- `docs/rfcs/0075-tmux-observable-mcp-agent-sessions.md`

This phase did not add that guardrail yet because the active implementation
slice for TODO 67 was the MCP/UI parity test closure.
