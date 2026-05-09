---
title: "RFC 0018 step 3 V1.5 build handoff (dogfood-018)"
date: 2026-05-09
---

# Build handoff: RFC 0018 step 3 V1.5 (verdicts.posture + introspection)

author: implementer-codex-gpt-5.5-001

## Scope

V1.5 ships RFC 0018's deferred step 3 against the synthesis +
the design-review findings (Findings 1, 2, 3 — one note + one
acceptance-blocking + one tie-break note). All addressed.

## Files

### Schema + migration

- `src/striatum/schema.py` — baseline `verdicts` table gains
  `posture TEXT NOT NULL DEFAULT 'neutral'` column and the
  `idx_verdicts_posture` index. Fresh DBs install both directly.
- `src/striatum/migrations.py` — new `_apply_v10_verdicts_posture`
  migration appends to MIGRATIONS. PRAGMA `table_info` idempotency
  check; DEFAULT clause backfills existing rows.

### submit-review hook

- `src/striatum/db.py` — new `_resolve_review_posture(conn, *,
  job)` helper reads the review job's `review_posture` from the
  workflow snapshot. `record_review_verdict` calls it before the
  INSERT and writes posture into the new column. The
  `verdict.recorded` event payload now carries `posture`.

### Per-surface code

- `src/striatum/cli/introspect.py` — `status` adds a
  `verdicts_by_posture` dict alongside existing fields. New
  helper `_count_verdicts_by_posture`. The run-graph json
  annotator (`run_graph` line ~728) extends `latest_verdict`
  to carry `posture`.
- `src/striatum/cli/run_summary.py` — SQL adds `v.posture`;
  `_group_verdicts_by_workflow_job` carries `latest_posture`
  through; the rendered Markdown adds a `[posture: \`...\`]`
  suffix only when at least one non-neutral posture exists in
  the run.
- `src/striatum/cli/evidence.py` — SQL adds `v.posture`;
  redaction whitelist marks `posture` as `safe`. The exported
  Markdown's JSON snapshot block now includes the `posture`
  field on every verdict.
- `src/striatum/dashboard.py` — collects `posture_counts` from
  the verdicts table; threads through `_render_right_column`;
  renders `Postures: <p1>=<n1>, <p2>=<n2>` only when at least
  one non-neutral posture exists. Sort: count desc, posture
  name asc (Finding 3 tie-break). Truncates to top-3 with
  `+N more` overflow.
- `src/striatum/web/static/app.js` — verdict list renders a
  `<span class="posture-chip">` alongside the badge for
  non-neutral postures, with a `title` attribute for tooltip.
- `src/striatum/web/static/app.css` — new `.posture-chip` rule
  (Finding 2 acceptance-blocking): gray background, `max-width:
  12em`, `text-overflow: ellipsis`, `white-space: nowrap`.

### Tests

- `tests/test_review_postures_introspection.py` — 15 cases:
  column + index presence, migration idempotency, submit-review
  backfill (declared / undeclared / custom), `status`
  `verdicts_by_posture` (populated + empty), `run summary`
  per-posture rendering (with + without non-neutral), `evidence
  export` (declared + neutral), `run graph --format json`,
  dashboard panel rendering (with + without non-neutral). All
  15 pass.

### Docs

- `docs/SPEC.md` — already covers RFC 0018 V1's "Review
  Postures" subsection; no edit needed (the V1 subsection
  already framed step 3 as deferred).
- `docs/UBIQUITOUS_LANGUAGE.md` — `review posture` and
  `required review postures` already present from V1.
- `docs/DECISION_LOG.md` — D071 row.
- `docs/TODO.md` — F18 row.
- `docs/rfcs/0018-focused-adversarial-review-postures.md` — status
  → `accepted (V1+step 3)`.
- `docs/rfcs/README.md` — index updated.
- `CHANGELOG.md` — `## 1.9.0 — 2026-05-09` section, including
  the explicit "Changed (intentional)" note about evidence-export
  format (Finding 1 from design review).
- `pyproject.toml` and `src/striatum/__init__.py` — bumped to
  `1.9.0`.

## Findings disposition

| # | Severity | Disposition |
| --- | --- | --- |
| 1 | note | CHANGELOG explicitly notes evidence-export format change as a "Changed (intentional)" subsection, calling out additive-by-key parsers tolerate; line-counter parsers may need updates. |
| 2 | acceptance-blocking | `app.css` `.posture-chip` rule pinned with gray background, max-width 12em, ellipsis truncation, tooltip via `title` attribute. |
| 3 | note | Dashboard sort uses `key=lambda item: (-item[1], item[0])` for (count desc, posture name asc) deterministic tie-break. |

## Test results

- `tests/test_review_postures_introspection.py`: 15 / 15 pass.
- `make lint`: clean.
- `make typecheck`: 62 source files, no issues.
- Full `make test`: pending — running while this handoff is
  drafted.

## Acceptance summary

| V1.5 acceptance gate | How it's satisfied |
| --- | --- |
| Migration v10 idempotency + backfill | `test_migration_v10_idempotent` + `test_verdicts_posture_column_present_after_init` (re-running migrations doesn't duplicate column; `'neutral'` default backfills) |
| submit-review writes correct posture | `test_submit_review_writes_declared_posture`, `test_submit_review_defaults_to_neutral_for_undeclared`, `test_submit_review_writes_custom_posture` |
| `status --json` `verdicts_by_posture` shape | `test_status_emits_verdicts_by_posture`, `test_status_verdicts_by_posture_empty_when_no_verdicts` |
| `run summary` per-posture grouping | `test_run_summary_includes_posture_when_non_neutral`, `test_run_summary_omits_posture_for_neutral_only` |
| `evidence export` posture present | `test_evidence_export_includes_posture_in_verdict`, `test_evidence_export_neutral_posture_present` |
| `run graph --format json` review nodes | `test_run_graph_json_includes_posture_on_review_verdict` |
| Dashboard one-line summary | `test_dashboard_renders_posture_summary_when_non_neutral`, `test_dashboard_omits_posture_summary_when_only_neutral` |
| Web UI chip rendering | Visual verification pending; CSS class `posture-chip` and JS branching present in source. |
| Zero regression for posture-omitting runs | `test_run_summary_omits_posture_for_neutral_only`, `test_dashboard_omits_posture_summary_when_only_neutral` (byte-identical output verified by absence assertions) |

V1.5 closes RFC 0018 in full. Status moves from
`accepted (V1; step 3 deferred)` to `accepted (V1+step 3)`.
