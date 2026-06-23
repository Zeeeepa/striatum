# HOLDER — RFC 0143 falsifiable implementation spec (design-v5 REVISION)

author: holder-author-001

> This is the **fifth** falsification pass on RFC 0143 (*lane credential survival
> across a daemon boot-epoch rotation*) and a **proper revision**. v1
> (`rfc-0143-design`) returned `needs_revision` with seven findings F1–F7. v2
> resolved **F2/F4** and distilled the residue into five binding constraints BC1–BC5.
> v3 resolved **BC2, BC3, BC4** and carried the v2-credited set forward unregressed.
> v4 resolved **BC5, two of BC1's three sub-grounds (C2 + the daemon-observed
> positive-intent source with the backend-gate bypass), and carried the v3-credited
> set forward unregressed**, but returned `needs_revision` on a single sharply-named
> ground: **BC1-CHANNEL — the W1/W2/W3 channel walls are designed for a DIRECT
> `exec.Cmd.ExtraFiles` child exec, but the production supervised lane is TMUX-BACKED,
> and the control-fd delivery through the real launch path is unspecified** (and every
> obvious bridge reopens the same-uid surface).
>
> This v5 spec starts from the **v4** `HOLDER.md` (required context) and the **v4**
> collaboration ledger (the BC1-CHANNEL finding + the exact "next revision must…"
> list). It **resolves BC1-CHANNEL in one place** by replacing the v4
> inherited-fd-through-`ExtraFiles` channel with a **CONNECT-OUT topology anchored on
> the production `tmux respawn-pane` / `sudo … env -i` / env-file launch path**, and
> **carries the entire v4-credited resolved set forward UNREGRESSED**. It does **not**
> relitigate the ratified OQ1 trust-model shape, the F2 non-bearer decision, or the
> W1/W2/W3 wall *shapes* (all pinned in `SEED.md`). The wall shapes are correct; v5
> fixes only their **installation** on the real launch path. Every source citation
> below was re-verified against the current worktree while authoring this revision;
> **drift is flagged inline in §Source re-verification.**

## Root reframe (held, unchanged)

**A boot-epoch rotation must never force a lane to choose between reading the
daemon's full-authority bootstrap admin `client-token` and exiting silently
unsealed.** A `striatum-lane` lane authenticates as *its own* narrow,
session-scoped credential and **never** as the shared operator admin override. v5
either lets the lane's in-flight work be sealed over a **daemon-projected,
session-tied authority that no lane bearer carries**, or makes the failure **loud,
typed, and routed** — never silent, never via the admin token.

## What v5 changes vs v4 (one place)

v4 was credited on BC5, on C2, on the daemon-observed positive intent + the
`ensureWorkSessionBackend` bypass, and on the entire carry-forward set. It fell on
exactly one ground: the W1/W2/W3 walls were specified for a **direct
`exec.Cmd.ExtraFiles` child**, but production lanes run inside a **tmux pane** spawned
by the tmux *server* (not a child of the launching client), under `sudo … env -i`
and an env-file shim — so there was no buildable way to get fd 3 to the pane wrapper
before `PR_SET_DUMPABLE(0)` without reopening a same-uid surface.

**v5 deletes the inherited-fd channel entirely and adopts a CONNECT-OUT channel
(SEED design hint).** The decisive observation that makes this land cleanly on the
real path: **the daemon already knows the pane wrapper's authoritative kernel
identity at launch.** `CaptureTmuxIdentity` reads `#{pane_pid}` and
`#{pane_start_time}` from the tmux server (`tmux_liveness.go:181-209`); `LaunchResult.PID`
**is** the pane wrapper pid (`pty.go:44-53`, `:527-533`), and the daemon persists
`pane_pid`/`pane_start_time`/`pane_start_token` from the `agent_started` event
(`helper.go:176-185` → `supervision.go:418-423`, `curatedTmuxMetadata`). So the
daemon can authenticate a connecting peer by **`SO_PEERCRED` pid + `/proc/<pid>/stat`
start-time matched against the already-captured pane identity** — no fd crosses
tmux, no secret crosses the env, and the same-uid surface is closed **structurally**.

This is the only material change. Everything else in the v4 spec is reproduced below
and carried forward; the falsifiers should re-attack the **connect-out installation**
(and confirm no carried item regressed).

## Source re-verification (every BC1-CHANNEL site CONFIRMED against current main; drift FLAGGED)

| Claim | Site | Status |
| --- | --- | --- |
| `HelperLaunchSpec` has **no** control-fd field | `helper_protocol.go:27-39` | **CONFIRMED** (fields: SchemaVersion, SupervisorID, ScratchDir, Command, Env, WorkingDir, RunAsUser, PacketInputPath, PTYLogPath, RequireTmux, RebridgeTmux) |
| `LaunchSpec` has **no** `ExtraFiles`/control-fd field | `pty.go:30-42` | **CONFIRMED** — ⚠️ *minor drift:* SEED cited `:30-41`; the struct spans `:30-42` (the `EnvFilePath` field shifts the close brace by one) |
| run-as path is `sudo -n -u <RunAsUser> -- env -i` | `commandInvocationWithEnvFile`, `pty.go:98-113` | **CONFIRMED** — ⚠️ *minor drift:* SEED cited `:98-112`; the function body is `:98-113` |
| env-file shim `set -a; . "$1"; rm -f -- "$1"; shift; exec "$@"` | `launchEnvFileExec`, `pty.go:24`; wrapped as `/bin/sh -c <shim> striatum-env <envFilePath> <command…>` in `envFileWrappedCommand`, `pty.go:282-289` | **CONFIRMED** verbatim |
| `launchPTY` uses `tmux respawn-pane` | `pty.go:479` | **CONFIRMED** (`respawnArgs := []string{"respawn-pane", "-k", "-t", sessionName+":0.0", …}`) |
| `RunHelper` forwards only command/env/wd/run-as/tmux into `LaunchSpec` | `helper.go:149-156` | **CONFIRMED** (also a rebridge attach path, `helper.go:158-159`) |
| **No** `socketpair`/`SCM_CREDENTIALS`/`SO_PASSCRED`/`SO_PEERCRED`/`PR_SET_DUMPABLE`/`ExtraFiles` primitive in `go/pkg/supervisor` or `go/pkg/agentloop` | repo-wide grep | **CONFIRMED ABSENT** (independently re-confirmed; all are new code) |
| **No** `go/pkg/agentloop/exitcodes.go` | dir listing | **CONFIRMED ABSENT** (97/98 constants are new) |
| `LatestOwnerBundleVersion` currently 20 | `owner.go:23` | **CONFIRMED 20** — note `RequiredOwnerBundleVersion = LatestOwnerBundleVersion` (`owner.go:35`), so the 20→21 bump moves both |
| `ResolveTokenMaterial` step order (env → file → runtime client-token → repo token) | `token.go:18-53`; `ReadTokenFile` rejects non-owner-only `token.go:75-92` | **CONFIRMED** |
| `agentExitPayload` ← `processExitCode` ← `result.Cmd.Wait()` | `helper.go:197`, `:244`, `:427-439`, `:499-507` | **CONFIRMED** — ⚠️ **MATERIAL DRIFT, see below** |
| `superviseReportEventTypes` admits no content/output event; `agent_exited` branch reads `event.Payload["exit_code"]`, sets `state="stopped"` | `supervision.go:19-28`, `:298-306` | **CONFIRMED** |
| `gitChangedPathSnapshots` / `collectInScopeAuthoredPaths` authored-path attribution; nil baseline for isolated worktrees | `write_scope_guard.go:225`; `artifact_source_publish.go:69`,`:88`,`:259-263`; `claim.go:622`; `write_scope_guard.go:81-83` | **CONFIRMED** |

### ⚠️ MATERIAL DRIFT found in the v4 BC1-(b) exit-code claim — and fixed in v5

The v4 spec asserted *"the helper already captures the wrapper's OS process exit
status into `agentExitPayload` → `processExitCode` (`helper.go:427-439`)."* **That is
true only for the NON-tmux/direct path.** On the production **tmux-backed** path
`result.Cmd` is the **`tmux attach-session` client** (`attachTmuxPTY`, `pty.go:517-533`:
`attachCmd := commandContext(ctx, spec, "tmux", "attach-session", …)`, `result.Cmd =
attachCmd`, `result.PID = identity.PanePID`, `result.AttachPID = attachCmd.Process.Pid`).
So `result.Cmd.Wait()` (`helper.go:197`) resolves the **attach client's** exit, not the
pane wrapper's: a tmux-backed exit either takes the `attach_client_exited` branch
(`helper.go:239`, pane still live) or emits `agent_exited` carrying the **attach
client's** status (`helper.go:244`). The pane wrapper's real status is observed only
via tmux liveness — and today the probe queries `#{pane_dead}` (a boolean) but **NOT**
`#{pane_dead_status}` (the pane's exit code) (`tmux_liveness.go:228`). So a reserved
exit 97/98 emitted by the pane wrapper **does not reach the daemon through
`result.Cmd.Wait()`** on the production path.

This is the same class of tmux-indirection defect that sank v4's channel, applied to
the exit-code floor. **v5 fixes it as part of BC1-CHANNEL:** the exit-97/98 floor is
no longer the *primary* signal — it is a **backstop**, and on the tmux path it is
observed by adding `#{pane_dead_status}` to the liveness/exit capture, never from
`result.Cmd.Wait()`. The **primary** floor/prompt signal is the authenticated
connect-out frame (below), which is structurally same-uid-safe (§BC1-CHANNEL). See A7'.

## Ratified design shape (pinned — built on, not relitigated)

- **OQ1 (ratified):** Slice A = Option 4 (mandatory, zero-trust-change, lands first)
  + Slice B (ratification-gated) = Option 2's narrow `CapabilityReseal` over a
  daemon-owned session-tied path + minimal Option 3 per-session endpoint+epoch
  republish. No lane-readable reseal bearer file under any option.
- **F2 (decided):** non-bearer, daemon-owned, session-tied channel; **no readable
  reseal token file at all** (every lane shares the `striatum-lane` uid, so any
  `0600` file is a same-uid replay surface). Not reopened. **v5 extends the same-uid
  threat model to the channel's INSTALLATION on the real launch path and closes it
  structurally (BC1-CHANNEL).**
- **W1/W2/W3 wall SHAPES (ratified):** W1 per-message kernel-stamped peer credentials
  binding every control frame to the launched wrapper pid+start-time; W2
  `PR_SET_DUMPABLE(0)`; W3 the control nonce out of the env entirely. The shapes are
  correct and **not relitigated**; v5 changes only how they are installed.
- **Slice B requires maintainer ratification** before any build slice touches
  credential code. Adjudicator clearance gates the spec's *soundness*, not the
  maintainer's product call. Slice A is zero-trust-change and may land first **once
  BC1-CHANNEL anchors a real, non-PTY channel whose same-uid authentication is
  installed through the production tmux/sudo/env-file path** — which v5 does.

## Architectural facts re-anchored (AF1–AF4 — carried forward unregressed)

- **AF1 — reachability, not reminting.** `mintSessionBoundToken`
  (`go/pkg/mutations/session_token.go`) inserts the client row + per-capability grants
  into daemon-owned PostgreSQL bound to `session_id`, 24h TTL. **PostgreSQL survives a
  `striatumd` restart** (D094 / RFC 0043). After a boot-epoch rotation the token is
  still *valid* — only *unreachable* (it lives as the `STRIATUM_MCP_TOKEN` env literal,
  step 1; the post-rotation re-readers skip step 1). The fix is routing, not
  re-minting. *Falsifier:* `TestTokenValidAcrossRestart`.
- **AF2 — post-rotation re-readers fall to step 3.** `ResolveTokenMaterial`
  (`token.go:18-53`) reaches the runtime `client-token` branch at `:31-42` whenever
  steps 1/2 are absent; the #323 fresh re-read (`ResolveTokenMaterialFresh`,
  `go/pkg/agentloop/endpoint.go`) likewise skips the env literal and falls to step 3 —
  the bug.
- **AF3 — step 3 is the full-authority admin token in a `0700` dir.**
  `admin.BootstrapRuntimeTokenIfNeeded` (`go/pkg/admin/bootstrap.go:18-27`) grants the
  runtime `client-token` the full `bootstrapCapabilities`
  `{admin,read,write,claim,review,apply,recovery,surgical_recovery}`, `0600` in a
  `0700` dir; `ReadTokenFile` (`token.go:75-92`) rejects any token file not owner-only.
- **AF4 — epoch/token decoupling.** Endpoint + boot epoch rotate together; #316
  deliberately retires a surviving lane's connection by rejecting a stale epoch. The
  token does **not** rotate on a normal restart — only the endpoint does. Preserved.

## Carried forward from v4, unregressed (do NOT reopen)

| Item | Status | Anchor / test kept |
| --- | --- | --- |
| **BC2** — reseal artifact identity from the job's `expected_artifacts` (daemon state); refuses unexpected paths; front-matter failure → floor | resolved (carried) | `verifyRequiredArtifacts`/`ensurePerJobPublishedArtifactsDurable` (`mutations.go:828-876`); `TestCodexResealUsesReceiverNotProviderStdout` (negative + positive) |
| **BC3** — `CapabilityReseal` a daemon-internal marker projected by `resealInFlightJob`; public route-alternate test-only | resolved (carried) | `TestResealCapabilityIsDaemonInternalNotBearer` / `TestResealTokenCanReachOnlyResealRoutesWithoutWrite` |
| **BC4** — concrete monotonic `jobs.recovery_generation` (owner bundle 0021), increment points, stamped value compared under the lock | resolved (carried) | `TestResealPredicateUsesStampedRecoveryGeneration` / `TestResealTokenRefusedAfterLeaseExpiryAndRecoveryRequeue` |
| **BC5** — `leases.reseal_grace_extended_at` in owner bundle 0021 (`leases` owner-held); corrected skip/replace/replay lock-order gate map | resolved (carried) | `TestResealBeyondGraceRoutesTypedNotLeaseError` / `TestResealGraceCannotReviveRequeuedLease` / `TestRecoveryRequeueWinsOverExpiredLeaseReseal` / `GD-1b` |
| **C2** — wrapper never propagates a provider child 97/98 into the reserved codes | resolved (carried) | `TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker` |
| **Daemon-observed positive intent + recovery-sweep backstop** | resolved (carried) | positive `TestCodexResealUsesReceiverNotProviderStdout` |
| **`ensureWorkSessionBackend` bypass** | resolved (carried) | `TestResealExit98BypassesBackendGateOrRoutesTyped` |
| **W1/W2/W3 wall SHAPES** | shape resolved; **installation = this revision** | §BC1-CHANNEL + `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` |
| **F2** — no lane-readable reseal bearer | resolved | `TestBorrowedResealBearerCannotSealVictimSession` |
| **F4** — route-alternate records `reseal` not `write` on only the 3 routes | resolved | `TestResealTokenCanReachOnlyResealRoutesWithoutWrite`; command-authority-matrix reseal column |
| **F7 file-mirror half** — daemon-owned lane-read-only `0644` endpoint/epoch, `O_NOFOLLOW`, atomic rename, reject MISSING epoch header (closes #316) | resolved | `TestResealEpochMirrorRejectsTamperOrMissingEpoch` |
| **AF1 / AF4 / no-admin-token-widening invariant** | kept / held + strengthened | `TestTokenValidAcrossRestart` / `TestResolveRefusesRuntimeClientTokenForLane` |
| **Per-claim falsifiable-assertion discipline** | extended to the connect-out real-path installation | A1–A18 + A3'/A4'/A7' below |

The carried-forward sections (BC2, BC3, BC4, BC5, the daemon-observed trigger) are
reproduced below verbatim in substance from v4; only **§BC1-CHANNEL** (the channel
installation + the exit-code-on-tmux backstop) is new.

---

# Security cluster (BC1-CHANNEL + BC2 + BC3)

## BC1-CHANNEL — CONNECT-OUT control channel installed on the production tmux launch path (closes F1 / F6 / the channel half of F2 / F7-channel)

### Why the v4 inherited-fd channel cannot be installed on the real path (the three problems, restated and DISSOLVED)

The v4 channel was a `socketpair` whose wrapper end was handed down via
`exec.Cmd.ExtraFiles`. On the production path this is unbuildable:

- **(a)** `launchPTY` runs the lane via `tmux respawn-pane` (`pty.go:479`); the pane
  process is spawned by the **long-lived tmux server**, not as a child of the
  launching client (`commandContext`'s `exec.Cmd`). An fd in `ExtraFiles` on that
  client is **never inherited by the pane wrapper.**
- **(b)** Threading the fd through the env-file shim (`/bin/sh -c '… exec "$@"'`,
  `pty.go:24`/`:282-289`) would make it **live before** the agentloop wrapper runs
  `PR_SET_DUMPABLE(0)` — a same-uid sibling could `openat(/proc/<wrapper-pid>/fd/3)`
  in that window. The required W2 ordering breaks.
- **(c)** Any env-var / filesystem-socket-name / lane-readable handoff that *names*
  the fd or the nonce reopens the exact same-uid surface BC1 exists to close (the v1
  `0600`-file mistake).

**Connect-out dissolves all three by inverting the direction and moving
authentication off secrecy onto kernel peer-credentials:**

> **There is no inherited fd at all.** The pane wrapper, *after* it has already made
> itself non-dumpable, **dials out** to a daemon-held listener. The daemon
> authenticates the connecting peer by `SO_PEERCRED` (uid+pid) plus a
> `/proc/<pid>/stat` start-time match against the **pane identity it already captured
> at launch**. The listener address is **non-secret** (a sibling that knows it and
> connects is rejected on pid/start-time); the nonce is delivered **daemon→wrapper,
> after authentication**, so it never appears in env or on disk.

### The process topology (re-anchored to the real path)

Three process layers, two privilege levels — unchanged from v4, but with the tmux
indirection now explicit:

- **helper** (`RunHelper`, `helper.go:128`) — runs at the **daemon uid** (e.g.
  `halbritt`). It calls `Launch` → `launchPTY` → `tmux new-session` + `respawn-pane`
  under `sudo -n -u <RunAsUser> -- env -i` + the env-file shim (`pty.go:435-491`),
  then `CaptureTmuxIdentity` (`pty.go:493`) to learn `PanePID`+`PaneStartToken`, then
  `attachTmuxPTY` for byte delivery. It "only moves process bytes and reports control
  events" (`helper.go:120-127`).
- **wrapper** (the agentloop process, `loop.go:220` `runWithIO`) — runs as
  **`striatum-lane`** **inside the tmux pane** (it is the command tmux respawns,
  wrapped by the env-file shim). It execs the provider as a **child**
  (`exec.CommandContext`, `loop.go:266`) and `cmd.Wait()`s it (`loop.go:365`). Per
  `loop.go:220-368` the wrapper does **not** know whether the deliverable is complete.
- **provider** (claude/codex CLI) — runs as **`striatum-lane`**, the wrapper's child.

The same-uid threat is a sibling `striatum-lane` process that is **neither the
provider child nor the launched pane wrapper.**

### BC1-CHANNEL-(a) — the daemon-held connect-out listener (W1, on the real path)

1. **Listener creation (daemon uid, at supervise start).** The helper creates a
   `socket(AF_UNIX, SOCK_SEQPACKET, 0)` listener with `SO_PASSCRED` set, bound to a
   **per-launch abstract address** `@striatum-supervisor-ctl/<supervisor_id>/<random>`
   (abstract → no filesystem node, auto-cleaned on close; reachable in the host net
   namespace, which is fine — see W1). The helper holds the listener and runs an
   `acceptControlChannel` reader goroutine **alongside** `pumpPTYProgress` /
   `forwardPacketStream` (`helper.go:200-208`). `SOCK_SEQPACKET` preserves message
   boundaries (one frame per datagram).
2. **Address delivery is NON-SECRET, over the existing env plumbing.** The address is
   advertised to the pane wrapper as `STRIATUM_SUPERVISOR_CONTROL_ADDR` via the
   existing env path that already reaches the pane (`tmuxEnvArgs` `-e KEY=VAL`,
   `pty.go:436`/`:480`, and the env-file). It is **not** a `sensitiveEnvKey`
   (`pty.go:140-155`), so it rides the normal channel. **This is not problem (c):**
   the address is non-secret by construction — W1 authenticates the *peer*, not
   knowledge of the address. (A sibling that reads it and connects is rejected; and
   with W2 a sibling cannot read the env at all.)
3. **W1 — peer-credential authentication against the daemon-captured pane identity.**
   When a peer connects, the helper reads `SO_PEERCRED` on the **accepted** connection
   — which returns the **connecting peer's** real `{pid, uid, gid}` at connect time
   (this is exactly the case `SO_PEERCRED` is designed for; the v4 `socketpair`
   SO_PEERCRED problem — it returning the helper's own pid — **does not arise** for an
   accepted connect-out). The helper accepts the connection **iff**:
   - `peer.uid == RunAsUser uid` (the lane user), AND
   - `peer.pid == result.PID` — the launched **pane** pid (`identity.PanePID`,
     `pty.go:529`), AND
   - `ProcessStartToken(peer.pid)` (`/proc/<pid>/stat` field 22,
     `process_identity_linux.go:12-17`) `== identity.PaneStartToken` — the start-time
     token the daemon **already captured at launch** (`tmux_liveness.go:194-208`),
     defeating pid reuse. Reuse the existing `PIDLiveWithStartToken`
     (`tmux_liveness.go:392`).

   The helper accepts the **first** connection whose peer-cred matches and binds the
   channel to it; every later/other connection is **refused**. A same-uid sibling that
   connects has a **different pid** → refused. **This is the structural no-replay
   property, and it now holds on the REAL channel** because the daemon's notion of
   "the wrapper" is the tmux-server-reported pane pid+start-time, captured before any
   frame is read.

4. **W3 — nonce delivered daemon→wrapper, AFTER auth.** On a matched connection the
   helper sends a single-use `control_nonce` (per launch, per generation) **down** the
   authenticated connection. The wrapper echoes it on every subsequent control frame;
   the helper rejects any frame whose nonce ≠ the issued nonce, and the BC4 generation
   guard refuses a nonce from a prior generation. The nonce **never** appears in env or
   on disk (W3 satisfied structurally — strengthened vs v4, where the nonce rode the
   socketpair; here it is daemon→wrapper post-auth, so a sibling never observes it).
   **W1 (peer-cred), not the nonce, is the primary and sufficient authentication;** the
   nonce is generation-binding + defense-in-depth.

### BC1-CHANNEL-(b) — W2 ordering, now TRIVIALLY satisfied (no inherited fd to protect)

The pane wrapper calls `prctl(PR_SET_DUMPABLE, 0)` (new `go/pkg/agentloop/dumpable_linux.go`,
with a non-Linux stub mirroring `process_identity_other.go`) as the **first instruction**
of the agentloop entrypoint — before it reads `STRIATUM_SUPERVISOR_CONTROL_ADDR`, before
it dials, before any nonce exists in its address space. Two independent reasons the W2
window is closed on the real path:

- **There is no inherited control fd at all.** The env-file shim
  (`/bin/sh -c '… exec "$@"'`) execs the agentloop binary; nothing is handed down to
  steal in the pre-dumpable window. Problem (b) is structurally void.
- **`dumpable=0` is reinforced by the `sudo` setuid launch.** A credential-changing
  `execve` (the `sudo -u striatum-lane` transition, `pty.go:104-112`) makes the kernel
  reset `dumpable` to `/proc/sys/fs/suid_dumpable` (default `0 = SUID_DUMP_DISABLE`) for
  the new process, so `/proc/<wrapper-pid>/{fd,environ,mem}` are already root-owned on a
  default host. The explicit `prctl` makes this host-independent (no reliance on the
  `suid_dumpable` sysctl).

So even the **non-secret** address in the wrapper's env is unreadable by a sibling
(`/proc/<wrapper-pid>/environ` is root-owned), and there is no fd to duplicate. The
connection is dialed **after** `dumpable=0`, by the wrapper itself.

### BC1-CHANNEL — named plumbing sites that reach the PANE wrapper (NOT the tmux client)

| Site | Today | v5 change |
| --- | --- | --- |
| `HelperLaunchSpec` (`helper_protocol.go:27-39`) | no control field | **add `ControlSocketAddr string`** (non-secret) — daemon→helper |
| `LaunchSpec` (`pty.go:30-42`) | no `ExtraFiles`/control field | **add `ControlSocketAddr string`**; `RunHelper` injects it into `spec.Env` as `STRIATUM_SUPERVISOR_CONTROL_ADDR`, so it reaches the pane via the existing `tmuxEnvArgs` (`pty.go:436`/`:480`) + env-file path — **no fd, no new tmux plumbing** |
| `RunHelper` (`helper.go:128`, `:149-156`) | forwards no fd | **create the `SO_PASSCRED` listener; capture `result.PID`+`identity.PaneStartToken`; run `acceptControlChannel` that authenticates `SO_PEERCRED` pid+start-time against them, issues the nonce, reads frames → emits `reseal_requested`/`unrecoverable_across_rotation` `HelperControlEvent`s.** No change to the tmux client. |
| agentloop entrypoint (`loop.go` `Run`/`runWithIO`) | nothing | **new `go/pkg/agentloop/dumpable_linux.go` (`PR_SET_DUMPABLE(0)` first); new `go/pkg/agentloop/control_channel.go` (read addr, dial, nonce handshake, send frames); new `go/pkg/agentloop/exitcodes.go` (97/98).** |
| tmux exit-status capture (`tmux_liveness.go:228`) | probes `#{pane_dead}` only | **add `#{pane_dead_status}`** so the pane wrapper's reserved exit code is observable as the **backstop** floor (fixes the §Source-re-verification drift). |

**Frame schema (unchanged from v4).** One `SupervisorControlFrame` per datagram:
`{ schema_version: "striatum.supervisor_control.v1", type: "reseal_requested" |
"unrecoverable_across_rotation", supervisor_id, control_nonce }`. It carries **NO**
job_id / artifact path / kind / body — identity is derived from daemon state (BC2).
The PTY (provider stdout/stderr) reaches **only** the volume meter (`pumpPTYProgress`,
D028); `acceptControlChannel` reads frames **only** off the authenticated connection;
`superviseReportEventTypes` (`supervision.go:19-28`) admits **no** content/output event.

**Structural no-replay (the spine, on the REAL channel).** A sibling `striatum-lane`
process that is neither the provider child nor the launched pane wrapper cannot: (W2)
read `/proc/<wrapper-pid>/{environ,fd}` (root-owned; and there is no inherited fd
anyway); (W3) observe the nonce (delivered daemon→wrapper post-auth, never in env);
**and decisively (W1) be accepted by the daemon** — `SO_PEERCRED` stamps the sibling's
real pid, which ≠ the captured pane pid (and `/proc` start-time defeats pid reuse).
Grant the sibling the address *and* (hypothetically) the nonce, W1 still refuses it.
No-replay holds **structurally on the production tmux channel**, not on a direct-exec
harness.

### BC1-CHANNEL — reserved exit codes: PRIMARY signal is the authenticated frame; the exit code is a tmux-observed BACKSTOP

v5 keeps both non-PTY mechanisms but corrects which is load-bearing on the tmux path:

- **Primary (post-rotation prompt / floor):** the pane wrapper sends the typed
  `SupervisorControlFrame` over the **already-authenticated connect-out channel**
  before exiting. `acceptControlChannel` emits the corresponding `HelperControlEvent`;
  the daemon evaluates the daemon-observed reseal condition (§BC1-positive-intent).
  This signal is structurally same-uid-safe (W1).
- **Backstop (the wrapper can't even send a frame):** two reserved agentloop exit-code
  constants (new, `exitcodes.go`): `ExitUnrecoverableAcrossRotation = 97` (the Option-4
  floor) and `ExitResealInFlightRequested = 98` (a latency hint only; its forgeability
  is immaterial — the daemon never seals on the strength of 98). On the tmux path these
  are observed via the **new `#{pane_dead_status}` capture** (NOT `result.Cmd.Wait()`,
  which is the attach client) and routed through the existing `agent_exited` branch
  (`supervision.go:298-306`). On a non-tmux/direct launch they still flow through
  `agentExitPayload`→`processExitCode` (`helper.go:427-439`) as v4 described.
- **Final backstop:** the **recovery sweep** evaluates the same daemon-observed
  condition under the same lock (§BC5), so even if neither a frame nor a pane status is
  observed, a complete-on-disk post-rotation deliverable still gets one daemon-observed
  reseal attempt (or the typed floor).

**C2 — reserved codes reserved BY COMMITMENT (carried forward, unchanged).** Choosing a
high range is **not** an auth boundary. The wrapper is committed to **never propagate a
provider child's status into the reserved codes**: in `runWithIO`, after `cmd.Wait()`
(`loop.go:365`) the wrapper inspects the provider child's exit status and, if it is 97
or 98, **remaps it to a non-control `agent_exited` outcome** (carrying the provider's raw
status in a non-reserved payload field). The reserved 97/98 are emitted **only** by the
wrapper's own typed-error path (`ErrSessionUnrecoverableAcrossRotation` → 97; the
wrapper's own post-rotation reseal-prompt → 98) and the authenticated frame, never
forwarded from the child. *Falsifier:*
`TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker`.

### BC1 — positive intent is DAEMON-OBSERVED, not provider-asserted (carried forward, unchanged)

`resealInFlightJob` fires **only** when ALL of the following hold, evaluated under the
run advisory lock (§BC5):

1. **A boot-epoch rotation occurred during this job's lease** (recorded packet epoch vs
   current `writeBootEpochFile` epoch). Absent a rotation, the normal seal path applies.
2. **The job is still `running`, the lane's lease is bound, the stamped generation
   matches** the live `jobs.recovery_generation` (BC4), **and** the lease is within
   grace (BC5). Any mismatch → typed floor.
3. **Every required `expected_artifact` path (from daemon state, BC2) is present in the
   job's active worktree AND was AUTHORED THIS ATTEMPT** — see §"modified-since-baseline
   build-test" below (this folds in falsifier-reviewer-002's precision item). Absent
   that, route to the floor, never a speculative seal.
4. `resealInFlightJob` then attempts **only daemon-derived artifacts** (BC2) and **maps
   ALL validation/backend/front-matter/durability failures to the typed
   `session_unrecoverable_across_rotation` floor** (Option-4). Never a silent seal,
   never a raw error.

**Two entry points, one condition, one backstop:** the wrapper's authenticated
post-rotation frame (or `#{pane_dead_status}` 98) **and** the recovery sweep, which
evaluates the **same** condition under the **same** lock **before** requeuing.

**Backend-gate routing (carried forward).** The reserved exit / frame stops the
supervisor; the normal `work.complete` core requires a live attached backend via
`ensureWorkSessionBackend` (`lifecycle.go:1181`). `resealInFlightJob` does **not** reuse
`HandleCompleteWork`; it calls the lower-level complete core and **deliberately bypasses
`ensureWorkSessionBackend`** — the reseal exists *precisely because* the live connection
is gone. The exact gate map is in §BC5. *Falsifier:*
`TestResealExit98BypassesBackendGateOrRoutesTyped`.

### Modified-since-baseline build-test (folded in from falsifier-reviewer-002)

The "deliverable observed" condition (3) must **not** treat "present + absent from
`write_scope_baseline.changed_paths`" as sufficient by itself: for per-job isolated
worktrees the baseline is **nil** (`write_scope_guard.go:81-83`), and source-change
publication already attributes authorship via `gitChangedPathSnapshots`
(`write_scope_guard.go:225`) + `collectInScopeAuthoredPaths` (`artifact_source_publish.go:263`,
used at `:88`/`:201`; nil baseline → every in-scope changed path is the attempt's,
`:259-263`). `resealInFlightJob` **reuses that authored-path attribution** to confirm
each required expected path was authored this attempt, so an UNCHANGED pre-existing
expected path is **NOT** resealed. *Falsifier:*
`TestResealRequiresAuthoredExpectedArtifactChange` — seed a clean pre-existing expected
path → assert typed floor; modify it → assert positive reseal (or the positive
`TestCodexResealUsesReceiverNotProviderStdout` case).

### BC1-CHANNEL — the real-path test (the load-bearing addition)

`TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` (name retained from the SEED for
continuity; mechanism is connect-out, not fd-inheritance). It launches through
`RunHelper` with `RequireTmux` + `RunAsUser` set (a host integration/game-day test
gated on `sudo` + `tmux`, build-tagged like `tmux_liveness_integration_test.go`), and
asserts **together**:

1. **W1 accept:** the launched pane wrapper connects out and sends a frame the daemon
   **accepts**, authenticated to the launched pane pid+start-time (`identity.PanePID` /
   `PaneStartToken`), emitting a `reseal_requested`/`unrecoverable_across_rotation`
   `HelperControlEvent`.
2. **Provider isolation:** the provider child cannot drive a control event — it is not
   the wrapper pid (W1 refuses it), and it has **no inherited fd** (there is none).
3. **Sibling refusal (W1) at ANY point in the launch chain:** a non-child/non-wrapper
   same-uid sibling that connects to the SAME `STRIATUM_SUPERVISOR_CONTROL_ADDR` is
   **refused** (wrong pid/start-time), AND cannot recover the nonce
   (`/proc/<wrapper-pid>/environ` is root-owned under W2; the nonce is never in env),
   AND — because there is no inherited fd — has nothing in `/proc/<wrapper-pid>/fd` to
   steal.

The direct-`os/exec` versions `TestControlFrameRequiresExpectedWrapperPeerCredentials` /
`TestSiblingSameUIDCannotOpenControlFDOrReplayNonceViaProc` (carried from v4) are kept
as fast unit coverage of the peer-cred + dumpability logic but are **necessary, not
sufficient** — the real-path test above is the one that fires against the production
tmux/sudo/env-file path.

**Additional kept tests (BC1):** `TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker`
(C2); `TestResealExit98BypassesBackendGateOrRoutesTyped`;
`TestPTYOutputCannotEmitSupervisorControlEvent` / `TestProviderOutputCannotDriveResealOrBlocker`;
`TestAgentLoopReservedExitCodeRoutesUnrecoverableWithoutPTYParsing` (extended to assert
the pane-status backstop reads `#{pane_dead_status}`, not the attach client).

## BC2 — Artifact identity from daemon state, never from output (resolved, carried forward)

`resealInFlightJob` derives the expected-artifact set from **its own state** and refuses
any unexpected path, reusing the existing handler payload contracts verbatim:

- **`artifact.publish`** requires `session_id`/`job_id`/`lease_id`/`kind`/`logical_name`/`path`
  (`artifact.go:52-60`), takes `lockRunForJob` first (`HandlePublishArtifact`, `artifact.go:64-83`).
- **`work.complete`** requires `session_id`/`job_id`/`lease_id` (`lifecycle.go:1124-1130`).
- **`interrogation.answer`** requires `session_id`/`interrogation_id`/`body`
  (`interrogation.go:217-221`).

For a reseal **complete**, the daemon resolves `jobs.expected_artifacts_json`
(attempt-resolved via `resolveExpectedArtifactCycles`) and verifies every required
artifact is durable, reusing `verifyRequiredArtifacts` (`mutations.go:828-876`) and
`ensurePerJobPublishedArtifactsDurable` (`artifact_durability.go`). For a reseal
**publish**, the daemon publishes **only** a `path` that is an open entry in the job's
`expected_artifacts`, reading the body from the job's own worktree (`job_worktrees`,
`0005:350-372`), and **refuses any path not in the expected set**. The signal supplies
neither path nor content. A front-matter/author-line failure (publisher exit code 6)
records the `session_unrecoverable_across_rotation` blocker with the validation error —
the Option-4 floor, never a silent drop. *Test:* `TestCodexResealUsesReceiverNotProviderStdout`
(negative: a frame/stdout claiming a path not in `expected_artifacts` is refused;
positive: a complete-on-disk post-rotation deliverable IS resealed from daemon state +
worktree).

## BC3 — `CapabilityReseal` is a daemon-internal marker, not a public bearer (resolved, carried forward)

- **Projection, not presentation.** `resealInFlightJob` maps `supervisor_id` →
  `session_id` from the supervision row (the same lookup `recordSuperviseReportEvent`
  uses via `findReportSupervisor`, `supervision.go:497-528`), constructs an **internal**
  `rpc.AuthContext{Capability: CapabilityReseal, SessionID, RepositoryID}` **without**
  the public `Authorize` prelude (`rpc/server.go:107-111`), threads it with
  `WithAuthContext`, and calls the lower-level publish/complete routines against the
  job's active worktree. No bearer reaches the lane.
- **Public route-alternate kept test-only.** `MethodEntry.ResealAlternate` set true on
  only `interrogation.answer`/`work.complete`/`artifact.publish`; the prelude
  re-authorises against `CapabilityReseal` on a `capability_missing` for those routes
  and records `AuthContext.Capability == reseal` (never `write`). With **no production
  reseal bearer**, this path is exercised **only by the guardrail tests**.
  `registry_methods.go` is generated (`// Code generated by … routergen … DO NOT EDIT`),
  so `ResealAlternate` lands in `contracts/daemon_methods.json` + the `MethodEntry`
  struct (`rpc/registry.go`) + the regenerated map + a reseal column in
  `docs/reference/command-authority-matrix.md` + the authority guardrail.

*Test:* `TestResealCapabilityIsDaemonInternalNotBearer` — no live caller can present
`CapabilityReseal`; the only path that seals is the internal `resealInFlightJob`
projection keyed to `supervisor_id`↔`session_id`; the route-alternate is reachable only
from the guardrail harness.

---

# Lifecycle cluster (BC4 + BC5)

## BC4 — Concrete monotonic generation column for the split-brain guard (resolved, carried forward)

`jobs` is **owner-held**, so a column-add is owner DDL —
`TestFutureRuntimeMigrationsDoNotCarryOwnerDDL` forbids a runtime migration from
ALTERing it. v5 ships owner bundle **`go/pkg/db/sql/owner/0021_job_recovery_generation.sql`**:
`ALTER TABLE striatumd.jobs ADD COLUMN IF NOT EXISTS recovery_generation integer NOT
NULL DEFAULT 0;`, bumps `LatestOwnerBundleVersion` **20→21** (`owner.go:23` — **confirmed
currently 20**; note `RequiredOwnerBundleVersion = LatestOwnerBundleVersion`, `owner.go:35`,
moves with it) with the ordinal-21 `[[owner_bundle]]` reservation in `RESERVATIONS.toml`
(`go/pkg/db/reservations.go`), modelled exactly on the credited `review_generation`
precedent (`owner/0009_review_generation.sql`).

- **Degrade-safe presence probe.** `db.JobRecoveryGenerationColumnPresent` (mirroring
  `SessionPipeReadColumnPresent` / `ArtifactPlacementColumnPresent` /
  `reviewGenerationEnabled`, `db/artifact_write.go:64-102`). Column absent (daemon ahead
  of bundle 21) → `resealInFlightJob` treats the generation as **unverifiable and routes
  to the typed floor**.
- **Increment points (each in the same UPDATE that retires/rebinds the job's
  authoritative lease, all under `lockRun`):** (1) **claim** — `claimChosenJob`
  (`claim.go:222-228`); (2) **requeue (same attempt)** — `requeueJobSameAttempt`
  (`recovery.go:2097-2109`); (3) **recovery sweep expire/transfer/respawn** — the
  `current_lease_id = NULL` transitions in `HandleRecoveryAuto`/`SweepRun`
  (`recovery.go:619`/`:2546`/`:2854`/`:2935`); (4) **release** — `work.release`.
  Monotonic by construction (only `+1`).
- **Stamped value.** `claimChosenJob` writes the post-increment `recovery_generation`
  into the work-packet `lease` block (`buildPacket`, `claim.go:229-260`) as
  `lease.recovery_generation`. At reseal, `resealInFlightJob` reads the stamped value
  from the bound `work_packets` row and compares it to the **live**
  `jobs.recovery_generation` under the lock — equal → proceed; unequal → typed class.

*Tests:* `TestResealPredicateUsesStampedRecoveryGeneration` /
`TestResealTokenRefusedAfterLeaseExpiryAndRecoveryRequeue`.

## BC5 — Numeric grace + PINNED migration site + CORRECTED lock order (resolved, carried forward)

### BC5-(1) — `leases.reseal_grace_extended_at` PINNED to owner bundle 0021

`striatumd.leases` is created in **runtime migration 0005**
(`0005_repo_local_workflow_state.sql:166`) and is **owner-held**: it is **NOT** in the
migration-0016+ ownership-transfer cohort (`owner/0018_runtime_table_ownership_transfer.sql`
— `leases` absent). So a column-add to `leases` is **owner DDL**. **Pinned:**
`reseal_grace_extended_at timestamptz` (NULL until used) is added in the **same owner
bundle 0021** as `jobs.recovery_generation` — a second statement in
`owner/0021_job_recovery_generation.sql`:
`ALTER TABLE striatumd.leases ADD COLUMN IF NOT EXISTS reseal_grace_extended_at timestamptz;`.
(Like `review_generation`, `striatumd_rw`'s table-level grants extend to the new column;
no new grant.) *Falsifier:* `TestRuntimeMigrationsDoNotCarryOwnerDDLForLeasesGraceColumn`
(folded into `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL`).

### BC5-(2) — numeric grace + one extension + CORRECTED lock order

- **`resealGrace` numeric + source + maximum.** `const resealGraceWindow = 30 *
  time.Second` (new, beside the lease constants in `go/pkg/mutations`), **hard-capped**
  at the packet heartbeat window: `grace = min(resealGraceWindow,
  packet.lease.heartbeat_after_seconds)`. Daemon-side allowance, **not** a
  lane-invokable `work.heartbeat` (`CapabilityReseal` carries no heartbeat verb).
- **One same-lease extension only**, gated by `leases.reseal_grace_extended_at` (NULL
  until used). Allowed only if `now() - expires_at ≤ grace` AND
  `jobs.recovery_generation == stamped` AND `reseal_grace_extended_at IS NULL`. A second
  expiry or any generation change → typed floor.
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

- **Serialization vs `artifact.publish` / `work.complete` / the recovery sweep.** All
  take `lockRunForJob` / `lockRun(run_id)` (the same `pg_advisory_xact_lock(hashtext(run_id))`,
  `mutations.go:663-665`, RFC 0104) **FIRST**. The sweep drains helper events in short
  txns BEFORE `lockRun` (`recovery.go:575-590`) but expires/requeues INSIDE the `lockRun`
  tx (`:610-621`). So: sweep-wins → it bumps the generation; reseal then blocks, acquires
  the lock, observes the changed generation/expired-beyond-grace lease, routes the typed
  class (**never revives a requeued lease**). Reseal-wins → seals within grace
  (`running→completed`); the sweep then sees a completed job and does not requeue.
- **Expired-beyond-grace ALWAYS routes the typed class** — `resealInFlightJob` never
  calls `activeLeaseFor`; the reseal predicate returns
  `ErrSessionUnrecoverableAcrossRotation` → the durable blocker. No raw `lease_error`
  ever reaches a post-rotation reseal.

*Tests:* `TestResealBeyondGraceRoutesTypedNotLeaseError`,
`TestResealGraceCannotReviveRequeuedLease`, `TestRecoveryRequeueWinsOverExpiredLeaseReseal`,
`GD-1b`, `TestResealExit98BypassesBackendGateOrRoutesTyped`, and the 0021-migration guard.

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

`typedFloor(reason)` records the durable `session_unrecoverable_across_rotation` blocker
(Option-4). The sketch is also the **recovery-sweep entry point** (same predicate, same
lock, before requeuing) and the **connect-out-frame entry point** (the authenticated
`reseal_requested` frame triggers the same evaluation). The `allRequiredExpectedArtifactsAuthoredThisAttempt`
predicate uses the `gitChangedPathSnapshots`/`collectInScopeAuthoredPaths` authored-path
attribution (nil baseline for isolated worktrees) — the folded-in build-test item.

## Security invariant (the spine) — held and strengthened

The runtime `client-token` carries the full `bootstrapCapabilities` and is `0600` in a
`0700` dir (AF3). **Any path that lets a lane read that file, or mints a lane-readable
credential carrying any of `{admin, apply, recovery, surgical_recovery}`, is
categorically out of bounds for a FIX** — v5 keeps it structurally impossible:

- The lane never gets OS read of the `0700` dir (AF3); the Slice-A floor removes the only
  code path that would have read the `client-token` (`token.go:31-42` /
  `endpoint.go` return the typed error for a supervised lane).
- The only new authority, `CapabilityReseal`, carries **no elevated verb** and is
  **never materialised into any lane-readable file or bearer** (BC3 + F2).
- The reseal is **projected by the daemon only** on the supervisor-proven path.
- The control channel is now a **connect-out authenticated by kernel `SO_PEERCRED`
  pid+start-time** (W1) against the daemon-captured pane identity, with the wrapper
  non-dumpable (W2) and the nonce delivered post-auth (W3) — **no bearer, no inherited
  fd, no secret in env**, and a sibling that connects is refused **structurally** on the
  real tmux path. The exit-97 floor is a tmux-`#{pane_dead_status}`-observed backstop, not
  a forgeable primary.
- The epoch republish moves **endpoint + epoch only** over the daemon-owned,
  integrity-protected path (F7 file-mirror, kept); never the admin token.

*Falsifier:* `TestResolveRefusesRuntimeClientTokenForLane` —
`ResolveTokenMaterial`/`ResolveTokenMaterialFresh` return
`ErrSessionUnrecoverableAcrossRotation` for a supervised lane, never the runtime
`client-token`.

## Falsifiable assertions (each with the named test / game-day that refutes it)

- **A1 — No-widening.** `CapabilityReseal` carries only the three reseal verbs and is
  daemon-internal. *Refuted if* `TestResealTokenCanReachOnlyResealRoutesWithoutWrite` /
  `TestResealCapabilityIsDaemonInternalNotBearer` shows it reaching any of
  `admin`/`apply`/`recovery`/`surgical_recovery`/`work.claim_next`/any non-reseal route,
  resolving to `write`, or presentable as a bearer.
- **A2 — No admin-token fall-through.** *Refuted if*
  `TestResolveRefusesRuntimeClientTokenForLane` returns the runtime `client-token` for a
  supervised lane instead of the typed error.
- **A3 — No-replay, STRUCTURAL (W1, direct harness).** Every accepted control frame is
  bound to the launched wrapper pid+start-time. *Refuted if*
  `TestControlFrameRequiresExpectedWrapperPeerCredentials` accepts a frame from any pid
  other than the launched wrapper's (or a mismatched start-time), or
  `TestBorrowedResealBearerCannotSealVictimSession` finds an on-disk reseal bearer or a
  sibling/foreign-session/provider-child sealing session A's job.
- **A3' — No-replay, STRUCTURAL, on the PRODUCTION tmux channel (the v5 closure).**
  *Refuted if* `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` (launched through
  `RunHelper` with `RequireTmux`/`RunAsUser`) shows: a same-uid sibling connection to
  `STRIATUM_SUPERVISOR_CONTROL_ADDR` **accepted** (wrong pid/start-time should refuse);
  OR the provider child driving a control event; OR the launched pane wrapper's
  authenticated frame **not** accepted; OR an inherited control fd present in the
  wrapper/provider; OR the channel installable only via a same-uid-readable secret bridge.
- **A4 — `/proc` surface closed (W2/W3).** *Refuted if*
  `TestSiblingSameUIDCannotOpenControlFDOrReplayNonceViaProc` opens
  `/proc/<wrapper-pid>/fd/*` or recovers the nonce from `/proc/<wrapper-pid>/environ` as
  a same-uid non-wrapper process, or the nonce is found in the wrapper env.
- **A4' — W2 ordering on the real path.** *Refuted if* the real-path test shows the
  wrapper readable (`dumpable != 0`) at any point before it connects out, or the nonce
  observable in env at any point in the launch chain (it must be delivered daemon→wrapper
  post-auth only).
- **A5 — Control path never parses output.** *Refuted if*
  `TestPTYOutputCannotEmitSupervisorControlEvent` / `TestProviderOutputCannotDriveResealOrBlocker`
  shows PTY/stdout bytes driving a reseal or blocker, or the helper inspecting child
  output for a control decision.
- **A6 — Reserved exit codes reserved by commitment (C2).** *Refuted if*
  `TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker` shows a provider
  child's 97/98 propagated into the reserved codes.
- **A7 — Floor is a typed signal recorded without parsing.** *Refuted if*
  `TestAgentLoopReservedExitCodeRoutesUnrecoverableWithoutPTYParsing` shows exit 97
  failing to route the durable blocker, or the decision reading output bytes.
- **A7' — Exit-code backstop observed via `#{pane_dead_status}`, not the attach client
  (the v5 drift fix).** *Refuted if* the tmux-path floor reads the pane wrapper's reserved
  exit from `result.Cmd.Wait()` (the attach client) rather than the new
  `#{pane_dead_status}` capture, or a pane-emitted 97/98 fails to route on the production
  path while the attach client exits 0.
- **A8 — Positive intent is daemon-observed + authored-this-attempt.** *Refuted if*
  `TestCodexResealUsesReceiverNotProviderStdout` shows a seal driven by provider-asserted
  intent without the daemon-observed condition, OR a path outside `expected_artifacts`
  accepted, OR `TestResealRequiresAuthoredExpectedArtifactChange` reseals an UNCHANGED
  pre-existing expected path.
- **A9 — Backend-gate routing.** *Refuted if*
  `TestResealExit98BypassesBackendGateOrRoutesTyped` leaks `invalid_transition`/backend
  errors instead of sealing via the internal core or routing the typed class, or requires
  a live attached backend.
- **A10 — No split-brain, by stamped generation.** *Refuted if*
  `TestResealPredicateUsesStampedRecoveryGeneration` /
  `TestResealTokenRefusedAfterLeaseExpiryAndRecoveryRequeue` shows a reseal succeeding
  after a generation bump, or publishing into a requeued/retired job.
- **A11 — Numeric grace, never raw `lease_error`.** *Refuted if*
  `TestResealBeyondGraceRoutesTypedNotLeaseError` yields a raw `lease_error`, or grace
  exceeds `min(resealGraceWindow, heartbeat_after_seconds)`.
- **A12 — One extension, no revive.** *Refuted if*
  `TestResealGraceCannotReviveRequeuedLease` extends a lease twice or revives a requeued
  lease.
- **A13 — Lock order serializes reseal vs sweep.** *Refuted if*
  `TestRecoveryRequeueWinsOverExpiredLeaseReseal` or the `run_lock_guard_test.go`
  guardrail shows a reseal taking a run-scoped `FOR UPDATE` before
  `pg_advisory_xact_lock`, or an interleave that split-brains.
- **A14 — Grace marker migration pinned (owner DDL).** *Refuted if* a runtime migration
  carries the `leases.reseal_grace_extended_at` ALTER
  (`TestFutureRuntimeMigrationsDoNotCarryOwnerDDL` /
  `TestRuntimeMigrationsDoNotCarryOwnerDDLForLeasesGraceColumn`), or the column lands
  outside bundle 0021.
- **A15 — Generation column migration pinned (owner DDL).** *Refuted if* a runtime
  migration carries the `jobs.recovery_generation` ALTER, or the
  `LatestOwnerBundleVersion` 20→21 bump is absent.
- **A16 — Epoch path does not weaken #316.** *Refuted if*
  `TestResealEpochMirrorRejectsTamperOrMissingEpoch` shows a lane-writable epoch source,
  a successful symlink/replace, or a missing-header supervised request accepted.
- **A17 — Token validity survives the restart.** *Refuted if* `TestTokenValidAcrossRestart`
  shows the PG-resident token rejected purely because the process restarted.
- **A18 — Loud, durable, lease-bounded failure.** *Refuted if* game-day **GD-1**
  (restart `striatumd` mid-job, no reachable token file) shows a silent unsealed exit, a
  raw permission error, or no durable `session_unrecoverable_across_rotation` blocker; or
  **GD-1b** yields a raw `lease_error`, stale-lease limbo, or a silent unsealed exit
  instead of a same-lease renew-and-seal within grace or the typed class; or **GD-CTL**
  (the connect-out real-path game-day) shows a sibling sealing, or the wrapper unable to
  reach the daemon over the connect-out channel under `RequireTmux`/`RunAsUser`.

## Adapter survival matrix (F6 — honest, re-grounded on the daemon-observed trigger)

No adapter needs to reload its MCP launch args to seal the in-flight job: the seal is
daemon-side (`resealInFlightJob`), triggered by the **daemon-observed post-rotation
condition** (prompted by the authenticated connect-out frame, the
`#{pane_dead_status}` backstop, or the recovery sweep) — adapter-independent and not
parsed from provider output.

| Adapter | Reseal-in-flight (Slice B) | Resume normal MCP work after rotation |
| --- | --- | --- |
| **Claude** (ephemeral MCP config) | daemon-observed condition → `resealInFlightJob` (no token reload) | #323 ephemeral-config rewrite + endpoint/epoch republish |
| **Agy / pipe** | same daemon-side path | same as Claude where supported |
| **Codex** (MCP URL baked into launch `-c` args) | same daemon-side path — **no in-place MCP survival claimed** | operator-assisted relaunch / `supervise rebridge` only |

*Refuting game-day — GD-Codex-Reseal-Rotation:* restart `striatumd` mid-job for a Codex
lane; the in-flight job seals over the daemon-observed path **or** fails legibly to
Option 4; the spec does **not** claim the Codex MCP client reconnected in place.

## Scope discipline (Non-Goals held)

- Does **not** re-classify the downstream `agent_exited_unsealed` recovery policy
  (RFC 0152 / D249).
- Does **not** change committee POSIX-ACL repo provisioning (#537 / #539).
- Does **not** touch `run drive`'s transient-socket behavior (#513).
- Does **not** weaken the #316 boot-epoch recycled-port defense (BC1/F7 strengthen it).
- Does **not** introduce any lane-readable credential file (the v1 `0600` reseal file
  stays retired).
- Does **not** collide with the RFC 0125 `HandleRecoveryReseal` worktree-durability verb
  (separate file `recovery_reseal.go`, separate verb, unrelated to credentials);
  `resealInFlightJob` lands in `recovery_reseal_rotation.go`.
- Local-first, single-host, daemon-owned PostgreSQL as the single writer.

## Maintainer ratification gate (required)

**Slice B introduces a new daemon-internal capability marker (`rpc.CapabilityReseal`), a
test-only auth-prelude route alternate, the daemon-owned supervisor control channel with
connect-out `SO_PEERCRED` (pid+start-time) authentication, the reserved agentloop exit
codes, the `jobs.recovery_generation` + `leases.reseal_grace_extended_at`
owner-bundle-0021 columns, and endpoint/epoch republish plumbing — a security/authz
trust-model change.** This cleared spec is a **RECOMMENDATION the maintainer ratifies
before any build slice touches credential code.** Slice A (the Option-4 typed floor) is
zero-trust-change and may land first under the normal review gate **now that BC1-CHANNEL
routes it over a real, non-PTY connect-out channel whose same-uid authentication is
anchored through the production tmux/`respawn-pane`/`sudo … env -i`/env-file launch path
(W1–W3 installed on the pane wrapper, not the tmux client).** Adjudicator clearance gates
the spec's **soundness**; it is not the maintainer's product call on the credential code.

---
<sub>Holder revised proposal (design-v5) for the RFC 0143 falsification-gate design run.
Resolves the single remaining binding constraint BC1-CHANNEL by replacing the v4
inherited-fd-through-`ExtraFiles` channel with a CONNECT-OUT channel installed on the
production tmux/`respawn-pane`/`sudo … env -i`/env-file launch path: the pane wrapper
sets `PR_SET_DUMPABLE(0)` first then dials a daemon-held `SO_PASSCRED` listener, the
daemon authenticates via `SO_PEERCRED` pid+`/proc`-start-time against the
already-captured `identity.PanePID`/`PaneStartToken`, the address is non-secret and the
nonce is delivered post-auth — so W1/W2/W3 hold STRUCTURALLY on the real channel with no
fd through tmux and no secret in env. Names the exact `HelperLaunchSpec`/`LaunchSpec`/
`RunHelper`/agentloop/`#{pane_dead_status}` plumbing sites, flags the v4 exit-code-on-tmux
drift and fixes it as a `#{pane_dead_status}` backstop, adds the real-path test
`TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper`, folds in the modified-since-baseline
authored-path build-test, and carries the v4-credited set (BC2/BC3/BC4/BC5 + C2 +
daemon-observed positive intent + backend-gate bypass + W1/W2/W3 shapes + F2/F4/F7-file/
AF1/AF4/no-widening/A1–A18) forward unregressed. The adjudicator's collaboration ledger —
not falsifier completion — decides whether this gate clears.</sub>
