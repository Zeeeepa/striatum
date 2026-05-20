---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/issues/28/SPEC.md", "docs/rfcs/0049-interactive-claude-lane-mcp-control-plane.md", "docs/ROADMAP.md", "docs/TODO.md", "docs/DECISION_LOG.md", "docs/POSTGRES_TRANSITION.md", "docs/SPEC.md", "docs/INDEX.md", "AGENTS.md", "go/pkg/workflowtemplates/catalog.json", "go/pkg/workflowtemplates/catalog.go", "go/pkg/workflowauthoring/workflow.go", "go/pkg/supervisor/helper.go", "go/pkg/supervisor/pty.go", "go/pkg/mcp", "go/pkg/rpc", "src/striatum/daemon_pg/sql"]
---

author: triager-unknown-model-001

# GH #28 -- Scope

Bound scope for GH #28, "Unified Interactive Harness, Tmux Supervisor, and
Escalation Inbox." The implementation should close the concrete issue
acceptance bullets while keeping Striatum's current product boundary intact:
daemon-owned PostgreSQL remains authoritative live state, `.striatum/` remains
scratch only, provider CLIs remain process-bound adapters, and no durable
transcript capture is introduced.

RFC 0049 is shelved by D106. GH #28 may reuse the generic `await_packet`
control-plane shape from that RFC, but it must not reintroduce Claude-specific
billing assumptions, make interactive Claude the default lifecycle, or depend
on undocumented provider behavior.

## Chosen Approach

### Go parity and harness updates

Make the Go workflow authoring surface match the current workflow fixtures:

- Add `agy_default` to the embedded Go workflow template catalog.
- Accept `agy` and `antigravity` in the Go harness profile tool-family
  validator.
- Validate optional lane `model` values in `go/pkg/workflowauthoring`: if the
  key is present, it must be a non-empty string.

This is a parity change only. Do not widen Python's accepted tool-family set in
this issue unless a failing parity test proves Python already expects the same
values from the shared catalog.

### Tmux-based command spawning and stream capture

Replace the Go helper's direct `creack/pty` launch path with a small tmux
backend for supervised interactive commands. The supervisor should create a
headless session named from sanitized identifiers:

```text
striatum-{run_id}-{lane_id}
```

The launch spec currently does not carry `run_id` or `lane_id`, so add them to
the helper launch JSON/spec using existing supervisor/session metadata. Fall
back to `supervisor_id` only in tests that construct old minimal specs.

Use `tmux new-session -d -s <name> -c <working_dir> -- <command...>` and wire
I/O through tmux primitives:

- Send packets with `tmux send-keys -t <name> -- <frame> Enter` or an
  equivalent stdin file/pipe path that preserves newline-delimited packets.
- Capture progress with `tmux pipe-pane -o -t <name> 'cat >> <progress_path>'`
  and tail/read that file from the helper so `HelperEventProgress` and
  supervisor heartbeat monitoring continue to see byte growth.
- Detect process exit by polling `tmux has-session` plus pane status, then emit
  the same `agent_exited` event shape the helper emits today.
- On context cancellation or stop, terminate the tmux session with
  `tmux kill-session -t <name>` and treat "session already gone" as a clean
  cleanup condition.

The captured stream is operational progress data, not a curated artifact. It
must stay in scratch/log paths and must not be published as a transcript.

### Long-polling `await_packet`

Add production RPC method `work.await_packet` and expose it through the Go MCP
tool list like other claim-capability methods. Implement it as long-polling
claim-next, not a second scheduler:

- Required params: `session_id`.
- Optional params: `timeout_ms` with a conservative maximum, and
  `lease_seconds` matching `work.claim_next`.
- Capability: `claim`.
- Scope: single repository.
- Return shape: same claimed packet envelope as `work.claim_next`, or
  `{"status": "timeout"}` when the long poll expires without work.

Internally, loop on the existing claim logic with a short sleep/jitter and
honor `context.Context` cancellation. Do not hold a database transaction or
connection while sleeping. Each poll should open its own short transaction,
reuse the same claim code path, and return immediately once a packet is
claimed. This preserves lease semantics and avoids connection pool exhaustion.

Add a Python-side `src/striatum/skills/mcp_loop_wrapper.py` wrapper as a thin
operator utility for non-native CLIs. It should connect to the daemon MCP
socket, call `work.await_packet`, hand packet JSON to the child process over
stdin/stdout-compatible framing, and repeat until interrupted or a shutdown
response appears. It must not mutate live state except through MCP/RPC calls.

### Stateful escalation inbox

The current escalation surface is a projection over `blockers` plus linked
escalation artifacts. GH #28 asks for a stateful inbox, so add a dedicated
PostgreSQL table and keep the existing projection as compatibility input:

- Add migration `0011_escalation_inbox.sql` in both Python and Go migration
  sets.
- Create `striatumd.escalation_inbox` keyed by `(repository_id,
  escalation_id)`, with foreign keys to `blockers` and nullable artifact links
  to `artifacts`.
- Track state with a closed check constraint: `pending`, `viewed`, `resolved`.
- Store `run_id`, optional `job_id`, optional `session_id`, `blocker_kind`,
  `severity`, `created_at`, `viewed_at`, `resolved_at`, optional
  `decision_artifact_id`, optional `resolution_note`, and `payload_json`.
- Populate/update from daemon code paths, not ad hoc SQL clients:
  `work.block` for escalation-class blockers, escalation artifact publish for
  artifact links, `escalation.show` or an explicit mark-viewed helper for
  viewed state, and `escalation.resolve` / `checkpoint.resolve` for resolved
  state.

Prefer application-level writes in existing Go/Python handlers over database
triggers unless tests show a missed mutation path. Triggers make backfills and
fixture setup harder to reason about; handler updates keep the authority
boundary visible in source.

## Files In Scope

- `go/pkg/workflowtemplates/catalog.json`
- `go/pkg/workflowtemplates/catalog.go`
- `go/pkg/workflowtemplates/*_test.go`
- `go/pkg/workflowauthoring/workflow.go`
- `go/pkg/workflowauthoring/*_test.go`
- `go/pkg/supervisor/helper.go`
- `go/pkg/supervisor/pty.go` or a new `go/pkg/supervisor/tmux.go`
- `go/pkg/supervisor/*_test.go`
- `go/pkg/mutations/claim.go`
- `go/pkg/mutations/mutations.go`
- `go/pkg/mutations/*await*_test.go` or focused claim lifecycle tests
- `go/pkg/mcp/*.go` and `go/pkg/mcp/*_test.go`
- `go/pkg/rpc/registry_methods.go` only via generated contract updates
- `contracts/daemon_methods.json`
- `scripts/generate_go_rpc_registry.py` only if generation support needs a
  narrow update
- `go/pkg/reads/detail.go`
- `go/pkg/reads/escalation_resolve.go`
- `go/pkg/reads/*escalation*_test.go`
- `src/striatum/daemon_pg/sql/0011_escalation_inbox.sql`
- `go/pkg/db/sql/0011_escalation_inbox.sql`
- `src/striatum/daemon_pg/migrations.py`
- `go/pkg/db/migrations.go`
- Python escalation handlers under `src/striatum/daemon_pg/handlers/`
- Python artifact publish linkage under
  `src/striatum/daemon_pg/handlers/workflow_loop/artifact_publish.py`
- `src/striatum/skills/mcp_loop_wrapper.py`
- Focused tests under `tests/`, especially daemon PG migration, escalation
  inbox, CLI/MCP wrapper, and contract-registry tests
- `docs/architecture/COMMAND_AUTHORITY_MATRIX.md` if `work.await_packet`
  enters the production daemon contract
- Generated `docs/architecture/DAEMON_METHOD_TABLES.md` if the contract table
  generator requires refreshed output
- `docs/issues/28/build/HANDOFF.md` for the later implementation handoff

## Files Out Of Scope

- Historical migrations `0001` through `0010`; add forward migration 0011
  only.
- `.striatum/`, `.venv/`, caches, transcripts, private diagnostics, and build
  output.
- Hosted services, cloud APIs, telemetry, external persistence, Slack/remote
  serving, or provider SDK integration.
- Claude-specific billing behavior, lifecycle defaults, or documentation that
  claims RFC 0049 is active again.
- Automatic commits, sealed apply behavior, blob storage, corpus export, run
  archive, cross-repo workflow semantics, and optional Git/PR authority.
- Broad web UI redesign. Existing inbox/CLI read surfaces may be adjusted only
  enough to consume the new stateful inbox.
- Legacy SQLite compatibility modules except where tests prove a current guard
  must assert they remain unused.
- Rewriting the Python CLI/web architecture or replacing daemon RPC routing
  wholesale.

## Acceptance Checklist

Each item maps to a bullet in `docs/issues/28/SPEC.md`.

1. **GH28-1 (`agy_default` catalog fragment).** `go/pkg/workflowtemplates/catalog.json`
   contains an `agy_default` harness profile fragment with tool family `agy`
   and a conservative no-follow-up native-delegation instruction.
2. **GH28-2 (Go tool families).** `go/pkg/workflowtemplates/catalog.go`
   accepts `agy` and `antigravity` as valid harness fragment tool families and
   rejects unknown families.
3. **GH28-3 (lane model validation).** `go/pkg/workflowauthoring.Validate`
   rejects a lane `model` property when present but blank or non-string.
4. **GH28-4 (Python and Go tests).** The changed Python and Go suites pass,
   including contract generation/guardrail tests affected by new RPC methods.
5. **GH28-5 (tmux session spawn).** The Go supervisor launches supervised
   commands inside headless tmux sessions named `striatum-{run_id}-{lane_id}`
   with safe sanitization and deterministic cleanup.
6. **GH28-6 (tmux stream capture).** The supervisor captures pane output into
   operational progress logs so heartbeat/progress behavior continues without
   publishing transcripts.
7. **GH28-7 (tmux stalls/exits).** Heartbeat stalls, child exits, and stop
   requests are reported through the same supervisor event model as before.
8. **GH28-8 (`work.await_packet`).** The Go daemon exposes
   `work.await_packet(session_id)` through RPC/MCP with claim capability and
   single-repo scope.
9. **GH28-9 (long-poll semantics and wrapper).** `work.await_packet`
   long-polls with timeout/context cancellation and returns a normal work
   packet when work becomes claimable; `src/striatum/skills/mcp_loop_wrapper.py`
   can drive non-native CLIs through that loop.
10. **GH28-10 (inbox migration).** Migration 0011 creates
   `striatumd.escalation_inbox` with pending/viewed/resolved state and artifact
   links.
11. **GH28-11 (inbox models and RPC support).** Go and Python models/handlers
   populate and update the inbox when escalation blockers/artifacts are
   created, viewed, or resolved.

## Verification Commands

Run at minimum:

```bash
make lint
make typecheck
make test
make smoke
go test ./go/pkg/...
```

Targeted checks should include:

```bash
pytest tests/test_artifact_schemas.py tests/daemon_pg
go test ./go/pkg/workflowtemplates ./go/pkg/workflowauthoring ./go/pkg/supervisor ./go/pkg/mutations ./go/pkg/mcp ./go/pkg/reads ./go/pkg/rpc
python scripts/generate_daemon_method_tables.py --check
python scripts/generate_go_rpc_registry.py --check
```

Manual tmux verification:

```bash
tmux ls | grep 'striatum-'
tmux attach -t striatum-<run_id>-<lane_id>
tmux capture-pane -pt striatum-<run_id>-<lane_id> -S -80
striatum supervise status --session-id <session_id> --json
striatum supervise stop --session-id <session_id> --json
```

Manual `await_packet` verification should start a registered session with no
claimable work, call `work.await_packet` with a short timeout and confirm a
timeout response, then queue/start a claimable job and confirm the same method
returns a normal packet with a fresh lease.

Manual database verification for migration 0011:

```sql
SELECT column_name, data_type
  FROM information_schema.columns
 WHERE table_schema = 'striatumd'
   AND table_name = 'escalation_inbox'
 ORDER BY ordinal_position;

SELECT escalation_id, state, blocker_id, escalation_artifact_id,
       decision_artifact_id, viewed_at, resolved_at
  FROM striatumd.escalation_inbox
 ORDER BY created_at DESC
 LIMIT 20;
```

## Risks and Mitigations

- **Tmux capture buffers can truncate history.** Use `pipe-pane` to append to a
  scratch progress file for heartbeat/progress and reserve `capture-pane` for
  operator inspection only.
- **Tmux may be missing on an operator machine.** Fail clearly at
  `supervise.start` with an actionable error. Do not silently fall back to a
  different authority path unless a workflow/lane flag explicitly allows it.
- **Long-polling can exhaust the connection pool.** Never sleep while holding
  a transaction or DB connection; poll through short claim attempts and honor
  context cancellation.
- **Long-polling can create scheduler latency or thundering herds.** Bound
  `timeout_ms`, add small jitter, and return `timeout` rather than requiring
  infinite waits.
- **Fresh-session semantics can be weakened by interactive loops.** Preserve
  existing `fresh_session_required` checks from `work.claim_next`; do not reuse
  a session that has already received a packet for fresh-required work.
- **Stateful inbox can drift from blockers.** Update the inbox in the same
  transaction as blocker creation/resolution and artifact linkage, and add
  idempotent backfill tests.
- **Transcript policy can regress.** Keep tmux output in operational scratch
  and progress events only; curated artifacts remain explicit Markdown outputs.
- **Generated contract files can drift.** Add `work.await_packet` to
  `contracts/daemon_methods.json`, regenerate Go registry/docs, and keep the
  authority guardrail tests green.
