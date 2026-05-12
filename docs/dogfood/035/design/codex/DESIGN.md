author: designer-codex-gpt-5.5-001

# RFC 0032 Implementation Design: Cross-Repo Workflows and MCP Mutation

Status: implementation design
Date: 2026-05-12
Target: RFC 0032 cross-repository workflows and MCP mutation capabilities

## Design Decision

Implement RFC 0032 as an additive daemon V2 layer over the dogfood-034
foundation. The daemon PostgreSQL database owns cross-repo coordination,
capability checks, request logs, and audit. Each participating target
repository keeps its own `.striatum/state.sqlite3` as authoritative local
workflow state. The daemon never moves job/session/artifact truth wholesale
into Postgres and never promises cross-repo filesystem atomicity.

The implementation has two separable surfaces:

1. Cross-repo workflow runs, introduced by a top-level `repositories` block and
   daemon-mediated lifecycle handlers that create one canonical
   `cross_repo_run_id` plus one repo-local run per participating repository.
2. Daemon MCP mutation tools, generated from the RFC 0030 method registry and
   filtered per token by capability and repository scope.

This design does not rebuild RPC framing, daemon supervision, sealed-apply key
custody, apply receipts, lane attestation, or the daemon PostgreSQL substrate.
It extends `src/striatum/workflow.py`, `src/striatum/daemon_rpc/`,
`src/striatum/mcp.py`, and the daemon/repo-local migrations where state shape
must be recorded.

## Workflow Schema

Cross-repo workflows remain `schema_version: "striatum.workflow.v1"` and opt in
with a top-level `repositories` object:

```json
{
  "repositories": {
    "primary": {"repo_id": "repo_abc"},
    "consumer": {"repo_id": "repo_def"}
  },
  "primary_repository": "primary"
}
```

Use repository aliases in workflow JSON and registered daemon `repository_id`
values in each alias body. This keeps job definitions readable while the
daemon validates that aliases resolve to active registered repositories.
`primary_repository` is optional; when omitted, the first object entry is the
primary repository. That is deterministic because JSON object order is
preserved by Python's parser, but the validator should warn through
`workflow plan` when a cross-repo workflow relies on implicit primary ordering.

Jobs may declare `repository: "<alias>"`. Omitted `repository` means primary.
All job-local paths remain repository-relative to the job's target repository:
`write_scope.allowed_paths`, `write_scope.forbidden_paths`, inputs that are
repo paths, and `expected_artifacts[].path`.

Validator changes in `src/striatum/workflow.py`:

- Add optional top-level fields `repositories` and `primary_repository`; do not
  make them required for existing single-repo workflows.
- Add `_validate_repositories_block(workflow)` returning a normalized
  `CrossRepoLayout(primary_alias, aliases)` value object for downstream
  validation.
- Reject an empty `repositories` object, duplicate `repo_id` values, aliases
  that are empty or not identifier-like, non-string `repo_id`, unknown
  `primary_repository`, and any job `repository` that does not name an alias.
- Extend reviewer access scope values with
  `cross_repo_artifact_augmented`, valid only for review jobs in workflows with
  a `repositories` block.
- Preserve existing artifact path safety and write-scope checks, but qualify
  uniqueness by repository alias. Two jobs in different repos may both publish
  `docs/report.md`; two jobs in the same repo may not.
- Extend parallel-group write-scope overlap detection by repository alias.
  Overlap is invalid only within the same target repository.
- Require `parallelism.per_repo_max_active_jobs`, when present, to be an object
  mapping repository aliases to positive integers.
- For cycles crossing repositories, require `cross_repo_cycle: true` on the
  cycle. `max_iterations` remains one global counter for the cycle, not one
  counter per repository.

Do not make `workflow validate` require a daemon connection. Local schema
validation can verify shape and aliases. `run prepare` performs daemon-backed
registration checks because only the daemon DB knows active repository ids.

## Cross-Repo State

Add daemon PostgreSQL migration `src/striatum/daemon_pg/sql/0003_cross_repo.sql`
and advance `LATEST_DAEMON_DB_VERSION` to 3. Minimum daemon-owned tables:

```text
striatumd.cross_repo_runs
  cross_repo_run_id text primary key
  workflow_id text not null
  workflow_version text
  workflow_snapshot_hash text not null
  primary_repository_id text not null references striatumd.repositories(repository_id)
  state text not null check (state in
    ('preparing','prepared','started','blocked','canceling','canceled','completed','aborted'))
  created_at timestamptz not null
  started_at timestamptz
  completed_at timestamptz
  reconcile_after timestamptz
  last_reconcile_error text

striatumd.cross_repo_run_repositories
  cross_repo_run_id text references striatumd.cross_repo_runs(cross_repo_run_id)
  repository_alias text not null
  repository_id text not null references striatumd.repositories(repository_id)
  local_run_id text
  state text not null check (state in ('pending','prepared','started','canceled','completed','aborted'))
  primary key (cross_repo_run_id, repository_alias)
  unique (repository_id, local_run_id)

striatumd.cross_repo_cycle_counters
  cross_repo_run_id text not null
  cycle_key text not null
  iterations integer not null default 0
  max_iterations integer not null
  primary key (cross_repo_run_id, cycle_key)
```

Add a repo-local SQLite migration after v13:

```text
cross_repo_run_pointers
  cross_repo_run_id text not null
  local_run_id text not null references runs(run_id)
  repository_alias text not null
  repository_id text not null
  primary_repository integer not null check (primary_repository in (0,1))
  created_at text not null
  primary key (cross_repo_run_id, local_run_id)
```

This pointer is intentionally small. Repo-local SQLite still owns local
`runs`, `jobs`, `sessions`, `artifacts`, `events`, and `verdicts`.

## Run Lifecycle

Add cross-repo lifecycle handlers behind daemon RPC in a new module such as
`src/striatum/cross_repo.py`, with `daemon_rpc/server.py` delegating to it for
cross-repo methods.

`run.prepare` behavior:

- Direct repo-local mode refuses a workflow with `repositories` using exit code
  10 or a stable workflow error, with guidance to use daemon mode.
- Daemon mode loads and schema-validates the workflow once, resolves every
  repository alias to an active `striatumd.repositories` row, and verifies the
  token has `write` for every participating repository or daemon-global
  `write`.
- The daemon inserts `cross_repo_runs(state='preparing')` and
  `cross_repo_run_repositories(state='pending')` in one Postgres transaction.
- The daemon then creates repo-local runs in each participating repository by
  calling the existing `create_run` path with a filtered local workflow
  snapshot. Each local snapshot contains all jobs for graph context but jobs
  outside that repository are stored as dependency placeholders only if the
  current scheduler needs local edge visibility. Prefer the smaller approach:
  local repos store only local jobs; cross-repo edges are held by the daemon.
- After each local run exists, insert the repo-local
  `cross_repo_run_pointers` row and update
  `cross_repo_run_repositories.local_run_id`.
- When all local rows are present, mark the daemon row `prepared`.

Crash consistency is best effort:

- A crash before the daemon transaction commits leaves no cross-repo row.
- A crash after `preparing` but before all local rows exist leaves a durable
  daemon row for reconciliation.
- Daemon startup runs `reconcile_cross_repo_preparing()`. It inspects each
  participant. If missing local rows can still be created and every repo is
  registered, it completes preparation and marks `prepared`. If a repo is
  removed, schema-incompatible, or cannot be opened, it marks the cross-repo
  run `aborted` and records `last_reconcile_error`.
- Reconciliation is idempotent by `(cross_repo_run_id, repository_alias)` and
  by local pointer lookup, not by guessing from workflow titles.

`run.start` behavior:

- `run.start --run-id <cross_repo_run_id>` is daemon-routed.
- The daemon checks all participant rows are `prepared`, starts each local run
  with existing branch/start semantics, then marks `cross_repo_runs.started`.
- If one local start fails, the daemon leaves the cross-repo run `blocked` and
  records a human-checkpoint style blocker in the primary repository. It does
  not roll back already-started local runs by deleting SQLite rows.

`status`, `dashboard`, `why`, and `run summary`:

- CLI run-id resolution first asks the daemon whether the id is a
  `cross_repo_run_id`. If yes, aggregate local status by participant.
- `dashboard --run-id <cross_repo_run_id>` shows one run header, participant
  repo states, global claimable/running/blocked counts, cross-repo blockers,
  and local run ids for drill-down.
- `cross-repo describe <id>` returns the mapping table and workflow aliases.
- `cross-repo why <id>` aggregates open blockers and degraded participant
  state.
- `run summary --run-id <cross_repo_run_id>` writes a summary from daemon
  metadata plus per-repo local summaries; artifact paths are displayed as
  `<alias>:<repo_path>`.

`run cancel`:

- Add a daemon-routed cross-repo cancel path that attempts local cancel for
  every non-terminal participant. The daemon state is `canceling` while any
  participant remains in flight, `canceled` when all cancel, and `blocked` if a
  participant cannot be reached.

Repo removed mid-run:

- Treat unregistration or removed repo state as a cross-repo blocker. The
  daemon refuses further cross-repo scheduling and opens a human checkpoint in
  the primary repository. Operator options are re-register the same repo,
  cancel the cross-repo run, or use a future explicit repair command. Do not
  silently retarget an alias to a different repository id.

## Scheduling and Cross-Repo Edges

The daemon owns cross-repo scheduling decisions for workflows with
`repositories`; repo-local schedulers continue to own single-repo runs.

Implementation approach:

- Keep local `claim-next`, `ack`, `publish-artifact`, `complete`, `verdict`,
  and `submit-review` semantics unchanged once a job is selected.
- Add daemon methods that resolve a cross-repo run id and target repository,
  then call the existing local command through the registered repo root.
- For cross-repo edges, the daemon observes local terminal events and releases
  downstream jobs in the target repository when upstream gates are satisfied.
- Store cross-repo cycle counters in Postgres and increment under row lock
  whenever a `needs_revision` cycle is taken. The counter key should be
  deterministic, for example `<from_job_id>-><to_job_id>:needs_revision`.
- Enforce `parallelism.max_active_jobs` globally by counting active jobs across
  participant local states. Enforce `per_repo_max_active_jobs` before
  releasing or claiming work in that repository.

This design avoids a distributed transaction for every job transition. The
daemon is the only cross-repo scheduler, and each local mutation remains a
normal repo-local mutation with daemon audit around it.

## MCP Mutation Surface

Extend `src/striatum/daemon_rpc/registry.py` first. Add `recovery` to the
closed capability set and move recovery-specific methods from `admin` to
`recovery` where appropriate:

```text
recovery.stale_leases
recovery.requeue_stale
recovery.process_reconcile
recovery.resume
recovery.cancel_job
```

Add cross-repo methods:

```text
cross_repo.list          read, daemon scope
cross_repo.describe      read, daemon scope
cross_repo.why           read, daemon scope
cross_repo.cancel        admin or recovery, daemon scope
cross_repo.reconcile     recovery, daemon scope
```

Keep ordinary workflow mutations mapped exactly once in the method registry:
`session.register` and `ack` require `write`; `claim_next` requires `claim`;
`verdict` and `submit_review` require `review`; `apply.reviewed_patch`
requires `apply`.

Refactor `src/striatum/mcp.py` so daemon MCP no longer has a hand-written tool
list. It should build MCP tools from `describe_methods()` and each
`MethodEntry`:

- `tools/list` requires a token parameter and returns only entries where the
  token has the required capability and repository scope. Methods with
  `required_capability is None` may be omitted from MCP tools unless explicitly
  useful.
- Cache effective tool sets by `(token_id, methods_etag)` inside the server
  instance. Invalidate naturally when token grants change by changing token id,
  expiry, or registry etag; do not persist this cache.
- `tools/call` validates that the method exists in the registry, authorizes
  through `daemon_rpc.capability.authorize`, and dispatches to
  `DaemonRpcRouter.handle` with `transport="mcp"` audit metadata.
- Unknown MCP tool name returns the standard JSON-RPC method/tool-not-found
  error. If the name maps to a known daemon method but the token lacks scope,
  return MCP invalid params / tool error with the daemon stable denial reason.
- Capability denials are audited with `decision='denied'` and
  `denial_reason` values from the existing vocabulary:
  `token_missing`, `token_malformed`, `token_invalid`, `token_revoked`,
  `token_expired`, `capability_missing`, `capability_expired`,
  `repo_not_registered`, or `method_unknown`.

Repository scope rules:

- A repo-scoped token can call a repository-scoped method only when the request
  `repository_id` matches the grant.
- Daemon-global tokens may call across repositories.
- Cross-repo run methods require either daemon-global capability or a grant for
  every participating repository. For `apply`, keep the stricter single-repo
  rule: an `apply` token scoped to repo A cannot apply against repo B, and one
  cross-repo apply spanning multiple repos remains out of scope.

There is no V2 `--allow-mutations` bypass. MCP clients are untrusted callers
whose authority is exactly their token's effective capability set.

## Audit Shape

Reuse `src/striatum/daemon_rpc/request_log.py` for MCP. Add a `transport`
argument to the router or audit append path so rows can record `rpc` vs `mcp`
without changing authorization semantics.

Every MCP `tools/call` records:

```text
method
transport = 'mcp'
client_id
token_id hash reference, never token secret
repository_id when single-repo scoped
cross_repo_run_id when present
params_hash
response_hash
decision = allowed|denied
denial_reason
daemon_version
substrate_version
```

For cross-repo calls, add `cross_repo_run_id` and either a nullable
`repository_id` plus a participant list hash, or a side table
`striatumd.audit_repositories(audit_id, repository_id)`. Prefer the side table:
it keeps existing single-repo audit queries simple while making multi-repo
activity inspectable.

Audit remains metadata-only. Do not store request bodies, response bodies,
artifact contents, token secrets, transcripts, blocker prose, or model
rationales.

## Concrete Touch Points

- `src/striatum/workflow.py`: repositories block validation, job repository
  defaults, cross-repo reviewer access scope, repository-qualified artifact
  uniqueness, parallelism, and cycle checks.
- `src/striatum/daemon_pg/sql/0003_cross_repo.sql`: daemon cross-repo run,
  participant, cycle-counter, and optional audit participant tables.
- `src/striatum/daemon_pg/migrations.py`: register migration v3.
- `src/striatum/migrations.py`: repo-local `cross_repo_run_pointers`
  migration.
- `src/striatum/daemon_rpc/registry.py`: add `recovery`, cross-repo routes,
  and ensure every MCP-exposed method has one capability declaration.
- `src/striatum/daemon_rpc/server.py`: dispatch cross-repo lifecycle methods
  and allow one router to resolve multiple registered repo roots instead of
  refusing any root different from `self.repo_root`.
- `src/striatum/mcp.py`: split local repo MCP from daemon MCP or add a daemon
  mode that uses method-registry generated tools, per-token `tools/list`, and
  `tools/call` through `DaemonRpcRouter`.
- `src/striatum/cli/parser.py` and `src/striatum/cli/dispatch.py`: add
  `cross-repo list|describe|why`, route cross-repo run ids through daemon, and
  refuse direct mode for cross-repo mutation.
- `src/striatum/cli/introspect.py` / dashboard modules: aggregate participant
  local statuses for cross-repo run ids.

## Test Strategy

Do not build the full multi-repo daemon harness in this dogfood. That is
explicitly deferred to `docs/TODO.md` Open item 19 and should land before a
future end-to-end implementation hardens RFC 0032.

Dogfood-035 implementation should still ship focused coverage:

- `tests/test_workflow_cross_repo.py`: validator accepts `repositories`,
  applies primary defaults, rejects unknown aliases, permits duplicate artifact
  paths across different repos, rejects duplicates within one repo, validates
  `cross_repo_artifact_augmented`, and requires `cross_repo_cycle: true` for
  cross-repo cycles.
- `tests/test_daemon_pg.py`: migration v3 creates cross-repo tables and keeps
  daemon audit append-only invariants.
- `tests/test_cross_repo_lifecycle.py`: mock registered repositories and fake
  repo-local `create_run` calls; assert preparing to prepared, idempotent
  reconciliation, abort on missing repo, and local pointer creation.
- `tests/test_cross_repo_write_scope.py`: use mocked repo roots to prove a job
  targeting repo A resolves artifact and write-scope paths under repo A only
  and cannot publish into repo B.
- `tests/test_daemon_rpc_registry.py`: registry includes `recovery` and every
  MCP-exposed method declares capability/scope metadata.
- `tests/test_mcp_mutation_capabilities.py`: `tools/list` filters by read,
  write, review, claim, apply, admin, and recovery grants; `tools/call` refuses
  missing, expired, revoked, and repo-mismatched tokens; denial rows are
  audited.
- `tests/test_mcp_prompt_injection_shapes.py`: requests that try to smuggle a
  higher capability in tool arguments still fail because authorization uses the
  token grant, not caller prose or params.

The eventual follow-up harness should add two initialized repositories, a real
daemon instance, restart/reconcile tests around `preparing`, real cross-repo
edge progression, and artifacts landing in separate working trees.

## Risks and Guardrails

The largest implementation risk is pretending cross-repo run creation is a
distributed transaction. It is not. Keep the product language to
daemon-mediated coordination plus best-effort startup reconciliation.

The second risk is duplicating authorization between MCP and RPC. Avoid that by
making MCP a transport facade over the RFC 0030 method registry and
`authorize()` helper. A tool that is not in the method registry should not
exist.

The third risk is path confusion. All repo-relative paths must be resolved only
after the target repository alias is known. Display paths should include the
alias in aggregate surfaces, but stored local artifact paths should remain
plain repo-relative paths for compatibility.

The fourth risk is overclaiming provenance. Cross-repo coordination does not
make bylines stronger, does not make sealed apply cross-repo atomic, and does
not make MCP clients trustworthy. It only concentrates documented local
authority behind capability tokens and metadata-only audit.
