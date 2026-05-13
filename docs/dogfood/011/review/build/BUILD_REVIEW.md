---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "info"
---

# RFC 0015 step 3 Build Review

author: reviewer-claude-opus-002

Date: 2026-05-08
Verdict: `accept`

## Pinned contracts (verified)

- **`ALLOWED_PROFILES`** = `{claude_code, codex, gemini, generic}`. ✓
- **`ALL_PROFILES_ORDER`** = `(claude_code, codex, gemini, generic)`,
  deterministic. ✓
- **Codex layout**: `.codex/agents/<ns><skill>.md` (5 files), manifest
  at `.codex/agents/<ns>workflow.manifest.json`. Template recorded as
  `claude_code/<skill>.md.tmpl` so codex inherits any future template
  fix automatically. ✓
- **Gemini layout**: single guide
  `<ns>STRIATUM_GEMINI_GUIDE.md`, manifest sibling. Distinct filename
  keeps `--profile all` collision-free. ✓
- **`--profile all`** envelope is `{"profile": "all", "scope": ...,
  "namespace": ..., "results": [4 entries]}`, deterministic order. ✓
- **`init --with-skills all`** flows through the same fan-out. ✓
- **Doctor**: `_check_skill_bundle` loops over all four profiles. ✓
- **Tests**: 25 pass (10 new), regression on existing 15 verified by
  the full-suite run (272 total).
- **Lint + typecheck**: clean.

## Notes

- The `_skills_install_dispatch` helper sits in `cli/dispatch.py`
  rather than `skills/install.py`. That keeps `install(...)` a
  single-profile primitive — clean separation.
- Codex template reuse means `bundled_template_sha256` returns
  the same value for the codex manifest's
  `template_sha256` as it would for the equivalent claude_code
  entry. Doctor's `skills_outdated` check naturally fires on
  template churn for codex too.
- Gemini's single-guide-with-distinct-filename is the right call
  per RFC 0015's permitted fallback. Promoting to a five-file
  layout is a future RFC concern.

## Decision

`accept`. RFC 0015 step 3 closes cleanly; the V1.2 release
delivers the deferred Codex + Gemini profiles + `--profile all`
without regression to claude_code or generic.
