---
title: "RFC 0018 V1 build handoff (dogfood-016)"
date: 2026-05-09
---

# Build handoff: RFC 0018 V1 (review postures)

author: implementer-codex-gpt-5.5-001

## Scope

V1 ships RFC 0018 steps 1+2 against the contract in
`DESIGN_SYNTHESIS.md` plus the V1_ACCEPTANCE re-cast (workflow-
validation gate, not runtime build-completion gate). Step 3
(`verdicts.posture` column + introspection) deferred to V1.5.

## Files

### New

- `tests/test_review_postures.py` — 24 cases covering the V1 test
  plan: validator rejections (posture-on-non-review,
  unknown/empty/bare-prefix/whitespace-only invalid values),
  packet exposure (declared vs omitted, instruction sentence
  appending, neutral and custom posture handling),
  `required_review_postures` validator (non-build-job rejection,
  empty list, unknown entry, non-list, first-class entries,
  custom entries), reachability gate (forward edge, reverse
  edge, unreachable, disconnected), and zero-regression for
  posture-omitting workflows. All 24 pass.

### Modified

- `src/striatum/workflow.py`:
  - New module-level constants `ALLOWED_POSTURES` (frozenset of
    nine first-class postures) and `POSTURE_INSTRUCTIONS` (dict
    mapping posture name → deterministic sentence appended to
    the work packet's `review_policy.instruction`).
  - New helpers `_validate_review_posture` (rejects posture on
    non-review jobs; validates closed-set or `custom:<name>`
    membership; rejects empty/whitespace-only custom names) and
    `_validate_required_review_postures` (rejects field on
    non-build jobs; validates non-empty list of valid entries).
  - New helper `_validate_required_postures_reachable` walks the
    directed edge graph in both directions from each build with
    `required_review_postures` and refuses (`WorkflowError`,
    exit code 8) when any required posture is not the
    `review_posture` of a reachable review job.
  - All three helpers wired into `_validate_jobs` alongside the
    existing `_validate_reviewer_policy` invocation; the
    reachability gate runs once per workflow after per-job
    validation completes.
- `src/striatum/db.py`:
  - `_build_review_policy` extended to recognise `review_posture`
    as a third trigger field. When a review job declares only
    posture (no access/context), the block now triggers and
    defaults access/context for the rendered `instruction`.
    `posture` is included in the returned dict only when
    explicitly declared, so workflows that declare only
    access/context produce byte-identical packets to v1.6.0.
  - `instruction` gains the `POSTURE_INSTRUCTIONS[posture]`
    sentence (empty string for `neutral` and `custom:<name>`).

### Docs

- `docs/SPEC.md` § "Reviewer Policy" gains a "Review Postures
  (RFC 0018 V1)" subsection covering the vocabulary, the packet
  exposure, and the workflow-validation gate.
- `docs/UBIQUITOUS_LANGUAGE.md` adds entries for `review posture`
  and `required review postures`.
- `docs/DECISION_LOG.md` adds D069 (RFC 0018 V1 acceptance with
  the validation-time re-cast).
- `docs/TODO.md` adds F16 (marked done).
- `docs/rfcs/0018-focused-adversarial-review-postures.md` status
  → `accepted (V1; step 3 deferred)`. The RFC's "Step 2"
  prose is patched: the runtime build-completion gate is
  replaced with the workflow-validation gate, with an
  "Implementation note" explaining the lifecycle re-cast.
- `docs/rfcs/README.md` index row for RFC 0018 updated.
- `CHANGELOG.md` `## 1.7.0 — 2026-05-09` section.
- `pyproject.toml` and `src/striatum/__init__.py` bumped to
  `1.7.0`.

## CLI / API surface

No new CLI verbs or flags. The two new fields are workflow JSON
declarations only:

```json
{
  "id": "review_security",
  "type": "review",
  "review_posture": "security",
  ...
}

{
  "id": "implement",
  "type": "build",
  "required_review_postures": ["security", "threat_model"],
  ...
}
```

The work-packet shape gains an optional `review_policy.posture`
key (omitted when posture is undeclared).

## Test results

- `tests/test_review_postures.py`: 24 / 24 pass.
- `make lint`: clean (1 unused import auto-fixed by ruff).
- `make typecheck`: 59 source files, no issues.
- Full `make test`: pending — running while this handoff is
  drafted; final result attached on PR.

## Out of scope (V1)

- `verdicts.posture` column (RFC 0018 step 3, deferred).
- `striatum status` `verdicts_by_posture` block.
- `run summary` / `evidence export` / `run graph --format json`
  posture surfacing.
- Dashboard verdicts panel posture chips.
- Web UI posture chip.
- Doctor check for `build_missing_required_posture_review`
  (subsumed by the workflow-validation gate).

These hinge on the `verdicts.posture` column landing first;
V1.5 picks them up after V1 is in operator hands.

## Re-cast: workflow-validation gate vs runtime gate

The RFC's "step 2" originally described a runtime build-
completion gate. That gate deadlocks against striatum's
lifecycle: a build's `complete` mutation precedes its downstream
review's verdict by construction. V1 instead enforces the gate
at workflow validation time (`workflow validate` + `run prepare`
both refuse with exit 8). Runtime enforcement is preserved by
the existing edge-verdict gate plus run-completion semantics. The
D069 row, the dogfood V1_ACCEPTANCE artifact, and the patched RFC
text all record this re-cast explicitly.
