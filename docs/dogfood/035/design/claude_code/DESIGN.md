author: designer-claude-opus-001

# RFC 0032 Implementation Design: Cross-Repo Workflows + MCP Mutation Capabilities

Status: design (handoff)
Date: 2026-05-12
Target RFC: [`docs/rfcs/0032-cross-repo-workflows-and-mcp-mutation-capabilities.md`](../../../../rfcs/0032-cross-repo-workflows-and-mcp-mutation-capabilities.md)
Foundation: dogfood-034 ([`DESIGN_SYNTHESIS.md`](../../../034/DESIGN_SYNTHESIS.md), [`BUILD_HANDOFF.md`](../../../034/BUILD_HANDOFF.md))

This design sits **on top of** the dogfood-034 V2 foundation. It does not
redesign the envelope, framing, transport, handshake, capability gate, audit
chain, request log, registry, supervisor migration, or sealed-apply scaffold
those landed. Every section here adds to those mechanisms; nothing replaces
them.

## 0. Scope and Deferrals

In scope:

- Workflow `repositories` block + cross-repo validator extensions.
- Cross-repo run lifecycle: `run prepare`, `run start`, `run summary`,
  `run cancel` — daemon-mediated coordination with best-effort consistency.
- Daemon-DB + repo-local-DB coordination during cross-repo runs.
- MCP `tools/call` and `tools/list` gating against the dogfood-034 capability
  vocabulary. RFC 0032 adds `recovery` to the in-code vocabulary set (the
  baseline SQL already permits it).
- Per-token `tools/list` filtering and per-token effective-tool-set caching.
- Audit row appended for every mutating MCP `tools/call`, allowed and denied.
- Prompt-injection-resistant operator UX for token issuance and revocation.

Out of scope — claims this dogfood **must not** make:

- Cross-machine / multi-host coordination. Cross-repo = cross-registered-
  local-repo only (RFC 0028 D082/D083; RFC 0032 §2 "no cross-host semantics").
- Multi-tenant or multi-OS-user MCP. Single OS user per daemon.
- Malicious-local-root resistance. Per the RFC 0031 §Threat Model the scope
  is over-eager AI agents + operator-mistake footguns; an operator with the
  daemon's OS user can read the daemon DB, the signing key, and any token.
  Documentation must repeat this verbatim.
- Atomic file-system mutations spanning two repositories. The daemon
  coordinates run state, verdict ordering, and refusal — it does not promise
  that two repos' working trees converge atomically. That remains the
  workflow author's responsibility.
- A multi-repo end-to-end daemon test harness. Per `docs/TODO.md` Open
  item 19 and the role spec, harness-level cross-repo integration tests
  land in a follow-up RFC. This dogfood ships unit-level + mock-based
  coverage only.

## 1. Trust Boundary Model

Five distinct trust zones interact in V2 cross-repo + MCP mutation flows.
The boundary between them is the daemon RPC envelope (RFC 0030 §1) plus
the capability token check (RFC 0030 §4, extended here).

| Zone | Identity | Trust posture | What it must NOT do |
|---|---|---|---|
| **Operator** | Local OS user that owns the daemon process and DB. | Inside the trust boundary by definition (RFC 0031 §Threat Model). | We do not claim to defend against the operator. They can read the daemon DB, dump tokens, kill the daemon. Documentation states this explicitly. |
| **Daemon process** | Long-running `striatumd` with PG connection + signing key. | Acts on behalf of the operator. Sole writer of `striatumd.*` tables, daemon-owned worktrees, and supervisor children. | Trust an MCP client's claim of identity. Bypass capability gating because a route is "internal" or "trusted." Invent provenance across repos. |
| **MCP client** | Any stdio JSON-RPC peer (Claude Desktop, IDE plugin, third-party tool, or a prompt-injected agent). | **Untrusted by default.** All authority derives from a capability token an operator explicitly issued. | Be assumed honest about its purpose. Be granted blanket write/apply/admin because it "looks like" a coding tool. |
| **Supervised lane process** | Daemon-spawned agent CLI (Claude Code, Codex, Gemini). | Trusted only within `write_scope.allowed_paths` of its current work packet, attested by `pid_start_time` per RFC 0026. | Read or write outside its lane scope. Self-attest a byline that doesn't match its supervisor row. Skip the daemon when mutating cross-repo state. |
| **Cross-repo coordinator** | A coordinator session registered against a cross-repo run, holding a multi-repo `read` scope. | Routes packets and monitors state. Authority capped at coordinator role (RFC 0032 §4). | Perform role work. Mutate cross-repo state outside the documented `run.*` RPC surface. |

Boundary rules that the implementation enforces, not just documents:

1. The MCP client and the supervised lane process are **separate trust
   zones** even when they're the same physical process. A supervised lane
   that opens its own MCP connection to the daemon authenticates again via
   its own capability token; it does not inherit daemon-side authority from
   the fact that the daemon spawned it.
2. The daemon never reads MCP stdout/stderr to derive authority. All
   authority enters via the envelope's `capability_token`.
3. Cross-repo state transitions cross the daemon trust boundary. There is
   no client-side path (CLI or MCP) that writes to two repos' SQLite stores
   without going through the daemon's two-phase commit.
4. The coordinator session's multi-repo `read` scope does not imply any
   `write`, `claim`, `review`, `apply`, `admin`, or `recovery` authority
   anywhere.

## 2. Capability Authorization for Mutation Tools

### 2.1 Vocabulary

Adopt the full RFC 0032 §5 vocabulary:

```text
read      introspection and describe routes
write     ordinary workflow mutations (claim, ack, publish, complete, ...)
review    record verdicts, submit reviews
claim     claim work packets
apply     sealed-apply authority (RFC 0031)
admin     register repos, create/revoke tokens, key rotation, recovery cancel
recovery  run recovery sweeps and resume blockers
```

Implementation work to formalize `recovery`:

- `src/striatum/daemon_rpc/registry.py` — extend `Capability` type and
  `CAPABILITIES` set to include `"recovery"`. The baseline SQL constraint
  in `daemon_pg/sql/0001_baseline.sql` already permits it.
- Method registry entries for `recovery.cancel_job`, `recovery.requeue_stale`,
  `recovery.process_reconcile`, `recovery.stale_leases` move from `admin`
  to `recovery`. `daemon.token.*`, `repo.*`, `daemon.shutdown`,
  `daemon.migrate`, `daemon.key.rotate` remain `admin`.
- Migration note: existing tokens with `admin` continue to invoke recovery
  routes because the route registry will accept either `admin` or `recovery`
  for `recovery.*` (intersection check, not equality). Documented as a
  one-release compatibility window.

### 2.2 Token shape and scope

Token shape is unchanged from dogfood-034: `<token_id>.<secret>`, salted
hash in `striatumd.clients.token_hash`, capabilities joined via
`striatumd.client_capabilities`. RFC 0032 enforces these semantic rules:

- **Repo-scoped tokens cannot cross repos.** Capability rows with non-NULL
  `repository_id` only authorize routes whose effective `repository_id`
  matches. For a cross-repo run, the daemon evaluates capability per
  participating repo, not against the `cross_repo_run_id`. A token scoped
  to repo A can never invoke `publish_artifact` against repo B even when
  both repos participate in the same cross-repo run.
- **Daemon-global tokens** (capability row `repository_id IS NULL`) still
  carry across repos, by design. These remain admin-only in normal
  operation; documentation lists them as the highest-blast-radius grant.
- **Cross-repo route gating.** A new `repository_scope_mode` column on
  the in-code `MethodEntry` selects between:
    - `single_repo` (current default — `repository_scope=True`): caller
      must supply `repository_id` and the token must cover it.
    - `cross_repo`: caller supplies `cross_repo_run_id`; daemon resolves the
      participating `repository_ids` and the token must cover **all** of
      them. Missing any participating repo's capability is denied with
      `capability_missing_for_participant` (new denial sub-reason in the
      closed vocabulary).
    - `daemon_global` (current `repository_scope=False`).
  This avoids accidentally upgrading single-repo route semantics into
  cross-repo while still expressing the multi-repo coverage check.

### 2.3 Token lifecycle

- **Issuance.** `daemon.token.create` (admin-only) accepts
  `capabilities[]`, optional `repository_id` per capability, and
  `expires_in_seconds`. The CLI surface adds
  `striatum daemon token create --capability write --repo-id repo_abc
  --expires-in 1h` and a verbose form that takes a JSON capability list.
- **Expiry.** `striatumd.clients.expires_at` and per-capability
  `striatumd.client_capabilities.expires_at` are both honored
  (dogfood-034 already enforces both). Expired tokens are denied with
  `token_expired` or `capability_expired`; the difference is preserved
  so the operator can tell whether the whole token or only the relevant
  capability lapsed.
- **Revocation.** `daemon.token.revoke` (admin-only) sets `revoked_at`
  on the client row, denying future requests with `token_revoked`. Adds
  a `--reason` field stored on the row (already exists on
  `client_capabilities.revoked_reason`; mirror onto `clients`).
- **Rotation.** RFC 0032 does not require operator-visible token rotation
  beyond "revoke and reissue." `daemon.token.rotate` is a convenience
  admin route that atomically issues a new token with the same capability
  set and revokes the old token with `revoked_reason="rotation"`. Audit
  records both the create and the revoke.

### 2.4 Mutation-default-short

Operator UX nudges short-lived tokens for any token that includes
`write`, `review`, `claim`, `apply`, or `recovery`:

- `daemon.token.create` defaults `expires_in_seconds=3600` (1 hour) when
  any mutation capability is in the request and `--expires-in` is not
  provided. `read`-only and `admin`-only tokens default to no expiry but
  warn at issuance.
- Documentation in `docs/HOW_TO_HUMAN.md` recommends one-hour tokens for
  ad-hoc MCP mutation sessions and explicitly long-lived only for
  daemon-internal supervised lanes that need stable identity.

### 2.5 Audit shape for mutating `tools/call`

Every MCP mutation request (`tools/call` against a method whose registry
entry requires a mutation capability) records:

- Authorization audit row in `striatumd.audit_log` with the dogfood-034
  shape: `ts, schema_version, daemon_version, client_id, repository_id,
  method, decision, denial_reason, transport='mcp', request_id,
  exit_code, params_sha256, previous_hash, row_hash, segment_id`. For
  denied requests, `decision='denied'` and `denial_reason` from the
  closed vocabulary.
- Request log row in `striatumd.rpc_request_log` referencing the audit_id.
- The denial vocabulary expands by these RFC 0032 entries (closed set,
  documented in MCP.md):
    - `capability_missing` — token lacks the required capability for the route.
    - `capability_missing_for_participant` — cross-repo route, token lacks
      capability for one or more participating repos.
    - `capability_scope_mismatch` — capability row exists but for the wrong
      `repository_id`.
    - `cross_repo_run_unknown` — `cross_repo_run_id` not registered.
    - `cross_repo_repo_unregistered` — a participating repo was unregistered
      after `run prepare` and the run has not been resumed.
    - `cross_repo_state_invalid` — caller asked for a transition the
      cross-repo run cannot accept (e.g. `run cancel` on a `completed` run).
    - `tools_list_filter_mismatch` — `tools/call` invoked a method that the
      caller's filtered `tools/list` would not have surfaced; allowed only
      because hidden ≠ unauthorized, but recorded so operators can spot
      out-of-band tool invocation. (Optional informational flag, not a
      denial reason by itself.)

`params_sha256` continues to use canonical JSON params (no token, no body
contents). MCP `tools/call` params are normalized before hashing so that
`{"arguments": {...}}` and the underlying RPC params produce the same
hash (one method, one hash regardless of caller surface).

The audit chain link (`previous_hash`, `row_hash`) is preserved exactly as
in dogfood-034; cross-repo activity adds rows to the same chain.

## 3. Default-Deny Gating

Implementation rules, in `daemon_rpc/server.py` and `mcp.py` daemon
surface:

1. **Unknown methods.** A method name not in `METHOD_REGISTRY` is refused
   with `method_unknown` (existing dogfood-034 behavior). MCP returns the
   JSON-RPC `-32601 method_not_found` error so MCP clients see the
   standard unknown-method shape, and the daemon also records an audit
   row with `decision='denied', denial_reason='method_unknown'`.
2. **Registry entry without a declared capability.** If a `MethodEntry`
   is added with `required_capability=None` and is **not** `daemon.hello`,
   the daemon refuses to start. `daemon.welcome.data` reports the
   refusal so the operator can fix the registry. This is implemented as a
   startup assertion in `daemon_rpc/registry.py`:

   ```python
   for entry in _ENTRIES:
       if entry.required_capability is None and entry.method != "daemon.hello":
           raise StriatumError(
               f"daemon RPC method {entry.method!r} has no required_capability; "
               "registry must declare one or be exempt by name",
               exit_code=9,
           )
   ```

   This makes "forgot to declare a capability" a build-time / boot-time
   failure rather than a silent open route. The handshake exemption for
   `daemon.hello` is the only allow-listed `None`.
3. **Per-request fail-closed.** In `DaemonRpcRouter.handle`, if
   `entry.required_capability is None and method != "daemon.hello"`,
   raise `RpcError("capability_missing", ...)` and audit denied. Belt-
   and-suspenders against a future code change that bypasses the
   startup assertion.
4. **No "trusted identity" bypass.** There is no code path in the
   daemon that skips capability authorization based on a client claim
   in `daemon.hello.client.name`. Reviewers should grep for any callable
   that constructs an `RpcAuthContext(decision='allowed', ...)` without
   going through `authorize()`; the only legitimate caller is
   `daemon.hello` itself (which has no capability).
5. **No global mutation flag.** V1's `striatum serve --allow-mutations`
   (RFC 0012) is retired in V2 per RFC 0032 §8. The CLI parser must not
   accept `--allow-mutations` on `daemon start` or `striatum serve`. If
   present in a config file from a V1 install, refuse startup with a
   documented message pointing at `daemon.token.create`.
6. **MCP-specific gates.** `tools/list` filters by capability (§4 below);
   `tools/call` re-checks capability **after** filtering. A tool that
   was hidden from `tools/list` is still callable by name if the caller
   somehow learned it; the gate that matters is the capability check on
   `tools/call`, not the visibility filter.

## 4. Prompt-Injection Mitigation

The threat: a prompt-injected MCP client (or one that is intentionally
malicious from the start) tries to invoke `tools/call` on
`publish_artifact`, `apply.reviewed_patch`, `daemon.token.create`, or
similar mutation surfaces.

Defenses, in priority order:

1. **Capability tokens are the only access path.** No environment-variable
   fallback, no "first connect wins" implicit token. If the MCP client
   does not present a `capability_token` (or `Authorization: Bearer` on
   loopback HTTP), the call is denied `token_missing`. This is the
   load-bearing defense; everything else is hardening.
2. **Short-lived mutation tokens.** §2.4 makes 1-hour the default for
   any token with a mutation capability. The operator UX
   (`daemon.token.create`) prints the expiry and a `daemon.token.revoke`
   one-liner the operator can paste into a terminal if the token is
   leaked.
3. **Per-token `tools/list` filtering.** A token without `apply` does not
   see `apply.reviewed_patch` in `tools/list`. This is not security
   (point 5 below), but it reduces the surface a prompt injection can
   plausibly call out by name, and it shapes the operator's expectations
   about what each client can do. Implementation:
    - `DaemonMcpServer.tools_list(token)` computes the effective tool
      set by intersecting `METHOD_REGISTRY` with the token's capability
      set + scope.
    - Cache keyed by `(token_id, methods_etag, capability_revision)`
      where `capability_revision` is a monotonic counter on
      `client_capabilities` updates. Cache invalidates on revoke /
      capability grant. Cache value is the filtered tool list, not the
      full registry.
4. **Per-token revocation UX with audit timeline.**
    - `daemon.token.revoke --token-id <id> --reason "leaked in chat"`
      sets `revoked_at`, `revoked_reason`. Subsequent calls denied
      `token_revoked`.
    - The audit chain shows, for any token_id: every allow before
      revocation, the revocation event, every deny after. An operator
      investigating a suspected leak runs
      `striatum daemon audit show --token-id <id>` to see the timeline.
      (This is a doctor-internal command in V2; surfacing externally is
      explicitly deferred.)
5. **Hidden ≠ unauthorized.** Documentation in `docs/MCP.md` states
   explicitly that `tools/list` filtering is an operator UX feature, not
   a security boundary. The capability check on `tools/call` is what
   refuses unauthorized mutations.
6. **Prompt-injection scenario test fixtures.** A documented prompt-shape
   test (see §9.4) sends `tools/list` then `tools/call` with elevated
   arguments. The test asserts:
    - filtered tool list omits the elevated tool;
    - calling the elevated tool by name is refused `capability_missing`;
    - the denial audit row is written before the response goes out.

What we deliberately don't claim:

- We do not claim resistance to a model that has been granted a
  long-lived `admin` token. That is operator-misconfiguration territory,
  which the threat model declares out of scope. Documentation warns
  against it; the daemon does not refuse it.

## 5. Cross-Repo Run Lifecycle

The lifecycle is `prepare → start → run-summary / dashboard → cancel`,
each routed through a new daemon RPC family `cross_repo.*`. Existing
single-repo routes (`run.prepare`, `run.start`, etc.) continue to handle
single-repo workflows unchanged.

### 5.1 Method registry additions

```text
cross_repo.prepare        write    cross_repo
cross_repo.start          write    cross_repo
cross_repo.summary        read     cross_repo
cross_repo.cancel         write    cross_repo
cross_repo.describe       read     cross_repo
cross_repo.why            read     cross_repo
cross_repo.list           read     daemon_global
```

Capability is `write` for state-changing routes. The cross-repo coverage
rule from §2.2 applies — caller token must hold `write` for every
participating repo.

CLI surface (RFC 0032 §7):

```text
striatum run prepare --workflow <path>          # auto-detects cross-repo, routes via cross_repo.prepare
striatum cross-repo list
striatum cross-repo describe <cross_repo_run_id>
striatum cross-repo why <cross_repo_run_id>
striatum status --run-id <cross_repo_run_id>    # CLI client resolves to cross_repo.summary
striatum dashboard --run-id <cross_repo_run_id> # cross_repo.summary + per-repo dashboards
striatum cross-repo cancel <cross_repo_run_id>
```

### 5.2 `cross_repo.prepare`

Two-phase commit inside the daemon process. Sketch in `daemon_rpc/server.py`
delegating to `daemon.cross_repo.prepare_cross_repo_run`:

```text
1. Validate the workflow (single SQL transaction in the daemon DB):
   - workflow.validate runs locally as today; validator extended to
     accept the `repositories` block and per-job `repository` field.
   - For each participating repo: SELECT repo_root, state FROM
     striatumd.repositories WHERE repository_id = $1 FOR UPDATE.
     Refuse `cross_repo_repo_unregistered` if any row missing or state
     != 'active'.
2. BEGIN daemon-DB transaction:
   2a. INSERT into striatumd.cross_repo_runs(... state='preparing' ...).
       Record participating_repos JSONB.
   2b. For each participating repo (in deterministic order by
       repository_id):
       - Open the repo-local SQLite via the registered repo_root.
       - Run the existing repo-local `run prepare` logic to create the
         `runs` row in `.striatum/state.sqlite3`, INSIDE a repo-local
         transaction.
       - Update the cross_repo_run_participants table with the
         per-repo local_run_id.
       - Update the repo-local `runs` row's new `cross_repo_run_id`
         column.
   2c. UPDATE striatumd.cross_repo_runs SET state='prepared'.
3. COMMIT daemon-DB transaction; commit each repo-local transaction in
   the same critical section so failure rolls everything back.
```

Crash semantics:

- If the daemon crashes between 2a and 2c, the daemon DB has a
  `preparing` row and zero-or-more repo-local rows possibly committed.
  Repo-local SQLite transactions are independent commits, so partial
  state is possible. On daemon restart, **startup reconciliation**:

  ```text
  for row in SELECT * FROM striatumd.cross_repo_runs WHERE state='preparing':
      for participant in row.participants:
          if local runs row exists and is 'prepared': mark intact
          else: mark missing
      if all participants intact:
          UPDATE state='prepared'
      else:
          UPDATE state='aborted', abort_reason='preparation_crashed'
          # Best-effort: try to roll back local runs rows. Failure to
          # reach a repo (e.g. moved/deleted on disk) is logged and
          # the cross_repo_runs row stays 'aborted'; operator handles.
  ```

  Tests assert both `prepared` and `aborted` reconciliation paths.

- Two-phase commit caveat: this is **best-effort**, not distributed-
  database 2PC. We don't use Postgres prepared transactions across SQLite
  boundaries. The honest framing is: "daemon-mediated coordination with
  startup reconciliation," as RFC 0032 §2 already states. Documentation
  must repeat that wording, not "atomic across repos."

### 5.3 `cross_repo.start`

Single daemon-DB transaction that:

1. Verifies cross_repo_runs row is `prepared`.
2. Re-checks every participating repo is still `active` in
   `striatumd.repositories`. If any is unregistered, refuse
   `cross_repo_repo_unregistered`; the cross-repo run remains `prepared`
   (not transitioned to a half-started state).
3. Updates each repo-local `runs` row to `running` (same SQL as
   single-repo `run.start`).
4. Sets `striatumd.cross_repo_runs.state='running'`,
   `started_at=now()`.
5. Emits `cross_repo.run_started` event in the request log.

### 5.4 `cross_repo.summary` and `cross_repo.describe`

Read routes (`read` capability). `summary` aggregates:

- cross_repo_runs row (state, started_at, completed_at, primary_repo).
- For each participant: SELECT FROM that repo's
  `.striatum/state.sqlite3` (jobs, completion, blockers, current
  iteration). The daemon opens each repo-local DB read-only.
- Token scope rules: a multi-repo `read` token sees all participating
  repos. A token scoped to one participating repo sees only that
  repo's slice of the summary; the other participants are reported as
  `{repository_id, redacted: true}`. This is the same shape as
  existing `dashboard --all` capability filtering.

`describe` returns the workflow definition (repositories block, jobs,
edges, cycles, parallelism limits) without per-job state.

### 5.5 `cross_repo.cancel`

Cascades through participants. Single daemon-DB transaction:

1. Verify cross_repo_runs row is `running` or `prepared`.
2. For each participating repo (deterministic order):
   - Open repo-local SQLite, run existing `cli/dispatch.run cancel`
     logic against that repo's local_run_id. Cancellation is idempotent.
   - Capture the local cancel result.
3. Set `striatumd.cross_repo_runs.state='canceled'`,
   `canceled_at=now()`, `cancel_reason=<from param>`.
4. Commit all repo-local cancels + the daemon-DB transition together.

Partial cancellation failure (a repo-local cancel raises): the daemon
keeps the cross_repo_runs row in `canceling` state, logs which
participants succeeded, and surfaces the partial state via
`cross_repo.summary`. Operator must resolve via per-repo CLI or a
retry. We do not retry automatically.

### 5.6 Mid-run repo unregistration

Per RFC 0032 §10 Open Question: a participating repo is unregistered
(via `repo remove`) while a cross-repo run is `running`.

Implementation:

- `repo.remove` checks for active `cross_repo_runs` referencing the
  repository. If any, refuse `repo_in_active_cross_repo_run` unless
  `--force` is given.
- With `--force`, `repo.remove` marks the repo `removed`. A daemon
  background check (the existing recovery sweep, extended) notices that
  a `running` cross_repo_run has an unregistered participant and
  transitions the cross_repo_run to `paused_repo_unregistered`,
  recording a `human_checkpoint` event in the request log.
- `cross_repo.start` / `cross_repo.advance_job` / equivalent route
  refuses `cross_repo_repo_unregistered` while paused.
- Operator unblocks by re-registering the repo (`repo add`) — the
  daemon detects the participant is back and the operator runs
  `striatum cross-repo resume <cross_repo_run_id>` to flip back to
  `running` — or by `cross_repo.cancel`.

### 5.7 Edges and cycles across repos

- Edges may reference job ids in any participating repo (RFC 0032 §3).
  Validator: each edge's `from`/`to` resolves to a job in the workflow's
  `jobs` array; the job's `repository` decides which repo it belongs to.
- Cycles: `max_iterations` is global to the cycle, not per-repo
  (RFC 0032 §3). Implementation stores cycle iteration counts in
  `striatumd.cross_repo_runs.cycle_state` (JSONB keyed by cycle id),
  not in repo-local `runs.metadata_json`.
- Parallelism: `parallelism.max_active_jobs` is global to the cross-repo
  run; `parallelism.per_repo_max_active_jobs` is per-repo. Daemon enforces
  both before handing out claims.

## 6. Daemon-DB ↔ Repo-Local-DB Coordination

### 6.1 New daemon-DB schema (`0003_cross_repo.sql`)

```sql
-- Migration 0003 lifts daemon DB to substrate_version 3.
CREATE TABLE IF NOT EXISTS striatumd.cross_repo_runs (
  cross_repo_run_id text PRIMARY KEY,
  workflow_id text NOT NULL,
  workflow_version text,
  primary_repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  state text NOT NULL CHECK (state IN (
    'preparing','prepared','running','paused_repo_unregistered',
    'canceling','canceled','completed','aborted'
  )),
  cycle_state jsonb NOT NULL DEFAULT '{}'::jsonb,
  parallelism jsonb NOT NULL DEFAULT '{}'::jsonb,
  started_at timestamptz,
  completed_at timestamptz,
  canceled_at timestamptz,
  cancel_reason text,
  abort_reason text,
  paused_at timestamptz,
  paused_reason text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cross_repo_runs_state
  ON striatumd.cross_repo_runs(state)
  WHERE state IN ('preparing','running','paused_repo_unregistered','canceling');

CREATE TABLE IF NOT EXISTS striatumd.cross_repo_run_participants (
  cross_repo_run_id text NOT NULL REFERENCES striatumd.cross_repo_runs(cross_repo_run_id) ON DELETE RESTRICT,
  repository_id text NOT NULL REFERENCES striatumd.repositories(repository_id),
  participant_role text NOT NULL,  -- e.g. 'primary' | 'consumer'
  local_run_id text,                -- repo-local runs.run_id after prepare
  prepare_state text NOT NULL CHECK (prepare_state IN ('pending','prepared','missing','aborted')),
  prepared_at timestamptz,
  last_observed_at timestamptz,
  PRIMARY KEY(cross_repo_run_id, repository_id)
);

CREATE INDEX IF NOT EXISTS idx_cross_repo_run_participants_repo_state
  ON striatumd.cross_repo_run_participants(repository_id, prepare_state);
```

The participants table is a deliberate second table (not a JSON column)
so we can index by repo and reconcile crash state per participant. Cycle
state stays JSONB on the parent row since it's read/written as a single
object per route.

### 6.2 Repo-local schema additions

New migration to repo-local SQLite (migration v14):

```sql
ALTER TABLE runs ADD COLUMN cross_repo_run_id TEXT;
CREATE INDEX IF NOT EXISTS idx_runs_cross_repo_run_id ON runs(cross_repo_run_id)
  WHERE cross_repo_run_id IS NOT NULL;
```

Repo-local `runs` rows for single-repo workflows leave `cross_repo_run_id`
NULL; existing code paths are unaffected. Cross-repo participants set the
column at `cross_repo.prepare` time and never mutate it after.

A `cross_repo_run_id` column on `runs` (rather than a new
`cross_repo_runs` table inside `.striatum/state.sqlite3`) keeps the
authority direction one-way: the daemon DB is the canonical record;
repo-local is a back-reference. Doctor / status surfaces show the
back-reference for context.

### 6.3 Transaction ordering

Cross-repo writes follow a strict order:

```text
daemon-DB BEGIN                                         (T_d)
  daemon-DB write cross_repo_runs row                    (T_d)
  for each participant (sorted by repository_id):
    repo-local-DB BEGIN                                  (T_r_i)
      repo-local write runs row                          (T_r_i)
      repo-local write runs.cross_repo_run_id            (T_r_i)
    repo-local-DB COMMIT                                 (T_r_i)
    daemon-DB write cross_repo_run_participants row      (T_d)
  daemon-DB write cross_repo_runs.state='prepared'       (T_d)
daemon-DB COMMIT                                         (T_d)
```

Key properties:

- The daemon-DB transaction wraps the whole sequence. Daemon-DB
  abort rolls back the cross_repo_runs and participant rows.
- Repo-local SQLite commits are independent and **cannot** be rolled
  back from the daemon-DB transaction. If a repo-local commit succeeds
  and the daemon-DB transaction subsequently aborts, that participant
  has a `runs` row with no daemon-DB participant row pointing at it.
  On next daemon start, reconciliation looks for orphan participants
  (repo-local `runs.cross_repo_run_id` referencing a non-existent
  `cross_repo_runs` row) and marks them `cross_repo_orphaned`. The
  operator surfaces these via `striatum doctor`, which adds a
  `cross_repo_orphaned_runs` check.
- Repo-local writes happen sequentially, sorted by `repository_id`, to
  give deterministic locking order and make recovery scripts easier to
  read.

This is **best-effort consistency**, not 2PC across heterogeneous
databases. The honest documentation framing is:

> Daemon-DB writes are transactional. Repo-local SQLite writes commit
> independently. Daemon-DB rollback cannot undo a committed repo-local
> write; the orphan reconciler surfaces and the operator decides whether
> to manually clean the repo-local row. We do not promise distributed
> atomicity; we promise that the daemon-DB is the source of truth and
> that crash recovery never invents a `running` cross-repo run.

### 6.4 Crash matrix

| Crash point | daemon DB state | repo-local state | Reconciliation |
|---|---|---|---|
| Before T_d BEGIN | clean | clean | nothing to do |
| After cross_repo_runs INSERT, before any repo write | `preparing`, no participants | clean | reconciler marks `aborted` |
| After repo A commit, before repo B BEGIN | `preparing`, 1 participant | A: prepared, B: clean | reconciler tries to roll back A locally; marks `aborted` |
| After all repo commits, before daemon COMMIT | n/a (daemon rolled back) | all participants prepared | orphan reconciler surfaces; operator cleans |
| After daemon COMMIT | `prepared` | all participants prepared | normal |

Tests assert each row.

## 7. Workflow Schema Changes

In `src/striatum/workflow.py`:

- Add `repositories` to the optional top-level keys (currently
  `REQUIRED_TOP_LEVEL` is the closed set of required keys; add an
  `OPTIONAL_TOP_LEVEL` set including `repositories` and validate that no
  other top-level keys appear). Each entry maps a workflow-local key
  ("primary", "consumer", ...) to `{ "repo_id": "repo_..." }`.
- A workflow is **cross-repo** iff `repositories` is present and has
  at least 2 entries. (`repositories` with 1 entry is a validation
  error: use single-repo schema instead.)
- For cross-repo workflows:
    - Each job MUST declare `repository` matching a key in
      `repositories`. Missing → validation error
      `cross_repo_job_repository_missing`.
    - `write_scope.allowed_paths` is interpreted relative to that job's
      target repo. Path validation re-runs per repo at prepare time.
    - `expected_artifacts[].path` is per-target-repo.
- For single-repo workflows: `repository` on a job is a validation
  error (`unexpected_field`); preserves V1 strictness.
- Edge validation: every edge's `from`/`to` must resolve to a job in
  the workflow. Cross-repo edges are allowed; the validator does not
  require that an edge stay within one repo.
- Cycle validation: cross-repo cycles allowed only when the workflow
  declares `cross_repo_cycle: true` per cycle entry (RFC 0032 §10).
  Without that flag, a cycle whose `from`/`to` belong to different
  repos is a validation error.
- `primary_repository_id`: optional top-level field. If absent, the
  first entry in `repositories` (by JSON-object insertion order in the
  parsed dict, which Python 3.7+ preserves) is the primary.

`require_daemon`: cross-repo workflows are implicitly `require_daemon:
true`. An explicit `require_daemon: false` with `repositories` present
is a validation error. `provenance_mode: sealed_patch` continues to
imply `require_daemon: true`.

## 8. MCP Mutation Surface

In `src/striatum/mcp.py` `DaemonRpcServer` (the daemon MCP surface, not
the repo-local `LocalRpcServer`):

### 8.1 `tools/list` filtering

```python
def tools_list(self, *, token: str | None) -> list[JsonObject]:
    if token is None:
        return []  # token_missing — empty list, no leak of method names
    auth = peek_token_capabilities(self.pg_conn, token=token)
    if auth is None:
        return []  # token invalid/expired/revoked — empty list
    effective = []
    for entry in METHOD_REGISTRY.values():
        if entry.method in NON_TOOL_METHODS:  # daemon.hello, daemon.describe
            continue
        if entry.required_capability is None:
            continue  # defensive: only daemon.hello is allowed here
        if not _capability_covers(auth, entry):
            continue
        effective.append(_to_tool_spec(entry))
    return effective
```

`peek_token_capabilities` returns a struct with the token's capability
set, repo scopes, and revision counter. Cache result by
`(token_id, methods_etag, capability_revision)`.

`_capability_covers` checks both the capability name AND the
`repository_scope_mode`:

- `single_repo`: capability covers any repo OR the specific repo
  requested in the eventual call. For `tools/list` (no specific repo
  yet), include the tool if the token has the capability for at least
  one registered repo. Documented as "you'll see it; whether you can
  invoke it on repo X depends on your scope."
- `cross_repo`: include if the token has the capability for **all**
  registered repos (conservative; reviewers may want to relax to "any").
- `daemon_global`: include if the token has the capability with
  `repository_id IS NULL`.

### 8.2 `tools/call` gating

Every `tools/call` is re-authorized through the same
`authorize(required, repository_id, token)` path as RPC. The MCP
adapter normalizes:

- Method name from `params.name`.
- Arguments → params (drop `arguments` envelope, hash the inner).
- `repository_id`: from `arguments.repository_id` if the method is
  single-repo, or `cross_repo_run_id` for cross-repo routes (daemon
  resolves participants and applies the multi-repo check).
- `capability_token`: from the daemon MCP transport's per-connection
  token (currently a `token` field on the JSON-RPC request params, per
  V1 `docs/MCP.md`).

Audit row appended unconditionally, before the response is written.
Failure to write the audit row aborts the response with a server-side
internal error.

### 8.3 Initialize advertises the registry

`initialize` returns `capabilities.tools: {}` and `capabilities.resources:
{}` as today; clients call `tools/list` to discover. We do NOT widen the
`initialize` payload to include capability information about the caller;
that would invite "I have apply" claims by clients.

### 8.4 Discoverability

Documentation in `docs/MCP.md` shows the new shape:

```text
Daemon MCP exposes both resources and tools as of V2.
- tools/list filters by the token's capabilities (see §6 "MCP mutation
  capabilities"). A read-only token sees zero mutation tools.
- tools/call is capability-gated regardless of tools/list visibility.
- Audit rows are written for every mutating tools/call, allowed and
  denied. There is no global --allow-mutations flag in V2 — capability
  tokens are the only access path.
```

## 9. Test Strategy (No Multi-Repo Harness)

### 9.1 Unit-level workflow validator

- `tests/test_workflow_field_errors.py` extends to cover:
    - `repositories` shape (1 entry rejected, 0 entries rejected,
      non-object value rejected).
    - Per-job `repository` reference resolution.
    - `primary_repository_id` resolution + default-to-first.
    - Cross-repo workflow without `require_daemon: true` allowed
      (implicit), but explicit `require_daemon: false` rejected.
    - Cross-repo edge accepted; cross-repo cycle without
      `cross_repo_cycle: true` rejected.

### 9.2 Daemon-DB schema + reconciliation (mocked)

- `tests/test_daemon_pg.py` extends to assert migration 0003 applies
  cleanly and adds the cross-repo tables.
- A new `tests/test_cross_repo_lifecycle.py` exercises
  `prepare → start → summary → cancel` against a **single registered
  repo** treated as a "trivially cross-repo" workflow with 1
  participant. (Note: the validator rejects 1-participant cross-repo
  workflows; the test bypasses the validator and exercises the
  daemon-side coordination code directly with synthetic inputs. The
  validator rejection is also tested separately.)
- Reconciliation tests use synthetic `cross_repo_runs` rows in the
  `preparing` state to assert the reconciler transitions to `prepared`
  or `aborted` as appropriate.

### 9.3 Capability gating

- `tests/test_daemon_rpc.py` extends:
    - A token with `read` only is denied on `publish_artifact`
      (`capability_missing`).
    - A token with `apply` scoped to repo A is denied on
      `apply.reviewed_patch` against repo B
      (`capability_scope_mismatch`).
    - A token with `write` scoped to repo A is denied on a
      cross-repo route that requires write across A and B
      (`capability_missing_for_participant`).
    - A `tools/list` invocation with a read-only token returns no
      mutation tools; a `tools/call` to a hidden tool returns
      `capability_missing` with audit.
- A new `tests/test_default_deny.py`:
    - Asserts the startup assertion fires for a `MethodEntry` with
      `required_capability=None` other than `daemon.hello`.
    - Asserts `--allow-mutations` is refused on `daemon start` /
      `striatum serve`.

### 9.4 Prompt-injection shape

- `tests/test_mcp_prompt_injection.py`:
    - Sends `tools/list` with a read-only token → asserts mutation
      tools are absent.
    - Sends `tools/call` directly invoking `publish_artifact` →
      asserts `capability_missing` with audit row.
    - Sends `tools/call` invoking `daemon.token.create` →
      asserts `capability_missing` with audit row.
    - Sends a sequence: `daemon.hello`, then immediately a mutation
      `tools/call` without a token → asserts `token_missing`.

### 9.5 Cross-repo crash matrix

- `tests/test_cross_repo_crash_reconcile.py` uses a synthetic
  daemon-DB connection and synthetic repo-local SQLite files in
  `tmp_path` to assert each row of the crash matrix in §6.4.

### 9.6 Explicitly deferred

- A real two-repo daemon harness running an actual cross-repo
  workflow end-to-end is deferred to the follow-up multi-repo test
  harness RFC (TODO Open item 19). The synthesis must record this
  deferral; the build review must accept it.

## 10. Concrete Touch Points

| File | Change |
|---|---|
| `src/striatum/workflow.py` | `repositories` block, per-job `repository`, cross-repo edge/cycle validation, `cross_repo_cycle` flag, `primary_repository_id`. |
| `src/striatum/daemon_rpc/registry.py` | Add `recovery` to `Capability` / `CAPABILITIES`. Add `cross_repo.*` `MethodEntry` rows. Add `repository_scope_mode` field. Add startup assertion that every non-`daemon.hello` entry declares a capability. |
| `src/striatum/daemon_rpc/server.py` | Route `cross_repo.*` methods to `daemon.cross_repo` handlers. Extend capability check for `cross_repo` scope mode (multi-repo coverage check). Extend denial vocabulary. |
| `src/striatum/daemon_rpc/capability.py` | Add `_capability_covers_cross_repo(auth, participants)` helper. Wire new denial reasons (`capability_missing_for_participant`, `capability_scope_mismatch`). |
| `src/striatum/daemon.py` (or new `src/striatum/daemon_cross_repo/`) | `prepare_cross_repo_run`, `start_cross_repo_run`, `summarize_cross_repo_run`, `cancel_cross_repo_run`, `reconcile_cross_repo_preparing_rows` (startup hook), `handle_repo_unregister_cross_repo_impact`. |
| `src/striatum/mcp.py` | `DaemonRpcServer.tools_list` (capability-filtered), `tools_call` (re-auth + audit), per-token effective-tool-set cache, normalized params hashing. |
| `src/striatum/cli/mutations.py` and `src/striatum/cli/dispatch.py` | Add `cross-repo list/describe/why/cancel/resume` verbs. Route `run prepare/start/cancel` through daemon for workflows that have a `repositories` block. Refuse `--no-daemon` for cross-repo workflows. |
| `src/striatum/daemon_pg/migrations.py` | Bump `LATEST_DAEMON_DB_VERSION` to 3. Add `PgMigration(3, "cross-repo runs", "0003_cross_repo.sql")`. |
| `src/striatum/daemon_pg/sql/0003_cross_repo.sql` | Cross-repo tables (§6.1). |
| `src/striatum/migrations.py` | Add v14 migration: `runs.cross_repo_run_id` column + index. Bump `LATEST_VERSION` to 14. |
| `docs/SPEC.md`, `docs/MCP.md`, `docs/UBIQUITOUS_LANGUAGE.md`, `docs/CLI_REFERENCE.md`, `docs/HOW_TO_HUMAN.md` | RFC 0032 §"Acceptance Criteria" documentation updates. Add `recovery` capability description. Document `--allow-mutations` removal. Document orphan reconciler check in `doctor`. |
| RFC 0028 status block | Update from "V1 introspection only" to "V2 cross-repo mutation under RFC 0032." |

Existing dogfood-034 modules `daemon_apply/`, `daemon_supervisor/`,
`daemon_pg/audit.py`, `daemon_rpc/envelope.py`, `daemon_rpc/handshake.py`,
`daemon_rpc/request_log.py`, `daemon_rpc/framing.py`,
`daemon_rpc/transport_unix.py`, `daemon_rpc/transport_http.py` are
**leveraged, not redesigned**.

## 11. Compatibility and Migration

- V1 single-repo workflows: unchanged. No schema migration to the
  workflow JSON itself; only the validator's accepted-keys set grows.
- V1 `--allow-mutations` on `striatum serve`: removed. Documented in
  `docs/MCP.md` and `CHANGELOG.md`. Operators relying on it are
  pointed at `daemon.token.create`.
- V1 single-repo `runs` rows: untouched. Migration v14 adds a nullable
  column; existing rows are NULL.
- V2 dogfood-034 daemon DB: substrate version goes 2 → 3 via the new
  migration. Migrations remain forward-only; an operator who downgrades
  the daemon after running migration 3 sees `SchemaVersionError`.
- Tokens: existing tokens continue to work. The `recovery` capability
  is additive; tokens that previously held `admin` continue to invoke
  `recovery.*` routes for one release per §2.1.

## 12. What Cannot Be Claimed After This Lands

To prevent overclaim in `docs/SPEC.md`, `CHANGELOG.md`, or release
notes:

1. **Not** cross-machine, multi-tenant, or hosted. Cross-repo runs span
   locally-registered repos only; the daemon refuses non-loopback HTTP.
2. **Not** resistant to malicious-local-root. The signing key, daemon
   DB, and tokens are operator-readable by design (RFC 0031 §Threat
   Model). Sealed apply continues to be an AI-guardrail, not
   cryptographic non-repudiation.
3. **Not** atomic across two repos' working trees. The daemon
   coordinates run state with best-effort consistency on crash. Two
   repos' file-system mutations are still the workflow author's
   responsibility.
4. **Not** validated by end-to-end multi-repo harness tests. Coverage
   is unit-level and mock-based until the follow-up RFC lands a real
   multi-repo daemon test harness.
5. **Not** a defense against an operator who issues a long-lived
   `admin` token to a prompt-injectable MCP client. That is operator
   misconfiguration; we warn but do not refuse.

## 13. Staging Plan

Recommended landing order inside the dogfood-035 build slice:

1. **Capability + default-deny hardening.** Add `recovery` to in-code
   vocabulary; add the startup assertion; add per-token cache scaffold;
   wire the denial-vocabulary additions. Tests in §9.3 and §9.4. Lands
   independently of cross-repo.
2. **MCP `tools/list`/`tools/call` filtering and audit.** Extend
   `DaemonRpcServer` for capability-filtered tools, normalized params
   hashing, and audit-on-every-call. Tests in §9.4.
3. **Workflow schema additions.** `repositories` block + per-job
   `repository` + cross-repo edge/cycle/parallelism rules. Tests in
   §9.1.
4. **Daemon-DB schema migration 0003.** Cross-repo tables. Repo-local
   migration v14. Tests in §9.2.
5. **Cross-repo lifecycle handlers.** `cross_repo.prepare/start/
   summary/cancel/describe/why/list` + startup reconciler. Tests in
   §9.2 and §9.5.
6. **CLI surface.** `cross-repo` verb family + auto-routing in
   `run prepare/start/cancel`.
7. **Documentation updates.** SPEC, MCP, UBIQUITOUS_LANGUAGE,
   CLI_REFERENCE, HOW_TO_HUMAN, RFC 0028 status, CHANGELOG. Mark RFC
   0032 status accepted/implemented after build review.

Each step's tests can land alongside the code; the synthesis pass may
recommend additional sequencing if a reviewer pushes back.

## 14. Open Questions Carried Into Synthesis

Items the synthesis pass must resolve, with this design's
recommendations:

1. **`cross_repo` `tools/list` filter posture: conservative vs. permissive.**
   Recommendation: conservative — include a cross-repo tool only when the
   token has the capability for **every** registered repo. Reviewers may
   want "any registered repo," accepting that `tools/call` will deny per
   participant. Cost of conservative: operator must rebuild their effective
   tool list after granting a new repo capability. Cost of permissive:
   `tools/list` advertises tools that `tools/call` will refuse, surfacing
   denials as the first sign of misconfiguration.
2. **`cross_repo_cycle` flag default.** RFC 0032 §10 recommends explicit
   opt-in. We agree; cross-repo cycles are subtle enough that authors
   should declare intent. No default.
3. **Cross-repo run id namespace.** Recommendation per RFC 0032 §10: use
   `cross_repo_run_<base32>` distinct from `run_<base32>`. Prevents prefix
   confusion in `striatum status <id>` / `dashboard --run-id <id>` — the
   CLI client introspects the prefix and routes accordingly.
4. **Per-token effective-tool-set cache TTL.** Recommendation: invalidate
   on `daemon.token.revoke` and any `client_capabilities` mutation. No TTL
   (event-based invalidation only). A capability_revision counter on the
   client row makes invalidation cheap.
5. **`cross_repo_orphaned` reconciler resolution UX.** Recommendation: a
   `doctor` check that lists orphaned local runs; an explicit CLI verb
   `striatum cross-repo orphan-resolve --run-id <local_run_id>
   --decision delete|keep_as_aborted` so the operator's call is
   recorded.
6. **Should `recovery` move out of `admin` cleanly in this release, or
   stay aliased through one minor?** Recommendation per §2.1: alias for
   one release. Build reviewers should challenge this if it preserves
   too much blast radius.

These belong to the synthesis pass to either accept, reject, or amend
in light of the codex and gemini designs.
