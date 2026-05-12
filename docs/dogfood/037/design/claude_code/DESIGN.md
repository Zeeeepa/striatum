author: designer-claude-opus-001

# RFC 0035 Multi-Repo Test Harness: Implementation Design

Status: design
Date: 2026-05-12
Target: RFC 0035 multi-repo test harness for cross-repo workflows
RFC inputs: RFC 0030 (daemon RPC), RFC 0031 (daemon-owned supervision + sealed apply), RFC 0032 (cross-repo + MCP mutation), RFC 0033 (storage substrate), RFC 0035 (this harness)
Source-state inputs:
- `src/striatum/cross_repo.py` (mock-friendly prepare/start/cancel/reconcile)
- `src/striatum/daemon_rpc/registry.py` (capability-bound method registry incl. `repository_scope_mode`)
- `src/striatum/daemon_rpc/capability.py` (`authorize()` returning `RpcAuthContext`)
- `src/striatum/daemon_rpc/request_log.py` (audit-row + request-log helpers with hash chain)
- `src/striatum/daemon_pg/sql/0003_cross_repo_mcp.sql` (cross-repo tables + `audit_repositories`)
- `src/striatum/mcp.py` `DaemonRpcServer` (`tools/list` filter + `tools/call` re-authorization)
- `tests/test_cross_repo_lifecycle.py`, `tests/test_mcp_mutation_capabilities.py`

## 1. Framing

Dogfood-035 shipped contract-level coverage: workflow validator rejects bad
shapes, the mock-friendly cross-repo lifecycle helper exercises
prepare/start/cancel/reconcile state transitions, the daemon MCP server
filters `tools/list` and re-authorizes `tools/call`, and the method
registry binds capabilities + repository scope modes. None of that hit a
real two-repo daemon. The threats RFC 0032 names — capability scope
mismatch on a live MCP path, audit chain continuity across mixed
allow/deny calls, daemon crash mid-prepare or mid-start across the
daemon-DB ↔ repo-local-SQLite boundary, unreachable participant
mid-run — are still hypothetical until the harness lets a real daemon
process write into real participant `.striatum/state.sqlite3` files
under a real Postgres database.

The harness in this design is the integration-test boundary, not a new
production code path. Every helper in `tests/_harness/` wraps an existing
production code path: token issuance through the same `daemon.token.create`
admin RPC, MCP calls through the same `DaemonRpcServer` instance, audit
row inspection by direct read of `striatumd.audit_log`. The harness must
not invent denial vocabulary, must not reimplement capability checks, and
must not bypass `authorize()` even in setup paths — if setup wants
admin-only writes, it issues an admin token through the documented
admin path.

The deliverable is the test infrastructure that lets seven trust-boundary
e2e tests (§5–§11) assert against a live daemon. Everything else in
this design exists to make those tests cheap to write, fast to run, and
deterministic on a daemon SIGKILL.

## 2. Module layout

```
tests/
  _harness/
    __init__.py             # public surface: MultiRepoHarness, fixtures
    multi_repo.py           # MultiRepoHarness orchestration
    daemon.py               # ephemeral daemon process + SIGKILL/restart helpers
    repos.py                # per-repo init + register + intentional-damage helpers
    pg.py                   # ephemeral Postgres DB + per-test TRUNCATE reset
    mcp.py                  # MCP client helpers (capability-filtered tools/list, tools/call)
    tokens.py               # capability token issuance / revoke / expire test helpers
    audit.py                # audit-row inspection + chain-continuity verifier
    scope.py                # forbidden-path / write-scope assertion helpers
  test_multi_repo_harness.py
  test_cross_repo_prepare_e2e.py
  test_cross_repo_lifecycle_e2e.py
  test_cross_repo_crash_recovery_e2e.py
  test_mcp_capability_scope_e2e.py
  test_mcp_audit_chain_e2e.py
  test_per_repo_write_scope_e2e.py
  test_adversarial_e2e.py
```

The harness adds no new production code under `src/striatum/`. The
`mcp.py`, `tokens.py`, `audit.py`, `scope.py` modules under
`tests/_harness/` are thin wrappers; if a test needs new behavior it goes
upstream into production code with its own RFC, not into the harness.

## 3. `MultiRepoHarness` lifecycle

`MultiRepoHarness.start()`:

1. Allocate an ephemeral Postgres database name
   `striatum_test_<uuid7-prefix>` against the system PG connection from
   `postgres_url` (CI provides; local dev runs `make pg-test`).
2. Run `striatum.daemon_pg.migrations.apply_migrations()` to bring the DB
   to `LATEST_DAEMON_DB_VERSION` (currently 3). The harness MUST call the
   production migration code, not a fixture copy, so a missing migration
   surfaces as an immediate harness failure.
3. Initialize `repo_count` target repositories under
   `scratch_dir/repo-{0..N-1}/`. Each gets `.striatum/state.sqlite3` with
   repo-local migrations applied via the production migration entry
   point.
4. Boot the daemon as a subprocess: `striatum daemon start
   --socket scratch_dir/daemon.sock --pg-url postgresql://.../<db>`.
   Stdout/stderr → DEVNULL per D028. Capture the daemon PID for §7
   crash tests. Wait for `daemon.hello` to succeed (60s timeout, 100ms
   poll) before yielding.
5. Register each repository via the daemon admin path
   (`striatum repo add --init`, which routes through `repo.add` RPC).
   Bootstrap an admin token for harness setup the same way the daemon
   bootstraps one on first start — re-using the documented bootstrap
   path, not a back-door write.
6. Yield. The fixture stores: `daemon_pid`, `socket_path`,
   `pg_dsn`, `repo_roots: list[Path]`, `repo_ids: list[str]`,
   `admin_token: str`.

`MultiRepoHarness.stop()`:

1. SIGTERM the daemon, wait up to 5s, SIGKILL if it hasn't exited.
2. Drop the ephemeral PG database (`DROP DATABASE ... WITH (FORCE)`).
3. Remove `scratch_dir` (the socket file inside it disappears with it).
4. Assert the socket no longer exists (smoke-test invariant).

Default fixture scope is **class** — daemon boot is the slow part and
amortizing it across tests in the same `Test*` class is the cost-of-CI
tradeoff RFC 0035 §3 already accepted. Tests that need per-function
isolation use the `clean_daemon_db` fixture (§4) to TRUNCATE between
calls without paying for daemon restart.

## 4. Per-test residue: what gets cleared and what survives

The harness's correctness rests entirely on residue discipline. If a
test leaves an audit row, a capability token, or a `cross_repo_runs` row
in the DB, the next test's chain-continuity assertion silently passes on
stale state and the harness becomes worse than no test at all.

### 4.1 Daemon Postgres reset (per-class daemon, per-function DB)

`MultiRepoHarness.reset_daemon_db()` issues:

```sql
TRUNCATE TABLE
  striatumd.audit_repositories,
  striatumd.audit_log,
  striatumd.audit_segments,
  striatumd.audit_chain_head,
  striatumd.cross_repo_cycle_counters,
  striatumd.cross_repo_run_repositories,
  striatumd.cross_repo_runs,
  striatumd.rpc_request_log,
  striatumd.client_capabilities,
  striatumd.clients,
  striatumd.repositories
RESTART IDENTITY CASCADE;
```

The schema-version row in `striatumd.schema_meta` and the migration
ledger in `striatumd.schema_migrations` are **not** truncated — those
encode the migration state of the live daemon process, and clearing
them would force the next `apply_migrations()` call to re-run every
migration. (The synthesis already chose TRUNCATE over DROP+CREATE to
amortize the schema-version metadata cost; this design just names the
exclusion explicitly.) Likewise `striatumd.rpc_methods` is not
truncated; method registry rows are seeded by migration and the daemon
expects them present.

`reset_daemon_db()` does NOT re-register participating repos. Tests that
need fresh registration call `harness.register_all()` (RFC 0035 §3 open
question — explicit re-register is the chosen behavior). This is the
"honest" choice: a test that forgets to register and then expects
`run.prepare` to succeed gets a clear `repo_not_registered` denial
rather than silently inheriting setup from a sibling test.

After TRUNCATE, the harness re-seeds the audit chain head with a single
row carrying `last_hash = NULL` (matching the daemon's fresh-install
state), so the first `append_audit_row` of the test computes
`previous_hash = NULL` cleanly. Without this re-seed,
`striatumd.audit_chain_head` is empty and the chain helper would treat
the first audit row as a chain restart rather than a chain start.

### 4.2 Per-repo SQLite reset

For each participating repo, `harness.reset_repo_local(repo_idx)`
truncates the rows in repo-local tables while keeping the schema-version
row:

```sql
DELETE FROM events;
DELETE FROM leases;
DELETE FROM queue_messages;
DELETE FROM artifacts;
DELETE FROM jobs;
DELETE FROM sessions;
DELETE FROM runs;
DELETE FROM process_supervisor_pointers;
-- migration_state, schema_version stay
```

Repo-local tables don't have audit chain semantics, so DELETE is
sufficient; we don't need RESTART IDENTITY because repo-local IDs are
content-addressed (`run_<...>`, `lease_<...>`).

`reset_repo_local()` also clears scratch artifacts under the repo root
that were published during the test. Tests that publish into a known
test-artifact path (e.g. `docs/dogfood/037/test_artifacts/...`) get
filesystem cleanup before the next test claims the same path.

### 4.3 Token cache

The harness keeps a per-class registry of tokens it issued so
`reset_daemon_db()` can revoke them client-side too (issuing a fresh
token after TRUNCATE invalidates the cached one anyway, but explicit
revocation in the test client surfaces accidental reuse as a `401`
rather than a silent capability_missing).

### 4.4 Socket + scratch teardown

On `stop()`, the harness asserts:

- `socket_path` no longer exists,
- `scratch_dir` no longer exists,
- the ephemeral PG database no longer appears in `pg_database`.

The smoke test (§12) treats these three invariants as table stakes.

### 4.5 Two-back-to-back invariant

`tests/test_multi_repo_harness.py` runs `MultiRepoHarness.start()` ->
`stop()` -> `start()` -> `stop()` in one process and asserts the second
start succeeds. Catches port-style collisions, lingering Unix sockets,
and PG `DROP DATABASE` waits that didn't finish.

## 5. Trust boundary: capability scope mismatch (token for repo A used against repo B)

`tests/test_mcp_capability_scope_e2e.py::test_repo_a_token_refused_against_repo_b`:

```text
harness = MultiRepoHarness(repo_count=2)
token_a = harness.issue_token(["write"], repo_id=harness.repo_ids[0])
# Use the same MCP client surface a real chat client would use.
client = harness.mcp_client(token=token_a)
result = client.call_tool(
    name="publish_artifact",
    arguments={
        "repository_id": harness.repo_ids[1],  # repo B
        "session_id": "sess_test",
        "job_id": "job_test",
        "lease_id": "lease_test",
        "kind": "handoff",
        "logical_name": "x",
        "path": "x.md",
    },
)
assert result["isError"] is True
assert result["structuredContent"]["error"] == "capability_scope_mismatch"
```

The denial vocabulary is exactly what `capability.authorize()` returns
when a token is scoped to repo A but `repository_id` is repo B and no
unscoped grant exists (`capability.py:90`). The test asserts both:

1. `isError: True` and `error: "capability_scope_mismatch"` in the MCP
   response (the client-visible refusal),
2. An audit row exists with `decision = 'denied'`,
   `denial_reason = 'capability_scope_mismatch'`,
   `method = 'publish_artifact'`, `repository_id` = repo B, `transport
   = 'mcp'`.

The audit-row assertion is the second half of the trust boundary: a
refusal without an audit row is a silent skip, which is the worst
possible failure mode. The harness's `audit.assert_row_matches()` helper
queries `striatumd.audit_log` directly by `request_id` and asserts the
documented denial-vocabulary string verbatim.

A symmetric case — `token_b` against repo A — runs in the same test
file to defeat the "happened to hardcode repo_a" failure mode.

The unscoped variant — token scoped to repo A invoking a
`cross_repo`-scope-mode method — is covered separately by the cross-repo
capability test in §10.

## 6. Trust boundary: `tools/list` capability filtering correctness

`tests/test_mcp_capability_scope_e2e.py::test_tools_list_filters_to_capability_set`:

```text
read_only = harness.issue_token(["read"], repo_id=harness.repo_ids[0])
writer    = harness.issue_token(["read", "write"], repo_id=harness.repo_ids[0])

client_ro = harness.mcp_client(token=read_only)
client_w  = harness.mcp_client(token=writer)

tools_ro = client_ro.list_tools(repository_id=harness.repo_ids[0])
tools_w  = client_w.list_tools(repository_id=harness.repo_ids[0])

ro_names = {t["name"] for t in tools_ro}
w_names  = {t["name"] for t in tools_w}

assert "status" in ro_names
assert "publish_artifact" not in ro_names    # write-only tool hidden
assert "apply.reviewed_patch" not in ro_names # apply-only tool hidden
assert "publish_artifact" in w_names          # writer sees the write tool
assert "apply.reviewed_patch" not in w_names  # writer still lacks apply
```

This catches the failure mode in which `tools/list` is implemented as
"return all method registry entries" instead of "intersect registry with
capability/scope" — the bug class that lets a prompt-injected client
discover tools its token can't actually invoke. (`tools/list` filtering
is the discoverability boundary; §5 is the authorization boundary —
both need their own tests because either alone can ship broken.)

The cross-repo-scope variant: a token scoped to repo A asks `tools/list`
with `repository_id=repo_A`. The synthesis's stricter policy is "hide
cross-repo tools until the token has daemon-global or all-participant
coverage." This test asserts `cross_repo.list` and `cross_repo.describe`
are absent from `tools_ro` and `tools_w` because both tokens are
repo-scoped, not daemon-global.

## 7. Trust boundary: default-deny on unknown methods

`tests/test_mcp_capability_scope_e2e.py::test_unknown_method_default_denies_with_audit`:

```text
writer = harness.issue_token(["read", "write"], repo_id=harness.repo_ids[0])
client = harness.mcp_client(token=writer)
result = client.call_tool(
    name="unknown.method",
    arguments={"repository_id": harness.repo_ids[0]},
)

assert result["isError"] is True
assert result["structuredContent"]["error"] == "method_unknown"

audit = harness.audit.last_row()
assert audit["method"] == "unknown.method"
assert audit["decision"] == "denied"
assert audit["denial_reason"] == "method_unknown"
assert audit["exit_code"] == 10
```

Today `DaemonRpcServer.call_daemon_tool` (`mcp.py:572`) constructs an
`RpcAuthContext` with `denial_reason="method_unknown"` and appends both
an audit row and a request-log row before returning the error to the
client. The test pins that behavior end-to-end through a real daemon
process and a real Postgres write, not the unit-mock pair currently in
`tests/test_mcp_mutation_capabilities.py`. The integration matters
because the audit chain hash depends on row ordering, and the unit test
doesn't exercise the chain-head update.

## 8. Trust boundary: audit chain integrity across allow/deny

`tests/test_mcp_audit_chain_e2e.py::test_audit_chain_continuity_across_mixed_allow_deny`:

```text
writer = harness.issue_token(["read", "write"], repo_id=harness.repo_ids[0])
reader = harness.issue_token(["read"], repo_id=harness.repo_ids[0])
client_w = harness.mcp_client(token=writer)
client_r = harness.mcp_client(token=reader)

# Mixed sequence: allow, allow, deny, allow, deny, deny.
client_w.call_tool(name="status",            arguments={"repository_id": repo_a})
client_w.call_tool(name="publish_artifact",  arguments=publish_args(repo_a))
client_r.call_tool(name="publish_artifact",  arguments=publish_args(repo_a))   # denied: capability_missing
client_w.call_tool(name="why",               arguments={"repository_id": repo_a, "id": "x"})
client_w.call_tool(name="unknown.method",    arguments={"repository_id": repo_a})  # denied: method_unknown
client_w.call_tool(name="publish_artifact",  arguments=publish_args(repo_b))      # denied: capability_scope_mismatch

assert harness.audit.chain_is_continuous()
decisions = [row["decision"] for row in harness.audit.all_rows()]
assert decisions == ["allowed", "allowed", "denied", "allowed", "denied", "denied"]
```

`audit.chain_is_continuous()` walks `striatumd.audit_log` in `audit_id`
order, recomputes `row_hash` via the production `v2_row_hash` helper
(`daemon_pg/audit.py`), and asserts every row's `previous_hash` equals
the prior row's `row_hash`. First row's `previous_hash` must equal the
re-seeded chain head's `last_hash` (NULL after `reset_daemon_db()`).
Tampered or missing rows fail loudly.

The same helper underpins the audit-tamper adversarial test (§11.4):
the daemon's audit-append path is append-only by design, but a test
that asserts "the chain is continuous after a mixed allow/deny burst"
catches drift before adversarial inputs do.

## 9. Trust boundary: crash recovery

The hardest tier of the harness. `tests/_harness/daemon.py` exposes:

```python
def sigkill_daemon(self) -> None: ...
def restart_daemon(self) -> None: ...   # boots a new daemon process against the same pg_dsn + sockets
def install_prepare_hook(self, *, stage: str) -> None: ...
```

`install_prepare_hook` lets a test pause the daemon's cross-repo
prepare path at a named checkpoint via an env-var-controlled debug
breakpoint compiled into the daemon (`STRIATUM_TEST_PAUSE_AT=prepare_after_daemon_db_write`).
The pause point uses a Unix FIFO under `scratch_dir/pause-<stage>.fifo`;
the daemon blocks on `read()` until the test writes a byte. The
production code path is unchanged; the pause point only activates when
the env var is set. (This is the only acceptable form of "test-only
seam in production code" — and only because the alternative is shipping
a parallel cross-repo prepare path under `tests/`, which the prompt
explicitly forbids.)

### 9.1 Crash mid-prepare

```text
harness.install_prepare_hook(stage="after_daemon_db_write_before_repo_local")
fut = harness.async_prepare_cross_repo_run(workflow=two_repo_workflow)
harness.wait_for_pause(stage="after_daemon_db_write_before_repo_local")
# Daemon has written cross_repo_runs(state='preparing') and the per-repo
# cross_repo_run_repositories rows, but no per-repo `runs` row yet.
harness.sigkill_daemon()

# Restart with a fresh daemon process against the same Postgres + sockets.
harness.restart_daemon()

# Daemon startup runs reconcile_cross_repo_preparing().
# Per cross_repo.py:227, the reconciler observes preparing rows whose
# participants' local_run_id is NULL and marks the row 'aborted'.
runs = harness.cross_repo.list_runs()
assert len(runs) == 1
assert runs[0]["state"] == "aborted"
assert runs[0]["last_reconcile_error"] == "participant local run missing during reconcile"

# No orphan local runs in either repo.
assert harness.repo_local(0).runs() == []
assert harness.repo_local(1).runs() == []
```

The test fixes the reconciliation state machine, not just "the daemon
restarted." If `cross_repo.py` ever drifts from
"missing local_run_id → aborted," the test fails before the runtime
discovers it.

### 9.2 Crash mid-start

```text
harness.install_prepare_hook(stage="after_first_repo_start_before_second")
prepared = harness.prepare_cross_repo_run(workflow=two_repo_workflow)
fut = harness.async_start_cross_repo_run(cross_repo_run_id=prepared["cross_repo_run_id"])
harness.wait_for_pause(stage="after_first_repo_start_before_second")
harness.sigkill_daemon()
harness.restart_daemon()

# Daemon startup reconciles. Per cross_repo.py:131, start refuses anything
# not in 'prepared'; reconcile from the crashed 'running' state moves to
# either fully-running (if all participants are intact) or 'blocked' with a
# structured error.
state = harness.cross_repo.describe(prepared["cross_repo_run_id"])
assert state["state"] in {"running", "blocked"}
if state["state"] == "blocked":
    assert state["last_reconcile_error"] is not None
    # Primary repo carries a human checkpoint.
    assert harness.repo_local(0).human_checkpoints() != []
```

`cross_repo.py` does not yet have a reconciler for `running`-state
crashes; the e2e test surfacing this gap is part of why the harness
exists. The test asserts the documented contract; the implementation
RFC follow-up wires the reconciler if the gap proves real.

### 9.3 Crash mid-cancel

Same shape as §9.1 with a pause hook in the cancel path. Asserts the
final state converges to either `canceled` or `blocked` with the
documented `cross_repo.py:182` cascade.

### 9.4 One participant unreachable mid-run

```text
prepared = harness.prepare_cross_repo_run(workflow=two_repo_workflow)
harness.start_cross_repo_run(cross_repo_run_id=prepared["cross_repo_run_id"])

# Simulate filesystem unreachability by chmod 000 on the participant's .striatum.
harness.simulate_repo_unreachable(repo_idx=1)
try:
    # Trigger any daemon path that touches repo B's state — e.g. a heartbeat or claim.
    harness.cross_repo.heartbeat_all(cross_repo_run_id=prepared["cross_repo_run_id"])

    state = harness.cross_repo.describe(prepared["cross_repo_run_id"])
    assert state["state"] in {"blocked", "running"}
    # Primary repo recorded the checkpoint.
    checkpoints = harness.repo_local(0).human_checkpoints()
    assert any("repo unreachable" in c["reason"] for c in checkpoints)
    # Daemon DB recorded the participant's degraded state.
    assert harness.cross_repo.participant_state(prepared["cross_repo_run_id"], repo_idx=1) in {
        "unavailable", "blocked"
    }
finally:
    harness.simulate_repo_reachable(repo_idx=1)
```

The `unavailable` state was added to the
`cross_repo_run_repositories.state` CHECK constraint in
`0003_cross_repo_mcp.sql:42` precisely for this case; the test exercises
the transition end-to-end. The chmod-000 simulation is the harness's
contract for "filesystem can't be reached"; on POSIX it produces real
`PermissionError` from the daemon's repo-local SQLite open, which is the
production failure shape we want surfaced.

The `finally` clause is mandatory: leaving a chmod-000 directory in
`scratch_dir` would cause harness teardown to fail with a confusing
permission error. The helper sets a `chmod_restore` hook on the
harness so teardown undoes the damage even if the test body crashes.

## 10. Trust boundary: per-repo write-scope enforcement

`tests/test_per_repo_write_scope_e2e.py`:

### 10.1 Validator-time refusal

A cross-repo workflow declares job `J` with
`expected_artifacts[].path = "../repo-0/x.md"` while `J`'s
`repository` is `repo-1`. The validator (already in
`src/striatum/workflow.py`) refuses at `workflow.validate` time with
`write_scope_violation`. The test asserts the validator's refusal
message verbatim and that no daemon DB writes happened.

### 10.2 Runtime refusal

Same job, but the workflow validator is bypassed by submitting a
maliciously constructed workflow snapshot directly to
`run.prepare` (the harness exposes `harness.submit_raw_workflow()` so
the test can exercise the runtime-time path). The daemon prepares the
run (the validator is what catches paths in V1), but the agent's
attempt to publish the artifact through `publish_artifact` against
repo-1 with a path resolving into repo-0's worktree refuses with
`write_scope_violation`. The audit row records the denial.

The test asserts both refusal sites — validator and runtime — so that a
future refactor that moves the check between layers doesn't leave a
gap. (RFC 0035 §8 names this explicitly.)

### 10.3 Cross-repo path resolution

The path-resolution helper (`tests/_harness/scope.py::resolve_artifact_path`)
mirrors the production resolver: every `expected_artifacts[].path` is
joined against the job's target repo root, then `realpath`'d, and the
result must be a subpath of that repo root. Symlinks pointing outside
the repo are caught by the `realpath` step; the harness publishes a
test that creates `repo-1/symlink-to-repo-0.md → ../repo-0/x.md` and
asserts `write_scope_violation`.

## 11. Adversarial tests

### 11.1 Hostile `tools/list` with elevated args

A prompt-injected client passes extra `arguments` keys to `tools/list`
trying to escape the capability filter (`scope="daemon"`,
`override_capability=true`, etc.). The harness's
`mcp_client.list_tools()` passes them through; the daemon ignores
unknown fields and returns the same capability-filtered set as a clean
client. Test asserts the returned tool set equals the clean-client
set. (The defense is "extra fields ignored," not "extra fields
rejected" — extra-field tolerance is the documented MCP behavior; the
test pins that no extra field upgrades capability.)

### 11.2 Header-claimed "trusted" identity

A malicious MCP client sends an `X-Striatum-Trusted-Client: yes`
pseudo-header along with the JSON-RPC body. Same shape as 11.1: the
daemon ignores it, authorization is decided entirely by
`capability_token`. Test asserts the call is refused with
`capability_missing` (or whatever the token's real capability would
return), not `allowed`.

### 11.3 Replay of expired token after rotation

```text
old = harness.issue_token(["write"], repo_id=repo_a, expires_in=60)
harness.expire_token(old)                  # advance daemon clock or update expires_at directly
new = harness.issue_token(["write"], repo_id=repo_a)

client_old = harness.mcp_client(token=old)
client_new = harness.mcp_client(token=new)

# Both clients hit publish_artifact against repo_a.
old_result = client_old.call_tool(name="publish_artifact", arguments=publish_args(repo_a))
new_result = client_new.call_tool(name="publish_artifact", arguments=publish_args(repo_a))

assert old_result["structuredContent"]["error"] == "token_expired"
assert new_result["isError"] is False
```

`harness.expire_token()` is a test-only helper documented as such. It
takes either a clock-advance path (`UPDATE striatumd.clients SET
expires_at = now() - interval '1 hour' WHERE token_id = %s`) or a
client-capabilities expiry path. Both shapes exist because RFC 0032 has
both token expiry and per-capability expiry; the test exercises each.

### 11.4 Cross-repo token leak

Identical shape to §5 but framed adversarially: a token scoped to repo A
deliberately attempts to read repo B's run summary. Refused with
`capability_scope_mismatch`; audit row records the scope mismatch and
the read attempt. The point is that the leak is *audited*, not just
refused — operators investigating a breach need the row.

### 11.5 Operator-confirmation gate bypass

V2 doesn't yet ship operator-confirmation on every write. The harness
simulates the bypass attempt by submitting a `tools/call` with a
documented `requires_confirmation` field omitted on a method that
requires it (the harness's per-method confirmation registry is a small
table seeded in `tests/_harness/mcp.py`). The daemon refuses with
`operator_confirmation_missing`. If the method registry doesn't yet
emit that denial reason, the test xfails with a documented `# FIXME:
RFC 0032 operator confirmation` marker so the gap is visible in CI
output, not silently absent.

### 11.6 Audit chain tamper via daemon API

Try every documented daemon API surface for "append a row, but with a
crafted `previous_hash`." The `append_audit_row` helper in
`daemon_rpc/request_log.py:78` computes `previous_hash` from
`striatumd.audit_chain_head` and never accepts a client-supplied
value, so this test is effectively asserting that the helper has no
parameter through which a client could inject `previous_hash`. The
test calls every mutating MCP method with a `params.previous_hash`
field present; asserts no row in `audit_log` has that value as its
`previous_hash`. (Belt-and-suspenders: protects against a future
refactor that adds a `previous_hash` parameter.)

## 12. Harness smoke test

`tests/test_multi_repo_harness.py` covers RFC 0035 §9 plus the residue
invariants from §4.5 above:

- `start()` → register 2 repos → `stop()`; both repos register, daemon
  exits cleanly, socket and scratch dir are gone, ephemeral DB
  dropped;
- `reset_daemon_db()` truncates every table named in §4.1, preserves
  `schema_meta` / `schema_migrations` / `rpc_methods`, and re-seeds
  `audit_chain_head`;
- `start()` → `stop()` → `start()` → `stop()` in one process works;
- `reset_repo_local(0)` clears repo-local rows but preserves
  `schema_version`;
- `harness.audit.chain_is_continuous()` over an empty post-reset DB
  returns True (vacuous truth — important so empty-chain tests aren't
  false-positive).

## 13. Capability token lifecycle helpers

The harness exposes three helpers under `tests/_harness/tokens.py`. Each
wraps a production code path; the wrapper exists only to make the test
body readable.

```python
def issue_token(
    self,
    capabilities: list[str],
    *,
    repo_id: str | None = None,
    expires_in: int = 3600,
) -> str:
    """Issue a capability token via daemon.token.create RPC. Returns the
    full <token_id>.<secret> string the MCP client uses."""

def revoke_token(self, token: str) -> None:
    """Invoke daemon.token.revoke RPC with the harness's admin token."""

def expire_token(self, token: str) -> None:
    """TEST-ONLY: shift the daemon's clock for this token by direct
    UPDATE on striatumd.clients.expires_at. Not a production helper —
    operators expire tokens by revoking + re-issuing, not by clock skew."""
```

`issue_token` and `revoke_token` are the documented operator paths.
`expire_token` is a deliberate test-only helper because expiring a real
token via clock advance requires either waiting for real time or shifting
the daemon's view of `now()`. Documenting it as test-only is the
discipline against "operator copy-pasted the harness helper into a real
operator workflow."

## 14. MCP client helper shape

`tests/_harness/mcp.py::McpClient`:

```python
class McpClient:
    def __init__(self, *, harness: MultiRepoHarness, token: str) -> None: ...
    def list_tools(self, *, repository_id: str | None = None) -> list[dict]: ...
    def call_tool(self, *, name: str, arguments: dict) -> dict: ...
```

`McpClient` constructs a `DaemonRpcServer(pg_conn=harness.pg_conn)`
inside the test process and dispatches `tools/list` / `tools/call`
through the production class (`src/striatum/mcp.py:461`). This is the
only deviation from "talk to the daemon process over the socket": MCP
tests don't need to go through the Unix socket because the
authorization boundary (`capability.authorize()` + `append_audit_row`)
is the same Python code whether dispatched in-process or via socket.
Socket-transport correctness is covered by the daemon-process tests in
§5–§10, which exercise the socket path through `striatum status`
and `striatum publish-artifact` invocations against the harness's
socket.

This split keeps the MCP capability-scope tests fast while letting the
crash-recovery tests exercise the real daemon process. The harness
documentation calls out the seam explicitly so a future refactor that
diverges in-process and over-socket behavior surfaces as a test gap.

## 15. State the harness does NOT introduce a parallel production-code path

To be explicit, because this is the largest review surface for the
harness:

- **Same daemon binary**: the harness boots `striatum daemon start`
  with arguments. No alternate entry point.
- **Same SQLite + Postgres migrations**: `apply_migrations()` from
  `striatum.daemon_pg.migrations` and the repo-local migrations from
  `striatum.migrations`. No fixture-copy schema.
- **Same RPC envelope** (RFC 0030): the harness's CLI invocations
  serialize via the production envelope code; in-process MCP tests use
  `DaemonRpcServer` directly.
- **Same capability vocabulary**: `striatum.daemon_rpc.registry.CAPABILITIES`
  is the source of truth. The harness never asserts against a
  hardcoded "read/write/review/claim/apply/admin/recovery" list — it
  imports the frozenset.
- **Same audit chain helper** (RFC 0032 V2):
  `striatum.daemon_pg.audit.v2_row_hash` is what
  `audit.chain_is_continuous()` re-uses to recompute hashes. The
  harness never reimplements the hash; if the production hash changes,
  the harness's verifier picks it up automatically.
- **Same MCP `tools/call` + `tools/list` code path**: the
  in-process `DaemonRpcServer` is the production class.

The only test-only seams added are:

1. `STRIATUM_TEST_PAUSE_AT=<stage>` env var, plumbed through the cross-
   repo prepare/start/cancel paths in `cross_repo.py`. The env var
   defaults to unset; production daemons never read it. Each stage
   name is documented in `cross_repo.py` next to the pause point. The
   pause is implemented as a no-op `_test_pause(stage)` helper that
   blocks on a FIFO read when the env var matches; in production the
   helper is a single `return` after an `if not _PAUSE_ENABLED`.
2. `harness.expire_token()` (§13).
3. The chmod-000 simulator in `tests/_harness/repos.py`; nothing the
   daemon reads.

## 16. What cannot be claimed even after the harness lands

- **Cross-machine multi-tenant testing** is out of scope per D083
  (single-user single-machine); the harness binds a Unix socket on
  one host and the daemon refuses non-loopback transports at startup.
  No follow-up planned.
- **Windows daemon testing** is out of scope per RFC 0030 V2; the
  harness runs Linux + macOS only, and CI skips on Windows with a
  documented message.
- **Malicious-local-root resistance** is out of scope per RFC 0031 §
  Threat Model. An operator with code execution as the daemon user
  can read the signing key, edit the audit chain via `psql`, or
  impersonate the daemon — none of those are what the harness
  defends against. The adversarial tests in §11 defend against
  "over-eager AI agent acting through the documented interfaces,"
  which is the framing RFC 0031 names.
- **Performance / load testing** is a separate effort. The harness's
  wall-clock budget is < 60s on a developer laptop per RFC 0035 §10.
  Benchmark-shaped tests live outside the harness.
- **Direct-mode retirement validation**: this harness assumes
  daemon-mediated mode. The `--no-daemon` direct path is still
  exercised by the existing single-repo tests; the harness does not
  cover it because cross-repo refuses direct mode by design (RFC 0032).

## 17. Implementation sequence

1. **Harness skeleton + smoke test** (RFC 0035 Step 1): land
   `tests/_harness/` modules with `MultiRepoHarness.start/stop`,
   `reset_daemon_db()`, `register_all()`, `reset_repo_local()`, and
   `test_multi_repo_harness.py`. No e2e tests yet.
2. **Test-only pause hooks**: land the `STRIATUM_TEST_PAUSE_AT`
   plumbing in `cross_repo.py` with documented pause points. Pure
   no-op in production. Smoke-test that pause/release works.
3. **Prepare + lifecycle e2e** (RFC 0035 Step 2): land §5, §6, §7
   tests against a real daemon. Surface any helper gaps and fold them
   back into `_harness/`.
4. **Crash recovery e2e** (RFC 0035 Step 3): §9.1, §9.2, §9.3, §9.4.
   This is where the §9.2 reconciler gap, if any, surfaces — the
   test makes the gap concrete.
5. **MCP capability scope e2e + audit chain** (RFC 0035 Step 4):
   §5, §6, §7, §8.
6. **Per-repo write-scope e2e** (RFC 0035 Step 5): §10.
7. **Adversarial e2e** (RFC 0035 Step 6'): §11. Some cases (§11.5)
   may xfail until follow-up RFCs land; document the xfail with the
   RFC reference.
8. **CI wiring + docs**: `make test-multi-repo`, `tests/conftest.py`
   fixture wiring, `docs/TODO.md` Open item 19 → most-done,
   `CHANGELOG.md`.

## 18. Open questions for the synthesis

- Should the `STRIATUM_TEST_PAUSE_AT` hook live in `cross_repo.py`
  inline or in a separate `striatum._testpause` module that the
  cross-repo code imports? Inline keeps the pause points adjacent to
  the production logic they pause; a separate module keeps `cross_repo.py`
  free of test-only references. Recommendation: inline with a
  one-line import from `striatum._testpause`, so the pause-point name
  is visible in `cross_repo.py` but the implementation lives in the
  test-only module.
- The §11.5 operator-confirmation gate test xfails today because RFC
  0032's confirmation field is documented but not yet enforced. Is
  the xfail acceptable, or should §11.5 be deferred until a
  follow-up RFC enforces confirmation? Recommendation: ship the
  xfail with a documented marker so the gap stays visible.
- Should `reset_daemon_db()` accept a `seed_clients: list[str]`
  parameter that re-issues a named set of tokens after TRUNCATE?
  Convenience for the common "every test wants a read+write token"
  case. Recommendation: yes, as a follow-up convenience helper, not
  in the v1 harness — explicit re-issue keeps tests honest.
- Should the harness ship an `examples/` two-repo workflow alongside
  the test fixtures, as RFC 0035 Open Q5 suggests? Recommendation:
  defer per the RFC; the test fixtures are not operator-onboarding
  artifacts.
