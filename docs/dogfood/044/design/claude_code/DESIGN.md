# RFC 0040 V1.5 Design — Claude Code Lane

author: designer-unknown-model-001

This design fixes the six findings (F1–F6) that dogfood-040 left as
deferred RFC 0040 V1.5 scope (`docs/dogfood/040/OPERATOR_REPORT.md`
§"Recorded Risks and Follow-ups"). Every change is wired into existing
modules — no new public verb, no new envelope, no new tool name.

The daemon RPC envelope-v1 schema is unchanged. The chat-tools surface
in `src/striatum/web/chat_tools.py` (read tools, `generate_workflow_*`,
and the RFC 0040 V1 dogfood-lifecycle thin shells) keeps the same names,
parameters, and result shapes. The eleven entries in
`TOOL_ARGV` in `src/striatum/mcp.py:49` keep working byte-for-byte.

## Where the F1–F6 work lands

| Finding | Module                                             | Function / line                                  |
|---------|----------------------------------------------------|--------------------------------------------------|
| F1      | `src/striatum/mcp.py`                              | `DaemonRpcServer.call_daemon_tool` (line 554)    |
| F2/F3   | `src/striatum/dogfood/operator_tools.py`           | `publish_on_behalf` (line 48)                    |
| F4      | `src/striatum/daemon.py` + `src/striatum/daemon_supervisor/` | `run_daemon_foreground` (line 862) + new `progress_loop.py` |
| F5      | `src/striatum/daemon_supervisor/progress_watcher.py` | `tick` (line 172), `check_progress_once` (line 290) |
| F6      | `tests/test_dogfood_mcp_e2e.py` (new)              | smoke harness hook in `Makefile`                 |

## F1 — Daemon MCP `tools/call` dispatch wiring

### Problem (exact code path)

The MCP entry point is `DaemonRpcServer.call_daemon_tool` in
`src/striatum/mcp.py:554`. The current body:

1. Looks up the registry entry via
   `METHOD_REGISTRY.get(name)` (line 566) — sourced from
   `src/striatum/daemon_rpc/registry.py:_ENTRIES` which already
   contains `dogfood.publish_on_behalf` (line 77) and
   `dogfood.surgical_recovery` (line 78).
2. Calls `authorize(...)` (line 599) and `append_audit_row(...)`
   (line 605).
3. On `auth.decision == "allowed"` (line 632) it returns
   `_daemon_tool_result(name, ok=True, audit_id=audit_id)` — a
   constant "success" envelope with `structuredContent.ok = True`
   and **no `result` payload**, no `data` block, no state mutation.

The audit row, the request_log row, and the MCP response all report
success while the underlying method handler in
`DaemonRpcRouter._route` (`src/striatum/daemon_rpc/server.py:146`) is
never reached. Operators calling `dogfood.publish_on_behalf` over MCP
see `{"ok": true}` and an `audit_id`, but the artifact is not
published, the lease is not advanced, no event is written.

### Fix

`DaemonRpcServer` already has a `pg_conn`; it needs a sibling
`DaemonRpcRouter` so the MCP path becomes a thin façade over the same
dispatcher the daemon RPC transport uses. Concretely:

1. **Hold the router as state**. Extend
   `DaemonRpcServer.__init__` (`src/striatum/mcp.py:469`) to accept
   (or lazily build) a `DaemonRpcRouter`:

   ```python
   from striatum.daemon_rpc.envelope import RpcEnvelope
   from striatum.daemon_rpc.server import DaemonRpcRouter

   class DaemonRpcServer:
       def __init__(
           self,
           *,
           pg_conn: Any | None = None,
           router: DaemonRpcRouter | None = None,
           repo_root: Path | None = None,
       ) -> None:
           self.pg_conn = pg_conn
           self.router = router or DaemonRpcRouter(
               pg_conn=pg_conn, repo_root=repo_root
           )
   ```

   The factory in `striatum.daemon` that constructs the MCP server
   passes the same `DaemonRpcRouter` used by the unix-socket RPC
   transport so audit IDs and `_handshaken_connections` are shared.

2. **Replace the audit-only branch with a router call**.
   `call_daemon_tool` (`src/striatum/mcp.py:554`) keeps its existing
   pre-flight: the `method_unknown` audit row at line 575 stays as is
   (so unknown tool names continue to deny + audit), and the
   `auth.decision != "allowed"` branch at line 614 stays as the deny
   audit row. The two changes are:

   a. On `auth.decision == "allowed"` (line 632), build a
      `RpcEnvelope` from `(name, arguments, request_id, token_value)`
      and call `self.router.handle(envelope, connection_id="mcp")`.
      Do **not** call `append_audit_row` here a second time — the
      router's `_record_and_return` (`server.py:104`) writes the
      transport=`mcp` audit row with the actual `exit_code`. Drop
      the existing `append_audit_row` at line 605 from the allowed
      branch so the audit chain has one row per call.

   b. Translate `RpcResponse` into the MCP `{content,
      structuredContent, isError}` shape via a new helper
      `_daemon_tool_result_from_rpc(name, response)`. If
      `response.ok` is true, `structuredContent` is
      `{"ok": True, "method": name, "audit_id": audit_id,
      "data": response.data}`; if false, `structuredContent` is
      `{"ok": False, "method": name, "audit_id": audit_id,
      "error": response.error.code,
      "error_message": response.error.message}` and `isError=True`.

3. **First-message handshake**. `DaemonRpcRouter.handle` enforces a
   `daemon.hello`-first invariant via `_handshaken_connections`
   (`server.py:64`). Adding a connection key for the MCP transport
   would require every MCP client to call `daemon.hello` before
   `tools/call`. To preserve backward compatibility for the existing
   `generate_workflow_*` tools (which are dispatched through
   `LocalRpcServer`, not the daemon path), the daemon route
   short-circuits the handshake: when constructing the router, the
   server pre-populates
   `router._handshaken_connections.add("mcp")` once at startup
   (after the daemon's own `daemon.hello` flow has loaded
   `METHODS_ETAG`). Tools/list and tools/call from the MCP route then
   reuse `connection_id="mcp"`.

4. **Repository scope plumbing**. The daemon route requires
   `repository_id` for `repository_scope=True` methods
   (`server.py:71`). The chat-tool argument shape in
   `chat_tools.py` does not currently surface `repository_id` because
   the V1 thin shells route through `striatum.api.invoke` against
   the local repo. To unblock single-repo MCP clients (Claude
   Desktop attached to one striatum), `DaemonRpcServer` injects
   `repository_id` from the bound `repo_root` when the caller does
   not supply one. The resolution helper looks up
   `striatumd.repositories.repository_id` by `repo_root` using the
   same query as `DaemonRpcRouter._repo_root_for`
   (`server.py:129`). Cross-repo callers may still supply it
   explicitly.

### Audit row shape after the fix

For every `tools/call`, the audit chain contains exactly one row:

- `transport = "mcp"`
- `method = <tool name>`
- `request_id` is either the caller-supplied value, the argument's
  `request_id`, or a fresh `mcp_<uuid>` generated at line 567 (no
  change).
- `exit_code = None` on allowed-and-handler-success;
  `exit_code = 10` on auth deny or unknown method (unchanged);
  `exit_code = <handler exit code>` on handler failure (new — the
  router already records this at `server.py:114` for the `rpc`
  transport).

The existing F1 friction (`exit_code=null` while the handler did
nothing) goes away because the audit row is appended after dispatch,
inside `DaemonRpcRouter._record_and_return`, with the actual handler
outcome.

### Backward compatibility (F1)

- `LocalRpcServer.call_tool` (`mcp.py:433`) is untouched. The
  legacy stdio framing path (`init`, `workflow_validate`,
  `claim_next`, …) keeps its current shape and stays decoupled from
  daemon authorization.
- `DaemonRpcServer.read_resource` (`mcp.py:513`),
  `daemon_tool_specs` (`mcp.py:523`), and the `tools/list` filter
  by capability stay as is.
- `chat_tools.py` thin shells (`run_prepare`, `run_start`, …,
  `evidence_export`) keep their existing argv-translation via
  `striatum.api.invoke`. The fix only matters when the daemon-MCP
  transport is the carrier (i.e., real MCP clients). When the
  chat-tools surface routes locally, `_tool_dogfood_lifecycle`
  (`chat_tools.py:835`) still works because it does not use
  `DaemonRpcServer` at all.

## F2 & F3 — Composite tool atomicity + verdict recording

### Problem (exact code path)

`publish_on_behalf` in
`src/striatum/dogfood/operator_tools.py:48` performs:

1. `ack_work(conn, …)` (line 98) — writes to `queue_messages`,
   `leases`, `jobs.state`.
2. `publish_artifact(conn, …)` (line 108) — writes to
   `artifacts`, `event_log`.
3. Either `record_review_verdict` (line 140) or `complete_job`
   (line 155) — writes to `verdicts` / `jobs.state` / `event_log`.
4. **Only then** is a single `dogfood.publish_on_behalf` event
   inserted inside a `with transaction(conn):` block at line 166.

Each of steps 1–3 opens and commits its own SQLite transaction.
If step 2 succeeds and step 3 raises, the database keeps the
half-applied state: the artifact is recorded, the job is acked,
but no verdict and no completion. The trailing
`composition_steps` array (line 93) is built optimistically and
only persisted in the success path, so the audit/event trail does
not name the failed step.

### Fix — single-transaction composition

Wrap the entire composition in one `with transaction(conn):`
opened immediately after argument validation (line 71). All
state-mutating helpers already take a `sqlite3.Connection`
parameter and do not call `conn.commit()` themselves — they rely
on the caller's transaction (see `complete_job` at
`src/striatum/db.py:1446` and `record_review_verdict` at
`src/striatum/db.py:1566`, which both use `transaction(conn)`
internally and nest cleanly with an outer transaction). The new
shape:

```python
def publish_on_behalf(conn, *, …) -> JsonObject:
    # … reason validation, work-context lookup (read-only, can sit
    # outside the transaction) …
    composition_steps: list[JsonObject] = []
    try:
        with transaction(conn):
            ack_result = _ack_step(conn, …, composition_steps)
            artifact = _publish_step(conn, …, composition_steps)
            if job_type == "review":
                verdict_result = _verdict_step(conn, …, composition_steps)
            else:
                completion = _complete_step(conn, …, composition_steps)
            insert_event(
                conn,
                event_type="dogfood.publish_on_behalf",
                payload={
                    "operation": "publish_on_behalf",
                    "operator_reason": reason,
                    "composition_steps": composition_steps,
                    "status": "applied",
                    …,
                },
            )
    except Exception as exc:
        return _composite_failure(
            operation="publish_on_behalf",
            failed_step=composition_steps[-1]["step"] if composition_steps else "ack",
            error=exc,
            applied_steps=composition_steps[:-1],
        )
    return _composite_success(…)
```

`transaction(conn)` in `striatum.db` already uses SQLite's
`BEGIN IMMEDIATE` semantics. On exception, the rollback rewinds
all three intermediate writes atomically; SQLite's auto-rollback
handles this for free.

### Compensation event vs SQL rollback

SQLite gives us free rollback for steps 1–3 because they are all
DB-only. The only non-DB side effect is the artifact file: by the
time `publish_artifact` is called, the file on disk already
exists (the operator wrote it before invoking the tool). The
artifact row insert (`publish_artifact` in
`src/striatum/artifacts.py:433`) is the only DB record;
rolling that back leaves the file in place, which is the
correct invariant — the artifact's bytes are still durable
provenance.

For the audit trail itself, we add a `dogfood.publish_on_behalf`
**failure** event written *outside* the rolled-back transaction:

```python
except Exception as exc:
    with transaction(conn):
        insert_event(
            conn,
            event_type="dogfood.publish_on_behalf_failed",
            payload={
                "operator_reason": reason,
                "applied_steps": composition_steps[:-1],
                "failed_step": composition_steps[-1]["step"],
                "error_type": type(exc).__name__,
                "error_message": str(exc),
            },
        )
    return _composite_failure(…)
```

The success event-row already names every step in
`composition_steps`; the failure-event row names the failing step
plus the steps that successfully ran inside the rolled-back
transaction (which were then discarded). The audit chain in
`audit_log` (daemon side) and `event_log` (repo side) both record
exactly one row per composite call: success-with-N-steps or
failure-with-step-name.

### Verdict-recording specifics (F3)

When `job_type == "review"`, the current code path passes
`findings_artifact_id or artifact_id` to `record_review_verdict`
(line 146). Two atomicity-relevant tightening points:

- The `verdict` argument is validated up-front (line 88) so the
  inside-transaction code path never sees a `None` value for a
  review job.
- `record_review_verdict` in `src/striatum/db.py:1566` is called
  while the outer transaction holds the write lock. Any failure
  it raises (e.g., `InvalidTransitionError` for a non-claimed
  job) rolls back the ack + publish_artifact writes too. The
  RFC's "single audit row per composite operation" invariant is
  preserved.

### Same-shape change for `surgical_recovery`

`surgical_recovery` in `operator_tools.py:296` already has the
right shape: it opens `with transaction(conn):` at line 328
covering the lease + queue_message + job + supervisor updates and
the event insert. No change needed for F2/F3 here; the design
just enforces that any future steps added to it stay inside the
same transaction.

### Backward compatibility (F2/F3)

- The success path's JSON envelope (`composition_steps`,
  `status: "published_on_behalf{,_reviewed,_completed}"`,
  `artifact_id`, `verdict_id`) is byte-identical to the V1 shape.
- The failure path now returns `{"ok": False, "status":
  "refused", "operation": "publish_on_behalf",
  "error": {"code": "composite_failed",
  "message": …, "details": {"failed_step": …,
  "applied_steps": […]}}}` — same shape as existing `_failure`
  envelopes returned by validation refusals (line 836). No
  caller change required.
- All existing tests in
  `tests/test_dogfood_publish_on_behalf.py` and
  `tests/test_dogfood_surgical_recovery.py` continue to pass: the
  success-path assertions match the existing JSON exactly.

## F4 — Watcher invocation in the supervisor lifecycle

### Problem (exact code path)

The `SupervisedProgressWatcher` class lives at
`src/striatum/daemon_supervisor/progress_watcher.py:154` and is
fully implemented (DB-aware `_active_lease` lookup at line 261,
mtime polling at line 88, advisory lock at line 128, idle warning
at line 228). It is **never instantiated by daemon code**: a grep
over `src/striatum/` only finds it referenced by the unit-test
file `tests/test_supervised_progress_watcher.py:15` and by
`operator_tools.py:29` for the `progress_advisory_lock` import.

The daemon's lifecycle loop is `run_daemon_foreground` at
`src/striatum/daemon.py:862`. Its inner loop (line 906) only
calls `daemon_sweep_once()`. Supervisor rows in
`process_supervisors` (written by `supervise_start` at
`src/striatum/supervisor.py:101`) are never observed by a
watcher.

### Fix — daemon-owned watcher loop

Add a sibling loop alongside `daemon_sweep_once` that, on every
sweep tick, enumerates active supervisors across registered
repositories and `tick`s the watcher once per supervisor. Two new
pieces:

1. **`src/striatum/daemon_supervisor/progress_loop.py`** (new
   module). One public helper:

   ```python
   def progress_loop_once(
       *,
       conn_registry: sqlite3.Connection,
       config: ProgressWatcherConfig = ProgressWatcherConfig(),
       clock: Callable[[], float] = time.time,
   ) -> dict[str, Any]:
       """One pass: for every active repo, tick every attached supervisor."""
       results: list[dict[str, Any]] = []
       for repo_row in _active_repositories(conn_registry):
           repo = Path(str(repo_row["repo_root"]))
           with connect_repo(repo) as repo_conn:
               supervisors = _attached_supervisors(repo_conn)
               for sup in supervisors:
                   watcher = SupervisedProgressWatcher(
                       heartbeat_callback=lambda **kw: heartbeat(
                           repo_conn, **kw
                       ),
                       conn=repo_conn,
                       repo=repo,
                       config=config,
                       clock=clock,
                   )
                   target = _record_to_target(sup)
                   result = watcher.tick(target)
                   results.append(_serialize_result(result))
       return {"results": results}
   ```

   The function is sync, returns structured results, and is
   trivially testable without sleeping. Production callers wrap
   it in the daemon's main loop; tests call it once with a fake
   clock.

2. **Wire into `run_daemon_foreground`**. Augment the inner loop
   (`daemon.py:907`) to call `progress_loop_once(conn_registry=conn,
   config=ProgressWatcherConfig.from_daemon_config())` immediately
   after `daemon_sweep_once()`. Both run at the same cadence
   (`sweep_interval_seconds`, default 60s — short of the
   watcher's 30s default; the watcher tolerates a slower poll
   rate fine, the `refresh_threshold_seconds` is the only knob
   that matters and it is configurable).

   The shutdown path is already correct: `stopping` is checked at
   the top of the inner loop (line 907). Per-supervisor ticks are
   bounded (mtime read + maybe one heartbeat callback +
   per-job advisory lock acquire). No per-supervisor task to
   join.

### Why no per-supervisor `asyncio.create_task` or
`threading.Thread`

The existing daemon is a single-process, sync, sweep-based
loop (`daemon.py:907`). Spawning a thread per supervisor adds
join-on-shutdown complexity and conflicts with D028's
"daemon never reads child stdout" posture (threads imply
shared file descriptors). The single-loop sync poller has:

- No race on watcher creation/teardown when supervisors come and
  go (each sweep observes the current set in `process_supervisors`).
- No SIGTERM-handler interaction with worker threads (the
  existing `_stop` handler in `daemon.py:899` just flips a flag).
- Trivial backpressure: if a sweep tick takes >30s, the
  watcher's `refresh_threshold_seconds=60` window absorbs it.

### `SupervisedProgressTarget` from supervisor row

The watcher target type (`progress_watcher.py:47`) already maps
1:1 onto `process_supervisors` columns: `supervisor_id`,
`session_id`, `pid`, `scratch_path`, `state`, `run_id`. The
`log_path` field is `None`; the watcher falls back to
`scratch_path.rglob("*.log")` via `newest_log_mtime`
(`progress_watcher.py:88`) — which matches the supervised wrapper's
log shape under `scratch/<supervisor_id>/`. The `lease_id` and
`job_id` are looked up from `leases` by `_active_lease`
(`progress_watcher.py:261`); the loop does not need to surface
them.

The advisory lock guarantees safety against a concurrent
`surgical_recovery` (`operator_tools.py:336`), which acquires the
same per-job `progress_lock_path`.

### Backward compatibility (F4)

- Daemons currently running with no progress watcher get one
  automatically when they restart against the V1.5 code. No CLI
  flag; the watcher is always on. (Operators who want it off can
  set `idle_threshold_seconds` to a huge value via daemon config,
  but that is escape-hatch material, not a documented surface.)
- `process_progress.py` referenced in the prompt does not exist;
  the watcher module is `daemon_supervisor/progress_watcher.py`,
  matching `BUILD_HANDOFF.md` of dogfood-040 (line 75) and the
  RFC (line 389). The design uses the actual path.

## F5 — Watcher race + signal hardening

### Problem (exact code path)

`SupervisedProgressWatcher.tick`
(`progress_watcher.py:172`) and `check_progress_once`
(`progress_watcher.py:290`) both depend on `os.stat(log).st_mtime`.
Three concrete race windows:

1. **Log rotation race.** Supervised wrappers may rotate their
   stdout log (`<scratch>/codex-logs/packet-NNNN.log`) when a new
   packet is delivered. Between the rotation and the new file
   being created, the watcher can observe one of: (a) a stale
   file with old mtime; (b) `FileNotFoundError` from the rename
   gap; (c) the new file but empty. `newest_log_mtime`
   (`progress_watcher.py:88`) iterates `rglob("*.log")` and
   catches `FileNotFoundError` per-path, so case (b) is
   currently handled. Case (a) is the dangerous one: a stale
   file's mtime is the rotation timestamp, which can be < 60s
   old and trigger a spurious heartbeat for a session that is
   not actually progressing.

2. **Watcher start before the wrapper's first log write.** When
   the watcher loop sees a fresh `attached` supervisor row, the
   wrapper child may not have flushed any log line yet. The
   first `newest_log_mtime` returns `None`, which falls through
   to `ProgressWatcherResult(status="no_log")` (line 187). That
   is correct (no heartbeat) but emits a warning on every tick
   until the wrapper writes. The warning channel
   (`warning_callback`, line 163) gets noisy.

3. **SIGTERM during heartbeat call.** The daemon's
   `_stop(SIGTERM)` handler at `daemon.py:899` flips `stopping`
   to True. The watcher tick is inside the main loop. If
   `_heartbeat` (`progress_watcher.py:251`) is mid-call when
   the signal arrives, the Python signal-handler interruption
   semantics let the call complete (no `KeyboardInterrupt`
   propagation through callable boundaries), but the advisory
   lock acquired by `progress_advisory_lock`
   (`progress_watcher.py:128`) is released by the `finally`
   block at line 148–151. The actual hazard is: if the signal
   handler raises `SystemExit` during the `fcntl.flock(LOCK_UN)`
   call, the file descriptor is leaked. This is a latent bug
   even though the kernel cleans up on process exit.

### Fix — three targeted guards

1. **Log rotation: track per-supervisor `(path, mtime)` between
   ticks.** Add a small dict
   `self._last_seen_log: dict[str, tuple[str, float]]` to
   `SupervisedProgressWatcher`. On each tick, `newest_log_mtime`
   returns `(path, mtime)` instead of just `mtime` (extend the
   helper's return type — `Optional[Tuple[Path, float]]`). The
   watcher only treats mtime as fresh when `path` has not changed
   since the last tick *or* the mtime has strictly increased. If
   path changed AND mtime did not increase, treat it as
   `no_recent_progress` and skip heartbeat. This collapses the
   rotation race to "miss one heartbeat after rotation", which
   the operator-side wrapper recovers from on the next packet
   write.

2. **First-write race: suppress the `no_log` warning for the
   first `grace_seconds` after supervisor `started_at`.** Pass
   `supervisor.started_at` through to the tick (already
   reachable via `process_supervisors`, just add it to
   `SupervisedProgressTarget`). When `mtime is None` and
   `clock() - started_at < grace_seconds` (default 30s; same as
   `refresh_threshold_seconds`), return
   `ProgressWatcherResult(status="warming_up", …)` instead of
   `no_log` and do not invoke `warning_callback`. After the
   grace window, normal `no_log` warning fires.

3. **SIGTERM during heartbeat: bracket the heartbeat call in a
   `try/finally` that flushes the lock release with masked
   signals.** Wrap the `with progress_advisory_lock(...)` block
   (`progress_watcher.py:205-217`) so signal handling for SIGTERM
   and SIGINT is deferred only for the lock-release window:

   ```python
   sigmask = signal.pthread_sigmask(
       signal.SIG_BLOCK, {signal.SIGTERM, signal.SIGINT}
   )
   try:
       with progress_advisory_lock(self.repo, job_id=lease.job_id) as acquired:
           if not acquired:
               return _lock_busy_result(…)
           self._heartbeat(supervisor.session_id, lease.lease_id)
   finally:
       signal.pthread_sigmask(signal.SIG_SETMASK, sigmask)
   ```

   Inside the `with` block, the kernel queues SIGTERM. Once the
   `flock(LOCK_UN)` in `progress_advisory_lock.__exit__` runs
   and we exit the `finally`, the queued signal is delivered to
   the daemon main loop, which sees `stopping=True` on the next
   loop iteration and shuts down cleanly. The fd is never leaked
   because `os.close(fd)` in the context manager runs before the
   signal is re-raised. The same wrapping applies to
   `check_progress_once` (`progress_watcher.py:290`) which also
   uses `progress_advisory_lock` indirectly via the module-level
   threading lock at line 315.

### Validation

The race windows above map to three new unit tests in
`tests/test_supervised_progress_watcher.py`:

- `test_rotation_with_stable_mtime_skips_heartbeat` — write log
  A, tick (heartbeat fires), rotate to log B with mtime < log
  A's, tick (no heartbeat).
- `test_no_log_grace_window` — start supervisor at T0, tick at
  T0+5s with no log present (assert `status="warming_up"`,
  warning_callback not invoked), tick at T0+90s (assert
  `status="no_log"`, warning_callback invoked once).
- `test_heartbeat_under_sigterm` — fork a child watcher,
  deliver SIGTERM during a synchronous heartbeat callback that
  sleeps 0.5s, assert lock file `flock` is released (verify by
  trying to acquire from parent) and child exits cleanly.

### Backward compatibility (F5)

The watcher's public surface (`tick`, `watch_progress`,
`check_progress_once`, `ProgressWatcherResult`,
`ProgressWatcherConfig`) keeps its existing shape. The new
`status="warming_up"` value is additive — existing callers
treating it like any non-`heartbeat` status will continue to
work. The new `started_at` field on `SupervisedProgressTarget`
defaults to `None`, so tests constructing targets directly
continue to work; only callers that want the grace window
populate it. The signal masking is local to the heartbeat
window; no interference with the daemon's existing SIGTERM
handler at `daemon.py:899`.

## F6 — End-to-end tests

### Problem (exact gap)

The existing suite covers each layer in isolation:

- `tests/test_mcp_mutation_capabilities.py` — capability gating
  at `tools/list` and the audit-row append (no dispatch
  assertion).
- `tests/test_daemon_rpc.py` — RPC envelope shapes against an
  in-process router (no MCP transport).
- `tests/test_dogfood_publish_on_behalf.py` — direct
  `publish_on_behalf` call against a SQLite connection (no
  daemon, no MCP).
- `tests/test_dogfood_surgical_recovery.py` — same shape.
- `tests/test_supervised_progress_watcher.py` — `tick` in
  isolation (no daemon loop).
- `tests/test_mcp_capability_scope_e2e.py` — capability scope
  filtering only; does not exercise dispatch into the registry.

Nothing exercises the full path: MCP `tools/call` → daemon auth +
audit → registry dispatch → composite tool atomicity (success
and failure) → state change → audit row.

### Fix — new e2e module `tests/test_dogfood_mcp_e2e.py`

Hooks into the existing pytest harness. The module owns four
tests, each running against a real daemon `DaemonRpcRouter`
constructed with a temp SQLite repo state and (where needed) a
fake `pg_conn` that supports the minimal `authorize` /
`append_audit_row` API. Existing fixtures from
`tests/test_daemon_rpc.py` and
`tests/test_dogfood_publish_on_behalf.py` are reused — no new
substrate.

1. **`test_mcp_publish_on_behalf_full_dispatch_success`**.
   - Prepare run, register session, claim packet, write the
     artifact file on disk.
   - Build an MCP `tools/call` envelope for
     `dogfood.publish_on_behalf` with a valid capability token.
   - Call `DaemonRpcServer.call_daemon_tool(...)` (post-F1
     fix).
   - Assert: response `structuredContent.ok=True`,
     `structuredContent.data.status="published_on_behalf_completed"`,
     `data.composition_steps` has 3 entries, exactly one new
     row in `audit_log` (transport=`mcp`, exit_code=`null`),
     one new row in `request_log`, one new event of type
     `dogfood.publish_on_behalf` in the repo `event_log`, and
     the underlying `jobs` table shows
     `state="completed"`.

2. **`test_mcp_publish_on_behalf_composite_rollback`**. Force
   the verdict-recording step to fail by passing a malformed
   `verdict` value to a review job, after the ack and
   publish_artifact steps would have succeeded.
   - Assert: response `structuredContent.ok=False`,
     `error="composite_failed"`,
     `details.failed_step="verdict"`,
     `details.applied_steps=["ack","publish_artifact"]`.
   - Assert that `jobs.state` is **back to its
     pre-call value** (e.g., `claimed`), no row in
     `artifacts`, no row in `verdicts`. The single rolled-back
     transaction is the F2/F3 invariant.
   - Assert exactly one `dogfood.publish_on_behalf_failed`
     event in `event_log` and exactly one row in `audit_log`
     with `exit_code != null`.

3. **`test_mcp_dispatch_audit_chain_single_row`**. Call any
   read-shaped dispatch method (e.g., `dashboard.all`) over
   MCP. Assert exactly one row appended to `audit_log` with
   transport=`mcp`. Pre-F1, this test would catch the
   double-append regression if a future refactor reintroduces
   the audit append in `call_daemon_tool` itself.

4. **`test_mcp_dispatch_unknown_method_audit_denied`**. Call
   `tools/call` with an unregistered method name. Assert
   `isError=True`, audit row with `decision="denied"`,
   `denial_reason="method_unknown"`, `exit_code=10`. Pre-F1
   this works by accident (the audit row is appended in the
   `method_unknown` branch at `mcp.py:575`); post-F1 it must
   continue to work because that branch is preserved.

### Smoke harness hook

`Makefile` already runs `pytest` as part of `make test` and a
narrower set in `make smoke`. The new module attaches to
`make smoke` via the existing smoke selector in the pytest
config (currently `tests/test_smoke_*.py` and an explicit
allowlist). Add `tests/test_dogfood_mcp_e2e.py::test_mcp_publish_on_behalf_full_dispatch_success`
to the allowlist; the other three stay in `make test`. The
smoke selector keeps `make smoke` under a minute.

### Backward compatibility (F6)

- No new test fixtures replace existing ones; the e2e module
  reuses the per-test temporary repo helper from
  `test_dogfood_publish_on_behalf.py` and the daemon helper
  from `test_daemon_rpc.py`.
- No change to `Makefile` lint/typecheck/test targets beyond
  the new file being picked up by glob discovery.

## Backward-compatibility summary

The proposal explicitly preserves:

- `RpcEnvelope` and `RpcResponse` schemas (no field added or
  removed) in `src/striatum/daemon_rpc/envelope.py`.
- `METHOD_REGISTRY` entries in
  `src/striatum/daemon_rpc/registry.py:_ENTRIES`. No new method
  added; the V1.5 work uses the existing
  `dogfood.publish_on_behalf` and `dogfood.surgical_recovery`
  entries.
- `chat_tools.py` exported `TOOL_NAMES`,
  `DOGFOOD_LIFECYCLE_TOOL_NAMES`, `ANTHROPIC_TOOLS`, `OPENAI_TOOLS`
  (no rename, no insertion).
- `LocalRpcServer` stdio path (`mcp.py:377`) untouched.
- Audit-row column shape unchanged; only the timing of when the
  row is appended in the MCP path changes (from "before dispatch"
  to "after dispatch", through the existing
  `DaemonRpcRouter._record_and_return`).

## Out of scope (consistent with the prompt)

- RFC 0040 §6 future work (Go daemon mirror, sidecar-signal
  watcher).
- New tools beyond F1–F6 wiring.
- Hosted services, multi-tenant MCP, frontend changes.
- Workflow JSON schema or the workflow generator catalog.

## Decision summary (one line each)

- **F1**: `DaemonRpcServer.call_daemon_tool`
  (`src/striatum/mcp.py:554`) constructs a `RpcEnvelope` and
  delegates to a held `DaemonRpcRouter`; the audit row is
  written once by the router's `_record_and_return`.
- **F2**: One `with transaction(conn):` wraps ack +
  publish_artifact + (verdict|complete) + success event in
  `publish_on_behalf` (`operator_tools.py:48`).
- **F3**: Failure path writes a single
  `dogfood.publish_on_behalf_failed` event outside the
  rolled-back transaction; rollback is SQLite-native.
- **F4**: New `daemon_supervisor/progress_loop.py` invoked from
  `run_daemon_foreground` (`daemon.py:907`) after
  `daemon_sweep_once`.
- **F5**: Track `(path, mtime)` between ticks; suppress
  `no_log` warning during start-up grace window; mask SIGTERM
  for the heartbeat + advisory-lock release window.
- **F6**: New `tests/test_dogfood_mcp_e2e.py` exercises full
  MCP-call → daemon-dispatch → state-change → audit-row path,
  including the F2/F3 rollback case; one test attached to
  `make smoke`.
