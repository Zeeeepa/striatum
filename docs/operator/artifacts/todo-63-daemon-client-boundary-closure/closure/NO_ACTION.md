---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/rfcs/0070-daemon-client-service-boundary.md", "docs/TODO.md", "docs/DECISION_LOG.md", "tests/test_cli_daemon_rpc_route.py", "tests/test_service.py", "tests/test_chat_tools_daemon_boundary.py", "tests/test_mcp_mutation_capabilities.py", "tests/test_mcp_dogfood_e2e.py", "go/pkg/mcp/http_test.go"]
---

# TODO 63 Daemon Client Boundary No-Action Closure
author: boundary-closer-codex-gpt-5-001

## Verdict

No production source change is required for TODO 63 in this slice. The
accepted supported path is primitive daemon methods, not reintroduced dogfood
composites. A narrow active chat-tool daemon-routing guardrail was added
because the older chat-tool test module is currently skipped as a legacy
fixture.

D110 removed `dogfood.publish_on_behalf` and `dogfood.surgical_recovery` from
the production daemon contract because the historical composites were
SQLite-bound. D112 removed `apply.reviewed_patch` for the same production
contract hygiene reason. RFC 0070's current implementation notes already name
the primitive-method posture: removed composite names should audit as
`method_unknown` unless a later PostgreSQL-native composite is accepted.

## Boundary Evidence

- `repo.resolve` exists in the daemon method contract as a daemon-global read
  method, and the Go repository service registers it.
- `tests/test_cli_daemon_rpc_route.py::test_client_boundary_source_does_not_import_daemon_pg_lookup`
  pins the CLI route helper and service daemon helper against importing
  `striatum.daemon_pg.connection` or `resolve_config` for normal repository
  resolution.
- `/v1/invoke` daemon-mapped read and mutation tests in `tests/test_service.py`
  monkeypatch `striatum.api.invoke` to fail and assert service calls route
  through `striatum.service_daemon.call_repo_method`.
- `tests/test_chat_tools_daemon_boundary.py` applies the same
  `striatum.api.invoke` tripwire to mapped chat read and lifecycle mutation
  tools.
- `tests/test_mcp_mutation_capabilities.py` verifies daemon MCP `tools/list`
  is capability-filtered, hides local workflow-file authoring methods, and
  excludes the removed dogfood composites.
- `tests/test_mcp_dogfood_e2e.py` verifies direct MCP calls to
  `dogfood.publish_on_behalf` and `dogfood.surgical_recovery` return
  `method_unknown` and audit the denied calls.
- `go/pkg/mcp/tools.go` refuses hidden production tools before daemon RPC
  dispatch, and `go/pkg/mcp/http_test.go` covers hidden-tool denial for both
  read-only and write-capable tokens.

## Residual Classification

The remaining direct PostgreSQL imports guarded by
`tests/architecture/test_authority_guardrails.py` are direct bootstrap/admin
or local authoring paths, not ordinary RFC 0070 client/service repository
resolution. They remain explicit in the command-authority matrix and should
not be changed from this closure artifact.

Local `striatum.api.invoke` also remains intentionally available for local
workflow-file authoring and explicit fixture compatibility. The RFC 0070
boundary is that production mapped reads and mutations do not use it as a live
state authority path.

## No-Action Decision

Do not add a PostgreSQL-native replacement for `dogfood.publish_on_behalf` or
`dogfood.surgical_recovery` in this TODO 63 closure. Operators should use the
existing primitive daemon methods:

- `work.ack`
- `artifact.publish`
- `review.verdict`
- `work.complete`
- `recovery.*` methods for stale leases, requeue, resume, process reconcile,
  auto-publish, sweep, cancel, and auto-finalize

If an operator-composite method is later needed, it should be introduced by a
new accepted RFC or decision that defines method names, capability
requirements, audit semantics, route-map updates, command-authority matrix
updates, and MCP visibility behavior.

## Validation Evidence

Commands run for this closure:

- `PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=src .venv/bin/python -m striatum.cli workflow validate docs/operator/workflows/todo-63-daemon-client-boundary-closure/workflow.json`
  -> valid.
- `PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=src .venv/bin/python - <<'PY' ... validate_artifact_front_matter(...)`
  -> work-plan and synthesis front matter valid.
- `PYTHONDONTWRITEBYTECODE=1 PYTEST_ADDOPTS='-p no:cacheprovider' PYTHONPATH=src .venv/bin/python -m pytest -q tests/test_cli_daemon_rpc_route.py::test_client_boundary_source_does_not_import_daemon_pg_lookup tests/test_service.py::test_v1_invoke_daemon_mapped_mutation_uses_daemon_rpc_not_api_invoke tests/test_service.py::test_v1_invoke_existing_ui_gap_routes_daemon_rpc_not_api_invoke tests/test_service.py::test_v1_invoke_daemon_mapped_read_uses_daemon_rpc_not_api_invoke tests/test_chat_tools_daemon_boundary.py tests/test_mcp_mutation_capabilities.py::test_daemon_mcp_tools_list_filters_by_capability tests/test_mcp_mutation_capabilities.py::test_daemon_mcp_tools_match_registered_non_deprecated_authorized_methods tests/test_mcp_mutation_capabilities.py::test_daemon_mcp_tools_list_excludes_removed_dogfood_composites`
  -> 12 passed.
- `PYTHONDONTWRITEBYTECODE=1 PYTEST_ADDOPTS='-p no:cacheprovider' PYTHONPATH=src .venv/bin/python -m pytest -q tests/test_mcp_dogfood_e2e.py::test_mcp_publish_on_behalf_is_removed_from_production_contract tests/test_mcp_dogfood_e2e.py::test_mcp_surgical_recovery_is_removed_from_production_contract`
  -> 2 passed.
- `PYTHONDONTWRITEBYTECODE=1 PYTEST_ADDOPTS='-p no:cacheprovider' PYTHONPATH=src .venv/bin/python -m pytest -q tests/architecture/test_authority_guardrails.py::test_direct_postgres_bootstrap_plane_is_explicitly_allowlisted tests/daemon_rpc/test_daemon_method_contract.py::test_every_contract_method_appears_in_method_registry tests/test_daemon_rpc_registry.py::test_registry_includes_recovery_and_repository_scope_mode`
  -> 3 passed.
- `go test ./pkg/mcp -run 'TestHTTPHandler(DeniedTokenCannotCallHiddenUnauthorizedMethod|WriteTokenCannotCallHiddenProductionTool)'`
  from `go/` -> passed.
- `PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=src .venv/bin/ruff check tests/test_chat_tools_daemon_boundary.py`
  -> passed.

## Shared Updates To Queue

These shared files should be updated by a later operator pass, not by this
scoped closure:

- `docs/TODO.md`: mark item 63 done/closed.
- `docs/ROADMAP.md`: remove TODO 63 from residual cleanup lists.
- `docs/operator/BRIEF.md`: mention TODO 63 closure in the next brief.
- `docs/rfcs/0070-daemon-client-service-boundary.md`: optionally move status
  from `mostly implemented` to a closed status if RFC status is being kept in
  lockstep with operator closure artifacts.
