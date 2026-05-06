# V1 MVP Build Review

Date: 2026-05-06
Reviewer: parent Codex coordinator
Verdict: accept_with_findings

## Scope Reviewed

- `pyproject.toml`
- `Makefile`
- `.gitignore`
- `src/agent_runner/`
- `examples/rfc-ledger-cleanup/`
- `tests/test_cli_mvp.py`
- `docs/SPEC.md`
- `docs/DECISION_LOG.md`
- `docs/UBIQUITOUS_LANGUAGE.md`

## Checklist

| Check | Result |
|-------|--------|
| Accepted decisions are honored | pass |
| SQLite schema supports queue, events, leases, sessions, artifacts, verdicts | pass |
| CLI commands enforce state transitions | pass after fixes |
| Agents use CLI mutations, not direct SQLite writes | pass |
| Workflow config is JSON and YAML is rejected | pass |
| Transcripts are not captured by default | pass |
| Persistent/fresh session policy is represented | pass |
| Tests cover core behavior | pass |
| Generated artifacts are idempotent where practical | pass for duplicate artifact hash/logical name |

## Findings

### B-F001 P1: `complete` allowed terminal transition before `ack`

Affected file: `src/agent_runner/db.py`

The first implementation allowed `complete` from `claimed` as well as
`running`, which contradicted the design's invalid-transition rule.

Disposition: accepted and fixed. `complete` now requires `running`, and
`tests/test_cli_mvp.py::test_complete_requires_ack` covers the behavior.

### B-F002 P1: `verdict` was not atomic with completion

Affected file: `src/agent_runner/cli.py`

The first implementation inserted a verdict in one transaction and then called
`complete_job` in a second transaction. If completion failed, the verdict could
remain while the job was still incomplete.

Disposition: accepted and fixed. `verdict` now verifies required artifacts,
records verdict, completes the job, releases the lease, emits events, and
enqueues downstream work in one transaction.

### B-F003 P1: Session registration accepted unknown roles and lanes

Affected file: `src/agent_runner/cli.py`

`register-session` initially accepted any role/lane strings. That would allow
sessions that can never claim valid workflow work and weakens auditability.

Disposition: accepted and fixed. Registration now validates role/lane against
the run's workflow snapshot, with test coverage.

### B-F004 P2: Generated package metadata could be accidentally committed

Affected files: `.gitignore`, generated `src/agent_runner.egg-info/`

Editable install generated package metadata under `src/`. The project needed
an ignore rule to avoid committing local build output.

Disposition: accepted and fixed. `*.egg-info/` is ignored and generated files
were removed from the working tree.

## Non-Blocking Gaps

- V1 defines the process/tmux adapter boundary but does not yet launch model
  CLIs.
- `command_requests` exists for future idempotency but the CLI does not yet
  expose `--request-id`.
- `branch confirm` records human confirmation and gates claims, but does not
  perform `git switch` in this MVP.
- `doctor` performs basic consistency checks; richer artifact hash and branch
  drift checks remain future work.

## Verification

- `make test`: passed, 9 tests.
- CLI smoke in a temporary directory: `agent_runner init`, `status --json`,
  and `doctor --json` all passed.

