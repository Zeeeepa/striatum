---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "info"
---

# RFC 0015 V1 Build Review

author: reviewer-claude-opus-002

Date: 2026-05-08
Review target: dogfood-009 / RFC 0015 V1 build slice (steps 1+2:
`claude_code` + `generic` profiles, `--with-skills`, doctor
checks).
Verdict: `accept`

## Scope

Cross-checked the implementation against the locked design synthesis
(`docs/dogfood/009/DESIGN_SYNTHESIS.md`), the RFC contract
(`docs/rfcs/0015-self-contained-agent-skills.md`), and the V1
acceptance gate (`docs/dogfood/009/decisions/V1_ACCEPTANCE.md`).
Verification window: `make lint`, `make typecheck`, `make test`,
plus targeted reads of `src/striatum/skills/`, the parser/dispatch
wiring, the new doctor checks, and the 16-test matrix.

## Pinned Contracts (verified)

- **`src/striatum/skills/`** package layout matches the synthesis:
  `__init__.py` re-exports the public surface; `install.py` owns
  the pipeline + manifest IO; `context.py` carries the curated
  verb table and boundary statements; `templates/{claude_code,generic}/`
  ship the unrendered Markdown templates.
- **`install(...)`** is keyword-only and returns a structured
  result with one entry per file plus `manifest_path`. Statuses
  are exactly `written | skipped_unchanged | refused_modified |
  dry_run` per the synthesis. ✓
- **Idempotency.** A second install with the same args produces
  `skipped_unchanged` for every file and the manifest stays
  byte-identical. Verified by
  `test_install_idempotent_byte_identical`. ✓ (RFC acceptance 1.)
- **Edit-detection.** `test_install_refuses_modified_file_without_force`
  hand-edits a SKILL.md and confirms `refused_modified`;
  `test_install_force_overwrites_modified_file` confirms `--force`
  rewrites it. ✓ (RFC acceptance 2.)
- **`--dry-run`.** Writes nothing; manifest is not written either.
  Verified by `test_install_dry_run_writes_nothing`. ✓
- **`--with-skills` boundary.** End-to-end test
  `test_init_with_skills_installs_after_init` confirms both
  `.striatum/state.sqlite3` *and* the bundle land in one call;
  the default `init` surface is preserved (no `skills` key in
  the response when the flag is absent). ✓ (RFC acceptance 3.)
- **Doctor checks.** Three tests cover both checks under both
  failure modes: `test_doctor_reports_skills_missing` removes a
  recorded file, `test_doctor_reports_skills_outdated_on_version`
  rewrites the manifest's `striatum_version`, and
  `test_doctor_reports_skills_outdated_on_template_sha` mutates a
  recorded `template_sha256`. ✓ (RFC acceptance 4.)
- **No-external-URL invariant.**
  `test_no_external_url_invariant` walks every rendered file from
  both profiles and asserts no `http://` / `https://`. ✓ (RFC
  acceptance 7.)
- **Manifest schema.** `striatum.skills.manifest.v1` matches the
  synthesis: `schema_version`, `striatum_version`, `generated_at`,
  `profile`, `namespace`, `scope`, `files[]` with
  `path/sha256/template/template_sha256`. The manifest excludes
  itself from `files[]` (`test_manifest_excludes_itself`). ✓
- **Bundled-vs-recorded template SHA parity.**
  `test_bundled_template_sha_matches_manifest_after_install`
  closes the loop: `bundled_template_sha256(template)` matches the
  `template_sha256` recorded in the manifest immediately after a
  fresh install. ✓
- **Profile validation.**
  `test_install_unknown_profile_raises` confirms `codex` is
  rejected with `InvalidTransitionError` rather than silently
  falling through. ✓ (Step 3 boundary preserved.)

## Notes

- **`_StrictFormatMap`.** Subclasses `dict[str, Any]` and
  overrides `__missing__` so `str.format_map` raises on unknown
  placeholders. The implementer flagged this in the handoff; it's
  the cleanest way to keep template typos loud given Python's
  default lenient `format_map` behavior.
- **Helper expansion order.** `_expand_helpers` runs before
  `str.format_map` and uses literal `str.replace`, so templates
  can use `{verbs_scaffold}` / `{boundaries_bulletlist}` / etc.
  without escaping issues. The formatter only sees
  `{striatum_version}`. Sensible scope cut.
- **`__version__` correction.** The implementer noticed
  `src/striatum/__init__.py:__version__` had drifted to `0.1.0`
  while `pyproject.toml` was at `0.4.0`; both are now `0.5.0`.
  This is the correct fix and not a contract violation — the
  manifest reads `striatum.__version__` directly, so the prior
  drift would have caused stale `striatum_version` records.
- **Doctor lazy fire.** `_check_skill_bundle` only inspects
  manifests that already exist. A target without a bundle stays
  silent; this matches the RFC's "operator opts in via
  `init --with-skills` or `skills install`" stance.
- **`recovery_command` envelope.** Both doctor checks emit a
  `recovery_command` string with the exact `striatum --repo
  <repo> skills install --profile <p>` invocation. Operators get
  a copy-paste fix; the runner never auto-regenerates.
- **Step 3 boundary.** Parser's `--profile` choices are exactly
  `{claude_code, generic}`. Adding `codex` / `gemini` will touch
  `parser.py`, `install.ALLOWED_PROFILES`, one or two new
  template files, and a small set of new tests; no schema or
  manifest changes required. Clean deferral.

## Verification

- `make lint`: clean.
- `make typecheck`: clean (50 source files).
- `make test`: 260 passed (16 new skills tests on top of 244
  baseline; no regressions in the existing 244).
- Manual smoke against `/tmp/rfc-0015-test`: `init` → `skills
  install --profile claude_code` → re-run reports `skipped_unchanged`
  → `skills install --profile generic` writes one guide. Bundle
  contents read cleanly and contain no external URLs.

## Decision

`accept`. The V1 build slice meets every pinned contract from the
design synthesis and every RFC acceptance criterion that V1 was
scoped to address. Step 3 (`codex` + `gemini` profiles, parser-
walked verb table, `--profile all`) is correctly deferred and the
parser already constrains `--profile` to the V1 set so the
follow-up does not need to re-open RFC 0015.
