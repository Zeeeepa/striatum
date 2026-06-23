# FALSIFIER - RFC 0143 design-v5 regression/lifecycle re-attack

author: falsifier-reviewer-004

## Verdict

**needs_revision.** I do not find an independent BC4/BC5 split-brain or lock-order
regression in the v5 proposal. The v4-credited lifecycle set is carried forward in
substance: `jobs.recovery_generation`, `leases.reseal_grace_extended_at`, the
skip/replace/replay map, the run-lock serialization against `artifact.publish` and
the recovery sweep, the backend-gate bypass, and the authored-this-attempt fold-in
are all named coherently.

The gate still should not clear. The v5 connect-out rework makes the authenticated
frame the primary entry point for the daemon-observed reseal condition, but the
specified W1 identity check compares two different start-token domains. That means
the legitimate pane wrapper can be rejected before the reseal predicate even runs,
leaving the design dependent on pane-dead-status and recovery-sweep backstops while
claiming a working primary channel. For this security/authz decision, a primary
entry point that is not specified with one consistent process identity is a standing
falsification, not a build-run detail.

This is not `reject`: I found no route that lets a lane read the runtime
`client-token`, and no lane-readable credential carrying `{admin, apply, recovery,
surgical_recovery}`.

## Challenge: Primary Connect-Out Frame Uses An Incoherent Start Token

### Precise Claim Attacked

The Holder claims the connect-out helper accepts the pane wrapper iff
`SO_PEERCRED` reports the run-as uid, `peer.pid == result.PID`, and
`ProcessStartToken(peer.pid)` equals the already-captured pane identity token
(`identity.PaneStartToken`). The real-path test is also defined in those terms:
the launched pane wrapper must send a frame accepted as `identity.PanePID` /
`PaneStartToken`.

This accepted frame is not decorative. v5 makes it the primary signal for
`resealInFlightJob`; the pane-dead-status and recovery-sweep paths are backstops.

### Concrete Refutation

Current source does not make `PaneStartToken` a kernel `/proc` field-22 token.
`CaptureTmuxIdentity` stores tmux `#{pane_start_time}` when it is numeric
(`go/pkg/supervisor/tmux_liveness.go:181-209`) and falls back to
`ProcessStartToken(panePID)` only when the tmux value is absent or non-numeric.
`ProbeTmuxLiveness` repeats the same tmux-token-first model for ongoing liveness
(`tmux_liveness.go:228-268`). The existing unit test treats `1748452211` from
tmux as a valid `PaneStartToken` without cross-checking `/proc`
(`go/pkg/supervisor/tmux_liveness_test.go:46-60`).

By contrast, `ProcessStartToken` is explicitly the Linux kernel token from
`/proc/<pid>/stat` field 22 (`go/pkg/supervisor/process_identity_linux.go:11-31`).
The v5 W1 check therefore compares a kernel process start tick on the left with a
tmux pane-start value on the right whenever tmux returns a numeric value. That is
not a stable pid-reuse proof. On the real tmux path it can reject the actual wrapper;
alternatively, an implementation could be pressured to weaken or skip the
start-token comparison, which reopens the pid-reuse hole W1 is supposed to close.

Lifecycle consequence: BC2/BC3/BC4/BC5 can all be correct and the primary reseal
frame still never enters their predicate, because the channel authentication fails
before `resealInFlightJob` obtains the run lock. The pane-dead-status and
recovery-sweep backstops are valuable, but they do not satisfy v5's claim that the
production connect-out channel is itself a working authenticated entry point.

### Strongest Rebuttal For The Holder

The connect-out topology itself is the right repair for the v4 fd-through-tmux
hole, and the fix is small: capture a distinct kernel start token from
`ProcessStartToken(identity.PanePID)` immediately after tmux reports the pane pid,
persist/use that for W1, and compare it to `ProcessStartToken(peer.pid)` on the
accepted connection. Tmux `#{pane_start_time}` can remain tmux liveness metadata,
but it cannot be the W1 pid-reuse token unless the implementation proves it is
identical on supported hosts.

That rebuttal does not clear the current text. The implementation contract must
name one identity token for W1 and test that exact token on the real path.

### Required Revision

- Add a named kernel identity field or contract for the control channel, captured
  from `/proc/<pane-pid>/stat` field 22 after `CaptureTmuxIdentity` reports the pane
  pid and before any control connection is accepted.
- Keep `PaneStartToken` / tmux `#{pane_start_time}` as liveness metadata unless it is
  proven equivalent to the kernel token.
- Extend `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` so the accepted
  connect-out frame compares `/proc/<peer-pid>/stat` field 22 to the captured
  `/proc/<pane-pid>/stat` field 22, and add a negative for a mismatched or stale
  kernel start token.

## Regression And Lifecycle Sweep

- **BC2:** no regression found. The frame still carries no job id, path, body, or
  artifact content; artifact identity remains daemon-derived from
  `expected_artifacts`.
- **BC3/F4:** no regression found. `CapabilityReseal` remains daemon-internal and
  the public route alternate remains test-only and limited to the three reseal
  routes.
- **BC4/BC5:** no regression found. The proposal preserves owner bundle 0021 for
  `jobs.recovery_generation` and `leases.reseal_grace_extended_at`, the stamped
  generation comparison, the one-grace extension, and the corrected lock-order
  map. `artifact.publish` still serializes by taking `lockRunForJob` first, and the
  recovery sweep still drains helper events before taking the run lock and
  expiring/requeuing inside that lock.
- **Positive intent / backend gate:** conceptually intact, but gated by the W1
  issue above for the primary frame. The recovery-sweep backstop and
  `#{pane_dead_status}` backstop remain valid fallback paths; the design must not
  count them as proof that the primary connect-out entry point is sound.
- **Authored-path fold-in:** credited. The v5 proposal correctly says the
  deliverable-observed predicate must reuse `gitChangedPathSnapshots` plus
  `collectInScopeAuthoredPaths`, with nil baseline for isolated per-job worktrees,
  so an unchanged pre-existing expected path is not resealed.
- **F7 file mirror / AF1 / AF4 / no-widening:** no regression found. I found no
  admin-token exposure, no new lane-readable reseal bearer, and no token reminting
  claim.

## Bottom Line

The lifecycle core that v4 credited is still present, but v5's new primary channel
entry point is not yet a falsifiable implementation contract. Pin W1 to a single
kernel process identity source, prove it through `RunHelper` with tmux and
`RunAsUser`, and this lens should be able to clear the regression/lifecycle set.
