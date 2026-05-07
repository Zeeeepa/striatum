# Review prompt — DOT export change

## Task

Independently review the dogfood-001 draft change. Your access scope is
`artifact_augmented`: you may read the draft handoff, the modified source
files, and the new test, but do not browse the rest of the repository.

## Context to read

- `docs/dogfood/001/DRAFT_HANDOFF.md` — author's summary.
- `src/striatum/workflow.py` — the new `workflow_graph_dot` function.
- `src/striatum/cli/parser.py` — the new `--format dot` choice.
- `src/striatum/cli/dispatch.py` — the dispatch path for `dot`.
- The new test in `tests/test_cli_mvp.py`.
- `docs/dogfood/001/HARNESS_PROPOSAL_TEMPLATE.md` (in case you want to
  file your own harness friction).

## What to check

1. **Parity.** Does the DOT output represent the same workflow graph as
   Mermaid and JSON? Same nodes? Same edges? Cycles annotated?
2. **Validity.** If you have `dot` on PATH, run `dot -Tsvg` against the
   output. Does it parse?
3. **Determinism.** Are node names stable (`n0`, `n1`, ...) so two runs
   on the same workflow produce byte-identical output?
4. **Test coverage.** Does the new test cover the cycle edge and
   parallel-group rendering paths, not just the happy-path nodes?
5. **Scope hygiene.** Did the author touch only files within the
   declared `write_scope.allowed_paths`?
6. **Doc currency.** Are `docs/SPEC.md` and `README.md` updated?

## Verdict

Pick one and submit via `striatum submit-review`:

- `accept` — clean draft, no blocking findings.
- `accept_with_findings` — accepted, but capture the findings for
  followup (downstream still proceeds).
- `needs_revision` — the workflow declares a one-shot revision cycle.
  The author gets one more attempt. Use this if the draft is salvageable
  but currently incorrect.
- `reject` — the draft fails the gate. Use sparingly.

## Finding artifact

Write your review at `docs/dogfood/001/review/FINDING.md` with valid
`striatum.finding.v1` front matter (the publisher will validate it):

```yaml
---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"   # or accept | needs_revision | reject
severity: "low"                           # info | low | medium | high | critical
tags: ["dogfood-001", "graph"]
---
```

Then submit:

```bash
striatum submit-review \
  --session-id "$REVIEWER" \
  --job-id "$REVIEW_JOB_ID" \
  --lease-id "$REVIEW_LEASE_ID" \
  --kind finding \
  --logical-name review_finding \
  --path docs/dogfood/001/review/FINDING.md \
  --verdict accept_with_findings \
  --json
```
