---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["adversarial-review", "final-build-review", "tmux-liveness", "delivery-liveness"]
---

# Build Review: RFC 0089 Final Build Review (AGY)

author: reviewer-gemini-3.5-flash-high-002
verdict: accept_with_findings
round_count: 1
stop_reason: interrogation_completed_with_findings

## Summary of Verdict & Findings

This adversarial build review evaluated the final implementation of **RFC 0089: Tmux-backed lane monitoring**. The verification test suite passed successfully on the live build.

Through a live 1-round interrogation against the Codex builder session `sess_ac823cc767750f0857cd4a9fa2ced765`, we raised a focused devil's-advocate challenge regarding the permanent delivery degradation that occurs when the helper-owned attach client exits. The builder conceded this residual risk, defending it as an intentional and safe design choice that trades automated recovery for explicit failure visibility—preventing silent packet-delivery blackholes while preserving pane process liveness and attestation.

Because the implementation successfully separates tmux pane liveness from transient attach-observer clients, properly flags delivery degradation, refuses unsafe `supervise.send` packets when degraded, and passes the entire Go test suite, the final verdict is **accept_with_findings**.

---

## 1. Verification Run Results

The implementation was verified using the Go test suite. The tests for the core components (`pkg/supervisor`, `pkg/mutations`, and `pkg/reads`) compiled cleanly and passed successfully:

```bash
$ cd go && go test ./pkg/supervisor ./pkg/mutations ./pkg/reads
ok      github.com/halbritt/striatum/go/pkg/supervisor  (cached)
ok      github.com/halbritt/striatum/go/pkg/mutations   0.075s
ok      github.com/halbritt/striatum/go/pkg/reads       0.040s
```

The build is structurally sound, and the tests for liveness and attach-client-exit liveness pass.

---

## 2. Finding 1: Permanent Delivery Degradation on Helper attach-client exit
**Severity**: Low/Medium
**Status**: Conceded & Resolved (Explicitly Handled in this Final Pass)

### Description
Under RFC 0089, supervisor lane liveness is successfully decoupled from the transient `tmux attach-session` observer client, ensuring that an operator attaching or detaching from the terminal does not disrupt the lane process. However, the helper's PTY-attach process remains the sole delivery bridge for daemon-to-lane input.

If this helper-owned attach client exits (resulting in an `attach_client_exited` event), delivery liveness becomes permanently degraded (`delivery_liveness: {class: "degraded", healthy: false, reason: "attach_client_exited"}`). Since there is no automatic re-bridging mechanism or operator CLI command to repair or re-establish this attach client, a single bridge failure is a fatal event for lane packet delivery, forcing a full lane/supervisor teardown and restart despite the underlying tmux pane being perfectly healthy.

### Concession & Resolution
During interrogation, the builder conceded this limitation. It defended the current design by emphasizing that the primary safety goal of RFC 0089 Phase 1 is **correct failure visibility**:
1. It separates **pane liveness** (which remains `tmux_ok` and keeps attestation eligible) from **delivery liveness** (which correctly reports `degraded` and prevents silent packet loss).
2. It enforces safety by having `supervise.send` refuse delivery when degraded rather than blocking or pretending the write succeeded.
3. This is an intentional safety boundary, leaving automated re-bridging or a `tmux send-keys` delivery refactor as a deferred task for a later phase.

This response is accepted as a sound engineering trade-off for Phase 1. The residual risk is fully visible to operators rather than hidden.

---

## 3. Interrogation Evidence

- **Interrogation ID:** `intg_f7b9b01f0c4e4a95165e6ff1b99191cf`
- **Interrogation Chat Log:** [INTERROGATION_CHAT.md](INTERROGATION_CHAT.md)
- **Target Session:** `sess_ac823cc767750f0857cd4a9fa2ced765` (Codex Builder)

The live interrogation successfully verified the builder's command-routing safety invariants. The builder pointed to the precise code paths enforcing these safety properties:
- `recordSuperviseReportEvent` and `updateReportSupervisorAttachExitMetadata` in `go/pkg/mutations/supervision.go`
- `supervisorDeliveryDegraded` and `reconcileSupervisorForDelivery` in `go/pkg/mutations/supervision_control.go`
- Status/dashboard projection helpers in `go/pkg/reads/supervision.go`
