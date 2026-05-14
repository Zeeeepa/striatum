# RFC 0050 V1 Implementation Handoff

Status: complete
Date: 2026-05-14
author: implementer-unknown-model-001

## Shipped Scope

- Added the RFC 0050 V1 shared TypeScript component contracts and components under `src/striatum/web/frontend/src/shared/components/`.
- Added `_components.html` Jinja2 macros mirroring the status, verdict, attestation, posture, byline, lane-evidence, and expected-artifact vocabulary.
- Extended `service.py` page shaping for run list, run detail, and job detail with chip-ready state, attestation, verdict provenance, override rationale, expected artifact, byline, and muted lane evidence data.
- Updated run list/detail and job detail templates to render the new macro vocabulary on server-rendered pages.
- Extended `dashboard.py` with text-mode parity for sessions, attestation, operator bylines, verdict overrides, blocker evidence, muted lane evidence, and V1.41 `next_actions` verbs.
- Added semantic CSS tokens and component classes in `static/base.css`, including reserved `--status-compromised`.
- Added regression tests for dashboard/web parity, unattested operator bylines, and override rationale visibility.

## Verification

- `python3 -m py_compile src/striatum/service.py src/striatum/dashboard.py tests/test_dashboard_web_parity.py tests/test_byline_regression.py tests/test_override_rationale_regression.py`
- `ruff check src/striatum/service.py src/striatum/dashboard.py tests/test_dashboard_web_parity.py tests/test_byline_regression.py tests/test_override_rationale_regression.py`
- `mypy src/striatum/service.py src/striatum/dashboard.py`
- `pytest tests/test_dashboard_web_parity.py tests/test_byline_regression.py tests/test_override_rationale_regression.py -q`
- `pytest tests/test_web_ui.py::test_assets_resolvable_via_importlib_resources tests/test_dashboard_web_parity.py tests/test_byline_regression.py tests/test_override_rationale_regression.py -q`
- `pytest tests/test_dashboard.py tests/test_next_actions_v141_burndown.py tests/test_service.py tests/test_web_ui.py -q -k 'not static_assets_no_external_urls'`
- `npm run build`
- `npm test`
- `git diff --check`

## Deviations And Notes

- Full `tests/test_web_ui.py` still has the known `static_assets_no_external_urls` bundle-policy failure around the React Flow namespace URL in `island-workflow-graph-editor.js`; this was already called out by the service worker and is outside RFC 0050 V1.
- `npm run build` removes the committed `static/build/manifest.sha256`; the file was restored after the build so the existing importlib-resource asset test passes.
- Current schema has no `verdicts.source` column. The render path tolerates future/source-shaped rows and infers an operator override when an accepting verdict follows a non-accepting verdict for the same review job.
- V1 keeps `LaneEvidenceChip` muted as `not_yet_correlated`; no green path evidence claim is emitted.

## Follow-Ups

- V1.5 should move the fuller recovery panel, expected-artifact table placement, artifact-view byline integrity, and posture verdict provenance columns into their dedicated screens.
- V1.7 should replace the muted lane-evidence placeholder with the path-specific `process_executions` correlation once that lookup ships.
