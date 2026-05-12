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
| [0026](0026-lane-attestation-and-operator-byline-honesty.md) | accepted (V1) | Make lane-liveness attestation a derived property of an attached supervised-session binding and downgrade unattested bylines to `author: operator`. V1 ships migration v12, operator labels, publish/verdict gates for review jobs, and status/evidence surfacing. |
| [0027](0027-sealed-patch-provenance-mode.md) | proposed (phase 2 guardrails + V2 apply foundation shipped) | Introduce opt-in sealed patch provenance: protected source writes through Striatum, lane scratch workspaces, immutable patch artifacts, hash-bound reviews, apply gate, signed receipts, and a narrow local signed-commit exception for sealed apply. Current code has provenance-mode guardrails plus RFC 0031 daemon apply receipt schema and fail-closed apply authority helpers; full patch mutation remains capability-gated daemon scope. |
| [0028](0028-long-running-daemon-and-multi-repository-control-plane.md) | accepted (V1) | Optional registry-backed multi-repository read visibility plus foreground sweep process (`striatumd`): registry, repo add/list/remove, explicit daemon read mode, global dashboard, resources-only daemon MCP, metadata-only audit, and recovery sweep. V1 did not ship a daemon RPC server; RFC 0030/0031 now add the V2 RPC/supervision/apply foundation while MCP mutation expansion, cross-repo workflows, service install, Windows support, audit retention/rotation, and hosted semantics remain deferred. |
| [0029](0029-operator-recovery-for-process-adapter-blockers.md) | accepted (V1 core) | Add `striatum recovery resume --blocker-id <id> [--complete]` so operators can close out the RFC 0014 process-adapter blocker family (`process_outputs_missing`, `process_review_verdict_missing`, `process_exit_nonzero`, `process_timeout_exceeded`, `process_lost_with_outputs_missing`) on repo-write jobs once remediation is on disk. Closes the loop the diagnostic envelope's `recovery_commands` field already advertises. |
| [0033](0033-storage-substrate-rewrite-for-daemon-v2.md) | accepted (V2) | Pick a non-SQLite substrate for daemon V2 daemon-owned state (resolves RFC 0028 OQ#3 per D086). Accepts system PostgreSQL supplied by the operator; the daemon owns schema, migrations, roles, and audit semantics but does not manage the Postgres lifecycle. V1 SQLite-registry → V2 daemon DB cutover uses `striatum daemon migrate --from sqlite --to pg`. Lands first in the daemon V2 follow-up sequence because RFC 0030 keys off the substrate choice. Repo-local `.striatum/state.sqlite3` stays SQLite. |
| [0030](0030-daemon-rpc-server-and-version-skew-protocol.md) | accepted (V2) | Daemon RPC server foundation, language-agnostic envelope, `daemon.hello`/`daemon.welcome` version handshake (resolves RFC 0028 OQ#7), capability-bound method registry, audit + request log helpers on the RFC 0033 substrate, and migration path from V1 direct-registry reads to daemon-mediated routing. Spine of daemon V2; depends on RFC 0033. |
| [0031](0031-daemon-owned-supervision-and-sealed-apply-boundary.md) | accepted (V2 foundation) | Move supervisor ownership metadata into the daemon DB, add repo-local supervisor pointers, declare daemon-mediated `supervise.*` RPC routes, add apply receipt schema, and keep sealed apply fail-closed unless the daemon has explicit apply authority. Resolves RFC 0028 OQ#2. Depends on RFC 0030. |
| [0032](0032-cross-repo-workflows-and-mcp-mutation-capabilities.md) | proposed | Cross-repository workflow schema (`repositories` block), daemon-mediated coordination with best-effort consistency on crash, MCP mutation capability vocabulary (`read`/`write`/`review`/`claim`/`apply`/`admin`/`recovery`) with default-deny gating. Resolves RFC 0028 OQ#4 and OQ#5. Depends on RFC 0030; uses sealed receipts from RFC 0031 where each repo is independently sealed-eligible. |
| [0021](0021-ddd-layout-scaffold-on-init.md) | accepted (V1+V1.5) | `striatum init --with-ddd-layout` scaffolds the seven canonical human-facing DDD documents (`docs/{SPEC,PRD,DECISION_LOG,UBIQUITOUS_LANGUAGE,DDD}.md`, `docs/rfcs/`) into the target repo. Mirrors RFC 0015's `--with-skills` for agent-facing files. V1 (D070, dogfood-017) shipped opt-in literal-copy scaffold. V1.5 (D072, dogfood-019) shipped `--ddd-layout-force` (overwrite with `prior_sha256` audit) and `--ddd-layout-dry-run` (preview via `would_*` status vocabulary). |
| [0034](0034-workflow-generator-and-template-catalog.md) | accepted (V1) | Add a first-class workflow generator and local template catalog so operators choose workflow shape, lane set, artifact root, and policy options instead of freestyling `workflow.json`. V1 ships generator core, package-data catalog, CLI/service surfaces, custom-plan compiler, and `workflow init --style` rewire; web chooser and chat scaffolding are deferred. |
| [0035](0035-multi-repo-test-harness-for-cross-repo-workflows.md) | proposed | `tests/_harness/MultiRepoHarness` fixture that boots a daemon + N registered target repositories with ephemeral Postgres so RFC 0032 cross-repo workflow + MCP capability scope behavior can be exercised end-to-end. Covers prepare/lifecycle/crash-recovery/MCP-capability-scope/per-repo-write-scope. Lands the harness-level coverage dogfood-035 deferred (TODO Open item 19). |
| [0036](0036-mcp-harness-for-daemon-v2-mutation-surface.md) | proposed | Agent-facing harness for the daemon V2 mutation surface: new `striatum-mcp` skill teaching the preview-then-write idiom, capability/token lifecycle, denial-vocabulary recovery, and capability scope semantics; plus the RFC 0034 §10 chat-assisted scaffolding tool (`generate_workflow_preview` + `generate_workflow_write`) as closed-set chat tools over the RFC 0023 chat surface with operator confirmation enforced before any write. Closes the harness gap left by RFC 0032 V2 + RFC 0034 V1. |

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
