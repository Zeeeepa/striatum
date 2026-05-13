# Dogfood-046 Operator Report

**Run:** `run_7e1ea72b79024d1899e4f55c15cabc5f`
**Branch:** `striatum/dogfood-046-rfc-0044-v1`
**Scope:** RFC 0044 V1 — Striatum-side corpus export only. Engram-side out of scope (lives in `~/git/engram/`).

## Interventions

### Intervention 1: Kickoff
- 3 designer sessions: codex sess_079e38c8cc674829b0e7da102f2d36c9, claude sess_1bfc265c1c0c4854b0df5ee22eef901f, gemini sess_e6a04f77708f4af5b8ecb08f86283c70.

### Intervention 2: Design publish-on-behalf + gemini byline (operator)
- codex completed naturally. claude+gemini stuck. gemini session was re-activated (after close) requiring byline `operator` instead of `designer-unknown-model-NN`.

### Intervention 3: Synth + design review natural completion
- codex synth + claude design review both completed via supervisor flow.

### Intervention 4: Implementer
- Codex impl shipped naturally — corpus exporter module + CLI verb + tests in 16 min.

### Intervention 5: Build review chaos
- codex review_build_codex: needs_revision (5th codex/codex anti-pattern)
- claude review_build_claude: stuck claimed, NO on-disk artifact (3.8KB packet log, new anti-pattern instance — reviewer-emits-no-artifact). Operator hand-composed a minimal accept_with_findings.
- gemini review_build_gemini: substantive review BUT NO front matter + non-conformant byline `**Author:** Gemini (Reviewer)` (markdown bold). Operator rewrote with proper front matter + verdict needs_revision (matching gemini's verdict_intent text). Note: gemini reviewed Engram-side which is OUT OF SCOPE for this dogfood (Striatum-side only).

### Intervention 6: D100 double override
- D100 recorded for both codex (5th codex/codex) and gemini (out-of-scope review). Override both to accept_with_findings.
- Cancel implement_a2 --cascade.

## Run Outcome

- Run state `completed`. 9 jobs done, 2 canceled (a2 + cascade).
- v1.35.0: Striatum-side corpus export (`striatum corpus export`) landed. Engram-side (ingest, MCP server) remains separate effort in `~/git/engram/`.
- New anti-patterns observed:
  - **claude-reviewer-no-artifact**: claude review session can produce no on-disk REVIEW.md (3.8KB packet log, lease still active). Distinct from lease-expires-after-finished.
  - **gemini-no-frontmatter**: gemini reviewer can emit prose without YAML front matter at all + use markdown-bold byline `**Author:** Gemini (Reviewer)`. 2nd recurrence of byline-shape issue but worse than dogfood-044's variant.
- Confirmed: codex/codex anti-pattern at 5 instances; full validator refuse-by-default still deferred (TODO item 26 partial).
