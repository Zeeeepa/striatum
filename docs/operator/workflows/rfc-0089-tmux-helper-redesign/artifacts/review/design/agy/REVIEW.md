---
schema_version: "striatum.finding.v1"
artifact_kind: "finding"
verdict_intent: "accept_with_findings"
severity: "low"
tags: ["synthesis", "review", "tmux", "caching", "attestation"]
---

# Design Review finding: RFC 0089 Phase 1 Synthesis

**Author:** reviewer-gemini-3.5-flash-high-001
**Role:** reviewer
**Lane:** agy
**Date:** 2026-05-28

## Summary of Findings

This is an adversarial design review of the Phase 1 synthesis (`DESIGN_SYNTHESIS.md`) for the tmux helper redesign (RFC 0089).

Following three rounds of active peer interrogation against the live synthesizer (`sess_69019c1711b188f953cce5529c3f9dd9`), three load-bearing assumptions were challenged and successfully resolved. The synthesizer has formally conceded three refinements, which are incorporated into the design as accepted findings.

The final verdict is **ACCEPT WITH FINDINGS**.

---

## Curated findings

### Finding 1: Document `C-b d` (Detach) Stalling in the Runbook
*   **Context:** The synthesis defers the reattach implementation to Phase 2 while keeping the attach client PTY master as the sole packet transport (byte sink). When the attach client exits, the supervisor transitions to `detached` but keeps the attestation alive, rejecting subsequent sends with `needs_reattach`.
*   **Problem:** This introduces operational friction during Phase 1: if an operator detaches from the session or experiences a network hiccup, the lane is permanently blocked from receiving packets until the supervisor is restarted.
*   **Resolution:** While acceptable for the Phase 1 unblocker to avoid expanding the implementation blast radius, the operator runbook (`daemon-runbook.md`) must explicitly document this friction. Operators must be warned to leave the terminal attached or use a non-killing path, and that restarting the supervisor is necessary if a detach occurs before Phase 2 lands.

### Finding 2: Implement Stale-While-Revalidate Caching for Delivery-Time Probes
*   **Context:** Heartbeats allow 3 consecutive probe misses (~15s) before marking the supervisor lost. However, delivery-time probes (§5.2) fail immediately on a single `tmux_unavailable` timeout (2s), refusing packet delivery with `invalid_transition`.
*   **Problem:** This asymmetry makes delivery highly fragile under transient host load or temporary tmux server slowness (a common concurrency issue), causing packets to flap and fail unnecessarily.
*   **Resolution:** The synthesis will adopt a **Stale-While-Revalidate** mechanism for delivery-time liveness:
    1.  The supervisor pointer metadata gains a `tmux_last_ok_at` timestamp, updated by the heartbeat loop upon successful probe.
    2.  If a `tmux_unavailable` or timeout occurs during `supervise.send`, `reconcileSupervisorForDelivery` checks `(now - tmux_last_ok_at) < 10s` (the default `tmux_probe_cache_max_age`).
    3.  If within this 10-second window, the delivery proceeds successfully, including `liveness_cache_age_ms` in the event payload for auditability.
    4.  If outside the window, it defers with `invalid_transition`.
    5.  Tests will be added: `TestReconcileForDeliveryAcceptsStaleProbeWithinCacheWindow` and its negative counterpart.

### Finding 3: Upgrade Start-Token Observability & Monotonicity Assurances
*   **Context:** On older tmux versions (< 2.9) or procfs-isolated environments (macOS/containers), both `pane_start_time` and `processStartToken` fallback are empty, causing the probe to fall back to `start_token_unverified` (but still reporting `TmuxLivenessOK`).
*   **Problem:** Disabling the start-token check degrades safety (pid reuse/start-time mismatch). While the monotonic counter of tmux pane IDs (`%N`) provides a stronger replacement guarantee (preventing slot reuse), this degradation is silently buried in the nested JSON liveness block.
*   **Resolution:** The synthesis will improve degradation visibility and document the monotonicity invariant:
    1.  **Top-Level Caveat:** The status projection (`supervise.status` and dashboard) will expose a top-level `attestation_caveat: "start_token_unverified"` when the cross-check is unverified.
    2.  **Launch-Time Event:** `supervisor.started` event payload gains `start_token_source` ∈ `{"tmux_pane_start_time", "process_start_token", "unverified"}`.
    3.  **Health Auditing:** `doctor --verbose` will emit a health problem string when running with unverified tokens to flag distros needing upgrades.
    4.  **Runbook Clarification:** `daemon-runbook.md` will explain that on legacy/procfs-isolated hosts, the start-token cross-check is unavailable; the pane_id monotonicity invariant carries the safety property; operators who consider that risk model unacceptable should upgrade tmux or mount `/proc`.
    5.  Tests will be added: `TestProbePaneIDMonotonicityIsSufficientWhenStartTokenEmpty` and `TestProbeRejectsPaneIDMismatchEvenWithMatchingPID`.

---

## Interrogation Summary

- **Total Rounds:** 3 Q&A turns
- **Stop Reason:** Bounded interrogation successfully completed with all concerns resolved and refinements accepted.
- **Interrogation Chat Log:** Recorded in [INTERROGATION_CHAT.md](file:///home/halbritt/git/striatum/docs/operator/workflows/rfc-0089-tmux-helper-redesign/artifacts/review/design/agy/INTERROGATION_CHAT.md).
