# Implement prompt: RFC 0018 step 3

Implement against `docs/dogfood/018/DESIGN_SYNTHESIS.md` modulo
findings from `review/design/DESIGN_REVIEW.md`.

Deliverables:

1. Migration v10 in `src/striatum/migrations.py`.
2. `record_review_verdict` in `src/striatum/db.py` writes
   `verdicts.posture` on INSERT.
3. Per-surface code:
   - `src/striatum/cli/introspect.py` (status `verdicts_by_posture`)
   - `src/striatum/cli/run_summary.py` (per-posture markdown)
   - `src/striatum/cli/evidence.py` (posture in verdict block)
   - `src/striatum/run_graph.py` or wherever (posture on review nodes)
   - dashboard renderer (per-posture line)
   - web UI (posture chip)
4. `tests/test_review_postures_introspection.py` covering each.
5. Doc updates — SPEC, UBIQUITOUS_LANGUAGE, DECISION_LOG D071,
   TODO F18, RFC 0018 status → `accepted` (V1+step 3),
   CHANGELOG `## 1.9.0 — 2026-05-09`, version bumps.
6. `docs/dogfood/018/BUILD_HANDOFF.md`.

Constraints: stay inside write_scope; `make lint`, `make typecheck`,
`make test` must pass; report results in BUILD_HANDOFF.md.
