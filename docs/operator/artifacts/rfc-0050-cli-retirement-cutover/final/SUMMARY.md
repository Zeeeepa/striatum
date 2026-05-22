---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/artifacts/rfc-0050-cli-retirement-cutover/classification/CLI_PARITY_MAP.md", "docs/operator/artifacts/rfc-0050-cli-retirement-cutover/parity-slice/NEXT_SLICE.md", "docs/operator/artifacts/rfc-0050-cli-retirement-cutover/build/HANDOFF.md", "docs/operator/artifacts/rfc-0050-cli-retirement-cutover/review/authority/REVIEW.md", "docs/operator/artifacts/rfc-0050-cli-retirement-cutover/review/regression/REVIEW.md"]
---

# RFC 0050 Cutover Slice Closure
author: operator [self-declared: codex-operator]

## Closure

This RFC 0050 cutover slice is accepted as a UI-first parity slice for
human-principal escalation handling. It does not claim full CLI retirement,
does not hide or delete workflow-control CLI verbs, and does not alter the
daemon method registry.

## Classification Result

The parity map keeps CLI surfaces in survivor categories until each verb has a
tested replacement and an explicit survivor/deletion decision. Bootstrap,
diagnostics, local authoring, retired compatibility, and agent packet-loop
surfaces remain CLI-compatible. Workflow-control verbs are hide candidates
only after MCP/operator-UI parity tests pass for the exact replacement path.

The human-principal cluster was identified as the next useful gap:
`inbox`, `escalation list`, `escalation show`, and `escalation resolve` were
selected first because their daemon methods already existed and they could be
implemented without new authority surface.

## Landed Slice

The implementation added a local web escalation inbox/detail/resolve path:

- `GET /escalations`
- `GET /escalations/<escalation_id>`
- `POST /escalations/<escalation_id>/resolve`

The web helpers route through existing daemon methods:
`escalation.list`, `escalation.show`, and `escalation.resolve`. The POST path
is gated by `--allow-mutations`, same-origin checks, safe path-id validation,
and daemon capability enforcement. Human-checkpoint rows do not render the
escalation resolve form.

No CLI workflow-control verb was hidden, renamed, deleted, or de-documented.
`docs/HOW_TO_HUMAN.md` now prefers `/escalations` for principal escalation
handling while keeping CLI commands as temporary compatibility and debugging
examples.

## Validation

The implementation handoff records:

- `.venv/bin/ruff check src/striatum/web/escalations.py src/striatum/service.py src/striatum/service_routes.py tests/test_web_escalations.py tests/test_mcp_mutation_capabilities.py`: pass.
- `.venv/bin/mypy src/striatum/web/escalations.py tests/test_web_escalations.py`: pass.
- `.venv/bin/python -m pytest tests/test_web_escalations.py tests/test_mcp_mutation_capabilities.py`: 33 passed.

The operator reran the same focused pytest, ruff, mypy, and `git diff --check`
before committing the implementation. The escalation-specific parity tests for
the selected replacement path passed. That is sufficient for UI-first docs,
but it is not treated as authorization for CLI hide/delete.

## Reviews

Authority review: `accept`.

Regression review: `accept`.

The reviews found no blocking authority, capability, mutation-gate, daemon,
MCP visibility, or premature CLI-deletion regression. Both retained the
residual gate that CLI deletion still requires a future survivor-category
artifact and exact replacement coverage.

## Still Blocked From Hide/Delete

The following CLI commands remain compatibility surfaces and are not retired
by this slice:

- `inbox`
- `escalation list`
- `escalation show`
- `escalation resolve`

Adjacent human-principal workflow-control commands also remain CLI-first until
their own parity slices land:

- `decision record`
- `checkpoint resolve`

## Next Safe Slice

The next smallest MCP/UI parity slice is the remaining human-principal pair:
`decision record` plus `checkpoint resolve`. It should follow the same pattern:
daemon-routed local web UI, `--allow-mutations` refusal, same-origin/context
protection, no SQLite fallback, and focused MCP capability/refusal tests. It
should still avoid CLI deletion until the replacement path has accepted
reviews and a survivor-category closure.
