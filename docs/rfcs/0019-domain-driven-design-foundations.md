# RFC 0019: Domain-Driven Design Foundations

Status: accepted
Date: 2026-05-08
Context:
`docs/UBIQUITOUS_LANGUAGE.md` (already a DDD signature),
`docs/SPEC.md` (state model, schemas, aggregates),
`docs/PRD.md` (product boundary),
`docs/DECISION_LOG.md` (D004, D005, D006, D007, D008, D009,
D020, D028 — the boundary decisions),
RFC 0002 (reviewer policy), 0003 (support ledgers), 0004
(action-item ledgers), 0005 (harness improvement proposals),
0010 (harness profiles), 0015 (skill bundles), 0018
(adversarial postures) — the running list of RFCs that all add
new *vocabulary* rather than new *plumbing*

## Problem

A reader meeting striatum for the first time can come to the
wrong conclusion about why it works. The CLI/service surface looks like
"a workflow runner with a database and verdicts." Most
operators have seen ten of those. They look at striatum, see
familiar parts (state, jobs, queues), and ask: *"what's the
magic?"*

If the answer they reach is "there is none — this is just a
workflow runner with extra ceremony," they will adopt it as a
glorified Make and miss the actual benefit. They will work
*around* the vocabulary instead of *with* it: marker files used
as state, prose used to advance jobs, ad-hoc shell scripts
making storage writes the runner doesn't see, reviews returned as
"looks good" instead of as `accept_with_findings` + a structured
finding artifact. The runner survives this for a while — it's
permissive at the storage level — and then a six-job workflow
with three reviewers and a `needs_revision` cycle melts down,
and the operator concludes the tool was the problem.

The actual answer is that striatum is **a domain-driven design
of workflow orchestration**. The entire vocabulary in
`docs/UBIQUITOUS_LANGUAGE.md` is not bookkeeping — it's the
*model*. The CLI verbs are the model's only legal mutators. The
schemas (workflow, work packet, artifact front matter) are the
model's grammar. The boundary decisions in `DECISION_LOG.md`
(D006, D009, D020, D028) are the *bounded context* — what
striatum models and what it deliberately doesn't.

This RFC documents that framing so a new reader can see what's
load-bearing and what isn't, and so future RFCs explicitly cite
their domain-modeling rationale rather than re-deriving it each
time.

This RFC is documentation only. It changes no behavior, no
schema, no defaults. It exists so the existing behavior stops
reading as accidental.

## Goals

- **Name the framing.** Add a top-level `docs/DDD.md` that
  describes striatum's bounded context, ubiquitous language,
  aggregate roots, value objects, domain events, and the
  daemon-method write-boundary invariant. This is the document a
  reader is pointed at when they ask *"why isn't this just a
  workflow runner with extra steps?"*
- **Map every existing concept to a DDD pattern.** The
  vocabulary already exists; the RFC's job is to label which
  pieces are aggregates (`run`, `session`, `job`, `lease`),
  which are value objects (`verdict`, `write_scope`,
  `harness_profile`, `posture`, `byline`), which are domain
  events (every row in the `events` table), and which are
  bounded-context boundaries (everything refused at the CLI
  surface or the workflow validator).
- **Make the cross-domain boundary explicit.** striatum models
  *coordination*. It deliberately does not model an artifact's
  *content*. The Markdown body of a finding is the agent's
  domain (writing); the verdict on it is striatum's domain
  (coordination). That split is currently implicit; the RFC
  documents it.
- **Give future RFCs a citation pattern.** When a new RFC adds
  a workflow field, an artifact kind, a verdict, or a posture,
  it can cite `docs/DDD.md § "Adding to the model"` instead of
  re-arguing the same ground. The discipline keeps the model
  coherent over time.
- **Surface the framing in the README.** A two-line callout
  under "What It Is For" — *"striatum is a domain-driven
  workflow runner: the vocabulary in `docs/UBIQUITOUS_LANGUAGE.md`
  is the model, not the documentation; the CLI verbs are the
  only legal mutations"* — so a reader notices the frame
  before they form a mental model from the surface.

## Non-Goals

- Refactoring code to fit DDD vocabulary more cleanly (e.g.,
  splitting modules into `domain/`, `application/`,
  `infrastructure/` directories). The current code organization
  is fine; this RFC is about how to *describe* it, not how to
  rearrange it.
- Adopting heavier DDD tactical patterns (CQRS, repositories
  per aggregate, separate read models, event sourcing). The
  current SQLite-as-state + events-as-log shape is already
  enough for V1; adding ceremony without need is the failure
  mode this RFC is trying to prevent in *operators*, and we
  shouldn't commit it ourselves.
- Renaming any existing terms. The vocabulary is stable as of
  v1.2.0; renaming would break the very point of a ubiquitous
  language.
- Adopting a particular DDD textbook's terminology over our
  own where they conflict. We use *value object* and *aggregate
  root* because they're standard names; we don't use
  *application service* because *CLI verb* is clearer for our
  audience.
- Translating `docs/UBIQUITOUS_LANGUAGE.md` into a DDD
  ontology format, RDF, OWL, or otherwise machine-checkable
  schema. Plain Markdown is the V1 surface.
- Replacing `docs/SPEC.md`. SPEC continues to be the
  implementation contract; DDD.md sits one level up and
  describes *why* SPEC looks the way it does.

## Proposal

A single new doc plus three small surface updates. No code.

### 1. `docs/DDD.md` (new, ~250 lines)

Sections:

#### Bounded context

What striatum models:

- The lifecycle of a *run*: prepare, branch, start, claim,
  complete, terminal.
- The *coordination* between sessions, jobs, and artifacts.
- The *gate* between an artifact existing and an artifact being
  acceptable (verdicts, postures, required postures, cycles).
- The *provenance* of every state transition (the `events`
  log).

What striatum deliberately does NOT model:

- The agent's reasoning (no transcripts, D028).
- The build's correctness (the runner does not run tests on
  the artifact body; reviewers do).
- The repository's deployment, packaging, or distribution.
  Striatum is a coordination layer; CI/CD is downstream.
- The agent CLI's internal state (supervisors send DEVNULL to
  agent stdout/stderr; the runner never parses agent output
  for state).
- The user's intent. The runner records decisions; it does not
  infer them.

The boundary is visible in the CLI surface: every legal
mutation passes through `striatum <verb>`, and every refusal
returns a stable exit code (3-9). If a feature wants to live
outside that boundary (telemetry, hosted service, transcript
capture, automatic commits), it lives outside striatum.

#### Ubiquitous language (canonical reference)

Pointer to `docs/UBIQUITOUS_LANGUAGE.md`. The RFC reasserts:

- **Every term in the vocabulary is load-bearing.** A
  reviewer's `accept` and `accept_with_findings` are not the
  same word; the runner treats them differently. A
  `coordinator` and an `operator` are not the same role; one
  sits inside the workflow and one drives it.
- **New features add to the vocabulary.** The right way to
  introduce concepts like *adversarial review posture* (RFC
  0018) or *agent skill bundle* (RFC 0015) is to give them a
  glossary entry first and a flag/field/schema second.
- **Code agrees with the vocabulary.** Class names, function
  names, parameter names, error messages, doctor checks, and
  CHANGELOG bullets all use the same words the glossary uses.
  Drift is a bug, not a stylistic choice.

#### Aggregate roots

Each row maps to an existing SQLite table:

| Aggregate | Table | Identity | Invariants the runner enforces |
|---|---|---|---|
| Run | `runs` | `run_id` | states `prepared → ready → running → {completed, failed, canceled}`; `branch_name` recorded once; `workflow_snapshot_id` immutable |
| Session | `sessions` | `session_id` | states `active → closed`; at most one active supervisor per session (RFC 0009); fresh-session policy (RFC 0002) |
| Job | `jobs` | `job_id` | states `pending → queued → claimed → running → {completed, failed, blocked, canceled, skipped}`; required artifacts before `complete` (RFC 0014); review verdict before downstream `complete` |
| Lease | `leases` | `lease_id` | one active lease per session per packet; lazy expiry; lease ownership required for every mutation |
| Work packet | `work_packets` | `packet_id` | one active packet per claimed message; immutable once issued |
| Artifact | `artifacts` | `artifact_id` | append-only (D008); path inside `write_scope.allowed_paths`; front matter schema-validated when present (RFC 0003/0004/0005) |
| Verdict | `verdicts` | `verdict_id` | one of `accept|accept_with_findings|needs_revision|reject`; references the source review job; (RFC 0018) carries posture |
| Blocker | `blockers` | `blocker_id` | severities `blocked` and `human_checkpoint`; payload metadata only, no agent prose (D028) |

Aggregates have *consistency boundaries*: a `run`'s state is
the runner's atomic unit at the SQLite level; a `BEGIN
IMMEDIATE` transaction crosses one aggregate at a time.

#### Value objects

Immutable, equality-by-value, no identity:

- `verdict` (`accept | accept_with_findings | needs_revision | reject`)
- `write_scope` (`allowed_paths`, `forbidden_paths`, `mode`)
- `harness_profile` (passthrough projection on the work packet)
- `byline` (`<role>-<model>-<ordinal>` lowercase string)
- `posture` (RFC 0018: `neutral | security | threat_model | …`)
- `adapter_constraint` + `enforcement_level`
  (`enforced | advisory_strict | advisory | unsupported`)
- `state-class` (Mermaid + ANSI palette key, RFC 0007/0016)

Value objects are constructed at validate time and never mutated
in flight. A finding's verdict is recorded once; "changing the
verdict" means recording a new verdict on a new attempt.

#### Domain events

The `events` table is literally a DDD-style event log:

- Every row is an immutable append.
- Every row carries `event_type`, `created_at`, the affected
  aggregate id, and a small structured `payload_json`.
- Reads (status, why, dashboard, evidence export) replay the
  log; mutations append to it.
- Subscribers (RFC 0012's SSE stream, future webhook adapters)
  observe the log; they don't observe the SQL state directly.

This is not "we happened to write events." It's the load-bearing
shape: the runner's read model is *derived* from events; the
SQL state is the materialized projection.

#### Daemon Methods As The Write Surface

D094/D104 supersede the original D006/D009 CLI-only framing. In DDD terms:

- The runner is an *application service* whose legal production
  invocations are daemon methods reached through approved local clients
  (CLI, MCP/chat tools, and the local web service).
- Direct live-state writes from outside the daemon are forbidden
  even when database permissions allow them; they bypass the
  invariant checks and break the model.
- Adapters (process, supervisor, web service, MCP wrapper) all
  go through `striatum.api.invoke` and then through the same
  CLI dispatch. There is one write path.

This is what makes the vocabulary load-bearing: a reviewer
cannot return `looks good` because the CLI doesn't accept that.
The vocabulary is enforced by *what the API will let you say*.

#### Adding to the model

When a new RFC introduces a concept:

1. **Glossary first.** Add an entry to
   `docs/UBIQUITOUS_LANGUAGE.md`. If the concept doesn't have
   a name yet, propose one in the RFC.
2. **Identify the pattern.** Is the new concept an aggregate
   root, a value object, or a domain event? RFCs cite which.
3. **Validator next.** If the concept is part of the workflow
   schema, the validator rejects unknown values. If it's a
   work-packet field, the build path emits it deterministically.
   If it's a verdict-time concept (RFC 0018 postures), the
   `submit-review` mutation records it.
4. **Surface in introspection.** `status`, `why`, `doctor`,
   `evidence export`, and the dashboard show the concept.
5. **CHANGELOG and DECISION_LOG cite the vocabulary entry.**

Concrete recent examples:

- RFC 0010 added `harness_profile` (value object) — glossary
  entry, validator rule, packet exposure.
- RFC 0015 added `skill bundle` and `skills manifest` (the
  first is a value object held outside the SQLite domain;
  the second is a value object on disk) — glossary, install
  path, doctor checks.
- RFC 0018 (proposed) adds `review posture` (value object) and
  `required_review_postures` (build-job invariant) — glossary,
  validator rule, packet exposure, completion gate.

#### What this isn't

- A justification for adding more abstractions.
- A reason to refactor existing code.
- An assertion that DDD is the only valid framing.

It's the framing the model already has. The RFC writes it down
so a reader can see it instead of reverse-engineering it.

### 2. README pointer (3 lines)

Under `## What It Is For`:

> striatum is a domain-driven workflow runner: the vocabulary
> in `docs/UBIQUITOUS_LANGUAGE.md` is the model, not just the
> documentation. The CLI verbs are the only legal mutations;
> the DDD framing in `docs/DDD.md` explains why.

(Link targets are bare in the example because `docs/DDD.md`
does not yet exist — this RFC creates it. When step 1 of the
implementation path lands, the README pointer should use
proper Markdown links.)

### 3. Index pointer

`docs/INDEX.md` adds a row for `DDD.md` under "Specifications
and decisions."

### 4. RFC template addendum (5 lines)

`docs/rfcs/README.md`'s template block gains an optional
section:

```text
## Domain Modeling

Identify which DDD pattern the new concept fits (aggregate,
value object, domain event, or boundary clarification). Cite
`docs/DDD.md § "Adding to the model"`.
```

Existing RFCs are not retro-edited; new ones include the
section.

## Acceptance Criteria

- `docs/DDD.md` exists at ~250 lines and contains the seven
  sections listed in step 1.
- README's `## What It Is For` cites DDD.md.
- `docs/INDEX.md` lists DDD.md.
- `docs/rfcs/README.md`'s RFC template includes a "Domain
  Modeling" section.
- `tests/test_doc_links.py` passes (the new file's relative
  links resolve).
- No source code changes. No schema changes. No version bump
  beyond what was current at the time the RFC lands.
- A reader who lands on the README and reads
  `What It Is For` → DDD.md sees the framing in under 5
  minutes and can re-derive *why* the vocabulary is the model.

## Open Questions

- **Should DDD.md graduate to a section in SPEC.md?** SPEC is
  the implementation contract; DDD is the modeling rationale.
  They live at different altitudes. V1 keeps them separate; if
  readers conflate them, V1.5 could merge.
- **Glossary maintenance discipline.** Today
  `docs/UBIQUITOUS_LANGUAGE.md` is hand-maintained. A V1.5
  follow-up could add a test that walks every striatum-specific
  word from CHANGELOG entries and asserts the vocabulary
  contains a definition. Out of scope here.
- **Adapting the framing for the agent skill bundle.** RFC 0015
  templates already cite the vocabulary indirectly (they list
  CLI verbs, mention front-matter kinds). A V1.5 rev of the
  bundle could ship a one-page "what striatum models" preface
  rendered from a snippet of DDD.md. Out of scope here.
- **Bounded context exports.** Some readers will want to feed
  DDD.md into an LLM as context. The doc is plain Markdown and
  fits in a single context window today (~250 lines). If it
  grows past ~600 lines, V1.5 should consider splitting (one
  doc per section).
- **Code organization.** Refactoring `src/striatum/` into
  `domain/` + `application/` + `infrastructure/` directories is
  an explicit non-goal here. If a future RFC argues for it,
  this RFC's framing is the *justification*, not the *trigger*.

## Implementation Path

V1 is a single PR:

1. Author `docs/DDD.md` with the seven sections.
2. Update README's `## What It Is For` with the 3-line
   pointer.
3. Update `docs/INDEX.md` with a new row.
4. Update `docs/rfcs/README.md` template with the "Domain
   Modeling" section.
5. Run `tests/test_doc_links.py` to confirm links resolve.

No version bump unless this lands alongside other behavior
changes.

## Relationship To Other RFCs

- **All accepted RFCs** retroactively benefit from the framing:
  every concept they introduced (worktree, supervisor, harness
  profile, skill bundle, posture, dashboard graph) is a DDD
  pattern instance.
- **RFC 0018** (focused adversarial review postures) is the
  cleanest "adds to the model" example for the doc to cite.
- **RFC 0015 step 4 (parser-walked verb tables)** — if and when
  it lands — is the example of a *non-domain* concern (CLI
  introspection plumbing) deliberately kept outside the model.
- **AGENTS.md and HOW_TO_AGENT.md** are how the model is
  exposed to operators and surrogates respectively. Both should
  cite DDD.md once they are next edited.
