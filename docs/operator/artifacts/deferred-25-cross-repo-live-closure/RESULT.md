---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["AGENTS.md", "docs/operator/BRIEF.md", "docs/TODO.md", "docs/ROADMAP.md", "docs/rfcs/0032-cross-repo-workflows-and-mcp-mutation-capabilities.md", "docs/rfcs/0035-multi-repo-test-harness-for-cross-repo-workflows.md", "docs/rfcs/0068-go-production-daemon-port.md", "docs/rfcs/README.md", "docs/SPEC.md", "docs/HOW_TO_HUMAN.md", "docs/architecture/COMMAND_AUTHORITY_MATRIX.md", "docs/architecture/CLI_RETIREMENT_PARITY.md", "src/striatum/cross_repo.py", "src/striatum/daemon_rpc/daemon_methods.json", "src/striatum/daemon_pg/handlers/run_lifecycle/run_prepare.py", "go/pkg/crossrepo/prepare.go", "go/pkg/crossrepo/lifecycle.go", "go/cmd/striatumd/main.go", "tests/test_workflow_cross_repo.py", "tests/test_cross_repo_lifecycle.py", "tests/test_cross_repo_prepare_e2e.py", "tests/test_cross_repo_lifecycle_e2e.py", "tests/test_cross_repo_crash_recovery_e2e.py", "tests/test_mcp_capability_scope_e2e.py", "tests/test_per_repo_write_scope_e2e.py", "tests/test_multi_repo_harness.py", "tests/test_cross_repo_pg_cancel.py", "tests/test_daemon_go_mutations.py"]
---

# Deferred 25 Cross-Repo Live Scheduling Closure
author: deferred25-cross-repo-codex-gpt-5-001
status: closed_new_rfc_needed
date: 2026-05-23

## Result

Deferred item 25 is closed as a classification and workflow-scaffold task.
No production source change is warranted in this slice. Full live cross-repo
scheduling is not already landed; it needs a new bounded RFC.

The current implementation is healthy for the landed slices. Striatum has
cross-repo workflow shape validation, daemon-owned PostgreSQL metadata,
MCP capability gating, multi-repo harness coverage, and production
cross-repo read/cancel routes. Those are not the same as a live scheduler
that prepares a cross-repo workflow through the production control plane,
fans jobs out to participant repositories, advances dependencies across
repository boundaries, and completes or recovers the aggregate run.

## Already Landed

- RFC 0032 workflow schema support: top-level `repositories`,
  `primary_repository`, per-job `repository`, per-repo parallelism,
  cross-repo cycle opt-in, and
  `reviewer_access_scope: "cross_repo_artifact_augmented"` validation.
- Daemon PostgreSQL metadata: `cross_repo_runs`,
  `cross_repo_run_repositories`, `cross_repo_cycle_counters`,
  `audit_repositories`, and `runs.cross_repo_run_id` exist under daemon-owned
  PostgreSQL.
- MCP capability model: `read`, `write`, `review`, `claim`, `apply`,
  `admin`, and `recovery` are registry capabilities; daemon MCP
  `tools/list` filters by capability and `tools/call` re-authorizes and
  audits denials.
- RFC 0035 harness: `tests/_harness/MultiRepoHarness` boots multiple
  registered target repositories against an ephemeral PostgreSQL daemon DB and
  exercises prepare/lifecycle helper paths, crash recovery, MCP capability
  scope, and per-repo write-scope behavior.
- Production cross-repo read/cancel: `cross_repo.list`,
  `cross_repo.describe`, `cross_repo.why`, and `cross_repo.cancel` are in the
  method registry and Go daemon handler map. Cancel calls the Go cross-repo
  lifecycle service and participant `run.cancel` path.
- Go default daemon alignment: RFC 0068 records Go as the production daemon,
  the Python daemon is retired, and `make daemon-go-conformance` includes the
  multi-repo harness lane with `CORE=go`.

## Helper-Level Or Harness-Only

- `src/striatum.cross_repo.prepare_cross_repo_run` and
  `start_cross_repo_run` exist, and Go has `crossrepo.PrepareRun` and
  `StartRun`, but the production daemon only registers cross-repo list,
  describe, why, and cancel handlers.
- The multi-repo harness creates participant runs through
  `PgParticipantRunner` and direct PostgreSQL helper calls. That proves table
  shape and lifecycle semantics, but it is not an operator-facing production
  scheduling route.
- Current `run.prepare` is repository-scoped. It accepts one
  `repository_id`, loads one workflow file from that repository, and inserts
  all run/job rows under that same repository. It does not split a
  cross-repo workflow into participant runs by job repository alias.
- Cross-repo cycle counter tests assert the global counter table shape, but
  they do not yet prove production verdict routing that automatically
  increments counters and requeues cross-repo jobs.

## Missing Live Scheduler Surface

A production live scheduler RFC should define and land these items together:

- Public control-plane semantics: either extend `run.prepare`/`run.start` for
  cross-repo workflows or add explicit `cross_repo.prepare` and
  `cross_repo.start` daemon methods, including capability requirements,
  method-registry entries, CLI/MCP/UI routes, and authority-matrix tests.
- Cross-repo prepare fan-out: validate repository registration and workflow
  aliases, create the aggregate cross-repo row, create participant
  `runs`/snapshots/jobs only for jobs targeting each repository, and preserve
  atomic rollback on participant failure.
- Cross-repo start and scheduling: enqueue roots per participant, enforce
  global and per-repo parallelism, and unblock downstream jobs when upstream
  jobs in another repository complete or produce an accepted verdict.
- Packet and session scope: specify coordinator sessions, lane matching,
  claim/await-packet behavior, artifact visibility, review access, and byline
  expectations across participant repositories.
- Cross-repo cycles: make verdict-driven cycle routing increment
  `cross_repo_cycle_counters` through the production scheduler, with global
  max-iteration enforcement and clear exhaustion behavior.
- Aggregate terminal semantics: define when the cross-repo run becomes
  completed, failed, blocked, canceled, or compromised based on participant
  states and audit/decision records.
- Recovery: cover daemon crash during prepare/start/cross-repo dependency
  transitions, participant repository removal or disappearance, stale leases,
  auto-finalize interaction, and cancel/retry repair paths.
- Operator surfaces: add enough dashboard/status/why/run-summary detail, and
  the missing cross-repo UI/cancel confirmation, so operators are not forced
  to infer aggregate state from per-repo rows.

## Bounded RFC Recommendation

Write a new RFC for "Cross-Repo Live Scheduler V1" rather than expanding RFC
0032 or RFC 0035 retroactively. Keep the V1 acceptance slice narrow:

- two registered local repositories only;
- one aggregate `cross_repo_run_id` with two participant `run_id`s;
- production prepare/start through daemon RPC and MCP;
- one cross-repo dependency from repo A to repo B;
- one review/accept gate crossing the repository boundary;
- one bounded cross-repo revision cycle;
- cancel, crash-reconcile, and participant-unreachable recovery;
- focused CLI/MCP/UI/docs/tests without hosted services, cloud APIs,
  telemetry, transcript capture, distributed transactions, or cross-machine
  semantics.

Non-goals for that RFC should explicitly exclude hosted Git providers,
multi-machine scheduling, cross-repo atomic filesystem mutation, sealed
cross-repo apply receipts, and external persistence.

## Status Fixes Made

- RFC 0032 now distinguishes the landed helper-level prepare/start/cancel
  progression from missing production live scheduler fan-out.
- RFC 0035 now says `accepted (V1)` and records that its shipped harness is
  developer test infrastructure, not the production live scheduler.

Shared TODO, roadmap, and operator brief files were not edited.

## Changed Files

- `docs/rfcs/0032-cross-repo-workflows-and-mcp-mutation-capabilities.md`
- `docs/rfcs/0035-multi-repo-test-harness-for-cross-repo-workflows.md`
- `docs/operator/plans/deferred-25-cross-repo-live-closure.md`
- `docs/operator/workflows/deferred-25-cross-repo-live-closure/workflow.json`
- `docs/operator/workflows/deferred-25-cross-repo-live-closure/prompts/classify_cross_repo_live_scheduler.md`
- `docs/operator/artifacts/deferred-25-cross-repo-live-closure/RESULT.md`

## Validation

Commands run for this closure:

- `PYTHONPATH=src python3 -m striatum.cli workflow validate docs/operator/workflows/deferred-25-cross-repo-live-closure/workflow.json --json`
  - Result: `{"data":{"valid":true,"workflow_id":"deferred-25-cross-repo-live-closure"},"ok":true}`.
- `PYTHONPATH=src python3 -m striatum.cli workflow plan docs/operator/workflows/deferred-25-cross-repo-live-closure/workflow.json --json`
  - Result: valid plan; 1 job, 0 edges, 0 cycles, 1 claim step.
- `PYTHONPATH=src python3 -m striatum.cli workflow lint docs/operator/workflows/deferred-25-cross-repo-live-closure/workflow.json --json`
  - Result: `valid: true`, `warning_count: 0`, coverage level `strong`.
- `PYTHONPATH=src .venv/bin/python -m pytest -q tests/test_workflow_cross_repo.py tests/test_cross_repo_lifecycle.py tests/test_mcp_mutation_capabilities.py::test_mcp_tools_call_exact_workflow_control_methods_route_to_daemon_rpc`
  - Result: 40 passed in 0.07s.
- `(cd go && go test ./pkg/crossrepo)`
  - Result: `ok github.com/halbritt/striatum/go/pkg/crossrepo (cached)`.
- `make test-multi-repo`
  - Result: 46 passed in 86.42s.
- `PYTHONPATH=src python3 - <<'PY' ... validate_artifact_front_matter(...)`
  - Result: `front matter valid`.
- `git diff --check -- docs/rfcs/0032-cross-repo-workflows-and-mcp-mutation-capabilities.md docs/rfcs/0035-multi-repo-test-harness-for-cross-repo-workflows.md docs/operator/plans/deferred-25-cross-repo-live-closure.md docs/operator/workflows/deferred-25-cross-repo-live-closure docs/operator/artifacts/deferred-25-cross-repo-live-closure`
  - Result: passed.
