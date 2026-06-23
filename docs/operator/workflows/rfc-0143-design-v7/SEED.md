# Design-Run Seed (v7 / REVISION) — RFC 0143 Lane credential survival across a daemon boot-epoch rotation

> **THIS IS THE SEVENTH REVISION (v7).** Six prior design runs ran the same
> falsification gate on this RFC. v1 (`rfc-0143-design`) returned `needs_revision`
> with seven findings F1–F7. v2 (`rfc-0143-design-v2`) **resolved F2 and F4
> cleanly** and distilled the residue into the five binding constraints BC1–BC5.
> v3 (`rfc-0143-design-v3`) **resolved BC2, BC3, BC4** and carried the v2-credited
> set forward unregressed. v4 (`rfc-0143-design-v4`) **resolved BC5, two of BC1's
> three sub-grounds (C2 + the daemon-observed positive intent with the backend-gate
> bypass)**, but returned `needs_revision` on a single ground: **BC1-CHANNEL — the
> W1/W2/W3 walls were designed for a DIRECT `exec.Cmd.ExtraFiles` child, but the
> production supervised lane is TMUX-BACKED, and the control-fd delivery through the
> real launch path was unspecified.** v5 (`rfc-0143-design-v5`) **RESOLVED the big
> v4→v5 rework**: it **deleted** the inherited-fd-through-`ExtraFiles` channel and
> adopted the **CONNECT-OUT topology** (the pane wrapper dials OUT after
> `PR_SET_DUMPABLE(0)`; no fd crosses the tmux client/server boundary; non-secret
> listener address; post-auth nonce), flagged and fixed a real v4 drift (the
> `#{pane_dead_status}` exit-code backstop), and carried the v4-credited set forward
> unregressed. **Both v5 falsifiers credited the topology, the named plumbing sites,
> the W2 ordering, and the real-path test shape.** But v5 returned `needs_revision`
> **again** on a single, sharply-named ground that BOTH falsifiers landed
> **independently**: **BC1-W1-TOKEN — W1's peer-credential proof compares two
> CATEGORICALLY DIFFERENT CLOCKS.** v6 (`rfc-0143-design-v6`) **RESOLVED
> BC1-W1-TOKEN** (the clock-domain operand defect): it pins ONE coherent kernel
> start-token source — `LaunchResult.PaneKernelStartToken` captured via
> `ProcessStartToken(identity.PanePID)` immediately after `CaptureTmuxIdentity`
> reports the pane pid, compared field-22-to-field-22 against the accepted peer's
> `ProcessStartToken(peer.pid)`, keeping tmux `#{pane_start_time}` as liveness
> metadata only, and closing the empty-token fail-open trap with a fail-closed
> `!= ""` assertion. **Both v6 falsifiers credited the BC1-W1-TOKEN clock-domain
> fix, the empty-token rule, and the real-path test shape improvements, and found NO
> regression of the v5-credited set.** But v6 returned `needs_revision` **again** on
> a single, sharply-named ground that BOTH falsifiers landed **independently**:
> **BC1-W1-CAPTURE — the kernel start token is captured POST-LAUNCH and is not
> structurally proven to belong to the born wrapper.** This v7 run is a **proper
> revision** that closes that one capture-boundary ground while carrying the entire
> v6-resolved set forward unregressed.
>
> The v6 design record — `dialogue/holder/HOLDER.md`,
> `dialogue/falsifier_1/FALSIFIER.md`, `dialogue/falsifier_2/FALSIFIER.md`, and
> `dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md` — is banked under
> `docs/operator/artifacts/rfc-0143-design-v6/`; the **v6** `HOLDER.md` (the spec
> being revised) and the **v6** collaboration ledger (the verdict + the full
> BC1-W1-CAPTURE finding, its rationale, and the exact "next revision must…" list)
> are wired in as required `context_docs`.
>
> This document is the **required input** for the RFC 0143 v7 design run. It is
> operator-supplied design-run scaffolding. The canonical proposal is committed at
> `docs/rfcs/0143-lane-credential-survival-across-boot-epoch-rotation.md` (status
> `proposed`) — read it in full as your primary source; this SEED carries the
> charter, **pins the ratified design shape (do not relitigate)**, states **what
> already cleared by v6 (carry forward, do not reopen)**, and lists the **single
> binding constraint v7 MUST resolve** (BC1-W1-CAPTURE), anchored to exact source
> sites. Read this whole file, the **v6** `HOLDER.md` + the **v6** collaboration
> ledger, and the RFC before producing any artifact.

> **Convergence note:** The W1 no-replay wall has yielded progressively finer
> same-uid races across cycles: BC1-CHANNEL (connect-out topology) → BC1-W1-TOKEN
> (clock-domain operand) → BC1-W1-CAPTURE (capture-boundary TOCTOU). The v7
> capture-boundary re-verification fix is concrete and bounded and is expected to
> close the structural binding of the launched wrapper's identity to the kernel token.
> This is stated as the convergence framing; no new mechanism beyond the prescribed
> capture-boundary fix is added.

## Framing — what this run must produce

This is a **design run**, not an implementation run. RFC 0143 is the security/authz
problem of **lane credential survival across a daemon boot-epoch rotation**: when a
lane loses its live RPC connection across a daemon restart, its
credential-resolution chain falls through to the full-authority bootstrap admin
`client-token` (which a `striatum-lane` lane cannot read), so a complete-on-disk
deliverable exits unsealed with a misleading permission error. The deliverable of
this run is a **falsifiable implementation spec** the `rfc-0143-build` run can
execute contract-first (TDD), produced by hardening the v6 spec against the one
remaining adversarial ground.

The v6 falsification gate found the v6 design **`needs_revision`** on a single
ground: **BC1-W1-CAPTURE** — the kernel start token is captured POST-LAUNCH and is
not structurally proven to belong to the born wrapper. The holder must produce a
proposal that **resolves BC1-W1-CAPTURE in ONE place** while **carrying the
v6-credited resolved set forward unregressed**. A revised spec that leaves
BC1-W1-CAPTURE open — or regresses any carried-forward item (the v6 BC1-W1-TOKEN
clock-domain fix including `LaunchResult.PaneKernelStartToken` + the empty-token
fail-closed `!= ""` rule + A3'/A3''; the connect-out topology, the named plumbing
sites, the non-secret address + post-auth nonce W3, the W2 ordering +
dumpable-before-dial, the `#{pane_dead_status}` exit-code backstop, C2, BC2, BC3,
BC4, BC5, the daemon-observed positive intent, the backend-gate bypass, the
W1/W2/W3 wall *shapes*, F2, F4, the F7 file-mirror half, AF1, AF4, the
no-admin-token-widening invariant, the A1–A18 discipline) — has NOT cleared the
gate. This is the gate's single allowed revision cycle for v7, so a second
`needs_revision` ends the gate uncleared. The fix must land in **one coherent
proposal**, inside the **security/lifecycle channel** the v6 spec already pinned
(the daemon-internal `resealInFlightJob` mutation, the connect-out control channel,
the reserved agentloop exit codes, the `jobs.recovery_generation` column +
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
    valuable on its own. **Per the still-open BC1-W1-CAPTURE it must route over the
    connect-out, non-PTY-output channel whose same-uid authentication (W1) is
    specified with ONE coherent kernel identity token STRUCTURALLY BOUND to the
    launched wrapper before it lands.**
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
  as the reseal credential under any option. **v5 closed the channel's INSTALLATION
  on the real tmux launch path structurally via the connect-out topology; v6 closed
  the channel's AUTHENTICATION primitive (W1) by pinning one coherent kernel
  start-token source (BC1-W1-TOKEN); v7 closes the channel's IDENTITY BINDING by
  requiring capture-boundary re-verification via tmux/liveness before the kernel
  token is stamped (BC1-W1-CAPTURE).**
- **Connect-out topology + the W1/W2/W3 wall SHAPES (ratified by the v5 gate).** The
  connect-out channel and the three structural walls are the right walls and are
  carried forward as the channel's authentication design — see
  `## Carried forward — resolved by v6`. Do NOT relitigate the topology or W1's
  load-bearing role. **The ONLY open question is the CAPTURE BOUNDARY of W1's kernel
  token (BC1-W1-CAPTURE, see below).**
- **Slice B requires maintainer ratification before any build slice touches
  credential code.** Adjudicator clearance gates the spec's *soundness*, not the
  maintainer's product call. Slice A is zero-trust-change and may land first under
  the normal review gate **once BC1-W1-CAPTURE structurally binds the kernel token
  to the launched wrapper** — which v7 must do.

## Carried forward — resolved by v6 (do NOT reopen)

> The v6 collaboration ledger records the following as genuinely resolved / sound;
> **both v6 falsifiers credited them and confirmed they are NOT regressed.** The v7
> revision MUST preserve them — verbatim from the **v6** `HOLDER.md` where
> applicable — and the cycle-7 adjudicator's clearing verdict requires them intact.
> Re-opening any of these is a regression that fails the gate.

- **BC1-W1-TOKEN — RESOLVED by v6 (carry the clock-domain operand fix forward).**
  v6 adds `LaunchResult.PaneKernelStartToken` (`pty.go:47-53`) and captures it via
  `ProcessStartToken(identity.PanePID)` in `launchPTY` (`pty.go:493-504`)
  immediately after `CaptureTmuxIdentity` reports the pane pid. W1 compares the
  accepted peer's `ProcessStartToken(peer.pid)` to `result.PaneKernelStartToken` via
  `PIDLiveWithStartToken(peer.pid, result.PaneKernelStartToken)`
  (`tmux_liveness.go:392-408`) — field-22 on both sides, one clock domain. tmux
  `#{pane_start_time}` is kept ONLY as liveness metadata (`identity.PaneStartToken`,
  `ProbeTmuxLiveness`), not claimed equivalent to `/proc` field 22, excluded from W1
  by construction. The empty-token fail-open trap is closed: `PIDLiveWithStartToken(pid, "")`
  skips the token comparison (`tmux_liveness.go:397`) and returns live on pid alone,
  so `RunHelper` asserts `result.PaneKernelStartToken != ""` before accepting any
  control connection. The same-pid stale-token negative (A3') and the empty-token
  negative (A3'') are both credited. DO NOT remove `LaunchResult.PaneKernelStartToken`,
  the empty-token `!= ""` guard, or A3'/A3''. The v7 capture-boundary fix BUILDS ON
  this; the field-22/field-22 operand source remains correct and must be retained.
- **BC1-CHANNEL — RESOLVED by the connect-out rework (carry the whole topology).**
  v5 **deleted** the v4 inherited-fd-through-`exec.Cmd.ExtraFiles` channel and
  adopted a **CONNECT-OUT topology** anchored on the production `tmux respawn-pane` /
  `sudo … env -i` / env-file launch path. The pane agentloop wrapper calls
  `PR_SET_DUMPABLE(0)` **FIRST**, then **dials OUT** to a daemon-held `SO_PASSCRED`
  `SOCK_SEQPACKET` listener at a **non-secret** abstract address advertised via the
  existing env plumbing (`STRIATUM_SUPERVISOR_CONTROL_ADDR`). **No fd crosses the
  tmux client/server boundary; the W2 ordering is trivially satisfied because no
  fd/nonce exists before agentloop runs.** Named plumbing sites (carry verbatim):
  add `ControlSocketAddr` to `HelperLaunchSpec` (`helper_protocol.go:27-39`) and
  `LaunchSpec` (`pty.go:30-42`); `RunHelper` (`helper.go:128`, `:149-156`) creates
  the listener, captures the pane identity, runs `acceptControlChannel`; new
  agentloop `dumpable_linux.go` / `control_channel.go` / `exitcodes.go`. The
  **non-secret address** (a sibling that learns it and connects is refused on
  pid/start-time) and the **post-auth nonce delivered daemon→wrapper** (W3, never via
  env or fd-inheritance) are credited and carried.
- **The `#{pane_dead_status}` exit-code backstop + the C2 commitment — RESOLVED.** v5
  flagged and fixed a real v4 drift: on the tmux path `result.Cmd.Wait()` resolves
  the **tmux ATTACH CLIENT**, not the pane wrapper (`attachTmuxPTY`, `pty.go:517-533`;
  `result.Cmd = attachCmd`, `result.PID = identity.PanePID`), so a reserved 97/98
  emitted by the pane wrapper does **not** reach the daemon through
  `result.Cmd.Wait()`. The **authenticated connect-out frame is the PRIMARY signal**;
  the reserved exit code (97/98) is a **BACKSTOP**, observed on the tmux path by
  adding `#{pane_dead_status}` to the liveness/exit capture (`tmux_liveness.go:228`),
  **never** from `result.Cmd.Wait()`. On a non-tmux/direct launch the codes still
  flow through `agentExitPayload`→`processExitCode` (`helper.go:427-439`). **C2**:
  the wrapper never propagates a provider child's status 97/98 into the reserved
  agentloop codes. Keep `TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker`
  and the extended `TestAgentLoopReservedExitCodeRoutesUnrecoverableWithoutPTYParsing`.
- **BC2 — RESOLVED (artifact identity from daemon state).** `resealInFlightJob`
  derives the expected-artifact set from the job's `expected_artifacts` (daemon
  state, attempt-resolved via `resolveExpectedArtifactCycles`), reuses
  `verifyRequiredArtifacts` / `ensurePerJobPublishedArtifactsDurable`
  (`go/pkg/mutations/mutations.go:828-876`), publishes only a `path` that is an open
  expected entry from the job's own worktree, and **refuses any unexpected path**;
  the connect-out frame carries NO job_id / artifact path / kind / body — identity is
  daemon-derived; a front-matter/author-line failure routes to the Option-4 floor.
  Keep `TestCodexResealUsesReceiverNotProviderStdout` (negative + positive).
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
  (`go/pkg/db/owner.go:23` — **currently 20**, re-verify; note
  `RequiredOwnerBundleVersion = LatestOwnerBundleVersion`, `owner.go:35`, moves with
  it) with the ordinal-21 `RESERVATIONS.toml` reservation, modelled exactly on the
  credited `review_generation` precedent (`go/pkg/db/sql/owner/0009_review_generation.sql`);
  `striatumd.jobs` is owner-held. A degrade-safe `JobRecoveryGenerationColumnPresent`
  probe routes to the typed floor when the column is absent. The four increment
  points (claim, requeue-same-attempt, recovery-sweep expire/transfer/respawn,
  release), each in the same UPDATE that retires/rebinds the authoritative lease
  under `lockRun`, are named; the post-increment value is stamped into
  `work_packets.packet_json` `lease.recovery_generation`, compared equal/unequal at
  reseal under the lock (mismatch → typed class). Keep
  `TestResealPredicateUsesStampedRecoveryGeneration` /
  `TestResealTokenRefusedAfterLeaseExpiryAndRecoveryRequeue`.
- **BC5 — RESOLVED (pinned migration site + corrected lock order).**
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
     `session_unrecoverable_across_rotation` class. Keep
     `TestResealBeyondGraceRoutesTypedNotLeaseError` /
     `TestResealGraceCannotReviveRequeuedLease` /
     `TestRecoveryRequeueWinsOverExpiredLeaseReseal` / `GD-1b` /
     `TestResealExit98BypassesBackendGateOrRoutesTyped`.
- **Daemon-OBSERVED positive intent + the recovery-sweep backstop — RESOLVED.**
  `resealInFlightJob` fires only on a **daemon-observed** post-rotation condition —
  boot-epoch rotated since the packet + the job still running with the stamped
  `recovery_generation` matching live + the lease within grace + every required
  `expected_artifact` present **and content-hash-modified since the
  `write_scope_baseline`** — never on a provider-asserted signal. Two entry points
  (the wrapper's authenticated post-rotation frame; the **recovery-sweep backstop**
  for "the wrapper can't even signal"), one condition, one lock.
- **The `ensureWorkSessionBackend` BYPASS (backend-gate routing) — RESOLVED.**
  `resealInFlightJob` deliberately bypasses `ensureWorkSessionBackend`
  (`lifecycle.go:1181`) — the reseal exists *precisely because* the live connection
  is gone — so a stopped supervisor routes the typed
  `session_unrecoverable_across_rotation` class rather than leaking
  `invalid_transition`/backend errors. Keep
  `TestResealExit98BypassesBackendGateOrRoutesTyped`.
- **The W1/W2/W3 wall SHAPES — RESOLVED IN SHAPE (only W1's CAPTURE BOUNDARY is the
  open BC1-W1-CAPTURE ground).** The three structural walls are the right walls and
  are carried forward as the channel's authentication design: **W1** — `SO_PEERCRED`
  peer-credentials on the accepted connect-out connection binding the channel to the
  *launched wrapper's* pid + kernel start-token (one coherent kernel domain); **W2**
  — `PR_SET_DUMPABLE(0)` on the wrapper before it dials; **W3** — the single-use
  control nonce delivered **daemon→wrapper post-auth**, never in env. **Do not
  relitigate the wall shapes or W1's load-bearing role.** The ONLY open question is
  BC1-W1-CAPTURE: whether the kernel token stamped in `LaunchResult.PaneKernelStartToken`
  is structurally bound to the still-live launched wrapper at capture time (see below).
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
  path. Keep `TestResealEpochMirrorRejectsTamperOrMissingEpoch`.
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
- **The per-claim falsifiable-assertion discipline (A1–A18, extended A3'/A3''/A4'/A7').**
  Every load-bearing claim is paired with the named test / game-day that refutes it.
  The A3'/A3'' negatives (same-pid stale-token + empty-token) are necessary but not
  sufficient; extend the discipline to cover the BC1-W1-CAPTURE capture-boundary
  structural binding.

## Ratified design shape — the resealInFlightJob channel (do NOT relitigate)

The v6 spec pinned (and both falsifiers credited) the following channel design; v7
**preserves it** and only resolves W1's capture boundary:

- A **daemon-internal `resealInFlightJob` mutation**
  (`go/pkg/mutations/recovery_reseal_rotation.go`, deliberately distinct from the RFC
  0125 `HandleRecoveryReseal` worktree-durability verb in
  `go/pkg/mutations/recovery_reseal.go`).
- A **connect-out control channel:** a daemon-held `SO_PASSCRED` `SOCK_SEQPACKET`
  listener at a non-secret abstract address; the pane wrapper dials OUT after
  `PR_SET_DUMPABLE(0)`; W1 authenticates the accepted peer by `SO_PEERCRED`
  uid+pid+kernel-start-token against the daemon-captured pane identity; the nonce
  (W3) is delivered daemon→wrapper post-auth.
- Two reserved agentloop exit codes (new, `go/pkg/agentloop/exitcodes.go`):
  `ExitUnrecoverableAcrossRotation = 97` (the Option-4 floor) and
  `ExitResealInFlightRequested = 98` (a latency hint only; its forgeability is
  immaterial — the daemon never seals on the strength of 98). On the tmux path these
  are observed via the **`#{pane_dead_status}`** capture (backstop), never from
  `result.Cmd.Wait()` (which is the attach client).
- The W1/W2/W3 wall shapes (above) authenticate any richer control frame; the frame
  schema carries NO job_id/artifact path/body — identity is derived from daemon
  state (BC2).

## The binding constraint v7 MUST resolve (the v6 adjudicator's sole unrebutted ground)

> The v6 ledger §"What the next revision MUST fix" pins the exact repair, and BOTH
> v6 falsifiers landed it INDEPENDENTLY. This SEED carries it verbatim in shape. It
> names exact source sites; anchor every load-bearing claim in the revised spec to
> them, paired with the named test. **Read the FULL v6 cycle_1 ledger for the exact
> text** (`docs/operator/artifacts/rfc-0143-design-v6/dialogue/adjudicator/COLLABORATION_LEDGER_cycle_1.md`).

### BC1-W1-CAPTURE — the kernel start token is captured POST-LAUNCH and is not structurally proven to belong to the born wrapper, so W1 can bind the channel to the wrong same-uid process

The v6 connect-out channel (carried forward) is correct: no fd crosses the tmux
client/server boundary, the address is non-secret, the nonce is delivered post-auth,
and the W1 operand is now coherent (field-22 on both sides). But **one material
precision gap remains, and it is inside the CAPTURE BOUNDARY of W1's kernel token**
— the load-bearing structural no-replay wall for the real channel.

The v6 W1 check accepts a connecting peer iff `peer.uid == RunAsUser uid`,
`peer.pid == result.PID` (`identity.PanePID`), `result.PaneKernelStartToken != ""`,
and `PIDLiveWithStartToken(peer.pid, result.PaneKernelStartToken)` returns live with
a matching field-22 token. The operand is coherent, but the **capture is post-launch**.
Independently confirmed against the current worktree:

- `launchPTY` starts the pane command with `tmux respawn-pane` (`pty.go:479-484`),
  **then** calls `CaptureTmuxIdentity` (`pty.go:493`), **then** `attachTmuxPTY`
  returns `LaunchResult{PID: identity.PanePID}` (`pty.go:517-533`);
  `ProcessStartToken` reads whatever process currently owns `/proc/<pid>/stat` field
  22 at read time (`process_identity_linux.go:11-32`).

So v6's `ProcessStartToken(identity.PanePID)` capture necessarily occurs **after**
the pane command launched and after a tmux round trip — a real, non-zero window. If
the launched wrapper exits inside that window and its numeric pid is reused by a
**live** same-uid sibling, the helper stamps the reused process's **non-empty**
field-22 token as `PaneKernelStartToken`; W1 then accepts the first connection from
that reused pid (`peer.pid == result.PID` and
`PIDLiveWithStartToken(peer.pid, capturedToken)` both hold), delivers the post-auth
nonce, and the sibling can emit a daemon-authenticated control frame into
`resealInFlightJob` — the same-uid replay surface BC1 exists to close.

**Why the existing negatives do NOT cover this:**

- The **empty-token fail-closed guard (A3'')** does **not** cover this: the reused
  process is live → `ProcessStartToken` returns `ok==true` with a non-empty token →
  the `!= ""` assertion passes.
- The **same-pid stale-token negative (A3')** does **not** cover this: it proves W1
  rejects a peer whose token differs from the already-captured token; this race is
  the **inverse** — the captured token is fresh but for the **wrong process**.

**Why this is material, not a build-run detail.** W1 is the load-bearing structural
no-replay wall for the real channel. For a security/authz channel, "the window is
small" is not a falsifiable implementation contract; no-replay must hold
**structurally**. The source already has the pieces to close it: `ProbeTmuxLiveness`
detects pane-dead (`tmux_liveness.go:257`), pane-pid mismatch (`:260`), and
pane-start mismatch (`:265`) — exactly the capture-boundary re-verification both
falsifiers prescribe.

**Prescribed fix (the v7 holder must do, in ONE place, within the v6
field-22/field-22 design):**

1. **Bind the captured kernel token to the still-live launched pane.** After
   `CaptureTmuxIdentity` reports `identity.PanePID`, capture
   `ProcessStartToken(identity.PanePID)` **AND** re-verify via tmux/liveness
   (`ProbeTmuxLiveness` states at `tmux_liveness.go:257-269`: pane-dead, pane-pid
   mismatch, pane-start mismatch) that the pane is **still live** and **still the
   same pane id/pid** at the capture boundary **BEFORE** stamping
   `PaneKernelStartToken` and **BEFORE** any accept loop binds the channel.
2. **Fail closed at the capture boundary.** If the pane is dead, missing,
   pid-mismatched, or identity-unverifiable at that boundary, do **NOT** stamp the
   kernel token and accept **NO** control connection for that launch; route through
   the `#{pane_dead_status}`/recovery-sweep typed
   `session_unrecoverable_across_rotation` floor (NEVER a raw launch/control error).
3. **Make the real-path test fire the race.** Extend
   `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` (or a paired integration
   negative) with a **"pane dies before kernel-token capture"** case that forces the
   wrapper to exit before W1 capture, makes the token read observe a
   same-pid/reused-process token by stub or controlled hook, and asserts the helper
   **refuses** rather than treating that token as the launched wrapper's identity.

**RETAIN** the v6 field-22/field-22 operand fix (`LaunchResult.PaneKernelStartToken`,
`ProcessStartToken(identity.PanePID)`, kernel-field-22-to-kernel-field-22), the
same-pid stale-token negative (A3'), and the empty-token fail-closed negative (A3'')
— they are necessary but not sufficient.

BC1-W1-CAPTURE is the **last load-bearing closure** of BC1 (F1 / F6 and the F2 / F7
channel halves all inherit it); the security invariant must hold **structurally** —
no-replay must hold structurally on the REAL channel with W1's kernel token
structurally bound to the launched wrapper, not as a probabilistic argument about
small windows.

**Source anchors (re-verify against current worktree HEAD and correct line numbers
if drifted):**

- `go/pkg/supervisor/pty.go` — `launchPTY`: `tmux respawn-pane` (~`:479-484`) →
  `CaptureTmuxIdentity` (~`:493`) → `attachTmuxPTY` returns `LaunchResult{PID:
  identity.PanePID}` (~`:517-533`); v6 added `LaunchResult.PaneKernelStartToken`
  (~`:47-53`) captured via `ProcessStartToken(identity.PanePID)` (~`:493-504`).
- `go/pkg/supervisor/process_identity_linux.go:11-32` — `ProcessStartToken` reads
  `/proc/<pid>/stat` field 22 at read time.
- `go/pkg/supervisor/tmux_liveness.go:257-269` — `ProbeTmuxLiveness` states:
  pane-dead (`:257`), pane-pid mismatch (`:260`), pane-start mismatch (`:265`) —
  exactly the capture-boundary re-verification to use.
- `go/pkg/supervisor/tmux_liveness.go:392-408` — `PIDLiveWithStartToken`; `:397` —
  `PIDLiveWithStartToken(pid, "")` skips the token comparison (the empty-token
  fail-open trap v6 closed).
- The named real-path test: `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper`
  (RunHelper + RequireTmux/RunAsUser).

## Also carry forward (the v6 build-test precision item, required in the spec)

The "deliverable observed" condition (BC2/positive-intent) must NOT treat "present +
absent from `write_scope_baseline.changed_paths`" as sufficient by itself — for
per-job isolated worktrees the baseline is **nil** (`write_scope_guard.go:81-83`),
and source-change publication already attributes authorship via
`gitChangedPathSnapshots` (`write_scope_guard.go:225`) +
`collectInScopeAuthoredPaths` (`artifact_source_publish.go:259-263`, used at `:88`;
`claim.go:622`). `resealInFlightJob` **reuses that authored-path attribution** so an
UNCHANGED pre-existing expected path is **NOT** resealed. Keep
`TestResealRequiresAuthoredExpectedArtifactChange` (seed a clean pre-existing
expected path → assert typed floor; modify it → assert positive reseal) or the
positive `TestCodexResealUsesReceiverNotProviderStdout` case.

## Clearing condition for this revision

The adjudicator clears the gate only if **BC1-W1-CAPTURE is genuinely resolved** with
a **capture-boundary re-verification** that structurally binds the kernel token to
the still-live launched pane (capture `ProcessStartToken(identity.PanePID)` AND
re-verify via `ProbeTmuxLiveness` that the pane is still live and still the same
pane id/pid at the capture boundary BEFORE stamping `PaneKernelStartToken` and
BEFORE any accept loop binds the channel; if the pane is dead, missing,
pid-mismatched, or identity-unverifiable, do NOT stamp and accept NO connection,
routing through the typed `session_unrecoverable_across_rotation` floor; and extend
`TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` or a paired integration negative
with a "pane dies before kernel-token capture" case that forces the wrapper to exit
before W1 capture, makes the token read observe a same-pid/reused-process token by
stub/hook, and asserts the helper refuses),
**AND structural no-replay is established on the REAL channel**,
**AND the whole v6-credited resolved set is carried forward unregressed** (the v6
BC1-W1-TOKEN clock-domain fix including `LaunchResult.PaneKernelStartToken` + the
empty-token `!= ""` rule + A3'/A3''; the connect-out topology + named plumbing
sites; the non-secret address + post-auth nonce W3; the W2 ordering +
dumpable-before-dial; the `#{pane_dead_status}` exit-code backstop + C2; BC2, BC3,
BC4, BC5; the daemon-observed positive intent; the backend-gate bypass; the
W1/W2/W3 wall shapes; F2, F4, the F7 file-mirror half; AF1, AF4; the
no-admin-token-widening invariant; the A1–A18 assertion discipline) with the
modified-since-baseline build-test folded in,
**AND no new material challenge** stands unrebutted.
The verdict is `reject` only if a path widens admin-token exposure or mints a
credential carrying any of `{admin, apply, recovery, surgical_recovery}`; otherwise
`needs_revision` if BC1-W1-CAPTURE remains open (a capture that does not re-verify
pane liveness/identity via tmux/liveness at the capture boundary, that does not fail
closed to the typed floor on a dead/missing/pid-mismatched/identity-unverifiable
pane, whose real-path test does not fire the "pane dies before capture" race, or
whose structural binding still depends only on a probabilistic temporal argument), if
any credited item is regressed, or any new material challenge lands. One revision
cycle is available within this run; the falsifiers re-attack the revised spec.

## Maintainer-ratification note (carries regardless of verdict)

Slice B — the daemon-internal `rpc.CapabilityReseal` marker, the test-only
auth-prelude route alternate, the daemon-owned supervisor control channel with
**connect-out `SO_PEERCRED` pid+kernel-start-token** authentication, the reserved
agentloop exit codes, the `jobs.recovery_generation` +
`leases.reseal_grace_extended_at` owner-bundle-0021 columns, and the endpoint/epoch
republish plumbing — is a **security/authz trust-model change requiring maintainer
ratification before any build slice touches credential code**. Adjudicator clearance
gates the spec's **soundness**, not the maintainer's product call. Slice A (the
Option-4 typed-exit-code floor) is zero-trust-change but, per BC1-W1-CAPTURE, still
must route over the connect-out, non-PTY channel whose same-uid authentication (W1)
is **specified with one coherent kernel identity token STRUCTURALLY BOUND to the
launched wrapper** before it lands.

---
<sub>Operator scaffold for the RFC 0143 falsification-gate design run (v7 / REVISION
of `rfc-0143-design-v6`; resolves the single remaining binding constraint
BC1-W1-CAPTURE — capture-boundary TOCTOU in the W1 no-replay wall — by requiring
capture-boundary re-verification via `ProbeTmuxLiveness` (pane-dead/pane-pid-mismatch/
pane-start-mismatch states at `tmux_liveness.go:257-269`) that the pane is still live
and still the same pane id/pid BEFORE stamping `PaneKernelStartToken` and BEFORE any
accept loop binds the channel, failing closed to the typed
`session_unrecoverable_across_rotation` floor if not, and extending
`TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` (or a paired integration negative)
with a "pane dies before kernel-token capture" case asserting the helper refuses a
same-pid/reused-process token — while retaining the v6 field-22/field-22 operand fix
(`LaunchResult.PaneKernelStartToken`, `ProcessStartToken(identity.PanePID)`, A3'/A3'')
and carrying the v6-credited set (the connect-out topology + named plumbing sites +
non-secret address + post-auth nonce W3 + W2 ordering + `#{pane_dead_status}` backstop
+ C2 + BC2/BC3/BC4/BC5 + daemon-observed positive intent + backend-gate bypass +
W1/W2/W3 shapes + F2/F4/F7-file/AF1/AF4/no-widening/A1–A18) forward unregressed,
folding in the modified-since-baseline authored-path build-test). Lanes: author=claude
(holder/adjudicator/committer), reviewer=codex (falsifiers).</sub>
