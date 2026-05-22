---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/TODO.md", "docs/rfcs/0070-daemon-client-service-boundary.md", "tests/architecture/test_authority_guardrails.py"]
---

# TODO 63: Daemon Client/Service Boundary Cleanup Plan

## Objective

Complete the daemon client/service boundary by removing remaining direct PostgreSQL connection imports from client-side modules (`src/striatum/cli/`, `src/striatum/day_zero.py`), as mandated by RFC 0070. This ensures that only the daemon handles PostgreSQL connections for production workflows, and clients interact with live state exclusively through daemon RPC.

## Residuals Identification

The following locations in the Python codebase still perform direct PostgreSQL connections and are listed in the `DIRECT_PG_BOOTSTRAP_IMPORT_ALLOWLIST` in `tests/architecture/test_authority_guardrails.py`:

1.  **`src/striatum/cli/workflow.py::_running_runs_for_workflow_pg`**: Opens a direct PG connection to check for active runs before a workflow upgrade.
2.  **`src/striatum/day_zero.py::_lookup_repository_id`**: Opens a direct PG connection to resolve a repository root to its `repository_id`.
3.  **`src/striatum/day_zero.py::adopt`**: Directly calls `repo_add_pg` after establishing a PG connection.
4.  **`src/striatum/cli/dispatch.py::_dispatch_cross_repo`**: Contains dead code with PG imports that was previously retired in favor of daemon routing.
5.  **`src/striatum/daemon_pg/client_admin.py`**: `daemon doctor` and `daemon stop` (audit) use direct PG connections.

## Implementation Tracks

### Track 1: Workflow Upgrade RPC Migration

1.  **Add `workflow.active_runs` Daemon Method**:
    *   Implement a new read handler `src/striatum/daemon_pg/handlers/reads/workflow_active_runs.py` (or similar) that takes a `source_path` and returns a list of non-terminal `run_id` values.
    *   Register the method in `contracts/daemon_methods.json` and `src/striatum/daemon_rpc/registry.py`.
    *   (Future) Port this handler to the Go daemon for parity.

2.  **Refactor `striatum.cli.workflow`**:
    *   Update `_running_runs_for_workflow_pg` to attempt a daemon RPC call to `workflow.active_runs` if the daemon socket is reachable.
    *   Maintain the current "fail closed" behavior if the daemon is unreachable or the check fails, ensuring `workflow upgrade` remains safe.

### Track 2: Day-Zero & Bootstrap Cleanup

1.  **Update `_lookup_repository_id` in `src/striatum/day_zero.py`**:
    *   Use `repo.resolve` daemon RPC (via `striatum.cli.daemon_rpc_route._resolve_repository_id`) if the daemon is reachable.
    *   Reserve direct PG connection as a fallback only when the daemon is explicitly not yet running (e.g., during the initial `adopt` call).

2.  **Update `adopt` in `src/striatum/day_zero.py`**:
    *   If the daemon is reachable, route the repository registration through the `repo.add` daemon method instead of calling `repo_add_pg` directly.

### Track 3: Dead Code & Allowlist Reduction

1.  **Clean up `src/striatum/cli/dispatch.py`**:
    *   Remove the dead code and direct PG imports in `_dispatch_cross_repo`. Verify that `cross-repo` commands correctly route through the daemon as designed.
    *   Remove `striatum.daemon_pg.connection.doctor` from the `_dispatch_daemon` allowlist if it can be replaced by a daemon-routed `doctor` call when the daemon is active.

2.  **Audit `src/striatum/daemon_pg/client_admin.py`**:
    *   Evaluate if `daemon stop` auditing can be moved to a pre-stop RPC call (`daemon.audit_stop`).
    *   Keep `daemon doctor` direct-mode only for recovery/onboarding scenarios where the daemon cannot start.

3.  **Update Architecture Guardrails**:
    *   Remove the successfuly migrated functions from `DIRECT_PG_BOOTSTRAP_IMPORT_ALLOWLIST` in `tests/architecture/test_authority_guardrails.py`.

## Verification Strategy

1.  **Unit Tests**: Add tests to `tests/test_workflow_upgrade.py` and `tests/test_day_zero.py` that monkeypatch the daemon RPC to simulate active runs or repository resolution.
2.  **Integration Tests**: Use `tests/test_cli_daemon_rpc_route.py` to verify that no direct PG connections are attempted during normal CLI operations when the daemon is active.
3.  **Architecture Guardrails**: Run `pytest tests/architecture/test_authority_guardrails.py` to ensure the allowlist has been successfully reduced.

## Sequencing

1.  **Phase 1**: Add `workflow.active_runs` to the daemon and migrate the workflow upgrade guard.
2.  **Phase 2**: Refactor `day_zero` and `adopt` to prefer RPC resolution.
3.  **Phase 3**: Remove dead code in `dispatch.py` and finalize the architecture allowlist reduction.
