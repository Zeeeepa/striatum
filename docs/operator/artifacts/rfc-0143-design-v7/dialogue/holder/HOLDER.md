# HOLDER — RFC 0143 falsifiable implementation spec (design-v7 REVISION)

author: holder-author-001

> This is the **seventh** falsification pass on RFC 0143 (*lane credential survival
> across a daemon boot-epoch rotation*) and a **proper revision**. v1
> (`rfc-0143-design`) returned `needs_revision` with seven findings F1–F7. v2 resolved
> **F2/F4** and distilled the residue into five binding constraints BC1–BC5. v3 resolved
> **BC2, BC3, BC4** and carried the v2-credited set forward unregressed. v4 resolved
> **BC5 + two of BC1's three sub-grounds (C2 + the daemon-observed positive-intent
> source with the backend-gate bypass)**, but fell on **BC1-CHANNEL** (the W1/W2/W3
> walls were specified for a DIRECT `exec.Cmd.ExtraFiles` child exec while the production
> lane is TMUX-BACKED). v5 **resolved BC1-CHANNEL** by deleting the inherited-fd channel
> and adopting a **CONNECT-OUT topology** (the pane wrapper dials OUT after
> `PR_SET_DUMPABLE(0)`; no fd crosses the tmux client/server boundary; non-secret
> listener address; post-auth nonce), but fell — both falsifiers INDEPENDENTLY — on
> **BC1-W1-TOKEN** (W1 compared a kernel `/proc` field-22 start-tick against a tmux
> `#{pane_start_time}` wall-clock timestamp: two categorically different clock domains).
> v6 **RESOLVED BC1-W1-TOKEN** by pinning ONE coherent KERNEL start-token source
> (`LaunchResult.PaneKernelStartToken` captured via `ProcessStartToken(identity.PanePID)`,
> compared field-22-to-field-22 against the accepted peer's `ProcessStartToken(peer.pid)`),
> kept tmux `#{pane_start_time}` as liveness metadata only, and closed the empty-token
> fail-open trap with a fail-closed `!= ""` assertion (A3'') plus the same-pid stale-token
> negative (A3'). **Both v6 falsifiers credited all of that and found NO regression of the
> v5-credited set.** But v6 fell — both falsifiers INDEPENDENTLY, again — on a single,
> sharply-named ground: **BC1-W1-CAPTURE — the now-coherent kernel start token is
> captured POST-LAUNCH and is not structurally proven to belong to the BORN wrapper, so a
> fast-exiting wrapper + pid reuse lets W1 bind the channel to a live same-uid sibling.**
> **BC1-W1-CAPTURE is the LAST open BC1 ground.**
>
> This v7 spec starts from the **v6** `HOLDER.md` (required context) and the **v6**
> collaboration ledger (the BC1-W1-CAPTURE finding + the exact "next revision must…"
> list). It **resolves BC1-W1-CAPTURE in ONE place** by adding a **fail-closed
> capture-boundary re-verification** that structurally binds the kernel token to the
> still-live launched pane — anchored on the kernel/tmux **reaping invariant**, not a
> temporal "the window is small" argument — and **carries the entire v6-credited resolved
> set forward UNREGRESSED**. It does **not** relitigate the ratified OQ1 trust-model
> shape, the F2 non-bearer decision, the connect-out topology, or the W1/W2/W3 wall
> *shapes* and W1's load-bearing role (all pinned in `SEED.md`). The connect-out topology,
> the wall shapes, and the field-22/field-22 operand fix are correct; v7 fixes only the
> **CAPTURE BOUNDARY of W1's kernel token**. Every source citation below was re-verified
> against the current worktree while authoring this revision; **drift is flagged inline in
> §Source re-verification.**

## Root reframe (held, unchanged)

**A boot-epoch rotation must never force a lane to choose between reading the daemon's
full-authority bootstrap admin `client-token` and exiting silently unsealed.** A
`striatum-lane` lane authenticates as *its own* narrow, session-scoped credential and
**never** as the shared operator admin override. v7 either lets the lane's in-flight work
be sealed over a **daemon-projected, session-tied authority that no lane bearer carries**,
or makes the failure **loud, typed, and routed** — never silent, never via the admin
token.

## What v7 changes vs v6 (ONE place)

v6 was credited on the field-22/field-22 operand fix (`LaunchResult.PaneKernelStartToken`,
`ProcessStartToken(identity.PanePID)`, kernel-vs-kernel comparison), the empty-token
fail-closed rule (A3''), the same-pid stale-token negative (A3'), the connect-out topology
+ named plumbing sites, the W2 ordering, the non-secret address + post-auth nonce (W3), the
`#{pane_dead_status}` backstop + C2, and the entire carry-forward set
(BC2/BC3/BC4/BC5, the daemon-observed positive intent, the backend-gate bypass, the
W1/W2/W3 wall *shapes*, F2, F4, the F7 file-mirror half, AF1, AF4, the
no-admin-token-widening invariant, A1–A18). It fell on exactly one ground inside W1 — **the
CAPTURE BOUNDARY of the kernel start-token operand**:

- The v6 W1 check accepts a connecting peer iff `peer.uid == RunAsUser uid`,
  `peer.pid == result.PID` (`identity.PanePID`), `result.PaneKernelStartToken != ""`, and
  `PIDLiveWithStartToken(peer.pid, result.PaneKernelStartToken)` returns live with a
  matching field-22 token. The **operand** is coherent (field-22 both sides), but the
  **capture is post-launch**: `launchPTY` starts the pane command with `tmux respawn-pane`
  (`pty.go:479-484`), **then** calls `CaptureTmuxIdentity` (`pty.go:493`), and v6 captures
  `ProcessStartToken(identity.PanePID)` only after that — a real, non-zero window after the
  pane command launched and after a tmux round trip.
- `ProcessStartToken` reads whatever process **currently** owns `/proc/<pid>/stat` field 22
  at read time (`process_identity_linux.go:13-32`). If the launched wrapper exits inside
  that window and its numeric pid is reused by a **live** same-uid sibling, v6's helper
  stamps the reused process's **non-empty** field-22 token as `PaneKernelStartToken`; W1
  then accepts the first connection from that reused pid (both `peer.pid == result.PID` and
  `PIDLiveWithStartToken(peer.pid, capturedToken)` hold), delivers the post-auth nonce, and
  the sibling can emit a daemon-authenticated control frame into `resealInFlightJob` — the
  same-uid replay surface BC1 exists to close.
- The empty-token guard (**A3''**) does **not** cover this: the reused process is live →
  `ProcessStartToken` returns `ok==true` with a non-empty token → the `!= ""` assertion
  passes. The same-pid stale-token negative (**A3'**) does **not** cover this either: it
  rejects a peer whose token differs from the *already-captured* token; this race is the
  **inverse** — the captured token is fresh but for the **wrong process**.

**v7 adds ONE fail-closed capture-boundary invariant inside the v6 field-22/field-22
design.** It binds the captured kernel token to the still-live launched pane by
**re-verifying pane liveness/identity via `ProbeTmuxLiveness` AFTER the kernel-token read
and BEFORE stamping `PaneKernelStartToken`** — and the binding is **structural**, resting
on the kernel/tmux reaping invariant (below), not on temporal proximity. So v7:

1. **Captures the named kernel token** `paneKernelStartToken, ok :=
   processStartToken(identity.PanePID)` immediately after `CaptureTmuxIdentity` validates
   `identity.PanePID > 0` (`pty.go:493-504`), through an **injectable seam** (a package
   function variable, for the deterministic negative test) — exactly as v6.
2. **Re-verifies the pane via `ProbeTmuxLiveness` AFTER the kernel read** (the new v7
   step): the probe must return `Healthy` / `TmuxLivenessOK` with `ObservedPanePID ==
   identity.PanePID` and a matching tmux start token — i.e. **not** pane-dead
   (`tmux_liveness.go:257`), **not** pane-pid-mismatched (`:260`), **not**
   pane-start-mismatched (`:265`), and **not** pane-missing/unavailable (`:217-233`).
3. **Stamps `PaneKernelStartToken` ONLY if that post-read re-verification passes.** If the
   pane is dead, missing, pid-mismatched, or identity-unverifiable at the boundary, v7 does
   **NOT** stamp the kernel token (it leaves `PaneKernelStartToken == ""`) and accepts
   **NO** control connection for that launch; the floor is reached through the existing
   typed `session_unrecoverable_across_rotation` path (the `#{pane_dead_status}` exit
   backstop + the recovery-sweep backstop), **never** a raw launch/control error.
4. **Retains the v6 fail-closed empty-token rule (A3'') as the unifying mechanism.** An
   unstamped token (whether because the read failed *or* because the re-verification
   failed) deterministically engages v6's "no pid-only accept" rule: `RunHelper` asserts
   `result.PaneKernelStartToken != ""` before accepting any connection, so a
   capture-boundary miss accepts nothing and routes the typed floor.

This is the only material change. Everything else in the v6 spec is reproduced below in
substance and carried forward; the falsifiers should re-attack the **capture-boundary
re-verification** (and confirm no carried item regressed).

### Why this is STRUCTURAL, not "the window is small" (the reaping invariant)

The v6 adjudicator rejected the Holder's temporal defense ("microseconds after launch")
because for a security/authz channel no-replay must hold **structurally**. v7's binding is
structural because it rests on a kernel + tmux invariant, not on timing:

1. **A pid cannot be reused while its previous owner is unreaped.** A process that has
   exited but whose parent has not `wait()`-ed it is a **zombie**; the kernel keeps its pid
   allocated until the parent reaps it. So **pid `P` is reusable ⟹ `P` has been reaped.**
2. **The pane process's reaper is the tmux server.** `tmux respawn-pane -k`
   (`pty.go:479-484`) makes the pane command a **direct child of the tmux server**; tmux
   reports its pid as `#{pane_pid}` and reaps it via its own `SIGCHLD` handler. (A
   grandchild that outlives the pane command is reparented to init, not tmux, and never
   holds `#{pane_pid}`.) So **`P` has been reaped ⟹ the tmux server reaped it.**
3. **tmux reaping `P` flips the pane out of the live state.** When tmux reaps its pane
   child it either marks the pane dead (`#{pane_dead}=1`, with `remain-on-exit on`) or
   destroys the pane entirely; tmux never auto-respawns (we respawn only via the explicit
   `respawn-pane`). Either outcome makes `ProbeTmuxLiveness` return **non-`Healthy`**:
   pane-dead (`tmux_liveness.go:257`) or pane-missing/unavailable (`:217-233`, `:226`,
   `:241`). So **the tmux server reaped `P` ⟹ `ProbeTmuxLiveness` is non-`Healthy`.**

Chaining (1)→(2)→(3): **`P` was reused ⟹ `ProbeTmuxLiveness` is non-`Healthy`.** The
contrapositive is the load-bearing guarantee:

> **`ProbeTmuxLiveness` returns `Healthy` with `ObservedPanePID == P` ⟹ `P` was NOT reaped
> through the moment of that probe ⟹ `P` was NOT reused through that moment.**

Because v7 takes that probe **after** the `processStartToken(P)` read, a passing
re-verification proves the kernel-token read saw `P`'s **own** process (alive, or its own
unreaped zombie — same field-22 start-tick), **never** a reused sibling. The window between
`CaptureTmuxIdentity` and the kernel read, and the window between the kernel read and the
probe, are **both** closed by the single post-read probe: any reaping in either window
forces the probe non-`Healthy`. *Falsifier:* **A3'''** + the real-path "pane dies before
kernel-token capture" negative.

### Why the residual `[probe → W1 accept]` window is also closed (no new gap)

The post-read probe closes `[launch → probe]`. The only remaining interval is `[probe →
W1 accept]`: the pane could die and `P` be reused *after* the probe but *before* a peer
connects. That interval is closed by the **retained v6 accept-time check (A3')**: W1
compares the connecting peer's `ProcessStartToken(peer.pid)` to the **captured** kernel
token via `PIDLiveWithStartToken(peer.pid, result.PaneKernelStartToken)`
(`tmux_liveness.go:392-408`). If `P` was reaped+reused after the probe, the reused process
carries a **different** field-22 token → `PIDLivenessIdentityMismatch` (`:404`) → W1
**refuses**. The captured token is now *proven authentic* by the post-read probe, so the
accept-time field-22 comparison is comparing against a trustworthy operand. **End to end:
the post-read re-verification (v7) closes `[launch → probe]`; the accept-time field-22
match (A3') closes `[probe → accept]`. No-replay holds structurally across the whole
lifetime of the launch.** *Falsifier:* A3' (accept-time) + A3''' (capture-boundary).

### Capture-boundary ordering (the exact sequence the build must implement)

Inside `launchPTY`, between the `CaptureTmuxIdentity` validation (`pty.go:493-504`) and the
`attachTmuxPTY` call (`pty.go:507`):

```
identity := CaptureTmuxIdentity(...)                  // pty.go:493 — reports PanePID=P, PaneStartToken=S_tmux (liveness only)
//          (identity validated: WindowID/PaneID non-empty, PanePID>0 — pty.go:494-504)
//   --- v7 capture-boundary re-verification (NEW; before attachTmuxPTY at :507) ---
kTok, ok := processStartToken(identity.PanePID)        // injectable seam; reads /proc/<P>/stat field 22 (process_identity_linux.go:13-32)
live      := ProbeTmuxLiveness(ctx, runner, identity)  // POST-READ re-verification (tmux_liveness.go:212-…)
if !ok || kTok == "" ||                                // empty/unreadable token (A3'') ...
   !(live.Healthy && live.Class == TmuxLivenessOK &&   // ... OR pane not re-verified live/same-pid (A3''')
     live.ObservedPanePID == identity.PanePID):
    //  DO NOT stamp PaneKernelStartToken (leave ""); accept NO control connection.
    //  Floor routes via #{pane_dead_status}/recovery-sweep typed session_unrecoverable_across_rotation.
    paneKernelStartToken = ""                           // explicit: unstamped
else:
    paneKernelStartToken = kTok                         // stamp ONLY after a passing post-read re-verify
result.PaneKernelStartToken = paneKernelStartToken      // threaded onto LaunchResult (pty.go:47-53), set at/after attachTmuxPTY (:527-533)
```

The probe is taken **after** the kernel read by construction; that ordering is the whole
structural point (a probe *before* the read would not prove the read saw `P`). `S_tmux`
(`identity.PaneStartToken`, tmux `#{pane_start_time}`) stays **liveness metadata only** —
it is the `ProbeTmuxLiveness` `expectedStart` operand (`tmux_liveness.go:251,265`), never
the W1 kernel operand, exactly as v6 pinned. *Falsifier:* A3''' asserts the stamp is gated
on a post-read `ProbeTmuxLiveness` `Healthy`/same-pid result, and that the "pane dies before
capture" case leaves the token unstamped and accepts no connection.

## Source re-verification (every BC1-W1-CAPTURE site CONFIRMED against current main; drift FLAGGED)

| Claim | Site | Status |
| --- | --- | --- |
| `launchPTY` starts the pane command with `tmux respawn-pane -k` | `pty.go:479-484` (`respawnArgs := []string{"respawn-pane", "-k", …}` at `:479`; `runPreparedTmuxSetupCommand` at `:484`) | **CONFIRMED** verbatim |
| `launchPTY` calls `CaptureTmuxIdentity` next, validates `PanePID > 0` before attaching | `pty.go:493` (call), `:494-504` (validation + RequireTmux fail-closed branch) | **CONFIRMED** — the v7 re-verification inserts here, after `:504`, before `attachTmuxPTY` at `:507` |
| `attachTmuxPTY` builds `LaunchResult{PID: identity.PanePID}` | `pty.go:507` (call), `:517-534` (func), `:527-533` (`PID: identity.PanePID`) | **CONFIRMED** — ⚠️ *minor drift:* SEED/v6 cite `:517-533`; the func body closes at `:534`. Same code. |
| `LaunchResult` struct (v6 adds `PaneKernelStartToken`) | `pty.go:47-53` | **CONFIRMED** — current fields `PID, StdinWriter, Cmd, AttachPID, Metadata`; `PaneKernelStartToken` is the v6 add (not yet in main, as expected for a design spec) |
| `LaunchSpec` has no control field (v5 adds `ControlSocketAddr`) | `pty.go:30-42` | **CONFIRMED** (fields: Command, Env, EnvFilePath, WorkingDir, RunAsUser, StdinPipePath, StdoutPath, StderrPath, UsePTY, RequireTmux, Extra) |
| `ProcessStartToken` = `/proc/<pid>/stat` **field 22**, read at call time (`const starttimeIndex = 22 - 3`; returns `fields[starttimeIndex]`) | `process_identity_linux.go:13-32` (field-22 read `:26-31`) | **CONFIRMED** — ⚠️ *minor drift:* SEED/prompt cite `:11-32`; the doc comment is `:11-12`, the func body `:13-32`. Same code. This is the "reads whatever process currently owns the pid" primitive the race exploits. |
| `CaptureTmuxIdentity` records tmux pane pid + `#{pane_start_time}`; does NOT pin a kernel field-22 token at process birth | `tmux_liveness.go:181-210` (`:182` query, `:194-202` token source, `:207-208` PanePID/PaneStartToken) | **CONFIRMED** — ⚠️ *minor drift:* SEED cites `:181-209`; the return closes at `:210`. Same code. |
| `ProbeTmuxLiveness` re-verification states the v7 fix uses | `tmux_liveness.go:212-` (func); **pane-dead `:257`**, **pane-pid mismatch `:260`**, **pane-start mismatch `:265`**; pane-missing/unavailable `:217`,`:223`,`:226`,`:233`,`:241` | **CONFIRMED** verbatim — `:257` `if strings.TrimSpace(parts[2]) == "1"` → `TmuxLivenessPaneDead`; `:260` `if observedPID != id.PanePID` → `TmuxLivenessPanePIDMismatch`; `:265` start-token mismatch → `TmuxLivenessPanePIDMismatch` |
| `ProbeTmuxLiveness` queries `#{pane_dead}` (NOT `#{pane_dead_status}`) | `tmux_liveness.go:228` (`#{pane_id}\|#{pane_pid}\|#{pane_dead}\|#{pane_start_time}`) | **CONFIRMED** — the v5 `#{pane_dead_status}` exit backstop addition still applies and is carried forward |
| `PIDLiveWithStartToken(pid, expectedStart)` compares `ProcessStartToken(pid)` (field 22) to `expectedStart`; **skips the comparison when `expectedStart == ""`** (returns live on pid alone) | `tmux_liveness.go:392-408` (skip branch `:397`; mismatch `:403-404` `PIDLivenessIdentityMismatch`; unavailable `:400-401` `PIDLivenessIdentityUnavailable`) | **CONFIRMED** — the field-22-vs-field-22 accept primitive (A3') **and** the empty-token fail-open trap (A3'') the fix reuses |
| `verifiedStartToken` merely checks the value parses as an unsigned integer (does NOT convert tmux wall-clock → `/proc` field 22) | `tmux_liveness.go:429-438` (`ParseUint` `:434`) | **CONFIRMED** — why `#{pane_start_time}` stays liveness-only, never the W1 operand |
| `LatestOwnerBundleVersion` currently **20** (so the 20→21 bump still lands; `RequiredOwnerBundleVersion = LatestOwnerBundleVersion` moves with it) | `owner.go:23` (`= 20`), `:35` (`= LatestOwnerBundleVersion`) | **CONFIRMED 20** |

All other v6 BC1-W1-TOKEN / BC1-CHANNEL / BC2 / BC3 / BC4 / BC5 / carry-forward citations
were credited as **not regressed** by both v6 falsifiers and the v6 adjudicator and are
reproduced below in substance; they are unchanged by v7, which touches only the W1
capture boundary. The `helper_protocol.go:27-39` / `helper.go:128,:149-156` /
`pty.go:30-42,:47-53` / `supervision.go` / `claim.go` / `write_scope_guard.go` /
`artifact_source_publish.go` anchors stand as v5/v6 verified them.

## Ratified design shape (pinned — built on, not relitigated)

- **OQ1 (ratified):** Slice A = Option 4 (mandatory, zero-trust-change, lands first) +
  Slice B (ratification-gated) = Option 2's narrow `CapabilityReseal` over a daemon-owned
  session-tied path + minimal Option 3 per-session endpoint+epoch republish. No
  lane-readable reseal bearer file under any option.
- **F2 (decided):** non-bearer, daemon-owned, session-tied channel; **no readable reseal
  token file at all** (every lane shares the `striatum-lane` uid, so any `0600` file is a
  same-uid replay surface). Not reopened. v5 closed the channel's INSTALLATION on the real
  tmux launch path structurally via the connect-out topology; v6 closed the channel's
  AUTHENTICATION primitive (W1) by pinning one coherent kernel start-token source;
  **v7 closes the channel's IDENTITY BINDING by requiring capture-boundary re-verification
  via tmux/liveness before the kernel token is stamped.**
- **Connect-out topology + W1/W2/W3 wall SHAPES (ratified by the v5 gate).** The
  connect-out channel and the three structural walls are the right walls and are carried
  forward as the channel's authentication design. **Not relitigated.** The ONLY open
  question v7 resolves is the CAPTURE BOUNDARY of W1's kernel token (BC1-W1-CAPTURE).
- **Slice B requires maintainer ratification** before any build slice touches credential
  code. Adjudicator clearance gates the spec's *soundness*, not the maintainer's product
  call. Slice A is zero-trust-change and may land first **once BC1-W1-CAPTURE structurally
  binds W1's kernel token to the launched wrapper** — which v7 does.

## Architectural facts re-anchored (AF1–AF4 — carried forward unregressed)

- **AF1 — reachability, not reminting.** `mintSessionBoundToken`
  (`go/pkg/mutations/session_token.go`) inserts the client row + per-capability grants into
  daemon-owned PostgreSQL bound to `session_id`, 24h TTL. **PostgreSQL survives a
  `striatumd` restart** (D094 / RFC 0043). After a boot-epoch rotation the token is still
  *valid* — only *unreachable* (it lives as the `STRIATUM_MCP_TOKEN` env literal, step 1;
  the post-rotation re-readers skip step 1). The fix is routing, not re-minting.
  *Falsifier:* `TestTokenValidAcrossRestart` (A17).
- **AF2 — post-rotation re-readers fall to step 3.** `ResolveTokenMaterial`
  (`token.go:18-53`) reaches the runtime `client-token` branch at `:31-42` whenever steps
  1/2 are absent; the #323 fresh re-read (`ResolveTokenMaterialFresh`,
  `go/pkg/agentloop/endpoint.go`) likewise skips the env literal and falls to step 3 — the
  bug.
- **AF3 — step 3 is the full-authority admin token in a `0700` dir.**
  `admin.BootstrapRuntimeTokenIfNeeded` (`go/pkg/admin/bootstrap.go:18-27`) grants the
  runtime `client-token` the full `bootstrapCapabilities` `{admin,read,write,claim,review,
  apply,recovery,surgical_recovery}`, `0600` in a `0700` dir; `ReadTokenFile`
  (`token.go:75-92`) rejects any token file not owner-only.
- **AF4 — epoch/token decoupling.** Endpoint + boot epoch rotate together; #316
  deliberately retires a surviving lane's connection by rejecting a stale epoch. The token
  does **not** rotate on a normal restart — only the endpoint does. Preserved.

## Carried forward from v6, unregressed (do NOT reopen)

| Item | Status | Anchor / test kept |
| --- | --- | --- |
| **BC1-W1-TOKEN — field-22/field-22 operand fix** (`LaunchResult.PaneKernelStartToken` captured via `ProcessStartToken(identity.PanePID)`; W1 compares the accepted peer's `ProcessStartToken(peer.pid)` to the captured kernel token; tmux `#{pane_start_time}` liveness-metadata only; empty-token `!= ""` fail-closed; A3'/A3'') | resolved (carried); **v7 builds the capture-boundary binding ON it** | `pty.go:47-53`,`:493-504`; `tmux_liveness.go:392-408,:397`; `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` (A3'/A3'') |
| **Connect-out topology + named plumbing sites** (no fd crosses the tmux boundary; pane wrapper dials OUT after `PR_SET_DUMPABLE(0)`) | resolved (carried) | `ControlSocketAddr` on `HelperLaunchSpec` (`helper_protocol.go:27-39`)/`LaunchSpec` (`pty.go:30-42`); `RunHelper` (`helper.go:128,:149-156`); new agentloop `dumpable_linux.go`/`control_channel.go`/`exitcodes.go`; `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` |
| **Non-secret address + post-auth nonce (W3)** | resolved (carried) | `STRIATUM_SUPERVISOR_CONTROL_ADDR` over existing env plumbing; nonce delivered daemon→wrapper post-auth |
| **W2 ordering + dumpable-before-dial** | resolved (carried) | `PR_SET_DUMPABLE(0)` first in the agentloop entrypoint; reinforced by `sudo` setuid launch |
| **`#{pane_dead_status}` exit-code backstop + C2** | resolved (carried) | `tmux_liveness.go:228` add `#{pane_dead_status}`; `TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker`; `TestAgentLoopReservedExitCodeRoutesUnrecoverableWithoutPTYParsing` |
| **BC2** — reseal artifact identity from the job's `expected_artifacts` (daemon state); refuses unexpected paths; front-matter failure → floor | resolved (carried) | `verifyRequiredArtifacts`/`ensurePerJobPublishedArtifactsDurable` (`mutations.go:828-876`); `TestCodexResealUsesReceiverNotProviderStdout` (negative + positive) |
| **BC3** — `CapabilityReseal` a daemon-internal marker projected by `resealInFlightJob`; public route-alternate test-only | resolved (carried) | `TestResealCapabilityIsDaemonInternalNotBearer` / `TestResealTokenCanReachOnlyResealRoutesWithoutWrite` |
| **BC4** — concrete monotonic `jobs.recovery_generation` (owner bundle 0021), increment points, stamped value compared under the lock | resolved (carried) | `TestResealPredicateUsesStampedRecoveryGeneration` / `TestResealTokenRefusedAfterLeaseExpiryAndRecoveryRequeue` |
| **BC5** — `leases.reseal_grace_extended_at` in owner bundle 0021 (`leases` owner-held); corrected skip/replace/replay lock-order gate map | resolved (carried) | `TestResealBeyondGraceRoutesTypedNotLeaseError` / `TestResealGraceCannotReviveRequeuedLease` / `TestRecoveryRequeueWinsOverExpiredLeaseReseal` / `GD-1b` |
| **Daemon-observed positive intent + recovery-sweep backstop** | resolved (carried) | positive `TestCodexResealUsesReceiverNotProviderStdout`; `TestResealRequiresAuthoredExpectedArtifactChange` |
| **`ensureWorkSessionBackend` bypass** | resolved (carried) | `TestResealExit98BypassesBackendGateOrRoutesTyped` |
| **W1/W2/W3 wall SHAPES** | shape resolved; **W1 capture boundary = this revision** | §BC1-W1-CAPTURE + `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` |
| **F2** — no lane-readable reseal bearer | resolved | `TestBorrowedResealBearerCannotSealVictimSession` |
| **F4** — route-alternate records `reseal` not `write` on only the 3 routes | resolved | `TestResealTokenCanReachOnlyResealRoutesWithoutWrite`; command-authority-matrix reseal column |
| **F7 file-mirror half** — daemon-owned lane-read-only `0644` endpoint/epoch, `O_NOFOLLOW`, atomic rename, reject MISSING epoch header (closes #316) | resolved | `TestResealEpochMirrorRejectsTamperOrMissingEpoch` |
| **AF1 / AF4 / no-admin-token-widening invariant** | kept / held + strengthened | `TestTokenValidAcrossRestart` / `TestResolveRefusesRuntimeClientTokenForLane` |
| **Per-claim falsifiable-assertion discipline** | extended to the W1 capture-boundary contract | A1–A18 + A3'/A3''/**A3'''**/A4'/A7' below |

The carried-forward sections (the connect-out channel topology, the exit-code backstop,
BC2, BC3, BC4, BC5, the daemon-observed trigger, the backend-gate bypass) are reproduced
below in substance from v6; only **§BC1-W1-CAPTURE** (the capture-boundary re-verification
+ A3''') is new.

---

# Security cluster (BC1-W1-CAPTURE + the carried BC1-W1-TOKEN operand + the connect-out channel + BC2 + BC3)

## BC1-W1-CAPTURE — fail-closed capture-boundary re-verification structurally binds the kernel token to the launched pane (the v7 closure)

This is the **one place** the v7 fix lands. It resolves the last open BC1 ground while
preserving the v6 field-22/field-22 operand fix, the connect-out topology, and the W1/W2/W3
wall shapes verbatim.

### The W1 accept predicate (operand from v6, now with a *proven-authentic* captured token)

When a peer connects to the daemon-held `SO_PASSCRED` `SOCK_SEQPACKET` listener, the helper
reads `SO_PEERCRED` on the **accepted** connection — the connecting peer's real `{pid, uid,
gid}` at connect time. The helper accepts **iff ALL hold**:

1. `peer.uid == RunAsUser uid` (the lane user), AND
2. `peer.pid == result.PID` — the launched **pane** pid (`identity.PanePID`,
   `pty.go:527-528`), AND
3. **`result.PaneKernelStartToken != ""`** — the launch-time kernel start token was both
   captured **and** re-verified against a still-live launched pane (the v7
   capture-boundary stamp; fail-closed if empty), AND
4. `PIDLiveWithStartToken(peer.pid, result.PaneKernelStartToken)`
   (`tmux_liveness.go:392-408`) returns **live with matching token** — the peer pid is
   signalable **and** `ProcessStartToken(peer.pid)` (`/proc` field 22) **equals the captured
   kernel token** (`/proc` field 22). **One clock domain on both sides.**

The helper accepts the **first** connection whose peer-cred matches and binds the channel to
it; every later/other connection is **refused**. A same-uid sibling that connects has a
**different pid** → refused at (2); a reused pid carrying a different process has a
**different field-22 token** → refused at (4). **What v7 adds:** condition (3)'s token is no
longer merely "read promptly after launch" — it is stamped **only after** a post-read
`ProbeTmuxLiveness` re-verification proves the pid was not reaped/reused through the read, so
the accept-time field-22 comparison at (4) compares against a *proven-authentic* operand. The
structural no-replay property now holds on the REAL channel across the **whole** launch
lifetime: `[launch → probe]` closed by (3)'s re-verification; `[probe → accept]` closed by
(4)'s field-22 match.

### The named kernel-token CAPTURE + the v7 re-verification (the operand binding of truth)

| Site | v6 | v7 change |
| --- | --- | --- |
| `LaunchResult` (`pty.go:47-53`) | carries `PaneKernelStartToken string` (v6 add) | **UNCHANGED** — still the launch-time `/proc` field-22 token of the pane wrapper |
| `launchPTY` (`pty.go:493-504`, after `CaptureTmuxIdentity` validates `identity.PanePID > 0`, before `attachTmuxPTY` at `:507`) | `paneKernelStartToken, ok := ProcessStartToken(identity.PanePID)` captured here | **CAPTURE through an injectable seam, THEN re-verify via `ProbeTmuxLiveness` AFTER the read; stamp `PaneKernelStartToken` ONLY if the probe returns `Healthy`/`TmuxLivenessOK` with `ObservedPanePID == identity.PanePID`** (else leave it `""`). Both still BEFORE any control connection is accepted. |
| `RunHelper` (`helper.go:128,:149-156`) | W1 compares `ProcessStartToken(peer.pid)` to `result.PaneKernelStartToken`; asserts `!= ""` before accepting | **UNCHANGED** — the `!= ""` fail-closed assertion now also covers a capture-boundary miss (unstamped token), so no pid-only accept ever occurs |
| `CaptureTmuxIdentity` (`tmux_liveness.go:181-210`) | sets `PaneStartToken = #{pane_start_time}` when numeric | **UNCHANGED** — `identity.PaneStartToken` stays the tmux value, used **only** for liveness (`ProbeTmuxLiveness`, incl. the v7 re-verification's `expectedStart` operand), **never** for W1 |
| `ProbeTmuxLiveness` (`tmux_liveness.go:212-…`) | used for ordinary liveness polling | **REUSED at the capture boundary** as the post-read re-verification; its existing pane-dead (`:257`), pane-pid-mismatch (`:260`), pane-start-mismatch (`:265`), and pane-missing/unavailable (`:217-233`) states are exactly the fail-closed triggers |

**`#{pane_start_time}` is kept only as liveness metadata.** v7 does **NOT** prove tmux
`#{pane_start_time}` equivalent to `/proc` field 22 — it is a wall-clock unix timestamp,
categorically distinct from start-ticks-since-boot — so it is excluded from the W1 *operand*
by construction. It serves only as the `ProbeTmuxLiveness` `expectedStart` (`tmux_liveness.go:251,265`),
which the v7 re-verification consults to detect a pane-start mismatch. The W1 operand is the
separately-captured kernel token; the two never cross.

### Fail-closed at the capture boundary (no pid-only degrade, no raw error)

v7 requires `launchPTY` to leave `PaneKernelStartToken == ""` (unstamped) whenever **any**
of these hold at the capture boundary:

- `processStartToken(identity.PanePID)` returns `ok == false` or an empty token (pane already
  gone, `/proc` unreadable, a non-Linux host) — the v6 empty-token case (A3''); **or**
- the post-read `ProbeTmuxLiveness` is non-`Healthy` — pane-dead (`:257`), pane-missing/
  unavailable (`:217-233`), pane-pid-mismatched (`:260`), or pane-start-mismatched
  (`:265`) — the v7 capture-boundary case (A3''').

An unstamped token means the launch-time kernel identity could not be **structurally bound**,
so `RunHelper`'s `result.PaneKernelStartToken != ""` assertion accepts **no** control
connection for that launch. The floor is still reached — never via a raw launch/control
error: the post-rotation reseal is driven by the `#{pane_dead_status}` exit backstop and,
finally, the recovery sweep (both evaluate the same daemon-observed condition under the same
lock, §BC5) → the typed `session_unrecoverable_across_rotation` class. A dead/reaped pane is
*exactly* the case the `#{pane_dead_status}`/recovery-sweep backstops were built to route, so
the capture-boundary miss flows into existing typed-floor machinery with **no new error
path**.

*Falsifier:* **A3'''** — `TestKernelTokenCaptureFailsClosedWhenPaneNotReverifiedLive` (unit,
deterministic via the injectable start-token seam + a stub `TmuxRunner`) and the real-path
"pane dies before kernel-token capture" negative below: with the launched pane forced dead
before the kernel read and the start-token read stubbed to observe a same-pid/reused token,
the helper must **not** stamp `PaneKernelStartToken`, must accept **no** connection, and must
route the typed floor via the backstop — never treat the reused token as the wrapper's
identity and never surface a raw launch error.

### Why capturing through an INJECTABLE seam is load-bearing (testability)

The "pane dies before kernel-token capture" negative cannot rely on winning a real
nondeterministic pid-reuse race. v7 captures the kernel token through a package-level
function variable (e.g. `var processStartToken = ProcessStartToken`, defaulting to the real
reader) so the test can force the **reused-process** observation (a non-empty token that is
*not* the launched wrapper's) deterministically, while a stub `TmuxRunner` drives
`ProbeTmuxLiveness` to report the pane dead/pid-mismatched. The assertion is then
deterministic: a non-empty (reused) token + a non-`Healthy` post-read probe ⟹ no stamp ⟹ no
accept ⟹ typed floor. *Falsifier:* A3''' (the seam exists and the negative fires through it).

### W1 read-permission under W2 (why the daemon CAN read field 22 + probe tmux of the non-dumpable pane) — carried, unchanged

A natural re-attack: does W2's `PR_SET_DUMPABLE(0)` (plus the `sudo` setuid launch resetting
`dumpable`) block the daemon (uid e.g. `halbritt`) from reading `/proc/<pane-pid>/stat` field
22 or probing the pane? **No, and the codebase already proves it:**

- W2 protects the **secrecy** surface — `/proc/<pid>/{environ,mem,fd,maps}` — gated by
  `ptrace_may_access`. **Field 22 (`starttime`) is part of the world-readable
  `/proc/<pid>/stat` (mode `0444`)** and is **not** masked for a non-ptrace reader. It is a
  **non-secret identity discriminator**, the right primitive for a no-replay binding — not a
  secret W2 must hide.
- `ProbeTmuxLiveness` queries the **tmux server** (`display-message`/`has-session`), not the
  pane's `/proc` secrecy surface, so the dumpable bit is irrelevant to it.
- **Decisively (empirical, source-verifiable):** the daemon **already** reads the pane
  wrapper's field-22 token cross-uid today — as the `CaptureTmuxIdentity` kernel fallback
  (`tmux_liveness.go:199`, `ProcessStartToken(panePID)`) **and** in `PIDLiveWithStartToken`
  (`:392-408`), which the supervisor runs against the production non-dumpable pane on every
  liveness poll, and it **already** runs `ProbeTmuxLiveness` against that pane on every poll.
  The W1 kernel-token capture/compare **and** the v7 capture-boundary re-verification reuse
  those **already-working** read paths — they introduce no new permission assumption.

So W1 (field-22 identity) + the v7 re-verification (tmux liveness) and W2 (secrecy of
environ/fd/mem) are **consistent**: W2 hides the secrets; field-22 and tmux pane state are not
among them. *Falsifier:* A3'/A3''' assert the real-path test reads `/proc/<peer-pid>/stat`
field 22 cross-uid and probes tmux against the live non-dumpable pane.

### BC1-W1-CAPTURE — the real-path test (the load-bearing assertion)

`TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` (name retained for continuity; mechanism is
connect-out, not fd-inheritance). Launched through `RunHelper` with `RequireTmux` +
`RunAsUser` (a host integration/game-day test gated on `sudo` + `tmux`, build-tagged like
`tmux_liveness_integration_test.go`), it asserts **together**:

1. **W1 accept, field-22 on BOTH sides, capture re-verified.** The launched pane wrapper
   connects out and sends a frame the daemon **accepts**, where the accept compares
   `/proc/<peer-pid>/stat` field 22 (`ProcessStartToken(peer.pid)`) to the **captured**
   `/proc/<pane-pid>/stat` field 22 (`result.PaneKernelStartToken`, stamped only after a
   passing post-read `ProbeTmuxLiveness`) — not against tmux `#{pane_start_time}` — emitting a
   `reseal_requested`/`unrecoverable_across_rotation` `HelperControlEvent`.
2. **NEGATIVE — pane dies BEFORE kernel-token capture (the v7 capture-boundary guard,
   A3''').** Force the launched wrapper to exit before the kernel read; make the start-token
   read observe a **same-pid/reused-process** token (non-empty, *not* the wrapper's) via the
   injectable seam; drive `ProbeTmuxLiveness` (stub `TmuxRunner`, or a real killed pane) to
   report the pane **dead / pid-mismatched**. Assert the helper **does not stamp**
   `PaneKernelStartToken`, **accepts no** control connection, and routes the typed
   `session_unrecoverable_across_rotation` floor via the `#{pane_dead_status}`/recovery-sweep
   backstop — **never** treats the reused token as the launched wrapper's identity and
   **never** surfaces a raw launch/control error.
3. **NEGATIVE — same pid, mismatched/stale kernel start token (the accept-time pid-reuse
   guard, A3').** A connection presenting `peer.pid == result.PID` but whose
   `ProcessStartToken(peer.pid)` ≠ the captured kernel token is **REFUSED** (W1's field-22
   mismatch branch, `PIDLivenessIdentityMismatch`, `tmux_liveness.go:404`). This closes the
   `[probe → accept]` window.
4. **NEGATIVE — empty captured kernel token (fail-closed, A3'').** With
   `result.PaneKernelStartToken == ""`, W1 refuses **every** connection (no pid-only accept);
   the floor routes via the `#{pane_dead_status}`/recovery-sweep backstop.
5. **Provider isolation.** The provider child cannot drive a control event — it is not the
   wrapper pid (W1 refuses at the pid check) and has **no inherited fd** (there is none).
6. **Sibling refusal (W1) anywhere in the launch chain.** A non-child/non-wrapper same-uid
   sibling that connects to the same `STRIATUM_SUPERVISOR_CONTROL_ADDR` is **refused** (wrong
   pid, and a different field-22 token even on pid reuse), cannot recover the nonce
   (`/proc/<wrapper-pid>/environ` root-owned under W2; nonce never in env), and has nothing
   in `/proc/<wrapper-pid>/fd` to steal.

The deterministic unit `TestKernelTokenCaptureFailsClosedWhenPaneNotReverifiedLive`
(injectable start-token seam + stub `TmuxRunner`) is the fast, host-independent proof of (2);
the direct-`os/exec` units `TestControlFrameRequiresExpectedWrapperPeerCredentials` /
`TestControlFrameRejectsEmptyCapturedKernelStartToken` /
`TestSiblingSameUIDCannotOpenControlFDOrReplayNonceViaProc` (carried from v5/v6) are kept as
fast coverage of the peer-cred + empty-token + dumpability logic. All are **necessary, not
sufficient** — the real-path test above is the one that fires against the production
tmux/sudo/env-file path with field-22 on both sides and the capture-boundary re-verification
engaged.

### The carried connect-out channel (verbatim in substance from v5/v6 — unregressed)

The capture-boundary re-verification above sits inside the v5/v6 channel, which is preserved:

- **Listener creation (daemon uid, at supervise start).** The helper creates a
  `socket(AF_UNIX, SOCK_SEQPACKET, 0)` listener with `SO_PASSCRED`, bound to a **per-launch
  abstract address** `@striatum-supervisor-ctl/<supervisor_id>/<random>` (abstract → no
  filesystem node, auto-cleaned on close). The helper holds the listener and runs an
  `acceptControlChannel` reader goroutine alongside `pumpPTYProgress`/`forwardPacketStream`
  (`helper.go:200-208`). `SOCK_SEQPACKET` preserves message boundaries (one frame per
  datagram).
- **Address delivery is NON-SECRET, over existing env plumbing.** The address is advertised
  as `STRIATUM_SUPERVISOR_CONTROL_ADDR` via the existing env path that reaches the pane
  (`tmuxEnvArgs` `-e KEY=VAL`, `pty.go:480`,`:536-544`, + the env-file). It is **not** a
  `sensitiveEnvKey` (`pty.go:140-155`). W1 authenticates the *peer*, not knowledge of the
  address.
- **W2 — `PR_SET_DUMPABLE(0)` first.** The pane wrapper calls `prctl(PR_SET_DUMPABLE, 0)`
  (new `go/pkg/agentloop/dumpable_linux.go`, non-Linux stub mirroring
  `process_identity_other.go`) as the **first instruction** of the agentloop entrypoint —
  before it reads the address, before it dials, before any nonce exists. Reinforced by the
  `sudo` setuid `execve` resetting `dumpable` to `/proc/sys/fs/suid_dumpable` (default `0`).
  No inherited control fd exists at all (the env-file shim execs the agentloop binary;
  nothing handed down to steal), and `dumpable=0` makes
  `/proc/<wrapper-pid>/{fd,environ,mem}` root-owned. The connection is dialed **after**
  `dumpable=0`, by the wrapper itself.
- **W3 — nonce delivered daemon→wrapper, AFTER auth.** On a matched connection the helper
  sends a single-use `control_nonce` (per launch, per generation) **down** the authenticated
  connection. The wrapper echoes it on every subsequent frame; the helper rejects any frame
  whose nonce ≠ the issued nonce, and the BC4 generation guard refuses a nonce from a prior
  generation. The nonce never appears in env or on disk. **W1 (peer-cred + re-verified
  kernel token), not the nonce, is the primary and sufficient authentication;** the nonce is
  generation-binding + defense-in-depth.
- **Frame schema.** One `SupervisorControlFrame` per datagram: `{ schema_version:
  "striatum.supervisor_control.v1", type: "reseal_requested" |
  "unrecoverable_across_rotation", supervisor_id, control_nonce }`. It carries **NO** job_id /
  artifact path / kind / body — identity is derived from daemon state (BC2). The PTY
  (provider stdout/stderr) reaches **only** the volume meter (`pumpPTYProgress`, D028);
  `acceptControlChannel` reads frames **only** off the authenticated connection;
  `superviseReportEventTypes` (`supervision.go:19-28`) admits **no** content/output event.

**Structural no-replay (the spine, on the REAL channel).** A sibling `striatum-lane`
process that is neither the provider child nor the launched pane wrapper cannot: (W2) read
`/proc/<wrapper-pid>/{environ,fd}` (root-owned; no inherited fd anyway); (W3) observe the
nonce (delivered daemon→wrapper post-auth, never in env); **and decisively (W1) be accepted
by the daemon** — `SO_PEERCRED` stamps the sibling's real pid (≠ the captured pane pid), and
even on pid reuse the `/proc` field-22 token differs from the **launch-time captured,
capture-boundary-re-verified kernel token**. Grant the sibling the address *and*
(hypothetically) the nonce, W1 still refuses it. **And the captured token can no longer be
the sibling's own** — v7's post-read `ProbeTmuxLiveness` re-verification means the helper
only ever stamps a token it has structurally bound to the still-live launched pane (the
reaping invariant). No-replay holds **structurally on the production tmux channel across the
whole launch lifetime**, with W1's kernel token structurally bound to the launched wrapper.

### Reserved exit codes: PRIMARY signal is the authenticated frame; the exit code is a tmux-observed BACKSTOP (carried, unchanged)

- **Primary (post-rotation prompt / floor):** the pane wrapper sends the typed
  `SupervisorControlFrame` over the **already-authenticated connect-out channel** (now
  authenticated with the capture-boundary-bound kernel token) before exiting;
  `acceptControlChannel` emits the `HelperControlEvent`; the daemon evaluates the
  daemon-observed reseal condition. Structurally same-uid-safe (W1).
- **Backstop (the wrapper can't even send a frame, incl. the capture-boundary fail-closed
  case):** two reserved agentloop exit-code constants (new, `exitcodes.go`):
  `ExitUnrecoverableAcrossRotation = 97` (the Option-4 floor) and
  `ExitResealInFlightRequested = 98` (a latency hint only; forgeability immaterial — the
  daemon never seals on the strength of 98). On the tmux path these are observed via the new
  `#{pane_dead_status}` capture (NOT `result.Cmd.Wait()`, which is the attach client) and
  routed through the existing `agent_exited` branch (`supervision.go:298-306`). On a
  non-tmux/direct launch they still flow through `agentExitPayload`→`processExitCode`
  (`helper.go:427-439`).
- **Final backstop:** the **recovery sweep** evaluates the same daemon-observed condition
  under the same lock (§BC5), so even if neither a frame nor a pane status is observed
  (including the fail-closed empty/unverified-token case), a complete-on-disk post-rotation
  deliverable still gets one daemon-observed reseal attempt (or the typed floor).

**C2 — reserved codes reserved BY COMMITMENT (carried, unchanged).** Choosing a high range
is **not** an auth boundary. After `cmd.Wait()` (`loop.go:365`) the wrapper inspects the
provider child's exit status and, if it is 97 or 98, **remaps it to a non-control
`agent_exited` outcome** (carrying the provider's raw status in a non-reserved field). The
reserved 97/98 are emitted **only** by the wrapper's own typed-error path and the
authenticated frame, never forwarded from the child. *Falsifier:*
`TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker` (A6).

### BC1 — positive intent is DAEMON-OBSERVED, not provider-asserted (carried, unchanged)

`resealInFlightJob` fires **only** when ALL hold, evaluated under the run advisory lock
(§BC5):

1. **A boot-epoch rotation occurred during this job's lease** (recorded packet epoch vs
   current `writeBootEpochFile` epoch). Absent a rotation, the normal seal path applies.
2. **The job is still `running`, the lease is bound, the stamped generation matches** the
   live `jobs.recovery_generation` (BC4), **and** the lease is within grace (BC5). Any
   mismatch → typed floor.
3. **Every required `expected_artifact` path (from daemon state, BC2) is present in the
   job's active worktree AND was AUTHORED THIS ATTEMPT** — see §modified-since-baseline
   build-test. Absent that, route to the floor, never a speculative seal.
4. `resealInFlightJob` then attempts **only daemon-derived artifacts** (BC2) and maps ALL
   validation/backend/front-matter/durability failures to the typed
   `session_unrecoverable_across_rotation` floor (Option-4). Never a silent seal, never a raw
   error.

**Two entry points, one condition, one backstop:** the wrapper's authenticated post-rotation
frame (or `#{pane_dead_status}` 98) **and** the recovery sweep, which evaluates the **same**
condition under the **same** lock **before** requeuing.

**Backend-gate routing (carried).** `resealInFlightJob` does **not** reuse
`HandleCompleteWork`; it calls the lower-level complete core and **deliberately bypasses
`ensureWorkSessionBackend`** (`lifecycle.go:1181`) — the reseal exists *precisely because*
the live connection is gone — so a stopped supervisor routes the typed class rather than
leaking `invalid_transition`/backend errors. *Falsifier:*
`TestResealExit98BypassesBackendGateOrRoutesTyped` (A9).

### Modified-since-baseline build-test (carried forward — the v5/v6 precision item)

The "deliverable observed" condition (3) must **not** treat "present + absent from
`write_scope_baseline.changed_paths`" as sufficient by itself: for per-job isolated worktrees
the baseline is **nil** (`write_scope_guard.go:81-83`), and source-change publication already
attributes authorship via `gitChangedPathSnapshots` (`write_scope_guard.go:225`) +
`collectInScopeAuthoredPaths` (`artifact_source_publish.go:259-263`, used at `:88`;
`claim.go:622`). `resealInFlightJob` **reuses that authored-path attribution** so an
UNCHANGED pre-existing expected path is **NOT** resealed. *Falsifier:*
`TestResealRequiresAuthoredExpectedArtifactChange` (seed a clean pre-existing expected path →
typed floor; modify it → positive reseal) or the positive
`TestCodexResealUsesReceiverNotProviderStdout` (A8).

## BC2 — Artifact identity from daemon state, never from output (resolved, carried forward)

`resealInFlightJob` derives the expected-artifact set from **its own state** and refuses any
unexpected path, reusing the existing handler payload contracts verbatim:

- **`artifact.publish`** requires `session_id`/`job_id`/`lease_id`/`kind`/`logical_name`/`path`
  (`artifact.go:52-60`), takes `lockRunForJob` first (`HandlePublishArtifact`,
  `artifact.go:64-83`).
- **`work.complete`** requires `session_id`/`job_id`/`lease_id` (`lifecycle.go:1124-1130`).
- **`interrogation.answer`** requires `session_id`/`interrogation_id`/`body`
  (`interrogation.go:217-221`).

For a reseal **complete**, the daemon resolves `jobs.expected_artifacts_json`
(attempt-resolved via `resolveExpectedArtifactCycles`) and verifies every required artifact
is durable, reusing `verifyRequiredArtifacts` (`mutations.go:828-876`) and
`ensurePerJobPublishedArtifactsDurable` (`artifact_durability.go`). For a reseal **publish**,
the daemon publishes **only** a `path` that is an open entry in the job's
`expected_artifacts`, reading the body from the job's own worktree, and **refuses any path
not in the expected set**. The signal supplies neither path nor content. A
front-matter/author-line failure (publisher exit code 6) records the
`session_unrecoverable_across_rotation` blocker with the validation error — the Option-4
floor, never a silent drop. *Test:* `TestCodexResealUsesReceiverNotProviderStdout` (negative
+ positive).

## BC3 — `CapabilityReseal` is a daemon-internal marker, not a public bearer (resolved, carried forward)

- **Projection, not presentation.** `resealInFlightJob` maps `supervisor_id` → `session_id`
  from the supervision row (the same lookup `recordSuperviseReportEvent` uses via
  `findReportSupervisor`, `supervision.go:497-528`), constructs an **internal**
  `rpc.AuthContext{Capability: CapabilityReseal, SessionID, RepositoryID}` **without** the
  public `Authorize` prelude (`rpc/server.go:107-111`), threads it with `WithAuthContext`,
  and calls the lower-level publish/complete routines against the job's active worktree. No
  bearer reaches the lane.
- **Public route-alternate kept test-only.** `MethodEntry.ResealAlternate` set true on only
  `interrogation.answer`/`work.complete`/`artifact.publish`; the prelude re-authorises
  against `CapabilityReseal` on a `capability_missing` for those routes and records
  `AuthContext.Capability == reseal` (never `write`). With **no production reseal bearer**,
  this path is exercised **only by the guardrail tests**. `registry_methods.go` is generated,
  so `ResealAlternate` lands in `contracts/daemon_methods.json` + the `MethodEntry` struct
  (`rpc/registry.go`) + the regenerated map + a reseal column in
  `docs/reference/command-authority-matrix.md` + the authority guardrail.

*Test:* `TestResealCapabilityIsDaemonInternalNotBearer` (A1).

---

# Lifecycle cluster (BC4 + BC5)

## BC4 — Concrete monotonic generation column for the split-brain guard (resolved, carried forward)

`jobs` is **owner-held**, so a column-add is owner DDL —
`TestFutureRuntimeMigrationsDoNotCarryOwnerDDL` forbids a runtime migration from ALTERing it.
v7 ships owner bundle **`go/pkg/db/sql/owner/0021_job_recovery_generation.sql`**: `ALTER
TABLE striatumd.jobs ADD COLUMN IF NOT EXISTS recovery_generation integer NOT NULL DEFAULT
0;`, bumps `LatestOwnerBundleVersion` **20→21** (`owner.go:23` — **confirmed currently 20**;
`RequiredOwnerBundleVersion = LatestOwnerBundleVersion`, `owner.go:35`, moves with it) with
the ordinal-21 `[[owner_bundle]]` reservation in `RESERVATIONS.toml`
(`go/pkg/db/reservations.go`), modelled exactly on the credited `review_generation`
precedent (`owner/0009_review_generation.sql`).

- **Degrade-safe presence probe.** `db.JobRecoveryGenerationColumnPresent` (mirroring
  `SessionPipeReadColumnPresent` / `ArtifactPlacementColumnPresent` / `reviewGenerationEnabled`,
  `db/artifact_write.go:64-102`). Column absent → route to the typed floor.
- **Increment points (each in the same UPDATE that retires/rebinds the job's authoritative
  lease, all under `lockRun`):** (1) **claim** — `claimChosenJob` (`claim.go:222-228`); (2)
  **requeue (same attempt)** — `requeueJobSameAttempt` (`recovery.go:2097-2109`); (3)
  **recovery sweep expire/transfer/respawn** — the `current_lease_id = NULL` transitions in
  `HandleRecoveryAuto`/`SweepRun` (`recovery.go:619`/`:2546`/`:2854`/`:2935`); (4)
  **release** — `work.release`. Monotonic by construction (only `+1`).
- **Stamped value.** `claimChosenJob` writes the post-increment `recovery_generation` into
  the work-packet `lease` block (`buildPacket`, `claim.go:229-260`) as
  `lease.recovery_generation`. At reseal, `resealInFlightJob` reads the stamped value and
  compares it to the **live** `jobs.recovery_generation` under the lock — equal → proceed;
  unequal → typed class.

*Tests:* `TestResealPredicateUsesStampedRecoveryGeneration` /
`TestResealTokenRefusedAfterLeaseExpiryAndRecoveryRequeue` (A10, A15).

## BC5 — Numeric grace + PINNED migration site + CORRECTED lock order (resolved, carried forward)

### BC5-(1) — `leases.reseal_grace_extended_at` PINNED to owner bundle 0021

`striatumd.leases` is created in **runtime migration 0005**
(`0005_repo_local_workflow_state.sql:166`) and is **owner-held**: it is **NOT** in the
migration-0016+ ownership-transfer cohort (`owner/0018_runtime_table_ownership_transfer.sql`
— `leases` absent). So a column-add to `leases` is **owner DDL**. **Pinned:**
`reseal_grace_extended_at timestamptz` (NULL until used) is added in the **same owner bundle
0021** as `jobs.recovery_generation` — a second statement in
`owner/0021_job_recovery_generation.sql`: `ALTER TABLE striatumd.leases ADD COLUMN IF NOT
EXISTS reseal_grace_extended_at timestamptz;`. (Like `review_generation`, `striatumd_rw`'s
table-level grants extend to the new column; no new grant.) *Falsifier:*
`TestRuntimeMigrationsDoNotCarryOwnerDDLForLeasesGraceColumn` (folded into
`TestFutureRuntimeMigrationsDoNotCarryOwnerDDL`) (A14).

### BC5-(2) — numeric grace + one extension + CORRECTED lock order

- **`resealGrace` numeric + source + maximum.** `const resealGraceWindow = 30 *
  time.Second` (new, beside the lease constants in `go/pkg/mutations`), **hard-capped** at the
  packet heartbeat window: `grace = min(resealGraceWindow,
  packet.lease.heartbeat_after_seconds)`. Daemon-side allowance, **not** a lane-invokable
  `work.heartbeat` (`CapabilityReseal` carries no heartbeat verb).
- **One same-lease extension only**, gated by `leases.reseal_grace_extended_at` (NULL until
  used). Allowed only if `now() - expires_at ≤ grace` AND `jobs.recovery_generation ==
  stamped` AND `reseal_grace_extended_at IS NULL`. A second expiry or any generation change →
  typed floor.
- **CORRECTED lock-order gate map.** `HandleCompleteWork` (`lifecycle.go:1119`) runs
  `enforceSessionBindingForSession` (`:1137`) + `enforceActiveActingSession` (`:1147`,
  conditional) **BEFORE** `lockRunForJob` (`:1154`), and `activeLeaseFor` (`:1178`) +
  `ensureWorkSessionBackend` (`:1181`) **AFTER**. `resealInFlightJob` does **not** call
  `HandleCompleteWork`; it is a distinct mutation with this exact gate map:

  | `HandleCompleteWork` gate | site | `resealInFlightJob` |
  | --- | --- | --- |
  | `enforceSessionBindingForSession` (pre-lock) | `:1137` | **SKIP → REPLACE** with the supervision-row projection (`supervisor_id`↔`session_id`, BC3) + the bound `work_packets` row. |
  | `enforceActiveActingSession` (pre-lock) | `:1147` | **SKIP → REPLACE** — supervisor stopped at exit/frame; binding proven from daemon state. |
  | `lockRunForJob` (run advisory lock) | `:1154` | **REPLAY FIRST** — `pg_advisory_xact_lock(hashtext(run_id))` before any `FOR UPDATE`. |
  | `FOR UPDATE` jobs → leases → job_recovery_state | (rows) | **REPLAY** in stable key order under the run lock. |
  | `activeLeaseFor` (raw `lease_error` on expiry) | `:1178` | **SKIP → REPLACE** with the **reseal predicate** (generation match BC4, within-grace BC5, lease bound to this job/session, session not retired) → typed floor on any miss; **never a raw `lease_error`.** |
  | `ensureWorkSessionBackend` (live backend) | `:1181` | **SKIP (BYPASS)** — the reseal exists *because* no backend is live. |
  | `enforceWriteScopeClean` + `verifyRequiredArtifacts` + `ensurePublishedArtifactsDurable` + `running→completed` | `:1191-` | **REPLAY** — the actual transition under the lock; any failure → typed floor. |

- **Serialization vs `artifact.publish` / `work.complete` / the recovery sweep.** All take
  `lockRunForJob` / `lockRun(run_id)` (the same `pg_advisory_xact_lock(hashtext(run_id))`,
  `mutations.go:663-665`, RFC 0104) **FIRST**. The sweep drains helper events in short txns
  BEFORE `lockRun` (`recovery.go:575-590`) but expires/requeues INSIDE the `lockRun` tx
  (`:610-621`). So: sweep-wins → it bumps the generation; reseal then blocks, acquires the
  lock, observes the changed generation/expired-beyond-grace lease, routes the typed class
  (**never revives a requeued lease**). Reseal-wins → seals within grace (`running→completed`);
  the sweep then sees a completed job and does not requeue.
- **Expired-beyond-grace ALWAYS routes the typed class** — `resealInFlightJob` never calls
  `activeLeaseFor`; the reseal predicate returns `ErrSessionUnrecoverableAcrossRotation` → the
  durable blocker. No raw `lease_error` ever reaches a post-rotation reseal.

*Tests:* `TestResealBeyondGraceRoutesTypedNotLeaseError`,
`TestResealGraceCannotReviveRequeuedLease`, `TestRecoveryRequeueWinsOverExpiredLeaseReseal`,
`GD-1b`, `TestResealExit98BypassesBackendGateOrRoutesTyped`, and the 0021-migration guard
(A11–A13).

---

## The one place it lands: `resealInFlightJob` (contract sketch)

```
resealInFlightJob(repositoryID, supervisorID, intent):   // intent ∈ {complete, publish, answer}
  withTxRetryOnDeadlock:
    session   := supervisorSession(supervisorID)            // process_supervisors row; closed/none -> typed floor
    job, pkt  := inFlightJobAndPacket(session)              // bound work_packets row (stamped recovery_generation)
    lockRunForJob(job.run_id)                               // pg_advisory_xact_lock — BEFORE any FOR UPDATE
    jobRow    := SELECT ... FROM jobs   WHERE job_id FOR UPDATE
    leaseRow  := SELECT ... FROM leases WHERE lease_id FOR UPDATE
    SELECT ... FROM job_recovery_state WHERE job_id FOR UPDATE
    if !JobRecoveryGenerationColumnPresent: return typedFloor("generation-unverifiable")
    if session not active:                  return typedFloor("session retired")
    if leaseRow.owner != session or leaseRow.resource != job: return typedFloor("lease not this job/session")
    if jobRow.recovery_generation != pkt.lease.recovery_generation: return typedFloor("generation changed")
    if !bootEpochRotatedSincePacket(pkt):   return typedFloor("no post-rotation case")
    if leaseRow.expired:
        if within grace and generation matches and reseal_grace_extended_at IS NULL:
            UPDATE leases SET expires_at = now()+grace, reseal_grace_extended_at = now()   // ONE extension
        else: return typedFloor("expired beyond grace")
    if !supervisedEpochAccepted(session):   return typedFloor("epoch missing/mismatch")    // F7 channel half
    if !allRequiredExpectedArtifactsAuthoredThisAttempt(job, pkt): return typedFloor("deliverable not observed")
    authCtx := internal AuthContext{Capability: reseal, SessionID: session}                // BC3 projection
    switch intent:                                                                         // bypasses ensureWorkSessionBackend
      complete: enforceWriteScopeClean(job); verifyRequiredArtifacts(job); completeCore(authCtx, job, lease)  // BC2
      publish:  require path ∈ expected_artifacts ; publishArtifactWithOptions(authCtx)    // BC2
      answer:   interrogationAnswerCore(authCtx, ...)
    // any binding/write-scope/front-matter/backend/durability failure -> typedFloor(reason)
```

The **W1 authentication is upstream of this sketch**, in `acceptControlChannel`
(`RunHelper`): a frame is only delivered to `resealInFlightJob` after the connection is
W1-accepted with the **capture-boundary-bound kernel start token** (fail-closed on empty
**or** on a failed post-read `ProbeTmuxLiveness` re-verification). The sketch is also the
**recovery-sweep entry point** (same predicate, same lock, before requeuing) and the
**connect-out-frame entry point**. `typedFloor(reason)` records the durable
`session_unrecoverable_across_rotation` blocker (Option-4).
`allRequiredExpectedArtifactsAuthoredThisAttempt` uses the
`gitChangedPathSnapshots`/`collectInScopeAuthoredPaths` authored-path attribution (nil
baseline for isolated worktrees).

## Security invariant (the spine) — held and strengthened

The runtime `client-token` carries the full `bootstrapCapabilities` and is `0600` in a
`0700` dir (AF3). **Any path that lets a lane read that file, or mints a lane-readable
credential carrying any of `{admin, apply, recovery, surgical_recovery}`, is categorically
out of bounds for a FIX** — v7 keeps it structurally impossible:

- The lane never gets OS read of the `0700` dir (AF3); the Slice-A floor removes the only
  code path that would have read the `client-token` (`token.go:31-42` / `endpoint.go` return
  the typed error for a supervised lane).
- The only new authority, `CapabilityReseal`, carries **no elevated verb** and is **never
  materialised into any lane-readable file or bearer** (BC3 + F2).
- The reseal is **projected by the daemon only** on the supervisor-proven path.
- The control channel is a **connect-out authenticated by kernel `SO_PEERCRED` pid + a
  launch-time-captured, capture-boundary-re-verified `/proc` field-22 kernel start token**
  (W1, one clock domain, structurally bound to the launched pane), with the wrapper
  non-dumpable (W2) and the nonce delivered post-auth (W3) — **no bearer, no inherited fd, no
  secret in env** — and a sibling that connects is refused **structurally** on the real tmux
  path. W1 **fails closed** when the kernel token cannot be captured **or cannot be bound to
  a still-live launched pane** (never a pid-only accept). The exit-97 floor is a
  tmux-`#{pane_dead_status}`-observed backstop, not a forgeable primary.
- The epoch republish moves **endpoint + epoch only** over the daemon-owned,
  integrity-protected path (F7 file-mirror, kept); never the admin token.

*Falsifier:* `TestResolveRefusesRuntimeClientTokenForLane` (A2) —
`ResolveTokenMaterial`/`ResolveTokenMaterialFresh` return `ErrSessionUnrecoverableAcrossRotation`
for a supervised lane, never the runtime `client-token`.

## Falsifiable assertions (each with the named test / game-day that refutes it)

- **A1 — No-widening.** `CapabilityReseal` carries only the three reseal verbs and is
  daemon-internal. *Refuted if* `TestResealTokenCanReachOnlyResealRoutesWithoutWrite` /
  `TestResealCapabilityIsDaemonInternalNotBearer` shows it reaching any of
  `admin`/`apply`/`recovery`/`surgical_recovery`/`work.claim_next`/any non-reseal route,
  resolving to `write`, or presentable as a bearer.
- **A2 — No admin-token fall-through.** *Refuted if*
  `TestResolveRefusesRuntimeClientTokenForLane` returns the runtime `client-token` for a
  supervised lane instead of the typed error.
- **A3 — No-replay, STRUCTURAL (direct harness, KERNEL token both sides).** Every accepted
  control frame is bound to the launched wrapper pid + the **captured `/proc` field-22 kernel
  start token**. *Refuted if* `TestControlFrameRequiresExpectedWrapperPeerCredentials`
  accepts a frame from any pid other than the launched wrapper's, or whose
  `ProcessStartToken(peer.pid)` ≠ the captured kernel token, or
  `TestBorrowedResealBearerCannotSealVictimSession` finds an on-disk reseal bearer or a
  sibling/foreign-session/provider-child sealing session A's job.
- **A3' — No-replay, STRUCTURAL, on the PRODUCTION tmux channel, field-22 BOTH sides (the v6
  closure, carried).** *Refuted if* `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper`
  (launched through `RunHelper` with `RequireTmux`/`RunAsUser`) shows: the accept comparing
  `ProcessStartToken(peer.pid)` against tmux `#{pane_start_time}` rather than the
  **launch-time-captured `/proc` field-22 kernel token**; OR a same-uid sibling connection
  **accepted** (wrong pid/token should refuse); OR a **same pid with a mismatched/stale kernel
  start token accepted** (the accept-time pid-reuse guard must refuse, closing `[probe →
  accept]`); OR the provider child driving a control event; OR the launched pane wrapper's
  authenticated frame **not** accepted; OR the captured token taken at accept rather than at
  launch (`launchPTY:493-504`); OR an inherited control fd present.
- **A3'' — W1 fails closed on an empty/unreadable captured kernel token (no pid-only degrade;
  carried).** *Refuted if* `TestControlFrameRejectsEmptyCapturedKernelStartToken` (and the
  real-path negative) shows W1 accepting a connection when `result.PaneKernelStartToken == ""`
  (i.e. passing `""` into `PIDLiveWithStartToken`, degrading to a pid-only check), instead of
  refusing every connection and routing the floor via the
  `#{pane_dead_status}`/recovery-sweep backstop.
- **A3''' — W1's kernel token is STRUCTURALLY bound to the still-live launched pane at the
  capture boundary (the v7 closure).** The helper stamps `PaneKernelStartToken` **only after**
  a post-read `ProbeTmuxLiveness` re-verification returns `Healthy`/`TmuxLivenessOK` with
  `ObservedPanePID == identity.PanePID`; a pane that is dead/missing/pid-mismatched/
  start-mismatched at the boundary leaves the token unstamped and accepts no connection.
  *Refuted if* `TestKernelTokenCaptureFailsClosedWhenPaneNotReverifiedLive` (deterministic,
  injectable start-token seam + stub `TmuxRunner`) **or** the real-path "pane dies before
  kernel-token capture" negative shows: the helper stamping a kernel token without a passing
  post-read `ProbeTmuxLiveness`; OR accepting any connection in the pane-died-then-pid-reused
  case (a non-empty reused token treated as the wrapper's identity); OR the re-verification
  probe taken BEFORE the kernel read rather than after; OR a capture-boundary miss surfacing a
  raw launch/control error instead of routing the typed
  `session_unrecoverable_across_rotation` floor via the backstop.
- **A4 — `/proc` surface closed (W2/W3).** *Refuted if*
  `TestSiblingSameUIDCannotOpenControlFDOrReplayNonceViaProc` opens `/proc/<wrapper-pid>/fd/*`
  or recovers the nonce from `/proc/<wrapper-pid>/environ` as a same-uid non-wrapper process,
  or the nonce is found in the wrapper env. (Field 22 of `/proc/<pid>/stat` is a non-secret
  identity discriminator, not a secret W2 hides — the daemon's cross-uid field-22 read and
  tmux probe are the already-exercised liveness read paths, `tmux_liveness.go:199`,`:392-408`,
  `ProbeTmuxLiveness`.)
- **A4' — W2 ordering on the real path.** *Refuted if* the real-path test shows the wrapper
  readable (`dumpable != 0`) at any point before it connects out, or the nonce observable in
  env at any point in the launch chain (it must be delivered daemon→wrapper post-auth only).
- **A5 — Control path never parses output.** *Refuted if*
  `TestPTYOutputCannotEmitSupervisorControlEvent` / `TestProviderOutputCannotDriveResealOrBlocker`
  shows PTY/stdout bytes driving a reseal or blocker.
- **A6 — Reserved exit codes reserved by commitment (C2).** *Refuted if*
  `TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker` shows a provider child's
  97/98 propagated into the reserved codes.
- **A7 — Floor is a typed signal recorded without parsing.** *Refuted if*
  `TestAgentLoopReservedExitCodeRoutesUnrecoverableWithoutPTYParsing` shows exit 97 failing to
  route the durable blocker, or the decision reading output bytes.
- **A7' — Exit-code backstop observed via `#{pane_dead_status}`, not the attach client.**
  *Refuted if* the tmux-path floor reads the pane wrapper's reserved exit from
  `result.Cmd.Wait()` (the attach client) rather than the `#{pane_dead_status}` capture, or a
  pane-emitted 97/98 fails to route on the production path while the attach client exits 0.
- **A8 — Positive intent is daemon-observed + authored-this-attempt.** *Refuted if*
  `TestCodexResealUsesReceiverNotProviderStdout` shows a seal driven by provider-asserted
  intent without the daemon-observed condition, OR a path outside `expected_artifacts`
  accepted, OR `TestResealRequiresAuthoredExpectedArtifactChange` reseals an UNCHANGED
  pre-existing expected path.
- **A9 — Backend-gate routing.** *Refuted if* `TestResealExit98BypassesBackendGateOrRoutesTyped`
  leaks `invalid_transition`/backend errors instead of sealing via the internal core or
  routing the typed class.
- **A10 — No split-brain, by stamped generation.** *Refuted if*
  `TestResealPredicateUsesStampedRecoveryGeneration` /
  `TestResealTokenRefusedAfterLeaseExpiryAndRecoveryRequeue` shows a reseal succeeding after a
  generation bump, or publishing into a requeued/retired job.
- **A11 — Numeric grace, never raw `lease_error`.** *Refuted if*
  `TestResealBeyondGraceRoutesTypedNotLeaseError` yields a raw `lease_error`, or grace exceeds
  `min(resealGraceWindow, heartbeat_after_seconds)`.
- **A12 — One extension, no revive.** *Refuted if* `TestResealGraceCannotReviveRequeuedLease`
  extends a lease twice or revives a requeued lease.
- **A13 — Lock order serializes reseal vs sweep.** *Refuted if*
  `TestRecoveryRequeueWinsOverExpiredLeaseReseal` or the `run_lock_guard_test.go` guardrail
  shows a reseal taking a run-scoped `FOR UPDATE` before `pg_advisory_xact_lock`, or an
  interleave that split-brains.
- **A14 — Grace marker migration pinned (owner DDL).** *Refuted if* a runtime migration
  carries the `leases.reseal_grace_extended_at` ALTER
  (`TestFutureRuntimeMigrationsDoNotCarryOwnerDDL` /
  `TestRuntimeMigrationsDoNotCarryOwnerDDLForLeasesGraceColumn`), or the column lands outside
  bundle 0021.
- **A15 — Generation column migration pinned (owner DDL).** *Refuted if* a runtime migration
  carries the `jobs.recovery_generation` ALTER, or the `LatestOwnerBundleVersion` 20→21 bump
  is absent.
- **A16 — Epoch path does not weaken #316.** *Refuted if*
  `TestResealEpochMirrorRejectsTamperOrMissingEpoch` shows a lane-writable epoch source, a
  successful symlink/replace, or a missing-header supervised request accepted.
- **A17 — Token validity survives the restart.** *Refuted if* `TestTokenValidAcrossRestart`
  shows the PG-resident token rejected purely because the process restarted.
- **A18 — Loud, durable, lease-bounded failure.** *Refuted if* game-day **GD-1** (restart
  `striatumd` mid-job, no reachable token file) shows a silent unsealed exit, a raw permission
  error, or no durable `session_unrecoverable_across_rotation` blocker; or **GD-1b** yields a
  raw `lease_error`, stale-lease limbo, or a silent unsealed exit instead of a same-lease
  renew-and-seal within grace or the typed class; or **GD-CTL** (the connect-out real-path
  game-day) shows a sibling sealing, the wrapper unable to reach the daemon over the
  connect-out channel under `RequireTmux`/`RunAsUser`, the W1 accept comparing anything other
  than `/proc` field 22 on both sides, **or the helper stamping a kernel token for a pane it
  did not re-verify still-live at the capture boundary** (GD-CTL extends the "pane dies before
  capture" race against the real tmux/sudo path).

## Adapter survival matrix (F6 — honest, re-grounded on the daemon-observed trigger)

No adapter needs to reload its MCP launch args to seal the in-flight job: the seal is
daemon-side (`resealInFlightJob`), triggered by the **daemon-observed post-rotation
condition** (prompted by the authenticated connect-out frame, the `#{pane_dead_status}`
backstop, or the recovery sweep) — adapter-independent and not parsed from provider output.

| Adapter | Reseal-in-flight (Slice B) | Resume normal MCP work after rotation |
| --- | --- | --- |
| **Claude** (ephemeral MCP config) | daemon-observed condition → `resealInFlightJob` (no token reload) | #323 ephemeral-config rewrite + endpoint/epoch republish |
| **Agy / pipe** | same daemon-side path | same as Claude where supported |
| **Codex** (MCP URL baked into launch `-c` args) | same daemon-side path — **no in-place MCP survival claimed** | operator-assisted relaunch / `supervise rebridge` only |

*Refuting game-day — GD-Codex-Reseal-Rotation:* restart `striatumd` mid-job for a Codex lane;
the in-flight job seals over the daemon-observed path **or** fails legibly to Option 4; the
spec does **not** claim the Codex MCP client reconnected in place.

## Scope discipline (Non-Goals held)

- Does **not** re-classify the downstream `agent_exited_unsealed` recovery policy (RFC 0152 /
  D249).
- Does **not** change committee POSIX-ACL repo provisioning (#537 / #539).
- Does **not** touch `run drive`'s transient-socket behavior (#513).
- Does **not** weaken the #316 boot-epoch recycled-port defense (BC1/F7 strengthen it).
- Does **not** introduce any lane-readable credential file (the v1 `0600` reseal file stays
  retired).
- Does **not** collide with the RFC 0125 `HandleRecoveryReseal` worktree-durability verb
  (separate file `recovery_reseal.go`, separate verb, unrelated to credentials);
  `resealInFlightJob` lands in `recovery_reseal_rotation.go`.
- Local-first, single-host, daemon-owned PostgreSQL as the single writer.

## Maintainer ratification gate (required)

**Slice B introduces a new daemon-internal capability marker (`rpc.CapabilityReseal`), a
test-only auth-prelude route alternate, the daemon-owned supervisor control channel with
connect-out `SO_PEERCRED` (pid + launch-time-captured, capture-boundary-re-verified `/proc`
field-22 kernel start token) authentication, the reserved agentloop exit codes, the
`jobs.recovery_generation` + `leases.reseal_grace_extended_at` owner-bundle-0021 columns, and
endpoint/epoch republish plumbing — a security/authz trust-model change.** This cleared spec
is a **RECOMMENDATION the maintainer ratifies before any build slice touches credential
code.** Slice A (the Option-4 typed floor) is zero-trust-change and may land first under the
normal review gate **now that BC1-W1-CAPTURE routes it over the real, non-PTY connect-out
channel whose same-uid authentication (W1) is specified with ONE coherent kernel identity
token STRUCTURALLY BOUND to the launched wrapper** — the captured `/proc` field-22 kernel
start token, stamped only after a post-read `ProbeTmuxLiveness` re-verification (the reaping
invariant), compared field-22-to-field-22, fail-closed on an empty or unverified token.
Adjudicator clearance gates the spec's **soundness**; it is not the maintainer's product call
on the credential code.

---
<sub>Holder revised proposal (design-v7) for the RFC 0143 falsification-gate design run.
Resolves the single remaining binding constraint BC1-W1-CAPTURE by adding a fail-closed
capture-boundary re-verification that STRUCTURALLY binds the W1 kernel start token to the
still-live launched pane: after `CaptureTmuxIdentity` reports `identity.PanePID`
(`pty.go:493-504`), capture `ProcessStartToken(identity.PanePID)` (`/proc` field 22,
`process_identity_linux.go:13-32`) through an injectable seam, then re-verify via
`ProbeTmuxLiveness` (pane-dead `tmux_liveness.go:257`, pane-pid mismatch `:260`, pane-start
mismatch `:265`) AFTER the read and BEFORE stamping `PaneKernelStartToken` / before any
accept loop binds the channel; stamp ONLY on a passing post-read probe, else leave the token
unstamped (engaging the v6 `!= ""` no-pid-only-accept rule) and route the typed
`session_unrecoverable_across_rotation` floor via the `#{pane_dead_status}`/recovery-sweep
backstop — never a raw launch/control error. The binding is structural, not temporal: pid
reuse requires the previous owner be reaped; the tmux server is the pane child's reaper;
reaping flips the pane out of the live state — so a `Healthy` post-read `ProbeTmuxLiveness`
with matching pane pid proves the kernel read saw the launched pane's own process. The
`[probe → accept]` residual is closed by the retained accept-time field-22 match (A3').
Extends `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` with a "pane dies before
kernel-token capture" negative (deterministic sibling
`TestKernelTokenCaptureFailsClosedWhenPaneNotReverifiedLive`). Retains the v6
field-22/field-22 operand fix (`LaunchResult.PaneKernelStartToken`,
`ProcessStartToken(identity.PanePID)`, A3'/A3'') and carries the v6-credited set (connect-out
topology + named plumbing sites + non-secret address + post-auth nonce W3 + W2 ordering +
`#{pane_dead_status}` backstop + C2 + BC2/BC3/BC4/BC5 + daemon-observed positive intent +
backend-gate bypass + W1/W2/W3 shapes + F2/F4/F7-file/AF1/AF4/no-widening/A1–A18) forward
unregressed, folding in the modified-since-baseline authored-path build-test. The
adjudicator's collaboration ledger — not falsifier completion — decides whether this gate
clears.</sub>
