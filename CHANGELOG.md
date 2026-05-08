# Changelog

## Unreleased

## 0.2.0 — 2026-05-08

First tagged release since the V1 scaffolding. The backlog of RFCs
landed before this point (run recovery / dogfood fixes, reviewer
independence policy, support ledgers + critique-to-action loops +
harness meta-optimization, SQLite migrations, workflow
visualization, worktree isolation, long-lived process supervision,
tool harness profiles V1+V1.5+V2, session close + auto-close,
process adapter completion guarantees) is treated as the `0.1.0`
baseline. `0.2.0` lands RFC 0012 V1 on top of that baseline as the
first explicitly versioned release. Subsequent RFCs bump the minor
version on landing.

### Added

- RFC 0012 V1 (dogfood-006): local HTTP / Unix-socket service. New
  `striatum serve` command runs a `ThreadingHTTPServer` on TCP
  loopback (default `127.0.0.1`) or a Unix-domain socket; refuses
  non-loopback hosts at startup with exit 8. Endpoints:
  `/v1/health`, `POST /v1/invoke`, `/v1/runs`, `/v1/runs/<id>`,
  `/v1/runs/<id>/why`, `/v1/runs/<id>/dashboard`,
  `/v1/runs/<id>/events` (SSE), `/v1/doctor`. Mutations gated
  behind `--allow-mutations` (whitelist of read verbs); auth via
  filesystem permissions on Unix sockets or optional `--token` on
  HTTP (length-safe constant-time compare). Single-instance via
  PID file; graceful shutdown on SIGTERM/SIGINT. New module
  `src/striatum/service.py`; tests at `tests/test_service.py` (16
  cases). Closes the long-standing D006 promise of an "optional
  Unix-socket / local HTTP API later for Slack, TUI, and web
  adapters" — the four V1 acceptance criteria all pass.
- RFC 0014 V1 / issue #1 (dogfood-005): process adapter completion
  guarantees. After every `striatum adapter run` exit (including
  timeout-fired SIGTERMs), the runner inspects required
  `expected_artifacts` and, for review jobs, the verdict table. When
  any required output is missing — or the child exited non-zero or
  hit the timeout — the job transitions from `running` to `blocked`,
  a blocker row is inserted with a structured `blocker_kind`
  (`process_outputs_missing`, `process_review_verdict_missing`,
  `process_exit_nonzero`, `process_timeout_exceeded`,
  `process_lost_with_outputs_missing`), and a privacy-safe diagnostic
  envelope is recorded as the new `blockers.payload_json` column.
  The envelope contains zero child stdout/stderr (D028 preserved); it
  carries `process_id`, `command`, `exit_code`, `duration_seconds`,
  `timeout_seconds`, `missing_artifact_paths`, `review_verdict_missing`,
  and operator-copyable `recovery_commands`. New CLI surface:
  `striatum adapter run --timeout-seconds <n>` (overrides
  `lanes.<id>.adapter_timeout_seconds`; capped at 86400) and
  `striatum recovery process-reconcile --run-id <id>` (mirrors the
  `recovery requeue-stale` lazy-on-CLI shape from D036). Two new
  doctor checks (`process_running_but_pid_gone`,
  `process_running_with_expired_lease`) and a `process_health`
  summary on `striatum status --run-id`. Migrations v8
  (`process_executions.state` enum + `'timed_out'` and `'lost'`) and
  v9 (`blockers.payload_json`); both idempotent against fresh DBs.
  Tests at `tests/test_process_adapter.py` (15 new cases). Closes
  [issue #1](https://github.com/halbritt/striatum/issues/1).
- `branch.mode` is now a closed enum (`"auto"` or `"confirm"`) and
  defaults to `"auto"` when omitted. In auto mode, `run prepare`
  atomically creates the suggested branch and transitions the run to
  `ready`, eliminating the separate `striatum branch confirm --create`
  step that was previously required. The response includes
  `branch_mode`, `branch`, `branch_created`, `current_git_branch`, and
  any warning. Workflows that explicitly want the manual gate can set
  `branch.mode: "confirm"`; behaviour there is unchanged. If git
  checkout fails during auto mode (dirty tree, conflicting branch),
  the run falls back to `needs_branch_confirmation` so the operator
  can resolve the issue and run `branch confirm` manually. Migrated
  the in-repo dogfood-001/-001-v2/-002/-003/-004 and the
  `examples/harness-profiles/` workflows to auto mode; remaining
  example fixtures keep `mode: "confirm"` for test-coverage symmetry.
  Five new tests in `tests/test_cli_mvp.py` cover the auto path,
  default-when-omitted, the still-functioning confirm path, unknown
  mode rejection, and the auto-without-suggested-name guard.
- RFC 0010 V2 / HARNESS-001 (dogfood-004): reference Claude Code
  supervised wrapper at `.striatum/bin/claude-supervised-wrapper.sh`.
  Bash `while IFS= read -r` loop that spawns a fresh `claude --print`
  per packet — each Striatum work packet is independent, so per-packet
  fresh-context matches the workflow's `fresh_session_required`
  defaults and avoids depending on Claude Code's undocumented
  multi-turn `--input-format stream-json` behaviour. Inner stdout
  and stderr go to `/dev/null` (RFC 0009 / D028); SIGTERM trap
  cleans up the in-flight inner process. Verification test at
  `tests/test_claude_supervised_wrapper.py` (4 cases, stub-claude on
  `$PATH` so it does not depend on the real binary). Closes
  `docs/dogfood/003/findings/HARNESS-001.md`.
- RFC 0010 V1.5 (HARNESS-001 follow-up): workflow-validate lint warning
  for missing repo-relative process-lane command paths. Fires when
  `lane.command[0]` looks like a repo-relative path (contains a slash
  or starts with `./`/`../`) and the file does not exist under the
  workflow's repo root. Surfaces under the `warnings` key in
  `workflow validate --json` and `workflow plan --json`. Non-blocking;
  bare binary names and absolute paths are not checked. Closes the V1.5
  step of `docs/dogfood/003/findings/HARNESS-001.md`.
- RFC 0010 V1 (dogfood-003): optional `harness_profiles` workflow map
  and per-lane `harness_profile_id` reference. When a lane references a
  declared profile, `claim-next` adds a `harness_profile` block to the
  work packet (passthrough projection of the profile body plus
  `profile_id`). Workflows that omit `harness_profiles` produce
  unchanged packets. Validation accepts the closed tool-family set
  `{generic, codex, claude_code, gemini_cli}`, requires `tool_family`
  and `strategy_version`, and enforces D021 accountability
  (`native_subagents = internal_to_parent_session`,
  `first_class_registration = not_supported`). Unknown sibling fields
  on profile bodies are accepted as lint warnings, surfaced under a
  `warnings` key in `striatum workflow validate --json` and
  `workflow plan --json`. Reference fixture lives at
  `examples/harness-profiles/workflow.json`. Tests in
  `tests/test_harness_profiles.py` cover validation, packet exposure,
  backwards compatibility, and fixture loading (including the
  dogfood-003 four-profile fixture).
- D055 follow-ups (post-RFC-0011): `recovery cancel-job --cascade`
  over a whole run now transitions `runs.state` to `'canceled'`
  (previously `'completed'`) when no job actually completed; auto-close
  fires under `source: "run_canceled"`, matching the source enum value
  RFC 0011 reserved. A new `test_run_failed_auto_closes_active_sessions`
  rounds out the source-enum matrix by exercising the reject-verdict
  path that drives a run to `'failed'`. Migration helper
  `striatum.migrations.rebuild_table()` extracts the FK-safe rebuild
  pattern (PRAGMA foreign_keys OFF + IF EXISTS partial-state recovery
  + DROP/RENAME) so future migrations against tables with
  self-referential FKs do not re-discover the requirement; v7 is
  retrofitted onto the helper. v5 remains untouched as immutable
  historical record.
- RFC 0011 (dogfood-002): explicit session close + run-terminal
  auto-close. New `striatum session close --session-id <id> --reason
  <text>` command transitions an `active` session to a new `closed`
  terminal state, recording `closed_at` and a non-empty
  `close_reason` and emitting a `session.closed` event with
  `source: "explicit"`. Idempotent against already-terminal sessions
  (returns the existing row plus a `note: "session was already
  <state>"`); refuses with exit 4 when the session holds an active
  lease (message points the operator at `striatum release`). When a
  run transitions to a terminal state, every still-active session on
  the run is auto-closed inside the same transaction with `source` of
  `"run_completed"`, `"run_failed"`, or `"run_canceled"` — eliminating
  the persistent `active_session_on_terminal_run` doctor warning that
  fired on every clean-finish run before this change. Migration
  version 7 adds the `closed` state value plus the `closed_at` and
  `close_reason` columns. `evidence export` and `run summary` carry a
  per-session block with the new fields; `RUN_SUMMARY.md` gains a
  `## Sessions` section.
- HARNESS-001 fixes (dogfood-001 v2): `docs/SPEC.md` "Supervised lane
  command contract" subsection making the three supervised-lane
  requirements explicit (alive across packets, NDJSON stdin, calls back
  via `striatum` CLI). New `doctor` problem record
  `supervisor_lost_with_held_lease` plus the stable `status` next-action
  `recover_orphan_supervisor` that fires when a supervisor row is
  `lost` while the session still owns an unexpired active lease.
  `striatum supervise stop` is idempotent against an already-`lost` or
  `stopped` supervisor: returns the existing terminal row plus a
  `note` describing the prior state instead of raising
  `InvalidTransitionError`.
- HARNESS-002 fixes (dogfood-001 v2): new `doctor` problem record
  `editable_install_outside_repo` warns when the running install is
  outside the repo argument and the repo is itself a Striatum source
  tree (suppressed when the repo is just a target, to avoid false
  positives). `striatum init` against a fresh DB now refuses with exit
  3 when the repo's source-tree `LATEST_VERSION` is higher than the
  running install's, with a clear message pointing at
  `pip install -e <repo>`. `Makefile install` resolves the install path
  via `$(MAKEFILE_DIR)` so `make install` from any cwd installs *this*
  Makefile's directory in editable mode (the previous `pip install -e
  .` was cwd-dependent and silently pinned to a Claude Code worktree).
- HARNESS-003 fixes (dogfood-001 v2): `docs/SPEC.md` "Reviewer
  Independence (advisory)" and "Byline Integrity" subsections making
  the runner's enforcement boundary explicit. New `doctor` problem
  record `reviewer_independence_unverified` flags two observable
  breaches — sessions that share a supervisor pid, or a reviewer
  session running unsupervised on a run whose author is supervised.
  `register-session --role reviewer` refuses when the workflow
  declares `reviewer_context_policy: fresh` and an active author
  session already exists, unless `--force-non-fresh --reason "..."` is
  passed; the reason is recorded in the new
  `sessions.non_fresh_reason` column. `publish-artifact` records the
  artifact file's actual `author:` line in the new
  `artifacts.author_line` column (NULL when the file omits it);
  evidence exports and run summaries read the actual column so a
  missing byline renders as `author: <missing>` rather than the
  workflow's declared expected. Migration version 6 adds both columns.
- HARNESS-004 fix (dogfood-001 v2): `docs/dogfood/001/roles/reviewer.md`
  now points reviewer harness proposals at
  `docs/dogfood/001/review/HARNESS-NNN.md` (inside the review job's
  `write_scope.allowed_paths`) instead of `docs/dogfood/001/findings/`
  (which is the author's path and is rejected by the publisher with
  exit 6). `tests/test_harness_v2_fixes.py::test_reviewer_role_doc_paths_match_write_scope`
  walks every dogfood reviewer role doc and asserts each
  `HARNESS-NNN.md` instruction path is contained in the corresponding
  review job's allowed paths.
- `striatum workflow graph --format dot <workflow.json>` emits a Graphviz
  `digraph striatum_workflow { ... }` alongside the existing Mermaid
  (default) and JSON outputs. Same nodes, dependency edges, parallel
  groups (rendered as `subgraph cluster_<group>` blocks), and bounded
  `needs_revision` cycle edges (rendered as dashed arrows with the
  `max_iterations` count). Pipe through `dot -Tsvg` to render.
- Three new artifact kinds and front-matter schemas (RFCs 0003/0004/0005,
  accepted): `support_ledger` (`striatum.support_ledger.v1`),
  `action_item_ledger` (`striatum.action_item_ledger.v1`), and
  `harness_improvement_proposal`
  (`striatum.harness_improvement_proposal.v1`). Migration version 5 drops the
  SQL `CHECK (artifact_kind IN (...))` on the `artifacts` table; allowed kinds
  now live in `striatum.artifacts.ALLOWED_ARTIFACT_KINDS` and are enforced by
  `publish-artifact` (`ArtifactError`, exit 6) and workflow validation
  (`WorkflowError`, exit 8). Reference fixture
  `examples/support-ledger-flow/` exercises the produce -> support ledger ->
  evidence audit -> final review pattern; "evidence audit" is a workflow
  convention name, not a new `job_type`.
- Reviewer independence policy fields on review jobs (RFC 0002, D051).
  `type: "review"` jobs may declare `reviewer_access_scope`
  (`document_only` | `artifact_augmented` | `repo_level`) and
  `reviewer_context_policy` (`fresh` | `cross_round`). The validator
  rejects unknown values, rejects the fields on non-review jobs, and
  rejects the explicit `reviewer_context_policy: "fresh"` +
  `fresh_session_required: false` conflict. Setting
  `reviewer_context_policy: "fresh"` without `fresh_session_required`
  silently stores the prepared job row with `fresh_session_required = 1`.
  Work packets gain a `review_policy` block (`access_scope`,
  `context_policy`, `instruction`) only when the workflow declares at
  least one of the fields; existing fixtures produce identical packets.
  The `examples/rfc-0014-operational-artifact-home/workflow.json` fixture
  now labels its three independent root reviews as `document_only` and
  `fresh`.
- `striatum run graph --run-id <id> [--format mermaid|json]` renders the
  workflow graph for an existing run with each node colored by current job
  state. Mermaid output appends a `classDef` palette plus per-node `class`
  assignments (completed/running/claimed/acked/blocked/stale_lease/
  waiting_human/failed/canceled/queued/pending); JSON output adds
  `current_state`, `attempt`, and a `latest_verdict` block on review nodes.
  The runner picks the highest-`attempt` row per `workflow_job_id` so
  requeued attempts show their latest state.
- `striatum list ...` subcommand group for read-only enumeration of runs,
  sessions, jobs, artifacts, and workflow snapshots. Each command returns a
  stable `{"items": [...], "count": N}` envelope shaped from existing SQLite
  state. `list runs` joins `workflow_snapshots` to surface `workflow_id`;
  `list sessions --run-id <id>` accepts `--state`, `--role`, `--lane`;
  `list jobs --run-id <id>` includes the latest verdict for review jobs and
  accepts `--state` and `--workflow-job-id`; `list artifacts --run-id <id>`
  embeds the structured author byline and accepts `--kind`; `list workflows`
  reports loaded snapshots with their `content_sha256`. Every run-scoped
  variant applies the lazy lease-expiry sweep before reading.
- `striatum checkpoint resolve --blocker-id <id> --action {continue|cancel}
  [--decision-id <id>]` resolves an open `human_checkpoint` blocker:
  `continue` re-queues the affected job and emits `checkpoint.resolved`;
  `cancel` marks the affected job `canceled` and emits
  `checkpoint.canceled`. Optional `--decision-id` validates a run-level
  decision artifact and records it on the resolution event payload.
- `striatum recovery cancel-job --run-id <id> --job-id <id> --reason <text>
  [--cascade]` is the explicit operator cancel for a non-terminal job.
  Refuses terminal-state jobs and refuses jobs with blocked dependents
  unless `--cascade` is set, in which case dependents are canceled
  transitively in the same transaction.
- Supervised-aware `claim-next`: when the claiming session has an
  `attached` supervisor, the runner writes the freshly built packet
  through the supervisor's stdin pipe inside the same transaction,
  refreshes `heartbeat_at`, and emits a `supervisor.packet_delivered`
  event. The CLI response gains an optional `supervisor_delivery` field.
  Pipe-missing or write-fail transitions the supervisor to `lost` while
  still committing and returning the packet so the caller can recover.
- Optional per-kind Markdown front-matter validation in `publish-artifact` for
  `decision` (`striatum.decision.v1`), `finding` (`striatum.finding.v1`),
  `findings_ledger` (`striatum.findings_ledger.v1`), and `synthesis`
  (`striatum.synthesis.v1`). Front matter is read with a minimal
  `key: <json-value>` parser, validated only when present, and never rewritten
  by the publisher. Other artifact kinds remain unschemaed.
- New example fixtures: `examples/human-checkpoint-flow/` (analyze -> review
  -> decide, where the decide job is a `human_checkpoint`-typed job whose
  session calls `block --severity human_checkpoint` to surface an operator
  checkpoint and the operator records the decision via
  `striatum decision record --outcome accepted`), and
  `examples/adapter-unavailable-flow/` (a process-lane workflow that requests
  `network=enforced` and is rejected at validation because the process adapter
  only provides `advisory_strict` for that constraint). Both are covered by
  end-to-end tests in `tests/test_cli_mvp.py`.
- `striatum dashboard` command: a compact, dependency-free terminal dashboard
  over the existing SQLite state that summarizes run state, job counts,
  verdicts, open blockers, claimable work, deterministic next actions, and
  the most recent events. Supports `--refresh` for live mode and `--once` for
  one-shot rendering in scripts and CI.
- Long-lived process supervision (RFC 0009). New `striatum supervise
  start | send | stop | status | list` commands hold an agent CLI alive
  across multiple work packets: `start` forks the lane command with
  `start_new_session=True` and a per-supervisor named pipe at
  `.striatum/scratch/<supervisor_id>/stdin.pipe`, `send` delivers a stored
  work packet as a newline-terminated JSON line through that pipe, `stop`
  sends `SIGTERM` (then `SIGKILL` after a five-second grace), `status` probes
  liveness and lazily transitions stuck rows to `lost`, and `list` reports
  supervisors for a run. The single-shot `striatum adapter run` command is
  unchanged — both flows coexist. Migration version 4 adds the new
  `process_supervisors` table with a partial unique index enforcing "at most
  one active supervisor per session". `expire_leases` marks supervised
  sessions `lost` without auto-killing the OS process, and `striatum doctor`
  flags supervisors whose pid is gone or whose stdin pipe is missing from
  disk. Stdout and stderr are sent to `DEVNULL`; the supervisor never
  captures transcripts or parses agent output for workflow state, preserving
  D028 and D037.
- `striatum workflow init [--style minimal|review|code-change] <path>` writes
  a starter workflow tree (`workflow.json` plus `roles/` and `prompts/`
  stubs) that validates cleanly with `workflow validate`. Refuses to
  overwrite an existing path. The `review` default mirrors the
  `examples/code-change-flow/` shape with placeholder paths; `minimal` skips
  review; `code-change` adds a one-shot `needs_revision` cycle.
- New example fixtures: `examples/code-change-flow/` (draft -> review -> apply
  with a one-shot needs_revision cycle) and
  `examples/failed-review-revision-cycle/` (single review whose second
  needs_revision opens a configured human checkpoint).
- Opt-in per-job git worktree isolation for parallel repo-write jobs
  (RFC 0008). Lanes declare `worktree_isolation: per_job` and the runner
  advertises `worktree_required: true` plus the `striatum worktree create`
  command on matching work packets without auto-creating anything. New CLI
  subcommands `worktree create | release | list` manage the worktrees,
  `publish-artifact` reads files from the active per-job worktree but
  records logical repo-relative paths so artifacts stay valid main-branch
  provenance, lease expiry marks worktrees `abandoned` for operator
  inspection, and `doctor` flags orphaned and missing-on-disk worktree rows.
  Migration version 2 adds the new `job_worktrees` table.
- Forward-only SQLite migration system. Schema version is tracked through
  `PRAGMA user_version`, the current schema is registered as
  `user_version = 1`, `striatum init` and every connect apply pending
  migrations inside a single `BEGIN IMMEDIATE` transaction, and a database
  newer than the runner supports is refused with the new exit code 9.
- Fourth adapter enforcement level `advisory_strict` (between `advisory` and
  `enforced`). The process adapter graduates `network=forbidden` and
  `repo_scope=local_only` to `advisory_strict`: proxy env vars are scrubbed
  from the child env when network is forbidden, and
  `STRIATUM_NETWORK_POLICY` / `STRIATUM_REPO_SCOPE` sentinels are set so
  cooperating agents can honor the policy.
- RFC 0009 (proposed) describing the V2 long-lived process supervisor for
  agent CLIs that span multiple work packets.

### Changed

- Split `striatum.cli` from a single ~3.5k-line module into a package
  (`src/striatum/cli/`) organized by concern: `parser`, `dispatch`,
  `mutations`, `introspect`, `evidence`, `run_summary`, `recovery`,
  `worktree`, `supervise`, and `workflow_init`. Public surface is preserved
  via re-exports in `striatum/cli/__init__.py`; the `striatum.cli:main`
  console entry point and `python -m striatum.cli` continue to work
  unchanged. Behavior is identical (pure refactor, all existing tests pass);
  cross-module helper calls that need to honor `monkeypatch.setattr` against
  `striatum.cli` use a lazy `from striatum import cli as _cli` lookup.
- `striatum doctor --verbose` now augments the historical string `problems`
  list with a `problem_records` list of structured rows. Each record carries
  a stable `check` name (e.g. `active_job_without_active_lease`,
  `stale_queue_message_claim`, `worktree_path_missing_on_disk`), the
  affected `id`, and a small `context` map. The string list is preserved
  verbatim so callers that already grep `problems` keep working.
- `striatum run summary` Markdown output now groups verdicts by review job
  with an attempt count and rolled-up prior verdicts, appends the structured
  author byline (`author: <role>-<model>-<ordinal>`) to each artifact line,
  surfaces the recorded branch alongside the current git branch with an
  explicit `(MISMATCH)` annotation when they differ, and prints a Timing
  block with `created_at`, `started_at`, `completed_at`, and wall-clock
  `duration`.
- Workflow validator now rejects cross-job expected-artifact path
  collisions, write-scope `allowed_paths` that overlap `forbidden_paths`,
  expected artifacts outside the job's write scope, unsound revision cycles
  whose target does not feed back into the cycle source through workflow
  edges, and parallel groups that mix `repo_write` with review-only jobs.
- Workflow validator emits a deprecation warning to stderr when jobs declare
  the legacy `needs` field; `edges` remains authoritative.
- Cycle resolution now redirects downstream dependencies to the new review
  attempt so jobs gated on the review verdict unblock once the new attempt
  accepts.
- MCP wrapper now speaks LSP-style `Content-Length` framing by default with
  automatic line-delimited fallback. Real MCP clients (Claude Desktop, IDE
  MCP integrations) can connect cleanly; existing line-delimited scripts and
  tests keep working unchanged. Added `python -m striatum.mcp --framing
  {auto,line,framed}` for operators that need to pin the wire shape.
- `striatum branch confirm` now honors the previously inert `--create` and
  `--use-current` flags and adds a new `--strict` flag. `--create` runs
  `git checkout -b <branch>` (with idempotent fallback to `git checkout`),
  `--use-current` records the actual current git branch, and `--strict`
  refuses to record unless the working tree already matches. Default
  behavior remains records-only, and the JSON response now includes `mode`
  and `created` fields.
- Replaced the evidence-export key-name blocklist with a default-deny policy
  registry. Any field not explicitly classified as `safe` in
  `EVIDENCE_POLICY` is redacted from exported Markdown, so future schema
  additions cannot silently leak agent or user prose.
- Pushed the `fresh_session_required` filter in `claim_next` into a single
  SQL query using a `NOT EXISTS` correlated subquery, replacing the
  per-candidate Python loop. Added covering index migration for
  `work_packets(run_id, session_id)`.

### Tooling

- (No tooling-only changes pending in this Unreleased window. Tooling work
  in this cycle is bundled with the feature commits above.)

## 0.1.0 - 2026-05-07

- Split Striatum from Engram with history preserved from the former
  `agent-runner/` incubation directory.
- Renamed the package, CLI, workflow schema, and repo-local state directory
  to `striatum`.
- Replaced the initial all-rights-reserved status with Apache-2.0 licensing.
- Added standalone project metadata, CI, and a fresh-clone smoke script.
- Added workflow planning, run-summary export, stale-lease recovery
  introspection, local API wrapper, and minimal process-adapter launch
  support.
- Added workflow graph export, bounded stale-work requeue, decision-artifact
  recording, a local MCP-like stdio wrapper, and explicit adapter
  enforcement validation.
- Added stricter release checks with `ruff`, `mypy`, wheel/sdist smoke, and
  installed package metadata validation.
