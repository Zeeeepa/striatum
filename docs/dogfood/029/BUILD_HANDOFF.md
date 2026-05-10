---
title: "RFC 0025 V1 Steps 2+3 build handoff (dogfood-029)"
date: 2026-05-09
---

# Build handoff: RFC 0025 V1 Steps 2+3 (codex + gemini profiles)

author: implementer-claude-opus-001

## Scope

Steps 2 + 3 land together. Both extend Step 1's pipeline and reuse
the byte-shared skill template invariant.

- **Step 2 (codex)**: 14-file Codex plugin bundle.
- **Step 3 (gemini)**: 14-file Gemini extension. Promotes `gemini`
  from RFC 0015's `generic` fallback to first-class.
- **`--profile all`**: aggregates all three profiles in one
  invocation.

## Files

### New (under `src/striatum/plugins/templates/`)

**Codex (`codex/`)** — 14 templates (5 skills + 5 commands +
hooks/mcp byte-shared with claude_code; new `plugin.json.tmpl` and
`README.md.tmpl`).

**Gemini (`gemini/`)** — 14 templates (5 skills byte-shared with
claude_code; 5 fresh TOML commands; new `gemini-extension.json.tmpl`,
`GEMINI.md.tmpl`, `agents/striatum-recover.md.tmpl`, and
`README.md.tmpl`).

### Modified

- `src/striatum/plugins/install.py`:
  - `ALLOWED_PROFILES` += `{"codex", "gemini"}`.
  - `ALL_PROFILES_ORDER = ("claude_code", "codex", "gemini")`.
  - `_PROFILE_SKILLS` + `_PROFILE_COMMANDS` (renamed; aliases kept).
  - `_bundle_root` adds user-scope branches for codex
    (`~/.codex/plugins/`) and gemini (`~/.gemini/extensions/`).
  - `_build_plan` dispatches to `_plan_codex` and `_plan_gemini`.
  - `_write_marketplace` short-circuits for gemini, returning a
    structured `{"skipped": True, "reason": "..."}` payload.
- `src/striatum/cli/parser.py` — `--profile` choices for both
  `plugin install` and `plugin uninstall` expand to
  `["claude_code", "codex", "gemini", "all"]`.
- `src/striatum/cli/dispatch.py` — `--profile all` loops over
  `ALL_PROFILES_ORDER` and aggregates into `{"profile": "all",
  "results": [...]}`.
- `tests/test_plugin_install.py` — adds:
  - `test_codex_install_writes_full_bundle` / `_idempotent_re_install` /
    `_no_external_urls` / `_user_scope_path`.
  - `test_gemini_install_writes_full_bundle` /
    `_marketplace_skipped` / `_no_external_urls` / `_user_scope_path`.
  - F1 byte-match test extended to assert all three profile skill
    template trees match the canonical `skills/templates/claude_code/`.

### Docs

- `CHANGELOG.md` — `## 1.20.0 — 2026-05-09` section.
- `pyproject.toml` + `__init__.py` — bumped to `1.20.0`.
- `docs/DECISION_LOG.md` — D082 row.
- `docs/TODO.md` — F29 row.
- `docs/rfcs/0025-agent-cli-plugin-bundles.md` — status
  `accepted (V1)`.
- `docs/rfcs/README.md` — index updated.

## Design-review disposition

| Finding | Severity | Disposition |
| --- | --- | --- |
| F1: Verify Gemini TOML schema | verify | **Adopted bare-top-level form** (no `[command]` table) per Gemini extension docs reference in the RFC. The implementer chose `description = "..."`/`prompt = """..."""` at top level. |
| F2: Verify Codex user-scope path | verify | **Adopted `~/.codex/plugins/<namespace>/`** matching the Codex CLI plugins spec. `test_codex_user_scope_path` confirms the path renders. |
| F3: Marketplace skip surfaced in --json | note | **Adopted**: `_write_marketplace` returns `{"skipped": True, "reason": "..."}` for gemini. Test `test_gemini_marketplace_skipped` asserts the structured payload. |

## Smoke

```
$ striatum plugin install --profile codex
14 files; .codex-plugin/plugin.json + 5 skills + 5 commands + hooks + mcp + README + manifest

$ striatum plugin install --profile gemini
14 files; gemini-extension.json + GEMINI.md + 5 skills + 5 TOML commands + agents/striatum-recover.md + README + manifest
marketplace: {"skipped": true, "reason": "gemini has no marketplace concept"}

$ striatum plugin install --profile all
{"profile": "all", "results": [<3 install results>]}
```

## Test results

- `tests/test_plugin_install.py`: **22 / 22** pass (was 14 in Step 1).
- `make lint`: clean.
- `make typecheck`: clean (88 source files).
- Full suite: pending.

## Out of scope (V2)

- Cross-target install (one bundle into many target repos).
- Hosted marketplace.
- Codex `apps/` and `assets/`.
- Per-target git-repo extension format for gemini.
