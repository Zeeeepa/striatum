# RFC 0168 Build v3 Final v2 Findings Context

This workflow supersedes canceled v2 run `run_b446843c817d2a44a7e7877a106f42e8`.

The final v2 review artifact was `art_048e8cad64d2d03c9d688591c6bac4e7`.
A local operator copy exists at:

`/tmp/striatum-rfc0168-run-b446843c817d2a44a7e7877a106f42e8/REVIEW.md`

The canceled v2 branch head was `24d85e5f`. Treat that branch as reference
material only. Do not manually merge, cherry-pick, or replay rejected code
outside this new daemon run. All accepted source must land through the v3
Striatum workflow.

## Accepted v2 Material To Preserve

- RFC 0171 generated-records schema and docket work was preserved in v2; keep
  it preserved.
- RFC 0168 used runtime migration ordinal `0047` and owner bundle ordinal
  `0023`; recheck current `main` before relying on those slots.
- MCP bearer material was moved under supervisor scratch.
- `CLAUDE_SECURESTORAGE_CONFIG_DIR` was modeled for absolute paths.
- `AGY_HOME` and `FIXTURE_CONFIG_DIR` were retained as positive controls for
  ordinary non-credential lane environment.
- The selected run-as uid received access to job worktrees and workspaces.

## Blocking F1: UID Return Proof Was Not Proof-Gated

The v2 implementation returned a uid whenever `scrubLaneUIDArtifacts` and
`appendLaneUIDScrubProof` returned nil in `go/pkg/mutations/lane_uid_leases.go`.
The relevant v2 review line references were `312`, `329`, `331`, `333`, and
`348`.

Only the P1 process scan was hard proof, referenced at `379-390`.

Specific gaps:

- S1 `kill -KILL -1` failure was recorded and ignored instead of fail-closing
  or quarantining, referenced at `397-400`.
- S2 removed `.codex` and `.claude` but did not prove provider credential-store
  absence after cleanup, referenced at `402-415`.
- S3 removed supervisor scratch but did not prove tmux socket, HOME and
  reseal-token cleanup, per-lease ACL cleanup, or worktree cleanup before uid
  return, referenced at `417-428`.
- P4 was explicitly recorded as `deferred_to_worktree_release_or_gc`, and P5
  was true unconditionally, referenced at `427-428`.
- Recovery listed stuck active and scrubbing leases, but quarantined rows only
  reported `operator_retry_required`, referenced at `522-604`.

Acceptance requirement: uid return must be blocked until complete S1-S3 and
P1-P5 proof exists. Quarantined or stuck-state recovery must expose an operator
retry path that reruns P1-P5 and returns the uid only after clean proof.

## Blocking F2: supervise.report Did Not Enforce Generation Freshness

V2 enforced generation freshness for attestation overlay and active control
lookup in `go/pkg/mutations/lane_uid_leases.go`, referenced at `200-243` and
`245-280`, and `go/pkg/mutations/supervision_control.go`, referenced at
`990-1039`.

The helper report path remained open:

- `supervise.report` used `recordSuperviseReportEvent`, then
  `findReportSupervisor`, and checked only active-ish supervisor state in
  `go/pkg/mutations/supervision.go`, referenced at `283-324` and `363-370`.
- `scanReportSupervisor` read pointer metadata and returned without a freshness
  check, referenced at `530-568`.

Acceptance requirement: the report path must compare live lane uid generation
against `lane_uid_leases` before heartbeat or terminal metadata updates. Add
stale-generation negative tests for helper reports.

## Blocking F3: Relative Provider Credential Selectors Were Not Refused

V2 accepted arbitrary string values in `command_env` in
`go/pkg/supervision_lane_config.go`, referenced at `422-443`.

The credential guard checked modeled and unmodeled provider credential
selectors, but `pathInsideRepo` returned false for non-absolute paths in
`go/pkg/credential_domain_guard.go`, referenced at `27-49` and `91-99`.

Supervised lanes launch with `cwd=config.RepoRoot` in
`go/pkg/mutations/supervision_control.go` and `go/pkg/supervision_env.go`,
referenced at `887-893` and `265-277`.

That means relative values such as
`CLAUDE_SECURESTORAGE_CONFIG_DIR=docs/.lane-auth/secure` and
`ANTHROPIC_CONFIG_DIR=docs/.lane-auth/anthropic` resolve inside the target repo
without refusal.

V2 tests only covered absolute in-repo provider selectors and ordinary
non-credential environment, referenced in `go/pkg/credential_domain_guard_test.go`
at `12-55`.

Acceptance requirement: relative provider credential selectors must resolve
against the lane launch cwd or repo root and fail closed if they point inside the
target repository. Ordinary non-credential relative environment must remain
allowed where intended.
