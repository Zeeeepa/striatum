# RFC 0009: Long-Lived Process Supervision

Status: accepted (reference wrappers under
`.striatum/bin/`; Claude Code wrapper landed
2026-05-08 under dogfood-004 / RFC 0010 V2)
Date: 2026-05-07
Context:
`src/striatum/process_adapter.py`,
`docs/DECISION_LOG.md` (D011, D028, D029, D037),
`docs/TODO.md` item 1 (process adapter expansion)

## Problem

The current `src/striatum/process_adapter.py` runs a single
`subprocess.Popen(...).communicate(...)` call per claimed packet. The child's
lifetime is bounded by that one call: when the configured agent CLI exits,
the adapter exits with it, and the next work packet must spawn a fresh
process from scratch.

Striatum's product framing is the opposite. The README describes the runner
as coordinating terminal AI coding agents such as Codex, Claude Code, and
Gemini CLI. Those CLIs are interactive multi-turn sessions: a single launch
holds context (working set, plan, conversation history) across many turns,
and starting over discards that context. Decision D011 codifies this as a
preference for persistent agent sessions until the assigned role expires,
with `fresh_session_required` reserved for cases where context must be
reset. Decision D029 reinforces that fresh context means a *new role
instantiation*, not "kill and respawn between every packet".

The gap is concrete: today, the runner cannot hold an agent CLI alive across
multiple work packets because the only adapter we ship terminates the child
between packets. The persistent-session model declared by D011 is therefore
unimplemented end-to-end. This RFC proposes the architecture for V2 of the
process adapter that closes this gap without weakening the SQLite-as-source-
of-truth boundary established by D037.

## Goals

- Hold an agent process alive across the lifetime of a Striatum session, so
  that a session that received packet N retains the same OS process when it
  later receives packet N+1.
- Deliver multiple work packets to that running process via a structured
  channel (stdin or a side channel such as a named pipe), one packet per
  agent turn.
- Observe lifecycle events at the runner boundary: `started`, `output
  milestone`, `exited`, plus operator-driven `terminated`.
- Allow operators to terminate supervised processes cleanly through a CLI
  command, with the same authoritative state transitions used by the rest
  of the runner.
- Keep the contract provider-portable: the supervisor must not require a
  PTY, must not depend on terminal output for state, and must not assume a
  particular tmux or window-manager environment.

## Non-Goals

- No transcript capture by default. Per D028, durable artifacts are curated
  outputs (decisions, prompts, findings, syntheses, markers, handoffs); raw
  transcripts and stdout streams remain off unless an explicit opt-in is
  introduced under a separate decision.
- No hidden auto-decision based on parsing terminal output. Per D037,
  process and tmux adapters are launch boundaries only and SQLite remains
  authoritative; the supervisor must never derive verdicts, completion, or
  job state from agent stdout.
- No PTY-only requirement. A PTY may be a future option, but the V2
  contract must be expressible against pipe stdio so headless lanes,
  CI lanes, and provider-portable wrappers continue to work.
- No daemon or background scheduler outside the existing CLI. `striatum
  supervise *` commands are single-shot CLI invocations, mirroring the
  rest of the runner.
- No automatic restart on crash. Recovery surfaces a `lost` supervisor and
  reuses the existing stale-lease path; restart, if any, is operator-driven.

## Proposal

### CLI surface

Add a `striatum supervise` command group with four subcommands. Each is
single-shot and idempotent in the same sense as the existing CLI: every
invocation is a state transition recorded in SQLite.

1. `striatum supervise start --session-id <id>`
   - Looks up the session, its lane, and the lane's `command` array.
   - Forks the configured command with the existing `process_adapter`
     environment (`STRIATUM_RUN_ID`, `STRIATUM_SESSION_ID`, etc.).
   - Records a `process_executions` row with the new `supervisor_state =
     'attached'`, plus `pid`, `stdin_pipe_path`, and `started_at`.
   - Detaches: writes pid and pipe path to state, prints them as JSON, and
     returns immediately. The child process keeps running under the user's
     session manager (shell, tmux, systemd-user, etc.) — Striatum does not
     own a long-running parent.
   - Inserts a `process.supervisor_started` event.

2. `striatum supervise send --session-id <id> --packet-id <id>`
   - Resolves the supervised process for the session.
   - Looks up the stored `work_packets.packet_json`.
   - Writes one JSON object plus a record separator (newline-delimited
     JSON, NDJSON) to the supervisor's stdin or its named pipe.
   - Bumps `process_executions.heartbeat_at` and inserts a
     `process.packet_delivered` event with the packet id and lease id.
   - Does *not* parse stdout; the agent's reaction to the packet is
     observed only via subsequent `striatum ack`, `publish-artifact`,
     `verdict`, and `complete` calls.

3. `striatum supervise stop --session-id <id> --reason <text>`
   - Sends `SIGTERM` to the recorded pid (if alive), waits a configurable
     grace window, then `SIGKILL` if the process is still present.
   - Updates `process_executions.supervisor_state = 'detached'`,
     `state = 'exited'`, `ended_at`, and `exit_code`.
   - Releases any active lease owned by this session via the existing
     stale-lease path so downstream queue messages become claimable again.
   - Inserts a `process.supervisor_stopped` event with the reason.

4. `striatum supervise status --session-id <id>`
   - Reads the `process_executions` row for the session.
   - Probes liveness using `os.kill(pid, 0)`; `ProcessLookupError` (or
     `PermissionError` for stale pids reused by another user) means the
     process is gone.
   - Reports `{pid, supervisor_state, last_heartbeat_at, alive: bool}` as
     JSON. Status itself never mutates state; it is the read-only view used
     by `doctor` and operator inspection.

### Schema additions

Add these columns to `process_executions` via a new migration in
`src/striatum/migrations.py`:

- `supervisor_state TEXT` with `CHECK IN ('attached', 'detached', 'lost')`
  and a default of `'attached'`. `attached` means Striatum still expects
  the pid to be alive; `detached` means a clean stop or completion;
  `lost` means liveness probing has failed and the row should be treated
  the same way as a stale lease.
- `stdin_pipe_path TEXT NULL` for supervised processes that use a named
  pipe rather than direct stdin. Direct stdin will be the V2 default;
  this column is for the option discussed under Open Questions.
- `heartbeat_at TEXT NULL` updated on every successful `supervise send`
  and any optional in-band heartbeat (see Open Questions).

The migration must also relax the existing `state` `CHECK` constraint if
the supervisor introduces new values, or add a separate `supervisor_state`
column orthogonal to `state` (preferred). Either way: forward-only, no
edits to `schema.py`'s `SCHEMA_SQL`.

### Lifecycle integration

The runner's `claim-next` semantics are unchanged: it still selects the
next eligible queue message, creates a lease, builds the work packet, and
records the `work_packets` row. What changes is what happens *after* a
packet is built for a session that already has a supervised process.

- If `process_executions` has an `attached` row for the session, the
  runner does NOT spawn a new process. Instead the runner emits the
  packet via `striatum supervise send` (in-process call into the same
  module path) and records the resulting `process.packet_delivered`
  event.
- If there is no attached supervisor, behavior matches V1: a one-shot
  `process_adapter` run for that packet. The two modes coexist so
  workflows that do not need persistence remain unaffected.
- `complete`, `verdict`, and `release` continue to be the authoritative
  state transitions. They may optionally call `supervise stop` when the
  workflow declares the session should exit at job boundaries (e.g., the
  job is `fresh_session_required`); the supervisor is otherwise left
  alive until role expiry or operator stop.

### Recovery

Liveness is checked lazily on every `supervise send`, `supervise status`,
and any CLI mutation that touches a session with an `attached` row.

- `os.kill(pid, 0)` raising `ProcessLookupError` ⇒ the supervisor is set
  to `lost`, the active lease (if any) is released through the existing
  `expire_leases` path, and the queue message returns to `pending`.
- `os.kill(pid, 0)` raising `PermissionError` ⇒ same treatment as `lost`
  conservatively, since the pid has been reused by another user.
- A `process.supervisor_lost` event records the recovery so `doctor` and
  `why` surfaces can explain the transition without reading agent
  output.

## Acceptance Criteria

A passing implementation must demonstrate:

- `striatum supervise start --session-id <id>` returns a JSON payload
  with `pid`, writes a `process_executions` row whose
  `supervisor_state = 'attached'`, and returns before the child exits.
- After `supervise start`, `supervise status` reports `alive = true` for
  a process that is still running and `alive = false` immediately after
  it exits, without requiring a separate poll.
- `supervise send --packet-id <id>` writes a JSON document plus newline
  to the recorded stdin/pipe channel, updates `heartbeat_at`, and
  inserts a `process.packet_delivered` event referencing the packet id
  and lease id.
- A `claim-next` issued for a session with an `attached` supervisor does
  NOT spawn a new process; instead, the runner emits the packet through
  the supervisor send path and the next agent turn reuses the same pid.
- `supervise stop` transitions the row to `supervisor_state = 'detached'`,
  `state = 'exited'`, releases the active lease through the same path
  used by stale leases, and inserts a `process.supervisor_stopped`
  event.
- A killed-from-outside pid is reflected as `supervisor_state = 'lost'`
  on the next mutation that observes the session, with the lease
  released and a `process.supervisor_lost` event recorded.
- The supervisor never reads agent stdout to derive workflow state; all
  state transitions remain CLI-driven (preserves D037).
- Transcripts are not captured by the supervisor (preserves D028); only
  packet delivery and lifecycle events are recorded.

## Open Questions

- **PTY versus pipe stdin.** Some CLIs (notably interactive REPLs)
  buffer or refuse to read from a non-tty stdin. Should V2 ship with
  pipe stdin and a follow-up RFC for PTY support, or should PTY be
  conditional on lane configuration from day one?
- **Mid-turn packet delivery.** What should `supervise send` do when the
  agent is in the middle of a turn (still emitting tokens for the prior
  packet)? Options: queue the packet at the runner side and flush on
  next `claim-next`; require the agent CLI to acknowledge before the
  next send; reject and surface a blocker. Each has UX trade-offs.
- **Foreground tmux pane attachment.** Should the supervisor support
  attaching to a long-lived tmux pane the operator already opened
  (read pid from the pane's foreground process), or always own the
  fork? Tmux attachment is convenient for human-in-the-loop work but
  conflates Striatum state with terminal state in ways D037 explicitly
  avoids.
- **Heartbeat protocol.** Should agent CLIs implement a simple heartbeat
  ping (write a JSON `{type:"ready"}` line on stdout) that the
  supervisor listens for to bump `heartbeat_at`? This would catch
  hung-but-alive processes that `os.kill(pid, 0)` cannot detect, but
  it requires opt-in agent cooperation and partially walks back the
  "do not parse stdout for state" rule from D037 — even if the parse
  is restricted to a structured liveness ping. Decide before
  implementation.
- **Cross-platform pipe paths.** On Windows, named pipes use a different
  namespace than POSIX FIFOs. If named-pipe stdin is adopted (rather
  than direct stdin), the schema needs to encode platform plus path,
  or restrict V2 to POSIX until a Windows port is scoped.
- **Authority on stop.** Should `supervise stop` be allowed when the
  session still owns an active lease, or must the operator
  `release --requeue` first? Forcing `release` first keeps state
  transitions explicit; allowing inline release reduces operator
  steps. Pick one and document.

## Implementation Notes

The implementation that landed here matches the proposal with one schema
refinement: rather than overloading `process_executions` with a
`supervisor_state` column, the supervisor flow uses a separate
`process_supervisors` table introduced as migration version 4. Single-shot
`adapter run` and supervised flows therefore keep distinct rows and event
streams — `process.*` events stay tied to single-shot launches, and
`supervisor.*` events describe long-lived sessions. Both flows coexist; the
existing `striatum adapter run` command is unchanged.

Schema (migration v4) for `process_supervisors`:

```sql
CREATE TABLE process_supervisors (
  supervisor_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES runs(run_id),
  session_id TEXT NOT NULL REFERENCES sessions(session_id),
  adapter TEXT NOT NULL,
  command_json TEXT NOT NULL,
  cwd TEXT NOT NULL,
  scratch_path TEXT NOT NULL,
  stdin_pipe_path TEXT,
  pid INTEGER,
  state TEXT NOT NULL CHECK (state IN ('starting','attached','detached','lost','stopped')),
  started_at TEXT NOT NULL,
  heartbeat_at TEXT,
  ended_at TEXT,
  stop_reason TEXT
);
CREATE UNIQUE INDEX uq_active_supervisor_per_session
  ON process_supervisors(session_id) WHERE state IN ('starting','attached','detached');
CREATE INDEX idx_process_supervisors_run
  ON process_supervisors(run_id, state);
```

CLI surface implemented in `src/striatum/supervisor.py` and wired into
`src/striatum/cli.py`:

- `striatum supervise start --session-id <id>` — validates the session is
  active and that its lane uses the `process` adapter, refuses if a
  supervisor row already exists in `('starting','attached','detached')`,
  creates `.striatum/scratch/<supervisor_id>/stdin.pipe` via `os.mkfifo`,
  forks the lane command with `start_new_session=True`, redirects
  stdout/stderr to `DEVNULL` (no transcripts, per D028), and transitions the
  row to `attached` once the pid is alive.
- `striatum supervise send --session-id <id> --packet-id <id>` — looks up
  the stored packet, writes its `packet_json` plus a trailing newline to
  the named pipe, refreshes `heartbeat_at`, and records a
  `supervisor.packet_delivered` event with the byte count.
- `striatum supervise stop --session-id <id> --reason <text>` — sends
  `SIGTERM`, waits up to 5 seconds, falls back to `SIGKILL` if needed,
  removes the FIFO, and transitions the row to `stopped`.
- `striatum supervise status --session-id <id>` — probes liveness with
  `os.kill(pid, 0)` and transitions an active row to `lost` (with a
  `supervisor.lost` event) when the pid is gone before returning the row
  plus a `liveness` field.
- `striatum supervise list --run-id <id> [--state <state>]` — lists rows
  for a run, optionally filtered by state.

`expire_leases` (in `src/striatum/db.py`) calls a small recovery hook that
marks any `attached` supervisor for the expiring lease's session as `lost`
and emits `supervisor.lease_expired_with_supervisor`. The OS process is not
auto-killed: operator inspection is required, mirroring the stale-lease
policy for repo-write work (D036).

The pipe transport opens the FIFO `O_RDWR` in the parent before forking,
clears `O_NONBLOCK` on the inherited fd so the child sees normal blocking
stdin semantics, and passes the fd to `subprocess.Popen` as `stdin`. This
keeps a kernel-level "has writer" reference for the duration of the child
so subsequent `supervise send` invocations can attach as writers without
the child seeing premature EOF when no other writer is currently connected.

Doctor checks in `cli.py:doctor()` flag two operator-actionable conditions:
supervisors in `('starting','attached','detached')` whose pid is gone (the
process was killed externally or the host was rebooted), and `attached`
supervisors whose `stdin_pipe_path` no longer exists on disk.

The Open Questions above remain open for follow-up RFCs: the V1 transport
ships pipe stdin (no PTY), `supervise send` is fire-and-forget (no mid-turn
guard), tmux attachment is out of scope, no heartbeat ping protocol is
required, Windows pipe namespaces are not handled, and `supervise stop`
does not auto-release leases — operators must call `release` separately.

### Supervised-aware `claim-next` (auto-delivery)

Current implementation note (2026-06-04): this section records the original
Python/SQLite RFC 0009 design. The current Go/PostgreSQL implementation does
not make `claim-next` write to a supervisor. Non-agent-loop supervised lanes
use `supervise start` push auto-dispatch instead; see
[`docs/reference/spec.md`](../reference/spec.md#process-supervision).

`claim_next` (in `src/striatum/db.py`) closes the lifecycle integration loop
by routing freshly built work packets through any `attached` supervisor for
the same session. After inserting the `work_packets` row and the
`queue.claimed` event, and still inside the same write transaction, the
runner calls
`striatum.supervisor.deliver_packet_to_attached_supervisor(...)` which:

- Looks for a `process_supervisors` row in state `attached` for the
  claiming session (the partial unique index already enforces at most one).
- Writes the stored `packet_json` (plus a trailing newline) to that row's
  `stdin_pipe_path`, refreshes `heartbeat_at`, and records a
  `supervisor.packet_delivered` event tagged with
  `via=claim_next_auto_delivery` and the byte count.
- If the named pipe is missing on disk or the write fails (broken pipe,
  unrecoverable `OSError`), transitions the row to `lost`, records a
  `supervisor.lost` event with `phase=claim_next_auto_delivery`, and
  surfaces a `supervisor_delivery: {supervisor_id, error:
  "stdin_pipe_missing"}` envelope in the claim response so the caller can
  decide whether to restart the supervisor and retry via
  `supervise send`. The packet itself is still committed and returned.

The CLI envelope from `claim-next` therefore gains an optional
`supervisor_delivery` field. The `packet` field is unchanged so callers
that ignore the supervisor flow continue to work; only sessions with an
attached supervisor see the new field. Sessions without a supervisor see
exactly the previous response shape.

Because the auto-delivery runs on the same connection inside the existing
`BEGIN IMMEDIATE` transaction, packet creation and the supervisor
bookkeeping (heartbeat, delivery event, or lost transition) are atomic
with the queue claim — there is no window in which a packet exists in
SQLite without its supervisor side-effects also being recorded.
