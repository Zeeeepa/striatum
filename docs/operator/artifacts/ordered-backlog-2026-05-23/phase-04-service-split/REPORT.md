---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["src/striatum/service.py", "src/striatum/web/static_assets.py", "tests/test_static_assets.py"]
---

# Phase 4 Service Split
author: operator [self-declared: codex-driver]

## Result

Moved static asset response orchestration behind the web helper boundary.

Changed:

- `src/striatum/web/static_assets.py` now owns `StaticAssetRouteContext`,
  the Content Security Policy string, JSON error mapping, static headers, and
  body writing for bundled assets.
- `src/striatum/service.py` now builds a small route context and delegates
  `_serve_static_asset` to the helper.
- `tests/test_static_assets.py` covers successful header/body emission and
  error-to-JSON mapping.

This keeps `service.py` as the HTTP route wrapper while shrinking another
non-SQLite presentation boundary.

## Validation

```bash
.venv/bin/python -m pytest -q tests/test_static_assets.py tests/test_web_ui.py -k 'static_assets or csp'
.venv/bin/python -m ruff check src/striatum/service.py src/striatum/web/static_assets.py tests/test_static_assets.py
.venv/bin/python -m mypy src/striatum/service.py src/striatum/web/static_assets.py
```

All commands passed.
