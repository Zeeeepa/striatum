---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
inputs: ["docs/dogfood/060/DESIGN_SYNTHESIS.md", "docs/dogfood/060/review/design/REVIEW.md"]
---
author: implementer-unknown-model-001

# RFC 0048 Phase C Read Handler Handoff

## Summary

Implemented the synthesis-locked read surface under
`src/striatum/daemon_pg/handlers/reads/` and wired package import through
`src/striatum/daemon_pg/handlers/__init__.py`. All 12 locked RPC methods
register through `@register_pg_handler("<method>", read_only=True)` call
sites and expose `handle(ctx, params) -> dict`.

Because `src/striatum/daemon_pg/handlers/registry.py` was outside this
work packet's write scope, the `read_only=True` metadata is preserved by a
reads-local compatibility wrapper in `reads/_registry.py`; the shared
registry still stores only method -> handler. No router or daemon-RPC files
were edited.

## Method Table

| RPC method | Handler | Test | Shape/parity status | Notes |
|---|---|---|---|---|
| `status` | `reads/status.py::handle` | `test_status.py`, `test_registration.py` | Top-level legacy keys asserted; PG parity tests are PG-gated | Query predicates include `repository_id = %s`; returns empty legacy shape on no rows. |
| `dashboard` | `reads/dashboard.py::handle` | `test_dashboard.py`, `test_registration.py` | Top-level contract and error handling asserted | Unknown/missing run ids return RPC errors; reads events/verdict/node-state projections. |
| `list.runs` | `reads/list_runs.py::handle` | `test_list_runs.py`, `test_list_read_handlers.py` | `{items,count}` parity shape; PG scoped test gated | Supports `state` and `limit`. |
| `list.sessions` | `reads/list_sessions.py::handle` | `test_list_sessions.py`, `test_list_read_handlers.py` | `{items,count}` parity shape; PG scoped test gated | Supports `run_id`, `state`, `role`, `lane`; parses capabilities and lane-attestation summary. |
| `list.jobs` | `reads/list_jobs.py::handle` | `test_list_jobs.py`, `test_list_read_handlers.py` | `{items,count}` parity shape; PG scoped test gated | Supports `run_id`, `state`, `workflow_job_id`; includes latest verdict for review jobs. |
| `list.artifacts` | `reads/list_artifacts.py::handle` | `test_list_artifacts.py`, `test_list_read_handlers.py` | `{items,count}` parity shape; PG scoped test gated | Supports `run_id`, `kind`; author identity projected from job/session/artifact rows. |
| `list.workflows` | `reads/list_workflows.py::handle` | `test_list_workflows.py`, `test_list_read_handlers.py` | `{items,count}` parity shape; PG scoped test gated | Supports `limit`; scoped to `ctx.repository_id`. |
| `run.summary` | `reads/run_summary.py::handle` | `test_run_summary.py`, `test_registration.py` | Response envelope and rendered-file path/digest shape asserted | Writes requested Markdown file only; does not append workflow events in Phase C. |
| `why` | `reads/why.py::handle` | `test_why.py`, `test_registration.py` | Branch top-level shapes implemented; schema-invalid test asserted | Resolves run/job/message/blocker/artifact/verdict/session/process targets. |
| `doctor` | `reads/doctor.py::handle` | `test_doctor.py`, `test_registration.py` | `ok/schema_version/problems` shape asserted | Implements clean baseline plus a focused missing-artifact check; full legacy doctor parity remains PG-gated follow-up depth. |
| `evidence.export` | `reads/evidence_export.py::handle` | `test_evidence_export.py`, `test_registration.py` | Response envelope and rendered-file path/digest shape asserted | Uses legacy redactor/renderer; does not append `evidence.exported`. Imported after Track B so this read handler wins registration. |
| `corpus.export` | `reads/corpus_export.py::handle` | `test_corpus_export.py`, `test_registration.py` | Bundle envelope shape asserted; PG export smoke is path/error focused | Exports filesystem corpus rows plus PG event/run-summary rows without SQLite fallback. |

## Shared Helpers

- `reads/_sql.py` owns param validation, JSON/timestamp normalization, path
  checks, and output writes.
- `reads/_read_model.py` owns status, dashboard, evidence, artifact,
  session, event, and run-summary projections.
- `reads/__init__.py` imports each method module for decorator side effects.

Every workflow-table SQL query added in this package keeps `repository_id`
visible in the statement text. Joins include repository matching predicates.

## Tests

Added `tests/daemon_pg/handlers/reads/` with focused registration,
signature, schema-invalid, empty-shape, path/error, and PG-gated scoped
fixture tests. The PG-gated tests exercise repository scoping for the list
handlers when a reachable Postgres test URL is available.

Verification run:

```bash
pytest tests/daemon_pg/handlers/reads -q
# 62 passed, 5 skipped

ruff check src/striatum/daemon_pg/handlers/reads tests/daemon_pg/handlers/reads src/striatum/daemon_pg/handlers/__init__.py
# All checks passed

python -m compileall -q src/striatum/daemon_pg/handlers/reads tests/daemon_pg/handlers/reads
# pass
```

The 5 skips are the existing multi-repo/Postgres-gated tests in
`test_list_read_handlers.py`; this environment did not expose a live PG test
URL for those cases.

## Behavior Deltas / Deferred Items

- No locked methods were intentionally deferred.
- Full byte-for-byte parity against the SQLite legacy functions is still not
  claimed for every renderer byte, but post-review fixes replaced the
  shape-only gaps that blocked operator use: `list.*` filters now reach the
  daemon RPC params, `corpus.export` routes to the PG handler, status uses the
  legacy `next_actions` vocabulary, and run-summary / evidence exports embed
  the real PG doctor payload instead of a hardcoded clean result.
- The reads-local `read_only=True` wrapper now preserves metadata on handler
  callables for tests and review introspection. It remains intentionally thin:
  read discipline is still enforced by handler SQL and tests, not by a new
  router mechanism.

## Post-Review Fixes

After the first build review returned `needs_revision`, the operator applied
the missing completion deltas directly:

- `src/striatum/cli/daemon_rpc_route.py` now preserves list filters
  (`run_id`, `state`, `role`, `lane`, `workflow_job_id`, `kind`, `limit`) and
  maps `striatum corpus export` to `corpus.export`.
- `src/striatum/corpus/redaction.py` now fail-closes run-summary corpus rows
  for artifact/session fields before rendering, including session
  `close_reason` / `non_fresh_reason` and artifact author/prose siblings.
- `src/striatum/daemon_pg/handlers/reads/_read_model.py` now mirrors the
  legacy status action names (`claim_available_work`,
  `inspect_packet_with_inbox`, `recovery_process_reconcile`, etc.), uses
  deterministic run ordering, and removes the no-op artifact assignment.
- `run.summary` and `evidence.export` now call the PG doctor projection; they
  no longer hardcode `{"ok": true}`.
- Focused regression tests cover CLI translator params, corpus redaction
  before Markdown rendering, status action vocabulary, doctor reuse in
  exports, read-only handler metadata, and the existing PG-gated repository
  scoping checks.

Verification after the fixes:

```bash
.venv/bin/pytest tests/daemon_pg/handlers/reads tests/test_cli_daemon_rpc_route.py tests/test_corpus_redaction.py -q
# 83 passed, 5 skipped

.venv/bin/ruff check src/striatum/cli/daemon_rpc_route.py src/striatum/corpus/redaction.py src/striatum/daemon_pg/handlers/reads tests/test_cli_daemon_rpc_route.py tests/test_corpus_redaction.py tests/daemon_pg/handlers/reads
# All checks passed
```
