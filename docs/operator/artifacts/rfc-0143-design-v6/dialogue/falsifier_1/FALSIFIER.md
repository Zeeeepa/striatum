# FALSIFIER - RFC 0143 design-v6 BC1-W1-TOKEN re-attack

author: falsifier-reviewer-003

## Verdict

**needs_revision.** The v6 Holder genuinely resolves the specific v5
BC1-W1-TOKEN defect: W1 no longer compares kernel `/proc` field 22 against tmux
`#{pane_start_time}`. The revised predicate names `PaneKernelStartToken`, captures
it with `ProcessStartToken(identity.PanePID)`, compares the peer's
`ProcessStartToken(peer.pid)` to that captured kernel token, keeps
`identity.PaneStartToken` as tmux liveness metadata only, and adds the right
same-pid stale-token and empty-token fail-closed negatives.

The remaining falsification is narrower but still material to W1's no-replay
proof on the real tmux path: the spec calls the new value a **launch-time** kernel
token, but the named capture point is still after `tmux respawn-pane` has already
started the pane command and after a separate tmux identity round trip. The Holder
does not require a fail-closed boundary check proving that the pane is still the
same live pane when the kernel token is stamped. If the launched wrapper exits
before that `/proc` read and the pid is reused before capture, the helper can bind
W1 to the reused process's fresh field-22 token. The comparison is then coherent
but coherent for the wrong process.

This is not `reject`: I found no admin-token widening, no lane-readable reseal
bearer, and no new credential carrying `{admin, apply, recovery,
surgical_recovery}`. It is not `accept_with_findings`: the defect is still inside
the load-bearing W1 identity proof that makes the same-uid connect-out channel
safe.

## Challenge: The Kernel Token Is Post-Launch Unless Capture Proves The Pane Is Still Live

### Precise claim attacked

The Holder says v6 captures a named kernel start token with
`ProcessStartToken(identity.PanePID)` immediately after `CaptureTmuxIdentity`
reports the pane pid in `launchPTY`, before any control connection is accepted,
and that this binds the launch-time identity of the pane wrapper. It then accepts
a control peer only when:

- `peer.uid == RunAsUser uid`;
- `peer.pid == result.PID`;
- `result.PaneKernelStartToken != ""`; and
- `PIDLiveWithStartToken(peer.pid, result.PaneKernelStartToken)` matches `/proc`
  field 22 on both sides.

Those operands are now the right clock domain. The missing piece is the capture
boundary: the spec does not prove the captured kernel token belongs to the
process tmux just launched.

### Concrete refutation

The current launch order makes the token a post-launch read, not an atomic birth
identity:

- `launchPTY` starts the lane command through `tmux respawn-pane` before any
  kernel token is captured (`go/pkg/supervisor/pty.go:479-484`).
- Only after that does it call `CaptureTmuxIdentity`
  (`go/pkg/supervisor/pty.go:493-504`) and then attach, returning
  `LaunchResult.PID = identity.PanePID` (`go/pkg/supervisor/pty.go:517-533`).
- `CaptureTmuxIdentity` queries `#{pane_pid}` and `#{pane_start_time}` only; it
  does not query `#{pane_dead}` and does not pin `/proc` field 22 at process birth
  (`go/pkg/supervisor/tmux_liveness.go:181-209`).
- `ProcessStartToken` reads whatever process currently owns `/proc/<pid>/stat`
  field 22 at the time of that later read
  (`go/pkg/supervisor/process_identity_linux.go:13-32`).

So the v6 fix closes the v5 "different clocks" hole, but it still has a
capture-race hole: if the pane wrapper exits after tmux reports `identity.PanePID`
but before the helper stamps `PaneKernelStartToken`, and that pid is reused before
the `/proc` read, the helper records the reused process's field-22 token as the
launched wrapper token. From then on W1 is internally consistent but bound to the
wrong subject: the reused-pid process can satisfy `peer.pid == result.PID` and
`PIDLiveWithStartToken(peer.pid, result.PaneKernelStartToken)`, receive the
post-auth nonce, and send a control frame that the daemon treats as wrapper
provenance.

The v6 same-pid stale-token negative does not catch this inverse case. It proves
W1 rejects when the captured token is stale relative to the peer. Here the captured
token is fresh, but fresh for a process that was not the launched wrapper. The
empty-token negative also does not catch it because `ProcessStartToken` succeeds.

### Strongest rebuttal for the Holder

The best Holder defense is that v6 captures immediately after tmux reports the pid,
before accepting any connection. That does close the lazy "read the token on first
accept" variant, and it gives ordinary sibling connections no authority: wrong pid
siblings are refused, stale-token same-pid cases are refused, and empty-token cases
fail closed instead of falling through the `PIDLiveWithStartToken(pid, "")` pid-only
branch.

That rebuttal is not enough for this security/authz gate. W1 exists specifically
because pid-only identity is not stable under reuse. A design cannot call the token
"launch-time" merely because the helper reads it promptly after launch; it must
also fail closed if the launched pane is already dead or identity-unverifiable at
the capture boundary.

### Required revision

Keep the v6 field-22-to-field-22 design, but add one capture-boundary invariant:

- after `CaptureTmuxIdentity` reports `identity.PanePID`, capture
  `ProcessStartToken(identity.PanePID)` and immediately verify the same tmux pane is
  still live, with the same pane id and pid, before the accept loop can bind a
  channel;
- if the pane is dead, missing, changed, or identity-unverifiable at that boundary,
  do not stamp `PaneKernelStartToken` and do not accept any control connection for
  that launch; route through the `#{pane_dead_status}` / recovery-sweep typed floor;
- extend `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` or a paired integration
  negative with the "pane dies before kernel-token capture" case: force the wrapper
  to exit before W1 capture, make the token read see a same-pid/reused-process token
  by stub or controlled hook, and assert the helper refuses instead of treating that
  token as the launched wrapper identity.

The existing same-pid stale-token and empty-token negatives should remain. They are
necessary, but they do not prove the post-launch capture belongs to the original
pane wrapper.

## Checks Credited

- **BC1-W1-TOKEN's direct v5 defect is fixed.** v6 uses one kernel token domain:
  captured `/proc/<pane-pid>/stat` field 22 compared to accepted peer `/proc`
  field 22. It does not keep tmux `#{pane_start_time}` as the W1 operand.
- **The empty-token trap is handled.** v6 correctly notes that
  `PIDLiveWithStartToken(pid, "")` skips the token comparison and requires W1 to
  fail closed instead of accepting on pid alone.
- **The real-path test contract is materially stronger.** It now requires
  `RunHelper` with `RequireTmux`/`RunAsUser`, field-22 on both sides, same-pid
  stale-token rejection, empty-token rejection, provider isolation, and sibling
  refusal.
- **The v5-credited set is not regressed by the W1-token change.** I found no
  regression in the connect-out topology, named plumbing sites, non-secret address
  plus post-auth nonce, W2 ordering, `#{pane_dead_status}` backstop plus C2, BC2,
  BC3/F4, BC4/BC5, daemon-observed positive intent, backend-gate bypass, F2, F7
  file-mirror half, AF1, AF4, or the no-admin-token-widening invariant.

## Bottom Line

v6 is the right repair direction and clears the v5 clock-domain objection. It still
needs a capture-boundary guard so W1 proves the kernel token is actually the
launched wrapper's token, not merely the token of whatever process owns the pane pid
when the helper performs its post-launch `/proc` read.
