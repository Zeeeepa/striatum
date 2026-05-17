# Documentation Index

Every Markdown document under `docs/`, with a one-line summary.
Per-run dogfood artifacts under `docs/dogfood/<id>/` are listed
collectively, not individually.

## Onboarding and how-tos

| File | Audience | Summary |
|---|---|---|
| [USING_STRIATUM.md](USING_STRIATUM.md) | New arrival (RFC 0054) | Day-zero usage guide: the operator + principal model, prerequisites, day-zero setup, first run, escalation surface. Read this first. |
| [GETTING_STARTED.md](GETTING_STARTED.md) | New user | From a fresh target repo to a running workflow in ~15 minutes; leads with AI-operator setup and keeps operator-by-hand notes as a sidebar. Superseded by USING_STRIATUM.md for the role-aware walkthrough; retained for the deeper install / verify steps. |
| [HOW_TO_HUMAN.md](HOW_TO_HUMAN.md) | Human principal | Escalation playbook for resolving blockers and decisions; retains the operator-by-hand walkthrough as reference. |
| [CONSUMER_REPO_LAYOUT.md](CONSUMER_REPO_LAYOUT.md) | Target-repo owner (RFC 0056) | Opinionated-but-non-mandatory directory layout recommendations: where the workflow file lives, where artifacts land, what to gitignore. |
| [HOW_TO_AGENT.md](HOW_TO_AGENT.md) | Coding agent | Long-form companion to the RFC 0015 skill bundle; the workflow loop, work-packet shape, and what not to do. |
| [CONTEXT_HYGIENE.md](CONTEXT_HYGIENE.md) | Operator / agent author | Why session quality is not a function of token budget; repo-side, session-side, and model-side practices for replicating high-taste sessions. |
| [WORKFLOW_TYPES.md](WORKFLOW_TYPES.md) | Workflow selector | Which workflow shape and lane set to choose; current starters, examples, defaults, and the roadmap toward a chooser UI. |
| [WRITING_WORKFLOWS.md](WRITING_WORKFLOWS.md) | Workflow author | How to author `workflow.json` from scratch: required fields, scaffold layout, common graph shapes, validation. |
| [CLI_REFERENCE.md](CLI_REFERENCE.md) | Anyone | Flat list of every CLI verb plus stable exit codes; `--help` is authoritative. |
| [POSTGRES_TRANSITION.md](POSTGRES_TRANSITION.md) | Operator | The D094 / RFC 0043 PostgreSQL cutover runbook: prerequisites, daemon doctor role/grant repair, daemon service startup, `striatum adopt`, `striatum daemon migrate-repo-local`, tombstone vs delete, verification, and exit codes 11 / 12. |

## Specifications and decisions

| File | Audience | Summary |
|---|---|---|
| [SPEC.md](SPEC.md) | Anyone | The implementation contract for the V1 surface. The source of truth when this index and the runner disagree. |
| [DDD.md](DDD.md) | Anyone curious about the framing | Why the vocabulary is the model, not bookkeeping; bounded context, aggregate roots, value objects, the events log, and the daemon-method write-boundary invariant. |
| [DOC_MAP.md](DOC_MAP.md) | Anyone editing the docs | The boundary contract: which doc owns what, what each doc deliberately does *not* contain, and the direction citations should flow. |
| [PRD.md](PRD.md) | Product reader | The product requirements that drove the V1 design. |
| [DECISION_LOG.md](DECISION_LOG.md) | Product / architecture reader | Every product and architecture decision (`D###` rows) with reason, consequences, and revisit triggers. |
| [UBIQUITOUS_LANGUAGE.md](UBIQUITOUS_LANGUAGE.md) | Anyone | Glossary of striatum-specific terms (run, session, lease, work packet, lane, etc.). |
| [TODO.md](TODO.md) | Maintainer | Active product-improvement tracker. |
| [ROADMAP.md](ROADMAP.md) | Operator kicking off / resuming work | Forward-looking sequencing of TODO items, RFC follow-ups, open GH issues, and active runway — read first when picking up cold. Stays in sync with version bumps. |
| [architecture/COMMAND_AUTHORITY_MATRIX.md](architecture/COMMAND_AUTHORITY_MATRIX.md) | Maintainer | Phase 0 inventory of CLI/RPC authority paths across Python PG handlers, daemon RPC route translations, closed fallback guardrails, Go handlers, and legacy SQLite dependencies. |
| [architecture/DAEMON_METHOD_TABLES.md](architecture/DAEMON_METHOD_TABLES.md) | Maintainer | Generated daemon method registry and CLI route translation reference, sourced from `contracts/daemon_methods.json` and guarded by `scripts/generate_daemon_method_tables.py --check`. |

## Background and reference

| File | Audience | Summary |
|---|---|---|
| [MCP.md](MCP.md) | MCP integrator | The local stdio JSON-RPC wrapper's framing and tool surface, plus the RFC 0040 V1 dogfood-lifecycle chat tools served by `striatum serve --web`. |
| [HARNESS_FRICTION_PATTERNS.md](HARNESS_FRICTION_PATTERNS.md) | Maintainer / RFC author | Long-form record of recurring dogfood friction shapes (036-039) and the V1 fixes that landed; companion to RFC 0040. |
| [README.md](README.md) | Doc tree reader | Pointer file for `docs/`. |

## Historical (incubation provenance — not current product material)

> Each file below carries a banner at the top calling out its
> historical status. Read these only when you need to understand
> how a load-bearing decision was originally framed; for current
> behavior, the sources of truth are `docs/SPEC.md`,
> `docs/DECISION_LOG.md`, and `docs/rfcs/`.

| File | Summary |
|---|---|
| [PRIOR_ART.md](PRIOR_ART.md) | Pre-PRD survey of orchestration tools that shaped early framing. Not a list of currently-tracked dependencies. |
| [INTERVIEW_LOG.md](INTERVIEW_LOG.md) | The interview rounds that produced the original PRD and the early `D###` decision rows. |
| [ENGRAM_INCUBATION_CONTEXT.md](ENGRAM_INCUBATION_CONTEXT.md) | Engram-extraction provenance; striatum was extracted from a parent project. |
| [RFC_0014_DOGFOOD_FIX_SPEC.md](RFC_0014_DOGFOOD_FIX_SPEC.md) | Pre-RFC-0001 dogfood findings; everything actionable here landed in subsequent RFCs. |

## RFCs

| File | Summary |
|---|---|
| [rfcs/](rfcs/) | All accepted/proposed Striatum RFCs. Each RFC has its own `.md` file plus a current entry in `rfcs/README.md`. |

## Dogfood material

| Path | Summary |
|---|---|
| [dogfood/](dogfood/) | Per-run scaffolds (`<id>/workflow.json` plus `prompts/`, `roles/`, `research/`, `review/`, `decisions/`, `BUILD_HANDOFF.md`, `RUN_SUMMARY.md`). |
| [dogfood/HISTORICAL.md](dogfood/HISTORICAL.md) | Distinguishes the historical incubation runs (001–013) from the current cadence (014+) and lists what each recent run shipped. |
| [dogfood/FRICTION_LOG.md](dogfood/FRICTION_LOG.md) | Aggregate friction register across runs. |

## Repository-level files

| File | Summary |
|---|---|
| [../README.md](../README.md) | Top-level pitch, daemon/Postgres quick start, project status, and documentation table. |
| [../AGENTS.md](../AGENTS.md) | Project instructions for agents and contributors working on the striatum source. |
| [../CHANGELOG.md](../CHANGELOG.md) | Per-version release notes since `0.1.0`. |
