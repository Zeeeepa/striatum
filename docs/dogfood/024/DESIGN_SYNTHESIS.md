# Design synthesis: RFC 0024 V1.5 (visual builder)

author: designer-claude-opus-001
date: 2026-05-09

## Scope

Form-driven editor at `/workflows/edit/<path>` per RFC 0024 V1.5. JS island manages state; save POSTs JSON to a new endpoint that validates + writes. Includes "+ New Workflow" entry on the index page.

## 1. Architecture: JS-island + JSON POST

Client-side state model mirrors the workflow JSON itself:

```javascript
state = {
    workflow_id: "...",
    workflow_version: "...",
    name: "...",
    branch: { mode: "auto", suggested_name: "...", allow_dirty: false },
    coordinator: { role_id: "...", lane_id: "..." },
    lanes: { lane_a: {...}, ... },
    roles: { role_a: {...}, ... },
    jobs: [ {...}, ... ],
    edges: [ {...}, ... ],
    cycles: [ {...}, ... ],
    parallelism: { ... },
    context_docs: [ ... ],
    schema_version: "striatum.workflow.v1",
}
```

The page is a container template. JS reads the initial state from a `<script id="workflow-data" type="application/json">...</script>` tag the server emits, then renders form sections.

### Render functions per section

- `renderHeader(state)` — workflow_id + version + name (text inputs).
- `renderRoles(state)` — table of roles (id + definition_path); add/remove buttons.
- `renderLanes(state)` — table of lanes (id + adapter + display_model + capabilities); add/remove.
- `renderJobs(state)` — collapsible per-job cards. Within each:
  - id (text), type (select), role_id (select from roles), lane_id (select from lanes).
  - objective (textarea), task_prompt path (text).
  - write_scope: mode (select), allowed_paths (list), forbidden_paths (list).
  - expected_artifacts: list of {logical_name, kind, path, required}.
  - Review-only: review_posture (select with closed set + custom: input), reviewer_access_scope, reviewer_context_policy, fresh_session_required.
  - Build-only: required_review_postures (multi-select).
- `renderEdges(state)` — table of edges (from select, to select, on, requires_verdict).
- `renderCycles(state)` — table of cycles (from, to, on_verdict, max_iterations).

Each render writes HTML into its named `<section id="...">` container; event listeners are attached after render. State mutations (add row, edit field, remove row) call the appropriate render function to update the UI.

### Save action

```javascript
async function save() {
    const resp = await fetch(`/workflows/edit/${encodeURIComponent(path)}`, {
        method: "POST",
        headers: {"Content-Type": "application/json"},
        body: JSON.stringify(state),
    });
    if (resp.status === 200) {
        window.location.href = `/workflows/${encodeURIComponent(path)}`;
    } else if (resp.status === 422) {
        const body = await resp.json();
        showErrorBanner(body.error.message);
    } else {
        showErrorBanner(`Save failed (${resp.status})`);
    }
}
```

State is preserved on validation failure (no page reload).

## 2. Endpoint contract

### `GET /workflows/edit/<path>`

- Path safety: `..`, leading `/`, null bytes, hidden dirs → 400/404.
- If file exists: load it (parsed dict — invalid is OK; the editor renders the user's intent regardless of validation status).
- If file doesn't exist: render the editor with an empty workflow scaffold (allowing operators to author new workflows by visiting an unused path).
- Renders `workflow_edit.html` with the JSON in a `<script>` tag.

### `POST /workflows/edit/<path>`

- Mutation-gated: 405 without `--allow-mutations`.
- Path safety: same as GET.
- Body: `Content-Type: application/json`; full workflow dict.
- Run `validate_workflow(body)`; on `WorkflowError`, return 422 with `{ok: false, error: {code: 422, message: "..."}}` — the JS island shows the message in an error banner.
- On success: write the file (atomic rename: write to `<path>.tmp`, then rename), respond 200 with `{ok: true}`.
- Operator's input state stays in the JS island; on success the JS redirects to detail page.

## 3. Empty workflow scaffold

When editing a non-existent path, start with:

```json
{
  "schema_version": "striatum.workflow.v1",
  "workflow_id": "<derived from path stem>",
  "workflow_version": "1",
  "name": "",
  "branch": {"mode": "confirm", "suggested_name": "wf/new", "allow_dirty": false},
  "coordinator": {"role_id": "", "lane_id": ""},
  "lanes": {},
  "roles": {},
  "context_docs": [],
  "parallelism": {"mode": "declared", "max_active_jobs": 1, "require_disjoint_write_scopes": true},
  "jobs": [],
  "edges": [],
  "cycles": []
}
```

`workflow_id` is derived from the parent dir name (e.g., `examples/foo/workflow.json` → `foo`). Operator can edit it.

## 4. Index-page additions

`workflows_index.html` adds a `+ New Workflow` button (mutation-gated) that prompts for a path (e.g., `examples/my-flow/workflow.json`) and navigates to `/workflows/edit/<that-path>`. Server creates dirs as needed on save.

## 5. Error rendering

V1.5 renders the WorkflowError message as a top-of-form banner. Field-level highlighting deferred to V1.6 (would require restructuring `validate_workflow` to yield typed errors).

The banner is dismissible (`<button>` clears the banner; state persists).

## 6. localStorage backup

V1.5 persists state to `localStorage[workflow_edit_<path>]` on every change. On page load, if a draft exists AND its serialized form differs from the on-disk content, show a "Recover unsaved changes?" banner.

This protects against accidental tab close + browser crashes. ~30 LoC of JS.

## 7. Test plan

`tests/test_web_workflow_edit.py` (new):

1. `test_edit_get_existing_renders_form` — GET an existing valid workflow → 200 with the data script tag.
2. `test_edit_get_invalid_renders_form` — GET an invalid workflow → still 200 (editor opens; user can fix).
3. `test_edit_get_nonexistent_path_returns_scaffold` — GET a non-existent path → 200 with scaffold.
4. `test_edit_post_valid_writes_file` — POST a valid workflow → 200; file on disk matches.
5. `test_edit_post_invalid_returns_422` — POST an invalid workflow → 422; file unchanged.
6. `test_edit_post_without_mutations_405` — `--allow-mutations` not set → 405.
7. `test_edit_post_path_traversal_400` — `/workflows/edit/../../etc/passwd` → 400.
8. `test_edit_post_creates_intermediate_dirs` — POST to a path whose parent dir doesn't exist → creates it.
9. `test_edit_post_atomic_write` — assert `.tmp` file pattern is used (or simply that on validation failure the file is unchanged).
10. `test_edit_link_on_detail_page` — `/workflows/<path>` includes `Edit` link.
11. `test_index_new_workflow_button_when_mutations` — index page renders the button only when allow-mutations.

## 8. Documentation surface

- `docs/SPEC.md` § "Local Web UI" — extend with `/workflows/edit/<path>`.
- `docs/UBIQUITOUS_LANGUAGE.md` — `edit draft`, `edit state`.
- `docs/DECISION_LOG.md` — D077.
- `docs/TODO.md` — F24.
- `docs/rfcs/0024-workflow-browser-and-builder.md` — status `accepted (V1+V1.5)`.
- `docs/rfcs/README.md` — index updated.
- `CHANGELOG.md` — `## 1.15.0 — 2026-05-09`.
- `pyproject.toml` + `__init__.py` — bump to `1.15.0`.

## 9. Out of scope (V2)

- Drag-and-drop graph editor.
- Workflow templates / marketplace.
- "Diff against another workflow" view.
- "Run this workflow now" full lifecycle button.
- Field-level error highlighting (requires `validate_workflow` API change).
- `If-Match: <sha256>` precondition for safe concurrent edits.
- AI-assisted scaffolding (chat tool that *writes* workflow.json).

## 10. Zero-regression contract

- Without visiting `/workflows/edit/<path>`: behavior unchanged from v1.14.0.
- The detail page's existing tabular display is untouched (the "Edit" link is additive).
- The existing `list_workflows` chat tool is unchanged.
- CSP byte-identical (the JS island uses fetch, no `unsafe-inline` needed).
