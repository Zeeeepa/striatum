# RFC 0001: Run Recovery And Dogfood Fixes

Status: accepted
Date: 2026-05-06
Context:
`docs/RFC_0014_DOGFOOD_FIX_SPEC.md`,
`docs/reviews/rfc-0014-operational-artifact-home/STRIATUM_VALIDATION_NOTES.md`

## Problem

The RFC 0014 validation run proved the SQLite control plane could coordinate
review jobs, publish artifacts, and route verdicts. It also exposed that the
runner is hard to recover from after an honest block:

- `status --json` did not show the open blocker or next useful action;
- `why <blocker_id>` could not explain blocker ids;
- redacted runner evidence had to be assembled by hand;
- common review submission required multiple manual commands;
- adapter constraints were declared in prose but not surfaced as enforcement
metadata;
- branch confirmation was records-only but not explicit enough in command
output;
- root-review `needs_revision` behavior was surprising because the workflow did
  not declare the checkpoint policy directly.

## Goals

- Make blocked runs recoverable through `status`, `why`, and committed evidence
  exports.
- Preserve the V1 boundary: SQLite is live control-plane state; repo artifacts
  are durable provenance.
- Reduce command friction for the common review artifact + verdict path.
- Make declared adapter constraints visible in work packets and state.
- Make root-review revision behavior explicit in workflow config.

## Non-Goals

- Do not commit `.striatum/` SQLite state.
- Do not capture transcripts by default.
- Do not add Slack, TUI, web, MCP, or autonomous process launch as part of this
  RFC.
- Do not decide Engram RFC 0014's disposition.

## Proposal

Promote the dogfood fix spec into the first runner RFC and implement the
following product changes:

1. Add `striatum evidence export --run-id <run_id> --path <repo_path>` to
   write a redacted Markdown snapshot of run state, jobs, blockers, verdicts,
   artifacts, status, doctor output, and blocked downstream jobs.
2. Extend `status --json` with open blockers, human checkpoints, latest
   non-accepting review verdicts, claimable jobs, blocked downstream jobs, and
   deterministic `next_actions`.
3. Extend `why <id> --json` to support run, job, queue message, blocker,
   artifact, verdict, and session ids.
4. Add `submit-review` as the atomic command for publishing a review artifact
   and recording its verdict.
5. Add lane-level adapter constraint declarations and expose requested vs.
   enforced/advisory/unsupported status in work packets.
6. Make `branch confirm --json` report `records_only`, requested branch,
   detected current branch, and mismatch warnings.
7. Add explicit root-review `needs_revision` policy to RFC-style workflows.

## Acceptance Criteria

- Tests cover blocked review verdicts in `status`, `why`, and evidence export.
- `submit-review` publishes an artifact, records the verdict, and applies gate
  behavior in one command.
- Workflow validation accepts known adapter constraints and rejects unknown
  values.
- The RFC 0014 fixture declares whether root-review `needs_revision` is an
  expected human checkpoint or routes to a revision job.
- `docs/SPEC.md` and `UBIQUITOUS_LANGUAGE.md` reflect the accepted
  behavior after implementation.

## Open Questions

- Should evidence export be a required artifact for every blocked run, or only
  for user-requested validation/reporting workflows?
- Should root-review `needs_revision` default to human checkpoint for all RFC
  workflows, or must every workflow state the policy explicitly?

## Implementation Notes

Each Proposal item has landed on `main`. Files and tests below prove it; the
RFC moves to `accepted` because no further behavior is owed.

1. `striatum evidence export --run-id <id> --path <repo_path>` writes a
   redacted Markdown snapshot of run state, jobs, blockers, verdicts,
   artifacts, status, doctor output, and blocked downstream jobs.
   - Implementation: `src/striatum/cli/evidence.py:evidence_export`,
     `evidence_snapshot`, `render_evidence_markdown`, `redact_evidence_payload`.
   - Test: `tests/test_cli_mvp.py:test_evidence_export_writes_redacted_markdown_and_rejects_bad_paths`.
2. `status --json` returns open blockers, human checkpoints, latest
   non-accepting review verdicts, claimable jobs grouped by `(role, lane)`,
   blocked downstream jobs, and deterministic `next_actions`.
   - Implementation: `src/striatum/cli/introspect.py:status` with helpers
     `blocker_summaries`, `latest_non_accepting_verdicts`,
     `claimable_jobs_by_role_lane`, `blocked_downstream_jobs`, `next_actions`.
   - Tests: `test_cli_mvp.py:test_why_resolves_blocker_artifact_and_verdict`
     and the surrounding `status` cases that assert the new keys.
3. `why <id> --json` resolves runs, jobs, queue messages, blockers,
   artifacts, verdicts, sessions, and process records.
   - Implementation: `src/striatum/cli/introspect.py:why`.
   - Test: `tests/test_cli_mvp.py:test_why_resolves_blocker_artifact_and_verdict`.
4. `submit-review` is a single command that publishes the review artifact and
   records the verdict atomically (with prevalidation against the lease and
   write scope before any artifact is written).
   - Implementation: `src/striatum/cli/mutations.py:submit_review`,
     `prevalidate_submit_review`.
   - Tests: `tests/test_cli_mvp.py:test_submit_review_publishes_artifact_and_applies_gate`,
     `:test_submit_review_prevalidates_before_publishing_artifact`,
     `:test_submit_review_rejects_non_review_before_publishing_artifact`.
5. Lane-level adapter constraint declarations with enforcement levels
   `unsupported|advisory|advisory_strict|enforced` are validated and surfaced
   in work packets as the requested vs. enforced status.
   - Implementation: `src/striatum/db.py` constants
     `ADAPTER_ENFORCEMENT_LEVELS`, `adapter_constraint_enforcement`,
     `adapter_enforcement_satisfies`; lane validation in
     `src/striatum/workflow.py:_validate_lane_constraints`.
   - Test: `tests/test_cli_mvp.py` packet assertion at the
     `constraints = packet["adapter_constraints"]` line, plus the workflow
     validation rejection cases.
6. `branch confirm --json` reports `records_only`, `requested_branch`,
   `current_git_branch`, `warning`, `mode`, and `created`.
   - Implementation: `src/striatum/cli/mutations.py:branch_confirm` and
     `current_git_branch`.
   - Test: `tests/test_cli_mvp.py:test_branch_confirm_reports_records_only_and_mismatch`.
7. Root-review `needs_revision` policy is declarable on the workflow via
   `review_revision_policy.root_review_needs_revision` (`human_checkpoint` or
   `declared_cycle`); validation enforces that `declared_cycle` requires a
   matching cycle and that `human_checkpoint` does not silently swallow a
   verdict.
   - Implementation: `src/striatum/workflow.py:_validate_revision_policy` and
     plan output via `_planned_review_gates`.
   - Tests: `tests/test_cli_mvp.py:test_rfc_0014_fixture_declares_root_review_revision_policy`,
     `:test_declared_cycle_policy_requires_root_review_cycles`.
