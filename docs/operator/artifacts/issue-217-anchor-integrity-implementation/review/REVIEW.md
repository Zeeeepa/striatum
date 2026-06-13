---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept"
severity: "info"
tags: ["doctor", "blob", "worktree-anchor", "issue-217"]
---

# Issue 217 Implementation Review
author: reviewer-local-fixture-001

## Verdict

accept

The implementation is within the #217 scope: it adds a read-only doctor
artifact-anchor cross-check, gates the check on healthy repo-provisioned blob
state, and reports stable problems plus verbose records for mismatch and
missing-file cases.

## Review Notes

- Blob skip semantics are explicit and tested: disabled blob and non-`ok`
  bucket state do not query artifact rows or add problems.
- Git inspection is read-only (`git show` against resolved commits) and does
  not move refs or touch the operator checkout.
- Problem records include repository, run, job, artifact, repository path,
  recorded content hash, anchor kind/ref/commit, and checked refs. They do not
  include artifact bodies or credential material.
- Tests use real git repositories for anchor behavior and a fake DB runner only
  at the storage boundary.
- Documentation in `docs/explanation/blob-transition.md` describes the new
  check and skip conditions.

## Verification Reviewed

- `go test ./pkg/reads`
- `go test ./...`
- `git diff --check`
- workflow validation for both #217 workflows
- `make test`
- `make lint`
- `make smoke`

No blocking findings.
