Produce `docs/operator/artifacts/issue-217-anchor-integrity-design/DESIGN.md`.

Use `docs/operator/workflows/issue-217-anchor-integrity-design/TASK.md` as the
working brief. The artifact should freeze:

- the exact doctor behavior to implement;
- skip conditions when blob storage is disabled, unhealthy, or not
  repo-provisioned;
- the source surfaces to inspect before writing code;
- focused test cases for disabled blob, healthy match, healthy mismatch,
  healthy missing file, and both durable anchor forms where practical;
- documentation and verification gates for the implementation workflow.

Keep it implementation-ready and avoid adding product scope beyond GitHub issue
#217.
