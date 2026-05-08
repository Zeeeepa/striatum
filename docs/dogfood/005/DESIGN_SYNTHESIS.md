---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/rfcs/0014-process-adapter-completion-guarantees.md", "docs/dogfood/005/research/CURRENT_ADAPTER.md", "src/striatum/process_adapter.py", "src/striatum/db.py", "src/striatum/cli/dispatch.py", "src/striatum/cli/parser.py", "src/striatum/cli/recovery.py", "src/striatum/schema.py", "src/striatum/migrations.py"]
---

# RFC 0014 V1 Design Synthesis

author: designer-codex-gpt-5.5-001

Date: 2026-05-08
Target: V1 build slice for RFC 0014 (process adapter completion
guarantees). Closes [issue #1](https://github.com/halbritt/striatum/issues/1).
Inputs above; the research handoff at
`docs/dogfood/005/research/CURRENT_ADAPTER.md` is load-bearing —
several RFC assumptions had to be revised after source verification.

This synthesis is implementation-ready. Contracts pinned below are
exact; the implementer follows them verbatim.

## 1. Schema Changes

### Migration v8 — `process_executions.state` enum extension

Rebuild `process_executions` with a CHECK that accepts the V1
state vocabulary:

```sql
state TEXT NOT NULL CHECK (state IN (
  'starting','running','exited','failed','timed_out','lost'
))
```

Use `striatum.migrations.rebuild_table` (the helper v7 already uses
for `sessions`). The CHECK in `process_adapter.py:PROCESS_SCHEMA_SQL`
must be updated to match so freshly-initialized DBs install the new
constraint directly without needing the migration to fire.

### Migration v9 — `blockers.payload_json`

Add a new column:

```sql
ALTER TABLE blockers
  ADD COLUMN payload_json TEXT NOT NULL DEFAULT '{}';
```

Forward-only; existing rows default to `'{}'`. The
`schema.py:CREATE TABLE blockers` definition must be updated so
fresh DBs install the column directly.

Both migrations land in the same commit. Migration tests must
cover: idempotent re-run, fresh-DB install matches migrated DB,
existing rows preserved.

## 2. Diagnostic Envelope JSON Schema

The diagnostic envelope is stored as
`blockers.payload_json` for every process-adapter blocker and
also embedded in the corresponding event's `events.payload_json`.

```json
{
  "envelope_version": "striatum.process_adapter.envelope.v1",
  "process_id": "proc_<hex>",
  "command": ["claude", "--model", "opus", "-p"],
  "exit_code": 0,
  "duration_seconds": 142.7,
  "timeout_seconds": null,
  "missing_artifact_paths": ["docs/.../REVIEW.md"],
  "review_verdict_missing": false,
  "recovery_commands": [
    "striatum publish-artifact --session-id ... --kind ... --path ...",
    "striatum verdict --session-id ... --verdict ...",
    "striatum recovery requeue-stale --run-id ... --job-id ..."
  ]
}
```

Field rules:

- `envelope_version`: literal `"striatum.process_adapter.envelope.v1"`.
- `process_id`: from `process_executions`.
- `command`: the lane command array as launched (already in
  `process_executions.command_json`).
- `exit_code`: `int` for normal exits; `null` for `timed_out` and
  `lost`.
- `duration_seconds`: float; `(ended_at - started_at)`.
- `timeout_seconds`: `int` if the timeout fired; otherwise `null`.
- `missing_artifact_paths`: list of repo-relative paths from
  `expected_artifacts` where `required: true` and the path is not
  present in the artifacts table for this `job_id`. Empty list when
  this isn't the failure mode (e.g., `process_exit_nonzero` with
  no artifact requirements).
- `review_verdict_missing`: `true` only when `job.type == "review"`
  and no row exists in `verdicts` for this `job_id`. `false`
  otherwise.
- `recovery_commands`: list of shell-string suggestions the operator
  can copy-paste. Always include `striatum recovery process-reconcile
  --run-id <id>` plus the publish/verdict/requeue commands relevant
  to the specific failure mode.

The envelope **never** includes child stdout, stderr, or model
output. D028 enforced by construction (the envelope is built from
schema fields only).

## 3. Blocker Reason Vocabulary

`blockers.blocker_kind` is open-vocabulary (no CHECK). V1 introduces
exactly five new kinds for the process-adapter completion path:

| `blocker_kind` | When |
|---|---|
| `process_outputs_missing` | exit 0, required artifact(s) not published |
| `process_review_verdict_missing` | exit 0, review job, no verdict recorded (artifacts may have been published or not) |
| `process_exit_nonzero` | exit code != 0 (regardless of artifacts) |
| `process_timeout_exceeded` | timeout fired; child SIGTERM'd |
| `process_lost_with_outputs_missing` | reconciler found dead PID; required outputs absent |

Severity for all five: `'blocked'`. State for all five (initial):
`'open'`.

If multiple conditions are true (e.g., exit 0 AND artifact missing
AND verdict missing on a review job), insert exactly one blocker
with the most-specific reason. Ranked priority:

1. `process_exit_nonzero` (exit code != 0; supersedes everything)
2. `process_timeout_exceeded` (timeout fired)
3. `process_review_verdict_missing` (review job + missing verdict)
4. `process_outputs_missing` (other outputs missing)

`missing_artifact_paths` and `review_verdict_missing` in the
envelope reflect the actual state regardless of the chosen reason,
so reviewers see the full picture even when the reason is the
priority winner.

## 4. Event Type

`events.event_type = 'process_adapter.outputs_missing'` for every
case (single event type, the `payload_json.envelope.blocker_kind`
field disambiguates). Rationale: keeping one event type simplifies
SSE filters and `striatum why` output. The blocker row carries the
specific kind for queryability.

Event `payload_json` shape:

```json
{
  "blocker_id": "blk_<hex>",
  "envelope": { /* the diagnostic envelope from § 2 */ }
}
```

## 5. Job State Transition

Blocker rows land with `severity = 'blocked'`. The job state
transitions from `running` to `'blocked'` (existing `jobs.state`
CHECK already accepts `'blocked'` — verified in research).

The lease is **not** released. RFC 0014 says: "leave the lease in
place until lease expiry or explicit operator recovery." This
matches D036's lazy-expiry posture. The session that ran the
adapter still owns the work; lease expiry will eventually transition
the job to `stale_lease` if the operator does nothing.

`queue_messages` row state stays whatever it was (typically
`assigned` for the active claim). Recovery commands in the envelope
include `recovery requeue-stale` so operators have a one-line path
to recovery.

## 6. CLI Surface

### `striatum adapter run --timeout-seconds <n>`

Add to `parser.py:adapter_run`:

```python
adapter_run.add_argument("--timeout-seconds", type=int, default=None)
```

Validation: `n > 0` when set; `argparse` error with exit 8 for
non-positive values.

When set, `run_process_adapter(..., timeout_seconds=n)` wraps
`process.communicate(payload, timeout=n)` in
`try/except subprocess.TimeoutExpired`. On timeout:

1. `process.terminate()` (SIGTERM).
2. `process.wait(timeout=5)`; if still alive, `process.kill()`
   (SIGKILL).
3. `mark_process_timed_out(conn, process_id, timeout_seconds=n)`
   (new helper) — sets `state='timed_out'`, `ended_at=utc_now()`,
   `exit_code=NULL`.
4. Insert blocker `process_timeout_exceeded` with envelope
   carrying `timeout_seconds`.
5. Job `'running' → 'blocked'`.
6. Return result envelope with `status: "timed_out"`.

When omitted, behaviour is unchanged — `process.communicate(payload)`
with no timeout. Default stays unbounded for backwards
compatibility.

### `lanes.<id>.adapter_timeout_seconds`

Workflow validation accepts the optional field. Type: positive
integer when present. The CLI flag overrides the lane field; the
lane field is the default when the flag is omitted; with neither
set, behaviour stays unbounded.

`workflow.py:_validate_lane_constraints` extended with one branch
that checks `lane_value.get("adapter_timeout_seconds")`.

### `striatum recovery process-reconcile --run-id <id> [--json]`

New `parser.py:recovery_subparsers` entry. Dispatch to a new
function in `cli/recovery.py:process_reconcile`. Mirrors the shape
of `requeue_stale`:

1. `row_by_id(conn, "runs", "run_id", run_id)` — verify run.
2. `with transaction(conn):` block.
3. Walk `process_executions` rows for this run with
   `state = 'running'`.
4. For each, `os.kill(pid, 0)`:
   - `OSError(ESRCH)` / `ProcessLookupError` → process gone.
     Transition `state = 'lost'`, `ended_at = utc_now()`. Re-run
     output validation against the job; insert blocker if
     applicable (kind `process_lost_with_outputs_missing` or
     `process_review_verdict_missing`).
   - `PermissionError` (EPERM) → process exists but not ours.
     Treat as alive; surface in JSON output as `still_running`
     with a `notes: ["process is running but owned by another uid"]`.
   - No error → process alive. Surface as `still_running`.
5. Return JSON envelope:

```json
{
  "ok": true,
  "data": {
    "run_id": "run_...",
    "still_running": [{"process_id": ..., "pid": ..., "elapsed_seconds": ...}],
    "transitioned_to_lost": [{"process_id": ..., "blocker_id": ..., "blocker_kind": ...}],
    "next_actions": ["inspect_blockers", "decide_recovery_path"]
  }
}
```

Lazy-expiry hook: do **not** auto-invoke from `claim-next` or
`status` in V1. Operator-driven only, mirroring D036.

## 7. Doctor Checks

Two new entries in `cli/introspect.py:doctor`:

### `process_running_but_pid_gone`

```python
SELECT pe.process_id, pe.run_id, pe.job_id, pe.pid, pe.started_at
FROM process_executions pe
WHERE pe.state = 'running'
```

For each row, `os.kill(pe.pid, 0)`. Surface those that raise
`ProcessLookupError` with severity `warning`, recommended action
`recovery process-reconcile --run-id <run_id>`.

### `process_running_with_expired_lease`

```python
SELECT pe.process_id, pe.run_id, pe.job_id, pe.pid, l.expires_at
FROM process_executions pe
JOIN leases l ON l.lease_id = pe.lease_id
WHERE pe.state = 'running' AND l.state = 'expired'
```

Surface with severity `warning`, action
`recovery process-reconcile`.

## 8. Status Field — `process_health`

`striatum status --run-id <id>` JSON envelope grows a sibling
key alongside the existing keys:

```json
{
  "process_health": {
    "running_count": 0,
    "stale_running_count": 0,
    "lost_count": 0,
    "timed_out_count": 0,
    "next_actions": []
  }
}
```

`running_count`: count of `process_executions.state = 'running'`
for this run.
`stale_running_count`: subset where the lease is expired.
`lost_count`: cumulative `state = 'lost'` for this run.
`timed_out_count`: cumulative `state = 'timed_out'` for this run.
`next_actions`: includes `"recovery process-reconcile"` when
`stale_running_count > 0` OR a doctor check would fire.

## 9. Implementation Order (Verbatim)

The implementer commits the following changes as one combined diff:

1. **`src/striatum/migrations.py`** — `_apply_v8_process_state_enum`
   and `_apply_v9_blockers_payload_json` plus registration entries.
2. **`src/striatum/schema.py`** — update the `process_executions`
   CHECK enum (so fresh DBs match) and add `payload_json` to the
   `blockers` CREATE TABLE.
3. **`src/striatum/process_adapter.py`** — update
   `PROCESS_SCHEMA_SQL` enum to match; add helpers
   `mark_process_timed_out`, `mark_process_lost`; refactor
   `run_process_adapter` to:
   - accept `timeout_seconds: int | None = None`,
   - call `process.communicate(payload, timeout=timeout_seconds)`
     in a try/except,
   - on success, run `validate_outputs_and_block_if_needed` (new),
   - on timeout, terminate/kill and block via the same helper.
4. **New module `src/striatum/process_completion.py`** — homes
   `build_diagnostic_envelope`, `validate_outputs`,
   `block_job_with_envelope`. Keeps `process_adapter.py` slim and
   gives the reconciler a single import surface.
5. **`src/striatum/workflow.py`** — `_validate_lane_constraints`
   accepts optional `adapter_timeout_seconds` (positive int).
6. **`src/striatum/cli/parser.py`** — `--timeout-seconds` on
   `adapter run`; new `process-reconcile` subcommand.
7. **`src/striatum/cli/dispatch.py`** — wire the new flag and
   subcommand to their handlers.
8. **`src/striatum/cli/recovery.py`** — `process_reconcile`
   function.
9. **`src/striatum/cli/introspect.py`** — two new doctor checks;
   `process_health` summary on `status`.
10. **`tests/test_process_adapter.py`** (new) — see § 10.
11. **Docs** — see § 11.

## 10. Test Plan (Verbatim)

`tests/test_process_adapter.py` covers:

| Test | Asserts |
|---|---|
| `test_happy_path_artifact_present_no_block` | exit 0 + artifact published → no blocker; job stays running |
| `test_exit_zero_missing_required_artifact_blocks` | exit 0 + required artifact absent → blocker `process_outputs_missing`, job state `'blocked'`, envelope contains the missing path |
| `test_exit_zero_review_no_verdict_blocks` | review job, exit 0, artifact present, no verdict → blocker `process_review_verdict_missing`, envelope `review_verdict_missing: true` |
| `test_exit_zero_review_no_artifact_no_verdict_priority` | both missing → blocker is `process_review_verdict_missing` (priority); envelope reflects both |
| `test_exit_nonzero_blocks_regardless` | exit code 1 → blocker `process_exit_nonzero`, envelope `exit_code: 1` |
| `test_timeout_terminates_and_blocks` | `--timeout-seconds 1` against a `sleep 30` child → SIGTERM, `state='timed_out'`, blocker `process_timeout_exceeded`, envelope `timeout_seconds: 1` |
| `test_timeout_sigkill_fallback` | child traps SIGTERM and ignores → SIGKILL after 5s |
| `test_lane_field_default_used_when_flag_omitted` | workflow declares `adapter_timeout_seconds: 1`, CLI flag omitted → timeout fires |
| `test_cli_flag_overrides_lane_default` | workflow declares `adapter_timeout_seconds: 60`, CLI flag `--timeout-seconds 1` → CLI wins |
| `test_reconcile_transitions_dead_pid_to_lost` | spawn subprocess that exits, mark process row as running with stale `state` (simulating external kill), run `recovery process-reconcile`, assert `state='lost'`, blocker inserted |
| `test_reconcile_keeps_alive_pid_running` | spawn long-running subprocess, run reconcile, assert `state` stays `running` |
| `test_doctor_flags_pid_gone` | manually create `process_executions` row with bogus PID, `doctor --json` surfaces the check |
| `test_doctor_flags_running_with_expired_lease` | run with expired lease + `state='running'` → check fires |
| `test_status_process_health_summary` | run with mixed state rows → `status --json` surfaces correct counts |
| `test_envelope_contains_no_stdout_stderr` | regression test: build envelope, JSON-dump, assert no field carries child output |
| `test_issue_one_reproduction` | minimal fixture: workflow with `bash -c 'exit 0'` lane and required artifact → `adapter run` lands in blocked state with `process_outputs_missing` |
| `test_blockers_payload_json_round_trip` | insert a blocker via the new helper, query back, assert envelope JSON parses to the expected shape |
| `test_migration_v8_v9_idempotent` | apply migrations twice → no error, schema unchanged second time |

Existing tests (`test_supervise.py`, `test_recovery_extended.py`)
should be unchanged.

## 11. Documentation Updates

- **`docs/SPEC.md`** § "Adapter Boundary" / § "Process Supervision"
  — add a new subsection "Single-Shot Process Adapter Completion
  Guarantees" describing post-exit validation, timeout, reconcile,
  and the diagnostic envelope. Reference issue #1 closure.
- **`docs/UBIQUITOUS_LANGUAGE.md`** — entries:
  - "diagnostic envelope" — privacy-safe metadata block recorded
    on every process-adapter blocker.
  - "process completion validation" — runner-side check that
    required artifacts and review verdicts were produced after
    `adapter run` exits.
- **`docs/rfcs/0014-process-adapter-completion-guarantees.md`** —
  status from `proposed` to `accepted (V1)`. Add "V1
  Implementation Slice" subsection citing this synthesis,
  the build handoff, and the two migrations.
- **`docs/DECISION_LOG.md`** — D-row (next available number)
  recording RFC 0014 acceptance.
- **`docs/TODO.md`** — F-row marking RFC 0014 V1 done.
- **`README.md`** — short paragraph under "Process Supervision"
  pointing at the new guarantees and the `--timeout-seconds` flag.
- **`CHANGELOG.md`** — Unreleased entry under Added covering
  envelope, timeout, reconcile, doctor checks.
- **`docs/rfcs/README.md`** — index entry status flip for 0014.

## 12. Reproduction Fixture (Issue #1)

Add `examples/process-adapter-failure-fixture/workflow.json` with
a single-job workflow:

```json
{
  "schema_version": "striatum.workflow.v1",
  "workflow_id": "process-adapter-failure-fixture",
  "name": "Process adapter failure fixture (issue #1)",
  ...
  "lanes": {
    "stub": {
      "adapter": "process",
      "command": ["bash", "-c", "exit 0"],
      "capabilities": ["write"]
    }
  },
  "jobs": [
    {
      "id": "demo",
      "type": "generic",
      "lane_id": "stub",
      "expected_artifacts": [{"path": "docs/demo/OUT.md", "required": true, ...}]
    }
  ]
}
```

The reproduction test loads this fixture, runs the lane, and
asserts the run lands in blocked state with
`blocker_kind = 'process_outputs_missing'`. This is the reproduction
RFC 0014 § "Reproduction From Issue #1" describes.

## 13. Deferred Items (Not In V1)

- **Heartbeat-based timeout.** A timeout that resets on each
  `striatum heartbeat` call. V1 ships a flat timeout. Heartbeat
  variant is a follow-up RFC.
- **Adapter result envelope file.** A "agent writes
  `.striatum/scratch/<id>/result.json`" contract. Out of scope
  for V1; the V1 contract is "agent calls `striatum publish-artifact`
  + `verdict` + `complete` directly" (existing convention).
- **Auto-reconcile on `claim-next` / `status`.** V1 stays
  operator-driven (D036 lazy-expiry pattern).
- **Queue-message state on block.** V1 leaves
  `queue_messages.state` at whatever the original claim set it to.
  A future RFC may flip messages to a new `'blocked'` state to
  improve `striatum why` clarity; out of scope.
- **`process_executions` reason / detail column.** V1 stores
  failure context in the blocker envelope. A dedicated
  `process_executions.failure_kind` column is conceivable but
  redundant; deferred.
- **Lease auto-release on block.** RFC 0014 says "leave the lease
  in place." Confirmed; D036's lazy-expiry handles cleanup.

## 14. Open Questions Reviewers May Raise

- **"Why one event type instead of one per blocker_kind?"**
  Single event type simplifies SSE filtering and avoids event-vocab
  growth. Disambiguation lives in the envelope's blocker reference
  + the blocker row's `blocker_kind`. Open to revision if reviewers
  prefer per-kind events.
- **"Why not auto-reconcile on `status`?"** D036 stayed lazy-on-CLI
  for stale leases; same precedent here. Reviewer may push for
  auto-reconcile from `claim-next` to close the operator-attention
  gap. Defer to V1.5 if requested.
- **"Why default timeout = unbounded?"** Backwards compatibility.
  Reference workflows under `examples/` and `docs/dogfood/` should
  set `adapter_timeout_seconds: 1800` as a sane default during the
  same change. Open to flipping the global default in V2.
- **"Should `recovery process-reconcile` accept `--all` to walk all
  runs?"** V1 ships per-run only. Multi-run variant deferred.

## Acceptance Gate

Per the dogfood-005 SKILL.md, the implementation job must block
until a human acceptance decision is recorded under
`docs/dogfood/005/decisions/`. This synthesis explicitly does not
authorize implementation; it produces the design that the review
and human acceptance gate evaluates.
