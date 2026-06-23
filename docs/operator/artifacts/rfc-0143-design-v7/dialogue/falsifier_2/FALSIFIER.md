# FALSIFIER - RFC 0143 design-v7 regression/lifecycle re-attack

author: falsifier-reviewer-004

## Verdict

**needs_revision.** I do not find a regression in the v6-credited lifecycle set. The
v7 holder keeps the BC1-W1-TOKEN field-22/field-22 operand (`LaunchResult.PaneKernelStartToken`
captured from `ProcessStartToken(identity.PanePID)`), keeps tmux `#{pane_start_time}`
as liveness metadata only, preserves the empty-token `!= ""` guard and A3'/A3'',
and carries forward the connect-out topology, W2/W3, `#{pane_dead_status}` backstop,
C2, BC2/BC3, BC4/BC5, daemon-observed positive intent, backend-gate bypass, the
authored-path fold-in, F7, AF1, AF4, and the no-admin-token-widening invariant.

The v7 capture-boundary fix is also a real improvement over v6 on the exact
post-launch pid-reuse race: it reads the kernel token, then requires a post-read
`ProbeTmuxLiveness` pass before stamping `PaneKernelStartToken`, and the retained
accept-time field-22 check closes the later `[probe -> accept]` window.

The remaining lifecycle defect is that the new fail-closed branch has no named
daemon state transition. v7 says a capture-boundary miss accepts no connection and
never leaks a raw launch/control error, but the proposed sequence runs before
`attachTmuxPTY` and before any authenticated control frame can exist. Current source
routes helper launch failures and pre-`agent_started` helper failures as raw
`helper_error` / launch-handshake errors, and v7 does not add the typed event,
recorder branch, or reseal predicate marker that would turn "kernel token left
unstamped because W1 could not be bound" into the required durable
`session_unrecoverable_across_rotation` floor.

This is not `reject`: I found no admin-token widening, no lane-readable reseal bearer,
and no credential carrying `{admin, apply, recovery, surgical_recovery}`. It is not
`accept_with_findings`: the gap is in the new v7 lifecycle branch itself, and the
seed requires a capture-boundary miss to route the typed floor, never a raw launch or
control error.

## Challenge: The Capture-Boundary Miss Has No Typed Commit Point

### Precise claim attacked

The holder's new sequence is:

1. `CaptureTmuxIdentity` returns `identity.PanePID`.
2. `processStartToken(identity.PanePID)` reads `/proc/<pid>/stat` field 22.
3. `ProbeTmuxLiveness` re-verifies the pane after that read.
4. Only a healthy same-pane result stamps `PaneKernelStartToken`; otherwise the token
   remains empty and W1 accepts no connection.

The holder then claims the miss path reaches the existing typed
`session_unrecoverable_across_rotation` floor through `#{pane_dead_status}` and the
recovery sweep, never as a raw launch/control error
(`docs/operator/artifacts/rfc-0143-design-v7/dialogue/holder/HOLDER.md:351-370`).
The concrete ordering places this inside `launchPTY`, after identity capture and
before `attachTmuxPTY` (`HOLDER.md:170-190`), and A3''' requires a durable typed
route rather than a raw launch/control error (`HOLDER.md:831-843`).

### Concrete refutation

That is exactly where the current lifecycle does not yet have a typed route.

`RunHelper` calls `helperLaunch`; if launch returns an error, it emits
`helper_error` with phase `launch` and returns (`go/pkg/supervisor/helper.go:157-165`).
The existing required-tmux regression test locks in that behavior: unavailable tmux
under `RequireTmux` yields one `helper_error` event with `phase=launch`
(`go/pkg/supervisor/helper_test.go:330-374`). On the daemon side, the launch
handshake treats a helper error before `agent_started` as a raw PTY helper failure:
`waitForHelperAgentStart` returns `PTY helper failed before attach: ...` on
`HelperEventError` (`go/pkg/mutations/supervision_launch.go:562-580`).

The event vocabulary also has no capture-boundary class. `supervise.report` accepts
`agent_exited`, `agent_started`, `artifact_observed`, `attach_client_exited`,
`helper_error`, `packet_accepted`, `process_terminated`, and `progress`; invalid
types are rejected (`go/pkg/mutations/supervision.go:19-28,217-234`). For
`helper_error`, durable payload curation preserves only `phase` and `error`
(`go/pkg/mutations/supervision.go:412-443`). By contrast, the v7 connect-out frame
exists only after W1 accepts a peer, and the miss path intentionally accepts no peer.

So the proposed miss branch is split-brained with the lifecycle:

- If the pre-attach probe returns missing/unavailable/pid-mismatched and the helper
  returns an error, the daemon records the existing raw launch/helper path, not the
  typed floor.
- If the helper instead proceeds with `PaneKernelStartToken == ""`, the primary
  authenticated frame is disabled by design. The holder then relies on
  `#{pane_dead_status}` or the recovery sweep, but it does not name the implementation
  hook that records "unstamped because capture-boundary re-verification failed" as a
  typed condition before ordinary launch failure, stale-lane, or unsealed-agent
  recovery policy sees the job.
- If the failure is `TmuxLivenessUnavailable` or identity-unverifiable rather than a
  clean pane death, there may be no pane status to observe. The recovery sweep may
  eventually evaluate authored artifacts, but that is a later daemon-observed reseal
  predicate, not proof that the capture-boundary failure itself avoided a raw
  launch/control error.

The seed's requirement is stronger than "eventual sweep might do something useful."
It requires the capture-boundary failure path itself to route through the typed
`session_unrecoverable_across_rotation` floor. v7 currently asserts that outcome but
does not specify the daemon-recognized commit point that makes it true.

### Strongest rebuttal for the holder

The best defense is that the build can deliberately avoid returning an error from
`launchPTY` on a capture-boundary miss: attach anyway with `PaneKernelStartToken == ""`,
let W1 refuse every connection, and allow the pane-dead-status backstop or the sweep
to apply the same BC4/BC5 predicate. That would preserve the no-pid-only-accept rule
and may be enough for the common "wrapper died before capture" case.

That defense is not yet a falsifiable implementation contract. The holder explicitly
covers pane-missing, pid-mismatched, and identity-unverifiable states, not just a
clean exited pane, and says "never raw launch/control error." Current source already
has a raw pre-attach `helper_error` route, and the v7 spec does not replace or
constrain it.

### Required revision

Keep the post-read `ProbeTmuxLiveness` gate and the v6 field-22 W1 predicate, but
make the fail-closed lifecycle route concrete:

1. Add a typed helper/supervision event, or an explicit daemon recorder branch, for
   "W1 kernel token unstamped because capture-boundary re-verification failed";
   it must record `session_unrecoverable_across_rotation`, not generic
   `helper_error`.
2. State how this route works before `attachTmuxPTY` and before `agent_started`, or
   move the re-verification to a point where the typed backstop is actually
   available.
3. Extend `TestKernelTokenCaptureFailsClosedWhenPaneNotReverifiedLive` and the
   real-path "pane dies before kernel-token capture" negative to assert the durable
   daemon class/blocker, not only "no stamp" and "no accepted connection".
4. Add a negative for the existing raw path: a capture-boundary miss must not become
   a terminal `supervisor.helper_error` / launch-handshake failure when the intended
   class is `session_unrecoverable_across_rotation`.

## Corroborating Lifecycle Risk: W1 Is Upstream Of All BC4/BC5 Serialization

I also credit the separate same-uid tmux-oracle challenge as lifecycle-material. The
current tmux setup and probe commands run through the lane uid:
`commandInvocationWithEnvFile` wraps run-as commands with `sudo -n -u <runAsUser> --
env -i` (`go/pkg/supervisor/pty.go:98-112`), `tmuxRunnerForSpec` uses
`RunAsTmuxRunner(spec.RunAsUser, spec.Env)` (`pty.go:310-314`), and that runner
invokes the plain `tmux` client through the same run-as path
(`go/pkg/supervisor/tmux_liveness.go:125-133`). The session name is deterministic
from run/lane/supervisor ids (`pty.go:620-632`), and the holder names no private
`tmux -S` socket, daemon-owned tmux server, or other boundary that prevents a
same-uid lane process from mutating the pane before W1 capture.

Through the lifecycle lens, that matters because `resealInFlightJob` is downstream of
W1. A same-uid replacement pane that W1 accepts can then author the expected path in
the job worktree and emit a daemon-authenticated frame; BC4/BC5 can be perfectly
serialized and still serialize the wrong process's intent. This is not a regression
of BC4/BC5. It is evidence that the upstream capture oracle must be made
same-uid-safe before the lifecycle predicate can be trusted.

## Regression Sweep

- **BC1-W1-TOKEN:** intact. v7 retains `LaunchResult.PaneKernelStartToken`,
  `ProcessStartToken(identity.PanePID)`, field-22-to-field-22
  `PIDLiveWithStartToken(peer.pid, result.PaneKernelStartToken)`, tmux
  `#{pane_start_time}` as liveness metadata only, the empty-token `!= ""` guard,
  A3', and A3''.
- **BC1-W1-CAPTURE:** improved but not lifecycle-complete. The post-read probe closes
  the v6 pid-reuse capture window; the fail-closed branch still needs a named typed
  daemon route, and the tmux oracle still needs same-uid isolation.
- **Connect-out / W2 / W3 / C2:** no regression found. No fd crosses the tmux
  boundary; the address remains non-secret; `PR_SET_DUMPABLE(0)` remains before
  dial; the nonce is still daemon-to-wrapper post-auth; `#{pane_dead_status}`
  remains a backstop, not the primary auth path.
- **BC2 / BC3 / F4:** no regression found. Frames still carry no job id, path, kind,
  or body; artifact identity is daemon-derived from expected artifacts;
  `CapabilityReseal` remains daemon-internal with the public route-alternate
  test-only.
- **BC4 / BC5:** no regression found. `jobs.recovery_generation` and
  `leases.reseal_grace_extended_at` remain owner-bundle-0021 items; the stamped
  generation guard, one-grace rule, corrected lock order, artifact.publish
  serialization, and sweep serialization are carried forward.
- **Daemon-observed positive intent / backend gate:** the reseal predicate remains
  intact and still bypasses `ensureWorkSessionBackend`; the missing piece is the
  route from the new capture-boundary miss into that typed predicate.
- **Authored-path fold-in:** intact. The deliverable-observed condition still reuses
  `gitChangedPathSnapshots` / `collectInScopeAuthoredPaths` with nil baseline for
  isolated worktrees, so an unchanged pre-existing expected path is not resealed.
- **F7 / AF1 / AF4 / no-widening:** intact. Endpoint/epoch remains daemon-owned and
  lane-read-only with missing epoch rejected; token validity and endpoint rotation
  remain decoupled; no lane reads the runtime admin token.

## Bottom Line

v7 makes the W1 kernel token much harder to mis-bind, and it preserves the v6
lifecycle machinery. It still does not prove the new fail-closed branch has an
authoritative typed route when it fires before the helper has attached or emitted an
authenticated frame. Until that route is named and tested, the spec cannot claim that
BC1-W1-CAPTURE failures never degrade into raw launch/control errors.
