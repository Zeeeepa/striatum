---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/rfcs/0020-autonomous-stalled-run-recovery.md", "docs/dogfood/014/research/RECOVERY_PRIMITIVES.md", "src/striatum/cli/recovery.py", "src/striatum/db.py", "src/striatum/workflow.py"]
---

# RFC 0020 V1 Design Synthesis

author: designer-codex-gpt-5.5-001

Date: 2026-05-09
Target: V1 build slice = RFC 0020 steps 1+2. Step 3
(`recovery watch` daemon) deferred. Per the RFC, V1 is
"accepted once steps 1+2 land."

## Locked Contracts

### `striatum.recovery.run_auto_sweep`

```python
def run_auto_sweep(
    conn: sqlite3.Connection,
    *,
    run_id: str,
    policy: dict,                    # resolved policy (workflow + CLI overrides + defaults)
    dry_run: bool = False,
    hook_runner: Callable | None = None,  # injected for tests
    now: Callable[[], str] = utc_now,
) -> dict:
```

Sweep order, in this exact sequence:

1. `expire_leases(conn, run_id=run_id)` — same as today.
2. `process_reconcile(conn, run_id=run_id)` — RFC 0014.
3. **Autonomous review-only requeue.** Read the rows
   `stale_leases(conn, run_id=run_id)` produces. For each row
   whose `repo_write` is `False` AND `recovery_policy` is
   `safe_to_reclaim_when_pending`, call `requeue_stale(conn,
   run_id, job_id)`. Cap at `policy.max_requeues_per_sweep`
   (default 8). Append each requeue to
   `actions[]`.
4. **Checkpoint timeout escalation.** Walk `blockers` rows
   where `state = 'open'` and `severity = 'human_checkpoint'`.
   For each `created_at` older than
   `policy.checkpoint_timeout_seconds` (default 14400 = 4h),
   call the escalation hook. Append to `escalations[]`.
5. **Eligible-blocker doctor flagging.** Walk `blockers` rows
   where `state = 'open'` and `severity = 'blocked'` and the
   job has no active lease and `created_at` is older than
   `policy.eligible_after_seconds` (default 600 = 10 min).
   Append to `still_stuck[]`. (No autonomous action; the new
   doctor check `blocker_recovery_eligible` surfaces it.)

Envelope:

```json
{
  "run_id": "...",
  "swept_at": "2026-05-09T00:30:00Z",
  "policy_source": "workflow" | "default" | "cli_override",
  "actions": [
    {"kind": "lease_expired", "lease_id": "..."},
    {"kind": "process_reconciled", "process_id": "..."},
    {"kind": "review_requeued", "job_id": "...", "workflow_job_id": "..."}
  ],
  "escalations": [
    {"kind": "checkpoint_timeout", "blocker_id": "...", "hook": {...}}
  ],
  "still_stuck": [
    {"reason": "blocker_recovery_eligible", "blocker_id": "...", "job_id": "..."}
  ]
}
```

`dry_run=True` runs the same logic but mutations short-circuit
(stage `actions` / `escalations` without writing).

### CLI

```text
striatum recovery auto
  --run-id <id>
  [--dry-run]
  [--max-requeue <n>]
  [--checkpoint-timeout <seconds>]
  [--escalation-hook <path>]   # CLI override of workflow hook
  [--json]
```

Read-permitted (the verb itself is operator-driven, but
`recovery` is on the read whitelist for the service mutation
gate per D058 — the `auto` subcommand is a *new* verb that
performs mutations, so it inherits the gate's
`requires_allow_mutations` rule). Update
`is_read_command` to recognise `recovery auto` as a mutation
verb.

### `recovery_policy` workflow block

```json
{
  "recovery_policy": {
    "autonomous_review_requeue": false,
    "autonomous_process_reconcile": false,
    "checkpoint_timeout_seconds": 14400,
    "eligible_after_seconds": 600,
    "max_requeues_per_sweep": 8,
    "max_total_requeues_per_job": 3,
    "escalation_hook": {
      "kind": "marker_file",
      "path": "striatum/<workflow-slug>/STALLED.md"
    }
  }
}
```

All fields optional. Omitted block → today's behavior preserved
byte-for-byte. Validator rules per RFC 0020 § "Step 2"; reject
unknown hook kinds, negative numbers, and `escalation_hook.path`
inside `.striatum/`.

`autonomous_review_requeue` and `autonomous_process_reconcile`
default to `false` so that `striatum recovery auto` against a
workflow without an explicit policy is purely diagnostic — it
runs `expire_leases` and reports `still_stuck` but does not
requeue. The CLI flag `--autonomous-review-requeue` overrides
to `true` for one-shot operator use.

### Hooks

`src/striatum/recovery/hooks.py`:

- `run_marker_file_hook(*, target: Path, path: str, body: str)
  -> dict` — append-only Markdown write. Refuses
  `.striatum/`, refuses paths outside the repo. Returns
  `{"kind": "marker_file", "wrote": True, "path": ...}`.
- `run_webhook_hook(*, url: str, payload: dict, timeout: float
  = 10.0) -> dict` — POSTs JSON via `urllib.request`. On
  failure, returns `{"kind": "webhook", "wrote": False,
  "error": ...}` and the sweep emits a
  `recovery.escalation_failed` event.
- `run_shell_hook(*, command: list[str], env: dict | None) ->
  dict` — runs through the existing process adapter so lane
  constraint enforcement applies. stdout/stderr go to
  `DEVNULL` (D028). Returns `{"kind": "shell", "exit_code":
  ...}`.

### Doctor

New check `blocker_recovery_eligible` registered in
`DOCTOR_CHECKS`. Helper `_check_blocker_recovery_eligible`
fires for blockers matching the criteria above and includes
the `recovery_command` (`striatum recovery auto --run-id
<id>`).

### Module layout

```
src/striatum/recovery/
  __init__.py              # re-exports run_auto_sweep + hook runners
  auto.py                  # the orchestrator
  hooks.py                 # marker_file, webhook, shell
  policy.py                # parse + default a workflow policy
```

Existing `src/striatum/cli/recovery.py` is **unchanged**. The
new package wraps it.

## Test Plan (pinned)

Two test files. Tests itemized in the research artifact §
"Test plan". Total ~17 new tests; suite count moves
288 → ~305.

## Acceptance Criteria

- `striatum recovery auto --run-id <id> --dry-run` produces a
  structured envelope and changes no state.
- `striatum recovery auto --run-id <id>
  --autonomous-review-requeue` requeues stale review-only jobs
  up to `max-requeue` and skips repo-write.
- `striatum recovery auto --run-id <id>
  --autonomous-process-reconcile` reconciles externally-killed
  processes.
- `human_checkpoint` blockers older than the timeout fire the
  escalation hook exactly once per sweep until the blocker
  resolves.
- The marker_file hook refuses `.striatum/` and out-of-repo
  paths.
- The webhook hook continues the sweep on failure with
  `recovery.escalation_failed` event.
- The shell hook runs through the process adapter.
- A workflow that omits `recovery_policy` produces packets,
  runs, and recovery output byte-identical to v1.4.1.
- `doctor --verbose --json` includes
  `blocker_recovery_eligible` records when applicable; the
  `recovery_command` field carries the exact CLI invocation.
- Test count = ~305; lint + typecheck clean.
- `pyproject.toml` and `__version__` bump 1.4.1 → 1.5.0.

## Acceptance Gate

Implementation job blocks until human acceptance recorded
under `docs/dogfood/014/decisions/`.
