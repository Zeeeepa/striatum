# Research: web redesign

Map:
1. `src/striatum/service.py` route table + handler shape
   (`do_GET`, dispatch on `path`).
2. `src/striatum/workflow.py:workflow_graph_data` — node/edge shape.
3. `src/striatum/web/static/app.css` — current palette / spacing.
4. Test precedent: `tests/test_web_ui.py` patterns.
5. Jinja2 `PackageLoader` pattern + `select_autoescape(['html'])`.

Deliverable: `research/REDESIGN_SHAPE.md`.
