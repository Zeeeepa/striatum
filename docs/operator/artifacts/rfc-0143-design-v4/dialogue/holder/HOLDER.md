# HOLDER — RFC 0143 falsifiable implementation spec (design-v4 REVISION)

author: holder-author-001

> This is the **fourth** falsification pass on RFC 0143 (*lane credential survival
> across a daemon boot-epoch rotation*) and a **proper revision**. v1
> (`rfc-0143-design`) returned `needs_revision` with seven findings F1–F7. v2
> (`rfc-0143-design-v2`) **resolved F2 and F4** cleanly and distilled the residue
> into five binding constraints BC1–BC5. v3 (`rfc-0143-design-v3`) **resolved BC2,
> BC3, BC4 at the design level** (both falsifiers credited them) and carried the
> v2-credited set forward unregressed, but returned `needs_revision` **again**:
> **BC1 stood open on three independent material grounds** and **BC5 had two open
> precision items**, both falsifiers' re-attacks landing unrebutted.
>
> This v4 spec starts from the **v3** `HOLDER.md` (required context), **resolves the
> two remaining binding constraints BC1 (all three grounds) and BC5 (both items)
> with a concrete, source-anchored mechanism and named tests**, and **carries the
> v3-credited resolved set forward unregressed** (BC2, BC3, BC4, F2, F4, the F7
> file-mirror half, AF1, AF4, the categorical no-admin-token-widening invariant, the
> per-claim falsifiable-assertion discipline). It does **not** relitigate the
> ratified OQ1 trust-model shape or the F2 non-bearer decision (both pinned in
> `SEED.md` `## Ratified design shape`). Every source citation below was re-verified
> against the current worktree while authoring this revision; the falsifiers
> re-attack this published claim.

## Root reframe (held, unchanged)

**A boot-epoch rotation must never force a lane to choose between reading the
daemon's full-authority bootstrap admin `client-token` and exiting silently
unsealed.** A `striatum-lane` lane authenticates as *its own* narrow,
session-scoped credential and **never** as the shared operator admin override. v4
either lets the lane's in-flight work be sealed over a **daemon-projected,
session-tied authority that no lane bearer carries**, or makes the failure **loud,
typed, and routed** — never silent, never via the admin token.

## What v4 changes vs v3 (the two remaining repairs, in one place each)

The v3 design was credited on BC2/BC3/BC4 and the entire carry-forward set; it fell
on two clusters. v4 closes both:

1. **The BC1 control channel is now STRUCTURALLY same-uid-authenticated, and the
   reseal trigger is daemon-OBSERVED, not provider-asserted.** v3's inherited-fd
   channel authenticated only fd-possession + an env nonce — both reachable by a
   same-uid sibling via `/proc/<wrapper-pid>/fd/3` and `/proc/<wrapper-pid>/environ`
   (the same-uid category mistake that killed the v1 `0600` file). v4 pins **three
   independent structural walls**: (W1) a **per-message kernel-stamped peer-credential
   check** (`SO_PASSCRED`/`SCM_CREDENTIALS`) binding every control frame to the
   *launched wrapper's* pid + start-time; (W2) **`PR_SET_DUMPABLE(0)`** on the
   wrapper (reinforced by the `sudo` setuid launch transition) so the `/proc` fd/env
   surface is root-owned; (W3) the **nonce delivered over the socketpair itself, out
   of the env entirely**. The reserved exit codes are **reserved by commitment** (the
   wrapper never propagates a provider child's 97/98). And the positive intent is
   **daemon-observed** — `resealInFlightJob` fires only on a precise post-rotation
   condition the daemon verifies against its own state + worktree, never on a signal
   the provider could assert.
2. **The BC5 lifecycle precision items are pinned to source.** `leases.reseal_grace_extended_at`
   lands in the **same owner bundle 0021** as `jobs.recovery_generation` (`leases`
   is owner-held — created by runtime migration 0005, NOT in the owner/0018
   ownership-transfer cohort). The `work.complete` lock-order story is corrected:
   `resealInFlightJob` **skips** the two pre-`lockRunForJob` public gates
   (`enforceSessionBindingForSession`, `enforceActiveActingSession`) and the
   post-lock `ensureWorkSessionBackend` gate — replacing them with a daemon-state
   projection — and **replays** `lockRunForJob` + the row locks + the reseal
   predicate, so expired-beyond-grace ALWAYS routes the typed class.

Both land in **one new daemon mutation, `resealInFlightJob`**
(`go/pkg/mutations/recovery_reseal_rotation.go`, deliberately a *different* file and
verb from the RFC 0125 `HandleRecoveryReseal` / `go/pkg/mutations/recovery_reseal.go`,
the worktree-durability operator verb, which is unrelated to credentials).

## Ratified design shape (pinned — built on, not relitigated)

- **OQ1 (ratified):** Slice A = Option 4 (mandatory, zero-trust-change, lands first)
  + Slice B (ratification-gated) = Option 2's narrow `CapabilityReseal` over a
  daemon-owned session-tied path + minimal Option 3 per-session endpoint+epoch
  republish. No lane-readable reseal bearer file under any option.
- **F2 (decided):** non-bearer, daemon-owned, session-tied channel; **no readable
  reseal token file at all** (every lane shares the `striatum-lane` uid, so any
  `0600` file is a same-uid replay surface). Not reopened. **v4 extends the same
  same-uid threat model to the live channel and closes it structurally (BC1).**
- **Slice B requires maintainer ratification** before any build slice touches
  credential code. Adjudicator clearance gates the spec's *soundness*, not the
  maintainer's product call. Slice A is zero-trust-change and may land first under
  the normal review gate **once BC1 routes it over a real, non-PTY channel with the
  same-uid authentication fixed** — which v4 does.

## Architectural facts re-anchored (AF1–AF4 — carried forward unregressed)

- **AF1 — reachability, not reminting (credited v1/v2/v3 strength, kept).**
  `mintSessionBoundToken` (`go/pkg/mutations/session_token.go`) inserts the client
  row + per-capability grants into daemon-owned PostgreSQL bound to `session_id`,
  24h TTL. **PostgreSQL survives a `striatumd` restart** (D094 / RFC 0043). After a
  boot-epoch rotation the token is still *valid* — only *unreachable*, because it
  lives as the `STRIATUM_MCP_TOKEN` env literal (step 1) and the post-rotation
  re-readers skip step 1. The fix is routing, not re-minting.
  *Falsifier:* `TestTokenValidAcrossRestart`.
- **AF2 — the post-rotation re-readers fall to step 3.** `ResolveTokenMaterial`
  (`go/pkg/agentloop/token.go:18-53`) reaches the runtime `client-token` branch at
  `:31-41` whenever steps 1/2 are absent; the #323 fresh re-read
  (`ResolveTokenMaterialFresh`, `go/pkg/agentloop/endpoint.go`) likewise skips the
  env literal and finds no lane-readable credential, falling to step 3 — the bug.
- **AF3 — step 3 is the full-authority admin token in a `0700` dir.**
  `admin.BootstrapRuntimeTokenIfNeeded` (`go/pkg/admin/bootstrap.go:18-27`) grants
  the runtime `client-token` the full `bootstrapCapabilities`
  `{admin,read,write,claim,review,apply,recovery,surgical_recovery}`, `0600` in a
  `0700` dir; `ReadTokenFile` (`token.go:75-92`) rejects any token file not
  owner-only. The `0700` dir is the first wall and is load-bearing for the invariant.
- **AF4 — epoch/token decoupling (credited strength, kept).** Endpoint + boot epoch
  rotate together; #316 deliberately retires a surviving lane's connection by
  rejecting a stale epoch. The token does **not** rotate on a normal restart — only
  the endpoint does. Preserved.

## Carried forward from v3, unregressed (do NOT reopen)

| Item | Status | Anchor / test kept |
| --- | --- | --- |
| **BC2** — reseal artifact identity from the job's `expected_artifacts` (daemon state); refuses unexpected paths; front-matter failure routes to the floor | resolved (carried) | `verifyRequiredArtifacts`/`ensurePerJobPublishedArtifactsDurable`; `TestCodexResealUsesReceiverNotProviderStdout` (+ v4 positive case) |
| **BC3** — `CapabilityReseal` is a daemon-internal marker projected by `resealInFlightJob`; public route-alternate test-only | resolved (carried) | `TestResealCapabilityIsDaemonInternalNotBearer` / `TestResealTokenCanReachOnlyResealRoutesWithoutWrite` |
| **BC4** — concrete monotonic `jobs.recovery_generation` (owner bundle 0021), increment points, stamped value compared under the lock | resolved (carried) | `TestResealPredicateUsesStampedRecoveryGeneration` / `TestResealTokenRefusedAfterLeaseExpiryAndRecoveryRequeue` |
| **F2** — v1 `0600` bearer file retired; reseal authority non-bearer | resolved | no on-disk reseal bearer exists; `TestBorrowedResealBearerCannotSealVictimSession` |
| **F4** — route-specific `MethodEntry.ResealAlternate` admits `CapabilityReseal` on only `interrogation.answer`/`work.complete`/`artifact.publish`, records `reseal` not `write` | resolved | `TestResealTokenCanReachOnlyResealRoutesWithoutWrite`; command-authority-matrix reseal column |
| **F7 file-mirror half** — endpoint/epoch on a daemon-owned lane-read-only `0644` file, `O_NOFOLLOW`, atomic rename, reject MISSING epoch header on the supervised path (closes #316 permissive header-absent) | resolved | `TestResealEpochMirrorRejectsTamperOrMissingEpoch` |
| **AF1** reachability-not-reminting | kept | `TestTokenValidAcrossRestart` |
| **AF4** epoch/token decoupling | kept | (above) |
| **No-admin-token-widening invariant** | held + strengthened | `TestResolveRefusesRuntimeClientTokenForLane` |
| **Per-claim falsifiable-assertion discipline** | extended to peer-cred / dumpability / nonce-isolation + migration-site + lock-order | A1–A18 below |

The carried-forward sections (BC2, BC3, BC4) are reproduced in full below for the
falsifiers and the build run, verbatim in substance from v3; only the BC1-dependent
*trigger* changes (now daemon-observed, §BC1).

---

# Security cluster (BC1 + BC2 + BC3)

## BC1 — Same-uid-AUTHENTICATED control channel + daemon-OBSERVED positive intent (closes F1 / F6 / the channel half of F2 / F7-channel)

v3 named two non-PTY mechanisms — the trusted-wrapper **OS exit code** and a
daemon-owned **inherited file descriptor** — and both falsifiers agreed v3 no
longer parses PTY output. v4 keeps both mechanisms and closes the three unrebutted
v3 grounds **in one place**. The structural property the gate demands —
**no-replay must hold structurally on the channel** — is delivered by three
independent walls (W1–W3), each of which alone defeats the `/proc` same-uid replay.

### The process topology (re-anchored)

Three process layers, two privilege levels:

- **helper** (`RunHelper`, `go/pkg/supervisor/helper.go:120-128`) — runs at the
  **daemon uid** (e.g. `halbritt`). It launches the wrapper under a PTY and, per
  `pty.go:223`, drops to the lane user via `sudo -n -u <RunAsUser> -- …`. It
  "deliberately does not … inspect workflow state … only moves process bytes and
  reports control events" (`helper.go:120-127`).
- **wrapper** (the agentloop process, `go/pkg/agentloop/loop.go:221` `runWithIO`) —
  runs as **`striatum-lane`**. It is `HelperLaunchSpec.Command`
  (`helper_protocol.go:31`). It prepares the lane command and execs the provider as
  a **child** via `exec.CommandContext(ctx, laneCommand[0], …)` (`loop.go:266`) and
  `cmd.Wait()`s it (`loop.go:365`). Per `loop.go:220-368` the wrapper does **not**
  know whether the deliverable is complete — the semantic work lives in the provider.
- **provider** (claude/codex CLI) — runs as **`striatum-lane`**, the wrapper's child.

Sibling lanes are also `striatum-lane`: the same-uid threat is a sibling
`striatum-lane` process that is **neither the provider child nor the launched
wrapper**.

### BC1-(b) — the trusted-wrapper EXIT CODE is the floor + the post-rotation reseal PROMPT (recorded without parsing output)

The helper already captures the wrapper's **OS process exit status** into the
`agent_exited` payload — `agentExitPayload` → `processExitCode`
(`helper.go:427-439`, via `(*exec.ExitError).ExitCode()`), an OS-level value, **never
read from stdout/stderr**; the curated payload keeps only `exit_code`/`error`/`cause`
(`supervision.go:424-425`). The daemon recognises reserved codes in the **existing**
`agent_exited` branch of `recordSuperviseReportEvent` (`supervision.go:298-306`,
which reads `event.Payload["exit_code"]` and sets `state="stopped"`).

v4 reserves two agentloop exit-code constants (new, `go/pkg/agentloop/exitcodes.go`):

- `ExitUnrecoverableAcrossRotation = 97` — the wrapper exits this **instead of
  falling through to the admin `client-token`**. `ResolveTokenMaterial`
  (`token.go:31-41`) / `ResolveTokenMaterialFresh` (`endpoint.go`) return a typed
  `ErrSessionUnrecoverableAcrossRotation` for a supervised lane rather than the
  runtime `client-token`; `loop.go` maps that sentinel to exit 97. On 97 the daemon
  records a durable `session_unrecoverable_across_rotation` blocker (the Option-4
  floor; `blockers` has **no CHECK on `blocker_kind`**, `0005:259-276`, so the
  free-text kind is buildable).
- `ExitResealInFlightRequested = 98` — a **latency hint only**: it PROMPTS the
  daemon to evaluate the post-rotation reseal condition (below) promptly. **Its
  forgeability is immaterial to security** — the daemon never seals on the strength
  of 98; it independently verifies the condition against its own state + worktree,
  and a forged 98 can do nothing but trigger a check that is a no-op or routes to the
  floor (BC2 bounds it to the lane's own job from daemon state).

**C2 — reserved codes reserved BY COMMITMENT (resolved).** Choosing a high range is
**not** an auth boundary. v4 commits the wrapper to **never propagate a provider
child's status into the reserved codes**: in `runWithIO`, after `cmd.Wait()`
(`loop.go:365`) the wrapper inspects the provider child's exit status and, if it is
97 or 98, **remaps it to a non-control `agent_exited` outcome** (a distinct
wrapper exit, e.g. the existing generic agent-exit code, carrying the provider's raw
status in a non-reserved payload field) — the reserved 97/98 are emitted **only** by
the wrapper's own typed-error path (`ErrSessionUnrecoverableAcrossRotation` → 97; the
wrapper's own post-rotation reseal-prompt → 98), never forwarded from the child. The
agentloop owns 97/98 semantics exclusively.
*Falsifier:* `TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker` — a
provider child that exits 97 or 98 produces a non-control `agent_exited`, never a
`session_unrecoverable_across_rotation` blocker and never a `resealInFlightJob` call.

### BC1-(a) — the private control channel: an inherited fd with STRUCTURAL same-uid authentication (W1+W2+W3)

For a mid-life signal (or a richer post-rotation prompt) v4 keeps the daemon-owned
**inherited file descriptor** but replaces "no filesystem name" with three
structural walls. The channel is a `socketpair(AF_UNIX, SOCK_SEQPACKET, 0)` created
by the **helper** (daemon uid): one end is passed to the wrapper as an inherited fd
via `exec.Cmd.ExtraFiles` (the wrapper sees fd 3; advertised as
`STRIATUM_SUPERVISOR_CONTROL_FD=3`), the helper keeps the other end. The wrapper does
**not** pass fd 3 to the provider (the provider is exec'd with stdio + the PTY only;
fd 3 is `O_CLOEXEC` across that exec). `HelperLaunchSpec` gains a `ControlFD`
plumbing field; `RunHelper` gains a `pumpControlChannel` reader goroutine alongside
`pumpPTYProgress`/`forwardPacketStream` (`helper.go:200-208`). `SOCK_SEQPACKET`
preserves message boundaries (one frame per datagram) and carries ancillary data.

- **W1 — per-message kernel-stamped peer credentials (LOAD-BEARING; holds even if
  W2/W3 leak).** The helper sets `SO_PASSCRED` on its end. Every control frame the
  wrapper sends MUST arrive with `SCM_CREDENTIALS` ancillary data; the **kernel**
  fills the sender's real `{pid, uid, gid}` and refuses to let an unprivileged
  process forge another pid's `ucred` (forging requires `CAP_SYS_ADMIN`; spoofing
  uid/gid requires `CAP_SETUID`/`CAP_SETGID`). The helper — which **launched** the
  wrapper and therefore holds its authoritative pid (`result.PID`) and its launch
  start-time (read once from `/proc/<pid>/stat` field 22 at launch, before any frame
  is accepted) — rejects any frame whose stamped `pid != launched-wrapper-pid` OR
  whose `/proc/<pid>/stat` start-time ≠ the recorded launch start-time (defeating pid
  reuse). Only then does the helper emit the `reseal_requested` `HelperControlEvent`.
  A same-uid sibling that somehow holds fd 3 still has the kernel stamp **its own**
  pid → rejected. This is the structural no-replay property: it does **not** depend on
  the fd or the nonce staying secret.
  - *Note on `SO_PEERCRED`:* a plain `getsockopt(SO_PEERCRED)` on a `socketpair` end
    returns the credentials recorded **at `socketpair()` time** — i.e. the helper's
    own pid — and is therefore useless here. v4 deliberately uses **per-message
    `SCM_CREDENTIALS`** (the sender's real pid at send time), not `SO_PEERCRED`.
- **W2 — `PR_SET_DUMPABLE(0)` (closes the `/proc` read surface).** The wrapper calls
  `prctl(PR_SET_DUMPABLE, 0)` as its **first runtime action**, before fd 3 or any
  nonce are live. With `dumpable=0` the kernel re-owns `/proc/<wrapper-pid>/fd`,
  `/proc/<wrapper-pid>/environ`, and `/proc/<wrapper-pid>/mem` to `root:root`, so a
  same-uid sibling cannot duplicate fd 3 or read the env at all. This is reinforced
  structurally by the `sudo -u striatum-lane` launch (`pty.go:223`): a setuid
  execve makes the kernel reset `dumpable` to `/proc/sys/fs/suid_dumpable` (default
  `0 = SUID_DUMP_DISABLE`) for the new credential-changed process — so on a default
  host the wrapper is non-dumpable from its first instruction. The explicit `prctl`
  makes it host-independent (does not rely on the `suid_dumpable` sysctl).
- **W3 — nonce out of the env entirely.** The single-use `control_nonce` (per launch,
  per generation) is **written by the helper into the socketpair itself** at launch
  and read by the wrapper from fd 3 — it is **never** placed in the env, so it never
  appears in `/proc/<wrapper-pid>/environ` even if W2 failed. The wrapper echoes the
  nonce on each frame; the daemon rejects any frame whose nonce ≠ the launch nonce
  for that `supervisor_id`, and the BC4 generation guard refuses a nonce from a prior
  generation. The nonce now ties the frame to the launch *generation*; **W1 (peer
  cred), not the nonce, is the primary authentication.**

**Frame schema.** One `SupervisorControlFrame` per datagram (mirroring
`HelperControlEvent`): `{ schema_version: "striatum.supervisor_control.v1", type:
"reseal_requested" | "unrecoverable_across_rotation", supervisor_id, control_nonce }`.
It carries **NO** job_id, artifact path, kind, or body — identity is derived from
daemon state (BC2). The PTY (provider stdout/stderr, fd 1/2) reaches **only** the
volume meter (`pumpPTYProgress`, D028, `helper.go:357-415`); `pumpControlChannel`
reads frames **only** from fd 3; `superviseReportEventTypes` (`supervision.go:19-28`)
admits **no** content/output event. Even a perfectly-formed frame can do nothing but
"evaluate *my own* in-flight job's reseal condition from daemon state" (BC2).

**Structural no-replay (the spine).** A sibling `striatum-lane` process that is
neither the provider child nor the launched wrapper cannot: (W2) open
`/proc/<wrapper-pid>/fd/3` or read `/proc/<wrapper-pid>/environ`; (W3) recover the
nonce (it is not in env); **and, decisively, (W1) author a frame the helper
accepts** — the kernel stamps the sibling's real pid, which ≠ the launched wrapper
pid. Granting the sibling *both* the fd and the nonce (W2+W3 bypassed), W1 still
rejects it. No-replay holds **structurally**, not as a trackable finding.

### BC1 — positive intent is DAEMON-OBSERVED, not provider-asserted (closes the v3 positive-intent gap)

The v3 gap: cutting the provider out of the channel left **no** non-PTY,
non-bearer, non-sibling-replayable source of "deliverable complete on disk, seal
it." v4 resolves this with SEED option **(a): automatic/speculative reseal on a
precise daemon-observed post-rotation condition** — the daemon's own observation is
the authority, so no provider-to-wrapper intent protocol is needed at all (this
sidesteps the `loop.go:220-368` wrapper-doesn't-know-completion problem entirely).
Option (b) (a provider→wrapper intent path) is **explicitly NOT relied upon**.

`resealInFlightJob` fires **only** when ALL of the following hold, evaluated under
the run advisory lock (§BC5):

1. **A boot-epoch rotation occurred during this job's lease.** The daemon observed
   its own boot-epoch increment since the packet was issued (the recorded packet
   epoch vs the current `writeBootEpochFile` epoch). Absent a rotation there is no
   reseal-after-rotation case — the normal seal path applies.
2. **The job is still `running`, the lane's lease is bound, the stamped generation
   matches** the live `jobs.recovery_generation` (BC4), **and** the lease is within
   grace (BC5). Any mismatch → typed floor.
3. **Every required `expected_artifact` path (from daemon state, BC2) is present in
   the job's active worktree AND was modified since the packet was issued.**
   Concretely the daemon re-hashes each required path in the job's worktree
   (`job_worktrees`, `0005:350-372`) and confirms the content hash **differs from the
   `write_scope_baseline.changed_paths` baseline the packet already carries** (the
   work packet ships `write_scope_baseline` with per-path pre-work hashes — observed
   in this very packet). Content-hash-vs-baseline is robust where mtime is not.
   "Present + modified-since-baseline" is the daemon's evidence the deliverable was
   actually produced; absent it, route to the floor, never a speculative seal.
4. `resealInFlightJob` then attempts **only daemon-derived artifacts** (BC2 — paths
   from `expected_artifacts`, content from the worktree) and **maps ALL
   validation/backend/front-matter/durability failures to the typed
   `session_unrecoverable_across_rotation` floor** (Option-4). Never a silent seal,
   never a raw error.

**Two entry points, one condition, one backstop for "the wrapper can't even
signal."** The condition is evaluated from (i) the wrapper's post-rotation exit
(the 98 prompt, or any clean post-rotation exit), routed via the `agent_exited`
branch; **and** (ii) the **recovery sweep** when it observes a post-rotation expired
lease with a complete-looking deliverable — it evaluates the **same** condition under
the **same** lock **before** requeuing. So even if the wrapper dies without emitting
98, the post-rotation deliverable still gets one daemon-observed reseal attempt (or
the typed floor), and the positive trigger has no "what if the wrapper can't signal"
hole.

**Backend-gate routing (the v3 conflict, resolved — same mechanism as BC5).** The
reserved exit arrives as `agent_exited`, whose branch **stops the supervisor**
(`supervision.go:298-306`, `state="stopped"`). The normal `work.complete` core then
requires a live attached backend via `ensureWorkSessionBackend`
(`lifecycle.go:1181`). v4 does **not** reuse `HandleCompleteWork`. `resealInFlightJob`
calls the lower-level complete core and **deliberately bypasses
`ensureWorkSessionBackend`** — the reseal exists *precisely because* the live
connection is gone; requiring a live backend would (correctly, for a normal complete)
leak a backend error, which is the v3 leak. The exact gate map is in §BC5.
*Falsifier:* `TestResealExit98BypassesBackendGateOrRoutesTyped` — a post-exit reseal
on a stopped supervisor seals via the internal core **or** routes the typed class;
it never leaks `invalid_transition`/backend errors, and never requires a live backend.

**New / updated tests (BC1):**
- `TestSiblingSameUIDCannotOpenControlFDOrReplayNonceViaProc` — a same-uid process
  that is **neither the provider child nor the launched wrapper** cannot open
  `/proc/<wrapper-pid>/fd/3` or read the nonce from `/proc/<wrapper-pid>/environ`
  (W2/W3); with `dumpable=0` both are root-owned.
- `TestControlFrameRequiresExpectedWrapperPeerCredentials` — a frame whose
  `SCM_CREDENTIALS` pid ≠ the launched wrapper pid (or whose start-time ≠ the
  recorded launch start-time) is **rejected**; only a frame stamped with the launched
  wrapper's pid+start-time is accepted (W1). Run against a non-child, non-wrapper,
  same-uid sender.
- `TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker` — a provider
  child exiting 97/98 is remapped to a non-control `agent_exited`; it never drives a
  blocker or `resealInFlightJob` (C2).
- `TestResealExit98BypassesBackendGateOrRoutesTyped` — exit 98 on a stopped
  supervisor seals via the internal core or routes the typed class, never a backend
  error.
- `TestPTYOutputCannotEmitSupervisorControlEvent` / `TestProviderOutputCannotDriveResealOrBlocker`
  (kept) — PTY/stdout bytes never produce a control event or reseal; a child without
  fd 3 cannot drive one.
- `TestAgentLoopReservedExitCodeRoutesUnrecoverableWithoutPTYParsing` (kept) — exit
  97 routes the typed blocker and exit 98 routes the reseal-condition evaluation
  purely from `event.Payload["exit_code"]`, the PTY pump stubbed to assert no output
  read participates.

## BC2 — Artifact identity from daemon state, never from output (resolved, carried forward)

Even over an authenticated channel — and especially under daemon-observed reseal —
the daemon must not trust any signal for **what** is sealed. `resealInFlightJob`
derives the expected-artifact set from **its own state** and refuses any unexpected
path, reusing the existing handler payload contracts verbatim:

- **`artifact.publish`** requires `session_id`/`job_id`/`lease_id`/`kind`/
  `logical_name`/`path` (`go/pkg/mutations/artifact.go:52-60`), takes `lockRunForJob`
  first and enforces session binding (`HandlePublishArtifact`, `artifact.go:64-83`).
- **`work.complete`** requires `session_id`/`job_id`/`lease_id` (`lifecycle.go:1124-1130`).
- **`interrogation.answer`** requires `session_id`/`interrogation_id`/`body`
  (`go/pkg/mutations/interrogation.go:217-221`).

For a reseal **complete**, the daemon resolves `jobs.expected_artifacts_json`
(attempt-resolved via `resolveExpectedArtifactCycles`) and verifies every required
artifact is durable, reusing `verifyRequiredArtifacts` (`mutations.go:828-876`) and
`ensurePerJobPublishedArtifactsDurable` (`artifact_durability.go`). For a reseal
**publish**, the daemon publishes **only** a `path` that is an open entry in the
job's `expected_artifacts`, reading the body from the job's own worktree
(`job_worktrees`, `0005:350-372`), and **refuses any path not in the expected set**.
The signal supplies neither path nor content.

**Front-matter / author-line failure routes to the floor, never a silent drop.** If
a reseal-publish hits an author-line / front-matter validation failure (the
publisher refuses invalid front matter with exit code 6), `resealInFlightJob` records
the `session_unrecoverable_across_rotation` blocker with the validation error in the
payload (the Option-4 floor), so a malformed reseal surfaces loudly.

**New / updated test (BC2):** `TestCodexResealUsesReceiverNotProviderStdout` — now
includes a **POSITIVE** case: a Codex lane whose deliverable is complete-on-disk
post-rotation is **automatically resealed** by the daemon-observed condition (all
`expected_artifacts` present + modified-since-baseline), sealing the in-flight job's
artifacts from daemon state + worktree; and the negative case — a frame/stdout
claiming a path **not** in `expected_artifacts` is refused, nothing read from
provider stdout.

## BC3 — `CapabilityReseal` is a daemon-internal marker, not a public bearer capability (resolved, carried forward)

`CapabilityReseal` is a **daemon-internal capability marker projected by the private
`resealInFlightJob` mutation**, not a public bearer capability:

- **Projection, not presentation.** `resealInFlightJob` maps `supervisor_id` →
  `session_id` from the supervision row (`process_supervisors` /
  `process_supervisor_pointers`, the same lookup `recordSuperviseReportEvent` uses via
  `findReportSupervisor`, `supervision.go:497-528`), constructs an **internal**
  `rpc.AuthContext{ Capability: CapabilityReseal, SessionID, RepositoryID }`
  **without** the public `Authorize` prelude (`go/pkg/rpc/server.go:107-111`), threads
  it with `WithAuthContext`, and calls the lower-level publish/complete routines
  against the job's active worktree. The authority is **daemon-projected**; no bearer
  reaches the lane.
- **Public route-alternate kept for tests only.** The F4 wiring stays exactly as
  credited — `MethodEntry.ResealAlternate` set true on only `interrogation.answer`/
  `work.complete`/`artifact.publish`; the prelude re-authorises against
  `CapabilityReseal` on a `capability_missing` for those routes and records
  `AuthContext.Capability == reseal` (never `write`). With **no production reseal
  bearer**, this path is exercised **only by the guardrail tests**. `registry_methods.go`
  is **generated** (`// Code generated by … routergen … DO NOT EDIT`), so
  `ResealAlternate` lands in `contracts/daemon_methods.json` + the `MethodEntry` struct
  (`go/pkg/rpc/registry.go`) + the regenerated map + a reseal column in
  `docs/reference/command-authority-matrix.md` + the authority guardrail.
- **Reseal payload schema + validation/reuse path.** The daemon-internal call reuses
  the existing payload contracts (BC2): complete = `{session_id, job_id, lease_id}`;
  publish = `{session_id, job_id, lease_id, kind, logical_name, path}` with `path ∈
  expected_artifacts`; answer = `{session_id, interrogation_id, body}`. Any
  validation failure (binding, write-scope, front matter, unexpected path) routes to
  the Option-4 floor (BC2), never a silent drop.

**Test (BC3):** `TestResealCapabilityIsDaemonInternalNotBearer` — no live caller can
present `CapabilityReseal` (no bearer exists); the only path that seals is the
internal `resealInFlightJob` projection keyed to `supervisor_id`↔`session_id`; the
route-alternate is reachable only from the guardrail harness.

---

# Lifecycle cluster (BC4 + BC5)

## BC4 — Concrete monotonic generation column for the split-brain guard (resolved, carried forward)

`jobs` is an **owner-held** table, so a column-add is owner DDL —
`TestFutureRuntimeMigrationsDoNotCarryOwnerDDL` forbids a runtime migration from
ALTERing it. v4 ships owner bundle **`go/pkg/db/sql/owner/0021_job_recovery_generation.sql`**:
`ALTER TABLE striatumd.jobs ADD COLUMN IF NOT EXISTS recovery_generation integer NOT
NULL DEFAULT 0;`, bumps `LatestOwnerBundleVersion` 20→21 (`go/pkg/db/owner.go:23` —
**currently 20**, re-verified) with the ordinal-21 `[[owner_bundle]]` reservation in
`RESERVATIONS.toml` (`go/pkg/db/reservations.go`), modelled exactly on the credited
`review_generation` precedent (`owner/0009_review_generation.sql`).

- **Degrade-safe presence probe.** A `db.JobRecoveryGenerationColumnPresent` helper
  (mirroring `SessionPipeReadColumnPresent` / `ArtifactPlacementColumnPresent` /
  `reviewGenerationEnabled`, `go/pkg/db/artifact_write.go:64-102`). If the column is
  absent (daemon ahead of bundle 21), `resealInFlightJob` treats the generation as
  **unverifiable and routes to the typed floor** — never seals without the guard.
- **Increment points (each in the same UPDATE that retires/rebinds the job's
  authoritative lease, all under `lockRun`):** (1) **claim** — `claimChosenJob`
  (`go/pkg/mutations/claim.go:222-228`); (2) **requeue (same attempt)** —
  `requeueJobSameAttempt` (`recovery.go:2097-2109`); (3) **recovery sweep
  expire/transfer/respawn** — the `current_lease_id = NULL` transitions in
  `HandleRecoveryAuto`/`SweepRun` (`recovery.go:619`/`:2546`/`:2854`/`:2935`); (4)
  **release** — `work.release`. Monotonic by construction (only `+1`), like
  `jobs.attempt`/`review_generation`.
- **Stamped value for reseal-time comparison.** `claimChosenJob` writes the
  post-increment `recovery_generation` into the work-packet's `lease` block
  (`buildPacket`, persisted in `work_packets.packet_json`, `claim.go:229-260`) as
  `lease.recovery_generation`. At reseal, `resealInFlightJob` reads the stamped
  generation from the bound `work_packets` row and compares it to the **live**
  `jobs.recovery_generation` under the lock — equal → proceed; unequal → typed class.

**Tests (BC4):** `TestResealPredicateUsesStampedRecoveryGeneration` /
`TestResealTokenRefusedAfterLeaseExpiryAndRecoveryRequeue` (kept).

## BC5 — Numeric grace + PINNED migration site + CORRECTED lock order (closes F5)

v3 left two precision items open. v4 closes both.

### BC5-(1) — `leases.reseal_grace_extended_at` PINNED to owner bundle 0021

`striatumd.leases` is created in **runtime migration 0005**
(`go/pkg/db/sql/0005_repo_local_workflow_state.sql:166`) and is **owner-held**: it is
**NOT** in the migration-0016+ ownership-transfer cohort
(`go/pkg/db/sql/owner/0018_runtime_table_ownership_transfer.sql` — re-verified; its
`v_cohort` is `job_recovery_state, barrier_staged_contributions, barrier_state,
fanin_freeze_points, conversations, conversation_post_dialog_hooks, dissent_ledger,
interrogations, job_workspaces, spawn_authorization_grants` — `leases` is absent). On
a two-role deploy a table CREATEd by a runtime migration is owned by the owner role,
so a column-add to `leases` is **owner DDL**, consistent with
`TestFutureRuntimeMigrationsDoNotCarryOwnerDDL`.

**Pinned:** `reseal_grace_extended_at timestamptz` (NULL until used) is added in the
**same owner bundle 0021** as `jobs.recovery_generation` —
`go/pkg/db/sql/owner/0021_job_recovery_generation.sql` gains a second statement:
`ALTER TABLE striatumd.leases ADD COLUMN IF NOT EXISTS reseal_grace_extended_at
timestamptz;`. This is **not** a downstream decision — it is the same bundle, same
ownership posture, same guardrail as BC4. (Like `review_generation`, `striatumd_rw`'s
existing table-level grants extend to the new column; no new grant required.)
*Falsifier:* `TestRuntimeMigrationsDoNotCarryOwnerDDLForLeasesGraceColumn` (a runtime
migration ALTERing `leases` for this column fails the floor-guard) — folded into the
existing `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL` discipline.

### BC5-(2) — numeric grace + one extension + CORRECTED lock order

- **`resealGrace` numeric + source + maximum.** `const resealGraceWindow = 30 *
  time.Second` (new, beside the lease constants in `go/pkg/mutations`), **hard-capped**
  at the packet's heartbeat window: `grace = min(resealGraceWindow,
  packet.lease.heartbeat_after_seconds)` (heartbeat is 300s here; grace is 30s). It
  is a daemon-side allowance, **not** a lane-invokable `work.heartbeat` —
  `CapabilityReseal` carries no heartbeat verb.
- **One same-lease extension only.** `resealInFlightJob` may move the bound lease
  row's `expires_at` forward by `grace` **exactly once**, gated by
  `leases.reseal_grace_extended_at` (NULL until used; pinned above). Allowed only if
  `now() - expires_at ≤ grace` **and** `jobs.recovery_generation == stamped` **and**
  `reseal_grace_extended_at IS NULL`. A second expiry, or any generation change,
  forecloses further extension → typed floor.

- **CORRECTED lock-order story (the v3 inaccuracy, fixed).** v3 wrongly asserted the
  seal paths "already take `lockRunForJob` first." That is **false for `work.complete`**:
  `HandleCompleteWork` (`lifecycle.go:1119`) runs `enforceSessionBindingForSession`
  (`:1137`) and `enforceActiveActingSession` (`:1147`, conditional) **BEFORE**
  `lockRunForJob` (`:1154`), and `activeLeaseFor` (`:1178`) + `ensureWorkSessionBackend`
  (`:1181`) **AFTER** the lock. `resealInFlightJob` does **not** call
  `HandleCompleteWork`. It is a distinct mutation with this **exact** gate map:

  | `HandleCompleteWork` gate | site | `resealInFlightJob` |
  | --- | --- | --- |
  | `enforceSessionBindingForSession` (pre-lock) | `:1137` | **SKIP → REPLACE** with the supervision-row projection (`supervisor_id`↔`session_id`, BC3) + the bound `work_packets` row; there is no live calling session presenting a bound token to bind. |
  | `enforceActiveActingSession` (pre-lock) | `:1147` | **SKIP → REPLACE** — the supervisor was stopped at `agent_exited`; the internal path proves binding from daemon state, not from an active acting session. |
  | `lockRunForJob` (run advisory lock) | `:1154` | **REPLAY FIRST** — `pg_advisory_xact_lock(hashtext(run_id))` before any `FOR UPDATE`, identical to the seal paths and the sweep. |
  | `FOR UPDATE` jobs → leases → job_recovery_state | (rows) | **REPLAY** in stable key order under the run lock. |
  | `activeLeaseFor` (raw `lease_error` on expiry) | `:1178` | **SKIP → REPLACE** with the **reseal predicate** (generation match BC4, within-grace BC5, lease bound to this job/session, session not retired) → typed floor on any miss; **never a raw `lease_error`.** |
  | `ensureWorkSessionBackend` (live backend) | `:1181` | **SKIP (BYPASS)** — the reseal exists *because* no backend is live; requiring one is the v3 leak. |
  | `enforceWriteScopeClean` + `verifyRequiredArtifacts` + `ensurePublishedArtifactsDurable` + `running→completed` | `:1191-` | **REPLAY** — the actual state transition under the lock; any failure → typed floor. |

  So `resealInFlightJob` **skips** `{enforceSessionBindingForSession,
  enforceActiveActingSession, activeLeaseFor, ensureWorkSessionBackend}` and
  **replays** `{lockRunForJob, the FOR UPDATE locks, the reseal predicate,
  enforceWriteScopeClean, verifyRequiredArtifacts, ensurePublishedArtifactsDurable,
  the running→completed transition}` — the skipped pre-lock gates replaced by the
  daemon-state projection (BC3), the skipped backend gate replaced by the
  daemon-observed condition (BC1). This is the BC5 lock-order correction and the BC1
  backend-gate routing **in one mechanism**.

- **Serialization vs `artifact.publish` / `work.complete` / the recovery sweep.** All
  four take `lockRunForJob` / `lockRun(run_id)` (the same
  `pg_advisory_xact_lock(hashtext(run_id))`, `mutations.go:663-665`, RFC 0104)
  **FIRST**, before any run-scoped `FOR UPDATE` (the `run_lock_guard_test.go`
  guardrail requires this) — so at most one run-scoped writer runs at a time. The
  recovery sweep **drains helper events in short txns BEFORE `lockRun`**
  (`recovery.go:575-590`) but does `expireLeases`/requeue **INSIDE** the `lockRun` tx
  (`:610-621`). Therefore:
  - **Sweep wins the lock first:** it expires the lease and (on requeue) bumps the
    generation; `resealInFlightJob` then blocks, acquires the lock, observes the
    changed generation / expired-beyond-grace lease, and routes the typed class —
    **never revives the requeued lease.**
  - **Reseal wins the lock first:** it seals within grace and commits
    (`running→completed`); the sweep then observes a completed job and does not
    requeue.
  - A concurrent live `work.complete`/`artifact.publish` (if a backend reattached)
    serializes on the same lock; at most one reaches `running→completed`, the second
    sees state ≠ running and is an idempotent no-op.
- **Expired-beyond-grace ALWAYS routes the typed class.** `resealInFlightJob` never
  calls `activeLeaseFor` (whose `lease_error` is raw). On expiry-beyond-grace,
  generation mismatch, or a closed/retired session, the reseal predicate returns
  `ErrSessionUnrecoverableAcrossRotation` → the durable
  `session_unrecoverable_across_rotation` blocker. No raw `lease_error` ever reaches a
  post-rotation reseal.

**Tests (BC5):** `TestResealBeyondGraceRoutesTypedNotLeaseError`,
`TestResealGraceCannotReviveRequeuedLease`, `TestRecoveryRequeueWinsOverExpiredLeaseReseal`,
`GD-1b` (all kept), plus `TestResealExit98BypassesBackendGateOrRoutesTyped` and the
0021-migration guard above.

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
    if session not active:                  return typedFloor("session retired")          // replaces session-binding/active gates
    if leaseRow.owner != session or leaseRow.resource != job: return typedFloor("lease not this job/session")
    if jobRow.recovery_generation != pkt.lease.recovery_generation: return typedFloor("generation changed")
    if !bootEpochRotatedSincePacket(pkt):   return typedFloor("no post-rotation case")     // positive-intent condition (1)
    if leaseRow.expired:
        if within grace and generation matches and reseal_grace_extended_at IS NULL:
            UPDATE leases SET expires_at = now()+grace, reseal_grace_extended_at = now()   // ONE extension
        else: return typedFloor("expired beyond grace")
    if !supervisedEpochAccepted(session):   return typedFloor("epoch missing/mismatch")    // F7 channel half
    if !allRequiredArtifactsPresentAndModifiedSinceBaseline(job, pkt): return typedFloor("deliverable not observed")  // condition (3)
    authCtx := internal AuthContext{Capability: reseal, SessionID: session}                // BC3 projection
    switch intent:                                                                         // bypasses ensureWorkSessionBackend
      complete: enforceWriteScopeClean(job); verifyRequiredArtifacts(job); completeCore(authCtx, job, lease)  // BC2
      publish:  require path ∈ expected_artifacts ; publishArtifactWithOptions(authCtx)    // BC2
      answer:   interrogationAnswerCore(authCtx, ...)
    // any binding/write-scope/front-matter/backend/durability failure -> typedFloor(reason)
```
`typedFloor(reason)` records the durable `session_unrecoverable_across_rotation`
blocker (Option-4) — never a raw `lease_error`, never `invalid_transition`, never a
silent drop. The sketch is also the **recovery-sweep entry point**: the sweep calls
the same predicate (under the same lock) before requeuing a post-rotation expired
lease, so a wrapper that died without emitting 98 still gets one daemon-observed
attempt.

## Security invariant (the spine) — held and strengthened

The runtime `client-token` carries the full `bootstrapCapabilities` and is `0600` in
a `0700` dir (AF3; `bootstrap.go:18-27`). **Any path that lets a lane read that file,
or mints a lane-readable credential carrying any of `{admin, apply, recovery,
surgical_recovery}`, is categorically out of bounds for a FIX** — and v4 keeps it
structurally impossible:

- The lane never gets OS read of the `0700` dir (AF3); the Slice-A floor removes the
  only code path that would have read the `client-token` (`token.go:31-41` /
  `endpoint.go` return the typed error for a supervised lane).
- The only new authority, `CapabilityReseal`, carries **no elevated verb** and is
  **never materialised into any lane-readable file or bearer** (BC3 + F2). Nothing to
  read, steal, or replay.
- The reseal is **projected by the daemon only** on the supervisor-proven path; a
  lane cannot present `CapabilityReseal`, let alone admin/apply/recovery.
- The control channel is an **inherited fd authenticated by per-message kernel
  credentials** (BC1-a, W1) or the **OS exit code** (BC1-b) — neither is a bearer,
  neither is forgeable by a sibling lane or the provider, **structurally** (W1–W3).
- The epoch republish moves **endpoint + epoch only** (non-secret anti-confusion
  tags) over the daemon-owned, integrity-protected path (F7 file-mirror, kept); never
  the admin token.

*Falsifier:* `TestResolveRefusesRuntimeClientTokenForLane` —
`ResolveTokenMaterial`/`ResolveTokenMaterialFresh` return
`ErrSessionUnrecoverableAcrossRotation` for a supervised lane, never the runtime
`client-token`.

## Falsifiable assertions (each with the named test / game-day that refutes it)

- **A1 — No-widening.** `CapabilityReseal` carries only the three reseal verbs and is
  daemon-internal. *Refuted if* `TestResealTokenCanReachOnlyResealRoutesWithoutWrite`
  or `TestResealCapabilityIsDaemonInternalNotBearer` shows it reaching any of
  `admin`/`apply`/`recovery`/`surgical_recovery`/`work.claim_next`/any non-reseal
  route, resolving to `write`, or presentable as a bearer.
- **A2 — No admin-token fall-through.** *Refuted if*
  `TestResolveRefusesRuntimeClientTokenForLane` returns the runtime `client-token`
  for a supervised lane instead of the typed error.
- **A3 — No-replay, STRUCTURAL (W1).** Every accepted control frame is bound to the
  launched wrapper's pid + start-time by per-message `SCM_CREDENTIALS`. *Refuted if*
  `TestControlFrameRequiresExpectedWrapperPeerCredentials` accepts a frame stamped
  with any pid other than the launched wrapper's (or a mismatched start-time), or
  `TestBorrowedResealBearerCannotSealVictimSession` finds an on-disk reseal bearer or
  a sibling/foreign-session/provider-child sealing session A's job.
- **A4 — `/proc` surface closed (W2/W3).** *Refuted if*
  `TestSiblingSameUIDCannotOpenControlFDOrReplayNonceViaProc` opens
  `/proc/<wrapper-pid>/fd/3` or recovers the nonce from `/proc/<wrapper-pid>/environ`
  as a same-uid non-wrapper process, or the nonce is found in the wrapper env.
- **A5 — Control path never parses output.** *Refuted if*
  `TestPTYOutputCannotEmitSupervisorControlEvent` /
  `TestProviderOutputCannotDriveResealOrBlocker` shows PTY/stdout bytes driving a
  reseal or blocker, or the helper inspecting child output for a control decision.
- **A6 — Reserved exit codes reserved by commitment (C2).** *Refuted if*
  `TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker` shows a provider
  child's 97/98 propagated into the reserved codes (driving a blocker or reseal).
- **A7 — Floor is a typed exit code recorded without parsing.** *Refuted if*
  `TestAgentLoopReservedExitCodeRoutesUnrecoverableWithoutPTYParsing` shows exit 97
  failing to route the durable blocker, or the decision reading output bytes.
- **A8 — Positive intent is daemon-observed.** A reseal fires only when the daemon
  verifies a post-rotation case with all `expected_artifacts` present +
  modified-since-baseline. *Refuted if* `TestCodexResealUsesReceiverNotProviderStdout`
  shows a seal driven by provider-asserted intent (stdout/forged 98) without the
  daemon-observed condition, OR a path outside `expected_artifacts` accepted, OR
  artifact content read from provider stdout. (Positive case: a real complete-on-disk
  deliverable IS resealed.)
- **A9 — Backend-gate routing.** *Refuted if*
  `TestResealExit98BypassesBackendGateOrRoutesTyped` leaks
  `invalid_transition`/backend errors instead of sealing via the internal core or
  routing the typed class, or requires a live attached backend.
- **A10 — No split-brain, by stamped generation.** *Refuted if*
  `TestResealPredicateUsesStampedRecoveryGeneration` /
  `TestResealTokenRefusedAfterLeaseExpiryAndRecoveryRequeue` shows a reseal
  succeeding after a generation bump, or publishing into a requeued/retired job.
- **A11 — Numeric grace, never raw `lease_error`.** *Refuted if*
  `TestResealBeyondGraceRoutesTypedNotLeaseError` yields a raw `lease_error`, or grace
  exceeds `min(resealGraceWindow, heartbeat_after_seconds)`.
- **A12 — One extension, no revive.** *Refuted if*
  `TestResealGraceCannotReviveRequeuedLease` extends a lease twice or revives a
  requeued lease.
- **A13 — Lock order serializes reseal vs sweep.** *Refuted if*
  `TestRecoveryRequeueWinsOverExpiredLeaseReseal` or the `run_lock_guard_test.go`
  guardrail shows a reseal taking a run-scoped `FOR UPDATE` before
  `pg_advisory_xact_lock`, or an interleave that split-brains.
- **A14 — Grace marker migration pinned (owner DDL).** `leases.reseal_grace_extended_at`
  ships in owner bundle 0021. *Refuted if* a runtime migration carries the ALTER (it
  must fail `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL` /
  `TestRuntimeMigrationsDoNotCarryOwnerDDLForLeasesGraceColumn`), or the column lands
  outside bundle 0021.
- **A15 — Generation column migration pinned (owner DDL).** `jobs.recovery_generation`
  ships in owner bundle 0021 with `LatestOwnerBundleVersion` 20→21. *Refuted if* a
  runtime migration carries the ALTER, or the bump is absent.
- **A16 — Epoch path does not weaken #316.** *Refuted if*
  `TestResealEpochMirrorRejectsTamperOrMissingEpoch` shows a lane-writable epoch
  source, a successful symlink/replace, or a missing-header supervised request
  accepted.
- **A17 — Token validity survives the restart.** *Refuted if*
  `TestTokenValidAcrossRestart` shows the PG-resident token rejected purely because
  the process restarted.
- **A18 — Loud, durable, lease-bounded failure.** *Refuted if* game-day **GD-1**
  (restart `striatumd` mid-job, no reachable token file) shows a silent unsealed exit,
  a raw permission error, or no durable `session_unrecoverable_across_rotation`
  blocker; or **GD-1b** yields a raw `lease_error`, stale-lease limbo, or a silent
  unsealed exit instead of a same-lease renew-and-seal within grace or the typed
  class.

## Adapter survival matrix (F6 — honest, re-grounded on the daemon-observed trigger)

No adapter needs to reload its MCP launch args to seal the in-flight job: the seal is
daemon-side (`resealInFlightJob`), triggered by the **daemon-observed post-rotation
condition** (prompted by the wrapper's exit code or, as backstop, the recovery
sweep) — adapter-independent and not parsed from provider output.

| Adapter | Reseal-in-flight (Slice B) | Resume normal MCP work after rotation |
| --- | --- | --- |
| **Claude** (ephemeral MCP config) | daemon-observed condition → `resealInFlightJob` (no token reload) | #323 ephemeral-config rewrite + endpoint/epoch republish |
| **Agy / pipe** | same daemon-side path | same as Claude where supported |
| **Codex** (MCP URL baked into launch `-c` args; `applyMCPEndpointRotation` can only log + inject a relaunch prompt) | same daemon-side path — **no in-place MCP survival claimed** | operator-assisted relaunch / `supervise rebridge` only |

*Refuting game-day — GD-Codex-Reseal-Rotation:* restart `striatumd` mid-job for a
Codex lane; the in-flight job seals over the daemon-observed path **or** fails legibly
to Option 4, and the spec does **not** claim the Codex MCP client reconnected in
place. *Refuted if* the spec relies on the Codex MCP client reloading baked args, or
the Codex lane silently exits unsealed.

## Scope discipline (Non-Goals held)

- Does **not** re-classify the downstream `agent_exited_unsealed` recovery policy
  (RFC 0152 / D249); the new `session_unrecoverable_across_rotation` blocker is a
  distinct, earlier class.
- Does **not** change committee POSIX-ACL repo provisioning (#537 / #539).
- Does **not** touch `run drive`'s transient-socket behavior (#513).
- Does **not** weaken the #316 boot-epoch recycled-port defense (BC1/F7 strengthen it
  on the supervised path).
- Does **not** introduce any lane-readable credential file (the v1 `0600` reseal file
  stays retired by the maintainer pin).
- Does **not** collide with the RFC 0125 `HandleRecoveryReseal` worktree-durability
  verb (separate file, separate verb, unrelated to credentials).
- Local-first, single-host, daemon-owned PostgreSQL as the single writer.

## Maintainer ratification gate (required)

**Slice B introduces a new daemon-internal capability marker
(`rpc.CapabilityReseal`), a test-only auth-prelude route alternate, the inherited-fd
supervisor control channel with per-message peer-credential authentication, the
reserved agentloop exit codes, the `jobs.recovery_generation` + `leases.reseal_grace_extended_at`
owner-bundle-0021 columns, and endpoint/epoch republish plumbing — a security/authz
trust-model change.** This cleared spec is a **RECOMMENDATION the maintainer ratifies
before any build slice touches credential code.** Slice A (the Option-4 typed-exit-code
floor) is zero-trust-change and may land first under the normal review gate **now that
BC1 routes it over a real, non-PTY channel with the same-uid authentication fixed
(W1–W3).** Adjudicator clearance gates the spec's **soundness**; it is not the
maintainer's product call on the credential code. (The maintainer has already ratified
the OQ1 shape and the F2 non-bearer decision in `SEED.md`; this gate governs the build
slice that writes the code.)

---
<sub>Holder revised proposal (design-v4) for the RFC 0143 falsification-gate design
run. Resolves the two remaining binding constraints — BC1 (the same-uid channel
authentication via W1 per-message `SCM_CREDENTIALS` peer-cred + W2 `PR_SET_DUMPABLE(0)`
+ W3 nonce-out-of-env, the C2 reserved-exit commitment, and the daemon-observed
positive-intent source with backend-gate bypass) and BC5 (the `leases.reseal_grace_extended_at`
pin to owner bundle 0021 and the corrected `work.complete` lock-order gate map) — and
carries the v3-credited set (BC2, BC3, BC4 + F2, F4, the F7 file-mirror half, AF1, AF4,
no-widening, A1–A18) forward unregressed. The adjudicator's collaboration ledger — not
falsifier completion — decides whether this gate clears.</sub>
