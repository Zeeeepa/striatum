---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# RFC 0078 Python Test Migration Coverage Ledger
author: operator [self-declared: coverage-ledger-codex-gpt-5-001]

## Summary

Snapshot command: `find tests -name '*.py' -type f | sort | wc -l` -> `176`.

Status counts in this ledger:

- `covered`: Go, shell, or frontend coverage exists for the core behavior.
- `needs_replacement`: active pytest behavior still needs a Go/shell/browser replacement before deletion.
- `retire`: pytest-only compatibility surface can be removed after the named gate because RFC 0078 retires the behavior.
- `historical_exception`: historical/provenance-only behavior can survive only under an explicit historical policy.
- `blocked`: deletion is blocked by a product or implementation decision.

## Deletion Gates

| Gate | Validation command |
|---|---|
| `pg_harness` | `cd go && go test ./pkg/db ./pkg/rpc ./pkg/repositories ./pkg/crossrepo ./pkg/mutations ./pkg/recovery ./pkg/reads` |
| `cli_tests` | `cd go && go test ./cmd/striatum ./pkg/rpc` and `scripts/go_package_smoke.sh` |
| `web_tests` | `npm test -- --run src/__tests__/api-client.test.ts` from `src/striatum/web/frontend`; Go web route replacement remains blocked until Go service routes land |
| `workflow_artifact_tests` | `cd go && go test ./pkg/workflowauthoring ./pkg/workflowgenerate ./pkg/workflowtemplates ./pkg/mutations` |
| `corpus_archive_tests` | `cd go && go test ./pkg/reads ./pkg/blob` |
| `packaging_smoke` | `scripts/go_package_smoke.sh`, `scripts/go_release_metadata_check.sh`, `scripts/go_fresh_clone_smoke.sh` |
| `final_deletion_readiness` | `scripts/check_no_python_test_surface.sh` or successor Python-trace guardrail plus the aggregate command named in final readiness |

## Row-Level Ledger

| Source pytest file or behavior class | Product behavior protected | Current replacement | Required replacement | Owner | Status | Deletion gate and command |
|---|---|---|---|---|---|---|
| `tests/_harness/__init__.py` | pytest harness package marker | none | delete with `tests/_harness` after Go harness helpers cover active setup | final_deletion_readiness | retire | `final_deletion_readiness`; trace guardrail |
| `tests/_harness/audit.py` | audit-chain fixture helpers | `go/pkg/db/audit_race_test.go` partial | reusable Go audit fixture for event-chain tests | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/db ./pkg/mutations` |
| `tests/_harness/daemon.py` | Python daemon process fixture | `go/cmd/striatumd/*_test.go` partial | Go daemon process harness or shell daemon smoke | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./cmd/striatumd` |
| `tests/_harness/mcp.py` | MCP fixture/client helpers | `go/pkg/mcp/*_test.go` | Go MCP helper parity for live capability-token calls | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/mcp` |
| `tests/_harness/multi_repo.py` | multi-target repository setup | `go/pkg/crossrepo/*_test.go` partial | reusable Go cross-repo fixture | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/crossrepo` |
| `tests/_harness/pg.py` | PostgreSQL fixture lifecycle | package-local Go fakes only | reusable Go PostgreSQL test-support package | pg_harness | blocked | `pg_harness`; needs real PG harness or accepted fake-only policy |
| `tests/_harness/repos.py` | target repository fixture setup | `go/pkg/repositories/*_test.go` partial | Go target-repository fixture helpers | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/repositories` |
| `tests/_harness/scope.py` | write-scope fixture helpers | `go/pkg/mutations/write_scope_guard_test.go` | reusable Go write-scope helper | workflow_artifact_tests | covered | `workflow_artifact_tests`; `cd go && go test ./pkg/mutations` |
| `tests/_harness/tokens.py` | capability-token fixture helpers | `go/pkg/admin/tokens_test.go`, `go/pkg/rpc/*test.go` partial | reusable Go token helper | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/admin ./pkg/rpc` |
| `tests/conftest.py` | global pytest fixtures | none | replace fixture duties in Go test-support or retire with pytest deletion | final_deletion_readiness | needs_replacement | `final_deletion_readiness`; aggregate command |
| `tests/read_handler_fixtures.py` | Python read-handler parity fixture rows | package-local Go fake runners | shared Go read fixture or retire parity-only helper | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/reads` |
| `tests/architecture/test_authority_guardrails.py` | daemon authority docs/code guardrails | `go/pkg/rpc/registry_contract_test.go` partial | Go/shell authority guardrail for active docs and contracts | final_deletion_readiness | needs_replacement | `final_deletion_readiness`; Python-trace/authority guardrails |
| `tests/architecture/test_cli_retirement_parity.py` | retired CLI parity decisions | `go/cmd/striatum/main_test.go` partial | Go CLI retirement matrix | cli_tests | needs_replacement | `cli_tests`; `cd go && go test ./cmd/striatum` |
| `tests/architecture/test_go_helper_boundary.py` | Go helper boundary | `go/pkg/supervisor/helper_test.go` partial | Go-only architecture guardrail | final_deletion_readiness | needs_replacement | `final_deletion_readiness`; trace guardrail |
| `tests/architecture/test_legacy_sqlite_quarantine.py` | legacy SQLite quarantine | docs/RFC decisions only | shell guardrail forbidding active SQLite state | final_deletion_readiness | needs_replacement | `final_deletion_readiness`; trace guardrail |
| `tests/architecture/test_mcp_cutover_docs.py` | MCP cutover docs consistency | `go/pkg/mcp/*_test.go` partial | docs guardrail in shell/Go | final_deletion_readiness | needs_replacement | `final_deletion_readiness`; trace guardrail |
| `tests/architecture/test_tmux_authority_boundary.py` | tmux non-authority boundary | none | shell/docs guardrail | final_deletion_readiness | needs_replacement | `final_deletion_readiness`; trace guardrail |
| `tests/cli/__init__.py` | pytest CLI package marker | none | delete with CLI pytest package | cli_tests | retire | `cli_tests`; `cd go && go test ./cmd/striatum` |
| `tests/cli/test_daemon_core.py` | daemon CLI lifecycle | `go/cmd/striatumd/*_test.go` partial | Go CLI daemon subcommands or explicit retirement | cli_tests | needs_replacement | `cli_tests`; `cd go && go test ./cmd/striatum ./cmd/striatumd` |
| `tests/cli/test_daemon_doctor_without_daemon.py` | doctor refusal without daemon | `go/pkg/reads/doctor_test.go` partial | Go CLI route test for refusal envelope | cli_tests | needs_replacement | `cli_tests`; `cd go && go test ./cmd/striatum ./pkg/reads` |
| `tests/cli/test_daemon_sqlite_import_retired.py` | retired SQLite import command | RFC 0043/RFC 0078 | Go CLI unknown/retired command test | cli_tests | retire | `cli_tests`; `cd go && go test ./cmd/striatum` |
| `tests/cli/test_dispatch_daemon_doctor.py` | CLI dispatch to daemon doctor | `go/pkg/rpc/registry_contract_test.go` partial | Go CLI/RPC route dispatch coverage | cli_tests | needs_replacement | `cli_tests`; `cd go && go test ./cmd/striatum ./pkg/rpc` |
| `tests/cli/test_no_daemon_retired.py` | `--no-daemon` retired | AGENTS/RFC 0043 | Go CLI refusal/unknown test | cli_tests | retire | `cli_tests`; `cd go && go test ./cmd/striatum` |
| `tests/cli/test_parser_help.py` | CLI help/parser | `go/cmd/striatum/main_test.go` | none | cli_tests | covered | `cli_tests`; `cd go && go test ./cmd/striatum` |
| `tests/cli/test_self_update.py` | Python self-update command | `go/cmd/striatum/main_test.go` unknown-command coverage | keep retired unless accepted Go update command appears | cli_tests | retire | `cli_tests`; `cd go && go test ./cmd/striatum` |
| `tests/daemon_rpc/__init__.py` | pytest RPC package marker | none | delete with pytest package | pg_harness | retire | `pg_harness`; trace guardrail |
| `tests/daemon_rpc/test_daemon_method_contract.py` | daemon method contract parity | `go/pkg/rpc/registry_contract_test.go` | none | pg_harness | covered | `pg_harness`; `cd go && go test ./pkg/rpc` |
| `tests/exit_codes/__init__.py` | pytest exit-code package marker | none | delete with pytest package | cli_tests | retire | `cli_tests`; trace guardrail |
| `tests/exit_codes/test_rfc0043_refusals.py` | daemon-required refusal exit codes | `go/pkg/rpc/*_test.go` partial | Go CLI process-level exit-code tests | cli_tests | needs_replacement | `cli_tests`; `cd go && go test ./cmd/striatum ./pkg/rpc` |
| `tests/recovery/test_auto_finalize_causes.py` | auto-finalize cause mapping | `go/pkg/recovery/*_test.go`, `go/pkg/mutations/workflow_accepted_risk_test.go` partial | Go mutation/read tests for all causes | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/recovery ./pkg/mutations` |
| `tests/daemon_pg/handlers/_parity.py` | Python/Go read-handler parity harness | Go read package tests | retire once Python path gone | pg_harness | retire | `pg_harness`; `cd go && go test ./pkg/reads` |
| `tests/daemon_pg/handlers/reads/test_archive.py` | archive read/create behavior | `go/pkg/reads/archive_test.go` | none | corpus_archive_tests | covered | `corpus_archive_tests`; `cd go && go test ./pkg/reads` |
| `tests/daemon_pg/handlers/reads/test_corpus_export.py` | corpus export read behavior | `go/pkg/reads/corpus_migration_test.go` | add redaction-depth tests if corpus writer remains | corpus_archive_tests | covered | `corpus_archive_tests`; `cd go && go test ./pkg/reads` |
| `tests/daemon_pg/handlers/reads/test_doctor.py` | doctor read behavior | `go/pkg/reads/doctor_test.go` | none | pg_harness | covered | `pg_harness`; `cd go && go test ./pkg/reads` |
| `tests/daemon_pg/handlers/reads/test_escalations.py` | escalation reads | `go/pkg/reads/detail_escalation_test.go`, `escalation_resolve_test.go` partial | list/show parity cases | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/reads` |
| `tests/daemon_pg/handlers/reads/test_evidence_export.py` | evidence export | `go/pkg/reads/archive_test.go` render/path tests | live PG export test | corpus_archive_tests | needs_replacement | `corpus_archive_tests`; `cd go && go test ./pkg/reads` |
| `tests/daemon_pg/handlers/reads/test_job_detail.py` | job detail read | `go/pkg/reads/detail_test.go` | none | pg_harness | covered | `pg_harness`; `cd go && go test ./pkg/reads` |
| `tests/daemon_pg/handlers/reads/test_list_artifacts.py` | artifact listing | `go/pkg/reads/listings_test.go` | none | pg_harness | covered | `pg_harness`; `cd go && go test ./pkg/reads` |
| `tests/daemon_pg/handlers/reads/test_list_jobs.py` | job listing | `go/pkg/reads/listings_test.go` partial | explicit list jobs rows | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/reads` |
| `tests/daemon_pg/handlers/reads/test_list_read_handlers.py` | read-handler registry | `go/pkg/reads/reads.go`, `go/pkg/rpc/registry_contract_test.go` | Go read registration test | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/reads ./pkg/rpc` |
| `tests/daemon_pg/handlers/reads/test_list_runs.py` | run listing | `go/pkg/reads/listings_test.go` partial | explicit list runs rows | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/reads` |
| `tests/daemon_pg/handlers/reads/test_list_sessions.py` | session listing | `go/pkg/reads/listings_test.go` partial | explicit list sessions rows | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/reads` |
| `tests/daemon_pg/handlers/reads/test_list_workflows.py` | workflow listing | `go/pkg/reads/workflow_templates_test.go` partial | explicit list workflows rows | workflow_artifact_tests | needs_replacement | `workflow_artifact_tests`; `cd go && go test ./pkg/reads` |
| `tests/daemon_pg/handlers/reads/test_pg_read_dashboard.py` | dashboard read | `go/pkg/reads/dashboard_test.go`, `dashboard_all_test.go` | none | pg_harness | covered | `pg_harness`; `cd go && go test ./pkg/reads` |
| `tests/daemon_pg/handlers/reads/test_registration.py` | repository registration reads | `go/pkg/repositories/pg_harness_test.go` | live PG registration test | pg_harness | covered | `pg_harness`; `cd go && go test ./pkg/repositories` |
| `tests/daemon_pg/handlers/reads/test_run_detail.py` | run detail read | `go/pkg/reads/detail_test.go` | none | pg_harness | covered | `pg_harness`; `cd go && go test ./pkg/reads` |
| `tests/daemon_pg/handlers/reads/test_run_events.py` | run event read | `go/pkg/reads/detail_test.go` partial | explicit run.events test | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/reads` |
| `tests/daemon_pg/handlers/reads/test_run_graph.py` | run graph read | `go/pkg/reads/run_graph_test.go` | none | workflow_artifact_tests | covered | `workflow_artifact_tests`; `cd go && go test ./pkg/reads` |
| `tests/daemon_pg/handlers/reads/test_run_posture_verdicts.py` | posture verdict read | `go/pkg/reads/detail_test.go` partial | explicit posture verdict read test | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/reads` |
| `tests/daemon_pg/handlers/reads/test_run_summary.py` | run summary read | `go/pkg/reads/archive_test.go` partial | explicit run.summary test | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/reads` |
| `tests/daemon_pg/handlers/reads/test_status.py` | status read | `go/pkg/reads/status_test.go` | none | pg_harness | covered | `pg_harness`; `cd go && go test ./pkg/reads` |
| `tests/daemon_pg/handlers/reads/test_why.py` | why read | `go/pkg/reads/why.go` no direct test | explicit why read test | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/reads` |
| `tests/daemon_pg/handlers/recovery_evidence/__init__.py` | recovery pytest package marker | none | delete with package | pg_harness | retire | `pg_harness`; trace guardrail |
| `tests/daemon_pg/handlers/recovery_evidence/_helpers.py` | recovery evidence fixtures | `go/pkg/recovery/*_test.go` partial | Go recovery test helpers | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/recovery ./pkg/mutations` |
| `tests/daemon_pg/handlers/recovery_evidence/conftest.py` | recovery pytest fixtures | none | Go recovery setup helper | pg_harness | needs_replacement | `pg_harness`; focused recovery tests |
| `tests/daemon_pg/handlers/recovery_evidence/test_auto_finalize.py` | auto-finalize recovery | `go/pkg/recovery/scheduler_test.go` partial | Go mutation integration for auto-finalize | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/recovery ./pkg/mutations` |
| `tests/daemon_pg/handlers/recovery_evidence/test_auto_publish_stale_artifacts.py` | auto-publish stale artifacts | `go/pkg/mutations/recovery_auto_finalize.go` no direct parity | Go recovery mutation test | pg_harness | needs_replacement | `pg_harness`; focused recovery tests |
| `tests/daemon_pg/handlers/recovery_evidence/test_cancel_job.py` | recovery cancel job | `go/pkg/crossrepo/lifecycle_test.go` partial | Go recovery.cancel_job test | pg_harness | needs_replacement | `pg_harness`; focused recovery tests |
| `tests/daemon_pg/handlers/recovery_evidence/test_process_reconcile.py` | process reconcile | `go/pkg/supervisor/*_test.go`, `go/pkg/recovery/*_test.go` partial | Go recovery.process_reconcile test | pg_harness | needs_replacement | `pg_harness`; focused recovery tests |
| `tests/daemon_pg/handlers/recovery_evidence/test_requeue_stale.py` | requeue stale jobs | `go/pkg/recovery/sweep_test.go` partial | Go recovery.requeue_stale test | pg_harness | needs_replacement | `pg_harness`; focused recovery tests |
| `tests/daemon_pg/handlers/recovery_evidence/test_resume_blocker.py` | resume blocker | no direct Go parity | Go recovery.resume test | pg_harness | needs_replacement | `pg_harness`; focused recovery tests |
| `tests/daemon_pg/handlers/recovery_evidence/test_stale_leases.py` | stale lease read | `go/pkg/recovery/sweep_test.go` partial | Go recovery.stale_leases test | pg_harness | needs_replacement | `pg_harness`; focused recovery tests |
| `tests/daemon_pg/handlers/recovery_evidence/test_sweep.py` | recovery sweep | `go/pkg/recovery/sweep_test.go` | broaden to daemon RPC route | pg_harness | needs_replacement | `pg_harness`; focused recovery tests |
| `tests/daemon_pg/handlers/run_lifecycle/test_branch_confirm.py` | branch confirm mutation | `go/pkg/mutations/lifecycle_test.go` partial | explicit branch.confirm test | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/mutations` |
| `tests/daemon_pg/handlers/run_lifecycle/test_checkpoint_resolve.py` | checkpoint resolve | `go/pkg/mutations/lifecycle_test.go` partial | explicit checkpoint.resolve test | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/mutations` |
| `tests/daemon_pg/handlers/run_lifecycle/test_operator_decisions.py` | operator decisions | no direct Go parity | Go decision.record test | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/mutations` |
| `tests/daemon_pg/handlers/run_lifecycle/test_run_prepare.py` | run prepare | `go/pkg/mutations/run.go` tests partial | explicit run.prepare test | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/mutations` |
| `tests/daemon_pg/handlers/run_lifecycle/test_run_state.py` | run start/pause/resume/cancel | `go/pkg/mutations/lifecycle_test.go` | broaden live route coverage | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/mutations` |
| `tests/daemon_pg/handlers/test_lane_liveness_attestation.py` | lane liveness attestation | `go/pkg/sessionliveness/liveness_test.go`, `go/pkg/reads/dashboard_test.go` | none | pg_harness | covered | `pg_harness`; `cd go && go test ./pkg/sessionliveness ./pkg/reads` |
| `tests/daemon_pg/handlers/test_supervision.py` | supervision mutations | `go/pkg/mutations/supervision_test.go`, `go/pkg/reads/supervision_test.go` | none | pg_harness | covered | `pg_harness`; `cd go && go test ./pkg/mutations ./pkg/reads` |
| `tests/daemon_pg/handlers/test_worktree.py` | worktree mutations | `go/pkg/mutations/worktree_test.go`, `go/pkg/reads/worktree_list_test.go` | none | pg_harness | covered | `pg_harness`; `cd go && go test ./pkg/mutations ./pkg/reads` |
| `tests/daemon_pg/handlers/workflow_loop/test_ack_work.py` | work ack | `go/pkg/mutations/claim_test.go` partial | explicit work.ack test | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/mutations` |
| `tests/daemon_pg/handlers/workflow_loop/test_block_job.py` | work block | `go/pkg/mutations/lifecycle_test.go` partial | explicit work.block test | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/mutations` |
| `tests/daemon_pg/handlers/workflow_loop/test_claim_next.py` | claim next | `go/pkg/mutations/claim_test.go` | none | pg_harness | covered | `pg_harness`; `cd go && go test ./pkg/mutations` |
| `tests/daemon_pg/handlers/workflow_loop/test_close_session.py` | close session | `go/pkg/mutations/claim_test.go` partial | explicit session.close test | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/mutations` |
| `tests/daemon_pg/handlers/workflow_loop/test_complete_job.py` | work complete | `go/pkg/mutations/lifecycle_test.go` partial | explicit work.complete test | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/mutations` |
| `tests/daemon_pg/handlers/workflow_loop/test_heartbeat_work.py` | work heartbeat | `go/pkg/mutations/claim_test.go` partial | explicit work.heartbeat test | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/mutations` |
| `tests/daemon_pg/handlers/workflow_loop/test_override_review_verdict.py` | review override | `go/pkg/mutations/review_test.go` partial | explicit review.override test | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/mutations` |
| `tests/daemon_pg/handlers/workflow_loop/test_publish_artifact.py` | artifact publish | `go/pkg/mutations/artifact_contract_test.go`, `artifact_contract_migration_test.go` partial | live publish path test with author-line and blob route | workflow_artifact_tests | needs_replacement | `workflow_artifact_tests`; `cd go && go test ./pkg/mutations` |
| `tests/daemon_pg/handlers/workflow_loop/test_record_verdict.py` | record verdict | `go/pkg/mutations/review_test.go` partial | explicit review.verdict test | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/mutations` |
| `tests/daemon_pg/handlers/workflow_loop/test_register_session.py` | register session | `go/pkg/mutations/claim_test.go` | none | pg_harness | covered | `pg_harness`; `cd go && go test ./pkg/mutations` |
| `tests/daemon_pg/handlers/workflow_loop/test_release_lease.py` | release lease | `go/pkg/mutations/claim_test.go` partial | explicit work.release test | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/mutations` |
| `tests/daemon_pg/handlers/workflow_loop/test_send_message.py` | send message | `go/pkg/mutations/work_send_message_test.go` | none | pg_harness | covered | `pg_harness`; `cd go && go test ./pkg/mutations` |
| `tests/daemon_pg/handlers/workflow_loop/test_submit_review.py` | submit review | `go/pkg/mutations/review_test.go` partial | explicit review.submit artifact test | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/mutations` |
| `tests/daemon_pg/test_append_only_role_grants.py` | DB role grants | `go/pkg/db/migrations_test.go` partial | explicit grants invariant | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/db` |
| `tests/daemon_pg/test_audit_chain_concurrency.py` | audit chain concurrency | `go/pkg/db/audit_race_test.go` | none | pg_harness | covered | `pg_harness`; `cd go && go test ./pkg/db` |
| `tests/daemon_pg/test_capability_denial_matrix.py` | capability denial matrix | `go/pkg/rpc/capabilities_test.go`, `pg_harness_test.go` partial | full denial matrix through server | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/rpc` |
| `tests/daemon_pg/test_migration_0006_event_chain.py` | event chain migration | `go/pkg/db/migrations_test.go` partial | explicit migration 0006 invariant | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/db` |
| `tests/daemon_pg/test_migration_0008_lane_evidence.py` | lane evidence migration | `go/pkg/db/migrations_test.go` partial | explicit migration 0008 invariant | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/db` |
| `tests/daemon_pg/test_migration_0010_artifact_blob_update.py` | artifact blob migration | `go/pkg/db/migrations_test.go`, `go/pkg/blob/*_test.go` partial | explicit migration 0010 invariant | corpus_archive_tests | needs_replacement | `corpus_archive_tests`; `cd go && go test ./pkg/db ./pkg/blob` |
| `tests/daemon_pg/test_pidfile_status.py` | pidfile status | `go/cmd/striatumd/pidfile_test.go` | none | pg_harness | covered | `pg_harness`; `cd go && go test ./cmd/striatumd` |
| `tests/daemon_pg/test_repo_registration.py` | repo registration | `go/pkg/repositories/service_test.go`, `pg_harness_test.go` | none | pg_harness | covered | `pg_harness`; `cd go && go test ./pkg/repositories` |
| `tests/daemon_pg/test_roles.py` | DB roles | `go/pkg/db/migrations_test.go` partial | explicit role grants test | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/db` |
| `tests/fixtures/multi_phase_workflow.json` | pytest fixture JSON | `go/pkg/workflowgenerate/generate_test.go` | move or retain as non-Python fixture | workflow_artifact_tests | covered | `workflow_artifact_tests`; `cd go && go test ./pkg/workflowgenerate` |
| `tests/test_archive_verify.py` | archive verify/replay | `go/pkg/reads/archive_test.go` partial | Go replay verifier, not only manifest shape | corpus_archive_tests | needs_replacement | `corpus_archive_tests`; `cd go && go test ./pkg/reads` |
| `tests/test_artifact_schemas.py` | artifact front-matter schemas | `go/pkg/mutations/artifact_contract_test.go`, `artifact_contract_migration_test.go` | none for covered schema classes; publisher-path tests still needed | workflow_artifact_tests | covered | `workflow_artifact_tests`; `cd go && go test ./pkg/mutations` |
| `tests/test_artifacts_web.py` | artifact web rendering | frontend tests partial | Go web route tests after service lands | web_tests | blocked | `web_tests`; Go web service route package required |
| `tests/test_chat_session.py` | web chat session | none | retire or port chat UI explicitly | web_tests | blocked | `web_tests`; product decision on chat route retention |
| `tests/test_chat_tools_daemon_boundary.py` | chat tools daemon boundary | none | retire or port chat tools via Go daemon RPC | web_tests | blocked | `web_tests`; product decision |
| `tests/test_claude_supervised_wrapper.py` | supervised wrapper | `go/pkg/supervisor/*_test.go` partial | Go CLI/process wrapper test | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/supervisor` |
| `tests/test_cli_daemon_rpc_route.py` | CLI daemon RPC route | `go/pkg/rpc/registry_contract_test.go`, `go/pkg/rpc/pg_harness_test.go` partial | Go CLI RPC client route tests | cli_tests | needs_replacement | `cli_tests`; `cd go && go test ./cmd/striatum ./pkg/rpc` |
| `tests/test_cli_mvp.py` | CLI MVP commands | `go/cmd/striatum/main_test.go` partial | finish Go CLI command families or retire | cli_tests | needs_replacement | `cli_tests`; `cd go && go test ./cmd/striatum` |
| `tests/test_corpus_enumerator.py` | corpus enumerator | no Go package | port or retire corpus file enumerator | corpus_archive_tests | needs_replacement | `corpus_archive_tests`; Go corpus package required |
| `tests/test_corpus_manifest.py` | corpus manifest | `go/pkg/reads/corpus_migration_test.go` partial | Go manifest writer/identity test | corpus_archive_tests | needs_replacement | `corpus_archive_tests`; `cd go && go test ./pkg/reads` |
| `tests/test_corpus_redaction.py` | corpus redaction | no Go redaction implementation | Go redaction tests | corpus_archive_tests | needs_replacement | `corpus_archive_tests`; Go corpus package required |
| `tests/test_corpus_verify.py` | corpus verify | no Go verifier | Go corpus verifier | corpus_archive_tests | needs_replacement | `corpus_archive_tests`; Go corpus package required |
| `tests/test_corpus_writer.py` | corpus writer | no Go writer | Go corpus writer tests | corpus_archive_tests | needs_replacement | `corpus_archive_tests`; Go corpus package required |
| `tests/test_cross_repo_crash_recovery_e2e.py` | cross-repo crash recovery | `go/pkg/crossrepo/*_test.go` partial | Go E2E or integration harness | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/crossrepo` |
| `tests/test_cross_repo_lifecycle.py` | cross-repo lifecycle | `go/pkg/crossrepo/lifecycle_test.go` | none | pg_harness | covered | `pg_harness`; `cd go && go test ./pkg/crossrepo` |
| `tests/test_cross_repo_lifecycle_e2e.py` | cross-repo lifecycle E2E | `go/pkg/crossrepo/lifecycle_test.go` partial | Go E2E harness | pg_harness | needs_replacement | `pg_harness`; cross-repo integration |
| `tests/test_cross_repo_pg_cancel.py` | cross-repo PG cancel | `go/pkg/crossrepo/lifecycle_test.go` | broaden PG route test | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/crossrepo` |
| `tests/test_cross_repo_prepare_e2e.py` | cross-repo prepare E2E | `go/pkg/crossrepo/prepare_test.go` partial | Go E2E harness | pg_harness | needs_replacement | `pg_harness`; cross-repo integration |
| `tests/test_daemon_go_audit.py` | Go daemon audit via pytest | `go/pkg/db/audit_race_test.go` | retire pytest shim | pg_harness | covered | `pg_harness`; `cd go && go test ./pkg/db` |
| `tests/test_daemon_go_mutations.py` | Go mutation smoke via pytest | `go/pkg/mutations/*_test.go` | retire pytest shim | pg_harness | covered | `pg_harness`; `cd go && go test ./pkg/mutations` |
| `tests/test_daemon_go_smoke.py` | Go daemon smoke via pytest | `go/cmd/striatumd/*_test.go` partial | shell daemon smoke if process behavior required | packaging_smoke | needs_replacement | `packaging_smoke`; `scripts/go_package_smoke.sh` |
| `tests/test_daemon_go_startup.py` | Go daemon startup via pytest | `go/cmd/striatumd/*_test.go` | process smoke still useful | packaging_smoke | needs_replacement | `packaging_smoke`; `scripts/go_package_smoke.sh` |
| `tests/test_daemon_go_supervisor.py` | Go supervisor via pytest | `go/pkg/supervisor/*_test.go` | retire pytest shim | pg_harness | covered | `pg_harness`; `cd go && go test ./pkg/supervisor` |
| `tests/test_daemon_method_tables_generation.py` | Python daemon method table generator | `go/pkg/rpc/registry_contract_test.go` | replace generator with Go or retire | cli_tests | needs_replacement | `cli_tests`; generator decision |
| `tests/test_daemon_pg.py` | daemon PG lifecycle | `go/pkg/db/*_test.go`, `go/pkg/admin/*_test.go` partial | live PG harness | pg_harness | blocked | `pg_harness`; reusable PG harness needed |
| `tests/test_daemon_pg_audit.py` | daemon PG audit | `go/pkg/db/audit_race_test.go` | live PG audit route | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/db` |
| `tests/test_daemon_pg_doctor.py` | daemon PG doctor | `go/pkg/reads/doctor_test.go` | live PG doctor route | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/reads` |
| `tests/test_daemon_pg_health.py` | daemon PG health | `go/pkg/admin/service_test.go` partial | Go health/doctor process route | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/admin ./pkg/reads` |
| `tests/test_daemon_pg_lifecycle.py` | daemon PG lifecycle | `go/pkg/mutations/lifecycle_test.go` partial | live PG lifecycle | pg_harness | blocked | `pg_harness`; reusable PG harness needed |
| `tests/test_daemon_pg_sweep.py` | daemon PG sweep | `go/pkg/recovery/sweep_test.go` | live sweep route | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/recovery` |
| `tests/test_daemon_rpc_registry.py` | RPC registry generation | `go/pkg/rpc/registry_contract_test.go` | none | cli_tests | covered | `cli_tests`; `cd go && go test ./pkg/rpc` |
| `tests/test_daemon_runtime.py` | daemon runtime | `go/pkg/admin/service_test.go` partial | process runtime smoke | packaging_smoke | needs_replacement | `packaging_smoke`; `scripts/go_package_smoke.sh` |
| `tests/test_dashboard_rfc0075.py` | dashboard RFC 0075 | `go/pkg/reads/dashboard_test.go` partial | explicit RFC 0075 assertions | web_tests | needs_replacement | `web_tests`; Go read/web route tests |
| `tests/test_day_zero.py` | day-zero setup | none | Go install/bootstrap smoke | packaging_smoke | needs_replacement | `packaging_smoke`; `scripts/go_fresh_clone_smoke.sh` |
| `tests/test_doc_links.py` | doc links | none | shell/docs link checker or retire from test migration | final_deletion_readiness | needs_replacement | `final_deletion_readiness`; docs guardrail |
| `tests/test_doctor_per_record_recipes.py` | doctor per-record recipes | `go/pkg/reads/doctor_test.go` partial | explicit recipe tests | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/reads` |
| `tests/test_dogfood_routes.py` | historical dogfood web routes | `go/pkg/reads/corpus_historical.go` partial | product decision: retain historical web or retire | web_tests | blocked | `web_tests`; route-retention decision |
| `tests/test_example_workflows.py` | example workflow validation | `go/pkg/workflowauthoring/workflow_test.go` partial | shell/Go fixture scan for examples | workflow_artifact_tests | needs_replacement | `workflow_artifact_tests`; `cd go && go test ./pkg/workflowauthoring` |
| `tests/test_go_rpc_registry_generation.py` | Python Go registry generator | `go/pkg/rpc/registry_contract_test.go` | replace generator with Go or shell | cli_tests | needs_replacement | `cli_tests`; generator decision |
| `tests/test_harness_friction_burndown.py` | harness burndown docs | none | retire or docs guardrail | final_deletion_readiness | retire | `final_deletion_readiness`; docs decision |
| `tests/test_issue_verify_prompts.py` | issue verify prompts | none | historical/operator-doc guardrail | final_deletion_readiness | historical_exception | `final_deletion_readiness`; historical policy |
| `tests/test_mcp_capability_scope_e2e.py` | MCP capability scope E2E | `go/pkg/mcp/*_test.go`, `go/pkg/rpc/*_test.go` partial | live MCP E2E | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/mcp ./pkg/rpc` |
| `tests/test_mcp_dogfood_e2e.py` | MCP dogfood E2E | no direct Go E2E | Go MCP E2E or retire historical dogfood route | pg_harness | blocked | `pg_harness`; product decision |
| `tests/test_mcp_fake_agent_loop_e2e.py` | fake agent loop E2E | `go/pkg/agentloop/*_test.go` partial | Go end-to-end loop test | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/agentloop` |
| `tests/test_mcp_mutation_capabilities.py` | MCP mutation capability checks | `go/pkg/mcp/capabilities_test.go` | none | pg_harness | covered | `pg_harness`; `cd go && go test ./pkg/mcp` |
| `tests/test_multi_repo_harness.py` | multi-repo harness | `go/pkg/crossrepo/*_test.go` partial | reusable Go multi-repo harness | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/crossrepo` |
| `tests/test_next_actions_v141_burndown.py` | next-actions burndown | none | retire if obsolete docs-only behavior | final_deletion_readiness | retire | `final_deletion_readiness`; docs decision |
| `tests/test_operator_current_brief.py` | operator brief freshness | none | shell/docs guardrail or retire from product tests | final_deletion_readiness | needs_replacement | `final_deletion_readiness`; docs guardrail |
| `tests/test_override_modal_context_validation.py` | web override modal context | frontend tests partial | browser/component test for retained override UI | web_tests | needs_replacement | `web_tests`; frontend test command |
| `tests/test_override_modal_payload.py` | web override modal payload | frontend tests partial | browser/component test for retained override UI | web_tests | needs_replacement | `web_tests`; frontend test command |
| `tests/test_per_repo_write_scope_e2e.py` | per-repo write scope E2E | `go/pkg/mutations/write_scope_guard_test.go` partial | Go E2E with registered repos | pg_harness | needs_replacement | `pg_harness`; live PG harness |
| `tests/test_plugin_install.py` | plugin installer | no Go plugin package | port or retire plugin installer | workflow_artifact_tests | blocked | `workflow_artifact_tests`; product decision |
| `tests/test_process_adapter.py` | process adapter | `go/pkg/supervisor/*_test.go` partial | Go process adapter equivalent or retire | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/supervisor` |
| `tests/test_recovery_daemon_watch.py` | recovery daemon watch | `go/pkg/recovery/*_test.go` partial | Go watcher/process test | pg_harness | needs_replacement | `pg_harness`; `cd go && go test ./pkg/recovery` |
| `tests/test_run_detail_recovery_panel.py` | recovery panel web data | `src/striatum/web/frontend/src/__tests__/recovery-panel.test.tsx` partial | Go route plus component test | web_tests | needs_replacement | `web_tests`; frontend + Go route tests |
| `tests/test_run_list.py` | run list web/API | Go reads list partial | Go web route test | web_tests | needs_replacement | `web_tests`; Go web service route package |
| `tests/test_scaffold_ddd_layout.py` | scaffold layout | no Go scaffold package | port or retire scaffold generator | workflow_artifact_tests | blocked | `workflow_artifact_tests`; product decision |
| `tests/test_service.py` | Python service root | no Go service package in tracked tree | Go local service route tests | web_tests | blocked | `web_tests`; Go web service required |
| `tests/test_service_request_io.py` | service request I/O | no Go service package in tracked tree | Go route/body parsing tests | web_tests | blocked | `web_tests`; Go web service required |
| `tests/test_service_request_security.py` | service request security | no Go service package in tracked tree | Go route security tests | web_tests | blocked | `web_tests`; Go web service required |
| `tests/test_service_runtime.py` | service runtime | no Go service package in tracked tree | Go local service runtime smoke | web_tests | blocked | `web_tests`; Go web service required |
| `tests/test_service_sse.py` | service SSE | no Go service package in tracked tree | Go SSE tests or retire SSE | web_tests | blocked | `web_tests`; Go web service/product decision |
| `tests/test_service_state.py` | service state | no Go service package in tracked tree | Go service state tests or retire | web_tests | blocked | `web_tests`; Go web service/product decision |
| `tests/test_skills_install.py` | skills installer | no Go skills package | port or retire skills installer | workflow_artifact_tests | blocked | `workflow_artifact_tests`; product decision |
| `tests/test_skills_install_wrappers.py` | skills wrapper install | no Go skills package | port or retire wrappers | workflow_artifact_tests | blocked | `workflow_artifact_tests`; product decision |
| `tests/test_static_assets.py` | web static assets | frontend build tests partial | Go embedded/static asset test | web_tests | needs_replacement | `web_tests`; Go web assets package |
| `tests/test_template_env.py` | Python template environment | no Go template env | Go templates or retire Jinja routes | web_tests | blocked | `web_tests`; Go web service/product decision |
| `tests/test_ui_packaging.py` | UI package data | frontend tests partial | Go archive/web asset packaging smoke | packaging_smoke | needs_replacement | `packaging_smoke`; `scripts/go_package_smoke.sh` |
| `tests/test_view_file.py` | web file viewer | `src/.../code-viewer.test.ts` partial | Go route + frontend test | web_tests | needs_replacement | `web_tests`; frontend + route tests |
| `tests/test_web_cutover_actions.py` | web mutation actions | no Go web service package | Go RPC-backed route tests | web_tests | blocked | `web_tests`; Go web service required |
| `tests/test_web_doctor.py` | web doctor route | `go/pkg/reads/doctor_test.go` partial | Go web route test | web_tests | needs_replacement | `web_tests`; Go web service route package |
| `tests/test_web_escalations.py` | web escalation routes | `go/pkg/reads/escalation_resolve_test.go` partial | Go web route test | web_tests | needs_replacement | `web_tests`; Go web service route package |
| `tests/test_web_job_detail.py` | web job detail | `go/pkg/reads/detail_test.go` partial | Go web route test | web_tests | needs_replacement | `web_tests`; Go web service route package |
| `tests/test_web_run_posture_verdicts_context.py` | web posture verdict context | Go reads partial | Go web route/component test | web_tests | needs_replacement | `web_tests`; Go web service route package |
| `tests/test_web_workflow_accepted_risks.py` | web accepted risks | `go/pkg/mutations/workflow_accepted_risk_test.go`, reads partial | Go web route test | web_tests | needs_replacement | `web_tests`; Go web service route package |
| `tests/test_workflow_adapter_constraints.py` | workflow adapter constraints | `go/pkg/workflowauthoring/workflow_test.go` partial | explicit adapter constraints | workflow_artifact_tests | needs_replacement | `workflow_artifact_tests`; `cd go && go test ./pkg/workflowauthoring` |
| `tests/test_workflow_cross_repo.py` | workflow cross-repo authoring | `go/pkg/crossrepo/prepare_test.go` partial | Go workflow/crossrepo authoring test | workflow_artifact_tests | needs_replacement | `workflow_artifact_tests`; `cd go && go test ./pkg/crossrepo ./pkg/workflowauthoring` |
| `tests/test_workflow_field_errors.py` | workflow field errors | `go/pkg/workflowauthoring/workflow_test.go`, `go/pkg/workflowgenerate/generate_test.go` | none for main authoring/generator errors | workflow_artifact_tests | covered | `workflow_artifact_tests`; `cd go && go test ./pkg/workflowauthoring ./pkg/workflowgenerate` |
| `tests/test_workflow_generation_web.py` | web workflow generation | `go/pkg/workflowgenerate/generate_test.go` partial | Go web route test for preview/generate | web_tests | needs_replacement | `web_tests`; Go web service route package |
| `tests/test_workflow_generator.py` | workflow generator | `go/pkg/workflowgenerate/generate_test.go` | none | workflow_artifact_tests | covered | `workflow_artifact_tests`; `cd go && go test ./pkg/workflowgenerate` |
| `tests/test_workflow_lint.py` | workflow lint | `go/pkg/workflowauthoring/workflow_test.go` | none | workflow_artifact_tests | covered | `workflow_artifact_tests`; `cd go && go test ./pkg/workflowauthoring` |
| `tests/test_workflow_phases.py` | workflow phases | `go/pkg/workflowgenerate/generate_test.go` | none | workflow_artifact_tests | covered | `workflow_artifact_tests`; `cd go && go test ./pkg/workflowgenerate` |
| `tests/test_workflow_upgrade.py` | workflow upgrade | `go/pkg/workflowgenerate/upgrade.go` no focused test | Go upgrade tests | workflow_artifact_tests | needs_replacement | `workflow_artifact_tests`; `cd go && go test ./pkg/workflowgenerate` |
