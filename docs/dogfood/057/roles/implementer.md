# Implementer Role (Dogfood 057)

Two parallel tracks. Each implementer is fresh-session, sub-agents allowed (local only for claude; aggressive for codex).

Track A (codex) — workflow-loop handlers from `src/striatum/cli/mutations.py`:

`register_session`, `claim_next`, `ack_work`, `complete_job`, `release_lease`, `block_job`, `record_verdict`, `submit_review`, `override_review_verdict`.

Track B (claude) — recovery + evidence handlers from `src/striatum/cli/recovery.py` + `src/striatum/cli/evidence.py`:

`stale_leases`, `requeue_stale`, `cancel_job`, `process_reconcile`, `resume_blocker`, `auto_publish_stale_artifacts`, `evidence_export`.

Inputs (mandatory reading):

- `docs/dogfood/057/DESIGN_SYNTHESIS.md` — locks every path, signature, test file.
- `docs/dogfood/057/review/design/REVIEW.md` — addresses any `accept_with_findings` items before coding.
- The RFCs (0048, 0043, 0033, 0030) for context.
- The source files for the methods you're porting (your starting point).

Output: `docs/dogfood/057/build/<track>/HANDOFF.md` per the implement prompt.

Hard constraint: stay inside your track's `write_scope.allowed_paths`. Track A may not touch `recovery.py`/`evidence.py`; Track B may not touch `mutations.py`. Neither touches `src/striatum/daemon_pg/sql/` (schema locked at migration 0005).

## Byline discipline

Plain markdown line. Lowercase `author:`. No decoration. Slug shape: `implementer-unknown-model-<NN>`.
