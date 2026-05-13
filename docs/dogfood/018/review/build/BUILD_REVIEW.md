# Build review: RFC 0018 step 3 V1.5

author: reviewer-claude-opus-002
date: 2026-05-09
verdict: accept

Devil's-advocate review of the V1.5 build against the synthesis,
the V1_ACCEPTANCE design-review findings (1, 2, 3), and the
zero-regression contract.

## Verdict

**accept** — V1.5 acceptance gate satisfied. All three design-
review findings addressed. 370/370 full-suite pass.

## Sweep matrix

| Acceptance gate | How V1.5 satisfies it | Verified |
| --- | --- | --- |
| **Migration v10 idempotency** | `_apply_v10_verdicts_posture` checks `PRAGMA table_info` before ALTER; `CREATE INDEX IF NOT EXISTS` for the index. Re-running migrations on a v10 DB is a no-op. | `test_migration_v10_idempotent` |
| **Backfill rule** | `DEFAULT 'neutral'` clause on the ALTER auto-fills existing rows. | `test_verdicts_posture_column_present_after_init` |
| **submit-review writes correct posture** | `_resolve_review_posture` reads from the immutable workflow snapshot (correct source per Finding §3 in design review); writes literal posture or `'neutral'`. Three tests cover declared / undeclared / custom. | All three pass. |
| **status `verdicts_by_posture`** | `_count_verdicts_by_posture` aggregates by posture; always emitted (empty dict when no verdicts) for stable shape. | `test_status_emits_verdicts_by_posture`, `test_status_verdicts_by_posture_empty_when_no_verdicts` |
| **run summary per-posture** | `_group_verdicts_by_workflow_job` carries `latest_posture`; the renderer adds the suffix only when at least one non-neutral posture exists in the run. Posture-omitting runs verify byte-identical output. | `test_run_summary_includes_posture_when_non_neutral`, `test_run_summary_omits_posture_for_neutral_only` |
| **evidence export** | `posture` added to the SELECT and to the redaction whitelist as `safe`. The exported Markdown's JSON snapshot block contains `"posture": "<value>"` for every verdict. | `test_evidence_export_includes_posture_in_verdict`, `test_evidence_export_neutral_posture_present` |
| **run graph json** | The `latest_verdict` annotation in `run_graph` reads `verdict_row.get("posture", "neutral")` and includes it; only emitted when a verdict exists. | `test_run_graph_json_includes_posture_on_review_verdict` |
| **Dashboard panel** | `_render_right_column` walks `posture_counts`; renders summary line only when `non_neutral` filter is non-empty; sorts by `(-count, name)` for deterministic tie-break (Finding 3); top-3 with `+N more` overflow. | `test_dashboard_renders_posture_summary_when_non_neutral`, `test_dashboard_omits_posture_summary_when_only_neutral` |
| **Web UI chip rendering** | `app.js` renders a `<span class="posture-chip" title="...">` only for non-neutral postures. `app.css` `.posture-chip` rule pinned per Finding 2: gray background, max-width 12em, ellipsis truncation, white-space nowrap. | Source review of `app.js` line 419-432 and `app.css` line 80-93. |
| **Zero regression** | Posture-omitting runs verified byte-identical in the run-summary and dashboard tests. The `evidence export` change (additive `posture: "neutral"` field) is the one intentional regression, called out as a "Changed (intentional)" CHANGELOG subsection. | Tests + CHANGELOG note. |
| **Suite health** | 15/15 introspection tests; 370/370 full suite; lint clean; mypy clean (62 source files). | Direct run output. |

## Counterargument sweep

### "The `_resolve_review_posture` helper is overkill"

Counterargument: the helper opens a fresh row lookup on every
verdict. For high-volume runs this is a minor overhead. Survives?
Yes — verdicts are recorded one at a time per submit-review;
this is not a hot path. **Accept.**

### "The dashboard sort is non-deterministic when counts tie *and* posture names tie"

Posture names are unique strings (the table groups by posture
and there's at most one row per posture). The `(-count, name)`
key fully orders the list. **Accept.**

### "The web UI `posture-chip` ellipsis hides the full custom name"

The chip's `title` attribute (HTML tooltip) shows the full
posture name on hover. The synthesis pinned this in Finding 2.
**Accept.**

### "The evidence-export format change is a real downstream
regression"

The CHANGELOG explicitly calls this out as a "Changed
(intentional)" subsection, naming the parser shapes that tolerate
vs. break. The runner's bounded context is to *record* the
verdict; exposing posture in the audit trail is exactly the
operator value V1.5 ships. **Accept.**

### "The migration's DEFAULT clause has SQLite ALTER caveats"

SQLite's ALTER TABLE ADD COLUMN with a literal DEFAULT value
(non-NULL constant) is supported and well-tested across all
SQLite versions striatum supports. The DEFAULT is a *table
default*, not a per-row materialization, but `SELECT posture
FROM verdicts` returns `'neutral'` for pre-migration rows
because SQLite resolves the column default on read when no
on-disk value exists. **Accept.** (`test_verdicts_posture_column_present_after_init`
verifies the column resolves to the default value.)

## Decision

Accept V1.5. Land the change, bump to 1.9.0, transition RFC 0018
to `accepted (V1+step 3)`. Step 3 closes RFC 0018 in full.

This dogfood (017 + 018) also serves as the second end-to-end
exercise of RFC 0018 V1's posture validation gate (017 was the
first; both runs validated cleanly).
