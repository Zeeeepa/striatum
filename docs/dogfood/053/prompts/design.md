# Design — RFC 0046 V1 lane evidence guard

Read first:
- `docs/rfcs/0046-lane-evidence-guard-at-publish-artifact.md` (this is the spec; treat it as authoritative)
- `docs/dogfood/050/review/build/gemini/REVIEW.md` (A1/A2/A3 — the original motivation)
- `src/striatum/artifacts.py::publish_artifact` + `validate_optional_markdown_author_line` + `expected_author_line`
- `src/striatum/identity.py::artifact_author_identity` + `session_lane_attestation`
- `src/striatum/schema.py` (find where `artifacts` table is declared)
- `src/striatum/cli/parser.py::publish` subparser
- `src/striatum/cli/dispatch.py::_resolve_publish_defaults` (V1.41 — your new override flag chains through here)

Design closure of RFC 0046 V1. Five concrete deltas:

- **F-schema:** add `attestation_override_rationale TEXT` column to
  `artifacts` table. Schema migration in `src/striatum/migrations.py`
  + the Postgres daemon path in
  `src/striatum/daemon_pg/sql/`. Existing rows read as NULL.
- **F-guard:** in `publish_artifact`, after byline validation, compute
  canonical byline. If it's a model byline (`<role>-<model>-<ord>`),
  look up `process_executions` rows for the session and verify the
  artifact path appears in at least one row's
  `observed_output_paths_json`. If not, raise `ArtifactError`.
- **F-override:** new CLI flags `--allow-no-process-execution` +
  `--override-rationale "..."` on `publish-artifact`. Override
  requires non-empty rationale, writes the rationale to the new
  column, and emits a `provenance.publish_without_process_execution`
  event.
- **F-event:** the override event payload includes
  `artifact_id`, `session_id`, `byline`, `expected_path`,
  `rationale`. Event-type registry entry in
  `src/striatum/events.py` (find where existing event types are
  registered).
- **F-test:** regression test `tests/test_lane_evidence_guard.py`
  covering all four cases from RFC 0046 acceptance.

Output: 800-1200 word design proposal listing files to touch,
SQL migration shape, code sketches (concise), test names + scenarios.
