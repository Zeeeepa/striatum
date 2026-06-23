# Design-Run Seed (v5 / REVISION) — RFC 0143 Lane credential survival across a daemon boot-epoch rotation

> **THIS IS THE FIFTH REVISION (v5).** Four prior design runs ran the same
> falsification gate on this RFC. v1 (`rfc-0143-design`) returned `needs_revision`
> with seven findings F1–F7. v2 (`rfc-0143-design-v2`) **resolved F2 and F4
> cleanly** and distilled the residue into the five binding constraints BC1–BC5.
> v3 (`rfc-0143-design-v3`) **resolved BC2, BC3, BC4** (both falsifiers credited
> them) and carried the v2-credited set forward unregressed, but returned
> `needs_revision`: **BC1 stood open on three independent grounds** and **BC5 had
> two precision items**. v4 (`rfc-0143-design-v4`) **resolved BC5 (both items),
> two of BC1's three sub-grounds (C2 + the positive-intent source with the
> backend-gate bypass), and carried the v3-credited set forward unregressed**, but
> returned `needs_revision` **again** on a single, sharply-named ground:
> **BC1-CHANNEL — the W1/W2/W3 channel walls are designed for a DIRECT
> `exec.Cmd.ExtraFiles` child, but the production supervised lane is TMUX-BACKED,
> and the control-fd delivery through the real launch path is unspecified** (and
> every obvious bridge reopens the same-uid surface). The v4 cycle exhausted its
> single allowed revision; this v5 run is a **proper revision** that closes that
> one ground while carrying the entire v4-resolved set forward unregressed.
>
> The v4 design record — `dialogue/holder/HOLDER.md`,
> `dialogue/falsifier_1/FALSIFIER.md`, `dialogue/falsifier_2/FALSIFIER.md`, and
> `dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md` — is banked under
> `docs/operator/artifacts/rfc-0143-design-v4/`; the **v4** `HOLDER.md` (the spec
> being revised) and the **v4** collaboration ledger (the verdict + the full
> BC1-CHANNEL finding, its rationale, and the exact "next revision must…" list)
> are wired in as required `context_docs`.
>
> This document is the **required input** for the RFC 0143 v5 design run. It is
> operator-supplied design-run scaffolding. The canonical proposal is committed at
> `docs/rfcs/0143-lane-credential-survival-across-boot-epoch-rotation.md` (status
> `proposed`) — read it in full as your primary source; this SEED carries the
> charter, **pins the ratified design shape (do not relitigate)**, states **what
> already cleared by v4 (carry forward, do not reopen)**, and lists the **single
> binding constraint v5 MUST resolve** (BC1-CHANNEL), anchored to exact source
> sites. Read this whole file, the **v4** `HOLDER.md` + the **v4** collaboration
> ledger, and the RFC before producing any artifact.

## Framing — what this run must produce

This is a **design run**, not an implementation run. RFC 0143 is the security/authz
problem of **lane credential survival across a daemon boot-epoch rotation**: when a
lane loses its live RPC connection across a daemon restart, its
credential-resolution chain falls through to the full-authority bootstrap admin
`client-token` (which a `striatum-lane` lane cannot read), so a complete-on-disk
deliverable exits unsealed with a misleading permission error. The deliverable of
this run is a **falsifiable implementation spec** the `rfc-0143-build` run can
execute contract-first (TDD), produced by hardening the v4 spec against the one
remaining adversarial ground.

The v4 falsification gate found the v4 design **`needs_revision`** on a single
ground: BC1-CHANNEL. The holder must produce a proposal that **resolves
BC1-CHANNEL** while **carrying the v4-credited resolved set forward unregressed**.
A revised spec that leaves BC1-CHANNEL open — or regresses any carried-forward item
(BC2, BC3, BC4, BC5, C2, the daemon-observed positive intent, the backend-gate
bypass, the W1/W2/W3 wall *shapes*, F2, F4, the F7 file-mirror half, AF1, AF4, the
no-admin-token-widening invariant, the A1–A18 discipline) — has NOT cleared the
gate. This is the gate's single allowed revision cycle for v5, so a second
`needs_revision` ends the gate uncleared. The fix must land in **one coherent
proposal**, inside the **security/lifecycle channel** the v4 spec already pinned
(the daemon-internal `resealInFlightJob` mutation, the trusted-wrapper exit code,
the daemon-owned control channel, the `jobs.recovery_generation` column +
`leases.reseal_grace_extended_at` in owner bundle 0021, the numeric grace + the
corrected lock order).

## Ratified design shape (do NOT relitigate)

The maintainer has ratified the trust-model shape and the F2 replay defense; these
are binding and **override any softer framing**. No prior gate cycle contested them —
do not reopen them, build on them:

- **OQ1 — trust-model shape (ratified): Option 4 + ratification-gated Option 2 +
  minimal Option 3.**
  - **Slice A (mandatory, lands first, ZERO trust-model change):** Option 4 — a
    legible, self-escalating `session_unrecoverable_across_rotation` signal
    replacing the silent unsealed exit. This is the floor; it must be buildable and
    valuable on its own. **Per the still-open BC1-CHANNEL it must route over a real,
    non-PTY-output channel whose same-uid authentication is anchored through the
    production tmux/sudo/env-file launch path before it lands.**
  - **Slice B (ratification-gated):** Option 2's *narrow* reseal authority — a
    session-scoped `CapabilityReseal` covering ONLY the in-flight job's seal
    (`work.complete` / `artifact.publish` / `interrogation.answer`), **never** any
    of `{admin, apply, recovery, surgical_recovery}` and **never plain `write`** —
    folding in a minimal Option 3 per-session endpoint+epoch republish so the lane
    never needs to read the admin `client-token`.
- **F2 — replay defense (DECIDED): non-bearer, daemon-owned, session-tied channel.
  NO readable reseal token file.** Because all lanes currently share the
  `striatum-lane` OS user, a `0600` reseal *file* is a same-uid replay surface
  readable by sibling lanes. The ratified resolution: deliver/verify the
  `CapabilityReseal` authority over the **daemon-owned supervisor session-tied
  channel** — there is NO lane-readable reseal token file at all. The daemon proves
  the calling session, not a bearer file. Do NOT reintroduce a readable bearer file
  as the reseal credential under any option. **v5 extends the same-uid threat model
  to the channel's INSTALLATION on the real tmux launch path and closes it
  structurally (BC1-CHANNEL).**
- **Slice B requires maintainer ratification before any build slice touches
  credential code.** Adjudicator clearance gates the spec's *soundness*, not the
  maintainer's product call. Slice A is zero-trust-change and may land first under
  the normal review gate **once BC1-CHANNEL anchors its same-uid-authenticated
  control channel through the production launch path** — which v5 must do.

## Carried forward — resolved by v4 (do NOT reopen)

> The v4 collaboration ledger records the following as genuinely resolved / sound;
> **both v4 falsifiers credited them and confirmed they are NOT regressed.** The v5
> revision MUST preserve them — verbatim from the **v4** `HOLDER.md` where
> applicable — and the cycle-5 adjudicator's clearing verdict requires them intact.
> Re-opening any of these is a regression that fails the gate.

- **BC2 — RESOLVED (artifact identity from daemon state).** `resealInFlightJob`
  derives the expected-artifact set from the job's `expected_artifacts` (daemon
  state, attempt-resolved via `resolveExpectedArtifactCycles`), reuses
  `verifyRequiredArtifacts` / `ensurePerJobPublishedArtifactsDurable`
  (`go/pkg/mutations/mutations.go:828-876`), publishes only a `path` that is an open
  expected entry from the job's own worktree, and **refuses any unexpected path**;
  the signal supplies neither path nor content, and a front-matter/author-line
  failure routes to the Option-4 floor rather than a silent drop. v4 added the
  POSITIVE case (a complete-on-disk deliverable post-rotation IS automatically
  resealed by the daemon-observed condition). Keep
  `TestCodexResealUsesReceiverNotProviderStdout` (both the negative and positive
  cases).
- **BC3 — RESOLVED (`CapabilityReseal` is a daemon-internal marker).**
  `resealInFlightJob` maps `supervisor_id` → `session_id` from the supervision row,
  constructs an **internal** `rpc.AuthContext{Capability: CapabilityReseal,
  SessionID, RepositoryID}` **without** the public `Authorize` prelude
  (`go/pkg/rpc/server.go:107-111`), and calls the lower-level publish/complete
  routines against the active worktree; the public route-alternate
  (`MethodEntry.ResealAlternate` on only `interrogation.answer` / `work.complete` /
  `artifact.publish`, recording `reseal` not `write`) is kept **test-only** since no
  production bearer exists. `registry_methods.go` is generated; the
  `command-authority-matrix` reseal column + the authority guardrail are updated.
  Keep `TestResealCapabilityIsDaemonInternalNotBearer` /
  `TestResealTokenCanReachOnlyResealRoutesWithoutWrite`.
- **BC4 — RESOLVED (concrete monotonic generation column).** The concrete
  `jobs.recovery_generation` column ships in owner bundle
  `go/pkg/db/sql/owner/0021_job_recovery_generation.sql`
  (`ALTER TABLE striatumd.jobs ADD COLUMN IF NOT EXISTS recovery_generation integer
  NOT NULL DEFAULT 0`), bumps `LatestOwnerBundleVersion` 20→21
  (`go/pkg/db/owner.go:23` — **currently 20**, re-verify) with the ordinal-21
  `RESERVATIONS.toml` reservation, modelled exactly on the credited
  `review_generation` precedent (`go/pkg/db/sql/owner/0009_review_generation.sql`);
  `striatumd.jobs` is owner-held. A degrade-safe `JobRecoveryGenerationColumnPresent`
  probe routes to the typed floor when the column is absent. The four increment
  points (claim, requeue-same-attempt, recovery-sweep expire/transfer/respawn,
  release), each in the same UPDATE that retires/rebinds the authoritative lease
  under `lockRun`, are named; the post-increment value is stamped into
  `work_packets.packet_json` `lease.recovery_generation`, compared equal/unequal at
  reseal under the lock (mismatch → typed class). Keep
  `TestResealPredicateUsesStampedRecoveryGeneration` /
  `TestResealTokenRefusedAfterLeaseExpiryAndRecoveryRequeue`.
- **BC5 — RESOLVED (pinned migration site + corrected lock order).** Both v3
  precision items are fixed and both v4 falsifiers (and the adjudicator) credited
  the close:
  1. **Migration site PINNED.** `leases.reseal_grace_extended_at timestamptz` is
     added in the **same owner bundle 0021** as `jobs.recovery_generation`.
     `striatumd.leases` is **owner-held** — created in runtime migration `0005`
     (`go/pkg/db/sql/0005_repo_local_workflow_state.sql:166`) and **absent** from the
     migration-0016+ ownership-transfer cohort
     (`go/pkg/db/sql/owner/0018_runtime_table_ownership_transfer.sql`) — so the
     column-add is owner DDL, consistent with
     `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL` /
     `TestRuntimeMigrationsDoNotCarryOwnerDDLForLeasesGraceColumn`. (Like
     `review_generation`, `striatumd_rw`'s table-level grants extend to the new
     column; no new grant.)
  2. **Lock order CORRECTED.** `HandleCompleteWork` runs
     `enforceSessionBindingForSession` + `enforceActiveActingSession` **before**
     `lockRunForJob`, and `activeLeaseFor` + `ensureWorkSessionBackend` **after**
     (`go/pkg/mutations/lifecycle.go:1124-1182`); `artifact.publish` takes
     `lockRunForJob` first (`go/pkg/mutations/artifact.go:75-85`); the sweep
     drains→locks→expires (`go/pkg/mutations/recovery.go:575-621`).
     `resealInFlightJob` does NOT call `HandleCompleteWork` — it is a distinct
     mutation with the exact **skip/replace** set (`enforceSessionBindingForSession`,
     `enforceActiveActingSession`, `activeLeaseFor`, `ensureWorkSessionBackend`) and
     **replay** set (`lockRunForJob`, the `FOR UPDATE` rows, the reseal predicate,
     write-scope, durability, the `running→completed` transition), plus the
     reseal-vs-sweep serialization, so expired-beyond-grace ALWAYS routes the typed
     `session_unrecoverable_across_rotation` class (never a raw `lease_error`, never a
     revived requeued lease). Keep `TestResealBeyondGraceRoutesTypedNotLeaseError` /
     `TestResealGraceCannotReviveRequeuedLease` /
     `TestRecoveryRequeueWinsOverExpiredLeaseReseal` / `GD-1b` /
     `TestResealExit98BypassesBackendGateOrRoutesTyped`.
- **C2 — RESOLVED (reserved exit codes reserved BY COMMITMENT).** The wrapper is
  committed to **never propagate a provider child's status 97/98 into the reserved
  agentloop codes** (after `cmd.Wait()` it remaps a child 97/98 to a non-control
  `agent_exited` carrying the provider's raw status in a non-reserved payload field);
  the agentloop owns 97/98 semantics exclusively. Keep
  `TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker`.
- **Daemon-OBSERVED positive intent + the recovery-sweep backstop — RESOLVED.** The
  v3 positive-intent gap is replaced by SEED option (a): `resealInFlightJob` fires
  only on a **daemon-observed** post-rotation condition — boot-epoch rotated since
  the packet + the job still running with the stamped `recovery_generation` matching
  live + the lease within grace + every required `expected_artifact` present **and
  content-hash-modified since the `write_scope_baseline`** — never on a
  provider-asserted signal. Two entry points (the wrapper's post-rotation exit; the
  **recovery-sweep backstop** for "the wrapper can't even signal"), one condition,
  one lock. Keep the positive `TestCodexResealUsesReceiverNotProviderStdout`.
- **The `ensureWorkSessionBackend` BYPASS (backend-gate routing) — RESOLVED.**
  `resealInFlightJob` deliberately bypasses `ensureWorkSessionBackend`
  (`lifecycle.go:1181`) — the reseal exists *precisely because* the live connection
  is gone — so a stopped supervisor routes the typed
  `session_unrecoverable_across_rotation` class rather than leaking
  `invalid_transition`/backend errors. Keep
  `TestResealExit98BypassesBackendGateOrRoutesTyped`.
- **The W1/W2/W3 wall SHAPES — RESOLVED IN SHAPE (only their INSTALLATION is the open
  BC1-CHANNEL ground).** The three structural walls are the right walls and are
  carried forward as the channel's authentication design: **W1** — per-message
  kernel-stamped peer credentials binding every control frame to the *launched
  wrapper's* pid + start-time (the kernel refuses an unprivileged process forging
  another pid's `ucred`); **W2** — `PR_SET_DUMPABLE(0)` on the wrapper (reinforced by
  the `sudo` setuid launch) so the `/proc` fd/env surface is root-owned; **W3** — the
  single-use control nonce delivered out of the env entirely (over the channel), so it
  never appears in `/proc/<wrapper-pid>/environ`. **Do not relitigate the wall shapes
  or W1's load-bearing role.** The ONLY open question is BC1-CHANNEL: how these walls
  are *installed* on the production tmux/sudo/env-file launch path (see below).
- **F2 — RESOLVED (bearer-file retirement).** No lane-readable reseal bearer exists;
  the v1 `0600` same-uid file replay stays retired. Keep
  `TestBorrowedResealBearerCannotSealVictimSession`.
- **F4 — RESOLVED (auth mechanism without plain `write`).** The route-specific
  `MethodEntry.ResealAlternate` admits `CapabilityReseal` on **only** the three routes
  and records `AuthContext.Capability == reseal` (never `write`). Keep
  `TestResealTokenCanReachOnlyResealRoutesWithoutWrite` and the
  command-authority-matrix reseal column. Preserve this wiring exactly.
- **F7 file-mirror half — RESOLVED.** Endpoint/epoch moves to a **daemon-owned,
  lane-read-only `0644` file** with `O_NOFOLLOW` symlink defense and atomic
  temp-and-rename, and a supervised request with a **MISSING** boot-epoch header is
  **rejected** — closing the permissive header-absent #316 path on the supervised
  path. Keep `TestResealEpochMirrorRejectsTamperOrMissingEpoch`. (The *channel* half
  of F7 inherits the open BC1-CHANNEL until v5 anchors the channel.)
- **AF1 — reachability-not-reminting.** The session-bound token stays *valid* across
  a restart; only its *reachability* breaks. The fix is **routing**, not re-minting.
  Keep `TestTokenValidAcrossRestart`.
- **AF4 — epoch/token decoupling.** Endpoint rotation and boot-epoch rotation are
  coupled; #316 deliberately retires a surviving lane's connection. The token does
  NOT rotate on a normal restart (only the endpoint does). Preserve this framing.
- **The categorical no-admin-token-widening invariant.** No lane ever reads the
  daemon's full-authority bootstrap admin `client-token`
  (`go/pkg/admin/bootstrap.go:18-27` grants
  `{admin,read,write,claim,review,apply,recovery,surgical_recovery}`); no minted
  credential carries any of `{admin, apply, recovery, surgical_recovery}`. Held +
  strengthened by never materializing `CapabilityReseal` into any lane-readable file.
  Keep `TestResolveRefusesRuntimeClientTokenForLane`.
- **The per-claim falsifiable-assertion discipline (A1–A18).** Every load-bearing
  claim is paired with the named test / game-day that refutes it. Extend it to cover
  the BC1-CHANNEL real-path installation; do not abandon it.

## Ratified design shape — the resealInFlightJob channel (do NOT relitigate)

The v4 spec pinned (and both falsifiers credited) the following channel design; v5
**preserves it** and only resolves where it attaches on the real launch path:

- A **daemon-internal `resealInFlightJob` mutation**
  (`go/pkg/mutations/recovery_reseal_rotation.go`, deliberately distinct from the RFC
  0125 `HandleRecoveryReseal` worktree-durability verb in
  `go/pkg/mutations/recovery_reseal.go`).
- Two reserved agentloop exit codes (new, `go/pkg/agentloop/exitcodes.go`):
  `ExitUnrecoverableAcrossRotation = 97` (the Option-4 floor) and
  `ExitResealInFlightRequested = 98` (a latency hint only; its forgeability is
  immaterial — the daemon never seals on the strength of 98).
- The helper already captures the wrapper's **OS process exit status** into
  `agentExitPayload` → `processExitCode` (`go/pkg/supervisor/helper.go:427-439`,
  never read from stdout/stderr); the daemon recognises reserved codes in the
  existing `agent_exited` branch of `recordSuperviseReportEvent`
  (`go/pkg/mutations/supervision.go:298-306`).
- The W1/W2/W3 wall shapes (above) authenticate any richer control frame; the frame
  schema carries NO job_id/artifact path/body — identity is derived from daemon
  state (BC2).

## The binding constraint v5 MUST resolve (the v4 adjudicator's sole unrebutted ground)

> The v4 ledger §"What the next (out-of-run) revision MUST fix" pins the exact
> repair. This SEED carries it verbatim in shape. It names exact source sites; anchor
> every load-bearing claim in the revised spec to them, paired with the named test.

### BC1-CHANNEL — the W1/W2/W3 walls are correct, but the control-fd delivery through the PRODUCTION (tmux-backed) launch path is unspecified, and every obvious bridge reopens the same-uid surface (closes F1 / F6 / the channel half of F2 / F7-channel)

The v4 walls (W1 per-message `SCM_CREDENTIALS` peer-credentials bound to the launched
wrapper pid+start-time; W2 `PR_SET_DUMPABLE(0)`; W3 nonce-out-of-env) are the **right**
walls — both v4 falsifiers and the adjudicator credited their shape. But the v4 spec
installs them via a **DIRECT `exec.Cmd.ExtraFiles` child exec**, while the **production
supervised lane is TMUX-BACKED**. The production lane launches via `launchPTY` →
**`tmux respawn-pane`** under **`sudo -n -u <RunAsUser> -- env -i`**, wrapped by the
env-file shell shim (`launchEnvFileExec`). The problems the v4 spec does not solve:

1. **An fd passed via `ExtraFiles` to the tmux CLIENT does NOT reach the
   tmux-SERVER-spawned pane process** (where agentloop runs). `respawn-pane` runs the
   lane command under the long-lived tmux server, not as a child of the launching
   client.
2. **Passing fd 3 through the env-file shim makes it live BEFORE agentloop can run
   `PR_SET_DUMPABLE(0)`** — the required W2 ordering breaks. A same-uid sibling can
   read `/proc/<wrapper-pid>/fd/3` in that window (the shim runs before agentloop's
   first instruction).
3. **Any env-var / filesystem-socket-name / lane-readable bridge** to hand off the fd
   or the nonce **reopens the exact same-uid surface BC1 exists to close** — the same
   category mistake that killed the v1 `0600` file.

So the v4 named tests (`TestControlFrameRequiresExpectedWrapperPeerCredentials`,
`TestSiblingSameUIDCannotOpenControlFDOrReplayNonceViaProc`) can pass on a **direct
`os/exec` harness** while the **real tmux lane never receives fd 3** — the load-bearing
test would NOT fire against the production path. Structural no-replay must hold on the
REAL channel, not on a direct-exec harness.

**Verified source sites (CONFIRM against current `main`, FLAG any drift):**

- `HelperLaunchSpec` has **NO** control-fd field (`go/pkg/supervisor/helper_protocol.go:27-39`).
- `LaunchSpec` has no `ExtraFiles`/control-fd field (`go/pkg/supervisor/pty.go:30-41`).
- The run-as path is `sudo -n -u RunAsUser -- env -i` with the env-file shim
  `launchEnvFileExec` (`go/pkg/supervisor/pty.go:24`, `:98-112`).
- `launchPTY` uses `tmux respawn-pane` (`go/pkg/supervisor/pty.go:479`).
- `RunHelper` forwards only command/env/wd/run-as/tmux into `LaunchSpec`
  (`go/pkg/supervisor/helper.go:149-156`).
- There is **NO** `socketpair` / `SCM_CREDENTIALS` / `SO_PASSCRED` / `SO_PEERCRED` /
  `PR_SET_DUMPABLE` / `ExtraFiles` primitive anywhere in `go/pkg/supervisor` or
  `go/pkg/agentloop` today (independently re-confirmed).

**Prescribed fix (the v5 holder must do, in ONE place):** pin the control-fd delivery
+ dumpability mechanism through the production tmux / sudo run-as / env-file path — OR
explicitly change the launch topology — and **name the EXACT
`HelperLaunchSpec`/`LaunchSpec`/`RunHelper` plumbing sites** that reach the **pane
agentloop wrapper** (NOT the tmux client). Guarantee **NO same-uid-readable shim
process holds fd 3 or the nonce before `PR_SET_DUMPABLE(0)` is effective** (resolve
the ordering: the env-file shim runs before agentloop's first instruction). Add a
**REAL-PATH test** `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` that launches
through `RunHelper` with `RequireTmux`/`RunAsUser` and asserts **together**: the
wrapper can send an accepted frame stamped with the launched wrapper pid+start-time
(W1); the provider lacks fd 3; and a non-child/non-wrapper same-uid sibling cannot
open `/proc/<wrapper-pid>/fd/3` OR recover the nonce at **ANY** point in the launch
chain (W2/W3). The direct-`os/exec` versions of
`TestControlFrameRequiresExpectedWrapperPeerCredentials` /
`TestSiblingSameUIDCannotOpenControlFDOrReplayNonceViaProc` are necessary but **not
sufficient**.

**Design hint (the holder CHOOSES — not prescriptive):** a **CONNECT-OUT topology**
likely sidesteps the fd-through-tmux problem cleanly. The agentloop wrapper (after the
env-file shim execs it inside the pane) calls `PR_SET_DUMPABLE(0)` **FIRST**, then
**CONNECTS OUT** to a daemon-held listener (e.g. an abstract or filesystem unix
socket); the daemon authenticates the connecting peer via `SO_PEERCRED` (uid + pid +
start-time matching the launched wrapper), so even though the socket name may be
same-uid-reachable, a **sibling that connects is REJECTED** (wrong pid/start-time), and
the nonce is delivered over that authenticated connection — **never via env or
fd-inheritance-through-tmux**. (Note: with a connect-out, the daemon learns the
wrapper's pid from the launch — `RunHelper`/tmux reports the pane pid as
`LaunchResult.PID` — and reads its start-time once at launch from `/proc/<pid>/stat`
field 22, then accepts the FIRST connection whose `SO_PEERCRED` matches; later
connections from any other pid are refused. This makes the connecting wrapper, not a
sibling, the authenticated peer — and there is no inherited fd to steal through tmux at
all.) The holder MAY instead pin a real fd-passing path if one genuinely reaches the
pane wrapper (e.g. via a tmux-server-side mechanism), but must **anchor it through the
actual tmux/sudo/env-file plumbing** and **preserve the W2 ordering** (no same-uid
shim holds fd 3 or the nonce before `PR_SET_DUMPABLE(0)`). Either way, name the exact
plumbing sites and add the real-path test.

BC1-CHANNEL is the security cluster's load-bearing closure (F1 / F6 and the F2 / F7
channel halves all inherit it); the security invariant must hold **structurally** —
no-replay must hold structurally on the REAL channel, not as a trackable
post-clearance finding.

## Also fold in (the v4 build-test precision item, non-blocking but required in the spec)

Fold in falsifier-reviewer-002's **modified-since-baseline build-test**: the
"deliverable observed" condition (BC2/positive-intent) must NOT treat "present +
absent from `write_scope_baseline.changed_paths`" as sufficient by itself — for
per-job isolated worktrees the baseline is **nil** (`go/pkg/mutations/write_scope_guard.go:69-85`),
and source-change publication already uses `gitChangedPathSnapshots` +
`collectInScopeAuthoredPaths` authored-path attribution
(`go/pkg/mutations/claim.go:601-630`;
`go/pkg/mutations/artifact_source_publish.go:69-88`, `:255-290`). The build must
**reuse that authored-path attribution** so an UNCHANGED pre-existing expected path is
NOT resealed. Close it with `TestResealRequiresAuthoredExpectedArtifactChange` (seed a
clean pre-existing expected path → assert typed floor; modify it → assert positive
reseal) or the positive `TestCodexResealUsesReceiverNotProviderStdout` case.

## Clearing condition for this revision

The adjudicator clears the gate only if **BC1-CHANNEL is genuinely resolved** with a
concrete mechanism anchored through the production launch path (the exact
`HelperLaunchSpec`/`LaunchSpec`/`RunHelper` sites that reach the pane wrapper, the
preserved W2 ordering so no same-uid shim holds fd 3 or the nonce before
`PR_SET_DUMPABLE(0)`, and the named real-path test
`TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` that would actually fire against
`RequireTmux`/`RunAsUser`), **AND structural no-replay is established on the REAL
channel**, **AND the whole v4-credited resolved set is carried forward unregressed**
(BC2, BC3, BC4, BC5, C2, the daemon-observed positive intent, the backend-gate bypass,
the W1/W2/W3 wall shapes, F2, F4, the F7 file-mirror half, AF1, AF4, the
no-admin-token-widening invariant, the A1–A18 assertion discipline) with the
modified-since-baseline build-test folded in, **AND no new material challenge** stands
unrebutted. The verdict is `reject` only if a path widens admin-token exposure or mints
a credential carrying any of `{admin, apply, recovery, surgical_recovery}`; otherwise
`needs_revision` if BC1-CHANNEL remains open (a "fix" that still leaves fd 3 / the
nonce same-uid-readable before `PR_SET_DUMPABLE(0)`, that does not reach the pane
wrapper through the real tmux/sudo path, or whose real-path test would not actually
fire), if any credited item is regressed, or any new material challenge lands. One
revision cycle is available within this run; the falsifiers re-attack the revised spec.

## Maintainer-ratification note (carries regardless of verdict)

Slice B — the daemon-internal `rpc.CapabilityReseal` marker, the test-only auth-prelude
route alternate, the daemon-owned supervisor control channel with per-message
peer-credential (or connect-out `SO_PEERCRED`) authentication, the reserved agentloop
exit codes, the `jobs.recovery_generation` + `leases.reseal_grace_extended_at`
owner-bundle-0021 columns, and the endpoint/epoch republish plumbing — is a
**security/authz trust-model change requiring maintainer ratification before any build
slice touches credential code**. Adjudicator clearance gates the spec's **soundness**,
not the maintainer's product call. Slice A (the Option-4 typed-exit-code floor) is
zero-trust-change but, per BC1-CHANNEL, still must route over a real, non-PTY channel
**anchored through the production tmux/sudo/env-file launch path** before it lands.

---
<sub>Operator scaffold for the RFC 0143 falsification-gate design run (v5 / REVISION
of `rfc-0143-design-v4`; resolves the single remaining binding constraint BC1-CHANNEL —
pinning the control-fd / connect-out delivery + dumpability mechanism through the
production tmux/`respawn-pane`/`sudo -- env -i`/env-file launch path so the W1/W2/W3
walls are installed on the REAL channel (pane wrapper, not the tmux client), the W2
ordering preserved, with the named real-path test
`TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` — and carries the v4-credited set
(BC2/BC3/BC4/BC5 + C2 + daemon-observed positive intent + backend-gate bypass +
W1/W2/W3 shapes + F2/F4/F7-file/AF1/AF4/no-widening/A1–A18) forward unregressed, folding
in the modified-since-baseline authored-path build-test). Lanes: author=claude
(holder/adjudicator/committer), reviewer=codex (falsifiers).</sub>
