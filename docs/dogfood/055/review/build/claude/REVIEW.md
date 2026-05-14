---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "info"
tags: ["ergonomics_dx", "rfc-0050", "v1-5", "build", "operator-ui"]
---

author: reviewer-unknown-model-002

# Build Review — RFC 0050 V1.5 (Claude, ergonomics_dx)

Fresh-context review of the V1.5 screen extensions against
`docs/design/UI_REWORK.md` §9 V1.5-applicable rows, the V1 fix-up
non-negotiables (`docs/dogfood/054b/build/HANDOFF.md`), the V1.5
synthesis (`docs/dogfood/055/DESIGN_SYNTHESIS.md`), and the V1.5
build handoff (`docs/dogfood/055/build/HANDOFF.md`). Posture:
ergonomics_dx — first-time-user discoverability, consistency,
information density, keyboard navigation, screen-reader cues, and
selectable identifiers in lieu of V2 copy-on-click.

## Verdict

**accept_with_findings**

V1 non-negotiables hold across the new V1.5 surfaces, every
extended screen consumes the V1 chip vocabulary without
redefinition, the new partials are wired correctly, doctor recipes
name deterministic CLI verbs, and the view-file breadcrumb is
heuristic-safe with regression coverage for the ambiguous case.
Seven ergonomics findings are recorded below; none block
acceptance — they are V2-or-later polish — but F2 and F3 are the
two most user-visible and worth scheduling soon.

## V1 non-negotiables (verified preserved)

- **No byline forgery on unattested sessions.**
  `_session_chip.html:6-8` only renders a byline line for
  `session.lane_attestation != "attested"` and routes through the
  shared `byline_line` macro, which substitutes
  `author: operator` when `attested is sameas false`
  (`_components.html:72-77`). No new V1.5 surface emits a
  `<role>-<lane>-<n>` byline string for a non-attested session.
- **Override rationale prominent on every surface that shows the
  verdict.** `job_detail.html:51-52` and
  `run_posture_verdicts.html:36-38` render the rationale outside
  any `<details>`. Pinned by
  `tests/test_posture_verdicts_override_provenance.py:50-55` and
  by the existing
  `tests/test_override_rationale_regression.py`.
- **`LaneEvidenceChip` muted (`not_yet_correlated`) in V1.5.**
  `artifact_view.html:33` and `job_detail.html:103` pass the
  literal `"not_yet_correlated"`; the `lane_evidence_chip`
  macro at `_components.html:90-100` renders the muted class.
  No green provenance claim anywhere.
- **No transcript capture introduced.** None of the new
  shaping functions (`_recovery_panel_payload`,
  `_process_evidence_rows`, `_artifact_provenance_trail`) reads
  or writes session transcripts; they only read existing
  `blockers`, `process_executions`, and `events` rows.
- **No inferred override.** `_shape_verdict_rows`
  (`service.py:641-688`) only marks a row as
  `operator_override` when a recorded `verdict.overridden`
  event references its `verdict_id`. The V1 fix-up F2
  heuristic is not reintroduced.
- **Attestation recording-time honesty.**
  `_shape_artifact_rows` continues to derive attestation from the
  recorded `artifacts.author_line`; V1.5 only adds
  `provenance_trail` and `expected_author_line` enrichment on top
  (`service.py:1719-1730`).

## V1.5 surfaces use V1 primitives only

Every extended screen composes — never redefines — V1 macros:

- `run_detail.html:62-66` reuses `lane_attestation_chip` +
  `verdict_chip` on the jobs rail.
- `_session_chip.html:5-7` composes `lane_attestation_chip` +
  `byline_line`.
- `_expected_artifacts_table.html:1-4` is a thin wrapper around
  the V1 `ui.expected_artifacts_table` macro.
- `job_detail.html:14,17,19,49,50,68,102,103` consistently uses
  `job_state_pill`, `lane_attestation_chip`, `verdict_chip`,
  `posture_chip`, `expected_artifacts_table`, `byline_line`, and
  `lane_evidence_chip`.
- `artifact_view.html:32-33` uses `byline_line` +
  `lane_evidence_chip`.
- `run_posture_verdicts.html:33,40` uses `verdict_chip` +
  `lane_attestation_chip`.

No new component file appears under
`src/striatum/web/frontend/src/shared/components/` for V1.5; the
React contracts shipped in V1 are not re-derived.

## New partials follow the v1.41 byline pattern

The three new partials (`_recovery_panel.html`,
`_expected_artifacts_table.html`, `_session_chip.html`) are Jinja2
macro files with no YAML front matter, so the
front-matter-no-author rule does not apply. The single new
Markdown artifact in this dogfood
(`docs/dogfood/055/DESIGN_SYNTHESIS.md`) keeps `author:` on the
title-block line and out of front matter — matches the v1.41
pattern.

## Doctor per-record recipes name deterministic CLI verbs

`_doctor_record_recipes` (`service.py:579-604`) maps known checks
to specific verbs:

- `process_running_but_pid_gone` →
  `striatum recovery process-reconcile --run-id <r>`
- `process_running_with_expired_lease` → same
- `supervisor_lost_with_held_lease` →
  `striatum recovery cancel-job …` and
  `striatum supervise stop --session-id <s>`
- `active_session_on_terminal_run` →
  `striatum session close --session-id <s> --reason terminal_run_cleanup`
- `orphaned_worktree` / `missing_worktree` →
  `striatum doctor --run-id <r> --verbose`
- `human_checkpoint_open` →
  `striatum checkpoint resolve --blocker-id <b> --action {continue,cancel}`

Records without a mapped check fall through to the empty list,
which the template renders as the docs-link prompt at
`doctor.html:54-56`. No invented verbs. Pinned by
`tests/test_doctor_per_record_recipes.py:38-39`.

## View-file breadcrumb is heuristic-safe

`_view_file_run_breadcrumb` (`service.py:618-638`) requires
the path to begin with `docs/dogfood/<digit-id>/...` and that
exactly one `runs.branch_name` matches
`striatum/dogfood-<id>-%`. Returns `None` otherwise. The
template only renders the run link when payload shaping returned
a value (`view_file.html:8-11`). Both the unambiguous and
ambiguous cases are pinned in
`tests/test_view_file_breadcrumb_heuristic.py:10-58`. The
"never wrong-link" rule from UI_REWORK OQ-6 holds.

## Ergonomics findings (info — none blocking)

### F1. Recovery panel does not visually separate human checkpoints from blocked rows

- Severity: minor (consistency / discoverability)
- Location: `src/striatum/web/templates/_recovery_panel.html:11-31`
- The macro concatenates
  `panel.human_checkpoints + panel.blocked` into one `<ul>` and
  uses `'blocked' if blocker.severity == 'human_checkpoint' else blocker.severity`
  for the chip class (`_recovery_panel.html:15`), so both kinds
  carry the same `status-blocked` chip color even though the
  chip text differs. DESIGN_SYNTHESIS §1 calls for "human
  checkpoints first, blocked rows second"; the order is right
  but a first-time user has no header marking the boundary.
  Recommend two sub-lists (`<h3>Human checkpoints</h3>` /
  `<h3>Blocked</h3>`) or a distinct severity chip class for
  `human_checkpoint`.

### F2. Recovery panel renders unconditionally on terminal runs

- Severity: minor (information density)
- Location: `src/striatum/web/templates/run_detail.html:50` and
  `src/striatum/service.py:1564`
- `recovery_ui.recovery_panel(recovery_panel)` is invoked
  outside the next-actions banner guard, so completed / failed /
  canceled runs still show a "Recovery" heading + "No open
  blockers." empty state. The next-actions banner is correctly
  suppressed on terminal runs (`run_detail.html:39`); the
  recovery panel should match. Otherwise terminal runs carry a
  permanent dead UI section that adds noise without value.

### F3. `<h2>Override verdict</h2>` always renders, even when the job has no verdict to override

- Severity: minor (discoverability noise)
- Location: `src/striatum/web/templates/job_detail.html:61-65`
- For an `implement` job (or a review job pre-verdict), the
  disabled "Override verdict" stub still appears with the
  caption "Override modal logic lands in V2; recorded override
  rationales remain visible above." There's nothing above to
  override. The DESIGN_SYNTHESIS §4 sanctions the modal stub
  for V1.5 — but it should be conditioned on `latest_verdict`
  (or on `job.job_type == 'review'`) so first-time users do not
  learn that every job has an override slot.

### F4. Disabled "Override verdict" button has no `aria-describedby`

- Severity: minor (screen reader cue)
- Location: `src/striatum/web/templates/job_detail.html:62-65`
- A screen reader on the disabled button announces "Override
  verdict, dimmed" without the V2 explanation that follows in
  the sibling `<span class="muted">`. Adding
  `aria-describedby="override-stub-help"` to the button and
  `id="override-stub-help"` to the span associates the help text
  with the control.

### F5. Provenance-trail and diagnostic envelopes render as raw JSON dumps

- Severity: minor (information density / first-time-user
  experience)
- Location: `src/striatum/web/templates/artifact_view.html:42-44`
  and `src/striatum/web/templates/job_detail.html:80-83`
- Each event / process diagnostic is wrapped in
  `<pre><code>{{ payload | tojson(indent=2) }}</code></pre>`.
  The load-bearing keys for ergonomics_dx are
  `payload.rationale` (provenance trail) and the recovery
  command list (diagnostic envelopes). Surfacing those as plain
  prose and keeping the raw JSON inside a collapsed `<details>`
  would let readers scan without parsing JSON. Acceptable as
  V1.5 "evidence-only" rendering, but worth a follow-up.

### F6. View-file breadcrumb shows opaque `run_<32-hex>` rather than the branch name

- Severity: minor (discoverability)
- Location: `src/striatum/web/templates/view_file.html:9` (and
  `src/striatum/service.py:638` already returns `branch_name`
  but the template ignores it)
- The breadcrumb chip `Run {{ run_breadcrumb.run_id }}` shows
  the opaque `run_id`. The service already returns
  `branch_name`; the template could render
  `Run {{ run_breadcrumb.branch_name or run_breadcrumb.run_id }}`
  to use the human-friendly handle. Doesn't affect the "never
  wrong-link" guarantee — the link target remains
  `/run/<run_id>`.

### F7. Posture verdicts `Provenance` cell shows a bare `<code>` token instead of a chip

- Severity: very minor (consistency)
- Location: `src/striatum/web/templates/run_posture_verdicts.html:34-39`
- Adjacent columns use chip macros (`verdict_chip`,
  `lane_attestation_chip`); the Provenance column reverts to a
  bare `<code>{{ v.provenance or "natural" }}</code>` token.
  Since `verdict_chip` already encodes provenance via the
  `verdict-provenance-*` modifier the column is largely
  redundant — could be removed or replaced with a small
  `provenance-chip` macro for visual parity. (Cosmetic.)

## Tests reviewed

Six new V1.5 tests landed and read sound:

- `tests/test_run_detail_recovery_panel.py:11-49` — asserts
  `recovery-panel`, blocker kind, payload-derived recipe, and
  the auto-publish recipe string.
- `tests/test_job_detail_expected_artifacts.py:11-48` — asserts
  the expected-artifacts partial, missing-required CLI hint,
  process evidence section, process_id, and the override-verdict
  caption.
- `tests/test_artifact_view_provenance_trail.py:11-45` — asserts
  byline integrity heading, muted lane evidence text, the
  `provenance.publish_without_process_execution` event type, and
  the recorded rationale string.
- `tests/test_posture_verdicts_override_provenance.py:11-57` —
  asserts provenance column, `operator-override` provenance,
  rationale presence, and `unattested` chip.
- `tests/test_doctor_per_record_recipes.py:10-41` — asserts
  the `process_running_with_expired_lease` check and its
  recovery recipe.
- `tests/test_view_file_breadcrumb_heuristic.py:10-58` —
  asserts both unambiguous link rendering and ambiguous-case
  suppression.

## Closing

Net: V1.5 ships the screen extensions called for by RFC 0050 and
DESIGN_SYNTHESIS, holds the V1 fix-up invariants, reuses the V1
chip vocabulary without redefinition, and has focused regression
coverage. The seven findings are ergonomics polish — F2 and F3
are the most worth picking up before V2 island work.
