---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

# RFC 0020 V1 — recovery primitives audit

author: researcher-codex-gpt-5.5-001

Date: 2026-05-09

## Existing primitives (the loop already has these)

- `src/striatum/db.py:expire_leases(conn, *, run_id)` — lazy
  expiry. Called by every CLI mutation. Marks expired leases
  `expired`, transitions repo-write jobs to `stale_lease`, and
  for review-only jobs reclaims work to `pending`.
- `src/striatum/cli/recovery.py:stale_leases(conn, *, run_id)` —
  read-only inventory. Calls `expire_leases` first; returns
  per-job recovery context with `repo_write: bool` and
  `recovery_policy: manual_inspection_required |
  safe_to_reclaim_when_pending`. **This is exactly the
  classifier `recovery auto` needs.**
- `src/striatum/cli/recovery.py:requeue_stale(conn, *, run_id,
  job_id)` — bounded operator mutation. Refuses repo-write jobs
  by design (D036). Already idempotent against an already-
  reclaimed message.
- `src/striatum/cli/recovery.py:process_reconcile(conn, *,
  run_id)` — RFC 0014. Walks `process_executions.state =
  'running'` and transitions externally-killed processes to
  `lost`.
- `src/striatum/db.py:is_repo_write(job)` — single source of
  truth for the safe/unsafe split. `recovery auto` reuses it.
- Doctor checks already differentiate `human_checkpoint` from
  `blocked` blockers; the autonomous classifier reads the same
  `severity` column.

## What `recovery auto` actually adds

Just an *orchestrator*. Concretely:

1. Call `expire_leases(conn, run_id=run)`.
2. Call `process_reconcile(conn, run_id=run)`.
3. Read `stale_leases(conn, run_id=run)`. For each row whose
   `repo_write` is `False` and `recovery_policy` is
   `safe_to_reclaim_when_pending`, call `requeue_stale(conn,
   run_id, job_id)`. Cap at `max_requeues_per_sweep`.
4. Walk `blockers` rows where `state = 'open'` and
   `severity = 'human_checkpoint'`. For each whose
   `created_at` is older than the policy's
   `checkpoint_timeout_seconds`, run the escalation hook.
5. Walk `blockers` rows where `state = 'open'` and
   `severity = 'blocked'` and the job has no active lease.
   Fire a new doctor check `blocker_recovery_eligible`; for V1
   this only flags, it does not autonomously act.
6. Return `{swept_at, actions: [...], escalations: [...],
   still_stuck: [...]}`.

No new SQLite tables, no new aggregate. The autonomous loop is
a coordinator over existing verbs.

## Module shape (recommended)

A single new file `src/striatum/recovery/auto.py` that exposes
`run_auto_sweep(conn, *, run_id, policy, dry_run, hook_runner)
-> JsonObject`. The CLI dispatcher in `cli/dispatch.py` calls
it; the parser in `cli/parser.py` adds the `recovery auto`
subcommand. Hooks live in `src/striatum/recovery/hooks.py` with
three small functions: `run_marker_file_hook`,
`run_webhook_hook`, `run_shell_hook`. Each returns a status
dict the sweep envelope folds into `escalations`.

Keep `src/striatum/recovery/__init__.py` small — re-export
`run_auto_sweep` and the hook runners.

The existing `src/striatum/cli/recovery.py` module stays the
home of the operator-facing read/mutation helpers
(`stale_leases`, `requeue_stale`, `cancel_job`,
`process_reconcile`); the new `striatum.recovery` package owns
the autonomous loop. Two import paths, but
`striatum.recovery.run_auto_sweep` calls *into*
`striatum.cli.recovery.requeue_stale` — that's the right
direction.

## `recovery_policy` block validator

The validator in `src/striatum/workflow.py` already has slots
for top-level workflow fields (e.g., `parallelism`,
`harness_profiles`, `branch`). Adding `recovery_policy` is
~30 lines of validation: type checks on each field, range
checks on the integer thresholds, hook-kind enum + per-kind
field requirements, and a defaults helper that returns the
runner-default policy when the workflow omits it.

## Doctor check `blocker_recovery_eligible`

Add to `src/striatum/cli/introspect.py`'s
`DOCTOR_CHECKS` tuple. The check fires for:

- Blockers in `state = 'open'`.
- `severity = 'blocked'` (not `human_checkpoint`).
- Job has no `current_lease_id` OR the lease is `expired`.
- `created_at` older than the policy's
  `eligible_after_seconds` (defaults to 600 = 10 minutes).

Surface in `doctor --verbose --json` as a structured record
with `recovery_command` set to the exact `striatum recovery
auto --run-id <id>` invocation.

## Hook implementations

- **marker_file**: append-only Markdown write at the policy's
  `path`. Refuses paths inside `.striatum/` or outside the repo
  root (same boundary `evidence export` enforces).
- **webhook**: `urllib.request.urlopen(url, data=...,
  timeout=10)` with Content-Type: application/json. On any
  failure (HTTPError, URLError, timeout), emit
  `recovery.escalation_failed` event but do not raise.
- **shell**: build a `command_request` row (existing schema)
  and shell out via the existing process adapter so lane
  constraints apply. The shell hook's stdout/stderr go to
  `DEVNULL` (D028).

## Test plan

`tests/test_recovery_auto.py`:

- `test_recovery_auto_dry_run_makes_no_state_changes`
- `test_recovery_auto_requeues_stale_review_only`
- `test_recovery_auto_does_not_requeue_stale_repo_write`
- `test_recovery_auto_calls_process_reconcile`
- `test_recovery_auto_escalates_human_checkpoint_after_timeout`
- `test_recovery_auto_does_not_escalate_human_checkpoint_below_timeout`
- `test_recovery_auto_envelope_shape`
- `test_recovery_auto_marker_file_hook_writes_path`
- `test_recovery_auto_marker_file_hook_refuses_striatum_path`
- `test_recovery_auto_max_requeues_per_sweep_cap`
- `test_recovery_policy_block_validator_unknown_kind_rejected`
- `test_recovery_policy_block_validator_negative_threshold_rejected`
- `test_recovery_policy_omitted_preserves_today_behavior`
- `test_doctor_blocker_recovery_eligible_fires`

`tests/test_recovery_hooks.py`:

- `test_webhook_hook_posts_envelope`
- `test_webhook_hook_failure_does_not_abort_sweep`
- `test_shell_hook_runs_through_process_adapter`

Step 3 (`recovery watch` daemon) deferred per RFC 0020 § 4.

## Friction anticipated

- **Determinism in tests.** `swept_at` must be a UTC string;
  tests compare against frozen-clock fixtures or use a
  monkeypatched `utc_now`.
- **Hook side effects in tests.** marker_file hook tests use
  `tmp_path`; webhook tests use a `socketserver` fixture;
  shell hook tests use `/bin/true` so the adapter constraint
  layer is exercised without real-world commands.
- **No-policy regression.** A workflow without
  `recovery_policy` must produce identical packets, identical
  state transitions, and identical doctor output to v1.4.1.
  Test by adding an explicit fixture that round-trips through
  validate → prepare → start and diffs the work-packet bytes
  against a v1.4.1 baseline.
