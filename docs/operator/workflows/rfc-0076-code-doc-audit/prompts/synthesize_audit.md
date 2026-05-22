# Synthesize Audit

Read the work packet first and use the exact `author:` line supplied in
`expected_artifacts`. Stay within the packet's write scope.

Read the three audit-lane artifacts and produce the synthesis artifact.
Do not reopen the audit as a fourth lane except to verify evidence needed
to reconcile conflicts.

The synthesis must:

- deduplicate overlapping findings and keep a merge table from lane ids
  to synthesized ids;
- preserve stable `AUD-###` references from the source lanes;
- call out conflicts between lanes and the evidence that resolves or
  leaves them open;
- classify each material finding by severity, category, owner surface,
  and follow-up path;
- separate authority regressions, docs drift, implementation gaps,
  operator ergonomics, test gaps, RFC-status issues, historical-only
  items, and open questions;
- recommend a priority order for follow-up work.

Use concrete evidence from the lane artifacts. Do not treat terminal
output, transcripts, marker files, tmux panes, or provider hooks as
authoritative workflow state. Preserve historical fixtures as provenance
unless a current doc or example claims their behavior is live.
