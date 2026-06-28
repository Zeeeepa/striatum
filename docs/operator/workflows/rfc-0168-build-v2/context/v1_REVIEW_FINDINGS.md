# RFC 0168 Build v2 Review Findings Context

This workflow supersedes canceled run `run_efbe6e1396adf8470e4862d6ba6194bb`.
The first RFC 0168 build run was canceled instead of hand-finished because the
daemon-owned review found real P0 blockers and the run branch became stale
behind current `main`.

Relevant first-run artifacts:

- Latest review artifact: `art_3c97d530c0e2b269afc8516cf36c983f`.
- Revised draft artifact reviewed by that finding:
  `art_e9a1645576936b9b1c444ff76cfabd9a`.

The reviewer credited two partial successes from the canceled run:

- The D272 discriminator was materially present: provider-owned credential
  selectors were separated from ordinary lane environment selectors, preserving
  the typed `lane_uncovered_credential_selector_inside_repo` refusal for
  uncovered provider credential directories resolving inside the repository and
  proving an in-repo non-credential env control such as `AGY_HOME` or
  `FIXTURE_CONFIG_DIR` still launches.
- The ephemeral MCP bearer was moved under supervisor-scoped scratch rather
  than using a broad scratch-root fallback.

The same review returned `needs_revision` with three high-severity blockers:

1. F1: the return/scrub/recovery implementation only proved a partial P1
   `/proc` scan. It did not prove S1 kill, S2 credential-store deletion, S3
   home/private scratch/ACL/worktree cleanup, or P2-P5. `/proc` read failures
   were treated as clean instead of fail-closed. Recovery only quarantined rows
   stuck in `scrubbing` after ten minutes; it lacked a dead/lost
   `active -> scrubbing` reaper and lacked proof-gated
   `quarantined -> returned` retry.
2. F2: `STRIATUM_LANE_UID_GENERATION` was allocated, injected, and stored in
   metadata, but generation was not enforced on attestation and control-frame
   paths. `sessionLaneAttestation` still delegated to legacy lanehealth and the
   local helper effectively reduced to "pid start token present" with no live
   generation comparison against `lane_uid_leases`.
3. F3: per-job worktrees were not granted to the leased uid. Scratch ACLs were
   handled, but `worktree.create` still created worktrees as the daemon user and
   did not `chown` or `setfacl` the active pool uid. The repository ACL planner
   was still keyed off legacy `STRIATUM_LANE_OS_USER`.

Current-main constraints as of this scaffold:

- `main` is at `e31342ba` and includes the RFC 0171 generated-records build.
  Runtime schema `0046_generated_records.sql` is taken. RFC 0168 must use the
  next free runtime migration slot after rechecking, expected to be `0047`.
- Do not delete or overwrite RFC 0171 generated-records source, CLI routes,
  docs, docket domain, or schema.
- `go/pkg/db/sql/owner/0022_operator_identity_run_attribution.sql` is the
  latest normal owner bundle on this base. Owner bundle `0023` appears free but
  must be rechecked before use.

Acceptance posture for v2:

- A review must remain `needs_revision` if any F1-F3 blocker is still only
  metadata, documentation, or a partial test proof.
- A review must remain `needs_revision` if migration numbering collides with
  RFC 0171, if RFC 0171 files disappear, or if the D272 discriminator over-
  refuses ordinary non-credential lane env.
