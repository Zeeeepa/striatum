# Interrogation Chat Log - Design Review Adversarial

- **Interrogation ID**: `intg_b91260c238a8cde62411458d50911834`
- **Target Session ID**: `sess_74b850037f6bedd94bc104c1c70ca6a9`
- **Interrogator Session ID**: `sess_4f3e09b8cbee4afdaf35b6c89d3e785e`
- **Posture**: `devils_advocate`

---

## Turn 0 (Test/Bootstrap Round)

### Question (Interrogator)
Hello, this is a test question.

### Answer (Synthesizer)
Acknowledged. Synthesizer is live and ready for adversarial questions about the rerun2 design synthesis. Summary of the load-bearing choices for ease of probing: (1) byte delivery stays on the attach PTY in Phase 1; send-keys decoupling deferred for TUI-compat reasons. (2) Central fix is daemon-side recordSuperviseReportEvent guarded by ProbeLaneLiveness so attach-bridge exit with a live pane no longer transitions state=detached. (3) Identity remains pane_pid + pane_start_time; helper.go child-exit branch consults the probe before choosing attach_client_exited vs agent_exited. (4) remain-on-exit on is added to distinguish tmux_pane_dead from tmux_session_missing. (5) AttachPID and attach_client_pid kept as optional diagnostics; no DDL.

---

## Turn 1 (Round 1: Helper Termination vs Byte Delivery)

### Question (Interrogator)
Under the Phase 1 synthesis, the byte delivery channel still rides the helper's internal attach PTY. The synthesis specifies that when the helper's attach client exits, 'RunHelper' will 'emit HelperEventAttachExited with payload {attach_pid, exit_code, observed_at, tmux_liveness: live.Class} and return nil without tearing down the supervisor.' However, if 'RunHelper' returns nil, the helper process itself terminates. If the helper process terminates, the daemon no longer has an active helper process running to receive PTY stdin or run 'forwardPacketStream'. In Phase 1, how can the supervisor deliver subsequent packet bytes to the pane after a 'graceful detach' has terminated the helper process? Doesn't this mean that any graceful detach is effectively a permanent loss of the byte-delivery bridge until a supervisor restart occurs?

### Answer (Synthesizer)
Concede the point: §4.2's "return nil without tearing down the supervisor" is hollow as written, because `RunHelper` returning terminates the helper process and so terminates byte delivery, which is exactly what the reviewer says. The synthesis is internally inconsistent on that line.

Let me distinguish two scenarios the synthesis conflated and then say what the honest Phase 1 fix has to be.

Scenario A — operator-side `tmux attach-session`. The operator runs `tmux attach-session -t <session>` from their own terminal. This is a tmux client entirely separate from the helper's `pty.Start(attachCmd)` bridge; tmux supports multiple clients per session. When the operator detaches, the helper's `result.Cmd.Wait()` does not return at all. This scenario is already observer-only today and was never the bug. The synthesis should have stated this explicitly and did not.

Scenario B — helper's own attach bridge exits. This is the actual gap. The helper's bridge can exit for transient reasons (PTY EIO, signal, OOM-kill on the bridge process specifically) where the tmux pane and the agent process continue running. Under today's code, the daemon marks lost. Under the synthesis as written, the daemon does not mark lost but the helper terminates and no helper exists to push the next FIFO frame into the pane. As the reviewer says, that is a permanent byte-delivery loss until a supervisor restart, which downgrades the lane just as effectively as marking lost would have.

There are three coherent ways to close this gap in Phase 1, and the synthesis must commit to one rather than gesture at "return nil":

1. **Helper self-heals the attach bridge**: On `result.Cmd.Wait()` returning while `ProbeLaneLiveness` reports `tmux_ok`, the helper closes the dead ptmx, runs `pty.Start(attachCmd2)` to get a fresh attach client and ptmx, replaces `result.Cmd`/`result.StdinWriter`/`result.AttachPID`, emits a curated `attach_client_recovered` event carrying the new pid so the daemon can update pointer metadata, and continues the packet pump. A bounded retry budget (say one immediate retry, then exponential backoff up to three tries inside a 60s window) prevents infinite loops if the attach client is wedged. If the retry budget exhausts, the helper falls through to `agent_exited` with `cause="attach_bridge_unrecoverable"` and the daemon marks lost. Cost: ~80 lines of new helper code plus one event type plus a metadata-merge path in `recordSuperviseReportEvent`.
2. **Send-keys delivery for tmux-backed lanes**: Claude's design. Helper holds no ptmx and never owns an attach client; byte delivery is `tmux send-keys -l -- <frame>; tmux send-keys C-m`. Helper exit becomes irrelevant to delivery; daemon can spawn a fresh helper at any time and reuse the existing pane. Cost: per-adapter TUI input regression testing (claude TUI bracketed paste, codex agent-loop, agy `--print` shim) that I explicitly scoped out in §12 for risk reasons. The risk is real but bounded — `send-keys -l` is literal-byte delivery and the adapters in scope all already accept stdin in their cooked-equivalent forms.
3. **Scope-honest narrowing**: Admit that Phase 1 only guarantees observer-only semantics for Scenario A (operator-attach), explicitly does not fix Scenario B, and that supervisor downgrade on helper-bridge exit is a known follow-on. The daemon-side `recordSuperviseReportEvent` guard from §4.3 still ships because it correctly handles legacy edge cases, but the helper-side §4.2 change is dropped. Phase 2 then has to choose between options 1 and 2 above.

**My recommended revision**: adopt option 1 (helper self-heal with bounded retry). It preserves the TUI input compatibility I cited as the reason to defer send-keys in §3, it closes Scenario B for real rather than rhetorically, and the engineering surface is contained to `RunHelper`, one new event type, and one branch in `recordSuperviseReportEvent`. Option 2 is structurally cleaner long-term and I think it eventually wins, but the per-adapter validation work justifies deferring it. Option 3 is intellectually honest but leaves the operator with the same downgrade behavior they have today for the specific case that motivated the RFC.

Concrete edits to the synthesis if option 1 is adopted: §4.2 replaces "return nil without tearing down" with the self-heal loop described above; a new `HelperEventAttachRecovered` constant is added; §4.3 gains a `recordSuperviseReportEvent` branch for `attach_client_recovered` that updates pointer metadata's `attach_client_pid` and appends a curated daemon event; §7's table gets a new row "Helper attach-bridge exit (probe OK) → helper self-heals up to N tries, then agent_exited"; §9.2 adds `TestHelperSelfHealsAttachBridgeOnTransientExit` and `TestHelperFallsThroughToAgentExitedAfterRetryBudget`; §12 keeps send-keys delivery deferred but adds "helper bridge self-heal retry tuning" as a follow-on observability concern.

The reviewer is right that the synthesis as published cannot deliver what its summary claims. I would revise to option 1 and welcome a follow-up question on whether the bounded-retry semantics are correct, or on whether the metadata-update event flow is safe under concurrent `supervise.send` calls.

---

## Turn 2 (Round 2: Helper Self-Heal Concurrency and Atomicity)

### Question (Interrogator)
Under the proposed Option 1 (helper self-heal with bounded retry), there is a critical concurrency window. While the helper is self-healing—meaning it is closing the dead ptmx, launching a new attach client via 'pty.Start', and replacing 'result.StdinWriter' and 'result.AttachPID'—subsequent 'supervise.send' requests might be actively dispatched from the daemon. 1. During this self-heal transition window, is there a thread-safety or lock-acquisition guard to prevent the packet pump ('forwardPacketStream') from attempting to write to a nil, closed, or partially replaced 'StdinWriter'? If not, doesn't this risk a Go runtime panic or a silent packet drop? 2. Since the helper updates the database pointer metadata asynchronously via 'HelperEventAttachRecovered' in 'recordSuperviseReportEvent', a race condition exists where 'supervise.send' could read a stale, dead 'AttachPID' or write to a dead stdin pipe descriptor before the new state is persisted. How does the supervisor ensure atomicity of the self-heal swap relative to concurrent packet delivery?

### Answer (Synthesizer)
Both concurrency hazards are real and the synthesis as it stands does not call them out. They are both fixable inside the helper without daemon changes, but the design must be explicit. Let me address each separately because they have different shapes.

#### Question 1 — in-process race between `forwardPacketStream` and the self-heal swap.

The current helper structure runs `forwardPacketStream`, `pumpPTYProgress`, and `result.Cmd.Wait()` as three concurrent goroutines coordinated through a `select` loop on `packetDone`/`childDone`. `forwardPacketStream` captures `result.StdinWriter` by value when it is spawned, so a naive in-place replacement of `result.StdinWriter` does nothing — the packet pump keeps writing to the same closed ptmx and errors out, which terminates the helper exactly as the reviewer fears.

The fix is to introduce an indirection layer that the packet pump writes through, not the raw ptmx. Concretely, a `bridgedWriter` type held by the helper:

```go
type bridgedWriter struct {
    mu      sync.RWMutex
    current io.WriteCloser
}
func (b *bridgedWriter) Write(p []byte) (int, error) {
    b.mu.RLock()
    defer b.mu.RUnlock()
    if b.current == nil { return 0, errBridgeUninitialized }
    return b.current.Write(p)
}
func (b *bridgedWriter) Swap(next io.WriteCloser) {
    b.mu.Lock()
    defer b.mu.Unlock()
    if b.current != nil { _ = b.current.Close() }
    b.current = next
}
```

`forwardPacketStream` is changed to take `*bridgedWriter` instead of `io.Writer`. The packet pump's `Write` call takes the read lock for the duration of one frame; `Swap` takes the write lock. While self-heal is in progress, the packet pump blocks on the write lock. Once the swap completes (new attach client started, new ptmx installed), the packet pump unblocks and writes the next frame to the fresh ptmx. From the packet pump's perspective the swap is atomic. From the daemon's perspective the FIFO writer briefly back-pressures (kernel pipe buffer absorbs typical packet sizes; if it fills, the daemon's `supervise.send` blocks on the FIFO write until the helper drains it, which is the correct semantic).

`pumpPTYProgress` needs the same treatment but is simpler because progress is read-only and idempotent: just tear down the old goroutine before closing the old ptmx, then spawn a fresh `pumpPTYProgress` goroutine over the new ptmx after `Swap`. No shared state.

The select loop changes so the `childDone` case for "probe ok" calls a `selfHeal()` helper that performs the sequence atomically from the loop's perspective: drain progress, run `pty.Start(attachCmd2)` with the original spec, `bridgedWriter.Swap(newPtmx)`, install new `childDone` watcher for the new `result.Cmd`, spawn fresh `progressDone`. If `pty.Start` fails, fall through to `agent_exited`. Retry budget is enforced via a counter on the helper struct.

So the in-process race resolves to: one mutex, one indirection wrapper, one helper function. The race is genuine but its surface is small enough to be reviewable.

#### Question 2 — daemon-side staleness of `attach_client_pid` relative to in-flight `supervise.send` calls.

This one I want to push back on, because I think the concern conflates two separate metadata uses:

`supervise.send` does not consult `attach_client_pid` to decide where to write packet bytes. The bytes go to the FIFO at `stdin_pipe_path`, which is stable across the entire supervisor lifetime regardless of how many times the attach bridge is replaced. The helper consumes from the FIFO and writes to whatever its current ptmx is (via the `bridgedWriter` above). The attach client pid is metadata for observability only — it appears in `supervise.status` JSON, in `doctor`, and in archive provenance. Nothing in the delivery path branches on it.

The probe consulted by `reconcileSupervisorForDelivery` is `ProbeLaneLiveness`, which reads `session_name`, `pane_id`, `pane_pid`, and `pane_start_token` from pointer metadata. None of those change during a self-heal — the pane itself is unchanged; only the attach bridge in front of it is replaced. So `reconcileSupervisorForDelivery` returns `tmux_ok` throughout the self-heal window. A concurrent `supervise.send` call that arrives mid-self-heal hits the FIFO, blocks on the kernel pipe buffer until the helper finishes the swap, then proceeds. No stale read can cause a wrong delivery decision because the only state that matters (pane identity) is stable.

The one staleness window that does exist is in `supervise.status` JSON: between the moment the old attach client exits and the moment `recordSuperviseReportEvent` merges the new `attach_client_pid` into pointer metadata via `HelperEventAttachRecovered`, a status call returns the old pid. That window is bounded by helper-to-daemon event flush latency (the helper events file is the standard JSONL path the daemon already consumes; flush is sub-second in practice). For diagnostic metadata that is the correct ordering — the reader either sees pre-recovery state (with `attach_client_pid` matching the dead pid plus a `tmux.liveness.class=tmux_ok` derived projection that already tells the operator something is in flux) or post-recovery state. No torn read, no field that disagrees with the actual pane.

If the reviewer is worried about a particular subscriber that does branch on `attach_client_pid` for a delivery-relevant decision, I would want to be pointed at the call site, because I am not aware of one and the synthesis would need to either remove that branch or order the metadata update before the swap (which is feasible: the helper emits `HelperEventAttachRecovered` *before* calling `Swap`, blocking the swap on the daemon's ack via the existing helper-events flush; cost is one extra round-trip per self-heal, which happens at most a handful of times per lane lifetime).

Net: question 1 needs a real code change (bridgedWriter + mutex + selfHeal()). Question 2 I believe is a non-issue in the delivery path and a bounded soft staleness in the diagnostic path; if there is a specific subscriber I'm missing, I'd revise.

---

## Turn 3 (Round 3: Security of Attestation Under Fallbacks & Rollbacks)

### Question (Interrogator)
Under §11, the synthesis describes the 'STRIATUM_TMUX_PROBE_DISABLE=1' rollback escape hatch, which falls back to PID-based liveness using the recorded pane pid. However, under D080/D149, lane-byline attestation is strictly bound to process identity to prevent hostile or recycled environment takeover. If the tmux probe is disabled, signal-0 pid checking makes pid recycling invisible. 1. If 'STRIATUM_TMUX_PROBE_DISABLE=1' is active, does the supervisor still grant the full lane/model byline attestation, or is it degraded to 'author: operator'? If it keeps the byline, doesn't this create a severe bypass where a compromised environment can set this env var to hijack attestation on a recycled PID? 2. If the byline is downgraded, how does the status/read projection communicate this degradation to the operator, since they might assume the lane is still fully attested?

### Answer (Synthesizer)
Good catch — the synthesis underspecified the fallback semantics in a way that does have a security shape, even though the bypass surface is narrower than the question implies. Let me answer each part and then specify what the synthesis needs to add to be safe.

First, the threat model. `STRIATUM_TMUX_PROBE_DISABLE` is read in the daemon process, not in lane subprocesses. The probe runs server-side; lane bytes never reach the probe-decision code. So a lane that sets this env var in its own subprocess environment cannot affect daemon-side liveness checks. The privilege boundary the env var lives behind is daemon-process control. An attacker who can set env vars in the daemon's process environment already owns the daemon and can do strictly more damage than faking one supervisor's byline. That is not a useful attack surface to defend against here — same reason we do not defend against an attacker who can write to PostgreSQL directly.

The real question is: assuming a legitimate operator turned the probe off for triage, does the fallback path still defend against pid recycling on the recorded pane pid? The synthesis as written said "PID-based liveness using the recorded pane pid", which is ambiguous and the reviewer is right to flag it. The intended semantic — and what the existing plain-PTY path already implements — is signal(0) **plus** start-token comparison via `PIDLiveWithStartToken`. Start token is `pane_start_time` captured at launch and compared against the live process's start time read from `/proc/<pid>/stat`. PID recycling reuses the pid but does not match the original start time, so the comparison fails and the supervisor transitions to `pid_identity_mismatch`. That class is terminal and triggers lost just like the tmux failure classes do.

Concretely on the two sub-questions:

1. **Attestation Integrity**: With probe disabled, the supervisor keeps the lane/model byline attestation iff the fallback runs signal(0) + start-token comparison and the start-token is present in metadata. PID recycling is detected via start-token mismatch and the lane drops out of attested before anything is bylined post-recycling. The synthesis must spell this out in §11 — current wording lets a reader interpret it as "skip the probe, signal(0) alone", which would indeed open the bypass the reviewer described. I would revise §11 to say verbatim: "Probe disable falls through to the existing `PIDLiveWithStartToken` path, not bare `signal(0)`. The recorded `pane_start_token` continues to gate attestation; pid recycling triggers `pid_identity_mismatch` and the lane transitions to lost." I would also harden the launch path to refuse `tmux.state="backed"` metadata if `pane_start_token` is empty, so probe disable cannot land a lane in a state where start-token comparison is silently a no-op. Today the start-token capture is best-effort with an empty fallback; for probe-disable safety it must be mandatory on backed launches (failing capture demotes to `launchPlainPTY` or fails closed under `RequireTmux`).
2. **Operator Visibility**: On operator visibility of the degradation, you are right that the existing read projections do not loudly distinguish "tmux probe disabled" from "tmux not in use". I would add a discriminator field in the read projections: when `STRIATUM_TMUX_PROBE_DISABLE` is active and the launch metadata says `tmux.state="backed"`, the derived liveness sub-object should carry `tmux.liveness.probe_disabled=true` and the lane attestation reason should append `tmux_probe_disabled` as a warning even when the class is OK. `doctor --verbose` should treat this as a problem-level finding rather than a non-problem, with remediation hint "unset STRIATUM_TMUX_PROBE_DISABLE; re-run the probe; investigate the underlying tmux flake". This is the same vocabulary I already specified for `tmux_unavailable` in §7, just promoted from warning to problem because the operator explicitly chose to disable a security-relevant check.

One adjacent concern the reviewer didn't ask about but is worth volunteering: `STRIATUM_TMUX_PROBE_DISABLE` being a stringy env-var is a fragile control surface for something with security shape. A future iteration should make this a typed daemon-config knob with a default-deny posture and an audit event on each activation (`daemon.tmux_probe_disabled` event with the operator identity who toggled it). That tightening doesn't have to land in Phase 1, but the synthesis should name it as a known follow-on so the env-var path doesn't ossify into the production default.
