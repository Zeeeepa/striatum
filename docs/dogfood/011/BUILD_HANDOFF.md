---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

# RFC 0015 step 3 Build Handoff

author: implementer-codex-gpt-5.5-001

Date: 2026-05-08
Run: dogfood-011 / RFC 0015 step 3 (codex + gemini profiles + `--profile all`)
Decision: `accepted_with_follow_up` (V1_ACCEPTANCE; autonomous)
Version: `1.2.0`

## Files Changed

- **`src/striatum/skills/install.py`**:
  - `ALLOWED_PROFILES` extended to `{claude_code, codex, gemini, generic}`.
  - New `ALL_PROFILES_ORDER = (claude_code, codex, gemini, generic)`.
  - `_build_plan` dispatches to four plan functions.
  - `_plan_codex` writes `.codex/agents/<ns><skill>.md`, reusing
    `templates/claude_code/<skill>.md.tmpl` verbatim.
  - `_plan_gemini` writes `<ns>STRIATUM_GEMINI_GUIDE.md` from
    `templates/gemini/STRIATUM_GEMINI_GUIDE.md.tmpl`.
  - `_manifest_path` adds codex + gemini cases.
- **`src/striatum/skills/templates/gemini/STRIATUM_GEMINI_GUIDE.md.tmpl`**
  (new) — gemini profile body.
- **`src/striatum/skills/__init__.py`** — re-exports
  `ALL_PROFILES_ORDER`, `ALLOWED_PROFILES`.
- **`src/striatum/cli/parser.py`** — `--profile` choices extend to
  `{claude_code, codex, gemini, generic, all}`.
- **`src/striatum/cli/dispatch.py`** — new
  `_skills_install_dispatch(...)` helper handles `all` fan-out;
  used by both `init --with-skills` and `skills install`.
- **`src/striatum/cli/introspect.py`** — `_check_skill_bundle`
  iterates over all four profiles.
- **`tests/test_skills_install.py`** — 10 new cases (25 total),
  `test_install_unknown_profile_raises` updated to use `mystery`.
- **`docs/SPEC.md`** — § "Self-Contained Agent Skills" updated
  to V1+step 3, four profiles + `all` documented.
- **`docs/GETTING_STARTED.md`** — agent path table extended to
  four profiles, four worked examples + `all`.
- **`docs/HOW_TO_HUMAN.md`** — skills-install block shows all
  four profiles + `all`.
- **`docs/HOW_TO_AGENT.md`** — bundle-location section
  enumerates all four destinations.
- **`docs/rfcs/0015-self-contained-agent-skills.md`** — status
  → `accepted (V1+step 3)`.
- **`docs/rfcs/README.md`** — index reflects `accepted (V1+step 3)`
  + D063 reference.
- **`docs/DECISION_LOG.md`** — D063.
- **`docs/TODO.md`** — F11.
- **`pyproject.toml`** — 1.1.0 → 1.2.0.
- **`src/striatum/__init__.py`** — `__version__` 1.1.0 → 1.2.0.
- **`CHANGELOG.md`** — `1.2.0` section.

## Verification

- `make lint` — clean.
- `make typecheck` — clean (51 source files).
- `make test` — 272 passed (262 baseline + 10 new). Net delta is
  +9 because `test_install_unknown_profile_raises` was updated
  in place rather than added.
- Smoke-tested at `/tmp/d011-test`:
  - `striatum --repo . skills install --profile codex --json` →
    5 files at `.codex/agents/striatum-*.md` + manifest.
  - `striatum --repo . skills install --profile all --json` →
    4 manifests, all 12 files (5+5+1+1) on disk.
  - Re-running either is byte-identical.

## Notes For The Reviewer

- **Codex template reuse.** `_plan_codex` calls
  `_render(template_rel="claude_code/<skill>.md.tmpl", ...)` —
  the codex manifest records `template: "claude_code/..."` for
  every entry. This is intentional: the skill content is
  identical; only the destination path differs.
- **`all` fan-out.** Lives in `_skills_install_dispatch` in
  `cli/dispatch.py`, not `install.py`. Keeps `install(...)` a
  single-profile primitive that's easier to unit-test;
  `dispatch` owns the CLI-shape sugar.
- **Doctor coverage.** The single-line loop edit in
  `_check_skill_bundle` extends to all four profiles. The
  helper functions (`manifest_path_for`, `load_manifest`,
  `skill_files_present`, `bundled_template_sha256`) already
  handle the per-profile branches.
- **Step 3 closes RFC 0015's deferred slice.** RFC index
  reflects `accepted (V1+step 3)`. The remaining RFC 0015
  follow-ups (parser-walked verb tables, slash commands,
  embedded workflow schema, auto-regeneration) are revisit
  triggers, not designed work.
