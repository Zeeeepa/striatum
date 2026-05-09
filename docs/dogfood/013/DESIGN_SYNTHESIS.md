---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/rfcs/0013-local-web-ui.md", "docs/rfcs/0012-local-service-api.md", "docs/dogfood/013/research/MUTATION_BUTTONS.md", "src/striatum/service.py", "src/striatum/web/static/app.js"]
---

# RFC 0013 step 7 Design Synthesis

author: designer-codex-gpt-5.5-001

Date: 2026-05-08
Target: V1 build slice for RFC 0013 step 7 (web UI mutation
buttons).

## Locked Contracts

### `/v1/health` extension (one-line schema add)

```json
{
  "ok": true,
  "data": {
    "started_at": "...",
    "version": "1.3.0",
    "mode": "tcp",
    "allow_mutations": false
  }
}
```

`allow_mutations` is the boolean the SPA caches once per page
load to decide whether to render mutation buttons.

### Mutation set (V1)

Five buttons, mapped 1:1 to existing CLI verbs:

| Button | View | Argv |
|---|---|---|
| **Continue blocker** | Job detail | `["checkpoint", "resolve", "--blocker-id", <id>, "--action", "continue"]` |
| **Cancel blocker** (red confirm) | Job detail | `["checkpoint", "resolve", "--blocker-id", <id>, "--action", "cancel"]` |
| **Record verdict** | Review job detail | `["verdict", "--session-id", <s>, "--job-id", <j>, "--lease-id", <l>, "--verdict", <v>, "--rationale", <text>]` |
| **Record decision** | Run detail | `["decision", "record", "--run-id", <r>, "--path", <p>, "--outcome", <o>, "--title", <t>]` |
| **Requeue stale review** | Job detail (state=stale_lease, review-only) | `["recovery", "requeue-stale", "--run-id", <r>, "--job-id", <j>]` |

Out of scope for V1: claim-next (no session ownership in the
SPA), publish-artifact (file upload UX is a future RFC),
release / heartbeat (lease lifetime is too short to surface
sensibly in a click flow).

### SPA flow

1. Modal opens with the literal argv that would be sent (so
   the operator can sanity-check).
2. Confirm → `fetch('/v1/invoke', {method: 'POST', body:
   JSON.stringify({argv})})`.
3. On `{ok: true}` → success banner with the result envelope;
   refresh the underlying view (run / job detail) by re-fetching.
4. On `{ok: false}` → render the error envelope inline (exit
   code, message); view does not refresh.
5. Buttons hidden / greyed when `allow_mutations: false`.

### CSP

Unchanged. No external dependencies. No `eval`, no inline
event handlers (use `addEventListener`). No `innerHTML` for
user-supplied strings.

### Tests

`tests/test_web_ui.py`:

- `test_health_includes_allow_mutations_flag` (read-side)
- `test_invoke_verdict_succeeds_when_mutations_allowed`
- `test_invoke_verdict_refused_without_allow_mutations`
- `test_invoke_decision_record_succeeds_when_mutations_allowed`
- `test_invoke_checkpoint_resolve_continue`
- `test_invoke_checkpoint_resolve_cancel`
- `test_invoke_recovery_requeue_stale_review_only`
- `test_invoke_rejects_unknown_verb`
- `test_spa_app_js_has_mutation_button_handlers` (string-grep)
- `test_no_external_url_invariant_after_mutations` (CSP guard)

### Implementation order

1. Extend `service.py:_handle_health` to include
   `allow_mutations`.
2. Add `mutationsAllowed()` helper in `app.js` that hits
   `/v1/health` once per page load.
3. Per-button: HTML in `app.js`'s view renderers + click
   handler + modal.
4. CSS in `app.css` (modal, success / error banners,
   destructive vs neutral confirm buttons).
5. Tests.
6. Doc updates (HOW_TO_HUMAN.md).
7. CHANGELOG, DECISION_LOG (D065), TODO (F13), RFC 0013 status.
8. Bump v1.3.0 → v1.4.0.

## Acceptance Criteria

- `/v1/health` returns `allow_mutations: bool`.
- All five mutation buttons render in the right views, only
  when `allow_mutations: true`.
- Each button confirm-and-fires through `POST /v1/invoke` with
  the documented argv shape.
- `--allow-mutations` off: buttons hidden/greyed; if
  hand-fired, server returns exit code 8 with structured
  error envelope; SPA renders it inline.
- View refreshes after a successful mutation.
- CSP unchanged; SPA has no external URLs.
- `make lint` / `make typecheck` / `make test` clean.
- `__version__` and `pyproject.toml` bump 1.3.0 → 1.4.0.

## Acceptance Gate

Implementation job blocks until human acceptance recorded under
`docs/dogfood/013/decisions/`.
