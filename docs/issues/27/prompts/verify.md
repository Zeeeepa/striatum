# Verify -- GH #27

Fresh-context review. Posture: `compliance_license`.

## Read

- `docs/issues/27/SPEC.md`
- `docs/issues/27/SCOPE.md`
- `docs/issues/27/build/HANDOFF.md`
- the changed files named by the handoff

## Output

Write `docs/issues/27/review/REVIEW.md` with `striatum.finding.v1` front matter.

Include:

- final verdict (`accept`, `accept_with_findings`, `needs_revision`, or `reject`);
- per-bullet acceptance verification with file:line evidence;
- adversarial probes:
  - **Negative case**: try to UPDATE `content_sha256`, `repo_path`, `artifact_kind`, `author_line` individually. Each must be refused with `P0001`.
  - **Positive case**: UPDATE all three blob columns at once succeeds.
  - **Mixed case**: UPDATE that touches one blob column AND one non-blob column is refused (don't let a blob-column update sneak through alongside an identity change).
  - **DELETE still refused**: the trigger refinement is for UPDATE only; DELETE on `artifacts` must still raise.
  - **`striatumd_rw` end-to-end**: re-run `artifact.backfill_blob` against a fresh artifact row with NULL blob_key, with the trigger ENABLED, and confirm the backfill succeeds without owner intervention.
  - **Migration idempotency**: running the new migration twice (or against a DB where it's already applied) is a no-op.
- test/verification assessment;
- findings with severity and exact remediation when any gap remains.
