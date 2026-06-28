---
schema_version: "striatum.handoff.v1"
artifact_kind: "handoff"
---

# RFC 0168 P0 Build v3 Draft
author: author-author-003

## Files Changed

- Runtime schema and authority: `go/pkg/db/sql/0047_lane_uid_leases.sql`, `go/pkg/db/sql/owner/0023_lane_uid_leases.sql`, migration/owner registries, and read/write authority inventories.
- Lane UID runtime: `go/pkg/mutations/lane_uid_leases.go`, supervision start/stop/control/report paths, recovery sweep, doctor readout, and RPC error catalog. Attempt 2 tightened P5 to inspect worktree/workspace rows through the owning session's job leases, matching the real row model where job lease IDs and lane UID lease IDs are distinct. Attempt 3 tightens P1 proof recording so `scrub_proof` carries every observed pool-uid PID with `/proc` state and `reaped`/`live`/`unknown` classification, while live or unknown observations still quarantine.
- Sandbox and credential boundaries: private supervisor scratch bearer placement, repo ACL planning/tests, per-job worktree/workspace run-as grants, provider credential resolver/guard/tests.
- State docs: `CHANGELOG.md`, `README.md`, `docs/how-to/lane-sandbox.md`, `docs/reference/command-authority-matrix.md`, `docs/rfcs/0168-per-lane-security-principal.md`, `docs/operator/rfc-roadmap.md`, and `docs/operator/BRIEF.md`.

## Blocker Closure

- F1: UID return is now proof-gated. Scrub cleanup is best-effort but any S1 kill failure, home/provider-store residue, supervisor scratch residue, live or unknown uid process, or active/abandoned worktree/workspace row owned by the lane UID lease's session keeps the lease quarantined. P1 now records observed PID, state byte, classification, and unknown-state errors in `scrub_proof` rather than collapsing proof to a bare PID count. `retry_quarantined_lane_uids=true` reruns the same proof and returns only on clean P1-P5 proof.
- F2: `supervise.report` validates the active lane UID lease generation before heartbeat, state, terminal metadata, or helper-event writes. Stale generations fail closed with `lane_uid_generation_mismatch`.
- F3: provider credential/cache selectors now resolve relative paths against the lane launch root/repo root before the in-repo refusal. Tests cover `CLAUDE_SECURESTORAGE_CONFIG_DIR=docs/.lane-auth/secure` and uncovered `ANTHROPIC_CONFIG_DIR=docs/.lane-auth/anthropic`, while ordinary relative non-credential env remains allowed.

## Verification

- `go test ./pkg/mutations -run 'TestLaneUIDWorkspaceCleanupProofBlocksActiveOrAbandonedWork|TestRecoverLaneUIDLeasesRetriesQuarantinedOnlyAfterCleanP5Proof'`
- `go test ./pkg/mutations -run 'TestLaneUIDProcessProofRecordsStateEvidenceAndBlocksUnknown|TestClassifyProcStateBlocksStoppedTracedLiveAndUnknown|TestScrubLaneUIDArtifactsFailsClosedOnKillFailure|TestLaneUIDHomeCleanupProofFailsWhenCredentialStoreRemains|TestLaneUIDWorkspaceCleanupProofBlocksActiveOrAbandonedWork|TestRecoverLaneUIDLeasesRetriesQuarantinedOnlyAfterCleanP5Proof|TestSuperviseReportRejectsStaleLaneUIDGenerationBeforeHeartbeat|TestLaneCredentialDomain'`
- `go test ./pkg/mutations ./pkg/laneproviderauth ./pkg/admin ./pkg/agentloop ./pkg/db ./pkg/reads ./pkg/rpc`
- `go build ./... && go vet ./...`
- `go test ./...` (`GOCACHE=/tmp/striatum-go-cache`; no temporary `GOPATH` override)
- `make check-docs`
- `make lint`
- `make typecheck`
- `make smoke` (PostgreSQL integration skipped because `STRIATUM_DAEMON_DB_URL` is unset; fresh-clone smoke passed)

## Residual Operator Work

Review and verify this build through the workflow. After acceptance, ship schema 47 and owner bundle 0023 with the matching binary before treating RFC 0143 Slice B as unblocked.
