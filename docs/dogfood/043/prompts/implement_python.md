# Track A Implement: RFC 0045 Python Core (codex)

Blocked until `review_design_python` returns an accepting verdict.

Implement RFC 0045 Python core per `docs/dogfood/043/DESIGN_SYNTHESIS_python.md`. **You write Python only.** Do NOT cross into frontend scope.

**Your scope (codex Python-side):**

- `src/striatum/workflow.py` — accept `striatum.workflow.v1.1`, parse `phases[]`, validate `phase_id` on jobs, enforce cross-phase edge routing through `phase_synthesis`, materialize phases in runtime state.
- `src/striatum/workflow_generator/` — add a `multi_phase` shape that emits `phases[]` plus `phase_synthesis` jobs at boundaries.
- `src/striatum/cli/` — add `striatum workflow upgrade --add-phases` verb that rewrites a v1 workflow to v1.1.
- `src/striatum/dashboard.py` — surface per-phase progress in the compact terminal view.
- `src/striatum/service.py` — emit per-phase status in the status feed.
- `tests/` — add `tests/fixtures/multi_phase_workflow.json` and an e2e test that exercises the v1.1 lifecycle (load → validate → execute → status). Add a backwards-compat test asserting every existing v1 fixture validates unchanged.
- `docs/dogfood/043/build/python/HANDOFF.md` — handoff summarizing shipped scope, files touched, test results, deviations from the synthesis (if any) with one-line rationale.

**Use sub-agents aggressively** — one per concern, dispatched in parallel:

- Sub-agent 1: validator changes in `src/striatum/workflow.py` (schema_version handling, phases parsing, cross-phase edge rule).
- Sub-agent 2: runtime materialization (phase metadata loaded into runtime state).
- Sub-agent 3: generator `multi_phase` shape under `src/striatum/workflow_generator/`.
- Sub-agent 4: `striatum workflow upgrade --add-phases` CLI verb.
- Sub-agent 5: dashboard + service status reporting.
- Sub-agent 6: fixture + e2e test + backwards-compat test sweep.

Reconcile sub-agent outputs yourself before writing HANDOFF.

**Do NOT write to**: anything outside `allowed_paths`. Specifically not `src/striatum/web/`, `docs/HOW_TO_HUMAN.md`, `docs/SPEC.md`, `docs/rfcs/`.

Verification: `make lint`, `make typecheck`, `make test` all pass. The new e2e test exercises v1.1; backwards-compat test confirms v1 fixtures unchanged.

One-shot supervised invocation. Do not ask follow-ups. If `striatum ack` is denied, write the HANDOFF and exit normally.
