# RFC 0010: Tool Harness Profiles

Status: proposed
Date: 2026-05-07
Context:
`docs/DECISION_LOG.md` (D003, D015, D021, D022, D037, D054),
`docs/rfcs/0005-harness-meta-optimization.md`,
`docs/rfcs/0008-worktree-isolation-for-parallel-jobs.md`,
`docs/rfcs/0009-long-lived-process-supervision.md`,
[Claude Code sub-agents](https://code.claude.com/docs/en/sub-agents),
[Claude Code agent teams](https://code.claude.com/docs/en/agent-teams),
[Codex](https://openai.com/codex/),
[Codex agent loop](https://openai.com/index/unrolling-the-codex-agent-loop/)

## Problem

Striatum's current adapter model intentionally starts from the minimum
portable process contract: command, cwd, environment, stdin, stdout, stderr,
exit code, and optional PTY. That boundary keeps the runner generic and
protects SQLite as the authoritative workflow state.

The leading terminal-agent tools are moving faster than the common process
contract. Claude Code exposes custom sub-agents and experimental agent teams.
Codex workflows can use custom agent roles, skills, and multiple isolated
agent workspaces. Gemini CLI exposes its own headless and MCP-oriented
surfaces. Users are already learning that prompts such as "spawn the maximum
number of useful agents to accomplish this task" can produce better outcomes
when the underlying tool knows how to delegate.

Without a Striatum-level concept for tool-specific harness behavior, this
knowledge lands in scattered prompts, local habits, or unreviewed operator
memory. That creates three problems:

- Workflows cannot say which native tool features are desirable for a lane.
- Agents may over-delegate or under-delegate because the work packet carries
  only the generic job contract.
- Harness improvements discovered during dogfood runs are hard to route back
  into a durable, reusable profile for the next run.

The runner needs a way to optimize interactions for each tool while
preserving the core boundary: provider-specific features are allowed at the
edge, but workflow authority remains in Striatum.

## Goals

- Define an optional tool harness profile layer distinct from the adapter
  layer.
- Let workflows describe how a lane should use native tool capabilities such
  as sub-agents, agent teams, skills, custom roles, hooks, MCP tools, and
  headless execution.
- Keep native sub-agents internal to the parent Striatum session by default,
  preserving D021 accountability.
- Surface harness guidance in work packets in a structured, auditable form.
- Let dogfood runs emit `harness_improvement_proposal` artifacts that can
  improve tool profiles over time.
- Preserve model portability: workflows should still run with a generic
  profile when a provider-specific feature is unavailable.

## Non-Goals

- Do not make Striatum parse provider transcripts, terminal output, or
  native sub-agent logs as authoritative state.
- Do not let hidden native sub-agent trees independently own Striatum queue
  messages, leases, artifacts, or verdicts unless they are explicitly
  registered as first-class sessions under a future decision.
- Do not require hosted services, cloud coordination, telemetry, or external
  persistence.
- Do not auto-apply harness changes. RFC 0005 remains the gate: proposals
  are advisory until reviewed and accepted.
- Do not hardcode one provider as the product identity. A lane remains
  configuration, not a provider assumption.

## Proposal

Add an optional `harness_profiles` section to workflow configuration and a
per-lane reference to one profile:

```json
{
  "harness_profiles": {
    "codex_default": {
      "tool_family": "codex",
      "strategy_version": "2026-05-07",
      "native_delegation": {
        "mode": "encouraged",
        "instruction": "Spawn the maximum number of useful agents only when their work is independent and bounded by the packet write scope.",
        "max_parallel_native_agents": "tool_default"
      },
      "feature_flags": {
        "skills": "allowed",
        "custom_agent_roles": "allowed",
        "worktree_agents": "allowed"
      },
      "accountability": {
        "native_subagents": "internal_to_parent_session",
        "first_class_registration": "not_supported"
      }
    }
  },
  "lanes": {
    "codex": {
      "adapter": "process",
      "harness_profile_id": "codex_default",
      "command": ["codex", "exec", "-"]
    }
  }
}
```

### Profile Fields

V1 profile fields should be intentionally small:

- `tool_family`: one of `generic`, `codex`, `claude_code`, `gemini_cli`, or
  another validator-accepted string once a profile schema exists.
- `strategy_version`: a human-readable version or date for the profile
  guidance.
- `native_delegation`: describes whether native delegation is `off`,
  `allowed`, `encouraged`, or `required_for_lane`, plus a short instruction.
- `feature_flags`: advisory declarations for native capabilities the lane may
  use, such as `subagents`, `agent_teams`, `skills`, `custom_agent_roles`,
  `hooks`, `mcp`, `headless`, or `worktree_agents`.
- `accountability`: records that native sub-agents are
  `internal_to_parent_session` unless future work supports first-class
  registration.
- `prompt_envelope_path` (optional): a reusable Markdown prompt wrapper that
  is appended to or referenced by the work packet for that tool family.
- `fallback_profile_id` (optional): a profile to use when the native feature
  set is unavailable.

### Work Packet Exposure

When a lane references a harness profile, `claim-next` should include a
`harness_profile` block in the work packet:

```json
{
  "harness_profile": {
    "profile_id": "codex_default",
    "tool_family": "codex",
    "strategy_version": "2026-05-07",
    "native_delegation": {
      "mode": "encouraged",
      "instruction": "Spawn the maximum number of useful agents only when their work is independent and bounded by the packet write scope."
    },
    "accountability": {
      "native_subagents": "internal_to_parent_session"
    }
  }
}
```

The block is guidance, not authority. The authoritative job contract remains
the existing work packet fields: write scope, expected artifacts, lease,
commands, review policy, adapter constraints, and worktree requirements.

### Delegation Semantics

The phrase "maximum number of useful agents" should not mean maximum process
count. It should mean the maximum number of independently useful native
delegations that satisfy all of these conditions:

- The delegated task is concrete, bounded, and materially advances the parent
  job.
- The parent session can integrate or reject the result without violating the
  work packet's write scope.
- Parallel native agents do not write overlapping files unless the parent
  tool provides isolated workspaces and the parent session remains responsible
  for final integration.
- Review-only, research, fixture inspection, and independent verification are
  preferred native delegation targets before broad repo-write work.
- The parent session still publishes the required artifacts and completes or
  blocks the Striatum job.

### Relationship To Existing Decisions

This RFC keeps D021 intact. Native sub-agents remain internal to the parent
session in V1. Tool profiles simply make the parent's delegation strategy
explicit.

This RFC also keeps D015 intact. Striatum's scheduler still requires declared
parallelism for first-class jobs. Native delegation inside a parent session is
tool-local optimization, not a new source of claimable Striatum work.

### Dogfood Path

Dogfood-001 should be used to collect the first practical profile changes:

1. Run the existing Striatum-on-Striatum workflow with real Claude Code and
   Codex lanes.
2. Record friction as `harness_improvement_proposal` artifacts.
3. Convert high-signal proposals into initial `codex`, `claude_code`, and
   `generic` profile fixtures.
4. Add profile exposure to work packets only after the initial profile shape
   survives review.

## Acceptance Criteria

- Workflow validation accepts a `harness_profiles` map and validates that any
  lane `harness_profile_id` references a declared profile.
- Unknown or malformed profile fields produce workflow validation errors, or
  profile lint warnings if the project chooses an advisory-first rollout.
- `claim-next` includes a `harness_profile` block in work packets for lanes
  that reference a profile.
- The default behavior for workflows without `harness_profiles` is unchanged.
- A generic profile fixture, a Codex-oriented profile fixture, and a Claude
  Code-oriented profile fixture are added under examples or dogfood docs.
- Documentation states that native sub-agents remain internal to the parent
  Striatum session unless registered as first-class sessions in a future
  decision.
- At least one dogfood run produces or reviews a
  `harness_improvement_proposal` that targets one of `prompt`, `workflow`,
  `defaults`, or `documentation` for a tool profile.

## Open Questions

- Should profile validation be strict from day one, or should unknown fields
  be accepted as lint warnings while provider capabilities are changing fast?
- Should `harness_profiles` live directly in workflow JSON, in reusable
  profile files referenced by path, or both?
- Should Striatum ship built-in profile templates for `codex`,
  `claude_code`, and `gemini_cli`, or keep all profiles user-authored until
  dogfood evidence accumulates?
- Should native delegation limits be numeric (`max_native_agents: 4`) or
  semantic (`tool_default`, `bounded_by_write_scope`, `review_only`)?
- What evidence should a parent session publish when it uses native
  sub-agents, given the no-transcript default?
- When, if ever, should native sub-agents graduate into first-class Striatum
  sessions with independent leases and artifacts?
