---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "medium"
tags: ["adversarial-review", "build-review", "tmux-liveness", "concurrency", "platform-gaps"]
---

# Build Review finding: RFC 0089 Phase 1 Build (rerun2)
author: reviewer-gemini-3.5-flash-high-003
verdict: accept_with_findings
round_count: 3
stop_reason: interrogation_completed_with_findings

## Summary of Findings

This adversarial build review evaluated the Phase 1 implementation of RFC 0089 (replacing attach-as-liveness with session/pane process liveness monitoring). The verification test suite passed successfully on the live build.

During the 3-round live interrogation of the Codex builder, several critical residual hazards and implementation edge cases were analyzed. The builder fully conceded these findings and proposed concrete mitigation strategies.

Because the implementation successfully delivers the core tmux-pane liveness substrate and safely keeps operator-run `tmux attach-session` observer-only, the verdict is **accept_with_findings**. The surfaced findings must be addressed in subsequent phases (Phase 2 and the deferred send-keys delivery refactor) to prevent silent delivery blackholes and to guarantee stale-pid-reuse protection across all supported platforms.

---

## 1. Verification Run Results

The implementation was compiled, statically analyzed, and tested in a clean workspace environment. The results are as follows:

```bash
$ cd go && go vet ./... && go test ./...
ok      github.com/halbritt/striatum/go/cmd/striatum    (cached)
ok      github.com/halbritt/striatum/go/cmd/striatumd   (cached)
ok      github.com/halbritt/striatum/go/pkg/agentloop   0.008s
ok      github.com/halbritt/striatum/go/pkg/mutations   0.091s
ok      github.com/halbritt/striatum/go/pkg/reads       0.047s
ok      github.com/halbritt/striatum/go/pkg/supervisor  0.453s
```

All 40 Go packages compiled cleanly, `go vet` reported no static analysis failures, and all supervisor-liveness and helper regression tests passed successfully.

---

## 2. Finding 1: Silent Packet-Delivery Blackhole on Helper-Owned Attach Bridge Exit
**Severity**: High
**Status**: Conceded & Resolved (Deferred to Phase 2 / Rebridge)

### Description
In Phase 1, `RunHelper` terminates and returns `nil` when the supervisor's internal attach client process exits (detected via the `childDone` channel). The daemon-side `recordSuperviseReportEvent` correctly recognizes that the pane is still live and maintains the supervisor row state as `attached` (to avoid false lost-supervisor states).

However, because the packet forwarding stream (`forwardPacketStream`) still runs within the helper process and pumps bytes directly to the attach PTY, the termination of `RunHelper` permanently severs the byte-delivery connection. A live lane is thus falsely reported as healthy/attached while being in a silent blackhole where no future packet bytes can ever be delivered (since `tmux send-keys` is deferred).

### Concession & Resolution
The builder conceded this residual risk, acknowledging that a helper-owned bridge exit currently leaves the lane publishable as healthy but delivery-degraded. The builder proposed two mitigation paths:
1. **Helper Self-Heal**: The helper could remain alive, close the dead ptmx, launch a fresh attach client, swap the descriptor, and emit a `HelperEventAttachRecovered` event to update the daemon.
2. **Attestation/Status Downgrade**: For the remainder of Phase 1, the daemon must not leave a helper-terminated session in a quiet healthy `attached` state. If the helper-owned bridge exits, the supervisor state must transition to a distinct `delivery_degraded` or `detached` classification, forcing an explicit rebridge rather than silently swallowing packets.

---

## 3. Finding 2: Imperative Option Configuration Race in `launchPTY`
**Severity**: Medium
**Status**: Conceded & Resolved

### Description
In `go/pkg/supervisor/pty.go`, the session/pane option `remain-on-exit` is applied imperatively via a separate `exec.Command` *after* the initial `createCmd.Run()` has already launched the tmux process.

If the supervised lane process crashes immediately upon startup (e.g., due to an early bootstrap/adapter failure), the tmux pane will exit and be destroyed before the `remain-on-exit on` option can be applied. This causes the supervisor to report `tmux_session_missing` or `tmux_pane_missing` rather than retaining the dead pane for a proper `tmux_pane_dead` liveness classification and operator terminal inspection.

### Concession & Resolution
The builder conceded this startup race. To resolve this, the builder proposed that the window option should be set before the lane command is allowed to execute:
- Create the detached tmux session/pane in an inert state first, apply the `status off` and `remain-on-exit on` window options, and then respawn the pane with the actual lane command via `respawn-pane -k` or another controlled launch command.
- Alternatively, use an isolated tmux configuration profile that enforces `remain-on-exit on` at session creation without mutating the operator's global tmux defaults.

---

## 4. Finding 3: Security & Attestation Degradation on Non-Linux / Older Tmux
**Severity**: Medium
**Status**: Conceded & Resolved

### Description
The stale-PID-reuse guardrail relies on verifying the process kernel start-time. However, `ProcessStartToken(pid)` is a stub returning `"", false` on non-Linux platforms (such as macOS). Furthermore, older versions of `tmux` (pre-2.1) do not support the `#{pane_start_time}` format variable, returning an empty or literal format string.

Under these platform/version gaps, `ProbeTmuxLiveness` silently falls back to basic PID/pane liveness, reporting `Detail: "start_token_unverified"` but still returning a healthy `tmux_ok` class. This allows a recycled process to bypass the anti-fabrication and provenance attestation safeguards of D080/D149 without trigger warning or attestation downgrade.

### Concession & Resolution
The builder conceded that while Linux tests prove the happy-path reuse guardrail, they do not guarantee the same stale-pid protection on macOS or with older tmux binaries. The builder proposed that:
- The daemon should validate `pane_start_time` as a numeric token; literal strings or empty outputs must evaluate to token unavailable.
- Token absence or platform verification failure must return a distinct class (`tmux_pane_start_token_unavailable`) and downgrade the lane attestation to `needs_verification` or `unattested`, rather than returning a quiet `tmux_ok`.
- CI/CD pipelines should incorporate non-Linux (e.g., macOS) execution runners or a mock simulation to ensure platform-specific degradation is explicitly tested rather than obscured by Linux-only success.
