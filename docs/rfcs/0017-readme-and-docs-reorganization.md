# RFC 0017: README And Docs Reorganization

Status: accepted (V1)
Date: 2026-05-08
Context:
`README.md` (1,012 lines as of `v1.0.0`),
`docs/SPEC.md` (1,041 lines),
`docs/UBIQUITOUS_LANGUAGE.md`,
`docs/DECISION_LOG.md`,
`docs/PRD.md`,
`docs/TODO.md`,
`docs/rfcs/0015-self-contained-agent-skills.md` (skill bundle is the
agent-facing onboarding path),
RFCs 0001–0016 (every V1 surface that needs to be documentable
without re-explaining)

## Problem

`README.md` has grown to 1,012 lines as we landed RFCs 0001–0016. It
now covers: what striatum is, who it is for, the full behavior model
(11 sub-sections — essentially a re-statement of `docs/SPEC.md`), an
11-step sequential usage walkthrough, four historical dogfood-001
through dogfood-005 sections, RFC-specific subsections for harness
profiles and the local web UI, a tmux bootstrap relic, and a
~100-line command reference. Two concrete failure modes follow:

1. **It's the wrong artifact for first contact.** A reader who lands
   on the GitHub page wants to answer two questions in 60 seconds:
   "what is this?" and "how do I try it?" Today they get a 1,012-line
   wall and bounce.
2. **It conflates two audiences.** A *human* (operator, contributor,
   evaluator) and a *coding agent* (Claude Code, Codex, Gemini)
   need very different starting points. The README treats them as
   one. The agent-facing answer should be "install the runner, run
   `striatum skills install`, then read the bundle"; that single
   path is buried in `### 1. Initialize Runner State` and only as a
   subsection.
3. **Content has drifted into the wrong file.** The behavior-model
   sections duplicate `docs/SPEC.md`; the dogfood-NNN sections are
   incubation history that belong under `docs/dogfood/`; the
   command reference is reference material that belongs in a
   dedicated CLI doc; the writing-workflows section is authoring
   material that belongs in its own doc. Each one is a maintenance
   burden — every RFC that touches behavior currently has to update
   both `SPEC.md` and `README.md`.

There is no "begin from scratch" path. The first thing a new
operator sees is the behavior model, not a "you have a fresh repo;
do these four things" guide.

## Goals

- **Cut README to ~250 lines.** Cover only: what striatum is, who
  it's for, install, two five-line quick starts (human and agent),
  and a documentation map. Everything else lives in `docs/`.
- **Make the human/agent split first-class.** The README points at
  `docs/HOW_TO_HUMAN.md` and `docs/HOW_TO_AGENT.md`. The two are
  short, parallel, and answer one question each: how a human drives
  the runner, and how a coding agent drives the runner. The
  agent-facing path is "install the runner, run `striatum skills
  install`, hand the agent the skill bundle, claim work."
- **Provide a from-scratch onboarding doc.** `docs/GETTING_STARTED.md`
  walks a new user from "I have a target repo and want to try this"
  to "I'm running a workflow." It is opinionated; it picks one path
  and follows it. It explicitly distinguishes "I'm a human running
  this myself" from "I'm setting this up so my coding agent can
  drive it."
- **Move reference material out of the README.** The 11-step
  sequential usage becomes `docs/HOW_TO_HUMAN.md`. The command
  reference becomes `docs/CLI_REFERENCE.md` (or stays in
  `--help`). The writing-workflows section becomes
  `docs/WRITING_WORKFLOWS.md`. Per-RFC subsections (harness
  profiles, local web UI, process adapter guarantees, dashboard
  graph) live in `docs/SPEC.md` only.
- **Retire incubation history from the README.** The four
  dogfood-NNN sections (001, 003, 004, 005) and the bootstrap tmux
  harness section move to `docs/dogfood/HISTORICAL.md` (a new
  pointer doc) or are deleted in favor of the existing
  `docs/dogfood/<id>/` material.
- **Single source of truth per concept.** Every behavior-model
  paragraph that exists in both `README.md` and `docs/SPEC.md`
  collapses to exactly one home (the SPEC). Cross-references go
  one direction: README → docs.
- **Zero behavior change.** This RFC is documentation only. No CLI
  surface, no schema, no defaults move. Every test that passes on
  `v1.0.0` still passes after the reorganization.

## Non-Goals

- Rewriting the SPEC. SPEC content is good; this RFC moves
  duplicated content into it, but does not re-author the existing
  sections.
- Producing tutorial videos, screenshots, or animated GIFs. Plain
  Markdown only.
- Documenting product roadmap. The TODO file already serves that
  purpose; this RFC does not move TODO content into the new docs.
- Generating a documentation site (mkdocs, Sphinx, ReadTheDocs).
  V1 stays on plain Markdown rendered by GitHub.
- Adding a CONTRIBUTING.md. AGENTS.md already serves the
  contributor / agent role; if a separate human-contributor guide
  is needed, it ships in a follow-up.
- Translating the docs. English only.

## Proposal

Three landable steps. Each can be its own PR.

### 1. Slim the README

The new README has exactly these sections, in this order:

```text
# striatum

(2-paragraph elevator pitch; existing intro condensed)

## Status
(one paragraph: v1.0.0, all V1 RFCs accepted, link to CHANGELOG)

## Install
(the existing Installation block, trimmed; pip install, editable
install, smoke command)

## Quick Start (Human Operator)
(5-line script: `striatum init`, `workflow validate`, `run prepare`,
`run start`, `dashboard --once`. Points at docs/HOW_TO_HUMAN.md.)

## Quick Start (Coding Agent)
(5-line script: `striatum init --with-skills claude_code`,
"now point your agent at the target repo and tell it to drive the
workflow"; one-paragraph explanation that the skill bundle teaches
the agent the rest. Points at docs/HOW_TO_AGENT.md.)

## What It Is For
(existing section; one paragraph trimmed.)

## Documentation Map
(table of doc filename → one-line summary. Subsumes the existing
"Documentation Map" but adds the new files.)

## License
(unchanged)
```

Target line count: ≤ 250.

The existing `## Behavior Model`, `## Usage Guide` (steps 1–11),
`## Writing Workflows`, `## Dogfood NNN Usage` (×4), per-RFC
subsections, `## Bootstrap Tmux Harness`, and `## Command
Reference` sections are removed from the README.

### 2. Author the new docs

Five new files under `docs/`:

| File | Audience | Contents |
|---|---|---|
| `docs/GETTING_STARTED.md` | New user, first 15 minutes | "From a fresh target repo to a running workflow." Picks one path. Distinguishes "I am the operator" from "I am setting up an agent to drive this." Ends with a link to either HOW_TO doc. |
| `docs/HOW_TO_HUMAN.md` | Human operator | The current README's 11-step usage guide, lifted verbatim with light edits. Adds a "common patterns" section: revision cycles, human checkpoints, recovery, dashboards, evidence exports. |
| `docs/HOW_TO_AGENT.md` | Coding agent (Claude Code / Codex / Gemini) | Mirror of HOW_TO_HUMAN, framed for the agent. Verbs: `register-session`, `claim-next`, `ack`, `heartbeat`, `publish-artifact`, `verdict` / `submit-review`, `complete`. Stresses that the agent loads the RFC 0015 skill bundle for current verb shape; this doc is the long-form companion. |
| `docs/WRITING_WORKFLOWS.md` | Workflow author | The existing `## Writing Workflows` README section, expanded: declared parallelism, write-scope rules, review policy fields, harness profiles, examples directory tour. |
| `docs/CLI_REFERENCE.md` | Anyone | The existing `## Command Reference` README section, kept as a copy-paste reference. Explicitly states it may lag; `striatum --help` is authoritative. |

Two existing docs gain a small update:

- `docs/dogfood/README.md` (new pointer file if absent, or updated
  if present) — gathers the dogfood-NNN history that lived in the
  README into one place. Each entry: dogfood id, RFC closed, decision
  link, BUILD_HANDOFF link.
- `docs/INDEX.md` (new) — a one-screen index of every doc under
  `docs/` with a one-line summary. The README's "Documentation Map"
  is a slim version of this index.

### 3. Cross-link and dedupe

After the new docs are in place:

- Every `## Behavior Model` paragraph that lived in both the README
  and `docs/SPEC.md` collapses to exactly one home (the SPEC). The
  README cites SPEC sections by anchor; never duplicates them.
- Per-RFC subsections currently in the README move into SPEC if
  they aren't already there. Anchors are stable.
- `docs/INDEX.md` lists every doc.
- `AGENTS.md` (project instructions) remains the contributor /
  agent rules and is updated to point at the new HOW_TO_AGENT.md
  rather than reciting the same workflow steps inline. AGENTS.md
  stays under 200 lines.
- Internal links use repo-relative paths (no `https://` URLs).

## Acceptance Criteria

- `wc -l README.md` ≤ 250.
- `README.md` contains the seven section headers listed in step 1
  (and only those), in that order.
- `docs/GETTING_STARTED.md`, `docs/HOW_TO_HUMAN.md`,
  `docs/HOW_TO_AGENT.md`, `docs/WRITING_WORKFLOWS.md`, and
  `docs/CLI_REFERENCE.md` exist.
- `docs/INDEX.md` exists and contains a row for every Markdown file
  under `docs/` (excluding the `docs/dogfood/<id>/` per-run
  artifacts).
- A grep for `## Behavior Model` returns one match (in `SPEC.md`),
  not two.
- A grep for the dogfood-NNN section headers in `README.md` returns
  zero matches.
- `make lint`, `make typecheck`, and `make test` are unchanged.
  Test count stays at 260; no test imports README content directly.
- A new test (`tests/test_doc_links.py`) walks every Markdown file
  under `README.md` and `docs/**/*.md`, extracts every relative
  link target, and asserts each target exists on disk. URLs
  (`http://`, `https://`) are not validated; only repo-relative
  links are checked.
- An additional test asserts the README contains both quick-start
  headings (`## Quick Start (Human Operator)` and `## Quick Start
  (Coding Agent)`) so the human/agent split cannot silently
  regress.
- Every RFC's `Context:` block stays valid: any RFC that referenced
  a moved README section is updated to reference the new home.
  This is bounded — most RFCs cite SPEC sections, not README
  sections.

## Open Questions

- **Do we need a separate CONTRIBUTING.md?** AGENTS.md is the
  contributor + agent file today. Splitting it into AGENTS.md
  (agent-facing) and CONTRIBUTING.md (human-contributor-facing)
  is a clean follow-up but is out of scope for V1 of this RFC. V1
  keeps AGENTS.md and adds a "How To Contribute" section to
  GETTING_STARTED.md.
- **Should the per-RFC subsections in README live in the SPEC or
  in their own files?** SPEC is already 1,041 lines. The proposal
  is "they live in SPEC because that's where every other behavior
  rule lives," with the trade-off that SPEC grows. An alternative
  is per-RFC docs under `docs/features/<rfc-id>.md`. V1 picks SPEC
  to avoid a third doc-tree shape; if SPEC grows past ~1,500
  lines, we revisit.
- **CLI_REFERENCE.md vs `--help`.** A copy-paste reference rots.
  An alternative is to drop CLI_REFERENCE.md entirely and tell
  readers to run `striatum --help`. V1 ships the reference as a
  convenience and explicitly labels it "may lag; `--help` is
  authoritative." A V1.5 follow-up could auto-generate it from the
  parser tree.
- **Quick-start screenshots / casts.** A `make demo` target plus
  a recorded asciicast would be the obvious next step. V1 stays
  text-only; recordings can be a follow-up RFC.
- **Migration of external links.** Any external page that linked
  to a `README.md` anchor (issue tracker, blog post, GitHub
  README badge) breaks. The risk is low — striatum is recent
  enough that the README anchors are not widely cited externally.
  V1 accepts the breakage.

## Implementation Path

V1 ships in three landable steps:

1. **Author the new docs.** Create
   `docs/GETTING_STARTED.md`, `docs/HOW_TO_HUMAN.md`,
   `docs/HOW_TO_AGENT.md`, `docs/WRITING_WORKFLOWS.md`,
   `docs/CLI_REFERENCE.md`, `docs/INDEX.md`. Each one is sourced
   from existing README content lifted verbatim, then lightly
   edited for the new context. No content is invented.
2. **Slim the README.** Replace the README with the seven-section
   structure described above. Every removed section is verifiably
   present in the new docs (a grep pass confirms).
3. **Add the link-validation test + dedupe.** Ship
   `tests/test_doc_links.py` with the relative-link assertion and
   the README quick-start-headings assertion. Run the dedupe pass
   against `SPEC.md` and any RFC that referenced a moved README
   section.

Each step is its own PR; the work can be parallelized once step 1
lands. RFC 0017 is "accepted" once steps 1–3 are on main.

## Relationship To Other RFCs

- **RFC 0015** — self-contained agent skills. The skill bundle is
  the agent-facing answer; HOW_TO_AGENT.md is the long-form
  companion that explains *why* the bundle exists and how to
  install / regenerate / debug it. The two do not duplicate each
  other; the bundle is operational, the doc is conceptual.
- **RFC 0007** — workflow visualization. `WRITING_WORKFLOWS.md`
  cites the graph commands as the authoring feedback loop.
- **RFCs 0012 / 0013** — local API and web UI. Both are mentioned
  in `HOW_TO_HUMAN.md` ("if you want to watch the run from a
  browser, `striatum serve --web`") and in `CLI_REFERENCE.md`.
- **AGENTS.md / CLAUDE.md** — project instructions that the
  Striatum source repo ships for its own contributors. After this
  RFC, AGENTS.md continues to govern *contributors to striatum*.
  HOW_TO_AGENT.md governs *agents driving striatum from a target
  repo*. The two never overlap.
