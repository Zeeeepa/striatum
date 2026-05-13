author: designer-unknown-model-001

# Design — RFC 0046 V1 lane evidence guard at `publish-artifact`

Closes GH #2 / GH #5 by gating model-byline artifact publishes on the
existence of a `process_executions` row whose observed outputs cover
the artifact path. The byline layer already differentiates on
attestation; the new layer asks the orthogonal question, "did the
attested lane process actually emit this file?"

Two source-of-truth deltas vs. the RFC text the implementer should
honor when this design is reviewed:

1. `process_executions` does **not** currently have
   `observed_output_paths_json` / `declared_output_paths_json`. The
   RFC quotes them as if they exist; they need to be added by this
   RFC and populated. Migration `_apply_v8_process_state_enum` is
   the most recent change to that table and it did not introduce
   either column.
2. There is no `src/striatum/events.py` registry — event types
   are bare string literals at the `insert_event(event_type=...)`
   call site (see `src/striatum/db.py::insert_event`). F-event is
   therefore "use a stable literal," not "register in a table."

## Files to touch

- `src/striatum/schema.py` — extend the V1 baseline `artifacts`
  and `process_executions` definitions so freshly initialised
  databases install both columns directly (matches the idempotency
  convention used by v9/v10/v11/v12).
- `src/striatum/migrations.py` — two new migrations, v15 and v16
  (see below).
- `src/striatum/daemon_pg/sql/0005_repo_local_workflow_state.sql`
  — add the same columns to `striatumd.artifacts` and
  `striatumd.process_executions`. Postgres path uses idempotent
  `ADD COLUMN IF NOT EXISTS` to match the file's existing pattern.
- `src/striatum/process_adapter.py` — populate
  `observed_output_paths_json` at process exit. See producer
  sketch below.
- `src/striatum/supervisor.py` — write a `process_executions`
  row per delivered packet so the supervised path produces the
  same evidence shape as the one-shot adapter path. Without this,
  the guard refuses every supervised artifact and the override
  flag becomes mandatory (the regression GH #2 already names).
- `src/striatum/artifacts.py::publish_artifact` — guard call site
  + override rationale column write.
- `src/striatum/cli/parser.py` — two new flags on the
  `publish-artifact` subparser.
- `src/striatum/cli/dispatch.py::_resolve_publish_defaults`
  (publish branch above it) — thread the new flag pair through
  to `publish_artifact`.
- `tests/test_lane_evidence_guard.py` — new file, four scenarios.
- `CHANGELOG.md` — v1.43.0 entry naming GH #2 / GH #5.

## SQL migration shape

`_apply_v15_artifact_override_rationale` (forward-only `ADD
COLUMN`, idempotent against the V1 baseline already containing the
column):

```python
def _apply_v15_artifact_override_rationale(conn):
    cols = [r[1] for r in conn.execute("PRAGMA table_info(artifacts)").fetchall()]
    if "attestation_override_rationale" not in cols:
        conn.execute(
            "ALTER TABLE artifacts ADD COLUMN attestation_override_rationale TEXT"
        )
```

`_apply_v16_process_observed_output_paths` (forward-only `ADD
COLUMN` with `DEFAULT '[]'` so existing rows read as the empty
list, not NULL):

```python
def _apply_v16_process_observed_output_paths(conn):
    cols = [r[1] for r in conn.execute("PRAGMA table_info(process_executions)").fetchall()]
    if "observed_output_paths_json" not in cols:
        conn.execute(
            "ALTER TABLE process_executions "
            "ADD COLUMN observed_output_paths_json TEXT NOT NULL DEFAULT '[]'"
        )
```

Postgres path: `ALTER TABLE striatumd.artifacts ADD COLUMN IF NOT
EXISTS attestation_override_rationale text;` and `ALTER TABLE
striatumd.process_executions ADD COLUMN IF NOT EXISTS
observed_output_paths_json jsonb NOT NULL DEFAULT '[]'::jsonb;`.

## Producer: observed output paths

`process_adapter.mark_process_exited` already runs inside a
transaction with the started_at/ended_at pair. Extend it to walk
`job.write_scope.allowed_paths` and capture the repo-relative
path of every file whose mtime falls within
`[started_at, ended_at]`. Write the list to
`observed_output_paths_json`. Bound the walk to allowed paths so
unrelated edits in the repo don't pollute evidence.

`supervisor.deliver_packet_to_attached_supervisor` is the second
producer: it currently emits `supervisor.packet_delivered` but no
`process_executions` row. Insert one in state `running` at
delivery, then close it (state `exited`, observed paths captured)
when the supervisor receives the next claim/heartbeat from the
same session — or when the operator calls `complete`. The narrow
V1.7 implementation can close at `complete`/`publish-artifact`
boundaries; tightening to live heartbeat watching is V1.8 work.

This is the missing half of the forgery story: today an operator
can `supervise start` a session, the wrapper exits, and they
publish on its behalf without any `process_executions` row to
miss. Without this producer the guard refuses everything and the
override flag becomes routine — which is exactly the regression
GH #2 documents.

## Guard sketch

In `publish_artifact`, after `validate_optional_markdown_author_line`
and `validate_artifact_front_matter`:

```python
expected = expected_author_line(conn, job=job, session_id=session_id)
if _is_model_byline(expected) and not allow_no_process_execution:
    rows = conn.execute(
        "SELECT observed_output_paths_json FROM process_executions "
        "WHERE session_id = ? AND state = 'exited'",
        (session_id,),
    ).fetchall()
    if not any(path_text in json_loads(r["observed_output_paths_json"] or "[]") for r in rows):
        raise ArtifactError(
            f"lane_evidence_missing: artifact path {path_text} not present in any "
            f"process_executions row for session {session_id}; pass "
            "--allow-no-process-execution to override with an operator rationale."
        )
```

`_is_model_byline` parses the operator template (`author: operator`
or `author: operator [self-declared: …]`) and returns False for
those, True otherwise. Sharing the templates with
`identity.operator_author_line` keeps the predicate single-sourced.

Override branch: when `--allow-no-process-execution` is passed,
require a non-empty `--override-rationale` (otherwise exit 2 via
argparse). Store the rationale in
`artifacts.attestation_override_rationale` and emit:

```python
insert_event(
    conn,
    run_id=str(job["run_id"]),
    event_type="provenance.publish_without_process_execution",
    actor_session_id=session_id,
    job_id=job_id,
    artifact_id=artifact_id,
    lease_id=lease_id,
    payload={
        "artifact_id": artifact_id,
        "session_id": session_id,
        "byline": expected,
        "expected_path": path_text,
        "rationale": override_rationale,
    },
)
```

## CLI surface

`parser.py` (additions to the existing `publish-artifact` subparser):

```python
publish.add_argument("--allow-no-process-execution", action="store_true")
publish.add_argument("--override-rationale", default=None)
```

`dispatch.py` (publish branch + `_resolve_publish_defaults` signature):
thread the new pair through `publish_artifact(..., allow_no_process_execution=..., override_rationale=...)`.
`_resolve_publish_defaults` itself is unaffected — it only resolves
kind/logical_name. The override pair is a direct keyword on the
existing call.

## Tests — `tests/test_lane_evidence_guard.py`

Four scenarios, each a function:

- `test_publish_succeeds_when_process_execution_covers_path` —
  session with a `process_executions` row whose
  `observed_output_paths_json` contains the artifact path → publish
  succeeds, model byline lands.
- `test_publish_refuses_when_no_process_execution_matches` —
  session without any covering row, model byline → publish raises
  `ArtifactError` whose message contains `lane_evidence_missing`;
  CLI dispatch exits 6.
- `test_override_requires_rationale` — `--allow-no-process-execution`
  without `--override-rationale` exits 2 (argparse-level refusal
  via a `parser.error` call when only one of the pair is set).
- `test_override_with_rationale_records_event_and_column` —
  publish succeeds, `attestation_override_rationale` column reads
  back the rationale, and an
  `events.event_type = 'provenance.publish_without_process_execution'`
  row is present with the named payload keys.

A fifth case is named in the RFC acceptance and worth keeping:
`test_operator_byline_bypasses_guard` — when
`expected_author_line` is `author: operator` (unattested or
operator-labelled session), publish passes through without
consulting `process_executions`.

Word count: ~1100 words. The implementer pass should treat the
two source-of-truth deltas (missing columns, no events registry)
as binding constraints and update the RFC text alongside the
build if appropriate.
