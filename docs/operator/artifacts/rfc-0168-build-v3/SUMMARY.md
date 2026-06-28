---
schema_version: "striatum.synthesis.v1"
artifact_kind: "synthesis"
---

# RFC 0168 P0 Build v3 Summary
author: author-author-004

## Outcome

RFC 0168 P0 build v3 is finalized on reviewed head `764d53f8`. The final review artifact
`art_17f472d7eeae11bce1aaaec265d49dff` by `reviewer-reviewer-003` accepted the branch
against RFC 0168, D272, the v6 accepted design context, and the final v2 blocker list. This
apply pass did not need additional source changes after that accepted review; it reran the
required verification and added this summary artifact.

## Files Changed By Gate

- Build artifacts: `docs/operator/artifacts/rfc-0168-build-v3/DRAFT.md` and this
  `docs/operator/artifacts/rfc-0168-build-v3/SUMMARY.md`.
- State and operator docs: `CHANGELOG.md`, `README.md`, `docs/how-to/lane-sandbox.md`,
  `docs/operator/BRIEF.md`, `docs/operator/rfc-roadmap.md`,
  `docs/reference/command-authority-matrix.md`, and
  `docs/rfcs/0168-per-lane-security-principal.md`.
- Runtime and owner schema: `go/pkg/db/sql/0047_lane_uid_leases.sql`,
  `go/pkg/db/sql/owner/0023_lane_uid_leases.sql`, `go/pkg/db/migrations.go`,
  `go/pkg/db/owner.go`, `go/pkg/db/sql/RESERVATIONS.toml`,
  `go/pkg/db/read_authority_inventory.go`, and `go/pkg/db/write_authority_inventory.go`.
- Lane UID leasing, cleanup proof, recovery, and reporting:
  `go/pkg/mutations/lane_uid_leases.go`, `go/pkg/mutations/lane_uid_leases_test.go`,
  `go/pkg/mutations/supervision.go`, `go/pkg/mutations/supervision_test.go`,
  `go/pkg/mutations/supervision_control.go`, `go/pkg/mutations/supervision_env.go`,
  `go/pkg/mutations/supervision_lane_config.go`, `go/pkg/mutations/supervision_launch.go`,
  `go/pkg/mutations/lifecycle.go`, `go/pkg/mutations/mutations.go`,
  `go/pkg/mutations/recovery_auto.go`, `go/pkg/reads/doctor.go`,
  `go/pkg/reads/doctor_lane_uid_leases.go`, and `go/pkg/rpc/error_catalog.go`.
- Credential and ACL boundaries: `go/pkg/mutations/credential_domain_guard.go`,
  `go/pkg/mutations/credential_domain_guard_test.go`, `go/pkg/laneproviderauth/resolver.go`,
  `go/pkg/laneproviderauth/rfc0162_test.go`, `go/pkg/admin/repo_acl.go`,
  `go/pkg/admin/repo_acl_test.go`, `go/pkg/mutations/scratch_acl.go`,
  `go/pkg/mutations/scratch_acl_test.go`, `go/pkg/agentloop/mcpconfig.go`,
  and `go/pkg/agentloop/mcpconfig_test.go`.
- Worktree and owner-revoke support: `go/pkg/mutations/worktree.go` and
  `go/pkg/db/owner_revoke_filter_test.go`.

## Review Findings Addressed

- F1, UID return proof gate: uid return is blocked until S1-S3 cleanup and P1-P5 proof are
  clean. S1 kill failures fail closed, provider/home/reseal/tmux/scratch/worktree cleanup is
  proved before return, P1 records per-PID `/proc` state/classification evidence, and
  quarantined retry reruns the same proof before returning a uid.
- F2, `supervise.report` freshness: helper reports now enforce lane UID lease generation
  freshness before any lease heartbeat, session work heartbeat, terminal metadata, session
  liveness update, or durable event write.
- F3, provider credential selector domain: modeled and uncovered provider-owned credential
  selectors resolve relative paths against the lane launch root/repo root and fail closed when
  they point inside the repository, including `CLAUDE_SECURESTORAGE_CONFIG_DIR` and
  `ANTHROPIC_CONFIG_DIR`, while ordinary non-credential relative env such as `AGY_HOME` and
  `FIXTURE_CONFIG_DIR` remains allowed.
- Preservation checks: RFC 0171 generated-record schema/docket work remains ahead of RFC
  0168's schema 47, owner bundle 0023 is registered coherently, MCP bearer material remains
  under supervisor scratch, and selected run-as uid access is explicit for job worktrees and
  plain-dir workspaces.

## Verification

- `git diff --check origin/main...HEAD` - passed.
- `cd go && GOCACHE=/tmp/striatum-rfc0168-apply-go-cache go build ./...` - passed.
- `cd go && GOCACHE=/tmp/striatum-rfc0168-apply-go-cache go vet ./...` - passed.
- `cd go && GOCACHE=/tmp/striatum-rfc0168-apply-go-cache go test -count=1 ./pkg/mutations/... ./pkg/agentloop/... ./pkg/laneproviderauth/... ./pkg/admin/... ./pkg/db/... ./pkg/reads/... ./pkg/rpc/...` - passed.
- `cd go && GOCACHE=/tmp/striatum-rfc0168-apply-go-cache go test ./...` - passed.
- `GOCACHE=/tmp/striatum-rfc0168-apply-go-cache make check-docs` - passed (`check-docs: OK`).
- `GOCACHE=/tmp/striatum-rfc0168-apply-go-cache make lint` - passed (`0 issues.`).
- `GOCACHE=/tmp/striatum-rfc0168-apply-go-cache make typecheck` - passed.
- `GOCACHE=/tmp/striatum-rfc0168-apply-go-cache make smoke` - passed; PostgreSQL integration
  was skipped because `STRIATUM_DAEMON_DB_URL` is unset, and fresh-clone smoke reported OK.

## Remaining Operator Work

- Provision and deploy schema 47 plus owner bundle 0023 with the matching binary before
  enabling the RFC 0168 posture on a live host.
- Pre-provision `STRIATUM_LANE_UID_POOL` users with the documented sudo, PostgreSQL-deny,
  scratch, repo ACL, provider credential-store, and worktree access posture.
- After deployment and host provisioning, re-run live operator checks before unblocking RFC
  0143 Slice B.
