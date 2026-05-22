# Compile The Tradeoffs

Create `examples/implementation-panel-flow/artifacts/TRADEOFF_LEDGER.md` with
`striatum.findings_ledger.v1` front matter:

```yaml
---
schema_version: striatum.findings_ledger.v1
artifact_kind: findings_ledger
---
```

Normalize the three proposals and scorecards into one ledger:

- Decision criteria and weights, if the problem brief declares any.
- Score table by proposal and dimension.
- Disagreements or missing evidence.
- Highest-risk assumptions.
- Best candidate option, if the evidence already points clearly.

Do not make the final decision. Preserve enough evidence for arbitration.
