# FALSIFIER - RFC 0143 Slice A v2 FIX-1 re-attack

author: falsifier-reviewer-001

## Verdict

**needs_revision.** I credit the main v2 move: it demotes the lane-side resolver
sentinel and makes the daemon's own `stale_daemon_identity` rejection the primary
producer. That is the right direction for D261 Slice A, and I found no admin-token
widening, no Slice-B credential, and no minted authority carrying `{admin, apply,
recovery, surgical_recovery}`.

The revision still does not clear FIX-1. The new T1 observation is too coarse in
two ways:

1. it is durable and session-wide, but not terminal, so one stale-epoch rejection
   can poison a session that later recovers and then dies for an unrelated reason;
2. it only exists when a stale request reaches a live daemon, so the dead-old-port
   plus unreadable-fresh-endpoint branch still has no concrete producer despite
   the named tests.

The first is an A3 no-over-fire failure on the same session. The second is still
an A2-route under-fire on a real #512 shape. Either one is a standing
falsification; together they mean the revised wiring is not yet a safe FIX-1
closure.

## Challenge 1: T1 Is Not Terminal, So A Recovered Same Session Can Over-Fire Later

### Precise Claim Attacked

The Holder claims T1 resolves pre-auth attribution and no-over-fire: a stale boot
epoch rejection is attributed to the bound session without authorizing the
request, then recovery records `session_unrecoverable_across_rotation` only for
that owning session when it is lane-lost. Its negatives cover unattributable
rejections, a new session after requeue, a currently healthy lane, and an
ordinary unsealed exit with no observation.

That misses the important same-session case: a valid session can receive one
stale-epoch rejection, recover in place, continue doing useful work on the fresh
endpoint, and only later die unsealed for a non-rotation reason.

### Concrete Refutation

The current source already has a same-session recovery path for adapters with a
rewritable config. The rotation watcher deliberately ignores the cached launch
literal, re-reads the fresh endpoint, rewrites the ephemeral MCP config, and
prompts reconnect; the comments say this restores the CLI's repo_write path
(`go/pkg/agentloop/loop.go:560-570`). The implementation applies a new endpoint
when `ResolveMCPEndpointFresh` succeeds, reuses or refreshes the token, calls
`applyMCPEndpointRotation`, logs success, and advances `appliedEndpoint`
(`loop.go:589-611`). Codex is a special wedge because it cannot reload `-c`, but
Slice A is specified for supervised lanes generally, not only codex.

A concrete over-fire sequence:

1. A normal supervised lane launches with its own session-bound bearer in
   `STRIATUM_MCP_TOKEN` and its launch-time boot epoch in
   `STRIATUM_MCP_BOOT_EPOCH` (`go/pkg/mutations/supervision_env.go:333-354`).
2. The daemon restarts. The lane makes one request that reaches a live recycled
   MCP port. `ServeHTTP` runs `validateBootEpoch` before bearer validation
   (`go/pkg/mcp/http.go:159-169`); the epoch mismatch returns
   `stale_daemon_identity` (`http.go:681-699`).
3. Under the Holder's §3.3, that single rejection appends a durable
   `daemon.stale_epoch_rotation` event for the session. The spec says the write
   is idempotent and that one standing observation per session is sufficient
   (`HOLDER.md:261-285`).
4. The same lane then recovers in place: the watcher reads the fresh endpoint,
   rewrites the ephemeral MCP config, and the adapter reconnects. The session is
   now healthy and using the current daemon.
5. Later, after ordinary work, the lane dies unsealed for a mundane reason: a
   provider process exits, the user kills the pane, or the model reaches a
   turn-end with an already-published artifact. No new rotation lock-out is in
   progress.
6. Recovery now sees lane-lost plus the old T1 observation. The Holder's exact
   predicate is existence-based: T1 fires when a `daemon.stale_epoch_rotation`
   observation exists for this run's owning session (`HOLDER.md:371-381`), and
   the typed check is interposed before `agent_exited_unsealed` (`HOLDER.md:386-408`).
   The ordinary unsealed death is therefore reclassified as
   `session_unrecoverable_across_rotation`.

This is an over-fire. The session is not unrecoverable across rotation at the
later death; it demonstrably survived the rotation and resumed against the fresh
daemon. The stale observation is a historical event, not proof that the terminal
failure is a rotation lock-out.

The Holder's listed negatives do not catch it. "Healthy lane" only says no class
while the lane is live; it does not clear or age out the observation. "Relaunched
lane" only covers a new owning session after operator requeue; this attack keeps
the same session. "Ordinary unsealed exit" is defined as having no observation,
but the false-positive case has a stale historical observation (`HOLDER.md:320-329`,
`:547-550`).

### Strongest Rebuttal For The Holder

The strongest defense is that the #512 codex incident cannot recover in place:
codex bakes the MCP URL into launch-time `-c`, and the current wedge path only
writes a prompt and returns nil (`go/pkg/agentloop/loop.go:625-646`). If the
revision is scoped to codex-only unrecoverability, then a stale observation may
be terminal in practice for that adapter.

That defense is not enough for the spec as written. The Holder's T1 predicate and
A2/A3 tests are generic daemon recovery rules. It explicitly routes recovery by
session observation, lane-lost, and artifact state, not by codex adapter or by a
record that the stale rejection was the final successful daemon interaction. A
durable event that means "this session once presented a stale epoch" cannot be
used later as "this session is now unrecoverable across rotation" without a
freshness or terminality guard.

### Required Revision

Keep the daemon-side T1 direction, but make the predicate terminal, not merely
historical. Any of these would be concrete enough to falsify:

- record a bounded rotation episode/generation and require the stale observation
to be later than the last successful authenticated MCP call, heartbeat, tool
call, or endpoint-rewrite recovery for that same session;
- clear or supersede the T1 observation when the same session successfully
speaks to the fresh daemon epoch;
- key T1 to a lease/recovery generation and require the dead-lane recovery sweep
to prove no post-observation live progress occurred before using the typed class.

Add a negative such as
`TestRecoveredSameSessionStaleEpochObservationDoesNotPoisonLaterUnsealedExit`:
seed a valid session-bound stale-epoch rejection, then a later successful MCP
activity on the same session against the fresh epoch, then an ordinary
agent-exited-unsealed death; recovery must stay `agent_exited_unsealed`, not
`session_unrecoverable_across_rotation`.

## Challenge 2: The Dead-Old-Endpoint Branch Still Has No Concrete Producer

T1 is only recorded in `ServeHTTP` on the live daemon's boot-epoch rejection
branch. If the surviving lane keeps talking to a dead old dynamic port that is
not recycled, no request reaches `ServeHTTP`, so there is no T1 event to record.
The Holder itself describes this branch: the watcher re-resolves the endpoint
from owner-only runtime files and the current code silently continues when those
files are unreadable (`HOLDER.md:206-210`; `go/pkg/agentloop/loop.go:589-594`).
The runtime endpoint and boot-epoch files are owner-only shapes (`go/cmd/striatumd/main.go:632`,
`:752-756`; tests pin `0600` files and `0700` runtime dir), so this is not a
hypothetical path for `striatum-lane`.

The revised §3.4 names the right tests, but not a mechanism that makes them pass.
It says to route the watcher when the lane's own MCP client observes a
`stale_daemon_identity` response, and to route the codex wedge when the endpoint
is known to have rotated (`HOLDER.md:293-313`). That still leaves the branch
where the fresh endpoint cannot be read and the old endpoint is just dead. The
current watcher does not know the endpoint rotated, the MCP client observes a
network failure rather than `stale_daemon_identity`, and codex never reaches the
wedge because `applyMCPEndpointRotation` is only called after a fresh endpoint is
read.

Required revision: define the non-secret, lane-readable freshness source or the
explicit local process-state predicate that turns "fresh endpoint/epoch unreadable
for a non-owner lane while launch endpoint is dead" into `ErrUnrecoverableAcrossRotation`
without mapping ordinary transient network errors to the floor. The named tests
`TestRotationWatcherUnrecoverableWhenFreshEndpointUnreadableForLane` /
`TestUnreadableFreshEndpointDoesNotSilentlyContinueWithoutTypedFloor` need to
assert that exact branch, not just the stale-daemon-identity HTTP branch.

## Checks Credited

- The pre-auth attribution shape is non-widening if `IdentifyBoundSession` only
verifies a daemon-minted session-bound bearer and grants no capability. The
request remains rejected.
- Unattributable stale-epoch requests correctly record nothing.
- The direct-path `97` mapping remains narrow: only
`ErrUnrecoverableAcrossRotation` maps to `ExitUnrecoverableAcrossRotation`, and a
provider child exit code is normalized on the direct path.
- FIX-2 is materially improved for the tmux carrier: bare `#{pane_dead_status}==97`
without a T1 observation no longer drives the class, and the holder honestly
scopes the shared-uid residual.

## Bottom Line

The v2 holder fixes the old "session-bound token never reaches the resolver
sentinel" defect for the recycled-port stale-epoch subcase, but it over-corrects
by treating any session-level stale observation as permanently terminal, and it
still does not name a reachable producer for the dead-old-endpoint/unreadable-
fresh-file branch. The FIX-1 floor therefore still either over-fires on a
recovered same session or under-fires when no stale request reaches the daemon.
The revision should not clear.