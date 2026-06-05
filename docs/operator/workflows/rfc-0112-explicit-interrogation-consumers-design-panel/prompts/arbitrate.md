# Arbitrate RFC 0112

Read the problem brief, all proposals, all scorecards, and the tradeoff ledger.
Publish `ARBITRATOR_SYNTHESIS.md` at the packet's expected artifact path.

The synthesis must choose one implementation-ready plan. It must answer these
questions explicitly:

- Is `interrogation_targets` the right field name and JSON shape?
- Is `required: true` advisory in V1 or a hard completion/verdict gate?
- Are multiple explicit targets allowed in V1?
- Which terminal mutation paths call the generalized release hook?
- What exact RFC 0105 fixture proves ACE can graduate after implementation?
- Does packet projection belong under `context.interrogation_targets`, or a
  different work-packet namespace?

Also include the implementation sequence, test sequence, docs sequence, risks,
rollback path, and any changes needed to RFC 0112 before acceptance.

This is design arbitration only. Do not update RFC status, the decision log, the
spec, source code, or VERSION.
