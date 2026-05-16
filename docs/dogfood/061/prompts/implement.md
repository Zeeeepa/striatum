# Implement task — RFC 0051 V1 auto-finalize (codex, single track)

Implement the auto-finalize feature per
`docs/dogfood/061/DESIGN_SYNTHESIS.md`. Single track; use sub-agents
per cluster (hook+scan, finalize sequence, events, feature flag).

## Required inputs

- `docs/dogfood/061/DESIGN_SYNTHESIS.md` — your contract. Match the
  locked file paths, function signatures, event shapes, and test
  names exactly.
- `docs/dogfood/061/review/design/REVIEW.md` — the design review.
  Address every named finding before publishing build/HANDOFF.
- The V1.5 inline helpers (already exported in main):
  - `striatum.daemon_pg.handlers.workflow_loop.complete_job.complete_inline`
  - `striatum.daemon_pg.handlers.workflow_loop.ack_work.ack_inline`
  - `striatum.daemon_pg.handlers.workflow_loop.submit_review.publish_artifact_inline`

## Write scope (literal)

`src/striatum/auto_finalize.py`,
`src/striatum/recovery/` (additive only — do not break existing
ticks),
`src/striatum/daemon_pg/handlers/` (if synthesis adds an internal
helper),
`tests/test_auto_finalize.py` (new file — 4 acceptance tests),
`tests/daemon_pg/` (additive),
`docs/dogfood/061/build/`.

## Forbidden

- `.striatum/` (operational scratch).
- `src/striatum/daemon.py` (daemon-start surface).
- `src/striatum/cli/parser.py` (no new CLI verbs in V1).
- `src/striatum/daemon_pg/sql/` (no schema migration in V1).

## Acceptance bullets (cite by ID in build/HANDOFF.md)

A1. `make lint typecheck test` green (note: full suite is ~14min;
    run the targeted set first then the full sweep).
A2. The four named tests pass against an ephemeral PG.
A3. `striatum --repo . status --json` post-implement returns the
    same shape as pre-implement on a fresh fixture.
A4. Audit-chain integrity: 12-worker concurrent denied requests
    still produce a contiguous chain (regression for V1.5 F3).

## Deliverable

`docs/dogfood/061/build/HANDOFF.md` — front-matter `handoff.v1`,
byline `author: implementer-codex-<model>-001`. List exact files
+ line counts touched, test output (last 30 lines from pytest), and
any deviation from synthesis with cited justification.
