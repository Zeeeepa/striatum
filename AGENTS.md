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
  scratch (PTY FIFOs, daemon-owned interactive lanes, pidfiles, the
  capability-token cache); the daemon is a hard prerequisite for every
  Striatum verb, and `--no-daemon` is retired. See `docs/POSTGRES_TRANSITION.md`
  for the operator runbook and native PostgreSQL repository adoption.
- Repository files are durable provenance, not the live message bus.
- Marker files, tmux panes, terminal output, and provider hooks are not
  authoritative workflow state.
- Do not introduce hosted services, cloud APIs, telemetry, transcript
  capture, or external persistence without an explicit product decision.
- Keep workflow examples generic unless they are clearly labeled as
  historical Engram reference fixtures.

## Working As A Striatum Agent

When you are running inside a striatum workflow (not just editing the repo),
the runner moves work through daemon MCP/RPC state transitions. Do not advance
state by printing phrases, scraping terminal output, or touching PostgreSQL
directly.

The workflow loop, work-packet shape, supervisor mode, decision artifacts,
front-matter rules, and stale-lease recovery instructions all live in
**[`docs/HOW_TO_AGENT.md`](docs/HOW_TO_AGENT.md)**. Read that doc — and the
RFC 0015 skill bundle when one is installed — before claiming work. The
short version:

- Use daemon MCP tools for live workflow control when an endpoint and
  capability token are available. Treat the CLI verbs supplied in each work
  packet's `commands` block as exact compatibility fallbacks and parameter
  references; if you must use CLI fallback, run them verbatim.
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
- Lease expiry is lazy. If MCP or CLI fallback refuses because a lease is
  stale, ask the operator to recover stale work through the local UI or daemon
  MCP recovery tools. CLI recovery verbs remain diagnostic/compatibility
  clients of the same daemon boundary.

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

The project is Go-only: the root Makefile delegates to `go/` for `striatum`,
`striatumd`, and `striatum-supervisor-helper` binaries. The legacy Python
runtime, source, and tests have been retired and removed per RFC 0078.
Examples live under `examples`. Historical execution prompts live under
`prompts`.

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
- Do not commit `.striatum/`, caches, transcripts, or private diagnostics.
- Avoid hardcoded home-directory absolute paths in tracked docs and
  fixtures; use repository-relative paths, environment variables, or
  generalized `~/` paths when a path shape matters.

## Historical Prompts

The P001-P004 prompts are retained as incubation provenance. They may
mention Engram, old branch names, or `--repo ..` command shapes. Do not
execute them as current standalone instructions without first rewriting
them for the standalone repository and the intended target repository.
