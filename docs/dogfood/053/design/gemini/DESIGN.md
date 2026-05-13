# Design — RFC 0046 V1 Lane Evidence Guard

author: designer-unknown-model-001

## Status
**Proposed** (Dogfood 053)

## Goals
- **Close the Trust Gap**: Prevent operators from intentionally or accidentally forging model bylines for artifacts that were not actually produced by the supervised model process.
- **Implement Lane Evidence Guard**: Integrate a check into the `publish-artifact` flow that verifies the existence of `process_executions` evidence for model-attributed artifacts.
- **Auditable Override Path**: Provide an explicit opt-out for legitimate operator-on-behalf scenarios (e.g., recovery, manual synthesis) that records a mandatory rationale in the audit trail.
- **Regression Testing**: Pin the behavior with a comprehensive test suite covering both the happy path and various failure/override modes.

## Background
Currently, `striatum` differentiates between "attested" and "unattested" sessions for the purpose of byline derivation. An unattested session (e.g., one without an active supervisor) correctly defaults to `author: operator`. However, once a session is attested (a supervisor is started), the runner trusts that any artifact published by that session was indeed produced by the supervised process. 

As discovered in triage, an operator can start a supervisor, let it exit (or even while it's running), and then manually write a file and call `publish-artifact`. The runner will accept this and attach a model byline, creating "false provenance." RFC 0046 addresses this by requiring evidence from the `process_executions` table—specifically that the file path being published was observed as an output of a process executed within that session.

## Design

### 1. Schema Migration (F-schema)

We need a persistent location to store the override rationale. This will be added to the `artifacts` table.

**Migration (v15) in `src/striatum/migrations.py`:**
The migration will use a simple `ALTER TABLE` to add the column. Since SQLite handles `NULL` values for new columns by default, existing rows will be automatically compatible.

```python
def _apply_v15_attestation_override_rationale(conn: sqlite3.Connection) -> None:
    """RFC 0046: add attestation_override_rationale column to artifacts."""
    # Idempotency check: only add if missing (useful for dev/test cycles)
    cols = [row[1] for row in conn.execute("PRAGMA table_info(artifacts)").fetchall()]
    if "attestation_override_rationale" not in cols:
        conn.execute("ALTER TABLE artifacts ADD COLUMN attestation_override_rationale TEXT")
```

**Postgres side (`src/striatum/daemon_pg/sql/0006_rfc0046_evidence_guard.sql`):**
The Postgres migration follows the same pattern, adding the column to the `striatumd.artifacts` table.

```sql
-- Migration 0006: RFC 0046 Lane Evidence Guard
ALTER TABLE striatumd.artifacts ADD COLUMN attestation_override_rationale TEXT;
```

### 2. Evidence Guard in `publish_artifact` (F-guard)

The core logic of the guard will be implemented in `src/striatum/artifacts.py`. It must be called within `publish_artifact` after the byline has been determined but before the database record is created.

**Implementation Logic:**
1.  **Identify Model Bylines**: Use `_canonical_byline_form` or a similar pattern to check if the author line matches the model template (`<role>-<model>-<ord>`). If the byline starts with `author: operator`, the guard is skipped (preserving today's operator authority).
2.  **Evidence Lookup**: If it is a model byline, query the `process_executions` table for all rows associated with the current `session_id` where `state = 'completed'`.
3.  **Path Verification**: Parse the `observed_output_paths_json` from these rows. If the `path_text` of the artifact being published is found in at least one of these lists, the guard passes.
4.  **Refusal**: If no evidence is found and no override is supplied, raise `ArtifactError` with a specific message and exit code 6.

**Refined Code Sketch:**
```python
def _enforce_lane_evidence_guard(
    conn: sqlite3.Connection,
    *,
    session_id: str,
    path_text: str,
    byline: str,
    allow_override: bool,
) -> None:
    # 1. Bypass for operator bylines
    if byline.startswith("author: operator"):
        return

    # 2. Query process_executions for evidence
    # We only trust 'completed' processes as definitive evidence.
    rows = conn.execute(
        """
        SELECT observed_output_paths_json FROM process_executions
        WHERE session_id = ? AND state = 'completed'
        """,
        (session_id,),
    ).fetchall()

    for row in rows:
        try:
            observed = json.loads(str(row["observed_output_paths_json"] or "[]"))
        except (json.JSONDecodeError, TypeError):
            continue
            
        if isinstance(observed, list) and path_text in observed:
            return

    # 3. Handle missing evidence
    if not allow_override:
        raise ArtifactError(
            f"lane_evidence_missing: artifact path {path_text!r} not present in any "
            f"process_executions row for session {session_id}. "
            "Evidence is required for model-attributed artifacts. "
            "Use --allow-no-process-execution --override-rationale \"...\" to override.",
            exit_code=6
        )
```

### 3. CLI Surface and Dispatch (F-override)

The override path requires two new flags: `--allow-no-process-execution` (a boolean toggle) and `--override-rationale` (a string).

**`src/striatum/cli/parser.py`:**
```python
publish.add_argument(
    "--allow-no-process-execution", 
    action="store_true",
    help="Override the lane evidence guard when evidence is missing"
)
publish.add_argument(
    "--override-rationale",
    help="Mandatory rationale when using --allow-no-process-execution"
)
```

**`src/striatum/cli/dispatch.py`:**
The `dispatch` function will extract these arguments and pass them to `publish_artifact`. We will also update `_resolve_publish_defaults` to ensure these flags are handled correctly if they are used in automated flows.

### 4. Audit Event and Record Keeping (F-event)

When the override is successfully triggered, we must record it.

1.  **Event Emission**: Call `insert_event` with type `provenance.publish_without_process_execution`.
    -   Payload: `artifact_id`, `session_id`, `byline`, `expected_path`, `rationale`.
2.  **Database Storage**: Store the `override_rationale` in the new `attestation_override_rationale` column of the `artifacts` table.

### 5. Test Strategy (F-test)

The new test file `tests/test_lane_evidence_guard.py` will use the `harness` fixture to simulate multiple workflow states.

**Scenarios to Cover:**
-   **Scenario A (Happy Path)**: Session with a completed process execution that lists `test.md`. `publish-artifact --path test.md` succeeds with model byline.
-   **Scenario B (Blocked Forgery)**: Session with NO process executions. `publish-artifact --path test.md` fails with exit code 6.
-   **Scenario C (Override Refused)**: Session without evidence. `publish-artifact --path test.md --allow-no-process-execution` (missing rationale) fails with exit code 2 (argparse validation).
-   **Scenario D (Override Accepted)**: Session without evidence. `publish-artifact --path test.md --allow-no-process-execution --override-rationale "Manual fix"` succeeds. Verify `events` row and `artifacts` column.
-   **Scenario E (Operator Immunity)**: Session without evidence, but byline recomputed as `author: operator` (e.g., unattested session). `publish-artifact --path test.md` succeeds.

## Files to Touch
- `src/striatum/migrations.py`: Add `v15` migration.
- `src/striatum/schema.py`: Update `SCHEMA_SQL` (baseline).
- `src/striatum/daemon_pg/sql/0006_rfc0046_evidence_guard.sql`: New PG migration.
- `src/striatum/artifacts.py`: Implement `_enforce_lane_evidence_guard` and update `publish_artifact`.
- `src/striatum/cli/parser.py`: Add new flags to `publish-artifact` subparser.
- `src/striatum/cli/dispatch.py`: Update `dispatch` and `_resolve_publish_defaults`.
- `tests/test_lane_evidence_guard.py`: New regression tests.

## Rollout and Compatibility
- **Migrations**: Automatic on next `striatum` invocation. Existing data remains compatible (rationale is NULL).
- **Tooling**: The guard is strictly additive to `publish-artifact`. No existing workflows are broken as long as agents are behaving correctly (actually producing the files they publish).
- **Recourse**: The override rationale provides a "safety valve" for edge cases while maintaining an audit trail.
