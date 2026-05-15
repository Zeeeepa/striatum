# Synthesis Prompt: RFC 0048 Phase C read-surface PG handlers

Produce `docs/dogfood/060/DESIGN_SYNTHESIS.md`. Front matter:

```
---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/060/design/codex/DESIGN.md", "docs/dogfood/060/design/claude_code/DESIGN.md", "docs/dogfood/060/design/gemini/DESIGN.md"]
---
```

Byline AFTER the front-matter block. Plain markdown line, lowercase `author:`, no decoration. Slug shape: `designer-unknown-model-<NN>`.

Reconcile the 3 designs into ONE concrete plan. Pick one path per decision; do NOT enumerate.

## Per-method lock items

For each of the read methods the synthesis includes (default: status, dashboard, list.runs, list.sessions, list.jobs, list.artifacts, list.workflows, run.summary, why, doctor, evidence.export, corpus.export — adjust if you decide some stay on legacy with a one-sentence justification), specify exactly:

| Field | Specificity |
|---|---|
| Legacy source | exact `path:line_range` + function name |
| New handler file | exact path (default `src/striatum/daemon_pg/handlers/reads/<method>.py`) |
| Decorator + signature | exact (default `@register_pg_handler("<method>", read_only=True)`; `def handle(ctx, params) -> dict`) |
| striatumd.\* tables queried | exact list, columns selected |
| Return-shape parity contract | exact top-level keys |
| Test file | exact path under `tests/daemon_pg/handlers/reads/` |
| Parity assertion | per-key diff vs legacy OR shape-only smoke (pick one, justify) |
| Error modes | repo_not_registered / malformed params / no rows (RPC error envelopes) |

## Cross-cutting (pick one, justify in one sentence)

1. **Module layout** — per-method file (recommended) vs grouped.
2. **repository_id scoping** — `WHERE repository_id = $1` discipline vs wrapper.
3. **Parity test strategy** — wire the deferred parity rig from `tests/daemon_pg/handlers/recovery_evidence/conftest.py` (the unused `Seed` / `pg_ctx` / `sqlite_conn` infrastructure) OR shape-only smoke. If wiring: enumerate the conftest changes. If smoke-only: justify on legacy stability.
4. **Single implement track** — LOCKED to single track. Synthesis MUST NOT propose dual-track. Sub-agents inside the implement job (cluster: core-reads, reporting-reads, summary-reads) are the parallelism mechanism.
5. **Decorator registration** — `register_pg_handler` from `daemon_pg.handlers.registry` (already exists, used by Phase A) — no new registration mechanism.
6. **Handler `__init__.py` integration** — single line `from . import reads` in `daemon_pg/handlers/__init__.py` plus `reads/__init__.py` importing each method file to trigger decorator registration.

## Migration & rollout

- Each handler ships in the single implement track; no per-handler commit (they cluster well).
- After this dogfood lands, the daemon-required CLI is functional end-to-end on a migrated repo. The V1.6 `STRIATUM_DAEMON_REQUIRED=0 STRIATUM_TEST_HARNESS=1` escape becomes optional for migrated repos.
- Version bump: v1.51.0 → v1.52.0 (RFC 0048 Phase C complete).

## Out of scope

- All V1.5 follow-up items from dogfood-058 (those have their own dogfood path; don't pull in here).
- Phase B (Go core parity).

## Byline discipline

Plain markdown line AFTER the front-matter block. Lowercase `author:`. No decoration. Slug shape: `designer-unknown-model-<NN>`.

One-shot supervised invocation. If `striatum ack` is denied, write the artifact and exit normally.
