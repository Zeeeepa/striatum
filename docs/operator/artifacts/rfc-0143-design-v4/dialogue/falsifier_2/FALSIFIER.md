# FALSIFIER - RFC 0143 design-v4 BC5 lifecycle re-attack

author: falsifier-reviewer-004

## Verdict

**BC5's two v3 precision items are genuinely resolved, but v4 still has a material lifecycle/correctness gap in the daemon-observed reseal predicate.** The new gap is not the old migration-site or lock-order bug. It is that the v4 positive trigger depends on (a) a recorded packet boot epoch and (b) a modified-since-packet artifact proof, but the spec does not pin either to a concrete, durable per-job evidence record that current source can provide.

Without that evidence, the implementation has only bad choices: never fire the positive reseal path, fire it without proving a boot-epoch rotation happened during this lease, or treat a stale/pre-existing artifact in the worktree as "produced" by this attempt. That is a standing falsification for Slice B and for the lifecycle tests that are supposed to prove no silent unsealed exit and no false seal across rotation.

## BC5 First-Pass Verification

### Migration site: resolved

**Claim attacked.** v3 left `leases.reseal_grace_extended_at` as a downstream "owner bundle if owner-held, else runtime migration" decision. v4 claims it is pinned to the same owner bundle 0021 as `jobs.recovery_generation`.

**Verification.** Current source supports the ownership premise. `striatumd.leases` is created in runtime migration 0005 (`go/pkg/db/sql/0005_repo_local_workflow_state.sql:166-182`). Owner bundle 0018 transfers a fixed runtime-table cohort to `striatumd_rw`, and that cohort does not include `leases` (`go/pkg/db/sql/owner/0018_runtime_table_ownership_transfer.sql:80-90`). The v4 holder now says the `ALTER TABLE striatumd.leases ADD COLUMN IF NOT EXISTS reseal_grace_extended_at timestamptz` statement lands in `go/pkg/db/sql/owner/0021_job_recovery_generation.sql` (`HOLDER.md:473-495`).

**Result.** This resolves the v3 migration-site item. The spec names a concrete owner-bundle location and a guardrail test that can fail if the build puts the owner DDL in a runtime migration.

### Lock-order accuracy and typed routing: resolved at design level

**Claim attacked.** v3 incorrectly implied `work.complete` already takes `lockRunForJob` first. Current source does not: `HandleCompleteWork` runs `enforceSessionBindingForSession` and `enforceActiveActingSession` before `lockRunForJob`, then calls `activeLeaseFor` and `ensureWorkSessionBackend` after the lock (`go/pkg/mutations/lifecycle.go:1124-1182`).

**Verification.** v4 corrects the story. It says `resealInFlightJob` does not call `HandleCompleteWork`; it skips/replaces the two pre-lock public caller gates, replays `lockRunForJob` first, locks `jobs -> leases -> job_recovery_state`, replaces `activeLeaseFor` with the reseal predicate, bypasses `ensureWorkSessionBackend`, and maps misses to `session_unrecoverable_across_rotation` (`HOLDER.md:512-567`, `:571-605`). That also lines up with `artifact.publish` taking `lockRunForJob` first inside its tx (`go/pkg/mutations/artifact.go:75-85`) and the recovery sweep taking `lockRun` before `expireLeases`/requeue (`go/pkg/mutations/recovery.go:575-621`, `:866-890`).

**Result.** The old BC5 lock-order item is fixed. `TestResealBeyondGraceRoutesTypedNotLeaseError`, `TestResealGraceCannotReviveRequeuedLease`, `TestRecoveryRequeueWinsOverExpiredLeaseReseal`, `GD-1b`, and `TestResealExit98BypassesBackendGateOrRoutesTyped` are meaningful tests for the corrected gate map.

## Material Challenge: The Daemon-Observed Trigger Has No Pinned Evidence Record

### C1 - "Boot-epoch rotation occurred during this lease" is not buildable as specified

**Precise claim attacked.** v4 says `resealInFlightJob` fires only when "the daemon observed its own boot-epoch increment since the packet was issued (the recorded packet epoch vs the current `writeBootEpochFile` epoch)" (`HOLDER.md:289-292`).

**Concrete refutation.** Current source has a live process boot epoch, but no durable packet epoch in the work packet shape:

- `buildPacket` persists `packet_json`, but the packet's `lease` block contains only `lease_id`, `message_id`, `expires_at`, and `heartbeat_after_seconds`; there is no boot-epoch field (`go/pkg/mutations/claim.go:543-600`, especially `:564-574`).
- The launch epoch is injected into the lane env as `STRIATUM_MCP_BOOT_EPOCH` (`go/pkg/mutations/supervision_env.go:344-353`) and echoed as an HTTP header, not persisted as packet evidence.
- `daemonBootEpoch()` is explicitly per-process and never persisted across restart (`go/cmd/striatumd/main.go:707-719`); `writeBootEpochFile` publishes the current process value (`:739-752`). After a restart, the new daemon cannot recover the old packet's epoch from that file.

So the key predicate "rotation occurred during this job's lease" lacks a named storage site. The build needs a concrete `work_packets.packet_json` field such as `lease.boot_epoch` or an equivalent DB column, stamped at claim before the packet is delivered. Without it, the positive reseal path either cannot prove the rotation and never fires, or it fires from "current epoch exists" and loses the v4 boundary that absent rotation uses the normal seal path.

**Strongest rebuttal for the Holder.** `HOLDER.md` does use the words "recorded packet epoch"; one could infer that the build will add a field to `packet_json`.

**Why a real gap remains.** The v4 spec is otherwise precise about schema sites (`jobs.recovery_generation`, `leases.reseal_grace_extended_at`) and tests. Here it does not name the field, the stamping site in `buildPacket`, or a test that asserts the old epoch is persisted and compared after restart. For a lifecycle predicate that gates seal vs typed floor, inference is not enough.

### C2 - `write_scope_baseline.changed_paths` is the wrong proof for "artifact modified since packet"

**Precise claim attacked.** v4 says every required expected artifact must be present in the active worktree and "modified since the packet was issued"; concretely, the daemon re-hashes each required path in the job worktree and checks it differs from `write_scope_baseline.changed_paths` (`HOLDER.md:296-304`).

**Concrete refutation.** Current `write_scope_baseline` is not a complete per-job artifact baseline:

- `buildWriteScopeBaseline` calls `gitChangedPathSnapshots(ctx, run["repo_root"])` at packet build time and records only the paths already dirty in the shared checkout (`go/pkg/mutations/claim.go:601-630`; `go/pkg/mutations/write_scope_guard.go:225-245`).
- The live packet for this very job demonstrates the shape: its baseline contains an unrelated dirty primary-checkout path, not a hash for this job's expected `FALSIFIER.md`.
- The active per-job worktree record stores `worktree_path`, `base_branch`, and lease/job identity, but no per-path baseline hash or base HEAD artifact snapshot (`go/pkg/db/sql/0005_repo_local_workflow_state.sql:350-372`).
- Existing completion verification checks artifact rows in PostgreSQL (`verifyRequiredArtifacts`, `go/pkg/mutations/mutations.go:828-876`); it does not answer whether an on-disk expected path was authored by this attempt.

That leaves an implementation ambiguity at the exact point v4 relies on for positive intent:

- If "not present in `changed_paths`" means "not modified", then newly authored expected artifacts are never positive evidence, so the Slice-B positive case never fires for the common complete-on-disk-but-unpublished failure.
- If "not present in `changed_paths`" means "new/modified", a stale expected artifact already present cleanly in the job worktree can be treated as produced by this attempt and auto-published/resealed.

This matters especially for reopened attempts and run-branch artifacts: a path can exist in the active worktree before the lane writes anything. Presence plus absence from the dirty-path baseline is not proof of production.

**Strongest rebuttal for the Holder.** The spec says "content-hash-vs-baseline," not "presence only," and existing source has helper patterns for authored-path attribution. The build can add the right baseline.

**Why a real gap remains.** The spec explicitly cites the wrong existing evidence object. `write_scope_baseline.changed_paths` is for write-scope attribution against pre-existing dirty work, not a full per-job expected-artifact baseline. A falsifiable spec must pin the replacement evidence: at claim/worktree creation, persist `{path, existed, sha256}` for every required expected artifact in the active job worktree, then require the post-rotation content to differ from that per-path baseline. Add a negative test where an expected artifact exists cleanly before claim and is not modified; post-rotation exit 98 / recovery sweep must route the typed floor, not seal. Add a positive sibling where the same path is modified and the daemon reseals.

## Carried-Forward Lifecycle Set

**BC4 is not regressed.** v4 keeps `jobs.recovery_generation` in owner bundle 0021, names the owner-frontier bump and reservation, keeps a degrade-safe presence probe, names the increment points, stamps the generation into the packet, and compares packet-vs-live generation under the lock (`HOLDER.md:435-467`).

**F7 file-mirror half is not regressed by BC5.** v4 keeps the daemon-owned endpoint/epoch mirror, `O_NOFOLLOW`, atomic rename, and supervised missing-epoch rejection as carried-forward assertions (`HOLDER.md:122`, `:591`, `:698-701`). The implementation must still reconcile this with current backward-compatible HTTP behavior that allows absent epoch headers (`go/pkg/mcp/http.go:159-166`, `:669-689`) on the supervised path.

**No admin-token widening found.** The v4 lifecycle path keeps `CapabilityReseal` daemon-internal and never materializes a lane-readable bearer carrying `{admin, apply, recovery, surgical_recovery}` (`HOLDER.md:607-633`). The challenge above is correctness/false-seal/liveness, not admin-token exposure.

## Required Fix Before Clearance

Keep the BC5 migration and gate-map repairs. Add the missing evidence contract:

1. Stamp the daemon boot epoch into durable per-packet state at claim, with a named field and a test that proves a post-restart daemon compares the old packet epoch to the live epoch.
2. Persist a per-job expected-artifact baseline for every required path in the active worktree, not just `write_scope_baseline.changed_paths` from the shared checkout.
3. Extend `TestCodexResealUsesReceiverNotProviderStdout` or add `TestResealRequiresPacketBootEpochAndAuthoredExpectedArtifactChange` to cover both negative cases above and the positive modified-artifact case.

Until then, v4 should remain **needs_revision**: the old BC5 precision defects are closed, but the new daemon-observed reseal trigger is not yet a buildable, falsifiable lifecycle contract.