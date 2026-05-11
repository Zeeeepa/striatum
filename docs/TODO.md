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
| 6 | Artifact schema (front matter) | ✅ 7 kinds + open registry |
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
| 18 | Workflow type catalog and chooser | ⏳ open |
| F1 | Run historical bootstrap as runner workflow | ⏳ open |
| F2 | Fuller publication policy | ⏳ open |
| F3 | Round-6 RFC 0002 + 0003/0004/0005 follow-up | ✅ done |
| F4 | RFC 0010 V1 (tool harness profiles, dogfood-003) | ✅ done |
| F5 | RFC 0014 V1 (process adapter completion guarantees, dogfood-005) | ✅ done |
| F6 | RFC 0012 V1 (local service API, dogfood-006) | ✅ done |
| F7 | RFC 0013 V1 (local web UI, dogfood-007) | ✅ done |
| F8 | RFC 0016 V1 (dashboard dependency graph, dogfood-008) | ✅ done |
| F9 | RFC 0015 V1 (self-contained agent skills, dogfood-009) | ✅ done |
| F10 | RFC 0017 V1 (README + docs reorganization, dogfood-010) | ✅ done |
| F11 | RFC 0015 step 3 (codex + gemini skill profiles, dogfood-011) | ✅ done |
| F12 | RFC 0016 step 3 (Unicode fancy + --graph-orient, dogfood-012) | ✅ done |
| F13 | RFC 0013 step 7 (web UI mutation buttons, dogfood-013) | ✅ done |
| F14 | RFC 0020 V1 (autonomous recovery sweeper, dogfood-014) | ✅ done |
| F15 | RFC 0020 step 3 (`recovery watch` daemon, dogfood-015) | ✅ done |
| F16 | RFC 0018 V1 (review postures, dogfood-016) | ✅ done |
| F17 | RFC 0021 V1 (DDD layout scaffold, dogfood-017) | ✅ done |
| F18 | RFC 0018 step 3 V1.5 (verdicts.posture + introspection, dogfood-018) | ✅ done |
| F19 | RFC 0021 V1.5 (--force + --dry-run, dogfood-019) | ✅ done |
| F20 | RFC 0022 V1 (web UI redesign, dogfood-020) | ✅ done |
| F21 | RFC 0023 V1 (web chat + view + artifact md, dogfood-021) | ✅ done |
| F22 | RFC 0023 V1.5 (chat tool use + briefing, dogfood-022) | ✅ done |
| F23 | RFC 0024 V1 (workflow browser, dogfood-023) | ✅ done |
| F24 | RFC 0024 V1.5 (visual builder, dogfood-024) | ✅ done |
| F25 | RFC 0024 V2 (run-now + If-Match + field-level errors, dogfood-025) | ✅ done |
| F26 | RFC 0024 V3 (cancel run + dirty-tree visibility, dogfood-026) | ✅ done |
| F27 | RFC 0024 V4 (pause/resume + per-job cancel/retry, dogfood-027) | ✅ done |
| F28 | RFC 0025 V1 Step 1 (claude_code plugin, dogfood-028) | ✅ done |
| F29 | RFC 0025 V1 Steps 2+3 (codex + gemini plugins, dogfood-029) | ✅ done |
| F30 | RFC 0026 V1 + RFC 0027 Phase 2 guardrails (dogfood-030) | ✅ done |
| F31 | RFC 0028 V1 registry-backed multi-repo read/sweep slice (dogfood-031) | ✅ done |

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
   machine-validated; per-kind front-matter schemas exist for seven kinds:
   `decision` (`striatum.decision.v1`), `finding` (`striatum.finding.v1`),
   `findings_ledger` (`striatum.findings_ledger.v1`), `synthesis`
   (`striatum.synthesis.v1`), `support_ledger` (`striatum.support_ledger.v1`,
   RFC 0003), `action_item_ledger` (`striatum.action_item_ledger.v1`, RFC
   0004), and `harness_improvement_proposal`
   (`striatum.harness_improvement_proposal.v1`, RFC 0005). Migration version
   5 dropped the SQL `CHECK` on `artifact_kind`; allowed kinds now live in
   `striatum.artifacts.ALLOWED_ARTIFACT_KINDS` and are enforced by both
   `publish-artifact` (exit 6) and workflow validation (exit 8). The
   publisher records artifacts rather than rewriting them. **Remaining:**
   schemas for additional kinds as future RFCs emerge.

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

18. **Workflow type and lane catalog chooser.** The web UI can already
    browse, edit, run, cancel, pause/resume, and retry workflows via
    RFC 0024, but the product still lacks a first-class choice surface
    for "which workflow should I start from, and which lanes should run
    it?" Promote the documented workflow types and lane-set options in
    `docs/WORKFLOW_TYPES.md` into a small metadata catalog, expose CLI
    verbs such as `workflow templates list/show` and
    `workflow init --template <id>`, then add a UI chooser that
    generates a workflow and opens it in the existing visual builder.

## Immediate Follow-Up

F1. Exercise the minimal process adapter on a Striatum-owned version of the
    historical bootstrap workflow, then retire the tmux harness from active
    workflow guidance.

F2. Define any fuller publication policy after the initial package smoke,
    typecheck, metadata check, and macOS/Linux CI wiring.

F3. ~~Land the round-6 follow-up integrations.~~ Done: RFC 0002 landed
    (D051) — reviewer-policy workflow fields plus work-packet exposure plus
    RFC 0014 fixture labels. RFCs 0003/0004/0005 landed (D052/D053/D054) —
    migration v5 opens `artifacts.artifact_kind` to Python validation, three
    new kinds (`support_ledger`, `action_item_ledger`,
    `harness_improvement_proposal`) registered with v1 front-matter schemas,
    workflow + publish validation reject unknown kinds, and
    `examples/support-ledger-flow/` ships as the reference fixture.

F30. ~~Land lane-liveness attestation and provenance-mode guardrails.~~ Done:
    unattested sessions now publish under `author: operator`, operator labels
    are constrained and self-declared, review jobs can require an attached lane
    supervisor, and `sealed_patch` workflows validate structurally but refuse
    to start until real containment exists.

F31. ~~Land the RFC 0028 V1 daemon acceptance slice.~~ Done: optional
    `striatumd` / `striatum daemon start`, daemon registry, repo
    add/list/remove with explicit `--init`, explicit daemon read routing,
    global dashboard, resources-only daemon MCP with explicit token
    parameters and repo-scope filtering, metadata-only audit with segment
    checks, and foreground recovery sweep events bylined
    `striatumd-<instance-id>` are in place. Deferred: daemon-owned
    supervision, MCP mutation tools, sealed apply/signing, cross-repository workflows,
    service-manager install, Windows daemon support, and operator tenancy.
