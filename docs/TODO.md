# striatum TODO

Status: active
Date: 2026-05-07
author: coordinator-codex-gpt-5.5-001

This list records completed split work and product improvements that should
follow. Engram remains the first validation fixture, not the product boundary.

## Repo Split TODOs

Completed on 2026-05-07:

1. Chosen public repository and Python distribution name: `striatum`.
2. Tagged the Engram extraction point as `striatum-extraction-2026-05-07`.
3. Created a history-preserving split from Engram's former `agent-runner/`
   prefix.
4. Prepared the standalone repository root with `src/`, `tests/`, `docs/`,
   `examples/`, `prompts/`, `scripts/`, `README.md`, `Makefile`,
   `pyproject.toml`, and `.gitignore`.
5. Kept redacted Engram dogfood material as validation history and external
   reference fixtures. The generic `rfc-ledger-cleanup` example remains the
   first walkthrough.
6. Left private diagnostics, transcripts, `.striatum/state.sqlite3`, caches,
   virtual environments, and target-repo runtime state out of the split.
7. Removed `TARGET_REPO=..` as the primary usage path.
8. Added standalone metadata: Apache-2.0 license status, contribution notes,
   changelog, supported Python versions, and CI.
9. Added `scripts/fresh_clone_smoke.sh` to install a fresh clone, initialize a
   scratch target repo, validate the generic example workflow, prepare and
   start a run, and export redacted evidence.
10. Recorded the split decision here. Engram keeps the incubation copy as
    historical provenance plus a pointer to the standalone repository until
    the owner decides whether to remove, subtree, or submodule it.

## Product Improvement TODOs

1. Expand the generic process adapter beyond the minimal local command runner
   added on 2026-05-07. The current slice can launch configured process lane
   command arrays for claimed work and record process metadata/events without
   transcript capture. Long-lived interactive supervision (RFC 0009) landed
   on 2026-05-07: a new `process_supervisors` table (migration version 4),
   `striatum supervise start | send | stop | status | list` commands that
   fork the lane command in a new session and deliver work packets through
   a per-supervisor named pipe, lazy lease-expiry recovery that flags
   supervisors `lost` without auto-killing the OS process, and `doctor`
   checks for dead pids and missing stdin pipes. Supervised-aware
   `claim-next` was delivered on 2026-05-07: when the claiming session has
   an `attached` supervisor, `claim_next` writes the freshly built packet
   through the supervisor's stdin pipe inside the same write transaction,
   records a `supervisor.packet_delivered` event, and surfaces a
   `supervisor_delivery: {supervisor_id, bytes_written}` field in the
   response (or `{supervisor_id, error: "stdin_pipe_missing"}` with the
   supervisor transitioned to `lost` if the pipe is gone). Sessions without
   a supervisor see no new field. Remaining work is PTY support for CLIs
   that refuse non-tty stdin and broader adapter recovery UX.
2. Continue adapter constraint enforcement. Workflow validation now supports
   lane `required_enforcement` on 2026-05-07, records requested and actual
   enforcement as `enforced`, `advisory`, or `unsupported`, and rejects lanes
   whose adapters cannot satisfy the declared requirement. Remaining work is
   richer sandbox/worktree adapters that can enforce more than transcript-off.
3. Continue workflow authoring tooling. A dry-run planner was added on
   2026-05-07 to explain claim order, graph edges, review gates, and revision
   cycles. Mermaid/JSON graph export was added on 2026-05-07. Validator
   additions on 2026-05-07: cross-job artifact path collisions,
   write-scope/forbidden-path overlap, artifact-in-write-scope, unsound
   cycle target, parallel-group repo_write/review-only mode consistency,
   plus a `needs` deprecation warning. `workflow init [--style] <path>`
   landed on 2026-05-07 and writes a validating starter tree
   (`workflow.json` plus role/prompt stubs) for `minimal`, `review`, and
   `code-change` styles. Remaining work includes linting output and path
   rewriting for reruns.
4. Continue human-checkpoint UX. `status` and `why` now include decision
   context, affected jobs, unblock path, and next actions. The explicit
   resume flow landed on 2026-05-07 as `striatum checkpoint resolve
   --blocker-id <id> --action {continue|cancel} [--decision-id <id>]`:
   `continue` closes the blocker and re-queues the affected job; `cancel`
   closes the blocker, marks the job canceled, and lets `maybe_complete_run`
   finalize the run when no other work remains. Optional `--decision-id`
   validates a run-level decision artifact and records its id on the
   `checkpoint.resolved`/`checkpoint.canceled` event payload so audit can
   link the resolution to the operator's decision artifact. Remaining work
   is any further evidence-export polish that emerges from real operator
   use.
5. Explicit decision-artifact support for owner choices was added on
   2026-05-07. `decision record` writes durable Markdown with
   machine-checkable `striatum.decision.v1` front matter for "accepted",
   "rejected", and "accepted with follow-up" outcomes, records it as artifact
   kind `decision`, and does not require an active lease.
6. Continue artifact schema support. Optional Markdown `author:` metadata is
   machine-validated when present, including YAML front matter and title-block
   lines, while preserving the current rule that the publisher records
   artifacts rather than rewriting them. Remaining work includes fuller
   front-matter schemas for artifact kinds beyond author metadata.
7. Extend redaction tests for evidence export and artifact publication. Cover
   workflow titles, job prompts, model rationales, blocker text, transcript-like
   fields, and path hygiene.
8. Continue recovery commands. `recovery stale-leases` reports expired lease
   recovery context and distinguishes repo-write work from review-only work.
   `recovery requeue-stale` provides a bounded operator requeue for expired
   non-repo-write jobs and refuses repo-write jobs. Explicit operator cancel
   landed on 2026-05-07 as `striatum recovery cancel-job --run-id <id>
   --job-id <id> --reason <text> [--cascade]`: it releases active/expired
   leases, marks the queue message canceled, transitions the job to
   `canceled`, emits `job.canceled`, and calls `maybe_complete_run`.
   Terminal-state jobs are refused with exit code 4; jobs with blocked
   dependents whose only path was through this one are refused unless
   `--cascade` is set, in which case dependents are canceled transitively
   in the same transaction. Remaining work includes rerun-attempt support
   and any further worktree-isolated abandoned-write recovery beyond the
   bounded paths above.
9. Compact TUI / local dashboard over the existing SQLite state was delivered
   on 2026-05-07: `striatum dashboard --run-id <id> [--refresh N] [--once]`
   renders a single-screen view of run state, job counts, verdicts, blockers,
   claimable work, next actions, and recent events using only the standard
   library.
10. Continue local API/MCP support. `striatum.api.invoke` now wraps the same
    CLI parser/dispatcher without direct SQLite writes, `docs/SPEC.md`
    defines the MCP boundary, and a minimal local stdio JSON-RPC wrapper maps
    tools/resources to that API. Remaining work is any richer MCP schema polish
    that emerges from real local agent use.
11. Per-job git worktree isolation for parallel repo-write jobs landed on
    2026-05-07 (RFC 0008). Lanes opt in with `worktree_isolation: per_job`;
    work packets for repo-write jobs in those lanes advertise
    `worktree_required: true` and the `striatum worktree create` invocation,
    `publish-artifact` reads from the per-job worktree but records the logical
    repo-relative path, lease expiry marks the worktree `abandoned` for
    operator inspection, and `striatum doctor` flags orphaned and
    missing-on-disk worktrees. Migration version 2 adds the `job_worktrees`
    table.
12. Build a richer fixture suite beyond Engram. Added on 2026-05-07: a small
    generic docs-only review flow, a code-change flow with a one-shot
    needs_revision cycle (`examples/code-change-flow/`), a single-review
    failed-revision flow that opens a configured human checkpoint
    (`examples/failed-review-revision-cycle/`), an explicit human-checkpoint
    flow whose decide job surfaces an operator checkpoint via
    `block --severity human_checkpoint` and is resolved with
    `striatum decision record` (`examples/human-checkpoint-flow/`), and an
    adapter-unavailable flow that requests `network=enforced` on a process
    lane and is rejected at validation with exit code 8
    (`examples/adapter-unavailable-flow/`). All listed gaps delivered.
13. Continue replacing temporary bootstrap scripts with runner-owned workflows.
    A minimal generic process adapter now launches configured local process
    lanes for claimed work and records process metadata/events without
    transcript capture. Remaining work is representing the old bootstrap
    design workflow end-to-end with the runner.
14. Continue packaging and release checks. Started on 2026-05-07 with `ruff`,
    wheel/sdist build, installed console-script smoke testing, and macOS/Linux
    CI wiring. Type checking and installed release metadata checks were added
    on 2026-05-07. Remaining work is any fuller release policy needed before
    publication.
15. Run summaries now have `run summary`, which writes a compact final run note
    with run id, branch, jobs, verdicts, artifacts, blockers, and verification.
    On 2026-05-07 the renderer was extended to group verdicts by review job
    with attempt counts, append the structured author byline to each
    artifact, surface recorded vs. current git branch with an explicit
    `(MISMATCH)` annotation when they differ, and print a Timing block
    (`created_at`, `started_at`, `completed_at`, wall-clock `duration`).
    Remaining work is any formatting polish that emerges from real runs.
16. Keep the generic language current. New docs should say "target repository",
    "workflow fixture", "runner state", "artifact", and "adapter" rather than
    assuming Engram-specific paths or marker names.
17. The local SQLite schema is now migration-aware. `striatum init` and every
    connect to an existing database apply pending migrations from the
    `striatum.migrations` registry inside a single `BEGIN IMMEDIATE`
    transaction, schema version is tracked through `PRAGMA user_version`
    (current `user_version = 1`), and a database newer than the runner
    supports is refused with exit code 9. Remaining work includes any
    data-backfill migrations that arrive with future schema changes and an
    optional `striatum db status` introspection command.

## Immediate Follow-Up

1. Exercise the minimal generic process adapter on a Striatum-owned version of
   the historical bootstrap workflow, then retire the tmux harness from active
   workflow guidance.
2. Define any fuller publication policy after the initial package smoke,
   typecheck, metadata check, and macOS/Linux CI wiring.
