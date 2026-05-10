---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# RFC 0025 V1 Step 1 design

author: designer-claude-opus-001

## Scope

V1 Step 1 ships the **claude_code plugin profile** end-to-end plus
the shared infrastructure (`striatum plugin` CLI, doctor checks,
`init --with-plugins`). Steps 2 (codex) and 3 (gemini) are deferred
to follow-up dogfoods.

## Module structure

```
src/striatum/plugins/
├── __init__.py
├── install.py        # mirror of skills/install.py
└── templates/
    └── claude_code/
        ├── plugin.json.tmpl
        ├── README.md.tmpl
        ├── mcp.json.tmpl
        ├── hooks/hooks.json.tmpl
        ├── commands/{claim-next,status,why,dashboard,doctor}.md.tmpl
        └── skills/{workflow,scaffold,claim-loop,supervise,recover}.md.tmpl
```

The five skill templates are byte-identical to the existing
`src/striatum/skills/templates/claude_code/{name}.md.tmpl` —
duplicate them rather than symlink (importlib.resources doesn't
follow symlinks reliably across packaging tools). This is honest
content duplication; a future RFC can de-dupe via shared template.

## Manifest

`MANIFEST_SCHEMA_VERSION = "striatum.plugins.manifest.v1"` (note the
plural `plugins`). Same shape as RFC 0015 with one delta: the
manifest lives **inside the bundle directory** (`<bundle>/.manifest.json`),
not at the target's `.striatum/skills.manifest.json`. This keeps
each bundle self-describing — uninstalling means deleting one
directory.

## CLI surface

```
striatum plugin install [--target <path>] [--profile claude_code]
                        [--scope project|user] [--namespace striatum]
                        [--force] [--dry-run] [--with-marketplace]
                        [--json]
striatum plugin uninstall --profile <id> [--target <path>]
                          [--scope project|user] [--namespace <ns>]
                          [--force] [--json]
```

Defaults: `profile=claude_code`, `scope=project`, `namespace=striatum`,
`with-marketplace=True`. The `uninstall` action reads the bundle's
`.manifest.json` and deletes only manifest-tracked files, refusing
to delete operator-edited ones without `--force`.

## `init --with-plugins`

```
striatum init --with-plugins [<profile>]
```

`nargs="?"`, defaults to `claude_code` when bare. Calls
`plugins.install.install(...)` after `.striatum/` is created.
Independent of `--with-skills`.

## Doctor checks

- **plugin_missing** — manifest exists at `.striatum/plugins/manifest_index.json`
  (a small index referencing each installed bundle) but the bundle
  dir is gone. Surfaces `striatum plugin install --profile <id>`.
- **plugin_outdated** — bundle's `.manifest.json` records a
  `striatum_version` older than `striatum.__version__`. Surfaces
  `striatum plugin install --profile <id>` (which is idempotent
  + version-stamped).

The `manifest_index.json` is a small JSON file at
`.striatum/plugins/manifest_index.json` listing each installed
bundle by profile so doctor can find them without walking. Updated
on every install/uninstall.

## Marketplace fixture

`.striatum/plugins/marketplace.json` (single shared file). For
Step 1 / claude_code only, contains:

```json
{
  "name": "local-striatum",
  "interface": {"displayName": "Local Striatum"},
  "plugins": [
    {
      "name": "striatum",
      "source": {"source": "local", "path": "./claude_code"},
      "policy": {"installation": "AVAILABLE"},
      "category": "Developer Tools"
    }
  ]
}
```

Default-on for `--with-marketplace`. Edit-detect via the
manifest_index (the file's content is regenerated; operator edits
inside an entry are preserved by reading the existing file and
merging).

## Slash command shapes

Each `commands/<cmd>.md` is Markdown with YAML frontmatter (Claude
Code spec). Body is one or two lines describing the wrapped CLI
verb.

```markdown
---
description: Claim the next eligible work packet
---

Run `striatum claim-next --session-id $SESSION_ID --json` and read
the returned packet.
```

## URL-leak test

A unit test:
1. Renders every template against a deterministic context.
2. Walks rendered text for these patterns:
   - `https?://` → fail
   - `git://` → fail
   - `file://` → fail
   - The literal source-repo path → fail
   - The operator's home directory path → fail
3. Allows: `./` and `../` relative paths only when they target
   inside the bundle.

## Test plan

| Test | Coverage |
| --- | --- |
| `test_plugin_install_creates_full_bundle` | 13 files written |
| `test_plugin_install_idempotent` | 2nd install is byte-identical |
| `test_plugin_install_dry_run` | No writes, plan returned |
| `test_plugin_install_force_overwrites_edited` | `--force` honors |
| `test_plugin_install_refuses_modified` | edit-detect refuses |
| `test_plugin_install_no_external_urls` | URL-leak invariant |
| `test_plugin_install_marketplace_appends` | reentrant marketplace.json |
| `test_plugin_uninstall_removes_tracked_files` | manifest-driven delete |
| `test_doctor_plugin_missing` / `_outdated` | doctor surfaces fix |
| `test_init_with_plugins` | combined init flow |

## Out of scope (Steps 2-3 / V2)

- `codex` profile (templates + CLI flag).
- `gemini` profile (promote from generic).
- Plugin uninstallation through agent CLI's own command.
- Cross-target install (one bundle, many target repos).
- Hosted marketplace.

## Migration

No DB migration needed. All artifacts are filesystem-only.
