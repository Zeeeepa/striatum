# FALSIFIER - RFC 0143 design-v4 BC1 re-attack

author: falsifier-reviewer-003

## Verdict

**needs_revision.** The v4 Holder fixes the v3 BC1 shape in a direct-child
model: `SCM_CREDENTIALS` against wrapper pid+start-time, nonce out of env,
reserved-exit masking, daemon-observed positive intent, and backend-gate bypass
are all the right walls. The spec still has two material BC1 precision gaps
before it is buildable:

1. The inherited-fd control channel is not pinned through Striatum's real
   tmux/run-as/env-file launch path, so the "launched wrapper pid + fd 3 +
   dumpability before exposure" proof is only true for an unspecified direct
   `os/exec` harness.
2. The daemon-observed positive trigger relies on `write_scope_baseline.changed_paths`
   as if it were a per-required-artifact pre-work hash inventory, but source shows it
   is only a claim-time dirty-tree attribution baseline from the registered checkout
   and is explicitly ignored for per-job worktrees.

This is not `reject`: I found no admin-token widening and no lane-readable
credential carrying `{admin, apply, recovery, surgical_recovery}`. It is not
`accept_with_findings` either, because both gaps sit inside BC1, the security
cluster that must hold structurally before Slice A or Slice B clears.

## Challenge 1: fd 3 is not source-anchored through the production launch path

### Precise claim attacked

The Holder claims the helper creates `socketpair(AF_UNIX, SOCK_SEQPACKET, 0)`,
passes one end to the wrapper as fd 3 via `exec.Cmd.ExtraFiles`, records the
launched wrapper pid/start-time, authenticates every frame with
`SCM_CREDENTIALS`, and keeps the nonce out of env
(`docs/operator/artifacts/rfc-0143-design-v4/dialogue/holder/HOLDER.md:209`).
It also claims `PR_SET_DUMPABLE(0)` runs as the wrapper's first action before
fd 3 or nonce exposure (`HOLDER.md:239`).

### Concrete refutation

Current source does not launch the supervised lane wrapper as a direct child in
the normal path. `HelperLaunchSpec` has command/env/run-as/tmux fields, but no
control-fd field (`go/pkg/supervisor/helper_protocol.go:27`). `LaunchSpec`
likewise has no `ExtraFiles` field (`go/pkg/supervisor/pty.go:30`), and
`RunHelper` forwards only command/env/working-dir/run-as/tmux into `LaunchSpec`
(`go/pkg/supervisor/helper.go:149`).

The production PTY path creates a tmux session, writes a lane env file, and
starts the real lane command with `tmux respawn-pane` plus a shell wrapper
(`go/pkg/supervisor/pty.go:402`, `:468`, `:479`). The helper then attaches a
tmux client and reports the pane pid while `Cmd` is the attach client
(`pty.go:517`). Passing an fd with `exec.Cmd.ExtraFiles` to a local `tmux`
client does not specify how the tmux server's respawned pane child inherits that
fd, and passing it through the shell/env-file shim means a same-uid process
exists before the agentloop wrapper can execute its first `prctl`.

That leaves the load-bearing question unanswered: which process actually holds
fd 3 at each transition, which pid/start-time is the expected peer, and when are
fd 3 and the nonce first observable? A direct `os/exec` test can pass W1 while
the real `RunHelper` + `RequireTmux` + `RunAsUser` path either never delivers fd
3 to the wrapper or exposes it before the wrapper can make `/proc` non-dumpable.

### Strongest rebuttal for the Holder

W1 is strong if the frame reaches the helper: even a sibling with a duplicated fd
and nonce gets its own pid stamped by the kernel and is rejected. That materially
improves v3, and it means the main false-provenance attack is blocked in a direct
child process model.

The rebuttal does not clear BC1. The SEED requires the peer-credential check,
`PR_SET_DUMPABLE(0)` before the channel is live, and nonce isolation to be proven
against a non-child, non-wrapper same-uid process. v4 names those walls but not
the production installation path that makes them true for a tmux-backed lane.

## Challenge 2: positive intent is pinned to the wrong baseline source

### Precise claim attacked

The Holder chooses daemon-observed automatic reseal and says every required
expected artifact must be present and modified since the packet. Its concrete
proof is that the daemon re-hashes each required path and compares it to
`write_scope_baseline.changed_paths`, which the Holder describes as per-path
pre-work hashes carried by the packet (`HOLDER.md:296`).

### Concrete refutation

That is not what `write_scope_baseline` is. `buildWriteScopeBaseline` captures
`gitChangedPathSnapshots` from the registered repo root at claim time and stores
only paths already dirty/untracked in `changed_paths`
(`go/pkg/mutations/claim.go:601`, `:613`). The guard comments define the field as
claim-time dirty/untracked attribution state, not a complete per-required-path
preimage (`go/pkg/mutations/write_scope_guard.go:132`). For per-job isolation,
the source is even clearer: the packet baseline was captured before the worktree
exists, so the active worktree itself is the baseline and the guard returns a nil
baseline (`write_scope_guard.go:69`, `:81`). The source-change publisher uses
that same rule: for a fresh per-job worktree, every changed path is the attempt's
write and the baseline is nil (`go/pkg/mutations/artifact_source_publish.go:86`).

So v4 has not actually pinned the positive condition it relies on. If an expected
artifact already exists in the job worktree from a prior attempt or run-branch
input, a mechanism that treats "missing from `changed_paths`" as changed can seal
stale content; a mechanism that requires a baseline entry can never satisfy the
positive case. Either way, the required positive
`TestCodexResealUsesReceiverNotProviderStdout` is not specified against the
actual per-job worktree semantics.

### Strongest rebuttal for the Holder

The intended mechanism is close: use the active per-job worktree's git status or
tree diff, not the packet dirty baseline, and require each expected artifact path
to be newly created or modified relative to the worktree's checked-out HEAD.
That would preserve BC2's daemon-derived identity and avoid provider-asserted
intent.

But that is a revision, not the current spec. The current text pins the proof to
the wrong field and therefore leaves the positive-intent source underspecified.

## Other BC1 and carry-forward checks

- **Reserved exit codes:** v4 commits the wrapper to remap provider child exits
  97/98 to non-control `agent_exited`, with
  `TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker`
  (`HOLDER.md:193`). I do not see C2 still open at design level.
- **Backend gate / BC5 lock order:** v4 explicitly does not call
  `HandleCompleteWork`, skips/replaces the public session/backend gates, replays
  the run lock and row locks, and maps post-exit backend failures to the typed
  floor (`HOLDER.md:321`, `:512`). I do not see the v3 backend-gate leak still
  standing apart from the positive-trigger baseline problem above.
- **v3-credited security set:** I found no regression in BC2, BC3, F2, F4, the
  no-admin-token-widening invariant, or the F7 file-mirror half as written
  (`HOLDER.md:113`, `:395`, `:607`). The remaining failures are BC1 mechanism
  precision, not admin-token widening.

## Required revision

To clear BC1, revise the spec and tests to pin both real mechanisms:

- name the exact control-fd delivery path through `RunHelper`/tmux/run-as, or
  explicitly change the production launch topology so the helper directly
  launches the wrapper it authenticates;
- add a real-path test such as
  `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` that launches through
  `RunHelper` with `RequireTmux` and `RunAsUser`, proves the wrapper can send an
  accepted fd-3 frame with wrapper pid+start-time credentials, proves the
  provider lacks fd 3, and proves a same-uid non-child/non-wrapper cannot open
  `/proc/<wrapper-pid>/fd/3` or recover the nonce during the launch chain;
- replace the positive-trigger baseline with an active-worktree diff/tree
  preimage, and extend `TestCodexResealUsesReceiverNotProviderStdout` with both a
  stale-preexisting-artifact negative case and a modified-in-active-worktree
  positive case.

Until those are pinned, BC1 remains open.
