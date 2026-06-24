# HOLDER — RFC 0143 **Slice A** falsifiable implementation spec: the decoupled `session_unrecoverable_across_rotation` typed-exit floor

author: holder-author-002

> **Scope.** This is a **fresh** design run for **Slice A only** — the
> maintainer-ratified (**D261**, 2026-06-24) **Option 4** floor: make a
> `striatum-lane` lane that cannot reseal after a daemon boot-epoch rotation **fail
> LEGIBLY** with a typed `session_unrecoverable_across_rotation` signal instead of a
> silent unsealed exit or a misleading "permission denied". Per D261 Slice A is **pure,
> daemon-side observability**: it **mints no credential, widens no token, and does not
> touch the credential trust model.** Slice B (the `CapabilityReseal` authority + the
> connect-out control channel, options 2/3) is **blocked on RFC 0168 (#585)** and is
> **OUT OF SCOPE** — this spec designs **none** of it.
>
> **What this is.** A falsifiable implementation spec the `rfc-0143-slice-a-build`
> `code_change` run executes contract-first (TDD): concrete file:line anchors (re-verified
> against current `main`), named Go tests, and a mechanically-derived classification — not
> a restatement of the RFC. Every load-bearing claim is an assertion (A1–A6) paired with
> its named test. This is the claim the two falsifiers re-attack; the adjudicator's
> collaboration ledger — not falsifier completion — decides whether the gate clears.
>
> **Inheritance from the v7 design record (decoupled re-statement).** The seven-cycle
> `rfc-0143-design-v{1..7}` `falsification_gate` proved the **authenticated reseal
> channel** (options 2/3 — the W1 connect-out channel, the kernel-token capture, the
> `CapabilityReseal` authority) is **unsolvable while every lane shares the
> `striatum-lane` uid** (`BC1-W1-ORACLE`: the production tmux control surface runs as the
> shared uid with a deterministic session name and no private socket, so a same-uid
> sibling can `respawn-pane`-replace the daemon-launched pane and the daemon — whose only
> handle is a post-launch tmux query — authenticates the replacement). That whole channel
> is **Slice B**. The v7 ledger left **`BC1-W1-CAPTURE-FLOOR`** open: *the
> capture-boundary-miss fail-closed path was not lifecycle-wired to the typed floor; a
> pre-attach miss surfaced as a raw `helper_error`*. **§3.5 below dissolves that finding
> in the decoupled world**: Slice A has **no capture boundary at launch** (no W1, no
> kernel-token capture), so the floor is **never** carried by the launch handshake — it is
> observed *only* as a reserved process exit code after `agent_started`. The raw-error-leak
> path is therefore **structurally absent** in Slice A, not merely avoided.

---

## §0 — Auditable resolution map: how this honors the decoupling premise and every HARD CONSTRAINT

The two spots and the floor signal are computed **entirely from daemon-side durable /
process state** with **no dependency on an authenticated inbound frame** from the lane.
Falsifiers can verify each property directly against the mechanism named in the right column.

| Property the gate requires | How Slice A delivers it (concrete mechanism) |
|---|---|
| **No inbound authenticated frame** (decoupling premise, D261) | The floor signal is a **process exit status** (`ExitUnrecoverableAcrossRotation = 97`), observed via the existing `supervisor.agent_exited` event's `exit_code` (direct path, `helper.go:433`→`supervision.go:425`) and via tmux **`#{pane_dead_status}`** (tmux path, extending `ProbeTmuxLiveness`, `tmux_liveness.go:228`). A process exit status is durable observed state, never a frame. Spot 1's *predicate* (a non-owner lane reaching the owner-only admin token) is computed from the local process euid + the file owner uid — no daemon round-trip, no frame. |
| **No token widening** (HARD CONSTRAINT 1; `reject` if violated) | Spot 1 **narrows** the resolution chain: at the runtime `client-token` tier it **refuses** the step for a non-owner and returns a typed sentinel **before any read**. No path adds a read of the admin token; no lane ever obtains it. No minted credential. The reserved exit code carries **no authority** — it is an integer process status. |
| **No new credential / no Slice B** (HARD CONSTRAINT 2) | No `CapabilityReseal`, no connect-out channel, no reseal-token file, no kernel-token capture (W1/W2/W3), **no reserved code 98**, no `resealInFlightJob`, no owner bundle 0021. Slice A owns **only** code `97`. The lane still cannot reseal; it fails legibly and an operator requeue (or Slice B, later) is still required to seal. |
| **Daemon-side / process state only** (HARD CONSTRAINT 3) | Every predicate reads: the local euid vs file-owner uid (Spot 1); the `striatumd.events` `supervisor.agent_exited` payload and the tmux `#{pane_dead_status}` probe (Spot 2); plus the existing `verifyRequiredArtifacts` + `verifyRequiredArtifactReconstructable` + `/proc`/`kill(0)`/tmux liveness already used by recovery. |
| **No over-fire** (HARD CONSTRAINT 4) | The typed class fires **iff** the observed exit code **== 97**. An ordinary unsealed exit (code `0`/`1`, complete-on-disk, no reserved code) stays `agent_exited_unsealed`; a never-engaged early crash stays `agent_pid_dead`; a genuine launch failure stays `helper_error`. (A3.) |
| **No raw-error leak** (HARD CONSTRAINT 4) | In Slice A the floor is **only** a post-`agent_started` exit code, so the launch-handshake path (`waitForHelperAgentStart`, `helper_error` phase `launch`) is **never** a floor carrier — it correctly stays raw for genuine launch failures, which are **not** the floor. No covered miss leaks a raw error because there is no covered miss on that path (§3.5). |
| **Additive-only** (HARD CONSTRAINT 5) | New file `exitcodes.go`; a refusal branch added *ahead of* an existing read; a new derived daemon event + a new stall class; the tmux probe gains one capture field. No existing exit code, helper-event type, stall class, or recovery verb changes meaning. The **one** existing test whose assertion changes is `TestNecrosisDomainMatchesConfirmedDeadConstants` — extended to admit the new necrosis member (an additive domain growth, not a meaning change); §3.4 + A6 call this out explicitly. |
| **Product-boundary-safe** (HARD CONSTRAINT 6 / AGENTS.md) | No hosted service, no durable transcript, no external persistence. All state is the existing daemon PostgreSQL + local process/tmux observation. |
| **Relationship to `agent_exited_unsealed` + `HandleRecoveryCompleteStalled` (#292)** | The typed floor is a **strict refinement** of `agent_exited_unsealed` for the exact-reserved-code case: it routes through the **same** finalize-from-durable-artifact path (it does **not** duplicate or override `HandleRecoveryCompleteStalled`), and when no durable deliverable exists it escalates with a **distinct, legible reason** (§3.6). |

---

## §1 — The reserved agentloop exit code (Slice A owns ONLY this)

**New file `go/pkg/agentloop/exitcodes.go`:**

```go
package agentloop

import "errors"

// ExitUnrecoverableAcrossRotation is the reserved agentloop process exit code for
// the RFC 0143 Slice A floor (D261, Option 4). A supervised lane wrapper exits with
// this code when its credential-resolution chain would fall through to the
// owner-only admin runtime client-token for a NON-OWNER lane — the lock-out a
// daemon boot-epoch rotation produces (#512). The daemon observes the code from
// durable process state (agent_exited.exit_code on the direct path; tmux
// #{pane_dead_status} on the tmux path) and routes the typed
// session_unrecoverable_across_rotation recovery class, instead of a silent unsealed
// exit or a misleading "permission denied".
//
// Slice A owns ONLY this floor code. The reseal-request code (98), resealInFlightJob,
// the connect-out channel, the kernel-token capture, the CapabilityReseal authority,
// and owner bundle 0021 are Slice B (RFC 0168 / #585) — NOT defined here.
const ExitUnrecoverableAcrossRotation = 97

// ErrUnrecoverableAcrossRotation is the typed sentinel the credential resolver
// returns (NEVER a token, NEVER a read of the admin file) when a non-owner lane's
// chain reaches the owner-only admin runtime client-token. Callers map it — and ONLY
// it — to a clean exit with ExitUnrecoverableAcrossRotation. An owner process never
// receives it (its euid matches the file owner; it resolves the token normally).
var ErrUnrecoverableAcrossRotation = errors.New("agentloop: session unrecoverable across daemon boot-epoch rotation (non-owner lane refused the owner-only admin runtime client-token)")
```

Rationale for `97` (carried from the v7 design record, which named
`ExitUnrecoverableAcrossRotation = 97` as "the Option-4 floor … observed via the
`#{pane_dead_status}` capture, never from `result.Cmd.Wait()`"): a value well outside
the 0/1/2 range adapters use for ordinary success/error/usage, and **distinct from any
Slice-B code** (Slice B's reseal-request `98` is **not** introduced here). `97` is a
process status, never a credential.

---

## §2 — Spot 1: credential-chain narrowing (RFC 0143 Option 4 / #512)

### 2.1 The two resolver sites (re-verified against `main`)

- **`ResolveTokenMaterial`** (`go/pkg/agentloop/token.go:18-53`). Step 1 = `STRIATUM_MCP_TOKEN`
  env literal (`:19-21`); step 2 = `STRIATUM_MCP_TOKEN_FILE` (`:23-29`); **step 3 = the
  runtime `client-token`** (`:31-42`, via `EnvDaemonRuntimeDir` or `admin.RuntimeTokenPath()`);
  step 4 = the repo `.striatum/capability_token` (`:44-52`).
- **`ResolveTokenMaterialFresh`** (`go/pkg/agentloop/endpoint.go:110-138`). The #323
  rotation-recovery reader: it deliberately bypasses the env launch literal and re-reads
  `STRIATUM_MCP_TOKEN_FILE` (`:119-124`) then **the runtime `client-token`** (`:125-136`).
- The runtime `client-token` is the daemon's **full-authority bootstrap admin token**:
  `bootstrapCapabilities = {admin, read, write, claim, review, apply, recovery,
  surgical_recovery}` (`go/pkg/admin/bootstrap.go:18-27`), written `0600` in a `0700`
  owner-only runtime dir (`writeRuntimeToken`, `bootstrap.go:134-168`).
- Today `ReadTokenFile` (`token.go:75-92`) rejects any file that is not owner-**mode**
  (`mode&0077 != 0`, `:82`) but does **not** check owner **identity**; for a non-owner
  lane the OS denies the `0600` read with `EACCES` ("permission denied" — the misleading
  dead-end), and the owner-only `0700` runtime dir often blocks traversal first.

### 2.2 The narrowing (the refusal — never a read)

Add an unexported predicate to `token.go`, applied at the runtime-`client-token` tier in
**both** resolvers, **before** the existing `readOptionalTokenFile` call:

```go
// adminTokenReachedByNonOwner reports whether the resolution chain has reached the
// owner-only admin runtime client-token while the current process is NOT its owner —
// the RFC 0143 #512 lock-out. It NEVER reads the token contents. Detection is local
// process state only (no inbound frame):
//   - the file exists and its owner uid != this process euid  -> non-owner (refuse); or
//   - stat is denied with EACCES/EPERM on the file or its 0700 owner-only parent dir
//     (a non-owner lane cannot even traverse the owner runtime dir) -> non-owner (refuse).
// A missing file (ENOENT) is NOT the floor (fall through to the next tier, unchanged).
// When the file owner uid == euid (the OWNER/operator process) it returns false and the
// caller reads the token exactly as today (owner unaffected).
func adminTokenReachedByNonOwner(path string) bool { /* os.Lstat + syscall.Stat_t.Uid vs os.Geteuid(); EACCES/EPERM => true; ENOENT => false */ }
```

Wiring at `token.go:31-42` (and the structurally-identical tier in
`endpoint.go:125-136`): when the chain reaches the runtime `client-token` tier and
`adminTokenReachedByNonOwner(path)` is true, **return `("", ErrUnrecoverableAcrossRotation)`**
instead of calling `readOptionalTokenFile`. Otherwise behavior is byte-for-byte unchanged
(owner reads normally; ENOENT falls through to step 4). This is a **narrowing**: it removes
a step for a non-owner; it adds **no** read path. `ReadTokenFile`'s existing owner-mode
rejection (`:82`) is retained unchanged (A4).

### 2.3 Caller mapping → clean reserved-code exit

- **Startup, direct (`loop.go:37`, `Run`) and in-process harness (`loop.go:78`,
  `RunContext`).** `ResolveTokenMaterial` returns the sentinel → `Run`/`RunContext`
  **return `ErrUnrecoverableAcrossRotation`** (wrapped) without starting the PTY. The
  agent-loop entrypoint maps it:

  **`go/cmd/striatumd/main.go:109-117`** (the `-agent-loop` subcommand) changes from a
  blanket `log.Fatalf` to:
  ```go
  if err := agentloop.Run(socketPath, repoRoot, runID, sessionID, flag.Args()); err != nil {
      if errors.Is(err, agentloop.ErrUnrecoverableAcrossRotation) {
          log.Printf("agent-loop: %v", err)
          os.Exit(agentloop.ExitUnrecoverableAcrossRotation) // 97 — reserved floor
      }
      log.Fatalf("agent-loop failed: %v", err) // unchanged: any other error -> exit 1
  }
  os.Exit(0)
  ```
  This is the **only** place the wrapper emits `97`, and it emits it **only** for the
  sentinel (C2 — §4).

- **#323 rotation watcher (`loop.go:602`, `ResolveTokenMaterialFresh`).** Today the
  watcher silently falls back to the launch token on any error (`loop.go:599-604`) — the
  precise silent-continue that ends in #512's unsealed exit. Slice A makes the lock-out
  legible **without over-firing**: distinguish the sentinel from a transient error.
  - When `errors.Is(terr, agentloop.ErrUnrecoverableAcrossRotation)` **and** the launch
    token is itself the admin runtime client-token (`cfg.Token.Source` is the runtime
    `client-token` path — i.e. the lane was already depending on the admin token a
    non-owner cannot use across the rotation), the watcher invokes a new
    `requestUnrecoverableExit()` callback (the structural twin of the existing
    `requestIdleExit`, `loop.go:328-334`): set an `atomic.Bool`, `SIGTERM` the child.
    `runWithIO` then returns `ErrUnrecoverableAcrossRotation` (mapped to `97` by
    `main.go`), instead of `nil`.
  - Otherwise (sentinel but the launch token is a usable session-bound bearer, or any
    non-sentinel error) the watcher keeps the **existing** launch-token fallback — **no
    behavior change, no over-fire** on a lane that can still work.

  *(Falsifier-1 probe surface, stated openly: the rotation-path exit is gated on
  `cfg.Token.Source == <runtime client-token>` precisely so a lane still holding a usable
  session-bound bearer is never killed. A lane with a working session-bound token never
  reaches step 3 of the fresh resolver, so the sentinel is not even produced for it.)*

### 2.4 Why this is decoupled and non-widening

The predicate reads only the **local process euid** and the **file owner uid** — no daemon
round-trip, no inbound frame, no boot-epoch record (there is **no** durable per-lease epoch
— `daemonBootEpoch()` is in-memory per-process, `main.go:733-738`, validated on MCP HTTP
via `X-Striatum-Boot-Epoch`, `mcp/http.go:681-699`; Slice A does **not** rely on detecting
"a rotation happened" from durable DB state). The causal information lives **here** (the
credential refusal), and Spot 2 routes the **reserved code**. No lane ever reads the admin
token; the owner is unaffected.

---

## §3 — Spot 2: daemon observation + typed-class recovery routing (closes `BC1-W1-CAPTURE-FLOOR`, decoupled)

### 3.1 Observing the reserved code from durable state — two paths

The supervised tmux pane runs the **agent-loop wrapper itself** (`pty.go:432` comment:
"the agent-loop wrapper exits code 1 on the env check"), and `remain-on-exit on` is set
**before** the lane command runs (`pty.go:459`), so a dead pane stays queryable.

- **Direct (plain-PTY) path.** `RunHelper` reaps the child and emits `agent_exited` with
  `exit_code` from `processExitCode(result.Cmd.Wait())` (`helper.go:433`, `agentExitPayload`
  `:427-439`; `processExitCode` `:499-507`). The daemon records it durably: the
  `agent_exited` branch of `recordSuperviseReportEvent` stamps the stop reason
  (`supervision.go:298-306`) and `curatedSuperviseReportPayload` **keeps `exit_code`**
  (`supervision.go:425`). So `97` is already durable on the direct path with **no schema
  change**.
- **Tmux path.** `result.Cmd` is the **attach client**, so `agent_exited.exit_code` is the
  attach client's, **not** the pane wrapper's. The pane wrapper's `97` lives in tmux
  **`#{pane_dead_status}`**. Extend the `display-message` capture in `ProbeTmuxLiveness`
  (`tmux_liveness.go:228`) from
  `#{pane_id}|#{pane_pid}|#{pane_dead}|#{pane_start_time}` to additionally request
  `#{pane_dead_status}`, parse it into a new `TmuxLiveness.PaneDeadStatus *int` (populated
  only when `#{pane_dead}==1`, `:257`), and surface it through `LaneLiveness` so the
  recovery sweep's cached probe carries it. This is an **additive** field; the existing
  pane-dead/pid-mismatch/start-mismatch/healthy classification is unchanged.

### 3.2 The new typed stall class

Add to `go/pkg/mutations/recovery_decision_tree.go` (alongside `:175-185`):
```go
// stallClassSessionUnrecoverableAcrossRotation — a confirmed-dead supervised agent
// whose wrapper exited with the reserved ExitUnrecoverableAcrossRotation (97) floor:
// a non-owner lane was locked out of resealing across a daemon boot-epoch rotation
// (RFC 0143 Slice A / #512). A strict refinement of agent_exited_unsealed for the
// exact-reserved-code case; it routes the SAME finalize-from-durable-artifact path but
// records a distinct, legible reason and remediation.
stallClassSessionUnrecoverableAcrossRotation = "session_unrecoverable_across_rotation"
```

### 3.3 The classification predicate (daemon-side, durable, exact-code-only)

```go
// deadAgentUnrecoverableAcrossRotation reports whether a confirmed-dead lane's
// OBSERVED process exit code is exactly ExitUnrecoverableAcrossRotation (97), read
// from durable daemon-side state ONLY:
//   - direct path: the latest supervisor.agent_exited event payload exit_code for the
//     owning supervisor (striatumd.events); OR
//   - tmux path: PaneDeadStatus from the cached tmux liveness probe (#{pane_dead_status}).
// Returns false for any other code (no over-fire). No inbound frame.
func deadAgentUnrecoverableAcrossRotation(ctx context.Context, tx db.TxRunner, repositoryID string, row map[string]any) bool
```

### 3.4 Wiring into `recoverStuckJobs` (`:704`) — additive, exact-code-gated

The dead-lane classification today (`:1136-1140`) is:
```go
if deadAgentExitedUnsealed(activity) { stallClass = stallClassAgentExitedUnsealed } else { stallClass = stallClassAgentPIDDead }
```
Slice A interposes the exact-code check **first**, leaving both existing branches intact:
```go
if deadAgentUnrecoverableAcrossRotation(ctx, tx, repositoryID, row) {
    stallClass = stallClassSessionUnrecoverableAcrossRotation
} else if deadAgentExitedUnsealed(activity) {
    stallClass = stallClassAgentExitedUnsealed
} else {
    stallClass = stallClassAgentPIDDead
}
```
The same exact-code gate is applied at the auto-finalize-unsealed branch (`:957-1027`) so a
`97`-exit lane with a complete-on-disk deliverable is auto-finalized exactly like
`agent_exited_unsealed` (same `tryFinalizeUnsealedFromDurableArtifact` + `closeStalledOwningSession`
path) but tagged with the typed class.

`isNecrosisStallClass` (`:196-198`) gains the new member (it is a confirmed-dead class):
```go
return stallClass == stallClassAgentPIDDead ||
    stallClass == stallClassAgentExitedUnsealed ||
    stallClass == stallClassSessionUnrecoverableAcrossRotation
```
**Disclosed test change (the only one):** the necrosis domain is pinned by
`TestNecrosisDomainMatchesConfirmedDeadConstants` to
`{agent_pid_dead, agent_exited_unsealed, recovery_exhausted}` (`:191`). It is **extended**
to admit `session_unrecoverable_across_rotation` — an **additive domain growth**, not a
meaning change to any existing member. The RFC 0137 Phase-B necrosis exporter then counts
the new class coherently. (A6.)

### 3.5 The launch-handshake path is NOT a floor carrier in Slice A (dissolving `BC1-W1-CAPTURE-FLOOR`)

The v7 ledger's `BC1-W1-CAPTURE-FLOOR` finding was a property of the **W1 capture
boundary** at launch (a Slice-B mechanism). In **decoupled** Slice A there is **no** W1, no
kernel-token capture, and no capture boundary at launch. The reserved code is produced by
the wrapper **after** `RunHelper` has already emitted `agent_started`:

- `RunHelper` emits `agent_started` immediately after `helperLaunch` returns the started
  process (`helper.go:186-193`), **then** starts the `result.Cmd.Wait()` goroutine
  (`:196-198`). A startup credential refusal makes the wrapper exit `97` **after** that —
  so it is observed as `agent_exited` (exit_code `97`), **never** as `helper_error`.
- `waitForHelperAgentStart` (`supervision_launch.go:562-591`) returns success on the first
  `agent_started` it reads (`:576-577`), which always **precedes** the `agent_exited` in the
  append-only JSONL — so even a fast `97` exit is observed past the handshake.

Therefore the launch/attach `helper_error` phase `launch` path (`helper.go:163-165`;
`waitForHelperAgentStart`'s raw "PTY helper failed before attach", `:579`) **requires no
change** and correctly stays a **raw** `helper_error` for **genuine** launch failures
(exec failure, tmux setup failure) — which are **not** the credential floor. There is no
"covered miss" on that path to leak, so HARD CONSTRAINT 4 (no raw-error leak) holds
**structurally**, and HARD CONSTRAINT 4 (no over-fire) holds because a genuine launch
failure is never reclassified as the floor. **A2 asserts the floor route (reserved code →
typed class); A2-neg asserts the launch failure stays `helper_error`.**

### 3.6 Relationship to `agent_exited_unsealed` and `HandleRecoveryCompleteStalled` (#292)

- vs **`agent_exited_unsealed`**: the typed floor is a **strict refinement** for the exact
  `97` case. It does **not** change `deadAgentExitedUnsealed` (`:1916-1923`, which still keys
  on activity timestamps) or the generic class; a `97`-exit lane is simply tagged with the
  more precise class **before** the generic test runs (§3.4).
- vs **`HandleRecoveryCompleteStalled` (#292, `recovery_complete_stalled.go:49`)**: the
  floor **routes to / does not duplicate** it. When the lane's required artifacts are
  complete-on-disk and reconstructable (`verifyRequiredArtifacts` `:180` +
  `verifyRequiredArtifactReconstructable` `:187`), the typed-class path finalizes via the
  same `finalizeStalledJob` (`:275`) — the deliverable is sealed, the moot blocker
  resolved. When there is **no** durable deliverable, the floor escalates with a **distinct,
  legible reason** ("session unrecoverable across rotation — operator requeue required, the
  supported #512 recovery") instead of a silent unsealed exit, leaving the existing #292
  operator verb (`recovery complete-stalled`) available exactly as today. The typed class
  never **overrides** #292's verdict-capable refusal or its liveness guard.

---

## §4 — C2 forge-resistance (carry the v7 commitment)

The wrapper must **never** let a **provider child's** exit status drive the reserved floor
code. In `runWithIO` (`loop.go:220-369`) the inner agent CLI is `cmd`; its exit is consumed
by `normalizeAgentExitError` (`:365-379`), which returns a **generic** `"agent command
exited"` error — it does **not** propagate the child's numeric code. The reserved `97` is
emitted **only** by `main.go:109-117` and **only** for `errors.Is(err,
ErrUnrecoverableAcrossRotation)`. A provider child that exits `97` (or `98`) therefore
produces a generic error → `log.Fatalf` → wrapper exit `1`, **never** `97`, and never the
sentinel. Test: **`TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker`** (new):
drive a fake inner agent that exits `97`/`98`; assert the wrapper does **not** exit `97`,
the sentinel is not produced, and the daemon records no `session_unrecoverable_across_rotation`
class.

---

## §5 — Falsifiable assertions (each paired with its named test)

- **A1 (Spot 1, narrowing + owner-unaffected).** A **non-owner** lane whose resolution
  chain reaches the admin runtime `client-token` gets `ErrUnrecoverableAcrossRotation` → a
  clean exit with `ExitUnrecoverableAcrossRotation` (97), **not** a generic permission error
  and **not** a silent unsealed exit; an **owner** process (euid == file owner) resolves the
  admin token **normally**, unaffected. **Test: `TestResolveRefusesRuntimeClientTokenForLane`**
  (non-owner: stat-owner-mismatch and EACCES cases → sentinel; **owner companion**
  `TestResolveAdminTokenUnaffectedForOwner` → normal read). Both for `ResolveTokenMaterial`
  and `ResolveTokenMaterialFresh`. Plus `TestAgentLoopMapsUnrecoverableSentinelToReservedExit`
  for the `main.go` mapping.
- **A2 (Spot 2, reserved code → typed class, both observation paths).** A confirmed-dead
  lane whose observed exit code is `97` records the typed
  `session_unrecoverable_across_rotation` class. **Tests:
  `TestRecoverySweepClassifiesReservedExitCodeAsUnrecoverableAcrossRotation`** (direct path,
  `agent_exited.exit_code==97`) and **`TestTmuxPaneDeadStatusReservedCodeRecordsTypedClass`**
  (tmux path, `#{pane_dead_status}==97`). **A2-neg:
  `TestLaunchHandshakeFailureStaysHelperErrorNotFloor`** — a genuine `helper_error` phase
  `launch` stays a raw helper error and produces **no** typed floor class.
- **A3 (no over-fire).** An ordinary unsealed exit (complete-on-disk deliverable, exit code
  `0`/`1`, **no** reserved code) stays `agent_exited_unsealed`; a never-engaged crash stays
  `agent_pid_dead`; the typed floor does **not** fire. **Test (negative):
  `TestOrdinaryUnsealedExitStaysAgentExitedUnsealed`**.
- **A4 (no widening).** No path reads the admin token from a lane; the resolver still
  refuses a non-owner-only file and a non-owner lane never obtains the admin token; no
  minted credential carries an elevated verb. **Test: `TestLaneNeverReadsAdminRuntimeToken`**
  (asserts the contents are never read for a non-owner; the sentinel is returned before any
  `os.ReadFile`), reinforced by A1's owner/non-owner split.
- **A5 (C2 forge-resistance).** A provider child's `97`/`98` cannot drive the reserved
  floor code or the typed class. **Test:
  `TestProviderExitCodeCannotDriveReservedAgentloopResealOrBlocker`** (§4).
- **A6 (no regression; additive).** Existing recovery (`recoverStuckJobs`, the
  `deadAgentExitedUnsealed`/`agent_pid_dead` classification, `HandleRecoveryCompleteStalled`)
  and supervise/agentloop suites pass **unchanged**; the new exit code, derived event, stall
  class, and tmux probe field are additive. **The single disclosed change** is
  `TestNecrosisDomainMatchesConfirmedDeadConstants`, extended to admit the new necrosis
  member (additive domain growth — §3.4). **Test:** the full existing
  `go/pkg/{agentloop,supervisor,mutations}` suites green, plus the updated necrosis-domain
  test asserting exactly `{agent_pid_dead, agent_exited_unsealed,
  session_unrecoverable_across_rotation, recovery_exhausted}`.

---

## §6 — Build slices (contract-first order, smallest safe first) + Acceptance Criteria

1. **Slice A-1 — reserved code + sentinel (pure, no behavior change).** Add
   `go/pkg/agentloop/exitcodes.go` (`ExitUnrecoverableAcrossRotation`,
   `ErrUnrecoverableAcrossRotation`). *Tests:* compile + a trivial constant/identity test.
   No live path touched yet.
2. **Slice A-2 — Spot 1 narrowing + caller mapping.** Add `adminTokenReachedByNonOwner`;
   wire the refusal into `token.go:31-42` and `endpoint.go:125-136`; map the sentinel in
   `loop.go` (`Run`/`RunContext`) and `main.go:109-117`; add the gated
   `requestUnrecoverableExit` rotation-path wiring (`loop.go:602`). *Tests:* **A1, A4, A5**.
   File touches: `token.go`, `endpoint.go`, `loop.go`, `cmd/striatumd/main.go`.
3. **Slice A-3 — Spot 2 tmux observation.** Extend `ProbeTmuxLiveness`
   (`tmux_liveness.go:228`) to capture `#{pane_dead_status}` into
   `TmuxLiveness.PaneDeadStatus`, surfaced via `LaneLiveness`. *Tests:* tmux probe unit test
   (additive field populated only on `pane_dead==1`); existing liveness tests green.
4. **Slice A-4 — Spot 2 typed-class classification + routing.** Add
   `stallClassSessionUnrecoverableAcrossRotation`, `deadAgentUnrecoverableAcrossRotation`;
   interpose the exact-code check in `recoverStuckJobs` (`:1136-1140` and `:957-1027`);
   extend `isNecrosisStallClass` and the necrosis-domain test; optional derived legibility
   event in `supervision.go`'s `agent_exited` branch (`:298-306`). *Tests:* **A2, A2-neg,
   A3, A6**. File touches: `recovery_decision_tree.go`, `supervision.go` (+ test files).

**Acceptance Criteria (the build + verify run must meet — the two game-day shapes):**

- **GD-1 (Spot 1).** A **non-owner** lane whose credential-resolution chain reaches the
  owner-only admin runtime `client-token` exits with `ExitUnrecoverableAcrossRotation` (97)
  and the run records the typed `session_unrecoverable_across_rotation` class — **not** a
  silent unsealed exit and **not** a "permission denied" dead-end; an **owner** process
  resolves the admin token normally (unaffected).
- **GD-2 (Spot 2).** A confirmed-dead lane whose wrapper exited `97` (observed via
  `agent_exited.exit_code` on the direct path or `#{pane_dead_status}` on the tmux path) is
  classified and routed as `session_unrecoverable_across_rotation` — finalized from durable
  artifacts when complete-on-disk, else escalated with the legible "operator requeue
  required" reason — while a genuine launch failure still surfaces a raw `helper_error`
  (no over-fire) and an ordinary unsealed exit stays `agent_exited_unsealed`.

---

<sub>Holder spec for the RFC 0143 **Slice A** falsification-gate design run — the
**decoupled** Option-4 `session_unrecoverable_across_rotation` typed-exit floor: a reserved
agentloop exit code (`ExitUnrecoverableAcrossRotation = 97`) produced by a credential-chain
**narrowing** at `agentloop.ResolveTokenMaterial{,Fresh}` (refuse the owner-only admin token
for a non-owner lane; never read it; owner unaffected), observed by the daemon from durable
process/tmux state (`agent_exited.exit_code` / `#{pane_dead_status}`), and routed as a new
necrosis-class refinement of `agent_exited_unsealed` in `recoverStuckJobs` — with **zero
trust-model change** (no admin-token widening, no minted credential, no Slice-B
channel/reseal/kernel-token/owner-bundle, no reserved code 98). Slice B (the
`CapabilityReseal` authority + connect-out channel) is blocked on RFC 0168 (#585) and is OUT
OF SCOPE. The launch-handshake `BC1-W1-CAPTURE-FLOOR` concern is dissolved structurally:
Slice A has no capture boundary at launch, so the floor is only ever a post-`agent_started`
exit code and the raw-error-leak path is absent. This is the published claim the falsifiers
re-attack; the adjudicator's collaboration ledger gates the phase.</sub>
