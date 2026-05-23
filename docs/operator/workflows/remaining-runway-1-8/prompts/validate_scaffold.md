# Validate Track Scaffold

Produce the expected validation artifact only. Do not edit source, tests, TODO,
roadmap, or the operator brief in this job.

Validate the scaffold artifact named in the work packet. Return `accept`,
`accept_with_findings`, or `needs_revision`.

Check that the scaffold:

- matches the track named by the job objective and preserves the global
  1-through-8 order;
- stays within implementation planning and does not claim source changes were
  made by this workflow;
- names concrete implementation write scopes, serialization points, and tests;
- honors the relevant accepted decisions and RFC boundaries;
- keeps daemon/PostgreSQL as live authority and repository artifacts as
  provenance;
- avoids hosted services, telemetry, transcript capture, provider SDKs,
  external persistence, and repo-local SQLite authority unless a cited decision
  explicitly allows them.

Use valid `striatum.finding.v1` front matter and set the `author:` line to the
exact expected artifact author line from the work packet.
