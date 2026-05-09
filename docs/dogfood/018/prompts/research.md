# Research: verdicts schema + introspection touchpoints

Map:

1. `verdicts` table schema (`src/striatum/schema.py` /
   `src/striatum/migrations.py`) — current columns, indices,
   triggers; the `PRAGMA user_version` cadence.
2. `record_review_verdict` in `src/striatum/db.py` — the INSERT
   statement; where the review job's `review_posture` would be
   read from (the workflow_snapshot or the job row's
   `capability_requirements_json`).
3. The six introspection surfaces:
   - `striatum status --run-id <id>` — where the verdict counts
     get assembled (`src/striatum/cli/introspect.py` likely).
   - `striatum run summary --run-id <id>` — where the markdown
     gets rendered (`src/striatum/cli/run_summary.py`).
   - `striatum evidence export` — where the verdict block lives
     (`src/striatum/cli/evidence.py`).
   - `striatum run graph --format json` — where review nodes get
     emitted.
   - Dashboard verdicts panel — `src/striatum/cli/...dashboard...`.
   - Web UI job detail view — `src/striatum/web/static/...`.
4. Test precedents — `test_recovery_extended.py`,
   `test_run_summary.py`, `test_evidence_export.py`,
   `test_dashboard.py`, `test_web_ui.py`.

Deliverable: `docs/dogfood/018/research/STEP3_SHAPE.md` listing
file:line citations and the migration v10 shape.
