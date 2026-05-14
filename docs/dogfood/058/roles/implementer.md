# Implementer Role (Dogfood 058)

Two parallel tracks, fresh-session, local sub-agents allowed (aggressive for codex).

Track A (codex): router fail-closed + audit-chain locking + accept loop + append-only role grants. Allowed paths: `src/striatum/daemon.py`, `src/striatum/daemon_rpc/`, `src/striatum/daemon_pg/handlers/workflow_loop/`, `daemon_pg/handlers/__init__.py`, `daemon_pg/handlers/registry.py`, `daemon_pg/handlers/context.py`, `tests/daemon_rpc/`, `tests/daemon_pg/handlers/workflow_loop/`, `docs/dogfood/058/build/track_a/`.

Track B (claude): parity rig + capability-denial tests + schema migration 0006 + dead code cleanup + `daemon doctor --explain` + `POSTGRES_TRANSITION.md` runbook. Allowed paths: `tests/daemon_pg/handlers/`, `tests/daemon_pg/`, `src/striatum/daemon_pg/sql/`, `src/striatum/daemon_pg/migrations.py`, `src/striatum/daemon_pg/connection.py`, `src/striatum/cli/daemon.py`, `src/striatum/cli/recovery.py`, `src/striatum/daemon_pg/handlers/recovery_evidence/`, `docs/POSTGRES_TRANSITION.md`, `docs/dogfood/058/build/track_b/`.

Inputs (mandatory reading): `docs/dogfood/058/DESIGN_SYNTHESIS.md`, `docs/dogfood/058/review/design/REVIEW.md`, the V1 reviews + handoffs, and the source files for items you're modifying.

Output: `docs/dogfood/058/build/<track>/HANDOFF.md` per the implement prompt.

## Byline

Plain markdown line. Lowercase `author:`. No decoration. Slug shape: `implementer-unknown-model-<NN>`.
