# Design Prompt: RFC 0040 V1.5 (F1-F6)

Produce the DESIGN.md artifact at the path your work packet specifies (under `docs/dogfood/044/design/<lane>/`).

Design **RFC 0040 V1.5 acceptance criteria** addressing the six codex findings F1-F6 surfaced in dogfood-040. Read `docs/rfcs/0040-mcp-driven-dogfood-harness.md` (Proposal §1–§5) and `docs/dogfood/040/OPERATOR_REPORT.md` (findings section) first.

Cover concretely:

- **F1 — Daemon MCP `tools/call` dispatch**: the MCP path authorizes + audits but never dispatches through the method registry. Compose tool calls land as "success" with no effect. Identify the exact daemon entry point under `src/striatum/daemon_pg/` that owns `tools/call`, the Python method registry it must route to, and the dispatch wiring change. Audit row currently records "allowed"; the design must keep that row but append result/error rows from the actual dispatch.
- **F2/F3 — Composite tool atomicity + verdict recording**: `dogfood.publish_on_behalf` runs ack → publish-artifact → verdict → complete internally. A failure mid-chain currently records a single "success" audit row. Design transactional or compensation semantics that the daemon RPC envelope-v1 carries without breaking it. Cite the composite-tool code path in `src/striatum/web/chat_tools.py` (or wherever composite tools land) and the daemon SQLite tables holding lease/queue/audit state.
- **F4 — Watcher invocation in supervisor lifecycle**: the supervised-progress watcher module exists in `src/striatum/process_progress.py` but is not started when the daemon spins up a supervisor. Identify the daemon supervisor lifecycle entry point in `src/striatum/daemon_pg/` (and/or `src/striatum/supervisor.py`) where the watcher task launches, and the shutdown path that joins it.
- **F5 — Watcher race + signal hardening**: the watcher reads `os.stat(...).st_mtime` and calls heartbeat; race between rotated log files, watcher start before the wrapper's first log write, and SIGTERM during a heartbeat call must be hardened. Spell out the exact race windows and the guards.
- **F6 — End-to-end tests**: existing tests cover mocked gating only. Design e2e tests that exercise the full MCP path: tool call → daemon dispatch → state change → audit row → composite atomicity rollback case. Identify the tests/fixtures and the smoke harness they hook into.

Designers MUST cite existing code in `src/striatum/daemon_pg/`, `src/striatum/web/chat_tools.py`, `src/striatum/process_progress.py`, `src/striatum/supervisor.py`, and `src/striatum/cli/` (function names, line refs). Hand-waving the dispatch wiring is grounds for design review to bounce.

**Backward compatibility (non-negotiable)**: existing MCP tools (`generate_workflow_preview`, `generate_workflow_write`, the RPC-thin tools) must keep working through the unchanged dispatch path. Daemon RPC envelope-v1 schema unchanged.

Out of scope: new tool surface beyond F1-F6 wiring, hosted services, RFC 0040 §6 future work.

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim. Plain Markdown line, NO bold, NO italics, NO heading prefix, NO quotes. Lowercase `author:`.

One-shot supervised invocation. Write the artifact directly. If `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf.
