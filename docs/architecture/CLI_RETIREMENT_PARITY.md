# CLI Retirement Parity Ledger
author: implementer-codex-gpt-5-001

Status: checked architecture artifact
Date: 2026-05-23
Scope inputs: `contracts/daemon_methods.json`, `docs/rfcs/0050-go-daemon-http-sse-mcp.md`, `docs/rfcs/0075-tmux-observable-mcp-agent-sessions.md`, `docs/operator/plans/rfc-0050-cli-retirement-cutover.md`, and `docs/operator/plans/rfc-0075-tmux-observable-mcp-agent-sessions.md`.

This ledger classifies the remaining non-read CLI routes before any RFC 0050
CLI retirement work. It does not hide, deprecate, or delete commands. The
retirement gate stays blocked unless a replacement is visible through daemon
MCP and, where the operation is human/operator-facing, the local web UI has
active tests for the replacement.

Read-only CLI routes such as `status`, `why`, `doctor`, `dashboard`, `list *`,
`run summary`, `run graph`, `supervise status`, `supervise list`, and
`cross-repo list/describe/why` are diagnostics or observation surfaces, not
live workflow-control mutations. Their daemon authority remains tracked in
`docs/architecture/COMMAND_AUTHORITY_MATRIX.md`.

Local workflow-file authoring commands such as `workflow validate`, `workflow
plan`, `workflow graph`, `workflow init`, `workflow generate`, and `workflow
upgrade` are not live workflow-control commands. Production MCP intentionally
hides local file-authoring write tools until a separate product decision makes
them supported production MCP tools.

## Status Values

- `bootstrap`: CLI survives as bootstrap/admin rather than workflow control.
- `replaced_by_mcp`: the live agent/operator action has an MCP replacement,
  but no operator UI replacement is required for that lane-owned action.
- `replaced_by_mcp_ui`: daemon MCP and local UI both have replacement paths.
- `temporary_compatibility`: CLI remains the operator-facing fallback because
  MCP, UI, active tests, or docs/skill cutover are incomplete.
- `mcp_exact`: an active MCP test exercises the specific method or its
  accepted replacement.
- `mcp_registry`: generic MCP tool visibility/dispatch covers the method, but
  the method still lacks a specific MCP workflow test.
- `ui_exact`: an active UI or service-route test exercises the replacement.
- `ui_missing`: no local UI replacement is identified.
- `ui_not_product`: lane-owned MCP action; no human UI action is currently
  expected.
- `ui_route_unchecked`: UI route exists but lacks an active non-skipped test.

## Checked CLI Control Ledger

| CLI command | RPC method | Capability | Classification | MCP parity | UI parity | Checked evidence | Retirement gate |
|---|---|---:|---|---|---|---|---|
| `git commit-apply` | `git.commit_apply` | apply | temporary_compatibility | mcp_exact | ui_missing | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc | blocked_ui_gap |
| `repo add` | `repo.add` | admin | bootstrap | n/a | n/a | none | keep_bootstrap_cli |
| `repo remove` | `repo.remove` | admin | bootstrap | n/a | n/a | none | keep_bootstrap_cli |
| `workflow accept-risk` | `workflow.accept_risk` | admin | replaced_by_mcp_ui | mcp_exact | ui_exact | go/pkg/mcp/http_test.go::TestHTTPHandlerToolsCallAcceptedRiskRequiresAdmin, tests/test_web_workflow_accepted_risks.py::test_handle_workflow_accept_risk_routes_daemon_mutation_without_file_write | blocked_retirement_test_gap |
| `run prepare` | `run.prepare` | admin | replaced_by_mcp_ui | mcp_exact | ui_exact | tests/test_mcp_fake_agent_loop_e2e.py::test_fake_mcp_agent_completes_work_packet_loop, tests/test_service.py::test_workflow_run_now_posts_daemon_lifecycle_without_sqlite | blocked_retirement_test_gap |
| `run start` | `run.start` | admin | replaced_by_mcp_ui | mcp_exact | ui_exact | tests/test_mcp_fake_agent_loop_e2e.py::test_fake_mcp_agent_completes_work_packet_loop, tests/test_service.py::test_workflow_run_now_posts_daemon_lifecycle_without_sqlite | blocked_retirement_test_gap |
| `run pause` | `run.pause` | admin | replaced_by_mcp_ui | mcp_exact | ui_exact | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc, tests/test_service.py::test_web_run_pause_resume_post_daemon_rpc_without_sqlite | blocked_retirement_test_gap |
| `run resume` | `run.resume` | admin | replaced_by_mcp_ui | mcp_exact | ui_exact | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc, tests/test_service.py::test_web_run_pause_resume_post_daemon_rpc_without_sqlite | blocked_retirement_test_gap |
| `run cancel` | `run.cancel` | admin | replaced_by_mcp_ui | mcp_exact | ui_exact | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc, tests/test_service.py::test_web_run_cancel_posts_daemon_rpc_without_sqlite | blocked_retirement_test_gap |
| `run retry-job` | `run.retry_job` | admin | replaced_by_mcp_ui | mcp_exact | ui_exact | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc, tests/test_service.py::test_web_job_actions_post_daemon_rpc_without_sqlite | blocked_retirement_test_gap |
| `register-session` | `session.register` | claim | replaced_by_mcp | mcp_exact | ui_not_product | tests/test_mcp_fake_agent_loop_e2e.py::test_fake_mcp_agent_completes_work_packet_loop | blocked_docs_skill_cutover |
| `session close` | `session.close` | claim | replaced_by_mcp | mcp_exact | ui_not_product | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc | blocked_docs_skill_cutover |
| `claim-next` | `work.claim_next` | claim | replaced_by_mcp | mcp_exact | ui_not_product | tests/test_mcp_fake_agent_loop_e2e.py::test_fake_mcp_agent_completes_work_packet_loop | blocked_docs_skill_cutover |
| `ack` | `work.ack` | claim | replaced_by_mcp | mcp_exact | ui_not_product | tests/test_mcp_fake_agent_loop_e2e.py::test_fake_mcp_agent_completes_work_packet_loop | blocked_docs_skill_cutover |
| `heartbeat` | `work.heartbeat` | claim | replaced_by_mcp | mcp_exact | ui_not_product | tests/test_mcp_fake_agent_loop_e2e.py::test_fake_mcp_agent_completes_work_packet_loop | blocked_docs_skill_cutover |
| `release` | `work.release` | claim | replaced_by_mcp | mcp_exact | ui_not_product | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc | blocked_docs_skill_cutover |
| `send` | `work.send_message` | write | temporary_compatibility | mcp_exact | ui_not_product | tests/test_mcp_capability_scope_e2e.py::test_repo_scoped_write_token_authorizes_repo_a_write_and_audits_allowed | blocked_docs_skill_cutover |
| `block` | `work.block` | write | replaced_by_mcp | mcp_exact | ui_not_product | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc | blocked_docs_skill_cutover |
| `complete` | `work.complete` | write | replaced_by_mcp | mcp_exact | ui_not_product | tests/test_mcp_fake_agent_loop_e2e.py::test_fake_mcp_agent_completes_work_packet_loop | blocked_docs_skill_cutover |
| `publish-artifact` | `artifact.publish` | write | replaced_by_mcp | mcp_exact | ui_not_product | tests/test_mcp_fake_agent_loop_e2e.py::test_fake_mcp_agent_completes_work_packet_loop | blocked_docs_skill_cutover |
| `verdict` | `review.verdict` | review | temporary_compatibility | mcp_exact | ui_exact | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc, tests/test_service.py::test_v1_invoke_daemon_mapped_mutation_uses_daemon_rpc_not_api_invoke | blocked_docs_skill_cutover |
| `submit-review` | `review.submit` | review | replaced_by_mcp | mcp_exact | ui_not_product | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc | blocked_docs_skill_cutover |
| `override-verdict` | `review.override` | admin | temporary_compatibility | mcp_exact | ui_exact | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc, tests/test_override_modal_payload.py::test_override_modal_posts_literal_invoke_argv, tests/test_service.py::test_v1_invoke_override_verdict_web_context_routes_daemon_rpc | blocked_docs_skill_cutover |
| `recovery stale-leases` | `recovery.stale_leases` | recovery | temporary_compatibility | mcp_exact | ui_missing | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc | blocked_ui_gap |
| `recovery requeue-stale` | `recovery.requeue_stale` | recovery | temporary_compatibility | mcp_exact | ui_route_unchecked | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc | blocked_active_ui_test_gap |
| `recovery cancel-job` | `recovery.cancel_job` | recovery | replaced_by_mcp_ui | mcp_exact | ui_exact | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc, tests/test_service.py::test_web_job_actions_post_daemon_rpc_without_sqlite | blocked_retirement_test_gap |
| `recovery process-reconcile` | `recovery.process_reconcile` | recovery | temporary_compatibility | mcp_exact | ui_missing | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc | blocked_ui_gap |
| `recovery resume` | `recovery.resume` | recovery | temporary_compatibility | mcp_exact | ui_missing | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc | blocked_ui_gap |
| `recovery auto` | `recovery.sweep` | recovery | temporary_compatibility | mcp_exact | ui_missing | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc, tests/test_recovery_daemon_watch.py::test_daemon_watch_runs_sweeps_through_daemon_rpc | blocked_ui_gap |
| `recovery auto-publish` | `recovery.auto_publish_stale_artifacts` | recovery | temporary_compatibility | mcp_exact | ui_route_unchecked | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc | blocked_active_ui_test_gap |
| `recovery auto-finalize` | `recovery.auto_finalize` | recovery | temporary_compatibility | mcp_exact | ui_missing | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc | blocked_ui_gap |
| `decision record` | `decision.record` | admin | temporary_compatibility | mcp_exact | ui_route_unchecked | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc | blocked_active_ui_test_gap |
| `checkpoint resolve` | `checkpoint.resolve` | admin | temporary_compatibility | mcp_exact | ui_route_unchecked | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc | blocked_active_ui_test_gap |
| `escalation resolve` | `escalation.resolve` | admin | replaced_by_mcp_ui | mcp_exact | ui_exact | tests/test_mcp_mutation_capabilities.py::test_escalation_resolve_mcp_call_refuses_read_only_or_wrong_repository_tokens, tests/test_web_escalations.py::test_escalation_resolve_posts_daemon_rpc_without_sqlite | blocked_retirement_test_gap |
| `branch confirm` | `branch.confirm` | admin | replaced_by_mcp_ui | mcp_exact | ui_exact | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc, tests/test_service.py::test_web_branch_confirm_posts_daemon_rpc_then_run_start_without_sqlite | blocked_retirement_test_gap |
| `worktree create` | `worktree.create` | write | temporary_compatibility | mcp_exact | ui_missing | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc | blocked_ui_gap |
| `worktree release` | `worktree.release` | write | temporary_compatibility | mcp_exact | ui_missing | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc | blocked_ui_gap |
| `supervise start` | `supervise.start` | claim | temporary_compatibility | mcp_exact | ui_missing | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc | blocked_rfc0075_ui_gap |
| `supervise send` | `supervise.send` | claim | temporary_compatibility | mcp_exact | ui_missing | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc | blocked_rfc0075_ui_gap |
| `supervise stop` | `supervise.stop` | claim | temporary_compatibility | mcp_exact | ui_missing | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc | blocked_rfc0075_ui_gap |
| `cross-repo cancel` | `cross_repo.cancel` | recovery | temporary_compatibility | mcp_exact | ui_missing | tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc | blocked_ui_gap |

## Immediate Guardrail

`tests/architecture/test_cli_retirement_parity.py` checks that this ledger
covers every non-read route in `contracts/daemon_methods.json`, that method
and capability cells match the daemon contract, that cited active test files
exist, and that no row is marked retireable. A future CLI hiding/deletion
change must first change this ledger, add the missing MCP/UI parity tests, and
then update the guardrail intentionally.
