---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/rfcs/0015-self-contained-agent-skills.md", "docs/dogfood/011/research/PROFILE_LAYOUTS.md", "src/striatum/skills/install.py", "src/striatum/skills/templates/"]
---

# RFC 0015 step 3 Design Synthesis

author: designer-codex-gpt-5.5-001

Date: 2026-05-08
Target: V1 build slice for RFC 0015 step 3 (codex + gemini
profiles + `--profile all`).

## Locked Contracts

### `ALLOWED_PROFILES` extension

```python
# src/striatum/skills/install.py
ALLOWED_PROFILES: frozenset[str] = frozenset(
    {"claude_code", "codex", "gemini", "generic"}
)
```

`all` is **not** in `ALLOWED_PROFILES` because it is a CLI alias
that fans out to the four real profiles. It is accepted by the
parser and dispatch but never reaches `install(...)` directly.

### Path resolution per profile

| Profile | Files (per skill) | Manifest |
|---|---|---|
| `claude_code` | `.claude/skills/<ns><skill>/SKILL.md` (5 files) | `.claude/skills/<ns>workflow/.manifest.json` |
| `codex` | `.codex/agents/<ns><skill>.md` (5 files) | `.codex/agents/<ns>workflow.manifest.json` |
| `gemini` | `<ns>STRIATUM_GEMINI_GUIDE.md` (1 file) | `<ns>STRIATUM_GEMINI_GUIDE.manifest.json` |
| `generic` | `<ns>STRIATUM_AGENT_GUIDE.md` (1 file) | `<ns>STRIATUM_AGENT_GUIDE.manifest.json` |

`gemini`'s single-file shape is the V1 fallback per RFC 0015 §
"Profile coverage" (`gemini` falls back to the `generic` shape
until Gemini CLI's skill convention stabilizes); the distinct
filename keeps `--profile all` collision-free.

### Template reuse

`codex` reuses the *exact same* five Markdown bodies as
`claude_code` — both profiles render
`templates/claude_code/<skill>.md.tmpl`. The plan function picks
the destination path; the template body is identical. This means
`bundled_template_sha256("claude_code/<skill>.md.tmpl")` is
recorded in both `claude_code` and `codex` manifests for the
same skill.

`gemini` and `generic` each have their own template
(`templates/{generic,gemini}/STRIATUM_AGENT_GUIDE.md.tmpl` vs
`templates/gemini/STRIATUM_GEMINI_GUIDE.md.tmpl`); the gemini
template is a near-copy of the generic one with a Gemini-specific
header note about discovery.

Add `templates/gemini/STRIATUM_GEMINI_GUIDE.md.tmpl` (~80 lines,
copy of generic with a one-paragraph header about "this guide is
intended for Gemini CLI; load it as system context").

### `--profile all`

Parser: `--profile {claude_code, codex, gemini, generic, all}`.

Dispatch: when `args.profile == "all"`, the dispatcher loops over
`["claude_code", "codex", "gemini", "generic"]` (deterministic
order), calling `install(...)` once per profile with all other
args passed through (`--scope`, `--namespace`, `--force`,
`--dry-run`).

Result envelope when `--profile all`:

```json
{
  "profile": "all",
  "scope": "project",
  "namespace": "striatum-",
  "results": [
    {"profile": "claude_code", "files": [...], "manifest_path": "..."},
    {"profile": "codex", "files": [...], "manifest_path": "..."},
    {"profile": "gemini", "files": [...], "manifest_path": "..."},
    {"profile": "generic", "files": [...], "manifest_path": "..."}
  ]
}
```

`init --with-skills all` works the same way.

### Doctor

`_check_skill_bundle` already iterates over a profile list. Add
`codex` and `gemini` to the loop so missing/outdated bundles are
surfaced for the new profiles too.

### Tests (additions)

`tests/test_skills_install.py` gains:

- `test_install_codex_writes_five_files_and_manifest`
- `test_install_codex_idempotent_byte_identical`
- `test_install_gemini_writes_single_guide`
- `test_install_gemini_idempotent_byte_identical`
- `test_install_profile_all_writes_every_profile`
- `test_install_profile_all_idempotent`
- `test_install_profile_all_dry_run_writes_nothing`
- `test_install_profile_all_force_overwrites`
- `test_doctor_reports_skills_missing_for_codex_profile`
- `test_doctor_reports_skills_missing_for_gemini_profile`

`test_no_external_url_invariant` extended to walk codex + gemini
outputs. `test_install_unknown_profile_raises` updated:
`mystery` still raises; `codex` and `gemini` are accepted.

## Acceptance Criteria

- `striatum skills install --profile codex` writes 5 files +
  manifest at `.codex/agents/striatum-*.md`.
- `striatum skills install --profile gemini` writes 1 file +
  manifest at `striatum-STRIATUM_GEMINI_GUIDE.md`.
- `striatum skills install --profile all` writes all four
  profile bundles in one call, returns a `results: [...]` envelope.
- A second `--profile all` call against the unchanged tree
  reports `skipped_unchanged` for every file across all four
  profiles.
- `striatum doctor --verbose` reports `skills_missing` /
  `skills_outdated` for codex and gemini bundles when
  applicable.
- `striatum init --with-skills all` initializes `.striatum/`
  and writes all four bundles.
- The `claude_code` and `generic` paths produce byte-identical
  output to v1.1.0 for the same args (regression).
- Test count moves from 263 → ~273 (10 new tests).
- `pyproject.toml` and `__version__` bump 1.1.0 → 1.2.0.

## Acceptance Gate

Implementation job blocks until human acceptance recorded under
`docs/dogfood/011/decisions/`.
