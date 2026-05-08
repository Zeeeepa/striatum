---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

# RFC 0015 V1 Build Handoff

author: implementer-codex-gpt-5.5-001

Date: 2026-05-08
Run: dogfood-009 / RFC 0015 (Self-Contained Agent Skills)
Decision: `accepted_with_follow_up` (V1_ACCEPTANCE; autonomous)
Version: `0.5.0`

V1 build slice (RFC 0015 steps 1+2) ships in one commit. Step 3
(`codex` + `gemini` profiles, parser-walked verb table,
`--profile all`) deferred per the RFC's implementation path.

## Files Changed

- **`src/striatum/skills/__init__.py`** (new) — re-exports the
  public surface (`install`, `gather_template_context`,
  `manifest_path_for`, `load_manifest`, `skill_files_present`,
  `bundled_template_sha256`, `MANIFEST_SCHEMA_VERSION`).
- **`src/striatum/skills/install.py`** (new, ~340 lines) — install
  pipeline. Resolves project- vs user-scope target root; renders
  packaged templates against the curated context; computes
  rendered + bundled-template SHA256; compares on-disk content
  against the manifest's recorded hash to drive
  `written` / `skipped_unchanged` / `refused_modified` / `dry_run`
  decisions; writes a sorted manifest at the end. ANSI-aware
  `_StrictFormatMap` raises `KeyError` for unknown placeholders so
  template typos surface as `NotFoundError` rather than silent
  passthrough. Public helpers `manifest_path_for`,
  `load_manifest`, `skill_files_present`, and
  `bundled_template_sha256` power the doctor checks.
- **`src/striatum/skills/context.py`** (new) — curated verb table
  (RFC research artifact's "Smallest set of CLI verbs"), boundary
  statements (the runner's "what not to do" enforced-by-construction
  rules), and `gather_template_context()` which returns
  `{striatum_version, verbs, boundaries, front_matter_kinds}`.
- **`src/striatum/skills/templates/claude_code/{workflow,scaffold,
  claim-loop,supervise,recover}.md.tmpl`** (new) — five Claude Code
  skill templates with frontmatter (`name`, `description`,
  `version`, `striatum_min_version`, `profile`), When-to-invoke /
  Authoritative-reference / Common-patterns / Where-to-look-next /
  What-not-to-do sections.
- **`src/striatum/skills/templates/generic/STRIATUM_AGENT_GUIDE.md.tmpl`**
  (new) — single concatenated guide for the generic profile.
- **`src/striatum/cli/parser.py`** — adds `striatum init
  --with-skills [profile]` (nargs="?", default `claude_code` when
  flag is present without a value) and the `striatum skills
  install` subcommand with `--profile`, `--scope`, `--namespace`,
  `--force`, `--dry-run`, `--json`.
- **`src/striatum/cli/dispatch.py`** — dispatches `skills install`
  to `install(...)`; runs `install(...)` after `init_repo(repo)`
  when `--with-skills` is set; result envelope from `init` gains
  a `skills` key when the flag fires.
- **`src/striatum/cli/introspect.py`** — registers
  `skills_missing` / `skills_outdated` in `DOCTOR_CHECKS` and adds
  `_check_skill_bundle(repo, report)` which is called near the end
  of `doctor(...)`. Both checks emit a `recovery_command` (the
  exact `striatum skills install --profile <p>` that would clear
  the condition); the runner never auto-regenerates.
- **`pyproject.toml`** — version 0.4.0 → 0.5.0; new
  `[tool.setuptools.package-data]` entry for
  `striatum.skills.templates`.
- **`src/striatum/__init__.py`** — `__version__` 0.1.0 → 0.5.0
  (the source version had drifted from the pyproject value;
  brought back into lockstep).
- **`tests/test_skills_install.py`** (new, 16 cases) — covers
  every RFC acceptance criterion plus the manifest-self-exclusion
  guard and the bundled-vs-recorded template SHA parity check.
- **`docs/SPEC.md`** — adds the "Self-Contained Agent Skills (RFC
  0015 V1)" subsection.
- **`docs/UBIQUITOUS_LANGUAGE.md`** — adds "skill bundle" + "skills
  manifest".
- **`docs/rfcs/0015-self-contained-agent-skills.md`** — status
  flips to `accepted (V1)`.
- **`docs/rfcs/README.md`** — index reflects `accepted (V1)` plus
  the D061 reference.
- **`docs/DECISION_LOG.md`** — D061 row.
- **`docs/TODO.md`** — F9 row.
- **`README.md`** — § "Initialize Runner State" gains the
  `init --with-skills` and `skills install` pointers.
- **`CHANGELOG.md`** — `0.5.0` section.

## Verification

- `make lint` — clean.
- `make typecheck` — clean (50 source files).
- `make test` — 260 passed (16 new skills tests on top of 244
  baseline).
- Smoke-tested `skills install` end-to-end against `/tmp/rfc-0015-test`:
  fresh install writes five SKILL.md files plus `.manifest.json`;
  re-install reports `skipped_unchanged` for each file; generic
  profile writes one guide and one manifest at the target root.

## Notes For The Reviewer

- **Curated verb table over parser-walked.** The synthesis pinned
  this trade-off: ~25 stable verbs, no need for argparse-tree
  walking. The verb table lives in `striatum/skills/context.py`
  alongside the boundary statements and is rendered into templates
  by `_expand_helpers` *before* `str.format_map`. Promoting to
  parser-walked is a V1.5 follow-up that will change `context.py`
  alone.
- **Template helper expansion order.** Helpers that emit
  Markdown (verb tables, bullet lists) are expanded with literal
  `str.replace` *before* `format_map` so the formatter only sees
  `{striatum_version}`. This keeps the template grammar trivial.
- **Manifest excludes itself.** A manifest's `files[]` list never
  contains the manifest path; otherwise the manifest's own SHA
  would invalidate itself on every write.
- **`_StrictFormatMap`** subclasses `dict[str, Any]` and overrides
  `__missing__`. `str.format_map` swallows missing keys for plain
  custom mappings; subclassing `dict` is the only way to keep
  unknown placeholders loud.
- **Doctor: lazy.** `_check_skill_bundle` only fires when a
  manifest exists. A target with no bundle silently passes (the
  RFC explicitly forbids prompting; the operator opts in via
  `init --with-skills` or `skills install`).
- **Step 3 deferred.** The parser's `--profile` choices are the
  V1 set `{claude_code, generic}` exactly. Adding `codex` /
  `gemini` is a follow-up commit that touches
  `parser.py`, `install.ALLOWED_PROFILES`, the templates dir, and
  one or two test cases — no schema changes required.
