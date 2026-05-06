# V1 MVP Design Synthesis

Date: 2026-05-06
Status: accepted for implementation after branch confirmation

## Inputs

- `docs/design/V1_MVP_DESIGN_INPUT_claude.md`
- `docs/design/V1_MVP_DESIGN_INPUT_codex.md`
- `docs/design/V1_MVP_DESIGN_INPUT_gemini.md`
- `docs/reviews/v1/V1_MVP_DESIGN_REVIEW.md`
- `docs/reviews/v1/V1_MVP_FINDINGS_LEDGER.md`

## Accepted Recommendations

- Use SQLite under `.agent_runner/` as the authoritative local state store,
  event log, and lightweight queue.
- Expose all mutations through `agent_runner` CLI commands.
- Store durable findings, decisions, syntheses, prompts, and markers as repo
  artifacts; do not use artifacts as the live message bus.
- Use JSON workflow configuration and reject YAML.
- Require registered sessions before claiming work.
- Return identity-aware work packets from `claim-next`.
- Keep role definitions, generic context docs, and task prompts separate.
- Make review jobs fresh-session by default.
- Gate branch selection/creation through explicit human confirmation.
- Test the RFC-ledger fixture with fake sessions and no live LLM calls.

## Accepted With Modification

- Gemini's simplification warning is accepted as an implementation constraint,
  not as a command removal. V1 keeps required queue commands but uses lazy
  expiry and manual intervention for stale repo-write work.
- Parallelism is supported only where workflow JSON declares it and write
  scopes are disjoint or review-only with unique artifacts.
- Process/tmux integration is an adapter boundary in the spec. The first
  implementation may focus on state, workflow loading, work packets, and CLI
  transitions before launching real model CLIs.

## Deferred

- Coordinator chat command parsing beyond deterministic CLI subcommands.
- Server-side coordinator command allowlist enforcement until coordinator chat
  is implemented.
- Production PTY/tmux supervision.
- TUI, web, Slack, MCP, plugin discovery, hosted services, telemetry, and
  transcript capture.

## Rejected

- Removing `ack`, `heartbeat`, `release`, `block`, `complete`, or `verdict`
  from V1.
- YAML workflow configuration.
- Automatic commits/pushes.
- Treating marker files, terminal text, tmux pane state, or provider hooks as
  authoritative orchestration state.
- Automatic requeue for stale repo-write jobs.

## Applied Deltas

- `docs/design/V1_MVP_DESIGN.md` was written as the implementation design.
- `docs/SPEC.md` was updated from placeholder to implementation contract.
- `docs/DECISION_LOG.md` was updated with V1 design decisions.
- `docs/UBIQUITOUS_LANGUAGE.md` was updated with terms introduced by the
  design.

## Implementation Readiness

The design is ready for a small Python MVP after human branch confirmation.
The next step is to create or switch to an appropriate feature branch before
editing source or build files.

