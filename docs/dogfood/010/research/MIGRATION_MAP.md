---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

# RFC 0017 V1 — README + Docs Reorganization Research

author: researcher-codex-gpt-5.5-001

Date: 2026-05-08

## README inventory (1,012 lines, v1.0.0)

Section anchors and proposed homes after the reorganization:

| README section | Lines | New home | Notes |
|---|---|---|---|
| `# striatum` (intro) | 1-23 | README (kept) | Trim to two paragraphs. |
| `## Current Status` | 25-66 | README (kept, slim) → drop the bullet list; collapse to one paragraph + link to CHANGELOG | The 30-bullet "today's surface" list is ~40 lines of behavior duplication. |
| `## What It Is For` | 68-85 | README (kept) | One paragraph; existing prose is fine. |
| `## Behavior Model` | 87-241 | **DELETE from README** — every paragraph is already in `docs/SPEC.md` | Verified below. |
| `### Local State` (87-94) | | SPEC § "State Store" (line 21) | Already there. |
| `### Workflow Snapshots` | | SPEC § "Workflow Config" (line 55) | Already there. |
| `### Branch Gate` | | SPEC § "Branches And Commits" (line 425) | Already there. |
| `### Sessions And Work Packets` | | SPEC § "Sessions" (~line 200) | Already there. |
| `### Leases And Recovery` | | SPEC § "Recovery" (line 606) | Already there. |
| `### Artifacts` | | SPEC § "Artifacts" (line 311) | Already there. |
| `### Review Gates` | | SPEC § "Reviewer Policy" + cycles | Already there. |
| `### Worktree Isolation` | | SPEC § "Worktree Isolation" (line 978) | Already there. |
| `### Process Supervision` | | SPEC § "Process Supervision" (line 820) | Already there. |
| `### Introspection` | | SPEC § "Introspection / Doctor / Dashboard" (line 546) | Already there. |
| `## Installation` (242-261) | | README (kept) | Five-line `pip install` block; keep as-is. |
| `## Usage Guide` (263-275) | | `docs/HOW_TO_HUMAN.md` | New file. |
| `### 1. Initialize Runner State` | 278-303 | `HOW_TO_HUMAN.md § "Initialize"` | Keep the `--with-skills` paragraph — it's the agent-facing pivot. |
| `### 2. Validate Or Scaffold A Workflow` | 305-328 | `HOW_TO_HUMAN.md § "Author or validate a workflow"` | Cross-link to `WRITING_WORKFLOWS.md`. |
| `### 2a. Shape A Custom Run Scaffold` | 330-403 | `WRITING_WORKFLOWS.md` | Authoring material. |
| `### 2b. View A Rendered Graph Example` | 404-433 | `WRITING_WORKFLOWS.md` | Authoring material. |
| `### 3. Prepare A Run` | 435-442 | `HOW_TO_HUMAN.md` | Sequential. |
| `### 4. Confirm The Branch And Start` | 444-456 | `HOW_TO_HUMAN.md` | Sequential. |
| `### 5. Register A Session` | 458-469 | `HOW_TO_HUMAN.md` | Sequential. |
| `### 6. Claim And Acknowledge Work` | 471-495 | `HOW_TO_HUMAN.md` and `HOW_TO_AGENT.md` (mirror) | Common verb shape. |
| `### 7. Publish Artifacts And Complete Non-Review Work` | 497-512 | `HOW_TO_HUMAN.md` and `HOW_TO_AGENT.md` (mirror) | |
| `### 8. Submit Review Work` | 514-526 | `HOW_TO_HUMAN.md` and `HOW_TO_AGENT.md` (mirror) | |
| `### 9. Record Owner Decisions` | 528-542 | `HOW_TO_HUMAN.md` | Operator-only. |
| `### 10. Report A Blocker` | 544-579 | `HOW_TO_HUMAN.md` and `HOW_TO_AGENT.md` (mirror) | |
| `### 11. Inspect, Watch, And Export Recovery Evidence` | 581-608 | `HOW_TO_HUMAN.md` | Operator-only. |
| `### Dashboard` | 610-652 | `HOW_TO_HUMAN.md` § "Dashboards and graphs" | |
| `## Writing Workflows` | 654-692 | `WRITING_WORKFLOWS.md` | New file. |
| `## Dogfood 001 Usage` | 694-735 | `docs/dogfood/HISTORICAL.md` | New pointer file. |
| `## Dogfood 005 Usage` | 737-749 | `docs/dogfood/HISTORICAL.md` | |
| `## Dogfood 004 Usage` | 751-768 | `docs/dogfood/HISTORICAL.md` | |
| `## Dogfood 003 Usage` | 770-793 | `docs/dogfood/HISTORICAL.md` | |
| `## Harness Profiles (RFC 0010 V1)` | 795-823 | SPEC § "Harness Profiles (RFC 0010 V1)" (already at line 96) | Already there — README copy is duplication. |
| `### Local Web UI (RFC 0013 V1)` | 825-845 | SPEC § "Local Web UI (RFC 0013 V1)" (line 771) | Already there. |
| `### Process Adapter Completion Guarantees (RFC 0014 V1)` | 847-872 | SPEC § "Process Adapter" / SPEC has "Process Supervision" (line 820); RFC 0014's V1 narrative belongs in SPEC under Process Adapter | One small SPEC append needed if missing. |
| `## Bootstrap Tmux Harness` | 874-886 | `docs/dogfood/HISTORICAL.md` | Engram-incubation relic. |
| `## Command Reference` | 888-993 | `docs/CLI_REFERENCE.md` | New file. |
| `## Documentation Map` | 995-1011 | README (kept, slim) | Becomes the slim version of `docs/INDEX.md`. |

## Dedupe rule (verified)

Every `## Behavior Model` sub-section in the README has a 1:1 home
in `docs/SPEC.md`. The reorg's dedupe pass deletes the README copy
and adds an SPEC anchor link in the README's "Documentation Map"
table. No content is invented; no behavior change.

`### Process Adapter Completion Guarantees (RFC 0014 V1)` (README
line 847) is the one section that may not be byte-for-byte in the
SPEC today; the implementer should verify and, if missing, append
to SPEC § "Process Supervision" or a new SPEC § "Process Adapter
Completion Guarantees" subsection. CHANGELOG already covers the
behavior, so any append is summarizing existing prose.

## New file size targets

- `README.md`: ≤ 250 lines (proposed in RFC).
- `docs/GETTING_STARTED.md`: ≤ 200 lines. Picks one path; ends with
  pointers to either HOW_TO doc.
- `docs/HOW_TO_HUMAN.md`: ≤ 500 lines (lifts steps 1–11 + dashboard).
- `docs/HOW_TO_AGENT.md`: ≤ 350 lines (mirror of HOW_TO_HUMAN's
  claim-loop steps, framed for the agent + skill-bundle pointer).
- `docs/WRITING_WORKFLOWS.md`: ≤ 300 lines (lifts § 2a + 2b + the
  authoring section).
- `docs/CLI_REFERENCE.md`: ≤ 400 lines (lifts § "Command Reference").
- `docs/INDEX.md`: ≤ 100 lines (one-line summary per doc).

Total `docs/` line count grows by ~2,000 lines net (the README's
1,012 lines → 250 + the lifted material redistributed). The diff
is mostly *moves*, not *new prose*.

## Link-validation test scope

`tests/test_doc_links.py` walks `README.md` and `docs/**/*.md`
(excluding `docs/dogfood/<id>/` per-run artifacts that may legally
reference removed paths). For each Markdown link `[text](target)`:

- If `target` starts with `http://` or `https://`, skip.
- If `target` starts with `#`, skip (in-doc anchor).
- Otherwise, resolve `target` relative to the containing file and
  assert the path exists on disk.

Stretch: also walk reference-style links (`[text][ref]`) and
auto-link bare URLs. V1 picks inline links only; bare URLs are
already covered by the existing no-external-URL invariant in the
RFC 0015 test.

## Friction anticipated

- **AGENTS.md drift.** AGENTS.md contains its own copy of the
  workflow-claim-loop instructions. After this RFC, AGENTS.md
  should point at `docs/HOW_TO_AGENT.md` rather than reciting the
  same verbs inline. The implementer must keep AGENTS.md ≤ 200
  lines (project rule).
- **`docs/PRD.md` and `docs/DECISION_LOG.md`** — neither moves; both
  remain canonical. The new `docs/INDEX.md` lists them.
- **External links.** The `## Documentation Map` already exists at
  README line 995; the reorg replaces it with a slimmer table that
  cites the new doc names. Anyone bookmarking a README anchor like
  `#dogfood-001-usage` will get a 404; the RFC accepts this risk.
- **Test suite.** No source code changes are required for V1. The
  one new test (`tests/test_doc_links.py`) raises the count from
  260 to 261; all other tests stay green.

## Recommended order

1. Author `docs/GETTING_STARTED.md` (new prose, ~150 lines) and the
   five other new docs by lifting from the README.
2. Author `docs/INDEX.md`.
3. Slim the README to seven sections; remove the moved content.
4. Run the dedupe pass: every removed README paragraph that was
   already in SPEC stays once; the README's "Documentation Map"
   section reduces to a slim seven-row table.
5. Add `tests/test_doc_links.py`.
6. Update AGENTS.md to point at `docs/HOW_TO_AGENT.md`.
7. Bump pyproject + `__version__` + CHANGELOG to `1.1.0` and ship.
