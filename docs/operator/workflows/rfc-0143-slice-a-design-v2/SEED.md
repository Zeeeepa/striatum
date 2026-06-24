# Design-Run Seed (v2 / REVISION) — RFC 0143 **Slice A**: the legible `session_unrecoverable_across_rotation` typed-exit floor

> **This is the SECOND Slice-A design run (v2 / REVISION).** The v1 run
> (`rfc-0143-slice-a-design`, `run_2065faf1`) ran the same falsification gate and
> returned **`needs_revision`** (gate uncleared; the v1 revision budget was consumed
> without a holder spec change because the gate cycle re-attacks the falsifiers, not
> the holder — revision happens via THIS fresh run). The v1 dialogue is banked under
> `docs/operator/artifacts/rfc-0143-slice-a-design/` and wired in as required
> `context_docs`: the v1 **`HOLDER.md`** (the spec you revise) and the **v1
> cycle_2 collaboration ledger** (the two open findings + the credited carry-forward
> set). Read both in full before producing any artifact.
>
> RFC 0143 is decided (**D261**): Slice A ships now as **PURE daemon-side
> observability** — the Option-4 typed `session_unrecoverable_across_rotation` floor
> that **mints no credential, widens no token, and touches no trust model**. Slice B
> (the `CapabilityReseal` authority + the W1 connect-out channel) is **OUT OF SCOPE,
> blocked on [RFC 0168](../../../rfcs/0168-per-lane-security-principal.md) (#585).**
> Do NOT design any of Slice B. The deliverable is the revised **falsifiable
> implementation spec** (`PROPOSAL.md`) the `rfc-0143-slice-a-build` run executes TDD.

## What v1 credited — carry forward UNREGRESSED (do NOT reopen)

The v1 gate credited the following as sound; both v1 falsifiers confirmed no widening
and no Slice-B dependency. The v2 revision MUST preserve them verbatim from the v1
`HOLDER.md` where applicable; reopening any is a regression that fails the gate.

- **§1 — the reserved agentloop exit code + sentinel.**
  `ExitUnrecoverableAcrossRotation = 97` and `ErrUnrecoverableAcrossRotation` in a new
  `go/pkg/agentloop/exitcodes.go`. Slice A owns ONLY code 97 — NOT reseal-98,
  `resealInFlightJob`, the connect-out channel, the kernel-token capture,
  `CapabilityReseal`, or owner bundle 0021 (all Slice B).
- **§2 — Spot 1 credential-chain NARROWING (A1/A4, no widening).**
  `adminTokenReachedByNonOwner` applied AHEAD of the read at the runtime
  `client-token` tier in both resolvers (`ResolveTokenMaterial` `token.go:31-42`,
  `ResolveTokenMaterialFresh` `endpoint.go:125-136`), returning
  `("", ErrUnrecoverableAcrossRotation)` BEFORE any `os.ReadFile` for a non-owner
  lane; `ReadTokenFile`'s owner-mode guard (`token.go:75-92`) retained; owner
  unaffected. This is a NARROWING, never a widening. **Keep it** — but note it is NOT
  the central producer (see FIX-1).
- **§3.2–3.4 — exact-code-only classification (A2, no over-fire by attribution).**
  The new `stallClassSessionUnrecoverableAcrossRotation` is interposed FIRST, gated on
  the observed exit code `== 97`, ahead of `agent_exited_unsealed` / `agent_pid_dead`
  in `recoverStuckJobs`. The exact-code rule (refuse to infer the floor from
  "complete-on-disk + lane-lost" alone) is the correct attribution discipline — keep it.
- **§3.5 — launch-handshake dissolution.** 97 is produced only AFTER `agent_started`,
  so a genuine launch failure stays a raw `helper_error` (the v7
  `BC1-W1-CAPTURE-FLOOR` raw-leak is structurally absent for the decoupled world).
- **§3.6 — relationship to `agent_exited_unsealed` + `HandleRecoveryCompleteStalled`
  (#292).** The typed class is a strict refinement that routes the existing recovery,
  not a duplicate/override. **Carry forward — but see the OBSERVABILITY-ONLY
  clarification below: it grants NO new auto-seal authority.**
- **§4 direct-path C2.** A provider child's 97/98 is normalized to a generic error by
  `normalizeAgentExitError` (`loop.go:365-379`) on the DIRECT child-exit path. Keep —
  but it is insufficient for the tmux carrier (see FIX-2).
- **The categorical no-widening invariant + the additive `isNecrosisStallClass` growth.**

## The two binding findings v2 MUST resolve

Both findings share ONE root: **under the shared `striatum-lane` uid, every
lane-side signal (an exit code, a helper event, a `0600` file, the tmux
`#{pane_dead_status}`) is forgeable by a same-uid sibling** — the v7 `BC1-W1-ORACLE`.
The v1 spec leaned on a lane-side reserved exit code as the PRIMARY floor producer,
which both (a) is unreachable on the real #512 path and (b) is forgeable on the tmux
carrier. The v2 fix direction is therefore: **make a DAEMON-SIDE observation the
primary, forge-resistant producer**, and demote the lane-side exit code to a
direct-path corroborator.

### FIX-1 — SA-ROTATION-UNDERFIRE (verdict-driving; carried, unaddressed in v1)

**The defect.** The only v1 producer of 97 is the lane-side credential-resolver
sentinel (Spot 1). But a normal supervised lane launches with its **session-bound
token** as `STRIATUM_MCP_TOKEN` (`supervision_env.go:341-343`), so
`ResolveTokenMaterial` returns at **step 1** (`token.go:19-21`) and NEVER reaches the
step-3 admin `client-token`; the v1 §2.3 rotation guard
`cfg.Token.Source == <runtime client-token>` is therefore **false** for the ordinary
lane, so no 97 is emitted. The actual #512 failure after #316: the lane holds a still
valid bearer but presents a **stale boot epoch** that the daemon rejects as
`stale_daemon_identity` **before** bearer validation / dispatch (`http.go:159-169`,
`validateBootEpoch` `http.go:681-699`); the #323 watcher silently continues when the
owner-only fresh endpoint/epoch files are unreadable by the lane (`loop.go:589-604`);
codex cannot reload its launch-time `-c` MCP URL and the wedge path only writes an
in-PTY prompt then returns `nil` (`loop.go:625-646`). So the lane dies with **no 97**
and recovery records an ordinary class — the floor never fires on the very case Slice
A exists to make legible. **This is the central deliverable; it MUST fire on the real
rotation path.**

**The fix — make the DAEMON's own observation the forge-resistant primary producer
(RECOMMENDED), reachable on the real path:** when the daemon rejects a request as
`stale_daemon_identity` (`validateBootEpoch`, `http.go:166-169/:681-699`), **record
that the presenting session is unrecoverable-across-rotation as durable daemon state**;
the recovery sweep then records the typed `session_unrecoverable_across_rotation`
class for a session that (i) was observed presenting a stale boot epoch, (ii) has its
required `expected_artifacts` complete-on-disk (`verifyRequiredArtifacts` +
`verifyRequiredArtifactReconstructable`), and (iii) is lane-lost
(`#{pane_dead}`/`/proc`/`kill(0)`). This fires on the exact #512 path regardless of
the lane's token source or exit code, and is **forge-resistant** (it is the daemon's
own observation, not a lane-side signal). **Open sub-question the holder MUST resolve
concretely:** `validateBootEpoch` runs **pre-auth** (`http.go:159-169`), so specify
exactly how the daemon **attributes** the rejection to a session — extract the
session id from the presented (stale-but-session-bound) bearer / the request's
session header at that layer, key the durable record on it, and ensure an unattributable
rejection does NOT over-fire — WITHOUT widening any token or trust boundary. If clean
attribution at the pre-auth layer is not achievable, the falsifier-endorsed fallback
is to map the `stale_daemon_identity` response observed on the **lane's own MCP client
path** to `ErrUnrecoverableAcrossRotation` → 97 (still decoupling-safe, no widening),
with the honest tmux-carrier scoping of FIX-2. Either way, **route the codex
rotated-endpoint wedge (`loop.go:625-646`) to the typed floor.**

**Required tests (name them, keep the negatives):**
`TestSessionBoundLaunchTokenDoesNotSuppressStaleEpochFloor`,
`TestRotationWatcherUnrecoverableWhenFreshEndpointUnreadableForLane` /
`TestUnreadableFreshEndpointDoesNotSilentlyContinueWithoutTypedFloor`,
`TestCodexRotatedEndpointWedgeRecordsTypedUnrecoverable`, and a daemon-side
`TestStaleEpochRejectionRecordsUnrecoverableForPresentingSession` (if the
daemon-observed shape is chosen); **keep**
`TestOrdinaryUnsealedExitStaysAgentExitedUnsealed` and
`TestLaunchHandshakeFailureStaysHelperErrorNotFloor` (the no-over-fire negatives).

### FIX-2 — SA-C2-TMUX-FORGE (new in v1 cycle 2; C2/A5 + A3 over-fire)

**The defect.** The v1 Spot-2 tmux carrier `#{pane_dead_status}` (the additive
`PaneDeadStatus` on `ProbeTmuxLiveness`, `tmux_liveness.go:228`) is **forgeable** by a
same-uid provider child: the production tmux surface runs as the shared
`striatum-lane` uid (`pty.go:98-112/:310-314`), the session name is deterministic
(`pty.go:620-633`), the lane command is launched by `tmux respawn-pane -k`
(`pty.go:479-484`), and `remain-on-exit` is set before it runs (`pty.go:459`). A
same-uid child can `tmux respawn-pane -k -t <pane> -- sh -c 'exit 97'`, drive
`#{pane_dead_status}==97`, and forge the typed class though the credential sentinel
never fired. Both a C2 forge (A5) and an A3 over-fire. Same shared-uid mutable-tmux
oracle as v7 `BC1-W1-ORACLE`.

**The fix — trust a forge-resistant carrier; honestly scope the tmux carrier:**
1. Make the **trusted** floor signal forge-resistant: the **daemon-observed
   stale-epoch rejection** (FIX-1) is forge-resistant; on the DIRECT (non-tmux) path
   the wrapper's own `agent_exited.exit_code` (`helper.go:433` → `supervision.go:425`,
   from the wrapper's `Cmd.Wait`) is forge-resistant. Treat THESE as the trusted
   carriers.
2. **Do NOT claim the tmux `#{pane_dead_status}` carrier is forge-resistant.** Either
   corroborate it against a forge-resistant signal before recording the typed class,
   OR honestly scope its forge-resistance as **RFC-0168-bounded** — the SAME per-lane-uid
   boundary that makes Slice B unsolvable — and state that under the shared uid the
   tmux carrier is best-effort legibility.
3. Add the tmux-path negative
   `TestProviderCannotRespawnPaneToForgeUnrecoverableAcrossRotation` (assert recovery
   does NOT record the typed class on a bare same-uid `respawn-pane … exit 97` with no
   forge-resistant corroboration). **Keep**
   `TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker` (direct path),
   treating it as insufficient alone.

## OBSERVABILITY-ONLY clarification (reduces the forge stakes — state it explicitly)

Slice A is **pure observability / legible-failure routing**. The typed floor is a
**classification refinement** of `agent_exited_unsealed`; it grants **NO new auto-seal
authority** — the lane still requires an operator requeue (or Slice B, later) to
actually seal its complete-on-disk deliverable. State this in the spec, and use it to
frame FIX-2's residual: a forged typed class produces an outcome **no more privileged
than a forged `agent_exited_unsealed`** (which a same-uid child can already cause by
killing its own lane), so the tmux carrier's residual forgeability is **not a new
privilege escalation** — it is a best-effort diagnostic whose worst case is an
honest-RFC-0168-bounded misclassification, never an unwanted seal. (If the typed
floor's recovery routing would, on its own, auto-finalize-from-durable-artifact in a
way `agent_exited_unsealed` would not, that is itself a defect — keep the routing
no-more-privileged than the existing class.)

## HARD CONSTRAINTS (unchanged from v1 — a violation is `reject`/`needs_revision`)

1. **No token widening.** No path widens who can read the admin runtime
   `client-token`; no minted credential carries any of `{admin, apply, recovery,
   surgical_recovery}`. Any FIX-1 attribution / FIX-2 carrier MUST preserve this.
2. **No new credential / no Slice B.** No `CapabilityReseal`, connect-out channel,
   kernel-token capture, reseal-token file, reseal-98, or owner bundle 0021.
3. **Daemon-side durable / process state + the daemon's own observation only.** No
   dependency on an authenticated inbound frame from the lane. (The daemon observing
   its OWN `stale_daemon_identity` rejection is daemon-side, not an inbound frame.)
4. **No over-fire; no raw-error leak; no silent unsealed exit on the rotation path.**
   The floor MUST fire on the real #512 rotation lock-out (FIX-1) and MUST NOT fire on
   an ordinary unsealed exit, a healthy lane, or a forged same-uid tmux respawn
   without forge-resistant corroboration (FIX-2).
5. **Default-off / additive.** Existing recovery + supervise + agentloop tests pass
   unchanged; new exit code / class / event / probe field / daemon record is additive.
6. **Product-boundary-safe** (AGENTS.md): no durable transcript / external persistence.

## Lenses for the two falsifiers

- **falsifier_1 — DECOUPLING / REACHABILITY lens.** Verify FIX-1 actually FIRES on the
  real #512 path (a session-bound lane presenting a stale boot epoch). Attack the
  daemon-side attribution: does `validateBootEpoch` (pre-auth, `http.go:159-169`)
  cleanly attribute the rejection to a session without widening, and does an
  unattributable rejection avoid over-firing? Does any step still secretly need a
  Slice-B artifact or an inbound authenticated frame? Is the codex wedge routed? If
  FIX-1 still under-fires on the central path, that is a standing falsification.
- **falsifier_2 — SECURITY / FORGE / REGRESSION lens.** Is the TRUSTED floor carrier
  forge-resistant (the daemon-observed rejection / the direct-path
  `agent_exited.exit_code`), and is the tmux `#{pane_dead_status}` carrier either
  corroborated or honestly RFC-0168-scoped (not claimed forge-resistant)? Does any
  path widen the admin token or mint an elevated credential (→ `reject`)? Does the
  typed floor over-fire (a forged same-uid respawn, a healthy lane, an ordinary
  unsealed exit)? Does the observability-only framing hold (no new auto-seal
  authority)? Are existing recovery/supervise/agentloop tests regressed, or is the
  v1-credited skeleton (§1/§2/§3.2-3.6/§4) reopened?

## Clearing condition

The adjudicator clears (`accept` / `accept_with_findings`) **only if** all of:

1. **FIX-1 resolved:** the typed floor FIRES on the real #512 rotation path (a
   session-bound lane on a stale epoch / dead endpoint), via a forge-resistant
   daemon-side observation (or the honestly-scoped lane-side fallback), with the
   pre-auth attribution specified and not over-firing; the codex wedge is routed; the
   named tests + the no-over-fire negatives are present.
2. **FIX-2 resolved:** the TRUSTED carrier is forge-resistant; the tmux carrier is
   corroborated or honestly RFC-0168-scoped (not claimed forge-resistant); the
   tmux-path negative test is present; the observability-only / no-new-authority
   framing is stated.
3. The v1-credited skeleton (§1, §2, §3.2–3.6, §4, no-widening) is carried forward
   UNREGRESSED.
4. **No HARD CONSTRAINT violated** and **no new material challenge stands unrebutted.**

**Verdict guidance.** `reject` only if a path widens admin-token exposure / mints a
credential carrying any of `{admin, apply, recovery, surgical_recovery}` / smuggles in
Slice B. Otherwise `needs_revision` if FIX-1 still under-fires on the central path, if
FIX-2's trusted carrier is not forge-resistant or the tmux carrier is still claimed
forge-resistant, if the floor over-fires, if a credited item is regressed, or if any
new material challenge lands. This run allows **one** revision cycle (the falsifiers
re-attack); a second `needs_revision` ends the gate uncleared.

---
<sub>Operator scaffold for the RFC 0143 **Slice A** falsification-gate design run (v2 /
REVISION of `rfc-0143-slice-a-design`; resolves SA-ROTATION-UNDERFIRE — make a
DAEMON-OBSERVED `stale_daemon_identity` rejection the forge-resistant primary producer
of the typed floor so it FIRES on the real #512 rotation path, with pre-auth session
attribution specified — and SA-C2-TMUX-FORGE — trust the forge-resistant
daemon-observed / direct-path `agent_exited.exit_code` carrier and honestly scope the
tmux `#{pane_dead_status}` carrier RFC-0168-bounded — while carrying the v1-credited
skeleton (§1 reserved code+sentinel / §2 Spot-1 narrowing / §3.2-3.4 exact-code
classification / §3.5 launch dissolution / §3.6 #292 relationship / §4 direct-path C2 /
no-widening) forward unregressed, and clarifying Slice A is observability-only with no
new auto-seal authority). Slice B blocked on RFC 0168 (#585), OUT OF SCOPE. Lanes:
author=claude (holder/adjudicate/commit), reviewer=codex --yolo (falsifiers).</sub>
