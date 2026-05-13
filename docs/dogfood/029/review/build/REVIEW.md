---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
---

# Build review: RFC 0025 V1 Steps 2+3 (devils_advocate)

author: reviewer-claude-opus-002

## Posture

Devil's advocate. Argue against scope, template choices, idempotency.

## Counter-claims

### C1: "Codex bundle is 14 files"

Test asserts `CODEX_FILES` set is exactly the 14-file shape.
**Survives.**

### C2: "Codex commands byte-share with claude_code"

Implementer chose to byte-share (just `cp` from claude_code/commands/).
This passes existing F1 byte-match (skills) and a new no-URL test.
**Survives.**

### C3: "Gemini ships 14 files (no hooks/mcp)"

Different from claude_code/codex. The synthesis is honest about
this. **Survives.**

### C4: "Gemini TOML uses bare top-level (F1 verify adopted)"

Implementer chose `description = "..."` and `prompt = """..."""` at
top level — no `[command]` table. Smoke-tested against rendered
output; tests confirm no schemes leak. **Survives** with the note
that I cannot validate this against a live Gemini install in test —
the contract is documented in the BUILD_HANDOFF for follow-up
verification by an operator with `gemini` installed.

### C5: "Codex user-scope is `~/.codex/plugins/<namespace>/`"

Test `test_codex_user_scope_path` confirms the path renders.
**Survives** with the same note as C4 — live Codex install isn't
exercised in test.

### C6: "Gemini marketplace skip is structured"

`test_gemini_marketplace_skipped` asserts `marketplace ==
{"skipped": True, "reason": "..."}`. **Survives.**

### C7: "F1 byte-match extended to all three profiles"

`test_skill_templates_match_skills_module` now iterates
`("claude_code", "codex", "gemini")` and asserts each profile's
skill template tree byte-matches `skills/templates/claude_code/`.
**Survives.**

### C8: "URL-leak invariant on all three profiles"

Three separate tests (`test_*_no_external_urls`). All pass.
**Survives.**

### C9: "--profile all aggregates correctly"

Dispatch loops over `ALL_PROFILES_ORDER`. Result shape is
`{"profile": "all", "results": [...]}`. **Survives** — would be
nice to have a smoke test for the dispatch path, but the
underlying `install` per profile is exhaustively covered.

### C10: "Idempotent re-install across profiles"

Codex idempotency tested directly. Gemini's manifest-driven flow
is the same code path. **Survives.**

## Findings

### F1 (note, non-blocking): Live agent-CLI smoke not exercised in test

Tests render and validate the bundle layout but do not invoke
`codex plugin marketplace add` or `gemini extensions install`.
RFC 0025 acceptance criteria list this as a manual operator check;
the BUILD_HANDOFF documents it. Not blocking — V1 is complete in
the implementer's surface; live integration is operator territory.

## Verdict

**accept**

The build survives every counterargument. RFC 0025 is now
**accepted (V1)** with all three first-class profiles shipped.
Steps 1-3 in three landable pieces matched the RFC's authored plan.
