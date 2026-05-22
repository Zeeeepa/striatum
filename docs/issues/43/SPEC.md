# GH #43 - accept_with_findings in non-accepting status projection

Source: https://github.com/halbritt/striatum/issues/43

## Summary

`striatum status --json` reported `accept_with_findings` in
`latest_non_accepting_review_verdicts`, which drove misleading next actions on
an otherwise completed run.

## Acceptance

1. Non-accepting verdict projections exclude both `accept` and
   `accept_with_findings`.
2. `needs_revision` and `reject` remain visible.
3. Focused tests cover the completed-run `accept_with_findings` case.
