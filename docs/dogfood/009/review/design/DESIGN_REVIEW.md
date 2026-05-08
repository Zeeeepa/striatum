---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "info"
---

# RFC 0015 V1 Design Review

author: reviewer-claude-opus-001

Date: 2026-05-08
Review target: dogfood-009 / RFC 0015 V1 design synthesis (steps
1+2: `claude_code` + `generic` profiles, `--with-skills`, doctor
checks).
Verdict: `accept`

## Scope

Cross-checked the design synthesis
(`docs/dogfood/009/DESIGN_SYNTHESIS.md`) against the RFC contract
(`docs/rfcs/0015-self-contained-agent-skills.md`) and the research
artifact (`docs/dogfood/009/research/SKILLS_INSTALL.md`).

## Pinned Contracts (verified)

- **Generator purity.** `install(...)` is keyword-only and returns
  a structured result. `render_template` uses `str.format_map` with
  a `MissingKeyDict` so a typo surfaces as `StriatumError` rather
  than silent passthrough. ✓
- **Idempotency.** Files list sorted by `path`; templates embed
  `striatum_version` only (no `generated_at` in body); manifest
  carries `generated_at`. A re-install on an unchanged tree
  produces byte-identical output. ✓ (RFC acceptance criterion 1.)
- **Edit-detection rule.** SHA256 compare against `manifest.files[].sha256`
  drives `refused_modified` without `--force`; with `--force` the
  file is rewritten and the manifest entry's `sha256` is updated;
  `--dry-run` lists the conflict and writes nothing. ✓ (RFC
  acceptance criterion 2.)
- **No-external-URL invariant.** Test
  `test_no_external_url_invariant` walks every rendered file and
  asserts no `http://` / `https://`. Cross-skill links are
  relative. ✓ (RFC acceptance criterion 7.)
- **Manifest schema.** `striatum.skills.manifest.v1` shape matches
  the RFC § 2 sketch with the addition of `template_sha256` per
  entry — this is required to power `skills_outdated` against
  template churn within a version bump (the research artifact
  flagged this). ✓
- **`--with-skills` boundary.** `striatum init --with-skills
  [profile]` is opt-in; default `init` surface is byte-for-byte
  preserved when the flag is absent. ✓ (RFC acceptance criterion 3.)
- **Doctor checks.** `skills_missing` and `skills_outdated` both
  emit `recovery_command` with the exact `striatum skills install`
  invocation. Doctor never auto-regenerates. ✓ (RFC acceptance
  criterion 4.)

## Notes

- The synthesis correctly defers `--profile codex` /
  `--profile gemini` to step 3 of the RFC's path. The locked
  `profile` choice set is `{claude_code, generic}` for V1; this
  matches the RFC's "V1 ships these profiles. Step 3 promotes
  additional profiles without re-opening the RFC."
- The synthesis chooses a *curated* verb whitelist for V1 over
  parser-walked verb introspection. This is a sensible scope cut:
  the verb table is small (<25 entries) and stable; promoting to
  parser-walked is a V1.5 follow-up that does not change the
  bundle's byte shape until it lands.
- The `--target` flag is omitted from V1 in favor of the existing
  `--repo` argument as the install target. This avoids a redundant
  flag and keeps the install boundary aligned with the rest of the
  CLI.
- The manifest's exclusion of itself from `files[]` is correct
  (otherwise a manifest write would always invalidate the manifest
  it just wrote).

## Test Plan Coverage

The pinned 14-test matrix covers every RFC acceptance criterion
plus the manifest-self-exclusion guard and the namespace+scope
matrix. No gaps.

## Decision

`accept`. The V1 build slice locks every contract from the RFC and
the research artifact; no open questions remain that block
implementation.
