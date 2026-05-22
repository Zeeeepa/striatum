# Plan TODO 61 Residual Cleanup

Produce the expected synthesis artifact only. Do not edit source in this job.

Focus on remaining Go/Python retirement and legacy SQLite fixture debt. The
artifact must:

- identify the next bounded batch of legacy SQLite test imports/skips to
  convert or delete;
- preserve explicitly historical fixtures where still needed;
- name guardrail tests that should tighten after the batch;
- avoid deciding TODO 55/56/59/60 policy again;
- define a small implementation batch with disjoint write scope.

Use valid `striatum.synthesis.v1` front matter and set the `author:` line to
the exact expected artifact author line from the work packet.
