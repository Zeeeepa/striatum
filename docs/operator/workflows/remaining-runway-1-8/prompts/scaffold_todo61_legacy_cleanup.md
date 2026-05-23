# Scaffold TODO 61 Legacy Cleanup

Produce the expected scaffold artifact only. Do not edit source, tests, TODO,
roadmap, or the operator brief in this job.

Focus on the next bounded TODO 61 legacy cleanup slice: remaining legacy
SQLite fixture/import cleanup, skipped-test conversion, and guardrail
tightening without restoring retired compatibility paths.

The scaffold must include:

- current-state assumptions from RFC 0068 and recent TODO 61 cleanup evidence;
- a map of the highest-value remaining skipped or quarantined legacy fixtures;
- a small first conversion/deletion slice with focused tests;
- guardrail tightening after converted coverage no longer needs broad
  exceptions;
- implementation write scopes, sequencing, and regression commands;
- explicit non-scope for reopening repo-local SQLite, `.striatum/` live state,
  or the legacy daemon registry in production.

Use valid `striatum.synthesis.v1` front matter and set the `author:` line to
the exact expected artifact author line from the work packet.
