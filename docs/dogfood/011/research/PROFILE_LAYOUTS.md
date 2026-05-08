---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

# RFC 0015 step 3 — codex + gemini profile layouts

author: researcher-codex-gpt-5.5-001

Date: 2026-05-08

## Codex CLI conventions

The OpenAI Codex CLI ships with a `~/.codex/AGENTS.md` mechanism
and per-project `AGENTS.md` files. RFC 0015 § "Profile coverage"
says: *"`codex` — writes to `.codex/agents/<namespace>*.md` using
the custom-agent-role idiom captured in RFC 0010."* That reads
the convention as five files under a directory, one per skill.

Concrete V1 plan: lift the five `claude_code` skill bodies into
`.codex/agents/<namespace>workflow.md`,
`<namespace>scaffold.md`, `<namespace>claim-loop.md`,
`<namespace>supervise.md`, `<namespace>recover.md`. Manifest at
`.codex/agents/<namespace>workflow.manifest.json`. Same content,
same five-skill structure, just flat files instead of one
directory per skill.

The `codex` body is *content-identical* to the `claude_code`
body for V1. Codex CLI does not require YAML frontmatter (it
reads agent docs as plain Markdown) but accepting frontmatter is
harmless. The simplest implementation reuses the
`templates/claude_code/<name>.md.tmpl` source for both profiles
via a profile-aware path resolver, and only the destination path
differs.

## Gemini CLI conventions

Gemini CLI's discovery convention is less documented than Claude
Code's; current public documentation centers on `~/.gemini/`
config and project-local `GEMINI.md`. RFC 0015 § "Profile
coverage" already flagged this: *"`gemini` — best-effort; V1
falls back to the `generic` shape if RFC 0010's `gemini` profile
does not yet declare a skill-equivalent surface."*

Concrete V1 plan: write a single concatenated guide at
`<namespace>STRIATUM_GEMINI_GUIDE.md` at the repo root — the same
shape as `generic` but distinct filename so an operator who runs
`--profile all` does not get a `generic`/`gemini` collision.
Manifest at `<namespace>STRIATUM_GEMINI_GUIDE.manifest.json`.
Gemini CLI's "load this Markdown as system context" convention
matches the generic profile's contract; promoting `gemini` to a
five-file shape is a follow-up once Gemini CLI's skill discovery
stabilizes.

## `--profile all`

V1 ships `claude_code` + `generic`. Step 3 adds `codex` +
`gemini`. RFC 0015 § "Profile coverage" defines `all` as: *"all
— writes every profile."*

Behavior:

- `--profile all` runs `install(...)` once per first-class profile
  in `{claude_code, codex, gemini, generic}`.
- The result envelope is a list of per-profile install
  summaries, one per element. The CLI returns 0 if every
  install returned 0; non-zero on the first failure.
- Each profile manifest stays its own file. There is no
  combined "all" manifest.
- Idempotency is preserved: a second `--profile all` against an
  unchanged tree reports `skipped_unchanged` for every file.
- `--scope user` applies uniformly: every profile lands under
  the user-scope root.
- `--namespace <prefix>` applies uniformly.

## Conflict matrix (paths land where, with the default namespace)

| Profile | Files | Manifest |
|---|---|---|
| `claude_code` | `.claude/skills/striatum-{workflow,scaffold,claim-loop,supervise,recover}/SKILL.md` | `.claude/skills/striatum-workflow/.manifest.json` |
| `codex` | `.codex/agents/striatum-{workflow,scaffold,claim-loop,supervise,recover}.md` | `.codex/agents/striatum-workflow.manifest.json` |
| `gemini` | `striatum-STRIATUM_GEMINI_GUIDE.md` (single guide) | `striatum-STRIATUM_GEMINI_GUIDE.manifest.json` |
| `generic` | `striatum-STRIATUM_AGENT_GUIDE.md` (single guide) | `striatum-STRIATUM_AGENT_GUIDE.manifest.json` |

All four profiles can coexist in one tree without collision.

## Test plan

`tests/test_skills_install.py` additions:

- `test_install_codex_writes_five_files_and_manifest` — mirrors
  `test_install_claude_code_writes_five_skills_and_manifest`.
- `test_install_gemini_writes_single_guide` — mirrors
  `test_install_generic_profile_single_guide`.
- `test_install_profile_all_writes_every_profile` — `--profile
  all` produces four manifests, every expected file exists, no
  collisions.
- `test_install_profile_all_idempotent` — second call against
  unchanged tree reports `skipped_unchanged` for every file.
- `test_no_external_url_invariant` (existing) extended to walk
  the codex + gemini outputs.
- `test_install_unknown_profile_raises` (existing) updated:
  `codex` and `gemini` are now allowed; only truly unknown names
  raise. `--profile mystery` still raises.

## Friction anticipated

- **Codex frontmatter.** Claude Code uses YAML frontmatter for
  skill discovery. Codex agent docs are usually plain Markdown,
  but accepting frontmatter is fine. We keep the existing
  templates with frontmatter to avoid maintaining two body
  variants.
- **Gemini five-file vs single-guide.** RFC 0015 explicitly
  permits the generic-shape fallback for V1; promoting to
  five-file is a future RFC follow-up after Gemini CLI's
  convention stabilizes.
- **`--profile all` with `--force`.** Force applies uniformly:
  every profile's edit-detection refusal becomes an overwrite.
  Same with `--dry-run`. The behavior is consistent across
  profiles.
