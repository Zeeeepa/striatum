---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["ergonomics_dx", "rfc-0040", "v1-5", "build"]
---

author: reviewer-unknown-model-002

# RFC 0040 V1.5 Build Review — Claude (ergonomics_dx)

Scope: developer-ergonomics review of the V1.5 implementation against
the F1–F6 surface described in `docs/dogfood/044/build/HANDOFF.md`.
Posture: first-time operator using composite tools via MCP, asking
whether the affordances are discoverable, the error surface is
actionable, and the operator workflow no longer needs hand-edits.

## Verdict

`accept_with_findings`. The composite tools deliver the operator-
decluttering goal: a single MCP call replaces the hand-copied
`ack` + `publish-artifact` + `verdict/complete` chain, lease lookup
is internal, and rollback emits a dedicated trail event. E2E tests
exercise the real MCP dispatch path. Findings are concentrated in
`tools/list` discoverability and the structured failure envelope's
silence about the failing composite step — useful gaps to close
before V2, none load-bearing on F1–F6 acceptance.

## F1 — MCP `tools/call` dispatch through the method registry

Implementation site:
- `src/striatum/daemon_pg/mcp_dispatch.py::dispatch_mcp_tool_call`
  (lines 16–123) owns lookup, capability authorization, envelope
  build, and routing through `DaemonRpcRouter.handle(...)`.
- `src/striatum/mcp.py::DaemonRpcServer.call_daemon_tool`
  (lines 556–578) parses `name`/`arguments`/`token`/`request_id`
  and delegates to the helper. Constructor now carries
  `repo_root` + `substrate_schema` (lines 469–472).
- `src/striatum/daemon_rpc/server.py::DaemonRpcRouter.handle`
  accepts `transport` (default `"rpc"`; MCP passes `"mcp"`) and
  `require_handshake` (MCP passes `False`).

Ergonomics: the dispatcher returns a strict MCP shape
`{content, structuredContent, isError}` with `structuredContent`
carrying `ok`, `method`, `audit_id`, and either `data` on success
or `error`/`error_message` on failure (`_tool_result` in
`mcp_dispatch.py:142–166`). `isError` correctly tracks `ok`, so
MCP clients can branch on a single boolean. Method-unknown and
capability denials both emit one `transport="mcp"` audit row with
exit code 10 (`mcp_dispatch.py:36–52, 66–91`) — the operator gets
the audit row id back in the same response. Discoverable: yes.
Actionable: yes for unknown methods and capability gates; less so
for handler-level composite failures (see F2/F3 finding below).

## F2 / F3 — `dogfood.publish_on_behalf` atomicity + verdict

Implementation site:
- `src/striatum/dogfood/operator_tools.py::publish_on_behalf`
  (lines 66–232). Validation up front: empty/too-long reason
  (82–88), `verdict_required` for review jobs (106–107),
  `verdict_not_allowed` for non-review (108–109), `invalid_verdict`
  enum check (110–111), `findings_artifact_required` for non-
  finding artifacts in review jobs (112–116), and existence check
  via `_validate_findings_artifact` (117–120, 501–516). All routed
  through `_failure(code, message, details=...)` which returns a
  uniform `{ok:false, status:"refused", error:{code,message,details}}`
  envelope (1166–1170).
- Lease lookup composes into a single helper
  `_active_work_for_artifact` (519–590). Operators no longer SQL-
  query for lease + queue message; the composite reads them from
  a single join and validates the (job_state, message_state)
  shape (575–580) with structured `lease_busy` / `no_active_lease`
  refusals.
- Composite mutations execute inside one `with transaction(conn):`
  block (123–219). `_ack_on_behalf_locked`,
  `_publish_artifact_locked`, `_record_verdict_locked`,
  `_complete_locked` are transaction-free helpers (267–498) so
  the outer scope is the single durable boundary.
- Rollback: on `ArtifactError` / `InvalidTransitionError` /
  `sqlite3.Error`, the transaction unwinds and a best-effort
  `dogfood.publish_on_behalf_failed` event is written with
  `outcome: "rolled_back"` (221–264).

Composite-rollback fixture:
`tests/test_dogfood_publish_on_behalf.py::test_publish_on_behalf_rolls_back_when_completion_fails`
(lines 275–304) monkeypatches `_complete_locked` to raise,
verifies the response is `composite_failed`, and asserts that the
job/message/lease states are preserved, no artifact or verdict
rows exist, no `job.completed` event fires, and exactly one
`dogfood.publish_on_behalf_failed` event lands.

### Finding F2-FIN-1 (medium): failure envelope does not name the failed step

Per the prompt's "Error messages name the failing composed step"
check: `publish_on_behalf` builds `composition_steps` inside the
transaction (operator_tools.py:124, appended at 135, 150, 186,
197) but on failure returns `_failure("composite_failed", str(exc))`
(line 232). The structured response gives the operator the
exception text but *not* the step list and not a `failed_step`
key. The rolled-back event payload (lines 252–262) is also silent
on which step raised. A first-time operator seeing
`composite_failed: forced complete failure` in their MCP client
has to read source to learn that the prior steps did run and roll
back. Recommend either: (a) include `failed_step` and partial
`composition_steps` in the `_failure` payload, or (b) tag the
`dogfood.publish_on_behalf_failed` event with the step name and
the trailing `composition_steps[-1].status` value.

### Finding F2-FIN-2 (low): MCP error code surface collapses composite refusals

`mcp_dispatch.py:114` reads `error = str(response.data.get("code")
or "command_failed")`. Composite refusals wrap the inner code
under `error.code` (operator_tools.py:1166–1170), not a top-level
`code`. Verify whether `DaemonRpcRouter` flattens this before
returning. If not, the MCP-visible `error` becomes the generic
`command_failed` string and the specific code
(`reason_required` / `lease_busy` / `findings_artifact_required` /
`invalid_verdict`) is buried in `data.error.code`. First-time MCP
users branching on `structuredContent.error` will hit a single
generic value. Recommend either threading the inner `code` up to
the top level, or documenting the `data.error.code` location as
the canonical error key.

## F4 — Watcher invocation in the daemon supervisor lifecycle

Implementation site:
- `src/striatum/process_progress.py::progress_loop_once` (lines
  27–73) joins `process_supervisors` ∩ `runs` so only attached
  supervisors under running/paused runs tick.
- `src/striatum/daemon.py::daemon_sweep_once` (line 1189) calls
  `progress_loop_once(repo_conn, repo=repo)` inside
  `connect_repo(repo)`, immediately before the per-run sweep
  cursor, and folds the result into the per-run sweep payload
  (line 1207: `result = {**result, "progress": progress_result}`).
- Heartbeat callback closes over the same repo connection
  (`process_progress.py:51–58`) and calls
  `striatum.cli.mutations.heartbeat` — no separate background
  thread, no extra connection.

Ergonomics: the watcher is invisible to operators. No new CLI
verb, no opt-in flag. The daemon sweep loop owns lifecycle. Event
types `supervisor.progress_watcher_heartbeat`,
`supervisor.progress_watcher_idle`,
`supervisor.progress_watcher_lost` (lines 135–179) give a clean
operator trail for `striatum status` / `striatum why` to surface
later. Log contents are never read — only `stat()` mtime — which
preserves D028 (no transcript capture). Good.

## F5 — Race / signal hardening

Implementation site:
- `ProcessProgressConfig.startup_grace_seconds = 60.0`
  (`process_progress.py:22–24`) plus the
  `_within_startup_grace` guard (121–132): within grace, a
  missing scratch path returns `waiting_for_log` (line 98)
  rather than warning.
- `should_stop` is plumbed from the caller and checked between
  supervisors (line 62) and inside `_tick_supervisor` (line 99).
  SIGTERM during sweep cannot start a new heartbeat after
  shutdown.
- Advisory lock is shared with `surgical_recovery` via
  `progress_advisory_lock(repo, job_id=...)` (operator_tools.py:
  666–670). Watcher returns `lock_busy`; surgical recovery
  returns `progress_lock_busy`. Neither revives stale work
  under the other.
- PID-reuse guard (`process_progress.py:86–95`) consults
  `process_start_time(pid)` before each tick; identity mismatch
  flips the row to `lost`, emits
  `supervisor.progress_watcher_lost`, and skips heartbeat.

Ergonomics: failure modes are visible as distinct event types
(`process_identity_changed`, `process_gone`, `lock_busy`,
`waiting_for_log`, `idle`) — easier to triage than a single
generic `stale` outcome.

## F6 — End-to-end execution-path tests

Implementation site:
- `tests/test_mcp_dogfood_e2e.py::test_mcp_publish_on_behalf_dispatches_and_completes_job`
  (lines 127–166) drives a real `tools/call` round-trip via
  `harness.mcp_client(token, repo_index=0).call_tool(
  "dogfood.publish_on_behalf", ...)`, asserts
  `data["status"] == "published_on_behalf_completed"`, and
  verifies the job/artifact/event rows + a single allowed audit
  row. Not mocked — uses the full MCP harness.
- `tests/test_mcp_dogfood_e2e.py::test_mcp_publish_on_behalf_records_review_verdict`
  (lines 169–203) exercises the review path through MCP and
  asserts `verdict_id`, `findings_artifact_id`, and the actual
  `verdicts` row.
- `tests/test_supervised_progress_watcher.py::test_progress_loop_once_heartbeats_attached_supervisor`
  (lines 274–305) wires the loop, inserts an attached supervisor,
  and asserts `expires_at` changed + the
  `supervisor.progress_watcher_heartbeat` event landed.
- `tests/test_supervised_progress_watcher.py::test_progress_loop_once_refuses_pid_identity_mismatch`
  (lines 308–334) inserts a supervisor with a poisoned
  `pid_start_time` and asserts the row flips to `lost`.

### Finding F6-FIN-1 (low): multi-repo MCP e2e tests skip without the PG harness

`tests/test_mcp_dogfood_e2e.py:15` declares
`pytestmark = pytest.mark.multi_repo`, so the two MCP e2e tests
do not run under `make test` on smoke-only environments. The
handoff's "10 skips" status note (HANDOFF.md:122–130) discloses
this. Direct-Python coverage of the same composite paths runs in
`tests/test_dogfood_publish_on_behalf.py`. Recommend either:
(a) a non-multi-repo MCP smoke that exercises one composite tool
through `DaemonRpcServer.handle_request` directly (no PG harness
needed), or (b) a CI note that flags the MCP-path coverage gap on
smoke-only runners.

## Backward compatibility

- Daemon RPC envelope-v1 unchanged: `RpcEnvelope`, `RpcResponse`,
  `daemon.hello`, `daemon.welcome`, `METHODS_ETAG` untouched
  (handoff confirms, and `daemon_rpc/server.py:218–244` adds a
  pure-routing path inside the existing `_route_dogfood` helper).
- `tools/list` shape unchanged. New entries `dogfood.publish_on_behalf`
  + `dogfood.surgical_recovery` already lived in METHOD_REGISTRY
  (`daemon_rpc/registry.py:77–78`); V1.5 makes their dispatch
  real instead of stub.
- Direct-Python `publish_on_behalf` keeps the
  `{ok, status, ...}` success and `{ok:false, status:"refused", error}`
  refusal shapes. New keys (`findings_artifact_id`, `verdict_id`)
  are additive. Verified by the unchanged assertions in
  `tests/test_dogfood_publish_on_behalf.py` (e.g. lines 240–272).

Regression coverage cited: `test_dogfood_publish_on_behalf.py`,
`test_mcp_mutation_capabilities.py` (per HANDOFF.md:113–118),
`test_mcp_capability_scope_e2e.py`. Per handoff, the slice runs
`42 passed, 10 skipped` under the `-k "mcp or progress or
dispatch or daemon_pg"` filter.

## Ergonomics gap not in F1–F6: `tools/list` lacks `inputSchema`

`src/striatum/mcp.py::DaemonRpcServer.daemon_tool_specs`
(lines 525–554) returns
`{name, description, required_capability, repository_scope_mode}`
per tool, with the description set generically to
`f"Invoke daemon RPC method `{entry.method}`."`. No `inputSchema`
field. A first-time MCP client doing `tools/list` cannot discover
that `dogfood.publish_on_behalf` requires `session_id`,
`artifact_path`, `artifact_kind`, `logical_name`, `reason` plus
optional `verdict` / `findings_artifact_id` / `verdict_rationale`
/ `summary`. The shape is documented in
`docs/rfcs/0040-mcp-driven-dogfood-harness.md` §2 and in the
docstring at `operator_tools.py:66–79`, but not pushed to the
tool client. Per the prompt's "first-time-discoverable" check,
this is the largest remaining ergonomics gap. Out of declared
V1.5 scope, but worth tracking for V1.6 — adding an
`inputSchema` per registry entry would let any MCP-compliant
client render an argument form without source-diving.

## Tests run / status

Per HANDOFF.md:122–130, `pytest -k "mcp or progress or dispatch
or daemon_pg" --tb=line` reports 42 passed / 10 skipped in
13.93s. The 10 skips are `multi_repo` PG-harness gated. The
build review did not re-run `make test`; the handoff's status
note flags an unrelated `D094` DECISION_LOG row over budget that
is outside this job's write scope. No regression cited.

## Summary

F1 through F6 are implemented and tested. Composite tools remove
the hand-edit footguns identified in dogfood-038/039/040. Two
medium/low ergonomics findings are worth landing before V2:
the failure envelope should name the failing composed step
(F2-FIN-1), and `tools/list` should expose `inputSchema`. One
low finding flags the MCP-path coverage gap on smoke-only CI
(F6-FIN-1). None block acceptance.
