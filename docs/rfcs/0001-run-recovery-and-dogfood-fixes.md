# RFC 0001: Run Recovery And Dogfood Fixes

Status: proposed
Date: 2026-05-06
Context:
`agent-runner/docs/RFC_0014_DOGFOOD_FIX_SPEC.md`,
`docs/reviews/rfc-0014-operational-artifact-home/AGENT_RUNNER_VALIDATION_NOTES.md`

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

- Do not commit `.agent_runner/` SQLite state.
- Do not capture transcripts by default.
- Do not add Slack, TUI, web, MCP, or autonomous process launch as part of this
  RFC.
- Do not decide Engram RFC 0014's disposition.

## Proposal

Promote the dogfood fix spec into the first runner RFC and implement the
following product changes:

1. Add `agent_runner evidence export --run-id <run_id> --path <repo_path>` to
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
- `agent-runner/docs/SPEC.md` and `UBIQUITOUS_LANGUAGE.md` reflect the accepted
  behavior after implementation.

## Open Questions

- Should evidence export be a required artifact for every blocked run, or only
  for user-requested validation/reporting workflows?
- Should root-review `needs_revision` default to human checkpoint for all RFC
  workflows, or must every workflow state the policy explicitly?
