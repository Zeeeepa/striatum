# Implement — RFC 0046 V1 lane evidence guard

Blocked until `review_design` returns an accepting verdict.

Implement per `docs/dogfood/053/DESIGN_SYNTHESIS.md`. Claude impl.

Write scope: `src/striatum/`, `tests/`,
`docs/rfcs/0046-lane-evidence-guard-at-publish-artifact.md`,
`docs/dogfood/053/build/`. No writes to `.striatum/`, `go/`, prior
dogfoods.

F-by-F:

- **F-schema:** ALTER TABLE artifacts ADD COLUMN
  attestation_override_rationale TEXT. Bump
  `LATEST_VERSION` in `src/striatum/migrations.py`. Postgres
  migration as a new `.sql` under
  `src/striatum/daemon_pg/sql/`.
- **F-guard:** in `src/striatum/artifacts.py::publish_artifact`,
  after byline validation, run the lane evidence check when the
  byline canonicalises to a model byline. Refuse with
  `ArtifactError("lane_evidence_missing: ...")`.
- **F-override:** add `--allow-no-process-execution` +
  `--override-rationale` flags to the `publish-artifact` argparse
  block in `src/striatum/cli/parser.py`. Wire through
  `src/striatum/cli/dispatch.py` so `publish_artifact` receives the
  override + rationale. The override-with-empty-rationale path
  refuses with exit code 2.
- **F-event:** new event type
  `provenance.publish_without_process_execution`. Register in
  the events module. Emit on override path.
- **F-test:** `tests/test_lane_evidence_guard.py` with four
  scenarios per RFC 0046 acceptance.

HANDOFF: `docs/dogfood/053/build/HANDOFF.md` with byline matching
expected_author_line. Summarize shipped scope + test commands +
deviations.
