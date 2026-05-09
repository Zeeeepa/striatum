---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# RFC 0024 V2 design

author: designer-codex-gpt-5.5-002

## Scope

Three editor additions, all on the `/workflows/*` surface. Drag-and-
drop, templates, and AI-assisted scaffolding remain V3.

1. **Run-now lifecycle** — `POST /workflows/run/<path>` lifts a
   workflow file into a fresh run via `create_run + branch_confirm +
   run_start`.
2. **Field-level validation errors** — extend `WorkflowError` to carry
   an optional `field_path`; surface as a structured `errors[]` list
   in the 422 body; editor JS highlights the offending fields.
3. **`If-Match: <sha256>` concurrency guard** — GET stamps the disk
   sha into a hidden `<script>` tag; editor echoes it on POST; on
   stale sha, server returns 412 with `current_sha256` in the body.

All three are mutation-gated and respect existing path-safety guards.

## Run-now contract

### Endpoint

```
POST /workflows/run/<path>
Content-Type: application/json
Body: {} (V2; reserved for future overrides)
```

### Response shapes

| Code | Condition | Body |
| --- | --- | --- |
| 200 | Run created and started | `{"ok": true, "data": {"run_id": "run_..."}}` |
| 405 | Mutation gate off | `{"ok": false, "error": {"code": 405, "message": "run requires --allow-mutations"}}` |
| 400 | Path traversal / null bytes | `{"ok": false, "error": {"code": 400, "message": "invalid path"}}` |
| 404 | Hidden / missing path | `{"ok": false, "error": {"code": 404, "message": "..."}}` |
| 415 | Wrong Content-Type | `{"ok": false, "error": {"code": 415, "message": "Content-Type must be application/json"}}` |
| 422 | `validate_workflow` fails | `{"ok": false, "error": {"code": 422, "message": "...", "errors": [...]}}` |
| 409 | `branch_confirm` rejects (dirty tree, conflicting branch) | `{"ok": false, "error": {"code": 409, "message": "..."}}` |

### Idempotency

Each POST creates a fresh run row. **Operators clicking "Run" twice
get two runs.** This is documented; the dashboard shows both. V2 does
*not* introduce a "preparing" lock per workflow path — the cost is
not worth the complexity for V2.

### Branch hygiene

When `branch.mode == "auto"`, the handler drives
`branch_confirm(create=True)`. On `BranchConfirmationError` (dirty
tree, etc.), the handler returns 409 with the original message. The
operator sees a banner: "Cannot start run: working tree has
uncommitted changes" and clears it manually before retrying.

When `branch.mode == "manual"`, the handler stops at the
`needs_branch_confirmation` state and returns 200 with the run_id +
a `requires_branch_confirmation: true` flag. The web UI (V3) could
surface a confirmation widget; V2 just shows the dashboard link.

## Field-level errors

### `WorkflowError` extension

```python
class WorkflowError(StriatumError):
    def __init__(self, message: str, *, field_path: str | None = None) -> None:
        super().__init__(message, exit_code=8)
        self.field_path = field_path
```

Backward compatible: existing call sites pass no `field_path`,
`field_path` is `None`, behavior unchanged. New call sites add
`field_path="jobs[2].role_id"` etc.

### Field-path conventions

Use form-friendly paths consistent with the editor's data attributes:

| Construct | Path |
| --- | --- |
| Top-level field | `schema_version`, `workflow_id` |
| Job field | `jobs[<index>].id`, `jobs[<index>].role_id` |
| Job artifact | `jobs[<index>].expected_artifacts[<index>].path` |
| Lane | `lanes.<lane_id>.command` |
| Cycle | `cycles[<index>].max_iterations` |

The editor JS reads each error and queries the form via:
```js
document.querySelector('[data-field-path="jobs[2].role_id"]')
```

### Required raise-site updates

Update **at minimum** the high-traffic raise sites in
`src/striatum/workflow.py`:

| Line | Current | New `field_path` |
| --- | --- | --- |
| 486 | `workflow is missing required fields` | first missing field name |
| 488 | `workflow schema_version` | `schema_version` |
| 517 | `duplicate job id` | `jobs[<index>].id` |
| 521 | `references unknown role` | `jobs[<index>].role_id` |
| 524 | `references unknown lane` | `jobs[<index>].lane_id` |
| 534 | `invalid artifact path` | `jobs[<index>].expected_artifacts[<j>].path` |
| 555 | `cycle references unknown job` | `cycles[<index>].from` (or `.to`) |
| 559 | `max_iterations` | `cycles[<index>].max_iterations` |

Other raise sites (lane constraints, harness profiles) keep their
current string-only behavior in V2 — `field_path` stays `None` so
the editor falls back to the top-of-form banner. V3 finishes the
sweep.

### 422 body shape

```json
{
  "ok": false,
  "error": {
    "code": 422,
    "message": "<the original WorkflowError str>",
    "errors": [
      {"field_path": "jobs[2].role_id", "message": "job 'review' references unknown role 'reviewre'"}
    ]
  }
}
```

The current V1.5 body shape (`{ok: false, error: {code, message}}`)
is preserved. New `errors[]` is additive. V1.5 clients keep working.

The 422 carries a single error in V2 (the first one
`validate_workflow` raises). Multi-error reporting is V3.

### Editor rendering

On 422:
- Read `body.error.errors[]`. For each entry, find the form element
  via `[data-field-path="<path>"]`, add `error` class, set
  `title=<message>`.
- If `errors[]` is empty (raise-site lacks `field_path`), fall back
  to the V1.5 top-of-form banner showing `error.message`.
- Always also show the top-of-form banner with the V1 message — that
  is the operator's "what failed?" answer; field highlighting is the
  "where?" answer.

## If-Match concurrency guard

### GET response

`_render_workflow_edit_page` reads the file's sha256 and injects:

```html
<script id="workflow-sha256" type="application/json">"a1b2c3..."</script>
```

For new (non-existent) workflows the sha is `""`.

### POST handler

```python
if_match = self.headers.get("If-Match", "").strip().strip('"')
if if_match and target.is_file():
    current_sha = hashlib.sha256(target.read_bytes()).hexdigest()
    if current_sha != if_match:
        self._send_json(412, {"ok": False, "error": {
            "code": 412,
            "message": "If-Match precondition failed; file changed on disk",
            "current_sha256": current_sha,
        }})
        return
```

Empty / missing `If-Match` → opt-out (V1.5 behavior, backward
compatible).

`If-Match: ""` (explicit empty) for a new file → match-by-design (the
file *did not exist*); accept.

### TOCTOU mitigation

Read sha → validate → re-read sha *immediately before* the
`tmp.replace(target)` rename, abort with 412 if changed mid-flight.
Single-process local-first means the race window is microseconds and
the file-system rename is atomic on POSIX. Acceptable for V2.
`flock(target)` is V3 if a real concurrency need surfaces.

### Editor flow

1. Page loads → `sha = JSON.parse(scriptTag.textContent)`.
2. User edits.
3. Save → `fetch(POST, headers: {"Content-Type": "...", "If-Match": '"'+sha+'"'})`.
4. On 412 → display recovery banner: "File changed on disk. Reload to
   merge?" with a "Reload" button. localStorage backup retained until
   user resolves.

## "Run now" UX

On the workflow detail page (`/workflows/<path>`), add a button:

```html
<form method="post" action="/workflows/run/<path>">
  <button>Run this workflow now</button>
</form>
```

…wired through fetch() so the response can be inspected:

```js
button.onclick = async () => {
  const resp = await fetch(`/workflows/run/${relPath}`, {
    method: "POST",
    headers: {"Content-Type": "application/json"},
    body: "{}",
  });
  if (resp.status === 200) {
    const body = await resp.json();
    window.location.href = `/run/${body.data.run_id}`;
  } else {
    const body = await resp.json();
    alert(body.error.message);
  }
};
```

Alert is acceptable for V2 (mirrors the alert-on-error pattern from
the editor V1.5).

## Test plan

| Surface | Test |
| --- | --- |
| `tests/test_workflow_field_errors.py` (new) | `validate_workflow` raises with `field_path` for each updated raise site |
| `tests/test_web_workflow_run.py` (new) | POST creates run, returns run_id; 405 without --allow-mutations; 422 invalid; 409 dirty tree (mock) |
| `tests/test_web_workflow_edit.py` (extend) | If-Match matching → 200; stale → 412 with current_sha256; missing → 200 (V1.5 compat) |
| `tests/test_web_workflow_edit.py` (extend) | 422 body carries `errors[]` for raise sites that have field_path |

## Out of scope (V3)

- Drag-and-drop graph editor.
- Workflow templates / marketplace.
- "Diff against another workflow" view.
- AI-assisted scaffolding via chat tool that *writes* workflow.json.
- Multi-error reporting (collect all errors, not just first).
- Field-path coverage for the remaining ~22 raise sites.
- `flock()` for hard concurrency guarantees.
