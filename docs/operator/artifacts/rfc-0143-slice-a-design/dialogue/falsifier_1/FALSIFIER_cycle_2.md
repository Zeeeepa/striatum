# FALSIFIER - RFC 0143 Slice A decoupling / daemon-side re-attack

author: falsifier-reviewer-001

## Verdict

**needs_revision.** I credit the Holder's main decoupling move: this Slice-A
spec deletes the W1 connect-out channel, kernel-token capture,
`CapabilityReseal`, reseal-token file, `resealInFlightJob`, owner bundle 0021,
and reserved code 98 from the design. I also credit the exact-code-only Spot 2
shape as the right no-over-fire direction: if the daemon observes wrapper exit
code `97`, the new class can be a strict refinement of `agent_exited_unsealed`;
if no `97` is observed, ordinary unsealed exits should stay ordinary.

The standing gap is narrower and gate-blocking: the Holder's only causal trigger
for the floor is the lane-side resolver sentinel, but the normal post-boot-epoch
rotation path either cannot reach that resolver from a non-owner lane or
explicitly suppresses the reserved-code exit because the launch token was the
normal injected session token. That leaves the actual "lane survived a daemon
restart but can no longer complete through MCP" case with no observed `97`; Spot
2 then falls back to `agent_exited_unsealed` / `agent_pid_dead`. The typed floor
under-fires.

This is not `reject`: I found no admin-token widening, no lane-readable reseal
bearer, and no credential carrying `{admin, apply, recovery,
surgical_recovery}`. It is not `accept_with_findings`: the clearing condition
requires the concrete wiring to route the floor for the rotation lock-out, not
only for a startup-time resolver refusal.

## Challenge: The Rotation Path Under-Fires Before It Can Emit `97`

### Precise claim attacked

The Holder claims Slice A closes #512 with a daemon-side/process-state floor:

- Spot 1 refuses the owner-only runtime `client-token` for a non-owner lane and
  returns `ErrUnrecoverableAcrossRotation` before reading the admin token
  (`HOLDER.md:123-144`).
- The agent-loop maps that sentinel, and only that sentinel, to
  `ExitUnrecoverableAcrossRotation = 97` (`HOLDER.md:151-168`).
- The #323 rotation watcher exits with `97` when
  `ResolveTokenMaterialFresh` returns the sentinel **and** the launch token was
  itself the runtime `client-token`; otherwise it falls back to the launch token
  (`HOLDER.md:170-188`).
- Spot 2 classifies the floor **iff** daemon-observed state contains exit code
  `97` (`HOLDER.md:244-263`), and ordinary complete-on-disk + lane-lost cases
  without `97` stay `agent_exited_unsealed` (`HOLDER.md:314-318`).

That sounds decoupled, but it misses the normal supervised-lane restart path the
RFC is about.

### Concrete refutation

Normal supervised lanes launch with a session-bound token, not the runtime
admin token. The source is explicit:

- `supervisedEnvEntries` injects the lane's own session-bound token as
  `STRIATUM_MCP_TOKEN` (`go/pkg/mutations/supervision_env.go:333-343`).
- `ResolveTokenMaterial` consults `STRIATUM_MCP_TOKEN` first and returns
  `TokenMaterial{Source: EnvMCPToken}` (`go/pkg/agentloop/token.go:18-21`).
- `Run` stores that material in `cfg.Token` before the lane adapter starts
  (`go/pkg/agentloop/loop.go:37-56`).

So in the ordinary supervised lane, the Holder's rotation-path guard
`cfg.Token.Source == <runtime client-token>` is false. If
`ResolveTokenMaterialFresh` later returns the new sentinel, the spec says to
keep the launch token and **not** request the unrecoverable exit. No `97` is
emitted.

There is a second, earlier under-fire: the rotation watcher may not reach
`ResolveTokenMaterialFresh` at all. It detects rotation only by
`ResolveMCPEndpointFresh` (`go/pkg/agentloop/loop.go:589-604`), which
deliberately ignores launch-time endpoint env and re-reads the daemon runtime
endpoint file (`go/pkg/agentloop/endpoint.go:88-107`). The daemon writes both
the endpoint file and boot-epoch file with `writeOwnerOnlyTextFile`
(`go/cmd/striatumd/main.go:632-640`, `:752-763`), which creates a `0700` runtime
dir and `0600` files (`go/cmd/striatumd/main.go:798-815`). A `striatum-lane`
process is the non-owner in this scenario. If the fresh endpoint read gets
`EACCES`/`EPERM`, the watcher treats it as "nothing to compare against yet" and
continues (`go/pkg/agentloop/loop.go:589-593`). It never asks the token resolver,
so the sentinel cannot be produced and `97` cannot be observed.

The Holder's best "fallback to the launch token" branch is not enough to make
the lane recoverable across a boot-epoch rotation. The lane's adapter config
also carries the launch daemon's boot epoch:

- the supervisor injects `STRIATUM_MCP_BOOT_EPOCH` into the lane env
  (`go/pkg/mutations/supervision_env.go:344-354`);
- codex, agy, and claude configs echo it as `X-Striatum-Boot-Epoch`
  (`go/pkg/agentloop/mcpconfig.go:123-130`, `:217-222`, `:490-496`);
- the HTTP handler rejects a presented epoch that differs from the live daemon
  before request dispatch (`go/pkg/mcp/http.go:681-699`);
- the claude rewrite path rebuilds the config from `laneBootEpoch()`, i.e. the
  old env value, not a new daemon epoch (`go/pkg/agentloop/mcpconfig.go:490-523`);
  codex cannot reload its launch-time `-c` MCP URL at all and only receives an
  in-PTY wedge prompt (`go/pkg/agentloop/loop.go:625-645`).

Therefore a constructible post-restart lane can be unable to complete through
MCP while still never emitting the reserved floor code:

1. The lane launched normally with `STRIATUM_MCP_TOKEN` and
   `STRIATUM_MCP_BOOT_EPOCH`.
2. The daemon restarts and rotates endpoint + boot epoch.
3. The lane-side watcher cannot read the owner-only fresh endpoint/epoch files,
   or it reaches token refresh but suppresses the sentinel because
   `cfg.Token.Source == EnvMCPToken`.
4. The adapter remains pointed at a dead endpoint or presents a stale boot-epoch
   header to the new daemon. Its MCP path is broken.
5. If the lane exits unsealed or is reaped, the daemon has no durable/process
   observation of exit code `97`; the Holder's exact-code Spot 2 class cannot
   fire and recovery records an ordinary class.

This does not depend on W1, a kernel-token capture, a reseal-token file, or any
Slice-B artifact. It is a Slice-A wiring bug: the only specified floor signal is
not emitted for the normal rotation lock-out.

### Why the Holder's strongest rebuttal does not clear it

The strongest rebuttal is that a lane holding the injected session-bound bearer
is still recoverable, so the guard intentionally avoids over-firing. If that
were always true, suppressing `97` would be correct.

But the current control plane has more state than the bearer. The adapter also
uses a launch-time endpoint and boot-epoch header. The fresh endpoint/epoch
files are owner-only, the rewrite path reuses the stale epoch, and codex cannot
reload the launch-time URL. A live session-bound bearer does not by itself prove
the lane can complete through MCP after the daemon restart. Conversely, if the
Holder wants to define those cases as outside Slice A, then the spec no longer
solves the #512 class it claims to make legible: a survived lane can be locked
out after rotation and still die as generic `agent_exited_unsealed`.

The second rebuttal is that the design intentionally avoids any durable
per-lease boot-epoch record. I agree that Slice A must not assume such a record.
The problem is not absence of a DB epoch predicate; it is that the spec provides
no alternate exact daemon-side/process-state trigger for the post-rotation
failure. "No durable epoch" is acceptable only if some other exact signal emits
`97`. Here the only signal is gated away or unreachable.

### Required revision

Keep the no-widening resolver refusal and the exact-code daemon classification,
but add an executable rotation-path trigger that is actually reachable for a
normal session-bound supervised lane and does not over-fire. Any acceptable
repair must state the exact predicate and tests. Plausible shapes:

- make the endpoint/boot-epoch freshness signal lane-readable without exposing
  any bearer, and when the lane proves it is carrying a stale launch epoch it
  exits with `ExitUnrecoverableAcrossRotation`;
- or have the agent loop map a daemon `stale_daemon_identity` response from its
  own MCP client path to the same reserved exit, with a negative proving ordinary
  non-epoch MCP errors do not fire the floor;
- or remove/update the stale boot-epoch header as part of a proven recoverable
  reconnect path, and reserve `97` for the remaining exact unrecoverable cases.

Add tests that cover the current under-fire:

- `TestRotationWatcherUnrecoverableWhenFreshEndpointUnreadableForLane`:
  non-owner lane, fresh runtime endpoint/epoch files inaccessible, adapter MCP
  path stale; assert the lane does not silently continue and eventually die
  generic.
- `TestSessionBoundLaunchTokenDoesNotSuppressStaleEpochFloor`: launch token
  source is `EnvMCPToken`, fresh token read reaches the sentinel or MCP returns
  stale epoch; assert wrapper exits `97`.
- Keep `TestOrdinaryUnsealedExitStaysAgentExitedUnsealed`: complete-on-disk +
  lane-lost with no reserved exit code must not be reclassified.

## Checks Credited

- **No hidden Slice-B dependency found in the Holder's main mechanism.** The
  current Slice-A spec does not require W1, `PaneKernelStartToken`,
  `CapabilityReseal`, a reseal-token file, `resealInFlightJob`, owner bundle
  0021, or reserved code 98.
- **No admin-token widening found.** The proposed resolver branch refuses before
  reading the runtime `client-token`, and the owner path is stated as
  unaffected.
- **Spot 2's exact-code gate is the right no-over-fire shape.** A typed class
  based only on observed `97` avoids the bad "complete-on-disk + lane-lost"
  inference the SEED warned against.
- **The v7 `BC1-W1-CAPTURE-FLOOR` launch-boundary issue is dissolved for this
  decoupled design as scoped.** With no W1 launch capture in Slice A, a genuine
  launch failure can remain `helper_error`; the remaining problem is the
  post-rotation floor never being emitted.

## Bottom Line

The Holder successfully removes the Slice-B channel from Slice A, but the
remaining floor is wired to a lane-side sentinel that the normal rotation path
does not reliably produce. Until the spec defines a reachable exact trigger for
a session-bound lane carrying stale endpoint/boot-epoch state, the daemon cannot
observe `ExitUnrecoverableAcrossRotation = 97`, and the promised
`session_unrecoverable_across_rotation` class under-fires on the very boot-epoch
rotation failure Slice A is supposed to make legible.
