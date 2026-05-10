# striatum RFCs

This directory holds `striatum` RFCs. Engram RFCs remain under
`docs/rfcs/`; they can be reference fixtures, but they are not the product
decision record for the runner.

RFCs here are for contested or cross-cutting `striatum` design changes:
workflow semantics, review gates, artifact contracts, adapter behavior, and
run-state policy. Accepted RFCs should update `docs/DECISION_LOG.md`
and, when behavior changes, `docs/SPEC.md`.

## Index

| RFC | Status | Topic |
| --- | --- | --- |
| [0001](0001-run-recovery-and-dogfood-fixes.md) | accepted | Turn the RFC 0014 dogfood fixes into a runner RFC. |
| [0002](0002-reviewer-independence-policy.md) | accepted | Make reviewer access scope and context policy explicit workflow fields. |
| [0003](0003-support-ledgers-and-evidence-audits.md) | accepted | Add support ledgers and evidence-audit jobs for claims made by artifacts. |
| [0004](0004-critique-to-action-loop.md) | accepted | Normalize review action items and require resolution checks. |
| [0005](0005-harness-meta-optimization.md) | accepted | Use runner events to propose harness improvements, gated by review. |
| [0006](0006-sqlite-schema-migration-system.md) | accepted | Forward-only SQLite migrations keyed off `PRAGMA user_version`. |
| [0007](0007-workflow-visualization.md) | accepted | Export workflow graphs for authoring and review. |
| [0008](0008-worktree-isolation-for-parallel-jobs.md) | accepted | Opt-in per-job Git worktree isolation for parallel repo-write jobs. |
| [0009](0009-long-lived-process-supervision.md) | accepted | Architecture for V2 supervised agent processes that span multiple work packets. |
| [0010](0010-tool-harness-profiles.md) | accepted | Add tool-specific harness profiles for native delegation and provider features. V1+V1.5+V2 implemented (D056). |
| [0011](0011-session-close-and-run-terminal-auto-close.md) | accepted | Explicit session-close CLI plus auto-close of active sessions on run-terminal transitions. |
| [0012](0012-local-service-api.md) | accepted | Local HTTP / Unix-socket API on top of `striatum.api.invoke`, with SSE for events. V1 implemented (D058) under dogfood-006. |
| [0013](0013-local-web-ui.md) | accepted (V1+step 7) | Static SPA bundled with the runner, served by `striatum serve --web`. V1 (D059, dogfood-007) shipped read-only views; step 7 (D065, dogfood-013) shipped mutation buttons (verdict, decision, checkpoint resolve continue/cancel, requeue stale review-only) over the existing RFC 0012 mutation gate. |
| [0014](0014-process-adapter-completion-guarantees.md) | accepted | Post-exit output validation, configurable timeouts, and liveness reconciliation for one-shot `adapter run`. V1 implemented (D057). Closes [#1](https://github.com/halbritt/striatum/issues/1). |
| [0015](0015-self-contained-agent-skills.md) | accepted (V1+step 3) | `striatum skills install` generates a self-contained agent skill bundle for any installed agent CLI. V1 (D061, dogfood-009) shipped Claude Code + generic profiles; step 3 (D063, dogfood-011) shipped Codex + Gemini profiles + `--profile all`. |
| [0016](0016-dashboard-dependency-graph.md) | accepted (V1+step 3) | Render the run's dependency graph inside `striatum dashboard` and `run graph --format ascii`. V1 (D060, dogfood-008) shipped layered/list ASCII + state colors; step 3 (D064, dogfood-012) shipped Unicode `fancy` style + `--graph-orient {tb,lr}` left-to-right layout. |
| [0017](0017-readme-and-docs-reorganization.md) | accepted (V1) | Slim the README to ~250 lines, split human and coding-agent quick starts, and move behavior-model / sequential-usage / dogfood-history / per-RFC subsections / command reference into dedicated `docs/` files. V1 implemented (D062) under dogfood-010. |
| [0018](0018-focused-adversarial-review-postures.md) | accepted (V1+step 3) | Declare review-job posture (security, threat_model, devils_advocate, etc.) and per-build `required_review_postures` so workflows can require focused adversarial coverage. V1 (D069, dogfood-016) shipped steps 1+2: validator + packet exposure + workflow-validation reachability gate (re-cast from runtime gate per V1_ACCEPTANCE). Step 3 (D071, dogfood-018) shipped `verdicts.posture` column + introspection surfacing across status, run-summary, evidence-export, run-graph json, dashboard, and web UI. |
| [0019](0019-domain-driven-design-foundations.md) | accepted | `docs/DDD.md` (D067) documents striatum's DDD framing — bounded context, ubiquitous language, aggregate roots, value objects, domain events, CLI-as-only-write-surface — so readers see *why* the vocabulary is load-bearing rather than reverse-engineering it. |
| [0020](0020-autonomous-stalled-run-recovery.md) | accepted (V1) | `recovery auto` one-shot sweeper + `recovery_policy` workflow block + escalation hooks (marker_file, webhook, shell) + `recovery watch` daemon. V1 closes after dogfood-014 (steps 1+2, D066) and dogfood-015 (step 3, D068). |
| [0024](0024-workflow-browser-and-builder.md) | accepted (V1+V1.5+V2+V3+V4) | Workflow browser + visual builder. V1: `/workflows/` lists every `**/workflow.json` in the repo with validation status + SVG graph thumbnail; `/workflows/<path>` detail page with full graph + tabular jobs/lanes/roles/edges. New chat tool `list_workflows` for the closed set. V1.5: form-driven visual editor with widgets for jobs, edges, lanes, roles, posture fields; save runs `workflow validate` server-side. No new runtime deps; reuses RFC 0022 V1's SVG renderer; no client-side graph library. Drag-and-drop deferred to V2. |
| [0023](0023-web-chat-and-browse.md) | accepted (V1+V1.5) | Web UI chat + codebase browse: provider-neutral chat client (Anthropic Messages + OpenAI Chat-compatible flavors, configured via env vars), read-only `/view/<path>` file viewer scoped to `--repo`, inline Markdown rendering for `.md` artifacts. V1 (D074, dogfood-021): chat lifecycle + provider client + view endpoint + 3-lane review pattern. V1.5 (D075, dogfood-022): six closed-set read-only chat tools (read_file, list_dir, striatum_status, striatum_why, git_log, git_diff) + system-prompt briefing on chat-session creation + bundled fixes (graph-node click 404, doctor problem list, chat double-render). |
| [0022](0022-web-ui-redesign.md) | accepted (V1) | Web UI redesign: server-rendered Jinja2 multi-page (`/`, `/run/<id>`, `/run/<id>/job/<id>`, `/run/<id>/artifact/<id>`, `/doctor`), refreshed visual palette + dark mode (via `prefers-color-scheme`) + system fonts + 4px spacing scale, layered SVG dependency graph with state-colored nodes and click-navigate. V1 (D073, dogfood-020) added Jinja2 as the project's first runtime dep. RFC 0013 V1's hash-routed SPA is superseded; CSP unchanged; JSON API + SSE feed unchanged. V1.5 deferred: inline dogfood Markdown rendering on artifact pages, SVG zoom/pan. |
| [0025](0025-agent-cli-plugin-bundles.md) | accepted (V1) | `striatum plugin install` emits an agent-CLI plugin bundle (Claude Code `.claude-plugin/`, Codex `.codex-plugin/`, Gemini `gemini-extension.json`) wrapping RFC 0015's skill bodies plus five imperative slash commands, an opt-in hooks stub, and a local marketplace fixture. Promotes `gemini` to first-class. Self-contained per D020; offline generation; no hosted services. V1 ships the three first-class profiles in three landable steps. |
| [0026](0026-lane-attestation-and-operator-byline-honesty.md) | proposed | Make lane attestation a derived property of a supervised-session binding and downgrade unattested bylines to `author: operator`. Closes the prevention half of [#2](https://github.com/halbritt/striatum/issues/2); recovery primitive ([#3](https://github.com/halbritt/striatum/issues/3)) is a companion RFC. |
| [0021](0021-ddd-layout-scaffold-on-init.md) | accepted (V1+V1.5) | `striatum init --with-ddd-layout` scaffolds the seven canonical human-facing DDD documents (`docs/{SPEC,PRD,DECISION_LOG,UBIQUITOUS_LANGUAGE,DDD}.md`, `docs/rfcs/`) into the target repo. Mirrors RFC 0015's `--with-skills` for agent-facing files. V1 (D070, dogfood-017) shipped opt-in literal-copy scaffold. V1.5 (D072, dogfood-019) shipped `--ddd-layout-force` (overwrite with `prior_sha256` audit) and `--ddd-layout-dry-run` (preview via `would_*` status vocabulary). |

## Template

Use this shape for new RFCs:

```text
# RFC NNNN: Title

Status: proposed | accepted | deferred | rejected | superseded
Date: YYYY-MM-DD
Context: links

## Problem
## Goals
## Non-Goals
## Proposal
## Acceptance Criteria
## Open Questions
## Domain Modeling
```

The optional `## Domain Modeling` section identifies which DDD
pattern the new concept fits (aggregate root, value object,
domain event, or boundary clarification) and cites
[`docs/DDD.md § "Adding to the model"`](../DDD.md#adding-to-the-model).
RFC 0019 is the precedent.
