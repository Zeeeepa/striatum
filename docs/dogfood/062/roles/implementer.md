# Implementer Role (Dogfood 062 — RFC 0046 V1.7 attestation gap)

author: implementer-role-001

Implement the V1.7 fix per `DESIGN_SYNTHESIS.md`. Single track,
sub-agents per layer.

**Work surfaces (literal allowed_paths):**

- `src/striatum/daemon_pg/handlers/workflow_loop/submit_review.py`
  — the PG-side gate.
- `src/striatum/daemon_pg/handlers/context.py` — if synthesis says
  to surface `process_execution_count`.
- `src/striatum/cli/mutations.py` — legacy SQLite fixture/quarantine
  regression coverage only. Production publish authority is the PG gate.
- `tests/test_lane_attestation_v17.py` — new file, 4 acceptance
  tests.
- `tests/daemon_pg/` — PG-specific regressions if synthesis adds
  them.

**Forbidden:**

- `.striatum/`
- `src/striatum/daemon.py`
- `src/striatum/cli/parser.py`
- `src/striatum/daemon_pg/sql/` (no schema migration; the
  `process_executions` table already exists per migration 0005)

**RFC 0046 V1 invariants preserved:**

- `--allow-no-process-execution --override-rationale '<reason>'`
  MUST continue to work and produce an audit-chained operator
  override row.
- The new error code is a new refusal reason, not a replacement
  for existing ones.

**Deliverable:** `docs/dogfood/062/build/HANDOFF.md`, byline
`author: implementer-codex-<model>-001`, front-matter `handoff.v1`.
