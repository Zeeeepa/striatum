# Plan TODO 59 Corpus Contract V2 Implementation

Produce the expected synthesis artifact only. Do not edit source in this job.

Focus on D126: Corpus Contract V2 uses composite corpus identity, graduated
redaction tiers, workflow opt-in augmentation references, hybrid archive
bundles, verification replay by default, read-only semantic inspection, no
comparative replay, deep-chain verification, and optional daemon audit-chain
cross-check. The plan must include:

- V2 manifest/schema fields and backward compatibility for V1 bundles;
- archive default changes and verification depth;
- augmentation-reference packet behavior that does not make external memory a
  live dependency;
- where accepted-risk and Git snapshot data should be incorporated after
  those surfaces exist;
- Python and Go export/read surfaces that must stay in parity;
- tests and no-Engram/no-memory-dependency guardrails;
- a small first implementation slice with disjoint write scope.

Use valid `striatum.synthesis.v1` front matter and set the `author:` line to
the exact expected artifact author line from the work packet.
