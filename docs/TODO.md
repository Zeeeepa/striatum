# striatum TODO

Status: active
Date: 2026-05-07
author: coordinator-codex-gpt-5.5-001

This list tracks product improvements after the V1 split. Engram remains the
first validation fixture, not the product boundary. Numbered IDs are stable
so external references keep resolving even as items move between sections.

## Status Snapshot

| ID | Item | Status |
|---:|------|:------:|
| R1 | Public repository name and history-preserving split | ✅ done |
| R2 | Standalone repo scaffold | ✅ done |
| R3 | `--repo` flag replaces `TARGET_REPO=..` | ✅ done |
| R4 | Standalone metadata, license, CI, fresh-clone smoke | ✅ done |
| R5 | Engram retains incubation copy + pointer | ✅ done |
| 1 | Process adapter (single-shot + supervised) | 🟡 most done |
| 2 | Adapter constraint enforcement | 🟡 most done |
| 3 | Workflow authoring tooling | 🟡 most done |
| 4 | Human-checkpoint UX | ✅ done |
| 5 | Decision-artifact support | ✅ done |
| 6 | Artifact schema (front matter) | 🟡 4 of N kinds |
| 7 | Redaction tests | 🟡 coverage continues |
| 8 | Recovery commands | ✅ done |
| 9 | TUI / local dashboard | ✅ done |
| 10 | Local API and MCP | ✅ done |
| 11 | Worktree isolation (RFC 0008) | ✅ done |
| 12 | Richer fixture suite | ✅ done |
| 13 | Replace bootstrap scripts | 🟡 most done |
| 14 | Packaging and release | 🟡 most done |
| 15 | `run summary` polish | ✅ done |
| 16 | Keep generic language current | ⏳ open |
| 17 | SQLite migration system (RFC 0006) | ✅ done |
| F1 | Run historical bootstrap as runner workflow | ⏳ open |
| F2 | Fuller publication policy | ⏳ open |

Legend: ✅ done · 🟡 most done (sub-tasks remain) · ⏳ open

## Completed

### Repo Split (2026-05-07)

- ~~**R1.** Public repository and Python distribution name `striatum`. Engram
  extraction tagged `striatum-extraction-2026-05-07`; history-preserving split
  from the former `agent-runner/` prefix.~~

- ~~**R2.** Standalone repo root scaffolded with `src/`, `tests/`, `docs/`,
  `examples/`, `prompts/`, `scripts/`, `README.md`, `Makefile`,
  `pyproject.toml`, `.gitignore`. Engram dogfood material retained as redacted
  validation history.~~

- ~~**R3.** Removed `TARGET_REPO=..` as the primary usage path. Replaced with
  the `--repo` CLI flag pattern.~~

- ~~**R4.** Standalone metadata: Apache-2.0 license, contribution notes,
  changelog, supported Python versions, CI, and
  `scripts/fresh_clone_smoke.sh`.~~

- ~~**R5.** Engram retains the incubation copy as historical provenance plus
  a pointer to the standalone repository until the owner chooses how to
  archive, subtree, or submodule it.~~

### Product Improvements (delivered)

- ~~**4. Human-checkpoint UX.** `status` and `why` carry decision context,
  affected jobs, unblock path, and next actions. `striatum checkpoint resolve
  --blocker-id <id> --action {continue|cancel} [--decision-id <id>]` is the
  explicit operator resume/cancel surface; `continue` requeues the affected
  job, `cancel` transitions it to `canceled`, and the optional `--decision-id`
  links the resolution to a recorded decision artifact.~~

- ~~**5. Decision-artifact support.** `striatum decision record` writes
  durable Markdown with `striatum.decision.v1` front matter for `accepted`,
  `rejected`, and `accepted_with_follow_up` outcomes, no active lease
  required.~~

- ~~**8. Recovery commands.** `recovery stale-leases` and `recovery
  requeue-stale` distinguish review-only from repo-write work and refuse
  repo-write requeues. `striatum recovery cancel-job --run-id <id> --job-id
  <id> --reason <text> [--cascade]` is the explicit operator cancel for
  non-terminal jobs; refuses terminal-state jobs and refuses jobs with
  blocked dependents unless `--cascade` is set.~~

- ~~**9. Compact terminal dashboard.** `striatum dashboard --run-id <id>
  [--refresh N] [--once]` renders a single-screen view of run state, job
  counts, verdicts, blockers, claimable work, next actions, and recent events
  using only the standard library.~~

- ~~**10. Local API and MCP.** `striatum.api.invoke` wraps the same
  parser/dispatcher without direct SQLite writes; the local stdio JSON-RPC
  wrapper speaks `Content-Length` framing with line-delimited fallback and
  maps tools/resources to the API.~~

- ~~**11. Per-job git worktree isolation (RFC 0008, accepted).** Lanes opt
  in with `worktree_isolation: per_job`; work packets carry
  `worktree_required: true` and the `striatum worktree create` invocation.
  Migration version 2 adds `job_worktrees`. `publish-artifact` reads from the
  worktree but records the logical repo-relative path; lease expiry marks
  worktrees `abandoned` for operator inspection; `doctor` flags orphaned and
  missing-on-disk worktree rows.~~

- ~~**12. Richer fixture suite beyond Engram.** Generic docs-only review,
  code-change with one-shot needs_revision cycle, single-review failed
  revision opening a configured human checkpoint, explicit human-checkpoint
  flow resolved by `decision record`, and adapter-unavailable rejected at
  validation. All listed gaps delivered.~~

- ~~**15. `run summary` polish.** Markdown groups verdicts by review job with
  attempt counts, appends the structured author byline to each artifact,
  surfaces recorded vs. current git branch with `(MISMATCH)` when they
  differ, and prints a Timing block (`created_at`, `started_at`,
  `completed_at`, wall-clock `duration`).~~

- ~~**17. SQLite migration system (RFC 0006, accepted).** Schema version
  tracked through `PRAGMA user_version`; `striatum init` and every connect
  apply pending migrations inside a single `BEGIN IMMEDIATE` transaction;
  databases newer than the runner exit with code 9.~~

## In Progress

1. **Process adapter.** Single-shot `adapter run` is shipped. Long-lived
   supervision (RFC 0009, accepted) landed: `process_supervisors` table
   (migration version 4), `striatum supervise start | send | stop | status |
   list`, lazy lease-expiry recovery that flags supervisors `lost` without
   auto-killing the OS process, supervised-aware `claim-next` that
   auto-delivers the freshly built packet through the supervisor's stdin pipe
   and lazily marks pipe-missing/write-fail supervisors `lost`, `doctor`
   checks for dead pids and missing stdin pipes. **Remaining:** PTY support
   for CLIs that refuse non-tty stdin; broader adapter recovery UX.

2. **Adapter constraint enforcement.** Workflow validation supports lane
   `required_enforcement` and rejects lanes whose adapters cannot satisfy it.
   The four-level model (`enforced`, `advisory_strict`, `advisory`,
   `unsupported`) is in place; the process adapter graduates `network` and
   `repo_scope` to `advisory_strict` via proxy-env scrubbing and sentinel env
   vars. **Remaining:** richer sandbox/worktree adapters that can
   mechanically enforce more than transcript-off (network namespacing,
   filesystem isolation beyond cwd-pinning).

3. **Workflow authoring tooling.** Dry-run planner, Mermaid/JSON graph
   export, stateful `striatum run graph --run-id <id>` (RFC 0007, accepted),
   and `workflow init` (`minimal`, `review`, `code-change` styles) are
   shipped. Validator covers cross-job artifact path collisions,
   write-scope/forbidden-path overlap, artifact-in-write-scope, unsound cycle
   target, parallel-group repo_write/review-only consistency, and a `needs`
   deprecation warning. **Remaining:** linting output (style hints distinct
   from validation errors) and path rewriting for reruns.

6. **Artifact schema support.** Optional Markdown `author:` metadata is
   machine-validated; per-kind front-matter schemas exist for `decision`
   (`striatum.decision.v1`), `finding` (`striatum.finding.v1`),
   `findings_ledger` (`striatum.findings_ledger.v1`), and `synthesis`
   (`striatum.synthesis.v1`). The publisher records artifacts rather than
   rewriting them. **Remaining:** schemas for additional kinds as use cases
   emerge (RFCs 0003/0004/0005 propose `support_ledger`,
   `action_item_ledger`, `harness_improvement_proposal` — pending follow-up
   integration).

7. **Redaction tests.** Default-deny evidence-export policy registry is in
   place; new evidence fields default to redacted unless explicitly marked
   safe. **Remaining:** continue extending coverage for workflow titles, job
   prompts, model rationales, blocker text, transcript-like fields, and path
   hygiene with synthetic injection tests.

13. **Replace bootstrap scripts with runner-owned workflows.** The minimal
    process adapter and supervised sessions cover claimed-work launch.
    **Remaining:** represent the historical bootstrap design workflow
    end-to-end with the runner so the tmux harness can retire from active
    guidance.

14. **Packaging and release.** `ruff`, `mypy`, wheel/sdist build, installed
    console-script smoke, installed package metadata check, and macOS/Linux
    CI are in place. **Remaining:** fuller publication policy (PyPI release
    cadence, signing, security disclosure) before a public release.

## Open

16. **Keep generic language current.** New docs should say "target
    repository", "workflow fixture", "runner state", "artifact", and
    "adapter" rather than assuming Engram-specific paths or marker names.
    No active sub-task.

## Immediate Follow-Up

F1. Exercise the minimal process adapter on a Striatum-owned version of the
    historical bootstrap workflow, then retire the tmux harness from active
    workflow guidance.

F2. Define any fuller publication policy after the initial package smoke,
    typecheck, metadata check, and macOS/Linux CI wiring.

F3. Land the round-6 follow-up integrations (RFC 0002 reviewer-independence
    workflow fields; RFCs 0003/0004/0005 new artifact kinds and
    `support-ledger-flow` fixture). The agent commits forked from a stale
    base and would lose post-round-1 work if cherry-picked as-is; redo
    against current `main`. RFC 0002 is now landed (D051): the workflow
    validator accepts `reviewer_access_scope` and `reviewer_context_policy`
    on review jobs, work packets surface a `review_policy` block when the
    fields are declared, and the RFC 0014 fixture labels its root reviews
    as `document_only`/`fresh`. RFCs 0003/0004/0005 remain to be redone.
