# RFC 0020: Autonomous Stalled-Run Recovery

Status: proposed
Date: 2026-05-08
Context:
`docs/SPEC.md` § "Recovery" + § "Process Supervision" + §
"Sessions",
`docs/DECISION_LOG.md` D036 (lazy-on-CLI lease expiry),
`src/striatum/cli/recovery.py` (`stale_leases`,
`requeue_stale`, `cancel_job`, `process_reconcile`),
`src/striatum/db.py:expire_leases`,
RFC 0014 (process adapter completion guarantees, accepted V1),
RFC 0019 (DDD foundations, proposed) — autonomous recovery is
*coordination* domain, not agent domain

## Problem

Today's recovery is **operator-driven by design**. D036 pinned
the policy: lease expiry is lazy — normal CLI commands expire
stale leases as they run, but no background daemon reaps them
on its own. That works for *attended* runs (an operator is at a
keyboard typing `striatum status`, which expires leases as a
side effect). It breaks for **overnight runs** and any run where
the operator is asleep, on a flight, or otherwise not poking
the CLI for hours.

A real incident, paraphrased from a recent Engram run:

> An overnight run stalled mid-way through, waiting on what
> looked like human input. The run could have been requeued
> easily — the work was review-only and D036 already permits
> autonomous review-only requeue. But there was no autonomous
> sweeper, and no operator-typed CLI verb to trigger the lazy
> expiry. The run sat blocked until morning.

The runner has every primitive needed to recover safely:

- `expire_leases(conn, run_id=...)` already knows how to mark
  stale leases.
- `recovery requeue-stale --run-id ... --job-id ...` already
  refuses repo-write work and requeues review-only work safely
  — review-only autonomy is *already approved* by D036.
- `recovery process-reconcile` reconciles externally-killed
  process executions per RFC 0014.
- `human_checkpoint` blockers are already distinct from
  `blocked` blockers, so "this is genuinely waiting on a human"
  vs "this is waiting on a silent failure" is a one-line query.
- `doctor --verbose` already structures problems by check name,
  so an autonomous policy can grep for known-recoverable shapes
  and act on them.

What's missing is **an autonomous policy layer** that decides
*when* to invoke those primitives without an operator at the
keyboard. Three concrete failure modes the V1 of this RFC must
prevent:

1. **Silent stale review-only work.** A reviewer's process
   crashed, its lease expired, and the work would be safe to
   requeue. Today nothing requeues it until an operator runs
   `recovery stale-leases --json`. Autonomously requeuing this
   class is already safe per D036 and should be the default.
2. **Silent process death without reconciliation.** A process
   adapter child was killed externally; `process_executions` is
   still `running`; no operator is around to run
   `recovery process-reconcile`. The downstream block ages out
   slowly; the run looks alive in `status` but nothing is
   moving.
3. **Indefinite waits on `human_checkpoint`.** A run hits a
   `human_checkpoint` blocker at 2am. The blocker is *legitimate*
   — it really does need a human — but with no escalation
   policy the run sits silently until someone notices. A simple
   "ping a webhook / write a marker file / exit non-zero after
   N hours" would let an external alerting layer act.

This RFC documents *both* the observation (overnight runs need
autonomous recovery for the safe classes) and the design (an
opt-in `recovery auto` mode plus a sweeper subcommand and an
escalation hook).

## Goals

- **Autonomous review-only requeue.** A new mode that runs the
  existing `recovery requeue-stale` against every stale
  review-only lease the runner finds, without operator
  intervention. Already approved by D036; this RFC just adds
  the loop.
- **Autonomous process reconciliation.** Same shape against
  `recovery process-reconcile` for `process_executions.state =
  'running'` rows whose pid is gone or whose lease has expired.
- **Bounded retry budget.** Each job has an attempt counter and
  a workflow-declared `max_attempts`; autonomous requeue
  honors it. A job that has burned its retry budget escalates
  rather than silently re-failing.
- **Genuine-vs-silent classifier.** The runner already
  distinguishes `severity: blocked` from
  `severity: human_checkpoint`. The autonomous mode treats
  `human_checkpoint` as legitimate (escalates after a
  configurable timeout) and `blocked` as suspect (attempts
  recovery first, escalates on failure).
- **Escalation hook.** When a run is *truly* stuck — autonomous
  recovery exhausted, no further options — the runner emits a
  structured event and (opt-in) writes a marker file at a
  workflow-declared path, executes a workflow-declared shell
  hook, or hits a workflow-declared webhook. Boundary-respecting:
  the marker file is a downstream concern, the runner just
  emits the signal.
- **`recovery auto` subcommand.** An operator-facing one-shot
  that does a single sweep and exits. Composable with `cron`,
  `systemd timer`, or the operator's own scheduling.
- **`recovery watch` subcommand (opt-in daemon).** A long-lived
  process that sweeps on a fixed cadence. Overnight-run-safe by
  construction.
- **Policy is workflow-declared, not runner-declared.** Different
  workflows have different appetites for autonomy. The runner
  ships sane defaults; per-workflow override via a new
  top-level `recovery_policy` block.
- **Zero regression for today's flow.** Workflows that omit
  `recovery_policy` get the existing operator-driven semantics
  byte-for-byte. `recovery auto` is opt-in; no daemon runs by
  default.

## Non-Goals

- A hosted autonomous-recovery service. D020 stands; the loop
  runs on the operator's machine.
- Healing repo-write stale work. D036 still refuses autonomous
  repo-write requeue; the worktree may be partially modified.
  Repo-write recovery stays operator-driven.
- Automatic agent restart. The agent CLI is a separate concern
  (RFC 0009 supervisor / RFC 0010 harness profiles). This RFC
  does not respawn dead agents; it requeues their *work* so a
  future session (started by anyone) can pick it up.
- Cross-run policy inference (e.g., "this workflow has had 3
  reruns in a row, pause it"). That's a meta-analytics layer.
- Replacing `doctor`. Doctor remains the structured-problem
  surface; the autonomous mode *consumes* doctor output, it
  doesn't replace it.
- Replacing `human_checkpoint` with autonomous decisions. A
  legitimate checkpoint stays a checkpoint; this RFC only
  *escalates* unanswered checkpoints, never resolves them.
- Auto-cancellation of jobs that exceed the retry budget. The
  V1 escalation is "emit signal, surface in doctor, mark
  blocker"; auto-cancellation needs a separate decision because
  it destroys work. (V1.5 may add it behind an explicit flag.)

## Proposal

V1 ships in three landable steps.

### Step 1. `recovery auto --run-id <id>` one-shot sweep

A new CLI verb that performs a single autonomous-recovery pass
on the given run and exits. No daemon, no retries — composable
with `cron`, `systemd timer`, `watch -n 60`, or an operator's
overnight wrapper script.

```text
striatum recovery auto
  --run-id <id>
  [--dry-run]                      # report what it would do, change nothing
  [--max-requeue <n>]              # cap requeues per sweep (default: 8)
  [--checkpoint-timeout <seconds>] # escalate human_checkpoint after N seconds
                                   # (default: per-workflow, then per-runner default 14400)
  [--escalation-hook <path>]       # override workflow-declared hook
  [--json]
```

Sweep order (deterministic):

1. Run `expire_leases(conn, run_id=run)` — same as today.
2. Run `process_reconcile(conn, run_id=run)` — RFC 0014 logic;
   transitions externally-killed processes to `lost`.
3. For each `stale_lease` review-only job (D036 already
   permits this autonomy), run `requeue_stale(conn, run_id,
   job_id)` until either the queue is empty or `max-requeue`
   is hit.
4. For each `human_checkpoint` blocker open longer than
   `checkpoint-timeout`, run the escalation hook (see step 3).
5. For each `blocked` blocker that the runner classifies as
   *recoverable* (open > N seconds, no progress events, no
   active lease), surface in doctor with a new check
   `blocker_recovery_eligible` and emit a
   `recovery.eligible` event.
6. Return a structured envelope `{"swept_at": ..., "actions":
   [...], "escalations": [...], "still_stuck": [...]}`.

`--dry-run` runs the same logic but emits the envelope without
mutating state. Useful for cron sanity-checks.

### Step 2. `recovery_policy` block on workflows

```json
{
  "recovery_policy": {
    "autonomous_review_requeue": true,
    "autonomous_process_reconcile": true,
    "checkpoint_timeout_seconds": 14400,
    "max_requeues_per_sweep": 8,
    "max_total_requeues_per_job": 3,
    "escalation_hook": {
      "kind": "marker_file",
      "path": "docs/runs/<run_id>/STALLED.md"
    }
  }
}
```

Validator additions:

- All fields optional. Omitted block → today's behavior
  preserved byte-for-byte.
- `autonomous_*` fields are booleans; default `false` (opt-in).
- `checkpoint_timeout_seconds` is a non-negative int; default
  14400 (4 hours).
- `max_*` fields are non-negative ints with defaults 8 and 3.
- `escalation_hook.kind` ∈ `{marker_file, webhook, shell}`;
  required when the block is present.
- `escalation_hook.path` (for `marker_file`): repo-relative,
  must be inside `write_scope.allowed_paths` of *no* job (this
  is a runner-owned write). The validator surfaces as a lint
  warning if the path collides with a job's expected_artifact.
- `escalation_hook.url` (for `webhook`): must be `http://` or
  `https://`; the runner POSTs a small JSON envelope.
- `escalation_hook.command` (for `shell`): the command and
  args; runs through the same process adapter the rest of the
  runner uses, with the same constraint enforcement.

### Step 3. `recovery watch --run-id <id>` opt-in daemon

A long-lived process that sweeps every N seconds. Wraps
`recovery auto` in a loop; same envelope per sweep emitted as
events. Honors SIGTERM/SIGINT for clean shutdown.

```text
striatum recovery watch
  --run-id <id>
  [--interval-seconds <n>]   # default: 60
  [--exit-on-terminal]       # exit when run hits terminal state
  [--max-sweeps <n>]         # cap total sweeps; default: unbounded
  [--json]                   # one envelope per sweep, JSONL
```

By default, the watcher exits when the run hits a terminal
state. `--max-sweeps` caps lifetime for paranoid CI/cron use.
PID file at `.striatum/scratch/recovery-watch-<run_id>.pid`,
single-instance per run (refuses with exit 4 if another
watcher is already active).

### Step 4 (deferred). `--auto-cancel-budget-exhausted`

When a job exceeds `max_total_requeues_per_job`, V1 escalates
but does not cancel. V1.5 could add an opt-in
`--auto-cancel-budget-exhausted` flag that calls
`recovery cancel-job --reason "auto: retry budget exhausted"`.
Defer because it destroys work.

## Acceptance Criteria

- A workflow that omits `recovery_policy` produces packets,
  runs, and recovery behavior byte-identical to v1.3.0.
- `recovery auto --run-id <id> --dry-run` produces a structured
  envelope listing actions it would take but mutates nothing.
- `recovery auto --run-id <id>` requeues every stale-lease
  review-only job up to `max-requeue`, transitions
  externally-killed `process_executions` to `lost`, and emits
  a `recovery.swept` event with the action list.
- A `human_checkpoint` blocker open longer than
  `checkpoint-timeout` triggers the escalation hook exactly
  once per sweep until the blocker resolves.
- The marker-file hook writes to the declared path with a small
  Markdown body summarizing the stall (`run_id`, `job_id`,
  `blocker_id`, `severity`, `opened_at`, `proposed_action`).
  The runner refuses paths inside `.striatum/`.
- The webhook hook POSTs a JSON envelope; failure to reach the
  URL emits a `recovery.escalation_failed` event but does not
  abort the sweep.
- The shell hook runs through the existing process adapter with
  the lane's constraint enforcement; D028 preserved (no
  transcripts captured by default).
- `recovery watch --run-id <id> --interval-seconds 5
  --max-sweeps 3` runs three sweeps at 5-second intervals and
  exits 0.
- A `recovery watch` invocation against an already-watched run
  exits 4 with a clear "another watcher is already active"
  message.
- A new `doctor` check `blocker_recovery_eligible` fires for
  blockers that V1 considers eligible for autonomous action.
- A new top-level `recovery_policy` validator rejects unknown
  hook kinds, repo-internal marker-file paths, and negative
  numbers.
- Tests at `tests/test_recovery_auto.py` and
  `tests/test_recovery_watch.py` cover: stale-lease sweep,
  process-reconcile sweep, checkpoint escalation, retry-budget
  enforcement, hook kinds, daemon lifetime, single-instance
  enforcement, dry-run idempotency, and the
  no-policy-no-change regression.
- `__version__` and `pyproject.toml` bump on landing
  (V1 → 1.4.0 or alongside whichever V1 slice lands).

## Open Questions

- **Per-job recovery overrides.** Today the policy is
  workflow-wide. Some jobs (a long-running build) might want
  different retry budgets than others (a quick review). V1
  ships workflow-wide; per-job override is a V1.5 follow-up
  that fits cleanly into the existing job-fields validator.
- **Cross-run watcher.** A single `recovery watch` per run is
  the V1 shape. A `recovery watch --all` that sweeps every
  active run is appealing for development environments but
  raises ordering questions (which run gets attention first)
  that V1 sidesteps by being one-watcher-per-run.
- **Escalation hook idempotency.** A poorly-written hook may
  fire repeatedly when the blocker stays open. V1 fires the
  hook once per sweep until the blocker resolves; the operator
  is responsible for hook idempotency (a marker file the hook
  checks before re-writing). V1.5 could add per-blocker
  "already-escalated" tracking.
- **Webhook authentication.** V1's webhook is unauthenticated;
  the operator is expected to put the URL behind their own
  reverse proxy / network policy. V1.5 could add HMAC signing.
- **Daemon resource use.** A single long-lived sweeper for a
  single run is cheap. A development environment with 50
  active runs would want a single sweeper across all of them.
  Out of scope for V1.
- **Repo-write retry policy.** This RFC explicitly does NOT
  autonomously requeue repo-write work (D036). A future RFC
  could add this *only* against worktree-isolated jobs (RFC
  0008), where partial-modification risk is contained to the
  worktree and the runner can `worktree release` then
  `worktree create` to start clean.
- **Interaction with RFC 0018 (adversarial review postures).**
  When a posture-required build's review fails or stalls, the
  autonomous requeue should respect posture coverage —
  re-running a security-posture review preserves the posture.
  RFC 0018's `submit-review` already records posture; this RFC
  inherits the existing behavior.

## Implementation Path

V1 ships in three landable steps:

1. **Step 1**: `recovery auto` subcommand. New module
   `src/striatum/recovery/auto.py`; new CLI parser entry; new
   doctor check `blocker_recovery_eligible`. Tests.
2. **Step 2**: `recovery_policy` workflow block. New validator
   in `workflow.py`; per-workflow defaults consumed by step 1's
   sweeper. Hook implementations (marker_file, webhook, shell).
   Tests.
3. **Step 3**: `recovery watch` daemon. New CLI parser entry;
   single-instance pidfile; signal handling; emits one envelope
   per sweep. Tests.

RFC 0020 is "accepted" once steps 1 and 2 land. Step 3 is a
quality-of-life add-on; cron + step 1 covers the overnight-run
case without it.

## Relationship To Other RFCs

- **D036 (lazy-on-CLI lease expiry)** is the pin: this RFC
  *adds an autonomous mode* without changing the operator-
  driven default.
- **RFC 0014 (process adapter completion guarantees)** —
  `recovery process-reconcile` is the primitive step 1 calls;
  no schema change needed.
- **RFC 0009 (long-lived process supervision)** — supervisors
  that go `lost` are surfaced by this RFC's escalation hook
  (already a doctor check, just adds the autonomous response).
- **RFC 0018 (adversarial review postures, proposed)** —
  posture-required builds inherit posture coverage during
  autonomous requeue; nothing to change here, but worth
  noting for future re-review attempts.
- **RFC 0019 (DDD foundations, proposed)** — autonomous
  recovery sits squarely inside striatum's bounded context
  (coordination), not the agent's domain (work content). The
  RFC adds new aggregates (sweep envelope, escalation
  records), not new model concepts.
- **RFC 0008 (worktree isolation)** — the deferred
  repo-write-recovery follow-up depends on worktree isolation
  to bound partial-modification risk; mentioned in Open
  Questions for future work.

## Domain Modeling

This RFC adds two new concepts:

- **`sweep envelope`** — value object emitted by each
  `recovery auto` invocation: `{swept_at, actions[],
  escalations[], still_stuck[]}`.
- **`recovery_policy`** — value object on the workflow
  aggregate: thresholds and toggles that govern autonomous
  behavior. Optional; defaults preserve today's flow.

Both are pure value objects; no new SQLite tables, no
mutations to existing aggregates. The autonomous loop *invokes*
existing CLI verbs; it does not bypass the CLI-as-only-write-
surface invariant (RFC 0019).
