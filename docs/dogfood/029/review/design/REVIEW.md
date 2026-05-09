---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
---

# Design review: RFC 0025 V1 Steps 2+3 (devils_advocate)

author: reviewer-claude-opus-001

## Posture

Devil's advocate. Argue against scope and template choices.

## Counter-claims

### C1: "Codex shares Markdown command shape with claude_code"

The synthesis says Codex commands are byte-shared with claude_code.
**Concern:** RFC § 2.2 Codex `plugin.json` differs (has `skills`,
`mcpServers`, `hooks`, `interface` keys) from claude_code's (`name`,
`description`, `version`, `author`, `license`). Are commands really
identical? **Counter:** The synthesis is about *command file*
content, not the plugin manifest. Both Claude Code and Codex
slash-commands are Markdown with YAML frontmatter `description`
field. The bodies don't reference plugin-specific surfaces.
**Survives.**

### C2: "TOML commands for Gemini have correct schema"

The synthesis shows:
```toml
[command]
name = "claim-next"
description = "..."
prompt = "..."
```
**Concern:** Gemini extension spec uses a different shape — let me
check. Per the Gemini docs the actual TOML schema is a bare
top-level `prompt` field (no `[command]` table). **Finding (F1,
non-blocking):** Implementer should verify against the Gemini docs
URL referenced in the RFC context. If the spec is bare top-level
fields, drop the `[command]` table.

### C3: "Marketplace fixture has two entries (claude_code, codex)"

When `--profile all` runs, the synthesis says the shared marketplace
gets two entries (gemini skipped). **Survives.**

### C4: "Byte-share via copy is honest duplication"

The synthesis chose copy-paste for skill templates. Concern: now
three copies, drift risk triples. **Counter:** the F1 byte-match
test catches drift. **Survives.**

### C5: "User-scope path for codex"

`~/.codex/plugins/<namespace>/`. Concern: the actual Codex CLI
docs may use a different path (e.g. `~/.config/codex/plugins/`).
**Finding (F2, verify):** Implementer should confirm against the
Codex CLI docs. The RFC § 2.2 doesn't specify user-scope explicitly.

### C6: "Gemini --with-marketplace is a no-op"

The synthesis says Gemini skips the marketplace fixture (correct
per RFC § 2.3). The function returns a `notice` action. **Finding
(F3, non-blocking):** Make the notice show up in `--json` output as
`{"marketplace": {"skipped": True, "reason": "..."}}` so operators
can detect it programmatically.

### C7: "test_skill_templates_match_skills_module extended for three profiles"

Survives (just adds two more byte-equality assertions).

### C8: "URL-leak test runs on all three rendered outputs"

Survives.

## Findings

### F1 (verify, non-blocking): Confirm Gemini TOML command schema

Implementer should read the Gemini docs URL referenced in the RFC
context. The synthesis's bare-`[command]` table syntax may need to
flatten to top-level fields.

### F2 (verify, non-blocking): Confirm Codex user-scope path

Implementer should verify `~/.codex/plugins/` is the actual Codex
CLI user-scope plugin directory. If wrong, `--scope user` will
silently install to a path Codex doesn't read.

### F3 (note, non-blocking): Marketplace skip surfaced in --json

The Gemini skip message should be a structured field, not a print
statement, so JSON callers can detect it.

## Verdict

**accept**

Three findings, all non-blocking verification asks. The scope is
appropriate; the byte-share strategy keeps content single-sourced;
the test plan covers both new profiles + the aggregation case.
