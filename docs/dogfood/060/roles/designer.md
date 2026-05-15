# Designer Role (Dogfood 060)

Three fresh-design lanes (codex, claude, gemini) produce independent perspectives on the 8-method read-surface PG handler port. Synthesis reconciles; single implement track.

Required reading:

- `docs/rfcs/0048-daemon-side-substrate-migration.md` — particularly the V1 Phase A landing summary and V1.5 follow-up list.
- `docs/dogfood/057/build/track_a/HANDOFF.md` + `track_b/HANDOFF.md` — the Phase A handler pattern you mirror.
- `docs/dogfood/058/OPERATOR_REPORT.md` — the cycle exhaustion lesson (dual-track caused boundary conflicts).
- `src/striatum/daemon_pg/handlers/workflow_loop/*.py` and `recovery_evidence/*.py` — reference implementations of the decorator + `RepoHandlerContext` pattern you mirror.
- `src/striatum/daemon_pg/handlers/registry.py` — the `register_pg_handler` decorator + `resolve_pg_handler` function.
- `src/striatum/daemon_pg/handlers/context.py` — `RepoHandlerContext` shape.
- The legacy functions cited in `prompts/design.md` for each read method.

Output: `docs/dogfood/060/design/<lane>/DESIGN.md`. Cover the 8 read methods + cross-cutting decisions per the prompt.

## Byline

Plain markdown line. Lowercase `author:`. No decoration. Slug shape: `designer-unknown-model-<NN>`.
