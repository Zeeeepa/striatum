Implement GitHub issue #217 according to the design artifact and task brief.

Use test-driven vertical slices:

1. Add the smallest behavior test that proves the new doctor cross-check skips
   cleanly when blob storage is not healthy or not repo-provisioned.
2. Add the healthy matched-anchor behavior.
3. Add mismatch and missing-file diagnostics.
4. Cover both durable anchor forms (`run_branch` and `job_pin`) where the
   existing fixture setup makes that practical.

The implementation must:

- reuse existing artifact body lookup and SHA-256 verification behavior;
- reuse existing completed repo-write durable-anchor classification;
- inspect git objects read-only at the anchored commit/ref;
- emit stable doctor problem strings and verbose `problem_records`;
- avoid printing artifact bodies or blob credentials;
- preserve all existing blob doctor diagnostics.

Publish `docs/operator/artifacts/issue-217-anchor-integrity-implementation/IMPLEMENTATION.md`
with the files changed and verification run.
