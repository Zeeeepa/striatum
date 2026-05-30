# striatum PRD

Status: V1 shipped
Date: 2026-05-07

The seed thesis and seed requirements below were collected through the
interview recorded in `docs/INTERVIEW_LOG.md` and decisions recorded in
`docs/DECISION_LOG.md`. Shared terms are defined in
`docs/UBIQUITOUS_LANGUAGE.md`. The implementation contract for V1 is
`docs/SPEC.md`; this file remains the product framing.

The V1 surface that has shipped covers the Seed Requirements below. Engram
is the reference customer and first validation fixture; the runner itself is
generic. Remaining product work is tracked in `docs/TODO.md`.

Current status note (2026-05-18): the seed decisions below preserve the
product framing that led to V1, but several substrate/interface decisions
have been superseded by D094 and D104. Current Striatum live state is
daemon-owned PostgreSQL scoped per registered target repository; `.striatum/`
is operational scratch; CLI, MCP/chat tools, and the local web service are
clients of the daemon boundary.

Accepted foundation:

- D001: Start with a PRD backed by a decision log. Use RFCs for contested
  architecture branches, then write the implementation spec after product
  boundaries stabilize.
- D002: Build `striatum` as a generic local terminal-agent orchestrator
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
- D006: Superseded by D094/D104 for production runtime. V1 used SQLite as
  the live coordination layer; current live coordination is daemon-owned
  PostgreSQL. Repo files remain durable artifacts, not the live message bus.
- D007: Superseded by D094/D104 for production runtime. V1 stored run state
  inside the target repo under `.striatum/`; current `.striatum/` usage is
  operational scratch and migration context only.
- D008: Superseded by D094/D104 for production runtime. V1 modelled SQLite
  as event log and local queue; current live events and queues are daemon
  Postgres rows.
- D009: Superseded by D094/D104 for production runtime where it names SQLite.
  Agents still update orchestration state through the `striatum` CLI/MCP/web
  surfaces, which are daemon clients over the same invariants.
- D010: Agents identify themselves through registered sessions before claiming
  work. Work packets are selected by role, lane, capabilities, run, and
  workflow state.
- D011: Prefer persistent agent sessions until the assigned role expires, while
  allowing fresh sessions where context reset matters.
- D012: Use opaque `session_id` plus human-readable `<role>-<lane>-<ordinal>`
  runtime slugs.
- D013: Use CLI as the primary agent control surface.
- D014: Use mostly DAG-shaped workflows with bounded retry/revision cycles.
- D015: V1 parallelism is workflow-declared and write-scope safe; AI-inferred
  build parallelization is deferred.
- D016: Build outputs are durable and idempotent.
- D017: Coordinator starts/selects a branch and requests commits from the
  human by default.
- D018: Implement v1 in Python. *(Superseded by RFC 0078: the Python runtime is
  retired and the `src/` tree removed; the product is Go-only — `striatum`,
  `striatumd`, `striatum-supervisor-helper` build from `go/`.)*
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
- D033: Incubate `striatum` inside Engram through MVP design/build, then
  split it into a separate project.

Seed thesis:

`striatum` is a local-first orchestration tool for coordinating multiple
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
- Use daemon-owned PostgreSQL for local live state: runs, jobs, messages,
  events, verdicts, process metadata, and artifact references.
- Store operational scratch under `.striatum/` in the target repo by
  default; repository files remain durable provenance.
- Provide queue semantics for work delivery, acknowledgements, leases, retries,
  blockers, and completion signals.
- Provide daemon method clients (CLI, MCP/chat tools, and local web service)
  that agents use to mutate orchestration state instead of writing live state
  directly.
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
  session unless explicitly registered as first-class `striatum` sessions.
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

## V1 Surface (shipped)

V1 satisfies the Seed Requirements above through the surface documented in
`docs/SPEC.md` and `README.md`:

- Current live state in daemon-owned PostgreSQL scoped per registered target
  repository, with `.striatum/` retained as operational scratch and
  pre-D094 SQLite only as migration/tombstone/fixture material.
- JSON workflow validation, snapshots, dry-run planning, Mermaid/JSON graph
  export, and `workflow init` starter trees (`minimal`, `review`,
  `code-change`).
- Confirmation-gated branch start with records-only default plus opt-in
  `--create`, `--use-current`, and `--strict` git enforcement.
- Sessions, leases, claim-next, ack, heartbeat, release, block, complete,
  verdict, and `submit-review`, with declared-cycle revision routing and a
  `human_checkpoint` fallback when no safe cycle exists.
- Artifact publishing with path/scope/kind validation, default-deny
  evidence redaction, and per-kind front-matter schemas for `decision`,
  `finding`, `findings_ledger`, and `synthesis`.
- Owner decisions through `decision record` (artifact kind `decision`,
  schema `striatum.decision.v1`), no active lease required.
- Introspection through `status`, `why`, `doctor --verbose`, and a
  dependency-free terminal `dashboard`.
- Stale-lease recovery (`recovery stale-leases`, `recovery requeue-stale`)
  that distinguishes review-only from repo-write work.
- A four-level adapter constraint model (`enforced`, `advisory_strict`,
  `advisory`, `unsupported`) enforced at workflow validation; the local
  process adapter graduates `network` and `repo_scope` to
  `advisory_strict`.
- A single-shot local process adapter (`adapter run`) and long-lived
  supervised sessions (`supervise start | send | stop | status | list`,
  RFC 0009).
- Opt-in per-job git worktree isolation (RFC 0008) for parallel repo-write
  jobs, with `worktree create | release | list`.
- A minimal local Python API (`striatum.api.invoke`) for local authoring/test
  compatibility, plus daemon RPC/MCP surfaces for production run authority.

Out of V1: hosted services, durable transcript provenance, web/Slack
dashboards, plugin marketplaces, automatic commits, and AI-inferred
build parallelization. Remaining product work is enumerated in
`docs/TODO.md`.
