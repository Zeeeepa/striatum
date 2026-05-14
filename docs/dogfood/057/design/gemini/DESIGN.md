author: designer-unknown-model-001
# Design: RFC 0048 Phase A Handler Port

**Author:** Gemini CLI (designer-gemini-1)
**Status:** accepted
**Date:** 2026-05-14

## Overview

RFC 0048 Phase A addresses the "substrate facade" where the daemon's RPC server delegates single-repo operations to the SQLite-backed CLI dispatch. This design specifies the porting of the first batch of mutation handlers to native PostgreSQL implementations using the `striatumd` schema introduced in RFC 0043.

## Goals

- Implement native Postgres-backed handlers for the first set of mutation verbs.
- Update `DaemonRpcRouter` to route these verbs to the new handlers instead of `striatum.api.invoke`.
- Ensure parity in behavior and state between the new PG handlers and the legacy SQLite handlers.

## Architecture

### 1. Handler Location

New handlers will be located in `src/striatum/daemon_pg/handlers/`. Each logical group of mutations will have its own module:

- `src/striatum/daemon_pg/handlers/mutations.py`: `register_session`, `claim_next`, `ack_work`, `complete_job`, `release_lease`, `block_job`, `record_verdict`, `submit_review`, `override_review_verdict`.
- `src/striatum/daemon_pg/handlers/recovery.py`: `stale_leases`, `requeue_stale`, `cancel_job`, `process_reconcile`, `resume_blocker`, `auto_publish_stale_artifacts`.
- `src/striatum/daemon_pg/handlers/evidence.py`: `evidence_export`.

### 2. Router Integration

The `DaemonRpcRouter._route` method in `src/striatum/daemon_rpc/server.py` will be updated to check for a native handler.

```python
# In src/striatum/daemon_rpc/server.py

NATIVE_HANDLERS: dict[str, Callable[[Any, RpcEnvelope, Path], dict[str, Any]]] = {
    "session.register": handle_register_session,
    "work.claim_next": handle_claim_next,
    # ...
}

def _route(self, envelope: RpcEnvelope, *, repo_root: Path) -> dict[str, Any]:
    # ... existing specialized routes (describe, dashboard, cross_repo, etc.)
    
    native_handler = NATIVE_HANDLERS.get(envelope.method)
    if native_handler:
        return native_handler(self.pg_conn, envelope, repo_root)
        
    # Fallback to legacy SQLite delegation
    # ...
```

### 3. Handler Signature

Native handlers will follow a consistent signature to allow the router to pass the connection, the full envelope, and the resolved repository root.

```python
def handle_verb(pg_conn: Any, envelope: RpcEnvelope, repo_root: Path) -> dict[str, Any]:
    # Implementation using psycopg and repository_id from envelope.params
```

## Implementation Strategy (Phase A)

We will port the handlers in the order specified by RFC 0048. All PG operations must use `SERIALIZABLE` isolation or appropriate locking to ensure consistency.

### Batch 1: Core Session & Work Lifecycle

1. **`session.register` (`register_session`)**:
   - Write to `striatumd.sessions`.
   - Validate `reviewer_context_policy: fresh` against active author sessions in PG.
   - Record `session.registered` event in `striatumd.events`.
2. **`work.claim_next` (`claim_next`)**:
   - Atomic claim using `SELECT ... FOR UPDATE SKIP LOCKED` on `striatumd.queue_messages`.
   - Create `striatumd.leases` and `striatumd.work_packets`.
   - Update `striatumd.jobs` state to `claimed`.
3. **`work.ack` (`ack_work`)**:
   - Update `striatumd.queue_messages` state to `acked`.
   - Update `striatumd.jobs` state to `running`.
4. **`work.complete` (`complete_job`)**:
   - Update `striatumd.jobs` state to `completed`.
   - Release `striatumd.leases`.
   - Update `striatumd.queue_messages` state to `completed`.

### Batch 2: Reviews & Verdicts

1. **`review.submit` (`submit_review`)**
2. **`review.verdict` (`record_verdict`)**
3. **`review.override` (`override_review_verdict`)**

### Batch 3: Recovery & Evidence

1. **`recovery.*`**
2. **`evidence.export`**

## Testing & Validation

### Parity Tests
We will use `tests/test_daemon_pg.py` and new specific handler tests.
Validation criteria:
- `striatumd.events` log matches legacy event log format exactly.
- Audit chain hashes remain consistent (verified by `compute_repo_local_reanchor` logic).

### Performance
We will monitor for `SerializationFailure` (SQLState 40001) and ensure the RPC server or handlers implement a retry loop.

## Risks & Mitigations

- **Event Parity:** Missing an event or mismatching a payload will break downstream projectors. We will cross-reference `src/striatum/cli/mutations.py` for every event emission.
- **Audit Chain:** The `striatumd.events` table has append-only triggers. We must ensure the native handlers respect the audit chain logic.
- **Substrate Divergence:** During the transition, a repo might have state in both SQLite and PG if not migrated. The router already enforces exit-code-12 (unmigrated) before reaching `_route`, ensuring we only run native handlers against migrated PG repos.
