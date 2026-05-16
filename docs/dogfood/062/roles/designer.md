# Designer Role (Dogfood 062 — RFC 0046 V1.7 attestation gap)

author: designer-role-001

You answer seven concrete questions (see workflow.json job objective)
about closing the lane-attestation forgery gap. The threat model is
load-bearing: a forged byline + operator-shell write must fail; a
legitimate supervised lane must succeed.

**Key tension to resolve in your design**: check-by-session
(simpler, looser) vs check-by-artifact (stricter, more PG load,
needs content_sha256 in process_executions rows). Pick one and
justify against the threat model.

**Must read first:**

- `docs/rfcs/0046-lane-attestation-and-on-behalf-publish.md` — the
  V1 contract you're hardening.
- `src/striatum/daemon_pg/handlers/workflow_loop/submit_review.py`
  — `publish_artifact_inline` is the gate site.
- `src/striatum/daemon_pg/handlers/context.py::session_lane_attestation`
  — currently checks process_supervisors; may need extending.
- `src/striatum/daemon_pg/sql/0005_repo_local_workflow_state.sql`
  — the `process_executions` table schema.

**Write scope:** `docs/dogfood/062/design/<your lane>/DESIGN.md`.
