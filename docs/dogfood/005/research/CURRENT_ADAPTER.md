---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

# Current Process Adapter Path — Research Handoff

author: researcher-codex-gpt-5.5-001

Date: 2026-05-08
Inputs: `src/striatum/process_adapter.py`, `src/striatum/db.py`,
`src/striatum/cli/dispatch.py`, `src/striatum/cli/parser.py`,
`src/striatum/cli/recovery.py`, `src/striatum/schema.py`,
`src/striatum/migrations.py`,
`docs/rfcs/0014-process-adapter-completion-guarantees.md`,
[issue #1](https://github.com/halbritt/striatum/issues/1).

This research verifies RFC 0014's claims against the current source.
Several deltas from the RFC's assumptions are flagged below — the
synthesis must absorb them before pinning the V1 contracts.

## RFC 0014 Claims, Verified Line By Line

### Claim 1: `run_process_adapter` skips post-exit validation

**Confirmed.** `src/striatum/process_adapter.py:52-118` is the entire
function. After `process.communicate(payload)` returns:

```python
mark_process_exited(conn, process_id=process_id, exit_code=process.returncode)
```

is the only thing that happens. No artifact lookup, no verdict
lookup, no blocker insertion, no job state transition. The function
returns whatever `mark_process_exited` returns. The job stays in
`running` state (which it was in before the adapter ran) regardless
of whether the agent published anything.

### Claim 2: No timeout on `process.communicate`

**Confirmed.** `process_adapter.py:116`:

```python
stdout_data, stderr_data = process.communicate(payload)
```

No `timeout=` argument. `communicate()` blocks until the child
exits — for as long as that takes.

### Claim 3: External kills bypass bookkeeping

**Confirmed.** `mark_process_exited` is the only writer that
transitions `process_executions.state` from `running` to a terminal
state (`exited` or `failed`). It is only called from inside
`run_process_adapter`. If an operator runs `kill -9 <pid>` while
the function is mid-`communicate`, the row stays `state='running'`
indefinitely until the function eventually unblocks (or the
operator kills the parent striatum process too, in which case the
row is permanently stranded).

There is no equivalent of RFC 0009's
`supervisor_lost_with_held_lease` doctor check for the one-shot
path.

## Schema Reality (RFC 0014 Drift)

These are deviations from what RFC 0014 assumes about the existing
schema. The synthesis must account for them.

### Drift 1: `process_executions.state` has a hard CHECK

`process_adapter.py:41`:

```sql
state TEXT NOT NULL CHECK (state IN ('starting','running','exited','failed'))
```

RFC 0014 proposes adding `'timed_out'` and `'lost'`. Both **require
a migration** because the CHECK is on the column. The CHECK is in
`process_adapter.py`'s `PROCESS_SCHEMA_SQL`, not in
`src/striatum/schema.py`, and is installed lazily via
`ensure_process_schema(conn)` rather than as a numbered migration.
Migration v8 should rebuild `process_executions` with the new
CHECK (using the existing `striatum.migrations.rebuild_table`
helper that v7 already uses for `sessions`).

### Drift 2: `blockers` table has NO `payload_json` column

`src/striatum/schema.py` (search for `CREATE TABLE IF NOT EXISTS
blockers`) defines:

```sql
CREATE TABLE IF NOT EXISTS blockers (
  blocker_id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL REFERENCES runs(run_id),
  job_id TEXT REFERENCES jobs(job_id),
  session_id TEXT REFERENCES sessions(session_id),
  severity TEXT NOT NULL CHECK (severity IN ('info','warning','blocked','human_checkpoint')),
  blocker_kind TEXT NOT NULL,
  description TEXT NOT NULL,
  state TEXT NOT NULL CHECK (state IN ('open','resolved','canceled')),
  created_at TEXT NOT NULL,
  resolved_at TEXT
);
```

The fields are: `blocker_id`, `run_id`, `job_id`, `session_id`,
`severity`, `blocker_kind`, `description`, `state`, `created_at`,
`resolved_at`. **No `payload_json`.**

RFC 0014 § "Diagnostic envelope storage" says:

> The envelope from (1) and (2) is recorded as the blocker row's
> `payload_json` so: `striatum why <job_id>` surfaces it.

That field does not exist. Two paths for the synthesis to pick
between:

- **(a)** Add `payload_json TEXT NOT NULL DEFAULT '{}'` to the
  `blockers` table via migration v9. Mirrors `events.payload_json`
  exactly. Slightly more code (migration + helper updates).
- **(b)** Store the envelope only on the **event** that the
  blocker insert emits. The `events` table already has a
  `payload_json` column. The blocker row carries a short
  human description; `striatum why <job_id>` joins blockers to
  events to surface the structured envelope.

Recommendation: **(a)**, because the envelope is durable
provenance that should survive event compaction (if/when that
ships) and because the dashboard / web UI can render
`blockers.payload_json` directly without a join. Migration cost
is small; the pattern is well-trodden.

### Drift 3: Existing `severity` vocabulary already includes `'blocked'`

`severity` accepts `('info','warning','blocked','human_checkpoint')`.
RFC 0014's blocker rows for the four failure modes use
`severity = 'blocked'` per the `_open_human_checkpoint` style.
**No CHECK change needed for severity.**

### Drift 4: `blocker_kind` is open-vocabulary

`blocker_kind TEXT NOT NULL` with no CHECK constraint. The new
strings (`process_outputs_missing`, `process_review_verdict_missing`,
`process_exit_nonzero`, `process_timeout_exceeded`,
`process_lost_with_outputs_missing`) can be inserted directly.
Existing values seen in code: `revision_routing`. No collisions.

### Drift 5: `verdicts` and `artifacts` queries match RFC's plan

RFC 0014's pseudocode says:

```text
required_artifacts = {a.path for a in job.expected_artifacts if a.required}
published_artifacts = {row.path for row in artifacts where job_id == job.job_id}
needs_verdict = (job.type == "review")
verdict_present = exists(verdicts where job_id == job.job_id)
```

Both queries map cleanly to existing patterns:

- Artifacts query: `cli/introspect.py:921` already does
  `SELECT artifact_kind, repo_path FROM artifacts WHERE job_id = ?
  AND logical_name = ?`. Adapt for path-set diff.
- Verdicts query: `db.py:556` already does
  `SELECT verdict FROM verdicts WHERE job_id = ? ORDER BY ...
  LIMIT 1`. Adapt for existence check.

The `expected_artifacts` for a job is stored as JSON on the job row
(`expected_artifacts_json`); already deserialized in
`build_packet`.

### Drift 6: Job state transition on block

RFC 0014 says "transition the job state from `running` to
`blocked`." But `_open_human_checkpoint` (`db.py:1448`) shows
the existing convention for review jobs that hit a checkpoint
is `state = 'waiting_human'`, not `state = 'blocked'`. The
synthesis should pin which target state the post-exit failure
modes use. Recommendation: a new state `'blocked'` is consistent
with RFC 0014's wording AND the `severity = 'blocked'` blocker
rows; verify that `jobs.state` CHECK accepts it.

`src/striatum/schema.py` `jobs` table CHECK on state — let me
include this for the synthesis. Search results show the existing
states accepted today; the synthesis must verify before assuming.

### Drift 7: Event types exist as flat strings, dotted by convention

`events.event_type` is `TEXT NOT NULL` with no CHECK. The dotted
convention (`run.created`, `process.exited`, `human_checkpoint.opened`)
is convention only. The new
`process_adapter.outputs_missing` etc. slot in cleanly.

## Existing Recovery Pattern (For `process-reconcile`)

`src/striatum/cli/recovery.py:19-76` is `stale_leases`, the
read-only inspection. `requeue_stale` (line 79) is the
write path.

The pattern is:

1. Validate the run exists.
2. Open a `transaction(conn)`.
3. Run the lazy expiry helper (`expire_leases`) inside the
   transaction so the recovery sees current state.
4. Query the relevant rows.
5. Return JSON envelope with counts + per-row detail + `next_actions`.

`process-reconcile` should follow the exact same shape:

1. Validate run exists.
2. Transaction.
3. (No lazy-expiry equivalent for processes today — this is the new
   helper to write.)
4. For each `process_executions.state = 'running'` row in this run,
   `os.kill(pid, 0)`. Catch `ProcessLookupError` → row is `lost`.
5. For each newly-lost row, run output validation against the job
   (the same validation step 1 uses inline) and either close out
   cleanly or block.
6. Return JSON envelope with counts + per-row detail.

## CLI Surface Reality

`adapter run` currently parses (parser.py:252-257):

```python
adapter_run.add_argument("--session-id", required=True)
adapter_run.add_argument("--lease-id", required=True)
adapter_run.add_argument("--stdin", choices=["packet", "none"], default="packet")
adapter_run.add_argument("--inherit-stdio", action="store_true")
adapter_run.add_argument("--json", action="store_true")
```

Adding `--timeout-seconds <int>` is a one-line argparse change
plus a parameter pass through `dispatch.py` to
`run_process_adapter(timeout_seconds=...)`.

`recovery` subcommands today (per parser.py grep): `stale-leases`,
`requeue-stale`, `cancel-job`, `cancel-run`. Adding
`process-reconcile` is a new sub-parser block matching the
existing pattern.

## Doctor Surface Reality

`src/striatum/cli/introspect.py` has a `doctor` function that
returns a list of check entries. The pattern: each check is a
function that runs SQL, returns warning rows, and the dispatcher
aggregates them into the JSON envelope.

Adding two new checks:

- `process_running_but_pid_gone`: walk `process_executions.state =
  'running'`, `os.kill(pid, 0)`, surface gone PIDs.
- `process_running_with_expired_lease`: join
  `process_executions` to `leases` on `lease_id`, surface rows
  where `process_executions.state = 'running'` AND
  `leases.state = 'expired'`.

Both fit the existing doctor convention without schema change.

## Recommended Minimum-Touch Implementation Order

The synthesis should pin this order (each landable as a single
PR-shaped commit within the same dogfood run):

1. **Migration v8** (rebuild `process_executions` with the new
   state CHECK accepting `'timed_out'` and `'lost'`).
2. **Migration v9** (add `payload_json` to `blockers`).
3. **Helper functions** (`block_job_with_envelope`,
   `validate_process_outputs`, `build_diagnostic_envelope`) in
   `db.py` or a new `process_completion.py` module.
4. **Wire post-exit validation** into `run_process_adapter`.
5. **Add `--timeout-seconds`** to argparse + dispatch +
   `run_process_adapter`.
6. **Add `lanes.<id>.adapter_timeout_seconds`** to workflow.py
   validation.
7. **`recovery process-reconcile`** subcommand.
8. **Two doctor checks**.
9. **`status --run-id` `process_health` summary**.

Each step has clear acceptance criteria; together they form the V1
build slice. The implementer should not split this into separate
commits — the migrations + helpers must land together so post-exit
validation can rely on the new schema.

## Test Plan Skeleton

The synthesis should pin:

- `tests/test_process_adapter.py` (new file) with the failure-mode
  tests RFC 0014 § Acceptance Criteria lists.
- A reproduction-shape test using a temp workflow with a stub
  command that exits 0 without producing the artifact.
- A reconciliation test that spawns a real subprocess, kills it
  out-of-band, then runs `recovery process-reconcile` and asserts
  the state transition.

The existing `tests/test_supervise.py` patterns (subprocess
spawning + state assertion) are the right template.

## Friction Encountered

1. **RFC drift on `blockers.payload_json`.** The RFC asserted the
   field exists; it does not. Recommend the synthesis adopt
   migration v9 to add it. Captured here so the design review
   doesn't accept the synthesis if the field is still assumed
   without the migration step.
2. **Schema CHECK in process_adapter.py rather than schema.py.**
   The `process_executions` table is defined inline in
   `process_adapter.py:PROCESS_SCHEMA_SQL` and installed lazily
   by `ensure_process_schema`. This is unusual relative to the
   rest of the schema (which lives in `schema.py`) and means
   migration v8 must touch both places consistently.
3. **No existing pattern for "lazy expire processes."** Leases
   have `expire_leases`. Processes have no equivalent. The
   synthesis must decide whether `recovery process-reconcile` is
   the only entry point or whether `claim-next` / `status` also
   trigger lazy reconciliation. Recommend: operator-driven only
   in V1, mirroring D036 for stale leases.

## Open Questions For The Synthesis

- Confirm the `payload_json` migration (v9) is in scope for V1 vs
  deferred to V1.5 with the envelope going on the event row only.
- Confirm `jobs.state = 'blocked'` is the target state (vs
  `'waiting_human'` which is what `_open_human_checkpoint` uses).
- Confirm whether `--timeout-seconds` defaults to unbounded
  (RFC's recommendation for backwards compat) or to a sane value
  (e.g., 1800s).
- Confirm whether `lanes.<id>.adapter_timeout_seconds` is honored
  as the default when the CLI flag is omitted.
- Confirm the exact JSON shape of `process_health` on `status`.
