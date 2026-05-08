---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

# RFC 0015 V1 — Self-Contained Agent Skills Research

author: researcher-codex-gpt-5.5-001

Date: 2026-05-08

## Existing surfaces

- **CLI parser** lives at `src/striatum/cli/parser.py`. `build_parser()`
  registers ~25 top-level subcommands; subcommand groups (`workflow`,
  `run`, `branch`, `session`, `recovery`, `worktree`, `supervise`,
  `adapter`, `checkpoint`, `decision`, `evidence`, `list`, `serve`)
  expose their own subparsers. Templates can call `build_parser()`
  and walk the resulting tree to enumerate verbs and flags without
  hard-coding them.
- **Public API entry** is `striatum.api.invoke(args, stdout)`.
  `striatum.cli.main` is its CLI shim. Skills should instruct agents
  to call the CLI verbs; skills do not need to embed the API
  signature.
- **Version** is `striatum.__version__` (sourced from
  `pyproject.toml`'s `[project] version`). Templates pin the
  generator's version into each rendered file's header.
- **Front-matter schemas** live at
  `src/striatum/artifacts.py:ALLOWED_ARTIFACT_KINDS` plus the
  schema registry (decision/finding/findings_ledger/synthesis/
  support_ledger/action_item_ledger/harness_improvement_proposal).
  Skills should reference the *kinds* that workflow authors will
  see, not embed the JSON schemas.
- **RFC 0010 profile entries** declare per-tool conventions
  (`subagents`, `agent_teams`, `skills`, `custom_agent_roles`).
  Path layouts derive from these entries; the V1 generator uses
  hard-coded layouts for `claude_code` and `generic`, with
  `codex` / `gemini` as deferred profiles.
- **Package data**. `pyproject.toml` already ships a
  `[tool.setuptools.package-data]` block for
  `src/striatum/web/static/`. The V1 generator adds a sibling
  entry for `src/striatum/skills/templates/` so the wheel ships
  with the templates.

## Smallest set of CLI verbs to cover (five-skill bundle)

Per the RFC § 3 table:

| Skill | Verbs |
|---|---|
| `striatum-workflow` | (router; cites the others) |
| `striatum-scaffold` | `init`, `workflow init`, `workflow validate`, `run prepare`, `run start`, `branch confirm` |
| `striatum-claim-loop` | `register-session`, `claim-next`, `ack`, `heartbeat`, `publish-artifact`, `verdict`, `submit-review`, `complete`, `worktree create`, `worktree release` |
| `striatum-supervise` | `supervise start | send | stop | status | list` |
| `striatum-recover` | `status`, `why`, `doctor --verbose`, `recovery stale-leases`, `recovery requeue-stale`, `recovery process-reconcile`, `checkpoint resolve`, `dashboard --once` |

Each command surface is verifiable from the parser tree without a
network round trip. Templates should embed *only* the verb shape
plus a one-line summary; the agent runs `striatum <verb> --help`
for the full flag set.

## Path layouts (V1)

- **`claude_code`**: `.claude/skills/<namespace>striatum-<skill>/SKILL.md`.
  One directory per skill. Manifest at
  `.claude/skills/<namespace>striatum-workflow/.manifest.json` (the
  router skill owns the manifest; this matches the RFC's "single
  source of truth" claim because the router is required for the
  bundle to be discoverable).
- **`generic`**: single Markdown file at
  `<namespace>STRIATUM_AGENT_GUIDE.md` at the target root. Contains
  all five skills concatenated with `## ` headings. Manifest at
  `<namespace>STRIATUM_AGENT_GUIDE.manifest.json` next to it.

`--scope user` rewrites the prefix to `~/.claude/skills/...` for
`claude_code`; for `generic`, `--scope user` writes to
`~/<namespace>STRIATUM_AGENT_GUIDE.md` (and the matching manifest)
because there is no skill-discovery convention in the generic case.

## Manifest schema (V1)

```json
{
  "schema_version": "striatum.skills.manifest.v1",
  "striatum_version": "0.4.0",
  "generated_at": "2026-05-08T19:00:00Z",
  "profile": "claude_code",
  "namespace": "striatum-",
  "scope": "project",
  "files": [
    {
      "path": ".claude/skills/striatum-workflow/SKILL.md",
      "sha256": "<hex>",
      "template": "claude_code/striatum-workflow.md.tmpl"
    }
  ]
}
```

`striatum_version` is `striatum.__version__`. `generated_at` is UTC
ISO-8601. `scope` is `project` or `user`. The `files` list is
sorted by `path` for byte-identical regeneration.

## Edit-detection rule

For each entry in `files`, the generator computes the SHA256 of the
on-disk file and compares it to `sha256`. If they differ:

- Without `--force`: the file is skipped, the result is recorded as
  `refused` in the install summary, and the manifest is left
  unchanged.
- With `--force`: the file is rewritten and the manifest entry's
  `sha256` is updated.
- With `--dry-run`: the conflict is logged; nothing is written.

A file present on disk but absent from the manifest is treated as
new and is written without prompting (a `--force` is not required to
add new files).

## Idempotency

A second `skills install` against an unchanged target writes
byte-identical files. To make this true:

- Templates embed `striatum_version` only; never `generated_at`
  inside the file body. (`generated_at` lives in the manifest only.)
- The manifest `files` list is sorted by `path`.
- The header line uses `striatum.__version__` exactly so two installs
  by the same runner version produce the same byte sequence.

## Doctor checks (V1)

- `skills_missing` — manifest exists, but at least one file in
  `files[].path` is absent.
- `skills_outdated` — manifest's `striatum_version` is older than
  `striatum.__version__`, OR a packaged template's bundled SHA256
  differs from the manifest entry's recorded template SHA256
  (catches template churn within a version bump).

Doctor never auto-regenerates; it surfaces the exact
`striatum skills install` invocation that would resolve the
condition.

## `--with-skills` wiring

`striatum init [--with-skills [profile]]` runs `init` first, then
calls the same code path as `skills install`. Default profile when
the flag has no value: `claude_code`. The flag is opt-in to
preserve the current `init` surface byte-for-byte.

## No-external-URL invariant

A unit test walks every rendered file's text after a fresh install
and asserts:

- No `http://` / `https://` substrings.
- No path tokens outside the bundle's namespace directory (we
  permit the verb names and the literal string `.striatum/state.sqlite3`
  in "What not to do" sections, both validated as part of the same
  test).

## Recommended order

1. Add `src/striatum/skills/__init__.py` + `templates/claude_code/`
   directory with five `*.md.tmpl` files + the renderer
   (`render_template`, `install`).
2. Add `striatum skills install` parser entry + dispatch.
3. Add `claude_code` profile output + manifest writer.
4. Add `generic` profile output (concat + manifest).
5. Add `striatum init --with-skills`.
6. Add doctor `skills_missing` / `skills_outdated`.
7. Tests at `tests/test_skills_install.py`.

## Friction anticipated

- **Template variable scope.** Templates need a small set of
  values: `striatum_version`, the verb table generated from the
  parser, and the front-matter kinds. A simple `str.format_map()`
  with a curated namespace is sufficient; we do not need a full
  templating engine.
- **Manifest hashing for templates.** The "template SHA256" used by
  `skills_outdated` should be computed over the *unrendered*
  template file as it ships in the wheel, not the rendered output.
  This keeps the comparison cheap and makes "the template churned"
  vs "the runner version churned" distinguishable.
- **`--scope user` for `generic`.** The generic profile has no
  skill-discovery convention, so `--scope user` writes a single
  guide to `~/<namespace>STRIATUM_AGENT_GUIDE.md`. We accept that
  this is not auto-discoverable and document it as such in the
  generic guide's "What not to do" section.
