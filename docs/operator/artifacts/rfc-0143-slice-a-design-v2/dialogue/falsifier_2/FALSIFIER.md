# FALSIFIER - RFC 0143 Slice A v2 security/forge/regression re-attack

author: falsifier-reviewer-002

## Verdict

**needs_revision.** The v2 holder materially improves the v1 tmux finding: a bare
`#{pane_dead_status}==97` is no longer trusted by itself, the direct-path
`agent_exited.exit_code` carrier is still forge-resistant, and I found no path that
widens the owner-only admin runtime `client-token` or mints a credential carrying
`{admin, apply, recovery, surgical_recovery}`. The observability-only premise also
holds on authority: the typed floor is specified as no-more-privileged than
`agent_exited_unsealed` and adds no new seal trigger.

But the revision still does not clear FIX-2. The new trusted corroborator, T1
(`daemon.stale_epoch_rotation`), is not forge-resistant in the shared-uid threat model.
The holder labels T1 trusted/forge-resistant and uses it to corroborate the tmux carrier,
but its own attribution mechanism accepts the lane bearer and boot epoch as sufficient
identity. Under the same `striatum-lane` uid, those are lane-side environment secrets.
A same-uid sibling can therefore forge the T1 observation first, then run the exact
`tmux respawn-pane ... exit 97` forge. The v2 negative only covers a bare tmux respawn
**without** a daemon observation; it does not cover a same-uid forged daemon observation.
That is a standing falsification: the typed class is still driveable by the same actor
that made `#{pane_dead_status}` untrusted.

This is not `reject`: no elevated credential is exposed or minted. It is not
`accept_with_findings`: the seed requires the **trusted** floor carrier to be
forge-resistant, and corroborating one forgeable lane-side oracle with another does not
clear FIX-2.

## Challenge: T1 Is A Forgeable Corroborator, So The Tmux Forge Still Drives The Class

### Precise claim attacked

The holder's FIX-2 claim is that the production tmux carrier is safe because
`PaneDeadStatus==97` records `session_unrecoverable_across_rotation` only when
corroborated by T1:

- T1 is described as the primary trusted carrier: a `daemon.stale_epoch_rotation` event
  for the owning session, recorded when `validateBootEpoch` rejects a request as
  `stale_daemon_identity`.
- `deadAgentUnrecoverableAcrossRotation` returns true for `PaneDeadStatus==97` only if
  that T1 observation exists for the owning session.
- The bare same-uid `tmux respawn-pane -k -t <pane> -- sh -c 'exit 97'` negative is
  expected to stay `agent_exited_unsealed` because there is no T1 observation.

That would be sufficient only if T1 itself were forge-resistant against the same shared-uid
actor. It is not.

### Concrete refutation

Current source puts both ingredients for T1 in the supervised lane environment. The lane's
session-bound bearer is injected as `STRIATUM_MCP_TOKEN` (`go/pkg/mutations/supervision_env.go:333-343`),
and the daemon boot epoch is injected as `STRIATUM_MCP_BOOT_EPOCH` (`supervision_env.go:344-354`).
The agent-loop MCP config reads that boot epoch from the environment and sends it as the
MCP boot-epoch header (`go/pkg/agentloop/mcpconfig.go:14-23,123-130`). Session-bound tokens
are ordinary `token_id.secret` bearers stored in `striatumd.clients` and bound to the session
through `striatumd.client_capabilities.session_id` (`go/pkg/mutations/session_token.go:60-96`).
The proposed `IdentifyBoundSession` reuses the same authorization store: split the bearer,
verify the HMAC, reject revoked/expired, and read the bound `session_id` from
`client_capabilities` (`go/pkg/rpc/auth_pg.go:60-117,145-156`). The HTTP handler runs
`validateBootEpoch` before bearer validation and rejects stale epochs as
`stale_daemon_identity` (`go/pkg/mcp/http.go:159-169,681-699`).

The holder itself admits the residual: a same-uid sibling can read another lane's
`STRIATUM_MCP_TOKEN` from its environment and present it, and that residual is the same
RFC-0168-bounded oracle (`HOLDER.md:249-259`). That admission is fatal for a **trusted** T1
carrier. The request that creates T1 is just `Authorization: Bearer <victim session token>`
plus the stale boot-epoch header; the daemon cannot distinguish the victim lane's own MCP
client from a same-uid sibling that copied the victim's bearer and epoch. The event is a
daemon observation of a forged request, not a forge-resistant observation that the owning
lane itself hit the rotation wall.

The full forge is therefore:

1. After a daemon boot-epoch rotation, a same-uid sibling reads the victim lane's old
   `STRIATUM_MCP_TOKEN` and `STRIATUM_MCP_BOOT_EPOCH` from the lane environment. This is
   the exact shared-uid class from `BC1-W1-ORACLE`, and the v2 holder acknowledges it.
2. The sibling sends any MCP request to the current daemon with the victim bearer and the
   stale boot-epoch header. `validateBootEpoch` rejects it as `stale_daemon_identity`; the
   proposed branch then calls `IdentifyBoundSession` and records
   `daemon.stale_epoch_rotation` for the victim session.
3. The sibling runs `tmux respawn-pane -k -t <victim-pane> -- sh -c 'exit 97'`. The tmux
   attack surface remains the same: tmux commands run as the lane uid
   (`go/pkg/supervisor/pty.go:98-112,310-314`), `remain-on-exit` is set before the real
   command (`pty.go:459`), the real command is launched by `respawn-pane -k`
   (`pty.go:479-484`), and the session name is deterministic (`pty.go:620-633`). The
   current probe reads tmux pane state from the same tmux surface
   (`go/pkg/supervisor/tmux_liveness.go:125-133,212-228,257-281`).
4. Recovery sees both predicates the holder requires: T1 exists for the owning session and
   `PaneDeadStatus==97`. Under the v2 predicate, that records
   `session_unrecoverable_across_rotation` even though the victim wrapper never produced
   `ErrUnrecoverableAcrossRotation` and the observed stale-epoch request was not from the
   victim lane.

The v2 test `TestProviderCannotRespawnPaneToForgeUnrecoverableAcrossRotation` is therefore
too weak if it only asserts "bare tmux respawn with no T1 stays unsealed." The actual
same-uid adversary can forge the corroborator first. A better required negative is:
`TestSameUidSiblingCannotForgeStaleEpochObservationWithStolenBoundToken`, followed by the
existing tmux respawn, asserting no typed class is recorded.

### Strongest rebuttal for the holder

The holder's best defense is that this remains observability-only. A same-uid sibling could
already kill its own lane or force an `agent_exited_unsealed` outcome; the typed floor does
not mint authority, does not widen the admin token, and does not seal anything that the
existing `agent_exited_unsealed` durable-artifact path would not seal. The holder also
honestly says the bearer-env residual is RFC-0168-bounded rather than solved.

That defense does not clear this packet's gate. The seed allowed honest RFC-0168 scoping for
the **tmux carrier**, not for the carrier the holder designates as **trusted** and
forge-resistant. Once T1 is same-uid forgeable, `PaneDeadStatus==97 AND T1` is not meaningful
corroboration; it is two observations derived from mutable lane-side material under the same
uid. Even if the result is "only" a false recovery reason, the task's clearing condition says
a forge that still drives the typed class is a standing falsification. Slice A exists to make
the rotation failure legible; a reason that same-uid code can manufacture is not the trusted
floor the revised spec claims.

### Required revision

Keep the v2 no-widening and direct-path pieces, but repair the trusted-carrier claim:

1. Do not call T1 forge-resistant unless the daemon can prove the stale-epoch request came
   from the owning lane through a primitive a same-uid sibling cannot copy. A bearer and boot
   epoch copied from lane env are not enough.
2. Add the missing negative: stolen victim session token + stale epoch must not let a same-uid
   sibling record `daemon.stale_epoch_rotation` for the victim in a way that can corroborate a
   tmux `exit 97` respawn.
3. If no such primitive exists before RFC 0168, say so explicitly: the tmux production path
   remains RFC-0168-bounded and cannot claim a forge-resistant trusted carrier from T1. In that
   case the build should either rely only on the direct-path wrapper `agent_exited.exit_code`
   where available, or defer the trusted tmux-path classification until per-lane principals or
   another unforgeable daemon-side provenance source exists.

## Regression / No-Widening Sweep

- **No admin-token widening found.** `ReadTokenFile` still rejects non-owner-mode files and
  reads the contents only after the mode guard (`go/pkg/agentloop/token.go:75-92`). The v2
  `IdentifyBoundSession` shape reads the daemon token store, not the admin runtime
  `client-token`, and grants no capability to the stale request.
- **No elevated credential minted.** The v2 Slice A changes are still an exit code, daemon
  observation event, stall class, and tmux probe field. I found no `{admin, apply, recovery,
  surgical_recovery}` credential path and no `CapabilityReseal`/connect-out Slice-B artifact.
- **Direct-path C2 remains credited.** The direct carrier comes from the wrapper's own
  `Cmd.Wait` exit status (`go/pkg/supervisor/helper.go:427-439,499-507`), preserved in
  `supervision.go:412-425`; a provider child exit is normalized to a generic wrapper failure
  by `normalizeAgentExitError` (`go/pkg/agentloop/loop.go:365-379`), so a child `exit 97`
  does not drive direct-path `agent_exited.exit_code==97`.
- **Observability-only authority framing holds, but is not enough.** The proposed routing is
  no-more-privileged than the existing unsealed-finalization path
  (`go/pkg/mutations/recovery_decision_tree.go:946-960`), so this is a `needs_revision`, not
  `reject`. The remaining gap is diagnostic/provenance truth: the supposedly trusted rotation
  class can still be forged under the shared uid.
