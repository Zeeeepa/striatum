---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# RFC 0025 V1 Steps 2+3 design

author: designer-codex-gpt-5.5-001

## Scope

- **Step 2: codex profile** — 13-file Codex plugin bundle.
- **Step 3: gemini profile** — 14-file Gemini extension. Promotes
  `gemini` from RFC 0015's `generic` fallback to first-class.
- **`--profile all`** — aggregates all three profiles in one
  invocation, with shared marketplace fixture.

## Module changes

### `plugins/install.py`

```python
ALLOWED_PROFILES = frozenset({"claude_code", "codex", "gemini"})
ALL_PROFILES_ORDER = ("claude_code", "codex", "gemini")
```

`_build_plan` adds `_plan_codex` and `_plan_gemini`. `_bundle_root`
adds user-scope branches:

```python
if profile == "codex":  # user
    return base / ".codex" / "plugins" / namespace
if profile == "gemini":  # user
    return base / ".gemini" / "extensions" / namespace
```

`_write_marketplace` skips when `profile == "gemini"`:

```python
if profile == "gemini":
    return {"path": None, "skipped": True, "reason": "gemini has no marketplace concept"}
```

### CLI

`--profile` choices for `plugin install/uninstall` and
`init --with-plugins` expand to
`{claude_code, codex, gemini, all}`. The `all` value loops
`install()` over `ALL_PROFILES_ORDER` and aggregates results.

## Templates to author

### Codex (under `plugins/templates/codex/`)

- `plugin.json.tmpl` — RFC § 2.2 shape.
- `skills/{workflow,scaffold,claim-loop,supervise,recover}.md.tmpl`
  — byte-identical copies of claude_code skill templates.
- `commands/{claim-next,status,why,dashboard,doctor}.md.tmpl`
  — byte-identical copies of claude_code command templates (Codex
  uses the same Markdown shape).
- `hooks/hooks.json.tmpl` — byte-identical copy of claude_code.
- `mcp.json.tmpl` — byte-identical copy of claude_code (empty `{}`).
- `README.md.tmpl` — Codex-specific install instructions.

### Gemini (under `plugins/templates/gemini/`)

- `gemini-extension.json.tmpl` — RFC § 2.3 shape.
- `GEMINI.md.tmpl` — context file linking to skills + commands.
- `skills/{workflow,scaffold,claim-loop,supervise,recover}.md.tmpl`
  — byte-identical copies of claude_code skill templates.
- `commands/{claim-next,status,why,dashboard,doctor}.toml.tmpl`
  — fresh TOML content per Gemini extension spec.
- `agents/striatum-recover.md.tmpl` — sub-agent definition.
- `README.md.tmpl` — Gemini-specific install instructions.

## F1 byte-match invariant

`test_skill_templates_match_skills_module` extended:

- Asserts `plugins/templates/{claude_code,codex,gemini}/skills/<name>.md.tmpl`
  all match `skills/templates/claude_code/<name>.md.tmpl` byte-for-byte.

## URL-leak invariant

Same test as Step 1, applied to all three rendered profile outputs.

## Test plan

| Test | Coverage |
| --- | --- |
| `test_codex_install_writes_full_bundle` | 13 files |
| `test_codex_idempotent_re_install` | byte-no-op |
| `test_codex_edit_detect` | edit-detect + force |
| `test_codex_no_external_urls` | URL-leak |
| `test_gemini_install_writes_full_bundle` | 14 files |
| `test_gemini_idempotent_re_install` | byte-no-op |
| `test_gemini_marketplace_skipped` | no marketplace.json entry |
| `test_gemini_no_external_urls` | URL-leak |
| `test_profile_all_writes_three_bundles` | aggregation |
| `test_profile_all_marketplace_has_two_entries` | claude_code + codex (gemini skipped) |
| `test_skill_templates_match_skills_module` | extended for all three |

## Out of scope (V2)

- Cross-target install.
- Hosted marketplace.
- Codex `apps/` and `assets/`.
- Per-target git-repo extension format for gemini.
- `--profile all` user-scope (V1 keeps it project-scope).
