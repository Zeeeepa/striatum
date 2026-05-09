---
title: "RFC 0025 V1 Step 1 shape research: claude_code plugin"
date: 2026-05-09
---

# Plugin install shape research

author: researcher-codex-gpt-5.5-001

## (1) RFC 0015 install pipeline (`skills/install.py`)

The skills pipeline is a clean four-stage flow:

1. **Plan**: `_build_plan(profile, namespace, base, context)` returns
   a list of `{path, rendered, template, template_sha256}` dicts.
2. **Compare**: each entry's rendered SHA against on-disk SHA + the
   `.manifest.json` recorded SHA; classify as `written`,
   `skipped_unchanged`, `refused_modified`, or `dry_run`.
3. **Write**: for `written`, atomically write the rendered bytes.
4. **Manifest**: re-serialize `.manifest.json` with the post-state.

The pattern is generic. Reusable for plugins with no architectural
change — the only differences are template paths, output paths, and
the manifest filename.

## (2) Reusable pieces

| Piece | Reusable? |
| --- | --- |
| Plan/render/compare loop | Yes — copy verbatim. |
| `_sha256` helper | Yes — same algorithm, same path. |
| `gather_template_context` | Yes — `striatum_version`, etc. |
| Manifest schema (`schema_version`, `files[]`) | Yes — bump `schema_version` to `striatum.plugins.manifest.v1`. |
| Edit-detect rules | Yes — same exact rules. |
| `--force`, `--dry-run` semantics | Yes. |
| Profile dispatch | Yes — but plugins start with single profile (`claude_code`); codex/gemini in Steps 2-3. |

## (3) New module: `src/striatum/plugins/install.py`

Mirror of `skills/install.py` but smaller initial surface:

```python
MANIFEST_SCHEMA_VERSION = "striatum.plugins.manifest.v1"
ALLOWED_PROFILES: frozenset[str] = frozenset({"claude_code"})  # Step 2-3 add codex, gemini
ALLOWED_SCOPES: frozenset[str] = frozenset({"project", "user"})

def install(*, target, profile, scope="project", namespace="striatum",
            force=False, dry_run=False, with_marketplace=True, home=None):
    """Render the plugin bundle for `profile` into `target`."""
```

The `namespace` default for plugins is `"striatum"` (no trailing
dash) because plugin manifests use it as a literal name field.

## (4) Bundle layout (claude_code)

Per RFC 0025 § 2.1, project scope writes under
`.striatum/plugins/claude_code/`:

```
.striatum/plugins/claude_code/
├── .claude-plugin/plugin.json
├── skills/
│   ├── striatum-workflow/SKILL.md
│   ├── striatum-scaffold/SKILL.md
│   ├── striatum-claim-loop/SKILL.md
│   ├── striatum-supervise/SKILL.md
│   └── striatum-recover/SKILL.md
├── commands/
│   ├── claim-next.md
│   ├── status.md
│   ├── why.md
│   ├── dashboard.md
│   └── doctor.md
├── hooks/hooks.json
├── .mcp.json
├── .manifest.json
└── README.md
```

Total: **13 files** (manifest + 1 plugin.json + 5 skills + 5 commands +
hooks + mcp + README).

User scope (`--scope user`) writes under
`~/.claude/plugins/striatum/`. The manifest sits at the bundle root
either way, NOT at `.striatum/skills.manifest.json` like RFC 0015 —
plugins ship one `.manifest.json` per bundle directory.

## (5) Marketplace fixture

For `claude_code` (and later `codex`), `--with-marketplace` writes
`.striatum/plugins/marketplace.json` (one file shared across
profiles). Format per RFC § 4. V1 single-profile case: the file
contains a single `plugins[]` entry pointing at `./claude_code`.

If the file already exists, the merge logic must:
- Add the profile entry if absent.
- Update the entry's version field if present and version moves.
- Preserve other entries unchanged.
- Apply edit-detect: refuse to overwrite operator edits without
  `--force`.

## (6) Templates to author

| File | Template | Source |
| --- | --- | --- |
| `.claude-plugin/plugin.json` | `claude_code/plugin.json.tmpl` | New (small JSON). |
| `skills/striatum-{NAME}/SKILL.md` × 5 | `claude_code/skills/{name}.md.tmpl` | **Reuse from `skills/templates/claude_code/{name}.md.tmpl`** byte-identical. |
| `commands/{cmd}.md` × 5 | `claude_code/commands/{cmd}.md.tmpl` | New (each is a thin imperative wrapper). |
| `hooks/hooks.json` | `claude_code/hooks/hooks.json.tmpl` | New (commented stubs). |
| `.mcp.json` | `claude_code/mcp.json.tmpl` | New (empty `{}`). |
| `README.md` | `claude_code/README.md.tmpl` | New (install instructions). |

## (7) CLI surface

```
striatum plugin install --profile claude_code [--target] [--scope] [--namespace]
                        [--force] [--dry-run] [--with-marketplace] [--json]
striatum plugin uninstall --profile claude_code [--target] [--scope] [--namespace] [--force] [--json]
```

Mirror of `striatum skills install/uninstall` exactly.

## (8) Doctor checks

Two new checks added to `striatum doctor`:

- **plugin_missing** — manifest exists but bundle directory is gone
  (operator deleted `.striatum/plugins/claude_code/`).
- **plugin_outdated** — manifest's `striatum_version` < current
  `__version__`.

Both surface the exact `striatum plugin install --profile X` command
to fix. Doctor never auto-regenerates.

## (9) `init --with-plugins`

`striatum init --with-plugins [profile]` mirror of `--with-skills`.
Calls `plugins.install.install(...)` after `.striatum/` is created.
Argparse: `--with-plugins` is `nargs="?"` with default `claude_code`.

## (10) URL-leak invariant

A test walks every rendered template output and asserts:
- No `http://`, `https://`, `git://`, `file://`, or other scheme.
- No path containing the source-repo Python package root.
- No path containing the operator's home directory.

Allowable: relative paths inside the bundle (`./skills/`,
`./commands/...`).

## (11) Step scope

| Step | Profile | Dogfood |
| --- | --- | --- |
| **Step 1** (this dogfood) | `claude_code` | dogfood-028 |
| Step 2 | `codex` | dogfood-029 (future) |
| Step 3 | `gemini` (promoted from generic) | dogfood-030 (future) |

Each step is independently mergeable: shared infrastructure
(`plugins/install.py`, CLI, doctor, `init --with-plugins`) lands in
Step 1, profiles plug into the existing dispatch.

## (12) Test plan

- `tests/test_plugin_install.py` — full claude_code bundle renders
  the expected 13 files.
- `test_plugin_install_idempotent` — second install is byte-identical.
- `test_plugin_install_edit_detect` — operator-modified file
  preserved; `--force` overwrites.
- `test_plugin_install_dry_run` — plan returned, no writes.
- `test_plugin_install_no_external_urls` — walks rendered output.
- `test_plugin_install_marketplace_append` — second profile entry
  appended cleanly.
- `test_doctor_plugin_missing` / `test_doctor_plugin_outdated`.
- `test_init_with_plugins` — combined init flow.

## Out of scope (Steps 2-3)

- Codex profile (`.codex-plugin/plugin.json`).
- Gemini profile (promote from generic).
- Templates for codex/gemini.
