# RFC 0014 Dogfood Fix Implementation

Status: implementation note
Date: 2026-05-06

## Implemented

- Expanded `why <id> --json` to resolve runs, jobs, queue messages, blockers,
  artifacts, verdicts, and sessions.
- Enriched `status --json` with open blockers, human checkpoints,
  non-accepting verdicts, claimable jobs, blocked downstream jobs, and
  deterministic next actions.
- Added `agent_runner evidence export` for redacted Markdown run snapshots that
  can be committed without the ignored `.agent_runner/` SQLite database.
- Added `agent_runner submit-review` to publish a review artifact, record its
  verdict, and apply review-gate behavior in one command.
- Clarified `branch confirm --json` output with records-only, requested branch,
  detected current branch, and mismatch warning fields.
- Added lane adapter constraints to workflow validation and work packets.
- Updated the RFC 0014 validation workflow to declare root-review
  `needs_revision` as an explicit human checkpoint policy.

## Deferred

- The V1 process adapter records constraint enforcement but does not sandbox
  network or repository access for arbitrary model CLIs.
- `branch confirm` still does not perform automatic git switching.
- Evidence export is a curated Markdown snapshot; live coordination state
  remains SQLite under `.agent_runner/`.

## Verification

Verified after implementation:

```bash
cd agent-runner
PYTHONPATH=src ../.venv/bin/python -m pytest -q
PYTHONPATH=src python3 -m agent_runner.cli workflow validate examples/rfc-0014-operational-artifact-home/workflow.json --json
```

Results:

- `25 passed`
- RFC 0014 fixture validation returned `valid: true`.
- A temporary-repo smoke sequence covered init, prepare, branch confirmation,
  run start, session registration, claiming, `submit-review`, `status --json`,
  `why <blocker_id> --json`, and `evidence export`.
