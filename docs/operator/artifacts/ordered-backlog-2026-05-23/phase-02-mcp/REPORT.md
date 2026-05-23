---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/architecture/CLI_RETIREMENT_PARITY.md", "tests/test_mcp_mutation_capabilities.py"]
---

# Phase 2A MCP Exact Parity
author: operator [self-declared: codex-driver]

## Result

Added exact MCP `tools/call` dispatch coverage for existing daemon-backed
workflow-control methods that were previously represented only by generic
registry visibility coverage.

Covered methods:

- `run.pause`
- `run.resume`
- `run.cancel`
- `run.retry_job`
- `recovery.cancel_job`
- `branch.confirm`
- `session.close`
- `work.release`
- `work.block`
- `review.submit`

The test proves `dispatch_mcp_tool_call` authorizes the method, constructs a
daemon RPC envelope with the requested method and capability token, and routes
it through the daemon RPC router over transport `mcp` without requiring a
handshake.

## Validation

```bash
.venv/bin/python -m pytest -q tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc tests/architecture/test_cli_retirement_parity.py
.venv/bin/python -m ruff check tests/test_mcp_mutation_capabilities.py tests/architecture/test_cli_retirement_parity.py
```

Both commands passed.
