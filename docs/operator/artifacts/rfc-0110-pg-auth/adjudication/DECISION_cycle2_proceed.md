---
schema_version: striatum.decision.v1
decision_id: "dec_b95396ff8f414eae5dbcced5476313ad"
run_id: "run_8e14cb48342e929d30043d6be24f9101"
artifact_kind: decision
owner: human
outcome: accepted_with_follow_up
follow_up_required: true
title: "Proceed past exhausted revision budget; carry cycle-2 findings as binding spec constraints"
created_at: "2026-06-03T17:14:12Z"
---

# Proceed past exhausted revision budget; carry cycle-2 findings as binding spec constraints

Decision ID: `dec_b95396ff8f414eae5dbcced5476313ad`
Run ID: `run_8e14cb48342e929d30043d6be24f9101`
Outcome: `accepted_with_follow_up`

## Rationale

Cycle-2 converged on the right direction: v3 bytea audit-hash with verifier dispatch resolves the Go<->PL/pgSQL chain-hash parity risk, and a RAM-only daemon-authority gate replaces spoofable attribution GUCs. The adjudicator's att2 needs_revision is well-founded but the workflow's single-revision budget (max_iterations:1) is exhausted. Operator decision: accept the converged design and carry the remaining 1 critical + 11 high findings forward as BINDING implementation constraints in the published spec rather than cancel the run.

## Follow-Up

IX2-001 (critical): the L0/L3 daemon_auth secret and rpc/principal attribution must NOT be carried via set_config/SET LOCAL under pgx simple protocol (params interpolate into query text, observable by another striatumd_rw session via pg_stat_activity) -> use extended-protocol bound parameters or a non-text carrier. Plus the high findings to pin before implementation: enforce the authority/attribution prelude inside an unavoidable authorized-transaction wrapper (not a prose 'run after BeginTx' rule); audit append must fail-closed not fail-open; owner-only DDL delivery for SECURITY DEFINER fns; pgtest privilege fidelity (42501 negative-path); daemon_auth freshness lifecycle + bounded reconnect + role-scoped rotator detection + deploy parity; narrow read-scope overclaim; pin the sole-durable-write-path phase scope; soften premature #87 closure language to 'mitigated, pending lane-OS-user default'.
