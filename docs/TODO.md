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
8. Added standalone metadata: all-rights-reserved license status, contribution
   notes, changelog, supported Python versions, and CI.
9. Added `scripts/fresh_clone_smoke.sh` to install a fresh clone, initialize a
   scratch target repo, validate the generic example workflow, prepare and
   start a run, and export redacted evidence.
10. Recorded the split decision here. Engram keeps the incubation copy as
    historical provenance plus a pointer to the standalone repository until
    the owner decides whether to remove, subtree, or submodule it.

## Product Improvement TODOs

1. Implement the generic process or tmux adapter that can actually launch and
   supervise configured agent commands. V1 currently tests the deterministic
   state/control-plane contract and does not launch production model processes.
2. Make adapter constraint enforcement first-class. Network policy, transcript
   handling, repo scope, and sandbox expectations should be reported as
   `enforced`, `advisory`, or `unsupported`, with workflows able to reject lanes
   that cannot meet a required enforcement level.
3. Improve workflow authoring tooling: templates, linting, graph validation
   output, path rewriting for reruns, artifact collision checks, and a
   dry-run planner that explains claim order and review gates.
4. Improve human-checkpoint UX. `status`, `why`, and evidence export should make
   the required human decision, affected jobs, and unblock path obvious.
5. Add explicit decision-artifact support for owner choices, including durable
   machine-checkable metadata for "accepted", "rejected", and "accepted with
   follow-up" outcomes.
6. Tighten artifact schema support. Durable Markdown artifacts should have
   optional machine-validated front matter, including the lowercase
   privacy-safe `author:` line, while preserving the current rule that the
   publisher records artifacts rather than rewriting them.
7. Extend redaction tests for evidence export and artifact publication. Cover
   workflow titles, job prompts, model rationales, blocker text, transcript-like
   fields, and path hygiene.
8. Add better recovery commands for stale leases, abandoned write jobs, blocked
   review cycles, and rerun attempts. Recovery should distinguish review-only
   work from repo-write work.
9. Add a compact TUI or local dashboard over the existing SQLite state before
   investing in web or Slack surfaces.
10. Add a local API or MCP adapter only as a wrapper over the CLI/state
    semantics, not as a second source of truth.
11. Support worktree isolation for parallel repo-write jobs so safe build
    parallelism can grow beyond "disjoint write scopes on one branch".
12. Build a richer fixture suite beyond Engram: small docs-only review flow,
    small code-change flow, failed-review revision cycle, human-checkpoint flow,
    and adapter-unavailable flow.
13. Replace temporary bootstrap scripts with runner-owned workflows wherever the
    deterministic core can represent the process.
14. Add packaging and release checks: `ruff`, type checking, wheel build,
    console-script smoke test, and cross-platform tests for macOS and Linux.
15. Make run summaries easier to publish: one command should produce a compact
    final run note with run id, branch, jobs, verdicts, artifacts, blockers, and
    verification.
16. Keep the generic language current. New docs should say "target repository",
    "workflow fixture", "runner state", "artifact", and "adapter" rather than
    assuming Engram-specific paths or marker names.

## Immediate Follow-Up

1. Decide whether to replace the all-rights-reserved license status with an
   open-source license.
2. Replace the temporary tmux bootstrap harness with a Striatum-owned adapter
   workflow.
3. Add packaging checks for wheel build, console-script smoke, and cross-
   platform macOS/Linux execution.
