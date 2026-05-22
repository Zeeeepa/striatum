# Apply Gap Fixes

You are the remediation implementer for the RFC 0076 audit remediation closure
workflow.

Read the source and docs verification reports first. Apply only small,
bounded fixes for gaps they prove. Do not broaden scope into generator/catalog
implementation, UI issue queues, new artifact schemas, hosted services,
telemetry, transcript capture, or external persistence.

Expected behavior:

- If verification found no gaps, publish a no-op handoff explaining that no
  code or docs changes were needed.
- If verification found gaps, patch only the cited files and add or update
  focused tests when behavior changes.
- Preserve the Striatum authority boundary: daemon-owned PostgreSQL and
  daemon MCP/RPC calls remain authoritative; terminal output, tmux panes,
  provider hooks, marker files, and repo-local SQLite are not workflow state.

Produce `docs/operator/artifacts/rfc-0076-audit-remediation/build/HANDOFF.md`
with changed paths, validation commands, and any deferred follow-up.
