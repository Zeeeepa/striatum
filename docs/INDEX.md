# Documentation Index

Every Markdown document under `docs/`, with a one-line summary.
Per-run dogfood artifacts under `docs/dogfood/<id>/` are listed
collectively, not individually.

## Onboarding and how-tos

| File | Audience | Summary |
|---|---|---|
| [GETTING_STARTED.md](GETTING_STARTED.md) | New user | From a fresh target repo to a running workflow in ~15 minutes; forks human-operator vs. coding-agent setups. |
| [HOW_TO_HUMAN.md](HOW_TO_HUMAN.md) | Human operator | Long-form playbook for driving striatum by hand: every CLI verb in the order you use it. |
| [HOW_TO_AGENT.md](HOW_TO_AGENT.md) | Coding agent | Long-form companion to the RFC 0015 skill bundle; the workflow loop, work-packet shape, and what not to do. |
| [CONTEXT_HYGIENE.md](CONTEXT_HYGIENE.md) | Operator / agent author | Why session quality is not a function of token budget; repo-side, session-side, and model-side practices for replicating high-taste sessions. |
| [WRITING_WORKFLOWS.md](WRITING_WORKFLOWS.md) | Workflow author | How to author `workflow.json` from scratch: required fields, scaffold layout, common graph shapes, validation. |
| [CLI_REFERENCE.md](CLI_REFERENCE.md) | Anyone | Flat list of every CLI verb plus stable exit codes; `--help` is authoritative. |

## Specifications and decisions

| File | Audience | Summary |
|---|---|---|
| [SPEC.md](SPEC.md) | Anyone | The implementation contract for the V1 surface. The source of truth when this index and the runner disagree. |
| [DDD.md](DDD.md) | Anyone curious about the framing | Why the vocabulary is the model, not bookkeeping; bounded context, aggregate roots, value objects, the events log, and the CLI-as-only-write-surface invariant. |
| [DOC_MAP.md](DOC_MAP.md) | Anyone editing the docs | The boundary contract: which doc owns what, what each doc deliberately does *not* contain, and the direction citations should flow. |
| [PRD.md](PRD.md) | Product reader | The product requirements that drove the V1 design. |
| [DECISION_LOG.md](DECISION_LOG.md) | Product / architecture reader | Every product and architecture decision (`D###` rows) with reason, consequences, and revisit triggers. |
| [UBIQUITOUS_LANGUAGE.md](UBIQUITOUS_LANGUAGE.md) | Anyone | Glossary of striatum-specific terms (run, session, lease, work packet, lane, etc.). |
| [TODO.md](TODO.md) | Maintainer | Active product-improvement tracker. |

## Background and reference

| File | Audience | Summary |
|---|---|---|
| [PRIOR_ART.md](PRIOR_ART.md) | Reader curious about lineage | What striatum borrows from and where it diverges. |
| [INTERVIEW_LOG.md](INTERVIEW_LOG.md) | Historical | The interview rounds that shaped the PRD. |
| [MCP.md](MCP.md) | MCP integrator | The local stdio JSON-RPC wrapper's framing and tool surface. |
| [ENGRAM_INCUBATION_CONTEXT.md](ENGRAM_INCUBATION_CONTEXT.md) | Historical | Engram-incubation provenance; not current product material. |
| [README.md](README.md) | Doc tree reader | Pointer file for `docs/`. |

## RFCs

| File | Summary |
|---|---|
| [rfcs/](rfcs/) | All accepted/proposed RFCs (RFC 0001–RFC 0017). Each RFC has its own `.md` file plus an entry in `rfcs/README.md`. |

## Dogfood material

| Path | Summary |
|---|---|
| [dogfood/](dogfood/) | Per-run scaffolds (`<id>/workflow.json` plus `prompts/`, `roles/`, `research/`, `review/`, `decisions/`, `BUILD_HANDOFF.md`, `RUN_SUMMARY.md`). |
| [dogfood/HISTORICAL.md](dogfood/HISTORICAL.md) | Pointers to the dogfood-001/003/004/005 incubation runs that previously lived in the README. |
| [dogfood/FRICTION_LOG.md](dogfood/FRICTION_LOG.md) | Aggregate friction register across runs. |

## Repository-level files

| File | Summary |
|---|---|
| [../README.md](../README.md) | Top-level pitch + install + two quick starts (human and agent) + this index. |
| [../AGENTS.md](../AGENTS.md) | Project instructions for agents and contributors working on the striatum source. |
| [../CHANGELOG.md](../CHANGELOG.md) | Per-version release notes since `0.1.0`. |
