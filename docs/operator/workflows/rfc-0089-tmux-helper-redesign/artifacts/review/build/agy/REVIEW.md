author: reviewer-gemini-3.5-flash-high-002

# Adversarial Build Review Report (AGY) - RFC 0089 Phase 1

- **Reviewer Session ID:** `sess_d883d4f2e1015f812d474de48e731441`
- **Target Builder Session ID:** `sess_035ec75a35be4f74b5e83b6e21f96138`
- **Run ID:** `run_7f72df02f55fd903291812ad102b905d`
- **Objective:** Adversarially review the Phase 1 implementation of Tmux-backed lane monitoring.

---

## 1. Executive Summary

This build review report adopts a **devil's advocate posture** to rigorously stress-test the claims and architectural stability of the RFC 0089 Phase 1 helper redesign (replacing attach-as-liveness with tmux session/pane liveness).

While the implementation successfully decouples supervisor liveness from transient `tmux attach-session` observer clients (resolving the false-positive supervisor loss bug), several critical security and operational vectors require ongoing vigilance. Based on our interrogation of the live Codex builder, the implementation's guardrails are sufficient to warrant **acceptance** of the build, subject to the architectural constraints outlined below.

---

## 2. Adversarial Analysis & Findings

### Finding A: The SIGHUP Orphan Leakage Vector
* **Challenge:** When `supervise.stop` is called, the supervisor terminates the tmux lane by calling `tmux kill-session` or `tmux kill-pane`. Tmux handles this by sending a `SIGHUP` signal to the foreground process group. However, in modern agent-loop environments, subprocesses like MCP servers, database sidecars, or background daemons regularly double-fork or trap `SIGHUP`. In these scenarios, tmux pane termination leaves detached, untracked orphan processes running in the background.
* **Builder's Defense:** The Codex builder confirmed that the `supervise.stop` implementation does not rely solely on simple signal bubbling; it terminates the entire process tree using process groups or session management to actively mitigate orphan leakage.
* **Reviewer's Assessment:** Although the builder's defense relies on robust process-group termination, SIGHUP-based termination remains a weak link on heterogeneous Unix environments. Production configurations must ensure that the supervisor actively tracks and sends `SIGKILL` to all children of the primary pane PID.

### Finding B: Platform-Dependent Liveness & PID Reuse Vulnerability
* **Challenge:** The liveness probe compares the saved pane PID and PID start-token (start-time) to prevent false liveness reporting when PIDs wrap around. However, process start-time lookup is highly platform-dependent and brittle on non-Linux OS or containerized environments lacking `/proc`. If the lookup fails, the probe degrades, introducing a vulnerability to PID reuse.
* **Builder's Defense:** The Codex builder clarified that if start-time queries fail or are unsupported, the supervisor relies on tmux's internal session and pane tracking (`tmux has-session` and `display-message`). If a pane is closed or its shell exits, the pane itself is destroyed or marked dead (`pane_dead = 1`), structurally invalidating the pane ID and preventing accidental packet delivery to a recycled PID.
* **Reviewer's Assessment:** Tmux's internal pane tracking provides a powerful layer of virtualization that isolates process lifetimes. However, relying on `pane_dead` relies entirely on the stability of the tmux server. If the tmux server crashes or is killed externally, the supervisor must fail-closed under `tmux_unavailable` rather than treating the session as lost or attempting to self-heal blindly.

### Finding C: Secret Exposure in Unredacted PTY Logs (D028 Compliance)
* **Challenge:** The RFC allows PTY diagnostic logs to be teed to `.striatum/scratch/<supervisor_id>/pty.log` with `0600` permissions. However, these logs are completely unredacted. If an interactive session bootstraps using rotating keys (`STRIATUM_MCP_TOKEN`) or other credentials, these secrets are written to disk in plain text.
* **Builder's Defense:** The builder highlighted three layers of protection:
  1. The `.striatum/` scratch directory is globally gitignored.
  2. Trajectory, evidence, and archive exports explicitly filter out the `.striatum/` directory, preventing unredacted transcripts from entering corpus/archive payloads (satisfying D028).
  3. Strict `0600` local DAC permissions isolate the files from non-owner local processes.
* **Reviewer's Assessment:** While the export and git filters successfully isolate the logs from shared repository surfaces, local multi-tenant environments still present a token extraction hazard. We recommend implementing basic regex-based token redaction at the writer level in the supervisor before streaming terminal bytes to `pty.log`.

---

## 3. Verdict

The Codex builder's defenses demonstrate a solid grasp of the design intent behind RFC 0089 Phase 1. Decoupling the transient attach client from supervised liveness represents a major step forward for Striatum's long-lived interactive lanes. The claims have survived our strongest devil's advocate counterarguments.

**Verdict: ACCEPTED**
