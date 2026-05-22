---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
inputs: ["rfc_0076", "rfc_0074", "workflow_types_guide", "audit_remediation_plan", "rfc_0076_audit_remediation_workflow"]
---

# RFC 0076 Catalog Follow-Up Plan
author: catalog-planner-claude-code-001
status: open
date: 2026-05-22

## Summary

RFC 0076 is accepted by D128 with the explicit caveat that "Generator/catalog
integration remains future catalog work; hand-authored workflows may use the
accepted shape now." The first runnable audit workflow at
`docs/operator/workflows/rfc-0076-code-doc-audit/workflow.json` proved the
shape works without any catalog or schema additions, and the remediation
closure workflow in
`docs/operator/workflows/rfc-0076-audit-remediation/workflow.json` is also
hand-authored.

This plan classifies four post-acceptance candidates surfaced by RFC 0076
open questions and the remediation plan's tracking section. None require
immediate work to make the audit shape usable. Two are reasonable Phase A
additions to RFC 0074 when its catalog metadata pass lands; two should
remain no-action until concrete reuse evidence justifies a new product
surface.

This artifact does not implement any catalog change; per the job objective,
it only classifies the work.

## Classification Table

| Candidate | Source | Classification | Rationale |
|---|---|---|---|
| Generated `code_doc_audit` workflow template | RFC 0076 §7; RFC 0076 acceptance criteria; RFC 0074 §2 | Deferred to RFC 0074 Phase A | The hand-authored workflow already validates and runs. Generation is a convenience, not a blocker. |
| Role pack `authority_docs_operator_audit` and adversary pack `authority_drift`/`docs_drift`/`operator_ergonomics` | RFC 0076 §7; RFC 0074 §4-5 | Deferred to RFC 0074 Phase A | The role/adversary vocabulary belongs in RFC 0074's catalog. Names are well-defined; promotion is metadata-only. |
| Dedicated `striatum.audit_finding.v1` front-matter schema | RFC 0076 §3; RFC 0076 Open Question 1; remediation plan tracking section | No action | The first run produced findings under the existing `finding`/`findings_ledger` schemas. No concrete missing field has been demonstrated. |
| Operator UI issue-like projection for audit findings | RFC 0076 Open Question 5; remediation plan tracking section | No action | This would conflate artifact-backed claims with daemon-owned workflow state. Existing artifact storage and `docs/issues/<N>/` already cover the use case. |

## Classifications

### CAT-001: Generated `code_doc_audit` workflow template

classification: deferred to RFC 0074 Phase A
severity: low
owner_surface: workflow catalog metadata; `striatum workflow generate`;
`striatum workflow templates list`; `docs/WORKFLOW_TYPES.md`
follow_up: RFC 0074 Phase A catalog metadata pass

**What was proposed.** RFC 0076 §7 proposes adding `code_doc_audit` as a
graph shape entry in the workflow generator and template catalog so an
operator can run `striatum workflow generate --shape code_doc_audit ...`
instead of copying the hand-authored workflow tree.

**Why this is not immediate.** Three reasons:

1. The RFC 0076 acceptance criteria explicitly allow a hand-authored
   workflow to satisfy "a runnable example workflow exists for
   `code_doc_audit`". The audit shape is already runnable; D128 made that
   the gating evidence.
2. RFC 0074 §2 lists the first five catalog entries worth adding:
   `implementation-panel-flow`, `strategy-review-flow`, `premortem-flow`,
   `release-readiness-flow`, `incident-response-flow`. `code_doc_audit` is
   not on that initial list. Inserting it ahead of the proposed Phase A
   set would skew the catalog vocabulary toward Striatum-internal audit
   work before the broader shape vocabulary lands.
3. The hand-authored workflow has now been used twice (the original
   audit, then this remediation closure run). Two uses is not yet a
   compelling reuse signal that justifies generation ahead of RFC 0074
   Phase A.

**Recommended action when this work picks up.** Add `code_doc_audit` to
the RFC 0074 Phase A catalog metadata pass as a sixth or later entry,
binding it to the `authority_docs_operator_audit` role pack (CAT-002).
Generation can emit the same shape the hand-authored workflow already
uses; do not change the graph or expected artifacts.

**No new RFC required.** RFC 0076 already owns the shape; RFC 0074 already
owns the catalog mechanism. This is a metadata addition only.

### CAT-002: Role pack and adversary pack catalog entries

classification: deferred to RFC 0074 Phase A
severity: low
owner_surface: workflow catalog metadata; `docs/WORKFLOW_TYPES.md`;
`docs/UBIQUITOUS_LANGUAGE.md` if catalog vocabulary is promoted
follow_up: RFC 0074 Phase A catalog metadata pass

**What was proposed.** RFC 0076 §7 names:

- role pack `authority_docs_operator_audit`
- default adversary pack made up of `authority_drift`, `docs_drift`,
  `operator_ergonomics`

RFC 0074 §4-5 owns the broader role-pack and adversary-pack vocabulary,
listing `strategy_panel`, `implementation_panel`, `red_team_repair`,
`release_readiness`, `incident_response`, `migration_planning`,
`research_synthesis`, plus adversary packs `security_privacy`,
`maintainer_cost`, `operator_ergonomics`, `premortem`, `product_strategy`,
`provenance_integrity`.

**Why this is not immediate.** The two RFC 0076 audit packs are
well-defined inline. The hand-authored workflow encodes the same roles
(authority_runtime auditor, docs_decision_drift auditor,
operator_adoption auditor) directly in its `roles` block. There is no
intermediate UX layer demanding catalog promotion before RFC 0074 Phase A
lands.

`operator_ergonomics` already overlaps between RFC 0076 and RFC 0074's
named adversary packs. Promoting both at once will let RFC 0074 Phase A
resolve that name overlap rather than locking it in piecemeal.

**Recommended action when this work picks up.** During the RFC 0074
Phase A metadata pass:

- Add `authority_docs_operator_audit` as a role pack with the three audit
  roles plus a synthesizer and a remediation planner.
- Either add the three RFC 0076 adversaries as a single
  `code_doc_audit_postures` adversary pack, or merge `operator_ergonomics`
  with the RFC 0074 adversary pack of the same name.
- Update `docs/WORKFLOW_TYPES.md` to reference the catalog names instead
  of describing the lanes inline.

**No new RFC required.** This is RFC 0074 Phase A scope.

### CAT-003: Dedicated `striatum.audit_finding.v1` front-matter schema

classification: no action
severity: info
owner_surface: artifact schema registry; finding validation; `docs/HOW_TO_AGENT.md`
follow_up: revisit only if a concrete missing field is demonstrated by a
future audit run

**What was proposed.** RFC 0076 Open Question 1 asks whether audit
findings should get a dedicated `striatum.audit_finding.v1` front-matter
schema, instead of using the existing `finding` and `findings_ledger`
schemas. The RFC 0076 audit remediation plan retains this as an open
tracking item.

**Why no action is the right answer right now.**

1. RFC 0076 §3 explicitly states that V1 can use existing `finding` and
   `findings_ledger` artifact kinds. That was the chosen path for the
   first run.
2. The first audit produced authority-runtime, docs-decision-drift, and
   operator-adoption findings under existing schemas. The synthesis at
   `docs/operator/artifacts/rfc-0076-code-doc-audit/SYNTHESIS.md` and
   remediation plan at
   `docs/operator/artifacts/rfc-0076-code-doc-audit/REMEDIATION_PLAN.md`
   used `striatum.synthesis.v1` front-matter and required no audit-only
   fields.
3. The recommended finding entry shape in RFC 0076 §3 (id, severity,
   category, claim, evidence, impact, recommended_action, follow_up) is
   prose inside a finding artifact, not a structured front-matter
   contract. Adding `striatum.audit_finding.v1` would freeze a
   first-attempt vocabulary into the schema registry before reuse
   evidence exists.
4. Schema additions are not free: they require validation code, schema
   docs, and migration handling for any existing finding artifacts. That
   cost is justified only when existing schemas fail to express
   information operators or downstream jobs need.

**When this might change.** If a second or third audit run produces
fields the synthesizer cannot derive from `finding` plus
`findings_ledger` (for example, structured per-finding evidence rows that
need machine validation, or category enums that operator tooling must
filter on), open a follow-up to add the schema. Until then, treat the
existing schemas as sufficient.

**No catalog change required.**

### CAT-004: Operator UI issue-like projection for audit findings

classification: no action
severity: info
owner_surface: web UI; daemon RPC; `docs/architecture/COMMAND_AUTHORITY_MATRIX.md`
follow_up: revisit only if multiple audits produce a finding backlog the
artifact tree cannot navigate

**What was proposed.** RFC 0076 Open Question 5 asks whether the operator
UI should surface audit findings as a durable issue-like queue. The
remediation plan retains this as an open tracking item.

**Why no action is the right answer right now.**

1. Striatum's product boundary in `docs/SPEC.md` and `AGENTS.md` says the
   daemon-owned PostgreSQL instance is the only live workflow state.
   Repository files are durable provenance, not the message bus.
   Introducing an issue-queue projection of findings risks creating a
   second live-state surface (queue state) that does not currently exist
   and is not required by the audit shape.
2. The repository already has a `docs/issues/<N>/` workflow type (see
   the user's reference memory for ROADMAP.md plus `docs/issues/<N>/`
   workflow). That covers the case where a finding deserves to graduate
   into a tracked issue: write an issue under `docs/issues/<N>/` with
   `triage`, `fix`, and `verify` jobs and link the originating finding.
3. The current artifact tree under
   `docs/operator/artifacts/rfc-0076-code-doc-audit/` and
   `docs/operator/artifacts/rfc-0076-audit-remediation/` is already
   navigable: per-lane findings, synthesis, remediation plan, and (in
   this run) closure. The operator brief and operator plan link them.
   There is no demonstrated discovery or triage failure.
4. The web UI's current strength is workflow-tree discovery, validation,
   and run. A finding-queue projection would either need a new RPC and
   schema (cost) or replicate fields already present in the artifact
   front-matter (duplication).

**When this might change.** If five or more audit runs accumulate and the
operator cannot triage open findings across runs without grepping the
artifact tree, propose a new RFC for a finding-projection surface that
either:

- materializes from artifact front-matter on demand (no new live state);
  or
- introduces a daemon-owned finding state with explicit RPC and authority
  matrix entries (new live state, requires SPEC and authority-matrix
  updates).

The decision then chooses between those two shapes. Until that pressure
exists, keep the operator surface artifact-backed.

**No catalog change required.**

## Tracking Recommendation

- Bundle CAT-001 and CAT-002 as one Phase A addition to RFC 0074 when
  that RFC's catalog metadata pass is scheduled. They do not need a
  separate RFC.
- Leave CAT-003 and CAT-004 as documented no-action items. Reference this
  plan from the eventual closure artifact so future operators see why
  these open questions were resolved as "not yet" instead of "yes".
- Do not add an immediate-action row for any of the four candidates to
  `docs/TODO.md` or `docs/ROADMAP.md`. The accepted RFC 0076 plus the
  hand-authored workflow plus this run's remediation closure are
  sufficient for now.

## References

- `docs/rfcs/0076-three-lane-code-and-doc-audit-workflow.md` — accepted
  shape, §7 catalog entries, Open Questions 1 and 5.
- `docs/rfcs/0074-workflow-shape-and-adversary-pack-catalog.md` — Phase 0
  proposal, §2 candidate shapes, §4-5 role and adversary packs, Phase A
  metadata-only scope.
- `docs/WORKFLOW_TYPES.md` — current operator-facing selection guide that
  cites the hand-authored RFC 0076 workflow.
- `docs/operator/plans/rfc-0076-audit-remediation.md` — closure plan and
  open-question tracking.
- `docs/operator/workflows/rfc-0076-code-doc-audit/workflow.json` —
  hand-authored audit workflow used in the first run.
- `docs/operator/workflows/rfc-0076-audit-remediation/workflow.json` —
  hand-authored remediation closure workflow used by this run.
- `docs/operator/artifacts/rfc-0076-code-doc-audit/SYNTHESIS.md` and
  `REMEDIATION_PLAN.md` — first-run synthesis and remediation outputs
  produced under existing `striatum.synthesis.v1` front-matter.
