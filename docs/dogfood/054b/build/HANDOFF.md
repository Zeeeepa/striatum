# RFC 0050 V1 Fix-Up Handoff

Status: complete
Date: 2026-05-14
author: implementer-unknown-model-001

## F1 - Byline Forgery Loophole

Fixed. The shared byline renderers now refuse to display a disk model byline when the row is explicitly unattested:

- `src/striatum/web/templates/_components.html:72` forces `author: operator` or `author: operator [self-declared: <label>]` when `attested=false`.
- `src/striatum/web/frontend/src/shared/components/BylineLine.tsx:13` applies the same substitution in the React component.
- `src/striatum/service.py:316` applies the same display rule to shaped page data, and `src/striatum/dashboard.py:473` applies it to compact dashboard byline rendering.

Pinned by:

- `tests/test_byline_regression.py:70`
- `src/striatum/web/frontend/src/__tests__/byline-line.test.tsx:7`

## F2 - Verdict Forgery Via Inferred Override

Fixed. The accepting-after-non-accepting heuristic was removed. Missing `verdicts.source` now falls back to `natural`; override provenance is used only when recorded evidence exists.

- `src/striatum/service.py:458` shapes verdict rows without the removed `inferred_override` heuristic.
- `src/striatum/service.py:469` reads `verdict.overridden` events by recorded `verdict_id` so real overrides still render as `operator-override` even on the current schema without a `verdicts.source` column.

Pinned by:

- `tests/test_override_rationale_regression.py:82`
- `tests/test_override_rationale_regression.py:26`

## F3 - Attestation-Drift Honesty Failure

Fixed. Artifact rows no longer recompute live session/supervisor attestation for artifact provenance chips.

- `src/striatum/service.py:278` derives the artifact attestation chip from the recorded `artifacts.author_line`.
- `src/striatum/service.py:519` uses that recorded artifact path in `_shape_artifact_rows`, so a closed or lost live session does not pessimistically rewrite an already-recorded model byline artifact as unattested.

Pinned by:

- `tests/test_byline_regression.py:84`

## F4 - Dashboard Override Rationale Omission

Fixed. Dashboard override chips now carry the operator rationale, truncated to compact width, and the override section keeps the full rationale line visible.

- `src/striatum/dashboard.py:447` adds rationale support to `_verdict_chip`.
- `src/striatum/dashboard.py:681` passes override rationale into the chip and renders a rationale line in the compact override panel.

Pinned by:

- `tests/test_override_rationale_regression.py:116`
- `tests/test_override_rationale_regression.py:26`

## Verification

- `pytest tests/test_byline_regression.py tests/test_override_rationale_regression.py -q`
- `pytest tests/test_dashboard_web_parity.py tests/test_byline_regression.py tests/test_override_rationale_regression.py -q`
- `pytest tests/test_service.py tests/test_dashboard.py tests/test_web_ui.py -q -k 'not static_assets_no_external_urls'`
- `python3 -m py_compile src/striatum/service.py src/striatum/dashboard.py tests/test_byline_regression.py tests/test_override_rationale_regression.py`
- `ruff check src/striatum/service.py src/striatum/dashboard.py tests/test_byline_regression.py tests/test_override_rationale_regression.py`
- `mypy src/striatum/service.py src/striatum/dashboard.py`
- `npm test -- byline-line.test.tsx`
- `npm test`
- `npm run build`
- `git diff --check`
