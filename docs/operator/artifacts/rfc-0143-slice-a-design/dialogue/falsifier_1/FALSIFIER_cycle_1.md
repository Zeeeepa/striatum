# FALSIFIER - RFC 0143 Slice A decoupling re-attack

author: falsifier-reviewer-003

## Verdict

**needs_revision.** The holder's Slice-A proposal is clean on the major decoupling boundaries I was asked to probe: it does not reintroduce W1, `PaneKernelStartToken`, a reseal-token file, `CapabilityReseal`, `resealInFlightJob`, owner bundle 0021, or reserved code 98. I also credit Spot 2's attribution rule as the right no-over-fire shape: the typed recovery class fires only when the daemon observes wrapper exit code `97` from durable/process state, not from ambiguous "complete-on-disk + lane-lost" alone.

The design still does not clear. The only concrete producer of exit `97` is the lane-side resolver sentinel, and the normal supervised boot-epoch-rotation path can be locked out without ever reaching that sentinel or being allowed to emit the reserved code. That is an under-fire on the exact #512 failure Slice A exists to make legible. The fix is not Slice B and not token widening; it is a missing reachable Slice-A trigger for a session-bound lane carrying stale endpoint/boot-epoch state.

This is not `reject`: I found no path that lets a lane read the admin runtime `client-token`, no minted credential carrying `{admin, apply, recovery, surgical_recovery}`, and no hidden Slice-B artifact. It is not `accept_with_findings`: the missing trigger is on the central game-day path, so the build contract would still allow a rotation lock-out to die as ordinary `agent_exited_unsealed`, `agent_pid_dead`, or a raw stale-MCP explanation.

## Challenge: The Normal Rotation Lock-Out Does Not Reliably Emit `97`

### Precise Claim Attacked

The holder claims Slice A is a pure daemon-side/process-state floor:

- Spot 1 narrows credential resolution by refusing the owner-only runtime `client-token` for a non-owner lane and returning `ErrUnrecoverableAcrossRotation` before any read.
- The agent-loop maps that sentinel, and only that sentinel, to `ExitUnrecoverableAcrossRotation = 97`.
- Spot 2 classifies `session_unrecoverable_across_rotation` only when it observes exit `97` from `agent_exited.exit_code` or tmux `#{pane_dead_status}`.
- Ordinary complete-on-disk + lane-lost cases with no reserved code stay in the existing classes.

That is a sound attribution rule for cases that actually produce `97`. The flaw is upstream: the ordinary post-restart lane path often produces no `97` at all.

### Concrete Refutation

A normal supervised lane launches with a session-bound bearer, not the runtime admin token. Current source makes that load-bearing:

- `supervisedEnvEntries` injects the lane's session-bound token as `STRIATUM_MCP_TOKEN` (`go/pkg/mutations/supervision_env.go:333-343`).
- `ResolveTokenMaterial` consults `STRIATUM_MCP_TOKEN` first and returns `Source: EnvMCPToken` (`go/pkg/agentloop/token.go:18-21`).
- `Run` stores that material in `cfg.Token` before the adapter starts (`go/pkg/agentloop/loop.go:37-56`).

So for the ordinary lane, the holder's proposed rotation guard `cfg.Token.Source == <runtime client-token>` is false. Even if `ResolveTokenMaterialFresh` later reached the proposed sentinel, the holder says to keep the launch token and not request the unrecoverable exit. The wrapper therefore emits no `97`.

There is an earlier failure mode too: the rotation watcher may never reach token refresh. The watcher first calls `ResolveMCPEndpointFresh` (`go/pkg/agentloop/loop.go:589-604`). The daemon publishes the fresh endpoint and boot epoch with `writeOwnerOnlyTextFile`: owner-only runtime dir plus `0600` files (`go/cmd/striatumd/main.go:632-640`, `:752-763`, `:798-815`). A `striatum-lane` process is the non-owner on the #512 path. If it gets `EACCES` or `EPERM` reading those freshness files, the watcher treats that as "nothing to compare against yet" and continues (`loop.go:589-593`). It does not ask the token resolver, so the sentinel cannot be produced.

The holder's strongest implied rebuttal is that a lane holding the session-bound bearer is still recoverable, so suppressing `97` avoids over-fire. That premise is false after the boot-epoch identity change:

- the supervisor also injects `STRIATUM_MCP_BOOT_EPOCH` (`go/pkg/mutations/supervision_env.go:344-354`);
- codex, agy, and claude echo it as `X-Striatum-Boot-Epoch` (`go/pkg/agentloop/mcpconfig.go:123-130`, `:217-222`, `:490-496`);
- the MCP HTTP handler rejects a stale presented epoch as `stale_daemon_identity` before dispatch (`go/pkg/mcp/http.go:681-699`);
- codex cannot reload its launch-time `-c` MCP URL, and the current loop only writes an in-PTY wedge prompt then returns nil (`go/pkg/agentloop/loop.go:625-645`).

A lane can therefore still possess a valid session bearer while being pointed at a dead endpoint or presenting a stale boot epoch the new daemon refuses. It cannot complete through MCP, but it also never emits the reserved process code. Since Spot 2 is exact-code-only, the daemon has no durable/process-state basis to record `session_unrecoverable_across_rotation`.

This is not a hidden dependency on W1 or any Slice-B artifact. It is the opposite: after deleting Slice B, Slice A still needs one exact, reachable, non-over-firing producer of the same reserved code Spot 2 already knows how to observe.

### Boot-Epoch Gap

The holder correctly avoids assuming a durable per-lease or per-job boot-epoch record. Current source confirms `daemonBootEpoch()` is a per-process in-memory value (`go/cmd/striatumd/main.go:720-738`) and the published epoch file is owner-only (`:752-763`). I am not asking for a durable DB epoch predicate.

The gap is that the design never replaces that missing durable predicate with an executable rotation-path signal. "No durable epoch record" is acceptable only if the lane can still produce an exact observable floor when it proves the launch endpoint/epoch is stale or when the daemon returns `stale_daemon_identity`. As written, the session-token lane can be locked out and still exit without `97`.

### Spot 2 Attribution

I credit the holder's Spot 2 attribution. It should not infer the typed floor from complete-on-disk plus lane-lost alone; that would over-fire on ordinary unsealed exits. The proposed exact-code gate is the right boundary.

The problem is under-attribution, not over-attribution. Because the normal rotation path lacks a reachable code producer, the exact-code rule will correctly refuse to fire, and the run will record the wrong ordinary class for the covered failure.

### Strongest Rebuttal and Why It Fails

The best holder defense is that the startup resolver refusal covers the original #512 CLI fallback: if a non-owner lane falls through to the owner-only admin token, it gets the sentinel instead of a permission error. I agree that this narrows the credential chain and is worth keeping.

But #512 is not only a startup fallback problem. The current supervised lane carries launch-time endpoint and boot-epoch identity into its adapter config. A daemon restart rotates those values. A session-bound bearer alone does not prove the adapter can talk to the new daemon, and owner-only freshness files can keep the watcher from detecting or repairing the mismatch. The promised floor must cover that normal lock-out shape, not just the narrower "chain reached runtime client-token" shape.

A second defense is that mapping `stale_daemon_identity` from a lane MCP response might look like an inbound frame. It is not an inbound authenticated frame to the daemon. The lane would be interpreting its own client error and exiting with the reserved process status; the daemon would still observe only a process exit code from durable/process state. That remains within the D261 decoupling premise.

## Required Revision

Keep the credited pieces unchanged: the refuse-before-read narrowing, no admin-token widening, no Slice-B artifacts, the reserved code, the exact-code Spot 2 classification, the launch-handshake raw-error dissolution, the #292 relationship, C2 forge-resistance, and the ordinary-unsealed negative.

Add a reachable, non-over-firing rotation-path trigger for a normal session-bound supervised lane. Any of these shapes can satisfy the lens if specified precisely:

- make the non-secret endpoint/boot-epoch freshness signal lane-readable, without exposing any bearer, and exit `97` when the lane proves it is carrying a stale launch epoch or dead endpoint;
- map the daemon's `stale_daemon_identity` response observed on the lane's own MCP client path to `ErrUnrecoverableAcrossRotation` -> exit `97`, with a negative that ordinary non-epoch network/MCP errors do not fire the floor;
- or prove and test an in-place reconnect path that updates endpoint and boot-epoch per adapter, reserving `97` for the remaining exact unrecoverable cases.

The codex wedge path must route the typed floor or a proven reconnect, not only write an in-PTY prompt and continue.

Required tests:

- `TestSessionBoundLaunchTokenDoesNotSuppressStaleEpochFloor`: launch token source is `EnvMCPToken`; after rotation the lane has stale endpoint/epoch state; assert wrapper exits `97` and recovery records `session_unrecoverable_across_rotation`.
- `TestRotationWatcherUnrecoverableWhenFreshEndpointUnreadableForLane` or `TestUnreadableFreshEndpointDoesNotSilentlyContinueWithoutTypedFloor`: non-owner lane cannot read fresh endpoint/epoch files; assert it does not silently continue and die generic.
- `TestCodexRotatedEndpointWedgeRecordsTypedUnrecoverable`: codex cannot reload its launch-time `-c` URL; assert the terminal state is the typed floor or a proven reconnect path.
- Keep `TestOrdinaryUnsealedExitStaysAgentExitedUnsealed` and `TestLaunchHandshakeFailureStaysHelperErrorNotFloor` so the new trigger does not over-fire.

## Checks Credited

- No hidden Slice-B dependency found: no W1 connect-out channel, no kernel-token capture, no reseal-token file, no `CapabilityReseal`, no `resealInFlightJob`, no owner bundle 0021, no code 98.
- No admin-token widening found: the proposed Spot 1 path refuses before any non-owner read, and the owner path is stated as unaffected.
- Spot 2's exact-code-only routing is the right daemon-side attribution rule and avoids the complete-on-disk-plus-lane-lost over-fire.
- The v7 `BC1-W1-CAPTURE-FLOOR` launch-boundary issue is dissolved for this decoupled Slice-A world: with no launch W1 capture, a genuine launch failure can remain `helper_error`. The remaining legibility failure has moved to the credential/rotation path.

## Bottom Line

The holder has a decoupled and non-widening floor observer, but not a reachable floor producer for the ordinary boot-epoch rotation lock-out. Until the spec wires a session-bound lane with stale endpoint/boot-epoch state to emit `ExitUnrecoverableAcrossRotation = 97` without widening credentials or over-firing on ordinary failures, this revision must not clear.