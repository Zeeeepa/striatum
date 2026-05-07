# Ubiquitous Language

Status: draft
Date: 2026-05-06

This document defines the shared vocabulary for `striatum`. Terms should be
updated when decisions sharpen the product model or the implementation exposes
new operator-facing behavior.

`striatum` is generic local terminal-agent orchestration. Engram is the
incubation repository, reference customer, and first validation fixture; it is
not the product boundary.

## Core Terms

| Term | Definition |
|------|------------|
| agent | A terminal-based AI coding process launched or supervised by `striatum`, such as Claude Code, Codex, Gemini CLI, or another configured model/runtime command. |
| agent lane | A named, portable launch configuration for an agent, including command template, adapter, capabilities, and optional default role. Lanes are configuration, not product identity. |
| agent identity | The registered identity of an agent session, including session id, role, lane, capabilities, run id, and optional human-readable name. |
| agent slug | A human-readable session name, recommended as `<role>-<lane>-<ordinal>`, used for tmux windows, dashboards, logs, and prompts. It is not the database primary key. |
| adapter | Code that connects `striatum` to an external execution or interaction surface, such as tmux, a subprocess, Slack, or a model CLI. |
| adapter boundary | The line between core scheduling semantics and provider-specific execution details. V1 records lane commands and constraints, but core scheduling does not parse terminal output or infer behavior from provider names. |
| binary | The `striatum` executable that agents and humans invoke to read or mutate orchestration state. The binary owns SQLite writes and invariant enforcement. |
| repo-local control plane | The deterministic local control layer that stores and mutates workflow state for one target repository. In V1 this is the `striatum` CLI plus SQLite under `.striatum/`. |
| target repository | The repository/workspace being orchestrated by a run. It may be Engram during incubation, but the term is generic and should be used instead of assuming Engram. |
| incubation repository | The repository where `striatum` temporarily lives before standalone extraction. Today this is Engram; core product logic should not depend on that. |
| reference fixture | A checked-in workflow or scenario used to validate the runner. Fixtures may come from Engram, but they are examples of generic orchestration behavior. |
| live state | Authoritative mutable run state used by the control plane. V1 live state lives in `.striatum/state.sqlite3`, not in marker files, terminal panes, or committed reports. |
| durable provenance | Repository artifacts that make the run auditable after the live state is gone or ignored. Provenance can describe the run but is not the live message bus. |
| artifact | A durable output that should be reviewable after the live run ends, often stored in the target repository. Examples: findings, syntheses, prompt drafts, marker summaries, PRDs, specs, decision logs, and redacted evidence exports. |
| artifact author identity | Stable authorship metadata for a job or artifact: role id, lane id, declared model display name, and workflow job id. It is used instead of free-text workflow job titles in evidence exports, not as the visible artifact byline. |
| artifact title block | The human-readable top matter of a durable artifact. It may include title, date, status, target, verdict, and author metadata, depending on artifact kind. |
| artifact author | The privacy-safe human-facing byline inside a durable artifact title block. Current convention is `author: <role-name>-<model-name>-<ordinal>`, for example `author: reviewer-codex-gpt-5.5-001`. It is for readers, not database identity, and intentionally excludes workflow job titles. |
| author line | A Markdown artifact title-block line formatted as `author: <role-name>-<model-name>-<ordinal>`, derived from the role and model identity visible to the workflow. |
| artifact publisher | The component that validates and records selected job outputs as durable repository artifacts. It checks lease ownership, write scope, path safety, artifact kind, file existence, and content hash. |
| artifact front matter | An optional YAML-style `---`-delimited block at the top of a durable Markdown artifact carrying machine-checkable metadata. V1 uses a `key: <json-value>` line format so no YAML dependency is required; nested mappings are not supported. The publisher records artifacts but never rewrites them. |
| front-matter schema | A registered per-kind schema for `artifact front matter`. V1 defines `striatum.decision.v1` (kind `decision`), `striatum.finding.v1` (kind `finding`), `striatum.findings_ledger.v1` (kind `findings_ledger`), and `striatum.synthesis.v1` (kind `synthesis`). When a file with a front-matter block is published under a kind that has a registered schema, validation runs and rejects invalid metadata; files without a front-matter block remain accepted. |
| evidence export | A redacted Markdown repository artifact generated from live runner state so a run can be audited from a fresh checkout without committing `.striatum/` SQLite data. |
| redaction | The act of replacing free-text fields that may contain user or agent prose, such as blocker descriptions and verdict rationales, before publishing an evidence export. |
| coordinator | The control role for a workflow. In the hybrid model, the deterministic coordinator owns state and gates, while one selected model lane may act as the AI coordinator for conversational project management. |
| deterministic coordinator | The non-AI control plane that owns workflow state, gates, process launch, retries, stop conditions, message routing, write-scope checks, and durable state updates. |
| AI coordinator | The selected model lane that the user can chat with for a workflow or phase. It is instantiated with a project-manager prompt and focuses on goal tracking, blocker triage, next actions, human checkpoints, and invoking workflow commands. It does not synthesize major artifacts unless assigned a synthesis job. |
| next action | A deterministic coordinator suggestion returned by status or introspection commands, such as resolving a human checkpoint, inspecting a blocker, claiming available work, or exporting run evidence. |
| coordinator skill | A deterministic, invocable coordinator capability exposed through chat and backed by `striatum` commands, such as resolving a prompt artifact, assembling a work packet, or dispatching a job. |
| synthesis job | A workflow job that combines findings, reviews, or intermediate artifacts into a new durable artifact. It is intentionally separate from the AI coordinator role to avoid attention dilution. |
| workflow | A configured graph of jobs, dependencies, gates, allowed write scopes, agent lanes, expected artifacts, and stop conditions. |
| workflow config | The user-authored JSON file that defines a workflow. V1 requires `striatum.workflow.v1` JSON and rejects YAML. |
| workflow cycle | An explicit bounded loop in a workflow graph, such as revision -> re-review -> proceed/stop. Unbounded autonomous cycles are out of scope for v1. |
| workflow snapshot | The immutable JSON workflow body and hash loaded into SQLite for one run, so later file edits do not silently change the run contract. |
| workflow fixture | A checked-in example workflow used to validate orchestration behavior without live model calls. |
| review revision policy | Workflow configuration that states how root-review `needs_revision` verdicts are routed, for example to a human checkpoint or to declared revision cycles. |
| reviewer access scope | The optional `reviewer_access_scope` workflow field on a review job (RFC 0002). One of `document_only` (read only the target documents in `inputs`), `artifact_augmented` (also read supporting artifacts/reports/ledgers in `inputs`), or `repo_level` (inspect the repository within the job's declared `write_scope`). The work packet exposes the value in a `review_policy` block. |
| reviewer context policy | The optional `reviewer_context_policy` workflow field on a review job (RFC 0002). One of `fresh` (require a brand-new role/session and avoid prior thread state) or `cross_round` (retain context to verify earlier issues were resolved). Setting it to `fresh` silently implies `fresh_session_required: true`; explicitly combining `fresh` with `fresh_session_required: false` is rejected. |
| fresh-context review | A review job whose `reviewer_context_policy` is `fresh`. The reviewer must be instantiated as a new role/session and must not rely on prior round state when forming a verdict. |
| cross-round review | A review job whose `reviewer_context_policy` is `cross_round`. The reviewer may retain context across rounds specifically to verify whether previously raised issues were resolved. |
| root review | A review job with no upstream workflow dependency. Root reviews need explicit `needs_revision` policy because there may be no natural author/revision job upstream. |
| job | One executable unit in a workflow, such as draft, review, synthesis, build, test, or human checkpoint. |
| task envelope | A structured instruction packet sent to an agent session for one job or subtask. It includes job id, objective, inputs, allowed write scope, expected artifacts, completion protocol, and stop/block conditions. |
| agent session | A live agent process with conversational or terminal context that may receive one or more task envelopes. |
| native sub-agent | A sub-agent spawned by a supported agent CLI inside an agent session. In v1 it is treated as part of the parent session unless registered as a first-class `striatum` session. |
| first-class session | An agent session registered directly with `striatum`, with its own identity, role, lane, queue claims, artifacts, heartbeats, and audit trail. |
| session slug | The human-readable runtime label stored with an agent session. V1 uses role, lane, and ordinal for uniqueness and scheduling readability; artifact authorship uses the more descriptive role/model/ordinal author line. |
| work packet | The task envelope returned to an identified agent session when it claims or receives work. |
| context doc | A generic workflow- or project-provided document bundle that orients an agent session before or alongside a work packet. Context docs should not be role-specific by default. |
| role definition | A reusable artifact that defines an agent's responsibility, stance, allowed behavior, and non-responsibilities. Task prompts may reference role definitions and add job-specific emphasis. |
| run | One execution attempt of a workflow against a repository/workspace. |
| prepared run | A run whose workflow JSON has been validated and snapshotted but whose jobs are not yet claimable because branch confirmation and run start have not both happened. |
| branch confirmation | The explicit human confirmation that a run should proceed on a named branch. It gates claims before `run start`. |
| records-only branch confirmation | V1 branch confirmation behavior: `branch confirm` records the requested branch and reports the detected current git branch, but does not create, switch, commit, push, merge, or rebase. |
| state store | The local SQLite coordination database for runs, jobs, messages, events, verdicts, process metadata, and artifact references. |
| state database | The concrete V1 state store file at `.striatum/state.sqlite3` inside the target repository. It is ignored by git and should not be committed. |
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
| worktree-isolated job | A repo-write job whose lane sets `worktree_isolation: "per_job"` and whose work packet therefore advertises `worktree_required: true`. The agent must call `striatum worktree create` before publishing artifacts; `publish-artifact` reads files from the per-job worktree but records the logical repo-relative path so artifacts remain valid main-branch provenance. |
| job worktree | The git worktree allocated for a `worktree-isolated job`, tracked in the `job_worktrees` SQLite table (added in migration version 2). It lives under `.striatum/worktrees/<worktree_id>` and is created via `git worktree add --detach`. Lazy lease expiry marks the row `abandoned` for operator inspection without removing the directory; `striatum doctor` flags orphaned and missing-on-disk worktree rows. |
| PTY adapter | An adapter that runs agents in pseudo-terminal sessions. Tmux is the first expected PTY adapter. |
| process contract | The minimum common integration boundary for agent lanes: command, current working directory, environment, stdin, stdout, stderr, exit code, and optional PTY. |
| adapter constraint | A workflow-declared lane requirement, such as network policy, transcript handling, or repository scope, paired with an adapter enforcement result of `enforced`, `advisory_strict`, `advisory`, or `unsupported`. |
| adapter enforcement | The recorded V1 answer to whether an adapter actually enforces a requested constraint. `enforced` means the adapter prevents the constraint from being violated; `advisory_strict` means the adapter takes best-effort steps the agent cannot easily undo (such as scrubbing proxy env vars or setting `STRIATUM_NETWORK_POLICY` / `STRIATUM_REPO_SCOPE` sentinels); `advisory` means the request is visible but externally enforced if at all; `unsupported` means the adapter cannot represent it. |
| advisory_strict | The fourth adapter enforcement level introduced after V1's first three. It signals that the adapter has taken concrete, non-trivial steps toward the constraint (env scrubbing, sentinel env vars) without claiming full sandbox enforcement. Used by the process adapter for `network=forbidden` and `repo_scope=local_only`. |
| transcript | Broad terminal or model-session output. Transcripts are not captured or published by default; curated artifacts are preferred. |
| bootstrap tmux harness | The temporary Engram-incubation script that starts model panes for early design/build work. It is not the generic product control plane. |
| dashboard | A human-facing status surface. The first dashboard may be TUI; a web dashboard and chat interface are possible later. |
| run dashboard | The V1 implementation of the dashboard term: `striatum dashboard --run-id <id>`, a compact dependency-free terminal renderer over the same SQLite state as `status` and `why`. Refreshes every 2 seconds by default; `--once` produces a single frame to stdout for scripts and CI. |
| supervisor | A long-lived agent CLI process held alive across multiple work packets via `striatum supervise start` (RFC 0009). State is tracked in the `process_supervisors` SQLite table (added in migration version 4); allowed states are `starting`, `attached`, `detached`, `lost`, and `stopped`. A partial unique index enforces "at most one active supervisor per session". |
| supervised session | An agent session whose lane uses the `process` adapter and whose process is held by a supervisor. Stdout and stderr are sent to `DEVNULL` so the supervisor never captures transcripts. Lease expiry transitions the supervisor row to `lost` without auto-killing the OS process; operator inspection is required. |
| supervisor pipe | The per-supervisor named FIFO at `.striatum/scratch/<supervisor_id>/stdin.pipe` that `striatum supervise send` writes work packets to as newline-terminated JSON lines. The supervised agent reads packets line-by-line from stdin and reacts through normal CLI commands; the supervisor never parses agent output. |
| claim auto-delivery | A future enhancement (tracked in `docs/TODO.md`) where a supervised-aware `claim-next` would route work packets through `supervise send` to an existing supervisor instead of spawning a fresh single-shot process. Not yet shipped in V1. |
| model portability | The design goal that workflows, state, and coordination semantics survive swapping model providers, model versions, and model CLIs. |

## Distinctions

- **Engram** is the incubation and first fixture; **striatum** is a generic
  tool for target repositories.
- **Live state** is SQLite under `.striatum/`; **durable provenance** is
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
