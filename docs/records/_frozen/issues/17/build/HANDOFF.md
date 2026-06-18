---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

author: implementer-unknown-model-001

# GH #17 -- Build Handoff (attempt 2)

Striatum-side documentation consistency pass plus the RFC 0057 scaffold for
the Corpus Contract V2. Scope was bounded by `docs/issues/17/SCOPE.md` and
the captured `docs/issues/17/SPEC.md`. No `src/`, `tests/`, `go/`, or
package-metadata files were touched. Attempt 1's content edits remain in
the worktree and are unchanged; attempt 2 closes out the two reviewer
findings from `docs/issues/17/review/REVIEW.md`.

## Attempt-2 deltas (closing prior REVIEW.md findings)

- **REVIEW F1 (HIGH) — untracked files.** `git add docs/issues/17/
  docs/rfcs/0057-corpus-contract-v2.md` was run, so the RFC 0057 scaffold
  and the entire `docs/issues/17/` workflow context directory
  (OPERATOR_REPORT.md, SCOPE.md, SPEC.md, build/, prompts/, review/,
  roles/, workflow.json) are now staged in the index. Verified with
  `git status --short docs/issues/17/ docs/rfcs/0057-corpus-contract-v2.md`
  showing 15 `A ` (added) entries and zero `?? ` (untracked) entries.
- **REVIEW F2 (LOW) — TODO.md top table out of sync.** Row 42 of
  `docs/TODO.md` (`docs/TODO.md:100`) was updated from
  `⏳ open` to `🟡 docs pass + RFC 0057 scaffold landed; row stays open
  until RFC 0057 V2 acceptance`, matching the detail-section status
  already present in the bottom of the file.
- **REVIEW F3 (INFO) — unrelated worktree changes.** The worktree
  contains source changes for GH #9/#10 (`src/striatum/service.py`,
  `src/striatum/recovery/auto.py`, `src/striatum/cli/recovery.py`),
  GH #15 (`docs/POSTGRES_TRANSITION.md`), the operator-init prompt
  (GH #16, multiple `**/skills/**` template touches), and frontend
  edits (`src/striatum/web/...`). These belong to other parallel
  workflows running against the shared branch
  `striatum/gh-issues-parallel`; this implementer did not introduce
  them and (per scope) does not modify them. The handoff explicitly
  enumerates them here so the reviewer/operator can reconcile when
  finalizing the branch.

## Summary (carried forward from attempt 1, content unchanged)

- Engram is consistently described across Striatum's current product
  docs as an **optional local augmentation consumer of `striatum corpus
  export` bundles**, not a runtime dependency. The non-negotiable
  invariants (no `import engram`, no `memory.*` capability, no state
  transition that fails when Engram is missing) are stated once in
  `docs/SPEC.md` and referenced from the surrounding operator-facing
  docs.
- A proposed RFC 0057 scaffold lives at
  `docs/rfcs/0057-corpus-contract-v2.md`. It frames the V2 decision
  surface (contract version, multi-corpus identity, source-kind
  extensibility, stable IDs, instance/repository identity, redaction
  tiers, incremental-export watermarks, validation rules, V1→V2
  backward compatibility, augmentation-boundary regression coverage,
  optional context-injection policy, Engram-availability recording)
  without committing acceptance criteria.
- `striatum corpus export` is now discoverable from
  `docs/CLI_REFERENCE.md`, the CLI listing in `docs/SPEC.md`, and the
  operator playbook `docs/HOW_TO_HUMAN.md`. `docs/HOW_TO_AGENT.md` and
  `docs/MCP.md` carry the augmentation-boundary callout for agents and
  chat-tool clients respectively.
- `docs/UBIQUITOUS_LANGUAGE.md` gained five new glossary terms
  (`Striatum corpus`, `Striatum corpus export`, `memory augmentation`,
  `augmentation-not-dependency`, `corpus contract version`) plus a
  re-pointed opening paragraph and the matching Distinctions bullet.
- `docs/ENGRAM_INCUBATION_CONTEXT.md` retains its historical status
  but now opens with a "Current direction" pointer to SPEC, RFC 0041,
  RFC 0044, and RFC 0057 so it cannot mislead a reader who lands
  there first.
- `docs/ROADMAP.md` §5.7 and `docs/TODO.md` item 42 cross-reference
  the scaffold and mark the docs pass as the GH #17 deliverable. Both
  explicitly note that GH #17 stays open until RFC 0057 V2 acceptance
  lands.

## Files changed

| Path | Kind | What changed |
|---|---|---|
| `docs/rfcs/0057-corpus-contract-v2.md` | new | Proposed RFC scaffold for Striatum Corpus Contract V2. 12 numbered decision sections, deferred acceptance criteria, 5 open questions, domain-modeling table with value-object names. Now `git add`ed. |
| `docs/rfcs/README.md` | edit | Added RFC 0057 row to the index (`proposed (scaffold)` status, one-paragraph hook, cross-link to GH #17). |
| `docs/SPEC.md` | edit | Added § "Corpus Export And Augmentation Boundary" after the artifact front-matter schemas section. Added `striatum corpus export --since <ref> --out <dir>` to the CLI listing. Augmented the Product Boundary section with a one-paragraph callout. |
| `docs/CLI_REFERENCE.md` | edit | New "Corpus export" section between Inspection-and-recovery and Adapter. Documents the verb, replay semantics, and cross-link to RFC 0057. |
| `docs/HOW_TO_HUMAN.md` | edit | New "Optional: export a corpus bundle for an external memory consumer" section near the end of the playbook, just before "See also". |
| `docs/HOW_TO_AGENT.md` | edit | New "do not" bullet about not assuming external memory/retrieval availability; packet context + canonical docs remain authoritative; augmentation policy scoped by RFC 0057. |
| `docs/MCP.md` | edit | Added paragraph after the daemon MCP mutation-capability vocabulary section clarifying that Striatum's MCP and chat tool surfaces do not include any `memory.*` capability and that Engram's `memory.*` capabilities live inside its own `engram-mcp-stdio` MCP server. |
| `docs/UBIQUITOUS_LANGUAGE.md` | edit | Rewrote the second paragraph of the doc preamble; added five new terms; added a Distinctions bullet. |
| `docs/ENGRAM_INCUBATION_CONTEXT.md` | edit | Added a "Current direction" callout block at the top while preserving the historical body verbatim. |
| `docs/ROADMAP.md` | edit | §5.7 gains a "RFC 0057 scaffold landed (2026-05-14)" paragraph linking to the new RFC and noting the deferral of V2 acceptance criteria. |
| `docs/TODO.md` | edit | Detail item 42 already captured the doc-consistency pass status (🟡, list of touched files, "stays open until RFC 0057 V2 acceptance lands"). Attempt 2 also updated the **top status snapshot table** (`docs/TODO.md:100`) so row 42 reads `🟡 docs pass + RFC 0057 scaffold landed; row stays open until RFC 0057 V2 acceptance` instead of `⏳ open`. |
| `docs/issues/17/` (entire dir) | new | Captured workflow context (SCOPE.md, SPEC.md, OPERATOR_REPORT.md, prompts/, review/, roles/, workflow.json, build/HANDOFF.md). All `git add`ed in attempt 2. |

No `src/`, `tests/`, `go/`, `examples/`, `prompts/`, `scripts/`, or
package-metadata files were touched by GH #17.

## Acceptance evidence (against SCOPE.md)

- **GH17-1 (single coherent operator path).** SPEC §"Corpus Export And
  Augmentation Boundary" (`docs/SPEC.md:625`-`docs/SPEC.md:663`) ↔
  HOW_TO_HUMAN §"Optional: export a corpus bundle for an external
  memory consumer" (`docs/HOW_TO_HUMAN.md:955`-`docs/HOW_TO_HUMAN.md:980`) ↔
  CLI_REFERENCE §"Corpus export" (`docs/CLI_REFERENCE.md:307`-
  `docs/CLI_REFERENCE.md:326`) ↔ RFC 0057
  (`docs/rfcs/0057-corpus-contract-v2.md`).
- **GH17-2 (what Striatum exports for ingestion).** SPEC §"Corpus
  Export And Augmentation Boundary" enumerates the V1 nine-kind set
  (RFCs, decision-log rows, operator reports, run summaries,
  audit-chain entries, changelog entries, ubiquitous-language terms,
  harness-friction patterns, recent commits) plus the `manifest.json`
  with per-file row counts and SHA-256 and a derived `bundle_sha256`.
  HOW_TO_HUMAN gives the operator the exact command to produce one.
- **GH17-3 (RFC 0057 scaffold names V2 decisions).** RFC 0057 has 12
  numbered decision sections covering manifest shape, multi-corpus
  identity, source-kind extensibility, stable IDs and per-row content
  hashing, instance/repository identity, redaction tiers,
  incremental-export watermark, validation rules, V1↔V2 backward
  compatibility, augmentation-boundary regression coverage, optional
  context-injection policy, and Engram-availability recording. All the
  §5.7 questions from ROADMAP are named.
- **GH17-4 (must run without Engram).** Stated as a non-negotiable
  invariant three times: SPEC §"Corpus Export And Augmentation
  Boundary", RFC 0057 Goals + Non-Goals + Decision 10, and
  UBIQUITOUS_LANGUAGE term `augmentation-not-dependency`. The
  invariant cites the existing regression test by name
  (`tests/test_cli_corpus_export.py::test_no_engram_imports_or_memory_capabilities_in_striatum`).
- **GH17-5 (no cloud / telemetry / hosted / transcript implication).**
  No new wording introduces hosted persistence, telemetry, transcript
  capture, or always-running daemons. SPEC's Product Boundary already
  refuses hosted services; the new paragraph extends the same refusal
  to external memory consumers. HOW_TO_HUMAN flags export as
  operator-triggered and local. MCP.md notes the operator wires
  `engram-mcp-stdio` out of band — Striatum does not configure or
  launch it.
- **GH17-6 (Striatum/Engram boundary preserved).** SPEC §"Corpus
  Export And Augmentation Boundary" and RFC 0057 both place
  responsibility for ingestion, indexing, retrieval, `memory.*`
  capabilities, and personal-memory isolation entirely on Engram.
  UBIQUITOUS_LANGUAGE `memory augmentation` matches.
- **GH17-7 (narrow PostgreSQL wording).** No new PostgreSQL transition
  guidance was introduced. The only Postgres references in changed
  files are in the RFC 0057 Open Question 4 ("how does the Postgres-
  as-sole-substrate transition affect corpus export's `--since <ref>`
  semantics") and the unchanged SPEC Product Boundary substrate
  paragraph. GH #15's full transition sweep is untouched by this
  workflow (a separate parallel workflow's `docs/POSTGRES_TRANSITION.md`
  is in the worktree, but is not a GH #17 deliverable).
- **GH17-8 (stale guidance updated or marked deferred).** Only one
  stale spot existed in current product docs:
  `docs/ENGRAM_INCUBATION_CONTEXT.md`. It now opens with an explicit
  "Current direction" pointer to SPEC and RFC 0041/0044/0052, while
  the historical body stays as-is for incubation provenance. Other
  Engram mentions in current docs (`docs/SPEC.md`,
  `docs/UBIQUITOUS_LANGUAGE.md`, `docs/INDEX.md`,
  `docs/rfcs/README.md`) were already accurate or are updated above.
  Historical `docs/dogfood/` and `prompts/` material is left alone per
  scope's "historical fixture" rule.
- **GH17-9 (handoff lists changes and verification).** This document.

## Verification

```bash
PYTHONPATH=src python3 -m pytest \
    tests/test_doc_links.py \
    tests/test_artifact_schemas.py \
    tests/test_cli_corpus_export.py -q
# attempt-2 result: 28 passed, 1 failed
#   FAILED tests/test_doc_links.py::test_decision_log_rows_under_word_budget
#       AssertionError: DECISION_LOG rows over budget: D094: 439 words
#   This failure is pre-existing, called out in docs/ROADMAP.md §9.2,
#   and unrelated to GH #17 (no DECISION_LOG.md edits in this workflow).

git status --short docs/issues/17/ docs/rfcs/0057-corpus-contract-v2.md
# 15 lines of "A " (added/staged), 0 lines of "?? " (untracked).

grep -n "^| 42 |" docs/TODO.md
# 100:| 42 | GH #17 — Striatum doc consistency for Engram memory integration
#       | 🟡 docs pass + RFC 0057 scaffold landed; row stays open until
#         RFC 0057 V2 acceptance |
```

Spot-check greps that confirm the attempt-1 content edits survive in the
worktree (no regressions):

```bash
grep -n "Engram\|memory\." docs/SPEC.md | head -20
#  50:External memory or retrieval systems (Engram, under RFC 0044, is the first
#  53:does not register `memory.*` capabilities, and does not call retrieval
# 645:external memory or retrieval system (Engram is the first reference consumer
# 648:any `memory.*` capability, or call any retrieval surface during state
# 652:- No `memory.*` capability in the Striatum daemon method registry.
# 1613:steps without Engram-specific paths or live model requirements.

grep -n "corpus export" docs/CLI_REFERENCE.md | head
# 345:striatum corpus export --since <ref> --out <dir>
# 348:`corpus export` emits a redacted JSONL bundle of Striatum's durable
```

## Verification not run

- `make test` (full pytest sweep) — not run. The captured SPEC restricts
  the workflow's verification to the doc-link + artifact-schema tests
  and the corpus-export tests; broader sweeps are out of scope for a
  docs-only change and the ROADMAP §9.2 baseline already records the
  pre-existing 11 env-dependent failures.
- `make lint`, `make typecheck` — not run. No Python/Go source changes
  in this workflow.
- Engram-side validation (ingest-striatum, engram-mcp-stdio, capability
  defaults) — out of scope; Engram lives in `~/git/engram/` under a
  separate roadmap.

## Residual risk

- **RFC 0057 scaffold is intentionally non-final.** It frames the
  decision surface and names the invariants; it does not commit
  acceptance criteria. A reviewer expecting a complete V2 RFC will
  correctly flag that gap, and the next dogfood's design phase must
  consume this scaffold and produce concrete answers.
- **Doc-consistency, not test-pinned.** The new wording in SPEC, MCP,
  HOW_TO_AGENT, and UBIQUITOUS_LANGUAGE is descriptive; the only
  test-pinned invariant is the existing
  `test_no_engram_imports_or_memory_capabilities_in_striatum`. If a
  future RFC introduces an injection-policy entry point, the
  augmentation-boundary regression must be extended (Decision 10 in
  RFC 0057 names this explicitly).
- **`docs/dogfood/`, `prompts/`, and `examples/` mention Engram many
  times.** Scope expressly excludes rewriting historical fixtures.
  Readers who land in `docs/dogfood/042/` or `prompts/P00X*` may see
  older framing; the ENGRAM_INCUBATION_CONTEXT pointer and the new
  UBIQUITOUS_LANGUAGE Distinctions bullet route them back to current
  product docs.
- **Shared parallel-workflow branch.** The worktree includes staged or
  unstaged changes from GH #9, #10, #15, and #16, plus frontend work,
  because all of these are running against the shared branch
  `striatum/gh-issues-parallel`. The operator finalizing GH #17 must
  decide whether to land GH #17 as its own commit (recommended:
  `git commit -- docs/issues/17/ docs/rfcs/0057-corpus-contract-v2.md
  docs/rfcs/README.md docs/SPEC.md docs/CLI_REFERENCE.md
  docs/HOW_TO_HUMAN.md docs/HOW_TO_AGENT.md docs/MCP.md
  docs/UBIQUITOUS_LANGUAGE.md docs/ENGRAM_INCUBATION_CONTEXT.md
  docs/ROADMAP.md docs/TODO.md`) or fold it into a combined parallel-
  branch landing.
- **`pyproject.toml` version not bumped.** Per scope, no source code
  changes mean no minor bump is required here. The operator-side
  finalize procedure should decide whether a `vX.Y.Z+1` patch release
  captures this doc consistency pass.
