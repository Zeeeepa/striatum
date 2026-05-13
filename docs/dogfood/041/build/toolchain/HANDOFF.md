# Toolchain Implementation Handoff

author: implementer-codex-gpt-5.5-002
date: 2026-05-12
status: implemented-with-local-node-blocker

## Scope Shipped

Implemented and verified the Python-side RFC 0038 V1 toolchain and Jinja2 integration slice:

- Added the Vite + React + TypeScript frontend scaffold under `src/striatum/web/frontend/` with `package.json`, `package-lock.json`, `vite.config.ts`, `tsconfig.json`, and `.gitignore`.
- Added Makefile targets `ui-install`, `ui-build`, `ui-dev`, `ui-test`, `ui-bundle-hash`, and `ui-check-bundle`.
- Added deterministic bundle hashing in `src/striatum/web/static/build/manifest.sha256`; `ui-build` refreshes it and `ui-check-bundle` diffs the committed build directory.
- Added package-data coverage for `src/striatum/web/static/build/*.js`, `*.css`, and `*.sha256`.
- Added Node 22 CI wiring with npm cache, UI install, UI bundle check, and UI tests.
- Added `GET /v1/repo/tree?path=<rel>` with repo-relative path safety and hidden `.git` / `.striatum` filtering.
- Added `/view` and `/view/` tree-browser shell rendering, `/workflows/new` chooser shell rendering, and island mount points for code viewer, tree browser, workflow chooser, workflow graph editor, and shared bundle loading.
- Promoted the workflow-detail Edit affordance from muted text to a primary button next to "Run this workflow now".
- Added and updated focused Python tests for the repo tree endpoint, tree/code island shells, workflow chooser shell, static build asset resolution, and the bundle manifest.

## Decisions And Notes

The Vite config includes a placeholder-island plugin so the toolchain half can emit stable island bundle filenames before or alongside the component lane's owned React source. When real entry files exist under `src/striatum/web/frontend/src/`, Vite resolves those files normally instead of the placeholders.

`/view` now renders the same tree-browser shell as `/view/` so direct navigation without a trailing slash works.

Two follow-ups are outside this job's allowed write scope:

- Root `.gitignore` currently ignores every `build/` directory, which means `src/striatum/web/static/build/` appears as ignored output locally. The build assets and `manifest.sha256` need either forced tracking or a precise `.gitignore` exception.
- `scripts/package_smoke.sh` excludes `build`, and `.github/workflows/release.yml` does not run the UI bundle check before `python -m build`. Those should be aligned with the CI package-data gate in a follow-up owned by a scope that can edit scripts/release workflow files.

## Verification

Passed:

- `.venv/bin/python -m pytest tests/test_service.py::test_service_repo_tree_lists_safe_directory tests/test_service.py::test_service_repo_tree_refuses_hidden_and_traversal tests/test_web_view.py::test_view_root_renders_tree_browser_island tests/test_web_view.py::test_view_root_without_trailing_slash_renders_tree_browser_island tests/test_web_view.py::test_view_text_renders_pre tests/test_web_workflows.py::test_workflows_new_renders_chooser_island tests/test_web_workflows.py::test_workflows_index_renders -q`
- `.venv/bin/python -m pytest tests/test_web_ui.py::test_static_assets_served_when_web_enabled tests/test_web_ui.py::test_static_assets_no_external_urls tests/test_web_ui.py::test_assets_resolvable_via_importlib_resources tests/test_web_view.py tests/test_service.py::test_service_repo_tree_lists_safe_directory tests/test_service.py::test_service_repo_tree_refuses_hidden_and_traversal tests/test_web_workflows.py::test_workflows_new_renders_chooser_island tests/test_web_workflows.py::test_workflows_index_renders -q`
- `.venv/bin/python -m ruff check src/striatum/service.py src/striatum/web/workflows.py tests/test_web_view.py tests/test_web_ui.py tests/test_service.py tests/test_web_workflows.py`
- `make ui-bundle-hash`

Blocked or not clean:

- `npm run test --prefix src/striatum/web/frontend` failed locally because `vitest` is not installed (`sh: 1: vitest: not found`). Network is forbidden for this packet, so I did not run `npm install`.
- `make ui-build` / `make ui-check-bundle` were not runnable for the same reason: the local frontend dependencies are absent.

## Deferred To Component Lane

The React component source under `src/striatum/web/frontend/src/` is the component lane's scope. I inspected it for integration context but did not edit it.
