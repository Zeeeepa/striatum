# Review Design Prompt: RFC 0048 Phase A synthesis

Produce `docs/dogfood/057/review/design/REVIEW.md`. Front matter:

```
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
inputs: ["docs/dogfood/057/DESIGN_SYNTHESIS.md"]
review_posture: "ergonomics_dx"
verdict: "<accept | accept_with_findings | needs_revision>"
---
```

Byline AFTER the front-matter block. Plain markdown line, lowercase `author:`, no decoration. Slug shape: `reviewer-unknown-model-<NN>`.

This is the **gate** before implementation. Fresh-session: read ONLY the synthesis + the cited code files. Do NOT read the three design inputs — that's the synthesis author's job already done.

## Mandatory checks (bounce on any failure)

1. **Method-list completeness.** All 16 Phase A methods enumerated? Track A: 9 workflow-loop. Track B: 7 recovery + evidence. Cross-check against `src/striatum/cli/mutations.py`, `src/striatum/cli/recovery.py`, `src/striatum/cli/evidence.py`. Any missing → `needs_revision`.
2. **Per-method specificity.** For every method, the synthesis names: source path+function, destination path+function, PG tables (write + read), transaction shape, audit-event row(s), test file path. A method-row with any of those blank → `needs_revision`.
3. **Handler module boundary.** Synthesis picked one layout (per-method or per-cluster). Picked one handler signature. Picked one delegation-swap pattern. If two paths still presented → `needs_revision`.
4. **Audit-chain anchor.** Synthesis says exactly how `striatumd.events.prev_hash` chains on insert. Hand-wavy "we anchor it" → `needs_revision`.
5. **Half-ported transition.** Synthesis says exactly how `_route` decides PG vs SQLite per-method during the transition. Missing → `needs_revision`.
6. **`repository_id` enforcement.** Synthesis names the enforcement mechanism (column constraint, WHERE-clause discipline, or wrapper). Missing → `needs_revision`.
7. **Test paths.** Each method-port has a concrete test file path. Wildcard "tests for handlers" → `needs_revision`.

## Ergonomics_dx checks (degrade verdict, don't bounce)

- Handler error messages cite operator-actionable next steps.
- Delegation-swap pattern is greppable.
- Operator can tell at a glance which methods are PG-backed vs SQLite-backed.
- Test failure messages diff state directly, not just "assert False".

## Output

A finding artifact with per-bullet evidence. Cite synthesis line numbers and code references. Verdict:

- `accept` — every mandatory check passes; no ergonomics_dx degradation.
- `accept_with_findings` — every mandatory check passes; ergonomics_dx findings recorded as required follow-ups, not blockers.
- `needs_revision` — any mandatory check fails.

## Byline discipline

Plain markdown line AFTER the front-matter block. Lowercase `author:`. No decoration. Slug shape: `reviewer-unknown-model-<NN>`.

One-shot supervised invocation. If `striatum ack` is denied, write the artifact and exit normally.
