# RFC 0056 — Consumer-repo directory-structure opinions

**Status:** proposed (Phase A shipped; optional scaffold updates in Phase B)
**Scope:** V1.7 documentation + V1.8 `init --with-ddd-layout` extension
**Composes with:** RFC 0021 (`init --with-ddd-layout` scaffold —
likely host for the recommended layout), RFC 0053 (operator/principal
model — affects where principal-visible artifacts live), RFC 0054
(day-zero guide — links to the directory-structure recommendations).

## Background

A target repository orchestrated by Striatum accumulates files in
several locations:

- `.striatum/` — operational scratch (per RFC 0043: FIFOs, pidfiles,
  supervisor stdout, token cache). No workflow state — that's in
  Postgres.
- A `workflow.json` somewhere — Striatum has no enforced location.
- Per-job artifact outputs — paths declared in `workflow.json`'s
  `expected_artifacts[].path` and `write_scope.allowed_paths`. The
  workflow author picks; the runner accepts whatever is declared.
- Decision artifacts — usually `docs/DECISION_LOG.md` (per RFC 0021
  DDD scaffold) but the runner records decisions through the audit
  chain regardless of where the Markdown body lives.
- Dogfood / run records — Striatum's own repo uses
  `docs/dogfood/<NNN>/` by convention, but that is project-specific
  habit, not product policy.
- RFCs — `docs/rfcs/` per RFC 0021 again.

Striatum's brand is generic ("target repository, workflow fixture,
runner state, artifact, adapter" per AGENTS.md); it deliberately
does not enforce Engram-specific paths. But "no enforcement" has
shaded into "no opinion at all," and new arrivals don't know where
to put their workflow file or where they should expect artifacts
to land. RFC 0021 ships the DDD doc scaffold (`SPEC.md`, `PRD.md`,
`DECISION_LOG.md`, `UBIQUITOUS_LANGUAGE.md`, `DDD.md`,
`rfcs/README.md`, `rfcs/0001-template.md`) but says nothing about
the workflow file, artifact root, or dogfood-record convention.

The day-zero reader (RFC 0054) and the README marketing reader (RFC
0055) both benefit from a doc that says explicitly: *here is how to
lay out a target repository for Striatum, and here is why.*

## Goals

- Publish explicit, opinionated recommendations for target-repo
  layout — without making any of them mandatory.
- Cover the load-bearing decisions a new operator hits in the first
  week: workflow file location, per-workflow artifact root,
  decision-log location, RFC location, dogfood/run-record
  convention, gitignore expectations.
- Anchor the recommendations to existing primitives (RFC 0021 DDD
  scaffold, RFC 0034 workflow generator/template catalog, RFC 0043
  `.striatum/`-as-scratch).
- Identify which recommendations should be enforced by the
  `init --with-ddd-layout` scaffold versus left as documented
  conventions.

## Non-goals

- Adding runtime enforcement of any location. The workflow file
  format, paths, and artifact roots remain workflow-declared per
  RFC 0034. Convention lives in docs and the scaffold; the runner
  doesn't refuse non-conforming layouts.
- Renaming any existing path inside Striatum's own codebase or
  dogfood records. `docs/dogfood/<NNN>/` stays.
- Locking the recommendations to a single application domain.
  Striatum is generic; the recommendations must work for an RFC
  ledger, a code-change loop, a research synthesis, and an
  Engram-style dogfood corpus alike.

## Proposed shape

A new file at `docs/CONSUMER_REPO_LAYOUT.md` recommending:

1. **`.striatum/` — reserved for runtime scratch.** Committed to
   `.gitignore`. Never the workflow source of truth; never an
   artifact destination. Already enforced by `striatum init`; the
   doc just makes the contract explicit.
2. **`striatum/workflows/<name>.json` — recommended workflow
   home.** A `striatum/` top-level directory in the target repo
   that holds one or more workflow files plus optional
   per-workflow assets. Falls in line with RFC 0034's generator
   catalog defaults.
3. **`striatum/<workflow-name>/` or `docs/<workflow-name>/` —
   recommended artifact root.** Declared per workflow in
   `expected_artifacts[].path` / `write_scope.allowed_paths`.
   Pattern: one directory per workflow run-family so artifacts
   are easy to inspect or delete.
4. **`docs/DECISION_LOG.md` + `docs/rfcs/`** — already shipped by
   RFC 0021 `--with-ddd-layout`. Reaffirm here as the
   recommendation, not a one-time scaffold.
5. **`docs/dogfood/<NNN>/`** — striatum's own convention for runs
   that produce auditable historical records. Recommended for
   any project doing structured multi-phase runs; not enforced.
6. **`README.md` and `AGENTS.md` / `CLAUDE.md`** — project entry
   points; the latter loads agent instructions per existing
   conventions.
7. **`.gitignore` expectations** — `.striatum/` plus any
   per-workflow artifact-root directories the operator does not
   want committed.

The doc should include a small ASCII tree showing one
recommended layout end to end:

```
target-repo/
├── README.md
├── AGENTS.md / CLAUDE.md
├── docs/
│   ├── SPEC.md
│   ├── PRD.md
│   ├── DECISION_LOG.md
│   ├── UBIQUITOUS_LANGUAGE.md
│   ├── DDD.md
│   ├── rfcs/
│   │   ├── README.md
│   │   └── 0001-*.md ...
│   └── dogfood/
│       └── 001/ ...
├── striatum/
│   ├── workflows/
│   │   └── code-change.json
│   └── code-change/
│       └── <artifact outputs land here>
└── .striatum/  # gitignored — runtime scratch only
```

## Open questions

1. **Prescriptive vs. descriptive.** How strongly should the doc
   say "do this"? Calibrate against Striatum's generic-product
   stance. Recommend: "these are defaults; deviate when you have
   a reason."
2. **Scaffold extension.** Should `init --with-ddd-layout` grow
   to also create `striatum/workflows/`, `striatum/<workflow>/`,
   and update `.gitignore` for the workflow artifact roots? Or
   does the doc stay docs-only and scaffold extension wait?
3. **Per-workflow generator integration.** RFC 0034's workflow
   generator already writes `workflow.json` to a path. Should the
   generator's default path be updated to
   `striatum/workflows/<style>.json`?
4. **Artifact root naming.** `striatum/<workflow-name>/` keeps
   workflows visually grouped; `docs/<workflow-name>/` keeps
   reviewable outputs in `docs/` where reviewers already look.
   The recommendation should pick one default and explain the
   trade.
5. **Coexistence with existing repos.** A consumer repo that
   adopts Striatum mid-life already has its own conventions.
   Should the doc include a "migration / adoption-after-the-fact"
   section, or just describe the green-field shape?
6. **Engram and similar dogfood-heavy projects.** Their
   `docs/dogfood/<NNN>/` is dense and load-bearing. Should the
   doc separate "general consumer" recommendations from
   "dogfood-heavy" extensions?

## Phasing

- **Phase 0 (this RFC):** scaffold accepted; Open questions 1, 4,
  6 resolved.
- **Phase A:** write `docs/CONSUMER_REPO_LAYOUT.md` per the
  resolved recommendations. Includes the ASCII tree, per-section
  rationale, and a brief migration paragraph. Cross-link from
  `docs/INDEX.md`, the day-zero guide (RFC 0054), and the
  rewritten README (RFC 0055).
- **Phase B:** extend `init --with-ddd-layout` (RFC 0021) to
  optionally scaffold `striatum/workflows/`, `striatum/<workflow>/`,
  and `.gitignore` entries. Behind a flag (`--with-striatum-layout`?)
  so existing scaffolded repos don't churn. Open question 2 may
  defer this to a separate RFC if scope grows.
- **Phase C (optional):** update RFC 0034 workflow generator
  defaults if Open question 3 resolves toward
  `striatum/workflows/<style>.json`.

## Provenance

- 2026-05-14 operator session: project owner asked for "more
  explicit opinions about directory structure in the consumer
  repo."
- Companion RFCs in the same wishlist: RFC 0054 (day-zero guide),
  RFC 0055 (marketing README). All three were proposed together;
  this scaffolds the third.
- Prior art: RFC 0021 (DDD layout scaffold) supplies the
  `docs/` skeleton; RFC 0034 (workflow generator) supplies
  workflow-file pathing; RFC 0043 (Postgres + daemon) fixes
  `.striatum/` as scratch-only.
