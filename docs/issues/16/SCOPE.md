---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/issues/16/SPEC.md", "prompts/OPERATOR_BOUNDARY_PROMPT.md", "prompts/README.md", "docs/HOW_TO_AGENT.md", "AGENTS.md", "docs/SPEC.md"]
---

author: triager-unknown-model-001

# GH #16 — SCOPE

Bound scope for the GH #16 fix job. The implementer must follow each
acceptance bullet literally; the verify job will cite by ID.

## 1. Files in scope

The implementer will create one new file and edit two existing files:

- **CREATE** `prompts/OPERATOR_INITIALIZATION_PROMPT.md` — the complete
  reusable operator initialization prompt described by the SPEC.
- **EDIT** `prompts/README.md` — add a new "Reusable Prompts" entry for
  `OPERATOR_INITIALIZATION_PROMPT.md` and rewrite the existing
  `OPERATOR_BOUNDARY_PROMPT.md` entry so the two are clearly
  differentiated ("use the initialization prompt to boot a fresh
  operator session; paste the boundary prompt only when you want the
  guardrail alone, e.g., inside an in-progress session that is drifting
  into role work").
- **EDIT** `prompts/OPERATOR_BOUNDARY_PROMPT.md` — **trim to focused
  guardrail (option b)**. Strip the RFC0026/RFC0027 examples and the
  bare "SQLite" mention; keep the file path and the boundary rules as a
  short, generic guardrail. The new initialization prompt will
  reference (not duplicate) this file for its boundary section.
  Justification: the SPEC calls the existing wording "stale/narrow";
  deleting examples + generalizing "SQLite" to "state substrate"
  preserves the value of the guardrail without duplicating boundary
  content inside the new init prompt.

## 2. Out of scope

The implementer must NOT touch:

- `docs/dogfood/` — historical run artifacts.
- `docs/rfcs/` — design records.
- `docs/DECISION_LOG.md`, `docs/TODO.md`, `docs/ROADMAP.md` — referenced
  only; not edited by this issue.
- `src/`, `tests/` — no code change is part of this issue.
- `.striatum/` — runner state substrate; never written by hand.
- Any other prompt under `prompts/` besides the two listed in §1.

## 3. Acceptance checklist

The verify job cites each ID below. Every item is extracted literally
from `docs/issues/16/SPEC.md`.

Definition of done:

- [DoD-1] `prompts/OPERATOR_INITIALIZATION_PROMPT.md` exists and is marked `Status: reusable`.
- [DoD-2] It is a complete initialization prompt, not merely a short boundary warning.
- [DoD-3] `prompts/README.md` lists it separately from `OPERATOR_BOUNDARY_PROMPT.md` and explains when to use each.
- [DoD-4] The old boundary prompt is either kept as a focused guardrail or refactored so the new initialization prompt reuses/points to it. (This scope picks "trim to focused guardrail + new prompt points at it".)
- [DoD-5] The prompt is generic and does not hardcode RFC0026/RFC0027 or Engram-specific paths.
- [DoD-6] The prompt reflects the current daemon/Postgres transition accurately, including any RFC 0048 caveat where needed.
- [DoD-7] A fresh operator session can use it to start or resume a run without reading historical dogfood prompts first.

Required shape — fill-in block must include:

- [RS-1] Striatum repo path.
- [RS-2] Striatum version / command path.
- [RS-3] Target repository path.
- [RS-4] Workflow path.
- [RS-5] Intended branch / branch-confirmation policy.
- [RS-6] Existing run id if resuming, otherwise whether to prepare/start a new run.
- [RS-7] Daemon/Postgres state and whether direct mode is allowed for this run.
- [RS-8] Required docs to read first.
- [RS-9] Expected artifact root.
- [RS-10] Operator report path and update cadence.
- [RS-11] Whether the operator may use MCP/chat tools, CLI only, or both.
- [RS-12] Whether native sub-agents may be used for operator-side read-only audits.
- [RS-13] Any current blockers, known open GitHub issues, or deferred work to preserve.
- [RS-14] Commit/push policy.

Required behavior — the prompt must explicitly instruct the operator to:

- [RB-1] Read the project instructions and canonical docs before acting.
- [RB-2] Check `git status --short --branch` before state-changing work.
- [RB-3] Confirm the Striatum command path/version.
- [RB-4] Inspect current run/workflow state when resuming.
- [RB-5] Validate the workflow before preparing or starting a run.
- [RB-6] Prepare/start/register/claim/ack/publish/complete only through Striatum commands or approved MCP/chat tools.
- [RB-7] Keep role work in role sessions; the operator may coordinate but must not author role artifacts.
- [RB-8] Use `status`, `why`, `doctor`, `dashboard`, `run summary`, and documented recovery/checkpoint commands for failures.
- [RB-9] Update the operator report incrementally, especially before compaction or handoff.
- [RB-10] Preserve unrelated user changes and never edit `.striatum/` or the state substrate directly.
- [RB-11] Stop for explicit human decision only when the workflow reaches a human checkpoint or the prompt says a decision is required.

Required first-action sequence — in this order:

- [RFAS-1] Load the project instructions and listed canonical docs.
- [RFAS-2] Check repository state and Striatum version.
- [RFAS-3] Inspect daemon/Postgres or direct-mode status as specified by the filled-in block.
- [RFAS-4] Validate the workflow.
- [RFAS-5] Inspect or create the run.
- [RFAS-6] Start or resume execution.
- [RFAS-7] Record/update `OPERATOR_REPORT.md`.
- [RFAS-8] Continue driving the workflow until blocked or complete.

## 4. Generic-language guardrails

Forbidden tokens in `OPERATOR_INITIALIZATION_PROMPT.md` and the trimmed
`OPERATOR_BOUNDARY_PROMPT.md`:

- `RFC 0026`, `RFC0026`, `RFC 0027`, `RFC0027` and any other specific
  RFC ordinal used as an illustrative example. RFC references are
  permitted only when they document a current product caveat (e.g.,
  RFC 0043 V1, RFC 0048 phase C, RFC 0051) and are cited as such.
- `Engram`, `engram` — project-specific name; do not appear in product
  prompts.
- Specific dogfood ordinals (`054`, `055`, `056`, `057`, ...) used as
  illustrative examples.
- Specific lane-stall codenames (`claude-no-publish`,
  `gemini-no-frontmatter`, `codex/codex co-blindness`) used as
  illustrative examples — defer to the operator decision rules in
  `docs/ROADMAP.md` §3.
- `SQLite` as the named substrate. Use "state substrate" or "runner
  state" instead, because RFC 0043 / RFC 0048 are migrating the
  substrate to Postgres.
- Hardcoded absolute home-directory paths (`/home/...`,
  `/Users/...`). Use environment variables or `~/`-style placeholders.

The generic-language rule lives in `AGENTS.md` under "Change
Discipline": "Prefer generic terms: target repository, workflow
fixture, runner state, artifact, adapter, lane, session, work packet"
and "Do not add new Engram-specific paths, branch names, prompt
ordinals, or marker names to product docs or core code."

## 5. Daemon/Postgres caveat

RFC 0043 V1 has landed (D094, dogfood-048): the Postgres-backed daemon
is shippable. The default has **not yet** been flipped to
`daemon-required` (item 31(b) in `docs/ROADMAP.md` §6); the
`STRIATUM_DAEMON_REQUIRED=0 + STRIATUM_TEST_HARNESS=1` escape still
works as a test-harness mode, and RFC 0048 phase C will remove that
escape entirely. The fill-in block guidance for the
"Daemon/Postgres state" field (RS-7) must therefore ask the human to
declare one of three current modes — (a) daemon + Postgres, (b) direct
mode against the SQLite substrate, (c) test-harness escape with
`STRIATUM_DAEMON_REQUIRED=0 + STRIATUM_TEST_HARNESS=1` — and warn that
mode (c) is scheduled for removal in RFC 0048 phase C. The prompt
should not assume any one mode is current.

## 6. Cross-references the implementer should embed

The new prompt **points at** these instead of duplicating them. Embed
each as a fully relative repo path so the operator can read them
without translation:

- `AGENTS.md` (top-level) — project boundary, generic-term rule,
  change discipline.
- `docs/HOW_TO_AGENT.md` — operator/agent contract, workflow loop, work
  packet shape, byline rules, "what not to do" list.
- `docs/SPEC.md` — artifact front-matter schemas, lease semantics,
  exit codes (esp. 5 / 6), supervisor model.
- `docs/SPEC.md#artifact-front-matter-schemas` (anchor) — for the
  byline + front-matter validation rules.
- `docs/ROADMAP.md` §3 (operator decision rules) — operator-on-behalf
  publish, verdict override, fix-up dogfood pattern, wrapper auth,
  anti-patterns, cycle-exhaustion override. The init prompt must NOT
  re-encode §3; it must reference it by name.
- `docs/CLI_REFERENCE.md` — flat list of CLI verbs with stable exit
  codes.
- `docs/DECISION_LOG.md` — architectural decisions; the latest D-row
  is operator-visible context.
- `docs/TODO.md` — open work, blocked items.
- `prompts/OPERATOR_BOUNDARY_PROMPT.md` — the boundary section the new
  prompt cites rather than duplicates.

## 7. What "complete" means

"Complete" means a capable AI session, handed only
`OPERATOR_INITIALIZATION_PROMPT.md` with the fill-in block filled, can
load the listed canonical docs, inspect repository and Striatum
state, decide whether it is preparing/starting or resuming a run,
and reach its first state-changing CLI call (`workflow validate`,
`run prepare`, `run start`, or `run summary` for resume) without
opening any prompt under `prompts/` other than `OPERATOR_BOUNDARY_PROMPT.md`,
and without reading any dogfood under `docs/dogfood/`. The verify job's
primary ergonomics check is: every step in the [RFAS-1..8] sequence
has an unambiguous, prompt-internal answer to "what do I do" given
the fill-in block, with only repo-relative pointers to canonical docs
for elaboration.
