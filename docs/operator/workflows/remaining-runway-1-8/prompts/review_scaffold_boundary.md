# Review Track Scaffold Boundary

Produce the expected boundary-review artifact only. Do not edit source, tests,
TODO, roadmap, or the operator brief in this job.

Review the scaffold artifact named in the work packet from an authority and
product-boundary posture. Return `accept`, `accept_with_findings`, or
`needs_revision`.

Focus on:

- whether the scaffold creates any new product decision instead of citing an
  accepted decision or RFC;
- whether live workflow authority remains daemon/PostgreSQL-owned;
- whether terminal panes, transcripts, marker files, workflow-file metadata,
  or provider hooks are incorrectly treated as state authority;
- whether any external service, provider API, telemetry, hosted persistence,
  memory dependency, or credential behavior slipped into core scope;
- whether the proposed validation evidence would be enough for a later
  implementer to know when the track is done.

Use valid `striatum.finding.v1` front matter and set the `author:` line to the
exact expected artifact author line from the work packet.
