---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "info"
---

# RFC 0017 V1 Design Review

author: reviewer-claude-opus-001

Date: 2026-05-08
Review target: dogfood-010 / RFC 0017 V1 design synthesis (steps
1+2+3 ship together).
Verdict: `accept`

## Scope

Cross-checked the design synthesis
(`docs/dogfood/010/DESIGN_SYNTHESIS.md`) against the RFC contract
(`docs/rfcs/0017-readme-and-docs-reorganization.md`) and the
research artifact (`docs/dogfood/010/research/MIGRATION_MAP.md`).

## Pinned Contracts (verified)

- **README section list completeness.** The synthesis pins exactly
  the seven `##` section headers RFC 0017 specified, in the
  specified order. Including `# striatum` as the document title is
  conventional and not a contract violation. ✓
- **New doc set.** Six new files match the RFC 0017 § 2 list
  (`GETTING_STARTED`, `HOW_TO_HUMAN`, `HOW_TO_AGENT`,
  `WRITING_WORKFLOWS`, `CLI_REFERENCE`, `INDEX`) plus
  `docs/dogfood/HISTORICAL.md` for the incubation sections. ✓
- **Dedupe rule.** Every `## Behavior Model` paragraph in the
  README has an existing SPEC anchor verified in the migration
  map's "verified" column. The single exception (`### Process
  Adapter Completion Guarantees (RFC 0014 V1)`) is called out
  explicitly with an instruction to lift, not invent. ✓
- **Cross-link rule.** Internal links use repo-relative paths.
  README → `docs/`: many; `docs/` → README: zero (the docs are
  canonical). This matches the RFC's "single source of truth per
  concept" goal. ✓
- **Test plan.** Three tests in `tests/test_doc_links.py`:
  relative-link resolution, README line budget (≤ 250), and the
  human/agent quick-start heading invariant. The test imports walk
  every Markdown file under `README.md`, `AGENTS.md`,
  `CLAUDE.md`, `docs/*.md`, and `docs/rfcs/*.md` while
  excluding per-run dogfood artifacts. ✓
- **No behavior change.** Documentation only. No CLI surface, no
  schema, no defaults move. Test count moves 260 → 263 entirely
  through the new `test_doc_links.py`. ✓
- **Version bump.** `1.0.0` → `1.1.0` is the right move
  (documentation-only minor bump per RFC). ✓

## Notes

- **AGENTS.md trim.** The synthesis correctly identifies that
  AGENTS.md duplicates the verb sequence today and points it at
  `docs/HOW_TO_AGENT.md` instead. The 200-line ceiling is the
  project rule per `feedback_dogfood_first.md`-style memory; the
  synthesis honors it.
- **Anchor-level link validation deferred.** V1 validates files
  only. Anchor checks are non-trivial (Markdown auto-anchor rules
  vary across renderers); deferring is the right call. A V1.5
  follow-up could extract the actual GitHub-rendered anchor table.
- **`docs/INDEX.md` vs README's "Documentation Map".** The
  synthesis correctly distinguishes these: INDEX is the canonical
  index (≤ 100 lines, one row per `docs/*.md`); the README's
  Documentation Map is a 7-row slim subset that points at INDEX.
  Avoids duplication.
- **`# striatum` title vs section count.** The design synthesis
  flags this explicitly ("yes, that's eight headers including
  `# striatum`"). The interpretation — title is not a section —
  is correct.
- **CLI_REFERENCE.md vs `--help`.** The header note ("may lag;
  `--help` is authoritative") preserves the RFC 0017 § "Open
  Questions" stance without silently accepting drift.
- **External links may break.** Anyone with a bookmark to
  `README.md#dogfood-001-usage` gets a 404 after the move.
  Acceptable per the RFC.

## Test Plan Coverage

The three pinned tests cover every RFC acceptance criterion that
can be enforced mechanically:

- `wc -l README.md` ≤ 250 → `test_readme_under_line_budget`.
- Both quick-start headings present →
  `test_readme_has_human_and_agent_quick_start_headings`.
- Every relative link resolves → `test_relative_doc_links_resolve`.

The remaining acceptance criteria (new docs exist, Behavior Model
paragraphs absent from README, version pins synced) are
file-existence and grep-able assertions that the build review will
verify against the working tree.

## Decision

`accept`. The V1 build slice locks every contract from the RFC and
the research artifact; no open questions block implementation. The
design correctly bundles steps 1+2+3 of RFC 0017's path into one
PR — splitting them would leave broken links during the
intermediate commit, which is worse than a single larger
documentation-only diff.
