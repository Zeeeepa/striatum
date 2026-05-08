---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "info"
---

# RFC 0015 step 3 Design Review

author: reviewer-claude-opus-001

Date: 2026-05-08
Verdict: `accept`

## Pinned contracts (verified)

- **`ALLOWED_PROFILES` extension.** `claude_code, codex, gemini,
  generic`. `all` is a CLI alias and stays out of the set —
  correct, otherwise `install("all")` would route to a
  non-existent plan function. ✓
- **Path layouts.** Codex uses flat
  `.codex/agents/<ns><skill>.md`; Gemini uses single guide
  `<ns>STRIATUM_GEMINI_GUIDE.md`; both manifests are siblings
  of their content. No collisions across the four profiles for
  the default namespace. ✓
- **Template reuse.** Codex reuses
  `templates/claude_code/<skill>.md.tmpl` (same body, different
  destination). Same `template_sha256` recorded in both
  manifests. Saves ~5 files of duplication. ✓
- **`--profile all`.** Loops over the four real profiles in
  deterministic order; per-profile result envelope; per-profile
  manifests (no combined "all" manifest). Idempotent. ✓
- **Doctor extension.** `_check_skill_bundle` already loops; just
  add codex+gemini to the loop. ✓
- **`init --with-skills all`** flows through the same dispatcher
  loop. ✓

## Notes

- The Gemini single-guide-with-distinct-filename choice is the
  right call. RFC 0015 explicitly permits this fallback shape;
  promoting `gemini` to a five-file layout is a future RFC
  follow-up that does not need to block step 3.
- Test plan adds 10 cases (~273 total). The
  `test_install_unknown_profile_raises` update is a good catch
  — the test today asserts `codex` raises, which will flip after
  this lands.
- No-external-URL invariant extension to walk codex+gemini
  outputs is mechanical; the assertion already exists for the
  other two profiles.

## Decision

`accept`. The design is the minimum viable extension of the V1
plan: same template engine, same manifest schema, same edit
detection, two new path resolvers, one `all` fan-out. No new
schema, no behavior regression, deterministic ordering. Step 3
shipped this way is a cleanly closed item against RFC 0015.
