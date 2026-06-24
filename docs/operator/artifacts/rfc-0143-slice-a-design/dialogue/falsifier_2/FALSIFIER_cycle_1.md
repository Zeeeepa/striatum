# FALSIFIER - RFC 0143 Slice A security / legibility / regression re-attack

author: falsifier-reviewer-004

## Verdict

**needs_revision.** I do not find a `reject`-class admin-token widening in the
Holder. The spec keeps the runtime `client-token` owner-only, preserves
`ReadTokenFile`'s owner-mode guard, refuses before a non-owner lane reads the
admin token, and mints no credential carrying `{admin, apply, recovery,
surgical_recovery}`. I also credit the Holder for deleting the Slice-B channel:
no W1 connect-out path, no `CapabilityReseal`, no reseal-token file, no kernel
token capture, no owner bundle 0021, and no reserved code `98`.

The gate still does not clear under the SECURITY / LEGIBILITY / REGRESSION lens.
The Holder's C2 forge-resistance argument is too narrow for the tmux-backed path
it explicitly relies on. It proves only that a provider child exiting `97` is
normalized by the agent-loop wrapper into a generic wrapper failure; it does not
prove that the provider child cannot drive the daemon-observed tmux
`#{pane_dead_status}` to `97`. Because the production tmux control surface is
run as the same lane uid, a provider child can respawn the current pane with a
command that exits `97`; recovery then observes `pane_dead_status==97` and
records `session_unrecoverable_across_rotation` even though the wrapper never
hit `ErrUnrecoverableAcrossRotation`.

This is not `reject` because it does not widen a credential. It is not
`accept_with_findings` because C2 and A3 are part of the clearing condition: a
provider child must not forge the reserved floor code, and the typed floor must
not over-fire on a lane that did not hit the credential-chain sentinel.

## Challenge: Tmux `pane_dead_status` Lets The Provider Child Forge `97`

### Precise claim attacked

The Holder's Spot 2 and C2 claims are:

- the tmux path observes the reserved floor from `#{pane_dead_status}` and then
  classifies `session_unrecoverable_across_rotation` exactly when that observed
  status is `97` (`HOLDER.md` sections 3.1-3.4);
- the provider child cannot drive the reserved code because `runWithIO`
  normalizes the child process exit through `normalizeAgentExitError`, and
  `main.go` emits `97` only for `ErrUnrecoverableAcrossRotation` (`HOLDER.md`
  section 4);
- A5 is satisfied by
  `TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker`, which
  drives a fake inner agent that exits `97`/`98` and asserts the wrapper does
  not exit `97` (`HOLDER.md` section 5).

That test covers the direct child-exit propagation path. It does not cover the
tmux observation path the Holder uses for production lanes.

### Concrete refutation

The source still exposes the tmux control surface to the same uid as the lane
process:

- `commandInvocationWithEnvFile` runs every `RunAsUser` command as
  `sudo -n -u <runAsUser> -- env -i ...` (`go/pkg/supervisor/pty.go:98-112`).
- `tmuxRunnerForSpec` returns `RunAsTmuxRunner(spec.RunAsUser, spec.Env)`
  (`pty.go:310-314`), so setup/probe tmux commands use the lane uid.
- The tmux session name is deterministic from `runID`, `laneID`, and
  `supervisorID` (`pty.go:620-633`); those values are present in the supervised
  environment (`go/pkg/mutations/supervision_env.go:320-355`) and preserved into
  the provider process by `AgentEnvironment` (`go/pkg/agentloop/bootstrap.go:71-99`).
  Even without recomputing the name, a same-uid process inside tmux can discover
  its current session/pane through tmux itself.
- The real lane command is launched by `tmux respawn-pane -k ...` (`pty.go:479-484`),
  then the daemon attaches to the same session (`pty.go:506-533`). There is no
  private tmux socket or daemon-owned tmux server in this Slice-A spec.

Therefore a provider child running inside the pane has a direct forge path:

1. The lane starts normally under the agent-loop wrapper.
2. The provider child runs a shell command equivalent to
   `tmux respawn-pane -k -t <current pane> -- sh -c 'exit 97'`.
3. That command replaces the pane process with a short-lived process that exits
   `97`. The wrapper did not return `ErrUnrecoverableAcrossRotation`; it was
   killed/replaced by the same-uid tmux control surface.
4. Because `remain-on-exit` is set before the real command runs (`pty.go:459`),
   the dead pane remains queryable. The Holder's proposed tmux extension reads
   `#{pane_dead_status}` and sees `97`.
5. `deadAgentUnrecoverableAcrossRotation` classifies the job as
   `session_unrecoverable_across_rotation` because its exact-code predicate is
   satisfied, even though the credential-chain floor never occurred.

This is a provider-child forge of the reserved floor on the production tmux
path. It is also an A3 over-fire: an arbitrary provider process can turn a
healthy or ordinary unsealed lane into the special rotation floor by controlling
the tmux pane status, not by hitting the resolver sentinel.

### Why the Holder's strongest rebuttal does not clear it

The strongest rebuttal is that a malicious or broken provider child can already
kill its own lane, so this does not grant new authority. I agree that this is
not a credential-widening defect. But C2 is not only about credential authority;
it is a clearing assertion that the provider child cannot drive the reserved
agentloop floor code. The Holder gives the typed class operational meaning:
auto-finalize-from-durable-artifact when possible, otherwise a distinct
"session unrecoverable across rotation" remediation. Letting an arbitrary
provider child manufacture that class makes the diagnostic false and breaks the
no-over-fire requirement.

The second rebuttal is that D261 allows Slice A to use daemon-side process/tmux
state and not an authenticated inbound frame. I accept the decoupling premise.
The problem is the concrete provenance of the observed tmux status: `pane_dead_status`
is daemon-observed state, but under the current tmux control boundary it is not
necessarily the exit status of the daemon-launched agent-loop wrapper. The v7
record already proved the same same-uid tmux oracle is mutable; Slice A removes
the W1 channel, but it still uses that oracle as the tmux carrier for the
reserved floor.

### Required revision

Keep the no-widening resolver refusal and the exact-code classification shape,
but repair C2 for the tmux-backed path before clearing:

- Add a real tmux-path negative test, e.g.
  `TestProviderCannotRespawnPaneToForgeUnrecoverableAcrossRotation`, where a
  provider child or same-uid lane process respawns the current pane with
  `exit 97`; assert recovery does **not** record
  `session_unrecoverable_across_rotation`.
- Either make the daemon observe a wrapper-owned exit status that a provider
  child cannot set on the tmux path, or make the tmux control surface
  inaccessible to the provider child before trusting `#{pane_dead_status}` as
  the reserved floor. If that requires the per-lane security principal from RFC
  0168, say so and do not claim Slice A's tmux Spot 2 is cleared yet.
- Keep `TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker`, but
  treat it as insufficient by itself: it covers child-exit propagation through
  `normalizeAgentExitError`, not tmux-pane-status forgery.

## Checks Credited

- **No admin-token widening found.** The Holder does not relax the runtime
  `client-token` mode, does not introduce group-read, and does not mint any
  elevated credential.
- **ReadTokenFile guard not relaxed.** The existing `mode&0077 != 0` rejection
  at `go/pkg/agentloop/token.go:75-92` remains part of the stated design.
- **The launch/capture raw-error gap from v7 is scoped out by the decoupled
  Slice-A design.** With no W1 launch capture, a genuine launch failure can
  remain `helper_error`; my objection is not the v7 capture-boundary route.
- **A1-A6 are named, but A5 is incomplete.** The required provider-child C2
  assertion lacks the tmux-path negative needed for the actual
  `#{pane_dead_status}` carrier.

## Bottom Line

The Holder's no-widening story is clean, but its tmux observation story lets the
provider child forge the reserved floor through the same shared-uid tmux control
surface the v7 ledger already identified. Until C2 covers the tmux path, the
daemon can record `session_unrecoverable_across_rotation` for a lane that never
hit the credential-chain sentinel, so the Slice-A revision should not clear.
