# Implement task — RFC 0046 V1.7 attestation gap closure

Build to `DESIGN_SYNTHESIS.md`. Single track. Sub-agents per layer
(PG gate, SQLite mirror, tests).

**Required inputs:** `DESIGN_SYNTHESIS.md`, `review/design/REVIEW.md`.
Every gemini design-review finding addressed before publishing build.

**Allowed paths:** see `roles/implementer.md` "Work surfaces". Do not
touch `src/striatum/daemon_pg/sql/` (no migration) or
`src/striatum/cli/parser.py` (no new CLI verbs — the gate is internal).

**Acceptance bullets (cite by ID in HANDOFF):**

- A1. `make lint typecheck test` green on the dogfood branch.
- A2. Four new tests in `tests/test_lane_attestation_v17.py` pass:
      forgery refused; supervised path allowed; override path allowed
      with audit reason; multi-session run where one lane is supervised
      and one is forged → only supervised publishes.
- A3. Existing RFC 0046 V1 operator-override tests still pass — the
      `--allow-no-process-execution --override-rationale '<reason>'`
      flow is intact.
- A4. The new audit row carries the `process_execution_id` reference
      so an auditor can trace the lane attestation backward.

**Deliverable:** `docs/dogfood/062/build/HANDOFF.md`.
