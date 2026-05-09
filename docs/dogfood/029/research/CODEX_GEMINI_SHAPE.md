---
title: "RFC 0025 V1 Steps 2+3 shape research: codex + gemini"
date: 2026-05-09
---

# Codex + Gemini plugin profile shape

author: researcher-codex-gpt-5.5-001

## Step 2: Codex profile

### Bundle layout (project scope)

```
.striatum/plugins/codex/
├── .codex-plugin/plugin.json
├── skills/
│   ├── striatum-workflow/SKILL.md   ── reuse claude_code skill bodies
│   ├── striatum-scaffold/SKILL.md
│   ├── striatum-claim-loop/SKILL.md
│   ├── striatum-supervise/SKILL.md
│   └── striatum-recover/SKILL.md
├── commands/
│   ├── claim-next.md   ── Markdown like claude_code (Codex shares the format)
│   ├── status.md
│   ├── why.md
│   ├── dashboard.md
│   └── doctor.md
├── hooks/hooks.json
├── .mcp.json
├── .manifest.json
└── README.md
```

13 files (no shared `mcp` placeholder difference; same shape as
claude_code with the manifest dir renamed `.codex-plugin/`).

### plugin.json shape (RFC § 2.2)

```json
{
  "name": "<namespace>",
  "version": "<striatum_version>",
  "description": "Drive Striatum workflows from Codex.",
  "skills": "./skills/",
  "mcpServers": "./.mcp.json",
  "hooks": "./hooks/hooks.json",
  "interface": {
    "displayName": "Striatum",
    "shortDescription": "Local-first workflow runner for terminal AI agents.",
    "category": "Developer Tools"
  }
}
```

### User scope path

`~/.codex/plugins/<namespace>/`.

### Templates to author

- `codex/plugin.json.tmpl`
- `codex/skills/{workflow,scaffold,claim-loop,supervise,recover}.md.tmpl`
  (byte-shared with claude_code per F1)
- `codex/commands/{claim-next,status,why,dashboard,doctor}.md.tmpl`
  (byte-shared with claude_code: same Markdown shape works for Codex)
- `codex/hooks/hooks.json.tmpl` (byte-share with claude_code)
- `codex/mcp.json.tmpl` (byte-share with claude_code: empty `{}`)
- `codex/README.md.tmpl` (Codex-specific install instructions)

Net new template content: just `plugin.json.tmpl` + `README.md.tmpl`.
Everything else byte-shares.

## Step 3: Gemini profile

### Bundle layout (project scope)

```
.striatum/plugins/gemini/
├── gemini-extension.json
├── GEMINI.md                ── top-level Striatum-driving guide (context file)
├── commands/
│   ├── claim-next.toml      ── TOML format per Gemini extension spec
│   ├── status.toml
│   ├── why.toml
│   ├── dashboard.toml
│   └── doctor.toml
├── skills/                  ── carry-over from claude_code (byte-share)
│   ├── striatum-workflow/SKILL.md
│   └── ...
├── agents/
│   └── striatum-recover.md
├── .manifest.json
└── README.md
```

14 files (`gemini-extension.json` + GEMINI.md + 5 TOML commands +
5 skills + agents/striatum-recover.md + manifest + README).

Note: Gemini does not have an `mcp.json` or `hooks.json` separate
file in V1 — its extension format embeds those in the
`gemini-extension.json` if needed. V1 keeps both empty / absent
per the RFC.

### gemini-extension.json shape (RFC § 2.3)

```json
{
  "name": "<namespace>",
  "version": "<striatum_version>",
  "description": "Drive Striatum workflows from Gemini CLI.",
  "contextFileName": "GEMINI.md",
  "excludeTools": []
}
```

### TOML command shape (Gemini)

```toml
[command]
name = "claim-next"
description = "Claim the next eligible work packet"
prompt = "Run `striatum claim-next --session-id $SESSION_ID --json`..."
```

### User scope path

`~/.gemini/extensions/<namespace>/`.

### `--with-marketplace` for gemini

No-op per RFC § 2.3 — Gemini doesn't have a comparable marketplace
concept. Print a notice; don't write a file.

### Templates to author

- `gemini/gemini-extension.json.tmpl`
- `gemini/GEMINI.md.tmpl` (the top-level context file)
- `gemini/skills/{workflow,scaffold,claim-loop,supervise,recover}.md.tmpl`
  (byte-shared with claude_code per F1)
- `gemini/commands/{claim-next,status,why,dashboard,doctor}.toml.tmpl`
  (TOML — different format, fresh content)
- `gemini/agents/striatum-recover.md.tmpl`
- `gemini/README.md.tmpl`

## Net work for the implementer

- `ALLOWED_PROFILES` += `{"codex", "gemini"}`. CLI `--profile`
  choices += same.
- `_bundle_root` adds user-scope paths for codex (`~/.codex/plugins/`)
  and gemini (`~/.gemini/extensions/`).
- `_build_plan` adds `_plan_codex` and `_plan_gemini` mirroring
  `_plan_claude_code` shape.
- `_write_marketplace` skips gemini (or returns `notice` action).
- New templates listed above.
- F1 byte-share test extended: skill bodies under all three
  profiles match `skills/templates/claude_code/` byte-for-byte.
- Tests for codex install (full bundle, idempotent, edit-detect,
  URL-leak); same for gemini.
- `--profile all` aggregation — when implementer wants — iterates
  over `ALL_PROFILES_ORDER` and calls install per profile.

## Out of scope (V2)

- Cross-target install.
- Hosted marketplace.
- Codex `apps/` and `assets/` (RFC Open Question; deferred).
- Per-target git-repo extension format for gemini.
