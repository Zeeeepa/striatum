---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# Issue 217 Final Summary
author: author-local-fixture-002

## Final Behavior

Issue #217 is implemented as a repository-scoped doctor check named
`artifact_anchor_integrity`.

The check runs only when the existing blob doctor block reports blob storage is
configured, reachable, repo-provisioned, and `bucket_status: "ok"`. It skips
cleanly for disabled blob, unreachable blob, missing repository bucket, or any
non-`ok` bucket status.

When enabled, doctor checks completed repo-write artifacts with a repository
path and recorded `content_sha256` against durable git anchors:

- run branch refs report `anchor_kind: "run_branch"`;
- `refs/striatum` pins report `anchor_kind: "job_pin"`.

Mismatches emit `artifact_anchor_hash_mismatch.<artifact_id>`. Missing files
emit `artifact_anchor_missing_file.<artifact_id>`. Verbose problem records
include identifiers and anchor metadata, not artifact bodies or blob
credentials.

## Review Result

The review job accepted the implementation with no blocking findings.

## Verification

Passed:

- `go test ./pkg/reads`
- `go test ./...`
- `git diff --check`
- workflow validation for both #217 workflows
- `make test`
- `make lint`
- `make smoke`

`make smoke` skipped PostgreSQL integration because the configured database was
not reachable, then completed the fresh-clone smoke successfully.

## Issue Closure Note

Use this closure note after the commit lands:

> Implemented in `<commit>`: doctor now performs the blob-gated artifact-anchor
> integrity cross-check for completed repo-write artifacts, with skip behavior
> for unhealthy blob state, stable mismatch/missing-file problems, verbose
> problem records, tests, and blob diagnostics documentation.
