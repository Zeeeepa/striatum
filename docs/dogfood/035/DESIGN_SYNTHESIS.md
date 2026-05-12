---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/dogfood/035/design/codex/DESIGN.md", "docs/dogfood/035/design/claude_code/DESIGN.md", "docs/dogfood/035/design/gemini/DESIGN.md"]
---

author: designer-codex-gpt-5.5-001

# Design Synthesis: RFC 0032 Cross-Repo Workflows and MCP Mutation

Status: implementation plan
Date: 2026-05-12
Target: RFC 0032 cross-repository workflows and MCP mutation capabilities

## Accepted Implementation Scope

Implement RFC 0032 as an additive daemon V2 layer on top of dogfood-034. The
daemon PostgreSQL database owns cross-repo coordination, capability policy,
MCP authorization, request logs, and metadata-only audit. Each participating
target repository keeps `.striatum/state.sqlite3` as the authoritative local
workflow store for its own runs, jobs, sessions, artifacts, verdicts, leases,
and events. This plan does not redesign the daemon RPC envelope, transport,
handshake, request log, supervision scaffold, apply scaffold, lane attestation,
or system-Postgres substrate.

| RFC 0032 acceptance criterion | Concrete code plan | Test owner |
|---|---|---|
| Two-repo workflow validates, prepares, and runs to completion against daemon RPC; artifacts land in correct per-repo paths. | Add workflow schema validation in `src/striatum/workflow.py`; add daemon cross-repo lifecycle service in new `src/striatum/cross_repo.py`; add RPC routes in `src/striatum/daemon_rpc/registry.py` and `server.py`; defer full end-to-end daemon harness run to TODO item 19. | `tests/test_workflow_cross_repo.py`, `tests/test_cross_repo_lifecycle.py`; full E2E deferred |
| Crash between daemon DB and local SQLite writes reconciles to `started` or `aborted`. | Store `cross_repo_runs(state='preparing')` and participants in daemon DB; write repo-local back-references idempotently; add `reconcile_cross_repo_preparing()` at daemon startup. | `tests/test_cross_repo_lifecycle.py` with mocked repo-local stores |
| Per-repo write-scope enforcement: repo A job cannot write repo B. | Resolve every job path only after its repository alias is known; qualify artifact uniqueness and parallel write-scope overlap by alias; route publish/complete through target repo root. | `tests/test_cross_repo_write_scope.py` |
| MCP `tools/list` filters by capability; `tools/call` refuses missing capability and audits. | Generate daemon MCP tool specs from the RFC 0030 method registry; compute effective tool set from token grants and scope; re-authorize every `tools/call` before dispatch. | `tests/test_mcp_mutation_capabilities.py` |
| Read-only token cannot invoke write tool; audit row records `capability_missing`. | Reuse `daemon_rpc.capability.authorize()` for MCP; denial path appends metadata-only audit and request-log rows with `transport='mcp'`. | `tests/test_mcp_mutation_capabilities.py`, `tests/test_mcp_prompt_injection_shapes.py` |
| `apply` token scoped to repo A cannot apply against repo B. | Keep `apply.reviewed_patch` single-repo scoped; require `repository_id` match the token grant; cross-repo sealed apply remains out of scope. | `tests/test_mcp_mutation_capabilities.py` |
| `striatum cross-repo list/why/describe` work end-to-end. | Add CLI parser/dispatch verbs that call `cross_repo.list`, `cross_repo.why`, and `cross_repo.describe`; aggregate daemon metadata plus repo-local status slices. | `tests/test_cli_cross_repo.py`, `tests/test_cross_repo_lifecycle.py` |
| Documentation updates. | Update `docs/SPEC.md`, `docs/MCP.md`, `docs/UBIQUITOUS_LANGUAGE.md`, `docs/CLI_REFERENCE.md`, `docs/HOW_TO_HUMAN.md`, RFC 0032 status text, and `CHANGELOG.md`. | `tests/test_doc_links.py` plus review |

## Deferred Scope

| Deferred item | Why deferred | Landing place |
|---|---|---|
| Multi-repo / cross-repo end-to-end integration tests | Current test fixtures are single-repo; building a real multi-repo daemon harness is developer infrastructure, not the RFC 0032 product slice. | `docs/TODO.md` Open item 19, multi-repo test harness RFC |
| Cross-machine multi-tenant semantics | D083 keeps daemon V2 single OS user and local-only. Cross-repo means local registered repositories only. | Deferred indefinitely; no follow-up planned |
| Python to Go daemon core | D084 wants the protocol to survive a Go core, but this dogfood implements the Python daemon surface. | Future D084 follow-up |
| Bundled / Dockerized Postgres | RFC 0033 chose system Postgres for daemon V2; bundling changes installer and lifecycle responsibility. | RFC 0033 packaging follow-up |
| Cross-repo sealed apply | RFC 0031 receipts are single-repo. A multi-repo apply receipt would overclaim atomic source mutation. | Future provenance RFC, if needed |
| Malicious-local-root resistance | RFC 0031 threat model excludes an operator who controls the daemon OS user, database, or signing key. | Out of scope by product decision |

## `repositories` Workflow Block Schema

Use workflow-local aliases and daemon registered repository ids:

```json
{
  "repositories": {
    "primary": {"repo_id": "repo_abc"},
    "consumer": {"repo_id": "repo_def"}
  },
  "primary_repository": "primary"
}
```

`repositories` is optional. A workflow is cross-repo only when the block is
present with at least two aliases. Single-repo workflows must not declare
job-level `repository`. Cross-repo jobs must declare `repository`; this plan
chooses Claude's stricter rule over Codex's omitted-means-primary default so
cross-repo intent is reviewable per job. `primary_repository` is required; no
implicit first-entry primary because JSON object order is too subtle for a
workflow boundary.

Validator rules in `src/striatum/workflow.py`:

- reject non-object `repositories`, fewer than two entries, empty aliases,
  duplicate `repo_id` values, non-string `repo_id`, unknown primary alias, and
  unknown job repository aliases;
- reject `repositories` combined with explicit `require_daemon: false`;
  cross-repo implies daemon mode;
- interpret `write_scope.allowed_paths`, `forbidden_paths`, job inputs that are
  paths, and `expected_artifacts[].path` relative to the job's target repo;
- qualify artifact path collisions and parallel write-scope overlap by
  repository alias;
- allow `reviewer_access_scope: "cross_repo_artifact_augmented"` only for
  review jobs in cross-repo workflows;
- allow cross-repo edges normally;
- allow cross-repo cycles only when the cycle declares `cross_repo_cycle:
  true`; `max_iterations` is global to the cycle and stored in daemon DB, not
  per repo;
- accept `parallelism.per_repo_max_active_jobs` as an alias-to-positive-integer
  object and keep `parallelism.max_active_jobs` global to the cross-repo run.

`run prepare` performs daemon-backed checks that the referenced `repo_id`
values are registered, active, and accessible. Plain `workflow validate` stays
local and shape-only.

## Cross-Repo Run Lifecycle

Cross-repo lifecycle is daemon-only. Direct repo-local mode refuses
cross-repo workflows with a stable error pointing at daemon mode.

`run prepare` creates one canonical `cross_repo_run_id` in daemon DB and one
local run per participant. The daemon writes `cross_repo_runs(state =
"preparing")`, creates participant rows, opens each registered repo root in
deterministic repository-id order, calls the existing local prepare service for
that repo's local jobs, writes the repo-local back-reference, records
`local_run_id`, then marks the daemon row `prepared`.

This is not distributed database 2PC. Daemon DB writes are transactional;
repo-local SQLite writes commit independently. Crash recovery is best-effort:
startup reconciliation inspects `preparing` rows and either completes missing
local rows when all repos are still active, marks the run `prepared` when all
participants are intact, or marks it `aborted` with `last_reconcile_error`.
Orphaned local rows whose daemon parent never committed are surfaced by
`doctor` as `cross_repo_orphaned_runs`; the daemon does not invent a running
cross-repo run from orphaned local state.

`run start --run-id <cross_repo_run_id>` verifies every participant is
prepared and active, starts each local run using existing branch/start
semantics, then marks the daemon row `running`. If one repo cannot start, the
daemon records a human-checkpoint blocker in the primary repository and leaves
the cross-repo run `blocked`; already-created local run rows are not deleted.

`run summary`, `status`, `dashboard`, and `why` first resolve whether the id is
a `cross_repo_run_id`. If so, they aggregate daemon metadata plus per-repo
local status. Display artifact paths as `<alias>:<repo_path>` while preserving
plain repo-relative artifact paths inside each local repository.

`run cancel` / `cross-repo cancel` attempts cancellation for every
non-terminal participant. State is `canceling` while any participant cannot be
reached, `canceled` once all are terminal-canceled, and `blocked` when operator
repair is required.

If a participant repo is removed or unavailable mid-run, the daemon pauses the
cross-repo run with a human checkpoint in the primary repository. The operator
may re-register the same repository id and resume, or cancel the cross-repo
run. The daemon must not silently retarget an alias to a different repository.

## MCP Mutation Capability Wiring

Extend the dogfood-034 capability vocabulary to the RFC 0032 closed set:
`read`, `write`, `review`, `claim`, `apply`, `admin`, and `recovery`.
`recovery.*` routes require `recovery`; `admin` may retain a one-release
compatibility allowance for recovery routes, but new least-privilege grants
should use `recovery`.

The daemon method registry remains the source of truth. Add
`repository_scope_mode` to `MethodEntry`: `single_repo`, `cross_repo`, or
`daemon_global`. `tools/list` returns the effective tool set:

```text
effective tool set = method registry ∩ token capabilities ∩ token scope
```

`tools/call` re-checks authorization through the same
`daemon_rpc.capability.authorize()` path, even if the tool appeared in
`tools/list`. Hidden is not unauthorized; the capability check is the security
boundary. Unknown methods, methods without capability declarations, missing
tokens, revoked tokens, expired tokens, and scope mismatches fail closed. There
is no V2 global `--allow-mutations` flag for daemon MCP.

For cross-repo routes, the token must be daemon-global for the required
capability or have that capability for every participating repository. For
single-repo routes, the token's repo scope must match the request
`repository_id`. For `apply`, keep the single-repo rule: repo A `apply` cannot
apply to repo B.

## Capability Token Lifecycle for Mutation

`daemon.token.create` is admin-only and accepts capability grants, optional
repo scope per grant, and expiry. Mutation-capability tokens (`write`,
`review`, `claim`, `apply`, `recovery`) default to one hour when the operator
does not pass an expiry. `read`-only and `admin`-only behavior stays explicit
in documentation; long-lived mutation tokens are allowed but warned.

`daemon.token.revoke` sets `revoked_at` and `revoked_reason`; future calls deny
with `token_revoked`. In-flight calls check authorization once at request
decision time. A revoke racing with an already-authorized call does not cancel
the call; the audit timeline shows allow-before-revoke or deny-after-revoke.
`daemon.token.rotate` is a convenience route that creates a replacement token
with the same grants and revokes the old token with reason `rotation`.

## Audit Shape for MCP Mutations

Every MCP `tools/call` that maps to a mutating method records an authorization
audit row and an RFC 0030 request-log row, allowed or denied, before the
response is returned. Audit remains metadata-only:

```text
transport = "mcp"
request_id
client_id
token_hash or token_id reference, never token secret
method
repository_id when single-repo scoped
cross_repo_run_id when present
params_hash
response_hash when available
decision = allowed | denied
denial_reason
daemon_version
substrate_version
previous_hash
row_hash
```

Use the existing denial vocabulary plus RFC 0032 additions:
`capability_missing`, `capability_missing_for_participant`,
`capability_scope_mismatch`, `token_revoked`, `token_expired`,
`capability_expired`, `repo_not_registered`, `cross_repo_run_unknown`,
`cross_repo_repo_unregistered`, `cross_repo_state_invalid`, and
`method_unknown`. For multi-repo audit readability, add
`striatumd.audit_repositories(audit_id, repository_id)` rather than stuffing
participant ids into prose or request bodies.

## Daemon DB + Repo-Local DB Coordination

Add daemon Postgres migration v3:

- `striatumd.cross_repo_runs(cross_repo_run_id, workflow_id,
  workflow_version, workflow_snapshot_hash, primary_repository_id, state,
  created_at, started_at, completed_at, canceled_at, reconcile_after,
  last_reconcile_error)`;
- `striatumd.cross_repo_run_repositories(cross_repo_run_id,
  repository_alias, repository_id, local_run_id, state, prepared_at,
  last_observed_at)`;
- `striatumd.cross_repo_cycle_counters(cross_repo_run_id, cycle_key,
  iterations, max_iterations)`;
- `striatumd.audit_repositories(audit_id, repository_id)` if audit rows need
  multi-repo participant indexing.

Add repo-local SQLite migration v14:

```sql
ALTER TABLE runs ADD COLUMN cross_repo_run_id TEXT;
CREATE INDEX idx_runs_cross_repo_run_id
  ON runs(cross_repo_run_id)
  WHERE cross_repo_run_id IS NOT NULL;
```

The synthesis chooses a `runs.cross_repo_run_id` back-reference over a
repo-local cross-repo table. The daemon DB is canonical for the cross-repo
run; repo-local state only needs to report that a local run participates.

## Schema Migration

`src/striatum/daemon_pg/migrations.py` advances to daemon DB version 3 and
registers `0003_cross_repo.sql`. The migration must preserve the dogfood-034
audit-chain and request-log tables unchanged except for optional participant
indexing. `src/striatum/migrations.py` advances repo-local SQLite to v14 with
the nullable `runs.cross_repo_run_id` column.

Migration tests assert daemon v3 tables exist, repo-local v14 is idempotent,
old single-repo runs keep NULL `cross_repo_run_id`, and no existing direct-mode
single-repo tests require a daemon.

## Test Strategy (With Explicit Deferral)

Ship unit-level, mock-based, and single-repo-mock coverage in dogfood-035:

- `tests/test_workflow_cross_repo.py`: repositories shape, explicit primary,
  required job repository, invalid single-repo job repository field,
  repository-qualified artifact uniqueness, `cross_repo_artifact_augmented`,
  cross-repo cycles requiring `cross_repo_cycle: true`, and
  `per_repo_max_active_jobs`;
- `tests/test_daemon_pg.py`: daemon migration v3 and audit participant table;
- `tests/test_cross_repo_lifecycle.py`: mocked registered repos, local prepare
  mocks, `preparing -> prepared`, reconciliation to `prepared` or `aborted`,
  and repo-unavailable checkpoint behavior;
- `tests/test_cross_repo_write_scope.py`: mocked repo roots prove repo A job
  cannot publish or resolve write paths under repo B;
- `tests/test_daemon_rpc_registry.py`: `recovery`, `repository_scope_mode`, and
  startup refusal for non-hello methods without a capability;
- `tests/test_mcp_mutation_capabilities.py`: filtered `tools/list`,
  authorized `tools/call`, missing/expired/revoked/scope-mismatched denials,
  and repo-scoped `apply` refusal;
- `tests/test_mcp_prompt_injection_shapes.py`: malicious arguments cannot
  escalate beyond token grants and denial audit rows are written.

Multi-repo / cross-repo end-to-end integration tests are explicitly deferred to
`docs/TODO.md` Open item 19, the multi-repo test harness RFC. Deferred coverage
includes a per-test daemon with two or more registered initialized repos, real
cross-repo edge progression, daemon restart during `preparing`, real artifact
publication into separate worktrees, and cross-repo cycle accounting through
live scheduler events.

## Documentation Deltas

Update:

- `docs/SPEC.md`: cross-repo workflow schema, lifecycle, best-effort
  consistency, daemon-only mutation, MCP capability vocabulary, and no
  distributed file atomicity;
- `docs/MCP.md`: daemon MCP tools, per-token `tools/list`, `tools/call`
  capability checks, denial vocabulary, short-lived mutation-token guidance,
  and no `--allow-mutations` bypass;
- `docs/UBIQUITOUS_LANGUAGE.md`: cross-repo run, primary repository,
  cross-repo coordinator, MCP mutation capability, effective tool set,
  repository scope mode;
- `docs/CLI_REFERENCE.md`: `cross-repo list|describe|why|cancel`, token
  lifecycle verbs, and daemon-routed cross-repo `run prepare/start/summary`;
- `docs/HOW_TO_HUMAN.md`: operator flow for creating repo-scoped mutation
  tokens and recovering paused cross-repo runs;
- RFC 0032: mark accepted/implemented for the shipped slice and record the E2E
  harness deferral;
- `CHANGELOG.md`: dogfood-035 release note.

## Staging Plan

Dogfood-035 should land the schema validator, daemon v3/repo-local v14
migrations, method-registry capability/scope expansion, MCP `tools/list` and
`tools/call` gating, metadata-only MCP audit, cross-repo lifecycle service with
mocked local repositories, CLI `cross-repo list|describe|why|cancel`, and
documentation.

Defer to future dogfoods: full multi-repo E2E harness, cross-repo sealed apply,
direct-mode retirement, bundled Postgres, Go daemon, Windows/WSL repository
path mapping, and any external audit-query UI beyond existing doctor/admin
surfaces.

## Human-Decision Questions

1. Should `admin` retain temporary compatibility for `recovery.*`, or should
   RFC 0032 require operators to grant `recovery` explicitly from day one?
2. Should cross-repo `tools/list` show cross-repo tools to a token that covers
   only some repos, or hide them until the token has daemon-global capability?
   This synthesis chooses the stricter hide-until-all-known-coverage behavior.
3. What exact command name should cancel use: `run cancel --run-id
   <cross_repo_run_id>` only, or also `cross-repo cancel`? This synthesis keeps
   both, with `cross-repo cancel` as the explicit operator-facing alias.
