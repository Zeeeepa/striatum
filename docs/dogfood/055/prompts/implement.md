# Implement — RFC 0050 V1.5 template extensions

Blocked until `review_design` returns an accepting verdict.

**Canonical inputs:**
- `docs/design/UI_REWORK.md` — full spec (1845 lines). V1.5 scope.
- `docs/dogfood/055/DESIGN_SYNTHESIS.md` — phase scope.
- `docs/rfcs/0050-operator-ui-rework-and-provenance-honesty.md`.
- `docs/dogfood/054/build/HANDOFF.md` +
  `docs/dogfood/054b/build/HANDOFF.md` — V1 primitives you reuse.

**Write scope:** `src/striatum/web/`, `src/striatum/service.py`,
`src/striatum/dashboard.py`, `tests/`,
`docs/dogfood/055/build/`. No writes to `.striatum/`, `go/`,
prior dogfoods.

## V1.5 deliverables (per synthesis + UI_REWORK.md §4 + §8)

1. **`run_detail.html` restructure**: next-actions banner at the
   top (consumes V1.45.0 next_actions verbatim), recovery panel
   section (lists open blockers + their stable next-actions),
   sessions strip (active sessions with attestation chips).
2. **`job_detail.html` extend**: `ExpectedArtifactsTable` partial
   shows declared expected_artifacts vs published artifacts;
   process-evidence section shows the linked `process_executions`
   row (read-only); override-modal stub (V2 wires the modal).
3. **`artifact_view.html` extend**: byline integrity surface
   (renders `BylineLine` with attestation context); provenance
   section showing recovery.auto_published / publish_without_process_execution
   event trail for the artifact.
4. **`run_posture_verdicts.html` extend**: per-row provenance
   chip (natural / operator-override / cycle-revised) + lane
   attestation chip; override rows visually distinct (CSS
   token already exists from V1).
5. **`doctor.html` extend**: per-record recipes — for each
   problem record, list the deterministic CLI next-actions
   that close it.
6. **`view_file.html` breadcrumb**: heuristic match against
   `runs.branch_name` (e.g. `striatum/dogfood-<NNN>-*`). When
   path looks like an artifact under that run, render a link
   back. NEVER wrong-link — when heuristic fails, render
   nothing.
7. **New partials**:
   - `_recovery_panel.html` — used by `run_detail`.
   - `_expected_artifacts_table.html` — used by `job_detail`.
   - `_session_chip.html` — used by `run_detail`, `job_detail`.
8. **`service.py` page-payload shaping** for the new sections:
   recovery panel data, expected-artifacts-vs-published data,
   process-execution-evidence data, posture-verdict provenance
   data, doctor recipes data, view_file breadcrumb data.
9. **`dashboard.py` text-mode parity** for any new dashboard
   widgets the synthesis names.
10. **Tests** — at minimum:
    - `tests/test_run_detail_recovery_panel.py`
    - `tests/test_job_detail_expected_artifacts.py`
    - `tests/test_artifact_view_provenance_trail.py`
    - `tests/test_posture_verdicts_override_provenance.py`
    - `tests/test_doctor_per_record_recipes.py`
    - `tests/test_view_file_breadcrumb_heuristic.py`

## Reuse, don't redefine

The V1 shared components (`RunStatePill`, `JobStatePill`,
`VerdictChip`, `LaneAttestationChip`, `BylineLine`, etc) and
their Jinja macro mirrors (`_components.html`) MUST be reused.
Do not redefine.

## Tools

- V1.41: `striatum byline`, `striatum inbox`,
  `recovery auto-publish`.
- V1.41: publish-artifact defaults `--kind` + `--logical-name`
  from `expected_artifacts`.
- RFC 0046 V1: lane evidence guard active — operator-on-behalf
  publishes need `--allow-no-process-execution
  --override-rationale "<text>"`.

## HANDOFF

`docs/dogfood/055/build/HANDOFF.md`. Front matter MUST NOT
include `author:`. Byline lives on title-block line. One
section per deliverable (1–10 above) with file:line evidence.
Test results.

## Out of scope

- Recovery-panel ISLAND (V2). The V1.5 partial is server-
  rendered HTML only.
- Override-MODAL LOGIC (V2). The V1.5 surface is the button +
  data attributes; modal JS lands in V2.
- Copy-on-click (V2).
- `workflow-graph-editor::require_attested_lane` (V2 — data
  binding only per operator direction).
