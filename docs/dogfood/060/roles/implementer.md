# Implementer Role (Dogfood 060)

Single track, codex lane, fresh-session, sub-agents aggressively.

Scope: port the 8 read-surface CLI methods (status, dashboard, list.*, run.summary, why, doctor, evidence.export, corpus.export — per synthesis) to PG-backed handlers under `src/striatum/daemon_pg/handlers/reads/`.

Inputs (mandatory reading):

- `docs/dogfood/060/DESIGN_SYNTHESIS.md` — locks every path, signature, test file.
- `docs/dogfood/060/review/design/REVIEW.md` — addresses any `accept_with_findings` items before coding.
- The Phase A handlers under `src/striatum/daemon_pg/handlers/workflow_loop/` and `recovery_evidence/` — reference patterns.
- The legacy read functions cited in the synthesis — read each one before porting.

Output: `docs/dogfood/060/build/HANDOFF.md` per the implement prompt.

Hard constraint: stay inside `write_scope.allowed_paths`. You may NOT touch `workflow_loop/`, `recovery_evidence/`, `daemon_pg/sql/`, `daemon_rpc/`, `daemon.py`, `cli/daemon_rpc_route.py`, `cli/dispatch.py`.

## Sub-agent clusters

- **core-reads**: status, why, doctor, dashboard.
- **reporting-reads**: list.runs, list.sessions, list.jobs, list.artifacts, list.workflows.
- **summary-reads**: run.summary, evidence.export, corpus.export.

Each sub-agent ports its cluster + writes its tests. You integrate and write the HANDOFF.

## Byline

Plain markdown line. Lowercase `author:`. No decoration. Slug shape: `implementer-unknown-model-<NN>`.
