---
schema_version: striatum.handoff.v1
artifact_kind: handoff
tool_family: codex
strategy_version: "2026-05-08"
tags: ["dogfood-003", "rfc-0010", "codex"]
---

# Codex Harness Profile — Refresh For Dogfood-003

author: researcher-codex-gpt-5.5-001

Date: 2026-05-08
Verified-against: `docs/research/0010-tool-harness-profiles/codex.md` (2026-05-08),
`docs/rfcs/0010-tool-harness-profiles.md`, `docs/dogfood/003/SOURCES.md`.

This is a verify-and-refresh report. The prior dogfood-001 v2 research note
for Codex is dated 2026-05-08 — the same calendar day as this run — so the
work here is confirmation rather than rediscovery. Where this report agrees
with the prior note, it cites the note and the upstream URL it relied on
rather than re-deriving the claim. Where it disagrees or wants to extend,
it says so explicitly.

## Source list

Primary upstream:

- Codex non-interactive: <https://developers.openai.com/codex/noninteractive>
- Codex CLI reference: <https://developers.openai.com/codex/cli/reference>
- Codex MCP: <https://developers.openai.com/codex/mcp>
- Codex skills: <https://developers.openai.com/codex/skills>
- Codex subagents: <https://developers.openai.com/codex/subagents>
- Codex hooks: <https://developers.openai.com/codex/hooks>
- Codex agent loop (HN/ZenML mirrors): <https://news.ycombinator.com/item?id=46737630>,
  <https://zenml.io/llmops-database/building-production-ready-ai-agents-openai-codex-cli-architecture-and-agent-loop-design>
- Issue gating parallel `codex exec`:
  <https://github.com/openai/codex/issues/11435>
- Codex App Worktrees:
  <https://developers.openai.com/codex/app/worktrees>

Secondary (community guidance):

- `oh-my-codex` worktree pattern, Daniel Vaughan's parallel orchestration
  blog, ZenML LLMOps mirror — see prior note for full URL set.

## RFC 0010 `codex_default` — confirmation

The `codex_default` profile in RFC 0010 (the version dated 2026-05-08
under "Concrete Profile Examples") is consistent with the prior research
note and the current upstream behaviour. The values that matter for the
first build slice are:

| Field | RFC 0010 value | Confirmed? |
|---|---|---|
| `tool_family` | `codex` | yes |
| `strategy_version` | `2026-05-08` | yes |
| `native_delegation.mode` | `encouraged` | yes |
| `native_delegation.instruction` | "Use sub-agents by routing the parent prompt; ship `.codex/agents/<role>.toml` definitions. Spawn parallel `codex exec` instances ONLY when each has its own `CODEX_HOME` and `--ephemeral`, otherwise session state corrupts (openai/codex#11435)." | yes |
| `feature_flags.subagents` | `allowed_via_natural_language_routing` | yes |
| `feature_flags.agent_teams` | `not_supported` | yes |
| `feature_flags.ephemeral_sessions` | `required_for_parallel` | yes |
| `supervision.compatible` | `true` | yes |
| `supervision.stdin_format` | `packet` | yes |
| `supervision.wrapper_required` | `false` | yes |
| `output_format` | `json` | yes |
| `approval_mode` | `default` | yes |
| `memory_files` | `["AGENTS.md"]` | yes |
| `accountability.native_subagents` | `internal_to_parent_session` | yes |
| `agent_loop_budget` | `{auto_compact_limit:280000, model_reasoning_effort:"medium", max_iterations:8}` | confirmed as community-standard guardrails; see "Caveats" below |
| `workspace_isolation` | `{state_dir_per_job:true, rollout_persistence:"off"}` | yes |

Caveats (non-blocking):

- `max_iterations: 8` is community guidance from Daniel Vaughan's parallel
  orchestration blog, not a Codex-enforced cap. The official agent loop
  has no documented upper bound; orchestrators that care must enforce
  iteration count externally. The profile field is therefore advisory in
  V1.
- `agent_loop_budget.auto_compact_limit` corresponds to Codex's own
  `auto_compact_limit` in `config.toml`, but Codex does not document a
  hard ceiling — the value is a knob, not a maximum. Treat it as a hint.

## Coverage of RFC 0010 extended fields

### `supervision`

- `compatible: true` — `codex exec --json -` is non-interactive and does
  not require a PTY (stderr progress, stdout JSON event stream). Long-lived
  supervision against `codex exec` ends after one packet because `codex
  exec` exits when the agent decides the task is done. Striatum's
  supervisor today re-launches the lane command per packet, so this is
  fine; if a future supervisor expects a long-lived stdin pipe, it must
  re-spawn per packet.
- `stdin_format: packet` — pass `-` as the prompt argument; the full
  work packet is the stdin payload. No wrapper required.
- `wrapper_required: false` — verified.
- Future option: `codex app-server --listen stdio://` is JSON-RPC over
  stdio and could become a true long-lived supervised lane. Out of scope
  for V1.

### `workspace_isolation`

- `state_dir_per_job: true` — required because of openai/codex#11435.
  Without per-job `CODEX_HOME`, parallel `codex exec` instances detect
  each other's session files and cross-pollinate context.
- `rollout_persistence: "off"` — corresponds to `--ephemeral`. Consistent
  with Striatum's transcripts-off default (D028).
- Implementation note: Striatum's lane `env` block already supports
  `${STRIATUM_SCRATCH_DIR}/codex-home` interpolation; the dogfood-003
  workflow uses that exact pattern.

### `agent_loop_budget`

- `auto_compact_limit: 280000` — Codex `config.toml` accepts this; it
  triggers internal compaction. Advisory in V1.
- `model_reasoning_effort: "medium"` — valid value
  (`minimal|low|medium|high|xhigh`). Per-agent override via custom-agent
  TOML.
- `max_iterations: 8` — not enforced by Codex; orchestrator-side cap.

### `approval_mode`

- `default` corresponds to Codex's `untrusted` approval mode (the documented
  default for `codex exec`). The dogfood-003 lane command passes
  `--ask-for-approval never`, which is correct for a Striatum-driven
  lane that should not pause for human approval mid-job.
- Recommended profile-to-CLI mapping:
  - `default` → `--ask-for-approval untrusted` (Codex default)
  - `auto_edit` → `--ask-for-approval on-request` with sandbox set to
    `workspace-write`
  - `yolo` → `--dangerously-bypass-approvals-and-sandbox`
  - `plan` → no Codex equivalent; document as not supported.

### `output_format`

- `json` is correct. Codex `--json` emits newline-delimited JSON events
  (`thread.started`, `turn.started`, `turn.completed`, `item.*`, etc.).
  This is the right shape for a Striatum supervisor that wants to parse
  lifecycle events without sniffing stdout text.

### `memory_files`

- `AGENTS.md` is what Codex reads on session start (project root, then
  user-level). The `.codex/AGENTS.md` shape some operators use is a
  per-tool override; the profile lists the canonical filename.

### `mcp_servers`

- The profile leaves this empty (`[]`). Correct for the V1 default
  profile. Codex's `config.toml` `[mcp_servers.<id>]` table is the
  authoring surface; if a workflow wants MCP servers, they should be
  declared at profile level so workflow validation can sanity-check.
- Known papercut: openai/codex#3441 — long-running Codex processes
  silently ignore `config.toml` MCP edits until restart. Striatum should
  not assume mid-run MCP reconfiguration works.

### `turn_caps`

- Codex does not expose `max_session_turns` / `subagent_max_turns` /
  `subagent_timeout_mins` as first-class flags. The closest equivalents
  are `agents.job_max_runtime_seconds` (per-worker, default 1800s) and
  the iteration cap that orchestrators enforce externally.
- Recommendation: omit `turn_caps` from `codex_default` and document that
  Codex's per-worker timeout is the relevant lever instead. The Gemini
  CLI profile is the one that benefits from `turn_caps`.

## Recommended lane command, environment, and wrapper

Lane command (matches dogfood-003 workflow.json):

```bash
codex exec \
  --json \
  --ephemeral \
  --skip-git-repo-check \
  --sandbox workspace-write \
  --ask-for-approval never \
  --ignore-user-config \
  -
```

Environment:

- `CODEX_HOME=${STRIATUM_SCRATCH_DIR}/codex-home` — required to neutralise
  openai/codex#11435 on parallel jobs.
- No additional vars required for the V1 build slice.

Wrapper requirement: none. `codex exec` is the supervised entry point
directly.

## What stays generic, advisory, forbidden, or unsupported

Generic and advisory (kept in V1 profile):

- Native delegation guidance (`encouraged`, with the natural-language
  routing instruction).
- `agent_loop_budget` numeric guardrails.
- `workspace_isolation` (state dir per job, ephemeral rollout).
- `output_format: json`, `approval_mode: default`, `memory_files`.

Forbidden / not-supported in V1:

- First-class registration of native subagents as Striatum sessions.
- Provider-specific transcript parsing.
- Mid-run `config.toml` MCP edits (depends on Codex CLI restart).
- `agent_teams` (no Codex concept).
- Hosted A2A subagents (no Codex concept; Gemini-only consideration).

Layered enforcement to call out in the profile's strategy notes:

- Codex enforces `network=forbidden` and `repo_scope=local_only` at the
  OS level (Landlock+seccomp on Linux, Seatbelt on macOS). The Striatum
  lane currently claims `advisory_strict` for the process adapter; in
  practice the Codex layer is `enforced`. Reviewers should not treat the
  two as equivalent; the profile's strategy notes should call out where
  the lane runs with stronger-than-advertised enforcement.

## Risks, missing docs, and unknowns

- **openai/codex#11435** is open and unmitigated upstream. Per-job
  `CODEX_HOME` is the only safe parallel-exec story today. If Codex
  ships a fix that lock-files or per-PID isolates sessions, the profile
  can drop the requirement.
- **openai/codex#10390** notes that `network_access = true` is silently
  ignored on macOS in some configurations. The Codex layer's "enforced"
  story is mostly Linux. Striatum should keep `advisory_strict` as the
  declared adapter level; the profile notes the layered enforcement is
  Linux-strong, macOS-best-effort.
- **openai/codex#3441** — MCP edits in `config.toml` ignored until CLI
  restart. Document as a known papercut in the profile.
- **No documented hard `max_iterations`**: third-party orchestrators
  enforce their own. The profile field is advisory.
- **Codex SDK** (`@openai/codex-sdk`, Python `AsyncCodex`) is experimental
  and out of scope for V1.

## Native subagents stay internal to the parent session

Yes — Codex spawns custom agents only when the parent natural-language
decides to. There is no `--use-agent reviewer` flag in `codex exec`.
Per RFC 0010 D021 accountability rules, the parent Striatum session
remains responsible for the final artifact and Striatum CLI commands;
custom subagents do not register independently with the runner. The
profile's `accountability.native_subagents = internal_to_parent_session`
reflects this and should remain V1 default.

## Harness friction encountered during this research job

Captured here for visibility; will be promoted to
`harness_improvement_proposal` artifacts at the run level once the lease
allows publishing in scope.

1. **Lane execution mode mismatch.** This research job was claimed by a
   `researcher-codex` session, but no actual `codex exec` process was
   spawned: the parent operator drove the work directly. The runner does
   not currently distinguish "lane is configured" from "lane is actually
   the process producing the artifact." The work packet's `author:` line
   correctly records `researcher-codex-gpt-5.5-001`, but the byline
   becomes a label of intent rather than provenance when the operator
   substitutes for the lane. Proposal: add a packet-level field
   `actual_runner` (or similar) that lets dogfood operators record when
   they ran a lane interactively rather than via the declared adapter,
   so reviewers can audit byline drift against runner identity.

2. **Missing Claude supervised wrapper script.** RFC 0010's
   `claude_code_default` profile and the dogfood-003 workflow both
   reference `.striatum/bin/claude-supervised-wrapper.sh`. The script
   does not exist in this checkout. The dogfood-003 RUNBOOK
   acknowledges this and tells operators to "run that lane manually or
   capture the missing wrapper as dogfood evidence." That guidance is
   correct, but the friction is that the workflow validates green even
   though the lane command file is absent. Proposal: a workflow-validate
   lint check that warns when a lane command's first arg looks like a
   path inside the repo and the file is missing.

3. **SKILL.md path drift.** `docs/dogfood/003/SKILL.md` instructs the
   operator to `cd /Users/halbritt/git/striatum`, which is a macOS
   absolute path. The repo is at `/home/halbritt/git/striatum` on the
   running host. Per AGENTS.md "Avoid hardcoded home-directory absolute
   paths in tracked docs and fixtures," this should be `cd ~/git/striatum`
   or repo-relative.

## Open questions for the designer

- Should `codex_default` strictly require `--ephemeral` and per-job
  `CODEX_HOME`, or accept either as workspace-isolation enforcement?
  Recommendation: require both and surface a workflow-validate error if
  a lane references `codex_default` but the lane `env` does not set
  `CODEX_HOME`.
- Should the profile carry `approvals_reviewer = "auto_review"` guidance
  to point operators at Codex's hook-based auto-approval as a future
  upgrade path? Out of scope for V1 build slice.
- Should the profile fixture in V1 ship under
  `examples/harness-profiles/codex_default.json` (separate file) or stay
  inline in workflow JSON like the dogfood-003 fixture? Recommend inline
  for V1 to keep validation simple; add a reference-by-path mode later.
