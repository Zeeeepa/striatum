---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["ergonomics_dx", "rfc-0040", "v1-5", "design"]
---

author: reviewer-unknown-model-001

# RFC 0040 V1.5 Design Synthesis — Ergonomics_DX Review

## Scope And Posture

Fresh-context review of `docs/dogfood/044/DESIGN_SYNTHESIS.md` only, from a
first-time-operator perspective. Verdict acceptance hinges on whether the
F1-F6 affordances are discoverable and consistent for someone calling the
composite MCP tools without prior thread state.

## Verdict Summary

**accept_with_findings.** The synthesis pinpoints concrete entry points,
files, and lifecycle hooks across F1-F6; backward-compat is asserted
explicitly. Two ergonomic gaps remain — composite-step failure naming and
operator-visible watcher status — that are inexpensive to close at build
time and do not require revising the design.

## Pinpoint Checks

### F1 dispatch wiring — concrete ✓

The synthesis names a concrete daemon entry function and a concrete
registry handle, not "the dispatcher":

- Entry: `src/striatum/daemon_pg/mcp_dispatch.py::dispatch_mcp_tool_call`
  (§F1 lines 19-22).
- Caller boundary: `src/striatum/mcp.py::DaemonRpcServer.call_daemon_tool`
  remains the MCP framing surface (§Implementation Boundary, lines 13-15).
- Method-registry handle: `METHOD_REGISTRY[name]` plus
  `DaemonRpcRouter._route_dogfood` for composites (§F1 line 36).
- Router compatibility flags `transport` and `require_handshake=False`
  are named and justified, not handwaved (§F1 lines 34-36).

A first-time reader can trace the path: MCP request → `call_daemon_tool` →
`dispatch_mcp_tool_call` → `METHOD_REGISTRY[name]` →
`DaemonRpcRouter.handle`. The audit-row contract ("one row from
`_record_and_return` after the handler returns", §F1 lines 37-38) replaces
the prior misleading allow-row and is checkable.

### F2/F3 atomicity model — one approach with justification ✓

§F2/F3 lines 44-46 chooses "a single SQL transaction wrapping the
composite" and gives the reason: every authoritative side effect is a
repo-local SQLite row, artifact bytes already exist on disk, so rollback
is bounded. No alternative menu. The transaction-free internal helpers
(`_ack_on_behalf_locked`, `_publish_artifact_locked`,
`_record_verdict_locked`, `_complete_locked`) are named (§F2/F3 line 44).

Review-job preconditions are concrete: verdict required before the
transaction starts; `findings_artifact_id` defaulting only when the
published kind is `finding` (§F2/F3 lines 47-48). Result keys
(`artifact_id`, `findings_artifact_id`, `verdict_id`) are listed (line
48).

### F4 watcher invocation — named lifecycle function ✓ (with location note)

Invocation point is concrete:
`src/striatum/daemon_supervisor/progress_loop.py::progress_loop_once`,
called from `src/striatum/daemon.py::run_daemon_foreground` immediately
after `daemon_sweep_once()` (§F4 lines 56-57).

The review-design prompt asks for the lifecycle function to live under
`src/striatum/daemon_pg/` or `src/striatum/supervisor.py`. The synthesis
places it under `src/striatum/daemon_supervisor/` instead. This is
acceptable from ergonomics_dx because `run_daemon_foreground` is a clear
single owner of startup/shutdown and "each daemon sweep owns watcher work
for all currently attached supervisors" (§F4 line 60). The build can
treat `daemon_supervisor/` as the canonical home; flag this only so the
implementer does not waste time relocating it to match the prompt's
literal phrasing.

The sweep design ("no per-supervisor background task to join", §F4 lines
60-61) makes shutdown reasoning trivial: there is nothing to join.

### F5 race and signal hardening — concrete guards ✓

Six race windows, each with a named guard (§F5 lines 66-73):

- Rotated logs: scan newest `*.log` under `scratch_path`; catch
  `FileNotFoundError`/`OSError` while walking.
- First-log latency: `startup_grace_seconds` field on
  `ProgressWatcherConfig`; `waiting_for_log` return before grace.
- SIGTERM: `stopping` predicate checked between repos and between
  supervisors inside `progress_loop_once`.
- Surgical recovery vs heartbeat: shared `progress_advisory_lock`;
  watcher returns `lock_busy`, recovery returns `progress_lock_busy`.
- PID reuse: `pid_start_time` on `SupervisedProgressTarget`, validated
  with `process_start_time(pid)`; returns `process_identity_changed`.
- State drift: reload active lease and job state before heartbeat;
  returns `no_active_lease` / `not_heartbeatable`.

Each return code is a stable string an operator can grep for. No "we add
a lock" placeholders.

### F6 end-to-end tests — exact filenames + smoke hook ✓

§F6 lines 79-83 names two new files with four tests each (mcp e2e and
progress lifecycle) plus extensions to three existing files (§F6 line
94). The smoke hook is explicit: reuse existing `make smoke` and add a
focused pytest invocation to the build handoff (lines 96-102). No new
Makefile target is introduced, which preserves the V1.5 boundary.

### Backward compatibility — explicit ✓

§Backward Compatibility Fixtures lines 106-117 enumerates what must not
change: `RpcEnvelope`, `RpcResponse`, `daemon.hello`, `daemon.welcome`,
`METHODS_ETAG`, MCP tool names, composite names, and the lifecycle
argument shapes. Six existing test files are listed as regression
coverage. The one allowed observable change is named: an allowed
`tools/call` returns the real handler result instead of a stubbed
`ok:true` (line 117).

## Ergonomic Findings (low severity, build-time fixable)

### Finding E1 — Composite-step failure naming is implicit

**Citation:** §F2/F3 line 50: "On failure, the transaction rolls back
and the function returns the existing `{ok:false, status:"refused",
error:{...}}` shape; a best-effort `dogfood.publish_on_behalf_failed`
event may be written after rollback, but it must not claim any
rolled-back step as durable."

A first-time operator calling `dogfood.publish_on_behalf` cannot tell
from the response which composed step caused the rollback. The shape
`error:{...}` is opaque. The review-design prompt asks specifically:
"what does the error message say when a composite step fails partway?"

The synthesis lists internal helpers (`_ack_on_behalf_locked`,
`_publish_artifact_locked`, `_record_verdict_locked`, `_complete_locked`,
§F2/F3 line 44) and lists composition steps (`ack`, `publish_artifact`,
`verdict`, `complete`, §F2/F3 line 50) but does not require the failure
shape to carry a `failing_step` (or equivalent) field naming which of
those four steps refused.

**Suggestion for the build:** require the `{ok:false, ...}` response to
include `failing_step: "<one of ack|publish_artifact|verdict|complete>"`
and surface it in `structuredContent.data.failing_step`. The
`publish_on_behalf_failed` event should carry the same key. This is
discoverable from the response itself, not by reading event rows.

### Finding E2 — Watcher emits events; operator-visible surface not named

**Citation:** §F4 line 62: "Emit metadata-only events
`supervisor.progress_watcher_heartbeat`,
`supervisor.progress_watcher_idle`, and
`supervisor.progress_watcher_lost`."

The synthesis specifies the event names but does not name an operator
affordance (CLI verb, dashboard column, or composite-tool field) that
surfaces watcher state to a first-time user. `striatum dashboard
--run-id` is the obvious candidate; the synthesis is silent on whether
the dashboard renders these statuses.

**Suggestion for the build:** mention in the implementer handoff
(non-normative for the design) that `striatum dashboard` should render
`progress_watcher_lost` distinctly from a generic lease expiry; even
without dashboard changes, the dogfood composite tool descriptions or
help text should mention these event names so an operator grepping logs
knows what to look for.

### Finding E3 — Tool description text not in scope of the synthesis

The synthesis does not include any guidance on the MCP `tools/list`
description text for `dogfood.publish_on_behalf` and
`dogfood.surgical_recovery`. From the ergonomics_dx lens, a first-time
caller of these tools sees only what `tools/list` exposes. The V1.5
boundary explicitly does not change tool names or argument shapes
(§Backward Compatibility), which is correct; the synthesis could still
note that descriptions may be revised to mention atomicity ("all
composed steps commit together or none do") without breaking
compatibility.

**Suggestion for the build:** non-blocking — if descriptions are
revised, do so in the same PR as the dispatch wiring so audit-row +
description ergonomics ship together.

## Why Not "Accept"

Findings E1 and E2 are concrete and directly answer the prompt's
ergonomics_dx question. They are not redesigns; they are small additions
the build can absorb. Recording them as findings (rather than accepting
silently) makes them traceable in the build packet.

## Why Not "Needs Revision"

The dispatch path, atomicity model, watcher hook, race guards, test
filenames, and backward-compat boundary are all pinpointed. No structural
ambiguity remains; the gaps are surface ergonomics that fit inside the
existing F2/F3 and F4 sections without changing the design.
