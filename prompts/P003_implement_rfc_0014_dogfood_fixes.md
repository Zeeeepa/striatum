# P003: Implement RFC 0014 Dogfood Fixes

Status: ready
Date: 2026-05-06
Scope: `agent_runner` follow-up implementation
Primary spec: `agent-runner/docs/RFC_0014_DOGFOOD_FIX_SPEC.md`

## Mission

You are the `agent_runner` implementation coordinator. Your job is to implement
the follow-up fixes specified after the RFC 0014 validation dogfood run.

The goal is not to decide RFC 0014. The goal is to make `agent_runner` better
at recovering from blocked workflows, exporting durable run evidence, and
reducing review-gate command friction.

## Read First

Read these files in order:

1. `agent-runner/README.md`
2. `agent-runner/docs/SPEC.md`
3. `agent-runner/docs/RFC_0014_DOGFOOD_FIX_SPEC.md`
4. `agent-runner/docs/DECISION_LOG.md`
5. `agent-runner/docs/UBIQUITOUS_LANGUAGE.md`
6. `docs/reviews/rfc-0014-operational-artifact-home/AGENT_RUNNER_VALIDATION_NOTES.md`
7. `agent-runner/examples/rfc-0014-operational-artifact-home/workflow.json`
8. this prompt

Treat `RFC_0014_DOGFOOD_FIX_SPEC.md` as the implementation contract unless you
find a contradiction with the existing `agent_runner` spec. If you find a
contradiction, stop and record the blocker instead of silently changing product
direction.

## Branch Discipline

Work on the current `agent-runner/rfc-0014-validation` branch unless the human
explicitly redirects you. Before editing:

1. Run `git status --short --branch`.
2. If there are uncommitted changes you did not make, inspect them and preserve
   them.
3. Do not rewrite the validation artifacts except where this prompt or the spec
   explicitly asks for links, rerun notes, or fixture updates.

## Implementation Scope

Implement the spec requirements in this order:

1. R003: expanded `why` introspection.
2. R002: richer `status --json` recovery state.
3. R001: `evidence export`.
4. R004: `submit-review`.
5. R006: branch confirmation clarity.
6. R005: adapter constraint declarations.
7. R007: RFC 0014 workflow revision policy.

Keep changes scoped to `agent-runner/` unless a validation artifact or root
ignore file must be updated.

## Required Behavior

### Expanded `why`

`agent_runner why <id> --json` must support:

- run ids;
- job ids;
- queue message ids;
- blocker ids;
- artifact ids;
- verdict ids;
- session ids.

For blocker IDs, include the blocker, owning run/job/session, related verdict
when applicable, blocked downstream jobs, and deterministic next actions.

### Richer `status`

`agent_runner status --json` must keep existing aggregate fields and add:

- open blockers;
- human checkpoints;
- latest non-accepting review verdicts;
- claimable jobs grouped by role/lane;
- blocked downstream jobs and the reason they are blocked;
- `next_actions`.

The RFC 0014 blocked run shape should be understandable from `status --json`
without reading SQLite manually.

### Evidence Export

Add:

```bash
agent_runner evidence export --run-id <run_id> --path <repo_path> --json
```

The command writes a redacted Markdown run snapshot. It must include run, job,
blocker, verdict, artifact, status, doctor, and downstream-blocking evidence.
It must reject paths outside the repo and paths under `.agent_runner/`.

Do not commit `.agent_runner/`.

### Submit Review

Add:

```bash
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

It must publish the artifact, record the verdict, apply review-gate behavior,
and return artifact/verdict/blocker/downstream state in one response.

### Branch Confirmation

`branch confirm --json` must disclose that V1 records branch confirmation but
does not perform git branch switching by default. Include:

- `records_only`;
- requested branch;
- current git branch when detectable;
- mismatch warning if relevant.

### Adapter Constraints

Support optional workflow lane constraints:

```json
{
  "constraints": {
    "network": "forbidden",
    "transcripts": "off",
    "repo_scope": "local_only"
  }
}
```

Validate known values, expose them in work packets, and record enforcement as
`enforced`, `advisory`, or `unsupported`. For V1 process adapters, advisory is
acceptable if real enforcement is not possible.

### RFC 0014 Fixture Policy

Update the RFC 0014 validation fixture and prompt so root-review
`needs_revision` behavior is explicit. Either:

- route root-review `needs_revision` to a named human checkpoint by design, or
- add a declared revision job and re-review loop.

Do not leave the current behavior implicit.

## Documentation Updates

Update as implementation lands:

- `agent-runner/docs/SPEC.md`
- `agent-runner/docs/UBIQUITOUS_LANGUAGE.md`
- `agent-runner/docs/README.md` if commands or docs are added
- `agent-runner/examples/rfc-0014-operational-artifact-home/workflow.json`
- `agent-runner/prompts/P002_validate_agent_runner_with_rfc_0014.md`

Add a short implementation note under:

```text
agent-runner/docs/reviews/v1/
```

Use a filename like:

```text
RFC_0014_DOGFOOD_FIX_IMPLEMENTATION.md
```

Record what was implemented, what was deferred, and how it was verified.

## Tests

Add or update focused tests under `agent-runner/tests/`.

Required coverage:

- blocked review verdict appears in `status --json`;
- `why <blocker_id>` resolves with related verdict/job/downstream context;
- `why <artifact_id>` and `why <verdict_id>` resolve;
- `evidence export` writes redacted Markdown and rejects invalid paths;
- `submit-review` publishes artifact, records verdict, and applies gate
  behavior;
- workflow lane constraints validate and appear in work packets;
- `branch confirm --json` reports records-only and mismatch state;
- RFC 0014 fixture validates and has explicit root-review revision policy.

## Verification

Run from `agent-runner/`:

```bash
PYTHONPATH=src ../.venv/bin/python -m pytest -q
PYTHONPATH=src python3 -m agent_runner.cli workflow validate examples/rfc-0014-operational-artifact-home/workflow.json --json
```

Also run a small smoke sequence in a temporary repo that exercises:

- `init`;
- `run prepare`;
- `branch confirm`;
- `run start`;
- `register-session`;
- `claim-next`;
- `submit-review`;
- `status --json`;
- `why <blocker_id> --json` when a blocker is created;
- `evidence export`.

## Final Response Requirements

Report:

- files changed;
- commands added;
- tests run and results;
- fixture/prompt changes;
- deferred items, if any;
- whether the branch is ready for review.

Do not claim the fixes are complete unless the tests pass and the RFC 0014
fixture has an explicit root-review revision policy.
