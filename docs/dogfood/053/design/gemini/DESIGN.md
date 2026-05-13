# Design — RFC 0046 V1 Lane Evidence Guard

## Status
**Proposed** (Dogfood 053)

## Goals
- Close the trust gap where an operator can forge a model byline for an artifact not produced by the lane supervisor.
- Implement the "Lane Evidence Guard" in `publish-artifact`.
- Provide an auditable override path for legitimate operator-on-behalf flows.

## Design

### 1. Schema Migration (F-schema)

We need to add a column to store the operator's rationale when overriding the evidence guard.

**Migration (v15) in `src/striatum/migrations.py`:**
```python
def _apply_v15_attestation_override_rationale(conn: sqlite3.Connection) -> None:
    """RFC 0046: add attestation_override_rationale column to artifacts."""
    cols = [row[1] for row in conn.execute("PRAGMA table_info(artifacts)").fetchall()]
    if "attestation_override_rationale" not in cols:
        conn.execute("ALTER TABLE artifacts ADD COLUMN attestation_override_rationale TEXT")
```

**Postgres side (`src/striatum/daemon_pg/sql/0006_rfc0046_evidence_guard.sql`):**
```sql
ALTER TABLE striatumd.artifacts ADD COLUMN attestation_override_rationale TEXT;
```

### 2. Evidence Guard in `publish_artifact` (F-guard)

The guard will live in `src/striatum/artifacts.py::publish_artifact`.

**Logic Sketch:**
1. After byline validation and computing `expected_author_line`.
2. Check if the byline is a model byline (e.g., `designer-gemini-1`).
3. If it's a model byline, query `process_executions` for the session.
4. Verify that the artifact's repo-relative path is present in `observed_output_paths_json` for at least one `completed` process.
5. If missing and no override is provided, raise `ArtifactError` (exit code 6).

**Code Sketch in `src/striatum/artifacts.py`:**
```python
def _enforce_lane_evidence_guard(
    conn: sqlite3.Connection,
    *,
    session_id: str,
    path_text: str,
    byline: str,
    allow_override: bool,
) -> None:
    # 1. Skip if it's an operator byline
    if byline.startswith("author: operator"):
        return

    # 2. Check for evidence in process_executions
    rows = conn.execute(
        """
        SELECT observed_output_paths_json FROM process_executions
        WHERE session_id = ? AND state = 'completed'
        """,
        (session_id,),
    ).fetchall()

    for row in rows:
        observed = json_loads(str(row["observed_output_paths_json"] or "[]"))
        if path_text in observed:
            return

    # 3. Handle missing evidence
    if not allow_override:
        raise ArtifactError(
            f"lane_evidence_missing: artifact path {path_text!r} not present in any "
            f"process_executions row for session {session_id}; "
            "pass --allow-no-process-execution to override with an operator rationale.",
            exit_code=6
        )
```

### 3. CLI Flags and Override Path (F-override)

**CLI Parser (`src/striatum/cli/parser.py`):**
Add `--allow-no-process-execution` and `--override-rationale` to the `publish-artifact` subparser.

**CLI Dispatch (`src/striatum/cli/dispatch.py`):**
Update `dispatch` and `_resolve_publish_defaults` to pass these flags through to `publish_artifact`.

**`publish_artifact` update:**
If `allow_no_process_execution` is True and evidence is missing:
- Verify `override_rationale` is non-empty.
- Proceed with publish.
- Write the rationale to the new column.
- Emit the audit event.

### 4. Audit Event (F-event)

Emit `provenance.publish_without_process_execution` when the override is used.

**Payload:**
```json
{
  "artifact_id": "...",
  "session_id": "...",
  "byline": "...",
  "expected_path": "...",
  "rationale": "..."
}
```

### 5. Regression Tests (F-test)

New test file: `tests/test_lane_evidence_guard.py`

**Scenarios:**
1. **Success (Evidence Present)**: Model byline + matching `process_executions` row -> Publish succeeds.
2. **Failure (Missing Evidence)**: Model byline + no matching `process_executions` row -> Publish fails with `lane_evidence_missing`.
3. **Failure (Override Missing Rationale)**: `--allow-no-process-execution` without `--override-rationale` -> Refuses (argparse or validation error).
4. **Success (Override with Rationale)**: `--allow-no-process-execution` + `--override-rationale "..."` -> Publish succeeds, event emitted, rationale stored.
5. **Success (Operator Byline)**: `author: operator` -> Guard is skipped even without evidence.

## Files to Touch
- `src/striatum/migrations.py`: Add v15 migration.
- `src/striatum/schema.py`: Update `SCHEMA_SQL`.
- `src/striatum/daemon_pg/sql/0006_rfc0046_evidence_guard.sql`: New PG migration.
- `src/striatum/artifacts.py`: Implement the guard and override logic in `publish_artifact`.
- `src/striatum/cli/parser.py`: Add new flags to `publish-artifact`.
- `src/striatum/cli/dispatch.py`: Pass flags through to `publish_artifact`.
- `tests/test_lane_evidence_guard.py`: New regression tests.
