# RFC 0165 v3 Falsifying Challenge: Runtime Freshness Is Not Bound to the Stalled Session
author: falsifier-reviewer-003

## Challenge

The v3 holder adds the right class of F1 hook: recovery is supposed to check provider-auth freshness before it increments `requeue_count` or `transfer_count`. The unresolved gap is that the proposed "current" check is keyed to the latest dependency row for a lane user/destination, not to the specific projection generation that the stalled session actually launched with or is using.

That distinction matters for the launch-fresh-then-expire-mid-session case. A Claude lane can launch with generation G1 and 35 minutes of freshness, do 45 minutes of local work, then fail its first Claude/MCP action because the process still has G1. If any later Claude lane under the same lane OS user projected generation G2 before recovery runs, the singleton `provider_auth_dependencies` row can now say "fresh" for G2. Recovery then has fresh evidence for the lane user, but not for the stuck process. F1 asked for runtime expiry to be classified before generic budget burn; classifying job A against job B's newer projection does not meet that bar.

## Claim Challenged

The holder claims F1 is resolved because:

- `recoverStuckJobs` checks freshness for Claude `agent_mcp_discovery_stall` after reading the recovery budget and before `recordRecoveryAction`;
- the check computes current seconds-to-expiry from daemon-owned `provider_auth_dependencies`;
- expired, near-expiry, or unverifiable state becomes `reseed_required` / `unverifiable` without incrementing generic counters; and
- the daemon-owned sweep marks running-lane near-expiry debt before generic MCP-discovery recovery handles the same lane.

The challenged part is not the placement before `recordRecoveryAction`. It is the identity of the credential being checked. A latest dependency row per lane user can be fresh while the stalled session's launch generation is expired.

## Evidence

The v1 ledger's binding C2 constraint required recovery-time freshness classification for Claude stalls "using current credential freshness or broker state rather than only the spawn-time receipt," and required that a runtime-expired or near-expiry credential produce provider-auth debt without incrementing generic recovery counters. C3 required a daemon-owned signal for credential decay while a lane is running, not lane-authored claims.

The v3 holder maps that to a recovery hook and decay signal, but its state model loses the per-session binding needed to make the hook authoritative:

- `docs/operator/artifacts/rfc-0165-design-v3/dialogue/holder/HOLDER.md:83-98` says the F1 fix is a current freshness check before generic counters, followed by `reseed_required` / `unverifiable` handling.
- `docs/operator/artifacts/rfc-0165-design-v3/dialogue/holder/HOLDER.md:360-378` says recovery looks up the lane's `provider_auth_dependencies` row and uses its current expiry before `recordRecoveryAction`.
- `docs/operator/artifacts/rfc-0165-design-v3/dialogue/holder/HOLDER.md:382-390` says the same singleton row drives running-lane near-expiry debt.
- `docs/operator/artifacts/rfc-0165-design-v3/dialogue/holder/HOLDER.md:398-407` keys the current-state row by `repository_id`, `provider`, `kind`, `lane_user`, and `destination_selector`, with one `source_generation_id`, one `destination_generation_id`, one `expires_at`, and one `last_receipt_id`.
- `docs/operator/artifacts/rfc-0165-design-v3/dialogue/holder/HOLDER.md:410-423` puts `run_id`, `session_id`, and `lane_id` only on append-only receipts, but the recovery hook does not say it selects the receipt/generation bound to the stalled job's owner session.

Current recovery already has the information needed to avoid this. `recoverStuckJobs` scans unfinished jobs with `job_id`, `workflow_job_id`, `owner_session_id`, `session_id`, and supervisor pointer metadata (`go/pkg/mutations/recovery_decision_tree.go:713-735`). It later reads the per-job budget at `readJobRecoveryBudget` and increments counters at `recordRecoveryAction` (`go/pkg/mutations/recovery_decision_tree.go:1143-1406`). The proposed design should bind provider-auth evidence to that job/session/supervisor context, but the holder only specifies a latest lane-user row.

## Concrete Counterexample

1. Lane A launches for job A at T0. The projector writes B1 access-token-only generation G1, records a receipt for session A, and stores `provider_auth_dependencies.expires_at = T0+35m`.
2. Lane A does local work for 45 minutes before its first Claude/MCP action. Its process is still operating on G1, either because the Claude CLI read the B1 file at startup or because the session's effective provider credential is otherwise the launch projection. G1 expires at T0+35m.
3. At T0+20m, lane B launches under the same `lane_user` and `destination_selector`. The projector writes generation G2 and updates the singleton dependency row to `expires_at = T0+55m`, `destination_generation_id = G2`, `last_receipt_id = receipt_B`.
4. At T0+45m, lane A's first Claude/MCP action fails from the expired G1 token and the session becomes `agent_mcp_discovery_stall`.
5. Recovery handles job A. The v3 hook looks up the lane's current `provider_auth_dependencies` row and sees G2 as fresh or outside the near-expiry lead. Because the spec does not require recovery to select session A's launch receipt/generation, the hook has no reason to mark A's expired G1 as provider-auth debt.
6. Recovery falls through to the ordinary requeue/transfer branch and `recordRecoveryAction` increments generic budget for a runtime provider-auth cause.

The same keying problem makes the near-expiry signal flap. Lane A crosses the near-expiry lead shortly after launch, but lane B's later projection can refresh the shared row and erase the evidence before A is classified. That is not a stable daemon-owned decay signal for running lanes; it is latest-row state for a shared lane credential slot.

## Strongest Rebuttal

The best defense is that B1 may not cache: if the running Claude process always re-reads the shared credential file at the first provider action, then lane A could pick up G2 and never fail from G1. B2 is stronger still, because an on-demand broker can return the freshest access token and the broker knows each fetch's expiry.

That rebuttal is not enough as written. B1 is the primary delivery mechanism, and the only named B1-vs-B2 decider test checks whether the Claude CLI accepts an access-token-only file while unexpired; it does not prove that an already-launched Claude process re-reads the file after another lane projection, nor that every provider action is atomically tied to the latest daemon row. F1 is about the credential generation that caused the stuck session, not whichever generation happens to be newest for the lane OS user when recovery runs.

## Required Revision

The SPEC needs an explicit per-running-session provider-auth binding before F1 can clear:

- Store the projection receipt id, destination generation id, `expires_at`, and delivery mode in launch-time session/supervisor/job metadata, or make `provider_auth_dependencies` include a session/supervisor generation dimension for active lanes.
- In `recoverStuckJobs`, classify a Claude stall against the provider-auth generation bound to the job's owner session/supervisor. A newer lane-user row may prove a fresh re-projection is available for the next launch; it must not prove that the stalled process's expired generation was fresh.
- Near-expiry debt must be per running session/generation. A later projection for another lane must not clear or overwrite debt for an older running session unless the design proves that session has adopted the newer token.
- If B1 cannot prove launched processes re-read and use the newest projection at every provider action, then B1 cannot be the primary runtime-expiry story. The design should either select B2 for running-lane freshness or restart/requeue sessions when their bound generation crosses the near-expiry lead.
- Add `TestRecoveryClassifiesExpiredLaunchGenerationDespiteNewerProjection`: launch A with G1 expiring in 35 minutes, launch B under the same lane user with fresh G2 before A's first provider action, advance A to 45 minutes so G1 is expired, trigger `agent_mcp_discovery_stall`, and assert recovery sets provider-auth debt for A without incrementing `requeue_count` / `transfer_count` or escalating `recovery_exhausted`.
- Add a decay-signal test where an older running session crosses near-expiry, a later projection updates the lane-user row, and the older session's near-expiry debt remains visible until that session is restarted or proven to use the newer generation.

## Bottom Line

The v3 holder resolves the simple single-lane F1 scenario, but not the overlapping projection case that current Striatum lanes can hit under a shared lane OS user. A latest row keyed by lane user/destination is not enough to classify the credential that made a specific session stall. F1 should remain open until runtime freshness is bound to the stalled job's actual projection generation, or until the design proves that every running Claude process always uses the latest daemon projection atomically.
