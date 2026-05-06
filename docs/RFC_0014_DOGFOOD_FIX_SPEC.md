# RFC 0014 Dogfood Fix Specification

Status: proposed implementation spec
Date: 2026-05-06
Source validation:
`docs/reviews/rfc-0014-operational-artifact-home/AGENT_RUNNER_VALIDATION_NOTES.md`

## Purpose

This spec turns the RFC 0014 dogfood findings into concrete `agent_runner`
follow-up work. The dogfood run correctly blocked after a root review returned
`needs_revision` without a declared revision cycle. That was a useful result,
but it exposed gaps in audit export, status visibility, blocker introspection,
review submission ergonomics, adapter constraints, and workflow modeling.

This spec is for the runner product. It does not decide the disposition of
Engram RFC 0014.

## Problems Observed

The validation run produced these runner-level findings:

1. `status --json` reported aggregate counts but did not surface the open
   blocker or next useful coordinator action.
2. `why <blocker_id> --json` failed because blocker IDs are not supported
   introspection targets.
3. Runner state evidence lived only in ignored `.agent_runner/` SQLite until a
   manual redacted snapshot was added to the validation notes.
4. Review submission required multiple manual commands: publish artifact,
   capture artifact ID, then pass that ID to `verdict`.
5. The process adapter could describe local-only scope but could not enforce or
   record whether launched lanes respected it.
6. Branch confirmation remained split between manual `git switch` and
   `agent_runner branch confirm`.
7. The RFC 0014 workflow had no explicit `needs_revision` path from root review
   jobs, so the runner opened a human checkpoint.

## Goals

- Make runner state auditable from committed, redacted artifacts without
  committing `.agent_runner/`.
- Make `status` and `why` sufficient for coordinator recovery after blocks.
- Reduce the most common review-artifact command sequence to one safe command.
- Preserve the V1 control-plane boundary: SQLite is live state; repo artifacts
  are durable provenance.
- Keep provider/model portability. Adapter constraints are declared in workflow
  config and recorded as enforced or advisory per adapter.
- Make root-review revision behavior explicit in RFC-style workflows.

## Non-Goals

- Do not commit SQLite state, transcripts, or broad model logs.
- Do not build Slack, TUI, web dashboard, MCP, or autonomous provider launch as
  part of this fix set.
- Do not make `agent_runner` parse repo marker files as live queue state.
- Do not let adapter constraints create a false sandbox guarantee for CLIs that
  cannot actually enforce them.
- Do not accept or revise Engram RFC 0014 in this spec.

## Requirements

### R001: Redacted Run Evidence Export

Add:

```text
agent_runner evidence export --run-id <run_id> --path <repo_path> [--json]
```

The export writes a curated, redacted run snapshot suitable for commit.

The snapshot must include:

- run id, workflow id, workflow version, branch, and run state;
- job ids, workflow job ids, job states, roles, lanes, attempts, and dependency
  summaries;
- open blockers and human checkpoints;
- verdict ids, verdict values, reviewer job ids, and linked artifact ids;
- artifact ids, logical names, artifact kinds, repo paths, and content hashes;
- aggregate `status` output;
- `doctor` output;
- downstream jobs blocked by a failed or waiting gate;
- export timestamp and schema version.

The snapshot must not include:

- transcript bodies;
- prompt payloads beyond repo-relative prompt paths;
- private corpus content;
- machine-specific absolute paths, except redacted as `<repo-root>` or
  `<state-db>`;
- raw terminal output beyond explicit command-result summaries.

Recommended default path:

```text
docs/reviews/<workflow_id>/RUN_EVIDENCE_<run_id>.md
```

For arbitrary workflows, the coordinator may choose another repo-relative path.

Acceptance:

- Export works from a live initialized run.
- Export fails closed if the target path is outside the repo or under
  `.agent_runner/`.
- Exported Markdown is deterministic enough for review after redaction.
- Tests assert that ignored SQLite is not needed to inspect the committed
  export.

### R002: Status Surfaces Recovery State

Extend:

```text
agent_runner status --json
```

The JSON response must include:

- existing aggregate run/job counts;
- open blocker summaries;
- human checkpoint summaries;
- latest non-accepting review verdict per waiting or failed review job;
- claimable jobs grouped by role/lane;
- blocked downstream jobs with the dependency or blocker that prevents them;
- a `next_actions` array with deterministic coordinator suggestions.

Example `next_actions` values:

- `resolve_human_checkpoint`
- `revise_workflow_cycle`
- `claim_available_work`
- `export_run_evidence`
- `inspect_blocker`

Acceptance:

- The RFC 0014 blocked state would have shown the Codex `needs_revision`
  checkpoint and the blocked ledger/synthesis/final-review jobs directly in
  `status --json`.
- Existing status callers remain compatible: aggregate counts stay present.

### R003: Expanded `why` Introspection

Extend:

```text
agent_runner why <id> --json
```

Supported targets:

- run id;
- job id;
- queue message id;
- blocker id;
- artifact id;
- verdict id;
- session id;

For blocker IDs, the response must include:

- blocker state, severity, kind, description;
- owning run/job/session;
- related verdict if one created the blocker;
- blocked downstream jobs if any;
- recommended next actions.

Acceptance:

- `why blk_... --json` succeeds for the RFC 0014 checkpoint shape.
- Unknown ids still return a stable not-found error.

### R004: Atomic Review Submission

Add:

```text
agent_runner submit-review \
  --session-id <session_id> \
  --job-id <job_id> \
  --lease-id <lease_id> \
  --path <repo_path> \
  --verdict accept|accept_with_findings|needs_revision|reject \
  [--logical-name review] \
  [--kind finding] \
  [--rationale <text>] \
  [--json]
```

Behavior:

1. Validate active lease and review job ownership.
2. Publish the review artifact.
3. Record the verdict with the published artifact id.
4. Apply the same gate behavior as `verdict`.
5. Return artifact id, verdict id, job state, run state, blocker id if created,
   and newly enqueued downstream jobs.

This command does not replace `publish-artifact` or `verdict`; it composes the
common review path.

Acceptance:

- A review job can be completed or checkpointed with one command.
- `needs_revision` without a declared cycle returns the created blocker id.
- Existing transition tests still cover low-level commands.

### R005: Adapter Constraint Declarations

Extend workflow lane config with optional adapter constraints:

```json
{
  "constraints": {
    "network": "forbidden",
    "transcripts": "off",
    "repo_scope": "local_only"
  }
}
```

Constraint values:

- `network`: `allowed`, `forbidden`, or `advisory_forbidden`;
- `transcripts`: `off`, `redacted`, or `allowed`;
- `repo_scope`: `local_only` or `unrestricted`.

Adapters must record an enforcement result when launching or preparing a lane:

```json
{
  "constraint": "network",
  "requested": "forbidden",
  "enforcement": "enforced|advisory|unsupported"
}
```

For V1 process adapters, constraints may be advisory if the underlying CLI
cannot enforce them. The work packet must still include the requested
constraints and the adapter enforcement result.

Acceptance:

- Workflow validation accepts the constraint object and rejects unknown values.
- Work packets expose constraints.
- The runner records whether constraints were enforced, advisory, or
  unsupported.

### R006: Branch Confirmation Clarity

Keep V1's no-automatic-git default, but make branch behavior explicit.

Add to `branch confirm --json` response:

- `records_only: true` when the runner did not execute `git switch`;
- current git branch if detectable;
- requested branch;
- mismatch warning if current branch differs from the confirmed branch.

Acceptance:

- The RFC 0014 validation prompt no longer needs to explain this caveat in
  prose alone; the command output makes it visible.

### R007: RFC Review Revision Workflow Pattern

Update the RFC 0014 fixture or add a new fixture version with an explicit root
review revision path.

One acceptable shape:

```text
parallel reviews -> findings ledger -> synthesis -> final review
review_* needs_revision -> human_checkpoint
final_review needs_revision -> synthesis retry
```

Alternative accepted shape:

```text
parallel reviews -> findings ledger -> synthesis -> final review
review_* needs_revision -> rfc_revision_spec -> re-review
```

The workflow must make the choice explicit rather than relying on the absence
of a cycle to create a surprise checkpoint.

Acceptance:

- Root-review `needs_revision` either routes to a declared revision job or
  creates a named human checkpoint by design.
- The validation prompt states whether a root-review checkpoint is success,
  expected block, or failure.

## Implementation Order

1. R003: implement `why` support for blockers, artifacts, verdicts, runs, and
   sessions.
2. R002: enrich `status --json` with blockers, checkpoints, blocked downstream
   jobs, and next actions.
3. R001: add `evidence export`.
4. R004: add `submit-review`.
5. R006: clarify branch confirmation output.
6. R005: add adapter constraint declarations and advisory/enforced recording.
7. R007: revise the RFC 0014 validation fixture and prompt.

R003 and R002 come first because they improve recovery during every later
implementation step.

## Test Plan

Add or update tests under `agent-runner/tests/`:

- blocked review verdict creates a blocker and appears in `status --json`;
- `why <blocker_id>` returns blocker, verdict, job, and downstream dependency
  context;
- `why <artifact_id>` and `why <verdict_id>` resolve;
- `evidence export` writes a redacted Markdown artifact and omits state DB
  paths/transcripts;
- `submit-review` publishes an artifact, records a verdict, and applies
  downstream gate behavior;
- workflow validation accepts valid constraints and rejects unknown constraint
  values;
- branch confirmation JSON includes mismatch or records-only information;
- RFC 0014 fixture handles root-review `needs_revision` according to its
  declared policy.

Required commands:

```bash
cd agent-runner
PYTHONPATH=src ../.venv/bin/python -m pytest -q
PYTHONPATH=src python3 -m agent_runner.cli workflow validate examples/rfc-0014-operational-artifact-home/workflow.json --json
```

Use the Engram repository virtualenv while `agent-runner` is incubated here. If
the eventual standalone project has its own virtualenv or test runner, use that
project's documented command.

## Documentation Updates

Update:

- `agent-runner/docs/SPEC.md` after implementation changes land;
- `agent-runner/docs/UBIQUITOUS_LANGUAGE.md` for "evidence export",
  "adapter constraint", and "next action";
- `agent-runner/examples/rfc-0014-operational-artifact-home/workflow.json`;
- `agent-runner/prompts/P002_validate_agent_runner_with_rfc_0014.md`;
- validation notes for any rerun.

## Done Criteria

This spec is complete when:

- the new CLI behavior is implemented and tested;
- RFC 0014 dogfood can produce a committed evidence export without manual JSON
  assembly;
- status and why are enough to recover from the same checkpoint without reading
  SQLite manually;
- the RFC 0014 workflow fixture declares root-review revision behavior;
- tests pass;
- review confirms no regression to the local-first/no-transcripts default.
