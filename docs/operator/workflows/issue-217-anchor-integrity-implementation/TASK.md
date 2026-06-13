# Issue 217 Artifact-Anchor Integrity Implementation Task

Implement GitHub issue #217 using the completed design artifact:
`docs/operator/artifacts/issue-217-anchor-integrity-design/DESIGN.md`.

## Acceptance Criteria

- With blob storage not configured, doctor behavior remains unchanged for this
  issue.
- With blob storage configured but unreachable, missing, or not
  repo-provisioned, doctor preserves current blob diagnostics and skips the new
  cross-check.
- With blob storage configured and bucket status `ok`, a completed repo-write
  job whose anchored commit contains each relevant artifact body at the
  recorded repository path produces no new doctor problem.
- With blob storage configured and bucket status `ok`, a content mismatch makes
  doctor `ok: false` and emits a stable problem plus a verbose
  `problem_records` entry.
- With blob storage configured and bucket status `ok`, a missing file at the
  anchored repository path is reported distinctly from a hash mismatch.
- Tests cover disabled/blob-unhealthy skip, healthy match, healthy mismatch,
  healthy missing-file, and both durable anchor forms where practical.
- Documentation for doctor/blob diagnostics mentions the new cross-check and
  skip conditions.

## Out Of Scope

- Changing artifact publish or work completion semantics.
- Automatic repair.
- Moving git refs, modifying the primary checkout, or writing repository files
  during doctor.
- Making blob storage mandatory.
- Bulk corpus or dogfood migration.
