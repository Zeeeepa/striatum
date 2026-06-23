# HOLDER — RFC 0143 falsifiable implementation spec (design-v6 REVISION)

author: holder-author-002

> This is the **sixth** falsification pass on RFC 0143 (*lane credential survival
> across a daemon boot-epoch rotation*) and a **proper revision**. v1
> (`rfc-0143-design`) returned `needs_revision` with seven findings F1–F7. v2
> resolved **F2/F4** and distilled the residue into five binding constraints BC1–BC5.
> v3 resolved **BC2, BC3, BC4** and carried the v2-credited set forward unregressed.
> v4 resolved **BC5, two of BC1's three sub-grounds (C2 + the daemon-observed
> positive-intent source with the backend-gate bypass)**, but fell on **BC1-CHANNEL**
> (the W1/W2/W3 walls were specified for a DIRECT `exec.Cmd.ExtraFiles` child exec
> while the production lane is TMUX-BACKED). v5 **resolved BC1-CHANNEL** by deleting
> the inherited-fd channel and adopting a **CONNECT-OUT topology** (the pane wrapper
> dials OUT after `PR_SET_DUMPABLE(0)`; no fd crosses the tmux client/server boundary;
> non-secret listener address; post-auth nonce), flagged and fixed a real v4
> exit-code-on-tmux drift (the `#{pane_dead_status}` backstop), and carried the
> v4-credited set forward unregressed. Both v5 falsifiers credited the topology, the
> named plumbing sites, the W2 ordering, and the real-path test shape — but v5 fell on
> a single, sharply-named ground that **BOTH falsifiers landed INDEPENDENTLY**:
> **BC1-W1-TOKEN — W1's peer-credential proof compares two CATEGORICALLY DIFFERENT
> CLOCKS** (a kernel `/proc` field-22 start-tick on the peer side against a tmux
> `#{pane_start_time}` wall-clock timestamp on the captured side). **BC1-W1-TOKEN is
> the LAST open BC1 ground.**
>
> This v6 spec starts from the **v5** `HOLDER.md` (required context) and the **v5**
> collaboration ledger (the BC1-W1-TOKEN finding + the exact "next revision must…"
> list). It **resolves BC1-W1-TOKEN in ONE place** by pinning a **single coherent
> KERNEL start-token source** for the W1 peer-credential check, and **carries the
> entire v5-credited resolved set forward UNREGRESSED**. It does **not** relitigate
> the ratified OQ1 trust-model shape, the F2 non-bearer decision, **the connect-out
> topology, or the W1/W2/W3 wall *shapes* and W1's load-bearing role** (all pinned in
> `SEED.md`). The connect-out topology and the wall shapes are correct; v6 fixes only
> the **SOURCE of W1's start-token operand**. Every source citation below was
> re-verified against the current worktree while authoring this revision; **drift is
> flagged inline in §Source re-verification.**

## Root reframe (held, unchanged)

**A boot-epoch rotation must never force a lane to choose between reading the
daemon's full-authority bootstrap admin `client-token` and exiting silently
unsealed.** A `striatum-lane` lane authenticates as *its own* narrow,
session-scoped credential and **never** as the shared operator admin override. v6
either lets the lane's in-flight work be sealed over a **daemon-projected,
session-tied authority that no lane bearer carries**, or makes the failure **loud,
typed, and routed** — never silent, never via the admin token.

## What v6 changes vs v5 (ONE place)

v5 was credited on the connect-out topology, the named plumbing sites, the W2
ordering, the non-secret address + post-auth nonce (W3), the `#{pane_dead_status}`
backstop + C2, and the entire carry-forward set (BC2/BC3/BC4/BC5, the daemon-observed
positive intent, the backend-gate bypass, the W1/W2/W3 wall *shapes*, F2, F4, the F7
file-mirror half, AF1, AF4, the no-admin-token-widening invariant, A1–A18). It fell on
exactly one ground inside W1 — **the START-TOKEN OPERAND SOURCE**:

- The v5 W1 check accepts a connecting peer iff `peer.uid == RunAsUser uid`,
  `peer.pid == result.PID` (`identity.PanePID`), and
  `ProcessStartToken(peer.pid) == identity.PaneStartToken`.
- `ProcessStartToken(peer.pid)` is the Linux `/proc/<pid>/stat` **field-22
  start-TICK-since-boot** (`process_identity_linux.go:13-32`).
- But `identity.PaneStartToken` is sourced from tmux `#{pane_start_time}`:
  `CaptureTmuxIdentity` sets `PaneStartToken = verifiedStartToken(parts[3])`
  (`parts[3]` = `#{pane_start_time}`) **whenever tmux returns a numeric value**
  (`tmux_liveness.go:194-197`), falling back to `ProcessStartToken(panePID)` **only**
  when the tmux value is empty/non-numeric (`:198-202`); `verifiedStartToken`
  (`:429-438`) merely checks the value parses as an unsigned integer.
- tmux `#{pane_start_time}` is a **WALL-CLOCK unix timestamp** (seconds since the
  epoch — the existing `TestProbeTmuxLivenessOK` treats `1748452211` as a valid
  `PaneStartToken`); `/proc` field 22 is **start-ticks-since-boot** (at `CLK_TCK` Hz).
  These are **categorically different domains, not merely different formats.**

So on the **PRODUCTION tmux path** (tmux returns numeric) the v5 W1 check compares a
kernel start-tick (left) against a tmux wall-clock timestamp (right). The consequence
is security-load-bearing **two ways**: **either** the legitimate pane wrapper is
**REJECTED** before `resealInFlightJob` ever takes the run lock (so the claimed
*primary* connect-out entry point does not work and the design silently leans on the
`#{pane_dead_status}` and recovery-sweep backstops while *claiming* a working
primary); **or** the build is pressured to **DROP/weaken the pid-reuse guard**,
reopening the same-uid replay surface BC1 exists to close.

**v6 pins ONE coherent KERNEL start-token source for W1.** The decisive observation:
the daemon already learns the pane wrapper's pid at launch (`CaptureTmuxIdentity`
reports `identity.PanePID`), and the kernel start-token read path
(`ProcessStartToken`) **already works cross-uid on the production non-dumpable pane**
— the codebase reads exactly that field-22 token for the pane wrapper today, both as
the `CaptureTmuxIdentity` fallback (`tmux_liveness.go:199`) and in the liveness probe
`PIDLiveWithStartToken` (`:392-408`). So v6:

1. **Captures a NAMED kernel start token** `paneKernelStartToken, ok :=
   ProcessStartToken(identity.PanePID)` **immediately after** `CaptureTmuxIdentity`
   reports the pane pid in `launchPTY` (`pty.go:493-504`) and **before any control
   connection is accepted**, threading it onto `LaunchResult.PaneKernelStartToken`
   (new field, `pty.go:47-53`).
2. **Compares kernel field-22 to kernel field-22** in W1: the accepted peer's
   `ProcessStartToken(peer.pid)` against the **captured kernel token**
   (`result.PaneKernelStartToken`) — one clock domain — by feeding the existing
   `PIDLiveWithStartToken(peer.pid, result.PaneKernelStartToken)` the **right
   operand** (v5 already cited `PIDLiveWithStartToken` but fed it the tmux-sourced
   token; v6 makes both operands field-22).
3. **Keeps tmux `#{pane_start_time}` ONLY as liveness metadata.** `identity.PaneStartToken`
   stays the tmux value (its existing liveness-probe role at `tmux_liveness.go:247-249`
   is untouched); it is **never** the W1 operand. v6 **does not** claim
   `#{pane_start_time}` equivalent to `/proc` field 22 — they are different domains —
   so it is excluded from W1 by construction.
4. **Fails closed on an empty/unreadable captured kernel token** — see the
   fail-closed subtlety below; W1 never degrades to a pid-only check.

This is the only material change. Everything else in the v5 spec is reproduced below
in substance and carried forward; the falsifiers should re-attack the **W1 kernel
start-token source** (and confirm no carried item regressed).

### The fail-closed empty-token subtlety (new — preempts the obvious re-attack)

`PIDLiveWithStartToken(pid, expectedStart)` (`tmux_liveness.go:392-408`) **skips the
start-token comparison entirely when `expectedStart == ""`** (`:397` —
`if expectedStart != ""`), returning *live* on **pid alone**. So if the captured
kernel token were empty (capture failed — pane already gone, `/proc` unreadable, a
non-Linux host) and W1 passed it straight through, W1 would silently degrade to a
**pid-only** check — exactly the weakened pid-reuse guard the constraint forbids.

**v6 requires W1 to refuse fail-closed when the captured kernel token is empty.**
`RunHelper` asserts `result.PaneKernelStartToken != ""` **before** it begins accepting
control connections; an empty captured token means the launch-time kernel identity
could not be bound, so **no control connection is ever accepted** for that launch and
the floor is reached via the `#{pane_dead_status}`/recovery-sweep backstops (the
typed `session_unrecoverable_across_rotation` class), never a pid-only accept. The W1
accept predicate is `peer.uid == RunAsUser uid` **AND** `peer.pid == result.PID`
**AND** `result.PaneKernelStartToken != ""` **AND** `PIDLiveWithStartToken(peer.pid,
result.PaneKernelStartToken)` returns live-with-matching-token. *Falsifier:* A3''.

### Why capturing IMMEDIATELY after launch is load-bearing

The capture binds the **launch-time** kernel identity of the actual pane wrapper. A
kernel start token captured *late* (e.g. lazily at first connect) could bind a pid
that had already died and been **reused** by a sibling between launch and capture —
defeating the very pid-reuse guard W1 exists to be. Capturing
`ProcessStartToken(identity.PanePID)` in `launchPTY` microseconds after the tmux
server reports the pane pid, before `attachTmuxPTY` returns and long before
`acceptControlChannel` accepts a connection, binds the born wrapper. *Falsifier:* A3'
asserts the captured token is taken at launch, not at accept.

## Source re-verification (every BC1-W1-TOKEN site CONFIRMED against current main; drift FLAGGED)

| Claim | Site | Status |
| --- | --- | --- |
| `ProcessStartToken` = `/proc/<pid>/stat` **field 22** (`const starttimeIndex = 22 - 3`; returns `fields[starttimeIndex]`) | `process_identity_linux.go:13-32` | **CONFIRMED** — ⚠️ *minor drift:* SEED/v5 cited `:11-32` and `:12-17`; the doc comment is `:11-12`, the func body is `:13-32`, the field-22 read is `:26-31`. Same code. |
| `CaptureTmuxIdentity` sources `PaneStartToken` from tmux `#{pane_start_time}` (`parts[3]`) whenever numeric; kernel fallback only when empty/non-numeric | `tmux_liveness.go:181-209` (`:182` query, `:194-197` tmux value, `:198-202` kernel fallback, `:203-208` return) | **CONFIRMED** verbatim — this is the defect operand source |
| `verifiedStartToken` merely checks the value parses as an unsigned integer (does NOT convert tmux wall-clock → `/proc` field 22) | `tmux_liveness.go:429-438` | **CONFIRMED** — ⚠️ *minor drift:* SEED cited `:429-436`; the func spans `:429-438` (`ParseUint` check at `:434-436`). Same code. |
| `PIDLiveWithStartToken(pid, expectedStart)` compares `ProcessStartToken(pid)` (field 22) to `expectedStart`; **skips the comparison when `expectedStart == ""`** (returns live on pid alone) | `tmux_liveness.go:392-408` (skip branch at `:397`) | **CONFIRMED** — the field-22-vs-field-22 primitive the fix reuses, **and** the empty-token fail-open trap the fix must close |
| Daemon already reads the pane wrapper's field-22 token cross-uid (CaptureTmuxIdentity kernel fallback + liveness probe) | `tmux_liveness.go:199`, `:392-408` | **CONFIRMED** — the W1 kernel-token read path is already exercised on the production non-dumpable pane (see §W1 read-permission) |
| `LaunchResult.PID` is the pane wrapper pid (`identity.PanePID`); `attachTmuxPTY` builds the result | `pty.go:47-53` (struct), `:507`/`:527-533` (`PID: identity.PanePID`) | **CONFIRMED** — the capture-site anchor for `PaneKernelStartToken` |
| `CaptureTmuxIdentity` called in `launchPTY`, identity validated (`PanePID > 0`) before `attachTmuxPTY` | `pty.go:493`, `:494-504`, `:507` | **CONFIRMED** — capture `ProcessStartToken(identity.PanePID)` here, before any connection is accepted |
| `LaunchSpec` has no control field (v5 adds `ControlSocketAddr`) | `pty.go:30-42` | **CONFIRMED** (fields: Command, Env, EnvFilePath, WorkingDir, RunAsUser, StdinPipePath, StdoutPath, StderrPath, UsePTY, RequireTmux, Extra) |
| ProbeTmuxLiveness queries `#{pane_dead}` only — **NOT** `#{pane_dead_status}` (v5 adds it as the exit backstop) | `tmux_liveness.go:228` | **CONFIRMED** — the v5 `#{pane_dead_status}` backstop addition still applies; carried forward |
| `LatestOwnerBundleVersion` currently **20** (so the 20→21 bump still lands; `RequiredOwnerBundleVersion = LatestOwnerBundleVersion`, `owner.go:35`, moves with it) | `owner.go:23` | **CONFIRMED 20** |

All other v5 BC1-CHANNEL / BC2 / BC3 / BC4 / BC5 / carry-forward citations were
credited as **not regressed** by both v5 falsifiers and the v5 adjudicator and are
reproduced below in substance; they are unchanged by v6, which touches only the W1
start-token operand. The `helper_protocol.go:27-39` / `helper.go:128,:149-156` /
`pty.go:24,:98-113,:282-289,:479,:517-533` / `supervision.go` / `claim.go` /
`write_scope_guard.go` / `artifact_source_publish.go` anchors stand as v5 verified
them.

## Ratified design shape (pinned — built on, not relitigated)

- **OQ1 (ratified):** Slice A = Option 4 (mandatory, zero-trust-change, lands first)
  + Slice B (ratification-gated) = Option 2's narrow `CapabilityReseal` over a
  daemon-owned session-tied path + minimal Option 3 per-session endpoint+epoch
  republish. No lane-readable reseal bearer file under any option.
- **F2 (decided):** non-bearer, daemon-owned, session-tied channel; **no readable
  reseal token file at all** (every lane shares the `striatum-lane` uid, so any
  `0600` file is a same-uid replay surface). Not reopened. v5 closed the channel's
  INSTALLATION on the real tmux launch path structurally via the connect-out topology;
  **v6 closes the channel's AUTHENTICATION primitive (W1) by pinning one coherent
  kernel start-token source.**
- **Connect-out topology + W1/W2/W3 wall SHAPES (ratified by the v5 gate).** The
  connect-out channel and the three structural walls are the right walls and are
  carried forward as the channel's authentication design. **Not relitigated.** The
  ONLY open question v6 resolves is the SOURCE of W1's start-token operand
  (BC1-W1-TOKEN).
- **Slice B requires maintainer ratification** before any build slice touches
  credential code. Adjudicator clearance gates the spec's *soundness*, not the
  maintainer's product call. Slice A is zero-trust-change and may land first **once
  BC1-W1-TOKEN anchors W1 on one coherent kernel identity token** — which v6 does.

## Architectural facts re-anchored (AF1–AF4 — carried forward unregressed)

- **AF1 — reachability, not reminting.** `mintSessionBoundToken`
  (`go/pkg/mutations/session_token.go`) inserts the client row + per-capability grants
  into daemon-owned PostgreSQL bound to `session_id`, 24h TTL. **PostgreSQL survives a
  `striatumd` restart** (D094 / RFC 0043). After a boot-epoch rotation the token is
  still *valid* — only *unreachable* (it lives as the `STRIATUM_MCP_TOKEN` env literal,
  step 1; the post-rotation re-readers skip step 1). The fix is routing, not
  re-minting. *Falsifier:* `TestTokenValidAcrossRestart` (A17).
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

## Carried forward from v5, unregressed (do NOT reopen)

| Item | Status | Anchor / test kept |
| --- | --- | --- |
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
| **W1/W2/W3 wall SHAPES** | shape resolved; **W1 start-token SOURCE = this revision** | §BC1-W1-TOKEN + `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` |
| **F2** — no lane-readable reseal bearer | resolved | `TestBorrowedResealBearerCannotSealVictimSession` |
| **F4** — route-alternate records `reseal` not `write` on only the 3 routes | resolved | `TestResealTokenCanReachOnlyResealRoutesWithoutWrite`; command-authority-matrix reseal column |
| **F7 file-mirror half** — daemon-owned lane-read-only `0644` endpoint/epoch, `O_NOFOLLOW`, atomic rename, reject MISSING epoch header (closes #316) | resolved | `TestResealEpochMirrorRejectsTamperOrMissingEpoch` |
| **AF1 / AF4 / no-admin-token-widening invariant** | kept / held + strengthened | `TestTokenValidAcrossRestart` / `TestResolveRefusesRuntimeClientTokenForLane` |
| **Per-claim falsifiable-assertion discipline** | extended to the W1 single-kernel-token contract | A1–A18 + A3'/A3''/A4'/A7' below |

The carried-forward sections (the connect-out channel topology, the exit-code
backstop, BC2, BC3, BC4, BC5, the daemon-observed trigger, the backend-gate bypass)
are reproduced below in substance from v5; only **§BC1-W1-TOKEN** (the W1 start-token
source of truth + the fail-closed empty-token rule) is changed.

---

# Security cluster (BC1-W1-TOKEN + the carried connect-out channel + BC2 + BC3)

## BC1-W1-TOKEN — ONE coherent KERNEL start-token source for the W1 peer-credential check (the v6 closure)

This is the **one place** the v6 fix lands. It resolves the last open BC1 ground while
preserving the connect-out topology and the W1/W2/W3 wall shapes verbatim.

### The W1 accept predicate (corrected operand)

When a peer connects to the daemon-held `SO_PASSCRED` `SOCK_SEQPACKET` listener, the
helper reads `SO_PEERCRED` on the **accepted** connection — returning the **connecting
peer's** real `{pid, uid, gid}` at connect time. The helper accepts the connection
**iff ALL hold**:

1. `peer.uid == RunAsUser uid` (the lane user), AND
2. `peer.pid == result.PID` — the launched **pane** pid (`identity.PanePID`,
   `pty.go:527-528`), AND
3. **`result.PaneKernelStartToken != ""`** — the launch-time kernel start token was
   captured (fail-closed if empty; see below), AND
4. `PIDLiveWithStartToken(peer.pid, result.PaneKernelStartToken)`
   (`tmux_liveness.go:392-408`) returns **live with matching token** — i.e. the peer
   pid is signalable **and** `ProcessStartToken(peer.pid)` (`/proc` field 22) **equals
   the captured kernel token** (`/proc` field 22). **One clock domain on both sides.**

The helper accepts the **first** connection whose peer-cred matches and binds the
channel to it; every later/other connection is **refused**. A same-uid sibling that
connects has a **different pid** → refused at (2); a reused pid carrying a different
process has a **different field-22 token** → refused at (4). **This is the structural
no-replay property, and it now holds on the REAL channel with W1 specified as ONE
coherent kernel identity token.**

### The named kernel start-token CAPTURE (the operand source of truth)

| Site | v5 | v6 change |
| --- | --- | --- |
| `LaunchResult` (`pty.go:47-53`) | carries `PID = identity.PanePID` | **add `PaneKernelStartToken string`** — the launch-time `/proc` field-22 token of the pane wrapper |
| `launchPTY` (`pty.go:493-504`, after `CaptureTmuxIdentity` validates `identity.PanePID > 0`, before `attachTmuxPTY`) | nothing | **`paneKernelStartToken, ok := ProcessStartToken(identity.PanePID)`** captured here, then stamped onto the `LaunchResult` returned by `attachTmuxPTY` (`:507`/`:527-533`). This is IMMEDIATELY after the pane pid is reported and BEFORE any control connection is accepted. |
| `RunHelper` (`helper.go:128,:149-156`) | W1 compares `ProcessStartToken(peer.pid)` to `identity.PaneStartToken` (tmux-sourced — the defect) | **W1 compares `ProcessStartToken(peer.pid)` to `result.PaneKernelStartToken`** via `PIDLiveWithStartToken`; asserts `result.PaneKernelStartToken != ""` before accepting any connection (fail-closed) |
| `CaptureTmuxIdentity` (`tmux_liveness.go:181-209`) | sets `PaneStartToken = #{pane_start_time}` when numeric | **UNCHANGED** — `identity.PaneStartToken` stays the tmux value, used **only** for liveness (`ProbeTmuxLiveness`, `tmux_liveness.go:247-249`), **never** for W1 |

**`#{pane_start_time}` is kept only as liveness metadata.** v6 does **NOT** prove tmux
`#{pane_start_time}` equivalent to `/proc` field 22 — it is a wall-clock unix timestamp
(seconds since the epoch), categorically distinct from start-ticks-since-boot — so it
is excluded from W1 by construction. `identity.PaneStartToken` retains its existing
liveness role (the `ProbeTmuxLiveness` `observedStart` comparison) unchanged; the W1
operand is the separately-captured kernel token. The two never cross.

### Fail-closed on an empty/unreadable captured kernel token (no pid-only degrade)

Because `PIDLiveWithStartToken(pid, "")` **skips** the start-token comparison
(`tmux_liveness.go:397`) and returns live on **pid alone**, an empty captured kernel
token must **never** be passed into W1 as the expected operand. v6 requires:

- `RunHelper` asserts `result.PaneKernelStartToken != ""` **before** it begins
  accepting control connections. An empty token means the launch-time kernel identity
  could not be bound — `ProcessStartToken(identity.PanePID)` returned `ok == false`
  (pane already gone, `/proc` unreadable, or a non-Linux host where the stub returns
  empty). In that case **no control connection is ever accepted** for that launch.
- The floor is still reached: the post-rotation reseal is driven by the
  `#{pane_dead_status}` exit-code backstop and, finally, the recovery sweep (both
  evaluate the same daemon-observed condition under the same lock, §BC5) → the typed
  `session_unrecoverable_across_rotation` class, never a silent unsealed exit and never
  a pid-only accept.

*Falsifier:* **A3''** — `TestControlFrameRejectsEmptyCapturedKernelStartToken` (unit)
and the real-path negative below: with an empty/unreadable captured kernel token, W1
must refuse every connection (no pid-only accept), and the floor must route via the
backstop.

### W1 read-permission under W2 (why the daemon CAN read field 22 of the non-dumpable pane)

A natural re-attack: does W2's `PR_SET_DUMPABLE(0)` (plus the `sudo` setuid launch
resetting `dumpable`) block the daemon (uid e.g. `halbritt`) from reading
`/proc/<pane-pid>/stat` field 22, since `/proc/<pane-pid>/` becomes root-owned? **No,
and the codebase already proves it:**

- W2 protects the **secrecy** surface — `/proc/<pid>/{environ,mem,fd,maps}` — whose
  inode access is gated by `ptrace_may_access`. **Field 22 (`starttime`) is part of the
  world-readable `/proc/<pid>/stat` (mode `0444`)** and is **not** among the fields
  masked for a non-ptrace-authorized reader. It is a **non-secret identity
  discriminator**, exactly the right primitive for a no-replay binding — not a secret
  W2 must hide.
- **Decisively (empirical, source-verifiable):** the daemon **already** reads the pane
  wrapper's field-22 token cross-uid today — as the `CaptureTmuxIdentity` kernel
  fallback (`tmux_liveness.go:199`, `ProcessStartToken(panePID)`) **and** in the
  liveness probe `PIDLiveWithStartToken` (`:392-408`), which the supervisor runs
  against the production non-dumpable pane on every liveness poll. The W1 kernel-token
  capture/compare reuses that **already-working** read path — it introduces no new
  permission assumption.

So W1 (field-22 identity) and W2 (secrecy of environ/fd/mem) are **consistent**: W2
hides the secrets; field-22 is not one of them; W1 reads field-22 over a path the
daemon already exercises. *Falsifier:* A3' asserts the real-path test reads
`/proc/<peer-pid>/stat` field 22 cross-uid against the live non-dumpable pane.

### BC1-W1-TOKEN — the real-path test (the load-bearing assertion)

`TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` (name retained for continuity;
mechanism is connect-out, not fd-inheritance). Launched through `RunHelper` with
`RequireTmux` + `RunAsUser` (a host integration/game-day test gated on `sudo` +
`tmux`, build-tagged like `tmux_liveness_integration_test.go`), it asserts
**together**:

1. **W1 accept, field-22 on BOTH sides.** The launched pane wrapper connects out and
   sends a frame the daemon **accepts**, where the accept compares
   `/proc/<peer-pid>/stat` field 22 (`ProcessStartToken(peer.pid)`) to the **captured**
   `/proc/<pane-pid>/stat` field 22 (`result.PaneKernelStartToken`, taken at launch) —
   not against tmux `#{pane_start_time}` — emitting a
   `reseal_requested`/`unrecoverable_across_rotation` `HelperControlEvent`.
2. **NEGATIVE — same pid, mismatched/stale kernel start token (the pid-reuse guard).**
   A connection presenting `peer.pid == result.PID` but whose
   `ProcessStartToken(peer.pid)` ≠ the captured kernel token is **REFUSED** (W1's
   field-22 mismatch branch, `PIDLivenessIdentityMismatch`). Constructed by stubbing
   the captured token to a known-stale value (or faking the peer field-22), this proves
   W1 rejects pid reuse rather than accepting on pid alone.
3. **NEGATIVE — empty captured kernel token (fail-closed, A3'').** With
   `result.PaneKernelStartToken == ""`, W1 refuses **every** connection (no pid-only
   accept); the floor routes via the `#{pane_dead_status}`/recovery-sweep backstop.
4. **Provider isolation.** The provider child cannot drive a control event — it is not
   the wrapper pid (W1 refuses it at the pid check), and it has **no inherited fd**
   (there is none).
5. **Sibling refusal (W1) anywhere in the launch chain.** A non-child/non-wrapper
   same-uid sibling that connects to the same `STRIATUM_SUPERVISOR_CONTROL_ADDR` is
   **refused** (wrong pid, and a different field-22 token even on pid reuse), cannot
   recover the nonce (`/proc/<wrapper-pid>/environ` root-owned under W2; nonce never in
   env), and has nothing in `/proc/<wrapper-pid>/fd` to steal.

The direct-`os/exec` units `TestControlFrameRequiresExpectedWrapperPeerCredentials` /
`TestSiblingSameUIDCannotOpenControlFDOrReplayNonceViaProc` (carried from v5) are kept
as fast coverage of the peer-cred + dumpability logic and are updated so the expected
operand is the captured kernel token (field-22), but they are **necessary, not
sufficient** — the real-path test above is the one that fires against the production
tmux/sudo/env-file path with field-22 on both sides.

### The carried connect-out channel (verbatim in substance from v5 — unregressed)

The W1 token source change above sits inside the v5 channel, which is preserved:

- **Listener creation (daemon uid, at supervise start).** The helper creates a
  `socket(AF_UNIX, SOCK_SEQPACKET, 0)` listener with `SO_PASSCRED`, bound to a
  **per-launch abstract address** `@striatum-supervisor-ctl/<supervisor_id>/<random>`
  (abstract → no filesystem node, auto-cleaned on close). The helper holds the listener
  and runs an `acceptControlChannel` reader goroutine alongside `pumpPTYProgress` /
  `forwardPacketStream` (`helper.go:200-208`). `SOCK_SEQPACKET` preserves message
  boundaries (one frame per datagram).
- **Address delivery is NON-SECRET, over existing env plumbing.** The address is
  advertised as `STRIATUM_SUPERVISOR_CONTROL_ADDR` via the existing env path that
  reaches the pane (`tmuxEnvArgs` `-e KEY=VAL`, `pty.go:436`/`:480`, + the env-file). It
  is **not** a `sensitiveEnvKey` (`pty.go:140-155`). The address is non-secret by
  construction — W1 authenticates the *peer*, not knowledge of the address.
- **W2 — `PR_SET_DUMPABLE(0)` first.** The pane wrapper calls `prctl(PR_SET_DUMPABLE,
  0)` (new `go/pkg/agentloop/dumpable_linux.go`, non-Linux stub mirroring
  `process_identity_other.go`) as the **first instruction** of the agentloop
  entrypoint — before it reads the address, before it dials, before any nonce exists.
  Reinforced by the `sudo` setuid `execve` resetting `dumpable` to
  `/proc/sys/fs/suid_dumpable` (default `0`). Two reasons the W2 window is closed: there
  is **no inherited control fd at all** (the env-file shim execs the agentloop binary;
  nothing handed down to steal), and `dumpable=0` makes `/proc/<wrapper-pid>/{fd,environ,mem}`
  root-owned. The connection is dialed **after** `dumpable=0`, by the wrapper itself.
- **W3 — nonce delivered daemon→wrapper, AFTER auth.** On a matched connection the
  helper sends a single-use `control_nonce` (per launch, per generation) **down** the
  authenticated connection. The wrapper echoes it on every subsequent frame; the helper
  rejects any frame whose nonce ≠ the issued nonce, and the BC4 generation guard refuses
  a nonce from a prior generation. The nonce never appears in env or on disk. **W1
  (peer-cred), not the nonce, is the primary and sufficient authentication;** the nonce
  is generation-binding + defense-in-depth.
- **Frame schema.** One `SupervisorControlFrame` per datagram: `{ schema_version:
  "striatum.supervisor_control.v1", type: "reseal_requested" |
  "unrecoverable_across_rotation", supervisor_id, control_nonce }`. It carries **NO**
  job_id / artifact path / kind / body — identity is derived from daemon state (BC2).
  The PTY (provider stdout/stderr) reaches **only** the volume meter (`pumpPTYProgress`,
  D028); `acceptControlChannel` reads frames **only** off the authenticated connection;
  `superviseReportEventTypes` (`supervision.go:19-28`) admits **no** content/output
  event.

**Structural no-replay (the spine, on the REAL channel).** A sibling `striatum-lane`
process that is neither the provider child nor the launched pane wrapper cannot: (W2)
read `/proc/<wrapper-pid>/{environ,fd}` (root-owned; no inherited fd anyway); (W3)
observe the nonce (delivered daemon→wrapper post-auth, never in env); **and decisively
(W1) be accepted by the daemon** — `SO_PEERCRED` stamps the sibling's real pid (≠ the
captured pane pid), and even on pid reuse the `/proc` field-22 token differs from the
**launch-time captured kernel token**. Grant the sibling the address *and*
(hypothetically) the nonce, W1 still refuses it. No-replay holds **structurally on the
production tmux channel**, with W1 specified as ONE coherent kernel identity token.

### Reserved exit codes: PRIMARY signal is the authenticated frame; the exit code is a tmux-observed BACKSTOP (carried, unchanged)

- **Primary (post-rotation prompt / floor):** the pane wrapper sends the typed
  `SupervisorControlFrame` over the **already-authenticated connect-out channel** (now
  authenticated with the coherent kernel token) before exiting; `acceptControlChannel`
  emits the `HelperControlEvent`; the daemon evaluates the daemon-observed reseal
  condition. Structurally same-uid-safe (W1).
- **Backstop (the wrapper can't even send a frame):** two reserved agentloop exit-code
  constants (new, `exitcodes.go`): `ExitUnrecoverableAcrossRotation = 97` (the Option-4
  floor) and `ExitResealInFlightRequested = 98` (a latency hint only; forgeability
  immaterial — the daemon never seals on the strength of 98). On the tmux path these are
  observed via the new `#{pane_dead_status}` capture (NOT `result.Cmd.Wait()`, which is
  the attach client) and routed through the existing `agent_exited` branch
  (`supervision.go:298-306`). On a non-tmux/direct launch they still flow through
  `agentExitPayload`→`processExitCode` (`helper.go:427-439`).
- **Final backstop:** the **recovery sweep** evaluates the same daemon-observed
  condition under the same lock (§BC5), so even if neither a frame nor a pane status is
  observed (including the fail-closed empty-token case), a complete-on-disk
  post-rotation deliverable still gets one daemon-observed reseal attempt (or the typed
  floor).

**C2 — reserved codes reserved BY COMMITMENT (carried, unchanged).** Choosing a high
range is **not** an auth boundary. After `cmd.Wait()` (`loop.go:365`) the wrapper
inspects the provider child's exit status and, if it is 97 or 98, **remaps it to a
non-control `agent_exited` outcome** (carrying the provider's raw status in a
non-reserved field). The reserved 97/98 are emitted **only** by the wrapper's own
typed-error path and the authenticated frame, never forwarded from the child.
*Falsifier:* `TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker` (A6).

### BC1 — positive intent is DAEMON-OBSERVED, not provider-asserted (carried, unchanged)

`resealInFlightJob` fires **only** when ALL hold, evaluated under the run advisory lock
(§BC5):

1. **A boot-epoch rotation occurred during this job's lease** (recorded packet epoch vs
   current `writeBootEpochFile` epoch). Absent a rotation, the normal seal path applies.
2. **The job is still `running`, the lease is bound, the stamped generation matches**
   the live `jobs.recovery_generation` (BC4), **and** the lease is within grace (BC5).
   Any mismatch → typed floor.
3. **Every required `expected_artifact` path (from daemon state, BC2) is present in the
   job's active worktree AND was AUTHORED THIS ATTEMPT** — see §modified-since-baseline
   build-test. Absent that, route to the floor, never a speculative seal.
4. `resealInFlightJob` then attempts **only daemon-derived artifacts** (BC2) and maps
   ALL validation/backend/front-matter/durability failures to the typed
   `session_unrecoverable_across_rotation` floor (Option-4). Never a silent seal, never
   a raw error.

**Two entry points, one condition, one backstop:** the wrapper's authenticated
post-rotation frame (or `#{pane_dead_status}` 98) **and** the recovery sweep, which
evaluates the **same** condition under the **same** lock **before** requeuing.

**Backend-gate routing (carried).** `resealInFlightJob` does **not** reuse
`HandleCompleteWork`; it calls the lower-level complete core and **deliberately
bypasses `ensureWorkSessionBackend`** (`lifecycle.go:1181`) — the reseal exists
*precisely because* the live connection is gone — so a stopped supervisor routes the
typed class rather than leaking `invalid_transition`/backend errors. *Falsifier:*
`TestResealExit98BypassesBackendGateOrRoutesTyped` (A9).

### Modified-since-baseline build-test (carried forward — the v5 precision item)

The "deliverable observed" condition (3) must **not** treat "present + absent from
`write_scope_baseline.changed_paths`" as sufficient by itself: for per-job isolated
worktrees the baseline is **nil** (`write_scope_guard.go:81-83`), and source-change
publication already attributes authorship via `gitChangedPathSnapshots`
(`write_scope_guard.go:225`) + `collectInScopeAuthoredPaths`
(`artifact_source_publish.go:259-263`, used at `:88`; `claim.go:622`).
`resealInFlightJob` **reuses that authored-path attribution** so an UNCHANGED
pre-existing expected path is **NOT** resealed. *Falsifier:*
`TestResealRequiresAuthoredExpectedArtifactChange` (seed a clean pre-existing expected
path → typed floor; modify it → positive reseal) or the positive
`TestCodexResealUsesReceiverNotProviderStdout` (A8).

## BC2 — Artifact identity from daemon state, never from output (resolved, carried forward)

`resealInFlightJob` derives the expected-artifact set from **its own state** and
refuses any unexpected path, reusing the existing handler payload contracts verbatim:

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
`expected_artifacts`, reading the body from the job's own worktree, and **refuses any
path not in the expected set**. The signal supplies neither path nor content. A
front-matter/author-line failure (publisher exit code 6) records the
`session_unrecoverable_across_rotation` blocker with the validation error — the
Option-4 floor, never a silent drop. *Test:* `TestCodexResealUsesReceiverNotProviderStdout`
(negative + positive).

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
  `registry_methods.go` is generated, so `ResealAlternate` lands in
  `contracts/daemon_methods.json` + the `MethodEntry` struct (`rpc/registry.go`) + the
  regenerated map + a reseal column in `docs/reference/command-authority-matrix.md` +
  the authority guardrail.

*Test:* `TestResealCapabilityIsDaemonInternalNotBearer` (A1).

---

# Lifecycle cluster (BC4 + BC5)

## BC4 — Concrete monotonic generation column for the split-brain guard (resolved, carried forward)

`jobs` is **owner-held**, so a column-add is owner DDL —
`TestFutureRuntimeMigrationsDoNotCarryOwnerDDL` forbids a runtime migration from
ALTERing it. v6 ships owner bundle **`go/pkg/db/sql/owner/0021_job_recovery_generation.sql`**:
`ALTER TABLE striatumd.jobs ADD COLUMN IF NOT EXISTS recovery_generation integer NOT
NULL DEFAULT 0;`, bumps `LatestOwnerBundleVersion` **20→21** (`owner.go:23` —
**confirmed currently 20**; `RequiredOwnerBundleVersion = LatestOwnerBundleVersion`,
`owner.go:35`, moves with it) with the ordinal-21 `[[owner_bundle]]` reservation in
`RESERVATIONS.toml` (`go/pkg/db/reservations.go`), modelled exactly on the credited
`review_generation` precedent (`owner/0009_review_generation.sql`).

- **Degrade-safe presence probe.** `db.JobRecoveryGenerationColumnPresent` (mirroring
  `SessionPipeReadColumnPresent` / `ArtifactPlacementColumnPresent` /
  `reviewGenerationEnabled`, `db/artifact_write.go:64-102`). Column absent → route to the
  typed floor.
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
  and compares it to the **live** `jobs.recovery_generation` under the lock — equal →
  proceed; unequal → typed class.

*Tests:* `TestResealPredicateUsesStampedRecoveryGeneration` /
`TestResealTokenRefusedAfterLeaseExpiryAndRecoveryRequeue` (A10, A15).

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
(Like `review_generation`, `striatumd_rw`'s table-level grants extend to the new
column; no new grant.) *Falsifier:* `TestRuntimeMigrationsDoNotCarryOwnerDDLForLeasesGraceColumn`
(folded into `TestFutureRuntimeMigrationsDoNotCarryOwnerDDL`) (A14).

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
  tx (`:610-621`). So: sweep-wins → it bumps the generation; reseal then blocks,
  acquires the lock, observes the changed generation/expired-beyond-grace lease, routes
  the typed class (**never revives a requeued lease**). Reseal-wins → seals within grace
  (`running→completed`); the sweep then sees a completed job and does not requeue.
- **Expired-beyond-grace ALWAYS routes the typed class** — `resealInFlightJob` never
  calls `activeLeaseFor`; the reseal predicate returns
  `ErrSessionUnrecoverableAcrossRotation` → the durable blocker. No raw `lease_error`
  ever reaches a post-rotation reseal.

*Tests:* `TestResealBeyondGraceRoutesTypedNotLeaseError`,
`TestResealGraceCannotReviveRequeuedLease`, `TestRecoveryRequeueWinsOverExpiredLeaseReseal`,
`GD-1b`, `TestResealExit98BypassesBackendGateOrRoutesTyped`, and the 0021-migration
guard (A11–A13).

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
W1-accepted with the **captured kernel start token** (fail-closed on empty). The sketch
is also the **recovery-sweep entry point** (same predicate, same lock, before requeuing)
and the **connect-out-frame entry point**. `typedFloor(reason)` records the durable
`session_unrecoverable_across_rotation` blocker (Option-4).
`allRequiredExpectedArtifactsAuthoredThisAttempt` uses the
`gitChangedPathSnapshots`/`collectInScopeAuthoredPaths` authored-path attribution (nil
baseline for isolated worktrees).

## Security invariant (the spine) — held and strengthened

The runtime `client-token` carries the full `bootstrapCapabilities` and is `0600` in a
`0700` dir (AF3). **Any path that lets a lane read that file, or mints a lane-readable
credential carrying any of `{admin, apply, recovery, surgical_recovery}`, is
categorically out of bounds for a FIX** — v6 keeps it structurally impossible:

- The lane never gets OS read of the `0700` dir (AF3); the Slice-A floor removes the
  only code path that would have read the `client-token` (`token.go:31-42` /
  `endpoint.go` return the typed error for a supervised lane).
- The only new authority, `CapabilityReseal`, carries **no elevated verb** and is
  **never materialised into any lane-readable file or bearer** (BC3 + F2).
- The reseal is **projected by the daemon only** on the supervisor-proven path.
- The control channel is a **connect-out authenticated by kernel `SO_PEERCRED` pid +
  a launch-time-captured `/proc` field-22 kernel start token** (W1, one clock domain),
  with the wrapper non-dumpable (W2) and the nonce delivered post-auth (W3) — **no
  bearer, no inherited fd, no secret in env** — and a sibling that connects is refused
  **structurally** on the real tmux path. W1 **fails closed** when the kernel token
  cannot be captured (never a pid-only accept). The exit-97 floor is a
  tmux-`#{pane_dead_status}`-observed backstop, not a forgeable primary.
- The epoch republish moves **endpoint + epoch only** over the daemon-owned,
  integrity-protected path (F7 file-mirror, kept); never the admin token.

*Falsifier:* `TestResolveRefusesRuntimeClientTokenForLane` (A2) —
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
- **A3 — No-replay, STRUCTURAL (direct harness, KERNEL token both sides).** Every
  accepted control frame is bound to the launched wrapper pid + the **captured `/proc`
  field-22 kernel start token**. *Refuted if*
  `TestControlFrameRequiresExpectedWrapperPeerCredentials` accepts a frame from any pid
  other than the launched wrapper's, or whose `ProcessStartToken(peer.pid)` ≠ the
  captured kernel token, or `TestBorrowedResealBearerCannotSealVictimSession` finds an
  on-disk reseal bearer or a sibling/foreign-session/provider-child sealing session A's
  job.
- **A3' — No-replay, STRUCTURAL, on the PRODUCTION tmux channel, field-22 BOTH sides
  (the v6 closure).** *Refuted if* `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper`
  (launched through `RunHelper` with `RequireTmux`/`RunAsUser`) shows: the accept
  comparing `ProcessStartToken(peer.pid)` against tmux `#{pane_start_time}` rather than
  the **launch-time-captured `/proc` field-22 kernel token**; OR a same-uid sibling
  connection **accepted** (wrong pid/start-time should refuse); OR a **same pid with a
  mismatched/stale kernel start token accepted** (the pid-reuse guard must refuse); OR
  the provider child driving a control event; OR the launched pane wrapper's
  authenticated frame **not** accepted; OR the captured token taken at accept rather
  than at launch (`launchPTY:493-504`); OR an inherited control fd present.
- **A3'' — W1 fails closed on an empty/unreadable captured kernel token (no pid-only
  degrade).** *Refuted if* `TestControlFrameRejectsEmptyCapturedKernelStartToken` (and
  the real-path negative) shows W1 accepting a connection when
  `result.PaneKernelStartToken == ""` (i.e. passing `""` into
  `PIDLiveWithStartToken`, degrading to a pid-only check), instead of refusing every
  connection and routing the floor via the `#{pane_dead_status}`/recovery-sweep
  backstop.
- **A4 — `/proc` surface closed (W2/W3).** *Refuted if*
  `TestSiblingSameUIDCannotOpenControlFDOrReplayNonceViaProc` opens
  `/proc/<wrapper-pid>/fd/*` or recovers the nonce from `/proc/<wrapper-pid>/environ` as
  a same-uid non-wrapper process, or the nonce is found in the wrapper env. (Field 22 of
  `/proc/<pid>/stat` is a non-secret identity discriminator, not a secret W2 hides — the
  daemon's cross-uid field-22 read is the already-exercised liveness read path,
  `tmux_liveness.go:199`,`:392-408`.)
- **A4' — W2 ordering on the real path.** *Refuted if* the real-path test shows the
  wrapper readable (`dumpable != 0`) at any point before it connects out, or the nonce
  observable in env at any point in the launch chain (it must be delivered daemon→wrapper
  post-auth only).
- **A5 — Control path never parses output.** *Refuted if*
  `TestPTYOutputCannotEmitSupervisorControlEvent` / `TestProviderOutputCannotDriveResealOrBlocker`
  shows PTY/stdout bytes driving a reseal or blocker.
- **A6 — Reserved exit codes reserved by commitment (C2).** *Refuted if*
  `TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker` shows a provider
  child's 97/98 propagated into the reserved codes.
- **A7 — Floor is a typed signal recorded without parsing.** *Refuted if*
  `TestAgentLoopReservedExitCodeRoutesUnrecoverableWithoutPTYParsing` shows exit 97
  failing to route the durable blocker, or the decision reading output bytes.
- **A7' — Exit-code backstop observed via `#{pane_dead_status}`, not the attach client.**
  *Refuted if* the tmux-path floor reads the pane wrapper's reserved exit from
  `result.Cmd.Wait()` (the attach client) rather than the `#{pane_dead_status}` capture,
  or a pane-emitted 97/98 fails to route on the production path while the attach client
  exits 0.
- **A8 — Positive intent is daemon-observed + authored-this-attempt.** *Refuted if*
  `TestCodexResealUsesReceiverNotProviderStdout` shows a seal driven by provider-asserted
  intent without the daemon-observed condition, OR a path outside `expected_artifacts`
  accepted, OR `TestResealRequiresAuthoredExpectedArtifactChange` reseals an UNCHANGED
  pre-existing expected path.
- **A9 — Backend-gate routing.** *Refuted if*
  `TestResealExit98BypassesBackendGateOrRoutesTyped` leaks `invalid_transition`/backend
  errors instead of sealing via the internal core or routing the typed class.
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
  (the connect-out real-path game-day) shows a sibling sealing, the wrapper unable to
  reach the daemon over the connect-out channel under `RequireTmux`/`RunAsUser`, or the
  W1 accept comparing anything other than `/proc` field 22 on both sides.

## Adapter survival matrix (F6 — honest, re-grounded on the daemon-observed trigger)

No adapter needs to reload its MCP launch args to seal the in-flight job: the seal is
daemon-side (`resealInFlightJob`), triggered by the **daemon-observed post-rotation
condition** (prompted by the authenticated connect-out frame, the `#{pane_dead_status}`
backstop, or the recovery sweep) — adapter-independent and not parsed from provider
output.

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
connect-out `SO_PEERCRED` (pid + launch-time-captured `/proc` field-22 kernel start
token) authentication, the reserved agentloop exit codes, the `jobs.recovery_generation`
+ `leases.reseal_grace_extended_at` owner-bundle-0021 columns, and endpoint/epoch
republish plumbing — a security/authz trust-model change.** This cleared spec is a
**RECOMMENDATION the maintainer ratifies before any build slice touches credential
code.** Slice A (the Option-4 typed floor) is zero-trust-change and may land first under
the normal review gate **now that BC1-W1-TOKEN routes it over the real, non-PTY
connect-out channel whose same-uid authentication (W1) is specified with ONE coherent
kernel identity token** — the captured `/proc` field-22 kernel start token, compared
field-22-to-field-22, fail-closed on an empty token. Adjudicator clearance gates the
spec's **soundness**; it is not the maintainer's product call on the credential code.

---
<sub>Holder revised proposal (design-v6) for the RFC 0143 falsification-gate design run.
Resolves the single remaining binding constraint BC1-W1-TOKEN by pinning ONE coherent
KERNEL start-token source for the connect-out W1 peer-credential check: a named kernel
start token captured via `ProcessStartToken(identity.PanePID)` (`/proc/<pid>/stat`
field 22, `process_identity_linux.go:13-32`) IMMEDIATELY after `CaptureTmuxIdentity`
reports the pane pid (`pty.go:493-504`) and BEFORE any control connection is accepted,
threaded onto `LaunchResult.PaneKernelStartToken` (`pty.go:47-53`), and compared
field-22-to-field-22 against the accepted peer's `ProcessStartToken(peer.pid)` by
feeding the existing `PIDLiveWithStartToken` (`tmux_liveness.go:392-408`) the captured
kernel token — failing closed when the token is empty (no pid-only degrade) and keeping
tmux `#{pane_start_time}` as liveness metadata only (not proven equivalent to `/proc`
field 22). Extends `TestTmuxRunAsControlFDIsDeliveredOnlyToWrapper` to compare `/proc`
field 22 on both sides plus a same-pid stale-token negative and an empty-token
fail-closed negative. Carries the v5-credited set (connect-out topology + named plumbing
sites + non-secret address + post-auth nonce W3 + W2 ordering + `#{pane_dead_status}`
backstop + C2 + BC2/BC3/BC4/BC5 + daemon-observed positive intent + backend-gate bypass
+ W1/W2/W3 shapes + F2/F4/F7-file/AF1/AF4/no-widening/A1–A18) forward unregressed,
folding in the modified-since-baseline authored-path build-test. The adjudicator's
collaboration ledger — not falsifier completion — decides whether this gate clears.</sub>
