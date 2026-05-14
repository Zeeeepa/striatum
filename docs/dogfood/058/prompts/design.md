# Design Prompt: RFC 0048 V1.5 fix-up

Produce DESIGN.md at the path your work packet specifies (`docs/dogfood/058/design/<lane>/`).

## Required reading (in order)

1. `docs/rfcs/0048-daemon-side-substrate-migration.md` — especially the new "V1 Phase A landing summary" + "V1.5 follow-up" sections.
2. `docs/dogfood/057/review/build/codex/REVIEW.md` — the verbatim threat-model findings F1-F4 you must address.
3. `docs/dogfood/057/review/build/claude/REVIEW.md` — the verbatim ergonomics_dx findings HIGH#1/#2 + MEDIUM#3-6 you must address.
4. `docs/dogfood/057/DESIGN_SYNTHESIS.md` — the synthesis you are amending.
5. `docs/dogfood/057/build/track_a/HANDOFF.md` + `track_b/HANDOFF.md` — current state of the V1 Phase A code.
6. Current source files you will touch: `src/striatum/daemon.py` (run_daemon_foreground), `src/striatum/daemon_rpc/server.py` (DaemonRpcRouter._route), `src/striatum/daemon_pg/handlers/`, `src/striatum/daemon_pg/sql/*.sql`, `tests/daemon_pg/handlers/`, `docs/POSTGRES_TRANSITION.md`.

## Design the two implementer tracks

### Track A (codex) — router + transport + handler internals

1. **Fail-closed routing (codex F1)** — `DaemonRpcRouter._route` must, for any method in the PG handler registry, treat handler exceptions / capability denials / parameter validation failures as terminal RPC errors. NO fall-through to `CLI_ROUTES` / `striatum.api.invoke` / `striatum.db.connect` / SQLite-backed dispatch. Design the registry method that decides PG-backed-ness and the error-envelope shape.

2. **Audit-chain SERIALIZABLE / row-lock (codex F3)** — every PG write handler in `workflow_loop/` and `recovery_evidence/` must append its event + mutate its state inside a single `SERIALIZABLE` transaction OR with an explicit row-lock on a chain-head table. Specify which pattern, why, and how a concurrent test verifies an unbroken chain.

3. **Unix-socket accept loop in `run_daemon_foreground`** — the daemon currently binds the socket and runs sweeps but never `accept()`s. This is the gap that forced dogfood-057 into legacy SQLite mode. Design the accept loop (asyncio? thread-per-connection? select?), the framing (already in `daemon_rpc.framing`), the handshake (already in `daemon_rpc.handshake`), and how it routes through `DaemonRpcRouter`. End-to-end goal: `striatum status` from the CLI reaches `DaemonRpcRouter._route` via the Unix socket.

4. **Append-only role enforcement (codex F4)** — SQL grants that ensure `striatumd_rw` cannot UPDATE/DELETE `striatumd.events` or `striatumd.artifacts`. Where does this go (existing 0001 migration or new 0007)? How do tests verify the privilege grant matches doctrine?

### Track B (claude) — tests + schema + docs + UX

1. **Byte-equivalence parity rig (claude HIGH #1)** — the conftest `Seed`/`pg_ctx`/`sqlite_conn` rig under `tests/daemon_pg/handlers/recovery_evidence/conftest.py` is advertised but unused. Wire it into all 16 handler test files. Add a `parity_seed` fixture. Add a per-key state diff helper. Remove the `RFC0048_PARITY` env-gate. Tests must fail loudly with a diff on regression.

2. **Capability-denial test matrix (codex F2)** — for every PG write handler, tests for: missing token, revoked, expired, wrong capability, wrong repository scope, replayed request_id. Per-test assertion: no workflow-table mutation, no audit-row append on the allow path, and a denied audit row with the documented reason. Specify the test scaffolding helper.

3. **Schema migration 0006 (claude #4)** — `src/striatum/daemon_pg/sql/0006_event_chain_columns.sql`:
   - `ALTER TABLE striatumd.events ADD COLUMN previous_hash bytea NOT NULL DEFAULT ...; ADD COLUMN row_hash bytea NOT NULL DEFAULT ...`
   - `CREATE TABLE striatumd.repo_event_chain_heads (...)`
   - Body re-anchors existing rows by reading `payload_json._event_chain.previous_hash` / `row_hash`.
   - Idempotent re-run via `schema_meta` version bump.

4. **Dead code cleanup (claude HIGH #2)** — `complete_inline`, `ack_inline`, `recovery.resume --complete`, `recovery.auto` live mode are referenced but never defined. Decide per-symbol: define + wire (with tests) OR delete entirely (with removed callers). Justify per item.

5. **`striatum daemon doctor --explain` (claude #5)** — new flag on `daemon doctor` that prints a per-method table: method name | PG-backed yes/no | SQLite fallback active yes/no. Specify the registry query.

6. **`docs/POSTGRES_TRANSITION.md` runbook (operator friction)** — new section "Provision the daemon-required role" with copy-pasteable SQL for `striatumd_rw` creation + grants + revokes. Cite the doctor refusal that surfaces the missing role.

## Out of scope

- RFC 0048 Phase B (Go core parity) and Phase C (SQLite removal flip).
- Bundled Postgres distribution, hosted/cloud daemon.
- Multi-tenancy (`tenant_id` enforcement).
- README / TODO / CHANGELOG / SPEC updates (operator-only after the dogfood lands).

## Byline discipline (hard constraint)

The work packet supplies an exact `author: <slug>` line. Copy it verbatim. Plain markdown line, lowercase `author:`. No decoration. Slug shape: `<role>-unknown-model-<NN>`.

One-shot supervised invocation. Write the artifact directly. If `striatum ack` is denied, write the artifact and exit normally; the operator publishes on your behalf.
