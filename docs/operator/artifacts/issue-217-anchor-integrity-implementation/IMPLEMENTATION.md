# Issue 217 Artifact-Anchor Integrity Implementation
author: author-local-fixture-002

## Shipped Behavior

Doctor now includes an `artifact_anchor_integrity` read block. The check is
disabled unless repository-scoped blob diagnostics report configured,
reachable, repo-provisioned storage with `bucket_status: "ok"`.

When enabled, doctor reads completed repo-write artifacts with a repository
path and recorded `content_sha256`, resolves each job's durable git anchors
from the run branch and `refs/striatum` pins, and compares the file content at
the artifact path in the anchored commit to the recorded hash.

Stable problems:

- `artifact_anchor_hash_mismatch.<artifact_id>`
- `artifact_anchor_missing_file.<artifact_id>`

Verbose problem records include repository, run, job, artifact, logical name,
repository path, recorded `content_sha256`, anchor kind, anchor ref, anchor
commit, checked refs, and either `anchor_content_sha256` or a missing-file
reason. Artifact bodies and blob credential material are not returned.

## Files Changed

- `go/pkg/reads/doctor.go`
- `go/pkg/reads/doctor_artifact_anchor.go`
- `go/pkg/reads/doctor_artifact_anchor_test.go`
- `docs/explanation/blob-transition.md`

## Verification

Passed:

- `go test ./pkg/reads`
- `go test ./...`
- `git diff --check`
- `striatum workflow validate docs/operator/workflows/issue-217-anchor-integrity-design/workflow.json --json`
- `striatum workflow validate docs/operator/workflows/issue-217-anchor-integrity-implementation/workflow.json --json`
- `make test`
- `make lint`
- `make smoke`

`make smoke` reported `PostgreSQL integration skipped; configured database is
not reachable`, then completed the fresh-clone smoke successfully.

## Deferred

No repair behavior, ref mutation, checkout mutation, blob mandate, or bulk
artifact migration was added.
