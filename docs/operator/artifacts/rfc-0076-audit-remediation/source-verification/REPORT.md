---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["rfc-0076-remediation-plan", "rfc-0076-audit-synthesis", "current-source-and-tests"]
---

# RFC 0076 Source Remediation Verification
author: source-verifier-codex-001
status: closed
date: 2026-05-22

## Summary

REM-001, REM-002, and REM-009 are closed against current source and focused
tests. I found no remaining source gap that requires a follow-up job for these
three items.

## Verification Matrix

| ID | Verdict | Evidence |
|---|---|---|
| REM-001 | closed | Go work-packet construction now routes `task_prompt` through `packetTaskPrompt`, which resolves workflow-local prompt paths against `workflow_snapshots.source_path` and preserves `workflow_relative_path` plus `workflow_source_path` diagnostics (`go/pkg/mutations/claim.go:273`, `go/pkg/mutations/claim.go:295`). Go unit coverage asserts a workflow under `docs/operator/workflows/demo/workflow.json` emits `docs/operator/workflows/demo/prompts/demo.md` (`go/pkg/mutations/claim_test.go:44`). Python packet construction applies the same `packet_task_prompt` logic (`src/striatum/daemon_pg/handlers/context.py:766`, `src/striatum/daemon_pg/handlers/context.py:794`), and the PostgreSQL claim-next regression verifies the emitted packet shape through the daemon handler path (`tests/daemon_pg/handlers/workflow_loop/test_claim_next.py:100`). |
| REM-002 | closed | Go MCP `tools/call` now refuses hidden production tools before daemon RPC dispatch with `tool_hidden` (`go/pkg/mcp/tools.go:27`). `tools/list` and `tools/call` share the hidden workflow-authoring method set (`go/pkg/mcp/capabilities.go:17`, `go/pkg/mcp/capabilities.go:60`). Regression coverage verifies both read-token denial and write-token denial, and asserts the hidden `workflow.generate` handler is not called (`go/pkg/mcp/http_test.go:111`, `go/pkg/mcp/http_test.go:123`). Current MCP docs also state the fail-closed policy (`docs/MCP.md:131`). |
| REM-009 | closed | `striatum adopt` exposes `suggested_workflow_guidance` explaining that the suggested workflow is a safe first-run scaffold and points directly to `docs/WORKFLOW_TYPES.md` (`src/striatum/day_zero.py:28`, `src/striatum/day_zero.py:104`). Tests cover both the dry-run data envelope and JSON CLI output (`tests/test_day_zero.py:48`, `tests/test_day_zero.py:79`). Usage docs already link day-zero users to workflow-shape guidance (`docs/USING_STRIATUM.md:244`, `docs/GETTING_STARTED.md:75`). |

## Validation Run

I ran these focused commands:

```bash
(cd go && go test ./pkg/mutations ./pkg/mcp)
pytest tests/test_day_zero.py
pytest tests/daemon_pg/handlers/workflow_loop/test_claim_next.py
```

Results:

- `go test ./pkg/mutations ./pkg/mcp` from `go/`: passed.
- `pytest tests/test_day_zero.py`: 12 passed.
- `pytest tests/daemon_pg/handlers/workflow_loop/test_claim_next.py`: 5 passed.

## Closure Commands

Before final closure, run the same focused validation plus the repository's
standard gates:

```bash
(cd go && go test ./pkg/mutations ./pkg/mcp)
pytest tests/test_day_zero.py
pytest tests/daemon_pg/handlers/workflow_loop/test_claim_next.py
make lint
make typecheck
make test
```

The focused commands are sufficient evidence for REM-001, REM-002, and
REM-009 source closure. The Makefile gates are the broader release-safety
checks for unrelated drift.
