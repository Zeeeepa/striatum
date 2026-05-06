# agent_runner PRD

Status: draft
Date: 2026-05-06

This PRD will be developed through the interview recorded in
`docs/INTERVIEW_LOG.md` and decisions recorded in `docs/DECISION_LOG.md`.
Shared terms are defined in `docs/UBIQUITOUS_LANGUAGE.md`.

Current accepted foundation:

- D001: Start with a PRD backed by a decision log. Use RFCs for contested
  architecture branches, then write the implementation spec after product
  boundaries stabilize.
- D002: Build `agent_runner` as a generic local terminal-agent orchestrator
  from the start, with Engram workflows as reference fixtures.
- D003: Model portability is a core design goal. Agent lanes are
  configuration, not product identity.
- D004: Use a hybrid coordinator. The deterministic coordinator owns the
  control plane; exactly one selected AI coordinator lane acts as a
  project-manager-style conversational interface. Synthesis remains an explicit
  workflow job.
- D005: The AI coordinator owns project-manager responsibilities through
  deterministic commands and does not synthesize major artifacts, write source
  patches, bypass gates, or replace reviewer judgment unless assigned an
  explicit workflow job.
- D006: Use SQLite as the v1 live coordination layer and state store, with CLI
  as the first interface. Repo files remain durable artifacts, not the live
  message bus.
- D007: Store v1 run state inside the target repo under `.agent_runner/`,
  ignored by default.
- D008: Model SQLite as both append-only event log and lightweight local
  message queue.
- D009: Agents update orchestration state through the `agent_runner` binary/CLI.
  The binary owns SQLite writes and invariants. MCP is optional adapter surface,
  not the core contract.
- D010: Agents identify themselves through registered sessions before claiming
  work. Work packets are selected by role, lane, capabilities, run, and
  workflow state.
- D011: Prefer persistent agent sessions until the assigned role expires, while
  allowing fresh sessions where context reset matters.
- D012: Use opaque `session_id` plus human-readable `<role>-<model>-<ordinal>`
  slugs.
- D013: Use CLI as the primary agent control surface.
- D014: Use mostly DAG-shaped workflows with bounded retry/revision cycles.
- D015: V1 parallelism is workflow-declared and write-scope safe; AI-inferred
  build parallelization is deferred.
- D016: Build outputs are durable and idempotent.
- D017: Coordinator starts/selects a branch and requests commits from the
  human by default.
- D018: Implement v1 in Python.
- D019: Role definitions are reusable artifacts; context docs are generic; task
  prompts may reference roles.
- D020: Coordinator chat commands such as "read prompt foo" are coordinator
  skills backed by deterministic CLI operations.
- D021: Native sub-agents are internal to the parent agent session in v1; the
  parent remains accountable unless a sub-agent is explicitly registered as a
  first-class session.
- D022: The minimum common integration contract is process-based.
- D023: Start with a provisional mutation command set.
- D024: The design team defines work packet and SQLite schema details before
  implementation.
- D025: V1 surfaces are CLI plus tmux introspection; TUI, Slack, and web come
  later.
- D026: Branch creation/selection is confirmation-gated at run start.
- D027: Workflow configuration uses JSON; YAML is rejected.
- D028: Decisions, prompts, findings, syntheses, markers, and handoffs are
  durable repo artifacts. Transcripts are not captured or published by default.
- D029: Fresh context means new role instantiation; reviews/builds are
  persistent while role remains active.
- D030: First validation workflow is RFC-ledger cleanup.
- D031: Emulate Engram's Python project discipline where appropriate.
- D032: The one-shot MVP process requires design input from all three frontier
  model lanes before synthesis/build.
- D033: Incubate `agent_runner` inside Engram through MVP design/build, then
  split it into a separate project.

Seed thesis:

`agent_runner` is a local-first orchestration tool for coordinating multiple
terminal-based AI coding agents over repository workflows. It should preserve
exact model command control, structured review gates, local state, and durable
repo-published findings without treating repository marker files as the live
message bus.

## Seed Requirements

- Support frontier model lanes for Claude Code, Codex, and Gemini CLI out of
  the box.
- Allow the owner to select any one lane as the AI coordinator for a workflow
  or phase.
- Keep model/provider assumptions in adapters and workflow config, not in core
  orchestration semantics.
- Provide a local deterministic coordinator for state, gates, launch, safety,
  and message routing.
- Allow interactive coordinator chat.
- Instantiate the AI coordinator with a project-manager prompt focused on the
  stated outcome, blockers, routing, and next actions.
- Keep major synthesis as an assigned workflow job rather than a default
  coordinator responsibility.
- Constrain coordinator capabilities so it moves work through the control plane
  rather than editing state or artifacts ad hoc.
- Eventually support Slack as an interaction surface.
- Keep agents introspectable through tmux.
- Consider a TUI dashboard first and a web dashboard with chat later.
- Provide a lightweight local message bus for agent/coordinator communication.
- Use SQLite for local live state: runs, jobs, messages, events, verdicts,
  process metadata, and artifact references.
- Store local state under `.agent_runner/` in the target repo by default.
- Provide queue semantics for work delivery, acknowledgements, leases, retries,
  blockers, and completion signals.
- Provide a binary/CLI control surface that agents use to mutate orchestration
  state instead of writing SQLite directly.
- Require agent/session identity for queue claims and job-state mutations.
- Provide human-readable agent session slugs for dashboards and tmux, while
  preserving stable internal session ids.
- Support explicit bounded workflow cycles for retry, revision, re-review, and
  human checkpoint loops.
- Treat AI-inferred build parallelization as a later capability; v1
  parallelism must be declared and write-scope safe.
- Prefer persistent sessions while a role remains active.
- Allow workflow jobs to require fresh context.
- Use Python for the first implementation.
- Assemble work packets from separate role definitions, generic context docs,
  task prompts, completion protocol, and artifact/write-scope requirements.
- Treat native sub-agents spawned inside an agent CLI as part of the parent
  session unless explicitly registered as first-class `agent_runner` sessions.
- Provide coordinator skills for prompt resolution, context loading, work packet
  assembly, confirmation checks, and dispatch through the control plane.
- Let the coordinator execute repo-defined prompts or workflows, for example:
  "read prompt foo."
- Use JSON workflow files.
- Confirm branch creation/selection before starting a run.
- Record decisions as durable artifacts.
- Do not capture broad transcripts by default.
- Use RFC-ledger cleanup as the first validation workflow.
- Require separate Claude, Codex, and Gemini design inputs for the first MVP
  design pass.
- Bootstrap orchestration may reuse the Engram tmux runner temporarily, but the
  product architecture should remain generic.
- Use Engram as reference customer/context while keeping core product logic
  generic and extractable.
