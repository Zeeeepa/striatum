# Task: adjudicate claims against ground truth

Read only `CLAIM_LEDGER.md` and `VERIFICATION_REPORT.md`. Publish
`COLLABORATION_LEDGER_${cycle}.md` with YAML front matter:

```yaml
---
verdict: accept            # or: needs_revision
---
```

Record `needs_revision` if ANY claim:
- is stated `VERIFIED` but its witness `FAIL`ed or was absent;
- uses completion language above the status its witness earns;
- has a witness that does not actually exercise it (per the verifier).

Otherwise `accept`. In the body, list each offending claim, the witness that
failed it, and the required resolution (build it, or downgrade the status).

`needs_revision` routes the run back to the builder; the cycle is bounded.
Follow your role definition (`roles/adjudicator.md`).
