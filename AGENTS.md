# Project Instructions

Striatum is a standalone, local-first workflow runner for terminal-based AI
coding agents. It is a generic orchestration tool for target repositories,
not an Engram-specific process script. The product boundary in `docs/SPEC.md`
is the source of truth; if a doc claim disagrees with current source
behavior, fix the doc.

## Start Here

Read these first, in order:

1. `README.md`
2. `docs/INDEX.md`
3. `docs/SPEC.md`
4. `docs/DECISION_LOG.md`
5. `docs/UBIQUITOUS_LANGUAGE.md`
6. `docs/TODO.md`
7. `docs/operator/BRIEF.md` for current operator state and the bounded
   plan links that supersede older handoffs.

Treat `docs/ENGRAM_INCUBATION_CONTEXT.md`,
`examples/rfc-0014-operational-artifact-home/`, and the older P00x prompts as
historical/reference fixtures unless a current task explicitly asks you to
work on Engram dogfood history.

## Product Boundary

- Striatum's authoritative live state is the daemon-owned PostgreSQL
  instance (RFC 0033 + D094 / RFC 0043), scoped per registered target
  repository. `.striatum/` next to each target repo is operational
  scratch (supervised wrapper FIFOs, pidfiles, the capability-token
  cache); the daemon is a hard prerequisite for every Striatum verb,
  and `--no-daemon` is retired. See `docs/POSTGRES_TRANSITION.md` for
  the operator runbook and the per-repo migration command.
- Repository files are durable provenance, not the live message bus.
- Marker files, tmux panes, terminal output, and provider hooks are not
  authoritative workflow state.
- Do not introduce hosted services, cloud APIs, telemetry, transcript
  capture, or external persistence without an explicit product decision.
- Keep workflow examples generic unless they are clearly labeled as
  historical Engram reference fixtures.

## Working As A Striatum Agent

When you are running inside a striatum workflow (not just editing the repo),
the runner moves work through structured commands. Do not advance state by
printing phrases or touching SQLite directly.

The workflow loop, work-packet shape, supervisor mode, decision artifacts,
front-matter rules, and stale-lease recovery instructions all live in
**[`docs/HOW_TO_AGENT.md`](docs/HOW_TO_AGENT.md)**. Read that doc — and the
RFC 0015 skill bundle when one is installed — before claiming work. The
short version:

- Use the CLI verbs supplied in each work packet's `commands` block
  verbatim. Do not derive your own.
- Stay inside `write_scope.allowed_paths`. Never write to
  `forbidden_paths` or `.striatum/`.
- Match `expected_artifacts[].author_line` exactly when an artifact's title
  block includes `author:`.
- Front-matter–carrying artifacts (`decision`, `finding`, `findings_ledger`,
  `synthesis`, `support_ledger`, `action_item_ledger`,
  `harness_improvement_proposal`, `escalation`, `operator_brief`,
  `work_plan`, `progress_note`, `operator_report`) must validate
  against their V1 schema —
  the publisher refuses invalid front matter with exit code 6.
- Lease expiry is lazy. If a normal CLI command refuses with exit code 5,
  ask the operator to recover stale work via
  `striatum recovery stale-leases` / `recovery requeue-stale` /
  `recovery process-reconcile`.

`striatum dashboard --run-id <id>` is the compact terminal view for humans
watching a run; `--once` produces a single frame to stdout for scripts and
CI assertions.

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
- Until generated daemon contracts land, new RPC methods or handwritten
  route maps must update `docs/architecture/COMMAND_AUTHORITY_MATRIX.md`
  and the authority guardrail tests.
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
