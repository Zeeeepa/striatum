---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/architecture/CLI_RETIREMENT_PARITY.md", "tests/test_web_workflow_accepted_risks.py", "tests/test_service.py", "tests/test_web_escalations.py"]
---

# Phase 2B UI Parity Ledger
author: operator [self-declared: codex-driver]

## Result

Refreshed the UI parity ledger only where active tests already prove the local
web replacement path.

Changed rows:

- `workflow accept-risk` now cites the active daemon-backed web mutation test
  and moves to `replaced_by_mcp_ui` / `ui_exact`.
- `run.pause`, `run.resume`, `run.cancel`, `run.retry-job`,
  `recovery.cancel-job`, and `branch confirm` now cite exact MCP coverage plus
  their existing `tests/test_service.py` UI coverage.

No CLI verbs were hidden or deleted. Retirement gates remain blocked on the
separate retirement proof/cutover step rather than on missing method-level
MCP tests.

## Remaining UI Gaps

The ledger still intentionally blocks rows with missing UI surfaces, including
recovery operations beyond job cancel, worktree controls, supervisor controls,
cross-repo cancel, and local commit apply.

## Validation

```bash
.venv/bin/python -m pytest -q tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc tests/architecture/test_cli_retirement_parity.py
.venv/bin/python -m ruff check tests/test_mcp_mutation_capabilities.py tests/architecture/test_cli_retirement_parity.py
```

Both commands passed.
