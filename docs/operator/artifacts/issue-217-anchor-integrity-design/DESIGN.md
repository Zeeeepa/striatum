# Issue 217 Artifact-Anchor Integrity Design
author: author-local-fixture-002

## Decision

Implement #217 as a repository-scoped doctor integrity check that runs only
after the existing blob diagnostics say the repository has a configured,
reachable, provisioned blob bucket with bucket status `ok`.

The check should compare each relevant completed repo-write artifact's recorded
artifact body hash against the file content present at the artifact repository
path in the completed job's durable git anchor. The durable anchor source stays
the existing worktree/ref-safety projection: `run_branch` or `job_pin`.

## Skip Semantics

The new check must skip without adding problems when:

- blob storage is not configured;
- blob storage is unreachable;
- the repository has no provisioned bucket;
- bucket status is not `ok`;
- no completed repo-write jobs have durable anchors;
- a completed anchored job has no repository-path artifact with
  `content_sha256`.

Existing blob doctor warnings and problems remain unchanged.

## Problem Records

When the check runs, doctor should report stable problems for:

- `artifact_anchor_hash_mismatch`: anchored file content exists but its SHA-256
  differs from the recorded artifact content hash.
- `artifact_anchor_missing_file`: the artifact repository path is absent in the
  anchored commit.
- `artifact_anchor_unreadable`: the anchored object cannot be inspected.

Verbose `problem_records` should include repository id, run id, job id,
artifact id or logical name, artifact repository path, recorded
`content_sha256`, anchor kind, and anchor ref or commit. It must not include
artifact body content or blob credential material.

## Implementation Surfaces

Start from the existing doctor and blob/read projections rather than adding a
parallel artifact contract:

- doctor assembly and verbose problem record construction under `go/pkg/reads`
  and the doctor CLI route;
- artifact body lookup and SHA-256 verification code already used by artifact
  publication/reads;
- completed repo-write durable-anchor classification used by worktree
  ref-safety diagnostics;
- read-only git object inspection helpers already used by worktree/anchor
  checks.

If any of those surfaces are narrower than expected, add the smallest private
helper needed inside the existing package boundary.

## Implementation Slices

1. Add a focused skip test that proves disabled or unhealthy blob state does not
   produce artifact-anchor findings.
2. Add a healthy anchored-match test and wire the read-only git comparison.
3. Add mismatch and missing-file diagnostics with stable problem strings and
   verbose problem records.
4. Extend fixture coverage to both `run_branch` and `job_pin` anchors where the
   existing setup makes that practical.
5. Document the check and rerun the standard Striatum gates.

## Verification

Run at least:

- `git diff --check`
- focused Go tests for the touched doctor/blob/worktree packages
- `make test`
- `make lint`
- `make smoke`
- workflow validation for the design and implementation scaffolds

If a gate cannot run because of host state, record the exact blocker in the
implementation summary.
