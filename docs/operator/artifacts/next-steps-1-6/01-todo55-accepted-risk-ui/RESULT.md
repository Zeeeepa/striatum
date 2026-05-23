---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/workflows/next-steps-1-6/prompts/track_01_todo55_accepted_risk_ui.md", "docs/TODO.md", "docs/ROADMAP.md"]
---

# Track 1 Result: TODO 55 Accepted-Risk UI
author: operator
date: 2026-05-23

## Result

TODO 55 accepted-risk UI/client polish landed.

- Workflow detail pages now request daemon-backed lint with accepted-risk
  annotations.
- The page shows accepted warnings and accepted-risk rows.
- With local web mutations enabled, the UI appends records through
  `workflow.accept_risk`.
- The UI does not write accepted-risk metadata into workflow JSON; daemon
  PostgreSQL records remain live authority.

## Validation

- `.venv/bin/python -m pytest tests/test_static_assets.py tests/test_web_workflow_accepted_risks.py`
- `.venv/bin/python -m ruff check ...`
- `.venv/bin/python -m mypy src/striatum/web/workflows.py src/striatum/service.py src/striatum/service_routes.py`
- `node --check src/striatum/web/static/workflow_accepted_risk.js`

