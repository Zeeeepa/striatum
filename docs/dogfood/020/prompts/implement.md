# Implement RFC 0022 V1

1. `pyproject.toml` adds Jinja2.
2. `src/striatum/web/templates/*.html` — base + 5 pages.
3. `src/striatum/web/static/base.css` — palette + dark mode +
   spacing scale.
4. `src/striatum/web/graph_svg.py` — layered SVG renderer.
5. `src/striatum/service.py` route table refactor + hash-route
   redirect + Jinja2 environment.
6. `tests/test_web_ui_redesign.py` — coverage per synthesis.
7. Doc updates: SPEC, UBIQUITOUS_LANGUAGE (page route +
   theme palette), DECISION_LOG D073, TODO F20, RFC 0022 status
   → `accepted (V1)`, RFC 0013 status note, CHANGELOG ## 1.11.0,
   version bump.
8. `BUILD_HANDOFF.md`.

Constraints: lint / typecheck / test all pass; CSP unchanged.
