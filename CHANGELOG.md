# Changelog

## Unreleased

- New example fixtures: `examples/human-checkpoint-flow/` (analyze -> review
  -> decide, where the decide job is a `human_checkpoint`-typed job whose
  session calls `block --severity human_checkpoint` to surface an operator
  checkpoint and the operator records the decision via
  `striatum decision record --outcome accepted`), and
  `examples/adapter-unavailable-flow/` (a process-lane workflow that requests
  `network=enforced` and is rejected at validation because the process
  adapter only provides `advisory_strict` for that constraint). Both are
  covered by end-to-end tests in `tests/test_cli_mvp.py`.
- Added `striatum dashboard` command: a compact, dependency-free terminal
  dashboard over the existing SQLite state that summarizes run state, job
  counts, verdicts, open blockers, claimable work, deterministic next actions,
  and the most recent events. Supports `--refresh` for live mode and `--once`
  for one-shot rendering in scripts and CI.
- Added long-lived process supervision (RFC 0009). New `striatum supervise
  start | send | stop | status | list` commands hold an agent CLI alive
  across multiple work packets: `start` forks the lane command with
  `start_new_session=True` and a per-supervisor named pipe at
  `.striatum/scratch/<supervisor_id>/stdin.pipe`, `send` delivers a stored
  work packet as a newline-terminated JSON line through that pipe, `stop`
  sends `SIGTERM` (then `SIGKILL` after a five-second grace), `status`
  probes liveness and lazily transitions stuck rows to `lost`, and `list`
  reports supervisors for a run. The single-shot `striatum adapter run`
  command is unchanged — both flows coexist. Migration version 4 adds the
  new `process_supervisors` table with a partial unique index enforcing
  "at most one active supervisor per session". `expire_leases` marks
  supervised sessions `lost` without auto-killing the OS process, and
  `striatum doctor` flags supervisors whose pid is gone or whose stdin
  pipe is missing from disk. Stdout and stderr are sent to `DEVNULL`; the
  supervisor never captures transcripts or parses agent output for
  workflow state, preserving D028 and D037.
- `striatum doctor --verbose` now augments the historical string `problems`
  list with a `problem_records` list of structured rows. Each record carries a
  stable `check` name (e.g. `active_job_without_active_lease`,
  `stale_queue_message_claim`, `worktree_path_missing_on_disk`), the affected
  `id`, and a small `context` map. The string list is preserved verbatim so
  callers that already grep `problems` keep working.
- `striatum run summary` Markdown output now groups verdicts by review job
  with an attempt count and rolled-up prior verdicts, appends the structured
  author byline (`author: <role>-<model>-<ordinal>`) to each artifact line,
  surfaces the recorded branch alongside the current git branch with an
  explicit `(MISMATCH)` annotation when they differ, and prints a Timing
  block with `created_at`, `started_at`, `completed_at`, and wall-clock
  `duration`.
- `striatum workflow init [--style minimal|review|code-change] <path>` writes
  a starter workflow tree (`workflow.json` plus `roles/` and `prompts/`
  stubs) that validates cleanly with `workflow validate`. Refuses to
  overwrite an existing path. The `review` default mirrors the
  `examples/code-change-flow/` shape with placeholder paths; `minimal` skips
  review; `code-change` adds a one-shot `needs_revision` cycle.
- Workflow validator now rejects cross-job expected-artifact path collisions,
  write-scope `allowed_paths` that overlap `forbidden_paths`, expected
  artifacts outside the job's write scope, unsound revision cycles whose
  target does not feed back into the cycle source through workflow edges, and
  parallel groups that mix `repo_write` with review-only jobs.
- Workflow validator emits a deprecation warning to stderr when jobs declare
  the legacy `needs` field; `edges` remains authoritative.
- Cycle resolution now redirects downstream dependencies to the new review
  attempt so jobs gated on the review verdict unblock once the new attempt
  accepts.
- New example fixtures: `examples/code-change-flow/` (draft -> review -> apply
  with a one-shot needs_revision cycle) and
  `examples/failed-review-revision-cycle/` (single review whose second
  needs_revision opens a configured human checkpoint).
- Added opt-in per-job git worktree isolation for parallel repo-write jobs
  (RFC 0008). Lanes declare `worktree_isolation: per_job` and the runner
  advertises `worktree_required: true` plus the `striatum worktree create`
  command on matching work packets without auto-creating anything. New
  CLI subcommands `worktree create | release | list` manage the worktrees,
  `publish-artifact` reads files from the active per-job worktree but records
  logical repo-relative paths so artifacts stay valid main-branch provenance,
  lease expiry marks worktrees `abandoned` for operator inspection, and
  `doctor` flags orphaned and missing-on-disk worktree rows. Migration
  version 2 adds the new `job_worktrees` table.
- Added a forward-only SQLite migration system. Schema version is tracked
  through `PRAGMA user_version`, the current schema is registered as
  `user_version = 1`, `striatum init` and every connect apply pending
  migrations inside a single `BEGIN IMMEDIATE` transaction, and a database
  newer than the runner supports is refused with the new exit code 9.
- MCP wrapper now speaks LSP-style `Content-Length` framing by default with
  automatic line-delimited fallback. Real MCP clients (Claude Desktop, IDE
  MCP integrations) can connect cleanly; existing line-delimited scripts and
  tests keep working unchanged. Added `python -m striatum.mcp --framing
  {auto,line,framed}` for operators that need to pin the wire shape.
- `striatum branch confirm` now honors the previously inert `--create` and
  `--use-current` flags and adds a new `--strict` flag. `--create` runs
  `git checkout -b <branch>` (with idempotent fallback to `git checkout`),
  `--use-current` records the actual current git branch, and `--strict`
  refuses to record unless the working tree already matches. Default behavior
  remains records-only, and the JSON response now includes `mode` and
  `created` fields.
- Replaced the evidence-export key-name blocklist with a default-deny policy
  registry. Any field not explicitly classified as `safe` in
  `EVIDENCE_POLICY` is redacted from exported Markdown, so future schema
  additions cannot silently leak agent or user prose.
- Pushed the `fresh_session_required` filter in `claim_next` into a single
  SQL query using a `NOT EXISTS` correlated subquery, replacing the
  per-candidate Python loop. Added covering index migration for
  `work_packets(run_id, session_id)`.
- Added RFC 0009 (proposed) describing the V2 long-lived process supervisor
  for agent CLIs that span multiple work packets.
- Added a fourth adapter enforcement level `advisory_strict` (between
  `advisory` and `enforced`). The process adapter graduates
  `network=forbidden` and `repo_scope=local_only` to `advisory_strict`:
  proxy env vars are scrubbed from the child env when network is forbidden,
  and `STRIATUM_NETWORK_POLICY` / `STRIATUM_REPO_SCOPE` sentinels are set so
  cooperating agents can honor the policy.

## 0.1.0 - 2026-05-07

- Split Striatum from Engram with history preserved from the former
  `agent-runner/` incubation directory.
- Renamed the package, CLI, workflow schema, and repo-local state directory to
  `striatum`.
- Replaced the initial all-rights-reserved status with Apache-2.0 licensing.
- Added standalone project metadata, CI, and a fresh-clone smoke script.
- Added workflow planning, run-summary export, stale-lease recovery
  introspection, local API wrapper, and minimal process-adapter launch support.
- Added workflow graph export, bounded stale-work requeue, decision-artifact
  recording, a local MCP-like stdio wrapper, and explicit adapter enforcement
  validation.
- Added stricter release checks with `ruff`, `mypy`, wheel/sdist smoke, and
  installed package metadata validation.
