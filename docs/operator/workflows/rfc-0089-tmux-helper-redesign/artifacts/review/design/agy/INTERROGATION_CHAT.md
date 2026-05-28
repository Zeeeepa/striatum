# Interrogation Chat Log

- **Interrogation ID:** `intg_88e0d609c8e9588bcecf5206c32893fb`
- **Reviewer Session ID (Interrogator):** `sess_1ce0d091195f57f960b6e49784b96cab`
- **Target Session ID (Synthesizer):** `sess_69019c1711b188f953cce5529c3f9dd9`
- **Run ID:** `run_7f72df02f55fd903291812ad102b905d`
- **Date:** 2026-05-28

---

## Turn 0: Question
**Author:** reviewer-gemini-3.5-flash-high-001 (AGY Reviewer)
**Message ID:** `msg_29f7de8dcabfd120263299100e9bc994`
**Timestamp:** `2026-05-28T15:21:46Z`

The synthesis chooses to defer the `reattach` implementation to Phase 2 while keeping the attach client PTY master as the sole packet transport (byte sink). When the attach client exits, the supervisor transitions to the `detached` state, and all subsequent `supervise.send` calls are rejected with `needs_reattach`.

Since Phase 1 has no automatic self-healing or reattach mechanism, this means that if an operator detaches from a tmux session (e.g., `C-b d` or terminal disconnect) or if the attach client exits due to any network/PTY hiccup, the entire lane becomes completely blocked from receiving any more packets until the operator manually intervenes.

Doesn't this structural choice introduce a severe regression where any transient attach client disconnect completely stalls the workflow and leaves it permanently blocked? Why is this acceptable for Phase 1 if the goal is robust long-lived lane monitoring?

---

## Turn 1: Answer
**Author:** synthesizer-claude-opus-4.7-001 (Claude Synthesizer)
**Message ID:** `msg_ec737df7e8d61660019872f35d9a63b7`
**Timestamp:** `2026-05-28T15:26:24Z`

The framing of this as a "severe regression" is wrong. Phase 1 strictly improves the failure mode; it does not create it. Compare the two states explicitly:

*   **Today (pre-Phase-1)**: attach client exits → `supervisor.lost` → `lane_attestation = unattested` → `recovery.process_reconcile` flips `process_executions` to `lost` → any later `supervise.send` returns `pid_gone` → byline rules downgrade artifacts to `author: operator`. The lane *process* may still be alive inside tmux, but Striatum has no model that says so, and the supervisor row cannot be recovered without starting a new supervisor (which today recreates the tmux session and loses the live agent's preserved context).
*   **Phase 1**: attach exit → `detached` (NOT `lost`) → `lane_attestation` stays `attested` because the pane probe is still `tmux_ok` → `supervise.send` returns `needs_reattach` with a structured failure class → operator sees the state in `supervise.status` and `dashboard` → `tmux attach-session -t <name>` still works for human observation → the agent's preserved context is intact.

So the regression scenario the reviewer describes ("any transient disconnect completely stalls the workflow until operator intervention") is *better than today's behaviour*, where the same disconnect kills the supervisor permanently. We are moving from "irrecoverable" to "recoverable but currently requires operator action". That is the canonical shape of a phased migration that the RFC itself sequences.

**Why reattach is sequenced to Phase 2, not bundled into Phase 1**:

1.  The reattach verb has its own design surface that does not belong in the liveness-probe slice: idempotency under concurrent reattach attempts; authorization scoping (which capability tokens may reattach a supervisor they did not start); the `detached → reattach_pending → attached` state machine with retry budgets and timeouts; failure mode when reattach itself fails (does the supervisor go `lost` or stay `detached` for retry?); and PTY-master ownership (the helper holds the master today; reattach has to swap it without dropping in-flight FIFO bytes). Each of those is a distinct decision and each one risks the liveness fix if bundled in.
2.  RFC 0089 §"Implementation Sequence" explicitly partitions the work this way, calling Phase 1 "the unblocker" and Phase 2 the read-surface phase that brings attach commands to first-class status. The synthesis is following the RFC's own ordering, not inventing it.
3.  TASK.md scopes Phase 1 as "make `tmux attach-session` an observer only" and lists liveness-probe code points exclusively. Adding a new RPC verb and pointer state-machine transitions into this slice expands the implementer's blast radius from ~6 files to ~12, and increases the chance Phase 1 itself slips.

**On the alternative the reviewer is implicitly pointing at (Codex's buffer-paste delivery)**:

Codex's design proposed `tmux load-buffer | tmux paste-buffer -t <pane_id>` as the byte transport, which would have removed the attach client from the delivery path entirely and made attach exit a true zero-impact event for packet flow. I rejected that for Phase 1 because:

*   Bracketed paste into the Claude TUI input editor is uncharacterized. The TUI handles paste events with special bracket sequences (`\e[200~ ... \e[201~`), and large packet prompts (multi-KB JSON) pasted as a single buffer may fragment across the TUI's input ring, trigger paste-detection modes, or interact with provider-side clipboard heuristics. Codex TUI has the same uncertainty. Neither is tested today.
*   The agy lane in this workflow does not run in tmux at all (`--print` one-shot wraps stdin-to-argv), so buffer-paste is moot for agy.
*   The current PTY-master delivery path works. The bug we are fixing is *liveness misclassification*, not byte transport. Conflating them lands a second blocker on top of the one we are clearing.

Buffer-paste is a defensible Phase 2 / Phase 3 candidate for substrate evolution, but it deserves its own characterization run against each TUI it touches — exactly the kind of work that is wrong to bundle into a liveness-probe synthesis.

**On AGY's self-heal proposal as a near-term mitigation**:

AGY suggested the helper relaunch a fresh `tmux attach-session` immediately on attach exit. I rejected that for Phase 1 because (a) the new attach client's PTY master changes mid-stream while the FIFO is still being written, opening a byte-loss window; (b) `attach_client_pid` mutates mid-life, so every status/dashboard projection has to handle pid swap during a single supervisor lifetime; (c) if the relaunch itself fails, we still need the `detached` / `lost` distinction in the daemon — so self-heal is *additive on top of* the synthesis, not a replacement for it.

Self-heal can layer onto Phase 1 trivially once `detached` exists (the helper just probes-then-relaunch on attach exit and only emits `attach_client_exited` if relaunch fails). The synthesis ships the necessary state machine; the auto-relaunch optimisation is a follow-up patch that does not require a new RFC.

**Concession**:

For very-long-lived lanes (operator runs lasting hours), the `C-b d` window is a real operational sharp edge during the gap between Phase 1 and Phase 2 landing. The implementer should add an explicit `docs/how-to/daemon-runbook.md` note: until Phase 2 ships, operators monitoring a tmux-backed lane via `tmux attach-session` should detach using a non-killing path or simply leave the terminal open; if a detach happens, the lane needs supervisor restart to receive further packets. This is documented friction, not silent breakage — and it is strictly less friction than today, where the same event ends the supervisor.

The Phase 1 slice is "operator-attachable lanes that do not silently die on attach exit". It is not "fully self-healing reattach". The RFC's three-phase plan exists precisely so the smaller, higher-confidence change ships first.

---

## Turn 2: Question
**Author:** reviewer-gemini-3.5-flash-high-001 (AGY Reviewer)
**Message ID:** `msg_82561937fe0f2a59c92e246ee95839ce`
**Timestamp:** `2026-05-28T15:26:27Z`

In section 8 (Failure classes), the synthesis introduces `tmux_unavailable` when the `tmux` executable is missing or times out (with a 2-second timeout), and a derived state `tmux_unavailable_persistent` which marks the supervisor lost after 3 consecutive tick failures (~15 seconds) in the heartbeat loop.

However, in `reconcileSupervisorForDelivery` (§5.2), a single `tmux_unavailable` probe result during `supervise.send` will immediately refuse packet delivery with `invalid_transition: tmux probe unavailable; cannot verify lane`, requiring the operator to manually fix the daemon environment.

If the daemon host experiences transient high load or a temporary tmux server hang (a well-known issue when multiple tmux sessions are queried under high concurrency), why is there no retry logic or grace period for delivery-time probes? If the daemon is healthy enough to run, but tmux is briefly slow, refusing delivery immediately on a single transient 2-second timeout creates a highly fragile operational environment where packet sends will flap and fail under load. Why is it acceptable to treat transient delivery-time timeouts as fatal, blocking errors rather than retrying?

---

## Turn 3: Answer
**Author:** synthesizer-claude-opus-4.7-001 (Claude Synthesizer)
**Message ID:** `msg_b885699268f6e83af52a927bee559137`
**Timestamp:** `2026-05-28T15:27:40Z`

The reviewer's framing of "fatal, blocking error" is wrong, but the underlying concern about flapping under transient tmux slowness is legitimate and the synthesis should adopt a refinement. Two separate things:

**(1) Why delivery-time `tmux_unavailable` is structurally not "fatal"**

The synthesis is explicit that `invalid_transition` at delivery time does NOT mark the supervisor `lost` and does NOT emit `supervisor.lost`. It is a *deferral* response: "I cannot verify; the lane state is unchanged". From the caller's perspective this is a retryable error class. The work-claim/send loop in the daemon already classifies `invalid_transition` as retryable with backoff for the same reason it classifies `lease_stale` retryable: the underlying state machine isn't ratcheted forward.

So "the operator must manually fix the daemon environment" is true only for *persistent* unavailability — exactly the case where intervention is correct. For a 2-second tmux hiccup followed by recovery, the caller's next retry will succeed and the packet lands. Nothing about the lane is permanently broken.

The heartbeat/delivery asymmetry is intentional:
*   **Heartbeat** is a daemon-internal polling loop with no external caller deadline. Tolerating 3 consecutive misses (~15 s) costs nothing and avoids flapping the row state on a single hiccup.
*   **Delivery** is request/response. The caller already owns the time budget (its own retry policy and deadline). Pushing the retry policy down into the handler means holding leases for up to N×2s on every transient tmux miss, which causes head-of-line blocking for unrelated mutations on the same daemon.

Returning quickly with a retryable class is the cleaner separation: the handler answers fast, the caller decides cadence.

**(2) Correctness tradeoff (why pessimistic is the safer default)**

When the probe is unavailable we *don't know* whether the lane is live. The two options have asymmetric blast radii:
*   **Optimistic (deliver anyway)**: writes packet bytes to a possibly-dead PTY/pane. The bytes silently disappear or land at a stale provider TUI in some intermediate state. This is exactly the failure class RFC 0089 P1 exists to fix.
*   **Pessimistic (deferral)**: nothing is written; the caller retries; the worst case is a few hundred ms of latency.

A pessimistic deferral on an unverifiable lane is the canonical safe default for a state machine that owns delivery to an external substrate. The reviewer's concern is right that the *granularity* of the deferral can be too coarse; the fix is to soften the deferral, not to invert the default.

**(3) Concession: the synthesis should adopt stale-while-revalidate**

The defensible refinement the reviewer is implicitly asking for is bounded cache reuse. The implementer should add this to §5.2 of the synthesis as a Phase-1 detail:

*   The pointer row gains a `tmux_last_ok_at` timestamp, written by the heartbeat loop whenever `ProbeTmuxLiveness` returns `TmuxLivenessOK`.
*   On `tmux_unavailable` at delivery time, `reconcileSupervisorForDelivery` checks `(now - tmux_last_ok_at) < tmux_probe_cache_max_age` (default 10 s, override via `STRIATUM_TMUX_PROBE_CACHE_MAX_AGE`).
*   If within the cache window, the delivery proceeds and the event payload carries `liveness_cache_age_ms` so the operator can see the probe was bypassed.
*   If outside the cache window, the deferral fires as today.

This bounds the "we just had a transient tmux hang" case to a no-op while preserving the safety invariant: a genuinely dead lane gets caught within `tmux_probe_cache_max_age + heartbeat_interval` (default ≤ 15 s), the same window the heartbeat already provides. It also composes naturally with the heartbeat's 3-tick threshold, because by the time stale-while-revalidate stops accepting (10 s) the heartbeat itself has either recorded a fresh `tmux_ok` (resetting the window) or has accumulated 2 of its 3 unavailable ticks (already escalating internally).

The tests need one addition: `TestReconcileForDeliveryAcceptsStaleProbeWithinCacheWindow` and its negative counterpart `TestReconcileForDeliveryRefusesOnExpiredCache`. The metadata allowlist (§10.8) needs `tmux_last_ok_at` and the response field `liveness_cache_age_ms` added.

**(4) On the 2-second timeout itself**

`STRIATUM_TMUX_PROBE_TIMEOUT` is already in the synthesis (§4.3) as the tuning knob. The runbook section in §12 should explicitly recommend tuning it on hosts with known tmux baseline slowness, and document that the default targets a healthy server (`tmux has-session` typical ~5–20ms). Combined with stale-while-revalidate, a tuned timeout effectively eliminates the transient-flap class without weakening the unavailable-detection class.

**Why I am not retreating to "retry inside the handler"**

A naive in-handler retry (e.g., 3 attempts × 2 s) makes the handler unbounded under sustained tmux pressure, which is the worst possible behaviour: the daemon serializes on a slow tmux server and every other supervise.send queues behind it. The correct pattern when an external dependency is slow is "answer fast, let the caller retry with its own cadence" — exactly what `invalid_transition` is designed for. Stale-while-revalidate is the layer that turns transient slowness into a no-op without introducing handler-side blocking; in-handler retry would introduce handler-side blocking without solving anything stale-while-revalidate doesn't already solve.

**Net change to the synthesis**

§5.2 (reconcileSupervisorForDelivery), §8 (failure classes table — add `tmux_unavailable_serve_from_cache` as a non-error sub-state), §10.5 (tests), §10.8 (D028 allowlist), §12 (daemon-runbook section). No new RPC, no schema migration; `tmux_last_ok_at` is just another key in `metadata_json`.

---

## Turn 4: Question
**Author:** reviewer-gemini-3.5-flash-high-001 (AGY Reviewer)
**Message ID:** `msg_1e2e10973687f7123585744ec249beb5`
**Timestamp:** `2026-05-28T15:27:50Z`

On platforms where both `pane_start_time` (tmux < 2.9) and `processStartToken(pane_pid)` (due to missing `/proc` or container isolation) are unavailable, the captured token becomes `""`, classified as `start_token_unverified`. In this situation, the probe still succeeds (`TmuxLivenessOK`), but the start token is never verified. This effectively disables the pid-re-use/start-time mismatch safety invariant (D080) for tmux-backed lanes on these platforms.

Under high concurrency or high churn, what prevents a dead pane process pid from being reused by another system process, and Striatum misclassifying it as `TmuxLivenessOK` because the start token is unverified? If we allow `start_token_unverified` to be treated as healthy, aren't we silently dropping the D080 safety guarantee in these environments? Why is this acceptable rather than falling back to a degraded state or prompting the operator?

---

## Turn 5: Answer
**Author:** synthesizer-claude-opus-4.7-001 (Claude Synthesizer)
**Message ID:** `msg_72e2458900e91143711e13d47f5523bb`
**Timestamp:** `2026-05-28T15:29:18Z`

The reviewer is right that this needs an explicit answer, but the framing "silently dropping the D080 safety guarantee" is incorrect for tmux-backed lanes specifically. D080 is about defending against pid identity collision; for tmux-backed lanes that defence is provided by an *independent and stronger* invariant that the synthesis underspecified. Let me walk through the attack and show what actually protects against it, then concede the visibility improvement the reviewer is implicitly asking for.

**(1) The actual pid-reuse attack on a tmux-backed lane**

For a misclassification to occur under the empty-start-token case, all of the following must hold simultaneously:

1.  The pane process (captured pid `P_old`) exits.
2.  The pane (captured `%4`) is destroyed by tmux.
3.  A *new* pane is created in the same tmux session.
4.  The new pane is assigned the same `pane_id` `%4`.
5.  The new pane runs a different command.
6.  That command's process happens to land on the same pid `P_old`.
7.  Our captured `pane_start_token` was empty so we cannot disambiguate.

Step 4 is the load-bearing assumption, and **it does not hold in tmux**. Tmux pane IDs (`%N`) are server-monotonic: the counter increments for every pane creation and is never decremented or reused, even after the pane is destroyed. From tmux's source (`server.c`/`window.c`), the `pane_id` counter only resets on server restart. So `%4` observed at probe time IS the same `%4` we captured at launch — there is no slot reuse to defeat.

This is operationally stronger than the pid+start-token check D080 was built to provide for plain-PTY lanes. For tmux-backed lanes, the synthesis's `pane_id` equality check in §4.1 step 2 is the D080 invariant: if `pane_id` matches, you are looking at the same pane, full stop. The pane_pid and start-token checks are belt-and-suspenders defence layered on top.

**(2) Tmux server restart case**

The reviewer's worry is most plausible if the tmux server itself restarts (which DOES reset the pane_id counter, potentially making `%4` available again). But in that scenario:
*   `tmux has-session -t <captured_session_name>` fails on the new server → `TmuxLivenessSessionMissing`, not `TmuxLivenessOK`.
*   Even if an operator manually recreates a session with the captured name (which embeds `run_id`/`lane_id`/`sup_id`, so this requires deliberate forgery), the new session starts at pane_id `%1` (or similar), not `%4`, so the pane_id check still fails.

There is no realistic path through which a tmux server restart yields `TmuxLivenessOK` against stale identity.

**(3) What the empty start_token actually changes**

Empty start_token defeats one specific attack: a hostile or buggy environment where tmux's pane_id counter is non-monotonic (e.g., a forked tmux fork that resets pane_ids). That's neither D080's threat model nor RFC 0089 P1's. For the realistic environments the reviewer cites — tmux < 2.9 on Linux, or containers with /proc unmounted — pane_id monotonicity holds and the probe is safe.

The legitimate weakness empty start_token introduces is *observability*: the operator of a tmux-2.8 or no-/proc deployment has no top-level signal that their lane is being attested without the start-token cross-check. Today's read projection only buries the detail in `tmux.liveness.detail`.

**(4) Concession: synthesis should add an explicit `attestation_caveat` field**

The synthesis (§3.3) already records "empty captured token ⇒ detail `start_token_unverified` ⇒ probe is OK". That detail rides in `tmux.liveness.detail` (§5.3). The implementer should additionally:

1.  **Top-level surface**: extend `supervise.status` and `dashboard` so when `tmux.liveness.detail == "start_token_unverified"`, the projection includes a top-level `attestation_caveat: "start_token_unverified"` field. `lane_attestation` stays `attested` (byline rules unchanged), but the caveat is visible without grepping into the nested liveness block.
2.  **Launch-time event**: `supervisor.started` payload includes `start_token_source` ∈ `{"tmux_pane_start_time", "process_start_token", "unverified"}`. Operators can audit on the first event whether their fleet is running with verified or unverified tokens.
3.  **Doctor problem string**: `doctor --verbose` adds a static problem string `tmux supervisor running with unverified start token (tmux < 2.9 or missing /proc)` so the degradation is part of routine health audit, not a buried JSON field.
4.  **Runbook section**: `docs/how-to/daemon-runbook.md` adds a paragraph explaining that on tmux < 2.9 or no-/proc hosts, the start-token cross-check is unavailable; the pane_id monotonicity invariant carries the safety property; operators who consider that risk model unacceptable should upgrade tmux or mount /proc.

Tests to add:

*   `TestProbePaneIDMonotonicityIsSufficientWhenStartTokenEmpty` — fake runner returns matching `pane_id` and `pane_pid` with empty observed `pane_start_time`, captured token empty. Assert `TmuxLivenessOK` with detail `start_token_unverified`.
*   `TestProbeRejectsPaneIDMismatchEvenWithMatchingPID` — fake runner returns `pane_pid` matching but `pane_id` different. Assert `TmuxLivenessPaneMissing` (verifies pane_id is the load-bearing check).
*   `TestStatusExposesAttestationCaveatWhenStartTokenUnverified` — pointer metadata `tmux.pane_start_token = ""`; status projection includes `attestation_caveat: "start_token_unverified"` and `lane_attestation: "attested"`.

The D028 allowlist (§10.8) gains `attestation_caveat` (top-level) and `start_token_source` (event payload).

**(5) Why not refuse the launch on empty start_token?**

The reviewer's alternative — "fall back to a degraded state or prompt the operator" at launch time — is overcorrection. It would:

*   Break tmux < 2.9 hosts entirely until manual operator intervention. That tmux version is still default on some long-LTS distros and on hardened-base container images.
*   Break any future no-/proc deployment shape (e.g., gVisor sandboxes) without a substantive safety benefit, because pane_id monotonicity already covers the attack.
*   Conflict with the synthesis's stated rollback property (§9.3): "no schema migration; code rollback is clean". Adding launch-time refusal predicated on start_token availability adds an environment-coupling we currently don't have.

Making the degradation *visible* at top-level is the right tradeoff: operators who care can audit and remediate (upgrade tmux, mount /proc); operators on healthy modern hosts see nothing. The safety invariant is preserved by pane_id monotonicity; the visibility property is upgraded so legacy hosts cannot run silently degraded.

**(6) Net change to the synthesis**

§3.3 (explicitly document pane_id monotonicity as the D080-equivalent invariant for tmux-backed lanes), §4.1 (note that pane_id equality is the structural load-bearing check), §5.3 (add `attestation_caveat` top-level field), §10.1/§10.6 (new tests above), §10.8 (allowlist updates), §12 (runbook section + decision-log note that the D080 invariant for tmux-backed lanes is realised via pane_id monotonicity, not start-token equality). No new RPC, no schema migration.
