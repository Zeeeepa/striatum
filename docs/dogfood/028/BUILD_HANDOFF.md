---
title: "RFC 0025 V1 Step 1 build handoff (dogfood-028)"
date: 2026-05-09
---

# Build handoff: RFC 0025 V1 Step 1 (claude_code plugin profile)

author: implementer-claude-opus-001

## Scope

V1 Step 1 ships the **claude_code plugin profile end-to-end** plus
the shared infrastructure that Steps 2-3 (codex, gemini) will plug
into:

1. New module `src/striatum/plugins/install.py` mirroring
   `skills/install.py`.
2. 14 templates under `src/striatum/plugins/templates/claude_code/`
   (5 skills shared with `skills/`, 5 commands, plugin.json,
   hooks.json, mcp.json, README.md).
3. CLI verbs: `striatum plugin install/uninstall`.
4. `striatum init --with-plugins` mirror of `--with-skills`.
5. Doctor checks: `plugin_missing`, `plugin_outdated`.
6. Marketplace fixture writer (default-on; reentrant merge).
7. 14 tests covering the full surface.

Steps 2 (codex) and 3 (gemini promote-from-generic) remain
deferred to follow-up dogfoods.

## Files

### New

- `src/striatum/plugins/__init__.py`
- `src/striatum/plugins/install.py` — install/uninstall pipeline.
- `src/striatum/plugins/templates/claude_code/`:
  - `plugin.json.tmpl` — `.claude-plugin/plugin.json` body.
  - `skills/{workflow,scaffold,claim-loop,supervise,recover}.md.tmpl`
    — byte-identical copies of `skills/templates/claude_code/*.md.tmpl`.
  - `commands/{claim-next,status,why,dashboard,doctor}.md.tmpl`.
  - `hooks/hooks.json.tmpl`, `mcp.json.tmpl`, `README.md.tmpl`.
- `tests/test_plugin_install.py` — 14 tests covering install,
  idempotency, dry-run, edit-detect, force overwrite, URL-leak
  invariant, marketplace append, uninstall (force + refused),
  unknown profile/scope, **and** the F1 skill-template-byte-match
  check.

### Modified

- `src/striatum/cli/parser.py` — `striatum plugin install/uninstall`
  subcommands; `init --with-plugins`.
- `src/striatum/cli/dispatch.py` — wires the new subcommands;
  `init --with-plugins` calls `plugins.install.install`.
- `src/striatum/cli/introspect.py` — adds `plugin_missing` and
  `plugin_outdated` to `DOCTOR_CHECKS`; doctor walks every
  installed bundle's `.manifest.json` and reports drift.

### Docs

- `CHANGELOG.md` — `## 1.19.0 — 2026-05-09` section.
- `pyproject.toml` + `__init__.py` — bumped to `1.19.0`.
- `docs/DECISION_LOG.md` — D081 row.
- `docs/TODO.md` — F28 row.
- `docs/rfcs/0025-agent-cli-plugin-bundles.md` — status
  `accepted (V1 step 1)`.
- `docs/rfcs/README.md` — index updated.

## Design-review disposition

| Finding | Severity | Disposition |
| --- | --- | --- |
| F1: Test that skill bodies match | recommend | **Adopted**: `test_skill_templates_match_skills_module` asserts byte-equality between the two template trees. |
| F2: Doctor walks bundles, index is fallback | recommend | **Simplified**: doctor walks `.striatum/plugins/<profile>/.manifest.json` directly using `PLUGIN_PROFILES`; no separate `manifest_index.json` was introduced. Cleaner than the synthesis (one less moving piece). |
| F3: URL-leak whitelist policy | note | **Adopted**: test forbids any scheme (`https?://`, `git://`, `file://`, `ssh://`, `ftp://`) in rendered output. |
| F4: Idempotency test scope | note | **Adopted**: `test_claude_code_idempotent_re_install` snapshots bundle file bytes, re-installs, and asserts bytes are unchanged for every bundle file (manifest is allowed to change because `generated_at` updates). |

## Implementation notes

### Helper expansion reuse

`plugins.install._render` calls `skills.install._expand_helpers` so
skill body templates render identically across both modules. This
avoids re-implementing the verb-table / boundaries / front-matter
substitution.

### Manifest location

`<bundle>/.manifest.json` (inside the bundle) — different from
`skills`' `<base>/.<bundle-name>/.manifest.json`. Each plugin
bundle is self-describing; deleting the bundle dir is a clean
uninstall fallback.

### Marketplace merge

`_write_marketplace` reads the existing `marketplace.json` (if any),
merges by matching `(name, source.path)`, and rewrites. A second
install of the same profile updates in place; a future codex
install will append a new entry without touching the claude_code
one.

### Plugin manifest schema

```json
{
  "schema_version": "striatum.plugins.manifest.v1",
  "striatum_version": "1.19.0",
  "generated_at": "2026-05-09T...",
  "profile": "claude_code",
  "namespace": "striatum",
  "scope": "project",
  "files": [
    {"path": ".claude-plugin/plugin.json", "sha256": "...", "template": "...", "template_sha256": "..."},
    ...
  ]
}
```

## Smoke

```
$ striatum --repo /tmp/test init --with-plugins
{"data": {"plugins": {"profile": "claude_code", ...}}, "ok": true}

$ ls /tmp/test/.striatum/plugins/claude_code/
.claude-plugin/  .manifest.json  .mcp.json  README.md
commands/  hooks/  skills/

$ striatum --repo /tmp/test plugin install --profile claude_code
# 14 files written; second invocation is byte-identical.

$ striatum --repo /tmp/test doctor --verbose
# After deleting .striatum/plugins/claude_code/commands/, doctor
# reports plugin_missing.
```

## Test results

- `tests/test_plugin_install.py`: 14 / 14 pass.
- `make lint`: clean.
- `make typecheck`: clean (88 source files).
- Full suite: pending.

## Out of scope (Steps 2-3 / future)

- `codex` profile (`.codex-plugin/plugin.json` + 5 skill templates
  copied + Codex-specific commands).
- `gemini` profile promotion (currently `generic`-shape; Step 3
  splits into the same five-skill structure).
- `--profile all` aggregation.
- Cross-target install ("install one bundle into many target repos").
- Hosted marketplace.

## Acceptance summary

| Gate | Verified |
| --- | --- |
| claude_code install writes 14 files | `test_claude_code_install_writes_full_bundle` |
| Idempotent re-install | `test_claude_code_idempotent_re_install` |
| Dry-run writes nothing | `test_claude_code_dry_run_writes_nothing` |
| Edit-detect refuses overwrite | `test_claude_code_edit_detect_refuses_overwrite` |
| --force overwrites | `test_claude_code_force_overwrites` |
| No external URLs (F3) | `test_claude_code_no_external_urls` |
| Marketplace appended | `test_claude_code_marketplace_appended` |
| Marketplace idempotent | `test_claude_code_marketplace_idempotent` |
| Uninstall removes tracked files | `test_uninstall_removes_tracked_files` |
| Uninstall refuses modified | `test_uninstall_refuses_modified_without_force` |
| Uninstall force | `test_uninstall_force_removes_modified` |
| Skill templates match (F1) | `test_skill_templates_match_skills_module` |
| Unknown profile errors | `test_unknown_profile_raises` |
| Unknown scope errors | `test_unknown_scope_raises` |
