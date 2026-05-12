author: implementer-codex-gpt-5.5-001

# RFC 0037 Build Handoff

Status: implemented
Date: 2026-05-12

## Summary

Implemented the accepted RFC 0037 V1 web UI ergonomics slice as
progressive enhancements over the existing server-rendered Jinja2 pages.
No runtime dependency, route, API, MCP, CSP, mutation-gate, SPA, or visual
redesign change was introduced.

Shipped:

- Header `UTC` / `Local` timestamp toggle, skip link, and keyboard shortcut
  help dialog.
- Run list search/state/date filters, persisted in `localStorage`, plus
  workflow id, created time, duration column, and true/filtered empty states.
- Workflow index search/status filters, persisted in `localStorage`, plus
  UTC last-modified timestamps and empty states.
- Doctor problem grouping by check kind, collapsible groups, terminal-run
  hide toggle, doc links, and empty states.
- Run-detail next-actions banner for non-terminal runs.
- Graph node tooltip metadata in SVG plus `run_detail.js` hover/focus tooltip.
- `app.css` dark-mode parity block for the RFC-listed app-specific selectors.
- Documentation updates in `HOW_TO_HUMAN.md`, `CHANGELOG.md`, `README.md`,
  `UBIQUITOUS_LANGUAGE.md`, and RFC 0037 status.

## Files Changed

Core/server data:

- `src/striatum/service.py`
- `src/striatum/web/workflows.py`
- `src/striatum/web/graph_svg.py`

Templates and static UI:

- `src/striatum/web/templates/base.html`
- `src/striatum/web/templates/run_list.html`
- `src/striatum/web/templates/workflows_index.html`
- `src/striatum/web/templates/doctor.html`
- `src/striatum/web/templates/run_detail.html`
- `src/striatum/web/static/base.css`
- `src/striatum/web/static/app.css`
- `src/striatum/web/static/base.js`
- `src/striatum/web/static/run_list.js`
- `src/striatum/web/static/workflows_index.js`
- `src/striatum/web/static/doctor.js`
- `src/striatum/web/static/run_detail.js`

Tests and docs:

- `tests/test_web_ui.py`
- `tests/test_web_ui_redesign.py`
- `tests/test_web_workflows.py`
- `tests/test_web_ergonomics.py`
- `docs/HOW_TO_HUMAN.md`
- `docs/rfcs/0037-web-ui-ergonomic-improvements.md`
- `docs/UBIQUITOUS_LANGUAGE.md`
- `README.md`
- `CHANGELOG.md`

## Tests Added Or Updated

Added `tests/test_web_ergonomics.py` for RFC 0037 cross-page output and
static asset assertions. Extended existing web tests for new static JS
assets, CSP coverage, workflow `modified_at`, workflows index markup,
run-list empty state, and app CSS dark-mode selector coverage.

## Verification

Commands run:

- `pytest tests/test_web_ui.py tests/test_web_ui_redesign.py tests/test_web_workflows.py tests/test_web_ergonomics.py`
  - Result: `49 passed in 35.21s`
- `make install`
  - Result: no work needed after `make lint` installed the editable package.
- `make lint`
  - Result: `All checks passed!`
- `make typecheck`
  - Result: `Success: no issues found in 146 source files`
- `make test`
  - Result: `637 passed, 31 skipped in 404.05s`
- `make smoke`
  - Result: exit 0. The smoke script emitted existing workflow `needs`
    deprecation warnings.

Manual checklist:

- Keyboard shortcut input-focus guard: implemented in `base.js` via
  `isEditableTarget()` for inputs, textareas, selects, buttons,
  contenteditable elements, and open dialogs; covered by static assertions.
- Help overlay dismissal: implemented with native `<dialog>`, close button,
  Esc behavior, and outside-click close; covered by template/static checks.
- Localtime rewriting: implemented for all `time[datetime]` elements on the
  touched pages; route tests assert timestamp markup is present.
- Graph tooltip positioning: implemented with a single fixed-position
  tooltip, viewport clamping, hover, mousemove, focus, and blur handlers;
  route tests assert SVG data hooks and script wiring.
- Dark-mode parity visual check: CSS selector coverage is automated; no
  browser screenshot was captured in this session.

## Delegation

Used native sub-agents for parallel read-only exploration:

- Web template/service integration notes.
- Existing web test and package-data test locations.
- CSS variable and dark-mode parity audit.
- Documentation insertion points and doc-link risks.

The parent session performed all file edits, integration, verification, and
this handoff artifact.

## Deferred Items

Deferred per accepted design scope:

- Filter state in query strings.
- Sticky next-actions banner.
- Keyboard shortcut configurability.
- Graph zoom/pan.
- Drag-and-drop workflow editing.
- Hosted-mode UX.
- New API/MCP/chat behavior.
