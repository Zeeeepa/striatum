# Dogfood Run Friction Log

Aggregate scan-friendly log of friction encountered during dogfood
iterations. New entries append to the top.

Each entry shape:

```text
## <dogfood-id> — <RFC or topic> — <YYYY-MM-DD>

**Severity:** info | low | medium | high | critical
**Nature:** <one-line>
**Status:** open | resolved | deferred

<one to three paragraphs of context>

**Mitigation / follow-up:** <what to do next, if anything>
```

Entries are operator-readable shorthand. Per-run
`harness_improvement_proposal` artifacts (RFC 0005 schema) under
`docs/dogfood/<id>/findings/HARNESS-NNN.md` remain the structured
form when a finding is substantive enough to publish through the
runner. This log is the lighter-touch register for friction that
doesn't need a full schema-validated artifact.

---
