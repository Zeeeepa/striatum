# BRIEFING — 2026-05-29T12:02:57Z

## Mission
Investigate PTY supervision issues #49 and #54: re-queued packet after checkpoint resolution does not resume, and RFC 0089 Phase 2 supervision rebridge and status details.

## 🔒 My Identity
- Archetype: explorer
- Roles: Explorer 3
- Working directory: ~/git/striatum/.agents/teamwork_preview_explorer_m1_3_gen3
- Original parent: bf988de2-7780-459e-9f86-805f4f350203
- Milestone: PTY Supervision issues #49 & #54

## 🔒 Key Constraints
- Read-only investigation — do NOT implement
- Operational code-only mode

## Current Parent
- Conversation ID: bf988de2-7780-459e-9f86-805f4f350203
- Updated: yes

## Investigation State
- **Explored paths**:
  - `go/pkg/mutations/claim.go` (HandleClaimNext)
  - `go/pkg/mutations/operator.go` (HandleCheckpointResolve)
  - `go/pkg/mutations/lifecycle.go` (HandleBlockWork)
  - `go/pkg/agentloop/loop.go` (startDaemonReceiverLoop, daemonReceiverReady)
  - `go/pkg/lanehealth/lanehealth.go` (Facts, Classify, Checker.Check)
  - `go/pkg/reads/supervision.go` (HandleSuperviseStatus, sessionProtocolLiveness, reattachStatusView)
  - `go/pkg/supervisor/liveness.go` (liveness checks)
  - `go/pkg/supervisor/tmux_liveness.go` (ProbeLaneLiveness)
- **Key findings**:
  - Identified the root cause of Issue #49: `HandleClaimNext` filters out re-queued jobs on the same session when `fresh_session_required` is `true`. Adding `wp.job_id != qm.job_id` resolves it.
  - Identified the mechanism and gaps for Issue #54: Rebridge uses `striatum-supervisor-helper` to bridge the daemon FIFO/MCP to Tmux PTY. However, `lanehealth.Check` lacks PID verification for the `helper_pid`, leaving silent helper process crashes unreported.
- **Unexplored areas**: None, task scope fully covered.

## Key Decisions Made
- Performed detailed read-only codebase search and logical tracing.
- Structured concrete implementations recommendations for both issues.

## Artifact Index
- ~/git/striatum/.agents/teamwork_preview_explorer_m1_3_gen3/original_prompt.md — Original task prompt
- ~/git/striatum/.agents/teamwork_preview_explorer_m1_3_gen3/analysis.md — Detailed deep dive report
- ~/git/striatum/.agents/teamwork_preview_explorer_m1_3_gen3/handoff.md — Self-contained five-component handoff report
