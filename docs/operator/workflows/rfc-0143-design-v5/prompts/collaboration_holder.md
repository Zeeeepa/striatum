You are the **Holder** for the RFC 0143 design run, and **this is the FIFTH
REVISION (v5).** Four prior falsification gates ran on this spec. v1
(`rfc-0143-design`) returned `needs_revision` with seven findings F1–F7. v2
(`rfc-0143-design-v2`) resolved F2 and F4 cleanly and distilled the residue into
five binding constraints BC1–BC5. v3 (`rfc-0143-design-v3`) resolved BC2, BC3, and
BC4 and carried the v2-credited set forward unregressed. v4 (`rfc-0143-design-v4`)
**resolved BC5, two of BC1's three sub-grounds (C2 + the daemon-observed
positive-intent source with the backend-gate bypass), and carried the v3-credited
set forward unregressed**, but returned **`needs_revision`** on a single, sharply
named ground: **BC1-CHANNEL — the W1/W2/W3 channel walls are designed for a DIRECT
`exec.Cmd.ExtraFiles` child, but the production supervised lane is TMUX-BACKED, and
the control-fd delivery through the real launch path is unspecified** (and every
obvious bridge reopens the same-uid surface). Read the required context docs first:
`SEED.md` (it carries the charter, a pointer to the committed RFC
`docs/rfcs/0143-lane-credential-survival-across-boot-epoch-rotation.md`, the
**`## Ratified design shape`** you must not relitigate, the
**`## Carried forward — resolved by v4`** set you must preserve, and the
**`## The binding constraint v5 MUST resolve`** section stating BC1-CHANNEL with its
prescribed fix, the verified source sites, the connect-out design hint, and the
named real-path test); the design-v4 spec you are revising,
`docs/operator/artifacts/rfc-0143-design-v4/dialogue/holder/HOLDER.md`; and the v4
verdict
`docs/operator/artifacts/rfc-0143-design-v4/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`
(read its BC1 finding + rationale + the exact "next revision must…" list for the
exact prescribed repair).

**Start from the v4 `HOLDER.md` (a required context doc).** Your revised spec MUST
**resolve BC1-CHANNEL per its prescribed fix**, and must **carry the v4-credited
resolved set forward UNREGRESSED** — re-opening or regressing any of it fails the
gate. Do NOT relitigate the ratified OQ1 trust-model shape (Option 4 mandatory floor
+ ratification-gated Option 2 narrow `CapabilityReseal` + minimal Option 3
per-session endpoint+epoch republish), the F2 non-bearer decision, **or the W1/W2/W3
wall shapes and W1's load-bearing role** — all pinned in `SEED.md`'s
`## Ratified design shape` and `## Carried forward — resolved by v4`. The wall shapes
are CORRECT; the open question is only their INSTALLATION on the real launch path.

Author the **revised falsifiable implementation spec** as your published
`HOLDER.md` artifact. This is the claim the falsifiers will RE-ATTACK and the
adjudicator will gate — make it concrete and falsifiable, not a restatement of the
RFC or the v4 spec. State every load-bearing security claim as a falsifiable
assertion paired with its named test / game-day. Re-verify every source citation
against the current worktree and FLAG any drift.

Hold the root reframe: **a boot-epoch rotation must never force a lane to choose
between reading the daemon's full-authority bootstrap admin `client-token` and
exiting silently unsealed.** A `striatum-lane` lane authenticates as its own narrow,
session-scoped credential and *never* as the shared operator admin override.

Your spec MUST:

1. **Resolve BC1-CHANNEL — pin the control-fd delivery + dumpability mechanism
   through the PRODUCTION (tmux-backed) launch path, in ONE place (the security
   cluster).** The v4 walls (W1 per-message `SCM_CREDENTIALS` peer-credentials bound
   to the launched wrapper pid+start-time; W2 `PR_SET_DUMPABLE(0)`; W3
   nonce-out-of-env) are the right walls, but the v4 spec installs them on a **direct
   `exec.Cmd.ExtraFiles` child exec** while the production lane runs inside a tmux
   pane: `launchPTY` → **`tmux respawn-pane`** (`go/pkg/supervisor/pty.go:479`) under
   **`sudo -n -u <RunAsUser> -- env -i`** (`pty.go:98-112`) wrapped by the env-file
   shim `launchEnvFileExec` (`set -a; . "$1"; rm -f -- "$1"; shift; exec "$@"`,
   `pty.go:24`). The problems to solve:
   - **(a)** an fd passed via `ExtraFiles` to the tmux CLIENT does NOT reach the
     tmux-SERVER-spawned pane process (where agentloop runs);
   - **(b)** passing fd 3 through the env-file shim makes it **live BEFORE** agentloop
     can run `PR_SET_DUMPABLE(0)` — breaking the required W2 ordering (a same-uid
     sibling can read `/proc/<wrapper-pid>/fd/3` in that window);
   - **(c)** any env-var / filesystem-socket-name / lane-readable bridge to hand off
     the fd or the nonce **reopens the exact same-uid surface BC1 must close** (the
     category mistake that killed the v1 `0600` file).

   Pin the mechanism through the real path — OR explicitly change the launch topology
   — and **name the EXACT plumbing sites that reach the PANE agentloop wrapper (NOT
   the tmux client):** `HelperLaunchSpec` (`go/pkg/supervisor/helper_protocol.go:27-39`,
   no control-fd field today), `LaunchSpec` (`go/pkg/supervisor/pty.go:30-41`, no
   `ExtraFiles`/control-fd field today), and `RunHelper`
   (`go/pkg/supervisor/helper.go:149-156`, forwards no fd today). Guarantee **NO
   same-uid-readable shim process holds fd 3 or the nonce before `PR_SET_DUMPABLE(0)`
   is effective.** Add a **REAL-PATH test
   `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper`** that launches through
   `RunHelper` with `RequireTmux`/`RunAsUser` and asserts **together**: the wrapper
   can send an accepted frame stamped with the launched wrapper pid+start-time (W1);
   the provider lacks fd 3; and a non-child/non-wrapper same-uid sibling cannot open
   `/proc/<wrapper-pid>/fd/3` OR recover the nonce at **ANY** point in the launch
   chain (W2/W3). The direct-`os/exec` versions of
   `TestControlFrameRequiresExpectedWrapperPeerCredentials` /
   `TestSiblingSameUIDCannotOpenControlFDOrReplayNonceViaProc` (carried from v4) are
   necessary but **not sufficient**.

   **Design hint (you choose — not prescriptive):** a **CONNECT-OUT topology** likely
   sidesteps the fd-through-tmux problem cleanly. The agentloop wrapper (after the
   env-file shim execs it inside the pane) calls `PR_SET_DUMPABLE(0)` **FIRST**, then
   **CONNECTS OUT** to a daemon-held listener (an abstract or filesystem unix socket);
   the daemon authenticates the connecting peer via `SO_PEERCRED` (uid + pid +
   start-time matching the LAUNCHED wrapper — `RunHelper`/tmux reports the pane pid as
   `LaunchResult.PID`, and the daemon reads its start-time once at launch from
   `/proc/<pid>/stat` field 22), so even though the socket name may be
   same-uid-reachable, a **sibling that connects is REJECTED** (wrong pid/start-time),
   and the nonce is delivered over that authenticated connection — **never via env or
   fd-inheritance-through-tmux** (there is no inherited fd to steal through tmux at
   all). You MAY instead pin a real fd-passing path if one genuinely reaches the pane
   wrapper, but you must **anchor it through the actual tmux/sudo/env-file plumbing**
   and **preserve the W2 ordering**.

2. **Fold in the v4 build-test precision item.** The daemon-observed
   "deliverable-observed" condition must NOT treat "present + absent from
   `write_scope_baseline.changed_paths`" as sufficient by itself — for per-job
   isolated worktrees the baseline is **nil**
   (`go/pkg/mutations/write_scope_guard.go:69-85`), and source-change publication
   already uses `gitChangedPathSnapshots` + `collectInScopeAuthoredPaths` authored-path
   attribution (`go/pkg/mutations/claim.go:601-630`;
   `go/pkg/mutations/artifact_source_publish.go:69-88`, `:255-290`). **Reuse that
   authored-path attribution** so an UNCHANGED pre-existing expected path is NOT
   resealed. Close it with `TestResealRequiresAuthoredExpectedArtifactChange` (seed a
   clean pre-existing expected path → assert typed floor; modify it → assert positive
   reseal) or the positive `TestCodexResealUsesReceiverNotProviderStdout` case.

3. **Carry the v4-credited resolved set forward UNREGRESSED** (verbatim where
   applicable; see `SEED.md` `## Carried forward — resolved by v4`): **BC2** (reseal
   artifact identity from the job's `expected_artifacts` daemon state, refusing
   unexpected paths), **BC3** (`CapabilityReseal` a daemon-internal marker projected
   by `resealInFlightJob`, public route-alternate test-only), **BC4** (the concrete
   `jobs.recovery_generation` column in owner bundle 0021, increment points, stamped
   value compared under the lock), **BC5** (`leases.reseal_grace_extended_at` in the
   same owner bundle 0021 — `leases` owner-held — and the corrected `work.complete`
   skip/replace/replay lock-order gate map), **C2** (the wrapper never propagates a
   provider child's 97/98 into the reserved agentloop codes), the **daemon-observed
   positive intent + recovery-sweep backstop**, the **`ensureWorkSessionBackend`
   bypass**, the **W1/W2/W3 wall shapes**, **F2** (no lane-readable reseal bearer),
   **F4** (route-alternate records `reseal` not `write`), the **F7 file-mirror half**
   (daemon-owned lane-read-only `0644` mirror, `O_NOFOLLOW`, atomic rename, reject
   MISSING boot-epoch header — closing #316), **AF1** reachability-not-reminting,
   **AF4** epoch/token decoupling, the categorical **no-admin-token-widening
   invariant**, and the **per-claim falsifiable-assertion discipline (A1–A18)** —
   extended to cover the BC1-CHANNEL real-path installation.

4. **Hold the security invariant as the spine.** Per the carried-forward set:
   `admin.BootstrapRuntimeTokenIfNeeded` (`go/pkg/admin/bootstrap.go:18-27`) grants
   the runtime client-token the FULL `bootstrapCapabilities` set
   `{admin, read, write, claim, review, apply, recovery, surgical_recovery}`, `0600`
   in a `0700` dir. Any path that lets a lane read that file, or mints a
   lane-readable credential carrying ANY of `{admin, apply, recovery,
   surgical_recovery}`, is **categorically out of bounds** — say so explicitly and
   keep it structurally impossible. The no-replay property must hold **structurally**
   on the BC1-CHANNEL channel (on the REAL tmux launch path, not on a direct-exec
   harness), not as a trackable post-clearance finding.

5. **Stay inside the product boundary and the Non-Goals.** Do NOT re-classify the
   downstream `agent_exited_unsealed` recovery policy (RFC 0152 / D249), do NOT
   change the committee POSIX-ACL repo provisioning (#537 / #539), and do NOT touch
   `run drive`'s transient-socket behavior (#513). Do NOT collide with the RFC 0125
   `HandleRecoveryReseal` worktree-durability verb (separate file, separate verb).
   Local-first, single-host, daemon-owned PostgreSQL as the single writer.

6. **Flag the maintainer ratification gate.** Slice B (the daemon-internal
   `rpc.CapabilityReseal` marker, the test-only auth-prelude route alternate, the
   daemon-owned supervisor control channel with per-message peer-credential or
   connect-out `SO_PEERCRED` authentication, the reserved agentloop exit codes, the
   `jobs.recovery_generation` + `leases.reseal_grace_extended_at` owner-bundle-0021
   columns, and endpoint/epoch republish plumbing) is a security/authz trust-model
   change. State plainly that the cleared spec is a RECOMMENDATION the maintainer
   ratifies before any build slice lands credential code, and that Slice A (the
   Option-4 floor) is zero-trust-change but must route over a real non-PTY channel
   with the same-uid authentication **anchored through the production tmux/sudo/env-file
   launch path** before it lands.

Do not treat falsifier completion as acceptance — the adjudicator's collaboration
ledger decides whether the gate clears.
