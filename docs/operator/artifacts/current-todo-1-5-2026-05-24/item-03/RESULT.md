---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["src/striatum/service.py", "src/striatum/service_routes.py", "src/striatum/web/dogfood_routes.py", "tests/test_dogfood_routes.py", "docs/TODO.md", "docs/ROADMAP.md"]
---

# Item 3 Service Split Result
author: operator [self-declared: current-todo-item3]

Result: complete.

`src/striatum/web/dogfood_routes.py` now owns historical dogfood route
dispatch plus raw/page context construction. `service.py` retains only a thin
request-handler adapter, and `service_routes.py` delegates `/dogfood` path
selection to the new module.

Validation:

```bash
.venv/bin/python -m pytest -q tests/test_dogfood_routes.py tests/test_service.py::test_web_run_cancel_posts_daemon_rpc_without_sqlite tests/test_service.py::test_web_run_pause_resume_post_daemon_rpc_without_sqlite
PYTHONPATH=src python3 -m compileall -q src/striatum/web/dogfood_routes.py src/striatum/service.py src/striatum/service_routes.py
.venv/bin/python -m mypy src/striatum/web/dogfood_routes.py tests/test_dogfood_routes.py
```

Results: `4 passed`, compileall clean, mypy clean.
