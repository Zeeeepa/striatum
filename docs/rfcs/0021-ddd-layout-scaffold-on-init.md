# RFC 0021: DDD Layout Scaffold On Init

Status: accepted (V1+V1.5)
Date: 2026-05-09
Context:
RFC 0019 (DDD foundations, accepted, D067) — `docs/DDD.md` names
the DDD framing the runner already has,
RFC 0015 (self-contained agent skills, accepted V1+step 3) —
established the "ship structure into the target repo" pattern
without shipping content,
`src/striatum/cli/dispatch.py` (`init` command — currently only
creates `.striatum/`),
`src/striatum/cli/parser.py` (`init [--with-skills <profile>]`
shape),
`docs/UBIQUITOUS_LANGUAGE.md` / `docs/SPEC.md` /
`docs/DECISION_LOG.md` / `docs/DDD.md` / `docs/rfcs/` (the doc
shape striatum's own repo uses to keep its DDD model coherent)

Current status note (2026-05-18): RFC 0043/D094 replaced repo-local
SQLite with daemon-owned PostgreSQL and `.striatum/` is operational
scratch only. RFC 0021 remains the DDD document scaffold; RFC 0056
adds the separate directory-only `striatum` layout scaffold.

## Problem

A reader who lands on a fresh striatum-managed repo and types
`striatum init` ends up with operational `.striatum/` scratch and
nothing else. The runner is now wired up to the daemon boundary; the
*model* the runner expects to coordinate against is not.

This is the failure mode RFC 0019 is meant to head off.
Striatum's whole bet is that the vocabulary in
`docs/UBIQUITOUS_LANGUAGE.md` is *load-bearing* — the verdict
enum, the artifact-kind set, the boundary between coordination
(striatum's domain) and content (the agent's domain). RFC 0019
documents that framing for striatum's *own* repo. It does not
help the operator who just adopted striatum into a target repo
that has none of the supporting documents and is now staring at
an empty editor wondering what to write where.

In practice this manifests three ways:

1. **No anchor for the ubiquitous language.** A new operator
   has nowhere local to put their project's glossary. They
   either re-derive striatum's vocabulary in their own words
   (drift) or skip the document entirely (the model becomes
   tribal knowledge). Neither preserves the load-bearing
   property.
2. **No anchor for the bounded context.** Without a
   target-repo `SPEC.md` / `DECISION_LOG.md` / `DDD.md`,
   product or architecture decisions land as PR descriptions
   and chat scrollback. RFC 0019's "the boundary is visible at
   the CLI surface" only works if the boundary is *also*
   visible in the docs the operator updates.
3. **No example RFC.** The RFC pattern (proposed →
   accepted, status header, sections) is a small but real
   barrier to first-time use. New operators see striatum's own
   `docs/rfcs/` and assume RFCs are striatum-specific
   ceremony, not a tool they're meant to use for their *own*
   product.

The existing scaffolding precedent — RFC 0015's `striatum
skills install` — already solves the structurally identical
problem for *agent-facing* docs: ship the structure, leave the
substance to the project. RFC 0021 extends that pattern to the
*human-facing* DDD layout.

## Goals

- **Add `striatum init --with-ddd-layout`** that scaffolds the
  human-facing DDD documents into the target repo. Default
  off; opt-in mirrors `--with-skills`.
- **Ship template stubs that name the model, not the
  content.** Each generated file contains exactly the
  scaffolding RFC 0019 says is load-bearing — section headers,
  the boundary contract, a "Why this file exists" preface — and
  leaves substantive content as TODO markers the operator
  fills in.
- **Cover the seven canonical files.** `docs/SPEC.md`,
  `docs/PRD.md`, `docs/DECISION_LOG.md`,
  `docs/UBIQUITOUS_LANGUAGE.md`, `docs/DDD.md`,
  `docs/rfcs/README.md`, and `docs/rfcs/0001-template.md`. All
  paths repository-relative. Existing files are never
  overwritten.
- **Stay opt-in and idempotent.** `striatum init` with no flag
  preserves today's flow byte-for-byte. Re-running with the
  flag is a no-op when files already exist (per-file refusal
  with the `skipped` reason in the JSON envelope).
- **Document the scaffold's intent.** The generated files cite
  RFC 0019 / RFC 0021 in their "Why this file exists" preface
  so future readers understand they're starting from a
  template, not from prose.
- **Zero coupling to project-specific terminology.** The
  scaffold uses generic terms (target repo, runner state,
  decision row); the operator localizes to their domain.
- **Composable with `--with-skills`.** A first-time operator
  can `striatum init --with-skills claude_code
  --with-ddd-layout` and end up with both the agent-facing and
  human-facing docs in one command.

## Non-Goals

- **Generating content.** The scaffold lays down section
  headers and the boundary contract, not domain prose. The
  runner does not know what the operator is building; it
  cannot author their SPEC.
- **Mandating the layout.** Operators who want a different
  doc shape (mono-doc, ADR-only, no separate `DDD.md`) skip
  the flag. The scaffold is a starter kit, not a contract.
- **Validating the layout.** A future doctor check could warn
  when a striatum-managed repo lacks any of the seven files,
  but that's out of scope for V1; we ship the scaffold first
  and learn from operator behavior whether the warning is
  wanted.
- **Translating striatum's own `docs/`** into the scaffold
  source. The scaffold is hand-authored Markdown templates
  living in `src/striatum/scaffold/templates/ddd_layout/`,
  not derived from striatum's own docs. (Striatum's repo is
  one specific instantiation of the pattern; the scaffold
  ships the *pattern*, not striatum's project content.)
- **Versioning the scaffold separately.** The templates ship
  with the runner; bumping the runner version bumps the
  template content. No template-only releases.
- **A dedicated `striatum scaffold` verb.** `init
  --with-ddd-layout` is enough; a top-level verb would imply
  re-runnability and template selection beyond V1's scope.
  V1.5 may revisit if multiple layouts (ADR-only, etc.) ship.

## Proposal

V1 ships in two landable steps. Each can be its own PR.

### Step 1. Template tree under `src/striatum/scaffold/`

A new package directory `src/striatum/scaffold/` containing:

- `__init__.py` — registers the layout name and template
  search path.
- `templates/ddd_layout/` — the seven Markdown templates,
  shipped via `[tool.setuptools.package-data]` in
  `pyproject.toml` (mirrors the existing `striatum.skills`
  packaging).

The templates use `.md.tmpl` extensions so a future
parameter-substitution layer (V1.5) can add placeholder
expansion without touching V1's plain-copy semantics. V1
performs literal copy: the `.tmpl` suffix is stripped on
write but the body is unchanged.

The seven templates and their target paths:

| Template | Target path | Purpose |
| --- | --- | --- |
| `SPEC.md.tmpl` | `docs/SPEC.md` | Implementation contract: what the project does, what it explicitly doesn't, what state it owns. Section headers + boundary callout + "Why this file exists" preface. |
| `PRD.md.tmpl` | `docs/PRD.md` | Product boundary: what the project is for, what it isn't, who the audience is. Three sections: Audience, In scope, Out of scope. |
| `DECISION_LOG.md.tmpl` | `docs/DECISION_LOG.md` | One-row-per-decision table with the four-cell shape from striatum's own log (D055+ tight form): Decision · Context · Consequence · Revisit. Header explains the cell budget. |
| `UBIQUITOUS_LANGUAGE.md.tmpl` | `docs/UBIQUITOUS_LANGUAGE.md` | Glossary table: Term · Definition. Header cites RFC 0019: "every term here is load-bearing; rename via decision row, not by edit." |
| `DDD.md.tmpl` | `docs/DDD.md` | The seven-section DDD framing from RFC 0019 (bounded context, ubiquitous language, aggregate roots, value objects, domain events, write-surface invariant, adding-to-the-model), with each section's body replaced by a single TODO sentence the operator fills in. |
| `rfcs/README.md.tmpl` | `docs/rfcs/README.md` | RFC index table + the template block from striatum's own `rfcs/README.md`. The template block is verbatim; the index table is empty with a "TODO: add RFC 0001 row" marker. |
| `rfcs/0001-template.md.tmpl` | `docs/rfcs/0001-template.md` | A copy-pasteable RFC skeleton: status, date, context, problem, goals, non-goals, proposal, acceptance criteria, open questions, implementation path, domain modeling. |

Each template's first line is an HTML comment:

```html
<!-- Generated by `striatum init --with-ddd-layout` (RFC 0021).
     Edit freely; this file is not regenerated. -->
```

This is the only marker. There is no front-matter generation
key, no checksum, no upgrade machinery — V1 treats the
templates as starter content the operator owns from the moment
of generation.

### Step 2. CLI wiring

`src/striatum/cli/parser.py`:

```python
init.add_argument(
    "--with-ddd-layout",
    action="store_true",
    help=(
        "After init, also scaffold the DDD-shaped human-facing "
        "doc layout (docs/{SPEC,PRD,DECISION_LOG,UBIQUITOUS_"
        "LANGUAGE,DDD}.md, docs/rfcs/) into the target repo. "
        "Existing files are preserved. RFC 0021."
    ),
)
```

`src/striatum/cli/dispatch.py` (`init` branch):

```python
if args.with_ddd_layout:
    init_result["ddd_layout"] = scaffold_ddd_layout(
        repo, force=False, dry_run=args.json and not args.write
    )
```

`scaffold_ddd_layout` lives in `src/striatum/scaffold/__init__.py`:

```python
def scaffold_ddd_layout(repo: Path, *, force: bool = False,
                       dry_run: bool = False) -> dict:
    """Copy the ddd_layout templates into <repo>/docs/.

    Returns a JSON-serializable envelope:

        {
          "layout": "ddd",
          "files": [
            {"path": "docs/SPEC.md", "status": "created"},
            {"path": "docs/DECISION_LOG.md", "status": "skipped",
             "reason": "exists"},
            ...
          ]
        }
    """
```

Status values: `created`, `skipped` (file exists; never
overwrites without `--force`, and `--force` is V1.5),
`error` (only on filesystem-level failure). The envelope is
the same shape `striatum skills install` returns.

The dispatch path runs *after* `.striatum/` initialization,
so partial failure of the scaffold leaves a working
runner-state directory. If any scaffold file errors, the
overall `init` exit code is 1 (operational failure), not 4
(invariant violation) — the runner state initialized
successfully; the scaffold is auxiliary.

### Composability with `--with-skills`

```bash
striatum init --with-skills claude_code --with-ddd-layout
```

Order of operations: `.striatum/` first, skills second,
scaffold third. Each step's envelope nests under its own key
in the overall `init` JSON output:

```json
{
  "repo": "/path/to/target",
  "state_initialized": true,
  "skills": {"profile": "claude_code", "files": [...]},
  "ddd_layout": {"layout": "ddd", "files": [...]}
}
```

Any step's failure does not roll back earlier steps —
`.striatum/` initialization is the only step that *must*
succeed for the runner to be useful; everything else is
auxiliary. Ordering matches the `--with-skills` precedent.

## Acceptance Criteria

- `striatum init` (no flag) produces output byte-identical to
  v1.6.0 — no scaffold files written, no envelope key added.
- `striatum init --with-ddd-layout` in an empty target repo
  creates exactly the seven files, each with the
  RFC 0021 generation comment as its first line.
- Re-running `striatum init --with-ddd-layout` after the first
  invocation produces an envelope where every file's status is
  `skipped` with `reason: "exists"`. No file content changes.
- `striatum init --with-ddd-layout` in a repo where one of the
  seven files already exists creates the six missing files and
  reports the seventh as `skipped`. No prompt; no
  interactive flow.
- Generated files pass `tests/test_doc_links.py` when adopted
  into a target repo (the templates' relative links resolve
  intra-template).
- `striatum init --with-skills claude_code --with-ddd-layout`
  produces both envelopes nested under their own keys; skills
  scaffolding is unchanged from v1.6.0.
- The scaffold templates ship in the wheel — `pip install
  striatum && python -c "import striatum.scaffold"` succeeds,
  and the seven `.md.tmpl` files are discoverable via
  `importlib.resources.files`.
- Tests at `tests/test_scaffold_ddd_layout.py` cover: empty-
  repo creation (all seven files), partial-overlap repo
  (only-missing files created), idempotency (second run is
  pure `skipped`), filesystem-error path (exit code 1, not 4),
  envelope shape, generation-comment presence, and the
  composability path with `--with-skills`.
- `tests/test_cli_mvp.py` adds a smoke test that
  `striatum init --with-ddd-layout` exits 0 and creates the
  seven files in the temporary target repo.

## Open Questions

- **Should the scaffold register a doctor check (V1.5)?** A
  check `target_repo_missing_ddd_layout` could warn when a
  striatum-managed repo lacks `docs/SPEC.md` and friends, but
  that pre-supposes the layout is mandatory. V1 does not
  warn; we learn from operator behavior whether the warning
  is wanted.
- **Template parameter substitution.** The `.md.tmpl`
  extension reserves room for `{{project_name}}`,
  `{{audience}}`, etc. V1 performs literal copy. V1.5 could
  add a `--with-ddd-layout=interactive` mode that prompts
  for the placeholders.
- **Multiple DDD layouts.** ADR-only repos, mono-doc repos, and
  large-team-with-OWASP-mappings repos have different ideal DDD
  document shapes. RFC 0021 still ships one DDD profile (the
  seven-file shape). RFC 0056 adds a separate `striatum` directory
  scaffold rather than a second DDD profile. For V1, "if you don't
  want this layout, don't pass the flag" is sufficient.
- **Update mechanism.** What happens when a future striatum
  release adds an eighth canonical document? V1 says "no
  upgrade path; the operator owns the files from generation
  onward." V1.5 could add a `striatum scaffold sync` verb
  that proposes additions but never overwrites.
- **Composability with `striatum skills install` invoked
  later.** Today the standalone verb works against a repo
  that already has `.striatum/`. If the operator runs
  `striatum init` first and `striatum skills install` later,
  should the latter also gain a `--with-ddd-layout` flag, or
  should the scaffold remain `init`-only? V1 keeps the
  scaffold init-only — it's first-time setup, not a recurring
  operation.
- **Front-matter on the templates.** The DECISION_LOG and
  UBIQUITOUS_LANGUAGE templates currently use a Markdown
  table with no front matter. RFC 0019 says "every artifact
  the runner reads has structured metadata." These files
  aren't artifacts in the runner's domain (the runner doesn't
  read them); they're target-repo documentation. V1 leaves
  them metadata-free.
- **Conflict with VCS-tracked existing scaffolding.** Some
  repos use cookie-cutter or copier-style template generators.
  V1 doesn't try to integrate; the operator who already has
  a templating layer skips the flag.

## Implementation Path

V1 ships in the two steps above, in order.

1. **Step 1 (templates + package data):** Author the seven
   `.md.tmpl` files under
   `src/striatum/scaffold/templates/ddd_layout/`. Wire
   `[tool.setuptools.package-data]` so they ship in the
   wheel. `src/striatum/scaffold/__init__.py` exposes
   `scaffold_ddd_layout(repo, *, force, dry_run) -> dict`
   using `importlib.resources` to read template bytes. Tests
   for template discovery and envelope shape.
2. **Step 2 (CLI wiring):** `parser.py` adds the
   `--with-ddd-layout` flag; `dispatch.py` calls
   `scaffold_ddd_layout` after `.striatum/` init. Tests for
   the empty-repo path, partial-overlap path, idempotency,
   composability with `--with-skills`, and the
   filesystem-error path.

The PR list mirrors RFC 0015's V1 (template package + skills
verb) — this is the same shape against the human-facing doc
layout instead of the agent-facing skill bundle.

## Domain Modeling

This RFC adds two value objects to the model:

- **scaffold layout** (RFC 0021: `ddd`, RFC 0056: `striatum`) —
  a named filesystem layout the runner can write or report for a
  target repo. Like `harness_profile` (RFC 0010) and `skill bundle`
  (RFC 0015), the scaffold layout is constructed at install time and
  never mutated in flight.
- **scaffold status** (`created | skipped | overwritten | error`
  plus dry-run `would_*` forms) — the per-target outcome reported
  in scaffold envelopes. RFC 0021 V1 originally used the same
  `created | skipped | error` shape as RFC 0015 skill-install
  statuses; V1.5 and RFC 0056 extend the vocabulary for force,
  dry-run, and directory-only scaffolds.

The runner's bounded context is *unchanged*: the scaffold
templates are written *into* the target repo's documentation
domain. Once the files are on disk, they belong to the operator,
not the runner. The runner does not read them, does not validate
them at runtime, does not refuse to operate when they're absent.
This is the same boundary RFC 0015 holds for the agent skill
bundle — striatum is a coordination layer, not a documentation
authority.

Cite: `docs/DDD.md § "Adding to the model"` —

> *"glossary first, identify the pattern, validator next,
> surface in introspection, CHANGELOG and DECISION_LOG cite
> the vocabulary entry."*

Per that sequence:

1. **Glossary** — add `scaffold layout` and `scaffold file
   status` to `docs/UBIQUITOUS_LANGUAGE.md`.
2. **Pattern** — both are value objects; documented above.
3. **Validator** — `--with-ddd-layout` accepts no value (it's
   a flag); the layout name is hard-coded to `ddd_layout` in
   V1. V1.5's `--ddd-layout-profile` would gain a closed-set
   validator.
4. **Introspection** — the `init` envelope's
   `ddd_layout.files[]` block is the introspection surface.
   `striatum status` does not surface scaffold state because
   the runner doesn't track it after generation.
5. **Citations** — CHANGELOG and DECISION_LOG cite the
   glossary entries when the RFC lands.

## Relationship To Other RFCs

- **RFC 0019** (DDD foundations, accepted) — RFC 0021 is the
  install-time complement. RFC 0019 names the framing
  striatum's own repo has; RFC 0021 ships that framing as a
  starter kit for target repos.
- **RFC 0015** (self-contained agent skills, accepted) — same
  scaffolding pattern, different audience. RFC 0015 ships
  agent-facing files into `.claude/skills/` etc.; RFC 0021
  ships human-facing files into `docs/`. The `init`
  composability path layers them deliberately.
- **RFC 0017** (README + docs reorganization, accepted) —
  RFC 0017 split striatum's own docs into the seven-file
  shape. RFC 0021 generalizes that shape to a template
  others can adopt.
- **RFC 0006** (SQLite migrations) — explicitly *not*
  followed for templates. RFC 0006's "forward-only,
  PRAGMA user_version" model assumes the runner owns the
  artifact; for scaffold templates, the operator owns
  the file from generation onward, so no migration
  machinery applies.
- **RFC 0014** (process-adapter completion guarantees,
  accepted) — irrelevant; the scaffold runs in the
  operator's `init` shell, not under an adapter.
