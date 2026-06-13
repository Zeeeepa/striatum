Review the #217 implementation and publish the required finding.

Check:

- doctor only runs the artifact-anchor cross-check after blob storage is
  configured, reachable, and repo-provisioned;
- mismatch, missing-file, and unreadable-anchor cases produce stable problems
  plus verbose `problem_records`;
- no artifact bodies, blob credentials, or raw diagnostic payloads are printed;
- git inspection is read-only and does not move refs or touch the operator
  checkout;
- tests cover the acceptance cases without over-mocking internal helpers;
- docs describe the new check and skip conditions accurately.

Use `accept` only if the implementation is merge-ready.
