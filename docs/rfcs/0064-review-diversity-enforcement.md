# RFC 0064: Workflow Risk Lint and Review Diversity Enforcement

## Status
Accepted / implemented (D220, closes #397) — durable accepted-risk persistence landed; the obsolete "Blocked Policy" clause is resolved and no further authoring-helper lint polish is planned

## Summary
Architecture remediation Phase 7 added workflow risk linting, review-diversity
warnings, strict override handling, generator/web surfacing, and validation
refusal for same-model review-pair risks.

## Motivation
Derived from the STRIATUM Architecture Review and Remediation Plan (2026-05-16).

## Implemented Surface

- `striatum workflow lint <workflow.json> --json` returns structured advisory
  warnings separately from validation errors.
- Rules cover same-model review pairs and revision cycles, stale review
  context, broad repo-write scope, repo-write jobs without per-job worktree
  isolation, and review workflows without a revision/escalation path.
- `workflow lint --strict` refuses warnings unless the operator supplies an
  override rationale, and JSON/API responses include the lint payload under
  error details.
- Workflow browser/detail pages, workflow generator previews, and the workflow
  chooser surface lint summaries without changing validation status.
- Strict overrides may carry `--accepted-risk-decision-id`.
- `workflow validate` refuses same-model review-pair and revision-cycle lint
  findings by default unless `--allow-same-model-pairing` is supplied.

## Blocked Policy (RESOLVED 2026-06-18)

> **Resolved:** durable accepted-risk persistence shipped — runtime migration
> 0013 `workflow_accepted_risks` + the `workflow.accept_risk` /
> `workflow.accepted_risks.list` RPCs and CLI. The product decision below (where
> accepted-risk evidence lives durably) was answered: an append-only daemon-owned
> table (runtime migration `go/pkg/db/sql/0013_workflow_accepted_risks.sql` + the
> `workflow.accept_risk` RPC/CLI). The original text is retained for provenance;
> per D220 (closes #397) no further authoring-helper lint polish is planned.

Accepted lint-risk persistence is not implemented because the durable authority
surface is undecided. `workflow lint` is currently a CLI-local authoring helper:
it reads workflow files and returns a result, but it does not mutate daemon
state, write audit rows, or create artifacts. Before implementation continues,
a product decision must choose where accepted-risk evidence lives durably:

- decision artifact linkage only,
- daemon audit row,
- workflow metadata,
- run-preparation record,
- another explicit durable artifact/table.

Until that decision exists, durable evidence is the operator-recorded decision
referenced by `--accepted-risk-decision-id`, and the lint command remains
non-mutating.
