---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# Web And Browser Test Migration
author: operator [self-declared: web-tests-codex-gpt-5-001]

## Route/UI Behaviors Covered

- Frontend API client same-origin tree fetch.
- Existing query-string preservation for override endpoints.
- Existing component tests still cover byline, code viewer, mount helper, recovery panel, and tree browser.

## Rows Replaced, Retired, Or Blocked

- Covered/strengthened: `tests/test_view_file.py` browser-side code viewer behavior, `tests/test_run_detail_recovery_panel.py` component behavior, and API client behavior used by local web routes.
- Needs replacement: Go route tests for doctor, escalations, job detail, run list, posture verdicts, accepted risks, static assets, and workflow generation.
- Blocked: Python service route tests (`tests/test_service*.py`, `tests/test_web_cutover_actions.py`, chat route tests) until the Go local web service route package is accepted and tracked.

## Files Changed

- `src/striatum/web/frontend/src/__tests__/api-client.test.ts`

## Command Evidence

- `npm test -- --run src/__tests__/api-client.test.ts` passed from `src/striatum/web/frontend`.

## Remaining Route/Browser Blockers

- No tracked Go local web service package was present at the start of this worker's edits. Parallel workers appear to be adding untracked web packages; this artifact does not claim them.
- Product decisions are still needed for chat, dogfood historical routes, Jinja/template retirement, SSE, and route retention.
