# Engram Incubation Context

Status: draft
Date: 2026-05-06

`agent_runner` is temporarily incubated inside the Engram repository so its
design and MVP build can use the real context that exposed the need for it.

The intent is to split `agent_runner` into a separate project after the MVP is
designed, reviewed, and built.

## Why Incubate Inside Engram

Engram produced the motivating workflow:

- multi-model design and review using Claude, Codex, and Gemini;
- exact model identity mattered to confidence;
- tmux panes gave useful visibility but poor introspection;
- marker files were useful durable artifacts but too weak as the live message
  bus;
- reject/re-review paths needed explicit state;
- prompt chains, findings, syntheses, and decisions needed durable artifacts;
- branch and commit authority needed to remain human-controlled.

Incubating here lets the design team inspect the actual rough process rather
than designing from a sanitized abstraction.

## Boundaries

- `agent_runner` remains a generic local terminal-agent orchestrator, not an
  Engram-only tool.
- Engram is the reference customer and first fixture.
- Engram's local-first/no-unapproved-cloud-dependency posture should inform
  safety and privacy defaults.
- Engram-specific paths, prompt ordinals, and marker names belong in examples
  or workflow fixtures, not core product logic.
- After MVP validation, split this directory into a standalone project.

## Engram Context To Read

From the Engram repo root:

1. `README.md`
2. `docs/process/multi-agent-review-loop.md`
3. `docs/process/project-judgment.md`
4. `docs/process/phase-3-agent-runbook.md`
5. `scripts/phase3_tmux_agents.sh`
6. `prompts/P021_generate_phase_3_claims_beliefs_spec.md` through
   `prompts/P031_begin_phase_3_pipeline.md`

Treat these as reference material for `agent_runner` requirements, not as
product architecture to copy blindly.
