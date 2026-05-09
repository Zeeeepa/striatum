# Research: verdicts schema + introspection touchpoints

author: researcher-codex-gpt-5.5-001
date: 2026-05-09

## Schema and migration

- `src/striatum/schema.py:178-190` — `CREATE TABLE verdicts`
  baseline. V1.5 adds `posture TEXT NOT NULL DEFAULT 'neutral'`
  to the baseline so fresh DBs install the column directly.
- `src/striatum/migrations.py:370-399` — current
  `MIGRATIONS` list, latest version 9. V1.5 appends migration
  v10:
  ```sql
  ALTER TABLE verdicts ADD COLUMN posture TEXT NOT NULL DEFAULT 'neutral';
  CREATE INDEX IF NOT EXISTS idx_verdicts_posture ON verdicts(posture);
  ```
  Idempotent against fresh DBs (PRAGMA `table_info` check
  before ALTER, mirroring `_apply_v9_blockers_payload_json`).
  Backfill: the `DEFAULT 'neutral'` clause sets the column
  value for existing rows on ALTER. Forward-only.

## submit-review hook

- `src/striatum/db.py:1222` — `record_review_verdict`. The
  INSERT statement extends to include `posture`. Source:
  - Look up the workflow snapshot via the run row.
  - Find the review job in the workflow JSON by
    `workflow_job_id`.
  - Read `review_posture` (default `"neutral"` when omitted
    OR when the field is unset).
  - INSERT into `verdicts.posture`.
- The job table itself does NOT carry `review_posture` —
  it lives only in the workflow snapshot. So the lookup must
  go through the snapshot.

## Six introspection surfaces

### 1. `striatum status --json` — `verdicts_by_posture`

`src/striatum/cli/introspect.py` assembles the run status
payload. V1.5 adds a `verdicts_by_posture` dict mapping each
distinct posture to a count of accepting verdicts (parallel
to the existing `verdicts` block). The block is *always*
emitted (empty dict when no postures exist) for stable shape.

### 2. `striatum run summary` — per-build per-posture grouping

`src/striatum/cli/run_summary.py:113-160` —
`_group_verdicts_by_workflow_job` groups verdicts by review
job. V1.5 adds an inner posture grouping when at least one
non-neutral posture exists in the run. The Markdown rendered
under `## Verdicts` (line 210) gains a posture column or
sub-bullet.

### 3. `striatum evidence export` — posture in verdict block

`src/striatum/cli/evidence.py` — the per-verdict block
already lists `verdict`, `created_at`, `findings_artifact_id`,
and (redacted) `rationale`. V1.5 adds `posture` alongside
`verdict`. Always emitted.

### 4. `striatum run graph --format json` — posture on review nodes

`src/striatum/cli/introspect.py:728-738` — the run_graph
json renderer wraps `latest_verdict_row` results into a
`latest_verdict` dict on review nodes. V1.5 adds `posture`
to that dict (read from `verdict_row["posture"]`). Always
emitted when a verdict exists.

### 5. Dashboard verdicts panel

`src/striatum/dashboard.py:326` — the existing Verdicts panel
renders a list of `<role>: <verdict>` lines. V1.5 adds a
one-line `Postures: <p1>=<n1>, <p2>=<n2>` summary when at
least one non-neutral posture exists. Truncates to the top-3
postures by count with `+N more` overflow when more exist (per
RFC 0018's "≤ 4 postures" guidance).

### 6. Web UI job detail — posture chip

`src/striatum/web/static/app.js:419-421` — the existing
verdicts panel renders verdict badges. V1.5 adds a posture
chip alongside the badge for review jobs whose `latest_verdict`
includes `posture`. CSS class `posture-chip` added to
`app.css`.

## Test plan

`tests/test_review_postures_introspection.py` (new file):

1. Migration v10 idempotency: apply twice, no error; column
   present once; index present once.
2. Backfill: a verdict row inserted before the migration has
   `posture = 'neutral'` after migration.
3. `record_review_verdict` writes the correct posture for a
   new verdict on a posture-declared review job.
4. `record_review_verdict` writes `'neutral'` for a posture-
   omitting review job.
5. `status --json` `verdicts_by_posture` has the right counts.
6. `run summary` Markdown includes per-posture grouping.
7. `evidence export` per-verdict block has `posture`.
8. `run graph --format json` review node `latest_verdict`
   has `posture`.
9. Dashboard panel one-line summary is present when non-neutral
   postures exist.
10. Web UI app.js renders posture chip (probe via static-file
    presence + class name presence in HTML output).
11. Zero regression: a posture-omitting run produces
    byte-identical output across all surfaces compared against
    a recorded baseline (or assert specific keys absent /
    `'neutral'`).

## Summary table

| Surface | File:line | V1.5 action |
| --- | --- | --- |
| Schema baseline | `schema.py:178` | Add `posture TEXT NOT NULL DEFAULT 'neutral'` |
| Migration v10 | `migrations.py:370` | Append migration; ALTER + INDEX |
| INSERT verdict | `db.py:1222` | Read posture from snapshot, write column |
| status JSON | `cli/introspect.py` | Add `verdicts_by_posture` dict |
| run summary | `cli/run_summary.py:113` | Group inner by posture |
| evidence | `cli/evidence.py` | Add posture to verdict block |
| run graph json | `cli/introspect.py:728` | Add `posture` to `latest_verdict` |
| dashboard | `dashboard.py:326` | Add posture summary line |
| web UI | `web/static/app.js:421` + `app.css` | Add posture chip |
