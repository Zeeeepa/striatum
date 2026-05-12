---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["threat_model", "rfc-0032", "cross-repo", "mcp-mutation", "build"]
---

author: reviewer-claude-opus-001

# Threat-Model Review: RFC 0032 Cross-Repo + MCP Mutation V2 Build

Status: accept_with_findings
Date: 2026-05-12
Posture: threat_model
Subject: dogfood-035 V2 slice (cross-repo workflow schema, daemon DB v3,
repo-local v14, daemon RPC capability/scope expansion, daemon MCP
`tools/list`/`tools/call` gating, mocked cross-repo lifecycle)

## Trust Boundaries Enumerated

1. **Workflow author → validator** (`workflow.py`). Schema must reject
   malformed `repositories` blocks before they reach the daemon.
2. **CLI / MCP / RPC client → daemon RPC router** (`daemon_rpc/server.py`).
   Capability-token authorization is the only gate.
3. **MCP client → daemon MCP** (`mcp.py:DaemonRpcServer`). Per-token
   `tools/list` filter + per-call re-authorization + audit on allow and
   deny.
4. **Daemon RPC router → repo-local SQLite**. Router is bound to one
   repo_root; cross-repo writes never originate from this surface.
5. **Daemon → daemon DB**. Cross-repo coordination state lives in
   PostgreSQL; repo-local SQLite gains only a nullable back-reference
   (`runs.cross_repo_run_id`).
6. **Operator OS user → daemon DB** (CLI direct PG path for
   `cross-repo list|describe|why`). Out of capability scope per D082/D083
   (single OS user, local-only); CLI users have OS-level DB access.
7. **Audit chain**. Every authorization decision (allow/deny) on RPC and
   MCP `tools/call` lands an `audit_log` row with `previous_hash` chain
   integrity; existing v2 hash format reused.

The threat model excludes a malicious local-root operator (RFC 0031 §Threat
Model). The findings below are scoped to over-eager AI agents acting
through documented interfaces and operator footguns.

## Required Checks → Verification

| Check | Status | Evidence |
|---|---|---|
| Capability gating on every mutation route | satisfied | `daemon_rpc/registry.py` declares `required_capability` for every non-`daemon.hello` entry; `server.py:88-95` calls `authorize` + `require_allowed`; `mcp.py:599-631` re-authorizes inside `tools/call` |
| Audit row on every mutating `tools/call` (allow + deny) | satisfied | `mcp.py:605-642` always calls `append_audit_row` and `append_request_log` before returning; tests `test_daemon_mcp_tools_call_reauthorizes_and_audits_denial` and `test_daemon_mcp_unknown_tool_is_default_denied_and_audited` |
| Per-token `tools/list` filtering | satisfied | `mcp.py:534-552` iterates registry and calls `authorize` per entry, returning only `allowed` tools; admin `daemon.*` methods also hidden by name-prefix filter; `test_daemon_mcp_tools_list_filters_by_capability` |
| Default-deny on unknown methods | satisfied for unknown tool name in `tools/call` | `mcp.py:572-595` builds a denied `RpcAuthContext("method_unknown")`, audits, and returns `isError=true`. See finding F5 for unknown JSON-RPC verb at the outer dispatch |
| Cross-repo run state coherence under daemon crash | satisfied (single-repo simulation) | `cross_repo.py:227-264` `reconcile_cross_repo_preparing` deterministically transitions `preparing → prepared` or `aborted`; `test_reconcile_preparing_marks_prepared_or_aborted` covers both branches; the prepare path also wraps participant prepares in try/except → `aborted` rollback at `cross_repo.py:112-122`. Real two-repo daemon-crash testing is documented as deferred |
| Per-repo write-scope enforcement when a job targets a different registered repo | satisfied (architecturally) | Workflow validator qualifies artifact-path uniqueness and parallel write-scope overlap by `_job_repository_alias` (`workflow.py:902-928, 1717-1739`); RPC router enforces `repo_root` match (`server.py:129-144`, `test_repo_scoped_rpc_refuses_mismatched_registered_repo`); cross-repo lifecycle helper carries `repository_id` into per-participant prepare. See finding F3 for the missing dedicated `test_cross_repo_write_scope.py` |
| No raw `.striatum/state.sqlite3` write paths through MCP mutation clients | satisfied | `DaemonRpcServer.call_daemon_tool` does not invoke any SQLite mutation path — it authorizes, audits, and returns an `audit_id` stub (`mcp.py:632`). `LocalRpcServer` writes via `striatum.api.invoke`, but it operates only inside the lane agent's already-trusted repo (V1 behavior unchanged). The daemon-MCP surface's effect on repo-local SQLite is currently empty by construction |
| Workflow schema additions: `repositories` validator | satisfied | `_validate_repositories_block` (`workflow.py:1059-1125`) rejects non-object, fewer-than-two entries, empty aliases, duplicate `repo_id`, missing `primary_repository`, unknown primary alias, and `require_daemon: false`; `_validate_job_repository` rejects single-repo workflows that declare `repository`; `test_workflow_cross_repo.py` covers each branch. Cross-repo cycles require `cross_repo_cycle: true` (`workflow.py:597-602`) |
| Documentation honesty | mostly satisfied | `SPEC.md`, `MCP.md`, `UBIQUITOUS_LANGUAGE.md`, `CLI_REFERENCE.md`, `HOW_TO_HUMAN.md`, RFC 0032 status, and `CHANGELOG.md` correctly describe the V2 slice as a foundation; `BUILD_HANDOFF.md` documents the multi-repo E2E deferral and pointer to TODO Open item 19. See finding F2 for one honesty gap on `tools/call` execution |
| Tests cover happy + adversarial paths for capability denial, audit append, write-scope, `tools/list` filtering, default-deny, scope mismatches | mostly satisfied | Coverage present for `tools/list` filter, `tools/call` deny + audit, unknown-tool deny + audit, registry scope mode, cross-repo lifecycle prepare/start/cancel/reconcile, scope-mismatch RPC refusal. Two synthesis-promised files were not shipped — see F3 |
| Write scopes / fixtures do not normalize `.striatum/` edits, transcript capture, or audit tampering | satisfied | This packet's `write_scope.allowed_paths = ["docs/dogfood/035/review/build/threat/"]` and `forbidden_paths = [".striatum/"]`; audit chain is append-only and chained via `previous_hash` (`request_log.py:78-165`) |

## Findings

### F1 — Cross-repo RPC routes accept attacker-controlled `repository_id` in envelope params, downscoping the auth check (medium)

`daemon_rpc/server.py:88-95` passes `_repository_id(envelope.params)` to
`authorize()` regardless of the route's `repository_scope_mode`. For
`cross_repo.list`, `cross_repo.describe`, `cross_repo.why`, and
`cross_repo.cancel` (declared `repository_scope_mode="cross_repo"`,
`repository_scope=False`), the route handler ignores `repository_id` from
params and only consults `cross_repo_run_id`. A caller that holds a
`read`-scoped token bound to `repo_a` can submit a cross_repo envelope
with `params={"repository_id": "repo_a", "cross_repo_run_id": "..."}`
and receive an `allowed` decision against the repo_a-scoped grant; the
route then returns the full cross-repo run state spanning every
participant repository.

The MCP path does not have this hole: `mcp.py:540` and `mcp.py:596-598`
explicitly force `auth_repository_id = None` when
`effective_repository_scope_mode != "single_repo"`, so repo-scoped
tokens cannot satisfy a cross_repo route there. The fix on the RPC
router is the symmetric one — when
`entry.effective_repository_scope_mode == "cross_repo"`, pass
`repository_id=None` to `authorize()` so the gate requires daemon-global
or per-participant capability per the RFC 0032 §5 / synthesis rule
("token must be daemon-global … or have that capability for every
participating repository").

Severity is medium rather than high because (a) the cross_repo routes
shipped here are read-only (`cancel` returns `not_implemented`) and
(b) dogfood-035 documentation positions the cross_repo RPC surface as
foundation for the deferred multi-repo harness rather than a
production-ready operator path. It is in scope for the threat model:
"over-eager AI agents acting through documented interfaces" can probe
this with a single token they were already given.

Recommended follow-up: gate cross_repo routes in the RPC router the
same way MCP `tools/list` / `tools/call` already do, and add an RPC-layer
test analogous to `test_repo_scoped_rpc_refuses_mismatched_registered_repo`
that asserts a repo-scoped token cannot widen via `repository_id` on a
cross_repo entry.

### F2 — Daemon MCP `tools/call` is auth+audit only; docs imply route execution (medium, documentation honesty)

`mcp.py:DaemonRpcServer.call_daemon_tool` returns
`_daemon_tool_result(name, ok=True, audit_id=audit_id)` for an allowed
call without dispatching to the underlying handler. This is consistent
with the V2 scaffolding and with `BUILD_HANDOFF.md` ("filter
`tools/list`, re-authorize `tools/call`, and append metadata-only audit
and request-log rows"), and it is the safest possible defaulting: no
mutation actually flows through the daemon-MCP surface yet. The
attack-surface implication is positive — even a token granted
`write`/`review`/`claim`/`apply`/`recovery`/`admin` cannot drive a
real mutation through MCP today.

The honesty gap is in the operator-facing prose. `docs/MCP.md`
("`tools/call` re-authorizes every request even if the tool was listed
earlier"), `docs/HOW_TO_HUMAN.md` ("Mutation grants … must be granted
deliberately, should usually be short-lived, and are re-checked on every
`tools/call`"), and `docs/SPEC.md` §RFC 0032 read as if a granted
capability lets a caller perform the mutation. Operators who hand out
`write` tokens to an MCP client may believe they are authorizing real
state changes; they are authorizing audit rows only.

Recommendation: add a short paragraph to `docs/MCP.md` clarifying that
in dogfood-035 the daemon-MCP `tools/call` surface authorizes and audits
only — execution will land with the multi-repo daemon harness (TODO Open
item 19). The capability vocabulary, default-deny, and audit shape are
real today; route dispatch is not.

### F3 — Two synthesis-promised tests are not shipped (low–medium)

`docs/dogfood/035/DESIGN_SYNTHESIS.md` lists
`tests/test_cross_repo_write_scope.py` and
`tests/test_mcp_prompt_injection_shapes.py`. Neither file exists in the
shipped tree; `BUILD_HANDOFF.md` does not list them either.

The threat surfaces they were meant to cover are partially covered
elsewhere:

- Per-repo write-scope: `test_workflow_cross_repo.py`
  exercises repository-qualified artifact-path uniqueness; the RPC
  router test asserts repo_id mismatches are refused. The synthesis
  intent — "mocked repo roots prove repo A job cannot publish or
  resolve write paths under repo B" — is not directly exercised.
- Prompt-injection shapes: `test_daemon_mcp_tools_call_reauthorizes_and_audits_denial`
  and `test_daemon_mcp_unknown_tool_is_default_denied_and_audited`
  cover the core "caller submits unauthorized arguments" cases but do
  not enumerate scope-mismatch / revoked-token / expired-token /
  expired-capability denial reasons against MCP shapes.

Acceptable for the V2 slice given multi-repo E2E is deferred, but the
gap should be tracked in TODO Open item 19 (multi-repo test harness)
and/or as explicit unit tests landed in a follow-up dogfood. This is
not a default-deny failure (the gate works) — it is missing
adversarial-shape coverage at the MCP layer.

### F4 — Hidden-but-callable: `daemon.*` admin methods are filtered from `tools/list` but still routable through `tools/call` (low, documentation)

`mcp.py:534-535` filters out methods with prefix `daemon.` from
`tools/list` (so `daemon.token.create`, `daemon.token.revoke`,
`daemon.key.rotate`, `daemon.shutdown`, `daemon.migrate`,
`daemon.describe` are never advertised). `call_daemon_tool` does not
apply this filter — `entry = METHOD_REGISTRY.get(name)` will find them,
and the `admin` capability check then decides. An MCP client granted
`admin` (which the documentation strongly discourages) can therefore
exercise admin routes that were never listed.

This is intentional — capability is the security boundary, not visibility
— and is the right behavior. But it is also the kind of thing operators
get wrong: "the tool is not in `tools/list` so MCP can't reach it" is a
plausible but false belief. `docs/MCP.md` should explicitly state that
`tools/list` is a UX filter and that `tools/call` will accept any
method in the registry that the token's capabilities authorize.

(Reminder: F2 still applies — even an `admin`-authorized call returns
the audit-only stub today, so the present blast radius is bounded.)

### F5 — Unknown JSON-RPC verbs on `DaemonRpcServer` are not audited, while unknown tool names inside `tools/call` are (low)

`mcp.py:DaemonRpcServer.handle_request` returns
`error_response(response_id, ERROR_METHOD_NOT_FOUND, ...)` for any
JSON-RPC `method` outside the small fixed set (`initialize`,
`tools/list`, `tools/call`, `resources/list`, `resources/read`) without
appending an audit row. By contrast, `call_daemon_tool` audits unknown
tool names inside `tools/call` (`mcp.py:572-595`). The work packet's
"default-deny enforcement for unknown methods … records an audit row"
is satisfied for the inner layer but not for the outer JSON-RPC verb.

Impact is low: unknown JSON-RPC verbs are non-mutating by definition
(there is no handler), and the small fixed verb set is unlikely to
grow without an explicit RFC. Recording an audit row for outer
JSON-RPC unknowns would close the gap and give operators a single
audit-log query for "any MCP probe activity."

### F6 — `_optional_string` silently coerces non-string `repository_id` to `str(value)` (low)

`mcp.py:733-739` accepts any scalar (`int | float | bool`) for
`repository_id` and returns its `str(...)` form. A client that supplies
`repository_id: 42` will be authorized against the daemon DB string
`"42"`. If no such repository row exists the call denies cleanly; if a
client deliberately stringifies the alias of another repo (`"repo_a"`
is already a string in normal flows), this is fine. The minor
threat-model concern is type confusion in custom integrations: the
implementation should either reject non-string `repository_id` outright
or document the coercion. Not a security boundary breach.

## Notes On Scope Not Tested Here

- Cross-machine semantics (out of scope per D082).
- Malicious local-root operator behavior (out of scope per RFC 0031
  §Threat Model).
- Live two-repo daemon harness, real artifact publication into
  separate worktrees, daemon restart during real `preparing`,
  cross-repo cycle accounting through the live scheduler — explicitly
  deferred to TODO Open item 19, multi-repo test harness RFC.
- Audit retention/rotation (reused dogfood-034 v2 chain; no schema
  change).
- Sealed-apply across repositories (RFC 0032 §10 punts to a future
  RFC; `apply.reviewed_patch` remains single-repo).

## Verdict

`accept_with_findings`. The V2 slice's capability vocabulary is
correctly closed (`read`, `write`, `review`, `claim`, `apply`, `admin`,
`recovery`); per-token `tools/list` filtering and per-call
re-authorization are both present; default-deny is enforced for unknown
tool names inside `tools/call`; the audit chain is appended on every
allow/deny including denials with the documented vocabulary; per-repo
write-scope enforcement at the RPC router and workflow-validator
layers is in place; cross-repo lifecycle reconciliation has both
`prepared` and `aborted` test branches; and documentation correctly
labels the multi-repo E2E surface as deferred. F1 is the most
load-bearing finding and worth landing as a follow-up before any
non-deferred cross_repo RPC consumer ships; F2 is a documentation
honesty fix; F3 is unit-test coverage owed to the threat surfaces; F4
and F5 are operator-clarity fixes; F6 is a minor input-validation
hygiene note.
