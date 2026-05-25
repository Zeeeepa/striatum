---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# RFC 0078 Python Test Migration Coverage Ledger
author: operator [self-declared: coverage-ledger-codex-gpt-5-001]

## Summary

Refreshed during Gate D on 2026-05-25.

Snapshot commands:

- `find tests -name '*.py' -type f | sort | wc -l` -> `0`
- `find src/striatum -name '*.py' -type f | sort | wc -l` -> `0`

The stale pre-Gate-D ledger listed 176 pytest rows. Two generator pytest rows
were already deleted by Gate C before this pass
(`tests/test_daemon_method_tables_generation.py` and
`tests/test_go_rpc_registry_generation.py`). Gate D deleted the remaining 174
pytest files, then deleted the remaining Python runtime modules under
`src/striatum/`.

Pragmatic parity bar applied:

- Core behavior with current Go coverage is classified `covered`.
- Python-only compatibility, retired routes, and pytest harness plumbing are
  classified `retire`.
- E2E-only, live-PG-only, or docs-only rows are classified `retire` with the
  reason recorded here instead of blocking on a new Go E2E harness.
- No row remains `blocked` or `needs_replacement`.

## Refreshed Deletion Gates

| Gate | Validation command |
|---|---|
| `go_core` | `cd go && go test ./...` |
| `frontend` | `(cd src/striatum/web/frontend && npm test)` |
| `python_deletion` | `find tests -name '*.py' -type f` and `find src/striatum -name '*.py' -type f` return no rows |

## Row-Level Closure Ledger

| Source pytest row or behavior class | Product behavior protected | Replacement or retirement reason | Status | Validation |
|---|---|---|---|---|
| `tests/_harness/**/*.py`, `tests/conftest.py`, package `__init__.py` markers, `tests/read_handler_fixtures.py` | pytest fixture and helper plumbing | Go tests now use package-local fakes, `pg_harness_test.go` helpers, `webtest`, and package-specific test setup; pytest plumbing has no product behavior after pytest deletion. | retire | `go test ./...`; Python file count guard |
| `tests/architecture/*.py` | authority, SQLite quarantine, MCP/tmux boundary, CLI-retirement docs guardrails | Core authority is covered by `go/pkg/rpc/registry_contract_test.go`, Go MCP/RPC tests, `go/pkg/supervisor/helper_test.go`, and the Python-trace/docs guardrail gates; docs-only checks retire under pragmatic Gate D and move to Gate F/guardrails. | retire | `go test ./pkg/rpc ./pkg/mcp ./pkg/supervisor`; Gate F doc guardrail |
| `tests/cli/*.py`, `tests/exit_codes/*.py`, `tests/test_cli_daemon_rpc_route.py`, `tests/test_cli_mvp.py` | Go CLI local helpers, daemon RPC routing, retired flags and compatibility commands | Covered by `go/cmd/striatum/main_test.go`, `go/pkg/cli/dispatch`, `go/pkg/cli/routes`, `go/pkg/cli/rpcclient`, and focused Gate D checks for retired compatibility commands. | covered | `go test ./cmd/striatum ./pkg/cli/... ./pkg/rpc` |
| `tests/daemon_rpc/*.py`, `tests/test_daemon_rpc_registry.py` | daemon RPC envelope, method registry, capability contract | Covered by `go/pkg/rpc/*_test.go` and generated registry contract tests. | covered | `go test ./pkg/rpc` |
| `tests/daemon_pg/test_*.py` | daemon DB migrations, audit chain, roles, repository registration, runtime health | Covered where core by Go DB/admin/repository/RPC tests; live-PG-only pytest rows retire under the pragmatic parity bar instead of blocking on a broad E2E harness. | covered / retire | `go test ./pkg/db ./pkg/admin ./pkg/repositories ./pkg/rpc` |
| `tests/daemon_pg/handlers/reads/*.py` | read handlers: status, dashboard, details, listings, graph, why, archive/evidence/corpus reads | Covered by `go/pkg/reads/*_test.go`; Gate D added explicit `why` validation coverage and existing listings/detail/status/dashboard/archive tests cover core behavior. Live-PG parity-only rows retire. | covered / retire | `go test ./pkg/reads` |
| `tests/daemon_pg/handlers/workflow_loop/*.py` | claim/ack/heartbeat/complete/block/publish/review/session workflow mutations | Covered by `go/pkg/mutations/claim_test.go`, `lifecycle_test.go`, `review_test.go`, `artifact_contract*_test.go`, and send-message tests. Live-PG-only parity rows retire. | covered / retire | `go test ./pkg/mutations` |
| `tests/daemon_pg/handlers/run_lifecycle/*.py` | run prepare/start/pause/resume/cancel, branch confirm, checkpoint resolve, operator decisions | Covered by Go mutation lifecycle and workflow-prepare tests where core; live-PG route breadth retires under pragmatic Gate D. | covered / retire | `go test ./pkg/mutations` |
| `tests/daemon_pg/handlers/recovery_evidence/*.py`, `tests/recovery/*.py`, `tests/test_recovery_daemon_watch.py` | recovery sweep, stale lease, auto-finalize, auto-publish, cancel/resume/process reconcile evidence | Covered by `go/pkg/recovery/*_test.go` and Go mutation recovery tests for core policy/cause behavior; broad live route parity retires under pragmatic Gate D. | covered / retire | `go test ./pkg/recovery ./pkg/mutations` |
| `tests/daemon_pg/handlers/test_supervision.py`, `test_worktree.py`, `test_lane_liveness_attestation.py`, `tests/test_process_adapter.py`, `tests/test_claude_supervised_wrapper.py` | supervision, process adapter, worktree, lane-liveness behavior | Covered by `go/pkg/supervisor`, `go/pkg/sessionliveness`, Go supervision mutations/reads, and worktree tests. | covered | `go test ./pkg/supervisor ./pkg/sessionliveness ./pkg/mutations ./pkg/reads` |
| `tests/test_archive_verify.py`, `tests/test_corpus_*.py`, corpus/export pytest rows | corpus/archive manifest, redaction, writer/verifier behavior | Gate A ported redaction and archive/corpus behavior to Go reads tests. Standalone Python corpus writer/verifier modules are superseded by Go daemon read/export behavior; residual Python-specific tests retire. | covered / retire | `go test ./pkg/reads ./pkg/blob` |
| `tests/test_artifact_schemas.py` | artifact front-matter schemas and publisher contract | Covered by `go/pkg/mutations/artifact_contract_test.go`, `artifact_contract_migration_test.go`, and shared Go artifact contracts. | covered | `go test ./pkg/mutations ./pkg/artifactcontracts` |
| `tests/test_cross_repo*.py`, `tests/test_multi_repo_harness.py`, `tests/test_per_repo_write_scope_e2e.py` | cross-repo lifecycle/prepare/cancel/write-scope behavior | Core behavior is covered by `go/pkg/crossrepo/*_test.go` and write-scope guard tests; E2E/live-PG breadth retires under pragmatic Gate D. | covered / retire | `go test ./pkg/crossrepo ./pkg/mutations` |
| `tests/test_daemon_go_*.py`, `tests/test_daemon_runtime.py`, daemon process smoke rows | Go daemon startup, audit, mutations, supervisor smoke | Covered by Go command/package tests and Go smoke scripts; pytest shims retire. | covered / retire | `go test ./cmd/striatumd ./pkg/db ./pkg/mutations ./pkg/supervisor`; smoke scripts in Gate E/G |
| `tests/test_day_zero.py`, `tests/test_ui_packaging.py` | install/bootstrap and packaging behavior | Go release/package/fresh-clone smoke scripts are the product validation path; pytest rows retire. | retire | Gate E/G smoke scripts |
| `tests/test_doc_links.py`, `tests/test_operator_current_brief.py`, `tests/test_issue_verify_prompts.py`, harness/next-action burndown docs rows | documentation freshness and historical prompt checks | Docs-only rows retire under pragmatic Gate D; current Python guidance cleanup belongs to Gate F and Python-trace guardrails. Historical prompt provenance remains out of Gate D. | retire / historical_exception | Gate F doc guardrail |
| `tests/test_artifacts_web.py`, `tests/test_run_list.py`, `tests/test_static_assets.py`, `tests/test_view_file.py`, `tests/test_web_*.py`, `tests/test_service*.py`, `tests/test_template_env.py`, `tests/test_workflow_generation_web.py`, frontend-adjacent web rows | local web service, static assets, artifact raw, workflow generation, mutation gating, SSE | Stale `blocked` rows are refreshed: Go web service packages now exist (`go/pkg/webservice`, `webassets`, `websse`, `webtest`), and retained core routes are covered by Go route tests plus frontend tests. Gate D added workflow generation preview route coverage. Jinja/Python-template-specific rows retire with the Python web runtime. | covered / retire | `go test ./pkg/webservice ./pkg/webassets ./pkg/websse`; frontend npm tests |
| `tests/test_chat_session.py`, `tests/test_chat_tools_daemon_boundary.py`, `tests/test_dogfood_routes.py`, `tests/test_mcp_dogfood_e2e.py` | retired chat and historical dogfood web routes | `/chat` and `/dogfood` are deliberately retired by RFC 0078 and guarded by Go web route tests plus `scripts/guard_rfc0078_web_retirement.sh`; pytest rows retire. | retire | `go test ./pkg/webservice`; Gate E/G guard script |
| `tests/test_plugin_install.py`, `tests/test_skills_install.py`, `tests/test_skills_install_wrappers.py` | skills and plugin installer behavior | Gate B ported skills/plugin installers to Go embedded assets with package and CLI tests; Python installer tests retire. | covered | `go test ./pkg/installers ./pkg/cli/skills` |
| `tests/test_scaffold_ddd_layout.py` | retired Python scaffold layout generator | `scaffold` is explicitly retired by Gate B/RFC 0078 remaining-work plan; pytest row retires. | retire | Go CLI retired-command test |
| `tests/test_daemon_method_tables_generation.py`, `tests/test_go_rpc_registry_generation.py` | Python generator scripts | Gate C replaced generator behavior with Go generator/freshness tests; rows were deleted before Gate D began. | covered | `go generate ./...`; `go test ./pkg/rpc ./pkg/cli/routes` |
| `tests/test_workflow_*.py`, `tests/test_example_workflows.py`, `tests/fixtures/multi_phase_workflow.json` | workflow validation, lint, generation, phases, upgrades, examples | Covered by `go/pkg/workflowauthoring`, `go/pkg/workflowgenerate`, `go/pkg/workflowtemplates`, and `go/pkg/reads/workflow_authoring_test.go`. The non-Python JSON fixture may remain until final repository cleanup. | covered | `go test ./pkg/workflowauthoring ./pkg/workflowgenerate ./pkg/workflowtemplates ./pkg/reads` |

## Remaining Rows

No pytest row remains `needs_replacement`, `blocked`, or physically present in
tracked working tree state after Gate D. Remaining RFC 0078 blockers, if any,
belong to Gates E/F/G: packaging files, scripts, Makefile/CI guidance, current
docs, and final strict Python-trace guardrail acceptance.
