# RFC 0002: Reviewer Independence Policy

Status: accepted
Date: 2026-05-06
Context: ARIS paper review, `docs/SPEC.md`,
`examples/rfc-0014-operational-artifact-home/workflow.json`

## Problem

`striatum` already supports fresh sessions, independent review jobs, and
model-portable lanes. The workflow contract still leaves reviewer independence
partly implicit. A reviewer can be fresh or persistent, but the workflow does
not explicitly say what the reviewer may read or whether it should retain state
across rounds.

That makes two different review modes look the same:

- a fresh document-only review intended to avoid confirmation bias;
- a cross-round repo-level review intended to verify that prior findings were
  actually fixed.

Both are useful, but they have different risk profiles.

## Goals

- Make reviewer context policy and access scope explicit, validated workflow
  fields.
- Keep the fields model-portable and adapter-neutral.
- Make work packets tell reviewers exactly what they may inspect.
- Preserve local-first constraints: access scope is repository/data access, not
  permission to use external services.

## Non-Goals

- Do not require different model families in core scheduling.
- Do not add provider-specific review APIs.
- Do not make same-family review invalid; workflows may still choose it.
- Do not grant network access to local-only workflows.

## Proposal

Add optional review-policy fields to review jobs:

```json
{
  "reviewer_access_scope": "document_only",
  "reviewer_context_policy": "fresh"
}
```

Accepted `reviewer_access_scope` values:

- `document_only` - reviewer reads only target documents listed as inputs;
- `artifact_augmented` - reviewer may also read supporting artifacts, reports,
  ledgers, rendered outputs, and test results listed as inputs;
- `repo_level` - reviewer may inspect the repository within the job's declared
  read/write scope.

Accepted `reviewer_context_policy` values:

- `fresh` - reviewer must use a fresh role/session and avoid relying on prior
  thread state;
- `cross_round` - reviewer may retain context to verify whether previously
  raised issues were resolved.

Validation rules:

- `reviewer_context_policy: "fresh"` implies `fresh_session_required: true`.
- `repo_level` review must not broaden write scope; reviewers still write only
  their declared artifacts unless the job is explicitly repo-write.
- Work packets expose both fields and include a short instruction block for the
  reviewer.

## Acceptance Criteria

- Workflow validation accepts the new fields and rejects unknown values.
- Existing workflows without the fields continue to validate with current
  behavior.
- The RFC 0014 fixture labels root reviews as fresh and document/artifact
  scoped.
- `docs/SPEC.md` documents the two axes.

## Open Questions

- Should the runner enforce `fresh` by rejecting claims from non-fresh sessions,
  or is warning in the work packet enough for V1?
- Should cross-model family separation be a first-class policy later, or remain
  workflow documentation?

## Implementation Notes

V1 implementation lives in `src/striatum/workflow.py` and `src/striatum/db.py`:

- `validate_workflow()` accepts the two optional fields on `type: "review"`
  jobs, rejects unknown values with the documented error strings, rejects the
  fields on non-review jobs, and rejects the explicit conflict between
  `reviewer_context_policy: "fresh"` and `fresh_session_required: false`.
- When a review job declares `reviewer_context_policy: "fresh"` and does not
  set `fresh_session_required`, `create_run()` silently stores the prepared
  job row with `fresh_session_required = 1` via
  `_effective_fresh_session_required()`. This preserves the implication
  without changing the user's workflow JSON.
- `db.build_packet()` adds a `review_policy` block to the work packet only
  when the workflow declared at least one of the fields. The block carries
  `access_scope`, `context_policy`, and a deterministic `instruction` string
  built from registered access-scope and context-policy templates. Workflows
  that do not declare the fields produce packets unchanged.
- The `examples/rfc-0014-operational-artifact-home/workflow.json` fixture
  now labels its three independent root reviews as `document_only` and
  `fresh`. Other fixtures continue to omit the fields and operate exactly as
  they did before V1 acceptance.

Tests live in `tests/test_reviewer_policy.py`; existing `tests/test_cli_mvp.py`
fixtures continue to pass without `review_policy` blocks, demonstrating that
unaffected workflows are unchanged.
