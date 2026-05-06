# Ubiquitous Language

Status: draft
Date: 2026-05-06

This document defines the shared vocabulary for `agent_runner`. Terms should be
updated when decisions sharpen the product model or the implementation exposes
new operator-facing behavior.

`agent_runner` is generic local terminal-agent orchestration. Engram is the
incubation repository, reference customer, and first validation fixture; it is
not the product boundary.

## Core Terms

| Term | Definition |
|------|------------|
| agent | A terminal-based AI coding process launched or supervised by `agent_runner`, such as Claude Code, Codex, Gemini CLI, or another configured model/runtime command. |
| agent lane | A named, portable launch configuration for an agent, including command template, adapter, capabilities, and optional default role. Lanes are configuration, not product identity. |
| agent identity | The registered identity of an agent session, including session id, role, lane, capabilities, run id, and optional human-readable name. |
| agent slug | A human-readable session name, recommended as `<role>-<lane>-<ordinal>`, used for tmux windows, dashboards, logs, and prompts. It is not the database primary key. |
| adapter | Code that connects `agent_runner` to an external execution or interaction surface, such as tmux, a subprocess, Slack, or a model CLI. |
| adapter boundary | The line between core scheduling semantics and provider-specific execution details. V1 records lane commands and constraints, but core scheduling does not parse terminal output or infer behavior from provider names. |
| binary | The `agent_runner` executable that agents and humans invoke to read or mutate orchestration state. The binary owns SQLite writes and invariant enforcement. |
| repo-local control plane | The deterministic local control layer that stores and mutates workflow state for one target repository. In V1 this is the `agent_runner` CLI plus SQLite under `.agent_runner/`. |
| target repository | The repository/workspace being orchestrated by a run. It may be Engram during incubation, but the term is generic and should be used instead of assuming Engram. |
| incubation repository | The repository where `agent_runner` temporarily lives before standalone extraction. Today this is Engram; core product logic should not depend on that. |
| reference fixture | A checked-in workflow or scenario used to validate the runner. Fixtures may come from Engram, but they are examples of generic orchestration behavior. |
| live state | Authoritative mutable run state used by the control plane. V1 live state lives in `.agent_runner/state.sqlite3`, not in marker files, terminal panes, or committed reports. |
| durable provenance | Repository artifacts that make the run auditable after the live state is gone or ignored. Provenance can describe the run but is not the live message bus. |
| artifact | A durable output that should be reviewable after the live run ends, often stored in the target repository. Examples: findings, syntheses, prompt drafts, marker summaries, PRDs, specs, decision logs, and redacted evidence exports. |
| artifact author identity | Stable authorship metadata for a job or artifact: role id, lane id, declared model display name, and workflow job id. It is used instead of free-text workflow job titles in evidence exports, not as the visible artifact byline. |
| artifact title block | The human-readable top matter of a durable artifact. It may include title, date, status, target, verdict, and author metadata, depending on artifact kind. |
| artifact author | The privacy-safe human-facing byline inside a durable artifact title block. Current convention is `author: <role-name>-<model-name>-<ordinal>`, for example `author: reviewer-codex-gpt-5.5-001`. It is for readers, not database identity, and intentionally excludes workflow job titles. |
| author line | A Markdown artifact title-block line formatted as `author: <role-name>-<model-name>-<ordinal>`, derived from the role and model identity visible to the workflow. |
| artifact publisher | The component that validates and records selected job outputs as durable repository artifacts. It checks lease ownership, write scope, path safety, artifact kind, file existence, and content hash. |
| evidence export | A redacted Markdown repository artifact generated from live runner state so a run can be audited from a fresh checkout without committing `.agent_runner/` SQLite data. |
| redaction | The act of replacing free-text fields that may contain user or agent prose, such as blocker descriptions and verdict rationales, before publishing an evidence export. |
| coordinator | The control role for a workflow. In the hybrid model, the deterministic coordinator owns state and gates, while one selected model lane may act as the AI coordinator for conversational project management. |
| deterministic coordinator | The non-AI control plane that owns workflow state, gates, process launch, retries, stop conditions, message routing, write-scope checks, and durable state updates. |
| AI coordinator | The selected model lane that the user can chat with for a workflow or phase. It is instantiated with a project-manager prompt and focuses on goal tracking, blocker triage, next actions, human checkpoints, and invoking workflow commands. It does not synthesize major artifacts unless assigned a synthesis job. |
| next action | A deterministic coordinator suggestion returned by status or introspection commands, such as resolving a human checkpoint, inspecting a blocker, claiming available work, or exporting run evidence. |
| coordinator skill | A deterministic, invocable coordinator capability exposed through chat and backed by `agent_runner` commands, such as resolving a prompt artifact, assembling a work packet, or dispatching a job. |
| synthesis job | A workflow job that combines findings, reviews, or intermediate artifacts into a new durable artifact. It is intentionally separate from the AI coordinator role to avoid attention dilution. |
| workflow | A configured graph of jobs, dependencies, gates, allowed write scopes, agent lanes, expected artifacts, and stop conditions. |
| workflow config | The user-authored JSON file that defines a workflow. V1 requires `agent-runner.workflow.v1` JSON and rejects YAML. |
| workflow cycle | An explicit bounded loop in a workflow graph, such as revision -> re-review -> proceed/stop. Unbounded autonomous cycles are out of scope for v1. |
| workflow snapshot | The immutable JSON workflow body and hash loaded into SQLite for one run, so later file edits do not silently change the run contract. |
| workflow fixture | A checked-in example workflow used to validate orchestration behavior without live model calls. |
| review revision policy | Workflow configuration that states how root-review `needs_revision` verdicts are routed, for example to a human checkpoint or to declared revision cycles. |
| root review | A review job with no upstream workflow dependency. Root reviews need explicit `needs_revision` policy because there may be no natural author/revision job upstream. |
| job | One executable unit in a workflow, such as draft, review, synthesis, build, test, or human checkpoint. |
| task envelope | A structured instruction packet sent to an agent session for one job or subtask. It includes job id, objective, inputs, allowed write scope, expected artifacts, completion protocol, and stop/block conditions. |
| agent session | A live agent process with conversational or terminal context that may receive one or more task envelopes. |
| native sub-agent | A sub-agent spawned by a supported agent CLI inside an agent session. In v1 it is treated as part of the parent session unless registered as a first-class `agent_runner` session. |
| first-class session | An agent session registered directly with `agent_runner`, with its own identity, role, lane, queue claims, artifacts, heartbeats, and audit trail. |
| session slug | The human-readable runtime label stored with an agent session. V1 uses role, lane, and ordinal for uniqueness and scheduling readability; artifact authorship uses the more descriptive role/model/ordinal author line. |
| work packet | The task envelope returned to an identified agent session when it claims or receives work. |
| context doc | A generic workflow- or project-provided document bundle that orients an agent session before or alongside a work packet. Context docs should not be role-specific by default. |
| role definition | A reusable artifact that defines an agent's responsibility, stance, allowed behavior, and non-responsibilities. Task prompts may reference role definitions and add job-specific emphasis. |
| run | One execution attempt of a workflow against a repository/workspace. |
| prepared run | A run whose workflow JSON has been validated and snapshotted but whose jobs are not yet claimable because branch confirmation and run start have not both happened. |
| branch confirmation | The explicit human confirmation that a run should proceed on a named branch. It gates claims before `run start`. |
| records-only branch confirmation | V1 branch confirmation behavior: `branch confirm` records the requested branch and reports the detected current git branch, but does not create, switch, commit, push, merge, or rebase. |
| state store | The local SQLite coordination database for runs, jobs, messages, events, verdicts, process metadata, and artifact references. |
| state database | The concrete V1 state store file at `.agent_runner/state.sqlite3` inside the target repository. It is ignored by git and should not be committed. |
| event log | The append-only SQLite record of facts that happened during a run, such as job started, message sent, verdict recorded, or workflow stopped. |
| message bus | The SQLite-backed local communication layer used by agents, coordinators, and adapters for live structured messages. It is not the repository artifact layer. |
| message queue | The actionable SQLite-backed queue for work delivery and coordination messages. Queue messages can be pending, claimed, acknowledged, blocked, completed, released, expired, canceled, or dead. |
| work message | A queue message of kind `work` that makes one workflow job claimable by a matching session. |
| claimable work | Pending work messages whose run is started, dependencies are satisfied, and target role/lane match an active session. |
| lease | A time-bounded claim on a queued message or job. Leases prevent two agents from taking the same work and allow recovery if a session dies. |
| acknowledgement | The transition where a session accepts claimed work and moves the job from claimed to running via `ack`. Completion is invalid until work has been acknowledged. |
| heartbeat | A liveness update that refreshes the session timestamp and extends an active lease during long work. |
| lazy lease expiry | The V1 policy where CLI commands detect and expire stale leases during normal mutations instead of relying on a background daemon. |
| stale lease | An expired or abandoned lease whose job cannot safely be requeued automatically, especially when repo-write scope may have been touched. |
| mutation command | A binary command that changes orchestration state, such as `claim-next`, `ack`, `block`, `complete`, or `verdict`. |
| command request | An idempotency record for a CLI mutation attempt, used to return the same result when an agent repeats the same request id and payload. |
| message | A structured communication record in the state store, such as a blocker, review verdict, finding, handoff, or human checkpoint request. |
| event | An append-only state transition record, such as job started, message sent, verdict recorded, or workflow stopped. |
| marker | A durable summary artifact indicating that a job reached a terminal state. Markers are useful provenance, but not the live message bus. |
| blocker | A recorded reason a job or run cannot proceed normally. Blockers can be normal blocked-state reports or human checkpoints. |
| open blocker | A blocker whose state is still `open`; `status --json` and `why <blocker_id> --json` surface these for recovery. |
| review gate | A workflow control point that evaluates review verdicts and decides whether to proceed, revise, re-review, stop, or request human input. |
| verdict | A structured review outcome: `accept`, `accept_with_findings`, `needs_revision`, or `reject`. |
| accepting verdict | `accept` or `accept_with_findings`. Downstream jobs gated on review completion require an accepting verdict. |
| non-accepting verdict | `needs_revision` or `reject`. These verdicts either request a revision route, open a human checkpoint, or fail the review/run. |
| submit-review | A convenience mutation command that validates a review job, publishes the review artifact, records the verdict, and applies review-gate behavior in one operation. |
| revision lane | The configured path that applies accepted review feedback after a `needs_revision` verdict. |
| re-review | A second review pass over a revised artifact. A second reject or unresolved revision request stops the workflow by default. |
| human checkpoint | A workflow stop or pause requiring explicit human judgment before continuing. |
| write scope | The set of files or directories a job is allowed to modify. Used to protect same-branch collaboration and dirty worktrees. |
| review-only artifact scope | A write scope for review jobs that permits publishing unique review artifacts but not modifying the source artifacts under review. |
| parallelism policy | The workflow rule that decides whether multiple jobs, including jobs with the same role, may run at the same time. V1 policy is declared parallelism plus disjoint write scopes or review-only artifacts. |
| PTY adapter | An adapter that runs agents in pseudo-terminal sessions. Tmux is the first expected PTY adapter. |
| process contract | The minimum common integration boundary for agent lanes: command, current working directory, environment, stdin, stdout, stderr, exit code, and optional PTY. |
| adapter constraint | A workflow-declared lane requirement, such as network policy, transcript handling, or repository scope, paired with an adapter enforcement result of `enforced`, `advisory`, or `unsupported`. |
| adapter enforcement | The recorded V1 answer to whether an adapter actually enforces a requested constraint. `enforced` means the adapter claims enforcement, `advisory` means the request is visible but externally enforced if at all, and `unsupported` means the adapter cannot represent it. |
| transcript | Broad terminal or model-session output. Transcripts are not captured or published by default; curated artifacts are preferred. |
| bootstrap tmux harness | The temporary Engram-incubation script that starts model panes for early design/build work. It is not the generic product control plane. |
| dashboard | A human-facing status surface. The first dashboard may be TUI; a web dashboard and chat interface are possible later. |
| model portability | The design goal that workflows, state, and coordination semantics survive swapping model providers, model versions, and model CLIs. |

## Distinctions

- **Engram** is the incubation and first fixture; **agent_runner** is a generic
  tool for target repositories.
- **Live state** is SQLite under `.agent_runner/`; **durable provenance** is
  committed or commit-ready repository artifacts.
- A **message** is live coordination state; an **artifact** is durable project
  provenance.
- A **session slug** identifies a runtime session; an **artifact author** is a
  byline for humans reading committed artifacts.
- A **marker** is an artifact summarizing completion; it is not sufficient as a
  message bus.
- An **agent lane** is portable configuration; it should not be treated as a
  hardcoded provider identity.
- **Branch confirmation** in V1 is records-only; git branch creation and
  switching remain external human/operator actions.
- **Adapter constraints** are not automatic sandboxes. Treat `advisory` as a
  visible request that still needs external enforcement.
- The **deterministic coordinator** enforces gates; the **AI coordinator**
  helps the human move the workflow through those gates.
