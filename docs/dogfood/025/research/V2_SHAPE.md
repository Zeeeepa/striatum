---
title: "V2 shape research: run-now + field-level errors + If-Match"
date: 2026-05-09
---

# V2 shape research

author: researcher-codex-gpt-5.5-001

## (1) `run prepare` + `run start` lifecycle from a file path

### CLI surface

```
striatum run prepare --workflow <path>   # creates run, optionally auto-confirms branch
striatum run start --run-id <id>         # transitions ready→running, enqueues roots
```

### Underlying functions

- `striatum.workflow.create_run(conn, *, repo, workflow_path) -> {run_id, state, branch_mode, suggested_branch_name, ...}` — `cli/dispatch.py:264`. Loads the workflow file, validates, INSERTs run + jobs + edges + dependencies, returns prepared payload.
- `striatum.cli.mutations.branch_confirm(conn, *, repo, run_id, branch, create=False, use_current=False, strict=False)` — `cli/mutations.py:38`. When `branch.mode == "auto"` and the runner can `git checkout`, dispatch.py auto-drives this; otherwise the run sits in `needs_branch_confirmation` until the operator confirms.
- `striatum.cli.mutations.run_start(conn, *, run_id) -> {run_id, state}` — `cli/mutations.py:185`. Transitions `ready → running`, enqueues root jobs, emits `run.started`.

### Idempotency

- `create_run` is **not** idempotent: each call inserts a new run row. If the operator clicks "run" twice, two runs are created. V2 needs to either (a) tolerate that (each click → fresh run), or (b) introduce a "preparing" lock per workflow path. **Recommendation: (a)** — each click is a fresh run; the operator can cancel from the dashboard. Documenting in CHANGELOG.
- `run_start` is idempotent: if the run is already `running`, it short-circuits and just returns `{state: running}` without re-enqueuing roots.

### Branch hygiene

`branch_confirm(create=True)` will refuse to switch branches if the working tree is dirty. The operator must commit or stash first. From the web surface, this means `POST /workflows/run/<path>` may return 409 (or 422 with a structured error) when the tree is dirty. The synthesis should pin the response code.

### "Run-now" minimal contract

```
POST /workflows/run/<path>
→ 200 {"ok": true, "data": {"run_id": "run_..."}}
→ 405 if --allow-mutations is off
→ 422 if validate_workflow fails
→ 409 if branch_confirm errors (dirty tree, etc.)
```

The handler:
1. Path-safe resolves `<path>` (mirrors `/workflows/edit/<path>` guards).
2. Calls `create_run` (which validates).
3. If `branch_mode == "auto"`, drives `branch_confirm(create=True)`; on `BranchConfirmationError`, returns 409.
4. Calls `run_start`.
5. Returns the run_id.

## (2) Field-level errors from `validate_workflow`

### Current shape

`WorkflowError(message: str)` — `errors.py:49`. Single string. **No field path.** Every `raise WorkflowError(...)` site in `workflow.py:160-560` is plain string interpolation:

```python
raise WorkflowError(f"job {job_id!r} references unknown role {role_id!r}")
raise WorkflowError(f"job {job_id!r} expected artifact must be an object")
raise WorkflowError("workflow schema_version must be striatum.workflow.v1")
```

Roughly **30 raise sites**, each with implicit field knowledge in the f-string.

### Options for V2

**Option A: Extend `WorkflowError` with optional `field_path`.**
- Add `field_path: str | None = None` to `WorkflowError.__init__`.
- Update each raise site (~30 calls) to pass the field path explicitly.
- Backward-compat: `field_path=None` falls back to V1 behavior.
- Effort: ~1-2 hours of mechanical edits + tests.

**Option B: Wrapper that re-walks the workflow on failure.**
- `validate_workflow_with_field_errors(workflow) -> list[FieldError]` re-implements key checks but yields `(field_path, message)` tuples.
- No change to the existing surface.
- Risk: drift between the two validators.

**Option C: Heuristic regex on the error message.**
- Service-layer parses the error string for `job 'X'`, `field 'Y'` etc. and synthesizes a field path.
- Brittle. Reject.

**Recommendation: Option A.** Mechanical, no drift, surface stays a single function. The 30 raise sites are well-isolated; sed or a single editor pass updates them. The synthesis should pin which raise sites get which field paths.

### Field-path conventions

Use dotted JSON-pointer-ish paths consistent with the form structure:
- `schema_version`
- `jobs[2].role_id` (job index)
- `jobs[2].expected_artifacts[0].path`
- `lanes.codex.command`
- `cycles[0].max_iterations`

Editor JS reads `errors[]` from the 422 body and queries the form via `[data-field-path="jobs[2].role_id"]`, adds an `error` class + tooltip.

### Backward-compat shape

```json
{
  "ok": false,
  "error": {
    "code": 422,
    "message": "workflow validation failed",
    "errors": [
      {"field_path": "jobs[2].role_id", "message": "job 'review_v2' references unknown role 'reviewre'"}
    ]
  }
}
```

V1.5 callers that read `error.message` continue to work. New callers read `error.errors[]`.

## (3) sha256 capture for If-Match precondition

### Capture surface

`_render_workflow_edit_page` at `service.py:931` already loads the file (or scaffolds an empty one). To compute sha256:

```python
import hashlib
sha = hashlib.sha256(target.read_bytes()).hexdigest() if target.is_file() else ""
```

Inject as a hidden script tag (alongside `workflow-data`):

```html
<script id="workflow-sha256" type="application/json">"abc123..."</script>
```

The editor JS reads it once on load and echoes it in the POST as `If-Match: <sha>`.

### POST handler

Read `If-Match` header; if present, recompute the file's sha256 *under the same atomic-write block* (read sha → validate → check sha == If-Match → write `.tmp` → rename). The check happens *after* validation and *before* the rename.

```python
if_match = self.headers.get("If-Match")
if if_match is not None and target.is_file():
    current_sha = hashlib.sha256(target.read_bytes()).hexdigest()
    if current_sha != if_match.strip('"'):
        self._send_json(412, {"ok": False, "error": {"code": 412, "message": "If-Match precondition failed; file changed on disk", "current_sha256": current_sha}})
        return
```

### Missing `If-Match` semantics

V1.5 callers don't send the header. **Treat missing as opt-out**: backward-compatible. Editors that *do* send it get safe concurrency. An attacker spoofing missing header isn't a threat we model here (single-operator local-first).

### TOCTOU note

There is a race between the sha read and the rename. To minimize: read sha, validate, re-read sha *immediately before* `tmp.replace(target)`, abort if changed. We can mitigate further with `flock(target)` but that adds platform complexity; V2 sticks with the read-twice pattern.

## V2 scope summary

| Surface | Action |
| --- | --- |
| `src/striatum/errors.py` | Extend `WorkflowError` with optional `field_path` |
| `src/striatum/workflow.py` | Update ~30 raise sites to include field paths |
| `src/striatum/service.py` | Add POST `/workflows/run/<path>`; extend POST `/workflows/edit/<path>` with If-Match + structured errors; extend GET to inject sha256 |
| `src/striatum/web/templates/workflow_edit.html` | Render sha256 hidden tag; field error containers |
| `src/striatum/web/static/workflow_edit.js` | Echo If-Match; render field-level errors; "Run now" button on detail page |
| `src/striatum/web/templates/workflow_detail.html` | "Run now" button (POST form or fetch) |
| `tests/test_web_workflow_edit.py` | If-Match cases (matching, stale, missing) |
| `tests/test_web_workflow_run.py` (new) | Run-now route tests |
| `tests/test_workflow_field_errors.py` (new) | validate_workflow yields field paths |

## Test precedent

Reuse the spawn-server helpers in `tests/test_web_workflow_edit.py` and `tests/test_web_workflows.py` for HTTP tests. Use `tmp_path` + a workflow.json fixture for run-now lifecycle tests; check that the run row appears in `runs` after POST.
