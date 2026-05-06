# Ubiquitous Language

Status: draft
Date: 2026-05-06

This document defines the shared vocabulary for `agent_runner`. Terms should be
updated when decisions sharpen the product model.

## Core Terms

| Term | Definition |
|------|------------|
| agent | A terminal-based AI coding process launched or supervised by `agent_runner`, such as Claude Code, Codex, Gemini CLI, or another configured model/runtime command. |
| agent lane | A named, portable launch configuration for an agent, including command template, adapter, capabilities, and optional default role. Lanes are configuration, not product identity. |
| agent identity | The registered identity of an agent session, including session id, role, lane, capabilities, run id, and optional human-readable name. |
| agent slug | A human-readable session name, recommended as `<role>-<model>-<ordinal>`, used for tmux windows, dashboards, logs, and prompts. It is not the database primary key. |
| adapter | Code that connects `agent_runner` to an external execution or interaction surface, such as tmux, a subprocess, Slack, or a model CLI. |
| binary | The `agent_runner` executable that agents and humans invoke to read or mutate orchestration state. The binary owns SQLite writes and invariant enforcement. |
| artifact | A durable output that should be reviewable after the live run ends, often stored in the target repository. Examples: findings, syntheses, prompt drafts, marker summaries, PRDs, specs, and decision logs. |
| artifact publisher | The component that turns selected state-store messages or job outputs into durable repository artifacts. |
| coordinator | The control role for a workflow. In the hybrid model, the deterministic coordinator owns state and gates, while one selected model lane may act as the AI coordinator for conversational project management. |
| deterministic coordinator | The non-AI control plane that owns workflow state, gates, process launch, retries, stop conditions, message routing, write-scope checks, and durable state updates. |
| AI coordinator | The selected model lane that the user can chat with for a workflow or phase. It is instantiated with a project-manager prompt and focuses on goal tracking, blocker triage, next actions, human checkpoints, and invoking workflow commands. It does not synthesize major artifacts unless assigned a synthesis job. |
| coordinator skill | A deterministic, invocable coordinator capability exposed through chat and backed by `agent_runner` commands, such as resolving a prompt artifact, assembling a work packet, or dispatching a job. |
| synthesis job | A workflow job that combines findings, reviews, or intermediate artifacts into a new durable artifact. It is intentionally separate from the AI coordinator role to avoid attention dilution. |
| workflow | A configured graph of jobs, dependencies, gates, allowed write scopes, agent lanes, expected artifacts, and stop conditions. |
| workflow cycle | An explicit bounded loop in a workflow graph, such as revision -> re-review -> proceed/stop. Unbounded autonomous cycles are out of scope for v1. |
| workflow snapshot | The immutable JSON workflow body and hash loaded into SQLite for one run, so later file edits do not silently change the run contract. |
| workflow fixture | A checked-in example workflow used to validate orchestration behavior without live model calls. |
| job | One executable unit in a workflow, such as draft, review, synthesis, build, test, or human checkpoint. |
| task envelope | A structured instruction packet sent to an agent session for one job or subtask. It includes job id, objective, inputs, allowed write scope, expected artifacts, completion protocol, and stop/block conditions. |
| agent session | A live agent process with conversational or terminal context that may receive one or more task envelopes. |
| native sub-agent | A sub-agent spawned by a supported agent CLI inside an agent session. In v1 it is treated as part of the parent session unless registered as a first-class `agent_runner` session. |
| first-class session | An agent session registered directly with `agent_runner`, with its own identity, role, lane, queue claims, artifacts, heartbeats, and audit trail. |
| work packet | The task envelope returned to an identified agent session when it claims or receives work. |
| context doc | A generic workflow- or project-provided document bundle that orients an agent session before or alongside a work packet. Context docs should not be role-specific by default. |
| role definition | A reusable artifact that defines an agent's responsibility, stance, allowed behavior, and non-responsibilities. Task prompts may reference role definitions and add job-specific emphasis. |
| run | One execution attempt of a workflow against a repository/workspace. |
| state store | The local SQLite coordination database for runs, jobs, messages, events, verdicts, process metadata, and artifact references. |
| event log | The append-only SQLite record of facts that happened during a run, such as job started, message sent, verdict recorded, or workflow stopped. |
| message bus | The SQLite-backed local communication layer used by agents, coordinators, and adapters for live structured messages. It is not the repository artifact layer. |
| message queue | The actionable SQLite-backed queue for work delivery and coordination messages. Queue messages can be pending, claimed, acknowledged, failed, or expired. |
| lease | A time-bounded claim on a queued message or job. Leases prevent two agents from taking the same work and allow recovery if a session dies. |
| lazy lease expiry | The V1 policy where CLI commands detect and expire stale leases during normal mutations instead of relying on a background daemon. |
| stale lease | An expired or abandoned lease whose job cannot safely be requeued automatically, especially when repo-write scope may have been touched. |
| mutation command | A binary command that changes orchestration state, such as `claim-next`, `ack`, `block`, `complete`, or `verdict`. |
| command request | An idempotency record for a CLI mutation attempt, used to return the same result when an agent repeats the same request id and payload. |
| message | A structured communication record in the state store, such as a blocker, review verdict, finding, handoff, or human checkpoint request. |
| event | An append-only state transition record, such as job started, message sent, verdict recorded, or workflow stopped. |
| marker | A durable summary artifact indicating that a job reached a terminal state. Markers are useful provenance, but not the live message bus. |
| review gate | A workflow control point that evaluates review verdicts and decides whether to proceed, revise, re-review, stop, or request human input. |
| verdict | A structured review outcome: `accept`, `accept_with_findings`, `needs_revision`, or `reject`. |
| revision lane | The configured path that applies accepted review feedback after a `needs_revision` verdict. |
| re-review | A second review pass over a revised artifact. A second reject or unresolved revision request stops the workflow by default. |
| human checkpoint | A workflow stop or pause requiring explicit human judgment before continuing. |
| write scope | The set of files or directories a job is allowed to modify. Used to protect same-branch collaboration and dirty worktrees. |
| parallelism policy | The workflow rule that decides whether multiple jobs, including jobs with the same role, may run at the same time. V1 policy is declared parallelism plus disjoint write scopes or review-only artifacts. |
| PTY adapter | An adapter that runs agents in pseudo-terminal sessions. Tmux is the first expected PTY adapter. |
| process contract | The minimum common integration boundary for agent lanes: command, current working directory, environment, stdin, stdout, stderr, exit code, and optional PTY. |
| dashboard | A human-facing status surface. The first dashboard may be TUI; a web dashboard and chat interface are possible later. |
| model portability | The design goal that workflows, state, and coordination semantics survive swapping model providers, model versions, and model CLIs. |

## Distinctions

- A **message** is live coordination state; an **artifact** is durable project
  provenance.
- A **marker** is an artifact summarizing completion; it is not sufficient as a
  message bus.
- An **agent lane** is portable configuration; it should not be treated as a
  hardcoded provider identity.
- The **deterministic coordinator** enforces gates; the **AI coordinator**
  helps the human move the workflow through those gates.
