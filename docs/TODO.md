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
| 3 | Workflow authoring tooling | ✅ done |
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
| 14 | Packaging and release | ✅ done |
| 15 | `run summary` polish | ✅ done |
| 16 | Keep generic language current | ⏳ open |
| 17 | SQLite migration system (RFC 0006) | ✅ done |
| 18 | Workflow type catalog and chooser | ✅ done |
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
| F32 | RFC 0030 + RFC 0031 V2 RPC/supervision/apply foundation (dogfood-034) | ✅ done |
| F33 | RFC 0033 V2 system-Postgres daemon substrate (dogfood-033) | ✅ done |
| F34 | RFC 0032 V2 cross-repo workflows + MCP mutation capabilities (dogfood-035) | ✅ done |
| F35 | RFC 0034 V1 workflow generator + template catalog (dogfood-036) | ✅ done |
| F36 | RFC 0036 V1 MCP harness + chat workflow generation tools (dogfood-038) | ✅ done |
| F37 | RFC 0035 V1 multi-repo test harness (dogfood-037) | ✅ done |
| F38 | RFC 0037 V1 web UI ergonomic improvements (dogfood-039) | ✅ done |
| F39 | RFC 0040 V1 MCP-driven dogfood harness (operator-side slice; dogfood-040) | ✅ done |
| F40 | RFC 0038 V1 web UI feature additions + frontend toolchain (dogfood-041) | ✅ done |
| F41 | RFC 0039 V1 Steps 1+2 Go daemon core (dogfood-042 Track A) | ✅ done |
| F42 | RFC 0044 draft Engram Phase 1 implementation spec (dogfood-042 Track B) | ✅ done |
| F43 | RFC 0042 draft repo-local state to Postgres (dogfood-042 Track C) | ✅ done |
| F44 | RFC 0045 V1 multi-phase workflow schema + React Flow editor (dogfood-043) | ✅ done |
| F45 | RFC 0040 V1.5 daemon-dispatch + composite tools + watcher (dogfood-044) | ✅ done |
| F46 | RFC 0038 V1.5 web UI integration gaps (F1-F4 + supply-chain, dogfood-045) | ✅ done |
| F47 | RFC 0044 V1 Striatum-side corpus export (dogfood-046; Engram-side separate) | ✅ done |
| F48 | RFC 0039 V1.5 Go daemon F1-F5 deltas (dogfood-047; D101 override) | ✅ done |
| F49 | RFC 0043 V1 Postgres-as-sole-substrate + daemon-required (dogfood-048; D102 override) | ✅ done |
| F50 | RFC 0050 V1+V1.5+V2 operator UI rework (dogfoods 054/054b/055/055b/056; v1.46.0-v1.48.0) | ✅ done |
| F51 | v1.48.1 wrapper auth fix — closes 10+ instance claude/gemini no-publish stall (validated by gh-16) | ✅ done |
| F52 | v1.48.2 CI green — Python typecheck + Go matrix pin (6 days of red closed) | ✅ done |
| F53 | `docs/issues/<N>/` GH-issue-driven workflow type (gh-16 first instance, accept verdict) | ✅ done |
| 33 | RFC 0042 V1 run-list workflow identity | ⏳ open |
| 34 | RFC 0046 V1 lane evidence guard at publish-artifact | 🟡 partially active via operator override path |
| 35 | RFC 0047 V1 decision-record propagation + `compromised` run state | ⏳ open |
| 36 | RFC 0048 daemon-side substrate migration (V2.0 phase, A→B→C) | ⏳ open |
| 37 | RFC 0049 interactive claude lane via MCP (experimental, decision needed) | ⏳ open |
| 38 | RFC 0050 follow-ups — GH #9-13 V2 surface findings | ⏳ open |
| 39 | RFC 0051 V1 auto-finalize from frontmatter (downgraded urgency post-v1.48.1) | ⏳ open |
| 40 | GH #14 — recovery cannot clear terminal-run `process_exit_nonzero` blocker | ⏳ open |
| 41 | GH #15 — docs clarify PostgreSQL transition guidance | ⏳ open |
| 42 | GH #17 — Striatum doc consistency for Engram memory integration | ⏳ open |

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

- ~~**3. Workflow authoring tooling.** All authoring verbs ship: `workflow
  validate`, `plan`, `graph` (mermaid/json/dot), `init` with styles,
  `upgrade` (`--force` / `--dry-run` / `--add-phases` / `--apply`),
  `templates list/show`, `generate` (shape + lane-set + artifact-root +
  modifiers), and stateful `run graph`. Implementations live in
  `src/striatum/workflow.py:259-380` plus
  `src/striatum/workflow_generator/{core,catalog,write}.py` (1266 lines).
  Validator covers cross-job artifact path collisions,
  write-scope/forbidden-path overlap, artifact-in-write-scope, unsound
  cycle target, parallel-group repo_write/review-only consistency, and a
  `needs` deprecation warning. Lint-style warnings already surface through
  the `warnings` channel. Deferred minor: dedicated `workflow lint` verb
  separating style from validation errors.~~

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

- ~~**14. Packaging and release.** `pyproject.toml` declares setuptools
  build, console scripts (`striatum`, `striatumd`), and `[dev]`
  ruff/mypy/pytest extras. `.github/workflows/ci.yml` runs ruff + mypy +
  pytest + UI build/test + `release_metadata_check.py` + `package_smoke.sh`
  + `fresh_clone_smoke.sh` across ubuntu/macOS and py3.11/py3.12.
  `.github/workflows/release.yml` builds wheel+sdist on `v*` tags, runs
  `twine check --strict`, publishes to PyPI via OIDC trusted publishing,
  and cuts a GitHub Release. Documentation policy items (signing,
  security disclosure, release cadence) tracked separately.~~

- ~~**15. `run summary` polish.** Markdown groups verdicts by review job with
  attempt counts, appends the structured author byline to each artifact,
  surfaces recorded vs. current git branch with `(MISMATCH)` when they
  differ, and prints a Timing block (`created_at`, `started_at`,
  `completed_at`, wall-clock `duration`).~~

- ~~**17. SQLite migration system (RFC 0006, accepted).** Schema version
  tracked through `PRAGMA user_version`; `striatum init` and every connect
  apply pending migrations inside a single `BEGIN IMMEDIATE` transaction;
  databases newer than the runner exit with code 9.~~

- ~~**18. Workflow type catalog and chooser.**
  `src/striatum/workflow_generator/{catalog.py,core.py,write.py}` plus
  `src/striatum/workflow_templates/catalog.json` provide the generator
  core. CLI verbs `workflow templates list/show` and `workflow generate`
  are wired through `src/striatum/cli/parser.py:255-267`. Web chooser
  lives at
  `src/striatum/web/frontend/src/islands/workflow-chooser/WorkflowChooser.tsx`
  with template `src/striatum/web/templates/workflow_new.html` and tests
  `workflow-chooser*.test.ts*`. Chat-assisted scaffolding ships via RFC
  0036 V1 (`generate_workflow_preview`, `generate_workflow_write`).
  Future target-repo catalog extensions remain a separate decision.~~

## In Progress

1. **Process adapter.** Single-shot `adapter run` is shipped
   (`src/striatum/process_adapter.py`). Long-lived supervision (RFC 0009,
   accepted) landed in `src/striatum/supervisor.py` plus
   `.striatum/bin/{claude,codex,gemini}-supervised-wrapper.sh`:
   `process_supervisors` table (migration version 4), `striatum supervise
   start | send | stop | status | list`, lazy lease-expiry recovery that
   flags supervisors `lost` without auto-killing the OS process,
   supervised-aware `claim-next` that auto-delivers the freshly built packet
   through the supervisor's stdin pipe and lazily marks pipe-missing/write-fail
   supervisors `lost`, `doctor` checks for dead pids and missing stdin pipes.
   **Remaining:** no PTY path — `grep pty src/striatum/` returns nothing, so
   CLIs that refuse non-TTY stdin still can't be supervised; `recovery
   process-reconcile` UX is partial.

2. **Adapter constraint enforcement.** Workflow validation supports lane
   `required_enforcement` and rejects lanes whose adapters cannot satisfy it
   (`src/striatum/workflow.py:1576-1682` `_validate_lane_constraints`
   rejects unknown constraint keys + unsatisfiable enforcement levels;
   constraint vocabulary at `workflow.py:55-57`; adapter-side matrix at
   `src/striatum/db.py:1422-1438`). The four-level model (`enforced`,
   `advisory_strict`, `advisory`, `unsupported`) is in place; the process
   adapter graduates `network` and `repo_scope` to `advisory_strict` via
   proxy-env scrubbing and sentinel env vars. **Remaining:** only the
   `process` adapter is modeled — no sandbox/worktree adapter exists to
   mechanically promote `network`/`repo_scope` from `advisory_strict` →
   `enforced` (filesystem isolation, network namespacing).

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
    `scripts/` is now CI-smoke-only (`fresh_clone_smoke.sh`,
    `package_smoke.sh`); `.striatum/bin/*-supervised-wrapper.sh` are
    supervisor wrappers, not bootstrappers; no P00* prompt is referenced
    from `src/`. **Remaining:** no `examples/` runner-owned workflow fixture
    yet reproduces the historical P001 three-lane design+build+review flow;
    tmux mention only survives in `src/striatum/skills/context.py:63` as
    "do not trust" framing, so the design-workflow fixture is the last step
    before the tmux harness fully retires from active guidance.

## Open

16. **Keep generic language current.** New docs should say "target
    repository", "workflow fixture", "runner state", "artifact", and
    "adapter" rather than assuming Engram-specific paths or marker names.
    No active sub-task.

20. ~~**RFC 0040 V1.5 follow-up.** Six codex findings (F1-F6) from
    dogfood-040 build review iteration 2.~~ ✅ Done: shipped under
    dogfood-044 (v1.33.0). (F1) daemon MCP `tools/call` now dispatches
    through `daemon_pg/mcp_dispatch.py::dispatch_mcp_tool_call` →
    composite tools `dogfood.publish_on_behalf` +
    `dogfood.surgical_recovery` are now functional through the MCP
    path; (F2/F3) `publish_on_behalf` runs ack/publish/verdict inside
    one outer transaction with rollback-event emission on failure, and
    review verdicts are validated + recorded with `findings_artifact_id`
    defaulting from the published artifact when kind=`finding`; (F4)
    `process_progress.progress_loop_once` is invoked from
    `daemon.daemon_sweep_once` and folds results into the sweep
    payload; (F5) `startup_grace_seconds=60` default, `FileNotFoundError`
    /`OSError` on log scan tolerated, `should_stop` predicate checked
    between supervisors, shared `progress_advisory_lock`, and PID-reuse
    guard via `process_start_time`; (F6) `tests/test_mcp_dogfood_e2e.py`
    + new `test_progress_loop_once_*` cases. 4th codex/codex
    anti-pattern instance (D098 cycle-exhaustion override). Codex
    needs_revision findings absorbed into RFC 0040 V1.6 (item 28
    below).

28. **RFC 0040 V1.6 follow-up.** Codex needs_revision findings from
    dogfood-044 build review, deferred by cycle-exhaustion override
    per D098 (decision `dec_242ea0b026d547c9baad9b353b149033`). 4th
    instance of the codex/codex implementer+reviewer anti-pattern
    (precedents D095 dogfood-042 Track A, D096 dogfood-042 Track C,
    D097 dogfood-043). 2-of-3 cross-lane verdicts: claude
    accept_with_findings (medium), gemini accept (low). Land the
    codex needs_revision delta via a future dogfood. The anti-pattern
    is now well-characterized across four independent runs; the
    refuse-by-default validator rule (TODO item 26) remains the
    deferred half.

21. ~~**RFC 0038 V1.5 follow-up.** Codex attempt-2 findings (F1-F4)
    from dogfood-041 build review iteration 2 + gemini attempt-1
    findings, deferred by cycle-exhaustion override (decision
    `dec_251e8a5f3d674c409de0dad9eacd5844`).~~ ✅ Done: shipped under
    dogfood-045 (v1.34.0). (F1) `placeholderIslandPlugin` removed from
    `vite.config.ts`; new `make ui-verify-bundle` + Python sentinel
    test refuse placeholder bundles. (F2) `/workflows/new` chooser
    rewritten around the server-stable `{"templates": [...]}` shape;
    `types.ts` / `api-client.ts` / `WorkflowChooser.tsx` realigned;
    modifier step removed. (F3) New
    `src/striatum/web/frontend/src/shared/island-shared-entry.ts`
    non-mounting entry is now the Rollup input for `island-shared`;
    vitest regression `island-shared-no-mount.test.ts` pins the
    single-mount guarantee. (F4) Vite output semantics aligned with
    package-data layout (`manifest: false`; sub-package entry already
    matches). Supply-chain hygiene: `npm ci` in `ui-install`,
    `ui-update-lock`, `ui-audit`, `npm-audit-baseline.json` committed.
    Implementer was **claude** (not codex) — first dogfood deliberately
    avoiding the codex/codex anti-pattern after 4 instances (D095-D098).
    Codex reviewer still came back harsh (`reject` critical,
    threat_model); cross-lane majority disagreed (claude
    `accept_with_findings` medium, gemini `accept` low); D099
    (`dec_ccfa1685878d41d69ccc6496cd6612fd`) overrode the codex reject.
    Codex critical findings (placeholder bundles still committed
    pending operator `make ui-update-lock` + `make ui-build`; supply-chain
    polish items) absorbed into RFC 0038 V1.6 follow-up (item 29 below).

29. **RFC 0038 V1.6 follow-up.** Codex reject-override deltas from
    dogfood-045 build review (decision `dec_ccfa1685878d41d69ccc6496cd6612fd`,
    D099): (a) committed bundles under `src/striatum/web/static/build/`
    are still the V1 placeholders pending operator-side
    `make ui-update-lock` + `make ui-build` + lockfile/bundle commit
    (HANDOFF.md Deviation: real-bundle commit); (b) move
    `@vitejs/plugin-react` to `devDependencies` during the same
    lockfile regeneration; (c) verify build verification gates
    (`make lint` / `make typecheck` / `make test` / `make ui-test`)
    pass against real output. Cross-lane majority accepted the source-side
    fixes; codex `reject critical` (threat_model posture) overridden because
    the missing real-bundle step is an operator-side mechanical follow-up
    explicitly documented in the HANDOFF, not an architectural defect. Land
    the real-bundle commit + supply-chain polish via a near-term operator
    sweep rather than a full dogfood cycle. This is the first reject-severity
    override (D099) on the books — prior cycle-exhaustion overrides
    (D095/D096/D097/D098) all overrode `needs_revision`.

22. ~~**Implement RFC 0043 V1 (Postgres as Sole Substrate, daemon-required).**
    Per D094 (accepted; supersedes D006/D007/D036 and SQLite half of D009).~~
    ✅ Done: shipped under dogfood-048 (v1.37.0). Two-track split:
    **Track A (codex)** landed daemon-side schema migration v5
    (`src/striatum/daemon_pg/sql/0005_repo_local_workflow_state.sql`)
    creating the 17 repo-local tables (`workflow_snapshots`, `runs`,
    `sessions`, `jobs`, `job_dependencies`, `queue_messages`, `leases`,
    `work_packets`, `artifacts`, `verdicts`, `blockers`,
    `command_requests`, `process_executions`, `events`, `job_worktrees`,
    `process_supervisors`, `process_supervisor_pointers`) under
    `repository_id text NOT NULL REFERENCES striatumd.repositories`,
    plus `striatumd.repo_migrations` checkpoint table and append-only
    `events`/`artifacts` triggers. Daemon DB version bumped 4 → 5.
    Migration body in `src/striatum/daemon_pg/repo_local_migration.py`
    (`RepoLocalMigrationOptions`, `migrate_repo_local`,
    `compute_repo_local_reanchor`) opens source SQLite read-only,
    verifies `PRAGMA user_version == LATEST_VERSION`, copies rows in
    dependency order inside one `SERIALIZABLE` Postgres transaction,
    writes the repo-migration checkpoint, then renames
    `.striatum/state.sqlite3 → state.sqlite3.tombstone` (mode `0444`)
    unless `--confirm-delete` is set. Byte-equivalent audit-chain
    re-anchor compares canonical `events` and `artifacts` row manifests
    between SQLite and Postgres via SHA-256. Daemon command helper at
    `src/striatum/cli/daemon.py`. **Track B (claude)** retired
    `--no-daemon` (argparse exit 2 `unrecognized arguments: --no-daemon`),
    introduced `DaemonUnreachableError` (exit 11) and
    `RepoNotMigratedError` (exit 12) in `src/striatum/errors.py` with
    canonical stderr remediation templates (Linux systemd, macOS
    launchd, foreground, Postgres) + JSON envelope `hint` field,
    wired env-gated `enforce_daemon_required` in
    `src/striatum/cli/daemon_required.py` + `src/striatum/cli/dispatch.py`
    with `DAEMON_OPTIONAL_COMMANDS` allowlist (`daemon`, `init`,
    `skills`, `plugin`), renumbered legacy V1 daemon errors
    (auth → 14, capability → 15) so codes 11 and 12 stay unambiguous
    for the RFC 0043 entry layer, and expanded
    `src/striatum/daemon_rpc/registry.py` + `server.py::CLI_ROUTES`
    to cover every mutation in `cli/mutations.py` per RFC 0043 §5
    (dotted vocabulary: `session.*`, `work.*`, `artifact.publish`,
    `review.*`, `decision.record`, `checkpoint.resolve`,
    `branch.confirm`, `run.*`, `worktree.*`, `recovery.*`,
    `supervise.*`, `workflow.*` + daemon-global `repo.list` +
    `daemon.migrate_repo_local`), keeping legacy undotted aliases as
    `deprecated=True`. New test suites: `tests/cli/test_no_daemon_retired.py`,
    `tests/cli/test_daemon_doctor_without_daemon.py`,
    `tests/exit_codes/test_rfc0043_refusals.py`,
    `tests/daemon_rpc/test_registry_rfc0043_coverage.py`,
    `tests/daemon_pg/test_repo_local_migration.py`,
    `tests/fixtures/v1_repo_local_sqlite/`. D102 cycle-exhaustion
    override applied: codex `needs_revision` high + gemini
    `needs_revision` medium (both with real findings on crash-recovery
    persistence gap, CLI escape path closure, migrate-repo-local
    subcommand wiring) overridden by single accepting verdict (claude
    `accept_with_findings` low). **D102 is distinct from D095-D101 in
    finding character**: both codex+gemini hit `needs_revision` with
    real findings rather than the codex/codex co-blindness anti-pattern
    (D095-D098, D100) or the codex-reviewer-of-claude-implementer
    pattern (D099, D101). Two run-quality regressions surfaced: the
    3rd `claude-no-artifact` instance (claude reviewer composed no
    REVIEW.md — operator-composed to recover) and the 3rd
    `gemini-no-frontmatter` instance (gemini REVIEW.md missing v1
    front matter — operator-fixed). Operator also performed SQL
    surgery on the `artifacts.logical_name` because the
    publish-on-behalf call passed the wrong logical name during the
    recovery. Findings folded into RFC 0043 V1.5 follow-up (item 31
    below).

23. ~~**Implement RFC 0044 V1 (Engram Phase 1 read-only MCP).** RFC drafted
    under dogfood-042 Track B; build review 3-of-3 accept (codex,
    claude, gemini). Implementation lands: Striatum-owned redacted
    JSONL export, Engram-owned `ingest-striatum`, standalone
    `engram-mcp-stdio` MCP server, four read-only retrieval tools,
    Engram-local `memory.*` capabilities, and the hard augmentation-
    not-dependency boundary (Striatum runs with Engram unavailable).~~
    🟡 Striatum-side V1 done under dogfood-046 (v1.35.0):
    `striatum corpus export --since <ref> --out <dir>` ships the
    redacted JSONL bundle (nine files + `manifest.json`) backed by
    `src/striatum/corpus/` (types, git helpers, enumerator, redactor,
    JSONL writer, manifest, export orchestration). The augmentation
    boundary is pinned by
    `tests/test_cli_corpus_export.py::test_no_engram_imports_or_memory_capabilities_in_striatum`
    (no `import engram`, no `from engram`, no `memory.*` capabilities
    across `corpus/`, `cli/`, `daemon_rpc/`, `daemon_pg/`, `mcp.py`,
    `service.py`, and `pyproject.toml`). D100 cycle-exhaustion
    override applied: codex `needs_revision` (5th codex/codex
    anti-pattern instance after D095/D096/D097/D098) + gemini
    `needs_revision` (focused entirely on out-of-scope Engram-side
    attack surface — MCP server, ingester, capability model — none of
    which ship in this dogfood); single accepting verdict claude
    `accept_with_findings` low covered the in-scope Striatum-side
    surface. Engram-side (ingester `engram ingest-striatum`,
    standalone `engram-mcp-stdio` server, four read-only retrieval
    tools, `memory.*` capabilities) remains a separate follow-up at
    `~/git/engram/` and is explicitly NOT in Striatum's TODO scope.
    Engram-side adversarial findings from gemini (RFC 0044 §6
    contradiction on `memory.read_personal` default, `corpus_id`
    isolation, indirect prompt injection memory poisoning, manifest
    forgery without cryptographic signing, secret leakage through
    curated artifacts) are forwarded to the Engram-side
    implementation effort.

32. **Queue Engram-side tenant-aware RFC 0044 Phase 1.** Striatum-side export
    is already done under dogfood-046 and remains the only in-repo shipped
    surface. The external Engram follow-up at `~/git/engram/` should implement
    the tenant-aware Phase 1 contract from RFC 0044: `tenant_id` as the local
    application-memory boundary, `corpus_id` as the workload/dataset boundary,
    `engram ingest-striatum --bundle <dir> [--repo <name>]`, read-only
    `engram-mcp-stdio`, and capability tests proving default Striatum operator
    access is restricted to the Striatum tenant/corpus while existing personal
    memory remains isolated. This is queued external work; do not add Engram
    ingester or MCP code to Striatum.

24. ~~**RFC 0039 V1.5: address Track A build review findings.** Cycle-
    exhaustion override per D095 (decision
    `dec_b75d66f38a3d40228891248c91a27774`). 2-of-3 reviewers
    accept_with_findings (claude, gemini); codex needs_revision
    overridden because the codex/codex implementer+reviewer pairing
    converged on its own findings (anti-pattern documented in D095
    follow-up). Land the codex / claude / gemini findings deltas via
    a future dogfood folded into Phase 2.~~ ✅ Done: shipped under
    dogfood-047 (v1.36.0). All five synthesis findings (F1-F5)
    landed in implementation order **F5 → F4 → F1 → F2 → F3**:
    (F5) `go/pkg/db/connection.go` rewritten on top of
    `github.com/jackc/pgx/v5 v5.7.2` — the Go daemon's first
    third-party runtime dependency; `PsqlRunner` /
    `exec.Command("psql", ...)` removed from production code paths;
    `db.Runner` + `db.TxRunner` interfaces expose parameterized
    `Exec`/`QueryRow`/`QueryScalar`/`BeginTx`; pool configured with
    `application_name="striatumd-go/<daemon_version>"` and default
    `statement_timeout=60000`. (F4) `go/pkg/db/audit.go::RecordRPC`
    opens one `READ COMMITTED` transaction via the F5 runner, locks
    the singleton `striatumd.audit_chain_head` row with
    `SELECT ... FOR UPDATE`, derives the open audit segment, computes
    the v2 row hash from the locked `previous_hash`, inserts with
    `INSERT ... RETURNING audit_id`, updates the chain head, commits,
    and returns the audit id (closes the V1 envelope-shape regression
    that returned empty `audit_id` to clients). (F1)
    `go/pkg/rpc/auth_pg.go` introduces `PostgresAuthorizer`; tokens
    are HMAC-SHA256(salt, secret) compared via
    `subtle.ConstantTimeCompare`; capability lookup mirrors
    `src/striatum/daemon_rpc/capability.py` exactly (same WHERE,
    wildcard ordering, scope-mismatch fallback); denial vocabulary
    matches Python so clients cannot tell the two cores apart from
    the refusal envelope; `go/cmd/striatumd/main.go` wires it
    whenever a Postgres URL is configured. (F2)
    `go/cmd/striatumd/main.go` flag surface is the synthesis-locked
    `--socket / --postgres-url / --migrate / --describe /
    --migrations-sha-source`; `go/Makefile` writes
    `go/bin/striatumd`; `tests/_harness/daemon.py::_start_go`
    launches with the locked argv. (F3) top-level `Makefile` exposes
    `CORE ?= python` and forwards as
    `STRIATUM_MULTI_REPO_DAEMON_CORE`; `tests/conftest.py` adds a
    class-scoped `daemon_core` fixture; CI shape is the
    synthesis-locked **two explicit jobs** (`CORE=python`,
    `CORE=go`). New tests:
    `tests/test_daemon_go_smoke.py` (boot + `daemon.hello` +
    `daemon.describe` + audit-chain-head moved),
    `tests/test_daemon_go_audit.py` (concurrent audit-emitting RPC
    calls against `MultiRepoHarness(daemon_core="go")`),
    `go/pkg/db/audit_race_test.go` (opt-in on
    `STRIATUM_PG_TEST_URL`). Implementer was **claude** (Go +
    Python harness mix), deliberately not codex — second dogfood
    avoiding the codex/codex anti-pattern (precedents D095-D098,
    D100). D101 override applied: codex `needs_revision` high
    (codex-reviewer-of-claude-implementer pattern, distinct from
    codex/codex co-blindness — same axis as D099 dogfood-045)
    overridden via 2-of-3 cross-lane consensus (claude
    `accept_with_findings` low ergonomics_dx, gemini
    `accept_with_findings` medium threat_model). Codex
    needs_revision findings F1-F5 (`go.sum` unchecksummed,
    unauthenticated/no-audit production fallback when no
    `--postgres-url` is configured, `CORE=go` matrix can pass with
    all tests skipped, smoke-test asserts no denial reason on
    unauthenticated `daemon.describe`, audit-append regression not
    executable without `STRIATUM_PG_TEST_URL`) absorbed into RFC
    0039 V1.6 follow-up (item 30 below).

30. **RFC 0039 V1.6 follow-up.** Codex needs_revision findings from
    dogfood-047 build review, deferred under D101 (decision
    `dec_f8d268f392ca44dd8a9bccb634249979`). Codex
    reviewer-of-claude-implementer pattern (distinct from codex/codex
    co-blindness; same axis as D099 dogfood-045). Land the codex
    F1-F5 deltas via a future dogfood: (F1) `(cd go && go mod tidy)`
    and commit `go.sum` so `pgx/v5` and indirect dependencies are
    cryptographically pinned and `make daemon-go-build` succeeds;
    (F2) remove the unauthenticated/no-audit production fallback in
    `go/cmd/striatumd/main.go:49` — a serving daemon without a
    Postgres URL must refuse to bind a socket rather than installing
    `AllowAllAuthorizer{}` with no `AuditRecorder` (or install a
    deny-all/no-db authorizer that audits a startup/config failure
    through a known safe path); (F3) make `make test-multi-repo
    CORE=go` hard-fail when the required Postgres harness is
    unavailable (or split a separate non-optional Go-core CI target),
    plus a sentinel assertion that
    `tests/test_daemon_go_smoke.py` / `tests/test_daemon_go_audit.py`
    actually executed rather than skipping; (F4) extend
    `tests/test_daemon_go_smoke.py` to assert unauthenticated
    `daemon.describe` is denied with the expected error/denial
    reason and that the denial row is present in the audit chain;
    (F5) make `go/pkg/db/audit_race_test.go` +
    `tests/test_daemon_go_audit.py` actually run in CI (require
    `STRIATUM_PG_TEST_URL` or equivalent ephemeral Postgres for the
    Go-core matrix job). Gemini accept_with_findings medium
    threat_model also flagged dependency-budget hygiene (`go mod
    verify` in the `CORE=go` matrix) and migration-advisory-lock
    persistence under the new `pgx` pool — both forwarded to this
    follow-up alongside the codex deltas.

25. **Phase 2 (RFC 0039 Steps 3-6).** CLI integration (`striatum daemon
    start --core go`), mutating workflow verbs on the Go core,
    supervised processes in Go, and distribution (release artifacts,
    macOS/Linux CI matrix across `daemon_core={python,go}`,
    `make` wiring for end users). **Now unblocked**: RFC 0043 V1 landed
    in dogfood-048, so the Go core has a single canonical Postgres
    substrate (no SQLite half remaining) and Phase 2 can proceed.

26. **Harness improvement: forbid codex/codex implementer+reviewer
    pairing in workflow validator.** Cycle-exhaustion observed three
    times across recent runs (dogfood-042 Track A per D095;
    dogfood-042 Track C per D096; dogfood-043 Python build per D097).
    When the implementer and a reviewer are both the same model
    (codex+codex specifically observed), the reviewer's findings
    cluster around the implementer's same blind spots, producing
    apparent "needs_revision" verdicts that 2-of-3 majority overrides.
    Partial: a soft warning landed in the dogfood-043 prep commit;
    full refuse-by-default (validator-level rejection with override
    knob) remains deferred. Add a workflow validator rule that warns
    or rejects same-model implementer↔reviewer pairs; alternative is
    a workflow-authoring guideline plus catalog-template enforcement.
    Pair with the dogfood-040 F39 note already documenting the same
    anti-pattern.

27. **RFC 0045 V1.5: address codex build review findings from
    dogfood-043** (cycle phase-jump validator gap, strict phase-skip
    restriction, phase_id strict-on-v1 check, drag-drop dropdown
    bypass, malformed v1.1 tolerance) — see D097. Cycle-exhaustion
    override per D097 (decision
    `dec_2c5fbf49e91441aca3562a66919ea8c1`). 2-of-3 cross-lane
    reviewers accept (claude accept_with_findings low, gemini accept
    low); codex needs_revision overridden because the codex/codex
    implementer+reviewer pairing produced the third instance of the
    convergent-blind-spot anti-pattern (D095, D096, D097). Land the
    codex findings deltas via a future dogfood.

31. **RFC 0043 V1.5 follow-up.** Codex + gemini needs_revision findings
    from dogfood-048 build review, deferred under D102 (decision
    `dec_0b953435368e40109e793378e1a75054`). Distinct from prior
    cycle-exhaustion overrides — both overridden verdicts carried real
    findings, not codex/codex co-blindness (D095-D098, D100) or codex-
    reviewer-of-claude-implementer baseline (D099, D101). Land the
    deltas via a future dogfood:
    (a) **Crash-recovery persistence gap (codex F1, gemini F2).**
    `migrate_repo_local()` performs the SQLite → Postgres copy inside
    one `SERIALIZABLE` transaction but the tombstone rename + repo
    deletion happen post-commit. If the daemon crashes between commit
    and rename, the repo is migrated in Postgres but
    `.striatum/state.sqlite3` remains writable; a re-run would refuse
    on the `repo_migrations` row but operators cannot tell from disk.
    Add a two-phase post-commit tombstone with a sentinel
    (`.striatum/state.sqlite3.migrated`) written before commit so
    crash-during-rename is detectable on next startup.
    (b) **CLI escape path closure (codex F2).** Track B's
    `enforce_daemon_required` runs at dispatch top; allowlist exits
    via `DAEMON_OPTIONAL_COMMANDS`. But the env-gated activation
    (`STRIATUM_DAEMON_REQUIRED=1`) means the default path is the
    pre-RFC-0043 SQLite fallback. RFC 0043 §3 specifies the
    daemon-required mode is the default. Flip the default to enforced
    once Track A's `daemon migrate-repo-local` subcommand is fully
    wired and the test suite green under enforcement.
    (c) **migrate-repo-local subcommand wiring (gemini F1).** Track A
    shipped `src/striatum/cli/daemon.py` as the dispatch helper but
    Track B's parser owns `src/striatum/cli/parser.py` and did not add
    the `daemon migrate-repo-local` subparser in this dogfood (the
    `_dispatch_daemon` delegation point exists but the subcommand is
    not yet reachable from the CLI surface). Wire the subparser with
    `--from sqlite --to pg --repo --postgres-url --dry-run
    --keep-sqlite-readonly --confirm-delete --json` flags per RFC 0043
    §4, exposing the migration body that already exists in
    `daemon_pg/repo_local_migration.py`.
    (d) **Test gaps (claude F1/F2).** Track A's
    `tests/daemon_pg/test_repo_local_migration.py` ships 11 passing
    cases but 5 skip absent a system Postgres URL; the
    `tests/exit_codes/test_rfc0043_refusals.py` suite covers the
    refusal templates but does not exercise the actual end-to-end
    `dispatch.main(...)` path with a live daemon socket. Add a
    `make test-rfc0043` target that requires `STRIATUM_PG_TEST_URL`
    and asserts the migration round-trip (dry-run → full-run → re-run
    refusal → manifest verification) plus exit code 11/12 stderr +
    JSON envelope smoke against a foreground daemon.

19. ~~**RFC for multi-repo / cross-repo test harness.** RFC 0035 V1
    landed in dogfood-037. `tests/_harness/MultiRepoHarness` boots a
    daemon + N registered target repositories with ephemeral Postgres,
    resets daemon DB state between tests, and supports prepare/lifecycle/
    crash-recovery/MCP-capability-scope/per-repo-write-scope e2e
    coverage through `make test-multi-repo`.~~

## V1.7-V2.0 Backlog

Items 33-39 cover RFCs proposed after RFC 0045 (item 27 boundary).
Sequencing and acceptance criteria live in `docs/ROADMAP.md`; this
section is the canonical status snapshot.

33. **RFC 0042 V1 (run-list workflow identity).** Proposed. The
    `striatum run list` surface conflates `workflow_snapshot_id` and
    `workflow_id`; the RFC adds a `workflow_identity` triple
    (`workflow_id`, `workflow_version`, `workflow_snapshot_id`) and
    stable display. No dogfood scheduled yet. Status: queued.

34. **RFC 0046 V1 (lane evidence guard at publish-artifact).** Closes
    GH #2 + #5. V1.7 scope. Already exercised informally in
    dogfoods-054b/055b/056 (operator-on-behalf publishes use
    `--allow-no-process-execution --override-rationale`); the V1
    runtime check at `publish-artifact` is what still needs to land
    formally. Status: proposed, partially active in operator practice.

35. **RFC 0047 V1 (decision-record propagation +
    `compromised` run state).** Closes GH #3 (now-closed issue had no
    implementation beyond an event row). Adds a `compromised` run state,
    supersession columns on `verdicts`, propagation logic on rejection,
    and reopen-on-accept semantics. V1.8 scope. ROADMAP §5.6.

36. **RFC 0048 (daemon-side substrate migration).** Closes gemini A1
    from dogfood-050 + codex F2 from dogfood-049 — the daemon's RPC
    router still delegates single-repo verbs to SQLite-backed CLI
    dispatch even after `migrate-repo-local`. Three phases: (A) port
    each `cli/mutations.py` handler to PG-backed daemon-internal logic;
    (B) implement same handlers in `go/pkg/rpc/`; (C) remove the
    `STRIATUM_DAEMON_REQUIRED=0 + STRIATUM_TEST_HARNESS=1` test-harness
    escape entirely. V2.0 scope (multi-week phase, paired with RFC
    0039 Phase 2 / item 25). ROADMAP §5.3.

37. **RFC 0049 (interactive claude lane via MCP).** Experimental,
    spike required. Motivated by Anthropic's 2026-06-15 plan-credit
    policy (`claude -p` moves off subscription quota onto separate
    $20-$200/month credit). On Max 20x the subscription is ~100×
    token-per-dollar improvement. **v1.48.1's wrapper auth fix relieved
    the urgency** — RFC 0049 is now a capability RFC rather than a
    blocker. Decision needed: spike or shelve? ROADMAP §5.5.

38. **RFC 0050 (operator UI rework and provenance honesty).** All
    three phases landed:
    - V1 (v1.46.0, dogfood-054 + 054b): UI primitives + dashboard parity.
    - V1.5 (v1.47.0, dogfood-055 + 055b): template extensions + 3
      provenance honesty fixes.
    - V2 (v1.48.0, dogfood-056): recovery panel island, override modal,
      copy-on-click, graph editor data binding.

    Open follow-ups from V2 review filed as GH issues #9-13:
    - **#9 HIGH** CSRF on `/v1/invoke` (ROADMAP §4.1 active runway)
    - **#10 MEDIUM** override modal DOM trust
    - **#11 MEDIUM** recovery dry-run side-effects
    - **#12 LOW** clipboard hijack via `data-copy`
    - **#13 LOW** workflow editor ghost field

39. **RFC 0051 V1 (auto-finalize from frontmatter).** Proposed
    2026-05-14. Driven by 8 operator-on-behalf publishes across
    dogfoods-054b/055/055b/056. Runner auto-finalizes when expected
    artifact appears on disk with valid `verdict_intent` and byline
    match. **Downgrades from urgent to safety-net-only after gh-16
    empirically validated v1.48.1's wrapper auth fix** (zero
    operator-on-behalf publishes across all 3 lanes). Status: queued
    for V1 implementation; ROADMAP §4.2.

43. **RFC 0052 V0 (committee deliberation workflow).** Proposed
    2026-05-14. Committee shape for high-stakes design phases: N
    producers deliberate under a named arbitrator with optional panel
    escalation and an adversarial-review sub-shape. Debate turns are
    typed front-mattered artifacts (`debate_turn`,
    `arbitration_ruling`, `panel_vote`, `panel_verdict`,
    `debate_synthesis`); solves D095-D102 reviewer co-blindness via
    lane composition rather than RFC 0018 posture labelling. Phase 0
    scaffold landed (RFC body + schema sketches). Status: queued; no
    implementation dogfood scheduled. V1.9/V2.0 — depends on RFC 0048
    daemon-side business-logic flip. ROADMAP §5.8.

44. **RFC 0053 V0 (human principal as escalation-only).** Proposed
    2026-05-14; doc-side fixes landed. Names the human role as
    `human principal`, restricts function to resolving unresolvable
    blockers or decisions; AI operator is the default driver. Same CLI
    surface, functionally bounded role. SPEC.md / GETTING_STARTED.md /
    HOW_TO_HUMAN.md prose realigned in commit 7e21399. D103 recorded
    in DECISION_LOG. **Deferred follow-ups**: workflow.json
    schema-field rename (`human_checkpoint` → `escalation_checkpoint`),
    `waiting_human` run state rename, CLI prompt-string sweep,
    `escalation` artifact-kind schema + RPC method. ROADMAP §5.8.

45. **RFC 0054 V0 (day-zero usage guide).** Proposed 2026-05-14.
    Scaffold for a single top-down doc walking new arrivals through
    the AI operator + human principal model, prerequisites, day-zero
    setup, first run, and the principal's escalation role. Phase 0
    scaffold landed; Phase A writes the doc — open question whether it
    replaces `docs/GETTING_STARTED.md` or lands as a new
    `docs/USING_STRIATUM.md`. ROADMAP §5.8.

46. **RFC 0055 V0 (marketing README + architecture graphics).**
    Proposed 2026-05-14. Scaffold for top-level README rewrite leading
    with vision/value rather than docs index; adds a system-architecture
    diagram (Mermaid recommended for native GitHub render; SVG as
    polish follow-up). Reflects the RFC 0043 / RFC 0053 substrate-and-
    role reality honestly. Phase 0 scaffold landed; Phase A rewrites
    `README.md`. ROADMAP §5.8.

47. **RFC 0056 V0 (consumer-repo directory-structure opinions).**
    Proposed 2026-05-14. Scaffold for explicit (but non-mandatory)
    recommendations for target-repo layout: `.striatum/` scratch (per
    RFC 0043), `striatum/workflows/<name>.json`, artifact roots,
    RFC 0021 DDD scaffold for `docs/`, `docs/dogfood/<NNN>/` for run
    records. Identifies which recommendations should extend
    `init --with-ddd-layout`. Phase 0 scaffold landed; Phase A writes
    `docs/CONSUMER_REPO_LAYOUT.md`; Phase B optionally extends the
    scaffold. ROADMAP §5.8.

## GH issue follow-ups (not yet bound to a workflow)

40. **GH #14 — recovery cannot clear terminal-run
    `process_exit_nonzero` blocker.** Real product bug. Reported
    against v1.48.1 with concrete repro (Engram-side run
    `run_9cadfc4d2e4646848e2d6539c23322b2`). Job is `completed` but a
    `process_exit_nonzero` blocker stays open because the process
    adapter exited nonzero AFTER the job's normal `complete`. Operator
    has no path to clear it without lease. Suggested approach:
    `docs/issues/14/` workflow (triage → fix → verify). Triage should
    decide whether the fix is (a) `recovery checkpoint resolve` accepts
    `process_exit_nonzero` on terminal runs, (b) a new
    `recovery dismiss-blocker --blocker-id <id>` verb, or (c) the
    process adapter's post-completion blocker insertion is gated by
    job state.

41. **GH #15 — docs clarify PostgreSQL transition guidance.**
    `README.md`, `docs/SPEC.md`, `docs/GETTING_STARTED.md`,
    `docs/HOW_TO_HUMAN.md` still describe `.striatum/state.sqlite3` as
    authoritative live state, contradicting D094/RFC 0043 V1 which
    moved workflow state to daemon-owned Postgres. Overlaps with
    item 31(b) (RFC 0043 V1.5 daemon-required default flip). Suggested
    approach: `docs/issues/15/` workflow; merge into RFC 0043 V1.5
    follow-up dogfood OR land first as a docs-only sweep before
    item 31 lands.

42. **GH #17 — update Striatum doc consistency for Engram memory
    integration.** Engram has been reprioritized around Striatum (see
    `~/git/engram/STRIATUM_MEMORY_ROADMAP.md`, dated 2026-05-14).
    Striatum docs should reflect: Engram ingests Striatum corpora;
    Striatum runs without Engram (augmentation, not dependency).
    Overlaps with ROADMAP §5.7. Suggested approach: `docs/issues/17/`
    workflow, paired with the Corpus Contract V2 RFC scaffold.

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
    `striatumd-<instance-id>` are in place. Deferred at V1: daemon-owned
    supervision, MCP mutation tools, sealed apply/signing,
    cross-repository workflows, service-manager install, Windows daemon
    support, and operator tenancy.

F32. ~~Land the RFC 0030 + RFC 0031 daemon V2 foundation.~~ Done:
    envelope-v1 daemon RPC codec, handshake, method registry, owner-local
    transport guards, PostgreSQL request/audit helpers, daemon DB
    supervisor/apply receipt tables, repo-local supervisor pointers, and
    fail-closed sealed-apply authority helpers.
