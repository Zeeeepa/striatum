# agent_runner TODO

Status: planning
Date: 2026-05-06
author: coordinator-codex-gpt-5.5-001

This list records the practical work needed to split `agent_runner` out of
Engram and the product improvements that should follow. Engram remains the
incubation repository and first validation fixture; it is not the product
boundary.

## Repo Split TODOs

1. Choose the public repository name and Python distribution name. Current code
   and docs use `agent_runner` for the CLI/module and `agent-runner` for the
   package directory.
2. Tag the Engram extraction point after the accepted dogfood artifacts,
   findings, and TODOs are committed.
3. Split preserving history with a prefix split, for example
   `git subtree split --prefix=agent-runner -b agent-runner-split`.
4. Create the standalone remote and push the split branch as the new repository
   main branch.
5. Confirm the split repository root has the intended layout: `src/`,
   `tests/`, `docs/`, `examples/`, `prompts/`, `scripts/`, `README.md`,
   `Makefile`, `pyproject.toml`, and `.gitignore`.
6. Keep Engram dogfood material as validation history, but label it as an
   external reference fixture. The generic `rfc-ledger-cleanup` example should
   remain the first walkthrough.
7. Move or copy only the redacted dogfood artifacts needed for runner history.
   Private diagnostics, transcripts, `.agent_runner/state.sqlite3`, caches,
   virtual environments, and target-repo runtime state stay out of the split.
8. Update paths that assume the parent Engram checkout. In particular, remove
   `TARGET_REPO=..` as the primary usage path and keep it only as incubation
   context where still useful.
9. Add standalone project metadata: license, contribution notes, release notes
   or changelog policy, supported Python versions, and CI.
10. Add a fresh-clone smoke test that installs the package, initializes a
    scratch target repo, validates the generic example workflow, prepares and
    starts a run, and exports redacted evidence.
11. Decide what remains in Engram after the split: a pointer document, a
    submodule/subtree, or no checked-in runner copy. Do not delete the
    incubation history until the standalone repository is verified.
12. Record the split decision in both Engram and `agent_runner` decision logs so
    future agents do not confuse the incubation path with the product boundary.

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

1. Fix the RFC 0014 marker-gate findings recorded in
   `docs/reviews/rfc-0014-operational-artifact-home/BRANCH_REVIEW_codex_2026_05_06.md`.
2. Run the Phase 3 marker-gate tests through the project-managed environment
   rather than relying on a globally installed `pytest`.
3. After the gate fixes are green, tag the extraction point and perform the
   history-preserving split.
