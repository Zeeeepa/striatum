# striatum TODO

Status: active
Date: 2026-05-07
author: coordinator-codex-gpt-5.5-001

This list tracks product improvements after the V1 split. Engram remains the
first validation fixture, not the product boundary. Numbered IDs are stable
so external references keep resolving even as items move between sections.

## Completed

### Repo Split (2026-05-07)

R1. Public repository and Python distribution name: `striatum`. Tagged the
    Engram extraction point as `striatum-extraction-2026-05-07` and produced
    a history-preserving split from Engram's former `agent-runner/` prefix.

R2. Standalone repo root scaffolded with `src/`, `tests/`, `docs/`,
    `examples/`, `prompts/`, `scripts/`, `README.md`, `Makefile`,
    `pyproject.toml`, and `.gitignore`. Engram dogfood material is retained
    as redacted validation history.

R3. Removed `TARGET_REPO=..` as the primary usage path. Replaced with the
    `--repo` CLI flag pattern.

R4. Standalone metadata: Apache-2.0 license, contribution notes, changelog,
    supported Python versions, CI, and `scripts/fresh_clone_smoke.sh`.

R5. Engram retains the incubation copy as historical provenance plus a
    pointer to the standalone repository until the owner chooses how to
    archive, subtree, or submodule it.

### Product Improvements (delivered)

5. Decision-artifact support for owner choices. `striatum decision record`
   writes durable Markdown with `striatum.decision.v1` front matter for
   `accepted`, `rejected`, and `accepted_with_follow_up` outcomes, no active
   lease required.

9. Compact terminal dashboard. `striatum dashboard --run-id <id>
   [--refresh N] [--once]` renders a single-screen view of run state, job
   counts, verdicts, blockers, claimable work, next actions, and recent
   events using only the standard library.

11. Per-job git worktree isolation (RFC 0008). Lanes opt in with
    `worktree_isolation: per_job`; work packets carry `worktree_required:
    true` and the `striatum worktree create` invocation. Migration version 2
    adds `job_worktrees`. `publish-artifact` reads from the worktree but
    records the logical repo-relative path; lease expiry marks worktrees
    `abandoned` for operator inspection; `doctor` flags orphaned and
    missing-on-disk worktree rows.

12. Richer fixture suite beyond Engram: generic docs-only review,
    code-change with one-shot needs_revision cycle, single-review failed
    revision opening a configured human checkpoint, explicit human
    checkpoint resolved by `decision record`, and adapter-unavailable
    rejected at validation. All listed gaps delivered.

15. `run summary` Markdown groups verdicts by review job with attempt
    counts, appends the structured author byline to each artifact, surfaces
    recorded vs. current git branch with `(MISMATCH)` when they differ, and
    prints a Timing block (`created_at`, `started_at`, `completed_at`,
    wall-clock `duration`).

17. SQLite migration system (RFC 0006). Schema version is tracked through
    `PRAGMA user_version`; `striatum init` and every connect apply pending
    migrations inside a single `BEGIN IMMEDIATE` transaction; databases
    newer than the runner exit with code 9.

## In Progress

1. **Process adapter**. Single-shot `adapter run` is shipped. Long-lived
   supervision (RFC 0009) landed: `process_supervisors` (migration version
   4), `striatum supervise start | send | stop | status | list`, lazy
   lease-expiry recovery that flags supervisors `lost` without auto-killing
   the OS process, `doctor` checks for dead pids and missing stdin pipes.
   Remaining: supervised-aware `claim-next` that routes packets through
   `supervise send` instead of a fresh single-shot launch, PTY support for
   CLIs that refuse non-tty stdin, broader adapter recovery UX.

2. **Adapter constraint enforcement**. Workflow validation supports lane
   `required_enforcement` and rejects lanes whose adapters cannot satisfy
   it. The four-level model (`enforced`, `advisory_strict`, `advisory`,
   `unsupported`) is in place; the process adapter graduates `network` and
   `repo_scope` to `advisory_strict`. Remaining: richer sandbox/worktree
   adapters that can mechanically enforce more than transcript-off.

3. **Workflow authoring tooling**. Dry-run planner, Mermaid/JSON graph
   export, and `workflow init` (`minimal`, `review`, `code-change` styles)
   are shipped. Validator covers cross-job artifact path collisions,
   write-scope/forbidden-path overlap, artifact-in-write-scope, unsound
   cycle target, parallel-group repo_write/review-only consistency, and a
   `needs` deprecation warning. Remaining: linting output and path
   rewriting for reruns.

4. **Human-checkpoint UX**. `status` and `why` include decision context,
   affected jobs, unblock path, and next actions. Remaining: keep refining
   evidence export and any explicit resume flow.

6. **Artifact schema support**. Optional Markdown `author:` metadata is
   machine-validated; per-kind front-matter schemas exist for `decision`
   (`striatum.decision.v1`), `finding` (`striatum.finding.v1`),
   `findings_ledger` (`striatum.findings_ledger.v1`), and `synthesis`
   (`striatum.synthesis.v1`). The publisher records artifacts rather than
   rewriting them. Remaining: schemas for additional kinds as use cases
   emerge.

7. **Redaction tests**. Default-deny evidence-export policy registry is in
   place. Continue extending coverage for workflow titles, job prompts,
   model rationales, blocker text, transcript-like fields, and path
   hygiene.

8. **Recovery commands**. `recovery stale-leases` and `recovery
   requeue-stale` distinguish repo-write from review-only and refuse
   repo-write requeues. Remaining: abandoned write jobs (likely via
   worktree-isolated recovery), blocked review cycles, rerun attempts,
   explicit operator resume flows.

10. **Local API and MCP**. `striatum.api.invoke` wraps the same
    parser/dispatcher; the local stdio JSON-RPC wrapper speaks
    `Content-Length` framing with line-delimited fallback and maps
    tools/resources to the API. Remaining: any richer MCP schema polish
    that emerges from real local agent use.

13. **Replace bootstrap scripts with runner-owned workflows**. The minimal
    process adapter and supervised sessions cover claimed-work launch.
    Remaining: represent the historical bootstrap design workflow end-to-end
    with the runner so the tmux harness can retire from active guidance.

14. **Packaging and release**. `ruff`, `mypy`, wheel/sdist build, installed
    console-script smoke, installed package metadata check, macOS/Linux CI
    are in place. Remaining: any fuller publication policy needed before a
    public release.

## Open

16. **Keep generic language current**. New docs should say "target
    repository", "workflow fixture", "runner state", "artifact", and
    "adapter" rather than assuming Engram-specific paths or marker names.
    No active sub-task.

## Immediate Follow-Up

F1. Exercise the minimal process adapter on a Striatum-owned version of
    the historical bootstrap workflow, then retire the tmux harness from
    active workflow guidance.

F2. Define any fuller publication policy after the initial package smoke,
    typecheck, metadata check, and macOS/Linux CI wiring.
