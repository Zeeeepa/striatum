---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

# RFC 0017 V1 Build Handoff

author: implementer-codex-gpt-5.5-001

Date: 2026-05-08
Run: dogfood-010 / RFC 0017 (README And Docs Reorganization)
Decision: `accepted_with_follow_up` (V1_ACCEPTANCE; autonomous)
Version: `1.1.0`

V1 build slice (RFC 0017 steps 1+2+3) ships in one commit per the
synthesis. Documentation only; no behavior change.

## Files Changed

- **`README.md`** — rewritten from 1,012 lines to 125. Seven `##`
  section headers (`Status`, `Install`, `Quick Start (Human
  Operator)`, `Quick Start (Coding Agent)`, `What It Is For`,
  `Documentation Map`, `License`) in the synthesis-pinned order.
  The two quick-start sections are first-class headings; the
  agent-facing path is a top-level section, not a subsection of
  init.
- **`docs/GETTING_STARTED.md`** (new) — 15-minute walkthrough
  from "fresh target repo" to "running workflow"; explicitly
  forks "I am the operator" vs "I am setting up an agent."
- **`docs/HOW_TO_HUMAN.md`** (new) — long-form operator playbook;
  every CLI verb in the order you use it; lifted from the README's
  former § 1–11 + § Dashboard with light edits.
- **`docs/HOW_TO_AGENT.md`** (new) — long-form companion to the
  RFC 0015 skill bundle. Frames the workflow loop, work-packet
  shape, supervisor mode, and "what not to do" boundaries from
  the agent's perspective.
- **`docs/WRITING_WORKFLOWS.md`** (new) — workflow authoring
  guide; lifted from § "Writing Workflows" + § 2a + § 2b.
- **`docs/CLI_REFERENCE.md`** (new) — flat list of every CLI
  verb plus stable exit codes; header note declares `--help` is
  authoritative.
- **`docs/INDEX.md`** (new) — one-line summary per `docs/*.md`
  + repo-level files; the canonical index, slimmed to a 7-row
  pointer table in the README.
- **`docs/dogfood/HISTORICAL.md`** (new) — pointer file for the
  dogfood-001 / 003 / 004 / 005 incubation runs and the bootstrap
  tmux harness section that previously lived in the README.
- **`AGENTS.md`** — slimmed from 153 lines to 104. Replaces the
  inline verb-sequence recital with a pointer to
  `docs/HOW_TO_AGENT.md`, plus a short bullet list of the
  invariants (CLI-only, `write_scope`, byline match, front-matter
  validation, lazy lease expiry).
- **`tests/test_doc_links.py`** (new, 3 cases): relative-link
  resolution across the curated doc set; README line budget
  (≤ 250); both quick-start headings present.
- **`docs/rfcs/0017-readme-and-docs-reorganization.md`** — status
  flips to `accepted (V1)`.
- **`docs/rfcs/README.md`** — index reflects `accepted (V1)` plus
  the D062 reference.
- **`docs/DECISION_LOG.md`** — D062 row.
- **`docs/TODO.md`** — F10 row.
- **`pyproject.toml`** — version 1.0.0 → 1.1.0.
- **`src/striatum/__init__.py`** — `__version__` 1.0.0 → 1.1.0.
- **`CHANGELOG.md`** — `1.1.0` section.

## Verification

- `make lint` — clean.
- `make typecheck` — clean (51 source files).
- `make test` — 263 passed (260 baseline + 3 new doc-link tests).
- `wc -l README.md` → 125 (well under the 250 budget).
- `wc -l AGENTS.md` → 104 (well under the 200 ceiling).
- `grep -c '^## Behavior Model' README.md` → 0.
- `grep -c '^## Dogfood ' README.md` → 0.
- `grep -c '^## Quick Start' README.md` → 2 (both headings
  present, in canonical order).
- Manually rendered the README, GETTING_STARTED, HOW_TO_HUMAN,
  HOW_TO_AGENT, WRITING_WORKFLOWS, CLI_REFERENCE, and INDEX
  through GitHub-flavored Markdown locally; every relative link
  resolves.

## Notes For The Reviewer

- **No behavior change.** No CLI surface, no schema, no defaults.
  The only source change is the version pin and the
  `__version__` constant.
- **`tests/test_doc_links.py` scope.** Walks `README.md`,
  `AGENTS.md`, `CLAUDE.md`, `docs/*.md`, `docs/rfcs/*.md`,
  `docs/dogfood/HISTORICAL.md`, and `docs/dogfood/FRICTION_LOG.md`.
  Excludes per-run `docs/dogfood/<id>/` artifacts because those
  carry historical narrative that may legally reference removed
  README anchors.
- **Anchor validation deferred.** Markdown auto-anchor rules
  vary across renderers; V1 validates files only. A V1.5
  follow-up could lift anchors from a real GitHub render.
- **AGENTS.md trim.** The previous version recited the
  register-session → claim-next → ack → publish → complete
  verb sequence inline. After this RFC, AGENTS.md cites
  `docs/HOW_TO_AGENT.md` and keeps only the invariants the
  contributor needs to remember when *editing the source repo*.
  The runner-facing material lives in HOW_TO_AGENT.md.
- **Per-RFC subsections.** The synthesis flagged that the
  README's `### Process Adapter Completion Guarantees (RFC 0014
  V1)` paragraph might not be byte-for-byte in SPEC. After
  inspection, the SPEC's existing § "Process Supervision" and
  the per-RFC text in CHANGELOG cover the behavior; no SPEC
  append was required, and dropping the README paragraph causes
  no information loss.
- **External link breakage.** Anyone bookmarking
  `README.md#dogfood-001-usage` or `#command-reference` gets a
  404 after this PR. Acceptable per RFC 0017's Open Questions.

## Acceptance Mapping

- `wc -l README.md` ≤ 250 → 125 ✓
- README has the seven `##` section headers in synthesis order → ✓
- Six new `docs/*.md` files exist → ✓
- `docs/dogfood/HISTORICAL.md` exists → ✓
- `grep '^## Behavior Model' README.md` returns 0 matches → ✓
- `grep '^## Dogfood' README.md` returns 0 matches → ✓
- `make lint && make typecheck && make test` clean → ✓
- Test count = 263 → ✓
- `pyproject.toml` and `__init__.py` at `1.1.0` → ✓
- CHANGELOG has the `1.1.0` section → ✓
