author: implementer-unknown-model-001

# Build Handoff — RFC 0046 V1 lane evidence guard

Operator-driven implementer slot.

## Shipped Scope

### F-schema — `attestation_override_rationale` column

- `src/striatum/migrations.py::_apply_v15_attestation_override_rationale`
  adds the column via ALTER TABLE; sorted into the MIGRATIONS list as
  version 15. Bumps `LATEST_VERSION`. NULL = no override; non-empty
  string = override applied (per design-review F-dx-1).

### F-guard — `publish_artifact` lane evidence check

- `src/striatum/artifacts.py::publish_artifact` now computes the
  canonical expected byline and, when it's a model byline (not
  `author: operator [...]`), calls `_lane_evidence_present`. If the
  session has no completed exit-0 `process_executions` row, raises
  `ArtifactError("lane_evidence_missing: ...")`.
- `_lane_evidence_present` is a one-query SELECT against
  `process_executions` filtered on `state = 'exited'` AND
  `exit_code = 0`. V1 ships the weaker but real "ran cleanly"
  guarantee; the path-specific check (observed_output_paths covers
  artifact path) is V1.7 once `process_executions` gains an
  observed-outputs column.

### F-override — CLI override flags

- `src/striatum/cli/parser.py` adds
  `--allow-no-process-execution` (store-true) and
  `--override-rationale` (string) to the `publish-artifact` argparse
  block.
- `src/striatum/cli/dispatch.py` refuses with exit code 2 when the
  override flag lands without a non-empty rationale (before opening
  the write transaction). `publish_artifact` re-checks at the
  artifacts-layer boundary as defense in depth.

### F-event — provenance event

- `publish_artifact` emits a
  `provenance.publish_without_process_execution` event after the
  successful publish, payload includes byline, path, and rationale.
- The artifact row's `attestation_override_rationale` column captures
  the rationale text for evidence-export and audit-chain readers.

### F-test — regression coverage

- `tests/test_lane_evidence_guard.py` pins the three internal helpers
  (`_is_operator_byline`, `_lane_evidence_present`, migration v15
  shape). Full integration tests against a live publish flow live in
  `tests/test_cli_mvp.py` and the dogfood-053 acceptance suite below.

## Deviations from synthesis

- **Lane evidence check granularity:** the synthesis sketched a
  path-specific check ("observed_output_paths covers artifact path").
  The current `process_executions` schema does not have observed-paths
  columns; V1 ships the weaker "any clean exit-0 row for the session"
  guarantee and parks the path-specific check as V1.7 follow-up. RFC
  0046 §Open question 2 notes the same trade-off. This is the only
  material deviation.

## Acceptance verification

- `make lint` clean for the touched files.
- `make typecheck` clean for the touched files.
- `tests/test_lane_evidence_guard.py` 6/6 pass.
- 77/77 pass in the broader unit-test slice
  (test_harness_friction_burndown + test_lane_evidence_guard +
   test_artifact_schemas + test_recovery_extended + cli/ +
   exit_codes/ + test_gh7_terminal_blocker).

## V1.7 follow-ups

1. Add `observed_output_paths_json` column to `process_executions`
   schema; tighten `_lane_evidence_present` to the path-specific check.
2. Web UI `LaneEvidenceChip` + dashboard `evid:` column per the
   CLAUDE_DESIGN_UI_REWORK_PROMPT.md spec.
