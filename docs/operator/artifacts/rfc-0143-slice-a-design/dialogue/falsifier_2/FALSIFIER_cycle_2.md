# FALSIFIER - RFC 0143 Slice A security / legibility / regression re-attack

author: falsifier-reviewer-002

## Verdict

**needs_revision.** I do not find a `reject`-class security widening in the Holder:
the spec preserves `ReadTokenFile`'s owner-mode guard, refuses the admin runtime
`client-token` before any non-owner lane read, mints no lane credential, and does
not introduce a bearer carrying `{admin, apply, recovery, surgical_recovery}`.
The C2/provider-child shape is also pointed the right way: the provider process
exit is normalized to a generic agent error, while only the agent-loop sentinel is
allowed to produce the reserved wrapper exit code.

The gate still does not clear under the legibility lens. The Holder makes the
typed class fire only from observed wrapper exit code `97`, but its only concrete
way to emit `97` is a resolver sentinel that the normal supervised
boot-epoch-rotation path does not reliably reach. A lane can therefore remain
wedged on stale endpoint / stale boot-epoch state and later be classified as
ordinary `agent_exited_unsealed`, `agent_pid_dead`, or a raw adapter/MCP failure,
with no durable `session_unrecoverable_across_rotation` explanation. That is an
under-fire / raw-explanation leak for the class Slice A is supposed to make
legible.

This is not `accept_with_findings`: the missing trigger is on the #512 path
itself, not a cosmetic diagnostic. The build contract would let the strongest
game-day shape still die without the daemon ever observing `97`.

## Challenge: The Normal Rotation Lock-Out Can Die Without Emitting `97`

### Precise claim attacked

The Holder's legibility claim has three load-bearing steps:

- Spot 1 refuses a non-owner lane at the owner-only runtime `client-token` tier
  and returns `ErrUnrecoverableAcrossRotation` before reading the admin token
  (`HOLDER.md:123-146`).
- The agent-loop wrapper maps that sentinel, and only that sentinel, to
  `ExitUnrecoverableAcrossRotation = 97` (`HOLDER.md:150-168`).
- Spot 2 classifies the typed recovery class iff daemon-observed state contains
  exit code `97` (`HOLDER.md:241-270`), while ordinary no-`97` unsealed exits
  stay `agent_exited_unsealed` (`HOLDER.md:312-327`, `:366-369`).

The #323 rotation watcher weakens this: the Holder only requests the
unrecoverable exit when `ResolveTokenMaterialFresh` returns the sentinel **and**
the launch token source was already the runtime `client-token` (`HOLDER.md:170-184`).
That is not the normal supervised-lane shape this RFC is about.

### Concrete refutation

Normal supervised lanes launch with their own session-bound token, not the
runtime admin token:

- `supervisedEnvEntries` injects the session-bound bearer as `STRIATUM_MCP_TOKEN`
  (`go/pkg/mutations/supervision_env.go:333-343`).
- `ResolveTokenMaterial` consults `STRIATUM_MCP_TOKEN` first and records
  `Source: EnvMCPToken` (`go/pkg/agentloop/token.go:18-21`).
- `Run` stores that material in `cfg.Token` before the adapter starts
  (`go/pkg/agentloop/loop.go:32-56`).

So the Holder's rotation guard is false in the ordinary case. If the fresh token
resolver reaches the new sentinel later, the spec says to keep the launch token
and not request the unrecoverable exit. No wrapper exit `97` is emitted.

There is also a path where the fresh token resolver is never reached. The watcher
first calls `ResolveMCPEndpointFresh` (`go/pkg/agentloop/loop.go:589-604`), which
reads the daemon runtime endpoint file (`go/pkg/agentloop/endpoint.go:88-107`).
The daemon writes that endpoint file and the boot-epoch file with
`writeOwnerOnlyTextFile`, creating a `0700` runtime directory and `0600` files
(`go/cmd/striatumd/main.go:632-640`, `:752-763`, `:798-815`). For a
`striatum-lane` process, unreadable endpoint/epoch files make the watcher
silently continue with the launch endpoint (`loop.go:589-593`) before it ever
asks for fresh token material.

The "launch token still works" rebuttal is incomplete because the bearer is not
the whole client identity after #316:

- the supervised env also injects `STRIATUM_MCP_BOOT_EPOCH`
  (`go/pkg/mutations/supervision_env.go:344-354`);
- codex receives the MCP URL and boot-epoch header as launch-time `-c` overrides
  (`go/pkg/agentloop/mcpconfig.go:113-132`);
- claude/agy configs echo the same boot epoch from `laneBootEpoch()`, which is
  the lane's launch env (`go/pkg/agentloop/mcpconfig.go:217-222`, `:490-496`);
- the HTTP handler rejects a request whose presented epoch differs from the live
  daemon epoch as `stale_daemon_identity` before dispatch
  (`go/pkg/mcp/http.go:681-699`);
- codex cannot reload its launch-time URL and only gets an in-PTY wedge prompt
  when the watcher notices rotation (`go/pkg/agentloop/loop.go:625-645`).

That constructs the legibility failure:

1. A supervised lane launches normally with `STRIATUM_MCP_TOKEN` and
   `STRIATUM_MCP_BOOT_EPOCH`.
2. The daemon restarts and rotates endpoint plus boot epoch.
3. The lane cannot read the owner-only fresh endpoint/epoch files, or it reaches
   token refresh but suppresses the sentinel because `cfg.Token.Source` is
   `EnvMCPToken`.
4. The adapter remains on a dead launch-time endpoint, or presents a stale
   `X-Striatum-Boot-Epoch` to the new daemon.
5. When the lane exits or is reaped, Spot 2 sees no `97`, so the daemon records
   an ordinary unsealed/dead class or a generic stale-MCP/adapter failure instead
   of the promised `session_unrecoverable_across_rotation`.

This is not a Slice-B dependency and not a request to widen token access. It is a
Slice-A legibility miss: the exact floor signal is not emitted for a normal
post-rotation lock-out, so the raw/stale endpoint explanation survives as the
terminal operator-visible reason.

### Strongest rebuttal for the Holder

The best defense is that a session-bound bearer should remain valid across a
normal daemon restart, so emitting `97` merely because the runtime admin token is
unreadable would over-fire. I agree with that narrow point. The floor must not
fire just because a lane is non-owner and an owner-only token exists.

But current source couples adapter reachability to launch-time endpoint and
boot-epoch state as well as the bearer. A valid bearer does not prove the lane can
complete through MCP after restart. If the spec wants to treat stale endpoint /
stale boot epoch as outside Slice A, it no longer makes the #512 failure legible;
if it wants to include that failure, it needs an exact non-over-firing trigger
that is reachable for session-token lanes.

## Security / Regression Sweep

- **No admin-token widening found.** The proposed branch refuses before
  `ReadTokenFile`; it does not relax the owner-only mode check
  (`go/pkg/agentloop/token.go:75-92`) and does not introduce group-read.
- **No elevated credential mint found.** Slice A still has only an exit code and
  derived recovery class; no token carries `{admin, apply, recovery,
  surgical_recovery}`.
- **C2 is directionally satisfied.** `normalizeAgentExitError` wraps provider
  process exit as `"agent command exited"` (`go/pkg/agentloop/loop.go:371-379`),
  and the Holder maps only `ErrUnrecoverableAcrossRotation` to `97`, so a child
  exit `97`/`98` should not forge the floor. The named test is present.
- **No-over-fire shape is right but incomplete.** Exact-code-only classification
  protects ordinary `agent_exited_unsealed` and healthy lanes from being
  reclassified. The miss is the opposite: a covered rotation lock-out under-fires
  because no exact code is produced.
- **A1-A6 are named but under-specify the real rotation path.** The Holder names
  A1-A6, A3 negative, and C2/A5. It does not name a test proving a
  session-token lane with stale endpoint/boot-epoch state exits `97` or records
  the typed class.

## Required Revision

Keep the no-widening resolver refusal and the exact-code-only recovery class, but
add a reachable, non-over-firing trigger for normal supervised lanes after daemon
rotation. An acceptable repair must specify the exact predicate and tests. For
example:

- make non-secret endpoint / boot-epoch freshness lane-readable without exposing
  any bearer, and map a proven stale-launch epoch to
  `ExitUnrecoverableAcrossRotation`;
- or map the daemon's `stale_daemon_identity` response from the lane MCP client
  path to the same reserved exit, with negatives for ordinary MCP/network errors;
- or prove and test an in-place reconnect path that updates endpoint and epoch
  for each adapter, reserving `97` only for the remaining unrecoverable cases.

Add tests such as:

- `TestSessionBoundLaunchTokenDoesNotSuppressStaleEpochFloor`: launch token source
  is `EnvMCPToken`; the lane presents a stale boot epoch after daemon rotation;
  assert the wrapper exits `97` and recovery records
  `session_unrecoverable_across_rotation`.
- `TestUnreadableFreshEndpointDoesNotSilentlyContinueWithoutTypedFloor`:
  non-owner lane cannot read the runtime endpoint/epoch files; assert the watcher
  does not silently leave the lane to die as ordinary `agent_exited_unsealed`.
- `TestCodexRotatedEndpointWedgeRecordsTypedUnrecoverable`: codex cannot reload
  the `-c` MCP URL; assert the terminal state is the typed floor, not only an
  in-PTY prompt or stale endpoint failure.
- Keep `TestOrdinaryUnsealedExitStaysAgentExitedUnsealed` and
  `TestLaunchHandshakeFailureStaysHelperErrorNotFloor` as negatives so the repair
  does not over-fire.

## Bottom Line

The Holder avoids the two reject-class hazards: it does not widen the admin token
and does not mint an elevated credential. It also avoids over-firing on ordinary
no-`97` unsealed exits. The problem is that the normal rotation wedge can also be
no-`97`; in that case the new typed floor never becomes durable daemon state.
Until the spec names and tests an exact trigger for stale endpoint / boot-epoch
lock-out in session-token lanes, Slice A does not deliver the legible
`session_unrecoverable_across_rotation` floor it promises.
