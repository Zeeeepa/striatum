# Implement Track B — tests + schema + docs + UX (claude)

Produce `docs/dogfood/058/build/track_b/HANDOFF.md`. Front matter:

```
---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
inputs: ["docs/dogfood/058/DESIGN_SYNTHESIS.md", "docs/dogfood/058/review/design/REVIEW.md"]
---
```

Byline AFTER the front-matter block. Plain markdown line, lowercase `author:`, no decoration. Slug shape: `implementer-unknown-model-<NN>`.

## Scope (6 work items)

### 1. Byte-equivalence parity rig (claude HIGH #1)

In `tests/daemon_pg/handlers/conftest.py` (or location per synthesis):
- Define `parity_seed` fixture that materializes both PG and SQLite state from the same `Seed` dataclass.
- Define a `assert_state_parity(pg_ctx, sqlite_conn)` helper that walks every row in every striatumd.* table and asserts per-key equality.
- On failure, print: `key=<path>: pg=<a> vs sqlite=<b>` for every mismatched key.

Wire the rig into all 16 handler test files (9 workflow_loop + 7 recovery_evidence). Remove `@pytest.mark.skipif(not os.environ.get('RFC0048_PARITY'))` everywhere; parity runs by default.

### 2. Capability-denial test matrix (codex F2)

For every PG write handler (16 methods), tests for the 6 cases:
- missing token
- revoked token
- expired token
- wrong capability (e.g., `write` not granted)
- wrong repository scope (token scoped to repo B; handler called against repo A)
- replayed `request_id`

Per-test assertion:
- No row mutated in any striatumd.* table.
- No audit-row append on the allow path.
- One denied audit row appended with the documented reason.

Scaffold helper in `tests/daemon_pg/handlers/_capability_denial.py` (per synthesis location).

### 3. Schema migration 0006 (claude #4)

`src/striatum/daemon_pg/sql/0006_event_chain_columns.sql`:
- `ALTER TABLE striatumd.events ADD COLUMN previous_hash bytea NOT NULL DEFAULT '\x00';`
- `ALTER TABLE striatumd.events ADD COLUMN row_hash bytea NOT NULL DEFAULT '\x00';`
- `CREATE TABLE striatumd.repo_event_chain_heads (repository_id text PRIMARY KEY REFERENCES striatumd.repositories(repository_id), head_hash bytea NOT NULL, updated_at timestamptz NOT NULL DEFAULT now());`
- Body re-anchors existing rows: for each `striatumd.events` row, read `payload_json->'_event_chain'->>'previous_hash'` / `->>'row_hash'` and write into the new columns. Recompute `repo_event_chain_heads` from the highest row per `repository_id`.

In `src/striatum/daemon_pg/migrations.py`: bump `LATEST_DAEMON_DB_VERSION` from 5 to 6. Add 0006 to the apply list.

Test: `tests/daemon_pg/test_migration_0006_reanchor.py` creates a fixture DB at schema 5 with `payload_json._event_chain` chain metadata, runs the migration, asserts new columns populated byte-equivalently.

### 4. Dead code cleanup (claude HIGH #2)

Per synthesis decision, for each symbol:
- `complete_inline` — define + wire (with test) OR delete (with removed callers).
- `ack_inline` — same.
- `recovery.resume --complete` — same.
- `recovery.auto` live mode — same.

Update tests accordingly. No orphaned imports.

### 5. `striatum daemon doctor --explain` (claude #5)

In `src/striatum/cli/daemon.py` (or `src/striatum/daemon_pg/connection.py` per synthesis):
- New `--explain` flag on `daemon doctor`.
- Output (with `--json`): a list of `{method_name, pg_backed: bool, sqlite_fallback_active: bool}` for every method in the registry.
- Plain output: a Markdown-ish table.

Test: `tests/cli/test_daemon_doctor_explain.py` runs the command in both `--json` and plain modes, asserts the 16 PG-backed methods are listed as such, asserts at least one non-PG method (if any) is listed as sqlite_fallback_active.

### 6. `docs/POSTGRES_TRANSITION.md` runbook (operator friction)

New section, before "Configure the daemon DB connection":
- Heading: "Provision the daemon-required role".
- One paragraph explaining why (owner-role implicit privileges fail the `unsafe_privileges` doctor check).
- Copy-pasteable SQL block:
  ```
  sudo -u postgres psql -d striatum_daemon <<SQL
  CREATE ROLE striatumd_rw WITH LOGIN PASSWORD '<your-pass>';
  GRANT CONNECT ON DATABASE striatum_daemon TO striatumd_rw;
  GRANT USAGE ON SCHEMA striatumd TO striatumd_rw;
  GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA striatumd TO striatumd_rw;
  GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA striatumd TO striatumd_rw;
  REVOKE UPDATE, DELETE ON striatumd.audit_log FROM striatumd_rw;
  REVOKE UPDATE, DELETE ON striatumd.events FROM striatumd_rw;
  REVOKE UPDATE, DELETE ON striatumd.artifacts FROM striatumd_rw;
  ALTER DEFAULT PRIVILEGES IN SCHEMA striatumd GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO striatumd_rw;
  GRANT CREATE ON DATABASE striatum_daemon TO striatumd_rw;
  GRANT CREATE ON SCHEMA striatumd TO striatumd_rw;
  SQL
  ```
- Quote the doctor refusal that surfaces the missing role: `"daemon role must not have UPDATE or DELETE on striatumd.audit_log"`.
- Cite the cleanup follow-up: once `daemon doctor --apply-migrations --provision-role` lands (deferred), this manual step retires.

## Sub-agents (use them aggressively, local only)

- **parity-rig**: conftest + helper + 16 file wiring.
- **capability-denial**: helper + 16 × 6 test cases.
- **schema-0006**: SQL + migration version bump + re-anchor test.
- **dead-code**: per-symbol decisions + cleanup + tests.
- **doctor-explain**: argparse + output shapes + tests.
- **doc-runbook**: POSTGRES_TRANSITION.md updates.

## Forbidden writes

Do NOT touch `src/striatum/daemon.py`, `src/striatum/daemon_rpc/`, `src/striatum/daemon_pg/handlers/workflow_loop/`, `src/striatum/daemon_pg/handlers/__init__.py`, `src/striatum/daemon_pg/handlers/registry.py`, `src/striatum/daemon_pg/handlers/context.py` — Track A owns those. Forbidden for both tracks: `.striatum/`.

## HANDOFF.md content

Per work item: file paths changed + function names + test paths + test command + behavior delta summary. Top-level summary table cross-referencing claude HIGH#1/#2 + codex F2 + claude #4/#5 + operator friction item.

## Byline discipline

Plain markdown line AFTER the front-matter block. Lowercase `author:`. No decoration. Slug shape: `implementer-unknown-model-<NN>`.

One-shot supervised invocation. If `striatum ack` is denied, write the artifact and exit normally.
