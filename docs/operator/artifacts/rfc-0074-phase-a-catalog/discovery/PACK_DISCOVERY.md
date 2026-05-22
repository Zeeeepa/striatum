---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["docs/rfcs/0074-workflow-shape-and-adversary-pack-catalog.md", "docs/rfcs/0076-three-lane-code-and-doc-audit-workflow.md", "docs/operator/artifacts/active-runway-1-5/phase4/SYNTHESIS.md", "docs/operator/artifacts/rfc-0076-audit-remediation/catalog-followup/PLAN.md", "src/striatum/workflow_templates/catalog.json"]
---

# RFC 0074 Phase A Pack Discovery
author: operator [self-declared: codex-operator]
status: open
date: 2026-05-22

## Summary

Phase A should finish a read-only authoring catalog, not add runtime
semantics. Graph shapes, role packs, adversary packs, and catalog variants are
metadata that help operators choose and author ordinary `workflow.json` trees.
They are not daemon state, model identity, session identity, lane authority,
or workflow-schema requirements.

Current source already has a partial Phase A scaffold: catalog collections for
`shape`, `lane_set`, `role_pack`, and `adversary_pack`; an
`implementation_panel` shape marked `example_only`; and starter
role/adversary packs. The next patch should normalize and complete that
metadata rather than invent another representation.

## Phase A Vocabulary

| Term | Phase A meaning | Required catalog fields |
|---|---|---|
| graph shape | A named job graph family with preview metadata. It may point to an example but does not imply generator support. | `template_id`, `kind: "shape"`, `display_name`, `summary`, `recommended_for`, `default_lane_sets`, `required_options`, `graph_preview` |
| role pack | A reusable authoring bundle of role ids and prompt defaults. | `template_id`, `kind: "role_pack"`, `display_name`, `summary`, `recommended_for`, `default_shapes`, `roles` |
| adversary pack | A reusable pressure pattern for reviews or skeptical roles. | `template_id`, `kind: "adversary_pack"`, `display_name`, `summary`, `recommended_for`, `default_shapes`, `postures` |
| catalog variant | A blessed combination exposed in docs/UI because it is common. | keep as shape metadata for Phase A: `role_packs`, `adversary_packs`, `generation_status`, optional `example_workflow_path` |

Use globally unique `template_id` values across catalog kinds unless the
implementation also adds explicit ambiguity handling for `templates show`.
Suffix role/adversary ids where needed to avoid collisions with same-named
graph shapes.

## Candidate Inventory

Phase A graph-shape entries to expose first:

| Shape | Status | Role pack | Adversary pack(s) |
|---|---|---|---|
| `implementation_panel` | current starter; keep `generation_status: "example_only"` | `implementation_panel_roles` | `maintainer_cost`, `operator_ergonomics` |
| `strategy_review` | add metadata-only | `strategy_panel_roles` | `product_strategy`, optional `security_privacy` |
| `premortem` | add metadata-only | `premortem_roles` | `premortem_postures` |
| `release_readiness` | add metadata-only | `release_readiness_roles` | `premortem_postures`, `operator_ergonomics` |
| `incident_response` | add metadata-only | `incident_response_roles` | `provenance_integrity`, `operator_ergonomics` |
| `code_doc_audit` | accepted RFC 0076 follow-on | `authority_docs_operator_audit` | `code_doc_audit_postures` |

Keep the remaining RFC 0074 candidates as backlog metadata until there is
reuse evidence: `architecture_tournament`, `red_team_repair`, `spec_to_tests`,
`backlog_triage`, `migration_plan`, `research_synthesis`, `decision_appeal`,
`dependency_upgrade`, `performance_budget`, and `operator_runbook`.

Initial role packs:

| Pack | Roles |
|---|---|
| `implementation_panel_roles` | `proposer_a`, `proposer_b`, `proposer_c`, `scorekeeper`, `arbitrator`, `dissent_reviewer` |
| `strategy_panel_roles` | `strategist`, `customer_persona`, `operator_persona`, `risk_reviewer`, `synthesizer`, `principal_decider` |
| `premortem_roles` | `proposal_author`, `failure_mode_reviewer`, `mitigation_planner`, `rollback_reviewer`, `final_reviewer` |
| `release_readiness_roles` | `release_manager`, `docs_reviewer`, `migration_reviewer`, `smoke_verifier`, `rollback_reviewer` |
| `incident_response_roles` | `reproducer`, `root_cause_analyst`, `fix_planner`, `verifier`, `retrospective_author` |
| `authority_docs_operator_audit` | `authority_runtime_auditor`, `docs_decision_drift_auditor`, `operator_adoption_auditor`, `audit_synthesizer`, `remediation_planner` |

Initial adversary packs:

| Pack | Postures |
|---|---|
| `security_privacy` | `security`, `privacy_redaction`, `supply_chain` |
| `maintainer_cost` | `future_maintainer`, `cost_complexity`, `testability` |
| `operator_ergonomics` | `operator_experience`, `new_user_onboarding`, `observability_debuggability` |
| `premortem_postures` | `failure_mode`, `rollback`, `compatibility_migration` |
| `product_strategy` | `skeptical_product`, `customer_persona`, `time_to_ship` |
| `provenance_integrity` | `data_integrity_provenance`, `evidence_auditor`, `formal_spec` |
| `code_doc_audit_postures` | `authority_drift`, `docs_drift`, `operator_ergonomics` |

`operator_ergonomics` is the only intentional overlap: keep it as the broad
RFC 0074 adversary pack/posture vocabulary, and let
`code_doc_audit_postures` include that pressure rather than defining an
audit-specific duplicate.

## RFC 0076 Fit

RFC 0076 should enter the catalog as an accepted follow-on, not as the first
RFC 0074 breadth example. Its hand-authored workflow has already proven the
shape, and D128 explicitly deferred generator/catalog integration to this
workstream.

Recommended metadata:

- graph shape: `code_doc_audit`
- role pack: `authority_docs_operator_audit`
- adversary pack: `code_doc_audit_postures`
- default lane set: `multi_review`; three fresh sessions remain acceptable
  when provider diversity is unavailable
- example reference: `docs/operator/workflows/rfc-0076-code-doc-audit/workflow.json`
- generation status: metadata/read-only until generator support lands

Do not add `striatum.audit_finding.v1` or an operator finding queue in this
catalog pass. Existing `finding`, `findings_ledger`, and `synthesis` artifacts
were sufficient for the first audit and remediation closure.

## Recommended Metadata Patch

1. Keep the existing `implementation_panel` shape and attach only read-only
   variant fields: `role_packs`, `adversary_packs`,
   `generation_status: "example_only"`, and `example_workflow_path`.
2. Add metadata-only shape entries for `strategy_review`, `premortem`,
   `release_readiness`, `incident_response`, and `code_doc_audit`.
3. Rename or suffix pack ids that would collide with shape ids before adding
   same-named shapes. Prefer `release_readiness_roles`,
   `incident_response_roles`, and `premortem_postures`.
4. Add the missing role/adversary packs listed above with flat role/posture id
   arrays. Structured prompt defaults can come later.
5. Expose packs only through read-only list/show/render and service responses.
   Generated workflow output must remain unchanged unless a later Phase B
   generator patch owns the behavior.

## Phase B Deferrals

Defer all generation and selection behavior: `workflow generate --shape` for
the expanded shapes, `--role-pack`, `--adversary-pack`, `proposal_count`,
`score_dimensions`, web chooser pack selectors, cost/artifact-volume
estimation, and RFC 0052 debate/panel artifacts. Also defer any change that
would make packs workflow validation inputs, daemon state, lane/model
identity, artifact schemas, or runtime gates.
