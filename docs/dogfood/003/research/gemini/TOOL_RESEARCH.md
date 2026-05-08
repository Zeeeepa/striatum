---
schema_version: striatum.handoff.v1
artifact_kind: handoff
tool_family: gemini_cli
strategy_version: "2026-05-08"
tags: ["dogfood-003", "rfc-0010", "gemini_cli"]
---

# Gemini CLI Harness Profile — Refresh For Dogfood-003

author: researcher-gemini-pro-001

Date: 2026-05-08
Verified-against: `docs/research/0010-tool-harness-profiles/gemini_cli.md`
(2026-05-08), `docs/rfcs/0010-tool-harness-profiles.md`,
`docs/dogfood/003/SOURCES.md`.

This is a verify-and-refresh report. The prior dogfood-001 v2 research note
for Gemini CLI is dated 2026-05-08 (today). The Gemini profile is the
profile with the most extension-field activity (subagents, A2A,
`--worktree`, headless modes), so the verification focus is whether
RFC 0010's V1 boundary cleanly excludes the riskier features.

## Source list

Primary upstream:

- Gemini CLI docs root: <https://google-gemini.github.io/gemini-cli/docs/>
- Headless mode: <https://google-gemini.github.io/gemini-cli/docs/cli/headless.html>
- MCP servers: <https://google-gemini.github.io/gemini-cli/docs/tools/mcp-server.html>
- Subagents: <https://geminicli.com/docs/core/subagents/>
- Google announcement (subagents): announcement post linked from the
  Subagents docs.
- CLI reference / hooks / configuration / sandboxing pages under the same
  docs root.

Striatum-side:

- `docs/SPEC.md` worktree contract.
- `src/striatum/mcp.py` reference implementation.

## RFC 0010 `gemini_cli_default` — confirmation

| Field | RFC 0010 value | Confirmed? |
|---|---|---|
| `tool_family` | `gemini_cli` | yes |
| `strategy_version` | `2026-05-08` | yes |
| `native_delegation.mode` | `allowed` | yes |
| `native_delegation.instruction` | `@<agent-name>` syntax; parallel for read/research/test only | yes |
| `feature_flags.subagents` | `allowed` | yes |
| `feature_flags.remote_subagents_a2a` | `forbidden` | yes |
| `feature_flags.agent_teams` | `not_supported` | yes |
| `feature_flags.skills` | `allowed` | yes |
| `feature_flags.custom_agent_roles` | `allowed` | yes |
| `feature_flags.hooks` | `allowed` | yes |
| `feature_flags.mcp` | `encouraged` | yes |
| `feature_flags.native_worktree` | `forbidden` | yes |
| `supervision.compatible` | `verify_pipe_behavior_first` | yes — see notes |
| `supervision.stdin_format` | `prompt_text` | yes |
| `supervision.wrapper_required` | `false` | conditional — see notes |
| `approval_mode` | `auto_edit` | yes |
| `output_format` | `stream-json` | yes |
| `turn_caps.subagent_max_turns` | `50` | yes |
| `turn_caps.subagent_timeout_mins` | `30` | yes |
| `memory_files` | `["GEMINI.md"]` | yes |
| `accountability.native_subagents` | `internal_to_parent_session` | yes |

No drift relative to the prior note.

## Coverage of RFC 0010 extended fields

### `supervision`

This is the field where Gemini CLI deserves the most operator caution.

- `compatible: "verify_pipe_behavior_first"` — confirmed. Gemini CLI's
  headless mode reads stdin until EOF and then exits. Long-lived
  supervision under RFC 0009 sends multiple newline-delimited packets
  on a stdin that stays open across packets. Whether `gemini --prompt -`
  honors per-line packet semantics, or treats the entire stdin as one
  prompt, depends on whether the CLI reads with line-buffering or
  block-buffering under a named pipe. The prior note flags this as the
  least-tested path of the three CLIs. Recommendation: keep
  `compatible: "verify_pipe_behavior_first"` as a marker that workflow
  validation should *warn* but not error, and require the wrapper or
  operator to verify behavior before running supervised lanes.

- `stdin_format: "prompt_text"` — `gemini -p` reads the prompt text
  from stdin (when prompt is omitted) or treats stdin as additional
  context (when prompt arg is given). Use `-p -` for stdin-as-prompt
  semantics. The dogfood-003 lane command uses `--prompt -`.

- `wrapper_required: false` — true only if pipe behavior under named
  pipes turns out to be safe. Otherwise a wrapper that re-execs
  `gemini -p` per packet is required, which is operationally similar
  to Codex's `codex exec` per-packet model. Recommend the schema accept
  `false` and let workflow validation surface the
  `verify_pipe_behavior_first` flag as a lane-level lint warning.

### `workspace_isolation`

Gemini CLI ships its own `--worktree` / `-w` feature, gated behind
`experimental.worktrees: true` in `settings.json`. **The profile
should keep `feature_flags.native_worktree: forbidden`** because
Striatum's RFC 0008 already owns workflow-level worktree isolation. If
both Gemini's worktree and Striatum's worktree fire, the Gemini
worktree shadows Striatum's, breaking the runner's tracking and
worktree-release commands.

The profile does not include a `workspace_isolation` block today
because Gemini CLI lacks an analogue to Codex's `CODEX_HOME` per-job
state hazard. Recommendation: leave `workspace_isolation` unset for
`gemini_cli_default`; schema should accept it as optional.

### `agent_loop_budget`

Gemini CLI doesn't expose `auto_compact_limit` or `model_reasoning_effort`
as user-facing flags. The relevant guardrails are the per-subagent
fields documented in subagent frontmatter:

- `max_turns` (default 30 per subagent).
- `timeout_mins` (default 10 per subagent).

These are encoded in RFC 0010 as `turn_caps.subagent_max_turns` and
`turn_caps.subagent_timeout_mins` rather than `agent_loop_budget`, which
is the right call. The profile should not include `agent_loop_budget`.

### `approval_mode`

`auto_edit` is the canonical Gemini CLI mode for non-interactive use.
The other documented modes are `default` (interactive prompts) and
`yolo` (skip confirmations). Gemini CLI exposes this as the
`--approval-mode` CLI flag — first-class, unlike Claude Code's
settings-file-driven permission system. Confirmed.

### `output_format`

`stream-json` is correct for supervised use. The shape:

- `init` event at session start.
- `message` events as the agent emits text.
- `tool_use` and `tool_result` for tool calls.
- `error` events.
- `result` event at session completion.

This is parsable by a supervisor that wants to track lifecycle. The
alternative `json` envelope (`{response, stats, error?}`) is fine for
single-shot use but provides less mid-run visibility.

### `memory_files`

`GEMINI.md` is the canonical file Gemini CLI reads on session start
(project root, then `~/.gemini/GEMINI.md`). The profile correctly lists
it. Some operators also seed `AGENTS.md`; Gemini CLI does not read
`AGENTS.md` natively, so it is not in the profile's list.

### `mcp_servers`

Empty for the V1 default profile. Gemini CLI is the most MCP-positive of
the three (the docs explicitly call it MCP-first), so workflows that
want to expose Striatum-as-MCP should add `striatum-supervisor` to the
profile's `mcp_servers` list and the project's `.gemini/settings.json:
mcpServers` block. V1 build slice should not auto-configure this; the
profile field is for declarative validation only.

### `turn_caps`

Confirmed as the place to encode subagent guardrails:

- `max_session_turns: null` — Gemini CLI does not expose a global
  session turn cap; `null` is the right schema value.
- `subagent_max_turns: 50` — overrides the documented default of 30.
  RFC 0010 picks 50 to give research subagents more headroom; the
  default profile fixture for V1 should accept either, but 50 is the
  recommended starting point given research jobs in dogfood-003.
- `subagent_timeout_mins: 30` — overrides default 10. Same rationale.

## Recommended lane command, environment, and wrapper

V1 lane command (matches dogfood-003 workflow.json):

```bash
gemini --prompt - --output-format stream-json --approval-mode auto_edit
```

Environment:

- `GEMINI_CLI_TRUST_WORKSPACE=1` — pre-approves the workspace as a trust
  context so Gemini doesn't pause at first run waiting for user trust
  confirmation. Documented in the trust/sandbox section. The dogfood-003
  workflow already sets this.
- No additional vars required for V1.

Wrapper requirement: conditional. If pipe-behavior verification (under
named-pipe stdin) succeeds, no wrapper is required and the lane command
above is the supervised entry point. If verification fails, a wrapper
that re-execs `gemini --prompt -` per packet is the fallback.

Recommendation: ship the V1 build slice without a Gemini wrapper, but
add a workflow-validate lint that surfaces `verify_pipe_behavior_first`
as a warning when a Gemini lane is used with `striatum supervise`.

## What stays generic, advisory, forbidden, or unsupported

Generic and advisory (kept in V1 profile):

- `feature_flags.subagents: allowed` (Gemini's strongest delegation
  surface).
- `feature_flags.mcp: encouraged`.
- `turn_caps` numeric guardrails.
- `output_format: stream-json`, `approval_mode: auto_edit`.
- `memory_files: [GEMINI.md]`.

Forbidden / not-supported in V1:

- `feature_flags.remote_subagents_a2a: forbidden` — A2A remote subagents
  require a hosted endpoint and clash with Striatum's no-hosted-services
  product boundary.
- `feature_flags.native_worktree: forbidden` — Gemini's `--worktree`
  shadows RFC 0008 worktree isolation.
- `feature_flags.agent_teams: not_supported` — Gemini has no concept.
- First-class registration of native subagents as Striatum sessions.
- Recursion of native subagents (Gemini already disallows this).

## Risks, missing docs, and unknowns

- **Pipe behavior under named pipes is the largest unknown.** Striatum's
  RFC 0009 supervisor uses `os.mkfifo` (or equivalent) for the supervised
  process's stdin. Whether Gemini CLI handles a named-pipe stdin the
  same way it handles a regular pipe is not documented. The prior note
  flags this; verification should happen as part of the wrapper-author
  task, not the V1 build slice.
- **JSON output regression history.** Issue #9009 documented earlier
  builds where `--output-format json` returned plain text. Current 2026
  builds work, but operators should pin the CLI version. The profile's
  `strategy_version` provides the audit trail.
- **`--worktree` is gated by `experimental.worktrees: true`.** A
  user-level `~/.gemini/settings.json` with that flag enabled would
  re-enable the feature even if the project profile says forbidden.
  Workflow validation cannot detect this; the wrapper or lane command
  should explicitly avoid passing `--worktree`. The dogfood-003 lane
  command does not pass `--worktree`, which is correct.
- **A2A subagents and OAuth flows.** A user-level Gemini config with
  remote A2A subagents (`kind: remote`) would import them into any
  workflow that runs Gemini, regardless of the profile's
  `remote_subagents_a2a: forbidden` setting. The profile-level rule is
  advisory unless workflow validation refuses to claim a job whose
  Gemini config exposes remote agents. V2 nice-to-have: a doctor check
  that warns when `~/.gemini/agents/*.md` files declare `kind: remote`.

## Native subagents stay internal to the parent session

Yes, with the same caveats as Claude Code: subagents are spawned by the
parent's natural-language decisions or `@<agent-name>` syntax; they do
not register as Striatum sessions. The parent session publishes
artifacts and runs Striatum CLI commands. Recursion is blocked at the
Gemini layer.

`accountability.native_subagents = internal_to_parent_session` is the
right V1 default.

## Harness friction encountered during this research job

1. **Lane execution mode mismatch.** Same pattern as the codex and
   claude_code research jobs: the parent operator drove the work
   directly rather than launching `gemini` via the supervised lane.
   This lets the run progress despite the wrapper-script gap, but
   makes byline-as-provenance weaker than byline-as-intent.

2. **`verify_pipe_behavior_first` is a hard-coded supervisor caveat.**
   The profile field correctly flags the unknown, but workflow
   validation does nothing with it today. A V1 nice-to-have: when
   `supervision.compatible == "verify_pipe_behavior_first"` and a lane
   referencing the profile is used in a `striatum supervise start`
   path, emit a lint warning that names the verification step the
   operator must take.

3. **Trust-environment dependency on `GEMINI_CLI_TRUST_WORKSPACE`.**
   The lane env declares this var; if a user runs Gemini outside the
   Striatum lane and then re-enters it, the trust state may persist
   in `~/.gemini/` and shadow the workflow's intent. Out of scope for
   V1; flag for future hardening.

## Open questions for the designer

- Should `feature_flags.native_worktree: forbidden` be enforced by
  rejecting any lane command that contains `--worktree` or `-w`?
  Recommendation: workflow-validate lint warning in V1, error in V2.
- Should `remote_subagents_a2a: forbidden` be enforced by inspecting
  `~/.gemini/agents/*.md` for `kind: remote`? That is filesystem-aware
  validation; out of scope for V1 build slice. Recommend deferring to
  a doctor check rather than workflow-validate.
- Should the schema accept `output_format` as `"stream-json"` plus a
  `stream_json_event_filter` field that names which events the
  supervisor should parse? Out of scope for V1 build slice.
