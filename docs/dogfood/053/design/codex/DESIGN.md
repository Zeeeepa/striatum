author: designer-unknown-model-001

# Design — RFC 0046 V1 Publish-Time Lane Evidence Guard

## Position

RFC 0046 should land as a narrow publish-time provenance guard, not as a
rewrite of lane attestation. `identity.py::artifact_author_identity` already
answers "may this session claim a model byline right now?" from lane-liveness
attestation. The missing question is "did a runner-recorded lane process
observe this artifact path?" That belongs in `publish_artifact`, after the
file, scope, byline, and front-matter checks have made the artifact concrete
but before the append-only artifact row is inserted.

There is one implementation caveat in this checkout: the current
`process_executions` schema in `schema.py` and `process_adapter.py` does not
yet contain `declared_output_paths_json` or `observed_output_paths_json`,
although RFC 0046 describes them as existing. The V1 build must either add
those columns in the same migration as RFC 0046 or first reconcile the RFC
against the real schema. I recommend adding them in the same migration,
defaulting both to JSON `[]`, because the guard cannot be implemented
honestly without a durable observed-output field.

## Files To Touch

- `src/striatum/schema.py`
- `src/striatum/migrations.py`
- `src/striatum/process_adapter.py`
- `src/striatum/daemon_pg/sql/0005_repo_local_workflow_state.sql` and the
  next daemon migration registered under `src/striatum/daemon_pg/`
- `src/striatum/daemon_pg/repo_local_migration.py`
- `src/striatum/artifacts.py`
- `src/striatum/cli/parser.py`
- `src/striatum/cli/dispatch.py`
- `tests/test_lane_evidence_guard.py`
- migration/daemon SQL tests that assert the new columns are copied and
  visible in fresh schemas

## Schema

Add SQLite migration v15:

```python
def _apply_v15_lane_evidence_guard(conn: sqlite3.Connection) -> None:
    conn.execute(
        "ALTER TABLE artifacts "
        "ADD COLUMN attestation_override_rationale TEXT"
    )
    conn.execute(
        "ALTER TABLE process_executions "
        "ADD COLUMN declared_output_paths_json TEXT NOT NULL DEFAULT '[]'"
    )
    conn.execute(
        "ALTER TABLE process_executions "
        "ADD COLUMN observed_output_paths_json TEXT NOT NULL DEFAULT '[]'"
    )
```

If a preceding branch has already added the process-output columns, make the
column adds idempotent by inspecting `PRAGMA table_info`; otherwise a plain
forward migration is consistent with existing migration style. Update
`SCHEMA_SQL` so fresh repos match migrated repos.

For Postgres, add a daemon migration after current v5:

```sql
ALTER TABLE striatumd.artifacts
  ADD COLUMN attestation_override_rationale text;
ALTER TABLE striatumd.process_executions
  ADD COLUMN declared_output_paths_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN observed_output_paths_json jsonb NOT NULL DEFAULT '[]'::jsonb;
```

Also update the repo-local workflow-state baseline SQL if this release
expects fresh daemon schemas to include the columns without replaying a
delta. Extend `repo_local_migration.py` `TableSpec("artifacts", ...)` with
`attestation_override_rationale`, and `TableSpec("process_executions", ...)`
with the two output-path columns. That keeps reanchor manifests honest: an
override rationale is append-only provenance and must be part of the copied
artifact row.

`process_adapter.py` should set `declared_output_paths_json` at process row
creation from `jobs.expected_artifacts_json` paths. On process exit,
`mark_process_exited` should compute `observed_output_paths_json` by checking
which declared paths exist under the relevant repository/worktree root, then
persist that JSON before emitting `process.exited`. This keeps artifact
observation metadata deterministic and privacy-safe: paths only, no stdout,
stderr, transcript, or file contents.

## Publish Guard

Extend `publish_artifact` signature:

```python
def publish_artifact(..., allow_no_process_execution: bool = False,
                     override_rationale: str | None = None) -> dict[str, object]:
```

After `validate_optional_markdown_author_line(...)`, compute:

```python
expected_byline = expected_author_line(conn, job=job, session_id=session_id)
override = _lane_evidence_override(
    conn,
    session_id=session_id,
    expected_byline=expected_byline,
    path_text=path_text,
    allow_no_process_execution=allow_no_process_execution,
    override_rationale=override_rationale,
)
```

The helper should pass through for `author: operator` and
`author: operator [self-declared: ...]`. For model bylines, query completed
process rows for the same `session_id` and parse `observed_output_paths_json`
with `json_loads`; any row containing exact repo-relative `path_text` passes.
Use the real state enum in this codebase: current rows use `state = 'exited'`
for a completed process, while RFC 0046 says `completed`. The build should
either accept `exited` or first rename the process state contract. I recommend
checking `state = 'exited' AND exit_code = 0`; `failed`, `lost`, and
`timed_out` must not count.

Missing evidence without override raises:

```text
lane_evidence_missing: artifact path '<path>' not present in any
process_executions observed output row for session <session_id>; pass
--allow-no-process-execution --override-rationale '<reason>' to override.
```

If `allow_no_process_execution` is true, require
`override_rationale.strip()` or raise `ArtifactError` from direct API calls.
The CLI should also fail earlier through argparse by making
`--override-rationale` conditionally required in dispatch when the allow flag
is present; argparse cannot express that directly, so dispatch should raise
`StriatumError(exit_code=2)` before opening the DB mutation path.

Insert `attestation_override_rationale` with the artifact row, `NULL` for the
normal path and the stripped rationale for the override path. Immediately
after `artifact.published`, emit:

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
        "byline": expected_byline,
        "expected_path": path_text,
        "rationale": rationale,
    },
)
```

This repository does not have a separate `events.py` registry; event types are
currently open strings inserted through `db.insert_event`. No registry update
is needed unless another branch has introduced one.

## CLI Surface

`parser.py` adds:

```python
publish.add_argument("--allow-no-process-execution", action="store_true")
publish.add_argument("--override-rationale")
```

`dispatch.py` keeps `_resolve_publish_defaults` unchanged except for no
longer treating override flags as defaulting inputs. In the
`publish-artifact` branch, validate the flag pair, then pass both values into
`publish_artifact`.

## Acceptance Tests

Create `tests/test_lane_evidence_guard.py` with focused CLI-level cases:

- `test_model_byline_with_observed_process_output_publishes`: arrange an
  attested session, matching byline, and `process_executions` row whose
  `observed_output_paths_json` contains the expected artifact path.
- `test_model_byline_without_observed_process_output_refuses`: same byline,
  no matching process evidence; assert exit code 6 and
  `lane_evidence_missing`.
- `test_allow_no_process_execution_requires_rationale`: pass only the allow
  flag; assert exit code 2.
- `test_override_records_rationale_and_provenance_event`: pass both flags;
  assert artifact row stores the rationale and the
  `provenance.publish_without_process_execution` payload includes
  `artifact_id`, `session_id`, `byline`, `expected_path`, and `rationale`.
- `test_operator_byline_skips_lane_evidence_guard`: unattested/operator
  byline publishes without process evidence and without override.

Add migration coverage proving v15 upgrades older SQLite files and fresh init
sets `LATEST_VERSION`, plus daemon SQL coverage proving the Postgres
artifact/process columns exist. Verification target: `make lint`,
`make typecheck`, and `make test -m "not multi_repo"`; run targeted
multi-repo migration tests separately when Postgres is available.
