# Research: workflow visual-builder shape

author: researcher-codex-gpt-5.5-001
date: 2026-05-09

## Form-serialization architecture options

### A. Structured form fields (`jobs[0][id]=...`)

Server parses the URL-encoded body into a nested dict on POST. Add/remove rows via re-submission with an action field; server adds an empty slot and re-renders.

- Pros: no JS state; works without JavaScript.
- Cons: every add/remove is a full page reload; for a 10-job workflow this is a lot of round-trips. Server has to track "which slot is empty" itself.

### B. JS island that serializes form state to JSON

JS reads the workflow from a `<script type="application/json">` tag, builds an in-memory state object, renders forms from that state, and on save POSTs the full state as JSON.

- Pros: snappy add/remove without round-trips; clean save semantics; no per-row submission.
- Cons: ~400 LoC of JS state mgmt (still well under the supervisor wrapper or chat island).

### C. JSON textarea fallback

Single `<textarea>` with the workflow JSON; "Save" parses + validates.

- Pros: trivial to implement.
- Cons: defeats the point of a "visual" builder; this is just web-Vim.

### Recommendation: **B**.

JS island manages state. State shape mirrors the workflow JSON itself (roles dict, lanes dict, jobs list, edges list, cycles list). Render functions per section. On save, fetch POSTs `Content-Type: application/json` body to `/workflows/edit/<path>`. Server validates. On 200, redirect to detail page. On 422, render errors inline.

## CSP compatibility

`script-src 'self'` allows fetch() POST with `application/json` body. The chat island already does this (`/chat/<id>/send`). No new CSP relaxation needed.

## validate_workflow error shape

`validate_workflow(data)` raises `WorkflowError` with a single string message — no field-path metadata. Inline-per-field errors require parsing the message ("review job 'X' has unknown reviewer_access_scope 'Y'") to extract identifiers.

V1.5 ships:
- Top-of-form error banner with the full WorkflowError message.
- Heuristic field highlighting: scan the error for known job_id, role_id, lane_id substrings; if found, highlight that row.
- V2 could refactor `validate_workflow` to yield structured errors with field paths.

## Data loss on validation failure

Critical UX concern. If the operator types for 30 minutes, saves, validation fails, do they lose work? Three guards:

1. **Save returns 422 with the same payload echoed back** — so JS keeps the user's state intact and renders errors.
2. **No page reload on save** — the JS island handles save via fetch; server-side re-renders are not used.
3. **localStorage backup** — JS persists state to localStorage every N seconds. On page load, if a draft exists for this path, prompt to recover. Cheap; ~30 LoC.

V1.5 ships (1) + (2). (3) is a V1.5 nice-to-have if scope allows.

## Mutation gate

`POST /workflows/edit/<path>` requires `--allow-mutations`. Pre-existing pattern from chat / verdict mutations.

## Concurrency model

Last-writer-wins. No lockfile in V1.5. The synthesis acknowledges this; V2 could add a `If-Match: <sha256>` precondition header.

## Path safety

Reuse `/view/<path>` and `/workflows/<path>` rules. Refuse `..`, leading `/`, null bytes, hidden dirs. Edit endpoint allows creating new files (write to a non-existent path) but only inside the repo.

## Test precedent

- `tests/test_web_workflows.py` (V1) — extend for the edit page.
- New `tests/test_web_workflow_edit.py` for save-success / save-validation-error / mutation-gate / path-safety / new-file-creation.

## V1.5 surface summary

| Surface | Action |
| --- | --- |
| `src/striatum/web/templates/workflow_edit.html` (new) | Container + embedded `<script type="application/json">` |
| `src/striatum/web/static/workflow_edit.js` (new) | State mgmt + render + fetch save |
| `src/striatum/web/static/base.css` | Edit-form styling |
| `src/striatum/service.py` | GET + POST routes |
| `src/striatum/web/templates/workflow_detail.html` | Add "Edit" link |
| `src/striatum/web/templates/workflows_index.html` | Add "+ New Workflow" button (mutation-gated) |
| `tests/test_web_workflow_edit.py` (new) | Coverage |
