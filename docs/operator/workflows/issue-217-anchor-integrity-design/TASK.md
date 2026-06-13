# Issue 217 Artifact-Anchor Integrity Design Task

Design the bounded implementation for GitHub issue #217:
<https://github.com/halbritt/striatum/issues/217>

The implementation gate is now satisfied on the local daemon: repository-scoped
doctor reports blob storage configured, reachable, repo-provisioned, and bucket
healthy for the Striatum repository. The next step is code, not more triage.

## Product Goal

When repository-scoped doctor runs with blob storage configured, reachable, and
repo-provisioned, it should cross-check completed repo-write jobs that have a
durable git anchor against their published repository artifacts.

For each relevant artifact with a repository path and recorded
`content_sha256`, doctor should verify that the file content at that path in the
anchored commit matches the daemon-recorded artifact body hash.

## Required Boundaries

- Keep the check read-only.
- Reuse existing artifact body lookup and SHA-256 verification behavior.
- Reuse the existing durable-anchor classification for completed repo-write
  jobs (`run_branch` and `job_pin`).
- Do not print artifact bodies, blob credentials, or private diagnostics.
- Do not repair mismatches, move refs, modify checkouts, or make blob storage
  mandatory.
- Preserve existing blob doctor diagnostics when blob is disabled, unreachable,
  or missing a repo bucket.

## Design Output

Publish an implementation-ready design contract at
`docs/operator/artifacts/issue-217-anchor-integrity-design/DESIGN.md` with:

- source surfaces to inspect;
- behavior and skip semantics;
- problem-record fields;
- focused implementation slices;
- test plan;
- docs and verification gates.
