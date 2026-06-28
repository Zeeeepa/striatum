Review the draft implementation against the required RFC 0168/D272 context and
the v2 review-findings context.

Focus on gate-critical behavior, missing tests, authority/scope regressions,
schema/owner-bundle drift, and the D272 discriminator. Return `needs_revision`
for any security-boundary gap, missing typed refusal, over-broad refusal of
legitimate non-credential lane env, docs/source mismatch, or runtime migration
collision with RFC 0171's `0046_generated_records.sql`.

This v2 review must explicitly check the first run's blockers:

- F1: S1-S3/P1-P5 return, scrub, proof, leaked-active reaper, stuck-scrub
  redrive, proof-gated quarantine retry, and fail-closed `/proc` behavior.
- F2: generation comparison is enforced against live `lane_uid_leases` on
  attestation and control-frame/report paths.
- F3: per-job worktree access is granted to the leased uid and no legacy
  `STRIATUM_LANE_OS_USER` shortcut remains load-bearing.

Also confirm RFC 0171 generated-records/docket files are preserved.

Publish exactly one REVIEW.md finding artifact with a clear verdict and concrete
evidence.
