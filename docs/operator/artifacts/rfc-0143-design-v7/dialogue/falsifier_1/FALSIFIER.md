# FALSIFIER - RFC 0143 design-v7 BC1-W1-CAPTURE re-attack

author: falsifier-reviewer-003

## Verdict

**needs_revision.** I do not land the old v6 objection unchanged. The v7
Holder does add the missing post-read capture-boundary check: after
`CaptureTmuxIdentity` reports a pane pid, it reads the kernel field-22 token,
then requires a `ProbeTmuxLiveness` re-verification before stamping
`PaneKernelStartToken`. Together with the retained accept-time
`PIDLiveWithStartToken(peer.pid, result.PaneKernelStartToken)` comparison, this
is a structural answer to the narrow "wrapper exits, pid is reused before the
kernel read" race. It is not a temporal "the window is small" defense.

The standing falsification is the trust boundary of the new oracle. v7 proves
that a passing tmux re-verification can bind the token to the process tmux
currently reports. It does not prove that the reported pane process is still the
daemon-launched wrapper, because the production tmux control path is itself
invoked as the shared lane uid. In the same-uid adversary model that already
made a `0600` reseal bearer unacceptable, a sibling lane can address the same
tmux server and can replace the pane before W1 capture. The helper then stamps
and authenticates the replacement process's kernel token.

This is not `reject`: I found no admin-token widening, no lane-readable reseal
bearer, and no credential carrying `{admin, apply, recovery,
surgical_recovery}`. It is not `accept_with_findings`: the gap is inside W1, the
load-bearing no-replay wall for the real tmux connect-out channel.

## Challenge: The Tmux Capture Oracle Is Same-Uid Mutable

### Precise claim attacked

The Holder claims v7 closes BC1-W1-CAPTURE because the helper stamps
`PaneKernelStartToken` only after this sequence:

1. `CaptureTmuxIdentity` returns `identity.PanePID`;
2. `processStartToken(identity.PanePID)` reads `/proc/<pid>/stat` field 22;
3. `ProbeTmuxLiveness` runs after that read;
4. the probe must return healthy with the same pane id/pid/start;
5. only then can W1 accept a peer whose `SO_PEERCRED` pid and field-22 token
   match the stamped result.

That is enough to close the specific v6 death-and-pid-reuse gap if tmux is a
trusted witness for the daemon's own `respawn-pane`. The missing claim is the
one W1 actually needs: a same-uid sibling cannot mutate that witness between the
daemon's launch and the capture-boundary stamp.

### Concrete refutation

The current launch and probe path gives tmux control to the lane uid:

- `commandInvocationWithEnvFile` wraps any `RunAsUser` command as
  `sudo -n -u <runAsUser> -- env -i ...` (`go/pkg/supervisor/pty.go:98-112`).
- `tmuxRunnerForSpec` uses `RunAsTmuxRunner(spec.RunAsUser, spec.Env)` for a
  run-as launch (`go/pkg/supervisor/pty.go:310-314`), and that runner invokes
  the plain `tmux` client through the same run-as command path
  (`go/pkg/supervisor/tmux_liveness.go:125-133`).
- The session name is deterministic and contains the run, lane, and supervisor
  prefix plus a hash (`go/pkg/supervisor/pty.go:620-632`). The source and v7
  proposal name no private `tmux -S` socket, daemon-owned tmux server, or
  non-lane-readable tmux control boundary.
- The daemon creates/configures the tmux session, then starts the real lane
  command with `tmux respawn-pane -k`, then calls `CaptureTmuxIdentity`
  (`go/pkg/supervisor/pty.go:421-459`, `:479-493`).

The same uid is not hypothetical. The RFC and the v7 Holder both rely on the
fact that supervised lanes share the `striatum-lane` OS user; that is why a
lane-readable `0600` reseal file was rejected as a replay surface. The v7
channel deliberately treats the listener address as non-secret and says W1 must
refuse a sibling even if it learns the address and even hypothetically the
nonce. But v7 does not require the same structural refusal for a sibling that
acts one layer earlier through tmux.

One concrete same-uid replay is:

1. Striatum creates the session and issues its intended `respawn-pane -k ...`
   for the agentloop wrapper.
2. A sibling process under the same lane uid lists or targets the tmux session
   and issues its own `tmux respawn-pane -k -t <pane> -- <attacker-wrapper>`
   before `CaptureTmuxIdentity`, or between the initial identity capture and the
   post-read probe.
3. `CaptureTmuxIdentity` and `ProbeTmuxLiveness` observe a live pane with a
   consistent pane id, pid, and start token. The observation is true for the
   current tmux pane, but the current pane was supplied by the sibling, not by
   Striatum's launch command.
4. The helper stamps the replacement process's `/proc` field-22 token as
   `PaneKernelStartToken`.
5. The replacement dials the non-secret control address. W1 accepts because
   `peer.uid == RunAsUser`, `peer.pid == result.PID`, and the accepted peer's
   field-22 token matches the stamped token. The helper then delivers the
   post-auth nonce to the replacement.

The Holder's reaping invariant does not rebut this. It proves that if the
original tmux child exited and its pid was reaped/reused, a later healthy probe
would not falsely validate that reused pid. In the replay above, the attacker
does not need tmux to lie about a reaped child; it changes the tmux pane itself.
The helper still receives a healthy probe and a coherent kernel token, but those
facts bind W1 to "whatever process the same-uid tmux server currently reports",
not necessarily to "the wrapper Striatum launched".

The proposed test set misses the class for the same reason. The v7 negative
"pane dies before kernel-token capture" forces the probe into a dead or
pid-mismatched result and proves fail-closed behavior for that result. This
attack requires the opposite: the probe is healthy because the sibling has
installed a live replacement pane. A stubbed `TmuxRunner` unit is useful for
A3''', but the clearing test must also exercise the real same-uid tmux control
surface that v7 now trusts.

### Strongest rebuttal for the Holder

The strongest Holder defense is scope: v5/v6 already credited the connect-out
tmux topology, and v7 was assigned only the post-launch kernel-token capture
boundary. Also, cooperative lanes are not expected to attack each other's tmux
sessions, and the session name includes a hash suffix.

That defense is not sufficient for this security/authz gate. W1 exists precisely
because the same uid can learn non-secret channel material and because ordinary
OS ownership cannot distinguish sibling lanes. A hash suffix is not a
capability when a same-uid process can ask tmux for the session list or target
the default tmux server. If the proof depends on tmux being untampered, the spec
must make tmux untamperable by the same uid or add a second birth-provenance
primitive that a same-uid `respawn-pane` cannot forge.

### Required revision

Keep the v7 post-read `ProbeTmuxLiveness` check and the v6 field-22-to-field-22
W1 operand, but bind tmux itself to Striatum's launch authority. Any one of
these would be a plausible repair:

- run or address the lane tmux server through a daemon-owned/private control
  socket that sibling `striatum-lane` processes cannot command during launch
  and capture;
- add an explicit birth/provenance marker to the launched wrapper, not supplied
  through same-uid-mutable tmux state or non-secret env, and require the kernel
  token to be bound to that marker before W1 accepts;
- or, if tmux control cannot be isolated, fail closed to the typed
  `session_unrecoverable_across_rotation` floor instead of claiming a
  same-uid-safe authenticated primary channel.

The refuting test should be adjacent to
`TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper`: launch through `RunHelper`
with `RequireTmux` and `RunAsUser`; from a same-uid process, issue a competing
`tmux respawn-pane -k` against the target pane before capture or before the
post-read probe; assert that the helper refuses to stamp the replacement's
kernel token and accepts no frame from it. A unit with an injectable token reader
and stubbed liveness is not enough for this particular claim, because the defect
is in the real tmux control surface being treated as a trusted oracle.

## Checks Credited

- **BC1-W1-CAPTURE as stated in v6 is fixed.** v7 requires the kernel field-22
  read to be followed by `ProbeTmuxLiveness` before the token is stamped. That
  closes the prior death-and-pid-reuse window structurally when tmux is a trusted
  witness.
- **The fail-closed capture path is specified.** Empty/unreadable token,
  pane-dead, missing, pid-mismatched, start-mismatched, or identity-unverifiable
  states leave `PaneKernelStartToken == ""`, accept no control connection, and
  route through the `#{pane_dead_status}` / recovery-sweep typed
  `session_unrecoverable_across_rotation` floor rather than a raw
  launch/control error.
- **The v6 BC1-W1-TOKEN pieces are retained.** The spec keeps
  `LaunchResult.PaneKernelStartToken`, `ProcessStartToken(identity.PanePID)`,
  field-22-to-field-22 `PIDLiveWithStartToken(peer.pid,
  result.PaneKernelStartToken)`, the empty-token `!= ""` guard, and A3'/A3''.
- **The v6-credited security set is otherwise unregressed.** I found no
  regression in the connect-out topology as scoped, the named plumbing sites,
  non-secret address plus post-auth nonce, W2 ordering, `#{pane_dead_status}`
  backstop plus C2, BC2, BC3/F4, BC4/BC5, daemon-observed positive intent,
  backend-gate bypass, F2, F7 file-mirror half, AF1, AF4, or the no-admin-token
  widening invariant.

## Bottom Line

v7 closes the exact post-launch pid-reuse hole that v6 left open, but it moves
the load-bearing trust to tmux without proving tmux is protected from the same
uid adversary that motivates the whole W1/W2/W3 design. Until the spec isolates
or authenticates the tmux capture oracle, W1 structurally binds the control
channel to the current tmux pane process, not necessarily to the daemon-launched
wrapper.
