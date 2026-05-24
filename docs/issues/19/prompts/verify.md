# Verify Prompt: GH #19 + #21

Fresh-session: read only:
- `docs/issues/19/SPEC.md` + `docs/issues/21/SPEC.md`
- `docs/issues/19/SCOPE.md`
- `docs/issues/19/build/HANDOFF.md`
- The source files + tests added/modified per HANDOFF.

Do NOT read other reviewers' findings or implementer chat.

Produce `docs/issues/19/review/REVIEW.md`. Front matter:

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
inputs: ["docs/issues/19/SPEC.md", "docs/issues/21/SPEC.md", "docs/issues/19/SCOPE.md", "docs/issues/19/build/HANDOFF.md"]
review_posture: "ergonomics_dx"
verdict: "<accept | accept_with_findings | needs_revision>"
---
```

Byline: `author: reviewer-unknown-model-<NN>`. Plain markdown line.

## Mandatory checks per issue

### GH #21

1. Test exists that simulates "serve restart while retired-local-state has rows."
2. Test asserts row count + sha256 invariant (not just "no exception").
3. If integrity_check fails on an existing file, serve errors out cleanly (no silent re-init).

### GH #19

1. Test exists that creates a stale_lease repo_write job and exercises the new operator verb.
2. Test asserts state transition (stale_lease → claimable) AND audit row landed with operator_override flag + justification text.
3. Without `--force --justification`, the original refusal still fires (regression guard).

## Verdict

- `accept` — all mandatory checks pass.
- `accept_with_findings` — mandatory pass; minor ergonomics improvements recorded.
- `needs_revision` — any mandatory check fails.

Cycle: `max_iterations: 1` (one revision only).
