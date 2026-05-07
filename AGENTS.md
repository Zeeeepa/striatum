# Project Instructions

Striatum is a standalone, local-first workflow runner for terminal-based AI
coding agents. It is a generic orchestration tool for target repositories,
not an Engram-specific process script. The product boundary in `docs/SPEC.md`
is the source of truth; if a doc claim disagrees with current source
behavior, fix the doc.

## Start Here

Read these first, in order:

1. `README.md`
2. `docs/README.md`
3. `docs/DECISION_LOG.md`
4. `docs/UBIQUITOUS_LANGUAGE.md`
5. `docs/SPEC.md`
6. `docs/TODO.md`

Treat `docs/ENGRAM_INCUBATION_CONTEXT.md`,
`examples/rfc-0014-operational-artifact-home/`, and the older P00x prompts as
historical/reference fixtures unless a current task explicitly asks you to
work on Engram dogfood history.

## Product Boundary

- Striatum's live state is `.striatum/state.sqlite3` in the target repository.
  Repository files are durable provenance, not the live message bus.
- Marker files, tmux panes, terminal output, and provider hooks are not
  authoritative workflow state.
- Do not introduce hosted services, cloud APIs, telemetry, transcript
  capture, or external persistence without an explicit product decision.
- Keep workflow examples generic unless they are clearly labeled as
  historical Engram reference fixtures.

## Working As A Striatum Agent

When you are running inside a Striatum workflow (not just editing the repo),
the runner moves work through structured commands. Do not advance state by
printing phrases or touching SQLite directly.

### Sessions And Work Packets

Register a session with `striatum register-session` before claiming work, then
call `striatum claim-next --session-id <id>`. The returned packet contains
the lease id, write scope, expected artifacts (with a privacy-safe
`author: <role>-<model>-<ordinal>` byline), task prompt, and the exact
follow-up commands you should call: `ack`, `heartbeat`, `publish-artifact`,
`block`, `verdict` / `submit-review`, `complete`. Use those commands; do not
mutate state any other way.

### Worktree Isolation

If the work packet contains `worktree_required: true`, the lane has opted
into per-job filesystem isolation (RFC 0008). Run the
`commands.worktree_create` invocation included in the packet
(`striatum worktree create --session-id ... --job-id ... --lease-id ...`)
before publishing artifacts. The runner does not auto-create the worktree on
claim. `publish-artifact` reads files from the worktree but records the
artifact's logical repo-relative path; you do not need to copy files back.
Release the worktree with `striatum worktree release --worktree-id <id>`
when the job is complete.

### Supervisor Mode

When an agent CLI is held alive across multiple work packets via
`striatum supervise start --session-id <id>` (RFC 0009), packets arrive as
newline-terminated JSON lines on the supervised process's stdin (delivered by
`striatum supervise send`). Read packets line-by-line, react through the
normal CLI commands, and never parse the supervisor's own output for
workflow state. Stdout and stderr are sent to `DEVNULL` so do not rely on
captured logs. If your session needs to stop, use `striatum supervise stop`
rather than killing the process directly.

### Decision Artifacts

For owner choices that are not the output of a job (e.g., resolving a human
checkpoint), call `striatum decision record --run-id ... --path ...
--outcome {accepted|rejected|accepted_with_follow_up} --title ...`. The
command writes a durable Markdown artifact with `striatum.decision.v1`
front matter and records it as artifact kind `decision`; it does not require
an active lease. For artifacts that come out of a claimed job, use
`publish-artifact` (or `submit-review` for reviews) instead.

### Front-Matter Schemas

Workflow-authored Markdown artifacts of kind `decision`, `finding`,
`findings_ledger`, and `synthesis` should include valid V1 front matter when
the artifact carries metadata. Front matter is YAML-style `---`-delimited
with `key: <json-value>` lines. Files without a front-matter block are
accepted; files with one are validated against the registered schema and
the publisher rejects invalid front matter (exit code 6). The publisher
never rewrites artifact files.

If the artifact title block includes an `author:` line, it must exactly
match the lowercase byline supplied in the work packet
(`author: <role>-<model>-<ordinal>`). Do not derive bylines from workflow
job titles.

### Stale-Lease Recovery

Lease expiry is lazy: normal CLI commands expire stale leases as they run.
Review-only stale work can be safely requeued via
`striatum recovery requeue-stale --run-id <id> --job-id <id>`. Repo-write
stale work is refused by `recovery requeue-stale` and requires explicit
operator action because the worktree may already be partially modified;
inspect with `striatum recovery stale-leases --json` and `striatum doctor
--verbose` first.

### Operator UX

`striatum dashboard --run-id <id>` is the compact terminal view for humans
watching a run; it reads the same SQLite state as `status` and `why`. Use
`--once` for scripts and CI assertions that should not redraw a TUI.

## Development

Use the Makefile targets:

- `make install`
- `make lint`
- `make typecheck`
- `make test`
- `make smoke`

Python source lives under `src/striatum`. The CLI is a package
(`src/striatum/cli/`) split by concern. Tests live under `tests`. Examples
live under `examples`. Historical execution prompts live under `prompts`.

## Change Discipline

- Keep changes aligned with `docs/TODO.md` and accepted decisions in
  `docs/DECISION_LOG.md`.
- Update `docs/DECISION_LOG.md` for product or architecture decisions.
- Add or update tests for behavior changes.
- Prefer generic terms: target repository, workflow fixture, runner state,
  artifact, adapter, lane, session, work packet.
- Do not add new Engram-specific paths, branch names, prompt ordinals, or
  marker names to product docs or core code.
- New durable Markdown artifacts should use the lowercase privacy-safe
  byline: `author: <role-name>-<model-name>-<ordinal>`.
- Do not commit `.striatum/`, `.venv/`, caches, egg-info, transcripts, or
  private diagnostics.
- Avoid hardcoded home-directory absolute paths in tracked docs and
  fixtures; use repository-relative paths, environment variables, or
  generalized `~/` paths when a path shape matters.

## Historical Prompts

The P001-P004 prompts are retained as incubation provenance. They may
mention Engram, old branch names, or `--repo ..` command shapes. Do not
execute them as current standalone instructions without first rewriting
them for the standalone repository and the intended target repository.
