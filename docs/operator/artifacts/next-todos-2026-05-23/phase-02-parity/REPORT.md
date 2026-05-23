---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/architecture/CLI_RETIREMENT_PARITY.md", "tests/test_mcp_mutation_capabilities.py", "tests/test_service.py"]
---

# Phase 2 Parity Gaps
author: operator [self-declared: codex-driver]

## Result

Added exact MCP dispatch coverage for the next registry-only workflow-control
methods in `tests/test_mcp_mutation_capabilities.py`, including
`git.commit_apply`, `review.verdict`, `review.override`, the remaining
recovery methods, worktree methods, supervise methods, and `cross_repo.cancel`.

Added `tests/test_service.py::test_v1_invoke_override_verdict_web_context_routes_daemon_rpc`
to prove the existing web-context-protected `override-verdict` invoke path
routes to daemon RPC method `review.override` instead of falling back to
local CLI/API dispatch.

Updated `docs/architecture/CLI_RETIREMENT_PARITY.md` so the affected rows now
cite exact MCP evidence and, for `review.verdict` / `review.override`, exact
service-route evidence. No CLI verb was hidden, deleted, or marked retireable.

## Validation

- `PYTHONPATH=src .venv/bin/python -m pytest -q tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc tests/test_service.py::test_v1_invoke_daemon_mapped_mutation_uses_daemon_rpc_not_api_invoke tests/test_service.py::test_v1_invoke_override_verdict_web_context_routes_daemon_rpc tests/architecture/test_cli_retirement_parity.py`
- `git diff --check`

The bare `python3 -m pytest` path is unavailable in this shell because pytest
is installed in `.venv`, so validation used the repo virtualenv.
