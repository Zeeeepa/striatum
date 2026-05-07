# RFC 0003: Support Ledgers And Evidence Audits

Status: accepted
Date: 2026-05-06
Context: ARIS paper review, `docs/SPEC.md`,
`docs/RFC_0014_DOGFOOD_FIX_SPEC.md`

## Problem

Long-running agent workflows can produce polished artifacts whose claims outrun
their evidence. `striatum` already tracks artifacts and verdicts, but it
does not have a general artifact contract for answering:

> Which claims did this artifact make, and what supports each one?

This matters for RFC syntheses, implementation summaries, run reports, test
reports, final review verdicts, and future Engram operational gates. A normal
review can miss a subtle mismatch between the artifact's claims and the
underlying files or command outputs.

## Goals

- Add a reusable support-ledger artifact type.
- Let workflows insert evidence-audit jobs between artifact production and
  downstream acceptance.
- Keep support ledgers curated and redacted; no transcript capture by default.
- Make evidence audits inspect file paths, command summaries, hashes, and
  reports rather than relying on the producing agent's prose.

## Non-Goals

- Do not turn `striatum` into Engram's belief/claim database.
- Do not store private corpus content in runner artifacts.
- Do not require every small workflow to maintain a support ledger.
- Do not add automated factual verification against the internet.

## Proposal

Add a new artifact kind:

```text
support_ledger
```

A support ledger is a Markdown artifact with stable claim ids:

```text
| ID | Claim | Support | Evidence paths | Status |
| --- | --- | --- | --- | --- |
| SL001 | Tests passed. | `pytest -q` completed with exit 0. | `RUN_EVIDENCE_...md` | supported |
```

Supported statuses:

- `supported`
- `partially_supported`
- `unsupported`
- `contradicted`
- `not_audited`

Add a job type or job convention:

```text
evidence_audit
```

An evidence-audit job reads a produced artifact, its support ledger, and the
referenced evidence artifacts. It emits a review artifact with verdicts for
ledger rows and may block downstream acceptance when high-severity rows are
unsupported or contradicted.

Recommended workflow pattern:

```text
artifact -> support ledger -> evidence audit -> downstream review/synthesis
```

## Acceptance Criteria

- Workflow validation accepts `support_ledger` as an artifact kind.
- Example prompts define how to write a support ledger without private content.
- At least one fixture demonstrates an evidence-audit job over a run evidence
  export or synthesis.
- `status --json` and evidence export include support-ledger artifacts like any
  other curated artifact.

## Open Questions

- Should support ledgers be mandatory for syntheses that recommend accepting an
  RFC?
- Should the runner model claim rows structurally in SQLite, or keep ledgers as
  artifact files in V1?

## Implementation Notes

- Migration version 5 drops the SQL `CHECK (artifact_kind IN (...))` on the
  `artifacts` table. Allowed kinds now live in
  `striatum.artifacts.ALLOWED_ARTIFACT_KINDS` and `publish-artifact` rejects
  unknown kinds with `ArtifactError` (exit code 6). Workflow validation imports
  the same set and rejects `expected_artifacts.kind` values it does not
  recognize with `WorkflowError` (exit code 8).
- `support_ledger` is added to `ALLOWED_ARTIFACT_KINDS` and registered in
  `FRONT_MATTER_SCHEMAS` as `striatum.support_ledger.v1`. Required front-matter
  fields are `schema_version`, `artifact_kind: support_ledger`, and
  `audited_artifact`; optional `claim_count` is a non-negative integer.
- `examples/support-ledger-flow/` is the reference fixture. Its
  `evidence_audit` job reuses the existing `review` job_type; the field
  `_comment_evidence_audit` documents that "evidence audit" is a workflow
  convention, not a new job_type.
- `tests/test_artifact_schemas.py` covers the new kind, its front-matter
  schema, and the support-ledger flow end to end.
