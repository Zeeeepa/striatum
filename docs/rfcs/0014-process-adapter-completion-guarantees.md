# RFC 0014: Process Adapter Completion Guarantees

Status: proposed
Date: 2026-05-08
Context:
[Issue #1](https://github.com/halbritt/striatum/issues/1) (Process
adapters can exit or hang without completing claimed jobs),
`docs/DECISION_LOG.md` (D028, D036, D055),
`docs/SPEC.md` § "Adapter Boundary", § "Process Supervision",
`src/striatum/process_adapter.py`,
`src/striatum/cli/recovery.py`,
RFC 0009 (long-lived supervision)

## Problem

Issue #1 documented an engram dogfood run where all three configured
process-adapter reviewer lanes failed silently in different ways:

| Lane | Process outcome | Job result |
|---|---|---|
| `claude --model opus -p` | hung; killed manually | no artifact, no verdict |
| `codex exec --model gpt-5.5 -` | exited `0` | no artifact, no verdict |
| `gemini --model gemini-3.1-pro-preview` | exited `1` | no artifact, no verdict |

The runner state machine kept working — the operator manually published
artifacts and verdicts to drive the workflow forward. The
`process_executions` row for the killed Claude process stayed
`state="running"` because the manual kill bypassed Striatum's
bookkeeping.

`run_process_adapter` in `src/striatum/process_adapter.py:52` does:

1. `subprocess.Popen(command, …)` with `stdin=PIPE`, stdout/stderr
   `DEVNULL` (the transcripts-off default).
2. `process.communicate(payload)` — blocks until the child exits,
   with **no timeout**.
3. `mark_process_exited(conn, …, exit_code=process.returncode)`
   records the exit.
4. Returns.

What it does **not** do:

- Validate that required `expected_artifacts` were published.
- Validate that a `type: "review"` job recorded a verdict.
- Enforce a timeout when the child hangs.
- Detect when the process was killed externally (only the in-process
  flow updates `process_executions.state`).
- Surface a privacy-safe diagnostic envelope when the child exits
  without producing the required outputs.

The result is a class of failure where the workflow runner state
machine *itself* is healthy, but the bridge to the agent CLI is
broken in a way operators have to debug by reading SQLite tables.

This is not a transcript-capture problem. The current `transcripts =
"off"` constraint (D028) is correct. The missing piece is determinism
around required outputs and process liveness.

## Goals

- After `striatum adapter run` completes, the workflow state must
  unambiguously reflect one of: success-with-required-outputs,
  blocked-with-missing-outputs, blocked-with-nonzero-exit,
  blocked-with-timeout, or `lost` (process gone, bookkeeping
  reconciled).
- A privacy-safe diagnostic envelope (command, exit code, duration,
  missing artifact paths, recovery commands) is preserved on every
  failure path. The envelope is *not* a transcript; it does not
  capture stdout, stderr, or model output.
- A runner-side timeout is configurable per `adapter run`
  invocation; default behaviour stays unbounded for backwards
  compatibility, but the CLI surfaces the option prominently and
  reference workflows opt in.
- Externally-killed processes are reconciled through a new recovery
  command and surfaced by `doctor`.
- No change to the no-transcripts product boundary or D020 (no
  hosted services).

## Non-Goals

- Capturing or persisting agent stdout/stderr. D028 stands.
- Replacing RFC 0009 long-lived supervision. This RFC targets the
  one-shot `adapter run` path; the supervised long-lived path
  already has its own liveness model (`process_supervisors`,
  `supervisor.lost`).
- Heuristic agent-output parsing. The runner does not interpret what
  the model said; it only inspects whether the contract artifacts
  and verdicts arrived.
- Auto-retry on missing outputs. Operators decide whether to
  recover; the runner only surfaces deterministic blocked states.
- Cross-process synchronization beyond a single repo's SQLite.

## Proposal

Three landable changes, intentionally scoped so each can ship in its
own PR.

### 1. Post-exit output validation

After `process.communicate` returns, before `mark_process_exited`
finalizes the row, perform a structured check inside the existing
`run_process_adapter` transaction:

```text
required_artifacts = {a.path for a in job.expected_artifacts if a.required}
published_artifacts = {row.path for row in artifacts where job_id == job.job_id}
missing_artifacts = required_artifacts - published_artifacts
needs_verdict = (job.type == "review")
verdict_present = exists(verdicts where job_id == job.job_id)
```

Decision matrix:

| Exit code | Missing artifacts | Needs verdict + missing | Action |
|---|---|---|---|
| `0` | empty | no | success — keep current behaviour |
| `0` | non-empty | — | block job: `process_outputs_missing` |
| `0` | empty | yes | block job: `process_review_verdict_missing` |
| non-zero | — | — | block job: `process_exit_nonzero` |

"Block job" means: insert a blocker row pointing at the job, transition
the job state from `running` to `blocked`, leave the lease in place
until lease expiry or explicit operator recovery, and emit a
`process_adapter.outputs_missing` event with the diagnostic envelope:

```json
{
  "process_id": "proc_...",
  "command": ["claude", "--model", "opus", "-p"],
  "exit_code": 0,
  "duration_seconds": 142.7,
  "missing_artifact_paths": ["docs/.../REVIEW.md"],
  "review_verdict_missing": true,
  "recovery_commands": [
    "striatum publish-artifact --session-id ... --kind finding ...",
    "striatum verdict --session-id ... --verdict ...",
    "striatum recovery requeue-stale --run-id ..."
  ]
}
```

The envelope is structured JSON; it never includes child stdout or
stderr (those remain DEVNULL'd) or model output. The child's
`pid`, `exit_code`, and the `command[0]` are the only process-level
identifiers in the envelope, plus the duration the runner already
measures.

### 2. Configurable timeout / heartbeat for `adapter run`

Add `--timeout-seconds <n>` to `striatum adapter run`. When set:

- `process.communicate(payload, timeout=n)` replaces the unbounded
  call.
- On `subprocess.TimeoutExpired`:
  - SIGTERM the child and `wait(timeout=5)`; SIGKILL if still alive.
  - Mark `process_executions.state = 'timed_out'` with an exit code
    of `None` (or a sentinel; see open question).
  - Block the job with `process_timeout_exceeded`, envelope shape
    identical to (1) plus the timeout value.

Workflows can declare a per-lane default via
`lanes.<id>.adapter_timeout_seconds` so per-job timeouts are
optional. Validation accepts the field as a positive integer.

The default remains unbounded — the field is opt-in to preserve
behaviour for any caller who relies on long-running adapters today.
Reference workflows under `examples/` and `docs/dogfood/` set a
30-minute (1800s) default during the same change as a sane starting
point that operators can override.

### 3. Liveness reconciliation

Add `striatum recovery process-reconcile --run-id <id>` mirroring the
existing `recovery requeue-stale` pattern (D036). Behaviour:

- Walk `process_executions` rows whose `state` is `running`.
- For each, `os.kill(pid, 0)`.
  - Process alive → leave the row alone; surface in JSON output as
    `still_running` with the pid and elapsed time.
  - Process gone → transition row to `state = 'lost'`, set
    `lost_at = utc_now()`, emit
    `process_adapter.process_lost_with_held_lease` if the underlying
    job is still in `running`.
- Re-run the post-exit validation from (1) against any
  newly-`lost` row to either close out a job that the agent did
  publish before crashing (rare but possible) or block it with
  `process_lost_with_outputs_missing`.

`striatum doctor` adds two checks:

- `process_running_but_pid_gone` — surfaces stale `running` rows
  whose pid no longer exists.
- `process_running_with_expired_lease` — surfaces rows where the
  lease has expired but `process_executions.state` is still
  `running`.

`striatum status --run-id` adds a `process_health` summary alongside
the existing `human_checkpoints` and `next_actions` keys.

### Diagnostic envelope storage

The envelope from (1) and (2) is recorded as the blocker row's
`payload_json` so:

- `striatum why <job_id>` surfaces it.
- `striatum dashboard` and the (proposed) RFC 0013 web UI render it
  alongside the blocker.
- Evidence export under `striatum evidence export` includes it in
  the durable JSON dump.

The envelope is the **only** new state this RFC adds. No new SQLite
tables; the `events` table and the `blockers` table absorb the
diagnostic shape.

## Acceptance Criteria

- After a process adapter exit with a missing required artifact, the
  job is in state `blocked`, a blocker row exists with the
  `process_outputs_missing` reason and the diagnostic envelope, and
  `striatum status --run-id <id>` surfaces the blocker.
- After a process adapter exit with a missing review verdict, same
  shape with reason `process_review_verdict_missing`.
- After a non-zero exit, same shape with reason
  `process_exit_nonzero`.
- `--timeout-seconds <n>` SIGTERMs the child after `n` seconds,
  marks `process_executions.state='timed_out'`, and blocks the job
  with `process_timeout_exceeded`.
- `recovery process-reconcile --run-id <id>` transitions
  externally-killed rows from `running` to `lost`, runs the same
  output validation, and exits 0 with a JSON summary.
- `doctor` flags `process_running_but_pid_gone` and
  `process_running_with_expired_lease`.
- The diagnostic envelope contains `process_id`, `command`,
  `exit_code`, `duration_seconds`, `missing_artifact_paths`,
  `review_verdict_missing`, and `recovery_commands`. It contains
  **no** child stdout, stderr, or model output.
- `tests/test_process_adapter.py` (new) covers each of the four
  failure modes plus the happy path; existing `test_supervise.py`
  is unchanged because RFC 0014 only touches the one-shot path.
- Reproduction matches the issue: a fixture workflow with a child
  command that exits 0 without writing the artifact reaches the
  blocked-with-`process_outputs_missing` state without operator
  intervention.

## Open Questions

- **Default timeout.** Should the V1 default be unbounded (current
  behaviour, backwards-compatible) or a generous default like 30
  minutes? The proposal recommends unbounded as the V1 default and
  per-lane opt-in as the migration path; promote to a default in
  V2 once dogfood evidence shows the cost of "no timeout" is
  higher than the cost of a surprise SIGTERM.
- **Sentinel exit code on timeout.** Use `None` (matches current
  schema for never-exited processes), `-1`, or a string like
  `"timed_out"`? V1 leans toward `None` plus `state='timed_out'` —
  the state column carries the meaning.
- **Should "missing artifact" block include partial publishes?** A
  job declaring three required artifacts where the agent published
  two should still block, with the diagnostic envelope listing the
  missing one. The proposal already does this; flagging here for
  reviewer clarity.
- **Optional outputs.** `expected_artifacts[].required = false`
  artifacts are not part of the missing-set computation. Verified
  against current schema.
- **Heartbeat vs timeout.** A heartbeat-based deadline (refresh on
  each `striatum heartbeat` call) would be more forgiving for
  long-running adapters that periodically check in. V1 ships only
  a flat timeout; heartbeat-based is a follow-up RFC if operators
  ask.
- **Reconcile cadence.** Should `recovery process-reconcile` be
  invoked automatically by `claim-next` or `status` calls, or
  remain operator-driven? V1 leans operator-driven (matching
  D036's stale-lease policy: lazy expiry on relevant CLI calls).
  Doctor surfaces the condition so operators know when to run it.
- **Long-lived supervised lanes (RFC 0009).** Those already have
  `supervisor.lost` plus the doctor check
  `supervisor_lost_with_held_lease`. This RFC does not change the
  supervised path; it only fills the parallel gap on the one-shot
  path. Should the two paths converge in V2? Probably yes, but
  not in this RFC.
- **Adapter-result envelope on the wire.** The reporter suggested
  "external CLIs either call Striatum completion commands
  themselves or write a well-known adapter result envelope." This
  RFC takes the first half (CLIs are still expected to call
  Striatum); the second half (a write-this-file contract) is a
  larger change deferred to a follow-up RFC.

## Relationship To Other RFCs

- **RFC 0009** — long-lived process supervision. Independent. RFC
  0009 owns the long-lived supervised flow; RFC 0014 owns the
  one-shot `adapter run` flow. The two paths share the
  `process_executions` table and the `command`/`pid`/`exit_code`
  shape but live at different layers of the lifecycle.
- **RFC 0010** — tool harness profiles. Independent of this RFC.
  Profile content is advisory; completion guarantees apply
  regardless of which profile a lane uses.
- **D028 (no transcripts)** — preserved. The diagnostic envelope
  contains zero child stdout/stderr; only metadata Striatum
  already collects (command, pid, exit code, duration) plus the
  output-validation deltas (missing artifact paths, missing
  verdict bool).
- **D036 (lazy stale-lease expiry)** — `recovery
  process-reconcile` follows the same pattern: operator-driven,
  lazy, mirrors the existing recovery surface.
- **RFC 0013 (proposed)** — the web UI's blocker view consumes the
  diagnostic envelope as structured data and renders the
  `recovery_commands` as click-to-copy buttons.

## Implementation Path

V1 ships in three landable steps:

1. **Post-exit validation + envelope.** New code in
   `process_adapter.py`; new blocker reasons; new event types;
   `tests/test_process_adapter.py`. (Smallest tractable PR.)
2. **Timeout / `--timeout-seconds`.** Adds CLI flag and lane field;
   covers the timeout test path.
3. **Reconciliation + doctor.** New `recovery process-reconcile`
   subcommand; two new doctor checks; `status` surface fields.

Each step has its own acceptance test. RFC 0014 is "accepted" once
all three steps land.

## Reproduction From Issue #1

A minimum reproduction once V1 lands:

```bash
striatum --repo . workflow validate examples/process-adapter-failure-fixture/workflow.json
striatum --repo . run prepare --workflow examples/process-adapter-failure-fixture/workflow.json
striatum --repo . run start --run-id <id>
striatum --repo . register-session --run-id <id> --role author --lane stub --capability write
striatum --repo . claim-next --session-id <session>
striatum --repo . adapter run --session-id <session> --lease-id <lease> \
  --stdin packet --timeout-seconds 5 --json
# Expected:
#   {"ok": true, "data": {"job_state": "blocked",
#                          "blocker_reason": "process_outputs_missing", ...}}
```

The fixture workflow declares a lane whose command is `bash -c
'exit 0'`, with a required `expected_artifact`. The reproduction
asserts the run lands in a clean blocked state without operator
intervention.
