# Interrogation Chat Log - Build Review Adversarial

- **Interrogation ID**: `intg_90d17b324a97c16e7be2a144eca4a5e4`
- **Target Session ID**: `sess_be356331bb2d0e88ef78541a9d2cfad2`
- **Interrogator Session ID**: `sess_09f5a1fa94e4006e7016e9f393e9936d`
- **Posture**: `devils_advocate`

---

## Turn 0 (Round 1: Helper-Owned Attach Bridge Exit & Silent Delivery Blackhole)

### Question (Interrogator)
Based on our devil's advocate review, we see a critical structural gap in Phase 1: when the operator detach/attach-client exits, the helper process (`RunHelper`) receives the exit on `childDone`, emits `HelperEventAttachExited`, and returns `nil`, causing the helper process to terminate and close the PTY master. Since the supervisor's packet delivery/forwarding stream rides this PTY master (and `tmux send-keys` is deferred), any subsequent work packets sent by the daemon will fail to deliver, yet the lane is still reported as healthy/attached. Can you explain why Phase 1 was designed to exit the helper on attach-client exit rather than maintaining the helper process or fallback forwarding, and how this prevents silent packet-delivery blackholes in a live run?

### Answer (Implementer / Codex Builder)
You are right to flag this as a real residual hazard. The Phase 1 design I implemented followed the synthesis distinction between two different attach clients: an operator-run `tmux attach-session`, which is genuinely observer-only and should never affect helper liveness, and the helper-owned attach bridge, which is still the byte-delivery PTY. Detaching the operator's separate attach client does not hit `RunHelper`'s `childDone` path, so the main RFC 0089 operator-observer bug is closed.

But if the helper-owned attach bridge itself exits while the pane remains live, Phase 1 currently emits `attach_client_exited`, returns nil, and leaves the daemon row `attached`. That does not by itself preserve future packet delivery. Since packet forwarding still rides the helper/FIFO -> attach PTY path and `tmux send-keys` was deferred, a later delivery could block or fail at the FIFO/pipe layer even though the tmux pane probe reports `tmux_ok`. My handoff called out this residual risk as "byte delivery still rides the helper-owned attach PTY," but that is not a complete prevention story for blackholes.

The correct tightening is one of two paths: keep the helper process alive and keep/restore a forwarding handle when only the attach bridge exits, or treat helper-owned attach bridge exit as delivery-degraded state rather than fully `attached` until a send-keys/rebridge mechanism exists. For Phase 1 as scoped, I would revise toward the second option: continue treating operator attach/detach as observer-only, but do not let a helper-owned bridge exit leave the lane publishable as healthy for delivery without an explicit `delivery_degraded`/lost-or-detached transition. That would avoid silent blackholes while preserving the core pane-liveness model.

---

## Turn 1 (Round 2: Imperative Configuration Race in launchPTY)

### Question (Interrogator)
There is a potential startup race condition in `launchPTY` in `go/pkg/supervisor/pty.go`: the window option `remain-on-exit on` is set via a separate `exec.Command` after `createCmd.Run()` has already launched the process. If the process is short-lived or crashes immediately upon bootstrap, the pane will exit and be destroyed before `remain-on-exit` is set, causing `CaptureTmuxIdentity` to fail. Why was this option set imperatively after launch rather than passing it as part of the initial `new-session` configuration (e.g., via global/session options or profile), and how does this handle early-bootstrap failure liveness?

### Answer (Implementer / Codex Builder)
That is a valid startup-race finding. I set `remain-on-exit` imperatively after `new-session` because the synthesis called for that one-line addition after `status off`, and I kept the launch shape otherwise unchanged. That choice is only safe for the normal long-lived agent path; it is not a robust treatment of a child that exits before the post-launch option call.

If the pane process crashes immediately after `createCmd.Run()`, tmux can tear down the pane/session before `CaptureTmuxIdentity` and before `remain-on-exit` is applied. In the current Phase 1 implementation, that early-bootstrap failure is handled only indirectly: `CaptureTmuxIdentity` fails, and then `supervision.require_tmux=true` fails closed while optional tmux falls back to plain PTY with `tmux_identity_capture_failed` metadata. That avoids false tmux attestation, but it does not preserve a dead pane for `tmux_pane_dead` classification or operator inspection.

The stronger implementation should set the window option before the agent command can exit. I would revise `launchPTY` to create an inert tmux session/pane first, apply `status off` and `remain-on-exit on`, then start the actual lane command via a controlled `respawn-pane -k`/equivalent launch step, or use an isolated tmux configuration/profile that applies `remain-on-exit` at session creation without changing the operator's global tmux defaults. That would make early-bootstrap failures classify as `tmux_pane_dead` instead of `tmux_identity_capture_failed` fallback/refusal. The current post-launch command is therefore a Phase 1 gap, not a complete early-failure liveness answer.

---

## Turn 2 (Round 3: Non-Linux / Older Tmux Platform Gaps)

### Question (Interrogator)
Let's look at platform and version gaps: `CaptureTmuxIdentity` and `ProbeTmuxLiveness` query the tmux format variable `#{pane_start_time}` and process pid start token `ProcessStartToken(pid)` to guard against PID reuse. However, `ProcessStartToken` is a stub returning `"", false` on non-Linux platforms (like macOS). Furthermore, on older tmux versions (or certain environments), `#{pane_start_time}` might not be supported or might evaluate to an empty or literal string. How does the implementation prevent silent failures/downgrades of the PID reuse guardrail under these platform gaps, and how can we guarantee that a test passing in a Linux container doesn't obscure a security gap on macOS or with older tmux binaries?

### Answer (Implementer / Codex Builder)
The current implementation does not fully prevent that downgrade. It avoids fabricating a mismatched token, but when both sources are unavailable it degrades to pid-only liveness: `CaptureTmuxIdentity` stores an empty `pane_start_token`, and `ProbeTmuxLiveness` treats empty stored or observed start token as `tmux_ok` with `Detail: "start_token_unverified"`. That detail is projected in `tmux.liveness.detail`, but the current attestation path can still mark the lane attested if pid and pane id match. On non-Linux, or with a tmux that does not provide a trustworthy `#{pane_start_time}`, that is weaker than the stated stale-pid-reuse guardrail.

So the honest answer is: Linux tests prove the happy-path guard and mismatch detection, but they do not guarantee the same stale-pid protection on macOS or older tmux. The current tests intentionally include `TestProbeTmuxLivenessEmptyStartTokenIsUnverifiedNotLost`, but that test documents the degradation; it does not close the security gap.

The fix I would make is to turn token absence or non-numeric/literal `#{pane_start_time}` into an explicit liveness/attestation class rather than a quiet OK detail. Concretely:

- Validate tmux `pane_start_time` as a numeric/non-empty token; treat empty or literal `#{pane_start_time}` as token unavailable.
- If a tmux-backed lane has no verifiable pane start token and `ProcessStartToken` cannot supply one, return a distinct class such as `tmux_pane_start_token_unavailable` or set `tmux_ok` only with `lane_attestation=needs_verification`/`unattested`.
- Make byline/lane attestation require pid plus start-token verification for tmux-backed lanes when the product claims stale-pid protection. Plain liveness may remain usable, but provenance attestation should be downgraded.
- Add unit tests that force empty, literal, and unsupported `pane_start_time` outputs via fake tmux runners, plus platform-specific tests or injected `ProcessStartToken` behavior for non-Linux. CI should include at least a macOS job or a deterministic non-Linux simulation test so Linux-only success cannot hide the downgrade.

The current implementation surfaces `start_token_unverified` but does not fail closed or downgrade attestation strongly enough. That is a Phase 1 gap if stale-pid-reuse protection is considered mandatory across supported platforms.
