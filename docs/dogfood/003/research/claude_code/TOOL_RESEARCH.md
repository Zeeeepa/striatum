---
schema_version: striatum.handoff.v1
artifact_kind: handoff
tool_family: claude_code
strategy_version: "2026-05-08"
tags: ["dogfood-003", "rfc-0010", "claude_code"]
---

# Claude Code Harness Profile — Refresh For Dogfood-003

author: researcher-claude-opus-001

Date: 2026-05-08
Verified-against: `docs/research/0010-tool-harness-profiles/claude_code.md`
(2026-05-08), `docs/rfcs/0010-tool-harness-profiles.md`,
`docs/dogfood/003/SOURCES.md`.

This is a verify-and-refresh report. The prior dogfood-001 v2 research note
for Claude Code is dated 2026-05-08 (today). The work here confirms the
prior note against the RFC 0010 profile and points to two refinements that
matter for the V1 build slice.

## Source list

Primary upstream:

- Sub-agents: <https://code.claude.com/docs/en/sub-agents>
- Agent teams (experimental): <https://code.claude.com/docs/en/agent-teams>
- Skills: <https://code.claude.com/docs/en/skills>
- Hooks: <https://code.claude.com/docs/en/hooks>
- MCP: <https://code.claude.com/docs/en/mcp>
- CLI reference: <https://code.claude.com/docs/en/cli-reference>
- Interactive mode: <https://code.claude.com/docs/en/interactive-mode>
- Memory files: <https://code.claude.com/docs/en/memory>

Striatum-side:

- `docs/SPEC.md` (Supervised Lane Command Contract).
- `src/striatum/mcp.py` reference implementation.

## RFC 0010 `claude_code_default` — confirmation

| Field | RFC 0010 value | Confirmed? |
|---|---|---|
| `tool_family` | `claude_code` | yes |
| `strategy_version` | `2026-05-08` | yes |
| `native_delegation.mode` | `preferred` | yes |
| `native_delegation.instruction` | sub-agents-first; agent teams experimental | yes |
| `feature_flags.subagents` | `preferred` | yes |
| `feature_flags.agent_teams` | `allowed_experimental` | yes |
| `feature_flags.skills` | `encouraged` | yes |
| `feature_flags.custom_agent_roles` | `allowed` | yes |
| `feature_flags.hooks` | `encouraged` | yes |
| `feature_flags.mcp` | `allowed` | yes |
| `feature_flags.headless_print_mode` | `forbidden_for_supervised_lanes` | yes |
| `supervision.compatible` | `true` | yes |
| `supervision.stdin_format` | `newline_delimited_json` | yes |
| `supervision.wrapper_required` | `true` | yes |
| `approval_mode` | `auto_edit` | yes |
| `memory_files` | `["CLAUDE.md", "AGENTS.md"]` | yes |
| `accountability.native_subagents` | `internal_to_parent_session` | yes |

No drift relative to the prior note.

## Coverage of RFC 0010 extended fields

### `supervision`

This is the field that most differentiates Claude Code from Codex.

- `compatible: true` — only via a wrapper. Claude Code's `-p` print mode
  reads one prompt, emits a response, and exits. That is single-shot
  semantics, not RFC 0009 long-lived supervision. The wrapper script
  (`.striatum/bin/claude-supervised-wrapper.sh`) must:
  1. Spawn a single long-lived `claude` session (interactive but with
     stdin/stdout piped — Claude Code accepts piped stdin without TTY).
  2. Read newline-delimited JSON packets from its own stdin.
  3. Feed each packet as a fresh user turn into the long-lived session.
  4. Forward the session's structured output (or just call back via
     `striatum` CLI commands inside the session) to advance workflow
     state.
  5. Never rely on parsing the supervisor's stdout for authoritative
     workflow state — Striatum's CLI commands remain authoritative.
- `stdin_format: newline_delimited_json` — confirms the contract.
- `wrapper_required: true` — confirmed. The wrapper is missing in this
  checkout (see "Harness friction" below), which is itself the most
  material V1 finding.

A second supervised mode worth noting (out of scope for V1 build slice):
Claude Code can be exposed via MCP — Striatum could be the MCP server,
and the long-lived `claude` session would call back via MCP tools rather
than via shell-out. That removes the need for a stdin-feeding wrapper but
requires Striatum to maintain an MCP surface; the current
`src/striatum/mcp.py` is an entry point but not yet a fully-supervised
lane runner. Profile field `mcp` therefore stays `allowed` rather than
`encouraged_for_supervision` in V1.

### `workspace_isolation`

Claude Code does not have an equivalent of Codex's `CODEX_HOME` per-job
state directory hazard. Multiple `claude` sessions can run in parallel
without mutually corrupting state — each session is independent, with no
shared SQLite or rollout. RFC 0008 still owns repo-level worktree
isolation when a job needs filesystem isolation; Claude Code is happy to
operate in whatever cwd it is given.

The `claude_code_default` profile in RFC 0010 omits `workspace_isolation`,
which is correct. The schema should accept the field but not require it.

### `agent_loop_budget`

Claude Code does not surface `auto_compact_limit`, `model_reasoning_effort`,
or `max_iterations` as user-facing knobs in the same way Codex does. The
closest analogues are skill-level `effort` and `model` overrides. The
profile should not include `agent_loop_budget` for `claude_code_default`;
the schema should accept the field as optional and tools that don't expose
it can simply omit it.

### `approval_mode`

`auto_edit` is the right default for a Striatum-driven lane. Claude Code's
permission system is settings-file-driven (`.claude/settings.json` and
`~/.claude/settings.json`), with `permissions.allow`/`permissions.deny`
lists and per-tool rules. Mapping the four-value enum:

- `default` → equivalent to using user/project settings as-is (operator
  default).
- `auto_edit` → permissions configured to allow Edit/Write/Bash within
  the work-packet write scope without prompting; deny outside the scope.
- `yolo` → `--dangerously-skip-permissions` (or `permissions.allow:["*"]`).
- `plan` → no Claude Code equivalent; document as not supported (or
  leverage Plan-mode keybindings, which is interactive-only).

### `output_format`

Claude Code supports `--json` for structured output, and the long-lived
session emits prompt-by-prompt assistant turns. For supervised lanes, the
wrapper should pass `--output-format stream-json` (Claude Code's streaming
JSON mode) or capture per-turn JSON. The profile field should be `text`
or `stream-json`, settled by the wrapper. RFC 0010 does not pin this for
`claude_code_default`; recommendation: leave unset for V1, let the
wrapper choose.

### `memory_files`

`CLAUDE.md` (Claude Code's first-class memory file) plus `AGENTS.md`
(Striatum's contributor instructions). Both are read at session start.
Confirmed.

### `mcp_servers`

Empty for `claude_code_default`. Correct for V1. Operators add MCP
servers via `.claude/mcp.json` (project) or `~/.claude/mcp.json` (user).
The profile-level field is for declarative validation (e.g., warn if a
workflow says it depends on MCP server `striatum-supervisor` but the
project doesn't ship `.claude/mcp.json` mentioning it).

### `turn_caps`

Not applicable to Claude Code in the same way as Gemini CLI. Claude Code
has no documented `subagent_max_turns` or `subagent_timeout_mins`. Skill
or sub-agent definitions can encode their own stopping conditions in
prompt text. Profile should leave `turn_caps` unset; schema should treat
it as optional.

## Recommended lane command, environment, and wrapper

V1 lane command (matches dogfood-003 workflow.json):

```bash
.striatum/bin/claude-supervised-wrapper.sh
```

Environment:

- No additional vars required for V1.
- `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS` should remain unset for
  supervised lanes (agent teams are experimental and add `~/.claude/teams/`
  shared state that violates the per-session isolation model).

Wrapper script (proposed shape, V1):

```bash
#!/usr/bin/env bash
# .striatum/bin/claude-supervised-wrapper.sh — proposed
# Reads newline-delimited JSON packets from stdin and feeds each as a
# user turn into a single long-lived `claude` session. Stdout/stderr are
# DEVNULL'd by the supervisor; workflow state advances only via the
# `striatum` CLI commands the agent runs inside the session.
set -euo pipefail
exec claude --print --output-format stream-json --input-format stream-json
```

Open implementation question for the designer: whether `claude --print`
with `--input-format stream-json` provides the long-lived loop semantics
we need, or whether we must wrap a TTY-backed `claude` session with
`expect` / a coproc. Recommendation for V1 build slice: defer wrapper
implementation to a follow-up RFC 0009 + RFC 0010 task; ship the profile
schema and the lane reference, capture the missing wrapper as a
high-severity harness improvement proposal.

## What stays generic, advisory, forbidden, or unsupported

Generic and advisory (kept in V1 profile):

- Native delegation: `preferred` with sub-agents-first instruction.
- `feature_flags`: subagents, skills, hooks, mcp.
- `headless_print_mode: forbidden_for_supervised_lanes` — this is a
  hard constraint for the lane validator.
- `memory_files: [CLAUDE.md, AGENTS.md]`.

Forbidden / not-supported in V1:

- Direct `claude -p` invocation as a supervised lane command.
- Agent teams as Striatum-coordinated workers (they ship `~/.claude/teams/`
  shared state that we don't want as authoritative; keep them as
  parent-internal optimization only).
- First-class registration of native sub-agents as Striatum sessions.
- HTTP and Prompt hook types as Striatum integration points in V1
  (Command and MCP hooks are sufficient and simpler to audit).

## Risks, missing docs, and unknowns

- **Wrapper script does not exist.** RFC 0010 references
  `.striatum/bin/claude-supervised-wrapper.sh` and the dogfood-003
  workflow uses it as the lane command. The script is missing in this
  checkout. Workflow validation accepts the lane reference because the
  validator does not check that the lane command file exists. The
  practical effect today is that anyone trying to run the
  `claude_code` lane via `striatum supervise start` will fail at exec
  time. For V1 build slice, the cleanest fix is either ship the
  wrapper script (pinned to whichever `claude` invocation supports
  long-lived stdin) or add a workflow-validate lint that warns on
  missing repo-relative lane command files. Recommendation: do the
  lint check now; defer the wrapper itself to a follow-up.
- **Print-mode JSON streaming details.** Claude Code's documented
  `--input-format stream-json` is recent and may have edge cases
  around partial-line buffering that affect a long-lived stdin loop.
  Verify behavior under named-pipe stdin (`os.mkfifo`) before pinning
  the wrapper.
- **Agent teams have shared global state.** `~/.claude/teams/` and
  `~/.claude/tasks/` are runtime files outside the project. RFC 0010's
  D021 accountability rule says native sub-agents stay internal to the
  parent session — agent teams violate this in spirit because they
  persist across sessions. Keep `agent_teams: allowed_experimental` so
  workflows can opt in, but document that they break parent-session
  containment.
- **Hooks and skills can shadow Striatum commands.** A
  user-installed skill named `/claim-next` that is not Striatum's would
  be invoked instead of the Striatum CLI command. The wrapper should
  refuse to run if the project's `.claude/skills/` contains entries
  that collide with reserved Striatum command names. Out of scope for
  V1 build slice but worth noting.

## Native subagents stay internal to the parent session

Yes. The Striatum runner does not see sub-agents; only the long-lived
`claude` session is registered with the supervisor. Sub-agents, skills,
agent teams, and hooks are all parent-internal mechanisms. The parent
session publishes the work-packet artifacts and runs the Striatum CLI.

`accountability.native_subagents = internal_to_parent_session` is the
right V1 default and should remain enforced even if a future RFC adds
first-class registration as a separate decision.

## Harness friction encountered during this research job

1. **Missing supervised wrapper.** Already covered above; promoted as
   the highest-severity harness friction for this run.

2. **Lane execution mode mismatch.** Same pattern as the Codex job: the
   `researcher-claude` session was claimed by the parent operator, not
   by an actual `claude` process invoked via the supervised lane. The
   `author:` line correctly records `researcher-claude-opus-001` but
   functions as label-of-intent rather than provenance. See the codex
   research handoff for the proposed `actual_runner` packet field.

3. **Skill discoverability for Striatum CLI commands.** Striatum has
   no `.claude/skills/` library shipped today. A V1 nice-to-have is a
   `striatum/init-claude-skills` command that drops minimal skill
   definitions for `claim-next`, `publish-artifact`, `complete`, etc.,
   so Claude Code agents discover them via slash commands. Out of
   scope for the RFC 0010 build slice but worth a follow-up.

## Open questions for the designer

- Should the `harness_profile` block in work packets surface the
  `forbidden_for_supervised_lanes` rule as a hard validation on the
  lane command (i.e., refuse a lane whose command starts with
  `claude` and includes `--print`/`-p` while the lane references
  `claude_code_default`)? Recommendation: yes, but as a lint warning
  in V1 and an error in V2.
- Should the schema accept `output_format` at lane level overriding
  profile level, or only at profile level? Recommendation: profile-
  level only in V1; lane-level overrides come if and when operators
  request them.
- Should the V1 build slice exclude `agent_teams` entirely from
  `feature_flags` and add it later? Recommendation: keep as
  `allowed_experimental` so workflows can opt in, but document the
  shared-state caveat in the profile's strategy notes.
