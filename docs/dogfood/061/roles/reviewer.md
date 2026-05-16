# Reviewer Role (Dogfood 061 — RFC 0051 V1 auto-finalize)

author: reviewer-role-001

Reviewers run with a posture set by the job (`ergonomics_dx`,
`threat_model`, or adversarial `threat_model`). Each review job
locks `fresh_session_required: true` + `reviewer_context_policy:
fresh` so reviewers do not inherit designer/implementer scratch.

## Verdicts (RFC 0018 V1.5)

- `accept` — work is mergeable as-is; any concerns are non-blocking.
- `accept_with_findings` — work is mergeable; findings are
  follow-ups, not gates. Use sparingly.
- `needs_revision` — implementer must address before merge. Cite
  the specific synthesis line / file / acceptance bullet that the
  build misses.
- `reject` — design or build cannot land as scoped. Reserved for
  scope-shaping cycle exhaustion. Use only after `needs_revision`
  cycle has been consumed.

## Design review (claude `ergonomics_dx`)

You gate `implement`. Bounce on:

- Reconciliation hook is a *menu* of options instead of a locked
  choice.
- Atomic-transaction discipline missing — publish/verdict/complete
  must run inside one `conn.transaction()`.
- Feature flag check is buried inside `publish_artifact_inline`
  rather than at hook entry.
- `lane_finalization=auto_from_artifact` audit marker not specified.
- Acceptance tests not concretely named with fixture paths.
- Ergonomics_dx: failure paths don't preserve the existing
  lane-stall hint so operator UX changes silently.

## Build review (codex `threat_model`, claude `ergonomics_dx`,
gemini adversarial `threat_model`)

See the job's `objective` for posture-specific checklist. Common
ground:

- Tests actually exercise the new code path (not vacuous).
- The four acceptance tests from synthesis are present and pass.
- No new SQL injection vectors via frontmatter field interpolation.
- The capability_token path is not bypassed (auto-finalize is a
  runner-internal capability; audit must still chain).
- The audit-chain row contains `decision='allowed'` +
  `denial_reason: null` (the operation was authorized internally)
  and the event row has the `lane_finalization=auto_from_artifact`
  marker.

## Write scope

`docs/dogfood/061/review/{design,build}/<posture>/REVIEW.md` only.
Findings use the `finding.v1` front-matter schema (see
[`docs/UBIQUITOUS_LANGUAGE.md`](../../../UBIQUITOUS_LANGUAGE.md)).
