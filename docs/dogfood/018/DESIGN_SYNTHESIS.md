# Design synthesis: RFC 0018 step 3 (V1.5)

author: designer-codex-gpt-5.5-001
date: 2026-05-09

## Scope

V1.5 ships RFC 0018's deferred step 3: `verdicts.posture`
column + introspection surfacing. Per `STEP3_SHAPE.md`, the
implementation is bounded — one migration + per-surface
adapters that read `verdicts.posture`.

## 1. Migration v10

```python
def _apply_v10_verdicts_posture(conn: sqlite3.Connection) -> None:
    """RFC 0018 step 3 (V1.5): add ``posture`` column to ``verdicts``."""
    cols = [row[1] for row in conn.execute(
        "PRAGMA table_info(verdicts)"
    ).fetchall()]
    if "posture" not in cols:
        conn.executescript(
            "ALTER TABLE verdicts ADD COLUMN posture TEXT NOT NULL "
            "DEFAULT 'neutral';"
        )
    conn.executescript(
        "CREATE INDEX IF NOT EXISTS idx_verdicts_posture "
        "ON verdicts(posture);"
    )
```

Backfill is implicit: the `DEFAULT 'neutral'` on the ALTER
populates existing rows. The index helps future per-posture
queries (V2 may aggregate across runs).

`schema.py` updates the baseline `CREATE TABLE verdicts` to
include `posture TEXT NOT NULL DEFAULT 'neutral'` so a
freshly-initialized DB installs the column directly without
needing to run the migration.

## 2. submit-review hook

`record_review_verdict` (`db.py:1222`) reads the review job's
`review_posture` from the workflow snapshot:

```python
# Inside record_review_verdict, before INSERT:
run = row_by_id(conn, "runs", "run_id", str(job["run_id"]))
snapshot = row_by_id(
    conn,
    "workflow_snapshots",
    "workflow_snapshot_id",
    str(run["workflow_snapshot_id"]),
)
workflow = json_loads(str(snapshot["workflow_json"]))
posture = "neutral"
for entry in workflow.get("jobs", []):
    if (
        isinstance(entry, dict)
        and entry.get("id") == str(job["workflow_job_id"])
        and entry.get("type") == "review"
    ):
        declared = entry.get("review_posture")
        if isinstance(declared, str) and declared:
            posture = declared
        break
```

Then INSERT extends to include `posture` in the VALUES.

## 3. Per-surface rendering

### `status --json` — `verdicts_by_posture`

`introspect.py` adds (alongside the existing `verdicts`
counts):

```json
{
  "verdicts_by_posture": {
    "neutral": 3,
    "security": 1,
    "devils_advocate": 2
  }
}
```

Always emitted (empty dict when no verdicts exist) for stable
shape. Counts are *all* verdicts (any value), not just
accepting — operators want to see whether a posture review
even ran.

### `run summary` Markdown — per-build per-posture

`run_summary.py` grouping gains a `posture` field on each
verdict entry. The rendered `## Verdicts` section
unconditionally includes a posture column when any non-neutral
posture exists in the run; otherwise the existing format is
preserved byte-for-byte (zero-regression).

### `evidence export` — posture in verdict block

The per-verdict Markdown block adds a `Posture: <value>` line.
Always emitted post-V1.5 (defaults to `neutral`). This is a
*format* change to evidence-export Markdown — downstream
consumers parsing the redacted block need to handle the new
line. Documented as a CHANGELOG note.

### `run graph --format json` — posture on review nodes

`introspect.py:728` adds `posture` to the `latest_verdict`
dict on review nodes. Only emitted when a verdict exists.

### Dashboard verdicts panel — posture summary line

`dashboard.py:326` adds (after the existing `Verdicts:`
header) a one-line `Postures: <p1>=<n1>, <p2>=<n2>` summary
when any non-neutral posture exists in the run. Truncates to
top-3 by count with `+N more` overflow.

### Web UI job detail — posture chip

`app.js:419-421` adds a posture chip alongside each verdict
badge for review jobs whose `latest_verdict` carries posture.
CSS class `posture-chip` added to `app.css` matching the
existing badge palette.

## 4. Test plan

`tests/test_review_postures_introspection.py`:

1. Migration v10 idempotency.
2. Backfill: pre-existing verdict rows get `posture = 'neutral'`.
3. submit-review writes declared posture.
4. submit-review writes `'neutral'` for posture-omitting jobs.
5. `status --json` `verdicts_by_posture` shape + counts.
6. `run summary` Markdown includes posture grouping when
   non-neutral postures exist.
7. `run summary` Markdown is byte-identical to v1.8.1 for
   posture-omitting runs.
8. `evidence export` posture line present.
9. `run graph --format json` review node `latest_verdict`
   carries posture.
10. Dashboard one-line summary present when non-neutral
    postures exist; absent otherwise.

## 5. Zero-regression contract

A run with no posture-declaring review jobs:

- `verdicts.posture` is `'neutral'` for every row.
- `status` `verdicts_by_posture` is `{"neutral": N}`.
- `run summary` Markdown is byte-identical to v1.8.1.
- `evidence export` adds the `Posture: neutral` line (the
  one intentional regression — documented).
- `run graph --format json` review nodes get `posture: "neutral"`
  on `latest_verdict`.
- Dashboard verdicts panel adds no new line.
- Web UI renders no posture chips (or chips for `neutral` —
  the synthesis chooses to render only non-neutral chips,
  matching the dashboard truncation rationale).
