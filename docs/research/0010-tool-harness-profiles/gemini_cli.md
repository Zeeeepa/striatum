# Gemini CLI — research for RFC 0010

Date: 2026-05-08
Author: research-claude-opus-4.7-1
Status: research note (not a decision)

This note collects what is publicly documented about Google Gemini CLI as of
May 2026 so that RFC 0010 can recommend a concrete `gemini_cli_default`
harness profile. It does not modify the RFC or any source code.

## Native delegation features

### Invocation modes

Gemini CLI exposes one binary, `gemini`, which switches between an
interactive TUI and a headless / scripting mode based on stdin and flags.

- `--prompt` / `-p <text>` — supplies a one-shot query and forces
  non-interactive mode. Stdin is read as additional context and the response
  prints to stdout, then the process exits. ([CLI reference][cli-ref];
  [Headless mode][headless])
- `--prompt-interactive` / `-i <text>` — opens the interactive UI but seeds
  the conversation with the given prompt. Designed for "warm-start" terminal
  sessions, not for piped supervision. ([CLI reference][cli-ref])
- Headless mode also triggers automatically when stdin is not a TTY, even
  without `-p`. The example `cat error.log | gemini -p "..."` is canonical.
  ([Automation tutorial][automation])
- `--output-format` / `-o {text,json,stream-json}` — `json` returns one
  envelope `{response, stats, error?}`; `stream-json` emits newline-delimited
  events of types `init`, `message`, `tool_use`, `tool_result`, `error`,
  `result`. ([Headless mode][headless])
- `--model` / `-m <alias|name>` — accepts aliases (`auto`, `pro`, `flash`,
  `flash-lite`) or concrete model names. ([CLI reference][cli-ref])
- `--include-directories <list>` — extends workspace context. ([CLI
  reference][cli-ref])
- `--debug` / `-d` — verbose logging. ([CLI reference][cli-ref])
- `--sandbox` / `-s` — runs the agent in a container or gVisor sandbox.
  ([Sandboxing][sandbox])
- `--extensions` / `-e <list>` — limits which extensions load.
- `--worktree` / `-w [name]` — starts the session inside a Git worktree.
  Requires `experimental.worktrees: true` in `settings.json`. ([Worktree
  PR][worktree-pr])

Exit codes documented for headless mode: `0` success, `1` general/API error,
`42` input error, `53` turn limit exceeded. ([Headless mode][headless])

JSON envelope:

```json
{
  "response": "string",
  "stats": { "...": "tokens, latency" },
  "error": { "...": "optional" }
}
```

A non-fatal warning: "Despite documentation, Gemini CLI does not support JSON
output" was logged as issue #9009 against earlier builds; the current
documented schema does work in 2026 builds, but operators should pin the CLI
version. ([Issue #9009][issue-9009])

### MCP support

Gemini CLI is MCP-first. Servers are declared in `settings.json` under
`mcpServers`, or registered via `gemini mcp add`. Three transports are
supported:

```json
{
  "mcpServers": {
    "github": {
      "command": "/path/to/server",
      "args": ["--flag"],
      "env": { "API_KEY": "$EXTERNAL_TOKEN" },
      "timeout": 30000,
      "trust": false,
      "includeTools": ["search", "create_issue"],
      "excludeTools": []
    },
    "remote-sse": {
      "url": "https://example.com/sse"
    },
    "remote-http": {
      "httpUrl": "https://example.com/mcp"
    }
  }
}
```

- Scope precedence: system → user (`~/.gemini/settings.json`) → project
  (`.gemini/settings.json`); project wins. ([MCP server config][mcp];
  [Configuration reference][config])
- Tool naming is automatically namespaced as `mcp_<server>_<tool>` to avoid
  collisions. ([MCP server config][mcp])
- `gemini mcp add -s user|project [--transport http|sse] -e KEY=VAL <name>
  <command|url>` is the imperative add path. ([MCP server config][mcp])
- Resources can be referenced inline as `@server://resource/path`.
- `trust: true` bypasses tool confirmation; this is the security knob that
  matters when Striatum wants to pre-approve a Striatum-owned MCP server.

Striatum's stdio JSON-RPC MCP wrapper (`python -m striatum.mcp`) maps
cleanly onto the `command`/`args` form. The framing default is
`Content-Length` LSP-style with line-delimited fallback; Gemini CLI's MCP
client should negotiate that automatically since both speak standard MCP
stdio.

### Custom slash commands (TOML)

- File location: `.gemini/commands/<path>.toml` (project) or
  `~/.gemini/commands/<path>.toml` (user). ([Custom commands][custom-cmd];
  [Cloud blog][cloud-blog])
- Filename → command name. Subdirectories become colon namespaces:
  `git/commit.toml` → `/git:commit`.
- Required field: `prompt`. Optional: `description`.
- Argument substitution via `{{args}}`; missing `{{args}}` causes the user
  text to be appended after two newlines.
- Inline shell substitution: `!{git diff --staged}` (the CLI prompts before
  executing).
- Inline file injection: `@{docs/best-practices.md}` (respects
  `.gitignore` and `.geminiignore`).
- `/commands list` and `/commands reload` manage runtime state.

Striatum mapping: a Striatum work packet could be wrapped as a TOML command
that prints the packet JSON via `!{striatum claim-next --session-id ...}`,
but the more durable pattern is to keep the packet as the actual stdin to
`gemini -p` and use TOML commands only for operator-facing helpers.

### Sub-agents / agent delegation

This is the strongest 2026 native-delegation surface and the reason a
Gemini-specific harness profile pays off. ([Subagents docs][subagents];
[Subagents repo source][subagents-md]; [Google announcement][google-blog];
[InfoQ][infoq])

- Definition file: Markdown with YAML frontmatter, stored at
  `.gemini/agents/*.md` (project) or `~/.gemini/agents/*.md` (user).
- Frontmatter fields:

  | Field | Required | Default | Notes |
  |---|---|---|---|
  | `name` | yes | — | lowercase slug, hyphens/underscores |
  | `description` | yes | — | drives automatic delegation |
  | `kind` | no | `local` | `local` or `remote` (A2A) |
  | `tools` | no | inherit all | array; supports `*`, `mcp_*`, `mcp_<srv>_*` |
  | `mcpServers` | no | — | inline MCP servers scoped to this agent |
  | `model` | no | parent | model override |
  | `temperature` | no | `1` | 0.0–2.0 |
  | `max_turns` | no | `30` | turn cap |
  | `timeout_mins` | no | `10` | wall-clock cap |

- Built-in subagents: `codebase_investigator`, `cli_help`, `generalist`,
  `browser_agent` (experimental). ([Subagents docs][subagents])
- Explicit delegation syntax: `@<agent-name> <task>` injects a system note
  that nudges the parent to call that agent immediately.
- Concurrent execution: documented and demonstrated (Google's announcement
  shows "Run the frontend-specialist on each package in parallel").
  ([Google announcement][google-blog]) Important caveat from the same post:
  parallel subagents that all *write code* can clobber each other; Google
  recommends parallel use for read/research/test tasks and sequential use
  for broad code edits.
- Recursion is blocked: subagents cannot spawn further subagents even with
  `tools: ['*']`.
- Policy engine integration: `~/.gemini/policies/*.toml` with `[[rules]]`
  blocks; the `subagent` field scopes a rule to a named agent. Striatum
  could ship a starter policy that pins shell-command prefixes.
- Remote subagents (A2A): `kind: remote` plus `agent_card_url` or
  `agent_card_json`. Auth: `apiKey`, `http` (Bearer/Basic), `google-credentials`,
  or `oauth`. ([Remote subagents][remote-agents])
- Disable globally: `experimental.enableAgents: false`.
- Override per-agent: `agents.overrides.<name>` in `settings.json`.

This is the closest analogue to Claude Code sub-agents in any non-Anthropic
tool today and the field RFC 0010 should treat as `preferred` for Gemini.

### Tools and extensions

Built-in tools include file ops (`read_file`, `write_file`, `replace`),
shell (`run_shell_command`), search (`grep_search`, `web_search`), and
`web_fetch`. Extensions live in `<repo>/.gemini/extensions/` or
`~/.gemini/extensions/` and are described by `gemini-extension.json`.
([Configuration reference][config]; [Extension reference][ext-ref])

`tools.allowed` and `tools.confirmationRequired` lists in `settings.json`
let project authors pre-approve tool names. `--allowed-tools` is the CLI
override but is documented as deprecated in favor of the policy engine.
([CLI reference][cli-ref])

### Hooks / lifecycle events

Hooks are scripts wired to lifecycle events via `settings.json`. ([Hooks
index][hooks]; [Hooks reference][hooks-ref])

Documented events:

- `SessionStart`, `SessionEnd`
- `BeforeAgent`, `AfterAgent`
- `BeforeModel`, `AfterModel`
- `BeforeToolSelection`, `BeforeTool`, `AfterTool`
- `PreCompress`
- `Notification`

Configuration shape:

```json
{
  "hooks": {
    "BeforeTool": [
      {
        "matcher": "write_file|replace",
        "hooks": [
          {
            "name": "striatum-write-scope-check",
            "type": "command",
            "command": "$GEMINI_PROJECT_DIR/.gemini/hooks/scope.sh",
            "timeout": 5000
          }
        ]
      }
    ]
  }
}
```

- Hook stdout MUST be a single JSON object; any other text breaks parsing.
- Exit code `2` is a hard block (CLI aborts the matched action); other
  non-zero exits are warnings and the action proceeds.
- Tool-event matchers are regexes; lifecycle matchers are exact strings.
- Environment vars: `GEMINI_PROJECT_DIR`, `GEMINI_PLANS_DIR`,
  `GEMINI_SESSION_ID`, `GEMINI_CWD`, plus a `CLAUDE_PROJECT_DIR`
  compatibility alias.

This is a directly useful integration point for Striatum: a `BeforeTool`
hook can enforce write-scope advisories, and `AfterTool` can publish
artifact references. RFC 0010's profile should expose hooks as a
first-class `allowed` capability for the Gemini lane.

### Memory / context persistence

- `GEMINI.md` files (`context.fileName` setting) act like
  `CLAUDE.md`/`AGENTS.md`: project-wide standing instructions. ([Memory
  management][memory])
- `/memory add <text>` appends to the active `GEMINI.md`; `/memory show`
  prints the concatenated context.
- Sessions persist to `~/.gemini/tmp/<project_hash>/chats/` automatically;
  switching directories switches session histories. ([Session
  management][sessions])
- `Checkpointing` saves a shadow Git snapshot before any tool-driven file
  modification, supporting `/rewind`. ([Checkpointing][checkpoint])

For Striatum this matters because: (a) Gemini CLI keeps its own session
state outside SQLite, so the "no transcript capture" invariant must be
preserved by *not* relying on `~/.gemini/tmp/...` for workflow state; and
(b) `GEMINI.md` is the natural place to inject the prompt envelope from a
harness profile.

### Multi-instance behavior

- Concurrent invocations on the same checkout share `.git` and risk
  corruption — this is a well-known Git-client invariant, not unique to
  Gemini CLI. ([Parallel discussion][parallel-discussion]; [Worktree
  issue][worktree-issue])
- Native answer: the `--worktree` / `-w` flag plus `experimental.worktrees:
  true` provisions a per-session worktree and tears it down on clean exit;
  worktrees with uncommitted changes are preserved. ([Worktree
  PR][worktree-pr])
- Community tools (e.g. `gemini-wt`) wrap the same pattern.

This dovetails with RFC 0008. A Striatum lane can either:

1. Trust Striatum's worktree manager and run `gemini` inside the
   pre-created worktree (preferred — keeps Striatum authoritative), or
2. Delegate worktree creation to Gemini's `--worktree`, which is
   simpler but moves a coordination concern outside SQLite.

Option 1 is the safer default; option 2 should only be allowed by an
explicit profile flag.

### Permissions / approval modes

`--approval-mode {default, auto_edit, yolo, plan}` plus the legacy
`--yolo` shortcut. ([CLI reference][cli-ref]; [Configuration
reference][config])

- `default` — confirm every tool call.
- `auto_edit` — auto-approve edit tools (`write_file`, `replace`); confirm
  others.
- `yolo` — auto-approve all tool calls (script/CI).
- `plan` — read-only "plan mode"; the agent cannot mutate the workspace.

Companion settings: `general.defaultApprovalMode`,
`security.disableYoloMode`, `tools.allowed`, `tools.confirmationRequired`.
Trusted-folder enforcement (`~/.gemini/trustedFolders.json`) ignores
project `.gemini/settings.json` for untrusted folders. ([Trusted
folders][trusted])

### Agentic / ReAct loop constraints

- `model.maxSessionTurns` caps the parent loop; subagent
  `max_turns`/`timeout_mins` cap each delegation independently.
- `general.plan.enabled` toggles plan mode globally.
- `experimental.autoMemory` lets the agent self-extract memory patches
  during the loop — operators who want determinism should disable it.

## How teams use it in the wild

- **Parallel worktree fan-out for code review and refactor.** Dagger's blog
  documents using Gemini CLI plus worktrees to run multiple agents on the
  same repo. ([Dagger blog][dagger]) The PI/Neural Engineer post does the
  same with explicit per-task worktrees. ([Neural Engineer][neural])
- **Multi-agent prompting before subagents shipped.** A Substack walkthrough
  shows how to fake subagent delegation with prompt scaffolding before the
  native feature existed. ([AI Positive][ai-positive]) Useful as a
  fallback profile when subagents are disabled.
- **Google Apps Script as A2A remote subagents.** Tanaike's repo demonstrates
  registering an Apps Script endpoint as a Gemini remote subagent.
  ([GAS A2A][gas-a2a])
- **Custom commands for repeated workflows.** Romin Irani's tutorial series
  and DEV.to walkthroughs show TOML commands for commit-message generation
  and refactor recipes. ([Tutorial part 7][tutorial-7]; [DEV][dev])
- **CI/CD shaping.** Inventive HQ and Addy Osmani document
  `git diff | gemini -p "..." --output-format json` as the canonical
  pipeline pattern. ([Inventive][inventive]; [Addy][addy])
- **Hook-driven safety nets.** The Maybe-Don't-AI guide and the Google
  developers blog show how to wrap `BeforeTool` for scope checking.
  ([Maybe-Don't][maybe-dont]; [Google blog hooks][google-hooks])

## Mapping to RFC 0010 schema

### Coverage check against current draft

RFC 0010 §"Profile Fields" lists `feature_flags` keys: `subagents`,
`agent_teams`, `skills`, `custom_agent_roles`, `hooks`, `mcp`, `headless`,
`worktree_agents`. Gemini CLI maps as follows:

| RFC 0010 flag | Gemini analogue | Maps cleanly? |
|---|---|---|
| `subagents` | `.gemini/agents/*.md`, `@<agent>` syntax | yes |
| `agent_teams` | none documented (parallel subagents are ad hoc) | partial — call it `parallel_subagents` instead |
| `skills` | none — Gemini does not have a "Skill" surface | no, leave unset / forbidden |
| `custom_agent_roles` | subagent definitions cover this | yes (overlaps `subagents`) |
| `hooks` | `settings.json` `hooks.<event>[]` | yes |
| `mcp` | `settings.json` `mcpServers` | yes |
| `headless` | `-p` + `--output-format json` | yes |
| `worktree_agents` | `--worktree` plus `experimental.worktrees` | yes |

Schema gaps the Gemini CLI lane will expose:

1. **`mcp_servers` field** — RFC 0010 should let a profile *enumerate*
   recommended MCP servers (with scope hints) rather than just flag MCP as
   `allowed`. Otherwise every project re-derives the same Striatum-MCP
   config.
2. **`slash_command_dir` / `prompt_command_dir` field** — the profile
   should be able to point at a directory of TOML commands shipped with
   Striatum.
3. **`approval_mode` field** — Gemini's four-valued enum (`default`,
   `auto_edit`, `yolo`, `plan`) does not fit `native_delegation.mode` and
   is not a `feature_flag`. Add a top-level `approval_mode` capability.
4. **`output_format` field** — codifying `text` vs `json` vs `stream-json`
   helps the supervisor (RFC 0009) commit to a parser without sniffing.
5. **`turn_caps`** — `max_session_turns`, `subagent_max_turns`,
   `subagent_timeout_mins`. These are the only knobs that bound runaway
   delegation.
6. **`memory_files`** — list of `GEMINI.md` (or analogue) paths the lane
   should ensure are present. Avoids drift between Striatum's prompt
   envelope and the tool's standing instructions.
7. **`policy_engine_path`** — `.gemini/policies/*.toml`. Maps directly to
   subagent-scoped allow/deny rules and is too specific to live inside the
   generic `accountability` block.

Items 1–3 are the most important; the others are nice-to-have once the
Codex/Claude Code parallel research lands.

### `gemini_cli_default` profile (proposed)

```json
{
  "tool_family": "gemini_cli",
  "strategy_version": "2026-05-08",
  "native_delegation": {
    "mode": "encouraged",
    "instruction": "Use @<agent> delegation for read-only investigation and parallel test/research fan-out. Keep code-write delegations sequential unless the parent integrates results before the next write. Subagents inherit the parent's write scope and lease.",
    "max_parallel_native_agents": "bounded_by_write_scope"
  },
  "feature_flags": {
    "subagents": "preferred",
    "parallel_subagents": "allowed_for_read_only",
    "skills": "unsupported",
    "custom_agent_roles": "preferred",
    "hooks": "allowed",
    "mcp": "preferred",
    "headless": "required_for_lane",
    "worktree_agents": "forbidden"
  },
  "approval_mode": "auto_edit",
  "output_format": "stream-json",
  "turn_caps": {
    "max_session_turns": -1,
    "subagent_max_turns": 30,
    "subagent_timeout_mins": 10
  },
  "mcp_servers": [
    { "name": "striatum", "scope": "project", "transport": "stdio",
      "command": "python", "args": ["-m", "striatum.mcp"] }
  ],
  "slash_command_dir": ".gemini/commands",
  "policy_engine_path": ".gemini/policies/striatum.toml",
  "memory_files": ["GEMINI.md"],
  "accountability": {
    "native_subagents": "internal_to_parent_session",
    "first_class_registration": "not_supported"
  },
  "fallback_profile_id": "generic_default"
}
```

Justifications:

- `subagents: preferred` — the tool ships them and they are the cleanest
  delegation surface available outside Claude Code.
- `parallel_subagents: allowed_for_read_only` — Google's own announcement
  warns against parallel code-write delegation. ([Google
  announcement][google-blog])
- `skills: unsupported` — there is no skills concept; do not pretend.
- `worktree_agents: forbidden` — Striatum's RFC 0008 worktree manager is
  authoritative; do not let `--worktree` create rows Striatum does not
  know about. Operators who want Gemini-managed worktrees can opt into a
  separate profile.
- `headless: required_for_lane` — the supervisor (RFC 0009) writes packets
  to a FIFO line-by-line; the agent must be in headless mode.
- `approval_mode: auto_edit` — `default` would block on every tool call
  inside an unattended session; `yolo` removes scope discipline. `auto_edit`
  is the documented middle ground.
- `output_format: stream-json` — gives the supervisor a stable JSONL
  channel without parsing prose. `json` would only deliver one envelope
  per session, which fights the multi-packet supervisor model.
- `turn_caps` — defaults from the published subagent schema.

## Recommended Striatum lane configuration

```json
{
  "lanes": {
    "gemini": {
      "adapter": "process",
      "harness_profile_id": "gemini_cli_default",
      "command": ["gemini", "--prompt", "-", "--output-format", "stream-json", "--approval-mode", "auto_edit"],
      "stdin_mode": "packet_jsonl",
      "env": {
        "GEMINI_CLI_TRUST_WORKSPACE": "1",
        "NO_COLOR": "1"
      },
      "constraints": {
        "network": "advisory_strict",
        "repo_scope": "advisory_strict",
        "transcript": "off"
      },
      "worktree_required": true
    }
  }
}
```

Quirks the supervisor should know about:

1. **Stdin behavior with `-p`.** Documented behavior: when `-p <text>` is
   provided, `<text>` is appended to whatever stdin contains. If the
   supervisor is sending one work packet per `supervise send`, the cleanest
   shape is `gemini --prompt-interactive -` *only if* the supervisor is
   willing to keep an interactive session open; otherwise prefer
   `gemini --prompt "<envelope>" --output-format stream-json` and write the
   packet body to stdin from the FIFO. This needs a real-CLI smoke test;
   the documented behavior of `-p` "appending to stdin" can be brittle in
   pipe contexts. ([Headless mode][headless])
2. **TTY detection.** Gemini CLI auto-enables headless when stdin is not a
   TTY. The Striatum supervisor sends stdin via `os.mkfifo`, which is not
   a TTY, so this should Just Work — but it also means *interactive*
   subagent delegation cannot be confirmed, which is why
   `approval_mode=auto_edit` (or `yolo`) is required.
3. **Trusted folder prompt.** A first run inside a fresh worktree may
   trigger the trusted-folder prompt and block stdin. Set
   `GEMINI_CLI_TRUST_WORKSPACE=1` or pre-populate
   `~/.gemini/trustedFolders.json` for the worktree path before
   `supervise start`.
4. **Multi-instance safety.** Per the Git invariant, never run two
   `gemini` processes against the same checkout. Striatum's worktree-per-
   lease (RFC 0008) already enforces this; the profile should set
   `worktree_required: true`.
5. **Hidden state.** Gemini CLI will write to
   `~/.gemini/tmp/<project_hash>/chats/` and may write checkpoints to a
   shadow `.git`. Workflow state must not depend on these files; the
   process adapter's transcript-off enforcement still holds because
   stdout/stderr go to `DEVNULL` in supervised mode.
6. **Stream-json schema is not pinned.** Issue #9009 shows past drift
   between docs and behavior; pin the Gemini CLI version in the lane env or
   the smoke script.

## Gaps and risks

- **Profile schema is too narrow for Gemini CLI.** Items 1–3 in the schema
  gap list above (MCP enumeration, slash-command dir, approval mode) are
  not optional once we ship a Gemini lane; they should land before the
  first dogfood run.
- **Parallel subagents inside a single Striatum job blur D015.** Native
  subagents are explicitly internal to the parent session per RFC 0010,
  but Gemini's parallel fan-out can produce many concurrent edit attempts
  inside one lease. The profile must keep `parallel_subagents` scoped to
  read-only delegations (or the parent must serialize writes) so a single
  lease does not rewrite the same files from N branches.
- **`--worktree` overlaps RFC 0008.** Two worktree managers fighting over
  the same checkout will lose state. The proposed profile forbids the
  feature; if a future RFC wants to use it, RFC 0008 needs a hand-off
  contract.
- **TTY edge cases for the supervisor.** The headless docs do not describe
  FIFO-as-stdin behavior. The first dogfood run with a real Gemini CLI must
  verify that packets delivered via `os.mkfifo` are read line-by-line and
  that `-p` interacts cleanly with stdin under that pipe shape.
- **Approval mode collisions.** `general.defaultApprovalMode` in
  `settings.json` and `--approval-mode` on the CLI both exist; a project
  `.gemini/settings.json` set to `default` would override the lane flag
  for anything that re-reads settings mid-session. Pin via CLI flag *and*
  document the project setting as part of the profile.
- **Schema drift.** Subagent frontmatter and hook event names have changed
  between Gemini CLI minor versions during 2025–2026. Profiles must carry
  `strategy_version` (RFC 0010 already does) and the smoke runner should
  warn when the installed `gemini` major/minor differs from the recorded
  profile version.

## Open questions for the human owner

1. Should `gemini_cli_default` ship with a Striatum-authored `.gemini/`
   tree (subagents, slash commands, hook scripts) under `examples/` so a
   target repository can opt in with a single copy, or should the profile
   stay declarative and let projects build their own?
2. Should Striatum support Gemini's remote (A2A) subagents at all? They
   reach external services, which conflicts with the no-hosted-services
   product boundary in `AGENTS.md`. Most likely: keep `kind: local` only
   and require a separate decision before allowing `kind: remote`.
3. Do we need a second Gemini profile (`gemini_cli_review_only`) that pins
   `approval_mode=plan` for review lanes, or is approval-mode per-lane
   (not per-profile) sufficient?
4. Does RFC 0010 want to model `output_format` as a *capability* (the lane
   declares what it can produce) or a *requirement* (the supervisor
   declares what it expects)? Today's draft has neither field.
5. How should the profile enumerate MCP servers without duplicating the
   workflow's lane env? Probably a `mcp_servers: [{name, transport, ...}]`
   list with a "must be reachable from the lane process" invariant, but
   the exact shape needs review.

[cli-ref]: https://geminicli.com/docs/cli/cli-reference/
[headless]: https://geminicli.com/docs/cli/headless/
[automation]: https://geminicli.com/docs/cli/tutorials/automation/
[config]: https://geminicli.com/docs/reference/configuration/
[mcp]: https://geminicli.com/docs/tools/mcp-server/
[custom-cmd]: https://geminicli.com/docs/cli/custom-commands/
[cloud-blog]: https://cloud.google.com/blog/topics/developers-practitioners/gemini-cli-custom-slash-commands
[subagents]: https://geminicli.com/docs/core/subagents/
[subagents-md]: https://github.com/google-gemini/gemini-cli/blob/main/docs/core/subagents.md
[google-blog]: https://developers.googleblog.com/subagents-have-arrived-in-gemini-cli/
[infoq]: https://www.infoq.com/news/2026/04/subagents-gemini-cli/
[remote-agents]: https://geminicli.com/docs/core/remote-agents/
[hooks]: https://geminicli.com/docs/hooks/
[hooks-ref]: https://geminicli.com/docs/hooks/reference/
[google-hooks]: https://developers.googleblog.com/tailor-gemini-cli-to-your-workflow-with-hooks/
[memory]: https://geminicli.com/docs/cli/tutorials/memory-management/
[sessions]: https://geminicli.com/docs/cli/session-management/
[checkpoint]: https://geminicli.com/docs/cli/checkpointing/
[sandbox]: https://geminicli.com/docs/cli/sandbox/
[trusted]: https://geminicli.com/docs/cli/trusted-folders/
[ext-ref]: https://geminicli.com/docs/extensions/reference/
[worktree-pr]: https://github.com/google-gemini/gemini-cli/pull/22973
[worktree-issue]: https://github.com/google-gemini/gemini-cli/issues/22945
[parallel-discussion]: https://github.com/google-gemini/gemini-cli/discussions/3395
[issue-9009]: https://github.com/google-gemini/gemini-cli/issues/9009
[dagger]: https://dagger.io/blog/gemini-cli/
[neural]: https://medium.com/neural-engineer/parallel-agentic-code-development-with-gemini-cli-and-git-worktrees-d2345180663c
[ai-positive]: https://aipositive.substack.com/p/how-i-turned-gemini-cli-into-a-multi
[gas-a2a]: https://github.com/tanaikech/gemini-cli-gas-a2a-subagents
[tutorial-7]: https://medium.com/google-cloud/gemini-cli-tutorial-series-part-7-custom-slash-commands-64c06195294b
[dev]: https://dev.to/gioboa/gemini-cli-custom-commands-are-so-cool-1nma
[inventive]: https://inventivehq.com/knowledge-base/gemini/how-to-use-headless-mode
[addy]: https://addyo.substack.com/p/gemini-cli-tips-and-tricks
[maybe-dont]: https://maybedont.ai/docs/agents/hooks/gemini-cli/
