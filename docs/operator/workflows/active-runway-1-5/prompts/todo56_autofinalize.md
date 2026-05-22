# Plan TODO 56 Auto-Finalize Follow-Up

Produce the expected synthesis artifact only. Do not edit source in this job.

Focus on D125: global auto-finalize remains dry-run projection, live
auto-finalize remains workflow opt-in, and default-on behavior waits for the
accepted evidence gate. The plan must include:

- lane-finalization visibility surfaces for status, dashboard, and web;
- skipped-candidate cause classes with stable names;
- consecutive-failure circuit-breaker semantics and whether durable state is
  required;
- preservation of dry-run/no-side-effect behavior;
- tests for live workflow opt-in, dry-run defaults, skipped causes, and
  breaker behavior;
- a small first implementation slice with disjoint write scope.

Use valid `striatum.synthesis.v1` front matter and set the `author:` line to
the exact expected artifact author line from the work packet.
