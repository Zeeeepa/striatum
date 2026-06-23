You are the **Holder** for the RFC 0143 design run, and **this is the SIXTH
REVISION (v6).** Five prior falsification gates ran on this spec. v1
(`rfc-0143-design`) returned `needs_revision` with seven findings F1–F7. v2
(`rfc-0143-design-v2`) resolved F2 and F4 cleanly and distilled the residue into
five binding constraints BC1–BC5. v3 (`rfc-0143-design-v3`) resolved BC2, BC3, and
BC4 and carried the v2-credited set forward unregressed. v4 (`rfc-0143-design-v4`)
resolved BC5, two of BC1's three sub-grounds (C2 + the daemon-observed
positive-intent source with the backend-gate bypass), but returned `needs_revision`
on **BC1-CHANNEL** (the W1/W2/W3 walls were specified for a direct
`exec.Cmd.ExtraFiles` child while the production lane is TMUX-BACKED). v5
(`rfc-0143-design-v5`) **RESOLVED the big v4→v5 rework**: it deleted the
inherited-fd channel and adopted the **CONNECT-OUT topology** (the pane wrapper
dials OUT after `PR_SET_DUMPABLE(0)`; no fd crosses the tmux client/server boundary;
non-secret listener address; post-auth nonce), flagged and fixed a real v4 drift
(the `#{pane_dead_status}` exit-code backstop), and carried the v4-credited set
forward unregressed — both falsifiers credited the topology, the named plumbing
sites, the W2 ordering, and the real-path test shape. But v5 returned
`needs_revision` **again** on a single, sharply named ground that BOTH falsifiers
landed INDEPENDENTLY: **BC1-W1-TOKEN — W1's peer-credential proof compares two
CATEGORICALLY DIFFERENT clocks** (a kernel `/proc` field-22 start-tick on the peer
side against a tmux `#{pane_start_time}` wall-clock timestamp on the captured side),
so it either rejects the legitimate wrapper or must be weakened (reopening the
same-uid replay surface). **BC1-W1-TOKEN is the LAST open BC1 ground.** Read the
required context docs first: `SEED.md` (it carries the charter, a pointer to the
committed RFC `docs/rfcs/0143-lane-credential-survival-across-boot-epoch-rotation.md`,
the **`## Ratified design shape`** you must not relitigate, the
**`## Carried forward — resolved by v5`** set you must preserve, and the
**`## The binding constraint v6 MUST resolve`** section stating BC1-W1-TOKEN with its
prescribed fix, the verified source sites, and the named real-path test extension);
the design-v5 spec you are revising,
`docs/operator/artifacts/rfc-0143-design-v5/dialogue/holder/HOLDER.md`; and the v5
verdict
`docs/operator/artifacts/rfc-0143-design-v5/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`
(read its BC1-W1-TOKEN finding + rationale + the exact "next revision must…" list for
the exact prescribed repair).

**Start from the v5 `HOLDER.md` (a required context doc).** Your revised spec MUST
**resolve BC1-W1-TOKEN per its prescribed fix**, and must **carry the v5-credited
resolved set forward UNREGRESSED** — re-opening or regressing any of it fails the
gate. Do NOT relitigate the ratified OQ1 trust-model shape (Option 4 mandatory floor
+ ratification-gated Option 2 narrow `CapabilityReseal` + minimal Option 3
per-session endpoint+epoch republish), the F2 non-bearer decision, **the connect-out
topology, or the W1/W2/W3 wall shapes and W1's load-bearing role** — all pinned in
`SEED.md`'s `## Ratified design shape` and `## Carried forward — resolved by v5`. The
connect-out topology and the wall shapes are CORRECT; the open question is ONLY the
SOURCE of W1's start-token operand.

Author the **revised falsifiable implementation spec** as your published
`HOLDER.md` artifact. This is the claim the falsifiers will RE-ATTACK and the
adjudicator will gate — make it concrete and falsifiable, not a restatement of the
RFC or the v5 spec. State every load-bearing security claim as a falsifiable
assertion paired with its named test / game-day. Re-verify every source citation
against the current worktree and FLAG any drift.

Hold the root reframe: **a boot-epoch rotation must never force a lane to choose
between reading the daemon's full-authority bootstrap admin `client-token` and
exiting silently unsealed.** A `striatum-lane` lane authenticates as its own narrow,
session-scoped credential and *never* as the shared operator admin override.

Your spec MUST:

1. **Resolve BC1-W1-TOKEN — pin a SINGLE consistent KERNEL start-token source for the
   connect-out W1 peer-credential check, in ONE place (the security cluster).** The
   v5 W1 check accepts a connecting peer iff `peer.uid == RunAsUser uid`,
   `peer.pid == result.PID` (`identity.PanePID`), and `ProcessStartToken(peer.pid)`
   `== identity.PaneStartToken`. The defect is the **right-hand operand**:
   `ProcessStartToken` is the Linux `/proc/<pid>/stat` **field-22 start-TICK-since-boot**
   (`go/pkg/supervisor/process_identity_linux.go:11-32`), but the spec sources
   `PaneStartToken` from tmux `#{pane_start_time}`, which `CaptureTmuxIdentity` stores
   via `verifiedStartToken(#{pane_start_time})` **whenever tmux returns a numeric
   value**, falling back to `ProcessStartToken(panePID)` only when the tmux value is
   absent/non-numeric (`go/pkg/supervisor/tmux_liveness.go:181-209`, esp. `:194-202`);
   `verifiedStartToken` (`tmux_liveness.go:429-436`) merely checks the value parses as
   an unsigned integer. tmux `#{pane_start_time}` is a **WALL-CLOCK unix timestamp**
   (the existing test treats `1748452211` as a valid `PaneStartToken`) while `/proc`
   field 22 is **start-ticks-since-boot** — categorically different domains. So on the
   production tmux path the v5 W1 check compares a kernel start-tick against a tmux
   pane-start timestamp; consequence: either the legit wrapper is REJECTED before
   `resealInFlightJob` takes the run lock, or the pid-reuse guard is dropped (reopening
   same-uid replay). **The fix (do this in ONE place):**
   - **Capture a NAMED kernel start token** via `ProcessStartToken(identity.PanePID)`
     (`/proc` field 22, `process_identity_linux.go:11-32`) **IMMEDIATELY after**
     `CaptureTmuxIdentity` reports the pane pid (`tmux_liveness.go:181-209`) and
     **BEFORE any control connection is accepted**; persist/use THAT value for the W1
     peer-credential check.
   - **Keep tmux `#{pane_start_time}` ONLY as liveness metadata** UNLESS you PROVE it
     equivalent to `/proc` field 22 on supported hosts.
   - **Compare kernel field-22 to kernel field-22:** the accepted peer's
     `ProcessStartToken(peer.pid)` against the captured kernel token (one clock
     domain), so a same-uid sibling that connects is REJECTED on pid/start-time.
   - **Extend the real-path test** `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper`
     (through `RunHelper` with `RequireTmux`/`RunAsUser`) so the accepted connect-out
     frame compares `/proc/<peer-pid>/stat` field 22 to the captured
     `/proc/<pane-pid>/stat` field 22, AND add a **NEGATIVE** that rejects the **same
     pid** with a **mismatched/stale kernel start token** (the pid-reuse guard).

   Name the exact sites — `tmux_liveness.go:181-209` (`CaptureTmuxIdentity` /
   `verifiedStartToken`), `process_identity_linux.go:11-32` (`ProcessStartToken` =
   `/proc` field 22) — confirm them against current main, and FLAG any drift. A
   restatement, a hand-wave that "the build will normalize this", a W1 proof that
   leaves the two sides in different clock domains, a kernel token captured too late
   to bind the launch-time identity, a dropped/weakened pid-reuse guard, or a
   real-path test that does not compare field-22-to-field-22 and does not reject the
   same-pid stale-token case does NOT resolve BC1-W1-TOKEN.

2. **Carry the v5 build-test precision item forward.** The daemon-observed
   "deliverable-observed" condition must NOT treat "present + absent from
   `write_scope_baseline.changed_paths`" as sufficient by itself — for per-job
   isolated worktrees the baseline is **nil** (`write_scope_guard.go:81-83`), and
   source-change publication already attributes authorship via `gitChangedPathSnapshots`
   (`write_scope_guard.go:225`) + `collectInScopeAuthoredPaths`
   (`artifact_source_publish.go:259-263`, used at `:88`; `claim.go:622`). Keep that
   authored-path attribution so an UNCHANGED pre-existing expected path is NOT
   resealed (`TestResealRequiresAuthoredExpectedArtifactChange` or the positive
   `TestCodexResealUsesReceiverNotProviderStdout` case).

3. **Carry the v5-credited resolved set forward UNREGRESSED** (verbatim where
   applicable; see `SEED.md` `## Carried forward — resolved by v5`): the **connect-out
   topology** + the named `HelperLaunchSpec`/`LaunchSpec`/`RunHelper` plumbing sites
   that reach the PANE wrapper (not the tmux client), the **non-secret address** + the
   **post-auth nonce (W3)**, the **W2 ordering** + dumpable-before-dial, the
   **`#{pane_dead_status}` exit-code backstop** (the authenticated frame is the
   primary signal; the reserved 97/98 exit code is observed on the tmux path via
   `#{pane_dead_status}`, never from `result.Cmd.Wait()` which resolves the attach
   client) + the **C2** commitment, **BC2** (reseal artifact identity from the job's
   `expected_artifacts` daemon state, refusing unexpected paths), **BC3**
   (`CapabilityReseal` a daemon-internal marker projected by `resealInFlightJob`,
   public route-alternate test-only), **BC4** (the concrete `jobs.recovery_generation`
   column in owner bundle 0021, increment points, stamped value compared under the
   lock), **BC5** (`leases.reseal_grace_extended_at` in the same owner bundle 0021 —
   `leases` owner-held — and the corrected `work.complete` skip/replace/replay
   lock-order gate map), the **daemon-observed positive intent + recovery-sweep
   backstop**, the **`ensureWorkSessionBackend` bypass**, the **W1/W2/W3 wall shapes**,
   **F2** (no lane-readable reseal bearer), **F4** (route-alternate records `reseal`
   not `write`), the **F7 file-mirror half** (daemon-owned lane-read-only `0644`
   mirror, `O_NOFOLLOW`, atomic rename, reject MISSING boot-epoch header — closing
   #316), **AF1** reachability-not-reminting, **AF4** epoch/token decoupling, the
   categorical **no-admin-token-widening invariant**, and the **per-claim
   falsifiable-assertion discipline (A1–A18, extended A3'/A4'/A7')** — extended to
   cover the BC1-W1-TOKEN single-kernel-token contract.

4. **Hold the security invariant as the spine.** Per the carried-forward set:
   `admin.BootstrapRuntimeTokenIfNeeded` (`go/pkg/admin/bootstrap.go:18-27`) grants
   the runtime client-token the FULL `bootstrapCapabilities` set
   `{admin, read, write, claim, review, apply, recovery, surgical_recovery}`, `0600`
   in a `0700` dir. Any path that lets a lane read that file, or mints a
   lane-readable credential carrying ANY of `{admin, apply, recovery,
   surgical_recovery}`, is **categorically out of bounds** — say so explicitly and
   keep it structurally impossible. The no-replay property must hold **structurally**
   on the connect-out channel (on the REAL tmux launch path, with W1 specified as ONE
   coherent kernel identity token), not as a trackable post-clearance finding.

5. **Stay inside the product boundary and the Non-Goals.** Do NOT re-classify the
   downstream `agent_exited_unsealed` recovery policy (RFC 0152 / D249), do NOT
   change the committee POSIX-ACL repo provisioning (#537 / #539), and do NOT touch
   `run drive`'s transient-socket behavior (#513). Do NOT collide with the RFC 0125
   `HandleRecoveryReseal` worktree-durability verb (separate file, separate verb).
   Local-first, single-host, daemon-owned PostgreSQL as the single writer.

6. **Flag the maintainer ratification gate.** Slice B (the daemon-internal
   `rpc.CapabilityReseal` marker, the test-only auth-prelude route alternate, the
   daemon-owned supervisor control channel with connect-out `SO_PEERCRED`
   pid+start-time authentication, the reserved agentloop exit codes, the
   `jobs.recovery_generation` + `leases.reseal_grace_extended_at` owner-bundle-0021
   columns, and endpoint/epoch republish plumbing) is a security/authz trust-model
   change. State plainly that the cleared spec is a RECOMMENDATION the maintainer
   ratifies before any build slice lands credential code, and that Slice A (the
   Option-4 floor) is zero-trust-change but must route over the connect-out, non-PTY
   channel whose same-uid authentication (W1) is **specified with one coherent kernel
   identity token** before it lands.

Do not treat falsifier completion as acceptance — the adjudicator's collaboration
ledger decides whether the gate clears.
