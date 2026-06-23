# FALSIFIER - RFC 0143 design-v6 regression/lifecycle re-attack

author: falsifier-reviewer-004

## Verdict

**needs_revision.** On the assigned regression/lifecycle lens, v6 preserves the
v5-credited lifecycle set and fixes the direct BC1-W1-TOKEN defect: W1 no longer
compares a kernel `/proc` field-22 token to tmux `#{pane_start_time}`. The revised
proposal names `PaneKernelStartToken`, captures it with
`ProcessStartToken(identity.PanePID)`, compares the accepted peer's
`ProcessStartToken(peer.pid)` to that captured kernel token, keeps tmux
`#{pane_start_time}` out of W1, and adds same-pid stale-token plus empty-token
fail-closed tests.

The remaining gap is a different lifecycle/correctness gap in the proposed W1
predicate: v6 says to feed the captured kernel token into the existing
`PIDLiveWithStartToken`, but that helper first requires `kill(pid, 0)` signalability
and collapses any signal error into `pid_gone`. On the production split-uid path the
helper runs as the daemon uid while the pane wrapper runs as `RunAsUser`
(`striatum-lane`) through `sudo -n -u ...`; a live cross-uid process can be
non-signalable to the helper even though `/proc/<pid>/stat` field 22 is readable and
the accepted `SO_PEERCRED` peer is the wrapper. That rejects the legitimate wrapper
before the new field-22 comparison ever runs, so the claimed primary connect-out
entry point can still fail on the real `RequireTmux`/`RunAsUser` path.

This is not `reject`: I found no admin-token widening, no lane-readable reseal
bearer, and no credential carrying `{admin, apply, recovery, surgical_recovery}`.
It is not `accept_with_findings`: the defect is upstream of `resealInFlightJob`,
inside the load-bearing W1 authentication path that is supposed to make the primary
authenticated frame work on the real tmux/sudo launch path.

## Challenge: W1 Reuses A Signalability Helper That Can Reject The Live Split-UID Wrapper

### Precise claim attacked

The Holder claims the v6 W1 accept predicate is:

1. `peer.uid == RunAsUser uid`;
2. `peer.pid == result.PID`;
3. `result.PaneKernelStartToken != ""`;
4. `PIDLiveWithStartToken(peer.pid, result.PaneKernelStartToken)` returns live with
   a matching `/proc` field-22 token.

The Holder also argues the daemon already exercises this identity read path against
the production non-dumpable pane, so reusing it introduces no new permission
assumption.

### Concrete refutation

The field-22-to-field-22 operand is coherent, but `PIDLiveWithStartToken` is not a
pure identity comparison.

- `RunHelper` preserves `RunAsUser` in the launch spec
  (`go/pkg/supervisor/helper.go:149-156`), and `commandInvocationWithEnvFile`
  runs the command through `sudo -n -u <RunAsUser> -- env -i`
  (`go/pkg/supervisor/pty.go:98-112`). The production supervised lane therefore
  runs as the lane uid, not the daemon uid.
- `PIDLiveWithStartToken` first calls `pidSignalable(pid)` and returns
  `PIDLivenessGone` if that check is false, before it reads or compares
  `ProcessStartToken(pid)` (`go/pkg/supervisor/tmux_liveness.go:392-408`).
- `pidSignalable` treats the process as live only when
  `proc.Signal(syscall.Signal(0)) == nil`; it does not distinguish "no such
  process" from permission-denied/non-signalable (`tmux_liveness.go:565-573`).
- `ProcessStartToken` itself is a simple read of `/proc/<pid>/stat` field 22
  (`go/pkg/supervisor/process_identity_linux.go:13-32`). The v6 W1 proof needs that
  read and the accepted peer credentials; it does not need the daemon uid to have
  kill permission over the lane uid.

That means v6 can still reject the legitimate wrapper on the exact production path
the gate is trying to clear: the accepted connection proves the peer exists at
connect time, `SO_PEERCRED` gives the peer pid/uid, and `/proc/<peer-pid>/stat`
could still supply the field-22 token, but `PIDLiveWithStartToken` can return
`pid_gone` first because the helper cannot signal a different uid's process. The
result is the same lifecycle symptom as the v5 clock-domain bug, just from a new
source: the primary authenticated frame never reaches `resealInFlightJob`, and the
design leans on `#{pane_dead_status}` / recovery-sweep backstops while claiming a
working primary connect-out channel.

The Holder's read-permission argument does not rebut this. It addresses whether the
daemon can read `/proc/<pid>/stat` field 22 under W2; it does not address the
separate signalability predicate. It also overstates current coverage: for tmux
metadata, `ProbeLaneLiveness` returns `ProbeTmuxLiveness` rather than
`PIDLiveWithStartToken` (`tmux_liveness.go:364-378`), and `ProbeTmuxLiveness` mostly
uses tmux's pane query path (`:212-281`). So W1's proposed use of
`PIDLiveWithStartToken` as an authentication gate is a new load-bearing use, not an
already-proven production liveness path.

This challenge is independent of the capture-boundary race raised by the other v6
falsifier: even if the launch-time token is captured from the correct process, the
current W1 predicate can still refuse the live wrapper before comparing that token.

### Strongest rebuttal for the Holder

The best defense is that some deployments may run the helper with enough privilege
to signal the lane process, or may run the test with `RunAsUser` equal to the helper
uid. The proposed real-path test might also expose the issue during the build.

That is not enough for a security/authz gate. The RFC's motivating production shape
is a supervised lane running as `striatum-lane`; the proposal itself says the real
test must use `RunHelper` with `RequireTmux` and `RunAsUser`. A falsifiable
implementation spec must say whether W1 is an identity check or a signal-permission
check. As written it says to reuse a helper that is both, and current source treats
permission-denied the same as dead.

### Required revision

Keep the v6 field-22/field-22 design, but do not route W1 through
`PIDLiveWithStartToken` as currently shaped.

1. Split W1 identity matching from process signalability. After `SO_PEERCRED`,
   compare `peer.pid`, `peer.uid`, and `ProcessStartToken(peer.pid)` against the
   launch-time captured kernel token. Fail closed on empty, unreadable, or mismatched
   field-22 tokens, but do not require `kill(pid, 0)` success for a cross-uid peer
   that is otherwise authenticated by `SO_PEERCRED`.
2. If the implementation keeps a liveness probe in the helper, distinguish
   permission-denied/non-signalable from "gone"; an `EPERM`-style result is not a W1
   identity mismatch when the field-22 token matches.
3. Extend `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` so it actually launches
   with `RunAsUser` as a different OS uid from the helper and asserts the wrapper's
   authenticated frame is accepted with matching field-22 tokens. Add a focused unit
   for the W1 helper: a peer with matching pid/uid/kernel token but a
   non-signalable/permission-denied `kill(0)` result must not be classified
   `pid_gone`.
4. Any identity-unavailable or token-mismatch path must still route the typed
   `session_unrecoverable_across_rotation` floor, not leak a raw `pid_gone`,
   `lease_error`, helper backend error, or silent primary-channel failure.

## Regression Sweep

- **BC1-W1-TOKEN direct v5 defect:** fixed in the text. W1 compares kernel field 22
  to kernel field 22, keeps tmux `#{pane_start_time}` as liveness metadata only, and
  fails closed on an empty captured kernel token instead of letting
  `PIDLiveWithStartToken(pid, "")` degrade to pid-only.
- **Connect-out topology / W2 / W3:** no regression found. The channel still dials
  out from the pane wrapper after `PR_SET_DUMPABLE(0)`; no fd crosses the tmux
  client/server boundary; the listener address remains non-secret; the nonce remains
  daemon-to-wrapper post-auth.
- **BC2 / BC3 / F4:** no regression found. The control frame still carries no
  job_id, artifact path, kind, or body; artifact identity remains daemon-derived from
  the job's expected artifacts; and `CapabilityReseal` remains daemon-internal /
  test-only, not a bearer.
- **BC4 / BC5:** no regression found in the lifecycle core. `jobs.recovery_generation`
  and `leases.reseal_grace_extended_at` remain pinned to owner bundle 0021; the
  stamped-generation predicate, one-grace rule, `artifact.publish` serialization, and
  recovery-sweep lock order are carried forward.
- **Typed floor / backend gate:** no regression found in the stated
  skip/replace/replay map. `resealInFlightJob` still bypasses
  `ensureWorkSessionBackend` and routes predicate misses to
  `session_unrecoverable_across_rotation`, but the W1 split-uid rejection must be
  specified to route there too.
- **Authored-path fold-in:** carried. The deliverable-observed condition still reuses
  `gitChangedPathSnapshots` / `collectInScopeAuthoredPaths`, with nil baseline for
  isolated worktrees, so an unchanged pre-existing expected path is not resealed.
- **F7 / AF1 / AF4 / no-widening:** no regression found. The endpoint/epoch mirror
  remains daemon-owned and lane-read-only with missing epoch rejected; token validity
  and endpoint rotation stay decoupled; no lane reads the runtime admin token.

## Bottom line

v6 fixes the exact v5 clock-domain error and preserves the credited lifecycle set,
but the W1 predicate still needs one more precision fix: make authentication a
field-22 identity check over `SO_PEERCRED`, not a cross-uid signalability check. Until
that is specified and tested on the real `RunAsUser` path, the primary connect-out
entry point is not yet a falsifiable lifecycle contract.
