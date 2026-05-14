# RFC 0050 V1.5 Implementation Handoff

Status: complete
Date: 2026-05-14
author: implementer-unknown-model-001

## 1. Run Detail Restructure

Implemented. `/run/<id>` now consumes shaped recovery payloads and renders the server-side recovery panel after the next-actions banner.

- `src/striatum/service.py:505` builds recovery-panel data from open blockers plus V1.45 `next_actions`.
- `src/striatum/service.py:1562` passes `next_actions` and `recovery_panel` to the run page.
- `src/striatum/web/templates/_recovery_panel.html:1` defines the reusable server-rendered panel.
- `src/striatum/web/templates/run_detail.html:89` keeps the next-actions banner at the top.
- `src/striatum/web/templates/run_detail.html:100` renders the recovery panel.

## 2. Job Detail Expected Artifacts And Process Evidence

Implemented. Job detail now uses the required expected-artifacts partial, renders process execution evidence, and includes a V2 override-modal stub without wiring mutation logic.

- `src/striatum/web/templates/_expected_artifacts_table.html:1` wraps the shared V1 expected-artifact macro.
- `src/striatum/service.py:525` shapes process execution rows for a job.
- `src/striatum/service.py:1656` passes `process_evidence` into `job_detail.html`.
- `src/striatum/web/templates/job_detail.html:231` renders the disabled override modal stub.
- `src/striatum/web/templates/job_detail.html:237` renders expected artifacts through the partial.
- `src/striatum/web/templates/job_detail.html:240` renders process evidence.

## 3. Artifact View Provenance

Implemented. Artifact view now renders byline integrity through the shared byline/evidence macros and shows operator-on-behalf provenance events without claiming correlated green process evidence.

- `src/striatum/service.py:552` extracts `recovery.auto_published` and `provenance.publish_without_process_execution` trail rows.
- `src/striatum/service.py:1699` enriches the single artifact row with expected byline, recorded-byline attestation, and provenance trail data.
- `src/striatum/web/templates/artifact_view.html:310` renders byline integrity and muted lane evidence.
- `src/striatum/web/templates/artifact_view.html:314` renders the provenance trail.

## 4. Posture Verdict Provenance

Implemented. Posture verdict rows now reuse shared verdict and attestation macros, add provenance/attestation columns, and mark operator override rows distinctly.

- `src/striatum/service.py:1465` shapes posture verdict rows through `_shape_verdict_rows`.
- `src/striatum/web/templates/run_posture_verdicts.html:20` adds provenance and attestation columns.
- `src/striatum/web/templates/run_posture_verdicts.html:32` marks override rows.
- `src/striatum/web/templates/run_posture_verdicts.html:33` renders `VerdictChip`.
- `src/striatum/web/templates/run_posture_verdicts.html:40` renders `LaneAttestationChip`.

## 5. Doctor Per-Record Recipes

Implemented. Doctor page problem records are shaped with deterministic CLI recipes where a known mechanical path exists.

- `src/striatum/service.py:579` maps known doctor checks to CLI recipes.
- `src/striatum/service.py:607` attaches recipes to verbose problem records.
- `src/striatum/service.py:2375` groups shaped records for `/doctor`.
- `src/striatum/web/templates/doctor.html:142` renders per-record recipe text.

## 6. View File Breadcrumb

Implemented. `/view/<path>` now uses the dogfood path and `runs.branch_name` heuristic to render a run breadcrumb only when the match is unambiguous.

- `src/striatum/service.py:618` implements the conservative branch/path heuristic.
- `src/striatum/service.py:2925` adds optional breadcrumb data to the file-view payload.
- `src/striatum/web/templates/view_file.html:182` renders the run link only when payload shaping found exactly one match.

## 7. New Partials

Implemented.

- `src/striatum/web/templates/_recovery_panel.html:1`
- `src/striatum/web/templates/_expected_artifacts_table.html:1`
- `src/striatum/web/templates/_session_chip.html:1`
- `src/striatum/web/templates/run_detail.html:148` uses `_session_chip.html` for the sessions strip.

## 8. Service Payload Shaping

Implemented in `src/striatum/service.py`.

- Recovery panel: `src/striatum/service.py:458`
- Process evidence: `src/striatum/service.py:525`
- Artifact provenance trail: `src/striatum/service.py:552`
- Doctor recipes: `src/striatum/service.py:579`
- View-file breadcrumb: `src/striatum/service.py:618`
- Run detail/job detail/artifact view/posture/doctor/view-file route wiring: `src/striatum/service.py:1443`, `src/striatum/service.py:1562`, `src/striatum/service.py:1656`, `src/striatum/service.py:1699`, `src/striatum/service.py:2375`, `src/striatum/service.py:2925`

## 9. Dashboard Parity

No new dashboard code was needed beyond the V1 parity primitives already present. The V1.5 web additions consume the same status vocabulary the dashboard already renders: sessions, attestation, open blockers, process diagnostic envelopes, override rationales, and next actions.

- `src/striatum/dashboard.py:102` consumes the same `status` payload as the web run page.
- `src/striatum/dashboard.py:103` enriches open blockers with payload envelopes.
- `src/striatum/dashboard.py:310` renders the sessions strip equivalent.
- `src/striatum/dashboard.py:315` renders blocker triage.
- `src/striatum/dashboard.py:643` renders process diagnostic evidence.
- `src/striatum/dashboard.py:675` renders override rationale visibility.
- `src/striatum/dashboard.py:563` renders the shared next-actions vocabulary.

## 10. Tests

Added the required focused V1.5 tests:

- `tests/test_run_detail_recovery_panel.py:11`
- `tests/test_job_detail_expected_artifacts.py:11`
- `tests/test_artifact_view_provenance_trail.py:11`
- `tests/test_posture_verdicts_override_provenance.py:11`
- `tests/test_doctor_per_record_recipes.py:10`
- `tests/test_view_file_breadcrumb_heuristic.py:10`

## Verification

- `python3 -m py_compile src/striatum/service.py src/striatum/dashboard.py tests/test_run_detail_recovery_panel.py tests/test_job_detail_expected_artifacts.py tests/test_artifact_view_provenance_trail.py tests/test_posture_verdicts_override_provenance.py tests/test_doctor_per_record_recipes.py tests/test_view_file_breadcrumb_heuristic.py`
- `pytest tests/test_run_detail_recovery_panel.py tests/test_job_detail_expected_artifacts.py tests/test_artifact_view_provenance_trail.py tests/test_posture_verdicts_override_provenance.py tests/test_doctor_per_record_recipes.py tests/test_view_file_breadcrumb_heuristic.py -q`
- `ruff check src/striatum/service.py src/striatum/dashboard.py tests/test_run_detail_recovery_panel.py tests/test_job_detail_expected_artifacts.py tests/test_artifact_view_provenance_trail.py tests/test_posture_verdicts_override_provenance.py tests/test_doctor_per_record_recipes.py tests/test_view_file_breadcrumb_heuristic.py`
- `mypy src/striatum/service.py src/striatum/dashboard.py`
- `pytest tests/test_service.py tests/test_web_ui.py tests/test_web_ergonomics.py tests/test_web_run_posture_verdicts.py tests/test_dashboard_web_parity.py tests/test_byline_regression.py tests/test_override_rationale_regression.py tests/test_dashboard.py -q -k 'not static_assets_no_external_urls'`
- `git diff --check`

## Notes

- Existing untracked files under `docs/dogfood/055/DESIGN_SYNTHESIS.md` and `docs/dogfood/055/review/` were present before this implementation and were left untouched.
- The known `static_assets_no_external_urls` exclusion remains the same adjacent-web-test exclusion noted in the prior RFC 0050 handoff.
