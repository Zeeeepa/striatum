# RFC 0053 — Human principal as escalation-only role + terminology truing

**Status:** proposed (doc-side fixes land with this RFC; schema rename
deferred)
**Scope:** V1.7 prose + V1.8/V2 schema and CLI sweep
**Closes (partially):** the ambient "human operator" framing baked into
`docs/SPEC.md`, `docs/GETTING_STARTED.md`, `docs/HOW_TO_HUMAN.md`, and the
remaining `human` references in `docs/UBIQUITOUS_LANGUAGE.md` partially
softened in [fb0175c](https://github.com/halbritt/striatum/commit/fb0175c).

## Background

Striatum's docs evolved from a single-keyboard human-operator model in
which a person ran every CLI verb by hand. Two shifts invalidated that
model:

1. **RFC 0015 skill bundles + RFC 0040 MCP dogfood harness** made the
   agent CLI the practical operator. In every shipped dogfood
   (028-056) the operator was an agent (Claude Code / Codex / Gemini)
   and humans intervened only on stalls and overrides.
2. **D094 / RFC 0043** made the daemon a hard prerequisite, and the
   V1.41 burndown verbs (`recovery auto-publish`, `inbox`, `byline`)
   plus RFC 0046 lane evidence guard closed the remaining
   "human-must-do-this" gaps as either AI-self-resolvable or
   operator-on-behalf overrides with audit trails.

What is left for the human is the residue: situations where the AI
operator cannot make progress and needs an authority figure to step in.
The product needs to name that role and stop pretending the human is the
default driver.

## Principle

**The human principal exists only to resolve unresolvable blockers or
decisions.** Routine claim / ack / publish / verdict / complete is the
AI operator's job. The principal is the authority of last resort and
does not drive normal runs.

The principal:

- Sets initial goals and accepts/rejects final decisions.
- Resolves declared blocker classes the AI operator cannot resolve.
- Resolves AI-self-declared escalations (the AI publishes an
  `escalation` artifact when it judges itself stuck and no declared
  class fits).
- Has the **same CLI surface** as the AI operator. The role is bounded
  by *function*, not by *interface*; nothing prevents a human from
  calling any CLI verb, but doing so for routine work is outside the
  role.

The principal does NOT:

- Confirm branches (the AI operator confirms via the same CLI verb;
  "human confirmation" is historical wording).
- Drive ordinary review/revision cycles (the AI operator's reviewer
  roles handle this; the principal sees only stalemates).
- Author artifacts on behalf of the AI as a default (the existing
  `--allow-no-process-execution --override-rationale` path remains for
  exceptional cases, audit-trailed per RFC 0046).

## Goals

- Name the role and define its scope on record so downstream docs can
  lean on it.
- Realign the doc surface to AI-operator-as-default and
  principal-as-escalation:
  - Update `docs/SPEC.md` operator-section prose (no behavioral change).
  - Restructure `docs/GETTING_STARTED.md` to lead with the AI-operator
    path; demote the human-driven path to "if you are the principal
    resolving an escalation."
  - Narrow `docs/HOW_TO_HUMAN.md` to an escalation playbook;
    cross-link `docs/HOW_TO_AGENT.md` for the full operator playbook.
- Define escalation triggers concretely (declared blocker classes plus
  AI-self-declared `escalation` artifacts).
- Scope the workflow.json schema-field rename
  (`human_checkpoint` → likely `escalation_checkpoint`) and the CLI
  prompt-string sweep as deferred follow-ups (next workflow-schema
  version bump).

## Non-goals

- Replacing the AI coordinator role (D004 / D005) or any in-workflow
  session vocabulary.
- Adding new capability classes; escalation moves use existing
  `write` / `review` capabilities.
- Defining the `escalation` artifact kind's full schema; that lands in
  a follow-up design RFC alongside the workflow-schema bump.
- Changing run-state-machine semantics in this commit. The
  `waiting_human` run state becomes vocabulary-misleading but stays as
  a state name until the schema bump (deferred follow-up).

## Validation

The principle is consistent with shipped reality:

- **AI-as-default is what dogfoods actually do.** Every shipped
  dogfood under `docs/dogfood/` had an AI operator. Humans intervened
  on stalls (RFC 0051 auto-finalize was added because the operator
  stall pattern was so common) and overrides (D095–D102 cycle-exhaustion
  decisions are operator-recorded). Both intervention shapes match this
  RFC's escalation surface.
- **RFC 0046 + V1.41 burndown verbs closed the "human-must-do-this"
  gaps.** What is left is genuinely authority-bounded — goal-setting,
  contradiction-resolution, stalemate — not a general-purpose "the human
  just does it better" residual.
- **RFC 0052 committee stalemate composes here.** A stalemate ruling
  from `arbitration_ruling` becomes an escalation that surfaces in the
  principal's inbox via the same path as any other escalation. The
  two RFCs compose without further work.

What this RFC does NOT validate:

- That the human principal is reachable in practice. If the AI is
  long-running and the principal is asleep, the escalation queues until
  someone looks. Async-by-default; this RFC does not propose paging,
  notification rails, or SLAs.
- That the schema-field rename is worth a breaking workflow-schema
  bump. That is a separate decision at the next version cut.

## Escalation triggers

Both shapes route to the principal's inbox:

**1. Declared blocker classes.** A closed set of blocker `kind` values
that the runner refuses to auto-resolve and that surface to the
principal:

- `ambiguous_goal` — the workflow's stated outcome admits multiple
  valid interpretations.
- `missing_authority` — the AI operator lacks delegated authority for
  a decision (scope expansion, third-party dependency adoption,
  license choice, etc.).
- `contradicting_decisions` — prior decisions in
  `docs/DECISION_LOG.md` conflict on the current question.
- `no_available_reviewer_lane` — required reviewer postures cannot be
  filled by the configured lanes.
- `committee_stalemate` — an RFC 0052 `arbitration_ruling` with
  `move_type: declare_stalemate`.
- `override_required` — a decision requires explicit override of a
  prior accepting verdict.

The exact kind enumeration is closed; new kinds need an RFC. The AI
operator cannot add to the set ad hoc.

**2. AI-self-declared escalation.** When no declared class fits but
the AI judges itself stuck, it publishes an `escalation` artifact (kind
to be defined in the follow-up schema RFC) carrying the same fields a
blocker would plus a free-text `reasoning`. The principal sees these
in the inbox alongside declared escalations.

Both shapes surface through the same surfaces (`striatum inbox`,
dashboard, web UI). The principal resolves either by recording a
decision (`striatum decision record`) or by resolving the blocker
(today: `striatum recovery resume` / operator-on-behalf override;
future: a dedicated `striatum escalation resolve` verb, deferred).

## Interaction with existing surfaces

- **`docs/UBIQUITOUS_LANGUAGE.md`** — adds `human principal` as a
  defined term. Existing `human`-tinged terms (`branch confirmation`,
  `human checkpoint`, `operator surrogate`) were softened in
  [fb0175c](https://github.com/halbritt/striatum/commit/fb0175c); this
  RFC validates that direction.
- **`docs/SPEC.md`** — prose updates only. The operator section
  reframes the default as agent CLI; the branch-confirmation section
  drops "human" from the gate description; the byline section keeps
  `author: operator` as the unattested form (RFC 0026 unchanged).
- **`docs/GETTING_STARTED.md`** — restructure to lead with the
  AI-operator path; demote the human-driven path.
- **`docs/HOW_TO_HUMAN.md`** — narrow to an escalation playbook.
  Points at `docs/HOW_TO_AGENT.md` for the rare full-driver case.
- **`docs/HOW_TO_AGENT.md`** — cross-link updates; no rewrite.
- **`docs/DECISION_LOG.md`** — D103 records the principle.

## Deferred follow-ups (scoped here, landed separately)

1. **Workflow.json schema-field rename.** `human_checkpoint` →
   `escalation_checkpoint` (TBD); bumps the schema version. The
   `waiting_human` run state renames in lockstep. Breaking; needs a
   `workflow upgrade` rule.
2. **CLI prompt-string sweep.** Any verb whose stderr / prompt text
   says "human confirmation required" or similar should say "operator
   confirmation."
3. **`escalation` artifact-kind schema.** Front matter, validator
   rules, dotted RPC method (e.g. `escalation.publish`,
   `escalation.resolve`), interaction with RFC 0051 auto-finalize.
4. **HOW_TO_AGENT.md alignment.** Minor pass for cross-references.
5. **Notification rails.** Paging / web UI badge / inbox-by-email if
   escalation-discovery latency proves to matter.

## Open questions

1. Should `striatum blocker resolve` (or `escalation resolve`) be a
   first-class verb, or does the principal always go through
   `decision record`?
2. Is the principal's session registered (so audit trails attribute
   the decision to a specific session), or are principal moves
   attributed only by `author: human-principal` byline? RFC 0026
   attestation does not apply to humans.
3. Should there be an escalation timeout — the AI gives up after N
   minutes of `waiting_principal` and switches to a configured
   fallback (auto-fail the run, notify-and-continue, operator decides)?
4. Does the principal need a separate capability class, or does
   `admin` (per RFC 0030) cover it?

## Phasing

- **Phase 0 (this RFC):** principle on record; doc-side fixes
  (SPEC / GETTING_STARTED / HOW_TO_HUMAN) land with the RFC; D103
  recorded.
- **Phase A (V1.7 or V1.8):** `escalation` artifact-kind schema,
  validator, dotted RPC method; CLI prompt-string sweep.
- **Phase B (workflow-schema bump):** `human_checkpoint` →
  `escalation_checkpoint` rename, `waiting_human` →
  `waiting_principal`, `workflow upgrade` rule.
- **Phase C:** notification rails if needed.

## Provenance

- 2026-05-14 operator session: principle stated by the project owner
  ("the human principal's only role is to resolve unresolvable
  blockers or decisions").
- AskUserQuestion answers in the same session: term = `human
  principal`; CLI surface = same as AI operator (functionally bounded,
  not interface bounded); trigger = both declared classes +
  AI-self-declared; scope = propose + land doc-side fixes.
- Precedents: D094 (Postgres + daemon-required); RFC 0046 (lane
  evidence guard + V1.41 burndown verbs); RFC 0040 (MCP dogfood
  harness); RFC 0052 (committee deliberation, composes with this
  RFC's stalemate escalation); [fb0175c](https://github.com/halbritt/striatum/commit/fb0175c)
  (UBIQUITOUS_LANGUAGE first-pass language sweep).
