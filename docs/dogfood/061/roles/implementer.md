# Implementer Role (Dogfood 061 — RFC 0051 V1 auto-finalize)

author: implementer-role-001

You implement the auto-finalize feature per the synthesis. Single
track; sub-agents per cluster.

## Work surfaces

- `src/striatum/auto_finalize.py` (new, OR per synthesis location)
- `src/striatum/recovery/` (if synthesis lands the hook there)
- `src/striatum/daemon_pg/handlers/` (only if a new internal handler
  is required — most likely reused from V1.5 HIGH#2)
- `tests/test_auto_finalize.py` (new — 4 acceptance tests)
- `tests/daemon_pg/` (if PG-side tests added)

## Forbidden

- `.striatum/` (operational scratch).
- `src/striatum/daemon.py` (daemon-start surface — not in scope).
- `src/striatum/cli/parser.py` (no new CLI verbs for V1; the feature
  is runner-internal).
- `src/striatum/daemon_pg/sql/` (no new schema migration for V1; the
  event types are payload_json values, not new columns).

## Internal helpers (already exported in v1.55.0)

- `striatum.daemon_pg.handlers.workflow_loop.complete_job.complete_inline`
- `striatum.daemon_pg.handlers.workflow_loop.ack_work.ack_inline`
- `striatum.daemon_pg.handlers.workflow_loop.submit_review.publish_artifact_inline`
- `striatum.daemon_pg.handlers.context.RepoHandlerContext` +
  `.append_event` (Schema v6 chain-anchored).

## Tests must cover

1. Happy path: valid artifact + matching byline + flag on → state
   transitions through the documented event sequence.
2. Byline mismatch: artifact exists but byline ≠ expected_author_line
   → no auto-finalize; existing lane-stall blocker visible.
3. Malformed frontmatter: artifact exists but YAML parse fails →
   same lane-stall fall-through.
4. Feature flag off: STRIATUM_AUTO_FINALIZE_ENABLE unset → no scan
   runs (assert the hook function exits early without touching PG).

## Deliverable

`docs/dogfood/061/build/HANDOFF.md` — front-matter `handoff.v1`. Cite
exact file paths + line counts touched, the test-run output, and any
deviation from the synthesis with justification.
