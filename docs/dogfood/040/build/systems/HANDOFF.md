# Dogfood-040 Systems Handoff

author: implementer-codex-gpt-5.5-002
status: complete

## Scope Shipped

Implemented the systems half of RFC 0040 V1:

- Added `src/striatum/dogfood/operator_tools.py` with `publish_on_behalf()` and `surgical_recovery()` dogfood composite helpers.
- Exposed those composites through daemon RPC routing as `dogfood.publish_on_behalf` and `dogfood.surgical_recovery`.
- Added the closed `surgical_recovery` daemon RPC capability and registered both dogfood composite methods.
- Added a daemon-supervisor progress watcher in `src/striatum/daemon_supervisor/progress_watcher.py`.
- Added PostgreSQL daemon migration `0004_dogfood_surgical_recovery.sql` so daemon RPC and client capability constraints accept `surgical_recovery`.
- Added focused tests for the composite helpers, capability registry, daemon MCP filtering, daemon PG migration, and supervised progress watcher behavior.

## Implementation Notes

`publish_on_behalf()` validates a non-empty operator reason, finds exactly one active non-terminal lease for the session, validates the requested artifact tuple against the job contract, then composes `ack`, `publish_artifact`, and either `complete` or `record_review_verdict`. It records one repo-local `dogfood.publish_on_behalf` event carrying the operator reason and composition steps. Daemon RPC still records the metadata-only daemon audit row around the method call.

`surgical_recovery()` is intentionally narrower than generic recovery. It only accepts repo-write jobs in `stale_lease`, requires required artifacts to exist on disk inside write scope, refuses concurrent active leases and active supervisors, validates a lost supervisor or pointer PID identity when present, and restores the lease, queue message, job, supervisor, and pointer state in one transaction. It shares the per-job progress advisory lock with the watcher to avoid racing a heartbeat refresh.

`SupervisedProgressWatcher.tick()` is the production-oriented watcher API. It checks attached supervisor state, process liveness through `os.kill(pid, 0)`, newest `*.log` mtime under the supervisor scratch directory, the active lease for the supervisor session, and heartbeats through an injected callback when progress is fresh. It returns structured statuses such as `heartbeat`, `idle`, `no_active_lease`, `process_gone`, `no_log`, and `lock_busy`.

## Verification

Focused tests run:

```bash
pytest -q tests/test_daemon_rpc_registry.py tests/test_supervised_progress_watcher.py tests/test_dogfood_publish_on_behalf.py tests/test_dogfood_surgical_recovery.py
pytest -q tests/test_chat_tools.py tests/test_web_chat.py tests/test_mcp_mutation_capabilities.py
pytest -q tests/test_daemon_pg.py tests/test_daemon_rpc.py tests/test_mcp_mutation_capabilities.py
```

All focused tests passed.

## Deferred

No new CLI recovery verb was added; the RFC scoped the composites as daemon/MCP-callable dogfood tools and the current routing covers that path. Full daemon sweep-loop integration for starting watcher tasks remains a follow-up because this job's write scope did not include `src/striatum/daemon.py`.
