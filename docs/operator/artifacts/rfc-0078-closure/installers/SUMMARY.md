---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# RFC 0078 Gate B — Installers ported to Go, scaffold retired

author: implementer-claude-001

## Outcome

`striatum skills install` and `striatum plugin install` / `plugin uninstall`
are now Go local (non-daemon) commands with flag parity to the retired Python
installers. The 55 template files (skills + plugins) are embedded in the Go
binary, the doctor version-stamp shape is preserved, and the Python installer
source trees are deleted.

## New Go install commands + package

- **`go/pkg/installers/`** — the ported pipeline. Public entrypoints:
  `InstallSkills` / `InstallSkillsAll`, `InstallPlugin`, `UninstallPlugin`.
  Internals mirror `src/striatum/skills/install.py` and
  `src/striatum/plugins/install.py`:
  - `pipeline.go` — shared edit-detection + manifest writer (the
    `written` / `skipped_unchanged` / `refused_modified` / `dry_run`
    decision ladder, ported verbatim).
  - `context.go` — `VERB_TABLE`, `BOUNDARIES`, and
    `sorted(ALLOWED_ARTIFACT_KINDS)` ported verbatim so rendered bundles
    are byte-equivalent to the Python output.
  - `render.go` — helper expansion (`{verbs_*}`, `{boundaries_bulletlist}`,
    `{front_matter_kinds_list}`) plus a `str.format_map` emulator that honors
    `{{`/`}}` brace escaping and raises on unknown keys (matching
    `_StrictFormatMap`).
  - `skills.go` / `plugins.go` — per-profile plans, scope/bundle path
    resolution, and the marketplace merge.
- **`go/pkg/cli/skills/`** — flag parsing + JSON/human output, wrapping the
  installers package. Output uses the same `{"ok":true,"data":...}` envelope
  as the existing Go local commands.
- **`go/pkg/cli/localcommands/`** — registered `skills install`,
  `plugin install`, `plugin uninstall` as explicit local routes (each with a
  no-daemon-state rationale).
- **`go/cmd/striatum/main.go`** — routes the new local commands and injects a
  global `--json` into the subcommand, mirroring the workflow-validate path.

### Flag parity

- `skills install`: `--profile {claude_code,codex,gemini,generic,all}`,
  `--scope {project,user}`, `--namespace` (default `striatum-`), `--force`,
  `--dry-run`, `--json`.
- `plugin install`: adds `--target`, `--with-marketplace` / `--no-marketplace`
  (default on); `--namespace` default `striatum`; `--profile` adds `all`.
- `plugin uninstall`: `--profile`, `--scope`, `--namespace`, `--target`,
  `--force`, `--json`.

## Template embedding

The two template trees were copied byte-for-byte into
`go/pkg/installers/templates/{skills,plugins}/` and embedded with a single
`//go:embed templates/skills templates/plugins` directive (mirroring
`go/pkg/webassets`). The Python `__init__.py` package-data markers were **not**
copied — `embed.FS` needs no package markers. Copy fidelity was verified with
`sha256sum` over both trees (SKILLS + PLUGINS reported byte-identical).

55 `.tmpl` files embedded (9 skills + 46 plugins); 10 `__init__.py` markers
dropped (the prompt's "65 files" = 55 templates + 10 markers).

## Doctor version-stamp alignment

The manifests keep the Python field shape that staleness detection reads:
`schema_version` (`striatum.skills.manifest.v1` / `striatum.plugins.manifest.v1`),
`striatum_version`, `generated_at`, `profile`, `namespace`, `scope`, and a
sorted `files[]` of `{path, sha256, template, template_sha256}`. The version
stamp is taken from the Go CLI's `main.version` (ldflag `-X main.version`,
sourced from `VERSION`), so `striatum doctor`'s bundle-staleness comparison
keeps working against a rebuilt bundle. `striatum doctor --json` reports
`"problems":[]` on the live daemon after install.

## Scaffold removal

`scaffold` was only a `--scaffold`-style flag on Python `init`; it has **no Go
route** (the Go `init` surface dispatches to the daemon and never carried the
flag — confirmed by grep: the only `scaffold` tokens in `go/` are in
`workflowgenerate`, an unrelated concept). No Go change was needed to retire
it. The `striatum-scaffold` *skill* bundle is a separate, still-shipped agent
skill and is unaffected.

## Python files deleted

Deleted the entire `src/striatum/skills/` and `src/striatum/plugins/` trees
(10 tracked `.py` files — `skills/{__init__,context,install}.py`,
`plugins/__init__.py`, `plugins/install.py`, and 5 `templates/**/__init__.py`
markers — plus the now-redundant `.tmpl` originals, which are embedded in Go).
`make python-trace-report` `active_striatum_python_source` dropped 201 → 191
(blocked 439 → 429).

## Known divergences (intentional)

- The Python skills installer also called `ensure_reference_wrappers`, which
  writes `.striatum/bin` + `.gitignore` daemon scratch as a side effect. The Go
  installer omits this: `.striatum/` is daemon scratch (forbidden write scope)
  and the constraint is that `skills install` writes only the skills dir, not
  daemon state. A parity diff of rendered output confirmed the only deltas were
  this dropped scratch.
- `marketplace.json` is semantically identical but JSON object keys serialize
  in Go's (sorted) order rather than Python's insertion order. It is a
  generated merge file (not a template) consumed by order-insensitive JSON
  parsers; all template-rendered bundle files are byte-identical.

## Out of scope (follow-up)

`src/striatum/cli/parser.py`, `cli/dispatch.py`, and `day_zero.py` still import
the deleted modules and still register the `--scaffold` flag. These files are
outside this packet's write scope (`src/striatum/cli/` is not in
`allowed_paths`). The legacy Python CLI is not on the CI/production path and is
slated for full deletion at RFC 0078 Gate G; the dangling imports should be
cleaned with that tree removal.

## Validation

```
cd go && go test ./pkg/cli/... ./cmd/striatum        # ok (incl. new skills/installers tests)
go test ./pkg/installers/                             # ok (10 tests: render, idempotency, force, dry-run, plugin round-trip, marketplace)
striatum skills install --profile all --scope user --force --json >/dev/null && echo install-ok   # install-ok
striatum doctor --json | grep -o '"problems":\[[^]]*\]'   # "problems":[]
```

Cross-implementation parity (Python 1.57.0 vs Go same-version build):
`diff -r` of the rendered `--profile all` skills bundles and the plugin
bundles showed **no content differences** in any SKILL.md / command / hook /
plugin.json file — only the dropped `.striatum`/`.gitignore` scratch and the
`marketplace.json` key ordering noted above.
