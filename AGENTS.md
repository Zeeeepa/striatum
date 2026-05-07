# Project Instructions

Striatum is a standalone, local-first workflow runner for terminal-based AI
coding agents. Preserve the core boundary: it is a generic orchestration tool
for target repositories, not an Engram-specific process script.

## Start Here

Read these first, in order:

1. `README.md`
2. `docs/README.md`
3. `docs/DECISION_LOG.md`
4. `docs/UBIQUITOUS_LANGUAGE.md`
5. `docs/SPEC.md`
6. `docs/TODO.md`

Treat `docs/ENGRAM_INCUBATION_CONTEXT.md`, `examples/rfc-0014-operational-artifact-home/`,
and older P00x prompts as historical/reference fixtures unless a current task
explicitly asks you to work on Engram dogfood history.

## Product Boundary

- Striatum's live state is `.striatum/state.sqlite3` in the target repository.
- Repository files are durable provenance, not the live message bus.
- Marker files, tmux panes, terminal output, and provider hooks are not
  authoritative workflow state.
- Do not introduce hosted services, cloud APIs, telemetry, transcript capture,
  or external persistence without an explicit product decision.
- Keep workflow examples generic unless they are clearly labeled as historical
  Engram reference fixtures.

## Development

Use the existing Makefile targets:

- `make install`
- `make test`
- `make smoke`

Python source lives under `src/striatum`.
Tests live under `tests`.
Examples live under `examples`.
Historical execution prompts live under `prompts`.

## Change Discipline

- Keep changes aligned with `docs/TODO.md` and accepted decisions in
  `docs/DECISION_LOG.md`.
- Update `docs/DECISION_LOG.md` for product or architecture decisions.
- Add or update tests for behavior changes.
- Prefer generic terms such as target repository, workflow fixture, runner
  state, artifact, adapter, lane, session, and work packet.
- Do not add new Engram-specific paths, branch names, prompt ordinals, or marker
  names to product docs or core code.
- New durable Markdown artifacts should use the lowercase privacy-safe byline:
  `author: <role-name>-<model-name>-<ordinal>`.
- Do not commit `.striatum/`, `.venv/`, caches, egg-info, transcripts, or
  private diagnostics.
- Avoid hardcoded home-directory absolute paths in tracked docs and fixtures;
  use repository-relative paths, environment variables, or generalized `~/`
  paths when a path shape matters.

## Historical Prompts

The P001-P004 prompts are retained as incubation provenance. They may mention
Engram, old branch names, or `--repo ..` command shapes. Do not execute them as
current standalone instructions without first rewriting them for the standalone
repository and the intended target repository.
