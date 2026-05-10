# Review Build Prompt

Produce the review artifact required by the work packet with valid `striatum.finding.v1` front matter.

Review the implementation under your assigned posture. Verify behavior, tests, docs, migrations, and workflow compatibility. Inspect the repository within the review write scope policy.

Required checks:

- RFC 0026 behavior does not permit unattested sessions to mint lane-typed bylines;
- RFC 0027 behavior does not overclaim model-token provenance, independent decision provenance, or sealed containment before hard local write denial exists;
- tests cover happy paths and adversarial bypass attempts appropriate to the implemented phase;
- docs and status/evidence/run-summary surfaces describe actual guarantees honestly;
- migrations are forward-compatible and existing advisory workflows remain usable;
- write scopes and fixtures do not normalize direct `.striatum/` edits or hidden transcript capture.

Use `needs_revision` for behavior gaps, missing tests for core claims, migration breakage, or documentation that overstates provenance. Use `accept_with_findings` only for non-blocking cleanup.
