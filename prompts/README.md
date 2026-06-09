# Prompts

Status: mixed historical and reusable
Date: 2026-05-10
author: coordinator-codex-gpt-5.5-001

Most prompts in this directory are retained as Striatum incubation provenance.
They document how the V1 MVP and RFC 0014 dogfood validation were originally
coordinated while Striatum still lived inside Engram. Reusable prompts are
explicitly marked with `Status: reusable` in their title block.

Historical prompts are not current standalone execution plans. Some prompts intentionally
mention Engram paths, old branch names, or `--repo ..` command shapes because
that was the historical validation environment.

For new Striatum work, start from `README.md`, `docs/index.md`, and
`docs/reference/todo.md`. If one of these prompts is useful as a template, copy
it into a new prompt and rewrite the target repository, branch, workflow paths,
and verification commands before running it.

## Reusable Prompts

- [`OPERATOR_INITIALIZATION_PROMPT.md`](OPERATOR_INITIALIZATION_PROMPT.md):
  canonical operator prompt source. It contains the full cold-start prompt for
  a fresh AI operator session plus the shorter boundary refresher. Fill in its
  run-specific block before using the full prompt.
- [`OPERATOR_BOUNDARY_PROMPT.md`](OPERATOR_BOUNDARY_PROMPT.md): convenience
  excerpt of the boundary refresher from `OPERATOR_INITIALIZATION_PROMPT.md`.
  Paste it into an already initialized operator session when you need the short
  reminder that separates control-plane coordination from workflow role work.
- [`RFC_0026_0027_SCAFFOLD_PROMPT.md`](RFC_0026_0027_SCAFFOLD_PROMPT.md):
  paste into a fresh CLI agent session to install/load Striatum guidance,
  scaffold a full three-lane design plus adversarial-review dogfood workflow
  for RFC 0026 and RFC 0027, validate it, and stop before run start.
- [`STRIATUM_DAEMON_RESEARCH_PROMPT.md`](STRIATUM_DAEMON_RESEARCH_PROMPT.md):
  paste into outside LLMs to critique the proposed long-running daemon,
  multi-repository control plane, MCP server, storage, runtime, and migration
  direction from first principles.
