---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["AGENTS.md", "docs/operator/BRIEF.md", "docs/TODO.md", "docs/ROADMAP.md", "docs/rfcs/0053-human-principal-and-terminology-truing.md", "docs/SPEC.md", "src/striatum/workflow.py", "src/striatum/cli/workflow.py", "src/striatum/daemon_pg/sql/0005_repo_local_workflow_state.sql", "go/pkg/db/sql/0005_repo_local_workflow_state.sql", "go/pkg/workflowauthoring/workflow.go", "go/pkg/mutations/run.go", "tests/test_workflow_upgrade.py", "go/pkg/workflowgenerate/generate_test.go"]
---

# RFC 0053 Phase B Surface Map
author: deferred16-surface-codex-gpt-5-001

## Current Authority

RFC 0053 and D103 accept the product model: the AI operator drives routine
workflow work and the human principal resolves escalations only. Phase A
escalation artifacts and inbox surfaces have landed. Phase B remains deferred:
rename the workflow vocabulary from `human_checkpoint` to
`escalation_checkpoint` and rename `waiting_human` to the principal-facing
replacement named in RFC 0053 as `waiting_principal`.

`docs/TODO.md` item 44 and `docs/ROADMAP.md` section 5.8 both say Phase B is
blocked on scheduling the workflow schema version bump and pairing it with a
`workflow upgrade` rule.

## Schema And Authoring Surface

- Python workflow validation accepts only `striatum.workflow.v1` and
  `striatum.workflow.v1.1` in `src/striatum/workflow.py`.
- Go workflow authoring accepts the same two versions in
  `go/pkg/workflowauthoring/workflow.go`.
- Go run preparation accepts the same two versions in
  `go/pkg/mutations/run.go`.
- `review_revision_policy.root_review_needs_revision` accepts
  `human_checkpoint` or `declared_cycle`; no `escalation_checkpoint` value is
  accepted yet.
- `workflow upgrade` currently covers harness-profile defaults and
  `--add-phases`; it has no RFC 0053 rename mode.
- The generator and template catalogs still expose a `human_checkpoint`
  shape/block vocabulary in Python and Go.

## Runtime State Surface

The current PostgreSQL schema migration that creates repository workflow state
still names:

- job type `human_checkpoint`;
- job state `waiting_human`;
- queue message kind `human_checkpoint`;
- blocker severity `human_checkpoint`.

The task phrase "waiting_human run state rename" is imprecise against current
source: `waiting_human` is a job state in the repository workflow schema and
read models, not a value in `runs.state`.

## Production Runtime Surface

The production Go daemon and remaining Python client/test handlers still write
and read the old vocabulary:

- `record_verdict` opens a `human_checkpoint` blocker and sets the job to
  `waiting_human`.
- `work.block` maps severity `human_checkpoint` to job state
  `waiting_human`.
- `checkpoint resolve` requires a `human_checkpoint` blocker and a
  `waiting_human` job.
- read models expose `human_checkpoints`, `resolve_human_checkpoint`, and
  dashboard color/state keys for `waiting_human`.

## Test Surface

Versioning coverage is split across Python and Go:

- `tests/test_workflow_upgrade.py` covers dry-run/apply behavior, the
  PostgreSQL running-run guard, and the v1 to v1.1 `--add-phases` rewrite.
- `tests/test_workflow_field_errors.py` covers schema-version error paths and
  field-path reporting.
- `tests/test_workflow_phases.py` covers v1/v1.1 phase rules.
- `tests/test_workflow_generator.py` covers generated shape validation,
  including the current `human_checkpoint` shape.
- `go/pkg/workflowauthoring/workflow_test.go` and
  `go/pkg/workflowgenerate/generate_test.go` mirror authoring/generator and
  upgrade coverage on the Go side.

No safe Phase B change can update only docs or only the workflow validator;
the live database constraints, Go daemon mutations, Python compatibility
handlers, generator/catalog output, read models, and tests all need to move
together or through a deliberate compatibility window.
