---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/operator/artifacts/rfc-0050-cli-retirement-cutover/classification/CLI_PARITY_MAP.md", "docs/operator/workflows/rfc-0050-cli-retirement-cutover/prompts/select_next_parity_slice.md", "docs/operator/plans/rfc-0050-cli-retirement-cutover.md", "docs/rfcs/0050-go-daemon-http-sse-mcp.md", "docs/operator/BRIEF.md", "src/striatum/web/run_actions.py", "src/striatum/service_routes.py", "tests/daemon_pg/handlers/reads/test_escalations.py"]
---

# Next MCP/UI Parity Slice
author: operator [self-declared: codex-operator]

## Selection

Implement the smallest human-principal parity slice first: a local web
escalation inbox with detail and resolve actions.

Selected CLI family:

- `inbox`
- `escalation list`
- `escalation show`
- `escalation resolve`

Replacement path:

- MCP methods already registered: `escalation.list`, `escalation.show`,
  `escalation.resolve`
- UI routes to add: `GET /escalations`,
  `GET /escalations/<escalation_id>`,
  `POST /escalations/<escalation_id>/resolve`

This slice deliberately excludes `decision record` and `checkpoint resolve`.
Those are the next adjacent human-principal gaps, but keeping them out makes
this slice a single inbox/detail/resolve surface over existing daemon methods.
Rows projected from human checkpoints may be shown in the inbox, but resolving
them through `checkpoint.resolve` remains a follow-up.

## Why This Slice

The classification map names the human-principal cluster as the next useful
operator-only gap. The escalation sub-slice is the smallest part that replaces
a real CLI workflow-control path without adding new daemon contract surface:
the daemon methods exist, MCP visibility exists, PostgreSQL handlers exist,
and the UI only needs to render/read/call those methods through the existing
local service mutation gate.

## Likely Files

Likely implementation files:

- `src/striatum/service_routes.py` for `/escalations` GET/POST dispatch.
- `src/striatum/service.py` for route-context wiring.
- `src/striatum/web/escalations.py` as the page/action helper module.
- `src/striatum/web/templates/escalation_list.html`.
- `src/striatum/web/templates/escalation_detail.html`.
- `src/striatum/web/templates/base.html` to add an Escalations nav link.
- `src/striatum/web/static/escalation_resolve.js` if the detail page uses
  progressive-enhancement POST behavior.
- `tests/test_web_escalations.py` or focused additions to `tests/test_service.py`
  for route and mutation-gate coverage.
- Existing MCP/capability tests if the current `escalation.*` visibility is
  not already asserted at the required granularity.

Files that should not change in this slice:

- `contracts/daemon_methods.json`.
- `docs/architecture/COMMAND_AUTHORITY_MATRIX.md`.
- `go/pkg/mcp/capabilities.go`.
- `src/striatum/cli/parser.py`.
- `docs/CLI_REFERENCE.md`.
- Any PostgreSQL migration or direct handler implementation unless a test
  proves the existing DTO cannot support the UI.
- `.striatum/`, legacy SQLite modules, and test-fixture SQLite paths.

## Acceptance Tests

Web read tests:

- `GET /escalations` calls daemon `escalation.list` through
  `call_repo_method`, renders open rows, and never opens repo-local SQLite.
- `GET /escalations?run_id=<id>` passes `{"run_id": "<id>"}` to
  `escalation.list`.
- `GET /escalations/<escalation_id>` validates the path id, calls
  `escalation.show`, renders the escalation detail, and propagates daemon
  errors without synthesizing state.

Web mutation tests:

- `POST /escalations/<escalation_id>/resolve` with `--allow-mutations` calls
  `escalation.resolve` with exactly the path escalation id plus JSON body
  fields `decision_id` and `resolution_note`.
- The same POST without `--allow-mutations` returns 405 and makes no daemon
  call.
- Cross-origin, missing-origin, and malformed-origin browser POSTs are refused
  by the existing same-origin gate before any daemon call.
- Empty, traversal, slash-containing, and NUL-containing escalation ids are
  refused.
- Daemon `ServiceDaemonRpcError` status and kind are preserved in the JSON
  response.

MCP/contract tests:

- `escalation.list` and `escalation.show` appear for read-capable tokens.
- `escalation.resolve` appears only for admin-capable tokens and is refused
  for read-only or wrong-repository tokens.
- Local authoring methods remain hidden from production MCP discovery.

Regression guard:

- Add or preserve a SQLite tripwire around web handlers so the route cannot
  satisfy the UI from repo-local SQLite or legacy daemon registry state.

## Authority Guardrails

- The UI must call `striatum.service_daemon.call_repo_method`; it must not
  shell out to `striatum`, invoke `/v1/invoke`, open PostgreSQL directly, or
  open repo-local SQLite.
- `escalation.resolve` remains the only writer. The UI sends parameters and
  displays the daemon response; it does not update blocker, inbox, event, or
  artifact state itself.
- POST resolution stays behind `--allow-mutations` and same-origin checks.
  Bearer-token API clients keep the existing service exemption; browser forms
  do not.
- The route records no transcripts and stores no extra local files.
- Human-checkpoint rows shown by `escalation.list` must not be resolved by
  this slice unless `checkpoint.resolve` UI is explicitly added with its own
  tests.

## Docs Impact

After the implementation tests pass, update human-principal docs to prefer
the web route for escalation inbox/detail/resolve:

- `docs/HOW_TO_HUMAN.md` should point principals to `/escalations` first and
  keep CLI commands as temporary compatibility/debugging examples.
- `docs/USING_STRIATUM.md` may mention the UI inbox in the principal section.
- `docs/CLI_REFERENCE.md` should not be removed or marked retired in this
  slice.
- Agent-loop docs and skills should not change for this slice; this is a
  principal/operator UI action, not a lane-agent packet-loop action.

## No CLI Deletion Gate

Do not hide, delete, rename, or de-document `inbox`,
`escalation list`, `escalation show`, or `escalation resolve` until all tests
above pass for the exact replacement path and a follow-up cutover artifact
records the survivor category. Passing this slice only authorizes a UI-first
docs preference for escalation handling; it does not authorize CLI workflow
control deletion.
