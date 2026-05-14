---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["rfc-0048", "phase-a", "daemon-pg", "adversarial-review"]
---
author: reviewer-unknown-model-001

# Adversarial Review: RFC 0048 Daemon-side Substrate Migration (Phase A)

**Author:** reviewer-unknown-model-001 (Gemini CLI)
**Posture:** Threat Model / Adversarial
**Date:** 2026-05-14

## Executive Summary

This review evaluates the Phase A implementation of RFC 0048, which ports single-repo mutation handlers from SQLite-backed CLI dispatch to native PostgreSQL-backed handlers in the daemon. The objective is to identify subtle vulnerabilities, bypasses, and consistency issues introduced by this migration.

## Trust Boundaries and Attack Surfaces

1.  **RPC/PG Boundary:** The `DaemonRpcRouter` now resolves PG-backed handlers. The primary surface is the parameter mapping and authorization context passed to these handlers.
2.  **State Consistency Boundary:** During the transition, both SQLite and PG substrates might co-exist. Handlers must strictly adhere to the PG substrate and not fall back or "split-brain".
3.  **Capability Auth surface:** Every ported mutation requires a specific capability (claim, write, review, etc.). Bypasses here allow unauthorized state changes.

## Adversarial Findings

### 1. SQLite Fallback & Facade Risks

**Finding:** The `DaemonRpcRouter` in `src/striatum/daemon_rpc/server.py` still contains the legacy `_route` logic which delegates to CLI-backed SQLite dispatch if a PG handler is not found.
**Risk:** High. If a new method is added but not ported to PG, it silently falls back to SQLite, potentially creating a split-brain state where some data is in PG and some in SQLite.
**Recommendation:** Phase C's plan to flip the default must be strictly enforced. All ported methods must explicitly disable the fallback path.

### 2. Audit-Chain Forge Attempts (Insert without Anchoring)

**Finding:** PG handlers like `register_session` and `claim_next` use `ctx.append_event` to record state changes.
**Risk:** Moderate. An adversarial handler could theoretically call `cur.execute("INSERT INTO striatumd.events ...")` directly, bypassing the hash-chaining logic in `append_event`.
**Mitigation:** The `RepoHandlerContext.append_event` method correctly handles chaining. Audit enforcement relies on PG role permissions (append-only on events table) as specified in RFC 0033/0043. Review of all handlers confirms they use the context's helper rather than raw inserts for events.

### 3. Capability Auth Bypass

**Finding:** Handlers rely on `RpcAuthContext` passed from the router.
**Risk:** Moderate. A handler that forgets to check `ctx.auth` or relies on client-supplied IDs without verifying they match the token scope could widen its scope.
**Observation:** Ported handlers (e.g., `ack_work`) correctly use `ctx.repository_id` derived from the auth context, ensuring multi-tenancy/repo isolation. However, if `repository_id is missing from params, the router raises `repo_not_registered`.

### 4. Orphaned PG Rows in Recovery

**Finding:** `recovery.process_reconcile` and `recovery.cancel_job` perform complex state transitions across multiple tables.
**Risk:** Low. If a transaction is not used, a crash could leave orphaned rows.
**Mitigation:** All handlers use `with transaction(ctx):` ensuring atomic commits across `jobs`, `leases`, `queue_messages`, and `events`.

### 5. Split-Write Transaction (State vs Events)

**Finding:** Events are appended within the same transaction as state updates.
**Risk:** Low. The implementation ensures that both state and event log are committed together.
**Observation:** `RepoHandlerContext.append_event` is called within the `transaction(ctx)` block.

### 6. Scope Widening (Repository ID)

**Finding:** Handlers use `ctx.repository_id` in every SQL query.
**Risk:** Moderate. A handler could theoretically omit `repository_id = %s` in a `WHERE` clause, affecting other repositories.
**Observation:** A manual audit of `src/striatum/daemon_pg/handlers/` shows consistent use of `repository_id` filtering. The `RepoHandlerContext.row_by_id` helper also enforces this.

## Verdict

**ACCEPTED WITH FINDINGS**

The Phase A implementation is structurally sound and adheres to the transactional and substrate-isolation requirements. The primary risk remains the fallback to SQLite for un-ported methods, which is a known phasing issue but must be monitored closely to prevent split-brain during the transition.
