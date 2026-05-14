# Implement Track A — router + transport + handler internals (codex)

Produce `docs/dogfood/058/build/track_a/HANDOFF.md`. Front matter:

```
---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
inputs: ["docs/dogfood/058/DESIGN_SYNTHESIS.md", "docs/dogfood/058/review/design/REVIEW.md"]
---
```

Byline AFTER the front-matter block. Plain markdown line, lowercase `author:`, no decoration. Slug shape: `implementer-unknown-model-<NN>`.

## Scope (4 work items)

### 1. Fail-closed routing (codex F1)

In `src/striatum/daemon_rpc/server.py::DaemonRpcRouter._route`:
- When the method is in the PG handler registry, ALL handler-side exceptions/capability-denials/parameter-validation-failures return an RPC error envelope.
- **NO** fall-through to `CLI_ROUTES` / `striatum.api.invoke` / `striatum.db.connect` / SQLite-backed dispatch for these methods.
- Add a registry method like `is_pg_backed(method_name)` and use it as the fork.

Test: monkeypatch a PG handler to raise; assert no SQLite read/write occurs (use `caplog` + a sentinel file path that any SQLite call would hit).

### 2. Audit-chain SERIALIZABLE / row-lock (codex F3)

In every `src/striatum/daemon_pg/handlers/workflow_loop/<method>.py`:
- Wrap event-append + state-mutation in a single `SERIALIZABLE` transaction (per synthesis pick).
- OR explicit row-lock on `striatumd.repo_event_chain_heads(repository_id)`.

Concurrent test under `tests/daemon_pg/handlers/test_audit_chain_concurrency.py`: drive 8 overlapping allowed + denied requests across claim/publish-artifact/verdict/complete/recovery; verify single contiguous audit chain by walking `previous_hash` pointers; verify no orphan workflow mutations.

### 3. Unix-socket accept loop in `run_daemon_foreground` (transport gap)

In `src/striatum/daemon.py::run_daemon_foreground`:
- Currently binds + `listen(1)` but never `accept()`. This is the gap.
- Add an accept loop (asyncio / threading per synthesis pick) that:
  - Accepts connections on the Unix socket.
  - Reads RPC envelopes via `daemon_rpc.framing`.
  - Performs handshake via `daemon_rpc.handshake`.
  - Routes through `DaemonRpcRouter._route`.
  - Writes responses.
- Graceful shutdown on SIGTERM (clean accept-loop exit before `sock.close()`).

End-to-end test in `tests/daemon_rpc/test_accept_loop.py`:
- Start a real daemon subprocess.
- Use the `daemon_rpc.client` to issue a real `striatum status`-equivalent call.
- Assert the response comes back with the expected shape.
- Tear down cleanly (no orphan PIDs, no leaked sockets).

### 4. Append-only role enforcement (codex F4)

SQL: new `src/striatum/daemon_pg/sql/0007_append_only_grants.sql` (or amendment per synthesis) that grants `striatumd_rw` only the privileges allowed:
- `GRANT SELECT, INSERT ON striatumd.events TO striatumd_rw;`
- `GRANT SELECT, INSERT ON striatumd.artifacts TO striatumd_rw;`
- `REVOKE UPDATE, DELETE ON striatumd.events FROM striatumd_rw;`
- `REVOKE UPDATE, DELETE ON striatumd.artifacts FROM striatumd_rw;`
- (Plus any other append-only tables identified in synthesis.)

Test in `tests/daemon_pg/test_role_grants.py`:
- Connect as `striatumd_rw`.
- Attempt `UPDATE striatumd.events SET ...` — assert `permission denied`.
- Attempt `DELETE FROM striatumd.events ...` — assert `permission denied`.

## Sub-agents (use them aggressively)

- **fail-closed-routing**: change to `_route` + registry + regression test.
- **chain-locking**: per-handler SERIALIZABLE/row-lock + concurrent test.
- **accept-loop**: `daemon.py` accept loop + framing wiring + e2e test.
- **role-enforcement**: SQL migration + privilege test.

## Forbidden writes

Do NOT touch `src/striatum/daemon_pg/handlers/recovery_evidence/`, `src/striatum/daemon_pg/sql/0006_*` (Track B's schema migration), `docs/POSTGRES_TRANSITION.md` (Track B's runbook), or `tests/daemon_pg/handlers/recovery_evidence/`.

## HANDOFF.md content

For each work item:
- File paths changed + function names.
- Test paths + test command.
- Confirmed concurrency-safety / fail-closedness via the test outputs.
- One-line summary of behavior delta vs V1 (preferably none beyond the fix).

Plus a top-level summary table cross-referencing codex F1/F3/F4 + accept-loop closure.

## Byline discipline

Plain markdown line AFTER the front-matter block. Lowercase `author:`. No decoration. Slug shape: `implementer-unknown-model-<NN>`.

One-shot supervised invocation. If `striatum ack` is denied, write the artifact and exit normally.
