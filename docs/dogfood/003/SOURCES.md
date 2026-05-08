# Dogfood 003 Source List

Status: draft
Date: 2026-05-08

Use this file as the starting source list for tool-harness research. Research
jobs should verify that each source is still current before relying on it,
prefer official documentation over blogs, social posts, or memory, and treat
the existing `docs/research/0010-tool-harness-profiles/` notes as prior art
to confirm or update rather than as settled truth.

## RFC And Existing Research

- `docs/rfcs/0010-tool-harness-profiles.md`
- `docs/research/0010-tool-harness-profiles/claude_code.md`
- `docs/research/0010-tool-harness-profiles/codex.md`
- `docs/research/0010-tool-harness-profiles/gemini_cli.md`

## Claude Code

- [Claude Code sub-agents](https://code.claude.com/docs/en/sub-agents)
- [Claude Code agent teams](https://code.claude.com/docs/en/agent-teams)
- Claude Code docs for hooks, memory, skills, MCP, command permissions, and
  non-interactive or print-mode operation, when relevant to the profile.

Research focus:

- custom subagent definition format and storage locations;
- foreground versus background subagent behavior;
- tool allowlists, denied tools, hooks, memory, and skill preloading;
- whether agent teams are available, stable enough to use, and how they are
  enabled;
- whether `claude -p` remains incompatible with RFC 0009 long-lived
  supervision, and what a newline-delimited JSON wrapper must provide;
- how Striatum should phrase generic delegation instructions without assuming
  that every Claude Code install supports the same features.

## Codex

- [Codex non-interactive mode](https://developers.openai.com/codex/noninteractive)
- [Codex CLI reference](https://developers.openai.com/codex/cli/reference)
- [Codex MCP](https://developers.openai.com/codex/mcp)
- [Codex product overview](https://openai.com/codex/)
- [Codex agent loop](https://openai.com/index/unrolling-the-codex-agent-loop/)
- [openai/codex#11435](https://github.com/openai/codex/issues/11435)

Research focus:

- `codex exec` behavior for scripted and CI-style use;
- `--json`, `--ephemeral`, `--skip-git-repo-check`, `--ignore-user-config`,
  sandbox, approval, and stdin `-` behavior;
- `CODEX_HOME` isolation for parallel jobs and whether issue #11435 still
  applies;
- MCP server configuration and project-scoped configuration;
- skills, `AGENTS.md`, custom agent roles, worktree/workspace behavior, and
  agent-loop budget controls;
- how to express native delegation in a way that keeps the parent Striatum
  session accountable for artifacts and completion.

## Gemini CLI

- [Gemini CLI docs](https://google-gemini.github.io/gemini-cli/docs/)
- [Gemini CLI headless mode](https://google-gemini.github.io/gemini-cli/docs/cli/headless.html)
- [Gemini CLI MCP servers](https://google-gemini.github.io/gemini-cli/docs/tools/mcp-server.html)
- [Gemini CLI subagents](https://geminicli.com/docs/core/subagents/)

Research focus:

- headless invocation through prompts and stdin;
- structured output or JSON output suitability for Striatum adapters;
- MCP server setup and constraints;
- local subagent definitions and `@<agent-name>` invocation;
- remote A2A subagents and why they remain forbidden without an explicit
  hosted-services decision;
- `--worktree` and why RFC 0008 should remain authoritative for Striatum
  worktree isolation;
- whether named-pipe stdin behavior is safe enough for supervised lanes.
