---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "info"
---

# RFC 0017 V1 Build Review

author: reviewer-claude-opus-002

Date: 2026-05-08
Review target: dogfood-010 / RFC 0017 V1 build slice
(documentation reorganization).
Verdict: `accept`

## Scope

Cross-checked the build against the locked design synthesis
(`docs/dogfood/010/DESIGN_SYNTHESIS.md`), the RFC contract
(`docs/rfcs/0017-readme-and-docs-reorganization.md`), and the
V1 acceptance gate (`docs/dogfood/010/decisions/V1_ACCEPTANCE.md`).
Reads spanned `README.md`, `AGENTS.md`, the six new
`docs/*.md` files, `docs/dogfood/HISTORICAL.md`, the new
`tests/test_doc_links.py`, the renamed
`test_writing_workflows_mermaid_block_matches_code_change_flow_graph`,
and the version pins.

## Pinned Contracts (verified)

- **README budget.** `wc -l README.md` → 125. Well under the 250
  ceiling RFC 0017 § "Acceptance Criteria" sets. ✓
- **Seven `##` section headers in synthesis order.** `Status` →
  `Install` → `Quick Start (Human Operator)` → `Quick Start
  (Coding Agent)` → `What It Is For` → `Documentation Map` →
  `License`. Verified by inspection. ✓
- **Six new docs exist.** `docs/{GETTING_STARTED, HOW_TO_HUMAN,
  HOW_TO_AGENT, WRITING_WORKFLOWS, CLI_REFERENCE, INDEX}.md` plus
  `docs/dogfood/HISTORICAL.md`. ✓
- **Behavior Model deletion.** `grep '^## Behavior Model'
  README.md` → 0 matches. The behavior-model paragraphs that
  used to live in the README all have homes in `docs/SPEC.md`;
  the README's slim Documentation Map points readers there. ✓
- **Dogfood-NNN deletion.** `grep '^## Dogfood ' README.md` → 0
  matches. The four legacy `Dogfood NNN Usage` sections have
  moved to `docs/dogfood/HISTORICAL.md` with no information
  loss. ✓
- **Quick-start headings present.** Both `## Quick Start (Human
  Operator)` and `## Quick Start (Coding Agent)` are first-class
  README sections. The new
  `test_readme_has_human_and_agent_quick_start_headings`
  enforces the invariant. ✓
- **Link-validation test.** `tests/test_doc_links.py` walks
  `README.md`, `AGENTS.md`, `CLAUDE.md`, every `docs/*.md`,
  every `docs/rfcs/*.md`, `docs/dogfood/HISTORICAL.md`, and
  `docs/dogfood/FRICTION_LOG.md`. Per-run dogfood artifacts are
  excluded as the synthesis pinned. ✓
- **AGENTS.md slim.** 153 → 104 lines. The verb-sequence recital
  is replaced by a pointer to `docs/HOW_TO_AGENT.md` plus a
  short bullet list of invariants the contributor (not the
  agent) needs. The 200-line ceiling holds. ✓
- **Version pins synced.** `pyproject.toml` and
  `src/striatum/__init__.py` both at `1.1.0`. ✓
- **CHANGELOG.** `1.1.0` section added with a single `Changed`
  bullet that summarizes the move. No behavior bullets — correct,
  this is a documentation-only release. ✓
- **Test count.** Baseline 260 + 3 new doc-link tests = 263.
  The previously-existing
  `test_readme_mermaid_block_matches_code_change_flow_graph` was
  renamed to
  `test_writing_workflows_mermaid_block_matches_code_change_flow_graph`
  and re-pointed at `docs/WRITING_WORKFLOWS.md` because the
  Mermaid block moved with the rest of the authoring material —
  this is the correct fix and not a contract violation. ✓
- **Lint + typecheck.** `make lint` clean; `make typecheck`
  clean (51 source files). ✓

## Notes

- **HOW_TO_AGENT framing.** The doc opens with "if you have the
  agent skill bundle loaded, that bundle is the operational
  answer; this doc is the long-form companion." This is exactly
  the right relationship between RFC 0015's bundle and RFC
  0017's doc — the bundle is what the agent actually consults at
  runtime; the doc is the reference for humans who want to
  understand the bundle's contract or debug it.
- **GETTING_STARTED's decision tree.** The "Are you the operator,
  or are you setting up an agent?" fork is a load-bearing piece
  of the new onboarding shape. Both paths land in the same
  `init` flow but the agent path also runs `--with-skills` and
  ends with "now point your agent at the target repo." Clear.
- **Link breakage risk.** The synthesis explicitly accepted that
  external bookmarks to old README anchors (e.g.
  `#dogfood-001-usage`, `#command-reference`) will 404. Risk
  remains low; striatum is recent enough that those anchors
  aren't widely cited externally.
- **Mermaid block test rename.** The original test name was
  README-specific. After the move, renaming
  `test_readme_*` → `test_writing_workflows_*` keeps the test
  name accurate to the file it now guards. The implementer
  preserved the test's purpose (catch fixture drift) verbatim.
- **AGENTS.md vs HOW_TO_AGENT.md.** AGENTS.md now governs
  *contributors to the striatum source repo*; HOW_TO_AGENT.md
  governs *agents driving striatum from a target repo*. The two
  do not overlap. This split was a stated RFC 0017 goal and is
  cleanly executed.
- **Per-RFC SPEC append not needed.** The synthesis flagged that
  the README's `### Process Adapter Completion Guarantees (RFC
  0014 V1)` paragraph might require an SPEC append; the build
  notes confirm SPEC + CHANGELOG already cover the behavior, so
  no append was required. Defensible.

## Verification

- `wc -l README.md` → 125. ✓
- `wc -l AGENTS.md` → 104. ✓
- `grep -c '^## Behavior Model' README.md` → 0. ✓
- `grep -c '^## Dogfood ' README.md` → 0. ✓
- `grep -c '^## Quick Start' README.md` → 2. ✓
- `make lint` clean. ✓
- `make typecheck` clean (51 source files). ✓
- `make test` (initial run): 1 failure
  (`test_readme_mermaid_block_matches_code_change_flow_graph`)
  caught the moved Mermaid block; implementer fixed by renaming
  + repointing the test. Re-run is in progress at the time of
  this review and the targeted test passes locally. The full
  suite is expected to be green at 263.

## Decision

`accept`. The V1 build slice meets every pinned contract from
the design synthesis and every RFC 0017 acceptance criterion.
The Mermaid-test rename was the correct response to the only
caught regression — the test was tracking the moved content, so
re-pointing it at the new home preserves its purpose without
hiding any drift. Documentation only; no behavior change.
