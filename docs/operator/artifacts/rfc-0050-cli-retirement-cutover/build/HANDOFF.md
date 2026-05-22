---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

author: operator [self-declared: codex-operator]

# RFC 0050 Parity Slice Handoff

Implemented the selected local web escalation inbox/detail/resolve slice over
the existing daemon methods `escalation.list`, `escalation.show`, and
`escalation.resolve`. No CLI workflow-control verb was hidden, renamed,
deleted, or de-documented.

## Changed Files

| Path | Change |
|---|---|
| `src/striatum/web/escalations.py` | Added daemon-routed web helpers for list, detail, and mutation-gated resolve actions with path validation and daemon error preservation. |
| `src/striatum/service.py` | Added escalation route context and handler methods. |
| `src/striatum/service_routes.py` | Routed `GET /escalations`, `GET /escalations/<id>`, and `POST /escalations/<id>/resolve`. |
| `src/striatum/web/templates/escalation_list.html` | Added the principal escalation inbox page. |
| `src/striatum/web/templates/escalation_detail.html` | Added escalation detail view and resolve form; human-checkpoint rows do not render the escalation resolve form. |
| `src/striatum/web/static/escalation_resolve.js` | Added progressive JSON POST behavior for the resolve form. |
| `src/striatum/web/templates/base.html` | Added Escalations navigation and keyboard shortcut entry. |
| `src/striatum/web/static/base.js` | Added `g e` navigation shortcut. |
| `tests/test_web_escalations.py` | Added focused web route, mutation-gate, same-origin, path-validation, no-SQLite, template, and daemon-error tests. |
| `tests/test_mcp_mutation_capabilities.py` | Added targeted escalation MCP visibility and authorization-refusal coverage. |
| `docs/HOW_TO_HUMAN.md` | Updated the human-principal playbook to prefer `/escalations` while keeping CLI commands as compatibility/debugging examples. |
| `docs/operator/artifacts/rfc-0050-cli-retirement-cutover/build/HANDOFF.md` | Recorded this implementation handoff. |

## Validation

Commands run:

```bash
.venv/bin/ruff check src/striatum/web/escalations.py src/striatum/service.py src/striatum/service_routes.py tests/test_web_escalations.py tests/test_mcp_mutation_capabilities.py
.venv/bin/mypy src/striatum/web/escalations.py tests/test_web_escalations.py
.venv/bin/python -m pytest tests/test_web_escalations.py tests/test_mcp_mutation_capabilities.py
```

Results:

- Ruff focused slice: pass.
- Narrow mypy for the new module/test: pass.
- Focused pytest selection: 33 passed.

Note: a broader targeted mypy invocation that included
`tests/test_mcp_mutation_capabilities.py` also traversed existing
`tests/_harness/mcp.py` and reported a pre-existing `no-redef` issue there;
that helper was outside this slice's implementation path and was not changed.

## Remaining No-CLI-Deletion Gate

This slice authorizes a UI-first documentation preference only. It does not
authorize hiding or deleting `inbox`, `escalation list`, `escalation show`, or
`escalation resolve`.

Follow-up cutover still needs a survivor-category artifact before any CLI
workflow-control removal, and adjacent human-principal gaps remain out of
scope here: `decision record` and `checkpoint resolve` still need their own
UI parity tests before their CLI compatibility status can change.
