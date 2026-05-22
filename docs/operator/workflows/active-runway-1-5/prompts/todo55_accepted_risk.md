# Plan TODO 55 Accepted-Risk Implementation

Produce the expected synthesis artifact only. Do not edit source in this job.

Focus on D124: daemon-core workflow lint becomes the authoritative
accepted-risk surface. The plan must include:

- a migration and data model proposal for durable accepted-risk records;
- daemon methods, capabilities, registry/contract updates, and audit behavior;
- how accepted-risk records bind to immutable workflow snapshots or
  fingerprints and cite decision artifacts;
- CLI/MCP/UI surfaces that write or read the record without making workflow
  file metadata live authority;
- tests and guardrails, including daemon contract tests and command-authority
  matrix updates;
- a small first implementation slice with disjoint write scope.

Use valid `striatum.synthesis.v1` front matter and set the `author:` line to
the exact expected artifact author line from the work packet.
