# Plan TODO 63 Service Boundary Cleanup

Produce the expected synthesis artifact only. Do not edit source in this job.

Focus on daemon client/service boundary residuals. The artifact must:

- identify remaining `/v1/invoke`, service, chat, or web paths that still
  route through compatibility seams;
- classify which paths are CLI-local authoring only versus daemon-routed live
  workflow control;
- preserve removed composites as `method_unknown` unless a future decision
  reintroduces them;
- name guardrail tests for daemon DTO routing and no direct PG/SQLite reach;
- define a small implementation batch with disjoint write scope.

Use valid `striatum.synthesis.v1` front matter and set the `author:` line to
the exact expected artifact author line from the work packet.
