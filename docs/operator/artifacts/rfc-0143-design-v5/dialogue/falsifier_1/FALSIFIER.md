# FALSIFIER - RFC 0143 design-v5 BC1-CHANNEL re-attack

author: falsifier-reviewer-003

## Verdict

**needs_revision.** The v5 Holder makes the right architectural move: it deletes
the v4 inherited-fd design, uses a connect-out channel, treats
`STRIATUM_SUPERVISOR_CONTROL_ADDR` as non-secret, delivers the nonce only after
peer authentication, names the `HelperLaunchSpec` / `LaunchSpec` / `RunHelper`
plumbing sites, and adds a real-path `RunHelper` + `RequireTmux` + `RunAsUser`
test shape. That genuinely answers the old "fd 3 was passed to the tmux client,
not the tmux-server-spawned pane" objection.

BC1-CHANNEL still does not clear. The replacement proof now rests on the daemon
accepting only the launched wrapper pid plus a start-time token, with W2
(`PR_SET_DUMPABLE(0)`) preventing a same-uid sibling from commandeering or
observing that wrapper before it receives the nonce. The current v5 text leaves
two material identity gaps in that proof:

1. The W2 "sudo setuid launch reinforces dumpable=0" claim is not installed on
   the pane wrapper in the real tmux path. Current source runs the tmux control
   commands under `RunAsUser`, but the pane command itself is the env-file shell
   shim that `exec`s the lane command from an already lane-owned tmux server.
   The spec names no parent-side or native pre-main mechanism that makes the
   shim/early agentloop non-dumpable before the Go agentloop code can run.
2. The W1 pid-reuse guard compares `/proc/<peer-pid>/stat` field 22 to an
   "already-captured" `PaneStartToken`, but current source fills that field from
   tmux `#{pane_start_time}` whenever tmux returns a numeric value, and only
   falls back to `/proc` when tmux gives no usable token. The spec must pin one
   kernel token source; otherwise the real wrapper can be rejected or the build
   can be tempted to drop the start-time check.

This is not `reject`: I found no admin-token widening, no lane-readable reseal
bearer, and no credential minted with any of `{admin, apply, recovery,
surgical_recovery}`. It is not `accept_with_findings`: both gaps sit inside the
load-bearing W1/W2 proof for structural no-replay on the production channel.

## Challenge 1: W2 is credited to the wrong process boundary

### Precise claim attacked

The Holder claims the pane wrapper calls `prctl(PR_SET_DUMPABLE, 0)` as the
first agentloop action, and that this is "reinforced by the `sudo` setuid
launch" because `sudo -u striatum-lane` causes the kernel to reset dumpability
before the wrapper is live. That claim is used to support the stronger
statement that a same-uid sibling cannot read or influence the wrapper before it
connects out and receives the nonce.

### Concrete refutation

The source path does not put the pane wrapper behind a direct `sudo ... exec`
boundary.

- `commandInvocationWithEnvFile` wraps commands in
  `sudo -n -u <RunAsUser> -- env -i ...` when `RunAsUser` is set
  (`go/pkg/supervisor/pty.go:98-112`).
- For tmux, `tmuxSetupLaunchSpec` only clears `EnvFilePath`; it preserves
  `RunAsUser` (`pty.go:115-118`). So `new-session`, `respawn-pane`, and the tmux
  identity probes are the commands that get launched through the run-as wrapper.
- The actual pane command passed to `tmux respawn-pane` is not `sudo`; it is
  `envFileWrappedCommand(laneSpec.EnvFilePath, laneSpec.Command)`
  (`pty.go:479-483`), whose shim is `/bin/sh -c 'set -a; . "$1"; rm -f -- "$1";
  shift; exec "$@"'` (`pty.go:24`, `pty.go:282-287`).
- `CaptureTmuxIdentity` then records `#{pane_pid}` from that tmux pane
  (`tmux_liveness.go:181-209`), and `attachTmuxPTY` reports that pane pid as
  `LaunchResult.PID` (`pty.go:517-533`).

That topology is exactly why v4's fd inheritance failed, and it also matters for
W2. The process that eventually has the accepted `SO_PEERCRED` pid is born from
the tmux server's pane command and the env-file shim, not from a direct
credential-changing `sudo` exec of agentloop. A design that relies on "sudo
setuid launch already made `/proc/<wrapper-pid>` root-owned" has not proved that
property for the real pane process.

The remaining explicit W2 mechanism is "agentloop calls `PR_SET_DUMPABLE(0)` as
the first instruction." The Holder places that in a new Go agentloop file and
the `loop.go` entrypoint. That is not the same as a kernel-level first
instruction after the shell shim execs the binary: the shell shim is same-uid
and dumpable, and the Go runtime necessarily runs before ordinary Go entrypoint
code. The spec names no native pre-main wrapper, parent-side dumpability
operation, or test that adversarially proves a same-uid sibling cannot attach to
or read the pane process in that pre-prctl window.

Why this is material: W1 authenticates a pid, not the integrity of the process
behind that pid. If a same-uid sibling can seize the still-dumpable pane process
before the wrapper sets dumpable=0, it can make the real pane pid perform the
connect-out, receive the daemon-issued nonce, and send a frame whose
`SO_PEERCRED` pid/start-time matches. That is false provenance by the accepted
wrapper pid, not merely a sibling with the wrong pid being refused.

### Strongest rebuttal for the Holder

The best Holder defense is that v5 removed the inherited fd and moved the nonce
post-auth, so the env-file shim holds only a non-secret socket address. A normal
sibling that merely learns the address still connects with its own pid and is
rejected by W1. The explicit `PR_SET_DUMPABLE(0)` call can also make the steady
state safe immediately after agentloop starts.

That defense is not enough for this gate. The SEED asks whether the same-uid
surface is closed at every launch step on the real tmux/sudo/env-file path. The
v5 proof still depends on a dumpability ordering it attributes to the wrong
process boundary and does not test under an adversarial pre-prctl race. For a
security/authz channel, "the address is non-secret" does not answer the case
where the accepted pid itself is commandeered before W2 is actually in force.

### Required revision

Pin a real W2 installation mechanism for the pane process, not just the tmux
client/server:

- either introduce a tiny native wrapper or equivalent pre-main path that sets
  `PR_SET_DUMPABLE(0)` before any Go runtime or control-channel code can run;
- or prove from the actual tmux server / pane child semantics that the pane
  process is already non-dumpable before the env-file shim and agentloop run;
- and extend `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` so a same-uid
  adversary probes `/proc/<pane-pid>/{environ,fd,mem}` and ptrace attachability
  during the shim-to-agentloop launch window, before the wrapper dials the
  control socket. A test that checks only after the wrapper has already connected
  does not refute this gap.

## Challenge 2: W1 names two different start-token sources

### Precise claim attacked

The Holder says the daemon authenticates the accepted peer by `SO_PEERCRED`
uid+pid plus `/proc/<pid>/stat` start-time matched against the already captured
pane identity. Its concrete text says:

- `peer.pid == result.PID`, the launched pane pid; and
- `ProcessStartToken(peer.pid)` from `/proc/<pid>/stat` field 22 equals
  `identity.PaneStartToken`, captured from tmux `#{pane_start_time}`.

### Concrete refutation

Current source does not make `PaneStartToken` a guaranteed kernel field-22 token.

`CaptureTmuxIdentity` asks tmux for
`#{window_id}|#{pane_id}|#{pane_pid}|#{pane_start_time}` and stores
`verifiedStartToken(parts[3])` when that value is numeric
(`go/pkg/supervisor/tmux_liveness.go:181-209`). It falls back to
`ProcessStartToken(panePID)` only when the tmux value is absent or not usable.
`verifiedStartToken` only parses an unsigned integer; it does not prove or
convert the token to Linux `/proc` field 22 (`tmux_liveness.go:429-437`).

The kernel token is a different source: `ProcessStartToken` explicitly reads
`/proc/<pid>/stat` field 22 (`go/pkg/supervisor/process_identity_linux.go:11-31`).
The existing tests preserve the distinction: `TestProbeTmuxLivenessOK` treats a
numeric `1748452211` tmux value as a valid `PaneStartToken`, while
`TestCaptureTmuxIdentityIgnoresLiteralStartToken` uses the `/proc` fallback only
when tmux returns a literal/unusable value
(`go/pkg/supervisor/tmux_liveness_test.go:46-59`, `:259-271`).

So as written, W1 can compare a `/proc` field-22 token on the peer side with a
tmux pane-start value on the captured side. The likely outcomes are both bad for
the spec: either the real wrapper is rejected because the tokens do not match,
or the implementation weakens the pid-reuse guard to get the channel working.

### Strongest rebuttal for the Holder

This is easy to repair without changing the v5 topology. The implementation can
capture a separate kernel start token with `ProcessStartToken(identity.PanePID)`
immediately after `CaptureTmuxIdentity` and compare the accepted peer's
`/proc` token to that same kernel token. Tmux `#{pane_start_time}` can remain
liveness metadata.

But that is not what the current spec says. The build contract must name the
kernel token explicitly, and the real-path test must assert the same token source
on both sides. "Reuse `PIDLiveWithStartToken`" is not sufficient while
`PaneStartToken` may still mean a tmux token.

### Required revision

Pin one source of truth for the W1 pid-reuse guard:

- add or name a `PaneKernelStartToken` captured from
  `ProcessStartToken(identity.PanePID)` before accepting control connections;
- keep tmux `#{pane_start_time}` out of the peer-auth predicate unless the build
  proves it is equivalent on supported hosts;
- make `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` reject a matching pid
  with a stale/mismatched kernel start token and accept only when both sides use
  `/proc` field 22.

## Checks credited

- **The v4 fd-through-tmux objection is genuinely addressed in shape.** Connect-out
  means no fd must traverse the tmux client/server boundary, and no fd 3 exists
  for the provider or sibling to steal.
- **The control address being readable is not itself a credential.** A sibling
  that only knows the abstract socket address should be refused by W1.
- **Provider stdout/stderr remains non-authoritative.** The frame carries no
  artifact path/body/job identity, and PTY bytes stay outside the control path.
- **The tmux exit-code drift is identified.** v5 correctly stops treating
  `result.Cmd.Wait()` as the pane wrapper exit on tmux launches and names
  `#{pane_dead_status}` as the backstop to test.

## Carry-forward sweep

I found no regression in the v4-credited set under this BC1 lens:

- **BC2:** artifact identity remains daemon-derived from the job's
  `expected_artifacts`; the control frame supplies no path or body.
- **BC3 / F4:** `CapabilityReseal` remains daemon-internal, with public
  route-alternate limited to the test-only reseal paths and recording `reseal`,
  not `write`.
- **C2:** provider child statuses 97/98 are still reserved away from wrapper
  control semantics.
- **Daemon-observed positive intent and backend-gate bypass:** the reseal
  predicate remains daemon-observed, authored-this-attempt, and routed through
  `resealInFlightJob` rather than `HandleCompleteWork` / live-backend gates.
- **BC4 / BC5:** the generation guard, owner-bundle-0021 migration placement,
  one-extension grace, and reseal-vs-sweep lock-order story are carried forward.
- **F2 / no-widening:** no lane-readable reseal bearer, no admin-token read path,
  and no minted credential carrying `{admin, apply, recovery,
  surgical_recovery}`.
- **F7 file-mirror half / AF1 / AF4:** no regression found.

## Bottom line

v5 is close and materially better than v4, but BC1-CHANNEL still needs a tighter
identity contract. The next revision should prove W2 on the actual pane process
before any same-uid sibling can seize the accepted pid, and should compare one
kernel start token source on both sides of W1. Until then, structural no-replay
on the production tmux channel remains unproven.
