author: operator

# Open GH Issues Friction Log

Date: 2026-05-14
Branch: `striatum/gh-issues-parallel`

## Entries

### F001 - Daemon And MCP Surface Unavailable During Initial Run Start

- Command/tool: `.venv/bin/striatum` run lifecycle commands.
- Observed friction: daemon-required commands failed with
  `daemon_unreachable` at `/run/user/1000/striatum/striatumd.sock`, and no
  daemon MCP tool surface was available in the session.
- Workaround used: initial scaffolding and runs used
  `STRIATUM_DAEMON_REQUIRED=0 STRIATUM_TEST_HARNESS=1`.
- Impact: the operator flow contradicted the desired daemon-first direction and
  made later D103 cleanup necessary.
- Follow-up: D103 now records daemon plus daemon MCP as mandatory; future
  operator prompts treat missing daemon MCP as a blocker unless the human
  explicitly authorizes break-glass test mode.

### F002 - Editable Install Reported A Stale Version

- Command/tool: `make install`, `.venv/bin/striatum --version`, and
  `PYTHONPATH=src python3 -m striatum.cli --version`.
- Observed friction: `make install` completed but reported
  `striatum-orchestrator-1.36.0`, while source metadata and module execution
  indicated a newer version.
- Workaround used: continued with `.venv/bin/striatum` because commands
  executed, while recording the version anomaly.
- Impact: reduced confidence in environment provenance and test repeatability.
- Follow-up: add an install/version sanity check to operator initialization or
  release metadata checks.

### F003 - Supervisor Logs Shared A Default Packet Path

- Command/tool: supervised wrappers for Codex, Claude, and Gemini sessions.
- Observed friction: wrappers wrote to shared default scratch log paths such as
  `.striatum/scratch/codex-logs/packet-0001.log`.
- Workaround used: trusted Striatum state and artifacts over logs.
- Impact: live log observability was poor while sessions ran concurrently.
- Follow-up: assign per-supervisor scratch/log paths or expose them through the
  control plane.

### F004 - GH #12/#13 Review Could Not See Required Evidence

- Command/tool: `review_ergonomics` in
  `run_1b89c643a3554bbaa86192e57bc5e791`.
- Observed friction: the second Codex ergonomics review returned
  `needs_revision` because the packet did not expose implementation and test
  evidence required by the review prompt.
- Workaround in progress: inspect checkpoint-resolution commands and reroute or
  requeue role work with the correct evidence surface.
- Impact: cycle exhausted into human checkpoint
  `blk_9df968ca407f4378b81936671634c739` even though the security review
  accepted the implementation.
- Follow-up: generated review jobs need self-contained evidence inputs or
  broader review read scope.

### F005 - GH #9/#10/#11 Final Security Review Had No Revision Cycle

- Command/tool: `review_build_codex` in
  `run_ba9f16af26204248b7f7d0a8e30ffa33`.
- Observed friction: final security review returned `needs_revision`, but the
  workflow had no matching revision cycle for that review job.
- Workaround in progress: send implementation gaps to a role-owned worker and
  inspect safe checkpoint rerouting commands.
- Impact: checkpoint `blk_82bb6b6033ef4abcab4393fe782171f6` stopped the run
  despite other final reviews accepting or accepting with findings.
- Follow-up: security-hardening bundle workflows need a bounded final-review
  revision loop.

### F006 - Full Test Run Exposed Unowned Failures

- Command/tool: `make test`.
- Observed friction: 873 passed and 45 skipped, but 6 failed across daemon
  tests, daemon-RPC schema version expectation, and static asset URL scanning.
- Workaround in progress: parallel sub-agents own independent failure slices.
- Impact: cannot call the combined branch releasable until failures are fixed
  or explicitly scoped out.
- Follow-up: keep narrow failure ownership and rerun the full suite after
  integration.

### F007 - Requeued Fresh-Required Review Refused Existing Session

- Command/tool: `striatum claim-next --session-id
  sess_11048dd583d048229d5ae657ef2cf76c` after resolving
  `blk_9df968ca407f4378b81936671634c739` with `checkpoint resolve
  --action continue`.
- Observed friction: the existing reviewer session returned `no_work` while the
  run had a claimable `codex` reviewer job.
- Workaround used: registered a new fresh Codex reviewer session
  `sess_8a1ded110e614e5c8a1c6019bfb995a4`, attached supervisor
  `sup_62d2b8d801c74ec6af3d7b9fddcfa938`, and claimed the requeued job.
- Impact: extra operator loop and additional session churn.
- Follow-up: checkpoint resolution output should hint when a fresh-required job
  needs a fresh session rather than an existing role/lane session.

### F008 - Review Packets Lacked Handoff And Repo Evidence Scope

- Command/tool: `docs/issues/9/workflow.json` and
  `docs/issues/12/workflow.json` review-job scaffolds.
- Observed friction: review jobs asked reviewers to verify changed files and
  implementer handoffs but defaulted to `document_only` review policy and
  omitted the build handoff from `context_docs`.
- Workaround used: patched the workflow scaffolds to include scope/handoff
  context and set final build reviews to `reviewer_access_scope: "repo_level"`.
- Impact: current run snapshots retain the old packet shape, but future reruns
  or regenerated issue workflows have enough evidence for role reviewers.
- Follow-up: workflow generator templates should default implementation reviews
  to repo-level access when the prompt asks for source/test verification.

### F009 - Native Worker Was Initially Given Implementation Scope

- Command/tool: native sub-agent
  `019e26f8-a487-76b3-8e0a-6e56797be818`.
- Observed friction: the worker was initially instructed to patch GH #9
  implementation gaps directly, which blurs the boundary between operator-side
  coordination and role-owned implementation.
- Workaround used: interrupted the worker and redirected it to stop editing and
  report findings/recommendations only.
- Impact: the operator must inspect any resulting working-tree changes for
  provenance before committing them.
- Follow-up: operator prompts should distinguish native operator-side audit
  sub-agents from first-class Striatum role sessions. Implementation and design
  patches should flow through role sessions or be explicitly recorded as
  operator-authored maintenance outside the workflow.
